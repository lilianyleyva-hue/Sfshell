package recetas

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Envolver turns a function into a whole program (package main) that reads the function's inputs from
// stdin, calls it and prints the results:
//
//   - ints, floats, bools, strings and runes: one per line (a string is the whole line);
//   - lists: one line of values separated by spaces ([]string as words, []rune as the characters of the line);
//   - [][]T (matrices): one row per line, ending with an empty line;
//   - results: one per line with fmt.Println; a list as values separated by spaces, a matrix one row per
//     line; a non-nil error is printed as "error: …" instead of the other results.
//
// Prompts go to stderr in Spanish, so stdout stays comparable. aCaso converts a function case into a
// program case (Comparar "lineas", or "numeros" when a result holds decimals); it reports false when the
// values cannot be written as input lines (a text with a line break, a word with spaces…).
func Envolver(funcionFuente string, f nucleo.Firma) (programa string, aCaso func(nucleo.Caso) (nucleo.CasoPrograma, bool), err error) {
	if f.Receptor != nil {
		return "", nil, errors.New("no sé envolver un método en un programa: envuelve una función normal")
	}
	if len(f.Res) == 0 {
		return "", nil, errors.New("la función no devuelve nada, así que no hay nada que escribir")
	}
	for i, p := range f.Params {
		if _, ok := lectorDe(p.Tipo, f.Variadica && i == len(f.Params)-1); !ok {
			return "", nil, fmt.Errorf("no sé leer desde la entrada un parámetro de tipo %s (%s)", p.Tipo.Go(), nombreParam(p, i))
		}
	}
	for i, t := range f.Res {
		if !imprimible(t) || (t.Clase == nucleo.CError && i != len(f.Res)-1) {
			return "", nil, fmt.Errorf("no sé escribir un resultado de tipo %s", t.Go())
		}
	}
	src := strings.TrimSpace(funcionFuente)
	if src == "" {
		return "", nil, errors.New("no hay código que envolver")
	}
	fset := token.NewFileSet()
	archivo, perr := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if perr != nil {
		src = "package main\n\n" + src
		archivo, perr = parser.ParseFile(fset, "x.go", src, parser.ParseComments)
		if perr != nil {
			return "", nil, fmt.Errorf("el código de la función no se puede leer: %v", perr)
		}
	}
	encontrada := false
	for _, d := range archivo.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
			switch fd.Name.Name {
			case "main":
				return "", nil, errors.New("el código ya tiene una función main")
			case f.Nombre:
				encontrada = true
			}
		}
	}
	if !encontrada {
		return "", nil, fmt.Errorf("no encuentro la función %s en el código", f.Nombre)
	}
	// the declarations after the package clause and the imports, as written (comments included)
	inicio := archivo.Name.End()
	imports := map[string]string{} // path → spec text
	var orden []string
	for _, d := range archivo.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		if gd.End() > inicio {
			inicio = gd.End()
		}
		for _, sp := range gd.Specs {
			is := sp.(*ast.ImportSpec)
			ruta, _ := strconv.Unquote(is.Path.Value)
			texto := is.Path.Value
			if is.Name != nil {
				texto = is.Name.Name + " " + texto
			}
			clave := ruta
			if is.Name != nil {
				clave = is.Name.Name + " " + ruta
			}
			if _, ya := imports[clave]; !ya {
				imports[clave] = texto
				orden = append(orden, clave)
			}
		}
	}
	cuerpo := src[fset.Position(inicio).Offset:]
	g := &generador{f: f}
	principal := g.main()
	ayudantes := g.ayudantes() // before imports(): it decides whether strconv is needed
	for _, ruta := range g.imports() {
		if _, ya := imports[ruta]; !ya {
			imports[ruta] = strconv.Quote(ruta)
			orden = append(orden, ruta)
		}
	}
	sort.Strings(orden)
	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n")
	for _, k := range orden {
		sb.WriteString("\t" + imports[k] + "\n")
	}
	sb.WriteString(")\n")
	sb.WriteString(cuerpo)
	sb.WriteString("\n\n")
	sb.WriteString(principal)
	sb.WriteString(ayudantes)
	out, ferr := format.Source([]byte(sb.String()))
	if ferr != nil {
		return "", nil, fmt.Errorf("el programa generado no se puede leer: %v", ferr)
	}
	return string(out), func(c nucleo.Caso) (nucleo.CasoPrograma, bool) { return aCasoPrograma(f, c) }, nil
}

