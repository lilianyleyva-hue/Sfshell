// funcion: Maximo
// casos: [3,1,2] -> 3; [5] -> 5; [-1,-7] -> -1; [2,9,4] -> 9
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
