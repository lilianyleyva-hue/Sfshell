package nucleo

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseValorIdaYVuelta(t *testing.T) {
	casos := []struct {
		texto  string
		t      Tipo
		valor  Valor
		humano string
		goLit  string
	}{
		{"[1, 2,3]", ListaDe(TInt), []Valor{1, 2, 3}, "[1, 2, 3]", "[]int{1, 2, 3}"},
		{"hola", TString, "hola", `"hola"`, `"hola"`},
		{`"hola"`, TString, "hola", `"hola"`, `"hola"`},
		{"'a'", TRune, int('a'), "'a'", "'a'"},
		{"a", TRune, int('a'), "'a'", "'a'"},
		{"3,5", TFloat, 3.5, "3.5", "3.5"},
		{"3.5", TFloat, 3.5, "3.5", "3.5"},
		{`{"a":1}`, MapaDe(TString, TInt), Mapa{{K: "a", V: 1}}, "{a: 1}", `map[string]int{"a": 1}`},
		{"Punto{X: 1, Y: 2}", tPunto, Estructura{1, 2}, "{X: 1, Y: 2}", "Punto{X: 1, Y: 2}"},
		{"{1, 2}", tPunto, Estructura{1, 2}, "{X: 1, Y: 2}", "Punto{X: 1, Y: 2}"},
		{"{Y: 2}", tPunto, Estructura{0, 2}, "{X: 0, Y: 2}", "Punto{X: 0, Y: 2}"},
		{"&Punto{X: 3}", PunteroA(tPunto), Estructura{3, 0}, "&{X: 3, Y: 0}", "&Punto{X: 3, Y: 0}"},
		{"nil", PunteroA(tPunto), nil, "nil", "nil"},
		{"-3", TInt, -3, "-3", "-3"},
		{"5", TInt, 5, "5", "5"},
		{"verdadero", TBool, true, "true", "true"},
		{"sí", TBool, true, "true", "true"},
		{"no", TBool, false, "false", "false"},
		{"nil", ListaDe(TInt), []Valor(nil), "[]", "nil"},
		{"[]", ListaDe(TInt), []Valor{}, "[]", "[]int{}"},
		{"[]int{1,2,3}", ListaDe(TInt), []Valor{1, 2, 3}, "[1, 2, 3]", "[]int{1, 2, 3}"},
		{"{1,2,3}", ListaDe(TInt), []Valor{1, 2, 3}, "[1, 2, 3]", "[]int{1, 2, 3}"},
		{"1 2 3", ListaDe(TInt), []Valor{1, 2, 3}, "[1, 2, 3]", "[]int{1, 2, 3}"},
		{"[1 2 3]", ListaDe(TInt), []Valor{1, 2, 3}, "[1, 2, 3]", "[]int{1, 2, 3}"},
		{`["a", "b c"]`, ListaDe(TString), []Valor{"a", "b c"}, `["a", "b c"]`, `[]string{"a", "b c"}`},
		{"[a, b]", ListaDe(TString), []Valor{"a", "b"}, `["a", "b"]`, `[]string{"a", "b"}`},
		{"[[1, 2], [3]]", ListaDe(ListaDe(TInt)), []Valor{[]Valor{1, 2}, []Valor{3}}, "[[1, 2], [3]]", "[][]int{{1, 2}, {3}}"},
		{"[3]int{1, 2}", ArregloDe(3, TInt), []Valor{1, 2, 0}, "[1, 2, 0]", "[3]int{1, 2, 0}"},
		{`map[string]int{"b": 2, "a": 1}`, MapaDe(TString, TInt), Mapa{{K: "a", V: 1}, {K: "b", V: 2}}, "{a: 1, b: 2}", `map[string]int{"a": 1, "b": 2}`},
		{"map[a:1 b:2]", MapaDe(TString, TInt), Mapa{{K: "a", V: 1}, {K: "b", V: 2}}, "{a: 1, b: 2}", `map[string]int{"a": 1, "b": 2}`},
		{`{"a": [1, 2]}`, MapaDe(TString, ListaDe(TInt)), Mapa{{K: "a", V: []Valor{1, 2}}}, "{a: [1, 2]}", `map[string][]int{"a": {1, 2}}`},
		{"{}", MapaDe(TString, TInt), Mapa{}, "{}", "map[string]int{}"},
		{`errors.New("x")`, TError, ErrorV("x"), `error("x")`, `errors.New("x")`},
		{"nil", TError, nil, "nil", "nil"},
		{"Celsius(36.6)", tCelsius, 36.6, "36.6", "Celsius(36.6)"},
		{"[Punto{1, 2}, {3, 4}]", ListaDe(tPunto), []Valor{Estructura{1, 2}, Estructura{3, 4}}, "[{X: 1, Y: 2}, {X: 3, Y: 4}]", "[]Punto{{X: 1, Y: 2}, {X: 3, Y: 4}}"},
		{"200", TByte, 200, "200", "200"},
		{"'\\n'", TRune, int('\n'), `'\n'`, `'\n'`},
		{"−7", TInt, -7, "-7", "-7"},
	}
	for _, c := range casos {
		v, err := ParseValor(c.texto, c.t)
		if err != nil {
			t.Errorf("ParseValor(%q, %s): %v", c.texto, c.t.Go(), err)
			continue
		}
		if !Igual(v, c.valor) {
			t.Errorf("ParseValor(%q, %s) = %#v, esperaba %#v", c.texto, c.t.Go(), v, c.valor)
		}
		if h := FormatoHumano(v, c.t); h != c.humano {
			t.Errorf("FormatoHumano(%#v, %s) = %q, esperaba %q", v, c.t.Go(), h, c.humano)
		}
		g := FormatoGo(v, c.t)
		if g != c.goLit {
			t.Errorf("FormatoGo(%#v, %s) = %q, esperaba %q", v, c.t.Go(), g, c.goLit)
		}
		w, err := ParseValor(g, c.t)
		if err != nil || !Igual(v, w) {
			t.Errorf("ParseValor(FormatoGo) de %q: %#v, %v", g, w, err)
		}
	}
}

