package pruebas

import (
	"context"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

func lista(xs ...int) []nucleo.Valor {
	out := make([]nucleo.Valor, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func firmaLista() nucleo.Firma {
	return nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
}

func TestMismaSemillaMismosCasos(t *testing.T) {
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{
		{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)},
		{Nombre: "s", Tipo: nucleo.TString},
		{Nombre: "m", Tipo: nucleo.MapaDe(nucleo.TString, nucleo.TFloat)},
	}, Res: []nucleo.Tipo{nucleo.TInt}}
	a := NuevoGenerador(42).Casos(f, OpcionesCasos{})
	b := NuevoGenerador(42).Casos(f, OpcionesCasos{})
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("longitudes %d y %d", len(a), len(b))
	}
	for i := range a {
		if claveEntradas(a[i].Entradas) != claveEntradas(b[i].Entradas) || a[i].Nota != b[i].Nota {
			t.Fatalf("caso %d distinto: %v vs %v", i, a[i].Entradas, b[i].Entradas)
		}
	}
	// calling Casos twice on the same generator also gives the same cases
	g := NuevoGenerador(42)
	c1, c2 := g.Casos(f, OpcionesCasos{}), g.Casos(f, OpcionesCasos{})
	if claveEntradas(c1[len(c1)-1].Entradas) != claveEntradas(c2[len(c2)-1].Entradas) {
		t.Fatal("Casos depende del estado anterior del generador")
	}
	c := NuevoGenerador(43).Casos(f, OpcionesCasos{})
	distintos := false
	for i := range c {
		if i < len(a) && claveEntradas(c[i].Entradas) != claveEntradas(a[i].Entradas) {
			distintos = true
		}
	}
	if !distintos {
		t.Fatal("otra semilla da exactamente los mismos casos")
	}
}

func TestCasosOrigenesYCantidades(t *testing.T) {
	f := firmaLista()
	cs := NuevoGenerador(1).Casos(f, OpcionesCasos{})
	borde, azar := 0, 0
	for _, c := range cs {
		if c.Expectativa != nucleo.EspNinguna || c.Esperado != nil {
			t.Fatalf("un caso generado no debe llevar esperado: %+v", c)
		}
		switch c.Origen {
		case nucleo.OrigenBorde:
			borde++
		case nucleo.OrigenAzar:
			azar++
		default:
			t.Fatalf("origen inesperado %q", c.Origen)
		}
		xs := c.Entradas[0].([]nucleo.Valor)
		if len(xs) > 30 {
			t.Fatalf("lista de %d elementos supera TamMax", len(xs))
		}
		for _, x := range xs {
			if v := x.(int); v > 1000 || v < -1000 {
				t.Fatalf("valor %d fuera de rango", v)
			}
		}
	}
	if borde < 5 || azar < 150 {
		t.Fatalf("borde=%d azar=%d", borde, azar)
	}
	grande := NuevoGenerador(1).Casos(f, OpcionesCasos{Grande: true, Azar: 3, Bordes: 2})
	ult := grande[len(grande)-1]
	if n := len(ult.Entradas[0].([]nucleo.Valor)); n != 10000 {
		t.Fatalf("el caso grande tiene %d elementos", n)
	}
}

func TestRangoAdaptadoALosEjemplos(t *testing.T) {
	f := firmaLista()
	ej := []nucleo.Caso{{Entradas: []nucleo.Valor{lista(1, 2, 3)}, Esperado: []nucleo.Valor{2}, Expectativa: nucleo.EspUsuario}}
	cs := NuevoGenerador(5).Casos(f, OpcionesCasos{Ejemplos: ej, Bordes: -1})
	for _, c := range cs {
		for _, x := range c.Entradas[0].([]nucleo.Valor) {
			if v := x.(int); v > 30 || v < -30 {
				t.Fatalf("con ejemplos pequeños el rango debe ser 30, salió %d", v)
			}
		}
	}
	// strings take the alphabet of the examples
	fs := nucleo.Firma{Nombre: "G", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt}}
	ejs := []nucleo.Caso{{Entradas: []nucleo.Valor{"xyzw"}}}
	vistas := 0
	for _, c := range NuevoGenerador(9).Casos(fs, OpcionesCasos{Ejemplos: ejs, Bordes: -1}) {
		vistas += strings.Count(c.Entradas[0].(string), "w")
	}
	if vistas == 0 {
		t.Fatal("las cadenas al azar no usan las letras de los ejemplos")
	}
}

func TestBordesListaInt(t *testing.T) {
	bs := NuevoGenerador(1).Bordes(nucleo.ListaDe(nucleo.TInt))
	var hayNil, hayVacia, hayUno, hayNeg, hayAsc bool
	for _, b := range bs {
		xs := b.([]nucleo.Valor)
		switch {
		case xs == nil:
			hayNil = true
		case len(xs) == 0:
			hayVacia = true
		case len(xs) == 1:
			hayUno = true
		}
		for _, x := range xs {
			if x.(int) < 0 {
				hayNeg = true
			}
		}
		if len(xs) > 3 && sort.SliceIsSorted(xs, func(i, j int) bool { return xs[i].(int) < xs[j].(int) }) {
			hayAsc = true
		}
	}
	if !hayNil || !hayVacia || !hayUno || !hayNeg || !hayAsc {
		t.Fatalf("faltan bordes: nil=%v vacía=%v uno=%v negativos=%v ascendente=%v en %v", hayNil, hayVacia, hayUno, hayNeg, hayAsc, bs)
	}
}

