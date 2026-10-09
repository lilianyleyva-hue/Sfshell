package reparar

import (
	"go/scanner"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ficha is one token of the source, with byte offsets.
type ficha struct {
	tok      token.Token
	lit      string
	ini, fin int
	linea    int
	auto     bool // automatic semicolon (newline or EOF)
}

// fichas scans src with go/scanner (comments included). Scan errors are ignored: foreign code is
// expected to be full of them.
func fichas(src string) []ficha {
	fset := token.NewFileSet()
	f := fset.AddFile(archivoFuente, -1, len(src))
	var s scanner.Scanner
	s.Init(f, []byte(src), func(token.Position, string) {}, scanner.ScanComments)
	var out []ficha
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		ini := f.Offset(pos)
		fc := ficha{tok: tok, lit: lit, ini: ini, linea: f.Line(pos)}
		switch {
		case tok == token.SEMICOLON && lit != ";":
			fc.auto = true
			fc.fin = ini
		case lit != "":
			fc.fin = ini + len(lit)
		default:
			fc.fin = ini + len(tok.String())
		}
		if fc.fin > len(src) {
			fc.fin = len(src)
		}
		out = append(out, fc)
	}
	return out
}

// Idiomas rewrites habits from other languages, editing only identifier, operator and punctuation
// tokens (never inside strings or comments): X.length → len(X), X.push(Y) → X = append(X, Y),
// print( → fmt.Println(, elif → else if, None/null → nil, True/False → true/false, and/or/not → && || !,
// int x = 5; → x := 5, for (i = 0; i < n; i++) → for i := 0; i < n; i++, function → func, and a few more.
// Every change says which language the habit comes from. Imports are not added here (the compile loop
// adds them).
func Idiomas(fuente string) (string, []Cambio) {
	return idiomas(fuente, nil)
}

// idiomas applies the rewrites whose first line passes permitir (nil = all), in up to 6 passes.
func idiomas(fuente string, permitir func(linea int) bool) (string, []Cambio) {
	var cambios []Cambio
	src := fuente
	for pasada := 0; pasada < 6; pasada++ {
		eds := buscarIdiomas(src, permitir)
		if len(eds) == 0 {
			break
		}
		var aplicadas []edicion
		src, aplicadas = aplicar(src, eds)
		for _, e := range aplicadas {
			cambios = append(cambios, e.cambio)
		}
		if len(aplicadas) == len(eds) {
			// everything applied: one more pass only finds new opportunities created by the edits
			continue
		}
	}
	return src, juntarPuntoYComa(cambios)
}

// juntarPuntoYComa reports all the removed semicolons as a single change.
func juntarPuntoYComa(cs []Cambio) []Cambio {
	var out []Cambio
	n, primera := 0, 0
	for _, c := range cs {
		if c.Regla == "punto_y_coma" {
			if n == 0 {
				primera = c.Linea
			}
			n++
			continue
		}
		out = append(out, c)
	}
	if n > 0 {
		porque := "en Go no hace falta «;» al final de la línea"
		if n > 1 {
			porque = "quité " + strconv.Itoa(n) + " «;» del final de las líneas: en Go no hacen falta"
		}
		out = append(out, Cambio{Regla: "punto_y_coma", Linea: primera, Antes: ";", Despues: "", Porque: porque})
	}
	return out
}

var (
	tiposOtros = map[string]string{
		"int": "int", "long": "int", "short": "int", "float": "float64", "double": "float64",
		"String": "string", "string": "string", "bool": "bool", "boolean": "bool", "char": "rune",
		"let": "", "auto": "",
	}
	nulos     = map[string]string{"null": "JavaScript", "None": "Python", "NULL": "C", "nullptr": "C++", "undefined": "JavaScript", "Nil": "", "NIL": ""}
	booleanos = map[string]string{"True": "true", "False": "false", "TRUE": "true", "FALSE": "false"}
	// metodosTexto: receiver.method(args) of other languages → Go function. "%x" is the receiver,
	// "%a" the arguments.
	metodosTexto = map[string][2]string{
		"upper":       {"strings.ToUpper(%x)", "Python"},
		"toUpperCase": {"strings.ToUpper(%x)", "JavaScript"},
		"lower":       {"strings.ToLower(%x)", "Python"},
		"toLowerCase": {"strings.ToLower(%x)", "JavaScript"},
		"strip":       {"strings.TrimSpace(%x)", "Python"},
		"trim":        {"strings.TrimSpace(%x)", "JavaScript"},
		"split":       {"strings.Split(%x, %a)", "JavaScript/Python"},
		"includes":    {"strings.Contains(%x, %a)", "JavaScript"},
		"startsWith":  {"strings.HasPrefix(%x, %a)", "JavaScript"},
		"startswith":  {"strings.HasPrefix(%x, %a)", "Python"},
		"endsWith":    {"strings.HasSuffix(%x, %a)", "JavaScript"},
		"endswith":    {"strings.HasSuffix(%x, %a)", "Python"},
		"replace":     {"strings.ReplaceAll(%x, %a)", "JavaScript/Python"},
		"indexOf":     {"strings.Index(%x, %a)", "JavaScript"},
		"find":        {"strings.Index(%x, %a)", "Python"},
		"join":        {"strings.Join(%a, %x)", "Python"},
		"toString":    {"fmt.Sprint(%x)", "JavaScript"},
	}
	reInitFor = regexp.MustCompile(`^\s*(?:(?:int|let|var|auto|long)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=([^=])`)
)

