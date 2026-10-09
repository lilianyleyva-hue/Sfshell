package analisis

// Optimizaciones (N3): rewrites that make code faster without changing what it computes. Each rewrite
// is applied on its own to the original file, re-formatted with gofmt and type-checked again; a rewrite
// that does not type-check is dropped.

import (
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reescritura is one suggested optimization: the finding that explains it and the whole rewritten file.
type Reescritura struct {
	Hallazgo Hallazgo
	Fuente   string // rewritten file
}

// Optimizaciones looks for three slow patterns in code that type-checks and returns one rewrite for each
// place found, ordered by line:
//
//   - a text built with += inside a loop → strings.Builder ("concat_en_bucle");
//   - slices.Contains(xs, v) inside a loop → a map used as a set, built once before the loop ("contains_en_bucle");
//   - a list that grows with append once per turn of a range loop → make with capacity ("append_sin_capacidad").
//
// A snippet that only type-checks after Normalizar (missing package clause or imports) is normalized
// first, and then the rewrites and their lines refer to the normalized file. Code with parse or type
// errors gives no rewrites.
func Optimizaciones(fuente string) []Reescritura {
	a, ok := optimizable(fuente)
	if !ok {
		if n, err := Normalizar(fuente, ""); err == nil {
			a, ok = optimizable(n)
		}
	}
	if !ok {
		return nil
	}
	o := &optimizador{a: a, padres: padresDe(a.f)}
	var cands []candidato
	cands = append(cands, o.concatenaciones()...)
	cands = append(cands, o.contiene()...)
	cands = append(cands, o.capacidades()...)
	var out []Reescritura
	for _, c := range cands {
		if src, ok := o.reescribir(c); ok {
			out = append(out, Reescritura{Hallazgo: c.h, Fuente: src})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Hallazgo.Linea < out[j].Hallazgo.Linea })
	return out
}

// optimizable parses and type-checks the source; it reports false on any error.
func optimizable(fuente string) (*archivo, bool) {
	a, err := parsear(fuente)
	if err != nil || len(a.errores) > 0 {
		return nil, false
	}
	a.comprobarTipos()
	if len(a.errores) > 0 || a.pkg == nil {
		return nil, false
	}
	return a, true
}

// edicion replaces the bytes [desde, hasta) of the parsed text.
type edicion struct {
	desde, hasta int
	texto        string
}

type candidato struct {
	h          Hallazgo
	eds        []edicion
	importar   string // import path to add ("strings"), if missing
	desimporta string // import path to remove when it is no longer used ("slices")
}

type optimizador struct {
	a      *archivo
	padres map[ast.Node]ast.Node
}

func padresDe(f *ast.File) map[ast.Node]ast.Node {
	padres := map[ast.Node]ast.Node{}
	var pila []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			pila = pila[:len(pila)-1]
			return false
		}
		if len(pila) > 0 {
			padres[n] = pila[len(pila)-1]
		}
		pila = append(pila, n)
		return true
	})
	return padres
}

func (o *optimizador) off(p token.Pos) int { return o.a.fset.Position(p).Offset }

func (o *optimizador) texto(n ast.Node) string {
	return o.a.texto[o.off(n.Pos()):o.off(n.End())]
}

// bucles returns the loops that contain n inside its function, innermost first. It stops at a function
// literal (a closure body is another function).
func (o *optimizador) bucles(n ast.Node) (out []ast.Stmt, enClosure bool) {
	for p := o.padres[n]; p != nil; p = o.padres[p] {
		switch x := p.(type) {
		case *ast.ForStmt:
			out = append(out, x)
		case *ast.RangeStmt:
			out = append(out, x)
		case *ast.FuncLit:
			return out, true
		case *ast.FuncDecl:
			return out, false
		}
	}
	return out, false
}

func contiene(n ast.Node, p token.Pos) bool { return n.Pos() <= p && p < n.End() }

