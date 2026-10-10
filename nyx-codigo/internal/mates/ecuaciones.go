package mates

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/cmplx"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Solucion is the result of Ecuaciones.
type Solucion struct {
	Variables  []string
	Valores    map[string][]Numero
	Tipo       string   // "unica" | "varias" | "ninguna" | "infinitas" | "aproximada"
	Pasos      []string // Spanish named moves
	Condicion  []string // "x ≠ 2"
	Comprobado bool     // substitution check passed
	// Comprobacion holds the substitution lines, e.g. "2·4 + 3 = 11 ✔" (additive).
	Comprobacion []string
	// Relacion describes infinitely many solutions, e.g. "y = 10 - x" (additive).
	Relacion []string
}

type ecuacion struct {
	texto  string
	l, r   Expr
	fl, fr fraccion
}

// diferencia is l - r as a rational function.
func (e ecuacion) diferencia() fraccion { return e.fl.sumar(e.fr, -1) }

func (e ecuacion) String() string { return e.l.String() + " = " + e.r.String() }

// partirEcuaciones splits "x+y=10, x-y=2" or "x+y=10; x-y=2" or "x+y=10 y x-y=2" into equations.
// A comma between digits ("3,5") is a decimal comma.
func partirEcuaciones(s string) []string {
	var out []string
	prof, ini := 0, 0
	corta := func(i, salto int) {
		if p := strings.TrimSpace(s[ini:i]); p != "" {
			out = append(out, p)
		}
		ini = i + salto
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[':
			prof++
		case ')', ']':
			prof--
		case ';', '\n':
			if prof == 0 {
				corta(i, 1)
			}
		case ',':
			decimal := i > 0 && i+1 < len(s) && esDig(s[i-1]) && esDig(s[i+1])
			if prof == 0 && !decimal {
				corta(i, 1)
			}
		case ' ':
			if prof == 0 && strings.HasPrefix(s[i:], " y ") && strings.Contains(s[ini:i], "=") && strings.Contains(s[i+3:], "=") {
				corta(i, 3)
				i += 2
			}
		}
	}
	corta(len(s), 0)
	return out
}

func esDig(c byte) bool { return c >= '0' && c <= '9' }

func analizarEcuacion(ctx context.Context, s string) (ecuacion, error) {
	s = strings.ReplaceAll(s, "==", "=")
	partes := strings.Split(s, "=")
	if len(partes) != 2 {
		if len(partes) < 2 {
			return ecuacion{}, fmt.Errorf("%w: «%s» no tiene «=»", nucleo.ErrNoEntiendo, s)
		}
		return ecuacion{}, fmt.Errorf("%w: «%s» tiene más de un «=»", nucleo.ErrNoEntiendo, s)
	}
	l, err := AnalizarExpr(partes[0])
	if err != nil {
		return ecuacion{}, err
	}
	r, err := AnalizarExpr(partes[1])
	if err != nil {
		return ecuacion{}, err
	}
	fl, err := aFraccion(ctx, l)
	if err != nil {
		return ecuacion{}, err
	}
	fr, err := aFraccion(ctx, r)
	if err != nil {
		return ecuacion{}, err
	}
	return ecuacion{texto: s, l: l, r: r, fl: fl, fr: fr}, nil
}

// Ecuaciones solves one equation or a system. One unknown: named moves (degree 1), the discriminant
// with exact radicals (degree 2), rational roots plus Ruffini (degrees 3–4), Durand-Kerner when no
// exact root is left ("aproximada"). Several linear equations: Gauss-Jordan over big.Rat. Two
// equations where one is linear: substitution. Every root is substituted back; Comprobado says so.
func Ecuaciones(ctx context.Context, ecs []string, n *nucleo.Nodo) (Solucion, error) {
	var lista []ecuacion
	for _, s := range ecs {
		for _, p := range partirEcuaciones(s) {
			e, err := analizarEcuacion(ctx, p)
			if err != nil {
				return Solucion{}, err
			}
			lista = append(lista, e)
		}
	}
	if len(lista) == 0 {
		return Solucion{}, fmt.Errorf("%w: no veo ninguna ecuación", nucleo.ErrNoEntiendo)
	}
	var vars []string
	visto := map[string]bool{}
	for _, e := range lista {
		for _, v := range append(variables(e.l), variables(e.r)...) {
			if !visto[v] {
				visto[v] = true
				vars = append(vars, v)
			}
		}
	}
	r := &resolucion{ctx: ctx, n: n, sol: Solucion{Variables: vars, Valores: map[string][]Numero{}}}
	var err error
	switch {
	case len(vars) == 0:
		err = r.sinIncognitas(lista)
	case len(lista) == 1 && len(vars) == 1:
		err = r.una(lista[0], vars[0])
	case todasLineales(lista):
		err = r.sistemaLineal(lista, vars)
	case len(lista) == 2 && len(vars) == 2:
		err = r.sustitucion(lista, vars)
	default:
		err = fmt.Errorf("%w: solo sé resolver sistemas lineales, o de dos ecuaciones con una lineal", nucleo.ErrNoSoportado)
	}
	if err != nil {
		if n != nil {
			n.Sub(nucleo.PasoError, "No he podido resolverlo: %s", mensaje(err))
		}
		return r.sol, err
	}
	if !r.sol.Comprobado && r.sol.Tipo != "ninguna" {
		if n != nil {
			n.Sub(nucleo.PasoError, "La comprobación no ha salido bien")
		}
	}
	return r.sol, nil
}

