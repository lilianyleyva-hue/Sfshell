package saber

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Go candidates from web code: parsing code blocks, building complete files and signatures,
// adapting them to a target signature, and mining hints. Nothing here runs code.

// Candidata is a piece of web code ready for the sandbox.
type Candidata struct {
	URL        string
	Codigo     string // complete file after normalization
	Funcion    string // function name ("" for programs)
	Firma      *nucleo.Firma
	EsPrograma bool
	Imports    []string
	Votos      int
}

// Adaptacion is a candidate file with a wrapper named like the target.
type Adaptacion struct {
	Fuente string   // file with a wrapper named like the target
	Pasos  []string // Spanish: "renombro sumEven a SumaPares", "convierto []int64 en []int"
}

const (
	maxAdaptaciones = 6
	paqueteFuncion  = "solucion"
)

// paquetesConocidos maps the usual qualifier of a standard package to its import path (to complete
// snippets that use fmt.Println without importing fmt).
var paquetesConocidos = map[string]string{
	"fmt": "fmt", "strings": "strings", "strconv": "strconv", "sort": "sort", "math": "math", "os": "os",
	"bufio": "bufio", "unicode": "unicode", "utf8": "unicode/utf8", "errors": "errors", "bytes": "bytes",
	"slices": "slices", "maps": "maps", "time": "time", "regexp": "regexp", "rand": "math/rand",
	"big": "math/big", "bits": "math/bits", "io": "io", "cmp": "cmp", "heap": "container/heap",
	"list": "container/list", "hex": "encoding/hex", "base64": "encoding/base64", "json": "encoding/json",
	"utf16": "unicode/utf16", "tabwriter": "text/tabwriter", "cmplx": "math/cmplx",
}

// ---- parsing ----

type bloqueGo struct {
	fset *token.FileSet
	file *ast.File
	modo int // 1 complete file, 2 declarations without package, 3 statements
}

// parsearBloque parses a code block as a file; failing that, as "package main" + the block; failing
// that, as "package main; func _() {" + the block + "}".
func parsearBloque(src string) (bloqueGo, error) {
	fset := token.NewFileSet()
	if f, err := parser.ParseFile(fset, "web.go", src, parser.ParseComments); err == nil {
		return bloqueGo{fset, f, 1}, nil
	}
	fset = token.NewFileSet()
	if f, err := parser.ParseFile(fset, "web.go", "package main\n\n"+src, parser.ParseComments); err == nil {
		return bloqueGo{fset, f, 2}, nil
	}
	fset = token.NewFileSet()
	f, err := parser.ParseFile(fset, "web.go", "package main\n\nfunc _() {\n"+src+"\n}\n", parser.ParseComments)
	if err != nil {
		return bloqueGo{}, err
	}
	// The block must be a pure statement list: the wrapper's own func _ is
	// the only declaration. A stray "}" in fetched text can close the wrapper
	// early and leave other declarations behind (e.g. "x()\n}\ntype T struct {").
	if funcEnvoltorio(f) == nil {
		return bloqueGo{}, errors.New("saber: el bloque no es una lista de sentencias")
	}
	return bloqueGo{fset, f, 3}, nil
}

// funcEnvoltorio returns the synthetic "func _" of a statement-mode block, or
// nil when the file is anything else than exactly that one declaration.
func funcEnvoltorio(f *ast.File) *ast.FuncDecl {
	if f == nil || len(f.Decls) != 1 {
		return nil
	}
	fd, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok || fd.Recv != nil || fd.Name == nil || fd.Name.Name != "_" || fd.Body == nil {
		return nil
	}
	return fd
}

// importacion is one import spec.
type importacion struct{ nombre, ruta string }

func (i importacion) calificador() string {
	if i.nombre != "" {
		return i.nombre
	}
	r := i.ruta
	partes := strings.Split(r, "/")
	ult := partes[len(partes)-1]
	if len(partes) > 1 && len(ult) >= 2 && ult[0] == 'v' && esNumero(ult[1:]) {
		ult = partes[len(partes)-2] // math/rand/v2 → rand
	}
	return ult
}

func (i importacion) Go() string {
	if i.nombre != "" {
		return i.nombre + " " + strconv.Quote(i.ruta)
	}
	return strconv.Quote(i.ruta)
}

func esNumero(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func importsDe(f *ast.File) []importacion {
	var out []importacion
	for _, s := range f.Imports {
		ruta, err := strconv.Unquote(s.Path.Value)
		if err != nil {
			continue
		}
		im := importacion{ruta: ruta}
		if s.Name != nil {
			im.nombre = s.Name.Name
		}
		out = append(out, im)
	}
	return out
}

// calificadoresUsados returns the X of every X.Sel where X is an unresolved identifier (a package).
func calificadoresUsados(nodos ...ast.Node) map[string]bool {
	usados := map[string]bool{}
	for _, n := range nodos {
		ast.Inspect(n, func(x ast.Node) bool {
			if s, ok := x.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && id.Obj == nil {
					usados[id.Name] = true
				}
			}
			return true
		})
	}
	return usados
}

