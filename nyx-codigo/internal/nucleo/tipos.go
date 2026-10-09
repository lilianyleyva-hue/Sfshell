// Package nucleo holds the shared contract of Nyx Código: the type universe,
// dynamic values and their wire codec, signatures and test cases, the sandbox
// contract, the reasoning trace, parsed requests and answers.
//
// It imports only the standard library (see §3.1 of the spec for the exact list).
package nucleo

import (
	"fmt"
	"strconv"
	"strings"
)

// Clase is the kind of a Go type that Nyx Código can generate values for, encode, compare and show.
type Clase uint8

const (
	CInvalida Clase = iota
	CInt            // int, int8..int64, uint..uint64 (held as int; uint64 > MaxInt64 unsupported)
	CFloat          // float32, float64 (held as float64)
	CBool           // bool
	CString         // string
	CRune           // rune / int32 spelled "rune" (held as int; shown 'a')
	CByte           // byte / uint8 spelled "byte" (held as int; shown 'a' if printable)
	CLista          // []Elem (held as []Valor; nil slice = nil)
	CArreglo        // [Largo]Elem (held as []Valor of len Largo)
	CMapa           // map[Clave]Elem; Clave must be CInt/CString/CRune/CByte/CBool (held as Mapa)
	CStruct         // struct, named or anonymous (held as Estructura)
	CPuntero        // *Elem where Elem is CStruct (held as nil or Estructura)
	CError          // error, results only (held as nil or ErrorV)
)

var nombresClase = [...]string{
	CInvalida: "invalida", CInt: "int", CFloat: "float", CBool: "bool", CString: "string",
	CRune: "rune", CByte: "byte", CLista: "lista", CArreglo: "arreglo", CMapa: "mapa",
	CStruct: "struct", CPuntero: "puntero", CError: "error",
}

func (c Clase) String() string {
	if int(c) < len(nombresClase) {
		return nombresClase[c]
	}
	return "clase(" + strconv.Itoa(int(c)) + ")"
}

// Tipo is a recursive description of a Go type.
type Tipo struct {
	Clase    Clase   `json:"clase"`
	Nombre   string  `json:"nombre,omitempty"`   // exact spelling: "int", "int64", "uint8", "rune", "float32"; or a defined type name "Punto"
	Definido bool    `json:"definido,omitempty"` // Nombre is a user-defined type (type Celsius float64; type Punto struct{…})
	Elem     *Tipo   `json:"elem,omitempty"`     // CLista, CArreglo, CMapa (value), CPuntero
	Clave    *Tipo   `json:"clave,omitempty"`    // CMapa key
	Largo    int     `json:"largo,omitempty"`    // CArreglo
	Campos   []Campo `json:"campos,omitempty"`   // CStruct, declaration order
}

type Campo struct {
	Nombre string `json:"nombre"`
	Tipo   Tipo   `json:"tipo"`
}

var (
	TInt    = Tipo{Clase: CInt, Nombre: "int"}
	TFloat  = Tipo{Clase: CFloat, Nombre: "float64"}
	TBool   = Tipo{Clase: CBool, Nombre: "bool"}
	TString = Tipo{Clase: CString, Nombre: "string"}
	TRune   = Tipo{Clase: CRune, Nombre: "rune"}
	TByte   = Tipo{Clase: CByte, Nombre: "byte"}
	TError  = Tipo{Clase: CError, Nombre: "error"}
)

// ListaDe returns []e.
func ListaDe(e Tipo) Tipo { return Tipo{Clase: CLista, Elem: &e} }

// ArregloDe returns [n]e.
func ArregloDe(n int, e Tipo) Tipo { return Tipo{Clase: CArreglo, Largo: n, Elem: &e} }

// MapaDe returns map[k]v.
func MapaDe(k, v Tipo) Tipo { return Tipo{Clase: CMapa, Clave: &k, Elem: &v} }

// PunteroA returns *e (e must be CStruct for the type to be Codificable).
func PunteroA(e Tipo) Tipo { return Tipo{Clase: CPuntero, Elem: &e} }

