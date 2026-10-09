package analisis

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// hallazgos runs every static check on the file.
func hallazgos(a *archivo) []Hallazgo {
	h := &buscador{a: a}
	h.mapasNil()
	h.erroresIgnorados()
	h.porFuncion()
	h.inalcanzable()
	return h.out
}

type buscador struct {
	a     *archivo
	out   []Hallazgo
	visto map[string]bool
}

func (h *buscador) add(p token.Pos, clave, gravedad, mensaje, consejo string) {
	l := h.a.linea(p)
	k := clave + "@" + strconv.Itoa(l)
	if h.visto == nil {
		h.visto = map[string]bool{}
	}
	if h.visto[k] {
		return
	}
	h.visto[k] = true
	h.out = append(h.out, Hallazgo{Linea: l, Clave: clave, Gravedad: gravedad, Mensaje: mensaje, Consejo: consejo})
}

// pila is the chain of ancestors while walking.
type pila []ast.Node

// recorrer walks n calling f with the ancestors of each node (innermost last, the node excluded).
func recorrer(n ast.Node, f func(x ast.Node, anc pila) bool) {
	var anc pila
	ast.Inspect(n, func(x ast.Node) bool {
		if x == nil {
			anc = anc[:len(anc)-1]
			return false
		}
		seguir := f(x, anc)
		if seguir {
			anc = append(anc, x)
		}
		return seguir
	})
}

// enBucle reports whether the innermost enclosing function-or-loop is a loop.
func (p pila) enBucle() bool {
	for i := len(p) - 1; i >= 0; i-- {
		switch p[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		}
	}
	return false
}

