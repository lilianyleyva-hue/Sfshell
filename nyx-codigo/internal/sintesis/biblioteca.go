package sintesis

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Library learning (§4.7 step 13): learned functions become primitives (cost 6) whose Eval and Go are
// derived from their Expr, and Comprimir invents one-hole abstractions shared by several learned programs.

const (
	costoAprendida  = 6
	costoInvento    = 6
	maxInventos     = 50
	cadaComprimir   = 5
	minTamInvento   = 3
	minGananciaInv  = 4
	maxSondasInvent = 30
)

// Biblioteca is a registry plus the learned components and the inventions made from them. It is safe for
// concurrent use.
type Biblioteca struct {
	mu         sync.Mutex
	reg        *Registro
	aprendidas []*Primitiva
	inventos   []*Primitiva
	desde      int // additions since the last Comprimir
}

// NuevaBiblioteca wraps r (Base() when nil). Learned components are added to r itself.
func NuevaBiblioteca(r *Registro) *Biblioteca {
	if r == nil {
		r = Base()
	}
	return &Biblioteca{reg: r}
}

// Registro returns the registry with the shipped, learned and invented primitives.
func (b *Biblioteca) Registro() *Registro { return b.reg }

// Aprendidas lists the learned components (not the inventions), oldest first.
func (b *Biblioteca) Aprendidas() []*Primitiva {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*Primitiva(nil), b.aprendidas...)
}

// Inventos lists the abstractions invented by Comprimir.
func (b *Biblioteca) Inventos() []*Primitiva {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*Primitiva(nil), b.inventos...)
}

// Aprender registers e (a program over f's parameters) as a primitive called nombre, with cost 6, a phrase
// made from descripcion and Go codegen as a call to a helper generated from e. Learning a name again
// replaces the old component. Every 5 additions it runs Comprimir.
func (b *Biblioteca) Aprender(nombre, descripcion string, e *Expr, f nucleo.Firma) (*Primitiva, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, err := b.aprender(nombre, descripcion, e, f)
	if err != nil {
		return nil, err
	}
	b.desde++
	if b.desde >= cadaComprimir {
		b.desde = 0
		b.comprimir()
	}
	return p, nil
}

func (b *Biblioteca) aprender(nombre, descripcion string, e *Expr, f nucleo.Firma) (*Primitiva, error) {
	if e == nil {
		return nil, errors.New("sintesis: no hay programa que aprender")
	}
	if !esIdent(nombre) || strings.ContainsAny(nombre, "λ?") {
		return nil, fmt.Errorf("sintesis: «%s» no sirve como nombre de función", nombre)
	}
	if tieneHuecos(e) {
		return nil, errors.New("sintesis: el programa tiene huecos sin rellenar")
	}
	if !Sintetizable(f) {
		return nil, fmt.Errorf("%w: la firma %s usa tipos que la síntesis no maneja", nucleo.ErrNoSoportado, f.Go())
	}
	if !mismoTipo(e.Tipo, f.Res[0]) {
		return nil, fmt.Errorf("sintesis: el programa da %s y la función debe dar %s", e.Tipo.Go(), f.Res[0].Go())
	}
	if varMaxima(e) >= len(f.Params)+profundidadMax(e) {
		return nil, errors.New("sintesis: el programa usa variables que la función no tiene")
	}
	for _, q := range b.reg.instancias(nombre) {
		if q.Cuerpo == nil {
			return nil, fmt.Errorf("sintesis: «%s» ya es una primitiva de base", nombre)
		}
		if usaPrimitiva(e, q) {
			return nil, fmt.Errorf("sintesis: «%s» no puede usarse a sí misma", nombre)
		}
	}
	p := nuevaCompuesta(nombre, fraseAprendida(nombre, descripcion, f), e, f, costoAprendida)
	b.reg.quitar(nombre)
	b.reg.Agregar(p)
	var resto []*Primitiva
	for _, q := range b.aprendidas {
		if q.Nombre != nombre {
			resto = append(resto, q)
		}
	}
	b.aprendidas = append(resto, p)
	return p, nil
}

