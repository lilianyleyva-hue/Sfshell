package cerebro

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/nucleo"
)

// Follow-ups (§4.14.4). seguir returns ok=false when the follow-up does not fit the previous turn
// (e.g. "explícalo" with no code in the session): the text is then routed as a new request.
func (c *Cerebro) seguir(k *tarea, seg lengua.Seguimiento, prev Turno) (nucleo.Respuesta, bool) {
	k.seguimiento = &seg
	k.previo = &prev
	k.turno.Seguimiento = seg.Tipo
	codigo := k.s.contexto().Codigo
	if codigo == "" {
		codigo = prev.Codigo
	}
	deCodigo := nucleo.Familia(prev.Intencion) == "programar"
	switch seg.Tipo {
	case "esta_mal", "esta_bien", "por_que", "otra_forma", "mas_simple", "mas_detalle", "cambiar_numero":
		if esPlantilla(prev) {
			return nucleo.Respuesta{}, false // a greeting or a template has nothing to judge or redo
		}
	}
	conCodigo := func(i nucleo.TipoIntencion) (nucleo.Respuesta, bool) {
		if codigo == "" {
			return nucleo.Respuesta{}, false
		}
		k.codigo, k.codigoPrevio = codigo, true
		k.q = c.analizar(k, k.p.Texto, codigo)
		k.raiz.Nota("Uso el código de antes")
		return c.despachar(k, i), true
	}
	switch seg.Tipo {
	case "cambiar_numero":
		return c.cambiarNumero(k, seg, prev)
	case "por_que":
		return c.porQue(k, prev), true
	case "mas_simple", "mas_detalle":
		if deCodigo && codigo != "" {
			k.detalle = 1
			if seg.Tipo == "mas_detalle" {
				k.detalle = 2
			}
			return conCodigo(nucleo.IExplicar)
		}
		if len(prev.Respuesta.Plan)+len(prev.Respuesta.Justificacion) > 0 {
			return c.reexplicarPasos(k, prev, seg.Tipo == "mas_detalle"), true
		}
		return nucleo.Respuesta{}, false
	case "otra_forma":
		if prev.Resolutor != "" {
			return c.otraFormaRazonada(k, prev), true
		}
		if esCreacion(prev.Intencion) && prev.Pregunta != nil {
			q := *prev.Pregunta
			k.q = &q
			k.turno.Texto = prev.Texto
			k.excluir = excluidosDe(prev)
			k.turno.Excluidos = append([]string(nil), k.excluir...)
			k.raiz.Nota("Busco otra forma distinta de las %d que ya te enseñé", len(k.excluir))
			r := c.despachar(k, prev.Intencion)
			if !otraFormaNueva(r, prev) {
				return c.sinOtraForma(k, prev), true
			}
			return r, true
		}
		return nucleo.Respuesta{}, false
	case "esta_mal":
		return c.estaMal(k, seg, prev), true
	case "esta_bien":
		return c.estaBien(k, prev), true
	case "probar_con", "vacia":
		if seg.Tipo == "probar_con" && reOrdenEjecutar.MatchString(nucleo.Normalizar(seg.Texto)) {
			// "ejecútalo con entrada 5" runs the code with that input (stdin "5⏎"); it is not a test
			k.turno.Seguimiento = "ejecutar_eso"
			return conCodigo(nucleo.IEjecutar)
		}
		k.probarCon = quitarPalabraEntrada(seg.Entrada)
		k.vacia = seg.Tipo == "vacia"
		return conCodigo(nucleo.IProbar)
	case "modificar":
		// "ahora con impares" changes the last request that created code, also after follow-ups about
		// that code ("pruébala con [3,3]", "hazlo programa", "explícalo")
		base, comoPrograma, ok := k.s.baseCreacion()
		if !ok {
			return nucleo.Respuesta{}, false
		}
		nueva := c.analizar(k, k.p.Texto, "")
		k.q = lengua.Fusionar(base.Pregunta, nueva)
		k.turno.Texto = k.q.Texto
		k.raiz.Nota("Cambio la petición «%s»: %s", recortar(primeraLinea(base.Texto), 80), seg.Texto)
		i := base.Intencion
		if comoPrograma && i == nucleo.ICrearFuncion {
			// it was turned into a program afterwards: the new version is a program too
			i, k.sinPlantillas = nucleo.ICrearPrograma, true
		}
		return c.despachar(k, i), true
	case "comentarios":
		k.comentarios = true
		return conCodigo(nucleo.IExplicar)
	case "explicar_eso":
		return conCodigo(nucleo.IExplicar)
	case "ejecutar_eso":
		return conCodigo(nucleo.IEjecutar)
	case "programa_eso":
		k.envolver = true
		return conCodigo(nucleo.ICrearPrograma)
	case "mas_rapido":
		return conCodigo(nucleo.IOptimizar)
	}
	return nucleo.Respuesta{}, false
}

