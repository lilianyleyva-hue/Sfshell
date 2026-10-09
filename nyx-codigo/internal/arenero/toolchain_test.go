package arenero_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/nucleo"
)

var bg = context.Background()

func firmaIntInt(nombre string) nucleo.Firma {
	return nucleo.Firma{Nombre: nombre, Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
}

func casosInt(ns ...int) []nucleo.Caso {
	out := make([]nucleo.Caso, len(ns))
	for i, n := range ns {
		out[i] = nucleo.Caso{Entradas: []nucleo.Valor{n}, Origen: nucleo.OrigenSonda}
	}
	return out
}

func preparar(t *testing.T, a *arenero.Arenero, p nucleo.Preparacion) nucleo.Binario {
	t.Helper()
	b, comp, err := a.Preparar(bg, p)
	if err != nil {
		t.Fatalf("Preparar: %v\n%s\n%+v", err, comp.Texto, comp.Errores)
	}
	t.Cleanup(func() { b.Cerrar() })
	return b
}

func probar(t *testing.T, b nucleo.Binario, casos []nucleo.Caso, op nucleo.OpcionesProbar) [][]nucleo.ResultadoCaso {
	t.Helper()
	rs, err := b.Probar(bg, casos, op)
	if err != nil {
		t.Fatalf("Probar: %v", err)
	}
	return rs
}

func TestHolaMundoCompila(t *testing.T) {
	a := conGo(t)
	src := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hola, mundo\")\n}\n"
	a.Compilar(bg, src) // warm
	inicio := time.Now()
	c := a.Compilar(bg, src)
	if !c.OK {
		t.Fatalf("no compila: %+v", c)
	}
	if d := time.Since(inicio); d > 3*time.Second {
		t.Errorf("con la caché caliente tardó %v (más de 3 s)", d)
	}
	lib := a.Compilar(bg, "package solucion\n\nfunc Doble(x int) int { return 2 * x }\n")
	if !lib.OK {
		t.Errorf("una biblioteca debe compilar: %+v", lib)
	}
}

func TestErrorDeSintaxis(t *testing.T) {
	a := conGo(t)
	c := a.Compilar(bg, "package main\n\nfunc main() {\n\tx := 3 +\n}\n")
	if c.OK || len(c.Errores) == 0 {
		t.Fatalf("debería fallar: %+v", c)
	}
	e := c.Errores[0]
	if e.Archivo != "x.go" || e.Linea != 5 || e.Col != 1 {
		t.Errorf("posición = %s:%d:%d (%s), esperaba x.go:5:1", e.Archivo, e.Linea, e.Col, e.Msg)
	}
	if !strings.Contains(c.Texto, "x.go:5:1") {
		t.Errorf("el texto debe mostrar x.go:5:1: %q", c.Texto)
	}
	c = a.Compilar(bg, "package solucion\n\nfunc F() int {\n\treturn \"x\"\n}\n")
	if c.OK || len(c.Errores) != 1 || c.Errores[0].Linea != 4 {
		t.Errorf("error de tipos mal leído: %+v", c)
	}
}

func TestVet(t *testing.T) {
	a := conGo(t)
	c := a.Vet(bg, "package main\n\nimport \"fmt\"\n\nfunc main() {\n\ts := \"hola\"\n\tfmt.Printf(\"%d\\n\", s)\n}\n")
	if c.OK || len(c.Errores) != 1 || c.Errores[0].Linea != 7 || !strings.Contains(c.Errores[0].Msg, "wrong type") {
		t.Errorf("vet debería avisar del formato: %+v", c)
	}
	if c := a.Vet(bg, "package solucion\n\nfunc F(x int) int { return x + 1 }\n"); !c.OK {
		t.Errorf("código limpio: %+v", c)
	}
}

func TestVigilanteYRecuperacion(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc F(n int) int {\n\tif n == 0 {\n\t\tfor {\n\t\t}\n\t}\n\treturn n * 2\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F")}) // no fuel
	inicio := time.Now()
	rs := probar(t, b, casosInt(1, 0, 2, 3), nucleo.OpcionesProbar{TiempoCaso: 500 * time.Millisecond})
	if d := time.Since(inicio); d > 500*time.Millisecond+time.Second+500*time.Millisecond {
		t.Errorf("tardó %v", d)
	}
	r := rs[0]
	if !r[1].Agotado || r[1].OK || !r[1].Ejecutado {
		t.Errorf("for {} debe quedar Agotado: %+v", r[1])
	}
	for _, c := range []int{0, 2, 3} {
		if !r[c].OK || !nucleo.Igual(r[c].Obtenido[0], 2*[]int{1, 0, 2, 3}[c]) {
			t.Errorf("el caso %d debería haberse ejecutado bien: %+v", c, r[c])
		}
	}
}

func TestSinMemoria(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc F(n int) int {\n\tif n == 0 {\n\t\tb := make([]byte, 2<<30)\n\t\tb[len(b)-1] = 1\n\t\treturn len(b)\n\t}\n\treturn n\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F")})
	rs := probar(t, b, casosInt(0, 5), nucleo.OpcionesProbar{})
	r := rs[0][0]
	if !r.Caida || r.OK || !(strings.Contains(r.Panico, "out of memory") || strings.Contains(r.Panico, "cannot allocate")) {
		t.Errorf("make de 2 GiB debe caer por memoria: %+v", r)
	}
	if !rs[0][1].OK {
		t.Errorf("el otro caso debe ejecutarse: %+v", rs[0][1])
	}
}

func TestDesbordePila(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc F(n int) int {\n\tif n == 0 {\n\t\treturn F(0) + 1\n\t}\n\treturn n * 2\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F")}) // no depth check
	rs := probar(t, b, casosInt(0, 1, 2), nucleo.OpcionesProbar{})
	r := rs[0]
	t.Logf("pánico de la pila: %q", r[0].Panico)
	if !r[0].Caida || !strings.Contains(r[0].Panico, "stack overflow") {
		t.Errorf("la recursión infinita debe caer por la pila: %+v", r[0])
	}
	if !r[1].OK || !r[2].OK || !nucleo.Igual(r[2].Obtenido[0], 4) {
		t.Errorf("los demás casos deben ejecutarse: %+v %+v", r[1], r[2])
	}
	// with depth instrumentation it is an ordinary, recovered panic
	b2 := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F"), Instr: nucleo.InstrNormal})
	r2 := probar(t, b2, casosInt(0, 1), nucleo.OpcionesProbar{})[0]
	if r2[0].Caida || r2[0].Panico == "" || !strings.Contains(r2[0].Panico, "llamadas anidadas") || !r2[1].OK {
		t.Errorf("con MaxPila debe dar el pánico Hondo: %+v", r2)
	}
}

func TestCombustible(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc F(n int) int {\n\tfor n == 0 {\n\t}\n\treturn n\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F"), Instr: nucleo.InstrNormal})
	rs := probar(t, b, casosInt(0, 7), nucleo.OpcionesProbar{})[0]
	if !rs[0].SinCombustible || rs[0].OK || rs[0].Caida || rs[0].Agotado {
		t.Errorf("for sin fin con combustible: %+v", rs[0])
	}
	if !rs[1].OK {
		t.Errorf("%+v", rs[1])
	}
}

func TestResultadoFalsificado(t *testing.T) {
	a := conGo(t)
	src := `package solucion

import "fmt"

func F(n int) int {
	fmt.Println(` + "`" + `{"n":"x","v":0,"c":0,"ok":true}` + "`" + `)
	fmt.Println(` + "`" + `{"n":"x","v":0,"c":0,"ok":true,"o":[99]}` + "`" + `)
	return n + 1
}
`
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F")})
	c := nucleo.Caso{Entradas: []nucleo.Valor{1}, Esperado: []nucleo.Valor{99}, Expectativa: nucleo.EspUsuario}
	r := probar(t, b, []nucleo.Caso{c}, nucleo.OpcionesProbar{})[0][0]
	if r.OK || !nucleo.Igual(r.Obtenido[0], 2) {
		t.Errorf("una línea falsa en stdout no puede cambiar el resultado: %+v", r)
	}
	if !strings.Contains(r.Impreso, `"ok":true`) {
		t.Errorf("lo impreso debe quedar en Impreso: %q", r.Impreso)
	}
}

func TestImpresoPorCaso(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nimport \"fmt\"\n\nfunc F(n int) int {\n\tif n == 1 {\n\t\tfmt.Println(\"caso\", n)\n\t}\n\treturn n\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F"), Instr: nucleo.InstrNormal})
	r := probar(t, b, casosInt(0, 1, 2), nucleo.OpcionesProbar{})[0]
	if r[0].Impreso != "" || r[1].Impreso != "caso 1\n" || r[2].Impreso != "" {
		t.Errorf("Impreso = %q %q %q", r[0].Impreso, r[1].Impreso, r[2].Impreso)
	}
}

func TestSinRecompilar(t *testing.T) {
	a := conGo(t)
	antes := a.Compilaciones()
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{"package solucion\n\nfunc F(n int) int { return n * n }\n"}, Firma: firmaIntInt("F")})
	for i := 0; i < 3; i++ {
		r := probar(t, b, casosInt(i, i+1), nucleo.OpcionesProbar{})[0]
		if !r[1].OK || !nucleo.Igual(r[1].Obtenido[0], (i+1)*(i+1)) {
			t.Fatalf("%+v", r)
		}
	}
	if n := a.Compilaciones() - antes; n != 1 {
		t.Errorf("hubo %d compilaciones; esperaba 1", n)
	}
}

