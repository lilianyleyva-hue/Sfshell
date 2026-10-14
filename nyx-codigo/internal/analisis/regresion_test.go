package analisis

// Regression tests for the problems found in the review of this package (one test per problem).

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

// A function without a body (valid Go) and half-typed code used to panic in Analizar.
func TestRegresionFuncionSinCuerpo(t *testing.T) {
	for _, src := range []string{"package p\n\nfunc F() int\n", "package main\nfunc f( {", "func f( {", "package p\n\nfunc (T) M()\n\ntype T struct{}\n"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Analizar(%q) entró en pánico: %v", src, r)
				}
			}()
			inf, err := Analizar(src)
			if err != nil {
				t.Fatalf("Analizar(%q): %v", src, err)
			}
			if len(inf.Funciones) == 0 {
				t.Errorf("Analizar(%q) no vio la función", src)
			}
			for _, f := range inf.Funciones {
				if f.Anidamiento != 0 {
					t.Errorf("%s: anidamiento %d sin cuerpo", f.Nombre, f.Anidamiento)
				}
			}
			Firmas(src)
			Narrar(src, "", 1)
			Roles(src, "")
			Comentar(src)
			Optimizaciones(src)
		}()
	}
}

// Type-checking user code must only read the standard library: module paths, relative and absolute
// paths are refused before any importer (and so any `go list` or cgo run) sees them.
func TestRegresionImportsSoloEstandar(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package p\n\nimport (\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("\tp" + string(rune('a'+i)) + " \"github.com/nadie/paquete" + string(rune('a'+i)) + "\"\n")
	}
	sb.WriteString("\tx \"../../../../../../usr/local/go/src/strings\"\n\ty \"/etc\"\n\tz \"./local\"\n\t\"strings\"\n)\n\n")
	sb.WriteString("func F(s string) string { _, _, _, _ = pa.X, x.ToUpper, y.Z, z.W; return strings.ToUpper(s) }\n")
	inf, err := Analizar(sb.String())
	if err != nil {
		t.Fatal(err)
	}
	rechazados := 0
	for _, e := range inf.Errores {
		if strings.Contains(e.Msg, "solo leo paquetes de la biblioteca estándar") {
			rechazados++
		}
		if strings.Contains(e.Msg, "strings") && strings.Contains(e.Msg, "could not import strings") {
			t.Errorf("no pudo leer strings: %v", e)
		}
	}
	if rechazados != 23 {
		t.Errorf("rechazó %d imports, quería 23: %+v", rechazados, inf.Errores)
	}
	muImportador.Lock()
	defer muImportador.Unlock()
	for ruta, quiero := range map[string]bool{
		"fmt": true, "strings": true, "math/big": true, "net/http": true, "unicode/utf8": true,
		"": false, "C": false, ".": false, "./x": false, "../strings": false, "/etc": false, "/usr/local/go/src/fmt": false,
		"github.com/a/b": false, "golang.org/x/tools": false, "cmd/go": false, "internal/abi": false,
		"strings/../os": false, "vendor/golang.org/x/net/dns/dnsmessage": false, "no/existe": false, "math/testdata": false,
		"container": false, // a directory without Go files
	} {
		if _, ok := dirEstandar(ruta, false); ok != quiero {
			t.Errorf("dirEstandar(%q) = %v, quería %v", ruta, ok, quiero)
		}
	}
	if _, ok := dirEstandar("internal/abi", true); !ok {
		t.Error("las dependencias internas de la biblioteca estándar sí se pueden leer")
	}
}

// The source fallback type-checks the standard library in-process with cgo off (os/user and net have
// cgo files: with cgo on they would run the C compiler).
func TestRegresionImportadorFuentesSinCgo(t *testing.T) {
	if testing.Short() {
		t.Skip("lee la biblioteca estándar desde el código fuente")
	}
	f := nuevoImportadorFuentes()
	if f.ctx.CgoEnabled {
		t.Fatal("el importador de fuentes tiene cgo activado")
	}
	muImportador.Lock()
	p, err := f.Import("os/user")
	muImportador.Unlock()
	if err != nil || p == nil {
		t.Fatalf("os/user: %v", err)
	}
	if p.Scope().Lookup("Current") == nil {
		t.Error("os/user sin Current")
	}
	muImportador.Lock()
	_, err = f.Import("../strings")
	muImportador.Unlock()
	if err == nil {
		t.Error("el importador de fuentes aceptó una ruta relativa")
	}
}