func TestBordesBasicos(t *testing.T) {
	g := NuevoGenerador(1)
	if got := g.Bordes(nucleo.TInt); !reflect.DeepEqual(got, []nucleo.Valor{0, 1, -1, 2, 7, -100, 1000}) {
		t.Fatalf("int: %v", got)
	}
	for _, b := range g.Bordes(nucleo.Tipo{Clase: nucleo.CInt, Nombre: "uint8"}) {
		if b.(int) < 0 || b.(int) > 255 {
			t.Fatalf("uint8 fuera de rango: %v", b)
		}
	}
	if got := g.Bordes(nucleo.TString); len(got) != 10 || got[0] != "" {
		t.Fatalf("string: %v", got)
	}
	if got := g.Bordes(nucleo.TBool); len(got) != 2 {
		t.Fatalf("bool: %v", got)
	}
	m := g.Bordes(nucleo.MapaDe(nucleo.TString, nucleo.TInt))
	if len(m) != 4 || m[0].(nucleo.Mapa) != nil || len(m[2].(nucleo.Mapa)) != 1 || len(m[3].(nucleo.Mapa)) != 3 {
		t.Fatalf("mapa: %v", m)
	}
	arr := g.Bordes(nucleo.ArregloDe(3, nucleo.TInt))
	for _, a := range arr {
		if len(a.([]nucleo.Valor)) != 3 {
			t.Fatalf("arreglo de largo distinto de 3: %v", a)
		}
	}
	if g.Bordes(nucleo.Tipo{Clase: nucleo.CInvalida}) != nil {
		t.Fatal("un tipo inválido no tiene bordes")
	}
}

func TestBordesStructVaríanCadaCampo(t *testing.T) {
	punto := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Punto", Definido: true, Campos: []nucleo.Campo{
		{Nombre: "X", Tipo: nucleo.TInt}, {Nombre: "nombre", Tipo: nucleo.TString}, {Nombre: "ok", Tipo: nucleo.TBool},
	}}
	bs := NuevoGenerador(1).Bordes(punto)
	base := bs[0].(nucleo.Estructura)
	if !nucleo.Igual(base, nucleo.Estructura{0, "", false}) {
		t.Fatalf("la base debe tener cada campo en su primer borde: %v", base)
	}
	variado := make([]bool, 3)
	for _, b := range bs[1:] {
		s := b.(nucleo.Estructura)
		distintos := 0
		for i := range s {
			if !nucleo.Igual(s[i], base[i]) {
				variado[i] = true
				distintos++
			}
		}
		if distintos != 1 {
			t.Fatalf("cada variación cambia un solo campo: %v", s)
		}
	}
	for i, v := range variado {
		if !v {
			t.Fatalf("el campo %d nunca varía", i)
		}
	}
	pb := NuevoGenerador(1).Bordes(nucleo.PunteroA(punto))
	if len(pb) != 2 || pb[0] != nil {
		t.Fatalf("puntero: %v", pb)
	}
}

func TestCombinarCubreParejas(t *testing.T) {
	tams := []int{7, 10, 4, 8}
	filas := combinar(tams, 200)
	if len(filas) >= 7*10*4*8 {
		t.Fatalf("no es un diseño por parejas: %d filas", len(filas))
	}
	for i := 0; i < len(tams); i++ {
		for j := i + 1; j < len(tams); j++ {
			visto := map[[2]int]bool{}
			for _, f := range filas {
				visto[[2]int{f[i], f[j]}] = true
			}
			if len(visto) != tams[i]*tams[j] {
				t.Fatalf("parámetros %d y %d: %d de %d parejas", i, j, len(visto), tams[i]*tams[j])
			}
		}
	}
	if got := combinar([]int{2, 3}, 64); len(got) != 6 {
		t.Fatalf("producto pequeño completo: %v", got)
	}
	if got := combinar([]int{10, 10, 10}, 5); len(got) != 5 {
		t.Fatalf("tope: %d filas", len(got))
	}
}

func TestEscalar(t *testing.T) {
	g := NuevoGenerador(3)
	c, ok := g.Escalar(nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}, {Nombre: "k", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}, 500)
	if !ok || len(c.Entradas[0].([]nucleo.Valor)) != 500 {
		t.Fatalf("Escalar: %v %v", ok, c)
	}
	s, ok := g.Escalar(nucleo.Firma{Nombre: "G", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt}}, 64)
	if !ok || len([]rune(s.Entradas[0].(string))) != 64 {
		t.Fatalf("Escalar texto: %v", s)
	}
	if _, ok := g.Escalar(nucleo.Firma{Nombre: "H", Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}, 10); ok {
		t.Fatal("una firma sin listas ni textos no se puede escalar")
	}
}

