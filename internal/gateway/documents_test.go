package gateway

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
)

// multipartBody construye un formulario. Una clave "file" se envía como
// archivo; el resto como campos de texto.
func multipartBody(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for name, value := range fields {
		fw, err := w.CreateFormField(name)
		if name == "file" {
			fw, err = w.CreateFormFile(name, "rca-042-2019.pdf")
		}
		if err == nil {
			_, err = fw.Write([]byte(value))
		}
		if err != nil {
			t.Fatalf("armar multipart: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("cerrar multipart: %v", err)
	}
	return &buf, w.FormDataContentType()
}

func postDocument(t *testing.T, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartBody(t, fields)
	req := httptest.NewRequest(http.MethodPost, "/v1/documents", body)
	req.Header.Set("Content-Type", contentType)
	return serve(t, newExampleServer(), req)
}

func TestCreateDocument(t *testing.T) {
	rec := postDocument(t, map[string]string{
		"file":       "%PDF-1.7 contenido de ejemplo",
		"collection": "rca",
		"metadata":   `{"titular": "Minera Loma Seca SpA"}`,
	})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body)
	}
	var job api.Job
	if err := json.NewDecoder(rec.Body).Decode(&job); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if job.Type != "ingest" || job.Status != "queued" {
		t.Errorf("got job %s/%s, want ingest/queued", job.Type, job.Status)
	}
	if want := "/v1/jobs/" + job.Id.String(); rec.Header().Get("Location") != want {
		t.Errorf("got Location %q, want %q", rec.Header().Get("Location"), want)
	}
}

func TestCreateDocumentErrors(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]string
		status int
	}{
		{
			name:   "sin colección",
			fields: map[string]string{"file": "%PDF-1.7"},
			status: http.StatusBadRequest,
		},
		{
			name:   "archivo vacío",
			fields: map[string]string{"file": "", "collection": "rca"},
			status: http.StatusBadRequest,
		},
		{
			name:   "metadata no es JSON",
			fields: map[string]string{"file": "%PDF-1.7", "collection": "rca", "metadata": "titular=x"},
			status: http.StatusBadRequest,
		},
		{
			name:   "campo demasiado grande",
			fields: map[string]string{"file": "%PDF-1.7", "collection": strings.Repeat("x", maxFieldBytes+1)},
			status: http.StatusRequestEntityTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decodeProblem(t, postDocument(t, tt.fields), tt.status)
		})
	}
}
