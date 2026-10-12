package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	pool := postgrestest.NewPool(t)
	if _, err := postgres.Migrate(context.Background(), pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	return store.New(pool)
}

func newDocument(sha string) (rag.Document, rag.Job) {
	doc := rag.Document{
		ID:            uuid.Must(uuid.NewV7()),
		Source:        "RCA 042/2019",
		Collection:    "rca",
		Metadata:      map[string]any{"titular": "Minera Loma Seca SpA"},
		ContentSHA256: sha,
	}
	job := rag.Job{ID: uuid.Must(uuid.NewV7()), Kind: rag.JobIngest}
	return doc, job
}

func TestCreateDocument(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")

	createdDoc, createdJob, err := s.CreateDocument(ctx, doc, job, nil)
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	if createdDoc.Status != rag.DocumentPending || createdJob.Status != rag.JobQueued {
		t.Errorf("got %s/%s, want pending/queued", createdDoc.Status, createdJob.Status)
	}
	if createdDoc.CreatedAt.IsZero() || createdJob.CreatedAt.IsZero() {
		t.Error("la base de datos no asignó las fechas de creación")
	}

	gotDoc, err := s.Document(ctx, doc.ID)
	if err != nil {
		t.Fatalf("leer el documento: %v", err)
	}
	if gotDoc.Source != doc.Source || gotDoc.Metadata["titular"] != "Minera Loma Seca SpA" {
		t.Errorf("el documento leído difiere del creado: %+v", gotDoc)
	}

	gotJob, err := s.Job(ctx, job.ID)
	if err != nil {
		t.Fatalf("leer el trabajo: %v", err)
	}
	if gotJob.DocumentID != doc.ID || gotJob.Kind != rag.JobIngest || gotJob.Error != "" {
		t.Errorf("el trabajo leído difiere del creado: %+v", gotJob)
	}
}

func TestCreateDocumentWithoutMetadata(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")
	doc.Metadata = nil

	if _, _, err := s.CreateDocument(ctx, doc, job, nil); err != nil {
		t.Fatalf("crear: %v", err)
	}
	got, err := s.Document(ctx, doc.ID)
	if err != nil {
		t.Fatalf("leer: %v", err)
	}
	if got.Metadata == nil || len(got.Metadata) != 0 {
		t.Errorf("got metadata %v, want objeto vacío", got.Metadata)
	}
}

func TestCreateDocumentRejectsDuplicateContent(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	doc, job := newDocument("aaaa")
	if _, _, err := s.CreateDocument(ctx, doc, job, nil); err != nil {
		t.Fatalf("crear el primero: %v", err)
	}

	// Mismo contenido, misma colección: es el mismo documento.
	dup, dupJob := newDocument("aaaa")
	if _, _, err := s.CreateDocument(ctx, dup, dupJob, nil); !errors.Is(err, rag.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	// El rechazo no debe dejar un trabajo huérfano.
	if _, err := s.Job(ctx, dupJob.ID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("el trabajo del duplicado quedó registrado: %v", err)
	}

	// Mismo contenido en otra colección: es otro documento.
	other, otherJob := newDocument("aaaa")
	other.Collection = "normativa"
	if _, _, err := s.CreateDocument(ctx, other, otherJob, nil); err != nil {
		t.Errorf("mismo contenido en otra colección: %v", err)
	}
}

// Una distancia entre vectores solo se resuelve si la migración instaló la
// extensión y esta es visible desde el esquema de trabajo.
func TestMigrationsEnableVector(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	var distance float64
	if err := pool.QueryRow(ctx, `SELECT '[1,0]'::vector <-> '[0,1]'::vector`).Scan(&distance); err != nil {
		t.Fatalf("pgvector no está disponible: %v", err)
	}
	if distance < 1.41 || distance > 1.42 {
		t.Errorf("got distance %f, want √2", distance)
	}
}

func TestNotFound(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7())

	if _, err := s.Document(ctx, id); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("documento: got %v, want ErrNotFound", err)
	}
	if _, err := s.Job(ctx, id); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("trabajo: got %v, want ErrNotFound", err)
	}
}

