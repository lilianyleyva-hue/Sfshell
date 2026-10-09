package analisis

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// paquetesConocidos maps the usual name of a standard library package to its import path; Normalizar
// uses it to add the imports a snippet forgot.
var paquetesConocidos = map[string]string{
	"fmt": "fmt", "strings": "strings", "strconv": "strconv", "sort": "sort", "slices": "slices",
	"maps": "maps", "math": "math", "rand": "math/rand", "big": "math/big", "bits": "math/bits",
	"cmplx": "math/cmplx", "unicode": "unicode", "utf8": "unicode/utf8", "utf16": "unicode/utf16",
	"os": "os", "bufio": "bufio", "io": "io", "errors": "errors", "time": "time", "bytes": "bytes",
	"regexp": "regexp", "json": "encoding/json", "hex": "encoding/hex", "base64": "encoding/base64",
	"csv": "encoding/csv", "binary": "encoding/binary", "http": "net/http", "url": "net/url",
	"heap": "container/heap", "list": "container/list", "ring": "container/ring", "sync": "sync",
	"atomic": "sync/atomic", "filepath": "path/filepath", "path": "path", "cmp": "cmp", "context": "context",
	"sha256": "crypto/sha256", "md5": "crypto/md5", "exec": "os/exec", "log": "log", "flag": "flag",
	"tabwriter": "text/tabwriter", "template": "text/template", "fs": "io/fs", "runtime": "runtime",
	"reflect": "reflect", "crc32": "hash/crc32", "fnv": "hash/fnv", "html": "html", "netip": "net/netip",
	"signal": "os/signal", "debug": "runtime/debug", "httptest": "net/http/httptest", "user": "os/user",
}

// Normalizar turns a snippet into a complete, gofmt'd file of package paquete ("" keeps the package
// clause, or uses main): it sets the package clause, wraps bare statements in func main(), keeps bare
// functions and types as top-level declarations, and adds the standard library imports it can infer.
func Normalizar(fuente, paquete string) (string, error) {
	src := strings.TrimSpace(strings.ReplaceAll(fuente, "\r\n", "\n"))
	if src == "" {
		return "", ErrVacio
	}
	fset := token.NewFileSet()
	var f *ast.File
	var err error
	if tienePaquete(src) {
		f, err = parser.ParseFile(fset, "x.go", src, parser.ParseComments)
		if err != nil {
			return "", errorDeSintaxis(err)
		}
	} else {
		texto, err2 := armar(src)
		if err2 != nil {
			return "", err2
		}
		f, err = parser.ParseFile(fset, "x.go", texto, parser.ParseComments)
		if err != nil {
			return "", errorDeSintaxis(err)
		}
	}
	if paquete != "" {
		f.Name.Name = paquete
	}
	agregarImports(fset, f)
	var buf bytes.Buffer
	if err := (&printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}).Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	out, err := format.Source(buf.Bytes())
	if err != nil {
		return "", errorDeSintaxis(err)
	}
	return string(out), nil
}

func errorDeSintaxis(err error) error {
	var lista scanner.ErrorList
	if errors.As(err, &lista) && len(lista) > 0 {
		e := lista[0]
		return fmt.Errorf("el código no se puede leer (línea %d): %s", e.Pos.Line, e.Msg)
	}
	return fmt.Errorf("el código no se puede leer: %w", err)
}

// trozo is a piece of a snippet: a top-level declaration or a run of statements.
type trozo struct {
	texto string
	decl  bool
	imp   bool
}

