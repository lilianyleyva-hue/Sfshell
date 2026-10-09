// quiero: compila
package solucion

func Pares(xs []int) []int {
	var out []int
	for _, x := range xs {
		if x%2 == 0 {
			out.push(x)
		}
	}
	return out
}