func nombreParam(p nucleo.Param, i int) string {
	if p.Nombre != "" && p.Nombre != "_" {
		return p.Nombre
	}
	return "entrada " + strconv.Itoa(i+1)
}

// ---- input readers ----

// lector describes how a parameter is read: which helper, what prompt.
type lector struct {
	ayudante string // helper that returns (T, error) from the reader
	pregunta string
	matriz   bool
}

func lectorDe(t nucleo.Tipo, variadico bool) (lector, bool) {
	switch t.Clase {
	case nucleo.CInt:
		return lector{"envEntero", "un número entero", false}, true
	case nucleo.CFloat:
		return lector{"envDecimal", "un número (puede tener decimales)", false}, true
	case nucleo.CBool:
		return lector{"envBool", "sí o no", false}, true
	case nucleo.CString:
		return lector{"envTexto", "un texto", false}, true
	case nucleo.CRune:
		return lector{"envRuna", "una letra o carácter", false}, true
	case nucleo.CByte:
		return lector{"envByte", "un carácter", false}, true
	case nucleo.CLista, nucleo.CArreglo:
		e := *t.Elem
		switch e.Clase {
		case nucleo.CInt:
			return lector{"envEnteros", "una lista de números enteros separados por espacios", false}, true
		case nucleo.CFloat:
			return lector{"envDecimales", "una lista de números separados por espacios", false}, true
		case nucleo.CBool:
			return lector{"envBools", "una lista de sí o no separados por espacios", false}, true
		case nucleo.CString:
			return lector{"envPalabras", "unas palabras separadas por espacios", false}, true
		case nucleo.CRune:
			return lector{"envRunas", "un texto (se usarán sus caracteres)", false}, true
		case nucleo.CByte:
			return lector{"envBytes", "un texto (se usarán sus bytes)", false}, true
		case nucleo.CLista:
			if t.Clase == nucleo.CLista && !variadico && e.Elem != nil {
				switch e.Elem.Clase {
				case nucleo.CInt:
					return lector{"envMatrizEnteros", "una matriz de números enteros: una fila por línea y una línea vacía al final", true}, true
				case nucleo.CFloat:
					return lector{"envMatrizDecimales", "una matriz de números: una fila por línea y una línea vacía al final", true}, true
				}
			}
		}
	}
	return lector{}, false
}

func imprimible(t nucleo.Tipo) bool {
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat, nucleo.CBool, nucleo.CString, nucleo.CRune, nucleo.CByte, nucleo.CError:
		return true
	case nucleo.CLista, nucleo.CArreglo, nucleo.CMapa, nucleo.CStruct, nucleo.CPuntero:
		return t.Codificable()
	}
	return false
}

// ---- code generation ----

type generador struct {
	f       nucleo.Firma
	usados  map[string]bool
	strconv bool
}

func (g *generador) usar(h string) {
	if g.usados == nil {
		g.usados = map[string]bool{}
	}
	g.usados[h] = true
}

func (g *generador) imports() []string {
	out := []string{"bufio", "fmt", "os", "strings"}
	if g.strconv {
		out = append(out, "strconv")
	}
	return out
}

