// Package nucleotest holds fakes of the nucleo ports for leaf tests: EjecutorFalso (a scripted
// sandbox), ContadorMemoria, BaseHechosMemoria, LematizadorSimple and RespuestasFijas.
// It must only be imported from _test.go files.
package nucleotest

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// EjecutorFalso is a scripted nucleo.Ejecutor. Compilar and Vet use go/parser + go/types in-process
// (no build, no run). Preparar returns a BinarioFalso whose results come from Funcs, keyed by variant
// index, so leaf tests can simulate "variant 3 is the correct mutant".
//
// Llamadas records every call by method name: "Revisar", "Compilar", "Vet", "Preparar",
// "PrepararPrograma", "Probar", "Correr". Use NumLlamadas to count them safely.
//
// The fields after Llamadas are optional extras for tests.
type EjecutorFalso struct {
	Funcs    map[int]func(in []nucleo.Valor) ([]nucleo.Valor, string) // results, panic message
	Llamadas []string

	// EvalProp decides whether a property holds for one case; nil means every property holds.
	EvalProp func(p nucleo.Propiedad, entradas, salidas []nucleo.Valor) bool
	// Programa answers Correr for programs prepared with PrepararPrograma; nil gives an honest error.
	Programa func(fuente string, c nucleo.CasoPrograma) (nucleo.Ejecucion, error)
	// Violaciones, when non-nil, replaces the built-in safety check in Revisar.
	Violaciones []nucleo.Violacion
	// EstadoFijo, when non-nil, is returned by Estado.
	EstadoFijo *nucleo.EstadoArenero

	mu sync.Mutex
}

var _ nucleo.Ejecutor = (*EjecutorFalso)(nil)

func (e *EjecutorFalso) anotar(que string) {
	e.mu.Lock()
	e.Llamadas = append(e.Llamadas, que)
	e.mu.Unlock()
}

// NumLlamadas counts the recorded calls named que.
func (e *EjecutorFalso) NumLlamadas(que string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, l := range e.Llamadas {
		if l == que {
			n++
		}
	}
	return n
}

// permitidosFalso is a small allowlist that mirrors arenero's spirit (§7.1), enough for leaf tests.
var permitidosFalso = map[string]bool{
	"fmt": true, "strings": true, "strconv": true, "sort": true, "slices": true, "maps": true,
	"math": true, "math/big": true, "math/bits": true, "math/rand": true, "math/rand/v2": true, "math/cmplx": true,
	"unicode": true, "unicode/utf8": true, "unicode/utf16": true, "errors": true, "bytes": true, "bufio": true,
	"container/heap": true, "container/list": true, "container/ring": true, "regexp": true, "cmp": true,
	"time": true, "os": true, "io": true, "encoding/json": true, "encoding/hex": true, "encoding/base64": true,
	"encoding/csv": true, "hash/crc32": true, "hash/fnv": true, "crypto/sha256": true, "crypto/md5": true,
	"text/tabwriter": true, "html": true, "path": true,
}

var riesgosFalso = map[string]string{
	"os/exec":       "usa el paquete os/exec: puede ejecutar otros programas",
	"net":           "usa el paquete net: puede conectarse a internet",
	"net/http":      "usa el paquete net/http: puede conectarse a internet",
	"syscall":       "usa el paquete syscall: habla directamente con el sistema",
	"unsafe":        "usa el paquete unsafe: puede romper la memoria del programa",
	"os/signal":     "usa el paquete os/signal: maneja señales del sistema",
	"path/filepath": "usa el paquete path/filepath: puede recorrer tus carpetas",
	"io/ioutil":     "usa el paquete io/ioutil: puede leer y escribir archivos",
	"runtime":       "usa el paquete runtime: puede cambiar cómo se ejecuta el programa",
	"reflect":       "usa el paquete reflect: puede saltarse las reglas de tipos",
	"sync":          "usa el paquete sync: trabaja con goroutines",
}

// Revisar is a simplified static safety check (imports, directives, go statements, the nyx__ prefix).
func (e *EjecutorFalso) Revisar(fuente string, p nucleo.Permiso) []nucleo.Violacion {
	e.anotar("Revisar")
	if e.Violaciones != nil {
		return filtrarPermiso(append([]nucleo.Violacion(nil), e.Violaciones...), p)
	}
	return filtrarPermiso(revisarFalso(fuente, p), p)
}

// filtrarPermiso keeps only what blocks a run under p: everything for PermisoAuto, Grave for PermisoUsuario.
func filtrarPermiso(vs []nucleo.Violacion, p nucleo.Permiso) []nucleo.Violacion {
	if p == nucleo.PermisoAuto {
		return vs
	}
	var out []nucleo.Violacion
	for _, v := range vs {
		if v.Grave {
			out = append(out, v)
		}
	}
	return out
}

