package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// fakeRetriever devuelve pasajes fijos y anota las búsquedas que recibe.
type fakeRetriever struct {
	mu       sync.Mutex
	passages []rag.Passage
	err      error
	searches []search
}

type search struct {
	query string
	opts  rag.SearchOptions
}

func (f *fakeRetriever) Search(_ context.Context, query string, opts rag.SearchOptions) ([]rag.Passage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches = append(f.searches, search{query, opts})
	if f.err != nil {
		return nil, f.err
	}
	return f.passages[:min(opts.TopK, len(f.passages))], nil
}

func (f *fakeRetriever) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.searches)
}

func passage(section string, pageStart, pageEnd int, score float64, text string) rag.Passage {
	return rag.Passage{
		ChunkID: uuid.New(), DocumentID: uuid.New(), Source: "RCA Tres Valles",
		Section: section, PageStart: pageStart, PageEnd: pageEnd, Score: score, Text: text,
	}
}

type fixture struct {
	service   *orchestrator.Service
	retriever *fakeRetriever
	clock     *time.Time
}

func newFixture(budget int, passages ...rag.Passage) fixture {
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	r := &fakeRetriever{passages: passages}
	return fixture{
		service: orchestrator.NewService(orchestrator.Config{
			Retriever: r, DailyBudget: budget, Now: func() time.Time { return clock },
		}),
		retriever: r,
		clock:     &clock,
	}
}

func ask(t *testing.T, s *orchestrator.Service, q orchestrator.Query) orchestrator.Answer {
	t.Helper()
	answer, err := s.Answer(context.Background(), q)
	if err != nil {
		t.Fatalf("responder %q: %v", q.Question, err)
	}
	return answer
}

func TestAnswerReturnsPassagesAsCitations(t *testing.T) {
	air := passage("Considerando 10.1", 89, 90, 0.82, "10.1. Plan de Seguimiento Estaciones de Monitoreo Calidad del Aire.")
	water := passage("Considerando 10.2", 91, 91, 0.61, "10.2. Plan de Seguimiento de Aguas.")
	f := newFixture(0, air, water)

	answer := ask(t, f.service, orchestrator.Query{Question: "¿Cómo se monitorea la calidad del aire?"})

	if len(answer.Citations) != 2 {
		t.Fatalf("got %d citas, want 2", len(answer.Citations))
	}
	c := answer.Citations[0]
	if c.ChunkID != air.ChunkID || c.DocumentID != air.DocumentID || c.Source != "RCA Tres Valles" || c.Score != 0.82 {
		t.Errorf("la cita no corresponde al pasaje: %+v", c)
	}
	if c.Locator != "Considerando 10.1, págs. 89-90" {
		t.Errorf("got ubicación %q", c.Locator)
	}
	// Un pasaje de una sola página no se cita como un rango.
	if got := answer.Citations[1].Locator; got != "Considerando 10.2, pág. 91" {
		t.Errorf("got ubicación %q", got)
	}
	if c.Excerpt != air.Text {
		t.Errorf("got extracto %q", c.Excerpt)
	}
	if !strings.Contains(answer.Text, "2 pasajes") {
		t.Errorf("la respuesta no dice cuántos pasajes entrega: %q", answer.Text)
	}
}

// Sin modelo que redacte, la respuesta entrega los pasajes y la traza debe
// decirlo: una respuesta sin generación no puede pasar por una generada.
func TestAnswerDeclaresDegradedMode(t *testing.T) {
	f := newFixture(0, passage("Considerando 1", 1, 1, 0.7, "texto"))

	answer := ask(t, f.service, orchestrator.Query{Question: "¿Qué dice el considerando 1?"})

	tr := answer.Trace
	if !tr.Degraded || tr.Provider != "none" || tr.Model != "none" {
		t.Errorf("got degradado %v, proveedor %q, modelo %q", tr.Degraded, tr.Provider, tr.Model)
	}
	if tr.Tokens.Prompt != 0 || tr.Tokens.Completion != 0 || tr.EstimatedCostUSD == nil || *tr.EstimatedCostUSD != 0 || tr.Latency.Generation != 0 {
		t.Errorf("la traza declara consumo de generación sin haber generado: %+v", tr)
	}
	if len(tr.ID) != 32 || strings.Contains(tr.ID, "-") {
		t.Errorf("got identificador de traza %q, want 32 hexadecimales", tr.ID)
	}
	if answer.ID == uuid.Nil {
		t.Error("la respuesta no tiene identificador")
	}
}