// importsNecesarios keeps the imports used by decls and adds known standard packages that are used but
// not imported. Blank and dot imports are kept.
func importsNecesarios(existentes []importacion, decls []ast.Decl, locales map[string]bool) []importacion {
	nodos := make([]ast.Node, len(decls))
	for i, d := range decls {
		nodos[i] = d
	}
	usados := calificadoresUsados(nodos...)
	var out []importacion
	tiene := map[string]bool{}
	for _, im := range existentes {
		q := im.calificador()
		if im.nombre == "_" || im.nombre == "." || usados[q] {
			out = append(out, im)
			tiene[q] = true
		}
	}
	var faltan []string
	for q := range usados {
		if tiene[q] || locales[q] {
			continue
		}
		if ruta, ok := paquetesConocidos[q]; ok {
			faltan = append(faltan, ruta)
		}
	}
	sort.Strings(faltan)
	for _, r := range faltan {
		out = append(out, importacion{ruta: r})
	}
	return out
}

// nombresLocales: top-level names declared in the file (they are not packages).
func nombresLocales(f *ast.File) map[string]bool {
	m := map[string]bool{}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				m[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					m[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, n := range s.Names {
						m[n.Name] = true
					}
				}
			}
		}
	}
	return m
}

// archivoGo prints a complete file: package clause, imports and decls (comments kept), gofmt'd.
func archivoGo(paquete string, imports []importacion, decls []ast.Decl, b bloqueGo) string {
	var sb strings.Builder
	sb.WriteString("package " + paquete + "\n\n")
	if len(imports) == 1 {
		sb.WriteString("import " + imports[0].Go() + "\n\n")
	} else if len(imports) > 1 {
		sb.WriteString("import (\n")
		for _, im := range imports {
			sb.WriteString("\t" + im.Go() + "\n")
		}
		sb.WriteString(")\n\n")
	}
	for _, d := range decls {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, b.fset, &printer.CommentedNode{Node: d, Comments: b.file.Comments}); err != nil {
			continue
		}
		sb.Write(buf.Bytes())
		sb.WriteString("\n\n")
	}
	return gofmt(sb.String())
}

func gofmt(src string) string {
	out, err := format.Source([]byte(src))
	if err != nil {
		return src
	}
	return string(out)
}

// normalizado prints a node without comments and without white space (the dedupe key).
func normalizado(fset *token.FileSet, n ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(buf.String()), "")
}

func imprimeAlgo(src string) bool {
	return strings.Contains(src, "fmt.Print") || strings.Contains(src, "fmt.Fprint") || strings.Contains(src, "println(")
}

// Candidatas extracts the Go candidates of a document: every function (with the types and helpers of
// its block), and whole programs. Duplicates (same normalized source) are dropped; the best-voted come
// first.
func Candidatas(d Documento) []Candidata {
	var out []Candidata
	vistos := map[string]int{}
	agregar := func(c Candidata, clave string) {
		if i, ok := vistos[clave]; ok {
			if c.Votos > out[i].Votos {
				out[i].Votos = c.Votos
				out[i].URL = c.URL
			}
			return
		}
		vistos[clave] = len(out)
		out = append(out, c)
	}
	for _, bl := range d.Bloques {
		if bl.Tipo != "codigo" || strings.TrimSpace(bl.Texto) == "" {
			continue
		}
		b, err := parsearBloque(bl.Texto)
		if err != nil {
			continue
		}
		for _, c := range candidatasDeBloque(b, bl.Texto) {
			c.URL = d.URL
			c.Votos = bl.Votos
			agregar(c.Candidata, c.clave)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Votos > out[j].Votos })
	return out
}

type candidataClave struct {
	Candidata
	clave string
}

func candidatasDeBloque(b bloqueGo, texto string) []candidataClave {
	f := b.file
	for _, im := range importsDe(f) {
		if im.ruta == "testing" || im.ruta == "C" {
			return nil // tests and cgo are not candidates
		}
	}
	locales := nombresLocales(f)
	existentes := importsDe(f)
	var out []candidataClave
	if b.modo == 3 {
		if !imprimeAlgo(texto) {
			return nil
		}
		// statements only: a program whose main is the block
		fd := funcEnvoltorio(f)
		if fd == nil {
			return nil
		}
		fd.Name.Name = "main"
		imps := importsNecesarios(nil, []ast.Decl{fd}, locales)
		src := archivoGo("main", imps, []ast.Decl{fd}, b)
		return []candidataClave{{Candidata: Candidata{Codigo: src, EsPrograma: true, Imports: rutas(imps)},
			clave: "p:" + strings.Join(strings.Fields(src), "")}}
	}
	var main *ast.FuncDecl
	var resto []ast.Decl // everything but main, init and _
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == "main" {
				main = d
				continue
			}
			if d.Recv == nil && (d.Name.Name == "init" || d.Name.Name == "_") {
				continue
			}
			resto = append(resto, d)
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			resto = append(resto, d)
		}
	}
	esMain := f.Name.Name == "main" || b.modo == 2
	if main != nil && esMain {
		todos := append(append([]ast.Decl(nil), resto...), main)
		imps := importsNecesarios(existentes, todos, locales)
		var decls []ast.Decl
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
				continue
			}
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "_" {
				continue
			}
			decls = append(decls, d)
		}
		src := archivoGo("main", imps, decls, b)
		out = append(out, candidataClave{Candidata: Candidata{Codigo: src, EsPrograma: true, Imports: rutas(imps)},
			clave: "p:" + strings.Join(strings.Fields(src), "")})
	}
	tipos := tiposDeclarados(f)
	imps := importsNecesarios(existentes, resto, locales)
	var fuente string
	for _, d := range resto {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		if fuente == "" {
			fuente = archivoGo(paqueteFuncion, imps, resto, b)
		}
		c := Candidata{Codigo: fuente, Funcion: fd.Name.Name, Imports: rutas(imps)}
		if fi, err := firmaDe(fd, tipos); err == nil {
			c.Firma = &fi
		}
		clave := "f:" + receptorTexto(fd) + "." + fd.Name.Name + ":" + normalizado(b.fset, fd)
		out = append(out, candidataClave{Candidata: c, clave: clave})
	}
	return out
}

