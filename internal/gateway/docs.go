package gateway

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"net/http"

	spec "github.com/rodroguett/ai-doc-platform/api"
)

//go:embed swagger.html
var swaggerHTML []byte

//go:embed demo.html
var demoHTML []byte

// demoPolicy restringe lo que la página de demo puede hacer: ejecutar solo
// su propio script, identificado por su hash, y conectarse solo a este
// servicio. El texto de las respuestas viene de un modelo; si alguna vez se
// insertara como HTML por error, la política impediría que se ejecute.
var demoPolicy = "default-src 'none'; script-src 'sha256-" + scriptHash(demoHTML) + "'; " +
	"style-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// scriptHash devuelve el hash SHA-256, en base64, del primer script en
// línea de la página, como lo espera una Content-Security-Policy.
func scriptHash(page []byte) string {
	_, rest, ok := bytes.Cut(page, []byte("<script>"))
	script, _, closed := bytes.Cut(rest, []byte("</script>"))
	if !ok || !closed {
		// La página se embebe al compilar: sin script es un error del código.
		panic("la página de demo no tiene un script en línea")
	}
	sum := sha256.Sum256(script)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// handleDemo sirve la página de demo, que consulta POST /v1/queries como
// cualquier otro cliente, con su mismo límite por origen y su tope diario.
func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", demoPolicy)
	_, _ = w.Write(demoHTML)
}

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
