package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// newExampleServer arma el servidor con el servicio real de rag sobre un
// almacén en memoria: los tests del gateway ejercitan la validación y la
// paginación verdaderas sin necesitar una base de datos.
func newExampleServer() *Server {
	return NewServer(Services{
		Queries:   service.ExampleQueries{},
		Documents: rag.NewService(newMemStore(), fakeExtractor{}, onePerPage),
	}, WithWriteToken(testToken))
}

// fakeExtractor reemplaza al motor de PDF. Trata el contenido como el texto
// de una sola página, y reproduce los dos rechazos del extractor real: un
// archivo que no empieza como un PDF, y un PDF sin texto.
type fakeExtractor struct {
	// hold, si no es nil, bloquea la extracción hasta que se cierre: permite
	// mantener una subida en curso.
	hold    <-chan struct{}
	started chan<- struct{}
}

const scannedPDF = "%PDF-1.7 escaneado"

func (f fakeExtractor) Extract(_ context.Context, data []byte) ([]rag.PageText, error) {
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.hold != nil {
		<-f.hold
	}
	switch {
	case !bytes.HasPrefix(data, []byte("%PDF")):
		return nil, fmt.Errorf("%w: el archivo no es un PDF legible", rag.ErrInvalidInput)
	case string(data) == scannedPDF:
		return nil, rag.ErrNoTextLayer
	}
	return []rag.PageText{{Number: 1, Text: string(data)}}, nil
}

// onePerPage es un fragmentador mínimo: un fragmento por página.
func onePerPage(pages []rag.PageText) []rag.Chunk {
	chunks := make([]rag.Chunk, len(pages))
	for i, p := range pages {
		chunks[i] = rag.Chunk{Index: i, PageStart: p.Number, PageEnd: p.Number, Text: p.Text}
	}
	return chunks
}

// testToken es el token de escritura de los servidores de prueba. Los
// ayudantes postDocument y do lo envían siempre; los tests de autorización
// arman sus peticiones a mano.
const testToken = "token-de-prueba"

func authorize(req *http.Request) *http.Request {
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req
}

// memStore es un rag.Store en memoria con el mismo comportamiento observable
// que el de Postgres.
type memStore struct {
	mu   sync.Mutex
	docs map[uuid.UUID]rag.Document
	jobs map[uuid.UUID]rag.Job
	now  time.Time
}

func newMemStore() *memStore {
	return &memStore{
		docs: map[uuid.UUID]rag.Document{},
		jobs: map[uuid.UUID]rag.Job{},
		now:  time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
}

func (m *memStore) DocumentExists(_ context.Context, collection, sha string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.docs {
		if d.Collection == collection && d.ContentSHA256 == sha {
			return true, nil
		}
	}
	return false, nil
}

func (m *memStore) CreateDocument(_ context.Context, doc rag.Document, job rag.Job, chunks []rag.Chunk) (rag.Document, rag.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.docs {
		if d.Collection == doc.Collection && d.ContentSHA256 == doc.ContentSHA256 {
			return rag.Document{}, rag.Job{}, rag.ErrDuplicate
		}
	}
	// Cada documento queda un segundo después del anterior, para que el
	// orden del listado sea determinista.
	m.now = m.now.Add(time.Second)
	doc.Status, doc.CreatedAt, doc.ChunkCount = rag.DocumentPending, m.now, len(chunks)
	job.Status, job.DocumentID, job.CreatedAt, job.UpdatedAt = rag.JobQueued, doc.ID, m.now, m.now
	m.docs[doc.ID], m.jobs[job.ID] = doc, job
	return doc, job, nil
}

func (m *memStore) Document(_ context.Context, id uuid.UUID) (rag.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.docs[id]
	if !ok {
		return rag.Document{}, rag.ErrNotFound
	}
	return doc, nil
}

func (m *memStore) Job(_ context.Context, id uuid.UUID) (rag.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return rag.Job{}, rag.ErrNotFound
	}
	return job, nil
}

func (m *memStore) ListDocuments(_ context.Context, collection string, after *rag.Position, limit int) ([]rag.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var docs []rag.Document
	for _, d := range m.docs {
		if collection != "" && d.Collection != collection {
			continue
		}
		if after != nil && !d.CreatedAt.Before(after.CreatedAt) {
			continue
		}
		docs = append(docs, d)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].CreatedAt.After(docs[j].CreatedAt) })
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs, nil
}

func (m *memStore) DeleteDocument(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.docs[id]; !ok {
		return rag.ErrNotFound
	}
	delete(m.docs, id)
	for jobID, job := range m.jobs {
		if job.DocumentID == id {
			delete(m.jobs, jobID)
		}
	}
	return nil
}

// failingQueries simula una dependencia que falla con un error interno.
type failingQueries struct{ err error }

func (f failingQueries) Answer(context.Context, service.Query) (service.Answer, error) {
	return service.Answer{}, f.err
}

func serve(t *testing.T, s *Server, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

// decodeJSON comprueba el estado y decodifica el cuerpo en v.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, status int, v any) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("got status %d, want %d; body: %s", rec.Code, status, rec.Body)
	}
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// decodeProblem comprueba que la respuesta sea un Problem con el estado dado.
func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder, status int) Problem {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("got status %d, want %d; body: %s", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("got content-type %q, want application/problem+json", ct)
	}
	var p Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p.Status != status {
		t.Errorf("got problem status %d, want %d", p.Status, status)
	}
	return p
}
