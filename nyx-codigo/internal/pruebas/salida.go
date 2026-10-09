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
//	numeros   the sequences of numbers printed are equal (floats within 1e-9)
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
		x, err1 := strconv.ParseFloat(a[i], 64)
		y, err2 := strconv.ParseFloat(b[i], 64)
		if err1 != nil || err2 != nil {
			if a[i] != b[i] {
				return false
			}
			continue
		}
		if math.Abs(x-y) > 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y))) {
			return false
		}
	}
	return true
}