func TestParseValorErrores(t *testing.T) {
	malos := []struct {
		texto string
		t     Tipo
	}{
		{"hola", TInt},
		{"3.5", TInt},
		{"300", TByte},
		{"-1", Tipo{Clase: CInt, Nombre: "uint"}},
		{"200", Tipo{Clase: CInt, Nombre: "int8"}},
		{"quizás", TBool},
		{"[1, x]", ListaDe(TInt)},
		{"[1, 2, 3, 4]", ArregloDe(3, TInt)},
		{"{1}", tPunto},
		{"{Z: 1}", tPunto},
		{`{"a" 1}`, MapaDe(TString, TInt)},
		{"abc", TRune},
	}
	for _, c := range malos {
		if v, err := ParseValor(c.texto, c.t); err == nil {
			t.Errorf("ParseValor(%q, %s) debería fallar; dio %#v", c.texto, c.t.Go(), v)
		}
	}
}

func TestParseValorDeFormatoGoAlAzar(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for _, tp := range tiposPrueba {
		for i := 0; i < 20; i++ {
			v := genValor(r, tp, 0)
			g := FormatoGo(v, tp)
			w, err := ParseValor(g, tp)
			if err != nil {
				t.Fatalf("%s: ParseValor(%q): %v", tp.Go(), g, err)
			}
			if tp.Clase == CError { // the message of an invalid-UTF-8 error may change; Igual ignores it
				if (v == nil) != (w == nil) {
					t.Fatalf("%s: %q dio %#v", tp.Go(), g, w)
				}
				continue
			}
			if !Igual(v, w) {
				t.Fatalf("%s: FormatoGo→ParseValor distinto:\n  %#v\n  %q\n  %#v", tp.Go(), v, g, w)
			}
		}
	}
}

