package recetas

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// Renombrar returns a copy of r whose function (or method) is called nombre: the declaration, every call
// in the file and the first word of its doc comment change (go/ast). r is returned unchanged when nombre
// is not a valid identifier or is already used by another top-level declaration.
func Renombrar(r Receta, nombre string) Receta {
	out := copiarReceta(r)
	viejo := r.Firma.Nombre
	if nombre == "" || nombre == viejo || !token.IsIdentifier(nombre) || token.IsKeyword(nombre) {
		return out
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", r.Codigo, parser.ParseComments)
	if err != nil {
		return out
	}
	metodo := r.Firma.Receptor != nil
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
