// Package analisis does the static and dynamic analysis of Go code for Nyx Código (§4.5): signatures
// (nucleo.Tipo trees built from go/types), normalization of snippets, findings, complexity, Spanish
// narration, variable roles, generated comments, verified behaviour summaries, variable traces and
// measured growth. It never builds anything itself: dynamic work goes through a nucleo.Binario.
package analisis

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Hallazgo is one static finding, in simple Spanish.
type Hallazgo struct {
	Linea    int    `json:"linea"`
	Clave    string `json:"clave"`    // "mapa_nil", "error_ignorado", "defer_en_bucle", "float_igual", "concat_en_bucle", "div_sin_comprobar", "indice_mas_uno", "bucle_sin_salida", "codigo_inalcanzable", "err_sombreado", "rango_copia_grande", "variable_bucle_capturada"
	Gravedad string `json:"gravedad"` // "error" | "aviso" | "consejo"
	Mensaje  string `json:"mensaje"`  // Spanish
	Consejo  string `json:"consejo"`  // Spanish, with a code hint
}

// InfoFunc describes one function or method of the analysed file.
type InfoFunc struct {
	Nombre      string        // "SumaPares", or "Punto.Distancia" for a method
	Firma       *nucleo.Firma // nil if not representable
	NoProbable  string        // Spanish reason when Firma == nil or !Probable(): "usa canales"
	Desde       int
	Hasta       int
	Ciclomatica int
	Anidamiento int
	Recursiva   bool
	Exponencial bool   // ≥ 2 self-calls on a path without memoization
	OGrande     string // "O(1)", "O(log n)", "O(√n)", "O(n)", "O(n log n)", "O(n²)", "O(n³)", "O(2ⁿ)?"
	Motivo      string // Spanish: "dos bucles anidados que recorren la entrada"
}

// Informe is the result of Analizar.
type Informe struct {
	Paquete   string
	Funciones []InfoFunc
	Hallazgos []Hallazgo
	Imports   []string
	TieneMain bool
	UsaGo     bool             // goroutines
	Errores   []nucleo.ErrorGo // parse/type errors (analysis continues with partial info)
}

// ErrVacio is returned when there is no Go code to analyse.
var ErrVacio = errors.New("analisis: no hay código")

// ---- parsing that keeps the user's line numbers ----

// archivo is a parsed (and, when possible, type-checked) source file.
type archivo struct {
	fset     *token.FileSet
	f        *ast.File
	info     *types.Info
	pkg      *types.Package
	errores  []nucleo.ErrorGo
	envuelto bool   // bare statements were wrapped in func main() (same lines)
	prefijo  int    // bytes added before the user's code on line 1
	texto    string // the text that was parsed (with the added prefix and suffix)
}

const (
	prefijoPaquete = "package main;"
	prefijoMain    = "package main; func main() {"
)

// tienePaquete reports whether the first token of src is the keyword package.
func tienePaquete(src string) bool {
	var s scanner.Scanner
	fset := token.NewFileSet()
	fl := fset.AddFile("x.go", -1, len(src))
	s.Init(fl, []byte(src), nil, scanner.ScanComments)
	for {
		_, tok, _ := s.Scan()
		switch tok {
		case token.COMMENT:
			continue
		case token.PACKAGE:
			return true
		default:
			return false
		}
	}
}

func erroresDeParse(err error, prefijo int) []nucleo.ErrorGo {
	if err == nil {
		return nil
	}
	var lista scanner.ErrorList
	if errors.As(err, &lista) {
		out := make([]nucleo.ErrorGo, 0, len(lista))
		for _, e := range lista {
			col := e.Pos.Column
			if e.Pos.Line == 1 && col > prefijo {
				col -= prefijo
			}
			out = append(out, nucleo.ErrorGo{Archivo: "x.go", Linea: e.Pos.Line, Col: col, Msg: e.Msg})
		}
		return out
	}
	return []nucleo.ErrorGo{{Archivo: "x.go", Linea: 1, Col: 1, Msg: err.Error()}}
}

