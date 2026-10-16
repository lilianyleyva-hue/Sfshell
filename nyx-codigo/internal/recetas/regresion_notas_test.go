package recetas

import (
	"strings"
	"testing"
)

// The golden exam (§12, rows 25 and 26) answers "¿qué hace strings.Fields?" and "¿para qué sirve defer?"
// from these notes: the Spanish summary must say what the function does in the plain words a learner
// would use ("separa", "al final").
func TestRegresionNotasExamen(t *testing.T) {
	for _, c := range []struct{ simbolo, quiero string }{
		{"strings.Fields", "Separa"},
		{"defer", "al final"},
	} {
		ns := BuscarNotas(nil, c.simbolo, 1)
		if len(ns) == 0 || ns[0].Clave != c.simbolo {
			t.Fatalf("BuscarNotas(%q) = %v", c.simbolo, ns)
		}
		if !strings.Contains(ns[0].Resumen, c.quiero) {
			t.Errorf("la nota de %s debe decir «%s»: %q", c.simbolo, c.quiero, ns[0].Resumen)
		}
	}
}