// nombreBase is the default spelling of a basic class.
func nombreBase(c Clase) string {
	switch c {
	case CInt:
		return "int"
	case CFloat:
		return "float64"
	case CBool:
		return "bool"
	case CString:
		return "string"
	case CRune:
		return "rune"
	case CByte:
		return "byte"
	case CError:
		return "error"
	}
	return ""
}

// nombre returns Nombre, or the default spelling of a basic class when Nombre is empty.
func (t Tipo) nombre() string {
	if t.Nombre != "" {
		return t.Nombre
	}
	return nombreBase(t.Clase)
}

func elemDe(p *Tipo) Tipo {
	if p == nil {
		return Tipo{}
	}
	return *p
}

// Go returns the Go spelling: "[]int", "map[string][]int", "Punto", "*Punto", "[3]float64", "struct{ X int }".
func (t Tipo) Go() string {
	if t.Definido && t.Nombre != "" {
		return t.Nombre
	}
	switch t.Clase {
	case CInt, CFloat, CBool, CString, CRune, CByte, CError:
		return t.nombre()
	case CLista:
		return "[]" + elemDe(t.Elem).Go()
	case CArreglo:
		return "[" + strconv.Itoa(t.Largo) + "]" + elemDe(t.Elem).Go()
	case CMapa:
		return "map[" + elemDe(t.Clave).Go() + "]" + elemDe(t.Elem).Go()
	case CPuntero:
		return "*" + elemDe(t.Elem).Go()
	case CStruct:
		if len(t.Campos) == 0 {
			return "struct{}"
		}
		partes := make([]string, len(t.Campos))
		for i, c := range t.Campos {
			partes[i] = c.Nombre + " " + c.Tipo.Go()
		}
		return "struct{ " + strings.Join(partes, "; ") + " }"
	}
	return "<tipo inválido>"
}

// Cero returns a zero-value expression: "0", "0.0", "false", `""`, "nil", "Punto{}", "[3]int{}".
// Defined basic types are converted explicitly ("Celsius(0.0)") so the expression has the right type.
func (t Tipo) Cero() string {
	var base string
	switch t.Clase {
	case CInt, CRune, CByte:
		base = "0"
	case CFloat:
		base = "0.0"
	case CBool:
		base = "false"
	case CString:
		base = `""`
	case CLista, CMapa:
		base = "nil"
	case CPuntero, CError:
		return "nil"
	case CArreglo, CStruct:
		return t.Go() + "{}"
	default:
		return "nil"
	}
	if t.Definido && t.Nombre != "" {
		return t.Nombre + "(" + base + ")"
	}
	return base
}

// Igual is structural equality (names included). An empty Nombre on a basic class equals its default spelling.
func (t Tipo) Igual(u Tipo) bool {
	if t.Clase != u.Clase || t.Definido != u.Definido || t.Largo != u.Largo || t.nombre() != u.nombre() {
		return false
	}
	if !igualPtr(t.Elem, u.Elem) || !igualPtr(t.Clave, u.Clave) {
		return false
	}
	if len(t.Campos) != len(u.Campos) {
		return false
	}
	for i := range t.Campos {
		if t.Campos[i].Nombre != u.Campos[i].Nombre || !t.Campos[i].Tipo.Igual(u.Campos[i].Tipo) {
			return false
		}
	}
	return true
}

func igualPtr(a, b *Tipo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Igual(*b)
}

// ClaveTipo returns a canonical string usable as a map key: Go() spelled out completely, so that
// defined types also carry their underlying structure ("Punto=struct{ X int; Y int }").
// ClaveTipo(t) == ClaveTipo(u) exactly when t.Igual(u).
//
// The spec (§3.1) calls this method Clave(), but Tipo already has a field named Clave (the map key
// type) and Go forbids a field and a method with the same name, so the method is ClaveTipo.
func (t Tipo) ClaveTipo() string {
	var sb strings.Builder
	t.escribirClave(&sb)
	return sb.String()
}

