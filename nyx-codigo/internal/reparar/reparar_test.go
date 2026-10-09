package reparar

import (
	"context"
	"errors"
	"go/format"
	"sort"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

func errorEn(linea int, msg string) nucleo.ErrorGo {
	return nucleo.ErrorGo{Archivo: archivoFuente, Linea: linea, Col: 1, Msg: msg}
}

func fixtures(t *testing.T) []Fixture {
	t.Helper()
	fxs, err := LeerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fxs) < 30 {
		t.Fatalf("testdata/rotos tiene %d archivos; quiero al menos 30", len(fxs))
	}
	return fxs
}

func fixture(t *testing.T, funcion string) Fixture {
	t.Helper()
	for _, fx := range fixtures(t) {
		if fx.Funcion == funcion {
			return fx
		}
	}
	t.Fatalf("no hay fixture para %s", funcion)
	return Fixture{}
}

// ---- Idiomas ----

func TestIdiomasNoTocaTextosNiComentarios(t *testing.T) {
	src := "package solucion\n\n// xs.length es la longitud; True y None aquí no cambian\nfunc F(xs []int) (int, string) {\n\ts := \"xs.length y print( y elif\"\n\tr := '.'\n\t_ = r\n\treturn xs.length, s + `None.push(1)`\n}\n"
	nuevo, cs := Idiomas(src)
	if !strings.Contains(nuevo, "// xs.length es la longitud; True y None aquí no cambian") {
		t.Errorf("cambió un comentario:\n%s", nuevo)
	}
	if !strings.Contains(nuevo, "\"xs.length y print( y elif\"") || !strings.Contains(nuevo, "`None.push(1)`") {
		t.Errorf("cambió un texto:\n%s", nuevo)
	}
	if !strings.Contains(nuevo, "return len(xs), s") {
		t.Errorf("no cambió el .length de verdad:\n%s", nuevo)
	}
	if len(cs) != 1 || cs[0].Regla == "" || !strings.Contains(cs[0].Porque, "en Go se escribe") {
		t.Errorf("cambios: %+v", cs)
	}
}

func TestIdiomasReglas(t *testing.T) {
	casos := []struct{ antes, contiene, lenguaje string }{
		{"xs.push(4)", "xs = append(xs, 4)", "JavaScript"},
		{"xs.append(4)", "xs = append(xs, 4)", "Python"},
		{"if a {\n} elif b {\n}", "} else if b {", "Python"},
		{"x := None", "x := nil", "Python"},
		{"x := null", "x := nil", "JavaScript"},
		{"ok := True", "ok := true", "Python"},
		{"ok := a and not b or c", "ok := a && !b || c", "Python"},
		{"console.log(x)", "fmt.Println(x)", "JavaScript"},
		{"System.out.println(x)", "fmt.Println(x)", "Java"},
		{"int x = 5;", "x := 5", ""},
		{"for (i = 0; i < n; i++) {\n}", "for i := 0; i < n; i++ {", ""},
		{"n := len(m[k].nombres)", "n := len(m[k].nombres)", ""},
		{"n := m[k].nombres.length", "n := len(m[k].nombres)", "JavaScript"},
	}
	for _, c := range casos {
		src := "package main\n\nfunc main() {\n\t" + strings.ReplaceAll(c.antes, "\n", "\n\t") + "\n}\n"
		nuevo, cs := Idiomas(src)
		if !strings.Contains(nuevo, c.contiene) {
			t.Errorf("%q → no contiene %q:\n%s", c.antes, c.contiene, nuevo)
			continue
		}
		if c.lenguaje != "" {
			ok := false
			for _, x := range cs {
				if strings.Contains(x.Porque, c.lenguaje) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%q: ningún cambio menciona %s: %+v", c.antes, c.lenguaje, cs)
			}
		}
	}
	// "function" → "func"
	nuevo, _ := Idiomas("package main\n\nfunction main() {\n}\n")
	if !strings.Contains(nuevo, "func main()") {
		t.Errorf("function: %s", nuevo)
	}
	// var x int = 5; keeps its form but loses the ";"
	nuevo, _ = Idiomas("package main\n\nfunc main() {\n\tvar x int = 5;\n\t_ = x\n}\n")
	if !strings.Contains(nuevo, "var x int = 5\n") {
		t.Errorf("var con punto y coma: %s", nuevo)
	}
}

// ---- compile loop ----

