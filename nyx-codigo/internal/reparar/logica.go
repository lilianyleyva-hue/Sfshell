package reparar

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Bounds of logic repair besides Limites.
const (
	casosSobreajuste = 300 // random cases of the overfitting check
	maxParciales     = 20  // best partial mutants combined pairwise
	paresPorLote     = 300 // pairs per build
	maxParchesLogica = 10  // patches returned
	rondasReducir    = 12
)

// candidato is a single mutant or a pair, ready to build.
type candidato struct {
	Fuente    string
	Cambios   []Cambio
	ops       []string
	ediciones int // mutation edits (1 or 2); cleanups do not count
	sospecha  float64
	orden     int
	pasan     int
	variante  int       // variant index in the batch that built it
	eds       []edicion // mutation edits on the original source
}

func (c candidato) parche(total int) Parche {
	return Parche{Fuente: c.Fuente, Cambios: c.Cambios, Pasan: c.pasan, Total: total, Sospecha: c.sospecha}
}

// Logica repairs code that compiles but gives wrong results (§4.4.6):
//
//  1. it builds the original with coverage, runs every case and computes Ochiai for each statement;
//  2. it takes the top 6 statements plus the headers of their loops (the whole file without coverage);
//  3. it generates mutants on those lines (plus the panic fixes when the original panics), drops
//     duplicates and the ones that do not type-check, and keeps at most Limite.Mutantes, most suspicious
//     first;
//  4. it builds them all at once and runs them with PararAlFallar, originally failing cases first;
//  5. it accepts only mutants that pass every case, ranked by fewest edits, then suspiciousness, then
//     Stats.Tasa("mutacion:"+op);
//  6. overfitting check: 300 random cases against the oracle (if any) or the properties; patches that
//     fail are dropped, and two patches that disagree on an input give Ambiguo;
//  7. with no single mutant, it combines the 20 best partial mutants pairwise (at most Limite.Pares,
//     one build per 300 pairs) within Limite.TLogica.
//
// It always reports the minimized failing case of the ORIGINAL code (with an oracle, or when the failure
// is a panic or a property, smaller inputs are tried). With no full patch, Mejor is the best partial one.
func (r *Reparador) Logica(ctx context.Context, fuente string, f nucleo.Firma, casos []nucleo.Caso, props []nucleo.Propiedad, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error), n *nucleo.Nodo) (ResultadoLogica, error) {
	var res ResultadoLogica
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.Ej == nil {
		return res, ErrSinEjecutor
	}
	if len(casos) == 0 {
		return res, errors.New("reparar: no hay casos con los que probar")
	}
	lim := r.Limite.conDefectos()
	ctx, cancelar := context.WithTimeout(ctx, lim.TLogica)
	defer cancelar()
	a := analizar(fuente)
	if a.sintaxis || len(a.errs) > 0 {
		return res, fmt.Errorf("%w: arregla primero los errores de compilación", nucleo.ErrNoCompila)
	}
	paso := n.Sub(nucleo.PasoArreglo, "Busco dónde está el fallo")
	casos = conOraculoLocal(casos, oraculo)

	// 1. the original, with coverage
	instr := nucleo.InstrNormal
	instr.Cobertura = true
	bin, comp, err := r.Ej.Preparar(ctx, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Props: props, Instr: instr, Permiso: nucleo.PermisoAuto})
	if err != nil {
		paso.Mal("No pude preparar el código: %s", motivoPreparar(err, comp))
		return res, err
	}
	defer bin.Cerrar()
	rs, err := bin.Probar(ctx, casos, nucleo.OpcionesProbar{})
	if err != nil {
		paso.Mal("No pude ejecutar las pruebas")
		return res, err
	}
	orig := rs[0]
	okOrig := make([]bool, len(casos))
	pasanOrig := 0
	for i, rc := range orig {
		okOrig[i] = rc.Ejecutado && rc.OK
		if okOrig[i] {
			pasanOrig++
		}
	}
	if pasanOrig == len(casos) {
		res.Motivo = "El código ya pasa las " + strconv.Itoa(len(casos)) + " pruebas: no encontré ningún fallo."
		paso.Info("Pasa las %d pruebas: no encuentro ningún fallo", len(casos))
		return res, nil
	}
	paso.Detalle("Pasa %d de %d pruebas", pasanOrig, len(casos))
	ce := r.contraejemplo(ctx, bin, f, casos, orig, oraculo)
	res.Contraejemplo = &ce
	if ce.ConEsperado() {
		paso.Nota("Falla con %s: esperaba %s", mostrarEntradas(ce.Entradas, f), mostrarSalidas(ce.Esperado, f))
	} else {
		paso.Nota("Falla con %s", mostrarEntradas(ce.Entradas, f))
	}

	// 2. fault localization
	var sosp map[int]float64
	var lineas []int
	if ls := bin.Lineas(0); len(ls) > 0 {
		cub := make([][]int32, len(orig))
		for i, rc := range orig {
			cub[i] = rc.Cubiertas
		}
		sosp, lineas = sospechosas(a, ls, Ochiai(cub, okOrig, len(ls)))
	}
	if len(lineas) > 0 {
		paso.Detalle("Las líneas más sospechosas (Ochiai): %s", unirEnteros(lineas))
	} else {
		paso.Detalle("No tengo datos de cobertura: pruebo cambios en todo el archivo")
	}

	// 3. mutants (+ panic fixes)
	ms := generarMutaciones(a, lineas)
	if msgs := panicosDe(orig); len(msgs) > 0 {
		ms = append(ms, candidatosPanico(a, f, msgs)...)
	}
	sort.SliceStable(ms, func(i, j int) bool {
		si, sj := sosp[ms[i].linea], sosp[ms[j].linea]
		if si != sj {
			return si > sj
		}
		if ti, tj := r.tasa("mutacion:"+ms[i].Operador), r.tasa("mutacion:"+ms[j].Operador); ti != tj {
			return ti > tj
		}
		return false
	})
	if len(ms) > 3*lim.Mutantes {
		ms = ms[:3*lim.Mutantes]
	}
	ms = r.compilables(ms)
	if len(ms) > lim.Mutantes {
		ms = ms[:lim.Mutantes]
	}
	singles := make([]candidato, len(ms))
	for i, m := range ms {
		singles[i] = candidato{Fuente: m.Fuente, Cambios: m.cambios(), ops: []string{m.Operador}, ediciones: 1,
			sospecha: sosp[m.linea], orden: i, eds: m.eds}
	}
	if len(singles) == 0 {
		res.Motivo = "No se me ocurre ningún cambio pequeño que compile."
		paso.Mal("No encontré ningún cambio que probar")
		return res, nil
	}

	// 4–5. one build, stop at the first failure
	ordenCasos := ordenFallidosPrimero(okOrig)
	casosOrd := permutar(casos, ordenCasos)
	pp := paso.Sub(nucleo.PasoPrueba, "Pruebo %d cambios pequeños", len(singles))
	inicio := time.Now()
	lt, err := r.probarLote(ctx, fuente, f, props, singles, casosOrd)
	if err != nil {
		pp.Mal("No pude probar los cambios")
		return res, err
	}
	defer lt.bin.Cerrar()
	aceptados := lt.aceptados
	pp.Detalle("%d variantes en %d ms", len(singles), time.Since(inicio).Milliseconds())

	// 7. second order
	if len(aceptados) == 0 && ctx.Err() == nil {
		pp.Info("Ningún cambio solo arregla todas las pruebas")
		parciales := r.parciales(ctx, lt, casosOrd)
		pares := combinarPares(fuente, parciales, lim.Pares)
		if len(pares) > 0 {
			p2 := paso.Sub(nucleo.PasoPrueba, "Pruebo %d combinaciones de dos cambios", len(pares))
			probados := 0
			for ini := 0; ini < len(pares) && ctx.Err() == nil; ini += paresPorLote {
				fin := min(ini+paresPorLote, len(pares))
				l2, err := r.probarLote(ctx, fuente, f, props, pares[ini:fin], casosOrd)
				if err != nil {
					break
				}
				probados = fin
				if len(l2.aceptados) > 0 {
					lt.bin.Cerrar()
					lt = l2
					aceptados = l2.aceptados
					defer l2.bin.Cerrar()
					break
				}
				parciales = append(parciales, r.parciales(ctx, l2, casosOrd)...)
				l2.bin.Cerrar()
			}
			if len(aceptados) > 0 {
				p2.Bien("Una combinación de dos cambios pasa todas las pruebas")
			} else {
				p2.Mal("Probé %d combinaciones y ninguna pasa todas las pruebas", probados)
			}
		}
		if len(aceptados) == 0 {
			res.Mejor = mejorParcial(parciales, pasanOrig, len(casos))
			res.Motivo = fmt.Sprintf("No encontré ningún cambio pequeño que pase las %d pruebas (probé %d cambios", len(casos), len(singles))
			if len(pares) > 0 {
				res.Motivo += fmt.Sprintf(" y %d combinaciones", len(pares))
			}
			res.Motivo += ")."
			if ctx.Err() != nil {
				res.Motivo += " Se acabó el tiempo."
			}
			paso.Mal("No lo he conseguido")
			return res, nil
		}
	}
	ordenarAceptados(r, aceptados)
	pp.Bien("%d de los cambios pasan todas las pruebas", len(aceptados))

	// 6. overfitting check
	vivos, caidos, amb := r.sobreajuste(ctx, lt, aceptados, f, oraculo)
	if len(vivos) == 0 {
		mejor := aceptados[0].parche(len(casos))
		res.Mejor = &mejor
		res.Motivo = "Encontré cambios que pasan tus pruebas, pero fallan con otras entradas (" + caidos + "): no me fío de ellos."
		paso.Mal("Los arreglos solo sirven para estos ejemplos")
		return res, nil
	}
	if len(vivos) > maxParchesLogica {
		vivos = vivos[:maxParchesLogica]
	}
	for _, c := range vivos {
		res.Parches = append(res.Parches, c.parche(len(casos)))
	}
	if amb != nil {
		amb = recortarAmbiguedad(amb, len(res.Parches))
	}
	if amb != nil {
		res.Ambiguo = amb
		paso.Sub(nucleo.PasoDuda, "Hay %d arreglos que dan resultados distintos con %s", len(amb.Parches), mostrarEntradas(amb.Entrada, f)).Info("")
	}
	for _, op := range vivos[0].ops {
		r.exito("mutacion:"+op, true)
	}
	for _, c := range vivos[0].Cambios {
		paso.Sub(nucleo.PasoArreglo, "%s (línea %d)", c.Porque, c.Linea).Bien("")
	}
	if res.Ambiguo != nil {
		paso.Info("Encontré %d arreglos que pasan las %d pruebas, pero no hacen lo mismo con otras entradas", len(res.Parches), len(casos))
	} else {
		paso.Bien("Lo arreglé: pasa las %d pruebas", len(casos))
	}
	return res, nil
}

