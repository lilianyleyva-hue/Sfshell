// Package instrumenta does pure source-to-source rewriting of one Go file: fuel and depth checks,
// coverage counters, variable-trace calls, and renaming of top-level identifiers so that several
// variants can live in one package. It never runs anything.
//
// Every rewrite here inserts text at byte offsets of the original file and never re-prints the
// syntax tree, so each line of the result is the same line of the original: compiler errors and
// coverage ids map straight back to what the user wrote.
package instrumenta

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Prefijo starts every identifier we generate. Candidate code may not use it (in any case).
const Prefijo = "nyx__"

// Bounds (§4.1).
const (
	MaxBytes      = 64 << 10
	MaxSentencias = 5000
)

// Resultado is an instrumented file.
type Resultado struct {
	Fuente string // instrumented file (same package clause)
	Lineas []int  // statement id → original line (coverage ids)
}

// Errors returned by Instrumentar.
var (
	ErrPrefijo    = errors.New("instrumenta: el código ya usa el prefijo reservado nyx__")
	ErrGrande     = errors.New("instrumenta: el archivo es demasiado grande (más de 64 KiB)")
	ErrSentencias = errors.New("instrumenta: el archivo tiene demasiadas sentencias (más de 5000)")
)

// Texts inserted. They are on one line so line numbers never move.
const (
	textoCombustible = "nyx__f--; if nyx__f < 0 { panic(nyx__SinComb{}) }; "
	textoPila        = "if nyx__p++; nyx__p > nyx__maxPila { panic(nyx__Hondo{}) }; defer func() { nyx__p-- }(); "
)

// orden of insertions that share an offset.
const (
	ordenEntrada   = 0 // function entry / loop body start / clause start
	ordenCobertura = 1
	ordenDespues   = 2 // events after a statement
)

type insercion struct {
	off   int
	orden int
	seq   int
	texto string
}

type instrumentador struct {
	op         nucleo.OpcionesInstr
	fset       *token.FileSet
	tf         *token.File
	ins        []insercion
	lineas     []int
	recursivas map[*ast.FuncDecl]bool
	globales   map[string]bool
	err        error
}

// Instrumentar rewrites one file. Fuel: "nyx__f--; if nyx__f < 0 { panic(nyx__SinComb{}) }" at every function
// entry, loop body start and labeled statement. Depth: "if nyx__p++; nyx__p > MaxPila { panic(nyx__Hondo{}) };
// defer func(){ nyx__p-- }()" only in functions that belong to a call-graph cycle (self or mutual recursion,
// computed on top-level FuncDecls) and in every FuncLit assigned to a variable of func type. Coverage:
// "nyx__c[ID]++" before each statement in a block (not in for-post / if-init). Variables: after each assignment,
// inc/dec and range header, "nyx__ev(LINE, \"x\", x)" for each non-blank local identifier assigned
// (≤ MaxEventos recorded at runtime). Returns an error if the source already uses Prefijo (case-insensitive).
//
// The limit MaxPila is read from the variable nyx__maxPila of the support file, so the harness can change it.
func Instrumentar(fuente string, op nucleo.OpcionesInstr) (Resultado, error) {
	if len(fuente) > MaxBytes {
		return Resultado{}, ErrGrande
	}
	if UsaPrefijo(fuente) {
		return Resultado{}, ErrPrefijo
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return Resultado{}, fmt.Errorf("instrumenta: el código no se puede leer: %w", err)
	}
	in := &instrumentador{
		op:       op,
		fset:     fset,
		tf:       fset.File(f.Pos()),
		globales: nombresGlobales(f),
	}
	in.recursivas = funcionesRecursivas(f)
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			in.recorrerExpr(d.Type)
			in.funcion(d.Body, op.MaxPila > 0 && in.recursivas[d])
		case *ast.GenDecl:
			for _, s := range d.Specs {
				if vs, ok := s.(*ast.ValueSpec); ok {
					for _, v := range vs.Values {
						in.exprAsignada(v)
					}
				}
			}
		}
		if in.err != nil {
			return Resultado{}, in.err
		}
	}
	return Resultado{Fuente: aplicar(fuente, in.ins), Lineas: in.lineas}, nil
}

