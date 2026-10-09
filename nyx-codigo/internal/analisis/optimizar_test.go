package analisis

import (
	"strings"
	"testing"
)

func unaReescritura(t *testing.T, src, clave string) Reescritura {
	t.Helper()
	rs := Optimizaciones(src)
	var hay []Reescritura
	for _, r := range rs {
		if r.Hallazgo.Clave == clave {
			hay = append(hay, r)
		}
	}
	if len(hay) != 1 {
		t.Fatalf("esperaba una reescritura %s y hay %d: %+v", clave, len(hay), rs)
	}
	if _, err := Analizar(hay[0].Fuente); err != nil {
		t.Fatalf("la reescritura no se puede analizar: %v", err)
	}
	inf, _ := Analizar(hay[0].Fuente)
	if len(inf.Errores) > 0 {
		t.Fatalf("la reescritura tiene errores: %+v\n%s", inf.Errores, hay[0].Fuente)
	}
	return hay[0]
}

func TestOptimizarConcatenacion(t *testing.T) {
	src := `package p

import "fmt"

// Unir junta las palabras.
func Unir(ws []string) string {
	s := ""
	for _, w := range ws {
		s += w + " "
	}
	fmt.Println(len(s))
	return s
}
`
	r := unaReescritura(t, src, "concat_en_bucle")
	for _, w := range []string{"var s strings.Builder", `s.WriteString(w + " ")`, "return s.String()", "len(s.String())", `"strings"`, "// Unir junta las palabras."} {
		if !strings.Contains(r.Fuente, w) {
			t.Errorf("falta %q en:\n%s", w, r.Fuente)
		}
	}
	if r.Hallazgo.Linea != 9 || r.Hallazgo.Gravedad != "consejo" {
		t.Errorf("hallazgo = %+v", r.Hallazgo)
	}
	// with an initial value and no imports at all
	src2 := "package p\n\nfunc F(n int) string {\n\tvar s = \"x\"\n\tfor i := 0; i < n; i++ {\n\t\ts += \"y\"\n\t}\n\treturn s\n}\n"
	r = unaReescritura(t, src2, "concat_en_bucle")
	if !strings.Contains(r.Fuente, `s.WriteString("x")`) || !strings.Contains(r.Fuente, `import "strings"`) {
		t.Errorf("valor inicial:\n%s", r.Fuente)
	}
}

func TestOptimizarConcatenacionNo(t *testing.T) {
	for _, src := range []string{
		// read inside the loop
		"package p\n\nfunc F(ws []string) int {\n\ts := \"\"\n\tn := 0\n\tfor _, w := range ws {\n\t\ts += w\n\t\tn += len(s)\n\t}\n\treturn n\n}\n",
		// not in a loop
		"package p\n\nfunc F(a, b string) string {\n\ts := a\n\ts += b\n\treturn s\n}\n",
		// a parameter
		"package p\n\nfunc F(s string, ws []string) string {\n\tfor _, w := range ws {\n\t\ts += w\n\t}\n\treturn s\n}\n",
		// assigned with =
		"package p\n\nfunc F(ws []string) string {\n\ts := \"\"\n\tfor _, w := range ws {\n\t\ts += w\n\t}\n\ts = \"otra\"\n\treturn s\n}\n",
		// does not compile
		"package p\n\nfunc F(ws []string) string {\n\ts := \"\"\n\tfor _, w := range ws {\n\t\ts += w\n\t}\n\treturn s + 1\n}\n",
	} {
		for _, r := range Optimizaciones(src) {
			if r.Hallazgo.Clave == "concat_en_bucle" {
				t.Errorf("no debería reescribir:\n%s\n→\n%s", src, r.Fuente)
			}
		}
	}
}

