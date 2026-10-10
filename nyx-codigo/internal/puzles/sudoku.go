package puzles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// LeerSudoku reads 9 lines of 9 cells (digits; 0 . _ * for blanks; spaces and | + - separators
// ignored) or one run of 81 cells.
func LeerSudoku(texto string) ([9][9]int, error) {
	var t [9][9]int
	celda := func(r rune) (int, bool) {
		switch {
		case r >= '1' && r <= '9':
			return int(r - '0'), true
		case r == '0' || r == '.' || r == '_' || r == '*':
			return 0, true
		}
		return 0, false
	}
	// line by line: a grid line has exactly 9 cells and nothing else but separators
	var filas [][]int
	for _, linea := range strings.Split(texto, "\n") {
		var fila []int
		ok := true
		for _, r := range linea {
			if v, es := celda(r); es {
				fila = append(fila, v)
				continue
			}
			if r == ' ' || r == '\t' || r == '|' || r == '+' || r == '-' || r == '\r' || r == '│' || r == '─' || r == '┼' {
				continue
			}
			ok = false
			break
		}
		if ok && len(fila) == 9 {
			filas = append(filas, fila)
		} else if ok && len(fila) == 81 && len(filas) == 0 {
			for i := 0; i < 9; i++ {
				filas = append(filas, fila[i*9:i*9+9])
			}
		}
	}
	if len(filas) != 9 {
		// one run of 81 cells anywhere in the text
		for _, campo := range strings.Fields(texto) {
			var vs []int
			for _, r := range campo {
				if v, es := celda(r); es {
					vs = append(vs, v)
				} else {
					vs = nil
					break
				}
			}
			if len(vs) == 81 {
				filas = nil
				for i := 0; i < 9; i++ {
					filas = append(filas, vs[i*9:i*9+9])
				}
				break
			}
		}
	}
	if len(filas) != 9 {
		return t, fmt.Errorf("puzles: no encuentro las 9 filas de 9 casillas del sudoku (encontré %d): %w", len(filas), nucleo.ErrNoEntiendo)
	}
	for i := 0; i < 9; i++ {
		copy(t[i][:], filas[i])
	}
	return t, nil
}

// grupos are the 27 units: rows, columns, boxes (cells as r*9+c).
var grupos = func() [27][9]int {
	var g [27][9]int
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			g[i][j] = i*9 + j
			g[9+i][j] = j*9 + i
			g[18+i][j] = (i/3*3+j/3)*9 + i%3*3 + j%3
		}
	}
	return g
}()

func nombreGrupo(g int) string {
	switch {
	case g < 9:
		return fmt.Sprintf("la fila %d", g+1)
	case g < 18:
		return fmt.Sprintf("la columna %d", g-8)
	}
	return fmt.Sprintf("el bloque %d", g-17)
}

// ValidarSudoku checks that no unit repeats a digit (blanks allowed) and, if completo, that every
// cell is filled with 1..9 so that all 27 units hold 1..9.
func ValidarSudoku(t [9][9]int, completo bool) error {
	for g, cs := range grupos {
		visto := [10]bool{}
		for _, c := range cs {
			v := t[c/9][c%9]
			if v < 0 || v > 9 {
				return fmt.Errorf("la casilla (%d,%d) tiene %d", c/9+1, c%9+1, v)
			}
			if v == 0 {
				if completo {
					return fmt.Errorf("la casilla (%d,%d) está vacía", c/9+1, c%9+1)
				}
				continue
			}
			if visto[v] {
				return fmt.Errorf("el %d está repetido en %s", v, nombreGrupo(g))
			}
			visto[v] = true
		}
	}
	return nil
}

// PasoSudoku is one human-technique step.
type PasoSudoku struct {
	Tecnica string // "único candidato", "único lugar", "par apuntador"
	Texto   string
}

const maxPasosDetallados = 40

