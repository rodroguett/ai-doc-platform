package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// fakeRecorder anota los registros de uso, o falla.
type fakeRecorder struct {
	mu      sync.Mutex
	records []orchestrator.Usage
	err     error
	ctxErr  error // el estado del contexto al registrar

	summary      orchestrator.UsageSummary
	summaryErr   error
	since, until time.Time

	count    int
	countErr error
	counts   []time.Time // desde cuándo se pidió contar, en cada llamada
}

func (r *fakeRecorder) CountNew(_ context.Context, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts = append(r.counts, since)
	return r.count, r.countErr
}

func (r *fakeRecorder) countCalls() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Time(nil), r.counts...)
}

func (r *fakeRecorder) Summarize(_ context.Context, since, until time.Time) (orchestrator.UsageSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.since, r.until = since, until
	return r.summary, r.summaryErr
}

func (r *fakeRecorder) Record(ctx context.Context, u orchestrator.Usage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctxErr = ctx.Err()
	if r.err != nil {
		return r.err
	}
	r.records = append(r.records, u)
	return nil
}

func (r *fakeRecorder) last(t *testing.T) orchestrator.Usage {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.records) == 0 {
		t.Fatal("la consulta no se registró")
	}
	return r.records[len(r.records)-1]
}

type usageFixture struct {
	service   *orchestrator.Service
	generator *fakeGenerator
	retriever *fakeRetriever
	recorder  *fakeRecorder
	clock     time.Time
}

func newUsageFixture(text string, cost *float64) usageFixture {
	f := usageFixture{
		generator: &fakeGenerator{text: text, cost: cost},
		retriever: &fakeRetriever{passages: []rag.Passage{
			passage("Considerando 10.1", 89, 90, 0.82, "Plan de seguimiento de calidad del aire: MP-10 y MPS."),
			passage("Considerando 8.1", 86, 86, 0.74, "Compromiso de monitoreo de calidad del aire; el estanque TK-65 tiene 34 m3."),
		}},
		recorder: &fakeRecorder{},
		clock:    time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC),
	}
	f.service = orchestrator.NewService(orchestrator.Config{
		Retriever: f.retriever, Generator: f.generator, Usage: f.recorder,
		Now: func() time.Time { return f.clock },
	})
	return f
}

func costOf(v float64) *float64 { return &v }

var usageQuestion = orchestrator.Query{Question: "¿Qué se monitorea?", Collection: "rca"}

func TestAnswerRecordsUsage(t *testing.T) {
	f := newUsageFixture("Se monitorea MP-10 [1].", costOf(0.00124))

	answer := ask(t, f.service, usageQuestion)

	u := f.recorder.last(t)
	if u.TraceID != answer.Trace.ID || !u.At.Equal(f.clock) || u.Collection != "rca" {
		t.Errorf("identificación inesperada: %+v", u)
	}
	if u.Provider != "gemini" || u.Model != "modelo-de-prueba" || u.PromptTokens != 3000 || u.OutputTokens != 150 {
		t.Errorf("consumo inesperado: %+v", u)
	}
	if u.Cache != orchestrator.CacheMiss || u.Outcome != orchestrator.OutcomeAnswered {
		t.Errorf("got caché %s, desenlace %s", u.Cache, u.Outcome)
	}
	if u.EstimatedCostUSD == nil || *u.EstimatedCostUSD != 0.00124 {
		t.Errorf("got costo %v, want 0.00124", u.EstimatedCostUSD)
	}
	if c := answer.Trace.EstimatedCostUSD; c == nil || *c != 0.00124 {
		t.Errorf("la traza declara costo %v, want 0.00124", c)
	}
}

// Una respuesta repetida desde la caché no le pide nada al modelo: cuenta
// para la tasa de aciertos, con costo cero.
func TestCachedAnswerIsRecordedWithoutCost(t *testing.T) {
	f := newUsageFixture("Se monitorea MP-10 [1].", costOf(0.00124))
	ask(t, f.service, usageQuestion)

	answer := ask(t, f.service, usageQuestion)

	u := f.recorder.last(t)
	if u.Cache != orchestrator.CacheHit || u.Outcome != orchestrator.OutcomeAnswered || u.PromptTokens != 0 || u.OutputTokens != 0 {
		t.Errorf("registro inesperado: %+v", u)
	}
	if u.EstimatedCostUSD == nil || *u.EstimatedCostUSD != 0 {
		t.Errorf("got costo %v, want 0", u.EstimatedCostUSD)
	}
	if c := answer.Trace.EstimatedCostUSD; c == nil || *c != 0 {
		t.Errorf("la traza declara costo %v, want 0", c)
	}
}

