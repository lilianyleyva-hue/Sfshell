// funcion: SumaCuadrados
// casos: [1,2,3] -> 14; [] -> 0; [-2] -> 4; [5] -> 25
package solucion

func cuadrado(x int) int {
	return x + x
}

func SumaCuadrados(xs []int) int {
	total := 0
	for _, x := range xs {
		total += cuadrado(x)
	}
	return total
}
