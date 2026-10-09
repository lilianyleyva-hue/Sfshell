// Package pruebas generates test inputs (boundary, random and scaled) for any codifiable nucleo.Tipo,
// builds concept properties, labels cases with an oracle, shrinks failing cases, compares program
// output, writes table-driven _test.go files and turns results into Comprobacion badges and tables.
//
// It imports only the standard library and internal/nucleo (§2, §4.3).
package pruebas

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Generador produces reproducible values from a seed.
type Generador struct {
	semilla   int64
	r         *rand.Rand
	rango     int    // |int| ≤ rango
	tamMax    int    // longest list/string for random values
	alfabeto  []rune // runes for random strings
	deEjemplo []rune // runes seen in the examples (preferred half of the time)
}

// alfabetoBase holds letters, a space and ñ (§4.3 "Random values").
var alfabetoBase = []rune("abcdefghijklmnopqrstuvwxyzABZ ñ")

// NuevoGenerador returns a generator seeded with semilla. Two generators with the same seed give the
// same values when they are asked the same things in the same order.
func NuevoGenerador(semilla int64) *Generador {
	return &Generador{
		semilla:  semilla,
		r:        rand.New(rand.NewSource(semilla)),
		rango:    1000,
		tamMax:   30,
		alfabeto: alfabetoBase,
	}
}

// Semilla returns the seed (shown in Comprobacion.Detalle).
func (g *Generador) Semilla() int64 { return g.semilla }

// ---- boundary values ----

var (
	bordesInt    = []int{0, 1, -1, 2, 7, -100, 1000}
	bordesFloat  = []float64{0, 1.5, -2.25, 1e-9, 1e6}
	bordesString = []string{"", "a", "hola", "Hola Mundo", "ñandú ÁÉ", "  x  ", "a,b,,c", "12345", "日本", "aaa"}
	bordesRune   = []rune{'a', 'Z', '0', ' ', 'ñ'}
	bordesByte   = []int{'a', 'Z', '0', ' ', 0, 255}
)

// maxBordesTipo caps the boundary values of one type (lists of lists, structs with many fields).
const maxBordesTipo = 16

// Bordes returns the boundary values of t (§4.3): ints 0, 1, -1, 2, 7, -100, 1000 (within the type's
// range); lists nil, {}, {b0}, {b1}, {b2,b2,b2}, ascending, descending and mixed signs; maps nil, {},
// one entry and three entries; structs with every field at its first boundary value and then each field
// varied in turn; pointers nil plus one struct. A type that cannot be encoded has no boundary values.
func (g *Generador) Bordes(t nucleo.Tipo) []nucleo.Valor {
	return bordes(t, 0)
}

func bordes(t nucleo.Tipo, prof int) []nucleo.Valor {
	if prof > 6 {
		return []nucleo.Valor{nucleo.ValorCero(t)}
	}
	var out []nucleo.Valor
	switch t.Clase {
	case nucleo.CInt:
		lo, hi := rangoEntero(t)
		for _, i := range bordesInt {
			if int64(i) >= lo && int64(i) <= hi {
				out = append(out, i)
			}
		}
	case nucleo.CFloat:
		for _, f := range bordesFloat {
			if esFloat32(t) {
				f = float64(float32(f))
			}
			out = append(out, f)
		}
	case nucleo.CBool:
		out = []nucleo.Valor{false, true}
	case nucleo.CString:
		for _, s := range bordesString {
			out = append(out, s)
		}
	case nucleo.CRune:
		for _, r := range bordesRune {
			out = append(out, int(r))
		}
	case nucleo.CByte:
		for _, b := range bordesByte {
			out = append(out, b)
		}
	case nucleo.CError:
		out = []nucleo.Valor{nil}
	case nucleo.CLista:
		out = bordesLista(t, prof)
	case nucleo.CArreglo:
		out = bordesArreglo(t, prof)
	case nucleo.CMapa:
		out = bordesMapa(t, prof)
	case nucleo.CStruct:
		out = bordesStruct(t, prof)
	case nucleo.CPuntero:
		out = []nucleo.Valor{nil}
		if t.Elem != nil {
			bs := bordes(*t.Elem, prof+1)
			switch {
			case len(bs) > 1:
				out = append(out, bs[1])
			case len(bs) == 1:
				out = append(out, bs[0])
			}
		}
	default:
		return nil
	}
	return sinRepetirValores(out)
}