func rutas(imps []importacion) []string {
	out := make([]string, 0, len(imps))
	for _, im := range imps {
		out = append(out, im.ruta)
	}
	return out
}

func receptorTexto(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	nombre, ptr := receptorBase(fd.Recv.List[0].Type)
	if ptr {
		return "*" + nombre
	}
	return nombre
}

// receptorBase returns the type name of a receiver expression and whether it is a pointer.
func receptorBase(e ast.Expr) (string, bool) {
	ptr := false
	if s, ok := e.(*ast.StarExpr); ok {
		ptr = true
		e = s.X
	}
	if p, ok := e.(*ast.ParenExpr); ok {
		e = p.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name, ptr
	}
	return "", ptr
}

// ---- types ----

var basicos = map[string]nucleo.Clase{
	"int": nucleo.CInt, "int8": nucleo.CInt, "int16": nucleo.CInt, "int32": nucleo.CInt, "int64": nucleo.CInt,
	"uint": nucleo.CInt, "uint8": nucleo.CInt, "uint16": nucleo.CInt, "uint32": nucleo.CInt, "uint64": nucleo.CInt,
	"uintptr": nucleo.CInt, "float32": nucleo.CFloat, "float64": nucleo.CFloat, "bool": nucleo.CBool,
	"string": nucleo.CString, "rune": nucleo.CRune, "byte": nucleo.CByte, "error": nucleo.CError,
}

func tiposDeclarados(f *ast.File) map[string]*ast.TypeSpec {
	m := map[string]*ast.TypeSpec{}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, s := range g.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok {
				m[ts.Name.Name] = ts
			}
		}
	}
	return m
}

// tipoDe converts a type expression into a nucleo.Tipo, resolving types declared in the same block.
func tipoDe(e ast.Expr, tipos map[string]*ast.TypeSpec, prof int) (nucleo.Tipo, error) {
	if prof > 12 {
		return nucleo.Tipo{}, fmt.Errorf("tipo demasiado anidado")
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return tipoDe(x.X, tipos, prof+1)
	case *ast.Ident:
		if c, ok := basicos[x.Name]; ok {
			return nucleo.Tipo{Clase: c, Nombre: x.Name}, nil
		}
		ts, ok := tipos[x.Name]
		if !ok {
			return nucleo.Tipo{}, fmt.Errorf("no conozco el tipo %s", x.Name)
		}
		if ts.TypeParams != nil {
			return nucleo.Tipo{}, fmt.Errorf("%s es genérico", x.Name)
		}
		sub, err := tipoDe(ts.Type, tipos, prof+1)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		if ts.Assign.IsValid() { // alias
			return sub, nil
		}
		if sub.Clase == nucleo.CError || sub.Clase == nucleo.CPuntero || sub.Clase == nucleo.CArreglo {
			return nucleo.Tipo{}, fmt.Errorf("no sé usar el tipo %s", x.Name)
		}
		sub.Nombre = x.Name
		sub.Definido = true
		return sub, nil
	case *ast.ArrayType:
		el, err := tipoDe(x.Elt, tipos, prof+1)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		if el.Clase == nucleo.CError {
			return nucleo.Tipo{}, fmt.Errorf("listas de error no")
		}
		if x.Len == nil {
			return nucleo.ListaDe(el), nil
		}
		lit, ok := x.Len.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return nucleo.Tipo{}, fmt.Errorf("tamaño de arreglo no literal")
		}
		n, err := strconv.Atoi(lit.Value)
		if err != nil || n < 0 || n > 1<<16 {
			return nucleo.Tipo{}, fmt.Errorf("tamaño de arreglo no válido")
		}
		return nucleo.ArregloDe(n, el), nil
	case *ast.MapType:
		k, err := tipoDe(x.Key, tipos, prof+1)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		switch k.Clase {
		case nucleo.CInt, nucleo.CString, nucleo.CRune, nucleo.CByte, nucleo.CBool:
		default:
			return nucleo.Tipo{}, fmt.Errorf("clave de mapa no soportada")
		}
		v, err := tipoDe(x.Value, tipos, prof+1)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		return nucleo.MapaDe(k, v), nil
	case *ast.StarExpr:
		el, err := tipoDe(x.X, tipos, prof+1)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		if el.Clase != nucleo.CStruct {
			return nucleo.Tipo{}, fmt.Errorf("solo punteros a estructuras")
		}
		return nucleo.PunteroA(el), nil
	case *ast.StructType:
		t := nucleo.Tipo{Clase: nucleo.CStruct}
		for _, fl := range x.Fields.List {
			if len(fl.Names) == 0 {
				return nucleo.Tipo{}, fmt.Errorf("campos embebidos no")
			}
			ft, err := tipoDe(fl.Type, tipos, prof+1)
			if err != nil {
				return nucleo.Tipo{}, err
			}
			for _, n := range fl.Names {
				t.Campos = append(t.Campos, nucleo.Campo{Nombre: n.Name, Tipo: ft})
			}
		}
		return t, nil
	}
	return nucleo.Tipo{}, fmt.Errorf("tipo no soportado")
}

