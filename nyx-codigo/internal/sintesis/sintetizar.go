package sintesis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Especificacion is what a synthesis run must satisfy.
type Especificacion struct {
	Firma      nucleo.Firma
	Ejemplos   []nucleo.Caso      // with Esperado
	Sondas     [][]nucleo.Valor   // extra inputs without outputs (≤ 8) used in OE keys; nil → SondasPorDefecto
	Conceptos  map[string]float64 // weights from the frame
	Constantes []nucleo.Valor
	Esqueletos []*Expr
	Pistas     *nucleo.Pistas
	Excluir    []string // Expr.String() of programs to skip ("otra forma")
}

// Opciones bound a run. Zero values take the defaults.
type Opciones struct {
	Limite        time.Duration // default 6 s ("Pensar más" doubles, max 48 s)
	MaxCosto      int           // default 90 (≈ 9–10 nodes)
	MaxBanco      int           // default 400 000 entries
	MaxSoluciones int           // default 5
	Semilla       int64         // the search is deterministic; the seed only names the run
	Priors        nucleo.Contador
}

type Solucion struct {
	Expr  *Expr
	Costo int
}

type Resultado struct {
	Soluciones []Solucion
	Explorados int
	Distintos  int
	Motivo     string // "encontrado" | "tiempo" | "banco_lleno" | "costo_max" | "tipos_no_soportados"
	Ms         int64
}

func (op Opciones) normalizar() Opciones {
	if op.Limite <= 0 {
		op.Limite = 6 * time.Second
	}
	if op.Limite > 48*time.Second {
		op.Limite = 48 * time.Second
	}
	if op.MaxCosto <= 0 {
		op.MaxCosto = 90
	}
	if op.MaxBanco <= 0 {
		op.MaxBanco = 400000
	}
	if op.MaxSoluciones <= 0 {
		op.MaxSoluciones = 5
	}
	return op
}

const (
	maxEjemplos = 64
	maxSondas   = 8
)

// SondasPorDefecto returns the first n probe inputs of nucleo.Sondas that are not among the examples
// (what cerebro passes as Especificacion.Sondas).
func SondasPorDefecto(f nucleo.Firma, ejemplos []nucleo.Caso, n int) [][]nucleo.Valor {
	var out [][]nucleo.Valor
	for _, c := range nucleo.Sondas(f) {
		if len(out) >= n {
			break
		}
		repetido := false
		for _, e := range ejemplos {
			if len(e.Entradas) == len(c.Entradas) && igualVec(e.Entradas, c.Entradas) {
				repetido = true
				break
			}
		}
		if !repetido {
			out = append(out, c.Entradas)
		}
	}
	return out
}

// ---- one synthesis session ----

type candSol struct {
	expr  *Expr
	costo int
	vec   []V
}

type sesion struct {
	ctx      context.Context
	esp      Especificacion
	reg      *Registro
	op       Opciones
	firma    nucleo.Firma
	nParams  int
	prims    []*Primitiva
	costos   map[*Primitiva]int
	envs     [][]V
	nEj      int
	esperado []V
	objetivo int
	consts   []*Expr
	pools    map[string]*pool
	mo       *motor
	sols     []candSol
	excluir  map[string]bool
	tope     int
	nodo     *nucleo.Nodo
	reqConc  []string
	extraLT  []LambdaTipo // lambda types needed by sketches
	evalEsq  int
}

