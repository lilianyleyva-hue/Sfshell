package puzles

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Filtro keeps the sequences (or, in a range, the one-element sequences [x]) for which F is true.
type Filtro struct {
	Nombre string
	F      func(sec []int) bool
}

// Modelo describes what is counted: sequences of Longitud symbols of Alfabeto (ordered or not,
// with or without repetition, around a circle), anagrams of a multiset, integers in a range, or
// ways to climb stairs. The last four fields are additive to the spec.
type Modelo struct {
	Alfabeto   []string
	Longitud   int
	Ordenado   bool
	Repeticion bool
	Circular   bool
	Multiconj  []int    // counts per symbol (anagrams)
	Rango      [2]int64 // "del 1 al 1000" style
	Filtros    []Filtro

	Divisores      []int64 // range: divisible by at least one of these (inclusion–exclusion)
	Contiene       []int   // symbols that must appear at least once ("al menos uno")
	SinCeroInicial bool    // "números de k cifras": the first symbol is not Alfabeto[0]
	Escalones      []int   // stairs: allowed step sizes; Longitud is the number of steps to climb
}

// LimiteFuerza is the largest space counted one by one (10⁷).
const LimiteFuerza = 10_000_000

func (m Modelo) esRango() bool { return m.Rango[0] != 0 || m.Rango[1] != 0 }

// --- exact combinatorics

func factorial(n int) *big.Int {
	if n < 0 {
		return big.NewInt(0)
	}
	return new(big.Int).MulRange(1, int64(max(n, 1)))
}

// combinaciones is C(n, k).
func combinaciones(n, k int) *big.Int {
	if k < 0 || n < 0 || k > n {
		return big.NewInt(0)
	}
	return new(big.Int).Binomial(int64(n), int64(k))
}

// variaciones is P(n, k) = n!/(n−k)!.
func variaciones(n, k int) *big.Int {
	if k < 0 || n < 0 || k > n {
		return big.NewInt(0)
	}
	if k == 0 {
		return big.NewInt(1)
	}
	return new(big.Int).MulRange(int64(n-k+1), int64(n))
}

func potencia(n, k int) *big.Int {
	return new(big.Int).Exp(big.NewInt(int64(n)), big.NewInt(int64(k)), nil)
}

func mcm(a, b int64) int64 {
	x, y := a, b
	for y != 0 {
		x, y = y, x%y
	}
	if x == 0 {
		return 0
	}
	return a / x * b
}

// base counts the sequences over an alphabet of n symbols (no filters).
func (m Modelo) base(n int) (*big.Int, string) {
	k := m.Longitud
	switch {
	case m.Circular && !m.Repeticion:
		if k == n {
			v := factorial(n - 1)
			return v, fmt.Sprintf("(%d−1)! = %s (en círculo, los giros no cuentan)", n, v)
		}
		v := new(big.Int).Div(variaciones(n, k), big.NewInt(int64(max(k, 1))))
		return v, fmt.Sprintf("P(%d,%d)/%d = %s (en círculo, los giros no cuentan)", n, k, k, v)
	case m.Ordenado && m.Repeticion:
		v := potencia(n, k)
		return v, fmt.Sprintf("%d^%d = %s (importa el orden y se puede repetir)", n, k, v)
	case m.Ordenado:
		v := variaciones(n, k)
		if k == n {
			return v, fmt.Sprintf("%d! = %s (importa el orden, sin repetir)", n, v)
		}
		return v, fmt.Sprintf("P(%d,%d) = %d!/%d! = %s (importa el orden, sin repetir)", n, k, n, n-k, v)
	case m.Repeticion:
		v := combinaciones(n+k-1, k)
		return v, fmt.Sprintf("C(%d+%d−1,%d) = C(%d,%d) = %s (no importa el orden y se puede repetir)", n, k, k, n+k-1, k, v)
	}
	v := combinaciones(n, k)
	return v, fmt.Sprintf("C(%d,%d) = %d!/(%d!·%d!) = %s (no importa el orden, sin repetir)", n, k, n, k, n-k, v)
}

