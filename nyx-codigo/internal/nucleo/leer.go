package nucleo

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// This file holds ParseValor, Inferir, Unificar and Ajustar (§3.2) and the small lexer they share.

// ParseValor reads a literal written by a person or by Go:
//
//	ints "5", "-3"; floats "3.5", "3,5"; bools true/false/verdadero/falso/sí/no;
//	strings with or without quotes when t is string; runes 'a' or a; lists [1,2,3], []int{1,2,3}, {1,2,3}, "1 2 3" for []int;
//	maps {"a":1, "b":2} or map[string]int{"a": 1}; structs {1, 2} or {X: 1, Y: 2} or Punto{X: 1, Y: 2}; nil.
//
// fmt's own output is accepted too: "[1 2 3]", "map[a:1 b:2]", "{1 2}", "{X:1 Y:2}".
func ParseValor(s string, t Tipo) (Valor, error) {
	return parseValor(limpiarTexto(s), t, 0)
}

const maxProfundidad = 64

var errProfundo = errors.New("el valor está demasiado anidado")

// limpiarTexto trims and replaces the typographic minus sign.
func limpiarTexto(s string) string {
	s = strings.TrimSpace(s)
	if strings.ContainsRune(s, '−') {
		s = strings.ReplaceAll(s, "−", "-")
	}
	return s
}

func parseValor(s string, t Tipo, prof int) (Valor, error) {
	if prof > maxProfundidad {
		return nil, errProfundo
	}
	s = strings.TrimSpace(s)
	switch t.Clase {
	case CInt, CRune, CByte:
		return parseEntero(s, t)
	case CFloat:
		return parseFloat(s, t)
	case CBool:
		return parseBool(s, t)
	case CString:
		return parseTexto(s, t), nil
	case CError:
		return parseError(s)
	case CLista, CArreglo:
		return parseLista(s, t, prof)
	case CMapa:
		return parseMapa(s, t, prof)
	case CStruct:
		return parseEstructura(s, t, prof)
	case CPuntero:
		if s == "nil" {
			return nil, nil
		}
		s = strings.TrimSpace(strings.TrimPrefix(s, "&"))
		return parseEstructura(s, elemDe(t.Elem), prof)
	case CInvalida:
		v, _, err := Inferir(s)
		return v, err
	}
	return nil, fmt.Errorf("no sé leer valores de tipo %s", t.Go())
}

// desenvolver quita una conversión "Nombre(x)" cuando Nombre es el tipo esperado o un tipo básico.
func desenvolver(s string, t Tipo) string {
	if !strings.HasSuffix(s, ")") {
		return s
	}
	i := strings.IndexByte(s, '(')
	if i <= 0 {
		return s
	}
	nombre := strings.TrimSpace(s[:i])
	if nombre == t.Nombre || nombre == t.nombre() || nombre == t.Go() || tiposBasicos[nombre] != CInvalida {
		if cierre(s, i) == len(s)-1 {
			return strings.TrimSpace(s[i+1 : len(s)-1])
		}
	}
	return s
}

