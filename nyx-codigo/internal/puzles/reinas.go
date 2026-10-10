package puzles

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"nyxcodigo/internal/nucleo"
)

// solucionesReinas is OEIS A000170 for n = 0..17, used to cross-check the counts.
var solucionesReinas = []int{1, 1, 0, 0, 2, 10, 4, 40, 92, 352, 724, 2680, 14200, 73712, 365596, 2279184, 14772512, 95815104}

const (
	maxReinasBitmask = 30
	maxReinasContar  = 17
)

// Reinas places n queens. With todas it counts every solution (bitmask backtracking, n ≤ 17,
// cross-checked with the known table and, for n ≤ 8, with the generic CSP solver); otherwise it
// finds one (bitmask backtracking for n ≤ 30, min-conflicts for larger n). una[i] is the column of
// the queen in row i (0-based). Every placement is re-checked.
func Reinas(ctx context.Context, n int, todas bool) (cuenta int, una []int, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if n <= 0 {
		return 0, nil, fmt.Errorf("puzles: el tablero necesita al menos una casilla: %w", nucleo.ErrNoEntiendo)
	}
	if n > 1_000_000 {
		return 0, nil, fmt.Errorf("puzles: %d reinas son demasiadas: %w", n, nucleo.ErrNoSoportado)
	}
	if todas && n > maxReinasContar {
		return 0, nil, fmt.Errorf("puzles: contar todas las soluciones con %d reinas tardaría demasiado (llego hasta %d): %w", n, maxReinasContar, nucleo.ErrNoSoportado)
	}
	limite := time.Now().Add(TiempoCSP)
	if n <= maxReinasBitmask {
		lim := limite
		if !todas && n > 20 {
			lim = time.Now().Add(time.Second)
		}
		cuenta, una, err = reinasBitmask(ctx, n, todas, lim)
		if err != nil && !todas && n > 20 {
			// very slow first solutions: fall back to min-conflicts
			una, err = reinasMinConflictos(ctx, n, limite.Add(TiempoCSP))
			cuenta = 0
			if una != nil {
				cuenta = 1
			}
		}
	} else {
		una, err = reinasMinConflictos(ctx, n, limite)
		if una != nil {
			cuenta = 1
		}
	}
	if err != nil {
		return 0, nil, err
	}
	if una != nil {
		if e := ComprobarReinas(una); e != nil {
			return 0, nil, fmt.Errorf("puzles: la colocación no es válida (error interno): %v", e)
		}
	}
	if todas {
		if cuenta != solucionesReinas[n] {
			return 0, nil, fmt.Errorf("puzles: conté %d soluciones y deberían ser %d (error interno)", cuenta, solucionesReinas[n])
		}
		if n <= 8 {
			if c, err := reinasCSP(ctx, n); err != nil {
				return 0, nil, err
			} else if c != cuenta {
				return 0, nil, fmt.Errorf("puzles: dos métodos dan %d y %d soluciones (error interno)", cuenta, c)
			}
		}
	}
	return cuenta, una, nil
}

// ComprobarReinas checks that no two queens attack each other.
func ComprobarReinas(cols []int) error {
	n := len(cols)
	usadas := make([]bool, n)
	d1 := map[int]bool{}
	d2 := map[int]bool{}
	for f, c := range cols {
		if c < 0 || c >= n {
			return fmt.Errorf("la reina de la fila %d está fuera del tablero", f+1)
		}
		if usadas[c] || d1[f-c] || d2[f+c] {
			return fmt.Errorf("la reina de la fila %d está atacada", f+1)
		}
		usadas[c], d1[f-c], d2[f+c] = true, true, true
	}
	return nil
}

