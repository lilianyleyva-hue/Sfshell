package cerebro

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/saber"
	"nyxcodigo/internal/sintesis"
)

// crear_funcion (§4.14.2): Entender → Memoria → Recetas → Síntesis → Codegen y verificación → Web →
// Guardar, or the honest failure. Follow-ups (otra forma, está mal, ahora con…) arrive with k.excluir,
// k.ejemplosExtra and a fused k.q already set.

// Limits of the pipeline.
const (
	crearUmbralMemoria     = 0.75 // memoria hit that is reused directly
	crearUmbralReceta      = 0.5  // recipe tried at all
	crearUmbralRecetaSola  = 0.75 // recipe accepted without user examples
	crearMaxRondas         = 3    // Distinguir questions
	crearMaxCandidatos     = 3    // synthesized programs taken to codegen
	crearMaxCasosGuardados = 40
	crearRepetidos         = 20 // the first cases run twice (determinism)
	crearMaxVariantesWeb   = 12
	crearCercaCosto        = 3 // without examples, programs this close in cost are "two readings"
)

// opcionesRecibe are the buttons of "¿Qué recibe la función?".
var opcionesRecibe = []string{"una lista de números", "un texto", "un número", "una lista de palabras", "dos números"}

// crearHecho is the outcome of crear_funcion; crear_programa wraps a successful one into a program.
type crearHecho struct {
	ok           bool
	sinComprobar bool // code exists but there was no sandbox to check it
	codigo       string
	firma        nucleo.Firma
	casos        []nucleo.Caso // cases that ran with an expectation and passed (user ones first)
	resp         nucleo.Respuesta
}

// crearIntento is the closest attempt so far (for the failure answer).
type crearIntento struct {
	codigo       string
	pasan, total int
	contra       string
	tabla        *nucleo.Tabla
}

type crearFlujo struct {
	c          *Cerebro
	k          *tarea
	f          nucleo.Firma
	usuario    []nucleo.Caso // EspUsuario: examples, counterexamples, answers to questions
	props      []nucleo.Propiedad
	necesarios []string // concepts a stored function or recipe must have (action, modifiers, predicates)
	otraForma  bool
	sinArenero bool
	dudas      []string
	mejor      crearIntento
	intentos   []string
}

// pipeCrearFuncion is the pipeline to crear funciones (crear_funcion, §4.14.2).
func pipeCrearFuncion(c *Cerebro, k *tarea) nucleo.Respuesta {
	return crearFuncion(c, k).resp
}

// crearFuncion runs crear_funcion and returns the verified function (if any) with its answer.
func crearFuncion(c *Cerebro, k *tarea) crearHecho {
	cf := &crearFlujo{c: c, k: k}
	if r, ok := cf.entender(); !ok {
		return crearHecho{resp: r}
	}
	cf.otraForma = len(k.excluir) > 0 && len(k.ejemplosExtra) == 0 && k.seguimiento != nil && k.seguimiento.Tipo == "otra_forma"
	if !cf.f.Probable() {
		k.raiz.Sub(nucleo.PasoNota, "No sé probar funciones con estos tipos: %s", cf.f.Go()).Info("")
		return crearHecho{resp: c.enConstruccion(k, "funciones con estos tipos ("+cf.f.Go()+")")}
	}
	plan := "Plan: 1) mirar mi memoria 2) mis recetas 3) construir el programa 4) comprobarlo"
	if cf.otraForma {
		plan = "Plan: construir otro programa distinto y comprobarlo"
	}
	k.raiz.Sub(nucleo.PasoPlan, "%s", plan).Info("")

	if !cf.otraForma {
		if h, ok := cf.memoria(); ok {
			return h
		}
		if h, ok := cf.recetas(); ok {
			return h
		}
	}
	if cf.otraForma {
		if h, ok := cf.otraFormaIgual(); ok {
			return h
		}
	}
	if h, ok := cf.sintesis(); ok {
		return h
	}
	if h, ok := cf.web(); ok {
		return h
	}
	return cf.fallo()
}

// ---- 1. Entender ----

func (cf *crearFlujo) entender() (nucleo.Respuesta, bool) {
	c, k, q := cf.c, cf.k, cf.k.q
	var f *nucleo.Firma
	switch {
	case q.Firma != nil:
		g := *q.Firma
		f = &g
	default:
		f = lengua.FirmaProbable(q.Marco, q.ParesTexto)
	}
	if f == nil && (k.seguimiento != nil || k.previo != nil) {
		if ctx := k.s.contexto(); ctx.Firma != nil {
			g := *ctx.Firma
			f = &g
		}
	}
	if f == nil {
		if resp, ok := c.preguntar(k, "¿Qué recibe la función?", opcionesRecibe); ok {
			f = crearFirmaDeRespuesta(resp, q.Marco)
		}
	}
	if f == nil {
		k.raiz.Sub(nucleo.PasoEntender, "No sé qué recibe la función").Mal("")
		return nucleo.Respuesta{
			Texto: "No sé qué datos recibe la función.",
			Parrafos: []string{"Dímelo así: «recibe una lista de números», o dame un ejemplo de entrada y salida: " +
				"[1,2,3] -> 6."},
			Comprobaciones: []nucleo.Comprobacion{},
			Nivel:          nucleo.NivelSinComprobar,
			Metodo:         "sintesis",
		}, false
	}
	if f.Nombre == "" || f.Nombre == "Funcion" && q.Marco != nil && q.Marco.Accion != "" {
		if nombre := lengua.NombreFuncion(q.Marco); nombre != "" {
			f.Nombre = nombre
		}
	}
	if f.Nombre == "" {
		f.Nombre = "Funcion"
	}
	cf.f = *f

	ejemplos := q.Ejemplos
	if len(ejemplos) == 0 && len(q.ParesTexto) > 0 {
		if cs, _, err := lengua.TiparEjemplos(q.ParesTexto, &cf.f); err == nil {
			ejemplos = cs
		}
	}
	visto := map[string]bool{}
	añadir := func(cs nucleo.Caso, nota string) {
		if len(cs.Entradas) != len(cf.f.Entradas()) || len(cs.Esperado) != len(cf.f.Res) {
			return
		}
		clave := crearClaveEntradas(cs.Entradas)
		if visto[clave] {
			return
		}
		visto[clave] = true
		cs.Expectativa = nucleo.EspUsuario
		if cs.Origen == "" {
			cs.Origen = nucleo.OrigenUsuario
		}
		if cs.Nota == "" {
			cs.Nota = nota
		}
		cf.usuario = append(cf.usuario, cs)
	}
	for i, e := range ejemplos {
		añadir(e, fmt.Sprintf("tu ejemplo %d", i+1))
	}
	for _, e := range k.ejemplosExtra {
		añadir(e, "tu contraejemplo")
	}
	guardados := c.contraejemplos(q.Conceptos, cf.f)
	for _, e := range guardados {
		añadir(e, "contraejemplo guardado")
	}
	cf.props = pruebas.Propiedades(q.Conceptos, cf.f)
	cf.necesarios = crearNecesarios(q.Marco)

	n := k.raiz.Sub(nucleo.PasoEntender, "Entiendo: `%s`", cf.f.Go())
	switch {
	case len(cf.usuario) == 1:
		n.Detalle("Con 1 ejemplo tuyo")
	case len(cf.usuario) > 1:
		n.Detalle("Con %d ejemplos tuyos", len(cf.usuario))
	}
	if len(guardados) > 0 {
		n.Detalle("Añado %d contraejemplo(s) que me diste otras veces", len(guardados))
	}
	n.Info("")
	return nucleo.Respuesta{}, true
}

// crearFirmaDeRespuesta builds the signature from the answer to "¿Qué recibe la función?".
func crearFirmaDeRespuesta(resp string, m *nucleo.Marco) *nucleo.Firma {
	var params []nucleo.Param
	switch nucleo.Normalizar(resp) {
	case "una lista de numeros":
		params = []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}
	case "un texto":
		params = []nucleo.Param{{Nombre: "texto", Tipo: nucleo.TString}}
	case "un numero":
		params = []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}
	case "una lista de palabras":
		params = []nucleo.Param{{Nombre: "palabras", Tipo: nucleo.ListaDe(nucleo.TString)}}
	case "dos numeros":
		params = []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}
	default:
		return nil
	}
	res := nucleo.TInt
	salida := ""
	if m != nil {
		salida = m.Salida
	}
	switch salida {
	case "decimal":
		res = nucleo.TFloat
	case "booleano":
		res = nucleo.TBool
	case "texto":
		res = nucleo.TString
	case "lista":
		if params[0].Tipo.Clase == nucleo.CLista {
			res = params[0].Tipo
		} else {
			res = nucleo.ListaDe(nucleo.TInt)
		}
	case "mapa":
		res = nucleo.MapaDe(nucleo.TString, nucleo.TInt)
	}
	f := &nucleo.Firma{Params: params, Res: []nucleo.Tipo{res}}
	if m != nil {
		f.Nombre = lengua.NombreFuncion(m)
	}
	return f
}