type resolucion struct {
	ctx context.Context
	n   *nucleo.Nodo
	sol Solucion
}

func (r *resolucion) paso(formato string, a ...any) {
	s := fmt.Sprintf(formato, a...)
	r.sol.Pasos = append(r.sol.Pasos, s)
	r.n.Sub(nucleo.PasoPlan, "%s", s).Info("")
}

func (r *resolucion) sinIncognitas(lista []ecuacion) error {
	todas := true
	for _, e := range lista {
		a, err := evaluar(r.ctx, e.l, nil)
		if err != nil {
			return err
		}
		b, err := evaluar(r.ctx, e.r, nil)
		if err != nil {
			return err
		}
		if a.igual(b) {
			r.paso("%s es cierto (%s = %s)", e, a.texto(), b.texto())
		} else {
			r.paso("%s es falso (%s ≠ %s)", e, a.texto(), b.texto())
			todas = false
		}
	}
	r.sol.Tipo = "ninguna"
	if todas {
		r.sol.Tipo = "infinitas"
	}
	r.sol.Comprobado = true
	return nil
}

// ---- one equation, one unknown ----

func (r *resolucion) una(e ecuacion, x string) error {
	r.paso("Ecuación: %s", e)
	dl, okl := e.fl.den.constante()
	dr, okr := e.fr.den.constante()
	if okl && okr && dl.Sign() != 0 && dr.Sign() != 0 && e.fl.num.gradoTotal() <= 1 && e.fr.num.gradoTotal() <= 1 {
		return r.lineal(e, x)
	}
	d := e.diferencia()
	num := d.num
	var den poli
	if c, ok := d.den.constante(); !ok || c.Sign() == 0 {
		den = d.den
		r.paso("Multiplico los dos lados por %s (debe ser distinto de 0)", textoPoli(den))
		r.condiciones(den, x)
	}
	cs, err := num.coeficientes(x)
	if err != nil {
		return err
	}
	// integer coefficients look better
	cs = enteros(cs)
	r.paso("Paso todo a un lado: %s = 0", textoPoli(polDesdeCoefs(cs, x)))
	raices, tipo, err := r.polinomio(cs, x)
	if err != nil {
		return err
	}
	if tipo == "ninguna" || tipo == "infinitas" {
		r.sol.Tipo = tipo
		r.sol.Comprobado = true
		if tipo == "infinitas" {
			r.sol.Relacion = []string{"cualquier " + x + condicionTexto(r.sol.Condicion)}
		}
		return nil
	}
	// conditions
	var validas []valor
	for _, v := range raices {
		if den != nil {
			if evaluarPoli(den, map[string]valor{x: v}).signo() == 0 {
				r.paso("%s = %s anula un denominador: no vale", x, v.texto())
				continue
			}
		}
		validas = append(validas, v)
	}
	return r.cerrar([]ecuacion{e}, x, validas, tipo)
}

func condicionTexto(c []string) string {
	if len(c) == 0 {
		return ""
	}
	return " con " + strings.Join(c, ", ")
}

// condiciones records "x ≠ a" for the rational roots of a denominator.
func (r *resolucion) condiciones(den poli, x string) {
	cs, err := den.coeficientes(x)
	if err != nil {
		r.sol.Condicion = append(r.sol.Condicion, textoPoli(den)+" ≠ 0")
		return
	}
	rs, resto := raicesRacionales(r.ctx, enteros(cs))
	for _, v := range rs {
		r.sol.Condicion = append(r.sol.Condicion, x+" ≠ "+textoRat(v))
	}
	if grado(resto) > 0 {
		r.sol.Condicion = append(r.sol.Condicion, textoPoli(polDesdeCoefs(resto, x))+" ≠ 0")
	}
}

// enteros multiplies the coefficients by the lcm of their denominators and divides by their gcd.
func enteros(cs []*big.Rat) []*big.Rat {
	l := big.NewInt(1)
	for _, c := range cs {
		l = mcmInt(l, c.Denom())
	}
	out := make([]*big.Rat, len(cs))
	g := new(big.Int)
	for i, c := range cs {
		out[i] = new(big.Rat).Mul(c, new(big.Rat).SetInt(l))
		g.GCD(nil, nil, g, out[i].Num())
	}
	if g.Sign() != 0 && g.Cmp(big.NewInt(1)) != 0 {
		inv := new(big.Rat).SetFrac(big.NewInt(1), g)
		for i := range out {
			out[i].Mul(out[i], inv)
		}
	}
	// positive leading coefficient
	if k := grado(out); k >= 0 && out[k].Sign() < 0 {
		for i := range out {
			out[i].Neg(out[i])
		}
	}
	return out
}

func lineal1(p poli, x string) (a, b *big.Rat) {
	a, b = new(big.Rat), new(big.Rat)
	if c, ok := p[x]; ok {
		a.Set(c)
	}
	if c, ok := p[""]; ok {
		b.Set(c)
	}
	return a, b
}

func sinPuntos(s string) string {
	return strings.NewReplacer("·", "", " ", "", "(", "", ")", "").Replace(s)
}

// textoLineal writes a·x + b: "2x + 3", "-x", "5".
func textoLineal(a, b *big.Rat, x string) string {
	p := poli{}
	if a.Sign() != 0 {
		p[x] = a
	}
	if b.Sign() != 0 {
		p[""] = b
	}
	return textoPoli(p)
}

