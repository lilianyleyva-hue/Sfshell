package cerebro

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"nyxcodigo/internal/habla"
	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/logica"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/saber"
)

// What words mean and learning to talk (internal/habla) inside the orchestrator: the templates
// (what a word means, how two words relate, a sentence about a topic, reading a text, how much it
// knows), their routing, and the learning hooks (every request is read; the Go notes once per
// version; Spanish web pages through hablaLeerWeb). Nothing here is checked: every answer says where
// its knowledge came from and the sentences it builds start with "Creo que" (Nivel sin_comprobar).

const (
	intervaloHabla = time.Minute // what is read from the requests is saved at most this often
	lecturaNotas   = "notas_go"
	origenNotas    = "lo leí en mis notas de Go"
	metodoHabla    = "habla"
)

// iniciarHabla opens habla.json in the data folder and reads the Go notes when they changed.
func (c *Cerebro) iniciarHabla(dir string) {
	h, err := habla.Abrir(dir)
	if err != nil {
		h = habla.Nuevo()
		h.Avisar(fmt.Sprintf("No puedo guardar lo que aprendo de las palabras: %v.", err))
	}
	c.habla = h
	c.hablaLeerNotas()
}

// hablaLeerNotas reads the Spanish summaries of the shipped Go notes (and the titles that define,
// "defer: dejar algo para el final") once per version of the notes.
func (c *Cerebro) hablaLeerNotas() {
	defer func() { _ = recover() }()
	suma := sha256.New()
	var sb strings.Builder
	for _, n := range recetas.Notas() {
		fmt.Fprintf(suma, "%s\x00%s\x00%s\x00", n.Clave, n.Titulo, n.Resumen)
		if strings.Contains(n.Titulo, ":") {
			sb.WriteString(n.Titulo + "\n")
		}
		sb.WriteString(n.Resumen + "\n")
	}
	version := hex.EncodeToString(suma.Sum(nil))[:16]
	if c.habla.Leida(lecturaNotas) == version {
		return
	}
	c.habla.Leer(sb.String(), habla.FuenteLectura, origenNotas)
	c.habla.MarcarLeida(lecturaNotas, version)
	if err := c.habla.Guardar(); err != nil {
		c.habla.Avisar(fmt.Sprintf("No pude guardar mi diccionario: %v.", err))
	}
}

// Cerrar saves what is still only in memory (the dictionary and the sentences read). main calls it at
// shutdown; calling it twice is harmless.
func (c *Cerebro) Cerrar() error { return c.habla.Guardar() }

// hablaLeerWeb reads the Spanish pages a pipeline fetched (doc_go, buscar, estudiar) with fuente "web":
// it learns the meanings they state and learns to talk from them. Pages in English are skipped.
func (c *Cerebro) hablaLeerWeb(docs ...saber.Documento) {
	if c.habla == nil {
		return
	}
	defer func() { _ = recover() }()
	for _, d := range docs {
		var sb strings.Builder
		for _, b := range d.Bloques {
			if b.Tipo != "codigo" {
				sb.WriteString(b.Texto + "\n")
			}
		}
		texto := sb.String()
		if !habla.EsEspanol(texto) {
			continue
		}
		c.habla.Leer(texto, habla.FuenteWeb, "lo leí en "+saber.Dominio(d.URL))
	}
}

// fechaHoy is "17/10".
func (c *Cerebro) fechaHoy() string {
	d := c.ahora()
	return strconv.Itoa(d.Day()) + "/" + strconv.Itoa(int(d.Month()))
}

// ---- recognizing the requests ----

// pedidoHabla is a recognized habla request.
type pedidoHabla struct {
	tipo       string // "leer" | "decir" | "relacion" | "estado" | "significado" | "ensena"
	palabra    string // significado, ensena: the word; decir: the topic
	otra       string // relacion: the second word
	definicion string // ensena
	texto      string // leer: the text to read
	forma      string // significado: "significa" | "es"; relacion: "relacion" | "parecen" | "contrario"
	lectura    string // Spanish reading for the trace
}

var (
	reHablaLeer      = regexp.MustCompile(`(?is)^\s*(?:l[eé]e(?:te|me)?|apr[eé]nde(?:te)?)\s+(?:esto|este texto|lo siguiente|esta frase|estas frases|este p[aá]rrafo)\s*[:,.]?\s*(.*)$`)
	reHablaDecir     = regexp.MustCompile(`^(?:hablame|habla|dime algo|di algo|dime una frase|di una frase|dime otra frase|di otra frase|dime otra cosa|cuentame algo|cuentame|escribe una frase|haz una frase|inventa una frase) (?:de|sobre|acerca de|con|del) (.+)$`)
	reHablaRelEntre  = regexp.MustCompile(`^(?:que relacion hay entre|que relacion tienen|cual es la relacion entre|que tienen que ver|en que se parecen|se parecen|son parecidos|son parecidas|son opuestos|son opuestas|son contrarios|son contrarias|son sinonimos|son antonimos) (.+?) (?:y|e|con) (.+)$`)
	reHablaRelPost   = regexp.MustCompile(`^(.+?) (?:y|e) (.+?) (se parecen|son parecidos|son parecidas|son opuestos|son opuestas|son contrarios|son contrarias|son sinonimos|son antonimos|tienen relacion|estan relacionados|estan relacionadas)$`)
	reHablaContrario = regexp.MustCompile(`^(?:es )?(.+?) (?:es )?(?:lo )?(?:contrario|opuesto) (?:de|a) (.+)$`)
	reHablaSeParece  = regexp.MustCompile(`^se parece (.+?) a (.+)$`)
	reHablaEstado    = regexp.MustCompile(`\b(que palabras sabes|cuantas palabras sabes|cuantas palabras conoces|que palabras conoces|cuanto sabes hablar|sabes hablar|como hablas|que tal hablas|tu vocabulario|palabras que sabes|que significados sabes)\b`)
	reHablaQueSig    = regexp.MustCompile(`^(?:que significa|que significan|que quiere decir|que quieren decir|cual es el significado de|que significado tiene|que es lo que significa|sabes que significa|sabes lo que significa|dime que significa) (.+)$`)
	reHablaQueEs     = regexp.MustCompile(`^(?:que es|que son|sabes que es|sabes lo que es|que cosa es) (.+)$`)
	reHablaPalabras  = regexp.MustCompile(`^[\pL][\pL-]*( [\pL][\pL-]*){0,2}$`)
	reHablaOrden     = regexp.MustCompile(`(?i)^\s*(?:recuerda|aprende|apunta|memoriza|ten en cuenta)\s+que\s+`)
	reHablaMarcador  = regexp.MustCompile(`(?i)\s(significa|significan|quiere decir|quieren decir)\s`)
)

