package reparar

// Regression tests for the problems found in review (one or more per problem).

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

// compilar runs the compile loop with the fake sandbox (go/types only).
func compilar(t *testing.T, src string) Arreglo {
	t.Helper()
	return (&Reparador{Ej: &nucleotest.EjecutorFalso{}}).Compilacion(context.Background(), src, nil)
}

// An undeclared one-letter variable is declared with «:=», never renamed to another variable in scope
// (renaming compiles but computes something else).
func TestSinDeclararNoRenombraOtraVariable(t *testing.T) {
	casos := []struct {
		nombre, src string
		quiero      []string // must appear in the result
		nunca       []string // must not appear
	}{
		{"suma", "package solucion\n\nfunc F(a, b int) int {\n\tc = a + b\n\treturn c * a\n}\n",
			[]string{"c := a + b", "return c * a"}, []string{"a = a + b", "return a * a"}},
		{"max", "package solucion\n\nfunc Max(xs []int) int {\n\tm = xs[0]\n\tfor _, x := range xs {\n\t\tif x > m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n",
			[]string{"m := xs[0]", "if x > m {", "m = x", "return m"}, []string{"x > x", "x = x"}},
		{"media", "package solucion\n\nfunc Media(x, y float64) float64 {\n\tz = (x + y) / 2\n\treturn z\n}\n",
			[]string{"z := (x + y) / 2", "return z"}, []string{"x = (x + y) / 2"}},
		{"resta", "package solucion\n\nfunc Resta(m, n int) int {\n\tr = m - n\n\treturn r\n}\n",
			[]string{"r := m - n", "return r"}, []string{"m = m - n", "return m"}},
		{"hipotenusa", "package solucion\n\nimport \"math\"\n\nfunc Hipotenusa(a, b float64) float64 {\n\tc = math.Sqrt(a*a + b*b)\n\treturn c\n}\n",
			[]string{"c := math.Sqrt(a*a + b*b)", "return c"}, []string{"a = math.Sqrt", "return a\n"}},
	}
	for _, c := range casos {
		ar := compilar(t, c.src)
		if !ar.OK {
			t.Errorf("%s: no compila: %+v\n%s", c.nombre, ar.Quedan, ar.Fuente)
			continue
		}
		for _, q := range c.quiero {
			if !strings.Contains(ar.Fuente, q) {
				t.Errorf("%s: falta %q:\n%s", c.nombre, q, ar.Fuente)
			}
		}
		for _, n := range c.nunca {
			if strings.Contains(ar.Fuente, n) {
				t.Errorf("%s: no debería salir %q:\n%s", c.nombre, n, ar.Fuente)
			}
		}
		for _, cb := range ar.Cambios {
			if cb.Regla == "levenshtein" {
				t.Errorf("%s: cambió un nombre: %+v", c.nombre, cb)
			}
		}
	}
	// a longer name first assigned with «=» and used again is declared, even with a near name in scope
	ar0 := compilar(t, "package solucion\n\nfunc F(rest, b int) int {\n\tresto = rest % b\n\treturn resto * rest\n}\n")
	if !ar0.OK || !strings.Contains(ar0.Fuente, "resto := rest % b") || !strings.Contains(ar0.Fuente, "return resto * rest") {
		t.Errorf("«resto»: %+v\n%s", ar0.Quedan, ar0.Fuente)
	}
	// a real typo of a longer name is still fixed by its nearest name
	ar := compilar(t, "package solucion\n\nfunc Suma(xs []int) int {\n\ttotal := 0\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn totl\n}\n")
	if !ar.OK || !strings.Contains(ar.Fuente, "return total") {
		t.Errorf("errata «totl»: %+v\n%s", ar.Quedan, ar.Fuente)
	}
}

