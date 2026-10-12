package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
)

const testKey = "clave-de-prueba-1234567890"

// fakeAPI imita la API de Gemini y registra lo que recibe.
type fakeAPI struct {
	t *testing.T

	mu       sync.Mutex
	requests []recorded
	// statuses son las respuestas a dar, en orden; agotadas, responde 200.
	statuses   []int
	retryAfter string
	// vector es lo que devuelve por cada texto; nil usa uno sin normalizar.
	vector func(text string) []float32
}

type recorded struct {
	path  string
	key   string
	texts []string
	tasks []string
	dims  []int
	model []string
}

func (f *fakeAPI) calls() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recorded(nil), f.requests...)
}

// rawVector devuelve un vector sin normalizar, como los que entrega el
// modelo al truncar: 3 en la primera componente, 4 en la segunda. Su norma
// es 5.
func rawVector(string) []float32 {
	v := make([]float32, dimensions)
	v[0], v[1] = 3, 4
	return v
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		embedRequest
		Requests []embedRequest `json:"requests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		f.t.Errorf("cuerpo ilegible: %v", err)
	}
	reqs := body.Requests
	batch := strings.HasSuffix(r.URL.Path, ":batchEmbedContents")
	if !batch {
		reqs = []embedRequest{body.embedRequest}
	}

	rec := recorded{path: r.URL.Path, key: r.Header.Get("x-goog-api-key")}
	for _, q := range reqs {
		rec.texts = append(rec.texts, q.Content.Parts[0].Text)
		rec.tasks = append(rec.tasks, q.TaskType)
		rec.dims = append(rec.dims, q.OutputDimensionality)
		rec.model = append(rec.model, q.Model)
	}

	f.mu.Lock()
	f.requests = append(f.requests, rec)
	status := http.StatusOK
	if len(f.statuses) > 0 {
		status, f.statuses = f.statuses[0], f.statuses[1:]
	}
	vector := f.vector
	f.mu.Unlock()

	if status != http.StatusOK {
		if f.retryAfter != "" {
			w.Header().Set("Retry-After", f.retryAfter)
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"error": {"code": %d, "message": "detalle interno del proveedor", "status": "X"}}`, status)
		return
	}
	if vector == nil {
		vector = rawVector
	}
	if batch {
		out := make([]embedding, len(reqs))
		for i, q := range reqs {
			out[i] = embedding{Values: vector(q.Content.Parts[0].Text)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": out})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"embedding": embedding{Values: vector(rec.texts[0])}})
}

// harness arma un cliente contra la API falsa, con las esperas registradas
// en lugar de ejecutadas.
type harness struct {
	client *Client
	api    *fakeAPI
	sleeps *[]time.Duration
	clock  *time.Time
}

func newHarness(t *testing.T, statuses ...int) harness {
	t.Helper()
	api := &fakeAPI{t: t, statuses: statuses}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	client, err := New(Config{APIKey: testKey, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("crear el cliente: %v", err)
	}

	var sleeps []time.Duration
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	// Dormir avanza el reloj: el ritmo se comporta como si el tiempo pasara.
	client.sleep = func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		clock = clock.Add(d)
		return ctx.Err()
	}
	client.jitter = func() float64 { return 1 } // la espera máxima de cada intento
	client.pacer = newPacer(defaultTokensPerMinute, func() time.Time { return clock }, client.sleepFn)

	return harness{client: client, api: api, sleeps: &sleeps, clock: &clock}
}

func norm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

func TestNewRequiresKey(t *testing.T) {
	for _, key := range []string{"", "   "} {
		if _, err := New(Config{APIKey: key}); !errors.Is(err, llmgw.ErrMisconfigured) {
			t.Errorf("clave %q: got %v, want ErrMisconfigured", key, err)
		}
	}
}

func TestEmbedDocuments(t *testing.T) {
	h := newHarness(t)

	vectors, err := h.client.EmbedDocuments(context.Background(), []string{"primer pasaje", "segundo pasaje"})
	if err != nil {
		t.Fatalf("embeddings: %v", err)
	}

	if len(vectors) != 2 {
		t.Fatalf("got %d vectores, want 2", len(vectors))
	}
	for i, v := range vectors {
		if len(v) != dimensions {
			t.Errorf("vector %d: %d dimensiones, want %d", i, len(v), dimensions)
		}
		// El modelo entrega norma 5; debe salir normalizado.
		if n := norm(v); math.Abs(n-1) > 1e-6 {
			t.Errorf("vector %d: norma %f, want 1", i, n)
		}
		if math.Abs(float64(v[0])-0.6) > 1e-6 || math.Abs(float64(v[1])-0.8) > 1e-6 {
			t.Errorf("vector %d: la normalización cambió la dirección: %v", i, v[:2])
		}
	}

	calls := h.api.calls()
	if len(calls) != 1 {
		t.Fatalf("got %d peticiones, want 1 por lotes", len(calls))
	}
	c := calls[0]
	if c.path != "/models/gemini-embedding-001:batchEmbedContents" {
		t.Errorf("got ruta %q", c.path)
	}
	if c.key != testKey {
		t.Error("la clave no viajó en el encabezado x-goog-api-key")
	}
	if strings.Join(c.texts, "|") != "primer pasaje|segundo pasaje" {
		t.Errorf("los textos llegaron en otro orden: %v", c.texts)
	}
	for i := range c.texts {
		if c.tasks[i] != "RETRIEVAL_DOCUMENT" || c.dims[i] != 768 || c.model[i] != "models/gemini-embedding-001" {
			t.Errorf("texto %d: tarea %q, dimensión %d, modelo %q", i, c.tasks[i], c.dims[i], c.model[i])
		}
	}
}

func TestEmbedQuery(t *testing.T) {
	h := newHarness(t)

	v, err := h.client.EmbedQuery(context.Background(), "¿Con qué frecuencia se monitorea?")
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if n := norm(v); math.Abs(n-1) > 1e-6 {
		t.Errorf("norma %f, want 1", n)
	}

	c := h.api.calls()[0]
	if c.path != "/models/gemini-embedding-001:embedContent" || c.tasks[0] != "RETRIEVAL_QUERY" || c.dims[0] != 768 {
		t.Errorf("petición inesperada: %+v", c)
	}
}

func TestEmbedDocumentsKeepsOrderAcrossBatches(t *testing.T) {
	h := newHarness(t)
	// Cada vector identifica a su texto: la primera componente es su número.
	h.api.vector = func(text string) []float32 {
		v := make([]float32, dimensions)
		var n int
		if _, err := fmt.Sscanf(text, "pasaje %d", &n); err != nil {
			t.Errorf("texto inesperado %q: %v", text, err)
		}
		v[0], v[1] = float32(n+1), 1
		return v
	}
	texts := make([]string, maxBatchTexts*2+3)
	for i := range texts {
		texts[i] = fmt.Sprintf("pasaje %d", i)
	}

	vectors, err := h.client.EmbedDocuments(context.Background(), texts)
	if err != nil {
		t.Fatalf("embeddings: %v", err)
	}

	calls := h.api.calls()
	if len(calls) != 3 {
		t.Fatalf("got %d peticiones, want 3 lotes", len(calls))
	}
	for i, c := range calls {
		if len(c.texts) > maxBatchTexts {
			t.Errorf("lote %d con %d textos, sobre el máximo de %d", i, len(c.texts), maxBatchTexts)
		}
	}
	for i, v := range vectors {
		// tangente del ángulo: componente 0 sobre componente 1.
		if got := math.Round(float64(v[0] / v[1])); got != float64(i+1) {
			t.Fatalf("el vector %d corresponde al texto %v", i, got-1)
		}
	}
}

func TestBatchesRespectTokenBudget(t *testing.T) {
	big := strings.Repeat("a", maxBatchTokens*bytesPerToken/2+bytesPerToken) // algo más de medio lote
	huge := strings.Repeat("b", maxBatchTokens*bytesPerToken*2)              // excede un lote por sí solo

	got := batches([]string{big, big, "chico", huge, "otro"})

	sizes := make([]int, len(got))
	for i, b := range got {
		sizes[i] = len(b)
	}
	// big | big+chico | huge | otro
	if fmt.Sprint(sizes) != "[1 2 1 1]" {
		t.Errorf("got lotes de %v textos, want [1 2 1 1]", sizes)
	}
}

func TestRejectsEmptyTextWithoutCallingTheAPI(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.client.EmbedDocuments(ctx, []string{"bien", "  "}); !errors.Is(err, llmgw.ErrInvalidInput) {
		t.Errorf("documentos: got %v, want ErrInvalidInput", err)
	}
	if _, err := h.client.EmbedQuery(ctx, ""); !errors.Is(err, llmgw.ErrInvalidInput) {
		t.Errorf("consulta: got %v, want ErrInvalidInput", err)
	}
	if n := len(h.api.calls()); n != 0 {
		t.Errorf("se enviaron %d peticiones con una entrada inválida", n)
	}
}

func TestRetriesTransientFailures(t *testing.T) {
	for name, statuses := range map[string][]int{
		"cuota por minuto":   {429, 429},
		"error del servidor": {500, 503},
		"403 intermitente":   {403},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, statuses...)

			if _, err := h.client.EmbedQuery(context.Background(), "consulta"); err != nil {
				t.Fatalf("got %v, want éxito tras reintentar", err)
			}
			if got, want := len(h.api.calls()), len(statuses)+1; got != want {
				t.Errorf("got %d peticiones, want %d", got, want)
			}
		})
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	h := newHarness(t, 500, 500, 500, 500)

	if _, err := h.client.EmbedQuery(context.Background(), "consulta"); err != nil {
		t.Fatalf("got %v, want éxito al quinto intento", err)
	}
	// Con la variación al máximo, la espera es la base duplicada cada vez.
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	if fmt.Sprint(*h.sleeps) != fmt.Sprint(want) {
		t.Errorf("got esperas %v, want %v", *h.sleeps, want)
	}

	if got := h.client.backoff(20, 0); got != maxBackoff {
		t.Errorf("intento 20: espera %v, want el tope de %v", got, maxBackoff)
	}
}

