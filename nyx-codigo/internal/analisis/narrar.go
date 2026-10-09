package analisis

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Frase is one sentence of a narration.
type Frase struct {
	Linea int    `json:"linea"`
	Texto string `json:"texto"`
	Nivel int    `json:"nivel"` // 1 = simple, 2 = detail
}

// codigo prints a node as gofmt does ("n%2 == 0").
func codigo(n ast.Node) string {
	if n == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), n); err != nil {
		return ""
	}
	s := buf.String()
	if strings.Contains(s, "\n") {
		s = strings.Join(strings.Fields(s), " ")
	}
	return s
}

// ---- expressions in Spanish ----

// ExprEnEspanol says an expression in simple Spanish: "x % 2 == 0" → "x es par",
// "len(xs) == 0" → "xs no tiene elementos", "strings.ToUpper(s)" → "s en mayúsculas".
func ExprEnEspanol(e ast.Expr) string {
	return (&lector{}).es(e)
}

// lector turns expressions into Spanish; with marcar, code is written between backticks.
type lector struct {
	marcar bool
	a      *archivo
}

func (l *lector) m(s string) string {
	if l.marcar {
		return "`" + s + "`"
	}
	return s
}

func (l *lector) c(e ast.Expr) string { return l.m(codigo(e)) }

// sujeto is the Spanish for an operand: identifiers and simple code are marked, everything else is read.
func (l *lector) sujeto(e ast.Expr) string {
	switch x := quitarParen(e).(type) {
	case *ast.Ident:
		switch x.Name {
		case "true":
			return "verdadero"
		case "false":
			return "falso"
		case "nil":
			return "nil"
		}
		return l.m(x.Name)
	case *ast.BasicLit:
		if x.Kind == token.INT || x.Kind == token.FLOAT {
			return x.Value
		}
		return l.m(x.Value)
	case *ast.SelectorExpr, *ast.IndexExpr:
		if s, ok := l.especial(x); ok {
			return s
		}
		return l.c(x)
	case *ast.BinaryExpr:
		switch x.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
			return l.es(x)
		}
		return l.c(x)
	}
	return l.es(e)
}

func esLit(e ast.Expr, v string) bool {
	b, ok := quitarParen(e).(*ast.BasicLit)
	return ok && b.Value == v
}

func esIdent(e ast.Expr, nombre string) bool {
	id, ok := quitarParen(e).(*ast.Ident)
	return ok && id.Name == nombre
}

var comparaciones = map[token.Token]string{
	token.EQL: "es igual a", token.NEQ: "es distinto de", token.LSS: "es menor que",
	token.LEQ: "es menor o igual que", token.GTR: "es mayor que", token.GEQ: "es mayor o igual que",
}

// es is the full reading of an expression.
func (l *lector) es(e ast.Expr) string {
	switch x := e.(type) {
	case nil:
		return ""
	case *ast.ParenExpr:
		return l.es(x.X)
	case *ast.Ident:
		return l.sujeto(x)
	case *ast.BasicLit:
		return l.sujeto(x)
	case *ast.BinaryExpr:
		return l.binaria(x)
	case *ast.UnaryExpr:
		switch x.Op {
		case token.NOT:
			if id, ok := quitarParen(x.X).(*ast.Ident); ok {
				return l.m(id.Name) + " es falso"
			}
			return "no se cumple que " + l.es(x.X)
		case token.SUB:
			return "menos " + l.sujeto(x.X)
		case token.AND:
			return "la dirección de " + l.sujeto(x.X)
		case token.ARROW:
			return "lo que llega por el canal " + l.sujeto(x.X)
		}
	case *ast.StarExpr:
		return "el valor al que apunta " + l.sujeto(x.X)
	case *ast.CallExpr:
		if s, ok := l.llamada(x); ok {
			return s
		}
		return "el resultado de " + l.c(x)
	case *ast.IndexExpr, *ast.SelectorExpr:
		if s, ok := l.especial(x); ok {
			return s
		}
		return l.c(x)
	case *ast.SliceExpr:
		switch {
		case x.Low != nil && x.High != nil:
			return "el trozo de " + l.sujeto(x.X) + " desde la posición " + l.sujeto(x.Low) + " hasta antes de la " + l.sujeto(x.High)
		case x.Low != nil:
			return l.sujeto(x.X) + " desde la posición " + l.sujeto(x.Low)
		case x.High != nil:
			return "los primeros " + l.sujeto(x.High) + " elementos de " + l.sujeto(x.X)
		}
		return "una copia de " + l.sujeto(x.X)
	case *ast.CompositeLit:
		if len(x.Elts) == 0 {
			switch x.Type.(type) {
			case *ast.ArrayType:
				return "una lista vacía"
			case *ast.MapType:
				return "un mapa vacío"
			}
			return l.c(x) + " (vacío)"
		}
		return l.c(x)
	case *ast.FuncLit:
		return "una función"
	case *ast.TypeAssertExpr:
		return l.sujeto(x.X) + " visto como " + l.c(x.Type)
	}
	return l.c(e)
}