func (g *generador) main() string {
	var sb strings.Builder
	sb.WriteString("func main() {\n\tentrada := bufio.NewReader(os.Stdin)\n")
	var args []string
	for i, p := range g.f.Params {
		variadico := g.f.Variadica && i == len(g.f.Params)-1
		l, _ := lectorDe(p.Tipo, variadico)
		g.usar(l.ayudante)
		nombre := "e" + strconv.Itoa(i)
		etiqueta := nombreParam(p, i)
		fmt.Fprintf(&sb, "\tfmt.Fprintln(os.Stderr, %s)\n", strconv.Quote("Escribe "+l.pregunta+" ("+etiqueta+"):"))
		fmt.Fprintf(&sb, "\tv%d, err := %s(entrada)\n", i, l.ayudante)
		fmt.Fprintf(&sb, "\tif err != nil {\n\t\tfmt.Fprintln(os.Stderr, %s, err)\n\t\tos.Exit(1)\n\t}\n", strconv.Quote("No entiendo "+etiqueta+":"))
		sb.WriteString("\t" + g.convertir(nombre, "v"+strconv.Itoa(i), p.Tipo) + "\n")
		if variadico {
			args = append(args, nombre+"...")
		} else {
			args = append(args, nombre)
		}
	}
	var res []string
	for i := range g.f.Res {
		res = append(res, "r"+strconv.Itoa(i))
	}
	fmt.Fprintf(&sb, "\t%s := %s(%s)\n", strings.Join(res, ", "), g.f.Nombre, strings.Join(args, ", "))
	ultimo := len(g.f.Res) - 1
	if g.f.Res[ultimo].Clase == nucleo.CError {
		fmt.Fprintf(&sb, "\tif r%d != nil {\n\t\tfmt.Println(\"error:\", r%d)\n\t\treturn\n\t}\n", ultimo, ultimo)
	}
	for i, t := range g.f.Res {
		if t.Clase == nucleo.CError {
			continue
		}
		sb.WriteString("\t" + g.imprimir("r"+strconv.Itoa(i), t) + "\n")
	}
	if len(g.f.Params) == 0 {
		sb.WriteString("\t_ = entrada\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// convertir declares nombre of type t from the helper's value v.
func (g *generador) convertir(nombre, v string, t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat, nucleo.CBool, nucleo.CString, nucleo.CRune, nucleo.CByte:
		if t.Go() == base(t) {
			return nombre + " := " + v
		}
		return nombre + " := " + t.Go() + "(" + v + ")"
	case nucleo.CArreglo:
		return "var " + nombre + " " + t.Go() + "\n\tif len(" + v + ") != " + strconv.Itoa(t.Largo) +
			" {\n\t\tfmt.Fprintln(os.Stderr, " + strconv.Quote("Hacen falta exactamente "+strconv.Itoa(t.Largo)+" valores") + ")\n\t\tos.Exit(1)\n\t}\n" +
			"\tfor i, x := range " + v + " {\n\t\t" + nombre + "[i] = " + conversionElem(*t.Elem, "x") + "\n\t}"
	case nucleo.CLista:
		e := *t.Elem
		if e.Clase == nucleo.CLista {
			ee := *e.Elem
			if t.Go() == "[]"+e.Go() && e.Go() == "[]"+base(ee) {
				return nombre + " := " + v
			}
			return "var " + nombre + " " + t.Go() + "\n\tfor _, fila := range " + v + " {\n\t\tvar f " + e.Go() +
				"\n\t\tfor _, x := range fila {\n\t\t\tf = append(f, " + conversionElem(ee, "x") + ")\n\t\t}\n\t\t" +
				nombre + " = append(" + nombre + ", f)\n\t}"
		}
		if t.Go() == "[]"+base(e) {
			return nombre + " := " + v
		}
		return "var " + nombre + " " + t.Go() + "\n\tfor _, x := range " + v + " {\n\t\t" + nombre + " = append(" + nombre + ", " + conversionElem(e, "x") + ")\n\t}"
	}
	return nombre + " := " + v
}

func conversionElem(e nucleo.Tipo, x string) string {
	if e.Go() == base(e) {
		return x
	}
	return e.Go() + "(" + x + ")"
}

// base is the Go type the helpers return for a class: int, float64, bool, string, rune, byte.
func base(t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CInt:
		return "int"
	case nucleo.CFloat:
		return "float64"
	case nucleo.CBool:
		return "bool"
	case nucleo.CString:
		return "string"
	case nucleo.CRune:
		return "rune"
	case nucleo.CByte:
		return "byte"
	}
	return t.Go()
}

func (g *generador) imprimir(r string, t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CRune, nucleo.CByte:
		return "fmt.Println(string(rune(" + r + ")))"
	case nucleo.CLista, nucleo.CArreglo:
		e := *t.Elem
		switch e.Clase {
		case nucleo.CRune, nucleo.CByte:
			g.usar("envUnirRunas")
			return "fmt.Println(envUnirRunas(" + r + "[:]))"
		case nucleo.CLista, nucleo.CArreglo:
			if e.Elem != nil && e.Elem.Basico() {
				return "for _, fila := range " + r + " {\n\t\tfmt.Println(envUnir(fila[:]))\n\t}"
			}
		}
		if e.Basico() {
			g.usar("envUnir")
			return "fmt.Println(envUnir(" + r + "[:]))"
		}
	}
	return "fmt.Println(" + r + ")"
}

