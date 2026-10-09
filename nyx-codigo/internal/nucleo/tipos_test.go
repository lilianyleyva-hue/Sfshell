package nucleo

import (
	"encoding/json"
	"testing"
)

func TestTipoGoYParseTipo15(t *testing.T) {
	casos := []struct {
		texto string
		tipo  Tipo
	}{
		{"int", TInt},
		{"int64", Tipo{Clase: CInt, Nombre: "int64"}},
		{"uint8", Tipo{Clase: CInt, Nombre: "uint8"}},
		{"float64", TFloat},
		{"float32", Tipo{Clase: CFloat, Nombre: "float32"}},
		{"bool", TBool},
		{"string", TString},
		{"rune", TRune},
		{"byte", TByte},
		{"error", TError},
		{"[]string", ListaDe(TString)},
		{"[][]int", ListaDe(ListaDe(TInt))},
		{"[3]float64", ArregloDe(3, TFloat)},
		{"map[string][]int", MapaDe(TString, ListaDe(TInt))},
		{"map[rune]bool", MapaDe(TRune, TBool)},
		{"struct{ X int; Y string }", Tipo{Clase: CStruct, Campos: []Campo{{"X", TInt}, {"Y", TString}}}},
		{"*struct{ A []int }", PunteroA(Tipo{Clase: CStruct, Campos: []Campo{{"A", ListaDe(TInt)}}})},
		{"struct{}", Tipo{Clase: CStruct}},
	}
	for _, c := range casos {
		if g := c.tipo.Go(); g != c.texto {
			t.Errorf("Go() = %q, esperaba %q", g, c.texto)
		}
		p, err := ParseTipo(c.texto)
		if err != nil {
			t.Errorf("ParseTipo(%q): %v", c.texto, err)
			continue
		}
		if !p.Igual(c.tipo) {
			t.Errorf("ParseTipo(%q) = %+v, esperaba %+v", c.texto, p, c.tipo)
		}
		if p.ClaveTipo() != c.tipo.ClaveTipo() {
			t.Errorf("ClaveTipo distinta para %q", c.texto)
		}
		// JSON round trip of the type itself
		datos, _ := json.Marshal(c.tipo)
		var vuelta Tipo
		if err := json.Unmarshal(datos, &vuelta); err != nil || !vuelta.Igual(c.tipo) {
			t.Errorf("JSON de %q no vuelve igual: %s", c.texto, datos)
		}
	}
	if p, err := ParseTipo(" map[ string ] [ ]int "); err != nil || p.Go() != "map[string][]int" {
		t.Errorf("ParseTipo con espacios: %v %v", p.Go(), err)
	}
	if p, err := ParseTipo("struct{ X, Y int }"); err != nil || p.Go() != "struct{ X int; Y int }" {
		t.Errorf("ParseTipo con campos agrupados: %v %v", p.Go(), err)
	}
	for _, malo := range []string{"Punto", "chan int", "func()", "interface{}", "any", "[]", "map[string]", "[x]int", "*int", "int int", "List[int]"} {
		if _, err := ParseTipo(malo); err == nil {
			t.Errorf("ParseTipo(%q) debería fallar", malo)
		}
	}
}

func TestTipoNombrados(t *testing.T) {
	if tPunto.Go() != "Punto" || PunteroA(tPunto).Go() != "*Punto" || ListaDe(tPunto).Go() != "[]Punto" {
		t.Error("Go() de tipos con nombre")
	}
	if tPunto.Cero() != "Punto{}" || tCelsius.Cero() != "Celsius(0.0)" || tLista.Cero() != "Lista(nil)" {
		t.Errorf("Cero de tipos con nombre: %s %s %s", tPunto.Cero(), tCelsius.Cero(), tLista.Cero())
	}
	ceros := map[string]Tipo{"0": TInt, "0.0": TFloat, "false": TBool, `""`: TString, "nil": ListaDe(TInt),
		"[3]int{}": ArregloDe(3, TInt), "struct{ X int }{}": {Clase: CStruct, Campos: []Campo{{"X", TInt}}}}
	for want, tp := range ceros {
		if tp.Cero() != want {
			t.Errorf("%s.Cero() = %q, esperaba %q", tp.Go(), tp.Cero(), want)
		}
	}
	otro := tPunto
	otro.Campos = []Campo{{"X", TInt}}
	if otro.Igual(tPunto) || otro.ClaveTipo() == tPunto.ClaveTipo() {
		t.Error("dos Punto con campos distintos no son iguales")
	}
	if (Tipo{Clase: CInt}).Igual(TInt) == false || (Tipo{Clase: CInt}).Go() != "int" {
		t.Error("un Nombre vacío equivale al nombre por defecto")
	}
	if TRune.Igual(Tipo{Clase: CInt, Nombre: "int32"}) {
		t.Error("rune e int32 son clases distintas en Nyx")
	}
}

func TestTipoCodificableYBasico(t *testing.T) {
	si := []Tipo{TInt, TError, ListaDe(tPunto), MapaDe(TBool, TString), PunteroA(tPunto), ArregloDe(0, TInt)}
	no := []Tipo{{}, ListaDe(TError), MapaDe(TFloat, TInt), MapaDe(ListaDe(TInt), TInt), PunteroA(TInt), {Clase: CLista},
		{Clase: CStruct, Campos: []Campo{{"E", TError}}}}
	for _, tp := range si {
		if !tp.Codificable() {
			t.Errorf("%s debería ser codificable", tp.Go())
		}
	}
	for _, tp := range no {
		if tp.Codificable() {
			t.Errorf("%s no debería ser codificable", tp.Go())
		}
	}
	if !TByte.Basico() || ListaDe(TInt).Basico() || TError.Basico() {
		t.Error("Basico")
	}
}

