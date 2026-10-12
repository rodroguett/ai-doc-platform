package pdftext_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/pdftext"
)

// El motor tarda unos segundos en iniciarse: los tests comparten uno.
var extractor = pdftext.New()

func TestMain(m *testing.M) {
	code := m.Run()
	_ = extractor.Close()
	os.Exit(code)
}

func TestExtractRealDocument(t *testing.T) {
	data, err := os.ReadFile("../testdata/rca-tres-valles.pdf")
	if err != nil {
		t.Fatalf("leer el documento de prueba: %v", err)
	}

	pages, err := extractor.Extract(context.Background(), data)
	if err != nil {
		t.Fatalf("extraer: %v", err)
	}

	if len(pages) != 108 {
		t.Fatalf("got %d páginas, want 108", len(pages))
	}
	for i, p := range pages {
		if p.Number != i+1 {
			t.Fatalf("la página en la posición %d tiene número %d", i, p.Number)
		}
		if strings.TrimSpace(p.Text) == "" {
			t.Errorf("la página %d no tiene texto", p.Number)
		}
		if strings.Contains(p.Text, "\r") {
			t.Errorf("la página %d conserva retornos de carro", p.Number)
		}
	}

	// El texto debe salir en orden de lectura, con sus tildes.
	checks := map[int]string{
		1:   "CALIFICA AMBIENTALMENTE EL PROYECTO",
		89:  "10.1. Plan de Seguimiento Estaciones de Monitoreo Calidad del Aire.",
		107: "RESUELVO:",
	}
	for page, want := range checks {
		if !strings.Contains(pages[page-1].Text, want) {
			t.Errorf("la página %d no contiene %q", page, want)
		}
	}
}

func TestExtractRejectsNonPDF(t *testing.T) {
	for name, data := range map[string][]byte{
		"texto":       []byte("esto no es un PDF"),
		"vacío":       {},
		"PDF cortado": []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := extractor.Extract(context.Background(), data)
			if !errors.Is(err, rag.ErrInvalidInput) {
				t.Fatalf("got %v, want ErrInvalidInput", err)
			}
			if errors.Is(err, rag.ErrNoTextLayer) {
				t.Error("un archivo ilegible no debe informarse como un escaneo")
			}
		})
	}
}

// Un documento escaneado es un PDF válido cuyas páginas no llevan texto. Se
// rechaza de forma explícita: aceptarlo dejaría un documento "ingerido"
// sobre el que ninguna consulta encontraría nada.
func TestExtractRejectsDocumentWithoutTextLayer(t *testing.T) {
	_, err := extractor.Extract(context.Background(), pdfWithoutText(3))

	if !errors.Is(err, rag.ErrNoTextLayer) {
		t.Fatalf("got %v, want ErrNoTextLayer", err)
	}
	// Es también una entrada inválida, de modo que la API responda 400.
	if !errors.Is(err, rag.ErrInvalidInput) {
		t.Error("ErrNoTextLayer debe envolver ErrInvalidInput")
	}
}

func TestExtractHonoursCancellation(t *testing.T) {
	data, err := os.ReadFile("../testdata/rca-tres-valles.pdf")
	if err != nil {
		t.Fatalf("leer el documento de prueba: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := extractor.Extract(ctx, data); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}

	// El motor sigue utilizable después de una extracción cancelada.
	if _, err := extractor.Extract(context.Background(), data); err != nil {
		t.Fatalf("extraer tras una cancelación: %v", err)
	}
}

// pdfWithoutText arma un PDF válido con n páginas que solo dibujan un
// rectángulo: como un escaneo, tienen contenido gráfico y ningún texto.
func pdfWithoutText(n int) []byte {
	var buf bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}

	buf.WriteString("%PDF-1.4\n")
	kids := make([]string, n)
	for i := range n {
		kids[i] = fmt.Sprintf("%d 0 R", 3+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n))
	const content = "0.8 g 50 50 500 700 re f"
	for i := range n {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R >>", 4+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}
