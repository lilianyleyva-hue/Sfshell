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
// computed with Myers' O(ND) algorithm. Equal inputs give "".
func Diff(a, b string) string {
	if a == b {
		return ""
	}
	la, lb := lineasDiff(a), lineasDiff(b)
	ops := myers(la, lb)
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

// myers returns the edit script from a to b. For '-' ops, b is the index in b where the deletion
// happens; for '+' ops, a is the index in a where the insertion happens.
func myers(a, b []string) []opDiff {
	n, m := len(a), len(b)
	max := n + m
	desp := max + 1
	v := make([]int, 2*max+3)
	var trazas [][]int
	encontrado := false
	for d := 0; d <= max && !encontrado; d++ {
		copia := make([]int, len(v))
		copy(copia, v)
		trazas = append(trazas, copia)
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
				encontrado = true
				break
			}
		}
	}
	// backtrack
	var ops []opDiff
	x, y := n, m
	for d := len(trazas) - 1; d >= 0; d-- {
		vv := trazas[d]
		k := x - y
		var kPrev int
		if k == -d || (k != d && vv[desp+k-1] < vv[desp+k+1]) {
			kPrev = k + 1
		} else {
			kPrev = k - 1
		}
		xPrev := vv[desp+kPrev]
		yPrev := xPrev - kPrev
		for x > xPrev && y > yPrev {
			x--
			y--
			ops = append(ops, opDiff{tipo: ' ', a: x, b: y})
		}
		if d > 0 {
			if x == xPrev {
				y--
				ops = append(ops, opDiff{tipo: '+', a: x, b: y})
			} else {
				x--
				ops = append(ops, opDiff{tipo: '-', a: x, b: y})
			}
		}
	}
	for x > 0 && y > 0 {
		x--
		y--
		ops = append(ops, opDiff{tipo: ' ', a: x, b: y})
	}
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
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
