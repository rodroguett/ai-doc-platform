package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
)

func postQuery(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/queries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return serve(t, s, req)
}

func TestCreateQuery(t *testing.T) {
	rec := postQuery(t, newExampleServer(),
		`{"question": "¿Con qué frecuencia se monitorea el agua subterránea?", "trace": "full"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body)
	}
	var resp api.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Citations) == 0 {
		t.Fatal("la respuesta no trae citas")
	}
	for _, c := range resp.Citations {
		if c.Source == "" || c.Locator == nil {
			t.Errorf("cita sin fuente o sin ubicación: %+v", c)
		}
	}
	if got := rec.Header().Get("X-Trace-Id"); got == "" || got != resp.Trace.TraceId {
		t.Errorf("X-Trace-Id %q no coincide con la traza %q", got, resp.Trace.TraceId)
	}
	if resp.Trace.Retrieved == nil || len(*resp.Trace.Retrieved) != len(resp.Citations) {
		t.Error("trace=full debe incluir los fragmentos recuperados")
	}
}

func TestCreateQueryRejectsShortQuestion(t *testing.T) {
	rec := postQuery(t, newExampleServer(), `{"question": "  a "}`)

	p := decodeProblem(t, rec, http.StatusBadRequest)
	if !strings.HasSuffix(p.Type, "/invalid-request") {
		t.Errorf("got type %q, want invalid-request", p.Type)
	}
	if p.Detail == "" {
		t.Error("el error de validación debe explicar qué falló")
	}
}

func TestCreateQueryRejectsMalformedJSON(t *testing.T) {
	rec := postQuery(t, newExampleServer(), `{"question":`)

	decodeProblem(t, rec, http.StatusBadRequest)
}

func TestCreateQueryHidesInternalErrors(t *testing.T) {
	const internalMsg = "dial tcp 10.0.0.7:5432: password authentication failed"
	s := NewServer(Services{Queries: failingService{err: errors.New(internalMsg)}})

	rec := postQuery(t, s, `{"question": "¿Qué dice la RCA sobre el agua?"}`)

	decodeProblem(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "10.0.0.7") {
		t.Errorf("la respuesta expone el error interno: %s", rec.Body)
	}
}