// Si el modelo respondió, sus tokens se gastaron aunque su texto se
// descarte: el costo se atribuye a ese modelo.
func TestDiscardedAnswerIsRecordedWithItsCost(t *testing.T) {
	f := newUsageFixture("Los pasajes no contienen esa información.", costOf(0.00124))

	answer := ask(t, f.service, usageQuestion)

	u := f.recorder.last(t)
	if u.Outcome != orchestrator.OutcomeNotAnswered || u.Model != "modelo-de-prueba" || u.PromptTokens != 3000 {
		t.Errorf("registro inesperado: %+v", u)
	}
	if u.EstimatedCostUSD == nil || *u.EstimatedCostUSD != 0.00124 {
		t.Errorf("got costo %v, want 0.00124", u.EstimatedCostUSD)
	}
	// La traza sigue sin autor: el texto no se entregó.
	if answer.Trace.Model != "none" || answer.Trace.Tokens.Prompt != 3000 {
		t.Errorf("got traza %+v", answer.Trace)
	}
}

// Un modelo que no respondió no consumió tokens.
func TestFailedGenerationIsRecordedWithoutCost(t *testing.T) {
	f := newUsageFixture("", nil)
	f.generator.err = fmt.Errorf("%w: 429", llmgw.ErrUnavailable)

	ask(t, f.service, usageQuestion)

	u := f.recorder.last(t)
	if u.Outcome != orchestrator.OutcomeDegraded || u.Model != "none" || u.PromptTokens != 0 {
		t.Errorf("registro inesperado: %+v", u)
	}
	if u.EstimatedCostUSD == nil || *u.EstimatedCostUSD != 0 {
		t.Errorf("got costo %v, want 0", u.EstimatedCostUSD)
	}
}

func TestAnswerWithoutSourcesIsRecorded(t *testing.T) {
	f := newUsageFixture("Se monitorea MP-10 [1].", costOf(0.00124))
	f.retriever.passages = []rag.Passage{passage("Considerando 4.3", 7, 7, 0.40, "Texto ajeno.")}

	ask(t, f.service, usageQuestion)

	if u := f.recorder.last(t); u.Outcome != orchestrator.OutcomeNoSources || u.EstimatedCostUSD == nil || *u.EstimatedCostUSD != 0 {
		t.Errorf("registro inesperado: %+v", u)
	}
	if f.generator.count() != 0 {
		t.Error("sin pasajes no debía pedirse nada al modelo")
	}
}

// Un modelo sin tarifa conocida deja el costo sin estimar, no en cero.
func TestUnknownCostStaysUnknown(t *testing.T) {
	f := newUsageFixture("Se monitorea MP-10 [1].", nil)

	answer := ask(t, f.service, usageQuestion)

	if u := f.recorder.last(t); u.EstimatedCostUSD != nil {
		t.Errorf("got costo registrado %v, want sin estimar", *u.EstimatedCostUSD)
	}
	if answer.Trace.EstimatedCostUSD != nil {
		t.Errorf("la traza declara costo %v, want sin estimar", *answer.Trace.EstimatedCostUSD)
	}
}

// No poder registrar no impide responder.
func TestAnswerSurvivesRecorderFailure(t *testing.T) {
	f := newUsageFixture("Se monitorea MP-10 [1].", costOf(0.00124))
	f.recorder.err = errors.New("conexión rechazada")

	answer := ask(t, f.service, usageQuestion)

	if answer.Text != "Se monitorea MP-10 [1]." {
		t.Errorf("got respuesta %q", answer.Text)
	}
}

// Quien consulta puede cortar la conexión justo cuando llega la respuesta,
// pero la consulta ya se atendió y gastó lo que gastó: se registra igual.
func TestUsageIsRecordedAfterClientLeaves(t *testing.T) {
	f := newUsageFixture("", nil)
	gen := &cancelingGenerator{fakeGenerator: &fakeGenerator{text: "Se monitorea MP-10 [1].", cost: costOf(0.00124)}}
	service := orchestrator.NewService(orchestrator.Config{Retriever: f.retriever, Generator: gen, Usage: f.recorder})

	ctx, cancel := context.WithCancel(context.Background())
	gen.cancel = cancel
	if _, err := service.Answer(ctx, usageQuestion); err != nil {
		t.Fatalf("responder: %v", err)
	}

	f.recorder.last(t)
	if f.recorder.ctxErr != nil {
		t.Errorf("el registro heredó la cancelación: %v", f.recorder.ctxErr)
	}
}

