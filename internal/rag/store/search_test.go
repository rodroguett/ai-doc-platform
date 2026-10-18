package store_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

// at devuelve un vector unitario de 768 dimensiones a los grados indicados
// del eje de la consulta. Su similitud coseno con la consulta es el coseno
// de ese ángulo.
func at(degrees float64) []float32 {
	v := make([]float32, 768)
	rad := degrees * math.Pi / 180
	v[0], v[1] = float32(math.Cos(rad)), float32(math.Sin(rad))
	return v
}

var queryVector = at(0)

// ingestWith registra un documento listo cuyos fragmentos tienen los
// vectores dados, y devuelve los identificadores de sus fragmentos.
func ingestWith(t *testing.T, s *store.Store, collection, source, model string, angles ...float64) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	return ingestTyped(t, s, collection, source, "", model, angles...)
}

// ingestTyped es ingestWith con el tipo de documento en sus metadatos.
func ingestTyped(t *testing.T, s *store.Store, collection, source, kind, model string, angles ...float64) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	doc, job := newDocument(source + collection)
	doc.Source, doc.Collection = source, collection
	if kind != "" {
		doc.Metadata[rag.MetadataType] = kind
	}

	chunks := newChunks(len(angles))
	embeddings := make([]rag.ChunkEmbedding, len(angles))
	ids := make([]uuid.UUID, len(angles))
	for i, a := range angles {
		chunks[i].Text = fmt.Sprintf("%s a %v grados", source, a)
		embeddings[i] = rag.ChunkEmbedding{ChunkID: chunks[i].ID, Vector: at(a)}
		ids[i] = chunks[i].ID
	}
	if _, _, err := s.CreateDocument(ctx, doc, job, chunks); err != nil {
		t.Fatalf("crear: %v", err)
	}
	if _, err := s.SaveEmbeddings(ctx, job.ID, model, embeddings); err != nil {
		t.Fatalf("guardar los embeddings: %v", err)
	}
	if _, err := s.FinishJob(ctx, job.ID, rag.JobSucceeded, ""); err != nil {
		t.Fatalf("cerrar el trabajo: %v", err)
	}
	return doc.ID, ids
}

func search(t *testing.T, s *store.Store, model string, opts rag.SearchOptions) []rag.Passage {
	t.Helper()
	passages, err := s.Search(context.Background(), queryVector, model, opts)
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	return passages
}

func TestSearchOrdersBySimilarity(t *testing.T) {
	s := newStore(t)
	docID, ids := ingestWith(t, s, "rca", "RCA Tres Valles", "modelo-a", 60, 0, 120, 30)

	got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10})

	// Del más parecido al menos: 0°, 30°, 60°, 120°.
	wantOrder := []uuid.UUID{ids[1], ids[3], ids[0], ids[2]}
	wantScore := []float64{1, math.Sqrt(3) / 2, 0.5, 0} // el coseno de 120° es negativo: se acota a cero
	if len(got) != 4 {
		t.Fatalf("got %d pasajes, want 4", len(got))
	}
	for i, p := range got {
		if p.ChunkID != wantOrder[i] {
			t.Errorf("posición %d: got el fragmento %q", i, p.Text)
		}
		if math.Abs(p.Score-wantScore[i]) > 1e-4 {
			t.Errorf("posición %d: got puntaje %.4f, want %.4f", i, p.Score, wantScore[i])
		}
	}

	// El pasaje trae lo necesario para citarlo.
	first := got[0]
	if first.DocumentID != docID || first.Source != "RCA Tres Valles" {
		t.Errorf("got documento %s y fuente %q", first.DocumentID, first.Source)
	}
	if first.Section != "Considerando 2" || first.Heading != "Título 2" || first.Context != "2. Que, el proyecto contempla lo siguiente:" || first.PageStart != 2 || first.PageEnd != 3 {
		t.Errorf("la ubicación no coincide con la del fragmento: %+v", first)
	}
	if first.Text != "RCA Tres Valles a 0 grados" {
		t.Errorf("got texto %q", first.Text)
	}
}

func TestSearchLimitsResults(t *testing.T) {
	s := newStore(t)
	_, ids := ingestWith(t, s, "rca", "RCA", "modelo-a", 10, 20, 30, 40, 50)

	got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 2})

	if len(got) != 2 || got[0].ChunkID != ids[0] || got[1].ChunkID != ids[1] {
		t.Errorf("got %d pasajes; want los dos más parecidos", len(got))
	}
}

