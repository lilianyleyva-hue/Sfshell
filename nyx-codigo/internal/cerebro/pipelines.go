package cerebro

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/reparar"
	"nyxcodigo/internal/saber"
	"nyxcodigo/internal/sintesis"
)

// This file is the internal contract between the orchestrator and the code pipelines (INTERNO.md).
// The pipeline functions live in crear.go, programa.go, reparar.go, probar.go, ejecutar.go,
// explicar.go, docgo.go, buscar.go and estudiar.go; each owner replaces its file's function.

// pipeline answers one task. It must not call k.t.Terminar (Responder does) and must not panic on
// purpose; it returns an honest answer, never a claim it did not check.
type pipeline func(c *Cerebro, k *tarea) nucleo.Respuesta

// pipelines is the hook table. Tests may replace entries (c.pipes.crearFuncion = …).
type pipelines struct {
	crearFuncion  pipeline // ICrearFuncion; follow-ups modificar, otra_forma, esta_mal (code)
	crearPrograma pipeline // ICrearPrograma; follow-up programa_eso (k.envolver)
	reparar       pipeline // IReparar; Accion "arreglar"
	probar        pipeline // IProbar; Accion "probar"; follow-ups probar_con / vacia
	ejecutar      pipeline // IEjecutar; Accion "ejecutar"; follow-up ejecutar_eso
	explicar      pipeline // IExplicar, IOptimizar; follow-ups explicar_eso, comentarios, mas_simple/mas_detalle (code), mas_rapido
	docGo         pipeline // IDocGo
	buscar        pipeline // IBuscar
	estudiar      pipeline // IEstudiar; Accion "estudiar"
}

func pipelinesPorDefecto() pipelines {
	return pipelines{
		crearFuncion:  pipeCrearFuncion,
		crearPrograma: pipeCrearPrograma,
		reparar:       pipeReparar,
		probar:        pipeProbar,
		ejecutar:      pipeEjecutar,
		explicar:      pipeExplicar,
		docGo:         pipeDocGo,
		buscar:        pipeBuscar,
		estudiar:      pipeEstudiar,
	}
}

// tarea is everything a pipeline needs about the request being answered.
type tarea struct {
	ctx      context.Context // the budget (a pausable *reloj while a pipeline runs): use it for all work
	ctxPadre context.Context // the request: cancelled when the user presses Parar
	reloj    *reloj

	p     nucleo.Peticion
	q     *nucleo.Pregunta // the parsed request (never nil inside a pipeline)
	s     *sesion
	turno *Turno // the turn being built: pipelines fill Funcion, Expr, Excluidos, Metodo
	t     *nucleo.Traza
	raiz  *nucleo.Nodo // root node of the trace: add steps with k.raiz.Sub(…)
	lex   *lengua.Lexico

	intencion         nucleo.TipoIntencion
	estrategia        string // strategy counter name ("crear_funcion", "mates.ecuacion")
	presupuesto       time.Duration
	presupuestoPrevio time.Duration // budget of the run that "Pensar más" repeats
	pensarMas         bool          // "Pensar más": double budget, double synthesis Limite, web on
	semilla           int64         // seed for generated cases (shown in the evidence)
	inicio            time.Time

	codigo        string        // code to work on: the request's, or the session's
	codigoPrevio  bool          // codigo came from the session ("Uso el código de antes")
	excluir       []string      // DSL programs already shown (sintesis.Especificacion.Excluir)
	ejemplosExtra []nucleo.Caso // counterexample(s) just given with "está mal" (also stored in memoria)
	previo        *Turno        // the previous turn when this task is a follow-up, else nil
	seguimiento   *lengua.Seguimiento

	probarCon   string // probar_con: the literal input as typed ("[3,3]"); may not parse ("otros datos")
	vacia       bool   // "y si la lista está vacía": run with the empty value
	detalle     int    // narration level for explicar: 0 default, 1 más simple, 2 con detalle
	comentarios bool   // "ponle comentarios"
	optimizar   bool   // "hazlo más rápido" / IOptimizar (N3)
	envolver    bool   // "hazlo programa": wrap the session's function (k.codigo) into a program

	programa  *sintesis.Expr // DSL program of the answer, if any (learning: componente:/asoc: counters)
	pendiente string         // non-empty: the question is added to pendientes with this reason
	fallidas  []string       // strategies tried that failed (their counters go down)
	meta      bool           // the answer only talks about the previous turn

	numPreguntas atomic.Int32
}