// palabraLimpia strips quotes and articles from a short phrase (normalized); "" when it is not 1-3 words.
func palabraLimpia(s string) string {
	s = strings.Trim(strings.TrimSpace(s), ` "'.,;:`)
	ws := strings.Fields(s)
	for len(ws) > 1 && articuloHabla[ws[0]] {
		ws = ws[1:]
	}
	s = strings.Join(ws, " ")
	if !reHablaPalabras.MatchString(s) {
		return ""
	}
	return s
}

var articuloHabla = map[string]bool{"el": true, "la": true, "los": true, "las": true, "un": true, "una": true,
	"unos": true, "unas": true, "lo": true, "del": true, "al": true, "palabra": true}

// palabraEscrita finds a normalized word in the original text, to show it with its accents.
func palabraEscrita(original, normal string) string {
	ws := strings.Fields(strings.NewReplacer("¿", " ", "?", " ", "¡", " ", "!", " ", "«", " ", "»", " ", "\"", " ", ",", " ", ".", " ").Replace(original))
	n := len(strings.Fields(normal))
	for i := 0; i+n <= len(ws); i++ {
		cand := strings.Join(ws[i:i+n], " ")
		if nucleo.Normalizar(cand) == normal {
			return strings.ToLower(cand)
		}
	}
	return normal
}

// detectarHabla recognizes the habla requests in a text.
func detectarHabla(texto string) pedidoHabla {
	if m := reHablaLeer.FindStringSubmatch(texto); m != nil {
		return pedidoHabla{tipo: "leer", texto: strings.TrimSpace(m[1]), lectura: "leer un texto para aprender"}
	}
	n := strings.Trim(nucleo.Normalizar(texto), " .;,:")
	if n == "" || len(n) > 400 {
		return pedidoHabla{}
	}
	if reHablaEstado.MatchString(n) {
		return pedidoHabla{tipo: "estado", lectura: "cuántas palabras sé"}
	}
	if m := reHablaDecir.FindStringSubmatch(n); m != nil {
		tema := strings.Trim(m[1], ` "'`)
		if temaConContenido(tema) {
			return pedidoHabla{tipo: "decir", palabra: palabraEscrita(texto, tema), lectura: "decir una frase sobre «" + tema + "»"}
		}
		return pedidoHabla{}
	}
	relacion := func(a, b, forma string) pedidoHabla {
		a, b = palabraLimpia(a), palabraLimpia(b)
		if a == "" || b == "" || interrogativoHabla[a] || interrogativoHabla[b] || vaciasHabla[a] || vaciasHabla[b] {
			return pedidoHabla{}
		}
		return pedidoHabla{tipo: "relacion", palabra: palabraEscrita(texto, a), otra: palabraEscrita(texto, b), forma: forma,
			lectura: "comparar lo que significan «" + a + "» y «" + b + "»"}
	}
	if m := reHablaRelEntre.FindStringSubmatch(n); m != nil {
		forma := "relacion"
		switch {
		case strings.HasPrefix(n, "se parecen") || strings.HasPrefix(n, "son parecid") || strings.HasPrefix(n, "en que se parecen") || strings.HasPrefix(n, "son sinonimos"):
			forma = "parecen"
		case strings.HasPrefix(n, "son opuest") || strings.HasPrefix(n, "son contrari") || strings.HasPrefix(n, "son antonimos"):
			forma = "contrario"
		}
		if p := relacion(m[1], m[2], forma); p.tipo != "" {
			return p
		}
	}
	if m := reHablaRelPost.FindStringSubmatch(n); m != nil {
		forma := "relacion"
		switch {
		case strings.Contains(m[3], "parec") || strings.Contains(m[3], "sinonimo"):
			forma = "parecen"
		case strings.Contains(m[3], "opuest") || strings.Contains(m[3], "contrari") || strings.Contains(m[3], "antonimo"):
			forma = "contrario"
		}
		if p := relacion(m[1], m[2], forma); p.tipo != "" {
			return p
		}
	}
	if m := reHablaSeParece.FindStringSubmatch(n); m != nil {
		if p := relacion(m[1], m[2], "parecen"); p.tipo != "" {
			return p
		}
	}
	if strings.Contains(texto, "?") || strings.Contains(texto, "¿") {
		if m := reHablaContrario.FindStringSubmatch(n); m != nil {
			if p := relacion(m[1], m[2], "contrario"); p.tipo != "" {
				return p
			}
		}
	}
	if m := reHablaQueSig.FindStringSubmatch(n); m != nil {
		if w := palabraLimpia(m[1]); w != "" {
			return pedidoHabla{tipo: "significado", palabra: palabraEscrita(texto, w), forma: "significa", lectura: "qué significa «" + w + "»"}
		}
		return pedidoHabla{}
	}
	if m := reHablaQueEs.FindStringSubmatch(n); m != nil {
		if w := palabraLimpia(m[1]); w != "" {
			return pedidoHabla{tipo: "significado", palabra: palabraEscrita(texto, w), forma: "es", lectura: "qué es «" + w + "»"}
		}
		return pedidoHabla{}
	}
	if !strings.ContainsAny(texto, "?¿") {
		if p, ok := leerEnsenanza(texto); ok {
			return p
		}
	}
	return pedidoHabla{}
}

