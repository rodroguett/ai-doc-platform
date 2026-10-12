package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

const (
	// noProvider es lo que la traza declara como proveedor y modelo cuando
	// ningún modelo redactó la respuesta.
	noProvider = "none"

	// defaultDailyBudget es cuántas consultas nuevas se atienden por día.
	// Cada una gasta una petición de embeddings, de las mil diarias de la
	// capa gratuita, que comparte con la ingesta (ADR-0008).
	defaultDailyBudget = 500

	// La caché guarda lo recuperado para una pregunta. Vence pronto porque
	// el corpus cambia: un documento recién ingerido debe poder aparecer.
	cacheTTL        = 10 * time.Minute
	maxCacheEntries = 500

	// minScore es la similitud bajo la cual un pasaje no se considera
	// sustento de una respuesta. La búsqueda siempre devuelve "lo más
	// parecido", aunque no tenga relación con la pregunta: sin este piso,
	// una pregunta ajena al corpus recibiría citas.
	//
	// El valor es provisional. Sale de una sola medición con el modelo de
	// ADR-0008: los pasajes pertinentes puntuaron entre 0,63 y 0,76, y los
	// de una pregunta ajena al documento, 0,51. Debe calibrarse con el
	// conjunto de preguntas de evaluación.
	minScore = 0.55

	// maxExcerptChars es el largo del extracto de una cita.
	maxExcerptChars = 700

	noSourcesAnswer = "No se encontraron pasajes en el corpus que sustenten una respuesta a esta pregunta."
)

// Retriever recupera los pasajes del corpus más parecidos a una consulta.
// Lo implementa rag.
type Retriever interface {
	Search(ctx context.Context, query string, opts rag.SearchOptions) ([]rag.Passage, error)
}

// Config configura el servicio.
type Config struct {
	Retriever Retriever

	// DailyBudget es el tope diario de consultas nuevas. Cero usa el valor
	// por omisión.
	DailyBudget int

	// Now permite a los tests controlar el reloj.
	Now func() time.Time
}

// Service responde consultas. Es seguro para uso concurrente.
type Service struct {
	retriever Retriever
	budget    int
	now       func() time.Time

	mu       sync.Mutex
	cache    map[string]cached
	spentDay string
	spent    int
}

type cached struct {
	passages []rag.Passage
	expires  time.Time
}

