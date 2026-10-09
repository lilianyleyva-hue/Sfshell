package sintesis

import (
	"context"
	"math/bits"
	"time"

	"nyxcodigo/internal/nucleo"
)

// The bank (§4.7 search steps 4–6): banco[tipo][costo] → entries. Every entry caches its output vector over
// the inputs (examples first, then probes), so a candidate is evaluated by applying its op to the
// children's cached vectors. Observational equivalence keeps one entry per (type, vector).

const (
	costoParam      = 3
	costoConst      = 5
	costoVarLambda  = 3
	costoSiUnificar = 6
)

const numTipos = 12 // len(tiposDSL)

type entrada struct {
	tipo  int
	costo int
	tam   int
	op    *Primitiva
	hijos []int32
	lams  []int32 // indexes into the slot's pool
	pools []*pool // pool of each lambda (parallel to lams)
	hoja  *Expr
	vec   []V
	mask  uint64 // examples satisfied (target type only)
	conc  uint64 // concept bits (DesdeConceptos)
}

// pool is the lambda pool of one lambda type.
type pool struct {
	lt       LambdaTipo
	cuerpos  []*Expr
	costos   []int
	tams     []int
	fns      []Funcion
	conc     []uint64
	porCosto [][]int32
	nombres  []string
}

func (p *pool) agregar(cuerpo *Expr, costo, tam int, fn Funcion, conc uint64) {
	id := int32(len(p.cuerpos))
	p.cuerpos = append(p.cuerpos, cuerpo)
	p.costos = append(p.costos, costo)
	p.tams = append(p.tams, tam)
	p.fns = append(p.fns, fn)
	p.conc = append(p.conc, conc)
	for len(p.porCosto) <= costo {
		p.porCosto = append(p.porCosto, nil)
	}
	p.porCosto[costo] = append(p.porCosto[costo], id)
}

func (p *pool) lambda(i int32) *Lambda {
	return &Lambda{Params: p.lt.Params, Cuerpo: p.cuerpos[i], nombres: p.nombres}
}

type slotMotor struct {
	lambda bool
	tipo   int
	pool   *pool
}

type opMotor struct {
	p     *Primitiva
	costo int
	res   int
	slots []slotMotor
	nHij  int
	nLam  int
	conc  uint64
	mismo bool // commutative with two non-lambda args of the same type
}

type hojaPendiente struct {
	expr  *Expr
	costo int
	vec   []V
	conc  uint64
	fija  bool // parameters are always kept
}

type motor struct {
	ctx       context.Context
	m         int // vector length
	nEj       int
	esperado  []V
	objetivo  int // -1: none
	entradas  []entrada
	nivel     [numTipos][][]int32
	visto     [numTipos]map[uint64][]int32
	ops       []opMotor
	hojas     map[int][]hojaPendiente
	maxCosto  int
	maxBanco  int
	maxTam    int
	usarConc  bool
	llenoMax  int // stop after this many entries of the target type (0 = no limit)
	nivelHecho int

	explorados int
	distintos  int
	parar      bool
	motivo     string
	limite     time.Time // hard deadline (zero = none)
	tPrimera   time.Time
	alSolucion func(id int32)
	progreso   func()
	ultimoPro  time.Time

	objetivos []int32 // target-type entries in insertion order
	booleanos []int32 // bool entries in insertion order

	sel    []int32
	argBuf []V
	hvBuf  [][]V
	fnsBuf []Funcion
	scr    []V
}

func nuevoMotor(ctx context.Context, m, nEj int, esperado []V, objetivo int) *motor {
	mo := &motor{ctx: ctx, m: m, nEj: nEj, esperado: esperado, objetivo: objetivo, hojas: map[int][]hojaPendiente{},
		maxCosto: 90, maxBanco: 400000}
	for t := 0; t < numTipos; t++ {
		mo.visto[t] = map[uint64][]int32{}
	}
	mo.sel = make([]int32, 8)
	mo.argBuf = make([]V, 8)
	mo.hvBuf = make([][]V, 8)
	mo.fnsBuf = make([]Funcion, 8)
	mo.scr = make([]V, m)
	return mo
}

func (mo *motor) agregarHoja(h hojaPendiente) {
	mo.hojas[h.costo] = append(mo.hojas[h.costo], h)
}

