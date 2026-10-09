package nucleo

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Valor holds a Go value in canonical dynamic form:
//
//	CInt/CRune/CByte → int     CFloat → float64     CBool → bool     CString → string
//	CLista/CArreglo  → []Valor (nil = nil slice)
//	CMapa            → Mapa (sorted by key, see Comparar)
//	CStruct          → Estructura (field values in Tipo.Campos order)
//	CPuntero         → nil or Estructura
//	CError           → nil (no error) or ErrorV
type Valor = any

type Mapa []Par
type Par struct{ K, V Valor }
type Estructura []Valor
type ErrorV string

// Ordenada returns a copy of m sorted by key (Comparar); when a key repeats, the last value wins.
func (m Mapa) Ordenada() Mapa {
	if m == nil {
		return nil
	}
	c := make(Mapa, len(m))
	copy(c, m)
	sort.SliceStable(c, func(i, j int) bool { return Comparar(c[i].K, c[j].K) < 0 })
	out := c[:0]
	for _, p := range c {
		if len(out) > 0 && Comparar(out[len(out)-1].K, p.K) == 0 {
			out[len(out)-1] = p
			continue
		}
		out = append(out, p)
	}
	return out
}

// Buscar returns the value stored under k.
func (m Mapa) Buscar(k Valor) (Valor, bool) {
	for _, p := range m {
		if Igual(p.K, k) {
			return p.V, true
		}
	}
	return nil, false
}

// ---- numeric helpers ----

// comoEntero reports whether v is an integer Go value and returns it.
func comoEntero(v Valor) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), x <= math.MaxInt64
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		return int64(x), x <= math.MaxInt64
	}
	return 0, false
}

// comoFloat reports whether v is a number (integer or float) and returns it as float64.
func comoFloat(v Valor) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	if i, ok := comoEntero(v); ok {
		return float64(i), true
	}
	return 0, false
}

func esFloat(v Valor) bool {
	switch v.(type) {
	case float64, float32:
		return true
	}
	return false
}