func textoTermino(c *big.Rat, x string) string {
	abs := new(big.Rat).Abs(c)
	if x == "" {
		return abs.RatString()
	}
	return textoCoef(abs) + x
}

// lineal solves a·x + b = c·x + d with named moves.
func (r *resolucion) lineal(e ecuacion, x string) error {
	dl, _ := e.fl.den.constante()
	dr, _ := e.fr.den.constante()
	a1, b1 := lineal1(e.fl.num.escalar(new(big.Rat).Inv(dl)), x)
	a2, b2 := lineal1(e.fr.num.escalar(new(big.Rat).Inv(dr)), x)
	actual := textoLineal(a1, b1, x) + " = " + textoLineal(a2, b2, x)
	if sinPuntos(actual) != sinPuntos(e.String()) {
		r.paso("Simplifico: %s", actual)
	}
	// fractions: multiply by the lcm of the denominators
	l := big.NewInt(1)
	for _, c := range []*big.Rat{a1, b1, a2, b2} {
		l = mcmInt(l, c.Denom())
	}
	if l.Cmp(big.NewInt(1)) != 0 {
		m := new(big.Rat).SetInt(l)
		for _, c := range []*big.Rat{a1, b1, a2, b2} {
			c.Mul(c, m)
		}
		r.paso("Multiplico los dos lados por %s → %s = %s", l, textoLineal(a1, b1, x), textoLineal(a2, b2, x))
	}
	if a1.Sign() == 0 && a2.Sign() != 0 {
		a1, b1, a2, b2 = a2, b2, a1, b1
		r.paso("Doy la vuelta a la igualdad → %s = %s", textoLineal(a1, b1, x), textoLineal(a2, b2, x))
	}
	if a2.Sign() != 0 {
		verbo := "resto"
		if a2.Sign() < 0 {
			verbo = "sumo"
		}
		t := textoTermino(a2, x)
		a1.Sub(a1, a2)
		a2.SetInt64(0)
		r.paso("%s %s a ambos lados → %s = %s", mayuscula(verbo), t, textoLineal(a1, b1, x), textoLineal(a2, b2, x))
	}
	if b1.Sign() != 0 {
		verbo := "resto"
		if b1.Sign() < 0 {
			verbo = "sumo"
		}
		t := textoTermino(b1, "")
		b2.Sub(b2, b1)
		b1.SetInt64(0)
		r.paso("%s %s a ambos lados → %s = %s", mayuscula(verbo), t, textoLineal(a1, b1, x), textoLineal(a2, b2, x))
	}
	if a1.Sign() == 0 {
		r.sol.Comprobado = true
		if b2.Sign() == 0 {
			r.paso("Queda 0 = 0: vale cualquier %s", x)
			r.sol.Tipo = "infinitas"
			r.sol.Relacion = []string{"cualquier " + x}
			return nil
		}
		r.paso("Queda 0 = %s, que es imposible: no hay solución", b2.RatString())
		r.sol.Tipo = "ninguna"
		return nil
	}
	sol := new(big.Rat).Quo(b2, a1)
	switch {
	case a1.Cmp(big.NewRat(1, 1)) == 0:
	case a1.Cmp(big.NewRat(-1, 1)) == 0:
		r.paso("Cambio el signo a los dos lados → %s = %s", x, textoRat(sol))
	case a1.IsInt():
		r.paso("Divido entre %s → %s = %s", textoRat(a1), x, textoRat(sol))
	default:
		r.paso("Multiplico por %s → %s = %s", new(big.Rat).Inv(a1).RatString(), x, textoRat(sol))
	}
	return r.cerrar([]ecuacion{e}, x, []valor{vRat(sol)}, "")
}

