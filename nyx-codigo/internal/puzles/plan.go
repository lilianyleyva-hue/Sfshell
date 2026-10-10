package puzles

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Estado is one state of a planning problem.
type Estado interface {
	Clave() string
	String() string // Spanish description
}

// Transicion is an action and the state it leads to.
type Transicion struct {
	Accion  string
	Destino Estado
	Coste   int
}

// ProblemaPlan is a planning problem. Heuristica must never overestimate (0 is always fine).
type ProblemaPlan interface {
	Inicial() Estado
	EsMeta(Estado) bool
	Sucesores(Estado) []Transicion
	Heuristica(Estado) int
}

// Plan is a sequence of actions; Estados has one more entry than Pasos (it starts with the initial state).
type Plan struct {
	Pasos      []string
	Estados    []string
	Coste      int
	Explorados int
}

// Limits of the planners (§4.11).
const (
	MaxEstados = 2_000_000
	TiempoPlan = 5 * time.Second
)

// ErrSinPlan is returned when no sequence of actions reaches the goal.
var ErrSinPlan = errors.New("puzles: no hay ningún plan que llegue a la meta")

type nodoPlan struct {
	estado Estado
	padre  int
	accion string
	g      int
}

func reconstruir(nodos []nodoPlan, i, explorados int) Plan {
	var pasos, estados []string
	coste := nodos[i].g
	for ; i >= 0; i = nodos[i].padre {
		estados = append(estados, nodos[i].estado.String())
		if nodos[i].padre >= 0 {
			pasos = append(pasos, nodos[i].accion)
		}
	}
	for a, b := 0, len(pasos)-1; a < b; a, b = a+1, b-1 {
		pasos[a], pasos[b] = pasos[b], pasos[a]
	}
	for a, b := 0, len(estados)-1; a < b; a, b = a+1, b-1 {
		estados[a], estados[b] = estados[b], estados[a]
	}
	return Plan{Pasos: pasos, Estados: estados, Coste: coste, Explorados: explorados}
}

func limitesPlan(ctx context.Context, limite int) (context.Context, int, time.Time) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limite <= 0 || limite > MaxEstados {
		limite = MaxEstados
	}
	return ctx, limite, time.Now().Add(TiempoPlan)
}

func errLimitePlan(n int) error {
	return fmt.Errorf("puzles: exploré %d estados sin llegar a la meta: %w", n, nucleo.ErrSinTiempo)
}

// BFS finds a plan with the fewest actions (optimal for unit costs). limite caps the states kept
// (≤ 2·10⁶); the search also stops after 5 s.
func BFS(ctx context.Context, p ProblemaPlan, limite int) (Plan, error) {
	ctx, limite, fin := limitesPlan(ctx, limite)
	ini := p.Inicial()
	nodos := []nodoPlan{{estado: ini, padre: -1}}
	vistos := map[string]bool{ini.Clave(): true}
	for cab := 0; cab < len(nodos); cab++ {
		if cab&1023 == 0 && (ctx.Err() != nil || time.Now().After(fin)) {
			return Plan{Explorados: cab}, errLimitePlan(cab)
		}
		n := nodos[cab]
		if p.EsMeta(n.estado) {
			return reconstruir(nodos, cab, cab+1), nil
		}
		for _, t := range p.Sucesores(n.estado) {
			k := t.Destino.Clave()
			if vistos[k] {
				continue
			}
			vistos[k] = true
			nodos = append(nodos, nodoPlan{estado: t.Destino, padre: cab, accion: t.Accion, g: n.g + t.Coste})
			if len(nodos) > limite {
				return Plan{Explorados: cab}, errLimitePlan(cab)
			}
		}
	}
	return Plan{Explorados: len(nodos)}, ErrSinPlan
}

type entradaCola struct {
	nodo int
	f, g int
}

type colaPrioridad []entradaCola

func (c colaPrioridad) Len() int { return len(c) }
func (c colaPrioridad) Less(i, j int) bool {
	if c[i].f != c[j].f {
		return c[i].f < c[j].f
	}
	return c[i].g > c[j].g
}
func (c colaPrioridad) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c *colaPrioridad) Push(x any)   { *c = append(*c, x.(entradaCola)) }
func (c *colaPrioridad) Pop() any {
	v := (*c)[len(*c)-1]
	*c = (*c)[:len(*c)-1]
	return v
}

