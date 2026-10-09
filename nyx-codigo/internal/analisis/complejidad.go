package analisis

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// ---- cyclomatic complexity and nesting ----

// ciclomatica is decisions + 1: if, for, range, non-default case and comm clauses, && and ||.
func ciclomatica(fd *ast.FuncDecl) int {
	n := 1
	if fd.Body == nil {
		return n
	}
	ast.Inspect(fd.Body, func(x ast.Node) bool {
		switch y := x.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			n++
		case *ast.CaseClause:
			if y.List != nil {
				n++
			}
		case *ast.CommClause:
			if y.Comm != nil {
				n++
			}
		case *ast.BinaryExpr:
			if y.Op == token.LAND || y.Op == token.LOR {
				n++
			}
		}
		return true
	})
	return n
}

// anidamiento is the deepest nesting of control statements (if, for, range, switch, select); an
// "else if" chain counts as one level.
func anidamiento(n ast.Node) int {
	if n == nil {
		return 0
	}
	max := 0
	var visitar func(x ast.Node, prof int)
	var hijos func(x ast.Node, prof int)
	hijos = func(x ast.Node, prof int) {
		ast.Inspect(x, func(y ast.Node) bool {
			if y == nil || y == x {
				return true
			}
			switch y.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				visitar(y, prof)
				return false
			}
			return true
		})
	}
	visitar = func(x ast.Node, prof int) {
		prof++
		if prof > max {
			max = prof
		}
		if is, ok := x.(*ast.IfStmt); ok {
			hijos(is.Body, prof)
			switch e := is.Else.(type) {
			case *ast.IfStmt:
				visitar(e, prof-1)
			case *ast.BlockStmt:
				hijos(e, prof)
			}
			return
		}
		hijos(x, prof)
	}
	hijos(n, 0)
	return max
}

// ---- asymptotic order ----

// orden is n^(p2/2) · log^lg n.
type orden struct{ p2, lg int }

func (o orden) mayor(u orden) bool {
	if o.p2 != u.p2 {
		return o.p2 > u.p2
	}
	return o.lg > u.lg
}

func (o orden) por(u orden) orden { return orden{o.p2 + u.p2, o.lg + u.lg} }

var superindices = []string{"⁰", "¹", "²", "³", "⁴", "⁵", "⁶", "⁷", "⁸", "⁹"}

func superindice(k int) string {
	s := strconv.Itoa(k)
	var sb strings.Builder
	for _, c := range s {
		sb.WriteString(superindices[c-'0'])
	}
	return sb.String()
}

func (o orden) String() string {
	var partes []string
	k := o.p2 / 2
	base := ""
	switch {
	case k == 1:
		base = "n"
	case k > 1:
		base = "n" + superindice(k)
	}
	if o.p2%2 == 1 {
		base += "√n"
	}
	if base != "" {
		partes = append(partes, base)
	}
	switch {
	case o.lg == 1:
		partes = append(partes, "log n")
	case o.lg > 1:
		partes = append(partes, "log"+superindice(o.lg)+" n")
	}
	if len(partes) == 0 {
		return "O(1)"
	}
	return "O(" + strings.Join(partes, " ") + ")"
}

var (
	o1     = orden{}
	oLog   = orden{0, 1}
	oRaiz  = orden{1, 0}
	oLin   = orden{2, 0}
	oNLogN = orden{2, 1}
)

// coste is an order plus the pieces that explain it (outermost first).
type coste struct {
	o      orden
	piezas []string
}

func (c coste) mayor(u coste) bool { return c.o.mayor(u.o) }

func maxCoste(cs ...coste) coste {
	var m coste
	for _, c := range cs {
		if c.mayor(m) {
			m = c
		}
	}
	return m
}

// resultadoFunc is the complexity verdict for one function.
type resultadoFunc struct {
	recursiva, exponencial bool
	orden                  orden
	motivo                 string
	ordenTxt               string
}

type complejidad struct {
	a       *archivo
	funcs   map[string]*ast.FuncDecl // plain functions and "T.M" methods
	memo    map[*ast.FuncDecl]*resultadoFunc
	enCurso map[*ast.FuncDecl]bool
}

