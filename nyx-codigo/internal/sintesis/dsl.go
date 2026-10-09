// Package sintesis is Nyx Código's program synthesizer (§4.7): a typed DSL of about 100 primitives with an
// interpreter, sketches built from the Spanish frame, cost-guided bottom-up enumeration with observational
// equivalence, lambda pools, EUSolver-style conditionals, Distinguir, readable Go codegen with loop fusion,
// and library learning.
//
// It imports only the standard library and internal/nucleo.
package sintesis

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// ErrIndefinido is the DSL's ⊥: a partial primitive has no result for this input (maxL of an empty list,
// atoi of a non-number, an overflow…). It is a sentinel so the search can test it cheaply.
var ErrIndefinido = errors.New("sintesis: el programa no da resultado con esta entrada")

// Primitiva is one DSL component. Polymorphic components are registered once per element type, all under
// the same Nombre; Parse picks the instance from the argument types.
type Primitiva struct {
	Nombre    string        // "suma", "filtra", "esPar"
	Args      []nucleo.Tipo // lambda args hold tipoFuncion (Clase CInvalida, Nombre "func")
	Lambdas   []LambdaTipo  // per arg: zero value if not a lambda
	Res       nucleo.Tipo
	Costo     float64  // base cost before priors (bits ×10)
	Conceptos []string // from nucleo.Conceptos
	Eval      func(a []nucleo.Valor, l []Funcion) (nucleo.Valor, error)
	Go        string // template: "strings.ToUpper({0})"; {0}.. args ({0:5} = wrap in parentheses below precedence 5)
	Ayudante  string // helper func emitted once (e.g. esPrimo); "" if none
	Imports   []string
	Frase     string // Spanish: "la suma de {0}"
	Cuerpo    *Expr  // non-nil for learned components (Eval/Go derived from it)
	Parcial   bool   // may fail (⊥)

	conmutativa bool   // a∘b == b∘a: the search keeps only id1 ≤ id2
	perezosa    bool   // si, y, o: arguments are evaluated only when needed
	involutiva  bool   // f(f(x)) == x: f∘f is skipped
	idempotente bool   // f(f(x)) == f(x): f∘f is skipped
	prec        int    // Go precedence of the template's result (6 = primary)
	adjetivo    string // plural adjective for predicates used in filters: "pares"
	sustantivo  string // plural noun for rune/string predicates: "vocales"
	femenino    bool   // gender of sustantivo
	firma       *nucleo.Firma // learned components and inventions
}

// LambdaTipo is the type of a lambda argument.
type LambdaTipo struct {
	Params []nucleo.Tipo
	Res    nucleo.Tipo
}

func (lt LambdaTipo) esCero() bool { return len(lt.Params) == 0 && lt.Res.Clase == nucleo.CInvalida }

func (lt LambdaTipo) clave() string {
	var sb strings.Builder
	sb.WriteString("(")
	for i, p := range lt.Params {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(p.ClaveTipo())
	}
	sb.WriteString(")")
	sb.WriteString(lt.Res.ClaveTipo())
	return sb.String()
}

// Funcion is an evaluated lambda.
type Funcion func(args []nucleo.Valor) (nucleo.Valor, error)

// Expr is a DSL program tree.
type Expr struct {
	Op      *Primitiva
	Hijos   []*Expr   // non-lambda arguments, in Args order
	Lambdas []*Lambda // lambda arguments, in Args order
	Var     int       // ≥ 0: parameter/lambda-variable slot (de Bruijn level: params first, then lambda params); -1 otherwise
	Const   nucleo.Valor
	EsConst bool
	Hueco   bool // sketch hole "?"
	Tipo    nucleo.Tipo

	nombre string // name of a variable, for String
}

// Lambda is a lambda argument: its parameters take the next env slots.
type Lambda struct {
	Params []nucleo.Tipo
	Cuerpo *Expr

	nombres []string
}

// tipoFuncion marks a lambda position in Primitiva.Args.
var tipoFuncion = nucleo.Tipo{Nombre: "func"}

// ---- the DSL type set ----