// elementosBorde returns the boundary values of a container's element, without nil/empty duplicates.
func elementosBorde(e nucleo.Tipo, prof int) []nucleo.Valor {
	bs := bordes(e, prof+1)
	if len(bs) > 8 {
		bs = bs[:8]
	}
	return bs
}

func bordesLista(t nucleo.Tipo, prof int) []nucleo.Valor {
	out := []nucleo.Valor{[]nucleo.Valor(nil), []nucleo.Valor{}}
	if t.Elem == nil {
		return out
	}
	b := elementosBorde(*t.Elem, prof)
	if len(b) == 0 {
		return out
	}
	en := func(i int) nucleo.Valor { return nucleo.Copiar(b[i%len(b)]) }
	out = append(out, []nucleo.Valor{en(0)})
	if len(b) > 1 {
		out = append(out, []nucleo.Valor{en(1)})
	}
	k := 2
	if len(b) < 3 {
		k = len(b) - 1
	}
	out = append(out, []nucleo.Valor{en(k), en(k), en(k)})
	if len(b) > 1 {
		orden := ordenados(b, 6)
		asc := make([]nucleo.Valor, len(orden))
		desc := make([]nucleo.Valor, len(orden))
		for i, v := range orden {
			asc[i] = nucleo.Copiar(v)
			desc[len(orden)-1-i] = nucleo.Copiar(v)
		}
		out = append(out, asc, desc, mezclados(b, t.Elem.Clase))
	}
	return out
}

// ordenados returns up to n distinct values of b in ascending order.
func ordenados(b []nucleo.Valor, n int) []nucleo.Valor {
	c := append([]nucleo.Valor(nil), b...)
	sort.SliceStable(c, func(i, j int) bool { return nucleo.Comparar(c[i], c[j]) < 0 })
	var out []nucleo.Valor
	for _, v := range c {
		if len(out) > 0 && nucleo.Comparar(out[len(out)-1], v) == 0 {
			continue
		}
		out = append(out, v)
	}
	if len(out) > n {
		// keep both ends and the middle
		sel := make([]nucleo.Valor, 0, n)
		for i := 0; i < n; i++ {
			sel = append(sel, out[i*(len(out)-1)/(n-1)])
		}
		out = sel
	}
	return out
}

// mezclados interleaves positive and negative numbers ("mixed signs"); for other classes it returns an
// unsorted mix of the boundary values.
func mezclados(b []nucleo.Valor, c nucleo.Clase) []nucleo.Valor {
	if c == nucleo.CInt || c == nucleo.CFloat {
		var pos, neg []nucleo.Valor
		for _, v := range b {
			f := comoFloat(v)
			switch {
			case f > 0:
				pos = append(pos, v)
			case f < 0:
				neg = append(neg, v)
			}
		}
		if len(neg) > 0 && len(pos) > 0 {
			var out []nucleo.Valor
			for i := 0; i < len(pos) || i < len(neg); i++ {
				if i < len(pos) {
					out = append(out, nucleo.Copiar(pos[i]))
				}
				if i < len(neg) {
					out = append(out, nucleo.Copiar(neg[i]))
				}
				if len(out) >= 6 {
					break
				}
			}
			return out
		}
	}
	idx := []int{1, 0, 3, 2, 5, 4}
	var out []nucleo.Valor
	for _, i := range idx {
		if i < len(b) {
			out = append(out, nucleo.Copiar(b[i]))
		}
	}
	return out
}

