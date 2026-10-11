package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	spec "github.com/rodroguett/ai-doc-platform/api"
)

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, newExampleServer(), httptest.NewRequest(http.MethodGet, path, nil))
}

func TestSpecIsServed(t *testing.T) {
	rec := get(t, "/openapi.yaml")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("got content-type %q, want application/yaml", ct)
	}
	if !bytes.Equal(rec.Body.Bytes(), spec.OpenAPI) {
		t.Error("el spec servido difiere del contrato del repositorio")
	}
}

// Swagger UI ejecuta las peticiones contra el servidor que declara el
// contrato. Si deja de ser relativo, la documentación desplegada apuntaría a
// otro origen.
func TestSpecDeclaresRelativeServer(t *testing.T) {
	if !bytes.Contains(spec.OpenAPI, []byte("\n  - url: /v1\n")) {
		t.Error("el contrato debe declarar el servidor relativo /v1")
	}
}

func TestDocsPage(t *testing.T) {
	rec := get(t, "/docs")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("got content-type %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `url: "/openapi.yaml"`) {
		t.Error("la página no carga el contrato desde /openapi.yaml")
	}
	// Todo recurso externo debe llevar su hash de integridad.
	if external, pinned := strings.Count(body, "https://cdn."), strings.Count(body, `integrity="sha384-`); external == 0 || external != pinned {
		t.Errorf("%d recursos externos, %d con integrity", external, pinned)
	}
}

func TestDocsAreOutsideVersionedPath(t *testing.T) {
	for _, path := range []string{"/v1/docs", "/v1/openapi.yaml"} {
		if rec := get(t, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: got status %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestRootRedirectsToDocs(t *testing.T) {
	rec := get(t, "/")

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/docs" {
		t.Errorf("got %d → %q, want 302 → /docs", rec.Code, rec.Header().Get("Location"))
	}
}
