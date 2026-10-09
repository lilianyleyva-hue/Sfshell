package sintesis

// Helper functions emitted once into generated code. Each one implements exactly the interpreter's
// semantics wherever the interpreter has a result (on ⊥ inputs it returns something harmless and never
// panics). Their imports are listed in importsAyudantes.

var ayudantes = map[string]string{
	"abs": `func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}`,
	"potencia": `func potencia(base, exp int) int {
	r := 1
	for exp > 0 {
		if exp%2 == 1 {
			r *= base
		}
		base *= base
		exp /= 2
	}
	return r
}`,
	"esPrimo": `func esPrimo(n int) bool {
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
}`,
	"esCuadrado": `func esCuadrado(n int) bool {
	if n < 0 {
		return false
	}
	r := int(math.Sqrt(float64(n)))
	for r > 0 && r > n/r {
		r--
	}
	for r+1 <= n/(r+1) {
		r++
	}
	return r*r == n
}`,
	"esCapicua": `func esCapicua(n int) bool {
	if n < 0 {
		return false
	}
	s := strconv.Itoa(n)
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		if s[i] != s[j] {
			return false
		}
	}
	return true
}`,
	"factorial": `func factorial(n int) int {
	if n > 20 {
		return 0 // no cabe en un int
	}
	f := 1
	for i := 2; i <= n; i++ {
		f *= i
	}
	return f
}`,
	"fibonacci": `func fibonacci(n int) int {
	if n > 92 {
		return 0 // no cabe en un int
	}
	a, b := 0, 1
	for i := 0; i < n; i++ {
		a, b = b, a+b
	}
	return a
}`,
	"mcd": `func mcd(a, b int) int {
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
}`,
	"mcm": `func mcm(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	x, y := a, b
	for y != 0 {
		x, y = y, x%y
	}
	return a / x * b
}`,
	"digitos": `func digitos(n int) []int {
	if n < 0 {
		n = -n
	}
	if n == 0 {
		return []int{0}
	}
	var ds []int
	for n > 0 {
		ds = append([]int{n % 10}, ds...)
		n /= 10
	}
	return ds
}`,
	"sumaDigitos": `func sumaDigitos(n int) int {
	if n < 0 {
		n = -n
	}
	s := 0
	for n > 0 {
		s += n % 10
		n /= 10
	}
	return s
}`,
	"invertirNumero": `func invertirNumero(n int) int {
	signo := 1
	if n < 0 {
		signo, n = -1, -n
	}
	r := 0
	for n > 0 {
		r = r*10 + n%10
		n /= 10
	}
	return signo * r
}`,
	"numDigitos": `func numDigitos(n int) int {
	if n == 0 {
		return 1
	}
	c := 0
	for n != 0 {
		c++
		n /= 10
	}
	return c
}`,
	"raizEntera": `func raizEntera(n int) int {
	if n < 2 {
		return max(n, 0)
	}
	r := int(math.Sqrt(float64(n)))
	for r > 0 && r > n/r {
		r--
	}
	for r+1 <= n/(r+1) {
		r++
	}
	return r
}`,
	"unicos": `func unicos[T comparable](xs []T) []T {
	visto := make(map[T]bool)
	var out []T
	for _, x := range xs {
		if !visto[x] {
			visto[x] = true
			out = append(out, x)
		}
	}
	return out
}`,
	"tomar": `func tomar[T any](xs []T, n int) []T {
	n = max(0, min(n, len(xs)))
	return xs[:n]
}`,
	"quitar": `func quitar[T any](xs []T, n int) []T {
	n = max(0, min(n, len(xs)))
	return xs[n:]
}`,
	"rango": `func rango(desde, hasta int) []int {
	var out []int
	for i := desde; i < hasta && len(out) < 10000; i++ {
		out = append(out, i)
	}
	return out
}`,
	"sumaAcumulada": `func sumaAcumulada(xs []int) []int {
	out := make([]int, len(xs))
	total := 0
	for i, x := range xs {
		total += x
		out[i] = total
	}
	return out
}`,
	"titulo": `func titulo(s string) string {
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
}`,
	"capitalizar": `func capitalizar(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}`,
	"invertirTexto": `func invertirTexto(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs)
}`,
	"esPalindromo": `func esPalindromo(s string) bool {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		if rs[i] != rs[j] {
			return false
		}
	}
	return true
}`,
	"esPalindromoLetras": `func esPalindromoLetras(s string) bool {
	var rs []rune
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		r = unicode.ToLower(r)
		switch r {
		case 'á':
			r = 'a'
		case 'é':
			r = 'e'
		case 'í':
			r = 'i'
		case 'ó':
			r = 'o'
		case 'ú', 'ü':
			r = 'u'
		}
		rs = append(rs, r)
	}
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		if rs[i] != rs[j] {
			return false
		}
	}
	return true
}`,
	"esVocal": `func esVocal(r rune) bool {
	return strings.ContainsRune("aeiouáéíóúüAEIOUÁÉÍÓÚÜ", r)
}`,
	"esConsonante": `func esConsonante(r rune) bool {
	return unicode.IsLetter(r) && !strings.ContainsRune("aeiouáéíóúüAEIOUÁÉÍÓÚÜ", r)
}`,
	"largos": `func largos(palabras []string) []int {
	out := make([]int, len(palabras))
	for i, p := range palabras {
		out[i] = utf8.RuneCountInString(p)
	}
	return out
}`,
	"frecuenciaLetras": `func frecuenciaLetras(s string) map[string]int {
	m := make(map[string]int)
	for _, r := range s {
		if unicode.IsLetter(r) {
			m[string(unicode.ToLower(r))]++
		}
	}
	return m
}`,
	"transpuesta": `func transpuesta(m [][]int) [][]int {
	if len(m) == 0 {
		return nil
	}
	for _, fila := range m {
		if len(fila) != len(m[0]) {
			return nil
		}
	}
	t := make([][]int, len(m[0]))
	for j := range t {
		t[j] = make([]int, len(m))
		for i := range m {
			t[j][i] = m[i][j]
		}
	}
	return t
}`,
	"sumaFilas": `func sumaFilas(m [][]int) []int {
	out := make([]int, len(m))
	for i, fila := range m {
		for _, x := range fila {
			out[i] += x
		}
	}
	return out
}`,
	"aplanar": `func aplanar(m [][]int) []int {
	var out []int
	for _, fila := range m {
		out = append(out, fila...)
	}
	return out
}`,
	"diagonal": `func diagonal(m [][]int) []int {
	out := make([]int, 0, len(m))
	for i, fila := range m {
		if i >= len(fila) {
			return nil
		}
		out = append(out, fila[i])
	}
	return out
}`,
}

var importsAyudantes = map[string][]string{
	"esPrimo":            {"math/big"},
	"esCuadrado":         {"math"},
	"raizEntera":         {"math"},
	"esCapicua":          {"strconv"},
	"titulo":             {"strings", "unicode"},
	"capitalizar":        {"unicode", "unicode/utf8"},
	"esPalindromoLetras": {"unicode"},
	"esVocal":            {"strings"},
	"esConsonante":       {"strings", "unicode"},
	"largos":             {"unicode/utf8"},
	"frecuenciaLetras":   {"unicode"},
}
