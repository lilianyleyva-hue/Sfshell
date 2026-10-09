// Package analisis does the static and dynamic analysis of Go code for Nyx Código (§4.5): signatures
// (nucleo.Tipo trees built from go/types), normalization of snippets, findings, complexity, Spanish
// narration, variable roles, generated comments, verified behaviour summaries, variable traces and
// measured growth. It never builds anything itself: dynamic work goes through a nucleo.Binario.
package analisis

import (
	"errors"
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
	fset    *token.FileSet
	f       *ast.File
	info    *types.Info
	pkg     *types.Package
	errores []nucleo.ErrorGo
	envuelto bool // bare statements were wrapped in func main() (same lines)
	prefijo  int  // bytes added before the user's code on line 1
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
		a.f, a.errores = f, erroresDeParse(err, 0)
		return a, nil
	}
	f, err := parser.ParseFile(a.fset, "x.go", prefijoPaquete+src, modo)
	if err == nil {
		a.f, a.prefijo = f, len(prefijoPaquete)
		return a, nil
	}
	fset2 := token.NewFileSet()
	f2, err2 := parser.ParseFile(fset2, "x.go", prefijoMain+src+"\n}", modo)
	if err2 == nil {
		a.fset, a.f, a.prefijo, a.envuelto = fset2, f2, len(prefijoMain), true
		return a, nil
	}
	if f == nil {
		return nil, err
	}
	a.f, a.prefijo, a.errores = f, len(prefijoPaquete), erroresDeParse(err, len(prefijoPaquete))
	return a, nil
}

// ---- type checking ----

var (
	muImportador sync.Mutex
	importadorGc types.Importer
	importadorFt types.Importer
	fsetFuentes  = token.NewFileSet()
	cachePaq     = map[string]*types.Package{}
	fallosPaq    = map[string]error{}
)

type importador struct{}

// Import uses export data (go list -export) and falls back to type-checking GOROOT sources.
// Results, and failures, are cached for the life of the process.
func (importador) Import(ruta string) (*types.Package, error) {
	muImportador.Lock()
	defer muImportador.Unlock()
	if p, ok := cachePaq[ruta]; ok {
		return p, nil
	}
	if err, ok := fallosPaq[ruta]; ok {
		return nil, err
	}
	if importadorGc == nil {
		importadorGc = importer.Default()
		importadorFt = importer.ForCompiler(fsetFuentes, "source", nil)
	}
	p, err := importadorGc.Import(ruta)
	if err != nil {
		p, err = importadorFt.Import(ruta)
	}
	if err != nil {
		fallosPaq[ruta] = err
		return nil, err
	}
	cachePaq[ruta] = p
	return p, nil
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
		Importer: importador{},
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
func TipoDe(t types.Type) (nucleo.Tipo, bool) {
	r, motivo := tipoDe(t, map[*types.Named]bool{})
	return r, motivo == ""
}

// tipoDe returns the type, or a Spanish reason why it cannot be represented.
func tipoDe(t types.Type, visto map[*types.Named]bool) (nucleo.Tipo, string) {
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
		if esDeOtroPaquete(obj) {
			return nucleo.Tipo{}, "usa el tipo " + obj.Pkg().Name() + "." + obj.Name() + " de otro paquete"
		}
		if visto[x] {
			return nucleo.Tipo{}, "usa un tipo recursivo (" + obj.Name() + ")"
		}
		visto[x] = true
		defer delete(visto, x)
		u, motivo := tipoDe(x.Underlying(), visto)
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		if u.Clase == nucleo.CError {
			return nucleo.Tipo{}, "usa el tipo " + obj.Name() + ", que es una interfaz"
		}
		u.Nombre, u.Definido = obj.Name(), true
		return u, ""
	case *types.Pointer:
		e, motivo := tipoDe(x.Elem(), visto)
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		if e.Clase != nucleo.CStruct {
			return nucleo.Tipo{}, "usa punteros a algo que no es una estructura"
		}
		return nucleo.PunteroA(e), ""
	case *types.Slice:
		e, motivo := tipoDe(x.Elem(), visto)
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		return nucleo.ListaDe(e), ""
	case *types.Array:
		e, motivo := tipoDe(x.Elem(), visto)
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		return nucleo.ArregloDe(int(x.Len()), e), ""
	case *types.Map:
		k, motivo := tipoDe(x.Key(), visto)
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		switch k.Clase {
		case nucleo.CInt, nucleo.CString, nucleo.CRune, nucleo.CByte, nucleo.CBool:
		default:
			return nucleo.Tipo{}, "usa un mapa con claves de tipo " + k.Go()
		}
		v, motivo := tipoDe(x.Elem(), visto)
		if motivo != "" {
			return nucleo.Tipo{}, motivo
		}
		return nucleo.MapaDe(k, v), ""
	case *types.Struct:
		out := nucleo.Tipo{Clase: nucleo.CStruct}
		for i := 0; i < x.NumFields(); i++ {
			c := x.Field(i)
			ct, motivo := tipoDe(c.Type(), visto)
			if motivo != "" {
				return nucleo.Tipo{}, motivo
			}
			if ct.Clase == nucleo.CError {
				return nucleo.Tipo{}, "tiene un campo de tipo error"
			}
			out.Campos = append(out.Campos, nucleo.Campo{Nombre: c.Name(), Tipo: ct})
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

// esDeOtroPaquete is true for named types declared in another (standard library) package, such as
// big.Int or time.Duration: the harness could not declare them in the candidate's package.
func esDeOtroPaquete(obj *types.TypeName) bool {
	p := obj.Pkg()
	if p == nil || p.Path() == "main" {
		return false
	}
	muLocales.Lock()
	defer muLocales.Unlock()
	if r, ok := foraneos[p.Path()]; ok {
		return r
	}
	r := false
	if raiz := build.Default.GOROOT; raiz != "" {
		if st, err := os.Stat(filepath.Join(raiz, "src", filepath.FromSlash(p.Path()))); err == nil && st.IsDir() {
			r = true
		}
	}
	if !r {
		muImportador.Lock()
		_, r = cachePaq[p.Path()]
		muImportador.Unlock()
	}
	foraneos[p.Path()] = r
	return r
}

var (
	muLocales sync.Mutex
	foraneos  = map[string]bool{}
)

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
	visto := map[*types.Named]bool{}
	if r := sig.Recv(); r != nil {
		t, motivo := tipoDe(r.Type(), visto)
		if motivo != "" {
			return nil, "es un método de un tipo que no sé representar: " + motivo
		}
		f.Receptor = &nucleo.Param{Nombre: nombreParam(r.Name()), Tipo: t}
	}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		t, motivo := tipoDe(p.Type(), visto)
		if motivo != "" {
			return nil, motivo
		}
		f.Params = append(f.Params, nucleo.Param{Nombre: nombreParam(p.Name()), Tipo: t})
	}
	for i := 0; i < sig.Results().Len(); i++ {
		t, motivo := tipoDe(sig.Results().At(i).Type(), visto)
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
		info.Anidamiento = anidamiento(fd.Body)
		r := comp.de(fd)
		info.Recursiva, info.Exponencial, info.OGrande, info.Motivo = r.recursiva, r.exponencial, r.ordenTxt, r.motivo
		inf.Funciones = append(inf.Funciones, info)
	}
	inf.Hallazgos = hallazgos(a)
	sort.SliceStable(inf.Hallazgos, func(i, j int) bool { return inf.Hallazgos[i].Linea < inf.Hallazgos[j].Linea })
	return inf, nil
}