// crearPredicados are the concepts that change what a function does when they are the frame's element.
var crearPredicados = map[string]bool{
	"par": true, "impar": true, "positivo": true, "negativo": true, "cero": true, "primo": true,
	"vocal": true, "consonante": true, "digito": true, "letra": true, "mayuscula": true, "minuscula": true,
	"espacio": true,
}

// crearNecesarios: the concepts a stored function or a recipe must carry to be the same request
// ("ahora con impares" must not reuse SumaPares).
func crearNecesarios(m *nucleo.Marco) []string {
	if m == nil {
		return nil
	}
	var out []string
	if m.Accion != "" {
		out = append(out, m.Accion)
	}
	if crearPredicados[m.Elemento] {
		out = append(out, m.Elemento)
	}
	for _, md := range m.Mods {
		if md.Concepto != "" && !md.Negado {
			out = append(out, md.Concepto)
		}
	}
	return out
}

// compatible: every needed concept is among cs (singular and plural count as the same).
func (cf *crearFlujo) compatible(cs []string) bool {
	tiene := map[string]bool{}
	for _, x := range cs {
		tiene[crearRaiz(x)] = true
	}
	for _, x := range cf.necesarios {
		if !tiene[crearRaiz(x)] {
			return false
		}
	}
	return true
}

func crearRaiz(c string) string { return strings.TrimSuffix(nucleo.Normalizar(c), "s") }

func crearClaveEntradas(vs []nucleo.Valor) string {
	partes := make([]string, len(vs))
	for i, v := range vs {
		partes[i] = nucleo.Clave(v)
	}
	return strings.Join(partes, "\x1f")
}

// ---- 2. Memoria ----

var reIdentificador = regexp.MustCompile(`\b[A-Z][A-Za-z0-9_]*\b`)

func (cf *crearFlujo) memoria() (crearHecho, bool) {
	c, k, q := cf.c, cf.k, cf.k.q
	type candidata struct {
		F      nucleo.FuncionAprendida
		puntos float64
		exacta bool
	}
	var cands []candidata
	visto := map[string]bool{}
	for _, w := range reIdentificador.FindAllString(q.Texto, -1) {
		if g, ok := c.mem.Funcion(w); ok && g.Firma.Forma() == cf.f.Forma() && !visto[g.Nombre] {
			visto[g.Nombre] = true
			cands = append(cands, candidata{F: g, puntos: 1, exacta: true})
		}
	}
	for _, h := range c.memoriaBuscar(k, k.raiz, q.Conceptos, lemasDe(q), cf.f.Forma(), 3) {
		if h.Puntos < crearUmbralMemoria || visto[h.F.Nombre] {
			continue
		}
		if !cf.compatible(h.F.Conceptos) {
			continue
		}
		visto[h.F.Nombre] = true
		cands = append(cands, candidata{F: h.F, puntos: h.Puntos})
	}
	for _, cd := range cands {
		F := cd.F
		if F.Codigo == "" || F.Firma.Forma() != cf.f.Forma() || crearExcluido(k.excluir, F.DSL, "memoria:"+F.Nombre) {
			continue
		}
		guardados, err := nucleo.LeerCasos(F.Firma, F.Casos)
		if err != nil {
			guardados = nil
		}
		for i := range guardados {
			if guardados[i].ConEsperado() {
				guardados[i].Expectativa = nucleo.EspReferencia
				guardados[i].Nota = "lo que guardé en mi memoria"
			}
		}
		casos := append(append([]nucleo.Caso(nil), cf.usuario...), guardados...)
		casos = append(casos, cf.sondas(F.Firma, nil)...)
		v, err := cf.comprobar("Compruebo "+F.Nombre+", que ya tengo en mi memoria", F.Codigo, F.Firma, casos, nil)
		if err != nil {
			if errors.Is(err, errSinArenero) {
				cf.sinArenero = true
				return crearHecho{}, false
			}
			continue
		}
		if len(v.Fallos) > 0 {
			cf.anotarIntento(F.Codigo, F.Firma, v)
			k.raiz.Sub(nucleo.PasoRecuerdo, "La que tenía en memoria (%s) no cumple: busco otra", F.Nombre).Mal("")
			continue
		}
		c.mem.Usada(F.Nombre)
		texto := fmt.Sprintf("Ya sé hacer esto (lo aprendí el %d/%d): `%s`.", F.Creada.Day(), int(F.Creada.Month()), F.Nombre)
		g := F
		k.turno.Funcion = &g
		var programa *sintesis.Expr
		if F.DSL != "" {
			if e, err := sintesis.Parse(F.DSL, c.registro(), F.Firma); err == nil {
				programa = e
			}
		}
		h := cf.respuesta("memoria", texto, F.Codigo, F.Firma, v, programa, F.Descripcion, nil)
		if programa != nil {
			k.programa = programa
			k.turno.Expr = programa.String()
		} else {
			k.turno.Excluidos = append(k.turno.Excluidos, "memoria:"+F.Nombre) // for "otra forma"
		}
		return h, true
	}
	return crearHecho{}, false
}

func crearExcluido(excluir []string, claves ...string) bool {
	for _, x := range excluir {
		for _, c := range claves {
			if c != "" && x == c {
				return true
			}
		}
	}
	return false
}

// ---- 3. Recetas ----

func (cf *crearFlujo) recetas() (crearHecho, bool) {
	k, q := cf.k, cf.k.q
	n := k.raiz.Sub(nucleo.PasoRecuerdo, "Miro mis recetas")
	hs := recetas.BuscarFunciones(crearLemas(q), &cf.f, 3)
	probadas := 0
	for _, h := range hs {
		if h.Receta == nil || h.Puntos < crearUmbralReceta {
			continue
		}
		rc := *h.Receta
		if crearExcluido(k.excluir, "receta:"+rc.Nombre) {
			continue
		}
		if !cf.compatible(rc.Conceptos) {
			n.Nota("%s se parece, pero no hace lo mismo", rc.Nombre)
			continue
		}
		if len(cf.usuario) == 0 && h.Puntos < crearUmbralRecetaSola {
			n.Nota("%s se parece (%s), pero sin ejemplos tuyos no me fío", rc.Nombre, decimal(h.Puntos))
			continue
		}
		nombre := cf.f.Nombre
		if nombre == "Funcion" {
			nombre = rc.Nombre
		}
		r := recetas.Renombrar(rc, nombre)
		probadas++
		nota := "la receta " + rc.Nombre
		var propias []nucleo.Caso
		for _, cs := range r.Casos {
			cs.Expectativa = nucleo.EspReferencia
			cs.Nota = nota
			propias = append(propias, cs)
		}
		casos := append(append([]nucleo.Caso(nil), cf.usuario...), propias...)
		casos = append(casos, cf.generados(r.Firma, nil, 32, 100)...)
		casos = append(casos, cf.sondas(r.Firma, nil)...)
		v, err := cf.comprobar("Compruebo la receta "+rc.Nombre+" ("+decimal(h.Puntos)+")", r.Codigo, r.Firma, casos, cf.props)
		if err != nil {
			if errors.Is(err, errSinArenero) {
				cf.sinArenero = true
				n.Info("No puedo comprobar recetas sin Go")
				return crearHecho{}, false
			}
			continue
		}
		if len(v.Fallos) > 0 {
			cf.anotarIntento(r.Codigo, r.Firma, v)
			cf.intentos = append(cf.intentos, "La receta "+rc.Nombre+" no cumple lo que pides.")
			continue
		}
		n.Bien("Uso la receta %s", rc.Nombre)
		texto := fmt.Sprintf("Aquí tienes `%s` (de mis recetas).", r.Firma.Nombre)
		hecho := cf.respuesta("receta", texto, r.Codigo, r.Firma, v, nil, r.Descripcion, nil)
		k.turno.Excluidos = append(k.turno.Excluidos, "receta:"+rc.Nombre) // for "otra forma"
		return hecho, true
	}
	if probadas == 0 {
		n.Info("No tengo una receta que encaje")
	} else {
		n.Mal("Ninguna receta cumple lo que pides")
	}
	return crearHecho{}, false
}

// ---- 4. Síntesis ----