func TestConOraculo(t *testing.T) {
	cs := []nucleo.Caso{
		{Entradas: []nucleo.Valor{lista(1, 2)}, Origen: nucleo.OrigenAzar},
		{Entradas: []nucleo.Valor{lista(-1)}, Origen: nucleo.OrigenAzar},
		{Entradas: []nucleo.Valor{lista(5)}, Esperado: []nucleo.Valor{9}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario},
		{Entradas: []nucleo.Valor{lista(7)}, Origen: nucleo.OrigenAzar},
	}
	or := func(e []nucleo.Valor) ([]nucleo.Valor, error) {
		xs := e[0].([]nucleo.Valor)
		if len(xs) == 1 && xs[0] == -1 {
			return nil, context.Canceled
		}
		if len(xs) == 1 && xs[0] == 7 {
			panic("roto")
		}
		s := 0
		for _, x := range xs {
			s += x.(int)
		}
		xs[0] = 100 // the oracle must receive a copy
		return []nucleo.Valor{s}, nil
	}
	out := ConOraculo(cs, or, nucleo.EspInterpretacion, "lo que entendí")
	if out[0].Expectativa != nucleo.EspInterpretacion || out[0].Esperado[0] != 3 || out[0].Nota != "lo que entendí" {
		t.Fatalf("caso 0: %+v", out[0])
	}
	if cs[0].Entradas[0].([]nucleo.Valor)[0] != 1 || cs[0].Expectativa != nucleo.EspNinguna {
		t.Fatal("ConOraculo modificó los casos de entrada")
	}
	if out[1].Expectativa != nucleo.EspNinguna || out[3].Expectativa != nucleo.EspNinguna {
		t.Fatal("cuando el oráculo falla, el caso se queda sin esperado")
	}
	if out[2].Expectativa != nucleo.EspUsuario || out[2].Esperado[0] != 9 {
		t.Fatal("un ejemplo del usuario no se toca")
	}
}

func TestReduccionesMenores(t *testing.T) {
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}, {Nombre: "s", Tipo: nucleo.TString}, {Nombre: "r", Tipo: nucleo.TRune}}, Res: []nucleo.Tipo{nucleo.TInt}}
	c := nucleo.Caso{Entradas: []nucleo.Valor{lista(5, -37, 8, 2, 11, 40, 3, 9, 10, 12, 77, 1), "ñandú es genial", int('z')}, Origen: nucleo.OrigenAzar}
	rs := Reducciones(c, f)
	if len(rs) == 0 || len(rs) > 40 {
		t.Fatalf("%d candidatos", len(rs))
	}
	tam := tamanoCaso(c.Entradas)
	visto := map[string]bool{}
	for _, r := range rs {
		if tamanoCaso(r.Entradas) >= tam {
			t.Fatalf("candidato no menor: %v", r.Entradas)
		}
		k := claveEntradas(r.Entradas)
		if visto[k] {
			t.Fatalf("candidato repetido: %v", r.Entradas)
		}
		visto[k] = true
	}
	// struct fields go to zero
	pt := nucleo.Tipo{Clase: nucleo.CStruct, Campos: []nucleo.Campo{{Nombre: "X", Tipo: nucleo.TInt}, {Nombre: "Y", Tipo: nucleo.TString}}}
	fs := nucleo.Firma{Nombre: "G", Params: []nucleo.Param{{Nombre: "p", Tipo: pt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	hayCero := false
	for _, r := range Reducciones(nucleo.Caso{Entradas: []nucleo.Valor{nucleo.Estructura{4, "ab"}}}, fs) {
		if nucleo.Igual(r.Entradas[0], nucleo.Estructura{0, "ab"}) {
			hayCero = true
		}
	}
	if !hayCero {
		t.Fatal("falta poner un campo a cero")
	}
}

func TestMinimizarAMenosUno(t *testing.T) {
	f := firmaLista()
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func(in []nucleo.Valor) ([]nucleo.Valor, string) {
			for _, x := range in[0].([]nucleo.Valor) {
				if x.(int) < 0 {
					return nil, "negativo"
				}
			}
			return []nucleo.Valor{0}, ""
		},
	}}
	bin, _, err := ej.Preparar(context.Background(), nucleo.Preparacion{Variantes: []string{"package solucion"}, Firma: f})
	if err != nil {
		t.Fatal(err)
	}
	llamadas := 0
	falla := func(cs []nucleo.Caso) []bool {
		llamadas++
		res, err := bin.Probar(context.Background(), cs, nucleo.OpcionesProbar{})
		if err != nil {
			t.Fatal(err)
		}
		out := make([]bool, len(cs))
		for i, r := range res[0] {
			out[i] = !r.OK
		}
		return out
	}
	c := nucleo.Caso{Entradas: []nucleo.Valor{lista(5, -37, 8, 2, 14, -3, 99)}, Esperado: []nucleo.Valor{0}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario}
	m := Minimizar(context.Background(), c, f, falla, 0)
	if !nucleo.Igual(m.Entradas[0], lista(-1)) {
		t.Fatalf("se esperaba [-1], salió %v", m.Entradas[0])
	}
	if m.Esperado != nil || m.Origen != nucleo.OrigenUsuario {
		t.Fatalf("caso reducido: %+v", m)
	}
	if llamadas > 12 || ej.NumLlamadas("Preparar") != 1 {
		t.Fatalf("%d rondas, %d Preparar", llamadas, ej.NumLlamadas("Preparar"))
	}
	// a case that does not fail when shrunk comes back unchanged
	sinCambio := Minimizar(context.Background(), nucleo.Caso{Entradas: []nucleo.Valor{lista(-1)}}, f, falla, 3)
	if !nucleo.Igual(sinCambio.Entradas[0], lista(-1)) {
		t.Fatal("un caso mínimo debe volver igual")
	}
}

