package reparar

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// regla matches a type-checker message and proposes ranked edits.
type regla struct {
	nombre string
	re     *regexp.Regexp
	fn     func(a *analisis, e nucleo.ErrorGo, m []string) []edicion
}

var reglas []regla

func init() {
	r := func(nombre, expr string, fn func(a *analisis, e nucleo.ErrorGo, m []string) []edicion) {
		reglas = append(reglas, regla{nombre: nombre, re: regexp.MustCompile(expr), fn: fn})
	}
	r("quitar_import", `^"([^"]+)" imported(?: as \w+)? and not used$`, reglaQuitarImport)
	r("importar_paquete", `^undefined: (\w+)$`, reglaImportar)
	r("levenshtein", `^undefined: (\w+)\.(\w+)$`, reglaMiembroPaquete)
	r("levenshtein", `^name (\w+) not exported by package (\w+)$`, func(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
		return reglaMiembroPaquete(a, e, []string{m[0], m[2], m[1]})
	})
	r("levenshtein", `^(.+)\.(\w+) undefined \(type (.+) has no field or method (\w+)(?:, but does have (?:field|method) (\w+))?\)$`, reglaCampo)
	r("levenshtein", `^undefined: (\w+)$`, reglaIdentificador)
	r("quitar_variable", `^declared and not used: (\w+)$`, reglaNoUsada)
	r("quitar_variable", `^(\w+) declared (?:and|but) not used$`, reglaNoUsada)
	r("igual_simple", `^no new variables on left side of :=$`, reglaSinVariablesNuevas)
	r("retorno_faltante", `^missing return$`, reglaRetornoFaltante)
	r("nil_a_cero", `^cannot use nil as (.+?) value in (.+)$`, reglaNilACero)
	r("convertir_tipo", `^cannot use (.+?) \((.+)\) as (.+?) value in (.+)$`, reglaConvertir)
	r("atoi_dos_valores", `^assignment mismatch: 1 variable but (.+) returns 2 values$`, reglaDosValores)
	r("valores_retorno", `^(too many|not enough) return values`, reglaValoresRetorno)
	r("condicion_booleana", `^non-boolean condition in (\w+) statement$`, reglaCondicion)
	r("condicion_booleana", `^invalid operation: operator ! not defined on (.+) \((.+)\)$`, reglaNegacion)
	r("tipos_distintos", `^invalid operation: (.+) \(mismatched types (.+) and (.+)\)$`, reglaTiposDistintos)
	r("asignar_campo_mapa", `^cannot assign to struct field (.+) in map$`, reglaCampoMapa)
	r("expresion_sin_usar", `^(.+) \((?:value|variable) of type (.+)\) is not used$`, reglaSinUsar)
	r("indice_entero", `^invalid argument: index (.+) \((.+) of type (float32|float64)\) must be integer$`, reglaIndiceEntero)
}

func primeraLinea(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// candidatos returns the ranked edits for one error: rules ordered by their success rate
// (Stats.Tasa("regla:"+nombre)), keeping each rule's own ranking.
func (r *Reparador) candidatos(a *analisis, e nucleo.ErrorGo) []edicion {
	msg := primeraLinea(e.Msg)
	var out []edicion
	for _, rg := range reglas {
		m := rg.re.FindStringSubmatch(msg)
		if m == nil {
			continue
		}
		for _, ed := range rg.fn(a, e, m) {
			if ed.cambio.Regla == "" {
				ed.cambio.Regla = rg.nombre
			}
			if ed.cambio.Linea == 0 {
				ed.cambio.Linea = e.Linea
			}
			out = append(out, ed)
		}
	}
	tasas := map[string]float64{}
	for _, ed := range out {
		if _, ok := tasas[ed.cambio.Regla]; !ok {
			tasas[ed.cambio.Regla] = r.tasa("regla:" + ed.cambio.Regla)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return tasas[out[i].cambio.Regla] > tasas[out[j].cambio.Regla] })
	return out
}

// ---- edit helpers ----

func limpiarLineas(s string) string {
	lineas := strings.Split(s, "\n")
	for i, l := range lineas {
		lineas[i] = strings.TrimSpace(l)
	}
	return strings.TrimSpace(strings.Join(lineas, "\n"))
}

// ed builds an edit replacing a.fuente[ini:fin] with texto; Antes/Despues are the affected lines.
func (a *analisis) ed(ini, fin int, texto, porque string) edicion {
	src := a.fuente
	li := inicioLinea(src, ini)
	f := fin
	if f > ini {
		f--
	}
	lf := finLinea(src, f)
	if lf < fin {
		lf = fin
	}
	return edicion{ini: ini, fin: fin, texto: texto, cambio: Cambio{
		Linea: lineaDe(src, ini), Antes: limpiarLineas(src[li:lf]), Despues: limpiarLineas(src[li:ini] + texto + src[fin:lf]), Porque: porque,
	}}
}

func (a *analisis) reemplazar(n ast.Node, texto, porque string) edicion {
	return a.ed(a.off(n.Pos()), a.off(n.End()), texto, porque)
}

// borrar removes a statement: whole lines when nothing else is on them.
func (a *analisis) borrar(n ast.Node, porque string) edicion {
	src := a.fuente
	ini, fin := a.off(n.Pos()), a.off(n.End())
	li, lf := inicioLinea(src, ini), finLinea(src, fin)
	if strings.TrimSpace(src[li:ini]) == "" && strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(src[fin:lf]), ";")) == "" {
		e := a.ed(li, lf, "", porque)
		if lf < len(src) {
			e.fin = lf + 1
		}
		e.cambio.Despues = ""
		return e
	}
	if fin < len(src) && src[fin] == ';' {
		fin++
	}
	return a.ed(ini, fin, "", porque)
}