// lote is one build of candidates (variant 0 is the original, candidate k is variant k+1).
type lote struct {
	bin       nucleo.Binario
	cands     []candidato
	aceptados []candidato
}

// probarLote builds the original plus cands in one Preparar and runs them with PararAlFallar.
func (r *Reparador) probarLote(ctx context.Context, fuente string, f nucleo.Firma, props []nucleo.Propiedad, cands []candidato, casos []nucleo.Caso) (*lote, error) {
	variantes := make([]string, 0, len(cands)+1)
	variantes = append(variantes, fuente)
	for _, c := range cands {
		variantes = append(variantes, c.Fuente)
	}
	bin, _, err := r.Ej.Preparar(ctx, nucleo.Preparacion{Variantes: variantes, Firma: f, Props: props, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	if err != nil {
		return nil, err
	}
	idx := make([]int, len(cands))
	for k := range cands {
		idx[k] = k + 1
	}
	rs, err := bin.Probar(ctx, casos, nucleo.OpcionesProbar{PararAlFallar: true, Variantes: idx})
	if err != nil {
		bin.Cerrar()
		return nil, err
	}
	lt := &lote{bin: bin, cands: append([]candidato(nil), cands...)}
	for k := range lt.cands {
		lt.cands[k].variante = k + 1
		fila := rs[k+1]
		if len(fila) == len(casos) && contarOK(fila) == len(casos) {
			lt.cands[k].pasan = len(casos)
			lt.aceptados = append(lt.aceptados, lt.cands[k])
		}
	}
	return lt, nil
}

// parciales runs every candidate of a batch on all cases (no stop at the first failure, no rebuild)
// and returns the best partial ones, at most maxParciales: most cases passing, then fewest originally
// passing cases broken, then suspiciousness and operator preference. Candidates that behave exactly like
// the original on every case are left out (they cannot help a pair).
func (r *Reparador) parciales(ctx context.Context, lt *lote, casos []nucleo.Caso) []candidato {
	if len(lt.cands) == 0 || ctx.Err() != nil {
		return nil
	}
	idx := make([]int, 0, len(lt.cands)+1)
	idx = append(idx, 0)
	for k := range lt.cands {
		idx = append(idx, lt.cands[k].variante)
	}
	rs, err := lt.bin.Probar(ctx, casos, nucleo.OpcionesProbar{Variantes: idx})
	if err != nil {
		return nil
	}
	orig := rs[0]
	type parcial struct {
		c     candidato
		rompe int
	}
	var ps []parcial
	for _, c := range lt.cands {
		fila := rs[c.variante]
		if !ejecutadaEntera(fila) {
			continue
		}
		c.pasan = contarOK(fila)
		rompe, igual := 0, true
		for k := range fila {
			if orig[k].OK && !fila[k].OK {
				rompe++
			}
			if claveResultado(fila[k]) != claveResultado(orig[k]) || fila[k].OK != orig[k].OK {
				igual = false
			}
		}
		if igual {
			continue
		}
		ps = append(ps, parcial{c, rompe})
	}
	sort.SliceStable(ps, func(i, j int) bool {
		x, y := ps[i], ps[j]
		if x.c.pasan != y.c.pasan {
			return x.c.pasan > y.c.pasan
		}
		if x.rompe != y.rompe {
			return x.rompe < y.rompe
		}
		if x.c.ediciones != y.c.ediciones {
			return x.c.ediciones < y.c.ediciones
		}
		if x.c.sospecha != y.c.sospecha {
			return x.c.sospecha > y.c.sospecha
		}
		return preferenciaDe(x.c.ops) < preferenciaDe(y.c.ops)
	})
	if len(ps) > maxParciales {
		ps = ps[:maxParciales]
	}
	out := make([]candidato, len(ps))
	for i, p := range ps {
		out[i] = p.c
	}
	return out
}

// combinarPares builds second-order candidates from the best partial single mutants.
func combinarPares(fuente string, parciales []candidato, maxPares int) []candidato {
	var out []candidato
	visto := map[string]bool{normalizarFuente(fuente): true}
	for i := 0; i < len(parciales) && len(out) < maxPares; i++ {
		for j := i + 1; j < len(parciales) && len(out) < maxPares; j++ {
			x, y := parciales[i], parciales[j]
			if x.ediciones != 1 || y.ediciones != 1 || len(x.eds) == 0 || len(y.eds) == 0 || solapanListas(x.eds, y.eds) {
				continue
			}
			eds := append(append([]edicion(nil), x.eds...), y.eds...)
			src, _ := aplicar(fuente, eds)
			src, extra, ok := limpiarYChequear(src)
			if !ok {
				continue
			}
			src = normalizarFuente(src)
			if visto[src] {
				continue
			}
			visto[src] = true
			out = append(out, candidato{
				Fuente:    src,
				Cambios:   append([]Cambio{x.Cambios[0], y.Cambios[0]}, extra...),
				ops:       []string{x.ops[0], y.ops[0]},
				ediciones: 2,
				sospecha:  max(x.sospecha, y.sospecha),
				orden:     len(out),
				eds:       eds,
			})
		}
	}
	return out
}

func solapanListas(a, b []edicion) bool {
	for _, x := range a {
		for _, y := range b {
			if solapan(x, y) || x.ini == y.ini {
				return true
			}
		}
	}
	return false
}

// mejorParcial returns the partial candidate that passes the most cases, when it beats the original.
func mejorParcial(cs []candidato, pasanOrig, total int) *Parche {
	var mejor *candidato
	for i := range cs {
		c := &cs[i]
		if c.pasan <= pasanOrig {
			continue
		}
		if mejor == nil || c.pasan > mejor.pasan || c.pasan == mejor.pasan && c.ediciones < mejor.ediciones {
			mejor = c
		}
	}
	if mejor == nil {
		return nil
	}
	p := mejor.parche(total)
	return &p
}

// preferencia breaks ties between operators with the same success rate: the fixes a person would
// write first (a comparison, an operator, a constant) before the clumsier ones (a negation, a deletion).
var preferencia = map[string]int{
	opRelacional: 0, opPanico: 1, opAritmetico: 2, opConstante: 3, opAcumulador: 4, opLimiteLen: 5,
	opInicioBucle: 6, opIndice: 7, opVariable: 8, opAsignacion: 9, opLogico: 10, opNegar: 11,
	opRamas: 12, opIntercambiar: 13, opBorrar: 14,
}

func preferenciaDe(ops []string) int {
	p := 0
	for _, op := range ops {
		v, ok := preferencia[op]
		if !ok {
			v = len(preferencia)
		}
		p += v
	}
	return p
}

// ordenarAceptados ranks passing candidates: fewest edits, then suspiciousness, then the operator's
// success rate, then a fixed operator preference, then generation order.
func ordenarAceptados(r *Reparador, cs []candidato) {
	tasa := func(c candidato) float64 {
		t := 0.0
		for _, op := range c.ops {
			t += r.tasa("mutacion:" + op)
		}
		return t / float64(max(len(c.ops), 1))
	}
	sort.SliceStable(cs, func(i, j int) bool {
		x, y := cs[i], cs[j]
		if x.ediciones != y.ediciones {
			return x.ediciones < y.ediciones
		}
		if x.sospecha != y.sospecha {
			return x.sospecha > y.sospecha
		}
		if tx, ty := tasa(x), tasa(y); tx != ty {
			return tx > ty
		}
		if px, py := preferenciaDe(x.ops), preferenciaDe(y.ops); px != py {
			return px < py
		}
		return x.orden < y.orden
	})
}

// sobreajuste runs the accepted candidates on 300 random cases. Candidates whose output contradicts the
// oracle, or that break a property, are dropped (caidos says why). Among the survivors, the first input
// on which two of them disagree gives an Ambiguedad (one representative per distinct output, indices
// into the returned list).
func (r *Reparador) sobreajuste(ctx context.Context, lt *lote, aceptados []candidato, f nucleo.Firma, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error)) ([]candidato, string, *Ambiguedad) {
	var azar []nucleo.Caso
	if r.Azar != nil {
		azar = r.Azar(f, casosSobreajuste)
	} else {
		azar = casosAzar(f, casosSobreajuste, semillaSobreajuste)
	}
	if len(azar) == 0 || ctx.Err() != nil {
		return aceptados, "", nil
	}
	azar = conOraculoLocal(azar, oraculo)
	idx := make([]int, len(aceptados))
	for i, c := range aceptados {
		idx[i] = c.variante
	}
	rs, err := lt.bin.Probar(ctx, azar, nucleo.OpcionesProbar{Variantes: idx})
	if err != nil {
		return aceptados, "", nil
	}
	var vivos []candidato
	var filas [][]nucleo.ResultadoCaso
	var motivos []string
	for i, c := range aceptados {
		fila := rs[idx[i]]
		malo := ""
		for k, x := range fila {
			if !x.Ejecutado || rota(x) {
				continue
			}
			if x.PropFallida != "" {
				malo = "no cumple «" + x.PropFallida + "»"
				break
			}
			if azar[k].ConEsperado() && !x.OK {
				malo = "con " + mostrarEntradas(azar[k].Entradas, f) + " no coincide con la referencia"
				break
			}
		}
		if malo != "" {
			motivos = append(motivos, malo)
			continue
		}
		vivos = append(vivos, c)
		filas = append(filas, fila)
	}
	caidos := ""
	if len(motivos) > 0 {
		caidos = motivos[0]
	}
	if len(vivos) < 2 {
		return vivos, caidos, nil
	}
	for k := range azar {
		claves := make([]string, len(vivos))
		distintas := map[string]int{}
		var orden []int
		for v := range vivos {
			claves[v] = claveResultado(filas[v][k])
			if _, ok := distintas[claves[v]]; !ok {
				distintas[claves[v]] = v
				orden = append(orden, v)
			}
		}
		if len(orden) < 2 {
			continue
		}
		amb := &Ambiguedad{Entrada: azar[k].Entradas}
		for _, v := range orden {
			amb.Parches = append(amb.Parches, v)
			var s []nucleo.Valor
			if x := filas[v][k]; x.Ejecutado && !rota(x) {
				s = x.Obtenido
			}
			amb.Salidas = append(amb.Salidas, s)
		}
		return vivos, caidos, amb
	}
	return vivos, caidos, nil
}

// recortarAmbiguedad keeps the patches with an index below n; nil when fewer than two remain.
func recortarAmbiguedad(amb *Ambiguedad, n int) *Ambiguedad {
	out := &Ambiguedad{Entrada: amb.Entrada}
	for i, p := range amb.Parches {
		if p < n {
			out.Parches = append(out.Parches, p)
			out.Salidas = append(out.Salidas, amb.Salidas[i])
		}
	}
	if len(out.Parches) < 2 {
		return nil
	}
	return out
}

// claveResultado identifies an output ("⊥" for a panic, crash or a case that did not run).
func claveResultado(x nucleo.ResultadoCaso) string {
	if !x.Ejecutado || rota(x) {
		return "⊥"
	}
	var sb strings.Builder
	for i, v := range x.Obtenido {
		if i > 0 {
			sb.WriteByte(0x1f)
		}
		sb.WriteString(nucleo.Clave(v))
	}
	return sb.String()
}

// contraejemplo returns the smallest failing case of the original, shrunk when a smaller input can be
// judged (with an oracle, or when the failure is a panic or a broken property).
func (r *Reparador) contraejemplo(ctx context.Context, bin nucleo.Binario, f nucleo.Firma, casos []nucleo.Caso, orig []nucleo.ResultadoCaso, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error)) nucleo.Caso {
	mejor := -1
	for i, rc := range orig {
		if rc.Ejecutado && rc.OK {
			continue
		}
		if mejor < 0 || tamanoEntradas(casos[i].Entradas) < tamanoEntradas(casos[mejor].Entradas) {
			mejor = i
		}
	}
	if mejor < 0 {
		return nucleo.Caso{}
	}
	c := casos[mejor]
	rc := orig[mejor]
	sinEsperado := rota(rc) || rc.PropFallida != ""
	if oraculo == nil && !sinEsperado {
		return c
	}
	falla := func(cs []nucleo.Caso) []bool {
		out := make([]bool, len(cs))
		cs = conOraculoLocal(cs, oraculo)
		rs, err := bin.Probar(ctx, cs, nucleo.OpcionesProbar{})
		if err != nil || len(rs) == 0 {
			return out
		}
		for i, x := range rs[0] {
			if i >= len(out) || !x.Ejecutado {
				continue
			}
			if oraculo == nil {
				out[i] = rota(x) || x.PropFallida != ""
			} else {
				out[i] = !x.OK
			}
		}
		return out
	}
	reducir := r.Reducir
	if reducir == nil {
		reducir = minimizarLocal
	}
	m := reducir(ctx, c, f, falla, rondasReducir)
	if oraculo != nil && !m.ConEsperado() {
		m = conOraculoLocal([]nucleo.Caso{m}, oraculo)[0]
	}
	if m.Origen == "" {
		m.Origen = c.Origen
	}
	return m
}