func TestPropiedadOrdenarDetectaIdentidad(t *testing.T) {
	f := nucleo.Firma{Nombre: "Ordenar", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.ListaDe(nucleo.TInt)}}
	props := Propiedades([]string{"ordenar", "lista", "numeros"}, f)
	if len(props) != 1 || props[0].Expr != "nyx__Ordenada(r0) && nyx__EsPermutacion(e0, r0)" {
		t.Fatalf("propiedades: %+v", props)
	}
	identidad := func(in []nucleo.Valor) ([]nucleo.Valor, string) { return []nucleo.Valor{in[0]}, "" }
	ordenar := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		xs := append([]nucleo.Valor(nil), in[0].([]nucleo.Valor)...)
		sort.SliceStable(xs, func(i, j int) bool { return xs[i].(int) < xs[j].(int) })
		return []nucleo.Valor{xs}, ""
	}
	soloPrimero := func(in []nucleo.Valor) ([]nucleo.Valor, string) { // sorted but loses elements
		xs := in[0].([]nucleo.Valor)
		if len(xs) > 1 {
			xs = xs[:1]
		}
		return []nucleo.Valor{xs}, ""
	}
	funcs := map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: identidad, 1: ordenar, 2: soloPrimero}
	ej := &nucleotest.EjecutorFalso{Funcs: funcs}
	ej.EvalProp = func(p nucleo.Propiedad, e, r []nucleo.Valor) bool {
		ok, err := evaluarProp(p.Expr, e, r, nil)
		if err != nil {
			t.Fatalf("evaluar %q: %v", p.Expr, err)
		}
		return ok
	}
	bin, _, err := ej.Preparar(context.Background(), nucleo.Preparacion{Firma: f, Props: props})
	if err != nil {
		t.Fatal(err)
	}
	casos := NuevoGenerador(7).Casos(f, OpcionesCasos{Azar: 50})
	res, err := bin.Probar(context.Background(), casos, nucleo.OpcionesProbar{})
	if err != nil {
		t.Fatal(err)
	}
	cuenta := func(v int) (malos int) {
		for _, r := range res[v] {
			if !r.OK {
				malos++
				if r.PropFallida != "ordenada" {
					t.Fatalf("propiedad fallida %q", r.PropFallida)
				}
			}
		}
		return
	}
	if cuenta(0) == 0 {
		t.Fatal("la propiedad no detecta la identidad")
	}
	if cuenta(1) != 0 {
		t.Fatal("la propiedad rechaza una ordenación correcta")
	}
	if cuenta(2) == 0 {
		t.Fatal("la propiedad no detecta que se pierden elementos")
	}
}

func TestPropiedadesEvaluables(t *testing.T) {
	li := nucleo.ListaDe(nucleo.TInt)
	uno := func(nombre string, p, r nucleo.Tipo) nucleo.Firma {
		return nucleo.Firma{Nombre: nombre, Params: []nucleo.Param{{Nombre: "x", Tipo: p}}, Res: []nucleo.Tipo{r}}
	}
	invertir := func(a []nucleo.Valor) nucleo.Valor {
		xs := a[0].([]nucleo.Valor)
		out := make([]nucleo.Valor, len(xs))
		for i, x := range xs {
			out[len(xs)-1-i] = x
		}
		return out
	}
	suma := func(a []nucleo.Valor) nucleo.Valor {
		s := 0
		for _, x := range a[0].([]nucleo.Valor) {
			if x.(int)%2 == 0 {
				s += x.(int)
			}
		}
		return s
	}
	sumaMal := func(a []nucleo.Valor) nucleo.Valor { return suma(a).(int) + 1 }
	casos := [][]nucleo.Valor{{lista()}, {lista(1, 2, 3, 4)}, {lista(-2, 5, 8)}, {lista(9)}}

	pruebas := []struct {
		conceptos []string
		f         nucleo.Firma
		F         func([]nucleo.Valor) nucleo.Valor
		bien      bool
	}{
		{[]string{"invertir"}, uno("Invertir", li, li), invertir, true},
		// the identity is its own inverse, so the property cannot catch it; dropping the last element can be caught
		{[]string{"invertir"}, uno("Invertir", li, li), func(a []nucleo.Valor) nucleo.Valor {
			xs := invertir(a).([]nucleo.Valor)
			if len(xs) > 0 {
				xs = xs[:len(xs)-1]
			}
			return xs
		}, false},
		{[]string{"sumar", "par"}, uno("SumaPares", li, nucleo.TInt), suma, true},
		{[]string{"sumar", "par"}, uno("SumaPares", li, nucleo.TInt), sumaMal, false},
	}
	for i, p := range pruebas {
		props := Propiedades(p.conceptos, p.f)
		if len(props) == 0 {
			t.Fatalf("prueba %d: sin propiedades", i)
		}
		todas := true
		for _, c := range casos {
			r := []nucleo.Valor{p.F(c)}
			for _, pr := range props {
				ok, err := evaluarProp(pr.Expr, c, r, p.F)
				if err != nil {
					t.Fatalf("prueba %d %q: %v", i, pr.Expr, err)
				}
				todas = todas && ok
			}
		}
		if todas != p.bien {
			t.Fatalf("prueba %d (%v): se esperaba %v", i, p.conceptos, p.bien)
		}
	}
}