func (p pila) funcion() *ast.FuncDecl {
	for i := len(p) - 1; i >= 0; i-- {
		if fd, ok := p[i].(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

func esFloat(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsFloat != 0
}

func esEntero(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

func esString(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

func esTipoError(t types.Type) bool {
	return t != nil && types.Identical(t, types.Universe.Lookup("error").Type())
}

// ---- mapa_nil ----

func (h *buscador) mapasNil() {
	// package-level and function-level `var m map[K]V` without a value
	type decl struct {
		obj    types.Object
		nombre string
		ambito ast.Node
	}
	var decls []decl
	ast.Inspect(h.a.f, func(n ast.Node) bool {
		gd, ok := n.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			return true
		}
		for _, sp := range gd.Specs {
			vs := sp.(*ast.ValueSpec)
			if len(vs.Values) != 0 || vs.Type == nil {
				continue
			}
			if _, ok := vs.Type.(*ast.MapType); !ok {
				if t := h.a.tipo(vs.Type); t == nil {
					continue
				} else if _, ok := t.Underlying().(*types.Map); !ok {
					continue
				}
			}
			for _, id := range vs.Names {
				decls = append(decls, decl{obj: h.a.objeto(id), nombre: id.Name})
			}
		}
		return true
	})
	for _, d := range decls {
		mismo := func(e ast.Expr) bool {
			id, ok := quitarParen(e).(*ast.Ident)
			if !ok || id.Name != d.nombre {
				return false
			}
			if d.obj != nil && h.a.info != nil {
				return h.a.objeto(id) == d.obj
			}
			return true
		}
		creado := false
		var declPos token.Pos
		if d.obj != nil {
			declPos = d.obj.Pos()
		}
		ast.Inspect(h.a.f, func(n ast.Node) bool {
			if creado || n == nil {
				return false
			}
			if declPos.IsValid() && n.End() < declPos {
				return true
			}
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if mismo(l) {
						creado = true
						return false
					}
				}
				for _, l := range x.Lhs {
					if ix, ok := l.(*ast.IndexExpr); ok && mismo(ix.X) {
						h.add(x.Pos(), "mapa_nil", "error",
							"Escribes en el mapa `"+d.nombre+"`, pero nunca lo creas: vale nil y el programa se parará con un pánico.",
							"Créalo antes de usarlo: `"+d.nombre+" = make("+tipoTexto(h.a, d.obj)+")`, o decláralo con `"+d.nombre+" := map…{}`.")
						creado = true
						return false
					}
				}
			case *ast.IncDecStmt:
				if ix, ok := x.X.(*ast.IndexExpr); ok && mismo(ix.X) {
					h.add(x.Pos(), "mapa_nil", "error",
						"Escribes en el mapa `"+d.nombre+"`, pero nunca lo creas: vale nil y el programa se parará con un pánico.",
						"Créalo antes de usarlo: `"+d.nombre+" = make("+tipoTexto(h.a, d.obj)+")`.")
					creado = true
					return false
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND && mismo(x.X) {
					creado = true // &m: someone may fill it (json.Unmarshal)
					return false
				}
			}
			return true
		})
	}
}

func tipoTexto(a *archivo, o types.Object) string {
	if o == nil {
		return "map[…]…"
	}
	return types.TypeString(o.Type(), func(p *types.Package) string {
		if p == a.pkg {
			return ""
		}
		return p.Name()
	})
}

// ---- error_ignorado ----

// sinComprobarOK are calls whose error is customarily ignored.
func errorIgnorable(a *archivo, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		if pk, ok := a.objeto(id).(*types.PkgName); ok && pk.Imported().Path() == "fmt" {
			return true
		}
	}
	if s := a.info.Selections[sel]; s != nil {
		t := s.Recv()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil {
			full := n.Obj().Pkg().Path() + "." + n.Obj().Name()
			switch full {
			case "strings.Builder", "bytes.Buffer", "hash.Hash", "crypto/sha256.digest":
				return true
			}
		}
	}
	return false
}

func nombreLlamada(call *ast.CallExpr) string {
	return codigo(call.Fun)
}

func (h *buscador) erroresIgnorados() {
	if h.a.info == nil {
		return
	}
	resultados := func(call *ast.CallExpr) []types.Type {
		t := h.a.tipo(call)
		if t == nil {
			return nil
		}
		if tu, ok := t.(*types.Tuple); ok {
			out := make([]types.Type, tu.Len())
			for i := range out {
				out[i] = tu.At(i).Type()
			}
			return out
		}
		return []types.Type{t}
	}
	consejo := func(call *ast.CallExpr) string {
		return "Guárdalo y míralo: `…, err := " + nombreLlamada(call) + "(…)` y después `if err != nil { … }`."
	}
	ast.Inspect(h.a.f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ExprStmt:
			call, ok := quitarParen(x.X).(*ast.CallExpr)
			if !ok || errorIgnorable(h.a, call) {
				return true
			}
			for _, t := range resultados(call) {
				if esTipoError(t) {
					h.add(x.Pos(), "error_ignorado", "aviso",
						"No compruebas el error que devuelve `"+nombreLlamada(call)+"`: si algo falla, no te enterarás.", consejo(call))
					break
				}
			}
		case *ast.AssignStmt:
			if len(x.Rhs) != 1 {
				return true
			}
			call, ok := quitarParen(x.Rhs[0]).(*ast.CallExpr)
			if !ok || errorIgnorable(h.a, call) {
				return true
			}
			rs := resultados(call)
			if len(rs) != len(x.Lhs) {
				return true
			}
			for i, t := range rs {
				if id, ok := x.Lhs[i].(*ast.Ident); ok && id.Name == "_" && esTipoError(t) {
					h.add(x.Pos(), "error_ignorado", "aviso",
						"Tiras a la basura (`_`) el error que devuelve `"+nombreLlamada(call)+"`.", consejo(call))
				}
			}
		case *ast.ValueSpec:
			if len(x.Values) != 1 {
				return true
			}
			call, ok := quitarParen(x.Values[0]).(*ast.CallExpr)
			if !ok {
				return true
			}
			rs := resultados(call)
			if len(rs) != len(x.Names) {
				return true
			}
			for i, t := range rs {
				if x.Names[i].Name == "_" && esTipoError(t) {
					h.add(x.Pos(), "error_ignorado", "aviso",
						"Tiras a la basura (`_`) el error que devuelve `"+nombreLlamada(call)+"`.", consejo(call))
				}
			}
		}
		return true
	})
}

