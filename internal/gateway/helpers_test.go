package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
)

func newExampleServer() *Server {
	return NewServer(Services{
		Queries:   service.ExampleQueries{},
		Documents: service.ExampleDocuments{},
	})
}

// failingService simula una dependencia que falla con un error interno.
type failingService struct{ err error }

func (f failingService) Answer(context.Context, service.Query) (service.Answer, error) {
	return service.Answer{}, f.err
}

func (f failingService) Submit(context.Context, service.Upload) (service.Job, error) {
	return service.Job{}, f.err
}

func serve(t *testing.T, s *Server, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
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