// floatsIguales: |a-b| <= 1e-9*max(1,|a|,|b|), NaN equals NaN, infinities equal when same sign.
func floatsIguales(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	if a == b {
		return true
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	escala := math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	return math.Abs(a-b) <= 1e-9*escala
}

// esVacio: nil, an empty list and an empty map are interchangeable.
func esVacio(v Valor) bool {
	switch x := v.(type) {
	case nil:
		return true
	case []Valor:
		return len(x) == 0
	case Mapa:
		return len(x) == 0
	}
	return false
}

// Igual is deep equality. A nil slice equals an empty slice. Floats are equal when
// |a-b| <= 1e-9*max(1,|a|,|b|), and NaN equals NaN.
// ErrorV values compare equal when both are non-nil, whatever the message (the message is shown, not compared).
func Igual(a, b Valor) bool {
	if esVacio(a) || esVacio(b) {
		return esVacio(a) && esVacio(b)
	}
	if esFloat(a) || esFloat(b) {
		fa, oka := comoFloat(a)
		fb, okb := comoFloat(b)
		return oka && okb && floatsIguales(fa, fb)
	}
	if ia, ok := comoEntero(a); ok {
		ib, okb := comoEntero(b)
		return okb && ia == ib
	}
	switch x := a.(type) {
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case ErrorV:
		_, ok := b.(ErrorV)
		return ok
	case []Valor:
		y, ok := b.([]Valor)
		return ok && listasIguales(x, y)
	case Estructura:
		y, ok := b.(Estructura)
		return ok && listasIguales(x, y)
	case Mapa:
		y, ok := b.(Mapa)
		if !ok || len(x) != len(y) {
			return false
		}
		xs, ys := x.Ordenada(), y.Ordenada()
		if len(xs) != len(ys) {
			return false
		}
		for i := range xs {
			if !Igual(xs[i].K, ys[i].K) || !Igual(xs[i].V, ys[i].V) {
				return false
			}
		}
		return true
	}
	return fmt.Sprintf("%#v", a) == fmt.Sprintf("%#v", b)
}

func listasIguales(a, b []Valor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !Igual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// rango orders the kinds of values for Comparar.
func rango(v Valor) int {
	switch v.(type) {
	case nil:
		return 0
	case bool:
		return 1
	case string:
		return 3
	case ErrorV:
		return 4
	case []Valor:
		return 5
	case Mapa:
		return 6
	case Estructura:
		return 7
	}
	if _, ok := comoFloat(v); ok {
		return 2
	}
	return 8
}

// Comparar is a total order: ints and floats numerically, strings bytewise, false<true, lists
// lexicographically, nil first (nil compares equal to an empty list or map, as in Igual).
// Floats that Igual considers equal compare 0; NaN sorts before every other number.
func Comparar(a, b Valor) int {
	if esVacio(a) && esVacio(b) {
		return 0
	}
	// nil versus a non-empty container: nil behaves as the empty container
	if a == nil {
		switch b.(type) {
		case []Valor, Mapa:
			return -1
		}
	}
	if b == nil {
		switch a.(type) {
		case []Valor, Mapa:
			return 1
		}
	}
	ra, rb := rango(a), rango(b)
	if ra != rb {
		return cmpInt(int64(ra), int64(rb))
	}
	switch ra {
	case 0:
		return 0
	case 1:
		x, y := a.(bool), b.(bool)
		switch {
		case x == y:
			return 0
		case !x:
			return -1
		}
		return 1
	case 2:
		if !esFloat(a) && !esFloat(b) {
			x, _ := comoEntero(a)
			y, _ := comoEntero(b)
			return cmpInt(x, y)
		}
		x, _ := comoFloat(a)
		y, _ := comoFloat(b)
		if floatsIguales(x, y) {
			return 0
		}
		if math.IsNaN(x) {
			return -1
		}
		if math.IsNaN(y) {
			return 1
		}
		if x < y {
			return -1
		}
		return 1
	case 3:
		return strings.Compare(a.(string), b.(string))
	case 4:
		return strings.Compare(string(a.(ErrorV)), string(b.(ErrorV)))
	case 5:
		return compararListas(a.([]Valor), b.([]Valor))
	case 6:
		x, y := a.(Mapa).Ordenada(), b.(Mapa).Ordenada()
		for i := 0; i < len(x) && i < len(y); i++ {
			if c := Comparar(x[i].K, y[i].K); c != 0 {
				return c
			}
			if c := Comparar(x[i].V, y[i].V); c != 0 {
				return c
			}
		}
		return cmpInt(int64(len(x)), int64(len(y)))
	case 7:
		return compararListas(a.(Estructura), b.(Estructura))
	}
	return strings.Compare(fmt.Sprintf("%#v", a), fmt.Sprintf("%#v", b))
}

func compararListas(a, b []Valor) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := Comparar(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmpInt(int64(len(a)), int64(len(b)))
}

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Clave is a canonical compact encoding. Floats are rounded to 9 significant digits.
// nil, an empty list and an empty map all give "nil"; every ErrorV gives "error" (as in Igual).
// Clave(a)==Clave(b) ⇒ Igual(a,b), except for floats closer than the 9-digit rounding (≈1e-8 relative)
// that Igual's 1e-9 tolerance would still tell apart.
func Clave(v Valor) string {
	var sb strings.Builder
	escribirClave(&sb, v)
	return sb.String()
}

func escribirClave(sb *strings.Builder, v Valor) {
	if esVacio(v) {
		sb.WriteString("nil")
		return
	}
	switch x := v.(type) {
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
		return
	case string:
		sb.WriteString(strconv.Quote(x))
		return
	case ErrorV:
		sb.WriteString("error")
		return
	case []Valor:
		sb.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				sb.WriteByte(',')
			}
			escribirClave(sb, e)
		}
		sb.WriteByte(']')
		return
	case Estructura:
		sb.WriteByte('(')
		for i, e := range x {
			if i > 0 {
				sb.WriteByte(',')
			}
			escribirClave(sb, e)
		}
		sb.WriteByte(')')
		return
	case Mapa:
		sb.WriteByte('{')
		for i, p := range x.Ordenada() {
			if i > 0 {
				sb.WriteByte(',')
			}
			escribirClave(sb, p.K)
			sb.WriteByte(':')
			escribirClave(sb, p.V)
		}
		sb.WriteByte('}')
		return
	}
	if esFloat(v) {
		f, _ := comoFloat(v)
		sb.WriteString(claveFloat(f))
		return
	}
	if i, ok := comoEntero(v); ok {
		sb.WriteString(strconv.FormatInt(i, 10))
		return
	}
	fmt.Fprintf(sb, "%#v", v)
}

func claveFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "+Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	case f == 0:
		return "0"
	}
	return strconv.FormatFloat(f, 'g', 9, 64)
}

// Copiar returns a deep copy.
func Copiar(v Valor) Valor {
	switch x := v.(type) {
	case []Valor:
		if x == nil {
			return []Valor(nil)
		}
		c := make([]Valor, len(x))
		for i, e := range x {
			c[i] = Copiar(e)
		}
		return c
	case Estructura:
		if x == nil {
			return Estructura(nil)
		}
		c := make(Estructura, len(x))
		for i, e := range x {
			c[i] = Copiar(e)
		}
		return c
	case Mapa:
		if x == nil {
			return Mapa(nil)
		}
		c := make(Mapa, len(x))
		for i, p := range x {
			c[i] = Par{K: Copiar(p.K), V: Copiar(p.V)}
		}
		return c
	}
	return v
}

// Tamano is a size for shrinking and ordering: |int|, len(string), 1+Σ children (saturating).
func Tamano(v Valor) int {
	switch x := v.(type) {
	case nil:
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		return len(x)
	case ErrorV:
		return len(x)
	case []Valor:
		return 1 + sumaTamanos(x)
	case Estructura:
		return 1 + sumaTamanos(x)
	case Mapa:
		t := 1
		for _, p := range x {
			t = sumaSat(t, sumaSat(Tamano(p.K), Tamano(p.V)))
		}
		return t
	}
	if esFloat(v) {
		f, _ := comoFloat(v)
		f = math.Abs(f)
		if math.IsNaN(f) {
			return 0
		}
		if f >= math.MaxInt64/2 {
			return math.MaxInt64 / 2
		}
		return int(f)
	}
	if i, ok := comoEntero(v); ok {
		if i == math.MinInt64 {
			return math.MaxInt64 / 2
		}
		if i < 0 {
			i = -i
		}
		if i > math.MaxInt64/2 {
			return math.MaxInt64 / 2
		}
		return int(i)
	}
	return 1
}

func sumaTamanos(xs []Valor) int {
	t := 0
	for _, e := range xs {
		t = sumaSat(t, Tamano(e))
	}
	return t
}

func sumaSat(a, b int) int {
	if a > math.MaxInt64/2-b {
		return math.MaxInt64 / 2
	}
	return a + b
}

// ---- formatting ----

// FormatoGo writes v as a Go literal of type t: []int{1, 2}, 'a', "hola", map[string]int{"a": 1},
// Punto{X: 1, Y: 2}, errors.New("x"). Elements of lists, arrays and maps elide their composite type,
// as gofmt -s does. NaN and infinities use math.NaN() and math.Inf(±1).
func FormatoGo(v Valor, t Tipo) string {
	var sb strings.Builder
	formatoGo(&sb, v, t, false)
	return sb.String()
}

