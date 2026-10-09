// funcion: Frecuencias
// casos: "aba" -> {"a": 2, "b": 1}; "" -> {}; "zz" -> {"z": 2}
package solucion

func Frecuencias(s string) map[string]int {
	var m map[string]int
	for _, r := range s {
		m[string(r)]++
	}
	return m
}