func (c *Cerebro) nuevaTarea(ctx context.Context, p nucleo.Peticion, s *sesion, t *nucleo.Traza) *tarea {
	if p.Tarea == "" {
		p.Tarea = "t" + strconv.FormatInt(c.numTareas.Add(1), 10)
	}
	semilla := c.cfg.Semilla
	if semilla == 0 {
		semilla = time.Now().UnixNano()%999_983 + 1
	}
	return &tarea{
		ctx: ctx, ctxPadre: ctx, p: p, s: s, t: t, raiz: t.Raiz(), turno: &Turno{},
		semilla: semilla, inicio: c.ahora(), lex: c.lexico(),
	}
}

// cerrar stops the budget clock.
func (k *tarea) cerrar() {
	if k.reloj != nil {
		k.reloj.Cancelar()
	}
}

// pausar stops the budget while waiting for the user; call the result to resume.
func (k *tarea) pausar() func() {
	if k.reloj != nil {
		return k.reloj.Pausar()
	}
	return func() {}
}

// restante is the budget left (the whole budget when no clock runs yet).
func (k *tarea) restante() time.Duration {
	if k.reloj != nil {
		return k.reloj.Restante()
	}
	return k.presupuesto
}

// conPresupuesto runs f with a fresh budget clock for intent i.
func (c *Cerebro) conPresupuesto(k *tarea, i nucleo.TipoIntencion, f func() nucleo.Respuesta) nucleo.Respuesta {
	k.presupuesto = c.presupuestoTarea(k, i)
	if k.reloj != nil {
		k.reloj.Cancelar()
	}
	k.reloj = nuevoReloj(k.ctxPadre, k.presupuesto)
	k.ctx = k.reloj
	return f()
}

// despachar runs the pipeline (or template, or reasoning) of intent i for the task.
func (c *Cerebro) despachar(k *tarea, i nucleo.TipoIntencion) nucleo.Respuesta {
	k.intencion = i
	if k.estrategia == "" {
		k.estrategia = string(i)
	}
	if k.q == nil {
		ctx := k.s.contexto()
		k.q = lengua.Analizar(k.p.Texto, k.p.Codigo, k.p.Modo, k.lex, &ctx)
	}
	c.elegirCodigo(k, i)
	var pipe pipeline
	switch i {
	case nucleo.ICrearFuncion:
		pipe = c.pipes.crearFuncion
	case nucleo.ICrearPrograma:
		pipe = c.pipes.crearPrograma
	case nucleo.IReparar:
		pipe = c.pipes.reparar
	case nucleo.IProbar:
		pipe = c.pipes.probar
	case nucleo.IEjecutar:
		pipe = c.pipes.ejecutar
	case nucleo.IExplicar:
		pipe = c.pipes.explicar
	case nucleo.IOptimizar:
		k.optimizar = true
		pipe = c.pipes.explicar
	case nucleo.IDocGo:
		pipe = c.pipes.docGo
	case nucleo.IBuscar:
		pipe = c.pipes.buscar
	case nucleo.IEstudiar:
		pipe = c.pipes.estudiar
	case nucleo.IMemoria:
		pipe = (*Cerebro).plantillaMemoria
	case nucleo.ICharla:
		pipe = (*Cerebro).plantillaCharla
	default:
		return c.conPresupuesto(k, i, func() nucleo.Respuesta { return c.razonarIntencion(k, i) })
	}
	if pipe == nil {
		return c.enConstruccion(k, etiquetaIntencion(i))
	}
	return c.conPresupuesto(k, i, func() nucleo.Respuesta { return pipe(c, k) })
}

