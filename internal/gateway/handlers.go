package gateway

import (
	"context"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
)

// QueryService responde consultas sobre el corpus.
type QueryService interface {
	Answer(ctx context.Context, q service.Query) (service.Answer, error)
}

// DocumentService recibe documentos para ingesta.
type DocumentService interface {
	Submit(ctx context.Context, u service.Upload) (service.Job, error)
}

// Services agrupa las dependencias de los handlers. Cada una está detrás de
// una interfaz para que la implementación de ejemplo pueda reemplazarse por
// la real sin tocar la capa HTTP.
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
}

var _ api.StrictServerInterface = (*API)(nil)

func (a *API) ListDocuments(ctx context.Context, req api.ListDocumentsRequestObject) (api.ListDocumentsResponseObject, error) {
	return nil, errNotImplemented
}

func (a *API) DeleteDocument(ctx context.Context, req api.DeleteDocumentRequestObject) (api.DeleteDocumentResponseObject, error) {
	return nil, errNotImplemented
}

func (a *API) GetDocument(ctx context.Context, req api.GetDocumentRequestObject) (api.GetDocumentResponseObject, error) {
	return nil, errNotImplemented
}

func (a *API) GetJob(ctx context.Context, req api.GetJobRequestObject) (api.GetJobResponseObject, error) {
	return nil, errNotImplemented
}

func (a *API) GetUsage(ctx context.Context, req api.GetUsageRequestObject) (api.GetUsageResponseObject, error) {
	return nil, errNotImplemented
}
