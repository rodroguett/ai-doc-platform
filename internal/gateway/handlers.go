package gateway

import (
	"context"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// QueryService responde consultas sobre el corpus.
type QueryService interface {
	Answer(ctx context.Context, q orchestrator.Query) (orchestrator.Answer, error)
}

// UsageService informa el consumo de un periodo.
type UsageService interface {
	Usage(ctx context.Context, p orchestrator.UsagePeriod) (orchestrator.UsageSummary, error)
}

// DocumentService registra y consulta documentos y sus trabajos de ingesta.
type DocumentService interface {
	Submit(ctx context.Context, u rag.Upload) (rag.Job, error)
	Job(ctx context.Context, id uuid.UUID) (rag.Job, error)
	Document(ctx context.Context, id uuid.UUID) (rag.Document, error)
	Documents(ctx context.Context, opts rag.ListOptions) (rag.Page, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// RunJob ejecuta una pasada del trabajo y devuelve su estado.
	RunJob(ctx context.Context, id uuid.UUID) (rag.Job, error)
}

// Services agrupa las dependencias de los handlers. Cada una está detrás de
// una interfaz definida aquí, del lado de quien la consume, y la implementa
// otro módulo.
//
// Todas pueden ser nil: sin base de datos configurada, los endpoints que
// dependen de ellas responden 503.
type Services struct {
	Queries   QueryService
	Documents DocumentService
	Usage     UsageService
}

// API implementa el contrato definido en api/openapi.yaml. Cada método
// traduce entre los tipos generados y los del dominio; la lógica vive en los
// servicios.
//
// Implementar la interfaz generada hace que cualquier endpoint declarado en
// el spec y ausente en el código impida construir el binario.
type API struct {
	svc Services

	// uploads limita las subidas simultáneas a su capacidad.
	uploads chan struct{}
}

var _ api.StrictServerInterface = (*API)(nil)
