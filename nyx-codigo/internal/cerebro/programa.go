package cerebro

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/saber"
)

// crear_programa (§4.14.2): a shipped template filled from the text, else the function for the core
// action wrapped by recetas.Envolver, else program candidates from the web. Every program is built,
// vetted and run before it is shown, and the answer always says how to run it.

// FraseGoRun closes every program answer.
const FraseGoRun = "Guárdalo como `main.go` y escribe `go run main.go`."

const (
	programaUmbralPlantilla = 0.5
	programaMaxCasos        = 12
	programaMaxSalida       = 400
)

// programaOperaciones maps the frame's action to the operacion slot of the templates.
var programaOperaciones = map[string]string{
	"sumar": "suma", "restar": "resta", "multiplicar": "multiplicación", "dividir": "división",
}

// pipeCrearPrograma is the pipeline to crear programas enteros (crear_programa, §4.14.2).
func pipeCrearPrograma(c *Cerebro, k *tarea) nucleo.Respuesta {
	if k.envolver && strings.TrimSpace(k.codigo) != "" {
		return programaDeCodigo(c, k)
	}
	k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) buscar una plantilla de programa 2) si no, construir la función y envolverla 3) ejecutarlo para comprobarlo").Info("")
	if r, ok := programaPlantilla(c, k); ok {
		return r
	}
	q := k.q
	var fallo *nucleo.Respuesta
	if q.Firma != nil || len(q.Ejemplos) > 0 || q.Marco != nil && q.Marco.Accion != "" {
		k.raiz.Sub(nucleo.PasoPlan, "Construyo la función que hace el trabajo y luego la envuelvo en un programa").Info("")
		hecho := crearFuncion(c, k)
		if hecho.sinComprobar {
			if fuente, _, err := recetas.Envolver(hecho.codigo, hecho.firma); err == nil {
				return programaSinComprobar(k, fuente, "lee los datos, llama a "+hecho.firma.Nombre+" y escribe el resultado")
			}
		}
		if hecho.ok {
			if r, ok := programaEnvolver(c, k, hecho); ok {
				return r
			}
		} else {
			r := hecho.resp
			fallo = &r
		}
	}
	if r, ok := programaWeb(c, k); ok {
		return r
	}
	if fallo != nil {
		return *fallo
	}
	r := c.respuestaFallo(k, "no tengo una plantilla para este programa y no supe construirlo", "", "")
	r.Metodo = "plantilla"
	return r
}

// ---- runs ----

// programaRun is one run of a program case.
type programaRun struct {
	caso nucleo.CasoPrograma
	ej   nucleo.Ejecucion
	err  error
	ok   bool
}

// programaPrueba is the evidence for one program.
type programaPrueba struct {
	comp  nucleo.Compilacion
	vet   nucleo.Compilacion
	runs  []programaRun
	comps []nucleo.Comprobacion
	tabla nucleo.Tabla
	malos int
}

// programaProbar vets the program, builds it once (PermisoAuto, InstrNormal) and runs every case.
func programaProbar(c *Cerebro, k *tarea, fuente string, casos []nucleo.CasoPrograma) (programaPrueba, error) {
	var pp programaPrueba
	n := k.raiz.Sub(nucleo.PasoComprobar, "Compruebo el programa")
	if c.ej == nil {
		n.Mal("No tengo caja segura para ejecutarlo")
		return pp, errSinArenero
	}
	if viol := c.ej.Revisar(fuente, nucleo.PermisoAuto); len(viol) > 0 {
		n.Mal("No lo ejecuto: %s", viol[0].Que)
		return pp, nucleo.ErrInseguro
	}
	b := n.Sub(nucleo.PasoPrueba, "Compilo el programa (go build)")
	p, comp, err := c.ej.PrepararPrograma(k.ctx, fuente, nucleo.InstrNormal, nucleo.PermisoAuto)
	pp.comp = comp
	if err != nil {
		b.Mal("No compila: %s", resumenErrores(comp, err))
		if comp.Texto != "" {
			b.Detalle("%s", recortar(comp.Texto, 2000))
		}
		n.Mal("No lo pude comprobar")
		return pp, err
	}
	defer p.Cerrar()
	b.Bien("Compila (%s)", milis(comp.Ms))
	pp.vet = c.ej.Vet(k.ctx, fuente)
	if pp.vet.OK {
		n.Sub(nucleo.PasoComprobar, "go vet no se queja").Bien("")
	} else if len(pp.vet.Errores) > 0 {
		n.Sub(nucleo.PasoComprobar, "go vet avisa: %s", resumenErrores(pp.vet, nil)).Mal("")
	}
	r := n.Sub(nucleo.PasoPrueba, "Lo ejecuto con %d entradas", len(casos))
	for i, cs := range casos {
		r.Progreso("%d/%d", i+1, len(casos))
		ej, err := p.Correr(k.ctx, cs)
		run := programaRun{caso: cs, ej: ej, err: err}
		limpio := err == nil && !ej.Agotado && ej.Senal == "" && !ej.SinCombustible
		if cs.Expectativa == nucleo.EspNinguna {
			run.ok = limpio && ej.Codigo == 0
		} else {
			run.ok = limpio && pruebas.CompararSalida(ej.Salida, cs)
		}
		if !run.ok {
			pp.malos++
		}
		pp.runs = append(pp.runs, run)
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			break
		}
	}
	if pp.malos == 0 {
		r.Bien("%d/%d bien", len(pp.runs), len(pp.runs))
	} else {
		r.Mal("%d/%d bien", len(pp.runs)-pp.malos, len(pp.runs))
	}
	pp.comps = programaComprobaciones(pp.runs, k.semilla)
	pp.tabla = programaTabla(pp.runs)
	if pp.malos == 0 {
		n.Bien("")
	} else {
		n.Mal("Falla con %d de %d entradas", pp.malos, len(pp.runs))
	}
	return pp, nil
}