var (
	tI   = nucleo.TInt
	tB   = nucleo.TBool
	tS   = nucleo.TString
	tR   = nucleo.TRune
	tF   = nucleo.TFloat
	tLI  = nucleo.ListaDe(nucleo.TInt)
	tLS  = nucleo.ListaDe(nucleo.TString)
	tLR  = nucleo.ListaDe(nucleo.TRune)
	tLB  = nucleo.ListaDe(nucleo.TBool)
	tLF  = nucleo.ListaDe(nucleo.TFloat)
	tLLI = nucleo.ListaDe(nucleo.ListaDe(nucleo.TInt))
	tMSI = nucleo.MapaDe(nucleo.TString, nucleo.TInt)
)

// tiposDSL is the closed type set; the index is the type id used by the search.
var tiposDSL = []nucleo.Tipo{tI, tB, tS, tR, tF, tLI, tLS, tLR, tLB, tLF, tLLI, tMSI}

var idTipos = func() map[string]int {
	m := map[string]int{}
	for i, t := range tiposDSL {
		m[t.ClaveTipo()] = i
	}
	return m
}()

// idTipo returns the type id, or -1 when t is outside the DSL type set.
func idTipo(t nucleo.Tipo) int {
	if t.Definido {
		return -1
	}
	if id, ok := idTipos[t.ClaveTipo()]; ok {
		return id
	}
	return -1
}

func mismoTipo(a, b nucleo.Tipo) bool { return a.ClaveTipo() == b.ClaveTipo() }

// Sintetizable reports whether every parameter and the result are within the DSL type set:
// no receiver, 1..4 parameters, exactly one result; map[string]int only as the result.
func Sintetizable(f nucleo.Firma) bool {
	if f.Receptor != nil || len(f.Params) == 0 || len(f.Params) > 4 || len(f.Res) != 1 {
		return false
	}
	for _, p := range f.Params {
		id := idTipo(p.Tipo)
		if id < 0 || mismoTipo(p.Tipo, tMSI) {
			return false
		}
	}
	return idTipo(f.Res[0]) >= 0
}

// ---- registry ----

// Registro indexes primitives by result type and by name. It is safe for concurrent use.
type Registro struct {
	mu        sync.RWMutex
	todas     []*Primitiva
	porNombre map[string][]*Primitiva
	porRes    map[string][]*Primitiva
}

func nuevoRegistro() *Registro {
	return &Registro{porNombre: map[string][]*Primitiva{}, porRes: map[string][]*Primitiva{}}
}

var (
	baseUna sync.Once
	baseReg *Registro
)

// Base returns a fresh registry with the shipped primitives. Every call returns a new registry, so callers
// may add to it freely.
func Base() *Registro {
	baseUna.Do(func() {
		baseReg = nuevoRegistro()
		registrarBase(baseReg)
	})
	return baseReg.clonar()
}

func (r *Registro) clonar() *Registro {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c := nuevoRegistro()
	c.todas = append([]*Primitiva(nil), r.todas...)
	for k, v := range r.porNombre {
		c.porNombre[k] = append([]*Primitiva(nil), v...)
	}
	for k, v := range r.porRes {
		c.porRes[k] = append([]*Primitiva(nil), v...)
	}
	return c
}

func clavePrim(p *Primitiva) string {
	var sb strings.Builder
	sb.WriteString(p.Nombre)
	sb.WriteString("/")
	for i, a := range p.Args {
		if i > 0 {
			sb.WriteString(",")
		}
		if i < len(p.Lambdas) && !p.Lambdas[i].esCero() {
			sb.WriteString(p.Lambdas[i].clave())
		} else {
			sb.WriteString(a.ClaveTipo())
		}
	}
	return sb.String()
}