func (a *analisis) identEn(p token.Pos) *ast.Ident {
	var id *ast.Ident
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if n == nil || id != nil {
			return false
		}
		if x, ok := n.(*ast.Ident); ok && x.Pos() == p {
			id = x
			return false
		}
		return n.Pos() <= p && p < n.End()
	})
	return id
}

// padreDe returns the parent of node x along the path to its position.
func (a *analisis) padreDe(x ast.Node) ast.Node {
	cam := a.camino(x.Pos())
	for i := len(cam) - 1; i > 0; i-- {
		if cam[i] == x {
			return cam[i-1]
		}
	}
	return nil
}

func basico(t types.Type) *types.Basic {
	if t == nil {
		return nil
	}
	b, _ := t.Underlying().(*types.Basic)
	return b
}

func esEntero(t types.Type) bool {
	b := basico(t)
	return b != nil && b.Info()&types.IsInteger != 0
}

func esDecimal(t types.Type) bool {
	b := basico(t)
	return b != nil && b.Info()&types.IsFloat != 0
}

func esNumero(t types.Type) bool { return esEntero(t) || esDecimal(t) }

func esTexto(t types.Type) bool {
	b := basico(t)
	return b != nil && b.Info()&types.IsString != 0
}

func esRunaOByte(t types.Type) bool {
	b := basico(t)
	return b != nil && (b.Kind() == types.Int32 || b.Kind() == types.Uint8 || b.Kind() == types.UntypedRune)
}

func esNulable(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
		return true
	}
	return false
}

func (a *analisis) escribirTipo(t types.Type) string { return types.TypeString(t, a.calificador()) }

// evalTipo resolves a type spelled as in a compiler message, at pos.
func (a *analisis) evalTipo(pos token.Pos, s string) types.Type {
	if a.paquete == nil {
		return nil
	}
	muTipos.Lock()
	tv, err := types.Eval(a.fset, a.paquete, pos, s)
	muTipos.Unlock()
	if err != nil || !tv.IsType() {
		return nil
	}
	return tv.Type
}

func envolverSiHaceFalta(texto string, e ast.Expr) string {
	switch e.(type) {
	case *ast.Ident, *ast.BasicLit, *ast.CallExpr, *ast.SelectorExpr, *ast.IndexExpr, *ast.ParenExpr, *ast.CompositeLit:
		return texto
	}
	return "(" + texto + ")"
}

// ---- rules ----

func reglaQuitarImport(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	ed, ok := quitarImport(a.fuente, m[1])
	if !ok {
		return nil
	}
	ed.cambio = Cambio{Linea: e.Linea, Antes: textoLinea(a.fuente, e.Linea), Porque: "importabas «" + m[1] + "» pero no lo usas"}
	return []edicion{ed}
}

// exporta reports whether package ruta has a top-level name (true when it cannot be checked).
func exporta(ruta, nombre string) (bool, bool) {
	muTipos.Lock()
	defer muTipos.Unlock()
	p, err := importadorCache{}.Import(ruta)
	if err != nil || p == nil {
		return false, false
	}
	return p.Scope().Lookup(nombre) != nil, true
}

func reglaImportar(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	rutas := paquetesStd[m[1]]
	if len(rutas) == 0 {
		return nil
	}
	pos := a.posDe(e)
	var sel *ast.SelectorExpr
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := s.X.(*ast.Ident); ok && x.Pos() == pos && x.Name == m[1] {
				sel = s
				return false
			}
		}
		return n != nil && sel == nil
	})
	if sel == nil {
		return nil
	}
	var elegidas []string
	for _, ruta := range rutas {
		if ok, comprobado := exporta(ruta, sel.Sel.Name); ok || !comprobado {
			elegidas = append(elegidas, ruta)
		}
	}
	if len(elegidas) == 0 {
		elegidas = rutas[:1] // the member is misspelled: import first, fix the name next round
	}
	var out []edicion
	for _, ruta := range elegidas {
		out = append(out, edicion{ini: -1, fin: -1, importar: []string{ruta}, cambio: Cambio{
			Linea: e.Linea, Antes: "", Despues: "import " + strconv.Quote(ruta), Porque: "faltaba importar «" + ruta + "»",
		}})
	}
	return out
}

func reglaMiembroPaquete(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	paquete, nombre := m[1], m[2]
	var sel *ast.SelectorExpr
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && sel == nil {
			if x, ok := s.X.(*ast.Ident); ok && x.Name == paquete && s.Sel.Name == nombre && a.linea(s.Pos()) == e.Linea {
				sel = s
			}
		}
		return n != nil && sel == nil
	})
	if sel == nil || a.info == nil {
		return nil
	}
	pn, ok := a.info.Uses[sel.X.(*ast.Ident)].(*types.PkgName)
	if !ok {
		return nil
	}
	var nombres []string
	for _, n := range pn.Imported().Scope().Names() {
		if ast.IsExported(n) {
			nombres = append(nombres, n)
		}
	}
	var out []edicion
	for _, c := range cercanos(nombre, nombres, 3) {
		out = append(out, a.reemplazar(sel.Sel, c, "«"+paquete+"."+nombre+"» no existe; quizá quisiste decir «"+paquete+"."+c+"»"))
	}
	return out
}

var palabrasLen = map[string]bool{"length": true, "lenght": true, "lengh": true, "size": true, "count": true, "longitud": true, "largo": true, "Len": true, "Length": true}

func tieneLen(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Slice, *types.Array, *types.Map, *types.Chan:
		return true
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Pointer:
		_, ok := u.Elem().Underlying().(*types.Array)
		return ok
	}
	return false
}

