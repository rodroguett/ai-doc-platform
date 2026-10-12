package gateway

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway/api"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
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

func postDocument(t *testing.T, s *Server, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := multipartBody(t, fields)
	req := httptest.NewRequest(http.MethodPost, "/v1/documents", body)
	req.Header.Set("Content-Type", contentType)
	return serve(t, s, authorize(req))
}

func do(t *testing.T, s *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, s, authorize(httptest.NewRequest(method, path, nil)))
}

// upload sube un documento y devuelve su trabajo.
func upload(t *testing.T, s *Server, content, collection string) api.Job {
	t.Helper()
	var job api.Job
	decodeJSON(t, postDocument(t, s, map[string]string{"file": content, "collection": collection}),
		http.StatusAccepted, &job)
	return job
}

func TestCreateDocument(t *testing.T) {
	s := newExampleServer()
	rec := postDocument(t, s, map[string]string{
		"file":       "%PDF-1.7 contenido de ejemplo",
		"collection": "rca",
		"metadata":   `{"source": "RCA 042/2019", "titular": "Minera Loma Seca SpA"}`,
	})

	var job api.Job
	decodeJSON(t, rec, http.StatusAccepted, &job)
	if job.Type != "ingest" || job.Status != "queued" {
		t.Errorf("got job %s/%s, want ingest/queued", job.Type, job.Status)
	}
	if want := "/v1/jobs/" + job.Id.String(); rec.Header().Get("Location") != want {
		t.Errorf("got Location %q, want %q", rec.Header().Get("Location"), want)
	}
	if job.ResourceId == nil {
		t.Fatal("el trabajo no referencia su documento")
	}

	// El trabajo y el documento quedan consultables.
	var gotJob api.Job
	decodeJSON(t, do(t, s, http.MethodGet, rec.Header().Get("Location")), http.StatusOK, &gotJob)
	if gotJob.Id != job.Id {
		t.Errorf("got job %s, want %s", gotJob.Id, job.Id)
	}

	var doc api.Document
	decodeJSON(t, do(t, s, http.MethodGet, "/v1/documents/"+job.ResourceId.String()), http.StatusOK, &doc)
	if doc.Source != "RCA 042/2019" || doc.Collection != "rca" || doc.Status != "pending" {
		t.Errorf("documento inesperado: %+v", doc)
	}
	if doc.Metadata == nil || (*doc.Metadata)["titular"] != "Minera Loma Seca SpA" {
		t.Errorf("los metadatos no se conservaron: %v", doc.Metadata)
	}
}

func TestCreateDocumentUsesFilenameAsSource(t *testing.T) {
	s := newExampleServer()
	job := upload(t, s, "%PDF-1.7", "rca")

	var doc api.Document
	decodeJSON(t, do(t, s, http.MethodGet, "/v1/documents/"+job.ResourceId.String()), http.StatusOK, &doc)
	if doc.Source != "rca-042-2019.pdf" {
		t.Errorf("got source %q, want el nombre del archivo", doc.Source)
	}
}

func TestCreateDocumentRejectsDuplicate(t *testing.T) {
	s := newExampleServer()
	upload(t, s, "%PDF-1.7 mismo contenido", "rca")

	rec := postDocument(t, s, map[string]string{"file": "%PDF-1.7 mismo contenido", "collection": "rca"})
	p := decodeProblem(t, rec, http.StatusConflict)
	if !strings.HasSuffix(p.Type, "/conflict") {
		t.Errorf("got type %q, want conflict", p.Type)
	}

	// El mismo contenido en otra colección es otro documento.
	upload(t, s, "%PDF-1.7 mismo contenido", "normativa")
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
			decodeProblem(t, postDocument(t, newExampleServer(), tt.fields), tt.status)
		})
	}
}

func TestGetUnknownResources(t *testing.T) {
	s := newExampleServer()
	id := uuid.NewString()

	for _, path := range []string{"/v1/jobs/" + id, "/v1/documents/" + id} {
		p := decodeProblem(t, do(t, s, http.MethodGet, path), http.StatusNotFound)
		if !strings.HasSuffix(p.Type, "/not-found") {
			t.Errorf("%s: got type %q, want not-found", path, p.Type)
		}
	}
	decodeProblem(t, do(t, s, http.MethodDelete, "/v1/documents/"+id), http.StatusNotFound)

	// Un identificador que no es UUID es una petición inválida, no un 404.
	decodeProblem(t, do(t, s, http.MethodGet, "/v1/documents/no-es-uuid"), http.StatusBadRequest)
}