// ---- checks done per function ----

func (h *buscador) porFuncion() {
	for _, d := range h.a.f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		h.enFuncion(fd)
		h.divisiones(fd)
		h.indicesMasUno(fd)
		h.sombreados(fd)
	}
}

func (h *buscador) enFuncion(fd *ast.FuncDecl) {
	recorrer(fd, func(n ast.Node, anc pila) bool {
		switch x := n.(type) {
		case *ast.DeferStmt:
			if anc.enBucle() {
				h.add(x.Pos(), "defer_en_bucle", "aviso",
					"Este `defer` está dentro de un bucle: no se ejecuta en cada vuelta, sino todos juntos al final de la función.",
					"Mete el cuerpo del bucle en una función aparte, o llama a la limpieza (por ejemplo `f.Close()`) al final de cada vuelta.")
			}
		case *ast.BinaryExpr:
			if (x.Op == token.EQL || x.Op == token.NEQ) && (esFloat(h.a.tipo(x.X)) || esFloat(h.a.tipo(x.Y))) {
				if !esCeroConst(h.a, x.X) && !esCeroConst(h.a, x.Y) && !(esConstanteTV(h.a, x.X) && esConstanteTV(h.a, x.Y)) {
					h.add(x.Pos(), "float_igual", "aviso",
						"Comparas números decimales con `"+x.Op.String()+"`: por el redondeo, 0.1+0.2 no es exactamente 0.3.",
						"Compara con un margen: `math.Abs(a-b) < 1e-9`.")
				}
			}
		case *ast.AssignStmt:
			if anc.enBucle() && len(x.Lhs) == 1 && len(x.Rhs) == 1 && esString(h.a.tipo(x.Lhs[0])) {
				concat := x.Tok == token.ADD_ASSIGN
				if x.Tok == token.ASSIGN {
					if b, ok := quitarParen(x.Rhs[0]).(*ast.BinaryExpr); ok && b.Op == token.ADD && codigo(b.X) == codigo(x.Lhs[0]) {
						concat = true
					}
				}
				if concat {
					v := codigo(x.Lhs[0])
					h.add(x.Pos(), "concat_en_bucle", "consejo",
						"Vas pegando texto a `"+v+"` en un bucle: cada vez Go copia el texto entero, y con muchos datos va lento.",
						"Usa `strings.Builder`: `var sb strings.Builder` antes del bucle, `sb.WriteString(…)` dentro y `"+v+" = sb.String()` al final.")
				}
			}
		case *ast.ForStmt:
			if x.Cond == nil && !tieneSalida(x) {
				h.add(x.Pos(), "bucle_sin_salida", "aviso",
					"Este bucle `for { … }` no tiene `break` ni `return`: no termina nunca.",
					"Añade una condición de salida: `if terminado { break }`, o escribe la condición en el propio `for`.")
			}
		case *ast.RangeStmt:
			h.copiaGrande(x)
		case *ast.GoStmt:
			h.capturas(x, anc)
		}
		return true
	})
}

func esConstanteTV(a *archivo, e ast.Expr) bool {
	if a.info == nil {
		return false
	}
	tv, ok := a.info.Types[e]
	return ok && tv.Value != nil
}

func esCeroConst(a *archivo, e ast.Expr) bool {
	if a.info != nil {
		if tv, ok := a.info.Types[e]; ok && tv.Value != nil {
			return tv.Value.String() == "0"
		}
	}
	if b, ok := quitarParen(e).(*ast.BasicLit); ok {
		v := strings.TrimRight(strings.TrimRight(b.Value, "0"), ".")
		return v == "" || v == "0"
	}
	return false
}