// enLista reports whether the statement sits directly in a statement list (block or case clause), so
// that text can be inserted before it. A labeled statement counts through its label.
func (o *optimizador) enLista(s ast.Stmt) (ast.Stmt, bool) {
	var actual ast.Node = s
	if l, ok := o.padres[s].(*ast.LabeledStmt); ok {
		actual = l
	}
	switch o.padres[actual].(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return actual.(ast.Stmt), true
	}
	return nil, false
}

// usos returns the identifiers that use obj, in source order.
func (o *optimizador) usos(obj types.Object) []*ast.Ident {
	var out []*ast.Ident
	for id, u := range o.a.info.Uses {
		if u == obj {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pos() < out[j].Pos() })
	return out
}

// nombresUsados returns every identifier name in the file.
func (o *optimizador) nombresUsados() map[string]bool {
	out := map[string]bool{}
	ast.Inspect(o.a.f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out[id.Name] = true
		}
		return true
	})
	return out
}

func nombreLibre(base string, usados map[string]bool) string {
	n := base
	for i := 2; usados[n]; i++ {
		n = base + strconv.Itoa(i)
	}
	return n
}

func mayusculaInicial(s string) string {
	r, tam := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[tam:]
}

// importado returns how path is imported ("" when it is not) and whether the name is free of shadowing.
func (o *optimizador) importado(ruta string) (nombre string, bien bool) {
	for _, im := range o.a.f.Imports {
		if p, err := strconv.Unquote(im.Path.Value); err == nil && p == ruta {
			if im.Name != nil {
				return im.Name.Name, true
			}
			return ruta[strings.LastIndex(ruta, "/")+1:], true
		}
	}
	return "", true
}

// nombreSinUsar reports whether no identifier of the file named nombre refers to something other than
// the package nombre, so that inserting nombre.X is safe.
func (o *optimizador) nombreSinUsar(nombre string) bool {
	for id, obj := range o.a.info.Defs {
		if id.Name == nombre && obj != nil {
			return false
		}
	}
	for id, obj := range o.a.info.Uses {
		if id.Name == nombre {
			if _, ok := obj.(*types.PkgName); !ok {
				return false
			}
		}
	}
	return true
}

// ---- 1. text += in a loop → strings.Builder ----

func (o *optimizador) concatenaciones() []candidato {
	var out []candidato
	nombreStrings, _ := o.importado("strings")
	if nombreStrings != "" && nombreStrings != "strings" {
		return nil
	}
	if !o.nombreSinUsar("strings") {
		return nil
	}
	ids := make([]*ast.Ident, 0)
	for id, obj := range o.a.info.Defs {
		if v, ok := obj.(*types.Var); ok && !v.IsField() && types.Identical(v.Type(), types.Typ[types.String]) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		if c, ok := o.concatenacion(id); ok {
			out = append(out, c)
		}
	}
	return out
}

