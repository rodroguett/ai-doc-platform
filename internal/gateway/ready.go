package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// defaultReadyTimeout es el plazo de cada verificación. Es holgado porque la
// base de datos puede estar suspendida y tardar unos segundos en despertar.
const defaultReadyTimeout = 5 * time.Second

// readinessCheck verifica una dependencia del servicio.
type readinessCheck struct {
	name  string
	check func(ctx context.Context) error
}

// WithReadinessCheck registra una dependencia que /ready debe verificar.
// check debe respetar la cancelación del contexto.
func WithReadinessCheck(name string, check func(ctx context.Context) error) Option {
	return func(s *Server) {
		s.readiness = append(s.readiness, readinessCheck{name: name, check: check})
	}
}

type readyResponse struct {
	Status string       `json:"status"`
	Checks []readyCheck `json:"checks"`
}

type readyCheck struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// handleReady responde si el servicio puede atender peticiones que dependen
// de sus dependencias externas: 200 si todas responden, 503 si alguna falla.
//
// Es distinto de /health, que solo indica que el proceso está vivo y nunca
// consulta nada externo. Un proceso puede estar vivo y no estar listo.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	resp := readyResponse{Status: "ready", Checks: make([]readyCheck, 0, len(s.readiness))}
	status := http.StatusOK

	for _, dep := range s.readiness {
		start := time.Now()
		err := s.runCheck(r.Context(), dep)
		result := readyCheck{Name: dep.name, Status: "ok", LatencyMs: time.Since(start).Milliseconds()}

		if err != nil {
			// El detalle queda en el log. La respuesta es pública y el
			// error puede nombrar hosts o usuarios internos.
			slog.ErrorContext(r.Context(), "dependencia no disponible", "dependencia", dep.name, "error", err)
			result.Status = "failed"
			resp.Status = "unavailable"
			status = http.StatusServiceUnavailable
		}
		resp.Checks = append(resp.Checks, result)
	}

	writeJSON(w, status, resp)
}

// runCheck ejecuta una verificación con plazo propio. Si la verificación no
// respeta la cancelación, se la abandona: /ready no puede quedar colgado por
// una dependencia que no responde.
func (s *Server) runCheck(ctx context.Context, dep readinessCheck) error {
	ctx, cancel := context.WithTimeout(ctx, s.readyTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- dep.check(ctx) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
