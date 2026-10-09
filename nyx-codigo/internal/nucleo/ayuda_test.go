package nucleo

import (
	"math"
	"math/rand"
	"strings"
)

// Test helpers shared by the nucleo tests: a declared struct type, 30 test types and a random value generator.

var tPunto = Tipo{Clase: CStruct, Nombre: "Punto", Definido: true, Campos: []Campo{
	{Nombre: "X", Tipo: TInt}, {Nombre: "Y", Tipo: TInt},
}}

var tPrivado = Tipo{Clase: CStruct, Nombre: "registro", Definido: true, Campos: []Campo{
	{Nombre: "nombre", Tipo: TString}, {Nombre: "edad", Tipo: Tipo{Clase: CInt, Nombre: "uint8"}},
	{Nombre: "notas", Tipo: ListaDe(TFloat)},
}}

var tCelsius = Tipo{Clase: CFloat, Nombre: "Celsius", Definido: true}
var tLista = Tipo{Clase: CLista, Nombre: "Lista", Definido: true, Elem: &TInt}

// tiposPrueba are the 30 types used by the codec and formatting round-trip tests.
var tiposPrueba = []Tipo{
	TInt,
	{Clase: CInt, Nombre: "int8"},
	{Clase: CInt, Nombre: "int64"},
	{Clase: CInt, Nombre: "uint"},
	{Clase: CInt, Nombre: "uint16"},
	TFloat,
	{Clase: CFloat, Nombre: "float32"},
	TBool,
	TString,
	TRune,
	TByte,
	TError,
	ListaDe(TInt),
	ListaDe(TString),
	ListaDe(TFloat),
	ListaDe(ListaDe(TInt)),
	ArregloDe(3, TInt),
	ArregloDe(2, TString),
	MapaDe(TString, TInt),
	MapaDe(TInt, TString),
	MapaDe(TString, ListaDe(TInt)),
	MapaDe(TRune, TBool),
	MapaDe(TBool, TFloat),
	tPunto,
	tPrivado,
	PunteroA(tPunto),
	ListaDe(tPunto),
	MapaDe(TString, tPunto),
	tCelsius,
	tLista,
}

// textosRaros includes invalid UTF-8, quotes, escapes and HTML characters.
var textosRaros = []string{"", "hola", "ñandú ÁÉ", "a\"b", `c:\dir`, "línea\nnueva\ttab", "<b>&</b>",
	"\xff\xfe", "ok\x80", "\u2028", "emoji 🙂", "\x00", "{\"b64\":\"x\"}"}

func genValor(r *rand.Rand, t Tipo, prof int) Valor {
	switch t.Clase {
	case CInt, CRune, CByte:
		lo, hi := rangoEntero(t)
		switch r.Intn(6) {
		case 0:
			return int(lo)
		case 1:
			return int(hi)
		case 2:
			return 0
		}
		if hi-lo < 0 || hi-lo > 1<<40 { // wide types: values near zero or anywhere
			if r.Intn(2) == 0 {
				if lo == 0 {
					return int(r.Int63n(1001))
				}
				return int(r.Int63n(2001) - 1000)
			}
			v := r.Int63()
			if lo < 0 && r.Intn(2) == 0 {
				v = -v
			}
			return int(v)
		}
		return int(lo + r.Int63n(hi-lo+1))
	case CFloat:
		switch r.Intn(8) {
		case 0:
			return math.NaN()
		case 1:
			return math.Inf(1 - 2*r.Intn(2))
		case 2:
			return 0.0
		case 3:
			return float64(r.Intn(200) - 100)
		}
		f := r.NormFloat64() * math.Pow(10, float64(r.Intn(12)-4))
		if t.Nombre == "float32" {
			f = float64(float32(f))
		}
		return f
	case CBool:
		return r.Intn(2) == 1
	case CString:
		if r.Intn(3) == 0 {
			return textosRaros[r.Intn(len(textosRaros))]
		}
		n := r.Intn(8)
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteRune([]rune("abcxyzñá ,.Z09")[r.Intn(14)])
		}
		return sb.String()
	case CError:
		if r.Intn(2) == 0 {
			return nil
		}
		return ErrorV(textosRaros[r.Intn(len(textosRaros))])
	case CLista:
		if r.Intn(6) == 0 {
			return []Valor(nil)
		}
		n := r.Intn(5)
		if prof > 2 {
			n = r.Intn(2)
		}
		out := make([]Valor, n)
		for i := range out {
			out[i] = genValor(r, *t.Elem, prof+1)
		}
		return out
	case CArreglo:
		out := make([]Valor, t.Largo)
		for i := range out {
			out[i] = genValor(r, *t.Elem, prof+1)
		}
		return out
	case CMapa:
		if r.Intn(6) == 0 {
			return Mapa(nil)
		}
		n := r.Intn(4)
		m := Mapa{}
		for i := 0; i < n; i++ {
			m = append(m, Par{K: genValor(r, *t.Clave, prof+1), V: genValor(r, *t.Elem, prof+1)})
		}
		return m.Ordenada()
	case CStruct:
		out := make(Estructura, len(t.Campos))
		for i, c := range t.Campos {
			out[i] = genValor(r, c.Tipo, prof+1)
		}
		return out
	case CPuntero:
		if r.Intn(3) == 0 {
			return nil
		}
		return genValor(r, *t.Elem, prof+1)
	}
	return nil
}