func (o *optimizador) concatenacion(id *ast.Ident) (candidato, bool) {
	obj := o.a.info.Defs[id]
	if obj.Parent() == nil || obj.Parent() == o.a.pkg.Scope() {
		return candidato{}, false
	}
	nombre := id.Name
	// the declaration: s := X  |  var s string  |  var s = X  |  var s string = X
	var decl ast.Stmt
	var inicial ast.Expr
	switch p := o.padres[id].(type) {
	case *ast.AssignStmt:
		if p.Tok != token.DEFINE || len(p.Lhs) != 1 || len(p.Rhs) != 1 {
			return candidato{}, false
		}
		decl, inicial = p, p.Rhs[0]
	case *ast.ValueSpec:
		gd, ok := o.padres[p].(*ast.GenDecl)
		if !ok || gd.Lparen.IsValid() || len(p.Names) != 1 || len(p.Values) > 1 {
			return candidato{}, false
		}
		ds, ok := o.padres[gd].(*ast.DeclStmt)
		if !ok {
			return candidato{}, false
		}
		decl = ds
		if len(p.Values) == 1 {
			inicial = p.Values[0]
		}
	default:
		return candidato{}, false // a parameter, a range variable…
	}
	if _, ok := o.enLista(decl); !ok {
		return candidato{}, false
	}
	usos := o.usos(obj)
	var sumas []*ast.AssignStmt
	var lecturas []*ast.Ident
	enBucle := false
	var primera *ast.AssignStmt
	for _, u := range usos {
		if _, closure := o.bucles(u); closure {
			return candidato{}, false
		}
		switch p := o.padres[u].(type) {
		case *ast.AssignStmt:
			esLhs := false
			for _, l := range p.Lhs {
				if l == u {
					esLhs = true
				}
			}
			if esLhs {
				if p.Tok != token.ADD_ASSIGN || len(p.Lhs) != 1 {
					return candidato{}, false
				}
				if usaObjeto(o.a, p.Rhs[0], obj) {
					return candidato{}, false
				}
				sumas = append(sumas, p)
				bs, _ := o.bucles(p)
				if len(bs) > 0 && !contiene(bs[0], decl.Pos()) {
					enBucle = true
					if primera == nil {
						primera = p
					}
				}
				continue
			}
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return candidato{}, false
			}
		case *ast.IncDecStmt:
			return candidato{}, false
		}
		lecturas = append(lecturas, u)
	}
	if !enBucle {
		return candidato{}, false
	}
	// a read inside a loop that also grows the text would call String() on every turn
	for _, l := range lecturas {
		bs, _ := o.bucles(l)
		for _, b := range bs {
			for _, s := range sumas {
				if contiene(b, s.Pos()) && !contiene(b, decl.Pos()) {
					return candidato{}, false
				}
			}
		}
	}
	var eds []edicion
	nuevo := "var " + nombre + " strings.Builder"
	if inicial != nil && !esTextoVacio(inicial) {
		nuevo += "; " + nombre + ".WriteString(" + o.texto(inicial) + ")"
	}
	eds = append(eds, edicion{o.off(decl.Pos()), o.off(decl.End()), nuevo})
	for _, s := range sumas {
		eds = append(eds, edicion{o.off(s.Pos()), o.off(s.End()), nombre + ".WriteString(" + o.texto(s.Rhs[0]) + ")"})
	}
	for _, l := range lecturas {
		eds = append(eds, edicion{o.off(l.Pos()), o.off(l.End()), nombre + ".String()"})
	}
	h := Hallazgo{
		Linea: o.a.linea(primera.Pos()), Clave: "concat_en_bucle", Gravedad: "consejo",
		Mensaje: "El texto `" + nombre + "` crece con += dentro de un bucle: en cada vuelta Go copia el texto entero.",
		Consejo: "Lo cambio por un `strings.Builder`: `var " + nombre + " strings.Builder`, `" + nombre + ".WriteString(…)` en el bucle y `" + nombre + ".String()` al leerlo.",
	}
	return candidato{h: h, eds: eds, importar: "strings"}, true
}

func esTextoVacio(e ast.Expr) bool {
	l, ok := e.(*ast.BasicLit)
	return ok && l.Kind == token.STRING && (l.Value == `""` || l.Value == "``")
}

func usaObjeto(a *archivo, n ast.Node, obj types.Object) bool {
	usa := false
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && a.info.Uses[id] == obj {
			usa = true
		}
		return !usa
	})
	return usa
}

// ---- 2. slices.Contains in a loop → a set ----