func parseEntero(s string, t Tipo) (Valor, error) {
	orig := s
	s = desenvolver(s, t)
	if s == "" {
		return nil, fmt.Errorf("falta el número")
	}
	var i int64
	var err error
	switch {
	case len(s) >= 2 && (s[0] == '\'' || strings.HasPrefix(s, "‘")):
		r, e := leerRunaCitada(s)
		if e != nil {
			return nil, e
		}
		i = int64(r)
	case t.Clase != CInt && utf8.RuneCountInString(s) == 1 && !(t.Clase == CByte && s[0] >= '0' && s[0] <= '9'):
		// a bare character: 'a' for runes and bytes; for bytes a lone digit is the number (FormatoGo writes 0..9 so)
		r, _ := utf8.DecodeRuneInString(s)
		i = int64(r)
	default:
		i, err = leerEntero(s)
		if err != nil {
			if f, e := leerDecimal(s); e == nil && f == math.Trunc(f) && math.Abs(f) < 1<<63 {
				i, err = int64(f), nil
			}
		}
		if err != nil {
			if (t.Clase == CRune || t.Clase == CByte) && utf8.RuneCountInString(s) == 1 {
				r, _ := utf8.DecodeRuneInString(s)
				i, err = int64(r), nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("«%s» no es un número entero", orig)
		}
	}
	return ajustarEntero(i, t)
}

func ajustarEntero(i int64, t Tipo) (Valor, error) {
	lo, hi := rangoEntero(t)
	if i < lo || i > hi {
		return nil, fmt.Errorf("%d no cabe en %s (de %d a %d)", i, t.Go(), lo, hi)
	}
	return int(i), nil
}

// leerEntero reads a base-10 integer with an optional sign and "_" separators, or a Go literal with a
// 0x, 0o or 0b prefix.
func leerEntero(s string) (int64, error) {
	cuerpo := strings.TrimLeft(s, "+-")
	if len(cuerpo) >= 2 && cuerpo[0] == '0' && strings.ContainsRune("xXoObB", rune(cuerpo[1])) {
		return strconv.ParseInt(s, 0, 64)
	}
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	return strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64)
}

// esNumeroDecimal reports whether s only has the characters of an ordinary decimal number.
func esNumeroDecimal(s string) bool {
	digitos := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digitos = true
		case c == '+' || c == '-' || c == '.' || c == ',' || c == 'e' || c == 'E' || c == '_':
		default:
			return false
		}
	}
	return digitos
}

// leerDecimal reads "3.5", "3,5", "-0.25", "1e3", "1_000.5" and fractions "3/4".
func leerDecimal(s string) (float64, error) {
	if strings.Count(s, "/") == 1 {
		r, ok := new(big.Rat).SetString(strings.ReplaceAll(s, " ", ""))
		if ok {
			f, _ := r.Float64()
			return f, nil
		}
	}
	if !esNumeroDecimal(s) {
		return 0, fmt.Errorf("«%s» no es un número", s)
	}
	if strings.Count(s, ",") == 1 && !strings.Contains(s, ".") {
		s = strings.Replace(s, ",", ".", 1)
	}
	s = strings.ReplaceAll(s, "_", "")
	return strconv.ParseFloat(s, 64)
}

func parseFloat(s string, t Tipo) (Valor, error) {
	orig := s
	s = desenvolver(s, t)
	switch strings.ToLower(s) {
	case "nan", "math.nan()":
		return math.NaN(), nil
	case "inf", "+inf", "math.inf(1)", "math.inf(+1)", "infinito", "+infinito":
		return math.Inf(1), nil
	case "-inf", "math.inf(-1)", "-infinito":
		return math.Inf(-1), nil
	}
	f, err := leerDecimal(s)
	if err != nil {
		return nil, fmt.Errorf("«%s» no es un número decimal", orig)
	}
	return redondearFloat(f, t), nil
}

// redondearFloat rounds to float32 precision when t is float32 (values are held as float64).
func redondearFloat(f float64, t Tipo) float64 {
	if t.nombre() == "float32" && !t.Definido {
		return float64(float32(f))
	}
	return f
}

func parseBool(s string, t Tipo) (Valor, error) {
	s = desenvolver(s, t)
	switch Normalizar(s) {
	case "true", "verdadero", "cierto", "si", "v":
		return true, nil
	case "false", "falso", "no", "f":
		return false, nil
	}
	return nil, fmt.Errorf("«%s» no es verdadero ni falso", s)
}

// comillas pairs opening and closing quote characters.
var comillas = map[rune]rune{'"': '"', '`': '`', '“': '”', '«': '»', '\'': '\'', '‘': '’', '„': '“'}

// quitarComillas returns the text inside matching quotes, if s is quoted.
func quitarComillas(s string) (string, bool) {
	if len(s) < 2 {
		return s, false
	}
	a, na := utf8.DecodeRuneInString(s)
	z, nz := utf8.DecodeLastRuneInString(s)
	cierra, ok := comillas[a]
	if !ok || z != cierra || len(s) < na+nz {
		return s, false
	}
	if a == '"' || a == '`' {
		if u, err := strconv.Unquote(s); err == nil {
			return u, true
		}
	}
	return s[na : len(s)-nz], true
}

