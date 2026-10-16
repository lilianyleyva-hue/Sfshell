package cerebro

import (
	"context"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/reparar"
	"nyxcodigo/internal/sintesis"
)

// explicar (§4.14.2): Narrar + Roles + Informe (OGrande, findings); when the target signature can be
// run automatically, a variable trace on a small input, Observar (summary checked on random inputs,
// fingerprint lookup) and, for loops over the input, the measured growth on an uninstrumented build.
// "ponle comentarios" → analisis.Comentar; IOptimizar / "hazlo más rápido" → analisis.Optimizaciones,
// shown only after comparing the rewrite with the original on generated cases.

// cabeceraFrases is the header of the per-line explanation table. UI contract (servidor/web/app.js,
// actualizarExplicacion): the first Tabla whose first header normalizes to "linea" feeds the editor
// gutter; its rows are [line number, sentence]. redactar keeps such tables whole.
var cabeceraFrases = []string{"Línea", "Frase"}

const (
	explicarMaxEventos    = 200
	explicarFilasTraza    = 40
	explicarAzar          = 200
	explicarLimiteResumen = 1500 * time.Millisecond
	explicarMinMedir      = 4 * time.Second // budget left needed to measure the growth
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
	x := &explicacion{c: c, k: k, codigo: codigo, inf: inf, fn: fn}
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

	roles map[string]string

	// dynamic evidence
	ejecutado   bool
	motivoNo    string // why it was not run ("" when it ran)
	fraseTraza  string
	tablas      []nucleo.Tabla
	resumen     string // verified summary ("devuelve la suma de …")
	pruebasRes  int
	igualA      string
	numSondas   int
	numAzar     int
	medida      string // measured growth class
	medidas     []analisis.Medida
	notaMedida  string
	comprobados []nucleo.Comprobacion
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
	case len(x.inf.Errores) > 0:
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
	if len(x.inf.Errores) > 0 {
		e := x.inf.Errores[0]
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("Ojo: así como está no compila (línea %d: %s), así que no lo he ejecutado.", e.Linea, e.Msg))
	} else if !x.ejecutado && x.motivoNo != "" && k.detalle != 1 {
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
	if x.ejecutado {
		r.Justificacion = append(r.Justificacion, fmt.Sprintf("Lo compilé en la caja segura y lo ejecuté con %d entradas fijas y %d al azar (semilla %d).", x.numSondas, x.numAzar, k.semilla))
	}
	if x.resumen != "" {
		r.Justificacion = append(r.Justificacion, fmt.Sprintf("Busqué un programa corto que diera lo mismo con las entradas fijas y lo comparé con tu código en %d entradas al azar: coincide en todas.", x.pruebasRes))
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
	fuente, delta, err := explicarFuente(x.codigo, x.fn.Nombre, x.fn.Desde)
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
	b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Instr: instr, Permiso: nucleo.PermisoAuto})
	if err != nil {
		x.motivoNo = "no compila en la caja segura (" + mensajeError(err) + ")"
		n.Mal("No lo pude ejecutar")
		return
	}
	defer b.Cerrar()
	x.ejecutado = true
	if k.detalle != 1 {
		x.trazar(n, b, f, delta)
	}
	x.observar(n, b, f)
	if k.detalle != 1 {
		x.medir(n, fuente, f)
	}
	n.Bien("")
}

