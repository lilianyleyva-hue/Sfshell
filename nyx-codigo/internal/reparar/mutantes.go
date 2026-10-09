package reparar

import (
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// Mutation operators (the Stats key is "mutacion:<operador>").
const (
	opRelacional   = "relacional"   // < <= > >= == !=
	opAritmetico   = "aritmetico"   // + - * / %, += -= …, ++ --
	opLogico       = "logico"       // && ||
	opNegar        = "negar"        // !(cond)
	opConstante    = "constante"    // n±1 (0↔1)
	opInicioBucle  = "inicio_bucle" // for i := 0 ↔ 1, len(x) ↔ len(x)-1
	opLimiteLen    = "limite_len"   // len(x) ↔ len(x)-1
	opIndice       = "indice"       // x[i] ↔ x[i±1]
	opAsignacion   = "asignacion"   // = ↔ +=
	opVariable     = "variable"     // another in-scope variable of the same type
	opRamas        = "ramas"        // swap if/else
	opBorrar       = "borrar"       // delete a statement
	opIntercambiar = "intercambiar" // swap adjacent statements
	opAcumulador   = "acumulador"   // initial value of an accumulator
	opPanico       = "panico"       // a guard or fix keyed by a runtime panic (panicos.go)
)

// maxMutantesDefecto is the cap of Mutantes when max ≤ 0.
const maxMutantesDefecto = 400

// mutacion is a mutant plus the edits that produce it from the original source, so that two mutants
// can be combined (second-order repair).
type mutacion struct {
	Mutante
	eds   []edicion
	extra []Cambio // cleanups after the edit (an unused variable or import removed)
	linea int
	orden int
}

// Mutantes returns single-edit variants of fuente on the given lines (nil = the whole file), in every
// function including helpers, deduplicated by their gofmt'ed source and capped at max (400 when max ≤ 0).
// The operators are: relational swap, arithmetic swap, && ↔ ||, negated condition, constant ±1, loop
// start 0 ↔ 1, len(x) ↔ len(x)-1, index i ↔ i±1, = ↔ +=, another variable of the same type, swapped
// if/else branches, a deleted statement, two swapped statements and the initial value of an
// accumulator. Mutants are not type-checked here (logic repair drops the ones that do not compile).
func Mutantes(fuente string, lineas []int, max int) []Mutante {
	if max <= 0 {
		max = maxMutantesDefecto
	}
	a := analizar(fuente)
	if a.archivo == nil {
		return nil
	}
	ms := generarMutaciones(a, lineas)
	if len(ms) > max {
		ms = ms[:max]
	}
	out := make([]Mutante, len(ms))
	for i, m := range ms {
		out[i] = m.Mutante
	}
	return out
}

// generarMutaciones lists the mutants of a.fuente on lineas (nil = all), deduplicated, in source order.
func generarMutaciones(a *analisis, lineas []int) []mutacion {
	g := &generadorMut{a: a, vistos: map[string]bool{}}
	if lineas != nil {
		g.lineas = map[int]bool{}
		for _, l := range lineas {
			g.lineas[l] = true
		}
	}
	g.vistos[normalizarFuente(a.fuente)] = true
	g.izquierdas = identsIzquierda(a.archivo)
	g.acumuladores = buscarAcumuladores(a)
	var pila []ast.Node
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if n == nil {
			pila = pila[:len(pila)-1]
			return false
		}
		var padre ast.Node
		if len(pila) > 0 {
			padre = pila[len(pila)-1]
		}
		g.visitar(n, padre)
		pila = append(pila, n)
		return true
	})
	return g.out
}

type generadorMut struct {
	a            *analisis
	lineas       map[int]bool
	vistos       map[string]bool
	izquierdas   map[*ast.Ident]bool
	acumuladores map[types.Object]bool
	out          []mutacion
}

// normalizarFuente gofmt's src when it parses (the dedupe key and the mutant's text).
func normalizarFuente(src string) string {
	if b, err := format.Source([]byte(src)); err == nil {
		return string(b)
	}
	return src
}

func (g *generadorMut) en(n ast.Node) bool {
	if n == nil {
		return false
	}
	return g.lineas == nil || g.lineas[g.a.linea(n.Pos())]
}