// Agregar registers p. A primitive with the same name and argument types is replaced.
func (r *Registro) Agregar(p *Primitiva) {
	if p == nil {
		return
	}
	if p.prec == 0 {
		p.prec = 6
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	clave := clavePrim(p)
	for i, q := range r.todas {
		if clavePrim(q) == clave {
			r.todas[i] = p
			r.reemplazar(q, p)
			return
		}
	}
	r.todas = append(r.todas, p)
	r.porNombre[p.Nombre] = append(r.porNombre[p.Nombre], p)
	k := p.Res.ClaveTipo()
	r.porRes[k] = append(r.porRes[k], p)
}

func (r *Registro) reemplazar(viejo, nuevo *Primitiva) {
	for _, m := range []map[string][]*Primitiva{r.porNombre, r.porRes} {
		for k, ps := range m {
			for i, q := range ps {
				if q == viejo {
					m[k][i] = nuevo
				}
			}
		}
	}
}

// alias registers p under another name too (filtraS → filtra on []string). It is not listed by Todas.
func (r *Registro) alias(nombre string, p *Primitiva) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.porNombre[nombre] = append(r.porNombre[nombre], p)
}

// quitar removes every instance named nombre (used by tests and by Biblioteca to forget).
func (r *Registro) quitar(nombre string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ps := r.porNombre[nombre]
	if len(ps) == 0 {
		return
	}
	fuera := map[*Primitiva]bool{}
	for _, p := range ps {
		fuera[p] = true
	}
	delete(r.porNombre, nombre)
	var todas []*Primitiva
	for _, p := range r.todas {
		if !fuera[p] {
			todas = append(todas, p)
		}
	}
	r.todas = todas
	for k, v := range r.porRes {
		var w []*Primitiva
		for _, p := range v {
			if !fuera[p] {
				w = append(w, p)
			}
		}
		r.porRes[k] = w
	}
	for k, v := range r.porNombre {
		var w []*Primitiva
		for _, p := range v {
			if !fuera[p] {
				w = append(w, p)
			}
		}
		r.porNombre[k] = w
	}
}

// Buscar returns the first instance registered under nombre (aliases included), or nil.
func (r *Registro) Buscar(nombre string) *Primitiva {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if ps := r.porNombre[nombre]; len(ps) > 0 {
		return ps[0]
	}
	return nil
}

func (r *Registro) instancias(nombre string) []*Primitiva {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*Primitiva(nil), r.porNombre[nombre]...)
}

// buscarTipos returns the instance of nombre whose non-lambda argument types are args (in order).
func (r *Registro) buscarTipos(nombre string, args ...nucleo.Tipo) *Primitiva {
	for _, p := range r.instancias(nombre) {
		var normales []nucleo.Tipo
		for i, a := range p.Args {
			if !p.esLambda(i) {
				normales = append(normales, a)
			}
		}
		if len(normales) != len(args) {
			continue
		}
		ok := true
		for i := range args {
			if !mismoTipo(normales[i], args[i]) {
				ok = false
				break
			}
		}
		if ok {
			return p
		}
	}
	return nil
}

// PorResultado returns the primitives whose result type is t.
func (r *Registro) PorResultado(t nucleo.Tipo) []*Primitiva {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*Primitiva(nil), r.porRes[t.ClaveTipo()]...)
}

// Todas returns every primitive in registration order.
func (r *Registro) Todas() []*Primitiva {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*Primitiva(nil), r.todas...)
}

func (p *Primitiva) esLambda(i int) bool {
	return i < len(p.Lambdas) && !p.Lambdas[i].esCero()
}

func (p *Primitiva) numLambdas() int {
	n := 0
	for i := range p.Args {
		if p.esLambda(i) {
			n++
		}
	}
	return n
}

// ---- expression helpers ----

func nuevaVar(i int, t nucleo.Tipo, nombre string) *Expr {
	return &Expr{Var: i, Tipo: t, nombre: nombre}
}

func nuevaConst(v nucleo.Valor, t nucleo.Tipo) *Expr {
	return &Expr{Var: -1, Const: v, EsConst: true, Tipo: t}
}

func nuevoHueco(t nucleo.Tipo) *Expr { return &Expr{Var: -1, Hueco: true, Tipo: t} }

func nuevaOp(p *Primitiva, hijos []*Expr, lambdas []*Lambda) *Expr {
	return &Expr{Op: p, Hijos: hijos, Lambdas: lambdas, Var: -1, Tipo: p.Res}
}