func (cf *crearFlujo) sintesis() (crearHecho, bool) {
	c, k, q := cf.c, cf.k, cf.k.q
	if !sintesis.Sintetizable(cf.f) {
		k.raiz.Sub(nucleo.PasoNota, "No sé construir programas con estos tipos").Info("")
		return crearHecho{}, false
	}
	if len(cf.usuario) == 0 && q.Marco == nil {
		k.raiz.Sub(nucleo.PasoNota, "Sin ejemplos ni una descripción que entienda no puedo construirlo").Info("")
		return crearHecho{}, false
	}
	reg := c.registro()
	n := k.raiz.Sub(nucleo.PasoIntento, "Construyo el programa")
	lim := cf.limiteSintesis()
	ctx, cancelar := context.WithTimeout(k.ctx, lim+time.Second)
	defer cancelar()

	var sols []sintesis.Solucion
	if len(cf.usuario) > 0 {
		sols = cf.conEjemplos(ctx, n, reg, lim)
	} else {
		sols = cf.desdeConceptos(ctx, n, reg, lim)
	}
	if cf.otraForma {
		sols = cf.sinPrevia(sols)
	}
	if len(sols) == 0 {
		n.Mal("No encontré ningún programa que encaje")
		cf.intentos = append(cf.intentos, "No encontré ningún programa que encaje.")
		return crearHecho{}, false
	}
	if len(sols) == 1 {
		n.Bien("Encontré un programa: %s", sols[0].Expr.String())
	} else {
		n.Bien("Encontré %d programas; el mejor: %s", len(sols), sols[0].Expr.String())
	}
	for i, s := range sols {
		if i >= crearMaxCandidatos {
			break
		}
		fuente, err := sintesis.AGo(s.Expr, cf.f)
		if err != nil {
			k.raiz.Sub(nucleo.PasoError, "No supe escribir en Go %s: %s", s.Expr.String(), mensajeError(err)).Mal("")
			continue
		}
		desc := sintesis.Describir(s.Expr, cf.f)
		if cf.sinArenero || c.ej == nil {
			return cf.sinComprobar(fuente, s.Expr, desc), true
		}
		casos := append([]nucleo.Caso(nil), cf.usuario...)
		oraculo := sintesis.Oraculo(s.Expr)
		casos = append(casos, cf.generados(cf.f, oraculo, 64, 200)...)
		casos = append(casos, cf.sondas(cf.f, oraculo)...)
		v, err := cf.comprobar("Compruebo el código en Go", fuente, cf.f, casos, cf.props)
		if err != nil {
			if errors.Is(err, errSinArenero) {
				return cf.sinComprobar(fuente, s.Expr, desc), true
			}
			cf.intentos = append(cf.intentos, "No pude comprobar "+s.Expr.String()+": "+mensajeError(err)+".")
			continue
		}
		if len(v.Fallos) > 0 {
			k.raiz.Sub(nucleo.PasoError, "El código en Go no hace lo mismo que mi programa %s: no lo uso ni lo guardo", s.Expr.String()).Mal("")
			cf.anotarIntento(fuente, cf.f, v)
			cf.intentos = append(cf.intentos, "El código de "+s.Expr.String()+" falló al comprobarlo.")
			continue
		}
		texto := fmt.Sprintf("Aquí tienes `%s`: %s.", cf.f.Nombre, strings.TrimSuffix(desc, "."))
		if desc == "" {
			texto = fmt.Sprintf("Aquí tienes `%s`.", cf.f.Nombre)
		}
		return cf.respuesta("sintesis", texto, fuente, cf.f, v, s.Expr, desc, nil), true
	}
	return crearHecho{}, false
}

func (cf *crearFlujo) limiteSintesis() time.Duration {
	base := 6 * time.Second
	if cf.k.pensarMas {
		base = 12 * time.Second
	}
	lim := cf.k.presupuesto * 45 / 100
	if r := cf.k.restante(); r > 0 && lim > r*8/10 {
		lim = r * 8 / 10
	}
	if lim <= 0 || lim > base {
		lim = base
	}
	if lim < 500*time.Millisecond {
		lim = 500 * time.Millisecond
	}
	return lim
}

func (cf *crearFlujo) especificacion(reg *sintesis.Registro) sintesis.Especificacion {
	q := cf.k.q
	esp := sintesis.Especificacion{
		Firma:      cf.f,
		Ejemplos:   cf.usuario,
		Conceptos:  crearPesos(q.Marco),
		Constantes: crearConstantes(q, cf.f, len(cf.usuario) > 0),
		Excluir:    cf.k.excluir,
	}
	if q.Marco != nil {
		esp.Esqueletos = sintesis.Esqueletos(q.Marco, cf.f, reg)
	}
	if q.ConsultaWeb != "" {
		if p, ok := cf.c.mem.Pistas(q.ConsultaWeb); ok {
			esp.Pistas = &p
		}
	}
	return esp
}

// conEjemplos: Sintetizar, then up to 3 Distinguir questions; each answer becomes a user example.
func (cf *crearFlujo) conEjemplos(ctx context.Context, n *nucleo.Nodo, reg *sintesis.Registro, lim time.Duration) []sintesis.Solucion {
	c, k := cf.c, cf.k
	op := sintesis.Opciones{Limite: lim, Semilla: k.semilla, Priors: c.stats}
	var sols []sintesis.Solucion
	resintetizar := true
	for ronda := 0; ronda <= crearMaxRondas; ronda++ {
		if resintetizar {
			esp := cf.especificacion(reg)
			res, err := sintesis.Sintetizar(ctx, esp, reg, op, n)
			if err != nil && len(res.Soluciones) == 0 {
				n.Nota("No encontré un programa: %s", mensajeError(err))
			}
			sols = res.Soluciones
			resintetizar = false
		}
		cf.ordenar(sols, true)
		if len(sols) < 2 || len(cf.f.Res) != 1 {
			break
		}
		sondas := sintesis.SondasPorDefecto(cf.f, cf.usuario, 8)
		ent, sal, grupos, ok := sintesis.Distinguir(sols, sondas)
		if !ok {
			break
		}
		if ronda == crearMaxRondas {
			cf.anotarDuda(ent, sal, grupos)
			break
		}
		valor, ok := cf.preguntarSalida(ent, sal)
		if !ok {
			cf.anotarDuda(ent, sal, grupos)
			break
		}
		cf.usuario = append(cf.usuario, nucleo.Caso{
			Entradas: ent, Esperado: []nucleo.Valor{valor}, Expectativa: nucleo.EspUsuario,
			Origen: nucleo.OrigenUsuario, Nota: "tu respuesta",
		})
		var quedan []sintesis.Solucion
		for g, out := range sal {
			if nucleo.Igual(out, valor) {
				for _, i := range grupos[g] {
					quedan = append(quedan, sols[i])
				}
			}
		}
		if len(quedan) == 0 {
			resintetizar = true
			continue
		}
		sols = quedan
	}
	return sols
}

// desdeConceptos: without examples, programs from the Spanish frame; ask only when two close
// readings disagree.
func (cf *crearFlujo) desdeConceptos(ctx context.Context, n *nucleo.Nodo, reg *sintesis.Registro, lim time.Duration) []sintesis.Solucion {
	k := cf.k
	todas := sintesis.DesdeConceptos(ctx, k.q.Marco, cf.f, reg, 5+len(k.excluir))
	var sols []sintesis.Solucion
	for _, s := range todas {
		if s.Expr != nil && !crearExcluido(k.excluir, s.Expr.String()) {
			sols = append(sols, s)
		}
	}
	cf.ordenar(sols, false)
	if len(sols) < 2 || len(cf.f.Res) != 1 {
		return sols
	}
	cl0 := cf.clave(sols[0].Expr)
	var cerca []sintesis.Solucion
	for _, s := range sols {
		if d := s.Costo - sols[0].Costo; cf.clave(s.Expr) == cl0 && d <= crearCercaCosto && d >= -crearCercaCosto {
			cerca = append(cerca, s)
		}
	}
	if len(cerca) < 2 {
		return sols
	}
	sondas := sintesis.SondasPorDefecto(cf.f, nil, 8)
	ent, sal, grupos, ok := sintesis.Distinguir(cerca, sondas)
	if !ok {
		return sols
	}
	valor, ok := cf.preguntarSalida(ent, sal)
	if !ok {
		cf.anotarDuda(ent, sal, grupos)
		return sols
	}
	caso := nucleo.Caso{Entradas: ent, Esperado: []nucleo.Valor{valor}, Expectativa: nucleo.EspUsuario,
		Origen: nucleo.OrigenUsuario, Nota: "tu respuesta"}
	cf.usuario = append(cf.usuario, caso)
	var quedan []sintesis.Solucion
	for g, out := range sal {
		if nucleo.Igual(out, valor) {
			for _, i := range grupos[g] {
				quedan = append(quedan, cerca[i])
			}
		}
	}
	if len(quedan) > 0 {
		return quedan
	}
	return cf.conEjemplos(ctx, n, reg, lim)
}

