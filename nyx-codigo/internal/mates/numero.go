// Package mates is the exact arithmetic of Nyx Código: expressions with big.Rat and exact square
// roots, number theory (primes, factorization, mcd/mcm, divisors, percentages), equations (linear,
// quadratic, rational roots, Durand-Kerner, Gauss-Jordan systems), linear inequalities and word
// problems (§4.9). Every answer is checked before it is returned (substitution, re-calculation).
//
// It imports only the standard library and internal/nucleo.
package mates

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

var (
	// ErrDemasiadoGrande: a power or factorial whose result would be too big to compute.
	ErrDemasiadoGrande = errors.New("es demasiado grande")
	// ErrDivisionCero: a division by zero.
	ErrDivisionCero = errors.New("no se puede dividir entre cero")
)

const (
	maxBitsPotencia = 200_000 // integer powers are capped at this many result bits
	maxFactorial    = 5_000
	maxCifrasTexto  = 2_000 // longer integers are abbreviated in String
)

// Radical is Coef·√Radicando.
type Radical struct {
	Coef      *big.Rat // Coef·√Radicando
	Radicando *big.Int // square-free, > 1
}

// Numero is a result: an exact rational, an exact Mas ± Coef·√Radicando, or an approximation.
type Numero struct {
	Exacto   *big.Rat // nil if not rational
	Mas      *big.Rat // rational part when Radical != nil: Mas ± Radical
	Radical  *Radical
	Aprox    float64
	EsExacto bool
}

// String: "11/12", "4", "3 + 2√5", "(5 + √13)/2", "≈ 1,4142" (Spanish decimal comma).
func (n Numero) String() string {
	switch {
	case n.Exacto != nil:
		return textoRat(n.Exacto)
	case n.Radical != nil:
		return textoRadical(n.Mas, n.Radical.Coef, n.Radical.Radicando)
	}
	return "≈ " + textoFloat(n.Aprox)
}

// textoRat writes a rational as "4", "-3/4"; integers with more than maxCifrasTexto digits are abbreviated.
func textoRat(r *big.Rat) string {
	if r.IsInt() {
		s := r.Num().String()
		cifras := len(strings.TrimPrefix(s, "-"))
		if cifras > maxCifrasTexto {
			return s[:20] + "…" + s[len(s)-10:] + " (" + strconv.Itoa(cifras) + " cifras)"
		}
		return s
	}
	return r.RatString()
}

// textoRadical writes a + b√r: "2√5", "√5", "-√5", "3 + 2√5", "(5 + √13)/2", "√13/2".
func textoRadical(a, b *big.Rat, r *big.Int) string {
	if a == nil {
		a = new(big.Rat)
	}
	// common denominator
	d := new(big.Int).Set(b.Denom())
	if a.Sign() != 0 {
		d = mcmInt(d, a.Denom())
	}
	dr := new(big.Rat).SetInt(d)
	an := new(big.Rat).Mul(a, dr) // integer
	bn := new(big.Rat).Mul(b, dr) // integer
	raiz := "√" + r.String()
	termino := func(c *big.Rat, conSigno bool) string {
		abs := new(big.Rat).Abs(c)
		s := raiz
		if abs.Cmp(big.NewRat(1, 1)) != 0 {
			s = textoRat(abs) + raiz
		}
		if c.Sign() < 0 && !conSigno {
			return "-" + s
		}
		return s
	}
	var cuerpo string
	if an.Sign() == 0 {
		cuerpo = termino(bn, false)
	} else {
		signo := " + "
		if bn.Sign() < 0 {
			signo = " - "
		}
		cuerpo = textoRat(an) + signo + termino(bn, true)
	}
	if d.Cmp(big.NewInt(1)) == 0 {
		return cuerpo
	}
	if an.Sign() == 0 {
		return cuerpo + "/" + d.String()
	}
	return "(" + cuerpo + ")/" + d.String()
}

