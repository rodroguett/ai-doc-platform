package gateway

import (
	"context"
	"errors"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
)

// API implementa el contrato definido en api/openapi.yaml.
//
// En esta etapa los métodos no tienen implementación: el objetivo es que el
// contrato y el servidor compilen juntos, de modo que cualquier endpoint
// declarado en el spec y ausente en el código impida construir el binario.
type API struct{}

var _ api.StrictServerInterface = (*API)(nil)

var errNotImplemented = errors.New("not implemented")

func (a *API) ListDocuments(ctx context.Context, req api.ListDocumentsRequestObject) (api.ListDocumentsResponseObject, error) {
	return nil, errNotImplemented
}

func (a *API) CreateDocument(ctx context.Context, req api.CreateDocumentRequestObject) (api.CreateDocumentResponseObject, error) {
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

func (a *API) CreateQuery(ctx context.Context, req api.CreateQueryRequestObject) (api.CreateQueryResponseObject, error) {
	return nil, errNotImplemented
}

func (a *API) GetUsage(ctx context.Context, req api.GetUsageRequestObject) (api.GetUsageResponseObject, error) {
	return nil, errNotImplemented
}
