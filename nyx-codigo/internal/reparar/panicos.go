package reparar

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// ErrSinEjecutor is returned by Panicos and Logica when the Reparador has no sandbox.
var ErrSinEjecutor = errors.New("reparar: no tengo caja de arena para ejecutar el código")

// Panicos repairs runtime panics. It runs the cases on the original code; for every distinct panic it
// applies the rules keyed by the panic text (a length guard, "<=" → "<" in the loop, make for a nil
// map, a guard against dividing by zero, a nil check, min(…, len(x)) for slice bounds), drops the
// candidates that do not type-check, and validates all of them with ONE batched Preparar. It returns the
// candidates that leave fewer panicking cases than the original, best first (no panic left, most cases
// passing, fewest changes). With no panic it returns nil; when no rule helps it returns an empty list.
func (r *Reparador) Panicos(ctx context.Context, fuente string, f nucleo.Firma, casos []nucleo.Caso, n *nucleo.Nodo) ([]Parche, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.Ej == nil {
		return nil, ErrSinEjecutor
	}
	if len(casos) == 0 {
		return nil, errors.New("reparar: no hay casos con los que probar")
	}
	a := analizar(fuente)
	if a.sintaxis || len(a.errs) > 0 {
		return nil, fmt.Errorf("%w: arregla primero los errores de compilación", nucleo.ErrNoCompila)
	}
	paso := n.Sub(nucleo.PasoArreglo, "Busco por qué se rompe")
	b, comp, err := r.Ej.Preparar(ctx, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	if err != nil {
		paso.Mal("No pude preparar el código: %s", motivoPreparar(err, comp))
		return nil, err
	}
	rs, err := b.Probar(ctx, casos, nucleo.OpcionesProbar{})
	b.Cerrar()
	if err != nil {
		paso.Mal("No pude ejecutar las pruebas")
		return nil, err
	}
	orig := rs[0]
	mensajes := panicosDe(orig)
	if len(mensajes) == 0 {
		paso.Info("No se rompe con ninguno de los %d casos", len(casos))
		return nil, nil
	}
	panicosOrig := contarPanicos(orig)
	for _, m := range mensajes {
		paso.Nota("%s", TraducirPanico(m))
	}
	cands := candidatosPanico(a, f, mensajes)
	cands = r.compilables(cands)
	if len(cands) == 0 {
		paso.Mal("No conozco ningún arreglo para este error")
		return []Parche{}, nil
	}
	paso.Detalle("Pruebo %d arreglos posibles", len(cands))
	variantes := make([]string, 0, len(cands)+1)
	variantes = append(variantes, fuente)
	for _, c := range cands {
		variantes = append(variantes, c.Fuente)
	}
	b, comp, err = r.Ej.Preparar(ctx, nucleo.Preparacion{Variantes: variantes, Firma: f, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	if err != nil {
		paso.Mal("No pude preparar los arreglos: %s", motivoPreparar(err, comp))
		return nil, err
	}
	defer b.Cerrar()
	rs, err = b.Probar(ctx, casos, nucleo.OpcionesProbar{})
	if err != nil {
		paso.Mal("No pude ejecutar las pruebas")
		return nil, err
	}
	type valorado struct {
		p       Parche
		panicos int
		regla   string
		orden   int
	}
	var buenos []valorado
	for k, c := range cands {
		fila := rs[k+1]
		if !ejecutadaEntera(fila) {
			r.exito("regla:"+c.Cambio.Regla, false)
			continue
		}
		np := contarPanicos(fila)
		r.exito("regla:"+c.Cambio.Regla, np == 0)
		if np >= panicosOrig {
			continue
		}
		buenos = append(buenos, valorado{p: Parche{Fuente: c.Fuente, Cambios: c.cambios(), Pasan: contarOK(fila), Total: len(casos)}, panicos: np, regla: c.Cambio.Regla, orden: k})
	}
	sort.SliceStable(buenos, func(i, j int) bool {
		x, y := buenos[i], buenos[j]
		if x.panicos != y.panicos {
			return x.panicos < y.panicos
		}
		if x.p.Pasan != y.p.Pasan {
			return x.p.Pasan > y.p.Pasan
		}
		if len(x.p.Cambios) != len(y.p.Cambios) {
			return len(x.p.Cambios) < len(y.p.Cambios)
		}
		if tx, ty := r.tasa("regla:"+x.regla), r.tasa("regla:"+y.regla); tx != ty {
			return tx > ty
		}
		return x.orden < y.orden
	})
	out := make([]Parche, len(buenos))
	for i, v := range buenos {
		out[i] = v.p
	}
	if len(out) == 0 {
		paso.Mal("Ninguno de los %d arreglos quita el error", len(cands))
		return out, nil
	}
	mejor := out[0]
	for _, c := range mejor.Cambios {
		paso.Sub(nucleo.PasoArreglo, "%s (línea %d)", c.Porque, c.Linea).Bien("")
	}
	if buenos[0].panicos == 0 {
		paso.Bien("Ya no se rompe: pasa %d de %d casos", mejor.Pasan, mejor.Total)
	} else {
		paso.Info("Se rompe menos: pasa %d de %d casos", mejor.Pasan, mejor.Total)
	}
	return out, nil
}

// cambios returns the mutant's change plus the cleanup changes.
func (m mutacion) cambios() []Cambio {
	return append([]Cambio{m.Cambio}, m.extra...)
}

func motivoPreparar(err error, comp nucleo.Compilacion) string {
	if errors.Is(err, nucleo.ErrNoCompila) && len(comp.Errores) > 0 {
		return Traducir(comp.Errores[0])
	}
	return err.Error()
}

// panicosDe returns the distinct panic messages of a row of results, in case order.
func panicosDe(fila []nucleo.ResultadoCaso) []string {
	var out []string
	visto := map[string]bool{}
	for _, r := range fila {
		m := r.Panico
		if m == "" {
			continue
		}
		m = strings.TrimPrefix(strings.TrimSpace(m), "runtime error: ")
		if !visto[m] {
			visto[m] = true
			out = append(out, m)
		}
	}
	return out
}

func rota(r nucleo.ResultadoCaso) bool {
	return r.Panico != "" || r.Caida || r.Agotado || r.SinCombustible
}

func contarPanicos(fila []nucleo.ResultadoCaso) int {
	n := 0
	for _, r := range fila {
		if rota(r) || !r.Ejecutado {
			n++
		}
	}
	return n
}

func contarOK(fila []nucleo.ResultadoCaso) int {
	n := 0
	for _, r := range fila {
		if r.Ejecutado && r.OK {
			n++
		}
	}
	return n
}

// ejecutadaEntera reports whether the variant ran (it compiled and was not left out).
func ejecutadaEntera(fila []nucleo.ResultadoCaso) bool {
	for _, r := range fila {
		if r.Ejecutado {
			return true
		}
	}
	return len(fila) == 0
}

// ---- candidates keyed by the panic text ----

// candidatosPanico returns the fixes for the given panic messages, as edits on a.fuente.
func candidatosPanico(a *analisis, f nucleo.Firma, mensajes []string) []mutacion {
	g := &generadorMut{a: a, vistos: map[string]bool{normalizarFuente(a.fuente): true}}
	if a.info == nil {
		return nil
	}
	funcs := funcionesOrdenadas(a, f.Nombre)
	for _, m := range mensajes {
		switch {
		case reIndice.MatchString(m):
			x := reIndice.FindStringSubmatch(m)
			k, _ := strconv.Atoi(x[1])
			largo, _ := strconv.Atoi(x[2])
			if k >= 0 && largo <= k {
				for _, fd := range funcs {
					g.guardaLargo(fd, k)
				}
			}
			for _, fd := range funcs {
				g.limiteBucle(fd, k < 0)
			}
		case strings.Contains(m, "assignment to entry in nil map"):
			for _, fd := range funcs {
				g.crearMapa(fd)
			}
		case strings.Contains(m, "integer divide by zero"):
			for _, fd := range funcs {
				g.guardaDivision(fd)
			}
		case strings.Contains(m, "nil pointer dereference") || strings.Contains(m, "invalid memory address"):
			for _, fd := range funcs {
				g.guardaNil(fd)
			}
		case reCorte.MatchString(m):
			for _, fd := range funcs {
				g.recortar(fd)
			}
		}
	}
	return g.out
}

// funcionesOrdenadas returns the function declarations with a body, the target first.
func funcionesOrdenadas(a *analisis, nombre string) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, d := range a.archivo.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			out = append(out, fd)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name.Name == nombre && out[j].Name.Name != nombre })
	return out
}

