package puzles

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// --- jugs

type estadoJarras struct {
	cap []int
	v   []int
}

func (e estadoJarras) Clave() string { return fmt.Sprint(e.v) }

func (e estadoJarras) String() string {
	var partes []string
	for i, x := range e.v {
		partes = append(partes, fmt.Sprintf("%s: %d l", nombreJarra(e.cap, i), x))
	}
	return strings.Join(partes, "; ")
}

func nombreJarra(caps []int, i int) string {
	for j, c := range caps {
		if j != i && c == caps[i] {
			return fmt.Sprintf("jarra %d (de %d l)", i+1, caps[i])
		}
	}
	return fmt.Sprintf("jarra de %d l", caps[i])
}

type problemaJarras struct {
	cap  []int
	meta int
}

// Jarras: jugs with the given capacities, all empty; the goal is some jug holding exactly meta
// litres. Actions: fill, empty, pour one into another.
func Jarras(capacidades []int, meta int) ProblemaPlan {
	return problemaJarras{cap: append([]int(nil), capacidades...), meta: meta}
}

func (p problemaJarras) Inicial() Estado {
	return estadoJarras{cap: p.cap, v: make([]int, len(p.cap))}
}

func (p problemaJarras) EsMeta(s Estado) bool {
	for _, x := range s.(estadoJarras).v {
		if x == p.meta {
			return true
		}
	}
	return false
}

func (p problemaJarras) Heuristica(Estado) int { return 0 }

func (p problemaJarras) Sucesores(s Estado) []Transicion {
	e := s.(estadoJarras)
	var out []Transicion
	nuevo := func(v []int) estadoJarras { return estadoJarras{cap: p.cap, v: v} }
	for i := range e.v {
		if e.v[i] < p.cap[i] {
			v := append([]int(nil), e.v...)
			v[i] = p.cap[i]
			out = append(out, Transicion{Accion: "llena la " + nombreJarra(p.cap, i), Destino: nuevo(v), Coste: 1})
		}
		if e.v[i] > 0 {
			v := append([]int(nil), e.v...)
			v[i] = 0
			out = append(out, Transicion{Accion: "vacía la " + nombreJarra(p.cap, i), Destino: nuevo(v), Coste: 1})
		}
	}
	for i := range e.v {
		for j := range e.v {
			if i == j || e.v[i] == 0 || e.v[j] == p.cap[j] {
				continue
			}
			x := min(e.v[i], p.cap[j]-e.v[j])
			v := append([]int(nil), e.v...)
			v[i] -= x
			v[j] += x
			out = append(out, Transicion{Accion: fmt.Sprintf("vierte la %s en la %s", nombreJarra(p.cap, i), nombreJarra(p.cap, j)), Destino: nuevo(v), Coste: 1})
		}
	}
	return out
}

// --- wolf, goat and cabbage

var nombresRio = []string{"el granjero", "el lobo", "la cabra", "la col"}

type estadoRio uint8 // bit i = 1: item i on the right bank (0 granjero, 1 lobo, 2 cabra, 3 col)

func (e estadoRio) Clave() string { return strconv.Itoa(int(e)) }

func (e estadoRio) String() string {
	var izq, der []string
	for i, n := range nombresRio {
		if e&(1<<uint(i)) != 0 {
			der = append(der, n)
		} else {
			izq = append(izq, n)
		}
	}
	vacio := func(xs []string) string {
		if len(xs) == 0 {
			return "nadie"
		}
		return strings.Join(xs, ", ")
	}
	return "orilla izquierda: " + vacio(izq) + " | orilla derecha: " + vacio(der)
}

type problemaRio struct{}

// Rio is the wolf, goat and cabbage problem: the farmer's boat carries him and at most one item; the
// wolf may not stay with the goat, nor the goat with the cabbage, without the farmer.
func Rio() ProblemaPlan { return problemaRio{} }

func (problemaRio) Inicial() Estado       { return estadoRio(0) }
func (problemaRio) EsMeta(s Estado) bool  { return s.(estadoRio) == 15 }
func (problemaRio) Heuristica(Estado) int { return 0 }