// crearClave is how well a program fits the request: the frame's concepts it uses (action, modifiers,
// predicates), whether it uses the frame's element, and how many parameters it reads.
type crearClave struct{ nec, elem, par int }

func (a crearClave) mejor(b crearClave) bool {
	if a.nec != b.nec {
		return a.nec > b.nec
	}
	if a.elem != b.elem {
		return a.elem > b.elem
	}
	return a.par > b.par
}

func (cf *crearFlujo) clave(e *sintesis.Expr) crearClave {
	cl := crearClave{nec: crearCobertura(e, crearExpandir(cf.necesarios)), par: crearParametros(e, len(cf.f.Entradas()))}
	if m := cf.k.q.Marco; m != nil && m.Elemento != "" {
		cl.elem = crearCobertura(e, []string{m.Elemento})
	}
	return cl
}

// ordenar ranks programs by crearClave; ties keep the order given (DesdeConceptos ranks by the frame)
// or, with porCosto, go by cost.
func (cf *crearFlujo) ordenar(sols []sintesis.Solucion, porCosto bool) {
	claves := make(map[*sintesis.Expr]crearClave, len(sols))
	for _, s := range sols {
		claves[s.Expr] = cf.clave(s.Expr)
	}
	sort.SliceStable(sols, func(i, j int) bool {
		a, b := claves[sols[i].Expr], claves[sols[j].Expr]
		if a != b {
			return a.mejor(b)
		}
		return porCosto && sols[i].Costo < sols[j].Costo
	})
}

// crearCompuestos: concepts that a program expresses with two primitives ("más de 3 letras" is
// (> (largoS x) 3)).
var crearCompuestos = map[string][]string{
	"longitud_mayor": {"longitud", "mayor_que"},
	"longitud_menor": {"longitud", "menor_que"},
	"ordenar_desc":   {"ordenar"},
	"suma_digitos":   {"digitos", "sumar"},
}

func crearExpandir(cs []string) []string {
	var out []string
	for _, c := range cs {
		if x, ok := crearCompuestos[c]; ok {
			out = append(out, x...)
		} else {
			out = append(out, c)
		}
	}
	return out
}

// crearParametros counts the distinct parameters a program uses (Suma(a, b) = a + a is a poor reading).
func crearParametros(e *sintesis.Expr, n int) int {
	usados := map[int]bool{}
	var andar func(x *sintesis.Expr)
	andar = func(x *sintesis.Expr) {
		if x == nil {
			return
		}
		if x.Op == nil && !x.EsConst && !x.Hueco && x.Var >= 0 && x.Var < n {
			usados[x.Var] = true
		}
		for _, h := range x.Hijos {
			andar(h)
		}
		for _, l := range x.Lambdas {
			if l != nil {
				andar(l.Cuerpo)
			}
		}
	}
	andar(e)
	return len(usados)
}

// crearCobertura counts the concepts of quiero that the program's primitives carry. A learned or
// invented primitive counts through its body (its own tags come from the request that taught it).
func crearCobertura(e *sintesis.Expr, quiero []string) int {
	if e == nil || len(quiero) == 0 {
		return 0
	}
	tiene := map[string]bool{}
	var andar func(x *sintesis.Expr, prof int)
	andar = func(x *sintesis.Expr, prof int) {
		if x == nil || prof > 40 {
			return
		}
		if x.Op != nil {
			if x.Op.Cuerpo != nil {
				andar(x.Op.Cuerpo, prof+1)
			} else {
				for _, c := range x.Op.Conceptos {
					tiene[crearRaiz(c)] = true
				}
			}
		}
		for _, h := range x.Hijos {
			andar(h, prof+1)
		}
		for _, l := range x.Lambdas {
			if l != nil {
				andar(l.Cuerpo, prof+1)
			}
		}
	}
	andar(e, 0)
	n := 0
	visto := map[string]bool{}
	for _, c := range quiero {
		r := crearRaiz(c)
		if tiene[r] && !visto[r] {
			visto[r] = true
			n++
		}
	}
	return n
}

// preguntarSalida asks "¿Qué debe dar F(x)?" with the outputs of the groups and "Otra cosa".
func (cf *crearFlujo) preguntarSalida(ent []nucleo.Valor, sal []nucleo.Valor) (nucleo.Valor, bool) {
	var ops []string
	visto := map[string]bool{}
	for _, v := range sal {
		if _, malo := v.(nucleo.ErrorV); malo {
			continue
		}
		s := nucleo.FormatoHumano(v, cf.f.Res[0])
		if !visto[s] {
			visto[s] = true
			ops = append(ops, s)
		}
	}
	ops = append(ops, "Otra cosa")
	texto := fmt.Sprintf("¿Qué debe dar %s?", crearLlamada(cf.f, ent))
	v, _, ok := cf.c.preguntarValor(cf.k, texto, ops, cf.f.Res[0])
	return v, ok
}

// anotarDuda keeps "Había otra forma de entenderlo: con X daría Y" for the answer (the best program is
// solution 0 of the slice Distinguir saw).
func (cf *crearFlujo) anotarDuda(ent []nucleo.Valor, sal []nucleo.Valor, grupos [][]int) {
	if len(cf.f.Res) != 1 {
		return
	}
	for g, gs := range grupos {
		esMejor := false
		for _, i := range gs {
			if i == 0 {
				esMejor = true
			}
		}
		if esMejor || g >= len(sal) {
			continue
		}
		if _, malo := sal[g].(nucleo.ErrorV); malo {
			continue
		}
		cf.dudas = append(cf.dudas, fmt.Sprintf("Había otra forma de entenderlo: con %s daría %s.",
			crearArgs(cf.f, ent), nucleo.FormatoHumano(sal[g], cf.f.Res[0])))
		return
	}
}

