package reparar

import (
	"errors"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"strings"
	"time"
)

// errorSintaxis is the first parse error of a source and how many there are.
type errorSintaxis struct {
	ok         bool
	linea, col int
	msg        string
	n          int
}

func revisarSintaxis(src string) errorSintaxis {
	_, err := parser.ParseFile(token.NewFileSet(), archivoFuente, src, parser.AllErrors)
	if err == nil {
		return errorSintaxis{ok: true}
	}
	var lista scanner.ErrorList
	if errors.As(err, &lista) && len(lista) > 0 {
		lista.Sort()
		return errorSintaxis{linea: lista[0].Pos.Line, col: lista[0].Pos.Column, msg: lista[0].Msg, n: len(lista)}
	}
	return errorSintaxis{linea: 1, col: 1, msg: err.Error(), n: 1}
}

// mejor reports whether a is strictly better than b: it parses, or its first error is further down,
// or (same place) it has fewer errors.
func (a errorSintaxis) mejor(b errorSintaxis) bool {
	switch {
	case b.ok:
		return false
	case a.ok:
		return true
	case a.linea != b.linea:
		return a.linea > b.linea
	case a.col != b.col:
		return a.col > b.col
	}
	return a.n < b.n
}

const maxParseosRonda = 30

// repararSintaxis edits the source until it parses (at most rondas rounds, until limite). Each round
// tries candidate edits within ±2 lines of the first error (plus a missing "}" found by indentation)
// and keeps the one that removes the first error or moves it furthest down.
func repararSintaxis(src string, rondas int, limite time.Time) (string, []Cambio, int, bool) {
	var cambios []Cambio
	actual := revisarSintaxis(src)
	n := 0
	for ; n < rondas && !actual.ok && time.Now().Before(limite); n++ {
		cands := candidatosSintaxis(src, actual)
		var mejorSrc string
		var mejorEd edicion
		mejorRes := actual
		for i, c := range cands {
			if i >= maxParseosRonda {
				break
			}
			nuevo, _ := aplicar(src, []edicion{c})
			if nuevo == src {
				continue
			}
			r := revisarSintaxis(nuevo)
			if r.mejor(mejorRes) {
				mejorRes, mejorSrc, mejorEd = r, nuevo, c
				if r.ok {
					break
				}
			}
		}
		if mejorSrc == "" {
			break
		}
		src, actual = mejorSrc, mejorRes
		cambios = append(cambios, mejorEd.cambio)
	}
	return src, cambios, n, actual.ok
}

