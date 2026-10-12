package rag

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
)

// Límites del listado. Coinciden con los del contrato HTTP, pero se validan
// aquí porque son reglas del dominio.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

const (
	// defaultPassBudget es cuánto trabaja una pasada antes de ceder. Queda
	// holgado bajo el plazo de 120 s de una petición: al cumplirse puede
	// haber un grupo en curso, que con el ritmo regulado tarda hasta medio
	// minuto.
	defaultPassBudget = 60 * time.Second

	// leaseMargin es cuánto más allá de la pasada dura la reserva del
	// trabajo, para cubrir ese último grupo.
	leaseMargin = 90 * time.Second

	// runBatchSize es cuántos fragmentos se convierten y guardan por vez.
	// Chico, para que el avance se guarde seguido y una falla pierda poco.
	runBatchSize = 8

	cleanupTimeout = 10 * time.Second
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

	// ClaimJob reserva un trabajo pendiente o en curso por la duración
	// indicada. Devuelve false, con el estado actual, si ya terminó o si
	// otra pasada lo tiene reservado.
	ClaimJob(ctx context.Context, id uuid.UUID, lease time.Duration) (Job, bool, error)
	ReleaseJob(ctx context.Context, id uuid.UUID) error

	// PendingChunks devuelve hasta limit fragmentos sin embedding, en orden.
	PendingChunks(ctx context.Context, documentID uuid.UUID, limit int) ([]Chunk, error)

	// SaveEmbeddings guarda los vectores y actualiza el avance del trabajo.
	SaveEmbeddings(ctx context.Context, jobID uuid.UUID, model string, embeddings []ChunkEmbedding) (Job, error)

	// FinishJob cierra el trabajo y deja a su documento listo o fallido.
	FinishJob(ctx context.Context, id uuid.UUID, status JobStatus, message string) (Job, error)
}

// TextExtractor obtiene el texto de un documento, página por página. Un
// archivo ilegible devuelve un error que envuelve ErrInvalidInput; uno sin
// texto, ErrNoTextLayer.
type TextExtractor interface {
	Extract(ctx context.Context, data []byte) ([]PageText, error)
}

// Splitter divide el texto de un documento en fragmentos con su ubicación.
type Splitter func(pages []PageText) []Chunk

// Embedder convierte pasajes en vectores. Es la parte de llmgw.Embedder que
// rag necesita, definida aquí porque rag es quien la consume.
type Embedder interface {
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)

	// Model identifica al modelo; se guarda junto a cada vector.
	Model() string
}

// Dispatcher hace que un trabajo se ejecute más tarde, fuera de la petición
// que lo encola.
type Dispatcher interface {
	Enqueue(ctx context.Context, jobID uuid.UUID) error
}

// Deps son las piezas de las que depende el servicio. Las implementan los
// subpaquetes de rag y otros módulos; el paquete raíz no puede importar a
// sus subpaquetes, porque ellos lo importan a él, así que las conecta el
// binario.
type Deps struct {
	Store     Store
	Extractor TextExtractor
	Split     Splitter

	// Embedder y Dispatcher pueden faltar: los documentos se registran con
	// sus fragmentos, pero sus trabajos quedan en cola.
	Embedder   Embedder
	Dispatcher Dispatcher

	// PassBudget y Now permiten a los tests controlar cuánto dura una
	// pasada. Vacíos usan los valores reales.
	PassBudget time.Duration
	Now        func() time.Time
}

// Service registra y consulta documentos y sus trabajos de ingesta.
type Service struct {
	store      Store
	extractor  TextExtractor
	split      Splitter
	embedder   Embedder
	dispatcher Dispatcher

	passBudget time.Duration
	now        func() time.Time
}

func NewService(deps Deps) *Service {
	s := &Service{
		store:      deps.Store,
		extractor:  deps.Extractor,
		split:      deps.Split,
		embedder:   deps.Embedder,
		dispatcher: deps.Dispatcher,
		passBudget: deps.PassBudget,
		now:        deps.Now,
	}
	if s.passBudget <= 0 {
		s.passBudget = defaultPassBudget
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
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
	if err != nil {
		return Job{}, err
	}

	if s.dispatcher == nil {
		return job, nil
	}
	if err := s.dispatcher.Enqueue(ctx, job.ID); err != nil {
		// Un documento registrado cuyo trabajo nadie va a ejecutar quedaría
		// en cola para siempre, y volver a subirlo daría "duplicado". Se
		// deshace el registro para que la subida pueda reintentarse.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if delErr := s.store.DeleteDocument(cleanup, docID); delErr != nil {
			return Job{}, fmt.Errorf("encolar el trabajo: %w (y no se pudo deshacer el registro: %w)", err, delErr)
		}
		return Job{}, fmt.Errorf("%w: no se pudo encolar el trabajo de ingesta", ErrUnavailable)
	}
	return job, nil
}