// Without random cases, a summary built from the probes cannot be checked: it is not verified.
func TestRegresionObservarSinAleatorios(t *testing.T) {
	ctx := context.Background()
	f := firmaSumaPares()
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func([]nucleo.Valor) ([]nucleo.Valor, string) { return []nucleo.Valor{0}, "" },
	}}
	b, _, err := ej.Preparar(ctx, nucleo.Preparacion{Variantes: []string{sumaPares}, Firma: f})
	if err != nil {
		t.Fatal(err)
	}
	resumir := func(context.Context, nucleo.Firma, []nucleo.Caso) (string, func([]nucleo.Valor) ([]nucleo.Valor, error), bool) {
		return "devuelve siempre 0", func([]nucleo.Valor) ([]nucleo.Valor, error) { return []nucleo.Valor{0}, nil }, true
	}
	tr := nucleo.NuevaTraza(nil, nil)
	c, err := Observar(ctx, b, f, nucleo.Sondas(f), nil, resumir, nil, tr.Raiz())
	if err != nil {
		t.Fatal(err)
	}
	if c.Verificado || c.Resumen != "" || c.Pruebas != 0 || len(c.Tabla.Filas) != 30 {
		t.Errorf("sin entradas al azar: %+v", c)
	}
	for _, p := range tr.Pasos() {
		if strings.Contains(p.Titulo+p.Detalle, "comprobado con 0") || (p.Estado == nucleo.EstadoBien && strings.Contains(p.Titulo, "En resumen")) {
			t.Errorf("la traza da el resumen por comprobado: %+v", p)
		}
	}
	// with random cases it is verified
	c, err = Observar(ctx, b, f, nucleo.Sondas(f), aleatorios(40), resumir, nil, nil)
	if err != nil || !c.Verificado || c.Pruebas != 40 {
		t.Errorf("con entradas al azar: %+v %v", c, err)
	}
}

func narracion(t *testing.T, src, funcion string, nivel int) string {
	t.Helper()
	fr, err := Narrar(src, funcion, nivel)
	if err != nil {
		t.Fatal(err)
	}
	var partes []string
	for _, f := range fr {
		partes = append(partes, f.Texto)
	}
	return strings.Join(partes, "\n")
}

// Parentheses change the computation: the Spanish keeps the grouping.
func TestRegresionNarrarParentesis(t *testing.T) {
	src := `package p

func Buscar(xs []int, x int) int {
	bajo, alto := 0, len(xs)-1
	for bajo <= alto {
		medio := bajo + (alto-bajo)/2
		if xs[medio] == x {
			return medio
		}
		bajo++
	}
	return -1
}

func Segundo(xs []int) int {
	primero, segundo, haySegundo := 0, 0, false
	for _, x := range xs {
		if x < primero && (!haySegundo || x > segundo) {
			segundo, haySegundo = x, true
		}
	}
	return segundo
}

func Moda(veces map[int]int) int {
	moda, maxVeces := 0, 0
	for x, v := range veces {
		if v > maxVeces || (v == maxVeces && x < moda) {
			moda, maxVeces = x, v
		}
	}
	return moda
}
`
	for funcion, quiero := range map[string]string{
		"Buscar":  "Crea `medio` (será el resultado) con `bajo` más (`alto` menos `bajo`) entre 2.",
		"Segundo": "Si `x` es menor que `primero` y (`haySegundo` es falso o `x` es mayor que `segundo`)",
		"Moda":    "Si `v` es mayor que `maxVeces` o (`v` es igual a `maxVeces` y `x` es menor que `moda`)",
	} {
		if texto := narracion(t, src, funcion, 2); !strings.Contains(texto, quiero) {
			t.Errorf("%s: falta %q en:\n%s", funcion, quiero, texto)
		}
	}
	for expr, quiero := range map[string]string{
		"a - (b - c)":       "a menos (b menos c)",
		"a - b - c":         "a menos b menos c",
		"a + (b + c)":       "a más b más c",
		"(a + b) * c":       "(a más b) por c",
		"a * b + c":         "a por b más c",
		"-(a + b)":          "menos (a más b)",
		"!(a || b)":         "no se cumple que (a o b)",
		"(a + b) % 2 == 0":  "(a más b) es par",
		"(a+b)*(a+b)":       "(a más b) al cuadrado",
		"a && b || c":       "(a y b) o c",
		"a || b || c":       "a o b o c",
		"a / (b * c)":       "a entre (b por c)",
		"len(f(x))":         "la longitud del resultado de f(x)",
		"xs[0] + (n-1)/2*k": "el primer elemento de xs más (n menos 1) entre 2 por k",
	} {
		e, err := parser.ParseExpr(expr)
		if err != nil {
			t.Fatal(err)
		}
		if got := ExprEnEspanol(e); got != quiero {
			t.Errorf("%s → %q, quería %q", expr, got, quiero)
		}
	}
}