func rioSeguro(e estadoRio) bool {
	lado := func(i int) bool { return e&(1<<uint(i)) != 0 }
	if lado(1) == lado(2) && lado(0) != lado(1) {
		return false
	}
	if lado(2) == lado(3) && lado(0) != lado(2) {
		return false
	}
	return true
}

func (problemaRio) Sucesores(s Estado) []Transicion {
	e := s.(estadoRio)
	granjero := e & 1
	destino := "a la orilla derecha"
	if granjero != 0 {
		destino = "a la orilla izquierda"
	}
	var out []Transicion
	for i := 0; i < 4; i++ {
		if i > 0 && (e>>uint(i))&1 != granjero {
			continue
		}
		n := e ^ 1
		accion := "el granjero cruza solo " + destino
		if i > 0 {
			n ^= 1 << uint(i)
			accion = "el granjero cruza con " + nombresRio[i] + " " + destino
		}
		if rioSeguro(n) {
			out = append(out, Transicion{Accion: accion, Destino: n, Coste: 1})
		}
	}
	return out
}

// --- missionaries and cannibals

type estadoMisioneros struct{ m, c, barca int } // on the left bank; barca 0 = left

func (e estadoMisioneros) Clave() string { return fmt.Sprintf("%d,%d,%d", e.m, e.c, e.barca) }

type problemaMisioneros struct{ m, c, barca int }

func (p problemaMisioneros) describir(e estadoMisioneros) string {
	lado := "izquierda"
	if e.barca == 1 {
		lado = "derecha"
	}
	return fmt.Sprintf("izquierda: %d misioneros y %d caníbales | derecha: %d misioneros y %d caníbales | barca a la %s",
		e.m, e.c, p.m-e.m, p.c-e.c, lado)
}

type estadoMisionerosTexto struct {
	estadoMisioneros
	texto string
}

func (e estadoMisionerosTexto) String() string { return e.texto }

// Misioneros: m missionaries and c cannibals cross with a boat for barca people; on neither bank
// may cannibals outnumber the missionaries present.
func Misioneros(m, c, barca int) ProblemaPlan {
	if barca <= 0 {
		barca = 2
	}
	return problemaMisioneros{m: m, c: c, barca: barca}
}

func (p problemaMisioneros) envolver(e estadoMisioneros) Estado {
	return estadoMisionerosTexto{estadoMisioneros: e, texto: p.describir(e)}
}

func (p problemaMisioneros) Inicial() Estado { return p.envolver(estadoMisioneros{p.m, p.c, 0}) }

func (p problemaMisioneros) EsMeta(s Estado) bool {
	e := s.(estadoMisionerosTexto)
	return e.m == 0 && e.c == 0 && e.barca == 1
}

// Heuristica: every crossing carries at most barca people (a lower bound on forward crossings).
func (p problemaMisioneros) Heuristica(s Estado) int {
	e := s.(estadoMisionerosTexto)
	return (e.m + e.c + p.barca - 1) / p.barca
}

func (p problemaMisioneros) seguro(m, c int) bool {
	if m < 0 || c < 0 || m > p.m || c > p.c {
		return false
	}
	if m > 0 && c > m {
		return false
	}
	mr, cr := p.m-m, p.c-c
	return mr == 0 || cr <= mr
}

func (p problemaMisioneros) Sucesores(s Estado) []Transicion {
	e := s.(estadoMisionerosTexto)
	signo := -1
	hacia := "a la derecha"
	if e.barca == 1 {
		signo, hacia = 1, "de vuelta a la izquierda"
	}
	var out []Transicion
	for dm := 0; dm <= p.barca; dm++ {
		for dc := 0; dm+dc <= p.barca; dc++ {
			if dm+dc == 0 {
				continue
			}
			m, c := e.m+signo*dm, e.c+signo*dc
			if !p.seguro(m, c) {
				continue
			}
			var quien []string
			if dm > 0 {
				quien = append(quien, plural(dm, "misionero", "misioneros"))
			}
			if dc > 0 {
				quien = append(quien, plural(dc, "caníbal", "caníbales"))
			}
			verbo := "cruzan"
			if dm+dc == 1 {
				verbo = "cruza"
			}
			out = append(out, Transicion{Accion: fmt.Sprintf("%s %s %s", verbo, strings.Join(quien, " y "), hacia),
				Destino: p.envolver(estadoMisioneros{m, c, 1 - e.barca}), Coste: 1})
		}
	}
	return out
}

