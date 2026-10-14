package rag_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/chunk"
	"github.com/rodroguett/ai-doc-platform/internal/rag/pdftext"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

// countingExtractor trata el contenido como el texto de una página y cuenta
// cuántas veces se lo invoca. Rechaza lo que no empieza como un PDF.
type countingExtractor struct{ calls *int }

func (e countingExtractor) Extract(_ context.Context, data []byte) ([]rag.PageText, error) {
	*e.calls++
	if !strings.HasPrefix(string(data), "%PDF") {
		return nil, fmt.Errorf("%w: el archivo no es un PDF legible", rag.ErrInvalidInput)
	}
	return []rag.PageText{{Number: 1, Text: string(data)}}, nil
}

// twoPerPage produce dos fragmentos por página, sin identificador: asignarlo
// es tarea del servicio.
func twoPerPage(pages []rag.PageText) []rag.Chunk {
	var chunks []rag.Chunk
	for _, p := range pages {
		for part := range 2 {
			chunks = append(chunks, rag.Chunk{
				Index: len(chunks), Section: fmt.Sprintf("Parte %d", part+1),
				PageStart: p.Number, PageEnd: p.Number, Text: p.Text,
			})
		}
	}
	return chunks
}

type fixture struct {
	service     *rag.Service
	store       *store.Store
	extractions *int
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := postgrestest.NewPool(t)
	if _, err := postgres.Migrate(context.Background(), pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	calls := 0
	st := store.New(pool)
	return fixture{
		service:     rag.NewService(rag.Deps{Store: st, Extractor: countingExtractor{calls: &calls}, Split: twoPerPage}),
		store:       st,
		extractions: &calls,
	}
}

func newService(t *testing.T) *rag.Service {
	t.Helper()
	return newFixture(t).service
}

// submit sube un documento cuyo contenido es un PDF de mentira.
func submit(t *testing.T, s *rag.Service, content, collection string) rag.Job {
	t.Helper()
	job, err := s.Submit(context.Background(), rag.Upload{
		Filename:   "rca-042-2019.pdf",
		Collection: collection,
		Content:    []byte("%PDF " + content),
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
	// sha256 de "%PDF contenido", el contenido que arma submit.
	const want = "b894d755d1ea98eaa8abb6652cf4dba01252c493b5991edfde8f87e79f26e3c9"
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
		{Collection: "rca", Content: []byte("%PDF x")},
		{Filename: "a.pdf", Content: []byte("%PDF x")},
		{Filename: "a.pdf", Collection: "rca"},
		{Filename: "a.pdf", Collection: "rca", Content: []byte{}},
	}
	for _, u := range invalid {
		if _, err := s.Submit(ctx, u); !errors.Is(err, rag.ErrInvalidInput) {
			t.Errorf("%+v: got %v, want ErrInvalidInput", u, err)
		}
	}

	submit(t, s, "contenido", "rca")
	_, err := s.Submit(ctx, rag.Upload{Filename: "otro-nombre.pdf", Collection: "rca", Content: []byte("%PDF contenido")})
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

func TestSubmitStoresChunks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	job := submit(t, f.service, "contenido", "rca")

	doc, err := f.service.Document(ctx, job.DocumentID)
	if err != nil {
		t.Fatalf("leer el documento: %v", err)
	}
	if doc.ChunkCount != 2 {
		t.Errorf("got chunk_count %d, want 2", doc.ChunkCount)
	}

	chunks, err := f.store.Chunks(ctx, job.DocumentID)
	if err != nil {
		t.Fatalf("leer los fragmentos: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d fragmentos, want 2", len(chunks))
	}
	seen := map[string]bool{}
	for i, c := range chunks {
		if c.Index != i || c.Section != fmt.Sprintf("Parte %d", i+1) || c.Text != "%PDF contenido" {
			t.Errorf("fragmento %d inesperado: %+v", i, c)
		}
		// El servicio asigna a cada fragmento un identificador propio.
		if c.ID.String() == "00000000-0000-0000-0000-000000000000" || seen[c.ID.String()] {
			t.Errorf("fragmento %d sin identificador único: %s", i, c.ID)
		}
		seen[c.ID.String()] = true
	}
}

// Extraer cuesta segundos y cientos de megabytes: un duplicado debe
// rechazarse sin llegar a hacerlo.
func TestSubmitRejectsDuplicateBeforeExtracting(t *testing.T) {
	f := newFixture(t)
	submit(t, f.service, "contenido", "rca")
	if *f.extractions != 1 {
		t.Fatalf("la primera subida extrajo %d veces, want 1", *f.extractions)
	}

	_, err := f.service.Submit(context.Background(), rag.Upload{
		Filename: "copia.pdf", Collection: "rca", Content: []byte("%PDF contenido"),
	})

	if !errors.Is(err, rag.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	if *f.extractions != 1 {
		t.Errorf("el duplicado se extrajo: %d extracciones, want 1", *f.extractions)
	}
}

func TestSubmitRejectsUnreadableFileWithoutStoringAnything(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.service.Submit(ctx, rag.Upload{Filename: "a.zip", Collection: "rca", Content: []byte("PK no es un PDF")})

	if !errors.Is(err, rag.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
	page, err := f.service.Documents(ctx, rag.ListOptions{})
	if err != nil || len(page.Documents) != 0 {
		t.Errorf("quedaron %d documentos registrados, err %v", len(page.Documents), err)
	}
}

// Un documento del que no sale ningún fragmento no tiene nada que consultar.
func TestSubmitRejectsDocumentWithoutChunks(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	calls := 0
	noChunks := func([]rag.PageText) []rag.Chunk { return nil }
	s := rag.NewService(rag.Deps{Store: store.New(pool), Extractor: countingExtractor{calls: &calls}, Split: noChunks})

	_, err := s.Submit(ctx, rag.Upload{Filename: "a.pdf", Collection: "rca", Content: []byte("%PDF vacío")})

	if !errors.Is(err, rag.ErrNoTextLayer) {
		t.Fatalf("got %v, want ErrNoTextLayer", err)
	}
}

// De punta a punta con las piezas reales: el extractor, el fragmentador y la
// base de datos, sobre la RCA de prueba.
func TestSubmitRealDocument(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	extractor := pdftext.New()
	t.Cleanup(func() { _ = extractor.Close() })
	st := store.New(pool)
	s := rag.NewService(rag.Deps{Store: st, Extractor: extractor, Split: chunk.Split})

	data, err := os.ReadFile("testdata/rca-tres-valles.pdf")
	if err != nil {
		t.Fatalf("leer el documento de prueba: %v", err)
	}
	job, err := s.Submit(ctx, rag.Upload{
		Filename:   "rca-tres-valles.pdf",
		Collection: "rca",
		Metadata:   map[string]any{"source": "RCA Continuidad Operacional Minera Tres Valles"},
		Content:    data,
	})
	if err != nil {
		t.Fatalf("subir: %v", err)
	}

	doc, err := s.Document(ctx, job.DocumentID)
	if err != nil {
		t.Fatalf("leer el documento: %v", err)
	}
	if doc.ChunkCount != 159 || doc.Source != "RCA Continuidad Operacional Minera Tres Valles" {
		t.Errorf("got %d fragmentos y fuente %q, want 159", doc.ChunkCount, doc.Source)
	}

	chunks, err := st.Chunks(ctx, job.DocumentID)
	if err != nil || len(chunks) != 159 {
		t.Fatalf("got %d fragmentos guardados, err %v; want 159", len(chunks), err)
	}
	var air *rag.Chunk
	for i := range chunks {
		if chunks[i].Section == "Considerando 10.1" {
			air = &chunks[i]
		}
	}
	if air == nil {
		t.Fatal("no se guardó el considerando 10.1")
	}
	if air.PageStart != 89 || air.PageEnd != 90 || !strings.Contains(air.Text, "150 ug/m3N") {
		t.Errorf("el considerando 10.1 se guardó mal: páginas %d-%d", air.PageStart, air.PageEnd)
	}

	// El mismo archivo, de nuevo, es un duplicado.
	if _, err := s.Submit(ctx, rag.Upload{Filename: "otra-copia.pdf", Collection: "rca", Content: data}); !errors.Is(err, rag.ErrDuplicate) {
		t.Errorf("segunda subida: got %v, want ErrDuplicate", err)
	}
}
