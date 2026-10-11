package gateway

import (
	_ "embed"
	"net/http"

	spec "github.com/rodroguett/ai-doc-platform/api"
)

//go:embed swagger.html
var swaggerHTML []byte

// handleSpec sirve el contrato tal como está en el repositorio. Es el mismo
// archivo desde el que se genera el servidor, de modo que lo publicado no
// puede diferir de lo implementado.
func (s *Server) handleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(spec.OpenAPI)
}

// handleDocs sirve la página de Swagger UI, que lee el contrato desde
// /openapi.yaml y ejecuta las peticiones contra este mismo servicio.
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(swaggerHTML)
}