// ayudantes returns the helper functions used by main.
func (g *generador) ayudantes() string {
	var sb strings.Builder
	sb.WriteString(`
// envLinea reads one line without its line break; at the end of the input it fails.
func envLinea(r *bufio.Reader) (string, error) {
	s, err := r.ReadString('\n')
	if err != nil && s == "" {
		return "", fmt.Errorf("no hay más datos")
	}
	return strings.TrimRight(s, "\r\n"), nil
}
`)
	necesita := func(hs ...string) bool {
		for _, h := range hs {
			if g.usados[h] {
				return true
			}
		}
		return false
	}
	if necesita("envEntero", "envEnteros", "envMatrizEnteros") {
		g.strconv = true
		sb.WriteString(`
func envUnEntero(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("«%s» no es un número entero", strings.TrimSpace(s))
	}
	return n, nil
}
`)
	}
	if necesita("envDecimal", "envDecimales", "envMatrizDecimales") {
		g.strconv = true
		sb.WriteString(`
func envUnDecimal(s string) (float64, error) {
	t := strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, fmt.Errorf("«%s» no es un número", strings.TrimSpace(s))
	}
	return f, nil
}
`)
	}
	if necesita("envBool", "envBools") {
		sb.WriteString(`
func envUnBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "verdadero", "si", "sí", "s", "1":
		return true, nil
	case "false", "falso", "no", "n", "0":
		return false, nil
	}
	return false, fmt.Errorf("«%s» no es sí ni no", strings.TrimSpace(s))
}
`)
	}
	escribir := func(h, codigo string) {
		if g.usados[h] {
			sb.WriteString(codigo)
		}
	}
	escribir("envEntero", `
func envEntero(r *bufio.Reader) (int, error) {
	s, err := envLinea(r)
	if err != nil {
		return 0, err
	}
	return envUnEntero(s)
}
`)
	escribir("envDecimal", `
func envDecimal(r *bufio.Reader) (float64, error) {
	s, err := envLinea(r)
	if err != nil {
		return 0, err
	}
	return envUnDecimal(s)
}
`)
	escribir("envBool", `
func envBool(r *bufio.Reader) (bool, error) {
	s, err := envLinea(r)
	if err != nil {
		return false, err
	}
	return envUnBool(s)
}
`)
	escribir("envTexto", `
func envTexto(r *bufio.Reader) (string, error) {
	return envLinea(r)
}
`)
	escribir("envRuna", `
func envRuna(r *bufio.Reader) (rune, error) {
	s, err := envLinea(r)
	if err != nil {
		return 0, err
	}
	for _, c := range s {
		return c, nil
	}
	return 0, fmt.Errorf("la línea está vacía")
}
`)
	escribir("envByte", `
func envByte(r *bufio.Reader) (byte, error) {
	s, err := envLinea(r)
	if err != nil {
		return 0, err
	}
	if s == "" {
		return 0, fmt.Errorf("la línea está vacía")
	}
	return s[0], nil
}
`)
	escribir("envEnteros", `
func envEnteros(r *bufio.Reader) ([]int, error) {
	s, err := envLinea(r)
	if err != nil {
		return nil, err
	}
	var out []int
	for _, c := range strings.Fields(strings.NewReplacer(",", " ", "[", " ", "]", " ").Replace(s)) {
		n, err := envUnEntero(c)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
`)
	escribir("envDecimales", `
func envDecimales(r *bufio.Reader) ([]float64, error) {
	s, err := envLinea(r)
	if err != nil {
		return nil, err
	}
	var out []float64
	for _, c := range strings.Fields(strings.NewReplacer("[", " ", "]", " ").Replace(s)) {
		f, err := envUnDecimal(c)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}
`)
	escribir("envBools", `
func envBools(r *bufio.Reader) ([]bool, error) {
	s, err := envLinea(r)
	if err != nil {
		return nil, err
	}
	var out []bool
	for _, c := range strings.Fields(s) {
		b, err := envUnBool(c)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
`)
	escribir("envPalabras", `
func envPalabras(r *bufio.Reader) ([]string, error) {
	s, err := envLinea(r)
	if err != nil {
		return nil, err
	}
	return strings.Fields(s), nil
}
`)
	escribir("envRunas", `
func envRunas(r *bufio.Reader) ([]rune, error) {
	s, err := envLinea(r)
	if err != nil {
		return nil, err
	}
	return []rune(s), nil
}
`)
	escribir("envBytes", `
func envBytes(r *bufio.Reader) ([]byte, error) {
	s, err := envLinea(r)
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}
`)
	escribir("envMatrizEnteros", `
func envMatrizEnteros(r *bufio.Reader) ([][]int, error) {
	var out [][]int
	for {
		s, err := envLinea(r)
		if err != nil || strings.TrimSpace(s) == "" {
			return out, nil
		}
		var fila []int
		for _, c := range strings.Fields(s) {
			n, err := envUnEntero(c)
			if err != nil {
				return nil, err
			}
			fila = append(fila, n)
		}
		out = append(out, fila)
	}
}
`)
	escribir("envMatrizDecimales", `
func envMatrizDecimales(r *bufio.Reader) ([][]float64, error) {
	var out [][]float64
	for {
		s, err := envLinea(r)
		if err != nil || strings.TrimSpace(s) == "" {
			return out, nil
		}
		var fila []float64
		for _, c := range strings.Fields(s) {
			f, err := envUnDecimal(c)
			if err != nil {
				return nil, err
			}
			fila = append(fila, f)
		}
		out = append(out, fila)
	}
}
`)
	// list printing helpers are generic over the element type through fmt.Sprint
	sb.WriteString(`
// envUnir writes the values separated by spaces.
func envUnir[T any](xs []T) string {
	partes := make([]string, len(xs))
	for i, x := range xs {
		partes[i] = fmt.Sprint(x)
	}
	return strings.Join(partes, " ")
}
`)
	escribir("envUnirRunas", `
// envUnirRunas writes characters separated by spaces.
func envUnirRunas[T rune | byte](xs []T) string {
	partes := make([]string, len(xs))
	for i, x := range xs {
		partes[i] = string(rune(x))
	}
	return strings.Join(partes, " ")
}
`)
	return sb.String()
}

