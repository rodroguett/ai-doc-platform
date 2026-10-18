// Package store persiste los documentos y trabajos de rag en Postgres.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// uniqueViolation es el código SQLSTATE de una restricción de unicidad.
const uniqueViolation = "23505"

// Store lee y escribe documentos y trabajos.
type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateDocument registra un documento, su trabajo de ingesta y sus
// fragmentos en una sola transacción: no puede quedar un documento sin
// trabajo, ni fragmentos de un documento que no se registró. Devuelve el
// documento y el trabajo con los valores que asignó la base de datos.
//
// Si el contenido ya existe en la colección devuelve rag.ErrDuplicate.
func (s *Store) CreateDocument(ctx context.Context, doc rag.Document, job rag.Job, chunks []rag.Chunk) (rag.Document, rag.Job, error) {
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	doc.ChunkCount = len(chunks)
	job.DocumentID = doc.ID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rag.Document{}, rag.Job{}, fmt.Errorf("iniciar la transacción: %w", err)
	}
	// Tras un Commit exitoso, Rollback no hace nada.
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		INSERT INTO documents (id, source, collection, chunk_count, metadata, content_sha256)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING status, created_at`,
		doc.ID, doc.Source, doc.Collection, doc.ChunkCount, doc.Metadata, doc.ContentSHA256,
	).Scan(&doc.Status, &doc.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return rag.Document{}, rag.Job{}, rag.ErrDuplicate
		}
		return rag.Document{}, rag.Job{}, fmt.Errorf("insertar el documento: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO jobs (id, kind, document_id)
		VALUES ($1, $2, $3)
		RETURNING status, progress, created_at, updated_at`,
		job.ID, job.Kind, job.DocumentID,
	).Scan(&job.Status, &job.Progress, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return rag.Document{}, rag.Job{}, fmt.Errorf("insertar el trabajo: %w", err)
	}

	// COPY inserta todos los fragmentos en un solo viaje a la base de datos.
	_, err = tx.CopyFrom(ctx,
		pgx.Identifier{"chunks"},
		[]string{"id", "document_id", "position", "section", "heading", "context", "page_start", "page_end", "content"},
		pgx.CopyFromSlice(len(chunks), func(i int) ([]any, error) {
			c := chunks[i]
			return []any{c.ID, doc.ID, c.Index, c.Section, c.Heading, c.Context, c.PageStart, c.PageEnd, c.Text}, nil
		}),
	)
	if err != nil {
		return rag.Document{}, rag.Job{}, fmt.Errorf("insertar los fragmentos: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return rag.Document{}, rag.Job{}, fmt.Errorf("confirmar la transacción: %w", err)
	}
	return doc, job, nil
}

// DocumentExists indica si la colección ya tiene un documento con ese
// contenido.
func (s *Store) DocumentExists(ctx context.Context, collection, contentSHA256 string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM documents WHERE collection = $1 AND content_sha256 = $2)`,
		collection, contentSHA256,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("buscar el documento: %w", err)
	}
	return exists, nil
}

// Chunks devuelve los fragmentos de un documento, en orden.
func (s *Store) Chunks(ctx context.Context, documentID uuid.UUID) ([]rag.Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, position, section, heading, context, page_start, page_end, content
		FROM chunks
		WHERE document_id = $1
		ORDER BY position`, documentID)
	if err != nil {
		return nil, fmt.Errorf("listar los fragmentos: %w", err)
	}
	chunks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (rag.Chunk, error) {
		var c rag.Chunk
		err := row.Scan(&c.ID, &c.Index, &c.Section, &c.Heading, &c.Context, &c.PageStart, &c.PageEnd, &c.Text)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("leer los fragmentos: %w", err)
	}
	return chunks, nil
}

