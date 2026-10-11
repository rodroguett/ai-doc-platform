// Package postgrestest entrega una base de datos aislada a los tests que
// necesitan un Postgres real.
package postgrestest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
)

// envURL es la variable con la cadena de conexión del Postgres de pruebas.
const envURL = "TEST_DATABASE_URL"

// NewPool devuelve un pool cuyo esquema por omisión es uno nuevo y vacío,
// que se elimina al terminar el test. Así los paquetes de test pueden correr
// en paralelo contra la misma base de datos sin verse entre sí.
//
// Sin TEST_DATABASE_URL el test se omite, salvo en CI, donde falla: un test
// de persistencia que se omite en silencio no verifica nada.
func NewPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv(envURL)
	if url == "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s no está definida en CI", envURL)
		}
		t.Skipf("%s no está definida; use `make test`", envURL)
	}

	ctx := context.Background()
	admin, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatalf("conectar al Postgres de pruebas: %v", err)
	}

	schema := pgx.Identifier{"test_" + randomSuffix(t)}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("crear el esquema de pruebas: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("leer %s: %v", envURL, err)
	}
	postgres.Configure(cfg)
	// public queda en la ruta para que las extensiones sean visibles.
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"

	pool, err := postgres.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("conectar al esquema de pruebas: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generar el nombre del esquema: %v", err)
	}
	return hex.EncodeToString(b)
}
