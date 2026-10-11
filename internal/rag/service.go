package rag

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Límites del listado. Coinciden con los del contrato HTTP, pero se validan
// aquí porque son reglas del dominio.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// metadataSource es la clave de los metadatos con la que quien sube un
// documento puede darle un nombre legible distinto del nombre del archivo.
const metadataSource = "source"

// Position identifica un documento dentro del orden del listado.
type Position struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Store es la persistencia que el servicio necesita. La implementa el
// subpaquete store; el binario las conecta.
type Store interface {
	CreateDocument(ctx context.Context, doc Document, job Job) (Document, Job, error)
	Document(ctx context.Context, id uuid.UUID) (Document, error)
	Job(ctx context.Context, id uuid.UUID) (Job, error)

	// ListDocuments devuelve hasta limit documentos posteriores a after en
	// el orden del listado, del más reciente al más antiguo.
	ListDocuments(ctx context.Context, collection string, after *Position, limit int) ([]Document, error)

	// DeleteDocument elimina el documento y todo lo que depende de él.
	DeleteDocument(ctx context.Context, id uuid.UUID) error
}

// Service registra y consulta documentos y sus trabajos de ingesta.
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// Submit registra un documento y encola su ingesta. El contenido se lee
// completo para identificarlo por su hash: subir dos veces el mismo archivo
// a la misma colección devuelve ErrDuplicate.
//
// El contenido todavía no se conserva: la ingesta que lo procesa no existe.
func (s *Service) Submit(ctx context.Context, u Upload) (Job, error) {
	filename := strings.TrimSpace(u.Filename)
	collection := strings.TrimSpace(u.Collection)
	switch {
	case filename == "":
		return Job{}, fmt.Errorf("%w: el archivo no tiene nombre", ErrInvalidInput)
	case collection == "":
		return Job{}, fmt.Errorf("%w: falta la colección", ErrInvalidInput)
	case u.Content == nil:
		return Job{}, fmt.Errorf("%w: falta el contenido del archivo", ErrInvalidInput)
	}

	hash := sha256.New()
	n, err := io.Copy(hash, u.Content)
	if err != nil {
		return Job{}, fmt.Errorf("leer el documento: %w", err)
	}
	if n == 0 {
		return Job{}, fmt.Errorf("%w: el archivo está vacío", ErrInvalidInput)
	}

	source := filename
	if name, ok := u.Metadata[metadataSource].(string); ok && strings.TrimSpace(name) != "" {
		source = strings.TrimSpace(name)
	}

	docID, err := uuid.NewV7()
	if err != nil {
		return Job{}, fmt.Errorf("generar el identificador del documento: %w", err)
	}
	jobID, err := uuid.NewV7()
	if err != nil {
		return Job{}, fmt.Errorf("generar el identificador del trabajo: %w", err)
	}

	_, job, err := s.store.CreateDocument(ctx,
		Document{
			ID:            docID,
			Source:        source,
			Collection:    collection,
			Metadata:      u.Metadata,
			ContentSHA256: hex.EncodeToString(hash.Sum(nil)),
		},
		Job{ID: jobID, Kind: JobIngest},
	)
	return job, err
}

func (s *Service) Job(ctx context.Context, id uuid.UUID) (Job, error) {
	return s.store.Job(ctx, id)
}

func (s *Service) Document(ctx context.Context, id uuid.UUID) (Document, error) {
	return s.store.Document(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteDocument(ctx, id)
}

// Documents devuelve una página del listado. Pagina por posición y no por
// desplazamiento: un documento que se ingiere mientras alguien recorre el
// listado no desplaza las páginas siguientes.
func (s *Service) Documents(ctx context.Context, opts ListOptions) (Page, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit < 1 || limit > maxPageSize {
		return Page{}, fmt.Errorf("%w: limit debe estar entre 1 y %d", ErrInvalidInput, maxPageSize)
	}

	var after *Position
	if opts.Cursor != "" {
		pos, err := decodeCursor(opts.Cursor)
		if err != nil {
			return Page{}, err
		}
		after = &pos
	}

	// Se pide uno de más para saber si hay página siguiente sin contar.
	docs, err := s.store.ListDocuments(ctx, strings.TrimSpace(opts.Collection), after, limit+1)
	if err != nil {
		return Page{}, err
	}

	page := Page{Documents: docs}
	if len(docs) > limit {
		page.Documents = docs[:limit]
		last := page.Documents[limit-1]
		page.NextCursor = encodeCursor(Position{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, nil
}

func encodeCursor(p Position) string {
	raw := p.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + p.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (Position, error) {
	invalid := fmt.Errorf("%w: el cursor no es válido", ErrInvalidInput)

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return Position{}, invalid
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return Position{}, invalid
	}
	createdAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Position{}, invalid
	}
	docID, err := uuid.Parse(id)
	if err != nil {
		return Position{}, invalid
	}
	return Position{CreatedAt: createdAt, ID: docID}, nil
}