// textoFloat writes a float with a Spanish decimal comma: 4 decimals, trailing zeros removed;
// scientific notation for very big or very small numbers.
func textoFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "indefinido"
	case math.IsInf(f, 1):
		return "infinito"
	case math.IsInf(f, -1):
		return "-infinito"
	}
	a := math.Abs(f)
	var s string
	if a != 0 && (a >= 1e15 || a < 1e-4) {
		s = strconv.FormatFloat(f, 'e', 5, 64)
		mant, exp, _ := strings.Cut(s, "e")
		if strings.Contains(mant, ".") {
			mant = strings.TrimRight(strings.TrimRight(mant, "0"), ".")
		}
		e, _ := strconv.Atoi(exp)
		s = mant + "·10^" + strconv.Itoa(e)
	} else {
		s = strconv.FormatFloat(f, 'f', 4, 64)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		if s == "-0" {
			s = "0"
		}
	}
	return strings.Replace(s, ".", ",", 1)
}

// Decimal returns the value with a Spanish decimal comma and at most 4 decimals ("0,9167"), for
// showing next to an exact fraction.
func (n Numero) Decimal() string { return textoFloat(n.Aprox) }

// ---- exact values: a + b·√r, or an approximation ----

// valor is the internal value: rational a (b == nil), a + b√r (b != 0, r square-free > 1), or aprox f.
type valor struct {
	a, b  *big.Rat
	r     *big.Int
	f     float64
	aprox bool
}

func vRat(r *big.Rat) valor        { return valor{a: r} }
func vInt(i int64) valor           { return valor{a: big.NewRat(i, 1)} }
func vAprox(f float64) valor       { return valor{f: f, aprox: true} }
func (v valor) racional() bool     { return !v.aprox && (v.b == nil || v.b.Sign() == 0) }
func (v valor) entero() bool       { return v.racional() && v.a.IsInt() }
func (v valor) cero() bool         { return v.racional() && v.a.Sign() == 0 }
func (v valor) conRadical() bool   { return !v.aprox && v.b != nil && v.b.Sign() != 0 }
func copiaRat(r *big.Rat) *big.Rat { return new(big.Rat).Set(r) }

// float returns the value as a float64.
func (v valor) float() float64 {
	if v.aprox {
		return v.f
	}
	f, _ := v.a.Float64()
	if v.conRadical() {
		bf, _ := v.b.Float64()
		rf, _ := new(big.Float).SetInt(v.r).Float64()
		f += bf * math.Sqrt(rf)
	}
	return f
}

// signo of the value (exact for a + b√r).
func (v valor) signo() int {
	if v.aprox {
		switch {
		case v.f > 0:
			return 1
		case v.f < 0:
			return -1
		}
		return 0
	}
	if !v.conRadical() {
		return v.a.Sign()
	}
	// a + b√r: compare a² with b²r when the signs differ
	sa, sb := v.a.Sign(), v.b.Sign()
	if sa == 0 || sa == sb {
		return sb
	}
	a2 := new(big.Rat).Mul(v.a, v.a)
	b2r := new(big.Rat).Mul(v.b, v.b)
	b2r.Mul(b2r, new(big.Rat).SetInt(v.r))
	if a2.Cmp(b2r) > 0 {
		return sa
	}
	return sb
}

func (v valor) numero() Numero {
	switch {
	case !v.aprox && v.a == nil:
		return Numero{}
	case v.aprox:
		return Numero{Aprox: v.f}
	case v.conRadical():
		return Numero{Mas: copiaRat(v.a), Radical: &Radical{Coef: copiaRat(v.b), Radicando: new(big.Int).Set(v.r)},
			Aprox: v.float(), EsExacto: true}
	}
	return Numero{Exacto: copiaRat(v.a), Aprox: v.float(), EsExacto: true}
}

// texto renders the value inside an expression (no "≈").
func (v valor) texto() string {
	switch {
	case v.aprox:
		return textoFloat(v.f)
	case v.conRadical():
		return textoRadical(v.a, v.b, v.r)
	}
	return textoRat(v.a)
}

// igual compares two values: exactly when both are exact, else with a relative tolerance.
func (v valor) igual(w valor) bool {
	if !v.aprox && !w.aprox {
		d := restar(v, w)
		if !d.aprox {
			return d.signo() == 0
		}
	}
	x, y := v.float(), w.float()
	return math.Abs(x-y) <= 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y)))
}

