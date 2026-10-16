// Package postgres abre la conexión a Postgres y aplica migraciones.
package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Límites del pool. El servicio escala a pocas instancias pequeñas y la base
// de datos suspende su cómputo cuando no hay actividad: se prefieren pocas
// conexiones que se cierran pronto a un pool amplio que las retiene.
const (
	maxConns        = 4
	maxConnIdleTime = 2 * time.Minute
	maxConnLifetime = 30 * time.Minute
	connectTimeout  = 15 * time.Second
)

// Connect abre un pool y verifica que la base de datos responda. El plazo de
// conexión contempla que la base de datos puede estar suspendida y tardar
// unos segundos en despertar.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// El error de pgx puede citar la cadena de conexión, que contiene
		// la contraseña. No se propaga.
		return nil, fmt.Errorf("la cadena de conexión a Postgres no es válida")
	}
	Configure(cfg)

	return ConnectConfig(ctx, cfg)
}

// Configure aplica los límites del pool a una configuración ya construida.
func Configure(cfg *pgxpool.Config) {
	cfg.MaxConns = maxConns
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.ConnConfig.ConnectTimeout = connectTimeout
}

// ConnectConfig abre un pool a partir de una configuración y verifica que la
// base de datos responda.
func ConnectConfig(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("crear el pool de Postgres: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar a Postgres: %w", err)
	}
	return pool, nil
}

// DefaultVersionTable es la tabla en que goose anota las migraciones
// aplicadas cuando no se indica otra. Es la de rag, el primer módulo con
// esquema propio.
const DefaultVersionTable = "goose_db_version"

// Migrate aplica las migraciones pendientes de migrations, anotándolas en la
// tabla de versiones por omisión, y devuelve cuántas aplicó.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) (int, error) {
	return MigrateTable(ctx, pool, migrations, DefaultVersionTable)
}

// MigrateTable aplica las migraciones pendientes de migrations y devuelve
// cuántas aplicó. Cada módulo con esquema propio anota sus versiones en una
// tabla distinta: con una sola, la versión 1 de un módulo se confundiría con
// la del otro.
//
// Toma un bloqueo de sesión en la base de datos, de modo que dos instancias
// que arrancan a la vez no migran simultáneamente: la segunda espera y
// encuentra el esquema al día.
func MigrateTable(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS, table string) (int, error) {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	// Quien no obtiene el bloqueo reintenta cada segundo, hasta un minuto.
	// El valor por omisión de goose reintenta cada cinco, lo que sumaría esa
	// espera al arranque de toda instancia que coincida con otra.
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(1, 60))
	if err != nil {
		return 0, fmt.Errorf("preparar el bloqueo de migraciones: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations,
		goose.WithSessionLocker(locker), goose.WithTableName(table))
	if err != nil {
		return 0, fmt.Errorf("leer las migraciones: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return len(results), fmt.Errorf("aplicar las migraciones: %w", err)
	}
	return len(results), nil
}
