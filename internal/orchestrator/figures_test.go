package orchestrator

import (
	"slices"
	"testing"

	"github.com/rodroguett/ai-doc-platform/internal/rag"
)

func TestFigures(t *testing.T) {
	for in, want := range map[string][]string{
		"una capacidad de 34 m3":            {"34"},
		"el MP-10 y el TK-65":               {"10", "65"},
		"el estanque TK65":                  nil,
		"el D.S. N° 90/2000":                {"90", "2000"},
		"4.000 litros/día durante 48 horas": {"4.000", "48"},
		"el Considerando 12.1":              {"12.1"},
	} {
		if got := figures(in); !slices.Equal(got, want) {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeFigure(t *testing.T) {
	for in, want := range map[string]string{
		"4.000":     "4000",
		"4000":      "4000",
		"6.485.551": "6485551",
		"01":        "1",
		"0":         "0",
		"0,30":      "0.30",
		"1.234,5":   "1234.5",
		"12.1":      "12.1",
		"12.05":     "12.05",
	} {
		if got := normalizeFigure(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func cite(texts ...string) []rag.Passage {
	out := make([]rag.Passage, len(texts))
	for i, t := range texts {
		out[i] = rag.Passage{Text: t}
	}
	return out
}

func TestUnverifiedFigures(t *testing.T) {
	for name, tt := range map[string]struct {
		text  string
		cited []rag.Passage
		want  []string
	}{
		"cifra en el pasaje": {
			"El estanque TK-65 tiene una capacidad de 34 m3 [1].",
			cite("Estanque de almacenamiento de diluyente TK-65, con capacidad de 34 m3."), nil,
		},
		"cifra cambiada": {
			"Se informa a la SMA en un plazo de 48 horas [1].",
			cite("se comunicará a la SMA en un plazo de 24 horas."), []string{"48"},
		},
		// La cifra está en los pasajes, pero no en el que cita su oración.
		"cifra en otro pasaje": {
			"El plazo es de 48 horas [1]. El informe se entrega en 24 horas [2].",
			cite("plazo de 24 horas", "dentro de las 48 horas"), []string{"48", "24"},
		},
		// Medido en la evaluación: el pasaje escribe "01 de agosto".
		"cero a la izquierda": {
			"La DIA se publicó el 1 de agosto de 2024 [1].",
			cite("con fecha 01 de agosto de 2024."), nil,
		},
		"separador de miles": {
			"Un volumen útil de 4000 litros/día [1].",
			cite("equivalente al caudal de diseño de aguas servidas (4.000 litros/día)"), nil,
		},
		// Medido en la evaluación: "D.S." partía la oración y la cifra
		// negada quedaba del lado de las marcas.
		"premisa negada con abreviatura": {
			"Los pasajes no contienen información sobre el D.S. N° 90/2000, pero señalan que los registros estarán disponibles [1].",
			cite("los registros se mantendrán disponibles en las oficinas del Proyecto"), nil,
		},
		"premisa negada y cifra afirmada": {
			"Los pasajes no indican que la capacidad sea de 4 m3, sino que señalan que tiene 34 m3 [1].",
			cite("con capacidad de 34 m3"), nil,
		},
		"la cifra afirmada también se verifica": {
			"Los pasajes no indican que la capacidad sea de 4 m3, sino que señalan que tiene 43 m3 [1].",
			cite("con capacidad de 34 m3"), []string{"43"},
		},
		"oración sin marcas": {
			"Los pasajes no contienen información sobre una capacidad de 4 m3. El estanque tiene 34 m3 [1].",
			cite("con capacidad de 34 m3"), nil,
		},
		"cifra en el contexto del pasaje": {
			"Son compromisos del considerando 8 [1].",
			[]rag.Passage{{Context: "8. Que, el proyecto contempla los siguientes compromisos:", Text: "8.1. Monitoreo de calidad del aire."}}, nil,
		},
		"cifra repetida se avisa una vez": {
			"El plazo es de 72 horas [1]. Son 72 horas [1].",
			cite("plazo de 24 horas"), []string{"72"},
		},
	} {
		if got := unverifiedFigures(tt.text, tt.cited); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, want %q", name, got, tt.want)
		}
	}
}

func TestSplitSentencesKeepsAbbreviations(t *testing.T) {
	got := splitSentences("Según el D.S. N° 90 del art. 5, rige la norma. Luego se informa a la S.A. Minera. Fin.")
	want := []string{"Según el D.S. N° 90 del art. 5, rige la norma. ", "Luego se informa a la S.A. Minera. ", "Fin."}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFiguresNotice(t *testing.T) {
	if got := figuresNotice([]string{"48"}); got != "Aviso: la cifra «48» no aparece en los pasajes que cita su oración. Verifícala en las citas antes de usarla." {
		t.Errorf("singular: %q", got)
	}
	if got := figuresNotice([]string{"48", "72", "4.000"}); got != "Aviso: las cifras «48», «72» y «4.000» no aparecen en los pasajes que citan sus oraciones. Verifícalas en las citas antes de usarlas." {
		t.Errorf("plural: %q", got)
	}
}