func TestRenombrePlausible(t *testing.T) {
	casos := []struct {
		a, b string
		ok   bool
	}{
		{"c", "a", false}, {"z", "x", false}, {"r", "m", false}, {"i", "I", true},
		{"ab", "ba", true}, {"ab", "ac", false}, {"tot", "to", false}, {"sm", "sum", true}, {"x", "xs", false},
		{"totl", "total", true}, {"lenght", "length", true}, {"nmbre", "nombre", true},
		{"sumaTotl", "sumaTotal", true}, {"cuenta", "suma", false},
	}
	for _, c := range casos {
		if got := renombrePlausible(c.a, c.b); got != c.ok {
			t.Errorf("renombrePlausible(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// "index out of range [k] with length n" with k > 0 gets no length guard (only [0] with length 0 does).
func TestPanicoSinGuardaParaIndiceMayor(t *testing.T) {
	src := "package solucion\n\nfunc EsPalindromo(s string) bool {\n\tfor i := 0; i < len(s)/2; i++ {\n\t\tif s[i] != s[len(s)-i] {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n"
	a := analizar(src)
	ms := (&Reparador{}).compilables(candidatosPanico(a, nucleo.Firma{Nombre: "EsPalindromo"}, []string{"index out of range [4] with length 4"}))
	for _, m := range ms {
		if m.Cambio.Regla == "guarda_largo" || strings.Contains(m.Fuente, "len(s) <=") {
			t.Errorf("guarda de largo para un índice > 0: %+v", m.Cambio)
		}
	}
	// the empty-input case still gets its guard
	ms = (&Reparador{}).compilables(candidatosPanico(a, nucleo.Firma{Nombre: "EsPalindromo"}, []string{"index out of range [0] with length 0"}))
	hay := false
	for _, m := range ms {
		if m.Cambio.Regla == "guarda_largo" && strings.Contains(m.Fuente, "len(s) == 0") {
			hay = true
		}
	}
	if !hay {
		t.Error("falta la guarda para la entrada vacía")
	}
}

// Panicos rejects a patch that stops the panic by breaking a case the original passed.
func TestPanicosNoRompeCasosQuePasaban(t *testing.T) {
	src := "package solucion\n\nfunc EsPalindromo(s string) bool {\n\tfor i := 0; i < len(s)/2; i++ {\n\t\tif s[i] != s[len(s)-i] {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}\n"
	f := nucleo.Firma{Nombre: "EsPalindromo", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TBool}}
	var casos []nucleo.Caso
	for _, c := range []struct {
		s  string
		ok bool
	}{{"ana", true}, {"casa", false}, {"", true}, {"aa", true}, {"ab", false}} {
		casos = append(casos, nucleo.Caso{Entradas: []nucleo.Valor{c.s}, Esperado: []nucleo.Valor{c.ok}, Expectativa: nucleo.EspUsuario})
	}
	original := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		s := in[0].(string)
		if len(s) >= 2 {
			return nil, "runtime error: index out of range [" + strconv.Itoa(len(s)) + "] with length " + strconv.Itoa(len(s))
		}
		return []nucleo.Valor{true}, ""
	}
	// every candidate behaves like "if len(s) <= 4 { return false }": no panic, but "" → false
	tapa := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		return []nucleo.Valor{false}, ""
	}
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: original}}
	for k := 1; k < 100; k++ {
		ej.Funcs[k] = tapa
	}
	stats := nucleotest.NuevoContador()
	ps, err := (&Reparador{Ej: ej, Stats: stats}).Panicos(context.Background(), src, f, casos, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 0 {
		t.Fatalf("aceptó un parche que rompe «\"\" → true»: %+v", ps[0])
	}
}

// The names the file declares include parameters, named results and closure parameters: a user's
// own «str», «print», «True» or «None» is never rewritten as a foreign idiom.
func TestIdiomasRespetaParametros(t *testing.T) {
	validos := []string{
		"package solucion\n\nfunc F(None, True int) int { return None + True }\n",
		"package solucion\n\nfunc F(print func(int) int) int { return print(1) }\n",
		"package solucion\n\nfunc F(str func(int) string) string { return str(1) }\n",
		"package solucion\n\nfunc F(x int) (null int) {\n\tnull = x\n\treturn null\n}\n",
		"package solucion\n\nfunc F() int {\n\tg := func(False bool, length int) int {\n\t\tif False {\n\t\t\treturn length\n\t\t}\n\t\treturn 0\n\t}\n\treturn g(true, 2)\n}\n",
		"package solucion\n\ntype T struct{}\n\nfunc (print T) M(String string) string { return String }\n",
		"package solucion\n\nfunc F[None any](x None) None { return x }\n",
	}
	for _, src := range validos {
		if nuevo, cs := Idiomas(src); nuevo != src || len(cs) > 0 {
			t.Errorf("cambió código válido:\n%s\n→\n%s\n%+v", src, nuevo, cs)
		}
	}
	// with a compile error on the line, Compilacion must not rewrite the user's own names either
	rotos := []struct{ src, nunca string }{
		{"package solucion\n\nfunc F(str func(int) string) string { return str(1) + 2 }\n", "fmt.Sprint"},
		{"package solucion\n\nfunc F(print func(int) int) int { return print(1) + \"a\" }\n", "fmt.Println"},
		{"package solucion\n\nfunc F(True bool, x int) bool { return True && x }\n", "true bool"},
	}
	for _, c := range rotos {
		ar := compilar(t, c.src)
		if strings.Contains(ar.Fuente, c.nunca) {
			t.Errorf("reescribió un nombre del usuario:\n%s", ar.Fuente)
		}
		for _, cb := range ar.Cambios {
			if strings.Contains(cb.Porque, "esto es de") {
				t.Errorf("cambio de idioma sobre un nombre del usuario: %+v", cb)
			}
		}
	}
	// a real foreign idiom next to the user's names is still rewritten
	nuevo, _ := Idiomas("package solucion\n\nfunc F(xs []int, str string) int {\n\tprint(str)\n\treturn xs.length\n}\n")
	if !strings.Contains(nuevo, "fmt.Println(str)") || !strings.Contains(nuevo, "return len(xs)") {
		t.Errorf("no reescribió los idiomas de verdad:\n%s", nuevo)
	}
}