func normal(v valor) valor {
	if v.aprox {
		return v
	}
	if v.b != nil && v.b.Sign() == 0 {
		v.b, v.r = nil, nil
	}
	return v
}

func neg(v valor) valor {
	if v.aprox {
		return vAprox(-v.f)
	}
	w := valor{a: new(big.Rat).Neg(v.a)}
	if v.conRadical() {
		w.b, w.r = new(big.Rat).Neg(v.b), v.r
	}
	return w
}

// mismoRadical reports whether v and w can be combined exactly (same radicand, or rational).
func mismoRadical(v, w valor) (*big.Int, bool) {
	if v.aprox || w.aprox {
		return nil, false
	}
	switch {
	case !v.conRadical() && !w.conRadical():
		return nil, true
	case !v.conRadical():
		return w.r, true
	case !w.conRadical():
		return v.r, true
	case v.r.Cmp(w.r) == 0:
		return v.r, true
	}
	return nil, false
}

func coefB(v valor) *big.Rat {
	if v.conRadical() {
		return v.b
	}
	return new(big.Rat)
}

func sumar(v, w valor) valor {
	r, ok := mismoRadical(v, w)
	if !ok {
		return vAprox(v.float() + w.float())
	}
	s := valor{a: new(big.Rat).Add(v.a, w.a)}
	if r != nil {
		s.b, s.r = new(big.Rat).Add(coefB(v), coefB(w)), r
	}
	return normal(s)
}

func restar(v, w valor) valor { return sumar(v, neg(w)) }

func multiplicar(v, w valor) valor {
	r, ok := mismoRadical(v, w)
	if !ok {
		// two different radicals: √p·√q = √(pq)
		if !v.aprox && !w.aprox && v.a.Sign() == 0 && w.a.Sign() == 0 {
			coef := new(big.Rat).Mul(v.b, w.b)
			x := raizRacional(new(big.Rat).SetInt(new(big.Int).Mul(v.r, w.r)))
			return multiplicar(vRat(coef), x)
		}
		return vAprox(v.float() * w.float())
	}
	if r == nil {
		return vRat(new(big.Rat).Mul(v.a, w.a))
	}
	// (a + b√r)(c + d√r) = (ac + bdr) + (ad + bc)√r
	a, b, c, d := v.a, coefB(v), w.a, coefB(w)
	rr := new(big.Rat).SetInt(r)
	pa := new(big.Rat).Mul(a, c)
	bd := new(big.Rat).Mul(b, d)
	pa.Add(pa, bd.Mul(bd, rr))
	pb := new(big.Rat).Mul(a, d)
	pb.Add(pb, new(big.Rat).Mul(b, c))
	return normal(valor{a: pa, b: pb, r: r})
}

func dividir(v, w valor) (valor, error) {
	if w.signo() == 0 {
		return valor{}, ErrDivisionCero
	}
	if w.aprox || v.aprox {
		return vAprox(v.float() / w.float()), nil
	}
	if !w.conRadical() {
		inv := new(big.Rat).Inv(w.a)
		return multiplicar(v, vRat(inv)), nil
	}
	// rationalize: 1/(c + d√r) = (c - d√r)/(c² - d²r)
	c, d := w.a, w.b
	den := new(big.Rat).Mul(c, c)
	d2r := new(big.Rat).Mul(d, d)
	den.Sub(den, d2r.Mul(d2r, new(big.Rat).SetInt(w.r)))
	if den.Sign() == 0 {
		return vAprox(v.float() / w.float()), nil
	}
	conj := valor{a: new(big.Rat).Quo(c, den), b: new(big.Rat).Neg(new(big.Rat).Quo(d, den)), r: w.r}
	return multiplicar(v, normal(conj)), nil
}