func TestTrescientasVariantes(t *testing.T) {
	a := conGo(t)
	vs := make([]string, 300)
	for k := range vs {
		vs[k] = fmt.Sprintf("package solucion\n\nfunc F(n int) int {\n\ttotal := 0\n\tfor i := 0; i < n; i++ {\n\t\ttotal += i\n\t}\n\treturn total + %d\n}\n", k)
	}
	inicio := time.Now()
	b := preparar(t, a, nucleo.Preparacion{Variantes: vs, Firma: firmaIntInt("F"), Instr: nucleo.InstrNormal})
	tc := time.Since(inicio)
	ns := make([]int, 300)
	for i := range ns {
		ns[i] = i % 50
	}
	inicio = time.Now()
	rs := probar(t, b, casosInt(ns...), nucleo.OpcionesProbar{})
	tp := time.Since(inicio)
	t.Logf("300 variantes: compilar %v, 300×300 casos %v", tc, tp)
	if len(rs) != 300 || b.NumVariantes() != 300 {
		t.Fatalf("hay %d filas", len(rs))
	}
	for k, fila := range rs {
		for c, r := range fila {
			n := ns[c]
			if !r.OK || !nucleo.Igual(r.Obtenido[0], n*(n-1)/2+k) {
				t.Fatalf("variante %d caso %d: %+v", k, c, r)
			}
		}
	}
	if tp > 20*time.Second {
		t.Errorf("90 000 llamadas tardaron %v", tp)
	}
}

