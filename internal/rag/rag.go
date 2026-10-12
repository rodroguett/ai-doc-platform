// Package rag es el contrato del módulo de ingesta y búsqueda: los tipos que
// cruzan su frontera y los errores que otros módulos pueden distinguir.
//
// Es lo único de rag que otro módulo puede importar (ADR-0005). Los
// subpaquetes son implementación privada.
package rag

import (
	"embed"
	"errors"
	"fmt"
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

	// ErrNoTextLayer indica que el documento no trae texto extraíble: es un
	// escaneo, páginas que son imágenes. Envuelve ErrInvalidInput.
	ErrNoTextLayer = fmt.Errorf("%w: el documento no tiene capa de texto; los documentos escaneados no se admiten", ErrInvalidInput)
)

// PageText es el texto de una página de un documento, tal como se extrajo.
type PageText struct {
	Number int // desde 1
	Text   string
}

// Chunk es un fragmento de un documento junto con su ubicación, que es lo que
// permite que una cita señale el lugar exacto del original.
type Chunk struct {
	ID uuid.UUID

	// Index es la posición del fragmento dentro del documento, desde 0.
	Index int

	// Section ubica el fragmento en la estructura del documento, por
	// ejemplo "Considerando 10.1". Vacía si el documento no tiene una
	// estructura reconocible.
	Section string

	// Heading es el título de la sección, cuando lo tiene.
	Heading string

	// PageStart y PageEnd son la primera y la última página que el
	// fragmento abarca, desde 1.
	PageStart int
	PageEnd   int

	Text string
}

// Upload es un documento enviado para ingesta.
type Upload struct {
	Filename   string
	Collection string
	Metadata   map[string]any
	Content    []byte
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
