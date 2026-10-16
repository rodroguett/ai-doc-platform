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
	"unicode"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// systemPrompt son las reglas con las que el modelo redacta.
//
// Las primeras versiones pedían, ante una pregunta que los pasajes no
// responden, decirlo y no citar nada. La evaluación mostró el costo: ante un
// "¿por qué?" que el documento no explica, el modelo lo decía y callaba lo
// que los pasajes sí contenían, y la respuesta se descartaba por no citar.
// Estas reglas le piden separar lo que no se responde de lo que sí.
//
// Pedirlo trajo un efecto que también midió la evaluación: ante una pregunta
// sin respuesta, el modelo decía "los pasajes no contienen esa información"
// y le ponía marcas a esa frase, de modo que la respuesta pasaba la
// verificación con citas que no sustentaban nada. Por eso las marcas quedan
// reservadas a lo que los pasajes sí dicen.
//
// En una enumeración, el modelo agregaba a veces un pasaje parecido de otra
// lista: la charla de difusión, que es monitoreo participativo, entre los
// compromisos voluntarios. La cita existía, así que la verificación no podía
// detectarlo; el contexto de cada pasaje es lo que permite distinguirlos.
const systemPrompt = `Eres un asistente que responde preguntas sobre documentos regulatorios ambientales chilenos.

Reglas:
- Responde solo con lo que dicen los pasajes entregados. No uses conocimiento propio.
- Cada afirmación sobre lo que dicen los pasajes debe terminar con la marca del pasaje que la sustenta, entre corchetes: [1], [2]. Si la sustentan varios, ponlos todos: [1][3].
- Una frase que dice que los pasajes no contienen algo no lleva marcas: una marca indica dónde está lo que se afirma, y ahí no hay nada que señalar.
- Si la pregunta da por cierto algo que los pasajes no confirman, no lo repitas como cierto: di lo que los pasajes efectivamente dicen sobre eso, con sus marcas.
- Si los pasajes responden solo una parte de la pregunta, por ejemplo qué se exige pero no por qué, di qué parte no responden y resume, con sus marcas, lo que sí dicen sobre el tema.
- Si la pregunta pide una enumeración, incluye todos los elementos que aparezcan en los pasajes y pertenezcan a lo preguntado. Un pasaje que "forma parte de" otra lista no es un elemento de la enumeración aunque se le parezca: no lo incluyas.
- Solo si los pasajes no tienen relación alguna con la pregunta, dilo en una frase y no cites ninguno.
- Responde en español, en un párrafo breve, sin encabezados ni listas.`

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
	cost         *float64
}

