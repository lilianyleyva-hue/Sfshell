package logica

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// NombreRegla names the classic inference rule that takes some of the premises to the conclusion:
// "modus ponens", "modus tollens", "silogismo hipotético", "silogismo disyuntivo", "dilema
// constructivo", "dilema destructivo", "De Morgan", "simplificación", "conjunción", "adición",
// "doble negación". It returns "" when no rule matches. It does not check validity by itself.
func NombreRegla(prem []Formula, concl Formula) string {
	if concl == nil {
		return ""
	}
	for i, p := range prem {
		for j, q := range prem {
			if i == j {
				continue
			}
			if imp, ok := p.(Implica); ok {
				if Igual(q, imp.A) && Igual(concl, imp.B) {
					return "modus ponens"
				}
				if Igual(q, negar(imp.B)) && Igual(concl, negar(imp.A)) {
					return "modus tollens"
				}
				if imp2, ok := q.(Implica); ok && Igual(imp.B, imp2.A) {
					if c, ok := concl.(Implica); ok && Igual(c.A, imp.A) && Igual(c.B, imp2.B) {
						return "silogismo hipotético"
					}
				}
			}
			if eq, ok := p.(Equiv); ok {
				if (Igual(q, eq.A) && Igual(concl, eq.B)) || (Igual(q, eq.B) && Igual(concl, eq.A)) {
					return "modus ponens"
				}
			}
			if o, ok := p.(O); ok {
				if (Igual(q, negar(o.A)) && Igual(concl, o.B)) || (Igual(q, negar(o.B)) && Igual(concl, o.A)) {
					return "silogismo disyuntivo"
				}
			}
		}
	}
	// dilemmas: A→C, B→D, A∨B ⊢ C∨D (C when C = D); A→C, B→D, ¬C∨¬D ⊢ ¬A∨¬B
	for i, p := range prem {
		ip, ok := p.(Implica)
		if !ok {
			continue
		}
		for j, q := range prem {
			iq, ok := q.(Implica)
			if !ok || i == j {
				continue
			}
			for k, r := range prem {
				o, ok := r.(O)
				if !ok || k == i || k == j {
					continue
				}
				if Igual(o.A, ip.A) && Igual(o.B, iq.A) {
					if Igual(concl, O{ip.B, iq.B}) || (Igual(ip.B, iq.B) && Igual(concl, ip.B)) {
						return "dilema constructivo"
					}
				}
				if Igual(o.A, negar(ip.B)) && Igual(o.B, negar(iq.B)) && Igual(concl, O{negar(ip.A), negar(iq.A)}) {
					return "dilema destructivo"
				}
			}
		}
	}
	for _, p := range prem {
		if esDeMorgan(p, concl) || esDeMorgan(concl, p) {
			return "De Morgan"
		}
		if n, ok := p.(No); ok {
			if n2, ok := n.F.(No); ok && Igual(n2.F, concl) {
				return "doble negación"
			}
		}
		if y, ok := p.(Y); ok && (Igual(y.A, concl) || Igual(y.B, concl)) {
			return "simplificación"
		}
		if o, ok := concl.(O); ok && (Igual(o.A, p) || Igual(o.B, p)) {
			return "adición"
		}
	}
	if y, ok := concl.(Y); ok {
		a, b := false, false
		for _, p := range prem {
			a = a || Igual(p, y.A)
			b = b || Igual(p, y.B)
		}
		if a && b {
			return "conjunción"
		}
	}
	return ""
}

// esDeMorgan: ¬(A∧B) ⇔ ¬A∨¬B and ¬(A∨B) ⇔ ¬A∧¬B.
func esDeMorgan(p, c Formula) bool {
	n, ok := p.(No)
	if !ok {
		return false
	}
	switch g := n.F.(type) {
	case Y:
		o, ok := c.(O)
		return ok && Igual(o.A, negar(g.A)) && Igual(o.B, negar(g.B))
	case O:
		y, ok := c.(Y)
		return ok && Igual(y.A, negar(g.A)) && Igual(y.B, negar(g.B))
	}
	return false
}

