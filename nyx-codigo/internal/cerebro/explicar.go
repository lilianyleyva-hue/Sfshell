package cerebro

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/reparar"
	"nyxcodigo/internal/sintesis"
)

// explicar (§4.14.2): Narrar + Roles + Informe (OGrande, findings); when the target signature can be
// run automatically, a variable trace on a small input, Observar (summary checked on random inputs,
// fingerprint lookup) and, for loops over the input, the measured growth on an uninstrumented build.
// "ponle comentarios" → analisis.Comentar; IOptimizar / "hazlo más rápido" → analisis.Optimizaciones,
// shown only after comparing the rewritten function with the original on generated cases.
//
// A snippet that only compiles once completed (a forgotten standard import, analisis.Normalizar) is run
// completed, and the answer says what was missing. Code that cannot be parsed is not narrated: the
// parser's recovery would make up statements.

// cabeceraFrases is the header of the per-line explanation table. UI contract (servidor/web/app.js,
// actualizarExplicacion): the first Tabla whose first header normalizes to "linea" feeds the editor
// gutter; its rows are [line number, sentence]. redactar keeps such tables whole.
var cabeceraFrases = []string{"Línea", "Frase"}

const (
	explicarMaxEventos    = 200
	explicarFilasTraza    = 40
	explicarAzar          = 300 // random cases that check the summary (§4.5, §1 #5)
	explicarAzarPedidos   = 400 // asked from the generator: duplicates are removed
	explicarLimiteResumen = 1500 * time.Millisecond
	explicarMinMedir      = 4 * time.Second // budget left needed to measure the growth
	explicarMaxMedir      = 6 * time.Second // the longest a growth measurement may take
	explicarReservaMedir  = 2 * time.Second // budget kept for the answer after measuring
	explicarMicrosMedir   = 1000            // the slowest time must reach 1 ms to tell how it grows
	explicarMicrosPunto   = 100             // a time under 0.1 ms is mostly noise: not used to classify
	explicarTiempoCaso    = 2 * time.Second // per-run watchdog when timing a rewrite
	explicarTamMedir      = 10000           // first input size when timing a rewrite
)

func pipeExplicar(c *Cerebro, k *tarea) nucleo.Respuesta {
	codigo := strings.TrimRight(k.codigo, " \t\r\n")
	if strings.TrimSpace(codigo) == "" {
		k.raiz.Sub(nucleo.PasoNota, "No veo código que explicar").Info("")
		return explicarRespuesta("Pega el código que quieres que te explique (en el cuadro de código o en el mensaje).", false)
	}
	codigo += "\n"
	ne := k.raiz.Sub(nucleo.PasoEntender, "Leo el código (go/parser y go/types)")
	inf, err := analisis.Analizar(codigo)
	if err != nil {
		ne.Mal("No lo puedo leer: %s", mensajeError(err))
		return explicarRespuesta("No veo código Go que pueda leer: "+mensajeError(err)+".", false)
	}
	x := &explicacion{c: c, k: k, codigo: codigo, inf: inf}
	if len(inf.Errores) > 0 {
		if f, _, _ := explicarParsear(codigo); f == nil {
			ne.Mal("No se puede leer")
			return x.ilegible()
		}
	}
	fn, ok := explicarElegir(k, inf)
	if !ok {
		ne.Mal("No tiene funciones")
		return explicarRespuesta("No encuentro ninguna función que explicar en este código.", false)
	}
	if fn.Firma != nil {
		ne.Bien("Entiendo: `%s`", fn.Firma.Go())
	} else {
		ne.Bien("Explico «%s»", fn.Nombre)
	}
	x.fn = fn
	x.completar()
	explicarLeerPeticion(k)
	switch {
	case k.comentarios:
		return x.comentar()
	case k.optimizar:
		return x.optimizarCodigo()
	}
	return x.explicar()
}

// explicacion carries the state of one explicar run.
type explicacion struct {
	c      *Cerebro
	k      *tarea
	codigo string
	inf    analisis.Informe
	fn     analisis.InfoFunc

	// the code as given has errors but compiles once analisis.Normalizar adds what it forgot (importados);
	// firmas are the functions of the completed file
	completado bool
	importados []string
	firmas     map[string]analisis.InfoFunc

	roles map[string]string

	// dynamic evidence: only what really ran is reported
	ejecutado      bool   // it was built in the sandbox
	motivoNo       string // why it was not run ("" when it ran)
	traza          string // the input of the variable trace, when it ran
	fraseTraza     string
	tablas         []nucleo.Tabla
	resumen        string // verified summary ("devuelve la suma de …")
	pruebasRes     int
	resumenFallido bool // a summary was tried on random cases and did not fit
	igualA         string
	numSondas      int // probes that ran
	numAzar        int // random cases that ran
	medida         string
	medidas        []analisis.Medida
	notaMedida     string // why there is no measured class
	notaParo       string // the measurement stopped early (it still gave a class)
	comprobados    []nucleo.Comprobacion
}

func explicarRespuesta(texto string, exito bool) nucleo.Respuesta {
	return nucleo.Respuesta{
		Texto:          texto,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "analisis",
		Exito:          exito,
	}
}

// explicarElegir picks the function to explain: the one named in the text, else the first function
// with a signature that can be run, else the first that is not main, else main.
func explicarElegir(k *tarea, inf analisis.Informe) (analisis.InfoFunc, bool) {
	if len(inf.Funciones) == 0 {
		return analisis.InfoFunc{}, false
	}
	pedidas := map[string]bool{}
	if k.q != nil {
		for _, tk := range k.q.Tokens {
			if tk.Clase == "palabra" {
				pedidas[nucleo.Normalizar(tk.Texto)] = true
			}
		}
	}
	for _, f := range inf.Funciones {
		nombre := f.Nombre
		if i := strings.LastIndexByte(nombre, '.'); i >= 0 {
			nombre = nombre[i+1:]
		}
		if pedidas[nucleo.Normalizar(nombre)] {
			return f, true
		}
	}
	var otra *analisis.InfoFunc
	for i, f := range inf.Funciones {
		if f.Nombre == "main" || f.Nombre == "init" {
			continue
		}
		if f.Firma != nil && f.Firma.Probable() {
			return f, true
		}
		if otra == nil {
			otra = &inf.Funciones[i]
		}
	}
	if otra != nil {
		return *otra, true
	}
	return inf.Funciones[0], true
}

// ---- what the request asks for ----

var (
	reExplicarComentarios = regexp.MustCompile(`\b(pon(le|les|ele)?|añade(le|les)?|agrega(le|les)?|mete(le|les)?|escribe(le|les)?) (un |unos |los |algunos )?comentarios?\b|\bcon comentarios\b|\bcomenta(lo|la|me|melo|mela)?\b|\bdocumenta(lo|la)\b`)
	reExplicarDetalle     = regexp.MustCompile(`\b(con (mas )?detalles?|mas detalles?|en detalle|detalladamente|paso a paso|a fondo)\b`)
	reExplicarSimple      = regexp.MustCompile(`\b(mas (facil|simple|sencillo|sencilla|claro|clara)|en palabras sencillas|de forma (simple|sencilla)|para (un |una )?niñ[oa]s?)\b`)
	reExplicarEficiencia  = regexp.MustCompile(`\b(eficien\w*|rapid[oa]s?|rapidez|lent[oa]s?|lentitud|complejidad|cuanto tarda\w*|o grande|big o|coste|costo|rendimiento)\b|\bo\(`)
)

// explicarTexto is the request's text without its code, normalized ("" when the message is only code).
func explicarTexto(k *tarea) string {
	if k.q != nil {
		return k.q.Normal
	}
	return nucleo.Normalizar(k.p.Texto)
}

// explicarLeerPeticion reads "ponle comentarios", "con detalle" and "más simple" in the request itself
// (follow-ups already set them: seguir.go), so a first message with code gets them too (§4.14.2).
func explicarLeerPeticion(k *tarea) {
	t := explicarTexto(k)
	if t == "" {
		return
	}
	if !k.comentarios && reExplicarComentarios.MatchString(t) {
		k.comentarios = true
	}
	if k.detalle == 0 {
		switch {
		case reExplicarDetalle.MatchString(t):
			k.detalle = 2
		case reExplicarSimple.MatchString(t):
			k.detalle = 1
		}
	}
}

// explicarPideEficiencia: the request asks about speed ("¿es eficiente?", "¿cuánto tarda?"). Whole words
// only: "excelente" does not ask whether it is slow.
func explicarPideEficiencia(k *tarea) bool {
	return reExplicarEficiencia.MatchString(explicarTexto(k))
}

// ---- code that does not compile as given ----

// completar looks at code with errors (that parses: pipeExplicar answers ilegible otherwise): completado
// when it compiles once analisis.Normalizar adds the standard imports it forgot (then it can be run that
// way).
func (x *explicacion) completar() {
	if len(x.inf.Errores) == 0 {
		return
	}
	normal, err := analisis.Normalizar(x.codigo, "")
	if err != nil {
		return
	}
	inf, err := analisis.Analizar(normal)
	if err != nil || len(inf.Errores) > 0 {
		return
	}
	x.completado = true
	antes := map[string]bool{}
	for _, im := range x.inf.Imports {
		antes[im] = true
	}
	for _, im := range inf.Imports {
		if !antes[im] {
			x.importados = append(x.importados, im)
		}
	}
	x.firmas = map[string]analisis.InfoFunc{}
	for _, g := range inf.Funciones {
		x.firmas[g.Nombre] = g
	}
	x.fn = x.funcion(x.fn)
}