// plantillas are answers that follow-ups about "the previous answer" do not apply to.
var plantillas = map[string]bool{"charla": true, "memoria": true, "no_entiendo": true, "respuesta": true, "error": true}

// esPlantilla: the turn answered with a fixed template. A function reused from memory ("Ya sé hacer
// esto") also has Metodo "memoria", but it is code the user can question, judge or redo.
func esPlantilla(t Turno) bool {
	m := t.Respuesta.Metodo
	if m == "memoria" && nucleo.Familia(t.Intencion) == "programar" {
		return false
	}
	return plantillas[m]
}

// reOrdenEjecutar: a probar_con follow-up that asks to run the code ("ejecútalo con entrada 5",
// "córrela con [1,2]"), not to test it.
var reOrdenEjecutar = regexp.MustCompile(`^(y )?(ejecutal[ao]|ejecuta|ejecutar|correl[ao]|corre|lanzal[ao])\b`)

// quitarPalabraEntrada: "pruébala con entrada [3,3]" → "[3,3]".
func quitarPalabraEntrada(s string) string {
	t := strings.TrimSpace(s)
	n := nucleo.Normalizar(t)
	for _, pre := range []string{"la entrada ", "entrada ", "los datos ", "el valor "} {
		if strings.HasPrefix(n, pre) && len(t) >= len(pre) {
			return strings.TrimSpace(strings.TrimLeft(t[len(pre):], ":"))
		}
	}
	return t
}

// otraFormaNueva: the re-run for "otra forma" produced something new: a success whose code differs
// from the code already shown.
func otraFormaNueva(r nucleo.Respuesta, prev Turno) bool {
	if !r.Exito || r.Nivel == nucleo.NivelFallo {
		return false
	}
	nuevo := ejcCompactar(r.Codigo)
	return nuevo == "" || nuevo != ejcCompactar(prev.Respuesta.Codigo)
}

// sinOtraForma is the honest answer when "otra forma" finds nothing new: the previous answer was solved,
// so this is not a failure, nothing goes to pendientes and no strategy loses points (a meta turn).
func (c *Cerebro) sinOtraForma(k *tarea, prev Turno) nucleo.Respuesta {
	c.meta(k, prev)
	k.pendiente = ""
	k.programa = nil
	k.raiz.Sub(nucleo.PasoNota, "No encontré otra forma distinta de la que te enseñé").Info("")
	return nucleo.Respuesta{
		Texto: "No conozco otra forma de hacerlo: solo sé hacerlo como te mostré.",
		Parrafos: []string{"Si quieres algo distinto, dime qué debe cambiar («ahora con …») o dame un ejemplo " +
			"(entrada -> salida)."},
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "seguimiento",
	}
}

// excluidosDe: the programs already shown in that line of turns (for "otra forma").
func excluidosDe(prev Turno) []string {
	out := append([]string(nil), prev.Excluidos...)
	if prev.Expr != "" {
		out = append(out, prev.Expr)
	}
	return out
}