func TestIgual(t *testing.T) {
	si := [][2]Valor{
		{nil, []Valor{}},
		{[]Valor(nil), []Valor{}},
		{nil, Mapa{}},
		{1.0, 1.0 + 1e-12},
		{1e12, 1e12 + 1e-1},
		{math.NaN(), math.NaN()},
		{math.Inf(1), math.Inf(1)},
		{3, 3.0},
		{ErrorV("a"), ErrorV("b")},
		{Mapa{{K: "b", V: 2}, {K: "a", V: 1}}, Mapa{{K: "a", V: 1}, {K: "b", V: 2}}},
		{[]Valor{1, []Valor{}}, []Valor{1, nil}},
		{Estructura{1, "x"}, Estructura{1, "x"}},
	}
	for _, p := range si {
		if !Igual(p[0], p[1]) || !Igual(p[1], p[0]) {
			t.Errorf("Igual(%#v, %#v) debería ser true", p[0], p[1])
		}
		if Comparar(p[0], p[1]) != 0 && !esErrorV(p[0]) {
			t.Errorf("Comparar(%#v, %#v) debería ser 0", p[0], p[1])
		}
	}
	no := [][2]Valor{
		{1.0, 1.0 + 1e-6},
		{math.NaN(), 0.0},
		{math.Inf(1), math.Inf(-1)},
		{nil, ErrorV("x")},
		{nil, Estructura{}},
		{1, 2},
		{"a", "b"},
		{true, false},
		{[]Valor{1}, []Valor{1, 2}},
		{Mapa{{K: "a", V: 1}}, Mapa{{K: "a", V: 2}}},
		{"1", 1},
	}
	for _, p := range no {
		if Igual(p[0], p[1]) || Igual(p[1], p[0]) {
			t.Errorf("Igual(%#v, %#v) debería ser false", p[0], p[1])
		}
	}
}

func esErrorV(v Valor) bool { _, ok := v.(ErrorV); return ok }

func TestCompararOrdenTotal(t *testing.T) {
	vals := []Valor{[]Valor{2}, "b", 3, nil, true, 2.5, false, []Valor{1, 2}, "a", -1, []Valor{1}, math.NaN(), ErrorV("e")}
	sort.SliceStable(vals, func(i, j int) bool { return Comparar(vals[i], vals[j]) < 0 })
	want := []Valor{nil, false, true, math.NaN(), -1, 2.5, 3, "a", "b", ErrorV("e"), []Valor{1}, []Valor{1, 2}, []Valor{2}}
	for i := range want {
		if Clave(vals[i]) != Clave(want[i]) {
			t.Fatalf("orden: obtuve %v, esperaba %v", vals, want)
		}
	}
	// antisymmetry and transitivity on random values
	r := rand.New(rand.NewSource(9))
	tp := ListaDe(MapaDe(TString, ListaDe(TFloat)))
	xs := make([]Valor, 60)
	for i := range xs {
		xs[i] = genValor(r, tp, 0)
	}
	for _, a := range xs {
		for _, b := range xs {
			if Comparar(a, b) != -Comparar(b, a) {
				t.Fatalf("Comparar no es antisimétrico en %#v / %#v", a, b)
			}
			for _, c := range xs[:15] {
				if Comparar(a, b) < 0 && Comparar(b, c) < 0 && Comparar(a, c) >= 0 {
					t.Fatalf("Comparar no es transitivo")
				}
			}
		}
	}
}

func TestClaveImplicaIgual(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for _, tp := range tiposPrueba {
		porClave := map[string]Valor{}
		for i := 0; i < 60; i++ {
			v := genValor(r, tp, 0)
			k := Clave(v)
			if w, ok := porClave[k]; ok && !Igual(v, w) {
				t.Fatalf("%s: misma Clave %q para %#v y %#v", tp.Go(), k, v, w)
			}
			porClave[k] = v
			if !Igual(v, Copiar(v)) || Clave(Copiar(v)) != k {
				t.Fatalf("%s: la copia de %#v no es igual", tp.Go(), v)
			}
		}
	}
	if Clave(Mapa{{K: "b", V: 2}, {K: "a", V: 1}}) != Clave(Mapa{{K: "a", V: 1}, {K: "b", V: 2}}) {
		t.Error("la Clave de un mapa debe ser canónica")
	}
	if Clave(1.0000000001) != Clave(1.0) || Clave(-0.0) != "0" || Clave(nil) != Clave([]Valor{}) {
		t.Error("Clave de floats o vacíos no es canónica")
	}
}

func TestCopiarEsProfunda(t *testing.T) {
	orig := []Valor{[]Valor{1, 2}, Mapa{{K: "a", V: []Valor{3}}}, Estructura{4}}
	c := Copiar(orig).([]Valor)
	c[0].([]Valor)[0] = 99
	c[1].(Mapa)[0].V.([]Valor)[0] = 99
	c[2].(Estructura)[0] = 99
	if !Igual(orig, []Valor{[]Valor{1, 2}, Mapa{{K: "a", V: []Valor{3}}}, Estructura{4}}) {
		t.Fatalf("Copiar compartió memoria: %#v", orig)
	}
	if Copiar([]Valor(nil)).([]Valor) != nil {
		t.Error("Copiar(nil slice) debe seguir siendo nil")
	}
}