func reglaCampo(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	var sel *ast.SelectorExpr
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && sel == nil && (s.Sel.Pos() == pos || (s.Sel.Name == m[4] && a.linea(s.Pos()) == e.Linea)) {
			sel = s
		}
		return n != nil && sel == nil
	})
	if sel == nil {
		return nil
	}
	t := a.tipoDe(sel.X)
	if t == nil {
		return nil
	}
	var out []edicion
	nombre := m[4]
	if palabrasLen[nombre] && tieneLen(t) {
		var nodo ast.Node = sel
		if c, ok := a.padreDe(sel).(*ast.CallExpr); ok && c.Fun == sel && len(c.Args) == 0 {
			nodo = c
		}
		ed := a.reemplazar(nodo, "len("+a.texto(sel.X)+")", "en Go el tamaño se pide con «len(…)»")
		ed.cambio.Regla = "usar_len"
		out = append(out, ed)
	}
	var cands []string
	if len(m) > 5 && m[5] != "" {
		cands = append(cands, m[5])
	}
	for _, c := range cercanos(nombre, camposYMetodos(t), 3) {
		if len(cands) == 0 || c != cands[0] {
			cands = append(cands, c)
		}
	}
	for _, c := range cands {
		out = append(out, a.reemplazar(sel.Sel, c, "«"+a.escribirTipo(t)+"» no tiene «"+nombre+"»; quizá quisiste decir «"+c+"»"))
	}
	return out
}

var palabrasOtros = map[string]string{"null": "nil", "None": "nil", "NULL": "nil", "undefined": "nil", "nullptr": "nil", "True": "true", "False": "false"}

func reglaIdentificador(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	nombre := m[1]
	pos := a.posDe(e)
	id := a.identEn(pos)
	if id == nil || id.Name != nombre {
		return nil
	}
	var out []edicion
	if lit := palabrasOtros[nombre]; lit != "" {
		ed := a.reemplazar(id, lit, "en Go se escribe «"+lit+"»")
		ed.cambio.Regla = "palabra_de_otro_lenguaje"
		out = append(out, ed)
	}
	if palabrasLen[nombre] {
		if c, ok := a.padreDe(id).(*ast.CallExpr); ok && c.Fun == id && len(c.Args) == 1 {
			ed := a.reemplazar(id, "len", "en Go el tamaño se pide con «len(…)»")
			ed.cambio.Regla = "usar_len"
			out = append(out, ed)
		}
	}
	cerca := cercanos(nombre, a.nombresVisibles(pos), 3)
	// do not propose package names for a plain value
	var filtrados []string
	for _, c := range cerca {
		if o := a.buscarVisible(pos, c); o != nil {
			if _, esPaquete := o.(*types.PkgName); esPaquete {
				continue
			}
		}
		filtrados = append(filtrados, c)
	}
	var porNombre []edicion
	for _, c := range filtrados {
		porNombre = append(porNombre, a.reemplazar(id, c, "«"+nombre+"» no existe; quizá quisiste decir «"+c+"»"))
	}
	declarar := a.declaraciones(id)
	muyCerca := len(filtrados) > 0 && damerau(strings.ToLower(filtrados[0]), strings.ToLower(nombre)) <= 1
	if muyCerca {
		out = append(out, porNombre...)
		out = append(out, declarar...)
	} else {
		out = append(out, declarar...)
		out = append(out, porNombre...)
	}
	return out
}

// declaraciones proposes declaring an undefined variable: turning its first "x = …" into "x := …",
// or adding "var x T" at the top of the function.
func (a *analisis) declaraciones(id *ast.Ident) []edicion {
	_, cuerpo := a.funcionEn(id.Pos())
	if cuerpo == nil {
		return nil
	}
	nombre := id.Name
	var primera *ast.AssignStmt
	var primeraRHS ast.Expr
	primerUso := token.NoPos
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok == token.ASSIGN && primera == nil {
				for i, l := range x.Lhs {
					if li, ok := l.(*ast.Ident); ok && li.Name == nombre {
						primera = x
						if len(x.Rhs) == len(x.Lhs) {
							primeraRHS = x.Rhs[i]
						}
					}
				}
			}
		case *ast.Ident:
			if x.Name == nombre && a.info.Uses[x] == nil && a.info.Defs[x] == nil && (primerUso == token.NoPos || x.Pos() < primerUso) {
				primerUso = x.Pos()
			}
		}
		return true
	})
	if primera == nil {
		return nil
	}
	var out []edicion
	enCuerpo := false
	for _, s := range cuerpo.List {
		if s == primera {
			enCuerpo = true
		}
	}
	definir := a.ed(a.off(primera.TokPos), a.off(primera.TokPos)+1, ":=", "«"+nombre+"» no existía: para crear una variable se usa «:=»")
	definir.cambio.Regla = "declarar_variable"
	// "var x T" at the top of the function
	var arriba *edicion
	if primeraRHS != nil {
		if t := a.tipoDe(primeraRHS); t != nil {
			t = types.Default(t)
			if b := basico(t); b == nil || b.Kind() != types.UntypedNil {
				ini := a.off(cuerpo.Lbrace) + 1
				ind := "\t"
				if len(cuerpo.List) > 0 {
					ind = indentacion(a.fuente, a.off(cuerpo.List[0].Pos()))
				}
				e := a.ed(ini, ini, "\n"+ind+"var "+nombre+" "+a.escribirTipo(t), "«"+nombre+"» no existía: la creo al principio de la función")
				e.cambio.Regla = "declarar_variable"
				e.cambio.Linea = lineaDe(a.fuente, ini) + 1
				e.cambio.Antes = ""
				e.cambio.Despues = "var " + nombre + " " + a.escribirTipo(t)
				arriba = &e
			}
		}
	}
	if enCuerpo && primera.Pos() <= primerUso {
		out = append(out, definir)
		if arriba != nil {
			out = append(out, *arriba)
		}
	} else {
		if arriba != nil {
			out = append(out, *arriba)
		}
		out = append(out, definir)
	}
	return out
}