// agregar records a mutant made of eds (edits on the original source).
func (g *generadorMut) agregar(op, regla string, n ast.Node, eds []edicion, porque string) {
	nuevo, _ := aplicar(g.a.fuente, eds)
	if nuevo == g.a.fuente {
		return
	}
	nuevo = normalizarFuente(nuevo)
	if g.vistos[nuevo] {
		return
	}
	g.vistos[nuevo] = true
	linea := g.a.linea(n.Pos())
	c := Cambio{Regla: regla, Linea: linea, Porque: porque}
	if len(eds) == 1 {
		c.Antes, c.Despues = eds[0].cambio.Antes, eds[0].cambio.Despues
	} else {
		c.Antes = compactar(g.a.texto(n))
		c.Despues = compactar(textoNodoTras(g.a, n, eds))
	}
	m := mutacion{Mutante: Mutante{Fuente: nuevo, Cambio: c, Operador: op}, eds: eds, linea: linea, orden: len(g.out)}
	g.out = append(g.out, m)
}

// textoNodoTras returns the text of node n after applying eds that fall inside it.
func textoNodoTras(a *analisis, n ast.Node, eds []edicion) string {
	ini, fin := a.off(n.Pos()), a.off(n.End())
	var dentro []edicion
	for _, e := range eds {
		if e.ini >= ini && e.fin <= fin {
			d := e
			d.ini -= ini
			d.fin -= ini
			dentro = append(dentro, d)
		}
	}
	s, _ := aplicar(a.fuente[ini:fin], dentro)
	return s
}

// compactar joins the lines of a fragment and caps it at 120 runes.
func compactar(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:119]) + "…"
	}
	return s
}

func (g *generadorMut) ed(ini, fin int, texto, porque string) edicion {
	return g.a.ed(ini, fin, texto, porque)
}

func (g *generadorMut) reemplazo(n ast.Node, texto, porque string) edicion {
	return g.a.reemplazar(n, texto, porque)
}

var nombresOp = map[token.Token]string{
	token.LSS: "menor", token.LEQ: "menor_igual", token.GTR: "mayor", token.GEQ: "mayor_igual",
	token.EQL: "igual", token.NEQ: "distinto",
	token.ADD: "suma", token.SUB: "resta", token.MUL: "producto", token.QUO: "division", token.REM: "resto",
	token.LAND: "y", token.LOR: "o",
	token.ADD_ASSIGN: "suma", token.SUB_ASSIGN: "resta", token.MUL_ASSIGN: "producto", token.QUO_ASSIGN: "division",
	token.REM_ASSIGN: "resto", token.INC: "incremento", token.DEC: "decremento", token.ASSIGN: "asignar",
}

// alternatives, the most common fix first
var (
	altRelacional = map[token.Token][]token.Token{
		token.LSS: {token.LEQ, token.GTR, token.GEQ, token.NEQ, token.EQL},
		token.LEQ: {token.LSS, token.GEQ, token.GTR, token.EQL, token.NEQ},
		token.GTR: {token.GEQ, token.LSS, token.LEQ, token.NEQ, token.EQL},
		token.GEQ: {token.GTR, token.LEQ, token.LSS, token.EQL, token.NEQ},
		token.EQL: {token.NEQ, token.LEQ, token.GEQ, token.LSS, token.GTR},
		token.NEQ: {token.EQL, token.LSS, token.GTR, token.LEQ, token.GEQ},
	}
	altAritmetico = map[token.Token][]token.Token{
		token.ADD: {token.SUB, token.MUL, token.QUO, token.REM},
		token.SUB: {token.ADD, token.MUL, token.QUO, token.REM},
		token.MUL: {token.ADD, token.QUO, token.SUB, token.REM},
		token.QUO: {token.MUL, token.REM, token.ADD, token.SUB},
		token.REM: {token.QUO, token.MUL, token.ADD, token.SUB},
	}
	altAsignacionOp = map[token.Token][]token.Token{
		token.ADD_ASSIGN: {token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN},
		token.SUB_ASSIGN: {token.ADD_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN},
		token.MUL_ASSIGN: {token.ADD_ASSIGN, token.QUO_ASSIGN, token.SUB_ASSIGN, token.REM_ASSIGN},
		token.QUO_ASSIGN: {token.MUL_ASSIGN, token.REM_ASSIGN, token.ADD_ASSIGN, token.SUB_ASSIGN},
		token.REM_ASSIGN: {token.QUO_ASSIGN, token.MUL_ASSIGN, token.ADD_ASSIGN, token.SUB_ASSIGN},
	}
)