// quitarArticulo drops a leading article from a phrase as written: "las funciones" → "funciones".
func quitarArticulo(s string) string {
	ws := strings.Fields(s)
	for len(ws) > 1 && articuloHabla[nucleo.Normalizar(ws[0])] {
		ws = ws[1:]
	}
	return strings.Join(ws, " ")
}

// temaConContenido: a short topic of plain words with one that is not an article, pronoun or
// preposition ("de ti" no; "de cuántas formas elijo 3 de 10" is a question, not a topic).
func temaConContenido(tema string) bool {
	ws := strings.Fields(tema)
	if len(ws) > 4 || !soloLetras(strings.ReplaceAll(tema, " ", "")) {
		return false
	}
	for _, w := range ws {
		if interrogativoHabla[w] {
			return false
		}
	}
	for _, w := range ws {
		if !articuloHabla[w] && !vaciasHabla[w] && len([]rune(w)) >= 2 {
			return true
		}
	}
	return false
}

var interrogativoHabla = map[string]bool{"que": true, "cual": true, "cuales": true, "como": true, "cuanto": true, "cuanta": true,
	"cuantos": true, "cuantas": true, "donde": true, "cuando": true, "quien": true, "quienes": true, "por": true, "porque": true}

var vaciasHabla = map[string]bool{"ti": true, "mi": true, "eso": true, "esto": true, "algo": true, "nada": true, "todo": true,
	"y": true, "o": true, "de": true, "en": true, "con": true, "tu": true, "su": true, "que": true, "ello": true}

// leerEnsenanza finds "X significa Y" / "X quiere decir Y" (also after "recuerda que") in a statement.
func leerEnsenanza(texto string) (pedidoHabla, bool) {
	t := strings.TrimSpace(reHablaOrden.ReplaceAllString(strings.TrimSpace(texto), ""))
	loc := reHablaMarcador.FindStringSubmatchIndex(" " + t + " ")
	if loc == nil {
		return pedidoHabla{}, false
	}
	antes := t[:max(0, loc[0])]
	despues := t[min(len(t), loc[1]-1):]
	// X: the last 1-3 words before the marker; before X only an article, "que" or a punctuation mark
	ws := strings.Fields(strings.NewReplacer("«", " ", "»", " ", "\"", " ", "“", " ", "”", " ").Replace(antes))
	j := len(ws)
	var x []string
	for j > 0 && len(x) < 3 {
		w := strings.Trim(ws[j-1], ",;:.")
		nw := nucleo.Normalizar(w)
		if nw == "" || articuloHabla[nw] || vaciasHabla[nw] || !soloLetras(w) || strings.ContainsAny(strings.TrimLeft(ws[j-1], ",;:."), ",;:.") {
			break
		}
		x = append([]string{w}, x...)
		j--
	}
	if len(x) == 0 {
		return pedidoHabla{}, false
	}
	if j > 0 {
		prev := ws[j-1]
		np := nucleo.Normalizar(strings.Trim(prev, ",;:."))
		if !articuloHabla[np] && np != "que" && !strings.ContainsAny(prev, ",;:.") {
			return pedidoHabla{}, false // "X significa Y" must start its sentence
		}
	}
	y := strings.TrimSpace(despues)
	if i := strings.IndexAny(y, ".;!?\n"); i >= 0 {
		y = y[:i]
	}
	y = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(y), "lo mismo que "))
	y = strings.Trim(y, ` "'«»“”,`)
	if y == "" {
		return pedidoHabla{}, false
	}
	palabra := strings.ToLower(strings.Join(x, " "))
	return pedidoHabla{tipo: "ensena", palabra: palabra, definicion: y, lectura: "aprender qué significa «" + palabra + "»"}, true
}

func soloLetras(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && r != '-' {
			return false
		}
	}
	return true
}

// ---- routing ----