// puro reports whether evaluating e has no side effects (so its statement can be removed).
func (a *analisis) puro(e ast.Expr) bool {
	switch x := e.(type) {
	case nil:
		return true
	case *ast.BasicLit, *ast.Ident, *ast.FuncLit:
		return true
	case *ast.ParenExpr:
		return a.puro(x.X)
	case *ast.UnaryExpr:
		return x.Op != token.ARROW && a.puro(x.X)
	case *ast.BinaryExpr:
		return a.puro(x.X) && a.puro(x.Y)
	case *ast.SelectorExpr:
		return a.puro(x.X)
	case *ast.IndexExpr:
		return a.puro(x.X) && a.puro(x.Index)
	case *ast.SliceExpr:
		return a.puro(x.X) && a.puro(x.Low) && a.puro(x.High) && a.puro(x.Max)
	case *ast.StarExpr:
		return a.puro(x.X)
	case *ast.KeyValueExpr:
		return a.puro(x.Key) && a.puro(x.Value)
	case *ast.CompositeLit:
		for _, el := range x.Elts {
			if !a.puro(el) {
				return false
			}
		}
		return true
	case *ast.CallExpr:
		for _, arg := range x.Args {
			if !a.puro(arg) {
				return false
			}
		}
		if a.info != nil {
			if tv, ok := a.info.Types[x.Fun]; ok && tv.IsType() {
				return true // conversion
			}
		}
		if id, ok := x.Fun.(*ast.Ident); ok {
			switch id.Name {
			case "len", "cap", "make", "new", "min", "max", "complex", "real", "imag":
				return a.info == nil || a.info.Uses[id] == nil || a.info.Uses[id].Parent() == types.Universe
			}
		}
	}
	return false
}

func reglaNoUsada(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	nombre := m[1]
	pos := a.posDe(e)
	id := a.identEn(pos)
	if id == nil || id.Name != nombre {
		return nil
	}
	porque := "creaste «" + nombre + "» pero nunca la usas"
	cam := a.camino(pos)
	for i := len(cam) - 1; i >= 0; i-- {
		switch s := cam[i].(type) {
		case *ast.RangeStmt:
			esClave, esValor := s.Key == ast.Expr(id), s.Value == ast.Expr(id)
			if !esClave && !esValor {
				continue
			}
			var ed edicion
			switch {
			case esClave && s.Value == nil:
				ed = a.ed(a.off(s.Key.Pos()), a.off(s.Range), "", porque)
			case esClave:
				ed = a.reemplazar(s.Key, "_", porque+": en un range se pone «_»")
			case esValor:
				if k, ok := s.Key.(*ast.Ident); ok && k.Name == "_" {
					ed = a.ed(a.off(s.Key.Pos()), a.off(s.Range), "", porque)
				} else {
					ed = a.ed(a.off(s.Key.End()), a.off(s.Value.End()), "", porque)
				}
			}
			ed.cambio.Regla = "rango_guion_bajo"
			return []edicion{ed}
		case *ast.AssignStmt:
			if s.Tok != token.DEFINE {
				continue
			}
			idx := -1
			for j, l := range s.Lhs {
				if l == ast.Expr(id) {
					idx = j
				}
			}
			if idx < 0 {
				continue
			}
			if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
				// "c := c + 1" inside a block, with an outer c: the user meant "="
				var out []edicion
				if o := a.buscarVisible(s.Pos(), nombre); o != nil {
					if v, esVar := o.(*types.Var); esVar && a.info.Defs[id] != nil && types.Identical(v.Type(), a.info.Defs[id].Type()) {
						tp := a.off(s.TokPos)
						ed := a.ed(tp, tp+2, "=", "ya existía una variable «"+nombre+"» fuera: para cambiarla se usa «=»; con «:=» creabas otra nueva")
						ed.cambio.Regla = "sombra"
						out = append(out, ed)
					}
				}
				if a.puro(s.Rhs[0]) {
					return append(out, a.borrar(s, porque+": la quito"))
				}
				return append(out, a.usarBlanco(s, nombre, porque))
			}
			if false {
				if a.puro(s.Rhs[0]) {
					return []edicion{a.borrar(s, porque+": la quito")}
				}
				return []edicion{a.usarBlanco(s, nombre, porque)}
			}
			// several variables: this one becomes "_" (and ":=" becomes "=" if none is left)
			partes := make([]string, len(s.Lhs))
			quedan := false
			for j, l := range s.Lhs {
				partes[j] = a.texto(l)
				if j == idx {
					partes[j] = "_"
				} else if li, ok := l.(*ast.Ident); !ok || li.Name != "_" {
					quedan = true
				}
			}
			tok := " := "
			if !quedan {
				tok = " = "
			}
			return []edicion{a.ed(a.off(s.Pos()), a.off(s.TokPos)+2, strings.Join(partes, ", ")+tok[:len(tok)-1], porque+": la cambio por «_»")}
		case *ast.ValueSpec:
			if len(s.Names) != 1 {
				continue
			}
			decl, _ := a.sentenciaEn(pos)
			ds, ok := decl.(*ast.DeclStmt)
			if !ok {
				return nil
			}
			if g, ok := ds.Decl.(*ast.GenDecl); !ok || len(g.Specs) != 1 {
				return nil
			}
			if len(s.Values) == 0 || a.puro(s.Values[0]) {
				return []edicion{a.borrar(ds, porque+": la quito")}
			}
			return []edicion{a.usarBlanco(ds, nombre, porque)}
		}
	}
	return nil
}