func parseTexto(s string, t Tipo) Valor {
	if u, ok := quitarComillas(s); ok {
		return u
	}
	if t.Definido {
		d := desenvolver(s, t)
		if d != s {
			if u, ok := quitarComillas(d); ok {
				return u
			}
		}
	}
	return s
}

func parseError(s string) (Valor, error) {
	if s == "nil" || s == "" {
		return nil, nil
	}
	for _, pre := range []string{"errors.New(", "fmt.Errorf(", "error("} {
		if strings.HasPrefix(s, pre) && strings.HasSuffix(s, ")") {
			s = strings.TrimSpace(s[len(pre) : len(s)-1])
			break
		}
	}
	if u, ok := quitarComillas(s); ok {
		return ErrorV(u), nil
	}
	return ErrorV(s), nil
}

func leerRunaCitada(s string) (rune, error) {
	if u, err := strconv.Unquote(s); err == nil {
		r, n := utf8.DecodeRuneInString(u)
		if n == len(u) && n > 0 {
			return r, nil
		}
	}
	if u, ok := quitarComillas(s); ok {
		if r, n := utf8.DecodeRuneInString(u); n == len(u) && n > 0 {
			return r, nil
		}
		if r, _, tail, err := strconv.UnquoteChar(u, '\''); err == nil && tail == "" {
			return r, nil
		}
	}
	return 0, fmt.Errorf("«%s» no es un solo carácter", s)
}

// ---- composite literals ----

// cierre returns the index of the bracket that closes the one at s[i], or -1. Quotes are skipped.
func cierre(s string, i int) int {
	var pila []byte
	for j := i; j < len(s); j++ {
		c := s[j]
		switch c {
		case '"', '`', '\'':
			if c == '\'' && !esApertura(s, j) {
				continue
			}
			k := saltarCita(s, j)
			if k < 0 {
				if c == '\'' {
					continue
				}
				return -1
			}
			j = k
		case '(', '[', '{':
			pila = append(pila, c)
		case ')', ']', '}':
			if len(pila) == 0 || pila[len(pila)-1] != abreDe(c) {
				return -1
			}
			pila = pila[:len(pila)-1]
			if len(pila) == 0 {
				return j
			}
		}
	}
	return -1
}

func abreDe(c byte) byte {
	switch c {
	case ')':
		return '('
	case ']':
		return '['
	}
	return '{'
}

// saltarCita returns the index of the quote closing the one at s[i], or -1.
func saltarCita(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		if s[j] == '\\' && q != '`' {
			j++
			continue
		}
		if s[j] == q {
			return j
		}
	}
	return -1
}

// cuerpoLiteral splits "Tipo{…}" / "[…]" / "{…}" into the prefix and the inside of the final brackets.
func cuerpoLiteral(s string) (prefijo, cuerpo string, abre byte, ok bool) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '`', '\'':
			if c == '\'' && !esApertura(s, i) {
				continue
			}
			k := saltarCita(s, i)
			if k < 0 {
				if c == '\'' {
					continue
				}
				return "", "", 0, false
			}
			i = k
		case '[', '{', '(':
			k := cierre(s, i)
			if k < 0 {
				return "", "", 0, false
			}
			if k == len(s)-1 && c != '(' {
				return strings.TrimSpace(s[:i]), s[i+1 : k], c, true
			}
			i = k
		}
	}
	return "", "", 0, false
}

