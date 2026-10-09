package analisis

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"math/rand"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

const sumaPares = `package solucion

// SumaPares suma los números pares de nums.
func SumaPares(nums []int) int {
	total := 0
	for _, n := range nums {
		if n%2 == 0 {
			total += n
		}
	}
	return total
}
`

func claves(hs []Hallazgo) map[string]int {
	out := map[string]int{}
	for _, h := range hs {
		out[h.Clave]++
	}
	return out
}

func analizar(t *testing.T, src string) Informe {
	t.Helper()
	inf, err := Analizar(src)
	if err != nil {
		t.Fatalf("Analizar: %v", err)
	}
	return inf
}

func TestHallazgos(t *testing.T) {
	casos := []struct {
		nombre, clave, gravedad, src string
		linea                        int
	}{
		{"mapa nil", "mapa_nil", "error", `package p

func Contar(ps []string) map[string]int {
	var m map[string]int
	for _, p := range ps {
		m[p]++
	}
	return m
}
`, 6},
		{"error ignorado", "error_ignorado", "aviso", `package p

import "strconv"

func Leer(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
`, 6},
		{"error tirado", "error_ignorado", "aviso", `package p

import "os"

func Borrar() int {
	os.Remove("x")
	return 1
}
`, 6},
		{"defer en bucle", "defer_en_bucle", "aviso", `package p

import "os"

func Abrir(nombres []string) error {
	for _, n := range nombres {
		f, err := os.Open(n)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	return nil
}
`, 11},
		{"float igual", "float_igual", "aviso", `package p

func Igual(a, b float64) bool {
	return a+0.1 == b
}
`, 4},
		{"concatenar", "concat_en_bucle", "consejo", `package p

func Unir(ps []string) string {
	s := ""
	for _, p := range ps {
		s += p
	}
	return s
}
`, 6},
		{"índice más uno", "indice_mas_uno", "error", `package p

func Subidas(s []int) int {
	c := 0
	for i := 0; i < len(s); i++ {
		if s[i+1] > s[i] {
			c++
		}
	}
	return c
}
`, 6},
		{"división", "div_sin_comprobar", "aviso", `package p

func Media(total, n int) int {
	return total / n
}
`, 4},
		{"bucle sin salida", "bucle_sin_salida", "aviso", `package p

func Siempre() int {
	x := 0
	for {
		x++
	}
}
`, 5},
		{"inalcanzable", "codigo_inalcanzable", "aviso", `package p

func F(x int) int {
	return x
	x++
	return 0
}
`, 5},
		{"err sombreado", "err_sombreado", "aviso", `package p

import "strconv"

func G(a, b string) (int, error) {
	x, err := strconv.Atoi(a)
	if x > 0 {
		y, err := strconv.Atoi(b)
		_ = y
		_ = err
	}
	return x, err
}
`, 8},
		{"copia grande", "rango_copia_grande", "consejo", `package p

type Grande struct {
	Datos [64]int
}

func Suma(gs []Grande) int {
	t := 0
	for _, g := range gs {
		t += g.Datos[0]
	}
	return t
}
`, 9},
		{"goroutine", "variable_bucle_capturada", "consejo", `package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Println(i)
		}()
	}
	wg.Wait()
}
`, 12},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			inf := analizar(t, c.src)
			var h *Hallazgo
			for i := range inf.Hallazgos {
				if inf.Hallazgos[i].Clave == c.clave {
					h = &inf.Hallazgos[i]
				}
			}
			if h == nil {
				t.Fatalf("no detecta %s; hallazgos: %+v (errores %+v)", c.clave, inf.Hallazgos, inf.Errores)
			}
			if h.Linea != c.linea || h.Gravedad != c.gravedad || h.Mensaje == "" || h.Consejo == "" {
				t.Errorf("hallazgo = %+v; quería línea %d, gravedad %s", *h, c.linea, c.gravedad)
			}
		})
	}
}