// usarBlanco adds "_ = x" after the statement.
func (a *analisis) usarBlanco(s ast.Node, nombre, porque string) edicion {
	fin := a.off(s.End())
	ind := indentacion(a.fuente, a.off(s.Pos()))
	e := a.ed(fin, fin, "\n"+ind+"_ = "+nombre, porque+": añado «_ = "+nombre+"»")
	e.cambio.Despues = "_ = " + nombre
	return e
}

func reglaSinVariablesNuevas(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	var as *ast.AssignStmt
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if x, ok := n.(*ast.AssignStmt); ok && x.Tok == token.DEFINE && (x.TokPos == pos || (x.Pos() <= pos && pos < x.End())) {
			as = x
		}
		return n != nil
	})
	if as == nil {
		return nil
	}
	o := a.off(as.TokPos)
	return []edicion{a.ed(o, o+2, "=", "las variables ya existían: para cambiarlas se usa «=» y no «:=»")}
}

// manejoError is the statement used when a conversion or call returns an error: return it when the
// function returns an error, otherwise panic (with a supposition note).
func (a *analisis) manejoError(pos token.Pos) (string, string, bool) {
	ft, _ := a.funcionEn(pos)
	rs := a.resultados(ft)
	if len(rs) > 0 && esError(rs[len(rs)-1]) {
		ceros, ok := a.cerosDe(ft)
		if !ok {
			return "", "", false
		}
		ceros[len(ceros)-1] = "err"
		return "return " + strings.Join(ceros, ", "), "", true
	}
	return "panic(err)", "si hay un error, el programa se para con «panic(err)»: es una suposición", true
}

func reglaRetornoFaltante(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	var ft *ast.FuncType
	var cuerpo *ast.BlockStmt
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		switch f := n.(type) {
		case *ast.FuncDecl:
			if f.Body != nil && (f.Body.Rbrace == pos || a.linea(f.Body.Rbrace) == e.Linea && f.Body.Pos() < pos) {
				ft, cuerpo = f.Type, f.Body
			}
		case *ast.FuncLit:
			if f.Body.Rbrace == pos || a.linea(f.Body.Rbrace) == e.Linea && f.Body.Pos() < pos {
				ft, cuerpo = f.Type, f.Body
			}
		}
		return n != nil
	})
	if cuerpo == nil {
		return nil
	}
	ceros, ok := a.cerosDe(ft)
	if !ok || len(ceros) == 0 {
		return nil
	}
	ret := "return " + strings.Join(ceros, ", ")
	rb := a.off(cuerpo.Rbrace)
	li := inicioLinea(a.fuente, rb)
	var ed edicion
	if strings.TrimSpace(a.fuente[li:rb]) == "" {
		ed = a.ed(li, li, indentacion(a.fuente, rb)+"\t"+ret+"\n", "a la función le faltaba un «return» al final")
	} else {
		ed = a.ed(rb, rb, "; "+ret+" ", "a la función le faltaba un «return» al final")
	}
	ed.cambio.Antes = ""
	ed.cambio.Despues = ret
	ed.suposicion = "puse «" + ret + "» al final: es una suposición"
	return []edicion{ed}
}

// ceroDeTexto returns the zero value of a type spelled as in a message.
func (a *analisis) ceroDeTexto(pos token.Pos, s string) string {
	if t := a.evalTipo(pos, s); t != nil {
		return a.ceroDe(t)
	}
	return ""
}

func reglaNilACero(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	id := a.identEn(pos)
	if id == nil || id.Name != "nil" {
		return nil
	}
	cero := a.ceroDeTexto(pos, m[1])
	if cero == "" || cero == "nil" {
		return nil
	}
	return []edicion{a.reemplazar(id, cero, "«nil» no vale como «"+m[1]+"»: su valor vacío es «"+cero+"»")}
}

