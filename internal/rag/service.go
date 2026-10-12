package rag

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
	// CreateDocument registra el documento, su trabajo y sus fragmentos, o
	// nada.
	CreateDocument(ctx context.Context, doc Document, job Job, chunks []Chunk) (Document, Job, error)

	// DocumentExists indica si la colección ya tiene un documento con ese
	// contenido.
	DocumentExists(ctx context.Context, collection, contentSHA256 string) (bool, error)
	Document(ctx context.Context, id uuid.UUID) (Document, error)
	Job(ctx context.Context, id uuid.UUID) (Job, error)

	// ListDocuments devuelve hasta limit documentos posteriores a after en
	// el orden del listado, del más reciente al más antiguo.
	ListDocuments(ctx context.Context, collection string, after *Position, limit int) ([]Document, error)

	// DeleteDocument elimina el documento y todo lo que depende de él.
	DeleteDocument(ctx context.Context, id uuid.UUID) error
}

// TextExtractor obtiene el texto de un documento, página por página. Un
// archivo ilegible devuelve un error que envuelve ErrInvalidInput; uno sin
// texto, ErrNoTextLayer.
type TextExtractor interface {
	Extract(ctx context.Context, data []byte) ([]PageText, error)
}

// Splitter divide el texto de un documento en fragmentos con su ubicación.
type Splitter func(pages []PageText) []Chunk

// Service registra y consulta documentos y sus trabajos de ingesta.
type Service struct {
	store     Store
	extractor TextExtractor
	split     Splitter
}

// NewService recibe las piezas que implementan los subpaquetes de rag. El
// paquete raíz no puede importarlos, porque ellos lo importan a él: las
// conecta el binario.
func NewService(store Store, extractor TextExtractor, split Splitter) *Service {
	return &Service{store: store, extractor: extractor, split: split}
}

// Submit registra un documento: extrae su texto, lo fragmenta y guarda los
// fragmentos junto con el documento y su trabajo de ingesta. Lo que queda
// para el trabajo es generar los embeddings.
//
// La extracción ocurre aquí y no en segundo plano por tres razones: tarda
// segundos, un archivo inválido se rechaza con una respuesta inmediata en
// lugar de un trabajo fallido, y no hace falta guardar el archivo original.
//
// El documento se identifica por el hash de su contenido: subir dos veces el
// mismo archivo a la misma colección devuelve ErrDuplicate.
func (s *Service) Submit(ctx context.Context, u Upload) (Job, error) {
	filename := strings.TrimSpace(u.Filename)
	collection := strings.TrimSpace(u.Collection)
	switch {
	case filename == "":
		return Job{}, fmt.Errorf("%w: el archivo no tiene nombre", ErrInvalidInput)
	case collection == "":
		return Job{}, fmt.Errorf("%w: falta la colección", ErrInvalidInput)
	case len(u.Content) == 0:
		return Job{}, fmt.Errorf("%w: el archivo está vacío", ErrInvalidInput)
	}

	sum := sha256.Sum256(u.Content)
	contentHash := hex.EncodeToString(sum[:])

	// Extraer cuesta segundos y cientos de megabytes: un duplicado se
	// rechaza antes. La restricción de unicidad de la base de datos sigue
	// cubriendo la carrera entre dos subidas simultáneas.
	exists, err := s.store.DocumentExists(ctx, collection, contentHash)
	if err != nil {
		return Job{}, err
	}
	if exists {
		return Job{}, ErrDuplicate
	}

	pages, err := s.extractor.Extract(ctx, u.Content)
	if err != nil {
		return Job{}, err
	}
	chunks := s.split(pages)
	if len(chunks) == 0 {
		return Job{}, ErrNoTextLayer
	}
	for i := range chunks {
		if chunks[i].ID, err = uuid.NewV7(); err != nil {
			return Job{}, fmt.Errorf("generar el identificador de un fragmento: %w", err)
		}
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
			ChunkCount:    len(chunks),
			Metadata:      u.Metadata,
			ContentSHA256: contentHash,
		},
		Job{ID: jobID, Kind: JobIngest},
		chunks,
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
