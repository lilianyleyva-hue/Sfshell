package cerebro

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/reparar"
)

// pipeEjecutar is the pipeline to ejecutar código (ejecutar, §4.14.2, §7): code that passes the automatic
// safety check runs at once; otherwise the answer carries the risks (Respuesta.Riesgos) and nothing runs
// until the user confirms (p.Confirmado), grave violations never run, and web code never runs with more
// than PermisoAuto. A function without main is wrapped into a program (recetas.Envolver); "con entrada 5"
// becomes the stdin "5⏎". The confirmation dialog resends only the code (ejcRetomar): the request that
// asked for permission is taken back, so its input and its action (probar, reparar) are not lost.
func pipeEjecutar(c *Cerebro, k *tarea) nucleo.Respuesta {
	if t, ok := c.ejcRetomar(k); ok {
		que := strings.TrimSpace(primeraLinea(k.q.Resto))
		if que == "" {
			que = primeraLinea(t.Texto)
		}
		k.raiz.Sub(nucleo.PasoNota, "Me diste permiso para lo que me pediste: «%s»", recortar(que, 80)).Info("")
		if t.Intencion == nucleo.IProbar || t.Intencion == nucleo.IReparar {
			k.estrategia = string(t.Intencion)
			return c.despachar(k, t.Intencion)
		}
	}
	codigo := c.repCodigo(k, "Pega el código que quieres que ejecute")
	if codigo == "" {
		return repSinCodigo("ejecutarlo")
	}
	if c.ej == nil {
		k.raiz.Sub(nucleo.PasoNota, "No tengo caja segura para ejecutar código").Info("")
		return ejcRespuesta("No puedo ejecutarlo: no tengo caja segura (falta Go).", "")
	}
	src := repCompletar(codigo)
	// what can never run is said first (cgo does not even type-check, so this goes before the compiler)
	if graves := ejcGraves(c.ej.Revisar(src.fuente, nucleo.PermisoUsuario)); len(graves) > 0 {
		return ejcNuncaRespuesta(k, graves, "ejecutar")
	}
	if errs, _ := reparar.ChequeoTipos(src.fuente); len(errs) > 0 {
		n := k.raiz.Sub(nucleo.PasoError, "No compila: %d errores", len(errs))
		var errores []string
		for _, e := range errs {
			t := reparar.Traducir(src.linea(e))
			errores = append(errores, t)
			n.Detalle("%s", t)
		}
		n.Mal("")
		r := ejcRespuesta("No compila, así que no lo puedo ejecutar.", "")
		r.Parrafos = repParrafosCambios(errores, nil, nil)
		r.Sugerencias = []string{"Arréglalo"}
		return r
	}

	programa := src.fuente
	caso := nucleo.CasoPrograma{Entrada: k.p.Entrada, Args: k.p.Args}
	pedida := ejcEntradaDelTexto(k)
	envuelto := false
	if src.paquete != "main" || !reFuncMain.MatchString(src.fuente) {
		inf, _ := analisis.Analizar(src.fuente)
		fn, hay := c.repElegir(k, src.fuente, inf)
		if !hay || fn.Firma == nil {
			return ejcRespuesta("No hay nada que ejecutar: no tiene func main ni una función que yo sepa llamar.", "")
		}
		n := k.raiz.Sub(nucleo.PasoPlan, "No tiene func main: envuelvo «%s» en un programa que lee su entrada", fn.Nombre)
		f := *fn.Firma
		k.q = repConFirma(k.q, &f)
		sola, err := analisis.Funcion(src.fuente, fn.Nombre)
		if err != nil {
			sola = src.fuente
		}
		prog, aCaso, err := recetas.Envolver(sola, f)
		if err != nil {
			n.Mal("No sé convertirla en programa: %s", err)
			return ejcRespuesta("No sé convertir «"+fn.Nombre+"» en un programa: "+err.Error()+".", "")
		}
		n.Bien("")
		programa, envuelto = prog, true
		if caso.Entrada == "" && pedida == "" && len(f.Entradas()) > 0 {
			if resp, ok := c.preguntarLibre(k, "¿Con qué datos la ejecuto? Escribe la entrada de "+f.Go(), nil, ""); ok {
				pedida = strings.TrimSpace(resp)
			}
		}
		if caso.Entrada == "" && pedida != "" {
			if ent, ok := repLeerEntradas(pedida, f); ok {
				if cp, ok := aCaso(nucleo.Caso{Entradas: ent}); ok {
					caso.Entrada = cp.Entrada
				}
			}
			if caso.Entrada == "" {
				caso.Entrada = pedida + "\n"
			}
		}
		if caso.Entrada == "" && len(f.Entradas()) > 0 {
			// the wrapper reads every parameter from stdin: with nothing to read it can only fail
			k.raiz.Sub(nucleo.PasoNota, "No sé con qué datos ejecutarla: no la ejecuto").Info("")
			r := ejcRespuesta("Para ejecutar «"+fn.Nombre+"» necesito sus datos: dime, por ejemplo, «ejecútala con …» con la entrada de "+f.Go()+".", programa)
			r.Parrafos = append(r.Parrafos, "Te dejo el programa que la envuelve: guárdalo como `main.go`, escribe `go run main.go` y teclea la entrada.")
			return r
		}
	} else if caso.Entrada == "" && pedida != "" {
		caso.Entrada = pedida + "\n"
	}

	permiso := nucleo.PermisoAuto
	if v := c.ej.Revisar(programa, nucleo.PermisoAuto); len(v) > 0 {
		r, ok := c.ejcPermiso(k, programa, v, "ejecutarlo")
		if !ok {
			if len(r.Riesgos) > 0 {
				// the dialog resends this code with confirmado:true (the chat has no other copy of it);
				// the turn keeps the stdin worked out here, which ejcRetomar gives back to the confirmed run
				r.Codigo = programa
				if k.p.Entrada == "" {
					k.p.Entrada = caso.Entrada
				}
			} else if envuelto {
				r.Codigo = programa
			}
			return r
		}
		permiso = nucleo.PermisoUsuario
	}

	ej, err := c.repCorrerPrograma(k, k.raiz, programa, caso, permiso)
	if err != nil {
		return ejcRespuesta("No pude ejecutarlo: "+mensajeError(err)+".", ejcCodigo(envuelto, programa))
	}
	cp, frase := repResultadoPrograma(ej, caso.Entrada)
	r := ejcRespuesta(primeraLinea(frase), ejcCodigo(envuelto, programa))
	if resto := strings.TrimSpace(strings.TrimPrefix(frase, primeraLinea(frase))); resto != "" {
		r.Parrafos = append(r.Parrafos, resto)
	}
	if cp != nil {
		r.Exito = true
		r.Comprobaciones = []nucleo.Comprobacion{*cp}
		r.Nivel = cp.Nivel
	} else {
		texto := "Lo ejecuté y terminó con un error"
		if caso.Entrada == "" {
			texto = "Lo ejecuté (sin entrada) y terminó con un error"
		}
		r.Comprobaciones = []nucleo.Comprobacion{{Nivel: nucleo.NivelFallo, Pasan: 0, Total: 1, Texto: texto, Comando: ej.Comando}}
		r.Nivel = nucleo.NivelFallo
		r.Sugerencias = []string{"Arréglalo"}
		if caso.Entrada == "" && !envuelto && ejcLeeEntrada(programa) {
			r.Parrafos = append(r.Parrafos, "Ojo: el programa lee datos y no me diste ninguno. Dime, por ejemplo, «ejecútalo con entrada 5».")
		}
	}
	if envuelto {
		r.Parrafos = append(r.Parrafos, "Lo envolví en un programa que lee la entrada. Guárdalo como `main.go` y escribe `go run main.go`.")
	}
	return r
}

