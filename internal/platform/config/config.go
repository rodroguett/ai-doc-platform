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

	// WriteToken autoriza las operaciones que modifican el corpus. Vacío
	// significa que esas operaciones rechazan toda petición.
	WriteToken string

	// GeminiAPIKey es la clave de la API de Gemini, con la que se generan
	// los embeddings. Vacía significa que no se pueden generar.
	GeminiAPIKey string

	// TasksQueue es el nombre completo de la cola de Cloud Tasks que
	// despacha los trabajos, y ServiceURL la dirección pública del servicio,
	// a la que la cola hace las peticiones. Sin cola, los trabajos se
	// ejecutan dentro del proceso, lo que solo sirve fuera de Cloud Run.
	TasksQueue string
	ServiceURL string
}

// Load lee la configuración del entorno y aplica los valores por omisión.
func Load() Config {
	cfg := Config{
		Port:         os.Getenv("PORT"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		WriteToken:   os.Getenv("API_WRITE_TOKEN"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		TasksQueue:   os.Getenv("CLOUD_TASKS_QUEUE"),
		ServiceURL:   os.Getenv("SERVICE_URL"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg
}