func TestCompilacionFixtures(t *testing.T) {
	for _, fx := range fixtures(t) {
		if !fx.Compila {
			continue
		}
		ej := &nucleotest.EjecutorFalso{}
		stats := nucleotest.NuevoContador()
		r := &Reparador{Ej: ej, Stats: stats}
		inicio := time.Now()
		ar := r.Compilacion(context.Background(), fx.Fuente, nil)
		d := time.Since(inicio)
		if !ar.OK {
			t.Errorf("%s: no compila tras %d rondas: %v\n%s", fx.Nombre, ar.Rondas, ar.Quedan, ar.Fuente)
			continue
		}
		if d > 2*time.Second {
			t.Errorf("%s: tardó %v (más de 2 s)", fx.Nombre, d)
		}
		if errs, err := ChequeoTipos(ar.Fuente); err != nil || len(errs) > 0 {
			t.Errorf("%s: el resultado no pasa go/types: %v", fx.Nombre, errs)
		}
		if ej.NumLlamadas("Compilar") != 1 || ej.NumLlamadas("Vet") != 1 {
			t.Errorf("%s: Compilar %d veces y Vet %d (quiero 1 y 1)", fx.Nombre, ej.NumLlamadas("Compilar"), ej.NumLlamadas("Vet"))
		}
		yaCompilaba := len(mustChequeo(fx.Fuente)) == 0
		if !yaCompilaba && len(ar.Cambios) == 0 {
			t.Errorf("%s: compila pero no explica ningún cambio", fx.Nombre)
		}
		for _, c := range ar.Cambios {
			if c.Regla == "" || c.Porque == "" || c.Linea <= 0 {
				t.Errorf("%s: cambio incompleto %+v", fx.Nombre, c)
			}
		}
		if b, err := format.Source([]byte(ar.Fuente)); err != nil || string(b) != ar.Fuente {
			t.Errorf("%s: el resultado no está gofmt'ed", fx.Nombre)
		}
	}
}

func mustChequeo(src string) []nucleo.ErrorGo {
	errs, err := ChequeoTipos(src)
	if err != nil && len(errs) == 0 {
		return []nucleo.ErrorGo{{Msg: err.Error()}}
	}
	return errs
}

func TestCompilacionCambiosConcretos(t *testing.T) {
	quiero := map[string][2]string{ // fixture → rule, text in the result
		"01_falta_strings.go":     {"importar_paquete", `import "strings"`},
		"03_import_sin_usar.go":   {"quitar_import", ""},
		"04_printl.go":            {"levenshtein", "fmt.Println(\"hola\")"},
		"10_falta_return.go":      {"retorno_faltante", "return 0"},
		"12_atoi.go":              {"atoi_dos_valores", "n, err := strconv.Atoi(s)"},
		"13_redeclarar.go":        {"sombra", "c = c + 1"},
		"14_lenght.go":            {"usar_len", "len(xs)/2"},
		"19_fieldz.go":            {"levenshtein", "strings.Fields(s)"},
		"20_runa.go":              {"runa_literal", "r == 'a'"},
		"22_retorno_valores.go":   {"valores_retorno", "return a / b, nil"},
		"24_numero_a_texto.go":    {"tipos_distintos", "strconv.Itoa(n)"},
		"02_variable_sin_usar.go": {"quitar_variable", ""},
	}
	for _, fx := range fixtures(t) {
		q, ok := quiero[fx.Nombre]
		if !ok {
			continue
		}
		stats := nucleotest.NuevoContador()
		r := &Reparador{Ej: &nucleotest.EjecutorFalso{}, Stats: stats}
		ar := r.Compilacion(context.Background(), fx.Fuente, nil)
		reglas := []string{}
		for _, c := range ar.Cambios {
			reglas = append(reglas, c.Regla)
		}
		if !contiene(reglas, q[0]) {
			t.Errorf("%s: quiero la regla %s, hubo %v", fx.Nombre, q[0], reglas)
		}
		if q[1] != "" && !strings.Contains(ar.Fuente, q[1]) {
			t.Errorf("%s: el resultado no contiene %q:\n%s", fx.Nombre, q[1], ar.Fuente)
		}
		if stats.Usos("regla:"+q[0]) == 0 {
			t.Errorf("%s: no se apuntó el uso de la regla %s", fx.Nombre, q[0])
		}
	}
}

