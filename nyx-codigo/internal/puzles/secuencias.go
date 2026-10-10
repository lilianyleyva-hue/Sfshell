package puzles

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Regla is a rule that explains a number sequence.
type Regla struct {
	Descripcion string // "las diferencias crecen de 2 en 2"
	Siguientes  []*big.Rat
	Params      int
	Confianza   string // "alta" | "media" | "pocos datos"
}

// candidata generates term i from the previous terms (len(prev) == i); the first inicial terms are
// taken as given.
type candidata struct {
	desc    string
	params  int
	inicial int
	termino func(prev []*big.Rat, i int) *big.Rat
}

func rat(x int64) *big.Rat { return new(big.Rat).SetInt64(x) }

// encaja re-checks the candidate on every term.
func (c candidata) encaja(seq []*big.Rat) bool {
	if c.inicial > len(seq) {
		return false
	}
	for i := c.inicial; i < len(seq); i++ {
		v := c.termino(seq[:i], i)
		if v == nil || v.Cmp(seq[i]) != 0 {
			return false
		}
	}
	return true
}

func (c candidata) extender(seq []*big.Rat, cuantos int) []*big.Rat {
	todo := append([]*big.Rat(nil), seq...)
	var out []*big.Rat
	for k := 0; k < cuantos; k++ {
		v := c.termino(todo, len(todo))
		if v == nil {
			break
		}
		todo = append(todo, v)
		out = append(out, v)
	}
	return out
}

// --- 1. known tables

type tabla struct {
	nombre string
	gen    func(n int) []*big.Int // first n terms
}

func tablasConocidas() []tabla {
	primos := func(n int) []*big.Int {
		var out []*big.Int
		for x := int64(2); len(out) < n; x++ {
			if esPrimo(x) {
				out = append(out, big.NewInt(x))
			}
		}
		return out
	}
	porFormula := func(f func(i int64) *big.Int) func(n int) []*big.Int {
		return func(n int) []*big.Int {
			var out []*big.Int
			for i := 0; i < n; i++ {
				out = append(out, f(int64(i)))
			}
			return out
		}
	}
	ts := []tabla{
		{"son los números primos", primos},
		{"son los cuadrados: n²", porFormula(func(i int64) *big.Int { return big.NewInt(i * i) })},
		{"son los cubos: n³", porFormula(func(i int64) *big.Int { return big.NewInt(i * i * i) })},
		{"son los números triangulares: n·(n+1)/2", porFormula(func(i int64) *big.Int { return big.NewInt(i * (i + 1) / 2) })},
		{"son los factoriales: n!", porFormula(func(i int64) *big.Int { return factorial(int(i)) })},
		{"cada término es la suma de los dos anteriores (Fibonacci)", func(n int) []*big.Int {
			out := []*big.Int{big.NewInt(0), big.NewInt(1)}
			for len(out) < n {
				out = append(out, new(big.Int).Add(out[len(out)-1], out[len(out)-2]))
			}
			return out[:n]
		}},
	}
	for k := int64(2); k <= 10; k++ {
		k := k
		ts = append(ts, tabla{fmt.Sprintf("son las potencias de %d", k), porFormula(func(i int64) *big.Int {
			return new(big.Int).Exp(big.NewInt(k), big.NewInt(i), nil)
		})})
	}
	return ts
}