// ---- cases ----

// aCasoPrograma converts a function case into a program case.
func aCasoPrograma(f nucleo.Firma, c nucleo.Caso) (nucleo.CasoPrograma, bool) {
	if len(c.Entradas) != len(f.Params) {
		return nucleo.CasoPrograma{}, false
	}
	var entrada strings.Builder
	for i, p := range f.Params {
		lineas, ok := lineasEntrada(c.Entradas[i], p.Tipo)
		if !ok {
			return nucleo.CasoPrograma{}, false
		}
		for _, l := range lineas {
			entrada.WriteString(l)
			entrada.WriteByte('\n')
		}
	}
	cp := nucleo.CasoPrograma{Entrada: entrada.String(), Comparar: "lineas", Expectativa: c.Expectativa, Nota: c.Nota}
	if !c.ConEsperado() {
		cp.Expectativa = nucleo.EspNinguna
		return cp, true
	}
	if len(c.Esperado) != len(f.Res) {
		return nucleo.CasoPrograma{}, false
	}
	ultimo := len(f.Res) - 1
	if f.Res[ultimo].Clase == nucleo.CError && c.Esperado[ultimo] != nil {
		cp.Esperado, cp.Comparar = "error:", "contiene"
		return cp, true
	}
	var salida []string
	for i, t := range f.Res {
		if t.Clase == nucleo.CError {
			continue
		}
		lineas, ok := lineasSalida(c.Esperado[i], t)
		if !ok {
			return nucleo.CasoPrograma{}, false
		}
		salida = append(salida, lineas...)
		if tieneDecimales(t) {
			cp.Comparar = "numeros"
		}
	}
	cp.Esperado = strings.Join(salida, "\n") + "\n"
	return cp, true
}