// retornoCero is "return <zero values>" for fd ("return" with no results).
func (a *analisis) retornoCero(fd *ast.FuncDecl) (string, string, bool) {
	ceros, ok := a.cerosDe(fd.Type)
	if !ok {
		return "", "", false
	}
	if len(ceros) == 0 {
		return "return", "", true
	}
	return "return " + strings.Join(ceros, ", "), strings.Join(ceros, ", "), true
}

// alPrincipio inserts lines at the top of fd's body.
func (g *generadorMut) alPrincipio(fd *ast.FuncDecl, lineas []string, porque string) edicion {
	a := g.a
	ini := a.off(fd.Body.Lbrace) + 1
	ind := "\t"
	if len(fd.Body.List) > 0 {
		ind = indentacion(a.fuente, a.off(fd.Body.List[0].Pos()))
	}
	texto := ""
	for _, l := range lineas {
		texto += "\n" + ind + l
	}
	e := a.ed(ini, ini, texto, porque)
	e.cambio.Linea = lineaDe(a.fuente, ini) + 1
	e.cambio.Antes = ""
	e.cambio.Despues = unirLineas(lineas)
	return e
}

// antesDe inserts lines before statement s (same indentation).
func (g *generadorMut) antesDe(s ast.Stmt, lineas []string, porque string) edicion {
	a := g.a
	ini := inicioLinea(a.fuente, a.off(s.Pos()))
	ind := indentacion(a.fuente, a.off(s.Pos()))
	texto := ""
	for _, l := range lineas {
		texto += ind + l + "\n"
	}
	e := a.ed(ini, ini, texto, porque)
	e.cambio.Antes = ""
	e.cambio.Despues = unirLineas(lineas)
	return e
}

