package arenero_test

import (
	"runtime"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// Regression: a field named like its own type (Punto Punto) is an ordinary field, not an embedded one; real
// embedded fields (also through a pointer) and anonymous structs with embedded fields still work.
func TestCampoConNombreDeSuTipo(t *testing.T) {
	a := conGo(t)
	punto := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Punto", Definido: true, Campos: []nucleo.Campo{
		{Nombre: "X", Tipo: nucleo.TInt}, {Nombre: "Y", Tipo: nucleo.TInt}}}
	seg := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Seg", Definido: true, Campos: []nucleo.Campo{
		{Nombre: "Punto", Tipo: punto}, {Nombre: "Fin", Tipo: punto}}}
	src := "package solucion\n\ntype Punto struct{ X, Y int }\n\ntype Seg struct {\n\tPunto Punto\n\tFin   Punto\n}\n\nfunc Largo(s Seg) int { return s.Fin.X - s.Punto.X + s.Fin.Y - s.Punto.Y }\n"
	emb := "package solucion\n\ntype Punto struct{ X, Y int }\n\ntype Seg struct {\n\tPunto\n\tFin Punto\n}\n\nfunc Largo(s Seg) int { return s.Fin.X - s.X + s.Fin.Y - s.Y }\n"
	f := nucleo.Firma{Nombre: "Largo", Params: []nucleo.Param{{Nombre: "s", Tipo: seg}}, Res: []nucleo.Tipo{nucleo.TInt}}
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{src, emb}, Firma: f})
	caso := nucleo.Caso{Entradas: []nucleo.Valor{nucleo.Estructura{nucleo.Estructura{1, 2}, nucleo.Estructura{4, 6}}}, Esperado: []nucleo.Valor{7}, Expectativa: nucleo.EspUsuario}
	for v, fila := range probar(t, b, []nucleo.Caso{caso}, nucleo.OpcionesProbar{}) {
		if !fila[0].OK {
			t.Errorf("variante %d: %+v", v, fila[0])
		}
	}
	// a pointer-embedded field and an anonymous struct parameter with an embedded field
	conPuntero := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Caja", Definido: true, Campos: []nucleo.Campo{
		{Nombre: "Punto", Tipo: nucleo.PunteroA(punto)}, {Nombre: "Punto2", Tipo: punto}}}
	anon := nucleo.Tipo{Clase: nucleo.CStruct, Campos: []nucleo.Campo{{Nombre: "Punto", Tipo: punto}, {Nombre: "N", Tipo: nucleo.TInt}}}
	src2 := "package solucion\n\ntype Punto struct{ X, Y int }\n\ntype Caja struct {\n\t*Punto\n\tPunto2 Punto\n}\n\nfunc Suma(c Caja, s struct {\n\tPunto\n\tN int\n}) int {\n\treturn c.X + c.Punto2.Y + s.X + s.N\n}\n"
	f2 := nucleo.Firma{Nombre: "Suma", Params: []nucleo.Param{{Nombre: "c", Tipo: conPuntero}, {Nombre: "s", Tipo: anon}}, Res: []nucleo.Tipo{nucleo.TInt}}
	b2 := preparar(t, a, nucleo.Preparacion{Variantes: []string{src2}, Firma: f2})
	caso2 := nucleo.Caso{Entradas: []nucleo.Valor{
		nucleo.Estructura{nucleo.Estructura{1, 2}, nucleo.Estructura{3, 4}},
		nucleo.Estructura{nucleo.Estructura{10, 20}, 100}}, Esperado: []nucleo.Valor{115}, Expectativa: nucleo.EspUsuario}
	if r := probar(t, b2, []nucleo.Caso{caso2}, nucleo.OpcionesProbar{})[0][0]; !r.OK {
		t.Errorf("campo embebido por puntero y struct anónimo: %+v", r)
	}
}

// Regression: a property literal with repeated spaces is checked exactly as written.
func TestPropiedadConEspaciosEnLiterales(t *testing.T) {
	a := conGo(t)
	f := nucleo.Firma{Nombre: "Doble", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TString}}
	props := []nucleo.Propiedad{
		{Nombre: "dobles", Expr: "r0 == \"a  ba  b\""},
		{Nombre: "crudo", Expr: "r0 == `a  b` + `a  b`", Requiere: "e0 ==\n\t\"a  b\""},
	}
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{"package solucion\n\nfunc Doble(s string) string { return s + s }\n"}, Firma: f, Props: props})
	r := probar(t, b, []nucleo.Caso{{Entradas: []nucleo.Valor{"a  b"}}}, nucleo.OpcionesProbar{})[0][0]
	if !r.OK || r.PropFallida != "" || !nucleo.Igual(r.Obtenido[0], "a  ba  b") {
		t.Errorf("%+v", r)
	}
}