// cambiarOp replaces the operator token at pos.
func (g *generadorMut) cambiarOp(n ast.Node, pos token.Pos, viejo, nuevo token.Token, op string) {
	o := g.a.off(pos)
	e := g.ed(o, o+len(viejo.String()), nuevo.String(), "")
	porque := "cambio «" + viejo.String() + "» por «" + nuevo.String() + "»"
	e.cambio.Porque = porque
	g.agregar(op, nombresOp[viejo]+"_a_"+nombresOp[nuevo], n, []edicion{e}, porque)
}

func (g *generadorMut) visitar(n, padre ast.Node) {
	a := g.a
	switch x := n.(type) {
	case *ast.BlockStmt:
		g.bloque(x)
	case *ast.AssignStmt:
		if !g.en(x) {
			return
		}
		g.acumulador(x)
		if x.Tok == token.ASSIGN && len(x.Lhs) == 1 && len(x.Rhs) == 1 {
			g.cambiarOp(x, x.TokPos, token.ASSIGN, token.ADD_ASSIGN, opAsignacion)
		}
		if alts, ok := altAsignacionOp[x.Tok]; ok {
			if x.Tok == token.ADD_ASSIGN {
				g.cambiarOp(x, x.TokPos, token.ADD_ASSIGN, token.ASSIGN, opAsignacion)
			}
			for _, alt := range alts {
				g.cambiarOp(x, x.TokPos, x.Tok, alt, opAritmetico)
			}
		}
	case *ast.DeclStmt:
		if g.en(x) {
			g.acumulador(x)
		}
	case *ast.IncDecStmt:
		if g.en(x) {
			nuevo := token.DEC
			if x.Tok == token.DEC {
				nuevo = token.INC
			}
			g.cambiarOp(x, x.TokPos, x.Tok, nuevo, opAritmetico)
		}
	case *ast.ForStmt:
		if g.en(x) {
			g.inicioBucle(x)
			if x.Cond != nil {
				g.negar(x, x.Cond)
			}
		}
	case *ast.IfStmt:
		if g.en(x) {
			g.negar(x, x.Cond)
			if el, ok := x.Else.(*ast.BlockStmt); ok {
				ib, fb := a.off(x.Body.Lbrace)+1, a.off(x.Body.Rbrace)
				ie, fe := a.off(el.Lbrace)+1, a.off(el.Rbrace)
				cuerpo, otro := a.fuente[ib:fb], a.fuente[ie:fe]
				eds := []edicion{{ini: ib, fin: fb, texto: otro}, {ini: ie, fin: fe, texto: cuerpo}}
				g.agregar(opRamas, "cambiar_ramas", x, eds, "intercambio lo que hace el «if» y lo que hace el «else»")
			}
		}
	case *ast.BinaryExpr:
		if !g.en(x) {
			return
		}
		for _, alt := range altRelacional[x.Op] {
			g.cambiarOp(x, x.OpPos, x.Op, alt, opRelacional)
		}
		for _, alt := range altAritmetico[x.Op] {
			g.cambiarOp(x, x.OpPos, x.Op, alt, opAritmetico)
		}
		switch x.Op {
		case token.LAND:
			g.cambiarOp(x, x.OpPos, token.LAND, token.LOR, opLogico)
		case token.LOR:
			g.cambiarOp(x, x.OpPos, token.LOR, token.LAND, opLogico)
		}
		if x.Op == token.SUB && esLen(a, x.X) && esLiteral(x.Y, "1") {
			t := a.texto(x.X)
			g.agregar(opLimiteLen, "quitar_menos_uno", x, []edicion{g.reemplazo(x, t, "uso «"+t+"» en vez de «"+t+"-1»")}, "uso «"+t+"» en vez de «"+t+"-1»")
		}
	case *ast.CallExpr:
		if g.en(x) && esLen(a, x) {
			if b, ok := padre.(*ast.BinaryExpr); ok && b.X == ast.Expr(x) && b.Op == token.SUB && esLiteral(b.Y, "1") {
				return
			}
			t := a.texto(x)
			nuevo := t + "-1"
			if necesitaParentesis(padre, x) {
				nuevo = "(" + nuevo + ")"
			}
			porque := "uso «" + t + "-1» en vez de «" + t + "»"
			g.agregar(opLimiteLen, "menos_uno", x, []edicion{g.reemplazo(x, nuevo, porque)}, porque)
		}
	case *ast.IndexExpr:
		if g.en(x) {
			g.indice(x)
		}
	case *ast.BasicLit:
		if g.en(x) && x.Kind == token.INT {
			g.constante(x)
		}
	case *ast.Ident:
		if g.en(x) {
			g.variable(x)
		}
	}
}