func TestHonoursRetryAfter(t *testing.T) {
	h := newHarness(t, 429)
	h.api.retryAfter = "17"

	if _, err := h.client.EmbedQuery(context.Background(), "consulta"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(*h.sleeps) != 1 || (*h.sleeps)[0] != 17*time.Second {
		t.Errorf("got esperas %v, want los 17 s que pidió el proveedor", *h.sleeps)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	h := newHarness(t, 503, 503, 503, 503, 503, 503)

	_, err := h.client.EmbedQuery(context.Background(), "consulta")

	if !errors.Is(err, llmgw.ErrUnavailable) {
		t.Fatalf("got %v, want ErrUnavailable", err)
	}
	if got := len(h.api.calls()); got != defaultMaxAttempts {
		t.Errorf("got %d intentos, want %d", got, defaultMaxAttempts)
	}
}

// Un 403 que se repite no es intermitencia: es una clave mala. Se abandona
// pronto y se informa como un problema de configuración.
func TestPersistentForbiddenIsMisconfiguration(t *testing.T) {
	h := newHarness(t, 403, 403, 403, 403, 403)

	_, err := h.client.EmbedQuery(context.Background(), "consulta")

	if !errors.Is(err, llmgw.ErrMisconfigured) {
		t.Fatalf("got %v, want ErrMisconfigured", err)
	}
	if got, want := len(h.api.calls()), maxForbiddenRetries+1; got != want {
		t.Errorf("got %d intentos, want %d", got, want)
	}
}

func TestDoesNotRetryPermanentErrors(t *testing.T) {
	for status, want := range map[int]error{
		400: llmgw.ErrInvalidInput,
		401: llmgw.ErrMisconfigured,
		404: llmgw.ErrMisconfigured,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			h := newHarness(t, status)

			_, err := h.client.EmbedQuery(context.Background(), "consulta")

			if !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
			if got := len(h.api.calls()); got != 1 {
				t.Errorf("got %d intentos, want 1", got)
			}
		})
	}
}