// parsear reads src as a file. A snippet without a package clause is read as "package main;" + src, and
// bare statements as "package main; func main() {" + src + "\n}", so line numbers stay the user's.
func parsear(src string) (*archivo, error) {
	if strings.TrimSpace(src) == "" {
		return nil, ErrVacio
	}
	modo := parser.ParseComments | parser.AllErrors
	a := &archivo{fset: token.NewFileSet()}
	if tienePaquete(src) {
		f, err := parser.ParseFile(a.fset, "x.go", src, modo)
		if f == nil {
			return nil, err
		}
		a.f, a.errores, a.texto = f, erroresDeParse(err, 0), src
		return a, nil
	}
	f, err := parser.ParseFile(a.fset, "x.go", prefijoPaquete+src, modo)
	if err == nil {
		a.f, a.prefijo, a.texto = f, len(prefijoPaquete), prefijoPaquete+src
		return a, nil
	}
	fset2 := token.NewFileSet()
	f2, err2 := parser.ParseFile(fset2, "x.go", prefijoMain+src+"\n}", modo)
	if err2 == nil {
		a.fset, a.f, a.prefijo, a.envuelto, a.texto = fset2, f2, len(prefijoMain), true, prefijoMain+src+"\n}"
		return a, nil
	}
	if f == nil {
		return nil, err
	}
	a.f, a.prefijo, a.errores, a.texto = f, len(prefijoPaquete), erroresDeParse(err, len(prefijoPaquete)), prefijoPaquete+src
	return a, nil
}

// ---- type checking ----

// The importer only reads the standard library: user or web code can name any import path, and
// resolving a module path, a relative path or an absolute path would run `go list` on it or
// type-check an arbitrary directory (with cgo, that means running the C compiler). Such paths fail
// at once with a Spanish error and the analysis goes on with partial information.

var (
	muImportador sync.Mutex
	importadorGc types.Importer
	fuentesStd   *importadorFuentes
	cachePaq     = map[string]*types.Package{}
	fallosPaq    = map[string]error{}
	rutasStd     = map[string]bool{} // validated paths (true: a standard library package)
)

type importador struct{}

// Import reads a standard library package from export data and falls back to type-checking its
// GOROOT sources in-process (with cgo off). Anything else is refused. Results, and failures, are cached
// for the life of the process.
func (importador) Import(ruta string) (*types.Package, error) {
	muImportador.Lock()
	defer muImportador.Unlock()
	return importarStd(ruta)
}

// importarStd does the work of Import; muImportador must be held.
func importarStd(ruta string) (*types.Package, error) {
	if ruta == "unsafe" {
		return types.Unsafe, nil
	}
	if p, ok := cachePaq[ruta]; ok {
		return p, nil
	}
	if err, ok := fallosPaq[ruta]; ok {
		return nil, err
	}
	if _, ok := dirEstandar(ruta, false); !ok {
		err := fmt.Errorf("solo leo paquetes de la biblioteca estándar, y %q no lo es", ruta)
		fallosPaq[ruta] = err
		return nil, err
	}
	if importadorGc == nil {
		importadorGc = importer.Default()
	}
	p, err := importadorGc.Import(ruta)
	if err != nil {
		if fuentesStd == nil {
			fuentesStd = nuevoImportadorFuentes()
		}
		p, err = fuentesStd.Import(ruta)
	}
	if err != nil {
		fallosPaq[ruta] = err
		return nil, err
	}
	cachePaq[ruta] = p
	return p, nil
}

// dirEstandar returns the GOROOT/src directory of a standard library package path. It refuses empty,
// relative and absolute paths, paths with "." or ".." elements, module paths (a dot in the first
// element), the cmd tree, vendor and testdata directories, and (unless internos) internal packages.
// muImportador must be held.
func dirEstandar(ruta string, internos bool) (string, bool) {
	clave := ruta
	if internos {
		clave = "+" + ruta
	}
	if ok, ya := rutasStd[clave]; ya {
		if !ok {
			return "", false
		}
		return filepath.Join(build.Default.GOROOT, "src", filepath.FromSlash(ruta)), true
	}
	dir, ok := validarRutaStd(ruta, internos)
	rutasStd[clave] = ok
	return dir, ok
}

