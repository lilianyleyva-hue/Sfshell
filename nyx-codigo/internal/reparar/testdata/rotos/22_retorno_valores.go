// quiero: compila
package solucion

import "errors"

func Dividir(a, b int) (int, error) {
	if b == 0 {
		return errors.New("división por cero")
	}
	return a / b
}
