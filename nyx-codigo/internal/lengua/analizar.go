package lengua

import (
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Analizar parses a request (§4.8): code and text, tokens with lemmas, numbers, example pairs and
// program cases, the frame, a probable signature (and the typed examples when it has one), concepts,
// intent scores (best first) and the English web query.
//
// codigo is the separate code box: when it is not empty it wins and the whole text is the request.
// Otherwise the code is extracted from the text. Token and number offsets refer to texto.
func Analizar(texto, codigo, modo string, l *Lexico, ctx *nucleo.Contexto) *nucleo.Pregunta {
	if l == nil {
		l = LexicoBase()
	}
	p := &nucleo.Pregunta{Texto: texto, Modo: modo, Contexto: ctx}
	visible := texto
	if strings.TrimSpace(codigo) != "" {
		p.Codigo = codigo
		p.Resto = strings.TrimSpace(texto)
	} else if c, rangos := extraerCodigo(texto); len(rangos) > 0 {
		p.Codigo = c
		p.Resto = strings.TrimSpace(quitarRangos(texto, rangos, false))
		visible = quitarRangos(texto, rangos, true)
	} else {
		p.Resto = strings.TrimSpace(texto)
	}
	p.Normal = nucleo.Normalizar(p.Resto)
	p.Tokens = Lematizar(visible)
	p.Numeros = Numeros(visible)
	if p.Numeros == nil {
		p.Numeros = []nucleo.Numero{}
	}
	p.ParesTexto, p.EjProgramas = ExtraerEjemplos(p.Resto)
	p.Marco = Marco(p.Tokens, l)
	p.Firma = FirmaProbable(p.Marco, p.ParesTexto)
	if p.Firma != nil && len(p.ParesTexto) > 0 {
		if casos, f, err := TiparEjemplos(p.ParesTexto, p.Firma); err == nil {
			p.Ejemplos, p.Firma = casos, f
		}
	}
	p.Conceptos = conceptosDe(p.Marco, p.Tokens, l)
	p.Intenciones = Intenciones(p)
	p.ConsultaWeb = ConsultaIngles(p, l)
	return p
}

// conceptosDe lists the concepts of a request: the frame's (action, modifiers, element, object) when
// there is a frame, else those of the words found in the lexicon. Only nucleo.Conceptos entries.
func conceptosDe(m *nucleo.Marco, tokens []nucleo.Token, l *Lexico) []string {
	if l == nil {
		l = LexicoBase()
	}
	var out []string
	visto := map[string]bool{}
	add := func(c string) {
		if c != "" && !visto[c] && nucleo.EsConcepto(c) {
			visto[c] = true
			out = append(out, c)
		}
	}
	if m != nil {
		add(m.Accion)
		for _, md := range m.Mods {
			add(md.Concepto)
		}
		add(m.Elemento)
		add(m.Objeto)
		if len(out) > 0 {
			return out
		}
	}
	for _, t := range tokens {
		if t.Clase != "palabra" {
			continue
		}
		lema := t.Lema
		if lema == "" {
			lema = Lema(t.Texto)
		}
		if c, ok := l.Concepto(lema); ok {
			add(c)
		}
	}
	return out
}

// tieneVerbo reports whether the request has a verb of its own that names an action ("ahora que
// filtre…"), as opposed to only modifiers ("ahora con impares").
func tieneVerbo(p *nucleo.Pregunta) bool {
	if p == nil {
		return false
	}
	lex := LexicoBase()
	for _, t := range p.Tokens {
		if t.Clase != "palabra" {
			continue
		}
		lema := t.Lema
		if lema == "" {
			lema = Lema(t.Texto)
		}
		if !verbosConocidos[lema] || verbosGenericos[lema] {
			continue
		}
		if c, ok := lex.Concepto(lema); ok && accionesMarco[c] {
			return true
		}
	}
	return false
}

// paradas are Spanish words dropped from web queries (besides funcionales): request verbs and fillers.
var paradas = conjunto(`buscar hacer crear escribir decir mostrar dar querer necesitar poder saber explicar
favor ayudar usar forma manera ejemplo algo cosa go golang codigo lenguaje programacion internet web
tener haber ir ver`)

// ConsultaIngles builds the English web query: Spanish stopwords and request verbs are dropped, lemmas
// are translated through the lexicon (invertir→reverse, cadena→string, lista→slice, mapa→map,
// palabra→word, ordenar→sort), Go symbols and quoted text are kept, and "golang" is added. Code
// requests (a frame or a create intent) also get "func".
func ConsultaIngles(p *nucleo.Pregunta, l *Lexico) string {
	if p == nil {
		return ""
	}
	if l == nil {
		l = LexicoBase()
	}
	toks := p.Tokens
	if len(toks) == 0 {
		toks = Lematizar(p.Resto)
		if len(toks) == 0 {
			toks = Lematizar(p.Texto)
		}
	}
	var out []string
	visto := map[string]bool{}
	add := func(w string) {
		w = strings.TrimSpace(w)
		if w == "" || visto[w] {
			return
		}
		visto[w] = true
		out = append(out, w)
	}
	conEjemplos := len(p.ParesTexto) > 0 || len(p.EjProgramas) > 0
	for i, t := range toks {
		if conEjemplos && t.Clase == "simbolo" && (t.Texto == ":" || t.Texto == "→" || t.Texto == "=" ||
			t.Texto == "-" && i+1 < len(toks) && toks[i+1].Texto == ">") {
			break // the examples are not part of the query
		}
		switch t.Clase {
		case "palabra":
			lema := t.Lema
			if lema == "" {
				lema = Lema(t.Texto)
			}
			if strings.Contains(t.Texto, ".") && esIdentASCII(t.Texto) {
				add(t.Texto) // strings.Split
				continue
			}
			if funcionales[t.Norma] || funcionales[lema] || paradas[lema] {
				continue
			}
			if i == 0 && verbosConocidos[lema] {
				if c, ok := l.Concepto(lema); !ok || c == "buscar" {
					continue
				}
			}
			if en, ok := l.Ingles(lema); ok {
				add(en)
			} else if en, ok := l.Ingles(t.Norma); ok {
				add(en)
			} else if esPalabraInglesa(t.Texto) {
				add(strings.ToLower(t.Texto))
			}
		case "cita":
			if t.Texto != "" && len(t.Texto) <= 40 {
				add(`"` + t.Texto + `"`)
			}
		}
		if len(out) >= 10 {
			break
		}
	}
	codigo := p.Marco != nil && p.Marco.Accion != ""
	for _, c := range p.Intenciones {
		if c.Tipo == nucleo.ICrearFuncion || c.Tipo == nucleo.ICrearPrograma {
			codigo = codigo || c.Puntos >= 0.5
		}
		break
	}
	if codigo && !visto["func"] {
		add("func")
	}
	add("golang")
	return strings.Join(out, " ")
}

// esPalabraInglesa: ASCII words that are not Spanish-looking (kept as written: "json", "http", "mutex").
func esPalabraInglesa(w string) bool {
	if len(w) < 3 || len(w) > 20 {
		return false
	}
	for i := 0; i < len(w); i++ {
		if !esLetraASCII(w[i]) {
			return false
		}
	}
	n := strings.ToLower(w)
	if verbosConocidos[Lema(n)] || comunes[Lema(n)] {
		return false
	}
	// Spanish endings are a strong hint
	for _, suf := range []string{"cion", "ando", "iendo", "ado", "ada", "mente", "ar", "er", "ir", "os", "as", "o", "a"} {
		if strings.HasSuffix(n, suf) {
			return false
		}
	}
	return w != strings.ToLower(w) || strings.ContainsAny(n, "kwy")
}
