// quiero: compila
package solucion

func Valido(xs []int, activo bool) bool {
	if xs == None {
		return False
	}
	return activo and len(xs) > 0
}