func (in *instrumentador) off(p token.Pos) int { return in.tf.Offset(p) }

func (in *instrumentador) linea(p token.Pos) int { return in.tf.Line(p) }

func (in *instrumentador) insertar(p token.Pos, orden int, texto string) {
	if texto == "" {
		return
	}
	in.ins = append(in.ins, insercion{off: in.off(p), orden: orden, seq: len(in.ins), texto: texto})
}

// funcion instruments a function body: entry fuel, optional depth check, then its statements.
func (in *instrumentador) funcion(body *ast.BlockStmt, pila bool) {
	var sb strings.Builder
	sb.WriteByte(' ')
	if pila {
		sb.WriteString(textoPila)
	}
	if in.op.Combustible > 0 {
		sb.WriteString(textoCombustible)
	}
	if sb.Len() > 1 {
		in.insertar(body.Lbrace+1, ordenEntrada, strings.TrimRight(sb.String(), " "))
	}
	in.lista(body.List)
}

// bloqueInicio inserts fuel (for loops) and variable events at the start of a block.
func (in *instrumentador) bloqueInicio(lbrace token.Pos, combustible bool, eventos string) {
	var sb strings.Builder
	if combustible && in.op.Combustible > 0 {
		sb.WriteString(textoCombustible)
	}
	sb.WriteString(eventos)
	if sb.Len() > 0 {
		in.insertar(lbrace+1, ordenEntrada, " "+strings.TrimRight(sb.String(), " "))
	}
}

// lista instruments a statement list (block, case clause or comm clause body).
func (in *instrumentador) lista(ss []ast.Stmt) {
	for _, s := range ss {
		if in.err != nil {
			return
		}
		if len(in.lineas) >= MaxSentencias {
			in.err = ErrSentencias
			return
		}
		id := len(in.lineas)
		in.lineas = append(in.lineas, in.linea(s.Pos()))
		if in.op.Cobertura {
			in.insertar(s.Pos(), ordenCobertura, "nyx__c["+strconv.Itoa(id)+"]++; ")
		}
		in.sentencia(s)
	}
}

// eventos returns the variable-trace calls for the identifiers in xs (empty when Variables is off).
func (in *instrumentador) eventos(linea int, xs []ast.Expr) string {
	if !in.op.Variables {
		return ""
	}
	var sb strings.Builder
	visto := map[string]bool{}
	for _, x := range xs {
		id, ok := x.(*ast.Ident)
		if !ok || id.Name == "_" || in.globales[id.Name] || visto[id.Name] {
			continue
		}
		visto[id.Name] = true
		fmt.Fprintf(&sb, "nyx__ev(%d, %q, %s); ", linea, id.Name, id.Name)
	}
	return sb.String()
}

func identsComoExpr(ids []*ast.Ident) []ast.Expr {
	out := make([]ast.Expr, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

// eventosDespues inserts events right after statement s.
func (in *instrumentador) eventosDespues(s ast.Stmt, xs []ast.Expr) {
	ev := in.eventos(in.linea(s.Pos()), xs)
	if ev == "" {
		return
	}
	in.insertar(s.End(), ordenDespues, "; "+strings.TrimSuffix(ev, "; "))
}

// sinFunciones drops the left-hand sides that receive a function literal (tracing a func value is noise).
func sinFunciones(lhs, rhs []ast.Expr) []ast.Expr {
	if len(lhs) != len(rhs) {
		return lhs
	}
	out := make([]ast.Expr, 0, len(lhs))
	for i, x := range lhs {
		if _, ok := ast.Unparen(rhs[i]).(*ast.FuncLit); ok {
			continue
		}
		out = append(out, x)
	}
	return out
}

// cabecera collects the identifiers assigned by a header statement (if/for/switch init, for post).
func cabecera(s ast.Stmt) []ast.Expr {
	switch s := s.(type) {
	case *ast.AssignStmt:
		return s.Lhs
	case *ast.IncDecStmt:
		return []ast.Expr{s.X}
	case *ast.DeclStmt:
		if gd, ok := s.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			var out []ast.Expr
			for _, sp := range gd.Specs {
				if vs, ok := sp.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
					out = append(out, identsComoExpr(vs.Names)...)
				}
			}
			return out
		}
	}
	return nil
}