// Sin pasajes no se afirma nada: la respuesta dice que no hay sustento.
func TestAnswerWithoutSources(t *testing.T) {
	f := newFixture(0)

	answer := ask(t, f.service, orchestrator.Query{Question: "¿Cuál es el plan de cierre?"})

	if len(answer.Citations) != 0 {
		t.Errorf("got %d citas", len(answer.Citations))
	}
	if !strings.Contains(answer.Text, "No se encontraron pasajes") {
		t.Errorf("got respuesta %q", answer.Text)
	}
}

func TestAnswerPassesOptionsToRetrieval(t *testing.T) {
	f := newFixture(0, passage("", 3, 3, 0.9, "a"), passage("", 4, 4, 0.8, "b"), passage("", 5, 5, 0.7, "c"))

	answer := ask(t, f.service, orchestrator.Query{Question: "  ¿Qué pozos?  ", Collection: " rca ", TopK: 2})

	got := f.retriever.searches[0]
	if got.query != "¿Qué pozos?" || got.opts.Collection != "rca" || got.opts.TopK != 2 {
		t.Errorf("búsqueda inesperada: %+v", got)
	}
	if len(answer.Citations) != 2 {
		t.Errorf("got %d citas, want 2", len(answer.Citations))
	}
	// Sin sección reconocida, la ubicación es la página.
	if answer.Citations[0].Locator != "pág. 3" {
		t.Errorf("got ubicación %q", answer.Citations[0].Locator)
	}
}

func TestAnswerDefaultsAndValidation(t *testing.T) {
	f := newFixture(0, passage("", 1, 1, 0.7, "a"))
	ask(t, f.service, orchestrator.Query{Question: "¿Qué pozos?"})
	if got := f.retriever.searches[0].opts.TopK; got != 5 {
		t.Errorf("got top_k por omisión %d, want 5", got)
	}

	for name, q := range map[string]orchestrator.Query{
		"pregunta corta":    {Question: "ab"},
		"pregunta larga":    {Question: strings.Repeat("a", 2001)},
		"top_k excesivo":    {Question: "¿Qué pozos?", TopK: 21},
		"top_k negativo":    {Question: "¿Qué pozos?", TopK: -1},
		"traza desconocida": {Question: "¿Qué pozos?", Trace: "debug"},
	} {
		if _, err := f.service.Answer(context.Background(), q); !errors.Is(err, orchestrator.ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", name, err)
		}
	}
	if f.retriever.count() != 1 {
		t.Errorf("las consultas inválidas llegaron a buscar: %d búsquedas, want 1", f.retriever.count())
	}
}

func TestAnswerTraceLevel(t *testing.T) {
	f := newFixture(0, passage("", 1, 1, 0.9, "a"), passage("", 2, 2, 0.8, "b"))

	summary := ask(t, f.service, orchestrator.Query{Question: "pregunta uno"})
	full := ask(t, f.service, orchestrator.Query{Question: "pregunta dos", Trace: orchestrator.TraceFull})

	if summary.Trace.Retrieved != nil {
		t.Error("trace=summary no debe incluir los pasajes recuperados")
	}
	if len(full.Trace.Retrieved) != 2 || full.Trace.Retrieved[0].Rank != 1 || full.Trace.Retrieved[1].Rank != 2 || full.Trace.Retrieved[0].Score != 0.9 {
		t.Errorf("got recuperados %+v", full.Trace.Retrieved)
	}
}

// Una pregunta repetida no vuelve a buscar: no gasta otra petición del
// proveedor de embeddings.
func TestAnswerCachesRepeatedQuestions(t *testing.T) {
	f := newFixture(0, passage("Considerando 1", 1, 1, 0.7, "texto"))

	first := ask(t, f.service, orchestrator.Query{Question: "¿Qué pozos se monitorean?"})
	second := ask(t, f.service, orchestrator.Query{Question: "  ¿qué  POZOS se monitorean? "})

	if first.Trace.Cache != orchestrator.CacheMiss || second.Trace.Cache != orchestrator.CacheHit {
		t.Errorf("got caché %q y %q, want miss y hit", first.Trace.Cache, second.Trace.Cache)
	}
	if f.retriever.count() != 1 {
		t.Errorf("got %d búsquedas, want 1", f.retriever.count())
	}
	if second.Citations[0].ChunkID != first.Citations[0].ChunkID {
		t.Error("la respuesta desde la caché trae otras citas")
	}
	// Cada respuesta es una consulta distinta, con su propia traza.
	if second.ID == first.ID || second.Trace.ID == first.Trace.ID {
		t.Error("la respuesta desde la caché repite los identificadores")
	}

	// Otra colección u otra cantidad de pasajes es otra búsqueda.
	ask(t, f.service, orchestrator.Query{Question: "¿Qué pozos se monitorean?", Collection: "rca"})
	ask(t, f.service, orchestrator.Query{Question: "¿Qué pozos se monitorean?", TopK: 3})
	if f.retriever.count() != 3 {
		t.Errorf("got %d búsquedas, want 3", f.retriever.count())
	}
}