func (l *lector) binaria(x *ast.BinaryExpr) string {
	switch x.Op {
	case token.LAND:
		return l.es(x.X) + " y " + l.es(x.Y)
	case token.LOR:
		return l.es(x.X) + " o " + l.es(x.Y)
	case token.ADD:
		return l.sujeto(x.X) + " más " + l.sujeto(x.Y)
	case token.SUB:
		return l.sujeto(x.X) + " menos " + l.sujeto(x.Y)
	case token.MUL:
		if codigo(x.X) == codigo(x.Y) {
			return l.sujeto(x.X) + " al cuadrado"
		}
		return l.sujeto(x.X) + " por " + l.sujeto(x.Y)
	case token.QUO:
		return l.sujeto(x.X) + " entre " + l.sujeto(x.Y)
	case token.REM:
		return "el resto de dividir " + l.sujeto(x.X) + " entre " + l.sujeto(x.Y)
	}
	frase, ok := comparaciones[x.Op]
	if !ok {
		return l.c(x)
	}
	izq, der := quitarParen(x.X), quitarParen(x.Y)
	// parity and divisibility: x%2 == 0, x%k == 0
	if r, ok := izq.(*ast.BinaryExpr); ok && r.Op == token.REM {
		if esLit(r.Y, "2") {
			switch {
			case x.Op == token.EQL && esLit(der, "0"), x.Op == token.NEQ && esLit(der, "1"):
				return l.sujeto(r.X) + " es par"
			case x.Op == token.NEQ && esLit(der, "0"), x.Op == token.EQL && esLit(der, "1"):
				return l.sujeto(r.X) + " es impar"
			}
		}
		if esLit(der, "0") {
			switch x.Op {
			case token.EQL:
				return l.sujeto(r.X) + " es divisible por " + l.sujeto(r.Y)
			case token.NEQ:
				return l.sujeto(r.X) + " no es divisible por " + l.sujeto(r.Y)
			}
		}
	}
	// len(xs) == 0
	if call, ok := izq.(*ast.CallExpr); ok && esLen(call) && esLit(der, "0") {
		xs := l.sujeto(call.Args[0])
		switch x.Op {
		case token.EQL, token.LEQ:
			return xs + " no tiene elementos"
		case token.NEQ, token.GTR:
			return xs + " tiene elementos"
		}
	}
	// err != nil
	if esIdent(der, "nil") {
		if esIdent(izq, "err") {
			if x.Op == token.NEQ {
				return "hay un error"
			}
			return "no hay error"
		}
		if x.Op == token.NEQ {
			return l.sujeto(izq) + " no es nil"
		}
		return l.sujeto(izq) + " es nil"
	}
	if b, ok := der.(*ast.BasicLit); ok && b.Kind == token.STRING && (b.Value == `""` || b.Value == "``") {
		if x.Op == token.EQL {
			return l.sujeto(izq) + " es el texto vacío"
		}
		if x.Op == token.NEQ {
			return l.sujeto(izq) + " no es el texto vacío"
		}
	}
	if esLit(der, "0") {
		switch x.Op {
		case token.EQL:
			return l.sujeto(izq) + " es cero"
		case token.NEQ:
			return l.sujeto(izq) + " no es cero"
		case token.GTR:
			return l.sujeto(izq) + " es positivo"
		case token.LSS:
			return l.sujeto(izq) + " es negativo"
		case token.GEQ:
			return l.sujeto(izq) + " no es negativo"
		case token.LEQ:
			return l.sujeto(izq) + " no es positivo"
		}
	}
	if esIdent(der, "true") && x.Op == token.EQL {
		return l.sujeto(izq) + " es verdadero"
	}
	if esIdent(der, "false") && x.Op == token.EQL {
		return l.sujeto(izq) + " es falso"
	}
	return l.sujeto(izq) + " " + frase + " " + l.sujeto(der)
}

// especial reads indexing and selectors with a known shape.
func (l *lector) especial(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.IndexExpr:
		xs := l.sujeto(x.X)
		if esLit(x.Index, "0") {
			return "el primer elemento de " + xs, true
		}
		if b, ok := quitarParen(x.Index).(*ast.BinaryExpr); ok && b.Op == token.SUB && esUno(b.Y) {
			if call, ok := quitarParen(b.X).(*ast.CallExpr); ok && esLen(call) && codigo(call.Args[0]) == codigo(x.X) {
				return "el último elemento de " + xs, true
			}
		}
		if l.a != nil {
			if t := l.a.tipo(x.X); t != nil {
				if _, ok := t.Underlying().(*types.Map); ok {
					return "el valor de " + l.sujeto(x.Index) + " en el mapa " + xs, true
				}
			}
		}
		return l.c(x), true
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			switch id.Name + "." + x.Sel.Name {
			case "math.MaxInt", "math.MaxInt64":
				return "el entero más grande", true
			case "math.MinInt", "math.MinInt64":
				return "el entero más pequeño", true
			case "math.Pi":
				return "pi", true
			case "os.Args":
				return "los argumentos del programa", true
			}
		}
		return l.c(x), true
	}
	return "", false
}

