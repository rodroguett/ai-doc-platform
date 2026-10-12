package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// systemPrompt son las reglas con las que el modelo redacta. Es el mismo
// texto con que se midió la elección del modelo (ADR-0012).
const systemPrompt = `Eres un asistente que responde preguntas sobre documentos regulatorios ambientales chilenos.

Reglas:
- Responde solo con lo que dicen los pasajes entregados. No uses conocimiento propio.
- Cada afirmación debe terminar con la marca del pasaje que la sustenta, entre corchetes: [1], [2]. Si la sustentan varios, ponlos todos: [1][3].
- Si los pasajes no responden la pregunta, dilo en una frase y no cites ninguno.
- Responde en español, en a lo más un párrafo breve, sin encabezados ni listas.`

const (
	// maxAnswerTokens acota el largo de la respuesta redactada: un párrafo.
	maxAnswerTokens = 1024

	// maxLoggedAnswer acota cuánto de una respuesta descartada va al log.
	maxLoggedAnswer = 400
)

var (
	// citationMark es la marca de un pasaje: [3].
	citationMark = regexp.MustCompile(`\[(\d{1,3})\]`)

	// citationGroup es la forma agrupada que algunos modelos escriben pese a
	// las instrucciones: [1, 3]. Se normaliza a [1][3].
	citationGroup = regexp.MustCompile(`\[(\d{1,3}(?:\s*[,;y]\s*\d{1,3})+)\]`)
	groupSplit    = regexp.MustCompile(`\s*[,;y]\s*`)
)

// draft es una respuesta redactada cuyas citas ya se verificaron.
type draft struct {
	// text es la respuesta, con las marcas renumeradas para que coincidan
	// con el orden de cited.
	text string

	// cited son las posiciones, dentro de los pasajes entregados al modelo,
	// de los que la respuesta cita, en orden.
	cited []int

	provider     string
	model        string
	promptTokens int
	outputTokens int
}

// errUnsupported indica que el modelo respondió, pero su respuesta no puede
// entregarse: no cita ningún pasaje, o cita uno que no existe.
var errUnsupported = errors.New("la respuesta redactada no está sustentada en los pasajes")

// generate le pide al modelo que redacte la respuesta a partir de los
// pasajes, y verifica sus citas.
func (s *Service) generate(ctx context.Context, question string, passages []rag.Passage) (draft, error) {
	generation, err := s.generator.Generate(ctx, llmgw.GenerateRequest{
		System:          systemPrompt,
		Prompt:          buildPrompt(question, passages),
		MaxOutputTokens: maxAnswerTokens,
	})
	if err != nil {
		return draft{}, err
	}

	text, cited, err := verifyCitations(generation.Text, len(passages))
	if err != nil {
		// Se registra lo que el modelo escribió: sin eso no hay cómo saber
		// por qué una consulta terminó en modo degradado.
		slog.WarnContext(ctx, "se descartó la respuesta redactada",
			"modelo", generation.Model, "motivo", err, "texto", truncate(generation.Text, maxLoggedAnswer))
		return draft{}, err
	}
	return draft{
		text:         text,
		cited:        cited,
		provider:     generation.Provider,
		model:        generation.Model,
		promptTokens: generation.PromptTokens,
		outputTokens: generation.OutputTokens,
	}, nil
}

// buildPrompt arma lo que el modelo lee: los pasajes numerados, cada uno con
// su ubicación, y la pregunta al final.
//
// La pregunta la escribe quien consulta, así que va aquí y no en las
// instrucciones. Eso no impide que alguien intente torcerlas desde la
// pregunta; lo que acota el daño es que una respuesta sin citas válidas no
// se entrega.
func buildPrompt(question string, passages []rag.Passage) string {
	var b strings.Builder
	b.WriteString("Pasajes:\n\n")
	for i, p := range passages {
		fmt.Fprintf(&b, "[%d] %s — %s\n%s\n\n", i+1, p.Source, locator(p), p.Text)
	}
	b.WriteString("Pregunta: ")
	b.WriteString(question)
	return b.String()
}

// verifyCitations comprueba que la respuesta cite pasajes, y que todos los
// que cita existan. Devuelve la respuesta con las marcas renumeradas según
// los pasajes efectivamente citados, y las posiciones de esos pasajes.
//
// Es una verificación de forma: asegura que cada marca apunta a un pasaje
// real, no que ese pasaje diga lo que la respuesta afirma. Eso no se puede
// comprobar sin otro modelo, y un modelo puede citar con holgura (ADR-0012).
//
// Una respuesta que no cita nada tampoco se entrega. Puede ser el modelo
// diciendo que los pasajes no responden la pregunta, o puede ser una
// afirmación sin sustento: desde aquí no se distinguen, y en ambos casos lo
// correcto es mostrar los pasajes en lugar del texto.
func verifyCitations(text string, passages int) (string, []int, error) {
	text = citationGroup.ReplaceAllStringFunc(text, func(group string) string {
		var out strings.Builder
		for _, n := range groupSplit.Split(strings.Trim(group, "[]"), -1) {
			out.WriteString("[" + strings.TrimSpace(n) + "]")
		}
		return out.String()
	})

	seen := map[int]bool{}
	for _, m := range citationMark.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > passages {
			return "", nil, fmt.Errorf("%w: cita el pasaje %s y se entregaron %d", errUnsupported, m[1], passages)
		}
		seen[n] = true
	}
	if len(seen) == 0 {
		return "", nil, fmt.Errorf("%w: no cita ningún pasaje", errUnsupported)
	}

	cited := make([]int, 0, len(seen))
	for n := range seen {
		cited = append(cited, n-1)
	}
	sort.Ints(cited)

	// Las citas que se devuelven son solo los pasajes citados, así que las
	// marcas se renumeran para que [1] sea la primera cita devuelta.
	renumber := make(map[int]int, len(cited))
	for position, index := range cited {
		renumber[index+1] = position + 1
	}
	text = citationMark.ReplaceAllStringFunc(text, func(mark string) string {
		n, _ := strconv.Atoi(strings.Trim(mark, "[]"))
		return "[" + strconv.Itoa(renumber[n]) + "]"
	})
	return strings.TrimSpace(text), cited, nil
}

// truncate corta s a lo más en max caracteres, sin partir uno.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