// parametros returns the parameters (and receiver) of fd with their objects.
func (a *analisis) parametros(fd *ast.FuncDecl) []*types.Var {
	var out []*types.Var
	listas := []*ast.FieldList{fd.Recv, fd.Type.Params}
	for _, l := range listas {
		if l == nil {
			continue
		}
		for _, campo := range l.List {
			for _, nm := range campo.Names {
				if v, ok := a.info.Defs[nm].(*types.Var); ok && nm.Name != "_" {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// usadosComo returns the params of fd that appear as x in nodes accepted by usa (in source order).
func (a *analisis) usadosComo(fd *ast.FuncDecl, usa func(n ast.Node) ast.Expr) []*types.Var {
	params := a.parametros(fd)
	es := map[types.Object]bool{}
	for _, p := range params {
		es[p] = true
	}
	visto := map[types.Object]bool{}
	var out []*types.Var
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if x := usa(n); x != nil {
			if id, ok := x.(*ast.Ident); ok {
				if o := a.info.Uses[id]; o != nil && es[o] && !visto[o] {
					visto[o] = true
					out = append(out, o.(*types.Var))
				}
			}
		}
		return true
	})
	return out
}

// guardaLargo: "index out of range [k] with length n" (n ≤ k) → a length guard at the top.
func (g *generadorMut) guardaLargo(fd *ast.FuncDecl, k int) {
	a := g.a
	ret, ceros, ok := a.retornoCero(fd)
	if !ok {
		return
	}
	ps := a.usadosComo(fd, func(n ast.Node) ast.Expr {
		if ix, ok := n.(*ast.IndexExpr); ok {
			if t := a.tipoDe(ix.X); t != nil {
				switch t.Underlying().(type) {
				case *types.Slice, *types.Basic, *types.Array:
					return ix.X
				}
			}
		}
		return nil
	})
	for _, p := range ps {
		cond := "len(" + p.Name() + ") == 0"
		quien := "«" + p.Name() + "» está vacía"
		if k > 0 {
			cond = "len(" + p.Name() + ") <= " + strconv.Itoa(k)
			quien = "«" + p.Name() + "» tiene " + strconv.Itoa(k) + " elementos o menos"
		}
		porque := "si " + quien + ", devuelvo " + valorTexto(ceros) + " (es una suposición)"
		e := g.alPrincipio(fd, []string{"if " + cond + " {", "\t" + ret, "}"}, porque)
		g.agregar(opPanico, "guarda_largo", fd.Body, []edicion{e}, porque)
	}
}

// unirLineas shows inserted lines on one line: "if len(xs) == 0 { return 0 }".
func unirLineas(ls []string) string {
	partes := make([]string, len(ls))
	for i, l := range ls {
		partes[i] = strings.TrimSpace(l)
	}
	return strings.Join(partes, " ")
}

func valorTexto(ceros string) string {
	if ceros == "" {
		return "sin hacer nada"
	}
	return "«" + ceros + "»"
}

// limiteBucle: "index out of range [i] with length i" inside a loop → "<=" becomes "<", or the bound
// len(x) becomes len(x)-1; for a negative index, ">=" becomes ">". A loop that starts at len(x) starts
// at len(x)-1.
func (g *generadorMut) limiteBucle(fd *ast.FuncDecl, negativo bool) {
	a := g.a
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		fs, ok := n.(*ast.ForStmt)
		if !ok || !indexa(fs.Body) {
			return true
		}
		if b, ok := fs.Cond.(*ast.BinaryExpr); ok {
			switch {
			case !negativo && b.Op == token.LEQ:
				g.cambiarOpRegla(fs, b.OpPos, token.LEQ, token.LSS, "menor_igual_a_menor", "el bucle se pasaba una vuelta: cambio «<=» por «<»")
			case negativo && b.Op == token.GEQ:
				g.cambiarOpRegla(fs, b.OpPos, token.GEQ, token.GTR, "mayor_igual_a_mayor", "el bucle se pasaba una vuelta: cambio «>=» por «>»")
			}
			if !negativo {
				for _, lado := range []ast.Expr{b.Y, b.X} {
					if esLen(a, lado) {
						t := a.texto(lado)
						porque := "el bucle se pasaba una vuelta: el límite es «" + t + "-1»"
						g.agregar(opPanico, "limite_menos_uno", fs, []edicion{a.reemplazar(lado, t+"-1", porque)}, porque)
					}
				}
			}
		}
		{
			if as, ok := fs.Init.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && esLen(a, as.Rhs[0]) {
				t := a.texto(as.Rhs[0])
				porque := "la última posición es «" + t + "-1», no «" + t + "»"
				g.agregar(opPanico, "inicio_menos_uno", fs, []edicion{a.reemplazar(as.Rhs[0], t+"-1", porque)}, porque)
			}
		}
		return true
	})
	// a range loop that reads x[i+1]: the bound is the caller's job; nothing to do here
}

func (g *generadorMut) cambiarOpRegla(n ast.Node, pos token.Pos, viejo, nuevo token.Token, regla, porque string) {
	o := g.a.off(pos)
	e := g.a.ed(o, o+len(viejo.String()), nuevo.String(), porque)
	g.agregar(opPanico, regla, n, []edicion{e}, porque)
}

func indexa(n ast.Node) bool {
	si := false
	ast.Inspect(n, func(x ast.Node) bool {
		if _, ok := x.(*ast.IndexExpr); ok {
			si = true
		}
		return !si
	})
	return si
}

// crearMapa: "assignment to entry in nil map" → var m map[K]V becomes m := make(map[K]V); a named map
// result gets m = make(…) at the top.
func (g *generadorMut) crearMapa(fd *ast.FuncDecl) {
	a := g.a
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		ds, ok := n.(*ast.DeclStmt)
		if !ok {
			return true
		}
		gd, ok := ds.Decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR || len(gd.Specs) != 1 {
			return true
		}
		vs := gd.Specs[0].(*ast.ValueSpec)
		if len(vs.Names) != 1 || len(vs.Values) != 0 || vs.Type == nil {
			return true
		}
		if t := a.tipoDe(vs.Type); t == nil || !esMapa(t) {
			return true
		}
		nombre, tipo := vs.Names[0].Name, a.texto(vs.Type)
		porque := "el mapa «" + nombre + "» no estaba creado: hace falta «make»"
		e := a.reemplazar(ds, nombre+" := make("+tipo+")", porque)
		g.agregar(opPanico, "crear_mapa", ds, []edicion{e}, porque)
		return true
	})
	if fd.Type.Results != nil {
		for _, campo := range fd.Type.Results.List {
			t := a.tipoDe(campo.Type)
			if t == nil || !esMapa(t) {
				continue
			}
			for _, nm := range campo.Names {
				porque := "el mapa «" + nm.Name + "» no estaba creado: hace falta «make»"
				e := g.alPrincipio(fd, []string{nm.Name + " = make(" + a.texto(campo.Type) + ")"}, porque)
				g.agregar(opPanico, "crear_mapa", fd.Body, []edicion{e}, porque)
			}
		}
	}
}