// AEstrella is A* with the problem's heuristic (optimal when it is admissible).
func AEstrella(ctx context.Context, p ProblemaPlan, limite int) (Plan, error) {
	ctx, limite, fin := limitesPlan(ctx, limite)
	ini := p.Inicial()
	nodos := []nodoPlan{{estado: ini, padre: -1}}
	mejorG := map[string]int{ini.Clave(): 0}
	cerrados := map[string]bool{}
	cola := &colaPrioridad{{nodo: 0, f: p.Heuristica(ini), g: 0}}
	explorados := 0
	for cola.Len() > 0 {
		e := heap.Pop(cola).(entradaCola)
		n := nodos[e.nodo]
		k := n.estado.Clave()
		if cerrados[k] || e.g > mejorG[k] {
			continue
		}
		cerrados[k] = true
		explorados++
		if explorados&1023 == 0 && (ctx.Err() != nil || time.Now().After(fin)) {
			return Plan{Explorados: explorados}, errLimitePlan(explorados)
		}
		if p.EsMeta(n.estado) {
			return reconstruir(nodos, e.nodo, explorados), nil
		}
		for _, t := range p.Sucesores(n.estado) {
			kd := t.Destino.Clave()
			g := n.g + t.Coste
			if cerrados[kd] {
				continue
			}
			if old, ok := mejorG[kd]; ok && old <= g {
				continue
			}
			mejorG[kd] = g
			nodos = append(nodos, nodoPlan{estado: t.Destino, padre: e.nodo, accion: t.Accion, g: g})
			heap.Push(cola, entradaCola{nodo: len(nodos) - 1, f: g + p.Heuristica(t.Destino), g: g})
			if len(nodos) > limite {
				return Plan{Explorados: explorados}, errLimitePlan(explorados)
			}
		}
	}
	return Plan{Explorados: explorados}, ErrSinPlan
}

// IDAEstrella is iterative-deepening A* (little memory; used for the 8-puzzle with Manhattan).
func IDAEstrella(ctx context.Context, p ProblemaPlan, limite int) (Plan, error) {
	ctx, limite, fin := limitesPlan(ctx, limite)
	ini := p.Inicial()
	cota := p.Heuristica(ini)
	explorados := 0
	enCamino := map[string]bool{ini.Clave(): true}
	estados := []Estado{ini}
	var acciones []string
	var errBusca error
	const infinito = 1 << 30
	var dfs func(s Estado, g int) (int, bool)
	dfs = func(s Estado, g int) (int, bool) {
		f := g + p.Heuristica(s)
		if f > cota {
			return f, false
		}
		if p.EsMeta(s) {
			return f, true
		}
		explorados++
		if explorados > limite*4 {
			errBusca = errLimitePlan(explorados)
			return infinito, false
		}
		if explorados&4095 == 0 && (ctx.Err() != nil || time.Now().After(fin)) {
			errBusca = errLimitePlan(explorados)
			return infinito, false
		}
		minimo := infinito
		for _, t := range p.Sucesores(s) {
			k := t.Destino.Clave()
			if enCamino[k] {
				continue
			}
			enCamino[k] = true
			estados = append(estados, t.Destino)
			acciones = append(acciones, t.Accion)
			v, ok := dfs(t.Destino, g+t.Coste)
			if ok {
				return v, true
			}
			if errBusca != nil {
				return infinito, false
			}
			estados = estados[:len(estados)-1]
			acciones = acciones[:len(acciones)-1]
			delete(enCamino, k)
			minimo = min(minimo, v)
		}
		return minimo, false
	}
	for {
		v, ok := dfs(ini, 0)
		if ok {
			plan := Plan{Pasos: append([]string(nil), acciones...), Explorados: explorados}
			for _, s := range estados {
				plan.Estados = append(plan.Estados, s.String())
			}
			// recompute the cost by replaying the actions
			s := ini
			for _, a := range acciones {
				for _, t := range p.Sucesores(s) {
					if t.Accion == a {
						plan.Coste += t.Coste
						s = t.Destino
						break
					}
				}
			}
			return plan, nil
		}
		if errBusca != nil {
			return Plan{Explorados: explorados}, errBusca
		}
		if v >= infinito {
			return Plan{Explorados: explorados}, ErrSinPlan
		}
		cota = v
	}
}

// Simular replays the plan from the initial state: every action must be available, every
// intermediate state must match Estados (when given), the cost must add up, and the last state
// must be a goal.
func Simular(p ProblemaPlan, plan Plan) bool {
	s := p.Inicial()
	if len(plan.Estados) > 0 && (len(plan.Estados) != len(plan.Pasos)+1 || plan.Estados[0] != s.String()) {
		return false
	}
	coste := 0
	for i, a := range plan.Pasos {
		hecho := false
		for _, t := range p.Sucesores(s) {
			if t.Accion != a {
				continue
			}
			if len(plan.Estados) > 0 && t.Destino.String() != plan.Estados[i+1] {
				continue
			}
			s = t.Destino
			coste += t.Coste
			hecho = true
			break
		}
		if !hecho {
			return false
		}
	}
	return p.EsMeta(s) && coste == plan.Coste
}
