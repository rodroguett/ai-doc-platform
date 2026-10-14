package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
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

	// MinScore es la similitud bajo la cual un pasaje no se considera
	// sustento de una respuesta. La búsqueda siempre devuelve "lo más
	// parecido", aunque no tenga relación con la pregunta: sin este piso,
	// una pregunta ajena al corpus recibiría citas.
	//
	// El valor queda en medio de la brecha medida con el modelo de ADR-0008
	// sobre la RCA de prueba completa: en siete preguntas que el documento
	// responde, ningún pasaje entre los cuatro primeros bajó de 0,659; en
	// dos ajenas al documento, ninguno pasó de 0,542. Son nueve preguntas:
	// alcanza para ubicar el piso, no para darlo por calibrado (ADR-0011).
	MinScore = 0.60

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

	// Generator redacta las respuestas. Puede faltar: las consultas se
	// responden entonces en modo degradado, con los pasajes.
	Generator llmgw.Generator

	// DailyBudget es el tope diario de consultas nuevas. Cero usa el valor
	// por omisión.
	DailyBudget int

	// Now permite a los tests controlar el reloj.
	Now func() time.Time
}

// Service responde consultas. Es seguro para uso concurrente.
type Service struct {
	retriever Retriever
	generator llmgw.Generator
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

	// draft es la respuesta redactada para estos pasajes, si hubo una. Una
	// redacción fallida no se guarda: la siguiente consulta vuelve a
	// intentarlo.
	draft *draft
}