func TestPropiedadesSoloSiEncajanLosTipos(t *testing.T) {
	li := nucleo.ListaDe(nucleo.TInt)
	if ps := Propiedades([]string{"ordenar"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt}}); len(ps) != 0 {
		t.Fatalf("ordenar con texto→int no encaja: %v", ps)
	}
	if ps := Propiedades([]string{"maximo"}, nucleo.Firma{Nombre: "Max", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{nucleo.TInt}}); len(ps) != 1 || ps[0].Requiere != "len(e0) > 0" {
		t.Fatalf("maximo: %v", ps)
	}
	// maximo of the even numbers: "every element ≤ r0" does not hold, so nothing is given
	if ps := Propiedades([]string{"maximo", "par"}, nucleo.Firma{Nombre: "Max", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{nucleo.TInt}}); len(ps) != 0 {
		t.Fatalf("maximo con modificador: %v", ps)
	}
	// filtering then sorting: no permutation property
	ps := Propiedades([]string{"ordenar", "par"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{li}})
	for _, p := range ps {
		if strings.Contains(p.Expr, "EsPermutacion") || p.Nombre == "subsecuencia" {
			t.Fatalf("propiedad que no se cumple al filtrar y ordenar: %+v", p)
		}
	}
	// error result: properties hold trivially when an error is returned
	ps = Propiedades([]string{"longitud"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt, nucleo.TError}})
	if len(ps) != 1 || ps[0].Expr != "r1 != nil || (r0 >= 0)" {
		t.Fatalf("con error: %+v", ps)
	}
	// methods get no property that calls F
	met := nucleo.Firma{Nombre: "Invertir", Receptor: &nucleo.Param{Nombre: "l", Tipo: nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "L", Definido: true}}, Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{li}}
	for _, p := range Propiedades([]string{"invertir"}, met) {
		if strings.Contains(p.Expr, "F(") {
			t.Fatalf("método con F: %+v", p)
		}
	}
}

// soporteProp declares plausible versions of the five harness helpers so the property expressions can be
// type-checked the way the harness does (one closure per property).
const soporteProp = `package solucion

import "cmp"

func nyx__Ordenada[T cmp.Ordered](xs []T) bool { return true }
func nyx__EsPermutacion[T comparable](a, b []T) bool { return true }
func nyx__Subsecuencia[T comparable](a, b []T) bool { return true }
func nyx__Contiene[T comparable](xs []T, x T) bool { return true }
func nyx__Igual(a, b any) bool { return true }
`

func importador(t *testing.T) types.Importer {
	t.Helper()
	imp := importer.Default()
	if _, err := imp.Import("fmt"); err == nil {
		return imp
	}
	src := importer.ForCompiler(token.NewFileSet(), "source", nil)
	if _, err := src.Import("fmt"); err != nil {
		t.Skip("no hay datos de la biblioteca estándar para go/types:", err)
	}
	return src
}

func revisarTipos(t *testing.T, archivos map[string]string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var fs []*ast.File
	nombres := make([]string, 0, len(archivos))
	for n := range archivos {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	for _, n := range nombres {
		f, err := parser.ParseFile(fset, n, archivos[n], parser.ParseComments)
		if err != nil {
			t.Fatalf("%s no se puede leer: %v\n%s", n, err, archivos[n])
		}
		fs = append(fs, f)
	}
	var errs []string
	conf := types.Config{Importer: importador(t), GoVersion: "go1.22", Error: func(err error) { errs = append(errs, err.Error()) }}
	_, _ = conf.Check("solucion", fset, fs, nil)
	return errs
}

func TestPropiedadesCompilan(t *testing.T) {
	li := nucleo.ListaDe(nucleo.TInt)
	ls := nucleo.ListaDe(nucleo.TString)
	casos := []struct {
		conceptos []string
		f         nucleo.Firma
	}{
		{[]string{"ordenar"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{li}}},
		{[]string{"ordenar_desc"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: ls}}, Res: []nucleo.Tipo{ls}}},
		{[]string{"invertir"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TString}}},
		{[]string{"filtrar", "par"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{li}}},
		{[]string{"sumar"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TFloat)}}, Res: []nucleo.Tipo{nucleo.TFloat}}},
		{[]string{"sumar"}, nucleo.Firma{Nombre: "F", Variadica: true, Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{nucleo.TInt}}},
		{[]string{"maximo"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{nucleo.TInt, nucleo.TError}}},
		{[]string{"minimo"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TFloat)}}, Res: []nucleo.Tipo{nucleo.TFloat}}},
		{[]string{"contar", "vocal"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt}}},
		{[]string{"contar"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}, {Nombre: "x", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}},
		{[]string{"sin_repetir"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{li}}},
		{[]string{"sin_repetir"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TString}}},
		{[]string{"palindromo"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TBool}}},
		{[]string{"palindromo"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: li}}, Res: []nucleo.Tipo{nucleo.TBool}}},
		{[]string{"mayusculas"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TString}}},
		{[]string{"titulo"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: ls}}, Res: []nucleo.Tipo{ls}}},
		{[]string{"longitud"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt}}},
		{[]string{"absoluto"}, nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TFloat}}, Res: []nucleo.Tipo{nucleo.TFloat}}},
	}
	for i, c := range casos {
		props := Propiedades(c.conceptos, c.f)
		if len(props) == 0 {
			t.Fatalf("caso %d (%v): sin propiedades", i, c.conceptos)
		}
		var sb strings.Builder
		sb.WriteString("package solucion\n\n")
		// the function under test, with a body that returns zero values
		sb.WriteString(c.f.Go() + " { ")
		ceros := make([]string, len(c.f.Res))
		for j, r := range c.f.Res {
			ceros[j] = r.Cero()
		}
		sb.WriteString("return " + strings.Join(ceros, ", ") + " }\n\n")
		ent := c.f.Entradas()
		params := make([]string, 0, len(ent)+len(c.f.Res))
		for j, e := range ent {
			params = append(params, "e"+itoa(j)+" "+e.Go())
		}
		for j, r := range c.f.Res {
			params = append(params, "r"+itoa(j)+" "+r.Go())
		}
		for j, p := range props {
			sb.WriteString("var _ = func(" + strings.Join(params, ", ") + ") bool {\n\tF := " + c.f.Nombre + "\n\t_ = F\n")
			for k := range ent {
				sb.WriteString("\t_ = e" + itoa(k) + "\n")
			}
			for k := range c.f.Res {
				sb.WriteString("\t_ = r" + itoa(k) + "\n")
			}
			if p.Requiere != "" {
				sb.WriteString("\tif !(" + p.Requiere + ") {\n\t\treturn true\n\t}\n")
			}
			sb.WriteString("\treturn " + p.Expr + "\n}\n")
			_ = j
		}
		if errs := revisarTipos(t, map[string]string{"a.go": sb.String(), "soporte.go": soporteProp}); len(errs) > 0 {
			t.Fatalf("caso %d (%v): %v\n%s", i, c.conceptos, errs, sb.String())
		}
	}
}