// elegirCodigo picks the code a code intent works on; with none in the request it reuses the
// session's last code and says so (§4.14.1 step 4).
func (c *Cerebro) elegirCodigo(k *tarea, i nucleo.TipoIntencion) {
	if k.codigo != "" {
		return
	}
	switch {
	case strings.TrimSpace(k.p.Codigo) != "":
		k.codigo = k.p.Codigo
	case k.q != nil && strings.TrimSpace(k.q.Codigo) != "":
		k.codigo = k.q.Codigo
	}
	if k.codigo != "" {
		return
	}
	switch i {
	case nucleo.IReparar, nucleo.IExplicar, nucleo.IProbar, nucleo.IEjecutar, nucleo.IOptimizar:
		if ctx := k.s.contexto(); ctx.Codigo != "" {
			k.codigo, k.codigoPrevio = ctx.Codigo, true
			k.raiz.Nota("Uso el código de antes")
		}
	}
}

// ---- sandbox helpers ----

// errSinArenero: there is no sandbox (no Go, or the app was built without one).
var errSinArenero = fmt.Errorf("no tengo caja segura para compilar: %w", nucleo.ErrSinGo)

// preparar builds the variants under nodo, with a trace step "Compilo …" that turns ✔ or ✘ (with the
// compiler errors in its detail). The caller must Cerrar the Binario.
func (c *Cerebro) preparar(k *tarea, nodo *nucleo.Nodo, p nucleo.Preparacion) (nucleo.Binario, nucleo.Compilacion, error) {
	if c.ej == nil {
		nodo.Sub(nucleo.PasoError, "No tengo caja segura para compilar").Mal("")
		return nil, nucleo.Compilacion{}, errSinArenero
	}
	titulo := "Compilo el código (go build)"
	if len(p.Variantes) > 1 {
		titulo = fmt.Sprintf("Compilo %d versiones a la vez (go build)", len(p.Variantes))
	}
	n := nodo.Sub(nucleo.PasoPrueba, "%s", titulo)
	b, comp, err := c.ej.Preparar(k.ctx, p)
	if err != nil {
		n.Mal("No compila: %s", resumenErrores(comp, err))
		if comp.Texto != "" {
			n.Detalle("%s", recortar(comp.Texto, 2000))
		}
		return nil, comp, err
	}
	n.Bien("Compila (%s)", milis(comp.Ms))
	if comp.Comando != "" {
		n.Detalle("%s", comp.Comando)
	}
	return b, comp, nil
}

// probarCasos runs the cases on a prepared binary with a trace step "Pruebo N casos".
func (c *Cerebro) probarCasos(k *tarea, nodo *nucleo.Nodo, b nucleo.Binario, casos []nucleo.Caso, op nucleo.OpcionesProbar) ([][]nucleo.ResultadoCaso, error) {
	n := nodo.Sub(nucleo.PasoPrueba, "Pruebo %d casos", len(casos))
	rs, err := b.Probar(k.ctx, casos, op)
	if err != nil {
		n.Mal("No pude terminar las pruebas: %s", mensajeError(err))
		return rs, err
	}
	if len(rs) > 0 {
		bien := 0
		for _, r := range rs[0] {
			if r.OK {
				bien++
			}
		}
		if bien == len(rs[0]) {
			n.Bien("%d/%d casos bien", bien, len(rs[0]))
		} else {
			n.Mal("%d/%d casos bien", bien, len(rs[0]))
		}
	}
	return rs, nil
}

// verificacion is the evidence for one candidate source.
type verificacion struct {
	Compilacion    nucleo.Compilacion
	Casos          []nucleo.Caso
	Resultados     []nucleo.ResultadoCaso // of the single variant
	Comprobaciones []nucleo.Comprobacion  // pruebas.Comprobaciones (one badge per group)
	Nivel          nucleo.Nivel           // pruebas.NivelGlobal
	Tabla          nucleo.Tabla           // pruebas.Tabla (Entrada | Esperado | Obtenido | ✔/✘ | Origen)
	Fallos         []int                  // indices of the failing cases
}

