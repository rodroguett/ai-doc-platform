package postgres_test

import (
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres"
	"github.com/rodroguett/ai-doc-platform/internal/platform/postgres/postgrestest"
)

// probeMigrations es un conjunto mínimo de migraciones propio de estos tests:
// platform no conoce los esquemas de los módulos.
func probeMigrations() fs.FS {
	return os.DirFS("testdata/migrations")
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	applied, err := postgres.Migrate(ctx, pool, probeMigrations())
	if err != nil {
		t.Fatalf("primera migración: %v", err)
	}
	if applied == 0 {
		t.Fatal("la primera ejecución no aplicó ninguna migración")
	}

	applied, err = postgres.Migrate(ctx, pool, probeMigrations())
	if err != nil {
		t.Fatalf("segunda migración: %v", err)
	}
	if applied != 0 {
		t.Errorf("la segunda ejecución aplicó %d migraciones, want 0", applied)
	}
}

func TestConnectDoesNotLeakTheURL(t *testing.T) {
	const password = "s3cr3t-p4ss"
	_, err := postgres.Connect(context.Background(), "postgres://user:"+password+"@host:port/db")
	if err == nil {
		t.Fatal("se esperaba un error por la cadena inválida")
	}
	if strings.Contains(err.Error(), password) {
		t.Errorf("el error expone la contraseña: %v", err)
	}
}

// Varias instancias pueden arrancar a la vez. Solo una debe aplicar cada
// migración; las demás esperan el bloqueo y encuentran el esquema al día.
func TestMigrateConcurrently(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	const instances = 3
	applied := make(chan int, instances)
	errs := make(chan error, instances)
	for range instances {
		go func() {
			n, err := postgres.Migrate(ctx, pool, probeMigrations())
			applied <- n
			errs <- err
		}()
	}

	total := 0
	for range instances {
		total += <-applied
		if err := <-errs; err != nil {
			t.Errorf("migración concurrente: %v", err)
		}
	}
	if total != 1 {
		t.Errorf("se aplicaron %d migraciones en total, want 1", total)
	}
}