// funcion is f with the signature of the completed file when the code needed completing (the types of
// a file with errors may be partial).
func (x *explicacion) funcion(f analisis.InfoFunc) analisis.InfoFunc {
	if g, ok := x.firmas[f.Nombre]; ok && g.Firma != nil {
		f.Firma, f.NoProbable = g.Firma, g.NoProbable
	}
	return f
}

// infoDe finds a function of the file by name.
func (x *explicacion) infoDe(nombre string) (analisis.InfoFunc, bool) {
	for _, g := range x.inf.Funciones {
		if g.Nombre == nombre {
			return x.funcion(g), true
		}
	}
	return analisis.InfoFunc{}, false
}

// explicarErrorTexto is a compiler error in simple Spanish (reparar.Traducir), written to go inside a
// sentence: "en la línea 4, usas `strings` pero no lo importaste".
func explicarErrorTexto(e nucleo.ErrorGo) string {
	t := strings.TrimSpace(reparar.Traducir(e))
	if e.Linea > 0 {
		t = strings.TrimSpace(strings.TrimSuffix(t, "(línea "+strconv.Itoa(e.Linea)+")"))
	}
	t = explicarMinuscula(strings.TrimRight(t, ". "))
	if e.Linea > 0 {
		return "en la línea " + strconv.Itoa(e.Linea) + ", " + t
	}
	return t
}

// explicarMinuscula lowers the first letter unless the word looks like a name ("Go", "OK").
func explicarMinuscula(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError || !unicode.IsUpper(r) {
		return s
	}
	if r2, _ := utf8.DecodeRuneInString(s[n:]); unicode.IsUpper(r2) || strings.HasPrefix(s, "Go ") {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

// explicarPanico is a runtime error in simple Spanish, to go inside parentheses: "intentaste leer la
// posición 3 de una lista vacía".
func explicarPanico(panico string) string {
	p := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(primeraLinea(panico)), "panic:"))
	return explicarMinuscula(strings.TrimRight(reparar.TraducirPanico(p), ". "))
}

// explicarImports: "`strings`", "`strings` y `os`".
func explicarImports(rutas []string) string {
	ps := make([]string, len(rutas))
	for i, r := range rutas {
		ps[i] = "`" + r + "`"
	}
	if len(ps) == 1 {
		return ps[0]
	}
	return strings.Join(ps[:len(ps)-1], ", ") + " y " + ps[len(ps)-1]
}

// ilegible answers for code that cannot be parsed: narrating it would describe statements the parser made
// up to recover.
func (x *explicacion) ilegible() nucleo.Respuesta {
	motivo := "tiene un error de escritura"
	if len(x.inf.Errores) > 0 {
		motivo = explicarErrorTexto(x.inf.Errores[0])
	}
	x.k.raiz.Sub(nucleo.PasoError, "No se puede leer: %s", motivo).Mal("")
	r := explicarRespuesta("Así como está, este código no se puede leer ("+motivo+").", false)
	r.Parrafos = []string{"No te lo explico línea a línea: te contaría cosas que no están en tu código. Arréglalo primero (o pídeme «arréglalo») y te lo explico."}
	r.Sugerencias = []string{"Arréglalo"}
	x.k.turno.Metodo = "analisis"
	return r
}

// ---- the explanation ----

func (x *explicacion) explicar() nucleo.Respuesta {
	k, c := x.k, x.c
	nivel := 1
	if k.detalle == 2 {
		nivel = 2
	}
	nn := k.raiz.Sub(nucleo.PasoIntento, "Lo explico línea a línea")
	frases, err := analisis.Narrar(x.codigo, x.fn.Nombre, nivel)
	if err != nil || len(frases) == 0 {
		motivo := "no encontré qué contar"
		if err != nil {
			motivo = mensajeError(err)
		}
		nn.Mal("No pude: %s", motivo)
		return explicarRespuesta("No sé explicar este código: "+motivo+".", false)
	}
	tabla := nucleo.Tabla{Titulo: "Qué hace cada línea", Cabecera: append([]string(nil), cabeceraFrases...)}
	for _, f := range frases {
		tabla.Filas = append(tabla.Filas, []string{strconv.Itoa(f.Linea), f.Texto})
	}
	nn.Tabla(tabla)
	nn.Bien("Lo explico en %d frases", len(frases))

	x.roles, _ = analisis.Roles(x.codigo, x.fn.Nombre)
	if len(x.roles) > 0 {
		k.raiz.Sub(nucleo.PasoNota, "Papel de cada variable: %s", explicarListaRoles(x.roles)).Info("")
	}
	if x.fn.OGrande != "" {
		k.raiz.Sub(nucleo.PasoNota, "Estimo %s mirando el código", x.fn.OGrande).Info("")
	}

	// run it, when that is safe and possible
	switch {
	case len(x.inf.Errores) > 0 && !x.completado:
		x.motivoNo = "no compila"
	case x.fn.Firma == nil || !x.fn.Firma.Probable():
		x.motivoNo = "no sé con qué datos llamarla"
		if x.fn.NoProbable != "" {
			x.motivoNo = x.fn.NoProbable
		}
	case c.ej == nil:
		x.motivoNo = "no tengo caja segura para ejecutar código"
	default:
		x.ejecutar()
	}

	pideEficiencia := explicarPideEficiencia(k)
	fraseO := x.fraseComplejidad()
	var r nucleo.Respuesta
	r.Comprobaciones = append([]nucleo.Comprobacion{}, x.comprobados...)
	r.Metodo = "analisis"
	r.Exito = true
	r.Codigo = x.codigo
	cabecera := frases[0].Texto
	switch {
	case pideEficiencia && fraseO != "":
		r.Texto = fraseO
		if v := explicarVeredicto(x.fn.OGrande); v != "" {
			r.Parrafos = append(r.Parrafos, v)
		}
		r.Parrafos = append(r.Parrafos, cabecera)
	case x.resumen != "":
		r.Texto = fmt.Sprintf("En resumen, `%s` %s (lo comprobé con %d entradas al azar).", explicarNombreCorto(x.fn.Nombre), x.resumen, x.pruebasRes)
		r.Parrafos = append(r.Parrafos, cabecera)
	default:
		r.Texto = cabecera
	}
	if pideEficiencia && x.resumen != "" {
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("En resumen, %s (lo comprobé con %d entradas al azar).", x.resumen, x.pruebasRes))
	}
	if s := explicarFraseRoles(x.roles, k.detalle); s != "" {
		r.Parrafos = append(r.Parrafos, s)
	}
	if !pideEficiencia && fraseO != "" {
		r.Parrafos = append(r.Parrafos, fraseO)
	}
	if x.fraseTraza != "" {
		r.Parrafos = append(r.Parrafos, x.fraseTraza)
	}
	if x.igualA != "" {
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("Con las %d entradas fijas da lo mismo que `%s`, que ya conozco.", x.numSondas, x.igualA))
	}
	if s := explicarAvisos(x.inf.Hallazgos); s != "" {
		r.Parrafos = append(r.Parrafos, s)
	}
	switch {
	case len(x.inf.Errores) > 0 && x.completado:
		p := "Ojo: así como está no compila (" + explicarErrorTexto(x.inf.Errores[0]) + ")."
		if len(x.importados) > 0 {
			p += " Le falta importar " + explicarImports(x.importados) + "."
		}
		if x.ejecutado {
			p += " Para ejecutarlo se lo añadí."
		}
		r.Parrafos = append(r.Parrafos, p)
		if !x.ejecutado && x.motivoNo != "" && k.detalle != 1 {
			r.Parrafos = append(r.Parrafos, "No lo he ejecutado: "+x.motivoNo+". Lo que digo sale de leer el código.")
		}
	case len(x.inf.Errores) > 0:
		r.Parrafos = append(r.Parrafos, "Ojo: así como está no compila ("+explicarErrorTexto(x.inf.Errores[0])+"), así que no lo he ejecutado.")
	case !x.ejecutado && x.motivoNo != "" && k.detalle != 1:
		r.Parrafos = append(r.Parrafos, "No lo he ejecutado: "+x.motivoNo+". Lo que digo sale de leer el código.")
	}
	if otras := x.otrasFunciones(); otras != "" {
		r.Parrafos = append(r.Parrafos, otras)
	}

	r.Tablas = append(r.Tablas, tabla)
	r.Tablas = append(r.Tablas, x.tablas...)
	if len(x.medidas) > 0 {
		tm := nucleo.Tabla{Titulo: "Tiempos medidos (sin instrumentar)", Cabecera: []string{"n", "tiempo (µs)"}}
		for _, m := range x.medidas {
			tm.Filas = append(tm.Filas, []string{strconv.Itoa(m.N), strconv.FormatInt(m.Micros, 10)})
		}
		r.Tablas = append(r.Tablas, tm)
	}

	r.Justificacion = append(r.Justificacion, "Leí el código con go/parser y go/types, sin ejecutarlo, para explicarlo línea a línea y ver el papel de cada variable.")
	if x.fn.OGrande != "" {
		j := "La complejidad " + x.fn.OGrande + " la estimé mirando los bucles y las llamadas"
		if x.fn.Motivo != "" {
			j += ": " + x.fn.Motivo
		}
		r.Justificacion = append(r.Justificacion, j+".")
	}
	if s := x.fraseEjecuciones(); s != "" {
		r.Justificacion = append(r.Justificacion, s)
	}
	if x.resumen != "" {
		r.Justificacion = append(r.Justificacion, fmt.Sprintf("Busqué un programa corto que diera lo mismo con las entradas fijas y lo comparé con tu código en %d entradas al azar: coincide en todas.", x.pruebasRes))
	} else if x.resumenFallido {
		r.Justificacion = append(r.Justificacion, "Probé a describirlo con un programa corto, pero no daba lo mismo que tu código en las entradas al azar, así que no te doy un resumen.")
	}
	if len(x.medidas) > 0 {
		var ns []string
		for _, m := range x.medidas {
			ns = append(ns, strconv.Itoa(m.N))
		}
		r.Justificacion = append(r.Justificacion, "Medí el tiempo con n = "+strings.Join(ns, ", ")+" en una versión sin instrumentar (mediana de 3 ejecuciones).")
	}
	if k.detalle == 2 {
		r.Sugerencias = []string{"Explícalo más simple", "Ponle comentarios", "Hazle pruebas"}
	} else {
		r.Sugerencias = []string{"Explícalo con detalle", "Ponle comentarios", "Hazle pruebas"}
	}
	if len(x.inf.Errores) > 0 {
		r.Sugerencias = []string{"Arréglalo", "Explícalo con detalle"}
	}
	k.turno.Metodo = "analisis"
	return r
}

