package logica

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// tok is one word or symbol of a logic text. norma is nucleo.Normalizar of a word, or the canonical
// form of a symbol ("->", "<->", "&", "|", "!", "(", ")", ",", "\"", ";", ".", "\n", "¿", "?", ":").
type tok struct {
	orig  string
	norma string
	sim   bool
	mayus bool
	desde int // byte offset in the text
}

// tokenizar splits a text into words and symbols. Symbolic connectives are canonicalized:
// → ⇒ -> => become "->"; ↔ ⇔ <-> <=> become "<->"; ∧ & && become "&"; ∨ | || become "|";
// ¬ ~ and a "!" glued to the next word become "!". Quotes of any kind become "\"".
func tokenizar(s string) []tok {
	var out []tok
	i := 0
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\n':
			out = append(out, tok{orig: "\n", norma: "\n", sim: true, desde: i})
			i += n
			continue
		case unicode.IsSpace(r):
			i += n
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			j := i + n
			for j < len(s) {
				r2, n2 := utf8.DecodeRuneInString(s[j:])
				if !(unicode.IsLetter(r2) || unicode.IsDigit(r2) || r2 == '_') {
					break
				}
				j += n2
			}
			w := s[i:j]
			out = append(out, tok{orig: w, norma: nucleo.Normalizar(w), mayus: unicode.IsUpper(r), desde: i})
			i = j
			continue
		}
		largo := ""
		for _, l := range []string{"<->", "<=>", "->", "=>", "&&", "||"} {
			if strings.HasPrefix(s[i:], l) {
				largo = l
				break
			}
		}
		if largo != "" {
			norma := map[string]string{"<->": "<->", "<=>": "<->", "->": "->", "=>": "->", "&&": "&", "||": "|"}[largo]
			out = append(out, tok{orig: largo, norma: norma, sim: true, desde: i})
			i += len(largo)
			continue
		}
		c := string(r)
		norma := c
		switch r {
		case '→', '⇒', '⊃':
			norma = "->"
		case '↔', '⇔', '≡':
			norma = "<->"
		case '∧', '&':
			norma = "&"
		case '∨', '|':
			norma = "|"
		case '¬', '~':
			norma = "!"
		case '!':
			// "!p" is a negation; "¡llueve!" is an exclamation and is ignored
			sig, _ := utf8.DecodeRuneInString(s[i+n:])
			if i+n < len(s) && (unicode.IsLetter(sig) || sig == '(') {
				norma = "!"
			} else {
				i += n
				continue
			}
		case '¡':
			i += n
			continue
		case '«', '»', '“', '”', '"', '\'', '‘', '’':
			norma = "\""
		case '[', '{':
			norma = "("
		case ']', '}':
			norma = ")"
		}
		out = append(out, tok{orig: c, norma: norma, sim: true, desde: i})
		i += n
	}
	return out
}

// frase is one sentence of the text.
type frase struct {
	toks       []tok
	pregunta   bool // written as ¿…? or ending in ?
	conclusion bool // introduced by "por lo tanto", "entonces", "luego"…
}

// marcadoresConclusion introduce the conclusion of an argument at the start of a sentence.
var marcadoresConclusion = [][]string{
	{"por", "lo", "tanto"}, {"por", "tanto"}, {"asi", "que"}, {"en", "consecuencia"}, {"de", "modo", "que"},
	{"de", "manera", "que"}, {"de", "ahi", "que"}, {"luego"}, {"entonces"}, {"ergo"}, {"se", "sigue", "que"},
	{"se", "deduce", "que"}, {"se", "concluye", "que"}, {"concluimos", "que"}, {"podemos", "concluir", "que"},
	{"se", "puede", "concluir", "que"}, {"se", "puede", "deducir", "que"}, {"podemos", "deducir", "que"},
	{"concluyo", "que"},
}

// marcadoresPregunta are stripped from the start of a question ("¿es cierto que llueve?").
var marcadoresPregunta = [][]string{
	{"es", "cierto", "que"}, {"es", "verdad", "que"}, {"sera", "cierto", "que"}, {"se", "cumple", "que"},
	{"podemos", "asegurar", "que"}, {"puedo", "concluir", "que"}, {"puedo", "deducir", "que"},
}

// marcadoresMedio split a sentence and start a conclusion: "llueve, por lo tanto el suelo se moja".
var marcadoresMedio = [][]string{
	{"por", "lo", "tanto"}, {"por", "tanto"}, {"asi", "que"}, {"en", "consecuencia"}, {"de", "modo", "que"},
	{"de", "manera", "que"}, {"ergo"},
}

// empiezaPor reports whether the normas of ts start with seq; it returns the length matched.
func empiezaPor(ts []tok, seq []string) bool {
	if len(ts) < len(seq) {
		return false
	}
	for i, w := range seq {
		if ts[i].norma != w {
			return false
		}
	}
	return true
}

// quitarPrefijo strips the first matching marker of lista (and any commas after it).
func quitarPrefijo(ts []tok, lista [][]string) ([]tok, bool) {
	for _, seq := range lista {
		if empiezaPor(ts, seq) && len(ts) > len(seq) {
			r := ts[len(seq):]
			for len(r) > 0 && (r[0].norma == "," || r[0].norma == ":") {
				r = r[1:]
			}
			return r, true
		}
	}
	return ts, false
}

