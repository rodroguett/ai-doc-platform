// Package gemini implementa llmgw.Embedder sobre la API de Gemini.
//
// Habla con la API por HTTP, sin el SDK de Google: son dos llamadas, y el
// SDK arrastraría dependencias sujetas al techo de versión de Go.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
)

const (
	// model y dimensions los fija ADR-0008.
	model      = "gemini-embedding-001"
	dimensions = 768

	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

	// defaultTokensPerMinute deja margen bajo los 30.000 de la capa
	// gratuita: la estimación de tokens no es exacta, y exceder la cuota
	// cuesta más que ir un poco más lento.
	defaultTokensPerMinute = 24_000

	// Una petición por lotes no lleva más de estos textos ni de estos
	// tokens estimados. Lotes chicos hacen que el ritmo se regule de a
	// poco, en lugar de gastar el minuto entero de una vez.
	maxBatchTexts  = 16
	maxBatchTokens = 6_000

	defaultMaxAttempts = 5

	// maxForbiddenRetries: la API devolvió 403 de forma intermitente con
	// una clave válida (ADR-0008). Se reintenta poco, porque un 403
	// sostenido es una clave mala y no se arregla insistiendo.
	maxForbiddenRetries = 2

	baseBackoff = time.Second
	maxBackoff  = 30 * time.Second

	requestTimeout = 60 * time.Second

	// bytesPerToken estima los tokens de un texto. Medido sobre el corpus,
	// un token son unos 3,3 caracteres; dividir los bytes por 3 sobreestima,
	// que es el lado seguro.
	bytesPerToken = 3
)

// Config configura el cliente.
type Config struct {
	APIKey string

	// BaseURL permite apuntar a un servidor de pruebas.
	BaseURL string

	// TokensPerMinute es el ritmo máximo. Cero usa el valor por omisión.
	TokensPerMinute int

	HTTPClient *http.Client
}

// Client genera embeddings con Gemini. Es seguro para uso concurrente.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
	pacer   *pacer

	maxAttempts int
	sleep       func(ctx context.Context, d time.Duration) error
	jitter      func() float64

	requests atomic.Int64
	tokens   atomic.Int64
}

var _ llmgw.Embedder = (*Client)(nil)

// New crea el cliente. Falla si no hay clave: un cliente sin ella no puede
// hacer nada, y es mejor saberlo al arrancar que en la primera ingesta.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("%w: falta la clave de la API de Gemini", llmgw.ErrMisconfigured)
	}
	c := &Client{
		apiKey:      strings.TrimSpace(cfg.APIKey),
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		http:        cfg.HTTPClient,
		maxAttempts: defaultMaxAttempts,
		sleep:       sleepContext,
		jitter:      rand.Float64,
	}
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: requestTimeout}
	}
	tpm := cfg.TokensPerMinute
	if tpm <= 0 {
		tpm = defaultTokensPerMinute
	}
	c.pacer = newPacer(tpm, time.Now, c.sleepFn)
	return c, nil
}

// sleepFn permite que los tests reemplacen sleep después de construir.
func (c *Client) sleepFn(ctx context.Context, d time.Duration) error { return c.sleep(ctx, d) }

func (c *Client) Model() string   { return model }
func (c *Client) Dimensions() int { return dimensions }

// Usage devuelve el consumo acumulado.
func (c *Client) Usage() llmgw.Usage {
	return llmgw.Usage{Requests: c.requests.Load(), Tokens: c.tokens.Load()}
}

type part struct {
	Text string `json:"text"`
}

type content struct {
	Parts []part `json:"parts"`
}

type embedRequest struct {
	Model                string  `json:"model"`
	Content              content `json:"content"`
	TaskType             string  `json:"taskType"`
	OutputDimensionality int     `json:"outputDimensionality"`
}

type embedding struct {
	Values []float32 `json:"values"`
}