func itoa(i int) string { return string(rune('0' + i)) }

func TestCompararSalida(t *testing.T) {
	casos := []struct {
		obtenida, esperado, modo string
		quiero                   bool
	}{
		{"La suma es 5\n", "La suma es 5\n", "exacto", true},
		{"La suma es 5\r\n", "La suma es 5\n", "exacto", true},
		{"La suma es 5", "La suma es 5\n", "exacto", false},
		{"La suma es 5  \nfin\n\n", "La suma es 5\nfin", "lineas", true},
		{"La suma es 5\nfin", "La suma es 6\nfin", "lineas", false},
		{"  hola\n", "hola\n", "lineas", false},
		{"a\n", "a", "", true},
		{"Escribe un número: La suma es 5\n", "La suma es 5", "contiene", true},
		{"La suma es 5\n", "suma es 6", "contiene", false},
		{"El resultado: 3.5 y luego -2", "3.50000000001, -2", "numeros", true},
		{"1 2 3", "1 2", "numeros", false},
		{"x=10", "10", "numeros", true},
		{"0.1", "0.2", "numeros", false},
	}
	for i, c := range casos {
		if got := CompararSalida(c.obtenida, nucleo.CasoPrograma{Esperado: c.esperado, Comparar: c.modo}); got != c.quiero {
			t.Errorf("caso %d (%s %q vs %q): %v", i, c.modo, c.obtenida, c.esperado, got)
		}
	}
}

func TestArchivoTestCompila(t *testing.T) {
	punto := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "punto", Definido: true, Campos: []nucleo.Campo{{Nombre: "x", Tipo: nucleo.TInt}, {Nombre: "nombre", Tipo: nucleo.TString}}}
	fuente := `package solucion

import (
	"errors"
	"math"
)

type punto struct {
	x      int
	nombre string
}

func SumaPares(nums []int) int { return 0 }

func dividir(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("división por cero")
	}
	return a / b, nil
}

func Max(xs ...int) int { return 0 }

func (p *punto) Mover(d int, nombre string) (punto, []string) { return *p, nil }

func Raiz(x float64) float64 { return math.Sqrt(x) }

func Indice(m map[string][]int, k rune) [2]bool { return [2]bool{} }
`
	firmas := []struct {
		f     nucleo.Firma
		casos []nucleo.Caso
	}{
		{firmaLista(), []nucleo.Caso{
			{Entradas: []nucleo.Valor{lista(1, 2, 3, 4)}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario, Nota: "tu ejemplo 1"},
			{Entradas: []nucleo.Valor{[]nucleo.Valor(nil)}, Esperado: []nucleo.Valor{0}, Expectativa: nucleo.EspInterpretacion},
			{Entradas: []nucleo.Valor{lista(-5)}, Origen: nucleo.OrigenAzar},
		}},
		{nucleo.Firma{Nombre: "dividir", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TFloat}, {Nombre: "b", Tipo: nucleo.TFloat}}, Res: []nucleo.Tipo{nucleo.TFloat, nucleo.TError}}, []nucleo.Caso{
			{Entradas: []nucleo.Valor{1.0, 0.0}, Esperado: []nucleo.Valor{0.0, nucleo.ErrorV("x")}, Expectativa: nucleo.EspUsuario},
			{Entradas: []nucleo.Valor{1.0, 4.0}, Esperado: []nucleo.Valor{0.25, nil}, Expectativa: nucleo.EspUsuario},
		}},
		{nucleo.Firma{Nombre: "Max", Variadica: true, Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}, []nucleo.Caso{
			{Entradas: []nucleo.Valor{lista(3, 1)}, Esperado: []nucleo.Valor{3}, Expectativa: nucleo.EspUsuario},
		}},
		{nucleo.Firma{Nombre: "Mover", Receptor: &nucleo.Param{Nombre: "p", Tipo: nucleo.PunteroA(punto)},
			Params: []nucleo.Param{{Nombre: "d", Tipo: nucleo.TInt}, {Nombre: "nombre", Tipo: nucleo.TString}},
			Res:    []nucleo.Tipo{punto, nucleo.ListaDe(nucleo.TString)}}, []nucleo.Caso{
			{Entradas: []nucleo.Valor{nucleo.Estructura{1, "a"}, 2, "b"}, Esperado: []nucleo.Valor{nucleo.Estructura{3, "b"}, lista()}, Expectativa: nucleo.EspUsuario},
			{Entradas: []nucleo.Valor{nil, 2, "b"}},
		}},
		{nucleo.Firma{Nombre: "Raiz", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TFloat}}, Res: []nucleo.Tipo{nucleo.TFloat}}, []nucleo.Caso{
			{Entradas: []nucleo.Valor{-1.0}, Esperado: []nucleo.Valor{nucleo.Valor(nanF())}, Expectativa: nucleo.EspReferencia, Nota: "math.Sqrt"},
		}},
		{nucleo.Firma{Nombre: "Indice", Params: []nucleo.Param{{Nombre: "m", Tipo: nucleo.MapaDe(nucleo.TString, nucleo.ListaDe(nucleo.TInt))}, {Nombre: "k", Tipo: nucleo.TRune}}, Res: []nucleo.Tipo{nucleo.ArregloDe(2, nucleo.TBool)}}, []nucleo.Caso{
			{Entradas: []nucleo.Valor{nucleo.Mapa{{K: "a", V: lista(1)}}, int('ñ')}, Esperado: []nucleo.Valor{[]nucleo.Valor{true, false}}, Expectativa: nucleo.EspUsuario},
		}},
	}
	archivos := map[string]string{"solucion.go": fuente}
	for i, fc := range firmas {
		src := ArchivoTest(fc.f, fc.casos, "solucion")
		if !strings.Contains(src, "func Test") {
			t.Fatalf("firma %d sin test:\n%s", i, src)
		}
		archivos["t"+itoa(i)+"_test.go"] = src
	}
	if errs := revisarTipos(t, archivos); len(errs) > 0 {
		for n, s := range archivos {
			t.Logf("== %s\n%s", n, s)
		}
		t.Fatalf("los tests generados no compilan: %v", errs)
	}
	// the table carries the user's note and the expected value
	src := archivos["t0_test.go"]
	if !strings.Contains(src, `"tu ejemplo 1"`) || !strings.Contains(src, "TestSumaParesSinPanico") || !strings.Contains(src, "quiero: 6") {
		t.Fatalf("contenido inesperado:\n%s", src)
	}
}