// tieneSalida reports whether the infinite loop can end: return, a break that targets it, goto,
// panic, os.Exit or log.Fatal*.
func tieneSalida(f *ast.ForStmt) bool {
	sale := false
	var etiqueta string
	_ = etiqueta
	var visitar func(n ast.Node, dentroDeOtro bool)
	visitar = func(n ast.Node, dentroDeOtro bool) {
		ast.Inspect(n, func(x ast.Node) bool {
			if sale {
				return false
			}
			switch y := x.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				sale = true
			case *ast.BranchStmt:
				switch y.Tok {
				case token.GOTO:
					sale = true
				case token.BREAK:
					if y.Label != nil || !dentroDeOtro {
						sale = true
					}
				}
			case *ast.CallExpr:
				switch codigo(y.Fun) {
				case "panic", "os.Exit", "log.Fatal", "log.Fatalf", "log.Fatalln", "log.Panic", "log.Panicf":
					sale = true
				}
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				if x != n {
					var cuerpo *ast.BlockStmt
					switch z := y.(type) {
					case *ast.ForStmt:
						cuerpo = z.Body
					case *ast.RangeStmt:
						cuerpo = z.Body
					case *ast.SwitchStmt:
						cuerpo = z.Body
					case *ast.TypeSwitchStmt:
						cuerpo = z.Body
					case *ast.SelectStmt:
						cuerpo = z.Body
					}
					visitar(cuerpo, true)
					return false
				}
			}
			return true
		})
	}
	visitar(f.Body, false)
	return sale
}

// ---- div_sin_comprobar ----

func (h *buscador) divisiones(fd *ast.FuncDecl) {
	if fd.Type.Params == nil {
		return
	}
	params := map[string]types.Object{}
	for _, campo := range fd.Type.Params.List {
		for _, id := range campo.Names {
			t := h.a.tipo(id)
			if t == nil {
				if tid, ok := campo.Type.(*ast.Ident); ok && strings.HasPrefix(tid.Name, "int") || strings.HasPrefix(codigo(campo.Type), "uint") {
					params[id.Name] = nil
				}
				continue
			}
			if esEntero(t) {
				params[id.Name] = h.a.objeto(id)
			}
		}
	}
	if len(params) == 0 {
		return
	}
	comparado := map[string]bool{}
	esParam := func(e ast.Expr) (string, bool) {
		id, ok := quitarParen(e).(*ast.Ident)
		if !ok {
			return "", false
		}
		o, ok := params[id.Name]
		if !ok {
			return "", false
		}
		if o != nil && h.a.objeto(id) != o {
			return "", false
		}
		return id.Name, true
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			switch x.Op {
			case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
				if p, ok := esParam(x.X); ok {
					comparado[p] = true
				}
				if p, ok := esParam(x.Y); ok {
					comparado[p] = true
				}
			}
		case *ast.SwitchStmt:
			if p, ok := esParam(x.Tag); ok {
				comparado[p] = true
			}
		case *ast.AssignStmt:
			for _, l := range x.Lhs { // reassigned: we cannot tell
				if p, ok := esParam(l); ok && x.Tok == token.ASSIGN {
					comparado[p] = true
				}
			}
		}
		return true
	})
	avisar := func(pos token.Pos, p string) {
		if comparado[p] {
			return
		}
		h.add(pos, "div_sin_comprobar", "aviso",
			"Divides entre `"+p+"` sin mirar antes si es 0: con 0 el programa se para con un pánico.",
			"Compruébalo antes: `if "+p+" == 0 { return … }`.")
		comparado[p] = true
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if x.Op == token.QUO || x.Op == token.REM {
				if p, ok := esParam(x.Y); ok {
					avisar(x.Pos(), p)
				}
			}
		case *ast.AssignStmt:
			if (x.Tok == token.QUO_ASSIGN || x.Tok == token.REM_ASSIGN) && len(x.Rhs) == 1 {
				if p, ok := esParam(x.Rhs[0]); ok {
					avisar(x.Pos(), p)
				}
			}
		}
		return true
	})
}

// ---- indice_mas_uno ----

