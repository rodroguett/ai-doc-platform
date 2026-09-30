package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
)

// Límites de lectura del multipart. El archivo se lee completo en memoria
// porque las partes llegan en orden arbitrario y el servicio necesita la
// colección junto con el contenido. El tope del archivo queda bajo el límite
// de 32 MiB por petición de Cloud Run.
const (
	maxUploadBytes = 20 << 20
	maxFieldBytes  = 64 << 10
)

func (a *API) CreateDocument(ctx context.Context, req api.CreateDocumentRequestObject) (api.CreateDocumentResponseObject, error) {
	if req.Body == nil {
		return nil, invalidRequest("falta el cuerpo multipart")
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

// readUpload recorre las partes del formulario. Las partes desconocidas se
// ignoran; las que faltan las detecta la validación del dominio.
func readUpload(mr *multipart.Reader) (service.Upload, error) {
	var u service.Upload
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return u, nil
		}
		if err != nil {
			return service.Upload{}, invalidRequest("el cuerpo multipart es inválido")
		}

		err = readPart(part, &u)
		_ = part.Close()
		if err != nil {
			return service.Upload{}, err
		}
	}
}

func readPart(part *multipart.Part, u *service.Upload) error {
	switch part.FormName() {
	case "file":
		data, err := readLimited(part, maxUploadBytes)
		if err != nil {
			return err
		}
		u.Filename = part.FileName()
		u.Content = bytes.NewReader(data)

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

func toJob(j service.Job) api.Job {
	out := api.Job{
		Id:        j.ID,
		Type:      api.JobType(j.Kind),
		Status:    api.JobStatus(j.Status),
		Progress:  &j.Progress,
		Error:     optional(j.Error),
		CreatedAt: j.CreatedAt,
	}
	if j.ResourceID != uuid.Nil {
		out.ResourceId = &j.ResourceID
	}
	return out
}
