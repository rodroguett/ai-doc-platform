// Comando eval mide la calidad de la búsqueda, y opcionalmente de las
// respuestas, contra un conjunto de preguntas cuya respuesta se conoce.
//
// No corre en CI ni en los tests: consume cuota del proveedor de embeddings
// y solo se ejecuta cuando alguien lo pide.
//
//	GEMINI_API_KEY=... go run ./cmd/eval -out docs/eval/resultado.md
//
// Usa un Postgres propio (por omisión, la base aidoc_eval del contenedor de
// `make db-up`). La primera vez ingiere el documento del conjunto, lo que
// cuesta unos 112.000 tokens de embeddings y unos cuatro minutos; las
// siguientes lo reutilizan.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rodroguett/ai-doc-platform/internal/llmgw/gemini"
	"github.com/rodroguett/ai-doc-platform/internal/orchestrator"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/rag"
	"github.com/rodroguett/ai-doc-platform/internal/rag/chunk"
	"github.com/rodroguett/ai-doc-platform/internal/rag/dispatch"
	"github.com/rodroguett/ai-doc-platform/internal/rag/pdftext"
	"github.com/rodroguett/ai-doc-platform/internal/rag/store"
)

const (
	collection = "eval"

	// cutoff es cuántos pasajes entrega una consulta por omisión.
	cutoff = 5

	// depth es cuántos pasajes se piden para ver si lo esperado estaba
	// cerca del corte aunque no lo pasara.
	depth = 10
)

type evalSet struct {
	Document string     `json:"document"`
	Notes    string     `json:"notes"`
	Cases    []evalCase `json:"cases"`
}

// evalCase es una pregunta. Expect son grupos de secciones: cada grupo se
// cumple si se recupera alguna de sus secciones, y una respuesta completa
// necesita todos los grupos. Sin grupos, el documento no responde la
// pregunta.
type evalCase struct {
	ID       string     `json:"id"`
	Origin   string     `json:"origin"`
	Question string     `json:"question"`
	Expect   [][]string `json:"expect"`
	Measures string     `json:"measures"`
}

func main() {
	setPath := flag.String("set", "eval/rca-tres-valles.json", "conjunto de preguntas")
	dbURL := flag.String("db", "postgres://postgres:postgres@localhost:55432/aidoc_eval?sslmode=disable", "Postgres para la evaluación")
	generate := flag.Bool("generate", false, "redactar también las respuestas, para revisarlas a mano")
	out := flag.String("out", "", "archivo del informe; vacío escribe en la salida estándar")
	flag.Parse()

	if err := run(*setPath, *dbURL, *generate, *out); err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

func run(setPath, dbURL string, generate bool, out string) error {
	ctx := context.Background()

	raw, err := os.ReadFile(filepath.Clean(setPath))
	if err != nil {
		return err
	}
	var set evalSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return fmt.Errorf("leer el conjunto: %w", err)
	}

	key := os.Getenv("GEMINI_API_KEY")
	client, err := gemini.New(gemini.Config{APIKey: key})
	if err != nil {
		return err
	}

	pool, err := postgres.Connect(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("%w (¿existe la base? créala con: docker exec aidoc-pg createdb -U postgres aidoc_eval)", err)
	}
	defer pool.Close()
	if _, err := postgres.Migrate(ctx, pool, rag.Migrations()); err != nil {
		return err
	}

	extractor := pdftext.New()
	defer func() { _ = extractor.Close() }()
	local := dispatch.NewInProcess()
	defer local.Stop()
	st := store.New(pool)
	documents := rag.NewService(rag.Deps{
		Store: st, Extractor: extractor, Split: chunk.Split, Embedder: client, Dispatcher: local,
	})
	local.Start(documents.RunJob)

	if err := ensureCorpus(ctx, pool, st, documents, set.Document); err != nil {
		return err
	}

	results := make([]result, len(set.Cases))
	for i, c := range set.Cases {
		passages, err := documents.Search(ctx, c.Question, rag.SearchOptions{Collection: collection, TopK: depth})
		if err != nil {
			return fmt.Errorf("buscar %q: %w", c.ID, err)
		}
		results[i] = score(c, passages)
		fmt.Fprintf(os.Stderr, "  %-28s %s\n", c.ID, results[i].summary())
	}

	if generate {
		generator, err := client.Generator(gemini.ModelFlashLite, gemini.ModelFlashLiteFallback)
		if err != nil {
			return err
		}
		queries := orchestrator.NewService(orchestrator.Config{Retriever: scoped{documents}, Generator: generator})
		for i, c := range set.Cases {
			answer, err := queries.Answer(ctx, orchestrator.Query{Question: c.Question, TopK: cutoff})
			if err != nil {
				return fmt.Errorf("responder %q: %w", c.ID, err)
			}
			results[i].answer = &answer
		}
	}

	w := os.Stdout
	if out != "" {
		f, err := os.Create(filepath.Clean(out))
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		w = f
	}
	report(w, set, results, client.Model(), generate)
	return nil
}

