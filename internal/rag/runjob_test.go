package rag_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

// recordingEmbedder devuelve un vector de 768 dimensiones por texto y anota
// lo que se le pide.
type recordingEmbedder struct {
	mu    sync.Mutex
	texts []string
	// fail, si no es nil, decide el error de cada llamada según su número
	// (desde 1).
	fail func(call int) error
	// onCall se ejecuta al inicio de cada llamada.
	onCall func(call int)
	calls  int

	queries  []string
	queryErr error
}

func (e *recordingEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	e.calls++
	call := e.calls
	e.mu.Unlock()

	if e.onCall != nil {
		e.onCall(call)
	}
	if e.fail != nil {
		if err := e.fail(call); err != nil {
			return nil, err
		}
	}

	e.mu.Lock()
	e.texts = append(e.texts, texts...)
	e.mu.Unlock()

	out := make([][]float32, len(texts))
	for i := range out {
		v := make([]float32, 768)
		v[0] = 1
		out[i] = v
	}
	return out, nil
}

// EmbedQuery devuelve el vector configurado para la consulta, o el error.
func (e *recordingEmbedder) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.queries = append(e.queries, text)
	if e.queryErr != nil {
		return nil, e.queryErr
	}
	v := make([]float32, 768)
	v[0] = 1
	return v, nil
}

func (e *recordingEmbedder) Model() string { return "modelo-de-prueba" }

func (e *recordingEmbedder) seen() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.texts...)
}

// recordingDispatcher anota los trabajos encolados sin ejecutarlos.
type recordingDispatcher struct {
	mu   sync.Mutex
	jobs []uuid.UUID
	err  error
}

func (d *recordingDispatcher) Enqueue(_ context.Context, id uuid.UUID) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.err != nil {
		return d.err
	}
	d.jobs = append(d.jobs, id)
	return nil
}

func (d *recordingDispatcher) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.jobs)
}

// nChunks produce n fragmentos por documento, el primero con el título en su
// texto y los demás sin él, como cuando una sección se divide.
func nChunks(n int) rag.Splitter {
	return func(pages []rag.PageText) []rag.Chunk {
		chunks := make([]rag.Chunk, n)
		for i := range chunks {
			text := fmt.Sprintf("parte %d", i)
			if i == 0 {
				text = "10.1. Plan de Seguimiento\nparte 0"
			}
			chunks[i] = rag.Chunk{Index: i, Section: "Considerando 10.1", Heading: "Plan de Seguimiento", PageStart: 1, PageEnd: 1, Text: text}
		}
		return chunks
	}
}

type runFixture struct {
	service    *rag.Service
	pool       *pgxpool.Pool
	embedder   *recordingEmbedder
	dispatcher *recordingDispatcher
	clock      *time.Time
}

