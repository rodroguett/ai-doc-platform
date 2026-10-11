package rag_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

func newService(t *testing.T) *rag.Service {
	t.Helper()
	pool := postgrestest.NewPool(t)
	if _, err := postgres.Migrate(context.Background(), pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	return rag.NewService(store.New(pool))
}

func submit(t *testing.T, s *rag.Service, content, collection string) rag.Job {
	t.Helper()
	job, err := s.Submit(context.Background(), rag.Upload{
		Filename:   "rca-042-2019.pdf",
		Collection: collection,
		Content:    strings.NewReader(content),
	})
	if err != nil {
		t.Fatalf("subir %q: %v", content, err)
	}
	return job
}

func TestSubmitRegistersDocumentAndJob(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	job := submit(t, s, "contenido", "rca")
	if job.Status != rag.JobQueued || job.Kind != rag.JobIngest {
		t.Errorf("got job %s/%s, want ingest/queued", job.Kind, job.Status)
	}

	doc, err := s.Document(ctx, job.DocumentID)
	if err != nil {
		t.Fatalf("leer el documento: %v", err)
	}
	// sha256("contenido"), calculado con sha256sum.
	const want = "6f9566ef46386b8cf372671cb9eddff5488256eff5ea5fb99e1041c3e27082bf"
	if doc.ContentSHA256 != want {
		t.Errorf("got hash %q, want %q", doc.ContentSHA256, want)
	}
	if doc.Status != rag.DocumentPending || doc.Source != "rca-042-2019.pdf" {
		t.Errorf("documento inesperado: %+v", doc)
	}
}

func TestSubmitRejectsInvalidAndDuplicate(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	invalid := []rag.Upload{
		{Collection: "rca", Content: strings.NewReader("x")},
		{Filename: "a.pdf", Content: strings.NewReader("x")},
		{Filename: "a.pdf", Collection: "rca"},
		{Filename: "a.pdf", Collection: "rca", Content: strings.NewReader("")},
	}
	for _, u := range invalid {
		if _, err := s.Submit(ctx, u); !errors.Is(err, rag.ErrInvalidInput) {
			t.Errorf("%+v: got %v, want ErrInvalidInput", u, err)
		}
	}

	submit(t, s, "contenido", "rca")
	_, err := s.Submit(ctx, rag.Upload{Filename: "otro-nombre.pdf", Collection: "rca", Content: strings.NewReader("contenido")})
	if !errors.Is(err, rag.ErrDuplicate) {
		t.Errorf("mismo contenido con otro nombre: got %v, want ErrDuplicate", err)
	}
}

func TestDocumentsPaginatesWithoutGapsOrRepeats(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	var want []string // del más reciente al más antiguo
	for i := range 7 {
		job := submit(t, s, fmt.Sprintf("documento %d", i), "rca")
		want = append([]string{job.DocumentID.String()}, want...)
	}
	submit(t, s, "de otra colección", "normativa")

	var got []string
	opts := rag.ListOptions{Collection: "rca", Limit: 3}
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("la paginación no termina")
		}
		page, err := s.Documents(ctx, opts)
		if err != nil {
			t.Fatalf("listar: %v", err)
		}
		for _, d := range page.Documents {
			got = append(got, d.ID.String())
		}
		if page.NextCursor == "" {
			break
		}
		opts.Cursor = page.NextCursor
	}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("recorrido del listado:\n got %v\nwant %v", got, want)
	}

	// Sin filtro aparecen las dos colecciones.
	all, err := s.Documents(ctx, rag.ListOptions{})
	if err != nil {
		t.Fatalf("listar todo: %v", err)
	}
	if len(all.Documents) != 8 || all.NextCursor != "" {
		t.Errorf("got %d documentos y cursor %q, want 8 y sin cursor", len(all.Documents), all.NextCursor)
	}
}

// Un documento ingerido mientras alguien recorre el listado no debe
// desplazar las páginas siguientes: es la razón de paginar por posición.
func TestDocumentsIsStableUnderConcurrentIngest(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	for i := range 4 {
		submit(t, s, fmt.Sprintf("documento %d", i), "rca")
	}

	first, err := s.Documents(ctx, rag.ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("primera página: %v", err)
	}
	submit(t, s, "llegó durante el recorrido", "rca")

	second, err := s.Documents(ctx, rag.ListOptions{Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("segunda página: %v", err)
	}
	seen := map[string]bool{}
	for _, d := range append(first.Documents, second.Documents...) {
		if seen[d.ID.String()] {
			t.Errorf("el documento %s aparece en dos páginas", d.ID)
		}
		seen[d.ID.String()] = true
	}
	if len(seen) != 4 {
		t.Errorf("got %d documentos distintos, want 4", len(seen))
	}
}

func TestDocumentsRejectsBadOptions(t *testing.T) {
	s := newService(t)
	for _, opts := range []rag.ListOptions{{Limit: -1}, {Limit: 101}, {Cursor: "no-es-un-cursor"}, {Cursor: "YWJj"}} {
		if _, err := s.Documents(context.Background(), opts); !errors.Is(err, rag.ErrInvalidInput) {
			t.Errorf("%+v: got %v, want ErrInvalidInput", opts, err)
		}
	}
}

func TestDeleteRemovesDocumentAndJob(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	job := submit(t, s, "contenido", "rca")

	if err := s.Delete(ctx, job.DocumentID); err != nil {
		t.Fatalf("eliminar: %v", err)
	}
	if _, err := s.Document(ctx, job.DocumentID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("documento: got %v, want ErrNotFound", err)
	}
	if _, err := s.Job(ctx, job.ID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("trabajo: got %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, job.DocumentID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("segundo borrado: got %v, want ErrNotFound", err)
	}
}