func nuevaComplejidad(a *archivo) *complejidad {
	c := &complejidad{a: a, funcs: map[string]*ast.FuncDecl{}, memo: map[*ast.FuncDecl]*resultadoFunc{}, enCurso: map[*ast.FuncDecl]bool{}}
	for _, d := range a.f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name != nil && fd.Body != nil {
			c.funcs[nombreFunc(fd)] = fd
		}
	}
	return c
}

// llamada returns the local function called by call (nil if none or unknown).
func (c *complejidad) llamada(call *ast.CallExpr, desde *ast.FuncDecl) *ast.FuncDecl {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		if fd, ok := c.funcs[f.Name]; ok && fd.Recv == nil {
			if c.a.info != nil {
				if o := c.a.info.Uses[f]; o != nil {
					if def := c.a.info.Defs[fd.Name]; def != nil && def != o {
						return nil
					}
				}
			}
			return fd
		}
	case *ast.SelectorExpr:
		if desde != nil && desde.Recv != nil && len(desde.Recv.List) > 0 {
			if id, ok := f.X.(*ast.Ident); ok && len(desde.Recv.List[0].Names) > 0 && desde.Recv.List[0].Names[0].Name == id.Name {
				if fd, ok := c.funcs[nombreReceptor(desde.Recv.List[0].Type)+"."+f.Sel.Name]; ok {
					return fd
				}
			}
		}
	}
	return nil
}

// alcanza reports whether from calls to (directly or indirectly).
func (c *complejidad) alcanza(desde, hasta *ast.FuncDecl) bool {
	visto := map[*ast.FuncDecl]bool{}
	var ir func(f *ast.FuncDecl) bool
	ir = func(f *ast.FuncDecl) bool {
		if visto[f] {
			return false
		}
		visto[f] = true
		encontrado := false
		ast.Inspect(f.Body, func(n ast.Node) bool {
			if encontrado {
				return false
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if g := c.llamada(call, f); g != nil {
					if g == hasta || ir(g) {
						encontrado = true
					}
				}
			}
			return true
		})
		return encontrado
	}
	return ir(desde)
}

// de computes (and caches) the verdict for fd.
func (c *complejidad) de(fd *ast.FuncDecl) *resultadoFunc {
	if r, ok := c.memo[fd]; ok {
		return r
	}
	if fd.Body == nil {
		r := &resultadoFunc{motivo: "no tiene cuerpo"}
		c.memo[fd] = r
		return r
	}
	if c.enCurso[fd] {
		return &resultadoFunc{} // mutual recursion in progress: counted by the caller
	}
	c.enCurso[fd] = true
	defer delete(c.enCurso, fd)

	r := &resultadoFunc{}
	r.recursiva = c.alcanza(fd, fd)
	cuerpo := c.costeLista(fd.Body.List, fd)
	if r.recursiva {
		propias := c.caminoMax(fd.Body, fd)
		memo := c.memoriza(fd)
		mitades := c.partePorMitades(fd)
		switch {
		case propias >= 2 && !memo && mitades:
			if cuerpo.o.p2 >= 2 {
				r.orden = oNLogN
			} else {
				r.orden = oLin
			}
			r.motivo = "divide la entrada en mitades y se llama a sí misma con cada mitad"
		case propias >= 2 && !memo:
			r.exponencial = true
			r.motivo = "se llama a sí misma " + veces(propias) + " en cada paso sin guardar los resultados"
		case memo:
			r.orden = maxCoste(cuerpo, coste{o: oLin}).o
			r.motivo = "se llama a sí misma, pero guarda los resultados que ya calculó"
		case mitades:
			r.orden = oLog.por(cuerpo.o)
			r.motivo = "se llama a sí misma con la mitad de la entrada"
		default:
			r.orden = oLin.por(cuerpo.o)
			r.motivo = "se llama a sí misma con una entrada un poco menor"
			if len(cuerpo.piezas) > 0 {
				r.motivo += ", y en cada llamada hay " + strings.Join(cuerpo.piezas, ", y dentro, ")
			}
		}
	} else {
		r.orden = cuerpo.o
		r.motivo = motivoDe(cuerpo)
	}
	r.ordenTxt = r.orden.String()
	if r.exponencial {
		r.ordenTxt = "O(2ⁿ)?"
	}
	c.memo[fd] = r
	return r
}

func (r *resultadoFunc) String() string { return r.ordenTxt }

