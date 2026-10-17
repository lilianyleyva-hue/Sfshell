package habla

import (
	"strings"
)

// Lectura is what one Leer call learned.
type Lectura struct {
	Frases     int           // sentences read
	Entrenadas int           // sentences used to learn to talk
	Nuevos     []Significado // meanings learned or changed
}

// Leer reads a text (at most MaxLectura bytes): it splits it into sentences, learns the meanings
// stated in definitional sentences ("X es un Y…", "X significa Y", "X quiere decir Y", "X sirve para
// Y", "llamamos X a Y", "X: Y") with less confidence than a meaning the user teaches (reading never
// replaces it), and trains the language model with every statement (questions, commands and code are
// not used to learn to talk). fuente: usuario | lectura | web; origen: "lo leí en mis notas de Go".
func (h *Habla) Leer(texto, fuente, origen string) Lectura {
	var l Lectura
	if h == nil || strings.TrimSpace(texto) == "" {
		return l
	}
	texto = recortarBytes(texto, MaxLectura)
	if fuente == "" {
		fuente = FuenteLectura
	}
	conf := 0.6
	if fuente == FuenteWeb {
		conf = 0.5
	}
	reservable := fuente == FuenteLectura || fuente == FuenteWeb
	frases := partirFrases(texto)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, f := range frases {
		if largo(f) > maxFraseRunas {
			continue
		}
		l.Frases++
		pregunta := strings.ContainsAny(f, "?¿")
		if !pregunta {
			for _, d := range extraerDefiniciones(f) {
				p, def, err := validar(d.palabra, d.definicion)
				if err != nil {
					continue
				}
				if s, cambio, err := h.poner(p, def, fuente, origen, conf*d.peso, false); err == nil && cambio {
					l.Nuevos = append(l.Nuevos, s)
				}
			}
		}
		if pregunta || pareceCodigo(f) {
			continue
		}
		ts := tokenizar(f)
		if !aptaParaHablar(ts) {
			continue
		}
		if reservable && h.lm.reservar(f) {
			continue
		}
		toks, may := tokensLM(ts)
		h.lm.entrenar(toks, may)
		h.frases++
		l.Entrenadas++
	}
	if l.Entrenadas > 0 || len(l.Nuevos) > 0 {
		h.sucio = true
	}
	return l
}

// aptaParaHablar: a statement with at least two words, mostly words, that is not a command.
func aptaParaHablar(ts []token) bool {
	palabras, raras := 0, 0
	primera := ""
	for _, t := range ts {
		switch t.clase {
		case 'w':
			palabras++
			if primera == "" {
				primera = t.pleg
			}
		case 'n', 's':
			raras++
		}
	}
	if palabras < 2 || ordenes[primera] {
		return false
	}
	return raras*4 <= palabras+raras
}

// definicionLeida is a definition found in a sentence.
type definicionLeida struct {
	palabra, definicion string
	peso                float64 // 1 for "es un", "significa"; less for "sirve para" and "X: Y"
}

// prefijosEnsenar are dropped before looking for a definition: "recuerda que X significa Y".
var prefijosEnsenar = conjunto(`recuerda aprende apunta memoriza sabias`)

