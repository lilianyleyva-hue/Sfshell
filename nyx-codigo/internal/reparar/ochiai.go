package reparar

import (
	"go/ast"
	"math"
	"sort"
)

// Ochiai computes the suspiciousness of every statement from per-case coverage:
//
//	fail(s) / sqrt(totalFail · (fail(s) + pass(s)))
//
// cubiertas[i] lists the statement ids run by case i and ok[i] says whether case i passed. Ids outside
// [0, nSentencias) are ignored and an id listed twice in one case counts once. Every failing case counts
// toward totalFail, even when it has no coverage (it crashed before reporting any). With no failing case
// every score is 0.
func Ochiai(cubiertas [][]int32, ok []bool, nSentencias int) []float64 {
	if nSentencias < 0 {
		nSentencias = 0
	}
	fallan := make([]int, nSentencias)
	pasan := make([]int, nSentencias)
	marca := make([]int, nSentencias)
	totalFallos := 0
	for i := range ok {
		if !ok[i] {
			totalFallos++
		}
		if i >= len(cubiertas) {
			continue
		}
		for _, s := range cubiertas[i] {
			if s < 0 || int(s) >= nSentencias || marca[s] == i+1 {
				continue
			}
			marca[s] = i + 1
			if ok[i] {
				pasan[s]++
			} else {
				fallan[s]++
			}
		}
	}
	out := make([]float64, nSentencias)
	if totalFallos == 0 {
		return out
	}
	for s := range out {
		if fallan[s] == 0 {
			continue
		}
		out[s] = float64(fallan[s]) / math.Sqrt(float64(totalFallos)*float64(fallan[s]+pasan[s]))
	}
	return out
}

// maxSospechosas is how many statements logic repair mutates (§4.4: the top 6), and maxEmpates how many
// it takes at most when several share the sixth score.
const (
	maxSospechosas = 6
	maxEmpates     = 12
)

// sospechosas turns Ochiai scores into the lines to mutate: the top statements (ties at the cut kept, up
// to maxEmpates) plus the headers of the loops that enclose them. It also returns the score of every
// line (the best statement on it). lineas maps statement id → source line.
func sospechosas(a *analisis, lineas []int, puntos []float64) (map[int]float64, []int) {
	porLinea := map[int]float64{}
	ids := make([]int, 0, len(puntos))
	for id, p := range puntos {
		if id >= len(lineas) {
			break
		}
		if p > porLinea[lineas[id]] {
			porLinea[lineas[id]] = p
		}
		if p > 0 {
			ids = append(ids, id)
		}
	}
	sort.SliceStable(ids, func(i, j int) bool {
		if puntos[ids[i]] != puntos[ids[j]] {
			return puntos[ids[i]] > puntos[ids[j]]
		}
		return lineas[ids[i]] < lineas[ids[j]]
	})
	elegidas := map[int]bool{}
	n := 0
	for k, id := range ids {
		if k >= maxSospechosas && (puntos[id] < puntos[ids[maxSospechosas-1]] || n >= maxEmpates) {
			break
		}
		elegidas[lineas[id]] = true
		n++
	}
	if len(elegidas) == 0 {
		return porLinea, nil
	}
	// headers of the enclosing loops
	if a != nil && a.archivo != nil {
		var pila []int
		var visitar func(n ast.Node) bool
		visitar = func(n ast.Node) bool {
			if n == nil {
				return false
			}
			switch x := n.(type) {
			case *ast.ForStmt, *ast.RangeStmt:
				cab := a.linea(x.Pos())
				pila = append(pila, cab)
				ast.Inspect(cuerpoBucle(x), visitar)
				pila = pila[:len(pila)-1]
				// the header itself (init, cond, post) is visited by the caller's walk below
				return false
			case ast.Stmt:
				if elegidas[a.linea(x.Pos())] {
					for _, l := range pila {
						elegidas[l] = true
					}
				}
			}
			return true
		}
		ast.Inspect(a.archivo, visitar)
	}
	out := make([]int, 0, len(elegidas))
	for l := range elegidas {
		out = append(out, l)
	}
	sort.Ints(out)
	return porLinea, out
}

func cuerpoBucle(n ast.Node) ast.Node {
	switch x := n.(type) {
	case *ast.ForStmt:
		return x.Body
	case *ast.RangeStmt:
		return x.Body
	}
	return nil
}