func TestVarianteRotaAislada(t *testing.T) {
	a := conGo(t)
	vs := make([]string, 50)
	for k := range vs {
		vs[k] = fmt.Sprintf("package solucion\n\nfunc F(n int) int { return n + %d }\n", k)
	}
	vs[17] = "package solucion\n\nfunc F(n int) int {\n\treturn \"diecisiete\"\n}\n"
	vs[31] = "package solucion\n\nfunc F(n string) int { return len(n) }\n" // does not fit the signature
	b, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: vs, Firma: firmaIntInt("F")})
	if err != nil {
		t.Fatalf("%v %+v", err, comp)
	}
	defer b.Cerrar()
	if !comp.OK {
		t.Errorf("con 48 variantes buenas la compilación es correcta: %+v", comp)
	}
	malos := map[string]bool{}
	for _, e := range comp.Errores {
		malos[e.Archivo] = true
		if e.Archivo == "v17.go" && e.Linea != 4 {
			t.Errorf("el error de v17 debe estar en la línea 4: %+v", e)
		}
	}
	if !malos["v17.go"] || !malos["v31.go"] || len(malos) != 2 {
		t.Errorf("errores = %+v", comp.Errores)
	}
	rs := probar(t, b, casosInt(1, 2), nucleo.OpcionesProbar{})
	for k, fila := range rs {
		roto := k == 17 || k == 31
		for c, r := range fila {
			if roto && (r.Ejecutado || r.OK) {
				t.Errorf("la variante %d no compila y no debe ejecutarse: %+v", k, r)
			}
			if !roto && (!r.OK || !nucleo.Igual(r.Obtenido[0], c+1+k)) {
				t.Errorf("variante %d caso %d: %+v", k, c, r)
			}
		}
	}
	if b.Lineas(17) != nil {
		t.Error("una variante rota no tiene líneas")
	}
}

