package nucleo

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Origen says where a case's INPUTS came from.
type Origen string

const (
	OrigenUsuario       Origen = "usuario"       // typed by the user
	OrigenContraejemplo Origen = "contraejemplo" // from "está mal" feedback, stored in memory
	OrigenBorde         Origen = "borde"         // boundary values
	OrigenAzar          Origen = "azar"          // random, seeded
	OrigenSonda         Origen = "sonda"         // fixed probe inputs (Sondas)
	OrigenReceta        Origen = "receta"        // shipped with a recipe
	OrigenWeb           Origen = "web"
)

// Expectativa says where a case's EXPECTED OUTPUT came from. This is what fixes the circular-badge problem.
type Expectativa string

const (
	EspNinguna        Expectativa = ""               // no expected value: only properties / no panic / determinism
	EspUsuario        Expectativa = "usuario"        // the user said so (examples, answers to questions, counterexamples)
	EspInterpretacion Expectativa = "interpretacion" // my reading of the request (DSL program from the Spanish frame)
	EspReferencia     Expectativa = "referencia"     // another implementation: stored verified function, recipe, original code, web consensus
)

type Caso struct {
	Entradas    []Valor     `json:"-"`
	Esperado    []Valor     `json:"-"` // one per result; nil when Expectativa == EspNinguna
	Expectativa Expectativa `json:"expectativa,omitempty"`
	Origen      Origen      `json:"origen"`
	Nota        string      `json:"nota,omitempty"` // "tu ejemplo 2", "lo que entendí", URL…
}

// ConEsperado reports whether the case carries an expected output.
func (c Caso) ConEsperado() bool {
	return c.Expectativa != EspNinguna && c.Esperado != nil
}

// CasoPrograma is one run of a whole program.
type CasoPrograma struct {
	Entrada     string      `json:"entrada"` // stdin
	Args        []string    `json:"args,omitempty"`
	Esperado    string      `json:"esperado,omitempty"`
	Comparar    string      `json:"comparar,omitempty"` // "exacto" | "lineas" (trim trailing spaces, ignore final newline) | "contiene" | "numeros" (compare the sequence of numbers printed)
	Expectativa Expectativa `json:"expectativa,omitempty"`
	Nota        string      `json:"nota,omitempty"`
}

// Propiedad is a Go boolean expression compiled into the harness. Free names:
//
//	e0..eN  copies of the inputs (receiver first), r0..rM results, F(...) calls the same variant,
//	helpers nyx__Ordenada, nyx__EsPermutacion, nyx__Subsecuencia, nyx__Contiene, nyx__Igual (harness support file).
type Propiedad struct {
	Nombre      string `json:"nombre"`             // "ordenada"
	Explicacion string `json:"explicacion"`        // "el resultado está ordenado"
	Expr        string `json:"expr"`               // "nyx__Ordenada(r0) && nyx__EsPermutacion(e0, r0)"
	Requiere    string `json:"requiere,omitempty"` // Go bool precondition, e.g. "len(e0) > 0"; the case is skipped if false
}

// ---- probes ----

// MaxSondas is the largest number of probe cases Sondas returns.
const MaxSondas = 30

// The probe lists below are frozen: they define fingerprints (Huella), so they must never change.
// The first values of each list are the ones named in §3.4; the rest extend it to 30 values.

var sondasInt = []int{
	0, 1, -1, 2, 3, 7, -5, 10, 100,
	4, 5, 6, 8, 9, 11, 12, 13, 15, 16, 20, 21, 25, 30, 42, 64, 99, -2, -3, -10, -100,
}

var sondasFloat = []float64{
	0, 1, -1, 0.5, 2.5, 3, 7, -5.5, 10, 100,
	0.1, 0.25, 1.5, 2, 3.14159, -0.5, -2.25, 4.75, 6, 9.99,
	12.5, 20, 33.3, 50, 99.5, -10, -100, 0.001, 1234.5678, 1e6,
}

var sondasString = []string{
	"", "a", "hola", "Hola Mundo", "ñandú ÁÉ", "  x  ", "a,b,,c", "12345", "anilina", "Go es genial",
	"b", "ab", "aba", "abc", "Ana", "reconocer", "MAYÚSCULAS", "uno dos tres", "x y", "a1b2",
	"zz", "hola hola", "123", "  ", "Árbol", "go", "Ejemplo, con: signos!", "aeiou", "xyz", "Dábale arroz",
}

var sondasRune = []rune{
	'a', 'b', 'z', 'A', 'Z', '0', '5', '9', ' ', 'ñ',
	'é', 'Á', '.', ',', '!', '?', '\n', '\t', 'x', 'm',
	'e', 'i', 'o', 'u', 'E', 'Ñ', 'ü', '1', '-', '_',
}

