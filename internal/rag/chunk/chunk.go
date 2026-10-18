// Package chunk divide el texto de un documento en fragmentos que conservan
// su ubicación en el original.
//
// Corta por la estructura del documento —partes, considerandos, secciones
// numeradas— y no por tamaño fijo: un fragmento que respeta los límites de
// una sección puede citarse como "Considerando 10.1", que es como una
// persona buscaría el pasaje en el documento. El tamaño solo interviene
// cuando una sección no cabe en un fragmento.
package chunk

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// MaxChars es el tamaño máximo de un fragmento, en caracteres.
//
// El modelo de embeddings admite 2.048 tokens por texto (ADR-0008). El
// español ronda los cuatro caracteres por token, pero las tablas de
// coordenadas y cifras bajan hasta dos: con 3.500 caracteres, el peor caso
// queda en unos 1.750 tokens.
const MaxChars = 3500

// boilerplateShare es la fracción de páginas en que debe repetirse una línea
// para considerarla un pie o encabezado de página y descartarla.
const boilerplateShare = 0.5

// minPagesForBoilerplate evita descartar líneas repetidas en documentos tan
// cortos que la repetición puede ser contenido.
const minPagesForBoilerplate = 4

var (
	// Una parte del documento: "VISTOS:", "CONSIDERANDO:", "RESUELVO:".
	partPattern = regexp.MustCompile(`^(VISTOS?|CONSIDERANDOS?|RESUELVO|RESUELVE|SE RESUELVE)\s*:?$`)

	// Un capítulo con numeración romana y título en mayúsculas, como los de
	// un ICSARA: "V. PLAN DE PREVENCIÓN DE CONTINGENCIAS Y DE EMERGENCIAS.".
	// Cumple el mismo papel que una parte: sus ítems se numeran desde 1. El
	// título puede ocupar varias líneas y nombrar un artículo ("DEL ARTÍCULO
	// 11 DE LA LEY"), pero empieza y termina en una letra mayúscula.
	chapterPattern = regexp.MustCompile(`^([IVX]{1,5})\.\s+(\p{Lu}[\p{Lu}\d\s,;()°]*\p{Lu})\.?$`)

	// Un ítem de primer nivel: "10. Que, el proyecto…".
	itemPattern = regexp.MustCompile(`^(\d{1,3})[.°]\s+(\S.*)$`)

	// Una subsección: "10.1. Plan de Seguimiento…". Cada componente tiene a
	// lo más dos dígitos, lo que deja fuera cifras como "6.485.551".
	subsectionPattern = regexp.MustCompile(`^(\d{1,2}(?:\.\d{1,2}){1,3})\.?\s+(\S.*)$`)

	spaces = regexp.MustCompile(`[ \t]+`)
)

// partNames da a cada parte el nombre con que se cita.
var partNames = map[string]string{
	"VISTO": "Vistos", "VISTOS": "Vistos",
	"CONSIDERANDO": "Considerando", "CONSIDERANDOS": "Considerando",
	"RESUELVO": "Resuelvo", "RESUELVE": "Resuelvo", "SE RESUELVE": "Resuelvo",
}

// maxHeadingChars separa un título de sección de un párrafo que empieza con
// un número de sección.
const maxHeadingChars = 200

// maxContextChars es el largo bajo el cual la introducción de un ítem con
// subsecciones se trata como contexto de ellas y no como fragmento propio.
// Una introducción que solo anuncia una lista tiene una o dos líneas; una
// con contenido propio, cientos de caracteres.
const maxContextChars = 400

// line es una línea lógica: un párrafo, un título o una celda, ya reunidas
// las líneas que el ancho de la página había partido.
type line struct {
	text      string
	pageStart int
	pageEnd   int
}

// Split divide el documento en fragmentos.
func Split(pages []rag.PageText) []rag.Chunk {
	b := builder{}
	for _, l := range logicalLines(pages) {
		b.add(l)
	}
	b.flush()

	for i := range b.chunks {
		b.chunks[i].Index = i
	}
	return b.chunks
}