// Normalizar keeps leading const and var declarations at package level, where functions can use them.
func TestRegresionNormalizarDeclaraciones(t *testing.T) {
	for _, src := range []string{
		"const N = 3\n\nfunc Triple(x int) int { return x * N }",
		"var cache = map[int]int{}\n\nfunc F(n int) int {\n\tif v, ok := cache[n]; ok {\n\t\treturn v\n\t}\n\tcache[n] = n * 2\n\treturn cache[n]\n}",
		"var total int\n\nfunc Sumar(x int) { total += x }",
		"const (\n\tA = 1\n\tB = 2\n)\n\nfunc Suma() int { return A + B }\n\nfmt.Println(Suma())",
	} {
		out, err := Normalizar(src, "")
		if err != nil {
			t.Fatalf("Normalizar(%q): %v", src, err)
		}
		inf, err := Analizar(out)
		if err != nil || len(inf.Errores) > 0 {
			t.Errorf("Normalizar(%q) no compila:\n%s\n%+v", src, out, inf.Errores)
		}
		if strings.Contains(out, "func main() {\n\tconst") || strings.Contains(out, "func main() {\n\tvar") {
			t.Errorf("la declaración acabó dentro de main:\n%s", out)
		}
	}
	// a var after a statement may use it: it stays in main
	out, err := Normalizar("n := 5\nvar xs = make([]int, n)\nfmt.Println(xs)", "")
	if err != nil || !strings.Contains(out, "\tvar xs = make([]int, n)") {
		t.Errorf("var después de una instrucción: %v\n%s", err, out)
	}
	if inf, _ := Analizar(out); len(inf.Errores) > 0 {
		t.Errorf("no compila: %+v", inf.Errores)
	}
}

// x%2 == 1 is false for negative odd numbers: the narration says "impar y positivo".
func TestRegresionImparNegativo(t *testing.T) {
	src := "package p\n\nfunc EsImpar(n int) bool {\n\treturn n%2 == 1\n}\n\nfunc EsImparU(n uint) bool {\n\treturn n%2 == 1\n}\n"
	if texto := narracion(t, src, "EsImpar", 2); !strings.Contains(texto, "`n` es impar y positivo") {
		t.Errorf("EsImpar:\n%s", texto)
	}
	if texto := narracion(t, src, "EsImparU", 2); !strings.Contains(texto, "`n` es impar,") || strings.Contains(texto, "positivo") {
		t.Errorf("EsImparU (sin signo):\n%s", texto)
	}
	for expr, quiero := range map[string]string{
		"n%2 == 1":  "n es impar y positivo",
		"n%2 != 1":  "n no es un impar positivo",
		"n%2 == -1": "n es impar y negativo",
		"n%2 != 0":  "n es impar",
		"n%2 == 0":  "n es par",
	} {
		e, _ := parser.ParseExpr(expr)
		if got := ExprEnEspanol(e); got != quiero {
			t.Errorf("%s → %q, quería %q", expr, got, quiero)
		}
	}
}

