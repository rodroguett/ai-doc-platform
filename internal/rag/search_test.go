package rag_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// Lo que se sube y se termina de ingerir queda disponible para buscar, con
// su fuente y su ubicación.
func TestSearchFindsIngestedDocument(t *testing.T) {
	f := newRunFixture(t, 3)
	ctx := context.Background()
	job := submit(t, f.service, "contenido", "rca")

	// Antes de generar los embeddings no hay nada que encontrar.
	if before, err := f.service.Search(ctx, "¿Qué plan de seguimiento hay?", rag.SearchOptions{TopK: 5}); err != nil || len(before) != 0 {
		t.Fatalf("antes de la ingesta: got %d pasajes, err %v", len(before), err)
	}

	if _, err := f.service.RunJob(ctx, job.ID); err != nil {
		t.Fatalf("ejecutar el trabajo: %v", err)
	}

	got, err := f.service.Search(ctx, "  ¿Qué plan de seguimiento hay?  ", rag.SearchOptions{TopK: 2, Collection: " rca "})
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d pasajes, want 2", len(got))
	}
	p := got[0]
	if p.DocumentID != job.DocumentID || p.Source != "rca-042-2019.pdf" || p.Section != "Considerando 10.1" || p.Heading != "Plan de Seguimiento" {
		t.Errorf("pasaje inesperado: %+v", p)
	}
	// Todos los vectores de prueba son iguales al de la consulta.
	if p.Score < 0.999 {
		t.Errorf("got puntaje %v, want 1", p.Score)
	}
	// La consulta llega al proveedor sin los espacios de más.
	if q := f.embedder.queries[len(f.embedder.queries)-1]; q != "¿Qué plan de seguimiento hay?" {
		t.Errorf("got consulta %q", q)
	}
}

func TestSearchValidatesInput(t *testing.T) {
	f := newRunFixture(t, 1)
	ctx := context.Background()

	for name, tt := range map[string]struct {
		query string
		topK  int
	}{
		"consulta vacía":     {"   ", 5},
		"sin pasajes":        {"pregunta", 0},
		"demasiados pasajes": {"pregunta", 51},
	} {
		if _, err := f.service.Search(ctx, tt.query, rag.SearchOptions{TopK: tt.topK}); !errors.Is(err, rag.ErrInvalidInput) {
			t.Errorf("%s: got %v, want ErrInvalidInput", name, err)
		}
	}
	if len(f.embedder.queries) != 0 {
		t.Errorf("se gastaron %d peticiones al proveedor con entradas inválidas", len(f.embedder.queries))
	}
}

func TestSearchTranslatesProviderErrors(t *testing.T) {
	for name, tt := range map[string]struct {
		cause error
		want  error
	}{
		"sin respuesta":        {fmt.Errorf("%w: 5 intentos", llmgw.ErrUnavailable), rag.ErrUnavailable},
		"mal configurado":      {fmt.Errorf("%w: clave AIza-secreta", llmgw.ErrMisconfigured), rag.ErrUnavailable},
		"consulta rechazada":   {fmt.Errorf("%w (400)", llmgw.ErrInvalidInput), rag.ErrInvalidInput},
		"error no clasificado": {errors.New("dial tcp 10.0.0.7:443"), rag.ErrUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRunFixture(t, 1)
			f.embedder.queryErr = tt.cause

			_, err := f.service.Search(context.Background(), "pregunta", rag.SearchOptions{TopK: 5})

			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			for _, secret := range []string{"AIza-secreta", "10.0.0.7"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("el error expone %q: %v", secret, err)
				}
			}
		})
	}
}

// Las siglas se buscan también por su nombre completo, que es como lo
// escriben muchas secciones.
func TestSearchExpandsAcronyms(t *testing.T) {
	f := newRunFixture(t, 1)

	if _, err := f.service.Search(context.Background(), "¿Cuáles son los PAS?", rag.SearchOptions{TopK: 5}); err != nil {
		t.Fatalf("buscar: %v", err)
	}

	if q := f.embedder.queries[len(f.embedder.queries)-1]; q != "¿Cuáles son los PAS (Permisos Ambientales Sectoriales)?" {
		t.Errorf("got consulta %q", q)
	}
}