func reinasBitmask(ctx context.Context, n int, todas bool, limite time.Time) (int, []int, error) {
	completo := uint64(1)<<uint(n) - 1
	cols := make([]int, n)
	var una []int
	cuenta, nodos := 0, 0
	var errBusca error
	var rec func(fila int, ocup, diag1, diag2 uint64) bool
	rec = func(fila int, ocup, diag1, diag2 uint64) bool {
		if fila == n {
			cuenta++
			if una == nil {
				una = append([]int(nil), cols...)
			}
			return todas
		}
		libres := completo &^ (ocup | diag1 | diag2)
		for libres != 0 {
			bit := libres & -libres
			libres &^= bit
			nodos++
			if nodos&0xFFFF == 0 && (ctx.Err() != nil || time.Now().After(limite)) {
				errBusca = fmt.Errorf("puzles: sin tiempo con %d reinas: %w", n, nucleo.ErrSinTiempo)
				return false
			}
			c := 0
			for b := bit; b > 1; b >>= 1 {
				c++
			}
			cols[fila] = c
			if !rec(fila+1, ocup|bit, (diag1|bit)<<1&completo, (diag2|bit)>>1) {
				return false
			}
		}
		return true
	}
	rec(0, 0, 0, 0)
	if errBusca != nil {
		return 0, nil, errBusca
	}
	return cuenta, una, nil
}

// reinasMinConflictos: random start, repeatedly move a conflicted queen to its least-attacked column.
func reinasMinConflictos(ctx context.Context, n int, limite time.Time) ([]int, error) {
	rng := rand.New(rand.NewSource(int64(n)*7919 + 1))
	if n == 2 || n == 3 {
		return nil, fmt.Errorf("puzles: con %d reinas no hay solución: %w", n, nucleo.ErrNoEntiendo)
	}
	for intento := 0; ; intento++ {
		cols := rng.Perm(n)
		porCol := make([]int, n)
		d1 := make([]int, 2*n)
		d2 := make([]int, 2*n)
		for f, c := range cols {
			porCol[c]++
			d1[f-c+n]++
			d2[f+c]++
		}
		ataques := func(f, c int) int { return porCol[c] + d1[f-c+n] + d2[f+c] }
		pasos := 0
		for pasos < 50*n+1000 {
			pasos++
			if pasos&1023 == 0 && (ctx.Err() != nil || time.Now().After(limite)) {
				return nil, fmt.Errorf("puzles: sin tiempo con %d reinas: %w", n, nucleo.ErrSinTiempo)
			}
			// pick a random conflicted row
			var conflictivas []int
			for f, c := range cols {
				if ataques(f, c) > 3 {
					conflictivas = append(conflictivas, f)
					if len(conflictivas) > 64 {
						break
					}
				}
			}
			if len(conflictivas) == 0 {
				return cols, nil
			}
			f := conflictivas[rng.Intn(len(conflictivas))]
			c := cols[f]
			porCol[c]--
			d1[f-c+n]--
			d2[f+c]--
			mejor, mejorA := c, 1<<30
			empate := 0
			for x := 0; x < n; x++ {
				a := ataques(f, x)
				if a < mejorA {
					mejor, mejorA, empate = x, a, 1
				} else if a == mejorA {
					empate++
					if rng.Intn(empate) == 0 {
						mejor = x
					}
				}
			}
			cols[f] = mejor
			porCol[mejor]++
			d1[f-mejor+n]++
			d2[f+mejor]++
		}
		if intento > 1000 {
			return nil, errors.New("puzles: min-conflicts no encontró solución")
		}
	}
}

// reinasCSP counts the solutions with the generic CSP solver (independent cross-check).
func reinasCSP(ctx context.Context, n int) (int, error) {
	p := &Problema{}
	for i := 0; i < n; i++ {
		p.Nombres = append(p.Nombres, fmt.Sprintf("fila %d", i+1))
		p.Dom = append(p.Dom, DominioRango(0, n-1))
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			d := j - i
			p.Restr = append(p.Restr, Restriccion{Alcance: []int{i, j}, Tipo: Binaria, Rel: func(v []int) bool {
				return v[0] != v[1] && v[0]-v[1] != d && v[1]-v[0] != d
			}})
		}
	}
	sols, _, err := Resolver(ctx, p, 1_000_000, nil)
	return len(sols), err
}
