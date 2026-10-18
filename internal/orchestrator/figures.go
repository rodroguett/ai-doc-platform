package orchestrator

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// figurePattern reconoce una cifra: dígitos, con separadores de miles o
// decimales. "4.000", "0,30", "12.1", "2024".
var figurePattern = regexp.MustCompile(`\d+(?:[.,]\d+)*`)

// thousands reconoce una cifra con punto como separador de miles: "4.000",
// "6.485.551". Un "12.1" no lo es: es un numeral de sección o un decimal
// escrito a la inglesa, y se compara tal cual.
var thousands = regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`)

// figures devuelve las cifras de un texto, tal como aparecen. Los dígitos
// pegados a una letra ("m3", "TK65") no son cifras sueltas: son parte de una
// unidad o de un nombre.
func figures(text string) []string {
	var out []string
	for _, loc := range figurePattern.FindAllStringIndex(text, -1) {
		if r, _ := utf8.DecodeLastRuneInString(text[:loc[0]]); unicode.IsLetter(r) {
			continue
		}
		out = append(out, text[loc[0]:loc[1]])
	}
	return out
}

// normalizeFigure lleva una cifra a una forma comparable: sin separador de
// miles, con punto decimal y sin ceros a la izquierda. "4.000" y "4000" son
// la misma cifra; "01" y "1" también; "0,30" pasa a "0.30" (ADR-0014).
func normalizeFigure(f string) string {
	switch {
	case strings.Contains(f, ",") && strings.Contains(f, "."):
		f = strings.ReplaceAll(f, ".", "")
	case thousands.MatchString(f):
		f = strings.ReplaceAll(f, ".", "")
	}
	f = strings.ReplaceAll(f, ",", ".")

	parts := strings.Split(f, ".")
	// Solo la parte entera pierde sus ceros a la izquierda; en "12.05" el
	// cero importa.
	parts[0] = strings.TrimLeft(parts[0], "0")
	if parts[0] == "" {
		parts[0] = "0"
	}
	return strings.Join(parts, ".")
}

// negatedClause reconoce una cláusula que niega lo que dicen los pasajes:
// "sin mencionar una capacidad de 4 m3", "ni hacer referencia al numeral
// 4.3.2", "no indican que sea de 4 m3", "tampoco mencionan los numerales
// 8.2 y 8.3". Llega hasta la coma, el punto y coma
// o el final de la oración: lo que sigue a "…, sino que señalan" es una
// afirmación y se verifica.
var negatedClause = regexp.MustCompile(`(?i)\b(?:sin|no|ni|tampoco)\s+(?:se\s+)?(?:mencion|hac\w*\s+referencia|indic|señal|contien|conten|inclu|precis|especific|establec|detall|confirm|refier|alud)\w*[^,;]*`)

// unverifiedFigures devuelve las cifras de la respuesta que no aparecen en
// los pasajes que cita la oración donde están (ADR-0014). text ya tiene las
// marcas renumeradas: [1] es cited[0].
//
// Solo se revisan las oraciones con marcas, y en ellas no se revisan las
// cláusulas que niegan lo que dicen los pasajes: la cifra negada viene de la
// pregunta y no tiene por qué estar en ellos. El modelo niega de dos formas,
// y las dos se midieron: empezando por la ausencia ("los pasajes no indican
// 4 m3, sino que señalan 34 m3 [1]") o agregándola a una afirmación
// ("tiene 34 m3, sin mencionar una capacidad de 4 m3 [1]").
func unverifiedFigures(text string, cited []rag.Passage) []string {
	var out []string
	seen := map[string]bool{}
	for _, sentence := range splitSentences(text) {
		marks := citationMark.FindAllStringSubmatch(sentence, -1)
		if len(marks) == 0 {
			continue
		}
		claim := sentence
		if absenceClaim.MatchString(sentence) {
			if at := affirmation(sentence); at >= 0 {
				claim = sentence[at:]
			}
		}

		var source strings.Builder
		for _, m := range marks {
			// Las marcas ya pasaron la verificación de forma.
			n, err := strconv.Atoi(m[1])
			if i := n - 1; err == nil && i >= 0 && i < len(cited) {
				source.WriteString(cited[i].Context + "\n" + cited[i].Text + "\n")
			}
		}
		available := map[string]bool{}
		for _, f := range figures(source.String()) {
			available[normalizeFigure(f)] = true
		}

		claim = negatedClause.ReplaceAllString(claim, "")
		for _, f := range figures(citationMark.ReplaceAllString(claim, "")) {
			if n := normalizeFigure(f); !available[n] && !seen[n] {
				seen[n] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// figuresNotice es el aviso que se agrega a una respuesta con cifras sin
// respaldo en sus citas.
func figuresNotice(unverified []string) string {
	quoted := make([]string, len(unverified))
	for i, f := range unverified {
		quoted[i] = "«" + f + "»"
	}
	if len(quoted) == 1 {
		return "Aviso: la cifra " + quoted[0] + " no aparece en los pasajes que cita su oración. Verifícala en las citas antes de usarla."
	}
	list := strings.Join(quoted[:len(quoted)-1], ", ") + " y " + quoted[len(quoted)-1]
	return "Aviso: las cifras " + list + " no aparecen en los pasajes que citan sus oraciones. Verifícalas en las citas antes de usarlas."
}