// sentencia instruments the inside of one statement (its own coverage counter is already placed).
func (in *instrumentador) sentencia(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.BlockStmt:
		in.lista(s.List)
	case *ast.LabeledStmt:
		switch s.Stmt.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			// the label must stay on the statement (break/continue L); loops get fuel in their body
		default:
			if in.op.Combustible > 0 {
				in.insertar(s.Stmt.Pos(), ordenEntrada, textoCombustible)
			}
		}
		in.sentencia(s.Stmt)
	case *ast.ExprStmt:
		in.recorrerExpr(s.X)
	case *ast.SendStmt:
		in.recorrerExpr(s.Chan)
		in.recorrerExpr(s.Value)
	case *ast.IncDecStmt:
		in.recorrerExpr(s.X)
		in.eventosDespues(s, []ast.Expr{s.X})
	case *ast.AssignStmt:
		for _, x := range s.Lhs {
			in.recorrerExpr(x)
		}
		for _, x := range s.Rhs {
			in.exprAsignada(x)
		}
		in.eventosDespues(s, sinFunciones(s.Lhs, s.Rhs))
	case *ast.DeclStmt:
		gd, ok := s.Decl.(*ast.GenDecl)
		if !ok {
			return
		}
		for _, sp := range gd.Specs {
			switch sp := sp.(type) {
			case *ast.ValueSpec:
				for _, v := range sp.Values {
					in.exprAsignada(v)
				}
			case *ast.TypeSpec:
				in.recorrerExpr(sp.Type)
			}
		}
		if gd.Tok == token.VAR {
			in.eventosDespues(s, cabecera(s))
		}
	case *ast.GoStmt:
		in.recorrerExpr(s.Call)
	case *ast.DeferStmt:
		in.recorrerExpr(s.Call)
	case *ast.ReturnStmt:
		for _, x := range s.Results {
			in.recorrerExpr(x)
		}
	case *ast.IfStmt:
		var ev string
		if s.Init != nil {
			in.cabeceraExpr(s.Init)
			ev = in.eventos(in.linea(s.Pos()), cabecera(s.Init))
		}
		in.recorrerExpr(s.Cond)
		in.bloqueInicio(s.Body.Lbrace, false, ev)
		in.lista(s.Body.List)
		switch e := s.Else.(type) {
		case *ast.BlockStmt:
			in.bloqueInicio(e.Lbrace, false, ev)
			in.lista(e.List)
		case *ast.IfStmt:
			in.sentencia(e)
		}
	case *ast.ForStmt:
		var cab []ast.Expr
		if s.Init != nil {
			in.cabeceraExpr(s.Init)
			cab = append(cab, cabecera(s.Init)...)
		}
		if s.Cond != nil {
			in.recorrerExpr(s.Cond)
		}
		if s.Post != nil {
			in.cabeceraExpr(s.Post)
			cab = append(cab, cabecera(s.Post)...)
		}
		in.bloqueInicio(s.Body.Lbrace, true, in.eventos(in.linea(s.Pos()), cab))
		in.lista(s.Body.List)
	case *ast.RangeStmt:
		in.recorrerExpr(s.X)
		var cab []ast.Expr
		if s.Tok != token.ILLEGAL {
			if s.Key != nil {
				cab = append(cab, s.Key)
			}
			if s.Value != nil {
				cab = append(cab, s.Value)
			}
		}
		in.bloqueInicio(s.Body.Lbrace, true, in.eventos(in.linea(s.Pos()), cab))
		in.lista(s.Body.List)
	case *ast.SwitchStmt:
		var ev string
		if s.Init != nil {
			in.cabeceraExpr(s.Init)
			ev = in.eventos(in.linea(s.Pos()), cabecera(s.Init))
		}
		if s.Tag != nil {
			in.recorrerExpr(s.Tag)
		}
		for _, c := range s.Body.List {
			cc := c.(*ast.CaseClause)
			for _, x := range cc.List {
				in.recorrerExpr(x)
			}
			if ev != "" {
				in.insertar(cc.Colon+1, ordenEntrada, " "+strings.TrimRight(ev, " "))
			}
			in.lista(cc.Body)
		}
	case *ast.TypeSwitchStmt:
		if s.Init != nil {
			in.cabeceraExpr(s.Init)
		}
		in.cabeceraExpr(s.Assign)
		for _, c := range s.Body.List {
			cc := c.(*ast.CaseClause)
			in.lista(cc.Body)
		}
	case *ast.SelectStmt:
		for _, c := range s.Body.List {
			cc := c.(*ast.CommClause)
			if cc.Comm != nil {
				in.cabeceraExpr(cc.Comm)
				if a, ok := cc.Comm.(*ast.AssignStmt); ok {
					if ev := in.eventos(in.linea(cc.Comm.Pos()), a.Lhs); ev != "" {
						in.insertar(cc.Colon+1, ordenEntrada, " "+strings.TrimRight(ev, " "))
					}
				}
			}
			in.lista(cc.Body)
		}
	}
}