func mayuscula(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// polinomio finds the real roots of a univariate polynomial (integer coefficients by degree).
// tipo is "ninguna"/"infinitas" for degree 0, "aproximada" when a root is approximate, else "".
func (r *resolucion) polinomio(cs []*big.Rat, x string) ([]valor, string, error) {
	g := grado(cs)
	switch g {
	case -1:
		r.paso("Queda 0 = 0: vale cualquier %s", x)
		return nil, "infinitas", nil
	case 0:
		r.paso("Queda %s = 0, que es imposible: no hay solución", cs[0].RatString())
		return nil, "ninguna", nil
	case 1:
		v := new(big.Rat).Quo(new(big.Rat).Neg(cs[0]), cs[1])
		r.paso("Despejo %s → %s = %s", x, x, textoRat(v))
		return []valor{vRat(v)}, "", nil
	case 2:
		vs := r.cuadratica(cs[:3], x)
		if len(vs) == 0 {
			return nil, "ninguna", nil
		}
		return vs, "", nil
	}
	// degree ≥ 3: x = 0, rational roots and Ruffini
	var out []valor
	cs = cs[:g+1]
	for grado(cs) > 0 && cs[0].Sign() == 0 {
		cs = cs[1:]
		out = append(out, vInt(0))
		r.paso("%s = 0 es raíz: saco factor común %s → queda %s", x, x, textoPoli(polDesdeCoefs(cs, x)))
	}
	rs, resto := raicesRacionales(r.ctx, enteros(cs))
	for _, v := range rs {
		out = append(out, vRat(v))
	}
	if len(rs) > 0 {
		cola := ""
		if grado(resto) > 0 {
			cola = "; queda " + textoPoli(polDesdeCoefs(resto, x))
		}
		r.paso("Pruebo los divisores (regla de Ruffini): %s %s%s", plural(len(rs), "es raíz", "son raíces"),
			listaRats(rs, x), cola)
	}
	tipo := ""
	switch k := grado(resto); {
	case k <= 0:
	case k == 1:
		v := new(big.Rat).Quo(new(big.Rat).Neg(resto[0]), resto[1])
		out = append(out, vRat(v))
	case k == 2:
		out = append(out, r.cuadratica(resto, x)...)
	default:
		if r.ctx.Err() != nil {
			return nil, "", nucleo.ErrSinTiempo
		}
		ap := durandKerner(resto)
		r.paso("No quedan raíces exactas: aproximo las de %s (Durand-Kerner)", textoPoli(polDesdeCoefs(resto, x)))
		for _, f := range ap {
			out = append(out, vAprox(f))
		}
		tipo = "aproximada"
	}
	if len(out) == 0 {
		return nil, "ninguna", nil
	}
	return out, tipo, nil
}

func plural(n int, uno, varios string) string {
	if n == 1 {
		return uno
	}
	return varios
}

func listaRats(rs []*big.Rat, x string) string {
	partes := make([]string, len(rs))
	for i, v := range rs {
		partes[i] = x + " = " + textoRat(v)
	}
	return strings.Join(partes, ", ")
}

// cuadratica solves a·x² + b·x + c = 0 (cs = [c, b, a]) with exact radicals.
func (r *resolucion) cuadratica(cs []*big.Rat, x string) []valor {
	c, b, a := cs[0], cs[1], cs[2]
	if b.Sign() == 0 {
		q := new(big.Rat).Quo(new(big.Rat).Neg(c), a)
		r.paso("Despejo %s² = %s", x, textoRat(q))
		if q.Sign() < 0 {
			r.paso("Ningún número al cuadrado da negativo: no hay solución real")
			return nil
		}
		s := raizRacional(q)
		if q.Sign() == 0 {
			r.paso("%s = 0 (solución doble)", x)
			return []valor{vInt(0)}
		}
		r.paso("Saco la raíz cuadrada → %s = ±%s", x, s.texto())
		return []valor{neg(s), s}
	}
	if c.Sign() == 0 {
		v := new(big.Rat).Quo(new(big.Rat).Neg(b), a)
		r.paso("Saco factor común %s: %s(%s) = 0 → %s = 0 o %s = %s", x, x, textoLineal(a, b, x), x, x, textoRat(v))
		out := []valor{vInt(0), vRat(v)}
		sort.Slice(out, func(i, j int) bool { return out[i].float() < out[j].float() })
		return out
	}
	delta := new(big.Rat).Mul(b, b)
	delta.Sub(delta, new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(a, c)))
	r.paso("Δ = b² - 4ac = %s - 4·%s·%s = %s", parentesis(new(big.Rat).Mul(b, b)), parentesis(a), parentesis(c), textoRat(delta))
	dosA := new(big.Rat).Mul(big.NewRat(2, 1), a)
	menosB := new(big.Rat).Neg(b)
	switch delta.Sign() {
	case -1:
		r.paso("Δ < 0: no hay soluciones reales")
		return nil
	case 0:
		v := new(big.Rat).Quo(menosB, dosA)
		r.paso("Δ = 0: una solución doble, %s = -b/(2a) = %s", x, textoRat(v))
		return []valor{vRat(v)}
	}
	s := raizRacional(delta)
	r.paso("%s = (-b ± √Δ)/(2a) = (%s ± %s)/%s", x, textoRat(menosB), s.texto(), textoRat(dosA))
	v1, _ := dividir(sumar(vRat(menosB), neg(s)), vRat(dosA))
	v2, _ := dividir(sumar(vRat(menosB), s), vRat(dosA))
	out := []valor{v1, v2}
	sort.Slice(out, func(i, j int) bool { return out[i].float() < out[j].float() })
	return out
}

func parentesis(r *big.Rat) string {
	if r.Sign() < 0 || !r.IsInt() {
		return "(" + textoRat(r) + ")"
	}
	return textoRat(r)
}

// raicesRacionales finds the rational roots of an integer polynomial by the rational root theorem
// and divides them out (Ruffini). It returns the roots (with repetition) and the remaining quotient.
func raicesRacionales(ctx context.Context, cs []*big.Rat) ([]*big.Rat, []*big.Rat) {
	cs = cs[:grado(cs)+1]
	var out []*big.Rat
	for grado(cs) > 0 && cs[0].Sign() == 0 {
		out = append(out, new(big.Rat))
		cs = cs[1:]
	}
	if grado(cs) <= 0 {
		return out, cs
	}
	a0 := new(big.Int).Abs(cs[0].Num())
	an := new(big.Int).Abs(cs[grado(cs)].Num())
	if a0.BitLen() > 64 || an.BitLen() > 64 {
		return out, cs
	}
	ps, err1 := Divisores(a0)
	qs, err2 := Divisores(an)
	if err1 != nil || err2 != nil || len(ps)*len(qs) > 20_000 {
		return out, cs
	}
	var cand []*big.Rat
	vistos := map[string]bool{}
	for _, p := range ps {
		for _, q := range qs {
			for _, s := range []int64{1, -1} {
				c := new(big.Rat).SetFrac(new(big.Int).Mul(p, big.NewInt(s)), q)
				if k := c.RatString(); !vistos[k] {
					vistos[k] = true
					cand = append(cand, c)
				}
			}
		}
	}
	sort.Slice(cand, func(i, j int) bool { return cand[i].Cmp(cand[j]) < 0 })
	for _, c := range cand {
		if ctx.Err() != nil {
			break
		}
		for grado(cs) > 0 && horner(cs, c).Sign() == 0 {
			out = append(out, new(big.Rat).Set(c))
			cs = dividirRuffini(cs, c)
		}
	}
	return out, cs
}