// crearPesos: concept weights from the frame (action 1.0, modifiers and predicates 0.8).
func crearPesos(m *nucleo.Marco) map[string]float64 {
	if m == nil {
		return nil
	}
	out := map[string]float64{}
	if m.Accion != "" {
		out[m.Accion] = 1
	}
	if crearPredicados[m.Elemento] {
		out[m.Elemento] = 0.8
	}
	for _, md := range m.Mods {
		if md.Concepto != "" {
			if _, ok := out[md.Concepto]; !ok {
				out[md.Concepto] = 0.8
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var reCitado = regexp.MustCompile(`«([^»]+)»|"([^"]+)"|“([^”]+)”`)

// crearConstantes: the frame's numbers and texts; with no examples also the request's numbers and
// quoted texts (with examples those would only fit the examples by chance).
func crearConstantes(q *nucleo.Pregunta, f nucleo.Firma, conEjemplos bool) []nucleo.Valor {
	usaFloat := false
	for _, t := range append(f.Entradas(), f.Res...) {
		if t.Clase == nucleo.CFloat || t.Elem != nil && t.Elem.Clase == nucleo.CFloat {
			usaFloat = true
		}
	}
	var out []nucleo.Valor
	visto := map[string]bool{}
	poner := func(v nucleo.Valor) {
		k := nucleo.Clave(v)
		if !visto[k] {
			visto[k] = true
			out = append(out, v)
		}
	}
	numero := func(x float64) {
		if x == float64(int64(x)) && x > -1e9 && x < 1e9 {
			poner(int(x))
		}
		if usaFloat {
			poner(x)
		}
	}
	if q.Marco != nil {
		for _, md := range q.Marco.Mods {
			for _, x := range md.Numeros {
				numero(x)
			}
			if md.Texto != "" {
				poner(md.Texto)
			}
		}
	}
	if !conEjemplos {
		for _, nm := range q.Numeros {
			if nm.Valor == nil {
				continue
			}
			x, _ := nm.Valor.Float64()
			numero(x)
		}
		texto := q.Resto
		if texto == "" {
			texto = q.Texto
		}
		for _, m := range reCitado.FindAllStringSubmatch(texto, -1) {
			for _, g := range m[1:] {
				if g != "" {
					poner(g)
				}
			}
		}
	}
	return out
}

// ---- otra forma ----

// previa is the name of the function shown in the previous turn (a learned primitive by now).
func (cf *crearFlujo) previa() string {
	if cf.k.previo != nil && cf.k.previo.Funcion != nil {
		return cf.k.previo.Funcion.Nombre
	}
	if cf.k.s != nil {
		return cf.k.s.contexto().Funcion
	}
	return ""
}

// sinPrevia drops programs that just call the function already shown (it was learned as a primitive).
func (cf *crearFlujo) sinPrevia(sols []sintesis.Solucion) []sintesis.Solucion {
	nombres := []string{cf.f.Nombre, cf.previa()}
	var out []sintesis.Solucion
	for _, s := range sols {
		if !crearUsaPrimitiva(s.Expr, nombres) {
			out = append(out, s)
		}
	}
	return out
}

// crearUsaPrimitiva: the program calls a learned function named like one of nombres (or a numbered copy
// of it: SumaPares2 is the memory key of a second SumaPares).
func crearUsaPrimitiva(e *sintesis.Expr, nombres []string) bool {
	if e == nil {
		return false
	}
	if e.Op != nil && e.Op.Cuerpo != nil {
		for _, n := range nombres {
			if n != "" && strings.HasPrefix(e.Op.Nombre, n) && strings.Trim(strings.TrimPrefix(e.Op.Nombre, n), "0123456789") == "" {
				return true
			}
		}
	}
	for _, h := range e.Hijos {
		if crearUsaPrimitiva(h, nombres) {
			return true
		}
	}
	for _, l := range e.Lambdas {
		if l != nil && crearUsaPrimitiva(l.Cuerpo, nombres) {
			return true
		}
	}
	return false
}

// otraFormaIgual: "otra forma" first looks for a different program that does the same as the one
// already shown (its outputs are the spec, Excluir drops the shown programs); without one, synthesis
// looks for another reading.
func (cf *crearFlujo) otraFormaIgual() (crearHecho, bool) {
	c, k := cf.c, cf.k
	if !sintesis.Sintetizable(cf.f) || len(cf.f.Res) != 1 || c.ej == nil {
		return crearHecho{}, false
	}
	reg := c.registro()
	var ref []nucleo.Caso
	var oraculoPrevio func([]nucleo.Valor) ([]nucleo.Valor, error)
	antes := "la versión anterior"
	for i := len(k.excluir) - 1; i >= 0 && len(ref) == 0; i-- {
		x := k.excluir[i]
		switch {
		case strings.HasPrefix(x, "receta:"):
			for _, rc := range recetas.Funciones() {
				if rc.Nombre == strings.TrimPrefix(x, "receta:") && rc.Firma.Forma() == cf.f.Forma() {
					ref = rc.Casos
				}
			}
		case strings.HasPrefix(x, "memoria:"):
			if g, ok := c.mem.Funcion(strings.TrimPrefix(x, "memoria:")); ok && g.Firma.Forma() == cf.f.Forma() {
				ref, _ = nucleo.LeerCasos(g.Firma, g.Casos)
			}
		default:
			e, err := sintesis.Parse(x, reg, cf.f)
			if err != nil {
				continue
			}
			o := sintesis.Oraculo(e)
			oraculoPrevio = o
			for _, sd := range sintesis.SondasPorDefecto(cf.f, nil, 12) {
				if out, err := o(sd); err == nil && out != nil {
					ref = append(ref, nucleo.Caso{Entradas: sd, Esperado: out, Origen: nucleo.OrigenSonda})
				}
			}
		}
	}
	var refs []nucleo.Caso
	for _, x := range ref {
		if len(x.Esperado) == len(cf.f.Res) && len(x.Entradas) == len(cf.f.Entradas()) {
			x.Expectativa, x.Nota = nucleo.EspReferencia, antes
			refs = append(refs, x)
		}
	}
	if len(refs) < 3 {
		return crearHecho{}, false
	}
	n := k.raiz.Sub(nucleo.PasoIntento, "Busco otro programa que haga lo mismo que %s", antes)
	lim := cf.limiteSintesis()
	ctx, cancelar := context.WithTimeout(k.ctx, lim+time.Second)
	defer cancelar()
	esp := cf.especificacion(reg)
	esp.Ejemplos = append(append([]nucleo.Caso(nil), cf.usuario...), refs...)
	res, _ := sintesis.Sintetizar(ctx, esp, reg, sintesis.Opciones{Limite: lim, Semilla: k.semilla, Priors: c.stats}, n)
	sols := cf.sinPrevia(res.Soluciones)
	if len(sols) == 0 {
		n.Info("No encontré otra forma de hacer lo mismo")
		return crearHecho{}, false
	}
	n.Bien("Encontré otra: %s", sols[0].Expr.String())
	for i, s := range sols {
		if i >= crearMaxCandidatos {
			break
		}
		fuente, err := sintesis.AGo(s.Expr, cf.f)
		if err != nil {
			continue
		}
		oraculo := sintesis.Oraculo(s.Expr)
		casos := append(append([]nucleo.Caso(nil), cf.usuario...), refs...)
		if oraculoPrevio != nil {
			// "the same as before" is checked against the previous program on every generated input
			gen := cf.c.generarCasos(k, cf.f, cf.usuario, 64, 200)
			casos = append(casos, pruebas.ConOraculo(gen, oraculoPrevio, nucleo.EspReferencia, antes)...)
		} else {
			casos = append(casos, cf.generados(cf.f, oraculo, 64, 200)...)
		}
		casos = append(casos, cf.sondas(cf.f, oraculo)...)
		v, err := cf.comprobar("Compruebo el código en Go", fuente, cf.f, casos, cf.props)
		if err != nil || len(v.Fallos) > 0 {
			if err == nil {
				cf.anotarIntento(fuente, cf.f, v)
			}
			continue
		}
		desc := sintesis.Describir(s.Expr, cf.f)
		texto := fmt.Sprintf("Otra forma de escribir `%s`: %s.", cf.f.Nombre, strings.TrimSuffix(desc, "."))
		return cf.respuesta("sintesis", texto, fuente, cf.f, v, s.Expr, desc, nil), true
	}
	return crearHecho{}, false
}

// ---- 6. Web ----

func (cf *crearFlujo) web() (crearHecho, bool) {
	c, k, q := cf.c, cf.k, cf.k.q
	if c.web == nil || c.ej == nil || strings.TrimSpace(q.ConsultaWeb) == "" || cf.sinArenero {
		return crearHecho{}, false
	}
	if !c.enLinea(k.ctx) {
		return crearHecho{}, false
	}
	presupuesto := k.presupuesto * 35 / 100
	if presupuesto <= 0 || presupuesto > k.restante() {
		presupuesto = k.restante()
	}
	if presupuesto < time.Second {
		return crearHecho{}, false
	}
	ctx, cancelar := context.WithTimeout(k.ctx, presupuesto)
	defer cancelar()
	n := k.raiz.Sub(nucleo.PasoBusqueda, "Busco en la web: «%s»", q.ConsultaWeb)
	_, docs, err := c.web.BuscarDocumentos(ctx, q.ConsultaWeb)
	if err != nil || len(docs) == 0 {
		if err != nil {
			n.Mal("No encontré nada útil: %s", mensajeError(err))
		} else {
			n.Mal("No encontré nada útil")
		}
		return crearHecho{}, false
	}
	type variante struct {
		fuente, url, titulo string
		pasos               []string
		votos               int
	}
	var vs []variante
	var codigos []string
	for _, d := range docs {
		for _, cand := range saber.Candidatas(d) {
			if cand.EsPrograma {
				continue
			}
			codigos = append(codigos, cand.Codigo)
			for _, a := range saber.Adaptar(cand, cf.f) {
				if len(vs) >= crearMaxVariantesWeb {
					break
				}
				if viol := c.ej.Revisar(a.Fuente, nucleo.PermisoAuto); len(viol) > 0 {
					n.Nota("No lo ejecuto: %s (%s)", viol[0].Que, saber.Dominio(cand.URL))
					continue
				}
				vs = append(vs, variante{fuente: a.Fuente, url: cand.URL, titulo: d.Titulo, pasos: a.Pasos, votos: cand.Votos})
			}
		}
	}
	if len(codigos) > 0 {
		if p := saber.Minar(codigos); len(p.Llamadas)+len(p.Operadores)+len(p.Constantes) > 0 {
			_ = c.mem.GuardarPistas(q.ConsultaWeb, p)
		}
	}
	if len(vs) == 0 {
		n.Info("No encontré código que pueda usar")
		return crearHecho{}, false
	}
	n.Info("Encontré %d versiones para probar", len(vs))
	fuentes := make([]string, len(vs))
	for i, v := range vs {
		fuentes[i] = v.fuente
	}
	gen := cf.generados(cf.f, nil, 32, 100)
	casos := append(append([]nucleo.Caso(nil), cf.usuario...), gen...)
	b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: fuentes, Firma: cf.f, Props: cf.props, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	if err != nil {
		return crearHecho{}, false
	}
	defer b.Cerrar()
	rs, err := c.probarCasos(k, n, b, casos, nucleo.OpcionesProbar{})
	if err != nil || len(rs) == 0 {
		return crearHecho{}, false
	}
	nu := len(cf.usuario)
	type nota struct {
		i, usuario, acuerdo, lineas int
		limpio                      bool
	}
	var notas []nota
	for i := range vs {
		if i >= len(rs) {
			break
		}
		nt := nota{i: i, limpio: true, lineas: strings.Count(vs[i].fuente, "\n")}
		for j, r := range rs[i] {
			if j < nu {
				if r.OK {
					nt.usuario++
				}
			} else if !crearTerminado(r) || r.PropFallida != "" {
				nt.limpio = false
			}
		}
		notas = append(notas, nt)
	}
	for a := range notas {
		for b2 := range notas {
			if a == b2 || vs[notas[a].i].url == vs[notas[b2].i].url {
				continue
			}
			if crearMismasSalidas(rs[notas[a].i][nu:], rs[notas[b2].i][nu:]) {
				notas[a].acuerdo++
			}
		}
	}
	sort.SliceStable(notas, func(a, b int) bool {
		x, y := notas[a], notas[b]
		if x.usuario != y.usuario {
			return x.usuario > y.usuario
		}
		if x.limpio != y.limpio {
			return x.limpio
		}
		if x.acuerdo != y.acuerdo {
			return x.acuerdo > y.acuerdo
		}
		if vs[x.i].votos != vs[y.i].votos {
			return vs[x.i].votos > vs[y.i].votos
		}
		return x.lineas < y.lineas
	})
	mejor := notas[0]
	if mejor.usuario < nu || !mejor.limpio {
		v := verificacion{Casos: casos, Resultados: rs[mejor.i]}
		v.Comprobaciones = pruebas.Comprobaciones(casos, rs[mejor.i], cf.props, k.semilla)
		v.Tabla = pruebas.Tabla(cf.f, casos, rs[mejor.i], 0)
		for j, r := range rs[mejor.i] {
			if !r.OK {
				v.Fallos = append(v.Fallos, j)
			}
		}
		cf.anotarIntento(vs[mejor.i].fuente, cf.f, v)
		n.Mal("Ninguna versión de la web cumple lo que pides")
		return crearHecho{}, false
	}
	elegida := vs[mejor.i]
	resultados := rs[mejor.i]
	casosV := append([]nucleo.Caso(nil), casos...)
	if mejor.acuerdo > 0 {
		nota := fmt.Sprintf("otras %d fuentes", mejor.acuerdo)
		if mejor.acuerdo == 1 {
			nota = "otra fuente"
		}
		for j := nu; j < len(casosV); j++ {
			if crearTerminado(resultados[j]) && resultados[j].Obtenido != nil {
				casosV[j].Esperado = resultados[j].Obtenido
				casosV[j].Expectativa = nucleo.EspReferencia
				casosV[j].Nota = nota
			}
		}
	}
	// Re-synthesize from its outputs so the stored function is a program I understand.
	if nu == 0 && sintesis.Sintetizable(cf.f) && k.restante() > 4*time.Second {
		var ejemplos []nucleo.Caso
		for j := nu; j < len(casos) && len(ejemplos) < 10; j++ {
			if crearTerminado(resultados[j]) && resultados[j].Obtenido != nil {
				ejemplos = append(ejemplos, nucleo.Caso{Entradas: casos[j].Entradas, Esperado: resultados[j].Obtenido,
					Expectativa: nucleo.EspReferencia, Origen: nucleo.OrigenWeb})
			}
		}
		if len(ejemplos) >= 3 {
			ctx3, c3 := context.WithTimeout(k.ctx, 3*time.Second)
			esp := cf.especificacion(c.registro())
			esp.Ejemplos = ejemplos
			res, _ := sintesis.Sintetizar(ctx3, esp, c.registro(), sintesis.Opciones{Limite: 3 * time.Second, Priors: c.stats}, n)
			c3()
			if len(res.Soluciones) > 0 {
				e := res.Soluciones[0].Expr
				if fuente, err := sintesis.AGo(e, cf.f); err == nil {
					cs := append([]nucleo.Caso(nil), casosV...)
					cs = append(cs, cf.sondas(cf.f, sintesis.Oraculo(e))...)
					if v, err := cf.comprobar("Compruebo el programa que saqué de la web", fuente, cf.f, cs, cf.props); err == nil && len(v.Fallos) == 0 {
						desc := sintesis.Describir(e, cf.f)
						texto := fmt.Sprintf("Aquí tienes `%s`: %s.", cf.f.Nombre, strings.TrimSuffix(desc, "."))
						fs := []nucleo.Fuente{{URL: elegida.url, Titulo: elegida.titulo, Nota: "lo comparé con este código"}}
						return cf.respuesta("web", texto, fuente, cf.f, v, e, desc, fs), true
					}
				}
			}
		}
	}
	v := verificacion{Casos: casosV, Resultados: resultados}
	v.Comprobaciones = pruebas.Comprobaciones(casosV, resultados, cf.props, k.semilla)
	v.Nivel = pruebas.NivelGlobal(v.Comprobaciones)
	v.Tabla = pruebas.Tabla(cf.f, casosV, resultados, 0)
	n.Bien("Uso el código de %s", saber.Dominio(elegida.url))
	texto := fmt.Sprintf("Aquí tienes `%s`, adaptado de código de %s.", cf.f.Nombre, saber.Dominio(elegida.url))
	fs := []nucleo.Fuente{{URL: elegida.url, Titulo: elegida.titulo, Nota: "fragmento de la web (licencia del sitio)"}}
	h := cf.respuesta("web", texto, elegida.fuente, cf.f, v, nil, "", fs)
	if len(elegida.pasos) > 0 {
		h.resp.Parrafos = append(h.resp.Parrafos, "Lo adapté así: "+strings.Join(elegida.pasos, "; ")+".")
	}
	return h, true
}

func crearTerminado(r nucleo.ResultadoCaso) bool {
	return r.Ejecutado && r.Panico == "" && !r.Caida && !r.Agotado && !r.SinCombustible && !r.NoDeterminista
}

func crearMismasSalidas(a, b []nucleo.ResultadoCaso) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	for i := range a {
		if !crearTerminado(a[i]) || !crearTerminado(b[i]) || len(a[i].Obtenido) != len(b[i].Obtenido) {
			return false
		}
		for j := range a[i].Obtenido {
			if !nucleo.Igual(a[i].Obtenido[j], b[i].Obtenido[j]) {
				return false
			}
		}
	}
	return true
}

// ---- 5. Codegen y verificación (shared) ----

// generados: seeded boundary and random cases; with an oracle they get "lo que entendí" expectations.
func (cf *crearFlujo) generados(f nucleo.Firma, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error), bordes, azar int) []nucleo.Caso {
	cs := cf.c.generarCasos(cf.k, f, cf.usuario, bordes, azar)
	if oraculo != nil {
		cs = pruebas.ConOraculo(cs, oraculo, nucleo.EspInterpretacion, "lo que entendí")
	}
	return cs
}

// sondas: the fixed probe inputs (they give the Huella and the "Así lo entendí" sentence).
func (cf *crearFlujo) sondas(f nucleo.Firma, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error)) []nucleo.Caso {
	cs := nucleo.Sondas(f)
	if oraculo != nil {
		cs = pruebas.ConOraculo(cs, oraculo, nucleo.EspInterpretacion, "lo que entendí")
	}
	return cs
}

// comprobar builds fuente once (PermisoAuto, InstrNormal, properties), runs the first 20 cases twice
// (determinism) and the rest once, runs go vet, and computes the comprobaciones.
func (cf *crearFlujo) comprobar(titulo, fuente string, f nucleo.Firma, casos []nucleo.Caso, props []nucleo.Propiedad) (verificacion, error) {
	c, k := cf.c, cf.k
	var v verificacion
	n := k.raiz.Sub(nucleo.PasoComprobar, "%s", titulo)
	b, comp, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Props: props, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	v.Compilacion = comp
	if err != nil {
		n.Mal("No lo pude comprobar")
		return v, err
	}
	defer b.Cerrar()
	primeros := min(crearRepetidos, len(casos))
	var res []nucleo.ResultadoCaso
	if primeros > 0 {
		rs, err := c.probarCasos(k, n, b, casos[:primeros], nucleo.OpcionesProbar{Repetir: 2})
		if err != nil || len(rs) == 0 {
			n.Mal("No lo pude comprobar")
			if err == nil {
				err = errors.New("el arenero no devolvió resultados")
			}
			return v, err
		}
		res = append(res, rs[0]...)
	}
	if primeros < len(casos) {
		rs, err := c.probarCasos(k, n, b, casos[primeros:], nucleo.OpcionesProbar{})
		if err != nil || len(rs) == 0 {
			n.Mal("No lo pude comprobar")
			if err == nil {
				err = errors.New("el arenero no devolvió resultados")
			}
			return v, err
		}
		for _, r := range rs[0] {
			r.Caso += primeros
			res = append(res, r)
		}
	}
	vet := c.ej.Vet(k.ctx, fuente)
	if vet.OK {
		n.Sub(nucleo.PasoComprobar, "Compila y go vet no se queja").Bien("")
	} else if len(vet.Errores) > 0 {
		n.Sub(nucleo.PasoComprobar, "go vet avisa: %s", resumenErrores(vet, nil)).Mal("")
		cf.intentos = append(cf.intentos, "go vet avisa: "+resumenErrores(vet, nil)+".")
	}
	v.Casos, v.Resultados = casos, res
	v.Comprobaciones = pruebas.Comprobaciones(casos, res, props, k.semilla)
	v.Nivel = pruebas.NivelGlobal(v.Comprobaciones)
	v.Tabla = pruebas.Tabla(f, casos, res, 0)
	for i, r := range res {
		if !r.OK {
			v.Fallos = append(v.Fallos, i)
		}
	}
	for _, cp := range v.Comprobaciones {
		x := n.Sub(nucleo.PasoComprobar, "%s", cp.Texto)
		if cp.Nivel == nucleo.NivelFallo {
			x.Mal("")
		} else {
			x.Bien("")
		}
	}
	if len(v.Fallos) == 0 {
		n.Bien("")
	} else {
		n.Mal("Fallan %d de %d casos", len(v.Fallos), len(casos))
	}
	return v, nil
}

// anotarIntento keeps the attempt that passed most user examples, with its first failing case.
func (cf *crearFlujo) anotarIntento(codigo string, f nucleo.Firma, v verificacion) {
	pasan, total := 0, 0
	contra := ""
	for i, cs := range v.Casos {
		if i >= len(v.Resultados) {
			break
		}
		r := v.Resultados[i]
		if cs.Expectativa == nucleo.EspUsuario {
			total++
			if r.OK {
				pasan++
			}
		}
		if !r.OK && contra == "" {
			contra = crearFrasefallo(f, cs, r)
		}
	}
	if cf.mejor.codigo != "" && pasan <= cf.mejor.pasan {
		return
	}
	tb := v.Tabla
	cf.mejor = crearIntento{codigo: codigo, pasan: pasan, total: total, contra: contra, tabla: &tb}
}

// crearFrasefallo: "Falla con [-1]: esperaba 0, da -1".
func crearFrasefallo(f nucleo.Firma, cs nucleo.Caso, r nucleo.ResultadoCaso) string {
	ent := crearArgs(f, cs.Entradas)
	switch {
	case r.Panico != "":
		return fmt.Sprintf("Falla con %s: se cae (%s).", ent, recortar(primeraLinea(r.Panico), 80))
	case r.Agotado || r.SinCombustible:
		return fmt.Sprintf("Falla con %s: no termina a tiempo.", ent)
	case r.PropFallida != "":
		return fmt.Sprintf("Falla con %s: no cumple «%s».", ent, r.PropFallida)
	case cs.ConEsperado() && len(f.Res) > 0:
		return fmt.Sprintf("Falla con %s: esperaba %s, da %s.", ent, crearValores(f.Res, cs.Esperado), crearValores(f.Res, r.Obtenido))
	}
	return fmt.Sprintf("Falla con %s.", ent)
}

// ---- answers ----

// respuesta builds the success answer, stores the function when the evidence allows it and fills the turn.
func (cf *crearFlujo) respuesta(origen, texto, fuente string, f nucleo.Firma, v verificacion, programa *sintesis.Expr, desc string, fuentes []nucleo.Fuente) crearHecho {
	c, k := cf.c, cf.k
	r := nucleo.Respuesta{
		Texto:          texto,
		Codigo:         fuente,
		Comprobaciones: v.Comprobaciones,
		Nivel:          v.Nivel,
		Metodo:         origen,
		Exito:          true,
		Fuentes:        fuentes,
	}
	if len(v.Tabla.Filas) > 0 {
		tb := v.Tabla
		if tb.Titulo == "" {
			tb.Titulo = "Pruebas"
		}
		r.Tablas = []nucleo.Tabla{tb}
	}
	var mejorComp *nucleo.Comprobacion
	for i := range v.Comprobaciones {
		if v.Comprobaciones[i].Nivel != nucleo.NivelFallo && v.Comprobaciones[i].Nivel != nucleo.NivelSinComprobar {
			mejorComp = &v.Comprobaciones[i]
			break
		}
	}
	if desc != "" && origen != "sintesis" {
		r.Parrafos = append(r.Parrafos, afirmar("Lo que hace: "+strings.TrimSuffix(desc, ".")+".", mejorComp))
	}
	claves := crearClaves(fuente, f.Nombre)
	r.Parrafos = append(r.Parrafos, claves...)
	r.Parrafos = append(r.Parrafos, cf.dudas...)
	if len(cf.usuario) == 0 {
		if e := crearEntendi(f, v.Casos, v.Resultados); e != "" {
			r.Entendi = e
		}
	}

	// Justificación for "¿por qué?": what was run and what the program means.
	just := []string{"Lo comprobé así:"}
	for _, cp := range v.Comprobaciones {
		linea := "· " + cp.Texto
		if cp.Detalle != "" {
			linea += " (" + cp.Detalle + ")"
		}
		just = append(just, linea)
	}
	if programa != nil {
		just = append(just, "Mi programa: "+programa.String())
	}
	if desc != "" {
		just = append(just, "Lo que hace: "+strings.TrimSuffix(desc, ".")+".")
	}
	just = append(just, claves...)
	if v.Compilacion.Comando != "" {
		just = append(just, "Comando: "+v.Compilacion.Comando)
	}
	r.Justificacion = just

	// Store (≥ entendido; web code also with sin_fallos).
	huella := crearHuella(f, v.Casos, v.Resultados)
	fa := nucleo.FuncionAprendida{
		Nombre: f.Nombre, Firma: f, Codigo: fuente, Descripcion: strings.TrimSuffix(strings.TrimSpace(desc), "."),
		Huella: huella, Nivel: v.Nivel, Origen: origen,
	}
	if desc == "" {
		fa.Descripcion = k.q.Texto
	}
	if len(fuentes) > 0 {
		fa.Fuente = fuentes[0].URL
	}
	if gs, err := nucleo.GuardarCasos(f, crearCasosGuardar(v)); err == nil {
		fa.Casos = gs
	}
	if programa != nil {
		fa.DSL = programa.String()
	}
	if origen != "memoria" {
		frase, err := c.guardarFuncion(k, fa, programa, origen == "web")
		if err == nil {
			r.Aprendido = frase
		} else {
			k.raiz.Sub(nucleo.PasoMemoria, "No la guardo en mi memoria: %s", mensajeError(err)).Info("")
			g := fa
			k.turno.Funcion = &g
			if programa != nil {
				k.turno.Expr = programa.String()
				if nivelVerificado(v.Nivel) {
					k.programa = programa
				}
			}
		}
	}
	k.turno.Metodo = origen
	return crearHecho{ok: true, codigo: fuente, firma: f, casos: crearCasosBuenos(v), resp: r}
}

// sinComprobar: there is no sandbox; the program satisfies the examples in my interpreter, but the Go
// code was never run, so it is shown as unverified and not stored.
func (cf *crearFlujo) sinComprobar(fuente string, e *sintesis.Expr, desc string) crearHecho {
	cf.k.raiz.Sub(nucleo.PasoNota, "No tengo Go para comprobar el código").Info("")
	r := nucleo.Respuesta{
		Texto:  afirmar(fmt.Sprintf("%s hace esto: %s.", "`"+cf.f.Nombre+"`", strings.TrimSuffix(desc, ".")), nil),
		Codigo: fuente,
		Parrafos: []string{"No lo he podido comprobar: no encuentro Go instalado, así que no lo he ejecutado ni lo guardo.",
			"Mi programa: " + e.String()},
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar: no tengo Go"}},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "sintesis",
		Exito:          false,
	}
	return crearHecho{sinComprobar: true, codigo: fuente, firma: cf.f, resp: r}
}

// fallo is the honest failure: the closest attempt, its failing case and the invitation to give examples.
func (cf *crearFlujo) fallo() crearHecho {
	c, k := cf.c, cf.k
	motivo := "no encontré un programa que haga lo que pides"
	switch {
	case cf.sinArenero:
		motivo = "no tengo Go para comprobar nada"
	case len(cf.usuario) == 0 && k.q.Marco == nil:
		motivo = "no entendí bien qué debe hacer la función"
	}
	r := c.respuestaFallo(k, motivo, cf.mejor.codigo, cf.mejor.contra)
	r.Metodo = "sintesis"
	if cf.mejor.tabla != nil && len(cf.mejor.tabla.Filas) > 0 {
		r.Tablas = []nucleo.Tabla{*cf.mejor.tabla}
	}
	r.Justificacion = append([]string{"Lo que intenté:"}, cf.intentos...)
	k.turno.Metodo = r.Metodo
	return crearHecho{resp: r}
}

// crearCasosGuardar: at most 40 cases with an expectation that passed, the user's first; when there are
// few, results of the run are kept as reference cases ("lo que hacía al guardarla").
func crearCasosGuardar(v verificacion) []nucleo.Caso {
	var usuario, otros, extra []nucleo.Caso
	for i, cs := range v.Casos {
		if i >= len(v.Resultados) || !v.Resultados[i].OK {
			continue
		}
		switch {
		case cs.ConEsperado() && cs.Expectativa == nucleo.EspUsuario:
			usuario = append(usuario, cs)
		case cs.ConEsperado():
			otros = append(otros, cs)
		case v.Resultados[i].Obtenido != nil && crearTerminado(v.Resultados[i]):
			x := cs
			x.Esperado = v.Resultados[i].Obtenido
			x.Expectativa = nucleo.EspReferencia
			x.Nota = "lo que hacía al guardarla"
			extra = append(extra, x)
		}
	}
	out := append(usuario, otros...)
	if len(out) < 20 {
		out = append(out, extra[:min(len(extra), 20-len(out))]...)
	}
	if len(out) > crearMaxCasosGuardados {
		out = out[:crearMaxCasosGuardados]
	}
	return out
}

// crearCasosBuenos: the cases with an expectation that passed, user ones first (for crear_programa).
func crearCasosBuenos(v verificacion) []nucleo.Caso {
	var usuario, otros []nucleo.Caso
	for i, cs := range v.Casos {
		if i >= len(v.Resultados) || !v.Resultados[i].OK || !cs.ConEsperado() {
			continue
		}
		if cs.Expectativa == nucleo.EspUsuario {
			usuario = append(usuario, cs)
		} else {
			otros = append(otros, cs)
		}
	}
	return append(usuario, otros...)
}

// crearHuella: the behavioural fingerprint from the probe cases of the run.
func crearHuella(f nucleo.Firma, casos []nucleo.Caso, rs []nucleo.ResultadoCaso) string {
	var filas [][]nucleo.Valor
	for i, cs := range casos {
		if cs.Origen != nucleo.OrigenSonda || i >= len(rs) {
			continue
		}
		if crearTerminado(rs[i]) {
			filas = append(filas, rs[i].Obtenido)
		} else {
			filas = append(filas, nil)
		}
	}
	if len(filas) == 0 {
		return ""
	}
	return nucleo.Huella(f, filas)
}

// crearEntendi: "Así lo entendí: SumaPares([]) = 0; SumaPares([1, 2, 3, 4]) = 6; …" from up to 4 probe
// cases that ran, preferring different outputs.
func crearEntendi(f nucleo.Firma, casos []nucleo.Caso, rs []nucleo.ResultadoCaso) string {
	type par struct {
		i     int
		texto string
	}
	var elegidos []par
	vistoEnt := map[string]bool{}
	vistoSal := map[string]bool{}
	var resto []par
	for i, cs := range casos {
		if cs.Origen != nucleo.OrigenSonda || i >= len(rs) || !rs[i].OK || !crearTerminado(rs[i]) || rs[i].Obtenido == nil {
			continue
		}
		ent := crearLlamada(f, cs.Entradas)
		if vistoEnt[ent] {
			continue
		}
		vistoEnt[ent] = true
		sal := crearValores(f.Res, rs[i].Obtenido)
		p := par{i, ent + " = " + sal}
		if !vistoSal[sal] && len(elegidos) < 4 {
			vistoSal[sal] = true
			elegidos = append(elegidos, p)
		} else {
			resto = append(resto, p)
		}
	}
	for _, p := range resto {
		if len(elegidos) >= 4 {
			break
		}
		elegidos = append(elegidos, p)
	}
	if len(elegidos) == 0 {
		return ""
	}
	sort.Slice(elegidos, func(a, b int) bool { return elegidos[a].i < elegidos[b].i })
	partes := make([]string, len(elegidos))
	for i, p := range elegidos {
		partes[i] = p.texto
	}
	return "Así lo entendí: " + strings.Join(partes, "; ")
}

// crearLlamada: "SumaPares([1, 2, 3, 4])".
func crearLlamada(f nucleo.Firma, ent []nucleo.Valor) string {
	return f.Nombre + "(" + crearArgsSin(f, ent) + ")"
}

// crearArgs: the inputs for a sentence ("[1, 2]" or "3 y 4").
func crearArgs(f nucleo.Firma, ent []nucleo.Valor) string {
	tipos := f.Entradas()
	var partes []string
	for i, v := range ent {
		var t nucleo.Tipo
		if i < len(tipos) {
			t = tipos[i]
		}
		partes = append(partes, nucleo.FormatoHumano(v, t))
	}
	if len(partes) <= 1 {
		return strings.Join(partes, "")
	}
	return strings.Join(partes[:len(partes)-1], ", ") + " y " + partes[len(partes)-1]
}

func crearArgsSin(f nucleo.Firma, ent []nucleo.Valor) string {
	tipos := f.Entradas()
	var partes []string
	for i, v := range ent {
		var t nucleo.Tipo
		if i < len(tipos) {
			t = tipos[i]
		}
		partes = append(partes, nucleo.FormatoHumano(v, t))
	}
	return strings.Join(partes, ", ")
}

func crearValores(tipos []nucleo.Tipo, vs []nucleo.Valor) string {
	if vs == nil {
		return "nada"
	}
	var partes []string
	for i, v := range vs {
		var t nucleo.Tipo
		if i < len(tipos) {
			t = tipos[i]
		}
		partes = append(partes, nucleo.FormatoHumano(v, t))
	}
	return strings.Join(partes, ", ")
}

// crearClaves explains the key conditions of the function: "Clave: «n % 2 == 0» (n es par)."
func crearClaves(fuente, nombre string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", fuente, 0)
	if err != nil {
		return nil
	}
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil && fd.Name.Name == nombre {
			fn = fd
		}
	}
	if fn == nil {
		return nil
	}
	var out []string
	visto := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if len(out) >= 2 {
			return false
		}
		var cond ast.Expr
		switch x := n.(type) {
		case *ast.IfStmt:
			cond = x.Cond
		case *ast.ReturnStmt:
			if len(x.Results) == 1 {
				if b, ok := x.Results[0].(*ast.BinaryExpr); ok && crearComparacion(b.Op) {
					cond = b
				}
			}
		}
		b, ok := cond.(*ast.BinaryExpr)
		if !ok || !crearComparacion(b.Op) && b.Op != token.LAND && b.Op != token.LOR {
			return true
		}
		texto := crearExprTexto(fset, cond)
		if texto == "" || visto[texto] || len(texto) > 60 {
			return true
		}
		visto[texto] = true
		frase := "Clave: «" + texto + "»"
		if es := strings.TrimSpace(analisis.ExprEnEspanol(cond)); es != "" && es != texto {
			frase += " (" + es + ")"
		}
		out = append(out, frase+".")
		return true
	})
	return out
}