// llamada reads known calls.
func (l *lector) llamada(x *ast.CallExpr) (string, bool) {
	arg := func(i int) string {
		if i < len(x.Args) {
			return l.sujeto(x.Args[i])
		}
		return "?"
	}
	nombre := codigo(x.Fun)
	switch nombre {
	case "len":
		return "la longitud de " + arg(0), true
	case "cap":
		return "la capacidad de " + arg(0), true
	case "append":
		if len(x.Args) == 2 && x.Ellipsis.IsValid() {
			return arg(0) + " seguido de los elementos de " + arg(1), true
		}
		if len(x.Args) == 2 {
			return arg(0) + " con " + arg(1) + " añadido al final", true
		}
		return arg(0) + " con más elementos al final", true
	case "make":
		if len(x.Args) > 0 {
			switch x.Args[0].(type) {
			case *ast.MapType:
				return "un mapa vacío", true
			case *ast.ArrayType:
				if len(x.Args) > 1 {
					return "una lista de " + arg(1) + " elementos a cero", true
				}
			case *ast.ChanType:
				return "un canal nuevo", true
			}
		}
	case "new":
		return "un " + arg(0) + " nuevo a cero", true
	case "min":
		return "el menor entre " + l.listaArgs(x.Args), true
	case "max":
		return "el mayor entre " + l.listaArgs(x.Args), true
	case "string":
		return arg(0) + " como texto", true
	case "int", "int64", "int32":
		return arg(0) + " como número entero", true
	case "float64", "float32":
		return arg(0) + " como número decimal", true
	case "[]rune":
		return "los caracteres de " + arg(0), true
	case "[]byte":
		return "los bytes de " + arg(0), true
	case "strings.ToUpper":
		return arg(0) + " en mayúsculas", true
	case "strings.ToLower":
		return arg(0) + " en minúsculas", true
	case "strings.TrimSpace":
		return arg(0) + " sin espacios al principio ni al final", true
	case "strings.Trim":
		return arg(0) + " sin " + arg(1) + " a los lados", true
	case "strings.Contains":
		return arg(0) + " contiene " + arg(1), true
	case "strings.ContainsRune":
		return arg(0) + " contiene el carácter " + arg(1), true
	case "strings.HasPrefix":
		return arg(0) + " empieza por " + arg(1), true
	case "strings.HasSuffix":
		return arg(0) + " termina en " + arg(1), true
	case "strings.Split":
		return arg(0) + " partido por " + arg(1), true
	case "strings.Fields":
		return "las palabras de " + arg(0), true
	case "strings.Join":
		return "los textos de " + arg(0) + " unidos con " + arg(1), true
	case "strings.Repeat":
		return arg(0) + " repetido " + arg(1) + " veces", true
	case "strings.Replace", "strings.ReplaceAll":
		return arg(0) + " cambiando " + arg(1) + " por " + arg(2), true
	case "strings.Index":
		return "la posición de " + arg(1) + " en " + arg(0), true
	case "strings.Count":
		return "cuántas veces aparece " + arg(1) + " en " + arg(0), true
	case "strings.EqualFold":
		return arg(0) + " es igual a " + arg(1) + " sin mirar mayúsculas", true
	case "strings.Title":
		return arg(0) + " con la primera letra de cada palabra en mayúscula", true
	case "strconv.Itoa", "strconv.FormatInt", "fmt.Sprint":
		return arg(0) + " convertido a texto", true
	case "strconv.Atoi":
		return arg(0) + " convertido a número", true
	case "strconv.ParseFloat":
		return arg(0) + " convertido a número decimal", true
	case "fmt.Sprintf":
		return "el texto con formato " + arg(0), true
	case "math.Sqrt":
		return "la raíz cuadrada de " + arg(0), true
	case "math.Abs":
		return "el valor absoluto de " + arg(0), true
	case "math.Pow":
		return arg(0) + " elevado a " + arg(1), true
	case "math.Max":
		return "el mayor entre " + arg(0) + " y " + arg(1), true
	case "math.Min":
		return "el menor entre " + arg(0) + " y " + arg(1), true
	case "math.Floor":
		return arg(0) + " redondeado hacia abajo", true
	case "math.Ceil":
		return arg(0) + " redondeado hacia arriba", true
	case "math.Round":
		return arg(0) + " redondeado", true
	case "unicode.IsLetter":
		return arg(0) + " es una letra", true
	case "unicode.IsDigit", "unicode.IsNumber":
		return arg(0) + " es un dígito", true
	case "unicode.IsUpper":
		return arg(0) + " es una mayúscula", true
	case "unicode.IsLower":
		return arg(0) + " es una minúscula", true
	case "unicode.IsSpace":
		return arg(0) + " es un espacio", true
	case "unicode.IsPunct":
		return arg(0) + " es un signo de puntuación", true
	case "unicode.ToUpper":
		return arg(0) + " en mayúscula", true
	case "unicode.ToLower":
		return arg(0) + " en minúscula", true
	case "utf8.RuneCountInString":
		return "el número de caracteres de " + arg(0), true
	case "slices.Contains":
		return arg(0) + " contiene " + arg(1), true
	case "slices.Index":
		return "la posición de " + arg(1) + " en " + arg(0), true
	case "slices.Max":
		return "el mayor elemento de " + arg(0), true
	case "slices.Min":
		return "el menor elemento de " + arg(0), true
	case "errors.New", "fmt.Errorf":
		return "un error que dice " + arg(0), true
	}
	if at, ok := x.Fun.(*ast.ArrayType); ok && at.Len == nil {
		switch codigo(at.Elt) {
		case "rune":
			return "los caracteres de " + arg(0), true
		case "byte":
			return "los bytes de " + arg(0), true
		}
	}
	return "", false
}

func (l *lector) listaArgs(args []ast.Expr) string {
	partes := make([]string, len(args))
	for i, a := range args {
		partes[i] = l.sujeto(a)
	}
	return unirY(partes)
}

func unirY(partes []string) string {
	switch len(partes) {
	case 0:
		return ""
	case 1:
		return partes[0]
	}
	return strings.Join(partes[:len(partes)-1], ", ") + " y " + partes[len(partes)-1]
}

// subjuntivo turns "x es menor que n" into "x sea menor que n" (after "mientras").
func subjuntivo(s string) string {
	r := strings.NewReplacer(" es ", " sea ", " está ", " esté ", " no tiene ", " no tenga ", " tiene ", " tenga ", " contiene ", " contenga ", " hay ", " haya ")
	return r.Replace(" " + s)[1:]
}

