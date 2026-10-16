// Maximo debería devolver el mayor número de la lista, pero compara al revés.
package solucion

func Maximo(xs []int) int {
	m := xs[0]
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}
