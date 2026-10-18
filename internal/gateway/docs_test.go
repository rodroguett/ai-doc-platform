package gateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
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

func TestRootRedirectsToDemo(t *testing.T) {
	rec := get(t, "/")

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/demo" {
		t.Errorf("got %d → %q, want 302 → /demo", rec.Code, rec.Header().Get("Location"))
	}
}

func TestDemoPage(t *testing.T) {
	rec := get(t, "/demo")

	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("got status %d y content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, want := range []string{`fetch("/v1/queries"`, "Gemini, de Google", `href="/docs"`} {
		if !strings.Contains(body, want) {
			t.Errorf("la página no contiene %q", want)
		}
	}
	// Sin recursos externos: no hay nada que fijar con un hash.
	if strings.Contains(body, "<script src") || strings.Contains(body, "<link") {
		t.Error("la página carga recursos externos")
	}
	// El texto que llega de la API nunca se interpreta como HTML.
	if strings.Contains(body, "innerHTML") {
		t.Error("la página usa innerHTML")
	}
}

// La política solo permite el script de la página, por su hash: si alguien
// edita el script sin que el hash cambie con él, el navegador no lo ejecuta.
func TestDemoPageContentSecurityPolicy(t *testing.T) {
	rec := get(t, "/demo")

	policy := rec.Header().Get("Content-Security-Policy")
	_, rest, _ := strings.Cut(rec.Body.String(), "<script>")
	script, _, _ := strings.Cut(rest, "</script>")
	sum := sha256.Sum256([]byte(script))
	want := "script-src 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if !strings.Contains(policy, want) || !strings.Contains(policy, "connect-src 'self'") || !strings.Contains(policy, "default-src 'none'") {
		t.Errorf("got política %q, want que incluya %q", policy, want)
	}
}