// fraseEjecuciones says what really ran (for "¿por qué?"): only the probes and random cases that were
// executed are counted.
func (x *explicacion) fraseEjecuciones() string {
	if !x.ejecutado {
		return ""
	}
	var partes []string
	if x.numSondas > 0 {
		partes = append(partes, cuantas(x.numSondas, "entrada fija", "entradas fijas"))
	}
	if x.numAzar > 0 {
		partes = append(partes, fmt.Sprintf("%d al azar (semilla %d)", x.numAzar, x.k.semilla))
	}
	switch {
	case len(partes) > 0:
		return "Lo compilé en la caja segura y lo ejecuté con " + strings.Join(partes, " y ") + "."
	case x.traza != "":
		return "Lo compilé en la caja segura y lo ejecuté con " + x.traza + "."
	}
	return "Lo compilé en la caja segura, pero no conseguí ejecutarlo."
}

// otrasFunciones mentions the other functions of the file.
func (x *explicacion) otrasFunciones() string {
	var otras []string
	for _, f := range x.inf.Funciones {
		if f.Nombre != x.fn.Nombre && f.Nombre != "init" {
			otras = append(otras, "`"+f.Nombre+"`")
		}
	}
	if len(otras) == 0 {
		return ""
	}
	if len(otras) > 4 {
		otras = append(otras[:4], "…")
	}
	return "He explicado `" + x.fn.Nombre + "`. También hay " + strings.Join(otras, ", ") + ": pídeme «explica " + strings.Trim(otras[0], "`") + "» si quieres."
}

// ejecutar builds the target function with the variable trace, follows one small input, observes it
// on probes and random cases, and measures its growth on an uninstrumented build.
func (x *explicacion) ejecutar() {
	k, c := x.k, x.c
	f := *x.fn.Firma
	n := k.raiz.Sub(nucleo.PasoComprobar, "Lo ejecuto para ver qué hace de verdad")
	fuente, err := explicarFuente(x.codigo, x.fn.Nombre)
	if err != nil {
		x.motivoNo = "no pude prepararlo para compilar (" + mensajeError(err) + ")"
		n.Info("No lo ejecuto: %s", x.motivoNo)
		return
	}
	if v := c.ej.Revisar(fuente, nucleo.PermisoAuto); len(v) > 0 {
		x.motivoNo = "por seguridad no lo ejecuto sin tu permiso (" + v[0].Que + ")"
		n.Info("No lo ejecuto solo: %s", v[0].Que)
		return
	}
	instr := nucleo.InstrNormal
	instr.Variables = true
	instr.MaxEventos = explicarMaxEventos
	b, comp, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Instr: instr, Permiso: nucleo.PermisoAuto})
	if err != nil {
		x.motivoNo = "no compila en la caja segura"
		if len(comp.Errores) > 0 {
			e := comp.Errores[0]
			e.Linea = 0 // a line of the file I built, not of yours
			x.motivoNo += " (" + explicarErrorTexto(e) + ")"
		}
		n.Mal("No lo pude ejecutar")
		return
	}
	defer b.Cerrar()
	x.ejecutado = true
	if k.detalle != 1 {
		x.trazar(n, b, f, fuente)
	}
	x.observar(n, b, f)
	if k.detalle != 1 {
		x.medir(n, fuente, f)
	}
	n.Bien("")
}

// trazar follows the variables on one small input. Event lines are lines of the built file (normalized
// and gofmt'd); explicarMapaLineas takes them back to the user's lines statement by statement.
func (x *explicacion) trazar(n *nucleo.Nodo, b nucleo.Binario, f nucleo.Firma, fuente string) {
	k := x.k
	caso, ok := explicarCasoPequeno(x.c, k, f)
	if !ok {
		return
	}
	ent := explicarEntradas(f, caso.Entradas)
	nt := n.Sub(nucleo.PasoPrueba, "Sigo las variables con %s", ent)
	rs, err := b.Probar(k.ctx, []nucleo.Caso{caso}, nucleo.OpcionesProbar{Variantes: []int{0}})
	if err != nil || len(rs) == 0 || len(rs[0]) == 0 || !rs[0][0].Ejecutado {
		nt.Mal("No pude seguirlas")
		return
	}
	x.traza = ent
	r0 := rs[0][0]
	lineas := explicarMapaLineas(x.codigo, fuente, x.fn.Nombre)
	tb := nucleo.Tabla{Titulo: "Así cambian las variables con " + ent, Cabecera: []string{"Paso", "Línea", "Variable", "Valor"}}
	for i, e := range r0.Eventos {
		if i >= explicarFilasTraza {
			break
		}
		linea := ""
		if l, ok := lineas[e.Linea]; ok && l >= x.fn.Desde && l <= x.fn.Hasta {
			linea = strconv.Itoa(l)
		}
		tb.Filas = append(tb.Filas, []string{strconv.Itoa(i + 1), linea, e.Var, e.Valor})
	}
	if len(tb.Filas) > 0 {
		nt.Tabla(tb)
		x.tablas = append(x.tablas, tb)
	}
	x.fraseTraza = explicarFraseTraza(f, ent, r0, x.roles)
	nt.Bien("%d cambios de variables", len(r0.Eventos))
}

// observar runs Observar with the synthesis summarizer and the memory fingerprint lookup.
func (x *explicacion) observar(n *nucleo.Nodo, b nucleo.Binario, f nucleo.Firma) {
	k, c := x.k, x.c
	sondas := nucleo.Sondas(f)
	if len(sondas) == 0 {
		return
	}
	aleatorios := explicarMuestra(c.generarCasos(k, f, nil, -1, explicarAzarPedidos), explicarAzar)
	var resumir analisis.Resumidor
	if sintesis.Sintetizable(f) {
		resumir = x.resumidor()
	}
	igual := func(h string) (string, bool) {
		for _, g := range c.mem.PorHuella(h) {
			if g.Nombre != "" {
				return g.Nombre, true
			}
		}
		return "", false
	}
	comp, err := analisis.Observar(k.ctx, b, f, sondas, aleatorios, resumir, igual, n)
	if err != nil {
		return
	}
	// only what ran: the random cases run only when a summary fits every probe (comp.Pruebas)
	x.numSondas, x.numAzar = len(sondas), comp.Pruebas
	if len(comp.Tabla.Filas) > 0 {
		x.tablas = append(x.tablas, comp.Tabla)
	}
	if comp.Verificado && comp.Resumen != "" && comp.Pruebas > 0 {
		x.resumen, x.pruebasRes = comp.Resumen, comp.Pruebas
		x.comprobados = append(x.comprobados, nucleo.Comprobacion{
			Nivel:   nucleo.NivelEntendido,
			Pasan:   comp.Pruebas,
			Total:   comp.Pruebas,
			Texto:   fmt.Sprintf("Resumen comprobado con %d entradas al azar", comp.Pruebas),
			Detalle: fmt.Sprintf("semilla %d; %d entradas fijas y %d al azar", k.semilla, len(sondas), comp.Pruebas),
		})
	} else if comp.Pruebas > 0 {
		x.resumenFallido = true
	}
	x.igualA = comp.IgualA
}

// explicarMuestra keeps at most n cases spread over the list (the generator's cases grow in size, so
// keeping the first n would drop the largest).
func explicarMuestra(cs []nucleo.Caso, n int) []nucleo.Caso {
	if len(cs) <= n {
		return cs
	}
	out := make([]nucleo.Caso, n)
	for i := range out {
		out[i] = cs[i*len(cs)/n]
	}
	return out
}