// Regression: in program mode, running out of fuel is seen even when a deep stack pushes the panic message
// out of the last 4 KiB of stderr.
func TestProgramaSinCombustibleConPilaHonda(t *testing.T) {
	a := conGo(t)
	src := `package main

func baja(n, a, b, c int, s string) int {
	if n > 0 {
		return baja(n-1, a+1, b+2, c+3, s)
	}
	for {
		a++
	}
}

func main() {
	println(baja(300, 1, 2, 3, "texto"))
}
`
	p, comp, err := a.PrepararPrograma(bg, src, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		t.Fatal(err, comp)
	}
	defer p.Cerrar()
	e, err := p.Correr(bg, nucleo.CasoPrograma{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.ErrSalida) <= 4096 {
		t.Logf("stderr solo tiene %d bytes; la prueba no fuerza el caso", len(e.ErrSalida))
	}
	if !e.SinCombustible || e.Codigo == 0 {
		t.Errorf("SinCombustible=%v código=%d, stderr %d bytes", e.SinCombustible, e.Codigo, len(e.ErrSalida))
	}
}

// Regression: Preparar returns the reasons of a variant left out as unsafe in Compilacion.Errores.
func TestInseguroDiceElMotivo(t *testing.T) {
	a := conGo(t)
	_, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: []string{"package solucion\n\nimport \"os\"\n\nfunc F(n int) int { os.Remove(\"x\"); return n }\n"}, Firma: firmaIntInt("F")})
	if err != nucleo.ErrInseguro || len(comp.Errores) == 0 {
		t.Fatalf("err=%v errores=%+v", err, comp.Errores)
	}
	e := comp.Errores[0]
	if e.Archivo != "v0.go" || e.Linea != 5 || !strings.Contains(e.Msg, "os.Remove") {
		t.Errorf("%+v", e)
	}
	// with one safe variant the build goes on and the unsafe one is still reported
	b, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: []string{
		"package solucion\n\nfunc F(n int) int { return n }\n",
		"package solucion\n\nimport \"os\"\n\nfunc F(n int) int { os.Remove(\"x\"); return n }\n"}, Firma: firmaIntInt("F")})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	if !comp.OK || len(comp.Errores) == 0 || comp.Errores[0].Archivo != "v1.go" {
		t.Errorf("%+v", comp)
	}
}

// Regression (security): a huge result never reaches the parent's memory. The harness sends only its size,
// the case fails with a clear reason, and one Probar keeps at most 16 MiB of results.
func TestResultadoEnorme(t *testing.T) {
	a := conGo(t)
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TString}}
	b := preparar(t, a, nucleo.Preparacion{Variantes: []string{"package solucion\n\nimport \"strings\"\n\nfunc F(n int) string { return strings.Repeat(\"a\", n) }\n"}, Firma: f, Instr: nucleo.InstrNormal})
	var antes, despues runtime.MemStats
	runtime.ReadMemStats(&antes)
	rs := probar(t, b, casosInt(60<<20, 3), nucleo.OpcionesProbar{})[0]
	runtime.ReadMemStats(&despues)
	if crecio := despues.TotalAlloc - antes.TotalAlloc; crecio > 32<<20 {
		t.Errorf("el proceso padre reservó %d MiB por un resultado enorme", crecio>>20)
	}
	r := rs[0]
	if r.OK || r.Obtenido != nil || !r.Ejecutado || !strings.Contains(r.Panico, "demasiado grande") {
		t.Errorf("resultado de 60 MiB: OK=%v Panico=%q, %d valores", r.OK, r.Panico, len(r.Obtenido))
	}
	if !rs[1].OK || !nucleo.Igual(rs[1].Obtenido[0], "aaa") {
		t.Errorf("el caso pequeño: %+v", rs[1])
	}
	// many results just under the per-case limit: the first ones are kept, the rest are refused
	ns := make([]int, 20)
	for i := range ns {
		ns[i] = 900_000
	}
	rs = probar(t, b, casosInt(ns...), nucleo.OpcionesProbar{})[0]
	guardados, rechazados := 0, 0
	for _, r := range rs {
		switch {
		case r.OK && len(r.Obtenido) == 1:
			guardados++
		case !r.OK && r.Obtenido == nil && strings.Contains(r.Panico, "ocupan demasiado"):
			rechazados++
		default:
			t.Errorf("caso inesperado: OK=%v Panico=%q", r.OK, r.Panico)
		}
	}
	if guardados == 0 || rechazados == 0 || guardados*900_000 > 16<<20 {
		t.Errorf("guardados %d, rechazados %d", guardados, rechazados)
	}
}