var sondasByte = []int{
	'a', 'b', 'z', 'A', 'Z', '0', '5', '9', ' ', '.',
	',', '!', '?', '\n', '\t', 'x', 'm', 'e', 'i', 'o',
	'u', 'E', '1', '-', '_', 0, 127, 128, 200, 255,
}

var sondasListaInt = [][]int{
	nil, {}, {0}, {5}, {-3}, {1, 2, 3, 4}, {4, 3, 2, 1}, {2, 2, 2}, {-5, 0, 5}, {1, 3, 5, 7, 9, 2},
	{1}, {2}, {1, 2}, {2, 1}, {3, 3}, {0, 0, 0}, {10, 20, 30}, {-1, -2, -3}, {7, 1, 7}, {1, 1, 2, 3, 5, 8},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}, {100, -100}, {1, 2, 1}, {5, 4, 5, 4}, {2, 4, 6, 8}, {1, 3, 5}, {-7}, {0, 1}, {42, 7, 42, 3}, {6, 1, 8, 3, 2, 9, 4, 7, 5, 0},
}

// patronesLista are indices into the element probes; the lists also start with nil and [].
var patronesLista = [][]int{
	{1}, {2}, {0}, {1, 2, 3}, {3, 2, 1}, {2, 2, 2}, {4, 0, 5}, {1, 3, 5, 7, 2}, {5}, {6},
	{1, 2}, {2, 1}, {3, 3}, {0, 0, 0}, {7, 8, 9}, {10, 11, 12}, {4, 1, 4}, {1, 1, 2, 3, 5},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}, {13, 14}, {1, 2, 1}, {5, 4, 5, 4}, {15, 16, 17, 18}, {19},
	{20, 21}, {22, 23, 24, 25}, {26, 27, 28, 29}, {0, 1, 2, 3, 4, 5, 6, 7},
}

// sondasTipo returns up to MaxSondas distinct probe values of type t (deterministic).
func sondasTipo(t Tipo, prof int) []Valor {
	var out []Valor
	switch t.Clase {
	case CInt:
		if sinSigno(t) {
			for _, i := range sondasInt {
				if i < 0 {
					i = -i
				}
				out = append(out, i)
			}
		} else {
			for _, i := range sondasInt {
				out = append(out, i)
			}
		}
	case CFloat:
		for _, f := range sondasFloat {
			out = append(out, f)
		}
	case CBool:
		out = []Valor{false, true}
	case CString:
		for _, s := range sondasString {
			out = append(out, s)
		}
	case CRune:
		for _, r := range sondasRune {
			out = append(out, int(r))
		}
	case CByte:
		for _, b := range sondasByte {
			out = append(out, b)
		}
	case CLista:
		e := elemDe(t.Elem)
		if e.Clase == CInt && !sinSigno(e) && prof == 0 {
			for _, l := range sondasListaInt {
				if l == nil {
					out = append(out, []Valor(nil))
					continue
				}
				xs := make([]Valor, len(l))
				for i, x := range l {
					xs[i] = x
				}
				out = append(out, xs)
			}
			break
		}
		p := sondasTipo(e, prof+1)
		out = append(out, []Valor(nil), []Valor{})
		if len(p) == 0 {
			break
		}
		for _, pat := range patronesLista {
			if prof > 0 && len(pat) > 3 {
				pat = pat[:3]
			}
			xs := make([]Valor, len(pat))
			for i, k := range pat {
				xs[i] = Copiar(p[k%len(p)])
			}
			out = append(out, xs)
		}
	case CArreglo:
		e := elemDe(t.Elem)
		if t.Largo == 0 {
			return []Valor{[]Valor{}}
		}
		p := sondasTipo(e, prof+1)
		out = append(out, ValorCero(t))
		if len(p) == 0 {
			break
		}
		for k := 1; k < MaxSondas; k++ {
			xs := make([]Valor, t.Largo)
			for i := range xs {
				xs[i] = Copiar(p[(k+i)%len(p)])
			}
			out = append(out, xs)
		}
	case CMapa:
		pk, pv := sondasTipo(elemDe(t.Clave), prof+1), sondasTipo(elemDe(t.Elem), prof+1)
		out = append(out, Mapa(nil), Mapa{})
		if len(pk) == 0 || len(pv) == 0 {
			break
		}
		for k := 2; k < MaxSondas; k++ {
			n := 1 + (k-2)%4
			if prof > 0 && n > 2 {
				n = 2
			}
			m := make(Mapa, 0, n)
			for j := 0; j < n; j++ {
				m = append(m, Par{K: Copiar(pk[(k+3*j)%len(pk)]), V: Copiar(pv[(7*k+j)%len(pv)])})
			}
			out = append(out, m.Ordenada())
		}
	case CStruct:
		if len(t.Campos) == 0 {
			return []Valor{Estructura{}}
		}
		ps := make([][]Valor, len(t.Campos))
		largo := 0
		for j, c := range t.Campos {
			ps[j] = sondasTipo(c.Tipo, prof+1)
			if len(ps[j]) == 0 {
				return nil
			}
			if len(ps[j]) > largo {
				largo = len(ps[j])
			}
		}
		for k := 0; k < largo && k < MaxSondas; k++ {
			s := make(Estructura, len(t.Campos))
			for j := range s {
				s[j] = Copiar(ps[j][(k+j)%len(ps[j])])
			}
			out = append(out, s)
		}
	case CPuntero:
		out = append(out, nil)
		for _, s := range sondasTipo(elemDe(t.Elem), prof+1) {
			out = append(out, s)
		}
	default:
		return nil
	}
	return sinRepetir(out)
}

