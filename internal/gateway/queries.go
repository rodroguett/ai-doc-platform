package gateway

import (
	"context"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
)

func (a *API) CreateQuery(ctx context.Context, req api.CreateQueryRequestObject) (api.CreateQueryResponseObject, error) {
	if req.Body == nil {
		return nil, invalidRequest("falta el cuerpo de la consulta")
	}

	ans, err := a.svc.Queries.Answer(ctx, toQuery(*req.Body))
	if err != nil {
		return nil, err
	}

	return api.CreateQuery200JSONResponse{
		Body: toQueryResponse(ans),
		Headers: api.CreateQuery200ResponseHeaders{
			XTraceId: &ans.Trace.ID,
		},
	}, nil
}

func toQuery(r api.QueryRequest) service.Query {
	q := service.Query{Question: r.Question}
	if r.Collection != nil {
		q.Collection = *r.Collection
	}
	if r.TopK != nil {
		q.TopK = *r.TopK
	}
	if r.Filters != nil {
		q.Filters = *r.Filters
	}
	if r.Trace != nil {
		q.Trace = service.TraceLevel(*r.Trace)
	}
	return q
}

func toQueryResponse(a service.Answer) api.QueryResponse {
	resp := api.QueryResponse{
		QueryId:   a.ID,
		Answer:    a.Text,
		Citations: make([]api.Citation, 0, len(a.Citations)),
		Trace:     toTrace(a.Trace),
	}
	for _, c := range a.Citations {
		resp.Citations = append(resp.Citations, api.Citation{
			ChunkId:    c.ChunkID,
			DocumentId: c.DocumentID,
			Source:     c.Source,
			Locator:    optional(c.Locator),
			Score:      float32(c.Score),
			Excerpt:    optional(c.Excerpt),
		})
	}
	return resp
}

func toTrace(t service.Trace) api.Trace {
	out := api.Trace{
		TraceId:  t.ID,
		Provider: t.Provider,
		Model:    t.Model,
		Cache:    api.TraceCache(t.Cache),
		Degraded: &t.Degraded,
		CostUsd:  ptr(float32(t.CostUSD)),
	}
	out.Tokens = &struct {
		Completion *int `json:"completion,omitempty"`
		Prompt     *int `json:"prompt,omitempty"`
	}{
		Completion: &t.Tokens.Completion,
		Prompt:     &t.Tokens.Prompt,
	}
	out.LatencyMs.Total = ptr(int(t.Latency.Total.Milliseconds()))
	out.LatencyMs.Retrieval = ptr(int(t.Latency.Retrieval.Milliseconds()))
	out.LatencyMs.Generation = ptr(int(t.Latency.Generation.Milliseconds()))

	if len(t.Retrieved) > 0 {
		retrieved := make([]struct {
			ChunkId *uuid.UUID `json:"chunk_id,omitempty"`
			Rank    *int       `json:"rank,omitempty"`
			Score   *float32   `json:"score,omitempty"`
		}, len(t.Retrieved))
		for i, r := range t.Retrieved {
			retrieved[i].ChunkId = &r.ChunkID
			retrieved[i].Rank = &r.Rank
			retrieved[i].Score = ptr(float32(r.Score))
		}
		out.Retrieved = &retrieved
	}
	return out
}

func ptr[T any](v T) *T { return &v }

// optional devuelve nil para la cadena vacía, de modo que los campos
// opcionales sin valor se omitan en lugar de viajar vacíos.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
