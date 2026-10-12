package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
)

type Server struct {
	mux *http.ServeMux
	svc Services

	// writeToken es el sha256 del token de escritura, o nil si no hay.
	writeToken []byte

	readiness    []readinessCheck
	readyTimeout time.Duration

	limiter *rateLimiter
	handler http.Handler
}

func NewServer(svc Services, opts ...Option) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		svc:          svc,
		readyTimeout: defaultReadyTimeout,
		// El límite está activo por omisión: un servidor construido sin
		// pensar en él queda protegido.
		limiter: newRateLimiter(defaultRatePerMinute, defaultRateBurst, time.Now),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.routes()
	s.handler = s.limit(s.mux)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Ni /health ni /ready terminan en "z": Cloud Run intercepta esas rutas
	// antes de que lleguen al contenedor.
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)

	// La documentación queda fuera de /v1: describe la API, no es parte de
	// ella, y no debe cambiar de ruta cuando la API cambie de versión.
	s.mux.HandleFunc("GET /openapi.yaml", s.handleSpec)
	s.mux.HandleFunc("GET /docs", s.handleDocs)
	s.mux.Handle("GET /{$}", http.RedirectHandler("/docs", http.StatusFound))

	strict := api.NewStrictHandlerWithOptions(&API{svc: s.svc, uploads: make(chan struct{}, 1)},
		[]api.StrictMiddlewareFunc{s.requireWriteToken},
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  handleRequestError,
			ResponseErrorHandlerFunc: handleResponseError,
		})

	_ = api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseURL:          "/v1",
		BaseRouter:       s.mux,
		ErrorHandlerFunc: handleRequestError,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