// armar builds "package main" + imports + declarations + func main() { statements }.
func armar(src string) (string, error) {
	trozos := partirTrozos(src)
	var imports, decls, stmts []string
	tieneMain := false
	for _, t := range trozos {
		switch {
		case t.imp:
			imports = append(imports, t.texto)
		case t.decl:
			decls = append(decls, t.texto)
			if strings.HasPrefix(strings.TrimSpace(t.texto), "func main(") {
				tieneMain = true
			}
		default:
			if strings.TrimSpace(t.texto) != "" {
				stmts = append(stmts, t.texto)
			}
		}
	}
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for _, im := range imports {
		sb.WriteString(im)
		sb.WriteString("\n")
	}
	if len(stmts) > 0 {
		if tieneMain {
			return "", errors.New("hay instrucciones sueltas y también una función main: no sé dónde ponerlas")
		}
		sb.WriteString("\nfunc main() {\n")
		for _, s := range stmts {
			sb.WriteString(s)
			sb.WriteString("\n")
		}
		sb.WriteString("}\n")
	}
	for _, d := range decls {
		sb.WriteString("\n")
		sb.WriteString(d)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// partirTrozos splits a snippet into top-level declarations (func, type, import, and var/const blocks
// that start a line before any statement) and statements, using go/scanner so strings and comments are safe.
func partirTrozos(src string) []trozo {
	fset := token.NewFileSet()
	fl := fset.AddFile("x.go", -1, len(src))
	var s scanner.Scanner
	s.Init(fl, []byte(src), nil, scanner.ScanComments)
	var toks []fichaGo
	for {
		p, t, l := s.Scan()
		if t == token.EOF {
			break
		}
		toks = append(toks, fichaGo{p, t, l})
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	var out []trozo
	inicio := 0     // byte offset of the current statement run
	prof := 0       // bracket depth
	inicioLinea := true
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if prof == 0 && inicioLinea && (t.tok == token.FUNC || t.tok == token.TYPE || t.tok == token.IMPORT) {
			// a func literal statement "func() { … }()" is not a declaration
			if t.tok == token.FUNC && i+1 < len(toks) && toks[i+1].tok == token.LPAREN && esLiteralFunc(toks, i) {
				goto normal
			}
			// close the statements before it
			if o := off(t.pos); o > inicio {
				out = append(out, trozo{texto: src[inicio:o]})
			}
			// find the end of the declaration: depth back to 0 and then a ';' (newline) token
			j := i + 1
			d := 0
			for ; j < len(toks); j++ {
				switch toks[j].tok {
				case token.LPAREN, token.LBRACE, token.LBRACK:
					d++
				case token.RPAREN, token.RBRACE, token.RBRACK:
					d--
				}
				if d == 0 && toks[j].tok == token.SEMICOLON {
					break
				}
			}
			fin := len(src)
			if j < len(toks) {
				fin = off(toks[j].pos)
				if toks[j].lit == "\n" {
					fin++
				} else {
					fin += len(toks[j].lit)
				}
				if fin > len(src) {
					fin = len(src)
				}
			}
			out = append(out, trozo{texto: strings.TrimRight(src[off(t.pos):fin], "\n"), decl: true, imp: t.tok == token.IMPORT})
			inicio = fin
			i = j
			inicioLinea = true
			continue
		}
	normal:
		switch t.tok {
		case token.LPAREN, token.LBRACE, token.LBRACK:
			prof++
		case token.RPAREN, token.RBRACE, token.RBRACK:
			prof--
		}
		inicioLinea = t.tok == token.SEMICOLON || t.tok == token.COMMENT
	}
	if inicio < len(src) {
		out = append(out, trozo{texto: src[inicio:]})
	}
	return out
}

// fichaGo is one token of a snippet.
type fichaGo struct {
	pos token.Pos
	tok token.Token
	lit string
}

// esLiteralFunc reports whether the func at toks[i] is a function literal that is called right away
// ("func() { … }()"): the brace that closes its body is followed by "(".
func esLiteralFunc(toks []fichaGo, i int) bool {
	j := i + 1
	paren := 0
	for ; j < len(toks); j++ {
		switch toks[j].tok {
		case token.LPAREN, token.LBRACK:
			paren++
		case token.RPAREN, token.RBRACK:
			paren--
		}
		if paren == 0 && toks[j].tok == token.LBRACE {
			break
		}
	}
	d := 0
	for ; j < len(toks); j++ {
		switch toks[j].tok {
		case token.LBRACE:
			d++
		case token.RBRACE:
			d--
			if d == 0 {
				return j+1 < len(toks) && toks[j+1].tok == token.LPAREN
			}
		}
	}
	return false
}

// agregarImports adds an import for every unresolved package-like identifier used as X in X.Sel.
func agregarImports(fset *token.FileSet, f *ast.File) {
	tiene := map[string]bool{}
	for _, im := range f.Imports {
		ruta, _ := strconv.Unquote(im.Path.Value)
		nombre := ruta[strings.LastIndex(ruta, "/")+1:]
		if im.Name != nil {
			nombre = im.Name.Name
		}
		tiene[nombre] = true
	}
	declarados := map[string]bool{}
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Recv == nil {
				declarados[x.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, sp := range x.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					declarados[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, n := range s.Names {
						declarados[n.Name] = true
					}
				}
			}
		}
	}
	sinResolver := map[string]bool{}
	for _, id := range f.Unresolved {
		sinResolver[id.Name] = true
	}
	faltan := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !sinResolver[id.Name] || tiene[id.Name] || declarados[id.Name] {
			return true
		}
		if ruta, ok := paquetesConocidos[id.Name]; ok {
			faltan[ruta] = true
		}
		return true
	})
	if len(faltan) == 0 {
		return
	}
	rutas := make([]string, 0, len(faltan))
	for r := range faltan {
		rutas = append(rutas, r)
	}
	sort.Strings(rutas)
	specs := make([]ast.Spec, 0, len(rutas))
	for _, r := range rutas {
		specs = append(specs, &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(r)}})
	}
	// merge into the first import declaration, or add a new one
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			if !gd.Lparen.IsValid() {
				gd.Lparen, gd.Rparen = gd.Pos(), gd.End()
			}
			gd.Specs = append(gd.Specs, specs...)
			ast.SortImports(fset, f)
			return
		}
	}
	gd := &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
	if len(specs) > 1 {
		gd.Lparen = 1
	}
	f.Decls = append([]ast.Decl{gd}, f.Decls...)
}

