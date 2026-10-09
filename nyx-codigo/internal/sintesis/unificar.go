package sintesis

import "sort"

// Conditionals (§4.7 step 8, "Unificar"). Every target-type entry carries the mask of the examples it
// satisfies. When the cheapest terms cover every example together, a decision tree is built whose splits are
// bool entries; its cost is the sum of its parts. To avoid memorizing the examples, every leaf must cover at
// least two examples and at least four examples are needed.

type arbolCond struct {
	termino int32 // leaf: entry id (≥ 0)
	pred    int32 // split: bool entry id
	si, no  *arbolCond
	costo   int
}

func (a *arbolCond) hoja() bool { return a.si == nil }

const (
	maxTerminosUnificar = 200
	maxPredsUnificar    = 1500
	maxPredsHondo       = 120
)

type unificador struct {
	mo       *motor
	terminos []int32 // cheapest entry per distinct mask, cost order
	preds    []int32 // bool entries defined on every example, not constant
	pmask    map[int32]uint64
	vistos   int // len(mo.objetivos) at last run
	vistosB  int
	mejor    *arbolCond
}

func (u *unificador) actualizar() bool {
	mo := u.mo
	if len(mo.objetivos) == u.vistos && len(mo.booleanos) == u.vistosB {
		return false
	}
	porMascara := map[uint64]bool{}
	for _, id := range u.terminos {
		porMascara[mo.entradas[id].mask] = true
	}
	for _, id := range mo.objetivos[u.vistos:] {
		m := mo.entradas[id].mask
		if popcount(m) < 2 || porMascara[m] || len(u.terminos) >= maxTerminosUnificar {
			continue
		}
		porMascara[m] = true
		u.terminos = append(u.terminos, id)
	}
	u.vistos = len(mo.objetivos)
	if u.pmask == nil {
		u.pmask = map[int32]uint64{}
	}
	full := mo.completa()
	for _, id := range mo.booleanos[u.vistosB:] {
		if len(u.preds) >= maxPredsUnificar {
			break
		}
		vec := mo.entradas[id].vec
		var m uint64
		ok := true
		for j := 0; j < mo.nEj && j < 64; j++ {
			if esFallo(vec[j]) {
				ok = false
				break
			}
			if boo(vec[j]) {
				m |= 1 << uint(j)
			}
		}
		if !ok || m == 0 || m == full {
			continue
		}
		u.preds = append(u.preds, id)
		u.pmask[id] = m
	}
	u.vistosB = len(mo.booleanos)
	return true
}

// intentar returns a new cheapest tree when one exists.
func (u *unificador) intentar() *arbolCond {
	mo := u.mo
	if mo.nEj < 4 || mo.nEj > 64 {
		return nil
	}
	if !u.actualizar() {
		return nil
	}
	var union uint64
	for _, id := range u.terminos {
		union |= mo.entradas[id].mask
	}
	if union != mo.completa() {
		return nil
	}
	sort.SliceStable(u.preds, func(i, j int) bool {
		return mo.entradas[u.preds[i]].costo < mo.entradas[u.preds[j]].costo
	})
	a := u.construir(mo.completa(), 2)
	if a == nil {
		return nil
	}
	if u.mejor != nil && u.mejor.costo <= a.costo {
		return nil
	}
	u.mejor = a
	return a
}

func (u *unificador) terminoPara(e uint64) *arbolCond {
	if popcount(e) < 2 {
		return nil
	}
	for _, id := range u.terminos { // cost order
		if u.mo.entradas[id].mask&e == e {
			return &arbolCond{termino: id, pred: -1, costo: u.mo.entradas[id].costo}
		}
	}
	return nil
}

func (u *unificador) construir(e uint64, prof int) *arbolCond {
	if t := u.terminoPara(e); t != nil {
		return t
	}
	if prof == 0 || popcount(e) < 4 {
		return nil
	}
	var mejor *arbolCond
	limite := len(u.preds)
	if prof > 1 && limite > maxPredsHondo {
		limite = maxPredsHondo
	}
	for _, p := range u.preds[:limite] {
		pc := u.mo.entradas[p].costo
		if mejor != nil && costoSiUnificar+pc+2*costoParam >= mejor.costo {
			break
		}
		m := u.pmask[p]
		t, f := e&m, e&^m
		if t == 0 || f == 0 {
			continue
		}
		si := u.construir(t, prof-1)
		if si == nil {
			continue
		}
		no := u.construir(f, prof-1)
		if no == nil {
			continue
		}
		total := costoSiUnificar + pc + si.costo + no.costo
		if mejor == nil || total < mejor.costo {
			mejor = &arbolCond{termino: -1, pred: p, si: si, no: no, costo: total}
		}
	}
	return mejor
}

// vector computes the tree's outputs from the parts' cached vectors.
func (u *unificador) vector(a *arbolCond) []V {
	mo := u.mo
	out := make([]V, mo.m)
	for j := range out {
		out[j] = u.valor(a, j)
	}
	return out
}

func (u *unificador) valor(a *arbolCond, j int) V {
	if a.hoja() {
		return u.mo.entradas[a.termino].vec[j]
	}
	c := u.mo.entradas[a.pred].vec[j]
	if esFallo(c) {
		return bottom
	}
	if boo(c) {
		return u.valor(a.si, j)
	}
	return u.valor(a.no, j)
}

// expr builds (si pred a b) using the registry's si instance for the target type.
func (u *unificador) expr(a *arbolCond, si *Primitiva) *Expr {
	if a.hoja() {
		return u.mo.expr(a.termino)
	}
	return nuevaOp(si, []*Expr{u.mo.expr(a.pred), u.expr(a.si, si), u.expr(a.no, si)}, nil)
}
