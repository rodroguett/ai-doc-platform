package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
)

const (
	provider = "gemini"

	// Los modelos de generación los fija ADR-0012: el primero es el único de
	// la capa gratuita que respondió en todas las mediciones; el segundo, el
	// respaldo cuando el primero está saturado.
	ModelFlashLite         = "gemini-3.5-flash-lite"
	ModelFlashLiteFallback = "gemini-3.1-flash-lite"

	// generateTimeout es el plazo de cada intento. Una generación tarda uno
	// o dos segundos; esperar mucho más por un modelo saturado retrasa el
	// respaldo sin mejorar el resultado.
	generateTimeout = 20 * time.Second

	defaultMaxOutputTokens = 1024

	// finishStop es el motivo de término de una respuesta completa.
	finishStop = "STOP"
)

// Generator redacta texto con Gemini, probando sus modelos en orden.
type Generator struct {
	client *Client
	models []string
}

var _ llmgw.Generator = (*Generator)(nil)

// Generator devuelve un generador que usa los modelos indicados, en orden:
// si uno no responde, se intenta con el siguiente.
func (c *Client) Generator(models ...string) (*Generator, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("%w: el generador necesita al menos un modelo", llmgw.ErrMisconfigured)
	}
	return &Generator{client: c, models: models}, nil
}

// Generate redacta con el primer modelo que responda.
//
// Cada modelo se intenta una sola vez, sin esperas: quien consulta está
// esperando, y otro modelo responde antes que un reintento sobre uno
// saturado. Un modelo que no responde, que fue retirado o que no completó
// su respuesta se salta. Si ninguno responde devuelve llmgw.ErrUnavailable.
func (g *Generator) Generate(ctx context.Context, req llmgw.GenerateRequest) (llmgw.Generation, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return llmgw.Generation{}, fmt.Errorf("%w: no hay contenido sobre el cual redactar", llmgw.ErrInvalidInput)
	}
	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxOutputTokens
	}

	body := map[string]any{
		"contents": []content{{Role: "user", Parts: []part{{Text: req.Prompt}}}},
		// Temperatura cero: la misma pregunta sobre los mismos pasajes debe
		// dar, en lo posible, la misma respuesta.
		"generationConfig": map[string]any{"temperature": 0, "maxOutputTokens": maxTokens},
	}
	if strings.TrimSpace(req.System) != "" {
		body["systemInstruction"] = content{Parts: []part{{Text: req.System}}}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return llmgw.Generation{}, fmt.Errorf("armar la petición: %w", err)
	}

	var last error
	for _, model := range g.models {
		generation, err := g.generate(ctx, model, payload)
		if err == nil {
			return generation, nil
		}
		if ctx.Err() != nil {
			return llmgw.Generation{}, ctx.Err()
		}
		// La entrada rechazada no mejora cambiando de modelo.
		if errors.Is(err, llmgw.ErrInvalidInput) {
			return llmgw.Generation{}, err
		}
		slog.WarnContext(ctx, "el modelo de generación no respondió", "modelo", model, "error", err)
		last = err
	}
	return llmgw.Generation{}, fmt.Errorf("%w: ningún modelo de generación respondió; el último: %w", llmgw.ErrUnavailable, last)
}

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// generate hace un intento con un modelo.
func (g *Generator) generate(ctx context.Context, model string, payload []byte) (llmgw.Generation, error) {
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()

	var resp generateResponse
	if _, err := g.client.send(ctx, model, "generateContent", payload, &resp); err != nil {
		return llmgw.Generation{}, err
	}
	g.client.tokens.Add(int64(resp.UsageMetadata.PromptTokenCount + resp.UsageMetadata.CandidatesTokenCount))

	if len(resp.Candidates) == 0 {
		return llmgw.Generation{}, errors.New("el modelo no entregó una respuesta")
	}
	candidate := resp.Candidates[0]
	// Una respuesta cortada por largo o bloqueada por un filtro está
	// incompleta: entregarla sería entregar media respuesta como entera.
	if candidate.FinishReason != finishStop {
		return llmgw.Generation{}, fmt.Errorf("el modelo no completó su respuesta (%s)", candidate.FinishReason)
	}
	var text strings.Builder
	for _, p := range candidate.Content.Parts {
		if !p.Thought {
			text.WriteString(p.Text)
		}
	}
	out := strings.TrimSpace(text.String())
	if out == "" {
		return llmgw.Generation{}, errors.New("el modelo entregó una respuesta vacía")
	}

	return llmgw.Generation{
		Text:         out,
		Provider:     provider,
		Model:        model,
		PromptTokens: resp.UsageMetadata.PromptTokenCount,
		OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
	}, nil
}
