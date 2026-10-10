package mates

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

const (
	maxPasosVisibles = 30
	maxReducciones   = 20_000
)

// ErrIncognita: the expression has an unknown, so it cannot be calculated.
var ErrIncognita = errors.New("la expresión tiene una incógnita")

func hoja(e Expr) (valor, bool) {
	switch x := e.(type) {
	case Num:
		return vRat(x.V), true
	case Val:
		return x.v, true
	}
	return valor{}, false
}

func aplicarOp(op byte, a, b valor) (valor, error) {
	switch op {
	case '+':
		return sumar(a, b), nil
	case '-':
		return restar(a, b), nil
	case '*':
		return multiplicar(a, b), nil
	case '/':
		return dividir(a, b)
	case '^':
		return potencia(a, b)
	case '%':
		return modulo(a, b)
	}
	return valor{}, fmt.Errorf("operación desconocida %q", op)
}

func aplicarFunc(nombre string, a valor) (valor, error) {
	switch nombre {
	case "√":
		return raiz(a)
	case "abs":
		if a.signo() < 0 {
			return neg(a), nil
		}
		return a, nil
	case "grados":
		return vAprox(a.float() * math.Pi / 180), nil
	case "sen":
		return vAprox(limpiarCero(math.Sin(a.float()))), nil
	case "cos":
		return vAprox(limpiarCero(math.Cos(a.float()))), nil
	case "tan":
		c := math.Cos(a.float())
		if math.Abs(c) < 1e-15 {
			return valor{}, errors.New("la tangente no existe ahí")
		}
		return vAprox(limpiarCero(math.Tan(a.float()))), nil
	case "log", "ln":
		if a.signo() <= 0 {
			return valor{}, errors.New("el logaritmo solo existe para números positivos")
		}
		base := 10.0
		if nombre == "ln" {
			base = math.E
		}
		f := math.Log(a.float()) / math.Log(base)
		// exact when the argument is an exact power of 10 (log) or 1
		if r := math.Round(f); math.Abs(f-r) < 1e-12 && a.racional() {
			if nombre == "log" && r >= -300 && r <= 300 {
				p, err := potenciaRat(big.NewRat(10, 1), int64(r))
				if err == nil && p.a.Cmp(a.a) == 0 {
					return vInt(int64(r)), nil
				}
			}
			if r == 0 && a.a.Cmp(big.NewRat(1, 1)) == 0 {
				return vInt(0), nil
			}
		}
		return vAprox(f), nil
	case "exp":
		if a.cero() {
			return vInt(1), nil
		}
		return vAprox(math.Exp(a.float())), nil
	}
	return valor{}, fmt.Errorf("no conozco la función %s", nombre)
}

func limpiarCero(f float64) float64 {
	if math.Abs(f) < 1e-15 {
		return 0
	}
	return f
}

// paso performs one reduction: the leftmost innermost operation whose operands are values.
func paso(e Expr) (Expr, error) {
	switch x := e.(type) {
	case Num, Val:
		return e, nil
	case Var:
		return nil, fmt.Errorf("%w %s", ErrIncognita, x.Nombre)
	case Op:
		a, okA := hoja(x.A)
		if !okA {
			na, err := paso(x.A)
			return Op{Op: x.Op, A: na, B: x.B}, err
		}
		b, okB := hoja(x.B)
		if !okB {
			nb, err := paso(x.B)
			return Op{Op: x.Op, A: x.A, B: nb}, err
		}
		v, err := aplicarOp(x.Op, a, b)
		return Val{v: v}, err
	case Neg:
		a, ok := hoja(x.A)
		if !ok {
			na, err := paso(x.A)
			return Neg{A: na}, err
		}
		return Val{v: neg(a)}, nil
	case Fact:
		a, ok := hoja(x.A)
		if !ok {
			na, err := paso(x.A)
			return Fact{A: na}, err
		}
		v, err := factorial(a)
		return Val{v: v}, err
	case Func:
		a, ok := hoja(x.A)
		if !ok {
			na, err := paso(x.A)
			return Func{Nombre: x.Nombre, A: na}, err
		}
		v, err := aplicarFunc(x.Nombre, a)
		return Val{v: v}, err
	}
	return nil, fmt.Errorf("expresión desconocida %T", e)
}

