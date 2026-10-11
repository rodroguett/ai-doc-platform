// Package api expone el contrato OpenAPI para que el binario pueda servirlo.
//
// El archivo se embebe en lugar de leerse del disco porque la imagen de
// producción es distroless y solo contiene el binario.
package api

import _ "embed"

// OpenAPI es el contenido de openapi.yaml, la fuente de verdad de la API.
//
//go:embed openapi.yaml
var OpenAPI []byte