func hashVec(vec []V) uint64 {
	h := uint64(fnvBase)
	for _, v := range vec {
		h = hashValor(h, v)
	}
	return h
}

func igualVec(a, b []V) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !igualExacto(a[i], b[i]) {
			return false
		}
	}
	return true
}

// duplicado returns the id of an entry of type t with the same vector, or -1.
func (mo *motor) duplicado(t int, h uint64, vec []V) int32 {
	for _, id := range mo.visto[t][h] {
		if igualVec(mo.entradas[id].vec, vec) {
			return id
		}
	}
	return -1
}

func (mo *motor) mascara(vec []V) uint64 {
	var m uint64
	for j := 0; j < mo.nEj && j < 64; j++ {
		if !esFallo(vec[j]) && nucleo.Igual(vec[j], mo.esperado[j]) {
			m |= 1 << uint(j)
		}
	}
	return m
}

func (mo *motor) completa() uint64 {
	if mo.nEj >= 64 {
		return ^uint64(0)
	}
	return 1<<uint(mo.nEj) - 1
}

// insertar adds an entry (vec is copied) and returns its id.
func (mo *motor) insertar(e entrada, h uint64) int32 {
	id := int32(len(mo.entradas))
	e.vec = append([]V(nil), e.vec...)
	if e.tipo == mo.objetivo && mo.nEj > 0 {
		e.mask = mo.mascara(e.vec)
	}
	mo.entradas = append(mo.entradas, e)
	for len(mo.nivel[e.tipo]) <= e.costo {
		mo.nivel[e.tipo] = append(mo.nivel[e.tipo], nil)
	}
	mo.nivel[e.tipo][e.costo] = append(mo.nivel[e.tipo][e.costo], id)
	mo.visto[e.tipo][h] = append(mo.visto[e.tipo][h], id)
	mo.distintos++
	if e.tipo == mo.objetivo {
		mo.objetivos = append(mo.objetivos, id)
	}
	if e.tipo == 1 { // bool
		mo.booleanos = append(mo.booleanos, id)
	}
	if len(mo.entradas) >= mo.maxBanco && !mo.parar {
		mo.parar = true
		mo.motivo = "banco_lleno"
	}
	if e.tipo == mo.objetivo && mo.nEj > 0 && e.mask == mo.completa() && mo.alSolucion != nil {
		mo.alSolucion(id)
	}
	return id
}

func (mo *motor) ids(s slotMotor, c int) []int32 {
	if s.lambda {
		if c < len(s.pool.porCosto) {
			return s.pool.porCosto[c]
		}
		return nil
	}
	if c < len(mo.nivel[s.tipo]) {
		return mo.nivel[s.tipo][c]
	}
	return nil
}

// correr processes cost levels up to hasta (inclusive), continuing from where it stopped.
func (mo *motor) correr(hasta int) {
	if hasta > mo.maxCosto {
		hasta = mo.maxCosto
	}
	for c := mo.nivelHecho + 1; c <= hasta && !mo.parar; c++ {
		mo.procesarNivel(c)
		if !mo.parar {
			mo.nivelHecho = c
		}
	}
}

func (mo *motor) procesarNivel(c int) {
	for _, h := range mo.hojas[c] {
		t := idTipo(h.expr.Tipo)
		if t < 0 {
			continue
		}
		hh := hashVec(h.vec)
		if d := mo.duplicado(t, hh, h.vec); d >= 0 && !h.fija {
			if mo.usarConc {
				mo.entradas[d].conc |= h.conc
			}
			continue
		}
		mo.insertar(entrada{tipo: t, costo: c, tam: 1, hoja: h.expr, vec: h.vec, conc: h.conc}, hh)
		if mo.parar {
			return
		}
	}
	for oi := range mo.ops {
		op := &mo.ops[oi]
		rem := c - op.costo
		if rem < costoVarLambda*len(op.slots) {
			continue
		}
		mo.elegir(op, 0, rem, c)
		if mo.parar {
			return
		}
	}
}