// resumidor describes the observed pairs with a short DSL program (sintesis, 1.5 s).
func (x *explicacion) resumidor() analisis.Resumidor {
	lim := explicarLimiteResumen
	if x.k.pensarMas {
		lim *= 2
	}
	return func(ctx context.Context, f nucleo.Firma, casos []nucleo.Caso) (string, func([]nucleo.Valor) ([]nucleo.Valor, error), bool) {
		cctx, cancelar := context.WithTimeout(ctx, lim)
		defer cancelar()
		var res sintesis.Resultado
		err := seguro(func() error {
			var e error
			res, e = sintesis.Sintetizar(cctx, sintesis.Especificacion{Firma: f, Ejemplos: casos}, x.c.registro(),
				sintesis.Opciones{Limite: lim, MaxSoluciones: 1, Priors: x.c.stats, Semilla: x.k.semilla}, nil)
			return e
		})
		if err != nil || len(res.Soluciones) == 0 || res.Soluciones[0].Expr == nil {
			return "", nil, false
		}
		e := res.Soluciones[0].Expr
		return "devuelve " + sintesis.Describir(e, f), sintesis.Oraculo(e), true
	}
}

// medir measures the growth when the static estimate is at least O(n) and an input can grow. The
// measurement has its own time limit (explicarMaxMedir), and a class is only reported when the times are
// large enough to mean something (explicarClasificarMedidas).
func (x *explicacion) medir(n *nucleo.Nodo, fuente string, f nucleo.Firma) {
	k, c := x.k, x.c
	maxN, nota := explicarMaxN(x.fn.OGrande)
	if maxN == 0 {
		x.notaMedida = nota
		return
	}
	gen := explicarGenerador(f, maxN)
	if gen == nil {
		x.notaMedida = "no sé fabricar entradas grandes de estos tipos"
		return
	}
	if k.restante() < explicarMinMedir {
		x.notaMedida = "no me quedaba tiempo para medirlo"
		n.Nota("No mido cuánto tarda: no me queda tiempo")
		return
	}
	b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Instr: nucleo.OpcionesInstr{}, Permiso: nucleo.PermisoAuto})
	if err != nil {
		x.notaMedida = "no pude compilar la versión para medir"
		return
	}
	defer b.Cerrar()
	lim := explicarLimiteMedir(k)
	cctx, cancelar := context.WithTimeout(k.ctx, lim)
	_, ms, err := analisis.Crecimiento(cctx, b, gen, n)
	agotado := cctx.Err() != nil && k.ctx.Err() == nil
	cancelar()
	x.medidas = ms
	if err != nil {
		switch {
		case agotado:
			paro := "paré de medir a los " + explicarTiempo(lim.Microseconds()) + ": tardaba demasiado"
			if cl, _ := explicarClasificarMedidas(ms); cl != "" {
				x.medida, x.notaParo = cl, paro
				n.Nota("Con las medidas que tengo: crece de forma %s (%s)", cl, paro)
				return
			}
			x.notaMedida = "con entradas grandes tardaba demasiado y paré de medir a los " + explicarTiempo(lim.Microseconds())
			n.Nota("Paro de medir: tarda demasiado")
		case k.ctx.Err() != nil:
			x.notaMedida = "se me acabó el tiempo antes de terminar de medirlo"
		default:
			x.notaMedida = "no conseguí medidas suficientes"
		}
		return
	}
	cl, nota := explicarClasificarMedidas(ms)
	if cl == "" {
		x.notaMedida = nota
		n.Nota("No me fío de esa medida: %s", nota)
		return
	}
	x.medida = cl
}

// explicarLimiteMedir is the time a measurement may take: explicarMaxMedir, keeping explicarReservaMedir
// of the budget for the answer.
func explicarLimiteMedir(k *tarea) time.Duration {
	lim := explicarMaxMedir
	if r := k.restante() - explicarReservaMedir; r < lim {
		lim = r
	}
	if lim < time.Second {
		lim = time.Second
	}
	return lim
}

// explicarClasificarMedidas says how the time grows, or why the timings cannot tell (nota). Times under
// explicarMicrosPunto are noise and are left out; when even the slowest run is under explicarMicrosMedir,
// or fewer than two times remain, there is no class: a sub-millisecond fit is not a measurement.
func explicarClasificarMedidas(ms []analisis.Medida) (clase, nota string) {
	if len(ms) == 0 {
		return "", "no conseguí medidas"
	}
	mayor := ms[0]
	var fiables []analisis.Medida
	for _, m := range ms {
		if m.Micros > mayor.Micros {
			mayor = m
		}
		if m.Micros >= explicarMicrosPunto {
			fiables = append(fiables, m)
		}
	}
	if mayor.Micros < explicarMicrosMedir {
		u := ms[len(ms)-1]
		return "", fmt.Sprintf("va tan rápido que no puedo medir cómo crece (con n = %d tarda %s)", u.N, explicarTiempo(u.Micros))
	}
	if len(fiables) < 2 {
		return "", fmt.Sprintf("solo con n = %d tarda lo bastante para medirlo, y con una sola medida no se ve cómo crece", mayor.N)
	}
	return analisis.ClasificarCrecimiento(fiables), ""
}

// explicarTiempo writes a time in µs for people: "178 µs", "1,5 ms", "2,25 s".
func explicarTiempo(us int64) string {
	switch {
	case us < 1000:
		return strconv.FormatInt(us, 10) + " µs"
	case us < 1_000_000:
		return strings.Replace(strconv.FormatFloat(float64(us)/1000, 'f', 1, 64), ".", ",", 1) + " ms"
	}
	t := strconv.FormatFloat(float64(us)/1e6, 'f', 2, 64)
	t = strings.TrimSuffix(strings.TrimSuffix(t, "0"), ".0")
	return strings.Replace(t, ".", ",", 1) + " s"
}

// fraseComplejidad: "Estimé O(n²) (cuadrática) mirando el código: …; midiendo parece cuadrática."
func (x *explicacion) fraseComplejidad() string {
	o := x.fn.OGrande
	if o == "" {
		return ""
	}
	s := "Estimé " + o
	est := explicarClase(o)
	if est != "" {
		s += " (" + est + ")"
	}
	s += " mirando el código"
	if x.fn.Motivo != "" {
		s += ": " + strings.TrimRight(x.fn.Motivo, ".")
	}
	if x.medida != "" {
		switch cmp := explicarComparar(o, x.medida); {
		case est == x.medida:
			s += "; midiendo parece " + x.medida
		case explicarCoinciden(est, x.medida):
			s += "; midiendo parece lineal o n·log n (al medir casi no se distinguen)"
		case cmp == 0:
			s += "; midiendo crece de forma parecida (parece " + x.medida + ", y al medir no se distinguen)"
		case cmp > 0:
			s += "; pero midiendo parece " + x.medida + ", así que no coinciden: crece más deprisa de lo que estimé"
		default:
			s += "; pero midiendo parece " + x.medida + ", así que no coinciden (puede que con mis entradas termine antes de recorrerlo todo)"
		}
		if x.notaParo != "" {
			s += " (" + x.notaParo + ")"
		}
	} else if x.notaMedida != "" {
		s += " (no lo he medido: " + x.notaMedida + ")"
	}
	return s + "."
}

// ---- comments ----

func (x *explicacion) comentar() nucleo.Respuesta {
	k := x.k
	nc := k.raiz.Sub(nucleo.PasoIntento, "Le pongo comentarios en español")
	com, err := analisis.Comentar(x.codigo)
	if err != nil {
		nc.Mal("No pude: %s", mensajeError(err))
		return explicarRespuesta("No pude ponerle comentarios: "+mensajeError(err)+".", false)
	}
	nc.Codigo(com)
	nc.Bien("")
	r := explicarRespuesta("Aquí lo tienes con comentarios en español.", true)
	r.Nivel = ""
	r.Codigo = com
	if explicarMismoCodigo(x.codigo, com) {
		k.raiz.Sub(nucleo.PasoComprobar, "Compruebo que solo cambian los comentarios").Bien("")
		r.Parrafos = append(r.Parrafos, "Solo he añadido comentarios: lo comparé sin ellos y el código es el mismo.")
	}
	r.Justificacion = []string{"Cada comentario es la frase que explica esa línea (analisis.Comentar), y la función lleva un comentario que dice qué recibe y qué devuelve."}
	r.Sugerencias = []string{"Explícalo con detalle", "Hazle pruebas"}
	k.turno.Metodo = "analisis"
	return r
}

// explicarMismoCodigo compares two sources without their comments (gofmt'd).
func explicarMismoCodigo(a, b string) bool {
	fa, okA := explicarSinComentarios(a)
	fb, okB := explicarSinComentarios(b)
	return okA && okB && fa == fb
}

func explicarSinComentarios(src string) (string, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		f, err = parser.ParseFile(fset, "x.go", "package p;"+src, 0)
		if err != nil {
			return "", false
		}
	}
	f.Name.Name = "p"
	var sb strings.Builder
	if err := format.Node(&sb, fset, f); err != nil {
		return "", false
	}
	// removed comments leave blank lines behind: they do not change the code
	var lineas []string
	for _, l := range strings.Split(sb.String(), "\n") {
		if strings.TrimSpace(l) != "" {
			lineas = append(lineas, l)
		}
	}
	return strings.Join(lineas, "\n"), true
}

// ---- optimize ----

