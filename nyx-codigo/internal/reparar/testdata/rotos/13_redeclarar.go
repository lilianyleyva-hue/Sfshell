// quiero: compila
package solucion

func Contar(xs []int) int {
	c := 0
	for _, x := range xs {
		if x > 0 {
			c := c + 1
		}
	}
	return c
}