// logicalLines limpia las páginas y reúne las líneas partidas.
//
// El extractor termina con un espacio las líneas que la página cortó por
// ancho, y sin él las que terminan un párrafo. Una línea que abre una parte o
// una sección nunca se une a la anterior, aunque esta termine en espacio:
// de lo contrario un título quedaría enterrado al final de un párrafo.
func logicalLines(pages []rag.PageText) []line {
	boilerplate := repeatedLines(pages)

	var out []line
	var cur *line
	end := func() {
		if cur != nil {
			cur.text = strings.TrimSpace(spaces.ReplaceAllString(cur.text, " "))
			if cur.text != "" {
				out = append(out, *cur)
			}
			cur = nil
		}
	}

	for _, page := range pages {
		raw := strings.Split(page.Text, "\n")
		// La primera línea de cada página suele ser su número.
		if len(raw) > 0 && strings.TrimSpace(raw[0]) == strconv.Itoa(page.Number) {
			raw = raw[1:]
		}

		for _, r := range raw {
			trimmed := strings.TrimSpace(r)
			if trimmed == "" || boilerplate[trimmed] {
				continue
			}

			if opensSection(trimmed) {
				end()
			}
			if cur == nil {
				cur = &line{pageStart: page.Number}
			}
			cur.text += r
			cur.pageEnd = page.Number

			// Una parte ("VISTOS:") es siempre una línea completa.
			if !strings.HasSuffix(r, " ") || partPattern.MatchString(trimmed) {
				end()
			}
		}
	}
	end()
	return out
}

func opensSection(s string) bool {
	return partPattern.MatchString(s) || chapterPattern.MatchString(s) || itemPattern.MatchString(s) || subsectionPattern.MatchString(s)
}

// repeatedLines devuelve las líneas que se repiten en la mayoría de las
// páginas: pies de firma, encabezados institucionales.
func repeatedLines(pages []rag.PageText) map[string]bool {
	if len(pages) < minPagesForBoilerplate {
		return nil
	}
	count := map[string]int{}
	for _, page := range pages {
		seen := map[string]bool{}
		for _, r := range strings.Split(page.Text, "\n") {
			if t := strings.TrimSpace(r); t != "" && !seen[t] {
				seen[t] = true
				count[t]++
			}
		}
	}
	out := map[string]bool{}
	for text, n := range count {
		if float64(n) > float64(len(pages))*boilerplateShare {
			out[text] = true
		}
	}
	return out
}

// builder acumula las líneas de la sección en curso y emite sus fragmentos.
type builder struct {
	chunks []rag.Chunk

	part string // "Considerando", o vacía antes de la primera parte

	// chapter es el título del capítulo en curso, en un documento dividido
	// en capítulos: "Plan de prevención de contingencias y de emergencias".
	// Acompaña a cada fragmento del capítulo como contexto.
	chapter string

	item int    // último ítem de primer nivel aceptado en la parte
	sub  string // subsección en curso, por ejemplo "10.1"

	heading string
	lines   []line

	// context es la introducción breve del ítem en curso, cuando el ítem se
	// divide en subsecciones: "8. Que, el proyecto contempla los siguientes
	// compromisos ambientales voluntarios:". Acompaña a cada subsección.
	context string
}

func (b *builder) add(l line) {
	switch {
	case partPattern.MatchString(l.text):
		b.flush()
		b.part = partNames[strings.TrimSpace(strings.TrimSuffix(l.text, ":"))]
		b.item, b.sub, b.heading, b.context = 0, "", "", ""
		// El nombre de la parte no lleva contenido propio: no se guarda.
		return

	case chapterPattern.MatchString(l.text):
		b.flush()
		m := chapterPattern.FindStringSubmatch(l.text)
		b.part, b.chapter = "Capítulo "+m[1], sentenceCase(m[2])
		b.item, b.sub, b.heading, b.context = 0, "", "", ""
		// El título no se guarda como fragmento: va como contexto.
		return

	case b.startsItem(l.text):
		b.flush()
		b.item++
		b.sub, b.heading, b.context = "", "", ""

	case b.startsSubsection(l.text):
		m := subsectionPattern.FindStringSubmatch(l.text)
		if m[1] == b.sub {
			// El mismo título repetido: una tabla que continúa en otra
			// página repite su encabezado. No abre una sección nueva.
			return
		}
		if b.sub == "" && b.introIsContext() {
			// La introducción del ítem no dice nada por sí sola: solo
			// anuncia lo que viene. Como fragmento propio ganaría la
			// búsqueda sin aportar contenido; como contexto de cada
			// subsección, les dice a qué pertenecen.
			b.context = b.text()
			b.lines = nil
		} else {
			b.flush()
		}
		b.sub = m[1]
		b.heading = ""
		if len(m[2]) <= maxHeadingChars {
			b.heading = strings.TrimRight(m[2], ". ")
		}
	}
	b.lines = append(b.lines, l)
}

