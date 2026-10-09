// funcion: SumaPares
// casos: [1,2,3,4] -> 6; [] -> 0; [5,7] -> 0; [2] -> 2
package solucion

func SumaPares(nums []int) int {
	total := 1
	for _, n := range nums {
		if n%2 == 0 {
			total += n
		}
	}
	return total
}