func bordesArreglo(t nucleo.Tipo, prof int) []nucleo.Valor {
	if t.Largo <= 0 || t.Elem == nil {
		return []nucleo.Valor{[]nucleo.Valor{}}
	}
	out := []nucleo.Valor{nucleo.ValorCero(t)}
	b := elementosBorde(*t.Elem, prof)
	if len(b) == 0 {
		return out
	}
	lleno := func(f func(i int) nucleo.Valor) []nucleo.Valor {
		xs := make([]nucleo.Valor, t.Largo)
		for i := range xs {
			xs[i] = nucleo.Copiar(f(i))
		}
		return xs
	}
	k := 1
	if len(b) < 2 {
		k = 0
	}
	out = append(out, lleno(func(int) nucleo.Valor { return b[k] }))
	orden := ordenados(b, len(b))
	out = append(out,
		lleno(func(i int) nucleo.Valor { return orden[i%len(orden)] }),
		lleno(func(i int) nucleo.Valor { return orden[len(orden)-1-i%len(orden)] }))
	m := mezclados(b, t.Elem.Clase)
	if len(m) > 0 {
		out = append(out, lleno(func(i int) nucleo.Valor { return m[i%len(m)] }))
	}
	return out
}

func bordesMapa(t nucleo.Tipo, prof int) []nucleo.Valor {
	out := []nucleo.Valor{nucleo.Mapa(nil), nucleo.Mapa{}}
	if t.Clave == nil || t.Elem == nil {
		return out
	}
	ks := elementosBorde(*t.Clave, prof)
	vs := elementosBorde(*t.Elem, prof)
	if len(ks) == 0 || len(vs) == 0 {
		return out
	}
	uno := nucleo.Mapa{{K: nucleo.Copiar(ks[min(1, len(ks)-1)]), V: nucleo.Copiar(vs[min(1, len(vs)-1)])}}
	out = append(out, uno)
	tres := nucleo.Mapa{}
	for i := 0; i < 3 && i < len(ks); i++ {
		tres = append(tres, nucleo.Par{K: nucleo.Copiar(ks[i]), V: nucleo.Copiar(vs[(i+2)%len(vs)])})
	}
	out = append(out, tres.Ordenada())
	return out
}

func bordesStruct(t nucleo.Tipo, prof int) []nucleo.Valor {
	if len(t.Campos) == 0 {
		return []nucleo.Valor{nucleo.Estructura{}}
	}
	campos := make([][]nucleo.Valor, len(t.Campos))
	base := make(nucleo.Estructura, len(t.Campos))
	for i, c := range t.Campos {
		campos[i] = bordes(c.Tipo, prof+1)
		if len(campos[i]) == 0 {
			return nil
		}
		base[i] = campos[i][0]
	}
	out := []nucleo.Valor{nucleo.Copiar(base)}
	// each field varied in turn, round-robin so every field gets variations before the cap
	for j := 1; len(out) < maxBordesTipo; j++ {
		algo := false
		for i := range t.Campos {
			if j >= len(campos[i]) {
				continue
			}
			algo = true
			s := nucleo.Copiar(base).(nucleo.Estructura)
			s[i] = nucleo.Copiar(campos[i][j])
			out = append(out, s)
			if len(out) >= maxBordesTipo {
				break
			}
		}
		if !algo {
			break
		}
	}
	return out
}

// ---- random values ----

// Azar returns a random value of type t whose size grows with tam (list and string lengths up to tam).
func (g *Generador) Azar(t nucleo.Tipo, tam int) nucleo.Valor {
	if tam < 0 {
		tam = 0
	}
	return g.azar(t, tam, 0)
}