func (t Tipo) escribirClave(sb *strings.Builder) {
	if t.Definido {
		sb.WriteString(t.Nombre)
		sb.WriteString("=")
		u := t
		u.Definido = false
		u.Nombre = ""
		if t.Clase == CStruct || t.Clase == CLista || t.Clase == CArreglo || t.Clase == CMapa || t.Clase == CPuntero {
			u.escribirClave(sb)
			return
		}
		// defined basic type: keep the underlying spelling, which is unknown here; the class name is enough
		sb.WriteString(t.Clase.String())
		return
	}
	switch t.Clase {
	case CInt, CFloat, CBool, CString, CRune, CByte, CError:
		sb.WriteString(t.nombre())
		if t.Clase == CInt && (t.nombre() == "rune" || t.nombre() == "byte") {
			sb.WriteString("#int") // distinct from the CRune/CByte classes
		}
	case CLista:
		sb.WriteString("[]")
		elemDe(t.Elem).escribirClave(sb)
	case CArreglo:
		sb.WriteString("[" + strconv.Itoa(t.Largo) + "]")
		elemDe(t.Elem).escribirClave(sb)
	case CMapa:
		sb.WriteString("map[")
		elemDe(t.Clave).escribirClave(sb)
		sb.WriteString("]")
		elemDe(t.Elem).escribirClave(sb)
	case CPuntero:
		sb.WriteString("*")
		elemDe(t.Elem).escribirClave(sb)
	case CStruct:
		sb.WriteString("struct{")
		for i, c := range t.Campos {
			if i > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(c.Nombre)
			sb.WriteString(" ")
			c.Tipo.escribirClave(sb)
		}
		sb.WriteString("}")
	default:
		sb.WriteString("?")
	}
}

// Codificable reports whether every part is supported by the wire codec (§3.3).
// CError is supported only at the top level (as a result).
func (t Tipo) Codificable() bool { return t.codificable(true) }

func (t Tipo) codificable(arriba bool) bool {
	switch t.Clase {
	case CInt, CFloat, CBool, CString, CRune, CByte:
		return true
	case CError:
		return arriba
	case CLista:
		return t.Elem != nil && t.Elem.codificable(false)
	case CArreglo:
		return t.Elem != nil && t.Largo >= 0 && t.Elem.codificable(false)
	case CMapa:
		if t.Clave == nil || t.Elem == nil {
			return false
		}
		switch t.Clave.Clase {
		case CInt, CString, CRune, CByte, CBool:
		default:
			return false
		}
		return t.Clave.codificable(false) && t.Elem.codificable(false)
	case CStruct:
		for _, c := range t.Campos {
			if !c.Tipo.codificable(false) {
				return false
			}
		}
		return true
	case CPuntero:
		return t.Elem != nil && t.Elem.Clase == CStruct && t.Elem.codificable(false)
	}
	return false
}

// Basico reports CInt, CFloat, CBool, CString, CRune, CByte.
func (t Tipo) Basico() bool {
	switch t.Clase {
	case CInt, CFloat, CBool, CString, CRune, CByte:
		return true
	}
	return false
}

// Humano describes the type in simple Spanish: "lista de números enteros", "texto", "número decimal",
// "mapa de texto a número".
func (t Tipo) Humano() string { return t.humano(false) }