// durandKerner approximates the real roots of a polynomial (coefficients by degree).
func durandKerner(cs []*big.Rat) []float64 {
	n := grado(cs)
	if n < 1 {
		return nil
	}
	lead, _ := cs[n].Float64()
	a := make([]float64, n+1)
	for i := 0; i <= n; i++ {
		f, _ := cs[i].Float64()
		a[i] = f / lead
	}
	p := func(z complex128) complex128 {
		r := complex(0, 0)
		for i := n; i >= 0; i-- {
			r = r*z + complex(a[i], 0)
		}
		return r
	}
	zs := make([]complex128, n)
	semilla := complex(0.4, 0.9)
	zs[0] = 1
	for i := range zs {
		zs[i] = cmplx.Pow(semilla, complex(float64(i), 0))
	}
	for it := 0; it < 2000; it++ {
		cambio := 0.0
		for i := range zs {
			den := complex(1, 0)
			for j := range zs {
				if i != j {
					den *= zs[i] - zs[j]
				}
			}
			if den == 0 {
				den = complex(1e-12, 0)
			}
			d := p(zs[i]) / den
			zs[i] -= d
			cambio = math.Max(cambio, cmplx.Abs(d))
		}
		if cambio < 1e-15 {
			break
		}
	}
	var out []float64
	for _, z := range zs {
		if math.Abs(imag(z)) > 1e-7*math.Max(1, math.Abs(real(z))) {
			continue
		}
		x := real(z)
		// Newton polish on the real polynomial
		for k := 0; k < 50; k++ {
			v, dv := 0.0, 0.0
			for i := n; i >= 0; i-- {
				dv = dv*x + v
				v = v*x + a[i]
			}
			if dv == 0 {
				break
			}
			paso := v / dv
			x -= paso
			if math.Abs(paso) < 1e-16*math.Max(1, math.Abs(x)) {
				break
			}
		}
		repetida := false
		for _, y := range out {
			if math.Abs(x-y) < 1e-7*math.Max(1, math.Abs(x)) {
				repetida = true
			}
		}
		if !repetida {
			out = append(out, x)
		}
	}
	sort.Float64s(out)
	return out
}

// cerrar substitutes the roots back into the original equations and fills Valores/Tipo/Comprobado.
func (r *resolucion) cerrar(ecs []ecuacion, x string, raices []valor, tipo string) error {
	// distinct values, sorted
	var distintas []valor
	for _, v := range raices {
		rep := false
		for _, w := range distintas {
			if v.igual(w) {
				rep = true
			}
		}
		if !rep {
			distintas = append(distintas, v)
		}
	}
	sort.Slice(distintas, func(i, j int) bool { return distintas[i].float() < distintas[j].float() })
	if len(distintas) == 0 {
		r.sol.Tipo = "ninguna"
		r.sol.Comprobado = true
		return nil
	}
	todo := true
	for _, v := range distintas {
		ok, lineas := r.comprobar(ecs, map[string]valor{x: v})
		r.sol.Comprobacion = append(r.sol.Comprobacion, lineas...)
		todo = todo && ok
		r.sol.Valores[x] = append(r.sol.Valores[x], v.numero())
	}
	r.sol.Comprobado = todo
	switch {
	case tipo == "aproximada":
		r.sol.Tipo = "aproximada"
	case len(distintas) == 1:
		r.sol.Tipo = "unica"
	default:
		r.sol.Tipo = "varias"
	}
	return nil
}

// comprobar substitutes vals into every equation: exactly when the values are exact, else with a
// relative tolerance on the residual.
func (r *resolucion) comprobar(ecs []ecuacion, vals map[string]valor) (bool, []string) {
	todo := true
	var lineas []string
	for _, e := range ecs {
		izq := sustituir(e.l, vals)
		der := sustituir(e.r, vals)
		a, errA := evaluar(r.ctx, izq, nil)
		b, errB := evaluar(r.ctx, der, nil)
		ok := errA == nil && errB == nil
		if ok {
			exacto := !a.aprox && !b.aprox
			if exacto {
				ok = a.igual(b)
			} else {
				fa, fb := a.float(), b.float()
				escala := math.Max(1, math.Max(math.Abs(fa), math.Abs(fb)))
				ok = math.Abs(fa-fb) <= 1e-7*escala
			}
		}
		marca := "✔"
		if !ok {
			marca = "✘"
			todo = false
		}
		linea := izq.String() + " = " + der.String()
		if ok && errA == nil {
			if s := a.texto(); s != izq.String() && s != der.String() {
				linea = izq.String() + " = " + a.texto()
				if der.String() != a.texto() {
					linea += " = " + der.String()
				}
			}
		}
		lineas = append(lineas, linea+" "+marca)
		nd := r.n.Sub(nucleo.PasoComprobar, "Compruebo: %s %s", linea, marca)
		if ok {
			nd.Bien("")
		} else {
			nd.Mal("")
		}
	}
	return todo, lineas
}

// ---- linear systems ----