func validarRutaStd(ruta string, internos bool) (string, bool) {
	raiz := build.Default.GOROOT
	if raiz == "" || ruta == "" || ruta == "C" || build.IsLocalImport(ruta) || strings.HasPrefix(ruta, "/") ||
		filepath.IsAbs(ruta) || strings.ContainsAny(ruta, "\\:@ \t\n\"'`") || !utf8.ValidString(ruta) {
		return "", false
	}
	partes := strings.Split(ruta, "/")
	if strings.Contains(partes[0], ".") || partes[0] == "cmd" {
		return "", false
	}
	for _, p := range partes {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasPrefix(p, "_") || p == "testdata" || p == "vendor" {
			return "", false
		}
		if p == "internal" && !internos {
			return "", false
		}
	}
	base := filepath.Join(raiz, "src")
	dir := filepath.Join(base, filepath.FromSlash(ruta))
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", false
	}
	if realBase, err := filepath.EvalSymlinks(base); err == nil {
		if rel, err := filepath.Rel(realBase, real); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return "", false
		}
	}
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entradas {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			return dir, true
		}
	}
	return "", false
}

// importadorFuentes type-checks standard library packages from GOROOT/src in-process, with cgo off
// (the pure Go files are used) and without function bodies. It never starts a process.
type importadorFuentes struct {
	fset     *token.FileSet
	ctx      build.Context
	paquetes map[string]*types.Package
	fallos   map[string]error
	enCurso  map[string]bool
}

func nuevoImportadorFuentes() *importadorFuentes {
	ctx := build.Default
	ctx.CgoEnabled = false
	return &importadorFuentes{fset: token.NewFileSet(), ctx: ctx, paquetes: map[string]*types.Package{},
		fallos: map[string]error{}, enCurso: map[string]bool{}}
}

// Import is called with muImportador held (directly or by go/types for the dependencies).
func (f *importadorFuentes) Import(ruta string) (*types.Package, error) {
	if ruta == "unsafe" {
		return types.Unsafe, nil
	}
	if p, ok := f.paquetes[ruta]; ok {
		return p, nil
	}
	if err, ok := f.fallos[ruta]; ok {
		return nil, err
	}
	if f.enCurso[ruta] {
		return nil, fmt.Errorf("el paquete %s se importa a sí mismo", ruta)
	}
	f.enCurso[ruta] = true
	defer delete(f.enCurso, ruta)
	p, err := f.cargar(ruta)
	if err != nil {
		f.fallos[ruta] = err
		return nil, err
	}
	f.paquetes[ruta] = p
	return p, nil
}

func (f *importadorFuentes) cargar(ruta string) (*types.Package, error) {
	dir, ok := dirEstandar(ruta, true)
	if !ok {
		return nil, fmt.Errorf("%q no es un paquete de la biblioteca estándar", ruta)
	}
	bp, err := f.ctx.ImportDir(dir, 0)
	if err != nil {
		return nil, fmt.Errorf("no pude leer el paquete %s: %v", ruta, err)
	}
	var archivos []*ast.File
	for _, n := range bp.GoFiles {
		af, err := parser.ParseFile(f.fset, filepath.Join(dir, n), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("no pude leer el paquete %s: %v", ruta, err)
		}
		archivos = append(archivos, af)
	}
	conf := types.Config{
		Importer:         f,
		IgnoreFuncBodies: true,
		FakeImportC:      true,
		Sizes:            types.SizesFor("gc", build.Default.GOARCH),
		Error:            func(error) {},
	}
	p, err := conf.Check(ruta, f.fset, archivos, nil)
	if p == nil {
		return nil, fmt.Errorf("no pude comprobar el paquete %s: %v", ruta, err)
	}
	return p, nil
}

// esImportado reports whether p is a package produced by the importer (the standard library), as
// opposed to a package type-checked from the user's code.
func esImportado(p *types.Package) bool {
	muImportador.Lock()
	defer muImportador.Unlock()
	if q, ok := cachePaq[p.Path()]; ok && q == p {
		return true
	}
	if fuentesStd != nil {
		if q, ok := fuentesStd.paquetes[p.Path()]; ok && q == p {
			return true
		}
	}
	return false
}