func TestSinHallazgos(t *testing.T) {
	limpios := []string{sumaPares, `package p

import (
	"fmt"
	"strconv"
	"strings"
)

type Punto struct{ X, Y float64 }

func Contar(ps []string) map[string]int {
	m := make(map[string]int)
	for _, p := range ps {
		m[strings.ToLower(p)]++
	}
	return m
}

func Leer(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("no es un número: %w", err)
	}
	return n, nil
}

func Media(total, n int) int {
	if n == 0 {
		return 0
	}
	return total / n
}

func Pares(s []int) int {
	c := 0
	for i := 0; i+1 < len(s); i++ {
		if s[i+1] > s[i] {
			c++
		}
	}
	fmt.Println(c)
	return c
}

func Esperar(xs []int) int {
	i := 0
	for {
		if i >= len(xs) || xs[i] < 0 {
			break
		}
		i++
	}
	return i
}

func Cero(x float64) bool { return x == 0 }

func Unir(ps []string) string {
	var sb strings.Builder
	for _, p := range ps {
		sb.WriteString(p)
	}
	return sb.String()
}
`}
	for i, src := range limpios {
		inf := analizar(t, src)
		if len(inf.Hallazgos) != 0 {
			t.Errorf("fixture %d: hallazgos inesperados: %+v", i, inf.Hallazgos)
		}
		if len(inf.Errores) != 0 {
			t.Errorf("fixture %d: errores inesperados: %+v", i, inf.Errores)
		}
	}
}

func funcion(t *testing.T, inf Informe, nombre string) InfoFunc {
	t.Helper()
	for _, f := range inf.Funciones {
		if f.Nombre == nombre {
			return f
		}
	}
	t.Fatalf("no encuentro %s en %+v", nombre, inf.Funciones)
	return InfoFunc{}
}

func TestComplejidad(t *testing.T) {
	src := `package p

import "sort"

func Uno(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func Dos(xs []int) int {
	c := 0
	for i := range xs {
		for j := i + 1; j < len(xs); j++ {
			if xs[i] == xs[j] {
				c++
			}
		}
	}
	return c
}

func Tres(n int) int {
	c := 0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			for k := 0; k < n; k++ {
				c++
			}
		}
	}
	return c
}

func Fib(n int) int {
	if n < 2 {
		return n
	}
	return Fib(n-1) + Fib(n-2)
}

func FibMemo(n int, memo map[int]int) int {
	if n < 2 {
		return n
	}
	if v, ok := memo[n]; ok {
		return v
	}
	memo[n] = FibMemo(n-1, memo) + FibMemo(n-2, memo)
	return memo[n]
}

func Fact(n int) int {
	if n <= 1 {
		return 1
	}
	return n * Fact(n-1)
}

func Constante(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	for i := 0; i < 10; i++ {
		xs[0]++
	}
	return xs[0]
}

func Ordenar(xs []int) []int {
	sort.Ints(xs)
	return xs
}

func Digitos(n int) int {
	c := 0
	for n > 0 {
		n /= 10
		c++
	}
	return c
}

func EsPrimo(n int) bool {
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return n > 1
}

func Binaria(xs []int, x int) int {
	lo, hi := 0, len(xs)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		switch {
		case xs[mid] == x:
			return mid
		case xs[mid] < x:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	return -1
}

func Mezclar(xs []int) []int {
	if len(xs) <= 1 {
		return xs
	}
	m := len(xs) / 2
	a, b := Mezclar(xs[:m]), Mezclar(xs[m:])
	out := make([]int, 0, len(xs))
	for len(a) > 0 && len(b) > 0 {
		if a[0] < b[0] {
			out, a = append(out, a[0]), a[1:]
		} else {
			out, b = append(out, b[0]), b[1:]
		}
	}
	return append(append(out, a...), b...)
}

func UsaUno(xss [][]int) int {
	t := 0
	for _, xs := range xss {
		t += Uno(xs)
	}
	return t
}
`
	inf := analizar(t, src)
	esperado := map[string]string{
		"Uno": "O(n)", "Dos": "O(n²)", "Tres": "O(n³)", "Fib": "O(2ⁿ)?", "FibMemo": "O(n)", "Fact": "O(n)",
		"Constante": "O(1)", "Ordenar": "O(n log n)", "Digitos": "O(log n)", "EsPrimo": "O(√n)",
		"Binaria": "O(log n)", "Mezclar": "O(n log n)", "UsaUno": "O(n²)",
	}
	for nombre, o := range esperado {
		f := funcion(t, inf, nombre)
		if f.OGrande != o {
			t.Errorf("%s: OGrande = %s (%s), quería %s", nombre, f.OGrande, f.Motivo, o)
		}
		if f.Motivo == "" {
			t.Errorf("%s: falta el motivo", nombre)
		}
	}
	if f := funcion(t, inf, "Fib"); !f.Recursiva || !f.Exponencial {
		t.Errorf("Fib: recursiva=%v exponencial=%v", f.Recursiva, f.Exponencial)
	}
	if f := funcion(t, inf, "FibMemo"); !f.Recursiva || f.Exponencial {
		t.Errorf("FibMemo: recursiva=%v exponencial=%v", f.Recursiva, f.Exponencial)
	}
	if f := funcion(t, inf, "Uno"); f.Recursiva {
		t.Error("Uno no es recursiva")
	}
	if m := funcion(t, inf, "Dos").Motivo; !strings.Contains(m, "dos bucles anidados") {
		t.Errorf("motivo de Dos = %q", m)
	}
}