// Document devuelve un documento, o rag.ErrNotFound si no existe.
func (s *Store) Document(ctx context.Context, id uuid.UUID) (rag.Document, error) {
	doc := rag.Document{ID: id}
	err := s.pool.QueryRow(ctx, `
		SELECT source, collection, status, chunk_count, metadata, content_sha256, created_at
		FROM documents
		WHERE id = $1`, id,
	).Scan(&doc.Source, &doc.Collection, &doc.Status, &doc.ChunkCount,
		&doc.Metadata, &doc.ContentSHA256, &doc.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return rag.Document{}, rag.ErrNotFound
	}
	if err != nil {
		return rag.Document{}, fmt.Errorf("leer el documento: %w", err)
	}
	return doc, nil
}

// Job devuelve un trabajo, o rag.ErrNotFound si no existe.
func (s *Store) Job(ctx context.Context, id uuid.UUID) (rag.Job, error) {
	job := rag.Job{ID: id}
	err := scanJob(s.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = $1`, id), &job)
	if errors.Is(err, pgx.ErrNoRows) {
		return rag.Job{}, rag.ErrNotFound
	}
	if err != nil {
		return rag.Job{}, fmt.Errorf("leer el trabajo: %w", err)
	}
	return job, nil
}

// ListDocuments devuelve hasta limit documentos, del más reciente al más
// antiguo, a partir del que sigue a after. Una colección vacía no filtra.
func (s *Store) ListDocuments(ctx context.Context, collection string, after *rag.Position, limit int) ([]rag.Document, error) {
	// Los parámetros nulos desactivan su condición, de modo que una sola
	// consulta cubre las cuatro combinaciones de filtro y cursor.
	var (
		collectionArg *string
		afterTime     *time.Time
		afterID       *uuid.UUID
	)
	if collection != "" {
		collectionArg = &collection
	}
	if after != nil {
		afterTime, afterID = &after.CreatedAt, &after.ID
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, source, collection, status, chunk_count, metadata, content_sha256, created_at
		FROM documents
		WHERE ($1::text IS NULL OR collection = $1)
		  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3::uuid))
		ORDER BY created_at DESC, id DESC
		LIMIT $4`, collectionArg, afterTime, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("listar los documentos: %w", err)
	}

	docs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (rag.Document, error) {
		var doc rag.Document
		err := row.Scan(&doc.ID, &doc.Source, &doc.Collection, &doc.Status, &doc.ChunkCount,
			&doc.Metadata, &doc.ContentSHA256, &doc.CreatedAt)
		return doc, err
	})
	if err != nil {
		return nil, fmt.Errorf("leer los documentos: %w", err)
	}
	return docs, nil
}

// DeleteDocument elimina un documento y, por la clave foránea, sus trabajos.
// Devuelve rag.ErrNotFound si no existe.
func (s *Store) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("eliminar el documento: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return rag.ErrNotFound
	}
	return nil
}

var _ rag.Store = (*Store)(nil)

const jobColumns = `kind, status, progress, document_id, error, created_at, updated_at`

func scanJob(row pgx.Row, job *rag.Job) error {
	var jobErr *string
	if err := row.Scan(&job.Kind, &job.Status, &job.Progress, &job.DocumentID,
		&jobErr, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return err
	}
	job.Error = ""
	if jobErr != nil {
		job.Error = *jobErr
	}
	return nil
}