func TestListDocumentsPaginates(t *testing.T) {
	s := newExampleServer()
	var want []string // del más reciente al más antiguo
	for i := range 5 {
		job := upload(t, s, fmt.Sprintf("%%PDF-1.7 documento %d", i), "rca")
		want = append([]string{job.ResourceId.String()}, want...)
	}
	upload(t, s, "%PDF-1.7 de otra colección", "normativa")

	var got []string
	path := "/v1/documents?collection=rca&limit=2"
	for pages := 0; path != ""; pages++ {
		if pages > 5 {
			t.Fatal("la paginación no termina")
		}
		var page api.DocumentPage
		decodeJSON(t, do(t, s, http.MethodGet, path), http.StatusOK, &page)
		if len(page.Data) > 2 {
			t.Fatalf("la página trae %d documentos, want a lo más 2", len(page.Data))
		}
		for _, d := range page.Data {
			got = append(got, d.Id.String())
		}
		path = ""
		if page.NextCursor != nil {
			path = "/v1/documents?collection=rca&limit=2&cursor=" + *page.NextCursor
		}
	}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("recorrido del listado:\n got %v\nwant %v", got, want)
	}
}

func TestListDocumentsEmpty(t *testing.T) {
	rec := do(t, newExampleServer(), http.MethodGet, "/v1/documents")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	// Una lista vacía es [], no null: el contrato declara data como arreglo.
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("got body %s, want data vacío", rec.Body)
	}
}

func TestListDocumentsRejectsBadParams(t *testing.T) {
	s := newExampleServer()
	for _, query := range []string{"limit=0", "limit=101", "limit=-1", "cursor=no-es-un-cursor"} {
		decodeProblem(t, do(t, s, http.MethodGet, "/v1/documents?"+query), http.StatusBadRequest)
	}
}

func TestDeleteDocument(t *testing.T) {
	s := newExampleServer()
	job := upload(t, s, "%PDF-1.7", "rca")
	docPath := "/v1/documents/" + job.ResourceId.String()

	if rec := do(t, s, http.MethodDelete, docPath); rec.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body)
	}

	// Desaparecen el documento y su trabajo.
	decodeProblem(t, do(t, s, http.MethodGet, docPath), http.StatusNotFound)
	decodeProblem(t, do(t, s, http.MethodGet, "/v1/jobs/"+job.Id.String()), http.StatusNotFound)

	// Eliminado el documento, el mismo contenido puede volver a subirse.
	upload(t, s, "%PDF-1.7", "rca")
}

func TestDocumentsWithoutDatabase(t *testing.T) {
	s := NewServer(Services{}, WithWriteToken(testToken))
	id := uuid.NewString()

	requests := []*httptest.ResponseRecorder{
		postDocument(t, s, map[string]string{"file": "%PDF-1.7", "collection": "rca"}),
		do(t, s, http.MethodGet, "/v1/documents"),
		do(t, s, http.MethodGet, "/v1/documents/"+id),
		do(t, s, http.MethodDelete, "/v1/documents/"+id),
		do(t, s, http.MethodGet, "/v1/jobs/"+id),
	}
	for _, rec := range requests {
		decodeProblem(t, rec, http.StatusServiceUnavailable)
	}
}

func TestCreateDocumentStoresChunkCount(t *testing.T) {
	s := newExampleServer()
	job := upload(t, s, "%PDF-1.7 contenido", "rca")

	var doc api.Document
	decodeJSON(t, do(t, s, http.MethodGet, "/v1/documents/"+job.ResourceId.String()), http.StatusOK, &doc)
	if doc.ChunkCount == nil || *doc.ChunkCount != 1 {
		t.Errorf("got chunk_count %v, want 1", doc.ChunkCount)
	}
}

// Un archivo que no se puede leer se rechaza en la misma petición, sin
// registrar nada: no queda un trabajo destinado a fallar.
func TestCreateDocumentRejectsUnreadableFiles(t *testing.T) {
	for name, content := range map[string]string{
		"no es un PDF":          "PK\x03\x04 esto es un zip",
		"PDF sin capa de texto": scannedPDF,
	} {
		t.Run(name, func(t *testing.T) {
			s := newExampleServer()
			rec := postDocument(t, s, map[string]string{"file": content, "collection": "rca"})

			p := decodeProblem(t, rec, http.StatusBadRequest)
			if p.Detail == "" {
				t.Error("el rechazo debe explicar el motivo")
			}

			var page api.DocumentPage
			decodeJSON(t, do(t, s, http.MethodGet, "/v1/documents"), http.StatusOK, &page)
			if len(page.Data) != 0 {
				t.Errorf("quedaron %d documentos registrados tras el rechazo", len(page.Data))
			}
		})
	}
}

