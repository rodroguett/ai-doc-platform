package gateway

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
)

func postQuery(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/queries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return serve(t, s, req)
}

// ingest sube un documento y ejecuta su trabajo: lo deja listo para buscar.
func ingest(t *testing.T, s *Server, content, collection, source string) api.Job {
	t.Helper()
	var job api.Job
	decodeJSON(t, postDocument(t, s, map[string]string{
		"file": content, "collection": collection, "metadata": fmt.Sprintf(`{"source": %q}`, source),
	}), http.StatusAccepted, &job)
	if rec := runJob(t, s, job.Id.String()); rec.Code != http.StatusOK {
		t.Fatalf("ejecutar el trabajo: got status %d; body: %s", rec.Code, rec.Body)
	}
	return job
}

func TestCreateQueryReturnsRealCitations(t *testing.T) {
	s := newExampleServer()
	job := ingest(t, s, "%PDF-1.7 El Titular deberá monitorear los pozos.", "rca", "RCA Tres Valles")

	rec := postQuery(t, s, `{"question": "¿Con qué frecuencia se monitorea el agua subterránea?", "trace": "full"}`)

	var resp api.QueryResponse
	decodeJSON(t, rec, http.StatusOK, &resp)

	if len(resp.Citations) != 1 {
		t.Fatalf("got %d citas, want 1", len(resp.Citations))
	}
	c := resp.Citations[0]
	if c.Source != "RCA Tres Valles" || c.DocumentId != *job.ResourceId {
		t.Errorf("la cita no apunta al documento ingerido: %+v", c)
	}
	if c.Locator == nil || *c.Locator != "Considerando 7, págs. 1-2" {
		t.Errorf("got ubicación %v, want Considerando 7, págs. 1-2", c.Locator)
	}
	if c.Excerpt == nil || !strings.Contains(*c.Excerpt, "monitorear los pozos") {
		t.Errorf("got extracto %v", c.Excerpt)
	}
	if c.Score <= 0 || c.Score > 1 {
		t.Errorf("got puntaje %v, want entre 0 y 1", c.Score)
	}

	// Sin generación, la traza lo declara.
	if resp.Trace.Degraded == nil || !*resp.Trace.Degraded {
		t.Error("la traza debe declarar el modo degradado")
	}
	if resp.Trace.Provider != "none" || resp.Trace.Cache != "miss" {
		t.Errorf("got proveedor %q y caché %q, want none y miss", resp.Trace.Provider, resp.Trace.Cache)
	}
	if got := rec.Header().Get("X-Trace-Id"); got == "" || got != resp.Trace.TraceId {
		t.Errorf("X-Trace-Id %q no coincide con la traza %q", got, resp.Trace.TraceId)
	}
	if resp.Trace.Retrieved == nil || len(*resp.Trace.Retrieved) != 1 {
		t.Error("trace=full debe incluir los pasajes recuperados")
	}
}

// Un documento que todavía no terminó su ingesta no aparece en las
// búsquedas, y sin fuentes la respuesta lo dice en lugar de inventar.
func TestCreateQueryWithoutSources(t *testing.T) {
	s := newExampleServer()
	upload(t, s, "%PDF-1.7 contenido sin embeddings todavía", "rca")

	var resp api.QueryResponse
	decodeJSON(t, postQuery(t, s, `{"question": "¿Qué pozos se monitorean?"}`), http.StatusOK, &resp)

	if len(resp.Citations) != 0 {
		t.Errorf("got %d citas de un documento sin terminar", len(resp.Citations))
	}
	if !strings.Contains(resp.Answer, "No se encontraron pasajes") {
		t.Errorf("got respuesta %q", resp.Answer)
	}
}

func TestCreateQueryRespectsCollectionAndTopK(t *testing.T) {
	s := newExampleServer()
	ingest(t, s, "%PDF-1.7 uno", "rca", "RCA uno")
	ingest(t, s, "%PDF-1.7 dos", "rca", "RCA dos")
	ingest(t, s, "%PDF-1.7 tres", "normativa", "Decreto")

	var all, one, other api.QueryResponse
	decodeJSON(t, postQuery(t, s, `{"question": "pregunta"}`), http.StatusOK, &all)
	decodeJSON(t, postQuery(t, s, `{"question": "pregunta", "top_k": 1}`), http.StatusOK, &one)
	decodeJSON(t, postQuery(t, s, `{"question": "pregunta", "collection": "normativa"}`), http.StatusOK, &other)

	if len(all.Citations) != 3 || len(one.Citations) != 1 {
		t.Errorf("got %d y %d citas, want 3 y 1", len(all.Citations), len(one.Citations))
	}
	if len(other.Citations) != 1 || other.Citations[0].Source != "Decreto" {
		t.Errorf("la colección no restringió la búsqueda: %+v", other.Citations)
	}
}

func TestCreateQueryRejectsInvalidInput(t *testing.T) {
	s := newExampleServer()
	for name, body := range map[string]string{
		"pregunta corta":    `{"question": "  a "}`,
		"top_k excesivo":    `{"question": "¿Qué pozos?", "top_k": 21}`,
		"traza desconocida": `{"question": "¿Qué pozos?", "trace": "debug"}`,
		"JSON mal formado":  `{"question":`,
	} {
		t.Run(name, func(t *testing.T) {
			p := decodeProblem(t, postQuery(t, s, body), http.StatusBadRequest)
			if !strings.HasSuffix(p.Type, "/invalid-request") {
				t.Errorf("got type %q, want invalid-request", p.Type)
			}
		})
	}
}

func TestCreateQueryMapsDomainErrors(t *testing.T) {
	const internalMsg = "dial tcp 10.0.0.7:5432: password authentication failed"
	tests := map[string]struct {
		err    error
		status int
		kind   string
	}{
		"proveedor caído": {fmt.Errorf("%w: no se pudo consultar el corpus", orchestrator.ErrUnavailable), http.StatusServiceUnavailable, "/unavailable"},
		"tope diario":     {fmt.Errorf("%w: se atendieron las 500", orchestrator.ErrQuotaExhausted), http.StatusTooManyRequests, "/quota-exhausted"},
		"error interno":   {errors.New(internalMsg), http.StatusInternalServerError, "/internal"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := NewServer(Services{Queries: failingQueries{err: tt.err}})

			rec := postQuery(t, s, `{"question": "¿Qué dice la RCA sobre el agua?"}`)

			p := decodeProblem(t, rec, tt.status)
			if !strings.HasSuffix(p.Type, tt.kind) {
				t.Errorf("got type %q, want %s", p.Type, tt.kind)
			}
			if strings.Contains(rec.Body.String(), "10.0.0.7") {
				t.Errorf("la respuesta expone el error interno: %s", rec.Body)
			}
		})
	}
}

// Sin base de datos o sin proveedor de embeddings no hay cómo buscar.
func TestCreateQueryWithoutBackend(t *testing.T) {
	decodeProblem(t, postQuery(t, NewServer(Services{}), `{"question": "¿Qué pozos se monitorean?"}`), http.StatusServiceUnavailable)
}