func reglaConvertir(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	x := a.exprEn(pos, m[1])
	if x == nil {
		return nil
	}
	S := a.tipoDe(x)
	if S == nil {
		return nil
	}
	destino, contexto := m[3], m[4]
	T := a.evalTipo(pos, destino)
	txt := a.texto(x)
	var out []edicion
	conv := func(texto, regla, porque, suposicion string, importar ...string) {
		ed := a.reemplazar(x, texto, porque)
		ed.cambio.Regla = regla
		ed.suposicion = suposicion
		ed.importar = importar
		out = append(out, ed)
	}
	var constante bool
	if tv, ok := a.info.Types[x]; ok && tv.Value != nil {
		constante = true
	}
	// append(a, b) with b a slice → append(a, b...)
	if strings.HasPrefix(contexto, "argument to append") {
		if sl, ok := S.Underlying().(*types.Slice); ok && T != nil && types.AssignableTo(sl.Elem(), T) {
			fin := a.off(x.End())
			ed := a.ed(fin, fin, "...", "para añadir todos los elementos de otra lista se escribe «…» detrás")
			ed.cambio.Regla = "puntos_suspensivos"
			return []edicion{ed}
		}
	}
	// "a" as a rune → 'a'
	if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.STRING && T != nil && esRunaOByte(T) {
		if s, err := strconv.Unquote(lit.Value); err == nil && utf8.RuneCountInString(s) == 1 {
			conv(strconv.QuoteRune([]rune(s)[0]), "runa_literal", "un solo carácter se escribe entre comillas simples: «"+strconv.QuoteRune([]rune(s)[0])+"»", "")
			return out
		}
	}
	if T == nil {
		return nil
	}
	escrito := a.escribirTipo(T)
	switch {
	case constante && esNumero(S) && esNumero(T):
		return nil // a constant that does not fit: no safe conversion
	case esNumero(S) && esNumero(T):
		sup := ""
		if esDecimal(S) && esEntero(T) {
			sup = "convertí a «" + escrito + "»: se pierden los decimales"
		}
		conv(escrito+"("+txt+")", "convertir_tipo", "«"+txt+"» es "+a.escribirTipo(S)+" y aquí hace falta "+escrito+": lo convierto", sup)
	case esEntero(S) && esTexto(T):
		if esRunaOByte(S) {
			conv(escrito+"("+txt+")", "convertir_tipo", "para pasar un carácter a texto se usa «string(…)»", "")
		} else {
			arg := txt
			if b := basico(S); b.Kind() != types.Int {
				arg = "int(" + txt + ")"
			}
			conv("strconv.Itoa("+arg+")", "convertir_tipo", "para pasar un número a texto se usa «strconv.Itoa»", "", "strconv")
			conv(escrito+"(rune("+txt+"))", "convertir_tipo", "convierto el número en el carácter con ese código", "")
			conv("fmt.Sprint("+txt+")", "convertir_tipo", "para pasar un número a texto se puede usar «fmt.Sprint»", "", "fmt")
		}
	case esDecimal(S) && esTexto(T):
		conv("strconv.FormatFloat("+txt+", 'f', -1, 64)", "convertir_tipo", "para pasar un decimal a texto se usa «strconv.FormatFloat»", "", "strconv")
		conv("fmt.Sprint("+txt+")", "convertir_tipo", "para pasar un decimal a texto se puede usar «fmt.Sprint»", "", "fmt")
	case basico(S) != nil && basico(S).Info()&types.IsBoolean != 0 && esTexto(T):
		conv("strconv.FormatBool("+txt+")", "convertir_tipo", "para pasar verdadero/falso a texto se usa «strconv.FormatBool»", "", "strconv")
	case esTexto(S) && esEntero(T):
		out = append(out, a.conError(x, "strconv.Atoi(%s)", escrito, "int")...)
	case esTexto(S) && esDecimal(T):
		out = append(out, a.conError(x, "strconv.ParseFloat(%s, 64)", escrito, "float64")...)
	case esNumero(S) && basico(T) != nil && basico(T).Info()&types.IsBoolean != 0:
		conv(envolverSiHaceFalta(txt, x)+" != 0", "condicion_booleana", "un número no es verdadero o falso: comparo con 0", "")
	case types.ConvertibleTo(S, T):
		conv(escrito+"("+txt+")", "convertir_tipo", "«"+txt+"» es "+a.escribirTipo(S)+" y aquí hace falta "+escrito+": lo convierto", "")
	}
	return out
}

// conError converts text x with a function that also returns an error (strconv.Atoi): the conversion
// goes on its own line before the statement, followed by "if err != nil { … }".
func (a *analisis) conError(x ast.Expr, plantilla, destino, base string) []edicion {
	stmt, _ := a.sentenciaEn(x.Pos())
	if stmt == nil {
		return nil
	}
	manejo, sup, ok := a.manejoError(x.Pos())
	if !ok {
		return nil
	}
	v := a.nombreLibre(x.Pos(), "n")
	ini := a.off(stmt.Pos())
	ind := indentacion(a.fuente, ini)
	llamada := strings.Replace(plantilla, "%s", a.texto(x), 1)
	uso := v
	if destino != base {
		uso = destino + "(" + v + ")"
	}
	errDecl := ":="
	if o := a.buscarVisible(x.Pos(), "err"); o != nil {
		if _, esVar := o.(*types.Var); esVar && esError(o.Type()) && a.buscarVisible(x.Pos(), v) == nil {
			errDecl = ":=" // v is new, so := still declares it and reuses err
		}
	}
	pre := v + ", err " + errDecl + " " + llamada + "\n" + ind + "if err != nil {\n" + ind + "\t" + manejo + "\n" + ind + "}\n" + ind
	texto := pre + a.fuente[ini:a.off(x.Pos())] + uso
	ed := a.ed(ini, a.off(x.End()), texto, "para pasar un texto a número se usa «"+strings.SplitN(plantilla, "(", 2)[0]+"», que también devuelve un error")
	ed.cambio.Regla = "texto_a_numero"
	ed.importar = []string{"strconv"}
	ed.suposicion = sup
	return []edicion{ed}
}

