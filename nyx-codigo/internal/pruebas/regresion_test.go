package pruebas

// Regression tests for problems found in review.

import (
	"context"
	"math"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// CompararSalida "numeros": integers are compared exactly, floats within 1e-9.
func TestCompararNumerosEnterosExactos(t *testing.T) {
	casos := []struct {
		obtenida, esperado string
		quiero             bool
	}{
		{"1000000000", "1000000001", false},
		{"5000000000", "5000000003", false},
		{"12345678901234567890", "12345678901234567891", false},
		{"12345678901234567890", "12345678901234567890", true},
		{"-0", "0", true},
		{"007", "7", true},
		{"-12", "12", false},
		{"1000000000", "1000000000.5", false},
		{"5", "5.0", true},
		{"3.5", "3.5000000001", true},
		{"0.1", "0.2", false},
		{"1e20", "1.0000000000001e20", true},
		{"1e20", "1.01e20", false},
	}
	for _, c := range casos {
		got := CompararSalida(c.obtenida, nucleo.CasoPrograma{Esperado: c.esperado, Comparar: "numeros"})
		if got != c.quiero {
			t.Errorf("%q vs %q: %v, quiero %v", c.obtenida, c.esperado, got, c.quiero)
		}
	}
}

// A huge Rango (or TamMax) must not make Casos panic (Int63n overflow).
func TestRangoEnorme(t *testing.T) {
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	for _, r := range []int{math.MaxInt64, math.MaxInt64 / 2, math.MaxInt64/2 + 1, 1 << 62} {
		cs := NuevoGenerador(1).Casos(f, OpcionesCasos{Rango: r, Azar: 50, TamMax: math.MaxInt})
		if len(cs) == 0 {
			t.Fatalf("Rango %d: sin casos", r)
		}
		grande := false
		for _, c := range cs {
			if v := c.Entradas[0].(int); v > 1e12 || v < -1e12 {
				grande = true
			}
		}
		if !grande {
			t.Errorf("Rango %d: ningún valor grande", r)
		}
	}
	// random lists do not grow beyond the cap either
	g := NuevoGenerador(2)
	if v := g.Azar(nucleo.ListaDe(nucleo.TInt), math.MaxInt); len(v.([]nucleo.Valor)) > maxTam {
		t.Fatalf("lista de %d elementos", len(v.([]nucleo.Valor)))
	}
}

// Three huge inputs used to overflow the size sum: no shrinking candidate looked smaller.
func TestReduccionesEnterosEnormes(t *testing.T) {
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}, {Nombre: "c", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	for _, x := range []int{math.MinInt64, math.MaxInt64} {
		c := nucleo.Caso{Entradas: []nucleo.Valor{x, x, x}, Origen: nucleo.OrigenAzar}
		if rs := Reducciones(c, f); len(rs) == 0 {
			t.Fatalf("%d: sin candidatos", x)
		}
		// fails while some input is non-zero: it shrinks to a single 1 or -1
		falla := func(cs []nucleo.Caso) []bool {
			out := make([]bool, len(cs))
			for i, c := range cs {
				for _, v := range c.Entradas {
					if v.(int) != 0 {
						out[i] = true
					}
				}
			}
			return out
		}
		m := Minimizar(context.Background(), c, f, falla, 200)
		if tamanoCaso(m.Entradas) >= tamanoCaso(c.Entradas) {
			t.Fatalf("%d: no se redujo: %v", x, m.Entradas)
		}
	}
}

// Shrunk candidates are not labelled as the user's own example.
func TestReduccionesNoSonDelUsuario(t *testing.T) {
	f := firmaLista()
	c := nucleo.Caso{Entradas: []nucleo.Valor{lista(3, -1, 4)}, Esperado: []nucleo.Valor{2}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario, Nota: "tu ejemplo 2"}
	rs := Reducciones(c, f)
	if len(rs) == 0 {
		t.Fatal("sin candidatos")
	}
	for _, r := range rs {
		if r.Origen != OrigenReducido || r.Nota != "reducido de «tu ejemplo 2»" || r.ConEsperado() {
			t.Fatalf("candidato mal etiquetado: %+v", r)
		}
		// shrinking again keeps the note
		for _, r2 := range Reducciones(r, f) {
			if r2.Nota != r.Nota {
				t.Fatalf("nota %q", r2.Nota)
			}
		}
	}
	cs := Comprobaciones([]nucleo.Caso{rs[0]}, []nucleo.ResultadoCaso{{Ejecutado: true, OK: true}}, nil, 1)
	if len(cs) != 1 || strings.Contains(cs[0].Detalle, "tuyo") || !strings.Contains(cs[0].Detalle, "1 reducido") {
		t.Fatalf("detalle: %+v", cs)
	}
}

// ArchivoTest: a function with no results, and names that clash with the imports.
func TestArchivoTestSinResultadosYNombresChocantes(t *testing.T) {
	sinRes := nucleo.Firma{Nombre: "Saluda", Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}}
	for _, casos := range [][]nucleo.Caso{
		nil,
		{{Entradas: []nucleo.Valor{1}, Esperado: []nucleo.Valor{}, Expectativa: nucleo.EspUsuario}},
		{{Entradas: []nucleo.Valor{2}}},
	} {
		src := ArchivoTest(sinRes, casos, "solucion")
		archivos := map[string]string{"a.go": "package solucion\n\nfunc Saluda(n int) {}\n", "a_test.go": src}
		if errs := revisarTipos(t, archivos); len(errs) > 0 {
			t.Fatalf("no compila: %v\n%s", errs, src)
		}
	}

	nan := nucleo.Firma{Nombre: "Raiz", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TFloat}}, Res: []nucleo.Tipo{nucleo.TFloat}}
	nanCaso := []nucleo.Caso{{Entradas: []nucleo.Valor{-1.0}, Esperado: []nucleo.Valor{math.NaN()}, Expectativa: nucleo.EspUsuario, Nota: "math.Sqrt(-1)"}}
	tipoTesting := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "testing", Definido: true, Campos: []nucleo.Campo{{Nombre: "x", Tipo: nucleo.TInt}}}
	pruebas := []struct {
		nombre, fuente string
		f              nucleo.Firma
		casos          []nucleo.Caso
	}{
		{"math", "package solucion\n\nfunc math(x int) int { return x }\n",
			nucleo.Firma{Nombre: "math", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}},
			[]nucleo.Caso{{Entradas: []nucleo.Valor{1}, Esperado: []nucleo.Valor{1}, Expectativa: nucleo.EspUsuario}}},
		{"reflect", "package solucion\n\nfunc reflect(x int) int { return x }\n",
			nucleo.Firma{Nombre: "reflect", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}},
			[]nucleo.Caso{{Entradas: []nucleo.Valor{1}, Esperado: []nucleo.Valor{1}, Expectativa: nucleo.EspUsuario}}},
		{"testing", "package solucion\n\nfunc testing(x int) int { return x }\n",
			nucleo.Firma{Nombre: "testing", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}},
			[]nucleo.Caso{{Entradas: []nucleo.Valor{1}, Esperado: []nucleo.Valor{1}, Expectativa: nucleo.EspUsuario}, {Entradas: []nucleo.Valor{2}}}},
		{"tipo testing", "package solucion\n\ntype testing struct{ x int }\n\nfunc Uno(p testing) int { return p.x }\n",
			nucleo.Firma{Nombre: "Uno", Params: []nucleo.Param{{Nombre: "p", Tipo: tipoTesting}}, Res: []nucleo.Tipo{nucleo.TInt}},
			[]nucleo.Caso{{Entradas: []nucleo.Valor{nucleo.Estructura{3}}, Esperado: []nucleo.Valor{3}, Expectativa: nucleo.EspUsuario}}},
		{"math con NaN", "package solucion\n\nimport m \"math\"\n\nfunc Raiz(x float64) float64 { return m.Sqrt(x) }\n\nfunc math() {}\n", nan, nanCaso},
		{"NaN", "package solucion\n\nimport \"math\"\n\nfunc Raiz(x float64) float64 { return math.Sqrt(x) }\n", nan, nanCaso},
	}
	for _, p := range pruebas {
		src := ArchivoTest(p.f, p.casos, "solucion")
		if errs := revisarTipos(t, map[string]string{"a.go": p.fuente, "a_test.go": src}); len(errs) > 0 {
			t.Errorf("%s: no compila: %v\n%s", p.nombre, errs, src)
		}
	}
	// the note "math.Sqrt(-1)" inside a string is not renamed
	src := ArchivoTest(nan, nanCaso, "solucion")
	if !strings.Contains(src, `"math.Sqrt(-1)"`) || !strings.Contains(src, "nyxmath.NaN()") {
		t.Fatalf("%s", src)
	}
	// no math import when nothing needs it
	if src := ArchivoTest(firmaLista(), nil, "solucion"); strings.Contains(src, `"math"`) || !strings.Contains(src, "*testing.T") {
		t.Fatalf("import de math innecesario:\n%s", src)
	}
}

func TestRenombrarPaquete(t *testing.T) {
	x, usa := renombrarPaquete(`[]float64{math.NaN(), 1, math.Inf(-1)}`, "math", "nyxmath")
	if !usa || x != `[]float64{nyxmath.NaN(), 1, nyxmath.Inf(-1)}` {
		t.Fatalf("%v %q", usa, x)
	}
	x, usa = renombrarPaquete(`"math.Pi"`, "math", "nyxmath")
	if usa || x != `"math.Pi"` {
		t.Fatalf("%v %q", usa, x)
	}
}