// rutearHabla answers the explicit habla requests (read a text, a sentence about a topic, how two
// words relate, how many words I know) and adds teaching to "X significa Y" (the logic and lexicon
// answer stays). ok=false lets the normal routing go on. Follow-ups were already tried.
func (c *Cerebro) rutearHabla(k *tarea) (nucleo.Respuesta, bool) {
	if c.habla == nil || strings.TrimSpace(k.p.Codigo) != "" || tieneCodigo(k.p.Texto) {
		return nucleo.Respuesta{}, false
	}
	a := detectarHabla(k.p.Texto)
	switch a.tipo {
	case "leer", "decir", "relacion", "estado":
		if a.tipo == "relacion" && preguntaDeGo(k.q) {
			return nucleo.Respuesta{}, false // "¿se parecen un slice y un array?" is for doc_go
		}
		k.raiz.Sub(nucleo.PasoEntender, "Lo entiendo como: %s", a.lectura).Info("")
		return c.despachar(k, nucleo.IMemoria), true
	case "ensena":
		r := c.interpretar(k)
		c.hablaEnsenar(k, &r, a)
		return r, true
	}
	return nucleo.Respuesta{}, false
}

// preguntaDeGo: lengua sees a question about Go (doc_go or buscar).
func preguntaDeGo(q *nucleo.Pregunta) bool {
	if q == nil {
		return false
	}
	for _, ci := range q.Intenciones {
		if ci.Puntos >= 0.5 && (ci.Tipo == nucleo.IDocGo || ci.Tipo == nucleo.IBuscar) {
			return true
		}
	}
	return false
}

// candidatosHabla is the "¿qué significa X?" / "¿qué es X?" interpretation. A question that logic can
// answer from a stored fact keeps going to logic, and "¿qué es X?" leaves Go concepts to doc_go and
// maths, logic or puzzles to their resolvers.
func (c *Cerebro) candidatosHabla(q *nucleo.Pregunta) []candidato {
	if c.habla == nil || q == nil || strings.TrimSpace(q.Codigo) != "" {
		return nil
	}
	a := detectarHabla(q.Texto)
	if a.tipo != "significado" || c.logicaSabe(q.Texto) {
		return nil
	}
	puntos := 0.95
	if a.forma == "es" {
		for _, ci := range q.Intenciones {
			if ci.Puntos >= 0.5 && (ci.Tipo == nucleo.IDocGo || ci.Tipo == nucleo.IBuscar || (intencionesRazonar[ci.Tipo] &&
				ci.Tipo != nucleo.IPreguntarHecho && ci.Tipo != nucleo.IRecordar)) {
				return nil
			}
		}
		puntos = 0.6
		if _, ok := c.habla.Buscar(a.palabra); ok {
			puntos = 0.9
		}
	}
	return []candidato{{nombre: metodoHabla, intencion: nucleo.IMemoria, familia: "saber", puntos: puntos,
		lectura: a.lectura, motivo: "me preguntas qué significa una palabra"}}
}

// logicaSabe: the question is an open question about a stored fact ("¿qué es Toby?" after "Toby es un
// perro"), which logic answers.
func (c *Cerebro) logicaSabe(texto string) bool {
	pa, ok := logica.LeerPreguntaAbierta(texto)
	if !ok {
		return false
	}
	rel := pa.Relacion
	vistos := map[string]bool{}
	for _, s := range []string{pa.Sujeto, lengua.Lema(pa.Sujeto)} {
		if vistos[s] {
			continue
		}
		vistos[s] = true
		for _, h := range c.mem.Hechos(s) {
			r := strings.ReplaceAll(nucleo.Normalizar(h.Relacion), " ", "_")
			if r == "es" || r == "es_una" || r == "ser" || r == "son" {
				r = "es_un"
			}
			if r == rel && !h.Negado {
				return true
			}
		}
	}
	return false
}

// plantillaHabla answers a habla request inside the memoria template; ok=false when the text is not one.
func (c *Cerebro) plantillaHabla(k *tarea) (nucleo.Respuesta, bool) {
	if c.habla == nil {
		return nucleo.Respuesta{}, false
	}
	a := detectarHabla(k.p.Texto)
	var r nucleo.Respuesta
	switch a.tipo {
	case "significado":
		r = c.respuestaSignificado(k, a.palabra)
	case "relacion":
		r = c.respuestaRelacion(k, a)
	case "decir":
		r = c.respuestaDecir(k, a.palabra)
	case "leer":
		r = c.respuestaLeer(k, a.texto)
	case "estado":
		r = c.respuestaEstadoHabla(k)
	default:
		return nucleo.Respuesta{}, false
	}
	k.turno.Metodo = metodoHabla
	return r, true
}

// ---- answers ----

func plantillaHablaR(texto string, parrafos, sugerencias []string, exito bool) nucleo.Respuesta {
	r := plantilla(texto, parrafos, sugerencias, metodoHabla)
	r.Exito = exito
	return r
}

// deDonde is the sentence that says where a meaning came from: "Me lo enseñaste tú el 17/10.",
// "Viene del diccionario de Nyx.", "Lo leí en mis notas de Go."
func deDonde(s habla.Significado) string {
	o := strings.TrimSpace(s.Origen)
	switch {
	case o == "":
		return ""
	case strings.HasPrefix(o, "del "), strings.HasPrefix(o, "de "):
		return "Viene " + o + "."
	}
	return mayusculaInicial(o) + "."
}

// enseñado: taught by the user (not read).
func ensenado(s habla.Significado) bool {
	return s.Fuente == habla.FuenteUsuario && s.Confianza >= 0.99
}

