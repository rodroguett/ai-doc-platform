package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
)

// fakeUsage devuelve un resumen fijo, o un error, y anota el periodo pedido.
type fakeUsage struct {
	summary orchestrator.UsageSummary
	err     error
	got     *orchestrator.UsagePeriod
}

func (f fakeUsage) Usage(_ context.Context, p orchestrator.UsagePeriod) (orchestrator.UsageSummary, error) {
	*f.got = p
	return f.summary, f.err
}

func getUsage(t *testing.T, s *Server, query string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, s, httptest.NewRequest(http.MethodGet, "/v1/usage"+query, nil))
}

func TestGetUsage(t *testing.T) {
	var got orchestrator.UsagePeriod
	s := NewServer(Services{Usage: fakeUsage{got: &got, summary: orchestrator.UsageSummary{
		From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		Queries: 8, CacheHits: 2, TokensTotal: 21_000, EstimatedCostUSD: 0.0074, QueriesWithoutCost: 1,
		ByProvider: []orchestrator.ProviderUsage{{Provider: "gemini", Queries: 7, EstimatedCostUSD: 0.0074}, {Provider: "none", Queries: 1}},
	}}})

	var resp api.Usage
	decodeJSON(t, getUsage(t, s, "?from=2026-10-01&to=2026-10-15"), http.StatusOK, &resp)

	if got.From.Format(time.DateOnly) != "2026-10-01" || got.To.Format(time.DateOnly) != "2026-10-15" {
		t.Errorf("se pidió el periodo %v a %v", got.From, got.To)
	}
	if resp.PeriodStart.Format(time.DateOnly) != "2026-10-01" || resp.PeriodEnd.Format(time.DateOnly) != "2026-10-15" {
		t.Errorf("got periodo %v a %v", resp.PeriodStart, resp.PeriodEnd)
	}
	if resp.Queries != 8 || resp.CacheHitRate != 0.25 || resp.TokensTotal != 21_000 || resp.EstimatedCostUsd != 0.0074 || resp.QueriesWithoutCostEstimate != 1 {
		t.Errorf("got totales %+v", resp)
	}
	if len(resp.ByProvider) != 2 || resp.ByProvider[0].Provider != "gemini" || resp.ByProvider[0].Queries != 7 || resp.ByProvider[1].Provider != "none" {
		t.Errorf("got por proveedor %+v", resp.ByProvider)
	}
}

// Sin consultas, la tasa de aciertos es cero y la lista está vacía, no nula.
func TestGetUsageEmptyPeriod(t *testing.T) {
	var got orchestrator.UsagePeriod
	s := NewServer(Services{Usage: fakeUsage{got: &got}})

	rec := getUsage(t, s, "")

	var resp api.Usage
	decodeJSON(t, rec, http.StatusOK, &resp)
	if !got.From.IsZero() || !got.To.IsZero() {
		t.Errorf("sin parámetros se pidió %v a %v; el periodo por omisión lo fija el dominio", got.From, got.To)
	}
	if resp.CacheHitRate != 0 || resp.ByProvider == nil {
		t.Errorf("got %+v", resp)
	}
}

func TestGetUsageRejectsInvalidInput(t *testing.T) {
	var got orchestrator.UsagePeriod
	invalid := NewServer(Services{Usage: fakeUsage{got: &got, err: fmt.Errorf("%w: from no puede ser posterior a to", orchestrator.ErrInvalidInput)}})
	decodeProblem(t, getUsage(t, invalid, "?from=2026-10-15&to=2026-10-01"), http.StatusBadRequest)

	// Una fecha mal escrita no llega al dominio.
	malformed := NewServer(Services{Usage: fakeUsage{got: &got}})
	decodeProblem(t, getUsage(t, malformed, "?from=15-10-2026"), http.StatusBadRequest)
}

func TestGetUsageUnavailable(t *testing.T) {
	// Sin base de datos no hay registro de uso.
	decodeProblem(t, getUsage(t, NewServer(Services{}), ""), http.StatusServiceUnavailable)

	var got orchestrator.UsagePeriod
	failing := NewServer(Services{Usage: fakeUsage{got: &got, err: fmt.Errorf("%w: no se pudo leer el registro de uso", orchestrator.ErrUnavailable)}})
	decodeProblem(t, getUsage(t, failing, ""), http.StatusServiceUnavailable)
}
