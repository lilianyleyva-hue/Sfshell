package reparar

import (
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// archivoFuente is the file name used in positions (the sandbox builds a single file).
const archivoFuente = "x.go"

// ---- cached importer ----

var (
	muTipos      sync.Mutex // serializes go/types runs: imported packages are shared
	impGc        types.Importer
	impFuente    types.Importer
	fsetImport   = token.NewFileSet()
	cacheImport  = map[string]*types.Package{}
	fallosImport = map[string]error{}
)

type importadorCache struct{}

// Import uses export data (go list -export) first and GOROOT sources as a fallback; results and
// failures are cached for the life of the process. Callers hold muTipos.
func (importadorCache) Import(ruta string) (*types.Package, error) {
	if p, ok := cacheImport[ruta]; ok {
		return p, nil
	}
	if err, ok := fallosImport[ruta]; ok {
		return nil, err
	}
	if impGc == nil {
		impGc = importer.Default()
		impFuente = importer.ForCompiler(fsetImport, "source", nil)
	}
	p, err := impGc.Import(ruta)
	if err != nil || p == nil {
		p, err = impFuente.Import(ruta)
	}
	if err != nil {
		fallosImport[ruta] = err
		return nil, err
	}
	cacheImport[ruta] = p
	return p, nil
}

// ---- analysis ----

// analisis is one parse + type-check of a file, kept for the rules that need types and scopes.
type analisis struct {
	fuente   string
	fset     *token.FileSet
	archivo  *ast.File
	info     *types.Info
	paquete  *types.Package
	errs     []nucleo.ErrorGo
	sintaxis bool // the file does not parse
}

// analizar parses and type-checks fuente in-process.
func analizar(fuente string) *analisis {
	a := &analisis{fuente: fuente, fset: token.NewFileSet()}
	f, err := parser.ParseFile(a.fset, archivoFuente, fuente, parser.AllErrors|parser.ParseComments)
	if err != nil {
		a.sintaxis = true
		a.archivo = f
		var lista scanner.ErrorList
		if errors.As(err, &lista) {
			for _, e := range lista {
				a.errs = append(a.errs, nucleo.ErrorGo{Archivo: archivoFuente, Linea: e.Pos.Line, Col: e.Pos.Column, Msg: e.Msg})
			}
		} else {
			a.errs = []nucleo.ErrorGo{{Archivo: archivoFuente, Linea: 1, Col: 1, Msg: err.Error()}}
		}
		return a
	}
	a.archivo = f
	a.info = &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Scopes:     map[ast.Node]*types.Scope{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	conf := types.Config{
		Importer: importadorCache{},
		Error: func(err error) {
			if te, ok := err.(types.Error); ok {
				p := te.Fset.Position(te.Pos)
				a.errs = append(a.errs, nucleo.ErrorGo{Archivo: archivoFuente, Linea: p.Line, Col: p.Column, Msg: te.Msg})
				return
			}
			a.errs = append(a.errs, nucleo.ErrorGo{Archivo: archivoFuente, Linea: 1, Col: 1, Msg: err.Error()})
		},
	}
	muTipos.Lock()
	a.paquete, _ = conf.Check(f.Name.Name, a.fset, []*ast.File{f}, a.info)
	muTipos.Unlock()
	if f.Name.Name == "main" && len(a.errs) == 0 {
		tieneMain := false
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "main" {
				tieneMain = true
			}
		}
		if !tieneMain {
			a.errs = append(a.errs, nucleo.ErrorGo{Archivo: archivoFuente, Linea: 1, Col: 1, Msg: "function main is undeclared in the main package"})
		}
	}
	sort.SliceStable(a.errs, func(i, j int) bool {
		if a.errs[i].Linea != a.errs[j].Linea {
			return a.errs[i].Linea < a.errs[j].Linea
		}
		return a.errs[i].Col < a.errs[j].Col
	})
	return a
}

// ChequeoTipos parses and type-checks one file in-process with go/types and a cached importer, and
// returns every error in compiler style. When the file does not parse, it returns the syntax errors and
// ErrSintaxis.
func ChequeoTipos(fuente string) ([]nucleo.ErrorGo, error) {
	a := analizar(fuente)
	if a.sintaxis {
		return a.errs, ErrSintaxis
	}
	return a.errs, nil
}

// ---- positions ----

// offset converts a 1-based line and byte column into a byte offset of src (clamped).
func offset(src string, linea, col int) int {
	if linea < 1 {
		return 0
	}
	off := 0
	for l := 1; l < linea; l++ {
		i := strings.IndexByte(src[off:], '\n')
		if i < 0 {
			return len(src)
		}
		off += i + 1
	}
	if col < 1 {
		col = 1
	}
	off += col - 1
	if off > len(src) {
		off = len(src)
	}
	return off
}

func (a *analisis) off(p token.Pos) int {
	if !p.IsValid() {
		return -1
	}
	return a.fset.Position(p).Offset
}

func (a *analisis) linea(p token.Pos) int { return a.fset.Position(p).Line }

func (a *analisis) texto(n ast.Node) string {
	i, j := a.off(n.Pos()), a.off(n.End())
	if i < 0 || j < i || j > len(a.fuente) {
		return ""
	}
	return a.fuente[i:j]
}

// posDe returns the token.Pos of an ErrorGo position.
func (a *analisis) posDe(e nucleo.ErrorGo) token.Pos {
	tf := a.fset.File(a.archivo.Pos())
	if tf == nil {
		return token.NoPos
	}
	off := offset(a.fuente, e.Linea, e.Col)
	if off > tf.Size() {
		off = tf.Size()
	}
	return tf.Pos(off)
}

// camino returns the chain of nodes from the file down to the innermost node containing p.
func (a *analisis) camino(p token.Pos) []ast.Node {
	var out []ast.Node
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n.Pos() <= p && p < n.End() {
			out = append(out, n)
			return true
		}
		return false
	})
	return out
}