func TestNingunaCompila(t *testing.T) {
	a := conGo(t)
	_, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: []string{"package solucion\n\nfunc F(n int) int { return x }\n"}, Firma: firmaIntInt("F")})
	if err != nucleo.ErrNoCompila || comp.OK || len(comp.Errores) == 0 || comp.Errores[0].Archivo != "v0.go" || comp.Errores[0].Linea != 3 {
		t.Errorf("err=%v comp=%+v", err, comp)
	}
	_, _, err = a.Preparar(bg, nucleo.Preparacion{Variantes: []string{"package solucion\n\nimport \"os\"\n\nfunc F(n int) int { os.Remove(\"x\"); return n }\n"}, Firma: firmaIntInt("F")})
	if err != nucleo.ErrInseguro {
		t.Errorf("código inseguro: %v", err)
	}
	_, _, err = a.Preparar(bg, nucleo.Preparacion{Variantes: []string{"package solucion\n\nfunc F() {}\n"}, Firma: nucleo.Firma{Nombre: "F"}})
	if err != nucleo.ErrNoProbable {
		t.Errorf("firma sin resultados: %v", err)
	}
}

func TestTiposVariados(t *testing.T) {
	a := conGo(t)
	punto := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Punto", Definido: true, Campos: []nucleo.Campo{
		{Nombre: "X", Tipo: nucleo.TInt}, {Nombre: "nombre", Tipo: nucleo.TString}}}
	contador := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Contador", Definido: true, Campos: []nucleo.Campo{{Nombre: "n", Tipo: nucleo.TInt}}}
	src := `package solucion

import (
	"errors"
	"strings"
)

type Punto struct {
	X      int
	nombre string
}

type Contador struct{ n int }

func Mover(p Punto, d int) Punto { p.X += d; p.nombre += "!"; return p }

func Agrupar(xs []string) map[string][]int {
	m := map[string][]int{}
	for i, x := range xs {
		k := strings.ToLower(x[:1])
		m[k] = append(m[k], i)
	}
	return m
}

func Dividir(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("no se puede dividir entre cero")
	}
	return a / b, nil
}

func (c *Contador) Sumar(k int) int { c.n += k; return c.n }

func Max(xs ...int) int {
	m := 0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}
`
	type prueba struct {
		f     nucleo.Firma
		casos []nucleo.Caso
	}
	esp := func(e []nucleo.Valor, r ...nucleo.Valor) nucleo.Caso {
		return nucleo.Caso{Entradas: e, Esperado: r, Expectativa: nucleo.EspUsuario}
	}
	pruebas := []prueba{
		{nucleo.Firma{Nombre: "Mover", Params: []nucleo.Param{{Nombre: "p", Tipo: punto}, {Nombre: "d", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{punto}},
			[]nucleo.Caso{esp([]nucleo.Valor{nucleo.Estructura{1, "a"}, 2}, nucleo.Estructura{3, "a!"})}},
		{nucleo.Firma{Nombre: "Agrupar", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TString)}}, Res: []nucleo.Tipo{nucleo.MapaDe(nucleo.TString, nucleo.ListaDe(nucleo.TInt))}},
			[]nucleo.Caso{esp([]nucleo.Valor{[]nucleo.Valor{"Ana", "bea", "alba"}}, nucleo.Mapa{{K: "a", V: []nucleo.Valor{0, 2}}, {K: "b", V: []nucleo.Valor{1}}})}},
		{nucleo.Firma{Nombre: "Dividir", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt, nucleo.TError}},
			[]nucleo.Caso{esp([]nucleo.Valor{7, 2}, 3, nil), esp([]nucleo.Valor{1, 0}, 0, nucleo.ErrorV("cualquier mensaje"))}},
		{nucleo.Firma{Nombre: "Sumar", Receptor: &nucleo.Param{Nombre: "c", Tipo: nucleo.PunteroA(contador)}, Params: []nucleo.Param{{Nombre: "k", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}},
			[]nucleo.Caso{esp([]nucleo.Valor{nucleo.Estructura{5}, 2}, 7)}},
		{nucleo.Firma{Nombre: "Max", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Variadica: true, Res: []nucleo.Tipo{nucleo.TInt}},
			[]nucleo.Caso{esp([]nucleo.Valor{[]nucleo.Valor{3, 9, 2}}, 9), esp([]nucleo.Valor{[]nucleo.Valor(nil)}, 0)}},
	}
	for _, p := range pruebas {
		t.Run(p.f.Nombre, func(t *testing.T) {
			b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: p.f, Instr: nucleo.InstrNormal})
			for c, r := range probar(t, b, p.casos, nucleo.OpcionesProbar{})[0] {
				if !r.OK {
					t.Errorf("caso %d: %+v", c, r)
				}
			}
		})
	}
	// a nil pointer receiver is an ordinary recovered panic
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: pruebas[3].f})
	r := probar(t, b, []nucleo.Caso{{Entradas: []nucleo.Valor{nil, 1}}}, nucleo.OpcionesProbar{})[0][0]
	if r.OK || r.Caida || !strings.Contains(r.Panico, "nil pointer") {
		t.Errorf("receptor nil: %+v", r)
	}
	// the error message is reported even though errors compare equal
	b = preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: pruebas[2].f})
	r = probar(t, b, casosDos(1, 0), nucleo.OpcionesProbar{})[0][0]
	if !r.OK || r.Obtenido[1] != nucleo.ErrorV("no se puede dividir entre cero") {
		t.Errorf("error: %+v", r)
	}
}