// cambiarNumero: "¿y si fuera 13?" replaces the number of the same unit (or the last number) in the
// last question and re-runs the same resolver (or pipeline): "Con 13: x = 5".
func (c *Cerebro) cambiarNumero(k *tarea, seg lengua.Seguimiento, prev Turno) (nucleo.Respuesta, bool) {
	if seg.Numero == nil || seg.Numero.Valor == nil || prev.Texto == "" {
		return nucleo.Respuesta{}, false
	}
	texto, viejo, ok := reemplazarNumero(prev.Texto, seg)
	if !ok {
		return nucleo.Respuesta{}, false
	}
	nuevo := seg.Numero.Valor.RatString()
	k.turno.Texto = texto
	k.raiz.Sub(nucleo.PasoEntender, "Cambio %s por %s: «%s»", viejo, nuevo, texto).Info("")
	k.q = c.analizar(k, texto, "")
	var r nucleo.Respuesta
	switch {
	case prev.Resolutor != "":
		r = c.rerazonarCon(k, prev.Resolutor, prev.Intencion, nil)
	case nucleo.Familia(prev.Intencion) == "programar" && prev.Intencion != "":
		r = c.despachar(k, prev.Intencion)
	default:
		return nucleo.Respuesta{}, false
	}
	if r.Exito {
		r.Texto = "Con " + nuevo + ": " + r.Texto
	}
	return r, true
}

// reemplazarNumero puts the follow-up's number in place of the number with the same unit in texto
// ("80 km/h" for "¿y si fuera a 100 km/h?"), else of the last number.
func reemplazarNumero(texto string, seg lengua.Seguimiento) (nuevoTexto, viejo string, ok bool) {
	nums := lengua.Numeros(texto)
	if len(nums) == 0 {
		return "", "", false
	}
	idx := len(nums) - 1
	norm := nucleo.Normalizar(seg.Texto)
	if seg.Numero.Texto != "" {
		if p := strings.Index(norm, nucleo.Normalizar(seg.Numero.Texto)); p >= 0 {
			if u := palabraTras(norm, p+len(nucleo.Normalizar(seg.Numero.Texto))); u != "" {
				for i := len(nums) - 1; i >= 0; i-- {
					if nucleo.Normalizar(palabraTras(texto, nums[i].Hasta)) == u {
						idx = i
						break
					}
				}
			}
		}
	}
	n := nums[idx]
	if n.Desde < 0 || n.Hasta > len(texto) || n.Desde >= n.Hasta {
		return "", "", false
	}
	return texto[:n.Desde] + seg.Numero.Valor.RatString() + texto[n.Hasta:], strings.TrimSpace(texto[n.Desde:n.Hasta]), true
}

// palabraTras returns the word (letters, digits and "/") right after byte offset i.
func palabraTras(s string, i int) string {
	if i < 0 || i > len(s) {
		return ""
	}
	r := strings.TrimLeft(s[i:], " \t")
	fin := strings.IndexFunc(r, func(x rune) bool { return !(unicode.IsLetter(x) || unicode.IsDigit(x) || x == '/') })
	if fin < 0 {
		fin = len(r)
	}
	return r[:fin]
}

// meta builds an answer that only talks about the previous turn.
func (c *Cerebro) meta(k *tarea, prev Turno) {
	k.meta = true
	k.intencion = prev.Intencion
	k.estrategia = ""
	k.turno.Texto = prev.Texto
	k.q = prev.Pregunta
}

// porQue renders the last justification: equations → steps, logic → rule chain, code → tests and
// the program's description.
func (c *Cerebro) porQue(k *tarea, prev Turno) nucleo.Respuesta {
	c.meta(k, prev)
	pr := prev.Respuesta
	pasos := pr.Justificacion
	if len(pasos) == 0 {
		pasos = pr.Plan
	}
	var parrafos []string
	if prev.Funcion != nil && prev.Funcion.Descripcion != "" {
		parrafos = append(parrafos, "En resumen: "+prev.Funcion.Descripcion+".")
	}
	if prev.Expr != "" {
		parrafos = append(parrafos, "El programa que encontré: "+prev.Expr)
	}
	var cs []string
	for _, cp := range pr.Comprobaciones {
		if cp.Texto != "" {
			cs = append(cs, cp.Texto)
		}
	}
	if len(cs) > 0 {
		parrafos = append(parrafos, "Lo comprobé así: "+strings.Join(cs, "; ")+".")
	}
	r := nucleo.Respuesta{
		Plan:           pasos,
		Justificacion:  pasos,
		Parrafos:       parrafos,
		Comprobaciones: pr.Comprobaciones,
		Nivel:          pr.Nivel,
		Exito:          pr.Exito,
		Metodo:         "seguimiento",
	}
	for _, tb := range pr.Tablas {
		if tablaCompleta(tb) {
			r.Tablas = append(r.Tablas, tb)
		}
	}
	switch {
	case len(pasos) > 0:
		r.Texto = "Te lo explico paso a paso:"
	case len(parrafos) > 0:
		r.Texto = "Te cuento cómo llegué a esa respuesta:"
	default:
		r.Texto = "No tengo más explicación que la que te mostré."
		r.Exito = false
		r.Nivel = nucleo.NivelSinComprobar
		r.Comprobaciones = []nucleo.Comprobacion{}
	}
	k.raiz.Sub(nucleo.PasoNota, "Repaso cómo llegué a la respuesta anterior").Info("")
	return r
}