// raizRacional returns √q exactly (q ≥ 0): (k/d)·√s with s square-free.
func raizRacional(q *big.Rat) valor {
	if q.Sign() == 0 {
		return vInt(0)
	}
	m := new(big.Int).Mul(q.Num(), q.Denom()) // √(n/d) = √(n·d)/d
	k, s := extraerCuadrados(m)
	coef := new(big.Rat).SetFrac(k, q.Denom())
	if s.Cmp(big.NewInt(1)) == 0 {
		return vRat(coef)
	}
	return valor{a: new(big.Rat), b: coef, r: s}
}

// extraerCuadrados writes m = k²·s with s free of square factors found by trial division up to 10⁶
// (a remaining cofactor that is a perfect square is extracted too).
func extraerCuadrados(m *big.Int) (k, s *big.Int) {
	k, s = big.NewInt(1), new(big.Int).Set(m)
	if r := new(big.Int).Sqrt(s); new(big.Int).Mul(r, r).Cmp(s) == 0 {
		return r, big.NewInt(1)
	}
	var p, p2, q, rem big.Int
	for i := int64(2); i <= 1_000_000; i++ {
		p.SetInt64(i)
		p2.Mul(&p, &p)
		if p2.Cmp(s) > 0 {
			break
		}
		for {
			q.QuoRem(s, &p2, &rem)
			if rem.Sign() != 0 {
				break
			}
			s.Set(&q)
			k.Mul(k, &p)
		}
		if i > 2 {
			i++ // odd numbers only after 2
		}
	}
	if r := new(big.Int).Sqrt(s); new(big.Int).Mul(r, r).Cmp(s) == 0 {
		k.Mul(k, r)
		s.SetInt64(1)
	}
	return k, s
}

func raiz(v valor) (valor, error) {
	if v.signo() < 0 {
		return valor{}, errors.New("no hay raíz cuadrada real de un número negativo")
	}
	if v.racional() {
		return raizRacional(v.a), nil
	}
	return vAprox(math.Sqrt(v.float())), nil
}

// bitsRat is the size of a rational in bits (numerator or denominator, the larger).
func bitsRat(r *big.Rat) int {
	return max(r.Num().BitLen(), r.Denom().BitLen())
}

func potencia(base, exp valor) (valor, error) {
	if exp.aprox || exp.conRadical() || base.aprox {
		return vAprox(math.Pow(base.float(), exp.float())), nil
	}
	e := exp.a
	if e.IsInt() {
		if !e.Num().IsInt64() || e.Num().Int64() > 1<<40 || e.Num().Int64() < -(1<<40) {
			if base.racional() && (base.a.Cmp(big.NewRat(1, 1)) == 0 || base.a.Sign() == 0) {
				return base, nil
			}
			return valor{}, ErrDemasiadoGrande
		}
		k := e.Num().Int64()
		if base.racional() {
			return potenciaRat(base.a, k)
		}
		// a + b√r: repeated squaring, small exponents only
		if k < -64 || k > 64 {
			return vAprox(math.Pow(base.float(), float64(k))), nil
		}
		neg := k < 0
		if neg {
			k = -k
		}
		res, b := vInt(1), base
		for k > 0 {
			if k&1 == 1 {
				res = multiplicar(res, b)
			}
			b = multiplicar(b, b)
			k >>= 1
		}
		if neg {
			return dividir(vInt(1), res)
		}
		return res, nil
	}
	// rational exponent p/q
	if !base.racional() {
		return vAprox(math.Pow(base.float(), exp.float())), nil
	}
	p, q := e.Num(), e.Denom()
	if q.IsInt64() && q.Int64() <= 64 && p.IsInt64() {
		qq := q.Int64()
		if base.a.Sign() < 0 && qq%2 == 0 {
			return valor{}, errors.New("no hay raíz real de un número negativo con índice par")
		}
		n, okn := raizEntera(new(big.Int).Abs(base.a.Num()), qq)
		d, okd := raizEntera(base.a.Denom(), qq)
		if okn && okd {
			r := new(big.Rat).SetFrac(n, d)
			if base.a.Sign() < 0 {
				r.Neg(r)
			}
			return potenciaRat(r, p.Int64())
		}
		if qq == 2 && p.Int64() >= -64 && p.Int64() <= 64 {
			s, err := raiz(base)
			if err != nil {
				return valor{}, err
			}
			return potencia(s, vInt(p.Int64()))
		}
	}
	f := math.Pow(math.Abs(base.float()), exp.float())
	if base.a.Sign() < 0 {
		f = -f
	}
	return vAprox(f), nil
}