// partirFrases splits the tokens on ; . newlines ¿ ? and "y además", and also before a
// mid-sentence conclusion marker ("…, por lo tanto …").
func partirFrases(ts []tok) []frase {
	var out []frase
	var cur []tok
	preg, concl := false, false
	cerrar := func() {
		c := recortar(cur)
		if len(c) > 0 {
			out = append(out, frase{toks: c, pregunta: preg, conclusion: concl})
		}
		cur, preg, concl = nil, false, false
	}
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		switch t.norma {
		case ";", ".", "\n":
			cerrar()
			continue
		case "¿":
			cerrar()
			preg = true
			continue
		case "?":
			preg = true
			cerrar()
			continue
		case "y":
			if i+1 < len(ts) && ts[i+1].norma == "ademas" {
				cerrar()
				i++
				continue
			}
		}
		if len(recortar(cur)) > 0 {
			partido := false
			for _, seq := range marcadoresMedio {
				if empiezaPor(ts[i:], seq) {
					cerrar()
					concl = true
					i += len(seq) - 1
					partido = true
					break
				}
			}
			if partido {
				continue
			}
			if (t.norma == "luego" || t.norma == "entonces") && len(cur) > 0 && cur[len(cur)-1].norma == "," && !contieneNorma(cur, "si") && !contieneNorma(cur, "cuando") {
				cerrar()
				concl = true
				continue
			}
		}
		cur = append(cur, t)
	}
	cerrar()
	// conclusion markers at the start of a sentence
	for i := range out {
		if r, ok := quitarPrefijo(out[i].toks, marcadoresConclusion); ok {
			out[i].toks = r
			out[i].conclusion = true
		}
		if out[i].pregunta {
			if r, ok := quitarPrefijo(out[i].toks, marcadoresPregunta); ok {
				out[i].toks = r
			}
			if r, ok := quitarPrefijo(out[i].toks, marcadoresConclusion); ok {
				out[i].toks = r
			}
		}
	}
	return out
}

func contieneNorma(ts []tok, w string) bool {
	for _, t := range ts {
		if t.norma == w {
			return true
		}
	}
	return false
}

// recortar drops leading and trailing commas, colons, quotes and dashes.
func recortar(ts []tok) []tok {
	inutil := func(t tok) bool {
		return t.sim && (t.norma == "," || t.norma == ":" || t.norma == "\"" || t.norma == "-" || t.norma == "—")
	}
	for len(ts) > 0 && inutil(ts[0]) {
		ts = ts[1:]
	}
	for len(ts) > 0 && inutil(ts[len(ts)-1]) {
		ts = ts[:len(ts)-1]
	}
	return ts
}

// sinComillas removes quote tokens.
func sinComillas(ts []tok) []tok {
	var out []tok
	for _, t := range ts {
		if t.sim && t.norma == "\"" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// textoToks joins the original words (lowercased unless they look like names) with spaces.
func textoToks(ts []tok) string {
	var partes []string
	for _, t := range ts {
		if t.sim {
			continue
		}
		w := t.orig
		letra := utf8.RuneCountInString(w) == 1
		if !(t.mayus && (letra || len(partes) > 0)) {
			w = strings.ToLower(w)
		}
		partes = append(partes, w)
	}
	return strings.Join(partes, " ")
}

// esQueConcluye reports whether a question asks "¿qué se concluye?" (or similar).
func esQueConcluye(f frase) bool {
	if !f.pregunta || len(f.toks) == 0 {
		return false
	}
	if f.toks[0].norma != "que" && f.toks[0].norma != "cual" && f.toks[0].norma != "cuales" {
		return false
	}
	for _, t := range f.toks {
		switch t.norma {
		case "concluye", "concluir", "conclusion", "conclusiones", "deduce", "deducir", "sigue", "seguir",
			"infiere", "inferir", "deduces", "concluyes", "pasa", "podemos", "puedes":
			return true
		}
	}
	return false
}

// raizTermino is the comparison key of a word: plural, infinitive ending and final vowel removed
// ("hombres", "hombre" → "hombr"; "llueve", "llover" → "llov"/"lluev"; "mamíferos" → "mamifer").
func raizTermino(w string) string {
	w = nucleo.Normalizar(w)
	n := utf8.RuneCountInString(w)
	switch {
	case n > 4 && strings.HasSuffix(w, "es"):
		w = w[:len(w)-2]
	case n > 3 && strings.HasSuffix(w, "s"):
		w = w[:len(w)-1]
	}
	n = utf8.RuneCountInString(w)
	if n > 4 && (strings.HasSuffix(w, "ar") || strings.HasSuffix(w, "er") || strings.HasSuffix(w, "ir")) {
		w = w[:len(w)-2]
	}
	n = utf8.RuneCountInString(w)
	if n > 3 && strings.ContainsAny(w[len(w)-1:], "aeo") {
		w = w[:len(w)-1]
	}
	return w
}

// lematizar returns the lemma of one word with lem (nil: the normalized word).
func lematizar(w string, lem nucleo.Lematizador) string {
	norma := nucleo.Normalizar(w)
	if lem == nil {
		return norma
	}
	for _, t := range lem(w) {
		if t.Clase == "palabra" || t.Clase == "" {
			if t.Lema != "" {
				return t.Lema
			}
			return t.Norma
		}
	}
	return norma
}

// lemaTermino is the stored form of a noun term: the lemma, unless the lemmatizer turned a noun
// into a verb form ("hombre" → "hombrer"), in which case the normalized word is kept.
func lemaTermino(w string, lem nucleo.Lematizador) string {
	norma := nucleo.Normalizar(w)
	l := lematizar(w, lem)
	if l == norma+"r" || l == "" {
		return norma
	}
	return l
}