func (t Tipo) humano(plural bool) string {
	elegir := func(sing, plu string) string {
		if plural {
			return plu
		}
		return sing
	}
	switch t.Clase {
	case CInt:
		return elegir("número entero", "números enteros")
	case CFloat:
		return elegir("número decimal", "números decimales")
	case CBool:
		return elegir("verdadero o falso", "valores verdadero o falso")
	case CString:
		return elegir("texto", "textos")
	case CRune:
		return elegir("carácter", "caracteres")
	case CByte:
		return elegir("byte", "bytes")
	case CError:
		return elegir("error", "errores")
	case CLista:
		return elegir("lista de ", "listas de ") + elemDe(t.Elem).humano(true)
	case CArreglo:
		return elegir("arreglo de ", "arreglos de ") + strconv.Itoa(t.Largo) + " " + elemDe(t.Elem).humano(t.Largo != 1)
	case CMapa:
		return elegir("mapa de ", "mapas de ") + elemDe(t.Clave).corto() + " a " + elemDe(t.Elem).corto()
	case CPuntero:
		return elegir("puntero a ", "punteros a ") + elemDe(t.Elem).humano(false)
	case CStruct:
		if t.Nombre != "" {
			return elegir("estructura ", "estructuras ") + t.Nombre
		}
		return elegir("estructura", "estructuras")
	}
	return "tipo desconocido"
}

// corto is the short singular form used inside map descriptions.
func (t Tipo) corto() string {
	if t.Clase == CInt {
		return "número"
	}
	return t.humano(false)
}

// ParseTipo reads "int", "[]string", "map[string][]int", "[3]float64", "error", "[][]int",
// "struct{ X int; Y string }" and "*struct{…}". Named types give an error (analisis builds those from go/types).
func ParseTipo(s string) (Tipo, error) {
	p := &lectorTipo{s: s}
	t, err := p.tipo()
	if err != nil {
		return Tipo{}, err
	}
	p.espacios()
	if p.i != len(p.s) {
		return Tipo{}, fmt.Errorf("tipo %q: sobra texto al final (%q)", s, p.s[p.i:])
	}
	return t, nil
}

var tiposBasicos = map[string]Clase{
	"int": CInt, "int8": CInt, "int16": CInt, "int32": CInt, "int64": CInt,
	"uint": CInt, "uint8": CInt, "uint16": CInt, "uint32": CInt, "uint64": CInt, "uintptr": CInt,
	"float32": CFloat, "float64": CFloat, "bool": CBool, "string": CString,
	"rune": CRune, "byte": CByte, "error": CError,
}

type lectorTipo struct {
	s string
	i int
}

func (p *lectorTipo) espacios() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n') {
		p.i++
	}
}

func (p *lectorTipo) prefijo(x string) bool {
	p.espacios()
	if strings.HasPrefix(p.s[p.i:], x) {
		p.i += len(x)
		return true
	}
	return false
}

func (p *lectorTipo) ident() string {
	p.espacios()
	j := p.i
	for j < len(p.s) {
		c := p.s[j]
		if c == '_' || c == '.' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c >= 0x80 {
			j++
			continue
		}
		break
	}
	id := p.s[p.i:j]
	p.i = j
	return id
}