func (o *optimizador) contiene() []candidato {
	var llamadas []*ast.CallExpr
	ast.Inspect(o.a.f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && o.esSlicesContains(c) {
			llamadas = append(llamadas, c)
		}
		return true
	})
	usados := o.nombresUsados()
	hechos := map[ast.Stmt]map[types.Object]bool{}
	var out []candidato
	for _, c := range llamadas {
		xs, ok := c.Args[0].(*ast.Ident)
		if !ok {
			continue
		}
		obj, ok := o.a.info.Uses[xs].(*types.Var)
		if !ok {
			continue
		}
		sl, ok := obj.Type().Underlying().(*types.Slice)
		if !ok || !types.Comparable(sl.Elem()) {
			continue
		}
		bs, closure := o.bucles(c)
		if closure || len(bs) == 0 {
			continue
		}
		// the outermost loop that does not change xs and comes after its declaration
		var bucle, lugar ast.Stmt
		for i := len(bs) - 1; i >= 0; i-- {
			b := bs[i]
			if obj.Pos() >= b.Pos() || !o.soloLee(b, obj) {
				continue
			}
			if l, ok := o.enLista(b); ok {
				bucle, lugar = b, l
				break
			}
		}
		if bucle == nil || hechos[bucle][obj] {
			continue
		}
		if hechos[bucle] == nil {
			hechos[bucle] = map[types.Object]bool{}
		}
		hechos[bucle][obj] = true
		tipoElem := types.TypeString(sl.Elem(), types.RelativeTo(o.a.pkg))
		if strings.Contains(tipoElem, "/") {
			continue
		}
		conj := nombreLibre("conjunto"+mayusculaInicial(xs.Name), usados)
		usados[conj] = true
		v := "v"
		if xs.Name == "v" {
			v = "elem"
		}
		var eds []edicion
		eds = append(eds, edicion{o.off(lugar.Pos()), o.off(lugar.Pos()),
			conj + " := make(map[" + tipoElem + "]bool, len(" + xs.Name + "))\n" +
				"for _, " + v + " := range " + xs.Name + " {\n" + conj + "[" + v + "] = true\n}\n"})
		quitadas := 0
		for _, otra := range llamadas {
			if !contiene(bucle, otra.Pos()) {
				continue
			}
			if id, ok := otra.Args[0].(*ast.Ident); ok && o.a.info.Uses[id] == obj {
				eds = append(eds, edicion{o.off(otra.Pos()), o.off(otra.End()), conj + "[" + o.texto(otra.Args[1]) + "]"})
				quitadas++
			}
		}
		cand := candidato{
			h: Hallazgo{
				Linea: o.a.linea(c.Pos()), Clave: "contains_en_bucle", Gravedad: "consejo",
				Mensaje: "`slices.Contains(" + xs.Name + ", …)` recorre la lista entera en cada vuelta del bucle.",
				Consejo: "Antes del bucle guardo los elementos de `" + xs.Name + "` en un mapa (`" + conj + "`) y pregunto `" + conj + "[x]`: cada consulta es inmediata.",
			},
			eds: eds,
		}
		if o.usosPaquete("slices") == quitadas {
			cand.desimporta = "slices"
		}
		out = append(out, cand)
	}
	return out
}

func (o *optimizador) esSlicesContains(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Contains" || len(c.Args) != 2 || c.Ellipsis.IsValid() {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := o.a.info.Uses[id].(*types.PkgName)
	return ok && pn.Imported().Path() == "slices"
}

// usosPaquete counts the references to the package imported with path ruta.
func (o *optimizador) usosPaquete(ruta string) int {
	n := 0
	for _, obj := range o.a.info.Uses {
		if pn, ok := obj.(*types.PkgName); ok && pn.Imported().Path() == ruta {
			n++
		}
	}
	return n
}

// soloLee reports whether, inside n, obj is only read: as the list of slices.Contains, in len(…), as
// the range expression or indexed on the right-hand side.
func (o *optimizador) soloLee(n ast.Node, obj types.Object) bool {
	bien := true
	ast.Inspect(n, func(x ast.Node) bool {
		id, ok := x.(*ast.Ident)
		if !ok || o.a.info.Uses[id] != obj || !bien {
			return bien
		}
		switch p := o.padres[id].(type) {
		case *ast.CallExpr:
			if o.esSlicesContains(p) && p.Args[0] == id {
				return true
			}
			if f, ok := p.Fun.(*ast.Ident); ok && (f.Name == "len" || f.Name == "cap") && o.a.info.Uses[f] != nil && o.a.info.Uses[f].Parent() == types.Universe {
				return true
			}
		case *ast.RangeStmt:
			if p.X == id {
				return true
			}
		case *ast.IndexExpr:
			if p.X == id && !o.esDestino(p) {
				return true
			}
		}
		bien = false
		return false
	})
	return bien
}

// esDestino reports whether e is written to (left of an assignment, ++/--, or its address is taken).
func (o *optimizador) esDestino(e ast.Expr) bool {
	switch p := o.padres[e].(type) {
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if l == e {
				return true
			}
		}
	case *ast.IncDecStmt:
		return true
	case *ast.UnaryExpr:
		return p.Op == token.AND
	}
	return false
}

