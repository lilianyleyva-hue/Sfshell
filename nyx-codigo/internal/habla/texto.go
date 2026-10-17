package habla

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Text helpers: folding, word lists, a small Spanish stemmer and the tolerant lookup forms. Every
// function here is linear in the length of its input.

// plegar folds a word or phrase: lowercase, no accents (ñ stays), single spaces, no quotes or
// punctuation at the ends. It mirrors nucleo.Normalizar.
func plegar(s string) string {
	s = nucleo.Normalizar(s)
	return strings.Trim(s, " \"'.,;:()[]-")
}

// clave is the dictionary key of a word or short phrase.
func clave(palabra string) string { return plegar(palabra) }

func largo(s string) int { return utf8.RuneCountInString(s) }

// esLetras: only letters, with single inner spaces or hyphens ("función anónima", "e-mail").
func esLetras(s string) bool {
	if s == "" {
		return false
	}
	letras := 0
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letras++
		case r == ' ' || r == '-':
		default:
			return false
		}
	}
	return letras > 0
}

// funcion are the function words (folded): articles, prepositions, conjunctions, pronouns, common
// adverbs and numerals, and the forms of the most common verbs (ser, estar, tener, hacer, haber, poder,
// ir, deber, querer, decir, dar, servir, soler, llamar). A sentence may use them without the app knowing
// what they mean.
var funcion = conjunto(`
el la los las un una unos unas lo al del
a ante bajo con contra de desde en entre hacia hasta para por segun sin sobre tras durante mediante
y e o u ni que pero sino porque pues como cuando donde si aunque mientras entonces luego asi cual cuales
yo tu ella ello nosotros nosotras vosotros vosotras ellos ellas usted ustedes me te se nos os le les
mi mis tus su sus nuestro nuestra nuestros nuestras vuestro vuestra
este esta esto estos estas ese esa eso esos esas aquel aquella aquello aquellos aquellas
quien quienes cuyo cuya algo alguien alguno alguna algunos algunas algun nada nadie ninguno ninguna ningun
todo toda todos todas otro otra otros otras mismo misma mismos mismas cada varios varias
mucho mucha muchos muchas poco poca pocos pocas tanto tanta tantos tantas demasiado demasiada ambos ambas
no si ya muy mas menos tambien tampoco solo sola solos solas bien mal aqui ahi alli alla hoy ayer
siempre nunca ahora antes despues casi aun todavia tan quiza quizas incluso ademas vez veces
cero uno dos tres cuatro cinco seis siete ocho nueve diez cien mil primero primera segundo segunda
ser es son era eran fue fueron sea sean siendo sido soy eres somos sera seran seria
estar estan estaba estaban estoy estas estamos estado este esten
tener tiene tienen tenia tenian tengo tienes tenemos tenido teniendo tenga tengan
hacer hace hacen hizo hago haces hacemos hecho haciendo haz haga hagan
haber hay ha han he has habia habian hubo habido haya
poder puede pueden puedo puedes podemos podia pudo pueda puedan
ir va van voy vas vamos iba iban vaya
deber debe deben debo debes debia
querer quiere quieren quiero quieres queria
decir dice dicen digo dices dicho diciendo
dar da dan doy das dio dado
servir sirve sirven sirvio servia
soler suele suelen
llamar llama llaman llamamos llamado llamada
`)