// reexplicarPasos shows the reasoning steps collapsed ("más simple") or all of them ("con detalle").
func (c *Cerebro) reexplicarPasos(k *tarea, prev Turno, detalle bool) nucleo.Respuesta {
	c.meta(k, prev)
	pr := prev.Respuesta
	pasos := pr.Plan
	if len(pasos) == 0 {
		pasos = pr.Justificacion
	}
	r := nucleo.Respuesta{
		Texto:          pr.Texto,
		Comprobaciones: pr.Comprobaciones,
		Nivel:          pr.Nivel,
		Exito:          pr.Exito,
		Justificacion:  pr.Justificacion,
		Metodo:         "seguimiento",
		Tablero:        pr.Tablero,
	}
	if detalle {
		r.Plan = append([]string(nil), pasos...)
		r.Parrafos = append([]string(nil), pr.Parrafos...)
		for _, j := range pr.Justificacion {
			if !contieneParrafo(r.Plan, j) {
				r.Parrafos = append(r.Parrafos, j)
			}
		}
		k.raiz.Sub(nucleo.PasoNota, "Lo cuento con todos los pasos").Info("")
		return r
	}
	if len(pasos) > 3 {
		r.Plan = append(append([]string(nil), pasos[:3]...), fmt.Sprintf("… y %d pasos más del mismo estilo.", len(pasos)-3))
	} else {
		r.Plan = append([]string(nil), pasos...)
	}
	r.Parrafos = []string{"En corto: " + primeraLinea(pr.Texto)}
	k.raiz.Sub(nucleo.PasoNota, "Lo cuento más corto").Info("")
	return r
}

// otraFormaRazonada re-runs the reasoning without the resolvers already used.
func (c *Cerebro) otraFormaRazonada(k *tarea, prev Turno) nucleo.Respuesta {
	excluir := map[string]bool{prev.Resolutor: true}
	for _, e := range prev.Excluidos {
		excluir[e] = true
	}
	k.turno.Texto = prev.Texto
	k.turno.Excluidos = append(append([]string(nil), prev.Excluidos...), prev.Resolutor)
	k.q = c.analizar(k, prev.Texto, "")
	hay := false
	for _, cd := range c.candidatosRazonar(k.q, prev.Intencion) {
		if !excluir[cd.nombre] && cd.puntos > 0 {
			hay = true
			break
		}
	}
	if !hay {
		c.meta(k, prev)
		k.raiz.Sub(nucleo.PasoNota, "Busco otra forma de resolverlo").Info("No conozco otra")
		return nucleo.Respuesta{
			Texto:          "No conozco otra forma de resolverlo: solo sé hacerlo como te mostré.",
			Comprobaciones: []nucleo.Comprobacion{},
			Nivel:          nucleo.NivelSinComprobar,
			Metodo:         "seguimiento",
		}
	}
	k.raiz.Nota("Busco otra forma, sin «%s»", prev.Resolutor)
	return c.rerazonarCon(k, "", prev.Intencion, excluir)
}

// estaBien: Opinar(true) raises the stored function's use count and confirms the lexicon proposals.
func (c *Cerebro) estaBien(k *tarea, prev Turno) nucleo.Respuesta {
	c.meta(k, prev)
	_ = c.Opinar(nucleo.Opinion{Sesion: k.p.Sesion, Tarea: prev.Tarea, Correcto: true})
	k.raiz.Sub(nucleo.PasoMemoria, "Apunto que la respuesta anterior estaba bien").Bien("")
	r := nucleo.Respuesta{
		Texto:          "Gracias, lo tendré en cuenta.",
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "seguimiento",
		Exito:          true,
	}
	if prev.Funcion != nil {
		r.Parrafos = []string{fmt.Sprintf("Usaré %s con más confianza.", prev.Funcion.Nombre)}
	}
	return r
}