var (
	// errUnsupported indica que el modelo respondió, pero su respuesta no
	// puede entregarse.
	errUnsupported = errors.New("la respuesta redactada no está sustentada en los pasajes")

	// errNoCitations es el caso en que la respuesta no cita ningún pasaje.
	// Casi siempre es el modelo diciendo que los pasajes no responden la
	// pregunta, y vale la pena que quien consulta lo sepa.
	errNoCitations = fmt.Errorf("%w: no cita ningún pasaje", errUnsupported)
)

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

	d := draft{
		provider:     generation.Provider,
		model:        generation.Model,
		promptTokens: generation.PromptTokens,
		outputTokens: generation.OutputTokens,
		cost:         generation.EstimatedCostUSD,
	}
	text, cited, err := verifyCitations(generation.Text, len(passages))
	if err != nil {
		// Se registra lo que el modelo escribió: sin eso no hay cómo saber
		// por qué una consulta terminó en modo degradado.
		slog.WarnContext(ctx, "se descartó la respuesta redactada",
			"modelo", generation.Model, "motivo", err, "texto", truncate(generation.Text, maxLoggedAnswer))
		// El texto se descarta, pero los tokens se gastaron: el borrador
		// vuelve sin texto, con su consumo.
		return d, err
	}
	d.text, d.cited = text, cited
	return d, nil
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
		fmt.Fprintf(&b, "[%d] %s — %s\n", i+1, p.Source, locator(p))
		// La introducción del ítem le dice al modelo a qué lista pertenece
		// el pasaje: sin ella, "8.3. Plan de monitoreo de aguas" no dice
		// que es un compromiso voluntario.
		if p.Context != "" {
			fmt.Fprintf(&b, "(Forma parte de: %s)\n", p.Context)
		}
		fmt.Fprintf(&b, "%s\n\n", p.Text)
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

	text = dropAbsenceCitations(text)

	seen := map[int]bool{}
	for _, m := range citationMark.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 || n > passages {
			return "", nil, fmt.Errorf("%w: cita el pasaje %s y se entregaron %d", errUnsupported, m[1], passages)
		}
		seen[n] = true
	}
	if len(seen) == 0 {
		return "", nil, errNoCitations
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

// absenceClaim reconoce una frase que habla de lo que los pasajes no
// contienen: "los pasajes no mencionan…", "no es posible confirmar…". El
// sujeto son los pasajes, no el proyecto: "el proyecto no contempla permisos
// de contenido ambiental" es algo que el documento sí dice, y conserva su
// cita.
var absenceClaim = regexp.MustCompile(`(?i)\b(los |estos |dichos )?(pasajes|documentos|textos|antecedentes entregados|fragmentos)( entregados| recuperados| proporcionados)?\s+no\s+(contienen|mencionan|incluyen|indican|señalan|explican|precisan|especifican|detallan|entregan|abordan|informan|establecen|hacen referencia)\b|\bno es posible (responder|confirmar|determinar)\b|\bno (hay|se encontró|se encuentra) información\b`)

// dropAbsenceCitations quita las marcas de las frases que solo dicen qué no
// contienen los pasajes. Una marca indica dónde está lo que se afirma; en una
// frase así no hay nada que señalar, y dejarla haría pasar por sustentada una
// respuesta que no lo está. La evaluación mostró que el modelo las pone pese
// a que sus instrucciones se lo prohíben.
func dropAbsenceCitations(text string) string {
	sentences := splitSentences(text)
	for i, s := range sentences {
		if absenceClaim.MatchString(s) && !affirms(s) {
			// El separador con la oración siguiente se conserva tal cual.
			body := strings.TrimRight(s, " ")
			trailing := s[len(body):]
			body = strings.TrimRight(citationMark.ReplaceAllString(body, ""), " ")
			// Sin las marcas puede quedar un espacio antes del punto.
			sentences[i] = strings.ReplaceAll(body, " .", ".") + trailing
		}
	}
	return strings.Join(sentences, "")
}

// reportingVerb es un verbo con el que una frase dice qué contienen los
// pasajes.
var reportingVerb = regexp.MustCompile(`(?i)\b(indican|señalan|señala|establecen|establece|dicen|mencionan|contemplan|precisan|detallan|exigen|disponen|fijan|limitándose a señalar)\b`)

// affirms indica si una frase de ausencia también afirma algo sobre lo que
// los pasajes dicen: "no es posible confirmar los 4 m3; indican que la
// capacidad es de 34 m3". Basta un verbo de ese tipo que no esté negado. La
// frase conserva entonces sus marcas, que sustentan la parte afirmativa.
func affirms(sentence string) bool {
	for _, loc := range reportingVerb.FindAllStringIndex(sentence, -1) {
		before := strings.ToLower(strings.TrimRight(sentence[:loc[0]], " "))
		negated := false
		for _, word := range []string{"no", "ni"} {
			if before == word || strings.HasSuffix(before, " "+word) {
				negated = true
			}
		}
		if !negated {
			return true
		}
	}
	return false
}

// splitSentences divide el texto en oraciones, conservando el espacio que
// las separa para poder volver a unirlas sin alterarlas. Corta después de un
// punto seguido de un espacio y una mayúscula; partir en una abreviatura
// como "D.S. N°4" es inofensivo aquí.
func splitSentences(text string) []string {
	var out []string
	runes := []rune(text)
	start := 0
	for i := 0; i < len(runes)-2; i++ {
		if runes[i] == '.' && runes[i+1] == ' ' && unicode.IsUpper(runes[i+2]) {
			out = append(out, string(runes[start:i+2]))
			start = i + 2
		}
	}
	return append(out, string(runes[start:]))
}