// Mutantes (and its edits) never panic on a file that does not parse.
func TestMutantesConErrorDeSintaxis(t *testing.T) {
	fuentes := []string{
		"// quiero: compila\npackage solucion\n\nfunc Gritar(s string) string {\n\tret{rn strings.ToUpper(s) + \"!\"\n}\n",
		"package solucion\n\nfunc F(xs []int) int {\n\tt := 0\n\tfor _, x := range xs {\n\t\tt += x\n\t\n\treturn t\n}\n",
		"package solucion\n\nfunc F(a int) int {\n\tif a > 0 {\n\t\treturn a\n\t} els {\n\t\treturn -a\n\t}\n}\n",
		"package solucion\n\nfunc F(a int int {\n\tb := a\n\tc := b\n\treturn c\n}\n",
		"package solucion\n\nfunc F(",
	}
	for _, src := range fuentes {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("pánico con %q: %v", src, r)
				}
			}()
			if ms := Mutantes(src, nil, 0); len(ms) != 0 {
				t.Errorf("mutantes de un archivo que no compila: %d", len(ms))
			}
		}()
	}
	// the edit helpers tolerate inverted positions
	a := analizar(fuentes[0])
	if e := a.ed(68, 18, "", "x"); e.ini <= e.fin {
		t.Errorf("edición con posiciones al revés: %+v", e)
	}
	if nuevo, eds := aplicar(a.fuente, []edicion{a.ed(68, 18, "", "x")}); nuevo != a.fuente || len(eds) != 0 {
		t.Errorf("aplicó una edición imposible")
	}
}

// Diff of a large paste where every line differs runs in bounded time and memory.
func TestDiffGrandeTodoDistinto(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < 3000; i++ {
		linea := "\tx" + strconv.Itoa(i) + " := " + strconv.Itoa(i*7)
		a.WriteString(linea + "\r\n")
		b.WriteString(linea + "\n")
	}
	var antes, despues runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&antes)
	inicio := time.Now()
	d := Diff(a.String(), b.String())
	dur := time.Since(inicio)
	runtime.ReadMemStats(&despues)
	if mb := float64(despues.TotalAlloc-antes.TotalAlloc) / (1 << 20); mb > 64 {
		t.Errorf("Diff reservó %.0f MB", mb)
	}
	if dur > 3*time.Second {
		t.Errorf("Diff tardó %v", dur)
	}
	menos, mas := 0, 0
	for _, l := range strings.Split(d, "\n")[2:] {
		switch {
		case strings.HasPrefix(l, "-"):
			menos++
		case strings.HasPrefix(l, "+"):
			mas++
		}
	}
	if menos != 3000 || mas != 3000 {
		t.Errorf("diff incompleto: -%d +%d", menos, mas)
	}
	// a small change in a big file is still a small hunk
	x := strings.Repeat("a\n", 5000) + "b\n" + strings.Repeat("c\n", 5000)
	y := strings.Repeat("a\n", 5000) + "B\n" + strings.Repeat("c\n", 5000)
	if d := Diff(x, y); d != "--- antes\n+++ después\n@@ -4999,5 +4999,5 @@\n a\n a\n-b\n+B\n c\n c\n" {
		t.Errorf("diff pequeño:\n%s", d)
	}
}