func nanF() float64 {
	var z float64
	return z / z
}

func resultadosOK(casos []nucleo.Caso, malos map[int]bool) []nucleo.ResultadoCaso {
	rs := make([]nucleo.ResultadoCaso, len(casos))
	for i, c := range casos {
		rs[i] = nucleo.ResultadoCaso{Caso: i, Ejecutado: true, OK: !malos[i], Obtenido: c.Esperado}
		if malos[i] && c.ConEsperado() {
			rs[i].Obtenido = []nucleo.Valor{-99}
		}
		if malos[i] && !c.ConEsperado() {
			rs[i].Panico = "runtime error: index out of range [0] with length 0"
		}
	}
	return rs
}

func TestComprobacionesTresInsignias(t *testing.T) {
	var casos []nucleo.Caso
	for i := 0; i < 3; i++ {
		casos = append(casos, nucleo.Caso{Entradas: []nucleo.Valor{lista(i)}, Esperado: []nucleo.Valor{i}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario})
	}
	for i := 0; i < 47; i++ {
		casos = append(casos, nucleo.Caso{Entradas: []nucleo.Valor{lista(i, i)}, Esperado: []nucleo.Valor{2 * i}, Expectativa: nucleo.EspInterpretacion, Origen: nucleo.OrigenAzar})
	}
	props := Propiedades([]string{"sumar"}, firmaLista())
	cs := Comprobaciones(casos, resultadosOK(casos, nil), props, 42)
	if len(cs) != 3 {
		t.Fatalf("se esperaban 3 insignias: %+v", cs)
	}
	if cs[0].Nivel != nucleo.NivelTusEjemplos || cs[0].Texto != "Cumple tus 3 ejemplos" {
		t.Fatalf("usuario: %+v", cs[0])
	}
	if cs[1].Nivel != nucleo.NivelEntendido || cs[1].Texto != "47/47 pruebas según lo que entendí" {
		t.Fatalf("entendido: %+v", cs[1])
	}
	if cs[2].Nivel != nucleo.NivelPropiedades || cs[2].Texto != "Propiedades: 50/50" {
		t.Fatalf("propiedades: %+v", cs[2])
	}
	if !strings.Contains(cs[0].Detalle, "semilla 42") || !strings.Contains(cs[0].Detalle, "47 al azar") {
		t.Fatalf("detalle: %q", cs[0].Detalle)
	}
	if NivelGlobal(cs) != nucleo.NivelTusEjemplos {
		t.Fatalf("nivel global %q", NivelGlobal(cs))
	}

	// a failing user example makes the whole thing fail
	cs = Comprobaciones(casos, resultadosOK(casos, map[int]bool{1: true}), nil, 1)
	if cs[0].Texto != "Falla 1 de tus 3 ejemplos" || cs[0].Nivel != nucleo.NivelFallo || NivelGlobal(cs) != nucleo.NivelFallo {
		t.Fatalf("fallo: %+v", cs)
	}
	// a property failure does not count against the user's example
	rs := resultadosOK(casos, nil)
	rs[0].OK, rs[0].PropFallida = false, "suma_partida"
	cs = Comprobaciones(casos, rs, props, 1)
	if cs[0].Nivel != nucleo.NivelTusEjemplos || cs[2].Texto != "Propiedades: 49/50" {
		t.Fatalf("propiedad fallida: %+v", cs)
	}
}