func (g *Generador) azar(t nucleo.Tipo, tam, prof int) nucleo.Valor {
	if prof > 6 {
		return nucleo.ValorCero(t)
	}
	switch t.Clase {
	case nucleo.CInt:
		return g.entero(t, tam)
	case nucleo.CFloat:
		lim := float64(g.limite(tam))
		f := (g.r.Float64()*2 - 1) * lim
		f = math.Round(f*100) / 100
		if esFloat32(t) {
			f = float64(float32(f))
		}
		return f
	case nucleo.CBool:
		return g.r.Intn(2) == 1
	case nucleo.CString:
		n := g.r.Intn(tam + 1)
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteRune(g.runa())
		}
		return sb.String()
	case nucleo.CRune:
		return int(g.runa())
	case nucleo.CByte:
		if g.r.Intn(10) == 0 {
			return g.r.Intn(256)
		}
		return 32 + g.r.Intn(95)
	case nucleo.CError:
		return nil
	case nucleo.CLista:
		if t.Elem == nil {
			return []nucleo.Valor{}
		}
		n := g.r.Intn(tam + 1)
		xs := make([]nucleo.Valor, n)
		sub := tamInterior(*t.Elem, tam)
		for i := range xs {
			xs[i] = g.azar(*t.Elem, sub, prof+1)
		}
		return xs
	case nucleo.CArreglo:
		if t.Elem == nil || t.Largo <= 0 {
			return []nucleo.Valor{}
		}
		xs := make([]nucleo.Valor, t.Largo)
		sub := tamInterior(*t.Elem, tam)
		for i := range xs {
			xs[i] = g.azar(*t.Elem, sub, prof+1)
		}
		return xs
	case nucleo.CMapa:
		if t.Clave == nil || t.Elem == nil {
			return nucleo.Mapa{}
		}
		n := g.r.Intn(tam/2 + 2)
		m := make(nucleo.Mapa, 0, n)
		sub := tamInterior(*t.Elem, tam)
		for i := 0; i < n; i++ {
			m = append(m, nucleo.Par{K: g.azar(*t.Clave, tam, prof+1), V: g.azar(*t.Elem, sub, prof+1)})
		}
		return m.Ordenada()
	case nucleo.CStruct:
		s := make(nucleo.Estructura, len(t.Campos))
		for i, c := range t.Campos {
			s[i] = g.azar(c.Tipo, tamInterior(c.Tipo, tam), prof+1)
		}
		return s
	case nucleo.CPuntero:
		if t.Elem == nil || g.r.Intn(5) == 0 {
			return nil
		}
		return g.azar(*t.Elem, tam, prof+1)
	}
	return nil
}

// tamInterior keeps nested containers small (a list of lists of 30×30 is too big to read).
func tamInterior(e nucleo.Tipo, tam int) int {
	switch e.Clase {
	case nucleo.CLista, nucleo.CMapa, nucleo.CArreglo, nucleo.CStruct, nucleo.CPuntero:
		return min(tam, 5)
	case nucleo.CString:
		return min(tam, 12)
	}
	return tam
}

// limite is the magnitude bound for numbers: half of the values are small (|v| ≤ tam+3) so that
// duplicates and sign changes are common, the other half use the whole range.
func (g *Generador) limite(tam int) int {
	lim := g.rango
	if lim <= 0 {
		lim = 1000
	}
	if g.r.Intn(2) == 0 && tam+3 < lim {
		lim = tam + 3
	}
	return lim
}

func (g *Generador) entero(t nucleo.Tipo, tam int) int {
	lim := int64(g.limite(tam))
	v := g.r.Int63n(2*lim+1) - lim
	lo, hi := rangoEntero(t)
	if lo == 0 && v < 0 {
		v = -v
	}
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return int(v)
}

func (g *Generador) runa() rune {
	if len(g.deEjemplo) > 0 && g.r.Intn(2) == 0 {
		return g.deEjemplo[g.r.Intn(len(g.deEjemplo))]
	}
	a := g.alfabeto
	if len(a) == 0 {
		a = alfabetoBase
	}
	return a[g.r.Intn(len(a))]
}

// ---- cases ----

// OpcionesCasos configures Casos. Zero values take the defaults below.
type OpcionesCasos struct {
	Bordes   int           // max boundary combinations, default 64
	Azar     int           // random cases, default 200
	TamMax   int           // max list/string length, default 30
	Rango    int           // |int| ≤ Rango, default 1000 (adapted: 10× the max |value| in Ejemplos, min 10)
	Grande   bool          // add one 10 000-element case for speed
	Ejemplos []nucleo.Caso // used to adapt magnitudes, string alphabets and list lengths
}

