// Package store guarda en Postgres el registro de uso de las consultas.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
)

// Store implementa orchestrator.UsageStore sobre Postgres.
type Store struct {
	pool *pgxpool.Pool
}

var _ orchestrator.UsageStore = (*Store)(nil)

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

// Summarize suma el registro de uso entre since (incluido) y until
// (excluido). Una consulta sin costo conocido no suma al costo: se cuenta
// aparte.
func (s *Store) Summarize(ctx context.Context, since, until time.Time) (orchestrator.UsageSummary, error) {
	var out orchestrator.UsageSummary
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE cache = 'hit'),
		       coalesce(sum(prompt_tokens + output_tokens), 0),
		       coalesce(sum(estimated_cost_usd), 0),
		       count(*) FILTER (WHERE estimated_cost_usd IS NULL)
		FROM query_usage
		WHERE occurred_at >= $1 AND occurred_at < $2`, since, until).
		Scan(&out.Queries, &out.CacheHits, &out.TokensTotal, &out.EstimatedCostUSD, &out.QueriesWithoutCost)
	if err != nil {
		return orchestrator.UsageSummary{}, fmt.Errorf("sumar el uso: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT provider, count(*), coalesce(sum(estimated_cost_usd), 0)
		FROM query_usage
		WHERE occurred_at >= $1 AND occurred_at < $2
		GROUP BY provider
		ORDER BY provider`, since, until)
	if err != nil {
		return orchestrator.UsageSummary{}, fmt.Errorf("sumar el uso por proveedor: %w", err)
	}
	defer rows.Close()
	out.ByProvider = []orchestrator.ProviderUsage{}
	for rows.Next() {
		var p orchestrator.ProviderUsage
		if err := rows.Scan(&p.Provider, &p.Queries, &p.EstimatedCostUSD); err != nil {
			return orchestrator.UsageSummary{}, fmt.Errorf("leer el uso por proveedor: %w", err)
		}
		out.ByProvider = append(out.ByProvider, p)
	}
	if err := rows.Err(); err != nil {
		return orchestrator.UsageSummary{}, fmt.Errorf("leer el uso por proveedor: %w", err)
	}
	return out, nil
}

// CountNew cuenta las consultas registradas desde since que no vinieron de
// la caché.
func (s *Store) CountNew(ctx context.Context, since time.Time) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM query_usage WHERE occurred_at >= $1 AND cache <> 'hit'`, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("contar las consultas nuevas: %w", err)
	}
	return n, nil
}
