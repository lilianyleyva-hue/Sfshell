package sintesis

import (
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Value helpers for the interpreter. DSL values are in nucleo's canonical form: int (also runes),
// float64, bool, string, []nucleo.Valor, nucleo.Mapa (sorted by key).

type V = nucleo.Valor

const (
	maxLargo = 10000  // longest list a primitive may build
	maxTexto = 100000 // longest string a primitive may build
)

func ent(v V) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	}
	return 0
}

func flo(v V) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

func boo(v V) bool   { b, _ := v.(bool); return b }
func tex(v V) string { s, _ := v.(string); return s }
func lis(v V) []V    { xs, _ := v.([]V); return xs }

func sumaSegura(a, b int) (int, bool) {
	c := a + b
	if (b > 0 && c < a) || (b < 0 && c > a) {
		return 0, false
	}
	return c, true
}

func restaSegura(a, b int) (int, bool) {
	c := a - b
	if (b > 0 && c > a) || (b < 0 && c < a) {
		return 0, false
	}
	return c, true
}

func multSegura(a, b int) (int, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == -1 && b == math.MinInt) || (b == -1 && a == math.MinInt) {
		return 0, false
	}
	c := a * b
	if c/b != a {
		return 0, false
	}
	return c, true
}

func absInt(a int) (int, bool) {
	if a == math.MinInt {
		return 0, false
	}
	if a < 0 {
		return -a, true
	}
	return a, true
}

func potencia(a, b int) (int, bool) {
	if b < 0 || b > 62 {
		return 0, false
	}
	r := 1
	for i := 0; i < b; i++ {
		var ok bool
		if r, ok = multSegura(r, a); !ok {
			return 0, false
		}
	}
	return r, true
}

func esPrimo(n int) bool {
	if n < 2 {
		return false
	}
	if n > 1<<40 {
		return big.NewInt(int64(n)).ProbablyPrime(20)
	}
	for d := 2; d <= n/d; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}

func raizEntera(n int) int {
	if n < 2 {
		return n
	}
	r := int(math.Sqrt(float64(n)))
	for r > 0 && r > n/r {
		r--
	}
	for r+1 <= n/(r+1) {
		r++
	}
	return r
}

func mcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// digitosDe returns the decimal digits of |n| (⊥ for math.MinInt, whose absolute value does not fit).
func digitosDe(n int) ([]int, bool) {
	if n == math.MinInt {
		return nil, false
	}
	if n < 0 {
		n = -n
	}
	if n == 0 {
		return []int{0}, true
	}
	var ds []int
	for n > 0 {
		ds = append(ds, n%10)
		n /= 10
	}
	for i, j := 0, len(ds)-1; i < j; i, j = i+1, j-1 {
		ds[i], ds[j] = ds[j], ds[i]
	}
	return ds, true
}

func esVocal(r rune) bool { return strings.ContainsRune("aeiouáéíóúüAEIOUÁÉÍÓÚÜ", r) }

func titulo(s string) string {
	var sb strings.Builder
	anterior := ' '
	for _, r := range s {
		if unicode.IsSpace(anterior) {
			sb.WriteRune(unicode.ToUpper(r))
		} else {
			sb.WriteRune(r)
		}
		anterior = r
	}
	return sb.String()
}

func capitalizar(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

func invertirTexto(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs)
}

func esPalindromo(s string) bool {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		if rs[i] != rs[j] {
			return false
		}
	}
	return true
}

func sinTilde(r rune) rune {
	switch r {
	case 'á':
		return 'a'
	case 'é':
		return 'e'
	case 'í':
		return 'i'
	case 'ó':
		return 'o'
	case 'ú', 'ü':
		return 'u'
	}
	return r
}

func esPalindromoLetras(s string) bool {
	var rs []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			rs = append(rs, sinTilde(unicode.ToLower(r)))
		}
	}
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		if rs[i] != rs[j] {
			return false
		}
	}
	return true
}

