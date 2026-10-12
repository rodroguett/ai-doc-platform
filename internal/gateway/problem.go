package gateway

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/service"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// problemBase es el prefijo de los tipos de problema que expone el servicio.
// RFC 9457 pide un URI que identifique la clase de error; no requiere que
// sea resoluble, pero mantenerlo estable permite que los clientes lo usen
// para discriminar sin depender del texto.
const problemBase = "https://github.com/rodroguett/ai-doc-platform/problems/"

// Problem representa un error conforme a RFC 9457.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	TraceID  string `json:"trace_id,omitempty"`
}

// serviceError es un error con la información necesaria para construir un
// Problem. Los handlers devuelven estos errores y la capa HTTP los traduce.
type serviceError struct {
	kind   string
	title  string
	status int
	detail string
}

func (e *serviceError) Error() string { return e.title }

func newServiceError(kind, title string, status int, detail string) *serviceError {
	return &serviceError{kind: kind, title: title, status: status, detail: detail}
}

// errNoDatabase se devuelve cuando el proceso corre sin base de datos.
var errNoDatabase = newServiceError(
	"unavailable",
	"Servicio no disponible",
	http.StatusServiceUnavailable,
	"El almacenamiento de documentos no está configurado en esta instancia.",
)

// errUploadInProgress se devuelve cuando ya hay una subida en curso.
var errUploadInProgress = newServiceError(
	"upload-in-progress",
	"Hay otra subida en curso",
	http.StatusServiceUnavailable,
	"El servicio procesa un documento a la vez. Reintente en unos segundos.",
)

var errNotImplemented = newServiceError(
	"not-implemented",
	"Funcionalidad no disponible",
	http.StatusNotImplemented,
	"Este endpoint todavía no tiene implementación.",
)

// invalidRequest construye un error 400 con un detalle apto para el cliente.
func invalidRequest(detail string) *serviceError {
	return newServiceError("invalid-request", "Petición inválida", http.StatusBadRequest, detail)
}

// writeProblem serializa un Problem como application/problem+json.
func writeProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	p.Instance = r.URL.Path

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.ErrorContext(r.Context(), "no se pudo escribir la respuesta de error", "error", err)
	}
}

// handleResponseError traduce cualquier error devuelto por un handler a una
// respuesta conforme al contrato. Los errores conocidos conservan su estado
// y su detalle; los inesperados se reportan como 500 sin exponer el mensaje
// original, que puede contener información interna.
func handleResponseError(w http.ResponseWriter, r *http.Request, err error) {
	err = fromDomainError(err)

	var se *serviceError
	if errors.As(err, &se) {
		writeProblem(w, r, Problem{
			Type:   problemBase + se.kind,
			Title:  se.title,
			Status: se.status,
			Detail: se.detail,
		})
		return
	}

	slog.ErrorContext(r.Context(), "error no controlado en el handler",
		"error", err, "path", r.URL.Path)

	writeProblem(w, r, Problem{
		Type:   problemBase + "internal",
		Title:  "Error interno",
		Status: http.StatusInternalServerError,
	})
}

// fromDomainError convierte los errores que los módulos exponen en su
// contrato en errores con estado HTTP. Los módulos no conocen HTTP; esta es
// la única traducción. Cualquier otro error se devuelve intacto y termina
// como 500.
func fromDomainError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidInput), errors.Is(err, rag.ErrInvalidInput):
		return invalidRequest(err.Error())
	case errors.Is(err, rag.ErrNotFound):
		return newServiceError("not-found", "Recurso no encontrado", http.StatusNotFound,
			"No existe un recurso con ese identificador.")
	case errors.Is(err, rag.ErrDuplicate):
		return newServiceError("conflict", "Documento duplicado", http.StatusConflict,
			"Ese contenido ya fue registrado en la colección.")
	case errors.Is(err, rag.ErrUnavailable):
		return newServiceError("unavailable", "Servicio no disponible", http.StatusServiceUnavailable,
			"Una dependencia externa no responde. Reintente más tarde.")
	}
	return err
}

// handleRequestError responde a fallos de deserialización o validación de la
// petición, que ocurren antes de que el handler llegue a ejecutarse.
func handleRequestError(w http.ResponseWriter, r *http.Request, err error) {
	writeProblem(w, r, Problem{
		Type:   problemBase + "invalid-request",
		Title:  "Petición inválida",
		Status: http.StatusBadRequest,
		Detail: err.Error(),
	})
}
