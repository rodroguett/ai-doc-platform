package rag

import (
	"regexp"
	"strings"
)

// acronyms son las siglas del Sistema de Evaluación de Impacto Ambiental con
// las que una persona del sector escribe una pregunta. El documento las
// define una vez y después usa una u otra forma; muchas secciones solo dicen
// el nombre completo.
//
// Cada expansión es la definición que da la propia RCA de prueba.
var acronyms = map[string]string{
	"DIA":  "Declaración de Impacto Ambiental",
	"EIA":  "Estudio de Impacto Ambiental",
	"ICE":  "Informe Consolidado de Evaluación",
	"PAC":  "Participación Ciudadana",
	"PAS":  "Permisos Ambientales Sectoriales",
	"RCA":  "Resolución de Calificación Ambiental",
	"SEA":  "Servicio de Evaluación Ambiental",
	"SEIA": "Sistema de Evaluación de Impacto Ambiental",
	"SMA":  "Superintendencia del Medio Ambiente",
}

// word es una palabra en cualquier alfabeto. Comparar palabras completas
// evita reconocer "PAS" dentro de "PASÓ": los límites de palabra de las
// expresiones regulares de Go solo consideran letras ASCII.
var word = regexp.MustCompile(`[\p{L}\p{N}]+`)

// expandAcronyms agrega a cada sigla de la consulta su nombre completo, entre
// paréntesis: "¿Cuáles son los PAS?" pasa a "¿Cuáles son los PAS (Permisos
// Ambientales Sectoriales)?".
//
// Una pregunta con la sigla sola se parece poco a las secciones que solo
// usan el nombre completo: con "PAS", la sección del permiso del artículo
// 160 quedaba fuera de los primeros pasajes. Solo se expande lo que se
// convierte en vector; la pregunta que ve el modelo no cambia.
//
// Una sigla cuyo nombre ya aparece en la consulta se deja como está.
func expandAcronyms(query string) string {
	lower := strings.ToLower(query)
	return word.ReplaceAllStringFunc(query, func(name string) string {
		full, ok := acronyms[name]
		if !ok || strings.Contains(lower, strings.ToLower(full)) {
			return name
		}
		return name + " (" + full + ")"
	})
}