func formatoGo(sb *strings.Builder, v Valor, t Tipo, elidir bool) {
	if t.Clase == CInvalida {
		t = tipoDinamico(v)
	}
	switch t.Clase {
	case CInt, CRune, CByte:
		i, ok := comoEntero(v)
		if !ok {
			if f, okf := comoFloat(v); okf && f == math.Trunc(f) {
				i, ok = int64(f), true
			}
		}
		if !ok {
			fmt.Fprintf(sb, "%v", v)
			return
		}
		s := strconv.FormatInt(i, 10)
		if t.Clase == CRune && i >= 0 && i <= unicode.MaxRune && utf8.ValidRune(rune(i)) {
			s = strconv.QuoteRune(rune(i))
		} else if t.Clase == CByte && i >= 0x20 && i < 0x7f {
			s = strconv.QuoteRune(rune(i))
		}
		escribirConversion(sb, t, s)
	case CFloat:
		f, ok := comoFloat(v)
		if !ok {
			fmt.Fprintf(sb, "%v", v)
			return
		}
		var s string
		switch {
		case math.IsNaN(f):
			s = "math.NaN()"
		case math.IsInf(f, 1):
			s = "math.Inf(1)"
		case math.IsInf(f, -1):
			s = "math.Inf(-1)"
		default:
			bits := 64
			if t.nombre() == "float32" {
				bits = 32
			}
			s = strconv.FormatFloat(f, 'g', -1, bits)
			if !strings.ContainsAny(s, ".eEn") {
				s += ".0"
			}
		}
		escribirConversion(sb, t, s)
	case CBool:
		b, _ := v.(bool)
		escribirConversion(sb, t, strconv.FormatBool(b))
	case CString:
		s, _ := v.(string)
		escribirConversion(sb, t, strconv.Quote(s))
	case CError:
		if v == nil {
			sb.WriteString("nil")
			return
		}
		sb.WriteString("errors.New(")
		sb.WriteString(strconv.Quote(fmt.Sprint(v)))
		sb.WriteString(")")
	case CLista, CArreglo:
		xs, _ := v.([]Valor)
		if xs == nil && t.Clase == CLista {
			if t.Definido {
				sb.WriteString(t.Nombre + "(nil)")
			} else {
				sb.WriteString("nil")
			}
			return
		}
		if !elidir {
			sb.WriteString(t.Go())
		}
		sb.WriteByte('{')
		e := elemDe(t.Elem)
		for i, x := range xs {
			if i > 0 {
				sb.WriteString(", ")
			}
			formatoGo(sb, x, e, true)
		}
		sb.WriteByte('}')
	case CMapa:
		m, _ := v.(Mapa)
		if m == nil {
			if t.Definido {
				sb.WriteString(t.Nombre + "(nil)")
			} else {
				sb.WriteString("nil")
			}
			return
		}
		if !elidir {
			sb.WriteString(t.Go())
		}
		sb.WriteByte('{')
		k, e := elemDe(t.Clave), elemDe(t.Elem)
		for i, p := range m.Ordenada() {
			if i > 0 {
				sb.WriteString(", ")
			}
			formatoGo(sb, p.K, k, true)
			sb.WriteString(": ")
			formatoGo(sb, p.V, e, true)
		}
		sb.WriteByte('}')
	case CStruct:
		s, _ := v.(Estructura)
		if !elidir {
			sb.WriteString(t.Go())
		}
		sb.WriteByte('{')
		for i, c := range t.Campos {
			if i >= len(s) {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.Nombre)
			sb.WriteString(": ")
			formatoGo(sb, s[i], c.Tipo, false)
		}
		sb.WriteByte('}')
	case CPuntero:
		if v == nil {
			sb.WriteString("nil")
			return
		}
		if !elidir {
			sb.WriteByte('&')
		}
		formatoGo(sb, v, elemDe(t.Elem), elidir)
	default:
		fmt.Fprintf(sb, "%#v", v)
	}
}

// escribirConversion wraps a basic literal in a conversion when the type is defined (Celsius(3.5)).
func escribirConversion(sb *strings.Builder, t Tipo, lit string) {
	if t.Definido && t.Nombre != "" {
		sb.WriteString(t.Nombre)
		sb.WriteByte('(')
		sb.WriteString(lit)
		sb.WriteByte(')')
		return
	}
	sb.WriteString(lit)
}

// tipoDinamico guesses a Tipo from a dynamic value (used when no type is known).
func tipoDinamico(v Valor) Tipo {
	switch x := v.(type) {
	case bool:
		return TBool
	case string:
		return TString
	case ErrorV:
		return TError
	case []Valor:
		var e Tipo
		for _, el := range x {
			if u, ok := Unificar(e, tipoDinamico(el)); ok {
				e = u
			}
		}
		return ListaDe(e)
	case Mapa:
		var k, e Tipo
		for _, p := range x {
			if u, ok := Unificar(k, tipoDinamico(p.K)); ok {
				k = u
			}
			if u, ok := Unificar(e, tipoDinamico(p.V)); ok {
				e = u
			}
		}
		return MapaDe(k, e)
	case Estructura:
		t := Tipo{Clase: CStruct}
		for i, el := range x {
			t.Campos = append(t.Campos, Campo{Nombre: "C" + strconv.Itoa(i), Tipo: tipoDinamico(el)})
		}
		return t
	}
	if esFloat(v) {
		return TFloat
	}
	if _, ok := comoEntero(v); ok {
		return TInt
	}
	return Tipo{}
}

const maxHumano = 200