func TestTamano(t *testing.T) {
	casos := []struct {
		v    Valor
		want int
	}{
		{-5, 5}, {"hola", 4}, {nil, 0}, {true, 1}, {[]Valor{1, 2}, 4}, {[]Valor{}, 1},
		{Mapa{{K: "ab", V: 3}}, 6}, {Estructura{1, 1}, 3}, {math.MinInt64, math.MaxInt64 / 2},
	}
	for _, c := range casos {
		if got := Tamano(c.v); got != c.want {
			t.Errorf("Tamano(%#v) = %d, esperaba %d", c.v, got, c.want)
		}
	}
}

func TestFormatoHumanoRecorta(t *testing.T) {
	larga := make([]Valor, 500)
	for i := range larga {
		larga[i] = i
	}
	s := FormatoHumano(larga, ListaDe(TInt))
	if utf8.RuneCountInString(s) != 201 || !strings.HasSuffix(s, "…") || !strings.HasPrefix(s, "[0, 1, 2") {
		t.Fatalf("recorte incorrecto (%d runas): %q", utf8.RuneCountInString(s), s)
	}
	if FormatoHumano(0.1+0.2, TFloat) != "0.3" {
		t.Errorf("0.1+0.2 se muestra %q", FormatoHumano(0.1+0.2, TFloat))
	}
	if FormatoHumano(int('\n'), TByte) != "10" || FormatoHumano(int('a'), TByte) != "'a'" {
		t.Error("formato de byte incorrecto")
	}
	if FormatoHumano(nil, Tipo{}) != "nil" || FormatoHumano([]Valor{1, "a"}, Tipo{}) == "" {
		t.Error("formato sin tipo incorrecto")
	}
	if got := FormatoGo(math.NaN(), TFloat); got != "math.NaN()" {
		t.Errorf("FormatoGo(NaN) = %q", got)
	}
	if got := FormatoGo([]Valor(nil), tLista); got != "Lista(nil)" {
		t.Errorf("FormatoGo(nil Lista) = %q", got)
	}
	if got := FormatoGo([]Valor{nil, Estructura{1, 2}}, ListaDe(PunteroA(tPunto))); got != "[]*Punto{nil, {X: 1, Y: 2}}" {
		t.Errorf("FormatoGo([]*Punto) = %q", got)
	}
}

func TestInferir(t *testing.T) {
	casos := []struct {
		texto string
		valor Valor
		tipo  string
	}{
		{"5", 5, "int"},
		{"-12", -12, "int"},
		{"3.5", 3.5, "float64"},
		{"3,5", 3.5, "float64"},
		{"true", true, "bool"},
		{"sí", true, "bool"},
		{"falso", false, "bool"},
		{"[1,2,3]", []Valor{1, 2, 3}, "[]int"},
		{"[1, 2.5]", []Valor{1.0, 2.5}, "[]float64"},
		{`["a", "b"]`, []Valor{"a", "b"}, "[]string"},
		{"[casa, árbol]", []Valor{"casa", "árbol"}, "[]string"},
		{"[[1], [2, 3]]", []Valor{[]Valor{1}, []Valor{2, 3}}, "[][]int"},
		{"[[], [2]]", []Valor{[]Valor{}, []Valor{2}}, "[][]int"},
		{"['a', 'b']", []Valor{int('a'), int('b')}, "[]rune"},
		{`"hola mundo"`, "hola mundo", "string"},
		{"«hola»", "hola", "string"},
		{"'x'", int('x'), "rune"},
		{`{"a": 1, "b": 2}`, Mapa{{K: "a", V: 1}, {K: "b", V: 2}}, "map[string]int"},
		{"{1: true}", Mapa{{K: 1, V: true}}, "map[int]bool"},
		{"casa", "casa", "string"},
		{"Go es genial", "Go es genial", "string"},
		{"1, 2, 3", []Valor{1, 2, 3}, "[]int"},
		{"[]int{4, 5}", []Valor{4, 5}, "[]int"},
		{`map[string]bool{"x": true}`, Mapa{{K: "x", V: true}}, "map[string]bool"},
	}
	for _, c := range casos {
		v, tp, err := Inferir(c.texto)
		if err != nil {
			t.Errorf("Inferir(%q): %v", c.texto, err)
			continue
		}
		if tp.Go() != c.tipo || !Igual(v, c.valor) {
			t.Errorf("Inferir(%q) = %#v %s, esperaba %#v %s", c.texto, v, tp.Go(), c.valor, c.tipo)
		}
	}
	v, tp, err := Inferir("[]")
	if err != nil || tp.Clase != CLista || tp.Elem.Clase != CInvalida || !Igual(v, []Valor{}) {
		t.Errorf("Inferir([]) = %#v %+v %v", v, tp, err)
	}
	if _, _, err := Inferir("[1, \"a\"]"); err == nil {
		t.Error("una lista con número y texto debería fallar")
	}
	if _, _, err := Inferir("   "); err == nil {
		t.Error("Inferir de nada debería fallar")
	}
}