// comprobarTipos fills a.info; type errors are recorded, never fatal.
func (a *archivo) comprobarTipos() {
	a.info = &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Scopes:     map[ast.Node]*types.Scope{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	conf := types.Config{
		Importer:    importador{},
		FakeImportC: true,
		Error: func(err error) {
			if te, ok := err.(types.Error); ok {
				pos := te.Fset.Position(te.Pos)
				col := pos.Column
				if pos.Line == 1 && col > a.prefijo {
					col -= a.prefijo
				}
				a.errores = append(a.errores, nucleo.ErrorGo{Archivo: "x.go", Linea: pos.Line, Col: col, Msg: te.Msg})
				return
			}
			a.errores = append(a.errores, nucleo.ErrorGo{Archivo: "x.go", Linea: 1, Col: 1, Msg: err.Error()})
		},
	}
	nombre := "main"
	if a.f.Name != nil {
		nombre = a.f.Name.Name
	}
	a.pkg, _ = conf.Check(nombre, a.fset, []*ast.File{a.f}, a.info)
}

func (a *archivo) linea(p token.Pos) int {
	if !p.IsValid() {
		return 0
	}
	return a.fset.Position(p).Line
}

// tipo returns the type of e, or nil when unknown.
func (a *archivo) tipo(e ast.Expr) types.Type {
	if a.info == nil || e == nil {
		return nil
	}
	if tv, ok := a.info.Types[e]; ok && tv.Type != nil {
		if b, ok := tv.Type.(*types.Basic); ok && b.Kind() == types.Invalid {
			return nil
		}
		return tv.Type
	}
	if id, ok := e.(*ast.Ident); ok {
		if o := a.objeto(id); o != nil {
			return o.Type()
		}
	}
	return nil
}

// objeto returns the object an identifier defines or uses.
func (a *archivo) objeto(id *ast.Ident) types.Object {
	if a.info == nil || id == nil {
		return nil
	}
	if o := a.info.Defs[id]; o != nil {
		return o
	}
	return a.info.Uses[id]
}

// ---- go/types → nucleo.Tipo ----

// TipoDe converts a go/types type into a nucleo.Tipo. It reports false for channels, functions,
// interfaces other than error, generics, complex numbers, types from other packages and recursive types.
// Without the package being analysed at hand, a named type is "from another package" when it comes from
// the standard library importer, or when its package path is a standard library package that declares
// a type with that name.
func TipoDe(t types.Type) (nucleo.Tipo, bool) {
	r, motivo := (&conversor{visto: map[*types.Named]bool{}}).tipo(t)
	return r, motivo == ""
}

// conversor turns go/types types into nucleo.Tipo trees.
type conversor struct {
	visto map[*types.Named]bool
	local *types.Package // the package being analysed (nil when unknown)
}

// tipoNucleo converts a type of the analysed file: named types are local exactly when they belong to
// the checked package.
func (a *archivo) tipoNucleo(t types.Type) (nucleo.Tipo, string) {
	return (&conversor{visto: map[*types.Named]bool{}, local: a.pkg}).tipo(t)
}

// tipoDe is the context-free conversion (as TipoDe), with the Spanish reason.
func tipoDe(t types.Type, visto map[*types.Named]bool) (nucleo.Tipo, string) {
	return (&conversor{visto: visto}).tipo(t)
}

// foraneo reports whether a named type is declared in a package other than the analysed one: the
// harness could not declare it in the candidate's package.
func (c *conversor) foraneo(obj *types.TypeName) bool {
	p := obj.Pkg()
	if p == nil {
		return false
	}
	if c.local != nil {
		return p != c.local
	}
	return esDeOtroPaquete(obj)
}

// tipo returns the type, or a Spanish reason why it cannot be represented.
func (c *conversor) tipo(t types.Type) (nucleo.Tipo, string) {
	visto := c.visto
	if t == nil {
		return nucleo.Tipo{}, "no conozco el tipo"
	}
	t = types.Unalias(t)
	switch x := t.(type) {
	case *types.Basic:
		return tipoBasico(x)
	case *types.Named:
		obj := x.Obj()
		if obj.Pkg() == nil && obj.Name() == "error" {
			return nucleo.TError, ""
		}
		if x.TypeArgs().Len() > 0 || x.TypeParams().Len() > 0 {
			return nucleo.Tipo{}, "es genérica"
		}
		if obj.Pkg() == nil {
			return nucleo.Tipo{}, "usa el tipo " + obj.Name()
		}
		if c.foraneo(obj) {
			return nucleo.Tipo{}, "usa el tipo " + obj.Pkg().Name() + "." + obj.Name() + " de otro paquete"
		}
		if visto[x] {
			return nucleo.Tipo{}, "usa un tipo recursivo (" + obj.Name() + ")"
		}
		visto[x] = true
		defer delete(visto, x)
		u, motivo := c.tipo(x.Underlying())
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		if u.Clase == nucleo.CError {
			return nucleo.Tipo{}, "usa el tipo " + obj.Name() + ", que es una interfaz"
		}
		u.Nombre, u.Definido = obj.Name(), true
		return u, ""
	case *types.Pointer:
		e, motivo := c.tipo(x.Elem())
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		if e.Clase != nucleo.CStruct {
			return nucleo.Tipo{}, "usa punteros a algo que no es una estructura"
		}
		return nucleo.PunteroA(e), ""
	case *types.Slice:
		e, motivo := c.tipo(x.Elem())
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		return nucleo.ListaDe(e), ""
	case *types.Array:
		e, motivo := c.tipo(x.Elem())
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		return nucleo.ArregloDe(int(x.Len()), e), ""
	case *types.Map:
		k, motivo := c.tipo(x.Key())
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		switch k.Clase {
		case nucleo.CInt, nucleo.CString, nucleo.CRune, nucleo.CByte, nucleo.CBool:
		default:
			return nucleo.Tipo{}, "usa un mapa con claves de tipo " + k.Go()
		}
		v, motivo := c.tipo(x.Elem())
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		return nucleo.MapaDe(k, v), ""
	case *types.Struct:
		out := nucleo.Tipo{Clase: nucleo.CStruct}
		for i := 0; i < x.NumFields(); i++ {
			campo := x.Field(i)
			ct, motivo := c.tipo(campo.Type())
			if motivo != "" {
				return nucleo.Tipo{}, motivo
			}
			if ct.Clase == nucleo.CError {
				return nucleo.Tipo{}, "tiene un campo de tipo error"
			}
			out.Campos = append(out.Campos, nucleo.Campo{Nombre: campo.Name(), Tipo: ct})
		}
		return out, ""
	case *types.Chan:
		return nucleo.Tipo{}, "usa canales"
	case *types.Signature:
		return nucleo.Tipo{}, "usa funciones como valores"
	case *types.Interface:
		if x.NumMethods() == 0 {
			return nucleo.Tipo{}, "usa any (interface{})"
		}
		return nucleo.Tipo{}, "usa interfaces"
	case *types.TypeParam:
		return nucleo.Tipo{}, "es genérica"
	case *types.Tuple:
		return nucleo.Tipo{}, "es una lista de resultados"
	}
	return nucleo.Tipo{}, "usa un tipo que no sé representar (" + t.String() + ")"
}

// esDeOtroPaquete is the context-free test used by TipoDe: the type comes from the standard library
// importer, or its package path is a standard library package that declares a type of that name (so
// big.Int or time.Duration are foreign, while a user package called image with its own Punto is not).
func esDeOtroPaquete(obj *types.TypeName) bool {
	p := obj.Pkg()
	if p == nil || p.Path() == "main" {
		return false
	}
	if esImportado(p) {
		return true
	}
	muImportador.Lock()
	_, std := dirEstandar(p.Path(), false)
	muImportador.Unlock()
	if !std {
		return false
	}
	q, err := importador{}.Import(p.Path())
	if err != nil || q == nil {
		return true // a standard library path we cannot read: assume it is not the user's
	}
	_, ok := q.Scope().Lookup(obj.Name()).(*types.TypeName)
	return ok
}

func tipoBasico(b *types.Basic) (nucleo.Tipo, string) {
	switch b.Kind() {
	case types.Bool, types.UntypedBool:
		return nucleo.TBool, ""
	case types.String, types.UntypedString:
		return nucleo.TString, ""
	case types.Float32:
		return nucleo.Tipo{Clase: nucleo.CFloat, Nombre: "float32"}, ""
	case types.Float64, types.UntypedFloat:
		return nucleo.TFloat, ""
	case types.UntypedInt:
		return nucleo.TInt, ""
	case types.UntypedRune:
		return nucleo.TRune, ""
	case types.Complex64, types.Complex128, types.UntypedComplex:
		return nucleo.Tipo{}, "usa números complejos"
	case types.UnsafePointer:
		return nucleo.Tipo{}, "usa unsafe.Pointer"
	case types.UntypedNil:
		return nucleo.Tipo{}, "es nil sin tipo"
	case types.Invalid:
		return nucleo.Tipo{}, "no pude entender los tipos (el código tiene errores)"
	}
	if b.Info()&types.IsInteger != 0 {
		switch b.Name() {
		case "byte":
			return nucleo.TByte, ""
		case "rune":
			return nucleo.TRune, ""
		}
		return nucleo.Tipo{Clase: nucleo.CInt, Nombre: b.Name()}, ""
	}
	return nucleo.Tipo{}, "usa el tipo " + b.Name()
}

// ---- signatures ----

// firmaDe builds the signature of a declared function from go/types. It returns nil and a reason when
// some part cannot be represented.
func (a *archivo) firmaDe(fd *ast.FuncDecl) (*nucleo.Firma, string) {
	if a.info == nil {
		return nil, "no pude comprobar los tipos"
	}
	fn, ok := a.info.Defs[fd.Name].(*types.Func)
	if !ok || fn == nil {
		return nil, "no pude comprobar los tipos"
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil, "no pude comprobar los tipos"
	}
	if sig.TypeParams().Len() > 0 || sig.RecvTypeParams().Len() > 0 {
		return nil, "es genérica"
	}
	f := &nucleo.Firma{Nombre: fd.Name.Name, Variadica: sig.Variadic()}
	if r := sig.Recv(); r != nil {
		t, motivo := a.tipoNucleo(r.Type())
		if motivo != "" {
			return nil, "es un método de un tipo que no sé representar: " + motivo
		}
		f.Receptor = &nucleo.Param{Nombre: nombreParam(r.Name()), Tipo: t}
	}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		t, motivo := a.tipoNucleo(p.Type())
		if motivo != "" {
			return nil, motivo
		}
		f.Params = append(f.Params, nucleo.Param{Nombre: nombreParam(p.Name()), Tipo: t})
	}
	for i := 0; i < sig.Results().Len(); i++ {
		t, motivo := a.tipoNucleo(sig.Results().At(i).Type())
		if motivo != "" {
			return nil, motivo
		}
		f.Res = append(f.Res, t)
	}
	return f, ""
}

