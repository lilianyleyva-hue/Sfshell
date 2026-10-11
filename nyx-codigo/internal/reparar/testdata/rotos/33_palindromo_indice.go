// funcion: EsPalindromo
// casos: "ana" -> true; "casa" -> false; "" -> true; "aa" -> true; "ab" -> false
package solucion

func EsPalindromo(s string) bool {
	for i := 0; i < len(s)/2; i++ {
		if s[i] != s[len(s)-i] {
			return false
		}
	}
	return true
}