func mayuscula(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

func minuscula(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 || s[0] == '`' {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

func sinPunto(s string) string {
	return strings.TrimRight(s, ".:…")
}

// ---- roles ----

// Roles classifies the local variables of a function: "acumulador", "contador", "bandera", "índice",
// "resultado" or "auxiliar".
func Roles(fuente, funcion string) (map[string]string, error) {
	a, err := parsear(fuente)
	if err != nil {
		return nil, err
	}
	fd := buscarFunc(a.f, funcion)
	if fd == nil {
		return nil, fmt.Errorf("no encuentro la función %s", funcion)
	}
	a.comprobarTipos()
	return rolesDe(a, fd), nil
}

func rolesDe(a *archivo, fd *ast.FuncDecl) map[string]string {
	out := map[string]string{}
	if fd.Body == nil {
		return out
	}
	definidas := map[string]token.Pos{} // name → definition position
	booleana := map[string]bool{}
	indices := map[string]bool{} // loop variables
	definir := func(id *ast.Ident, valor ast.Expr) {
		if id == nil || id.Name == "_" {
			return
		}
		if _, ya := definidas[id.Name]; !ya {
			definidas[id.Name] = id.Pos()
		}
		if esIdent(valor, "true") || esIdent(valor, "false") {
			booleana[id.Name] = true
		}
		if t := a.tipo(id); t != nil {
			if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.Bool {
				booleana[id.Name] = true
			}
		}
	}
	if fd.Type.Results != nil {
		for _, c := range fd.Type.Results.List {
			for _, n := range c.Names {
				definir(n, nil)
			}
		}
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for i, l := range x.Lhs {
					id, _ := l.(*ast.Ident)
					var v ast.Expr
					if len(x.Rhs) == len(x.Lhs) {
						v = x.Rhs[i]
					}
					definir(id, v)
				}
			}
		case *ast.ValueSpec:
			for i, id := range x.Names {
				var v ast.Expr
				if i < len(x.Values) {
					v = x.Values[i]
				}
				definir(id, v)
				if esIdent(x.Type, "bool") {
					booleana[id.Name] = true
				}
			}
		case *ast.RangeStmt:
			if id, ok := x.Key.(*ast.Ident); ok && x.Tok == token.DEFINE {
				definir(id, nil)
				indices[id.Name] = true
			}
			if id, ok := x.Value.(*ast.Ident); ok && x.Tok == token.DEFINE {
				definir(id, nil)
			}
		case *ast.ForStmt:
			if as, ok := x.Init.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
				for _, l := range as.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						indices[id.Name] = true
					}
				}
			}
		}
		return true
	})
	contador := map[string]bool{}
	acumulador := map[string]bool{}
	otraActualizacion := map[string]bool{}
	bandera := map[string]bool{}
	usadoComoIndice := map[string]bool{}
	devuelto := map[string]bool{}
	recorrer(fd.Body, func(n ast.Node, anc pila) bool {
		bucle := bucleExterior(anc)
		antesDelBucle := func(nombre string) bool {
			p, ok := definidas[nombre]
			return ok && bucle != nil && p < bucle.Pos()
		}
		switch x := n.(type) {
		case *ast.IncDecStmt:
			if id, ok := x.X.(*ast.Ident); ok && bucle != nil {
				contador[id.Name] = true
			}
		case *ast.AssignStmt:
			if len(x.Lhs) != 1 || len(x.Rhs) != 1 {
				break
			}
			id, ok := x.Lhs[0].(*ast.Ident)
			if !ok {
				break
			}
			switch x.Tok {
			case token.ADD_ASSIGN, token.SUB_ASSIGN:
				if bucle != nil && esUno(x.Rhs[0]) {
					contador[id.Name] = true
				} else if antesDelBucle(id.Name) {
					acumulador[id.Name] = true
				}
			case token.MUL_ASSIGN, token.QUO_ASSIGN, token.OR_ASSIGN, token.AND_ASSIGN, token.XOR_ASSIGN:
				if antesDelBucle(id.Name) {
					acumulador[id.Name] = true
				}
			case token.ASSIGN:
				rhs := quitarParen(x.Rhs[0])
				if (esIdent(rhs, "true") || esIdent(rhs, "false")) && dentroDeIf(anc) {
					bandera[id.Name] = true
				}
				if antesDelBucle(id.Name) {
					if b, ok := rhs.(*ast.BinaryExpr); ok && (esIdent(b.X, id.Name) || esIdent(b.Y, id.Name)) {
						acumulador[id.Name] = true
					} else if call, ok := rhs.(*ast.CallExpr); ok && esIdent(call.Fun, "append") && len(call.Args) > 0 && esIdent(call.Args[0], id.Name) {
						acumulador[id.Name] = true
					} else if !esIdent(rhs, "true") && !esIdent(rhs, "false") {
						otraActualizacion[id.Name] = true
					}
				}
			}
		case *ast.IndexExpr:
			if id, ok := quitarParen(x.Index).(*ast.Ident); ok {
				usadoComoIndice[id.Name] = true
			}
		case *ast.ReturnStmt:
			for _, r := range x.Results {
				if id, ok := quitarParen(r).(*ast.Ident); ok {
					devuelto[id.Name] = true
				}
			}
		}
		return true
	})
	for nombre := range definidas {
		switch {
		case contador[nombre] && !acumulador[nombre] && !indices[nombre]:
			out[nombre] = "contador"
		case acumulador[nombre]:
			out[nombre] = "acumulador"
		case bandera[nombre] && booleana[nombre]:
			out[nombre] = "bandera"
		case indices[nombre] && usadoComoIndice[nombre]:
			out[nombre] = "índice"
		case devuelto[nombre]:
			out[nombre] = "resultado"
		default:
			out[nombre] = "auxiliar"
		}
	}
	return out
}

// bucleExterior returns the innermost enclosing loop (nil when not inside one).
func bucleExterior(anc pila) ast.Node {
	for i := len(anc) - 1; i >= 0; i-- {
		switch anc[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			// skip the loop header itself: only bodies count
			if i+1 < len(anc) {
				if _, ok := anc[i+1].(*ast.BlockStmt); ok {
					return anc[i]
				}
			}
		case *ast.FuncLit:
			return nil
		}
	}
	return nil
}

func dentroDeIf(anc pila) bool {
	for i := len(anc) - 1; i >= 0; i-- {
		switch anc[i].(type) {
		case *ast.IfStmt, *ast.CaseClause:
			return true
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		}
	}
	return false
}

// ---- narration ----

type narrador struct {
	a      *archivo
	l      *lector
	roles  map[string]string
	frases []Frase
}

// Narrar explains a function line by line in Spanish. Level 1 gives one sentence per top-level
// statement (whole loops in one sentence); level 2 gives one sentence per line. funcion "" narrates
// the first function (or main).
func Narrar(fuente, funcion string, nivel int) ([]Frase, error) {
	a, err := parsear(fuente)
	if err != nil {
		return nil, err
	}
	fd := buscarFunc(a.f, funcion)
	if fd == nil {
		if funcion == "" {
			return nil, fmt.Errorf("no hay ninguna función que explicar")
		}
		return nil, fmt.Errorf("no encuentro la función %s", funcion)
	}
	a.comprobarTipos()
	return narrarFunc(a, fd, nivel), nil
}

func narrarFunc(a *archivo, fd *ast.FuncDecl, nivel int) []Frase {
	n := &narrador{a: a, l: &lector{marcar: true, a: a}, roles: rolesDe(a, fd)}
	n.cabecera(fd)
	if fd.Body == nil {
		return n.frases
	}
	if nivel <= 1 {
		for _, s := range fd.Body.List {
			if a.envuelto && fd.Name.Name == "main" && a.linea(s.Pos()) == 0 {
				continue
			}
			n.add(s.Pos(), mayuscula(n.resumen(s))+".", 1)
		}
		return n.frases
	}
	n.roles2()
	n.lista(fd.Body.List)
	return n.frases
}

func (n *narrador) add(p token.Pos, texto string, nivel int) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return
	}
	n.frases = append(n.frases, Frase{Linea: n.a.linea(p), Texto: texto, Nivel: nivel})
}

func (n *narrador) humanoDe(t types.Type) string {
	if t == nil {
		return ""
	}
	nt, ok := TipoDe(t)
	if !ok {
		return types.TypeString(t, func(p *types.Package) string { return p.Name() })
	}
	return nt.Humano()
}