func veces(n int) string {
	switch n {
	case 2:
		return "dos veces"
	case 3:
		return "tres veces"
	}
	return strconv.Itoa(n) + " veces"
}

var numerales = []string{"", "un", "dos", "tres", "cuatro", "cinco"}

func motivoDe(c coste) string {
	if c.o == o1 || len(c.piezas) == 0 {
		return "no tiene bucles ni llamadas que dependan del tamaño de la entrada"
	}
	lineales := true
	for _, p := range c.piezas {
		if !strings.HasPrefix(p, "un bucle que recorre") {
			lineales = false
		}
	}
	if lineales && len(c.piezas) > 1 && len(c.piezas) < len(numerales) {
		return numerales[len(c.piezas)] + " bucles anidados que recorren la entrada"
	}
	return strings.Join(c.piezas, ", y dentro, ")
}

// costeLista is the cost of a statement list: the most expensive statement.
func (c *complejidad) costeLista(l []ast.Stmt, fd *ast.FuncDecl) coste {
	var m coste
	for _, s := range l {
		if k := c.costeStmt(s, fd); k.mayor(m) {
			m = k
		}
	}
	return m
}

func (c *complejidad) costeStmt(s ast.Stmt, fd *ast.FuncDecl) coste {
	switch x := s.(type) {
	case nil:
		return coste{}
	case *ast.BlockStmt:
		return c.costeLista(x.List, fd)
	case *ast.LabeledStmt:
		return c.costeStmt(x.Stmt, fd)
	case *ast.IfStmt:
		k := maxCoste(c.costeStmt(x.Init, fd), c.costeExpr(x.Cond, fd), c.costeLista(x.Body.List, fd))
		if x.Else != nil {
			k = maxCoste(k, c.costeStmt(x.Else, fd))
		}
		return k
	case *ast.SwitchStmt:
		k := maxCoste(c.costeStmt(x.Init, fd), c.costeExpr(x.Tag, fd))
		for _, cc := range x.Body.List {
			k = maxCoste(k, c.costeLista(cc.(*ast.CaseClause).Body, fd))
		}
		return k
	case *ast.TypeSwitchStmt:
		var k coste
		for _, cc := range x.Body.List {
			k = maxCoste(k, c.costeLista(cc.(*ast.CaseClause).Body, fd))
		}
		return k
	case *ast.SelectStmt:
		var k coste
		for _, cc := range x.Body.List {
			k = maxCoste(k, c.costeLista(cc.(*ast.CommClause).Body, fd))
		}
		return k
	case *ast.ForStmt:
		o, desc := c.costeBucleFor(x)
		dentro := c.costeLista(x.Body.List, fd)
		return maxCoste(componer(o, desc, dentro), c.costeExpr(x.Cond, fd))
	case *ast.RangeStmt:
		o, desc := c.costeBucleRange(x)
		dentro := c.costeLista(x.Body.List, fd)
		return maxCoste(componer(o, desc, dentro), c.costeExpr(x.X, fd))
	}
	return c.costeExpr(s, fd)
}

func componer(o orden, desc string, dentro coste) coste {
	k := coste{o: o.por(dentro.o)}
	if o != o1 {
		k.piezas = append(k.piezas, desc)
	}
	k.piezas = append(k.piezas, dentro.piezas...)
	return k
}

// llamadasCaras are library calls whose cost grows with the input.
var llamadasCaras = map[string]orden{
	"sort.Ints": oNLogN, "sort.Strings": oNLogN, "sort.Float64s": oNLogN, "sort.Slice": oNLogN,
	"sort.SliceStable": oNLogN, "sort.Sort": oNLogN, "sort.Stable": oNLogN,
	"slices.Sort": oNLogN, "slices.SortFunc": oNLogN, "slices.SortStableFunc": oNLogN,
	"slices.Contains": oLin, "slices.Index": oLin, "slices.ContainsFunc": oLin, "slices.IndexFunc": oLin,
	"slices.Max": oLin, "slices.Min": oLin, "slices.Reverse": oLin, "slices.Equal": oLin,
}

