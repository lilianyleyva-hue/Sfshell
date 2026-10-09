package nucleo

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var actualizar = flag.Bool("actualizar", false, "reescribe los archivos golden de testdata")

func TestSondasDeterministasYAcotadas(t *testing.T) {
	firmas := []Firma{
		{Nombre: "F", Params: []Param{{"x", TInt}}, Res: []Tipo{TInt}},
		{Nombre: "F", Params: []Param{{"xs", ListaDe(TInt)}}, Res: []Tipo{TInt}},
		{Nombre: "F", Params: []Param{{"s", TString}}, Res: []Tipo{TBool}},
		{Nombre: "F", Params: []Param{{"a", TInt}, {"b", TInt}}, Res: []Tipo{TInt}},
		{Nombre: "F", Params: []Param{{"a", TBool}, {"b", TBool}}, Res: []Tipo{TBool}},
		{Nombre: "F", Params: []Param{{"m", MapaDe(TString, ListaDe(TInt))}, {"k", TString}}, Res: []Tipo{TInt}},
		{Nombre: "F", Receptor: &Param{"p", PunteroA(tPunto)}, Params: []Param{{"q", tPunto}}, Res: []Tipo{TFloat}},
		{Nombre: "F", Params: []Param{{"x", ArregloDe(3, TFloat)}, {"r", TRune}, {"b", TByte}}, Res: []Tipo{TInt}},
		{Nombre: "F", Params: []Param{{"x", ListaDe(ListaDe(ListaDe(TString)))}}, Res: []Tipo{TInt}},
		{Nombre: "F", Params: []Param{{"x", tPrivado}, {"u", Tipo{Clase: CInt, Nombre: "uint8"}}}, Res: []Tipo{TInt}},
		{Nombre: "F", Res: []Tipo{TInt}},
	}
	for _, f := range firmas {
		a, b := Sondas(f), Sondas(f)
		if len(a) == 0 || len(a) > MaxSondas {
			t.Fatalf("%s: %d sondas", f.Go(), len(a))
		}
		if len(a) != len(b) {
			t.Fatalf("%s: Sondas no es determinista", f.Go())
		}
		ent := f.Entradas()
		for i := range a {
			if a[i].Origen != OrigenSonda || a[i].ConEsperado() || len(a[i].Entradas) != len(ent) {
				t.Fatalf("%s: sonda %d mal formada: %+v", f.Go(), i, a[i])
			}
			for j, tp := range ent {
				if Clave(a[i].Entradas[j]) != Clave(b[i].Entradas[j]) {
					t.Fatalf("%s: Sondas no es determinista en %d", f.Go(), i)
				}
				if _, err := CodificarJSON(a[i].Entradas[j], tp); err != nil {
					t.Fatalf("%s: la sonda %d no se codifica como %s: %v", f.Go(), i, tp.Go(), err)
				}
			}
		}
	}
	// the values named in the spec come first
	var ints []string
	for _, c := range Sondas(firmas[0])[:9] {
		ints = append(ints, Clave(c.Entradas[0]))
	}
	if strings.Join(ints, ",") != "0,1,-1,2,3,7,-5,10,100" {
		t.Errorf("sondas de int: %v", ints)
	}
	var listas []string
	for _, c := range Sondas(firmas[1])[:10] {
		listas = append(listas, FormatoGo(c.Entradas[0], ListaDe(TInt)))
	}
	if got := strings.Join(listas, " "); got != "nil []int{} []int{0} []int{5} []int{-3} []int{1, 2, 3, 4} []int{4, 3, 2, 1} []int{2, 2, 2} []int{-5, 0, 5} []int{1, 3, 5, 7, 9, 2}" {
		t.Errorf("sondas de []int: %s", got)
	}
	var textos []string
	for _, c := range Sondas(firmas[2])[:10] {
		textos = append(textos, c.Entradas[0].(string))
	}
	if strings.Join(textos, "|") != "|a|hola|Hola Mundo|ñandú ÁÉ|  x  |a,b,,c|12345|anilina|Go es genial" {
		t.Errorf("sondas de string: %q", textos)
	}
	if n := len(Sondas(firmas[0])); n != 30 {
		t.Errorf("una entrada int debería dar 30 sondas, da %d", n)
	}
	// zipping: two ints never get the same pair twice and are not always equal
	pares := map[string]bool{}
	iguales := 0
	for _, c := range Sondas(firmas[3]) {
		k := Clave([]Valor(c.Entradas))
		if pares[k] {
			t.Errorf("par repetido %s", k)
		}
		pares[k] = true
		if Igual(c.Entradas[0], c.Entradas[1]) {
			iguales++
		}
	}
	if iguales > 0 {
		t.Errorf("las sondas de (int, int) no deberían repetir el mismo valor en las dos entradas")
	}
	// small domains use the full product
	if n := len(Sondas(firmas[4])); n != 4 {
		t.Errorf("(bool, bool) debería dar las 4 combinaciones, da %d", n)
	}
	if n := len(Sondas(firmas[10])); n != 1 {
		t.Errorf("una función sin entradas tiene 1 sonda, da %d", n)
	}
	// probes are independent copies
	s := Sondas(firmas[1])
	s[5].Entradas[0].([]Valor)[0] = 999
	if Sondas(firmas[1])[5].Entradas[0].([]Valor)[0] != 1 {
		t.Error("modificar una sonda no debe cambiar las siguientes")
	}
	if Sondas(Firma{Nombre: "G", Params: []Param{{"e", TError}}, Res: []Tipo{TInt}}) != nil {
		t.Error("una entrada error no tiene sondas")
	}
}

