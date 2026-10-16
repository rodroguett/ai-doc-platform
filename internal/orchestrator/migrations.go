package orchestrator

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrations embed.FS

// MigrationsTable es la tabla en que se anotan las migraciones de este
// módulo, distinta de la de rag: cada módulo numera las suyas desde 1.
const MigrationsTable = "orchestrator_goose_version"

// Migrations devuelve las migraciones del esquema del módulo, para aplicarlas
// con postgres.MigrateTable y MigrationsTable.
func Migrations() fs.FS {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		// Solo falla si el directorio embebido no existe, lo que es un
		// error de compilación del paquete, no una condición de ejecución.
		panic(err)
	}
	return sub
}