func casosDos(a, b int) []nucleo.Caso {
	return []nucleo.Caso{{Entradas: []nucleo.Valor{a, b}}}
}

func TestPropiedades(t *testing.T) {
	a := conGo(t)
	f := nucleo.Firma{Nombre: "Ordenar", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.ListaDe(nucleo.TInt)}}
	bien := "package solucion\n\nimport \"sort\"\n\nfunc Ordenar(xs []int) []int {\n\tc := append([]int(nil), xs...)\n\tsort.Ints(c)\n\treturn c\n}\n"
	identidad := "package solucion\n\nfunc Ordenar(xs []int) []int { return xs }\n"
	mutadora := "package solucion\n\nimport \"sort\"\n\nfunc Ordenar(xs []int) []int {\n\tsort.Ints(xs)\n\treturn xs[:len(xs)/2]\n}\n"
	props := []nucleo.Propiedad{
		{Nombre: "ordenada", Expr: "nyx__Ordenada(r0) && nyx__EsPermutacion(e0, r0)"},
		{Nombre: "no compila", Expr: "r0 + 1 > 0"},
		{Nombre: "con requisito", Expr: "nyx__Contiene(e0, r0[0])", Requiere: "len(e0) > 0"},
		{Nombre: "llama a F", Expr: "nyx__Igual(F(r0), r0)"},
	}
	b, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: []string{bien, identidad, mutadora}, Firma: f, Props: props})
	if err != nil {
		t.Fatal(err, comp.Texto)
	}
	defer b.Cerrar()
	if !comp.OK || !strings.Contains(comp.Texto, "no compila") {
		t.Errorf("la propiedad que no compila se quita con una nota: %+v", comp)
	}
	casos := []nucleo.Caso{
		{Entradas: []nucleo.Valor{[]nucleo.Valor{3, 1, 2}}},
		{Entradas: []nucleo.Valor{[]nucleo.Valor(nil)}},
	}
	rs := probar(t, b, casos, nucleo.OpcionesProbar{})
	if !rs[0][0].OK || !rs[0][1].OK {
		t.Errorf("la buena: %+v", rs[0])
	}
	if rs[1][0].OK || rs[1][0].PropFallida != "ordenada" || !rs[1][1].OK {
		t.Errorf("la identidad debe fallar «ordenada» en [3 1 2]: %+v", rs[1])
	}
	if rs[2][0].OK || rs[2][0].PropFallida != "ordenada" {
		t.Errorf("la que muta su entrada se compara con una copia: %+v", rs[2][0])
	}
}