func contiene(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func TestCompilacionSuposicion(t *testing.T) {
	src := "package solucion\n\nfunc Mayor(a, b int) int {\n\tif a > b {\n\t\treturn a\n\t}\n}\n"
	ar := (&Reparador{}).Compilacion(context.Background(), src, nil)
	if !ar.OK || len(ar.Suposiciones) == 0 || !strings.Contains(ar.Suposiciones[0], "return 0") {
		t.Errorf("%+v", ar)
	}
}

func TestCompilacionVerboPrintf(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\ts := \"hola\"\n\tfmt.Printf(\"%d\\n\", s)\n}\n"
	ej := &vetFalso{EjecutorFalso: &nucleotest.EjecutorFalso{}}
	ar := (&Reparador{Ej: ej}).Compilacion(context.Background(), src, nil)
	if !strings.Contains(ar.Fuente, `fmt.Printf("%s\n", s)`) || !ar.OK {
		t.Errorf("%+v\n%s", ar, ar.Fuente)
	}
}

// vetFalso adds the Printf check that the fake executor lacks.
type vetFalso struct {
	*nucleotest.EjecutorFalso
}

func (v *vetFalso) Vet(ctx context.Context, fuente string) nucleo.Compilacion {
	c := v.EjecutorFalso.Vet(ctx, fuente)
	if strings.Contains(fuente, `"%d\n", s`) {
		c.OK = false
		c.Errores = append(c.Errores, nucleo.ErrorGo{Archivo: "x.go", Linea: 7, Col: 2, Msg: "fmt.Printf format %d has arg s of wrong type string"})
	}
	return c
}

func TestChequeoTipos(t *testing.T) {
	errs, err := ChequeoTipos("package solucion\n\nfunc F() int {\n\tx := 1\n\treturn y\n}\n")
	if err != nil || len(errs) != 2 {
		t.Fatalf("%v %v", errs, err)
	}
	if errs[0].Linea != 4 || errs[1].Linea != 5 {
		t.Errorf("líneas: %+v", errs)
	}
	errs, err = ChequeoTipos("package solucion\n\nfunc F( {\n")
	if !errors.Is(err, ErrSintaxis) || len(errs) == 0 {
		t.Errorf("%v %v", errs, err)
	}
	if errs, err := ChequeoTipos("package solucion\n\nimport \"strings\"\n\nfunc F(s string) string { return strings.ToUpper(s) }\n"); err != nil || len(errs) != 0 {
		t.Errorf("%v %v", errs, err)
	}
}

func TestTraducirMensajesReales(t *testing.T) {
	// the first error of every "compila" fixture has a Spanish template (no "El compilador dice")
	for _, fx := range fixtures(t) {
		if !fx.Compila {
			continue
		}
		errs := mustChequeo(fx.Fuente)
		if len(errs) == 0 {
			continue
		}
		if es := Traducir(errs[0]); strings.HasPrefix(es, "El compilador dice") {
			t.Errorf("%s: %q no tiene plantilla", fx.Nombre, errs[0].Msg)
		}
	}
}

// ---- Diff ----

func TestDiffDorado(t *testing.T) {
	a := "package solucion\n\nfunc Maximo(xs []int) int {\n\tm := xs[0]\n\tfor _, x := range xs {\n\t\tif x < m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n"
	b := strings.Replace(a, "x < m", "x > m", 1)
	quiero := "--- antes\n+++ después\n@@ -4,5 +4,5 @@\n \tm := xs[0]\n \tfor _, x := range xs {\n-\t\tif x < m {\n+\t\tif x > m {\n \t\t\tm = x\n \t\t}\n"
	if got := Diff(a, b); got != quiero {
		t.Errorf("Diff =\n%q\nquiero\n%q", got, quiero)
	}
	// the same hunks as GNU "diff -U2"
	casos := []struct{ a, b, hunks string }{
		{"a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\n", "a\nB\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\n",
			"@@ -1,4 +1,4 @@\n a\n-b\n+B\n c\n d\n@@ -11,2 +11,3 @@\n k\n l\n+m\n"},
		{"uno\ndos\ntres\ncuatro\ncinco\nseis\nsiete\nocho\nnueve\ndiez\n", "cero\nuno\ndos\ntres\ncuatro\nCINCO\nseis\nsiete\nocho\nnueve\n",
			"@@ -1,10 +1,10 @@\n+cero\n uno\n dos\n tres\n cuatro\n-cinco\n+CINCO\n seis\n siete\n ocho\n nueve\n-diez\n"},
		{"a\nb\nc\n", "", "@@ -1,3 +0,0 @@\n-a\n-b\n-c\n"},
		{"", "a\nb\nc\n", "@@ -0,0 +1,3 @@\n+a\n+b\n+c\n"},
	}
	for _, c := range casos {
		if got := Diff(c.a, c.b); got != "--- antes\n+++ después\n"+c.hunks {
			t.Errorf("Diff(%q, %q) =\n%s\nquiero\n%s", c.a, c.b, got, c.hunks)
		}
	}
	if Diff(a, a) != "" {
		t.Error("textos iguales dan un diff vacío")
	}
}

// ---- Ochiai ----

func TestOchiai(t *testing.T) {
	// 5 statements; statement 3 is the bug: run by every failing case and by one passing case
	cubiertas := [][]int32{
		{0, 1, 2, 4},    // pass
		{0, 1, 3, 4},    // fail
		{0, 1, 2, 3, 4}, // fail
		{0, 1, 2, 4},    // pass
		{0, 1, 3, 3, 4}, // pass (3 listed twice counts once)
		{0, 3},          // fail
		{0, 99, -1},     // pass (ids out of range are ignored)
	}
	ok := []bool{true, false, false, true, true, false, true}
	p := Ochiai(cubiertas, ok, 5)
	if len(p) != 5 {
		t.Fatalf("%v", p)
	}
	orden := []int{0, 1, 2, 3, 4}
	sort.SliceStable(orden, func(i, j int) bool { return p[orden[i]] > p[orden[j]] })
	if orden[0] != 3 {
		t.Errorf("la sentencia 3 debería ser la más sospechosa: %v", p)
	}
	// fail(3)=3, pass(3)=1, totalFail=3 → 3/sqrt(3·4) = 0.866
	if d := p[3] - 0.8660254; d > 1e-6 || d < -1e-6 {
		t.Errorf("Ochiai(3) = %v", p[3])
	}
	if p2 := Ochiai(cubiertas[:1], ok[:1], 5); p2[0] != 0 {
		t.Errorf("sin fallos todo vale 0: %v", p2)
	}
}

func TestSospechosasIncluyeCabeceraDelBucle(t *testing.T) {
	src := "package solucion\n\nfunc F(xs []int) int {\n\tt := 0\n\tfor _, x := range xs {\n\t\tif x > 0 {\n\t\t\tt += x\n\t\t}\n\t}\n\treturn t\n}\n"
	a := analizar(src)
	lineas := []int{4, 5, 6, 7, 10}
	puntos := []float64{0, 0, 0, 0.9, 0}
	porLinea, ls := sospechosas(a, lineas, puntos)
	if !igualesInt(ls, []int{5, 7}) {
		t.Errorf("líneas = %v (quiero la sentencia 7 y la cabecera del bucle 5)", ls)
	}
	if porLinea[7] != 0.9 {
		t.Errorf("%v", porLinea)
	}
}

func igualesInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- Mutantes ----

const fuenteSuma = "package solucion\n\nfunc Suma(xs []int) int {\n\ttotal := 0\n\tfor i := 0; i < len(xs); i++ {\n\t\ttotal += xs[i]\n\t}\n\treturn total\n}\n"

func contarOperadores(ms []Mutante) map[string]int {
	out := map[string]int{}
	for _, m := range ms {
		out[m.Operador]++
	}
	return out
}

func TestMutantesOperadores(t *testing.T) {
	cabecera := Mutantes(fuenteSuma, []int{5}, 0)
	quiero := map[string]int{opIntercambiar: 1, opInicioBucle: 1, opNegar: 1, opConstante: 1, opRelacional: 5, opVariable: 1, opLimiteLen: 1, opAritmetico: 1}
	if got := contarOperadores(cabecera); !igualesMapas(got, quiero) {
		t.Errorf("línea 5: %v\nquiero %v", got, quiero)
	}
	todo := Mutantes(fuenteSuma, nil, 0)
	quiero = map[string]int{opIntercambiar: 1, opAcumulador: 4, opConstante: 2, opInicioBucle: 1, opNegar: 1, opRelacional: 5,
		opVariable: 2, opLimiteLen: 1, opAritmetico: 5, opBorrar: 1, opAsignacion: 1, opIndice: 2}
	if got := contarOperadores(todo); !igualesMapas(got, quiero) {
		t.Errorf("todo el archivo: %v\nquiero %v", got, quiero)
	}
	vistos := map[string]bool{normalizarFuente(fuenteSuma): true}
	for _, m := range todo {
		if vistos[m.Fuente] {
			t.Errorf("mutante repetido (o igual al original): %+v", m.Cambio)
		}
		vistos[m.Fuente] = true
		if m.Cambio.Porque == "" || m.Cambio.Regla == "" || m.Cambio.Linea == 0 {
			t.Errorf("cambio incompleto: %+v", m.Cambio)
		}
	}
	if n := len(Mutantes(fuenteSuma, nil, 7)); n != 7 {
		t.Errorf("max 7 da %d", n)
	}
	if len(Mutantes(fuenteSuma, []int{1}, 0)) != 0 {
		t.Error("la línea 1 (package) no tiene mutantes")
	}
}

func igualesMapas(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestMutantesContienenArreglos: for every logic fixture, the human fix is among the compiling mutants.
func TestMutantesContienenArreglos(t *testing.T) {
	quiero := map[string]func(string) bool{
		"SumaTodos":     func(s string) bool { return strings.Contains(s, "i < len(xs)") },
		"Maximo":        func(s string) bool { return strings.Contains(s, "x > m") },
		"SumaPares":     func(s string) bool { return strings.Contains(s, "total := 0") },
		"Mayor":         func(s string) bool { return strings.Contains(s, "return mayor") && !strings.Contains(s, "ultimo") },
		"SumaCuadrados": func(s string) bool { return strings.Contains(s, "return x * x") },
	}
	r := &Reparador{}
	for _, fx := range fixtures(t) {
		ok, hay := quiero[fx.Funcion]
		if !hay {
			continue
		}
		ms := r.compilables(generarMutaciones(analizar(fx.Fuente), nil))
		encontrado := false
		for _, m := range ms {
			if ok(m.Fuente) {
				encontrado = true
			}
		}
		if !encontrado {
			t.Errorf("%s: ningún mutante es el arreglo (%d mutantes)", fx.Nombre, len(ms))
		}
	}
}

func TestLimpiarVariableSinUsar(t *testing.T) {
	fx := fixture(t, "Mayor")
	src := strings.Replace(fx.Fuente, "return ultimo", "return mayor", 1)
	nuevo, extra, ok := limpiarYChequear(src)
	if !ok || strings.Contains(nuevo, "ultimo") || len(extra) != 2 {
		t.Errorf("ok=%v extra=%+v\n%s", ok, extra, nuevo)
	}
	// a variable whose assignment has side effects keeps it: "_ = x"
	src = "package solucion\n\nfunc g() int { return 1 }\n\nfunc F() int {\n\tx := 0\n\tx = g()\n\treturn 2\n}\n"
	nuevo, _, ok = limpiarYChequear(src)
	if !ok || !strings.Contains(nuevo, "x = g()") {
		t.Errorf("ok=%v\n%s", ok, nuevo)
	}
}

// ---- panic fixes ----

func TestCandidatosPanico(t *testing.T) {
	casos := []struct {
		src, funcion, msg, contiene string
	}{
		{fixture(t, "Primero").Fuente, "Primero", "index out of range [0] with length 0", "if len(xs) == 0 {\n\t\treturn 0\n\t}"},
		{fixture(t, "SumaTodos").Fuente, "SumaTodos", "index out of range [3] with length 3", "i < len(xs)"},
		{fixture(t, "SumaTodos").Fuente, "SumaTodos", "index out of range [3] with length 3", "i <= len(xs)-1"},
		{fixture(t, "Frecuencias").Fuente, "Frecuencias", "assignment to entry in nil map", "m := make(map[string]int)"},
		{"package solucion\n\nfunc Media(xs []int) int {\n\ts := 0\n\tfor _, x := range xs {\n\t\ts += x\n\t}\n\treturn s / len(xs)\n}\n",
			"Media", "integer divide by zero", "if len(xs) == 0 {\n\t\treturn 0\n\t}\n\treturn s / len(xs)"},
		{"package solucion\n\ntype Nodo struct {\n\tV   int\n\tSig *Nodo\n}\n\nfunc Valor(n *Nodo) int {\n\treturn n.V\n}\n",
			"Valor", "invalid memory address or nil pointer dereference", "if n == nil {\n\t\treturn 0\n\t}"},
		{"package solucion\n\nfunc Primeros(xs []int, n int) []int {\n\treturn xs[:n]\n}\n",
			"Primeros", "slice bounds out of range [:5] with capacity 3", "xs[:min(n, len(xs))]"},
		{"package solucion\n\nfunc Al(xs []int) int {\n\tt := 0\n\tfor i := len(xs); i >= 0; i-- {\n\t\tt += xs[i]\n\t}\n\treturn t\n}\n",
			"Al", "index out of range [3] with length 3", "i := len(xs) - 1"},
	}
	for _, c := range casos {
		a := analizar(c.src)
		ms := (&Reparador{}).compilables(candidatosPanico(a, nucleo.Firma{Nombre: c.funcion}, []string{c.msg}))
		ok := false
		var vistos []string
		for _, m := range ms {
			if strings.Contains(m.Fuente, c.contiene) {
				ok = true
				if m.Operador != opPanico || m.Cambio.Porque == "" {
					t.Errorf("%s: cambio incompleto %+v", c.funcion, m.Cambio)
				}
			}
			vistos = append(vistos, m.Cambio.Despues)
		}
		if !ok {
			t.Errorf("%s / %q: ningún candidato contiene %q; hay %q", c.funcion, c.msg, c.contiene, vistos)
		}
	}
}

func listaInt(xs ...int) []nucleo.Valor {
	out := make([]nucleo.Valor, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func entrada(c nucleo.Caso) []nucleo.Valor { return c.Entradas[0].([]nucleo.Valor) }

func TestPanicosConFalso(t *testing.T) {
	fx := fixture(t, "Primero")
	primero := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		xs := in[0].([]nucleo.Valor)
		if len(xs) == 0 {
			return nil, "runtime error: index out of range [0] with length 0"
		}
		return []nucleo.Valor{xs[0]}, ""
	}
	arreglada := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		xs := in[0].([]nucleo.Valor)
		if len(xs) == 0 {
			return []nucleo.Valor{0}, ""
		}
		return []nucleo.Valor{xs[0]}, ""
	}
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: primero}}
	for k := 1; k < 50; k++ {
		ej.Funcs[k] = arreglada
	}
	stats := nucleotest.NuevoContador()
	r := &Reparador{Ej: ej, Stats: stats}
	traza := nucleo.NuevaTraza(func(nucleo.Evento) {}, nil)
	ps, err := r.Panicos(context.Background(), fx.Fuente, fx.Firma, fx.Casos, traza.Raiz())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) == 0 {
		t.Fatal("no hay parches")
	}
	p := ps[0]
	if p.Pasan != 3 || p.Total != 3 || p.Cambios[0].Regla != "guarda_largo" || !strings.Contains(p.Fuente, "len(xs) == 0") {
		t.Errorf("%+v", p)
	}
	if ej.NumLlamadas("Preparar") != 2 {
		t.Errorf("Preparar %d veces (quiero 2: el original y un lote)", ej.NumLlamadas("Preparar"))
	}
	if stats.Usos("regla:guarda_largo") == 0 {
		t.Error("no se apuntó la regla")
	}
	hayPaso := false
	for _, pa := range traza.Pasos() {
		if strings.Contains(pa.Titulo, "lista vacía") || strings.Contains(pa.Titulo, "Ya no se rompe") {
			hayPaso = true
		}
	}
	if !hayPaso {
		t.Errorf("la traza no lo cuenta: %+v", traza.Pasos())
	}

	// without panics there is nothing to do
	ej2 := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: arreglada}}
	ps, err = (&Reparador{Ej: ej2}).Panicos(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil)
	if err != nil || ps != nil {
		t.Errorf("%v %v", ps, err)
	}
}

