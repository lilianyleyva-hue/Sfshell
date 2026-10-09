package instrumenta

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
)

// Renombrar appends sufijo to every top-level identifier declared in the file (funcs, types, vars, consts) and
// rewrites every use, resolved with go/types (methods follow their renamed type automatically; field names and
// method names are NOT renamed). Returns the new source and old→new names.
//
// `init` and the blank identifier are never renamed. An embedded field whose type is renamed changes its
// name with the type (Go names such a field after its type), and so do the selectors and keys that use it.
// Only identifiers change, in place, so every line keeps its number.
func Renombrar(fuente, sufijo string) (string, map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return "", nil, fmt.Errorf("instrumenta: el código no se puede leer: %w", err)
	}
	globales := nombresGlobales(f)
	delete(globales, "init")
	nombres := map[string]string{}
	if sufijo == "" || len(globales) == 0 {
		return fuente, nombres, nil
	}
	for n := range globales {
		nombres[n] = n + sufijo
	}

	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}
	conf := types.Config{
		Importer: importadorVacio{},
		Error:    func(error) {}, // keep going: we only need the identifiers resolved
	}
	pkg, _ := conf.Check(f.Name.Name, fset, []*ast.File{f}, info)

	renombrar := map[*ast.Ident]bool{}
	resuelto := map[*ast.Ident]bool{}
	esGlobal := func(obj types.Object) bool {
		if obj == nil || pkg == nil || obj.Pkg() != pkg {
			return false
		}
		if obj.Parent() != pkg.Scope() {
			return false
		}
		_, ok := nombres[obj.Name()]
		return ok
	}
	campoEmbebido := func(obj types.Object) bool {
		v, ok := obj.(*types.Var)
		if !ok || !v.IsField() || !v.Embedded() {
			return false
		}
		t := v.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		n, ok := t.(*types.Named)
		return ok && esGlobal(n.Obj())
	}
	for id, obj := range info.Defs {
		resuelto[id] = true
		if obj != nil && (esGlobal(obj) || campoEmbebido(obj)) {
			renombrar[id] = true
		}
	}
	for id, obj := range info.Uses {
		resuelto[id] = true
		if esGlobal(obj) || campoEmbebido(obj) {
			renombrar[id] = true
		}
	}
	// Fallback for identifiers go/types could not resolve (code with errors): rename a free identifier
	// with a top-level name unless it is clearly something else (a selector, a key, a field, a label).
	noLibres := map[*ast.Ident]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			noLibres[x.Sel] = true
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok {
				noLibres[id] = true
			}
		case *ast.Field:
			for _, id := range x.Names {
				noLibres[id] = true
			}
		case *ast.LabeledStmt:
			noLibres[x.Label] = true
		case *ast.BranchStmt:
			if x.Label != nil {
				noLibres[x.Label] = true
			}
		case *ast.FuncDecl:
			if x.Recv != nil {
				noLibres[x.Name] = true
			}
		}
		return true
	})
	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || resuelto[id] || noLibres[id] {
			return true
		}
		if _, ok := nombres[id.Name]; ok {
			renombrar[id] = true
		}
		return true
	})
	// the package clause is never renamed
	delete(renombrar, f.Name)

	tf := fset.File(f.Pos())
	type cambio struct {
		ini, fin int
		texto    string
	}
	cambios := make([]cambio, 0, len(renombrar))
	for id := range renombrar {
		cambios = append(cambios, cambio{tf.Offset(id.Pos()), tf.Offset(id.End()), id.Name + sufijo})
	}
	sort.Slice(cambios, func(i, j int) bool { return cambios[i].ini < cambios[j].ini })
	var sb strings.Builder
	sb.Grow(len(fuente) + len(cambios)*len(sufijo))
	ult := 0
	for _, c := range cambios {
		if c.ini < ult {
			continue
		}
		sb.WriteString(fuente[ult:c.ini])
		sb.WriteString(c.texto)
		ult = c.fin
	}
	sb.WriteString(fuente[ult:])
	return sb.String(), nombres, nil
}

// CambiarPaquete replaces the package clause name (in place, so lines do not move).
func CambiarPaquete(fuente, paquete string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("instrumenta: no encuentro la línea «package»: %w", err)
	}
	tf := fset.File(f.Pos())
	ini, fin := tf.Offset(f.Name.Pos()), tf.Offset(f.Name.End())
	return fuente[:ini] + paquete + fuente[fin:], nil
}

// importadorVacio gives every import an empty, complete package with the usual name, so type
// checking resolves the file's own identifiers without reading anything from disk.
type importadorVacio struct{}

func (importadorVacio) Import(ruta string) (*types.Package, error) {
	p := types.NewPackage(ruta, nombrePaquete(ruta))
	p.MarkComplete()
	return p, nil
}

// nombrePaquete guesses the package name of an import path: the last element, skipping a /vN suffix.
func nombrePaquete(ruta string) string {
	partes := strings.Split(ruta, "/")
	n := partes[len(partes)-1]
	if len(partes) > 1 && len(n) > 1 && n[0] == 'v' {
		if _, err := strconv.Atoi(n[1:]); err == nil {
			n = partes[len(partes)-2]
		}
	}
	n = strings.TrimPrefix(n, "go-")
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, ".", "_")
	return n
}