func nuevaSesion(ctx context.Context, esp Especificacion, r *Registro, op Opciones, conSondas bool) (*sesion, error) {
	f := esp.Firma
	if !Sintetizable(f) {
		return nil, fmt.Errorf("%w: la firma %s usa tipos que la síntesis no maneja", nucleo.ErrNoSoportado, f.Go())
	}
	if r == nil {
		r = Base()
	}
	s := &sesion{ctx: ctx, esp: esp, reg: r, op: op, firma: f, nParams: len(f.Params), excluir: map[string]bool{}}
	for _, x := range esp.Excluir {
		s.excluir[x] = true
	}
	res := f.Res[0]
	s.objetivo = idTipo(res)
	if len(esp.Ejemplos) > maxEjemplos {
		return nil, fmt.Errorf("sintesis: como mucho %d ejemplos (hay %d)", maxEjemplos, len(esp.Ejemplos))
	}
	for i, c := range esp.Ejemplos {
		if len(c.Entradas) != s.nParams {
			return nil, fmt.Errorf("sintesis: el ejemplo %d tiene %d entradas y la función recibe %d", i+1, len(c.Entradas), s.nParams)
		}
		if len(c.Esperado) != 1 {
			return nil, fmt.Errorf("sintesis: el ejemplo %d no tiene un resultado", i+1)
		}
		ent := make([]V, s.nParams)
		for j, v := range c.Entradas {
			x, err := canon(v, f.Params[j].Tipo)
			if err != nil {
				return nil, fmt.Errorf("sintesis: ejemplo %d, entrada %d: %v", i+1, j+1, err)
			}
			ent[j] = x
		}
		y, err := canon(c.Esperado[0], res)
		if err != nil {
			return nil, fmt.Errorf("sintesis: ejemplo %d, resultado: %v", i+1, err)
		}
		s.envs = append(s.envs, ent)
		s.esperado = append(s.esperado, y)
	}
	s.nEj = len(s.envs)
	if conSondas {
		sondas := esp.Sondas
		if sondas == nil {
			sondas = SondasPorDefecto(f, esp.Ejemplos, maxSondas)
		}
		for _, sd := range sondas {
			if len(s.envs)-s.nEj >= maxSondas {
				break
			}
			if len(sd) != s.nParams {
				continue
			}
			ent := make([]V, s.nParams)
			ok := true
			for j, v := range sd {
				x, err := canon(v, f.Params[j].Tipo)
				if err != nil {
					ok = false
					break
				}
				ent[j] = x
			}
			if ok {
				s.envs = append(s.envs, ent)
			}
		}
	}
	for _, p := range r.Todas() {
		if idTipo(p.Res) < 0 {
			continue
		}
		s.prims = append(s.prims, p)
	}
	s.costos = calcularCostos(s.prims, esp, op)
	s.consts = s.constantes()
	return s, nil
}

// ---- costs (§4.7 step 1) ----

func calcularCostos(prims []*Primitiva, esp Especificacion, op Opciones) map[*Primitiva]int {
	costos := make(map[*Primitiva]int, len(prims))
	hayIntencion := len(esp.Conceptos) > 0 || (esp.Pistas != nil && (len(esp.Pistas.Llamadas) > 0 || len(esp.Pistas.Operadores) > 0))
	nombres := map[string]bool{}
	total := 0
	if op.Priors != nil {
		for _, p := range prims {
			if !nombres[p.Nombre] {
				nombres[p.Nombre] = true
				total += op.Priors.Usos("componente:" + p.Nombre)
			}
		}
	}
	n := float64(len(nombres))
	for _, p := range prims {
		pBase := math.Pow(2, -p.Costo/10)
		pInt := pBase
		if hayIntencion {
			w := 0.0
			for _, cpt := range p.Conceptos {
				w += esp.Conceptos[cpt]
				if op.Priors != nil {
					for cf, wc := range esp.Conceptos {
						if wc > 0 && op.Priors.Usos("asoc:"+cf+":"+p.Nombre) > 0 && cf == cpt {
							w += 0.5 * wc * op.Priors.Tasa("asoc:"+cf+":"+p.Nombre)
						}
					}
				}
			}
			if esp.Pistas != nil {
				texto := p.Go + " " + p.Ayudante
				for llamada, k := range esp.Pistas.Llamadas {
					if k > 0 && llamada != "" && strings.Contains(texto, llamada) {
						w += 0.3
					}
				}
				for oper, k := range esp.Pistas.Operadores {
					if k > 0 && oper != "" && strings.Contains(p.Go, oper) {
						w += 0.15
					}
				}
			}
			if w > 0 {
				pInt = math.Min(0.95, pBase*(1+3*math.Min(w, 2)))
			} else if len(esp.Conceptos) > 0 && len(p.Conceptos) > 0 {
				pInt = pBase * 0.7
			}
		}
		pBib := pBase
		if op.Priors != nil && total > 0 && n > 0 {
			usos := float64(op.Priors.Usos("componente:" + p.Nombre))
			pBib = math.Min(0.95, pBase*(usos+1)/(float64(total)+n)*n)
		}
		prob := 0.4*pBase + 0.4*pInt + 0.2*pBib
		c := int(math.Round(10 * -math.Log2(prob)))
		if c < 3 {
			c = 3
		}
		if c > 25 {
			c = 25
		}
		costos[p] = c
	}
	return costos
}