func TestCiclomatica(t *testing.T) {
	casos := []struct {
		src  string
		want int
	}{
		{"package p\nfunc F() int { return 1 }\n", 1},
		{sumaPares, 3},
		{"package p\nfunc F(a, b bool) int {\n\tif a && b || !a {\n\t\treturn 1\n\t}\n\treturn 0\n}\n", 4},
		{"package p\nfunc F(x int) string {\n\tswitch x {\n\tcase 1:\n\t\treturn \"uno\"\n\tcase 2, 3:\n\t\treturn \"pocos\"\n\tdefault:\n\t\treturn \"muchos\"\n\t}\n}\n", 3},
		{"package p\nfunc F(xs []int) int {\n\tc := 0\n\tfor i := 0; i < len(xs); i++ {\n\t\tfor _, y := range xs {\n\t\t\tif y > xs[i] {\n\t\t\t\tc++\n\t\t\t} else if y == 0 {\n\t\t\t\tc--\n\t\t\t}\n\t\t}\n\t}\n\treturn c\n}\n", 5},
	}
	for i, c := range casos {
		inf := analizar(t, c.src)
		if len(inf.Funciones) != 1 {
			t.Fatalf("fixture %d: %d funciones", i, len(inf.Funciones))
		}
		if got := inf.Funciones[0].Ciclomatica; got != c.want {
			t.Errorf("fixture %d: ciclomática = %d, quería %d", i, got, c.want)
		}
	}
	inf := analizar(t, casos[4].src)
	if inf.Funciones[0].Anidamiento != 3 {
		t.Errorf("anidamiento = %d, quería 3", inf.Funciones[0].Anidamiento)
	}
}

func TestInforme(t *testing.T) {
	inf := analizar(t, `package main

import (
	"fmt"
	"strings"
)

func main() {
	go fmt.Println(strings.ToUpper("x"))
}
`)
	if !inf.TieneMain || !inf.UsaGo || inf.Paquete != "main" || len(inf.Imports) != 2 || inf.Imports[0] != "fmt" {
		t.Errorf("informe = %+v", inf)
	}
	inf = analizar(t, "package p\nfunc F() int {\n\treturn x\n}\n")
	if len(inf.Errores) == 0 || inf.Errores[0].Linea != 3 {
		t.Errorf("faltan errores de tipos: %+v", inf.Errores)
	}
	// a bare function without a package clause keeps its line numbers
	inf = analizar(t, "func Doble(x int) int {\n\treturn 2 * x\n}\n")
	if len(inf.Funciones) != 1 || inf.Funciones[0].Desde != 1 || inf.Funciones[0].Hasta != 3 || inf.Funciones[0].Firma == nil {
		t.Errorf("función suelta: %+v", inf.Funciones)
	}
}

func tipoDeFuente(t *testing.T, src, expr string) types.Type {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}}
	pkg, err := (&types.Config{Importer: importador{}}).Check("p", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	o := pkg.Scope().Lookup(expr)
	if o == nil {
		t.Fatalf("no encuentro %s", expr)
	}
	return o.Type()
}