// esLen reports whether e is a call to the builtin len.
func esLen(a *analisis, e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != "len" {
		return false
	}
	if a.info != nil {
		if o := a.info.Uses[id]; o != nil && o.Parent() != types.Universe {
			return false
		}
	}
	return true
}

func esLiteral(e ast.Expr, v string) bool {
	b, ok := e.(*ast.BasicLit)
	return ok && b.Value == v
}

// necesitaParentesis reports whether "x-1" must be parenthesized where x sits.
func necesitaParentesis(padre ast.Node, x ast.Expr) bool {
	switch p := padre.(type) {
	case *ast.BinaryExpr:
		if p.Op.Precedence() > token.SUB.Precedence() {
			return true
		}
		return p.Op == token.SUB && p.Y == x
	case *ast.UnaryExpr, *ast.StarExpr:
		return true
	case *ast.SelectorExpr:
		return true
	}
	return false
}

func (g *generadorMut) negar(n ast.Node, cond ast.Expr) {
	a := g.a
	var nuevo string
	if u, ok := cond.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		x := u.X
		if p, ok := x.(*ast.ParenExpr); ok {
			x = p.X
		}
		nuevo = a.texto(x)
	} else {
		switch cond.(type) {
		case *ast.Ident, *ast.CallExpr, *ast.ParenExpr, *ast.SelectorExpr, *ast.IndexExpr:
			nuevo = "!" + a.texto(cond)
		default:
			nuevo = "!(" + a.texto(cond) + ")"
		}
	}
	porque := "niego la condición: «" + nuevo + "»"
	g.agregar(opNegar, "negar_condicion", n, []edicion{g.reemplazo(cond, nuevo, porque)}, porque)
}

func (g *generadorMut) inicioBucle(f *ast.ForStmt) {
	a := g.a
	as, ok := f.Init.(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return
	}
	rhs := as.Rhs[0]
	var nuevo string
	switch {
	case esLiteral(rhs, "0"):
		nuevo = "1"
	case esLiteral(rhs, "1"):
		nuevo = "0"
	case esLen(a, rhs):
		nuevo = a.texto(rhs) + "-1"
	default:
		if b, ok := rhs.(*ast.BinaryExpr); ok && b.Op == token.SUB && esLen(a, b.X) && esLiteral(b.Y, "1") {
			nuevo = a.texto(b.X)
		}
	}
	if nuevo == "" {
		return
	}
	porque := "el bucle empieza en «" + nuevo + "» en vez de «" + a.texto(rhs) + "»"
	g.agregar(opInicioBucle, "inicio_bucle", f, []edicion{g.reemplazo(rhs, nuevo, porque)}, porque)
}

func (g *generadorMut) constante(b *ast.BasicLit) {
	v, err := strconv.ParseInt(b.Value, 0, 64)
	if err != nil {
		return
	}
	for _, d := range []int64{1, -1} {
		nuevo := strconv.FormatInt(v+d, 10)
		porque := "cambio el número " + b.Value + " por " + nuevo
		regla := "mas_uno"
		if d < 0 {
			regla = "menos_uno"
		}
		g.agregar(opConstante, regla, b, []edicion{g.reemplazo(b, nuevo, porque)}, porque)
	}
}

func (g *generadorMut) indice(x *ast.IndexExpr) {
	a := g.a
	if _, ok := x.Index.(*ast.BasicLit); ok {
		return
	}
	if t := a.tipoDe(x.X); t != nil {
		switch u := t.Underlying().(type) {
		case *types.Map, *types.Signature:
			return
		case *types.Pointer:
			if _, ok := u.Elem().Underlying().(*types.Array); !ok {
				return
			}
		}
	}
	if a.info != nil {
		if tv, ok := a.info.Types[x.Index]; ok && tv.Value != nil {
			return // constant index
		}
	}
	idx := a.texto(x.Index)
	if b, ok := x.Index.(*ast.BinaryExpr); ok && (b.Op == token.ADD || b.Op == token.SUB) && esLiteral(b.Y, "1") {
		nuevo := a.texto(b.X)
		porque := "uso la posición «" + nuevo + "» en vez de «" + idx + "»"
		g.agregar(opIndice, "indice_exacto", x, []edicion{g.reemplazo(x.Index, nuevo, porque)}, porque)
		return
	}
	for _, s := range []string{"+1", "-1"} {
		nuevo := idx + s
		porque := "uso la posición «" + nuevo + "» en vez de «" + idx + "»"
		regla := "indice_mas_uno"
		if s == "-1" {
			regla = "indice_menos_uno"
		}
		g.agregar(opIndice, regla, x, []edicion{g.reemplazo(x.Index, nuevo, porque)}, porque)
	}
}