// estaMal: Opinar(false); the example becomes a stored counterexample and the pipeline re-runs.
// Without an example (code), it asks "¿Qué debería dar? Escribe un ejemplo: entrada -> salida".
func (c *Cerebro) estaMal(k *tarea, seg lengua.Seguimiento, prev Turno) nucleo.Respuesta {
	k.raiz.Sub(nucleo.PasoNota, "Me dices que la respuesta anterior está mal").Info("")
	if prev.Seguimiento != "" && !esCreacion(prev.Intencion) && prev.Intencion != nucleo.IReparar && nucleo.Familia(prev.Intencion) == "programar" {
		// after "explícalo" or "pruébala con …", "está mal" is about the function the app wrote
		if base, _, ok := k.s.baseCreacion(); ok {
			prev = base
			k.previo = &base
		}
	}
	f, conceptos, conFirma := firmaDeTurno(prev)
	deCodigo := nucleo.Familia(prev.Intencion) == "programar"
	var caso *nucleo.Caso
	if deCodigo && conFirma {
		texto := seg.Ejemplo
		if texto == "" {
			texto = quitarEstaMal(seg.Texto)
		}
		var entrada string
		if texto != "" {
			caso, entrada, _ = leerEjemplo(texto, f)
		}
		if caso == nil && entrada != "" && len(f.Res) == 1 {
			if v, _, ok := c.preguntarValor(k, fmt.Sprintf("¿Qué debería dar con %s?", entrada), nil, f.Res[0]); ok {
				caso, _, _ = leerEjemplo(entrada+" -> "+nucleo.FormatoHumano(v, f.Res[0]), f)
			}
		}
		if caso == nil && entrada == "" {
			if resp, ok := c.preguntarLibre(k, "¿Qué debería dar? Escribe un ejemplo: entrada -> salida", nil, ""); ok {
				caso, _, _ = leerEjemplo(resp, f)
			}
		}
	}
	_ = c.Opinar(nucleo.Opinion{Sesion: k.p.Sesion, Tarea: prev.Tarea, Correcto: false, Comentario: seg.Texto})
	if caso != nil {
		if err := c.guardarContraejemplo(conceptos, f, *caso); err != nil {
			k.raiz.Sub(nucleo.PasoMemoria, "No pude guardar el contraejemplo: %s", mensajeError(err)).Mal("")
		} else {
			k.raiz.Sub(nucleo.PasoMemoria, "Guardo el contraejemplo: %s", describirCaso(f, *caso)).Bien("")
		}
		if (esCreacion(prev.Intencion) || prev.Intencion == nucleo.IReparar) && prev.Pregunta != nil {
			q := *prev.Pregunta
			q.Ejemplos = append(append([]nucleo.Caso(nil), prev.Pregunta.Ejemplos...), *caso)
			k.q = &q
			k.ejemplosExtra = []nucleo.Caso{*caso}
			k.excluir = excluidosDe(prev)
			k.turno.Texto = prev.Texto
			if prev.Intencion == nucleo.IReparar {
				k.codigo = prev.Pregunta.Codigo
				if k.codigo == "" {
					k.codigo = prev.Peticion.Codigo
				}
			}
			k.raiz.Nota("Lo vuelvo a intentar con tu ejemplo")
			return c.despachar(k, prev.Intencion)
		}
		c.meta(k, prev)
		return nucleo.Respuesta{
			Texto:          "Gracias: lo he apuntado y lo comprobaré siempre a partir de ahora.",
			Parrafos:       []string{"Contraejemplo: " + describirCaso(f, *caso) + "."},
			Comprobaciones: []nucleo.Comprobacion{},
			Nivel:          nucleo.NivelSinComprobar,
			Metodo:         "seguimiento",
			Exito:          true,
		}
	}
	c.meta(k, prev)
	r := nucleo.Respuesta{
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "seguimiento",
	}
	if deCodigo {
		r.Texto = "Dime un ejemplo de lo que debería dar (entrada -> salida) y lo vuelvo a intentar."
		r.Parrafos = []string{"Por ejemplo: «está mal: con [2] debe dar 0»."}
		return r
	}
	r.Texto = "Gracias por avisar: lo he apuntado."
	var cs []string
	for _, cp := range prev.Respuesta.Comprobaciones {
		if cp.Nivel != nucleo.NivelSinComprobar && cp.Nivel != nucleo.NivelFallo && cp.Texto != "" {
			cs = append(cs, cp.Texto)
		}
	}
	if len(cs) > 0 {
		r.Parrafos = append(r.Parrafos, "Mi respuesta estaba comprobada ("+strings.Join(cs, "; ")+"), así que lo más probable es que entendiera otra cosa.")
	}
	if prev.Respuesta.Entendi != "" {
		r.Parrafos = append(r.Parrafos, prev.Respuesta.Entendi)
	}
	r.Parrafos = append(r.Parrafos, "Prueba a escribir la pregunta de otra forma.")
	return r
}

