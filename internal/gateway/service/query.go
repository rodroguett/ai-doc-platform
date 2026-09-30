package service

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Límites de una consulta. Coinciden con los declarados en el contrato, pero
// se validan aquí porque son reglas del dominio: una pregunta vacía no tiene
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

// Citation identifica el fragmento de un documento que sustenta la respuesta.
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
	ID        string
	Provider  string
	Model     string
	Cache     CacheStatus
	Degraded  bool
	Tokens    TokenUsage
	CostUSD   float64
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

// RetrievedChunk es un fragmento recuperado, se haya citado o no.
type RetrievedChunk struct {
	ChunkID uuid.UUID
	Score   float64
	Rank    int
}