func revisarFalso(fuente string, p nucleo.Permiso) []nucleo.Violacion {
	var out []nucleo.Violacion
	if len(fuente) > 64<<10 {
		out = append(out, nucleo.Violacion{Que: "el archivo es demasiado grande (más de 64 KiB)", Grave: true})
	}
	for i, l := range strings.Split(fuente, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "//go:") || strings.HasPrefix(t, "//line") || strings.HasPrefix(t, "//export") || strings.HasPrefix(t, "#cgo") {
			out = append(out, nucleo.Violacion{Linea: i + 1, Que: "usa una directiva especial del compilador (" + t + ")", Grave: true})
		}
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.ParseComments)
	if err != nil {
		return out // compilation will fail anyway
	}
	for _, im := range f.Imports {
		ruta, _ := strconv.Unquote(im.Path.Value)
		linea := fset.Position(im.Pos()).Line
		switch {
		case ruta == "C":
			out = append(out, nucleo.Violacion{Linea: linea, Paquete: ruta, Que: "usa cgo (import \"C\")", Grave: true})
		case im.Name != nil && im.Name.Name == "." && (ruta == "os" || ruta == "time"):
			out = append(out, nucleo.Violacion{Linea: linea, Paquete: ruta, Que: "importa " + ruta + " con punto", Grave: true})
		case strings.Contains(strings.SplitN(ruta, "/", 2)[0], "."):
			out = append(out, nucleo.Violacion{Linea: linea, Paquete: ruta, Que: "usa un paquete de fuera de la biblioteca estándar (" + ruta + ")", Grave: true})
		case !permitidosFalso[ruta]:
			que := riesgosFalso[ruta]
			if que == "" {
				que = "usa el paquete " + ruta + ", que no está en la lista segura"
			}
			out = append(out, nucleo.Violacion{Linea: linea, Paquete: ruta, Que: que})
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			if p == nucleo.PermisoAuto {
				out = append(out, nucleo.Violacion{Linea: fset.Position(x.Pos()).Line, Que: "lanza una goroutine (go …)"})
			}
		case *ast.Ident:
			if strings.HasPrefix(strings.ToLower(x.Name), "nyx__") {
				out = append(out, nucleo.Violacion{Linea: fset.Position(x.Pos()).Line, Que: "usa el prefijo reservado nyx__", Grave: true})
			}
		}
		return true
	})
	return out
}

// ---- in-process type checking ----

var (
	muImportador sync.Mutex
	importadorGc types.Importer
	importadorFt types.Importer
	fsetFuente   = token.NewFileSet()
	cacheImport  = map[string]*types.Package{}
)

type importador struct{}

func (importador) Import(ruta string) (*types.Package, error) {
	muImportador.Lock()
	defer muImportador.Unlock()
	if p, ok := cacheImport[ruta]; ok {
		return p, nil
	}
	if importadorGc == nil {
		importadorGc = importer.Default()
		importadorFt = importer.ForCompiler(fsetFuente, "source", nil)
	}
	p, err := importadorGc.Import(ruta)
	if err != nil {
		p, err = importadorFt.Import(ruta)
	}
	if err != nil {
		return nil, err
	}
	cacheImport[ruta] = p
	return p, nil
}

// revisarTipos parses and type-checks one file; it returns errors in compiler style.
func revisarTipos(fuente string) []nucleo.ErrorGo {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.AllErrors)
	if err != nil {
		var lista scanner.ErrorList
		if errors.As(err, &lista) {
			out := make([]nucleo.ErrorGo, 0, len(lista))
			for _, e := range lista {
				out = append(out, nucleo.ErrorGo{Archivo: filepath.Base(e.Pos.Filename), Linea: e.Pos.Line, Col: e.Pos.Column, Msg: e.Msg})
			}
			return out
		}
		return []nucleo.ErrorGo{{Archivo: "x.go", Linea: 1, Col: 1, Msg: err.Error()}}
	}
	var out []nucleo.ErrorGo
	conf := types.Config{
		Importer: importador{},
		Error: func(err error) {
			if te, ok := err.(types.Error); ok {
				pos := te.Fset.Position(te.Pos)
				out = append(out, nucleo.ErrorGo{Archivo: filepath.Base(pos.Filename), Linea: pos.Line, Col: pos.Column, Msg: te.Msg})
				return
			}
			out = append(out, nucleo.ErrorGo{Archivo: "x.go", Linea: 1, Col: 1, Msg: err.Error()})
		},
	}
	_, _ = conf.Check(f.Name.Name, fset, []*ast.File{f}, nil)
	if f.Name.Name == "main" && len(out) == 0 {
		tieneMain := false
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "main" {
				tieneMain = true
			}
		}
		if !tieneMain {
			out = append(out, nucleo.ErrorGo{Archivo: "x.go", Linea: 1, Col: 1, Msg: "function main is undeclared in the main package"})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Linea != out[j].Linea {
			return out[i].Linea < out[j].Linea
		}
		return out[i].Col < out[j].Col
	})
	return out
}