// sinRepetir drops values whose Clave was already seen and keeps at most MaxSondas.
// nil and [] are kept apart (they are different probes even though Igual says they are equal).
func sinRepetir(xs []Valor) []Valor {
	visto := map[string]bool{}
	out := xs[:0]
	for _, x := range xs {
		k := Clave(x)
		if x != nil && esVacio(x) {
			k = "vacío"
		}
		if lst, ok := x.([]Valor); ok && lst == nil {
			k = "nil"
		}
		if m, ok := x.(Mapa); ok && m == nil {
			k = "nil"
		}
		if visto[k] {
			continue
		}
		visto[k] = true
		out = append(out, x)
		if len(out) == MaxSondas {
			break
		}
	}
	return out
}

// Sondas returns the fixed probe inputs for a signature: deterministic, ≤ 30 cases, the same in every
// version of the program (they define fingerprints). Ints: 0,1,-1,2,3,7,-5,10,100,…; []int: nil,[],[0],[5],[-3],
// [1,2,3,4],[4,3,2,1],[2,2,2],[-5,0,5],[1,3,5,7,9,2],…; strings: "", "a", "hola", "Hola Mundo", "ñandú ÁÉ",
// "  x  ", "a,b,,c", "12345", "anilina", "Go es genial",… (each list has up to 30 values).
//
// Inputs are combined by zipping (not a product), cycling the shorter lists: input j of case i takes value
// (i+j) mod len, so two inputs of the same type get different values. When the full product of the lists
// has at most 30 combinations (e.g. two bools), the product is used instead, in odometer order.
// A signature with an input that cannot be encoded (or an error input) has no probes (nil).
func Sondas(f Firma) []Caso {
	ent := f.Entradas()
	if len(ent) > 6 {
		return nil
	}
	listas := make([][]Valor, len(ent))
	producto := 1
	maximo := 1
	for j, t := range ent {
		if t.Clase == CError || !t.Codificable() {
			return nil
		}
		listas[j] = sondasTipo(t, 0)
		if len(listas[j]) == 0 {
			return nil
		}
		if producto <= MaxSondas {
			producto *= len(listas[j])
		}
		if len(listas[j]) > maximo {
			maximo = len(listas[j])
		}
	}
	var filas [][]Valor
	if producto <= MaxSondas {
		idx := make([]int, len(ent))
		for n := 0; n < producto; n++ {
			fila := make([]Valor, len(ent))
			for j := range ent {
				fila[j] = Copiar(listas[j][idx[j]])
			}
			filas = append(filas, fila)
			for j := len(ent) - 1; j >= 0; j-- {
				idx[j]++
				if idx[j] < len(listas[j]) {
					break
				}
				idx[j] = 0
			}
		}
	} else {
		n := maximo
		if n > MaxSondas {
			n = MaxSondas
		}
		for i := 0; i < n; i++ {
			fila := make([]Valor, len(ent))
			for j := range ent {
				fila[j] = Copiar(listas[j][(i+j)%len(listas[j])])
			}
			filas = append(filas, fila)
		}
	}
	out := make([]Caso, len(filas))
	for i, fila := range filas {
		out[i] = Caso{Entradas: fila, Origen: OrigenSonda, Nota: "sonda " + strconv.Itoa(i+1)}
	}
	return out
}

// Huella is the behavioural fingerprint: sha256 hex of Forma() plus the Clave of every output on Sondas.
// salidas has one row per probe case, in Sondas order; a nil row means the case gave no result
// (panic, crash, timeout, out of fuel) and is written as "⊥".
//
// Frozen format: Forma() + "\n", then per row the Claves joined by "\x1f" (or "⊥") + "\n".
func Huella(f Firma, salidas [][]Valor) string {
	h := sha256.New()
	var sb strings.Builder
	sb.WriteString(f.Forma())
	sb.WriteByte('\n')
	for _, fila := range salidas {
		if fila == nil {
			sb.WriteString("⊥")
		} else {
			for i, v := range fila {
				if i > 0 {
					sb.WriteByte(0x1f)
				}
				sb.WriteString(Clave(v))
			}
		}
		sb.WriteByte('\n')
	}
	h.Write([]byte(sb.String()))
	return hex.EncodeToString(h.Sum(nil))
}