func ejcRespuesta(texto, codigo string) nucleo.Respuesta {
	return nucleo.Respuesta{
		Texto:          texto,
		Codigo:         codigo,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "ejecutar",
	}
}

// ejcCodigo: the wrapped program is shown (it is what ran); the user's own code stays in the session.
func ejcCodigo(envuelto bool, programa string) string {
	if envuelto {
		return programa
	}
	return ""
}

func ejcRiesgo(v nucleo.Violacion) string {
	if v.Linea > 0 {
		return "línea " + strconv.Itoa(v.Linea) + ": " + v.Que
	}
	return v.Que
}

func ejcGraves(vs []nucleo.Violacion) []nucleo.Violacion {
	var out []nucleo.Violacion
	for _, v := range vs {
		if v.Grave {
			out = append(out, v)
		}
	}
	return out
}

// ejcNuncaRespuesta: code with a grave violation (cgo, //go:linkname, a non-stdlib import) is never run,
// with or without permission, so no dialog is offered (no Riesgos).
func ejcNuncaRespuesta(k *tarea, graves []nucleo.Violacion, verbo string) nucleo.Respuesta {
	n := k.raiz.Sub(nucleo.PasoNota, "Esto no lo ejecuto nunca: %s", graves[0].Que)
	for _, x := range graves {
		n.Detalle("%s", ejcRiesgo(x))
	}
	n.Mal("")
	r := ejcRespuesta("No lo puedo "+verbo+": "+graves[0].Que+".", "")
	for _, x := range graves[1:] {
		r.Parrafos = append(r.Parrafos, mayusculaInicial(ejcRiesgo(x))+".")
	}
	r.Parrafos = append(r.Parrafos, "Esto no lo ejecuto ni con tu permiso: ni siquiera dentro de la caja segura es seguro.")
	return r
}

