// Package pdftext extrae el texto de un PDF, página por página.
//
// Usa PDFium, el motor de PDF de Chromium, compilado a WebAssembly y
// ejecutado dentro del proceso (ADR-0009). No requiere CGO ni programas
// externos, y el motor queda aislado de la memoria del proceso: procesa
// archivos que provienen de fuera.
package pdftext

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// minCharsPerPage es el promedio de caracteres por página bajo el cual se
// considera que el documento no tiene capa de texto. Un escaneo no entrega
// ninguno, o apenas los de un sello; una página de texto real tiene cientos.
const minCharsPerPage = 50

// instanceTimeout es el plazo para obtener el motor cuando está ocupado.
const instanceTimeout = time.Minute

// Extractor extrae texto de documentos PDF. Es seguro para uso concurrente,
// pero procesa un documento a la vez.
//
// El motor se inicia en el primer uso y no al construir el Extractor: tarda
// unos segundos y reserva unos 300 MiB, un costo que solo deben pagar las
// instancias que efectivamente ingieren un documento.
type Extractor struct {
	init sync.Once
	pool pdfium.Pool
	err  error

	// mu serializa las extracciones. El motor admite una sola instancia
	// para no multiplicar la memoria reservada.
	mu sync.Mutex
}

func New() *Extractor {
	return &Extractor{}
}

func (e *Extractor) start() error {
	e.init.Do(func() {
		e.pool, e.err = webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	})
	if e.err != nil {
		return fmt.Errorf("iniciar el motor de PDF: %w", e.err)
	}
	return nil
}

// Close libera el motor. El Extractor no puede usarse después.
func (e *Extractor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.pool == nil {
		return nil
	}
	return e.pool.Close()
}

// Extract devuelve el texto de cada página del PDF, en orden de lectura.
//
// Un archivo que no es un PDF legible devuelve un error que envuelve
// rag.ErrInvalidInput. Un PDF sin capa de texto devuelve rag.ErrNoTextLayer:
// producir fragmentos vacíos dejaría un documento "ingerido" sobre el que
// ninguna consulta encontraría nada.
func (e *Extractor) Extract(ctx context.Context, data []byte) ([]rag.PageText, error) {
	if err := e.start(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	instance, err := e.pool.GetInstance(instanceTimeout)
	if err != nil {
		return nil, fmt.Errorf("obtener el motor de PDF: %w", err)
	}
	defer func() { _ = instance.Close() }()

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		// El detalle de PDFium no se propaga: describe el formato interno y
		// no ayuda a quien subió el archivo.
		return nil, fmt.Errorf("%w: el archivo no es un PDF legible", rag.ErrInvalidInput)
	}
	defer func() { _, _ = instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}) }()

	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, fmt.Errorf("contar las páginas: %w", err)
	}
	if count.PageCount == 0 {
		return nil, fmt.Errorf("%w: el PDF no tiene páginas", rag.ErrInvalidInput)
	}

	pages := make([]rag.PageText, 0, count.PageCount)
	chars := 0
	for i := range count.PageCount {
		// PDFium no admite cancelación; se revisa entre páginas.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		text, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i}},
		})
		if err != nil {
			return nil, fmt.Errorf("extraer la página %d: %w", i+1, err)
		}
		// PDFium separa las líneas con CRLF.
		clean := strings.ReplaceAll(text.Text, "\r\n", "\n")
		pages = append(pages, rag.PageText{Number: i + 1, Text: clean})
		chars += countLetters(clean)
	}

	if chars < minCharsPerPage*len(pages) {
		return nil, rag.ErrNoTextLayer
	}
	return pages, nil
}

func countLetters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}
