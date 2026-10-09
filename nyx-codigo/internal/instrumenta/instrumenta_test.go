package instrumenta

import (
	"flag"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

var actualizar = flag.Bool("actualizar", false, "reescribe los archivos .instr.golden")

var opGolden = nucleo.OpcionesInstr{Combustible: 1000, MaxPila: 50, Cobertura: true, Variables: true}

func leer(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGolden(t *testing.T) {
	archivos, err := filepath.Glob("testdata/*.go")
	if err != nil || len(archivos) == 0 {
		t.Fatalf("no hay archivos de prueba: %v", err)
	}
	for _, a := range archivos {
		t.Run(filepath.Base(a), func(t *testing.T) {
			fuente := leer(t, a)
			r, err := Instrumentar(fuente, opGolden)
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.TrimSuffix(a, ".go") + ".instr.golden"
			if *actualizar {
				if err := os.WriteFile(golden, []byte(r.Fuente), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			esperado := leer(t, golden)
			if r.Fuente != esperado {
				t.Errorf("la salida no coincide con %s (usa -actualizar si el cambio es correcto)\n--- obtenido:\n%s", golden, r.Fuente)
			}
		})
	}
}

// fuenteImportador type-checks against GOROOT sources; it is skipped when they are not available.
func fuenteImportador(t *testing.T, fset *token.FileSet) types.Importer {
	t.Helper()
	imp := importer.ForCompiler(fset, "source", nil)
	if _, err := imp.Import("fmt"); err != nil {
		t.Skipf("no puedo leer la biblioteca estándar: %v", err)
	}
	return imp
}

func TestInstrumentadoCompilaConSoporte(t *testing.T) {
	archivos, _ := filepath.Glob("testdata/*.go")
	for _, a := range archivos {
		t.Run(filepath.Base(a), func(t *testing.T) {
			fuente := leer(t, a)
			for _, op := range []nucleo.OpcionesInstr{opGolden, nucleo.InstrNormal, {Cobertura: true}, {Variables: true}, {}} {
				r, err := Instrumentar(fuente, op)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(r.Fuente, "\n") != strings.Count(fuente, "\n") {
					t.Errorf("%+v: el número de líneas cambió", op)
				}
				fset := token.NewFileSet()
				f1, err := parser.ParseFile(fset, "v.go", r.Fuente, 0)
				if err != nil {
					t.Fatalf("%+v: la salida no se puede leer: %v\n%s", op, err, r.Fuente)
				}
				f2, err := parser.ParseFile(fset, "s.go", Soporte(f1.Name.Name, op, len(r.Lineas)), 0)
				if err != nil {
					t.Fatalf("el soporte no se puede leer: %v", err)
				}
				conf := types.Config{Importer: fuenteImportador(t, fset)}
				if _, err := conf.Check(f1.Name.Name, fset, []*ast.File{f1, f2}, nil); err != nil {
					t.Fatalf("%+v: no pasa go/types: %v\n%s", op, err, r.Fuente)
				}
			}
		})
	}
}

func TestLineas(t *testing.T) {
	fuente := leer(t, "testdata/ramas.go")
	r, err := Instrumentar(fuente, nucleo.OpcionesInstr{Cobertura: true})
	if err != nil {
		t.Fatal(err)
	}
	// every coverage counter sits on the line Lineas says
	lineas := strings.Split(r.Fuente, "\n")
	re := regexp.MustCompile(`nyx__c\[(\d+)\]\+\+`)
	vistos := 0
	for i, l := range lineas {
		for _, m := range re.FindAllStringSubmatch(l, -1) {
			id, _ := strconv.Atoi(m[1])
			if id >= len(r.Lineas) || r.Lineas[id] != i+1 {
				t.Errorf("el contador %d está en la línea %d pero Lineas dice %v", id, i+1, r.Lineas)
			}
			vistos++
		}
	}
	if vistos != len(r.Lineas) {
		t.Errorf("hay %d contadores y %d líneas", vistos, len(r.Lineas))
	}
	// expected statements: if(7) return(8) return(10); nombre:=(15) switch(16) =(18) =(20) =(22) return(24);
	// total:=(28) range(29) if(30) +=(31) return(34)
	esperado := []int{7, 8, 10, 15, 16, 18, 20, 22, 24, 28, 29, 30, 31, 34}
	if len(r.Lineas) != len(esperado) {
		t.Fatalf("Lineas = %v, esperaba %v", r.Lineas, esperado)
	}
	for i := range esperado {
		if r.Lineas[i] != esperado[i] {
			t.Fatalf("Lineas = %v, esperaba %v", r.Lineas, esperado)
		}
	}
}

func TestRecursivas(t *testing.T) {
	fuente := leer(t, "testdata/bucles.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, 0)
	if err != nil {
		t.Fatal(err)
	}
	rec := funcionesRecursivas(f)
	nombres := map[string]bool{}
	for fd := range rec {
		nombres[fd.Name.Name] = true
	}
	for _, n := range []string{"Factorial", "esPar", "esImpar"} {
		if !nombres[n] {
			t.Errorf("%s debería ser recursiva", n)
		}
	}
	for _, n := range []string{"Cuenta", "Busca", "main", "Salta"} {
		if nombres[n] {
			t.Errorf("%s no es recursiva", n)
		}
	}
	r, err := Instrumentar(fuente, nucleo.OpcionesInstr{MaxPila: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Factorial, esPar, esImpar and the closure assigned to fib
	if n := strings.Count(r.Fuente, "nyx__Hondo{}"); n != 4 {
		t.Errorf("hay %d comprobaciones de profundidad, esperaba 4:\n%s", n, r.Fuente)
	}
	if strings.Contains(r.Fuente, "nyx__f--") || strings.Contains(r.Fuente, "nyx__c[") {
		t.Error("sin combustible ni cobertura no debe insertar esos contadores")
	}
}

func TestCombustibleEnEtiquetas(t *testing.T) {
	fuente := leer(t, "testdata/bucles.go")
	r, err := Instrumentar(fuente, nucleo.OpcionesInstr{Combustible: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Fuente, "otra:\n\tnyx__f--; if nyx__f < 0 { panic(nyx__SinComb{}) }; i++") {
		t.Errorf("la etiqueta de goto debe llevar combustible:\n%s", r.Fuente)
	}
	if !strings.Contains(r.Fuente, "fuera:\n\tfor i, fila := range m { nyx__f--") {
		t.Errorf("la etiqueta de un bucle debe quedarse pegada al bucle:\n%s", r.Fuente)
	}
}

func TestPrefijo(t *testing.T) {
	casos := map[string]bool{
		"var Nyx__x int":            true,
		"package p\nvar NYX__Y = 1": true,
		"package p\nfunc f() { a_nyx__b := 1; _ = a_nyx__b }": true,
		"package p\n// nyx__ en un comentario\nvar x int":     false,
		"package p\nvar s = \"nyx__\"":                        false,
		"package p\nvar nyx_x int":                            false,
	}
	for src, esp := range casos {
		if got := UsaPrefijo(src); got != esp {
			t.Errorf("UsaPrefijo(%q) = %v, esperaba %v", src, got, esp)
		}
	}
	if _, err := Instrumentar("package p\nvar nyx__f int", opGolden); err != ErrPrefijo {
		t.Errorf("Instrumentar debe rechazar el prefijo, dio %v", err)
	}
	if l, ok := PosicionPrefijo("package p\n\nvar a, Nyx__b int"); !ok || l != 3 {
		t.Errorf("PosicionPrefijo = %d, %v", l, ok)
	}
}

func TestGoroutinas(t *testing.T) {
	if !TieneGoroutinas("package p\nfunc f() { go f() }") {
		t.Error("no detectó go f()")
	}
	if TieneGoroutinas("package p\nfunc f() { f() }") {
		t.Error("detectó una goroutine que no existe")
	}
	if !TieneGoroutinas("package p\nfunc f() { go f( }") {
		t.Error("con código roto debe buscar la palabra go")
	}
}

func TestLimites(t *testing.T) {
	grande := "package p\n" + strings.Repeat("// relleno relleno relleno relleno relleno relleno\n", 1400)
	if _, err := Instrumentar(grande, opGolden); err != ErrGrande {
		t.Errorf("esperaba ErrGrande, dio %v", err)
	}
	var sb strings.Builder
	sb.WriteString("package p\nfunc f() {\n")
	for i := 0; i < 5001; i++ {
		sb.WriteString("_ = 1\n")
	}
	sb.WriteString("}\n")
	if _, err := Instrumentar(sb.String(), opGolden); err != ErrSentencias {
		t.Errorf("esperaba ErrSentencias, dio %v", err)
	}
	if _, err := Instrumentar("package p\nfunc (", opGolden); err == nil {
		t.Error("código roto debe dar error")
	}
}

func TestEventos(t *testing.T) {
	src := `package p

var global int

func f(xs []int) int {
	s := 0
	for i, x := range xs {
		s += x * i
	}
	global = s
	var a, _ = 1, 2
	a++
	return s + a
}
`
	r, err := Instrumentar(src, nucleo.OpcionesInstr{Variables: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, quiero := range []string{
		`s := 0; nyx__ev(6, "s", s)`,
		`range xs { nyx__ev(7, "i", i); nyx__ev(7, "x", x);`,
		`s += x * i; nyx__ev(8, "s", s)`,
		`var a, _ = 1, 2; nyx__ev(11, "a", a)`,
		`a++; nyx__ev(12, "a", a)`,
	} {
		if !strings.Contains(r.Fuente, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, r.Fuente)
		}
	}
	if strings.Contains(r.Fuente, `"global"`) {
		t.Errorf("las variables globales no se trazan:\n%s", r.Fuente)
	}
}

func TestRenombrar(t *testing.T) {
	src := `package q

import "fmt"

type Punto struct {
	X, Y int
	nombre string
}

type Caja struct {
	Punto
	Alto int
}

const Max = 10

var total int

func (p *Punto) Mover(dx int) { p.X += dx; total++ }

func (p Punto) String() string { return fmt.Sprint(p.X, p.Y, p.nombre) }

func Fact(n int) int {
	if n <= 1 {
		return 1
	}
	return n * Fact(n-1)
}

func Usar() int {
	f := func() int { return total + Max }
	c := Caja{Punto: Punto{X: 1, nombre: "a"}, Alto: 2}
	c.Punto.Mover(1)
	c.Mover(Max)
	m := map[int]string{Max: "max"}
	_ = m
	Fact := 3 // local shadow: not renamed
	return f() + Fact + c.X + c.Alto
}

func init() { total = 1 }
`
	out, nombres, err := Renombrar(src, "_v3")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Punto", "Caja", "Max", "total", "Fact", "Usar"} {
		if nombres[n] != n+"_v3" {
			t.Errorf("nombres[%q] = %q", n, nombres[n])
		}
	}
	if _, ok := nombres["init"]; ok {
		t.Error("init no se renombra")
	}
	for _, quiero := range []string{
		"type Punto_v3 struct",
		"X, Y int",
		"nombre string",
		"Punto_v3\n\tAlto int",
		"func (p *Punto_v3) Mover(dx int) { p.X += dx; total_v3++ }",
		"func (p Punto_v3) String() string",
		"return n * Fact_v3(n-1)",
		"return total_v3 + Max_v3",
		"c := Caja_v3{Punto_v3: Punto_v3{X: 1, nombre: \"a\"}, Alto: 2}",
		"c.Punto_v3.Mover(1)",
		"c.Mover(Max_v3)",
		"map[int]string{Max_v3: \"max\"}",
		"Fact := 3",
		"return f() + Fact + c.X + c.Alto",
		"func init() { total_v3 = 1 }",
		"package q",
	} {
		if !strings.Contains(out, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, out)
		}
	}
	if strings.Count(out, "\n") != strings.Count(src, "\n") {
		t.Error("el número de líneas cambió")
	}
	// the result type-checks (fmt from GOROOT sources)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", out, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: fuenteImportador(t, fset)}
	if _, err := conf.Check("q", fset, []*ast.File{f}, nil); err != nil {
		t.Fatalf("el resultado no pasa go/types: %v\n%s", err, out)
	}
}

func TestRenombrarConErrores(t *testing.T) {
	src := "package p\n\nfunc Doble(x int) int { return x * 2 + noExiste }\n\nfunc Usa() int { return Doble(3) }\n"
	out, _, err := Renombrar(src, "_v0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "func Doble_v0(") || !strings.Contains(out, "return Doble_v0(3)") || !strings.Contains(out, "noExiste") {
		t.Errorf("renombrado incorrecto:\n%s", out)
	}
}

func TestCambiarPaquete(t *testing.T) {
	out, err := CambiarPaquete("// comentario\npackage main\n\nfunc main() {}\n", "solucion")
	if err != nil {
		t.Fatal(err)
	}
	if out != "// comentario\npackage solucion\n\nfunc main() {}\n" {
		t.Errorf("CambiarPaquete = %q", out)
	}
}

func TestSoporte(t *testing.T) {
	s := Soporte("solucion", nucleo.OpcionesInstr{}, 0)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "s.go", s, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: fuenteImportador(t, fset)}
	if _, err := conf.Check("solucion", fset, []*ast.File{f}, nil); err != nil {
		t.Fatalf("el soporte no pasa go/types: %v", err)
	}
	if !strings.Contains(s, "nyx__f          int64 = 4611686018427387904") {
		t.Errorf("sin combustible el contador debe empezar muy alto:\n%s", s)
	}
}