// frase de una definición: "«variable»: una caja con nombre donde guardas un valor"
func lineaDefinicion(s habla.Significado) string {
	return "«" + s.Palabra + "»: " + strings.TrimRight(s.Definicion, ".") + " (" + strings.TrimSuffix(minusculaInicial(deDonde(s)), ".") + ")."
}

func minusculaInicial(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func (c *Cerebro) noSeQueSignifica(palabra string, extra ...string) nucleo.Respuesta {
	ps := append([]string{fmt.Sprintf("Enséñamelo así: «%s significa …».", palabra)}, extra...)
	return plantillaHablaR(fmt.Sprintf("No sé qué significa «%s».", palabra), ps,
		[]string{"¿Qué palabras sabes?", "¿Qué significa variable?"}, false)
}

// respuestaSignificado: "¿qué significa X?" from the dictionary, saying where it came from.
func (c *Cerebro) respuestaSignificado(k *tarea, palabra string) nucleo.Respuesta {
	paso := k.raiz.Sub(nucleo.PasoMemoria, "Miro mi diccionario: «%s»", palabra)
	s, ok := c.habla.Buscar(palabra)
	if !ok {
		paso.Info("No la tengo")
		return c.noSeQueSignifica(palabra)
	}
	if !s.Propio {
		paso.Info("Solo la conozco de oídas: %s", s.Origen)
		var ps []string
		for i, w := range s.Conceptos {
			if i == 3 {
				break
			}
			if d, ok := c.habla.Buscar(w); ok && d.Propio {
				ps = append(ps, lineaDefinicion(d))
			}
		}
		ps = append(ps, fmt.Sprintf("Enséñamelo así: «%s significa …».", palabra))
		r := plantillaHablaR(fmt.Sprintf("No tengo una definición de «%s»: solo sé que %s.", palabra, s.Origen), ps,
			[]string{"¿Qué significa " + s.Conceptos[0] + "?", "¿Qué palabras sabes?"}, true)
		r.Justificacion = []string{"Una palabra que solo aparece dentro de otras definiciones está anclada: sé de qué tema habla, pero no qué significa."}
		return r
	}
	paso.Bien("%s", recortar(s.Definicion, 120))
	def := strings.TrimRight(s.Definicion, ".")
	var texto string
	switch {
	case s.Fuente == habla.FuenteSemilla:
		texto = fmt.Sprintf("«%s»: %s.", s.Palabra, def)
	case ensenado(s):
		texto = fmt.Sprintf("«%s» significa: %s.", s.Palabra, def)
	default:
		texto = fmt.Sprintf("Creo que «%s» significa: %s.", s.Palabra, def)
	}
	ps := []string{deDonde(s)}
	if !ensenado(s) && s.Fuente != habla.FuenteSemilla {
		ps[0] = strings.TrimSuffix(ps[0], ".") + " (no lo he comprobado)."
	}
	if s.Ingles != "" {
		ps = append(ps, "En inglés: "+s.Ingles+".")
	}
	if n := len(s.Anteriores); n > 0 {
		ps = append(ps, "Antes la tenía como: «"+s.Anteriores[n-1]+"».")
	}
	chips := []string{"Háblame de " + s.Palabra}
	for _, w := range s.Conceptos {
		if d, ok := c.habla.Buscar(w); ok && d.Propio && d.Palabra != s.Palabra {
			chips = append(chips, "¿Qué relación hay entre "+s.Palabra+" y "+d.Palabra+"?")
			break
		}
	}
	r := plantillaHablaR(texto, ps, append(chips, "¿Qué palabras sabes?"), true)
	r.Justificacion = []string{"Lo busqué en mi diccionario de significados.", deDonde(s)}
	return r
}

// respuestaRelacion: how two words relate, with the shared words that justify it.
func (c *Cerebro) respuestaRelacion(k *tarea, a pedidoHabla) nucleo.Respuesta {
	paso := k.raiz.Sub(nucleo.PasoMemoria, "Comparo lo que significan «%s» y «%s»", a.palabra, a.otra)
	rel, ok := c.habla.Relacion(a.palabra, a.otra)
	if !ok {
		paso.Info("No sé qué significa %s", strings.Join(comillas(rel.Desconocidas), " ni "))
		r := c.noSeQueSignifica(rel.Desconocidas[0], "Sin saber qué significa no puedo compararla: no me invento la relación.")
		if len(rel.Desconocidas) > 1 {
			r.Texto = fmt.Sprintf("No sé qué significan %s.", strings.Join(comillas(rel.Desconocidas), " ni "))
		}
		return r
	}
	x, y := "«"+a.palabra+"»", "«"+a.otra+"»"
	tema := ""
	if len(rel.Compartidas) > 0 {
		tema = " (" + strings.Join(rel.Compartidas[:min(3, len(rel.Compartidas))], ", ") + ")"
	}
	var frase string
	switch rel.Etiqueta {
	case "opuestos":
		frase = fmt.Sprintf("%s y %s son opuestos: hablan de lo mismo%s, pero van en sentidos contrarios", x, y, tema)
	case "afines":
		frase = fmt.Sprintf("%s y %s son afines: hablan de lo mismo%s y van en el mismo sentido", x, y, tema)
	case "vecinos de tema":
		frase = fmt.Sprintf("%s y %s son del mismo tema%s, sin ser lo mismo ni lo contrario", x, y, tema)
	case "la misma palabra":
		frase = fmt.Sprintf("%s y %s son la misma palabra para mí", x, y)
	default:
		frase = fmt.Sprintf("%s y %s no tienen relación: lo que significan no comparte nada", x, y)
	}
	var texto string
	switch a.forma {
	case "contrario":
		if rel.Etiqueta == "opuestos" {
			texto = "Creo que sí: " + frase + "."
		} else {
			texto = "Creo que no: " + frase + "."
		}
	case "parecen":
		switch rel.Etiqueta {
		case "afines", "vecinos de tema", "la misma palabra":
			texto = "Creo que sí se parecen: " + frase + "."
		case "opuestos":
			texto = "Se parecen en el tema, pero creo que son lo contrario: " + frase + "."
		default:
			texto = "Creo que no se parecen: " + frase + "."
		}
	default:
		texto = afirmar(frase+".", nil)
	}
	paso.Bien("%s (parecido %s)", rel.Etiqueta, decimal(rel.Coseno))
	var ps []string
	if len(rel.Compartidas) > 0 {
		ps = append(ps, "Palabras que comparten sus significados: "+strings.Join(rel.Compartidas, ", ")+".")
	}
	for _, s := range []habla.Significado{rel.SignificadoA, rel.SignificadoB} {
		if s.Propio {
			ps = append(ps, lineaDefinicion(s))
		} else {
			ps = append(ps, fmt.Sprintf("«%s» no tiene definición propia: %s.", s.Palabra, s.Origen))
		}
	}
	ps = append(ps, fmt.Sprintf("Lo calculo comparando las palabras de sus definiciones (parecido %s de 1); no lo he comprobado.", decimal(rel.Coseno)))
	r := plantillaHablaR(texto, ps, []string{"Háblame de " + a.palabra, "¿Qué significa " + a.otra + "?"}, true)
	r.Justificacion = []string{
		fmt.Sprintf("Cada palabra es un vector con las palabras de su definición; el parecido (coseno) es %s.", decimal(rel.Coseno)),
		fmt.Sprintf("Sentido: %s %s y %s %s (traer, guardar, crear… suman; perder, quitar, olvidar… quitan).", x, sentido(rel.PolaridadA), y, sentido(rel.PolaridadB)),
	}
	return r
}

func sentido(p int) string {
	switch {
	case p > 0:
		return "suma"
	case p < 0:
		return "quita"
	}
	return "no suma ni quita"
}

func comillas(ws []string) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = "«" + w + "»"
	}
	return out
}

