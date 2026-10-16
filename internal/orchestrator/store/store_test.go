package store_test

import (
	"context"
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
