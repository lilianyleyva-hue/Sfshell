package cerebro

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/nucleo"
)

// Routing (§4.14.1 steps 2-7).

const (
	corte      = 0.25 // candidates scoring below this are dropped
	margenDuda = 0.15 // the second is "close" when it is within 15 % of the first
)

// candidato is one interpretation of a request.
type candidato struct {
	nombre    string // strategy key: the intent name, or the resolver name ("mates.ecuacion")
	intencion nucleo.TipoIntencion
	familia   string
	puntos    float64 // 0..1, from lengua or the resolver's Reconoce
	score     float64 // puntos × (0.5 + Tasa("estrategia:"+nombre)), in [0.5, 1.5]×puntos
	res       nucleo.Resolutor
	lectura   string // Spanish reading ("ecuación lineal en x")
	motivo    string
	generico  bool // a reasoning intent from lengua that no resolver recognized
}

// intencionesRazonar are answered by the reasoning resolvers (mates, logica, puzles), not by pipelines.
var intencionesRazonar = map[nucleo.TipoIntencion]bool{
	nucleo.ICalculo: true, nucleo.IEcuacion: true, nucleo.IProblema: true, nucleo.INumeros: true,
	nucleo.ILogica: true, nucleo.ISilogismo: true, nucleo.IRecordar: true, nucleo.IPreguntarHecho: true,
	nucleo.IPuzle: true, nucleo.IConteo: true, nucleo.ISecuencia: true, nucleo.IPlan: true,
}

// rutear answers a request: explicit action, follow-up, or a new question.
func (c *Cerebro) rutear(k *tarea) nucleo.Respuesta {
	if k.p.Accion != "" {
		return c.accionExplicita(k)
	}
	if prev, ok := k.s.ultimo(); ok && strings.TrimSpace(k.p.Codigo) == "" && !tieneCodigo(k.p.Texto) {
		ctx := k.s.contexto()
		if seg, ok := lengua.DetectarSeguimiento(k.p.Texto, &ctx); ok {
			if r, ok := c.seguir(k, seg, prev); ok {
				return r
			}
		}
	}
	if strings.TrimSpace(k.p.Codigo) == "" && !tieneCodigo(k.p.Texto) {
		if r, ok := c.ordenSobreCodigo(k); ok {
			return r
		}
	}
	k.q = c.analizar(k, k.p.Texto, k.p.Codigo)
	if r, ok := c.rutearHabla(k); ok {
		return r
	}
	return c.interpretar(k)
}

// ordenesCodigo are short requests about the code of the conversation that are not follow-ups for
// lengua ("¿tiene errores?", "¿es eficiente?", "arréglalo"). Without code in the request they act on
// the session's code, like explicar_eso or ejecutar_eso (§4.14.1 step 4: "Uso el código de antes").
var ordenesCodigo = []struct {
	re   *regexp.Regexp
	i    nucleo.TipoIntencion
	tipo string
}{
	{regexp.MustCompile(`^(y )?((hazle|haz|hagale|le haces) (unas |las |algunas )?pruebas|pruebal[ao]|testeal[ao]|revisal[ao]|(tiene|hay) (algun |algunos )?(error|errores|fallo|fallos|bug|bugs)|(funciona|va) bien)( (a |en )?(el|este|ese|mi) codigo| (a |en )?(la|esta|esa|mi) funcion| eso| esto)?$`),
		nucleo.IProbar, "probar_eso"},
	{regexp.MustCompile(`^(y )?(es (eficiente|rapid[oa]|lent[oa])|que (complejidad|coste|costo) tiene|cuanto tarda|explicame (este|ese) codigo|explica (este|ese) codigo|que hace (este|ese) codigo|que hace (la|esta|esa) funcion)( (el|este|ese|mi) codigo| (la|esta|esa|mi) funcion| eso| esto)?$`),
		nucleo.IExplicar, "explicar_eso"},
	{regexp.MustCompile(`^(y )?(arreglal[ao]|corrigel[ao]|reparal[ao]|arregla(lo|la)? (el|este|ese|mi) codigo|arregla(la)? (la|esta|esa|mi) funcion|no compila)$`),
		nucleo.IReparar, "reparar_eso"},
}