func terminaExpr(t token.Token) bool {
	switch t {
	case token.IDENT, token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING, token.RPAREN, token.RBRACK, token.RBRACE:
		return true
	}
	return false
}

func empiezaExpr(t token.Token) bool {
	switch t {
	case token.IDENT, token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING, token.LPAREN, token.NOT,
		token.SUB, token.ADD, token.XOR, token.LBRACK, token.FUNC, token.MUL, token.AND, token.MAP, token.STRUCT:
		return true
	}
	return false
}

// buscarIdiomas finds the rewrites in src (comments excluded from the token stream).
func buscarIdiomas(src string, permitir func(linea int) bool) []edicion {
	todas := fichas(src)
	var eds []edicion
	nueva := func(ini, fin int, texto, regla, lenguaje string) {
		linea := lineaDe(src, ini)
		if permitir != nil && !permitir(linea) {
			return
		}
		porque := "esto es de " + lenguaje + "; en Go se escribe «" + strings.TrimSpace(texto) + "»"
		if lenguaje == "" {
			porque = "en Go se escribe «" + strings.TrimSpace(texto) + "»"
		}
		eds = append(eds, edicion{ini: ini, fin: fin, texto: texto, cambio: Cambio{
			Regla: regla, Linea: linea, Antes: src[ini:fin], Despues: texto, Porque: porque,
		}})
	}

	// Python comment lines: "# …" → "// …" (and the rest of the line is not code)
	comentadas := map[int]bool{}
	for k, t := range todas {
		if t.tok == token.ILLEGAL && t.lit == "#" && (k == 0 || todas[k-1].linea < t.linea || todas[k-1].auto) &&
			strings.TrimSpace(src[inicioLinea(src, t.ini):t.ini]) == "" {
			comentadas[t.linea] = true
			if permitir == nil || permitir(t.linea) {
				eds = append(eds, edicion{ini: t.ini, fin: t.fin, texto: "//", cambio: Cambio{
					Regla: "comentario", Linea: t.linea, Antes: "#", Despues: "//",
					Porque: "esto es de Python; en Go los comentarios empiezan con «//»",
				}})
			}
		}
	}
	var ts []ficha
	for _, t := range todas {
		if t.tok == token.COMMENT || comentadas[t.linea] {
			continue
		}
		ts = append(ts, t)
	}
	declarados := nombresDeclarados(ts)
	es := func(k int, t token.Token) bool { return k >= 0 && k < len(ts) && ts[k].tok == t }
	esId := func(k int, nombre string) bool { return es(k, token.IDENT) && ts[k].lit == nombre }
	inicioSentencia := func(k int) bool {
		return k == 0 || es(k-1, token.SEMICOLON) || es(k-1, token.LBRACE) || es(k-1, token.RBRACE) || es(k-1, token.COLON)
	}

	for k := 0; k < len(ts); k++ {
		t := ts[k]
		switch t.tok {
		case token.PERIOD:
			if !es(k+1, token.IDENT) || k == 0 {
				continue
			}
			nombre := ts[k+1].lit
			if declarados[nombre] {
				continue
			}
			ini := inicioOperando(ts, k-1)
			if ini < 0 {
				continue
			}
			x := src[ts[ini].ini:ts[k-1].fin]
			switch {
			case nombre == "length" || nombre == "lenght":
				fin := ts[k+1].fin
				if es(k+2, token.LPAREN) && es(k+3, token.RPAREN) {
					fin = ts[k+3].fin
				}
				nueva(ts[ini].ini, fin, "len("+x+")", "longitud", "JavaScript")
			case (nombre == "size" || nombre == "len" || nombre == "count") && es(k+2, token.LPAREN) && es(k+3, token.RPAREN):
				nueva(ts[ini].ini, ts[k+3].fin, "len("+x+")", "longitud", "Java")
			case (nombre == "push" || nombre == "append" || nombre == "extend") && es(k+2, token.LPAREN) && inicioSentencia(ini):
				m := cierre(ts, k+2)
				if m < 0 || !(m+1 >= len(ts) || es(m+1, token.SEMICOLON) || es(m+1, token.RBRACE)) {
					continue
				}
				args := strings.TrimSpace(src[ts[k+2].fin:ts[m].ini])
				if args == "" {
					continue
				}
				lenguaje := "JavaScript"
				if nombre != "push" {
					lenguaje = "Python"
				}
				if nombre == "extend" {
					args += "..."
				}
				nueva(ts[ini].ini, ts[m].fin, x+" = append("+x+", "+args+")", "agregar", lenguaje)
			case metodosTexto[nombre][0] != "" && es(k+2, token.LPAREN):
				m := cierre(ts, k+2)
				if m < 0 {
					continue
				}
				args := strings.TrimSpace(src[ts[k+2].fin:ts[m].ini])
				plantilla, lenguaje := metodosTexto[nombre][0], metodosTexto[nombre][1]
				if nombre == "split" && args == "" {
					plantilla = "strings.Fields(%x)"
				}
				if strings.Contains(plantilla, "%a") && args == "" || !strings.Contains(plantilla, "%a") && args != "" {
					continue
				}
				if _, esPaquete := paquetesStd[x]; esPaquete && !declarados[x] {
					continue
				}
				texto := strings.ReplaceAll(strings.ReplaceAll(plantilla, "%x", x), "%a", args)
				nueva(ts[ini].ini, ts[m].fin, texto, "metodo_de_texto", lenguaje)
			}
		case token.IDENT:
			if es(k-1, token.PERIOD) {
				continue
			}
			nombre := t.lit
			if declarados[nombre] {
				continue
			}
			switch {
			case (nombre == "print" || nombre == "println") && es(k+1, token.LPAREN):
				lenguaje := "Python"
				if nombre == "println" {
					lenguaje = ""
				}
				nueva(t.ini, t.fin, "fmt.Println", "imprimir", lenguaje)
				if nombre == "println" {
					eds[len(eds)-1].cambio.Porque = "«println» escribe en la salida de errores; en Go se usa «fmt.Println»"
				}
			case nombre == "printf" && es(k+1, token.LPAREN):
				nueva(t.ini, t.fin, "fmt.Printf", "imprimir", "C")
			case nombre == "console" && es(k+1, token.PERIOD) && esId(k+2, "log") && es(k+3, token.LPAREN):
				nueva(t.ini, ts[k+2].fin, "fmt.Println", "imprimir", "JavaScript")
			case nombre == "System" && es(k+1, token.PERIOD) && esId(k+2, "out") && es(k+3, token.PERIOD) && es(k+4, token.IDENT) && es(k+5, token.LPAREN):
				f := map[string]string{"println": "fmt.Println", "print": "fmt.Print", "printf": "fmt.Printf"}[ts[k+4].lit]
				if f != "" {
					nueva(t.ini, ts[k+4].fin, f, "imprimir", "Java")
				}
			case (nombre == "str" || nombre == "String") && es(k+1, token.LPAREN) && !es(k-1, token.LBRACK):
				lenguaje := "Python"
				if nombre == "String" {
					lenguaje = "JavaScript"
				}
				nueva(t.ini, t.fin, "fmt.Sprint", "a_texto", lenguaje)
			case nombre == "elif":
				nueva(t.ini, t.fin, "else if", "elif", "Python")
			case nulos[nombre] != "" || nombre == "Nil" || nombre == "NIL":
				nueva(t.ini, t.fin, "nil", "nulo", nulos[nombre])
			case booleanos[nombre] != "":
				nueva(t.ini, t.fin, booleanos[nombre], "booleano", "Python")
			case (nombre == "and" || nombre == "or") && k > 0 && terminaExpr(ts[k-1].tok) && k+1 < len(ts) && empiezaExpr(ts[k+1].tok):
				op := "&&"
				if nombre == "or" {
					op = "||"
				}
				nueva(t.ini, t.fin, op, "operador_logico", "Python")
			case nombre == "not" && (k == 0 || !terminaExpr(ts[k-1].tok)) && k+1 < len(ts) && empiezaExpr(ts[k+1].tok):
				fin := t.fin // "not x" → "!x": the space goes too
				if ts[k+1].ini > t.fin && strings.TrimSpace(src[t.fin:ts[k+1].ini]) == "" {
					fin = ts[k+1].ini
				}
				nueva(t.ini, fin, "!", "operador_logico", "Python")
			case nombre == "function":
				nueva(t.ini, t.fin, "func", "funcion", "JavaScript")
			case nombre == "while" && inicioSentencia(k):
				nueva(t.ini, t.fin, "for", "mientras", "C/Java/JavaScript")
				eds[len(eds)-1].cambio.Porque = "en Go no existe «while»: se escribe «for condición { … }»"
			case tiposOtros[nombre] != "" || nombre == "let" || nombre == "auto":
				if !inicioSentencia(k) || !es(k+1, token.IDENT) {
					continue
				}
				v := ts[k+1].lit
				goTipo := tiposOtros[nombre]
				switch {
				case es(k+2, token.ASSIGN):
					texto := v + " :="
					if goTipo != "" && goTipo != "int" {
						texto = "var " + v + " " + goTipo + " ="
					}
					nueva(t.ini, ts[k+2].fin, texto, "declaracion", lenguajeTipo(nombre))
				case es(k+2, token.SEMICOLON) && goTipo != "":
					nueva(t.ini, ts[k+1].fin, "var "+v+" "+goTipo, "declaracion", lenguajeTipo(nombre))
				}
			}
		case token.FOR:
			if !es(k+1, token.LPAREN) {
				continue
			}
			m := cierre(ts, k+1)
			if m < 0 || !es(m+1, token.LBRACE) {
				continue
			}
			puntos := 0
			for j := k + 2; j < m; j++ {
				if ts[j].tok == token.SEMICOLON && !ts[j].auto {
					puntos++
				}
			}
			interior := src[ts[k+1].fin:ts[m].ini]
			switch {
			case puntos == 2:
				partes := strings.SplitN(interior, ";", 2)
				ini := reInitFor.ReplaceAllString(partes[0], "$1 :=$2")
				texto := strings.TrimSpace(ini) + ";" + strings.TrimRight(partes[1], " ")
				nueva(ts[k+1].ini, ts[m].fin, texto, "for_sin_parentesis", "C/Java/JavaScript")
				eds[len(eds)-1].cambio.Porque = "en Go el «for» va sin paréntesis y la variable se crea con «:=»"
			default:
				// for (x of xs) / for (const x in xs)
				j := k + 2
				if esId(j, "let") || esId(j, "const") || es(j, token.CONST) || es(j, token.VAR) {
					j++
				}
				if es(j, token.IDENT) && (esId(j+1, "of") || esId(j+1, "in")) && j+2 < m {
					v := ts[j].lit
					coleccion := strings.TrimSpace(src[ts[j+2].ini:ts[m].ini])
					texto := "_, " + v + " := range " + coleccion
					if ts[j+1].lit == "in" {
						texto = v + " := range " + coleccion
					}
					nueva(ts[k+1].ini, ts[m].fin, texto, "for_range", "JavaScript")
					eds[len(eds)-1].cambio.Porque = "en Go se recorre una lista con «for _, x := range lista»"
				}
			}
		case token.EQL, token.NEQ:
			if es(k+1, token.ASSIGN) && ts[k+1].ini == t.fin {
				nueva(t.ini, ts[k+1].fin, t.tok.String(), "comparacion", "JavaScript")
			}
		case token.SEMICOLON:
			if t.auto {
				continue
			}
			if k+1 >= len(ts) || ts[k+1].linea > t.linea || (ts[k+1].auto && ts[k+1].ini == len(src)) {
				if permitir == nil || permitir(t.linea) {
					eds = append(eds, edicion{ini: t.ini, fin: t.fin, cambio: Cambio{Regla: "punto_y_coma", Linea: t.linea, Antes: ";"}})
				}
			}
		case token.CHAR:
			cuerpo := t.lit
			if len(cuerpo) < 2 || cuerpo[0] != '\'' {
				continue
			}
			cuerpo = strings.TrimSuffix(cuerpo[1:], "'")
			if utf8.RuneCountInString(strings.ReplaceAll(cuerpo, "\\", "")) <= 1 || strings.HasPrefix(cuerpo, "\\") && len(cuerpo) <= 6 {
				continue
			}
			cuerpo = strings.ReplaceAll(cuerpo, `\'`, `'`)
			cuerpo = strings.ReplaceAll(cuerpo, `"`, `\"`)
			nueva(t.ini, t.fin, `"`+cuerpo+`"`, "comillas", "JavaScript/Python")
			eds[len(eds)-1].cambio.Porque = "en Go los textos van entre comillas dobles; las simples son para un solo carácter"
		}
	}
	return eds
}

