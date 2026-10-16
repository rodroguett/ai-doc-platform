package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
)

// reply es lo que la API falsa responde a un modelo.
type reply struct {
	status int
	text   string
	finish string
	empty  bool // sin candidatos

	thoughts int // tokens de razonamiento que informa el proveedor
}

// fakeGenerateAPI responde a generateContent según el modelo pedido, y anota
// las peticiones.
type fakeGenerateAPI struct {
	t       *testing.T
	replies map[string]reply

	mu     sync.Mutex
	models []string
	bodies []map[string]any
	key    string
}

func (f *fakeGenerateAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// /models/<modelo>:generateContent
	model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/models/"), ":generateContent")
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		f.t.Errorf("cuerpo ilegible: %v", err)
	}

	f.mu.Lock()
	f.models = append(f.models, model)
	f.bodies = append(f.bodies, body)
	f.key = r.Header.Get("x-goog-api-key")
	rep, ok := f.replies[model]
	f.mu.Unlock()

	if !ok {
		rep = reply{status: http.StatusNotFound}
	}
	if rep.status != 0 && rep.status != http.StatusOK {
		w.WriteHeader(rep.status)
		_, _ = w.Write([]byte(`{"error": {"message": "detalle interno del proveedor"}}`))
		return
	}
	if rep.empty {
		_, _ = w.Write([]byte(`{"candidates": [], "usageMetadata": {"promptTokenCount": 10}}`))
		return
	}
	finish := rep.finish
	if finish == "" {
		finish = "STOP"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{"parts": []map[string]any{
				{"text": "razonamiento interno", "thought": true},
				{"text": rep.text},
			}},
			"finishReason": finish,
		}},
		"usageMetadata": map[string]any{"promptTokenCount": 3000, "candidatesTokenCount": 150, "thoughtsTokenCount": rep.thoughts},
	})
}