func potenciaRat(b *big.Rat, k int64) (valor, error) {
	if k == 0 {
		return vInt(1), nil
	}
	if b.Sign() == 0 {
		if k < 0 {
			return valor{}, ErrDivisionCero
		}
		return vInt(0), nil
	}
	abs := k
	if abs < 0 {
		abs = -abs
	}
	// |1| stays small whatever the exponent
	if new(big.Rat).Abs(b).Cmp(big.NewRat(1, 1)) == 0 {
		if b.Sign() < 0 && abs%2 == 1 {
			return vInt(-1), nil
		}
		return vInt(1), nil
	}
	bits := int64(max(bitsRat(b), 1))
	if abs > maxBitsPotencia || abs*bits > maxBitsPotencia {
		return valor{}, ErrDemasiadoGrande
	}
	n := new(big.Int).Exp(b.Num(), big.NewInt(abs), nil)
	d := new(big.Int).Exp(b.Denom(), big.NewInt(abs), nil)
	r := new(big.Rat).SetFrac(n, d)
	if k < 0 {
		r.Inv(r)
	}
	return vRat(r), nil
}

// raizEntera returns the exact q-th root of n ≥ 0, if there is one.
func raizEntera(n *big.Int, q int64) (*big.Int, bool) {
	if n.Sign() == 0 || n.Cmp(big.NewInt(1)) == 0 {
		return new(big.Int).Set(n), true
	}
	if q == 1 {
		return new(big.Int).Set(n), true
	}
	if q == 2 {
		r := new(big.Int).Sqrt(n)
		return r, new(big.Int).Mul(r, r).Cmp(n) == 0
	}
	// binary search on [1, 2^(bits/q + 1)]
	lo := big.NewInt(1)
	hi := new(big.Int).Lsh(big.NewInt(1), uint(n.BitLen()/int(q)+1))
	qq := big.NewInt(q)
	var mid, p big.Int
	for lo.Cmp(hi) <= 0 {
		mid.Add(lo, hi)
		mid.Rsh(&mid, 1)
		p.Exp(&mid, qq, nil)
		switch p.Cmp(n) {
		case 0:
			return new(big.Int).Set(&mid), true
		case -1:
			lo = new(big.Int).Add(&mid, big.NewInt(1))
		default:
			hi = new(big.Int).Sub(&mid, big.NewInt(1))
		}
	}
	return nil, false
}

func factorial(v valor) (valor, error) {
	if !v.entero() || v.a.Sign() < 0 {
		return valor{}, errors.New("el factorial solo existe para enteros no negativos")
	}
	if !v.a.Num().IsInt64() || v.a.Num().Int64() > maxFactorial {
		return valor{}, ErrDemasiadoGrande
	}
	n := v.a.Num().Int64()
	if n < 2 {
		return vInt(1), nil
	}
	return vRat(new(big.Rat).SetInt(new(big.Int).MulRange(1, n))), nil
}

func modulo(v, w valor) (valor, error) {
	if !v.entero() || !w.entero() {
		return valor{}, errors.New("el resto solo tiene sentido con números enteros")
	}
	if w.a.Sign() == 0 {
		return valor{}, ErrDivisionCero
	}
	m := new(big.Int).Mod(v.a.Num(), new(big.Int).Abs(w.a.Num())) // Euclidean: 0 ≤ m < |w|
	return vRat(new(big.Rat).SetInt(m)), nil
}

func mcmInt(a, b *big.Int) *big.Int {
	g := new(big.Int).GCD(nil, nil, a, b)
	if g.Sign() == 0 {
		return new(big.Int)
	}
	m := new(big.Int).Mul(a, b)
	m.Quo(m, g)
	return m.Abs(m)
}
