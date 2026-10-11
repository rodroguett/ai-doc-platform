package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func getReady(t *testing.T, s *Server) (*httptest.ResponseRecorder, readyResponse) {
	t.Helper()
	rec := serve(t, s, httptest.NewRequest(http.MethodGet, "/ready", nil))
	raw := rec.Body.String()

	var resp readyResponse
	decodeJSON(t, rec, rec.Code, &resp)
	rec.Body.WriteString(raw) // decodeJSON consumió el cuerpo; se repone para inspeccionarlo
	return rec, resp
}

func ok(context.Context) error { return nil }

func TestReadyWithHealthyDependencies(t *testing.T) {
	s := NewServer(Services{}, WithReadinessCheck("database", ok), WithReadinessCheck("otra", ok))

	rec, resp := getReady(t, s)

	if rec.Code != http.StatusOK || resp.Status != "ready" {
		t.Fatalf("got %d %q, want 200 ready", rec.Code, resp.Status)
	}
	if len(resp.Checks) != 2 || resp.Checks[0].Name != "database" || resp.Checks[0].Status != "ok" {
		t.Errorf("checks inesperados: %+v", resp.Checks)
	}
}

func TestReadyWithoutDependencies(t *testing.T) {
	rec, resp := getReady(t, NewServer(Services{}))

	if rec.Code != http.StatusOK || resp.Status != "ready" {
		t.Fatalf("got %d %q, want 200 ready", rec.Code, resp.Status)
	}
	// Una lista vacía es [], no null.
	if !strings.Contains(rec.Body.String(), `"checks":[]`) {
		t.Errorf("got body %s, want checks vacío", rec.Body)
	}
}

func TestReadyReportsFailedDependency(t *testing.T) {
	const internalMsg = "dial tcp 10.0.0.7:5432: connection refused"
	s := NewServer(Services{},
		WithReadinessCheck("database", func(context.Context) error { return errors.New(internalMsg) }),
		WithReadinessCheck("otra", ok),
	)

	rec, resp := getReady(t, s)

	if rec.Code != http.StatusServiceUnavailable || resp.Status != "unavailable" {
		t.Fatalf("got %d %q, want 503 unavailable", rec.Code, resp.Status)
	}
	// Se informa cuál falló y cuál no, sin detenerse en la primera.
	if len(resp.Checks) != 2 || resp.Checks[0].Status != "failed" || resp.Checks[1].Status != "ok" {
		t.Errorf("checks inesperados: %+v", resp.Checks)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.7") {
		t.Errorf("la respuesta expone el error interno: %s", rec.Body)
	}
}

// Una dependencia que no responde ni respeta la cancelación no puede dejar
// colgado a /ready.
func TestReadyDoesNotHangOnStuckDependency(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	s := NewServer(Services{}, WithReadinessCheck("database", func(context.Context) error {
		<-release
		return nil
	}))
	s.readyTimeout = 50 * time.Millisecond

	start := time.Now()
	rec, resp := getReady(t, s)
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable || resp.Checks[0].Status != "failed" {
		t.Errorf("got %d %+v, want 503 con la dependencia fallida", rec.Code, resp.Checks)
	}
	if elapsed > time.Second {
		t.Errorf("/ready tardó %v con un plazo de 50 ms", elapsed)
	}
}

// /health indica que el proceso está vivo y nada más: con la base de datos
// caída debe seguir respondiendo 200, y sin consultarla.
func TestHealthIgnoresDependencies(t *testing.T) {
	var calls atomic.Int32
	s := NewServer(Services{}, WithReadinessCheck("database", func(context.Context) error {
		calls.Add(1)
		return errors.New("caída")
	}))

	rec := serve(t, s, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if calls.Load() != 0 {
		t.Errorf("/health consultó la dependencia %d veces", calls.Load())
	}
}