// identsIzquierda collects the identifiers that are written, not read (left of =, :=, ++, range vars).
func identsIzquierda(f *ast.File) map[*ast.Ident]bool {
	out := map[*ast.Ident]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					out[id] = true
				}
			}
		case *ast.IncDecStmt:
			if id, ok := x.X.(*ast.Ident); ok {
				out[id] = true
			}
		case *ast.RangeStmt:
			if id, ok := x.Key.(*ast.Ident); ok {
				out[id] = true
			}
			if id, ok := x.Value.(*ast.Ident); ok {
				out[id] = true
			}
		case *ast.SelectorExpr:
			out[x.Sel] = true
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok {
				out[id] = true // a field name in a struct literal (a map key ident is rare)
			}
		}
		return true
	})
	return out
}

// esLocal reports whether o is a variable declared inside a function (params included).
func (a *analisis) esLocal(o types.Object) bool {
	v, ok := o.(*types.Var)
	if !ok || v.IsField() || a.paquete == nil {
		return false
	}
	return v.Parent() != nil && v.Parent() != a.paquete.Scope() && v.Parent() != types.Universe
}

// maxVariables caps the replacements of one identifier.
const maxVariables = 5

func (g *generadorMut) variable(id *ast.Ident) {
	a := g.a
	if a.info == nil || g.izquierdas[id] || id.Name == "_" {
		return
	}
	o := a.info.Uses[id]
	if o == nil || !a.esLocal(o) {
		return
	}
	n := 0
	visto := map[string]bool{id.Name: true}
	for s := a.paquete.Scope().Innermost(id.Pos()); s != nil && s != a.paquete.Scope() && n < maxVariables; s = s.Parent() {
		for _, nombre := range s.Names() {
			if visto[nombre] || n >= maxVariables {
				continue
			}
			otra := s.Lookup(nombre)
			if !a.esLocal(otra) || otra.Pos() >= id.Pos() || !types.Identical(otra.Type(), o.Type()) {
				continue
			}
			visto[nombre] = true
			n++
			porque := "uso «" + nombre + "» en vez de «" + id.Name + "»"
			g.agregar(opVariable, "otra_variable", id, []edicion{g.reemplazo(id, nombre, porque)}, porque)
		}
	}
}

// bloque generates the statement-level mutants: deletions and swaps of adjacent statements.
func (g *generadorMut) bloque(b *ast.BlockStmt) {
	a := g.a
	for i, s := range b.List {
		if g.en(s) {
			switch x := s.(type) {
			case *ast.ExprStmt, *ast.IncDecStmt, *ast.BranchStmt:
				g.borrarSentencia(s)
			case *ast.AssignStmt:
				if x.Tok != token.DEFINE {
					g.borrarSentencia(s)
				}
			}
		}
		if i+1 < len(b.List) && (g.en(s) || g.en(b.List[i+1])) {
			t := b.List[i+1]
			if _, ok := t.(*ast.ReturnStmt); ok {
				continue // moving a statement after a return only makes it unreachable
			}
			ii, fi := a.off(s.Pos()), a.off(s.End())
			it, ft := a.off(t.Pos()), a.off(t.End())
			eds := []edicion{{ini: ii, fin: fi, texto: a.fuente[it:ft]}, {ini: it, fin: ft, texto: a.fuente[ii:fi]}}
			porque := "cambio el orden de «" + compactar(a.fuente[ii:fi]) + "» y «" + compactar(a.fuente[it:ft]) + "»"
			antes := len(g.out)
			g.agregar(opIntercambiar, "intercambiar_sentencias", s, eds, porque)
			if len(g.out) > antes {
				g.out[len(g.out)-1].Cambio.Antes = compactar(a.fuente[ii:ft])
				nuevo, _ := aplicar(a.fuente[ii:ft], []edicion{{ini: 0, fin: fi - ii, texto: a.fuente[it:ft]}, {ini: it - ii, fin: ft - ii, texto: a.fuente[ii:fi]}})
				g.out[len(g.out)-1].Cambio.Despues = compactar(nuevo)
			}
		}
	}
}