// cabecera describes what the function receives and returns.
func (n *narrador) cabecera(fd *ast.FuncDecl) {
	nombre := "`" + nombreFunc(fd) + "`"
	if n.a.envuelto && fd.Name.Name == "main" {
		n.add(fd.Pos(), "Estas instrucciones se ejecutan en orden, de arriba abajo.", 1)
		return
	}
	var ent []string
	if fd.Type.Params != nil {
		for _, c := range fd.Type.Params.List {
			for _, id := range c.Names {
				h := n.humanoDe(n.a.tipo(id))
				if h == "" {
					h = codigo(c.Type)
				}
				ent = append(ent, "`"+id.Name+"` ("+h+")")
			}
		}
	}
	var sal []string
	if fd.Type.Results != nil {
		for _, c := range fd.Type.Results.List {
			h := n.humanoDe(n.a.tipo(c.Type))
			if h == "" {
				h = codigo(c.Type)
			}
			k := len(c.Names)
			if k == 0 {
				k = 1
			}
			for i := 0; i < k; i++ {
				sal = append(sal, articulo(h))
			}
		}
	}
	que := "La función " + nombre
	if fd.Recv != nil {
		que = "El método " + nombre
	}
	switch {
	case len(ent) == 0 && len(sal) == 0:
		que += " no recibe nada ni devuelve nada"
	case len(ent) == 0:
		que += " no recibe nada y devuelve " + unirY(sal)
	case len(sal) == 0:
		que += " recibe " + unirY(ent) + " y no devuelve nada"
	default:
		que += " recibe " + unirY(ent) + " y devuelve " + unirY(sal)
	}
	n.add(fd.Pos(), que+".", 1)
}

func articulo(h string) string {
	switch {
	case strings.HasPrefix(h, "lista"), strings.HasPrefix(h, "estructura"):
		return "una " + h
	case strings.HasPrefix(h, "verdadero"):
		return h
	case h == "texto", strings.HasPrefix(h, "número"), strings.HasPrefix(h, "mapa"), strings.HasPrefix(h, "arreglo"),
		strings.HasPrefix(h, "carácter"), strings.HasPrefix(h, "byte"), strings.HasPrefix(h, "error"), strings.HasPrefix(h, "puntero"):
		return "un " + h
	}
	return h
}

// roles2 adds one detail sentence naming the special variables.
func (n *narrador) roles2() {
	nombres := make([]string, 0, len(n.roles))
	for v, r := range n.roles {
		if r != "auxiliar" && r != "resultado" {
			nombres = append(nombres, v)
		}
	}
	sort.Strings(nombres)
	var partes []string
	for _, v := range nombres {
		partes = append(partes, "`"+v+"` como "+n.roles[v])
	}
	if len(partes) > 0 {
		n.frases = append(n.frases, Frase{Linea: n.frases[len(n.frases)-1].Linea, Texto: "Usa " + unirY(partes) + ".", Nivel: 2})
	}
}

func (n *narrador) lista(l []ast.Stmt) {
	for _, s := range l {
		n.stmt(s)
	}
}

// stmt narrates one statement at level 2 (and its children).
func (n *narrador) stmt(s ast.Stmt) {
	switch x := s.(type) {
	case *ast.BlockStmt:
		n.lista(x.List)
	case *ast.LabeledStmt:
		n.stmt(x.Stmt)
	case *ast.IfStmt:
		n.si(x, false)
	case *ast.ForStmt:
		n.add(x.Pos(), mayuscula(n.cabeceraFor(x))+".", 2)
		n.lista(x.Body.List)
	case *ast.RangeStmt:
		n.add(x.Pos(), mayuscula(n.cabeceraRange(x))+".", 2)
		n.lista(x.Body.List)
	case *ast.SwitchStmt:
		if x.Init != nil {
			n.stmt(x.Init)
		}
		if x.Tag != nil {
			n.add(x.Pos(), "Según el valor de "+n.l.sujeto(x.Tag)+":", 2)
		} else {
			n.add(x.Pos(), "Mira estos casos en orden:", 2)
		}
		for _, cc := range x.Body.List {
			c := cc.(*ast.CaseClause)
			n.add(c.Pos(), n.caso(c, x.Tag != nil), 2)
			n.lista(c.Body)
		}
	case *ast.TypeSwitchStmt:
		n.add(x.Pos(), "Según el tipo del valor:", 2)
		for _, cc := range x.Body.List {
			c := cc.(*ast.CaseClause)
			n.add(c.Pos(), n.caso(c, true), 2)
			n.lista(c.Body)
		}
	case *ast.SelectStmt:
		n.add(x.Pos(), "Espera a que uno de estos canales esté listo:", 2)
		for _, cc := range x.Body.List {
			c := cc.(*ast.CommClause)
			if c.Comm == nil {
				n.add(c.Pos(), "Si ninguno está listo:", 2)
			} else {
				n.add(c.Pos(), "Si "+minuscula(sinPunto(n.simple(c.Comm)))+":", 2)
			}
			n.lista(c.Body)
		}
	default:
		n.add(s.Pos(), mayuscula(n.simple(s))+".", 2)
	}
}

func (n *narrador) si(x *ast.IfStmt, sino bool) {
	if x.Init != nil {
		n.add(x.Init.Pos(), mayuscula(n.simple(x.Init))+".", 2)
	}
	cond := n.condicion(x.Cond, true)
	if sino {
		n.add(x.Pos(), "Si no, si "+cond+":", 2)
	} else {
		n.add(x.Pos(), "Si "+cond+":", 2)
	}
	n.lista(x.Body.List)
	switch e := x.Else.(type) {
	case *ast.IfStmt:
		n.si(e, true)
	case *ast.BlockStmt:
		n.add(e.Pos(), "Si no:", 2)
		n.lista(e.List)
	}
}

// condicion reads a condition; with codigo, the code goes in parentheses when it differs.
func (n *narrador) condicion(e ast.Expr, conCodigo bool) string {
	es := n.l.es(e)
	if id, ok := quitarParen(e).(*ast.Ident); ok {
		return "`" + id.Name + "` es verdadero"
	}
	cod := "`" + codigo(e) + "`"
	if conCodigo && es != cod && !strings.Contains(es, cod) {
		return es + " (" + cod + ")"
	}
	return es
}

func (n *narrador) caso(c *ast.CaseClause, conTag bool) string {
	if c.List == nil {
		return "En cualquier otro caso:"
	}
	partes := make([]string, len(c.List))
	for i, e := range c.List {
		if conTag {
			partes[i] = n.l.sujeto(e)
		} else {
			partes[i] = n.condicion(e, false)
		}
	}
	if conTag {
		return "Si es " + strings.Join(partes, " o ") + ":"
	}
	return "Si " + strings.Join(partes, " o ") + ":"
}