// programaComprobaciones: one badge per kind of expectation (tus ejemplos / lo ejecuté con N entradas /
// sin errores).
func programaComprobaciones(runs []programaRun, semilla int64) []nucleo.Comprobacion {
	var usuario, ref, ninguna [2]int
	for _, r := range runs {
		var g *[2]int
		switch r.caso.Expectativa {
		case nucleo.EspUsuario:
			g = &usuario
		case nucleo.EspNinguna:
			g = &ninguna
		default:
			g = &ref
		}
		g[1]++
		if r.ok {
			g[0]++
		}
	}
	detalle := fmt.Sprintf("%d ejecuciones del programa compilado", len(runs))
	if len(runs) == 1 {
		detalle = "1 ejecución del programa compilado"
	}
	var out []nucleo.Comprobacion
	nueva := func(n nucleo.Nivel, g [2]int, bien, mal string) {
		if g[1] == 0 {
			return
		}
		texto := bien
		if g[0] < g[1] {
			n = nucleo.NivelFallo
			texto = mal
		}
		out = append(out, nucleo.Comprobacion{Nivel: n, Pasan: g[0], Total: g[1], Texto: texto, Detalle: detalle})
	}
	tus := fmt.Sprintf("Cumple tus %d ejemplos", usuario[1])
	if usuario[1] == 1 {
		tus = "Cumple tu ejemplo"
	}
	nueva(nucleo.NivelTusEjemplos, usuario, tus, fmt.Sprintf("Falla %d de tus %d ejemplos", usuario[1]-usuario[0], usuario[1]))
	lo := fmt.Sprintf("Lo ejecuté con %d entradas", ref[1])
	if ref[1] == 1 {
		lo = "Lo ejecuté con 1 entrada"
	}
	nueva(nucleo.NivelEntendido, ref, lo, fmt.Sprintf("Falla con %d de %d entradas", ref[1]-ref[0], ref[1]))
	nueva(nucleo.NivelSinFallos, ninguna, fmt.Sprintf("Lo ejecuté %d veces sin errores", ninguna[1]),
		fmt.Sprintf("Se cae en %d de %d ejecuciones", ninguna[1]-ninguna[0], ninguna[1]))
	return out
}

func programaTabla(runs []programaRun) nucleo.Tabla {
	tb := nucleo.Tabla{Titulo: "Ejecuciones", Cabecera: []string{"Entrada", "Esperado", "Obtenido", "✔/✘", "Origen"}}
	for _, r := range runs {
		esperado := r.caso.Esperado
		switch {
		case r.caso.Expectativa == nucleo.EspNinguna:
			esperado = "(que no falle)"
		case r.caso.Comparar == "contiene":
			esperado = "contiene «" + esperado + "»"
		}
		obtenido := r.ej.Salida
		if r.err != nil {
			obtenido = "error: " + mensajeError(r.err)
		} else if r.ej.Agotado {
			obtenido += " (no terminó a tiempo)"
		}
		marca, m := "✔", "bien"
		if !r.ok {
			marca, m = "✘", "mal"
		}
		origen := string(r.caso.Expectativa)
		if r.caso.Nota != "" {
			origen = r.caso.Nota
		}
		tb.Filas = append(tb.Filas, []string{programaEntrada(r.caso), recortar(strings.TrimRight(esperado, "\n"), programaMaxSalida),
			recortar(strings.TrimRight(obtenido, "\n"), programaMaxSalida), marca, origen})
		tb.Marcas = append(tb.Marcas, m)
	}
	return tb
}