func tieneDecimales(t nucleo.Tipo) bool {
	if t.Clase == nucleo.CFloat {
		return true
	}
	if t.Elem != nil && tieneDecimales(*t.Elem) {
		return true
	}
	if t.Clave != nil && tieneDecimales(*t.Clave) {
		return true
	}
	for _, c := range t.Campos {
		if tieneDecimales(c.Tipo) {
			return true
		}
	}
	return false
}

// lineasEntrada writes one input value as the lines the generated program reads.
func lineasEntrada(v nucleo.Valor, t nucleo.Tipo) ([]string, bool) {
	unaLinea := func(s string) ([]string, bool) {
		if strings.ContainsAny(s, "\n\r") {
			return nil, false
		}
		return []string{s}, true
	}
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat, nucleo.CBool:
		return unaLinea(textoBasico(v, t, false))
	case nucleo.CString:
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		return unaLinea(s)
	case nucleo.CRune:
		n, ok := v.(int)
		if !ok || n == '\n' || n == '\r' || !utf8.ValidRune(rune(n)) {
			return nil, false
		}
		return []string{string(rune(n))}, true
	case nucleo.CByte:
		n, ok := v.(int)
		if !ok || n >= utf8.RuneSelf || n == '\n' || n == '\r' {
			return nil, false
		}
		return []string{string(rune(n))}, true
	case nucleo.CLista, nucleo.CArreglo:
		xs, _ := v.([]nucleo.Valor)
		e := *t.Elem
		switch e.Clase {
		case nucleo.CInt, nucleo.CFloat, nucleo.CBool:
			partes := make([]string, len(xs))
			for i, x := range xs {
				partes[i] = textoBasico(x, e, false)
			}
			return []string{strings.Join(partes, " ")}, true
		case nucleo.CString:
			partes := make([]string, len(xs))
			for i, x := range xs {
				s, ok := x.(string)
				if !ok || s == "" || len(strings.Fields(s)) != 1 || strings.TrimSpace(s) != s {
					return nil, false
				}
				partes[i] = s
			}
			return []string{strings.Join(partes, " ")}, true
		case nucleo.CRune, nucleo.CByte:
			var sb strings.Builder
			for _, x := range xs {
				n, ok := x.(int)
				if !ok || n == '\n' || n == '\r' || (e.Clase == nucleo.CByte && n >= utf8.RuneSelf) {
					return nil, false
				}
				sb.WriteRune(rune(n))
			}
			return []string{sb.String()}, true
		case nucleo.CLista:
			if e.Elem == nil || (e.Elem.Clase != nucleo.CInt && e.Elem.Clase != nucleo.CFloat) {
				return nil, false
			}
			var out []string
			for _, fila := range xs {
				ys, _ := fila.([]nucleo.Valor)
				if len(ys) == 0 {
					return nil, false // an empty row would end the matrix
				}
				partes := make([]string, len(ys))
				for i, y := range ys {
					partes[i] = textoBasico(y, *e.Elem, false)
				}
				out = append(out, strings.Join(partes, " "))
			}
			return append(out, ""), true
		}
	}
	return nil, false
}

// lineasSalida writes an expected result as the generated program prints it.
func lineasSalida(v nucleo.Valor, t nucleo.Tipo) ([]string, bool) {
	switch t.Clase {
	case nucleo.CRune, nucleo.CByte:
		n, ok := v.(int)
		if !ok {
			return nil, false
		}
		return strings.Split(string(rune(n)), "\n"), true
	case nucleo.CString:
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		return strings.Split(s, "\n"), true
	case nucleo.CLista, nucleo.CArreglo:
		xs, _ := v.([]nucleo.Valor)
		e := *t.Elem
		switch e.Clase {
		case nucleo.CRune, nucleo.CByte:
			partes := make([]string, len(xs))
			for i, x := range xs {
				n, _ := x.(int)
				partes[i] = string(rune(n))
			}
			return []string{strings.Join(partes, " ")}, true
		case nucleo.CLista, nucleo.CArreglo:
			if e.Elem != nil && e.Elem.Basico() {
				var out []string
				for _, fila := range xs {
					ys, _ := fila.([]nucleo.Valor)
					partes := make([]string, len(ys))
					for i, y := range ys {
						partes[i] = textoV(y, *e.Elem)
					}
					out = append(out, strings.Join(partes, " "))
				}
				return out, true
			}
		}
		if e.Basico() {
			partes := make([]string, len(xs))
			for i, x := range xs {
				partes[i] = textoV(x, e)
			}
			return []string{strings.Join(partes, " ")}, true
		}
	}
	s, ok := textoValor(v, t)
	if !ok {
		return nil, false
	}
	return strings.Split(s, "\n"), true
}

