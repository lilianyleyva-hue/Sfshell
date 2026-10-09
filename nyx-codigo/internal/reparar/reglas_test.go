package reparar

import (
	"context"
	"strings"
	"testing"
)

// TestReglasDeTipos: each type rule of §4.4.3 on a small snippet (pure: go/types only).
func TestReglasDeTipos(t *testing.T) {
	casos := []struct{ src, regla, contiene string }{
		{"func F() rune {\n\tvar r rune = \"a\"\n\treturn r\n}\n", "runa_literal", "var r rune = 'a'"},
		{"func F(s string) int {\n\tvar n int = s\n\treturn n\n}\n", "texto_a_numero", "strconv.Atoi(s)"},
		{"func F(n int) string {\n\tvar s string = n\n\treturn s\n}\n", "convertir_tipo", "strconv.Itoa(n)"},
		{"func F(a int, b float64) float64 {\n\treturn a + b\n}\n", "tipos_distintos", "float64(a) + b"},
		{"func F(a int) int {\n\treturn a, nil\n}\n", "valores_retorno", "return a\n"},
		{"func F(s string) bool {\n\tif s {\n\t\treturn true\n\t}\n\treturn false\n}\n", "condicion_booleana", `if s != "" {`},
		{"func F(total int) int {\n\treturn totl\n}\n", "levenshtein", "return total"},
		{"type P struct{ Nombre string }\n\nfunc F(p P) string {\n\treturn p.nombre\n}\n", "levenshtein", "p.Nombre"},
		{"func F(xs []int) int {\n\tfor i, x := range xs {\n\t\treturn x\n\t}\n\treturn 0\n}\n", "rango_guion_bajo", "for _, x := range xs"},
		{"import \"strconv\"\n\nfunc F(s string) (int, error) {\n\tn := strconv.Atoi(s)\n\treturn n, nil\n}\n", "atoi_dos_valores", "return 0, err"},
		{"func F(xs []int) float64 {\n\treturn len(xs) / 2.5\n}\n", "convertir_tipo", "float64(len(xs)) / 2.5"},
		{"func F(x float64) int {\n\treturn x\n}\n", "convertir_tipo", "return int(x)"},
		{"func F(n int) bool {\n\treturn !n\n}\n", "condicion_booleana", "return n == 0"},
		{"func F(xs []int, i float64) int {\n\treturn xs[i]\n}\n", "indice_entero", "xs[int(i)]"},
		{"func F(x int) int {\n\tx + 1\n\treturn x\n}\n", "expresion_sin_usar", "x += 1"},
		{"func F() int {\n\treturn nil\n}\n", "nil_a_cero", "return 0"},
		{"func F(xs []int) []int {\n\tys := []int{\n\t\t1,\n\t\t2\n\t}\n\treturn ys\n}\n", "coma_final", "2,"},
		{"func F(s string) string {\n\treturn \"hola + s\n}\n", "cerrar_cadena", `"hola + s"`},
		{"func F(n int) int {\n\tif n > 0\n\t{\n\t\treturn 1\n\t}\n\treturn 0\n}\n", "unir_llave", "if n > 0 {"},
		{"x := 5\n\nfunc F() int {\n\treturn x\n}\n", "dos_puntos_igual", "var x = 5"},
		{"func F(m map[string][]int, k string, v int) {\n\tm[k].append(v)\n}\n", "agregar", "m[k] = append(m[k], v)"},
		{"func F(xs []int) []int {\n\tys := []int{}\n\tys = append(ys, xs)\n\treturn ys\n}\n", "puntos_suspensivos", "append(ys, xs...)"},
	}
	for _, c := range casos {
		src := "package solucion\n\n" + c.src
		ar := (&Reparador{}).Compilacion(context.Background(), src, nil)
		var reglas []string
		for _, x := range ar.Cambios {
			reglas = append(reglas, x.Regla)
		}
		if !ar.OK || !contiene(reglas, c.regla) || !strings.Contains(ar.Fuente, c.contiene) {
			t.Errorf("%s: ok=%v reglas=%v quedan=%v\n%s", c.regla, ar.OK, reglas, ar.Quedan, ar.Fuente)
		}
	}
}
