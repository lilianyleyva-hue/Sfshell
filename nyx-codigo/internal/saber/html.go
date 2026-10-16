package saber

import (
	"context"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Our own HTML tokenizer (a small state machine over '<', '>', attribute quoting and comments) and
// the document extractor built on it. Nothing here executes or renders fetched text: it only becomes
// plain strings in Bloque values.

// Bloque is one piece of an extracted page.
type Bloque struct {
	Tipo     string `json:"tipo"` // "titulo" | "parrafo" | "codigo"
	Texto    string `json:"texto"`
	Contexto string `json:"contexto,omitempty"` // preceding paragraph for code
	Votos    int    `json:"votos,omitempty"`    // Stack Overflow answer score if detectable
}

// Documento is an extracted page (what the cache stores).
type Documento struct {
	URL     string    `json:"url"`
	Titulo  string    `json:"titulo"`
	Bloques []Bloque  `json:"bloques"`
	Fecha   time.Time `json:"fecha"`
}

// Extraction bounds.
const (
	maxBloques  = 400
	maxCodigo   = 32 << 10 // bytes per code block
	maxParrafo  = 2000     // runes per paragraph
	minSuelto   = 25       // loose text (outside p/li) needs at least this many runes to count
	maxTokens   = 400000   // tokens per page (a 2 MiB page has far fewer)
	maxAtributo = 4096
	maxPila     = 256  // open elements kept on the stack; deeper start tags are ignored
	maxBusqueda = 64   // how far down the stack a closing or implicitly closing tag looks
	cadaTokens  = 1024 // ctx is checked every this many tokens
)

// ---- tokenizer ----

type tipoToken int

const (
	tokTexto tipoToken = iota
	tokInicio
	tokFin
	tokComentario
)

type atributo struct{ nombre, valor string }

type tokenHTML struct {
	tipo   tipoToken
	nombre string // lowercase tag name
	attrs  []atributo
	texto  string // raw text (entities NOT decoded) for tokTexto
	auto   bool   // self-closing "<br/>"
}

func (t tokenHTML) attr(nombre string) (string, bool) {
	for _, a := range t.attrs {
		if a.nombre == nombre {
			return a.valor, true
		}
	}
	return "", false
}

// tieneClase reports whether the class attribute contains c as a whole word.
func (t tokenHTML) tieneClase(c string) bool {
	v, ok := t.attr("class")
	if !ok {
		return false
	}
	for _, x := range strings.Fields(v) {
		if x == c {
			return true
		}
	}
	return false
}

// elementos de texto crudo: su contenido llega como un solo texto hasta la etiqueta de cierre.
var textoCrudo = map[string]bool{"script": true, "style": true, "textarea": true, "title": true, "xmp": true, "noscript": true}

type lectorHTML struct {
	s     string
	i     int
	crudo string // pending raw-text element name
}

func esLetra(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// siguiente returns the next token; ok is false at the end of input.
func (l *lectorHTML) siguiente() (tokenHTML, bool) {
	if l.i >= len(l.s) {
		return tokenHTML{}, false
	}
	if l.crudo != "" {
		nombre := l.crudo
		l.crudo = ""
		fin := indiceSinMayus(l.s[l.i:], "</"+nombre)
		if fin < 0 {
			t := tokenHTML{tipo: tokTexto, texto: l.s[l.i:]}
			l.i = len(l.s)
			return t, true
		}
		t := tokenHTML{tipo: tokTexto, texto: l.s[l.i : l.i+fin]}
		l.i += fin
		if t.texto != "" {
			return t, true
		}
		return l.siguiente()
	}
	s := l.s
	if s[l.i] != '<' {
		j := l.i + 1
		for j < len(s) {
			if s[j] == '<' && l.abreEtiqueta(j) {
				break
			}
			j++
		}
		t := tokenHTML{tipo: tokTexto, texto: s[l.i:j]}
		l.i = j
		return t, true
	}
	if !l.abreEtiqueta(l.i) {
		l.i++
		return tokenHTML{tipo: tokTexto, texto: "<"}, true
	}
	switch {
	case strings.HasPrefix(s[l.i:], "<!--"):
		fin := strings.Index(s[l.i+4:], "-->")
		var t tokenHTML
		if fin < 0 {
			t = tokenHTML{tipo: tokComentario, texto: s[l.i+4:]}
			l.i = len(s)
		} else {
			t = tokenHTML{tipo: tokComentario, texto: s[l.i+4 : l.i+4+fin]}
			l.i += 4 + fin + 3
		}
		return t, true
	case s[l.i+1] == '!' || s[l.i+1] == '?':
		fin := strings.IndexByte(s[l.i:], '>')
		if fin < 0 {
			l.i = len(s)
		} else {
			l.i += fin + 1
		}
		return tokenHTML{tipo: tokComentario}, true
	case s[l.i+1] == '/':
		j := l.i + 2
		k := j
		for k < len(s) && (esLetra(s[k]) || (s[k] >= '0' && s[k] <= '9') || s[k] == '-' || s[k] == ':') {
			k++
		}
		nombre := strings.ToLower(s[j:k])
		fin := strings.IndexByte(s[k:], '>')
		if fin < 0 {
			l.i = len(s)
		} else {
			l.i = k + fin + 1
		}
		return tokenHTML{tipo: tokFin, nombre: nombre}, true
	}
	return l.etiquetaInicio(), true
}

// abreEtiqueta: '<' followed by a letter, '/' + letter, '!' or '?'.
func (l *lectorHTML) abreEtiqueta(i int) bool {
	if i+1 >= len(l.s) {
		return false
	}
	c := l.s[i+1]
	if esLetra(c) || c == '!' || c == '?' {
		return true
	}
	return c == '/' && i+2 < len(l.s) && esLetra(l.s[i+2])
}

func (l *lectorHTML) etiquetaInicio() tokenHTML {
	s := l.s
	j := l.i + 1
	k := j
	for k < len(s) && (esLetra(s[k]) || (s[k] >= '0' && s[k] <= '9') || s[k] == '-' || s[k] == ':') {
		k++
	}
	t := tokenHTML{tipo: tokInicio, nombre: strings.ToLower(s[j:k])}
	i := k
	for i < len(s) {
		for i < len(s) && esEspacioASCII(s[i]) {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == '>' {
			i++
			break
		}
		if s[i] == '/' {
			if i+1 < len(s) && s[i+1] == '>' {
				t.auto = true
				i += 2
				break
			}
			i++
			continue
		}
		a := i
		for i < len(s) && !esEspacioASCII(s[i]) && s[i] != '=' && s[i] != '>' && !(s[i] == '/' && i+1 < len(s) && s[i+1] == '>') {
			i++
		}
		nombre := strings.ToLower(s[a:i])
		for i < len(s) && esEspacioASCII(s[i]) {
			i++
		}
		valor := ""
		if i < len(s) && s[i] == '=' {
			i++
			for i < len(s) && esEspacioASCII(s[i]) {
				i++
			}
			if i < len(s) && (s[i] == '"' || s[i] == '\'') {
				q := s[i]
				fin := strings.IndexByte(s[i+1:], q)
				if fin < 0 {
					valor = s[i+1:]
					i = len(s)
				} else {
					valor = s[i+1 : i+1+fin]
					i += fin + 2
				}
			} else {
				b := i
				for i < len(s) && !esEspacioASCII(s[i]) && s[i] != '>' {
					i++
				}
				valor = s[b:i]
			}
		}
		if nombre == "" {
			if i < len(s) {
				i++
			}
			continue
		}
		if len(valor) > maxAtributo {
			valor = valor[:maxAtributo]
		}
		if len(t.attrs) < 32 {
			t.attrs = append(t.attrs, atributo{nombre, html.UnescapeString(valor)})
		}
	}
	l.i = i
	if textoCrudo[t.nombre] && !t.auto {
		l.crudo = t.nombre
	}
	return t
}

func esEspacioASCII(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }

// indiceSinMayus is strings.Index ignoring ASCII case of the pattern (which is lowercase).
func indiceSinMayus(s, patron string) int {
	n := len(patron)
	for i := 0; i+n <= len(s); i++ {
		ok := true
		for j := 0; j < n; j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != patron[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// ---- extractor ----

var (
	omitidos = map[string]bool{"script": true, "style": true, "nav": true, "footer": true, "header": true,
		"noscript": true, "svg": true, "template": true, "iframe": true, "button": true, "select": true}
	vacios = map[string]bool{"br": true, "img": true, "hr": true, "input": true, "meta": true, "link": true,
		"area": true, "base": true, "col": true, "embed": true, "source": true, "track": true, "wbr": true, "param": true}
	bloquesHTML = map[string]bool{"p": true, "li": true, "ul": true, "ol": true, "div": true, "section": true,
		"article": true, "main": true, "aside": true, "table": true, "tr": true, "td": true, "th": true,
		"tbody": true, "thead": true, "dl": true, "dd": true, "dt": true, "blockquote": true, "figure": true,
		"figcaption": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"pre": true, "details": true, "summary": true, "body": true, "form": true, "fieldset": true, "address": true}
	parrafos = map[string]bool{"p": true, "li": true, "dd": true, "dt": true, "blockquote": true, "td": true,
		"th": true, "figcaption": true, "summary": true}
	titulos = map[string]bool{"h1": true, "h2": true, "h3": true, "h4": true}
)

type bloqueCrudo struct {
	Bloque
	shell bool // code that looks like a shell session ("$ go run …")
}

type extractor struct {
	pila      []string
	nTitulos  int // open elements of pila in titulos (so clasificar does not scan the stack)
	nParrafos int // open elements of pila in parrafos
	omitir    int
	texto     strings.Builder
	enPre     int
	codigo    strings.Builder
	enCode    int
	marcaCode int // texto length when an inline <code> opened
	bufCode   strings.Builder
	contexto  string
	votos     int
	capVotos  bool
	titulo    string
	h1        string
	bloques   []bloqueCrudo
	// sección: with ancla != "", nothing is kept until an element with that id opens, and keeping stops
	// at the next heading after it.
	ancla        string
	activo       bool
	cerrado      bool
	nivelAncla   int
	anclaTitulo  bool
	anclaAbierta bool
	anclaCerrada bool
}

// ExtraerDocumento extracts titles, paragraphs and code blocks from an HTML page. It drops script,
// style, nav, footer and header; h1-h3 (and h4, used by pkg.go.dev) become titles; p and li become
// paragraphs; pre and multi-line code become code blocks (entities decoded, line numbers and "$ "
// prompts removed). Stack Overflow votes come from data-score or itemprop="upvoteCount".
func ExtraerDocumento(pagina []byte, direccion string) Documento {
	d, _ := extraer(context.Background(), pagina, direccion, "")
	return d
}

// ExtraerSeccion is ExtraerDocumento limited to the part of the page that starts at the element whose
// id is ancla and ends at the next heading (used for a symbol on pkg.go.dev).
func ExtraerSeccion(pagina []byte, direccion, ancla string) Documento {
	d, _ := extraer(context.Background(), pagina, direccion, ancla)
	return d
}

// extraer does the work of ExtraerDocumento/ExtraerSeccion. Every step is bounded (stack depth, search
// depth, token count), so the cost is linear in the page; ctx is still checked every cadaTokens tokens and
// a cancelled extraction returns what it has together with ctx.Err().
func extraer(ctx context.Context, pagina []byte, direccion, ancla string) (Documento, error) {
	s := string(pagina)
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	e := &extractor{ancla: ancla, activo: ancla == ""}
	l := &lectorHTML{s: s}
	enTitulo := false
	var errCtx error
	for n := 0; n < maxTokens; n++ {
		if n%cadaTokens == cadaTokens-1 {
			if err := ctx.Err(); err != nil {
				errCtx = err
				break
			}
		}
		t, ok := l.siguiente()
		if !ok {
			break
		}
		if t.tipo == tokInicio && t.nombre == "title" && e.omitir == 0 {
			enTitulo = true
			continue
		}
		if enTitulo {
			if t.tipo == tokTexto && e.titulo == "" {
				e.titulo = colapsar(html.UnescapeString(t.texto))
			}
			if t.tipo == tokFin && t.nombre == "title" || t.tipo == tokInicio {
				enTitulo = false
			}
			if t.tipo == tokTexto || t.tipo == tokFin {
				continue
			}
		}
		e.token(t)
		if len(e.bloques) >= maxBloques*2 || e.cerrado {
			break
		}
	}
	e.vaciar("")
	d := Documento{URL: direccion, Titulo: e.titulo}
	if d.Titulo == "" {
		d.Titulo = e.h1
	}
	d.Bloques = fusionarSegmentos(e.bloques)
	if len(d.Bloques) > maxBloques {
		d.Bloques = d.Bloques[:maxBloques]
	}
	return d, errCtx
}

// meter pushes an open element. The stack never grows past maxPila, so a page made of thousands of
// unclosed elements cannot make each later step slower: on a full stack the new element takes the place
// of the top one (as browsers flatten over-deep trees), unless the top one is a script-like, code or pre
// element whose counters must stay right; then the new element is ignored and meter returns false.
func (e *extractor) meter(n string) bool {
	if len(e.pila) >= maxPila {
		tope := e.pila[len(e.pila)-1]
		if omitidos[tope] || tope == "code" || tope == "pre" {
			return false
		}
		e.cortar(len(e.pila) - 1)
	}
	e.pila = append(e.pila, n)
	if titulos[n] {
		e.nTitulos++
	}
	if parrafos[n] {
		e.nParrafos++
	}
	return true
}

// cortar pops every element from position i up.
func (e *extractor) cortar(i int) {
	if i < 0 {
		i = 0
	}
	for _, x := range e.pila[i:] {
		if titulos[x] {
			e.nTitulos--
		}
		if parrafos[x] {
			e.nParrafos--
		}
	}
	e.pila = e.pila[:i]
}

func (e *extractor) clasificar() string {
	if e.nTitulos > 0 {
		return "titulo"
	}
	if e.nParrafos > 0 {
		return "parrafo"
	}
	return "suelto"
}

// vaciar turns the pending inline text into a block of the current class.
func (e *extractor) vaciar(clase string) {
	if e.texto.Len() == 0 {
		return
	}
	txt := colapsar(e.texto.String())
	e.texto.Reset()
	if txt == "" || !e.activo {
		return
	}
	if clase == "" {
		clase = e.clasificar()
	}
	txt = recortarRunas(txt, maxParrafo)
	switch clase {
	case "titulo":
		e.agregar(bloqueCrudo{Bloque: Bloque{Tipo: "titulo", Texto: txt}})
		if e.h1 == "" {
			e.h1 = txt
		}
	case "parrafo":
		e.agregar(bloqueCrudo{Bloque: Bloque{Tipo: "parrafo", Texto: txt}})
		e.contexto = txt
	default:
		if utf8.RuneCountInString(txt) >= minSuelto {
			e.agregar(bloqueCrudo{Bloque: Bloque{Tipo: "parrafo", Texto: txt}})
			e.contexto = txt
		}
	}
}

func (e *extractor) agregar(b bloqueCrudo) {
	if len(e.bloques) < maxBloques*2 {
		e.bloques = append(e.bloques, b)
	}
}

func (e *extractor) emitirCodigo(crudo string) {
	if !e.activo {
		return
	}
	shell := esShell(crudo)
	cod := limpiarCodigo(crudo)
	if strings.TrimSpace(cod) == "" || len(cod) > maxCodigo {
		return
	}
	e.agregar(bloqueCrudo{Bloque: Bloque{Tipo: "codigo", Texto: cod, Contexto: e.contexto, Votos: e.votos}, shell: shell})
}

func (e *extractor) token(t tokenHTML) {
	switch t.tipo {
	case tokComentario:
		return
	case tokTexto:
		if e.omitir > 0 {
			return
		}
		txt := html.UnescapeString(t.texto)
		if e.capVotos {
			if v, err := strconv.Atoi(strings.TrimSpace(txt)); err == nil {
				e.votos = v
				e.capVotos = false
			} else if strings.TrimSpace(txt) != "" {
				e.capVotos = false
			}
		}
		if e.enPre > 0 {
			e.codigo.WriteString(txt)
			return
		}
		if e.enCode > 0 {
			e.bufCode.WriteString(txt)
		}
		e.texto.WriteString(txt)
	case tokInicio:
		e.inicio(t)
	case tokFin:
		e.fin(t.nombre)
	}
}

func (e *extractor) inicio(t tokenHTML) {
	n := t.nombre
	if e.omitir > 0 {
		if !vacios[n] && !t.auto && e.meter(n) && omitidos[n] {
			e.omitir++
		}
		return
	}
	// votes (Stack Overflow)
	if v, ok := t.attr("data-score"); ok {
		if x, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			e.votos = x
		}
	}
	if ip, _ := t.attr("itemprop"); ip == "upvoteCount" {
		if v, ok := t.attr("data-value"); ok {
			if x, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				e.votos = x
			}
		} else if v, ok := t.attr("content"); ok {
			if x, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				e.votos = x
			}
		} else {
			e.capVotos = true
		}
	}
	// section mode
	if e.ancla != "" {
		if id, _ := t.attr("id"); !e.activo && id == e.ancla {
			e.texto.Reset()
			e.activo = true
			e.nivelAncla = len(e.pila)
			// headings and inline/void anchors mark the start of a section that runs to the next heading;
			// a container anchor (div, section…) holds the section and ends when it closes
			e.anclaTitulo = titulos[n] || !bloquesHTML[n]
			e.anclaAbierta = !vacios[n] && !t.auto
			e.anclaCerrada = !e.anclaAbierta
		} else if e.activo && e.anclaCerrada && titulos[n] {
			e.vaciar("")
			e.cerrado = true
			return
		}
	}
	if omitidos[n] {
		e.vaciar("")
		if !vacios[n] && !t.auto && e.meter(n) {
			e.omitir++
		}
		return
	}
	if e.enPre > 0 {
		if n == "br" {
			e.codigo.WriteByte('\n')
		}
		if !vacios[n] && !t.auto && e.meter(n) && n == "pre" {
			e.enPre++
		}
		return
	}
	switch {
	case n == "br":
		e.texto.WriteByte('\n')
		if e.enCode > 0 {
			e.bufCode.WriteByte('\n')
		}
		return
	case vacios[n] || t.auto:
		if n == "img" {
			if alt, ok := t.attr("alt"); ok && alt != "" && e.enCode == 0 {
				e.texto.WriteString(" " + alt + " ")
			}
		}
		return
	case n == "code":
		if !e.meter(n) {
			return
		}
		e.enCode++
		if e.enCode == 1 {
			e.marcaCode = e.texto.Len()
			e.bufCode.Reset()
		}
		return
	case bloquesHTML[n]:
		e.cerrarImplicitos(n)
		e.vaciar("")
		if e.meter(n) && n == "pre" {
			e.enPre = 1
			e.codigo.Reset()
		}
		return
	}
	e.meter(n) // inline element
}

// cerrarImplicitos closes an open <p> when a block starts, and the previous <li> when a new one starts.
func (e *extractor) cerrarImplicitos(n string) {
	if n == "li" {
		for i := len(e.pila) - 1; i >= 0 && len(e.pila)-i <= maxBusqueda; i-- {
			x := e.pila[i]
			if x == "ul" || x == "ol" {
				break
			}
			if x == "li" {
				e.vaciar("")
				e.cortar(i)
				break
			}
		}
	}
	if n == "p" || n == "li" || titulos[n] || n == "pre" || n == "div" || n == "table" || n == "ul" || n == "ol" {
		for i := len(e.pila) - 1; i >= 0 && len(e.pila)-i <= maxBusqueda; i-- {
			x := e.pila[i]
			if x == "p" {
				e.vaciar("")
				e.cortar(i)
				break
			}
			if bloquesHTML[x] {
				break
			}
		}
	}
}

func (e *extractor) fin(n string) {
	// find the matching open element
	idx := -1
	for i := len(e.pila) - 1; i >= 0; i-- {
		if e.pila[i] == n {
			idx = i
			break
		}
		if len(e.pila)-i > maxBusqueda {
			break
		}
	}
	if idx < 0 {
		return
	}
	if e.omitir > 0 {
		for i := len(e.pila) - 1; i >= idx; i-- {
			if omitidos[e.pila[i]] {
				e.omitir--
			}
		}
		e.cortar(idx)
		if e.omitir < 0 {
			e.omitir = 0
		}
		return
	}
	cerradas := e.pila[idx:]
	if e.enPre > 0 {
		for _, x := range cerradas {
			if x == "pre" {
				e.enPre--
			}
		}
		e.cortar(idx)
		if e.enPre <= 0 {
			e.enPre = 0
			e.emitirCodigo(e.codigo.String())
			e.codigo.Reset()
		}
		return
	}
	for _, x := range cerradas {
		if x == "code" && e.enCode > 0 {
			e.enCode--
			if e.enCode == 0 {
				cod := e.bufCode.String()
				if strings.Count(strings.TrimSpace(cod), "\n") >= 1 {
					// multi-line inline code: a code block of its own
					txt := e.texto.String()
					if e.marcaCode <= len(txt) {
						e.texto.Reset()
						e.texto.WriteString(txt[:e.marcaCode])
					}
					e.vaciar("")
					e.emitirCodigo(cod)
				}
				e.bufCode.Reset()
			}
		}
	}
	bloque := false
	for _, x := range cerradas {
		if bloquesHTML[x] {
			bloque = true
		}
	}
	if bloque {
		e.vaciar("")
	}
	e.cortar(idx)
	if e.activo && e.ancla != "" && !e.cerrado {
		switch {
		case idx < e.nivelAncla: // the element that held the section closed
			e.vaciar("")
			e.cerrado = true
		case idx == e.nivelAncla && e.anclaAbierta:
			e.anclaAbierta = false
			if e.anclaTitulo {
				e.anclaCerrada = true // keep the section until the next heading
			} else {
				e.vaciar("")
				e.cerrado = true
			}
		}
	}
}

// ---- text helpers ----

// colapsar collapses white space (including NBSP) to single spaces and trims.
func colapsar(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.Join(strings.Fields(s), " ")
}

func recortarRunas(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for j := range s {
		if i == n {
			return s[:j] + "…"
		}
		i++
	}
	return s
}

var (
	reNumeroLinea = regexp.MustCompile(`^\s*\d+(\s|$)`)
	reQuitarNum   = regexp.MustCompile(`^\s*\d+ ?`)
)

// esShell: the first non-empty line starts with a "$" prompt.
func esShell(s string) bool {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(strings.ReplaceAll(l, " ", " "))
		if l == "" {
			continue
		}
		return l == "$" || strings.HasPrefix(l, "$ ")
	}
	return false
}

// limpiarCodigo normalizes a code block: CRLF, NBSP, trailing spaces, line numbers (when most lines
// carry one), "$ " prompts and blank edges.
func limpiarCodigo(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, " ", " ")
	lineas := strings.Split(s, "\n")
	noVacias, conNumero := 0, 0
	for i, l := range lineas {
		l = strings.TrimRight(l, " \t")
		lineas[i] = l
		if strings.TrimSpace(l) == "" {
			continue
		}
		noVacias++
		if reNumeroLinea.MatchString(l) {
			conNumero++
		}
	}
	numerado := conNumero >= 2 && conNumero*10 >= noVacias*6 && numerosCrecientes(lineas)
	if numerado {
		for i, l := range lineas {
			lineas[i] = reQuitarNum.ReplaceAllString(l, "")
		}
		lineas = desindentar(lineas)
	}
	for i, l := range lineas {
		t := strings.TrimLeft(l, " \t")
		if t == "$" {
			lineas[i] = ""
		} else if strings.HasPrefix(t, "$ ") {
			lineas[i] = t[2:]
		}
	}
	for len(lineas) > 0 && strings.TrimSpace(lineas[0]) == "" {
		lineas = lineas[1:]
	}
	for len(lineas) > 0 && strings.TrimSpace(lineas[len(lineas)-1]) == "" {
		lineas = lineas[:len(lineas)-1]
	}
	return strings.Join(lineas, "\n")
}

func numerosCrecientes(lineas []string) bool {
	prev := -1
	for _, l := range lineas {
		m := reNumeroLinea.FindString(l)
		if m == "" {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(m))
		if err != nil {
			return false
		}
		if v <= prev {
			return false
		}
		prev = v
	}
	return true
}

func desindentar(lineas []string) []string {
	min := -1
	for _, l := range lineas {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return lineas
	}
	for i, l := range lineas {
		if len(l) >= min {
			lineas[i] = l[min:]
		} else {
			lineas[i] = strings.TrimLeft(l, " ")
		}
	}
	return lineas
}

// fusionarSegmentos joins code that a page split into many pieces (gobyexample shows one program as
// a table of segments): a code block that starts with "package" but is not a complete file absorbs
// the following non-shell code blocks until a shell block, another "package" block or a title.
func fusionarSegmentos(bs []bloqueCrudo) []Bloque {
	out := make([]Bloque, 0, len(bs))
	for i := 0; i < len(bs); i++ {
		b := bs[i]
		if b.Tipo != "codigo" || b.shell || !strings.HasPrefix(strings.TrimSpace(b.Texto), "package ") ||
			(balanceLlaves(b.Texto) == 0 && strings.Contains(b.Texto, "func ")) {
			out = append(out, b.Bloque)
			continue
		}
		var sb strings.Builder
		sb.WriteString(b.Texto)
		var intermedios []Bloque
		j := i + 1
		for ; j < len(bs); j++ {
			c := bs[j]
			if c.Tipo == "titulo" {
				break
			}
			if c.Tipo != "codigo" {
				intermedios = append(intermedios, c.Bloque)
				continue
			}
			if c.shell || strings.HasPrefix(strings.TrimSpace(c.Texto), "package ") {
				break
			}
			sb.WriteString("\n")
			sb.WriteString(c.Texto)
		}
		// paragraphs read in between stay, before the merged program
		unido := b.Bloque
		unido.Texto = sb.String()
		out = append(out, intermedios...)
		out = append(out, unido)
		i = j - 1
	}
	return out
}

// balanceLlaves counts { minus } outside strings, runes and comments.
func balanceLlaves(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '{':
			n++
		case '}':
			n--
		case '"', '\'':
			for i++; i < len(s) && s[i] != c && s[i] != '\n'; i++ {
				if s[i] == '\\' {
					i++
				}
			}
		case '`':
			for i++; i < len(s) && s[i] != '`'; i++ {
			}
		case '/':
			if i+1 < len(s) && s[i+1] == '/' {
				for i < len(s) && s[i] != '\n' {
					i++
				}
			} else if i+1 < len(s) && s[i+1] == '*' {
				fin := strings.Index(s[i+2:], "*/")
				if fin < 0 {
					return n
				}
				i += fin + 3
			}
		}
	}
	return n
}