// Falacia names a classic fallacy in the argument: "afirmar el consecuente", "negar el
// antecedente", "afirmar un disyunto", "invertir el condicional". It returns "" when none matches.
// Callers report it only when the argument is invalid.
func Falacia(prem []Formula, concl Formula) string {
	if concl == nil {
		return ""
	}
	for i, p := range prem {
		for j, q := range prem {
			if i == j {
				continue
			}
			if imp, ok := p.(Implica); ok {
				if Igual(q, imp.B) && Igual(concl, imp.A) {
					return "afirmar el consecuente"
				}
				if Igual(q, negar(imp.A)) && Igual(concl, negar(imp.B)) {
					return "negar el antecedente"
				}
			}
			if o, ok := p.(O); ok {
				if (Igual(q, o.A) && Igual(concl, negar(o.B))) || (Igual(q, o.B) && Igual(concl, negar(o.A))) {
					return "afirmar un disyunto"
				}
			}
		}
	}
	for _, p := range prem {
		if imp, ok := p.(Implica); ok {
			if c, ok := concl.(Implica); ok && Igual(c.A, imp.B) && Igual(c.B, imp.A) {
				return "invertir el condicional"
			}
		}
	}
	return ""
}

// ExplicarRegla describes a named rule or fallacy in one Spanish sentence.
func ExplicarRegla(nombre string) string {
	switch nombre {
	case "modus ponens":
		return "Modus ponens: de «si A, entonces B» y «A» se sigue «B»."
	case "modus tollens":
		return "Modus tollens: de «si A, entonces B» y «no B» se sigue «no A»."
	case "silogismo hipotético":
		return "Silogismo hipotético: de «si A, entonces B» y «si B, entonces C» se sigue «si A, entonces C»."
	case "silogismo disyuntivo":
		return "Silogismo disyuntivo: de «A o B» y «no A» se sigue «B»."
	case "dilema constructivo":
		return "Dilema constructivo: de «si A, entonces C», «si B, entonces D» y «A o B» se sigue «C o D»."
	case "dilema destructivo":
		return "Dilema destructivo: de «si A, entonces C», «si B, entonces D» y «no C o no D» se sigue «no A o no B»."
	case "De Morgan":
		return "De Morgan: «no (A y B)» equivale a «no A o no B», y «no (A o B)» equivale a «no A y no B»."
	case "simplificación":
		return "Simplificación: de «A y B» se sigue «A»."
	case "conjunción":
		return "Conjunción: de «A» y «B» se sigue «A y B»."
	case "adición":
		return "Adición: de «A» se sigue «A o B»."
	case "doble negación":
		return "Doble negación: «no no A» es lo mismo que «A»."
	case "afirmar el consecuente":
		return "Falacia de afirmar el consecuente: de «si A, entonces B» y «B» no se sigue «A» (B puede tener otra causa)."
	case "negar el antecedente":
		return "Falacia de negar el antecedente: de «si A, entonces B» y «no A» no se sigue «no B»."
	case "afirmar un disyunto":
		return "Falacia de afirmar un disyunto: de «A o B» y «A» no se sigue «no B» (pueden ser verdad las dos)."
	case "invertir el condicional":
		return "Falacia de invertir el condicional: «si A, entonces B» no es lo mismo que «si B, entonces A»."
	}
	return ""
}

// --- clauses and resolution

// clausula is a sorted set of literals (atom i ↔ i+1, negated −(i+1)).
type clausula []int

func (c clausula) clave() string {
	var sb strings.Builder
	for _, l := range c {
		sb.WriteString(strconv.Itoa(l))
		sb.WriteByte(',')
	}
	return sb.String()
}

