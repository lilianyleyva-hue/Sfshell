package reparar

import (
	"strconv"
	"strings"
)

// contextoDiff is the number of unchanged lines shown around each change.
const contextoDiff = 2

type opDiff struct {
	tipo byte // ' ', '-', '+'
	a, b int  // line index in a (for ' ' and '-') and in b (for ' ' and '+')
}

// Diff returns a unified diff from a to b ("--- antes" / "+++ después"), with 2 lines of context,
// computed with Myers' O(ND) algorithm. Inputs with the same lines give "" (also when they only differ
// in the final newline). A very large difference (more than 1000 changed lines in the middle) is shown
// as the old lines removed and the new ones added.
func Diff(a, b string) string {
	if a == b {
		return ""
	}
	la, lb := lineasDiff(a), lineasDiff(b)
	ops := myers(la, lb)
	cambia := false
	for _, o := range ops {
		if o.tipo != ' ' {
			cambia = true
			break
		}
	}
	if !cambia {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("--- antes\n+++ después\n")
	// group the operations into hunks
	n := len(ops)
	for i := 0; i < n; {
		// next change
		for i < n && ops[i].tipo == ' ' {
			i++
		}
		if i >= n {
			break
		}
		ini := i - contextoDiff
		if ini < 0 {
			ini = 0
		}
		fin := i
		for fin < n {
			if ops[fin].tipo != ' ' {
				fin++
				continue
			}
			// run of equal lines: does another change come within 2*context?
			j := fin
			for j < n && ops[j].tipo == ' ' {
				j++
			}
			if j < n && j-fin <= 2*contextoDiff {
				fin = j
				continue
			}
			fin += contextoDiff
			if fin > n {
				fin = n
			}
			break
		}
		escribirTrozo(&sb, ops[ini:fin], la, lb)
		i = fin
	}
	return sb.String()
}

func lineasDiff(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func escribirTrozo(sb *strings.Builder, ops []opDiff, la, lb []string) {
	inA, inB, nA, nB := -1, -1, 0, 0
	for _, o := range ops {
		switch o.tipo {
		case ' ':
			if inA < 0 {
				inA = o.a
			}
			if inB < 0 {
				inB = o.b
			}
			nA++
			nB++
		case '-':
			if inA < 0 {
				inA = o.a
			}
			nA++
		case '+':
			if inB < 0 {
				inB = o.b
			}
			nB++
		}
	}
	// a side with no lines starts after the previous line (0-based index of the line before)
	if nA == 0 {
		inA = inicioVacio(ops, true)
	} else {
		inA++
	}
	if nB == 0 {
		inB = inicioVacio(ops, false)
	} else {
		inB++
	}
	sb.WriteString("@@ -" + rango(inA, nA) + " +" + rango(inB, nB) + " @@\n")
	for _, o := range ops {
		switch o.tipo {
		case ' ':
			sb.WriteString(" " + la[o.a] + "\n")
		case '-':
			sb.WriteString("-" + la[o.a] + "\n")
		case '+':
			sb.WriteString("+" + lb[o.b] + "\n")
		}
	}
}

// inicioVacio: for a hunk with no lines on one side, the line number before the insertion point.
func inicioVacio(ops []opDiff, ladoA bool) int {
	for _, o := range ops {
		if ladoA && o.tipo == '+' {
			return o.a
		}
		if !ladoA && o.tipo == '-' {
			return o.b
		}
	}
	return 0
}

func rango(ini, n int) string {
	if n == 1 {
		return strconv.Itoa(ini)
	}
	return strconv.Itoa(ini) + "," + strconv.Itoa(n)
}

// maxDDiff bounds the edit distance Myers' search looks for. Beyond it (every line of a big paste
// differs, e.g. after reindenting or CRLF → LF) the differing middle is shown as removed and re-added:
// still a valid diff, computed in linear memory instead of O((n+m)·D).
const maxDDiff = 1000

// myers returns the edit script from a to b. For '-' ops, b is the index in b where the deletion
// happens; for '+' ops, a is the index in a where the insertion happens. The common prefix and suffix
// are matched first; the middle gets Myers' O(ND) search with the traces trimmed to the diagonals in
// use (O(D²) memory), or a whole replacement when D > maxDDiff.
func myers(a, b []string) []opDiff {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	ops := make([]opDiff, 0, len(a)+len(b)-pre-suf)
	for i := 0; i < pre; i++ {
		ops = append(ops, opDiff{tipo: ' ', a: i, b: i})
	}
	ops = append(ops, myersMedio(a[pre:len(a)-suf], b[pre:len(b)-suf], pre, pre)...)
	for i := 0; i < suf; i++ {
		ops = append(ops, opDiff{tipo: ' ', a: len(a) - suf + i, b: len(b) - suf + i})
	}
	// show deletions before insertions inside each change block
	for i := 0; i < len(ops); {
		if ops[i].tipo == ' ' {
			i++
			continue
		}
		j := i
		for j < len(ops) && ops[j].tipo != ' ' {
			j++
		}
		bloque := append([]opDiff(nil), ops[i:j]...)
		k := i
		for _, o := range bloque {
			if o.tipo == '-' {
				ops[k] = o
				k++
			}
		}
		for _, o := range bloque {
			if o.tipo == '+' {
				ops[k] = o
				k++
			}
		}
		i = j
	}
	return ops
}

// myersMedio is Myers' search on a and b (whose first lines are a[0] = line offA, b[0] = line offB).
func myersMedio(a, b []string, offA, offB int) []opDiff {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	reemplazo := func() []opDiff {
		ops := make([]opDiff, 0, n+m)
		for i := 0; i < n; i++ {
			ops = append(ops, opDiff{tipo: '-', a: offA + i, b: offB})
		}
		for j := 0; j < m; j++ {
			ops = append(ops, opDiff{tipo: '+', a: offA + n, b: offB + j})
		}
		return ops
	}
	if n == 0 || m == 0 {
		return reemplazo()
	}
	limite := min(n+m, maxDDiff)
	desp := limite + 1
	v := make([]int, 2*limite+3)
	// trazas[d] holds v[k] for k in [-d, d] after step d
	var trazas [][]int
	fin := -1
	for d := 0; d <= limite && fin < 0; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[desp+k-1] < v[desp+k+1]) {
				x = v[desp+k+1]
			} else {
				x = v[desp+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[desp+k] = x
			if x >= n && y >= m {
				fin = d
				break
			}
		}
		if fin < 0 {
			trazas = append(trazas, append([]int(nil), v[desp-d:desp+d+1]...))
		}
	}
	if fin < 0 {
		return reemplazo()
	}
	// backtrack from (n, m)
	var ops []opDiff
	x, y := n, m
	for d := fin; d > 0; d-- {
		prev := trazas[d-1] // v after step d-1, k in [-(d-1), d-1]
		get := func(k int) int { return prev[k+d-1] }
		k := x - y
		var kPrev int
		if k == -d || (k != d && get(k-1) < get(k+1)) {
			kPrev = k + 1
		} else {
			kPrev = k - 1
		}
		xPrev := get(kPrev)
		yPrev := xPrev - kPrev
		for x > xPrev && y > yPrev {
			x--
			y--
			ops = append(ops, opDiff{tipo: ' ', a: offA + x, b: offB + y})
		}
		if x == xPrev {
			y--
			ops = append(ops, opDiff{tipo: '+', a: offA + x, b: offB + y})
		} else {
			x--
			ops = append(ops, opDiff{tipo: '-', a: offA + x, b: offB + y})
		}
	}
	for x > 0 && y > 0 {
		x--
		y--
		ops = append(ops, opDiff{tipo: ' ', a: offA + x, b: offB + y})
	}
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	return ops
}