var (
	reCabecera   = regexp.MustCompile(`^\s*(\}\s*else\s+if|\}\s*else|if|for|func|switch|else\s+if|else)\b`)
	reDefineTope = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*(.*)$`)
)

// candidatosSintaxis lists the edits to try for the first parse error e, most likely first.
func candidatosSintaxis(src string, e errorSintaxis) []edicion {
	lineas := strings.Split(src, "\n")
	inicios := make([]int, len(lineas)+1)
	for i, l := range lineas {
		inicios[i+1] = inicios[i] + len(l) + 1
	}
	finDe := func(l int) int { // offset of the end of line l (1-based), before any trailing comment
		t := lineas[l-1]
		if i := indiceComentario(t); i >= 0 {
			t = t[:i]
		}
		return inicios[l-1] + len(strings.TrimRight(t, " \t"))
	}
	var out []edicion
	agregar := func(ini, fin int, texto, regla, porque string) {
		if ini < 0 || fin < ini || fin > len(src) {
			return
		}
		l := lineaDe(src, ini)
		antes := strings.TrimSpace(lineas[l-1])
		for _, o := range out {
			if o.ini == ini && o.fin == fin && o.texto == texto {
				return
			}
		}
		nuevo := src[:ini] + texto + src[fin:]
		despues := textoLinea(nuevo, l)
		if strings.HasPrefix(texto, "\n") { // a new line: show what was added
			antes, despues = "", strings.TrimSpace(texto)
		}
		out = append(out, edicion{ini: ini, fin: fin, texto: texto, cambio: Cambio{Regla: regla, Linea: l, Antes: antes, Despues: despues, Porque: porque}})
	}
	msg := e.msg

	// a "}" missing according to the indentation
	if off, texto, ok := llaveFaltante(src); ok && (strings.Contains(msg, "'}'") || strings.Contains(msg, "EOF") || strings.Contains(msg, "expected") || strings.Contains(msg, "unexpected")) {
		agregar(off, off, texto, "cerrar_llave", "faltaba cerrar un bloque con «}»")
	}
	desde, hasta := e.linea-2, e.linea+2
	if desde < 1 {
		desde = 1
	}
	if hasta > len(lineas) {
		hasta = len(lineas)
	}
	// the error line first, then its neighbours
	orden := []int{e.linea}
	for d := 1; d <= 2; d++ {
		orden = append(orden, e.linea-d, e.linea+d)
	}
	for _, l := range orden {
		if l < desde || l > hasta || l > len(lineas) {
			continue
		}
		t := lineas[l-1]
		tt := strings.TrimSpace(t)
		fin := finDe(l)
		ini := inicios[l-1]
		// unterminated string: close it before the last ")" or at the end of the line
		if strings.Count(sinEscapes(t), `"`)%2 == 1 {
			q := strings.LastIndex(t, `"`)
			if p := strings.LastIndex(t, ")"); p > q {
				agregar(ini+p, ini+p, `"`, "cerrar_cadena", "faltaba cerrar el texto con comillas")
			}
			agregar(fin, fin, `"`, "cerrar_cadena", "faltaba cerrar el texto con comillas")
		}
		// missing "{" after if/for/func headers
		if reCabecera.MatchString(t) && !strings.HasSuffix(strings.TrimRight(t[:fin-ini], " \t"), "{") && tt != "}" {
			agregar(fin, fin, " {", "abrir_llave", "faltaba «{» para abrir el bloque")
		}
		// "}" and "else" on different lines
		if strings.HasPrefix(tt, "else") && l >= 2 && strings.TrimSpace(lineas[l-2]) == "}" {
			agregar(finDe(l-1), ini+len(t)-len(strings.TrimLeft(t, " \t")), " ", "unir_else",
				"en Go «else» tiene que ir en la misma línea que la «}» de antes")
		}
		// "{" alone on its line
		if tt == "{" && l >= 2 {
			agregar(finDe(l-1), ini+strings.Index(t, "{"), " ", "unir_llave", "en Go la «{» va en la misma línea que la función o el if")
		}
		// missing ")": before a trailing "{", at the error position, or at the end of the line
		if strings.Count(t, "(") > strings.Count(t, ")") {
			if strings.HasSuffix(strings.TrimRight(t[:fin-ini], " \t"), "{") {
				p := ini + strings.LastIndex(t[:fin-ini], "{")
				for p > ini && (src[p-1] == ' ' || src[p-1] == '\t') {
					p--
				}
				agregar(p, p, ")", "cerrar_parentesis", "faltaba cerrar un paréntesis «)»")
			}
			if l == e.linea {
				p := offset(src, e.linea, e.col)
				if p <= fin {
					agregar(p, p, ")", "cerrar_parentesis", "faltaba cerrar un paréntesis «)»")
				}
			}
			agregar(fin, fin, ")", "cerrar_parentesis", "faltaba cerrar un paréntesis «)»")
		}
		// missing "," at the end of a line inside a composite literal or a call
		if strings.Contains(msg, "missing ','") || strings.Contains(msg, "composite literal") || strings.Contains(msg, "possibly missing comma") {
			if fin > ini && !strings.HasSuffix(tt, ",") && !strings.HasSuffix(tt, "{") {
				agregar(fin, fin, ",", "coma_final", "faltaba una coma «,» al final de la línea")
			}
		}
		// stray ";"
		for j, c := 0, 0; j < fin-ini && c < 3; j++ {
			if t[j] == ';' {
				c++
				agregar(ini+j, ini+j+1, "", "quitar_punto_y_coma", "sobraba un «;»")
			}
		}
		// ":=" ↔ "="
		if m := reDefineTope.FindStringSubmatch(t); m != nil && strings.Contains(msg, "expected declaration") {
			agregar(ini, ini+len(t), m[1]+"var "+m[2]+" = "+m[3], "dos_puntos_igual", "fuera de una función no se puede usar «:=»: se escribe «var x = …»")
		}
		if i := strings.Index(t, ":="); i >= 0 {
			agregar(ini+i, ini+i+2, "=", "dos_puntos_igual", "aquí va «=» y no «:=»")
		} else if i := indiceAsignacion(t); i >= 0 {
			agregar(ini+i, ini+i+1, ":=", "dos_puntos_igual", "aquí va «:=» para crear la variable")
		}
		// a "}" at the end of the line or of the file
		agregar(fin, fin, "\n"+indentacion(src, ini)+"}", "cerrar_llave", "faltaba cerrar un bloque con «}»")
	}
	agregar(len(src), len(src), "\n}\n", "cerrar_llave", "faltaba cerrar un bloque con «}»")
	return out
}

