// Package puzles solves constraint puzzles (sudoku, queens, cryptarithms, zebra, map colouring),
// plans (BFS, A*, IDA*), counting and probability questions and number sequences (§4.11). Every
// answer is re-checked before it is returned. It imports only the stdlib and nucleo.
package puzles

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Dominio is a bitset of the values 0..63.
type Dominio uint64

// DominioRango is {a, …, b} (clamped to 0..63).
func DominioRango(a, b int) Dominio {
	var d Dominio
	for v := max(a, 0); v <= b && v < 64; v++ {
		d |= 1 << uint(v)
	}
	return d
}

// Tiene reports whether v is in the domain.
func (d Dominio) Tiene(v int) bool { return v >= 0 && v < 64 && d&(1<<uint(v)) != 0 }

// Cuenta is the number of values.
func (d Dominio) Cuenta() int { return bits.OnesCount64(uint64(d)) }

// Valores lists the values in increasing order.
func (d Dominio) Valores() []int {
	var out []int
	for x := uint64(d); x != 0; x &= x - 1 {
		out = append(out, bits.TrailingZeros64(x))
	}
	return out
}

// Unico returns the only value when the domain has exactly one.
func (d Dominio) Unico() (int, bool) {
	if d.Cuenta() != 1 {
		return 0, false
	}
	return bits.TrailingZeros64(uint64(d)), true
}

func (d Dominio) minimo() int { return bits.TrailingZeros64(uint64(d)) }
func (d Dominio) maximo() int { return 63 - bits.LeadingZeros64(uint64(d)) }

// Restriccion is one constraint over the variables in Alcance.
type Restriccion struct {
	Alcance []int
	Tipo    int // Distintos | SumaIgual | Binaria | Func
	K       int
	Rel     func(vals []int) bool // for Binaria/Func
}

const (
	Distintos = iota
	SumaIgual
	Binaria
	Func
)

// Problema is a finite-domain constraint problem.
type Problema struct {
	Nombres []string
	Dom     []Dominio
	Restr   []Restriccion
}

// Estadisticas count the search effort.
type Estadisticas struct {
	Nodos      int
	Retrocesos int
	Ms         int64
}

// Limits of Resolver (§4.11).
const (
	MaxNodos   = 5_000_000
	TiempoCSP  = 5 * time.Second
	maxValores = 64
)

// ErrLimite is wrapped (with nucleo.ErrSinTiempo) when the search hits its node or time limit.
var ErrLimite = errors.New("puzles: límite de búsqueda alcanzado")

type buscador struct {
	p        *Problema
	porVar   [][]int
	grado    []int
	maxSol   int
	sols     [][]int
	est      Estadisticas
	ctx      context.Context
	limite   time.Time
	n        *nucleo.Nodo
	err      error
	maxNodos int
}

// Resolver finds up to maxSol solutions (maxSol ≤ 0 means 1) by AC-3 style propagation, MRV with a
// degree tie-break and forward checking. Limits: 5·10⁶ nodes and 5 s; on a limit it returns the
// solutions found so far and an error wrapping nucleo.ErrSinTiempo. Every solution is re-checked
// against all the constraints before it is returned.
func Resolver(ctx context.Context, p *Problema, maxSol int, n *nucleo.Nodo) ([][]int, Estadisticas, error) {
	return resolverCon(ctx, p, maxSol, n, MaxNodos)
}

func resolverCon(ctx context.Context, p *Problema, maxSol int, n *nucleo.Nodo, maxNodos int) ([][]int, Estadisticas, error) {
	ini := time.Now()
	if ctx == nil {
		ctx = context.Background()
	}
	if maxSol <= 0 {
		maxSol = 1
	}
	if err := validarProblema(p); err != nil {
		return nil, Estadisticas{}, err
	}
	b := &buscador{p: p, maxSol: maxSol, ctx: ctx, limite: ini.Add(TiempoCSP), n: n, maxNodos: maxNodos}
	b.porVar = make([][]int, len(p.Dom))
	b.grado = make([]int, len(p.Dom))
	for ci, r := range p.Restr {
		for _, v := range r.Alcance {
			b.porVar[v] = append(b.porVar[v], ci)
			b.grado[v] += len(r.Alcance) - 1
		}
	}
	doms := append([]Dominio(nil), p.Dom...)
	if b.propagar(doms, nil) {
		b.buscar(doms)
	}
	b.est.Ms = time.Since(ini).Milliseconds()
	for _, s := range b.sols {
		if err := Comprobar(p, s); err != nil {
			return nil, b.est, fmt.Errorf("puzles: una solución no cumple las restricciones (error interno): %w", err)
		}
	}
	return b.sols, b.est, b.err
}