// firmaDe builds the nucleo.Firma of a function declaration.
func firmaDe(fd *ast.FuncDecl, tipos map[string]*ast.TypeSpec) (nucleo.Firma, error) {
	if fd.Type.TypeParams != nil && len(fd.Type.TypeParams.List) > 0 {
		return nucleo.Firma{}, fmt.Errorf("%s es genérica", fd.Name.Name)
	}
	f := nucleo.Firma{Nombre: fd.Name.Name}
	if fd.Recv != nil && len(fd.Recv.List) == 1 {
		r := fd.Recv.List[0]
		t, err := tipoDe(r.Type, tipos, 0)
		if err != nil {
			return nucleo.Firma{}, err
		}
		p := nucleo.Param{Tipo: t}
		if len(r.Names) > 0 {
			p.Nombre = r.Names[0].Name
		}
		f.Receptor = &p
	}
	if fd.Type.Params != nil {
		for i, fl := range fd.Type.Params.List {
			te := fl.Type
			if el, ok := te.(*ast.Ellipsis); ok {
				if i != len(fd.Type.Params.List)-1 {
					return nucleo.Firma{}, fmt.Errorf("variádica en medio")
				}
				f.Variadica = true
				te = &ast.ArrayType{Elt: el.Elt}
			}
			t, err := tipoDe(te, tipos, 0)
			if err != nil {
				return nucleo.Firma{}, err
			}
			if len(fl.Names) == 0 {
				f.Params = append(f.Params, nucleo.Param{Tipo: t})
			}
			for _, n := range fl.Names {
				f.Params = append(f.Params, nucleo.Param{Nombre: n.Name, Tipo: t})
			}
		}
	}
	if fd.Type.Results != nil {
		for _, fl := range fd.Type.Results.List {
			t, err := tipoDe(fl.Type, tipos, 0)
			if err != nil {
				return nucleo.Firma{}, err
			}
			n := len(fl.Names)
			if n == 0 {
				n = 1
			}
			for i := 0; i < n; i++ {
				f.Res = append(f.Res, t)
			}
		}
	}
	return f, nil
}

// ---- adaptation ----

func esEnteroT(t nucleo.Tipo) bool {
	return t.Clase == nucleo.CInt || t.Clase == nucleo.CRune || t.Clase == nucleo.CByte
}

func esListaDe(t nucleo.Tipo, pred func(nucleo.Tipo) bool) bool {
	return t.Clase == nucleo.CLista && t.Elem != nil && pred(*t.Elem)
}

func esByte(t nucleo.Tipo) bool {
	return t.Clase == nucleo.CByte || (t.Clase == nucleo.CInt && t.Nombre == "uint8" && !t.Definido)
}

func esRuna(t nucleo.Tipo) bool {
	return t.Clase == nucleo.CRune || (t.Clase == nucleo.CInt && t.Nombre == "int32" && !t.Definido)
}

// subyacente drops the defined name at the top level.
func subyacente(t nucleo.Tipo) nucleo.Tipo {
	if !t.Definido {
		return t
	}
	t.Definido = false
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat, nucleo.CBool, nucleo.CString, nucleo.CRune, nucleo.CByte:
		t.Nombre = map[nucleo.Clase]string{nucleo.CInt: "int", nucleo.CFloat: "float64", nucleo.CBool: "bool",
			nucleo.CString: "string", nucleo.CRune: "rune", nucleo.CByte: "byte"}[t.Clase]
		if t.Clase == nucleo.CInt || t.Clase == nucleo.CFloat {
			t.Nombre = "" // unknown width: never "identical"
		}
	default:
		t.Nombre = ""
	}
	return t
}

