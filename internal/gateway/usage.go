package gateway

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
)

func (a *API) GetUsage(ctx context.Context, req api.GetUsageRequestObject) (api.GetUsageResponseObject, error) {
	if a.svc.Usage == nil {
		return nil, errNoDatabase
	}

	var p orchestrator.UsagePeriod
	if req.Params.From != nil {
		p.From = req.Params.From.Time
	}
	if req.Params.To != nil {
		p.To = req.Params.To.Time
	}

	s, err := a.svc.Usage.Usage(ctx, p)
	if err != nil {
		return nil, err
	}
	return api.GetUsage200JSONResponse(toUsage(s)), nil
}

func toUsage(s orchestrator.UsageSummary) api.Usage {
	out := api.Usage{
		PeriodStart:                openapi_types.Date{Time: s.From},
		PeriodEnd:                  openapi_types.Date{Time: s.To},
		Queries:                    s.Queries,
		TokensTotal:                s.TokensTotal,
		EstimatedCostUsd:           s.EstimatedCostUSD,
		QueriesWithoutCostEstimate: s.QueriesWithoutCost,
	}
	if s.Queries > 0 {
		out.CacheHitRate = float64(s.CacheHits) / float64(s.Queries)
	}
	out.ByProvider = make([]struct {
		EstimatedCostUsd float64 `json:"estimated_cost_usd"`
		Provider         string  `json:"provider"`
		Queries          int     `json:"queries"`
	}, len(s.ByProvider))
	for i, p := range s.ByProvider {
		out.ByProvider[i].Provider = p.Provider
		out.ByProvider[i].Queries = p.Queries
		out.ByProvider[i].EstimatedCostUsd = p.EstimatedCostUSD
	}
	return out
}