func validarProblema(p *Problema) error {
	if p == nil {
		return errors.New("puzles: problema vacío")
	}
	for ci, r := range p.Restr {
		for _, v := range r.Alcance {
			if v < 0 || v >= len(p.Dom) {
				return fmt.Errorf("puzles: la restricción %d usa la variable %d, que no existe", ci, v)
			}
		}
		if (r.Tipo == Binaria || r.Tipo == Func) && r.Rel == nil {
			return fmt.Errorf("puzles: la restricción %d no tiene relación", ci)
		}
		if r.Tipo == Binaria && len(r.Alcance) != 2 {
			return fmt.Errorf("puzles: la restricción binaria %d no tiene dos variables", ci)
		}
	}
	return nil
}

// Comprobar re-checks a full assignment against every constraint and domain.
func Comprobar(p *Problema, s []int) error {
	if len(s) != len(p.Dom) {
		return fmt.Errorf("la solución tiene %d valores y hay %d variables", len(s), len(p.Dom))
	}
	for i, v := range s {
		if !p.Dom[i].Tiene(v) {
			return fmt.Errorf("%s = %d está fuera de su dominio", nombreVar(p, i), v)
		}
	}
	for ci, r := range p.Restr {
		vals := make([]int, len(r.Alcance))
		for k, v := range r.Alcance {
			vals[k] = s[v]
		}
		if !cumple(r, vals) {
			return fmt.Errorf("no se cumple la restricción %d", ci)
		}
	}
	return nil
}

func nombreVar(p *Problema, i int) string {
	if i < len(p.Nombres) && p.Nombres[i] != "" {
		return p.Nombres[i]
	}
	return fmt.Sprintf("x%d", i)
}

func cumple(r Restriccion, vals []int) bool {
	switch r.Tipo {
	case Distintos:
		var visto Dominio
		for _, v := range vals {
			if visto.Tiene(v) {
				return false
			}
			visto |= 1 << uint(v)
		}
		return true
	case SumaIgual:
		s := 0
		for _, v := range vals {
			s += v
		}
		return s == r.K
	default:
		return r.Rel(vals)
	}
}

// propagar makes the domains consistent; it returns false on a wipe-out. cambiadas lists the
// variables whose domains changed (nil: check every constraint).
func (b *buscador) propagar(doms []Dominio, cambiadas []int) bool {
	cola := make([]int, 0, len(b.p.Restr))
	enCola := make([]bool, len(b.p.Restr))
	meter := func(ci int) {
		if !enCola[ci] {
			enCola[ci] = true
			cola = append(cola, ci)
		}
	}
	if cambiadas == nil {
		for ci := range b.p.Restr {
			meter(ci)
		}
	} else {
		for _, v := range cambiadas {
			for _, ci := range b.porVar[v] {
				meter(ci)
			}
		}
	}
	for len(cola) > 0 {
		ci := cola[0]
		cola = cola[1:]
		enCola[ci] = false
		r := b.p.Restr[ci]
		cambio, ok := b.revisar(r, doms)
		if !ok {
			return false
		}
		for _, v := range cambio {
			for _, cj := range b.porVar[v] {
				if cj != ci {
					meter(cj)
				}
			}
		}
	}
	return true
}