// Boolean returns, contractions and single-case switches read as correct Spanish.
func TestRegresionEspanolClaro(t *testing.T) {
	src := `package p

import (
	"strings"
	"unicode"
)

func EsBisiesto(anio int) bool {
	return anio%4 == 0 && (anio%100 != 0 || anio%400 == 0)
}

func Contar(xs []int) map[int]int {
	veces := map[int]int{}
	for _, x := range xs {
		veces[x]++
	}
	return veces
}

func Letras(s string) []rune {
	var out []rune
	for _, r := range strings.ToLower(f(s)) {
		if unicode.IsLetter(r) {
			out = append(out, r)
		}
	}
	return out
}

func f(s string) string { return s }

func Uno(x int) int {
	switch x {
	case 1:
		return 2
	}
	return 0
}

func Empieza(s string) (int, bool) {
	return len(s), strings.HasPrefix(s, "a")
}
`
	casos := []struct{ funcion, quiero string }{
		{"EsBisiesto", "Devuelve verdadero si `anio` es divisible por 4 y (`anio` no es divisible por 100 o `anio` es divisible por 400), y falso si no."},
		{"Contar", "suma 1 al valor de `x` en el mapa `veces`"},
		{"Letras", "Recorre cada carácter `r` del resultado de `f(s)` en minúsculas"},
		{"Uno", "Según el valor de `x`, comprueba un solo caso."},
		{"Empieza", "Devuelve la longitud de `s` y verdadero o falso según si `s` empieza por `\"a\"`."},
	}
	for _, c := range casos {
		texto := narracion(t, src, c.funcion, 1)
		if !strings.Contains(texto, c.quiero) {
			t.Errorf("%s: falta %q en:\n%s", c.funcion, c.quiero, texto)
		}
		if strings.Contains(texto, " a el ") || strings.Contains(texto, " de el ") || strings.Contains(texto, " 1 casos") {
			t.Errorf("%s: español incorrecto:\n%s", c.funcion, texto)
		}
	}
	e, _ := parser.ParseExpr("len(g(x))")
	if got := ExprEnEspanol(e); got != "la longitud del resultado de g(x)" {
		t.Errorf("contracción: %q", got)
	}
	if got := contraer("suma 1 a el valor de `a el` y de el resto"); got != "suma 1 al valor de `a el` y del resto" {
		t.Errorf("contraer: %q", got)
	}
}