// scoped restringe las búsquedas del orquestador a la colección de la
// evaluación.
type scoped struct{ documents *rag.Service }

func (s scoped) Search(ctx context.Context, query string, opts rag.SearchOptions) ([]rag.Passage, error) {
	opts.Collection = collection
	return s.documents.Search(ctx, query, opts)
}

// ensureCorpus ingiere el documento del conjunto si todavía no está listo.
func ensureCorpus(ctx context.Context, pool *pgxpool.Pool, st *store.Store, documents *rag.Service, path string) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM documents WHERE collection = $1 AND content_sha256 = $2`,
		collection, hex.EncodeToString(sum[:])).Scan(&status)
	switch {
	case err == nil && status == string(rag.DocumentReady):
		return nil
	case err == nil:
		return fmt.Errorf("el documento está en estado %q; elimínalo de la base de evaluación y vuelve a correr", status)
	}

	fmt.Fprintln(os.Stderr, "eval: ingiriendo el documento; consume unos 112.000 tokens de embeddings y unos cuatro minutos")
	job, err := documents.Submit(ctx, rag.Upload{Filename: path, Collection: collection, Content: data})
	if err != nil {
		return fmt.Errorf("subir el documento: %w", err)
	}
	for {
		time.Sleep(5 * time.Second)
		current, err := st.Job(ctx, job.ID)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\r  ingesta: %s %d %%   ", current.Status, current.Progress)
		switch current.Status {
		case rag.JobSucceeded:
			fmt.Fprintln(os.Stderr)
			return nil
		case rag.JobFailed:
			return errors.New("la ingesta falló: " + current.Error)
		}
	}
}

// result es lo medido para una pregunta.
type result struct {
	answerable bool
	groups     int

	// hits y deepHits cuentan los grupos cumplidos entre los primeros
	// cutoff y depth pasajes; citable, los cumplidos por pasajes que además
	// alcanzan el puntaje mínimo y por lo tanto se citarían.
	hits, deepHits, citable int

	// best es el mejor puntaje de un pasaje esperado entre los primeros
	// cutoff, o cero si no hay ninguno. top es el del primer pasaje.
	best, top float64

	retrieved []string // sección y puntaje de los primeros cutoff
	answer    *orchestrator.Answer
}

func score(c evalCase, passages []rag.Passage) result {
	r := result{answerable: len(c.Expect) > 0, groups: len(c.Expect)}
	if len(passages) > 0 {
		r.top = passages[0].Score
	}
	for _, p := range passages[:min(cutoff, len(passages))] {
		r.retrieved = append(r.retrieved, fmt.Sprintf("%s (%.3f)", label(p), p.Score))
	}

	for _, group := range c.Expect {
		hit, deep, cite := false, false, false
		for i, p := range passages {
			if !slices.Contains(group, p.Section) {
				continue
			}
			deep = true
			if i < cutoff {
				hit = true
				r.best = max(r.best, p.Score)
				if p.Score >= orchestrator.MinScore {
					cite = true
				}
			}
		}
		r.hits += b2i(hit)
		r.deepHits += b2i(deep)
		r.citable += b2i(cite)
	}
	return r
}

func label(p rag.Passage) string {
	if p.Section == "" {
		return fmt.Sprintf("pág. %d", p.PageStart)
	}
	return p.Section
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r result) summary() string {
	if !r.answerable {
		verdict := "sin citas"
		if r.top >= orchestrator.MinScore {
			verdict = "CITARÍA"
		}
		return fmt.Sprintf("no respondible: primer puntaje %.3f, %s", r.top, verdict)
	}
	return fmt.Sprintf("%d/%d grupos en los primeros %d (%d/%d en %d), %d citables", r.hits, r.groups, cutoff, r.deepHits, r.groups, depth, r.citable)
}

func report(w io.Writer, set evalSet, results []result, model string, generated bool) {
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }

	var answerable, complete, citableComplete, groups, hits, deep int
	var unanswerable, falseCites int
	for _, r := range results {
		if r.answerable {
			answerable++
			groups += r.groups
			hits += r.hits
			deep += r.deepHits
			complete += b2i(r.hits == r.groups)
			citableComplete += b2i(r.citable == r.groups)
		} else {
			unanswerable++
			falseCites += b2i(r.top >= orchestrator.MinScore)
		}
	}

	p("# Evaluación de la búsqueda: %s\n\n", set.Document)
	p("- **Fecha:** %s\n", time.Now().Format(time.DateOnly))
	p("- **Modelo de embeddings:** `%s`\n", model)
	p("- **Puntaje mínimo para citar:** %.2f\n", orchestrator.MinScore)
	p("- **Preguntas:** %d (%d respondibles, %d que el documento no responde)\n\n", len(results), answerable, unanswerable)
	p("%s\n\n", set.Notes)

	p("## Resultado\n\n")
	p("| Medida | Valor |\n|---|---|\n")
	p("| Preguntas con todo lo esperado entre los primeros %d pasajes | %d de %d |\n", cutoff, complete, answerable)
	p("| Ídem, y además con puntaje suficiente para citarse | %d de %d |\n", citableComplete, answerable)
	p("| Grupos de secciones esperadas recuperados entre los primeros %d | %d de %d |\n", cutoff, hits, groups)
	p("| Ídem entre los primeros %d | %d de %d |\n", depth, deep, groups)
	p("| Preguntas sin respuesta en el documento que recibirían citas | %d de %d |\n\n", falseCites, unanswerable)

	p("## Por pregunta\n\n")
	p("| Pregunta | Origen | Esperado | Recuperado (primeros %d) | Resultado |\n|---|---|---|---|---|\n", cutoff)
	for i, c := range set.Cases {
		r := results[i]
		expected := "—"
		if r.answerable {
			parts := make([]string, len(c.Expect))
			for j, g := range c.Expect {
				parts[j] = strings.Join(g, " o ")
			}
			expected = strings.Join(parts, "; ")
		}
		p("| `%s` | %s | %s | %s | %s |\n", c.ID, c.Origin, expected, strings.Join(r.retrieved, "<br>"), r.summary())
	}

	p("\n## Puntaje mínimo\n\n")
	p("Para cada umbral: cuántas preguntas respondibles tendrían su mejor pasaje esperado en condiciones de citarse, y cuántas sin respuesta recibirían citas.\n\n")
	p("| Umbral | Respondibles con sustento citable | Sin respuesta que recibirían citas |\n|---|---|---|\n")
	for t := 0.50; t <= 0.7501; t += 0.025 {
		var ok, bad int
		for _, r := range results {
			if r.answerable && r.best >= t {
				ok++
			}
			if !r.answerable && r.top >= t {
				bad++
			}
		}
		marker := ""
		if t > orchestrator.MinScore-0.001 && t < orchestrator.MinScore+0.001 {
			marker = " **(actual)**"
		}
		p("| %.3f%s | %d de %d | %d de %d |\n", t, marker, ok, answerable, bad, unanswerable)
	}

	if !generated {
		return
	}
	p("\n## Respuestas redactadas, para revisión manual\n\n")
	p("La verificación automática comprueba que las citas existan, no que la respuesta sea correcta. Estas respuestas se revisan a mano contra lo que cada pregunta mide.\n\n")
	for i, c := range set.Cases {
		a := results[i].answer
		p("### `%s`\n\n**Mide:** %s\n\n**Pregunta:** %s\n\n", c.ID, c.Measures, c.Question)
		mode := fmt.Sprintf("redactada por `%s`", a.Trace.Model)
		if a.Trace.Degraded {
			// Cubre tanto la falla del proveedor como la respuesta descartada
			// por no citar nada; el texto entregado dice cuál fue.
			mode = "sin texto redactado"
		} else if a.Trace.Model == "none" {
			mode = "sin sustento"
		}
		p("**Respuesta** (%s):\n\n> %s\n\n", mode, strings.ReplaceAll(a.Text, "\n", "\n> "))
		for j, cit := range a.Citations {
			p("%d. %s (%.3f)\n", j+1, cit.Locator, cit.Score)
		}
		p("\n")
	}
}

func init() {
	// Las advertencias del sistema van a la salida de error, sin mezclarse
	// con el informe.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
}
