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
}

func NewServer(svc Services) *Server {
	s := &Server{mux: http.NewServeMux(), svc: svc}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)

	strict := api.NewStrictHandlerWithOptions(&API{svc: s.svc}, nil, api.StrictHTTPServerOptions{
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