func newEmbedRequest(text, taskType string) embedRequest {
	return embedRequest{
		Model:                "models/" + model,
		Content:              content{Parts: []part{{Text: text}}},
		TaskType:             taskType,
		OutputDimensionality: dimensions,
	}
}

// EmbedDocuments genera los vectores en lotes, regulando el ritmo. Si un
// lote falla devuelve el error sin los vectores de los lotes anteriores:
// quien necesite conservar el avance debe llamar con grupos pequeños.
func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, batch := range batches(texts) {
		reqs := make([]embedRequest, len(batch))
		tokens := 0
		for i, text := range batch {
			if strings.TrimSpace(text) == "" {
				return nil, fmt.Errorf("%w: un texto está vacío", llmgw.ErrInvalidInput)
			}
			reqs[i] = newEmbedRequest(text, "RETRIEVAL_DOCUMENT")
			tokens += estimateTokens(text)
		}

		var resp struct {
			Embeddings []embedding `json:"embeddings"`
		}
		if err := c.call(ctx, "batchEmbedContents", map[string]any{"requests": reqs}, tokens, &resp); err != nil {
			return nil, err
		}
		if len(resp.Embeddings) != len(batch) {
			return nil, fmt.Errorf("el proveedor devolvió %d vectores para %d textos", len(resp.Embeddings), len(batch))
		}
		for _, e := range resp.Embeddings {
			v, err := normalize(e.Values)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	}
	return out, nil
}

func (c *Client) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: la consulta está vacía", llmgw.ErrInvalidInput)
	}
	var resp struct {
		Embedding embedding `json:"embedding"`
	}
	if err := c.call(ctx, "embedContent", newEmbedRequest(text, "RETRIEVAL_QUERY"), estimateTokens(text), &resp); err != nil {
		return nil, err
	}
	return normalize(resp.Embedding.Values)
}

// batches agrupa los textos sin exceder la cantidad ni los tokens por lote.
// Un texto que por sí solo excede los tokens va en un lote propio.
func batches(texts []string) [][]string {
	var out [][]string
	var cur []string
	tokens := 0
	for _, text := range texts {
		t := estimateTokens(text)
		if len(cur) > 0 && (len(cur) == maxBatchTexts || tokens+t > maxBatchTokens) {
			out = append(out, cur)
			cur, tokens = nil, 0
		}
		cur = append(cur, text)
		tokens += t
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func estimateTokens(text string) int {
	return (len(text) + bytesPerToken - 1) / bytesPerToken
}

// normalize lleva el vector a norma 1. El modelo solo normaliza los de su
// dimensión nativa; los truncados llegan con norma menor (ADR-0008), y sin
// normalizar las distancias entre ellos no serían comparables.
func normalize(v []float32) ([]float32, error) {
	if len(v) != dimensions {
		return nil, fmt.Errorf("el proveedor devolvió un vector de %d dimensiones, se esperaban %d", len(v), dimensions)
	}
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := math.Sqrt(sum)
	if norm == 0 || math.IsNaN(norm) || math.IsInf(norm, 0) {
		return nil, errors.New("el proveedor devolvió un vector sin magnitud")
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out, nil
}

// call envía una petición, regulando el ritmo y reintentando las fallas
// transitorias con espera creciente.
func (c *Client) call(ctx context.Context, method string, body any, tokens int, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("armar la petición: %w", err)
	}

	forbidden := 0
	var last error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		// Cada intento descuenta de la cuota, también los que fallan.
		if err := c.pacer.wait(ctx, tokens); err != nil {
			return err
		}

		retryAfter, err := c.send(ctx, method, payload, out)
		if err == nil {
			c.tokens.Add(int64(tokens))
			return nil
		}
		last = err

		var transient *transientError
		switch {
		case errors.As(err, &transient):
			if transient.forbidden {
				forbidden++
				if forbidden > maxForbiddenRetries {
					return fmt.Errorf("%w: el proveedor rechaza la clave (403 sostenido)", llmgw.ErrMisconfigured)
				}
			}
		default:
			return err // permanente, o el contexto terminó
		}

		if attempt == c.maxAttempts {
			break
		}
		if err := c.sleep(ctx, c.backoff(attempt, retryAfter)); err != nil {
			return err
		}
	}
	return fmt.Errorf("%w: %d intentos fallidos; el último: %w", llmgw.ErrUnavailable, c.maxAttempts, last)
}

// backoff es la espera antes del siguiente intento: la que pide el
// proveedor si la indica, o una que se duplica en cada intento, con una
// variación aleatoria para que varios clientes no reintenten a la vez.
func (c *Client) backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, 2*maxBackoff)
	}
	d := baseBackoff << (attempt - 1)
	d = min(d, maxBackoff)
	return d/2 + time.Duration(c.jitter()*float64(d/2))
}