// La caché vence: un documento ingerido después debe poder aparecer.
func TestAnswerCacheExpires(t *testing.T) {
	f := newFixture(0, passage("", 1, 1, 0.7, "texto"))
	q := orchestrator.Query{Question: "¿Qué pozos se monitorean?"}

	ask(t, f.service, q)
	*f.clock = f.clock.Add(9 * time.Minute)
	if got := ask(t, f.service, q); got.Trace.Cache != orchestrator.CacheHit {
		t.Errorf("a los 9 minutos: got %q, want hit", got.Trace.Cache)
	}
	*f.clock = f.clock.Add(2 * time.Minute)
	if got := ask(t, f.service, q); got.Trace.Cache != orchestrator.CacheMiss {
		t.Errorf("a los 11 minutos: got %q, want miss", got.Trace.Cache)
	}
}

// Cada consulta nueva gasta cuota del proveedor de embeddings. Alcanzado el
// tope del día, las nuevas se rechazan y las ya respondidas siguen sirviendo.
func TestAnswerEnforcesDailyBudget(t *testing.T) {
	f := newFixture(2, passage("", 1, 1, 0.7, "texto"))
	ctx := context.Background()

	ask(t, f.service, orchestrator.Query{Question: "pregunta uno"})
	ask(t, f.service, orchestrator.Query{Question: "pregunta dos"})

	if _, err := f.service.Answer(ctx, orchestrator.Query{Question: "pregunta tres"}); !errors.Is(err, orchestrator.ErrQuotaExhausted) {
		t.Fatalf("got %v, want ErrQuotaExhausted", err)
	}
	if f.retriever.count() != 2 {
		t.Errorf("la consulta rechazada llegó a buscar: %d búsquedas", f.retriever.count())
	}
	if got := ask(t, f.service, orchestrator.Query{Question: "pregunta uno"}); got.Trace.Cache != orchestrator.CacheHit {
		t.Errorf("una pregunta ya respondida debe seguir sirviendo desde la caché: %q", got.Trace.Cache)
	}

	// Al día siguiente el tope se renueva.
	*f.clock = f.clock.Add(24 * time.Hour)
	ask(t, f.service, orchestrator.Query{Question: "pregunta tres"})
}

func TestAnswerTranslatesRetrievalErrors(t *testing.T) {
	for name, tt := range map[string]struct {
		cause error
		want  error
	}{
		"corpus no disponible": {fmt.Errorf("%w: el proveedor de embeddings no responde", rag.ErrUnavailable), orchestrator.ErrUnavailable},
		"consulta rechazada":   {fmt.Errorf("%w: el proveedor de embeddings rechazó la consulta", rag.ErrInvalidInput), orchestrator.ErrInvalidInput},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(0)
			f.retriever.err = tt.cause

			_, err := f.service.Answer(context.Background(), orchestrator.Query{Question: "¿Qué pozos?"})

			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}

	// Un error no previsto se propaga tal cual: lo convierte en 500 quien
	// responde la petición.
	f := newFixture(0)
	f.retriever.err = errors.New("la base de datos no responde")
	_, err := f.service.Answer(context.Background(), orchestrator.Query{Question: "¿Qué pozos?"})
	if err == nil || errors.Is(err, orchestrator.ErrUnavailable) || errors.Is(err, orchestrator.ErrInvalidInput) {
		t.Errorf("got %v, want el error original", err)
	}
}

// Una búsqueda fallida no queda en la caché: la siguiente vuelve a intentar.
func TestAnswerDoesNotCacheFailures(t *testing.T) {
	f := newFixture(0, passage("", 1, 1, 0.7, "texto"))
	q := orchestrator.Query{Question: "¿Qué pozos?"}
	f.retriever.err = rag.ErrUnavailable

	if _, err := f.service.Answer(context.Background(), q); err == nil {
		t.Fatal("se esperaba un error")
	}
	f.retriever.err = nil
	if got := ask(t, f.service, q); got.Trace.Cache != orchestrator.CacheMiss || len(got.Citations) != 1 {
		t.Errorf("tras la falla: caché %q con %d citas, want miss con 1", got.Trace.Cache, len(got.Citations))
	}
}

