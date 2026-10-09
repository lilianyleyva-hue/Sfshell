// quiero: compila
package solucion

type Cuenta struct {
	Saldo int
}

func Ingresar(m map[string]Cuenta, quien string, cantidad int) {
	m[quien].Saldo += cantidad
}
