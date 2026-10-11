package gateway

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
)

// protectedOperations son las operaciones que exigen el token de escritura:
// las que modifican el corpus. Leer y consultar es público.
//
// El contrato declara lo mismo con `security` en cada operación. El código
// generado no expone ese dato, así que la lista se repite aquí; un test
// verifica que ambas coincidan.
var protectedOperations = map[string]bool{
	"CreateDocument": true,
	"DeleteDocument": true,
}

var errUnauthorized = newServiceError(
	"unauthorized",
	"Credenciales ausentes o inválidas",
	http.StatusUnauthorized,
	"Esta operación requiere un token de escritura en el encabezado Authorization.",
)

// Option ajusta la construcción del servidor.
type Option func(*Server)

// WithWriteToken define el token que autoriza las operaciones de escritura.
// Sin token, o con un token vacío, esas operaciones rechazan toda petición:
// un despliegue mal configurado queda cerrado, no abierto.
func WithWriteToken(token string) Option {
	return func(s *Server) {
		if token == "" {
			return
		}
		sum := sha256.Sum256([]byte(token))
		s.writeToken = sum[:]
	}
}

// requireWriteToken rechaza las operaciones protegidas que no presentan el
// token. Corre antes que el handler, de modo que una subida no autorizada se
// descarta sin leer el archivo.
func (s *Server) requireWriteToken(next api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
	if !protectedOperations[operationID] {
		return next
	}
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		if !s.authorized(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="escritura"`)
			return nil, errUnauthorized
		}
		return next(ctx, w, r, request)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	if s.writeToken == nil {
		return false
	}
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	// Se comparan los hashes, en tiempo constante: el tiempo de respuesta no
	// revela ni el largo del token ni cuántos caracteres coinciden.
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return subtle.ConstantTimeCompare(sum[:], s.writeToken) == 1
}