func esMapa(t types.Type) bool {
	_, ok := t.Underlying().(*types.Map)
	return ok
}

// guardaDivision: "integer divide by zero" → if d == 0 { return zeros } before the statement (or
// continue inside a loop).
func (g *generadorMut) guardaDivision(fd *ast.FuncDecl) {
	a := g.a
	ret, ceros, ok := a.retornoCero(fd)
	if !ok {
		return
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		var div ast.Expr
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if (x.Op == token.QUO || x.Op == token.REM) && esEntero(a.tipoDe(x)) {
				div = x.Y
			}
		case *ast.AssignStmt:
			if (x.Tok == token.QUO_ASSIGN || x.Tok == token.REM_ASSIGN) && len(x.Rhs) == 1 && esEntero(a.tipoDe(x.Rhs[0])) {
				div = x.Rhs[0]
			}
		}
		if div == nil || !a.puro(div) {
			return true
		}
		if tv, ok := a.info.Types[div]; ok && tv.Value != nil {
			return true
		}
		s, bloque := a.sentenciaEn(div.Pos())
		if s == nil || bloque == nil {
			return true
		}
		d := a.texto(div)
		porque := "si «" + d + "» vale 0 no se puede dividir: devuelvo " + valorTexto(ceros) + " (es una suposición)"
		e := g.antesDe(s, []string{"if " + d + " == 0 {", "\t" + ret, "}"}, porque)
		g.agregar(opPanico, "guarda_division", s, []edicion{e}, porque)
		if dentroDeBucle(a, s) {
			porque := "si «" + d + "» vale 0 no se puede dividir: me salto esa vuelta (es una suposición)"
			e := g.antesDe(s, []string{"if " + d + " == 0 {", "\tcontinue", "}"}, porque)
			g.agregar(opPanico, "guarda_division", s, []edicion{e}, porque)
		}
		return true
	})
}

