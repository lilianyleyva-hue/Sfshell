// Package logica implements propositional logic (Tseitin, DPLL, named rules, resolution, truth
// tables), categorical syllogisms on Venn regions, facts and rules with forward chaining, and
// knights and knaves (§4.10). It imports only the stdlib and nucleo.
package logica

import (
	"fmt"
	"strings"
)

// Formula is a propositional formula built from Atomo, No, Y, O, Implica and Equiv.
type Formula interface{ String() string }

// Atomo is the propositional variable number N (0-based; Atomos.Frases[N] names it).
type Atomo struct{ N int }

// No is ¬F.
type No struct{ F Formula }

// Y is A ∧ B.
type Y struct{ A, B Formula }

// O is A ∨ B.
type O struct{ A, B Formula }

// Implica is A → B.
type Implica struct{ A, B Formula }

// Equiv is A ↔ B.
type Equiv struct{ A, B Formula }

var letras = []string{"p", "q", "r", "s", "t", "u", "v", "w"}

// NombreAtomo is the default symbolic name of atom n: p, q, r, s, t, u, v, w, then x8, x9…
func NombreAtomo(n int) string {
	if n >= 0 && n < len(letras) {
		return letras[n]
	}
	return fmt.Sprintf("x%d", n)
}

func (a Atomo) String() string   { return NombreAtomo(a.N) }
func (f No) String() string      { return "¬" + envolver(f.F) }
func (f Y) String() string       { return envolver(f.A) + " ∧ " + envolver(f.B) }
func (f O) String() string       { return envolver(f.A) + " ∨ " + envolver(f.B) }
func (f Implica) String() string { return envolver(f.A) + " → " + envolver(f.B) }
func (f Equiv) String() string   { return envolver(f.A) + " ↔ " + envolver(f.B) }

func envolver(f Formula) string {
	switch f.(type) {
	case Atomo, No:
		return f.String()
	case nil:
		return "?"
	}
	return "(" + f.String() + ")"
}

// Mostrar writes f using the atom phrases of a (Spanish words for the connectives):
// "si «llueve», entonces «el suelo se moja»". With a == nil it is f.String().
func Mostrar(f Formula, a *Atomos) string {
	if a == nil {
		return fmt.Sprint(f)
	}
	return mostrar(f, a, true)
}

func mostrar(f Formula, a *Atomos, raiz bool) string {
	par := func(s string) string {
		if raiz {
			return s
		}
		return "(" + s + ")"
	}
	switch g := f.(type) {
	case Atomo:
		if g.N >= 0 && g.N < len(a.Frases) {
			return "«" + a.Frases[g.N] + "»"
		}
		return g.String()
	case No:
		if at, ok := g.F.(Atomo); ok {
			return "no " + mostrar(at, a, false)
		}
		return "no es cierto que " + mostrar(g.F, a, false)
	case Y:
		return par(mostrar(g.A, a, false) + " y " + mostrar(g.B, a, false))
	case O:
		return par(mostrar(g.A, a, false) + " o " + mostrar(g.B, a, false))
	case Implica:
		return par("si " + mostrar(g.A, a, false) + ", entonces " + mostrar(g.B, a, false))
	case Equiv:
		return par(mostrar(g.A, a, false) + " si y solo si " + mostrar(g.B, a, false))
	}
	return "?"
}

// Evaluar computes f under the assignment v (v[i] is atom i; missing atoms are false).
func Evaluar(f Formula, v []bool) bool {
	switch g := f.(type) {
	case Atomo:
		return g.N >= 0 && g.N < len(v) && v[g.N]
	case No:
		return !Evaluar(g.F, v)
	case Y:
		return Evaluar(g.A, v) && Evaluar(g.B, v)
	case O:
		return Evaluar(g.A, v) || Evaluar(g.B, v)
	case Implica:
		return !Evaluar(g.A, v) || Evaluar(g.B, v)
	case Equiv:
		return Evaluar(g.A, v) == Evaluar(g.B, v)
	}
	return false
}

// Igual is structural equality.
func Igual(a, b Formula) bool {
	switch x := a.(type) {
	case Atomo:
		y, ok := b.(Atomo)
		return ok && x.N == y.N
	case No:
		y, ok := b.(No)
		return ok && Igual(x.F, y.F)
	case Y:
		y, ok := b.(Y)
		return ok && Igual(x.A, y.A) && Igual(x.B, y.B)
	case O:
		y, ok := b.(O)
		return ok && Igual(x.A, y.A) && Igual(x.B, y.B)
	case Implica:
		y, ok := b.(Implica)
		return ok && Igual(x.A, y.A) && Igual(x.B, y.B)
	case Equiv:
		y, ok := b.(Equiv)
		return ok && Igual(x.A, y.A) && Igual(x.B, y.B)
	case nil:
		return b == nil
	}
	return false
}

// maxAtomo returns the largest atom index in fs, or -1.
func maxAtomo(fs ...Formula) int {
	m := -1
	var rec func(f Formula)
	rec = func(f Formula) {
		switch g := f.(type) {
		case Atomo:
			if g.N > m {
				m = g.N
			}
		case No:
			rec(g.F)
		case Y:
			rec(g.A)
			rec(g.B)
		case O:
			rec(g.A)
			rec(g.B)
		case Implica:
			rec(g.A)
			rec(g.B)
		case Equiv:
			rec(g.A)
			rec(g.B)
		}
	}
	for _, f := range fs {
		rec(f)
	}
	return m
}

// atomosUsados lists the distinct atom indices in fs in increasing order.
func atomosUsados(fs ...Formula) []int {
	vistos := map[int]bool{}
	var rec func(f Formula)
	rec = func(f Formula) {
		switch g := f.(type) {
		case Atomo:
			vistos[g.N] = true
		case No:
			rec(g.F)
		case Y:
			rec(g.A)
			rec(g.B)
		case O:
			rec(g.A)
			rec(g.B)
		case Implica:
			rec(g.A)
			rec(g.B)
		case Equiv:
			rec(g.A)
			rec(g.B)
		}
	}
	for _, f := range fs {
		rec(f)
	}
	var out []int
	for i := 0; i <= maxAtomo(fs...); i++ {
		if vistos[i] {
			out = append(out, i)
		}
	}
	return out
}

func negar(f Formula) Formula {
	if n, ok := f.(No); ok {
		return n.F
	}
	return No{f}
}

func conjuncion(fs []Formula) Formula {
	if len(fs) == 0 {
		return nil
	}
	r := fs[0]
	for _, f := range fs[1:] {
		r = Y{r, f}
	}
	return r
}

// literalTexto describes atom i with value v in words: "llueve" / "NO llueve".
func literalTexto(a *Atomos, i int, v bool) string {
	nombre := NombreAtomo(i)
	if a != nil && i < len(a.Frases) {
		nombre = a.Frases[i]
	}
	if v {
		return "«" + nombre + "» es verdad"
	}
	return "«" + nombre + "» es falso"
}

// DescribirModelo puts an assignment in words for the atoms used: "«llueve» es falso y «el suelo
// se moja» es verdad".
func DescribirModelo(modelo []bool, a *Atomos, usados []int) string {
	var partes []string
	for _, i := range usados {
		v := i < len(modelo) && modelo[i]
		partes = append(partes, literalTexto(a, i, v))
	}
	return unirY(partes)
}

func unirY(partes []string) string {
	switch len(partes) {
	case 0:
		return ""
	case 1:
		return partes[0]
	}
	return strings.Join(partes[:len(partes)-1], ", ") + " y " + partes[len(partes)-1]
}