// convertible reports whether a value of type de can be turned into type a by the generated wrapper.
func convertible(de, a nucleo.Tipo, prof int) bool {
	if prof > 6 {
		return false
	}
	if de.Igual(a) {
		return true
	}
	switch {
	case esEnteroT(de) && esEnteroT(a):
		return true
	case de.Clase == nucleo.CFloat && a.Clase == nucleo.CFloat:
		return true
	case de.Clase == nucleo.CBool && a.Clase == nucleo.CBool:
		return true
	case de.Clase == nucleo.CString && a.Clase == nucleo.CString:
		return true
	case de.Clase == nucleo.CString && (esListaDe(a, esByte) || esListaDe(a, esRuna)):
		return true
	case a.Clase == nucleo.CString && (esListaDe(de, esByte) || esListaDe(de, esRuna)):
		return true
	case de.Clase == nucleo.CLista && a.Clase == nucleo.CLista:
		return convertible(*de.Elem, *a.Elem, prof+1)
	case de.Clase == nucleo.CMapa && a.Clase == nucleo.CMapa:
		return convertible(*de.Clave, *a.Clave, prof+1) && convertible(*de.Elem, *a.Elem, prof+1)
	case (de.Definido || a.Definido) && subyacente(de).Igual(subyacente(a)) && de.Clase != nucleo.CStruct:
		return true
	}
	return false
}

type generador struct {
	sb     strings.Builder
	sangr  int
	n      int
	ocupar map[string]bool
}

func (g *generador) linea(formato string, a ...any) {
	g.sb.WriteString(strings.Repeat("\t", g.sangr))
	fmt.Fprintf(&g.sb, formato, a...)
	g.sb.WriteByte('\n')
}

func (g *generador) nombre(base string) string {
	for {
		g.n++
		n := base + strconv.Itoa(g.n)
		if !g.ocupar[n] {
			g.ocupar[n] = true
			return n
		}
	}
}

// conv returns an expression of type a for expr (of type de), emitting loops when needed.
func (g *generador) conv(expr string, de, a nucleo.Tipo) string {
	if de.Igual(a) {
		return expr
	}
	switch {
	case de.Clase == nucleo.CLista && a.Clase == nucleo.CLista && !(esSoloNombre(de, a)):
		if de.Elem.Igual(*a.Elem) {
			return a.Go() + "(" + expr + ")"
		}
		dst := g.nombre("conv")
		k, v := g.nombre("k"), g.nombre("v")
		g.linea("%s := make(%s, len(%s))", dst, a.Go(), expr)
		g.linea("for %s, %s := range %s {", k, v, expr)
		g.sangr++
		el := g.conv(v, *de.Elem, *a.Elem)
		g.linea("%s[%s] = %s", dst, k, el)
		g.sangr--
		g.linea("}")
		return dst
	case de.Clase == nucleo.CMapa && a.Clase == nucleo.CMapa && !(esSoloNombre(de, a)):
		dst := g.nombre("conv")
		k, v := g.nombre("k"), g.nombre("v")
		g.linea("%s := make(%s, len(%s))", dst, a.Go(), expr)
		g.linea("for %s, %s := range %s {", k, v, expr)
		g.sangr++
		ck := g.conv(k, *de.Clave, *a.Clave)
		cv := g.conv(v, *de.Elem, *a.Elem)
		g.linea("%s[%s] = %s", dst, ck, cv)
		g.sangr--
		g.linea("}")
		return dst
	}
	return a.Go() + "(" + expr + ")"
}

// esSoloNombre: the two types differ only by a defined name (a plain conversion works).
func esSoloNombre(de, a nucleo.Tipo) bool {
	return (de.Definido || a.Definido) && subyacente(de).Igual(subyacente(a))
}

type entradaCand struct {
	tipo      nucleo.Tipo
	receptor  bool
	variadica bool
}

