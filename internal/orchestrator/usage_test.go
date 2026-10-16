package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
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
			passage("Considerando 10.1", 89, 90, 0.82, "Plan de seguimiento de calidad del aire."),
			passage("Considerando 8.1", 86, 86, 0.74, "Compromiso de monitoreo de calidad del aire."),
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