func TestSearchFiltersByCollection(t *testing.T) {
	s := newStore(t)
	ingestWith(t, s, "rca", "RCA", "modelo-a", 10)
	ingestWith(t, s, "normativa", "Decreto", "modelo-a", 5)

	if got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10}); len(got) != 2 || got[0].Source != "Decreto" {
		t.Errorf("sin filtro: got %d pasajes, el primero de %q", len(got), got[0].Source)
	}
	if got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10, Collection: "rca"}); len(got) != 1 || got[0].Source != "RCA" {
		t.Errorf("con filtro: got %+v", got)
	}
	if got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10, Collection: "inexistente"}); len(got) != 0 {
		t.Errorf("colección inexistente: got %d pasajes", len(got))
	}
}

// Los vectores de otro modelo viven en otro espacio: su parecido con la
// consulta no significa nada, así que no deben aparecer.
func TestSearchIgnoresVectorsFromOtherModels(t *testing.T) {
	s := newStore(t)
	ingestWith(t, s, "rca", "Con el modelo viejo", "modelo-viejo", 0)
	ingestWith(t, s, "rca", "Con el modelo actual", "modelo-a", 45)

	got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10})

	if len(got) != 1 || got[0].Source != "Con el modelo actual" {
		t.Errorf("got %+v; want solo el pasaje del modelo actual", got)
	}
}

// Un documento a medio ingerir daría una imagen parcial de sí mismo.
func TestSearchOnlyConsidersReadyDocuments(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	ingestWith(t, s, "rca", "Listo", "modelo-a", 45)

	// Un documento en proceso: un fragmento ya tiene vector, el otro no.
	doc, job := newDocument("en-proceso")
	doc.Source = "En proceso"
	chunks := newChunks(2)
	if _, _, err := s.CreateDocument(ctx, doc, job, chunks); err != nil {
		t.Fatalf("crear: %v", err)
	}
	if _, err := s.SaveEmbeddings(ctx, job.ID, "modelo-a", []rag.ChunkEmbedding{{ChunkID: chunks[0].ID, Vector: at(0)}}); err != nil {
		t.Fatalf("guardar: %v", err)
	}

	got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10})
	if len(got) != 1 || got[0].Source != "Listo" {
		t.Fatalf("got %+v; want solo el documento listo", got)
	}

	// Cerrado como fallido, tampoco.
	if _, err := s.FinishJob(ctx, job.ID, rag.JobFailed, "falló"); err != nil {
		t.Fatalf("cerrar: %v", err)
	}
	if got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10}); len(got) != 1 {
		t.Errorf("un documento fallido aparece en la búsqueda: %d pasajes", len(got))
	}
}

func TestSearchEmptyCorpus(t *testing.T) {
	s := newStore(t)
	if got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 5}); len(got) != 0 {
		t.Errorf("got %d pasajes de un corpus vacío", len(got))
	}
}

// El documento vinculante gana el orden cuando su similitud está a menos de
// la ventaja de otro, y lo pierde cuando está más lejos (ADR-0015). El
// puntaje informado no incluye la ventaja.
func TestSearchPrioritizesBindingDocument(t *testing.T) {
	s := newStore(t)
	// cos 30° = 0,866; cos 28° = 0,883 (0,017 más); cos 25° = 0,906 (0,040 más).
	_, rca := ingestTyped(t, s, "rca", "RCA Tres Valles", rag.BindingType, "modelo-a", 30)
	_, icsara := ingestTyped(t, s, "rca", "ICSARA Tres Valles", "ICSARA", "modelo-a", 28, 25)

	got := search(t, s, "modelo-a", rag.SearchOptions{TopK: 10})

	want := []uuid.UUID{icsara[1], rca[0], icsara[0]}
	if len(got) != 3 {
		t.Fatalf("got %d pasajes, want 3", len(got))
	}
	for i, p := range got {
		if p.ChunkID != want[i] {
			t.Errorf("posición %d: got %q", i, p.Text)
		}
	}
	if math.Abs(got[1].Score-math.Sqrt(3)/2) > 1e-4 {
		t.Errorf("el puntaje de la RCA incluye la ventaja: %.4f", got[1].Score)
	}
}
