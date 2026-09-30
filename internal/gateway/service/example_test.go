package service

import (
	"context"
	"errors"
	"testing"
)

func TestQueryNormalize(t *testing.T) {
	q, err := Query{Question: "  ¿Qué pozos se monitorean?  "}.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.TopK != defaultTopK || q.Trace != TraceSummary {
		t.Errorf("no se aplicaron los valores por omisión: %+v", q)
	}

	invalid := []Query{
		{Question: "ab"},
		{Question: "¿Qué pozos?", TopK: maxTopK + 1},
		{Question: "¿Qué pozos?", TopK: -1},
		{Question: "¿Qué pozos?", Trace: "debug"},
	}
	for _, q := range invalid {
		if _, err := q.Normalize(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%+v: got %v, want ErrInvalidInput", q, err)
		}
	}
}

func TestExampleQueriesRespectsTopK(t *testing.T) {
	ans, err := ExampleQueries{}.Answer(context.Background(), Query{Question: "¿Qué pozos?", TopK: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ans.Citations) != 1 {
		t.Errorf("got %d citations, want 1", len(ans.Citations))
	}
	if ans.Trace.Retrieved != nil {
		t.Error("trace=summary no debe incluir los fragmentos recuperados")
	}
}

func TestExampleQueriesWithoutSourcesHasNoCitations(t *testing.T) {
	ans, err := ExampleQueries{}.Answer(context.Background(),
		Query{Question: "¿Qué pozos?", Collection: "normativa"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ans.Citations) != 0 {
		t.Errorf("got %d citations, want 0", len(ans.Citations))
	}
}
