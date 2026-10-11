// Package rag es el contrato del módulo de ingesta y búsqueda: los tipos que
// cruzan su frontera y los errores que otros módulos pueden distinguir.
//
// Es lo único de rag que otro módulo puede importar (ADR-0005). Los
// subpaquetes son implementación privada.
package rag

import (
	"embed"
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound indica que el recurso pedido no existe.
	ErrNotFound = errors.New("no encontrado")

	// ErrDuplicate indica que el documento ya existe en la colección.
	ErrDuplicate = errors.New("documento duplicado")

	// ErrInvalidInput indica que la entrada no cumple las reglas del
	// dominio. Los errores que lo envuelven llevan un mensaje apto para el
	// cliente.
	ErrInvalidInput = errors.New("entrada inválida")
)

// Upload es un documento enviado para ingesta.
type Upload struct {
	Filename   string
	Collection string
	Metadata   map[string]any
	Content    io.Reader
}

// ListOptions acota un listado de documentos.
type ListOptions struct {
	Collection string // vacía: todas las colecciones
	Cursor     string // vacío: desde el más reciente
	Limit      int    // cero: el valor por omisión
}

// Page es una página de documentos, del más reciente al más antiguo.
type Page struct {
	Documents []Document

	// NextCursor es el cursor de la página siguiente, o vacío si no hay
	// más. Es opaco: su formato puede cambiar.
	NextCursor string
}

// DocumentStatus es el estado de un documento respecto de su ingesta.
type DocumentStatus string

const (
	DocumentPending    DocumentStatus = "pending"
	DocumentProcessing DocumentStatus = "processing"
	DocumentReady      DocumentStatus = "ready"
	DocumentFailed     DocumentStatus = "failed"
)

// Document es un documento del corpus.
type Document struct {
	ID         uuid.UUID
	Source     string // nombre legible, por ejemplo "RCA 042/2019"
	Collection string
	Status     DocumentStatus
	ChunkCount int
	Metadata   map[string]any

	// ContentSHA256 identifica el contenido. Dos documentos con el mismo
	// hash en la misma colección son el mismo documento.
	ContentSHA256 string

	CreatedAt time.Time
}

// JobKind es el tipo de trabajo asíncrono.
type JobKind string

const JobIngest JobKind = "ingest"

// JobStatus es el estado de un trabajo asíncrono.
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

// Job es un trabajo asíncrono sobre un documento.
type Job struct {
	ID         uuid.UUID
	Kind       JobKind
	Status     JobStatus
	Progress   int // de 0 a 100
	DocumentID uuid.UUID
	Error      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

//go:embed migrations/*.sql
var migrations embed.FS

// Migrations devuelve las migraciones del esquema que pertenece a rag.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		// Solo falla si el directorio embebido no existe, lo que es un
		// error de compilación del paquete, no una condición de ejecución.
		panic(err)
	}
	return sub
}