// partir splits s at top-level separators sep (outside quotes and brackets).
// With sep == ' ', any run of whitespace separates.
func partir(s string, sep byte) []string {
	var out []string
	prof := 0
	inicio := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '`' || (c == '\'' && esApertura(s, i)):
			if k := saltarCita(s, i); k >= 0 {
				i = k
			}
		case c == '(' || c == '[' || c == '{':
			prof++
		case c == ')' || c == ']' || c == '}':
			prof--
		case prof == 0 && (c == sep || (sep == ' ' && (c == '\t' || c == '\n' || c == '\r'))):
			out = append(out, s[inicio:i])
			inicio = i + 1
		}
	}
	out = append(out, s[inicio:])
	if sep == ' ' {
		var limpio []string
		for _, p := range out {
			if strings.TrimSpace(p) != "" {
				limpio = append(limpio, strings.TrimSpace(p))
			}
		}
		return limpio
	}
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	if len(out) > 0 && out[len(out)-1] == "" { // trailing comma, as Go allows
		out = out[:len(out)-1]
	}
	return out
}

// esApertura: a single quote opens a rune/string literal only at the start of an element
// (so "don't" inside a bare word is not a quote).
func esApertura(s string, i int) bool {
	if i == 0 {
		return true
	}
	p := s[i-1]
	return p == ' ' || p == ',' || p == '[' || p == '{' || p == '(' || p == ':' || p == '\t' || p == '\n'
}

// indiceDosPuntos returns the index of the first top-level ':' in s, or -1.
func indiceDosPuntos(s string) int {
	prof := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '`' || (c == '\'' && esApertura(s, i)):
			if k := saltarCita(s, i); k >= 0 {
				i = k
			}
		case c == '(' || c == '[' || c == '{':
			prof++
		case c == ')' || c == ']' || c == '}':
			prof--
		case c == ':' && prof == 0:
			return i
		}
	}
	return -1
}

// Modes for elementos.
const (
	sinEspacios   = iota // only commas separate
	conEspacios          // without a top-level comma, whitespace separates (fmt style "[1 2 3]")
	paresEspacios        // like conEspacios, but only if every piece is a "clave:valor" pair ("map[a:1 b:2]")
)

// elementos returns the element texts of a list/map/struct body: comma separated, or whitespace
// separated (fmt style) when there is no top-level comma, as modo allows.
func elementos(cuerpo string, modo int) []string {
	cuerpo = strings.TrimSpace(cuerpo)
	if cuerpo == "" {
		return []string{}
	}
	partes := partir(cuerpo, ',')
	if len(partes) != 1 || modo == sinEspacios {
		return partes
	}
	trozos := partir(cuerpo, ' ')
	if modo == paresEspacios {
		for _, t := range trozos {
			if i := indiceDosPuntos(t); i <= 0 || i == len(t)-1 {
				return partes
			}
		}
	}
	return trozos
}

func parseLista(s string, t Tipo, prof int) (Valor, error) {
	e := elemDe(t.Elem)
	s = desenvolver(s, t)
	if s == "nil" {
		if t.Clase == CArreglo {
			return nil, fmt.Errorf("un arreglo no puede ser nil")
		}
		return []Valor(nil), nil
	}
	var partes []string
	if _, cuerpo, _, ok := cuerpoLiteral(s); ok {
		modo := sinEspacios
		if e.Basico() {
			modo = conEspacios
		}
		partes = elementos(cuerpo, modo)
	} else if e.Clase == CInt || e.Clase == CFloat {
		// bare "1 2 3" or "1, 2, 3" (floats: "1,5 2,5" uses spaces)
		if strings.ContainsAny(s, " \t;") {
			partes = partir(strings.ReplaceAll(s, ";", " "), ' ')
		} else {
			partes = partir(s, ',')
		}
	} else if s == "" {
		partes = []string{}
	} else {
		return nil, fmt.Errorf("«%s» no parece una lista; escríbela entre corchetes, como [1, 2, 3]", recortar(s))
	}
	if t.Clase == CArreglo && len(partes) > t.Largo {
		return nil, fmt.Errorf("el arreglo %s admite %d elementos y hay %d", t.Go(), t.Largo, len(partes))
	}
	out := make([]Valor, 0, len(partes))
	for i, p := range partes {
		v, err := parseValor(p, e, prof+1)
		if err != nil {
			return nil, fmt.Errorf("elemento %d: %w", i+1, err)
		}
		out = append(out, v)
	}
	if t.Clase == CArreglo {
		for len(out) < t.Largo { // Go fills missing array elements with zero values
			out = append(out, ValorCero(e))
		}
	}
	return out, nil
}