// El servicio procesa una subida a la vez: mientras una está en curso, la
// siguiente se rechaza de inmediato en lugar de acumularse en memoria.
func TestCreateDocumentOneAtATime(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	s := NewServer(Services{
		Documents: newDocuments(fakeExtractor{hold: release, started: started}, fakeEmbedder{}),
	}, WithWriteToken(testToken))

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		first <- postDocument(t, s, map[string]string{"file": "%PDF-1.7 primero", "collection": "rca"})
	}()
	<-started // la primera subida ya está dentro de la extracción

	rec := postDocument(t, s, map[string]string{"file": "%PDF-1.7 segundo", "collection": "rca"})
	p := decodeProblem(t, rec, http.StatusServiceUnavailable)
	if !strings.HasSuffix(p.Type, "/upload-in-progress") {
		t.Errorf("got type %q, want upload-in-progress", p.Type)
	}

	// Las lecturas no se ven afectadas.
	if rec := do(t, s, http.MethodGet, "/v1/documents"); rec.Code != http.StatusOK {
		t.Errorf("listar durante una subida: got status %d, want %d", rec.Code, http.StatusOK)
	}

	close(release)
	if rec := <-first; rec.Code != http.StatusAccepted {
		t.Fatalf("la primera subida: got status %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Terminada la primera, el servicio vuelve a aceptar.
	rec = postDocument(t, s, map[string]string{"file": "%PDF-1.7 tercero", "collection": "rca"})
	if rec.Code != http.StatusAccepted {
		t.Errorf("tras liberar: got status %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body)
	}
}

func runJob(t *testing.T, s *Server, jobID string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s, http.MethodPost, "/v1/jobs/"+jobID+"/run")
}

// Ejecutar el trabajo genera los embeddings y deja el documento listo. Es lo
// que hace la cola después de cada subida.
func TestRunJobCompletesIngestion(t *testing.T) {
	s := newExampleServer()
	job := upload(t, s, "%PDF-1.7 contenido", "rca")
	if job.Status != "queued" {
		t.Fatalf("recién subido: got estado %q, want queued", job.Status)
	}

	var ran api.Job
	decodeJSON(t, runJob(t, s, job.Id.String()), http.StatusOK, &ran)
	if ran.Status != "succeeded" || ran.Progress == nil || *ran.Progress != 100 {
		t.Errorf("tras ejecutar: got %s con avance %v, want succeeded al 100", ran.Status, ran.Progress)
	}

	var doc api.Document
	decodeJSON(t, do(t, s, http.MethodGet, "/v1/documents/"+job.ResourceId.String()), http.StatusOK, &doc)
	if doc.Status != "ready" {
		t.Errorf("got documento %q, want ready", doc.Status)
	}

	// Ejecutarlo de nuevo es inofensivo: devuelve el mismo estado.
	var again api.Job
	decodeJSON(t, runJob(t, s, job.Id.String()), http.StatusOK, &again)
	if again.Status != "succeeded" {
		t.Errorf("segunda ejecución: got %q, want succeeded", again.Status)
	}
}

func TestRunJobRequiresToken(t *testing.T) {
	s := newExampleServer()
	job := upload(t, s, "%PDF-1.7 contenido", "rca")

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs/"+job.Id.String()+"/run", nil)
	decodeProblem(t, serve(t, s, req), http.StatusUnauthorized)

	// Sin token el trabajo no se ejecutó.
	var got api.Job
	decodeJSON(t, do(t, s, http.MethodGet, "/v1/jobs/"+job.Id.String()), http.StatusOK, &got)
	if got.Status != "queued" {
		t.Errorf("got estado %q, want queued", got.Status)
	}
}

func TestRunJobUnknown(t *testing.T) {
	decodeProblem(t, runJob(t, newExampleServer(), uuid.NewString()), http.StatusNotFound)
}

// unavailableRuns hace que ejecutar un trabajo falle como cuando el
// proveedor de embeddings no responde.
type unavailableRuns struct{ DocumentService }

func (unavailableRuns) RunJob(context.Context, uuid.UUID) (rag.Job, error) {
	return rag.Job{}, fmt.Errorf("%w: el proveedor de embeddings no responde", rag.ErrUnavailable)
}

// El 503 es lo que hace que la cola reintente la pasada más tarde.
func TestRunJobReportsUnavailableProvider(t *testing.T) {
	s := NewServer(Services{
		Documents: unavailableRuns{newDocuments(fakeExtractor{}, fakeEmbedder{})},
	}, WithWriteToken(testToken))

	p := decodeProblem(t, runJob(t, s, uuid.NewString()), http.StatusServiceUnavailable)
	if !strings.HasSuffix(p.Type, "/unavailable") {
		t.Errorf("got type %q, want unavailable", p.Type)
	}
}

// Sin proveedor de embeddings configurado el trabajo no puede ejecutarse, y
// queda en cola en lugar de fallar.
func TestRunJobWithoutEmbedder(t *testing.T) {
	s := NewServer(Services{Documents: newDocuments(fakeExtractor{}, nil)}, WithWriteToken(testToken))
	job := upload(t, s, "%PDF-1.7 contenido", "rca")

	decodeProblem(t, runJob(t, s, job.Id.String()), http.StatusServiceUnavailable)

	var got api.Job
	decodeJSON(t, do(t, s, http.MethodGet, "/v1/jobs/"+job.Id.String()), http.StatusOK, &got)
	if got.Status != "queued" {
		t.Errorf("got estado %q, want queued", got.Status)
	}
}
