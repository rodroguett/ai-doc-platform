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

// Dos módulos numeran sus migraciones desde 1. Con tablas de versiones
// distintas, la versión 1 de uno no impide aplicar la del otro.
func TestMigrateTableKeepsModulesApart(t *testing.T) {
	pool := postgrestest.NewPool(t)
	ctx := context.Background()

	if _, err := postgres.Migrate(ctx, pool, probeMigrations()); err != nil {
		t.Fatalf("migrar el primer módulo: %v", err)
	}
	applied, err := postgres.MigrateTable(ctx, pool, os.DirFS("testdata/other"), "other_goose_version")
	if err != nil {
		t.Fatalf("migrar el segundo módulo: %v", err)
	}
	if applied != 1 {
		t.Fatalf("el segundo módulo aplicó %d migraciones, want 1", applied)
	}
	if _, err := pool.Exec(ctx, "SELECT 1 FROM other_probe"); err != nil {
		t.Errorf("no existe la tabla del segundo módulo: %v", err)
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
