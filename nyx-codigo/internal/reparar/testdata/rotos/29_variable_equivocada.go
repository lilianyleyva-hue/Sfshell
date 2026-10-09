// funcion: Mayor
// casos: [3,1,2] -> 3; [1,5,2] -> 5; [7] -> 7; [2,4] -> 4
package solucion

func Mayor(xs []int) int {
	mayor := xs[0]
	ultimo := 0
	for _, x := range xs {
		ultimo = x
		if x > mayor {
			mayor = x
		}
	}
	return ultimo
}
