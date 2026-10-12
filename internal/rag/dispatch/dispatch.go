// Package dispatch ejecuta los trabajos de rag dentro del propio proceso.
//
// Sirve donde el proceso tiene CPU todo el tiempo: en desarrollo local y en
// los tests. En Cloud Run no, porque la CPU se retira al terminar cada
// petición; ahí los trabajos los despacha Cloud Tasks (ADR-0010).
package dispatch

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

const (
	// maxAttempts y retryDelay gobiernan los reintentos cuando el proveedor
	// de embeddings no responde.
	maxAttempts = 10
	retryDelay  = 15 * time.Second
)

// Runner ejecuta una pasada de un trabajo.
type Runner func(ctx context.Context, jobID uuid.UUID) (rag.Job, error)

// InProcess ejecuta cada trabajo encolado en una goroutine.
type InProcess struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	runner Runner

	retryDelay time.Duration
}

var _ rag.Dispatcher = (*InProcess)(nil)

// NewInProcess crea el despachador. Los trabajos no corren hasta que se le
// entrega el Runner con Start: el servicio que los ejecuta necesita al
// despachador para construirse, y el despachador al servicio para ejecutar.
func NewInProcess() *InProcess {
	ctx, cancel := context.WithCancel(context.Background())
	return &InProcess{ctx: ctx, cancel: cancel, retryDelay: retryDelay}
}

// Start entrega la función que ejecuta los trabajos.
func (d *InProcess) Start(runner Runner) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.runner = runner
}

// Enqueue ejecuta el trabajo en segundo plano. No usa el contexto de quien
// encola: el trabajo debe sobrevivir a la petición que lo originó.
func (d *InProcess) Enqueue(_ context.Context, jobID uuid.UUID) error {
	d.mu.Lock()
	runner := d.runner
	d.mu.Unlock()
	if runner == nil {
		return errors.New("el despachador no tiene quien ejecute los trabajos")
	}
	if d.ctx.Err() != nil {
		return errors.New("el despachador está detenido")
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.run(runner, jobID)
	}()
	return nil
}

func (d *InProcess) run(runner Runner, jobID uuid.UUID) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err := runner(d.ctx, jobID)
		switch {
		case err == nil:
			// La pasada terminó. Si quedó trabajo, ella misma encoló la
			// siguiente.
			return
		case errors.Is(err, rag.ErrUnavailable):
			slog.Warn("el trabajo se reintentará", "trabajo", jobID, "intento", attempt, "error", err)
			select {
			case <-time.After(d.retryDelay):
			case <-d.ctx.Done():
				return
			}
		default:
			if d.ctx.Err() == nil {
				slog.Error("el trabajo falló", "trabajo", jobID, "error", err)
			}
			return
		}
	}
	slog.Error("el trabajo se abandonó tras agotar los reintentos", "trabajo", jobID)
}

// Wait espera a que terminen los trabajos en curso.
func (d *InProcess) Wait() { d.wg.Wait() }

// Stop cancela los trabajos en curso y espera a que terminen. Lo que quedó
// pendiente se retoma cuando el trabajo vuelva a ejecutarse.
func (d *InProcess) Stop() {
	d.cancel()
	d.wg.Wait()
}