func TestUnificarYAjustar(t *testing.T) {
	if u, ok := Unificar(TInt, TFloat); !ok || u.Clase != CFloat {
		t.Error("int+float → float")
	}
	if u, ok := Unificar(ListaDe(TInt), ListaDe(Tipo{})); !ok || !u.Igual(ListaDe(TInt)) {
		t.Error("[]int + [] → []int")
	}
	if u, ok := Unificar(MapaDe(TString, Tipo{}), MapaDe(Tipo{}, TFloat)); !ok || !u.Igual(MapaDe(TString, TFloat)) {
		t.Error("map[string]? + map[?]float64 → map[string]float64")
	}
	if _, ok := Unificar(TString, TInt); ok {
		t.Error("string+int no unifica")
	}
	if _, ok := Unificar(tCelsius, TFloat); ok {
		t.Error("Celsius+float64 no unifica")
	}
	if _, ok := Unificar(ArregloDe(2, TInt), ArregloDe(3, TInt)); ok {
		t.Error("[2]int+[3]int no unifica")
	}
	v, err := Ajustar([]Valor{1, 2}, ListaDe(TFloat))
	if err != nil || !Igual(v, []Valor{1.0, 2.0}) {
		t.Errorf("Ajustar ints a []float64: %#v %v", v, err)
	}
	if f, ok := v.([]Valor)[0].(float64); !ok || f != 1 {
		t.Errorf("Ajustar debe dar float64, dio %T", v.([]Valor)[0])
	}
	if _, err := Ajustar(300, TByte); err == nil {
		t.Error("300 no cabe en byte")
	}
	if r, err := Ajustar("a", TRune); err != nil || r != int('a') {
		t.Errorf("Ajustar(\"a\", rune) = %#v %v", r, err)
	}
	if e, err := Ajustar("boom", TError); err != nil || e != ErrorV("boom") {
		t.Errorf("Ajustar a error: %#v %v", e, err)
	}
	if _, err := Ajustar(2.5, TInt); err == nil {
		t.Error("2.5 no es entero")
	}
	if v, err := Ajustar(Mapa{{K: "b", V: 1}, {K: "a", V: 2}}, MapaDe(TString, TFloat)); err != nil || Clave(v) != `{"a":2,"b":1}` {
		t.Errorf("Ajustar mapa: %s %v", Clave(v), err)
	}
	orig := []Valor{1}
	if _, err := Ajustar(orig, ListaDe(TFloat)); err != nil || orig[0] != 1 {
		t.Error("Ajustar no debe modificar su entrada")
	}
}

func TestMapaOrdenada(t *testing.T) {
	m := Mapa{{K: 3, V: "c"}, {K: 1, V: "a"}, {K: 3, V: "z"}}
	o := m.Ordenada()
	if len(o) != 2 || o[0].K != 1 || o[1].V != "z" {
		t.Errorf("Ordenada = %#v", o)
	}
	if v, ok := o.Buscar(3); !ok || v != "z" {
		t.Errorf("Buscar(3) = %#v %v", v, ok)
	}
	if m[0].K != 3 {
		t.Error("Ordenada no debe modificar el original")
	}
}