func lenguajeTipo(nombre string) string {
	switch nombre {
	case "let":
		return "JavaScript"
	case "auto":
		return "C++"
	case "String", "boolean":
		return "Java"
	}
	return "C/Java"
}

// cierre returns the index of the token closing the bracket at k, or -1.
func cierre(ts []ficha, k int) int {
	abre := ts[k].tok
	var cierra token.Token
	switch abre {
	case token.LPAREN:
		cierra = token.RPAREN
	case token.LBRACK:
		cierra = token.RBRACK
	case token.LBRACE:
		cierra = token.RBRACE
	default:
		return -1
	}
	nivel := 0
	for j := k; j < len(ts); j++ {
		switch ts[j].tok {
		case abre:
			nivel++
		case cierra:
			nivel--
			if nivel == 0 {
				return j
			}
		}
	}
	return -1
}

// apertura returns the index of the token opening the bracket closed at k, or -1.
func apertura(ts []ficha, k int) int {
	cierra := ts[k].tok
	var abre token.Token
	switch cierra {
	case token.RPAREN:
		abre = token.LPAREN
	case token.RBRACK:
		abre = token.LBRACK
	default:
		return -1
	}
	nivel := 0
	for j := k; j >= 0; j-- {
		switch ts[j].tok {
		case cierra:
			nivel++
		case abre:
			nivel--
			if nivel == 0 {
				return j
			}
		}
	}
	return -1
}