func todasLineales(lista []ecuacion) bool {
	for _, e := range lista {
		d := e.diferencia()
		if c, ok := d.den.constante(); !ok || c.Sign() == 0 {
			return false
		}
		if d.num.gradoTotal() > 1 {
			return false
		}
	}
	return true
}

func textoFila(fila []*big.Rat, vars []string) string {
	p := poli{}
	for i, v := range vars {
		if fila[i].Sign() != 0 {
			p[v] = fila[i]
		}
	}
	return textoPoli(p) + " = " + textoRat(fila[len(vars)])
}

func (r *resolucion) sistemaLineal(lista []ecuacion, vars []string) error {
	m := len(lista)
	nv := len(vars)
	A := make([][]*big.Rat, m)
	for i, e := range lista {
		d := e.diferencia()
		c, _ := d.den.constante()
		num := d.num.escalar(new(big.Rat).Inv(c))
		fila := make([]*big.Rat, nv+1)
		for j, v := range vars {
			fila[j] = new(big.Rat)
			if k, ok := num[v]; ok {
				fila[j].Set(k)
			}
		}
		fila[nv] = new(big.Rat)
		if k, ok := num[""]; ok {
			fila[nv].Neg(k)
		}
		A[i] = fila
	}
	var filas []string
	for i := range A {
		filas = append(filas, fmt.Sprintf("F%d: %s", i+1, textoFila(A[i], vars)))
	}
	r.paso("Escribo el sistema (Gauss-Jordan): %s", strings.Join(filas, "; "))
	fila := 0
	var pivotes []int
	for col := 0; col < nv && fila < m; col++ {
		if r.ctx.Err() != nil {
			return nucleo.ErrSinTiempo
		}
		p := -1
		for i := fila; i < m; i++ {
			if A[i][col].Sign() != 0 {
				p = i
				break
			}
		}
		if p < 0 {
			continue
		}
		if p != fila {
			A[p], A[fila] = A[fila], A[p]
			r.paso("Intercambio F%d y F%d", p+1, fila+1)
		}
		if piv := A[fila][col]; piv.Cmp(big.NewRat(1, 1)) != 0 {
			inv := new(big.Rat).Inv(piv)
			desc := "/ " + parentesis(piv)
			if piv.Cmp(big.NewRat(-1, 1)) == 0 {
				desc = "· (-1)"
			}
			for j := range A[fila] {
				A[fila][j].Mul(A[fila][j], inv)
			}
			r.paso("F%d → F%d %s: %s", fila+1, fila+1, desc, textoFila(A[fila], vars))
		}
		for i := 0; i < m; i++ {
			if i == fila || A[i][col].Sign() == 0 {
				continue
			}
			f := new(big.Rat).Set(A[i][col])
			for j := range A[i] {
				A[i][j].Sub(A[i][j], new(big.Rat).Mul(f, A[fila][j]))
			}
			op, abs := "-", new(big.Rat).Abs(f)
			if f.Sign() < 0 {
				op = "+"
			}
			coef := ""
			if abs.Cmp(big.NewRat(1, 1)) != 0 {
				coef = parentesis(abs) + "·"
			}
			r.paso("F%d → F%d %s %sF%d: %s", i+1, i+1, op, coef, fila+1, textoFila(A[i], vars))
		}
		pivotes = append(pivotes, col)
		fila++
	}
	for i := fila; i < m; i++ {
		if A[i][nv].Sign() != 0 {
			r.paso("F%d queda 0 = %s, que es imposible: el sistema no tiene solución", i+1, textoRat(A[i][nv]))
			r.sol.Tipo = "ninguna"
			r.sol.Comprobado = true
			return nil
		}
	}
	if len(pivotes) < nv {
		return r.infinitasLineal(lista, A[:len(pivotes)], pivotes, vars)
	}
	vals := map[string]valor{}
	for i, col := range pivotes {
		vals[vars[col]] = vRat(new(big.Rat).Set(A[i][nv]))
		r.sol.Valores[vars[col]] = []Numero{vals[vars[col]].numero()}
	}
	ok, lineas := r.comprobar(lista, vals)
	r.sol.Comprobacion = lineas
	r.sol.Comprobado = ok
	r.sol.Tipo = "unica"
	return nil
}

// infinitasLineal describes the solutions of an underdetermined system: pivot variables in terms of
// the free ones; it checks two particular solutions.
func (r *resolucion) infinitasLineal(lista []ecuacion, A [][]*big.Rat, pivotes []int, vars []string) error {
	nv := len(vars)
	esPivote := map[int]int{}
	for i, c := range pivotes {
		esPivote[c] = i
	}
	var libres []string
	for j, v := range vars {
		if _, ok := esPivote[j]; !ok {
			libres = append(libres, v)
		}
	}
	for i, col := range pivotes {
		p := poli{}
		for j := range vars {
			if _, ok := esPivote[j]; ok || A[i][j].Sign() == 0 {
				continue
			}
			p[vars[j]] = new(big.Rat).Neg(A[i][j])
		}
		if A[i][nv].Sign() != 0 {
			p[""] = new(big.Rat).Set(A[i][nv])
		}
		r.sol.Relacion = append(r.sol.Relacion, vars[col]+" = "+textoPoli(p))
	}
	r.paso("Hay más incógnitas que ecuaciones independientes: infinitas soluciones (%s libre%s): %s",
		strings.Join(libres, ", "), plural(len(libres), "", "s"), strings.Join(r.sol.Relacion, ", "))
	r.sol.Tipo = "infinitas"
	todo := true
	for _, prueba := range []int64{0, 1} {
		vals := map[string]valor{}
		for _, v := range libres {
			vals[v] = vInt(prueba)
		}
		for i, col := range pivotes {
			x := new(big.Rat).Set(A[i][nv])
			for j := range vars {
				if _, ok := esPivote[j]; !ok {
					x.Sub(x, new(big.Rat).Mul(A[i][j], big.NewRat(prueba, 1)))
				}
			}
			vals[vars[col]] = vRat(x)
		}
		ok, lineas := r.comprobar(lista, vals)
		r.sol.Comprobacion = append(r.sol.Comprobacion, lineas...)
		todo = todo && ok
	}
	r.sol.Comprobado = todo
	return nil
}