// ordenSobreCodigo runs one of ordenesCodigo on the session's code. Without code in the session the
// pipeline itself asks for it ("Pega el código que quieres que pruebe").
func (c *Cerebro) ordenSobreCodigo(k *tarea) (nucleo.Respuesta, bool) {
	n := strings.TrimSpace(strings.Trim(nucleo.Normalizar(k.p.Texto), " .;,:"))
	for _, o := range ordenesCodigo {
		if !o.re.MatchString(n) {
			continue
		}
		var codigo string
		if k.s.numTurnos() > 0 {
			codigo = k.s.contexto().Codigo
		}
		if strings.TrimSpace(codigo) == "" {
			k.q = c.analizar(k, k.p.Texto, "")
			return c.despachar(k, o.i), true
		}
		k.turno.Seguimiento = o.tipo
		k.codigo, k.codigoPrevio = codigo, true
		k.q = c.analizar(k, k.p.Texto, codigo)
		k.raiz.Nota("Uso el código de antes")
		return c.despachar(k, o.i), true
	}
	return nucleo.Respuesta{}, false
}

// analizar parses a text with the learned lexicon and the session context (nil without turns).
func (c *Cerebro) analizar(k *tarea, texto, codigo string) *nucleo.Pregunta {
	var pctx *nucleo.Contexto
	if k.s.numTurnos() > 0 {
		ctx := k.s.contexto()
		pctx = &ctx
	}
	return lengua.Analizar(texto, codigo, k.p.Modo, k.lex, pctx)
}

func tieneCodigo(texto string) bool {
	cod, _ := lengua.ExtraerCodigo(texto)
	return strings.TrimSpace(cod) != ""
}

// accionExplicita: p.Accion skips routing (§4.14.1 step 2).
func (c *Cerebro) accionExplicita(k *tarea) nucleo.Respuesta {
	var i nucleo.TipoIntencion
	switch k.p.Accion {
	case "arreglar":
		i = nucleo.IReparar
	case "explicar":
		i = nucleo.IExplicar
	case "probar":
		i = nucleo.IProbar
	case "ejecutar":
		i = nucleo.IEjecutar
	case "estudiar":
		i = nucleo.IEstudiar
	case "pensar_mas":
		return c.pensarMasAccion(k)
	default:
		k.raiz.Nota("No conozco la acción «%s»: lo trato como una pregunta", k.p.Accion)
		k.p.Accion = ""
		return c.rutear(k)
	}
	k.q = c.analizar(k, k.p.Texto, k.p.Codigo)
	return c.despachar(k, i)
}

// pensarMasAccion re-runs a previous task with the budget doubled (up to MaxPresupuesto), the synthesis
// limit doubled and the web enabled (k.pensarMas).
func (c *Cerebro) pensarMasAccion(k *tarea) nucleo.Respuesta {
	k.pensarMas = true
	k.raiz.Nota("Pienso más: con el doble de tiempo")
	prev, ok := k.s.buscarTurno(k.p.Previa)
	if !ok {
		prev, ok = c.buscarTurno(k.p.Sesion, k.p.Previa)
	}
	if !ok {
		k.p.Accion = ""
		return c.rutear(k)
	}
	k.presupuestoPrevio = prev.Presupuesto
	if strings.TrimSpace(k.p.Texto) == "" {
		k.p.Texto = prev.Peticion.Texto
	}
	if strings.TrimSpace(k.p.Codigo) == "" {
		k.p.Codigo = prev.Peticion.Codigo
	}
	k.turno.Texto = prev.Texto
	switch {
	case prev.Peticion.Accion != "" && prev.Peticion.Accion != "pensar_mas":
		k.p.Accion = prev.Peticion.Accion
		return c.accionExplicita(k)
	case prev.Resolutor != "":
		k.q = c.analizar(k, prev.Texto, "")
		lista := c.candidatosRazonar(k.q, prev.Intencion)
		lista = primeroElResolutor(lista, prev.Resolutor)
		return c.conPresupuesto(k, prev.Intencion, func() nucleo.Respuesta { return c.razonar(k, lista, false) })
	case prev.Intencion == "":
		k.p.Accion = ""
		return c.rutear(k)
	}
	if prev.Pregunta != nil {
		q := *prev.Pregunta
		k.q = &q
	}
	if nucleo.Familia(prev.Intencion) == "programar" && prev.Seguimiento != "" {
		k.codigo = prev.Codigo
	}
	k.excluir = append([]string(nil), prev.Excluidos...)
	return c.despachar(k, prev.Intencion)
}