// programaEntrada shows stdin with ⏎ for line breaks (and the args, if any).
func programaEntrada(cs nucleo.CasoPrograma) string {
	s := strings.ReplaceAll(cs.Entrada, "\n", "⏎")
	if len(cs.Args) > 0 {
		s = strings.TrimSpace(strings.Join(cs.Args, " ") + " " + s)
	}
	if s == "" {
		return "(nada)"
	}
	return s
}

// programaFraseRun: "Lo ejecuté con «2⏎3⏎» y escribió «La suma es 5»."
func programaFraseRun(pp programaPrueba) string {
	r, ok := programaRunVistoso(pp)
	if ok {
		salida := strings.TrimSpace(r.ej.Salida)
		if salida == "" {
			return "Lo ejecuté con «" + programaEntrada(r.caso) + "» y no escribió nada (como debía)."
		}
		salida = strings.ReplaceAll(salida, "\n", " ⏎ ")
		return "Lo ejecuté con «" + programaEntrada(r.caso) + "» y escribió «" + recortar(salida, 200) + "»."
	}
	return ""
}

// programaRunVistoso picks the run to tell: a user example first, else one whose input and output are
// not trivial (not empty, not just 0).
func programaRunVistoso(pp programaPrueba) (programaRun, bool) {
	trivial := map[string]bool{"": true, "0": true, "false": true, "[]": true}
	var primera *programaRun
	for i := range pp.runs {
		r := &pp.runs[i]
		if !r.ok || r.err != nil {
			continue
		}
		if r.caso.Expectativa == nucleo.EspUsuario {
			return *r, true
		}
		if primera == nil {
			primera = r
		}
	}
	for i := range pp.runs {
		r := &pp.runs[i]
		if r.ok && r.err == nil && !trivial[strings.TrimSpace(r.ej.Salida)] && strings.Trim(r.caso.Entrada, "0 \n") != "" {
			return *r, true
		}
	}
	if primera != nil {
		return *primera, true
	}
	return programaRun{}, false
}

func programaJustificacion(pp programaPrueba) []string {
	j := []string{"Lo comprobé así:"}
	for _, cp := range pp.comps {
		j = append(j, "· "+cp.Texto+" ("+cp.Detalle+")")
	}
	if pp.vet.OK {
		j = append(j, "· go vet no se queja")
	}
	if pp.comp.Comando != "" {
		j = append(j, "Comando: "+pp.comp.Comando)
	}
	return j
}

// ---- 1. Plantillas ----