func TestOptimizarContains(t *testing.T) {
	src := `package p

import "slices"

func Comunes(a, b []int) []int {
	var out []int
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
`
	r := unaReescritura(t, src, "contains_en_bucle")
	for _, w := range []string{"conjuntoB := make(map[int]bool, len(b))", "conjuntoB[v] = true", "if conjuntoB[x] {"} {
		if !strings.Contains(r.Fuente, w) {
			t.Errorf("falta %q en:\n%s", w, r.Fuente)
		}
	}
	if strings.Contains(r.Fuente, "slices") {
		t.Errorf("el import de slices debe desaparecer:\n%s", r.Fuente)
	}
	// conditional append: no capacity rewrite
	for _, x := range Optimizaciones(src) {
		if x.Hallazgo.Clave == "append_sin_capacidad" {
			t.Errorf("append condicional no debe reescribirse:\n%s", x.Fuente)
		}
	}
	// the list changes inside the loop: no rewrite
	malo := "package p\n\nimport \"slices\"\n\nfunc F(a []int) []int {\n\tvar vistos []int\n\tfor _, x := range a {\n\t\tif !slices.Contains(vistos, x) {\n\t\t\tvistos = append(vistos, x)\n\t\t}\n\t}\n\treturn vistos\n}\n"
	for _, x := range Optimizaciones(malo) {
		if x.Hallazgo.Clave == "contains_en_bucle" {
			t.Errorf("la lista cambia en el bucle; no debe reescribirse:\n%s", x.Fuente)
		}
	}
	// another use of slices keeps the import
	otro := "package p\n\nimport \"slices\"\n\nfunc F(a, b []string) int {\n\tslices.Sort(a)\n\tn := 0\n\tfor _, x := range a {\n\t\tif slices.Contains(b, x) {\n\t\t\tn++\n\t\t}\n\t}\n\treturn n\n}\n"
	r = unaReescritura(t, otro, "contains_en_bucle")
	if !strings.Contains(r.Fuente, "slices.Sort(a)") || !strings.Contains(r.Fuente, "map[string]bool") {
		t.Errorf("import de slices:\n%s", r.Fuente)
	}
}

func TestOptimizarCapacidad(t *testing.T) {
	src := `package p

func Dobles(xs []int) []int {
	var out []int
	for _, x := range xs {
		out = append(out, 2*x)
	}
	return out
}

func Claves(m map[string]int) []string {
	claves := []string{}
	for k := range m {
		claves = append(claves, k)
	}
	return claves
}
`
	rs := Optimizaciones(src)
	if len(rs) != 2 {
		t.Fatalf("esperaba 2 reescrituras: %+v", rs)
	}
	if !strings.Contains(rs[0].Fuente, "out := make([]int, 0, len(xs))") || rs[0].Hallazgo.Clave != "append_sin_capacidad" || rs[0].Hallazgo.Linea != 6 {
		t.Errorf("Dobles:\n%+v", rs[0])
	}
	if !strings.Contains(rs[1].Fuente, "claves := make([]string, 0, len(m))") {
		t.Errorf("Claves:\n%s", rs[1].Fuente)
	}
	// two appends per turn, or the list used before the loop: no rewrite
	for _, src := range []string{
		"package p\n\nfunc F(xs []int) []int {\n\tvar out []int\n\tfor _, x := range xs {\n\t\tout = append(out, x)\n\t\tout = append(out, x)\n\t}\n\treturn out\n}\n",
		"package p\n\nfunc F(xs []int) []int {\n\tvar out []int\n\tout = append(out, 0)\n\tfor _, x := range xs {\n\t\tout = append(out, x)\n\t}\n\treturn out\n}\n",
		"package p\n\nfunc F(s string) []rune {\n\tvar out []rune\n\tfor _, r := range s {\n\t\tout = append(out, r)\n\t}\n\treturn out\n}\n",
	} {
		if rs := Optimizaciones(src); len(rs) != 0 {
			t.Errorf("no debería reescribir:\n%s\n→\n%+v", src, rs)
		}
	}
}

func TestOptimizarFragmento(t *testing.T) {
	// a snippet without package clause still works (the result is a whole file)
	src := "func Unir(ws []string) string {\n\ts := \"\"\n\tfor _, w := range ws {\n\t\ts += w\n\t}\n\treturn s\n}\n"
	r := unaReescritura(t, src, "concat_en_bucle")
	if !strings.HasPrefix(r.Fuente, "package main") || r.Hallazgo.Linea != 4 {
		t.Errorf("fragmento:\n%s\n%+v", r.Fuente, r.Hallazgo)
	}
	// bare statements that need imports are normalized first
	r = unaReescritura(t, "x := \"\"\nfor i := 0; i < 3; i++ {\n\tx += \"a\"\n}\nfmt.Println(x)\n", "concat_en_bucle")
	for _, w := range []string{"func main() {", "\"fmt\"", "\"strings\"", "fmt.Println(x.String())"} {
		if !strings.Contains(r.Fuente, w) {
			t.Errorf("falta %q en:\n%s", w, r.Fuente)
		}
	}
	if rs := Optimizaciones("esto no es Go {"); rs != nil {
		t.Errorf("código roto: %+v", rs)
	}
}
