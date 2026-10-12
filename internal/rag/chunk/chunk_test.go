package chunk_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/chunk"
)

// page arma una página a partir de sus líneas. Una línea que termina en
// espacio es una que el ancho de la página partió, como las entrega el
// extractor.
func page(number int, lines ...string) rag.PageText {
	return rag.PageText{Number: number, Text: strings.Join(lines, "\n")}
}

// sections devuelve la ubicación de cada fragmento, en orden.
func sections(chunks []rag.Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.Section
	}
	return out
}

func find(t *testing.T, chunks []rag.Chunk, section string) rag.Chunk {
	t.Helper()
	for _, c := range chunks {
		if c.Section == section {
			return c
		}
	}
	t.Fatalf("no hay un fragmento en %q; secciones: %v", section, sections(chunks))
	return rag.Chunk{}
}

func TestSplitFollowsDocumentStructure(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{
		page(1,
			"REPÚBLICA DE CHILE",
			"VISTOS:",
			"1. La Declaración de Impacto Ambiental.",
			"2. Los pronunciamientos de los órganos.",
			"CONSIDERANDO:",
			"1. Que, el Titular ha sometido el proyecto.",
		),
		page(2,
			"2. Que, el proyecto contempla el siguiente plan:",
			"2.1. Plan de Seguimiento de Calidad del Aire.",
			"Se mantienen las estaciones de monitoreo.",
			"2.2. Plan de Seguimiento de Aguas.",
			"Monitoreo trimestral de pozos.",
			"RESUELVO:",
			"1. Calificar favorablemente el proyecto.",
		),
	})

	want := []string{
		"", "Vistos 1", "Vistos 2",
		"Considerando 1", "Considerando 2", "Considerando 2.1", "Considerando 2.2",
		"Resuelvo 1",
	}
	if got := sections(chunks); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("secciones:\n got %q\nwant %q", got, want)
	}

	air := find(t, chunks, "Considerando 2.1")
	if air.Heading != "Plan de Seguimiento de Calidad del Aire" {
		t.Errorf("got título %q", air.Heading)
	}
	if air.PageStart != 2 || air.PageEnd != 2 {
		t.Errorf("got páginas %d-%d, want 2-2", air.PageStart, air.PageEnd)
	}
	// El título va también en el texto: es parte de lo que se busca.
	if want := "2.1. Plan de Seguimiento de Calidad del Aire.\nSe mantienen las estaciones de monitoreo."; air.Text != want {
		t.Errorf("got texto %q, want %q", air.Text, want)
	}

	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("el fragmento en la posición %d tiene índice %d", i, c.Index)
		}
		if strings.TrimSpace(c.Text) == "" {
			t.Errorf("el fragmento %d está vacío", i)
		}
	}
}

// Una enumeración dentro de un considerando no abre considerandos nuevos:
// solo lo hace el número que sigue al considerando en curso.
func TestSplitIgnoresInnerEnumerations(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{page(1,
		"CONSIDERANDO:",
		"1. Que, el proyecto considera.",
		"2. Que, las obras son las siguientes:",
		"1. Estanque de acumulación.",
		"4. Planta de procesamiento.",
		"3. Que, el Titular deberá informar.",
	)})

	want := []string{"Considerando 1", "Considerando 2", "Considerando 3"}
	if got := sections(chunks); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("secciones:\n got %q\nwant %q", got, want)
	}
	if c := find(t, chunks, "Considerando 2"); !strings.Contains(c.Text, "1. Estanque") || !strings.Contains(c.Text, "4. Planta") {
		t.Errorf("la enumeración interna debe quedar en su considerando: %q", c.Text)
	}
}

func TestSplitJoinsWrappedLines(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{
		page(1,
			"CONSIDERANDO:",
			"1. Que, el Titular deberá monitorear ",
			"trimestralmente el nivel freático ",
		),
		page(2,
			"en los pozos PM-01 a PM-06.",
			"Los resultados se informan a la autoridad.",
		),
	})

	c := find(t, chunks, "Considerando 1")
	want := "1. Que, el Titular deberá monitorear trimestralmente el nivel freático en los pozos PM-01 a PM-06.\n" +
		"Los resultados se informan a la autoridad."
	if c.Text != want {
		t.Errorf("got texto %q\nwant %q", c.Text, want)
	}
	// Un párrafo que cruza de página ubica al fragmento en ambas.
	if c.PageStart != 1 || c.PageEnd != 2 {
		t.Errorf("got páginas %d-%d, want 1-2", c.PageStart, c.PageEnd)
	}
}

