package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

// fakeGenerator responde con el texto configurado y anota lo que recibe.
type fakeGenerator struct {
	mu       sync.Mutex
	text     string
	err      error
	cost     *float64
	requests []llmgw.GenerateRequest
}

func (g *fakeGenerator) Generate(_ context.Context, req llmgw.GenerateRequest) (llmgw.Generation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, req)
	if g.err != nil {
		return llmgw.Generation{}, g.err
	}
	return llmgw.Generation{Text: g.text, Provider: "gemini", Model: "modelo-de-prueba", PromptTokens: 3000, OutputTokens: 150, EstimatedCostUSD: g.cost}, nil
}

func (g *fakeGenerator) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.requests)
}

type genFixture struct {
	fixture
	generator *fakeGenerator
	passages  []rag.Passage
}

// newGenFixture arma el servicio con cuatro pasajes pertinentes y un modelo
// que responde text.
func newGenFixture(text string) genFixture {
	passages := []rag.Passage{
		passage("Considerando 10.1", 89, 90, 0.82, "Plan de seguimiento de calidad del aire: MP-10 y MPS."),
		passage("Considerando 8.1", 86, 86, 0.74, "Compromiso de monitoreo de calidad del aire; el estanque TK-65 tiene 34 m3."),
		passage("Considerando 10.2", 90, 90, 0.70, "Plan de monitoreo de aguas subterráneas."),
		passage("Considerando 10.3", 90, 91, 0.66, "Plan de monitoreo de aguas superficiales."),
	}
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	r := &fakeRetriever{passages: passages}
	g := &fakeGenerator{text: text}
	return genFixture{
		fixture: fixture{
			service: orchestrator.NewService(orchestrator.Config{
				Retriever: r, Generator: g, Now: func() time.Time { return clock },
			}),
			retriever: r,
			clock:     &clock,
		},
		generator: g,
		passages:  passages,
	}
}

var airQuestion = orchestrator.Query{Question: "¿Qué parámetros de calidad del aire se monitorean?", TopK: 4}

func TestAnswerIsWrittenFromPassages(t *testing.T) {
	f := newGenFixture("Se monitorea MP-10 y MPS [1][2].")

	answer := ask(t, f.service, airQuestion)

	if answer.Text != "Se monitorea MP-10 y MPS [1][2]." {
		t.Errorf("got respuesta %q", answer.Text)
	}
	tr := answer.Trace
	if tr.Degraded || tr.Provider != "gemini" || tr.Model != "modelo-de-prueba" {
		t.Errorf("got degradado %v, proveedor %q, modelo %q", tr.Degraded, tr.Provider, tr.Model)
	}
	if tr.Tokens.Prompt != 3000 || tr.Tokens.Completion != 150 {
		t.Errorf("got tokens %+v, want los que informó el modelo", tr.Tokens)
	}

	// El modelo recibe las reglas y, aparte, los pasajes numerados con su
	// fuente y su ubicación, seguidos de la pregunta.
	req := f.generator.requests[0]
	if !strings.Contains(req.System, "Responde solo con lo que dicen los pasajes") {
		t.Errorf("las instrucciones no exigen ceñirse a los pasajes: %q", req.System)
	}
	for _, want := range []string{
		"[1] RCA Tres Valles — Considerando 10.1, págs. 89-90\nPlan de seguimiento de calidad del aire: MP-10 y MPS.",
		"[4] RCA Tres Valles — Considerando 10.3, págs. 90-91\n",
		"Pregunta: ¿Qué parámetros de calidad del aire se monitorean?",
	} {
		if !strings.Contains(req.Prompt, want) {
			t.Errorf("el contenido enviado al modelo no incluye %q:\n%s", want, req.Prompt)
		}
	}
	if strings.Contains(req.System, "calidad del aire se monitorean") {
		t.Error("la pregunta de quien consulta llegó a las instrucciones")
	}
}

