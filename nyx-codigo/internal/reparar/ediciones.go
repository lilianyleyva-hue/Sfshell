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
// the imports. It returns the new source and the edits applied. It runs in O(len(src) + n log n): the
// chosen edits are kept sorted by position (so an overlap check looks only at the neighbours) and the
// result is written once.
func aplicar(src string, eds []edicion) (string, []edicion) {
	var elegidas []edicion
	var rangos []edicion // chosen text edits, sorted by (ini, fin)
	for _, e := range eds {
		if e.soloImport() {
			elegidas = append(elegidas, e)
			continue
		}
		if e.ini > e.fin || e.fin > len(src) {
			continue
		}
		// first chosen edit that starts at or after e.ini
		p := sort.Search(len(rangos), func(i int) bool {
			return rangos[i].ini > e.ini || rangos[i].ini == e.ini && rangos[i].fin >= e.fin
		})
		choca := false
		for j := p - 1; j >= 0; j-- { // earlier edits end in order: stop once they end before e
			if solapan(e, rangos[j]) {
				choca = true
				break
			}
			if rangos[j].fin < e.ini {
				break
			}
		}
		for j := p; j < len(rangos) && !choca; j++ {
			if solapan(e, rangos[j]) {
				choca = true
				break
			}
			if rangos[j].ini > e.fin {
				break
			}
		}
		if choca {
			continue
		}
		elegidas = append(elegidas, e)
		rangos = append(rangos, edicion{})
		copy(rangos[p+1:], rangos[p:])
		rangos[p] = e
	}
	if len(rangos) > 0 {
		var sb strings.Builder
		sb.Grow(len(src))
		ultimo := 0
		for _, e := range rangos {
			if e.ini < ultimo {
				continue // cannot happen: the chosen edits do not overlap
			}
			sb.WriteString(src[ultimo:e.ini])
			sb.WriteString(e.texto)
			ultimo = e.fin
		}
		sb.WriteString(src[ultimo:])
		src = sb.String()
	}
	for _, e := range elegidas {
		for _, ruta := range e.importar {
			src = agregarImport(src, ruta)
		}
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
