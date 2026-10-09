package nucleo

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"testing"
)

func TestCodecIdaYVuelta30Tipos(t *testing.T) {
	if len(tiposPrueba) != 30 {
		t.Fatalf("se esperaban 30 tipos de prueba, hay %d", len(tiposPrueba))
	}
	r := rand.New(rand.NewSource(42))
	vistos := map[string]bool{}
	for _, tp := range tiposPrueba {
		if !tp.Codificable() {
			t.Fatalf("%s debería ser codificable", tp.Go())
		}
		for i := 0; i < 20; i++ {
			v := genValor(r, tp, 0)
			m, err := CodificarJSON(v, tp)
			if err != nil {
				t.Fatalf("%s: codificar %#v: %v", tp.Go(), v, err)
			}
			if !json.Valid(m) {
				t.Fatalf("%s: JSON no válido: %s", tp.Go(), m)
			}
			w, err := DecodificarJSON(m, tp)
			if err != nil {
				t.Fatalf("%s: decodificar %s: %v", tp.Go(), m, err)
			}
			if !Igual(v, w) {
				t.Fatalf("%s: ida y vuelta distinta:\n  antes %#v\n  JSON  %s\n  después %#v", tp.Go(), v, m, w)
			}
			m2, err := CodificarJSON(w, tp)
			if err != nil || string(m2) != string(m) {
				t.Fatalf("%s: la codificación no es estable: %s / %s (%v)", tp.Go(), m, m2, err)
			}
			if strings.Contains(string(m), "b64") {
				vistos["b64"] = true
			}
			if strings.Contains(string(m), "NaN") {
				vistos["NaN"] = true
			}
		}
	}
	for _, k := range []string{"b64", "NaN"} {
		if !vistos[k] {
			t.Errorf("las pruebas al azar no cubrieron %s", k)
		}
	}
}

func TestCodecFormatoExacto(t *testing.T) {
	casos := []struct {
		v    Valor
		t    Tipo
		json string
	}{
		{5, TInt, `5`},
		{-3, Tipo{Clase: CInt, Nombre: "int8"}, `-3`},
		{2.5, TFloat, `2.5`},
		{3.0, TFloat, `3`},
		{math.NaN(), TFloat, `"NaN"`},
		{math.Inf(1), TFloat, `"+Inf"`},
		{math.Inf(-1), TFloat, `"-Inf"`},
		{true, TBool, `true`},
		{"hola", TString, `"hola"`},
		{"<a&b>", TString, `"<a&b>"`},
		{"línea\n\"x\"", TString, `"línea\n\"x\""`},
		{"\xff", TString, `{"b64":"/w=="}`},
		{int('a'), TRune, `97`},
		{[]Valor(nil), ListaDe(TInt), `null`},
		{[]Valor{}, ListaDe(TInt), `[]`},
		{[]Valor{1, 2}, ListaDe(TInt), `[1,2]`},
		{[]Valor{1, 2, 3}, ArregloDe(3, TInt), `[1,2,3]`},
		{Mapa{{K: "b", V: 2}, {K: "a", V: 1}}, MapaDe(TString, TInt), `[["a",1],["b",2]]`},
		{Mapa(nil), MapaDe(TString, TInt), `null`},
		{Estructura{1, 2}, tPunto, `[1,2]`},
		{nil, PunteroA(tPunto), `null`},
		{Estructura{1, 2}, PunteroA(tPunto), `[1,2]`},
		{nil, TError, `null`},
		{ErrorV("no hay"), TError, `{"error":"no hay"}`},
	}
	for _, c := range casos {
		m, err := CodificarJSON(c.v, c.t)
		if err != nil {
			t.Errorf("%s %#v: %v", c.t.Go(), c.v, err)
			continue
		}
		if string(m) != c.json {
			t.Errorf("%s %#v: obtuve %s, esperaba %s", c.t.Go(), c.v, m, c.json)
		}
		w, err := DecodificarJSON(json.RawMessage(c.json), c.t)
		if err != nil || !Igual(w, c.v) {
			t.Errorf("decodificar %s como %s: %#v, %v", c.json, c.t.Go(), w, err)
		}
	}
}