// inicioOperando returns the first token of the maximal operand that ends at token k: an identifier
// chain, possibly with balanced (...) and [...], or a string literal. -1 if there is none.
func inicioOperando(ts []ficha, k int) int {
	j := k
	for j >= 0 {
		switch ts[j].tok {
		case token.IDENT, token.STRING:
		case token.RPAREN, token.RBRACK:
			a := apertura(ts, j)
			if a < 0 {
				return -1
			}
			if a > 0 && (ts[a-1].tok == token.IDENT || ts[a-1].tok == token.RPAREN || ts[a-1].tok == token.RBRACK) {
				j = a - 1
				continue
			}
			return a
		default:
			return -1
		}
		if j >= 2 && ts[j-1].tok == token.PERIOD && (ts[j-2].tok == token.IDENT || ts[j-2].tok == token.RPAREN || ts[j-2].tok == token.RBRACK) {
			j -= 2
			continue
		}
		return j
	}
	return -1
}

// nombresDeclarados collects names the file itself declares (functions, methods, variables, types,
// fields), so that a user's own "print" or "length" is never rewritten.
func nombresDeclarados(ts []ficha) map[string]bool {
	d := map[string]bool{}
	for k, t := range ts {
		switch t.tok {
		case token.FUNC:
			if k+1 < len(ts) && ts[k+1].tok == token.IDENT {
				d[ts[k+1].lit] = true
			}
			if k+1 < len(ts) && ts[k+1].tok == token.LPAREN { // method: func (r T) Name
				if m := cierre(ts, k+1); m > 0 && m+1 < len(ts) && ts[m+1].tok == token.IDENT {
					d[ts[m+1].lit] = true
				}
			}
		case token.VAR, token.CONST, token.TYPE:
			if k+1 < len(ts) && ts[k+1].tok == token.IDENT {
				d[ts[k+1].lit] = true
			}
		case token.DEFINE:
			for j := k - 1; j >= 0 && (ts[j].tok == token.IDENT || ts[j].tok == token.COMMA); j-- {
				if ts[j].tok == token.IDENT {
					d[ts[j].lit] = true
				}
			}
		case token.STRUCT:
			if k+1 < len(ts) && ts[k+1].tok == token.LBRACE {
				m := cierre(ts, k+1)
				for j := k + 2; j < m && m > 0; j++ {
					if ts[j].tok == token.IDENT && (ts[j-1].tok == token.LBRACE || ts[j-1].tok == token.SEMICOLON || ts[j-1].tok == token.COMMA) {
						d[ts[j].lit] = true
					}
				}
			}
		}
	}
	return d
}