func TestErroresDeEntrada(t *testing.T) {
	fx := fixture(t, "Primero")
	if _, err := (&Reparador{}).Panicos(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil); !errors.Is(err, ErrSinEjecutor) {
		t.Errorf("sin ejecutor: %v", err)
	}
	if _, err := (&Reparador{}).Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, nil, nil); !errors.Is(err, ErrSinEjecutor) {
		t.Errorf("sin ejecutor: %v", err)
	}
	r := &Reparador{Ej: &nucleotest.EjecutorFalso{}}
	roto := strings.Replace(fx.Fuente, "return xs[0]", "return ys[0]", 1)
	if _, err := r.Logica(context.Background(), roto, fx.Firma, fx.Casos, nil, nil, nil); !errors.Is(err, nucleo.ErrNoCompila) {
		t.Errorf("no compila: %v", err)
	}
	if _, err := r.Panicos(context.Background(), roto, fx.Firma, fx.Casos, nil); !errors.Is(err, nucleo.ErrNoCompila) {
		t.Errorf("no compila: %v", err)
	}
	if _, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, nil, nil, nil, nil); err == nil {
		t.Error("sin casos debería dar error")
	}
}

// ---- logic repair with the fake sandbox ----

func sumaPares(xs []nucleo.Valor) int {
	t := 0
	for _, x := range xs {
		if x.(int)%2 == 0 {
			t += x.(int)
		}
	}
	return t
}