func programaPlantilla(c *Cerebro, k *tarea) (nucleo.Respuesta, bool) {
	q := k.q
	n := k.raiz.Sub(nucleo.PasoRecuerdo, "Miro mis plantillas de programas")
	lemas := crearLemas(q)
	hs := recetas.BuscarPlantillas(lemas, 3)
	probadas := 0
	for _, h := range hs {
		if h.Plantilla == nil || !programaEncaja(q, *h.Plantilla, h.Puntos, lemas) {
			continue
		}
		p := *h.Plantilla
		probadas++
		n.Nota("Pruebo la plantilla «%s» (%s)", p.Titulo, decimal(h.Puntos))
		valores, notas, ok := programaRellenar(c, k, p)
		if !ok {
			continue
		}
		fuente, err := recetas.Instanciar(p, valores)
		if err != nil {
			k.raiz.Sub(nucleo.PasoError, "No pude rellenar la plantilla: %s", mensajeError(err)).Mal("")
			continue
		}
		casos := programaCasosPlantilla(p, valores)
		for _, e := range q.EjProgramas {
			e.Expectativa = nucleo.EspUsuario
			if e.Nota == "" {
				e.Nota = "tu ejemplo"
			}
			casos = append(casos, e)
		}
		if len(casos) == 0 {
			cs := nucleo.CasoPrograma{Nota: "sin entrada"}
			if p.Interactivo {
				cs.Args = []string{"-semilla", "1"}
			}
			casos = append(casos, cs)
		}
		pp, err := programaProbar(c, k, fuente, casos)
		if err != nil {
			if errors.Is(err, errSinArenero) {
				n.Info("No puedo comprobar programas sin Go")
				return programaSinComprobar(k, fuente, p.Titulo), true
			}
			continue
		}
		if pp.malos > 0 {
			continue
		}
		n.Bien("Uso la plantilla «%s»", p.Titulo)
		titulo := strings.TrimSuffix(strings.TrimSpace(p.Titulo), ".")
		r := nucleo.Respuesta{
			Texto:          "Aquí tienes el programa: " + programaMinuscula(titulo) + ".",
			Codigo:         fuente,
			Comprobaciones: pp.comps,
			Nivel:          pruebas.NivelGlobal(pp.comps),
			Tablas:         []nucleo.Tabla{pp.tabla},
			Metodo:         "plantilla",
			Exito:          true,
			Justificacion:  append(programaJustificacion(pp), "Plantilla: "+p.Nombre),
		}
		if p.Descripcion != "" {
			r.Parrafos = append(r.Parrafos, strings.TrimSpace(p.Descripcion))
		}
		frase := programaFraseRun(pp)
		if frase != "" {
			r.Parrafos = append(r.Parrafos, frase)
		}
		r.Parrafos = append(r.Parrafos, notas...)
		r.Parrafos = append(r.Parrafos, FraseGoRun)
		if len(q.EjProgramas) == 0 && frase != "" {
			r.Entendi = "Así lo entendí: " + programaEntendi(pp)
		}
		k.turno.Metodo = "plantilla"
		return r, true
	}
	if probadas == 0 {
		n.Info("No tengo una plantilla que encaje")
	} else {
		n.Mal("Ninguna plantilla sirve")
	}
	return nucleo.Respuesta{}, false
}

// programaGenericas are template-name words too common to identify a template on their own.
var programaGenericas = map[string]bool{"programa": true, "numero": true, "hasta": true, "dos": true, "tres": true,
	"lista": true, "texto": true, "juego": true}

// programaEncaja: score ≥ 0.5 (§4.14.2); a template that does not mention the frame's action needs 0.75
// ("la suma de los pares" is not par_impar). Below 0.5, a template is still tried from 0.35 when a
// distinctive word of the request names it ("haz el FizzBuzz hasta 15").
func programaEncaja(q *nucleo.Pregunta, p recetas.Plantilla, puntos float64, lemas []string) bool {
	if q.Marco != nil && q.Marco.Accion != "" && puntos < 0.75 {
		tiene := false
		for _, w := range p.Palabras {
			if crearRaiz(w) == crearRaiz(q.Marco.Accion) {
				tiene = true
			}
		}
		if !tiene {
			return false
		}
	}
	if puntos >= programaUmbralPlantilla {
		return true
	}
	if puntos < 0.35 {
		return false
	}
	partes := map[string]bool{}
	for _, w := range strings.Split(p.Nombre, "_") {
		partes[crearRaiz(w)] = true
	}
	for _, l := range lemas {
		if r := crearRaiz(l); partes[r] && !programaGenericas[r] {
			return true
		}
	}
	return false
}

// programaEntendi: "con «2⏎3⏎» escribe «La suma es 5»; con … ".
func programaEntendi(pp programaPrueba) string {
	var partes []string
	for _, r := range pp.runs {
		if !r.ok || len(partes) >= 2 {
			continue
		}
		salida := strings.ReplaceAll(strings.TrimSpace(r.ej.Salida), "\n", " ⏎ ")
		partes = append(partes, "con «"+programaEntrada(r.caso)+"» escribe «"+recortar(salida, 80)+"»")
	}
	return strings.Join(partes, "; ")
}