// Formula returns the closed-form count, a Spanish explanation and ok; ok is false when there is
// no formula for this model (arbitrary filters, say), and then only brute force can count it.
func Formula(m Modelo) (*big.Int, string, bool) {
	switch {
	case len(m.Escalones) > 0:
		if len(m.Filtros) > 0 || m.Longitud < 0 {
			return nil, "", false
		}
		f := make([]*big.Int, m.Longitud+1)
		f[0] = big.NewInt(1)
		for i := 1; i <= m.Longitud; i++ {
			f[i] = new(big.Int)
			for _, p := range m.Escalones {
				if p > 0 && i-p >= 0 {
					f[i].Add(f[i], f[i-p])
				}
			}
		}
		var partes []string
		for _, p := range m.Escalones {
			partes = append(partes, fmt.Sprintf("f(n−%d)", p))
		}
		expl := fmt.Sprintf("f(n) = %s con f(0) = 1, así que f(%d) = %s", strings.Join(partes, " + "), m.Longitud, f[m.Longitud])
		if len(m.Escalones) == 2 && m.Escalones[0] == 1 && m.Escalones[1] == 2 {
			expl += " (es la sucesión de Fibonacci)"
		}
		return f[m.Longitud], expl, true
	case m.esRango():
		a, b := m.Rango[0], m.Rango[1]
		if b < a {
			return big.NewInt(0), "el rango está vacío", true
		}
		if len(m.Filtros) > 0 {
			return nil, "", false
		}
		if len(m.Divisores) == 0 {
			v := big.NewInt(b - a + 1)
			return v, fmt.Sprintf("%d − %d + 1 = %s", b, a, v), true
		}
		if len(m.Divisores) > 12 {
			return nil, "", false
		}
		cuenta := func(d int64) int64 { return piso(b, d) - piso(a-1, d) }
		total := int64(0)
		var terminos []string
		k := len(m.Divisores)
		for mask := 1; mask < 1<<k; mask++ {
			l, bitsN := int64(1), 0
			for i := 0; i < k; i++ {
				if mask&(1<<i) != 0 {
					l = mcm(l, m.Divisores[i])
					bitsN++
				}
			}
			if l <= 0 {
				return nil, "", false
			}
			c := cuenta(l)
			signo := "+"
			if bitsN%2 == 0 {
				total -= c
				signo = "−"
			} else {
				total += c
			}
			if len(terminos) == 0 && signo == "+" {
				terminos = append(terminos, fmt.Sprintf("%d (múltiplos de %d)", c, l))
			} else {
				terminos = append(terminos, fmt.Sprintf("%s %d (múltiplos de %d)", signo, c, l))
			}
		}
		v := big.NewInt(total)
		if k == 1 {
			return v, fmt.Sprintf("hay %d múltiplos de %d", total, m.Divisores[0]), true
		}
		return v, fmt.Sprintf("inclusión–exclusión: %s = %d", strings.Join(terminos, " "), total), true
	case len(m.Multiconj) > 0:
		if len(m.Filtros) > 0 || len(m.Contiene) > 0 {
			return nil, "", false
		}
		total := 0
		div := big.NewInt(1)
		var partes []string
		for _, c := range m.Multiconj {
			total += c
			div.Mul(div, factorial(c))
			partes = append(partes, fmt.Sprintf("%d!", c))
		}
		v := new(big.Int).Div(factorial(total), div)
		return v, fmt.Sprintf("%d!/(%s) = %s", total, strings.Join(partes, "·"), v), true
	}
	if len(m.Filtros) > 0 || (m.Circular && m.Repeticion) {
		return nil, "", false
	}
	n := len(m.Alfabeto)
	if m.SinCeroInicial {
		if len(m.Contiene) > 0 || !m.Ordenado || m.Circular || m.Longitud < 1 {
			return nil, "", false
		}
		if m.Repeticion {
			v := new(big.Int).Mul(big.NewInt(int64(n-1)), potencia(n, m.Longitud-1))
			return v, fmt.Sprintf("%d·%d^%d = %s (la primera cifra no puede ser 0)", n-1, n, m.Longitud-1, v), true
		}
		v := new(big.Int).Mul(big.NewInt(int64(n-1)), variaciones(n-1, m.Longitud-1))
		return v, fmt.Sprintf("%d·P(%d,%d) = %s (la primera cifra no puede ser 0)", n-1, n-1, m.Longitud-1, v), true
	}
	if len(m.Contiene) == 0 {
		v, e := m.base(n)
		return v, e, true
	}
	// inclusion–exclusion over the symbols that must appear
	k := len(m.Contiene)
	if k > 12 {
		return nil, "", false
	}
	total := new(big.Int)
	var terminos []string
	for mask := 0; mask < 1<<k; mask++ {
		quitados := 0
		for i := 0; i < k; i++ {
			if mask&(1<<i) != 0 {
				quitados++
			}
		}
		v, _ := m.base(n - quitados)
		if quitados%2 == 1 {
			total.Sub(total, v)
			terminos = append(terminos, "− "+v.String())
		} else {
			total.Add(total, v)
			if mask == 0 {
				terminos = append(terminos, v.String())
			} else {
				terminos = append(terminos, "+ "+v.String())
			}
		}
	}
	return total, fmt.Sprintf("todas menos las que no lo cumplen (inclusión–exclusión): %s = %s", strings.Join(terminos, " "), total), true
}