// ejcPermiso decides what happens to code that does not pass the automatic check v (§4.14.2 ejecutar
// step 2, §7). ok=true: the user confirmed and it may run with PermisoUsuario (a trace step says so).
// Otherwise the answer says why it does not run: grave violations never run, web code never gets more
// than PermisoAuto, and anything else needs the confirmation dialog (the answer carries Riesgos). para is
// the infinitive with "lo" ("ejecutarlo", "probarlo"). The caller sets Codigo and Metodo.
func (c *Cerebro) ejcPermiso(k *tarea, programa string, v []nucleo.Violacion, para string) (nucleo.Respuesta, bool) {
	verbo := strings.TrimSuffix(para, "lo")
	if graves := ejcGraves(append(append([]nucleo.Violacion(nil), v...), c.ej.Revisar(programa, nucleo.PermisoUsuario)...)); len(graves) > 0 {
		return ejcNuncaRespuesta(k, graves, verbo), false
	}
	if c.ejcDeLaWeb(k, programa) || c.ejcDeLaWeb(k, k.codigo) {
		n := k.raiz.Sub(nucleo.PasoNota, "Es código de internet: solo lo ejecuto con las reglas automáticas")
		for _, x := range v {
			n.Detalle("%s", ejcRiesgo(x))
		}
		n.Mal("")
		r := ejcRespuesta("No lo puedo "+verbo+": es código de internet y solo lo ejecuto con las reglas automáticas, que no cumple.", "")
		for _, x := range v {
			r.Parrafos = append(r.Parrafos, mayusculaInicial(ejcRiesgo(x))+".")
		}
		r.Parrafos = append(r.Parrafos, "Al código que viene de internet no le doy más permisos, ni aunque me lo pidas.")
		return r, false
	}
	if !k.p.Confirmado {
		n := k.raiz.Sub(nucleo.PasoPregunta, "No lo ejecuto sin tu permiso")
		for _, x := range v {
			n.Detalle("%s", ejcRiesgo(x))
		}
		n.Info("")
		r := ejcRespuesta("Antes de "+para+" necesito tu permiso:", "")
		for _, x := range v {
			r.Parrafos = append(r.Parrafos, mayusculaInicial(ejcRiesgo(x))+".")
		}
		r.Parrafos = append(r.Parrafos, "Si lo confirmas, lo ejecuto en la caja segura, sin red y con límites de tiempo y memoria.")
		r.Riesgos = v
		return r, false
	}
	k.raiz.Sub(nucleo.PasoNota, "Me diste permiso: lo ejecuto en la caja segura").Info("")
	return nucleo.Respuesta{}, true
}