// respuestaDecir: a sentence of its own about a topic, built only with words whose meaning it knows.
func (c *Cerebro) respuestaDecir(k *tarea, tema string) nucleo.Respuesta {
	tema = quitarArticulo(tema)
	paso := k.raiz.Sub(nucleo.PasoIntento, "Busco una frase sobre «%s» con lo que he leído", tema)
	semilla := k.semilla
	for _, r := range k.p.Tarea + k.p.Sesion {
		semilla = semilla*31 + int64(r)
	}
	rng := rand.New(rand.NewSource(semilla))
	// "dime otra frase": not the sentence it has just said
	anterior := ""
	if prev, ok := k.s.ultimo(); ok && prev.Respuesta.Metodo == metodoHabla {
		anterior = prev.Respuesta.Texto
	}
	f, ok := c.habla.Decir(rng, []string{tema}, habla.MaxPalabrasDir)
	for i := 0; ok && i < 4 && anterior != "" && strings.Contains(anterior, f.Texto); i++ {
		f, ok = c.habla.Decir(rng, []string{tema}, habla.MaxPalabrasDir)
	}
	var def []string
	for _, w := range strings.Fields(tema) {
		if s, ok := c.habla.Buscar(w); ok && s.Propio && !articuloHabla[nucleo.Normalizar(w)] {
			def = append(def, lineaDefinicion(s))
			break
		}
	}
	if !ok {
		paso.Info("No me sale ninguna frase (%d probadas)", f.Probadas)
		ps := []string{}
		switch f.Motivo {
		case "sin_significado":
			ps = append(ps, fmt.Sprintf("He leído frases con «%s», pero no sé qué significa: dímelo así: «%s significa …».", tema, tema))
		case "sin_lectura":
			ps = append(ps, "Todavía no he leído ninguna frase sobre eso.")
		default:
			ps = append(ps, "Las frases que me salen usan palabras que no sé qué significan, y esas no las digo.")
		}
		if len(def) > 0 {
			ps = append(ps, "Lo que sí sé es qué significa: "+def[0])
		}
		ps = append(ps, "Dame un texto con «lee esto: …» y aprenderé a hablar de ello.")
		return plantillaHablaR(fmt.Sprintf("Todavía no sé decir nada sobre «%s»: léeme algo o enséñame qué significa.", tema), ps,
			[]string{"¿Qué significa " + tema + "?", "¿Qué palabras sabes?"}, false)
	}
	paso.Bien("Me quedo con: %s", f.Texto)
	e := c.habla.Estado()
	texto := "Con mis propias palabras (no lo he comprobado): " + afirmar(f.Texto, nil)
	ps := append([]string(nil), def...)
	ps = append(ps, fmt.Sprintf("He formado la frase con lo que he leído (%d frases): la elegí entre %d que solo usan palabras cuyo significado conozco. Puede parecerse mucho a algo que leí.", e.Frases, f.Validas))
	if len(f.Ancladas) > 0 {
		ps = append(ps, "Estas palabras solo las conozco de oídas (sin definición propia): "+strings.Join(comillas(f.Ancladas), ", ")+".")
	}
	r := plantillaHablaR(texto, ps, []string{"Dime otra frase sobre " + tema, "¿Qué significa " + tema + "?"}, true)
	r.Comprobaciones = []nucleo.Comprobacion{{Nivel: nucleo.NivelSinComprobar, Texto: "Frase mía, sin comprobar"}}
	r.Justificacion = []string{
		fmt.Sprintf("Probé %d frases siguiendo lo que he leído (un modelo de trigramas de palabras); %d usaban solo palabras que conozco.", f.Probadas, f.Validas),
		fmt.Sprintf("Me quedé con una de las más probables que habla de «%s» (puntuación %s).", tema, decimal(f.Puntos)),
	}
	return r
}