// conOraculoLocal labels the cases without an expectation with the oracle's output (EspReferencia).
func conOraculoLocal(cs []nucleo.Caso, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error)) []nucleo.Caso {
	out := make([]nucleo.Caso, len(cs))
	copy(out, cs)
	if oraculo == nil {
		return out
	}
	for i := range out {
		if out[i].ConEsperado() {
			continue
		}
		copia := make([]nucleo.Valor, len(out[i].Entradas))
		for j, v := range out[i].Entradas {
			copia[j] = nucleo.Copiar(v)
		}
		if res, err := llamarSeguro(oraculo, copia); err == nil && res != nil {
			out[i].Esperado = res
			out[i].Expectativa = nucleo.EspReferencia
		}
	}
	return out
}

func llamarSeguro(o func([]nucleo.Valor) ([]nucleo.Valor, error), in []nucleo.Valor) (res []nucleo.Valor, err error) {
	defer func() {
		if p := recover(); p != nil {
			res, err = nil, fmt.Errorf("la referencia falló: %v", p)
		}
	}()
	return o(in)
}

// ordenFallidosPrimero returns the case order for PararAlFallar: originally failing cases first.
func ordenFallidosPrimero(ok []bool) []int {
	var out []int
	for i, b := range ok {
		if !b {
			out = append(out, i)
		}
	}
	for i, b := range ok {
		if b {
			out = append(out, i)
		}
	}
	return out
}