func compilacion(comando string, errs []nucleo.ErrorGo, inicio time.Time) nucleo.Compilacion {
	c := nucleo.Compilacion{OK: len(errs) == 0, Errores: errs, Comando: comando, Ms: time.Since(inicio).Milliseconds()}
	var sb strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&sb, "./%s:%d:%d: %s\n", e.Archivo, e.Linea, e.Col, e.Msg)
	}
	c.Texto = sb.String()
	return c
}

// Compilar type-checks the file in-process (go/parser + go/types).
func (e *EjecutorFalso) Compilar(ctx context.Context, fuente string) nucleo.Compilacion {
	e.anotar("Compilar")
	inicio := time.Now()
	return compilacion("go/types (falso)", revisarTipos(fuente), inicio)
}

// Vet is Compilar plus nothing else: the fake has no vet analyzers.
func (e *EjecutorFalso) Vet(ctx context.Context, fuente string) nucleo.Compilacion {
	e.anotar("Vet")
	inicio := time.Now()
	return compilacion("go vet (falso: solo go/types)", revisarTipos(fuente), inicio)
}

// Preparar checks the signature and the safety rules, then returns a BinarioFalso. It does not compile
// the variants (use Compilar for that). With no Variantes, the variants are 0..max(Funcs keys).
func (e *EjecutorFalso) Preparar(ctx context.Context, p nucleo.Preparacion) (nucleo.Binario, nucleo.Compilacion, error) {
	e.anotar("Preparar")
	comp := nucleo.Compilacion{OK: true, Comando: "preparar (falso)"}
	if err := ctx.Err(); err != nil {
		return nil, comp, err
	}
	if !p.Firma.Probable() {
		return nil, nucleo.Compilacion{Comando: comp.Comando}, nucleo.ErrNoProbable
	}
	for _, v := range p.Variantes {
		vs := revisarFalso(v, p.Permiso)
		if e.Violaciones != nil {
			vs = e.Violaciones
		}
		if len(filtrarPermiso(vs, p.Permiso)) > 0 {
			return nil, nucleo.Compilacion{Comando: comp.Comando}, nucleo.ErrInseguro
		}
	}
	n := len(p.Variantes)
	if n == 0 {
		for k := range e.Funcs {
			if k+1 > n {
				n = k + 1
			}
		}
	}
	funcs := make(map[int]func([]nucleo.Valor) ([]nucleo.Valor, string), len(e.Funcs))
	for k, f := range e.Funcs {
		funcs[k] = f
	}
	return &BinarioFalso{e: e, n: n, funcs: funcs, props: append([]nucleo.Propiedad(nil), p.Props...), firma: p.Firma}, comp, nil
}

// PrepararPrograma returns a ProgramaFalso that answers through the Programa field.
func (e *EjecutorFalso) PrepararPrograma(ctx context.Context, f string, i nucleo.OpcionesInstr, p nucleo.Permiso) (nucleo.Programa, nucleo.Compilacion, error) {
	e.anotar("PrepararPrograma")
	if err := ctx.Err(); err != nil {
		return nil, nucleo.Compilacion{}, err
	}
	vs := revisarFalso(f, p)
	if e.Violaciones != nil {
		vs = e.Violaciones
	}
	if len(filtrarPermiso(vs, p)) > 0 {
		return nil, nucleo.Compilacion{Comando: "preparar programa (falso)"}, nucleo.ErrInseguro
	}
	comp := compilacion("go/types (falso)", revisarTipos(f), time.Now())
	if !comp.OK {
		return nil, comp, nucleo.ErrNoCompila
	}
	return &ProgramaFalso{e: e, fuente: f}, comp, nil
}

// Estado returns EstadoFijo, or a fake state ("básica", Go available).
func (e *EjecutorFalso) Estado() nucleo.EstadoArenero {
	if e.EstadoFijo != nil {
		return *e.EstadoFijo
	}
	return nucleo.EstadoArenero{GoVersion: "go (falso)", GoOK: true, Nivel: "básica"}
}