// humano applies naked singles, hidden singles and pointing pairs until stuck. It returns the
// grid, the candidate sets and the steps taken.
func humano(t [9][9]int) ([9][9]int, [81]uint16, []PasoSudoku, error) {
	var cand [81]uint16
	for c := 0; c < 81; c++ {
		if t[c/9][c%9] != 0 {
			cand[c] = 1 << uint(t[c/9][c%9])
		} else {
			cand[c] = 0x3FE
		}
	}
	vecinos := func(c int) []int {
		var out []int
		for _, g := range grupos {
			for _, x := range g {
				if x == c {
					for _, y := range g {
						if y != c {
							out = append(out, y)
						}
					}
					break
				}
			}
		}
		return out
	}
	var pasos []PasoSudoku
	colocar := func(c, v int) {
		t[c/9][c%9] = v
		cand[c] = 1 << uint(v)
		for _, y := range vecinos(c) {
			cand[y] &^= 1 << uint(v)
		}
	}
	for c := 0; c < 81; c++ {
		if v := t[c/9][c%9]; v != 0 {
			for _, y := range vecinos(c) {
				cand[y] &^= 1 << uint(v)
			}
		}
	}
	for {
		avance := false
		for c := 0; c < 81; c++ {
			if t[c/9][c%9] == 0 && cand[c] == 0 {
				return t, cand, pasos, errors.New("una casilla se queda sin candidatos")
			}
		}
		// naked singles
		for c := 0; c < 81; c++ {
			if t[c/9][c%9] != 0 || popcount16(cand[c]) != 1 {
				continue
			}
			v := unico16(cand[c])
			pasos = append(pasos, PasoSudoku{Tecnica: "único candidato", Texto: textoUnicoCandidato(t, c, v)})
			colocar(c, v)
			avance = true
		}
		if avance {
			continue
		}
		// hidden singles
		for g, cs := range grupos {
			for v := 1; v <= 9; v++ {
				pos, n, ya := -1, 0, false
				for _, c := range cs {
					if t[c/9][c%9] == v {
						ya = true
						break
					}
					if t[c/9][c%9] == 0 && cand[c]&(1<<uint(v)) != 0 {
						pos, n = c, n+1
					}
				}
				if ya || n != 1 {
					continue
				}
				pasos = append(pasos, PasoSudoku{Tecnica: "único lugar", Texto: fmt.Sprintf("en %s, el %d solo cabe en la casilla (%d,%d)", nombreGrupo(g), v, pos/9+1, pos%9+1)})
				colocar(pos, v)
				avance = true
			}
		}
		if avance {
			continue
		}
		// pointing pairs: a digit confined to one row/column inside a box
		for b := 0; b < 9; b++ {
			caja := grupos[18+b]
			for v := 1; v <= 9; v++ {
				var celdas []int
				for _, c := range caja {
					if t[c/9][c%9] == 0 && cand[c]&(1<<uint(v)) != 0 {
						celdas = append(celdas, c)
					}
				}
				if len(celdas) < 2 {
					continue
				}
				mismaFila, mismaCol := true, true
				for _, c := range celdas[1:] {
					mismaFila = mismaFila && c/9 == celdas[0]/9
					mismaCol = mismaCol && c%9 == celdas[0]%9
				}
				var linea []int
				var nombre string
				if mismaFila {
					linea, nombre = grupos[celdas[0]/9][:], fmt.Sprintf("la fila %d", celdas[0]/9+1)
				} else if mismaCol {
					linea, nombre = grupos[9+celdas[0]%9][:], fmt.Sprintf("la columna %d", celdas[0]%9+1)
				} else {
					continue
				}
				quitados := 0
				for _, c := range linea {
					enCaja := (c/9)/3*3+(c%9)/3 == b
					if !enCaja && t[c/9][c%9] == 0 && cand[c]&(1<<uint(v)) != 0 {
						cand[c] &^= 1 << uint(v)
						quitados++
					}
				}
				if quitados > 0 {
					pasos = append(pasos, PasoSudoku{Tecnica: "par apuntador", Texto: fmt.Sprintf("en el bloque %d, el %d solo puede ir en %s: lo quito del resto de %s (%d candidatos)", b+1, v, nombre, nombre, quitados)})
					avance = true
				}
			}
		}
		if !avance {
			return t, cand, pasos, nil
		}
	}
}

func textoUnicoCandidato(t [9][9]int, c, v int) string {
	r, col := c/9, c%9
	var fila, columna []string
	for j := 0; j < 9; j++ {
		if x := t[r][j]; x != 0 {
			fila = append(fila, fmt.Sprint(x))
		}
	}
	for i := 0; i < 9; i++ {
		if x := t[i][col]; x != 0 {
			columna = append(columna, fmt.Sprint(x))
		}
	}
	texto := fmt.Sprintf("la casilla (%d,%d) solo puede ser %d", r+1, col+1, v)
	partes := []string{}
	if len(fila) > 0 {
		partes = append(partes, "su fila ya tiene "+strings.Join(fila, ","))
	}
	if len(columna) > 0 {
		partes = append(partes, "su columna ya tiene "+strings.Join(columna, ","))
	}
	if len(partes) > 0 {
		texto += ": " + strings.Join(partes, "; ") + " y su bloque descarta el resto"
	}
	return texto
}

func popcount16(x uint16) int {
	n := 0
	for ; x != 0; x &= x - 1 {
		n++
	}
	return n
}

func unico16(x uint16) int {
	for v := 1; v <= 9; v++ {
		if x == 1<<uint(v) {
			return v
		}
	}
	return 0
}