// cabeceraExpr walks a header statement (no coverage, no events of its own) looking for FuncLits.
func (in *instrumentador) cabeceraExpr(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.AssignStmt:
		for _, x := range s.Lhs {
			in.recorrerExpr(x)
		}
		for _, x := range s.Rhs {
			in.exprAsignada(x)
		}
	case *ast.ExprStmt:
		in.recorrerExpr(s.X)
	case *ast.IncDecStmt:
		in.recorrerExpr(s.X)
	case *ast.SendStmt:
		in.recorrerExpr(s.Chan)
		in.recorrerExpr(s.Value)
	case *ast.DeclStmt:
		in.sentencia(s)
	}
}

// exprAsignada walks an expression on the right of an assignment: a FuncLit there gets the depth check.
func (in *instrumentador) exprAsignada(x ast.Expr) {
	if fl, ok := ast.Unparen(x).(*ast.FuncLit); ok {
		in.recorrerExpr(fl.Type)
		in.funcion(fl.Body, in.op.MaxPila > 0)
		return
	}
	in.recorrerExpr(x)
}

// recorrerExpr finds FuncLits inside an expression and instruments their bodies.
func (in *instrumentador) recorrerExpr(x ast.Node) {
	if x == nil {
		return
	}
	ast.Inspect(x, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			in.funcion(fl.Body, false)
			return false
		}
		return true
	})
}

// aplicar splices the insertions into the source.
func aplicar(fuente string, ins []insercion) string {
	sort.SliceStable(ins, func(i, j int) bool {
		if ins[i].off != ins[j].off {
			return ins[i].off < ins[j].off
		}
		if ins[i].orden != ins[j].orden {
			return ins[i].orden < ins[j].orden
		}
		return ins[i].seq < ins[j].seq
	})
	var sb strings.Builder
	sb.Grow(len(fuente) + 32*len(ins))
	ult := 0
	for _, x := range ins {
		sb.WriteString(fuente[ult:x.off])
		sb.WriteString(x.texto)
		ult = x.off
	}
	sb.WriteString(fuente[ult:])
	return sb.String()
}

// nombresGlobales returns every name declared at the top level of f (funcs without receiver, types, vars, consts).
func nombresGlobales(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				out[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.ValueSpec:
					for _, n := range s.Names {
						out[n.Name] = true
					}
				case *ast.TypeSpec:
					out[s.Name.Name] = true
				}
			}
		}
	}
	delete(out, "_")
	return out
}