// The diff applied to a reproduces b, for random edits (checks the trimmed traces of myers).
func TestDiffReconstruye(t *testing.T) {
	semilla := uint32(7)
	azar := func(n int) int {
		semilla = semilla*1664525 + 1013904223
		return int(semilla>>8) % n
	}
	for prueba := 0; prueba < 300; prueba++ {
		var la, lb []string
		for i := 0; i < azar(30); i++ {
			la = append(la, string(rune('a'+azar(4))))
		}
		for i := 0; i < azar(30); i++ {
			lb = append(lb, string(rune('a'+azar(4))))
		}
		ops := myers(la, lb)
		var ra, rb []string
		for _, o := range ops {
			switch o.tipo {
			case ' ':
				ra, rb = append(ra, la[o.a]), append(rb, lb[o.b])
			case '-':
				ra = append(ra, la[o.a])
			case '+':
				rb = append(rb, lb[o.b])
			}
		}
		if strings.Join(ra, "") != strings.Join(la, "") || strings.Join(rb, "") != strings.Join(lb, "") {
			t.Fatalf("%v → %v: el guion no reconstruye (%v / %v)", la, lb, ra, rb)
		}
	}
}

// Idiomas is linear in the size of the paste, and Compilacion keeps its time budget and ctx.
func TestIdiomasYCompilacionGrandes(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package solucion\n\nfunc F(xs []int) int {\n")
	for i := 0; i < 8000; i++ {
		sb.WriteString("\tlet x" + strconv.Itoa(i) + " = xs.length;\n")
	}
	sb.WriteString("\treturn 0\n}\n")
	src := sb.String()
	inicio := time.Now()
	nuevo, cs := Idiomas(src)
	if d := time.Since(inicio); d > 3*time.Second {
		t.Errorf("Idiomas con 8000 líneas tardó %v", d)
	}
	if len(cs) < 8000 || strings.Contains(nuevo, ".length") {
		t.Errorf("%d cambios", len(cs))
	}
	// an expired ctx stops the loop at once
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inicio = time.Now()
	ar := (&Reparador{Ej: &nucleotest.EjecutorFalso{}}).Compilacion(ctx, src, nil)
	if d := time.Since(inicio); d > 2*time.Second {
		t.Errorf("Compilacion con el ctx cancelado tardó %v", d)
	}
	if ar.OK {
		t.Error("con el ctx cancelado no debería dar OK")
	}
	// a small time budget is kept on a big paste (with some slack for -race)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	inicio = time.Now()
	(&Reparador{Ej: &nucleotest.EjecutorFalso{}, Limite: Limites{TCompilacion: time.Second}}).Compilacion(ctx2, src, nil)
	if d := time.Since(inicio); d > 4*time.Second {
		t.Errorf("Compilacion con 1 s de límite tardó %v", d)
	}
}

// Only standard-library packages are imported by the in-process type checker: a relative path must
// not make it read files of the server's disk.
func TestChequeoTiposSoloBibliotecaEstandar(t *testing.T) {
	dir := t.TempDir()
	secreto := filepath.Join(dir, "secreto")
	if err := os.MkdirAll(secreto, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secreto, "s.go"), []byte("package secreto\n\nconst Clave = \"hunter2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, secreto)
	if err != nil {
		t.Fatal(err)
	}
	for _, ruta := range []string{filepath.ToSlash(rel), secreto, "./secreto", "C", "github.com/x/y", "nyxcodigo/internal/nucleo"} {
		src := "package solucion\n\nimport s " + strconv.Quote(ruta) + "\n\nvar X int = s.Clave\n"
		errs, _ := ChequeoTipos(src)
		if len(errs) == 0 {
			t.Errorf("%s: sin errores", ruta)
		}
		for _, e := range errs {
			if strings.Contains(e.Msg, "hunter2") {
				t.Fatalf("%s: se leyó el archivo: %s", ruta, e.Msg)
			}
		}
	}
	// standard packages still work
	if errs, err := ChequeoTipos("package solucion\n\nimport \"strings\"\n\nvar X = strings.ToUpper(\"a\")\n"); err != nil || len(errs) > 0 {
		t.Errorf("strings: %v %v", errs, err)
	}
}

// Shrunk cases are not labelled as the user's own example, and the size sum does not overflow.
func TestReduccionLocalEtiquetaYDesborde(t *testing.T) {
	c := nucleo.Caso{Entradas: []nucleo.Valor{listaInt(3, -1, 4)}, Esperado: []nucleo.Valor{2}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario, Nota: "tu ejemplo 2"}
	rs := reduccionesLocal(c)
	if len(rs) == 0 {
		t.Fatal("sin candidatos")
	}
	for _, r := range rs {
		if r.Origen == nucleo.OrigenUsuario || r.Origen != origenReducido || r.Nota != "reducido de «tu ejemplo 2»" || r.ConEsperado() {
			t.Fatalf("candidato mal etiquetado: %+v", r)
		}
	}
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	m := minimizarLocal(context.Background(), c, f, func(cs []nucleo.Caso) []bool {
		out := make([]bool, len(cs))
		for i, x := range cs {
			for _, v := range entrada(x) {
				out[i] = out[i] || v.(int) < 0
			}
		}
		return out
	}, 12)
	if m.Origen != origenReducido || m.Nota == "tu ejemplo 2" {
		t.Errorf("minimizado: %+v", m)
	}

	for _, x := range []int{math.MinInt64, math.MaxInt64} {
		grande := nucleo.Caso{Entradas: []nucleo.Valor{x, x, x}, Origen: nucleo.OrigenAzar}
		if tamanoEntradas(grande.Entradas) < 0 {
			t.Fatalf("%d: el tamaño se desborda", x)
		}
		if rs := reduccionesLocal(grande); len(rs) == 0 {
			t.Fatalf("%d: sin candidatos", x)
		}
	}
}

// Logica records a failure for every mutation operator tried that did not win, so the rate used
// for ranking can also go down.
func TestLogicaApuntaFracasosDeOperadores(t *testing.T) {
	fx := fixture(t, "SumaPares")
	vs := map[int]func([]nucleo.Valor) int{0: buggy}
	for k := 1; k < 1000; k++ {
		vs[k] = buggy // nothing fixes it
	}
	vs[1] = sumaPares // except the first mutant
	stats := nucleotest.NuevoContador()
	r := &Reparador{Ej: ejecutorSumaPares(vs), Stats: stats}
	res, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, oraculoSumaPares, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) == 0 {
		t.Fatalf("sin parches: %s", res.Motivo)
	}
	ops := []string{opRelacional, opAritmetico, opLogico, opNegar, opConstante, opInicioBucle, opLimiteLen, opIndice, opAsignacion, opVariable, opRamas, opBorrar, opIntercambiar, opAcumulador}
	bajan := 0
	for _, op := range ops {
		if stats.Usos("mutacion:"+op) > 0 && stats.Tasa("mutacion:"+op) < 0.5 {
			bajan++
		}
	}
	if bajan == 0 {
		t.Errorf("ningún operador apuntó un fracaso")
	}

	// with no fix at all, every operator tried gets a failure
	vs = map[int]func([]nucleo.Valor) int{0: buggy}
	for k := 1; k < 1000; k++ {
		vs[k] = buggy
	}
	stats = nucleotest.NuevoContador()
	r = &Reparador{Ej: ejecutorSumaPares(vs), Stats: stats, Limite: Limites{Pares: 10}}
	if _, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	usos := 0
	for _, op := range ops {
		usos += stats.Usos("mutacion:" + op)
		if stats.Usos("mutacion:"+op) > 0 && stats.Tasa("mutacion:"+op) >= 0.5 {
			t.Errorf("%s: tasa %v sin ningún éxito", op, stats.Tasa("mutacion:"+op))
		}
	}
	if usos == 0 {
		t.Error("no se apuntó ningún fracaso")
	}
}

// An empty source is "no hay código", not two inserted «}»; Diff ignores only a final newline.
func TestCompilacionVaciaYDiffFinDeLinea(t *testing.T) {
	for _, src := range []string{"", "  \n\t\n"} {
		ar := compilar(t, src)
		if ar.OK || len(ar.Cambios) != 0 || ar.Fuente != src || len(ar.Quedan) != 1 || !strings.Contains(ar.Quedan[0].Msg, "no hay código") {
			t.Errorf("%q: %+v", src, ar)
		}
	}
	casos := []struct{ a, b, quiero string }{
		{"a", "a\n", ""},
		{"a\n", "a", ""},
		{"a\nb", "a\nb\n", ""},
		{"a", "b", "--- antes\n+++ después\n@@ -1 +1 @@\n-a\n+b\n"},
		{"", "", ""},
	}
	for _, c := range casos {
		if d := Diff(c.a, c.b); d != c.quiero {
			t.Errorf("Diff(%q, %q) = %q", c.a, c.b, d)
		}
	}
}

// TestSinPanicoConFuentesRotas mutates the fixtures byte by byte (deterministically) and checks that
// no public entry point panics on the broken result.
func TestSinPanicoConFuentesRotas(t *testing.T) {
	fxs := fixtures(t)
	semilla := uint32(11)
	azar := func(n int) int {
		semilla = semilla*1664525 + 1013904223
		return int(semilla>>8) % n
	}
	trozos := []string{"{", "}", "(", ")", "[", "]", ";", "\n", "\"", "`", "'", ":=", "=", "for ", "if ", "func ", "return ", "x", ".length", "print(", "None", "<=", "/*"}
	n := 400
	if testing.Short() {
		n = 60
	}
	for i := 0; i < n; i++ {
		src := fxs[azar(len(fxs))].Fuente
		for k := 0; k < 1+azar(3); k++ {
			p := azar(len(src) + 1)
			switch azar(3) {
			case 0: // delete a few bytes
				q := min(len(src), p+1+azar(4))
				src = src[:p] + src[q:]
			case 1: // insert a token
				src = src[:p] + trozos[azar(len(trozos))] + src[p:]
			default: // overwrite one byte
				if p < len(src) {
					src = src[:p] + trozos[azar(len(trozos))][:1] + src[p+1:]
				}
			}
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("pánico con\n%s\n%v", src, r)
				}
			}()
			Mutantes(src, nil, 50)
			Idiomas(src)
			ChequeoTipos(src)
			ar := (&Reparador{Ej: &nucleotest.EjecutorFalso{}, Limite: Limites{TCompilacion: 2 * time.Second}}).Compilacion(context.Background(), src, nil)
			Diff(src, ar.Fuente)
		}()
	}
}

