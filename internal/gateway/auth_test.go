package gateway

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	spec "github.com/rodroguett/ai-doc-platform/api"
	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
)

// writeRequests devuelve una petición por cada operación de escritura, sin
// credenciales.
func writeRequests(t *testing.T) map[string]*http.Request {
	t.Helper()
	body, contentType := multipartBody(t, map[string]string{"file": "%PDF-1.7", "collection": "rca"})
	create := httptest.NewRequest(http.MethodPost, "/v1/documents", body)
	create.Header.Set("Content-Type", contentType)

	return map[string]*http.Request{
		"crear documento":    create,
		"eliminar documento": httptest.NewRequest(http.MethodDelete, "/v1/documents/"+uuid.NewString(), nil),
	}
}

func TestWritesRequireToken(t *testing.T) {
	credentials := map[string]string{
		"sin encabezado":   "",
		"token incorrecto": "Bearer otro-token",
		"prefijo del real": "Bearer " + testToken[:len(testToken)-1],
		"otro esquema":     "Basic " + testToken,
		"sin esquema":      testToken,
	}
	for credName, header := range credentials {
		for opName, req := range writeRequests(t) {
			t.Run(opName+"/"+credName, func(t *testing.T) {
				if header != "" {
					req.Header.Set("Authorization", header)
				}
				rec := serve(t, newExampleServer(), req)

				p := decodeProblem(t, rec, http.StatusUnauthorized)
				if !strings.HasSuffix(p.Type, "/unauthorized") {
					t.Errorf("got type %q, want unauthorized", p.Type)
				}
				if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
					t.Errorf("got WWW-Authenticate %q, want esquema Bearer", got)
				}
			})
		}
	}
}

func TestWritesAcceptToken(t *testing.T) {
	s := newExampleServer()
	create := writeRequests(t)["crear documento"]
	// El esquema no distingue mayúsculas (RFC 9110).
	create.Header.Set("Authorization", "bearer "+testToken)

	var job api.Job
	decodeJSON(t, serve(t, s, create), http.StatusAccepted, &job)

	del := httptest.NewRequest(http.MethodDelete, "/v1/documents/"+job.ResourceId.String(), nil)
	if rec := serve(t, s, authorize(del)); rec.Code != http.StatusNoContent {
		t.Errorf("eliminar: got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}

// Un servidor sin token configurado queda cerrado: ninguna credencial sirve,
// ni siquiera una vacía.
func TestWritesAreClosedWithoutConfiguredToken(t *testing.T) {
	for _, s := range []*Server{NewServer(Services{}), NewServer(Services{}, WithWriteToken(""))} {
		for _, header := range []string{"", "Bearer ", "Bearer cualquiera"} {
			for name, req := range writeRequests(t) {
				if header != "" {
					req.Header.Set("Authorization", header)
				}
				if rec := serve(t, s, req); rec.Code != http.StatusUnauthorized {
					t.Errorf("%s con %q: got status %d, want %d", name, header, rec.Code, http.StatusUnauthorized)
				}
			}
		}
	}
}

func TestReadsArePublic(t *testing.T) {
	s := newExampleServer()
	job := upload(t, s, "%PDF-1.7", "rca")

	for _, path := range []string{
		"/v1/documents",
		"/v1/documents/" + job.ResourceId.String(),
		"/v1/jobs/" + job.Id.String(),
		"/docs",
		"/openapi.yaml",
	} {
		// Sin encabezado Authorization.
		if rec := serve(t, s, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusOK {
			t.Errorf("%s: got status %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
	if rec := postQuery(t, s, `{"question": "¿Qué pozos se monitorean?"}`); rec.Code != http.StatusOK {
		t.Errorf("consulta: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// El contrato declara qué operaciones exigen token y el código lo repite en
// protectedOperations, porque el código generado no expone ese dato. Este
// test falla si una operación se protege en un lado y no en el otro.
//
// Lee el YAML como texto y depende de su forma: `security` debe ir en la
// línea siguiente a `operationId`.
func TestProtectedOperationsMatchContract(t *testing.T) {
	operation := regexp.MustCompile(`(?m)^      operationId: (\w+)\n(      security:\n        - bearerAuth: \[\]\n)?`)

	var inContract []string
	matches := operation.FindAllStringSubmatch(string(spec.OpenAPI), -1)
	if len(matches) == 0 {
		t.Fatal("no se encontraron operaciones en el contrato")
	}
	for _, m := range matches {
		if m[2] != "" {
			// El código generado nombra las operaciones en PascalCase.
			inContract = append(inContract, strings.ToUpper(m[1][:1])+m[1][1:])
		}
	}
	if strings.Count(string(spec.OpenAPI), "- bearerAuth: []") != len(inContract) {
		t.Fatal("hay un requisito de seguridad en el contrato que este test no supo leer")
	}

	var inCode []string
	for op := range protectedOperations {
		inCode = append(inCode, op)
	}
	sort.Strings(inContract)
	sort.Strings(inCode)

	if strings.Join(inContract, ",") != strings.Join(inCode, ",") {
		t.Errorf("operaciones protegidas:\n contrato %v\n   código %v", inContract, inCode)
	}
}