// funcionesRecursivas returns the top-level FuncDecls that belong to a cycle of the call graph
// (self or mutual recursion). Calls are resolved by name: a plain call f() points at the top-level
// func f, and x.m() points at every method named m (an over-approximation, which is safe).
func funcionesRecursivas(f *ast.File) map[*ast.FuncDecl]bool {
	var nodos []*ast.FuncDecl
	funcs := map[string]int{}
	metodos := map[string][]int{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		i := len(nodos)
		nodos = append(nodos, fd)
		if fd.Recv == nil {
			funcs[fd.Name.Name] = i
		} else {
			metodos[fd.Name.Name] = append(metodos[fd.Name.Name], i)
		}
	}
	aristas := make([][]int, len(nodos))
	for i, fd := range nodos {
		visto := map[int]bool{}
		agregar := func(j int) {
			if !visto[j] {
				visto[j] = true
				aristas[i] = append(aristas[i], j)
			}
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				for _, j := range metodos[x.Sel.Name] {
					agregar(j)
				}
			case *ast.Ident:
				if j, ok := funcs[x.Name]; ok {
					agregar(j)
				}
			}
			return true
		})
	}
	// Tarjan's strongly connected components.
	indice := make([]int, len(nodos))
	bajo := make([]int, len(nodos))
	enPila := make([]bool, len(nodos))
	for i := range indice {
		indice[i] = -1
	}
	var pila []int
	cont := 0
	out := map[*ast.FuncDecl]bool{}
	var conectar func(v int)
	conectar = func(v int) {
		indice[v], bajo[v] = cont, cont
		cont++
		pila = append(pila, v)
		enPila[v] = true
		for _, w := range aristas[v] {
			if indice[w] < 0 {
				conectar(w)
				bajo[v] = min(bajo[v], bajo[w])
			} else if enPila[w] {
				bajo[v] = min(bajo[v], indice[w])
			}
		}
		if bajo[v] != indice[v] {
			return
		}
		var comp []int
		for {
			w := pila[len(pila)-1]
			pila = pila[:len(pila)-1]
			enPila[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		if len(comp) > 1 {
			for _, w := range comp {
				out[nodos[w]] = true
			}
			return
		}
		for _, w := range aristas[v] {
			if w == v {
				out[nodos[v]] = true
			}
		}
	}
	for v := range nodos {
		if indice[v] < 0 {
			conectar(v)
		}
	}
	return out
}

// UsaPrefijo reports whether any identifier, comment-free, contains "nyx__" in any case.
func UsaPrefijo(fuente string) bool {
	_, ok := PosicionPrefijo(fuente)
	return ok
}

// PosicionPrefijo returns the line of the first identifier that contains "nyx__" in any case.
func PosicionPrefijo(fuente string) (linea int, ok bool) {
	fset := token.NewFileSet()
	tf := fset.AddFile("x.go", -1, len(fuente))
	var s scanner.Scanner
	s.Init(tf, []byte(fuente), func(token.Position, string) {}, 0)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return 0, false
		}
		if tok == token.IDENT && strings.Contains(strings.ToLower(lit), Prefijo) {
			return fset.Position(pos).Line, true
		}
	}
}

// TieneGoroutinas reports `go` statements (fuel is then disabled for PermisoUsuario runs).
func TieneGoroutinas(fuente string) bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.SkipObjectResolution)
	if err != nil {
		// unreadable code: look for the keyword itself
		tf := fset.AddFile("y.go", -1, len(fuente))
		var s scanner.Scanner
		s.Init(tf, []byte(fuente), func(token.Position, string) {}, 0)
		for {
			_, tok, _ := s.Scan()
			if tok == token.EOF {
				return false
			}
			if tok == token.GO {
				return true
			}
		}
	}
	hay := false
	ast.Inspect(f, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			hay = true
		}
		return !hay
	})
	return hay
}