// respuestaLeer: "lee esto: …" reads a text the user gives (meanings and sentences).
func (c *Cerebro) respuestaLeer(k *tarea, texto string) nucleo.Respuesta {
	if strings.TrimSpace(texto) == "" {
		return plantillaHablaR("¿Qué quieres que lea?", []string{"Escríbelo así: «lee esto: una variable es una caja con nombre donde guardas un valor»."}, nil, false)
	}
	paso := k.raiz.Sub(nucleo.PasoMemoria, "Leo el texto para aprender")
	l := c.habla.Leer(texto, habla.FuenteUsuario, "lo leí en un texto que me diste el "+c.fechaHoy())
	des := c.habla.Desconocidas(texto)
	if err := c.habla.Guardar(); err != nil {
		paso.Mal("No pude guardarlo: %s", mensajeError(err))
	} else {
		paso.Bien("%d frases, %d significados nuevos", l.Frases, len(l.Nuevos))
	}
	textoR := fmt.Sprintf("He leído %s.", cuantas(l.Frases, "frase", "frases"))
	var ps []string
	if len(l.Nuevos) > 0 {
		var ws []string
		for _, s := range l.Nuevos {
			ws = append(ws, s.Palabra)
		}
		textoR += " Aprendí qué significan: " + strings.Join(comillas(ws), ", ") + "."
	}
	ps = append(ps, fmt.Sprintf("Uso %s para aprender a hablar.", cuantas(l.Entrenadas, "frase", "frases")))
	if len(des) > 0 {
		if len(des) > 8 {
			des = des[:8]
		}
		ps = append(ps, fmt.Sprintf("No sé qué significan: %s. Enséñamelas así: «%s significa …».", strings.Join(comillas(des), ", "), des[0]))
	}
	r := plantillaHablaR(textoR, ps, nil, true)
	if len(l.Nuevos) > 0 {
		tb := nucleo.Tabla{Titulo: "Lo que aprendí", Cabecera: []string{"Palabra", "Significado"}}
		for _, s := range l.Nuevos {
			tb.Filas = append(tb.Filas, []string{s.Palabra, s.Definicion})
		}
		r.Tablas = []nucleo.Tabla{tb}
		r.Aprendido = "Lo he guardado en mi diccionario (como leído, no comprobado)"
		r.Sugerencias = []string{"¿Qué significa " + l.Nuevos[0].Palabra + "?", "Háblame de " + l.Nuevos[0].Palabra}
	} else if len(des) > 0 {
		r.Sugerencias = []string{"¿Qué palabras sabes?"}
	}
	return r
}

// respuestaEstadoHabla: "¿qué palabras sabes?" as a short table (plus the learned lexicon words).
func (c *Cerebro) respuestaEstadoHabla(k *tarea) nucleo.Respuesta {
	e := c.habla.Estado()
	k.raiz.Sub(nucleo.PasoMemoria, "Cuento lo que sé de palabras").Info("")
	texto := fmt.Sprintf("Sé lo que significan %d palabras: %d del diccionario de Nyx, %d que me enseñaste y %d que leí.",
		e.ConSignificado, e.DeSemilla, e.DelUsuario, e.Leidos)
	fluidez := "todavía sin datos"
	if e.HayFluidez {
		fluidez = fmt.Sprintf("perplejidad %d (más baja es mejor)", int(math.Round(math.Exp(-e.Fluidez))))
	}
	tb := nucleo.Tabla{Titulo: "Lo que sé de palabras", Cabecera: []string{"Qué", "Cuántas"}, Filas: [][]string{
		{"Palabras con significado propio", strconv.Itoa(e.ConSignificado)},
		{"Palabras ancladas (sin definición propia)", strconv.Itoa(e.Anclados)},
		{"Palabras distintas que he leído", strconv.Itoa(e.Vocabulario)},
		{"Frases leídas para aprender a hablar", strconv.Itoa(e.Frases)},
		{"Trozos de frase que recuerdo (n-gramas)", strconv.Itoa(e.Ngramas)},
		{"Fluidez", fluidez},
	}}
	r := plantillaHablaR(texto, []string{
		"Pregúntame «¿qué significa X?», enséñame con «X significa …» o dame un texto con «lee esto: …».",
	}, []string{"¿Qué significa variable?", "Háblame de las funciones", "¿Qué relación hay entre aprender y olvidar?"}, true)
	r.Tablas = []nucleo.Tabla{tb}
	var aprendidas []habla.Significado
	for _, s := range c.habla.Lista() {
		if s.Fuente != habla.FuenteSemilla {
			aprendidas = append(aprendidas, s)
		}
	}
	if len(aprendidas) > 0 {
		sort.SliceStable(aprendidas, func(i, j int) bool { return aprendidas[i].Fecha.After(aprendidas[j].Fecha) })
		ta := nucleo.Tabla{Titulo: "Significados que he aprendido", Cabecera: []string{"Palabra", "Significado", "De dónde"}}
		for _, s := range aprendidas {
			ta.Filas = append(ta.Filas, []string{s.Palabra, recortar(s.Definicion, 80), strings.TrimSuffix(deDonde(s), ".")})
		}
		r.Tablas = append(r.Tablas, ta)
	}
	if ps := c.mem.Palabras(); len(ps) > 0 {
		var claves []string
		for p := range ps {
			claves = append(claves, p)
		}
		sort.Strings(claves)
		tl := nucleo.Tabla{Titulo: "Palabras que he aprendido para programar", Cabecera: []string{"Palabra", "Quiere decir"}}
		for _, p := range claves {
			tl.Filas = append(tl.Filas, []string{p, strings.ReplaceAll(ps[p], "_", " ")})
		}
		r.Tablas = append(r.Tablas, tl)
	}
	return r
}

