// Package config lee la configuración del proceso desde el entorno.
package config

import "os"

// Config es la configuración del binario.
type Config struct {
	// Port es el puerto HTTP. Cloud Run lo entrega en PORT.
	Port string

	// DatabaseURL es la cadena de conexión a Postgres. Vacía significa que
	// el proceso corre sin base de datos, con los servicios de ejemplo.
	DatabaseURL string
}

// Load lee la configuración del entorno y aplica los valores por omisión.
func Load() Config {
	cfg := Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg
}
