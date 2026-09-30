package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Upload es un documento enviado para ingesta.
type Upload struct {
	Filename   string
	Collection string
	Metadata   map[string]any
	Content    io.Reader
}

// Validate comprueba que el documento tenga lo mínimo para ingerirse.
func (u Upload) Validate() error {
	if strings.TrimSpace(u.Filename) == "" {
		return fmt.Errorf("%w: el archivo no tiene nombre", ErrInvalidInput)
	}
	if strings.TrimSpace(u.Collection) == "" {
		return fmt.Errorf("%w: falta la colección", ErrInvalidInput)
	}
	if u.Content == nil {
		return fmt.Errorf("%w: falta el contenido del archivo", ErrInvalidInput)
	}
	return nil
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

// Job es un trabajo asíncrono, como la ingesta de un documento.
type Job struct {
	ID         uuid.UUID
	Kind       JobKind
	Status     JobStatus
	Progress   int
	ResourceID uuid.UUID // uuid.Nil mientras no haya documento resultante
	Error      string
	CreatedAt  time.Time
}