func nombreParam(n string) string {
	if n == "_" {
		return ""
	}
	return n
}

// motivoNoProbable explains in Spanish why a representable signature is not Probable ("" if it is).
func motivoNoProbable(f nucleo.Firma) string {
	if f.Probable() {
		return ""
	}
	ent := f.Entradas()
	switch {
	case len(f.Res) == 0:
		return "no devuelve nada"
	case len(f.Res) > 3:
		return "devuelve más de 3 resultados"
	case len(ent) > 6:
		return "tiene más de 6 entradas"
	}
	if f.Receptor != nil && f.Receptor.Tipo.Clase != nucleo.CStruct && f.Receptor.Tipo.Clase != nucleo.CPuntero {
		return "es un método de un tipo que no es una estructura"
	}
	for _, t := range ent {
		if t.Clase == nucleo.CError {
			return "recibe un error como entrada"
		}
		if !t.Codificable() {
			return "usa el tipo " + t.Go() + ", que no sé enviar a la prueba"
		}
	}
	for i, t := range f.Res {
		if t.Clase == nucleo.CError && i != len(f.Res)-1 {
			return "devuelve un error que no es el último resultado"
		}
		if !t.Codificable() {
			return "devuelve el tipo " + t.Go() + ", que no sé leer de la prueba"
		}
	}
	return "no se puede probar automáticamente"
}