// ---- substitution (two equations, one of them linear) ----

func sustituirPoli(p poli, x string, q poli) poli {
	out := poli{}
	for m, c := range p {
		d := decodificar(m)
		k := d[x]
		delete(d, x)
		t := poli{codificar(d): new(big.Rat).Set(c)}
		for i := 0; i < k; i++ {
			t = t.mul(q)
		}
		out = out.sumar(t, 1)
	}
	return out
}

func (r *resolucion) sustitucion(lista []ecuacion, vars []string) error {
	li := -1
	for i, e := range lista {
		d := e.diferencia()
		if c, ok := d.den.constante(); ok && c.Sign() != 0 && d.num.gradoTotal() <= 1 {
			li = i
			break
		}
	}
	if li < 0 {
		return fmt.Errorf("%w: ninguna de las dos ecuaciones es lineal", nucleo.ErrNoSoportado)
	}
	lin, otra := lista[li], lista[1-li]
	d := lin.diferencia()
	c, _ := d.den.constante()
	num := d.num.escalar(new(big.Rat).Inv(c))
	// solve for the variable with a nonzero coefficient: x = -(rest)/a
	x, y := vars[0], vars[1]
	a, ok := num[x]
	if !ok || a.Sign() == 0 {
		x, y = y, x
		a = num[x]
	}
	if a == nil || a.Sign() == 0 {
		return fmt.Errorf("%w: la ecuación lineal no tiene incógnitas", nucleo.ErrNoSoportado)
	}
	resto := num.copia()
	delete(resto, x)
	expr := resto.escalar(new(big.Rat).Neg(new(big.Rat).Inv(a)))
	r.paso("Despejo %s en %s → %s = %s", x, lin, x, textoPoli(expr))
	d2 := otra.diferencia()
	if c2, ok := d2.den.constante(); !ok || c2.Sign() == 0 {
		return fmt.Errorf("%w: hay incógnitas en un denominador", nucleo.ErrNoSoportado)
	}
	sub := sustituirPoli(d2.num, x, expr)
	cs, err := sub.coeficientes(y)
	if err != nil {
		return err
	}
	cs = enteros(cs)
	r.paso("Sustituyo en %s → %s = 0", otra, textoPoli(polDesdeCoefs(cs, y)))
	raices, tipo, err := r.polinomio(cs, y)
	if err != nil {
		return err
	}
	if tipo == "ninguna" || len(raices) == 0 {
		r.sol.Tipo = "ninguna"
		r.sol.Comprobado = true
		return nil
	}
	if tipo == "infinitas" {
		r.sol.Tipo = "infinitas"
		r.sol.Relacion = []string{x + " = " + textoPoli(expr)}
		r.sol.Comprobado = true
		return nil
	}
	todo := true
	n := 0
	for _, vy := range raices {
		vx := evaluarPoli(expr, map[string]valor{y: vy})
		r.paso("Con %s = %s → %s = %s", y, vy.texto(), x, vx.texto())
		vals := map[string]valor{x: vx, y: vy}
		ok, lineas := r.comprobar(lista, vals)
		r.sol.Comprobacion = append(r.sol.Comprobacion, lineas...)
		todo = todo && ok
		r.sol.Valores[x] = append(r.sol.Valores[x], vx.numero())
		r.sol.Valores[y] = append(r.sol.Valores[y], vy.numero())
		n++
	}
	r.sol.Comprobado = todo
	switch {
	case tipo == "aproximada":
		r.sol.Tipo = "aproximada"
	case n == 1:
		r.sol.Tipo = "unica"
	default:
		r.sol.Tipo = "varias"
	}
	return nil
}

// ---- inequalities ----

var operadoresDesigualdad = []struct{ texto, op string }{
	{"<=", "≤"}, {">=", "≥"}, {"=<", "≤"}, {"=>", "≥"}, {"≤", "≤"}, {"≥", "≥"}, {"<", "<"}, {">", ">"},
}

func voltear(op string) string {
	return map[string]string{"<": ">", ">": "<", "≤": "≥", "≥": "≤"}[op]
}

func cumple(a, b valor, op string) bool {
	s := restar(a, b).signo()
	switch op {
	case "<":
		return s < 0
	case ">":
		return s > 0
	case "≤":
		return s <= 0
	case "≥":
		return s >= 0
	}
	return false
}