func parseMapa(s string, t Tipo, prof int) (Valor, error) {
	s = desenvolver(s, t)
	if s == "nil" {
		return Mapa(nil), nil
	}
	_, cuerpo, _, ok := cuerpoLiteral(s)
	if !ok {
		return nil, fmt.Errorf("«%s» no parece un mapa; escríbelo como {\"a\": 1, \"b\": 2}", recortar(s))
	}
	k, e := elemDe(t.Clave), elemDe(t.Elem)
	m := Mapa{}
	for _, p := range elementos(cuerpo, paresEspacios) {
		i := indiceDosPuntos(p)
		if i < 0 {
			return nil, fmt.Errorf("en «%s» falta «:» entre la clave y el valor", recortar(p))
		}
		kv, err := parseValor(p[:i], k, prof+1)
		if err != nil {
			return nil, fmt.Errorf("clave: %w", err)
		}
		vv, err := parseValor(p[i+1:], e, prof+1)
		if err != nil {
			return nil, fmt.Errorf("valor de %s: %w", FormatoHumano(kv, k), err)
		}
		m = append(m, Par{K: kv, V: vv})
	}
	return m.Ordenada(), nil
}

func parseEstructura(s string, t Tipo, prof int) (Valor, error) {
	if t.Clase != CStruct {
		return nil, fmt.Errorf("se esperaba una estructura")
	}
	_, cuerpo, _, ok := cuerpoLiteral(s)
	if !ok {
		return nil, fmt.Errorf("«%s» no parece una estructura; escríbela como {X: 1, Y: 2}", recortar(s))
	}
	modo := sinEspacios
	switch {
	case indiceDosPuntos(cuerpo) >= 0:
		modo = paresEspacios
	case len(t.Campos) > 1:
		modo = conEspacios
	}
	partes := elementos(cuerpo, modo)
	out := make(Estructura, len(t.Campos))
	for i, c := range t.Campos {
		out[i] = ValorCero(c.Tipo)
	}
	if len(partes) == 0 {
		return out, nil
	}
	if indiceDosPuntos(partes[0]) >= 0 {
		for _, p := range partes {
			i := indiceDosPuntos(p)
			if i < 0 {
				return nil, fmt.Errorf("mezclas campos con nombre y sin nombre en «%s»", recortar(s))
			}
			nombre := strings.TrimSpace(p[:i])
			j := buscarCampo(t, nombre)
			if j < 0 {
				return nil, fmt.Errorf("%s no tiene el campo %s", t.Go(), nombre)
			}
			v, err := parseValor(p[i+1:], t.Campos[j].Tipo, prof+1)
			if err != nil {
				return nil, fmt.Errorf("campo %s: %w", nombre, err)
			}
			out[j] = v
		}
		return out, nil
	}
	if len(partes) != len(t.Campos) {
		return nil, fmt.Errorf("%s tiene %d campos y escribiste %d valores", t.Go(), len(t.Campos), len(partes))
	}
	for i, p := range partes {
		v, err := parseValor(p, t.Campos[i].Tipo, prof+1)
		if err != nil {
			return nil, fmt.Errorf("campo %s: %w", t.Campos[i].Nombre, err)
		}
		out[i] = v
	}
	return out, nil
}

func buscarCampo(t Tipo, nombre string) int {
	for i, c := range t.Campos {
		if c.Nombre == nombre {
			return i
		}
	}
	for i, c := range t.Campos {
		if strings.EqualFold(c.Nombre, nombre) {
			return i
		}
	}
	return -1
}

