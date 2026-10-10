package mates

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// poli is a polynomial in several variables with rational coefficients. A monomial key is canonical:
// "" (constant), "x", "x^2", "x·y" (variables sorted, exponents > 1 written).
type poli map[string]*big.Rat

// fraccion is a rational function num/den.
type fraccion struct{ num, den poli }

const maxGradoPotencia = 40

func polConst(r *big.Rat) poli {
	p := poli{}
	if r.Sign() != 0 {
		p[""] = new(big.Rat).Set(r)
	}
	return p
}

func polVar(x string) poli { return poli{x: big.NewRat(1, 1)} }

func decodificar(m string) map[string]int {
	out := map[string]int{}
	if m == "" {
		return out
	}
	for _, f := range strings.Split(m, "·") {
		v, e, ok := strings.Cut(f, "^")
		k := 1
		if ok {
			k, _ = strconv.Atoi(e)
		}
		out[v] += k
	}
	return out
}

func codificar(m map[string]int) string {
	vs := make([]string, 0, len(m))
	for v, e := range m {
		if e != 0 {
			vs = append(vs, v)
		}
	}
	sort.Strings(vs)
	partes := make([]string, len(vs))
	for i, v := range vs {
		if m[v] == 1 {
			partes[i] = v
		} else {
			partes[i] = v + "^" + strconv.Itoa(m[v])
		}
	}
	return strings.Join(partes, "·")
}

func mulMonomios(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	m := decodificar(a)
	for v, e := range decodificar(b) {
		m[v] += e
	}
	return codificar(m)
}

func (p poli) copia() poli {
	q := poli{}
	for m, c := range p {
		q[m] = new(big.Rat).Set(c)
	}
	return q
}

func (p poli) sumar(q poli, signo int) poli {
	r := p.copia()
	for m, c := range q {
		x, ok := r[m]
		if !ok {
			x = new(big.Rat)
			r[m] = x
		}
		if signo < 0 {
			x.Sub(x, c)
		} else {
			x.Add(x, c)
		}
		if x.Sign() == 0 {
			delete(r, m)
		}
	}
	return r
}

func (p poli) mul(q poli) poli {
	r := poli{}
	for m1, c1 := range p {
		for m2, c2 := range q {
			m := mulMonomios(m1, m2)
			x, ok := r[m]
			if !ok {
				x = new(big.Rat)
				r[m] = x
			}
			x.Add(x, new(big.Rat).Mul(c1, c2))
			if x.Sign() == 0 {
				delete(r, m)
			}
		}
	}
	return r
}

func (p poli) escalar(c *big.Rat) poli {
	r := poli{}
	if c.Sign() == 0 {
		return r
	}
	for m, x := range p {
		r[m] = new(big.Rat).Mul(x, c)
	}
	return r
}

func (p poli) cero() bool { return len(p) == 0 }

// constante returns the value when p has no variables.
func (p poli) constante() (*big.Rat, bool) {
	for m := range p {
		if m != "" {
			return nil, false
		}
	}
	if c, ok := p[""]; ok {
		return c, true
	}
	return new(big.Rat), true
}

func (p poli) gradoTotal() int {
	g := 0
	for m := range p {
		t := 0
		for _, e := range decodificar(m) {
			t += e
		}
		g = max(g, t)
	}
	return g
}