// sinEscapes removes escaped quotes and the contents of raw strings and comments, roughly.
func sinEscapes(t string) string {
	t = strings.ReplaceAll(t, `\\`, "")
	t = strings.ReplaceAll(t, `\"`, "")
	if i := indiceComentario(t); i >= 0 {
		t = t[:i]
	}
	// a quote inside a rune literal '"'
	return strings.ReplaceAll(t, `'"'`, "")
}

// indiceComentario returns the index of a "//" that is not inside a string, or -1.
func indiceComentario(t string) int {
	dentro := byte(0)
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case dentro != 0:
			if c == '\\' && dentro != '`' {
				i++
			} else if c == dentro {
				dentro = 0
			}
		case c == '"' || c == '`' || c == '\'':
			dentro = c
		case c == '/' && i+1 < len(t) && t[i+1] == '/':
			return i
		}
	}
	return -1
}

// indiceAsignacion returns the index of a lone "=" (not ==, <=, >=, !=, :=, +=…), or -1.
func indiceAsignacion(t string) int {
	for i := 0; i < len(t); i++ {
		if t[i] != '=' {
			continue
		}
		if i+1 < len(t) && t[i+1] == '=' {
			i++
			continue
		}
		if i > 0 && strings.ContainsRune("=!<>:+-*/%&|^", rune(t[i-1])) {
			continue
		}
		return i
	}
	return -1
}

// llaveFaltante finds a block whose "}" is missing, by indentation: a line that is not deeper than the
// line that opened the innermost "{" (and does not start with "}", case or default) closes it. It returns
// where to insert the "}" (end of the previous non-empty line) and the text to insert.
func llaveFaltante(src string) (int, string, bool) {
	type abierta struct {
		ancho   int
		sangria string
	}
	var pila []abierta
	ts := fichas(src)
	lineaPrevia := -1 // offset of the end of the last line with tokens
	k := 0
	lineas := strings.Split(src, "\n")
	ini := 0
	for l := 1; l <= len(lineas); l++ {
		t := lineas[l-1]
		finL := ini + len(t)
		// tokens of this line
		var primera *ficha
		j := k
		for j < len(ts) && ts[j].ini < finL+1 && ts[j].linea <= l {
			if ts[j].linea == l && primera == nil && !ts[j].auto && ts[j].tok != token.COMMENT {
				primera = &ts[j]
			}
			j++
		}
		if primera != nil {
			sangria := t[:len(t)-len(strings.TrimLeft(t, " \t"))]
			ancho := anchoSangria(sangria)
			if len(pila) > 0 && primera.tok != token.RBRACE && primera.tok != token.CASE && primera.tok != token.DEFAULT &&
				primera.tok != token.RPAREN && ancho <= pila[len(pila)-1].ancho && lineaPrevia >= 0 {
				return lineaPrevia, "\n" + pila[len(pila)-1].sangria + "}", true
			}
			for x := k; x < j; x++ {
				switch ts[x].tok {
				case token.LBRACE:
					pila = append(pila, abierta{ancho: ancho, sangria: sangria})
				case token.RBRACE:
					if len(pila) > 0 {
						pila = pila[:len(pila)-1]
					}
				}
			}
			lineaPrevia = ini + len(strings.TrimRight(t, " \t"))
		}
		k = j
		ini = finL + 1
	}
	if len(pila) > 0 {
		fin := strings.TrimRight(src, " \t\n")
		return len(fin), "\n" + pila[len(pila)-1].sangria + "}", true
	}
	return 0, "", false
}

func anchoSangria(s string) int {
	n := 0
	for _, c := range s {
		if c == '\t' {
			n += 4
		} else {
			n++
		}
	}
	return n
}
