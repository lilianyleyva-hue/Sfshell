package reparar

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// edicion replaces fuente[ini:fin] with texto. Imports listed in importar are added afterwards.
type edicion struct {
	ini, fin   int
	texto      string
	importar   []string
	cambio     Cambio
	suposicion string
}

func (e edicion) soloImport() bool { return e.ini < 0 }

func solapan(a, b edicion) bool {
	if a.soloImport() || b.soloImport() {
		return false
	}
	switch {
	case a.ini == a.fin && b.ini == b.fin:
		return a.ini == b.ini
	case a.ini == a.fin:
		return a.ini > b.ini && a.ini < b.fin
	case b.ini == b.fin:
		return b.ini > a.ini && b.ini < a.fin
	}
	return a.ini < b.fin && b.ini < a.fin
}

// aplicar applies the edits that do not overlap an earlier one (in the given order) and then adds
// the imports. It returns the new source and the edits applied.
func aplicar(src string, eds []edicion) (string, []edicion) {
	var elegidas []edicion
	for _, e := range eds {
		ok := true
		for _, o := range elegidas {
			if solapan(e, o) {
				ok = false
				break
			}
		}
		if ok {
			elegidas = append(elegidas, e)
		}
	}
	rangos := make([]edicion, 0, len(elegidas))
	var importar []string
	for _, e := range elegidas {
		if !e.soloImport() {
			rangos = append(rangos, e)
		}
		importar = append(importar, e.importar...)
	}
	sort.SliceStable(rangos, func(i, j int) bool { return rangos[i].ini > rangos[j].ini })
	for _, e := range rangos {
		if e.ini < 0 || e.fin > len(src) || e.ini > e.fin {
			continue
		}
		src = src[:e.ini] + e.texto + src[e.fin:]
	}
	for _, ruta := range importar {
		src = agregarImport(src, ruta)
	}
	return src, elegidas
}

// importado reports whether src already imports ruta.
func importado(src, ruta string) bool {
	f, _ := parser.ParseFile(token.NewFileSet(), archivoFuente, src, parser.ImportsOnly)
	if f == nil {
		return strings.Contains(src, strconv.Quote(ruta))
	}
	for _, im := range f.Imports {
		if p, _ := strconv.Unquote(im.Path.Value); p == ruta {
			return true
		}
	}
	return false
}

// agregarImport adds import ruta (sorted inside an existing group, or as a new declaration).
func agregarImport(src, ruta string) string {
	if importado(src, ruta) {
		return src
	}
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, archivoFuente, src, parser.ImportsOnly)
	cita := strconv.Quote(ruta)
	if f == nil || f.Name == nil {
		i := strings.Index(src, "package ")
		if i < 0 {
			return "import " + cita + "\n\n" + src
		}
		fin := finLinea(src, i)
		return src[:fin] + "\n\nimport " + cita + src[fin:]
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT {
			continue
		}
		if g.Lparen.IsValid() {
			for _, s := range g.Specs {
				is := s.(*ast.ImportSpec)
				if is.Path.Value > cita {
					ini := inicioLinea(src, off(is.Pos()))
					if is.Name != nil {
						ini = inicioLinea(src, off(is.Name.Pos()))
					}
					return src[:ini] + "\t" + cita + "\n" + src[ini:]
				}
			}
			r := off(g.Rparen)
			ini := inicioLinea(src, r)
			if strings.TrimSpace(src[ini:r]) == "" {
				return src[:ini] + "\t" + cita + "\n" + src[ini:]
			}
			return src[:r] + "\n\t" + cita + "\n" + src[r:]
		}
		// import "x" → import ( "x"; "ruta" )
		is := g.Specs[0].(*ast.ImportSpec)
		viejo := src[off(is.Pos()):off(is.End())]
		lineas := []string{viejo, cita}
		if cita < is.Path.Value {
			lineas = []string{cita, viejo}
		}
		return src[:off(g.Pos())] + "import (\n\t" + strings.Join(lineas, "\n\t") + "\n)" + src[off(g.End()):]
	}
	fin := finLinea(src, off(f.Name.End()))
	return src[:fin] + "\n\nimport " + cita + src[fin:]
}

// quitarImport removes the import spec of ruta (and the whole declaration when it is the only one).
func quitarImport(src, ruta string) (edicion, bool) {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, archivoFuente, src, parser.ImportsOnly)
	if f == nil {
		return edicion{}, false
	}
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT {
			continue
		}
		for _, s := range g.Specs {
			is := s.(*ast.ImportSpec)
			if p, _ := strconv.Unquote(is.Path.Value); p != ruta {
				continue
			}
			var ini, fin int
			if len(g.Specs) == 1 {
				ini, fin = off(g.Pos()), off(g.End())
			} else {
				ini, fin = off(is.Pos()), off(is.End())
			}
			// take the whole line when nothing else is on it
			li, lf := inicioLinea(src, ini), finLinea(src, fin)
			if strings.TrimSpace(src[li:ini]) == "" && strings.TrimSpace(strings.SplitN(src[fin:lf], "//", 2)[0]) == "" {
				ini, fin = li, lf
				if fin < len(src) {
					fin++
				}
			}
			return edicion{ini: ini, fin: fin, texto: ""}, true
		}
	}
	return edicion{}, false
}