// Adaptar turns a candidate into files whose function has the target's name and signature: matching
// parameter types modulo conversions ([]int64↔[]int, int32↔rune, []byte↔string, float32↔float64),
// trying parameter permutations, dropping a trailing error result (the wrapper panics when it is not
// nil), filling an extra trailing n int with len(xs), and wrapping methods on slice types. At most 6.
func Adaptar(c Candidata, objetivo nucleo.Firma) []Adaptacion {
	if c.EsPrograma || c.Funcion == "" || objetivo.Nombre == "" || !token.IsIdentifier(objetivo.Nombre) {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "web.go", c.Codigo, parser.ParseComments)
	if err != nil {
		return nil
	}
	tipos := tiposDeclarados(file)
	var fd *ast.FuncDecl
	for _, d := range file.Decls {
		x, ok := d.(*ast.FuncDecl)
		if !ok || x.Name.Name != c.Funcion || x.Body == nil {
			continue
		}
		if c.Firma != nil && (c.Firma.Receptor != nil) != (x.Recv != nil) {
			continue
		}
		fd = x
		break
	}
	if fd == nil {
		return nil
	}
	cf, err := firmaDe(fd, tipos)
	if err != nil && fd.Recv == nil {
		return nil
	}
	locales := nombresLocales(file)
	var recvNombre string
	if fd.Recv != nil {
		// receivers: methods on defined slice/map/basic types are wrapped; struct methods only when the
		// target is a method on the same type.
		nombre, ptr := receptorBase(fd.Recv.List[0].Type)
		if _, ok := tipos[nombre]; !ok {
			return nil
		}
		t, err2 := tipoDe(&ast.Ident{Name: nombre}, tipos, 0)
		if err2 != nil {
			return nil
		}
		recvNombre = nombre
		// rebuild the firma with the plain receiver type (pointers to non-structs are not nucleo types)
		fdSin := *fd
		fdSin.Recv = nil
		cf, err = firmaDe(&fdSin, tipos)
		if err != nil {
			return nil
		}
		cf.Receptor = &nucleo.Param{Tipo: t}
		if t.Clase == nucleo.CStruct {
			if objetivo.Receptor == nil || !objetivo.Receptor.Tipo.Igual(t) || ptr {
				return nil
			}
		}
	}
	// inputs of the candidate
	var ent []entradaCand
	structMetodo := cf.Receptor != nil && cf.Receptor.Tipo.Clase == nucleo.CStruct
	if cf.Receptor != nil && !structMetodo {
		ent = append(ent, entradaCand{tipo: cf.Receptor.Tipo, receptor: true})
	}
	for i, p := range cf.Params {
		ent = append(ent, entradaCand{tipo: p.Tipo, variadica: cf.Variadica && i == len(cf.Params)-1})
	}
	objParams := objetivo.Params
	// extra trailing n int
	extraN := false
	if len(ent) == len(objParams)+1 && len(ent) > 0 {
		ult := ent[len(ent)-1]
		if ult.tipo.Clase == nucleo.CInt && !ult.receptor && !ult.variadica {
			extraN = true
			ent = ent[:len(ent)-1]
		}
	}
	if len(ent) != len(objParams) || len(ent) > 6 {
		return nil
	}
	// results
	res := cf.Res
	quitaError := false
	if len(res) > 0 && res[len(res)-1].Clase == nucleo.CError &&
		(len(objetivo.Res) == 0 || objetivo.Res[len(objetivo.Res)-1].Clase != nucleo.CError) {
		quitaError = true
		res = res[:len(res)-1]
	}
	if len(res) != len(objetivo.Res) {
		return nil
	}
	for i := range res {
		if !convertible(res[i], objetivo.Res[i], 0) {
			return nil
		}
	}
	// parameter assignments: identity first, then the other permutations
	perms := permutaciones(len(ent))
	var validas [][]int
	for _, p := range perms {
		ok := true
		for i, e := range ent {
			if !convertible(objParams[p[i]].Tipo, e.tipo, 0) {
				ok = false
				break
			}
		}
		if ok {
			validas = append(validas, p)
			if len(validas) >= maxAdaptaciones {
				break
			}
		}
	}
	if len(validas) == 0 {
		return nil
	}
	// the length source for an extra n
	lenDe := -1
	if extraN {
		for i, p := range objParams {
			k := p.Tipo.Clase
			if k == nucleo.CLista || k == nucleo.CString || k == nucleo.CMapa || k == nucleo.CArreglo {
				lenDe = i
				break
			}
		}
		if lenDe < 0 {
			return nil
		}
	}
	nombreCand := c.Funcion
	// a top-level name equal to the target (other than the candidate itself) would collide
	if locales[objetivo.Nombre] && !(objetivo.Nombre == nombreCand && fd.Recv == nil) {
		return nil
	}
	if fd.Recv != nil && structMetodo {
		// the method set of the type must not already have the target name
		for _, d := range file.Decls {
			if x, ok := d.(*ast.FuncDecl); ok && x.Recv != nil && x.Name.Name == objetivo.Nombre {
				return nil
			}
		}
	}
	var out []Adaptacion
	for _, p := range validas {
		identidad := true
		for i := range p {
			if p[i] != i {
				identidad = false
			}
		}
		// plain rename: same types, same order, nothing to drop or fill, a plain function
		if identidad && !extraN && !quitaError && fd.Recv == nil && tiposIguales(ent, objParams, res, objetivo.Res) && cf.Variadica == objetivo.Variadica {
			src, err := renombrarFuncion(c.Codigo, nombreCand, objetivo.Nombre)
			if err != nil {
				continue
			}
			pasos := []string{"uso " + nombreCand + " tal cual"}
			if nombreCand != objetivo.Nombre {
				pasos = []string{"renombro " + nombreCand + " a " + objetivo.Nombre}
			}
			out = append(out, Adaptacion{Fuente: src, Pasos: pasos})
			continue
		}
		fuente := c.Codigo
		llamada := nombreCand
		if nombreCand == objetivo.Nombre && fd.Recv == nil {
			nuevo := nombreCand + "Web"
			for locales[nuevo] {
				nuevo += "_"
			}
			var err error
			fuente, err = renombrarFuncion(c.Codigo, nombreCand, nuevo)
			if err != nil {
				continue
			}
			llamada = nuevo
		}
		envoltura, pasos := envolver(objetivo, cf, ent, p, llamada, recvNombre, structMetodo, extraN, lenDe, quitaError, locales)
		src := gofmt(strings.TrimRight(fuente, "\n") + "\n\n" + envoltura)
		out = append(out, Adaptacion{Fuente: src, Pasos: pasos})
	}
	return out
}

func tiposIguales(ent []entradaCand, obj []nucleo.Param, res, objRes []nucleo.Tipo) bool {
	for i := range ent {
		if !ent[i].tipo.Igual(obj[i].Tipo) {
			return false
		}
	}
	for i := range res {
		if !res[i].Igual(objRes[i]) {
			return false
		}
	}
	return true
}