// Comentar never edits inside a doc comment or a multi-line string.
func TestRegresionComentarSintaxis(t *testing.T) {
	out, err := Comentar("package p\n\n// Doble devuelve el doble de x.\nfunc Doble(x int) int { return x * 2 }\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "// Doble devuelve el doble de x.\nfunc Doble(x int) int { return x * 2 } // Devuelve `x` por 2.") {
		t.Errorf("comentario de documentación tocado:\n%s", out)
	}
	src := "package p\n\nimport \"fmt\"\n\nfunc F() {\n\tfmt.Println(1)\n\tx := `abc\ndef`; y := \"!\"\n\tfmt.Println(x, y)\n}\n"
	out, err = Comentar(src)
	if err != nil {
		t.Fatalf("Comentar con una cadena de varias líneas: %v", err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", out, parser.ParseComments)
	if err != nil {
		t.Fatalf("no parsea: %v\n%s", err, out)
	}
	cadena := ""
	ast.Inspect(f, func(n ast.Node) bool {
		if b, ok := n.(*ast.BasicLit); ok && b.Kind == token.STRING && strings.HasPrefix(b.Value, "`") {
			cadena = b.Value
		}
		return true
	})
	if cadena != "`abc\ndef`" {
		t.Errorf("la cadena cambió: %q\n%s", cadena, out)
	}
	if !strings.Contains(out, "y := \"!\" // Crea `y` con `\"!\"`.") {
		t.Errorf("falta el comentario al final de la línea:\n%s", out)
	}
	// a line that ends inside a block comment gets nothing
	out, err = Comentar("package p\n\nfunc G(n int) int { /* a\nb */ return n }\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", out, 0); err != nil {
		t.Fatalf("no parsea: %v\n%s", err, out)
	}
}

// m[k+1] on a map never goes out of range.
func TestRegresionIndiceMasUnoMapa(t *testing.T) {
	inf := analizar(t, "package p\n\nfunc F(m map[int]int) int {\n\tt := 0\n\tfor k := range m {\n\t\tt += m[k+1]\n\t}\n\tfor i := 0; i < len(m); i++ {\n\t\tt += m[i+1]\n\t}\n\treturn t\n}\n")
	for _, h := range inf.Hallazgos {
		if h.Clave == "indice_mas_uno" {
			t.Errorf("falso positivo en un mapa: %+v", h)
		}
	}
	inf = analizar(t, "package p\n\nfunc F(s []int) int {\n\tt := 0\n\tfor k := range s {\n\t\tt += s[k+1]\n\t}\n\treturn t\n}\n")
	if len(inf.Hallazgos) == 0 || inf.Hallazgos[0].Clave != "indice_mas_uno" {
		t.Errorf("con una lista sí: %+v", inf.Hallazgos)
	}
}

// A user package named like a standard library one (image, sort) keeps its own types.
func TestRegresionTipoPropioConNombreEstandar(t *testing.T) {
	for _, paquete := range []string{"image", "sort", "math", "time", "errors"} {
		inf := analizar(t, "package "+paquete+"\n\ntype Punto struct{ X, Y int }\n\nfunc Suma(a, b Punto) int { return a.X + b.X }\n")
		f := funcion(t, inf, "Suma")
		if f.Firma == nil || f.NoProbable != "" {
			t.Errorf("package %s: firma %v, motivo %q", paquete, f.Firma, f.NoProbable)
		}
		if fs, _ := Firmas("package " + paquete + "\n\ntype Punto struct{ X, Y int }\n\nfunc Suma(a, b Punto) int { return a.X + b.X }\n"); len(fs) != 1 {
			t.Errorf("package %s: Firmas = %v", paquete, fs)
		}
	}
	// types of the standard library are still foreign
	inf := analizar(t, "package p\n\nimport \"time\"\n\nfunc Espera(d time.Duration) int { return int(d) }\n")
	if f := funcion(t, inf, "Espera"); f.Firma != nil || !strings.Contains(f.NoProbable, "otro paquete") {
		t.Errorf("time.Duration: %+v", f)
	}
	// the context-free TipoDe: a user package "image" with its own Punto, and the real image.Point
	src := "package image\n\ntype Punto struct{ X, Y int }\n\nvar P Punto\n"
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&types.Config{Importer: importador{}}).Check("image", fset, []*ast.File{af}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := TipoDe(pkg.Scope().Lookup("P").Type()); !ok {
		t.Error("TipoDe rechaza image.Punto del usuario")
	}
	real, err := importador{}.Import("image")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := TipoDe(real.Scope().Lookup("Point").Type()); ok {
		t.Error("TipoDe acepta image.Point de la biblioteca estándar")
	}
}

// Only bounds that depend on the input count towards the order.
func TestRegresionComplejidadCotas(t *testing.T) {
	src := `package p

func Fija(xs []int) int {
	k := 5
	t := 0
	for i := 0; i < k; i++ {
		t += i
	}
	for j := 0; j < len(xs); j++ {
		t += xs[j]
	}
	return t
}

func SoloFija() int {
	k := 5
	var m = k * 2
	t := 0
	for i := 0; i < m; i++ {
		t += i
	}
	return t
}

func Cambia(n int) int {
	k := 5
	k = n
	t := 0
	for i := 0; i < k; i++ {
		t += i
	}
	return t
}

func Criba(n int) []int {
	tachado := make([]bool, n+1)
	var primos []int
	for i := 2; i <= n; i++ {
		if tachado[i] {
			continue
		}
		primos = append(primos, i)
		for j := i * i; j <= n; j += i {
			tachado[j] = true
		}
	}
	return primos
}

func MCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
`
	inf := analizar(t, src)
	for nombre, quiero := range map[string]string{"Fija": "O(n)", "SoloFija": "O(1)", "Cambia": "O(n)", "Criba": "O(n log n)", "MCD": "O(log n)"} {
		if f := funcion(t, inf, nombre); f.OGrande != quiero {
			t.Errorf("%s: %s (%s), quería %s", nombre, f.OGrande, f.Motivo, quiero)
		}
	}
	if m := funcion(t, inf, "Fija").Motivo; strings.Contains(m, "`k`") {
		t.Errorf("Fija: el bucle hasta k no depende de la entrada: %s", m)
	}
	if m := funcion(t, inf, "MCD").Motivo; !strings.Contains(m, "Euclides") {
		t.Errorf("MCD: %s", m)
	}
}