func dentroDeBucle(a *analisis, s ast.Stmt) bool {
	cam := a.camino(s.Pos())
	for i := len(cam) - 1; i >= 0; i-- {
		switch cam[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		case *ast.FuncDecl, *ast.FuncLit:
			return false
		}
	}
	return false
}

// guardaNil: "nil pointer dereference" → if p == nil { return zeros } at the top, for every pointer
// parameter that is dereferenced.
func (g *generadorMut) guardaNil(fd *ast.FuncDecl) {
	a := g.a
	ret, ceros, ok := a.retornoCero(fd)
	if !ok {
		return
	}
	ps := a.usadosComo(fd, func(n ast.Node) ast.Expr {
		var x ast.Expr
		switch y := n.(type) {
		case *ast.SelectorExpr:
			x = y.X
		case *ast.StarExpr:
			x = y.X
		case *ast.IndexExpr:
			x = y.X
		}
		if x == nil {
			return nil
		}
		if t := a.tipoDe(x); t != nil {
			if _, ok := t.Underlying().(*types.Pointer); ok {
				return x
			}
		}
		return nil
	})
	for _, p := range ps {
		porque := "si «" + p.Name() + "» es nil, devuelvo " + valorTexto(ceros) + " (es una suposición)"
		e := g.alPrincipio(fd, []string{"if " + p.Name() + " == nil {", "\t" + ret, "}"}, porque)
		g.agregar(opPanico, "guarda_nil", fd.Body, []edicion{e}, porque)
	}
}