// reducir evaluates e step by step and returns the value and the visible steps (each one the whole
// expression after one reduction; steps that do not change the text are skipped).
func reducir(ctx context.Context, e Expr) (valor, []string, error) {
	pasos := []string{e.String()}
	for k := 0; ; k++ {
		if v, ok := hoja(e); ok {
			return v, pasos, nil
		}
		if k > maxReducciones {
			return valor{}, pasos, errors.New("la operación es demasiado larga")
		}
		if k%64 == 0 && ctx.Err() != nil {
			return valor{}, pasos, nucleo.ErrSinTiempo
		}
		ne, err := paso(e)
		if err != nil {
			return valor{}, pasos, err
		}
		e = ne
		if s := e.String(); s != pasos[len(pasos)-1] {
			pasos = append(pasos, s)
		}
	}
}

// resumirPasos keeps at most maxPasosVisibles steps: the first ones, a note, and the last ones.
func resumirPasos(p []string) []string {
	if len(p) <= maxPasosVisibles {
		return p
	}
	ini, fin := 18, 10
	out := append([]string{}, p[:ini]...)
	out = append(out, "… ("+strconv.Itoa(len(p)-ini-fin)+" pasos más) …")
	return append(out, p[len(p)-fin:]...)
}

// evaluar computes e directly with the given values for its unknowns (no steps).
func evaluar(ctx context.Context, e Expr, vals map[string]valor) (valor, error) {
	if ctx.Err() != nil {
		return valor{}, nucleo.ErrSinTiempo
	}
	switch x := e.(type) {
	case Num:
		return vRat(x.V), nil
	case Val:
		return x.v, nil
	case Var:
		if v, ok := vals[x.Nombre]; ok {
			return v, nil
		}
		return valor{}, fmt.Errorf("%w %s", ErrIncognita, x.Nombre)
	case Op:
		a, err := evaluar(ctx, x.A, vals)
		if err != nil {
			return valor{}, err
		}
		b, err := evaluar(ctx, x.B, vals)
		if err != nil {
			return valor{}, err
		}
		return aplicarOp(x.Op, a, b)
	case Neg:
		a, err := evaluar(ctx, x.A, vals)
		return neg(a), err
	case Fact:
		a, err := evaluar(ctx, x.A, vals)
		if err != nil {
			return valor{}, err
		}
		return factorial(a)
	case Func:
		a, err := evaluar(ctx, x.A, vals)
		if err != nil {
			return valor{}, err
		}
		return aplicarFunc(x.Nombre, a)
	}
	return valor{}, fmt.Errorf("expresión desconocida %T", e)
}

// Evaluar computes e exactly (big.Rat, exact square roots; trigonometry and logarithms are float64
// and the result is then approximate). The reduction steps go to n: (2+3)·4 → 5·4 → 20.
func Evaluar(ctx context.Context, e Expr, n *nucleo.Nodo) (Numero, error) {
	v, pasos, err := reducir(ctx, e)
	if err != nil {
		if n != nil {
			n.Sub(nucleo.PasoError, "No puedo calcular %s: %s", e, mensaje(err))
		}
		return Numero{}, err
	}
	if n != nil {
		sub := n.Sub(nucleo.PasoIntento, "Calculo %s", e)
		for _, p := range resumirPasos(pasos)[1:] {
			sub.Detalle("→ %s", p)
		}
		sub.Bien("%s = %s", e, v.numero())
	}
	return v.numero(), nil
}

// Calcular parses and evaluates s. Leading question words ("cuánto es", "calcula") are ignored.
func Calcular(ctx context.Context, s string, n *nucleo.Nodo) (Numero, error) {
	e, err := AnalizarExpr(limpiarCalculo(s))
	if err != nil {
		return Numero{}, err
	}
	return Evaluar(ctx, e, n)
}

// mensaje is the Spanish text of an error, without the nucleo prefix.
func mensaje(err error) string {
	s := err.Error()
	for _, pre := range []string{"nucleo: no entiendo el enunciado: ", "nucleo: "} {
		s = strings.TrimPrefix(s, pre)
	}
	return s
}

// ---- independent check: the same expression in big.Float ----

const precComprobar = 256

