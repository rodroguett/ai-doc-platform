package cloudtasks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const queue = "projects/mi-proyecto/locations/us-central1/queues/ingest"

type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

type failingToken struct{}

func (failingToken) Token(context.Context) (string, error) {
	return "", errors.New("sin servidor de metadatos")
}

func newClient(t *testing.T, handler http.HandlerFunc, tokens TokenSource) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewWithTokenSource(queue, tokens)
	if err != nil {
		t.Fatalf("crear el cliente: %v", err)
	}
	c.baseURL = srv.URL
	return c
}

func TestEnqueueSendsTheTask(t *testing.T) {
	var path, auth string
	var body struct {
		Task struct {
			HTTPRequest struct {
				URL        string            `json:"url"`
				HTTPMethod string            `json:"httpMethod"`
				Headers    map[string]string `json:"headers"`
				Body       []byte            `json:"body"`
			} `json:"httpRequest"`
		} `json:"task"`
	}
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("cuerpo ilegible: %v", err)
		}
		_, _ = w.Write([]byte(`{"name": "tasks/123"}`))
	}, staticToken("token-de-acceso"))

	err := c.Enqueue(context.Background(), Task{
		URL:     "https://servicio.example/v1/jobs/abc/run",
		Headers: map[string]string{"Authorization": "Bearer token-interno"},
		Body:    []byte(`{"a":1}`),
	})
	if err != nil {
		t.Fatalf("encolar: %v", err)
	}

	if path != "/"+queue+"/tasks" {
		t.Errorf("got ruta %q", path)
	}
	if auth != "Bearer token-de-acceso" {
		t.Errorf("got Authorization %q", auth)
	}
	req := body.Task.HTTPRequest
	if req.URL != "https://servicio.example/v1/jobs/abc/run" || req.HTTPMethod != "POST" {
		t.Errorf("tarea inesperada: %+v", req)
	}
	if req.Headers["Authorization"] != "Bearer token-interno" {
		t.Errorf("la tarea no lleva su encabezado: %v", req.Headers)
	}
	if string(req.Body) != `{"a":1}` {
		t.Errorf("got cuerpo %q", req.Body)
	}
}

func TestEnqueueReportsFailuresWithoutLeakingTheTask(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusForbidden)
		// La API puede repetir en el error lo que recibió.
		_, _ = w.Write(raw)
	}, staticToken("token-de-acceso"))

	err := c.Enqueue(context.Background(), Task{
		URL:     "https://servicio.example/run",
		Headers: map[string]string{"Authorization": "Bearer token-interno"},
	})

	if err == nil {
		t.Fatal("se esperaba un error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("el error no indica el estado: %v", err)
	}
	for _, secret := range []string{"token-interno", "token-de-acceso"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("el error expone %q: %v", secret, err)
		}
	}
}

func TestEnqueueFailsWithoutToken(t *testing.T) {
	called := false
	c := newClient(t, func(http.ResponseWriter, *http.Request) { called = true }, failingToken{})

	if err := c.Enqueue(context.Background(), Task{URL: "https://servicio.example/run"}); err == nil {
		t.Fatal("se esperaba un error")
	}
	if called {
		t.Error("se llamó a la API sin token")
	}
}

func TestRejectsMalformedQueueName(t *testing.T) {
	for _, name := range []string{"", "ingest", "projects/p/locations/l/queues/", "projects//locations/l/queues/q", "proyectos/p/locations/l/queues/q"} {
		if _, err := NewWithTokenSource(name, staticToken("x")); err == nil {
			t.Errorf("se aceptó el nombre de cola %q", name)
		}
	}
}

// El token se pide una vez y se reutiliza hasta poco antes de que venza.
func TestMetadataTokenIsCachedUntilItExpires(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			t.Error("falta el encabezado Metadata-Flavor")
		}
		n := requests.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-" + string(rune('0'+n)), "expires_in": 3600})
	}))
	t.Cleanup(srv.Close)

	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	m := &metadataTokens{url: srv.URL, http: srv.Client(), now: func() time.Time { return clock }}
	ctx := context.Background()

	first, err := m.Token(ctx)
	if err != nil || first != "token-1" {
		t.Fatalf("got %q, err %v", first, err)
	}
	clock = clock.Add(30 * time.Minute)
	if again, _ := m.Token(ctx); again != "token-1" || requests.Load() != 1 {
		t.Errorf("a media vida: got %q tras %d peticiones, want el mismo token sin pedir otro", again, requests.Load())
	}

	// A menos de un minuto del vencimiento se renueva.
	clock = clock.Add(29*time.Minute + 30*time.Second)
	if renewed, _ := m.Token(ctx); renewed != "token-2" {
		t.Errorf("cerca del vencimiento: got %q, want uno nuevo", renewed)
	}
}

func TestMetadataTokenFailures(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"estado de error": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
		"sin token":       func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) },
		"no es JSON":      func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`<html>`)) },
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)
			m := &metadataTokens{url: srv.URL, http: srv.Client(), now: time.Now}
			if token, err := m.Token(context.Background()); err == nil {
				t.Errorf("got token %q sin error", token)
			}
		})
	}
}