// Regression: a variant whose package-level code kills the process at start (a panicking init, a bad var
// initializer, os.Exit in init) is left out at Preparar and reported on each of its cases; the healthy
// variants of the same batch run normally.
func TestArranqueQueCaeSoloCulpaAlCulpable(t *testing.T) {
	a := conGo(t)
	vs := []string{
		"package solucion\n\nfunc F(n int) int { return n }\n",
		"package solucion\n\nvar m map[string]int\n\nfunc init() { m[\"a\"] = 1 }\n\nfunc F(n int) int { return n }\n",
		"package solucion\n\nvar base = 10\n\nfunc F(n int) int { return n + base }\n",
		"package solucion\n\nimport \"os\"\n\nfunc init() { os.Exit(0) }\n\nfunc F(n int) int { return n }\n",
		"package solucion\n\nvar tabla = []int{1, 2}\n\nvar malo = sumar(5)\n\nfunc sumar(i int) int { return tabla[i] }\n\nfunc F(n int) int { return n + malo }\n",
	}
	antes := a.Compilaciones()
	b, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: vs, Firma: firmaIntInt("F"), Instr: nucleo.InstrNormal})
	if err != nil {
		t.Fatal(err, comp.Texto)
	}
	defer b.Cerrar()
	t.Logf("compilaciones: %d\n%s", a.Compilaciones()-antes, comp.Texto)
	malos := map[string]nucleo.ErrorGo{}
	for _, e := range comp.Errores {
		malos[e.Archivo] = e
	}
	if len(malos) != 3 || malos["v1.go"].Linea != 5 || !strings.Contains(malos["v1.go"].Msg, "se cae al arrancar") ||
		malos["v4.go"].Linea != 7 || malos["v3.go"].Archivo != "v3.go" {
		t.Errorf("errores: %+v", comp.Errores)
	}
	rs := probar(t, b, casosInt(1, 2), nucleo.OpcionesProbar{})
	for c := 0; c < 2; c++ {
		if !rs[0][c].OK || !nucleo.Igual(rs[0][c].Obtenido[0], c+1) {
			t.Errorf("variante 0 caso %d: %+v", c, rs[0][c])
		}
		if !rs[2][c].OK || !nucleo.Igual(rs[2][c].Obtenido[0], c+11) {
			t.Errorf("variante 2 caso %d: %+v", c, rs[2][c])
		}
		if r := rs[1][c]; r.OK || !r.Caida || !strings.Contains(r.Panico, "assignment to entry in nil map") || strings.Contains(r.Panico, "_v1") {
			t.Errorf("variante 1 caso %d: %+v", c, r)
		}
		if r := rs[3][c]; r.OK || !r.Caida || !strings.Contains(r.Panico, "os.Exit") {
			t.Errorf("variante 3 caso %d: %+v", c, r)
		}
		if r := rs[4][c]; r.OK || !r.Caida || !strings.Contains(r.Panico, "index out of range") {
			t.Errorf("variante 4 caso %d: %+v", c, r)
		}
	}
	// a batch whose start-up code is healthy is not probed and not rebuilt
	antes = a.Compilaciones()
	b2 := preparar(t, a, nucleo.Preparacion{Variantes: []string{vs[0], vs[2]}, Firma: firmaIntInt("F")})
	if n := a.Compilaciones() - antes; n != 1 {
		t.Errorf("hubo %d compilaciones", n)
	}
	if r := probar(t, b2, casosInt(1), nucleo.OpcionesProbar{}); !r[0][0].OK || !r[1][0].OK {
		t.Errorf("%+v", r)
	}
}

// Regression: when start-up code only fails during Probar (here: from its second run on), the culprit is
// named on its own cases, and the other variants' cases say which variant kept them from running.
func TestArranqueQueCaeEnProbar(t *testing.T) {
	a := conGo(t)
	vs := []string{
		"package solucion\n\nfunc F(n int) int { return n }\n",
		"package solucion\n\nimport \"os\"\n\nfunc init() {\n\tif _, err := os.Stat(\"marca\"); err == nil {\n\t\tpanic(\"segunda vez\")\n\t}\n\tos.WriteFile(\"marca\", nil, 0o600)\n}\n\nfunc F(n int) int { return n }\n",
	}
	b, comp, err := a.Preparar(bg, nucleo.Preparacion{Variantes: vs, Firma: firmaIntInt("F"), Permiso: nucleo.PermisoUsuario})
	if err != nil || len(comp.Errores) != 0 {
		t.Fatal(err, comp)
	}
	defer b.Cerrar()
	rs := probar(t, b, casosInt(1, 2), nucleo.OpcionesProbar{})
	for c := 0; c < 2; c++ {
		if r := rs[1][c]; !r.Caida || !strings.HasPrefix(r.Panico, "panic: segunda vez") {
			t.Errorf("variante 1 caso %d: %+v", c, r)
		}
		if r := rs[0][c]; !r.Caida || !strings.Contains(r.Panico, "la variante 1 se cae al arrancar") {
			t.Errorf("variante 0 caso %d: %+v", c, r)
		}
	}
	rs = probar(t, b, casosInt(1), nucleo.OpcionesProbar{Variantes: []int{0}})
	if r := rs[0][0]; !r.Caida || !strings.Contains(r.Panico, "la variante 1 se cae al arrancar") || rs[1][0].Ejecutado {
		t.Errorf("solo la variante 0: %+v / %+v", r, rs[1][0])
	}
}