// Las citas que se devuelven son los pasajes que la respuesta cita, y las
// marcas se renumeran para apuntar a ellas.
func TestAnswerReturnsOnlyCitedPassages(t *testing.T) {
	f := newGenFixture("El aire se monitorea según el plan [1]. Las aguas superficiales tienen el suyo [4].")

	answer := ask(t, f.service, airQuestion)

	if len(answer.Citations) != 2 {
		t.Fatalf("got %d citas, want las 2 citadas", len(answer.Citations))
	}
	if answer.Citations[0].ChunkID != f.passages[0].ChunkID || answer.Citations[1].ChunkID != f.passages[3].ChunkID {
		t.Errorf("las citas no son los pasajes 1 y 4: %+v", answer.Citations)
	}
	// El pasaje 4 pasó a ser la segunda cita.
	if answer.Text != "El aire se monitorea según el plan [1]. Las aguas superficiales tienen el suyo [2]." {
		t.Errorf("las marcas no se renumeraron: %q", answer.Text)
	}
	if answer.Citations[1].Locator != "Considerando 10.3, págs. 90-91" {
		t.Errorf("got ubicación %q", answer.Citations[1].Locator)
	}
}

func TestAnswerNormalizesGroupedCitations(t *testing.T) {
	for written, want := range map[string]string{
		"Se monitorea MP-10 [1, 3].":   "Se monitorea MP-10 [1][2].",
		"Se monitorea MP-10 [1 y 3].":  "Se monitorea MP-10 [1][2].",
		"Se monitorea MP-10 [3][1][3]": "Se monitorea MP-10 [2][1][2]",
	} {
		f := newGenFixture(written)
		answer := ask(t, f.service, airQuestion)
		if answer.Text != want || len(answer.Citations) != 2 {
			t.Errorf("%q: got %q con %d citas, want %q con 2", written, answer.Text, len(answer.Citations), want)
		}
	}
}

// Una respuesta que cita un pasaje que no existe, o que no cita ninguno, no
// se entrega: no hay forma de verificarla. Se muestran los pasajes.
func TestAnswerRejectsUnsupportedText(t *testing.T) {
	const notAnswered, notWritten = "no responden esta pregunta", "No hay una respuesta redactada"
	for name, tt := range map[string]struct{ written, want string }{
		"cita un pasaje inexistente":      {"Se monitorea MP-10 [1] y también ruido [7].", notWritten},
		"cita el pasaje cero":             {"Se monitorea MP-10 [0].", notWritten},
		"no cita ninguno":                 {"El proyecto monitorea MP-10 con un límite de 150 ug/m3N.", notAnswered},
		"dice que no hay información":     {"Los pasajes entregados no contienen información sobre eso.", notAnswered},
		"solo corchetes que no son citas": {"Se monitorea MP-10 [según el plan].", notAnswered},
	} {
		t.Run(name, func(t *testing.T) {
			f := newGenFixture(tt.written)

			answer := ask(t, f.service, airQuestion)

			if answer.Text == tt.written || strings.Contains(answer.Text, "150 ug/m3N") {
				t.Fatalf("se entregó el texto sin sustento: %q", answer.Text)
			}
			if !answer.Trace.Degraded {
				t.Error("la traza debe declarar el modo degradado")
			}
			// Quien consulta debe poder distinguir que el modelo no
			// respondió de que respondió sin citar.
			if !strings.Contains(answer.Text, tt.want) {
				t.Errorf("got %q, want un texto que diga %q", answer.Text, tt.want)
			}
			if !strings.Contains(answer.Text, "4") || len(answer.Citations) != 4 {
				t.Errorf("got %q con %d citas, want los 4 pasajes", answer.Text, len(answer.Citations))
			}
			// El texto no se entregó, así que no tiene autor; pero los tokens
			// se gastaron y la traza los declara (ADR-0013).
			if answer.Trace.Provider != "none" || answer.Trace.Model != "none" {
				t.Errorf("la traza atribuye la respuesta al modelo: %+v", answer.Trace)
			}
			if answer.Trace.Tokens.Prompt != 3000 || answer.Trace.Tokens.Completion != 150 {
				t.Errorf("la traza no declara los tokens gastados: %+v", answer.Trace.Tokens)
			}
		})
	}
}