func (h *buscador) indicesMasUno(fd *ast.FuncDecl) {
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		var i, s string
		var cuerpo *ast.BlockStmt
		switch x := n.(type) {
		case *ast.ForStmt:
			b, ok := quitarParen(x.Cond).(*ast.BinaryExpr)
			if !ok {
				return true
			}
			id, ok := quitarParen(b.X).(*ast.Ident)
			if !ok {
				return true
			}
			call, ok := quitarParen(b.Y).(*ast.CallExpr)
			if b.Op == token.LSS && ok && esLen(call) {
				i, s = id.Name, codigo(call.Args[0])
			} else if bb, ok2 := quitarParen(b.Y).(*ast.BinaryExpr); b.Op == token.LEQ && ok2 && bb.Op == token.SUB && esUno(bb.Y) {
				if call, ok := quitarParen(bb.X).(*ast.CallExpr); ok && esLen(call) {
					i, s = id.Name, codigo(call.Args[0])
				}
			}
			cuerpo = x.Body
		case *ast.RangeStmt:
			if id, ok := x.Key.(*ast.Ident); ok && id.Name != "_" {
				i, s = id.Name, codigo(x.X)
			}
			cuerpo = x.Body
		default:
			return true
		}
		if i == "" {
			return true
		}
		recorrer(cuerpo, func(m ast.Node, anc pila) bool {
			ix, ok := m.(*ast.IndexExpr)
			if !ok || codigo(ix.X) != s {
				return true
			}
			b, ok := quitarParen(ix.Index).(*ast.BinaryExpr)
			if !ok || b.Op != token.ADD {
				return true
			}
			id, ok := quitarParen(b.X).(*ast.Ident)
			if !ok || id.Name != i || !esLiteralEntero(b.Y) {
				return true
			}
			for _, a := range anc { // guarded by an if that mentions len(s)?
				if ifs, ok := a.(*ast.IfStmt); ok && strings.Contains(codigo(ifs.Cond), "len("+s+")") {
					return true
				}
			}
			h.add(ix.Pos(), "indice_mas_uno", "error",
				"Cuando `"+i+"` llegue al último elemento, `"+codigo(ix)+"` se sale de `"+s+"` (index out of range).",
				"Recorre hasta el penúltimo: `for "+i+" := 0; "+i+" < len("+s+")-1; "+i+"++`.")
			return true
		})
		return true
	})
}

func esLen(call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "len" && len(call.Args) == 1
}

func esUno(e ast.Expr) bool {
	b, ok := quitarParen(e).(*ast.BasicLit)
	return ok && b.Value == "1"
}

// ---- codigo_inalcanzable ----

func termina(s ast.Stmt) bool {
	switch x := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return x.Tok != token.FALLTHROUGH
	case *ast.ExprStmt:
		if call, ok := x.X.(*ast.CallExpr); ok {
			switch codigo(call.Fun) {
			case "panic", "os.Exit", "log.Fatal", "log.Fatalf", "log.Fatalln":
				return true
			}
		}
	}
	return false
}

func (h *buscador) inalcanzable() {
	revisar := func(l []ast.Stmt) {
		for i := 0; i+1 < len(l); i++ {
			if !termina(l[i]) {
				continue
			}
			sig := l[i+1]
			if _, ok := sig.(*ast.LabeledStmt); ok {
				continue
			}
			if _, ok := sig.(*ast.EmptyStmt); ok {
				continue
			}
			que := "un `return`"
			switch x := l[i].(type) {
			case *ast.BranchStmt:
				que = "un `" + x.Tok.String() + "`"
			case *ast.ExprStmt:
				que = "`" + codigo(x.X.(*ast.CallExpr).Fun) + "(…)`"
			}
			h.add(sig.Pos(), "codigo_inalcanzable", "aviso",
				"Este código nunca se ejecuta: va justo después de "+que+".",
				"Bórralo, o muévelo antes de "+que+".")
			return
		}
	}
	ast.Inspect(h.a.f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BlockStmt:
			revisar(x.List)
		case *ast.CaseClause:
			revisar(x.Body)
		case *ast.CommClause:
			revisar(x.Body)
		}
		return true
	})
}

// ---- err_sombreado ----