// extraerDefiniciones finds the definitional patterns of one sentence. Linear in its length.
func extraerDefiniciones(frase string) []definicionLeida {
	frase = quitarParentesis(frase)
	ts := tokenizar(frase)
	if len(ts) < 3 {
		return nil
	}
	i := 0
	switch {
	case len(ts) > 2 && prefijosEnsenar[ts[0].pleg] && ts[1].pleg == "que":
		i = 2
	case len(ts) > 4 && ts[0].pleg == "ten" && ts[1].pleg == "en" && ts[2].pleg == "cuenta" && ts[3].pleg == "que":
		i = 4
	}
	// llamamos X a Y / se llama X a Y
	if j := i; j < len(ts) && (ts[j].pleg == "llamamos" || (ts[j].pleg == "se" && j+1 < len(ts) && (ts[j+1].pleg == "llama" || ts[j+1].pleg == "llaman"))) {
		if ts[j].pleg == "se" {
			j++
		}
		j++
		for k := j + 1; k < len(ts) && k <= j+3; k++ {
			if ts[k].pleg == "a" || ts[k].pleg == "al" {
				if x, ok := palabraDe(ts[j:k]); ok {
					if def := definicionDesde(frase, ts, k+1, 2); def != "" {
						if ts[k].pleg == "al" {
							def = "el " + def
						}
						return []definicionLeida{{x, def, 1}}
					}
				}
				break
			}
		}
		return nil
	}
	plural := false
	if i < len(ts) && articulos[ts[i].pleg] {
		plural = articulosPlurales[ts[i].pleg]
		i++
	}
	for n := 1; n <= 3 && i+n < len(ts); n++ {
		x := ts[i : i+n]
		ultimo := x[n-1]
		if ultimo.clase != 'w' || (n == 1 && esVacia(ultimo.pleg)) {
			return nil
		}
		if n > 1 && esVacia(ultimo.pleg) {
			if ultimo.pleg != "de" {
				return nil
			}
			continue // "cadena de texto es…": the "de" goes inside X
		}
		j := i + n
		m := ts[j].pleg
		var def string
		peso := 1.0
		plural2 := plural
		switch {
		case (m == "es" || m == "son") && j+1 < len(ts) && articulos[ts[j+1].pleg]:
			def = definicionDesde(frase, ts, j+1, 2)
			plural2 = plural2 || m == "son"
		case m == "significa" || m == "significan":
			k := j + 1
			if k+2 < len(ts) && ts[k].pleg == "lo" && ts[k+1].pleg == "mismo" && ts[k+2].pleg == "que" {
				k += 3
			}
			def = definicionDesde(frase, ts, k, 1)
			plural2 = plural2 || m == "significan"
		case (m == "quiere" || m == "quieren") && j+1 < len(ts) && ts[j+1].pleg == "decir":
			def = definicionDesde(frase, ts, j+2, 1)
			plural2 = plural2 || m == "quieren"
		case (m == "sirve" || m == "sirven") && j+1 < len(ts) && ts[j+1].pleg == "para":
			if d := definicionDesde(frase, ts, j+2, 1); d != "" {
				def = "sirve para " + d
			}
			peso = 0.8
			plural2 = plural2 || m == "sirven"
		case m == ":" && n == 1 && j+1 < len(ts) && (ts[j+1].clase == 'w') && (!esVacia(ts[j+1].pleg) || articulos[ts[j+1].pleg]):
			def = definicionDesde(frase, ts, j+1, 2)
			if strings.ContainsAny(def, "0123456789") || singular(x[0].texto) != strings.ToLower(x[0].texto) {
				def = "" // "Bucles: for y range" is a title that lists, not a definition
			}
			peso = 0.8
		default:
			continue
		}
		if def == "" {
			return nil
		}
		p, ok := palabraDe(x)
		if !ok {
			return nil
		}
		if plural2 {
			p = singularFrase(p)
		}
		return []definicionLeida{{p, def, peso}}
	}
	return nil
}

// palabraDe joins the words of X ("función anónima"); false when they are not plain words.
func palabraDe(ts []token) (string, bool) {
	var ws []string
	for _, t := range ts {
		if t.clase != 'w' || pareceIdentificador(t.texto) {
			return "", false
		}
		ws = append(ws, strings.ToLower(t.texto))
	}
	if len(ws) == 0 || esVacia(plegar(ws[0])) {
		return "", false
	}
	return strings.Join(ws, " "), true
}

// definicionDesde is the text of the sentence from token k to the end of the clause (":", ";" or the
// end), with at least minPalabras words; "" when there is none or it looks like code.
func definicionDesde(frase string, ts []token, k, minPalabras int) string {
	if k >= len(ts) {
		return ""
	}
	fin := len(frase)
	palabras := 0
	for j := k; j < len(ts); j++ {
		if ts[j].texto == ":" || ts[j].texto == ";" {
			fin = ts[j].desde
			break
		}
		if ts[j].clase == 'w' {
			palabras++
		}
		if ts[j].clase == 's' && strings.ContainsAny(ts[j].texto, "=<>{}[]*&|") {
			return ""
		}
	}
	if palabras < minPalabras {
		return ""
	}
	d := strings.TrimSpace(frase[ts[k].desde:fin])
	d = strings.TrimRight(d, " .,;:!?…—-")
	if pareceCodigo(d) {
		return ""
	}
	return cortarRunas(d, MaxDefinicion)
}

// quitarParentesis removes "(…)" groups (not nested): "Un módulo (archivo go.mod) es…" → "Un módulo es…".
func quitarParentesis(s string) string {
	if !strings.Contains(s, "(") {
		return s
	}
	var sb strings.Builder
	dentro := false
	for _, r := range s {
		switch {
		case r == '(':
			dentro = true
		case r == ')' && dentro:
			dentro = false
		case !dentro:
			sb.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}