// La línea anterior a un título puede terminar en espacio. El título no debe
// quedar pegado a ella, o la sección desaparecería.
func TestSplitDoesNotSwallowHeadings(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{page(1,
		"VISTOS: ",
		"1. La Declaración de Impacto Ambiental. ",
		"CONSIDERANDO: ",
		"1. Que, lo primero. ",
		"2. Que, lo segundo. ",
		"2.1. Detalle de lo segundo. ",
		"Contenido del detalle.",
	)})

	want := []string{"Vistos 1", "Considerando 1", "Considerando 2", "Considerando 2.1"}
	if got := sections(chunks); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("secciones:\n got %q\nwant %q", got, want)
	}
}

func TestSplitDropsPageFurniture(t *testing.T) {
	var pages []rag.PageText
	for n := 1; n <= 5; n++ {
		pages = append(pages, page(n,
			fmt.Sprint(n),
			" ",
			fmt.Sprintf("Contenido propio de la página %d.", n),
			"Para validar las firmas de este documento ingrese a la url",
			"https://validador.example/validar/123",
		))
	}

	text := ""
	for _, c := range chunk.Split(pages) {
		text += c.Text + "\n"
	}

	if strings.Contains(text, "validar") {
		t.Errorf("el pie repetido no se descartó:\n%s", text)
	}
	for n := 1; n <= 5; n++ {
		if !strings.Contains(text, fmt.Sprintf("Contenido propio de la página %d.", n)) {
			t.Errorf("se perdió el contenido de la página %d", n)
		}
		// El número de página, solo en su línea, no es contenido.
		if strings.Contains(text, fmt.Sprintf("\n%d\n", n)) || strings.HasPrefix(text, fmt.Sprintf("%d\n", n)) {
			t.Errorf("quedó el número de la página %d", n)
		}
	}
}

// En un documento corto, una línea repetida puede ser contenido.
func TestSplitKeepsRepeatedLinesInShortDocuments(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{
		page(1, "Sin observaciones.", "Primera parte."),
		page(2, "Sin observaciones.", "Segunda parte."),
	})

	text := ""
	for _, c := range chunks {
		text += c.Text + "\n"
	}
	if strings.Count(text, "Sin observaciones.") != 2 {
		t.Errorf("se descartó una línea repetida de un documento corto:\n%s", text)
	}
}

// Una tabla que continúa en otra página repite el título de su sección. No es
// una sección nueva, y el título repetido no se duplica en el texto.
func TestSplitMergesRepeatedSectionHeading(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{
		page(1,
			"CONSIDERANDO:",
			"1. Que, el proyecto se describe así:",
			"1.3. PARTES Y OBRAS DEL PROYECTO",
			"Obra uno.",
		),
		page(2,
			"1.3. PARTES Y OBRAS DEL PROYECTO",
			"Obra dos.",
		),
	})

	want := []string{"Considerando 1", "Considerando 1.3"}
	if got := sections(chunks); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("secciones:\n got %q\nwant %q", got, want)
	}
	c := find(t, chunks, "Considerando 1.3")
	if strings.Count(c.Text, "PARTES Y OBRAS") != 1 {
		t.Errorf("el título repetido se duplicó: %q", c.Text)
	}
	if !strings.Contains(c.Text, "Obra uno.") || !strings.Contains(c.Text, "Obra dos.") {
		t.Errorf("se perdió contenido: %q", c.Text)
	}
	if c.PageStart != 1 || c.PageEnd != 2 {
		t.Errorf("got páginas %d-%d, want 1-2", c.PageStart, c.PageEnd)
	}
}