func reglaDosValores(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	stmt, _ := a.sentenciaEn(pos)
	var lhs ast.Expr
	var rhs ast.Expr
	tok := token.DEFINE
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return nil
		}
		lhs, rhs, tok = s.Lhs[0], s.Rhs[0], s.Tok
	case *ast.DeclStmt:
		g, ok := s.Decl.(*ast.GenDecl)
		if !ok || len(g.Specs) != 1 {
			return nil
		}
		vs, ok := g.Specs[0].(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type != nil {
			return nil
		}
		lhs, rhs, tok = vs.Names[0], vs.Values[0], token.VAR
	default:
		return nil
	}
	tupla, ok := a.tipoDe(rhs).(*types.Tuple)
	if !ok || tupla.Len() != 2 {
		return nil
	}
	ini, fin := a.off(stmt.Pos()), a.off(stmt.End())
	ind := indentacion(a.fuente, ini)
	izq, der := a.texto(lhs), a.texto(rhs)
	if !esError(tupla.At(1).Type()) {
		var texto string
		switch tok {
		case token.VAR:
			texto = "var " + izq + ", _ = " + der
		default:
			texto = izq + ", _ " + tok.String() + " " + der
		}
		ed := a.ed(ini, fin, texto, "«"+m[1]+"» devuelve 2 valores: el segundo lo descarto con «_»")
		ed.cambio.Regla = "descartar_valor"
		return []edicion{ed}
	}
	manejo, sup, ok := a.manejoError(pos)
	if !ok {
		return nil
	}
	si := "\n" + ind + "if err != nil {\n" + ind + "\t" + manejo + "\n" + ind + "}"
	var texto string
	switch tok {
	case token.VAR:
		texto = "var " + izq + ", err = " + der + si
	case token.DEFINE:
		texto = izq + ", err := " + der + si
	default:
		if o := a.buscarVisible(pos, "err"); o != nil && esError(o.Type()) {
			texto = izq + ", err = " + der + si
		} else {
			texto = "var err error\n" + ind + izq + ", err = " + der + si
		}
	}
	ed := a.ed(ini, fin, texto, "«"+m[1]+"» devuelve 2 valores: el resultado y un error, que hay que comprobar")
	ed.suposicion = sup
	return []edicion{ed}
}

func reglaValoresRetorno(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	var ret *ast.ReturnStmt
	for _, n := range a.camino(pos) {
		if r, ok := n.(*ast.ReturnStmt); ok {
			ret = r
		}
	}
	if ret == nil {
		return nil
	}
	ft, _ := a.funcionEn(ret.Pos())
	rs := a.resultados(ft)
	ceros, ok := a.cerosDe(ft)
	if !ok {
		return nil
	}
	var partes []string
	for _, r := range ret.Results {
		partes = append(partes, a.texto(r))
	}
	var porque string
	if m[1] == "not enough" {
		porque = "a este «return» le faltaban valores: la función devuelve " + strconv.Itoa(len(rs))
		var out []edicion
		// first: put each value where its type fits (return errors.New(…) → return 0, errors.New(…))
		if alineadas, ok := a.alinearResultados(ret.Results, rs, ceros); ok {
			out = append(out, a.reemplazar(ret, "return "+strings.Join(alineadas, ", "), porque))
		}
		for i := len(partes); i < len(rs); i++ {
			if i == len(rs)-1 && esError(rs[i]) {
				partes = append(partes, "nil")
			} else {
				partes = append(partes, ceros[i])
			}
		}
		simple := a.reemplazar(ret, "return "+strings.Join(partes, ", "), porque)
		if len(out) == 0 || out[0].texto != simple.texto {
			out = append(out, simple)
		}
		return out
	} else {
		if len(partes) > len(rs) {
			partes = partes[:len(rs)]
		}
		porque = "este «return» devolvía de más: la función devuelve " + strconv.Itoa(len(rs))
	}
	texto := "return"
	if len(partes) > 0 {
		texto += " " + strings.Join(partes, ", ")
	}
	return []edicion{a.reemplazar(ret, texto, porque)}
}

// alinearResultados places each returned value at the first remaining result position its type can be
// assigned to, and fills the other positions with zero values (nil for errors).
func (a *analisis) alinearResultados(tiene []ast.Expr, rs []types.Type, ceros []string) ([]string, bool) {
	out := make([]string, len(rs))
	j := 0
	for _, x := range tiene {
		t := a.tipoDe(x)
		if t == nil {
			return nil, false
		}
		for j < len(rs) && !types.AssignableTo(t, rs[j]) {
			j++
		}
		if j >= len(rs) {
			return nil, false
		}
		out[j] = a.texto(x)
		j++
	}
	for i := range out {
		if out[i] == "" {
			if esError(rs[i]) {
				out[i] = "nil"
			} else {
				out[i] = ceros[i]
			}
		}
	}
	return out, true
}

func (a *analisis) comparacionCero(x ast.Expr, op string) (string, bool) {
	t := a.tipoDe(x)
	if t == nil {
		return "", false
	}
	txt := envolverSiHaceFalta(a.texto(x), x)
	switch {
	case esNumero(t):
		return txt + " " + op + " 0", true
	case esTexto(t):
		return txt + " " + op + ` ""`, true
	case esNulable(t):
		return txt + " " + op + " nil", true
	}
	return "", false
}

func reglaCondicion(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	x := a.exprEn(a.posDe(e), "")
	if x == nil {
		return nil
	}
	texto, ok := a.comparacionCero(x, "!=")
	if !ok {
		return nil
	}
	return []edicion{a.reemplazar(x, texto, "la condición tiene que ser verdadero o falso: escribo «"+texto+"»")}
}

func reglaNegacion(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	var un *ast.UnaryExpr
	ast.Inspect(a.archivo, func(n ast.Node) bool {
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.NOT && (u.X.Pos() == pos || u.Pos() == pos) {
			un = u
		}
		return n != nil
	})
	if un == nil {
		return nil
	}
	texto, ok := a.comparacionCero(un.X, "==")
	if !ok {
		return nil
	}
	return []edicion{a.reemplazar(un, texto, "«!» solo vale con verdadero o falso: escribo «"+texto+"»")}
}