// costoExpr is the cost of a whole program on the session's scale (holes cost 0).
func (s *sesion) costoExpr(e *Expr) int {
	switch {
	case e == nil || e.Hueco:
		return 0
	case e.EsConst:
		return costoConst
	case e.Op == nil:
		if e.Var < s.nParams {
			return costoParam
		}
		return costoVarLambda
	}
	c, ok := s.costos[e.Op]
	if !ok {
		c = int(math.Round(e.Op.Costo))
		c = max(3, min(25, c))
	}
	for _, h := range e.Hijos {
		c += s.costoExpr(h)
	}
	for _, l := range e.Lambdas {
		if l != nil {
			c += s.costoExpr(l.Cuerpo)
		}
	}
	return c
}

// ---- constants (§4.7 step 2) ----

func (s *sesion) constantes() []*Expr {
	var out []*Expr
	vistos := map[string]bool{}
	add := func(v V, t nucleo.Tipo) {
		k := t.ClaveTipo() + "|" + nucleo.Clave(v)
		if vistos[k] {
			return
		}
		vistos[k] = true
		out = append(out, nuevaConst(v, t))
	}
	ints, strs, runas := 0, 0, 0
	addInt := func(i int) {
		if ints < 12 && !vistos[tI.ClaveTipo()+"|"+strconv.Itoa(i)] {
			ints++
			add(i, tI)
		}
	}
	addStr := func(x string) {
		if strs < 8 && !vistos[tS.ClaveTipo()+"|"+nucleo.Clave(x)] {
			strs++
			add(x, tS)
		}
	}
	addRuna := func(r rune) {
		if runas < 4 && !vistos[tR.ClaveTipo()+"|"+strconv.Itoa(int(r))] {
			runas++
			add(int(r), tR)
		}
	}
	dados := append([]V(nil), s.esp.Constantes...)
	if s.esp.Pistas != nil {
		dados = append(dados, s.esp.Pistas.Constantes...)
	}
	for _, v := range dados {
		switch x := v.(type) {
		case int:
			addInt(x)
		case int64:
			addInt(int(x))
		case float64:
			if x == math.Trunc(x) && math.Abs(x) < 1e15 {
				addInt(int(x))
			} else {
				add(x, tF)
			}
		case string:
			addStr(x)
			if rs := []rune(x); len(rs) == 1 {
				addRuna(rs[0])
			}
		case rune:
			addRuna(x)
		}
	}
	// numbers in the outputs
	if s.objetivo == idTipo(tI) {
		n := 0
		for _, y := range s.esperado {
			if i, ok := y.(int); ok && i >= -1000 && i <= 1000 && n < 4 {
				addInt(i)
				n++
			}
		}
	}
	for _, i := range []int{0, 1, 2, 10} {
		addInt(i)
	}
	for _, x := range s.flashFill() {
		addStr(x)
	}
	for _, x := range []string{"", " ", ","} {
		addStr(x)
	}
	return out
}

// flashFill proposes string constants: common prefix/suffix of the outputs and separators that appear in
// the outputs but not in the inputs (or the other way round).
func (s *sesion) flashFill() []string {
	var out []string
	if s.nEj == 0 {
		return nil
	}
	var salidas []string
	for _, y := range s.esperado {
		switch x := y.(type) {
		case string:
			salidas = append(salidas, x)
		case []V:
			for _, e := range x {
				if t, ok := e.(string); ok {
					salidas = append(salidas, t)
				}
			}
		}
	}
	var entradas []string
	for _, env := range s.envs[:s.nEj] {
		for _, v := range env {
			recogerTextos(v, &entradas)
		}
	}
	todoEntrada := strings.Join(entradas, "\x00")
	if len(salidas) >= 2 && s.objetivo == idTipo(tS) {
		pre, suf := salidas[0], salidas[0]
		for _, x := range salidas[1:] {
			pre = prefijoComun(pre, x)
			suf = sufijoComun(suf, x)
		}
		if pre != "" && !strings.Contains(todoEntrada, pre) {
			out = append(out, pre)
		}
		if suf != "" && suf != pre && !strings.Contains(todoEntrada, suf) {
			out = append(out, suf)
		}
	}
	for _, x := range append(append([]string(nil), salidas...), entradas...) {
		for _, sep := range separadores(x) {
			out = append(out, sep)
		}
	}
	return out
}