func TestErrorsDoNotLeakKeyOrProviderDetail(t *testing.T) {
	for _, statuses := range [][]int{{400}, {401}, {503, 503, 503, 503, 503}, {403, 403, 403}} {
		h := newHarness(t, statuses...)
		_, err := h.client.EmbedQuery(context.Background(), "consulta")
		if err == nil {
			t.Fatalf("%v: se esperaba un error", statuses)
		}
		for _, secret := range []string{testKey, "detalle interno del proveedor"} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("%v: el error expone %q: %v", statuses, secret, err)
			}
		}
	}
}

func TestRejectsMalformedVectors(t *testing.T) {
	for name, vector := range map[string]func(string) []float32{
		"dimensión equivocada": func(string) []float32 { return make([]float32, 3072) },
		"sin magnitud":         func(string) []float32 { return make([]float32, dimensions) },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.api.vector = vector
			if _, err := h.client.EmbedQuery(context.Background(), "consulta"); err == nil {
				t.Error("se aceptó un vector inválido")
			}
		})
	}
}

// El ritmo no debe exceder los tokens por minuto: agotado el cupo, el
// siguiente lote espera lo que tarde en recargarse.
func TestPacesToTokensPerMinute(t *testing.T) {
	h := newHarness(t)
	clock := h.clock
	h.client.pacer = newPacer(600, func() time.Time { return *clock }, h.client.sleepFn) // 10 tokens por segundo

	text := strings.Repeat("a", 300*bytesPerToken) // 300 tokens estimados
	ctx := context.Background()

	// Dos peticiones consumen los 600 del cupo inicial sin esperar.
	for range 2 {
		if _, err := h.client.EmbedQuery(ctx, text); err != nil {
			t.Fatalf("embedding: %v", err)
		}
	}
	if len(*h.sleeps) != 0 {
		t.Fatalf("esperó %v dentro del cupo", *h.sleeps)
	}

	// La tercera necesita 300 tokens más: 30 segundos de recarga.
	if _, err := h.client.EmbedQuery(ctx, text); err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if len(*h.sleeps) != 1 || (*h.sleeps)[0] != 30*time.Second {
		t.Errorf("got esperas %v, want 30 s", *h.sleeps)
	}
}

func TestStopsWhenContextIsCancelled(t *testing.T) {
	h := newHarness(t, 503, 503, 503)
	ctx, cancel := context.WithCancel(context.Background())
	h.client.sleep = func(ctx context.Context, d time.Duration) error {
		cancel() // el contexto termina durante la primera espera
		return ctx.Err()
	}

	_, err := h.client.EmbedQuery(ctx, "consulta")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if got := len(h.api.calls()); got != 1 {
		t.Errorf("got %d intentos tras cancelar, want 1", got)
	}
}

func TestUsageCountsEveryAttempt(t *testing.T) {
	h := newHarness(t, 429) // un intento fallido y uno exitoso
	text := strings.Repeat("a", 30*bytesPerToken)

	if _, err := h.client.EmbedQuery(context.Background(), text); err != nil {
		t.Fatalf("embedding: %v", err)
	}

	usage := h.client.Usage()
	if usage.Requests != 2 {
		t.Errorf("got %d peticiones, want 2: el intento fallido también descuenta de la cuota", usage.Requests)
	}
	if usage.Tokens != 30 {
		t.Errorf("got %d tokens, want 30", usage.Tokens)
	}
}