// ---- link extraction ----

type enlaceHTML struct {
	href, texto string
	enLista     bool // inside an <li>
}

// enlacesHTML returns every <a href> of a page with its text (entities decoded, spaces collapsed).
func enlacesHTML(pagina []byte) []enlaceHTML {
	l := &lectorHTML{s: string(pagina)}
	var out []enlaceHTML
	actual := -1
	prof := 0
	listas := 0
	omitir := 0
	var sb strings.Builder
	for n := 0; n < maxTokens; n++ {
		t, ok := l.siguiente()
		if !ok {
			break
		}
		switch t.tipo {
		case tokInicio:
			if (t.nombre == "script" || t.nombre == "style") && !t.auto {
				omitir++
				continue
			}
			if t.nombre == "li" && !t.auto {
				listas++
			}
			if t.nombre == "a" {
				if actual >= 0 {
					out[actual].texto = colapsar(sb.String())
				}
				href, _ := t.attr("href")
				out = append(out, enlaceHTML{href: href, enLista: listas > 0})
				actual = len(out) - 1
				prof = 1
				sb.Reset()
			} else if actual >= 0 && !vacios[t.nombre] && !t.auto {
				prof++
			}
		case tokFin:
			if t.nombre == "script" || t.nombre == "style" {
				if omitir > 0 {
					omitir--
				}
				continue
			}
			if t.nombre == "li" && listas > 0 {
				listas--
			}
			if actual >= 0 {
				if t.nombre == "a" {
					prof = 0
				} else {
					prof--
				}
				if prof <= 0 {
					out[actual].texto = colapsar(sb.String())
					actual = -1
				}
			}
		case tokTexto:
			if actual >= 0 && omitir == 0 {
				sb.WriteString(html.UnescapeString(t.texto))
			}
		}
	}
	if actual >= 0 {
		out[actual].texto = colapsar(sb.String())
	}
	return out
}