// startsItem indica si la línea abre el ítem siguiente de la parte en curso.
//
// Exigir el número consecutivo distingue un considerando de una enumeración
// dentro de otro: "1. Estanque…" en medio del considerando 4 no es el
// considerando 1. No distingue el caso en que la enumeración interna llega
// justo al número siguiente.
func (b *builder) startsItem(text string) bool {
	if b.part == "" {
		return false
	}
	m := itemPattern.FindStringSubmatch(text)
	if m == nil {
		return false
	}
	n, err := strconv.Atoi(m[1])
	return err == nil && n == b.item+1
}

// startsSubsection indica si la línea abre una subsección del ítem en curso:
// su número debe empezar por el del ítem.
//
// En un documento por capítulos, solo el segundo nivel abre una subsección:
// "1.2" la abre y "1.2.3" queda dentro de ella. Un ICSARA divide cada
// solicitud en incisos de una o dos líneas; como fragmentos propios
// gastarían una petición de embeddings cada uno y se buscarían sin la
// solicitud a la que pertenecen.
func (b *builder) startsSubsection(text string) bool {
	if b.part == "" || b.item == 0 {
		return false
	}
	m := subsectionPattern.FindStringSubmatch(text)
	if m == nil || !strings.HasPrefix(m[1], strconv.Itoa(b.item)+".") {
		return false
	}
	return b.chapter == "" || strings.Count(m[1], ".") == 1
}

// introIsContext indica si lo acumulado para el ítem, antes de su primera
// subsección, es una introducción breve y no contenido propio.
func (b *builder) introIsContext() bool {
	return len(b.lines) > 0 && len(b.text()) <= maxContextChars
}

func (b *builder) text() string {
	texts := make([]string, len(b.lines))
	for i, l := range b.lines {
		texts[i] = l.text
	}
	return strings.Join(texts, "\n")
}

func (b *builder) section() string {
	switch {
	case b.part == "":
		return ""
	case b.sub != "":
		return b.part + " " + b.sub
	case b.item > 0:
		return b.part + " " + strconv.Itoa(b.item)
	default:
		return b.part
	}
}

// flush emite los fragmentos de la sección en curso. Si no cabe en uno, la
// divide entre líneas; todos los fragmentos conservan la misma ubicación.
func (b *builder) flush() {
	if len(b.lines) == 0 {
		return
	}
	section := b.section()

	var cur []line
	size := 0
	emit := func() {
		if len(cur) == 0 {
			return
		}
		texts := make([]string, len(cur))
		for i, l := range cur {
			texts[i] = l.text
		}
		context := ""
		if b.sub != "" {
			context = b.context
		}
		if b.chapter != "" {
			context = strings.TrimSpace(b.part + ". " + b.chapter + "\n" + context)
		}
		b.chunks = append(b.chunks, rag.Chunk{
			Section:   section,
			Heading:   b.heading,
			Context:   context,
			PageStart: cur[0].pageStart,
			PageEnd:   cur[len(cur)-1].pageEnd,
			Text:      strings.Join(texts, "\n"),
		})
		cur, size = nil, 0
	}

	for _, l := range b.lines {
		for _, piece := range splitLong(l) {
			if size > 0 && size+len(piece.text)+1 > MaxChars {
				emit()
			}
			cur = append(cur, piece)
			size += len(piece.text) + 1
		}
	}
	emit()
	b.lines = nil
}

// splitLong divide una línea que por sí sola excede el tamaño máximo. Corta
// al final de una oración si la hay en la segunda mitad del tramo; si no, en
// un espacio.
func splitLong(l line) []line {
	if len(l.text) <= MaxChars {
		return []line{l}
	}
	var out []line
	rest := l.text
	for len(rest) > MaxChars {
		window := rest[:MaxChars]
		cut := strings.LastIndex(window, ". ")
		if cut < MaxChars/2 {
			cut = strings.LastIndex(window, " ")
		}
		if cut <= 0 {
			// Sin espacios: se corta en el límite, sin partir un carácter.
			cut = MaxChars
			for cut > 0 && !isRuneStart(rest[cut]) {
				cut--
			}
		} else {
			cut++ // incluye el punto o el espacio
		}
		out = append(out, line{text: strings.TrimSpace(rest[:cut]), pageStart: l.pageStart, pageEnd: l.pageEnd})
		rest = strings.TrimSpace(rest[cut:])
	}
	if rest != "" {
		out = append(out, line{text: rest, pageStart: l.pageStart, pageEnd: l.pageEnd})
	}
	return out
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// sentenceCase pasa un título en mayúsculas a minúsculas, salvo la primera
// letra: "PLAN DE PREVENCIÓN" pasa a "Plan de prevención".
func sentenceCase(s string) string {
	s = strings.Join(strings.Fields(strings.ToLower(s)), " ")
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