// Casos returns boundary cases (pairwise combinations, Origen borde) followed by random cases
// (Origen azar) and, with Grande, one 10 000-element case. No case carries an expected value
// (Expectativa EspNinguna). The result depends only on the seed, f and op: calling it twice gives the
// same cases. A signature with an input that cannot be encoded gives nil.
func (g *Generador) Casos(f nucleo.Firma, op OpcionesCasos) []nucleo.Caso {
	ent := f.Entradas()
	for _, t := range ent {
		if t.Clase == nucleo.CError || !t.Codificable() {
			return nil
		}
	}
	if op.Bordes == 0 {
		op.Bordes = 64
	}
	if op.Azar == 0 {
		op.Azar = 200
	}
	sub := NuevoGenerador(g.semilla)
	sub.adaptar(op)

	visto := map[string]bool{}
	var out []nucleo.Caso
	agregar := func(c nucleo.Caso) {
		k := claveEntradas(c.Entradas)
		if visto[k] {
			return
		}
		visto[k] = true
		out = append(out, c)
	}

	// boundary combinations
	if op.Bordes > 0 {
		listas := make([][]nucleo.Valor, len(ent))
		tams := make([]int, len(ent))
		for i, t := range ent {
			listas[i] = sub.Bordes(t)
			if len(listas[i]) == 0 {
				return nil
			}
			tams[i] = len(listas[i])
		}
		n := 0
		for _, fila := range combinar(tams, op.Bordes) {
			e := make([]nucleo.Valor, len(fila))
			for i, k := range fila {
				e[i] = nucleo.Copiar(listas[i][k])
			}
			n++
			agregar(nucleo.Caso{Entradas: e, Origen: nucleo.OrigenBorde, Nota: "borde " + strconv.Itoa(n)})
		}
	}

	// random cases, size growing linearly from 0 to TamMax
	if op.Azar > 0 && len(ent) > 0 {
		for k := 0; k < op.Azar; k++ {
			tam := sub.tamMax
			if op.Azar > 1 {
				tam = k * sub.tamMax / (op.Azar - 1)
			}
			e := make([]nucleo.Valor, len(ent))
			for i, t := range ent {
				e[i] = sub.Azar(t, tam)
			}
			agregar(nucleo.Caso{Entradas: e, Origen: nucleo.OrigenAzar, Nota: "al azar " + strconv.Itoa(k+1)})
		}
	}

	if op.Grande {
		if c, ok := sub.Escalar(f, 10000); ok {
			c.Nota = "grande (10 000 elementos)"
			agregar(c)
		}
	}
	return out
}

// adaptar sets range, maximum size and alphabet from the options and the user's examples.
func (g *Generador) adaptar(op OpcionesCasos) {
	maxAbs, maxLargo := -1.0, 0
	var runas []rune
	vistas := map[rune]bool{}
	var mirar func(v nucleo.Valor)
	mirar = func(v nucleo.Valor) {
		switch x := v.(type) {
		case string:
			if utf8.RuneCountInString(x) > maxLargo {
				maxLargo = utf8.RuneCountInString(x)
			}
			for _, r := range x {
				if !vistas[r] && len(runas) < 64 {
					vistas[r] = true
					runas = append(runas, r)
				}
			}
		case []nucleo.Valor:
			if len(x) > maxLargo {
				maxLargo = len(x)
			}
			for _, e := range x {
				mirar(e)
			}
		case nucleo.Estructura:
			for _, e := range x {
				mirar(e)
			}
		case nucleo.Mapa:
			if len(x) > maxLargo {
				maxLargo = len(x)
			}
			for _, p := range x {
				mirar(p.K)
				mirar(p.V)
			}
		case bool, nil, nucleo.ErrorV:
		default:
			if a := math.Abs(comoFloat(v)); a > maxAbs && !math.IsInf(a, 0) && !math.IsNaN(a) {
				maxAbs = a
			}
		}
	}
	for _, c := range op.Ejemplos {
		for _, v := range c.Entradas {
			mirar(v)
		}
		for _, v := range c.Esperado {
			mirar(v)
		}
	}
	switch {
	case op.Rango > 0:
		g.rango = op.Rango
	case maxAbs >= 0:
		r := 10 * maxAbs
		if r < 10 {
			r = 10
		}
		if r > 1e9 {
			r = 1e9
		}
		g.rango = int(r)
	default:
		g.rango = 1000
	}
	switch {
	case op.TamMax > 0:
		g.tamMax = op.TamMax
	case 2*maxLargo > 30:
		g.tamMax = min(2*maxLargo, 100)
	default:
		g.tamMax = 30
	}
	g.deEjemplo = runas
}

