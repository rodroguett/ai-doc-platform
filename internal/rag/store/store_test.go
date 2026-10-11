package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	pool := postgrestest.NewPool(t)
	if _, err := postgres.Migrate(context.Background(), pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	return store.New(pool)
}

func newDocument(sha string) (rag.Document, rag.Job) {
	doc := rag.Document{
		ID:            uuid.Must(uuid.NewV7()),
		Source:        "RCA 042/2019",
		Collection:    "rca",
		Metadata:      map[string]any{"titular": "Minera Loma Seca SpA"},
		ContentSHA256: sha,
	}
	job := rag.Job{ID: uuid.Must(uuid.NewV7()), Kind: rag.JobIngest}
	return doc, job
}

func TestCreateDocument(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")

	createdDoc, createdJob, err := s.CreateDocument(ctx, doc, job)
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	if createdDoc.Status != rag.DocumentPending || createdJob.Status != rag.JobQueued {
		t.Errorf("got %s/%s, want pending/queued", createdDoc.Status, createdJob.Status)
	}
	if createdDoc.CreatedAt.IsZero() || createdJob.CreatedAt.IsZero() {
		t.Error("la base de datos no asignó las fechas de creación")
	}

	gotDoc, err := s.Document(ctx, doc.ID)
	if err != nil {
		t.Fatalf("leer el documento: %v", err)
	}
	if gotDoc.Source != doc.Source || gotDoc.Metadata["titular"] != "Minera Loma Seca SpA" {
		t.Errorf("el documento leído difiere del creado: %+v", gotDoc)
	}

	gotJob, err := s.Job(ctx, job.ID)
	if err != nil {
		t.Fatalf("leer el trabajo: %v", err)
	}
	if gotJob.DocumentID != doc.ID || gotJob.Kind != rag.JobIngest || gotJob.Error != "" {
		t.Errorf("el trabajo leído difiere del creado: %+v", gotJob)
	}
}

func TestCreateDocumentWithoutMetadata(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	doc, job := newDocument("aaaa")
	doc.Metadata = nil

	if _, _, err := s.CreateDocument(ctx, doc, job); err != nil {
		t.Fatalf("crear: %v", err)
	}
	got, err := s.Document(ctx, doc.ID)
	if err != nil {
		t.Fatalf("leer: %v", err)
	}
	if got.Metadata == nil || len(got.Metadata) != 0 {
		t.Errorf("got metadata %v, want objeto vacío", got.Metadata)
	}
}

func TestCreateDocumentRejectsDuplicateContent(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	doc, job := newDocument("aaaa")
	if _, _, err := s.CreateDocument(ctx, doc, job); err != nil {
		t.Fatalf("crear el primero: %v", err)
	}

	// Mismo contenido, misma colección: es el mismo documento.
	dup, dupJob := newDocument("aaaa")
	if _, _, err := s.CreateDocument(ctx, dup, dupJob); !errors.Is(err, rag.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	// El rechazo no debe dejar un trabajo huérfano.
	if _, err := s.Job(ctx, dupJob.ID); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("el trabajo del duplicado quedó registrado: %v", err)
	}

	// Mismo contenido en otra colección: es otro documento.
	other, otherJob := newDocument("aaaa")
	other.Collection = "normativa"
	if _, _, err := s.CreateDocument(ctx, other, otherJob); err != nil {
		t.Errorf("mismo contenido en otra colección: %v", err)
	}
}

// Una distancia entre vectores solo se resuelve si la migración instaló la
// extensión y esta es visible desde el esquema de trabajo.
func TestMigrationsEnableVector(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	var distance float64
	if err := pool.QueryRow(ctx, `SELECT '[1,0]'::vector <-> '[0,1]'::vector`).Scan(&distance); err != nil {
		t.Fatalf("pgvector no está disponible: %v", err)
	}
	if distance < 1.41 || distance > 1.42 {
		t.Errorf("got distance %f, want √2", distance)
	}
}

func TestNotFound(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7())

	if _, err := s.Document(ctx, id); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("documento: got %v, want ErrNotFound", err)
	}
	if _, err := s.Job(ctx, id); !errors.Is(err, rag.ErrNotFound) {
		t.Errorf("trabajo: got %v, want ErrNotFound", err)
	}
}

// Con fechas de creación idénticas, el desempate por id es lo único que
// impide que el recorrido repita o salte documentos.
func TestListDocumentsBreaksTiesByID(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	s := store.New(pool)

	for _, sha := range []string{"aaaa", "bbbb", "cccc"} {
		doc, job := newDocument(sha)
		if _, _, err := s.CreateDocument(ctx, doc, job); err != nil {
			t.Fatalf("crear: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE documents SET created_at = '2026-01-01T00:00:00Z'`); err != nil {
		t.Fatalf("igualar las fechas: %v", err)
	}

	seen := map[uuid.UUID]bool{}
	var after *rag.Position
	for range 3 {
		docs, err := s.ListDocuments(ctx, "rca", after, 1)
		if err != nil {
			t.Fatalf("listar: %v", err)
		}
		if len(docs) != 1 {
			t.Fatalf("got %d documentos, want 1", len(docs))
		}
		if seen[docs[0].ID] {
			t.Fatalf("el documento %s se repite", docs[0].ID)
		}
		seen[docs[0].ID] = true
		after = &rag.Position{CreatedAt: docs[0].CreatedAt, ID: docs[0].ID}
	}

	if docs, err := s.ListDocuments(ctx, "rca", after, 1); err != nil || len(docs) != 0 {
		t.Errorf("tras el último documento: got %d y %v, want 0", len(docs), err)
	}
}