func TestCoberturaYVariables(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc Signo(n int) string {\n\ts := \"\"\n\tif n > 0 {\n\t\ts = \"positivo\"\n\t} else {\n\t\ts = \"no positivo\"\n\t}\n\treturn s\n}\n"
	f := nucleo.Firma{Nombre: "Signo", Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TString}}
	instr := nucleo.OpcionesInstr{Combustible: 1000, Cobertura: true, Variables: true}
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: f, Instr: instr})
	lineas := b.Lineas(0)
	if len(lineas) != 5 {
		t.Fatalf("Lineas = %v", lineas)
	}
	rs := probar(t, b, casosInt(5, 0), nucleo.OpcionesProbar{})[0]
	cubiertas := func(r nucleo.ResultadoCaso) map[int]bool {
		m := map[int]bool{}
		for _, id := range r.Cubiertas {
			m[lineas[id]] = true
		}
		return m
	}
	c5, c0 := cubiertas(rs[0]), cubiertas(rs[1])
	if !c5[6] || c5[8] || !c0[8] || c0[6] || !c0[10] {
		t.Errorf("cobertura 5: %v, 0: %v", c5, c0)
	}
	ev := rs[1].Eventos
	if len(ev) != 2 || ev[0].Linea != 4 || ev[0].Var != "s" || ev[1].Linea != 8 || ev[1].Valor != "no positivo" {
		t.Errorf("eventos: %+v", ev)
	}
}

