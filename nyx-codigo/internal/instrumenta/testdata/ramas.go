package solucion

import "strings"

// Clasifica says what kind of number n is.
func Clasifica(n int) string {
	if n > 0 {
		return "positivo"
	} else {
		return "no positivo"
	}
}

func Dia(d int) string {
	nombre := ""
	switch d {
	case 1:
		nombre = "lunes"
	case 2:
		nombre = "martes"
	default:
		nombre = "otro"
	}
	return strings.ToUpper(nombre)
}

func SumaPares(nums []int) int {
	total := 0
	for _, n := range nums {
		if n%2 == 0 {
			total += n
		}
	}
	return total
}
