package gemini

import (
	"context"
	"math"
	"os"
	"testing"
	"time"
)

// TestLive llama a la API real. Consume cuota, así que solo corre si se pide:
//
//	GEMINI_LIVE_TEST=1 GEMINI_API_KEY=... go test -run TestLive ./internal/llmgw/gemini/
//
// Verifica lo que el servidor falso no puede: que la API real acepta las
// peticiones tal como este cliente las arma.
func TestLive(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("GEMINI_LIVE_TEST no está definida; el test consume cuota real")
	}
	client, err := New(Config{APIKey: os.Getenv("GEMINI_API_KEY")})
	if err != nil {
		t.Fatalf("crear el cliente: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	docs, err := client.EmbedDocuments(ctx, []string{
		"El Titular deberá monitorear trimestralmente el nivel freático en los pozos PM-01 a PM-06.",
		"El plan de cierre contempla el desmantelamiento de la planta de procesamiento.",
	})
	if err != nil {
		t.Fatalf("embeddings de documentos: %v", err)
	}
	query, err := client.EmbedQuery(ctx, "¿Con qué frecuencia se monitorea el agua subterránea?")
	if err != nil {
		t.Fatalf("embedding de la consulta: %v", err)
	}

	dot := func(a, b []float32) float64 {
		var sum float64
		for i := range a {
			sum += float64(a[i]) * float64(b[i])
		}
		return sum
	}
	for i, v := range append(docs, query) {
		if len(v) != dimensions {
			t.Fatalf("vector %d: %d dimensiones, want %d", i, len(v), dimensions)
		}
		if n := math.Sqrt(dot(v, v)); math.Abs(n-1) > 1e-4 {
			t.Errorf("vector %d: norma %f, want 1", i, n)
		}
	}

	// Con vectores normalizados, el producto punto es la similitud coseno.
	related, unrelated := dot(query, docs[0]), dot(query, docs[1])
	t.Logf("similitud con el pasaje relacionado: %.3f; con el no relacionado: %.3f", related, unrelated)
	if related <= unrelated {
		t.Errorf("la consulta se parece más al pasaje no relacionado (%.3f) que al relacionado (%.3f)", unrelated, related)
	}
	t.Logf("consumo: %+v", client.Usage())
}