func recortar(s string) string {
	if utf8.RuneCountInString(s) <= 40 {
		return s
	}
	n := 0
	for i := range s {
		if n == 40 {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

// ValorCero returns the canonical zero value of t (0, 0.0, false, "", nil, zero struct, zero array).
func ValorCero(t Tipo) Valor {
	switch t.Clase {
	case CInt, CRune, CByte:
		return 0
	case CFloat:
		return 0.0
	case CBool:
		return false
	case CString:
		return ""
	case CArreglo:
		out := make([]Valor, t.Largo)
		for i := range out {
			out[i] = ValorCero(elemDe(t.Elem))
		}
		return out
	case CStruct:
		out := make(Estructura, len(t.Campos))
		for i, c := range t.Campos {
			out[i] = ValorCero(c.Tipo)
		}
		return out
	}
	return nil
}

// ---- inference ----

// Inferir reads a literal with no known type. Preference order: int, float, bool, list (element type unified:
// all ints → []int, any float → []float64, all strings → []string), quoted string, rune 'x', map, bare word → string.
// An empty list gives ListaDe(Tipo{}) (element unknown; Unificar fills it in); "nil" gives (nil, Tipo{}).
func Inferir(s string) (Valor, Tipo, error) {
	return inferir(limpiarTexto(s), 0)
}

func inferir(s string, prof int) (Valor, Tipo, error) {
	if prof > maxProfundidad {
		return nil, Tipo{}, errProfundo
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, Tipo{}, fmt.Errorf("no hay ningún valor")
	}
	if i, err := leerEntero(s); err == nil && esNumeroDecimal(s) {
		return int(i), TInt, nil
	}
	if esNumeroDecimal(s) {
		if f, err := leerDecimal(s); err == nil {
			return f, TFloat, nil
		}
	}
	switch Normalizar(s) {
	case "true", "verdadero", "si":
		return true, TBool, nil
	case "false", "falso", "no":
		return false, TBool, nil
	case "nil":
		return nil, Tipo{}, nil
	}
	if prefijo, cuerpo, abre, ok := cuerpoLiteral(s); ok {
		if prefijo != "" && prefijo != "map" {
			t, err := ParseTipo(prefijo)
			if err != nil {
				return nil, Tipo{}, err
			}
			v, err := parseValor(s, t, prof+1)
			return v, t, err
		}
		if prefijo == "map" || (abre == '{' && indiceDosPuntos(cuerpo) >= 0) ||
			(abre == '[' && indiceDosPuntos(cuerpo) >= 0 && strings.HasPrefix(s, "map")) {
			return inferirMapa(cuerpo, prof)
		}
		return inferirLista(cuerpo, prof)
	}
	if u, ok := quitarComillas(s); ok {
		if s[0] == '\'' || strings.HasPrefix(s, "‘") {
			if r, n := utf8.DecodeRuneInString(u); n == len(u) && n > 0 {
				return int(r), TRune, nil
			}
			if r, err := leerRunaCitada(s); err == nil {
				return int(r), TRune, nil
			}
		}
		return u, TString, nil
	}
	// numbers separated by commas or spaces, without brackets: "1, 2, 3" or "1 2 3"
	for _, sep := range []byte{',', ' '} {
		partes := partir(s, sep)
		if len(partes) < 2 {
			continue
		}
		todos := true
		for _, p := range partes {
			if !esNumeroDecimal(p) {
				todos = false
				break
			}
		}
		if todos {
			return inferirLista(strings.Join(partes, ","), prof)
		}
	}
	return s, TString, nil
}

func inferirLista(cuerpo string, prof int) (Valor, Tipo, error) {
	partes := elementos(cuerpo, conEspacios)
	out := make([]Valor, 0, len(partes))
	var e Tipo
	for i, p := range partes {
		v, t, err := inferir(p, prof+1)
		if err != nil {
			return nil, Tipo{}, fmt.Errorf("elemento %d: %w", i+1, err)
		}
		u, ok := Unificar(e, t)
		if !ok {
			return nil, Tipo{}, fmt.Errorf("la lista mezcla %s y %s", e.Humano(), t.Humano())
		}
		e = u
		out = append(out, v)
	}
	for i := range out {
		v, err := Ajustar(out[i], e)
		if err != nil {
			return nil, Tipo{}, err
		}
		out[i] = v
	}
	return out, ListaDe(e), nil
}

func inferirMapa(cuerpo string, prof int) (Valor, Tipo, error) {
	var k, e Tipo
	m := Mapa{}
	for _, p := range elementos(cuerpo, paresEspacios) {
		i := indiceDosPuntos(p)
		if i < 0 {
			return nil, Tipo{}, fmt.Errorf("en «%s» falta «:» entre la clave y el valor", recortar(p))
		}
		kv, kt, err := inferir(p[:i], prof+1)
		if err != nil {
			return nil, Tipo{}, err
		}
		vv, vt, err := inferir(p[i+1:], prof+1)
		if err != nil {
			return nil, Tipo{}, err
		}
		var ok bool
		if k, ok = Unificar(k, kt); !ok {
			return nil, Tipo{}, fmt.Errorf("las claves del mapa mezclan tipos")
		}
		if e, ok = Unificar(e, vt); !ok {
			return nil, Tipo{}, fmt.Errorf("los valores del mapa mezclan tipos")
		}
		m = append(m, Par{K: kv, V: vv})
	}
	switch k.Clase {
	case CInt, CString, CRune, CByte, CBool, CInvalida:
	default:
		return nil, Tipo{}, fmt.Errorf("las claves de un mapa deben ser números, textos, caracteres o verdadero/falso")
	}
	t := MapaDe(k, e)
	v, err := Ajustar(m, t)
	return v, t, err
}

// Unificar returns the most specific type compatible with both (int+float → float; []int + [] → []int).
// An invalid (unknown) Tipo, or a nil Elem/Clave, is compatible with anything.
func Unificar(a, b Tipo) (Tipo, bool) {
	if a.Clase == CInvalida {
		return b, true
	}
	if b.Clase == CInvalida {
		return a, true
	}
	if a.Clase != b.Clase {
		if a.Clase == CInt && b.Clase == CFloat && !a.Definido {
			return b, true
		}
		if a.Clase == CFloat && b.Clase == CInt && !b.Definido {
			return a, true
		}
		return Tipo{}, false
	}
	if a.Definido || b.Definido {
		if !(a.Definido && b.Definido && a.Nombre == b.Nombre) {
			return Tipo{}, false
		}
	}
	if a.Basico() || a.Clase == CError {
		if a.nombre() != b.nombre() {
			return Tipo{}, false
		}
		return a, true
	}
	r := a
	switch a.Clase {
	case CLista, CArreglo, CPuntero:
		if a.Clase == CArreglo && a.Largo != b.Largo {
			return Tipo{}, false
		}
		e, ok := Unificar(elemDe(a.Elem), elemDe(b.Elem))
		if !ok {
			return Tipo{}, false
		}
		r.Elem = &e
	case CMapa:
		k, ok := Unificar(elemDe(a.Clave), elemDe(b.Clave))
		if !ok {
			return Tipo{}, false
		}
		e, ok := Unificar(elemDe(a.Elem), elemDe(b.Elem))
		if !ok {
			return Tipo{}, false
		}
		r.Clave, r.Elem = &k, &e
	case CStruct:
		if len(a.Campos) != len(b.Campos) {
			return Tipo{}, false
		}
		r.Campos = make([]Campo, len(a.Campos))
		for i := range a.Campos {
			if a.Campos[i].Nombre != b.Campos[i].Nombre {
				return Tipo{}, false
			}
			c, ok := Unificar(a.Campos[i].Tipo, b.Campos[i].Tipo)
			if !ok {
				return Tipo{}, false
			}
			r.Campos[i] = Campo{Nombre: a.Campos[i].Nombre, Tipo: c}
		}
	}
	return r, true
}

// Ajustar coerces a value after inference: int→float64, []Valor of ints → elements typed, a one-letter
// string → rune, text → ErrorV; it checks integer, rune and byte ranges and array lengths.
// It returns a new value; v is not modified.
func Ajustar(v Valor, t Tipo) (Valor, error) {
	return ajustar(v, t, 0)
}

func ajustar(v Valor, t Tipo, prof int) (Valor, error) {
	if prof > maxProfundidad {
		return nil, errProfundo
	}
	if t.Clase == CInvalida {
		return Copiar(v), nil
	}
	switch t.Clase {
	case CInt, CRune, CByte:
		if i, ok := comoEntero(v); ok {
			return ajustarEntero(i, t)
		}
		if f, ok := comoFloat(v); ok {
			if f != math.Trunc(f) || math.IsInf(f, 0) || math.IsNaN(f) || math.Abs(f) >= 1<<63 {
				return nil, fmt.Errorf("%v no es un número entero", FormatoHumano(f, TFloat))
			}
			return ajustarEntero(int64(f), t)
		}
		if s, ok := v.(string); ok && t.Clase != CInt && utf8.RuneCountInString(s) == 1 {
			r, _ := utf8.DecodeRuneInString(s)
			return ajustarEntero(int64(r), t)
		}
	case CFloat:
		if f, ok := comoFloat(v); ok {
			if t.nombre() == "float32" && !math.IsInf(f, 0) && !math.IsNaN(f) && math.Abs(f) > math.MaxFloat32 {
				return nil, fmt.Errorf("%v no cabe en float32", f)
			}
			return redondearFloat(f, t), nil
		}
	case CBool:
		if b, ok := v.(bool); ok {
			return b, nil
		}
	case CString:
		if s, ok := v.(string); ok {
			return s, nil
		}
	case CError:
		switch x := v.(type) {
		case nil:
			return nil, nil
		case ErrorV:
			return x, nil
		case string:
			return ErrorV(x), nil
		}
	case CLista, CArreglo:
		if v == nil && t.Clase == CLista {
			return []Valor(nil), nil
		}
		xs, ok := v.([]Valor)
		if !ok {
			break
		}
		if t.Clase == CArreglo && len(xs) != t.Largo {
			return nil, fmt.Errorf("el arreglo %s necesita %d elementos y hay %d", t.Go(), t.Largo, len(xs))
		}
		if xs == nil {
			return []Valor(nil), nil
		}
		out := make([]Valor, len(xs))
		for i, x := range xs {
			y, err := ajustar(x, elemDe(t.Elem), prof+1)
			if err != nil {
				return nil, fmt.Errorf("elemento %d: %w", i+1, err)
			}
			out[i] = y
		}
		return out, nil
	case CMapa:
		if v == nil {
			return Mapa(nil), nil
		}
		m, ok := v.(Mapa)
		if !ok {
			break
		}
		if m == nil {
			return Mapa(nil), nil
		}
		out := make(Mapa, len(m))
		for i, p := range m {
			k, err := ajustar(p.K, elemDe(t.Clave), prof+1)
			if err != nil {
				return nil, fmt.Errorf("clave: %w", err)
			}
			e, err := ajustar(p.V, elemDe(t.Elem), prof+1)
			if err != nil {
				return nil, fmt.Errorf("valor: %w", err)
			}
			out[i] = Par{K: k, V: e}
		}
		return out.Ordenada(), nil
	case CStruct:
		s, ok := v.(Estructura)
		if !ok {
			break
		}
		if len(s) != len(t.Campos) {
			return nil, fmt.Errorf("%s tiene %d campos y el valor tiene %d", t.Go(), len(t.Campos), len(s))
		}
		out := make(Estructura, len(s))
		for i, x := range s {
			y, err := ajustar(x, t.Campos[i].Tipo, prof+1)
			if err != nil {
				return nil, fmt.Errorf("campo %s: %w", t.Campos[i].Nombre, err)
			}
			out[i] = y
		}
		return out, nil
	case CPuntero:
		if v == nil {
			return nil, nil
		}
		return ajustar(v, elemDe(t.Elem), prof+1)
	}
	return nil, fmt.Errorf("%s no es %s", FormatoHumano(v, Tipo{}), t.Humano())
}