// recortar: "slice bounds out of range" → x[a:min(b, len(x))], x[min(a, len(x)):b], x[min(a, b):b].
func (g *generadorMut) recortar(fd *ast.FuncDecl) {
	a := g.a
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		se, ok := n.(*ast.SliceExpr)
		if !ok || se.Slice3 || !a.puro(se.X) {
			return true
		}
		x := a.texto(se.X)
		lo, hi := "", ""
		if se.Low != nil {
			lo = a.texto(se.Low)
		}
		if se.High != nil {
			hi = a.texto(se.High)
		}
		var opciones [][2]string
		if hi != "" {
			opciones = append(opciones, [2]string{lo, "min(" + hi + ", len(" + x + "))"})
		}
		if lo != "" {
			opciones = append(opciones, [2]string{"min(" + lo + ", len(" + x + "))", hi})
		}
		if lo != "" && hi != "" {
			opciones = append(opciones, [2]string{"min(" + lo + ", len(" + x + "))", "min(" + hi + ", len(" + x + "))"})
			opciones = append(opciones, [2]string{"min(" + lo + ", " + hi + ")", hi})
		}
		for _, o := range opciones {
			nuevo := x + "[" + o[0] + ":" + o[1] + "]"
			porque := "el corte se salía de la lista: lo limito con «min» → «" + nuevo + "»"
			g.agregar(opPanico, "recortar_corte", se, []edicion{a.reemplazar(se, nuevo, porque)}, porque)
		}
		return true
	})
}

// ---- compile filter ----

// compilables keeps the candidates that type-check, after removing the variables and imports that the
// edit left unused (those cleanups are added to the candidate's changes). Duplicates are dropped.
func (r *Reparador) compilables(ms []mutacion) []mutacion {
	var out []mutacion
	visto := map[string]bool{}
	for _, m := range ms {
		src, extra, ok := limpiarYChequear(m.Fuente)
		if !ok {
			continue
		}
		src = normalizarFuente(src)
		if visto[src] {
			continue
		}
		visto[src] = true
		m.Fuente = src
		m.extra = extra
		out = append(out, m)
	}
	return out
}

// limpiarYChequear type-checks src; when the only errors are unused variables or imports, it removes
// them (up to 3 rounds) and reports the cleanup as changes.
func limpiarYChequear(src string) (string, []Cambio, bool) {
	var extra []Cambio
	for ronda := 0; ronda < 4; ronda++ {
		a := analizar(src)
		if a.sintaxis {
			return src, nil, false
		}
		if len(a.errs) == 0 {
			return src, extra, true
		}
		if ronda == 3 {
			break
		}
		var eds []edicion
		for _, e := range a.errs {
			msg := primeraLinea(e.Msg)
			if m := reNoUsadaVar.FindStringSubmatch(msg); m != nil {
				nombre := m[1]
				if nombre == "" {
					nombre = m[2]
				}
				es := a.quitarVariable(e, nombre)
				if len(es) == 0 {
					return src, nil, false
				}
				eds = append(eds, es...)
				continue
			}
			if m := reNoUsadoImport.FindStringSubmatch(msg); m != nil {
				if ed, ok := quitarImport(src, m[1]); ok {
					ed.cambio = Cambio{Regla: "quitar_import", Linea: e.Linea, Antes: textoLinea(src, e.Linea), Porque: "«" + m[1] + "» ya no se usa: quito el import"}
					eds = append(eds, ed)
					continue
				}
			}
			return src, nil, false
		}
		nuevo, aplicadas := aplicar(src, eds)
		if nuevo == src {
			return src, nil, false
		}
		for _, e := range aplicadas {
			extra = append(extra, e.cambio)
		}
		src = nuevo
	}
	return src, nil, false
}