// ---- learning ----

// hablaEnsenar: "X significa Y" also goes to the dictionary. When logic and the lexicon could not
// store it, this alone answers.
func (c *Cerebro) hablaEnsenar(k *tarea, r *nucleo.Respuesta, a pedidoHabla) {
	s, err := c.habla.Ensenar(a.palabra, a.definicion, habla.FuenteUsuario, "me lo enseñaste tú el "+c.fechaHoy())
	if err != nil {
		k.raiz.Sub(nucleo.PasoMemoria, "No lo guardo en mi diccionario: %s", err.Error()).Info("")
		return
	}
	_ = c.habla.GuardarSiHaceFalta(intervaloHabla)
	k.raiz.Sub(nucleo.PasoMemoria, "Guardo en mi diccionario: «%s» significa «%s»", s.Palabra, s.Definicion).Bien("")
	frase := fmt.Sprintf("Guardo en mi diccionario que «%s» significa «%s»", s.Palabra, s.Definicion)
	if r.Exito {
		if r.Aprendido != "" {
			r.Aprendido += ". " + frase
		} else {
			r.Aprendido = frase
		}
		return
	}
	k.pendiente = ""
	k.fallidas = nil
	k.estrategia = "-"
	k.intencion = nucleo.IMemoria
	*r = plantillaHablaR(fmt.Sprintf("Entendido: «%s» significa «%s».", s.Palabra, s.Definicion),
		[]string{fmt.Sprintf("Lo he guardado en mi diccionario. Pregúntamelo con «¿qué significa %s?».", s.Palabra)},
		[]string{"¿Qué significa " + s.Palabra + "?", "Háblame de " + s.Palabra}, true)
	r.Aprendido = frase
}

// hablaDespues reads every request (without its code) to learn meanings and to talk, and saves when
// it is time. It runs after the answer and never fails the request.
func (c *Cerebro) hablaDespues(k *tarea) {
	if c.habla == nil {
		return
	}
	defer func() { _ = recover() }()
	if k.p.Accion == "" && strings.TrimSpace(k.p.Codigo) == "" && detectarHabla(k.p.Texto).tipo != "leer" {
		_, resto := lengua.ExtraerCodigo(k.p.Texto)
		resto = reHablaOrden.ReplaceAllString(strings.TrimSpace(resto), "")
		if strings.TrimSpace(resto) != "" {
			c.habla.Leer(resto, habla.FuenteUsuario, "lo leí en lo que me escribiste el "+c.fechaHoy())
		}
	}
	_ = c.habla.GuardarSiHaceFalta(intervaloHabla)
}

// palabrasHabla is how many words have their own meaning and are not already lexicon words.
func (c *Cerebro) palabrasHabla(lexico map[string]string) int {
	n := 0
	for _, s := range c.habla.Lista() {
		if _, ya := lexico[nucleo.Normalizar(s.Palabra)]; !ya {
			n++
		}
	}
	return n
}

// palabrasConSignificado are the lexicon words (word → concept) plus the learned meanings
// (word → definition; not the seed ones), for the Memoria tab.
func (c *Cerebro) palabrasConSignificado() map[string]string {
	out := map[string]string{}
	for k, v := range c.mem.Palabras() {
		out[k] = v
	}
	for _, s := range c.habla.Lista() {
		if s.Fuente == habla.FuenteSemilla {
			continue
		}
		if _, ya := out[nucleo.Normalizar(s.Palabra)]; ya {
			continue
		}
		out[s.Palabra] = s.Definicion
	}
	return out
}

// olvidarPalabra forgets a lexicon word and its meaning; it fails only when neither existed.
func (c *Cerebro) olvidarPalabra(id string) error {
	errMem := c.mem.OlvidarPalabra(id)
	errHabla := c.habla.Olvidar(id)
	if errHabla == nil {
		_ = c.habla.GuardarSiHaceFalta(intervaloHabla)
	}
	if errMem == nil || errHabla == nil {
		return nil
	}
	return errMem
}

// hablaConoce: the word has a learned meaning (for "olvida la palabra X").
func (c *Cerebro) hablaConoce(palabra string) bool {
	s, ok := c.habla.Buscar(palabra)
	return ok && s.Propio && s.Fuente != habla.FuenteSemilla && nucleo.Normalizar(s.Palabra) == nucleo.Normalizar(palabra)
}
