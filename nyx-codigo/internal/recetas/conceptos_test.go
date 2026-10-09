package recetas

import (
	"testing"

	"nyxcodigo/internal/nucleo"
)

// Every Conceptos entry of a recipe must be in the frozen vocabulary nucleo.Conceptos.
func TestConceptos(t *testing.T) {
	for _, r := range Funciones() {
		if len(r.Conceptos) == 0 {
			t.Errorf("%s: no tiene conceptos", r.Nombre)
		}
		for _, c := range r.Conceptos {
			if !nucleo.EsConcepto(c) {
				t.Errorf("%s: el concepto %q no está en nucleo.Conceptos", r.Nombre, c)
			}
		}
	}
}