// programaRellenar fills the slots from the request: numbers, the frame's operation, quoted texts and
// options named in the text; a slot that is still missing is asked with its Pregunta (buttons), and
// without an answer it takes its default.
func programaRellenar(c *Cerebro, k *tarea, p recetas.Plantilla) (map[string]string, []string, bool) {
	q := k.q
	valores := map[string]string{}
	var notas []string
	var enteros []string
	for _, nm := range q.Numeros {
		if nm.Valor != nil && nm.Valor.IsInt() && nm.Valor.Num().IsInt64() {
			enteros = append(enteros, nm.Valor.Num().String())
		}
	}
	numEnteros := 0
	for _, h := range p.Huecos {
		if h.Clase == "entero" {
			numEnteros++
		}
	}
	citas := programaCitas(q)
	normal := nucleo.Normalizar(q.Texto)
	usados := 0
	for _, h := range p.Huecos {
		v := ""
		switch h.Clase {
		case "entero":
			switch {
			case numEnteros == 1 && len(enteros) > 0:
				v = enteros[len(enteros)-1]
			case usados < len(enteros):
				v = enteros[usados]
				usados++
			}
		case "operacion":
			if q.Marco != nil {
				v = programaOperaciones[q.Marco.Accion]
			}
			if v == "" {
				for _, cpt := range q.Conceptos {
					if op, ok := programaOperaciones[cpt]; ok {
						v = op
						break
					}
				}
			}
		case "opcion":
			for _, o := range h.Opciones {
				if o != "" && strings.Contains(normal, nucleo.Normalizar(o)) {
					v = o
					break
				}
			}
		case "texto", "palabra":
			if len(citas) > 0 {
				v = citas[0]
				citas = citas[1:]
			}
		}
		if v == "" {
			switch {
			case len(h.Opciones) > 0:
				if resp, ok := c.preguntar(k, programaPregunta(h), h.Opciones); ok {
					v = resp
				}
			case h.PorDefecto == "":
				if resp, ok := c.preguntarLibre(k, programaPregunta(h), nil, ""); ok {
					v = strings.TrimSpace(resp)
				}
			}
		}
		if v == "" {
			if h.PorDefecto == "" {
				k.raiz.Sub(nucleo.PasoNota, "Me falta un dato: %s", programaPregunta(h)).Mal("")
				return nil, nil, false
			}
			v = h.PorDefecto
			notas = append(notas, fmt.Sprintf("Para «%s» uso %s; si quieres otro valor, dímelo.", strings.TrimSuffix(strings.TrimPrefix(programaPregunta(h), "¿"), "?"), v))
		}
		valores[h.Nombre] = v
	}
	return valores, notas, true
}

func programaPregunta(h recetas.Hueco) string {
	if strings.TrimSpace(h.Pregunta) != "" {
		return h.Pregunta
	}
	return "¿Qué valor pongo en «" + h.Nombre + "»?"
}

// programaCitas: the quoted texts of the request.
func programaCitas(q *nucleo.Pregunta) []string {
	var out []string
	for _, m := range reCitado.FindAllStringSubmatch(q.Texto, -1) {
		for _, g := range m[1:] {
			if g != "" {
				out = append(out, g)
			}
		}
	}
	return out
}

// programaCasosPlantilla: the template's cases whose slot values (defaults filled in) are the chosen ones.
func programaCasosPlantilla(p recetas.Plantilla, valores map[string]string) []nucleo.CasoPrograma {
	var out []nucleo.CasoPrograma
	for _, cp := range p.Casos {
		igual := true
		for _, h := range p.Huecos {
			quiere := h.PorDefecto
			if v, ok := cp.Huecos[h.Nombre]; ok {
				quiere = v
			}
			if strings.TrimSpace(quiere) != strings.TrimSpace(valores[h.Nombre]) {
				igual = false
				break
			}
		}
		if !igual {
			continue
		}
		cs := cp.Caso
		if cs.Expectativa == nucleo.EspNinguna && cs.Esperado != "" {
			cs.Expectativa = nucleo.EspReferencia
		}
		if cs.Nota == "" {
			cs.Nota = "la plantilla"
		}
		out = append(out, cs)
		if len(out) >= programaMaxCasos {
			break
		}
	}
	return out
}

func programaSinComprobar(k *tarea, fuente, que string) nucleo.Respuesta {
	k.raiz.Sub(nucleo.PasoNota, "No tengo Go para comprobar el programa").Info("")
	return nucleo.Respuesta{
		Texto:          afirmar("Este programa hace esto: "+programaMinuscula(strings.TrimSuffix(que, "."))+".", nil),
		Codigo:         fuente,
		Parrafos:       []string{"No lo he podido ejecutar: no encuentro Go instalado.", FraseGoRun},
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar: no tengo Go"}},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "plantilla",
	}
}

// ---- 2. Función + envoltorio ----