// El modelo recibe la introducción del ítem al que pertenece cada pasaje:
// sin ella, una subsección no dice de qué lista forma parte.
func TestAnswerGivesPassageContextToModel(t *testing.T) {
	f := newGenFixture("Se monitorean las aguas [1].")
	f.retriever.passages[0].Context = "8. Que, el proyecto contempla realizar los siguientes compromisos ambientales voluntarios:"

	ask(t, f.service, airQuestion)

	prompt := f.generator.requests[0].Prompt
	want := "[1] RCA Tres Valles — Considerando 10.1, págs. 89-90\n(Forma parte de: 8. Que, el proyecto contempla realizar los siguientes compromisos ambientales voluntarios:)\nPlan de seguimiento"
	if !strings.Contains(prompt, want) {
		t.Errorf("el contexto no llegó al modelo junto a su pasaje:\n%s", prompt)
	}
	// Un pasaje sin contexto no lleva la línea.
	if strings.Count(prompt, "(Forma parte de:") != 1 {
		t.Errorf("got %d líneas de contexto, want 1", strings.Count(prompt, "(Forma parte de:"))
	}
}

// Las reglas piden separar lo que el documento no responde de lo que sí, y
// no repetir como cierta una premisa que los pasajes no confirman.
func TestSystemPromptAsksForPartialAnswers(t *testing.T) {
	f := newGenFixture("Se monitorea MP-10 [1].")
	ask(t, f.service, airQuestion)

	system := f.generator.requests[0].System
	for _, rule := range []string{
		"no lo repitas como cierto",
		"di qué parte no responden y resume, con sus marcas, lo que sí dicen",
		"incluye todos los elementos",
		"forma parte de\" otra lista no es un elemento",
		"no contienen algo no lleva marcas",
	} {
		if !strings.Contains(system, rule) {
			t.Errorf("las instrucciones no incluyen %q", rule)
		}
	}
}

// Que el modelo falle no es un error de la consulta: se responde con los
// pasajes. Y la falla no queda guardada: la siguiente consulta lo reintenta.
func TestAnswerDegradesWhenGeneratorFails(t *testing.T) {
	f := newGenFixture("Se monitorea MP-10 [1].")
	f.generator.err = fmt.Errorf("%w: ningún modelo de generación respondió", llmgw.ErrUnavailable)

	degraded := ask(t, f.service, airQuestion)

	if !degraded.Trace.Degraded || len(degraded.Citations) != 4 || !strings.Contains(degraded.Text, "No hay una respuesta redactada") {
		t.Fatalf("got degradado %v, %d citas, %q", degraded.Trace.Degraded, len(degraded.Citations), degraded.Text)
	}

	f.generator.err = nil
	recovered := ask(t, f.service, airQuestion)

	if recovered.Trace.Degraded || recovered.Text != "Se monitorea MP-10 [1]." {
		t.Errorf("tras recuperarse el modelo: degradado %v, %q", recovered.Trace.Degraded, recovered.Text)
	}
	// Los pasajes sí estaban en la caché: no se buscó de nuevo.
	if f.retriever.count() != 1 || recovered.Trace.Cache != orchestrator.CacheHit {
		t.Errorf("got %d búsquedas y caché %q, want 1 y hit", f.retriever.count(), recovered.Trace.Cache)
	}
	if f.generator.count() != 2 {
		t.Errorf("got %d peticiones al modelo, want 2", f.generator.count())
	}
}