func ejecutorSumaPares(variantes map[int]func(xs []nucleo.Valor) int) *nucleotest.EjecutorFalso {
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){}}
	for k, f := range variantes {
		f := f
		ej.Funcs[k] = func(in []nucleo.Valor) ([]nucleo.Valor, string) { return []nucleo.Valor{f(in[0].([]nucleo.Valor))}, "" }
	}
	return ej
}

func oraculoSumaPares(in []nucleo.Valor) ([]nucleo.Valor, error) {
	return []nucleo.Valor{sumaPares(in[0].([]nucleo.Valor))}, nil
}

func buggy(xs []nucleo.Valor) int { return sumaPares(xs) + 1 }

func TestLogicaConFalso(t *testing.T) {
	fx := fixture(t, "SumaPares")
	vs := map[int]func([]nucleo.Valor) int{0: buggy}
	for k := 1; k < 1000; k++ {
		vs[k] = sumaPares
	}
	ej := ejecutorSumaPares(vs)
	stats := nucleotest.NuevoContador()
	r := &Reparador{Ej: ej, Stats: stats}
	res, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, oraculoSumaPares, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) == 0 || len(res.Parches) > maxParchesLogica {
		t.Fatalf("%d parches: %s", len(res.Parches), res.Motivo)
	}
	for _, p := range res.Parches {
		if p.Pasan != 4 || p.Total != 4 || len(p.Cambios) == 0 {
			t.Errorf("%+v", p)
		}
		if errs := mustChequeo(p.Fuente); len(errs) > 0 {
			t.Errorf("un parche no compila: %v", errs)
		}
	}
	if res.Ambiguo != nil {
		t.Errorf("todas las variantes hacen lo mismo: %+v", res.Ambiguo)
	}
	ce := res.Contraejemplo
	if ce == nil || len(entrada(*ce)) != 0 || !ce.ConEsperado() || ce.Esperado[0] != 0 {
		t.Errorf("contraejemplo: %+v", ce)
	}
	usos := 0
	for _, op := range []string{opRelacional, opAritmetico, opLogico, opNegar, opConstante, opInicioBucle, opLimiteLen, opIndice, opAsignacion, opVariable, opRamas, opBorrar, opIntercambiar, opAcumulador} {
		usos += stats.Usos("mutacion:" + op)
	}
	if usos == 0 {
		t.Error("no se apuntó el operador del arreglo")
	}
	// one coverage build plus one batch of mutants
	if n := ej.NumLlamadas("Preparar"); n != 2 {
		t.Errorf("Preparar %d veces", n)
	}
}

