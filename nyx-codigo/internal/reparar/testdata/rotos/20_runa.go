// quiero: compila
package solucion

func ContarA(s string) int {
	n := 0
	for _, r := range s {
		if r == "a" {
			n++
		}
	}
	return n
}