func permutar(cs []nucleo.Caso, orden []int) []nucleo.Caso {
	out := make([]nucleo.Caso, len(orden))
	for i, k := range orden {
		out[i] = cs[k]
	}
	return out
}

// mostrarEntradas writes the inputs of a case as a person would: "[1, 2, 3]" or "3, \"hola\"".
func mostrarEntradas(e []nucleo.Valor, f nucleo.Firma) string {
	ts := f.Entradas()
	partes := make([]string, len(e))
	for i, v := range e {
		var t nucleo.Tipo
		if i < len(ts) {
			t = ts[i]
		}
		partes[i] = nucleo.FormatoHumano(v, t)
	}
	return strings.Join(partes, ", ")
}

// mostrarSalidas writes the results of a case.
func mostrarSalidas(vs []nucleo.Valor, f nucleo.Firma) string {
	partes := make([]string, len(vs))
	for i, v := range vs {
		var t nucleo.Tipo
		if i < len(f.Res) {
			t = f.Res[i]
		}
		partes[i] = nucleo.FormatoHumano(v, t)
	}
	return strings.Join(partes, ", ")
}

func unirEnteros(xs []int) string {
	partes := make([]string, len(xs))
	for i, x := range xs {
		partes[i] = strconv.Itoa(x)
	}
	return strings.Join(partes, ", ")
}