// ---- 3. append once per turn → make with capacity ----

func (o *optimizador) capacidades() []candidato {
	var out []candidato
	ast.Inspect(o.a.f, func(n ast.Node) bool {
		r, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if c, ok := o.capacidad(r); ok {
			out = append(out, c)
		}
		return true
	})
	return out
}

func (o *optimizador) capacidad(r *ast.RangeStmt) (candidato, bool) {
	switch o.a.tipo(r.X).(type) {
	case nil:
		return candidato{}, false
	}
	switch o.a.tipo(r.X).Underlying().(type) {
	case *types.Slice, *types.Map, *types.Array:
	default:
		return candidato{}, false
	}
	if !expresionSimple(r.X) {
		return candidato{}, false
	}
	lugar, ok := o.enLista(r)
	if !ok {
		return candidato{}, false
	}
	// out = append(out, e) at the top level of the body, exactly once
	var obj types.Object
	var ap *ast.AssignStmt
	for _, s := range r.Body.List {
		as, ok := s.(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			continue
		}
		id, ok := as.Lhs[0].(*ast.Ident)
		if !ok {
			continue
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || call.Ellipsis.IsValid() {
			continue
		}
		f, ok := call.Fun.(*ast.Ident)
		if !ok || f.Name != "append" || o.a.info.Uses[f] == nil || o.a.info.Uses[f].Parent() != types.Universe {
			continue
		}
		a0, ok := call.Args[0].(*ast.Ident)
		if !ok || o.a.info.Uses[a0] == nil || o.a.info.Uses[a0] != o.a.info.Uses[id] {
			continue
		}
		if obj != nil {
			return candidato{}, false // two appends: not one per turn
		}
		obj, ap = o.a.info.Uses[id], as
	}
	if obj == nil {
		return candidato{}, false
	}
	// no other write to out inside the loop
	for _, u := range o.usos(obj) {
		if contiene(r, u.Pos()) && o.esDestino(u) && u != ap.Lhs[0] {
			return candidato{}, false
		}
	}
	// the declaration: var out []T  |  out := []T{}  — in the same list, before the loop, unused in between
	declId := (*ast.Ident)(nil)
	for id, d := range o.a.info.Defs {
		if d == obj {
			declId = id
		}
	}
	if declId == nil {
		return candidato{}, false
	}
	var decl ast.Stmt
	var tipoTxt string
	switch p := o.padres[declId].(type) {
	case *ast.ValueSpec:
		gd, ok := o.padres[p].(*ast.GenDecl)
		if !ok || gd.Lparen.IsValid() || len(p.Names) != 1 || len(p.Values) != 0 || p.Type == nil {
			return candidato{}, false
		}
		ds, ok := o.padres[gd].(*ast.DeclStmt)
		if !ok {
			return candidato{}, false
		}
		decl, tipoTxt = ds, o.texto(p.Type)
	case *ast.AssignStmt:
		if p.Tok != token.DEFINE || len(p.Lhs) != 1 || len(p.Rhs) != 1 {
			return candidato{}, false
		}
		cl, ok := p.Rhs[0].(*ast.CompositeLit)
		if !ok || len(cl.Elts) != 0 || cl.Type == nil {
			return candidato{}, false
		}
		decl, tipoTxt = p, o.texto(cl.Type)
	default:
		return candidato{}, false
	}
	if _, ok := obj.Type().Underlying().(*types.Slice); !ok {
		return candidato{}, false
	}
	if o.padres[decl] != o.padres[lugar] || decl.Pos() >= lugar.Pos() {
		return candidato{}, false
	}
	for _, u := range o.usos(obj) {
		if u.Pos() > decl.End() && u.Pos() < lugar.Pos() {
			return candidato{}, false
		}
	}
	// the range expression must not change between the declaration and the loop: it is evaluated earlier now
	if !o.estableEntre(r.X, decl.End(), lugar.Pos()) {
		return candidato{}, false
	}
	nombre := declId.Name
	nuevo := nombre + " := make(" + tipoTxt + ", 0, len(" + o.texto(r.X) + "))"
	h := Hallazgo{
		Linea: o.a.linea(ap.Pos()), Clave: "append_sin_capacidad", Gravedad: "consejo",
		Mensaje: "La lista `" + nombre + "` crece con append en cada vuelta, y Go tiene que agrandarla y copiarla varias veces.",
		Consejo: "Como sé cuántos elementos tendrá, la creo con sitio para todos: `" + nuevo + "` (vacía será [] en lugar de nil).",
	}
	return candidato{h: h, eds: []edicion{{o.off(decl.Pos()), o.off(decl.End()), nuevo}}}, true
}

// expresionSimple is an identifier or a selector chain of identifiers (xs, p.Items).
func expresionSimple(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return expresionSimple(x.X)
	case *ast.ParenExpr:
		return expresionSimple(x.X)
	}
	return false
}