func TestTipoDe(t *testing.T) {
	src := `package p

type privado struct {
	nombre string
	edad   int
}

type Celsius float64

var A privado
var B map[string][]int
var C Celsius
var D chan int
var E *privado
var F [3]rune
var G []byte
var H func(int) int
var I any
`
	a, ok := TipoDe(tipoDeFuente(t, src, "A"))
	if !ok || a.Clase != nucleo.CStruct || a.Nombre != "privado" || !a.Definido || len(a.Campos) != 2 || a.Campos[0].Nombre != "nombre" || a.Campos[1].Tipo.Clase != nucleo.CInt {
		t.Errorf("struct con campos privados: %+v %v", a, ok)
	}
	b, ok := TipoDe(tipoDeFuente(t, src, "B"))
	if !ok || b.Go() != "map[string][]int" {
		t.Errorf("mapa: %s %v", b.Go(), ok)
	}
	c, ok := TipoDe(tipoDeFuente(t, src, "C"))
	if !ok || c.Clase != nucleo.CFloat || c.Nombre != "Celsius" || !c.Definido {
		t.Errorf("Celsius: %+v", c)
	}
	if _, ok := TipoDe(tipoDeFuente(t, src, "D")); ok {
		t.Error("un canal no se puede representar")
	}
	e, ok := TipoDe(tipoDeFuente(t, src, "E"))
	if !ok || e.Clase != nucleo.CPuntero || e.Go() != "*privado" {
		t.Errorf("puntero: %+v", e)
	}
	f, ok := TipoDe(tipoDeFuente(t, src, "F"))
	if !ok || f.Go() != "[3]rune" {
		t.Errorf("arreglo: %s", f.Go())
	}
	g, ok := TipoDe(tipoDeFuente(t, src, "G"))
	if !ok || g.Go() != "[]byte" {
		t.Errorf("bytes: %s", g.Go())
	}
	for _, n := range []string{"H", "I"} {
		if _, ok := TipoDe(tipoDeFuente(t, src, n)); ok {
			t.Errorf("%s no se puede representar", n)
		}
	}
	// (int, error) and chan params through Analizar
	inf := analizar(t, `package p

import "errors"

func Dividir(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("división entre cero")
	}
	return a / b, nil
}

func Recibir(c chan int) int { return <-c }
`)
	d := funcion(t, inf, "Dividir")
	if d.Firma == nil || d.Firma.Go() != "func Dividir(a, b int) (int, error)" || d.NoProbable != "" {
		t.Errorf("Dividir: %+v", d)
	}
	r := funcion(t, inf, "Recibir")
	if r.Firma != nil || r.NoProbable != "usa canales" {
		t.Errorf("Recibir: firma %v, motivo %q", r.Firma, r.NoProbable)
	}
}