var reEstaMal = regexp.MustCompile(`(?i)^\s*(no|est[aá] mal|incorrect[oa]|mal|eso no es|no es eso|falla|no es correct[oa]|eso est[aá] mal|te equivocas|error|no funciona|no est[aá] bien)\s*[,:.;!]?\s*`)

func quitarEstaMal(s string) string { return strings.TrimSpace(reEstaMal.ReplaceAllString(s, "")) }

var reEjemploMal = regexp.MustCompile(`(?i)^\s*(?:con|para|si (?:le )?(?:paso|doy|pongo))\s+(.+?)\s+(debe dar|deber[ií]a dar|tiene que dar|tendr[ií]a que dar|debe devolver|deber[ií]a devolver|da|devuelve|sale)\s+(.+?)\s*[.!]?\s*$`)

var errSinEjemplo = errors.New("no veo un ejemplo (entrada -> salida)")

// leerEjemplo reads "con [2] debe dar 0" or "[2] -> 0" as a counterexample case for f. For a report of
// what it gives now ("con [2] da 2") it returns no case and the input, so the caller asks what it
// should give.
func leerEjemplo(texto string, f nucleo.Firma) (*nucleo.Caso, string, error) {
	texto = strings.TrimSpace(texto)
	if pares, _ := lengua.ExtraerEjemplos(texto); len(pares) > 0 {
		casos, _, err := lengua.TiparEjemplos(pares[:1], &f)
		if err != nil || len(casos) == 0 {
			return nil, "", err
		}
		cs := casos[0]
		return contraejemplo(cs), "", nil
	}
	m := reEjemploMal.FindStringSubmatch(texto)
	if m == nil {
		return nil, "", errSinEjemplo
	}
	verbo := strings.ToLower(m[2])
	if verbo == "da" || verbo == "devuelve" || verbo == "sale" {
		return nil, strings.TrimSpace(m[1]), nil
	}
	casos, _, err := lengua.TiparEjemplos([][2]string{{m[1], m[3]}}, &f)
	if err != nil || len(casos) == 0 {
		return nil, strings.TrimSpace(m[1]), err
	}
	return contraejemplo(casos[0]), "", nil
}

func contraejemplo(c nucleo.Caso) *nucleo.Caso {
	c.Origen = nucleo.OrigenContraejemplo
	c.Expectativa = nucleo.EspUsuario
	c.Nota = "contraejemplo"
	return &c
}

// describirCaso: "con [2] debe dar 0".
func describirCaso(f nucleo.Firma, c nucleo.Caso) string {
	ent := f.Entradas()
	var es []string
	for i, v := range c.Entradas {
		if i < len(ent) {
			es = append(es, nucleo.FormatoHumano(v, ent[i]))
		}
	}
	var ss []string
	for i, v := range c.Esperado {
		if i < len(f.Res) {
			ss = append(ss, nucleo.FormatoHumano(v, f.Res[i]))
		}
	}
	return "con " + strings.Join(es, ", ") + " debe dar " + strings.Join(ss, ", ")
}