// candidatasTabla looks the sequence up in the known tables. Short runs match too many tables
// ("1, 2, 3" is also a piece of Fibonacci), so at least 4 terms are required.
func candidatasTabla(seq []*big.Rat) []candidata {
	if len(seq) < 4 {
		return nil
	}
	var ints []*big.Int
	for _, x := range seq {
		if !x.IsInt() {
			return nil
		}
		ints = append(ints, new(big.Int).Set(x.Num()))
	}
	var out []candidata
	for _, t := range tablasConocidas() {
		const largo = 120
		valores := t.gen(largo)
		for o := 0; o+len(ints) <= len(valores); o++ {
			ok := true
			for i, x := range ints {
				if valores[o+i].Cmp(x) != 0 {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			o, gen := o, t.gen
			out = append(out, candidata{desc: t.nombre, params: 2, inicial: 0, termino: func(_ []*big.Rat, i int) *big.Rat {
				vs := gen(o + i + 1)
				if len(vs) <= o+i {
					return nil
				}
				return new(big.Rat).SetInt(vs[o+i])
			}})
			break
		}
	}
	return out
}

// --- 2. difference table

func diferencias(seq []*big.Rat) []*big.Rat {
	var out []*big.Rat
	for i := 1; i < len(seq); i++ {
		out = append(out, new(big.Rat).Sub(seq[i], seq[i-1]))
	}
	return out
}

func listaRat(xs []*big.Rat) string {
	var partes []string
	for _, x := range xs {
		partes = append(partes, TextoRat(x))
	}
	return strings.Join(partes, ", ")
}

// TextoRat writes an integer as "5" and a fraction as "3/4".
func TextoRat(x *big.Rat) string {
	if x.IsInt() {
		return x.Num().String()
	}
	return x.RatString()
}

func candidataPolinomio(seq []*big.Rat, grado int) candidata {
	if len(seq) < grado+1 {
		return candidata{}
	}
	// Newton forward differences at 0
	base := make([]*big.Rat, grado+1)
	nivel := seq[:grado+1]
	for j := 0; j <= grado; j++ {
		base[j] = new(big.Rat).Set(nivel[0])
		nivel = diferencias(nivel)
	}
	var desc string
	switch grado {
	case 0:
		desc = "siempre es " + TextoRat(seq[0])
	case 1:
		paso := base[1]
		switch paso.Sign() {
		case 1:
			desc = "suma " + TextoRat(paso) + " cada vez"
		default:
			desc = "resta " + TextoRat(new(big.Rat).Neg(paso)) + " cada vez"
		}
	case 2:
		d := diferencias(seq)
		desc = fmt.Sprintf("las diferencias (%s…) crecen de %s en %s", listaRat(d[:min(len(d), 4)]), TextoRat(base[2]), TextoRat(base[2]))
	default:
		desc = fmt.Sprintf("la tabla de diferencias se vuelve constante en el nivel %d (un polinomio de grado %d)", grado, grado)
	}
	return candidata{desc: desc, params: grado + 1, inicial: 0, termino: func(_ []*big.Rat, i int) *big.Rat {
		v := new(big.Rat)
		for j := 0; j <= grado; j++ {
			c := new(big.Rat).SetInt(combinaciones(i, j))
			v.Add(v, c.Mul(c, base[j]))
		}
		return v
	}}
}

// --- 3. constant ratio

func candidataRazon(seq []*big.Rat) (candidata, bool) {
	if len(seq) < 2 || seq[0].Sign() == 0 {
		return candidata{}, false
	}
	r := new(big.Rat).Quo(seq[1], seq[0])
	desc := "multiplica por " + TextoRat(r) + " cada vez"
	if r.Sign() > 0 && !r.IsInt() && new(big.Rat).Inv(r).IsInt() {
		desc = "divide entre " + TextoRat(new(big.Rat).Inv(r)) + " cada vez"
	}
	return candidata{desc: desc, params: 2, inicial: 1, termino: func(prev []*big.Rat, i int) *big.Rat {
		return new(big.Rat).Mul(prev[i-1], r)
	}}, true
}

// --- 4. linear recurrences of order ≤ 3

// resolverLineal solves A·x = b over the rationals (nil when singular).
func resolverLineal(a [][]*big.Rat, b []*big.Rat) []*big.Rat {
	n := len(b)
	m := make([][]*big.Rat, n)
	for i := range a {
		m[i] = make([]*big.Rat, n+1)
		for j := 0; j < n; j++ {
			m[i][j] = new(big.Rat).Set(a[i][j])
		}
		m[i][n] = new(big.Rat).Set(b[i])
	}
	for col := 0; col < n; col++ {
		piv := -1
		for f := col; f < n; f++ {
			if m[f][col].Sign() != 0 {
				piv = f
				break
			}
		}
		if piv < 0 {
			return nil
		}
		m[col], m[piv] = m[piv], m[col]
		for f := 0; f < n; f++ {
			if f == col || m[f][col].Sign() == 0 {
				continue
			}
			factor := new(big.Rat).Quo(m[f][col], m[col][col])
			for j := col; j <= n; j++ {
				m[f][j].Sub(m[f][j], new(big.Rat).Mul(factor, m[col][j]))
			}
		}
	}
	x := make([]*big.Rat, n)
	for i := 0; i < n; i++ {
		x[i] = new(big.Rat).Quo(m[i][n], m[i][i])
	}
	return x
}

func candidataRecurrencia(seq []*big.Rat, orden int, afin bool) (candidata, bool) {
	u := orden
	if afin {
		u++
	}
	if len(seq) < orden+u {
		return candidata{}, false
	}
	var a [][]*big.Rat
	var b []*big.Rat
	for i := orden; i < orden+u; i++ {
		var fila []*big.Rat
		for j := 1; j <= orden; j++ {
			fila = append(fila, seq[i-j])
		}
		if afin {
			fila = append(fila, rat(1))
		}
		a = append(a, fila)
		b = append(b, seq[i])
	}
	x := resolverLineal(a, b)
	if x == nil {
		return candidata{}, false
	}
	coef := x[:orden]
	cte := rat(0)
	if afin {
		cte = x[orden]
	}
	return candidata{desc: describirRecurrencia(coef, cte), params: orden + u, inicial: orden, termino: func(prev []*big.Rat, i int) *big.Rat {
		v := new(big.Rat).Set(cte)
		for j := 1; j <= orden; j++ {
			v.Add(v, new(big.Rat).Mul(coef[j-1], prev[i-j]))
		}
		return v
	}}, true
}

func describirRecurrencia(coef []*big.Rat, cte *big.Rat) string {
	uno := rat(1)
	if len(coef) == 2 && coef[0].Cmp(uno) == 0 && coef[1].Cmp(uno) == 0 && cte.Sign() == 0 {
		return "cada término es la suma de los dos anteriores"
	}
	if len(coef) == 3 && coef[0].Cmp(uno) == 0 && coef[1].Cmp(uno) == 0 && coef[2].Cmp(uno) == 0 && cte.Sign() == 0 {
		return "cada término es la suma de los tres anteriores"
	}
	nombres := []string{"el anterior", "el de dos antes", "el de tres antes"}
	var partes []string
	for j, c := range coef {
		if c.Sign() == 0 {
			continue
		}
		switch {
		case c.Cmp(uno) == 0:
			partes = append(partes, nombres[j])
		case c.Cmp(rat(2)) == 0:
			partes = append(partes, "el doble de "+nombres[j])
		case c.Cmp(rat(3)) == 0:
			partes = append(partes, "el triple de "+nombres[j])
		default:
			partes = append(partes, TextoRat(c)+" veces "+nombres[j])
		}
	}
	texto := "cada término es " + strings.Join(partes, " más ")
	if len(partes) == 0 {
		texto = "cada término es"
	}
	switch cte.Sign() {
	case 1:
		texto += " más " + TextoRat(cte)
	case -1:
		texto += " menos " + TextoRat(new(big.Rat).Neg(cte))
	}
	return strings.ReplaceAll(texto, " de el ", " del ")
}

// reglasBasicas are the steps 1–4 in MDL order.
func reglasBasicas(seq []*big.Rat) []candidata {
	var out []candidata
	out = append(out, candidatasTabla(seq)...)
	for g := 0; g <= 4; g++ {
		if c := candidataPolinomio(seq, g); c.termino != nil {
			out = append(out, c)
		}
	}
	if c, ok := candidataRazon(seq); ok {
		out = append(out, c)
	}
	for _, r := range []struct {
		orden int
		afin  bool
	}{{1, true}, {2, false}, {2, true}, {3, false}, {3, true}} {
		if c, ok := candidataRecurrencia(seq, r.orden, r.afin); ok {
			out = append(out, c)
		}
	}
	return out
}

// primeraAceptada returns the first candidate (MDL order) that fits every term with at least one
// term to spare; pocas reports whether some simple rule fits but lacks data.
func primeraAceptada(seq []*big.Rat) (candidata, bool, bool) {
	pocas := false
	for _, c := range reglasBasicas(seq) {
		if !c.encaja(seq) {
			continue
		}
		if len(seq) >= c.params+1 {
			return c, true, pocas
		}
		if c.params <= 4 {
			pocas = true
		}
	}
	return candidata{}, false, pocas
}

// candidataIntercalada: the even and odd positions follow two separate rules.
func candidataIntercalada(seq []*big.Rat) (candidata, bool) {
	if len(seq) < 5 {
		return candidata{}, false
	}
	var pares, impares []*big.Rat
	for i, x := range seq {
		if i%2 == 0 {
			pares = append(pares, x)
		} else {
			impares = append(impares, x)
		}
	}
	cp, okp, _ := primeraAceptadaRelajada(pares)
	ci, oki, _ := primeraAceptadaRelajada(impares)
	if !okp || !oki {
		return candidata{}, false
	}
	desc := fmt.Sprintf("son dos secuencias intercaladas: en los lugares 1, 3, 5… %s; en los lugares 2, 4, 6… %s", cp.desc, ci.desc)
	return candidata{desc: desc, params: cp.params + ci.params, inicial: 2 * max(cp.inicial, ci.inicial), termino: func(prev []*big.Rat, i int) *big.Rat {
		var sub []*big.Rat
		for k := i % 2; k < len(prev); k += 2 {
			sub = append(sub, prev[k])
		}
		c := cp
		if i%2 == 1 {
			c = ci
		}
		return c.termino(sub, len(sub))
	}}, true
}

// primeraAceptadaRelajada accepts a subsequence rule that fits all its terms (the whole interleaved
// rule is then held to the params + 1 requirement).
func primeraAceptadaRelajada(seq []*big.Rat) (candidata, bool, bool) {
	for _, c := range reglasBasicas(seq) {
		if c.encaja(seq) && len(seq) >= c.params {
			return c, true, false
		}
	}
	return candidata{}, false, false
}

// ErrPocosDatos marks a sequence that some simple rule fits, but with no term left to check it.
var ErrPocosDatos = errors.New("puzles: posible, pero con pocos datos")

// Siguiente finds the rule of a sequence in MDL order (known tables, difference table up to degree 4,
// constant ratio, linear recurrences of order ≤ 3, two interleaved sequences) and gives the next
// cuantos terms. A rule is accepted only if it fits every term and there are at least params + 1
// terms; Confianza is "alta" with params + 2 terms or more and "media" otherwise. When no rule
// reaches params + 1 the result has Confianza "pocos datos", no terms, and the error ErrPocosDatos.
func Siguiente(seq []*big.Rat, cuantos int) (Regla, error) {
	if cuantos <= 0 {
		cuantos = 1
	}
	if len(seq) == 0 {
		return Regla{}, fmt.Errorf("puzles: no hay números: %w", nucleo.ErrNoEntiendo)
	}
	c, ok, pocas := primeraAceptada(seq)
	if !ok {
		if ci, oki := candidataIntercalada(seq); oki && ci.encaja(seq) && len(seq) >= ci.params+1 {
			c, ok = ci, true
		}
	}
	if !ok {
		if pocas || len(seq) < 5 {
			return Regla{Descripcion: "posible, pero con pocos datos", Confianza: "pocos datos"}, ErrPocosDatos
		}
		return Regla{}, fmt.Errorf("puzles: no encuentro ninguna regla sencilla que cumplan todos los términos: %w", nucleo.ErrNoEntiendo)
	}
	if !c.encaja(seq) {
		return Regla{}, errors.New("puzles: la regla no encaja al comprobarla (error interno)")
	}
	r := Regla{Descripcion: c.desc, Params: c.params, Siguientes: c.extender(seq, cuantos), Confianza: "media"}
	if len(seq) >= c.params+2 {
		r.Confianza = "alta"
	}
	if len(r.Siguientes) == 0 {
		return Regla{}, errors.New("puzles: no pude calcular el siguiente término")
	}
	return r, nil
}