// verificar compiles fuente (PermisoAuto, InstrNormal) and runs the cases with the properties, under
// a "Compruebo" step. It returns the comprobaciones and the overall level (§4.14.3).
func (c *Cerebro) verificar(k *tarea, nodo *nucleo.Nodo, fuente string, f nucleo.Firma, casos []nucleo.Caso, props []nucleo.Propiedad, op nucleo.OpcionesProbar) (verificacion, error) {
	var v verificacion
	n := nodo.Sub(nucleo.PasoComprobar, "Compruebo")
	b, comp, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{fuente}, Firma: f, Props: props, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	v.Compilacion = comp
	if err != nil {
		n.Mal("No lo pude comprobar")
		return v, err
	}
	defer b.Cerrar()
	rs, err := c.probarCasos(k, n, b, casos, op)
	if err != nil || len(rs) == 0 {
		n.Mal("No lo pude comprobar")
		if err == nil {
			err = errors.New("el arenero no devolvió resultados")
		}
		return v, err
	}
	v.Casos, v.Resultados = casos, rs[0]
	v.Comprobaciones = pruebas.Comprobaciones(casos, rs[0], props, k.semilla)
	v.Nivel = pruebas.NivelGlobal(v.Comprobaciones)
	v.Tabla = pruebas.Tabla(f, casos, rs[0], 0)
	for i, r := range rs[0] {
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
		n.Mal("Compruebo: fallan %d de %d casos", len(v.Fallos), len(casos))
	}
	return v, nil
}

// generarCasos returns seeded boundary and random cases without expected values (EspNinguna).
// bordes or azar < 0 turns that part off; 0 takes pruebas' defaults (64 and 200).
func (c *Cerebro) generarCasos(k *tarea, f nucleo.Firma, ejemplos []nucleo.Caso, bordes, azar int) []nucleo.Caso {
	return pruebas.NuevoGenerador(k.semilla).Casos(f, pruebas.OpcionesCasos{Bordes: bordes, Azar: azar, Ejemplos: ejemplos})
}

// reparador is the repair engine wired as reparar's notes ask (sandbox, memoria counters, pruebas'
// shrinker and generator).
func (c *Cerebro) reparador(k *tarea) *reparar.Reparador {
	return &reparar.Reparador{
		Ej:      c.ej,
		Stats:   c.stats,
		Reducir: pruebas.Minimizar,
		Azar: func(f nucleo.Firma, n int) []nucleo.Caso {
			return pruebas.NuevoGenerador(k.semilla).Casos(f, pruebas.OpcionesCasos{Bordes: -1, Azar: n})
		},
	}
}

// registro is the synthesis registry with the learned primitives (one Biblioteca per process).
func (c *Cerebro) registro() *sintesis.Registro { return c.biblio.Registro() }

// indice is the GOROOT example index, or nil while it is being built.
func (c *Cerebro) indice() *saber.IndiceStd { return c.std.Load() }

// ---- memory helpers ----

// memoriaBuscar looks for a stored function (≤ 100 ms, a "Miro mi memoria" step).
func (c *Cerebro) memoriaBuscar(k *tarea, nodo *nucleo.Nodo, conceptos, palabras []string, forma string, max int) []memoria.Puntuada {
	n := nodo.Sub(nucleo.PasoRecuerdo, "Miro mi memoria")
	hs := c.mem.BuscarFunciones(conceptos, palabras, forma, max)
	if len(hs) == 0 || hs[0].Puntos <= 0 {
		n.Info("No la tengo en memoria")
		return nil
	}
	n.Info("La más parecida: %s (%s)", hs[0].F.Nombre, decimal(hs[0].Puntos))
	return hs
}

// contraejemplos returns the stored counterexamples for these concepts and signature, as EspUsuario cases.
func (c *Cerebro) contraejemplos(conceptos []string, f nucleo.Firma) []nucleo.Caso {
	return c.mem.Contraejemplos(memoria.ClaveContraejemplo(conceptos, f), f)
}