var (
	reNoUsadaVar    = regexp.MustCompile(`^(?:declared and not used: (\w+)|(\w+) declared (?:and|but) not used)$`)
	reNoUsadoImport = regexp.MustCompile(`^"([^"]+)" imported(?: as \w+)? and not used$`)
)

// quitarVariable removes a variable that an edit left unused: its declaration and every assignment to
// it when they have no side effects; otherwise it adds "_ = x".
func (a *analisis) quitarVariable(e nucleo.ErrorGo, nombre string) []edicion {
	pos := a.posDe(e)
	id := a.identEn(pos)
	if id == nil || id.Name != nombre {
		return nil
	}
	porque := "«" + nombre + "» ya no se usa: la quito"
	// range variables and multi-variable definitions: the compile rule knows how to blank them
	cam := a.camino(pos)
	for i := len(cam) - 1; i >= 0; i-- {
		if s, ok := cam[i].(*ast.RangeStmt); ok && (s.Key == ast.Expr(id) || s.Value == ast.Expr(id)) {
			return conRegla(reglaNoUsada(a, e, []string{"", nombre}))
		}
		if s, ok := cam[i].(*ast.AssignStmt); ok {
			if len(s.Lhs) != 1 {
				return conRegla(reglaNoUsada(a, e, []string{"", nombre}))
			}
			break
		}
		if _, ok := cam[i].(ast.Stmt); ok {
			break
		}
	}
	o := a.info.Defs[id]
	if o == nil {
		return nil
	}
	_, cuerpo := a.funcionEn(pos)
	if cuerpo == nil {
		return nil
	}
	var sentencias []ast.Stmt
	limpio := true
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		if !limpio || n == nil {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == 1 {
				if li, ok := s.Lhs[0].(*ast.Ident); ok && (a.info.Defs[li] == o || a.info.Uses[li] == o) {
					if len(s.Rhs) != 1 || !a.puro(s.Rhs[0]) || s.Tok != token.DEFINE && s.Tok != token.ASSIGN {
						limpio = false
						return false
					}
					sentencias = append(sentencias, s)
					return false
				}
			}
		case *ast.IncDecStmt:
			if li, ok := s.X.(*ast.Ident); ok && a.info.Uses[li] == o {
				sentencias = append(sentencias, s)
				return false
			}
		case *ast.DeclStmt:
			if gd, ok := s.Decl.(*ast.GenDecl); ok && len(gd.Specs) == 1 {
				if vs, ok := gd.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && a.info.Defs[vs.Names[0]] == o {
					if len(vs.Values) == 1 && !a.puro(vs.Values[0]) {
						limpio = false
						return false
					}
					sentencias = append(sentencias, s)
					return false
				}
			}
		case *ast.Ident:
			if a.info.Uses[s] == o {
				limpio = false // read somewhere we do not understand
			}
		}
		return true
	})
	if !limpio || len(sentencias) == 0 {
		// the assignments have side effects: keep them and mark the variable as used
		if decl, _ := a.sentenciaEn(id.Pos()); decl != nil {
			return conRegla([]edicion{a.usarBlanco(decl, nombre, "«"+nombre+"» ya no se usa")})
		}
		return nil
	}
	var out []edicion
	for _, s := range sentencias {
		out = append(out, a.borrar(s, porque))
	}
	return conRegla(out)
}

func conRegla(eds []edicion) []edicion {
	for i := range eds {
		if eds[i].cambio.Regla == "" {
			eds[i].cambio.Regla = "quitar_variable"
		}
	}
	return eds
}
