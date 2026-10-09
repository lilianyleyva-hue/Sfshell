// quiero: compila
package solucion

func Promedio(xs []int) float64 {
	suma := 0
	for _, x := range xs {
		suma += x
	}
	var media float64
	media = suma / len(xs)
	return media
}