// costeExpr is the cost of the calls inside a node (function literals are not entered).
func (c *complejidad) costeExpr(n ast.Node, fd *ast.FuncDecl) coste {
	if n == nil {
		return coste{}
	}
	var m coste
	ast.Inspect(n, func(x ast.Node) bool {
		switch y := x.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if sel, ok := y.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					nombre := id.Name + "." + sel.Sel.Name
					if o, ok := llamadasCaras[nombre]; ok {
						desc := "una búsqueda que recorre la lista (`" + nombre + "`)"
						if o == oNLogN {
							desc = "una ordenación (`" + nombre + "`), que cuesta n log n"
						}
						if k := (coste{o: o, piezas: []string{desc}}); k.mayor(m) {
							m = k
						}
					}
				}
			}
			if g := c.llamada(y, fd); g != nil && g != fd {
				r := c.de(g)
				o := r.orden
				if r.exponencial {
					o = orden{8, 0}
				}
				if k := (coste{o: o, piezas: []string{"una llamada a `" + nombreFunc(g) + "`, que es " + r.ordenTxt}}); o != o1 && k.mayor(m) {
					m = k
				}
			}
		}
		return true
	})
	return m
}

// esConstante reports whether e is a literal or a typed/untyped constant.
func (c *complejidad) esConstante(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return c.esConstante(x.X)
	case *ast.UnaryExpr:
		return c.esConstante(x.X)
	case *ast.BinaryExpr:
		return c.esConstante(x.X) && c.esConstante(x.Y)
	case *ast.Ident:
		if c.a.info != nil {
			if tv, ok := c.a.info.Types[x]; ok && tv.Value != nil {
				return true
			}
			if _, ok := c.a.objeto(x).(*types.Const); ok {
				return true
			}
		}
		return x.Obj != nil && x.Obj.Kind == ast.Con
	}
	return false
}

func (c *complejidad) costeBucleRange(r *ast.RangeStmt) (orden, string) {
	switch x := r.X.(type) {
	case *ast.CompositeLit:
		return o1, "un bucle de tamaño fijo"
	default:
		if c.esConstante(x) {
			return o1, "un bucle de tamaño fijo"
		}
		if t := c.a.tipo(r.X); t != nil {
			if a, ok := t.Underlying().(*types.Array); ok && a.Len() <= 64 {
				return o1, "un bucle de tamaño fijo"
			}
		}
	}
	return oLin, "un bucle que recorre `" + codigo(r.X) + "`"
}

// identsDe returns the names of the identifiers inside e.
func identsDe(e ast.Node) map[string]bool {
	out := map[string]bool{}
	if e == nil {
		return out
	}
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out[id.Name] = true
		}
		return true
	})
	return out
}

// mitadEn reports whether e contains "/ 2", ">> 1" or similar halving.
func mitadEn(e ast.Node) bool {
	si := false
	ast.Inspect(e, func(n ast.Node) bool {
		if b, ok := n.(*ast.BinaryExpr); ok {
			if (b.Op == token.QUO || b.Op == token.SHR) && esLiteralEntero(b.Y) {
				si = true
			}
		}
		if a, ok := n.(*ast.AssignStmt); ok && (a.Tok == token.QUO_ASSIGN || a.Tok == token.SHR_ASSIGN) {
			si = true
		}
		return !si
	})
	return si
}

func esLiteralEntero(e ast.Expr) bool {
	b, ok := e.(*ast.BasicLit)
	return ok && b.Kind == token.INT
}