func (h *buscador) sombreados(fd *ast.FuncDecl) {
	if h.a.info == nil {
		return
	}
	recorrer(fd.Body, func(n ast.Node, anc pila) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(anc) == 0 {
			return true
		}
		// only := statements that stand in a block (not the init of if/for/switch)
		switch anc[len(anc)-1].(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		default:
			return true
		}
		if len(anc) < 2 {
			return true // the function body itself
		}
		for _, l := range as.Lhs {
			id, ok := l.(*ast.Ident)
			if !ok || id.Name != "err" {
				continue
			}
			obj := h.a.info.Defs[id]
			if obj == nil || obj.Parent() == nil || obj.Parent().Parent() == nil {
				continue
			}
			_, fuera := obj.Parent().Parent().LookupParent("err", as.Pos())
			if fuera == nil || fuera.Pos() < fd.Pos() || fuera.Pos() > fd.End() {
				continue
			}
			if _, ok := fuera.(*types.Var); !ok {
				continue
			}
			h.add(as.Pos(), "err_sombreado", "aviso",
				"Aquí `:=` crea un `err` nuevo que tapa al de fuera (línea "+strconv.Itoa(h.a.linea(fuera.Pos()))+"): el `err` de fuera no cambia.",
				"Si querías usar el de fuera, escribe `=` en lugar de `:=` (y declara antes las otras variables con `var`).")
		}
		return true
	})
}

// ---- rango_copia_grande ----

var tamanos = types.SizesFor("gc", "amd64")

func (h *buscador) copiaGrande(r *ast.RangeStmt) {
	if r.Value == nil {
		return
	}
	if id, ok := r.Value.(*ast.Ident); ok && id.Name == "_" {
		return
	}
	t := h.a.tipo(r.X)
	if t == nil {
		return
	}
	var elem types.Type
	switch u := t.Underlying().(type) {
	case *types.Slice:
		elem = u.Elem()
	case *types.Array:
		elem = u.Elem()
	case *types.Pointer:
		if a, ok := u.Elem().Underlying().(*types.Array); ok {
			elem = a.Elem()
		}
	}
	if elem == nil {
		return
	}
	if _, ok := elem.Underlying().(*types.Struct); !ok {
		return
	}
	tam := tamanos.Sizeof(elem)
	if tam < 128 {
		return
	}
	v := codigo(r.Value)
	h.add(r.Pos(), "rango_copia_grande", "consejo",
		"En cada vuelta se copia `"+v+"`, una estructura de "+strconv.FormatInt(tam, 10)+" bytes.",
		"Recorre con el índice y usa un puntero: `for i := range "+codigo(r.X)+" { "+v+" := &"+codigo(r.X)+"[i] … }`.")
}

// ---- variable_bucle_capturada ----

func (h *buscador) capturas(g *ast.GoStmt, anc pila) {
	lit, ok := g.Call.Fun.(*ast.FuncLit)
	if !ok {
		return
	}
	vars := map[types.Object]string{}
	nombres := map[string]bool{}
	for i := len(anc) - 1; i >= 0; i-- {
		switch x := anc[i].(type) {
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
					vars[h.a.objeto(id)] = id.Name
					nombres[id.Name] = true
				}
			}
		case *ast.ForStmt:
			if as, ok := x.Init.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
				for _, l := range as.Lhs {
					if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
						vars[h.a.objeto(id)] = id.Name
						nombres[id.Name] = true
					}
				}
			}
		case *ast.FuncDecl, *ast.FuncLit:
			i = -1
		}
	}
	if len(vars) == 0 {
		return
	}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || !nombres[id.Name] {
			return true
		}
		o := h.a.objeto(id)
		nombre, es := vars[o]
		if !es || (o == nil && h.a.info != nil) {
			return true
		}
		h.add(g.Pos(), "variable_bucle_capturada", "consejo",
			"La goroutine usa la variable del bucle `"+nombre+"`. Desde Go 1.22 cada vuelta tiene su propia copia, así que aquí es seguro.",
			"En versiones antiguas de Go, pásala como argumento: `go func("+nombre+" …) { … }("+nombre+")`.")
		return false
	})
}