func (x *explicacion) optimizarCodigo() nucleo.Respuesta {
	k := x.k
	no := k.raiz.Sub(nucleo.PasoIntento, "Busco partes lentas que se puedan reescribir")
	rws := analisis.Optimizaciones(x.codigo)
	if len(rws) == 0 {
		no.Info("No encuentro ninguna")
		r := explicarRespuesta("No veo nada claro que pueda hacer más rápido sin cambiar lo que hace.", true)
		r.Nivel = ""
		if fraseO := x.fraseComplejidad(); fraseO != "" {
			r.Parrafos = append(r.Parrafos, fraseO)
		}
		r.Sugerencias = []string{"Explícalo", "Hazle pruebas"}
		return r
	}
	// compare and time the function the rewrite really changes, not the one picked to explain
	rw, objetivo := x.elegirReescritura(rws)
	if objetivo != "" && objetivo != x.fn.Nombre {
		if g, ok := x.infoDe(objetivo); ok {
			x.fn = g
			k.raiz.Nota("El cambio está en `%s`: comparo y mido esa función", objetivo)
		} else {
			objetivo = ""
		}
	}
	donde := "este código"
	if objetivo != "" {
		donde = "`" + explicarNombreCorto(objetivo) + "`"
	}
	no.Bien("%s: %s", mayusculaInicial(donde), rw.Hallazgo.Mensaje)
	r := explicarRespuesta("", true)
	r.Nivel = ""
	r.Codigo = rw.Fuente
	r.Diff = reparar.Diff(x.codigo, rw.Fuente)
	var notas []string
	// Hallazgo.Linea is a line of the code as given only when it type-checks as given (else it is a line
	// of the completed file, so it is not shown)
	lugar := "Lo lento está en " + donde + "."
	if len(x.inf.Errores) == 0 && rw.Hallazgo.Linea > 0 {
		if objetivo != "" {
			lugar = fmt.Sprintf("Lo lento está en la línea %d, dentro de %s.", rw.Hallazgo.Linea, donde)
		} else {
			lugar = fmt.Sprintf("Lo lento está en la línea %d.", rw.Hallazgo.Linea)
		}
	}
	notas = append(notas, strings.TrimSpace(lugar+" "+strings.TrimSpace(rw.Hallazgo.Mensaje)))
	if rw.Hallazgo.Consejo != "" {
		notas = append(notas, mayusculaInicial(strings.TrimRight(rw.Hallazgo.Consejo, "."))+".")
	}
	if len(rws) > 1 {
		if len(rws) == 2 {
			notas = append(notas, "Hay 1 cambio posible más; pídemelo otra vez cuando apliques este.")
		} else {
			notas = append(notas, fmt.Sprintf("Hay %d cambios posibles más; pídemelo otra vez cuando apliques este.", len(rws)-1))
		}
	}

	var comp *nucleo.Comprobacion
	var diferencia, motivo string
	if objetivo == "" {
		motivo = "no sé en qué función está el cambio"
	} else {
		comp, diferencia, motivo = x.compararReescritura(rw.Fuente)
	}
	medido := false
	switch {
	case comp != nil:
		// same results is checked; "faster" is only said when the timing shows it (§0)
		r.Comprobaciones = append(r.Comprobaciones, *comp)
		tiempo, rapido, porque := x.medirReescritura(rw.Fuente)
		medido = tiempo != ""
		switch {
		case rapido:
			r.Texto = "Así " + donde + " va más rápido y da lo mismo que tu código."
		case medido:
			r.Texto = "Este cambio en " + donde + " da lo mismo que tu código, pero al medirlo no he visto que vaya más rápido."
		default:
			r.Texto = "Creo que así " + donde + " va más rápido, aunque no he podido medirlo (" + porque + ")."
		}
		pre := []string{fmt.Sprintf("Comparé la nueva %s con la tuya en %d pruebas y dan lo mismo.", donde, comp.Total)}
		if medido {
			pre = append(pre, tiempo)
		}
		r.Parrafos = append(pre, notas...)
	case diferencia != "":
		r.Texto = "Encontré un cambio posible en " + donde + ", pero no te lo propongo: " + diferencia + "."
		r.Parrafos = notas[:1]
		r.Codigo, r.Diff = "", ""
		r.Exito = false
		r.Nivel = nucleo.NivelSinComprobar
	default:
		r.Texto = afirmar("así "+donde+" va más rápido y hace lo mismo, pero no lo he podido comprobar ("+motivo+").", nil)
		r.Parrafos = notas
		r.Nivel = nucleo.NivelSinComprobar
	}
	if fraseO := x.fraseComplejidad(); fraseO != "" && objetivo != "" {
		r.Parrafos = append(r.Parrafos, fraseO)
	}
	r.Justificacion = []string{"Busqué patrones lentos conocidos (concatenar textos en un bucle, buscar en una lista dentro de un bucle, append sin capacidad).",
		"Antes de proponer el cambio comparo la función que cambia con la tuya en las mismas entradas."}
	if medido {
		r.Justificacion = append(r.Justificacion, "Medí las dos con la misma entrada grande en una versión sin instrumentar (mediana de 3 ejecuciones).")
	}
	r.Sugerencias = []string{"Explícalo", "Hazle pruebas"}
	k.turno.Metodo = "optimizar"
	return r
}

// elegirReescritura picks the rewrite to show and the function it changes ("" when that cannot be told):
// one that changes the function being explained, else the first.
func (x *explicacion) elegirReescritura(rws []analisis.Reescritura) (analisis.Reescritura, string) {
	antes, err := analisis.Normalizar(x.codigo, "")
	if err != nil {
		antes = x.codigo
	}
	elegida, nombre := -1, ""
	for i, rw := range rws {
		cambiadas := explicarCambiadas(antes, rw.Fuente)
		if len(cambiadas) == 0 {
			continue
		}
		for _, c := range cambiadas {
			if c == x.fn.Nombre {
				return rw, c
			}
		}
		if elegida < 0 {
			elegida, nombre = i, cambiadas[0]
		}
	}
	if elegida < 0 {
		return rws[0], ""
	}
	return rws[elegida], nombre
}

// medirReescritura times the original and the rewrite on the same large input (uninstrumented build,
// median of 3 runs each): first with n = 10000, then with n = 100000 when that was too fast to tell and
// the estimate allows it. frase gives the times ("" when it could not measure, porque says why);
// rapido is true only when the rewrite is clearly faster.
func (x *explicacion) medirReescritura(nueva string) (frase string, rapido bool, porque string) {
	k, c := x.k, x.c
	if x.fn.Firma == nil || c.ej == nil {
		return "", false, "no lo puedo ejecutar"
	}
	f := *x.fn.Firma
	maxN, _ := explicarMaxN(x.fn.OGrande)
	if maxN < explicarTamMedir {
		maxN = explicarTamMedir
	}
	gen := explicarGenerador(f, maxN)
	if gen == nil {
		return "", false, "no sé fabricar entradas grandes de estos tipos"
	}
	if k.restante() < explicarMinMedir {
		return "", false, "no me quedaba tiempo"
	}
	orig, err := explicarFuente(x.codigo, x.fn.Nombre)
	if err != nil {
		return "", false, mensajeError(err)
	}
	nuevo, err := explicarFuente(nueva, x.fn.Nombre)
	if err != nil {
		return "", false, mensajeError(err)
	}
	n := k.raiz.Sub(nucleo.PasoPrueba, "Mido cuánto tardan los dos")
	b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{orig, nuevo}, Firma: f, Instr: nucleo.OpcionesInstr{}, Permiso: nucleo.PermisoAuto})
	if err != nil {
		n.Mal("")
		return "", false, "no pude compilar la versión para medir"
	}
	defer b.Cerrar()
	ctx, cancelar := context.WithTimeout(k.ctx, explicarLimiteMedir(k))
	defer cancelar()
	for tam := explicarTamMedir; tam <= maxN; tam *= 10 {
		caso, ok := gen(tam)
		if !ok {
			break
		}
		tNuevo, err := explicarCronometrar(ctx, b, 1, caso)
		if err != nil || tNuevo < 0 {
			break
		}
		inicio := time.Now()
		tOrig, err := explicarCronometrar(ctx, b, 0, caso)
		cortado := false
		switch {
		case err != nil && ctx.Err() != nil && k.ctx.Err() == nil:
			// the original had not finished when the time to measure ran out: it took at least this long
			tOrig, cortado = time.Since(inicio).Microseconds(), true
		case err != nil:
			tOrig = -1
		case tOrig < 0:
			tOrig, cortado = explicarTiempoCaso.Microseconds(), true // the watchdog stopped it
		}
		if tOrig < 0 {
			break
		}
		frase, rapido = explicarVeredictoTiempo(tam, tOrig, tNuevo, cortado)
		if cortado || tOrig >= explicarMicrosMedir || tam*10 > maxN {
			break
		}
		n.Progreso("con n = %d tardan muy poco: pruebo más grande", tam)
	}
	switch {
	case frase == "":
		n.Mal("")
		return "", false, "no pude terminar de medirlo"
	case rapido:
		n.Bien("%s", frase)
	default:
		n.Info("%s", frase)
	}
	return frase, rapido, ""
}