func TestNoDeterminista(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nvar llamadas int\n\nfunc F(n int) int {\n\tllamadas++\n\treturn n + llamadas\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F"), Instr: nucleo.InstrNormal})
	r := probar(t, b, casosInt(1), nucleo.OpcionesProbar{Repetir: 2})[0][0]
	if !r.NoDeterminista || r.OK {
		t.Errorf("%+v", r)
	}
	b2 := preparar(t, a, nucleo.Preparacion{Variantes: []string{"package solucion\n\nfunc F(n int) int { return n }\n"}, Firma: firmaIntInt("F")})
	if r := probar(t, b2, casosInt(1), nucleo.OpcionesProbar{Repetir: 3})[0][0]; !r.OK || r.NoDeterminista {
		t.Errorf("%+v", r)
	}
}

func TestPararAlFallar(t *testing.T) {
	a := conGo(t)
	vs := []string{
		"package solucion\n\nfunc F(n int) int { return n }\n",
		"package solucion\n\nfunc F(n int) int { return -n }\n",
		"package solucion\n\nfunc F(n int) int { if n == 2 { panic(\"dos\") }; return n }\n",
	}
	b := preparar(t, a, nucleo.Preparacion{Variantes: vs, Firma: firmaIntInt("F")})
	casos := make([]nucleo.Caso, 4)
	for i := range casos {
		casos[i] = nucleo.Caso{Entradas: []nucleo.Valor{i + 1}, Esperado: []nucleo.Valor{i + 1}, Expectativa: nucleo.EspUsuario}
	}
	rs := probar(t, b, casos, nucleo.OpcionesProbar{PararAlFallar: true, Variantes: []int{1, 2}})
	if rs[0][0].Ejecutado {
		t.Error("la variante 0 no se pidió")
	}
	if !rs[1][0].Ejecutado || rs[1][0].OK || rs[1][1].Ejecutado {
		t.Errorf("la variante 1 para en su primer fallo: %+v", rs[1])
	}
	if !rs[2][0].OK || rs[2][1].OK || rs[2][1].Panico != "dos" || rs[2][2].Ejecutado {
		t.Errorf("la variante 2 para en el pánico: %+v", rs[2])
	}
}

func TestInitQueCae(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nvar tabla = []int{1, 2}\n\nvar malo = tabla[5]\n\nfunc F(n int) int { return n + malo }\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F")})
	rs := probar(t, b, casosInt(1, 2), nucleo.OpcionesProbar{})[0]
	for _, r := range rs {
		if !r.Caida || !strings.Contains(r.Panico, "index out of range") {
			t.Errorf("%+v", r)
		}
	}
}

func TestProgramaSalidaRecortada(t *testing.T) {
	a := conGo(t)
	src := "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Print(strings.Repeat(\"x\", 2<<20))\n}\n"
	p, comp, err := a.PrepararPrograma(bg, src, nucleo.OpcionesInstr{}, nucleo.PermisoAuto)
	if err != nil {
		t.Fatal(err, comp)
	}
	defer p.Cerrar()
	e, err := p.Correr(bg, nucleo.CasoPrograma{})
	if err != nil {
		t.Fatal(err)
	}
	if !e.Recortado || len(e.Salida) != 1<<20 || e.Codigo != 0 {
		t.Errorf("salida %d bytes, recortado %v, código %d", len(e.Salida), e.Recortado, e.Codigo)
	}
}

func TestProgramaEntradaYCombustible(t *testing.T) {
	a := conGo(t)
	src := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	suma := 0
	for sc.Scan() {
		n, _ := strconv.Atoi(sc.Text())
		suma += n
	}
	fmt.Println("La suma es", suma, len(os.Args))
	if suma == 0 {
		for {
		}
	}
}
`
	p, comp, err := a.PrepararPrograma(bg, src, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		t.Fatal(err, comp)
	}
	defer p.Cerrar()
	e, err := p.Correr(bg, nucleo.CasoPrograma{Entrada: "2\n3\n", Args: []string{"a", "b c"}})
	if err != nil {
		t.Fatal(err)
	}
	if e.Salida != "La suma es 5 3\n" || e.Codigo != 0 || e.SinCombustible {
		t.Errorf("%+v", e)
	}
	e, err = p.Correr(bg, nucleo.CasoPrograma{Entrada: ""})
	if err != nil {
		t.Fatal(err)
	}
	if !e.SinCombustible || e.Codigo == 0 {
		t.Errorf("el bucle sin fin debe quedarse sin combustible: %+v", e)
	}
	if _, _, err := a.PrepararPrograma(bg, "package main\n\nimport \"os\"\n\nfunc main() { os.Remove(\"x\") }\n", nucleo.InstrNormal, nucleo.PermisoAuto); err != nucleo.ErrInseguro {
		t.Errorf("os.Remove en automático: %v", err)
	}
	if _, comp, err := a.PrepararPrograma(bg, "package main\n\nfunc main() { x }\n", nucleo.InstrNormal, nucleo.PermisoAuto); err != nucleo.ErrNoCompila || len(comp.Errores) == 0 || comp.Errores[0].Archivo != "x.go" {
		t.Errorf("no compila: %v %+v", err, comp)
	}
}

func TestProgramaTiempoDePared(t *testing.T) {
	conGo(t)
	b, err := arenero.Nuevo(arenero.Config{DirDatos: t.TempDir(), DirCache: cacheCompartida(), TEjecutar: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	src := "package main\n\nimport \"time\"\n\nfunc main() {\n\ttime.Sleep(time.Minute)\n}\n"
	p, comp, err := b.PrepararPrograma(bg, src, nucleo.OpcionesInstr{}, nucleo.PermisoUsuario)
	if err != nil {
		t.Fatal(err, comp)
	}
	defer p.Cerrar()
	inicio := time.Now()
	e, err := p.Correr(bg, nucleo.CasoPrograma{})
	if err != nil {
		t.Fatal(err)
	}
	if !e.Agotado || e.Senal != "SIGKILL" || time.Since(inicio) > 3*time.Second {
		t.Errorf("%+v (%v)", e, time.Since(inicio))
	}
}

func TestAislamiento(t *testing.T) {
	a := conGo(t)
	src := `package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

func main() {
	dir, _ := os.Getwd()
	fmt.Println("HOME", os.Getenv("HOME") == dir, os.Getenv("HOME") != "")
	fmt.Println("PATH", os.Getenv("PATH") == "")
	var l syscall.Rlimit
	syscall.Getrlimit(syscall.RLIMIT_DATA, &l)
	fmt.Println("DATA", l.Cur)
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &l)
	fmt.Println("NOFILE", l.Cur)
	syscall.Getrlimit(syscall.RLIMIT_FSIZE, &l)
	fmt.Println("FSIZE", l.Cur)
	_, err := net.DialTimeout("tcp", "1.1.1.1:80", 2*time.Second)
	fmt.Println("RED", err != nil)
}
`
	if vs := a.Revisar(src, nucleo.PermisoAuto); len(vs) == 0 {
		t.Error("net y syscall no se permiten en automático")
	}
	p, comp, err := a.PrepararPrograma(bg, src, nucleo.OpcionesInstr{}, nucleo.PermisoUsuario)
	if err != nil {
		t.Fatal(err, comp)
	}
	defer p.Cerrar()
	e, err := p.Correr(bg, nucleo.CasoPrograma{})
	if err != nil {
		t.Fatal(err)
	}
	salida := e.Salida
	for _, quiero := range []string{"HOME true true", "PATH true", "DATA " + strconv.Itoa(512<<20), "NOFILE 64", "FSIZE " + strconv.Itoa(1<<20)} {
		if !strings.Contains(salida, quiero+"\n") {
			t.Errorf("falta %q en la salida:\n%s\n%s", quiero, salida, e.ErrSalida)
		}
	}
	if a.Estado().RedAislada {
		if !strings.Contains(salida, "RED true") {
			t.Errorf("con espacios de nombres, net.Dial debe fallar:\n%s", salida)
		}
	} else {
		t.Log("sin espacios de nombres en este sistema: no compruebo la red")
	}
}

func TestEstadoYCalentar(t *testing.T) {
	a := conGo(t)
	e := a.Estado()
	if !e.GoOK || !strings.HasPrefix(e.GoVersion, "go1.") || e.Nivel == "sin Go" || a.GoRoot() == "" {
		t.Errorf("estado: %+v, goroot %q", e, a.GoRoot())
	}
	if err := a.Calentar(bg); err != nil {
		t.Error(err)
	}
	sin, err := arenero.Nuevo(arenero.Config{DirDatos: t.TempDir(), GoBin: "/no/existe/go"})
	if err != nil {
		t.Fatal(err)
	}
	if e := sin.Estado(); e.GoOK || e.Nivel != "sin Go" {
		t.Errorf("sin Go: %+v", e)
	}
	if _, _, err := sin.Preparar(bg, nucleo.Preparacion{Variantes: []string{"package p"}, Firma: firmaIntInt("F")}); err != nucleo.ErrSinGo {
		t.Errorf("sin Go: %v", err)
	}
}

func TestCancelar(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc F(n int) int {\n\tfor {\n\t}\n}\n"
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src}, Firma: firmaIntInt("F")})
	ctx, cancelar := context.WithTimeout(bg, 300*time.Millisecond)
	defer cancelar()
	inicio := time.Now()
	_, err := b.Probar(ctx, casosInt(1), nucleo.OpcionesProbar{TiempoCaso: time.Minute})
	if err == nil || time.Since(inicio) > 3*time.Second {
		t.Errorf("cancelar debe parar enseguida: %v (%v)", err, time.Since(inicio))
	}
}