func normalizarClausula(ls []int) (clausula, bool) {
	vistos := map[int]bool{}
	var out clausula
	for _, l := range ls {
		if vistos[-l] {
			return nil, false // tautology
		}
		if !vistos[l] {
			vistos[l] = true
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if abs(a) != abs(b) {
			return abs(a) < abs(b)
		}
		return a < b
	})
	return out, true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// nnf pushes negations to the atoms and removes → and ↔.
func nnf(f Formula, neg bool) Formula {
	switch g := f.(type) {
	case Atomo:
		if neg {
			return No{g}
		}
		return g
	case No:
		return nnf(g.F, !neg)
	case Y:
		if neg {
			return O{nnf(g.A, true), nnf(g.B, true)}
		}
		return Y{nnf(g.A, false), nnf(g.B, false)}
	case O:
		if neg {
			return Y{nnf(g.A, true), nnf(g.B, true)}
		}
		return O{nnf(g.A, false), nnf(g.B, false)}
	case Implica:
		return nnf(O{No{g.A}, g.B}, neg)
	case Equiv:
		return nnf(Y{O{No{g.A}, g.B}, O{g.A, No{g.B}}}, neg)
	}
	return f
}

// cnfDirecta converts f to clauses over the atoms by distribution; ok is false past maxClausulas.
func cnfDirecta(f Formula, maxClausulas int) ([]clausula, bool) {
	var rec func(f Formula) ([][]int, bool)
	rec = func(f Formula) ([][]int, bool) {
		switch g := f.(type) {
		case Atomo:
			return [][]int{{g.N + 1}}, true
		case No:
			if a, ok := g.F.(Atomo); ok {
				return [][]int{{-(a.N + 1)}}, true
			}
		case Y:
			a, ok := rec(g.A)
			if !ok {
				return nil, false
			}
			b, ok := rec(g.B)
			if !ok {
				return nil, false
			}
			r := append(a, b...)
			return r, len(r) <= maxClausulas
		case O:
			a, ok := rec(g.A)
			if !ok {
				return nil, false
			}
			b, ok := rec(g.B)
			if !ok || len(a)*len(b) > maxClausulas {
				return nil, false
			}
			var r [][]int
			for _, x := range a {
				for _, y := range b {
					r = append(r, append(append([]int(nil), x...), y...))
				}
			}
			return r, true
		}
		return nil, false
	}
	crudas, ok := rec(nnf(f, false))
	if !ok {
		return nil, false
	}
	var out []clausula
	for _, c := range crudas {
		if n, ok := normalizarClausula(c); ok {
			out = append(out, n)
		}
	}
	return out, true
}

func textoClausula(c clausula, nombres []string) string {
	if len(c) == 0 {
		return "□"
	}
	var partes []string
	for _, l := range c {
		i := abs(l) - 1
		n := NombreAtomo(i)
		if i < len(nombres) && nombres[i] != "" {
			n = nombres[i]
		}
		if l < 0 {
			n = "¬" + n
		}
		partes = append(partes, n)
	}
	return strings.Join(partes, " ∨ ")
}

const (
	maxClausulasResolucion = 12
	maxResolventes         = 2000
)

// Resolucion tries a resolution refutation of prem ∧ ¬concl (≤ 12 clauses, ≤ 2000 resolvents).
// When it finds the empty clause it returns the numbered proof (only the clauses used) and true.
func Resolucion(prem []Formula, concl Formula, nAtomos int) ([]string, bool) {
	return resolucionCon(prem, concl, nil)
}

func resolucionCon(prem []Formula, concl Formula, nombres []string) ([]string, bool) {
	type nodo struct {
		c       clausula
		origen  string
		p1, p2  int
		usado   bool
		numeral int
	}
	var nodos []nodo
	vistas := map[string]bool{}
	agregar := func(c clausula, origen string, p1, p2 int) int {
		k := c.clave()
		if vistas[k] {
			return -1
		}
		vistas[k] = true
		nodos = append(nodos, nodo{c: c, origen: origen, p1: p1, p2: p2})
		return len(nodos) - 1
	}
	for _, p := range prem {
		cs, ok := cnfDirecta(p, maxClausulasResolucion)
		if !ok {
			return []string{"Hay demasiadas cláusulas para una prueba por resolución."}, false
		}
		for _, c := range cs {
			agregar(c, "premisa", -1, -1)
		}
	}
	if concl != nil {
		cs, ok := cnfDirecta(No{concl}, maxClausulasResolucion)
		if !ok {
			return []string{"Hay demasiadas cláusulas para una prueba por resolución."}, false
		}
		for _, c := range cs {
			agregar(c, "negación de la conclusión", -1, -1)
		}
	}
	if len(nodos) > maxClausulasResolucion {
		return []string{fmt.Sprintf("Hay %d cláusulas; solo hago pruebas por resolución con %d o menos.", len(nodos), maxClausulasResolucion)}, false
	}
	vacia := -1
	for i, n := range nodos {
		if len(n.c) == 0 {
			vacia = i
		}
	}
	resolventes := 0
	subsumida := func(c clausula) bool {
		for _, n := range nodos {
			if len(n.c) <= len(c) && incluyeLits(c, n.c) {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(nodos) && vacia < 0 && resolventes < maxResolventes; i++ {
		for j := 0; j < i && vacia < 0 && resolventes < maxResolventes; j++ {
			a, b := nodos[i].c, nodos[j].c
			pivote := 0
			for _, l := range a {
				for _, m := range b {
					if l == -m {
						if pivote != 0 {
							pivote = 1 << 30 // more than one clash: tautological resolvent
						} else {
							pivote = l
						}
					}
				}
			}
			if pivote == 0 || pivote == 1<<30 {
				continue
			}
			var ls []int
			for _, l := range a {
				if l != pivote {
					ls = append(ls, l)
				}
			}
			for _, m := range b {
				if m != -pivote {
					ls = append(ls, m)
				}
			}
			c, ok := normalizarClausula(ls)
			if !ok {
				continue
			}
			resolventes++
			if len(c) > 0 && subsumida(c) {
				continue
			}
			if k := agregar(c, "", j, i); k >= 0 && len(c) == 0 {
				vacia = k
			}
		}
	}
	if vacia < 0 {
		return nil, false
	}
	// mark the clauses used by the refutation and number them in order
	var marcar func(i int)
	marcar = func(i int) {
		if i < 0 || nodos[i].usado {
			return
		}
		nodos[i].usado = true
		marcar(nodos[i].p1)
		marcar(nodos[i].p2)
	}
	marcar(vacia)
	var pasos []string
	num := 0
	for i := range nodos {
		if !nodos[i].usado {
			continue
		}
		num++
		nodos[i].numeral = num
		n := nodos[i]
		var por string
		if n.p1 < 0 {
			por = n.origen
		} else {
			por = fmt.Sprintf("de %d y %d", nodos[n.p1].numeral, nodos[n.p2].numeral)
		}
		linea := fmt.Sprintf("%d. %s   (%s)", num, textoClausula(n.c, nombres), por)
		if len(n.c) == 0 {
			linea += ": contradicción, así que la conclusión se sigue"
		}
		pasos = append(pasos, linea)
	}
	return pasos, true
}

func incluyeLits(sup, sub clausula) bool {
	for _, l := range sub {
		ok := false
		for _, m := range sup {
			if l == m {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// --- truth tables

const maxAtomosTablaVerdad = 12

// TablaVerdad builds the truth table of fs over the atoms they use (≤ max atoms, and never more
// than 12). Rows go from all-true to all-false; a row where every formula is true is marked "bien".
func TablaVerdad(fs []Formula, a *Atomos, max int) (nucleo.Tabla, error) {
	if max <= 0 || max > maxAtomosTablaVerdad {
		max = maxAtomosTablaVerdad
	}
	usados := atomosUsados(fs...)
	if len(usados) > max {
		return nucleo.Tabla{}, fmt.Errorf("logica: %d átomos son demasiados para una tabla de verdad (máximo %d)", len(usados), max)
	}
	nombres := a.Nombres(maxAtomo(fs...) + 1)
	tb := nucleo.Tabla{Titulo: "Tabla de verdad"}
	if ley := a.Leyenda(usados, nombres); ley != "" {
		tb.Titulo += ": " + ley
	}
	for _, i := range usados {
		tb.Cabecera = append(tb.Cabecera, nombres[i])
	}
	for _, f := range fs {
		tb.Cabecera = append(tb.Cabecera, EscribirCon(f, nombres))
	}
	n := len(usados)
	v := make([]bool, maxAtomo(fs...)+1)
	for fila := 0; fila < 1<<n; fila++ {
		var celdas []string
		for k, i := range usados {
			v[i] = fila&(1<<(n-1-k)) == 0
			celdas = append(celdas, vf(v[i]))
		}
		todas := true
		for _, f := range fs {
			x := Evaluar(f, v)
			todas = todas && x
			celdas = append(celdas, vf(x))
		}
		tb.Filas = append(tb.Filas, celdas)
		if todas {
			tb.Marcas = append(tb.Marcas, "bien")
		} else {
			tb.Marcas = append(tb.Marcas, "")
		}
	}
	return tb, nil
}

func vf(b bool) string {
	if b {
		return "V"
	}
	return "F"
}

// tablaArgumento is the truth table of an argument: premises then conclusion. Rows where the
// premises are true are marked "bien" if the conclusion is true and "mal" if it is false
// (a counter-model). It also returns the number of counter-model rows.
func tablaArgumento(prem []Formula, concl Formula, a *Atomos) (nucleo.Tabla, int, error) {
	fs := append(append([]Formula(nil), prem...), concl)
	tb, err := TablaVerdad(fs, a, 5)
	if err != nil {
		return tb, 0, err
	}
	nu := len(atomosUsados(fs...))
	contras := 0
	for i, fila := range tb.Filas {
		premV := true
		for k := range prem {
			premV = premV && fila[nu+k] == "V"
		}
		switch {
		case premV && fila[len(fila)-1] == "V":
			tb.Marcas[i] = "bien"
		case premV:
			tb.Marcas[i] = "mal"
			contras++
		default:
			tb.Marcas[i] = ""
		}
	}
	return tb, contras, nil
}