// explicarCronometrar is the median time (µs) of 3 runs of one variant on caso; -1 when it did not finish
// within explicarTiempoCaso (it stops at the first run that does not).
func explicarCronometrar(ctx context.Context, b nucleo.Binario, v int, caso nucleo.Caso) (int64, error) {
	rs, err := b.Probar(ctx, []nucleo.Caso{caso, caso, caso}, nucleo.OpcionesProbar{Variantes: []int{v}, TiempoCaso: explicarTiempoCaso, PararAlFallar: true})
	if err != nil {
		return 0, err
	}
	if v >= len(rs) {
		return 0, errors.New("el arenero no devolvió resultados")
	}
	var ts []int64
	for _, r := range rs[v] {
		switch {
		case r.Agotado || r.SinCombustible:
			return -1, nil
		case r.Ejecutado && r.OK:
			ts = append(ts, r.Micros)
		}
	}
	if len(ts) == 0 {
		return 0, errors.New("falló al medirlo")
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	return ts[len(ts)/2], nil
}

// explicarVeredictoTiempo compares the original's time with the rewrite's on one input of size n
// (cortado: the original was stopped after orig µs without finishing). The rewrite is "faster" only when
// the original took at least 1 ms and the rewrite at most 80 % of it.
func explicarVeredictoTiempo(n int, orig, nuevo int64, cortado bool) (string, bool) {
	rapido := orig >= explicarMicrosMedir && nuevo*10 <= orig*8
	if cortado {
		s := fmt.Sprintf("Lo medí con n = %d: tu código seguía sin terminar a los %s (no esperé más) y el nuevo tarda %s", n, explicarTiempo(orig), explicarTiempo(nuevo))
		if !rapido {
			return s + ", así que no sé cuál va más rápido.", false
		}
		return s + ".", true
	}
	base := fmt.Sprintf("Lo medí con n = %d: tu código tarda %s y el nuevo %s", n, explicarTiempo(orig), explicarTiempo(nuevo))
	if rapido {
		return base + ".", true
	}
	if orig < explicarMicrosMedir {
		return base + ", demasiado poco para notar la diferencia.", false
	}
	return base + ", así que con este tamaño no he visto que vaya más rápido.", false
}

// compararReescritura runs the original and the rewrite of the changed function (x.fn) on the same
// generated cases. It returns a comprobación when they agree on all, the first difference otherwise, or
// why it could not compare.
func (x *explicacion) compararReescritura(nueva string) (*nucleo.Comprobacion, string, string) {
	k, c := x.k, x.c
	if x.fn.Firma == nil || !x.fn.Firma.Probable() {
		motivo := "no sé con qué datos llamar a `" + explicarNombreCorto(x.fn.Nombre) + "`"
		if x.fn.NoProbable != "" {
			motivo += ": " + x.fn.NoProbable
		}
		return nil, "", motivo
	}
	if c.ej == nil {
		return nil, "", "no tengo caja segura para ejecutar código"
	}
	f := *x.fn.Firma
	orig, err := explicarFuente(x.codigo, x.fn.Nombre)
	if err != nil {
		return nil, "", mensajeError(err)
	}
	nuevo, err := explicarFuente(nueva, x.fn.Nombre)
	if err != nil {
		return nil, "", mensajeError(err)
	}
	for _, src := range []string{orig, nuevo} {
		if v := c.ej.Revisar(src, nucleo.PermisoAuto); len(v) > 0 {
			return nil, "", "por seguridad no lo ejecuto sin tu permiso, " + v[0].Que
		}
	}
	n := k.raiz.Sub(nucleo.PasoComprobar, "Comparo la nueva `%s` con la tuya", explicarNombreCorto(x.fn.Nombre))
	b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{orig, nuevo}, Firma: f, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	if err != nil {
		n.Mal("")
		return nil, "", "no compila en la caja segura"
	}
	defer b.Cerrar()
	casos := c.generarCasos(k, f, nil, 0, 0)
	rs, err := c.probarCasos(k, n, b, casos, nucleo.OpcionesProbar{})
	if err != nil || len(rs) < 2 {
		n.Mal("")
		return nil, "", "no pude terminar las pruebas"
	}
	iguales := 0
	for i := range casos {
		if i >= len(rs[0]) || i >= len(rs[1]) {
			break
		}
		a, d := rs[0][i], rs[1][i]
		if explicarMismoResultado(a, d) {
			iguales++
			continue
		}
		dif := "con " + explicarEntradas(f, casos[i].Entradas) + " tu código da " + explicarResultado(f, a) + " y el nuevo da " + explicarResultado(f, d)
		n.Mal("No dan lo mismo: %s", dif)
		return nil, dif, ""
	}
	if iguales == 0 {
		n.Mal("")
		return nil, "", "no pude ejecutar las pruebas"
	}
	n.Bien("Dan lo mismo en %d pruebas", iguales)
	return &nucleo.Comprobacion{
		Nivel: nucleo.NivelEntendido, Pasan: iguales, Total: iguales,
		Texto:   fmt.Sprintf("Da lo mismo que tu código en %d pruebas", iguales),
		Detalle: fmt.Sprintf("función %s; semilla %d", x.fn.Nombre, k.semilla),
	}, "", ""
}

func explicarMismoResultado(a, b nucleo.ResultadoCaso) bool {
	fa := !a.Ejecutado || a.Panico != "" || a.Caida || a.Agotado || a.SinCombustible
	fb := !b.Ejecutado || b.Panico != "" || b.Caida || b.Agotado || b.SinCombustible
	if fa || fb {
		return fa == fb
	}
	if len(a.Obtenido) != len(b.Obtenido) {
		return false
	}
	for i := range a.Obtenido {
		if !nucleo.Igual(a.Obtenido[i], b.Obtenido[i]) {
			return false
		}
	}
	return true
}

func explicarResultado(f nucleo.Firma, r nucleo.ResultadoCaso) string {
	if r.Panico != "" || r.Caida {
		return "un error (" + recortar(explicarPanico(r.Panico), 100) + ")"
	}
	if !r.Ejecutado || r.Agotado || r.SinCombustible {
		return "nada (no terminó)"
	}
	return explicarSalidas(f, r.Obtenido)
}

// ---- reading the code's structure ----

// explicarFuente makes a buildable file with the target function and what it needs (snippets are
// normalized; main and unrelated code are left out).
func explicarFuente(codigo, nombre string) (string, error) {
	fuente, err := analisis.Normalizar(codigo, "")
	if err != nil {
		return "", err
	}
	if sola, err := analisis.Funcion(fuente, nombre); err == nil && strings.TrimSpace(sola) != "" {
		fuente = sola
	}
	return fuente, nil
}

// explicarParsear parses src as analisis does, keeping the user's line numbers: as a file, else with
// "package main;" in front, else with its statements wrapped in func main(). It returns the parsed
// text too (offsets refer to it), or a nil file when the code cannot be parsed.
func explicarParsear(src string) (*ast.File, *token.FileSet, string) {
	for _, t := range []string{src, "package main;" + src, "package main; func main() {" + src + "\n}"} {
		fset := token.NewFileSet()
		if f, err := parser.ParseFile(fset, "x.go", t, 0); err == nil {
			return f, fset, t
		}
	}
	return nil, nil, ""
}

// explicarNombreDecl names a function as analisis does: "F", or "Tipo.Metodo" for a method.
func explicarNombreDecl(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	for {
		switch e := t.(type) {
		case *ast.StarExpr:
			t = e.X
			continue
		case *ast.ParenExpr:
			t = e.X
			continue
		case *ast.IndexExpr:
			t = e.X
			continue
		case *ast.IndexListExpr:
			t = e.X
			continue
		case *ast.Ident:
			return e.Name + "." + fd.Name.Name
		}
		return "?." + fd.Name.Name
	}
}

func explicarBuscarFunc(f *ast.File, nombre string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name != nil && explicarNombreDecl(fd) == nombre {
			return fd
		}
	}
	return nil
}

// explicarMapaLineas maps the lines of the built file (fuente: normalized and gofmt'd, where the trace
// events happen) to the lines of the same statements in the user's code. Both trees of the function
// have the same shape, because gofmt only moves code between lines, so their nodes are paired in order.
// nil when the function cannot be paired (then no line is shown rather than a wrong one).
func explicarMapaLineas(codigo, fuente, nombre string) map[int]int {
	fu, fsu, _ := explicarParsear(codigo)
	fb, fsb, _ := explicarParsear(fuente)
	if fu == nil || fb == nil {
		return nil
	}
	du, db := explicarBuscarFunc(fu, nombre), explicarBuscarFunc(fb, nombre)
	if du == nil || db == nil {
		return nil
	}
	nu, nb := explicarNodos(du), explicarNodos(db)
	if len(nu) != len(nb) {
		return nil
	}
	m := map[int]int{}
	for i := range nb {
		lb := fsb.Position(nb[i].Pos()).Line
		if _, ok := m[lb]; !ok {
			m[lb] = fsu.Position(nu[i].Pos()).Line
		}
	}
	return m
}

// explicarNodos lists the syntax nodes of a function in visiting order (comments left out).
func explicarNodos(fd *ast.FuncDecl) []ast.Node {
	var out []ast.Node
	ast.Inspect(fd, func(n ast.Node) bool {
		switch n.(type) {
		case nil:
			return false
		case *ast.CommentGroup, *ast.Comment:
			return false
		}
		out = append(out, n)
		return true
	})
	return out
}