// textoBasico writes a basic value: for input (sinGo=false) floats use the shortest form.
func textoBasico(v nucleo.Valor, t nucleo.Tipo, _ bool) string {
	switch t.Clase {
	case nucleo.CFloat:
		f, _ := v.(float64)
		if i, ok := v.(int); ok {
			f = float64(i)
		}
		return strconv.FormatFloat(f, 'g', -1, 64)
	case nucleo.CBool:
		b, _ := v.(bool)
		return strconv.FormatBool(b)
	}
	return textoV(v, t)
}

// textoV is fmt's %v of a value of type t (as fmt.Println writes it).
func textoV(v nucleo.Valor, t nucleo.Tipo) string {
	s, _ := textoValor(v, t)
	return s
}

func textoValor(v nucleo.Valor, t nucleo.Tipo) (string, bool) {
	switch t.Clase {
	case nucleo.CInt, nucleo.CRune, nucleo.CByte:
		switch x := v.(type) {
		case int:
			return strconv.Itoa(x), true
		case float64:
			return strconv.FormatInt(int64(x), 10), true
		}
		return "", false
	case nucleo.CFloat:
		var f float64
		switch x := v.(type) {
		case float64:
			f = x
		case int:
			f = float64(x)
		default:
			return "", false
		}
		if t.Go() == "float32" || (t.Definido && t.Nombre == "float32") {
			return fmt.Sprint(float32(f)), true
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return fmt.Sprint(f), true
		}
		return fmt.Sprint(f), true
	case nucleo.CBool:
		b, ok := v.(bool)
		return strconv.FormatBool(b), ok
	case nucleo.CString:
		s, ok := v.(string)
		return s, ok
	case nucleo.CError:
		if v == nil {
			return "<nil>", true
		}
		return fmt.Sprint(v), true
	case nucleo.CLista, nucleo.CArreglo:
		xs, _ := v.([]nucleo.Valor)
		partes := make([]string, len(xs))
		for i, x := range xs {
			s, ok := textoValor(x, *t.Elem)
			if !ok {
				return "", false
			}
			partes[i] = s
		}
		return "[" + strings.Join(partes, " ") + "]", true
	case nucleo.CMapa:
		m, _ := v.(nucleo.Mapa)
		m = m.Ordenada() // the same order as fmt: numbers by value, texts by bytes, false before true
		partes := make([]string, len(m))
		for i, p := range m {
			k, ok1 := textoValor(p.K, *t.Clave)
			x, ok2 := textoValor(p.V, *t.Elem)
			if !ok1 || !ok2 {
				return "", false
			}
			partes[i] = k + ":" + x
		}
		return "map[" + strings.Join(partes, " ") + "]", true
	case nucleo.CStruct:
		s, _ := v.(nucleo.Estructura)
		if len(s) != len(t.Campos) {
			return "", false
		}
		partes := make([]string, len(s))
		for i, c := range t.Campos {
			if c.Tipo.Clase == nucleo.CPuntero {
				return "", false // fmt prints the address
			}
			x, ok := textoValor(s[i], c.Tipo)
			if !ok {
				return "", false
			}
			partes[i] = x
		}
		return "{" + strings.Join(partes, " ") + "}", true
	case nucleo.CPuntero:
		if v == nil {
			return "<nil>", true
		}
		s, ok := textoValor(v, *t.Elem)
		return "&" + s, ok
	}
	return "", false
}