func (n *narrador) cabeceraRange(x *ast.RangeStmt) string {
	k, v := "", ""
	if id, ok := x.Key.(*ast.Ident); ok && id.Name != "_" {
		k = id.Name
	}
	if id, ok := x.Value.(*ast.Ident); ok && id.Name != "_" {
		v = id.Name
	}
	xs := n.l.sujeto(x.X)
	tipo := "lista"
	if t := n.a.tipo(x.X); t != nil {
		switch u := t.Underlying().(type) {
		case *types.Map:
			tipo = "mapa"
		case *types.Basic:
			if u.Info()&types.IsString != 0 {
				tipo = "texto"
			} else if u.Info()&types.IsInteger != 0 {
				tipo = "entero"
			}
		case *types.Chan:
			tipo = "canal"
		}
	} else if b, ok := x.X.(*ast.BasicLit); ok && b.Kind == token.INT {
		tipo = "entero"
	}
	switch tipo {
	case "mapa":
		switch {
		case k != "" && v != "":
			return "recorre cada clave `" + k + "` y su valor `" + v + "` del mapa " + xs
		case k != "":
			return "recorre cada clave `" + k + "` del mapa " + xs
		case v != "":
			return "recorre cada valor `" + v + "` del mapa " + xs
		}
		return "repite una vez por cada entrada del mapa " + xs
	case "texto":
		switch {
		case v != "" && k != "":
			return "recorre cada carácter `" + v + "` de " + xs + " (en la posición `" + k + "`)"
		case v != "":
			return "recorre cada carácter `" + v + "` de " + xs
		case k != "":
			return "recorre cada posición `" + k + "` de " + xs
		}
		return "repite una vez por cada carácter de " + xs
	case "entero":
		if k != "" {
			return "repite " + xs + " veces, con `" + k + "` desde 0"
		}
		return "repite " + xs + " veces"
	case "canal":
		if k != "" {
			return "recibe cada valor `" + k + "` del canal " + xs
		}
		return "recibe valores del canal " + xs + " hasta que se cierre"
	}
	switch {
	case k != "" && v != "":
		return "recorre " + xs + " con la posición `" + k + "` y el elemento `" + v + "`"
	case v != "":
		return "recorre cada elemento `" + v + "` de " + xs
	case k != "":
		return "recorre cada posición `" + k + "` de " + xs
	}
	return "repite una vez por cada elemento de " + xs
}

func (n *narrador) cabeceraFor(x *ast.ForStmt) string {
	if x.Init == nil && x.Cond == nil && x.Post == nil {
		return "repite sin fin (hasta que un `break` o un `return` lo corte)"
	}
	s := "repite"
	if as, ok := x.Init.(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 {
		s += " con " + n.l.sujeto(as.Lhs[0]) + " desde " + n.l.sujeto(as.Rhs[0])
	} else if x.Init != nil {
		s = minuscula(sinPunto(n.simple(x.Init))) + " y repite"
	}
	if x.Cond != nil {
		s += " mientras " + subjuntivo(n.l.es(x.Cond))
	}
	if x.Post != nil {
		s += ", y en cada vuelta " + minuscula(sinPunto(n.simple(x.Post)))
	}
	return s
}

// simple narrates a statement without children, as one clause (capitalized by the caller).
func (n *narrador) simple(s ast.Stmt) string {
	l := n.l
	switch x := s.(type) {
	case *ast.AssignStmt:
		return n.asignacion(x)
	case *ast.IncDecStmt:
		v := l.sujeto(x.X)
		if x.Tok == token.INC {
			if n.rol(x.X) == "contador" {
				return "cuenta uno más en " + v
			}
			return "suma 1 a " + v
		}
		return "resta 1 a " + v
	case *ast.DeclStmt:
		gd, ok := x.Decl.(*ast.GenDecl)
		if !ok {
			break
		}
		var partes []string
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, id := range vs.Names {
				nombre := "`" + id.Name + "`"
				if r := n.roles[id.Name]; r != "" && r != "auxiliar" && r != "resultado" {
					nombre = "el " + r + " " + nombre
				}
				if i < len(vs.Values) {
					partes = append(partes, "crea "+nombre+" con "+l.es(vs.Values[i]))
					continue
				}
				h := n.humanoDe(n.a.tipo(id))
				if h == "" && vs.Type != nil {
					h = codigo(vs.Type)
				}
				cero := "vacío"
				if t := n.a.tipo(id); t != nil {
					if nt, ok := TipoDe(t); ok {
						cero = "a cero (" + nt.Cero() + ")"
					}
				}
				partes = append(partes, "declara "+nombre+" ("+h+"), que empieza "+cero)
			}
		}
		if gd.Tok == token.CONST {
			return "define constantes: " + strings.Join(partes, "; ")
		}
		return strings.Join(partes, "; ")
	case *ast.ReturnStmt:
		if len(x.Results) == 0 {
			return "termina la función"
		}
		partes := make([]string, len(x.Results))
		for i, r := range x.Results {
			partes[i] = l.es(r)
		}
		return "devuelve " + unirY(partes)
	case *ast.BranchStmt:
		switch x.Tok {
		case token.BREAK:
			if x.Label != nil {
				return "sale del bucle `" + x.Label.Name + "`"
			}
			return "sale del bucle"
		case token.CONTINUE:
			return "pasa a la siguiente vuelta"
		case token.GOTO:
			return "salta a la etiqueta `" + x.Label.Name + "`"
		case token.FALLTHROUGH:
			return "sigue con el caso siguiente"
		}
	case *ast.ExprStmt:
		if call, ok := quitarParen(x.X).(*ast.CallExpr); ok {
			return n.llamadaSuelta(call)
		}
		return "calcula " + l.es(x.X)
	case *ast.DeferStmt:
		return "al terminar la función, " + minuscula(n.llamadaSuelta(x.Call))
	case *ast.GoStmt:
		return "lanza en paralelo (una goroutine): " + minuscula(n.llamadaSuelta(x.Call))
	case *ast.SendStmt:
		return "envía " + l.sujeto(x.Value) + " por el canal " + l.sujeto(x.Chan)
	case *ast.IfStmt:
		return "si " + n.condicion(x.Cond, false)
	case *ast.ForStmt:
		return n.cabeceraFor(x)
	case *ast.RangeStmt:
		return n.cabeceraRange(x)
	case *ast.EmptyStmt:
		return ""
	}
	return "ejecuta `" + codigo(s) + "`"
}

func (n *narrador) rol(e ast.Expr) string {
	if id, ok := quitarParen(e).(*ast.Ident); ok {
		return n.roles[id.Name]
	}
	return ""
}

