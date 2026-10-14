package recetas

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Renombrar returns a copy of r whose function (or method) is called nombre: the declaration, every call
// in the file and the first word of its doc comment change (go/ast). r is returned unchanged when the
// rename could break the code: nombre is not a valid identifier, is "_", "init" or "main", is a
// predeclared name (len, string, true…) for a function, or is already used anywhere in the file (another
// declaration, an import, a field of the receiver, a parameter or local that would shadow it…); and, for a
// method, when another type of the file has a method or field with the old name (calls are renamed by
// name, so they could not be told apart).
func Renombrar(r Receta, nombre string) Receta {
	out := copiarReceta(r)
	viejo := r.Firma.Nombre
	if nombre == "" || nombre == viejo || !token.IsIdentifier(nombre) || token.IsKeyword(nombre) ||
		nombre == "_" || nombre == "init" || nombre == "main" {
		return out
	}
	metodo := r.Firma.Receptor != nil
	if !metodo && types.Universe.Lookup(nombre) != nil {
		return out
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", r.Codigo, parser.ParseComments)
	if err != nil {
		return out
	}
	if nombreUsado(f, nombre) {
		return out
	}
	if metodo && viejoAmbiguo(f, viejo, nombreTipoReceptorFirma(r.Firma)) {
		return out
	}
	var fd *ast.FuncDecl
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == nombre && (x.Recv == nil) == !metodo {
				return out // the new name is taken
			}
			if x.Name.Name == viejo && (x.Recv != nil) == metodo {
				fd = x
			}
		case *ast.GenDecl:
			if metodo {
				continue
			}
			for _, sp := range x.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == nombre {
						return out
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name == nombre {
							return out
						}
					}
				}
			}
		}
	}
	if fd == nil {
		return out
	}
	if !metodo {
		obj := fd.Name.Obj
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == viejo && (id.Obj == obj || id.Obj == nil) {
				id.Name = nombre
			}
			return true
		})
	} else {
		fd.Name.Name = nombre
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == viejo {
				sel.Sel.Name = nombre
			}
			return true
		})
	}
	if fd.Doc != nil && len(fd.Doc.List) > 0 {
		c := fd.Doc.List[0]
		if strings.HasPrefix(c.Text, "// "+viejo+" ") || c.Text == "// "+viejo {
			c.Text = "// " + nombre + strings.TrimPrefix(c.Text, "// "+viejo)
		}
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return out
	}
	out.Codigo = buf.String()
	out.Firma.Nombre = nombre
	out.Nombre = nombre
	for i := range out.Casos {
		out.Casos[i].Nota = strings.Replace(out.Casos[i].Nota, "receta "+r.Nombre, "receta "+nombre, 1)
	}
	return out
}

// nombreUsado reports whether nombre already names something in the file: any identifier (declarations,
// locals, parameters, fields, methods, labels, selectors, explicit import names) or the implicit name of
// an import.
func nombreUsado(f *ast.File, nombre string) bool {
	usado := false
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == nombre {
			usado = true
		}
		return !usado
	})
	if usado {
		return true
	}
	for _, im := range f.Imports {
		if im.Name == nil && nombreImplicito(im.Path.Value) == nombre {
			return true
		}
	}
	return false
}

// nombreImplicito guesses the package name of an import path: its last element, without a version suffix.
func nombreImplicito(literal string) string {
	ruta, err := strconv.Unquote(literal)
	if err != nil {
		return ""
	}
	partes := strings.Split(ruta, "/")
	ultimo := partes[len(partes)-1]
	if len(partes) > 1 && len(ultimo) > 1 && ultimo[0] == 'v' && strings.Trim(ultimo[1:], "0123456789") == "" {
		ultimo = partes[len(partes)-2]
	}
	if i := strings.Index(ultimo, ".v"); i > 0 {
		ultimo = ultimo[:i]
	}
	return strings.NewReplacer("-", "_", ".", "_").Replace(ultimo)
}

// viejoAmbiguo reports whether a type other than tipo also has a method or a field called viejo.
func viejoAmbiguo(f *ast.File, viejo, tipo string) bool {
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Recv != nil && len(x.Recv.List) > 0 && x.Name.Name == viejo && nombreTipoReceptor(x.Recv.List[0].Type) != tipo {
				return true
			}
		case *ast.GenDecl:
			for _, sp := range x.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok {
					continue
				}
				ambiguo := false
				ast.Inspect(ts.Type, func(n ast.Node) bool {
					if campo, ok := n.(*ast.Field); ok {
						for _, id := range campo.Names {
							if id.Name == viejo {
								ambiguo = true
							}
						}
					}
					return !ambiguo
				})
				if ambiguo {
					return true
				}
			}
		}
	}
	return false
}

// nombreTipoReceptorFirma is the base type name of a method's receiver ("Rectangulo" for *Rectangulo).
func nombreTipoReceptorFirma(f nucleo.Firma) string {
	if f.Receptor == nil {
		return ""
	}
	t := f.Receptor.Tipo
	if t.Clase == nucleo.CPuntero && t.Elem != nil {
		t = *t.Elem
	}
	return t.Nombre
}