func plural(n int, uno, varios string) string {
	if n == 1 {
		return "1 " + uno
	}
	return fmt.Sprintf("%d %s", n, varios)
}

// --- 8-puzzle

type estadoPuzzle [9]int8

func (e estadoPuzzle) Clave() string {
	var b [9]byte
	for i, x := range e {
		b[i] = byte('0' + x)
	}
	return string(b[:])
}

func (e estadoPuzzle) String() string {
	var filas []string
	for f := 0; f < 3; f++ {
		var cs []string
		for c := 0; c < 3; c++ {
			x := e[f*3+c]
			if x == 0 {
				cs = append(cs, "_")
			} else {
				cs = append(cs, strconv.Itoa(int(x)))
			}
		}
		filas = append(filas, strings.Join(cs, " "))
	}
	return strings.Join(filas, " / ")
}

type problemaPuzzle struct{ ini estadoPuzzle }

// Puzzle8 is the 8-puzzle (0 is the blank; goal 1 2 3 / 4 5 6 / 7 8 _). The bool reports whether it
// is solvable (even number of inversions); the problem is nil when inicial is not a permutation of 0..8.
func Puzzle8(inicial [9]int) (ProblemaPlan, bool) {
	var e estadoPuzzle
	visto := [9]bool{}
	for i, x := range inicial {
		if x < 0 || x > 8 || visto[x] {
			return nil, false
		}
		visto[x] = true
		e[i] = int8(x)
	}
	inv := 0
	for i := 0; i < 9; i++ {
		for j := i + 1; j < 9; j++ {
			if e[i] != 0 && e[j] != 0 && e[i] > e[j] {
				inv++
			}
		}
	}
	return problemaPuzzle{ini: e}, inv%2 == 0
}

func (p problemaPuzzle) Inicial() Estado { return p.ini }

func (p problemaPuzzle) EsMeta(s Estado) bool {
	e := s.(estadoPuzzle)
	for i := 0; i < 8; i++ {
		if int(e[i]) != i+1 {
			return false
		}
	}
	return e[8] == 0
}