// explicarCambiadas lists the functions of despues whose code differs from antes (compared token by
// token, so layout and comments do not count), in the order of despues.
func explicarCambiadas(antes, despues string) []string {
	fa, fsa, ta := explicarParsear(antes)
	fd, fsd, td := explicarParsear(despues)
	if fa == nil || fd == nil {
		return nil
	}
	previas := map[string]string{}
	for _, d := range fa.Decls {
		if g, ok := d.(*ast.FuncDecl); ok && g.Name != nil {
			previas[explicarNombreDecl(g)] = explicarFichas(fsa, ta, g)
		}
	}
	var out []string
	for _, d := range fd.Decls {
		g, ok := d.(*ast.FuncDecl)
		if !ok || g.Name == nil {
			continue
		}
		nombre := explicarNombreDecl(g)
		if previa, ok := previas[nombre]; !ok || previa != explicarFichas(fsd, td, g) {
			out = append(out, nombre)
		}
	}
	return out
}

// explicarFichas is the token sequence of a node's source (no comments, no semicolons: they depend on
// the layout).
func explicarFichas(fset *token.FileSet, texto string, n ast.Node) string {
	ini, fin := fset.Position(n.Pos()).Offset, fset.Position(n.End()).Offset
	if ini < 0 || fin > len(texto) || ini > fin {
		return ""
	}
	src := []byte(texto[ini:fin])
	var s scanner.Scanner
	s.Init(token.NewFileSet().AddFile("", -1, len(src)), src, nil, 0)
	var sb strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON || tok == token.COMMENT {
			continue
		}
		sb.WriteString(tok.String())
		sb.WriteByte(' ')
		sb.WriteString(lit)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// ---- small helpers ----

// explicarCasoPequeno picks a small, readable input: a probe with 3 to 8 elements, else the smallest
// non-empty one, else the first probe.
func explicarCasoPequeno(c *Cerebro, k *tarea, f nucleo.Firma) (nucleo.Caso, bool) {
	cands := nucleo.Sondas(f)
	if len(cands) == 0 {
		cands = c.generarCasos(k, f, nil, 0, -1)
	}
	if len(cands) == 0 {
		return nucleo.Caso{}, false
	}
	mejor, tam := -1, 0
	for i, cs := range cands {
		t := 0
		for _, v := range cs.Entradas {
			t += explicarLargo(v)
		}
		if t >= 3 && t <= 8 {
			return cands[i], true
		}
		if t > 0 && (mejor < 0 || t < tam) {
			mejor, tam = i, t
		}
	}
	if mejor >= 0 {
		return cands[mejor], true
	}
	return cands[0], true
}

// explicarLargo is the number of elements of a value (1 for scalars, 0 for empty or nil).
func explicarLargo(v nucleo.Valor) int {
	switch x := v.(type) {
	case nil:
		return 0
	case string:
		return len([]rune(x))
	case []nucleo.Valor:
		n := 0
		for _, e := range x {
			n += explicarLargo(e)
		}
		if n == 0 {
			return len(x)
		}
		return n
	case nucleo.Mapa:
		return len(x)
	}
	return 1
}

// explicarEntradas: "nums = [1, 2, 3, 4]".
func explicarEntradas(f nucleo.Firma, vs []nucleo.Valor) string {
	ent := f.Entradas()
	var nombres []string
	if f.Receptor != nil {
		n := f.Receptor.Nombre
		if n == "" {
			n = "el receptor"
		}
		nombres = append(nombres, n)
	}
	for i, p := range f.Params {
		n := p.Nombre
		if n == "" {
			n = "la entrada " + strconv.Itoa(i+1)
		}
		nombres = append(nombres, n)
	}
	var partes []string
	for i, v := range vs {
		if i >= len(ent) || i >= len(nombres) {
			break
		}
		partes = append(partes, nombres[i]+" = "+nucleo.FormatoHumano(v, ent[i]))
	}
	return strings.Join(partes, ", ")
}

func explicarSalidas(f nucleo.Firma, vs []nucleo.Valor) string {
	var partes []string
	for i, v := range vs {
		if i < len(f.Res) {
			partes = append(partes, nucleo.FormatoHumano(v, f.Res[i]))
		}
	}
	return strings.Join(partes, ", ")
}

// explicarFraseTraza: "Lo ejecuté con nums = [1, 2, 3, 4]: `total` pasa por 0 → 2 → 6; devuelve 6."
func explicarFraseTraza(f nucleo.Firma, ent string, r nucleo.ResultadoCaso, roles map[string]string) string {
	seq := map[string][]string{}
	var orden []string
	for _, e := range r.Eventos {
		vs := seq[e.Var]
		if len(vs) == 0 {
			orden = append(orden, e.Var)
		}
		if len(vs) == 0 || vs[len(vs)-1] != e.Valor {
			seq[e.Var] = append(vs, e.Valor)
		}
	}
	importante := func(v string) bool {
		switch roles[v] {
		case "acumulador", "contador", "bandera", "resultado":
			return true
		}
		return false
	}
	sort.SliceStable(orden, func(i, j int) bool { return importante(orden[i]) && !importante(orden[j]) })
	var partes []string
	for _, v := range orden {
		if len(partes) == 3 {
			break
		}
		vs := seq[v]
		if len(vs) > 8 {
			vs = append(vs[:8:8], "…")
		}
		if len(vs) == 1 {
			partes = append(partes, "`"+v+"` vale "+vs[0])
		} else {
			partes = append(partes, "`"+v+"` pasa por "+strings.Join(vs, " → "))
		}
	}
	s := "Lo ejecuté con " + ent
	if len(partes) > 0 {
		s += ": " + strings.Join(partes, ", ")
	}
	switch {
	case r.Panico != "" || r.Caida:
		s += "; se para con un error (" + recortar(explicarPanico(r.Panico), 120) + ")"
	case len(r.Obtenido) == len(f.Res) && len(f.Res) > 0:
		s += "; devuelve " + explicarSalidas(f, r.Obtenido)
	}
	return s + "."
}

var (
	explicarOrdenRoles = []string{"acumulador", "contador", "bandera", "índice", "resultado", "auxiliar"}
	explicarRolUno     = map[string]string{"acumulador": "el acumulador", "contador": "un contador", "bandera": "una bandera",
		"índice": "un índice", "resultado": "el resultado", "auxiliar": "una variable de paso"}
	explicarRolVarios = map[string]string{"acumulador": "acumuladores", "contador": "contadores", "bandera": "banderas",
		"índice": "índices", "resultado": "resultados", "auxiliar": "variables de paso"}
	explicarRolPara = map[string]string{"acumulador": "ir juntando el resultado", "contador": "contar cuántas veces pasa algo",
		"bandera": "recordar si algo ha pasado (verdadero o falso)", "índice": "marcar la posición por la que va",
		"resultado": "guardar lo que se devuelve", "auxiliar": "guardar un valor un momento"}
)

func explicarPorRol(roles map[string]string) map[string][]string {
	por := map[string][]string{}
	for v, rol := range roles {
		por[rol] = append(por[rol], v)
	}
	for _, vs := range por {
		sort.Strings(vs)
	}
	return por
}

// explicarListaRoles: "total: acumulador, i: índice" (for the trace).
func explicarListaRoles(roles map[string]string) string {
	var partes []string
	por := explicarPorRol(roles)
	for _, rol := range explicarOrdenRoles {
		for _, v := range por[rol] {
			partes = append(partes, v+": "+rol)
		}
	}
	return strings.Join(partes, ", ")
}

// explicarFraseRoles: "Papel de cada variable: `total` es el acumulador (sirve para ir juntando el
// resultado); `i` y `j` son índices (sirven para marcar la posición por la que va)."
func explicarFraseRoles(roles map[string]string, detalle int) string {
	por := explicarPorRol(roles)
	var partes []string
	for _, rol := range explicarOrdenRoles {
		vs := por[rol]
		if len(vs) == 0 || rol == "auxiliar" && detalle != 2 {
			continue
		}
		nombres := make([]string, len(vs))
		for i, v := range vs {
			nombres[i] = "`" + v + "`"
		}
		var s string
		if len(vs) == 1 {
			s = nombres[0] + " es " + explicarRolUno[rol] + " (sirve para " + explicarRolPara[rol] + ")"
		} else {
			s = strings.Join(nombres[:len(nombres)-1], ", ") + " y " + nombres[len(nombres)-1] + " son " + explicarRolVarios[rol] +
				" (sirven para " + explicarRolPara[rol] + ")"
		}
		partes = append(partes, s)
	}
	if len(partes) == 0 {
		return ""
	}
	return "Papel de cada variable: " + strings.Join(partes, "; ") + "."
}

// explicarAvisos turns up to 3 static findings into one paragraph.
func explicarAvisos(hs []analisis.Hallazgo) string {
	var partes []string
	for _, h := range hs {
		if len(partes) == 3 {
			break
		}
		s := fmt.Sprintf("línea %d: %s", h.Linea, strings.TrimRight(h.Mensaje, ". "))
		if h.Consejo != "" {
			s += " (" + strings.TrimRight(h.Consejo, ". ") + ")"
		}
		partes = append(partes, s)
	}
	if len(partes) == 0 {
		return ""
	}
	return "Ojo, " + strings.Join(partes, "; ") + "."
}

// explicarClase names a static complexity in words.
func explicarClase(o string) string {
	switch strings.TrimSuffix(o, "?") {
	case "O(1)":
		return "constante"
	case "O(log n)":
		return "logarítmica"
	case "O(√n)":
		return "raíz cuadrada de n"
	case "O(n)":
		return "lineal"
	case "O(n log n)":
		return "n·log n"
	case "O(n²)":
		return "cuadrática"
	case "O(n³)":
		return "cúbica"
	case "O(2ⁿ)":
		return "exponencial"
	}
	grado, logs, exp, ok := explicarOrden(o)
	switch {
	case !ok:
		return ""
	case exp:
		return "exponencial"
	case grado != float64(int(grado)):
		return "entre " + explicarNombreGrado(int(grado)) + " y " + explicarNombreGrado(int(grado)+1)
	case logs > 0 && grado >= 1:
		return "algo más que " + explicarNombreGrado(int(grado))
	case logs > 0:
		return "logarítmica"
	}
	return explicarNombreGrado(int(grado))
}

func explicarNombreGrado(k int) string {
	switch k {
	case 0:
		return "constante"
	case 1:
		return "lineal"
	case 2:
		return "cuadrática"
	case 3:
		return "cúbica"
	}
	return "polinómica de grado " + strconv.Itoa(k)
}

// explicarOrden reads an estimate of analisis ("O(n² log n)", "O(n√n)", "O(log² n)", "O(2ⁿ)?") as its
// polynomial degree and its number of log factors; exp is true for an exponential estimate.
func explicarOrden(o string) (grado float64, logs int, exp, ok bool) {
	s := strings.TrimSuffix(strings.TrimSpace(o), "?")
	if !strings.HasPrefix(s, "O(") || !strings.HasSuffix(s, ")") {
		return 0, 0, false, false
	}
	s = strings.TrimSpace(s[2 : len(s)-1])
	switch {
	case s == "1":
		return 0, 0, false, true
	case strings.Contains(s, "ⁿ"):
		return 0, 0, true, true
	}
	for {
		i := strings.Index(s, "log")
		if i < 0 {
			break
		}
		resto := s[i+len("log"):]
		k, w := explicarSuperindice(resto)
		if k == 0 {
			k = 1
		}
		resto = strings.TrimSpace(resto[w:])
		if !strings.HasPrefix(resto, "n") {
			return 0, 0, false, false
		}
		logs += k
		s = strings.TrimSpace(s[:i] + resto[1:])
	}
	if strings.HasSuffix(s, "√n") {
		grado += 0.5
		s = strings.TrimSuffix(s, "√n")
	}
	if s != "" {
		if s[0] != 'n' {
			return 0, 0, false, false
		}
		k, w := explicarSuperindice(s[1:])
		if w != len(s)-1 {
			return 0, 0, false, false
		}
		if k == 0 {
			k = 1
		}
		grado += float64(k)
	}
	return grado, logs, false, true
}

// explicarSuperindice reads leading superscript digits ("²", "¹⁰"): their value and their byte width.
func explicarSuperindice(s string) (int, int) {
	const digitos = "⁰¹²³⁴⁵⁶⁷⁸⁹"
	v, w := 0, 0
	for _, r := range s {
		d := strings.IndexRune(digitos, r)
		if d < 0 {
			break
		}
		v = v*10 + utf8.RuneCountInString(digitos[:d])
		w += utf8.RuneLen(r)
	}
	return v, w
}

// explicarMaxN is the largest input size to measure for a static estimate. 0 means do not measure, and
// nota says why when the estimate is at least O(n) (below O(n) the growth is not measured, §4.14.2).
func explicarMaxN(o string) (int, string) {
	grado, _, exp, ok := explicarOrden(o)
	switch {
	case !ok:
		return 0, "no sé qué tamaños usar con esa estimación"
	case exp:
		return 0, "crece tan deprisa que con entradas grandes tardaría demasiado"
	case grado < 1:
		return 0, ""
	case grado < 2:
		return 100000, ""
	case grado < 3:
		return 10000, ""
	case grado < 4:
		return 1000, ""
	}
	return 0, "crece tan deprisa que con entradas grandes tardaría demasiado"
}

// explicarComparar places a measured class against the static estimate: 0 when the measured growth is
// what the estimate gives at the measured sizes, 1 when it grows faster, -1 when slower. The bands are
// those of analisis.ClasificarCrecimiento (slope of log t over log n).
func explicarComparar(o, medida string) int {
	bandas := map[string][2]float64{
		"constante": {0, 0.25}, "lineal": {0.25, 1.3}, "n·log n": {0.25, 1.3},
		"cuadrática": {1.3, 2.4}, "cúbica": {2.4, 3.4}, "explosiva": {3.4, 1e9},
	}
	b, ok := bandas[medida]
	grado, logs, exp, okO := explicarOrden(o)
	if !ok || !okO {
		return -1
	}
	if exp {
		grado = 1e8
	}
	g := grado + 0.1*float64(logs) // a log factor adds a little to the slope
	switch {
	case g < b[0]:
		return 1
	case g >= b[1]:
		return -1
	}
	return 0
}

// explicarCoinciden: the measured class agrees with the static one (lineal and n·log n are hard to
// tell apart by timing, so they count as agreeing).
func explicarCoinciden(estatica, medida string) bool {
	if estatica == medida {
		return true
	}
	casi := map[string]bool{"lineal": true, "n·log n": true}
	return casi[estatica] && casi[medida]
}

// explicarVeredicto says what the estimate means in practice.
func explicarVeredicto(o string) string {
	switch o {
	case "O(1)":
		return "Según esa estimación es muy eficiente: tarda lo mismo con entradas grandes o pequeñas."
	case "O(log n)", "O(√n)":
		return "Según esa estimación es eficiente: aunque la entrada crezca mucho, el tiempo crece poco."
	case "O(n)":
		return "Según esa estimación es eficiente: si la entrada es 10 veces más grande, tarda unas 10 veces más."
	case "O(n log n)":
		return "Según esa estimación es eficiente: si la entrada es 10 veces más grande, tarda algo más de 10 veces más."
	case "O(n²)":
		return "Según esa estimación no es muy eficiente con entradas grandes: si la entrada es 10 veces más grande, tarda unas 100 veces más."
	case "O(n³)":
		return "Según esa estimación es lenta con entradas grandes: si la entrada es 10 veces más grande, tarda unas 1000 veces más."
	case "O(2ⁿ)?", "O(2ⁿ)":
		return "Según esa estimación es muy lenta: cada elemento más puede doblar el tiempo."
	}
	return ""
}

func explicarNombreCorto(nombre string) string {
	if i := strings.LastIndexByte(nombre, '.'); i >= 0 {
		return nombre[i+1:]
	}
	return nombre
}

// explicarGenerador builds inputs of size n for Crecimiento: every list or text input gets n elements;
// with none, a single int input is n. Other inputs get fixed small values. nil: not possible.
func explicarGenerador(f nucleo.Firma, maxN int) func(n int) (nucleo.Caso, bool) {
	if f.Receptor != nil {
		return nil
	}
	ent := f.Entradas()
	crece := make([]bool, len(ent))
	hay := false
	for i, t := range ent {
		if t.Clase == nucleo.CString || t.Clase == nucleo.CLista && t.Elem != nil && explicarElemSimple(*t.Elem) {
			crece[i], hay = true, true
		}
	}
	if !hay {
		enteros := 0
		for i, t := range ent {
			if t.Clase == nucleo.CInt {
				enteros++
				crece[i] = true
			}
		}
		if enteros != 1 {
			return nil
		}
	}
	fijos := make([]nucleo.Valor, len(ent))
	for i, t := range ent {
		if crece[i] {
			continue
		}
		v, ok := explicarValorFijo(t)
		if !ok {
			return nil
		}
		fijos[i] = v
	}
	return func(n int) (nucleo.Caso, bool) {
		if n > maxN {
			return nucleo.Caso{}, false
		}
		vals := make([]nucleo.Valor, len(ent))
		for i, t := range ent {
			if crece[i] {
				vals[i] = explicarValorDeTamano(t, n)
			} else {
				vals[i] = nucleo.Copiar(fijos[i])
			}
		}
		return nucleo.Caso{Entradas: vals, Origen: nucleo.OrigenAzar, Nota: "tamaño " + strconv.Itoa(n)}, true
	}
}

func explicarElemSimple(t nucleo.Tipo) bool {
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat, nucleo.CString, nucleo.CRune, nucleo.CByte, nucleo.CBool:
		return true
	}
	return false
}