func TestLogicaConFalsoAmbigua(t *testing.T) {
	fx := fixture(t, "SumaPares")
	otra := func(xs []nucleo.Valor) int {
		if len(xs) > 5 {
			return sumaPares(xs) + 1
		}
		return sumaPares(xs)
	}
	ej := ejecutorSumaPares(map[int]func([]nucleo.Valor) int{0: buggy, 1: sumaPares, 2: otra})
	r := &Reparador{Ej: ej}
	res, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) != 2 || res.Ambiguo == nil {
		t.Fatalf("quiero 2 parches y una ambigüedad: %+v", res)
	}
	amb := res.Ambiguo
	if len(amb.Parches) != 2 || len(amb.Salidas) != 2 || len(entrada(nucleo.Caso{Entradas: amb.Entrada})) <= 5 {
		t.Errorf("%+v", amb)
	}
	if nucleo.Igual(amb.Salidas[0][0], amb.Salidas[1][0]) {
		t.Errorf("las salidas deberían ser distintas: %v", amb.Salidas)
	}
	// with the oracle, the overfitted patch is dropped and nothing is ambiguous
	res, err = r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, oraculoSumaPares, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) != 1 || res.Ambiguo != nil {
		t.Errorf("con referencia quiero 1 parche sin ambigüedad: %+v", res)
	}
}