func (p poli) variables() []string {
	visto := map[string]bool{}
	for m := range p {
		for v := range decodificar(m) {
			visto[v] = true
		}
	}
	out := make([]string, 0, len(visto))
	for v := range visto {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func (p poli) igual(q poli) bool { return p.sumar(q, -1).cero() }

// coeficientes returns the coefficients of a polynomial in one variable x, index = degree.
func (p poli) coeficientes(x string) ([]*big.Rat, error) {
	g := 0
	for m := range p {
		d := decodificar(m)
		for v := range d {
			if v != x {
				return nil, fmt.Errorf("hay más de una incógnita")
			}
		}
		g = max(g, d[x])
	}
	cs := make([]*big.Rat, g+1)
	for i := range cs {
		cs[i] = new(big.Rat)
	}
	for m, c := range p {
		cs[decodificar(m)[x]].Set(c)
	}
	return cs, nil
}

func polDesdeCoefs(cs []*big.Rat, x string) poli {
	p := poli{}
	for i, c := range cs {
		if c.Sign() == 0 {
			continue
		}
		m := ""
		if i == 1 {
			m = x
		} else if i > 1 {
			m = x + "^" + strconv.Itoa(i)
		}
		p[m] = new(big.Rat).Set(c)
	}
	return p
}

// ---- rational functions ----

func fracPol(p poli) fraccion { return fraccion{num: p, den: polConst(big.NewRat(1, 1))} }

func (f fraccion) normal() fraccion {
	if c, ok := f.den.constante(); ok && c.Sign() != 0 && c.Cmp(big.NewRat(1, 1)) != 0 {
		return fracPol(f.num.escalar(new(big.Rat).Inv(c)))
	}
	if f.num.cero() {
		return fracPol(poli{})
	}
	return f
}

func (f fraccion) sumar(g fraccion, signo int) fraccion {
	if f.den.igual(g.den) {
		return fraccion{num: f.num.sumar(g.num, signo), den: f.den}.normal()
	}
	return fraccion{num: f.num.mul(g.den).sumar(g.num.mul(f.den), signo), den: f.den.mul(g.den)}.normal()
}

func (f fraccion) mul(g fraccion) fraccion {
	return fraccion{num: f.num.mul(g.num), den: f.den.mul(g.den)}.normal()
}

func (f fraccion) div(g fraccion) (fraccion, error) {
	if g.num.cero() {
		return fraccion{}, ErrDivisionCero
	}
	return fraccion{num: f.num.mul(g.den), den: f.den.mul(g.num)}.normal(), nil
}

func (f fraccion) potencia(k int) (fraccion, error) {
	if k < 0 {
		inv, err := fracPol(polConst(big.NewRat(1, 1))).div(f)
		if err != nil {
			return fraccion{}, err
		}
		return inv.potencia(-k)
	}
	r := fracPol(polConst(big.NewRat(1, 1)))
	for i := 0; i < k; i++ {
		r = r.mul(f)
	}
	return r, nil
}

// aFraccion turns an expression into a rational function of its unknowns.
func aFraccion(ctx context.Context, e Expr) (fraccion, error) {
	if _, esVar := e.(Var); !esVar && len(variables(e)) == 0 {
		if _, esNum := e.(Num); !esNum {
			return constanteDe(ctx, e)
		}
	}
	switch x := e.(type) {
	case Num:
		return fracPol(polConst(x.V)), nil
	case Val:
		if !x.v.racional() {
			return fraccion{}, fmt.Errorf("%w: coeficientes que no son fracciones", nucleo.ErrNoSoportado)
		}
		return fracPol(polConst(x.v.a)), nil
	case Var:
		return fracPol(polVar(x.Nombre)), nil
	case Neg:
		a, err := aFraccion(ctx, x.A)
		if err != nil {
			return fraccion{}, err
		}
		return fraccion{num: a.num.escalar(big.NewRat(-1, 1)), den: a.den}, nil
	case Op:
		if x.Op == '^' {
			a, err := aFraccion(ctx, x.A)
			if err != nil {
				return fraccion{}, err
			}
			k, err := exponenteConstante(ctx, x.B)
			if err != nil {
				return fraccion{}, err
			}
			if c, ok := a.num.constante(); ok {
				if d, ok := a.den.constante(); ok {
					v, err := potencia(vRat(new(big.Rat).Quo(c, d)), vInt(int64(k)))
					if err != nil {
						return fraccion{}, err
					}
					return fracPol(polConst(v.a)), nil
				}
			}
			if k > maxGradoPotencia || k < -maxGradoPotencia {
				return fraccion{}, fmt.Errorf("%w: el exponente %d es demasiado alto", nucleo.ErrNoSoportado, k)
			}
			return a.potencia(k)
		}
		if x.Op == '%' {
			return constanteDe(ctx, e)
		}
		a, err := aFraccion(ctx, x.A)
		if err != nil {
			return fraccion{}, err
		}
		b, err := aFraccion(ctx, x.B)
		if err != nil {
			return fraccion{}, err
		}
		switch x.Op {
		case '+':
			return a.sumar(b, 1), nil
		case '-':
			return a.sumar(b, -1), nil
		case '*':
			return a.mul(b), nil
		case '/':
			return a.div(b)
		}
	case Fact, Func:
		return constanteDe(ctx, e)
	}
	return fraccion{}, fmt.Errorf("%w: no sé convertir %s", nucleo.ErrNoSoportado, e)
}

// constanteDe evaluates a subexpression that must not contain unknowns and must be rational.
func constanteDe(ctx context.Context, e Expr) (fraccion, error) {
	if len(variables(e)) > 0 {
		return fraccion{}, fmt.Errorf("%w: la incógnita está dentro de %s", nucleo.ErrNoSoportado, e)
	}
	v, err := evaluar(ctx, e, nil)
	if err != nil {
		return fraccion{}, err
	}
	if !v.racional() {
		return fraccion{}, fmt.Errorf("%w: coeficientes que no son fracciones (%s)", nucleo.ErrNoSoportado, e)
	}
	return fracPol(polConst(v.a)), nil
}

func exponenteConstante(ctx context.Context, e Expr) (int, error) {
	if len(variables(e)) > 0 {
		return 0, fmt.Errorf("%w: la incógnita está en un exponente", nucleo.ErrNoSoportado)
	}
	v, err := evaluar(ctx, e, nil)
	if err != nil {
		return 0, err
	}
	if !v.entero() || !v.a.Num().IsInt64() || v.a.Num().Int64() > 1_000_000 || v.a.Num().Int64() < -1_000_000 {
		return 0, fmt.Errorf("%w: el exponente no es un entero pequeño", nucleo.ErrNoSoportado)
	}
	return int(v.a.Num().Int64()), nil
}

// ---- text ----

// textoCoef writes a coefficient before a variable: "", "-", "3", "-3", "(1/2)".
func textoCoef(c *big.Rat) string {
	switch {
	case c.Cmp(big.NewRat(1, 1)) == 0:
		return ""
	case c.Cmp(big.NewRat(-1, 1)) == 0:
		return "-"
	case c.IsInt():
		return c.Num().String()
	}
	return "(" + c.RatString() + ")"
}

// textoPoli writes a polynomial: "x^2 - 5x + 6", "2x + 3y - 1", "0".
func textoPoli(p poli) string {
	if p.cero() {
		return "0"
	}
	ms := make([]string, 0, len(p))
	for m := range p {
		ms = append(ms, m)
	}
	// higher degree first, then alphabetical; the constant last
	sort.Slice(ms, func(i, j int) bool {
		gi, gj := gradoMonomio(ms[i]), gradoMonomio(ms[j])
		if gi != gj {
			return gi > gj
		}
		return ms[i] < ms[j]
	})
	var sb strings.Builder
	for i, m := range ms {
		c := p[m]
		abs := new(big.Rat).Abs(c)
		if i == 0 {
			if c.Sign() < 0 {
				sb.WriteString("-")
			}
		} else if c.Sign() < 0 {
			sb.WriteString(" - ")
		} else {
			sb.WriteString(" + ")
		}
		if m == "" {
			sb.WriteString(abs.RatString())
			continue
		}
		cf := textoCoef(abs)
		sb.WriteString(cf)
		sb.WriteString(strings.ReplaceAll(m, "·", ""))
	}
	return sb.String()
}

func gradoMonomio(m string) int {
	t := 0
	for _, e := range decodificar(m) {
		t += e
	}
	return t
}

func textoFraccion(f fraccion) string {
	if c, ok := f.den.constante(); ok && c.Cmp(big.NewRat(1, 1)) == 0 {
		return textoPoli(f.num)
	}
	return "(" + textoPoli(f.num) + ")/(" + textoPoli(f.den) + ")"
}

// evaluarPoli evaluates p with exact values for its variables.
func evaluarPoli(p poli, vals map[string]valor) valor {
	total := vInt(0)
	for m, c := range p {
		t := vRat(c)
		for v, e := range decodificar(m) {
			for k := 0; k < e; k++ {
				t = multiplicar(t, vals[v])
			}
		}
		total = sumar(total, t)
	}
	return total
}

// horner evaluates a univariate polynomial (coefficients by degree) at a rational point.
func horner(cs []*big.Rat, x *big.Rat) *big.Rat {
	r := new(big.Rat)
	for i := len(cs) - 1; i >= 0; i-- {
		r.Mul(r, x)
		r.Add(r, cs[i])
	}
	return r
}

// dividirRuffini divides by (x - r): returns the quotient's coefficients (the remainder must be 0).
func dividirRuffini(cs []*big.Rat, r *big.Rat) []*big.Rat {
	n := len(cs) - 1
	q := make([]*big.Rat, n)
	acc := new(big.Rat)
	for i := n; i >= 1; i-- {
		acc = new(big.Rat).Add(new(big.Rat).Mul(acc, r), cs[i])
		q[i-1] = acc
	}
	return q
}

func grado(cs []*big.Rat) int {
	for i := len(cs) - 1; i >= 0; i-- {
		if cs[i].Sign() != 0 {
			return i
		}
	}
	return -1
}