// Una pregunta repetida no vuelve a pedir la redacción, y su traza no
// declara tokens que esta consulta no consumió.
func TestAnswerCachesWrittenAnswer(t *testing.T) {
	f := newGenFixture("El aire se monitorea [1]. Las aguas superficiales también [4].")

	first := ask(t, f.service, airQuestion)
	second := ask(t, f.service, airQuestion)

	if f.generator.count() != 1 {
		t.Fatalf("got %d peticiones al modelo, want 1", f.generator.count())
	}
	if second.Text != first.Text || len(second.Citations) != 2 || second.Citations[1].ChunkID != first.Citations[1].ChunkID {
		t.Errorf("la respuesta desde la caché difiere: %q con %d citas", second.Text, len(second.Citations))
	}
	tr := second.Trace
	if tr.Cache != orchestrator.CacheHit || tr.Degraded || tr.Model != "modelo-de-prueba" {
		t.Errorf("got caché %q, degradado %v, modelo %q", tr.Cache, tr.Degraded, tr.Model)
	}
	if tr.Tokens.Prompt != 0 || tr.Tokens.Completion != 0 || tr.Latency.Generation != 0 {
		t.Errorf("la traza declara consumo en una respuesta desde la caché: %+v", tr)
	}
}

// Sin pasajes que sustenten una respuesta no se le pide nada al modelo, y
// "no hay sustento" es una respuesta definitiva, no una degradación.
func TestAnswerDoesNotGenerateWithoutSources(t *testing.T) {
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	g := &fakeGenerator{text: "El pisco sour lleva pisco y limón [1]."}
	r := &fakeRetriever{passages: []rag.Passage{passage("", 3, 3, 0.54, "sin relación")}}
	s := orchestrator.NewService(orchestrator.Config{Retriever: r, Generator: g, Now: func() time.Time { return clock }})

	answer := ask(t, s, orchestrator.Query{Question: "¿Cuál es la receta del pisco sour?"})

	if g.count() != 0 {
		t.Errorf("se le pidió al modelo redactar sin pasajes: %d peticiones", g.count())
	}
	if len(answer.Citations) != 0 || !strings.Contains(answer.Text, "No se encontraron pasajes") {
		t.Errorf("got %q con %d citas", answer.Text, len(answer.Citations))
	}
	if answer.Trace.Degraded {
		t.Error("una pregunta sin sustento no es una respuesta degradada")
	}
}