func NewService(cfg Config) *Service {
	s := &Service{
		retriever: cfg.Retriever,
		generator: cfg.Generator,
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

// Answer responde una consulta.
//
// Recupera los pasajes del corpus que la sustentan y le pide a un modelo que
// redacte la respuesta a partir de ellos. La respuesta redactada se entrega
// solo si cita pasajes que existen; las citas que se devuelven son esos
// pasajes.
//
// Si no hay modelo, si el modelo no responde o si su respuesta no está
// sustentada, la consulta se responde en modo degradado: se entregan los
// pasajes sin texto redactado, y la traza lo declara. Una falla del modelo
// nunca es un error de la consulta.
func (s *Service) Answer(ctx context.Context, q Query) (Answer, error) {
	start := s.now()

	q, err := q.Normalize()
	if err != nil {
		return Answer{}, err
	}

	key := cacheKey(q)
	retrieved, written, cache, err := s.retrieve(ctx, key, q)
	if err != nil {
		return Answer{}, err
	}
	// Los pasajes llegan del más al menos parecido: los que sustentan una
	// respuesta son un prefijo.
	passages := retrieved
	for i, p := range retrieved {
		if p.Score < MinScore {
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
		ID: uuid.New(),
		Trace: Trace{
			ID:       strings.ReplaceAll(traceID.String(), "-", ""),
			Provider: noProvider,
			Model:    noProvider,
			Cache:    cache,
			Latency:  Latency{Retrieval: retrieval},
		},
	}

	// Sin pasajes no se le pide nada al modelo: no hay de qué redactar, y
	// la respuesta "no hay sustento" es definitiva, no una degradación.
	cited := passages
	switch {
	case len(passages) == 0:
		answer.Text = noSourcesAnswer

	case written != nil:
		// La misma pregunta ya se redactó hace poco: no se gasta otra
		// petición del modelo, y la traza no declara tokens que esta
		// consulta no consumió.
		answer.Text, cited = written.text, pick(passages, written.cited)
		answer.Trace.Provider, answer.Trace.Model = written.provider, written.model

	case s.generator == nil:
		answer.Text, answer.Trace.Degraded = degradedText(len(passages)), true

	default:
		generationStart := s.now()
		d, err := s.generate(ctx, q.Question, passages)
		answer.Trace.Latency.Generation = s.now().Sub(generationStart)
		switch {
		case err == nil:
			s.remember(key, &d)
			answer.Text, cited = d.text, pick(passages, d.cited)
			answer.Trace.Provider, answer.Trace.Model = d.provider, d.model
			answer.Trace.Tokens = TokenUsage{Prompt: d.promptTokens, Completion: d.outputTokens}
		case ctx.Err() != nil:
			return Answer{}, ctx.Err()
		case errors.Is(err, errNoCitations):
			// El modelo respondió, pero sin citar: casi siempre, que los
			// pasajes no responden. No se entrega su texto, que no se puede
			// verificar, pero sí se dice qué pasó.
			answer.Text, answer.Trace.Degraded = notAnsweredText(len(passages)), true
		default:
			slog.WarnContext(ctx, "la consulta se responde en modo degradado", "error", err)
			answer.Text, answer.Trace.Degraded = degradedText(len(passages)), true
		}
	}

	answer.Citations = make([]Citation, len(cited))
	for i, p := range cited {
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
	// alcanzó el puntaje mínimo o el modelo no citó: permite ver por qué
	// una respuesta trae las citas que trae.
	if q.Trace == TraceFull {
		for i, p := range retrieved {
			answer.Trace.Retrieved = append(answer.Trace.Retrieved, RetrievedChunk{ChunkID: p.ChunkID, Score: p.Score, Rank: i + 1})
		}
	}
	answer.Trace.Latency.Total = s.now().Sub(start)
	return answer, nil
}

// pick devuelve los pasajes en las posiciones dadas.
func pick(passages []rag.Passage, positions []int) []rag.Passage {
	out := make([]rag.Passage, 0, len(positions))
	for _, i := range positions {
		if i < len(passages) {
			out = append(out, passages[i])
		}
	}
	return out
}

// remember guarda la respuesta redactada junto a los pasajes de la consulta.
func (s *Service) remember(key string, d *draft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.cache[key]; ok {
		entry.draft = d
		s.cache[key] = entry
	}
}

// retrieve devuelve los pasajes de la consulta y, si la hay, la respuesta ya
// redactada para ellos, desde la caché si la misma pregunta se respondió
// hace poco. Una pregunta nueva descuenta del tope
// diario, porque gasta una petición del proveedor de embeddings.
func (s *Service) retrieve(ctx context.Context, key string, q Query) ([]rag.Passage, *draft, CacheStatus, error) {
	now := s.now()

	s.mu.Lock()
	if entry, ok := s.cache[key]; ok && now.Before(entry.expires) {
		s.mu.Unlock()
		return entry.passages, entry.draft, CacheHit, nil
	}
	if day := now.UTC().Format(time.DateOnly); day != s.spentDay {
		s.spentDay, s.spent = day, 0
	}
	if s.spent >= s.budget {
		s.mu.Unlock()
		return nil, nil, "", fmt.Errorf("%w: se atendieron las %d consultas nuevas de hoy", ErrQuotaExhausted, s.budget)
	}
	// Se descuenta antes de consultar: así varias consultas simultáneas no
	// pasan todas con el último cupo.
	s.spent++
	s.mu.Unlock()

	passages, err := s.retriever.Search(ctx, q.Question, rag.SearchOptions{Collection: q.Collection, TopK: q.TopK})
	switch {
	case err == nil:
	case errors.Is(err, rag.ErrInvalidInput):
		return nil, nil, "", fmt.Errorf("%w: %s", ErrInvalidInput, strings.TrimPrefix(err.Error(), rag.ErrInvalidInput.Error()+": "))
	case errors.Is(err, rag.ErrUnavailable):
		return nil, nil, "", fmt.Errorf("%w: no se pudo consultar el corpus", ErrUnavailable)
	default:
		return nil, nil, "", err
	}

	s.mu.Lock()
	if len(s.cache) >= maxCacheEntries {
		// Se descarta todo en lugar de llevar la cuenta de qué se usó
		// menos: a esta escala, perder la caché cuesta unas pocas consultas.
		s.cache = map[string]cached{}
	}
	s.cache[key] = cached{passages: passages, expires: now.Add(cacheTTL)}
	s.mu.Unlock()

	return passages, nil, CacheMiss, nil
}

// cacheKey identifica una consulta sin distinguir mayúsculas ni espacios de
// más: "¿Qué pozos?" y "  ¿qué  pozos? " son la misma pregunta.
func cacheKey(q Query) string {
	question := strings.Join(strings.Fields(strings.ToLower(q.Question)), " ")
	return fmt.Sprintf("%s\x00%s\x00%d", question, q.Collection, q.TopK)
}

// degradedText es la respuesta cuando ningún modelo redactó una: dice qué se
// entrega en su lugar.
// notAnsweredText es la respuesta cuando el modelo consideró que los pasajes
// no responden la pregunta.
func notAnsweredText(passages int) string {
	return fmt.Sprintf("Los pasajes recuperados no responden esta pregunta. Se muestran los %d más relacionados, por si contienen algo útil; verifica en ellos antes de concluir que el documento no trata el tema.", passages)
}

func degradedText(passages int) string {
	if passages == 1 {
		return "No hay una respuesta redactada para esta consulta. Este es el pasaje del corpus más relacionado con la pregunta, con su documento y su ubicación."
	}
	return fmt.Sprintf("No hay una respuesta redactada para esta consulta. Estos son los %d pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.", passages)
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