// trazar follows the variables on one small input.
func (x *explicacion) trazar(n *nucleo.Nodo, b nucleo.Binario, f nucleo.Firma, delta int) {
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
	r0 := rs[0][0]
	tb := nucleo.Tabla{Titulo: "Así cambian las variables con " + ent, Cabecera: []string{"Paso", "Línea", "Variable", "Valor"}}
	for i, e := range r0.Eventos {
		if i >= explicarFilasTraza {
			break
		}
		linea := ""
		if l := e.Linea - delta; l >= x.fn.Desde && l <= x.fn.Hasta {
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
	x.numSondas = len(sondas)
	aleatorios := c.generarCasos(k, f, nil, -1, explicarAzar)
	x.numAzar = len(aleatorios)
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
	}
	x.igualA = comp.IgualA
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

// medir measures the growth when the static estimate is at least O(n) and an input can grow.
func (x *explicacion) medir(n *nucleo.Nodo, fuente string, f nucleo.Firma) {
	k, c := x.k, x.c
	maxN := explicarMaxN(x.fn.OGrande)
	if maxN == 0 {
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
	clase, ms, err := analisis.Crecimiento(k.ctx, b, gen, n)
	if err != nil {
		x.notaMedida = "no conseguí medidas suficientes"
		return
	}
	x.medida, x.medidas = clase, ms
}

// fraseComplejidad: "Estimé O(n²) (cuadrática) mirando el código: …; midiendo parece cuadrática."
func (x *explicacion) fraseComplejidad() string {
	o := x.fn.OGrande
	if o == "" {
		return ""
	}
	s := "Estimé " + o
	if cl := explicarClase(o); cl != "" {
		s += " (" + cl + ")"
	}
	s += " mirando el código"
	if x.fn.Motivo != "" {
		s += ": " + strings.TrimRight(x.fn.Motivo, ".")
	}
	if x.medida != "" {
		switch est := explicarClase(o); {
		case est == x.medida:
			s += "; midiendo parece " + x.medida
		case explicarCoinciden(est, x.medida):
			s += "; midiendo parece lineal o n·log n (al medir casi no se distinguen)"
		default:
			s += "; pero midiendo parece " + x.medida + ", así que no coinciden (puede que con mis entradas termine antes, o que el tiempo de arrancar tape la diferencia)"
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
	fraseO := x.fraseComplejidad()
	if len(rws) == 0 {
		no.Info("No encuentro ninguna")
		r := explicarRespuesta("No veo nada claro que pueda hacer más rápido sin cambiar lo que hace.", true)
		r.Nivel = ""
		if fraseO != "" {
			r.Parrafos = append(r.Parrafos, fraseO)
		}
		r.Sugerencias = []string{"Explícalo", "Hazle pruebas"}
		return r
	}
	rw := rws[0]
	no.Bien("Línea %d: %s", rw.Hallazgo.Linea, rw.Hallazgo.Mensaje)
	r := explicarRespuesta("", true)
	r.Nivel = ""
	r.Codigo = rw.Fuente
	r.Diff = reparar.Diff(x.codigo, rw.Fuente)
	cambio := strings.TrimRight(rw.Hallazgo.Mensaje, ".")
	if rw.Hallazgo.Consejo != "" {
		r.Parrafos = append(r.Parrafos, mayusculaInicial(strings.TrimRight(rw.Hallazgo.Consejo, "."))+".")
	}
	if len(rws) > 1 {
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("Hay %d cambios posibles más; pídemelo otra vez cuando apliques este.", len(rws)-1))
	}

	comp, diferencia, motivo := x.compararReescritura(rw.Fuente)
	switch {
	case comp != nil:
		r.Texto = "Lo hago más rápido: " + cambio + "."
		r.Comprobaciones = append(r.Comprobaciones, *comp)
		r.Parrafos = append([]string{fmt.Sprintf("Lo comparé con tu código en %d pruebas y da lo mismo.", comp.Total)}, r.Parrafos...)
	case diferencia != "":
		r.Texto = "Encontré un cambio posible (" + cambio + "), pero no te lo propongo: " + diferencia + "."
		r.Codigo, r.Diff = "", ""
		r.Exito = false
		r.Nivel = nucleo.NivelSinComprobar
	default:
		r.Texto = afirmar("esto va más rápido y hace lo mismo ("+cambio+"), pero no lo he podido comprobar: "+motivo+".", nil)
		r.Nivel = nucleo.NivelSinComprobar
	}
	if fraseO != "" {
		r.Parrafos = append(r.Parrafos, fraseO)
	}
	r.Justificacion = []string{"Busqué patrones lentos conocidos (concatenar textos en un bucle, buscar en una lista dentro de un bucle, append sin capacidad).",
		"Antes de proponer el cambio comparo el código nuevo con el tuyo en las mismas entradas."}
	r.Sugerencias = []string{"Explícalo", "Hazle pruebas"}
	k.turno.Metodo = "optimizar"
	return r
}

// compararReescritura runs the original and the rewrite on the same generated cases. It returns a
// comprobación when they agree on all, the first difference otherwise, or why it could not compare.
func (x *explicacion) compararReescritura(nueva string) (*nucleo.Comprobacion, string, string) {
	k, c := x.k, x.c
	if x.fn.Firma == nil || !x.fn.Firma.Probable() {
		return nil, "", "no sé con qué datos llamarla"
	}
	if c.ej == nil {
		return nil, "", "no tengo caja segura para ejecutar código"
	}
	f := *x.fn.Firma
	orig, _, err := explicarFuente(x.codigo, x.fn.Nombre, x.fn.Desde)
	if err != nil {
		return nil, "", mensajeError(err)
	}
	nuevo, _, err := explicarFuente(nueva, x.fn.Nombre, 0)
	if err != nil {
		return nil, "", mensajeError(err)
	}
	for _, src := range []string{orig, nuevo} {
		if v := c.ej.Revisar(src, nucleo.PermisoAuto); len(v) > 0 {
			return nil, "", "por seguridad no lo ejecuto sin tu permiso (" + v[0].Que + ")"
		}
	}
	n := k.raiz.Sub(nucleo.PasoComprobar, "Comparo el código nuevo con el tuyo")
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
		Detalle: fmt.Sprintf("semilla %d", k.semilla),
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
		return "un error (" + recortar(primeraLinea(r.Panico), 80) + ")"
	}
	if !r.Ejecutado || r.Agotado || r.SinCombustible {
		return "nada (no terminó)"
	}
	return explicarSalidas(f, r.Obtenido)
}

// ---- helpers ----

// explicarFuente makes a buildable file with the target function and what it needs (snippets are
// normalized; main and unrelated code are left out). delta is how many lines the function moved.
func explicarFuente(codigo, nombre string, desde int) (string, int, error) {
	fuente, err := analisis.Normalizar(codigo, "")
	if err != nil {
		return "", 0, err
	}
	if sola, err := analisis.Funcion(fuente, nombre); err == nil && strings.TrimSpace(sola) != "" {
		fuente = sola
	}
	delta := 0
	if desde > 0 {
		if inf, err := analisis.Analizar(fuente); err == nil {
			for _, g := range inf.Funciones {
				if g.Nombre == nombre {
					delta = g.Desde - desde
					break
				}
			}
		}
	}
	return fuente, delta, nil
}

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
		s += "; se para con un error: " + recortar(primeraLinea(r.Panico), 120)
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
	return ""
}

// explicarMaxN is the largest input size to measure for a static estimate (0: do not measure).
func explicarMaxN(o string) int {
	switch o {
	case "O(n)", "O(n log n)":
		return 100000
	case "O(n²)":
		return 10000
	case "O(n³)":
		return 1000
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

// explicarPideEficiencia: the request asks about speed ("¿es eficiente?", "¿cuánto tarda?").
func explicarPideEficiencia(k *tarea) bool {
	texto := k.p.Texto
	if k.q != nil && strings.TrimSpace(k.q.Resto) != "" {
		texto = k.q.Resto
	}
	t := nucleo.Normalizar(texto)
	for _, w := range []string{"eficien", "rapid", "lent", "complejidad", "cuanto tarda", "o grande", "big o", "coste", "costo", "rendimiento", "o(n"} {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
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
