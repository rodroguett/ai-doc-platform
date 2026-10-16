package store_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator/store"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

func TestRecord(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	// Los dos módulos migran en la misma base, como en producción: cada uno
	// con su tabla de versiones.
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar rag: %v", err)
	}
	if _, err := postgres.MigrateTable(ctx, pool, orchestrator.Migrations(), orchestrator.MigrationsTable); err != nil {
		t.Fatalf("migrar orchestrator: %v", err)
	}
	s := store.New(pool)

	at := time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC)
	cost := 0.00124
	for _, u := range []orchestrator.Usage{
		{At: at, TraceID: "a1", Collection: "rca", Provider: "gemini", Model: "gemini-3.5-flash-lite",
			PromptTokens: 3600, OutputTokens: 60, Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered, EstimatedCostUSD: &cost},
		// Un modelo sin tarifa: el costo queda nulo, no cero.
		{At: at, TraceID: "b2", Collection: "", Provider: "gemini", Model: "modelo-nuevo",
			PromptTokens: 3600, OutputTokens: 60, Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeNotAnswered},
	} {
		if err := s.Record(ctx, u); err != nil {
			t.Fatalf("registrar %s: %v", u.TraceID, err)
		}
	}

	var (
		occurred       time.Time
		model, outcome string
		prompt, output int
		known, unknown *float64
	)
	if err := pool.QueryRow(ctx, `SELECT occurred_at, model, outcome, prompt_tokens, output_tokens, estimated_cost_usd
		FROM query_usage WHERE trace_id = 'a1'`).Scan(&occurred, &model, &outcome, &prompt, &output, &known); err != nil {
		t.Fatalf("leer a1: %v", err)
	}
	if !occurred.Equal(at) || model != "gemini-3.5-flash-lite" || outcome != "answered" || prompt != 3600 || output != 60 || known == nil || *known != cost {
		t.Errorf("fila inesperada: %v %s %s %d %d %v", occurred, model, outcome, prompt, output, known)
	}
	if err := pool.QueryRow(ctx, `SELECT estimated_cost_usd FROM query_usage WHERE trace_id = 'b2'`).Scan(&unknown); err != nil {
		t.Fatalf("leer b2: %v", err)
	}
	if unknown != nil {
		t.Errorf("got costo %v, want nulo", *unknown)
	}
}

// Un valor fuera de lo que el esquema admite no se guarda en silencio.
func TestRecordRejectsInvalidOutcome(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.MigrateTable(ctx, pool, orchestrator.Migrations(), orchestrator.MigrationsTable); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	err := store.New(pool).Record(ctx, orchestrator.Usage{At: time.Now(), TraceID: "c3", Provider: "none", Model: "none",
		Cache: orchestrator.CacheMiss, Outcome: "inventado"})
	if err == nil {
		t.Fatal("se registró un desenlace que no existe")
	}
}

func TestSummarize(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.MigrateTable(ctx, pool, orchestrator.Migrations(), orchestrator.MigrationsTable); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	s := store.New(pool)

	day := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	cost := func(v float64) *float64 { return &v }
	for i, u := range []orchestrator.Usage{
		{At: day.Add(time.Hour), Provider: "gemini", Model: "m", PromptTokens: 3000, OutputTokens: 60, Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered, EstimatedCostUSD: cost(0.001)},
		{At: day.Add(2 * time.Hour), Provider: "gemini", Model: "m", Cache: orchestrator.CacheHit, Outcome: orchestrator.OutcomeAnswered, EstimatedCostUSD: cost(0)},
		// Sin tarifa: cuenta como consulta, no suma al costo.
		{At: day.Add(3 * time.Hour), Provider: "gemini", Model: "nuevo", PromptTokens: 1000, OutputTokens: 40, Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered},
		{At: day.Add(4 * time.Hour), Provider: "none", Model: "none", Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeNoSources, EstimatedCostUSD: cost(0)},
		// Fuera del periodo, por un instante a cada lado.
		{At: day.Add(-time.Nanosecond * 1000), Provider: "gemini", Model: "m", PromptTokens: 9, Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered, EstimatedCostUSD: cost(5)},
		{At: day.Add(24 * time.Hour), Provider: "gemini", Model: "m", PromptTokens: 9, Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered, EstimatedCostUSD: cost(5)},
	} {
		u.TraceID = fmt.Sprint(i)
		if err := s.Record(ctx, u); err != nil {
			t.Fatalf("registrar %d: %v", i, err)
		}
	}

	got, err := s.Summarize(ctx, day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("sumar: %v", err)
	}

	if got.Queries != 4 || got.CacheHits != 1 || got.TokensTotal != 4100 || got.QueriesWithoutCost != 1 {
		t.Errorf("totales inesperados: %+v", got)
	}
	if math.Abs(got.EstimatedCostUSD-0.001) > 1e-12 {
		t.Errorf("got costo %v, want 0.001", got.EstimatedCostUSD)
	}
	want := []orchestrator.ProviderUsage{{Provider: "gemini", Queries: 3, EstimatedCostUSD: 0.001}, {Provider: "none", Queries: 1}}
	if len(got.ByProvider) != 2 || got.ByProvider[0].Provider != want[0].Provider || got.ByProvider[0].Queries != 3 ||
		math.Abs(got.ByProvider[0].EstimatedCostUSD-0.001) > 1e-12 || got.ByProvider[1] != want[1] {
		t.Errorf("got por proveedor %+v, want %+v", got.ByProvider, want)
	}
}

// Un periodo sin consultas no es un error: todo en cero.
func TestSummarizeEmptyPeriod(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.MigrateTable(ctx, pool, orchestrator.Migrations(), orchestrator.MigrationsTable); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	got, err := store.New(pool).Summarize(ctx, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("sumar: %v", err)
	}
	if got.Queries != 0 || got.EstimatedCostUSD != 0 || got.ByProvider == nil || len(got.ByProvider) != 0 {
		t.Errorf("got %+v, want todo en cero y una lista vacía", got)
	}
}

// Las consultas nuevas son las que no vinieron de la caché.
func TestCountNew(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.MigrateTable(ctx, pool, orchestrator.Migrations(), orchestrator.MigrationsTable); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	s := store.New(pool)

	today := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	for i, u := range []orchestrator.Usage{
		{At: today.Add(time.Hour), Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered},
		{At: today.Add(2 * time.Hour), Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeNoSources},
		{At: today.Add(3 * time.Hour), Cache: orchestrator.CacheHit, Outcome: orchestrator.OutcomeAnswered},
		{At: today.Add(-time.Hour), Cache: orchestrator.CacheMiss, Outcome: orchestrator.OutcomeAnswered},
	} {
		u.TraceID, u.Provider, u.Model = fmt.Sprint(i), "none", "none"
		if err := s.Record(ctx, u); err != nil {
			t.Fatalf("registrar %d: %v", i, err)
		}
	}

	n, err := s.CountNew(ctx, today)
	if err != nil {
		t.Fatalf("contar: %v", err)
	}
	if n != 2 {
		t.Errorf("got %d consultas nuevas, want 2", n)
	}
}
