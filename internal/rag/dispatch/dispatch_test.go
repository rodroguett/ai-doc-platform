package dispatch

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

func TestEnqueueRunsTheJobOutsideTheCaller(t *testing.T) {
	d := NewInProcess()
	defer d.Stop()
	var mu sync.Mutex
	var ran []uuid.UUID
	d.Start(func(_ context.Context, id uuid.UUID) (rag.Job, error) {
		mu.Lock()
		defer mu.Unlock()
		ran = append(ran, id)
		return rag.Job{}, nil
	})

	// El contexto de quien encola ya terminó: el trabajo debe correr igual.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id := uuid.Must(uuid.NewV7())
	if err := d.Enqueue(ctx, id); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	d.Wait()

	if len(ran) != 1 || ran[0] != id {
		t.Errorf("got ejecutados %v, want [%s]", ran, id)
	}
}

func TestRetriesWhileProviderIsUnavailable(t *testing.T) {
	d := NewInProcess()
	defer d.Stop()
	d.retryDelay = time.Millisecond
	var calls atomic.Int32
	d.Start(func(context.Context, uuid.UUID) (rag.Job, error) {
		if calls.Add(1) < 3 {
			return rag.Job{}, fmt.Errorf("%w: sin respuesta", rag.ErrUnavailable)
		}
		return rag.Job{}, nil
	})

	if err := d.Enqueue(context.Background(), uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	d.Wait()

	if got := calls.Load(); got != 3 {
		t.Errorf("got %d intentos, want 3", got)
	}
}

func TestDoesNotRetryOtherErrors(t *testing.T) {
	d := NewInProcess()
	defer d.Stop()
	d.retryDelay = time.Millisecond
	var calls atomic.Int32
	d.Start(func(context.Context, uuid.UUID) (rag.Job, error) {
		calls.Add(1)
		return rag.Job{}, errors.New("la base de datos no responde")
	})

	if err := d.Enqueue(context.Background(), uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	d.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("got %d intentos, want 1", got)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	d := NewInProcess()
	defer d.Stop()
	d.retryDelay = time.Microsecond
	var calls atomic.Int32
	d.Start(func(context.Context, uuid.UUID) (rag.Job, error) {
		calls.Add(1)
		return rag.Job{}, rag.ErrUnavailable
	})

	if err := d.Enqueue(context.Background(), uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	d.Wait()

	if got := calls.Load(); got != maxAttempts {
		t.Errorf("got %d intentos, want %d", got, maxAttempts)
	}
}

func TestEnqueueFailsWithoutRunnerOrAfterStop(t *testing.T) {
	d := NewInProcess()
	if err := d.Enqueue(context.Background(), uuid.Must(uuid.NewV7())); err == nil {
		t.Error("se encoló sin tener quien ejecute")
	}

	d.Start(func(context.Context, uuid.UUID) (rag.Job, error) { return rag.Job{}, nil })
	d.Stop()
	if err := d.Enqueue(context.Background(), uuid.Must(uuid.NewV7())); err == nil {
		t.Error("se encoló en un despachador detenido")
	}
}

// Detener el despachador cancela el trabajo en curso y espera a que termine.
func TestStopCancelsRunningJobs(t *testing.T) {
	d := NewInProcess()
	started := make(chan struct{})
	var cancelled atomic.Bool
	d.Start(func(ctx context.Context, _ uuid.UUID) (rag.Job, error) {
		close(started)
		<-ctx.Done()
		cancelled.Store(true)
		return rag.Job{}, ctx.Err()
	})

	if err := d.Enqueue(context.Background(), uuid.Must(uuid.NewV7())); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	<-started
	d.Stop()

	if !cancelled.Load() {
		t.Error("Stop volvió antes de que el trabajo terminara")
	}
}