// The linker prints the missing-main error with a "runtime.main_main·f: " prefix; it must still be
// translated (it used to reach the user in English).
func TestTraducirMainEnlazador(t *testing.T) {
	for _, msg := range []string{
		"function main is undeclared in the main package",
		"runtime.main_main·f: function main is undeclared in the main package",
	} {
		got := Traducir(nucleo.ErrorGo{Linea: 1, Msg: msg})
		if !strings.Contains(got, "necesita una función `main`") || strings.Contains(got, "undeclared") {
			t.Errorf("Traducir(%q) = %q", msg, got)
		}
	}
}

// A snippet whose package clause shares the first line with code ("package solucion; func F() …", as
// cerebro completes a pasted function so its line numbers do not move) still gets its missing import,
// on that same line, and nothing moves.
func TestImportarConPaqueteEnLaMismaLinea(t *testing.T) {
	casos := []string{
		"package solucion; func F(s string) bool { return strings.HasPrefix(s, \"a\") }\n",
		"package solucion; func Vocales(s string) int {\n\tn := 0\n\tfor _, r := range s {\n\t\tif strings.ContainsRune(\"aeiou\", r) {\n\t\t\tn++\n\t\t}\n\t}\n\treturn n\n}\n",
		"package solucion; func F(x float64) string { return fmt.Sprint(math.Sqrt(x)) }\n",
	}
	for _, src := range casos {
		ar := compilar(t, src)
		if !ar.OK {
			t.Errorf("no compila: %v\n%s", ar.Quedan, ar.Fuente)
			continue
		}
		if !strings.Contains(ar.Fuente, "import") || len(ar.Cambios) == 0 {
			t.Errorf("debía añadir el import:\n%s", ar.Fuente)
		}
		for _, c := range ar.Cambios {
			if c.Regla == "importar" && c.Linea != 1 {
				t.Errorf("el import va en la línea 1: %+v", c)
			}
		}
	}
	// the usual layout is unchanged
	ar := compilar(t, "package solucion\n\nfunc F(s string) bool { return strings.HasPrefix(s, \"a\") }\n")
	if !ar.OK || !strings.HasPrefix(ar.Fuente, "package solucion\n\nimport \"strings\"\n") {
		t.Errorf("import en su sitio de siempre:\n%s", ar.Fuente)
	}
}