// Olvidar removes a learned component from the registry. Inventions stay: each one was checked on its own
// and keeps working (its body holds the primitives it needs). It reports whether nombre was known.
func (b *Biblioteca) Olvidar(nombre string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	var resto []*Primitiva
	hallada := false
	for _, q := range b.aprendidas {
		if q.Nombre == nombre {
			hallada = true
			continue
		}
		resto = append(resto, q)
	}
	if !hallada {
		return false
	}
	b.aprendidas = resto
	b.reg.quitar(nombre)
	return true
}

// nuevaCompuesta builds a primitive whose behaviour is the expression e over f's parameters.
func nuevaCompuesta(nombre, frase string, e *Expr, f nucleo.Firma, costo float64) *Primitiva {
	cuerpo := copiarExpr(e)
	ev := compilar(cuerpo, 0)
	firma := f
	firma.Nombre = nombre
	firma.Params = append([]nucleo.Param(nil), f.Params...)
	firma.Res = []nucleo.Tipo{f.Res[0]}
	args := make([]nucleo.Tipo, len(f.Params))
	partes := make([]string, len(f.Params))
	for i, p := range f.Params {
		args[i] = p.Tipo
		partes[i] = "{" + strconv.Itoa(i) + "}"
	}
	parcial := false
	for _, q := range primitivasDe(cuerpo) {
		parcial = parcial || q.Parcial
	}
	return &Primitiva{
		Nombre: nombre, Args: args, Res: f.Res[0], Costo: costo, Conceptos: conceptosDe(cuerpo),
		Eval: func(a []V, _ []Funcion) (V, error) {
			env := make([]V, len(a))
			copy(env, a)
			return ev(env)
		},
		Go:      nombre + "(" + strings.Join(partes, ", ") + ")",
		Frase:   frase,
		Cuerpo:  cuerpo,
		Parcial: parcial,
		prec:    7,
		firma:   &firma,
	}
}