// evaluarFloat computes e in big.Float by a different path (no reductions, no exact radicals).
func evaluarFloat(e Expr, vals map[string]float64) (*big.Float, error) {
	nuevo := func() *big.Float { return new(big.Float).SetPrec(precComprobar) }
	switch x := e.(type) {
	case Num:
		return nuevo().SetRat(x.V), nil
	case Val:
		if x.v.racional() {
			return nuevo().SetRat(x.v.a), nil
		}
		return nuevo().SetFloat64(x.v.float()), nil
	case Var:
		f, ok := vals[x.Nombre]
		if !ok {
			return nil, ErrIncognita
		}
		return nuevo().SetFloat64(f), nil
	case Neg:
		a, err := evaluarFloat(x.A, vals)
		if err != nil {
			return nil, err
		}
		return a.Neg(a), nil
	case Fact:
		a, err := evaluarFloat(x.A, vals)
		if err != nil {
			return nil, err
		}
		i, acc := a.Int64()
		if acc != big.Exact || i < 0 || i > maxFactorial {
			return nil, errors.New("factorial fuera de rango")
		}
		return nuevo().SetInt(new(big.Int).MulRange(1, max(i, 1))), nil
	case Func:
		a, err := evaluarFloat(x.A, vals)
		if err != nil {
			return nil, err
		}
		if x.Nombre == "√" {
			if a.Sign() < 0 {
				return nil, errors.New("raíz de negativo")
			}
			return nuevo().Sqrt(a), nil
		}
		f, _ := a.Float64()
		v, err := aplicarFunc(x.Nombre, vAprox(f))
		if err != nil {
			return nil, err
		}
		return nuevo().SetFloat64(v.float()), nil
	case Op:
		a, err := evaluarFloat(x.A, vals)
		if err != nil {
			return nil, err
		}
		b, err := evaluarFloat(x.B, vals)
		if err != nil {
			return nil, err
		}
		r := nuevo()
		switch x.Op {
		case '+':
			return r.Add(a, b), nil
		case '-':
			return r.Sub(a, b), nil
		case '*':
			return r.Mul(a, b), nil
		case '/':
			if b.Sign() == 0 {
				return nil, ErrDivisionCero
			}
			return r.Quo(a, b), nil
		case '%':
			ai, _ := a.Int(nil)
			bi, _ := b.Int(nil)
			if bi.Sign() == 0 {
				return nil, ErrDivisionCero
			}
			return r.SetInt(new(big.Int).Mod(ai, new(big.Int).Abs(bi))), nil
		case '^':
			k, acc := b.Int64()
			if acc == big.Exact && k >= -maxBitsPotencia && k <= maxBitsPotencia {
				return potenciaFloat(a, k), nil
			}
			af, _ := a.Float64()
			bf, _ := b.Float64()
			if af < 0 {
				// odd roots of negatives
				return r.SetFloat64(-math.Pow(-af, bf)), nil
			}
			return r.SetFloat64(math.Pow(af, bf)), nil
		}
	}
	return nil, fmt.Errorf("expresión desconocida %T", e)
}

func potenciaFloat(b *big.Float, k int64) *big.Float {
	neg := k < 0
	if neg {
		k = -k
	}
	res := new(big.Float).SetPrec(precComprobar).SetInt64(1)
	base := new(big.Float).SetPrec(precComprobar).Set(b)
	for k > 0 {
		if k&1 == 1 {
			res.Mul(res, base)
		}
		base.Mul(base, base)
		k >>= 1
	}
	if neg {
		return new(big.Float).SetPrec(precComprobar).Quo(new(big.Float).SetInt64(1), res)
	}
	return res
}

// comprobarValor recomputes e in big.Float and compares with v (relative error ≤ 1e-20 for exact
// rationals, 1e-9 otherwise).
func comprobarValor(e Expr, v valor, vals map[string]float64) bool {
	f, err := evaluarFloat(e, vals)
	if err != nil {
		return false
	}
	var g *big.Float
	tol := 1e-9
	if v.racional() {
		g = new(big.Float).SetPrec(precComprobar).SetRat(v.a)
		if exactaExpr(e) {
			tol = 1e-20
		}
	} else {
		g = new(big.Float).SetPrec(precComprobar).SetFloat64(v.float())
	}
	d := new(big.Float).Sub(f, g)
	d.Abs(d)
	escala := new(big.Float).Abs(f)
	if escala.Cmp(big.NewFloat(1)) < 0 {
		escala = big.NewFloat(1)
	}
	lim := new(big.Float).Mul(escala, big.NewFloat(tol))
	return d.Cmp(lim) <= 0
}

// exactaExpr reports whether big.Float evaluates e with full precision (no float64 functions and no
// fractional exponents), so the check can be strict.
func exactaExpr(e Expr) bool {
	switch x := e.(type) {
	case Val:
		return !x.v.aprox
	case Op:
		if x.Op == '^' {
			if n, ok := x.B.(Num); !ok || !n.V.IsInt() {
				return false
			}
		}
		return exactaExpr(x.A) && exactaExpr(x.B)
	case Neg:
		return exactaExpr(x.A)
	case Fact:
		return exactaExpr(x.A)
	case Func:
		return x.Nombre == "√" && exactaExpr(x.A)
	}
	return true
}
