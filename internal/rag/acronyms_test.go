package rag

import "testing"

func TestExpandAcronyms(t *testing.T) {
	for name, tt := range map[string]struct{ in, want string }{
		"sigla sola": {
			"¿Cuáles son los PAS del proyecto?",
			"¿Cuáles son los PAS (Permisos Ambientales Sectoriales) del proyecto?",
		},
		"varias siglas": {
			"¿Qué se informa a la SMA según la RCA?",
			"¿Qué se informa a la SMA (Superintendencia del Medio Ambiente) según la RCA (Resolución de Calificación Ambiental)?",
		},
		"SEA y SEIA son siglas distintas": {
			"El SEA administra el SEIA",
			"El SEA (Servicio de Evaluación Ambiental) administra el SEIA (Sistema de Evaluación de Impacto Ambiental)",
		},
		"nombre ya presente": {
			"¿Qué permisos ambientales sectoriales (PAS) aplican?",
			"¿Qué permisos ambientales sectoriales (PAS) aplican?",
		},
		"minúsculas no son sigla": {
			"¿Qué día se presentó?",
			"¿Qué día se presentó?",
		},
		"parte de otra palabra": {
			"Los PASOS y la DIAGONAL",
			"Los PASOS y la DIAGONAL",
		},
		"seguida de una letra no ASCII": {
			"LO QUE PASÓ CON EL ÍNDICE",
			"LO QUE PASÓ CON EL ÍNDICE",
		},
		"sin siglas": {
			"¿Cuál es la vida útil del proyecto?",
			"¿Cuál es la vida útil del proyecto?",
		},
	} {
		if got := expandAcronyms(tt.in); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", name, got, tt.want)
		}
	}
}