// candidatos ranks every interpretation (§4.14.1 step 5): the intents from lengua (programming,
// memory, chat, search), the Reconoce of every reasoning resolver, and the chat templates that only
// cerebro knows. score = puntos × (0.5 + Tasa("estrategia:"+nombre)). Best first.
func (c *Cerebro) candidatos(q *nucleo.Pregunta) []candidato {
	var out []candidato
	if extra := charlaExtra(q.Normal); extra != "" {
		out = append(out, candidato{nombre: string(nucleo.ICharla), intencion: nucleo.ICharla, familia: "charla",
			puntos: 0.97, lectura: etiquetaIntencion(nucleo.ICharla), motivo: "es una pregunta sobre mí"})
	}
	out = append(out, c.candidatosHabla(q)...) // "¿qué significa X?" (habla.go)
	visto := map[nucleo.TipoIntencion]bool{}
	if m := reUsaFuncion.FindStringSubmatch(q.Texto); m != nil {
		if g, ok := c.mem.Funcion(m[1]); ok && g.Codigo != "" {
			// "usa SumaPares para [1,2,3,4]": a function of my memory, named to be used (crear_funcion
			// reuses it by its exact name)
			visto[nucleo.ICrearFuncion] = true
			out = append(out, candidato{nombre: string(nucleo.ICrearFuncion), intencion: nucleo.ICrearFuncion, familia: "programar",
				puntos: 0.95, lectura: "Usar la función " + g.Nombre + ", que ya sé", motivo: "nombras una función de mi memoria"})
		}
	}
	for _, ci := range q.Intenciones {
		if intencionesRazonar[ci.Tipo] || visto[ci.Tipo] {
			continue
		}
		visto[ci.Tipo] = true
		out = append(out, candidato{nombre: string(ci.Tipo), intencion: ci.Tipo, familia: familiaDe(ci.Tipo),
			puntos: ci.Puntos, lectura: etiquetaIntencion(ci.Tipo), motivo: ci.Motivo})
	}
	hayResolutor := false
	cantidad := preguntaCantidad(q.Texto)
	qr := paraRazonar(q)
	for _, r := range c.resolutores {
		if cantidad && soloVerdad[r.Nombre()] {
			continue
		}
		rec := reconocer(r, qr)
		if rec.Puntos <= 0 {
			continue
		}
		cd := candidato{nombre: r.Nombre(), intencion: rec.Intencion, familia: familiaDe(rec.Intencion),
			puntos: rec.Puntos, res: r, lectura: rec.Lectura}
		if cd.lectura == "" {
			cd.lectura = etiquetaIntencion(rec.Intencion)
		}
		cd.score = c.puntuar(cd)
		if cd.score >= corte {
			hayResolutor = true
		}
		out = append(out, cd)
	}
	if !hayResolutor {
		// lengua sees a reasoning question that no resolver can read: try that family anyway, so the
		// answer is "No sé plantearlo. Prueba a escribirlo así: …" instead of "no te entiendo"
		visto := map[nucleo.TipoIntencion]bool{}
		for _, ci := range q.Intenciones {
			if !intencionesRazonar[ci.Tipo] || visto[ci.Tipo] {
				continue
			}
			visto[ci.Tipo] = true
			out = append(out, candidato{nombre: string(ci.Tipo), intencion: ci.Tipo, familia: familiaDe(ci.Tipo),
				puntos: 0.6 * ci.Puntos, lectura: etiquetaIntencion(ci.Tipo), motivo: ci.Motivo, generico: true})
		}
	}
	switch notacionFlechas(q) {
	case "datos":
		// "[1,2,3,4] -> 6", "casa -> 2": example pairs, not implications or numbers to compute
		for i := range out {
			if out[i].res != nil || out[i].generico {
				out[i].puntos *= 0.5
			}
		}
	case "logica":
		// "p -> q; p; ¿q?": an implication between letters, not examples of a function
		for i := range out {
			if out[i].intencion == nucleo.ICrearFuncion || out[i].intencion == nucleo.ICrearPrograma {
				out[i].puntos *= 0.5
			}
		}
	}
	for i := range out {
		out[i].score = c.puntuar(out[i])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out
}

// notacionFlechas tells what the "a -> b" pairs lengua found are: "logica" when every side is made of
// one-letter atoms and connectives ("p -> q"), "datos" when lengua sees no reasoning question and some
// side is a value ("[1,2] -> 3", "casa -> 2", "hola -> aloh"), "" otherwise.
func notacionFlechas(q *nucleo.Pregunta) string {
	if q == nil || len(q.ParesTexto) == 0 {
		return ""
	}
	todasLogica := true
	for _, par := range q.ParesTexto {
		if !ladoLogico(par[0]) || !ladoLogico(par[1]) {
			todasLogica = false
		}
	}
	if todasLogica {
		return "logica"
	}
	for _, ci := range q.Intenciones {
		if intencionesRazonar[ci.Tipo] && ci.Puntos >= 0.5 {
			return ""
		}
	}
	return "datos"
}

var conectivas = map[string]bool{"y": true, "o": true, "no": true, "ni": true, "si": true, "entonces": true, "implica": true}

// ladoLogico: only one-letter atoms, connectives and logic symbols ("q y p", "(p", "¬q").
func ladoLogico(s string) bool {
	s = strings.NewReplacer("->", " ", "<->", " ", "(", " ", ")", " ", "&", " ", "|", " ", "!", " ", "~", " ", "¬", " ",
		"∧", " ", "∨", " ", "→", " ", "¿", " ", "?", " ", ",", " ", ".", " ", ";", " ").Replace(nucleo.Normalizar(s))
	ws := strings.Fields(s)
	if len(ws) == 0 {
		return false
	}
	for _, w := range ws {
		if conectivas[w] {
			continue
		}
		if len(w) != 1 || w[0] < 'a' || w[0] > 'z' {
			return false
		}
	}
	return true
}

func (c *Cerebro) puntuar(cd candidato) float64 {
	return cd.puntos * (0.5 + c.stats.Tasa("estrategia:"+cd.nombre))
}

// paraRazonar is the question the reasoning resolvers see. They read Texto, so Go code pasted inside
// the text is left out: its "if err != nil {" would read as an implication, and "x := 5" as an
// equation. The copy's Texto is the text without the code, with its own tokens and numbers.
func paraRazonar(q *nucleo.Pregunta) *nucleo.Pregunta {
	if q == nil || strings.TrimSpace(q.Codigo) == "" || strings.TrimSpace(q.Texto) == q.Resto {
		return q
	}
	cp := *q
	cp.Texto = q.Resto
	cp.Normal = nucleo.Normalizar(q.Resto)
	cp.Tokens = lengua.Lematizar(q.Resto)
	cp.Numeros = lengua.Numeros(q.Resto)
	if cp.Numeros == nil {
		cp.Numeros = []nucleo.Numero{}
	}
	return &cp
}

// reconocer calls Reconoce without letting a panic out.
func reconocer(r nucleo.Resolutor, q *nucleo.Pregunta) (rec nucleo.Reconocimiento) {
	defer func() {
		if x := recover(); x != nil {
			rec = nucleo.Reconocimiento{}
		}
	}()
	return r.Reconoce(q)
}

func familiaDe(i nucleo.TipoIntencion) string {
	if f := nucleo.Familia(i); f != "" {
		return f
	}
	return "razonar"
}

// interpretar ranks, cuts, disambiguates and runs (§4.14.1 steps 5-7).
func (c *Cerebro) interpretar(k *tarea) nucleo.Respuesta {
	todos := c.candidatos(k.q)
	var vivos []candidato
	for _, cd := range todos {
		if cd.score >= corte {
			vivos = append(vivos, cd)
		}
	}
	if len(vivos) == 0 {
		k.raiz.Sub(nucleo.PasoEntender, "No sé qué me pides").Info("")
		return c.noTeEntiendo(k, todos)
	}
	elegido := vivos[0]
	paso := k.raiz.Sub(nucleo.PasoEntender, "Lo entiendo como: %s", elegido.lectura)
	for i, cd := range vivos {
		if i == 4 {
			break
		}
		paso.Detalle("%s (%s)%s", cd.lectura, decimal(cd.score), motivoEntre(cd.motivo))
	}
	paso.Info("")
	if len(vivos) > 1 {
		seg := vivos[1]
		if seg.familia != elegido.familia && seg.score >= (1-margenDuda)*elegido.score {
			if elegido.res != nil && seg.res != nil {
				// both are cheap reasoning resolvers: run both at once, each with half the budget
				lista := resolutoresDe(vivos)
				return c.conPresupuesto(k, elegido.intencion, func() nucleo.Respuesta { return c.razonar(k, lista, true) })
			}
			e, ok := c.desambiguar(k, elegido, seg)
			if !ok {
				return c.noTeEntiendo(k, todos)
			}
			elegido = e
		}
	}
	if elegido.res != nil || elegido.generico {
		var lista []candidato
		if elegido.generico {
			lista = c.candidatosRazonar(k.q, elegido.intencion)
		} else {
			lista = primeroElResolutor(resolutoresDe(vivos), elegido.nombre)
		}
		return c.conPresupuesto(k, elegido.intencion, func() nucleo.Respuesta { return c.razonar(k, lista, false) })
	}
	return c.despachar(k, elegido.intencion)
}

func motivoEntre(m string) string {
	if m == "" {
		return ""
	}
	return ": " + m
}

// resolutoresDe keeps the resolver candidates, in order.
func resolutoresDe(cs []candidato) []candidato {
	var out []candidato
	for _, cd := range cs {
		if cd.res != nil {
			out = append(out, cd)
		}
	}
	return out
}

// primeroElResolutor moves the named resolver to the front.
func primeroElResolutor(cs []candidato, nombre string) []candidato {
	for i, cd := range cs {
		if cd.nombre == nombre && i > 0 {
			out := append([]candidato{cd}, cs[:i]...)
			return append(out, cs[i+1:]...)
		}
	}
	return cs
}

// candidatosRazonar returns the resolvers for a reasoning intent: those of its package first (mates,
// logica or puzles), ranked by their Reconoce, then the others that recognize the question.
func (c *Cerebro) candidatosRazonar(q *nucleo.Pregunta, i nucleo.TipoIntencion) []candidato {
	prefijo := prefijoIntencion(i)
	var propios, otros []candidato
	cantidad := q != nil && preguntaCantidad(q.Texto)
	qr := paraRazonar(q)
	for _, r := range c.resolutores {
		if cantidad && soloVerdad[r.Nombre()] {
			continue // a "¿cuántos…?" question never gets a "se sigue / no se sigue" answer (§0)
		}
		rec := reconocer(r, qr)
		cd := candidato{nombre: r.Nombre(), intencion: rec.Intencion, familia: familiaDe(rec.Intencion),
			puntos: rec.Puntos, res: r, lectura: rec.Lectura}
		if cd.intencion == "" {
			cd.intencion = i
		}
		if cd.lectura == "" {
			cd.lectura = etiquetaIntencion(cd.intencion)
		}
		cd.score = c.puntuar(cd)
		switch {
		case prefijo != "" && strings.HasPrefix(r.Nombre(), prefijo):
			propios = append(propios, cd)
		case rec.Puntos > 0:
			otros = append(otros, cd)
		}
	}
	sort.SliceStable(propios, func(a, b int) bool { return propios[a].score > propios[b].score })
	sort.SliceStable(otros, func(a, b int) bool { return otros[a].score > otros[b].score })
	if len(propios) == 0 && len(otros) == 0 {
		for _, r := range c.resolutores {
			if cantidad && soloVerdad[r.Nombre()] {
				continue
			}
			propios = append(propios, candidato{nombre: r.Nombre(), intencion: i, familia: familiaDe(i), res: r, lectura: etiquetaIntencion(i)})
		}
	}
	return append(propios, otros...)
}

// soloVerdad are the resolvers whose answer is a truth value ("se sigue", "no se sigue", "siempre es
// verdad"): they cannot answer a question that asks for a quantity or for someone or something.
var soloVerdad = map[string]bool{"logica.proposicional": true, "logica.silogismo": true}

var (
	reCantidad = regexp.MustCompile(`\bcuant[oa]s?\b`)
	// reAbierta: a question that starts with an interrogative word asks for a thing, a person, a
	// place or a time, not for "sí / no" ("¿quién es el menor?", "¿qué pasa?", "¿cuál gana?").
	reAbierta = regexp.MustCompile(`^(y |entonces |pues )?(quien(es)?|que|cual(es)?|donde|adonde|cuando|como|por que)\b`)
)

// preguntaCantidad: the question asks for a number ("¿cuántos le quedan?", "¿cuánto cuesta?") or is
// an open question ("¿quién es el menor?"), so a truth value is never its answer (§0).
func preguntaCantidad(texto string) bool {
	if reCantidad.MatchString(nucleo.Normalizar(texto)) {
		return true
	}
	// only the question itself is read: "cuando llueve, el suelo se moja; llueve. ¿se moja?" asks sí/no
	i := strings.LastIndex(texto, "¿")
	if i < 0 {
		return false
	}
	return reAbierta.MatchString(nucleo.Normalizar(texto[i:]))
}

func prefijoIntencion(i nucleo.TipoIntencion) string {
	switch i {
	case nucleo.ICalculo, nucleo.IEcuacion, nucleo.IProblema, nucleo.INumeros:
		return "mates."
	case nucleo.ILogica, nucleo.ISilogismo, nucleo.IRecordar, nucleo.IPreguntarHecho:
		return "logica."
	case nucleo.IPuzle, nucleo.IConteo, nucleo.ISecuencia, nucleo.IPlan:
		return "puzles."
	}
	return ""
}

// desambiguar asks which of two close readings the user meant (§4.14.1 step 6). Without an answer it
// takes the first; "Otra cosa" gives ok=false.
func (c *Cerebro) desambiguar(k *tarea, a, b candidato) (candidato, bool) {
	ea, eb := etiquetaIntencion(a.intencion), etiquetaIntencion(b.intencion)
	if ea == eb {
		ea, eb = ea+" ("+a.lectura+")", eb+" ("+b.lectura+")"
	}
	resp, ok := c.preguntar(k, "Lo puedo entender de dos formas. ¿Qué quieres que haga?", []string{ea, eb, "Otra cosa"})
	if !ok {
		return a, true
	}
	switch strings.Trim(nucleo.Normalizar(resp), " .") {
	case strings.Trim(nucleo.Normalizar(eb), " ."):
		return b, true
	case "otra cosa":
		return candidato{}, false
	}
	return a, true
}

// ejemplosFamilia are the rephrasing examples of the "no te entiendo" template.
var ejemplosFamilia = map[string][2]string{
	"programar": {"haz una función que sume los números pares de una lista", "[1,2,3,4] -> 6; [5,7] -> 0; [] -> 0"},
	"razonar":   {"resuelve 2x+3=11", "¿de cuántas formas elijo 3 de 10?"},
	"saber":     {"¿para qué sirve defer?", "busca cómo invertir un mapa en Go"},
	"charla":    {"¿qué sabes hacer?", "¿cómo aprendes?"},
}

// noTeEntiendo is the template for a request with no candidate above the cut: buttons for the three
// most likely families and two ways to rephrase it.
func (c *Cerebro) noTeEntiendo(k *tarea, todos []candidato) nucleo.Respuesta {
	peso := map[string]float64{}
	for _, cd := range todos {
		peso[cd.familia] += cd.puntos
	}
	familias := []string{"programar", "razonar", "saber", "charla"}
	sort.SliceStable(familias, func(i, j int) bool { return peso[familias[i]] > peso[familias[j]] })
	familias = familias[:3]
	ej := ejemplosFamilia[familias[0]]
	var chips []string
	for _, f := range familias {
		chips = append(chips, ejemplosFamilia[f][0])
	}
	k.pendiente = "no_entiendo"
	k.estrategia = "-"
	return nucleo.Respuesta{
		Texto: "No te entiendo.",
		Parrafos: []string{
			"Entiendo frases parecidas a los ejemplos de abajo: pedir una función o un programa, arreglar o explicar código, cuentas, ecuaciones, lógica, puzles y preguntas sobre Go.",
			fmt.Sprintf("Prueba a escribirlo así: «%s» o «%s».", ej[0], ej[1]),
		},
		Sugerencias:    chips,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "no_entiendo",
	}
}

// etiquetaIntencion is the Spanish label of an intent (question buttons, trace).
func etiquetaIntencion(i nucleo.TipoIntencion) string {
	switch i {
	case nucleo.ICrearFuncion:
		return "Escribir una función"
	case nucleo.ICrearPrograma:
		return "Escribir un programa"
	case nucleo.IReparar:
		return "Arreglar el código"
	case nucleo.IExplicar:
		return "Explicar el código"
	case nucleo.IProbar:
		return "Probar el código"
	case nucleo.IEjecutar:
		return "Ejecutar el código"
	case nucleo.IOptimizar:
		return "Hacer el código más rápido"
	case nucleo.IDocGo:
		return "Explicarte algo de Go"
	case nucleo.ICalculo:
		return "Hacer la cuenta"
	case nucleo.IEcuacion:
		return "Resolver la ecuación"
	case nucleo.IProblema:
		return "Resolver el problema"
	case nucleo.INumeros:
		return "Calcular con números (primos, divisores…)"
	case nucleo.ILogica:
		return "Razonar con lógica"
	case nucleo.ISilogismo:
		return "Razonar el silogismo"
	case nucleo.IRecordar:
		return "Recordarlo"
	case nucleo.IPreguntarHecho:
		return "Responder con lo que sé"
	case nucleo.IPuzle:
		return "Resolver el puzle"
	case nucleo.IConteo:
		return "Contar las posibilidades"
	case nucleo.ISecuencia:
		return "Seguir la serie"
	case nucleo.IPlan:
		return "Buscar los pasos"
	case nucleo.IBuscar:
		return "Buscarlo en internet"
	case nucleo.IMemoria:
		return "Mirar mi memoria"
	case nucleo.ICharla:
		return "Charlar"
	case nucleo.IEstudiar:
		return "Estudiar Go"
	}
	if i == "" {
		return "otra cosa"
	}
	return string(i)
}