func TestLogicaConFalsoSinArreglo(t *testing.T) {
	fx := fixture(t, "SumaPares")
	casi := func(xs []nucleo.Valor) int {
		if len(xs) == 0 {
			return 1
		}
		return sumaPares(xs)
	}
	ej := ejecutorSumaPares(map[int]func([]nucleo.Valor) int{0: buggy, 1: casi})
	r := &Reparador{Ej: ej}
	res, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) != 0 || res.Motivo == "" || res.Contraejemplo == nil {
		t.Fatalf("%+v", res)
	}
	if res.Mejor == nil || res.Mejor.Pasan != 3 || res.Mejor.Total != 4 {
		t.Errorf("mejor parcial: %+v", res.Mejor)
	}
}

func TestLogicaYaPasa(t *testing.T) {
	fx := fixture(t, "SumaPares")
	ej := ejecutorSumaPares(map[int]func([]nucleo.Valor) int{0: sumaPares})
	res, err := (&Reparador{Ej: ej}).Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, nil, nil)
	if err != nil || len(res.Parches) != 0 || res.Contraejemplo != nil || !strings.Contains(res.Motivo, "ya pasa") {
		t.Errorf("%+v %v", res, err)
	}
}

// ---- built-in generator and shrinker ----

func TestCasosAzar(t *testing.T) {
	punto := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Punto", Definido: true, Campos: []nucleo.Campo{{Nombre: "X", Tipo: nucleo.TInt}, {Nombre: "Y", Tipo: nucleo.TFloat}}}
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{
		{Nombre: "a", Tipo: nucleo.ListaDe(nucleo.TInt)}, {Nombre: "s", Tipo: nucleo.TString},
		{Nombre: "m", Tipo: nucleo.MapaDe(nucleo.TString, nucleo.TInt)}, {Nombre: "p", Tipo: nucleo.PunteroA(punto)},
		{Nombre: "b", Tipo: nucleo.Tipo{Clase: nucleo.CInt, Nombre: "uint8"}}, {Nombre: "r", Tipo: nucleo.TRune},
	}, Res: []nucleo.Tipo{nucleo.TInt}}
	a, b := casosAzar(f, 50, 7), casosAzar(f, 50, 7)
	if len(a) != 50 {
		t.Fatalf("%d casos", len(a))
	}
	for i := range a {
		for j := range a[i].Entradas {
			if nucleo.Clave(a[i].Entradas[j]) != nucleo.Clave(b[i].Entradas[j]) {
				t.Fatalf("la misma semilla da casos distintos")
			}
		}
		if _, ok := a[i].Entradas[0].([]nucleo.Valor); !ok && a[i].Entradas[0] != nil {
			t.Errorf("lista: %T", a[i].Entradas[0])
		}
		if _, ok := a[i].Entradas[2].(nucleo.Mapa); !ok {
			t.Errorf("mapa: %T", a[i].Entradas[2])
		}
		if x := a[i].Entradas[4].(int); x < 0 || x > 255 {
			t.Errorf("uint8 fuera de rango: %d", x)
		}
		if a[i].Expectativa != nucleo.EspNinguna || a[i].Origen != nucleo.OrigenAzar {
			t.Errorf("%+v", a[i])
		}
	}
}