// programaEnvolver wraps a verified function into a program and re-runs its cases on the program.
func programaEnvolver(c *Cerebro, k *tarea, hecho crearHecho) (nucleo.Respuesta, bool) {
	n := k.raiz.Sub(nucleo.PasoIntento, "Lo convierto en un programa que lee los datos y escribe el resultado")
	fuente, aCaso, err := recetas.Envolver(hecho.codigo, hecho.firma)
	if err != nil {
		n.Mal("No sé convertirlo en programa: %s", mensajeError(err))
		return nucleo.Respuesta{}, false
	}
	n.Bien("")
	casos := programaDeCasos(hecho.casos, aCaso)
	pp, err := programaProbar(c, k, fuente, casos)
	if err != nil {
		if errors.Is(err, errSinArenero) {
			return programaSinComprobar(k, fuente, "lee los datos, llama a "+hecho.firma.Nombre+" y escribe el resultado"), true
		}
		return nucleo.Respuesta{}, false
	}
	if pp.malos > 0 {
		return nucleo.Respuesta{}, false
	}
	fr := hecho.resp
	comps := append(append([]nucleo.Comprobacion(nil), pp.comps...), fr.Comprobaciones...)
	r := nucleo.Respuesta{
		Texto:          fmt.Sprintf("Aquí tienes el programa: lee los datos, usa `%s` y escribe el resultado.", hecho.firma.Nombre),
		Codigo:         fuente,
		Entendi:        fr.Entendi,
		Comprobaciones: comps,
		Nivel:          pruebas.NivelGlobal(comps),
		Tablas:         []nucleo.Tabla{pp.tabla},
		Aprendido:      fr.Aprendido,
		Metodo:         fr.Metodo,
		Exito:          true,
		Fuentes:        fr.Fuentes,
	}
	if r.Metodo == "" {
		r.Metodo = "sintesis"
	}
	r.Parrafos = append(r.Parrafos, programaLee(hecho.firma))
	if len(fr.Parrafos) > 0 {
		r.Parrafos = append(r.Parrafos, fr.Parrafos[0])
	}
	if frase := programaFraseRun(pp); frase != "" {
		r.Parrafos = append(r.Parrafos, frase)
	}
	r.Parrafos = append(r.Parrafos, FraseGoRun)
	r.Justificacion = append(programaJustificacion(pp), fr.Justificacion...)
	if len(fr.Tablas) > 0 {
		r.Tablas = append(r.Tablas, fr.Tablas...)
	}
	k.turno.Metodo = r.Metodo
	return r, true
}

// programaDeCasos converts function cases into program cases (at most 12, user ones first).
func programaDeCasos(cs []nucleo.Caso, aCaso func(nucleo.Caso) (nucleo.CasoPrograma, bool)) []nucleo.CasoPrograma {
	var out []nucleo.CasoPrograma
	for _, x := range cs {
		if len(out) >= programaMaxCasos {
			break
		}
		cp, ok := aCaso(x)
		if !ok {
			continue
		}
		if cp.Expectativa == nucleo.EspNinguna && x.ConEsperado() {
			cp.Expectativa = x.Expectativa
		}
		if cp.Nota == "" {
			cp.Nota = x.Nota
		}
		out = append(out, cp)
	}
	return out
}

// programaLee says what the wrapped program reads: "Lee 2 números (uno por línea) y escribe el resultado."
func programaLee(f nucleo.Firma) string {
	var partes []string
	for _, p := range f.Params {
		partes = append(partes, programaDato(p.Tipo))
	}
	if len(partes) == 0 {
		return "No lee nada: escribe el resultado."
	}
	return "Lee " + strings.Join(partes, ", luego ") + " y escribe el resultado."
}

func programaDato(t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CString:
		return "una línea de texto"
	case nucleo.CBool:
		return "true o false"
	case nucleo.CFloat:
		return "un número decimal"
	case nucleo.CLista:
		if t.Elem != nil && t.Elem.Clase == nucleo.CLista {
			return "una tabla (una fila por línea y una línea vacía al final)"
		}
		if t.Elem != nil && t.Elem.Clase == nucleo.CString {
			return "una línea de palabras"
		}
		return "una línea de números separados por espacios"
	}
	return "un número"
}

