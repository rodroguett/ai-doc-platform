// Package store guarda en Postgres el registro de uso de las consultas.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
)

// Store implementa orchestrator.UsageRecorder sobre Postgres.
type Store struct {
	pool *pgxpool.Pool
}

var _ orchestrator.UsageRecorder = (*Store)(nil)

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Record guarda el registro de una consulta.
func (s *Store) Record(ctx context.Context, u orchestrator.Usage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO query_usage (occurred_at, trace_id, collection, provider, model,
			prompt_tokens, output_tokens, cache, outcome, estimated_cost_usd)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		u.At, u.TraceID, u.Collection, u.Provider, u.Model,
		u.PromptTokens, u.OutputTokens, string(u.Cache), string(u.Outcome), u.EstimatedCostUSD)
	if err != nil {
		return fmt.Errorf("registrar el uso de la consulta: %w", err)
	}
	return nil
}
