package puzles

import (
	"context"
	"errors"
	"fmt"

	"nyxcodigo/internal/nucleo"
)

// NombresColores are the names used for colours 0, 1, 2…
var NombresColores = []string{"rojo", "verde", "azul", "amarillo", "morado", "naranja", "rosa", "gris", "marrón", "negro"}

// Colorear colours the graph given by its edges with k colours so that neighbours differ (N2). The
// result maps every node to a colour 0..k−1 and is re-checked on every edge.
func Colorear(ctx context.Context, aristas [][2]string, k int) (map[string]int, error) {
	if k <= 0 || k > 64 {
		return nil, fmt.Errorf("puzles: el número de colores debe estar entre 1 y 64: %w", nucleo.ErrNoEntiendo)
	}
	indice := map[string]int{}
	p := &Problema{}
	nodo := func(s string) int {
		if i, ok := indice[s]; ok {
			return i
		}
		indice[s] = len(p.Nombres)
		p.Nombres = append(p.Nombres, s)
		p.Dom = append(p.Dom, DominioRango(0, k-1))
		return len(p.Nombres) - 1
	}
	for _, a := range aristas {
		x, y := nodo(a[0]), nodo(a[1])
		if x == y {
			return nil, fmt.Errorf("puzles: «%s» no puede ser vecino de sí mismo", a[0])
		}
		p.Restr = append(p.Restr, Restriccion{Alcance: []int{x, y}, Tipo: Binaria, Rel: func(v []int) bool { return v[0] != v[1] }})
	}
	if len(p.Nombres) == 0 {
		return nil, fmt.Errorf("puzles: no hay regiones que colorear: %w", nucleo.ErrNoEntiendo)
	}
	// symmetry breaking: the first node takes colour 0
	p.Dom[0] = DominioRango(0, 0)
	sols, _, err := Resolver(ctx, p, 1, nil)
	if err != nil && len(sols) == 0 {
		return nil, err
	}
	if len(sols) == 0 {
		return nil, fmt.Errorf("puzles: no se puede colorear con %d colores: %w", k, ErrSinColoreo)
	}
	out := map[string]int{}
	for nombre, i := range indice {
		out[nombre] = sols[0][i]
	}
	for _, a := range aristas {
		if out[a[0]] == out[a[1]] {
			return nil, fmt.Errorf("puzles: %s y %s quedaron del mismo color (error interno)", a[0], a[1])
		}
	}
	return out, nil
}

// ErrSinColoreo marks a graph that cannot be coloured with the given number of colours.
var ErrSinColoreo = errors.New("no hay coloreo posible")

// NumeroCromatico finds the fewest colours that work (trying 1, 2, … up to max).
func NumeroCromatico(ctx context.Context, aristas [][2]string, max int) (int, map[string]int, error) {
	for k := 1; k <= max; k++ {
		m, err := Colorear(ctx, aristas, k)
		if err == nil {
			return k, m, nil
		}
		if !esSinColoreo(err) {
			return 0, nil, err
		}
	}
	return 0, nil, fmt.Errorf("puzles: hacen falta más de %d colores: %w", max, ErrSinColoreo)
}

func esSinColoreo(err error) bool { return errors.Is(err, ErrSinColoreo) }