func newGenerator(t *testing.T, replies map[string]reply, models ...string) (*Generator, *fakeGenerateAPI, *Client) {
	t.Helper()
	api := &fakeGenerateAPI{t: t, replies: replies}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	client, err := New(Config{APIKey: testKey, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("crear el cliente: %v", err)
	}
	g, err := client.Generator(models...)
	if err != nil {
		t.Fatalf("crear el generador: %v", err)
	}
	return g, api, client
}

var request = llmgw.GenerateRequest{System: "Responde solo con los pasajes.", Prompt: "Pasajes: ...\n\nPregunta: ¿Qué pozos?"}

func TestGenerate(t *testing.T) {
	g, api, client := newGenerator(t, map[string]reply{"principal": {text: "  El titular monitorea seis pozos [1].\n"}}, "principal", "respaldo")

	got, err := g.Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("generar: %v", err)
	}

	// El texto llega sin el razonamiento interno del modelo ni espacios.
	if got.Text != "El titular monitorea seis pozos [1]." {
		t.Errorf("got texto %q", got.Text)
	}
	if got.Provider != "gemini" || got.Model != "principal" || got.PromptTokens != 3000 || got.OutputTokens != 150 {
		t.Errorf("procedencia o consumo inesperados: %+v", got)
	}

	if len(api.models) != 1 || api.models[0] != "principal" {
		t.Fatalf("got peticiones a %v, want solo al modelo principal", api.models)
	}
	if api.key != testKey {
		t.Error("la clave no viajó en el encabezado x-goog-api-key")
	}
	body := api.bodies[0]
	system := body["systemInstruction"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"]
	user := body["contents"].([]any)[0].(map[string]any)
	if system != request.System {
		t.Errorf("got instrucciones %q", system)
	}
	// Las instrucciones y el contenido viajan por separado.
	if user["role"] != "user" || user["parts"].([]any)[0].(map[string]any)["text"] != request.Prompt {
		t.Errorf("got contenido %v", user)
	}
	config := body["generationConfig"].(map[string]any)
	if config["temperature"] != float64(0) || config["maxOutputTokens"] != float64(defaultMaxOutputTokens) {
		t.Errorf("got configuración %v", config)
	}

	if usage := client.Usage(); usage.Requests != 1 || usage.Tokens != 3150 {
		t.Errorf("got consumo %+v, want 1 petición y 3150 tokens", usage)
	}
}

// Un modelo saturado, retirado o que no completa su respuesta se salta: el
// siguiente responde.
func TestGenerateFallsBackToNextModel(t *testing.T) {
	for name, failure := range map[string]reply{
		"saturado":             {status: http.StatusServiceUnavailable},
		"cuota agotada":        {status: http.StatusTooManyRequests},
		"retirado":             {status: http.StatusNotFound},
		"respuesta cortada":    {text: "El titular monitorea", finish: "MAX_TOKENS"},
		"bloqueado por filtro": {text: "", finish: "SAFETY"},
		"sin candidatos":       {empty: true},
		"respuesta vacía":      {text: "   "},
	} {
		t.Run(name, func(t *testing.T) {
			g, api, _ := newGenerator(t, map[string]reply{
				"principal": failure,
				"respaldo":  {text: "Respuesta del respaldo [1]."},
			}, "principal", "respaldo")

			got, err := g.Generate(context.Background(), request)
			if err != nil {
				t.Fatalf("generar: %v", err)
			}

			if got.Model != "respaldo" || got.Text != "Respuesta del respaldo [1]." {
				t.Errorf("got %q del modelo %q, want la respuesta del respaldo", got.Text, got.Model)
			}
			// Cada modelo se intenta una sola vez: quien consulta espera.
			if strings.Join(api.models, ",") != "principal,respaldo" {
				t.Errorf("got peticiones a %v", api.models)
			}
		})
	}
}

func TestGenerateUnavailableWhenNoModelAnswers(t *testing.T) {
	g, api, _ := newGenerator(t, map[string]reply{
		"principal": {status: http.StatusServiceUnavailable},
		"respaldo":  {status: http.StatusServiceUnavailable},
	}, "principal", "respaldo")

	_, err := g.Generate(context.Background(), request)

	if !errors.Is(err, llmgw.ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	if len(api.models) != 2 {
		t.Errorf("got %d peticiones, want una por modelo", len(api.models))
	}
	for _, secret := range []string{testKey, "detalle interno del proveedor"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("el error expone %q: %v", secret, err)
		}
	}
}

// Una entrada que el proveedor rechaza no mejora cambiando de modelo.
func TestGenerateDoesNotFallBackOnRejectedInput(t *testing.T) {
	g, api, _ := newGenerator(t, map[string]reply{
		"principal": {status: http.StatusBadRequest},
		"respaldo":  {text: "no debería llegar aquí [1]"},
	}, "principal", "respaldo")

	_, err := g.Generate(context.Background(), request)

	if !errors.Is(err, llmgw.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
	if len(api.models) != 1 {
		t.Errorf("got peticiones a %v, want solo al principal", api.models)
	}
}

func TestGenerateValidatesBeforeCalling(t *testing.T) {
	g, api, client := newGenerator(t, nil, "principal")

	if _, err := g.Generate(context.Background(), llmgw.GenerateRequest{System: "reglas", Prompt: "  "}); !errors.Is(err, llmgw.ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
	if len(api.models) != 0 {
		t.Error("se llamó al proveedor sin contenido")
	}
	if _, err := client.Generator(); !errors.Is(err, llmgw.ErrMisconfigured) {
		t.Errorf("generador sin modelos: got %v, want ErrMisconfigured", err)
	}
}

func TestGenerateStopsWhenContextIsCancelled(t *testing.T) {
	g, api, _ := newGenerator(t, map[string]reply{"respaldo": {text: "tarde [1]"}}, "principal", "respaldo")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := g.Generate(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if len(api.models) > 1 {
		t.Errorf("siguió con el respaldo tras la cancelación: %v", api.models)
	}
}

// El costo se estima con la tarifa de lista del modelo que respondió.
func TestGenerateEstimatesCost(t *testing.T) {
	g, _, _ := newGenerator(t, map[string]reply{ModelFlashLite: {text: "Seis pozos [1]."}}, ModelFlashLite)

	got, err := g.Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("generar: %v", err)
	}

	// 3.000 tokens de entrada a 0,30 USD y 150 de salida a 2,50 USD por millón.
	const want = 0.0009 + 0.000375
	if got.EstimatedCostUSD == nil || math.Abs(*got.EstimatedCostUSD-want) > 1e-12 {
		t.Errorf("got costo %v, want %v", got.EstimatedCostUSD, want)
	}
}

// Los tokens de razonamiento no aparecen en la respuesta, pero se cobran
// como salida.
func TestGenerateCountsThinkingAsOutput(t *testing.T) {
	g, _, client := newGenerator(t, map[string]reply{ModelFlashLite: {text: "Seis pozos [1].", thoughts: 400}}, ModelFlashLite)

	got, err := g.Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("generar: %v", err)
	}

	if got.OutputTokens != 550 {
		t.Errorf("got %d tokens de salida, want 550", got.OutputTokens)
	}
	const want = 0.0009 + 550*2.50/1e6
	if got.EstimatedCostUSD == nil || math.Abs(*got.EstimatedCostUSD-want) > 1e-12 {
		t.Errorf("got costo %v, want %v", got.EstimatedCostUSD, want)
	}
	if usage := client.Usage(); usage.Tokens != 3550 {
		t.Errorf("got %d tokens contados, want 3550", usage.Tokens)
	}
}

// Un modelo sin tarifa conocida no cuesta cero: su costo queda sin estimar.
func TestGenerateLeavesUnknownCostUnset(t *testing.T) {
	g, _, _ := newGenerator(t, map[string]reply{"modelo-nuevo": {text: "Seis pozos [1]."}}, "modelo-nuevo")

	got, err := g.Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("generar: %v", err)
	}
	if got.EstimatedCostUSD != nil {
		t.Errorf("got costo %v, want sin estimar", *got.EstimatedCostUSD)
	}
}

// Cada modelo que el proyecto usa tiene tarifa. Cambiar de modelo sin
// agregarla deja las consultas sin costo estimado.
func TestConfiguredModelsHavePrices(t *testing.T) {
	for _, m := range []string{ModelFlashLite, ModelFlashLiteFallback, model} {
		if _, ok := prices[m]; !ok {
			t.Errorf("el modelo %s no tiene tarifa", m)
		}
	}
}