func reglaTiposDistintos(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	// the error points at one operand: take the binary expression around it with the reported text
	var bin, interior *ast.BinaryExpr
	quitar := func(s string) string { return strings.Join(strings.Fields(s), "") }
	for _, n := range a.camino(pos) {
		if b, ok := n.(*ast.BinaryExpr); ok {
			interior = b
			if quitar(a.texto(b)) == quitar(m[1]) || quitar(types.ExprString(b)) == quitar(m[1]) {
				bin = b
			}
		}
	}
	if bin == nil {
		bin = interior
	}
	if bin == nil {
		return nil
	}
	L, R := a.tipoDe(bin.X), a.tipoDe(bin.Y)
	if L == nil || R == nil {
		return nil
	}
	var out []edicion
	envolver := func(lado ast.Expr, pre, post, regla, porque string, importar ...string) {
		ed := a.reemplazar(lado, pre+a.texto(lado)+post, porque)
		ed.cambio.Regla = regla
		ed.importar = importar
		out = append(out, ed)
	}
	// c == "a" with c a rune or byte → 'a'
	for _, par := range [][2]ast.Expr{{bin.X, bin.Y}, {bin.Y, bin.X}} {
		otro, lit := par[0], par[1]
		if bl, ok := lit.(*ast.BasicLit); ok && bl.Kind == token.STRING && esRunaOByte(a.tipoDe(otro)) {
			if s, err := strconv.Unquote(bl.Value); err == nil && utf8.RuneCountInString(s) == 1 {
				q := strconv.QuoteRune([]rune(s)[0])
				ed := a.reemplazar(bl, q, "un solo carácter se escribe entre comillas simples: «"+q+"»")
				ed.cambio.Regla = "runa_literal"
				return []edicion{ed}
			}
		}
	}
	tl, tr := a.escribirTipo(L), a.escribirTipo(R)
	porque := "no se puede operar «" + tl + "» con «" + tr + "» directamente: convierto uno de los dos"
	switch {
	case esNumero(L) && esNumero(R):
		switch {
		case esDecimal(R) && !esDecimal(L):
			envolver(bin.X, tr+"(", ")", "tipos_distintos", porque)
		case esDecimal(L) && !esDecimal(R):
			envolver(bin.Y, tl+"(", ")", "tipos_distintos", porque)
		default:
			envolver(bin.Y, tl+"(", ")", "tipos_distintos", porque)
			envolver(bin.X, tr+"(", ")", "tipos_distintos", porque)
		}
	case esTexto(L) && esEntero(R) && !esRunaOByte(R):
		envolver(bin.Y, "strconv.Itoa(", ")", "tipos_distintos", "para juntar un número con un texto hay que pasarlo a texto con «strconv.Itoa»", "strconv")
		envolver(bin.Y, "fmt.Sprint(", ")", "tipos_distintos", "para juntar un número con un texto hay que pasarlo a texto", "fmt")
	case esEntero(L) && esTexto(R) && !esRunaOByte(L):
		envolver(bin.X, "strconv.Itoa(", ")", "tipos_distintos", "para juntar un número con un texto hay que pasarlo a texto con «strconv.Itoa»", "strconv")
		envolver(bin.X, "fmt.Sprint(", ")", "tipos_distintos", "para juntar un número con un texto hay que pasarlo a texto", "fmt")
	case esTexto(L) && esRunaOByte(R):
		envolver(bin.Y, "string(", ")", "tipos_distintos", "para juntar un carácter con un texto se usa «string(…)»")
	case esRunaOByte(L) && esTexto(R):
		envolver(bin.X, "string(", ")", "tipos_distintos", "para juntar un carácter con un texto se usa «string(…)»")
	case esTexto(L) && esDecimal(R):
		envolver(bin.Y, "fmt.Sprint(", ")", "tipos_distintos", "para juntar un decimal con un texto hay que pasarlo a texto", "fmt")
	case esDecimal(L) && esTexto(R):
		envolver(bin.X, "fmt.Sprint(", ")", "tipos_distintos", "para juntar un decimal con un texto hay que pasarlo a texto", "fmt")
	}
	return out
}

func reglaCampoMapa(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	stmt, _ := a.sentenciaEn(pos)
	var lhs ast.Expr
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if len(s.Lhs) == 1 {
			lhs = s.Lhs[0]
		}
	case *ast.IncDecStmt:
		lhs = s.X
	}
	sel, ok := lhs.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	ix, ok := sel.X.(*ast.IndexExpr)
	if !ok {
		return nil
	}
	copia := a.nombreLibre(pos, "copia")
	ini, fin := a.off(stmt.Pos()), a.off(stmt.End())
	ind := indentacion(a.fuente, ini)
	elem := a.texto(ix)
	resto := copia + a.fuente[a.off(ix.End()):fin]
	texto := copia + " := " + elem + "\n" + ind + resto + "\n" + ind + elem + " = " + copia
	return []edicion{a.ed(ini, fin, texto, "un campo de un valor guardado en un mapa no se puede cambiar directamente: lo copio, lo cambio y lo guardo otra vez")}
}

func reglaSinUsar(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	pos := a.posDe(e)
	stmt, _ := a.sentenciaEn(pos)
	es, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	bin, ok := es.X.(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	switch bin.Op {
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
	default:
		return nil
	}
	switch bin.X.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr:
	default:
		return nil
	}
	texto := a.texto(bin.X) + " " + bin.Op.String() + "= " + a.texto(bin.Y)
	return []edicion{a.reemplazar(es, texto, "calculabas «"+a.texto(bin)+"» pero no guardabas el resultado: escribo «"+texto+"»")}
}

func reglaIndiceEntero(a *analisis, e nucleo.ErrorGo, m []string) []edicion {
	x := a.exprEn(a.posDe(e), m[1])
	if x == nil {
		return nil
	}
	if tv, ok := a.info.Types[x]; ok && tv.Value != nil && tv.Value.Kind() == constant.Float {
		return nil
	}
	return []edicion{a.reemplazar(x, "int("+a.texto(x)+")", "la posición de una lista tiene que ser un número entero: la convierto con «int(…)»")}
}
