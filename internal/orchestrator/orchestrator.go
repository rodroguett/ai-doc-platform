// Package orchestrator responde consultas sobre el corpus: recupera los
// pasajes que sustentan una respuesta y los devuelve como citas.
//
// El paquete raíz es el contrato del módulo (ADR-0005): los tipos de una
// consulta y de su respuesta, y los errores que quien llama puede distinguir.
package orchestrator

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	// ErrInvalidInput indica que la consulta no cumple las reglas del
	// dominio. Los errores que lo envuelven llevan un mensaje apto para el
	// cliente.
	ErrInvalidInput = errors.New("entrada inválida")

	// ErrUnavailable indica que una dependencia externa no respondió. La
	// misma consulta puede funcionar más tarde.
	ErrUnavailable = errors.New("no disponible por ahora")

	// ErrQuotaExhausted indica que se alcanzó el tope diario de consultas
	// nuevas. Las ya respondidas siguen disponibles desde la caché.
	ErrQuotaExhausted = errors.New("tope diario de consultas alcanzado")
)

// Límites de una consulta. Coinciden con los del contrato HTTP, pero se
// validan aquí porque son reglas del dominio: una pregunta vacía no tiene
// respuesta posible, llegue por HTTP o por cualquier otro camino.
const (
	minQuestionLen = 3
	maxQuestionLen = 2000
	defaultTopK    = 5
	maxTopK        = 20
)

// TraceLevel indica cuánto detalle de la ejecución se devuelve.
type TraceLevel string

const (
	TraceSummary TraceLevel = "summary"
	TraceFull    TraceLevel = "full"
)

// CacheStatus indica si la respuesta provino de la caché.
type CacheStatus string

const (
	CacheHit    CacheStatus = "hit"
	CacheMiss   CacheStatus = "miss"
	CacheBypass CacheStatus = "bypass"
)

// Query es una pregunta sobre el corpus.
type Query struct {
	Question   string
	Collection string
	TopK       int
	Filters    map[string]any
	Trace      TraceLevel
}

// Normalize aplica los valores por omisión y valida la consulta.
func (q Query) Normalize() (Query, error) {
	q.Question = strings.TrimSpace(q.Question)
	if n := utf8.RuneCountInString(q.Question); n < minQuestionLen || n > maxQuestionLen {
		return Query{}, fmt.Errorf("%w: la pregunta debe tener entre %d y %d caracteres",
			ErrInvalidInput, minQuestionLen, maxQuestionLen)
	}
	q.Collection = strings.TrimSpace(q.Collection)

	if q.TopK == 0 {
		q.TopK = defaultTopK
	}
	if q.TopK < 1 || q.TopK > maxTopK {
		return Query{}, fmt.Errorf("%w: top_k debe estar entre 1 y %d", ErrInvalidInput, maxTopK)
	}

	switch q.Trace {
	case "":
		q.Trace = TraceSummary
	case TraceSummary, TraceFull:
	default:
		return Query{}, fmt.Errorf("%w: nivel de traza desconocido %q", ErrInvalidInput, q.Trace)
	}

	return q, nil
}

// Answer es la respuesta a una consulta, con las fuentes que la sustentan y
// la traza de cómo se produjo.
type Answer struct {
	ID        uuid.UUID
	Text      string
	Citations []Citation
	Trace     Trace
}

// Citation identifica el pasaje de un documento que sustenta la respuesta.
type Citation struct {
	ChunkID    uuid.UUID
	DocumentID uuid.UUID
	Source     string
	Locator    string
	Score      float64
	Excerpt    string
}

// Trace describe cómo se produjo una respuesta.
type Trace struct {
	ID       string
	Provider string
	Model    string
	Cache    CacheStatus

	// Degraded indica que había pasajes para responder pero ningún modelo
	// redactó la respuesta: se entregan los pasajes sin texto generado.
	Degraded bool

	Tokens TokenUsage

	// EstimatedCostUSD es lo que habría costado la consulta con la tarifa
	// de lista del modelo (ADR-0013); no es lo que se paga. Es nil si el
	// modelo no tiene tarifa conocida.
	EstimatedCostUSD *float64

	Latency   Latency
	Retrieved []RetrievedChunk // solo con TraceFull
}

// TokenUsage es el consumo de tokens de la generación.
type TokenUsage struct {
	Prompt     int
	Completion int
}

// Latency desglosa el tiempo de la consulta por etapa.
type Latency struct {
	Total      time.Duration
	Retrieval  time.Duration
	Generation time.Duration
}

// Outcome es cómo terminó una consulta atendida.
type Outcome string

const (
	// OutcomeAnswered: un modelo redactó la respuesta y sus citas pasaron
	// la verificación.
	OutcomeAnswered Outcome = "answered"

	// OutcomeNotAnswered: el modelo respondió sin citar, casi siempre
	// porque los pasajes no responden la pregunta.
	OutcomeNotAnswered Outcome = "not_answered"

	// OutcomeDegraded: ningún modelo redactó una respuesta utilizable.
	OutcomeDegraded Outcome = "degraded"

	// OutcomeNoSources: ningún pasaje alcanzó el puntaje mínimo.
	OutcomeNoSources Outcome = "no_sources"
)

// Usage es el registro de una consulta atendida (ADR-0013). No contiene la
// pregunta, la respuesta ni los pasajes.
type Usage struct {
	At         time.Time
	TraceID    string
	Collection string

	// Provider y Model son los del modelo que respondió, aunque su texto se
	// haya descartado, o "none" si ninguno respondió.
	Provider string
	Model    string

	PromptTokens int
	OutputTokens int

	Cache   CacheStatus
	Outcome Outcome

	// EstimatedCostUSD es nil si el modelo no tiene tarifa conocida.
	EstimatedCostUSD *float64
}

// RetrievedChunk es un pasaje recuperado, se haya citado o no.
type RetrievedChunk struct {
	ChunkID uuid.UUID
	Score   float64
	Rank    int
}
