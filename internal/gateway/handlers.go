package gateway

import (
	"context"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// QueryService responde consultas sobre el corpus.
type QueryService interface {
	Answer(ctx context.Context, q service.Query) (service.Answer, error)
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
// Documents puede ser nil: sin base de datos configurada, los endpoints de
// documentos y trabajos responden 503.
type Services struct {
	Queries   QueryService
	Documents DocumentService
}

// API implementa el contrato definido en api/openapi.yaml. Cada método
// traduce entre los tipos generados y los del dominio; la lógica vive en los
// servicios.
//
// Implementar la interfaz generada hace que cualquier endpoint declarado en
// el spec y ausente en el código impida construir el binario. Los que aún no
// tienen servicio detrás responden 501.
type API struct {
	svc Services

	// uploads limita las subidas simultáneas a su capacidad.
	uploads chan struct{}
}

var _ api.StrictServerInterface = (*API)(nil)

func (a *API) GetUsage(ctx context.Context, req api.GetUsageRequestObject) (api.GetUsageResponseObject, error) {
	return nil, errNotImplemented
}