func TestExcerptIsShortenedBetweenWords(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("El Titular deberá mantener la medición de la calidad del agua subterránea. ", 40))
	f := newFixture(0, passage("", 1, 1, 0.7, long))

	got := ask(t, f.service, orchestrator.Query{Question: "¿Qué pozos?"}).Citations[0].Excerpt

	if n := utf8.RuneCountInString(got); n > 701 {
		t.Errorf("el extracto tiene %d caracteres", n)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("el extracto no indica que el texto sigue: %q", got[len(got)-30:])
	}
	// Corta entre palabras: lo que queda es un prefijo exacto del original,
	// y lo que sigue en el original empieza con un espacio.
	body := strings.TrimSuffix(got, "…")
	if !strings.HasPrefix(long, body) || long[len(body)] != ' ' {
		t.Errorf("el extracto corta una palabra: %q", body[len(body)-30:])
	}
}

// La búsqueda siempre devuelve lo más parecido, aunque no tenga relación con
// la pregunta. Un pasaje bajo el puntaje mínimo no sustenta nada y no se
// cita; si ninguno lo alcanza, la respuesta dice que no hay sustento.
func TestAnswerDropsPassagesBelowMinimumScore(t *testing.T) {
	relevant := passage("Considerando 10.1", 89, 90, 0.76, "pertinente")
	borderline := passage("Considerando 10.2", 91, 91, 0.60, "en el límite")
	unrelated := passage("Considerando 4.3", 12, 12, 0.59, "sin relación")
	f := newFixture(0, relevant, borderline, unrelated)

	answer := ask(t, f.service, orchestrator.Query{Question: "¿Cómo se monitorea el aire?", Trace: orchestrator.TraceFull})

	if len(answer.Citations) != 2 || answer.Citations[1].ChunkID != borderline.ChunkID {
		t.Fatalf("got %d citas, want las dos que alcanzan el mínimo", len(answer.Citations))
	}
	if !strings.Contains(answer.Text, "2 pasajes") {
		t.Errorf("la respuesta cuenta pasajes que no cita: %q", answer.Text)
	}
	// La traza completa conserva lo descartado, para poder explicarlo.
	if len(answer.Trace.Retrieved) != 3 || answer.Trace.Retrieved[2].Score != 0.59 {
		t.Errorf("got recuperados %+v, want los tres", answer.Trace.Retrieved)
	}
}

func TestAnswerHasNoCitationsWhenNothingIsRelevant(t *testing.T) {
	// Los puntajes que obtuvo esta pregunta contra la RCA de prueba.
	f := newFixture(0, passage("", 3, 3, 0.542, "a"), passage("", 4, 4, 0.534, "b"))

	answer := ask(t, f.service, orchestrator.Query{Question: "¿Cuál es la receta del pisco sour?"})

	if len(answer.Citations) != 0 {
		t.Errorf("got %d citas para una pregunta ajena al corpus", len(answer.Citations))
	}
	if !strings.Contains(answer.Text, "No se encontraron pasajes") {
		t.Errorf("got respuesta %q", answer.Text)
	}
}

// Los filtros llegan a la búsqueda, y la misma pregunta con otros filtros
// es otra consulta: no sale de la caché.
func TestAnswerAppliesFilters(t *testing.T) {
	f := newFixture(0, passage("Considerando 10.1", 89, 90, 0.82, "Plan de seguimiento."))
	q := orchestrator.Query{Question: "¿Qué plan de seguimiento hay?", Filters: map[string]any{"tipo": "RCA"}}

	ask(t, f.service, q)
	ask(t, f.service, q)
	q.Filters = map[string]any{"tipo": "ICSARA"}
	ask(t, f.service, q)
	q.Filters = nil
	ask(t, f.service, q)

	if len(f.retriever.searches) != 3 {
		t.Fatalf("got %d búsquedas, want 3: la repetida sale de la caché", len(f.retriever.searches))
	}
	if got := f.retriever.searches[0].opts.Filters; got["tipo"] != "RCA" {
		t.Errorf("got filtros %v, want tipo RCA", got)
	}
	if got := f.retriever.searches[2].opts.Filters; len(got) != 0 {
		t.Errorf("got filtros %v, want ninguno", got)
	}
}

func TestQueryRejectsCompositeFilters(t *testing.T) {
	for name, value := range map[string]any{
		"objeto": map[string]any{"es": "RCA"},
		"lista":  []any{"RCA", "ICSARA"},
		"nulo":   nil,
	} {
		_, err := orchestrator.Query{Question: "¿Qué pozos?", Filters: map[string]any{"tipo": value}}.Normalize()
		if !errors.Is(err, orchestrator.ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", name, err)
		}
	}
	if _, err := (orchestrator.Query{Question: "¿Qué pozos?", Filters: map[string]any{"tipo": "RCA", "año": 2026.0, "vigente": true}}).Normalize(); err != nil {
		t.Errorf("filtros simples: %v", err)
	}
}
