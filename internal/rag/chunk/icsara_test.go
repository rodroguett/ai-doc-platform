package chunk_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/chunk"
)

// Los ICSARA del mismo proyecto se dividen en capítulos con numeración
// romana, y dentro de cada uno las solicitudes se numeran desde 1.
func TestICSARA(t *testing.T) {
	for _, tt := range []struct {
		file     string
		chunks   int
		chapters int
	}{
		{"icsara-tres-valles.pdf", 168, 12},
		{"icsara-complementario-tres-valles.pdf", 119, 10},
	} {
		t.Run(tt.file, func(t *testing.T) {
			_, chunks := testDocument(t, tt.file)

			if len(chunks) != tt.chunks {
				t.Errorf("got %d fragmentos, want %d", len(chunks), tt.chunks)
			}

			var chapters []string
			for _, c := range chunks {
				if len(c.Text) > chunk.MaxChars {
					t.Errorf("%s excede el tamaño máximo: %d", c.Section, len(c.Text))
				}
				// Un capítulo sin solicitudes numeradas es su propia sección:
				// "Capítulo X".
				part, number, _ := strings.Cut(strings.TrimPrefix(c.Section, "Capítulo "), " ")
				if c.Section == "" {
					continue // el encabezado del informe, antes del capítulo I
				}
				if !strings.HasPrefix(c.Section, "Capítulo ") {
					t.Errorf("sección inesperada %q", c.Section)
					continue
				}
				// Los incisos de tercer nivel quedan dentro de su solicitud.
				if strings.Count(number, ".") > 1 {
					t.Errorf("%q es de tercer nivel; debía quedar dentro de su solicitud", c.Section)
				}
				if len(chapters) == 0 || chapters[len(chapters)-1] != part {
					chapters = append(chapters, part)
				}
			}
			want := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII"}[:tt.chapters]
			if !slices.Equal(chapters, want) {
				t.Errorf("got capítulos %v, want %v", chapters, want)
			}
		})
	}

	t.Run("el capítulo acompaña a sus fragmentos", func(t *testing.T) {
		_, chunks := testDocument(t, "icsara-tres-valles.pdf")
		i := slices.IndexFunc(chunks, func(c rag.Chunk) bool { return c.Section == "Capítulo V 1.3" })
		if i < 0 {
			t.Fatal("no hay un fragmento Capítulo V 1.3")
		}
		c := chunks[i]
		if !strings.HasPrefix(c.Context, "Capítulo V. Plan de prevención de contingencias y de emergencias") {
			t.Errorf("got contexto %q", c.Context)
		}
		if !strings.HasPrefix(c.Text, "1.3. En el presente plan se identifican los riesgos") || c.PageStart != 37 {
			t.Errorf("got %q en la página %d", c.Text[:60], c.PageStart)
		}
		// La solicitud 1.2 de la descripción del proyecto incluye sus incisos.
		j := slices.IndexFunc(chunks, func(c rag.Chunk) bool { return c.Section == "Capítulo I 1.2" })
		if j < 0 || !strings.Contains(chunks[j].Text, "1.2.4. Aclarar las superficies") {
			t.Errorf("la solicitud 1.2 no incluye sus incisos")
		}
	})
}