func (c *complejidad) costeBucleFor(f *ast.ForStmt) (orden, string) {
	if f.Cond == nil {
		return oLin, "un bucle que se repite hasta que algo lo corta"
	}
	enCond := identsDe(f.Cond)
	// 1. a variable of the condition is divided (or the range is halved) in each round
	divide, multiplica := "", ""
	revisar := func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch y := x.(type) {
			case *ast.AssignStmt:
				for i, l := range y.Lhs {
					id, ok := l.(*ast.Ident)
					if !ok || !enCond[id.Name] {
						continue
					}
					switch y.Tok {
					case token.QUO_ASSIGN, token.SHR_ASSIGN:
						divide = id.Name
					case token.MUL_ASSIGN, token.SHL_ASSIGN:
						multiplica = id.Name
					case token.ASSIGN, token.DEFINE:
						if i < len(y.Rhs) {
							rhs := y.Rhs[i]
							if b, ok := rhs.(*ast.BinaryExpr); ok && identsDe(b)[id.Name] {
								switch b.Op {
								case token.QUO, token.SHR:
									divide = id.Name
								case token.MUL, token.SHL:
									multiplica = id.Name
								}
							}
							if divide == "" && mitadEn(rhs) {
								divide = id.Name
							}
							// lo = mid + 1 where mid := (lo+hi)/2
							for nombre := range identsDe(rhs) {
								if esMitad(nombre) {
									divide = "rango"
								}
							}
						}
					}
				}
			case *ast.IncDecStmt:
			}
			return true
		})
	}
	revisar(f.Body)
	if f.Post != nil {
		revisar(f.Post)
	}
	switch {
	case divide == "rango":
		return oLog, "un bucle que parte el rango por la mitad en cada vuelta"
	case divide != "":
		return oLog, "un bucle que divide `" + divide + "` en cada vuelta"
	case multiplica != "":
		return oLog, "un bucle que multiplica `" + multiplica + "` en cada vuelta"
	}
	b, ok := quitarParen(f.Cond).(*ast.BinaryExpr)
	if !ok {
		return oLin, "un bucle que depende de la entrada"
	}
	if b.Op == token.LAND || b.Op == token.LOR {
		if izq, ok := quitarParen(b.X).(*ast.BinaryExpr); ok {
			b = izq
		} else {
			return oLin, "un bucle que depende de la entrada"
		}
	}
	switch b.Op {
	case token.LSS, token.LEQ, token.GTR, token.GEQ, token.NEQ:
	default:
		return oLin, "un bucle que depende de la entrada"
	}
	for _, lado := range []ast.Expr{b.X, b.Y} {
		if m, ok := quitarParen(lado).(*ast.BinaryExpr); ok && m.Op == token.MUL && codigo(m.X) == codigo(m.Y) {
			otro := b.Y
			if lado == b.Y {
				otro = b.X
			}
			return oRaiz, "un bucle que llega hasta la raíz cuadrada de `" + codigo(otro) + "`"
		}
	}
	if c.esConstante(b.X) || c.esConstante(b.Y) {
		otro := b.X
		if c.esConstante(b.X) {
			otro = b.Y
		}
		if id, ok := quitarParen(otro).(*ast.Ident); ok && c.contadorDelFor(f, id.Name) {
			return o1, "un bucle de tamaño fijo"
		}
		return oLin, "un bucle que depende de `" + codigo(otro) + "`"
	}
	cota := b.Y
	if b.Op == token.GTR || b.Op == token.GEQ {
		cota = b.X
	}
	if call, ok := quitarParen(cota).(*ast.CallExpr); ok {
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "len" && len(call.Args) == 1 {
			return oLin, "un bucle que recorre `" + codigo(call.Args[0]) + "`"
		}
	}
	if bb, ok := quitarParen(cota).(*ast.BinaryExpr); ok {
		if call, ok := quitarParen(bb.X).(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "len" && len(call.Args) == 1 {
				return oLin, "un bucle que recorre `" + codigo(call.Args[0]) + "`"
			}
		}
	}
	return oLin, "un bucle que va hasta `" + codigo(cota) + "`"
}

// contadorDelFor reports whether name is the loop counter (initialized in Init and stepped in Post).
func (c *complejidad) contadorDelFor(f *ast.ForStmt, name string) bool {
	if f.Post == nil {
		return false
	}
	switch p := f.Post.(type) {
	case *ast.IncDecStmt:
		id, ok := p.X.(*ast.Ident)
		return ok && id.Name == name
	case *ast.AssignStmt:
		if len(p.Lhs) == 1 {
			id, ok := p.Lhs[0].(*ast.Ident)
			return ok && id.Name == name && (p.Tok == token.ADD_ASSIGN || p.Tok == token.SUB_ASSIGN)
		}
	}
	return false
}

func esMitad(n string) bool {
	switch strings.ToLower(n) {
	case "mid", "medio", "mitad", "centro", "m":
		return true
	}
	return false
}

func quitarParen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// esPropia reports whether call is a direct call of fd itself.
func (c *complejidad) esPropia(call *ast.CallExpr, fd *ast.FuncDecl) bool {
	return c.llamada(call, fd) == fd
}