// varMaxima returns the largest variable slot used in e (-1 if none).
func varMaxima(e *Expr) int {
	m := -1
	var rec func(x *Expr)
	rec = func(x *Expr) {
		if x == nil {
			return
		}
		if x.esVar() && x.Var > m {
			m = x.Var
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
	return m
}

// profundidadMax is the largest number of nested lambda parameters in e.
func profundidadMax(e *Expr) int {
	if e == nil {
		return 0
	}
	m := 0
	for _, h := range e.Hijos {
		m = max(m, profundidadMax(h))
	}
	for _, l := range e.Lambdas {
		if l != nil {
			m = max(m, len(l.Params)+profundidadMax(l.Cuerpo))
		}
	}
	return m
}

func usaPrimitiva(e *Expr, p *Primitiva) bool {
	for _, q := range primitivasDe(e) {
		if q == p || (q.Cuerpo != nil && q.Nombre == p.Nombre) {
			return true
		}
	}
	return false
}

// Cargar learns the stored functions that carry a DSL program. Entries may use each other in any order.
func (b *Biblioteca) Cargar(fs []nucleo.FuncionAprendida) (cargadas int, errs []error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var pendientes []nucleo.FuncionAprendida
	for _, fa := range fs {
		if strings.TrimSpace(fa.DSL) != "" {
			pendientes = append(pendientes, fa)
		}
	}
	ultimos := map[string]error{}
	for avance := true; avance && len(pendientes) > 0; {
		avance = false
		var quedan []nucleo.FuncionAprendida
		for _, fa := range pendientes {
			e, err := Parse(fa.DSL, b.reg, fa.Firma)
			if err != nil {
				ultimos[fa.Nombre] = err
				quedan = append(quedan, fa)
				continue
			}
			if _, err := b.aprender(fa.Nombre, fa.Descripcion, e, fa.Firma); err != nil {
				errs = append(errs, fmt.Errorf("sintesis: no pude cargar %s: %w", fa.Nombre, err))
				continue
			}
			cargadas++
			b.desde++
			avance = true
		}
		pendientes = quedan
	}
	for _, fa := range pendientes {
		errs = append(errs, fmt.Errorf("sintesis: no pude leer el programa de %s: %w", fa.Nombre, ultimos[fa.Nombre]))
	}
	if b.desde >= cadaComprimir {
		b.desde = 0
		b.comprimir()
	}
	return cargadas, errs
}

// Comprimir invents one-hole abstractions shared by the learned programs (at most 50 in total) and returns
// the new ones.
func (b *Biblioteca) Comprimir() []*Primitiva {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.desde = 0
	return b.comprimir()
}

// ocurrencia is one subtree of a learned program that matches an abstraction.
type ocurrencia struct {
	prog int
	nodo *Expr
	arg  *Expr // what fills the hole
}

type candidatoInv struct {
	clave  string
	cuerpo *Expr // over slot 0 (the hole) and its own lambda slots from 1
	hueco  nucleo.Tipo
	tam    int
	ocs    []ocurrencia
}

func (b *Biblioteca) comprimir() []*Primitiva {
	if len(b.inventos) >= maxInventos || len(b.aprendidas) < 2 {
		return nil
	}
	cands := map[string]*candidatoInv{}
	var orden []string
	existentes := map[string]bool{}
	for _, p := range append(append([]*Primitiva(nil), b.aprendidas...), b.inventos...) {
		if p.firma != nil && len(p.firma.Params) == 1 {
			existentes[claveCanonica(p.Cuerpo)] = true
		}
	}
	for i, p := range b.aprendidas {
		nPar := len(p.firma.Params)
		recorrerSubarboles(p.Cuerpo, nPar, 0, func(t *Expr, base int, raiz bool) {
			if raiz || t.Op == nil || Tamano(t) < minTamInvento {
				return
			}
			for _, ab := range abstracciones(t, base) {
				k := claveCanonica(ab.cuerpo)
				c := cands[k]
				if c == nil {
					c = &candidatoInv{clave: k, cuerpo: ab.cuerpo, hueco: ab.hueco, tam: Tamano(ab.cuerpo)}
					cands[k] = c
					orden = append(orden, k)
				}
				c.ocs = append(c.ocs, ocurrencia{prog: i, nodo: t, arg: ab.arg})
			}
		})
	}
	var lista []*candidatoInv
	for _, k := range orden {
		c := cands[k]
		if existentes[k] || (len(c.ocs)-1)*(c.tam-1) < minGananciaInv {
			continue
		}
		lista = append(lista, c)
	}
	sort.SliceStable(lista, func(i, j int) bool {
		gi := (len(lista[i].ocs) - 1) * (lista[i].tam - 1)
		gj := (len(lista[j].ocs) - 1) * (lista[j].tam - 1)
		return gi > gj
	})
	var nuevos []*Primitiva
	for _, c := range lista {
		if len(b.inventos) >= maxInventos {
			break
		}
		p := b.inventar(c)
		if p == nil {
			continue
		}
		nuevos = append(nuevos, p)
	}
	return nuevos
}

type abstraccion struct {
	cuerpo *Expr
	hueco  nucleo.Tipo
	arg    *Expr
}

// abstracciones returns the one-hole abstractions of t: its only free variable becomes the hole, or (when t
// is closed) one of its constants does.
func abstracciones(t *Expr, base int) []abstraccion {
	libres := map[int]*Expr{}
	var consts []*Expr
	var rec func(x *Expr)
	rec = func(x *Expr) {
		if x == nil {
			return
		}
		switch {
		case x.esVar():
			if x.Var < base {
				libres[x.Var] = x
			}
		case x.EsConst:
			consts = append(consts, x)
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
	rec(t)
	var out []abstraccion
	switch len(libres) {
	case 1:
		for slot, v := range libres {
			if idTipo(v.Tipo) < 0 {
				return nil
			}
			cuerpo := rebasar(t, base, func(x *Expr) *Expr {
				if x.esVar() && x.Var == slot {
					return &Expr{Var: 0, Tipo: v.Tipo, nombre: nombreHueco(v.Tipo)}
				}
				return nil
			})
			out = append(out, abstraccion{cuerpo: cuerpo, hueco: v.Tipo, arg: v})
		}
	case 0:
		for _, k := range consts {
			k := k
			if idTipo(k.Tipo) < 0 {
				continue
			}
			cuerpo := rebasar(t, base, func(x *Expr) *Expr {
				if x == k {
					return &Expr{Var: 0, Tipo: k.Tipo, nombre: nombreHueco(k.Tipo)}
				}
				return nil
			})
			out = append(out, abstraccion{cuerpo: cuerpo, hueco: k.Tipo, arg: k})
		}
	}
	return out
}

func nombreHueco(t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CLista:
		return "xs"
	case nucleo.CString:
		return "s"
	case nucleo.CRune:
		return "r"
	case nucleo.CBool:
		return "b"
	case nucleo.CFloat:
		return "f"
	}
	return "n"
}

// rebasar copies t (whose bound variables start at base) into a standalone body: the hole goes to slot 0
// (via cambio) and the bound variables move to 1, 2…
func rebasar(t *Expr, base int, cambio func(x *Expr) *Expr) *Expr {
	var rec func(x *Expr) *Expr
	rec = func(x *Expr) *Expr {
		if x == nil {
			return nil
		}
		if r := cambio(x); r != nil {
			return r
		}
		if x.esVar() {
			c := *x
			if x.Var >= base {
				c.Var = x.Var - base + 1
			}
			return &c
		}
		c := *x
		if x.Hijos != nil {
			c.Hijos = make([]*Expr, len(x.Hijos))
			for i, h := range x.Hijos {
				c.Hijos[i] = rec(h)
			}
		}
		if x.Lambdas != nil {
			c.Lambdas = make([]*Lambda, len(x.Lambdas))
			for i, l := range x.Lambdas {
				if l != nil {
					c.Lambdas[i] = &Lambda{Params: l.Params, Cuerpo: rec(l.Cuerpo), nombres: l.nombres}
				}
			}
		}
		return &c
	}
	return rec(t)
}

// recorrerSubarboles visits every node of e with the slot where its bound variables start.
func recorrerSubarboles(e *Expr, nPar, prof int, f func(t *Expr, base int, raiz bool)) {
	var rec func(x *Expr, prof int, raiz bool)
	rec = func(x *Expr, prof int, raiz bool) {
		if x == nil {
			return
		}
		f(x, nPar+prof, raiz)
		for _, h := range x.Hijos {
			rec(h, prof, false)
		}
		for _, l := range x.Lambdas {
			if l != nil {
				rec(l.Cuerpo, prof+len(l.Params), false)
			}
		}
	}
	rec(e, prof, true)
}

// claveCanonica prints e with variables as slots and without lambda names.
func claveCanonica(e *Expr) string {
	var sb strings.Builder
	var rec func(x *Expr)
	rec = func(x *Expr) {
		switch {
		case x == nil:
			sb.WriteString("_")
		case x.Hueco:
			sb.WriteString("?")
		case x.EsConst:
			sb.WriteString(x.Tipo.ClaveTipo() + ":" + nucleo.Clave(x.Const))
		case x.Op == nil:
			sb.WriteString("$" + strconv.Itoa(x.Var))
		default:
			sb.WriteString("(" + clavePrim(x.Op))
			for _, h := range x.Hijos {
				sb.WriteString(" ")
				rec(h)
			}
			for _, l := range x.Lambdas {
				sb.WriteString(" λ")
				if l != nil {
					rec(l.Cuerpo)
				}
			}
			sb.WriteString(")")
		}
	}
	rec(e)
	return sb.String()
}

// inventar checks a candidate on every learned program that uses it and registers it.
func (b *Biblioteca) inventar(c *candidatoInv) *Primitiva {
	nombre := b.nombreInvento(c.cuerpo)
	firma := nucleo.Firma{Nombre: nombre, Params: []nucleo.Param{{Nombre: nombreHueco(c.hueco), Tipo: c.hueco}},
		Res: []nucleo.Tipo{c.cuerpo.Tipo}}
	frase := Describir(c.cuerpo, nucleo.Firma{Params: []nucleo.Param{{Nombre: "{0}", Tipo: c.hueco}}, Res: firma.Res})
	p := nuevaCompuesta(nombre, frase, c.cuerpo, firma, costoInvento)
	// re-check: every rewritten program evaluates the same as the original
	porProg := map[int][]ocurrencia{}
	for _, oc := range c.ocs {
		porProg[oc.prog] = append(porProg[oc.prog], oc)
	}
	for i, ocs := range porProg {
		orig := b.aprendidas[i]
		nuevo := reescribir(orig.Cuerpo, ocs, p)
		if !mismoComportamiento(orig.Cuerpo, nuevo, *orig.firma) {
			return nil
		}
	}
	b.reg.Agregar(p)
	b.inventos = append(b.inventos, p)
	return p
}

// reescribir replaces the occurrences in e by calls to the invention.
func reescribir(e *Expr, ocs []ocurrencia, p *Primitiva) *Expr {
	cambia := map[*Expr]*Expr{}
	for _, oc := range ocs {
		cambia[oc.nodo] = oc.arg
	}
	var rec func(x *Expr) *Expr
	rec = func(x *Expr) *Expr {
		if x == nil {
			return nil
		}
		if arg, ok := cambia[x]; ok {
			return nuevaOp(p, []*Expr{copiarExpr(arg)}, nil)
		}
		c := *x
		if x.Hijos != nil {
			c.Hijos = make([]*Expr, len(x.Hijos))
			for i, h := range x.Hijos {
				c.Hijos[i] = rec(h)
			}
		}
		if x.Lambdas != nil {
			c.Lambdas = make([]*Lambda, len(x.Lambdas))
			for i, l := range x.Lambdas {
				if l != nil {
					c.Lambdas[i] = &Lambda{Params: l.Params, Cuerpo: rec(l.Cuerpo), nombres: l.nombres}
				}
			}
		}
		return &c
	}
	return rec(e)
}

// mismoComportamiento compares two programs on the signature's probes (both ⊥, or equal).
func mismoComportamiento(a, b *Expr, f nucleo.Firma) bool {
	oa, ob := Oraculo(a), Oraculo(b)
	sondas := nucleo.Sondas(f)
	if len(sondas) > maxSondasInvent {
		sondas = sondas[:maxSondasInvent]
	}
	for _, c := range sondas {
		ra, ea := oa(c.Entradas)
		rb, eb := ob(c.Entradas)
		if (ea != nil) != (eb != nil) {
			return false
		}
		if ea == nil && !nucleo.Igual(ra[0], rb[0]) {
			return false
		}
	}
	return true
}

var nombresOperador = map[string]string{
	"+": "suma", "-": "resta", "*": "por", "/": "entre", "%": "resto", "<": "menor", ">": "mayor", "<=": "menorIgual",
	">=": "mayorIgual", "==": "igual", "!=": "distinto",
}

// nombreInvento names an abstraction after its operations: (filtra esPar v) → filtraEsPar.
func (b *Biblioteca) nombreInvento(cuerpo *Expr) string {
	var partes []string
	var rec func(x *Expr)
	rec = func(x *Expr) {
		if x == nil || len(partes) >= 3 {
			return
		}
		if x.Op != nil {
			if n, ok := nombresOperador[x.Op.Nombre]; ok {
				partes = append(partes, n)
			} else if esIdent(x.Op.Nombre) {
				partes = append(partes, x.Op.Nombre)
			}
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
	rec(cuerpo)
	base := "invento"
	if len(partes) > 0 {
		base = partes[0]
		for _, s := range partes[1:] {
			rs := []rune(s)
			rs[0] = unicode.ToUpper(rs[0])
			base += string(rs)
		}
	}
	n := base
	for i := 2; len(b.reg.instancias(n)) > 0; i++ {
		n = base + strconv.Itoa(i)
	}
	return n
}