func piso(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// Espacio is the number of elements brute force would visit (or −1 when it cannot be computed).
func Espacio(m Modelo) *big.Int {
	switch {
	case len(m.Escalones) > 0:
		mm := m
		mm.Filtros = nil
		v, _, _ := Formula(mm)
		return v
	case m.esRango():
		if m.Rango[1] < m.Rango[0] {
			return big.NewInt(0)
		}
		return big.NewInt(m.Rango[1] - m.Rango[0] + 1)
	case len(m.Multiconj) > 0:
		mm := m
		mm.Filtros, mm.Contiene = nil, nil
		v, _, _ := Formula(mm)
		return v
	}
	n, k := len(m.Alfabeto), m.Longitud
	if !m.Repeticion {
		return variaciones(n, k)
	}
	return potencia(n, k)
}

var errDemasiado = errors.New("puzles: demasiados casos para contarlos uno a uno")

// ContarFuerza counts the model one element at a time (≤ limite elements, at most 10⁷): sequences
// are enumerated, unordered models keep only the sorted canonical form, circular ones only the
// minimal rotation, and every filter is applied.
func ContarFuerza(ctx context.Context, m Modelo, limite int64) (*big.Int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limite <= 0 || limite > LimiteFuerza {
		limite = LimiteFuerza
	}
	esp := Espacio(m)
	if esp == nil || esp.Cmp(big.NewInt(limite)) > 0 {
		return nil, fmt.Errorf("%w: el espacio tiene %v elementos y cuento hasta %d: %w", errDemasiado, esp, limite, nucleo.ErrNoSoportado)
	}
	fin := time.Now().Add(10 * time.Second)
	var cuenta int64
	visitados := 0
	cortar := func() error {
		visitados++
		if visitados&0xFFFF == 0 && (ctx.Err() != nil || time.Now().After(fin)) {
			return fmt.Errorf("puzles: el recuento no terminó a tiempo: %w", nucleo.ErrSinTiempo)
		}
		return nil
	}
	pasa := func(sec []int) bool {
		for _, f := range m.Filtros {
			if !f.F(sec) {
				return false
			}
		}
		return true
	}
	switch {
	case len(m.Escalones) > 0:
		sec := []int{}
		var errR error
		var rec func(resto int)
		rec = func(resto int) {
			if errR != nil {
				return
			}
			if resto == 0 {
				if errR = cortar(); errR == nil && pasa(sec) {
					cuenta++
				}
				return
			}
			for _, p := range m.Escalones {
				if p > 0 && p <= resto {
					sec = append(sec, p)
					rec(resto - p)
					sec = sec[:len(sec)-1]
				}
			}
		}
		rec(m.Longitud)
		if errR != nil {
			return nil, errR
		}
	case m.esRango():
		sec := []int{0}
		for x := m.Rango[0]; x <= m.Rango[1]; x++ {
			if err := cortar(); err != nil {
				return nil, err
			}
			sec[0] = int(x)
			ok := len(m.Divisores) == 0
			for _, d := range m.Divisores {
				if d != 0 && x%d == 0 {
					ok = true
					break
				}
			}
			if ok && pasa(sec) {
				cuenta++
			}
		}
	case len(m.Multiconj) > 0:
		restantes := append([]int(nil), m.Multiconj...)
		total := 0
		for _, c := range restantes {
			total += c
		}
		sec := make([]int, 0, total)
		var errR error
		var rec func()
		rec = func() {
			if errR != nil {
				return
			}
			if len(sec) == total {
				if errR = cortar(); errR == nil && contieneTodos(sec, m.Contiene) && pasa(sec) {
					cuenta++
				}
				return
			}
			for s := range restantes {
				if restantes[s] == 0 {
					continue
				}
				restantes[s]--
				sec = append(sec, s)
				rec()
				sec = sec[:len(sec)-1]
				restantes[s]++
			}
		}
		rec()
		if errR != nil {
			return nil, errR
		}
	default:
		n, k := len(m.Alfabeto), m.Longitud
		sec := make([]int, k)
		usado := make([]bool, n)
		var errR error
		var rec func(pos int)
		rec = func(pos int) {
			if errR != nil {
				return
			}
			if pos == k {
				if errR = cortar(); errR != nil {
					return
				}
				if !m.Ordenado && !ordenada(sec, m.Repeticion) {
					return
				}
				if m.Circular && !rotacionMinima(sec) {
					return
				}
				if m.SinCeroInicial && k > 0 && sec[0] == 0 {
					return
				}
				if contieneTodos(sec, m.Contiene) && pasa(sec) {
					cuenta++
				}
				return
			}
			for s := 0; s < n; s++ {
				if !m.Repeticion && usado[s] {
					continue
				}
				usado[s] = true
				sec[pos] = s
				rec(pos + 1)
				usado[s] = false
			}
		}
		if k >= 0 && (k <= n || m.Repeticion) {
			rec(0)
		}
		if errR != nil {
			return nil, errR
		}
	}
	return big.NewInt(cuenta), nil
}

func contieneTodos(sec []int, simbolos []int) bool {
	for _, s := range simbolos {
		ok := false
		for _, x := range sec {
			if x == s {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// ordenada: strictly increasing (no repetition) or non-decreasing (with repetition).
func ordenada(sec []int, repeticion bool) bool {
	for i := 1; i < len(sec); i++ {
		if sec[i] < sec[i-1] || (!repeticion && sec[i] == sec[i-1]) {
			return false
		}
	}
	return true
}

// rotacionMinima reports whether sec is the lexicographically smallest of its rotations.
func rotacionMinima(sec []int) bool {
	n := len(sec)
	for r := 1; r < n; r++ {
		for i := 0; i < n; i++ {
			a, b := sec[(i+r)%n], sec[i]
			if a < b {
				return false
			}
			if a > b {
				break
			}
		}
	}
	return true
}

// Probabilidad is (number of elements passing exito) / (number of elements), both counted by brute
// force over equally likely outcomes; it returns an exact fraction.
func Probabilidad(ctx context.Context, m Modelo, exito Filtro) (*big.Rat, error) {
	total, err := ContarFuerza(ctx, m, LimiteFuerza)
	if err != nil {
		return nil, err
	}
	if total.Sign() == 0 {
		return nil, fmt.Errorf("puzles: no hay ningún caso posible: %w", nucleo.ErrNoEntiendo)
	}
	conExito := m
	conExito.Filtros = append(append([]Filtro(nil), m.Filtros...), exito)
	fav, err := ContarFuerza(ctx, conExito, LimiteFuerza)
	if err != nil {
		return nil, err
	}
	return new(big.Rat).SetFrac(fav, total), nil
}