// guardarContraejemplo stores a counterexample ("está mal: con [2] debe dar 0") for a turn's function.
func (c *Cerebro) guardarContraejemplo(conceptos []string, f nucleo.Firma, caso nucleo.Caso) error {
	caso.Origen = nucleo.OrigenContraejemplo
	caso.Expectativa = nucleo.EspUsuario
	return c.mem.GuardarContraejemplo(memoria.ClaveContraejemplo(conceptos, f), f, caso)
}

// guardarFuncion stores a verified function (Nivel tus_ejemplos or entendido; web code with sin_fallos
// when web is true), teaches its DSL program (if any) to the synthesis library, records it in the turn
// and returns the sentence "Lo he guardado en mi memoria como X".
func (c *Cerebro) guardarFuncion(k *tarea, f nucleo.FuncionAprendida, programa *sintesis.Expr, web bool) (string, error) {
	switch {
	case f.Nivel == nucleo.NivelTusEjemplos || f.Nivel == nucleo.NivelEntendido:
	case web && f.Nivel == nucleo.NivelSinFallos:
	default:
		return "", fmt.Errorf("no la guardo: su nivel (%s) no basta", f.Nivel)
	}
	if f.Conceptos == nil && k.q != nil {
		f.Conceptos = append([]string(nil), k.q.Conceptos...)
	}
	if f.Palabras == nil && k.q != nil {
		f.Palabras = lemasDe(k.q)
	}
	ahora := c.ahora()
	if f.Creada.IsZero() {
		f.Creada = ahora
	}
	if f.Usada.IsZero() {
		f.Usada = ahora
	}
	if programa != nil && f.DSL == "" {
		f.DSL = programa.String()
	}
	nombre, err := c.mem.GuardarFuncion(f)
	if err != nil {
		k.raiz.Sub(nucleo.PasoMemoria, "No pude guardarla: %s", err).Mal("")
		return "", err
	}
	if programa != nil {
		if _, err := c.biblio.Aprender(nombre, f.Descripcion, programa, f.Firma); err != nil {
			k.raiz.Nota("No la añado como pieza para construir otras: %s", err)
		}
	}
	g := f
	g.Nombre = nombre
	k.turno.Funcion = &g
	if programa != nil {
		k.programa = programa
		k.turno.Expr = programa.String()
	}
	frase := "Lo he guardado en mi memoria como " + nombre
	k.raiz.Sub(nucleo.PasoMemoria, "%s", frase).Bien("")
	return frase, nil
}

// lemasDe returns the word lemmas of a request (FuncionAprendida.Palabras).
func lemasDe(q *nucleo.Pregunta) []string {
	var out []string
	visto := map[string]bool{}
	for _, t := range q.Tokens {
		if t.Clase != "palabra" || t.Lema == "" || visto[t.Lema] {
			continue
		}
		visto[t.Lema] = true
		out = append(out, t.Lema)
	}
	return out
}

// ---- questions ----

// preguntar asks with buttons (and free text when opciones is empty) and waits up to 5 minutes with the
// budget paused. ok=false: no answer; use your default.
func (c *Cerebro) preguntar(k *tarea, texto string, opciones []string) (string, bool) {
	return c.preguntarCon(k, nucleo.PreguntaUsuario{Texto: texto, Opciones: opciones, Libre: len(opciones) == 0}, nil)
}

// preguntarLibre asks with buttons plus a free-text field.
func (c *Cerebro) preguntarLibre(k *tarea, texto string, opciones []string, porDefecto string) (string, bool) {
	return c.preguntarCon(k, nucleo.PreguntaUsuario{Texto: texto, Opciones: opciones, Libre: true, PorDefecto: porDefecto}, nil)
}

// preguntarValor asks for a value of type tipo ("¿Qué debe dar SumaPares([-2])?"); the options are
// buttons and free text is allowed. A typed answer in the session that parses as tipo is accepted too.
// It returns the parsed value; ok=false when there is no answer or it does not parse ("Otra cosa"…).
func (c *Cerebro) preguntarValor(k *tarea, texto string, opciones []string, tipo nucleo.Tipo) (nucleo.Valor, string, bool) {
	tp := tipo
	s, ok := c.preguntarCon(k, nucleo.PreguntaUsuario{Texto: texto, Opciones: opciones, Libre: true}, &tp)
	if !ok {
		return nil, s, false
	}
	v, err := nucleo.ParseValor(s, tipo)
	if err != nil {
		return nil, s, false
	}
	return v, s, true
}