// Firmas returns the testable signatures of the file, in source order (main and init are skipped).
func Firmas(fuente string) ([]nucleo.Firma, error) {
	a, err := parsear(fuente)
	if err != nil {
		return nil, err
	}
	a.comprobarTipos()
	var out []nucleo.Firma
	for _, d := range a.f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name == nil || (fd.Recv == nil && (fd.Name.Name == "main" || fd.Name.Name == "init")) {
			continue
		}
		f, _ := a.firmaDe(fd)
		if f != nil && f.Probable() {
			out = append(out, *f)
		}
	}
	return out, nil
}

// nombreFunc is "Nombre" for functions and "Tipo.Nombre" for methods.
func nombreFunc(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	return nombreReceptor(fd.Recv.List[0].Type) + "." + fd.Name.Name
}

func nombreReceptor(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return nombreReceptor(x.X)
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return nombreReceptor(x.X)
	case *ast.IndexListExpr:
		return nombreReceptor(x.X)
	case *ast.ParenExpr:
		return nombreReceptor(x.X)
	}
	return "?"
}

// buscarFunc finds a function by "Nombre", "Tipo.Nombre" or "(*Tipo).Nombre". With nombre == "" it
// returns the first function that is not main/init, or main.
func buscarFunc(f *ast.File, nombre string) *ast.FuncDecl {
	nombre = strings.TrimSpace(nombre)
	nombre = strings.NewReplacer("(*", "", "(", "", ")", "", "*", "").Replace(nombre)
	var primera, main *ast.FuncDecl
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name == nil {
			continue
		}
		if nombre == "" {
			if fd.Recv == nil && fd.Name.Name == "main" {
				main = fd
			} else if primera == nil && !(fd.Recv == nil && fd.Name.Name == "init") {
				primera = fd
			}
			continue
		}
		if nombreFunc(fd) == nombre {
			return fd
		}
	}
	if nombre == "" {
		if primera != nil {
			return primera
		}
		return main
	}
	for _, d := range f.Decls { // a bare method name
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name != nil && fd.Name.Name == nombre {
			return fd
		}
	}
	return nil
}

