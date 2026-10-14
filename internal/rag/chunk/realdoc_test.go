package chunk_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/chunk"
	"github.com/rodroguett/ai-doc-platform/internal/rag/pdftext"
)

// realDocument extrae y fragmenta la RCA de prueba, un documento real del
// SEA (ver testdata/README.md).
func realDocument(t *testing.T) ([]rag.PageText, []rag.Chunk) {
	t.Helper()
	data, err := os.ReadFile("../testdata/rca-tres-valles.pdf")
	if err != nil {
		t.Fatalf("leer el documento de prueba: %v", err)
	}
	extractor := pdftext.New()
	t.Cleanup(func() { _ = extractor.Close() })

	pages, err := extractor.Extract(context.Background(), data)
	if err != nil {
		t.Fatalf("extraer: %v", err)
	}
	return pages, chunk.Split(pages)
}

func TestRealDocument(t *testing.T) {
	pages, chunks := realDocument(t)

	t.Run("reconoce la estructura completa", func(t *testing.T) {
		// Ítems de primer nivel de cada parte, en el orden en que aparecen.
		// Un ítem cuya introducción pasó a ser contexto se reconoce por sus
		// subsecciones: "Considerando 8.1" es parte del ítem 8.
		items := map[string][]int{}
		for _, c := range chunks {
			part, number, ok := strings.Cut(c.Section, " ")
			if !ok {
				continue
			}
			number, _, _ = strings.Cut(number, ".")
			n, err := strconv.Atoi(number)
			if err != nil {
				t.Fatalf("sección inesperada %q", c.Section)
			}
			if last := items[part]; len(last) == 0 || last[len(last)-1] != n {
				items[part] = append(items[part], n)
			}
		}

		for part, count := range map[string]int{"Vistos": 7, "Considerando": 23, "Resuelvo": 6} {
			got := items[part]
			if len(got) != count {
				t.Errorf("%s: got %d ítems %v, want %d", part, len(got), got, count)
				continue
			}
			for i, n := range got {
				if n != i+1 {
					t.Errorf("%s: en la posición %d está el ítem %d; la secuencia tiene un salto", part, i, n)
				}
			}
		}
	})

	t.Run("las subsecciones de los compromisos voluntarios saben a qué lista pertenecen", func(t *testing.T) {
		const intro = "8. Que, el proyecto contempla realizar los siguientes compromisos ambientales voluntarios:"
		for _, section := range []string{"Considerando 8.1", "Considerando 8.2", "Considerando 8.3", "Considerando 8.4"} {
			if got := find(t, chunks, section).Context; got != intro {
				t.Errorf("%s: got contexto %q", section, got)
			}
		}
		for _, c := range chunks {
			if c.Section == "Considerando 8" {
				t.Error("el encabezado del considerando 8 sigue siendo un fragmento propio")
			}
		}
	})

	t.Run("ubica el plan de seguimiento de calidad del aire", func(t *testing.T) {
		c := find(t, chunks, "Considerando 10.1")

		if c.Heading != "Plan de Seguimiento Estaciones de Monitoreo Calidad del Aire" {
			t.Errorf("got título %q", c.Heading)
		}
		if c.PageStart != 89 || c.PageEnd != 90 {
			t.Errorf("got páginas %d-%d, want 89-90", c.PageStart, c.PageEnd)
		}
		// Las celdas de la tabla quedan reunidas con su etiqueta.
		for _, want := range []string{
			"Componente Ambiental Calidad de aire.",
			"150 ug/m3N (valor promedio diario)",
			"Coordenadas Estación Manquehua (MP-10): Norte 6.490.298 y Este 315.979.",
		} {
			if !strings.Contains(c.Text, want) {
				t.Errorf("el fragmento no contiene %q", want)
			}
		}
	})

	t.Run("ningún fragmento excede el máximo ni queda vacío", func(t *testing.T) {
		for _, c := range chunks {
			if len(c.Text) > chunk.MaxChars {
				t.Errorf("fragmento %d (%s): %d caracteres", c.Index, c.Section, len(c.Text))
			}
			if strings.TrimSpace(c.Text) == "" {
				t.Errorf("fragmento %d (%s) vacío", c.Index, c.Section)
			}
			if c.PageStart < 1 || c.PageEnd > len(pages) || c.PageStart > c.PageEnd {
				t.Errorf("fragmento %d (%s): páginas %d-%d", c.Index, c.Section, c.PageStart, c.PageEnd)
			}
		}
	})

	t.Run("las páginas avanzan con el documento", func(t *testing.T) {
		last := 0
		for _, c := range chunks {
			if c.PageStart < last {
				t.Fatalf("fragmento %d (%s) empieza en la página %d, antes que el anterior (%d)", c.Index, c.Section, c.PageStart, last)
			}
			last = c.PageStart
		}
	})

	// Lo único que puede faltar en los fragmentos, respecto del texto
	// extraído, es lo que se descarta a propósito: el número y el pie de
	// cada página, los nombres de las partes y los títulos que una tabla
	// repite al continuar en otra página. Cualquier otra palabra que falte
	// es contenido perdido.
	t.Run("no pierde contenido", func(t *testing.T) {
		missing := map[string]int{}
		for _, p := range pages {
			for _, w := range strings.Fields(p.Text) {
				missing[w]++
			}
		}
		headingWords := map[string]bool{}
		// La introducción que pasó a ser contexto se repite en cada
		// subsección: se cuenta una vez por ítem.
		contexts := map[string]bool{}
		for _, c := range chunks {
			if c.Context != "" && !contexts[c.Context] {
				contexts[c.Context] = true
				for _, w := range strings.Fields(c.Context) {
					missing[w]--
				}
			}
			for _, w := range strings.Fields(c.Text) {
				missing[w]--
			}
			if c.Heading != "" {
				firstLine, _, _ := strings.Cut(c.Text, "\n")
				for _, w := range strings.Fields(firstLine) {
					headingWords[w] = true
				}
			}
		}

		allowed := map[string]int{"VISTOS:": 1, "CONSIDERANDO:": 1, "RESUELVO:": 1}
		for n := 1; n <= len(pages); n++ {
			allowed[strconv.Itoa(n)]++
		}
		footer := "Para validar las firmas de este documento usted debe ingresar a la siguiente url https://validador.sea.gob.cl/validar/2167574355"
		for _, w := range strings.Fields(footer) {
			allowed[w] += len(pages)
		}

		var lost []string
		for w, n := range missing {
			switch {
			case n < 0:
				t.Errorf("la palabra %q aparece %d veces más en los fragmentos que en el documento", w, -n)
			case n > allowed[w] && !headingWords[w]:
				lost = append(lost, fmt.Sprintf("%d×%q", n-allowed[w], w))
			}
		}
		if len(lost) > 0 {
			t.Errorf("contenido perdido: %s", strings.Join(lost, " "))
		}
	})
}