func TestComprobacionesReferenciaYSinFallos(t *testing.T) {
	casos := []nucleo.Caso{
		{Entradas: []nucleo.Valor{lista(1)}, Esperado: []nucleo.Valor{1}, Expectativa: nucleo.EspReferencia, Nota: "Maximo"},
		{Entradas: []nucleo.Valor{lista(2)}, Esperado: []nucleo.Valor{2}, Expectativa: nucleo.EspReferencia, Nota: "Maximo"},
		{Entradas: []nucleo.Valor{lista(3)}, Origen: nucleo.OrigenAzar},
		{Entradas: []nucleo.Valor{lista()}, Origen: nucleo.OrigenBorde},
	}
	cs := Comprobaciones(casos, resultadosOK(casos, nil), nil, 7)
	if len(cs) != 2 || cs[0].Texto != "2/2 pruebas comparando con Maximo" || cs[1].Texto != "2 casos sin errores" || cs[1].Nivel != nucleo.NivelSinFallos {
		t.Fatalf("%+v", cs)
	}
	if NivelGlobal(cs) != nucleo.NivelEntendido {
		t.Fatalf("nivel %q", NivelGlobal(cs))
	}
	cs = Comprobaciones(casos, resultadosOK(casos, map[int]bool{3: true}), nil, 7)
	if cs[1].Nivel != nucleo.NivelFallo || cs[1].Texto != "1 de 2 casos fallan" {
		t.Fatalf("%+v", cs[1])
	}
	// a case that never ran does not pass
	rs := resultadosOK(casos, nil)
	rs[2].Ejecutado = false
	if cs := Comprobaciones(casos, rs, nil, 7); cs[1].Pasan != 1 {
		t.Fatalf("%+v", cs[1])
	}
	if NivelGlobal(nil) != nucleo.NivelSinComprobar {
		t.Fatal("sin comprobaciones")
	}
}

func TestTabla(t *testing.T) {
	f := firmaLista()
	casos := []nucleo.Caso{
		{Entradas: []nucleo.Valor{lista(1, 2)}, Esperado: []nucleo.Valor{2}, Expectativa: nucleo.EspUsuario, Nota: "tu ejemplo 1"},
		{Entradas: []nucleo.Valor{lista(4)}, Esperado: []nucleo.Valor{4}, Expectativa: nucleo.EspUsuario, Nota: "tu ejemplo 2"},
		{Entradas: []nucleo.Valor{lista()}, Origen: nucleo.OrigenBorde},
	}
	rs := resultadosOK(casos, map[int]bool{1: true, 2: true})
	tb := Tabla(f, casos, rs, 2)
	if !reflect.DeepEqual(tb.Cabecera, []string{"Entrada", "Esperado", "Obtenido", "✔/✘", "Origen"}) {
		t.Fatalf("cabecera %v", tb.Cabecera)
	}
	if len(tb.Filas) != 2 || tb.Filas[0][4] != "tu ejemplo 2" || tb.Filas[0][3] != "✘" || tb.Marcas[0] != "mal" {
		t.Fatalf("filas %v", tb.Filas)
	}
	if tb.Filas[0][0] != "[4]" || tb.Filas[0][1] != "4" || tb.Filas[0][2] != "-99" {
		t.Fatalf("fila 0: %v", tb.Filas[0])
	}
	if !strings.HasPrefix(tb.Filas[1][2], "pánico:") || tb.Filas[1][1] != "—" || tb.Filas[1][4] != "de borde" {
		t.Fatalf("fila 1: %v", tb.Filas[1])
	}
}

func TestArchivoTestSeEjecuta(t *testing.T) {
	if testing.Short() {
		t.Skip("modo corto")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no hay go instalado")
	}
	f := firmaLista()
	casos := []nucleo.Caso{
		{Entradas: []nucleo.Valor{lista(1, 2, 3, 4)}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario, Nota: "tu ejemplo"},
		{Entradas: []nucleo.Valor{[]nucleo.Valor(nil)}, Esperado: []nucleo.Valor{0}, Expectativa: nucleo.EspUsuario},
		{Entradas: []nucleo.Valor{lista(-3, 7)}},
	}
	correr := func(cuerpo string) (string, error) {
		dir := t.TempDir()
		archivos := map[string]string{
			"go.mod":           "module solucion\n\ngo 1.22\n",
			"solucion.go":      "package solucion\n\nfunc SumaPares(nums []int) int {\n" + cuerpo + "\n}\n",
			"solucion_test.go": ArchivoTest(f, casos, "solucion"),
		}
		for n, s := range archivos {
			if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(goBin, "test", "-count=1", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	bien := "\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n%2 == 0 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total"
	if out, err := correr(bien); err != nil {
		t.Fatalf("una implementación correcta no pasa sus pruebas: %v\n%s", err, out)
	}
	mal := "\treturn len(nums)"
	out, err := correr(mal)
	if err == nil || !strings.Contains(out, "SumaPares([1 2 3 4]) = 4; quiero 6") {
		t.Fatalf("una implementación mala debe fallar con un mensaje claro: %v\n%s", err, out)
	}
}