func conjunto(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// esFuncion reports whether a (folded) word is a function word.
func esFuncion(w string) bool { return funcion[w] }

// ordenes are first words of commands: a sentence that starts with one is a request, not a statement,
// and is not used to learn to talk ("haz una función que…").
var ordenes = conjunto(`
haz hazme crea creame escribe escribeme dame dime di resuelve resuelveme calcula calculame busca buscame
explica explicame explicalo arregla arreglame corrige prueba pruebala pruebalo ejecuta ejecutalo lee leeme
aprende recuerda olvida olvidate factoriza cuenta devuelve invierte genera implementa necesito quiero
hablame ordena convierte enseñame muestra muestrame elige pon ponle traduce traduceme
`)

// ---- tokens ----

// token is a piece of a sentence: a word (letters, maybe digits), a punctuation mark or a symbol.
type token struct {
	texto  string // as written
	pleg   string // folded
	desde  int    // byte offsets in the sentence
	hasta  int
	clase  byte // 'w' word of letters, 'n' has digits, 'p' punctuation , ; :, 's' other symbol / code
	inicio bool // first word of the sentence
}

// tokenizar splits a sentence into words, punctuation and symbols. Dots inside a word
// ("strings.Fields", "3.5") keep it as one token (a symbol).
func tokenizar(frase string) []token {
	var out []token
	i := 0
	for i < len(frase) {
		r, n := utf8.DecodeRuneInString(frase[i:])
		switch {
		case unicode.IsSpace(r):
			i += n
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			j := i
			letras, otros := true, false
			for j < len(frase) {
				r2, n2 := utf8.DecodeRuneInString(frase[j:])
				if unicode.IsLetter(r2) || unicode.IsDigit(r2) || r2 == '_' {
					if !unicode.IsLetter(r2) {
						letras = false
					}
					j += n2
					continue
				}
				// inner hyphen, apostrophe or dot followed by a letter or digit: same token
				if (r2 == '-' || r2 == '.' || r2 == '\'') && j+n2 < len(frase) {
					r3, _ := utf8.DecodeRuneInString(frase[j+n2:])
					if unicode.IsLetter(r3) || unicode.IsDigit(r3) {
						if r2 != '-' {
							otros = true
						}
						j += n2
						continue
					}
				}
				break
			}
			t := token{texto: frase[i:j], desde: i, hasta: j}
			t.pleg = strings.ToLower(t.texto)
			switch {
			case otros:
				t.clase = 's'
			case letras:
				t.clase = 'w'
			default:
				t.clase = 'n'
				for _, c := range t.texto {
					if unicode.IsLetter(c) {
						t.clase = 's' // x2, int64: code-like
						break
					}
				}
			}
			out = append(out, t)
			i = j
		case r == ',' || r == ';' || r == ':':
			out = append(out, token{texto: string(r), pleg: string(r), desde: i, hasta: i + n, clase: 'p'})
			i += n
		case r == '"' || r == '«' || r == '»' || r == '“' || r == '”' || r == '\'' || r == '¿' || r == '¡' ||
			r == '?' || r == '!' || r == '.' || r == '(' || r == ')' || r == '…':
			i += n // quotes, question marks and brackets are not tokens
		default:
			j := i + n
			for j < len(frase) {
				r2, n2 := utf8.DecodeRuneInString(frase[j:])
				if unicode.IsSpace(r2) || unicode.IsLetter(r2) || unicode.IsDigit(r2) {
					break
				}
				j += n2
			}
			out = append(out, token{texto: frase[i:j], pleg: frase[i:j], desde: i, hasta: j, clase: 's'})
			i = j
		}
	}
	for k := range out {
		if out[k].clase == 'w' {
			out[k].pleg = plegar(out[k].texto)
			out[k].inicio = true
			break
		}
	}
	return out
}

// partirFrases splits a text into sentences: at . ! ? followed by a space or the end, and at line
// breaks. Bullets at the start of a line are dropped. Linear in the text.
func partirFrases(texto string) []string {
	var out []string
	empezar := 0
	agregar := func(fin int) {
		f := strings.TrimSpace(texto[empezar:fin])
		f = strings.TrimLeft(f, "-*•#> \t")
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	for i := 0; i < len(texto); {
		r, n := utf8.DecodeRuneInString(texto[i:])
		switch {
		case r == '\n':
			agregar(i)
			empezar = i + n
		case r == '.' || r == '!' || r == '?' || r == '…':
			siguiente := i + n
			if siguiente >= len(texto) {
				agregar(siguiente)
				empezar = siguiente
			} else if r2, _ := utf8.DecodeRuneInString(texto[siguiente:]); unicode.IsSpace(r2) {
				agregar(siguiente)
				empezar = siguiente
			}
		}
		i += n
	}
	if empezar < len(texto) {
		agregar(len(texto))
	}
	return out
}

// pareceCodigo: a sentence with code operators is not prose.
func pareceCodigo(frase string) bool {
	for _, s := range []string{":=", "{", "}", "==", "!=", "<-", "&&", "||", "[]", "()", "->", "=>", "//", "/*"} {
		if strings.Contains(frase, s) {
			return true
		}
	}
	return false
}

// ---- stemmer ----

// sufijos are stripped (the first that leaves at least 3 letters) before cutting the stem to 5 letters.
var sufijos = []string{
	"amientos", "imientos", "aciones", "amiento", "imiento", "abamos", "aremos", "eremos", "iremos",
	"acion", "aron", "ieron", "iendo", "yendo", "ando", "aban", "abas", "amos", "emos", "imos", "ados", "adas",
	"idos", "idas", "aste", "iste", "aba", "ado", "ada", "ido", "ida", "ais", "eis", "ias", "ian", "ios",
	"ar", "er", "ir", "as", "es", "os", "an", "en", "ia", "io", "a", "e", "o", "s",
}

var encliticos = []string{"selo", "sela", "melo", "mela", "telo", "tela", "nos", "les", "los", "las", "lo", "la", "le", "me", "te", "se"}

// tallo is a rough stem of at most 5 letters: memoria/memorias → memor, guardar/guardas → guard,
// quedarte → qued, funciones/función → funci. Words with the same stem count as the same idea.
func tallo(w string) string {
	w = plegar(w)
	if largo(w) <= 3 {
		return w
	}
	for _, e := range encliticos {
		if strings.HasSuffix(w, e) {
			base := w[:len(w)-len(e)]
			if largo(base) >= 4 && (strings.HasSuffix(base, "ar") || strings.HasSuffix(base, "er") || strings.HasSuffix(base, "ir") ||
				strings.HasSuffix(base, "ando") || strings.HasSuffix(base, "iendo")) {
				w = base
				break
			}
		}
	}
	for _, s := range sufijos {
		if strings.HasSuffix(w, s) && largo(w)-largo(s) >= 3 {
			w = w[:len(w)-len(s)]
			break
		}
	}
	r := []rune(w)
	if len(r) > 5 {
		r = r[:5]
	}
	return string(r)
}

// ---- lookup forms ----

type terminacion struct {
	suf string
	inf []string
}

// verbales map conjugated endings to infinitive endings (most likely first).
var verbales = []terminacion{
	{"iendo", []string{"er", "ir"}}, {"yendo", []string{"er", "ir"}}, {"ando", []string{"ar"}},
	{"aron", []string{"ar"}}, {"ieron", []string{"er", "ir"}}, {"abamos", []string{"ar"}},
	{"aban", []string{"ar"}}, {"abas", []string{"ar"}}, {"aba", []string{"ar"}},
	{"ados", []string{"ar"}}, {"adas", []string{"ar"}}, {"ado", []string{"ar"}}, {"ada", []string{"ar"}},
	{"idos", []string{"er", "ir"}}, {"idas", []string{"er", "ir"}}, {"ido", []string{"er", "ir"}}, {"ida", []string{"er", "ir"}},
	{"iamos", []string{"er", "ir"}}, {"ian", []string{"er", "ir"}}, {"ias", []string{"er", "ir"}}, {"ia", []string{"er", "ir"}},
	{"aste", []string{"ar"}}, {"iste", []string{"er", "ir"}}, {"io", []string{"er", "ir"}},
	{"amos", []string{"ar"}}, {"emos", []string{"er"}}, {"imos", []string{"ir"}},
	{"ais", []string{"ar"}}, {"eis", []string{"er"}},
	{"an", []string{"ar", "er", "ir"}}, {"en", []string{"er", "ir", "ar"}},
	{"as", []string{"ar", "er", "ir"}}, {"es", []string{"er", "ir", "ar"}},
	{"a", []string{"ar", "er", "ir"}}, {"e", []string{"er", "ir", "ar"}}, {"o", []string{"ar", "er", "ir"}},
	{"i", []string{"er", "ir"}},
}

// diptongos undoes the stem change of some verbs: recuerd → record, pierd → perd, pid → ped.
func diptongos(raiz string) []string {
	var out []string
	if i := strings.LastIndex(raiz, "ue"); i >= 0 {
		out = append(out, raiz[:i]+"o"+raiz[i+2:])
	}
	if i := strings.LastIndex(raiz, "ie"); i >= 0 {
		out = append(out, raiz[:i]+"e"+raiz[i+2:])
	}
	if i := strings.LastIndex(raiz, "i"); i > 0 {
		out = append(out, raiz[:i]+"e"+raiz[i+1:])
	}
	return out
}

// formas lists the dictionary forms a folded word may come from, most likely first (the word itself
// included): plurals, feminine, verb forms and enclitic pronouns.
func formas(w string) []string {
	visto := map[string]bool{}
	var out []string
	agregar := func(s string) {
		if largo(s) >= 1 && !visto[s] {
			visto[s] = true
			out = append(out, s)
		}
	}
	agregar(w)
	n := len(w)
	if strings.HasSuffix(w, "iones") && n > 6 {
		agregar(w[:n-5] + "ion")
	}
	if strings.HasSuffix(w, "ces") && n > 4 {
		agregar(w[:n-3] + "z")
	}
	if strings.HasSuffix(w, "es") && n > 3 {
		agregar(w[:n-2])
	}
	if strings.HasSuffix(w, "s") && n > 2 {
		agregar(w[:n-1])
	}
	for _, c := range append([]string(nil), out...) {
		switch {
		case strings.HasSuffix(c, "a") && len(c) > 2:
			agregar(c[:len(c)-1] + "o")
		case strings.HasSuffix(c, "o") && len(c) > 2:
			agregar(c[:len(c)-1] + "a")
		}
	}
	verbos := func(x string) {
		for _, t := range verbales {
			if !strings.HasSuffix(x, t.suf) || len(x)-len(t.suf) < 2 {
				continue
			}
			raiz := x[:len(x)-len(t.suf)]
			for _, inf := range t.inf {
				agregar(raiz + inf)
			}
			for _, r := range diptongos(raiz) {
				for _, inf := range t.inf {
					agregar(r + inf)
				}
			}
		}
	}
	verbos(w)
	for _, e := range encliticos {
		if strings.HasSuffix(w, e) && len(w)-len(e) >= 3 {
			base := w[:len(w)-len(e)]
			if strings.HasSuffix(base, "ar") || strings.HasSuffix(base, "er") || strings.HasSuffix(base, "ir") {
				agregar(base)
			} else {
				verbos(base) // súmalos → suma → sumar
			}
		}
	}
	return out
}

// singular turns a plural noun into its singular, keeping accents: funciones → función,
// variables → variable, papeles → papel, lápices → lápiz.
func singular(palabra string) string {
	w := strings.ToLower(strings.TrimSpace(palabra))
	n := len(w)
	if largo(w) <= 3 || !strings.HasSuffix(w, "s") {
		return w
	}
	switch {
	case strings.HasSuffix(w, "iones"):
		return w[:n-5] + "ión"
	case strings.HasSuffix(w, "ces") && largo(w) > 4:
		return w[:n-3] + "z"
	case strings.HasSuffix(w, "es"):
		base := w[:n-2]
		r := []rune(base)
		if len(r) >= 2 && strings.ContainsRune("lnrdjyz", r[len(r)-1]) && strings.ContainsRune("aeiouáéíóú", r[len(r)-2]) {
			return base // papeles → papel, valores → valor, ciudades → ciudad
		}
		return w[:n-1] // variables → variable, clases → clase
	}
	return w[:n-1]
}

// singularFrase applies singular to every word of a short phrase ("funciones anónimas").
func singularFrase(s string) string {
	ws := strings.Fields(s)
	for i, w := range ws {
		ws[i] = singular(w)
	}
	return strings.Join(ws, " ")
}

// sinArticulo drops a leading article: "una variable" → "variable".
func sinArticulo(s string) string {
	ws := strings.Fields(s)
	if len(ws) > 1 && articulos[plegar(ws[0])] {
		return strings.Join(ws[1:], " ")
	}
	return strings.Join(ws, " ")
}

var articulos = conjunto(`el la los las un una unos unas lo`)

// articulosPlurales mark a plural noun after them.
var articulosPlurales = conjunto(`los las unos unas`)

// fecha is "17/10".
func fechaCorta(d int, m int) string { return itoa(d) + "/" + itoa(m) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// cortarRunas cuts s to at most n runes at a space when possible.
func cortarRunas(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	corte := n
	for j := n; j > n/2; j-- {
		if r[j] == ' ' {
			corte = j
			break
		}
	}
	return strings.TrimSpace(string(r[:corte]))
}
