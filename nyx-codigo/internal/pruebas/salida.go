package pruebas

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

var reNumero = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`)

// CompararSalida compares a program's stdout with the expected text of c, after turning "\r\n" into
// "\n" on both sides:
//
//	exacto    byte equality
//	lineas    trailing spaces of each line and the final newline(s) are ignored (also the default)
//	contiene  the expected text appears in the output
//	numeros   the sequences of numbers printed are equal: integers exactly, floats within 1e-9
func CompararSalida(obtenida string, c nucleo.CasoPrograma) bool {
	o := strings.ReplaceAll(obtenida, "\r\n", "\n")
	e := strings.ReplaceAll(c.Esperado, "\r\n", "\n")
	switch c.Comparar {
	case "exacto":
		return o == e
	case "contiene":
		return strings.Contains(o, e)
	case "numeros":
		return numerosIguales(reNumero.FindAllString(o, -1), reNumero.FindAllString(e, -1))
	default: // "lineas" and anything unknown
		return normalizarLineas(o) == normalizarLineas(e)
	}
}

func normalizarLineas(s string) string {
	lineas := strings.Split(s, "\n")
	for i, l := range lineas {
		lineas[i] = strings.TrimRight(l, " \t")
	}
	for len(lineas) > 0 && lineas[len(lineas)-1] == "" {
		lineas = lineas[:len(lineas)-1]
	}
	return strings.Join(lineas, "\n")
}

func numerosIguales(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !numeroIgual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// numeroIgual compares two numbers as printed. Two integers are compared exactly, digit by digit (so
// 1000000000 ≠ 1000000001 and integers beyond 2^53 keep their precision); when either one has a decimal
// point or an exponent, both are compared as floats within 1e-9 (relative 1e-12 for huge magnitudes).
func numeroIgual(x, y string) bool {
	if esEnteroTexto(x) && esEnteroTexto(y) {
		return normalizarEntero(x) == normalizarEntero(y)
	}
	f, err1 := strconv.ParseFloat(x, 64)
	g, err2 := strconv.ParseFloat(y, 64)
	if err1 != nil || err2 != nil {
		// out of range for a float64 (1e999): only the same text is equal
		return x == y
	}
	if f == g {
		return true
	}
	tol := math.Max(1e-9, 1e-12*math.Max(math.Abs(f), math.Abs(g)))
	return math.Abs(f-g) <= tol
}

func esEnteroTexto(s string) bool {
	return !strings.ContainsAny(s, ".eE")
}

// normalizarEntero drops leading zeros and the sign of zero: "-007" → "-7", "-0" → "0".
func normalizarEntero(s string) string {
	neg := strings.HasPrefix(s, "-")
	d := strings.TrimLeft(strings.TrimPrefix(s, "-"), "0")
	if d == "" {
		return "0"
	}
	if neg {
		return "-" + d
	}
	return d
}