// Escalar returns one input of "size" n: every list, string and map input gets n elements; arrays keep
// their length and other inputs get a small fixed value. It returns false when no input bears a size.
func (g *Generador) Escalar(f nucleo.Firma, n int) (nucleo.Caso, bool) {
	ent := f.Entradas()
	if n < 0 {
		n = 0
	}
	hay := false
	for _, t := range ent {
		switch t.Clase {
		case nucleo.CLista, nucleo.CString, nucleo.CMapa:
			hay = true
		case nucleo.CError:
			return nucleo.Caso{}, false
		}
		if !t.Codificable() {
			return nucleo.Caso{}, false
		}
	}
	if !hay {
		return nucleo.Caso{}, false
	}
	sub := NuevoGenerador(g.semilla + int64(n))
	sub.rango, sub.tamMax, sub.alfabeto, sub.deEjemplo = g.rango, g.tamMax, g.alfabeto, g.deEjemplo
	e := make([]nucleo.Valor, len(ent))
	for i, t := range ent {
		e[i] = sub.deTamano(t, n)
	}
	return nucleo.Caso{Entradas: e, Origen: nucleo.OrigenAzar, Nota: "tamaño " + strconv.Itoa(n)}, true
}

func (g *Generador) deTamano(t nucleo.Tipo, n int) nucleo.Valor {
	switch t.Clase {
	case nucleo.CLista:
		xs := make([]nucleo.Valor, n)
		for i := range xs {
			xs[i] = g.azar(*t.Elem, tamInterior(*t.Elem, 10), 1)
		}
		return xs
	case nucleo.CString:
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteRune(alfabetoBase[g.r.Intn(len(alfabetoBase))])
		}
		return sb.String()
	case nucleo.CMapa:
		m := make(nucleo.Mapa, 0, n)
		usadas := map[string]bool{}
		for i := 0; len(m) < n && i < 4*n+10; i++ {
			var k nucleo.Valor
			switch t.Clave.Clase {
			case nucleo.CInt:
				k = i
			case nucleo.CString:
				k = "k" + strconv.Itoa(i)
			default:
				k = g.azar(*t.Clave, 10, 1)
			}
			if c := nucleo.Clave(k); !usadas[c] {
				usadas[c] = true
				m = append(m, nucleo.Par{K: k, V: g.azar(*t.Elem, 5, 1)})
			}
		}
		return m.Ordenada()
	}
	bs := g.Bordes(t)
	if len(bs) == 0 {
		return nucleo.ValorCero(t)
	}
	return nucleo.Copiar(bs[min(3, len(bs)-1)])
}

// ---- pairwise covering ----

