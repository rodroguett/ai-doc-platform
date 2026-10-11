// Package store persiste los documentos y trabajos de rag en Postgres.
package store

import (
	"context"
	"errors"
	"fmt"
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

// CreateDocument registra un documento y su trabajo de ingesta en una sola
// transacción: no puede quedar un documento sin trabajo ni al revés. Devuelve
// ambos con los valores que asignó la base de datos.
//
// Si el contenido ya existe en la colección devuelve rag.ErrDuplicate.
func (s *Store) CreateDocument(ctx context.Context, doc rag.Document, job rag.Job) (rag.Document, rag.Job, error) {
	if doc.Metadata == nil {
		doc.Metadata = map[string]any{}
	}
	job.DocumentID = doc.ID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rag.Document{}, rag.Job{}, fmt.Errorf("iniciar la transacción: %w", err)
	}
	// Tras un Commit exitoso, Rollback no hace nada.
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `
		INSERT INTO documents (id, source, collection, metadata, content_sha256)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING status, chunk_count, created_at`,
		doc.ID, doc.Source, doc.Collection, doc.Metadata, doc.ContentSHA256,
	).Scan(&doc.Status, &doc.ChunkCount, &doc.CreatedAt)
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

	if err := tx.Commit(ctx); err != nil {
		return rag.Document{}, rag.Job{}, fmt.Errorf("confirmar la transacción: %w", err)
	}
	return doc, job, nil
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
	var jobErr *string
	err := s.pool.QueryRow(ctx, `
		SELECT kind, status, progress, document_id, error, created_at, updated_at
		FROM jobs
		WHERE id = $1`, id,
	).Scan(&job.Kind, &job.Status, &job.Progress, &job.DocumentID,
		&jobErr, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return rag.Job{}, rag.ErrNotFound
	}
	if err != nil {
		return rag.Job{}, fmt.Errorf("leer el trabajo: %w", err)
	}
	if jobErr != nil {
		job.Error = *jobErr
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