func TestFirmas(t *testing.T) {
	fs, err := Firmas(`package p

type Punto struct{ X, Y int }

func (p Punto) Suma(q Punto) Punto { return Punto{p.X + q.X, p.Y + q.Y} }

func Max(xs ...int) int {
	m := 0
	for _, x := range xs {
		m = max(m, x)
	}
	return m
}

func Nada() {}

func main() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("firmas = %+v", fs)
	}
	if fs[0].Go() != "func (p Punto) Suma(q Punto) Punto" || fs[0].Receptor == nil || len(fs[0].Receptor.Tipo.Campos) != 2 {
		t.Errorf("método: %s", fs[0].Go())
	}
	if fs[1].Go() != "func Max(xs ...int) int" || !fs[1].Variadica {
		t.Errorf("variádica: %s", fs[1].Go())
	}
}

func TestNormalizar(t *testing.T) {
	out, err := Normalizar("x := 3\nfmt.Println(x)", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"package main", `import "fmt"`, "func main() {", "\tx := 3", "\tfmt.Println(x)"} {
		if !strings.Contains(out, s) {
			t.Errorf("falta %q en:\n%s", s, out)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", out, 0); err != nil {
		t.Errorf("no parsea: %v", err)
	}
	// bare functions, statements and an import, mixed
	out, err = Normalizar("import \"strings\"\n\nfunc doble(x int) int { return 2 * x }\n\nfmt.Println(doble(3), strings.ToUpper(\"a\"))\n", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"package main", `"fmt"`, `"strings"`, "func doble(x int) int", "func main() {"} {
		if !strings.Contains(out, s) {
			t.Errorf("falta %q en:\n%s", s, out)
		}
	}
	// bare function, package solucion
	out, err = Normalizar("func Doble(x int) int {\nreturn 2*x\n}", "solucion")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "package solucion\n") || strings.Contains(out, "func main") || !strings.Contains(out, "return 2 * x") {
		t.Errorf("función suelta:\n%s", out)
	}
	// a complete file changes its package clause only
	out, err = Normalizar("package main\n\nfunc main() {\n\tprintln(1)\n}\n", "otro")
	if err != nil || !strings.HasPrefix(out, "package otro\n") {
		t.Errorf("cambio de paquete: %v\n%s", err, out)
	}
	// immediately-called function literal stays a statement
	out, err = Normalizar("func() {\n\tfmt.Println(1)\n}()\n", "")
	if err != nil || !strings.Contains(out, "func main() {\n\tfunc() {") {
		t.Errorf("literal: %v\n%s", err, out)
	}
	if _, err := Normalizar("x := (", ""); err == nil {
		t.Error("un fragmento roto debe dar error")
	}
}

func TestFuncion(t *testing.T) {
	src := `package p

import (
	"fmt"
	"strings"
)

const limite = 10

type Punto struct{ X, Y int }

func (p Punto) Norma() int { return abs(p.X) + abs(p.Y) }

// abs es el valor absoluto.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func Lejos(ps []Punto) int {
	c := 0
	for _, p := range ps {
		if p.Norma() > limite {
			c++
		}
	}
	return c
}

func Otra() string { return strings.ToUpper(fmt.Sprint(1)) }
`
	out, err := Funcion(src, "Lejos")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"func Lejos", "type Punto", "func (p Punto) Norma", "func abs", "// abs es el valor absoluto.", "const limite = 10"} {
		if !strings.Contains(out, s) {
			t.Errorf("falta %q en:\n%s", s, out)
		}
	}
	for _, s := range []string{"func Otra", "strings", "fmt"} {
		if strings.Contains(out, s) {
			t.Errorf("sobra %q en:\n%s", s, out)
		}
	}
	out, err = Funcion(src, "Otra")
	if err != nil || !strings.Contains(out, `"strings"`) || !strings.Contains(out, `"fmt"`) || strings.Contains(out, "Punto") {
		t.Errorf("Otra: %v\n%s", err, out)
	}
	if _, err := Funcion(src, "NoExiste"); err == nil {
		t.Error("debe fallar con una función que no existe")
	}
}

func TestNarrarSumaPares(t *testing.T) {
	for _, nivel := range []int{1, 2} {
		fr, err := Narrar(sumaPares, "SumaPares", nivel)
		if err != nil {
			t.Fatal(err)
		}
		var todo []string
		for _, f := range fr {
			if f.Linea < 1 || f.Linea > 12 || f.Texto == "" {
				t.Errorf("frase rara: %+v", f)
			}
			todo = append(todo, f.Texto)
		}
		texto := strings.ToLower(strings.Join(todo, "\n"))
		for _, w := range []string{"recorre", "si", "par", "suma", "devuelve"} {
			if !strings.Contains(texto, w) {
				t.Errorf("nivel %d: falta %q en:\n%s", nivel, w, texto)
			}
		}
		if nivel == 1 && !strings.Contains(texto, "recorre cada elemento `n` de `nums` y, si `n` es par, suma `n` a `total`") {
			t.Errorf("nivel 1 no agrupa el bucle:\n%s", texto)
		}
		if nivel == 2 && !strings.Contains(texto, "si `n` es par (`n%2 == 0`):") {
			t.Errorf("nivel 2 sin la condición:\n%s", texto)
		}
	}
	r, err := Roles(sumaPares, "SumaPares")
	if err != nil {
		t.Fatal(err)
	}
	if r["total"] != "acumulador" {
		t.Errorf("roles = %v", r)
	}
}

func TestRoles(t *testing.T) {
	src := `package p

func F(xs []int) (int, bool) {
	c := 0
	hay := false
	res := 0
	for i := 0; i < len(xs); i++ {
		if xs[i] > 10 {
			hay = true
			c++
		}
		res = xs[i]
	}
	tmp := c * 2
	return tmp + res, hay
}
`
	r, err := Roles(src, "F")
	if err != nil {
		t.Fatal(err)
	}
	quiero := map[string]string{"c": "contador", "hay": "bandera", "i": "índice", "tmp": "auxiliar"}
	for v, rol := range quiero {
		if r[v] != rol {
			t.Errorf("%s: %q, quería %q (todos: %v)", v, r[v], rol, r)
		}
	}
}

func TestNarrarVariado(t *testing.T) {
	src := `package p

import (
	"fmt"
	"strings"
)

func Variado(m map[string]int, s string, n int) []string {
	var out []string
	for k, v := range m {
		if v > 0 && strings.HasPrefix(k, "a") {
			out = append(out, k)
		}
	}
	for i, r := range s {
		_ = i
		_ = r
	}
	switch n {
	case 1, 2:
		fmt.Println("poco")
	default:
		return nil
	}
	for n > 0 {
		n /= 2
	}
	return out
}
`
	fr, err := Narrar(src, "Variado", 2)
	if err != nil {
		t.Fatal(err)
	}
	var todo []string
	for _, f := range fr {
		todo = append(todo, f.Texto)
	}
	texto := strings.Join(todo, "\n")
	for _, w := range []string{"recibe `m` (mapa de texto a número)", "Recorre cada clave `k` y su valor `v` del mapa `m`", "`k` empieza por", "Añade `k` al final de `out`", "Recorre cada carácter `r` de `s`", "Según el valor de `n`", "Si es 1 o 2", "En cualquier otro caso", "Repite mientras `n` sea positivo", "Divide `n` entre 2"} {
		if !strings.Contains(texto, w) {
			t.Errorf("falta %q en:\n%s", w, texto)
		}
	}
}

func TestComentar(t *testing.T) {
	for _, src := range []string{sumaPares, "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tx := 3\n\tfor i := 0; i < x; i++ {\n\t\tfmt.Println(i)\n\t}\n}\n"} {
		out, err := Comentar(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "x.go", out, parser.ParseComments); err != nil {
			t.Fatalf("no parsea: %v\n%s", err, out)
		}
		if strings.Count(out, "//") < 3 {
			t.Errorf("pocos comentarios:\n%s", out)
		}
	}
	out, _ := Comentar("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tx := 3\n\tfmt.Println(x)\n}\n")
	if !strings.Contains(out, "\t// Crea `x` con 3.\n\tx := 3") || !strings.Contains(out, "// main no recibe nada ni devuelve nada.") {
		t.Errorf("comentarios:\n%s", out)
	}
}

func TestExprEnEspanol(t *testing.T) {
	casos := map[string]string{
		"x % 2 == 0":               "x es par",
		"x%2 != 0":                 "x es impar",
		"n%3 == 0":                 "n es divisible por 3",
		"len(xs) == 0":             "xs no tiene elementos",
		"len(xs) > 0":              "xs tiene elementos",
		"a < b":                    "a es menor que b",
		"a >= b":                   "a es mayor o igual que b",
		"x > 0":                    "x es positivo",
		"x < 0":                    "x es negativo",
		"x == 0":                   "x es cero",
		"err != nil":               "hay un error",
		"s == \"\"":                "s es el texto vacío",
		"strings.ToUpper(s)":       "s en mayúsculas",
		"strings.Contains(s, \"a\")": "s contiene \"a\"",
		"strings.HasPrefix(s, p)":  "s empieza por p",
		"len(s)":                   "la longitud de s",
		"append(xs, x)":            "xs con x añadido al final",
		"xs[0]":                    "el primer elemento de xs",
		"xs[len(xs)-1]":            "el último elemento de xs",
		"a + b":                    "a más b",
		"x * x":                    "x al cuadrado",
		"a % b":                    "el resto de dividir a entre b",
		"!ok":                      "ok es falso",
		"a > 0 && b > 0":           "a es positivo y b es positivo",
		"math.Sqrt(x)":             "la raíz cuadrada de x",
		"unicode.IsUpper(r)":       "r es una mayúscula",
		"strconv.Itoa(n)":          "n convertido a texto",
		"xs[1:3]":                  "el trozo de xs desde la posición 1 hasta antes de la 3",
		"strings.Fields(s)":        "las palabras de s",
		"true":                     "verdadero",
		"max(a, b)":                "el mayor entre a y b",
	}
	for src, want := range casos {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := ExprEnEspanol(e); got != want {
			t.Errorf("%s → %q, quería %q", src, got, want)
		}
	}
}

// ---- dynamic observation ----

func firmaSumaPares() nucleo.Firma {
	return nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
}

func sumaParesGo(in []nucleo.Valor) []nucleo.Valor {
	xs, _ := in[0].([]nucleo.Valor)
	t := 0
	for _, x := range xs {
		if v := x.(int); v%2 == 0 {
			t += v
		}
	}
	return []nucleo.Valor{t}
}

func aleatorios(n int) []nucleo.Caso {
	r := rand.New(rand.NewSource(1))
	out := make([]nucleo.Caso, n)
	for i := range out {
		xs := make([]nucleo.Valor, r.Intn(8))
		for j := range xs {
			xs[j] = r.Intn(41) - 20
		}
		out[i] = nucleo.Caso{Entradas: []nucleo.Valor{xs}, Origen: nucleo.OrigenAzar}
	}
	return out
}

func TestObservar(t *testing.T) {
	ctx := context.Background()
	f := firmaSumaPares()
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func(in []nucleo.Valor) ([]nucleo.Valor, string) { return sumaParesGo(in), "" },
	}}
	b, _, err := ej.Preparar(ctx, nucleo.Preparacion{Variantes: []string{sumaPares}, Firma: f})
	if err != nil {
		t.Fatal(err)
	}
	sondas := nucleo.Sondas(f)
	if len(sondas) != 30 {
		t.Fatalf("sondas = %d", len(sondas))
	}
	llamado := 0
	resumir := func(ctx context.Context, f nucleo.Firma, casos []nucleo.Caso) (string, func([]nucleo.Valor) ([]nucleo.Valor, error), bool) {
		llamado = len(casos)
		return "devuelve la suma de los números pares de nums", func(in []nucleo.Valor) ([]nucleo.Valor, error) { return sumaParesGo(in), nil }, true
	}
	var huella string
	igual := func(h string) (string, bool) { huella = h; return "SumaPares", true }
	tr := nucleo.NuevaTraza(nil, nil)
	c, err := Observar(ctx, b, f, sondas, aleatorios(300), resumir, igual, tr.Raiz())
	if err != nil {
		t.Fatal(err)
	}
	if !c.Verificado || c.Resumen == "" || c.Pruebas != 300 || c.IgualA != "SumaPares" || c.Huella == "" || huella != c.Huella {
		t.Errorf("comportamiento = %+v", c)
	}
	if len(c.Tabla.Filas) != 30 || llamado != 30 || len(c.Tabla.Cabecera) != 2 || c.Tabla.Cabecera[0] != "nums" {
		t.Errorf("tabla de %d filas, cabecera %v, resumidor con %d casos", len(c.Tabla.Filas), c.Tabla.Cabecera, llamado)
	}
	if c.Tabla.Filas[5][0] != "[1, 2, 3, 4]" || c.Tabla.Filas[5][1] != "6" {
		t.Errorf("fila 6 = %v", c.Tabla.Filas[5])
	}
	if len(tr.Pasos()) == 0 {
		t.Error("no dejó pasos en la traza")
	}
	// a wrong summary is not reported
	malo := func(ctx context.Context, f nucleo.Firma, casos []nucleo.Caso) (string, func([]nucleo.Valor) ([]nucleo.Valor, error), bool) {
		return "devuelve 0", func(in []nucleo.Valor) ([]nucleo.Valor, error) { return []nucleo.Valor{0}, nil }, true
	}
	c, err = Observar(ctx, b, f, sondas, aleatorios(50), malo, nil, nil)
	if err != nil || c.Verificado || c.Resumen != "" {
		t.Errorf("resumen falso aceptado: %+v %v", c, err)
	}
}

// binarioGuion is a scripted nucleo.Binario for timing and trace tests.
type binarioGuion struct {
	micros  func(c nucleo.Caso) int64
	eventos []nucleo.EventoVar
}

func (b *binarioGuion) Probar(ctx context.Context, casos []nucleo.Caso, op nucleo.OpcionesProbar) ([][]nucleo.ResultadoCaso, error) {
	out := make([]nucleo.ResultadoCaso, len(casos))
	for i, c := range casos {
		out[i] = nucleo.ResultadoCaso{Caso: i, Ejecutado: true, OK: true, Obtenido: []nucleo.Valor{0}, Eventos: b.eventos}
		if b.micros != nil {
			out[i].Micros = b.micros(c)
		}
	}
	return [][]nucleo.ResultadoCaso{out}, nil
}
func (b *binarioGuion) Lineas(int) []int  { return nil }
func (b *binarioGuion) NumVariantes() int { return 1 }
func (b *binarioGuion) Cerrar() error     { return nil }

func TestCrecimiento(t *testing.T) {
	caso := func(n int) (nucleo.Caso, bool) {
		return nucleo.Caso{Entradas: []nucleo.Valor{n}, Origen: nucleo.OrigenAzar}, true
	}
	cuad := &binarioGuion{micros: func(c nucleo.Caso) int64 {
		n := float64(c.Entradas[0].(int))
		return int64(50 + n*n/1000)
	}}
	clase, m, err := Crecimiento(context.Background(), cuad, caso, nil)
	if err != nil || clase != "cuadrática" {
		t.Errorf("cuadrática: %q %v %v", clase, m, err)
	}
	if len(m) != 3 || m[len(m)-1].N != 10000 {
		t.Errorf("debía parar al pasar de 1 s: %v", m)
	}
	lin := &binarioGuion{micros: func(c nucleo.Caso) int64 { return int64(30 + c.Entradas[0].(int)/10) }}
	clase, m, err = Crecimiento(context.Background(), lin, caso, nil)
	if err != nil || clase != "lineal" || len(m) != 4 {
		t.Errorf("lineal: %q %v %v", clase, m, err)
	}
	ev := []nucleo.EventoVar{{Linea: 5, Var: "total", Valor: "0"}, {Linea: 8, Var: "total", Valor: "2"}}
	got, err := TrazaVariables(context.Background(), &binarioGuion{eventos: ev}, nucleo.Caso{Entradas: []nucleo.Valor{[]nucleo.Valor{2}}})
	if err != nil || len(got) != 2 || got[1].Valor != "2" {
		t.Errorf("traza: %v %v", got, err)
	}
}

func TestClasificarCrecimiento(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	ruido := func(x float64) int64 { return int64(x * (1 + 0.04*(r.Float64()-0.5))) }
	modelos := map[string]func(n float64) float64{
		"constante":  func(n float64) float64 { return 200 },
		"lineal":     func(n float64) float64 { return 300 + 0.5*n },
		"n·log n":    func(n float64) float64 { return 300 + 0.3*n*math.Log(n) },
		"cuadrática": func(n float64) float64 { return 300 + n*n/100 },
		"cúbica":     func(n float64) float64 { return 300 + n*n*n/1e6 },
		"explosiva":  func(n float64) float64 { return 300 + math.Pow(n, 4)/1e9 },
	}
	for i := 0; i < 20; i++ {
		for want, f := range modelos {
			var m []Medida
			for _, n := range []int{100, 1000, 10000, 100000} {
				m = append(m, Medida{N: n, Micros: ruido(f(float64(n)))})
			}
			if got := ClasificarCrecimiento(m); got != want {
				t.Errorf("ronda %d: %s → %q (%v)", i, want, got, m)
			}
		}
	}
	if got := ClasificarCrecimiento([]Medida{{100, 1000}, {1000, 2_000_000}}); got != "explosiva" && got != "cúbica" {
		t.Errorf("dos medidas muy separadas: %q", got)
	}
}