// ResultadoSudoku carries the extra information of a sudoku solve (additive helper).
type ResultadoSudoku struct {
	Pasos        []PasoSudoku
	PorTecnicas  int // cells filled by human techniques
	SoloTecnicas bool
	Soluciones   int // 1 or 2 (2 means "more than one")
}

// Sudoku solves t: first human techniques (logged as Spanish steps under n, at most 40 detailed),
// then the CSP with maxSol = 2 for the uniqueness check. The solution is re-validated on all 27
// units and must keep every given.
func Sudoku(ctx context.Context, t [9][9]int, n *nucleo.Nodo) (sol [9][9]int, unica bool, est Estadisticas, err error) {
	sol, unica, est, _, err = SudokuDetallado(ctx, t, n)
	return
}

// SudokuDetallado is Sudoku plus the human steps.
func SudokuDetallado(ctx context.Context, t [9][9]int, n *nucleo.Nodo) ([9][9]int, bool, Estadisticas, ResultadoSudoku, error) {
	var res ResultadoSudoku
	if err := ValidarSudoku(t, false); err != nil {
		return t, false, Estadisticas{}, res, fmt.Errorf("puzles: el sudoku no es válido: %v: %w", err, nucleo.ErrNoEntiendo)
	}
	dados := 0
	for c := 0; c < 81; c++ {
		if t[c/9][c%9] != 0 {
			dados++
		}
	}
	h := n.Sub(nucleo.PasoIntento, "Técnicas humanas (único candidato, único lugar, par apuntador)")
	parcial, cand, pasos, errH := humano(t)
	res.Pasos = pasos
	for i, p := range pasos {
		if i < maxPasosDetallados {
			h.Nota("%s", p.Texto)
		}
	}
	if len(pasos) > maxPasosDetallados {
		h.Nota("… y %d pasos más del mismo tipo", len(pasos)-maxPasosDetallados)
	}
	llenas := 0
	for c := 0; c < 81; c++ {
		if parcial[c/9][c%9] != 0 {
			llenas++
		}
	}
	res.PorTecnicas = llenas - dados
	res.SoloTecnicas = llenas == 81 && errH == nil
	if errH != nil {
		h.Mal("Las técnicas llegan a una contradicción: el sudoku no tiene solución")
		return t, false, Estadisticas{}, res, fmt.Errorf("puzles: el sudoku no tiene solución: %w", nucleo.ErrNoEntiendo)
	}
	if res.SoloTecnicas {
		h.Bien("Resuelto solo con técnicas humanas (%d casillas)", res.PorTecnicas)
	} else if res.PorTecnicas == 0 {
		h.Info("Las técnicas sencillas no colocan ninguna casilla; paso a buscar")
	} else {
		h.Info("Con técnicas relleno %d casillas; el resto lo busco", res.PorTecnicas)
	}
	// CSP: 81 variables with the candidate domains, 27 Distintos
	p := &Problema{}
	for c := 0; c < 81; c++ {
		p.Nombres = append(p.Nombres, fmt.Sprintf("(%d,%d)", c/9+1, c%9+1))
		p.Dom = append(p.Dom, Dominio(cand[c]))
	}
	for _, g := range grupos {
		p.Restr = append(p.Restr, Restriccion{Alcance: append([]int(nil), g[:]...), Tipo: Distintos})
	}
	b := n.Sub(nucleo.PasoIntento, "Búsqueda con restricciones y comprobación de unicidad")
	sols, est, err := Resolver(ctx, p, 2, b)
	if err != nil && len(sols) == 0 {
		b.Mal("No terminé la búsqueda")
		return t, false, est, res, err
	}
	if len(sols) == 0 {
		b.Mal("No tiene solución")
		return t, false, est, res, fmt.Errorf("puzles: el sudoku no tiene solución: %w", nucleo.ErrNoEntiendo)
	}
	var sol [9][9]int
	for c := 0; c < 81; c++ {
		sol[c/9][c%9] = sols[0][c]
	}
	res.Soluciones = len(sols)
	unica := len(sols) == 1 && err == nil
	if errV := ValidarSudoku(sol, true); errV != nil {
		return t, false, est, res, fmt.Errorf("puzles: la solución no es válida (error interno): %v", errV)
	}
	for c := 0; c < 81; c++ {
		if t[c/9][c%9] != 0 && t[c/9][c%9] != sol[c/9][c%9] {
			return t, false, est, res, errors.New("puzles: la solución cambia una casilla dada (error interno)")
		}
	}
	if unica {
		b.Bien("Probé %d caminos (%d retrocesos): la solución es única", est.Nodos, est.Retrocesos)
	} else {
		b.Info("Probé %d caminos: hay más de una solución", est.Nodos)
	}
	return sol, unica, est, res, err
}
