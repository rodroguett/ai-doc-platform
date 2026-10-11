package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/gateway"
	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
	"github.com/rodroguett/ai-doc-platform/internal/platform/config"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	services := gateway.Services{
		// Datos de ejemplo hasta que exista la búsqueda.
		Queries: service.ExampleQueries{},
	}

	// Sin base de datos configurada el proceso arranca igual y los endpoints
	// de documentos responden 503. Con ella, un fallo al conectar o migrar
	// impide arrancar, de modo que un despliegue con el esquema roto no
	// recibe tráfico.
	if cfg.DatabaseURL == "" {
		slog.Warn("DATABASE_URL no está definida; se arranca sin base de datos")
	} else {
		pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()

		applied, err := postgres.Migrate(ctx, pool, rag.Migrations())
		if err != nil {
			return err
		}
		slog.Info("base de datos lista", "migraciones_aplicadas", applied)

		services.Documents = rag.NewService(store.New(pool))
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           gateway.NewServer(services),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