func TestTipoHumano(t *testing.T) {
	casos := map[string]Tipo{
		"lista de números enteros":            ListaDe(TInt),
		"texto":                               TString,
		"número decimal":                      TFloat,
		"mapa de texto a número":              MapaDe(TString, TInt),
		"lista de listas de textos":           ListaDe(ListaDe(TString)),
		"arreglo de 3 números decimales":      ArregloDe(3, TFloat),
		"estructura Punto":                    tPunto,
		"puntero a estructura Punto":          PunteroA(tPunto),
		"verdadero o falso":                   TBool,
		"carácter":                            TRune,
		"mapa de texto a lista de caracteres": MapaDe(TString, ListaDe(TRune)),
	}
	for want, tp := range casos {
		if got := tp.Humano(); got != want {
			t.Errorf("%s.Humano() = %q, esperaba %q", tp.Go(), got, want)
		}
	}
}

func TestFirmaGo(t *testing.T) {
	casos := []struct {
		f    Firma
		want string
	}{
		{Firma{Nombre: "SumaPares", Params: []Param{{"nums", ListaDe(TInt)}}, Res: []Tipo{TInt}}, "func SumaPares(nums []int) int"},
		{Firma{Nombre: "Dist", Receptor: &Param{"p", tPunto}, Params: []Param{{"q", tPunto}}, Res: []Tipo{TFloat, TError}},
			"func (p Punto) Dist(q Punto) (float64, error)"},
		{Firma{Nombre: "Mover", Receptor: &Param{"p", PunteroA(tPunto)}, Params: []Param{{"dx", TInt}, {"dy", TInt}}},
			"func (p *Punto) Mover(dx, dy int)"},
		{Firma{Nombre: "Max", Params: []Param{{"xs", ListaDe(TInt)}}, Res: []Tipo{TInt}, Variadica: true}, "func Max(xs ...int) int"},
		{Firma{Nombre: "Unir", Params: []Param{{"sep", TString}, {"partes", ListaDe(TString)}}, Res: []Tipo{TString}, Variadica: true},
			"func Unir(sep string, partes ...string) string"},
		{Firma{Nombre: "F", Params: []Param{{"", TInt}, {"", TString}}, Res: []Tipo{TBool}}, "func F(int, string) bool"},
		{Firma{Nombre: "Cuenta", Params: []Param{{"s", TString}}, Res: []Tipo{MapaDe(TRune, TInt)}}, "func Cuenta(s string) map[rune]int"},
		{Firma{Nombre: "Div", Params: []Param{{"a", TInt}, {"b", TInt}, {"c", TFloat}}, Res: []Tipo{TInt, TInt, TError}},
			"func Div(a, b int, c float64) (int, int, error)"},
	}
	for _, c := range casos {
		if got := c.f.Go(); got != c.want {
			t.Errorf("Go() = %q, esperaba %q", got, c.want)
		}
	}
	formas := map[string]Firma{
		"([]int)int":                   casos[0].f,
		"(Punto,Punto)(float64,error)": casos[1].f,
		"(...int)int":                  casos[3].f,
		"(string,...string)string":     casos[4].f,
		"(*Punto,int,int)":             casos[2].f,
	}
	for want, f := range formas {
		if got := f.Forma(); got != want {
			t.Errorf("Forma() = %q, esperaba %q", got, want)
		}
	}
	if len(casos[1].f.Entradas()) != 2 || !casos[1].f.Entradas()[0].Igual(tPunto) {
		t.Error("Entradas: el receptor va primero")
	}
}

func TestFirmaProbable(t *testing.T) {
	si := []Firma{
		{Nombre: "A", Params: []Param{{"x", TInt}}, Res: []Tipo{TInt}},
		{Nombre: "B", Receptor: &Param{"p", PunteroA(tPunto)}, Res: []Tipo{TInt, TError}},
		{Nombre: "C", Params: []Param{{"xs", ListaDe(TInt)}}, Res: []Tipo{TInt}, Variadica: true},
	}
	no := []Firma{
		{Nombre: "SinRes", Params: []Param{{"x", TInt}}},
		{Nombre: "ErrorEntrada", Params: []Param{{"e", TError}}, Res: []Tipo{TInt}},
		{Nombre: "ErrorPrimero", Res: []Tipo{TError, TInt}},
		{Nombre: "Muchas", Params: []Param{{"a", TInt}, {"b", TInt}, {"c", TInt}, {"d", TInt}, {"e", TInt}, {"f", TInt}, {"g", TInt}}, Res: []Tipo{TInt}},
		{Nombre: "CuatroRes", Res: []Tipo{TInt, TInt, TInt, TInt}},
		{Nombre: "Raro", Params: []Param{{"x", Tipo{}}}, Res: []Tipo{TInt}},
		{Nombre: "Receptor", Receptor: &Param{"x", TInt}, Res: []Tipo{TInt}},
	}
	for _, f := range si {
		if !f.Probable() {
			t.Errorf("%s debería ser probable", f.Go())
		}
	}
	for _, f := range no {
		if f.Probable() {
			t.Errorf("%s no debería ser probable", f.Go())
		}
	}
}