func TestAnswerStopsWhenContextIsCancelled(t *testing.T) {
	f := newGenFixture("Se monitorea MP-10 [1].")
	ctx, cancel := context.WithCancel(context.Background())
	f.generator.err = context.Canceled
	cancel()

	if _, err := f.service.Answer(ctx, airQuestion); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

// La traza completa lista todo lo recuperado, también lo que el modelo no
// citó.
func TestAnswerFullTraceKeepsUncitedPassages(t *testing.T) {
	f := newGenFixture("Se monitorea la calidad del aire [2].")
	q := airQuestion
	q.Trace = orchestrator.TraceFull

	answer := ask(t, f.service, q)

	if len(answer.Citations) != 1 || answer.Text != "Se monitorea la calidad del aire [1]." {
		t.Fatalf("got %d citas y %q", len(answer.Citations), answer.Text)
	}
	if len(answer.Trace.Retrieved) != 4 || answer.Trace.Retrieved[0].ChunkID != f.passages[0].ChunkID {
		t.Errorf("got %d recuperados, want los 4", len(answer.Trace.Retrieved))
	}
}

// Una frase que solo dice qué no contienen los pasajes no puede llevar
// citas: si las trae, se le quitan. La evaluación mostró que el modelo las
// pone aunque sus instrucciones se lo prohíban.
func TestAnswerIgnoresCitationsOnAbsenceClaims(t *testing.T) {
	tests := map[string]struct {
		written   string
		want      string // vacío: la respuesta no se entrega
		citations int
	}{
		"solo dice que no hay información": {
			written: "Los pasajes entregados no contienen información sobre el sueldo promedio [1][2][3][4].",
		},
		"no es posible responder": {
			written: "No es posible responder con los pasajes [1][2].",
		},
		"ausencia y luego lo que sí dicen": {
			written:   "Los pasajes no explican por qué se usa esa fórmula [1][2]. Los planes indican límites máximos permitidos por la autoridad correspondiente [3][4].",
			want:      "Los pasajes no explican por qué se usa esa fórmula. Los planes indican límites máximos permitidos por la autoridad correspondiente [1][2].",
			citations: 2,
		},
		"una misma frase que corrige y afirma": {
			written:   "Los pasajes no mencionan una capacidad de 4 m3, sino que indican 34 m3 [2][3].",
			want:      "Los pasajes no mencionan una capacidad de 4 m3, sino que indican 34 m3 [1][2].",
			citations: 2,
		},
		"ausencia y afirmación separadas por punto y coma": {
			written:   "Los pasajes no mencionan 4 m3, por lo que no es posible confirmar esa cifra; indican que la capacidad es de 34 m3 [1][2].",
			want:      "Los pasajes no mencionan 4 m3, por lo que no es posible confirmar esa cifra; indican que la capacidad es de 34 m3 [1][2].",
			citations: 2,
		},
		"varios verbos, todos negados": {
			written: "Los pasajes no indican ni señalan la causa; no establecen un plazo [1].",
		},
		"una negación que el documento sí hace": {
			written:   "El proyecto no contempla permisos de contenido únicamente ambiental [3].",
			want:      "El proyecto no contempla permisos de contenido únicamente ambiental [1].",
			citations: 1,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newGenFixture(tt.written)

			answer := ask(t, f.service, airQuestion)

			if tt.want == "" {
				if !answer.Trace.Degraded || !strings.Contains(answer.Text, "no responden esta pregunta") {
					t.Fatalf("got %q (degradado %v), want el texto de que los pasajes no responden", answer.Text, answer.Trace.Degraded)
				}
				return
			}
			if answer.Trace.Degraded || answer.Text != tt.want || len(answer.Citations) != tt.citations {
				t.Errorf("got %q con %d citas (degradado %v),\nwant %q con %d", answer.Text, len(answer.Citations), answer.Trace.Degraded, tt.want, tt.citations)
			}
		})
	}
}

// Una cifra que no está en lo que cita su oración no descarta la respuesta:
// se entrega con un aviso, y la traza la declara (ADR-0014).
func TestAnswerWarnsAboutUnverifiedFigures(t *testing.T) {
	f := newGenFixture("El límite del MP-10 es de 150 ug/m3N [1].")

	answer := ask(t, f.service, airQuestion)

	if answer.Trace.Degraded || !strings.HasPrefix(answer.Text, "El límite del MP-10 es de 150 ug/m3N [1].") {
		t.Fatalf("got degradado %v y %q", answer.Trace.Degraded, answer.Text)
	}
	if !strings.Contains(answer.Text, "Aviso: la cifra «150»") {
		t.Errorf("la respuesta no avisa de la cifra: %q", answer.Text)
	}
	if !slices.Equal(answer.Trace.UnverifiedFigures, []string{"150"}) {
		t.Errorf("got cifras sin respaldo %q, want [150]", answer.Trace.UnverifiedFigures)
	}

	// La misma pregunta desde la caché conserva el aviso.
	cached := ask(t, f.service, airQuestion)
	if cached.Trace.Cache != orchestrator.CacheHit || !slices.Equal(cached.Trace.UnverifiedFigures, []string{"150"}) || cached.Text != answer.Text {
		t.Errorf("desde la caché: %+v, %q", cached.Trace, cached.Text)
	}
}

// Una respuesta cuyas cifras están en lo citado no lleva aviso.
func TestAnswerWithSupportedFiguresHasNoNotice(t *testing.T) {
	f := newGenFixture("Se monitorea el MP-10 [1].")

	answer := ask(t, f.service, airQuestion)

	if answer.Text != "Se monitorea el MP-10 [1]." || answer.Trace.UnverifiedFigures != nil {
		t.Errorf("got %q con cifras sin respaldo %q", answer.Text, answer.Trace.UnverifiedFigures)
	}
}