// ClaimJob reserva un trabajo pendiente o en curso por la duración indicada,
// y lo deja en curso junto con su documento. Devuelve false, con el estado
// actual del trabajo, si ya terminó o si otra pasada lo tiene reservado.
//
// La reserva vence sola: una pasada que muere sin liberar no deja el trabajo
// trabado para siempre.
func (s *Store) ClaimJob(ctx context.Context, id uuid.UUID, lease time.Duration) (rag.Job, bool, error) {
	job := rag.Job{ID: id}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rag.Job{}, false, fmt.Errorf("iniciar la transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = scanJob(tx.QueryRow(ctx, `
		UPDATE jobs
		SET status = 'running', locked_until = now() + make_interval(secs => $2), updated_at = now()
		WHERE id = $1
		  AND status IN ('queued', 'running')
		  AND (locked_until IS NULL OR locked_until < now())
		RETURNING `+jobColumns, id, lease.Seconds()), &job)
	if errors.Is(err, pgx.ErrNoRows) {
		// No se pudo reservar: o no existe, o terminó, o está reservado.
		current, err := s.Job(ctx, id)
		return current, false, err
	}
	if err != nil {
		return rag.Job{}, false, fmt.Errorf("reservar el trabajo: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE documents SET status = 'processing' WHERE id = $1 AND status = 'pending'`,
		job.DocumentID); err != nil {
		return rag.Job{}, false, fmt.Errorf("marcar el documento en proceso: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return rag.Job{}, false, fmt.Errorf("confirmar la transacción: %w", err)
	}
	return job, true, nil
}

// ReleaseJob libera la reserva de un trabajo sin cambiar su estado.
func (s *Store) ReleaseJob(ctx context.Context, id uuid.UUID) error {
	if _, err := s.pool.Exec(ctx, `UPDATE jobs SET locked_until = NULL WHERE id = $1`, id); err != nil {
		return fmt.Errorf("liberar el trabajo: %w", err)
	}
	return nil
}

// PendingChunks devuelve hasta limit fragmentos del documento que aún no
// tienen embedding, en orden.
func (s *Store) PendingChunks(ctx context.Context, documentID uuid.UUID, limit int) ([]rag.Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, position, section, heading, context, page_start, page_end, content
		FROM chunks
		WHERE document_id = $1 AND embedding IS NULL
		ORDER BY position
		LIMIT $2`, documentID, limit)
	if err != nil {
		return nil, fmt.Errorf("listar los fragmentos pendientes: %w", err)
	}
	chunks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (rag.Chunk, error) {
		var c rag.Chunk
		err := row.Scan(&c.ID, &c.Index, &c.Section, &c.Heading, &c.Context, &c.PageStart, &c.PageEnd, &c.Text)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("leer los fragmentos pendientes: %w", err)
	}
	return chunks, nil
}

// SaveEmbeddings guarda los vectores de un grupo de fragmentos y actualiza
// el avance del trabajo, en una transacción: el avance informado nunca se
// adelanta a lo que efectivamente quedó guardado.
func (s *Store) SaveEmbeddings(ctx context.Context, jobID uuid.UUID, model string, embeddings []rag.ChunkEmbedding) (rag.Job, error) {
	job := rag.Job{ID: jobID}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rag.Job{}, fmt.Errorf("iniciar la transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, e := range embeddings {
		batch.Queue(`UPDATE chunks SET embedding = $1::public.vector, embedding_model = $2 WHERE id = $3`,
			vectorLiteral(e.Vector), model, e.ChunkID)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return rag.Job{}, fmt.Errorf("guardar los embeddings: %w", err)
	}

	err = scanJob(tx.QueryRow(ctx, `
		UPDATE jobs
		SET progress = (
			SELECT CASE WHEN count(*) = 0 THEN 0
			            ELSE (count(*) FILTER (WHERE c.embedding IS NOT NULL)) * 100 / count(*) END
			FROM chunks c WHERE c.document_id = jobs.document_id
		), updated_at = now()
		WHERE id = $1
		RETURNING `+jobColumns, jobID), &job)
	if errors.Is(err, pgx.ErrNoRows) {
		return rag.Job{}, rag.ErrNotFound
	}
	if err != nil {
		return rag.Job{}, fmt.Errorf("actualizar el avance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return rag.Job{}, fmt.Errorf("confirmar la transacción: %w", err)
	}
	return job, nil
}

// FinishJob cierra un trabajo como exitoso o fallido, deja a su documento en
// el estado correspondiente y libera la reserva.
func (s *Store) FinishJob(ctx context.Context, id uuid.UUID, status rag.JobStatus, message string) (rag.Job, error) {
	job := rag.Job{ID: id}
	documentStatus := rag.DocumentReady
	if status != rag.JobSucceeded {
		documentStatus = rag.DocumentFailed
	}
	var jobErr *string
	if message != "" {
		jobErr = &message
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rag.Job{}, fmt.Errorf("iniciar la transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = scanJob(tx.QueryRow(ctx, `
		UPDATE jobs
		SET status = $2, error = $3, locked_until = NULL, updated_at = now(),
		    progress = CASE WHEN $2 = 'succeeded' THEN 100 ELSE progress END
		WHERE id = $1
		RETURNING `+jobColumns, id, status, jobErr), &job)
	if errors.Is(err, pgx.ErrNoRows) {
		return rag.Job{}, rag.ErrNotFound
	}
	if err != nil {
		return rag.Job{}, fmt.Errorf("cerrar el trabajo: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE documents SET status = $2 WHERE id = $1`, job.DocumentID, documentStatus); err != nil {
		return rag.Job{}, fmt.Errorf("actualizar el documento: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return rag.Job{}, fmt.Errorf("confirmar la transacción: %w", err)
	}
	return job, nil
}

// vectorLiteral escribe un vector en el formato de texto de pgvector.
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 12)
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// Search devuelve los pasajes cuyo vector es más parecido al dado, del más al
// menos parecido.
//
// Solo considera documentos listos: los fragmentos de uno a medio ingerir
// darían una imagen parcial del documento. Y solo vectores del modelo
// indicado: los de otro modelo no son comparables.
//
// La búsqueda es exacta: compara la consulta contra todos los vectores, sin
// índice. Con miles de fragmentos toma milisegundos y no puede omitir un
// resultado. Un índice aproximado (HNSW) lo necesitaría un corpus mucho
// mayor, y trae un costo propio: combinado con estos filtros puede devolver
// menos pasajes de los pedidos.
func (s *Store) Search(ctx context.Context, vector []float32, model string, opts rag.SearchOptions) ([]rag.Passage, error) {
	var collection *string
	if opts.Collection != "" {
		collection = &opts.Collection
	}
	// Los filtros se comparan por contención: el documento debe tener al
	// menos esos pares en sus metadatos.
	var filters *string
	if len(opts.Filters) > 0 {
		raw, err := json.Marshal(opts.Filters)
		if err != nil {
			return nil, fmt.Errorf("serializar los filtros: %w", err)
		}
		s := string(raw)
		filters = &s
	}

	// `<=>` es la distancia coseno de pgvector, entre 0 y 2. La similitud es
	// su complemento; se acota a cero porque un puntaje negativo no dice
	// nada útil a quien lee la cita.
	//
	// El orden resta a la distancia la ventaja del documento vinculante
	// (ADR-0015), que equivale a sumarla a la similitud; el puntaje devuelto
	// no la incluye.
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.document_id, d.source, c.section, c.heading, c.context, c.page_start, c.page_end, c.content,
		       GREATEST(0, 1 - (c.embedding <=> $1::public.vector)) AS score
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.embedding IS NOT NULL
		  AND c.embedding_model = $2
		  AND d.status = 'ready'
		  AND ($3::text IS NULL OR d.collection = $3)
		  AND ($8::jsonb IS NULL OR d.metadata @> $8::jsonb)
		ORDER BY (c.embedding <=> $1::public.vector)
		         - CASE WHEN d.metadata->>$5 = $6 THEN $7::float8 ELSE 0 END,
		         c.id
		LIMIT $4`, vectorLiteral(vector), model, collection, opts.TopK,
		rag.MetadataType, rag.BindingType, rag.BindingBonus, filters)
	if err != nil {
		return nil, fmt.Errorf("buscar los pasajes: %w", err)
	}

	passages, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (rag.Passage, error) {
		var p rag.Passage
		err := row.Scan(&p.ChunkID, &p.DocumentID, &p.Source, &p.Section, &p.Heading, &p.Context,
			&p.PageStart, &p.PageEnd, &p.Text, &p.Score)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("leer los pasajes: %w", err)
	}
	return passages, nil
}