func (e *Expr) esVar() bool { return e != nil && e.Op == nil && !e.EsConst && !e.Hueco && e.Var >= 0 }

// Tamano counts the nodes of e (lambda bodies included, the λ itself not).
func Tamano(e *Expr) int {
	if e == nil {
		return 0
	}
	n := 1
	for _, h := range e.Hijos {
		n += Tamano(h)
	}
	for _, l := range e.Lambdas {
		if l != nil {
			n += Tamano(l.Cuerpo)
		}
	}
	return n
}

// copiarExpr returns a deep copy of the tree (primitives are shared).
func copiarExpr(e *Expr) *Expr {
	if e == nil {
		return nil
	}
	c := *e
	if e.Hijos != nil {
		c.Hijos = make([]*Expr, len(e.Hijos))
		for i, h := range e.Hijos {
			c.Hijos[i] = copiarExpr(h)
		}
	}
	if e.Lambdas != nil {
		c.Lambdas = make([]*Lambda, len(e.Lambdas))
		for i, l := range e.Lambdas {
			if l == nil {
				continue
			}
			c.Lambdas[i] = &Lambda{Params: l.Params, Cuerpo: copiarExpr(l.Cuerpo), nombres: l.nombres}
		}
	}
	return &c
}

// desplazar adds delta to every variable index ≥ desde (re-basing a lambda body into another depth).
func desplazar(e *Expr, desde, delta int) *Expr {
	if e == nil || delta == 0 {
		return e
	}
	c := copiarExpr(e)
	var rec func(x *Expr)
	rec = func(x *Expr) {
		if x == nil {
			return
		}
		if x.esVar() && x.Var >= desde {
			x.Var += delta
		}
		for _, h := range x.Hijos {
			rec(h)
		}
		for _, l := range x.Lambdas {
			if l != nil {
				rec(l.Cuerpo)
			}
		}
	}
	rec(c)
	return c
}

// usaVar reports how many times variable i appears in e.
func usaVar(e *Expr, i int) int {
	if e == nil {
		return 0
	}
	if e.esVar() && e.Var == i {
		return 1
	}
	n := 0
	for _, h := range e.Hijos {
		n += usaVar(h, i)
	}
	for _, l := range e.Lambdas {
		if l != nil {
			n += usaVar(l.Cuerpo, i)
		}
	}
	return n
}

func tieneHuecos(e *Expr) bool {
	if e == nil {
		return false
	}
	if e.Hueco {
		return true
	}
	for _, h := range e.Hijos {
		if tieneHuecos(h) {
			return true
		}
	}
	for _, l := range e.Lambdas {
		if l != nil && tieneHuecos(l.Cuerpo) {
			return true
		}
	}
	return false
}

// String writes the program as an s-expression: "(suma (filtra (λ x (esPar x)) nums))".
func (e *Expr) String() string {
	var sb strings.Builder
	escribirExpr(&sb, e)
	return sb.String()
}

func escribirExpr(sb *strings.Builder, e *Expr) {
	switch {
	case e == nil:
		sb.WriteString("<nil>")
	case e.Hueco:
		sb.WriteString("?")
	case e.EsConst:
		sb.WriteString(textoConst(e.Const, e.Tipo))
	case e.Op == nil:
		if e.nombre != "" {
			sb.WriteString(e.nombre)
		} else {
			sb.WriteString("v" + strconv.Itoa(e.Var))
		}
	default:
		sb.WriteString("(")
		sb.WriteString(e.Op.Nombre)
		ih, il := 0, 0
		for i := range e.Op.Args {
			sb.WriteString(" ")
			if e.Op.esLambda(i) {
				if il < len(e.Lambdas) {
					escribirLambda(sb, e.Lambdas[il])
				} else {
					sb.WriteString("?")
				}
				il++
				continue
			}
			if ih < len(e.Hijos) {
				escribirExpr(sb, e.Hijos[ih])
			} else {
				sb.WriteString("?")
			}
			ih++
		}
		sb.WriteString(")")
	}
}

