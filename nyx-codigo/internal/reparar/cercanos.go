package reparar

import (
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
)

// damerau is the Damerau-Levenshtein distance (optimal string alignment) between a and b, by runes.
func damerau(a, b string) int {
	x, y := []rune(a), []rune(b)
	n, m := len(x), len(y)
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			coste := 1
			if x[i-1] == y[j-1] {
				coste = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+coste)
			if i > 1 && j > 1 && x[i-1] == y[j-2] && x[i-2] == y[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[n][m]
}

// cercanos returns the names closest to nombre: case-insensitive matches first, then Damerau-Levenshtein
// distance ≤ 2 (≤ 1 for names of 3 runes or fewer), at most max, best first.
func cercanos(nombre string, candidatos []string, max int) []string {
	type par struct {
		n string
		d int
	}
	var ps []par
	visto := map[string]bool{}
	bajo := strings.ToLower(nombre)
	limite := 2
	if len([]rune(nombre)) <= 3 {
		limite = 1
	}
	for _, c := range candidatos {
		if c == nombre || c == "_" || visto[c] {
			continue
		}
		visto[c] = true
		if strings.ToLower(c) == bajo {
			ps = append(ps, par{c, 0})
			continue
		}
		if d := damerau(bajo, strings.ToLower(c)); d <= limite {
			ps = append(ps, par{c, d})
		}
	}
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].d != ps[j].d {
			return ps[i].d < ps[j].d
		}
		// "Printl" was probably "Println" (cut short), not "Print"
		pi, pj := strings.HasPrefix(strings.ToLower(ps[i].n), bajo), strings.HasPrefix(strings.ToLower(ps[j].n), bajo)
		if pi != pj {
			return pi
		}
		if len(ps[i].n) != len(ps[j].n) {
			return len(ps[i].n) < len(ps[j].n)
		}
		return ps[i].n < ps[j].n
	})
	var out []string
	for _, p := range ps {
		if len(out) == max {
			break
		}
		out = append(out, p.n)
	}
	return out
}

// nombresVisibles returns the names visible at pos: locals declared before pos, package-level names
// and the universe.
func (a *analisis) nombresVisibles(pos token.Pos) []string {
	if a.paquete == nil {
		return nil
	}
	var out []string
	s := a.paquete.Scope().Innermost(pos)
	if s == nil {
		s = a.paquete.Scope()
	}
	for ; s != nil; s = s.Parent() {
		local := s != a.paquete.Scope() && s != types.Universe
		for _, n := range s.Names() {
			o := s.Lookup(n)
			if local && o.Pos() > pos {
				continue
			}
			out = append(out, n)
		}
	}
	return out
}

// buscarVisible looks a name up from pos.
func (a *analisis) buscarVisible(pos token.Pos, nombre string) types.Object {
	if a.paquete == nil {
		return nil
	}
	s := a.paquete.Scope().Innermost(pos)
	if s == nil {
		s = a.paquete.Scope()
	}
	_, o := s.LookupParent(nombre, pos)
	return o
}

// nombreLibre returns base, or base2, base3… so that it does not clash with a visible name.
func (a *analisis) nombreLibre(pos token.Pos, base string) string {
	n := base
	for i := 2; a.buscarVisible(pos, n) != nil && i < 100; i++ {
		n = base + strconv.Itoa(i)
	}
	return n
}

// camposYMetodos lists the field and method names reachable from a value of type t.
func camposYMetodos(t types.Type) []string {
	var out []string
	visto := map[string]bool{}
	agregar := func(n string) {
		if !visto[n] {
			visto[n] = true
			out = append(out, n)
		}
	}
	for _, tt := range []types.Type{t, types.NewPointer(t)} {
		ms := types.NewMethodSet(tt)
		for i := 0; i < ms.Len(); i++ {
			agregar(ms.At(i).Obj().Name())
		}
	}
	var campos func(t types.Type, prof int)
	campos = func(t types.Type, prof int) {
		if prof > 3 {
			return
		}
		if p, ok := t.Underlying().(*types.Pointer); ok {
			t = p.Elem()
		}
		s, ok := t.Underlying().(*types.Struct)
		if !ok {
			return
		}
		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			agregar(f.Name())
			if f.Embedded() {
				campos(f.Type(), prof+1)
			}
		}
	}
	campos(t, 0)
	return out
}