// menor orders two DSL values of the same element type (ints, runes, floats, strings).
func menor(a, b V) bool {
	switch x := a.(type) {
	case int:
		y, _ := b.(int)
		return x < y
	case float64:
		y, _ := b.(float64)
		return x < y
	case string:
		y, _ := b.(string)
		return x < y
	case bool:
		y, _ := b.(bool)
		return !x && y
	}
	return nucleo.Comparar(a, b) < 0
}

func ordenarValores(xs []V) []V {
	c := append([]V(nil), xs...)
	sort.SliceStable(c, func(i, j int) bool { return menor(c[i], c[j]) })
	return c
}

func invertirValores(xs []V) []V {
	c := make([]V, len(xs))
	for i, x := range xs {
		c[len(xs)-1-i] = x
	}
	return c
}

// igualExacto is structural equality on DSL values (nil list == empty list; floats bit-equal or both NaN).
func igualExacto(a, b V) bool {
	switch x := a.(type) {
	case int:
		y, ok := b.(int)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && (x == y || (x != x && y != y))
	case fallo:
		_, ok := b.(fallo)
		return ok
	case nil:
		return esVacioV(b)
	case []V:
		if len(x) == 0 {
			return esVacioV(b)
		}
		y, ok := b.([]V)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !igualExacto(x[i], y[i]) {
				return false
			}
		}
		return true
	case nucleo.Mapa:
		if len(x) == 0 {
			return esVacioV(b)
		}
		y, ok := b.(nucleo.Mapa)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !igualExacto(x[i].K, y[i].K) || !igualExacto(x[i].V, y[i].V) {
				return false
			}
		}
		return true
	}
	return nucleo.Igual(a, b)
}

func esVacioV(v V) bool {
	switch x := v.(type) {
	case nil:
		return true
	case []V:
		return len(x) == 0
	case nucleo.Mapa:
		return len(x) == 0
	}
	return false
}

// fallo is ⊥ inside the search's output vectors.
type fallo struct{}

var bottom V = fallo{}

func esFallo(v V) bool { _, ok := v.(fallo); return ok }

// FNV-1a over a value, without allocating.
const (
	fnvBase  = 14695981039346656037
	fnvPrimo = 1099511628211
)

func mezclar(h uint64, x uint64) uint64 {
	for i := 0; i < 8; i++ {
		h ^= x & 0xff
		h *= fnvPrimo
		x >>= 8
	}
	return h
}

func hashValor(h uint64, v V) uint64 {
	switch x := v.(type) {
	case int:
		return mezclar(mezclar(h, 1), uint64(x))
	case bool:
		if x {
			return mezclar(h, 2)
		}
		return mezclar(h, 3)
	case string:
		h = mezclar(h, 4)
		for i := 0; i < len(x); i++ {
			h ^= uint64(x[i])
			h *= fnvPrimo
		}
		return mezclar(h, uint64(len(x)))
	case float64:
		if x == 0 {
			x = 0 // -0 → 0
		}
		return mezclar(mezclar(h, 5), math.Float64bits(x))
	case fallo:
		return mezclar(h, 6)
	case nil:
		return mezclar(h, 7)
	case []V:
		if len(x) == 0 {
			return mezclar(h, 7)
		}
		h = mezclar(h, 8)
		for _, e := range x {
			h = hashValor(h, e)
		}
		return mezclar(h, uint64(len(x)))
	case nucleo.Mapa:
		if len(x) == 0 {
			return mezclar(h, 7)
		}
		h = mezclar(h, 9)
		for _, p := range x {
			h = hashValor(hashValor(h, p.K), p.V)
		}
		return h
	}
	return mezclar(h, 10)
}

// canon converts a value to the DSL's canonical form for type t (ints as int, lists as []V…).
func canon(v V, t nucleo.Tipo) (V, error) {
	return nucleo.Ajustar(v, t)
}