// Heuristica is the Manhattan distance of the tiles to their places.
func (p problemaPuzzle) Heuristica(s Estado) int {
	e := s.(estadoPuzzle)
	d := 0
	for i, x := range e {
		if x == 0 {
			continue
		}
		obj := int(x) - 1
		d += abs(i/3-obj/3) + abs(i%3-obj%3)
	}
	return d
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (p problemaPuzzle) Sucesores(s Estado) []Transicion {
	e := s.(estadoPuzzle)
	b := 0
	for i, x := range e {
		if x == 0 {
			b = i
		}
	}
	var out []Transicion
	mover := func(t int, dir string) {
		n := e
		n[b], n[t] = n[t], 0
		out = append(out, Transicion{Accion: fmt.Sprintf("mueve el %d %s", e[t], dir), Destino: n, Coste: 1})
	}
	if b%3 < 2 {
		mover(b+1, "a la izquierda")
	}
	if b%3 > 0 {
		mover(b-1, "a la derecha")
	}
	if b/3 < 2 {
		mover(b+3, "arriba")
	}
	if b/3 > 0 {
		mover(b-3, "abajo")
	}
	return out
}

// --- maze

type estadoLaberinto struct{ f, c int }

func (e estadoLaberinto) Clave() string  { return fmt.Sprintf("%d,%d", e.f, e.c) }
func (e estadoLaberinto) String() string { return fmt.Sprintf("fila %d, columna %d", e.f+1, e.c+1) }

type problemaLaberinto struct {
	mapa      [][]rune
	ini, meta estadoLaberinto
}

// Laberinto reads a maze drawn in text: S start, E exit, # wall; anything else is open.
func Laberinto(lineas []string) (ProblemaPlan, error) {
	p := problemaLaberinto{ini: estadoLaberinto{-1, -1}, meta: estadoLaberinto{-1, -1}}
	for f, l := range lineas {
		fila := []rune(strings.TrimRight(l, "\r"))
		for c, r := range fila {
			switch r {
			case 'S', 's':
				if p.ini.f >= 0 {
					return nil, fmt.Errorf("puzles: el laberinto tiene dos salidas «S»: %w", nucleo.ErrNoEntiendo)
				}
				p.ini = estadoLaberinto{f, c}
			case 'E', 'e':
				if p.meta.f >= 0 {
					return nil, fmt.Errorf("puzles: el laberinto tiene dos «E»: %w", nucleo.ErrNoEntiendo)
				}
				p.meta = estadoLaberinto{f, c}
			}
		}
		p.mapa = append(p.mapa, fila)
	}
	if p.ini.f < 0 || p.meta.f < 0 {
		return nil, fmt.Errorf("puzles: el laberinto necesita una «S» (inicio) y una «E» (salida): %w", nucleo.ErrNoEntiendo)
	}
	return p, nil
}

func (p problemaLaberinto) Inicial() Estado { return p.ini }

func (p problemaLaberinto) EsMeta(s Estado) bool { return s.(estadoLaberinto) == p.meta }

func (p problemaLaberinto) Heuristica(s Estado) int {
	e := s.(estadoLaberinto)
	return abs(e.f-p.meta.f) + abs(e.c-p.meta.c)
}

func (p problemaLaberinto) libre(f, c int) bool {
	return f >= 0 && f < len(p.mapa) && c >= 0 && c < len(p.mapa[f]) && p.mapa[f][c] != '#'
}

func (p problemaLaberinto) Sucesores(s Estado) []Transicion {
	e := s.(estadoLaberinto)
	var out []Transicion
	for _, m := range []struct {
		df, dc int
		dir    string
	}{{-1, 0, "arriba"}, {1, 0, "abajo"}, {0, -1, "izquierda"}, {0, 1, "derecha"}} {
		if p.libre(e.f+m.df, e.c+m.dc) {
			out = append(out, Transicion{Accion: m.dir, Destino: estadoLaberinto{e.f + m.df, e.c + m.dc}, Coste: 1})
		}
	}
	return out
}

// --- Hanoi

const maxDiscosLista = 10

// Hanoi returns the moves for n discs from A to C (listed only for n ≤ 10) and the total 2ⁿ − 1.
func Hanoi(n int) (movimientos []string, total *big.Int) {
	if n < 0 {
		n = 0
	}
	total = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(n)), big.NewInt(1))
	if n > maxDiscosLista {
		return nil, total
	}
	var rec func(k int, de, a, via string)
	rec = func(k int, de, a, via string) {
		if k == 0 {
			return
		}
		rec(k-1, de, via, a)
		movimientos = append(movimientos, fmt.Sprintf("mueve el disco %d de %s a %s", k, de, a))
		rec(k-1, via, a, de)
	}
	rec(n, "A", "C", "B")
	return movimientos, total
}

// ComprobarHanoi replays the moves: every move must take the top disc onto a larger one, and all n
// discs must end on C.
func ComprobarHanoi(n int, movimientos []string) error {
	torres := map[string][]int{"A": nil, "B": nil, "C": nil}
	for d := n; d >= 1; d-- {
		torres["A"] = append(torres["A"], d)
	}
	for i, m := range movimientos {
		var d int
		var de, a string
		if _, err := fmt.Sscanf(m, "mueve el disco %d de %s a %s", &d, &de, &a); err != nil {
			return fmt.Errorf("movimiento %d ilegible: %q", i+1, m)
		}
		t := torres[de]
		if len(t) == 0 || t[len(t)-1] != d {
			return fmt.Errorf("movimiento %d: el disco %d no está arriba en %s", i+1, d, de)
		}
		if dst := torres[a]; len(dst) > 0 && dst[len(dst)-1] < d {
			return fmt.Errorf("movimiento %d: el disco %d no cabe sobre uno menor", i+1, d)
		}
		torres[de] = t[:len(t)-1]
		torres[a] = append(torres[a], d)
	}
	if len(torres["C"]) != n {
		return fmt.Errorf("al final hay %d discos en C, no %d", len(torres["C"]), n)
	}
	return nil
}