func recogerTextos(v V, dst *[]string) {
	switch x := v.(type) {
	case string:
		*dst = append(*dst, x)
	case []V:
		for _, e := range x {
			recogerTextos(e, dst)
		}
	}
}

func prefijoComun(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	for i > 0 && !utf8ValidoHasta(a, i) {
		i--
	}
	return a[:i]
}

func sufijoComun(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[len(a)-1-i] == b[len(b)-1-i] {
		i++
	}
	for i > 0 && !utf8ValidoHasta(a, len(a)-i) {
		i--
	}
	return a[len(a)-i:]
}

func utf8ValidoHasta(s string, i int) bool {
	return i == 0 || i == len(s) || (s[i]&0xC0) != 0x80
}

// separadores returns the runs of punctuation (and spaces next to them) inside s.
func separadores(s string) []string {
	var out []string
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || unicode.IsSpace(rs[i]) {
			i++
			continue
		}
		j := i
		for j < len(rs) && !unicode.IsLetter(rs[j]) && !unicode.IsDigit(rs[j]) {
			j++
		}
		if j-i <= 3 {
			out = append(out, string(rs[i:j]))
		}
		i = j
	}
	return out
}

// ---- probe values for lambda pools (§4.7 step 3) ----

func (s *sesion) valoresDe(t nucleo.Tipo) []V {
	var out []V
	vistos := map[uint64]bool{}
	add := func(v V) {
		h := hashValor(fnvBase, v)
		if !vistos[h] {
			vistos[h] = true
			out = append(out, v)
		}
	}
	var rec func(v V, tv nucleo.Tipo)
	rec = func(v V, tv nucleo.Tipo) {
		if mismoTipo(tv, t) {
			add(v)
		}
		switch tv.Clase {
		case nucleo.CLista:
			for _, e := range lis(v) {
				rec(e, *tv.Elem)
			}
		case nucleo.CString:
			x := tex(v)
			if t.Clase == nucleo.CString {
				for _, w := range strings.Fields(x) {
					add(w)
				}
			}
			if t.Clase == nucleo.CRune {
				for _, r := range x {
					add(int(r))
				}
			}
		}
	}
	for j, env := range s.envs {
		for i, v := range env {
			rec(v, s.firma.Params[i].Tipo)
		}
		if j < s.nEj {
			rec(s.esperado[j], s.firma.Res[0])
		}
	}
	switch t.Clase {
	case nucleo.CInt:
		for _, i := range []int{-3, -1, 0, 1, 2, 5, 10} {
			add(i)
		}
	case nucleo.CRune:
		for _, r := range "aeiobzAZ09 ñáÉ.," {
			add(int(r))
		}
	case nucleo.CString:
		for _, x := range []string{"", "a", "hola", "Go", "casa", "árbol", "abc", "aeiou", "Ana", "programa"} {
			add(x)
		}
	case nucleo.CBool:
		add(false)
		add(true)
	case nucleo.CFloat:
		for _, f := range []float64{-1.5, 0, 0.5, 1, 2.25} {
			add(f)
		}
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// ---- lambda pools ----

func (s *sesion) tiposLambda() []LambdaTipo {
	var hojas []int
	for _, p := range s.firma.Params {
		hojas = append(hojas, idTipo(p.Tipo))
	}
	for _, c := range s.consts {
		hojas = append(hojas, idTipo(c.Tipo))
	}
	prod, util := alcance(s.prims, hojas, []int{s.objetivo}, nil)
	var out []LambdaTipo
	vistos := map[string]bool{}
	for _, p := range s.prims {
		res := idTipo(p.Res)
		if !util[res] {
			continue
		}
		ok := true
		for i, a := range p.Args {
			if !p.esLambda(i) {
				if t := idTipo(a); t < 0 || !prod[t] {
					ok = false
				}
			}
		}
		if !ok {
			continue
		}
		for i := range p.Args {
			if p.esLambda(i) && !vistos[p.Lambdas[i].clave()] {
				vistos[p.Lambdas[i].clave()] = true
				out = append(out, p.Lambdas[i])
			}
		}
	}
	for _, l := range s.extraLT {
		if !vistos[l.clave()] {
			vistos[l.clave()] = true
			out = append(out, l)
		}
	}
	return out
}

const (
	maxPorPool     = 20000
	maxTamLambda   = 4
	maxCostoLambda = 40
)

func (s *sesion) construirPools(limite time.Time) {
	s.pools = map[string]*pool{}
	grupos := map[string][]LambdaTipo{}
	var ordenGrupos []string
	for _, l := range s.tiposLambda() {
		var sb strings.Builder
		for _, p := range l.Params {
			sb.WriteString(p.ClaveTipo() + ";")
		}
		k := sb.String()
		if _, ok := grupos[k]; !ok {
			ordenGrupos = append(ordenGrupos, k)
		}
		grupos[k] = append(grupos[k], l)
	}
	usados := map[string]bool{}
	for _, p := range s.firma.Params {
		usados[p.Nombre] = true
	}
	for _, k := range ordenGrupos {
		lts := grupos[k]
		params := lts[0].Params
		// probe tuples
		var envs [][]V
		if len(params) == 1 {
			for _, v := range s.valoresDe(params[0]) {
				envs = append(envs, []V{v})
			}
		} else {
			var listas [][]V
			for _, p := range params {
				vs := s.valoresDe(p)
				if len(vs) > 10 {
					vs = vs[:10]
				}
				listas = append(listas, vs)
			}
			envs = producto(listas)
		}
		if len(envs) == 0 {
			continue
		}
		nombres := nombresLambda(len(params), usados)
		mo := nuevoMotor(s.ctx, len(envs), 0, nil, -1)
		mo.maxTam = maxTamLambda
		mo.maxCosto = maxCostoLambda
		mo.maxBanco = 60000
		mo.limite = limite
		mo.usarConc = s.reqConc != nil
		var hojas []int
		for i, p := range params {
			vec := make([]V, len(envs))
			for j := range envs {
				vec[j] = envs[j][i]
			}
			mo.agregarHoja(hojaPendiente{expr: nuevaVar(s.nParams+i, p, nombres[i]), costo: costoVarLambda, vec: vec, fija: true})
			hojas = append(hojas, idTipo(p))
		}
		for _, c := range s.consts {
			vec := make([]V, len(envs))
			for j := range vec {
				vec[j] = c.Const
			}
			mo.agregarHoja(hojaPendiente{expr: c, costo: costoConst, vec: vec})
			hojas = append(hojas, idTipo(c.Tipo))
		}
		var objetivos []int
		for _, l := range lts {
			objetivos = append(objetivos, idTipo(l.Res))
		}
		var primeras []*Primitiva
		for _, p := range s.prims {
			if p.numLambdas() == 0 && p.Nombre != "si" {
				primeras = append(primeras, p)
			}
		}
		prod, util := alcance(primeras, hojas, objetivos, nil)
		mo.prepararOps(primeras, s.costos, prod, util, nil, s.concDe)
		mo.correr(maxCostoLambda)
		for _, l := range lts {
			t := idTipo(l.Res)
			pl := &pool{lt: l, nombres: nombres}
			for c := 0; c < len(mo.nivel[t]); c++ {
				for _, id := range mo.nivel[t][c] {
					if len(pl.cuerpos) >= maxPorPool {
						break
					}
					e := &mo.entradas[id]
					cuerpo := mo.expr(id)
					pl.agregar(cuerpo, e.costo, e.tam, compilarLambdaCerrada(cuerpo, s.nParams), e.conc)
				}
			}
			s.pools[l.clave()] = pl
		}
	}
}

func producto(listas [][]V) [][]V {
	out := [][]V{{}}
	for _, l := range listas {
		var sig [][]V
		for _, pre := range out {
			for _, v := range l {
				fila := append(append([]V(nil), pre...), v)
				sig = append(sig, fila)
			}
		}
		out = sig
	}
	return out
}

func (s *sesion) concDe(p *Primitiva) uint64 {
	var m uint64
	for i, c := range s.reqConc {
		for _, pc := range p.Conceptos {
			if pc == c {
				m |= 1 << uint(i)
			}
		}
	}
	return m
}

// ---- the main bank ----

func (s *sesion) prepararMotor() {
	mo := nuevoMotor(s.ctx, len(s.envs), s.nEj, s.esperado, s.objetivo)
	mo.maxCosto = s.op.MaxCosto
	mo.maxBanco = s.op.MaxBanco
	mo.usarConc = s.reqConc != nil
	var hojas []int
	for i, p := range s.firma.Params {
		vec := make([]V, len(s.envs))
		for j := range s.envs {
			vec[j] = s.envs[j][i]
		}
		mo.agregarHoja(hojaPendiente{expr: nuevaVar(i, p.Tipo, p.Nombre), costo: costoParam, vec: vec, fija: true})
		hojas = append(hojas, idTipo(p.Tipo))
	}
	for _, c := range s.consts {
		vec := make([]V, len(s.envs))
		for j := range vec {
			vec[j] = c.Const
		}
		mo.agregarHoja(hojaPendiente{expr: c, costo: costoConst, vec: vec})
		hojas = append(hojas, idTipo(c.Tipo))
	}
	lambdaOK := func(l LambdaTipo) bool {
		p := s.pools[l.clave()]
		return p != nil && len(p.cuerpos) > 0
	}
	prod, util := alcance(s.prims, hojas, []int{s.objetivo}, lambdaOK)
	mo.prepararOps(s.prims, s.costos, prod, util, s.pools, s.concDe)
	s.mo = mo
}

// agregarSol records a solution unless it is excluded or behaves like one already found.
func (s *sesion) agregarSol(e *Expr, costo int, vec []V) bool {
	if s.excluir[e.String()] {
		return false
	}
	for i, o := range s.sols {
		if igualVec(o.vec[s.nEj:], vec[s.nEj:]) {
			if costo < o.costo {
				s.sols[i] = candSol{expr: e, costo: costo, vec: vec}
			}
			return false
		}
	}
	s.sols = append(s.sols, candSol{expr: e, costo: costo, vec: vec})
	minimo := s.sols[0].costo
	for _, o := range s.sols {
		minimo = min(minimo, o.costo)
	}
	s.tope = int(math.Ceil(float64(minimo) * 1.15))
	if s.mo != nil && s.mo.tPrimera.IsZero() {
		s.mo.tPrimera = time.Now()
	}
	return true
}

// excluido is the motor hook for Excluir: a full-match target candidate whose text is excluded is dropped
// before it enters the bank, so an equivalent program can take its place.
func (s *sesion) instalarExcluir() {
	if len(s.excluir) == 0 {
		return
	}
	mo := s.mo
	orig := mo.alSolucion
	mo.alSolucion = func(id int32) {
		if s.excluir[mo.expr(id).String()] {
			// forget it: remove from the OE table so an equivalent program can replace it
			e := &mo.entradas[id]
			h := hashVec(e.vec)
			lst := mo.visto[e.tipo][h]
			for i, x := range lst {
				if x == id {
					mo.visto[e.tipo][h] = append(lst[:i:i], lst[i+1:]...)
					break
				}
			}
			e.mask = 0
			return
		}
		if orig != nil {
			orig(id)
		}
	}
}

func miles(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var partes []string
	for len(s) > 3 {
		partes = append([]string{s[len(s)-3:]}, partes...)
		s = s[:len(s)-3]
	}
	partes = append([]string{s}, partes...)
	out := strings.Join(partes, ".")
	if neg {
		out = "-" + out
	}
	return out
}

// Sintetizar searches for the cheapest programs that satisfy every example (§4.7).
func Sintetizar(ctx context.Context, esp Especificacion, r *Registro, op Opciones, n *nucleo.Nodo) (Resultado, error) {
	inicio := time.Now()
	op = op.normalizar()
	if !Sintetizable(esp.Firma) {
		return Resultado{Motivo: "tipos_no_soportados"}, fmt.Errorf("%w: la firma %s usa tipos que la síntesis no maneja", nucleo.ErrNoSoportado, esp.Firma.Go())
	}
	if len(esp.Ejemplos) == 0 {
		return Resultado{Motivo: "costo_max"}, errors.New("sintesis: hacen falta ejemplos (usa DesdeConceptos sin ejemplos)")
	}
	ctx, cancel := context.WithTimeout(ctx, op.Limite)
	defer cancel()
	s, err := nuevaSesion(ctx, esp, r, op, true)
	if err != nil {
		return Resultado{}, err
	}
	nodo := n.Sub(nucleo.PasoIntento, "Busco un programa que cumpla los %d ejemplos", s.nEj)
	s.nodo = nodo
	for _, sk := range esp.Esqueletos {
		s.extraLT = append(s.extraLT, lambdasHueco(sk)...)
	}
	limitePools := time.Now().Add(op.Limite / 4)
	s.construirPools(limitePools)
	s.prepararMotor()
	mo := s.mo
	mo.alSolucion = func(id int32) {
		s.agregarSol(mo.expr(id), mo.entradas[id].costo, mo.entradas[id].vec)
	}
	s.instalarExcluir()
	mo.progreso = func() {
		nodo.Progreso("probé %s programas, %s distintos (costo %d)", miles(mo.explorados+s.evalEsq), miles(mo.distintos), mo.nivelHecho+1)
	}
	// sketches compete on the same cost scale
	if len(esp.Esqueletos) > 0 {
		s.probarEsqueletos(esp.Esqueletos, time.Now().Add(min(time.Second, op.Limite/3)))
	}
	motivo := s.buscar()
	resul := s.resultado(motivo, inicio)
	if len(resul.Soluciones) > 0 {
		nodo.Bien("Encontré %d programa(s); el más sencillo: %s", len(resul.Soluciones), resul.Soluciones[0].Expr)
	} else {
		nodo.Mal("No encontré ningún programa (%s; probé %s)", textoMotivo(motivo), miles(resul.Explorados))
	}
	return resul, nil
}

func textoMotivo(m string) string {
	switch m {
	case "tiempo":
		return "se acabó el tiempo"
	case "banco_lleno":
		return "se llenó la memoria de programas"
	case "costo_max":
		return "llegué al tamaño máximo"
	case "tipos_no_soportados":
		return "tipos que no sé manejar"
	}
	return m
}

// buscar runs the bottom-up levels with conditionals until the stopping rule.
func (s *sesion) buscar() string {
	mo := s.mo
	un := &unificador{mo: mo}
	var pendiente *arbolCond
	si := s.reg.buscarTipos("si", tB, s.firma.Res[0], s.firma.Res[0])
	for c := mo.nivelHecho + 1; c <= s.op.MaxCosto; c++ {
		if len(s.sols) > 0 && c > s.tope {
			return "encontrado"
		}
		if len(s.sols) >= s.op.MaxSoluciones {
			return "encontrado"
		}
		mo.correr(c)
		if pendiente == nil && len(s.sols) == 0 && si != nil {
			if a := un.intentar(); a != nil {
				pendiente = a
			}
		} else if pendiente != nil && len(s.sols) == 0 && si != nil {
			if a := un.intentar(); a != nil && a.costo < pendiente.costo {
				pendiente = a
			}
		}
		if pendiente != nil && pendiente.costo <= c {
			s.agregarSol(un.expr(pendiente, si), pendiente.costo, un.vector(pendiente))
			pendiente = nil
		}
		if mo.parar {
			if len(s.sols) > 0 {
				return "encontrado"
			}
			return mo.motivo
		}
	}
	if pendiente != nil {
		s.agregarSol(un.expr(pendiente, si), pendiente.costo, un.vector(pendiente))
	}
	if len(s.sols) > 0 {
		return "encontrado"
	}
	return "costo_max"
}

func (s *sesion) resultado(motivo string, inicio time.Time) Resultado {
	sort.SliceStable(s.sols, func(i, j int) bool {
		a, b := s.sols[i], s.sols[j]
		if a.costo != b.costo {
			return a.costo < b.costo
		}
		ta, tb := Tamano(a.expr), Tamano(b.expr)
		if ta != tb {
			return ta < tb
		}
		return a.expr.String() < b.expr.String()
	})
	res := Resultado{Motivo: motivo, Ms: time.Since(inicio).Milliseconds()}
	if s.mo != nil {
		res.Explorados = s.mo.explorados + s.evalEsq
		res.Distintos = s.mo.distintos
	}
	for i, c := range s.sols {
		if i >= s.op.MaxSoluciones {
			break
		}
		res.Soluciones = append(res.Soluciones, Solucion{Expr: c.expr, Costo: c.costo})
	}
	return res
}