// combinar returns index rows over parameters with tams[i] values each: the full product when it has at
// most max rows, otherwise a greedy pairwise covering design (every pair of values of every two
// parameters appears in some row) capped at max rows. The first row is all zeros.
func combinar(tams []int, max int) [][]int {
	k := len(tams)
	if k == 0 {
		return [][]int{{}}
	}
	if max <= 0 {
		return nil
	}
	producto := 1
	for _, n := range tams {
		if n <= 0 {
			return nil
		}
		if producto <= max {
			producto *= n
		}
	}
	if producto <= max {
		var out [][]int
		idx := make([]int, k)
		for n := 0; n < producto; n++ {
			out = append(out, append([]int(nil), idx...))
			for j := k - 1; j >= 0; j-- {
				idx[j]++
				if idx[j] < tams[j] {
					break
				}
				idx[j] = 0
			}
		}
		return out
	}
	if k == 1 {
		out := make([][]int, 0, max)
		for i := 0; i < tams[0] && i < max; i++ {
			out = append(out, []int{i})
		}
		return out
	}
	// cubierto[i][j][a*tams[j]+b] for i < j
	cubierto := make([][][]bool, k)
	pendientes := 0
	for i := 0; i < k; i++ {
		cubierto[i] = make([][]bool, k)
		for j := i + 1; j < k; j++ {
			cubierto[i][j] = make([]bool, tams[i]*tams[j])
			pendientes += tams[i] * tams[j]
		}
	}
	marcar := func(fila []int) {
		for i := 0; i < k; i++ {
			for j := i + 1; j < k; j++ {
				c := &cubierto[i][j][fila[i]*tams[j]+fila[j]]
				if !*c {
					*c = true
					pendientes--
				}
			}
		}
	}
	var out [][]int
	primera := make([]int, k)
	out = append(out, primera)
	marcar(primera)
	for pendientes > 0 && len(out) < max {
		fila := make([]int, k)
		for i := range fila {
			fila[i] = -1
		}
		// first uncovered pair
	buscar:
		for i := 0; i < k; i++ {
			for j := i + 1; j < k; j++ {
				for p, c := range cubierto[i][j] {
					if !c {
						fila[i], fila[j] = p/tams[j], p%tams[j]
						break buscar
					}
				}
			}
		}
		for p := 0; p < k; p++ {
			if fila[p] >= 0 {
				continue
			}
			mejor, mejorN := 0, -1
			for v := 0; v < tams[p]; v++ {
				n := 0
				for q := 0; q < k; q++ {
					if q == p || fila[q] < 0 {
						continue
					}
					i, j, a, b := q, p, fila[q], v
					if p < q {
						i, j, a, b = p, q, v, fila[q]
					}
					if !cubierto[i][j][a*tams[j]+b] {
						n++
					}
				}
				if n > mejorN {
					mejor, mejorN = v, n
				}
			}
			fila[p] = mejor
		}
		out = append(out, fila)
		marcar(fila)
	}
	return out
}

// ---- small helpers ----

// rangoEntero returns the inclusive range of an integer type (by its spelling).
func rangoEntero(t nucleo.Tipo) (lo, hi int64) {
	switch t.Clase {
	case nucleo.CRune:
		return -1 << 31, 1<<31 - 1
	case nucleo.CByte:
		return 0, 255
	}
	switch t.Nombre {
	case "int8":
		return -128, 127
	case "int16":
		return -1 << 15, 1<<15 - 1
	case "int32", "rune":
		return -1 << 31, 1<<31 - 1
	case "uint8", "byte":
		return 0, 255
	case "uint16":
		return 0, 1<<16 - 1
	case "uint32":
		return 0, 1<<32 - 1
	case "uint", "uint64", "uintptr":
		return 0, 1<<63 - 1
	}
	return -1 << 63, 1<<63 - 1
}

func esFloat32(t nucleo.Tipo) bool { return t.Nombre == "float32" }

// comoFloat converts a numeric Valor to float64 (0 for anything else).
func comoFloat(v nucleo.Valor) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	case float64:
		return x
	case float32:
		return float64(x)
	case uint8:
		return float64(x)
	}
	return 0
}

// claveValor is nucleo.Clave, except that a nil list or map and an empty one are told apart
// (they are different inputs for Go code even though nucleo.Igual treats them as equal).
func claveValor(v nucleo.Valor) string {
	switch x := v.(type) {
	case []nucleo.Valor:
		if x == nil {
			return "nil"
		}
		if len(x) == 0 {
			return "[]"
		}
	case nucleo.Mapa:
		if x == nil {
			return "nil"
		}
		if len(x) == 0 {
			return "{}"
		}
	}
	return nucleo.Clave(v)
}

func claveEntradas(e []nucleo.Valor) string {
	partes := make([]string, len(e))
	for i, v := range e {
		partes[i] = claveValor(v)
	}
	return strings.Join(partes, "\x1f")
}

func sinRepetirValores(xs []nucleo.Valor) []nucleo.Valor {
	visto := map[string]bool{}
	out := xs[:0]
	for _, x := range xs {
		k := claveValor(x)
		if visto[k] {
			continue
		}
		visto[k] = true
		out = append(out, x)
	}
	return out
}