func explicarValorFijo(t nucleo.Tipo) (nucleo.Valor, bool) {
	switch t.Clase {
	case nucleo.CInt:
		return -1, true
	case nucleo.CFloat:
		return 0.5, true
	case nucleo.CString:
		return "z", true
	case nucleo.CBool:
		return false, true
	case nucleo.CRune, nucleo.CByte:
		return int('z'), true
	}
	return nil, false
}

// explicarValorDeTamano: distinct values, so loops that stop at a repeat or a match run to the end.
func explicarValorDeTamano(t nucleo.Tipo, n int) nucleo.Valor {
	switch t.Clase {
	case nucleo.CInt:
		return n
	case nucleo.CString:
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteByte(byte('a' + i%26))
		}
		return sb.String()
	case nucleo.CLista:
		out := make([]nucleo.Valor, n)
		for i := range out {
			switch t.Elem.Clase {
			case nucleo.CInt:
				out[i] = i
			case nucleo.CFloat:
				out[i] = float64(i) + 0.5
			case nucleo.CString:
				out[i] = "p" + strconv.Itoa(i)
			case nucleo.CRune, nucleo.CByte:
				out[i] = int('a' + i%26)
			case nucleo.CBool:
				out[i] = i%2 == 0
			}
		}
		return out
	}
	return nil
}