// cuentaPropias counts the self-calls inside n (function literals included).
func (c *complejidad) cuentaPropias(n ast.Node, fd *ast.FuncDecl) int {
	if n == nil {
		return 0
	}
	k := 0
	ast.Inspect(n, func(x ast.Node) bool {
		if call, ok := x.(*ast.CallExpr); ok && c.esPropia(call, fd) {
			k++
		}
		return true
	})
	return k
}

// caminoMax is the largest number of self-calls on one execution path (a loop counts twice).
func (c *complejidad) caminoMax(n ast.Node, fd *ast.FuncDecl) int {
	switch x := n.(type) {
	case nil:
		return 0
	case *ast.BlockStmt:
		if x == nil {
			return 0
		}
		t := 0
		for _, s := range x.List {
			t += c.caminoMax(s, fd)
		}
		return t
	case *ast.LabeledStmt:
		return c.caminoMax(x.Stmt, fd)
	case *ast.IfStmt:
		base := c.caminoMaxStmt(x.Init, fd) + c.cuentaPropias(x.Cond, fd)
		b := c.caminoMax(x.Body, fd)
		e := 0
		if x.Else != nil {
			e = c.caminoMax(x.Else, fd)
		}
		if e > b {
			b = e
		}
		return base + b
	case *ast.SwitchStmt:
		base := c.caminoMaxStmt(x.Init, fd) + c.cuentaPropias(x.Tag, fd)
		m := 0
		for _, cc := range x.Body.List {
			t := 0
			for _, s := range cc.(*ast.CaseClause).Body {
				t += c.caminoMax(s, fd)
			}
			if t > m {
				m = t
			}
		}
		return base + m
	case *ast.TypeSwitchStmt:
		m := 0
		for _, cc := range x.Body.List {
			t := 0
			for _, s := range cc.(*ast.CaseClause).Body {
				t += c.caminoMax(s, fd)
			}
			if t > m {
				m = t
			}
		}
		return m
	case *ast.SelectStmt:
		m := 0
		for _, cc := range x.Body.List {
			t := 0
			for _, s := range cc.(*ast.CommClause).Body {
				t += c.caminoMax(s, fd)
			}
			if t > m {
				m = t
			}
		}
		return m
	case *ast.ForStmt:
		return c.cuentaPropias(x.Cond, fd) + 2*c.caminoMax(x.Body, fd)
	case *ast.RangeStmt:
		return c.cuentaPropias(x.X, fd) + 2*c.caminoMax(x.Body, fd)
	case ast.Stmt:
		return c.cuentaPropias(x, fd)
	}
	return c.cuentaPropias(n, fd)
}

func (c *complejidad) caminoMaxStmt(s ast.Stmt, fd *ast.FuncDecl) int {
	if s == nil {
		return 0
	}
	return c.caminoMax(s, fd)
}

// memoriza reports whether fd looks memoized: it reads a map (or a memo/cache/dp table).
func (c *complejidad) memoriza(fd *ast.FuncDecl) bool {
	si := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		ix, ok := n.(*ast.IndexExpr)
		if !ok {
			return !si
		}
		if t := c.a.tipo(ix.X); t != nil {
			if _, ok := t.Underlying().(*types.Map); ok {
				si = true
			}
		}
		if id, ok := ix.X.(*ast.Ident); ok {
			l := strings.ToLower(id.Name)
			if strings.Contains(l, "memo") || strings.Contains(l, "cache") || l == "dp" || strings.Contains(l, "tabla") {
				si = true
			}
		}
		return !si
	})
	return si
}

// partePorMitades reports whether every self-call receives a halved argument (slicing, /2, mid).
func (c *complejidad) partePorMitades(fd *ast.FuncDecl) bool {
	total, mitades := 0, 0
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !c.esPropia(call, fd) {
			return true
		}
		total++
		for _, arg := range call.Args {
			mitad := false
			ast.Inspect(arg, func(x ast.Node) bool {
				switch y := x.(type) {
				case *ast.SliceExpr:
					mitad = true
				case *ast.BinaryExpr:
					if (y.Op == token.QUO || y.Op == token.SHR) && esLiteralEntero(y.Y) {
						mitad = true
					}
				case *ast.Ident:
					if esMitad(y.Name) {
						mitad = true
					}
				}
				return !mitad
			})
			if mitad {
				mitades++
				break
			}
		}
		return true
	})
	return total > 0 && mitades == total
}