// programaDeCodigo: "hazlo programa" wraps the session's function and compares the program with it.
func programaDeCodigo(c *Cerebro, k *tarea) nucleo.Respuesta {
	codigo := k.codigo
	if strings.Contains(codigo, "package main") && strings.Contains(codigo, "func main(") {
		k.raiz.Sub(nucleo.PasoEntender, "Ya es un programa completo").Info("")
		pp, err := programaProbar(c, k, codigo, []nucleo.CasoPrograma{{Nota: "sin entrada"}})
		r := nucleo.Respuesta{Texto: "Ya es un programa completo.", Codigo: codigo, Metodo: "envoltorio",
			Comprobaciones: []nucleo.Comprobacion{}, Nivel: nucleo.NivelSinComprobar}
		if err == nil {
			r.Comprobaciones = pp.comps
			r.Nivel = pruebas.NivelGlobal(pp.comps)
			r.Tablas = []nucleo.Tabla{pp.tabla}
			r.Exito = pp.malos == 0
		} else {
			r.Parrafos = append(r.Parrafos, "No lo pude ejecutar: "+mensajeError(err)+".")
		}
		r.Parrafos = append(r.Parrafos, FraseGoRun)
		return r
	}
	f, ok := programaFirmaDe(k, codigo)
	if !ok {
		k.raiz.Sub(nucleo.PasoEntender, "No encuentro una función que pueda envolver").Mal("")
		return nucleo.Respuesta{
			Texto:          "No encuentro una función que pueda convertir en programa.",
			Parrafos:       []string{"Necesito una función normal (no un método) que reciba números, textos o listas."},
			Comprobaciones: []nucleo.Comprobacion{},
			Nivel:          nucleo.NivelSinComprobar,
			Metodo:         "envoltorio",
		}
	}
	k.raiz.Sub(nucleo.PasoEntender, "Envuelvo `%s` en un programa", f.Go()).Info("")
	fuente, aCaso, err := recetas.Envolver(codigo, f)
	if err != nil {
		k.raiz.Sub(nucleo.PasoError, "No sé convertirla en programa: %s", mensajeError(err)).Mal("")
		return nucleo.Respuesta{
			Texto:          "No sé convertir esta función en programa: " + mensajeError(err) + ".",
			Comprobaciones: []nucleo.Comprobacion{},
			Nivel:          nucleo.NivelSinComprobar,
			Metodo:         "envoltorio",
		}
	}
	// The function's own outputs on generated inputs are the reference for the program.
	var casos []nucleo.CasoPrograma
	if c.ej != nil && c.ej.Revisar(codigo, nucleo.PermisoAuto) == nil {
		gen := c.generarCasos(k, f, nil, 8, 12)
		n := k.raiz.Sub(nucleo.PasoPrueba, "Ejecuto tu función para saber qué debe escribir el programa")
		b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{codigo}, Firma: f, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
		if err == nil {
			rs, err := b.Probar(k.ctx, gen, nucleo.OpcionesProbar{})
			b.Cerrar()
			if err == nil && len(rs) > 0 {
				var ref []nucleo.Caso
				for i, r := range rs[0] {
					if i < len(gen) && crearTerminado(r) && r.Obtenido != nil {
						x := gen[i]
						x.Esperado, x.Expectativa, x.Nota = r.Obtenido, nucleo.EspReferencia, "tu función"
						ref = append(ref, x)
					}
				}
				casos = programaDeCasos(ref, aCaso)
				n.Info("")
			} else {
				n.Mal("No pude ejecutarla")
			}
		}
	}
	if len(casos) == 0 {
		casos = []nucleo.CasoPrograma{{Nota: "sin entrada"}}
	}
	pp, err := programaProbar(c, k, fuente, casos)
	r := nucleo.Respuesta{
		Texto:          fmt.Sprintf("Aquí tienes `%s` convertida en programa.", f.Nombre),
		Codigo:         fuente,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "envoltorio",
	}
	r.Parrafos = append(r.Parrafos, programaLee(f))
	if err != nil {
		r.Parrafos = append(r.Parrafos, "No lo pude ejecutar: "+mensajeError(err)+".")
	} else {
		r.Comprobaciones = pp.comps
		r.Nivel = pruebas.NivelGlobal(pp.comps)
		r.Tablas = []nucleo.Tabla{pp.tabla}
		r.Exito = pp.malos == 0
		r.Justificacion = programaJustificacion(pp)
		if frase := programaFraseRun(pp); frase != "" {
			r.Parrafos = append(r.Parrafos, frase)
		}
	}
	r.Parrafos = append(r.Parrafos, FraseGoRun)
	k.turno.Metodo = r.Metodo
	return r
}

// programaFirmaDe picks the function to wrap: the session's function name, else the first plain one.
func programaFirmaDe(k *tarea, codigo string) (nucleo.Firma, bool) {
	fs, err := analisis.Firmas(codigo)
	if err != nil || len(fs) == 0 {
		return nucleo.Firma{}, false
	}
	quiero := k.s.contexto().Funcion
	if k.previo != nil && k.previo.Funcion != nil {
		quiero = k.previo.Funcion.Nombre
	}
	var primera *nucleo.Firma
	for i := range fs {
		f := fs[i]
		if f.Receptor != nil || f.Nombre == "main" || f.Nombre == "init" || !f.Probable() {
			continue
		}
		if f.Nombre == quiero {
			return f, true
		}
		if primera == nil {
			primera = &fs[i]
		}
	}
	if primera == nil {
		return nucleo.Firma{}, false
	}
	return *primera, true
}