// revisar filters the domains of one constraint; it returns the variables changed.
func (b *buscador) revisar(r Restriccion, doms []Dominio) ([]int, bool) {
	var cambio []int
	poner := func(v int, d Dominio) bool {
		if d == doms[v] {
			return true
		}
		doms[v] = d
		cambio = append(cambio, v)
		return d != 0
	}
	switch r.Tipo {
	case Distintos:
		// naked singles, repeated until stable
		for repetir := true; repetir; {
			repetir = false
			for _, v := range r.Alcance {
				x, ok := doms[v].Unico()
				if !ok {
					continue
				}
				for _, w := range r.Alcance {
					if w != v && doms[w].Tiene(x) {
						if !poner(w, doms[w]&^(1<<uint(x))) {
							return cambio, false
						}
						repetir = true
					}
				}
			}
		}
		var union Dominio
		for _, v := range r.Alcance {
			union |= doms[v]
		}
		if union.Cuenta() < len(r.Alcance) {
			return cambio, false
		}
		if union.Cuenta() == len(r.Alcance) {
			// hidden singles: a value that only one variable can take
			for _, x := range union.Valores() {
				solo, n := -1, 0
				for _, v := range r.Alcance {
					if doms[v].Tiene(x) {
						solo, n = v, n+1
					}
				}
				if n == 1 && doms[solo].Cuenta() > 1 {
					if !poner(solo, 1<<uint(x)) {
						return cambio, false
					}
				}
			}
		}
	case SumaIgual:
		minT, maxT := 0, 0
		for _, v := range r.Alcance {
			minT += doms[v].minimo()
			maxT += doms[v].maximo()
		}
		if r.K < minT || r.K > maxT {
			return cambio, false
		}
		for _, v := range r.Alcance {
			lo := r.K - (maxT - doms[v].maximo())
			hi := r.K - (minT - doms[v].minimo())
			d := doms[v] & DominioRango(lo, hi)
			if !poner(v, d) {
				return cambio, false
			}
		}
	case Binaria:
		x, y := r.Alcance[0], r.Alcance[1]
		par := []int{0, 0}
		var dx, dy Dominio
		for _, a := range doms[x].Valores() {
			for _, c := range doms[y].Valores() {
				par[0], par[1] = a, c
				if r.Rel(par) {
					dx |= 1 << uint(a)
					dy |= 1 << uint(c)
				}
			}
		}
		if !poner(x, doms[x]&dx) || !poner(y, doms[y]&dy) {
			return cambio, false
		}
	case Func:
		// forward checking: when at most one variable is open, filter it
		libre, nLibres := -1, 0
		for k, v := range r.Alcance {
			if doms[v].Cuenta() > 1 {
				libre, nLibres = k, nLibres+1
			}
		}
		if nLibres > 1 {
			return cambio, true
		}
		vals := make([]int, len(r.Alcance))
		for k, v := range r.Alcance {
			vals[k] = doms[v].minimo()
		}
		if nLibres == 0 {
			return cambio, r.Rel(vals)
		}
		v := r.Alcance[libre]
		var d Dominio
		for _, x := range doms[v].Valores() {
			vals[libre] = x
			if r.Rel(vals) {
				d |= 1 << uint(x)
			}
		}
		if !poner(v, d) {
			return cambio, false
		}
	}
	return cambio, true
}

// buscar is the recursive search; it returns false when the search must stop.
func (b *buscador) buscar(doms []Dominio) bool {
	// MRV with degree tie-break
	elegida := -1
	for v, d := range doms {
		c := d.Cuenta()
		if c <= 1 {
			continue
		}
		if elegida < 0 || c < doms[elegida].Cuenta() || (c == doms[elegida].Cuenta() && b.grado[v] > b.grado[elegida]) {
			elegida = v
		}
	}
	if elegida < 0 {
		sol := make([]int, len(doms))
		for i, d := range doms {
			sol[i] = d.minimo()
		}
		b.sols = append(b.sols, sol)
		if b.n != nil {
			b.n.Progreso("%d soluciones", len(b.sols))
		}
		return len(b.sols) < b.maxSol
	}
	for _, x := range doms[elegida].Valores() {
		b.est.Nodos++
		if b.est.Nodos > b.maxNodos {
			b.err = fmt.Errorf("puzles: más de %d nodos (%w): %w", b.maxNodos, ErrLimite, nucleo.ErrSinTiempo)
			return false
		}
		if b.est.Nodos&1023 == 0 {
			if b.ctx.Err() != nil || time.Now().After(b.limite) {
				b.err = fmt.Errorf("puzles: sin tiempo tras %d nodos (%w): %w", b.est.Nodos, ErrLimite, nucleo.ErrSinTiempo)
				return false
			}
			if b.n != nil {
				b.n.Progreso("%d caminos probados", b.est.Nodos)
			}
		}
		hijo := append([]Dominio(nil), doms...)
		hijo[elegida] = 1 << uint(x)
		if b.propagar(hijo, []int{elegida}) {
			if !b.buscar(hijo) {
				return false
			}
		} else {
			b.est.Retrocesos++
		}
	}
	return true
}