// Con fechas de creación idénticas, el desempate por id es lo único que
// impide que el recorrido repita o salte documentos.
func TestListDocumentsBreaksTiesByID(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	s := store.New(pool)

	for _, sha := range []string{"aaaa", "bbbb", "cccc"} {
		doc, job := newDocument(sha)
		if _, _, err := s.CreateDocument(ctx, doc, job, nil); err != nil {
			t.Fatalf("crear: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE documents SET created_at = '2026-01-01T00:00:00Z'`); err != nil {
		t.Fatalf("igualar las fechas: %v", err)
	}

	seen := map[uuid.UUID]bool{}
	var after *rag.Position
	for range 3 {
		docs, err := s.ListDocuments(ctx, "rca", after, 1)
		if err != nil {
			t.Fatalf("listar: %v", err)
		}
		if len(docs) != 1 {
			t.Fatalf("got %d documentos, want 1", len(docs))
		}
		if seen[docs[0].ID] {
			t.Fatalf("el documento %s se repite", docs[0].ID)
		}
		seen[docs[0].ID] = true
		after = &rag.Position{CreatedAt: docs[0].CreatedAt, ID: docs[0].ID}
	}

	if docs, err := s.ListDocuments(ctx, "rca", after, 1); err != nil || len(docs) != 0 {
		t.Errorf("tras el último documento: got %d y %v, want 0", len(docs), err)
	}
}

func newChunks(n int) []rag.Chunk {
	chunks := make([]rag.Chunk, n)
	for i := range chunks {
		chunks[i] = rag.Chunk{
			ID:        uuid.Must(uuid.NewV7()),
			Index:     i,
			Section:   fmt.Sprintf("Considerando %d", i+1),
			Heading:   fmt.Sprintf("Título %d", i+1),
			PageStart: i + 1,
			PageEnd:   i + 2,
			Text:      fmt.Sprintf("Texto del fragmento %d.", i),
		}
	}
	return chunks
}

func TestCreateDocumentStoresChunks(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	s := store.New(pool)
	doc, job := newDocument("aaaa")
	want := newChunks(3)

	created, _, err := s.CreateDocument(ctx, doc, job, want)
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	if created.ChunkCount != 3 {
		t.Errorf("got chunk_count %d, want 3", created.ChunkCount)
	}
	if got, err := s.Document(ctx, doc.ID); err != nil || got.ChunkCount != 3 {
		t.Errorf("documento leído: chunk_count %d, err %v", got.ChunkCount, err)
	}

	got, err := s.Chunks(ctx, doc.ID)
	if err != nil {
		t.Fatalf("leer los fragmentos: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d fragmentos, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fragmento %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}

	// Los vectores quedan vacíos hasta que se generan los embeddings.
	var pending int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM chunks WHERE embedding IS NULL AND embedding_model IS NULL`).Scan(&pending); err != nil || pending != 3 {
		t.Errorf("got %d fragmentos pendientes, err %v; want 3", pending, err)
	}

	// Eliminar el documento elimina sus fragmentos.
	if err := s.DeleteDocument(ctx, doc.ID); err != nil {
		t.Fatalf("eliminar: %v", err)
	}
	if got, err := s.Chunks(ctx, doc.ID); err != nil || len(got) != 0 {
		t.Errorf("tras eliminar quedan %d fragmentos, err %v", len(got), err)
	}
}

// Si los fragmentos no pueden guardarse, no debe quedar el documento.
func TestCreateDocumentIsAtomicWithChunks(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")
	chunks := newChunks(2)
	chunks[1].Index = chunks[0].Index // posición repetida: viola la unicidad

	if _, _, err := s.CreateDocument(ctx, doc, job, chunks); err == nil {
		t.Fatal("se esperaba un error por la posición repetida")
	}
	if _, err := s.Document(ctx, doc.ID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("el documento quedó registrado sin sus fragmentos: %v", err)
	}
	if _, err := s.Job(ctx, job.ID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("el trabajo quedó registrado: %v", err)
	}
}

func TestDocumentExists(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")
	if _, _, err := s.CreateDocument(ctx, doc, job, nil); err != nil {
		t.Fatalf("crear: %v", err)
	}

	for _, tt := range []struct {
		collection, sha string
		want            bool
	}{
		{"rca", "aaaa", true},
		{"rca", "bbbb", false},
		{"normativa", "aaaa", false},
	} {
		got, err := s.DocumentExists(ctx, tt.collection, tt.sha)
		if err != nil || got != tt.want {
			t.Errorf("%s/%s: got %v, err %v; want %v", tt.collection, tt.sha, got, err, tt.want)
		}
	}
}

// Una reserva vencida puede tomarla otra pasada: una que murió sin liberar
// no deja el trabajo trabado para siempre.
func TestClaimJobLeaseExpires(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")
	if _, _, err := s.CreateDocument(ctx, doc, job, newChunks(1)); err != nil {
		t.Fatalf("crear: %v", err)
	}

	claimed, ok, err := s.ClaimJob(ctx, job.ID, time.Hour)
	if err != nil || !ok || claimed.Status != rag.JobRunning {
		t.Fatalf("primera reserva: ok %v, estado %q, err %v", ok, claimed.Status, err)
	}
	if got, err := s.Document(ctx, doc.ID); err != nil || got.Status != rag.DocumentProcessing {
		t.Errorf("got documento %q, err %v; want processing", got.Status, err)
	}

	// Mientras la reserva está vigente, nadie más lo toma.
	if current, ok, err := s.ClaimJob(ctx, job.ID, time.Hour); err != nil || ok || current.Status != rag.JobRunning {
		t.Errorf("segunda reserva: ok %v, estado %q, err %v; want rechazada", ok, current.Status, err)
	}

	// Liberada, sí.
	if err := s.ReleaseJob(ctx, job.ID); err != nil {
		t.Fatalf("liberar: %v", err)
	}
	if _, ok, err := s.ClaimJob(ctx, job.ID, -time.Second); err != nil || !ok {
		t.Fatalf("reserva tras liberar: ok %v, err %v", ok, err)
	}

	// Esa última reserva nació vencida: otra pasada puede tomarla sin que
	// nadie la libere.
	if _, ok, err := s.ClaimJob(ctx, job.ID, time.Hour); err != nil || !ok {
		t.Errorf("reserva sobre una vencida: ok %v, err %v; want aceptada", ok, err)
	}

	if _, _, err := s.ClaimJob(ctx, uuid.Must(uuid.NewV7()), time.Hour); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("trabajo inexistente: got %v, want ErrNotFound", err)
	}
}

func TestSaveEmbeddingsAndFinish(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	s := store.New(pool)
	doc, job := newDocument("aaaa")
	chunks := newChunks(4)
	if _, _, err := s.CreateDocument(ctx, doc, job, chunks); err != nil {
		t.Fatalf("crear: %v", err)
	}

	vector := make([]float32, 768)
	vector[0], vector[767] = 0.6, -0.8

	updated, err := s.SaveEmbeddings(ctx, job.ID, "modelo-x", []rag.ChunkEmbedding{{ChunkID: chunks[0].ID, Vector: vector}})
	if err != nil {
		t.Fatalf("guardar: %v", err)
	}
	if updated.Progress != 25 {
		t.Errorf("got avance %d %%, want 25", updated.Progress)
	}

	// El vector se recupera con los mismos valores.
	var first, last float32
	var model string
	err = pool.QueryRow(ctx, `
		SELECT (embedding::real[])[1], (embedding::real[])[768], embedding_model
		FROM chunks WHERE id = $1`, chunks[0].ID).Scan(&first, &last, &model)
	if err != nil || first != 0.6 || last != -0.8 || model != "modelo-x" {
		t.Errorf("got %v, %v, %q, err %v; want 0.6, -0.8, modelo-x", first, last, model, err)
	}

	pending, err := s.PendingChunks(ctx, doc.ID, 10)
	if err != nil || len(pending) != 3 || pending[0].ID != chunks[1].ID {
		t.Fatalf("got %d pendientes, err %v; want 3 a partir del segundo", len(pending), err)
	}
	if limited, err := s.PendingChunks(ctx, doc.ID, 2); err != nil || len(limited) != 2 {
		t.Errorf("con límite 2: got %d, err %v", len(limited), err)
	}

	finished, err := s.FinishJob(ctx, job.ID, rag.JobFailed, "No se pudo.")
	if err != nil || finished.Status != rag.JobFailed || finished.Error != "No se pudo." || finished.Progress != 25 {
		t.Errorf("cierre fallido: %+v, err %v", finished, err)
	}
	if got, err := s.Document(ctx, doc.ID); err != nil || got.Status != rag.DocumentFailed {
		t.Errorf("got documento %q, err %v; want failed", got.Status, err)
	}
	// Un trabajo cerrado no se puede reservar.
	if _, ok, err := s.ClaimJob(ctx, job.ID, time.Hour); err != nil || ok {
		t.Errorf("reserva de un trabajo cerrado: ok %v, err %v; want rechazada", ok, err)
	}
}

// Un vector de otra dimensión no puede guardarse: la columna la fija.
func TestSaveEmbeddingsRejectsWrongDimension(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")
	chunks := newChunks(1)
	if _, _, err := s.CreateDocument(ctx, doc, job, chunks); err != nil {
		t.Fatalf("crear: %v", err)
	}

	_, err := s.SaveEmbeddings(ctx, job.ID, "modelo-x", []rag.ChunkEmbedding{{ChunkID: chunks[0].ID, Vector: make([]float32, 3)}})
	if err == nil {
		t.Fatal("se aceptó un vector de 3 dimensiones")
	}
	if pending, err := s.PendingChunks(ctx, doc.ID, 10); err != nil || len(pending) != 1 {
		t.Errorf("got %d pendientes, err %v; want 1", len(pending), err)
	}
}