func (mo *motor) elegir(op *opMotor, i, rem, c int) {
	s := op.slots[i]
	if i == len(op.slots)-1 {
		for _, id := range mo.ids(s, rem) {
			mo.sel[i] = id
			mo.probar(op, c)
			if mo.parar {
				return
			}
		}
		return
	}
	resto := costoVarLambda * (len(op.slots) - 1 - i)
	for ci := 1; ci <= rem-resto; ci++ {
		lista := mo.ids(s, ci)
		for _, id := range lista {
			mo.sel[i] = id
			mo.elegir(op, i+1, rem-ci, c)
			if mo.parar {
				return
			}
		}
	}
}

func (mo *motor) revisar() {
	if mo.ctx.Err() != nil {
		mo.parar = true
		mo.motivo = "tiempo"
		return
	}
	ahora := time.Now()
	if !mo.limite.IsZero() && ahora.After(mo.limite) {
		mo.parar = true
		mo.motivo = "tiempo"
		return
	}
	if !mo.tPrimera.IsZero() && ahora.Sub(mo.tPrimera) > time.Second {
		mo.parar = true
		mo.motivo = "encontrado"
		return
	}
	if mo.progreso != nil && ahora.Sub(mo.ultimoPro) >= 250*time.Millisecond {
		mo.ultimoPro = ahora
		mo.progreso()
	}
}

func (mo *motor) probar(op *opMotor, c int) {
	mo.explorados++
	if mo.explorados&1023 == 0 {
		mo.revisar()
		if mo.parar {
			return
		}
	}
	sel := mo.sel[:len(op.slots)]
	// pruning (c): commutativity and identities
	if op.mismo && sel[0] > sel[1] {
		return
	}
	tam := 1
	var conc uint64 = op.conc
	nh := 0
	for i, s := range op.slots {
		if s.lambda {
			tam += s.pool.tams[sel[i]]
			conc |= s.pool.conc[sel[i]]
			continue
		}
		hijo := &mo.entradas[sel[i]]
		if nh == 0 && (op.p.involutiva || op.p.idempotente) && hijo.op == op.p {
			return
		}
		tam += hijo.tam
		conc |= hijo.conc
		mo.hvBuf[nh] = hijo.vec
		nh++
	}
	if mo.maxTam > 0 && tam > mo.maxTam {
		return
	}
	nl := 0
	for i, s := range op.slots {
		if s.lambda {
			mo.fnsBuf[nl] = s.pool.fns[sel[i]]
			nl++
		}
	}
	vec := mo.scr
	if !mo.evaluar(op, vec) {
		return // (b) ⊥ on every input
	}
	h := hashVec(vec)
	if d := mo.duplicado(op.res, h, vec); d >= 0 { // (a) observational equivalence
		if mo.usarConc {
			mo.entradas[d].conc |= conc
		}
		return
	}
	e := entrada{tipo: op.res, costo: c, tam: tam, op: op.p, vec: vec, conc: conc}
	e.hijos = make([]int32, 0, nh)
	for i, s := range op.slots {
		if s.lambda {
			e.lams = append(e.lams, sel[i])
			e.pools = append(e.pools, s.pool)
		} else {
			e.hijos = append(e.hijos, sel[i])
		}
	}
	mo.insertar(e, h)
}

// evaluar applies op to the children's cached vectors; it reports whether some input has a result.
func (mo *motor) evaluar(op *opMotor, out []V) (algun bool) {
	defer func() {
		if r := recover(); r != nil {
			algun = false
		}
	}()
	p := op.p
	hv := mo.hvBuf[:op.nHij]
	fns := mo.fnsBuf[:op.nLam]
	args := mo.argBuf[:op.nHij]
	nombre := p.Nombre
	for j := 0; j < mo.m; j++ {
		var v V
		switch {
		case p.perezosa && nombre == "si":
			cnd := hv[0][j]
			switch {
			case esFallo(cnd):
				v = bottom
			case boo(cnd):
				v = hv[1][j]
			default:
				v = hv[2][j]
			}
		case p.perezosa && nombre == "y":
			a := hv[0][j]
			switch {
			case esFallo(a):
				v = bottom
			case !boo(a):
				v = false
			default:
				v = hv[1][j]
			}
		case p.perezosa && nombre == "o":
			a := hv[0][j]
			switch {
			case esFallo(a):
				v = bottom
			case boo(a):
				v = true
			default:
				v = hv[1][j]
			}
		default:
			ok := true
			for h := range hv {
				a := hv[h][j]
				if esFallo(a) {
					ok = false
					break
				}
				args[h] = a
			}
			if !ok {
				v = bottom
				break
			}
			r, err := p.Eval(args, fns)
			if err != nil {
				v = bottom
			} else {
				v = r
			}
		}
		out[j] = v
		if !esFallo(v) {
			algun = true
		}
	}
	return algun
}

