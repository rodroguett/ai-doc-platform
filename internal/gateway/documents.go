package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// Límites de lectura del multipart. El archivo se lee completo en memoria
// porque las partes llegan en orden arbitrario y la extracción de texto lo
// necesita entero. El tope del archivo queda bajo el límite de 32 MiB por
// petición de Cloud Run, y junto con el motor de extracción cabe en la
// memoria de la instancia (ADR-0009).
const (
	maxUploadBytes = 20 << 20
	maxFieldBytes  = 64 << 10
)

func (a *API) CreateDocument(ctx context.Context, req api.CreateDocumentRequestObject) (api.CreateDocumentResponseObject, error) {
	if a.svc.Documents == nil {
		return nil, errNoDatabase
	}
	if req.Body == nil {
		return nil, invalidRequest("falta el cuerpo multipart")
	}

	// Una subida a la vez. Cada una retiene el archivo en memoria mientras
	// el motor de extracción ocupa la mayor parte de la que tiene la
	// instancia: aceptar varias la agotaría. La segunda se rechaza en lugar
	// de esperar, para no retener además su conexión.
	select {
	case a.uploads <- struct{}{}:
		defer func() { <-a.uploads }()
	default:
		return nil, errUploadInProgress
	}

	u, err := readUpload(req.Body)
	if err != nil {
		return nil, err
	}

	job, err := a.svc.Documents.Submit(ctx, u)
	if err != nil {
		return nil, err
	}

	return api.CreateDocument202JSONResponse{
		Body: toJob(job),
		Headers: api.CreateDocument202ResponseHeaders{
			Location: ptr("/v1/jobs/" + job.ID.String()),
		},
	}, nil
}

func (a *API) GetJob(ctx context.Context, req api.GetJobRequestObject) (api.GetJobResponseObject, error) {
	if a.svc.Documents == nil {
		return nil, errNoDatabase
	}
	job, err := a.svc.Documents.Job(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	return api.GetJob200JSONResponse(toJob(job)), nil
}

// RunJob ejecuta una pasada de un trabajo. Lo invoca Cloud Tasks, con el
// token de escritura, y el trabajo ocurre dentro de esta petición: es la
// forma de tener CPU fuera de la petición que subió el documento.
//
// Si el proveedor de embeddings no responde devuelve 503, que es lo que hace
// que la cola reintente más tarde.
func (a *API) RunJob(ctx context.Context, req api.RunJobRequestObject) (api.RunJobResponseObject, error) {
	if a.svc.Documents == nil {
		return nil, errNoDatabase
	}
	job, err := a.svc.Documents.RunJob(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	return api.RunJob200JSONResponse(toJob(job)), nil
}

// RunJobPath es la ruta que ejecuta una pasada del trabajo.
func RunJobPath(jobID uuid.UUID) string {
	return "/v1/jobs/" + jobID.String() + "/run"
}

func (a *API) GetDocument(ctx context.Context, req api.GetDocumentRequestObject) (api.GetDocumentResponseObject, error) {
	if a.svc.Documents == nil {
		return nil, errNoDatabase
	}
	doc, err := a.svc.Documents.Document(ctx, req.DocumentId)
	if err != nil {
		return nil, err
	}
	return api.GetDocument200JSONResponse(toDocument(doc)), nil
}

func (a *API) ListDocuments(ctx context.Context, req api.ListDocumentsRequestObject) (api.ListDocumentsResponseObject, error) {
	if a.svc.Documents == nil {
		return nil, errNoDatabase
	}

	var opts rag.ListOptions
	if req.Params.Collection != nil {
		opts.Collection = *req.Params.Collection
	}
	if req.Params.Cursor != nil {
		opts.Cursor = *req.Params.Cursor
	}
	if req.Params.Limit != nil {
		opts.Limit = *req.Params.Limit
		// Cero es "sin valor" para el dominio; enviado de forma explícita
		// es un límite inválido.
		if opts.Limit == 0 {
			return nil, invalidRequest("limit debe ser mayor que cero")
		}
	}

	page, err := a.svc.Documents.Documents(ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := api.ListDocuments200JSONResponse{
		Data:       make([]api.Document, 0, len(page.Documents)),
		NextCursor: optional(page.NextCursor),
	}
	for _, doc := range page.Documents {
		resp.Data = append(resp.Data, toDocument(doc))
	}
	return resp, nil
}

func (a *API) DeleteDocument(ctx context.Context, req api.DeleteDocumentRequestObject) (api.DeleteDocumentResponseObject, error) {
	if a.svc.Documents == nil {
		return nil, errNoDatabase
	}
	if err := a.svc.Documents.Delete(ctx, req.DocumentId); err != nil {
		return nil, err
	}
	return api.DeleteDocument204Response{}, nil
}

// readUpload recorre las partes del formulario. Las partes desconocidas se
// ignoran; las que faltan las detecta la validación del dominio.
func readUpload(mr *multipart.Reader) (rag.Upload, error) {
	var u rag.Upload
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return u, nil
		}
		if err != nil {
			return rag.Upload{}, invalidRequest("el cuerpo multipart es inválido")
		}

		err = readPart(part, &u)
		_ = part.Close()
		if err != nil {
			return rag.Upload{}, err
		}
	}
}

func readPart(part *multipart.Part, u *rag.Upload) error {
	switch part.FormName() {
	case "file":
		data, err := readLimited(part, maxUploadBytes)
		if err != nil {
			return err
		}
		u.Filename = part.FileName()
		u.Content = data

	case "collection":
		data, err := readLimited(part, maxFieldBytes)
		if err != nil {
			return err
		}
		u.Collection = string(data)

	case "metadata":
		data, err := readLimited(part, maxFieldBytes)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &u.Metadata); err != nil {
			return invalidRequest("metadata debe ser un objeto JSON")
		}
	}
	return nil
}

// readLimited lee la parte completa y falla si excede max.
func readLimited(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, invalidRequest("no se pudo leer el cuerpo multipart")
	}
	if int64(len(data)) > max {
		return nil, newServiceError("payload-too-large", "Contenido demasiado grande",
			http.StatusRequestEntityTooLarge, fmt.Sprintf("Cada parte admite como máximo %d bytes.", max))
	}
	return data, nil
}

func toJob(j rag.Job) api.Job {
	return api.Job{
		Id:         j.ID,
		Type:       api.JobType(j.Kind),
		Status:     api.JobStatus(j.Status),
		Progress:   &j.Progress,
		ResourceId: &j.DocumentID,
		Error:      optional(j.Error),
		CreatedAt:  j.CreatedAt,
	}
}

func toDocument(d rag.Document) api.Document {
	out := api.Document{
		Id:         d.ID,
		Source:     d.Source,
		Collection: d.Collection,
		Status:     api.DocumentStatus(d.Status),
		ChunkCount: &d.ChunkCount,
		CreatedAt:  d.CreatedAt,
	}
	if len(d.Metadata) > 0 {
		out.Metadata = &d.Metadata
	}
	return out
}