func NewService(cfg Config) *Service {
	s := &Service{
		retriever: cfg.Retriever,
		budget:    cfg.DailyBudget,
		now:       cfg.Now,
		cache:     map[string]cached{},
	}
	if s.budget <= 0 {
		s.budget = defaultDailyBudget
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// Answer responde una consulta con los pasajes del corpus que mejor se
// relacionan con ella.
//
// Todavía ningún modelo redacta la respuesta: se entregan los pasajes como
// citas, y la traza lo declara como modo degradado. Es el mismo camino que
// seguirá una consulta cuando el proveedor de generación no esté disponible.
func (s *Service) Answer(ctx context.Context, q Query) (Answer, error) {
	start := s.now()

	q, err := q.Normalize()
	if err != nil {
		return Answer{}, err
	}

	retrieved, cache, err := s.retrieve(ctx, q)
	if err != nil {
		return Answer{}, err
	}
	// Los pasajes llegan del más al menos parecido: los que sustentan una
	// respuesta son un prefijo.
	passages := retrieved
	for i, p := range retrieved {
		if p.Score < minScore {
			passages = retrieved[:i]
			break
		}
	}
	retrieval := s.now().Sub(start)

	traceID, err := uuid.NewRandom()
	if err != nil {
		return Answer{}, fmt.Errorf("generar el identificador de la traza: %w", err)
	}
	answer := Answer{
		ID:        uuid.New(),
		Text:      composeText(len(passages)),
		Citations: make([]Citation, len(passages)),
		Trace: Trace{
			ID:       strings.ReplaceAll(traceID.String(), "-", ""),
			Provider: noProvider,
			Model:    noProvider,
			Cache:    cache,
			Degraded: true,
			Latency:  Latency{Retrieval: retrieval},
		},
	}
	for i, p := range passages {
		answer.Citations[i] = Citation{
			ChunkID:    p.ChunkID,
			DocumentID: p.DocumentID,
			Source:     p.Source,
			Locator:    locator(p),
			Score:      p.Score,
			Excerpt:    excerpt(p.Text),
		}
	}
	// La traza completa muestra todo lo recuperado, también lo que no
	// alcanzó el puntaje mínimo: permite ver por qué una pregunta quedó sin
	// citas.
	if q.Trace == TraceFull {
		for i, p := range retrieved {
			answer.Trace.Retrieved = append(answer.Trace.Retrieved, RetrievedChunk{ChunkID: p.ChunkID, Score: p.Score, Rank: i + 1})
		}
	}
	answer.Trace.Latency.Total = s.now().Sub(start)
	return answer, nil
}

// retrieve devuelve los pasajes de la consulta, desde la caché si la misma
// pregunta se respondió hace poco. Una pregunta nueva descuenta del tope
// diario, porque gasta una petición del proveedor de embeddings.
func (s *Service) retrieve(ctx context.Context, q Query) ([]rag.Passage, CacheStatus, error) {
	key := cacheKey(q)
	now := s.now()

	s.mu.Lock()
	if entry, ok := s.cache[key]; ok && now.Before(entry.expires) {
		s.mu.Unlock()
		return entry.passages, CacheHit, nil
	}
	if day := now.UTC().Format(time.DateOnly); day != s.spentDay {
		s.spentDay, s.spent = day, 0
	}
	if s.spent >= s.budget {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("%w: se atendieron las %d consultas nuevas de hoy", ErrQuotaExhausted, s.budget)
	}
	// Se descuenta antes de consultar: así varias consultas simultáneas no
	// pasan todas con el último cupo.
	s.spent++
	s.mu.Unlock()

	passages, err := s.retriever.Search(ctx, q.Question, rag.SearchOptions{Collection: q.Collection, TopK: q.TopK})
	switch {
	case err == nil:
	case errors.Is(err, rag.ErrInvalidInput):
		return nil, "", fmt.Errorf("%w: %s", ErrInvalidInput, strings.TrimPrefix(err.Error(), rag.ErrInvalidInput.Error()+": "))
	case errors.Is(err, rag.ErrUnavailable):
		return nil, "", fmt.Errorf("%w: no se pudo consultar el corpus", ErrUnavailable)
	default:
		return nil, "", err
	}

	s.mu.Lock()
	if len(s.cache) >= maxCacheEntries {
		// Se descarta todo en lugar de llevar la cuenta de qué se usó
		// menos: a esta escala, perder la caché cuesta unas pocas consultas.
		s.cache = map[string]cached{}
	}
	s.cache[key] = cached{passages: passages, expires: now.Add(cacheTTL)}
	s.mu.Unlock()

	return passages, CacheMiss, nil
}

// cacheKey identifica una consulta sin distinguir mayúsculas ni espacios de
// más: "¿Qué pozos?" y "  ¿qué  pozos? " son la misma pregunta.
func cacheKey(q Query) string {
	question := strings.Join(strings.Fields(strings.ToLower(q.Question)), " ")
	return fmt.Sprintf("%s\x00%s\x00%d", question, q.Collection, q.TopK)
}

func composeText(passages int) string {
	switch passages {
	case 0:
		return noSourcesAnswer
	case 1:
		return "Todavía no hay una respuesta redactada. Este es el pasaje del corpus más relacionado con la pregunta, con su documento y su ubicación."
	default:
		return fmt.Sprintf("Todavía no hay una respuesta redactada. Estos son los %d pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.", passages)
	}
}

// locator dice dónde está el pasaje dentro de su documento, como lo buscaría
// una persona: "Considerando 10.1, págs. 89-90".
func locator(p rag.Passage) string {
	pages := fmt.Sprintf("pág. %d", p.PageStart)
	if p.PageEnd > p.PageStart {
		pages = fmt.Sprintf("págs. %d-%d", p.PageStart, p.PageEnd)
	}
	if p.Section == "" {
		return pages
	}
	return p.Section + ", " + pages
}

// excerpt acorta el texto de un pasaje para mostrarlo en una cita. Corta
// entre palabras y marca que el texto sigue.
func excerpt(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= maxExcerptChars {
		return text
	}
	cut := maxExcerptChars
	for cut > maxExcerptChars/2 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	return strings.TrimRightFunc(string(runes[:cut]), unicode.IsSpace) + "…"
}
