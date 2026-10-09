package sintesis

import (
	"fmt"
	"go/ast"
	"go/format"
	goparser "go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Readable Go codegen (§4.7 step 12). A chain reducer ∘ (mapea|filtra)* ∘ xs becomes one fused loop with the
// lambdas inlined by substitution; variable names come from concepts (total, cuenta, mayor, resultado…).
// Partial primitives get a guard that returns the zero value with a comment. The generated code computes
// exactly what the interpreter computes wherever the interpreter has a result (⊥ cases are left out of the
// differential tests); it never evaluates a conditional branch the interpreter would skip.

// gx is one generated Go expression and its precedence: 7 operand, 6 unary, 5 * / %, 4 + -, 3 comparison,
// 2 &&, 1 ||. usos counts how often a loop variable was used.
type gx struct {
	s    string
	prec int
	usos *int
}

func (x gx) en(p int) string {
	if x.prec < p {
		return "(" + x.s + ")"
	}
	return x.s
}

var reIdent = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

func esIdent(s string) bool { return reIdent.MatchString(s) && !token.IsKeyword(s) }

// esSimple: an identifier or a literal, cheap to repeat.
func esSimple(x gx) bool {
	if esIdent(x.s) {
		return true
	}
	if _, err := strconv.ParseFloat(x.s, 64); err == nil {
		return true
	}
	return strings.HasPrefix(x.s, `"`) && strings.Count(x.s, `"`) == 2 && !strings.Contains(x.s, `\`) ||
		strings.HasPrefix(x.s, "'") && len(x.s) <= 6
}

type bloque struct{ lineas []string }

func (b *bloque) linea(s string) { b.lineas = append(b.lineas, s) }

func (b *bloque) agregar(o *bloque) { b.lineas = append(b.lineas, o.lineas...) }

// paquetes the generated code may use, by local name.
var paquetesGo = map[string]string{
	"strings": "strings", "strconv": "strconv", "slices": "slices", "unicode": "unicode", "utf8": "unicode/utf8",
	"math": "math", "sort": "sort", "maps": "maps", "big": "math/big",
}

var reservadosGo = []string{
	"append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new",
	"panic", "print", "println", "real", "recover", "bool", "byte", "error", "float32", "float64", "int",
	"int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64",
	"uintptr", "any", "comparable", "true", "false", "nil", "iota",
}

type generador struct {
	ayudas     []string // helper funcs, in emission order
	vistas     map[string]bool
	aprendidas map[*Primitiva]string // learned component → helper name
	globales   map[string]bool
	err        error
}

type funcGen struct {
	g      *generador
	firma  nucleo.Firma
	nPar   int
	usados map[string]bool
	cero   string
}

// AGo writes e as a complete Go file: package solucion, imports, the function, helpers, gofmt'd.
func AGo(e *Expr, f nucleo.Firma) (string, error) {
	if e == nil {
		return "", fmt.Errorf("sintesis: no hay programa")
	}
	if tieneHuecos(e) {
		return "", fmt.Errorf("sintesis: el programa tiene huecos sin rellenar")
	}
	if len(f.Res) != 1 {
		return "", fmt.Errorf("sintesis: la función debe dar exactamente un resultado")
	}
	if !mismoTipo(e.Tipo, f.Res[0]) {
		return "", fmt.Errorf("sintesis: el programa da %s y la función debe dar %s", e.Tipo.Go(), f.Res[0].Go())
	}
	if f.Nombre == "" || !esIdent(f.Nombre) {
		f.Nombre = "Solucion"
	}
	g := &generador{vistas: map[string]bool{}, aprendidas: map[*Primitiva]string{}, globales: map[string]bool{}}
	for p := range paquetesGo {
		g.globales[p] = true
	}
	for _, r := range reservadosGo {
		g.globales[r] = true
	}
	g.globales[f.Nombre] = true
	g.reservar(e, map[*Primitiva]bool{})
	principal := g.funcion(f.Nombre, f, e)
	if g.err != nil {
		return "", g.err
	}
	var sb strings.Builder
	sb.WriteString("package solucion\n\n")
	sb.WriteString(principal)
	for _, a := range g.ayudas {
		sb.WriteString("\n")
		sb.WriteString(a)
		sb.WriteString("\n")
	}
	return terminarFuente(sb.String())
}

// terminarFuente adds the imports the code uses and formats it.
func terminarFuente(src string) (string, error) {
	fset := token.NewFileSet()
	af, err := goparser.ParseFile(fset, "solucion.go", src, 0)
	if err != nil {
		return "", fmt.Errorf("sintesis: el código generado no se puede leer: %v\n%s", err, src)
	}
	usados := map[string]bool{}
	ast.Inspect(af, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				if ruta, ok := paquetesGo[id.Name]; ok {
					usados[ruta] = true
				}
			}
		}
		return true
	})
	if len(usados) > 0 {
		rutas := make([]string, 0, len(usados))
		for r := range usados {
			rutas = append(rutas, strconv.Quote(r))
		}
		sort.Strings(rutas)
		bloqueImp := "import " + rutas[0] + "\n"
		if len(rutas) > 1 {
			bloqueImp = "import (\n\t" + strings.Join(rutas, "\n\t") + "\n)\n"
		}
		src = strings.Replace(src, "package solucion\n", "package solucion\n\n"+bloqueImp, 1)
	}
	out, err := format.Source([]byte(src))
	if err != nil {
		return "", fmt.Errorf("sintesis: el código generado no tiene formato Go: %v\n%s", err, src)
	}
	return string(out), nil
}

var reNombreFunc = regexp.MustCompile(`^func (\w+)`)

// reservar records the helper names e needs, so locals never shadow them.
func (g *generador) reservar(e *Expr, vistos map[*Primitiva]bool) {
	if e == nil {
		return
	}
	if p := e.Op; p != nil && !vistos[p] {
		vistos[p] = true
		if m := reNombreFunc.FindStringSubmatch(p.Ayudante); m != nil {
			g.globales[m[1]] = true
		}
		if p.Cuerpo != nil {
			g.nombreAprendida(p)
			g.reservar(p.Cuerpo, vistos)
		}
	}
	for _, h := range e.Hijos {
		g.reservar(h, vistos)
	}
	for _, l := range e.Lambdas {
		if l != nil {
			g.reservar(l.Cuerpo, vistos)
		}
	}
}

// nombreAprendida returns the helper name of a learned component (lower-case first letter).
func (g *generador) nombreAprendida(p *Primitiva) string {
	if n, ok := g.aprendidas[p]; ok {
		return n
	}
	base := p.Nombre
	if !esIdent(base) {
		base = "componente"
	}
	rs := []rune(base)
	rs[0] = unicode.ToLower(rs[0])
	base = string(rs)
	n := base
	for i := 2; g.globales[n]; i++ {
		n = base + strconv.Itoa(i)
	}
	g.globales[n] = true
	g.aprendidas[p] = n
	return n
}

func (g *generador) ayuda(codigo string) {
	if codigo == "" || g.vistas[codigo] {
		return
	}
	g.vistas[codigo] = true
	g.ayudas = append(g.ayudas, codigo)
}

func (g *generador) fallo(formato string, a ...any) {
	if g.err == nil {
		g.err = fmt.Errorf("sintesis: "+formato, a...)
	}
}

// funcion generates one function declaration for e with signature f (named nombre).
func (g *generador) funcion(nombre string, f nucleo.Firma, e *Expr) string {
	fg := &funcGen{g: g, firma: f, nPar: len(f.Params), usados: map[string]bool{}, cero: f.Res[0].Cero()}
	for k := range g.globales {
		fg.usados[k] = true
	}
	env := map[int]gx{}
	decl := f
	decl.Nombre = nombre
	decl.Receptor = nil
	decl.Params = make([]nucleo.Param, len(f.Params))
	for i, p := range f.Params {
		n := p.Nombre
		if n == "" || !esIdent(n) || fg.usados[n] {
			base := "v"
			if n != "" && esIdent(n) {
				rs := []rune(n)
				rs[0] = unicode.ToUpper(rs[0])
				base = "v" + string(rs)
			}
			n = fg.nuevo(base)
		}
		fg.usados[n] = true
		decl.Params[i] = nucleo.Param{Nombre: n, Tipo: p.Tipo}
		env[i] = gx{s: n, prec: 7}
	}
	b := &bloque{}
	fg.retornar(e, b, env, 0)
	var sb strings.Builder
	sb.WriteString(decl.Go())
	sb.WriteString(" {\n")
	for _, l := range b.lineas {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// nuevo returns a fresh local name based on base.
func (fg *funcGen) nuevo(base string) string {
	n := base
	for i := 2; fg.usados[n]; i++ {
		n = base + strconv.Itoa(i)
	}
	fg.usados[n] = true
	return n
}

func (fg *funcGen) guarda(b *bloque, cond, motivo string) {
	b.linea("if " + cond + " {")
	b.linea("return " + fg.cero + " // " + motivo + " devuelvo " + fg.cero)
	b.linea("}")
}

// retornar emits statements that return the value of e.
func (fg *funcGen) retornar(e *Expr, b *bloque, env map[int]gx, prof int) {
	if e.Op != nil && e.Op.Nombre == "si" && len(e.Hijos) == 3 {
		c := fg.expr(e.Hijos[0], b, env, prof)
		si := &bloque{}
		fg.retornar(e.Hijos[1], si, env, prof)
		b.linea("if " + c.s + " {")
		b.agregar(si)
		b.linea("}")
		fg.retornar(e.Hijos[2], b, env, prof)
		return
	}
	if e.Op != nil && (e.Op.Nombre == "todos" || e.Op.Nombre == "alguno") && len(e.Lambdas) == 1 && len(e.Hijos) == 1 {
		fg.cuantificarRaiz(e, b, env, prof)
		return
	}
	x := fg.expr(e, b, env, prof)
	b.linea("return " + x.s)
}

func (fg *funcGen) lit(v V, t nucleo.Tipo) gx {
	switch t.Clase {
	case nucleo.CLista:
		if xs, _ := v.([]V); len(xs) == 0 {
			return gx{s: t.Go() + "{}", prec: 7}
		}
	case nucleo.CMapa:
		if m, _ := v.(nucleo.Mapa); len(m) == 0 {
			return gx{s: t.Go() + "{}", prec: 7}
		}
	}
	s := nucleo.FormatoGo(v, t)
	if strings.HasPrefix(s, "-") {
		return gx{s: s, prec: 6}
	}
	return gx{s: s, prec: 7}
}

var reTpl = regexp.MustCompile(`\{(\d+)(?::(\d))?\}`)

func (fg *funcGen) plantilla(p *Primitiva, args []gx) gx {
	s := reTpl.ReplaceAllStringFunc(p.Go, func(m string) string {
		sub := reTpl.FindStringSubmatch(m)
		i, _ := strconv.Atoi(sub[1])
		if i >= len(args) {
			fg.g.fallo("la plantilla de %s usa el argumento %d", p.Nombre, i)
			return "nil"
		}
		if sub[2] != "" {
			pr, _ := strconv.Atoi(sub[2])
			return args[i].en(pr)
		}
		return args[i].s
	})
	pr := p.prec
	if pr == 0 {
		pr = 7
	}
	return gx{s: s, prec: pr}
}

func (fg *funcGen) hijos(e *Expr, b *bloque, env map[int]gx, prof int) []gx {
	out := make([]gx, len(e.Hijos))
	for i, h := range e.Hijos {
		out[i] = fg.expr(h, b, env, prof)
	}
	return out
}

// identificador hoists x into a fresh variable unless it already is an identifier.
func (fg *funcGen) identificador(x gx, b *bloque, base string) gx {
	if esIdent(x.s) {
		return x
	}
	n := fg.nuevo(base)
	b.linea(n + " := " + x.s)
	return gx{s: n, prec: 7}
}

func esOp(e *Expr, nombre string) bool { return e != nil && e.Op != nil && e.Op.Nombre == nombre }

// expr generates e, emitting the statements it needs into b.
func (fg *funcGen) expr(e *Expr, b *bloque, env map[int]gx, prof int) gx {
	switch {
	case e == nil || e.Hueco:
		fg.g.fallo("el programa está incompleto")
		return gx{s: "nil", prec: 7}
	case e.EsConst:
		return fg.lit(e.Const, e.Tipo)
	case e.Op == nil:
		v, ok := env[e.Var]
		if !ok {
			fg.g.fallo("variable %d sin definir", e.Var)
			return gx{s: "nil", prec: 7}
		}
		if v.usos != nil {
			*v.usos++
		}
		return gx{s: v.s, prec: v.prec}
	}
	p := e.Op
	if p.Cuerpo != nil {
		nombre := fg.g.nombreAprendida(p)
		fg.g.generarAprendida(p, nombre)
		args := fg.hijos(e, b, env, prof)
		partes := make([]string, len(args))
		for i, a := range args {
			partes[i] = a.s
		}
		return gx{s: nombre + "(" + strings.Join(partes, ", ") + ")", prec: 7}
	}
	switch p.Nombre {
	case "si":
		return fg.si(e, b, env, prof)
	case "y", "o":
		return fg.logico(e, b, env, prof)
	case "suma", "producto", "contar", "todos", "alguno", "maxL", "minL", "promedio", "pliega", "masLarga",
		"masCorta", "frecuencias":
		return fg.reducir(e, b, env, prof)
	case "largo":
		if fuente, etapas := desarmar(e.Hijos[0]); len(etapas) > 0 {
			return fg.reducir(e, b, env, prof)
		} else if esOp(fuente, "runas") {
			s := fg.expr(fuente.Hijos[0], b, env, prof)
			return gx{s: "utf8.RuneCountInString(" + s.s + ")", prec: 7}
		}
	case "filtra", "mapea":
		return fg.reducir(e, b, env, prof)
	case "ordenar", "ordenarDesc", "invertir":
		xs := fg.expr(e.Hijos[0], b, env, prof)
		base := "ordenados"
		if p.Nombre == "invertir" {
			base = "invertida"
		}
		n := fg.nuevo(base)
		if esIdent(xs.s) {
			b.linea(n + " := slices.Clone(" + xs.s + ")")
		} else {
			b.linea(n + " := " + xs.s)
		}
		if p.Nombre != "invertir" {
			b.linea("slices.Sort(" + n + ")")
		}
		if p.Nombre != "ordenar" {
			b.linea("slices.Reverse(" + n + ")")
		}
		return gx{s: n, prec: 7}
	case "primero", "ultimo", "elemento":
		xs := fg.identificador(fg.expr(e.Hijos[0], b, env, prof), b, nombreLista(e.Hijos[0]))
		switch p.Nombre {
		case "primero":
			fg.guarda(b, "len("+xs.s+") == 0", "con la lista vacía")
			return gx{s: xs.s + "[0]", prec: 7}
		case "ultimo":
			fg.guarda(b, "len("+xs.s+") == 0", "con la lista vacía")
			return gx{s: xs.s + "[len(" + xs.s + ")-1]", prec: 7}
		}
		i := fg.expr(e.Hijos[1], b, env, prof)
		if e.Hijos[1].EsConst && ent(e.Hijos[1].Const) >= 0 {
			fg.guarda(b, "len("+xs.s+") <= "+i.s, "fuera de la lista")
		} else {
			i = fg.identificador(i, b, "pos")
			fg.guarda(b, i.s+" < 0 || "+i.s+" >= len("+xs.s+")", "fuera de la lista")
		}
		return gx{s: xs.s + "[" + i.s + "]", prec: 7}
	case "atoi":
		s := fg.expr(e.Hijos[0], b, env, prof)
		n := fg.nuevo("numero")
		er := fg.nuevo("err")
		b.linea(n + ", " + er + " := strconv.Atoi(" + s.s + ")")
		fg.guarda(b, er+" != nil", "si no es un número")
		return gx{s: n, prec: 7}
	case "/", "%", "divisible":
		args := fg.hijos(e, b, env, prof)
		if d := e.Hijos[1]; !(d.EsConst && ent(d.Const) != 0) {
			args[1] = fg.identificador(args[1], b, "divisor")
			fg.guarda(b, args[1].s+" == 0", "sin divisor")
		}
		return fg.plantilla(p, args)
	case "repetir":
		args := fg.hijos(e, b, env, prof)
		if n := e.Hijos[1]; !(n.EsConst && ent(n.Const) >= 0) {
			args[1] = fg.identificador(args[1], b, "veces")
			fg.guarda(b, args[1].s+" < 0", "con un número negativo de veces")
		}
		return fg.plantilla(p, args)
	}
	if p.Ayudante != "" {
		fg.g.ayuda(p.Ayudante)
	}
	if p.Go == "" {
		fg.g.fallo("no sé escribir «%s» en Go", p.Nombre)
		return gx{s: "nil", prec: 7}
	}
	return fg.plantilla(p, fg.hijos(e, b, env, prof))
}

func nombreLista(xs *Expr) string {
	if xs != nil && xs.Tipo.Elem != nil {
		switch xs.Tipo.Elem.Clase {
		case nucleo.CString:
			return "palabras"
		case nucleo.CRune:
			return "letras"
		}
	}
	return "lista"
}

func (fg *funcGen) si(e *Expr, b *bloque, env map[int]gx, prof int) gx {
	c := fg.expr(e.Hijos[0], b, env, prof)
	b1, b2 := &bloque{}, &bloque{}
	a := fg.expr(e.Hijos[1], b1, env, prof)
	d := fg.expr(e.Hijos[2], b2, env, prof)
	v := fg.nuevo("valor")
	if len(b2.lineas) == 0 && (esIdent(d.s) || e.Tipo.Clase == nucleo.CString || e.Tipo.Clase == nucleo.CBool) && esSimple(d) {
		b.linea(v + " := " + d.s)
		b.linea("if " + c.s + " {")
		b.agregar(b1)
		b.linea(v + " = " + a.s)
		b.linea("}")
		return gx{s: v, prec: 7}
	}
	b.linea("var " + v + " " + e.Tipo.Go())
	b.linea("if " + c.s + " {")
	b.agregar(b1)
	b.linea(v + " = " + a.s)
	b.linea("} else {")
	b.agregar(b2)
	b.linea(v + " = " + d.s)
	b.linea("}")
	return gx{s: v, prec: 7}
}

func (fg *funcGen) logico(e *Expr, b *bloque, env map[int]gx, prof int) gx {
	y := e.Op.Nombre == "y"
	a := fg.expr(e.Hijos[0], b, env, prof)
	b2 := &bloque{}
	c := fg.expr(e.Hijos[1], b2, env, prof)
	if len(b2.lineas) == 0 {
		if y {
			return gx{s: a.en(2) + " && " + c.en(3), prec: 2}
		}
		return gx{s: a.en(1) + " || " + c.en(2), prec: 1}
	}
	v := fg.nuevo("ok")
	b.linea(v + " := " + a.s)
	if y {
		b.linea("if " + v + " {")
	} else {
		b.linea("if !" + v + " {")
	}
	b.agregar(b2)
	b.linea(v + " = " + c.s)
	b.linea("}")
	return gx{s: v, prec: 7}
}

// ---- lambdas ----

func (fg *funcGen) conLambda(l *Lambda, args []gx, env map[int]gx, prof int) (map[int]gx, int) {
	nenv := make(map[int]gx, len(env)+len(args))
	for k, v := range env {
		nenv[k] = v
	}
	for k := range l.Params {
		if k < len(args) {
			nenv[fg.nPar+prof+k] = args[k]
		}
	}
	return nenv, prof + len(l.Params)
}

func (fg *funcGen) aplicar(l *Lambda, args []gx, b *bloque, env map[int]gx, prof int) gx {
	if l == nil || l.Cuerpo == nil {
		fg.g.fallo("falta una función")
		return gx{s: "nil", prec: 7}
	}
	nenv, nprof := fg.conLambda(l, args, env, prof)
	return fg.expr(l.Cuerpo, b, nenv, nprof)
}

var opuestos = map[string]string{"<": ">=", ">": "<=", "<=": ">", ">=": "<", "==": "!=", "!=": "=="}

var negadas = map[string]string{
	"esPar": "{0:5}%2 != 0", "esImpar": "{0:5}%2 == 0", "esPositivo": "{0:4} <= 0", "esNegativo": "{0:4} >= 0",
	"esCero": "{0:4} != 0",
}

// negado generates the negation of a predicate lambda applied to args, as readably as it can.
func (fg *funcGen) negado(l *Lambda, args []gx, b *bloque, env map[int]gx, prof int) gx {
	nenv, nprof := fg.conLambda(l, args, env, prof)
	cuerpo := l.Cuerpo
	if cuerpo.Op != nil && cuerpo.Op.Cuerpo == nil {
		p := cuerpo.Op
		switch {
		case p.Nombre == "no" && len(cuerpo.Hijos) == 1:
			return fg.expr(cuerpo.Hijos[0], b, nenv, nprof)
		case opuestos[p.Nombre] != "" && strings.HasPrefix(p.Go, "{0:4} "+p.Nombre+" "):
			hs := fg.hijos(cuerpo, b, nenv, nprof)
			return gx{s: hs[0].en(4) + " " + opuestos[p.Nombre] + " " + hs[1].en(4), prec: 3}
		case negadas[p.Nombre] != "" && len(cuerpo.Hijos) == 1:
			hs := fg.hijos(cuerpo, b, nenv, nprof)
			q := *p
			q.Go = negadas[p.Nombre]
			return fg.plantilla(&q, hs)
		}
	}
	x := fg.expr(cuerpo, b, nenv, nprof)
	return gx{s: "!" + x.en(7), prec: 6}
}

// ---- fused loops ----

type etapa struct {
	filtro bool
	lam    *Lambda
}

func esCadena(e *Expr) bool {
	return e != nil && e.Op != nil && e.Op.Cuerpo == nil && (e.Op.Nombre == "filtra" || e.Op.Nombre == "mapea") &&
		len(e.Lambdas) == 1 && len(e.Hijos) == 1
}

// desarmar splits xs into its source and the (mapea|filtra) stages applied to it, innermost first.
func desarmar(xs *Expr) (*Expr, []etapa) {
	var etapas []etapa
	for esCadena(xs) {
		etapas = append([]etapa{{filtro: xs.Op.Nombre == "filtra", lam: xs.Lambdas[0]}}, etapas...)
		xs = xs.Hijos[0]
	}
	return xs, etapas
}

// nombreElemento picks the loop variable name for an element type.
func nombreElemento(t nucleo.Tipo, fuente *Expr) string {
	switch t.Clase {
	case nucleo.CRune:
		return "r"
	case nucleo.CString:
		return "palabra"
	case nucleo.CBool:
		return "b"
	case nucleo.CLista:
		return "fila"
	}
	if esOp(fuente, "rango") {
		return "i"
	}
	return "x"
}

func nombreValor(t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CRune:
		return "c"
	case nucleo.CString:
		return "texto"
	case nucleo.CBool:
		return "ok"
	case nucleo.CLista:
		return "lista"
	}
	return "y"
}

// recorrido is one fused loop under construction.
type recorrido struct {
	fg     *funcGen
	b      *bloque
	env    map[int]gx
	prof   int
	inits  []string
	pila   []*bloque
	heads  []string
	cond   string
	condPr int
}

func (r *recorrido) actual() *bloque { return r.pila[len(r.pila)-1] }

func (r *recorrido) cerrarCond() {
	if r.cond == "" {
		return
	}
	r.heads = append(r.heads, "if "+r.cond+" {")
	r.pila = append(r.pila, &bloque{})
	r.cond = ""
}

func (r *recorrido) filtro(p gx, previas *bloque) {
	if len(previas.lineas) > 0 {
		r.cerrarCond()
		r.actual().agregar(previas)
	}
	if r.cond == "" {
		r.cond, r.condPr = p.s, p.prec
		return
	}
	c := r.cond
	if r.condPr < 2 {
		c = "(" + c + ")"
	}
	r.cond, r.condPr = c+" && "+p.en(3), 2
}

// usosDesde counts the uses of the element produced before stage k by the stages k.. and the final action.
func (fg *funcGen) usosDesde(etapas []etapa, k, prof, final int) int {
	slot := fg.nPar + prof
	n := 0
	for _, et := range etapas[k:] {
		n += usaVar(et.lam.Cuerpo, slot)
		if !et.filtro {
			return n
		}
	}
	return n + final
}

// recorrer emits the loop over fuente through etapas; accion runs at the innermost point with the element.
func (fg *funcGen) recorrer(b *bloque, fuente *Expr, etapas []etapa, env map[int]gx, prof int, inits []string,
	final int, accion func(cur *bloque, elem gx)) {
	elemT := tI
	if fuente.Tipo.Elem != nil {
		elemT = *fuente.Tipo.Elem
	}
	usos := new(int)
	var cabecera func() string
	switch {
	case esOp(fuente, "runas"):
		s := fg.expr(fuente.Hijos[0], b, env, prof)
		n := fg.nuevo("r")
		cabecera = func() string {
			if *usos == 0 {
				return "for range " + s.s
			}
			return "for _, " + n + " := range " + s.s
		}
		fg.cuerpoBucle(b, etapas, env, prof, inits, final, accion, gx{s: n, prec: 7, usos: usos}, cabecera)
		return
	case esOp(fuente, "rango"):
		a := fg.expr(fuente.Hijos[0], b, env, prof)
		op, hasta := " < ", fuente.Hijos[1]
		if esOp(hasta, "+") && len(hasta.Hijos) == 2 && hasta.Hijos[1].EsConst && ent(hasta.Hijos[1].Const) == 1 {
			op, hasta = " <= ", hasta.Hijos[0]
		}
		z := fg.expr(hasta, b, env, prof)
		if !esSimple(z) && strings.Contains(z.s, "(") {
			z = fg.identificador(z, b, "hasta")
		}
		n := fg.nuevo("i")
		cabecera = func() string { return "for " + n + " := " + a.s + "; " + n + op + z.en(4) + "; " + n + "++" }
		fg.cuerpoBucle(b, etapas, env, prof, inits, final, accion, gx{s: n, prec: 7, usos: usos}, cabecera)
		return
	case esOp(fuente, "palabras"):
		s := fg.expr(fuente.Hijos[0], b, env, prof)
		n := fg.nuevo("palabra")
		cabecera = func() string {
			if *usos == 0 {
				return "for range strings.Fields(" + s.s + ")"
			}
			return "for _, " + n + " := range strings.Fields(" + s.s + ")"
		}
		fg.cuerpoBucle(b, etapas, env, prof, inits, final, accion, gx{s: n, prec: 7, usos: usos}, cabecera)
		return
	}
	xs := fg.expr(fuente, b, env, prof)
	n := fg.nuevo(nombreElemento(elemT, fuente))
	cabecera = func() string {
		if *usos == 0 {
			return "for range " + xs.s
		}
		return "for _, " + n + " := range " + xs.s
	}
	fg.cuerpoBucle(b, etapas, env, prof, inits, final, accion, gx{s: n, prec: 7, usos: usos}, cabecera)
}

func (fg *funcGen) cuerpoBucle(b *bloque, etapas []etapa, env map[int]gx, prof int, inits []string, final int,
	accion func(cur *bloque, elem gx), elem gx, cabecera func() string) {
	cuerpo := &bloque{}
	r := &recorrido{fg: fg, b: b, env: env, prof: prof, pila: []*bloque{cuerpo}}
	for k, et := range etapas {
		if et.filtro {
			previas := &bloque{}
			p := fg.aplicar(et.lam, []gx{elem}, previas, env, prof)
			r.filtro(p, previas)
			continue
		}
		usos := fg.usosDesde(etapas, k+1, prof, final)
		if usos == 0 {
			elem = gx{s: "_", prec: 7}
			continue
		}
		previas := &bloque{}
		m := fg.aplicar(et.lam, []gx{elem}, previas, env, prof)
		ligar := usos > 1 && !esSimple(m)
		if len(previas.lineas) > 0 || ligar {
			r.cerrarCond()
			r.actual().agregar(previas)
			if ligar {
				n := fg.nuevo(nombreValor(et.lam.Cuerpo.Tipo))
				r.actual().linea(n + " := " + m.s)
				m = gx{s: n, prec: 7}
			}
		}
		elem = gx{s: m.s, prec: m.prec}
	}
	r.cerrarCond()
	if final > 0 && elem.usos != nil {
		*elem.usos += final
	}
	accion(r.actual(), elem)
	for i := len(r.pila) - 1; i > 0; i-- {
		padre := r.pila[i-1]
		padre.linea(r.heads[i-1])
		padre.agregar(r.pila[i])
		padre.linea("}")
	}
	for _, in := range inits {
		b.linea(in)
	}
	b.linea(cabecera() + " {")
	b.agregar(cuerpo)
	b.linea("}")
}