func TestMinimizarLocal(t *testing.T) {
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	c := nucleo.Caso{Entradas: []nucleo.Valor{listaInt(5, 3, -7, 8, 2, -4)}, Esperado: []nucleo.Valor{1}, Expectativa: nucleo.EspUsuario}
	falla := func(cs []nucleo.Caso) []bool {
		out := make([]bool, len(cs))
		for i, x := range cs {
			for _, v := range entrada(x) {
				if v.(int) < 0 {
					out[i] = true
				}
			}
		}
		return out
	}
	m := minimizarLocal(context.Background(), c, f, falla, 12)
	if xs := entrada(m); len(xs) != 1 || xs[0] != -1 || m.ConEsperado() {
		t.Errorf("%+v", m)
	}
}

// TestCompilacionConcurrente: several repairs at once share the cached importer safely.
func TestCompilacionConcurrente(t *testing.T) {
	fxs := fixtures(t)
	hecho := make(chan string, len(fxs))
	n := 0
	for _, fx := range fxs {
		if !fx.Compila {
			continue
		}
		n++
		go func(fx Fixture) {
			ar := (&Reparador{}).Compilacion(context.Background(), fx.Fuente, nil)
			if !ar.OK {
				hecho <- fx.Nombre
				return
			}
			hecho <- ""
		}(fx)
	}
	for i := 0; i < n; i++ {
		if nombre := <-hecho; nombre != "" {
			t.Errorf("%s no compila cuando se repara a la vez que otros", nombre)
		}
	}
}