// expr rebuilds the program of an entry.
func (mo *motor) expr(id int32) *Expr {
	e := &mo.entradas[id]
	if e.hoja != nil {
		return e.hoja
	}
	x := &Expr{Op: e.op, Var: -1, Tipo: e.op.Res}
	for _, h := range e.hijos {
		x.Hijos = append(x.Hijos, mo.expr(h))
	}
	for i, l := range e.lams {
		x.Lambdas = append(x.Lambdas, e.pools[i].lambda(l))
	}
	return x
}

// prepararOps builds the op table for the given primitives and costs, keeping only ops whose argument types
// can be produced and whose result can reach one of the wanted types.
func (mo *motor) prepararOps(prims []*Primitiva, costos map[*Primitiva]int, producibles [numTipos]bool,
	utiles [numTipos]bool, pools map[string]*pool, conc func(p *Primitiva) uint64) {
	for _, p := range prims {
		res := idTipo(p.Res)
		if res < 0 || !utiles[res] {
			continue
		}
		costo, ok := costos[p]
		if !ok || costo > mo.maxCosto {
			continue
		}
		op := opMotor{p: p, costo: costo, res: res}
		valido := true
		for i, a := range p.Args {
			if p.esLambda(i) {
				pl := pools[p.Lambdas[i].clave()]
				if pl == nil || len(pl.cuerpos) == 0 {
					valido = false
					break
				}
				op.slots = append(op.slots, slotMotor{lambda: true, pool: pl})
				op.nLam++
				continue
			}
			t := idTipo(a)
			if t < 0 || !producibles[t] {
				valido = false
				break
			}
			op.slots = append(op.slots, slotMotor{tipo: t})
			op.nHij++
		}
		if !valido || len(op.slots) == 0 {
			continue
		}
		if p.conmutativa && op.nHij == 2 && op.nLam == 0 && op.slots[0].tipo == op.slots[1].tipo {
			op.mismo = true
		}
		if conc != nil {
			op.conc = conc(p)
		}
		mo.ops = append(mo.ops, op)
	}
	n := 0
	for _, op := range mo.ops {
		if len(op.slots) > n {
			n = len(op.slots)
		}
	}
	if n > len(mo.sel) {
		mo.sel = make([]int32, n)
		mo.argBuf = make([]V, n)
		mo.hvBuf = make([][]V, n)
		mo.fnsBuf = make([]Funcion, n)
	}
}

// alcance computes which types can be produced from the leaves and which can reach the wanted types.
func alcance(prims []*Primitiva, hojas []int, objetivos []int, lambdaOK func(LambdaTipo) bool) (prod, util [numTipos]bool) {
	for _, t := range hojas {
		if t >= 0 {
			prod[t] = true
		}
	}
	for cambio := true; cambio; {
		cambio = false
		for _, p := range prims {
			res := idTipo(p.Res)
			if res < 0 || prod[res] {
				continue
			}
			ok := true
			for i, a := range p.Args {
				if p.esLambda(i) {
					if lambdaOK != nil && !lambdaOK(p.Lambdas[i]) {
						ok = false
					}
					continue
				}
				if t := idTipo(a); t < 0 || !prod[t] {
					ok = false
				}
			}
			if ok {
				prod[res] = true
				cambio = true
			}
		}
	}
	for _, t := range objetivos {
		if t >= 0 {
			util[t] = true
		}
	}
	for cambio := true; cambio; {
		cambio = false
		for _, p := range prims {
			res := idTipo(p.Res)
			if res < 0 || !util[res] || !prod[res] {
				continue
			}
			for i, a := range p.Args {
				if p.esLambda(i) {
					continue
				}
				if t := idTipo(a); t >= 0 && prod[t] && !util[t] {
					util[t] = true
					cambio = true
				}
			}
		}
	}
	return prod, util
}

func popcount(x uint64) int { return bits.OnesCount64(x) }
