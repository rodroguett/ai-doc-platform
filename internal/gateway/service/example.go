package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Las implementaciones de este archivo devuelven datos fijos, coherentes con
// el dominio, mientras no existan la búsqueda ni la ingesta reales. Sustituyen
// a las implementaciones reales sin que los handlers cambien.
//
// El proyecto "Loma Seca" y sus documentos son ficticios.

// exampleCollection es la única colección del corpus de ejemplo.
const exampleCollection = "rca"

// exampleProvider identifica en la traza que la respuesta no la generó un
// modelo. En un dominio fiscalizado una respuesta fabricada no debe poder
// confundirse con una real.
const (
	exampleProvider = "example"
	exampleModel    = "static"
)

type exampleChunk struct {
	citation Citation
	// sentence es la parte de la respuesta que este fragmento sustenta.
	sentence string
}

var (
	docRCA      = uuid.MustParse("5b3f6c1e-2a4d-4e8b-9f10-6c2d8e4a7b01")
	docAdenda   = uuid.MustParse("5b3f6c1e-2a4d-4e8b-9f10-6c2d8e4a7b02")
	docInformeT = uuid.MustParse("5b3f6c1e-2a4d-4e8b-9f10-6c2d8e4a7b03")
)

// exampleCorpus está ordenado por relevancia descendente.
var exampleCorpus = []exampleChunk{
	{
		citation: Citation{
			ChunkID:    uuid.MustParse("9e1a7d42-0c6b-4f3e-8a21-3d5f9b7c1e01"),
			DocumentID: docRCA,
			Source:     "RCA 042/2019",
			Locator:    "considerando 7.3",
			Score:      0.89,
			Excerpt: "El Titular deberá monitorear trimestralmente el nivel freático y " +
				"la calidad química del agua subterránea en los pozos PM-01 a PM-06, " +
				"informando los resultados a la Superintendencia del Medio Ambiente.",
		},
		sentence: "El titular debe monitorear cada trimestre el nivel freático y la " +
			"calidad química del agua subterránea en los pozos PM-01 a PM-06",
	},
	{
		citation: Citation{
			ChunkID:    uuid.MustParse("9e1a7d42-0c6b-4f3e-8a21-3d5f9b7c1e02"),
			DocumentID: docAdenda,
			Source:     "Adenda 2, EIA Proyecto Loma Seca",
			Locator:    "respuesta 4.12",
			Score:      0.83,
			Excerpt: "Durante los dos primeros años de operación, la frecuencia de " +
				"monitoreo de los pozos PM-03 y PM-04, ubicados aguas abajo del " +
				"depósito de relaves, será mensual.",
		},
		sentence: "Los pozos PM-03 y PM-04, aguas abajo del depósito de relaves, se " +
			"monitorean mensualmente durante los dos primeros años de operación",
	},
	{
		citation: Citation{
			ChunkID:    uuid.MustParse("9e1a7d42-0c6b-4f3e-8a21-3d5f9b7c1e03"),
			DocumentID: docInformeT,
			Source:     "Informe de seguimiento ambiental 2024-T2",
			Locator:    "sección 4.1",
			Score:      0.71,
			Excerpt: "En el período se registró en el pozo PM-04 una concentración de " +
				"sulfatos de 412 mg/L, superior al umbral de alerta de 350 mg/L " +
				"establecido en el Plan de Alerta Temprana.",
		},
		sentence: "El informe del segundo trimestre de 2024 reporta en PM-04 sulfatos " +
			"por sobre el umbral de alerta del Plan de Alerta Temprana",
	},
}

// ExampleQueries responde cualquier consulta con el mismo corpus de ejemplo,
// recortado a TopK. Ignora los filtros.
type ExampleQueries struct{}

func (ExampleQueries) Answer(ctx context.Context, q Query) (Answer, error) {
	start := time.Now()

	q, err := q.Normalize()
	if err != nil {
		return Answer{}, err
	}

	var chunks []exampleChunk
	if q.Collection == "" || q.Collection == exampleCollection {
		chunks = exampleCorpus[:min(q.TopK, len(exampleCorpus))]
	}

	ans := Answer{
		ID:        uuid.New(),
		Text:      composeAnswer(q, chunks),
		Citations: make([]Citation, 0, len(chunks)),
		Trace: Trace{
			ID:       strings.ReplaceAll(uuid.NewString(), "-", ""),
			Provider: exampleProvider,
			Model:    exampleModel,
			Cache:    CacheBypass,
		},
	}
	for i, c := range chunks {
		ans.Citations = append(ans.Citations, c.citation)
		if q.Trace == TraceFull {
			ans.Trace.Retrieved = append(ans.Trace.Retrieved, RetrievedChunk{
				ChunkID: c.citation.ChunkID,
				Score:   c.citation.Score,
				Rank:    i + 1,
			})
		}
	}
	ans.Trace.Latency.Total = time.Since(start)

	return ans, nil
}

// composeAnswer arma la respuesta con una oración por fragmento citado, cada
// una marcada con el número de su cita. Sin fragmentos no hay respuesta: en
// este dominio una afirmación sin fuente no sirve.
func composeAnswer(q Query, chunks []exampleChunk) string {
	if len(chunks) == 0 {
		return fmt.Sprintf("No hay fragmentos en la colección %q que sustenten una respuesta.", q.Collection)
	}

	var b strings.Builder
	for i, c := range chunks {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%s [%d].", c.sentence, i+1)
	}
	return b.String()
}

// ExampleDocuments acepta cualquier documento válido y devuelve un trabajo de
// ingesta en cola que nunca avanza.
type ExampleDocuments struct{}

func (ExampleDocuments) Submit(ctx context.Context, u Upload) (Job, error) {
	if err := u.Validate(); err != nil {
		return Job{}, err
	}

	n, err := io.Copy(io.Discard, u.Content)
	if err != nil {
		return Job{}, fmt.Errorf("leer el documento: %w", err)
	}
	if n == 0 {
		return Job{}, fmt.Errorf("%w: el archivo está vacío", ErrInvalidInput)
	}

	return Job{
		ID:        uuid.New(),
		Kind:      JobIngest,
		Status:    JobQueued,
		CreatedAt: time.Now().UTC(),
	}, nil
}