func escribirLambda(sb *strings.Builder, l *Lambda) {
	if l == nil {
		sb.WriteString("?")
		return
	}
	if l.Cuerpo != nil && l.Cuerpo.Hueco {
		sb.WriteString("?")
		return
	}
	nombres := l.nombres
	if len(nombres) != len(l.Params) {
		nombres = nombresLambda(len(l.Params), nil)
	}
	sb.WriteString("(λ ")
	if len(nombres) == 1 {
		sb.WriteString(nombres[0])
	} else {
		sb.WriteString("(" + strings.Join(nombres, " ") + ")")
	}
	sb.WriteString(" ")
	escribirExpr(sb, l.Cuerpo)
	sb.WriteString(")")
}

// nombresLambda picks k lambda-variable names not in usados.
func nombresLambda(k int, usados map[string]bool) []string {
	var cands []string
	if k == 2 {
		cands = []string{"a", "b", "acc", "x", "u", "v", "p", "q"}
	} else {
		cands = []string{"x", "y", "z", "v", "w", "e", "u", "t"}
	}
	var out []string
	for _, c := range cands {
		if len(out) == k {
			break
		}
		if !usados[c] {
			out = append(out, c)
		}
	}
	for i := 0; len(out) < k; i++ {
		c := "x" + strconv.Itoa(i)
		if !usados[c] {
			out = append(out, c)
		}
	}
	return out
}

func textoConst(v nucleo.Valor, t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CString:
		s, _ := v.(string)
		return strconv.Quote(s)
	case nucleo.CRune:
		i, _ := v.(int)
		return strconv.QuoteRune(rune(i))
	case nucleo.CFloat:
		f, _ := v.(float64)
		s := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eEn") {
			s += ".0"
		}
		return s
	case nucleo.CLista, nucleo.CMapa:
		return compacto(v, t)
	}
	return fmt.Sprint(v)
}

// compacto writes a list/map constant without spaces, so it stays one s-expression token.
func compacto(v nucleo.Valor, t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CLista:
		xs, _ := v.([]nucleo.Valor)
		partes := make([]string, len(xs))
		for i, x := range xs {
			partes[i] = textoConst(x, *t.Elem)
		}
		return "[" + strings.Join(partes, ",") + "]"
	case nucleo.CMapa:
		m, _ := v.(nucleo.Mapa)
		partes := make([]string, len(m))
		for i, p := range m.Ordenada() {
			partes[i] = textoConst(p.K, *t.Clave) + ":" + textoConst(p.V, *t.Elem)
		}
		return "{" + strings.Join(partes, ",") + "}"
	}
	return textoConst(v, t)
}

// ---- evaluation ----

// evaluador runs a compiled expression on an environment.
type evaluador func(env []nucleo.Valor) (nucleo.Valor, error)