// ---- Analizar ----

// Analizar parses and type-checks the file and reports functions, complexity and findings.
// Parse and type errors go to Informe.Errores; the analysis continues with partial information.
func Analizar(fuente string) (Informe, error) {
	a, err := parsear(fuente)
	if err != nil {
		return Informe{}, err
	}
	a.comprobarTipos()
	inf := Informe{Paquete: a.f.Name.Name, Errores: a.errores}
	for _, im := range a.f.Imports {
		if ruta, err := strconv.Unquote(im.Path.Value); err == nil {
			inf.Imports = append(inf.Imports, ruta)
		}
	}
	ast.Inspect(a.f, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			inf.UsaGo = true
		}
		return true
	})
	comp := nuevaComplejidad(a)
	for _, d := range a.f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name == nil {
			continue
		}
		if fd.Recv == nil && fd.Name.Name == "main" && inf.Paquete == "main" {
			inf.TieneMain = true
		}
		info := InfoFunc{Nombre: nombreFunc(fd), Desde: a.linea(fd.Pos()), Hasta: a.linea(fd.End())}
		if a.envuelto && fd.Name.Name == "main" && fd.Recv == nil {
			info.Hasta-- // the closing brace we added
		}
		f, motivo := a.firmaDe(fd)
		if f != nil {
			info.Firma = f
			motivo = motivoNoProbable(*f)
		}
		if fd.Recv == nil && (fd.Name.Name == "main" || fd.Name.Name == "init") {
			motivo = "es la función " + fd.Name.Name + ", que no recibe ni devuelve nada"
		}
		info.NoProbable = motivo
		info.Ciclomatica = ciclomatica(fd)
		if fd.Body != nil { // a declaration without a body, or half-typed code such as "func f( {"
			info.Anidamiento = anidamiento(fd.Body)
		}
		r := comp.de(fd)
		info.Recursiva, info.Exponencial, info.OGrande, info.Motivo = r.recursiva, r.exponencial, r.ordenTxt, r.motivo
		inf.Funciones = append(inf.Funciones, info)
	}
	inf.Hallazgos = hallazgos(a)
	sort.SliceStable(inf.Hallazgos, func(i, j int) bool { return inf.Hallazgos[i].Linea < inf.Hallazgos[j].Linea })
	return inf, nil
}
