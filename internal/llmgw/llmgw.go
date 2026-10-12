// Package llmgw es el contrato del módulo que habla con los proveedores de
// modelos: lo que otros módulos pueden pedirle y los errores que pueden
// distinguir.
//
// Es lo único de llmgw que otro módulo puede importar (ADR-0005). Ningún
// tipo de un proveedor cruza esta frontera: cambiar de proveedor no debe
// tocar a quienes consumen el módulo.
package llmgw

import (
	"context"
	"errors"
)

var (
	// ErrInvalidInput indica que el proveedor rechazó la entrada: reintentar
	// la misma petición dará el mismo resultado.
	ErrInvalidInput = errors.New("entrada rechazada por el proveedor")

	// ErrUnavailable indica que el proveedor no respondió tras agotar los
	// reintentos: una falla transitoria o una cuota agotada. La misma
	// petición puede funcionar más tarde.
	ErrUnavailable = errors.New("proveedor no disponible")

	// ErrMisconfigured indica que el proveedor rechaza las credenciales o no
	// conoce el modelo. No se resuelve reintentando, sino corrigiendo la
	// configuración.
	ErrMisconfigured = errors.New("proveedor mal configurado")
)

// Embedder convierte textos en vectores.
//
// Los vectores de un mismo Embedder son comparables entre sí y están
// normalizados. Los de modelos o dimensiones distintos no lo son: por eso
// quien los guarda debe guardar también Model.
type Embedder interface {
	// EmbedDocuments devuelve un vector por texto, en el mismo orden. Los
	// textos son pasajes de documentos que después se buscarán.
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)

	// EmbedQuery devuelve el vector de una consulta. El modelo distingue una
	// consulta de un pasaje: usar el método equivocado degrada la búsqueda.
	EmbedQuery(ctx context.Context, text string) ([]float32, error)

	// Model identifica al modelo y la dimensión que producen los vectores.
	Model() string

	// Dimensions es el largo de cada vector.
	Dimensions() int
}

// GenerateRequest es lo que se le pide redactar a un modelo.
type GenerateRequest struct {
	// System son las instrucciones: qué puede y qué no puede hacer el
	// modelo. Va separado de Prompt porque Prompt puede incluir texto de
	// quien consulta.
	System string

	// Prompt es el contenido sobre el que el modelo trabaja.
	Prompt string

	// MaxOutputTokens acota el largo de la respuesta.
	MaxOutputTokens int
}

// Generation es el texto que redactó un modelo, con lo necesario para
// declarar en una traza de dónde salió y cuánto costó.
type Generation struct {
	Text     string
	Provider string
	Model    string

	// PromptTokens y OutputTokens son los que informa el proveedor.
	PromptTokens int
	OutputTokens int
}

// Generator redacta texto a partir de instrucciones y contenido.
//
// Devuelve ErrUnavailable cuando ningún modelo pudo responder. Quien llama
// decide entonces cómo degradar: el generador no inventa una respuesta.
type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) (Generation, error)
}

// Usage es el consumo acumulado de un proveedor desde que arrancó el
// proceso.
type Usage struct {
	// Requests cuenta las peticiones enviadas, incluidas las que fallaron y
	// los reintentos: todas descuentan de la cuota.
	Requests int64

	// Tokens es una estimación: el proveedor no informa cuántos consumió
	// cada petición.
	Tokens int64
}