// ventanaRetomar is how long a request that stopped for the confirmation dialog can be taken back.
const ventanaRetomar = 10 * time.Minute

// ejcRetomar handles the confirmation dialog's resend. The UI (app.js) resends only the code, as
// Accion "ejecutar" with no text and confirmado:true, whatever was asked. When the session's last turn
// stopped to ask for permission (it carries Riesgos) for this same code, the confirmation is for that
// request: its question, text, stdin and args are put back in k (so "con entrada 5" and the examples of
// "hazle pruebas: 3 -> 3" are kept) and its turn is returned; the caller sends a probar or reparar
// request back to its own pipeline. Every check still runs again (ejcPermiso: web code, grave
// violations), so nothing gets more permission than a direct confirmed request would.
func (c *Cerebro) ejcRetomar(k *tarea) (Turno, bool) {
	if k.s == nil || !k.p.Confirmado || k.p.Accion != "ejecutar" || strings.TrimSpace(k.p.Texto) != "" ||
		k.codigoPrevio || strings.TrimSpace(k.codigo) == "" {
		return Turno{}, false
	}
	t, ok := k.s.ultimo()
	if !ok || len(t.Respuesta.Riesgos) == 0 || t.Peticion.Confirmado {
		return Turno{}, false
	}
	switch t.Intencion {
	case nucleo.IEjecutar, nucleo.IProbar, nucleo.IReparar:
	default:
		return Turno{}, false
	}
	if c.ahora != nil && !t.Fecha.IsZero() && c.ahora().Sub(t.Fecha) > ventanaRetomar {
		return Turno{}, false
	}
	yo := ejcCompactar(k.codigo)
	mismo := false
	for _, s := range []string{t.Respuesta.Codigo, t.Peticion.Codigo, t.Codigo} {
		if s = ejcCompactar(s); s != "" && s == yo {
			mismo = true
		}
	}
	if !mismo {
		return Turno{}, false
	}
	if t.Pregunta != nil {
		q := *t.Pregunta
		k.q = &q
	}
	k.p.Texto = t.Peticion.Texto
	k.p.Accion = t.Peticion.Accion // the turn is recorded as the request it answers ("Pensar más" repeats that)
	if k.p.Entrada == "" {
		k.p.Entrada = t.Peticion.Entrada
	}
	if len(k.p.Args) == 0 {
		k.p.Args = append([]string(nil), t.Peticion.Args...)
	}
	return t, true
}

// ejcDeLaWeb: the code came from the web (§7.1: it never runs with more than PermisoAuto). A session turn
// is web code when its method or its function's origin is "web", or when it worked on the session's code
// while that code was web code. The code is web code when it was taken from the session while the
// session's code was web code, or when it is (or contains, or is part of) the code of a web turn.
func (c *Cerebro) ejcDeLaWeb(k *tarea, codigo string) bool {
	if k.s == nil {
		return false
	}
	k.s.mu.Lock()
	turnos := append([]Turno(nil), k.s.turnos...)
	k.s.mu.Unlock()
	var web []string
	actualWeb := false // the session's current code (nucleo.Contexto.Codigo) is web code
	for _, t := range turnos {
		if t.Meta || strings.TrimSpace(t.Codigo) == "" {
			continue
		}
		esWeb := t.Metodo == "web" || t.Respuesta.Metodo == "web" || (t.Funcion != nil && t.Funcion.Origen == "web")
		if !esWeb && actualWeb && ejcUsaCodigoDeSesion(t) {
			esWeb = true
		}
		suyo := ejcCompactar(t.Codigo)
		for _, w := range web {
			if !esWeb && ejcMismoCodigo(suyo, w) {
				esWeb = true // web code brought back (pasted, or resent by the dialog)
			}
		}
		actualWeb = esWeb
		if esWeb {
			web = append(web, suyo)
		}
	}
	if k.codigoPrevio && actualWeb {
		return true
	}
	yo := ejcCompactar(codigo)
	for _, w := range web {
		if ejcMismoCodigo(yo, w) {
			return true
		}
	}
	return false
}