// Coordenadas, fechas y números de otra sección empiezan una línea con
// dígitos y puntos sin ser títulos.
func TestSplitDoesNotMistakeNumbersForSections(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{page(1,
		"CONSIDERANDO:",
		"1. Que, las estaciones se ubican en:",
		"6.485.551 y Este 305.439.",
		"2026. Año de inicio de la operación.",
		"7.2. Referencia a una sección de otro considerando.",
	)})

	if got := sections(chunks); len(got) != 1 || got[0] != "Considerando 1" {
		t.Fatalf("got secciones %q, want solo Considerando 1", got)
	}
}

func TestSplitDividesOversizedSection(t *testing.T) {
	lines := []string{"CONSIDERANDO:", "1. Que, el plan es el siguiente:", "1.1. Plan extenso."}
	for i := range 120 {
		lines = append(lines, fmt.Sprintf("Compromiso número %d: el Titular deberá ejecutar la medida correspondiente y reportarla.", i))
	}
	lines = append(lines, "2. Que, lo siguiente.")

	chunks := chunk.Split([]rag.PageText{page(1, lines...)})

	var parts []rag.Chunk
	for _, c := range chunks {
		if len(c.Text) > chunk.MaxChars {
			t.Errorf("un fragmento de %q mide %d caracteres, sobre el máximo de %d", c.Section, len(c.Text), chunk.MaxChars)
		}
		if c.Section == "Considerando 1.1" {
			parts = append(parts, c)
		}
	}
	if len(parts) < 2 {
		t.Fatalf("la sección extensa quedó en %d fragmentos, want varios", len(parts))
	}
	// Todas las partes conservan la ubicación y el título.
	joined := ""
	for _, c := range parts {
		if c.Heading != "Plan extenso" {
			t.Errorf("una parte perdió el título: %q", c.Heading)
		}
		joined += c.Text + "\n"
	}
	// Se corta entre líneas: ninguna queda partida ni se pierde.
	for i := range 120 {
		if want := fmt.Sprintf("Compromiso número %d: el Titular deberá ejecutar la medida correspondiente y reportarla.\n", i); !strings.Contains(joined, want) {
			t.Fatalf("el compromiso %d quedó partido o se perdió", i)
		}
	}
	find(t, chunks, "Considerando 2")
}

// Un párrafo que por sí solo excede el máximo se corta al final de una
// oración, sin perder texto ni partir una letra acentuada.
func TestSplitDividesOversizedParagraph(t *testing.T) {
	sentence := "El Titular deberá mantener la medición de la calidad del agua subterránea en cada estación. "
	paragraph := strings.TrimSpace(strings.Repeat(sentence, 100))

	chunks := chunk.Split([]rag.PageText{page(1, paragraph)})

	if len(chunks) < 2 {
		t.Fatalf("got %d fragmentos, want varios", len(chunks))
	}
	rebuilt := ""
	for _, c := range chunks {
		if len(c.Text) > chunk.MaxChars {
			t.Errorf("un fragmento mide %d caracteres", len(c.Text))
		}
		if !strings.HasSuffix(c.Text, "estación.") {
			t.Errorf("un fragmento no termina en una oración completa: %q", c.Text[len(c.Text)-40:])
		}
		rebuilt += c.Text + " "
	}
	if strings.TrimSpace(rebuilt) != paragraph {
		t.Error("al reunir los fragmentos no se recupera el párrafo original")
	}
}

func TestSplitUnstructuredDocument(t *testing.T) {
	chunks := chunk.Split([]rag.PageText{
		page(1, "Informe de seguimiento ambiental.", "Durante el período se realizaron las mediciones."),
		page(2, "No se registraron superaciones de norma."),
	})

	if len(chunks) != 1 || chunks[0].Section != "" {
		t.Fatalf("got %d fragmentos con secciones %q, want uno sin sección", len(chunks), sections(chunks))
	}
	if chunks[0].PageStart != 1 || chunks[0].PageEnd != 2 {
		t.Errorf("got páginas %d-%d, want 1-2", chunks[0].PageStart, chunks[0].PageEnd)
	}
}

func TestSplitEmptyInput(t *testing.T) {
	if chunks := chunk.Split(nil); len(chunks) != 0 {
		t.Errorf("got %d fragmentos, want 0", len(chunks))
	}
	if chunks := chunk.Split([]rag.PageText{page(1, " ", "")}); len(chunks) != 0 {
		t.Errorf("una página en blanco produjo %d fragmentos", len(chunks))
	}
}
