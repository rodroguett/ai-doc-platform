// Package cloudtasks encola tareas HTTP en Cloud Tasks.
//
// Cloud Run solo asigna CPU mientras atiende una petición. Para trabajar
// fuera de la petición que origina el trabajo, se le pide a Cloud Tasks que
// haga otra petición más tarde: el trabajo ocurre dentro de ella (ADR-0010).
//
// Habla con la API por HTTP, sin el SDK de Google, por la misma razón que el
// cliente de Gemini: es una sola llamada.
package cloudtasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://cloudtasks.googleapis.com/v2"

	// metadataEndpoint entrega, dentro de Google Cloud, un token de acceso
	// de la cuenta de servicio con la que corre el proceso.
	metadataEndpoint = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

	requestTimeout = 15 * time.Second

	// tokenMargin es cuánto antes de su vencimiento se renueva un token.
	tokenMargin = time.Minute
)

// TokenSource entrega el token con el que se autoriza la llamada a la API.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// Task es una petición HTTP que Cloud Tasks hará más tarde.
type Task struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

// Client encola tareas en una cola. Es seguro para uso concurrente.
type Client struct {
	queue   string
	baseURL string
	http    *http.Client
	tokens  TokenSource
}

// New crea un cliente para la cola indicada, con el nombre completo que le
// da Cloud Tasks: projects/P/locations/L/queues/Q. Se autoriza con la cuenta
// de servicio del entorno.
func New(queue string) (*Client, error) {
	httpClient := &http.Client{Timeout: requestTimeout}
	return NewWithTokenSource(queue, &metadataTokens{url: metadataEndpoint, http: httpClient, now: time.Now})
}

// NewWithTokenSource crea un cliente que se autoriza con tokens.
func NewWithTokenSource(queue string, tokens TokenSource) (*Client, error) {
	parts := strings.Split(queue, "/")
	if len(parts) != 6 || parts[0] != "projects" || parts[2] != "locations" || parts[4] != "queues" {
		return nil, fmt.Errorf("el nombre de la cola debe tener la forma projects/P/locations/L/queues/Q")
	}
	for _, p := range []string{parts[1], parts[3], parts[5]} {
		if p == "" {
			return nil, fmt.Errorf("el nombre de la cola tiene una parte vacía")
		}
	}
	return &Client{
		queue:   queue,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: requestTimeout},
		tokens:  tokens,
	}, nil
}

// Enqueue agrega la tarea a la cola.
func (c *Client) Enqueue(ctx context.Context, t Task) error {
	type httpRequest struct {
		URL        string            `json:"url"`
		HTTPMethod string            `json:"httpMethod"`
		Headers    map[string]string `json:"headers,omitempty"`
		Body       []byte            `json:"body,omitempty"` // JSON lo codifica en base64, como pide la API
	}
	payload, err := json.Marshal(map[string]any{
		"task": map[string]any{
			"httpRequest": httpRequest{URL: t.URL, HTTPMethod: http.MethodPost, Headers: t.Headers, Body: t.Body},
		},
	})
	if err != nil {
		return fmt.Errorf("armar la tarea: %w", err)
	}

	token, err := c.tokens.Token(ctx)
	if err != nil {
		return fmt.Errorf("obtener el token de acceso: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+c.queue+"/tasks", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("armar la petición: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("no se pudo contactar a Cloud Tasks")
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// El cuerpo no se propaga: las tareas llevan encabezados con
		// credenciales y la API puede repetirlos en sus errores.
		return fmt.Errorf("la API de Cloud Tasks respondió %d", resp.StatusCode)
	}
	return nil
}

// metadataTokens obtiene tokens del servidor de metadatos y los reutiliza
// hasta poco antes de que venzan.
type metadataTokens struct {
	url  string
	http *http.Client
	now  func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

func (m *metadataTokens) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token != "" && m.now().Before(m.expires.Add(-tokenMargin)) {
		return m.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := m.http.Do(req)
	if err != nil {
		return "", errors.New("no se pudo contactar al servidor de metadatos; ¿el proceso corre fuera de Google Cloud?")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el servidor de metadatos respondió %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil || body.AccessToken == "" {
		return "", errors.New("el servidor de metadatos no entregó un token")
	}
	m.token = body.AccessToken
	m.expires = m.now().Add(time.Duration(body.ExpiresIn) * time.Second)
	return m.token, nil
}