// newRunFixture arma el servicio con n fragmentos por documento, un reloj
// que solo avanza cuando el test lo indica y una pasada de un minuto.
func newRunFixture(t *testing.T, n int) runFixture {
	t.Helper()
	pool := postgrestest.NewPool(t)
	if _, err := postgres.Migrate(context.Background(), pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	calls := 0
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	f := runFixture{pool: pool, embedder: &recordingEmbedder{}, dispatcher: &recordingDispatcher{}, clock: &clock}
	f.service = rag.NewService(rag.Deps{
		Store:      store.New(pool),
		Extractor:  countingExtractor{calls: &calls},
		Split:      nChunks(n),
		Embedder:   f.embedder,
		Dispatcher: f.dispatcher,
		PassBudget: time.Minute,
		Now:        func() time.Time { return clock },
	})
	return f
}

// embedded cuenta los fragmentos del documento que ya tienen vector.
func (f runFixture) embedded(t *testing.T, documentID uuid.UUID) (done, total int) {
	t.Helper()
	err := f.pool.QueryRow(context.Background(), `
		SELECT count(*) FILTER (WHERE embedding IS NOT NULL), count(*)
		FROM chunks WHERE document_id = $1`, documentID).Scan(&done, &total)
	if err != nil {
		t.Fatalf("contar los fragmentos: %v", err)
	}
	return done, total
}

func TestSubmitEnqueuesTheJob(t *testing.T) {
	f := newRunFixture(t, 3)

	job := submit(t, f.service, "contenido", "rca")

	if f.dispatcher.count() != 1 || f.dispatcher.jobs[0] != job.ID {
		t.Errorf("got trabajos encolados %v, want [%s]", f.dispatcher.jobs, job.ID)
	}
	if job.Status != rag.JobQueued {
		t.Errorf("got estado %q, want queued", job.Status)
	}
}

// Si el trabajo no puede encolarse, el documento no debe quedar registrado:
// nadie lo procesaría, y volver a subirlo daría "duplicado".
func TestSubmitUndoesRegistrationWhenEnqueueFails(t *testing.T) {
	f := newRunFixture(t, 3)
	ctx := context.Background()
	f.dispatcher.err = errors.New("Cloud Tasks respondió 503")

	_, err := f.service.Submit(ctx, rag.Upload{Filename: "a.pdf", Collection: "rca", Content: []byte("%PDF contenido")})

	if !errors.Is(err, rag.ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	if page, err := f.service.Documents(ctx, rag.ListOptions{}); err != nil || len(page.Documents) != 0 {
		t.Errorf("quedaron %d documentos registrados, err %v", len(page.Documents), err)
	}

	// Resuelta la falla, la misma subida funciona.
	f.dispatcher.err = nil
	submit(t, f.service, "contenido", "rca")
}

func TestRunJobEmbedsEveryChunk(t *testing.T) {
	f := newRunFixture(t, 20)
	ctx := context.Background()
	job := submit(t, f.service, "contenido", "rca")

	ran, err := f.service.RunJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("ejecutar: %v", err)
	}

	if ran.Status != rag.JobSucceeded || ran.Progress != 100 {
		t.Errorf("got %s al %d %%, want succeeded al 100", ran.Status, ran.Progress)
	}
	if done, total := f.embedded(t, job.DocumentID); done != 20 || total != 20 {
		t.Errorf("got %d de %d fragmentos con vector, want 20 de 20", done, total)
	}
	doc, err := f.service.Document(ctx, job.DocumentID)
	if err != nil || doc.Status != rag.DocumentReady {
		t.Errorf("got documento %q, err %v; want ready", doc.Status, err)
	}

	// Cada fragmento se pidió una sola vez, y el modelo quedó anotado.
	if got := len(f.embedder.seen()); got != 20 {
		t.Errorf("se pidieron %d embeddings, want 20", got)
	}
	var models int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM chunks WHERE embedding_model = 'modelo-de-prueba' AND public.vector_dims(embedding) = 768`).Scan(&models); err != nil || models != 20 {
		t.Errorf("got %d fragmentos con modelo y dimensión correctos, err %v; want 20", models, err)
	}
}

// A los fragmentos que no llevan el título de su sección se les antepone:
// sin él, el vector no sabría de qué trata el pasaje.
func TestRunJobGivesContextToContinuationChunks(t *testing.T) {
	f := newRunFixture(t, 2)
	job := submit(t, f.service, "contenido", "rca")

	if _, err := f.service.RunJob(context.Background(), job.ID); err != nil {
		t.Fatalf("ejecutar: %v", err)
	}

	texts := f.embedder.seen()
	if texts[0] != "10.1. Plan de Seguimiento\nparte 0" {
		t.Errorf("el primer fragmento ya lleva el título y no debe repetirlo: %q", texts[0])
	}
	if texts[1] != "Plan de Seguimiento\nparte 1" {
		t.Errorf("al segundo fragmento le falta el título: %q", texts[1])
	}
}

// La introducción del ítem al que pertenece un fragmento se antepone al
// texto que se convierte en vector: "8.3. Plan de monitoreo de aguas" debe
// poder encontrarse preguntando por los compromisos voluntarios.
func TestRunJobPrependsItemContext(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	calls := 0
	embedder := &recordingEmbedder{}
	withContext := func([]rag.PageText) []rag.Chunk {
		return []rag.Chunk{
			{Index: 0, Section: "Considerando 8.3", Heading: "Plan de Monitoreo de Aguas Superficiales",
				Context:   "8. Que, el proyecto contempla realizar los siguientes compromisos ambientales voluntarios:",
				PageStart: 87, PageEnd: 88, Text: "8.3. Plan de Monitoreo de Aguas Superficiales.\nMonitoreo trimestral."},
		}
	}
	s := rag.NewService(rag.Deps{Store: store.New(pool), Extractor: countingExtractor{calls: &calls}, Split: withContext, Embedder: embedder})
	job := submit(t, s, "contenido", "rca")

	if _, err := s.RunJob(ctx, job.ID); err != nil {
		t.Fatalf("ejecutar: %v", err)
	}

	want := "8. Que, el proyecto contempla realizar los siguientes compromisos ambientales voluntarios:\n8.3. Plan de Monitoreo de Aguas Superficiales.\nMonitoreo trimestral."
	if got := embedder.seen(); len(got) != 1 || got[0] != want {
		t.Errorf("got texto para el vector %q,\nwant %q", got, want)
	}
}

// Una pasada trabaja un tiempo acotado. Lo que no alcanza queda para la
// siguiente, que continúa donde quedó sin repetir lo ya hecho.
func TestRunJobWorksInBoundedPasses(t *testing.T) {
	f := newRunFixture(t, 20)
	ctx := context.Background()
	job := submit(t, f.service, "contenido", "rca")
	enqueuedBySubmit := f.dispatcher.count()

	// El plazo se cumple durante el segundo grupo de fragmentos.
	f.embedder.onCall = func(call int) {
		if call == 2 {
			*f.clock = f.clock.Add(2 * time.Minute)
		}
	}

	first, err := f.service.RunJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	if first.Status != rag.JobRunning {
		t.Fatalf("got estado %q, want running", first.Status)
	}
	done, total := f.embedded(t, job.DocumentID)
	if done == 0 || done == total {
		t.Fatalf("la primera pasada dejó %d de %d fragmentos; debía avanzar sin terminar", done, total)
	}
	if first.Progress != done*100/total {
		t.Errorf("got avance %d %%, want %d %%", first.Progress, done*100/total)
	}
	if got := f.dispatcher.count() - enqueuedBySubmit; got != 1 {
		t.Fatalf("la pasada encoló %d continuaciones, want 1", got)
	}

	f.embedder.onCall = nil
	second, err := f.service.RunJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if second.Status != rag.JobSucceeded || second.Progress != 100 {
		t.Errorf("got %s al %d %%, want succeeded al 100", second.Status, second.Progress)
	}

	// Entre las dos pasadas, cada fragmento se pidió exactamente una vez.
	seen := map[string]int{}
	for _, text := range f.embedder.seen() {
		seen[text]++
	}
	if len(seen) != 20 {
		t.Errorf("se pidieron %d fragmentos distintos, want 20", len(seen))
	}
	for text, n := range seen {
		if n != 1 {
			t.Errorf("el fragmento %q se pidió %d veces", text, n)
		}
	}
}

// Si el proveedor no responde, la pasada falla sin perder lo avanzado y sin
// dejar el trabajo reservado: puede reintentarse de inmediato.
func TestRunJobKeepsProgressWhenProviderIsUnavailable(t *testing.T) {
	f := newRunFixture(t, 20)
	ctx := context.Background()
	job := submit(t, f.service, "contenido", "rca")

	f.embedder.fail = func(call int) error {
		if call == 2 {
			return fmt.Errorf("%w: sin respuesta", llmgw.ErrUnavailable)
		}
		return nil
	}

	_, err := f.service.RunJob(ctx, job.ID)
	if !errors.Is(err, rag.ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	done, _ := f.embedded(t, job.DocumentID)
	if done != 8 {
		t.Errorf("quedaron %d fragmentos con vector, want los 8 del primer grupo", done)
	}
	current, err := f.service.Job(ctx, job.ID)
	if err != nil || current.Status != rag.JobRunning || current.Error != "" {
		t.Errorf("got trabajo %q con error %q, err %v; want running sin error", current.Status, current.Error, err)
	}

	// El reintento continúa y termina.
	f.embedder.fail = nil
	ran, err := f.service.RunJob(ctx, job.ID)
	if err != nil || ran.Status != rag.JobSucceeded {
		t.Fatalf("reintento: got %q, err %v; want succeeded", ran.Status, err)
	}
}

// Un error que no se arregla reintentando cierra el trabajo como fallido,
// con un mensaje para quien lo consulta que no expone el detalle interno.
func TestRunJobFailsOnPermanentErrors(t *testing.T) {
	for name, cause := range map[string]error{
		"contenido rechazado": fmt.Errorf("%w (400)", llmgw.ErrInvalidInput),
		"mala configuración":  fmt.Errorf("%w: el proveedor rechaza la clave AIza-secreta", llmgw.ErrMisconfigured),
		"error desconocido":   errors.New("dial tcp 10.0.0.7:443: connection refused"),
	} {
		t.Run(name, func(t *testing.T) {
			f := newRunFixture(t, 3)
			ctx := context.Background()
			job := submit(t, f.service, "contenido", "rca")
			f.embedder.fail = func(int) error { return cause }

			ran, err := f.service.RunJob(ctx, job.ID)
			if err != nil {
				t.Fatalf("got error %v; un fallo permanente se informa en el trabajo, no como error", err)
			}

			if ran.Status != rag.JobFailed || ran.Error == "" {
				t.Fatalf("got %q con mensaje %q, want failed con mensaje", ran.Status, ran.Error)
			}
			for _, secret := range []string{"AIza-secreta", "10.0.0.7", "400"} {
				if strings.Contains(ran.Error, secret) {
					t.Errorf("el mensaje expone %q: %s", secret, ran.Error)
				}
			}
			doc, err := f.service.Document(ctx, job.DocumentID)
			if err != nil || doc.Status != rag.DocumentFailed {
				t.Errorf("got documento %q, err %v; want failed", doc.Status, err)
			}

			// Un trabajo fallido no se vuelve a ejecutar.
			calls := f.embedder.calls
			again, err := f.service.RunJob(ctx, job.ID)
			if err != nil || again.Status != rag.JobFailed || f.embedder.calls != calls {
				t.Errorf("segunda ejecución: estado %q, err %v, llamadas nuevas %d", again.Status, err, f.embedder.calls-calls)
			}
		})
	}
}

// Dos pasadas simultáneas del mismo trabajo no deben procesar los mismos
// fragmentos: la segunda encuentra el trabajo reservado y no hace nada.
func TestRunJobDoesNotRunTwiceAtOnce(t *testing.T) {
	f := newRunFixture(t, 3)
	ctx := context.Background()
	job := submit(t, f.service, "contenido", "rca")

	inside, release := make(chan struct{}), make(chan struct{})
	f.embedder.onCall = func(call int) {
		if call == 1 {
			close(inside)
			<-release
		}
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := f.service.RunJob(ctx, job.ID)
		firstDone <- err
	}()
	<-inside // la primera pasada tiene el trabajo y espera al proveedor

	second, err := f.service.RunJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("segunda pasada: %v", err)
	}
	if second.Status != rag.JobRunning {
		t.Errorf("got estado %q, want running", second.Status)
	}
	if f.embedder.calls != 1 {
		t.Errorf("la segunda pasada llamó al proveedor: %d llamadas, want 1", f.embedder.calls)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("primera pasada: %v", err)
	}
	if done, total := f.embedded(t, job.DocumentID); done != total {
		t.Errorf("quedaron %d de %d fragmentos con vector", done, total)
	}
}

func TestRunJobUnknownAndUnconfigured(t *testing.T) {
	f := newRunFixture(t, 1)
	ctx := context.Background()

	if _, err := f.service.RunJob(ctx, uuid.Must(uuid.NewV7())); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("trabajo inexistente: got %v, want ErrNotFound", err)
	}

	// Sin proveedor de embeddings, el trabajo no se toca.
	calls := 0
	withoutEmbedder := rag.NewService(rag.Deps{Store: store.New(f.pool), Extractor: countingExtractor{calls: &calls}, Split: nChunks(1)})
	job := submit(t, withoutEmbedder, "otro", "rca")
	if _, err := withoutEmbedder.RunJob(ctx, job.ID); !errors.Is(err, rag.ErrUnavailable) {
		t.Errorf("sin embeddings: got %v, want ErrUnavailable", err)
	}
	if current, err := withoutEmbedder.Job(ctx, job.ID); err != nil || current.Status != rag.JobQueued {
		t.Errorf("got trabajo %q, err %v; want queued", current.Status, err)
	}
}