// Reference implementations used to pin the fingerprint format.
func sumaPares(xs []Valor) Valor {
	t := 0
	for _, x := range xs {
		if x.(int)%2 == 0 {
			t += x.(int)
		}
	}
	return t
}

func longitud(s string) Valor { return len(s) }

func salidasDe(f Firma, fn func(in []Valor) []Valor) [][]Valor {
	var out [][]Valor
	for _, c := range Sondas(f) {
		out = append(out, fn(c.Entradas))
	}
	return out
}

func TestHuellaEstableYGolden(t *testing.T) {
	fSuma := Firma{Nombre: "SumaPares", Params: []Param{{"nums", ListaDe(TInt)}}, Res: []Tipo{TInt}}
	fLen := Firma{Nombre: "Longitud", Params: []Param{{"s", TString}}, Res: []Tipo{TInt}}
	fMax := Firma{Nombre: "Max", Params: []Param{{"a", TInt}, {"b", TInt}}, Res: []Tipo{TInt}}
	fMin := Firma{Nombre: "Min", Params: []Param{{"a", TInt}, {"b", TInt}}, Res: []Tipo{TInt}}
	fDiv := Firma{Nombre: "Div", Params: []Param{{"a", TFloat}, {"b", TFloat}}, Res: []Tipo{TFloat, TError}}
	huellas := map[string]string{
		"SumaPares": Huella(fSuma, salidasDe(fSuma, func(in []Valor) []Valor { return []Valor{sumaPares(in[0].([]Valor))} })),
		"Longitud":  Huella(fLen, salidasDe(fLen, func(in []Valor) []Valor { return []Valor{longitud(in[0].(string))} })),
		"Max": Huella(fMax, salidasDe(fMax, func(in []Valor) []Valor {
			return []Valor{max(in[0].(int), in[1].(int))}
		})),
		"Min": Huella(fMin, salidasDe(fMin, func(in []Valor) []Valor {
			return []Valor{min(in[0].(int), in[1].(int))}
		})),
		"Div": Huella(fDiv, salidasDe(fDiv, func(in []Valor) []Valor {
			if in[1].(float64) == 0 {
				return []Valor{0.0, ErrorV("división por cero")}
			}
			return []Valor{in[0].(float64) / in[1].(float64), nil}
		})),
		"Panico": Huella(fLen, [][]Valor{nil, {1}, nil}),
	}
	// stable across runs
	if again := Huella(fSuma, salidasDe(fSuma, func(in []Valor) []Valor { return []Valor{sumaPares(in[0].([]Valor))} })); again != huellas["SumaPares"] {
		t.Fatal("Huella no es estable")
	}
	if huellas["Max"] == huellas["Min"] {
		t.Error("Max y Min no deberían tener la misma huella")
	}
	if len(huellas["SumaPares"]) != 64 {
		t.Errorf("la huella debe ser sha256 en hex: %q", huellas["SumaPares"])
	}
	// equal outputs with different spelling (nil vs empty, unordered map) give the same fingerprint
	f := Firma{Nombre: "X", Res: []Tipo{ListaDe(TInt)}}
	if Huella(f, [][]Valor{{[]Valor(nil)}}) != Huella(f, [][]Valor{{[]Valor{}}}) {
		t.Error("nil y [] deben dar la misma huella")
	}
	var nombres []string
	for n := range huellas {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	var sb strings.Builder
	for _, n := range nombres {
		fmt.Fprintf(&sb, "%s %s\n", n, huellas[n])
	}
	ruta := filepath.Join("testdata", "huella.golden")
	if *actualizar {
		if err := os.WriteFile(ruta, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	golden, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s (ejecuta go test -run Huella -actualizar): %v", ruta, err)
	}
	if string(golden) != sb.String() {
		t.Fatalf("las huellas cambiaron; Sondas o Huella no deben cambiar nunca.\nobtenido:\n%s\ngolden:\n%s", sb.String(), golden)
	}
}