func (p *lectorTipo) tipo() (Tipo, error) {
	p.espacios()
	if p.i >= len(p.s) {
		return Tipo{}, fmt.Errorf("tipo %q: falta el tipo", p.s)
	}
	switch {
	case p.prefijo("["):
		p.espacios()
		if p.prefijo("]") {
			e, err := p.tipo()
			if err != nil {
				return Tipo{}, err
			}
			return ListaDe(e), nil
		}
		j := p.i
		for j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
			j++
		}
		if j == p.i {
			return Tipo{}, fmt.Errorf("tipo %q: el tamaño del arreglo debe ser un número", p.s)
		}
		n, err := strconv.Atoi(p.s[p.i:j])
		if err != nil {
			return Tipo{}, fmt.Errorf("tipo %q: tamaño de arreglo no válido", p.s)
		}
		p.i = j
		if !p.prefijo("]") {
			return Tipo{}, fmt.Errorf("tipo %q: falta «]»", p.s)
		}
		e, err := p.tipo()
		if err != nil {
			return Tipo{}, err
		}
		return ArregloDe(n, e), nil
	case p.prefijo("*"):
		e, err := p.tipo()
		if err != nil {
			return Tipo{}, err
		}
		if e.Clase != CStruct {
			return Tipo{}, fmt.Errorf("tipo %q: solo sé usar punteros a estructuras", p.s)
		}
		return PunteroA(e), nil
	case p.prefijo("map["):
		k, err := p.tipo()
		if err != nil {
			return Tipo{}, err
		}
		if !p.prefijo("]") {
			return Tipo{}, fmt.Errorf("tipo %q: falta «]» después de la clave del mapa", p.s)
		}
		v, err := p.tipo()
		if err != nil {
			return Tipo{}, err
		}
		return MapaDe(k, v), nil
	case p.prefijo("chan"), p.prefijo("<-chan"):
		return Tipo{}, fmt.Errorf("tipo %q: no sé trabajar con canales", p.s)
	case p.prefijo("func"):
		return Tipo{}, fmt.Errorf("tipo %q: no sé trabajar con funciones como valores", p.s)
	case p.prefijo("interface"), p.prefijo("any"):
		return Tipo{}, fmt.Errorf("tipo %q: no sé trabajar con interfaces (solo error)", p.s)
	case p.prefijo("struct"):
		return p.estructura()
	}
	id := p.ident()
	if id == "" {
		return Tipo{}, fmt.Errorf("tipo %q: no entiendo «%s»", p.s, p.s[p.i:])
	}
	if c, ok := tiposBasicos[id]; ok {
		return Tipo{Clase: c, Nombre: id}, nil
	}
	p.espacios()
	if p.i < len(p.s) && p.s[p.i] == '[' {
		return Tipo{}, fmt.Errorf("tipo %q: no sé trabajar con genéricos", p.s)
	}
	return Tipo{}, fmt.Errorf("tipo %q: «%s» es un tipo con nombre; necesito su declaración", p.s, id)
}

func (p *lectorTipo) estructura() (Tipo, error) {
	if !p.prefijo("{") {
		return Tipo{}, fmt.Errorf("tipo %q: falta «{» después de struct", p.s)
	}
	t := Tipo{Clase: CStruct}
	for {
		p.espacios()
		if p.prefijo("}") {
			return t, nil
		}
		if p.prefijo(";") {
			continue
		}
		var nombres []string
		for {
			id := p.ident()
			if id == "" {
				return Tipo{}, fmt.Errorf("tipo %q: falta el nombre de un campo", p.s)
			}
			nombres = append(nombres, id)
			if !p.prefijo(",") {
				break
			}
		}
		ct, err := p.tipo()
		if err != nil {
			return Tipo{}, err
		}
		for _, n := range nombres {
			t.Campos = append(t.Campos, Campo{Nombre: n, Tipo: ct})
		}
		p.espacios()
		if p.i < len(p.s) && p.s[p.i] == '`' { // struct tag: skip
			j := strings.IndexByte(p.s[p.i+1:], '`')
			if j < 0 {
				return Tipo{}, fmt.Errorf("tipo %q: etiqueta sin cerrar", p.s)
			}
			p.i += j + 2
		}
		if p.i >= len(p.s) {
			return Tipo{}, fmt.Errorf("tipo %q: falta «}»", p.s)
		}
	}
}

// rangoEntero returns the inclusive range of an integer type spelled n (CInt, CRune or CByte).
func rangoEntero(t Tipo) (lo, hi int64) {
	n := t.nombre()
	switch t.Clase {
	case CRune:
		return -1 << 31, 1<<31 - 1
	case CByte:
		return 0, 255
	}
	switch n {
	case "int8":
		return -128, 127
	case "int16":
		return -1 << 15, 1<<15 - 1
	case "int32", "rune":
		return -1 << 31, 1<<31 - 1
	case "uint8", "byte":
		return 0, 255
	case "uint16":
		return 0, 1<<16 - 1
	case "uint32":
		return 0, 1<<32 - 1
	case "uint", "uint64", "uintptr":
		return 0, 1<<63 - 1
	}
	return -1 << 63, 1<<63 - 1
}

func sinSigno(t Tipo) bool {
	lo, _ := rangoEntero(t)
	return lo == 0
}