func (n *narrador) asignacion(x *ast.AssignStmt) string {
	l := n.l
	if len(x.Lhs) == 2 && len(x.Rhs) == 2 && codigo(x.Lhs[0]) == codigo(x.Rhs[1]) && codigo(x.Lhs[1]) == codigo(x.Rhs[0]) {
		return "intercambia " + l.sujeto(x.Lhs[0]) + " y " + l.sujeto(x.Lhs[1])
	}
	if len(x.Lhs) == 2 && len(x.Rhs) == 1 {
		if ix, ok := quitarParen(x.Rhs[0]).(*ast.IndexExpr); ok {
			return "busca " + l.sujeto(ix.Index) + " en el mapa " + l.sujeto(ix.X) + ": " + l.sujeto(x.Lhs[0]) + " es el valor y " + l.sujeto(x.Lhs[1]) + " dice si estaba"
		}
		if call, ok := quitarParen(x.Rhs[0]).(*ast.CallExpr); ok && esIdent(x.Lhs[1], "err") {
			return "llama a " + l.m(codigo(call.Fun)) + " y guarda el resultado en " + l.sujeto(x.Lhs[0]) + " y el posible error en `err`"
		}
		if esIdent(x.Lhs[1], "_") && !esIdent(x.Lhs[0], "_") {
			if x.Tok == token.DEFINE {
				return "crea " + l.sujeto(x.Lhs[0]) + " con " + l.es(x.Rhs[0]) + " (el segundo resultado se descarta)"
			}
			return l.sujeto(x.Lhs[0]) + " pasa a valer " + l.es(x.Rhs[0]) + " (el segundo resultado se descarta)"
		}
	}
	if len(x.Lhs) != 1 || len(x.Rhs) != 1 {
		var izq, der []string
		for _, e := range x.Lhs {
			izq = append(izq, l.sujeto(e))
		}
		for _, e := range x.Rhs {
			der = append(der, l.es(e))
		}
		if x.Tok == token.DEFINE {
			return "crea " + unirY(izq) + " con " + unirY(der)
		}
		return unirY(izq) + " pasan a valer " + unirY(der)
	}
	lhs, rhs := x.Lhs[0], quitarParen(x.Rhs[0])
	v := l.sujeto(lhs)
	if esIdent(lhs, "_") {
		return "ignora " + l.es(rhs)
	}
	switch x.Tok {
	case token.ADD_ASSIGN:
		if t := n.a.tipo(lhs); esString(t) {
			return "añade " + l.sujeto(rhs) + " al final del texto " + v
		}
		return "suma " + l.sujeto(rhs) + " a " + v
	case token.SUB_ASSIGN:
		return "resta " + l.sujeto(rhs) + " a " + v
	case token.MUL_ASSIGN:
		return "multiplica " + v + " por " + l.sujeto(rhs)
	case token.QUO_ASSIGN:
		return "divide " + v + " entre " + l.sujeto(rhs)
	case token.REM_ASSIGN:
		return "guarda en " + v + " el resto de dividirlo entre " + l.sujeto(rhs)
	case token.DEFINE:
		desc := l.es(rhs)
		switch r := n.rol(lhs); r {
		case "acumulador", "contador", "bandera":
			return "crea el " + r + " " + v + ", que empieza en " + desc
		case "resultado":
			return "crea " + v + " (será el resultado) con " + desc
		}
		return "crea " + v + " con " + desc
	case token.ASSIGN:
		if call, ok := rhs.(*ast.CallExpr); ok && esIdent(call.Fun, "append") && len(call.Args) >= 2 && codigo(call.Args[0]) == codigo(lhs) {
			if call.Ellipsis.IsValid() {
				return "añade los elementos de " + l.sujeto(call.Args[1]) + " al final de " + v
			}
			partes := make([]string, 0, len(call.Args)-1)
			for _, a := range call.Args[1:] {
				partes = append(partes, l.sujeto(a))
			}
			return "añade " + unirY(partes) + " al final de " + v
		}
		if b, ok := rhs.(*ast.BinaryExpr); ok && codigo(b.X) == codigo(lhs) {
			switch b.Op {
			case token.ADD:
				return "suma " + l.sujeto(b.Y) + " a " + v
			case token.SUB:
				return "resta " + l.sujeto(b.Y) + " a " + v
			case token.MUL:
				return "multiplica " + v + " por " + l.sujeto(b.Y)
			case token.QUO:
				return "divide " + v + " entre " + l.sujeto(b.Y)
			}
		}
		return v + " pasa a valer " + l.es(rhs)
	}
	return "ejecuta `" + codigo(x) + "`"
}

func (n *narrador) llamadaSuelta(call *ast.CallExpr) string {
	l := n.l
	args := make([]string, len(call.Args))
	for i, a := range call.Args {
		args[i] = l.sujeto(a)
	}
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return "?"
	}
	nombre := codigo(call.Fun)
	switch nombre {
	case "fmt.Println", "fmt.Print":
		if len(args) == 0 {
			return "escribe una línea vacía"
		}
		return "escribe " + unirY(args) + " en pantalla"
	case "fmt.Printf":
		return "escribe en pantalla con el formato " + arg(0)
	case "fmt.Scan", "fmt.Scanln", "fmt.Scanf":
		return "lee del teclado y guarda lo leído"
	case "panic":
		return "detiene el programa con el error " + arg(0)
	case "delete":
		return "borra " + arg(1) + " del mapa " + arg(0)
	case "copy":
		return "copia los elementos de " + arg(1) + " en " + arg(0)
	case "sort.Ints", "sort.Strings", "sort.Float64s", "slices.Sort":
		return "ordena " + arg(0) + " de menor a mayor"
	case "sort.Slice", "sort.SliceStable", "slices.SortFunc":
		return "ordena " + arg(0) + " según la función de comparación"
	case "slices.Reverse":
		return "da la vuelta a " + arg(0)
	case "close":
		return "cierra el canal " + arg(0)
	case "os.Exit":
		return "termina el programa con el código " + arg(0)
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "WriteString", "WriteByte", "WriteRune":
			return "añade " + arg(0) + " al texto " + l.sujeto(sel.X)
		case "Lock":
			return "cierra el candado " + l.sujeto(sel.X)
		case "Unlock":
			return "abre el candado " + l.sujeto(sel.X)
		case "Add":
			if strings.HasSuffix(strings.ToLower(codigo(sel.X)), "wg") {
				return "anota " + arg(0) + " tareas más en " + l.sujeto(sel.X)
			}
		case "Done":
			return "avisa a " + l.sujeto(sel.X) + " de que una tarea terminó"
		case "Wait":
			return "espera a que terminen las tareas de " + l.sujeto(sel.X)
		case "Close":
			return "cierra " + l.sujeto(sel.X)
		}
	}
	if s, ok := l.llamada(call); ok {
		return "calcula " + s
	}
	return "llama a " + l.m(codigo(call))
}