// Funcion extracts one function (or method, "Tipo.Metodo") with the declarations it needs: the
// functions it calls, the types, constants and variables it uses, the methods of those types, and the
// imports, as a gofmt'd file of the same package.
func Funcion(fuente, nombre string) (string, error) {
	a, err := parsear(fuente)
	if err != nil {
		return "", err
	}
	fd := buscarFunc(a.f, nombre)
	if fd == nil {
		return "", fmt.Errorf("no encuentro la función %s", nombre)
	}
	// top-level names → declarations
	nivel := map[string][]ast.Decl{}
	for _, d := range a.f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Recv == nil {
				nivel[x.Name.Name] = append(nivel[x.Name.Name], x)
			} else {
				t := nombreReceptor(x.Recv.List[0].Type)
				nivel["método "+t] = append(nivel["método "+t], x)
			}
		case *ast.GenDecl:
			if x.Tok == token.IMPORT {
				continue
			}
			for _, sp := range x.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					nivel[s.Name.Name] = append(nivel[s.Name.Name], x)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						nivel[n.Name] = append(nivel[n.Name], x)
					}
				}
			}
		}
	}
	incluidas := map[ast.Decl]bool{}
	tiposVistos := map[string]bool{}
	var cola []ast.Decl
	agregar := func(d ast.Decl) {
		if !incluidas[d] {
			incluidas[d] = true
			cola = append(cola, d)
		}
	}
	agregar(fd)
	usados := map[string]bool{}
	for len(cola) > 0 {
		d := cola[0]
		cola = cola[1:]
		ast.Inspect(d, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					usados[id.Name] = true
				}
			case *ast.Ident:
				for _, dd := range nivel[x.Name] {
					agregar(dd)
				}
				if len(nivel[x.Name]) > 0 && !tiposVistos[x.Name] {
					tiposVistos[x.Name] = true
					for _, m := range nivel["método "+x.Name] {
						agregar(m)
					}
				}
			}
			return true
		})
	}
	var sb strings.Builder
	sb.WriteString("package " + a.f.Name.Name + "\n\n")
	var imps []string
	for _, im := range a.f.Imports {
		ruta, _ := strconv.Unquote(im.Path.Value)
		nombreImp := ruta[strings.LastIndex(ruta, "/")+1:]
		if im.Name != nil {
			nombreImp = im.Name.Name
		}
		if usados[nombreImp] || nombreImp == "_" || nombreImp == "." {
			imps = append(imps, "\t"+textoNodo(a.fset, im))
		}
	}
	if len(imps) > 0 {
		sb.WriteString("import (\n" + strings.Join(imps, "\n") + "\n)\n\n")
	}
	for _, d := range a.f.Decls {
		if !incluidas[d] {
			continue
		}
		sb.WriteString(textoConDoc(a, d))
		sb.WriteString("\n\n")
	}
	out, err := format.Source([]byte(sb.String()))
	if err != nil {
		return "", errorDeSintaxis(err)
	}
	return string(out), nil
}

// textoConDoc prints a declaration with its doc comment.
func textoConDoc(a *archivo, d ast.Decl) string {
	var buf bytes.Buffer
	cf := &printer.CommentedNode{Node: d, Comments: a.f.Comments}
	if err := (&printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}).Fprint(&buf, a.fset, cf); err != nil {
		return textoNodo(a.fset, d)
	}
	doc := ""
	switch x := d.(type) {
	case *ast.FuncDecl:
		if x.Doc != nil {
			doc = textoNodo(a.fset, x.Doc)
		}
	case *ast.GenDecl:
		if x.Doc != nil {
			doc = textoNodo(a.fset, x.Doc)
		}
	}
	s := buf.String()
	if doc != "" && !strings.HasPrefix(strings.TrimSpace(s), "//") {
		s = doc + "\n" + s
	}
	return s
}

func textoNodo(fset *token.FileSet, n ast.Node) string {
	var buf bytes.Buffer
	if c, ok := n.(*ast.CommentGroup); ok {
		var ls []string
		for _, x := range c.List {
			ls = append(ls, x.Text)
		}
		return strings.Join(ls, "\n")
	}
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	return buf.String()
}