var reservadas = map[string]bool{"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true, "range": true,
	"return": true, "select": true, "struct": true, "switch": true, "type": true, "var": true,
	"len": true, "make": true, "panic": true, "string": true, "int": true, "append": true, "nil": true}

// envolver writes the wrapper function named like the target.
// A pointer receiver on a slice type works too: the receiver is first stored in an addressable variable.
func envolver(obj, cf nucleo.Firma, ent []entradaCand, perm []int, llamada, recvNombre string, structMetodo, extraN bool,
	lenDe int, quitaError bool, locales map[string]bool) (string, []string) {
	g := &generador{sangr: 1, ocupar: map[string]bool{}}
	for n := range locales {
		g.ocupar[n] = true
	}
	g.ocupar[llamada] = true
	// wrapper parameter names: the target's when usable
	cab := obj
	cab.Params = append([]nucleo.Param(nil), obj.Params...)
	for i := range cab.Params {
		n := cab.Params[i].Nombre
		if n == "" || n == "_" || reservadas[n] || g.ocupar[n] || paquetesConocidos[n] != "" {
			n = "p" + strconv.Itoa(i)
			for g.ocupar[n] {
				n += "_"
			}
		}
		cab.Params[i].Nombre = n
		g.ocupar[n] = true
	}
	var recvExpr string
	if structMetodo {
		r := *obj.Receptor
		if r.Nombre == "" || r.Nombre == "_" || g.ocupar[r.Nombre] {
			r.Nombre = "rec"
			for g.ocupar[r.Nombre] {
				r.Nombre += "_"
			}
		}
		g.ocupar[r.Nombre] = true
		cab.Receptor = &r
		recvExpr = r.Nombre
	}
	var pasos []string
	pasos = append(pasos, "envuelvo "+llamada+" en "+obj.Nombre)
	identidad := true
	for i := range perm {
		if perm[i] != i {
			identidad = false
		}
	}
	if !identidad {
		pasos = append(pasos, "cambio el orden de los parámetros")
	}
	var args []string
	for i, e := range ent {
		op := cab.Params[perm[i]]
		de := op.Tipo
		expr := g.conv(op.Nombre, de, e.tipo)
		if expr != op.Nombre {
			pasos = append(pasos, fmt.Sprintf("convierto %s (%s) en %s", op.Nombre, de.Go(), e.tipo.Go()))
		}
		if e.receptor {
			r := g.nombre("rec")
			g.linea("%s := %s", r, expr)
			recvExpr = r
			pasos = append(pasos, "uso el método "+llamada+" del tipo "+recvNombre)
			continue
		}
		if e.variadica {
			expr += "..."
		}
		args = append(args, expr)
	}
	if extraN {
		n := cab.Params[lenDe].Nombre
		args = append(args, "len("+n+")")
		pasos = append(pasos, "relleno el último parámetro con len("+n+")")
	}
	llamadaCompleta := llamada + "(" + strings.Join(args, ", ") + ")"
	if recvExpr != "" {
		llamadaCompleta = recvExpr + "." + llamada + "(" + strings.Join(args, ", ") + ")"
	}
	nres := len(cf.Res)
	var resNombres []string
	for i := 0; i < len(obj.Res); i++ {
		resNombres = append(resNombres, g.nombre("res"))
	}
	errNombre := ""
	if quitaError {
		errNombre = g.nombre("err")
		pasos = append(pasos, "quito el error final: si no es nil, la prueba falla")
	}
	switch {
	case nres == 0:
		g.linea("%s", llamadaCompleta)
	default:
		izq := append([]string(nil), resNombres...)
		if quitaError {
			izq = append(izq, errNombre)
		}
		g.linea("%s := %s", strings.Join(izq, ", "), llamadaCompleta)
		if quitaError {
			g.linea("if %s != nil {", errNombre)
			g.linea("\tpanic(%s)", errNombre)
			g.linea("}")
		}
		var rets []string
		for i, rn := range resNombres {
			de := cf.Res[i]
			expr := g.conv(rn, de, obj.Res[i])
			if expr != rn {
				pasos = append(pasos, fmt.Sprintf("convierto el resultado de %s a %s", de.Go(), obj.Res[i].Go()))
			}
			rets = append(rets, expr)
		}
		g.linea("return %s", strings.Join(rets, ", "))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "// %s adapta %s a la firma pedida.\n", obj.Nombre, llamada)
	sb.WriteString(cab.Go())
	sb.WriteString(" {\n")
	sb.WriteString(g.sb.String())
	sb.WriteString("}\n")
	return sb.String(), pasos
}

// permutaciones returns every permutation of 0..n-1, identity first (n ≤ 6).
func permutaciones(n int) [][]int {
	base := make([]int, n)
	for i := range base {
		base[i] = i
	}
	var out [][]int
	var rec func(k int)
	rec = func(k int) {
		if k == n {
			out = append(out, append([]int(nil), base...))
			return
		}
		for i := k; i < n; i++ {
			base[k], base[i] = base[i], base[k]
			rec(k + 1)
			base[k], base[i] = base[i], base[k]
		}
	}
	rec(0)
	return out
}