// estableEntre reports whether no identifier of e is assigned between desde and hasta.
func (o *optimizador) estableEntre(e ast.Expr, desde, hasta token.Pos) bool {
	objs := map[types.Object]bool{}
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if obj := o.a.info.Uses[id]; obj != nil {
				objs[obj] = true
			}
		}
		return true
	})
	for id, obj := range o.a.info.Uses {
		if objs[obj] && id.Pos() > desde && id.Pos() < hasta {
			if o.esDestino(id) {
				return false
			}
			if _, ok := o.padres[id].(*ast.SelectorExpr); ok {
				return false // p.Items = … would be a write through a selector; be careful
			}
		}
	}
	return true
}

// ---- applying a rewrite ----

func (o *optimizador) reescribir(c candidato) (string, bool) {
	eds := append([]edicion(nil), c.eds...)
	if c.importar != "" {
		if nombre, _ := o.importado(c.importar); nombre == "" {
			eds = append(eds, o.insertarImport(c.importar))
		}
	}
	if c.desimporta != "" {
		if e, ok := o.quitarImport(c.desimporta); ok {
			eds = append(eds, e)
		}
	}
	sort.SliceStable(eds, func(i, j int) bool { return eds[i].desde > eds[j].desde })
	src := o.a.texto
	limite := len(src) + 1
	for _, e := range eds {
		if e.hasta > limite || e.desde > e.hasta {
			return "", false // overlapping edits
		}
		src = src[:e.desde] + e.texto + src[e.hasta:]
		limite = e.desde
	}
	salida, err := format.Source([]byte(src))
	if err != nil {
		return "", false
	}
	b, err := parsear(string(salida))
	if err != nil || len(b.errores) > 0 {
		return "", false
	}
	b.comprobarTipos()
	if len(b.errores) > 0 {
		return "", false
	}
	return string(salida), true
}

func (o *optimizador) insertarImport(ruta string) edicion {
	f := o.a.f
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		if gd.Lparen.IsValid() {
			p := o.off(gd.Lparen) + 1
			return edicion{p, p, "\n" + strconv.Quote(ruta) + "\n"}
		}
		// import "fmt" → import ("fmt"; "strings")
		return edicion{o.off(gd.Pos()), o.off(gd.End()), "import (\n" + o.texto(gd.Specs[0]) + "\n" + strconv.Quote(ruta) + "\n)"}
	}
	p := o.off(f.Name.End())
	if p < len(o.a.texto) && o.a.texto[p] == ';' { // "package main;" added in front of a snippet
		p++
	}
	return edicion{p, p, "\n\nimport " + strconv.Quote(ruta) + "\n"}
}

func (o *optimizador) quitarImport(ruta string) (edicion, bool) {
	for _, d := range o.a.f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		for _, sp := range gd.Specs {
			is := sp.(*ast.ImportSpec)
			if p, err := strconv.Unquote(is.Path.Value); err != nil || p != ruta {
				continue
			}
			if !gd.Lparen.IsValid() {
				return edicion{o.off(gd.Pos()), o.off(gd.End()), ""}, true
			}
			return edicion{o.off(is.Pos()), o.off(is.End()), ""}, true
		}
	}
	return edicion{}, false
}