// cancelingGenerator responde y cancela el contexto de la consulta, como un
// cliente que se va justo cuando llega la respuesta.
type cancelingGenerator struct {
	*fakeGenerator
	cancel context.CancelFunc
}

func (g *cancelingGenerator) Generate(ctx context.Context, req llmgw.GenerateRequest) (llmgw.Generation, error) {
	out, err := g.fakeGenerator.Generate(ctx, req)
	g.cancel()
	return out, err
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// Sin periodo, se informan los 30 días que terminan hoy, en días UTC.
func TestUsageDefaultsToLast30Days(t *testing.T) {
	f := newUsageFixture("", nil)
	f.recorder.summary = orchestrator.UsageSummary{Queries: 7}

	got, err := f.service.Usage(context.Background(), orchestrator.UsagePeriod{})
	if err != nil {
		t.Fatalf("informar: %v", err)
	}

	// El reloj marca el 15 de octubre a las 21:30 UTC.
	if !got.From.Equal(date(2026, 9, 16)) || !got.To.Equal(date(2026, 10, 15)) || got.Queries != 7 {
		t.Errorf("got %v a %v con %d consultas", got.From, got.To, got.Queries)
	}
	// El último día se incluye entero.
	if !f.recorder.since.Equal(date(2026, 9, 16)) || !f.recorder.until.Equal(date(2026, 10, 16)) {
		t.Errorf("se sumó de %v a %v", f.recorder.since, f.recorder.until)
	}
}

func TestUsageOfExplicitPeriod(t *testing.T) {
	f := newUsageFixture("", nil)

	got, err := f.service.Usage(context.Background(), orchestrator.UsagePeriod{From: date(2026, 10, 1), To: date(2026, 10, 1)})
	if err != nil {
		t.Fatalf("informar: %v", err)
	}
	if !got.From.Equal(date(2026, 10, 1)) || !got.To.Equal(date(2026, 10, 1)) || !f.recorder.until.Equal(date(2026, 10, 2)) {
		t.Errorf("got %v a %v, sumado hasta %v", got.From, got.To, f.recorder.until)
	}
}

func TestUsageRejectsInvalidPeriods(t *testing.T) {
	f := newUsageFixture("", nil)
	for name, p := range map[string]orchestrator.UsagePeriod{
		"desde después de hasta": {From: date(2026, 10, 2), To: date(2026, 10, 1)},
		"más de 366 días":        {From: date(2025, 10, 14), To: date(2026, 10, 15)},
	} {
		if _, err := f.service.Usage(context.Background(), p); !errors.Is(err, orchestrator.ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", name, err)
		}
	}
	// 366 días exactos se aceptan.
	if _, err := f.service.Usage(context.Background(), orchestrator.UsagePeriod{From: date(2025, 10, 15), To: date(2026, 10, 15)}); err != nil {
		t.Errorf("366 días: %v", err)
	}
}

func TestUsageUnavailable(t *testing.T) {
	// Sin base de datos no hay registro que leer.
	noStore := orchestrator.NewService(orchestrator.Config{Retriever: &fakeRetriever{}})
	if _, err := noStore.Usage(context.Background(), orchestrator.UsagePeriod{}); !errors.Is(err, orchestrator.ErrUnavailable) {
		t.Errorf("sin registro: got %v, want ErrUnavailable", err)
	}

	// Una falla de la base de datos no expone su detalle.
	f := newUsageFixture("", nil)
	f.recorder.summaryErr = errors.New("dial tcp 10.0.0.7:5432: conexión rechazada")
	_, err := f.service.Usage(context.Background(), orchestrator.UsagePeriod{})
	if !errors.Is(err, orchestrator.ErrUnavailable) || strings.Contains(err.Error(), "10.0.0.7") {
		t.Errorf("got %v, want ErrUnavailable sin el detalle", err)
	}
}

// newBudgetFixture arma un servicio con tope de budget consultas nuevas por
// día, cuyo registro de uso ya cuenta registered para hoy.
func newBudgetFixture(budget, registered int) (*orchestrator.Service, *fakeRecorder, *time.Time) {
	f := newUsageFixture("Se monitorea MP-10 [1].", costOf(0.001))
	f.recorder.count = registered
	clock := f.clock
	s := orchestrator.NewService(orchestrator.Config{
		Retriever: f.retriever, Generator: f.generator, Usage: f.recorder, DailyBudget: budget,
		Now: func() time.Time { return clock },
	})
	return s, f.recorder, &clock
}

func question(i int) orchestrator.Query {
	return orchestrator.Query{Question: fmt.Sprintf("¿Pregunta número %d?", i)}
}

// Un reinicio de la instancia no reinicia el tope: el conteo del día se lee
// del registro de uso.
func TestDailyBudgetSurvivesRestart(t *testing.T) {
	s, _, _ := newBudgetFixture(3, 2)

	ask(t, s, question(1))
	_, err := s.Answer(context.Background(), question(2))

	if !errors.Is(err, orchestrator.ErrQuotaExhausted) {
		t.Fatalf("got %v, want ErrQuotaExhausted: el registro ya contaba 2 de 3", err)
	}
}

// El registro se lee una vez por día, no en cada pregunta.
func TestDailyCountIsLoadedOncePerDay(t *testing.T) {
	s, recorder, clock := newBudgetFixture(100, 0)

	for i := range 3 {
		ask(t, s, question(i))
	}
	if calls := recorder.countCalls(); len(calls) != 1 || !calls[0].Equal(date(2026, 10, 15)) {
		t.Fatalf("got lecturas %v, want una, desde la medianoche UTC", calls)
	}

	*clock = clock.Add(24 * time.Hour)
	ask(t, s, question(10))
	if calls := recorder.countCalls(); len(calls) != 2 || !calls[1].Equal(date(2026, 10, 16)) {
		t.Errorf("got lecturas %v, want una más al cambiar el día", calls)
	}
}

// Las preguntas repetidas salen de la caché y no gastan cupo ni leen el
// registro.
func TestCachedAnswersDoNotSpendBudget(t *testing.T) {
	s, recorder, _ := newBudgetFixture(1, 0)

	ask(t, s, question(1))
	ask(t, s, question(1))

	if calls := recorder.countCalls(); len(calls) != 1 {
		t.Errorf("got %d lecturas del registro, want 1", len(calls))
	}
}

// Si el registro no responde, el tope sigue en memoria y la lectura se
// reintenta en la consulta siguiente.
func TestDailyBudgetFallsBackToMemory(t *testing.T) {
	s, recorder, _ := newBudgetFixture(2, 0)
	recorder.countErr = errors.New("conexión rechazada")

	ask(t, s, question(1))
	if calls := recorder.countCalls(); len(calls) != 1 {
		t.Fatalf("got %d lecturas, want 1", len(calls))
	}

	// El registro vuelve, y dice que el día ya tenía una consulta además de
	// la que esta instancia atendió: se toma el mayor.
	recorder.mu.Lock()
	recorder.countErr, recorder.count = nil, 2
	recorder.mu.Unlock()
	_, err := s.Answer(context.Background(), question(2))

	if !errors.Is(err, orchestrator.ErrQuotaExhausted) {
		t.Errorf("got %v, want ErrQuotaExhausted", err)
	}
	if calls := recorder.countCalls(); len(calls) != 2 {
		t.Errorf("got %d lecturas, want 2: la segunda reintenta", len(calls))
	}
}

// Lo atendido mientras el registro no respondía no se olvida cuando vuelve:
// las consultas en curso o no registradas siguen contando.
func TestDailyCountKeepsWhatMemorySpent(t *testing.T) {
	s, recorder, _ := newBudgetFixture(2, 0)
	recorder.countErr = errors.New("conexión rechazada")
	ask(t, s, question(1))

	// El registro vuelve sin la consulta anterior, que no alcanzó a
	// registrarse.
	recorder.mu.Lock()
	recorder.countErr = nil
	recorder.mu.Unlock()
	ask(t, s, question(2))
	_, err := s.Answer(context.Background(), question(3))

	if !errors.Is(err, orchestrator.ErrQuotaExhausted) {
		t.Errorf("got %v, want ErrQuotaExhausted: esta instancia ya atendió 2", err)
	}
}