func crearComparacion(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return true
	}
	return false
}

// crearExprTexto writes an expression with spaces around every binary operator ("n % 2 == 0").
func crearExprTexto(fset *token.FileSet, e ast.Expr) string {
	switch x := e.(type) {
	case *ast.BinaryExpr:
		return crearExprTexto(fset, x.X) + " " + x.Op.String() + " " + crearExprTexto(fset, x.Y)
	case *ast.ParenExpr:
		return "(" + crearExprTexto(fset, x.X) + ")"
	case *ast.UnaryExpr:
		return x.Op.String() + crearExprTexto(fset, x.X)
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, e); err != nil {
		return ""
	}
	return buf.String()
}

// crearVacias are function words left out of recipe and template searches (they only dilute TF-IDF).
var crearVacias = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "lo": true, "un": true, "una": true, "uno": true, "unos": true,
	"unas": true, "de": true, "del": true, "que": true, "y": true, "o": true, "en": true, "con": true, "a": true,
	"al": true, "por": true, "para": true, "se": true, "le": true, "me": true, "mi": true, "tu": true, "su": true,
	"sus": true, "hacer": true, "haz": true, "dar": true, "devolver": true, "querer": true, "poder": true,
	"este": true, "esta": true, "eso": true, "esto": true, "ese": true, "esa": true, "como": true, "si": true,
	"funcion": true, "dame": true, "quiero": true, "necesito": true,
}

// crearLemas: the request's lemmas without function words, for recetas.BuscarFunciones/BuscarPlantillas.
func crearLemas(q *nucleo.Pregunta) []string {
	var out []string
	for _, l := range lemasDe(q) {
		if !crearVacias[nucleo.Normalizar(l)] {
			out = append(out, l)
		}
	}
	return out
}
