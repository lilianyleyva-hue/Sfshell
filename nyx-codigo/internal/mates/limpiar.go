package mates

import (
	"regexp"
	"strings"
)

var reInicioCalculo = regexp.MustCompile(`^(?i)\s*(¿\s*)?(cu[aá]nto\s+(es|da|vale|son|sale)|calcula(me)?|calcular|dime\s+cu[aá]nto\s+(es|da)|cu[aá]l\s+es\s+el\s+resultado\s+de|el\s+resultado\s+de|resultado\s+de|opera|simplifica|eval[uú]a|haz)\s*:?\s*`)

// limpiarCalculo removes the question words around a calculation: "¿cuánto es 3/4+1/6?" → "3/4+1/6".
func limpiarCalculo(s string) string {
	s = strings.TrimSpace(s)
	for {
		t := reInicioCalculo.ReplaceAllString(s, "")
		if t == s {
			break
		}
		s = t
	}
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "?.¿ "))
	s = strings.TrimSuffix(s, "=")
	return strings.TrimSpace(s)
}
