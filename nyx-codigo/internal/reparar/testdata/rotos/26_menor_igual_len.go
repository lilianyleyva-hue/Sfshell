// funcion: SumaTodos
// casos: [1,2,3] -> 6; [] -> 0; [5] -> 5; [-1,4] -> 3
package solucion

func SumaTodos(xs []int) int {
	total := 0
	for i := 0; i <= len(xs); i++ {
		total += xs[i]
	}
	return total
}
