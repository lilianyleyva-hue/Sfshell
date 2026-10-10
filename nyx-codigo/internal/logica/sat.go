package logica

import (
	"context"
	"errors"
	"fmt"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Limits of the SAT engine (§4.10).
const (
	MaxDecisiones = 1_000_000
	TiempoSAT     = 2 * time.Second
	maxAtomosTabla = 12
)

// ErrNoCoinciden is returned when the self-check (truth table vs DPLL) disagrees. It should never
// happen; it exists so that a wrong answer is never shown.
var ErrNoCoinciden = errors.New("logica: la tabla de verdad y DPLL no coinciden")

// Tseitin converts the conjunction of fs into an equisatisfiable CNF. Atom i is variable i+1;
// fresh variables follow. Top-level conjunctions are split, and negations of atoms do not create
// fresh variables.
func Tseitin(fs []Formula, nAtomos int) (cnf [][]int, nVars int) {
	nVars = nAtomos
	if m := maxAtomo(fs...) + 1; m > nVars {
		nVars = m
	}
	var lit func(f Formula) int
	lit = func(f Formula) int {
		switch g := f.(type) {
		case Atomo:
			return g.N + 1
		case No:
			return -lit(g.F)
		case Y:
			a, b := lit(g.A), lit(g.B)
			nVars++
			x := nVars
			cnf = append(cnf, []int{-x, a}, []int{-x, b}, []int{x, -a, -b})
			return x
		case O:
			a, b := lit(g.A), lit(g.B)
			nVars++
			x := nVars
			cnf = append(cnf, []int{-x, a, b}, []int{x, -a}, []int{x, -b})
			return x
		case Implica:
			a, b := lit(g.A), lit(g.B)
			nVars++
			x := nVars
			cnf = append(cnf, []int{-x, -a, b}, []int{x, a}, []int{x, -b})
			return x
		case Equiv:
			a, b := lit(g.A), lit(g.B)
			nVars++
			x := nVars
			cnf = append(cnf, []int{-x, -a, b}, []int{-x, a, -b}, []int{x, a, b}, []int{x, -a, -b})
			return x
		}
		// unknown node: a fresh unconstrained variable keeps the CNF well formed
		nVars++
		return nVars
	}
	var afirmar func(f Formula)
	afirmar = func(f Formula) {
		switch g := f.(type) {
		case nil:
			return
		case Y:
			afirmar(g.A)
			afirmar(g.B)
		case No:
			if o, ok := g.F.(O); ok { // ¬(a ∨ b) = ¬a ∧ ¬b
				afirmar(No{o.A})
				afirmar(No{o.B})
				return
			}
			cnf = append(cnf, []int{lit(g)})
		case O:
			cnf = append(cnf, []int{lit(g.A), lit(g.B)})
		case Implica:
			cnf = append(cnf, []int{-lit(g.A), lit(g.B)})
		default:
			cnf = append(cnf, []int{lit(g)})
		}
	}
	for _, f := range fs {
		afirmar(f)
	}
	return cnf, nVars
}

type decision struct {
	lit     int
	volteada bool
}

type dpll struct {
	n        int
	clausulas [][]int
	valor    []int8 // 0 unassigned, 1 true, -1 false
	vigias   [][]int
	traza    []int
	cabeza   int
	inicios  []int
	decs     []decision
	actividad []float64
	inc      float64
}

func idxLit(l int) int {
	if l < 0 {
		return 2*(-l) + 1
	}
	return 2 * l
}

func (d *dpll) val(l int) int8 {
	if l < 0 {
		return -d.valor[-l]
	}
	return d.valor[l]
}

func (d *dpll) poner(l int) {
	if l < 0 {
		d.valor[-l] = -1
	} else {
		d.valor[l] = 1
	}
	d.traza = append(d.traza, l)
}

// propagar runs unit propagation over the two watched literals; it returns the index of a
// conflicting clause, or -1.
func (d *dpll) propagar() int {
	for d.cabeza < len(d.traza) {
		l := d.traza[d.cabeza]
		d.cabeza++
		falso := -l
		ws := d.vigias[idxLit(falso)]
		i, j := 0, 0
		for i < len(ws) {
			ci := ws[i]
			i++
			c := d.clausulas[ci]
			if c[0] == falso {
				c[0], c[1] = c[1], c[0]
			}
			if d.val(c[0]) == 1 {
				ws[j] = ci
				j++
				continue
			}
			movida := false
			for k := 2; k < len(c); k++ {
				if d.val(c[k]) != -1 {
					c[1], c[k] = c[k], c[1]
					d.vigias[idxLit(c[1])] = append(d.vigias[idxLit(c[1])], ci)
					movida = true
					break
				}
			}
			if movida {
				continue
			}
			ws[j] = ci
			j++
			if d.val(c[0]) == -1 {
				for i < len(ws) {
					ws[j] = ws[i]
					i++
					j++
				}
				d.vigias[idxLit(falso)] = ws[:j]
				return ci
			}
			d.poner(c[0])
		}
		d.vigias[idxLit(falso)] = ws[:j]
	}
	return -1
}

func (d *dpll) deshacer(hasta int) {
	for len(d.traza) > hasta {
		l := d.traza[len(d.traza)-1]
		d.traza = d.traza[:len(d.traza)-1]
		if l < 0 {
			l = -l
		}
		d.valor[l] = 0
	}
	if d.cabeza > len(d.traza) {
		d.cabeza = len(d.traza)
	}
}

// DPLL decides the CNF (variables 1..nVars) with unit propagation (two watched literals), pure
// literals at the root, activity scores bumped on conflict (VSIDS-lite) and chronological
// backtracking. modelo[i] is the value of variable i+1. maxDecisiones ≤ 0 means 10⁶. It gives up
// after 2 s or when ctx is done, returning an error wrapping nucleo.ErrSinTiempo.
func DPLL(ctx context.Context, cnf [][]int, nVars, maxDecisiones int) (modelo []bool, sat bool, err error) {
	if maxDecisiones <= 0 {
		maxDecisiones = MaxDecisiones
	}
	if ctx == nil {
		ctx = context.Background()
	}
	limite := time.Now().Add(TiempoSAT)
	d := &dpll{n: nVars, valor: make([]int8, nVars+1), vigias: make([][]int, 2*nVars+2), actividad: make([]float64, nVars+1), inc: 1}
	var unidades []int
	for _, c0 := range cnf {
		vistos := map[int]bool{}
		var c []int
		tauto := false
		for _, l := range c0 {
			if l == 0 || l > nVars || -l > nVars {
				return nil, false, fmt.Errorf("logica: literal %d fuera de rango (hay %d variables)", l, nVars)
			}
			if vistos[-l] {
				tauto = true
				break
			}
			if !vistos[l] {
				vistos[l] = true
				c = append(c, l)
			}
		}
		if tauto {
			continue
		}
		switch len(c) {
		case 0:
			return nil, false, nil
		case 1:
			unidades = append(unidades, c[0])
		default:
			ci := len(d.clausulas)
			d.clausulas = append(d.clausulas, c)
			d.vigias[idxLit(c[0])] = append(d.vigias[idxLit(c[0])], ci)
			d.vigias[idxLit(c[1])] = append(d.vigias[idxLit(c[1])], ci)
		}
	}
	for _, u := range unidades {
		switch d.val(u) {
		case -1:
			return nil, false, nil
		case 0:
			d.poner(u)
		}
	}
	if d.propagar() >= 0 {
		return nil, false, nil
	}
	// pure literals at the root, repeated while new ones appear
	for {
		pos := make([]bool, nVars+1)
		neg := make([]bool, nVars+1)
		for _, c := range d.clausulas {
			satisfecha := false
			for _, l := range c {
				if d.val(l) == 1 {
					satisfecha = true
					break
				}
			}
			if satisfecha {
				continue
			}
			for _, l := range c {
				if d.val(l) != 0 {
					continue
				}
				if l > 0 {
					pos[l] = true
				} else {
					neg[-l] = true
				}
			}
		}
		nuevos := 0
		for v := 1; v <= nVars; v++ {
			if d.valor[v] != 0 || pos[v] == neg[v] {
				continue
			}
			if pos[v] {
				d.poner(v)
			} else {
				d.poner(-v)
			}
			nuevos++
		}
		if nuevos == 0 {
			break
		}
		if d.propagar() >= 0 { // cannot happen with pure literals, but stay safe
			return nil, false, nil
		}
	}
	decisiones := 0
	for {
		// pick the unassigned variable with the highest activity
		mejor := 0
		for v := 1; v <= nVars; v++ {
			if d.valor[v] == 0 && (mejor == 0 || d.actividad[v] > d.actividad[mejor]) {
				mejor = v
			}
		}
		if mejor == 0 {
			break // all assigned, no conflict
		}
		decisiones++
		if decisiones > maxDecisiones {
			return nil, false, fmt.Errorf("logica: más de %d decisiones: %w", maxDecisiones, nucleo.ErrSinTiempo)
		}
		if decisiones&1023 == 0 {
			if ctx.Err() != nil || time.Now().After(limite) {
				return nil, false, fmt.Errorf("logica: DPLL sin tiempo: %w", nucleo.ErrSinTiempo)
			}
		}
		d.inicios = append(d.inicios, len(d.traza))
		d.decs = append(d.decs, decision{lit: -mejor})
		d.poner(-mejor)
		for {
			conflicto := d.propagar()
			if conflicto < 0 {
				break
			}
			for _, l := range d.clausulas[conflicto] {
				if l < 0 {
					l = -l
				}
				d.actividad[l] += d.inc
			}
			d.inc /= 0.95
			if d.inc > 1e100 {
				for v := range d.actividad {
					d.actividad[v] *= 1e-100
				}
				d.inc *= 1e-100
			}
			// chronological backtracking: flip the most recent decision not yet flipped
			volteo := false
			for len(d.decs) > 0 {
				k := len(d.decs) - 1
				dc := d.decs[k]
				d.deshacer(d.inicios[k])
				d.decs = d.decs[:k]
				d.inicios = d.inicios[:k]
				if !dc.volteada {
					d.inicios = append(d.inicios, len(d.traza))
					d.decs = append(d.decs, decision{lit: -dc.lit, volteada: true})
					d.poner(-dc.lit)
					volteo = true
					break
				}
			}
			if !volteo {
				return nil, false, nil
			}
		}
	}
	modelo = make([]bool, nVars)
	for v := 1; v <= nVars; v++ {
		modelo[v-1] = d.valor[v] == 1
	}
	// self-check against the original clauses
	for _, c := range cnf {
		ok := false
		for _, l := range c {
			if (l > 0 && modelo[l-1]) || (l < 0 && !modelo[-l-1]) {
				ok = true
				break
			}
		}
		if !ok {
			return nil, false, errors.New("logica: el modelo de DPLL no cumple una cláusula (error interno)")
		}
	}
	return modelo, true, nil
}

// Satisfacible runs Tseitin + DPLL on the conjunction of fs and returns a model over the atoms.
func Satisfacible(ctx context.Context, fs []Formula, nAtomos int) ([]bool, bool, error) {
	n := nAtomos
	if m := maxAtomo(fs...) + 1; m > n {
		n = m
	}
	cnf, nv := Tseitin(fs, n)
	modelo, sat, err := DPLL(ctx, cnf, nv, MaxDecisiones)
	if err != nil || !sat {
		return nil, sat, err
	}
	m := modelo[:n]
	for _, f := range fs {
		if !Evaluar(f, m) {
			return nil, false, errors.New("logica: el modelo no cumple las fórmulas (error interno)")
		}
	}
	return append([]bool(nil), m...), true, nil
}

// fuerzaBruta enumerates the 2^n assignments and returns the first that makes every f true.
func fuerzaBruta(fs []Formula, n int) ([]bool, bool) {
	v := make([]bool, n)
	for mask := 0; mask < 1<<n; mask++ {
		for i := 0; i < n; i++ {
			v[i] = mask&(1<<i) != 0
		}
		ok := true
		for _, f := range fs {
			if !Evaluar(f, v) {
				ok = false
				break
			}
		}
		if ok {
			return append([]bool(nil), v...), true
		}
	}
	return nil, false
}

// Implicacion decides whether prem ⊨ concl: prem ∧ ¬concl must be UNSAT. When it is not, contra
// is a counter-model over the atoms (premises true, conclusion false). With ≤ 12 atoms the result
// is cross-checked against the truth table.
func Implicacion(ctx context.Context, prem []Formula, concl Formula, nAtomos int) (valida bool, contra []bool, err error) {
	if concl == nil {
		return false, nil, errors.New("logica: no hay conclusión que comprobar")
	}
	n := nAtomos
	if m := maxAtomo(append(append([]Formula(nil), prem...), concl)...) + 1; m > n {
		n = m
	}
	fs := append(append([]Formula(nil), prem...), No{concl})
	modelo, sat, err := Satisfacible(ctx, fs, n)
	if err != nil {
		return false, nil, err
	}
	if n <= maxAtomosTabla {
		_, satTabla := fuerzaBruta(fs, n)
		if satTabla != sat {
			return false, nil, ErrNoCoinciden
		}
	}
	if sat {
		return false, modelo, nil
	}
	return true, nil, nil
}

// Tautologia reports whether f is true under every assignment; when it is not, the second result is
// an assignment that makes it false. DPLL decides it, cross-checked with the truth table for ≤ 12 atoms.
func Tautologia(f Formula, nAtomos int) (bool, []bool) {
	n := nAtomos
	if m := maxAtomo(f) + 1; m > n {
		n = m
	}
	modelo, sat, err := Satisfacible(context.Background(), []Formula{No{f}}, n)
	if err != nil || n <= maxAtomosTabla {
		if n <= 20 {
			m, s := fuerzaBruta([]Formula{No{f}}, n)
			return !s, m
		}
		return false, nil
	}
	return !sat, modelo
}