// transientError es una falla que puede no repetirse.
type transientError struct {
	status    int
	forbidden bool
	cause     error
}

func (e *transientError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return "el proveedor respondió " + strconv.Itoa(e.status)
}

// send hace un intento. Devuelve la espera que el proveedor pide antes de
// reintentar, si la indica.
func (c *Client) send(ctx context.Context, method string, payload []byte, out any) (time.Duration, error) {
	url := c.baseURL + "/models/" + model + ":" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("armar la petición: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// La clave va en un encabezado y no en la URL, para que no aparezca en
	// registros ni en mensajes de error.
	req.Header.Set("x-goog-api-key", c.apiKey)

	c.requests.Add(1)
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		// Una falla de red o un plazo vencido. No se propaga el error
		// original: podría citar la URL completa.
		return 0, &transientError{cause: errors.New("no se pudo contactar al proveedor")}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return 0, &transientError{cause: errors.New("la respuesta del proveedor no se pudo leer")}
		}
		return 0, nil
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

	switch {
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return retryAfter, &transientError{status: resp.StatusCode}
	case resp.StatusCode == http.StatusForbidden:
		return retryAfter, &transientError{status: resp.StatusCode, forbidden: true}
	case resp.StatusCode == http.StatusBadRequest:
		return 0, fmt.Errorf("%w (400)", llmgw.ErrInvalidInput)
	default:
		// 401, 404 y similares: la clave o el modelo están mal.
		return 0, fmt.Errorf("%w: el proveedor respondió %d", llmgw.ErrMisconfigured, resp.StatusCode)
	}
}

func parseRetryAfter(v string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pacer regula el ritmo con un cubo de fichas: una por token, que se
// recargan de forma continua hasta el máximo de un minuto.
type pacer struct {
	mu        sync.Mutex
	capacity  float64
	perSecond float64
	available float64
	updated   time.Time
	now       func() time.Time
	sleep     func(ctx context.Context, d time.Duration) error
}

func newPacer(tokensPerMinute int, now func() time.Time, sleep func(context.Context, time.Duration) error) *pacer {
	return &pacer{
		capacity:  float64(tokensPerMinute),
		perSecond: float64(tokensPerMinute) / 60,
		available: float64(tokensPerMinute),
		updated:   now(),
		now:       now,
		sleep:     sleep,
	}
}

// wait espera hasta que haya fichas para la petición y las descuenta. Una
// petición más grande que el cubo se trata como una del tamaño del cubo: de
// lo contrario nunca pasaría.
func (p *pacer) wait(ctx context.Context, tokens int) error {
	need := math.Min(float64(tokens), p.capacity)

	p.mu.Lock()
	now := p.now()
	p.available = math.Min(p.capacity, p.available+now.Sub(p.updated).Seconds()*p.perSecond)
	p.updated = now

	// Se descuenta antes de esperar: así dos peticiones simultáneas no
	// cuentan con las mismas fichas. El saldo puede quedar negativo, y eso
	// es justamente la espera de la siguiente.
	p.available -= need
	deficit := -p.available
	p.mu.Unlock()

	if deficit <= 0 {
		return ctx.Err()
	}
	return p.sleep(ctx, time.Duration(deficit/p.perSecond*float64(time.Second)))
}