// Inecuacion solves a linear inequality in one unknown and returns the solution, e.g. "x > 3". When
// it divides by a negative number the sign is turned around. The answer is checked on points around
// the boundary.
func Inecuacion(ctx context.Context, s string, n *nucleo.Nodo) (string, error) {
	op, pos, largo := "", -1, 0
	for _, o := range operadoresDesigualdad {
		if i := strings.Index(s, o.texto); i >= 0 && (pos < 0 || i < pos || i == pos && len(o.texto) > largo) {
			op, pos, largo = o.op, i, len(o.texto)
		}
	}
	if pos < 0 {
		return "", fmt.Errorf("%w: no veo «<» ni «>»", nucleo.ErrNoEntiendo)
	}
	ls, rs := s[:pos], s[pos+largo:]
	for _, o := range operadoresDesigualdad {
		if strings.Contains(rs, o.texto) {
			return "", fmt.Errorf("%w: solo sé inecuaciones con un signo", nucleo.ErrNoSoportado)
		}
	}
	l, err := AnalizarExpr(ls)
	if err != nil {
		return "", err
	}
	rr, err := AnalizarExpr(rs)
	if err != nil {
		return "", err
	}
	vars := variables(Op{Op: '+', A: l, B: rr})
	if len(vars) != 1 {
		return "", fmt.Errorf("%w: solo sé inecuaciones con una incógnita", nucleo.ErrNoSoportado)
	}
	x := vars[0]
	fl, err := aFraccion(ctx, l)
	if err != nil {
		return "", err
	}
	fr, err := aFraccion(ctx, rr)
	if err != nil {
		return "", err
	}
	dl, okl := fl.den.constante()
	dr, okr := fr.den.constante()
	if !okl || !okr || fl.num.gradoTotal() > 1 || fr.num.gradoTotal() > 1 {
		return "", fmt.Errorf("%w: solo sé inecuaciones lineales", nucleo.ErrNoSoportado)
	}
	res := &resolucion{ctx: ctx, n: n}
	res.paso("Inecuación: %s %s %s", l, op, rr)
	a1, b1 := lineal1(fl.num.escalar(new(big.Rat).Inv(dl)), x)
	a2, b2 := lineal1(fr.num.escalar(new(big.Rat).Inv(dr)), x)
	l2 := big.NewInt(1)
	for _, c := range []*big.Rat{a1, b1, a2, b2} {
		l2 = mcmInt(l2, c.Denom())
	}
	if l2.Cmp(big.NewInt(1)) != 0 {
		m := new(big.Rat).SetInt(l2)
		for _, c := range []*big.Rat{a1, b1, a2, b2} {
			c.Mul(c, m)
		}
		res.paso("Multiplico los dos lados por %s (positivo: el signo no cambia) → %s %s %s", l2, textoLineal(a1, b1, x), op, textoLineal(a2, b2, x))
	}
	if a2.Sign() != 0 {
		verbo := "Resto"
		if a2.Sign() < 0 {
			verbo = "Sumo"
		}
		t := textoTermino(a2, x)
		a1.Sub(a1, a2)
		a2.SetInt64(0)
		res.paso("%s %s a ambos lados → %s %s %s", verbo, t, textoLineal(a1, b1, x), op, textoLineal(a2, b2, x))
	}
	if b1.Sign() != 0 {
		verbo := "Resto"
		if b1.Sign() < 0 {
			verbo = "Sumo"
		}
		t := textoTermino(b1, "")
		b2.Sub(b2, b1)
		b1.SetInt64(0)
		res.paso("%s %s a ambos lados → %s %s %s", verbo, t, textoLineal(a1, b1, x), op, textoRat(b2))
	}
	original := func(v valor) (bool, error) {
		vals := map[string]valor{x: v}
		a, err := evaluar(ctx, l, vals)
		if err != nil {
			return false, err
		}
		b, err := evaluar(ctx, rr, vals)
		if err != nil {
			return false, err
		}
		return cumple(a, b, op), nil
	}
	if a1.Sign() == 0 {
		siempre := cumple(vInt(0), vRat(b2), op)
		res.paso("Queda 0 %s %s", op, textoRat(b2))
		// check with a few points
		for _, p := range []int64{-7, 0, 5} {
			ok, err := original(vInt(p))
			if err != nil || ok != siempre {
				return "", errors.New("no he podido comprobar la solución")
			}
		}
		if siempre {
			return "se cumple para cualquier " + x, nil
		}
		return "no se cumple para ningún " + x, nil
	}
	borde := new(big.Rat).Quo(b2, a1)
	final := op
	switch {
	case a1.Cmp(big.NewRat(1, 1)) == 0:
	case a1.Sign() < 0:
		final = voltear(op)
		res.paso("Divido entre %s y doy la vuelta al signo (al dividir entre un negativo cambia) → %s %s %s", textoRat(a1), x, final, textoRat(borde))
	default:
		res.paso("Divido entre %s → %s %s %s", textoRat(a1), x, final, textoRat(borde))
	}
	// check: points on both sides and the boundary itself
	medio := big.NewRat(1, 2)
	for _, p := range []*big.Rat{new(big.Rat).Sub(borde, big.NewRat(1, 1)), new(big.Rat).Sub(borde, medio), borde,
		new(big.Rat).Add(borde, medio), new(big.Rat).Add(borde, big.NewRat(1, 1))} {
		ok, err := original(vRat(p))
		if err != nil || ok != cumple(vRat(p), vRat(borde), final) {
			n.Sub(nucleo.PasoError, "La comprobación con %s = %s ha fallado", x, textoRat(p))
			return "", errors.New("no he podido comprobar la solución")
		}
	}
	n.Sub(nucleo.PasoComprobar, "Compruebo con valores a los dos lados de %s y en el borde: todo cuadra", textoRat(borde)).Bien("")
	return x + " " + final + " " + textoRat(borde), nil
}