// ---- level 1 ----

// resumen narrates a top-level statement as one sentence (without the final period).
func (n *narrador) resumen(s ast.Stmt) string {
	switch x := s.(type) {
	case *ast.ForStmt:
		return n.bucle(n.cabeceraFor(x), x.Body)
	case *ast.RangeStmt:
		return n.bucle(n.cabeceraRange(x), x.Body)
	case *ast.IfStmt:
		return n.siResumido(x)
	case *ast.LabeledStmt:
		return n.resumen(x.Stmt)
	case *ast.BlockStmt:
		var partes []string
		for _, y := range x.List {
			partes = append(partes, n.resumen(y))
		}
		return strings.Join(partes, "; ")
	case *ast.SwitchStmt:
		if x.Tag != nil {
			return "según el valor de " + n.l.sujeto(x.Tag) + ", elige entre " + fmt.Sprint(len(x.Body.List)) + " casos"
		}
		return "elige entre " + fmt.Sprint(len(x.Body.List)) + " casos según varias condiciones"
	case *ast.TypeSwitchStmt:
		return "según el tipo del valor, elige entre " + fmt.Sprint(len(x.Body.List)) + " casos"
	case *ast.SelectStmt:
		return "espera a que uno de " + fmt.Sprint(len(x.Body.List)) + " canales esté listo"
	}
	return n.simple(s)
}

// acciones lists the body of a loop or if as short clauses; ok is false when it is too long.
func (n *narrador) acciones(l []ast.Stmt, limite int) ([]string, bool) {
	var out []string
	for _, s := range l {
		switch x := s.(type) {
		case *ast.IfStmt:
			out = append(out, n.siResumido(x))
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
			out = append(out, n.resumen(x))
			if strings.Contains(out[len(out)-1], "varias cosas") {
				return out, false
			}
		case *ast.BlockStmt:
			sub, ok := n.acciones(x.List, limite)
			if !ok {
				return out, false
			}
			out = append(out, sub...)
		default:
			if t := n.simple(s); t != "" {
				out = append(out, t)
			}
		}
	}
	return out, len(out) <= limite
}

func (n *narrador) bucle(cab string, cuerpo *ast.BlockStmt) string {
	acc, ok := n.acciones(cuerpo.List, 3)
	if !ok {
		return cab + " y hace varias cosas en cada vuelta"
	}
	if len(acc) == 0 {
		return cab + " sin hacer nada más"
	}
	if strings.HasPrefix(acc[0], "si ") {
		return cab + " y, " + unirY(acc)
	}
	return cab + " y, en cada vuelta, " + unirY(acc)
}

func (n *narrador) siResumido(x *ast.IfStmt) string {
	pre := ""
	if x.Init != nil {
		pre = n.simple(x.Init) + "; "
	}
	cond := n.condicion(x.Cond, false)
	acc, ok := n.acciones(x.Body.List, 2)
	cuerpo := unirY(acc)
	if !ok {
		cuerpo = "hace varias cosas"
	}
	s := pre + "si " + cond + ", " + cuerpo
	switch e := x.Else.(type) {
	case *ast.IfStmt:
		s += "; si no, " + n.siResumido(e)
	case *ast.BlockStmt:
		acc, ok := n.acciones(e.List, 2)
		if ok {
			s += "; si no, " + unirY(acc)
		} else {
			s += "; si no, hace varias cosas"
		}
	}
	return s
}

// ---- comments ----

// Comentar inserts Spanish comments (level 1) above the statements of every function, plus a doc
// comment for functions that have none, and returns the gofmt'd source.
func Comentar(fuente string) (string, error) {
	a, err := parsear(fuente)
	if err != nil {
		return "", err
	}
	if len(a.errores) > 0 {
		e := a.errores[0]
		return "", fmt.Errorf("el código no se puede leer (línea %d): %s", e.Linea, e.Msg)
	}
	a.comprobarTipos()
	lineas := strings.Split(strings.ReplaceAll(fuente, "\r\n", "\n"), "\n")
	insertar := map[int]string{} // line (1-based) → comment text
	for _, d := range a.f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		frases := narrarFunc(a, fd, 1)
		for i, f := range frases {
			if f.Linea <= 0 || f.Linea > len(lineas) {
				continue
			}
			if i == 0 && (fd.Doc != nil || a.envuelto) {
				continue // keep the existing doc comment
			}
			if _, ya := insertar[f.Linea]; ya {
				continue
			}
			if i == 0 {
				f.Texto = strings.Replace(f.Texto, "La función `"+fd.Name.Name+"`", fd.Name.Name, 1)
				f.Texto = strings.Replace(f.Texto, "El método `"+nombreFunc(fd)+"`", fd.Name.Name, 1)
			}
			insertar[f.Linea] = f.Texto
		}
	}
	var sb strings.Builder
	for i, l := range lineas {
		if c, ok := insertar[i+1]; ok {
			sangria := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
			for _, parte := range partirComentario(c, 90) {
				sb.WriteString(sangria + "// " + parte + "\n")
			}
		}
		sb.WriteString(l)
		if i < len(lineas)-1 {
			sb.WriteString("\n")
		}
	}
	out, err := format.Source([]byte(sb.String()))
	if err != nil {
		return "", errorDeSintaxis(err)
	}
	return string(out), nil
}

// partirComentario wraps a comment at about ancho runes.
func partirComentario(s string, ancho int) []string {
	palabras := strings.Fields(s)
	var out []string
	var linea []string
	largo := 0
	for _, p := range palabras {
		n := utf8.RuneCountInString(p)
		if largo > 0 && largo+1+n > ancho {
			out = append(out, strings.Join(linea, " "))
			linea, largo = nil, 0
		}
		linea = append(linea, p)
		if largo > 0 {
			largo++
		}
		largo += n
	}
	if len(linea) > 0 {
		out = append(out, strings.Join(linea, " "))
	}
	return out
}