// funcionEn returns the innermost function (declaration or literal) containing p, with its type.
func (a *analisis) funcionEn(p token.Pos) (*ast.FuncType, *ast.BlockStmt) {
	cam := a.camino(p)
	for i := len(cam) - 1; i >= 0; i-- {
		switch f := cam[i].(type) {
		case *ast.FuncDecl:
			return f.Type, f.Body
		case *ast.FuncLit:
			return f.Type, f.Body
		}
	}
	return nil, nil
}

// sentenciaEn returns the innermost statement containing p that sits directly in a block (with the block).
func (a *analisis) sentenciaEn(p token.Pos) (ast.Stmt, *ast.BlockStmt) {
	cam := a.camino(p)
	for i := len(cam) - 1; i > 0; i-- {
		s, ok := cam[i].(ast.Stmt)
		if !ok {
			continue
		}
		switch b := cam[i-1].(type) {
		case *ast.BlockStmt:
			return s, b
		case *ast.CaseClause, *ast.CommClause:
			return s, nil
		}
	}
	return nil, nil
}

// exprEn returns the expression starting exactly at p whose source text equals texto (ignoring spaces),
// or, failing that, the largest expression starting at p.
func (a *analisis) exprEn(p token.Pos, texto string) ast.Expr {
	var mejor, mayor ast.Expr
	quitar := func(s string) string { return strings.Join(strings.Fields(s), "") }
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if n == nil || n.Pos() > p || n.End() <= p {
			return n != nil && n.Pos() <= p && p < n.End()
		}
		if e, ok := n.(ast.Expr); ok && e.Pos() == p {
			if mejor == nil && texto != "" && (quitar(a.texto(e)) == quitar(texto) || quitar(types.ExprString(e)) == quitar(texto)) {
				mejor = e
			}
			if mayor == nil || e.End() > mayor.End() {
				mayor = e
			}
		}
		return true
	})
	if mejor != nil {
		return mejor
	}
	return mayor
}

// tipoDe returns the type of an expression, or nil.
func (a *analisis) tipoDe(e ast.Expr) types.Type {
	if a.info == nil {
		return nil
	}
	if tv, ok := a.info.Types[e]; ok {
		return tv.Type
	}
	if id, ok := e.(*ast.Ident); ok {
		if o := a.info.Uses[id]; o != nil {
			return o.Type()
		}
		if o := a.info.Defs[id]; o != nil {
			return o.Type()
		}
	}
	return nil
}

// calificador writes types relative to the file's own package.
func (a *analisis) calificador() types.Qualifier {
	return func(p *types.Package) string {
		if a.paquete != nil && p == a.paquete {
			return ""
		}
		return p.Name()
	}
}

// ceroDe writes the zero value of t as Go source.
func (a *analisis) ceroDe(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return conversionSiNombre(a, t, "false")
		case u.Info()&types.IsString != 0:
			return conversionSiNombre(a, t, `""`)
		case u.Info()&types.IsNumeric != 0:
			return conversionSiNombre(a, t, "0")
		case u.Kind() == types.UnsafePointer:
			return "nil"
		}
		return "nil"
	case *types.Struct, *types.Array:
		return types.TypeString(t, a.calificador()) + "{}"
	}
	return "nil"
}

func conversionSiNombre(a *analisis, t types.Type, lit string) string {
	if _, ok := t.(*types.Named); ok {
		return types.TypeString(t, a.calificador()) + "(" + lit + ")"
	}
	return lit
}

// resultados returns the result types of a function type (expanded: "a, b int" gives two).
func (a *analisis) resultados(ft *ast.FuncType) []types.Type {
	if ft == nil || ft.Results == nil {
		return nil
	}
	var out []types.Type
	for _, campo := range ft.Results.List {
		t := a.tipoDe(campo.Type)
		n := len(campo.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, t)
		}
	}
	return out
}

// cerosDe writes the zero values of the results of ft, "" when they are unknown.
func (a *analisis) cerosDe(ft *ast.FuncType) ([]string, bool) {
	rs := a.resultados(ft)
	out := make([]string, len(rs))
	for i, t := range rs {
		if t == nil {
			return nil, false
		}
		out[i] = a.ceroDe(t)
	}
	return out, true
}

// esError reports whether t is the predeclared error type.
func esError(t types.Type) bool {
	return t != nil && types.Identical(t, types.Universe.Lookup("error").Type())
}

// indentacion returns the leading whitespace of the line containing offset off.
func indentacion(src string, off int) string {
	ini := strings.LastIndexByte(src[:off], '\n') + 1
	fin := ini
	for fin < len(src) && (src[fin] == ' ' || src[fin] == '\t') {
		fin++
	}
	return src[ini:fin]
}

// inicioLinea returns the offset of the start of the line containing off.
func inicioLinea(src string, off int) int { return strings.LastIndexByte(src[:off], '\n') + 1 }

// finLinea returns the offset of the '\n' ending the line containing off (or len(src)).
func finLinea(src string, off int) int {
	i := strings.IndexByte(src[off:], '\n')
	if i < 0 {
		return len(src)
	}
	return off + i
}

// lineaDe returns the 1-based line of offset off.
func lineaDe(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return strings.Count(src[:off], "\n") + 1
}

// textoLinea returns the trimmed text of line n (1-based).
func textoLinea(src string, n int) string {
	lineas := strings.Split(src, "\n")
	if n < 1 || n > len(lineas) {
		return ""
	}
	return strings.TrimSpace(lineas[n-1])
}