// compilar turns e into closures. base is subtracted from every variable index: a closed lambda body is
// compiled with base = index of its first parameter, so its Funcion reads the arguments directly.
func compilar(e *Expr, base int) evaluador {
	switch {
	case e == nil:
		return func([]nucleo.Valor) (nucleo.Valor, error) { return nil, errors.New("sintesis: expresión vacía") }
	case e.Hueco:
		return func([]nucleo.Valor) (nucleo.Valor, error) {
			return nil, errors.New("sintesis: el programa tiene un hueco sin rellenar")
		}
	case e.EsConst:
		v := e.Const
		return func([]nucleo.Valor) (nucleo.Valor, error) { return v, nil }
	case e.Op == nil:
		i := e.Var - base
		return func(env []nucleo.Valor) (nucleo.Valor, error) {
			if i < 0 || i >= len(env) {
				return nil, fmt.Errorf("sintesis: variable %d fuera del entorno", i)
			}
			return env[i], nil
		}
	}
	p := e.Op
	hijos := make([]evaluador, len(e.Hijos))
	for i, h := range e.Hijos {
		hijos[i] = compilar(h, base)
	}
	// a lambda body sees the enclosing environment followed by its own parameters
	lams := make([]evaluador, len(e.Lambdas))
	for i, l := range e.Lambdas {
		if l == nil {
			return func([]nucleo.Valor) (nucleo.Valor, error) {
				return nil, errors.New("sintesis: falta una función")
			}
		}
		lams[i] = compilar(l.Cuerpo, base)
	}
	if p.perezosa && p.Nombre == "si" && len(hijos) == 3 {
		return func(env []nucleo.Valor) (nucleo.Valor, error) {
			c, err := hijos[0](env)
			if err != nil {
				return nil, err
			}
			if b, _ := c.(bool); b {
				return hijos[1](env)
			}
			return hijos[2](env)
		}
	}
	if p.perezosa && (p.Nombre == "y" || p.Nombre == "o") && len(hijos) == 2 {
		corto := p.Nombre == "o"
		return func(env []nucleo.Valor) (nucleo.Valor, error) {
			a, err := hijos[0](env)
			if err != nil {
				return nil, err
			}
			if b, _ := a.(bool); b == corto {
				return corto, nil
			}
			return hijos[1](env)
		}
	}
	eval := p.Eval
	return func(env []nucleo.Valor) (nucleo.Valor, error) {
		args := make([]nucleo.Valor, len(hijos))
		for i, h := range hijos {
			v, err := h(env)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		var fs []Funcion
		if len(lams) > 0 {
			fs = make([]Funcion, len(lams))
			for i := range lams {
				cuerpo := lams[i]
				fs[i] = func(a []nucleo.Valor) (nucleo.Valor, error) {
					nenv := make([]nucleo.Valor, len(env)+len(a))
					copy(nenv, env)
					copy(nenv[len(env):], a)
					return cuerpo(nenv)
				}
			}
		}
		if eval == nil {
			return nil, fmt.Errorf("sintesis: %s no se puede evaluar", p.Nombre)
		}
		return eval(args, fs)
	}
}

// compilarLambdaCerrada compiles a lambda body that only uses its own parameters (which start at slot
// inicio) into a Funcion that reads the arguments directly.
func compilarLambdaCerrada(cuerpo *Expr, inicio int) Funcion {
	ev := compilar(cuerpo, inicio)
	return func(a []nucleo.Valor) (nucleo.Valor, error) { return ev(a) }
}

// Evaluar runs e on env (parameters first). ⊥ is ErrIndefinido; a panic inside a primitive is an error too.
func Evaluar(e *Expr, env []nucleo.Valor) (v nucleo.Valor, err error) {
	defer func() {
		if r := recover(); r != nil {
			v, err = nil, fmt.Errorf("sintesis: fallo interno al evaluar: %v", r)
		}
	}()
	return compilar(e, 0)(env)
}

// Oraculo returns e as a reference implementation: one result per call.
func Oraculo(e *Expr) func([]nucleo.Valor) ([]nucleo.Valor, error) {
	ev := compilar(e, 0)
	return func(ent []nucleo.Valor) (out []nucleo.Valor, err error) {
		defer func() {
			if r := recover(); r != nil {
				out, err = nil, fmt.Errorf("sintesis: fallo interno al evaluar: %v", r)
			}
		}()
		v, err := ev(ent)
		if err != nil {
			return nil, err
		}
		return []nucleo.Valor{v}, nil
	}
}

// primitivasDe lists the primitives used by e (each once, in first-use order).
func primitivasDe(e *Expr) []*Primitiva {
	var out []*Primitiva
	visto := map[*Primitiva]bool{}
	var rec func(x *Expr)
	rec = func(x *Expr) {
		if x == nil {
			return
		}
		if x.Op != nil && !visto[x.Op] {
			visto[x.Op] = true
			out = append(out, x.Op)
		}
		for _, h := range x.Hijos {
			rec(h)
		}
		for _, l := range x.Lambdas {
			if l != nil {
				rec(l.Cuerpo)
			}
		}
	}
	rec(e)
	return out
}

// conceptosDe returns the sorted union of the concept tags of e's primitives.
func conceptosDe(e *Expr) []string {
	m := map[string]bool{}
	for _, p := range primitivasDe(e) {
		for _, c := range p.Conceptos {
			m[c] = true
		}
	}
	out := make([]string, 0, len(m))
	for c := range m {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