// ---- standard answers ----

// respuestaFallo is the standard honest failure (§0.9, §4.14.2 crear step 8): the closest attempt
// (mejor, may be ""), the minimized failing case sentence (contra, e.g. "Falla con [-1]: esperaba 0,
// da -1"), and the invitation to give examples. The question goes to pendientes with motivo.
func (c *Cerebro) respuestaFallo(k *tarea, motivo, mejor, contra string) nucleo.Respuesta {
	k.pendiente = motivo
	if k.pendiente == "" {
		k.pendiente = "fallo"
	}
	r := nucleo.Respuesta{
		Texto:          "No lo he conseguido.",
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelFallo,
		Exito:          false,
	}
	if motivo != "" {
		r.Parrafos = append(r.Parrafos, mayusculaInicial(motivo)+".")
	}
	if mejor != "" {
		r.Codigo = mejor
		r.Parrafos = append(r.Parrafos, "Esto es lo más cerca que llegué.")
	}
	if contra != "" {
		r.Parrafos = append(r.Parrafos, contra)
	}
	r.Parrafos = append(r.Parrafos, "Puedes darme más ejemplos (entrada -> salida) o decirlo de otra forma.")
	return r
}

// enConstruccion is the honest answer of a pipeline that is not built yet (never a fake success).
func (c *Cerebro) enConstruccion(k *tarea, que string) nucleo.Respuesta {
	k.raiz.Sub(nucleo.PasoNota, "Esta parte (%s) todavía está en construcción", que)
	return nucleo.Respuesta{
		Texto: "Todavía no sé hacer esto (en construcción).",
		Parrafos: []string{"Me falta la parte de «" + que + "». Mientras tanto puedo resolver cuentas, ecuaciones, " +
			"problemas de lógica, puzles y preguntas de contar."},
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "en_construccion",
		Exito:          false,
	}
}

// segun introduces web text: "Según go.dev: …" (§1 wording rules).
func segun(url, texto string) string {
	d := saber.Dominio(url)
	if d == "" {
		d = "la web"
	}
	return "Según " + d + ": " + texto
}

// ---- small text helpers ----

func resumenErrores(comp nucleo.Compilacion, err error) string {
	if len(comp.Errores) > 0 {
		e := comp.Errores[0]
		s := e.Msg
		if e.Linea > 0 {
			s = fmt.Sprintf("línea %d: %s", e.Linea, e.Msg)
		}
		if len(comp.Errores) > 1 {
			s += fmt.Sprintf(" (y %d más)", len(comp.Errores)-1)
		}
		return s
	}
	return mensajeError(err)
}

// mensajeError is a short Spanish text for an error (without the package prefixes).
func mensajeError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, nucleo.ErrSinTiempo):
		return "se acabó el tiempo"
	case errors.Is(err, context.Canceled):
		return "lo has parado"
	case errors.Is(err, nucleo.ErrSinGo):
		return "no encuentro Go instalado"
	case errors.Is(err, nucleo.ErrSinInternet):
		return "no hay internet"
	}
	s := err.Error()
	for _, pre := range []string{"nucleo: ", "logica: ", "mates: ", "puzles: ", "saber: ", "memoria: ", "cerebro: "} {
		s = strings.ReplaceAll(s, pre, "")
	}
	return s
}

func milis(ms int64) string {
	if ms < 1000 {
		return strconv.FormatInt(ms, 10) + " ms"
	}
	return decimal(float64(ms)/1000) + " s"
}

// decimal writes a number with a comma and at most 2 decimals ("0,82").
func decimal(x float64) string {
	s := strconv.FormatFloat(x, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return strings.ReplaceAll(s, ".", ",")
}

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func mayusculaInicial(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