func (g *generadorMut) borrarSentencia(s ast.Stmt) {
	porque := "quito «" + compactar(g.a.texto(s)) + "»"
	e := g.a.borrar(s, porque)
	g.agregar(opBorrar, "borrar_sentencia", s, []edicion{e}, porque)
}

// buscarAcumuladores returns the local variables that are changed inside a loop body.
func buscarAcumuladores(a *analisis) map[types.Object]bool {
	out := map[types.Object]bool{}
	if a.info == nil {
		return out
	}
	marcar := func(e ast.Expr) {
		if id, ok := e.(*ast.Ident); ok {
			if o := a.info.Uses[id]; o != nil && a.esLocal(o) {
				out[o] = true
			}
		}
	}
	var enBucle func(n ast.Node) bool
	enBucle = func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				for _, l := range x.Lhs {
					marcar(l)
				}
			}
		case *ast.IncDecStmt:
			marcar(x.X)
		}
		return true
	}
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ForStmt:
			ast.Inspect(x.Body, enBucle)
		case *ast.RangeStmt:
			ast.Inspect(x.Body, enBucle)
		}
		return true
	})
	return out
}

// acumulador changes the initial value of an accumulator: 0 ↔ 1, 0 ↔ xs[0], math.MinInt / math.MaxInt.
func (g *generadorMut) acumulador(s ast.Stmt) {
	a := g.a
	if a.info == nil {
		return
	}
	var id *ast.Ident
	var valor ast.Expr
	switch x := s.(type) {
	case *ast.AssignStmt:
		if len(x.Lhs) != 1 || len(x.Rhs) != 1 || (x.Tok != token.DEFINE && x.Tok != token.ASSIGN) {
			return
		}
		id, _ = x.Lhs[0].(*ast.Ident)
		valor = x.Rhs[0]
	case *ast.DeclStmt:
		gd, ok := x.Decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR || len(gd.Specs) != 1 {
			return
		}
		vs := gd.Specs[0].(*ast.ValueSpec)
		if len(vs.Names) != 1 || len(vs.Values) != 1 {
			return
		}
		id, valor = vs.Names[0], vs.Values[0]
	}
	if id == nil {
		return
	}
	o := a.info.Defs[id]
	if o == nil {
		o = a.info.Uses[id]
	}
	if o == nil || !g.acumuladores[o] {
		return
	}
	t := o.Type()
	if !esNumero(t) {
		return
	}
	viejo := a.texto(valor)
	var alts []string
	var importar []string
	esInt := false
	if b := basico(t); b != nil && b.Kind() == types.Int {
		esInt = true
	}
	switch {
	case viejo == "0" || viejo == "0.0":
		alts = append(alts, "1")
		if esInt {
			alts = append(alts, g.primeros(id.Pos(), t)...)
			alts = append(alts, "math.MinInt", "math.MaxInt")
		}
	case viejo == "1" || viejo == "1.0":
		alts = append(alts, "0")
	case viejo == "math.MinInt" || viejo == "math.MaxInt":
		alts = append(alts, "0")
		if viejo == "math.MinInt" {
			alts = append(alts, "math.MaxInt")
		} else {
			alts = append(alts, "math.MinInt")
		}
	default:
		if ix, ok := valor.(*ast.IndexExpr); ok && esLiteral(ix.Index, "0") {
			alts = append(alts, "0")
			if esInt {
				alts = append(alts, "math.MinInt", "math.MaxInt")
			}
		}
	}
	for _, alt := range alts {
		importar = nil
		if strings.HasPrefix(alt, "math.") {
			importar = []string{"math"}
		}
		porque := "«" + id.Name + "» empieza en «" + alt + "» en vez de «" + viejo + "»"
		e := g.reemplazo(valor, alt, porque)
		e.importar = importar
		g.agregar(opAcumulador, "acumulador_inicial", s, []edicion{e}, porque)
	}
}

// primeros returns "xs[0]" for the slices in scope at pos whose element type is t.
func (g *generadorMut) primeros(pos token.Pos, t types.Type) []string {
	a := g.a
	var out []string
	for s := a.paquete.Scope().Innermost(pos); s != nil && s != a.paquete.Scope(); s = s.Parent() {
		for _, nombre := range s.Names() {
			o := s.Lookup(nombre)
			if !a.esLocal(o) || o.Pos() >= pos {
				continue
			}
			if sl, ok := o.Type().Underlying().(*types.Slice); ok && types.Identical(sl.Elem(), t) {
				out = append(out, nombre+"[0]")
			}
		}
	}
	return out
}