// renombrarFuncion renames a top-level function and every reference to it.
func renombrarFuncion(src, viejo, nuevo string) (string, error) {
	if viejo == nuevo {
		return gofmt(src), nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "web.go", src, parser.ParseComments)
	if err != nil {
		return "", err
	}
	var decl *ast.FuncDecl
	for _, d := range f.Decls {
		if x, ok := d.(*ast.FuncDecl); ok && x.Recv == nil && x.Name.Name == viejo {
			decl = x
		}
	}
	if decl == nil {
		return "", fmt.Errorf("no encuentro %s", viejo)
	}
	obj := decl.Name.Obj
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			ast.Inspect(x.X, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok && id.Name == viejo && (id.Obj == obj || id.Obj == nil) {
					id.Name = nuevo
				}
				return true
			})
			return false // never rename the selector part
		case *ast.Ident:
			if x.Name == viejo && (x.Obj == obj || x.Obj == nil) {
				x.Name = nuevo
			}
		case *ast.CommentGroup:
			return false
		}
		return true
	})
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ---- mining ----

// Minar counts package calls (strings.Fields), operators and basic literals across web code. The hints
// bias synthesis priors.
func Minar(codigos []string) nucleo.Pistas {
	p := nucleo.Pistas{Llamadas: map[string]int{}, Operadores: map[string]int{}}
	type cuentaCte struct {
		v nucleo.Valor
		n int
		i int
	}
	ctes := map[string]*cuentaCte{}
	orden := 0
	for _, src := range codigos {
		b, err := parsearBloque(src)
		if err != nil {
			continue
		}
		paquetes := map[string]bool{}
		for _, im := range importsDe(b.file) {
			paquetes[im.calificador()] = true
		}
		locales := nombresLocales(b.file)
		ast.Inspect(b.file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.GenDecl:
				if x.Tok == token.IMPORT {
					return false
				}
			case *ast.CallExpr:
				if s, ok := x.Fun.(*ast.SelectorExpr); ok {
					if id, ok := s.X.(*ast.Ident); ok && id.Obj == nil && !locales[id.Name] &&
						(paquetes[id.Name] || paquetesConocidos[id.Name] != "") {
						p.Llamadas[id.Name+"."+s.Sel.Name]++
					}
				}
			case *ast.BinaryExpr:
				p.Operadores[x.Op.String()]++
			case *ast.UnaryExpr:
				if x.Op != token.AND && x.Op != token.ARROW {
					p.Operadores["u"+x.Op.String()]++
				}
			case *ast.AssignStmt:
				if op := operadorAsignacion(x.Tok); op != "" {
					p.Operadores[op]++
				}
			case *ast.IncDecStmt:
				p.Operadores[x.Tok.String()]++
			case *ast.BasicLit:
				v, ok := valorLiteral(x)
				if ok {
					k := nucleo.Clave(v)
					if c, ok := ctes[k]; ok {
						c.n++
					} else {
						ctes[k] = &cuentaCte{v: v, n: 1, i: orden}
						orden++
					}
				}
			}
			return true
		})
	}
	lista := make([]*cuentaCte, 0, len(ctes))
	for _, c := range ctes {
		lista = append(lista, c)
	}
	sort.Slice(lista, func(i, j int) bool {
		if lista[i].n != lista[j].n {
			return lista[i].n > lista[j].n
		}
		return lista[i].i < lista[j].i
	})
	for i, c := range lista {
		if i >= 30 {
			break
		}
		p.Constantes = append(p.Constantes, c.v)
	}
	return p
}

func operadorAsignacion(t token.Token) string {
	switch t {
	case token.ADD_ASSIGN:
		return "+"
	case token.SUB_ASSIGN:
		return "-"
	case token.MUL_ASSIGN:
		return "*"
	case token.QUO_ASSIGN:
		return "/"
	case token.REM_ASSIGN:
		return "%"
	case token.AND_ASSIGN:
		return "&"
	case token.OR_ASSIGN:
		return "|"
	case token.XOR_ASSIGN:
		return "^"
	case token.SHL_ASSIGN:
		return "<<"
	case token.SHR_ASSIGN:
		return ">>"
	}
	return ""
}

func valorLiteral(x *ast.BasicLit) (nucleo.Valor, bool) {
	switch x.Kind {
	case token.INT:
		v, err := strconv.ParseInt(strings.ReplaceAll(x.Value, "_", ""), 0, 64)
		if err != nil {
			return nil, false
		}
		return int(v), true
	case token.FLOAT:
		v, err := strconv.ParseFloat(strings.ReplaceAll(x.Value, "_", ""), 64)
		if err != nil {
			return nil, false
		}
		return v, true
	case token.STRING:
		s, err := strconv.Unquote(x.Value)
		if err != nil || len(s) > 40 {
			return nil, false
		}
		return s, true
	case token.CHAR:
		s, err := strconv.Unquote(x.Value)
		if err != nil {
			return nil, false
		}
		for _, r := range s {
			if unicode.IsPrint(r) || r == '\n' || r == '\t' {
				return int(r), true
			}
		}
	}
	return nil, false
}