// RunJob ejecuta una pasada del trabajo: genera los embeddings de los
// fragmentos pendientes durante un tiempo acotado. Si quedan fragmentos al
// terminar la pasada, encola la siguiente.
//
// Trabaja en pasadas porque quien lo invoca es una petición, y una petición
// tiene plazo. Cada grupo de fragmentos se guarda apenas se obtiene, así que
// una pasada interrumpida no pierde lo ya hecho: la siguiente continúa desde
// los pendientes.
//
// Devuelve el estado del trabajo tras la pasada. Un trabajo terminado, o que
// otra pasada tiene reservado, se devuelve sin tocarlo: invocarlo de más es
// inofensivo. Devuelve ErrUnavailable si el proveedor de embeddings no
// responde; el trabajo queda en curso y puede reintentarse.
func (s *Service) RunJob(ctx context.Context, id uuid.UUID) (Job, error) {
	if s.embedder == nil {
		return Job{}, fmt.Errorf("%w: los embeddings no están configurados", ErrUnavailable)
	}

	job, claimed, err := s.store.ClaimJob(ctx, id, s.passBudget+leaseMargin)
	if err != nil {
		return Job{}, err
	}
	if !claimed {
		return job, nil
	}
	// La reserva se libera aunque la petición se haya cancelado.
	release := func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		_ = s.store.ReleaseJob(cleanup, id)
	}

	deadline := s.now().Add(s.passBudget)
	for {
		chunks, err := s.store.PendingChunks(ctx, job.DocumentID, runBatchSize)
		if err != nil {
			release()
			return Job{}, err
		}
		if len(chunks) == 0 {
			return s.store.FinishJob(ctx, id, JobSucceeded, "")
		}

		// El plazo se revisa entre grupos y no se impone al grupo en curso:
		// cancelar una petición al proveedor a medio camino gastaría cuota
		// sin obtener nada.
		if !s.now().Before(deadline) {
			release()
			if s.dispatcher != nil {
				if err := s.dispatcher.Enqueue(ctx, id); err != nil {
					return job, fmt.Errorf("%w: no se pudo encolar la continuación del trabajo", ErrUnavailable)
				}
			}
			return job, nil
		}

		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = embeddingInput(c)
		}
		vectors, err := s.embedder.EmbedDocuments(ctx, texts)
		switch {
		case err == nil:
		case ctx.Err() != nil:
			release()
			return Job{}, ctx.Err()
		case errors.Is(err, llmgw.ErrUnavailable):
			release()
			return job, fmt.Errorf("%w: el proveedor de embeddings no responde", ErrUnavailable)
		default:
			// Reintentar no lo arregla: el trabajo falla con un mensaje
			// para quien lo consulta. El detalle técnico va al error.
			failed, finishErr := s.store.FinishJob(ctx, id, JobFailed, failureMessage(err))
			if finishErr != nil {
				release()
				return Job{}, finishErr
			}
			return failed, nil
		}

		embeddings := make([]ChunkEmbedding, len(chunks))
		for i, c := range chunks {
			embeddings[i] = ChunkEmbedding{ChunkID: c.ID, Vector: vectors[i]}
		}
		if job, err = s.store.SaveEmbeddings(ctx, id, s.embedder.Model(), embeddings); err != nil {
			release()
			return Job{}, err
		}
	}
}

// embeddingInput es el texto de un fragmento que se convierte en vector.
// Cuando una sección se dividió en varios fragmentos, solo el primero lleva
// su título en el texto: a los demás se les antepone, para que el vector
// sepa de qué trata el pasaje.
func embeddingInput(c Chunk) string {
	if c.Heading == "" || strings.Contains(firstLine(c.Text), c.Heading) {
		return c.Text
	}
	return c.Heading + "\n" + c.Text
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// failureMessage es lo que ve quien consulta un trabajo fallido. No incluye
// el error original, que puede traer detalles internos.
func failureMessage(err error) string {
	switch {
	case errors.Is(err, llmgw.ErrInvalidInput):
		return "El proveedor de embeddings rechazó el contenido de un fragmento."
	case errors.Is(err, llmgw.ErrMisconfigured):
		return "El proveedor de embeddings está mal configurado."
	default:
		return "No se pudieron generar los embeddings del documento."
	}
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