func TestCodecErrores(t *testing.T) {
	malos := []struct {
		json string
		t    Tipo
	}{
		{`300`, TByte},
		{`-1`, Tipo{Clase: CInt, Nombre: "uint"}},
		{`1.5`, TInt},
		{`18446744073709551615`, Tipo{Clase: CInt, Nombre: "uint64"}},
		{`[1,2]`, ArregloDe(3, TInt)},
		{`null`, ArregloDe(3, TInt)},
		{`[1]`, tPunto},
		{`"x"`, TInt},
		{`{"b64":"***"}`, TString},
		{`[1] x`, ListaDe(TInt)},
		{`[[1]]`, MapaDe(TInt, TInt)},
		{`"error"`, TError},
	}
	for _, c := range malos {
		if v, err := DecodificarJSON(json.RawMessage(c.json), c.t); err == nil {
			t.Errorf("decodificar %s como %s debería fallar; dio %#v", c.json, c.t.Go(), v)
		}
	}
	if _, err := CodificarJSON("x", TInt); err == nil {
		t.Error("codificar un texto como int debería fallar")
	}
	if _, err := CodificarJSON([]Valor{1}, ArregloDe(2, TInt)); err == nil {
		t.Error("codificar un arreglo de largo incorrecto debería fallar")
	}
}

func TestBase64ComoLaBiblioteca(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for n := 0; n < 40; n++ {
		b := make([]byte, n)
		r.Read(b)
		propio := base64Codificar(b)
		if want := base64.StdEncoding.EncodeToString(b); propio != want {
			t.Fatalf("base64(%x) = %q, la biblioteca da %q", b, propio, want)
		}
		vuelta, err := base64Decodificar(propio)
		if err != nil || string(vuelta) != string(b) {
			t.Fatalf("base64 vuelta de %q: %x, %v", propio, vuelta, err)
		}
	}
	for _, malo := range []string{"a", "ab=c", "a===", "@@@@", "QQ==QQ=="} {
		if _, err := base64Decodificar(malo); err == nil {
			t.Errorf("base64Decodificar(%q) debería fallar", malo)
		}
	}
}

func TestGuardarYLeerCasos(t *testing.T) {
	f := Firma{Nombre: "Dividir", Params: []Param{{"a", TInt}, {"b", TInt}}, Res: []Tipo{TInt, TError}}
	cs := []Caso{
		{Entradas: []Valor{6, 3}, Esperado: []Valor{2, nil}, Expectativa: EspUsuario, Origen: OrigenUsuario, Nota: "tu ejemplo 1"},
		{Entradas: []Valor{1, 0}, Esperado: []Valor{0, ErrorV("división por cero")}, Expectativa: EspReferencia, Origen: OrigenBorde},
		{Entradas: []Valor{5, 5}, Origen: OrigenAzar},
	}
	gs, err := GuardarCasos(f, cs)
	if err != nil {
		t.Fatal(err)
	}
	datos, err := json.Marshal(gs)
	if err != nil {
		t.Fatal(err)
	}
	var gs2 []CasoGuardado
	if err := json.Unmarshal(datos, &gs2); err != nil {
		t.Fatal(err)
	}
	vuelta, err := LeerCasos(f, gs2)
	if err != nil {
		t.Fatal(err)
	}
	if len(vuelta) != len(cs) {
		t.Fatalf("leí %d casos de %d", len(vuelta), len(cs))
	}
	for i := range cs {
		a, b := cs[i], vuelta[i]
		if !Igual([]Valor(a.Entradas), []Valor(b.Entradas)) || a.Expectativa != b.Expectativa || a.Origen != b.Origen || a.Nota != b.Nota {
			t.Errorf("caso %d distinto: %+v / %+v", i, a, b)
		}
		if (a.Esperado == nil) != (b.Esperado == nil) || !Igual([]Valor(a.Esperado), []Valor(b.Esperado)) {
			t.Errorf("caso %d: esperado distinto: %#v / %#v", i, a.Esperado, b.Esperado)
		}
	}
	if !vuelta[0].ConEsperado() || vuelta[2].ConEsperado() {
		t.Error("ConEsperado no coincide tras leer")
	}
	if _, err := GuardarCasos(f, []Caso{{Entradas: []Valor{1}}}); err == nil {
		t.Error("un caso con menos entradas debería fallar")
	}
}
