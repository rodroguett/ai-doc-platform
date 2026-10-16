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
	"github.com/rodroguett/ai-doc-platform/internal/llmgw/gemini"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	usagestore "github.com/rodroguett/ai-doc-platform/internal/orchestrator/store"
	"github.com/rodroguett/ai-doc-platform/internal/platform/config"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/chunk"
	"github.com/rodroguett/ai-doc-platform/internal/rag/dispatch"
	"github.com/rodroguett/ai-doc-platform/internal/rag/pdftext"
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

	options := []gateway.Option{gateway.WithWriteToken(cfg.WriteToken)}

	var services gateway.Services

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

		// Cada módulo es dueño de su esquema y anota sus versiones aparte.
		applied, err := postgres.Migrate(ctx, pool, rag.Migrations())
		if err != nil {
			return err
		}
		appliedOrchestrator, err := postgres.MigrateTable(ctx, pool, orchestrator.Migrations(), orchestrator.MigrationsTable)
		if err != nil {
			return err
		}
		slog.Info("base de datos lista", "migraciones_aplicadas", applied+appliedOrchestrator)

		// El motor de extracción se inicia en la primera subida, no aquí.
		extractor := pdftext.New()
		defer func() { _ = extractor.Close() }()

		deps := rag.Deps{Store: store.New(pool), Extractor: extractor, Split: chunk.Split}

		// Sin clave no hay embeddings: los documentos se registran con sus
		// fragmentos y sus trabajos quedan en cola.
		var local *dispatch.InProcess
		var embedder *gemini.Client
		if cfg.GeminiAPIKey == "" {
			slog.Warn("GEMINI_API_KEY no está definida; los trabajos de ingesta quedan en cola y no hay consultas")
		} else {
			embedder, err = gemini.New(gemini.Config{APIKey: cfg.GeminiAPIKey})
			if err != nil {
				return err
			}
			deps.Embedder = embedder

			if cfg.TasksQueue != "" {
				dispatcher, err := newTasksDispatcher(cfg)
				if err != nil {
					return err
				}
				deps.Dispatcher = dispatcher
				slog.Info("los trabajos se despachan con Cloud Tasks", "cola", cfg.TasksQueue)
			} else {
				// Fuera de Cloud Run el proceso tiene CPU todo el tiempo y
				// puede ejecutar los trabajos por sí mismo.
				local = dispatch.NewInProcess()
				defer local.Stop()
				deps.Dispatcher = local
				slog.Info("los trabajos se ejecutan dentro del proceso")
			}
		}

		documents := rag.NewService(deps)
		if local != nil {
			local.Start(documents.RunJob)
		}
		services.Documents = documents

		// Las consultas necesitan convertir la pregunta en vector: sin
		// proveedor de embeddings no hay cómo buscar.
		if embedder != nil {
			// El modelo principal y su respaldo los fija ADR-0012. Si
			// ninguno responde, la consulta se entrega en modo degradado.
			generator, err := embedder.Generator(gemini.ModelFlashLite, gemini.ModelFlashLiteFallback)
			if err != nil {
				return err
			}
			// Cada consulta deja su registro de uso y costo estimado
			// (ADR-0013).
			queries := orchestrator.NewService(orchestrator.Config{
				Retriever: documents, Generator: generator, Usage: usagestore.New(pool),
			})
			services.Queries, services.Usage = queries, queries
		}
		options = append(options, gateway.WithReadinessCheck("database", pool.Ping))
	}

	if cfg.WriteToken == "" {
		slog.Warn("API_WRITE_TOKEN no está definida; las operaciones de escritura rechazan toda petición")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           gateway.NewServer(services, options...),
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