// ejcUsaCodigoDeSesion: the turn brought no code of its own and worked on the session's code.
func ejcUsaCodigoDeSesion(t Turno) bool {
	if strings.TrimSpace(t.Peticion.Codigo) != "" || (t.Pregunta != nil && strings.TrimSpace(t.Pregunta.Codigo) != "") {
		return false
	}
	if t.Seguimiento != "" {
		return true
	}
	switch t.Intencion {
	case nucleo.IReparar, nucleo.IExplicar, nucleo.IProbar, nucleo.IEjecutar, nucleo.IOptimizar:
		return true
	}
	return false
}

// ejcCompactar drops every blank, so formatting does not hide that two pieces of code are the same.
func ejcCompactar(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func ejcMismoCodigo(a, b string) bool {
	const minimo = 30 // shorter pieces are too common to say where they came from
	switch {
	case a == "" || b == "":
		return false
	case a == b:
		return true
	case len(b) >= minimo && strings.Contains(a, b):
		return true
	case len(a) >= minimo && strings.Contains(b, a):
		return true
	}
	return false
}

var reLeeEntrada = regexp.MustCompile(`\bos\.Stdin\b|\bfmt\.Scan|\bfmt\.Fscan|\bbufio\.NewScanner\b|\bbufio\.NewReader\b`)

// ejcLeeEntrada: the program reads standard input.
func ejcLeeEntrada(fuente string) bool {
	return reLeeEntrada.MatchString(fuente)
}

var (
	reConEntrada = regexp.MustCompile(`(?i)\bcon\s+(?:la\s+|esta\s+)?entrada\s*:?\s*(.+?)\s*$`)
	reConDatos   = regexp.MustCompile(`(?i)\bcon\s+(?:los\s+datos\s+|el\s+valor\s+)?(.+?)\s*$`)
)

// ejcEntradaDelTexto reads "con entrada 5" (or "con [3,1,2]") from the request text.
func ejcEntradaDelTexto(k *tarea) string {
	return ejcLeerEntrada(k, true)
}

// ejcEntradaExplicita reads only "con entrada 5" (for reparar, where "con [3,1,2] da 1" is a report).
func ejcEntradaExplicita(k *tarea) string {
	return ejcLeerEntrada(k, false)
}

func ejcLeerEntrada(k *tarea, datos bool) string {
	texto := ""
	if k.q != nil {
		texto = k.q.Resto
	}
	if strings.TrimSpace(texto) == "" {
		texto = k.p.Texto
	}
	texto = primeraLinea(texto)
	limpiar := func(s string) string {
		s = strings.TrimSpace(strings.TrimRight(s, ":.;"))
		s = strings.Trim(s, "«»“”")
		return strings.ReplaceAll(s, "⏎", "\n")
	}
	if m := reConEntrada.FindStringSubmatch(texto); m != nil {
		return limpiar(m[1])
	}
	if !datos {
		return ""
	}
	if m := reConDatos.FindStringSubmatch(texto); m != nil {
		s := limpiar(m[1])
		if s != "" && strings.ContainsAny(s[:1], "[(\"'0123456789-") {
			return s
		}
	}
	return ""
}