// ---- 3. Web ----

// programaWeb: program candidates from the web, checked by the auto policy, built and vetted; those that
// do not read stdin are run once and their output shown. Badge "Sin comprobar · compila".
func programaWeb(c *Cerebro, k *tarea) (nucleo.Respuesta, bool) {
	q := k.q
	if c.web == nil || c.ej == nil || strings.TrimSpace(q.ConsultaWeb) == "" || !c.enLinea(k.ctx) {
		return nucleo.Respuesta{}, false
	}
	presupuesto := k.presupuesto * 35 / 100
	if presupuesto <= 0 || presupuesto > k.restante() {
		presupuesto = k.restante()
	}
	if presupuesto < time.Second {
		return nucleo.Respuesta{}, false
	}
	ctx, cancelar := context.WithTimeout(k.ctx, presupuesto)
	defer cancelar()
	n := k.raiz.Sub(nucleo.PasoBusqueda, "Busco un programa en la web: «%s»", q.ConsultaWeb)
	_, docs, err := c.web.BuscarDocumentos(ctx, q.ConsultaWeb)
	if err != nil || len(docs) == 0 {
		n.Mal("No encontré nada útil")
		return nucleo.Respuesta{}, false
	}
	for _, d := range docs {
		for _, cand := range saber.Candidatas(d) {
			if !cand.EsPrograma {
				continue
			}
			if viol := c.ej.Revisar(cand.Codigo, nucleo.PermisoAuto); len(viol) > 0 {
				n.Nota("No lo ejecuto: %s (%s)", viol[0].Que, saber.Dominio(cand.URL))
				continue
			}
			comp := c.ej.Compilar(ctx, cand.Codigo)
			if !comp.OK {
				n.Nota("El de %s no compila", saber.Dominio(cand.URL))
				continue
			}
			vet := c.ej.Vet(ctx, cand.Codigo)
			n.Bien("Uso un programa de %s", saber.Dominio(cand.URL))
			comps := []nucleo.Comprobacion{{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar · compila", Comando: comp.Comando}}
			r := nucleo.Respuesta{
				Texto:          segun(cand.URL, "este programa puede servirte."),
				Codigo:         cand.Codigo,
				Comprobaciones: comps,
				Nivel:          nucleo.NivelSinComprobar,
				Fuentes:        []nucleo.Fuente{{URL: cand.URL, Titulo: d.Titulo, Nota: "fragmento de la web (licencia del sitio)"}},
				Metodo:         "web",
				Exito:          true,
			}
			r.Parrafos = append(r.Parrafos, "No sé qué debe escribir, así que no lo he comprobado: solo sé que compila.")
			if !vet.OK && len(vet.Errores) > 0 {
				r.Parrafos = append(r.Parrafos, "go vet avisa: "+resumenErrores(vet, nil)+".")
			}
			if !programaLeeEntrada(cand.Codigo) {
				if pp, err := programaProbar(c, k, cand.Codigo, []nucleo.CasoPrograma{{Nota: "sin entrada"}}); err == nil && len(pp.runs) > 0 && pp.runs[0].ok {
					r.Parrafos = append(r.Parrafos, "Lo ejecuté una vez y escribió «"+recortar(strings.TrimSpace(pp.runs[0].ej.Salida), 200)+"».")
					r.Tablas = []nucleo.Tabla{pp.tabla}
				}
			}
			r.Parrafos = append(r.Parrafos, FraseGoRun)
			k.turno.Metodo = "web"
			return r, true
		}
	}
	n.Mal("No encontré un programa que pueda usar")
	return nucleo.Respuesta{}, false
}

func programaLeeEntrada(codigo string) bool {
	for _, s := range []string{"os.Stdin", "fmt.Scan", "bufio.NewScanner(os", "bufio.NewReader(os", "os.Args"} {
		if strings.Contains(codigo, s) {
			return true
		}
	}
	return false
}

func programaMinuscula(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	if empiezaConNombre(s) || len(r) > 1 && r[1] >= 'A' && r[1] <= 'Z' {
		return s // an acronym or identifier ("FizzBuzz" stays)
	}
	return strings.ToLower(string(r[0])) + string(r[1:])
}