// FormatoHumano shows v for a person: [1, 2]  'a'  "hola"  {a: 1}  {X: 1, Y: 2}  error("x")  nil
// (at most 200 runes, then "…"). Nil lists show as [] and nil maps as {}.
func FormatoHumano(v Valor, t Tipo) string {
	var sb strings.Builder
	formatoHumano(&sb, v, t)
	s := sb.String()
	if utf8.RuneCountInString(s) > maxHumano {
		n := 0
		for i := range s {
			if n == maxHumano {
				return s[:i] + "…"
			}
			n++
		}
	}
	return s
}

func formatoHumano(sb *strings.Builder, v Valor, t Tipo) {
	if sb.Len() > 4*maxHumano+16 { // enough to cut at 200 runes; stop early on huge values
		return
	}
	if t.Clase == CInvalida {
		t = tipoDinamico(v)
	}
	switch t.Clase {
	case CInt, CRune, CByte:
		i, ok := comoEntero(v)
		if !ok {
			if f, okf := comoFloat(v); okf && f == math.Trunc(f) {
				i, ok = int64(f), true
			}
		}
		switch {
		case !ok:
			fmt.Fprint(sb, v)
		case t.Clase == CRune && i >= 0 && utf8.ValidRune(rune(i)):
			sb.WriteString(strconv.QuoteRune(rune(i)))
		case t.Clase == CByte && i >= 0x20 && i < 0x7f:
			sb.WriteString(strconv.QuoteRune(rune(i)))
		default:
			sb.WriteString(strconv.FormatInt(i, 10))
		}
	case CFloat:
		f, ok := comoFloat(v)
		if !ok {
			fmt.Fprint(sb, v)
			return
		}
		sb.WriteString(floatHumano(f))
	case CBool:
		fmt.Fprint(sb, v)
	case CString:
		s, _ := v.(string)
		sb.WriteString(strconv.Quote(s))
	case CError:
		if v == nil {
			sb.WriteString("nil")
			return
		}
		sb.WriteString("error(")
		sb.WriteString(strconv.Quote(fmt.Sprint(v)))
		sb.WriteString(")")
	case CLista, CArreglo:
		xs, _ := v.([]Valor)
		sb.WriteByte('[')
		e := elemDe(t.Elem)
		for i, x := range xs {
			if i > 0 {
				sb.WriteString(", ")
			}
			formatoHumano(sb, x, e)
			if sb.Len() > 4*maxHumano+16 {
				break
			}
		}
		sb.WriteByte(']')
	case CMapa:
		m, _ := v.(Mapa)
		sb.WriteByte('{')
		k, e := elemDe(t.Clave), elemDe(t.Elem)
		for i, p := range m.Ordenada() {
			if i > 0 {
				sb.WriteString(", ")
			}
			if s, ok := p.K.(string); ok && k.Clase == CString && esPalabraSimple(s) {
				sb.WriteString(s)
			} else {
				formatoHumano(sb, p.K, k)
			}
			sb.WriteString(": ")
			formatoHumano(sb, p.V, e)
			if sb.Len() > 4*maxHumano+16 {
				break
			}
		}
		sb.WriteByte('}')
	case CStruct:
		s, _ := v.(Estructura)
		sb.WriteByte('{')
		for i, c := range t.Campos {
			if i >= len(s) {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.Nombre)
			sb.WriteString(": ")
			formatoHumano(sb, s[i], c.Tipo)
		}
		sb.WriteByte('}')
	case CPuntero:
		if v == nil {
			sb.WriteString("nil")
			return
		}
		sb.WriteByte('&')
		formatoHumano(sb, v, elemDe(t.Elem))
	default:
		if v == nil {
			sb.WriteString("nil")
			return
		}
		fmt.Fprint(sb, v)
	}
}

// floatHumano shows a float with at most 12 significant digits and no exponent for ordinary sizes.
func floatHumano(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "+Inf"
	case math.IsInf(f, -1):
		return "-Inf"
	}
	r, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 12, 64), 64)
	if err != nil {
		r = f
	}
	a := math.Abs(r)
	if r == 0 || (a >= 1e-4 && a < 1e15) {
		return strconv.FormatFloat(r, 'f', -1, 64)
	}
	return strconv.FormatFloat(r, 'g', -1, 64)
}

func esPalabraSimple(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