// BinarioFalso runs the EjecutorFalso.Funcs in-process.
type BinarioFalso struct {
	e      *EjecutorFalso
	n      int
	funcs  map[int]func([]nucleo.Valor) ([]nucleo.Valor, string)
	props  []nucleo.Propiedad
	firma  nucleo.Firma
	mu     sync.Mutex
	cerrar bool
}

var _ nucleo.Binario = (*BinarioFalso)(nil)

// llamar runs f with a copy of the inputs and turns a real panic into a panic message.
func llamar(f func([]nucleo.Valor) ([]nucleo.Valor, string), in []nucleo.Valor) (out []nucleo.Valor, panico string) {
	defer func() {
		if r := recover(); r != nil {
			out, panico = nil, fmt.Sprint(r)
		}
	}()
	copia := make([]nucleo.Valor, len(in))
	for i, v := range in {
		copia[i] = nucleo.Copiar(v)
	}
	return f(copia)
}

// Probar returns [variante][caso]. A variant without a Func is not run (Ejecutado=false).
func (b *BinarioFalso) Probar(ctx context.Context, casos []nucleo.Caso, op nucleo.OpcionesProbar) ([][]nucleo.ResultadoCaso, error) {
	b.e.anotar("Probar")
	b.mu.Lock()
	cerrado := b.cerrar
	b.mu.Unlock()
	if cerrado {
		return nil, errors.New("nucleotest: el binario ya está cerrado")
	}
	variantes := op.Variantes
	if variantes == nil {
		for v := 0; v < b.n; v++ {
			variantes = append(variantes, v)
		}
	}
	out := make([][]nucleo.ResultadoCaso, b.n)
	for v := 0; v < b.n; v++ {
		out[v] = make([]nucleo.ResultadoCaso, len(casos))
		for c := range casos {
			out[v][c] = nucleo.ResultadoCaso{Variante: v, Caso: c}
		}
	}
	for _, v := range variantes {
		if v < 0 || v >= b.n {
			return nil, fmt.Errorf("nucleotest: la variante %d no existe (hay %d)", v, b.n)
		}
		f := b.funcs[v]
		if f == nil {
			continue
		}
		for c, caso := range casos {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			r := &out[v][c]
			r.Ejecutado = true
			inicio := time.Now()
			obt, pan := llamar(f, caso.Entradas)
			r.Micros = time.Since(inicio).Microseconds()
			r.Obtenido, r.Panico = obt, pan
			r.OK = pan == ""
			if r.OK && caso.ConEsperado() {
				r.OK = len(obt) == len(caso.Esperado)
				for i := 0; r.OK && i < len(obt); i++ {
					r.OK = nucleo.Igual(obt[i], caso.Esperado[i])
				}
			}
			if r.OK && b.e.EvalProp != nil {
				for _, p := range b.props {
					if !b.e.EvalProp(p, caso.Entradas, obt) {
						r.OK, r.PropFallida = false, p.Nombre
						break
					}
				}
			}
			if r.OK && op.Repetir >= 2 {
				for k := 1; k < op.Repetir; k++ {
					otra, pan2 := llamar(f, caso.Entradas)
					if pan2 != "" || !igualFila(otra, obt) {
						r.NoDeterminista, r.OK = true, false
						break
					}
				}
			}
			if !r.OK && op.PararAlFallar {
				break
			}
		}
	}
	return out, nil
}

func igualFila(a, b []nucleo.Valor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !nucleo.Igual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// Lineas has no instrumentation in the fake: it returns nil.
func (b *BinarioFalso) Lineas(variante int) []int { return nil }

func (b *BinarioFalso) NumVariantes() int { return b.n }

func (b *BinarioFalso) Cerrar() error {
	b.mu.Lock()
	b.cerrar = true
	b.mu.Unlock()
	return nil
}

// ProgramaFalso answers Correr with EjecutorFalso.Programa.
type ProgramaFalso struct {
	e      *EjecutorFalso
	fuente string
}

var _ nucleo.Programa = (*ProgramaFalso)(nil)

func (p *ProgramaFalso) Correr(ctx context.Context, c nucleo.CasoPrograma) (nucleo.Ejecucion, error) {
	p.e.anotar("Correr")
	if err := ctx.Err(); err != nil {
		return nucleo.Ejecucion{}, err
	}
	if p.e.Programa == nil {
		return nucleo.Ejecucion{}, errors.New("nucleotest: EjecutorFalso.Programa no está definido; el falso no ejecuta programas de verdad")
	}
	return p.e.Programa(p.fuente, c)
}

func (p *ProgramaFalso) Cerrar() error { return nil }
