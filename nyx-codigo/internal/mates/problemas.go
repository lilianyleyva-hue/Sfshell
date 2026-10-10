package mates

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Planteamiento is a word problem turned into equations.
type Planteamiento struct {
	Ecuaciones  []string
	Incognitas  map[string]string // "x" → "el número"
	Pregunta    string            // which unknown or quantity is asked
	Mapeo       []string          // "«el doble de un número» → 2·x"
	Desconocido []string          // verbs not understood
	// Additive fields used to write the answer.
	Tipo      string // "traduccion" | "relaciones" | "situacion" | "movimiento" | "precio"
	Respuesta string // answer template; "%s" is replaced by the value of Pregunta (or by "x = …" lists)
	Unidad    string
}

// Verbos is the learned verb-effect table (memoria implements it).
type Verbos interface {
	Efecto(verbo string) (string, bool) // "sumar" | "restar" | "fijar" | "transferir"
	Aprender(verbo, efecto string) error
}

// efectosBase are the verb effects every problem knows.
var efectosBase = map[string]string{
	"tener": "fijar", "haber": "fijar", "quedar": "fijar",
	"dar": "transferir", "regalar": "transferir", "prestar": "transferir", "entregar": "transferir",
	"pasar": "transferir", "donar": "transferir", "enviar": "transferir", "mandar": "transferir",
	"comprar": "sumar", "ganar": "sumar", "recibir": "sumar", "encontrar": "sumar", "conseguir": "sumar",
	"coger": "sumar", "recoger": "sumar", "añadir": "sumar", "agregar": "sumar", "meter": "sumar",
	"cobrar": "sumar", "ahorrar": "sumar", "pescar": "sumar", "obtener": "sumar", "sumar": "sumar",
	"juntar": "sumar", "traer": "sumar", "llegar": "sumar", "nacer": "sumar",
	"perder": "restar", "gastar": "restar", "comer": "restar", "vender": "restar", "romper": "restar",
	"usar": "restar", "tirar": "restar", "quitar": "restar", "sacar": "restar", "beber": "restar",
	"pagar": "restar", "restar": "restar", "irse": "restar", "marcharse": "restar", "salir": "restar",
	"morir": "restar", "devolver": "restar", "olvidar": "restar", "consumir": "restar", "gastarse": "restar",
	"comerse": "restar", "repartir": "repartir",
}

// irregularesVerbo maps conjugated forms that the endings cannot undo.
var irregularesVerbo = map[string]string{
	"tiene": "tener", "tienen": "tener", "tengo": "tener", "tienes": "tener", "tenia": "tener", "tenian": "tener",
	"tuvo": "tener", "tuvieron": "tener", "tendra": "tener", "tendran": "tener", "tenemos": "tener",
	"da": "dar", "dan": "dar", "doy": "dar", "das": "dar", "dio": "dar", "dieron": "dar", "dado": "dar", "damos": "dar",
	"hay": "haber", "habia": "haber", "hubo": "haber",
	"pierde": "perder", "pierden": "perder", "pierdo": "perder", "pierdes": "perder",
	"encuentra": "encontrar", "encuentran": "encontrar", "encuentro": "encontrar",
	"consigue": "conseguir", "consiguen": "conseguir", "consiguio": "conseguir",
	"coge": "coger", "cogen": "coger", "recoge": "recoger", "recogen": "recoger",
	"devuelve": "devolver", "devuelven": "devolver", "muere": "morir", "mueren": "morir", "murieron": "morir",
	"trae": "traer", "traen": "traer", "trajo": "traer", "sale": "salir", "salen": "salir",
	"queda": "quedar", "quedan": "quedar", "quedaron": "quedar", "quedo": "quedar",
	"va": "ir", "van": "ir", "fue": "ir", "fueron": "ir", "reparte": "repartir", "reparten": "repartir",
	"vende": "vender", "venden": "vender", "come": "comer", "comen": "comer", "rompe": "romper", "rompen": "romper",
	"bebe": "beber", "beben": "beber", "recibe": "recibir", "reciben": "recibir", "obtiene": "obtener",
}

// noVerbos are words that look like conjugated verbs but are not (for unknown-verb detection).
var noVerbos = conjuntoPal(`cada otra otro otras otros una uno unas unos para sobre entre hasta desde tras
mas menos casa caja bolsa cesta hora horas semana semanas manzana manzanas canica canicas galleta galletas
moneda monedas pelota pelotas tarta tartas fresa fresas naranja naranjas pera peras uva uvas cosa cosas
nota notas ahora luego despues entonces tambien ademas ayer hoy manana todavia aun solo sola juntos juntas
media medio doble triple mitad tercio cuarto quinto docena decena veces vez euro euros total`)

var funcionalesP = conjuntoPal(`a al ante con contra de del desde en entre hacia hasta para por segun sin
sobre tras el la lo los las un una unos unas y e o u ni que si no le les se me te nos os su sus mi mis tu tus
ya muy tan mas menos este esta estos estas ese esa esos esas aquel aquella al cual cuyo donde cuando como
ha han he has hemos habia luego despues ahora tambien ademas otro otra otros otras todo toda todos todas
cada ambos ambas mismo misma`)

func conjuntoPal(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// tokP is a word-problem token.
type tokP struct {
	orig   string
	w      string   // normalized
	num    *big.Rat // numbers (digits or words)
	punto  bool     // . ; ? ! ¿ ¡ newline
	mayus  bool     // written with an initial capital
	inicio bool     // first word of a sentence
}

func (t tokP) esNum() bool { return t.num != nil }

// tokensProblema splits a problem into words, numbers (digits, "3,5", words, "y media"), "%" and
// sentence marks.
func tokensProblema(s string) []tokP {
	var out []tokP
	inicio := true
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r) && r != '\n':
			i += n
		case unicode.IsLetter(r):
			j := i
			for j < len(s) {
				r2, n2 := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(r2) {
					break
				}
				j += n2
			}
			w := s[i:j]
			norm := nucleo.Normalizar(w)
			if (norm == "km" || norm == "m") && strings.HasPrefix(s[j:], "/h") || norm == "m" && strings.HasPrefix(s[j:], "/s") {
				norm += s[j : j+2]
				j += 2
				w = s[i:j]
			}
			out = append(out, tokP{orig: w, w: norm, mayus: unicode.IsUpper(r), inicio: inicio})
			inicio = false
			i = j
		case r >= '0' && r <= '9':
			j := i
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j+1 < len(s) && (s[j] == ',' || s[j] == '.') && s[j+1] >= '0' && s[j+1] <= '9' {
				k := j + 1
				for k < len(s) && s[k] >= '0' && s[k] <= '9' {
					k++
				}
				j = k
			}
			v, _ := new(big.Rat).SetString(strings.Replace(s[i:j], ",", ".", 1))
			out = append(out, tokP{orig: s[i:j], w: s[i:j], num: v, inicio: inicio})
			inicio = false
			i = j
		case strings.ContainsRune(".;?!¿¡\n", r):
			out = append(out, tokP{orig: string(r), w: string(r), punto: true})
			inicio = true
			i += n
		default:
			out = append(out, tokP{orig: string(r), w: string(r), inicio: inicio})
			i += n
		}
	}
	return juntarNumeros(out)
}

var unidadesUno = conjuntoPal(`hora minuto segundo kilo kilometro litro euro metro dia semana mes ano año docena`)

// juntarNumeros merges numbers written in words, "y media", "media hora", and "N por ciento".
func juntarNumeros(ts []tokP) []tokP {
	var out []tokP
	ws := make([]string, len(ts))
	for i, t := range ts {
		ws[i] = t.w
	}
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if t.num == nil && !t.punto {
			if v, k := numeroPalabras(ws, i); k > 0 {
				t.num = big.NewRat(v, 1)
				t.orig = strings.Join(ws[i:i+k], " ")
				i += k - 1
			} else if (t.w == "un" || t.w == "una") && i+1 < len(ts) && unidadesUno[singular(ts[i+1].w)] {
				t.num = big.NewRat(1, 1)
			} else if t.w == "media" && i+1 < len(ts) && strings.HasPrefix(ts[i+1].w, "hora") {
				t.num = big.NewRat(1, 2)
			}
		}
		out = append(out, t)
		// "2 horas y media", "1 hora y cuarto"
		if len(out) >= 2 && t.num == nil && strings.HasPrefix(t.w, "hora") && out[len(out)-2].num != nil &&
			i+2 < len(ts) && ts[i+1].w == "y" && (ts[i+2].w == "media" || ts[i+2].w == "cuarto") {
			extra := big.NewRat(1, 2)
			if ts[i+2].w == "cuarto" {
				extra = big.NewRat(1, 4)
			}
			n := &out[len(out)-2]
			n.num = new(big.Rat).Add(n.num, extra)
			n.orig += " " + t.orig + " y " + ts[i+2].orig
			i += 2
		}
		// "15 por ciento" → 15 %
		if t.num != nil && i+2 < len(ts) && ts[i+1].w == "por" && ts[i+2].w == "ciento" {
			out = append(out, tokP{orig: "%", w: "%"})
			i += 2
		}
	}
	return out
}

// singular undoes a Spanish plural: caramelos → caramelo, peces → pez, canicas → canica.
func singular(w string) string {
	n := len(w)
	switch {
	case n > 4 && strings.HasSuffix(w, "ces"):
		return w[:n-3] + "z"
	case n > 4 && strings.HasSuffix(w, "es") && !strings.ContainsRune("aeiou", rune(w[n-3])) && w[n-3] != 's':
		return w[:n-2]
	case n > 3 && strings.HasSuffix(w, "s") && strings.ContainsRune("aeiou", rune(w[n-2])):
		return w[:n-1]
	}
	return w
}

// lemaVerbo returns the infinitive of a conjugated form of a known verb ("regala" → "regalar").
func lemaVerbo(w string) (string, bool) {
	if v, ok := irregularesVerbo[w]; ok {
		return v, true
	}
	if _, ok := efectosBase[w]; ok {
		return w, true
	}
	for _, suf := range []string{"aron", "ieron", "aban", "ado", "ido", "amos", "emos", "imos", "aba", "ia",
		"an", "en", "as", "es", "a", "e", "o"} {
		if !strings.HasSuffix(w, suf) || len(w)-len(suf) < 2 {
			continue
		}
		raiz := w[:len(w)-len(suf)]
		for _, inf := range []string{"ar", "er", "ir"} {
			if _, ok := efectosBase[raiz+inf]; ok {
				return raiz + inf, true
			}
		}
	}
	return "", false
}

// infinitivoProbable guesses the infinitive of an unknown verb: troca → trocar, bebe → beber.
func infinitivoProbable(w string) string {
	for _, c := range []struct{ suf, inf string }{{"aron", "ar"}, {"ieron", "er"}, {"an", "ar"}, {"en", "er"},
		{"a", "ar"}, {"e", "er"}, {"o", "ar"}} {
		if strings.HasSuffix(w, c.suf) && len(w) > len(c.suf)+1 {
			return w[:len(w)-len(c.suf)] + c.inf
		}
	}
	return w
}

// efectoDe looks a verb up: learned effects first, then the base table.
func efectoDe(v Verbos, lema string) (string, bool) {
	if v != nil {
		if e, ok := v.Efecto(lema); ok {
			return e, true
		}
	}
	e, ok := efectosBase[lema]
	return e, ok
}

// ---- sentences ----

type frase struct {
	toks     []tokP
	pregunta bool
}

var nombresMagnitud = map[string]string{"distancia": "la distancia", "tiempo": "el tiempo", "velocidad": "la velocidad"}

var interrogativos = conjuntoPal(`cuanto cuanta cuantos cuantas que cual cuales quien donde cuando como`)

// empiezaPregunta: "cuántos…", "qué distancia…" (a bare relative "que" is not a question).
func empiezaPregunta(ts []tokP) bool {
	if len(ts) == 0 || !interrogativos[ts[0].w] {
		return false
	}
	if ts[0].w == "que" || ts[0].w == "como" || ts[0].w == "cuando" || ts[0].w == "donde" || ts[0].w == "quien" {
		return len(ts) > 1 && conjuntoPal("distancia velocidad edad numero tiempo precio hora cantidad")[ts[1].w]
	}
	return true
}

func frases(ts []tokP) []frase {
	var out []frase
	var cur []tokP
	preg := false
	cerrar := func(fin string) {
		if len(cur) > 0 {
			if fin == "?" || len(cur) > 0 && (interrogativos[cur[0].w] || cur[0].w == "a" && len(cur) > 1 && interrogativos[cur[1].w]) {
				preg = true
			}
			out = append(out, frase{toks: cur, pregunta: preg})
		}
		cur, preg = nil, false
	}
	for _, t := range ts {
		if t.punto {
			if t.w == "¿" {
				cerrar("")
				preg = true
				continue
			}
			if len(cur) > 0 {
				cerrar(t.w)
			}
			continue
		}
		cur = append(cur, t)
	}
	cerrar("")
	// ", ¿qué distancia recorre?" without "¿": split the last sentence at a comma before an interrogative
	var res []frase
	for _, f := range out {
		k := -1
		for i := 0; i+1 < len(f.toks); i++ {
			if f.toks[i].w == "," && empiezaPregunta(f.toks[i+1:]) {
				k = i
			}
		}
		if k > 0 && !f.pregunta {
			res = append(res, frase{toks: f.toks[:k]}, frase{toks: f.toks[k+1:], pregunta: true})
			continue
		}
		res = append(res, f)
	}
	return res
}

// clausulas splits a sentence at "y", "pero", "luego", "después" or "," when what follows starts a new
// clause (a verb, a name, "entre", "se", "le").
func clausulas(ts []tokP, v Verbos) [][]tokP {
	var out [][]tokP
	ini := 0
	for i := 0; i < len(ts); i++ {
		if !(ts[i].w == "y" || ts[i].w == "pero" || ts[i].w == "luego" || ts[i].w == "despues" || ts[i].w == "," || ts[i].w == "entonces") {
			continue
		}
		if i+1 >= len(ts) {
			continue
		}
		sig := ts[i+1]
		nuevo := false
		switch {
		case ts[i].w == "," && sig.w == "que" && i+2 < len(ts):
			_, esV := lemaVerbo(ts[i+2].w)
			nuevo = esV
		case sig.w == "entre" || sig.w == "se" || sig.w == "le" || sig.w == "les" || sig.w == "luego" || sig.w == "despues" || sig.w == "dentro" || sig.w == "hace":
			nuevo = true
		case esNombre(sig):
			nuevo = true
		default:
			if _, ok := lemaVerbo(sig.w); ok {
				nuevo = true
			} else if i+2 < len(ts) && ts[i+2].esNum() && pareceVerbo(sig.w) {
				nuevo = true
			}
		}
		if nuevo {
			if ini < i {
				out = append(out, ts[ini:i])
			}
			ini = i + 1
		}
	}
	if ini < len(ts) {
		out = append(out, ts[ini:])
	}
	return out
}

// pareceVerbo: a word that could be a conjugated verb (for unknown verbs).
func pareceVerbo(w string) bool {
	if funcionalesP[w] || noVerbos[w] || interrogativos[w] || len(w) < 3 {
		return false
	}
	for _, s := range []string{"a", "e", "o", "an", "en", "aron", "ieron"} {
		if strings.HasSuffix(w, s) {
			return true
		}
	}
	return false
}

var mayusculasNoNombre = conjuntoPal(`el la los las un una unos unas si en de del al cuantos cuantas cuanto cuanta
que cual entre a con por para y o hay dentro hace se le juntos ambos todos luego despues cada mi tu su yo
calcula halla resuelve`)

var parientes = conjuntoPal(`padre madre hijo hija hermano hermana abuelo abuela tio tia primo prima amigo amiga
nieto nieta`)

func esNombre(t tokP) bool {
	return t.mayus && !mayusculasNoNombre[t.w] && t.num == nil && len(t.w) > 1
}

// ---- Plantear ----

// Plantear turns a word problem into equations with two front-ends: a translation grammar ("el doble
// de un número más 3 es 11" → 2·x + 3 = 11; ages and relations between people) and a situation model
// (people, objects and verbs that change quantities; rates such as distance = speed × time, prices and
// discounts). Unknown verbs are listed in Desconocido; the caller asks and teaches them with v.Aprender.
func Plantear(p *nucleo.Pregunta, v Verbos) ([]Planteamiento, error) {
	if p == nil {
		return nil, nucleo.ErrNoEntiendo
	}
	texto := p.Texto
	if strings.TrimSpace(p.Resto) != "" && p.Codigo != "" {
		texto = p.Resto
	}
	ts := tokensProblema(texto)
	fs := frases(ts)
	var out []Planteamiento
	if pl, ok := traducir(fs); ok {
		out = append(out, pl)
	}
	if pl, ok := relaciones(fs, v); ok {
		out = append(out, pl)
	}
	if pl, ok := movimiento(fs); ok {
		out = append(out, pl)
	}
	if pl, ok := precio(fs); ok {
		out = append(out, pl)
	}
	if len(out) == 0 {
		if pl, ok := situacion(fs, v); ok {
			out = append(out, pl)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no sé plantear este problema", nucleo.ErrNoEntiendo)
	}
	return out, nil
}

// ---- translation grammar ----

type gram struct {
	t     []tokP
	i     int
	mapeo []string
	inc   map[string]string
	usaY  bool
}

func (g *gram) w(k int) string {
	if g.i+k < len(g.t) {
		return g.t[g.i+k].w
	}
	return ""
}

// frase accepts one of the "|"-separated multiword phrases at the cursor.
func (g *gram) frase(alts string) bool {
	for _, a := range strings.Split(alts, "|") {
		ps := strings.Fields(a)
		ok := true
		for k, p := range ps {
			if g.w(k) != p {
				ok = false
				break
			}
		}
		if ok {
			g.i += len(ps)
			return true
		}
	}
	return false
}

func (g *gram) texto(desde int) string {
	var ps []string
	for _, t := range g.t[desde:g.i] {
		ps = append(ps, t.orig)
	}
	return strings.Join(ps, " ")
}

func (g *gram) mapear(desde int, e Expr) {
	g.mapeo = append(g.mapeo, "«"+g.texto(desde)+"» → "+e.String())
}

func (g *gram) expr() (Expr, bool) {
	a, ok := g.term()
	if !ok {
		return nil, false
	}
	for {
		desde := g.i
		switch {
		case g.frase("mas|+|sumado a|aumentado en|mas que"):
			b, ok := g.term()
			if !ok {
				g.i = desde
				return a, true
			}
			a = Op{Op: '+', A: a, B: b}
		case g.frase("menos|-|disminuido en|restado|menos que"):
			b, ok := g.term()
			if !ok {
				g.i = desde
				return a, true
			}
			a = Op{Op: '-', A: a, B: b}
		default:
			return a, true
		}
	}
}

func (g *gram) term() (Expr, bool) {
	a, ok := g.factor()
	if !ok {
		return nil, false
	}
	for {
		desde := g.i
		switch {
		case g.frase("multiplicado por|por|*|x"):
			b, ok := g.factor()
			if !ok {
				g.i = desde
				return a, true
			}
			a = Op{Op: '*', A: a, B: b}
		case g.frase("dividido entre|dividido por|entre|/"):
			b, ok := g.factor()
			if !ok {
				g.i = desde
				return a, true
			}
			a = Op{Op: '/', A: a, B: b}
		default:
			return a, true
		}
	}
}

func ent(i int64) Expr { return Num{V: big.NewRat(i, 1)} }

func (g *gram) incognita(nombre, desc string) Expr {
	if _, ok := g.inc[nombre]; !ok {
		g.inc[nombre] = desc
	}
	if nombre == "y" {
		g.usaY = true
	}
	return Var{Nombre: nombre}
}

func (g *gram) factor() (Expr, bool) {
	desde := g.i
	mult := func(k int64, frases string) (Expr, bool) {
		if !g.frase(frases) {
			return nil, false
		}
		f, ok := g.factor()
		if !ok {
			g.i = desde
			return nil, false
		}
		e := Op{Op: '*', A: ent(k), B: f}
		g.mapear(desde, e)
		return e, true
	}
	div := func(k int64, frases string) (Expr, bool) {
		if !g.frase(frases) {
			return nil, false
		}
		f, ok := g.factor()
		if !ok {
			g.i = desde
			return nil, false
		}
		e := Op{Op: '/', A: f, B: ent(k)}
		g.mapear(desde, e)
		return e, true
	}
	x := func() Expr { return g.incognita("x", "el número") }
	if e, ok := mult(2, "el doble de|el doble del|doble de|el duplo de"); ok {
		return e, true
	}
	if e, ok := mult(3, "el triple de|el triple del|triple de"); ok {
		return e, true
	}
	if e, ok := mult(4, "el cuadruple de|el cuadruple del"); ok {
		return e, true
	}
	if e, ok := div(2, "la mitad de|la mitad del|mitad de"); ok {
		return e, true
	}
	if e, ok := div(3, "un tercio de|la tercera parte de|un tercio del|la tercera parte del"); ok {
		return e, true
	}
	if e, ok := div(4, "un cuarto de|la cuarta parte de|un cuarto del|la cuarta parte del"); ok {
		return e, true
	}
	// relative to x: "su doble", "su consecutivo"
	relativos := []struct {
		frases string
		e      func() Expr
	}{
		{"su doble|el doble", func() Expr { return Op{Op: '*', A: ent(2), B: x()} }},
		{"su triple|el triple", func() Expr { return Op{Op: '*', A: ent(3), B: x()} }},
		{"su mitad", func() Expr { return Op{Op: '/', A: x(), B: ent(2)} }},
		{"su cuadrado", func() Expr { return Op{Op: '^', A: x(), B: ent(2)} }},
		{"su consecutivo|su siguiente|el siguiente|el consecutivo|el numero siguiente", func() Expr { return Op{Op: '+', A: x(), B: ent(1)} }},
		{"su anterior|el anterior|el numero anterior", func() Expr { return Op{Op: '-', A: x(), B: ent(1)} }},
	}
	for _, r := range relativos {
		if g.frase(r.frases) && !g.frase("de|del") {
			e := r.e()
			g.mapear(desde, e)
			return e, true
		}
		g.i = desde
	}
	if g.frase("el cuadrado de|el cuadrado del") {
		if f, ok := g.factor(); ok {
			e := Op{Op: '^', A: f, B: ent(2)}
			g.mapear(desde, e)
			return e, true
		}
		g.i = desde
		return nil, false
	}
	if g.frase("el cubo de|el cubo del") {
		if f, ok := g.factor(); ok {
			e := Op{Op: '^', A: f, B: ent(3)}
			g.mapear(desde, e)
			return e, true
		}
		g.i = desde
		return nil, false
	}
	if g.frase("el consecutivo de|el siguiente de") {
		if f, ok := g.factor(); ok {
			e := Op{Op: '+', A: f, B: ent(1)}
			g.mapear(desde, e)
			return e, true
		}
		g.i = desde
		return nil, false
	}
	// "la suma de dos números consecutivos"
	if g.frase("la suma de dos numeros consecutivos|la suma de dos enteros consecutivos") {
		e := Op{Op: '+', A: x(), B: Op{Op: '+', A: x(), B: ent(1)}}
		g.mapear(desde, e)
		return e, true
	}
	if g.frase("el producto de dos numeros consecutivos") {
		e := Op{Op: '*', A: x(), B: Op{Op: '+', A: x(), B: ent(1)}}
		g.mapear(desde, e)
		return e, true
	}
	binarios := []struct {
		frases, sep string
		op          byte
	}{
		{"la suma de|la suma del", "y|mas", '+'},
		{"la diferencia de|la diferencia entre|la resta de", "y|menos", '-'},
		{"el producto de|el producto del", "y|por", '*'},
		{"el cociente de|el cociente entre|la division de", "y|entre", '/'},
	}
	for _, b := range binarios {
		if !g.frase(b.frases) {
			continue
		}
		a, ok := g.expr()
		if ok && g.frase(b.sep) {
			c, ok2 := g.expr()
			if ok2 {
				e := Op{Op: b.op, A: a, B: c}
				g.mapear(desde, e)
				return e, true
			}
		}
		g.i = desde
		return nil, false
	}
	// "N veces E", "N% de E"
	if g.i < len(g.t) && g.t[g.i].esNum() {
		v := g.t[g.i].num
		g.i++
		if g.frase("veces") {
			if f, ok := g.factor(); ok {
				e := Op{Op: '*', A: Num{V: v}, B: f}
				g.mapear(desde, e)
				return e, true
			}
			g.i = desde
			return nil, false
		}
		if g.frase("% de|% del") {
			if f, ok := g.factor(); ok {
				e := Op{Op: '*', A: Num{V: new(big.Rat).Quo(v, big.NewRat(100, 1))}, B: f}
				g.mapear(desde, e)
				return e, true
			}
			g.i = desde
			return nil, false
		}
		return Num{V: v}, true
	}
	if g.frase("un numero cualquiera|un numero|cierto numero|el numero|dicho numero|ese numero|este numero|un entero|una cantidad|el primero|el primer numero|x") {
		e := x()
		g.mapear(desde, e)
		return e, true
	}
	if g.frase("otro numero|el otro numero|el otro|el segundo numero|el segundo|y") {
		e := g.incognita("y", "el otro número")
		g.mapear(desde, e)
		return e, true
	}
	if g.frase("el") || g.frase("la") {
		if f, ok := g.factor(); ok {
			return f, true
		}
		g.i = desde
	}
	return nil, false
}

var igualdades = "es igual a|es igual al|es|da como resultado|da|nos da|resulta|equivale a|se obtiene|obtengo|obtenemos|sale|son|=|vale"

// traducir reads "el doble de un número más 3 es 11" and "si a un número le sumo 5 obtengo 12".
func traducir(fs []frase) (Planteamiento, bool) {
	var ecs []string
	g := &gram{inc: map[string]string{}}
	for _, f := range fs {
		if f.pregunta {
			continue
		}
		toks := f.toks
		// "si a un número le sumo 5, obtengo 12"
		if e, ok := traducirSi(g, toks); ok {
			ecs = append(ecs, e)
			continue
		}
		g.t, g.i = toks, 0
		l, ok := g.expr()
		if !ok {
			continue
		}
		g.frase(",")
		if !g.frase(igualdades) {
			continue
		}
		r, ok := g.expr()
		if !ok || g.i < len(g.t) && !soloRelleno(g.t[g.i:]) {
			continue
		}
		e := Op{Op: '=', A: l, B: r}
		if len(variables(l))+len(variables(r)) == 0 {
			continue
		}
		ecs = append(ecs, textoEcuacion(e))
	}
	if len(ecs) == 0 || len(g.inc) == 0 {
		return Planteamiento{}, false
	}
	pl := Planteamiento{Ecuaciones: ecs, Incognitas: g.inc, Pregunta: "x", Mapeo: g.mapeo, Tipo: "traduccion",
		Respuesta: "El número es %s"}
	if g.usaY {
		pl.Respuesta = "Los números son %s"
	}
	return pl, true
}

func soloRelleno(ts []tokP) bool {
	for _, t := range ts {
		if !(t.w == "," || t.w == "." || funcionalesP[t.w]) {
			return false
		}
	}
	return true
}

// textoEcuacion writes an equation for Ecuaciones (machine form, with "*").
func textoEcuacion(e Expr) string {
	o := e.(Op)
	return strings.ReplaceAll(o.A.String()+" = "+o.B.String(), "·", "*")
}

func traducirSi(g *gram, ts []tokP) (string, bool) {
	g.t, g.i = ts, 0
	if !g.frase("si a|si al") {
		return "", false
	}
	base, ok := g.factor()
	if !ok {
		return "", false
	}
	g.frase("le|se le")
	var op byte
	switch {
	case g.frase("sumo|sumamos|anado|añado|sumas|se suma|le sumo|agrego"):
		op = '+'
	case g.frase("resto|restamos|quito|restas|se resta|le resto"):
		op = '-'
	case g.frase("multiplico por|multiplicamos por|multiplicas por"):
		op = '*'
	case g.frase("divido entre|divido por|dividimos entre|divides entre"):
		op = '/'
	default:
		return "", false
	}
	k, ok := g.factor()
	if !ok {
		return "", false
	}
	g.frase(",")
	if !g.frase("obtengo|obtenemos|da|me da|nos da|sale|resulta|se obtiene|queda|obtienes|tengo") {
		return "", false
	}
	r, ok := g.expr()
	if !ok {
		return "", false
	}
	l := Op{Op: op, A: base, B: k}
	g.mapeo = append(g.mapeo, "«"+g.texto(0)+"» → "+l.String()+" = "+r.String())
	return textoEcuacion(Op{Op: '=', A: l, B: r}), true
}

// ---- relations between people (ages, "entre los dos suman") ----

type persona struct {
	nombre string
	v      string
}

func relaciones(fs []frase, v Verbos) (Planteamiento, bool) {
	var personas []persona
	idx := map[string]int{}
	letras := []string{"x", "y", "z", "u", "v", "w"}
	ver := func(t tokP, prev tokP) (string, bool) {
		nom := ""
		switch {
		case esNombre(t):
			nom = t.orig
		case parientes[singular(t.w)] && (prev.w == "su" || prev.w == "el" || prev.w == "la" || prev.w == "un" || prev.w == "una" || prev.w == "del"):
			nom = singular(t.w)
		default:
			return "", false
		}
		if _, ok := idx[nom]; !ok {
			if len(personas) >= len(letras) {
				return "", false
			}
			idx[nom] = len(personas)
			personas = append(personas, persona{nombre: nom, v: letras[len(personas)]})
		}
		return nom, true
	}
	hayRelacion := false
	for _, f := range fs {
		for i, t := range f.toks {
			prev := tokP{}
			if i > 0 {
				prev = f.toks[i-1]
			}
			ver(t, prev)
			switch t.w {
			case "doble", "triple", "cuadruple", "mitad", "veces", "suman", "edades", "mayor", "menor", "ambos", "juntos":
				hayRelacion = true
			}
		}
	}
	if len(personas) < 2 || !hayRelacion {
		return Planteamiento{}, false
	}
	var ecs, mapeo []string
	unidad := ""
	for _, f := range fs {
		if f.pregunta {
			continue
		}
		desp := desplazamiento(f.toks)
		for _, c := range clausulas(f.toks, v) {
			e, desc, ok := relacionClausula(c, idx, personas, ver, desp)
			if !ok {
				continue
			}
			for _, t := range c {
				if t.w == "edad" || t.w == "edades" || t.w == "anos" || t.w == "años" {
					unidad = "años"
				}
			}
			ecs = append(ecs, e)
			mapeo = append(mapeo, "«"+textoToks(c)+"» → "+desc)
		}
	}
	if len(ecs) == 0 {
		return Planteamiento{}, false
	}
	inc := map[string]string{}
	var partes []string
	for _, p := range personas {
		if unidad == "años" {
			inc[p.v] = "la edad de " + p.nombre
		} else {
			inc[p.v] = "lo que tiene " + p.nombre
		}
		partes = append(partes, p.nombre+" = "+p.v)
	}
	mapeo = append([]string{"Llamo " + strings.Join(partes, ", ")}, mapeo...)
	return Planteamiento{Ecuaciones: ecs, Incognitas: inc, Pregunta: personas[0].v, Mapeo: mapeo, Tipo: "relaciones",
		Respuesta: "%s", Unidad: unidad}, true
}

func textoToks(ts []tokP) string {
	var ps []string
	for _, t := range ts {
		ps = append(ps, t.orig)
	}
	return strings.Join(ps, " ")
}

// relacionClausula reads one clause: "Ana tiene el triple de la edad de Luis" → x = 3*y;
// "entre los dos suman 48" → x + y = 48; "Ana tiene 30 años" → x = 30; "dentro de 5 años…" shifts.
func relacionClausula(c []tokP, idx map[string]int, ps []persona, ver func(tokP, tokP) (string, bool), desp *big.Rat) (string, string, bool) {
	var nombres []string
	var numeros []*big.Rat
	desplazar := new(big.Rat).Set(desp)
	pal := map[string]bool{}
	for i, t := range c {
		prev := tokP{}
		if i > 0 {
			prev = c[i-1]
		}
		if nom, ok := ver(t, prev); ok {
			nombres = append(nombres, nom)
		}
		pal[t.w] = true
		if t.esNum() {
			// "dentro de 5 años" / "hace 5 años"
			if i >= 2 && c[i-2].w == "dentro" && c[i-1].w == "de" || i >= 1 && c[i-1].w == "hace" {
				continue // already in desp
			}
			numeros = append(numeros, t.num)
		}
	}
	varDe := func(nom string) Expr {
		x := Var{Nombre: ps[idx[nom]].v}
		if desplazar.Sign() != 0 {
			return Op{Op: '+', A: x, B: Num{V: desplazar}}
		}
		return x
	}
	if desplazar.Sign() < 0 {
		varDe = func(nom string) Expr {
			return Op{Op: '-', A: Var{Nombre: ps[idx[nom]].v}, B: Num{V: new(big.Rat).Neg(desplazar)}}
		}
	}
	ec := func(l, r Expr) (string, string, bool) {
		e := Op{Op: '=', A: l, B: r}
		return textoEcuacion(e), l.String() + " = " + r.String(), true
	}
	// sums: "entre los dos suman 48", "la suma de sus edades es 48", "juntos tienen 30"
	if (pal["entre"] && (pal["dos"] || pal["ambos"] || pal["todos"] || pal["tres"])) || pal["juntos"] || pal["edades"] && pal["suma"] || pal["suman"] && len(nombres) == 0 {
		if len(numeros) >= 1 {
			var suma Expr
			for _, p := range ps {
				var t Expr = Var{Nombre: p.v}
				if desplazar.Sign() != 0 {
					t = Op{Op: '+', A: t, B: Num{V: desplazar}}
				}
				if suma == nil {
					suma = t
				} else {
					suma = Op{Op: '+', A: suma, B: t}
				}
			}
			return ec(suma, Num{V: numeros[len(numeros)-1]})
		}
	}
	if len(nombres) == 0 {
		return "", "", false
	}
	a := nombres[0]
	if len(nombres) >= 2 && nombres[1] != a {
		b := nombres[1]
		var k *big.Rat
		switch {
		case pal["doble"]:
			k = big.NewRat(2, 1)
		case pal["triple"]:
			k = big.NewRat(3, 1)
		case pal["cuadruple"]:
			k = big.NewRat(4, 1)
		case pal["mitad"]:
			k = big.NewRat(1, 2)
		case pal["tercio"] || pal["tercera"]:
			k = big.NewRat(1, 3)
		case pal["veces"] && len(numeros) > 0:
			k = numeros[0]
		}
		if k != nil {
			return ec(varDe(a), Op{Op: '*', A: Num{V: k}, B: varDe(b)})
		}
		if len(numeros) > 0 && (pal["mas"] || pal["mayor"]) {
			return ec(varDe(a), Op{Op: '+', A: varDe(b), B: Num{V: numeros[0]}})
		}
		if len(numeros) > 0 && (pal["menos"] || pal["menor"]) {
			return ec(varDe(a), Op{Op: '-', A: varDe(b), B: Num{V: numeros[0]}})
		}
		return "", "", false
	}
	if len(numeros) == 1 && (pal["tiene"] || pal["tenia"] || pal["tendra"] || pal["cumple"] || pal["es"]) {
		return ec(varDe(a), Num{V: numeros[0]})
	}
	return "", "", false
}

// desplazamiento reads "dentro de N años" (+N) and "hace N años" (−N) in a sentence.
func desplazamiento(ts []tokP) *big.Rat {
	d := new(big.Rat)
	for i, t := range ts {
		if !t.esNum() {
			continue
		}
		if i >= 2 && ts[i-2].w == "dentro" && ts[i-1].w == "de" {
			d.Add(d, t.num)
		} else if i >= 1 && ts[i-1].w == "hace" {
			d.Sub(d, t.num)
		}
	}
	return d
}

// ---- rates: distance = speed × time ----

// magnitud is a quantity read from the text with its unit.
type magnitud struct {
	v      *big.Rat
	unidad string // "km/h" "m/s" | "h" "min" "s" | "km" "m"
	txt    string
}

// aBase converts a quantity to the base system: km, h, km/h (or m, s, m/s).
func (m magnitud) aBase(metrico bool) *big.Rat {
	f := big.NewRat(1, 1)
	switch {
	case !metrico && m.unidad == "min":
		f = big.NewRat(1, 60)
	case !metrico && m.unidad == "s":
		f = big.NewRat(1, 3600)
	case !metrico && m.unidad == "m":
		f = big.NewRat(1, 1000)
	case !metrico && m.unidad == "m/s":
		f = big.NewRat(18, 5)
	case metrico && m.unidad == "h":
		f = big.NewRat(3600, 1)
	case metrico && m.unidad == "min":
		f = big.NewRat(60, 1)
	case metrico && m.unidad == "km":
		f = big.NewRat(1000, 1)
	case metrico && m.unidad == "km/h":
		f = big.NewRat(5, 18)
	}
	return new(big.Rat).Mul(m.v, f)
}

func unidadTiempo(w string) string {
	switch {
	case strings.HasPrefix(w, "hora"):
		return "h"
	case strings.HasPrefix(w, "minuto") || w == "min":
		return "min"
	case strings.HasPrefix(w, "segundo") || w == "seg":
		return "s"
	}
	return ""
}

func unidadDistancia(w string) string {
	switch {
	case w == "km" || strings.HasPrefix(w, "kilometro"):
		return "km"
	case w == "m" || strings.HasPrefix(w, "metro"):
		return "m"
	}
	return ""
}

// movimiento: distance = speed × time, with km↔m and h↔min↔s conversions.
func movimiento(fs []frase) (Planteamiento, bool) {
	var vel, tiempo, dist *magnitud
	pregunta, unidadPregunta := "", ""
	for _, f := range fs {
		ts := f.toks
		for i, t := range ts {
			sig := ""
			if i+1 < len(ts) {
				sig = ts[i+1].w
			}
			if f.pregunta {
				switch {
				case t.w == "distancia" || strings.HasPrefix(t.w, "recorr") || (t.w == "cuantos" && unidadDistancia(sig) != ""):
					if pregunta == "" {
						pregunta, unidadPregunta = "distancia", unidadDistancia(sig)
					}
				case strings.HasPrefix(t.w, "tard") || t.w == "tiempo" || ((t.w == "cuantas" || t.w == "cuantos") && unidadTiempo(sig) != ""):
					if pregunta == "" {
						pregunta, unidadPregunta = "tiempo", unidadTiempo(sig)
					}
				case t.w == "velocidad" || t.w == "rapido" || t.w == "deprisa":
					if pregunta == "" {
						pregunta = "velocidad"
					}
				}
			}
			if !t.esNum() || i+1 >= len(ts) {
				continue
			}
			txt := t.orig + " " + ts[i+1].orig
			if strings.Contains(t.orig, " ") && unidadTiempo(sig) != "" {
				txt = t.orig // "2 horas y media"
			}
			switch {
			case sig == "km/h" || sig == "m/s":
				vel = &magnitud{t.num, sig, txt}
			case unidadDistancia(sig) != "" && i+3 < len(ts) && ts[i+2].w == "por" && strings.HasPrefix(ts[i+3].w, "hora"):
				vel = &magnitud{t.num, "km/h", txt + " por hora"}
				if unidadDistancia(sig) == "m" {
					vel.v = new(big.Rat).Quo(t.num, big.NewRat(1000, 1))
				}
			case unidadTiempo(sig) != "":
				tiempo = &magnitud{t.num, unidadTiempo(sig), txt}
			case unidadDistancia(sig) != "":
				dist = &magnitud{t.num, unidadDistancia(sig), txt}
			}
		}
	}
	if pregunta == "" {
		return Planteamiento{}, false
	}
	metrico := vel != nil && vel.unidad == "m/s"
	uD, uT, uV := "km", "h", "km/h"
	if metrico {
		uD, uT, uV = "m", "s", "m/s"
	}
	desc := func(nombre string, m *magnitud, u string) string {
		b := m.aBase(metrico)
		s := "«" + m.txt + "» → " + nombre + " = " + textoRat(b) + " " + u
		return s
	}
	var ec, resp, unidad string
	var mapeo []string
	escala := big.NewRat(1, 1)
	switch {
	case pregunta == "distancia" && vel != nil && tiempo != nil:
		v, t := vel.aBase(metrico), tiempo.aBase(metrico)
		unidad = uD
		if unidadPregunta != "" && unidadPregunta != uD {
			unidad = unidadPregunta
			escala = map[string]*big.Rat{"m": big.NewRat(1000, 1), "km": big.NewRat(1, 1000)}[unidadPregunta]
		}
		ec = "x = " + v.RatString() + "*(" + t.RatString() + ")*(" + escala.RatString() + ")"
		mapeo = []string{desc("velocidad", vel, uV), desc("tiempo", tiempo, uT), "distancia = velocidad · tiempo = " + textoRat(v) + "·" + textoRat(t) + " " + uD}
		resp = "Recorre %s " + unidad
	case pregunta == "tiempo" && vel != nil && dist != nil:
		v, d := vel.aBase(metrico), dist.aBase(metrico)
		if v.Sign() == 0 {
			return Planteamiento{}, false
		}
		unidad = uT
		if unidadPregunta != "" && unidadPregunta != uT {
			unidad = unidadPregunta
			escala = map[string]*big.Rat{"min": big.NewRat(60, 1), "s": big.NewRat(3600, 1), "h": big.NewRat(1, 3600)}[unidadPregunta]
			if metrico && unidadPregunta == "min" {
				escala = big.NewRat(1, 60)
			}
		}
		ec = "x = " + d.RatString() + "/(" + v.RatString() + ")*(" + escala.RatString() + ")"
		mapeo = []string{desc("distancia", dist, uD), desc("velocidad", vel, uV), "tiempo = distancia / velocidad"}
		resp = "Tarda %s " + map[string]string{"h": "horas", "min": "minutos", "s": "segundos"}[unidad]
	case pregunta == "velocidad" && dist != nil && tiempo != nil:
		metrico = dist.unidad == "m" && tiempo.unidad == "s"
		uD, uT, uV = "km", "h", "km/h"
		if metrico {
			uD, uT, uV = "m", "s", "m/s"
		}
		d, t := dist.aBase(metrico), tiempo.aBase(metrico)
		if t.Sign() == 0 {
			return Planteamiento{}, false
		}
		ec = "x = " + d.RatString() + "/(" + t.RatString() + ")"
		mapeo = []string{desc("distancia", dist, uD), desc("tiempo", tiempo, uT), "velocidad = distancia / tiempo"}
		resp, unidad = "Va a %s "+uV, uV
	default:
		return Planteamiento{}, false
	}
	if escala.Cmp(big.NewRat(1, 1)) != 0 {
		mapeo = append(mapeo, "lo paso a "+unidad)
	}
	return Planteamiento{Ecuaciones: []string{ec}, Incognitas: map[string]string{"x": nombresMagnitud[pregunta]}, Pregunta: "x",
		Mapeo: mapeo, Tipo: "movimiento", Respuesta: resp, Unidad: unidad}, true
}

// ---- prices and discounts ----

func precio(fs []frase) (Planteamiento, bool) {
	var precioU, cantidad, porcentaje *big.Rat
	var precioTxt, cantTxt, cantUnidad, precioPor string
	sube := false
	preguntaAhorro := false
	hayPregunta := false
	for _, f := range fs {
		ts := f.toks
		for i, t := range ts {
			if f.pregunta {
				switch t.w {
				case "cuesta", "cuestan", "paga", "pago", "pagar", "pagare", "vale", "valen", "gasta", "cobran", "total":
					hayPregunta = true
				case "ahorro", "ahorra", "descuentan", "descuento", "rebajan":
					hayPregunta, preguntaAhorro = true, true
				}
				if t.esNum() && i+1 < len(ts) && cantidad == nil {
					cantidad, cantTxt, cantUnidad = t.num, t.orig+" "+ts[i+1].orig, ts[i+1].w
				}
				continue
			}
			if !t.esNum() || i+1 >= len(ts) {
				continue
			}
			u := ts[i+1].w
			switch {
			case u == "euros" || u == "euro" || u == "€" || u == "dolares" || u == "dolar":
				if precioU == nil {
					precioU, precioTxt = t.num, t.orig+" "+ts[i+1].orig
					if i+3 < len(ts) && (ts[i+2].w == "el" || ts[i+2].w == "la" || ts[i+2].w == "cada" || ts[i+2].w == "por") {
						precioPor = ts[i+3].w
					}
				}
			case u == "%":
				porcentaje = t.num
				for _, x := range ts {
					if x.w == "sube" || x.w == "aumenta" || x.w == "subida" || x.w == "aumento" || x.w == "encarece" || x.w == "incremento" || x.w == "iva" {
						sube = true
					}
				}
			default:
				if cantidad == nil && !(i > 0 && ts[i-1].w == "a") {
					cantidad, cantTxt, cantUnidad = t.num, t.orig+" "+ts[i+1].orig, ts[i+1].w
				}
			}
		}
	}
	if !hayPregunta || precioU == nil {
		return Planteamiento{}, false
	}
	switch {
	case porcentaje != nil:
		p := porcentaje.RatString()
		if preguntaAhorro {
			return Planteamiento{Ecuaciones: []string{"x = " + precioU.RatString() + "*" + p + "/100"},
				Incognitas: map[string]string{"x": "el descuento"}, Pregunta: "x", Tipo: "precio",
				Mapeo:     []string{"«" + precioTxt + "» → precio = " + textoRat(precioU), "descuento = precio · " + p + "/100"},
				Respuesta: "Te ahorras %s euros", Unidad: "euros"}, true
		}
		signo, palabra := "-", "rebaja"
		if sube {
			signo, palabra = "+", "subida"
		}
		return Planteamiento{Ecuaciones: []string{"x = " + precioU.RatString() + "*(1 " + signo + " " + p + "/100)"},
			Incognitas: map[string]string{"x": "el precio final"}, Pregunta: "x", Tipo: "precio",
			Mapeo:     []string{"«" + precioTxt + "» → precio = " + textoRat(precioU), palabra + " del " + p + " %: precio · (1 " + signo + " " + p + "/100)"},
			Respuesta: "Cuesta %s euros", Unidad: "euros"}, true
	case cantidad != nil:
		// grams at a price per kilo, millilitres at a price per litre
		porMil := (precioPor == "kilo" || precioPor == "kg" || precioPor == "kilogramo") &&
			(cantUnidad == "gramos" || cantUnidad == "g" || cantUnidad == "gr" || cantUnidad == "gramo") ||
			precioPor == "litro" && (cantUnidad == "ml" || strings.HasPrefix(cantUnidad, "mililitro"))
		if porMil {
			cantidad = new(big.Rat).Quo(cantidad, big.NewRat(1000, 1))
			cantTxt += " (= " + textoRat(cantidad) + " " + precioPor + ")"
		}
		return Planteamiento{Ecuaciones: []string{"x = " + cantidad.RatString() + "*" + precioU.RatString()},
			Incognitas: map[string]string{"x": "el precio total"}, Pregunta: "x", Tipo: "precio",
			Mapeo:     []string{"«" + cantTxt + "» → cantidad = " + textoRat(cantidad), "«" + precioTxt + "» → precio de cada uno = " + textoRat(precioU), "total = cantidad · precio"},
			Respuesta: "Son %s euros", Unidad: "euros"}, true
	}
	return Planteamiento{}, false
}

// ---- situation model ----

// cantidad is what someone has of something: an initial amount (nil = not said) and the changes.
type cantidad struct {
	ini     Expr
	cambios []cambio
}

type cambio struct {
	op byte
	e  Expr
}

func (c *cantidad) expr() Expr {
	e := Expr(ent(0))
	if c.ini != nil {
		e = c.ini
	}
	for _, k := range c.cambios {
		e = Op{Op: k.op, A: e, B: k.e}
	}
	return e
}

type situ struct {
	v        Verbos
	estado   map[string]map[string]*cantidad // persona → objeto → amount
	orden    []string                        // personas in order
	sujeto   string
	ultimo   string // last person named (for "…, que tenía 3")
	objeto   string
	plural   map[string]string // objeto → as written
	mapeo    []string
	descon   []string
	reparto  Expr
	porVerbo map[string]Expr
}

func (s *situ) cant(p, o string) *cantidad {
	m, ok := s.estado[p]
	if !ok {
		m = map[string]*cantidad{}
		s.estado[p] = m
		s.orden = append(s.orden, p)
	}
	c, ok := m[o]
	if !ok {
		c = &cantidad{}
		m[o] = c
	}
	return c
}

func (s *situ) tiene(p, o string) bool {
	_, ok := s.estado[p][o]
	return ok
}

func situacion(fs []frase, v Verbos) (Planteamiento, bool) {
	s := &situ{v: v, estado: map[string]map[string]*cantidad{}, plural: map[string]string{}, porVerbo: map[string]Expr{}}
	var pregunta []tokP
	for _, f := range fs {
		if f.pregunta {
			pregunta = f.toks
			continue
		}
		for _, c := range clausulas(f.toks, v) {
			s.clausula(c)
		}
	}
	if len(s.descon) > 0 {
		return Planteamiento{Desconocido: s.descon, Tipo: "situacion", Mapeo: s.mapeo}, true
	}
	if pregunta == nil || len(s.estado) == 0 && s.reparto == nil {
		return Planteamiento{}, false
	}
	e, desc, resp, ok := s.responder(pregunta)
	if !ok {
		return Planteamiento{}, false
	}
	return Planteamiento{Ecuaciones: []string{textoEcuacion(Op{Op: '=', A: Var{Nombre: "x"}, B: e})},
		Incognitas: map[string]string{"x": desc}, Pregunta: "x", Mapeo: s.mapeo, Tipo: "situacion",
		Respuesta: resp, Unidad: s.plural[s.objeto]}, true
}

var imperfectos = conjuntoPal(`tenia tenian habia habian tuvo`)

// clausula applies one clause: "[Persona] verbo N objeto [a Persona]".
func (s *situ) clausula(c []tokP) {
	i := 0
	suj := s.sujeto
	explicito := false
	relativo := len(c) > 0 && c[0].w == "que"
	if relativo {
		suj, explicito, i = s.ultimo, true, 1
	}
	for i < len(c) && (funcionalesP[c[i].w] && c[i].w != "se" && c[i].w != "le" && c[i].w != "les" && c[i].w != "en" && c[i].w != "a" || c[i].w == ",") {
		i++
	}
	if i < len(c) && esNombre(c[i]) {
		suj, explicito = c[i].orig, true
		i++
	} else if i+1 < len(c) && (c[i].w == "su" || c[i].w == "el" || c[i].w == "la") && parientes[singular(c[i+1].w)] {
		suj, explicito = singular(c[i+1].w), true
		i += 2
	}
	// verb
	lema, efecto, verbo := "", "", ""
	le := false
	k := i
	for ; k < len(c); k++ {
		w := c[k].w
		if w == "le" || w == "les" {
			le = true
			continue
		}
		if w == "se" || w == "ha" || w == "han" || w == "ya" || w == "luego" || w == "despues" || w == "ahora" || w == "tambien" || w == "en" {
			continue
		}
		if l, ok := lemaVerbo(w); ok {
			lema, verbo = l, w
			if e, ok := efectoDe(s.v, l); ok {
				efecto = e
			}
			if k > 0 && c[k-1].w == "se" && (l == "comer" || l == "beber" || l == "gastar") {
				efecto = "restar"
			}
			break
		}
		if c[k].esNum() || funcionalesP[w] || esNombre(c[k]) {
			continue
		}
		// an unknown word right before a number is an unknown verb
		if pareceVerbo(w) && k+1 < len(c) && (c[k+1].esNum() || k+2 < len(c) && funcionalesP[c[k+1].w] && c[k+2].esNum()) {
			inf := infinitivoProbable(w)
			if e, ok := efectoDe(s.v, inf); ok {
				lema, efecto, verbo = inf, e, w
				break
			}
			if !contiene(s.descon, inf) {
				s.descon = append(s.descon, inf)
			}
			return
		}
	}
	if lema == "" {
		return
	}
	// quantity, object, receiver
	var num, factor *big.Rat
	obj, objTxt, receptor := "", "", ""
	obj2, obj2Txt := "", "" // the noun after a second number: "3 cajas de 12 caramelos"
	mitad, cada, grupos := false, false, false
	for j := k + 1; j < len(c); j++ {
		t := c[j]
		switch {
		case t.esNum() && num == nil:
			num = t.num
		case t.esNum() && factor == nil:
			factor = t.num
			grupos = j > 0 && (c[j-1].w == "de" || c[j-1].w == "con")
			if j+1 < len(c) && c[j+1].num == nil && !funcionalesP[c[j+1].w] && c[j+1].w != "" && unicode.IsLetter([]rune(c[j+1].w)[0]) {
				obj2, obj2Txt = singular(c[j+1].w), c[j+1].orig
				j++
			}
		case t.w == "mitad":
			mitad = true
		case t.w == "cada":
			cada = true
		case (t.w == "a" || t.w == "al") && j+1 < len(c) && esNombre(c[j+1]):
			receptor = c[j+1].orig
			j++
		case (t.w == "a" || t.w == "al") && j+2 < len(c) && c[j+1].w == "su" && parientes[singular(c[j+2].w)]:
			receptor = singular(c[j+2].w)
			j += 2
		case obj == "" && (num != nil || mitad) && t.num == nil && !funcionalesP[t.w] && t.w != "," && !esNombre(t) &&
			t.w != "" && unicode.IsLetter([]rune(t.w)[0]):
			if _, esVerbo := lemaVerbo(t.w); !esVerbo {
				obj, objTxt = singular(t.w), t.orig
			}
		}
	}
	// "3 cajas de 12 caramelos (cada una)": the object is what is inside
	if factor != nil && (cada || grupos) && obj2 != "" && lema != "repartir" {
		obj, objTxt = obj2, obj2Txt
	}
	// "Ana le da 5": "le" is the previous person
	if receptor == "" && le && explicito && s.sujeto != "" && s.sujeto != suj {
		receptor = s.sujeto
	}
	if obj == "" {
		obj = s.objeto
	}
	if obj == "" {
		obj = "unidad"
	}
	if objTxt != "" {
		if _, ok := s.plural[obj]; !ok || strings.HasSuffix(objTxt, "s") {
			s.plural[obj] = objTxt
		}
	}
	s.objeto = obj
	if lema == "haber" || suj == "" {
		suj = "(todos)"
	}
	if !relativo {
		s.sujeto = suj
	}
	if !strings.HasPrefix(suj, "(") {
		s.ultimo = suj
	}
	if receptor != "" {
		s.ultimo = receptor
	}
	actual := s.cant(suj, obj)
	var cantE Expr
	switch {
	case mitad:
		cantE = Op{Op: '/', A: actual.expr(), B: ent(2)}
	case num != nil && factor != nil && (cada || grupos) && lema != "repartir":
		cantE = Op{Op: '*', A: Num{V: num}, B: Num{V: factor}}
	case num != nil:
		cantE = Num{V: num}
	default:
		return
	}
	frase := "«" + textoToks(c) + "» → "
	nombre := suj
	if strings.HasPrefix(suj, "(") {
		nombre = "hay"
	}
	unidad := s.plural[obj]
	switch efecto {
	case "fijar":
		if imperfectos[verbo] && actual.ini == nil {
			actual.ini = cantE // "…, que tenía 3": it had that before the changes
		} else {
			actual.ini, actual.cambios = cantE, nil
		}
		s.mapeo = append(s.mapeo, frase+nombre+": "+actual.expr().String()+" "+unidad)
	case "sumar":
		if !s.tiene(suj, obj) && s.tiene("(todos)", obj) {
			actual = s.cant("(todos)", obj)
		}
		actual.cambios = append(actual.cambios, cambio{'+', cantE})
		s.mapeo = append(s.mapeo, frase+nombre+": "+actual.expr().String()+" "+unidad)
	case "repartir":
		if factor != nil {
			s.reparto = Op{Op: '/', A: cantE, B: Num{V: factor}}
			s.mapeo = append(s.mapeo, frase+"a cada uno: "+s.reparto.String()+" "+unidad)
		}
	case "restar", "transferir":
		origen := actual
		if actual.ini == nil && len(actual.cambios) == 0 && s.tiene("(todos)", obj) && suj != "(todos)" {
			origen = s.cant("(todos)", obj)
		}
		// a giver we know nothing about is not tracked
		if !(efecto == "transferir" && origen.ini == nil && len(origen.cambios) == 0 && receptor != "") {
			origen.cambios = append(origen.cambios, cambio{'-', cantE})
			s.mapeo = append(s.mapeo, frase+nombre+": "+origen.expr().String()+" "+unidad)
		}
		if prev, ok := s.porVerbo[lema]; ok {
			s.porVerbo[lema] = Op{Op: '+', A: prev, B: cantE}
		} else {
			s.porVerbo[lema] = cantE
		}
		if efecto == "transferir" && receptor != "" {
			r := s.cant(receptor, obj)
			r.cambios = append(r.cambios, cambio{'+', cantE})
			s.mapeo = append(s.mapeo, frase+receptor+": "+r.expr().String()+" "+unidad)
		}
	case "nada":
		s.mapeo = append(s.mapeo, frase+"no cambia nada")
	}
}

func contiene(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// responder reads the question: "¿cuántos le quedan?", "¿cuántos tiene Ana?", "¿cuántos tienen entre
// los dos?", "¿cuántos más tiene Ana que Luis?", "¿cuántos le tocan a cada uno?", "¿cuántos regaló?".
func (s *situ) responder(q []tokP) (Expr, string, string, bool) {
	obj := s.objeto
	var nombres []string
	pal := map[string]bool{}
	for i, t := range q {
		pal[t.w] = true
		if esNombre(t) {
			nombres = append(nombres, t.orig)
		}
		if (t.w == "cuantos" || t.w == "cuantas") && i+1 < len(q) {
			if o := singular(q[i+1].w); s.plural[o] != "" {
				obj = o
			}
		}
		if i > 0 && (q[i-1].w == "su" || q[i-1].w == "a") && parientes[singular(t.w)] {
			nombres = append(nombres, singular(t.w))
		}
	}
	nomObj := s.plural[obj]
	if nomObj == "" {
		nomObj = obj
	}
	suj := s.sujeto
	if len(nombres) > 0 {
		suj = nombres[0]
	}
	valorDe := func(p string) (Expr, bool) {
		if c, ok := s.estado[p][obj]; ok {
			return c.expr(), true
		}
		return nil, false
	}
	switch {
	case pal["cada"] && s.reparto != nil:
		return s.reparto, nomObj + " para cada uno", "A cada uno le tocan %s " + nomObj, true
	case pal["total"] || pal["juntos"] || pal["entre"] && (pal["dos"] || pal["todos"] || pal["ambos"] || pal["tres"]):
		var suma Expr
		for _, p := range s.orden {
			if c, ok := s.estado[p][obj]; ok {
				if suma == nil {
					suma = c.expr()
				} else {
					suma = Op{Op: '+', A: suma, B: c.expr()}
				}
			}
		}
		if suma == nil {
			return nil, "", "", false
		}
		return suma, nomObj + " en total", "En total hay %s " + nomObj, true
	case pal["mas"] && pal["que"] && len(nombres) >= 2:
		a, ok1 := valorDe(nombres[0])
		b, ok2 := valorDe(nombres[1])
		if !ok1 || !ok2 {
			return nil, "", "", false
		}
		return Op{Op: '-', A: a, B: b}, nomObj + " de diferencia", nombres[0] + " tiene %s " + nomObj + " más que " + nombres[1], true
	}
	for _, t := range q {
		if l, ok := lemaVerbo(t.w); ok && l != "tener" && l != "quedar" && l != "haber" {
			if e, ok := s.porVerbo[l]; ok {
				return e, nomObj + " (" + l + ")", "En total: %s " + nomObj, true
			}
		}
	}
	e, ok := valorDe(suj)
	if !ok {
		if e2, ok2 := valorDe("(todos)"); ok2 {
			e, ok = e2, true
			suj = "(todos)"
		}
	}
	if !ok {
		return nil, "", "", false
	}
	quien := suj
	if strings.HasPrefix(suj, "(") {
		quien = ""
	}
	switch {
	case pal["quedan"] || pal["queda"]:
		if quien == "" {
			return e, nomObj + " que quedan", "Quedan %s " + nomObj, true
		}
		return e, nomObj + " que le quedan a " + quien, "A " + quien + " le quedan %s " + nomObj, true
	case quien == "":
		return e, nomObj, "Hay %s " + nomObj, true
	}
	return e, nomObj + " de " + quien, quien + " tiene %s " + nomObj, true
}

// nombresOrdenados lists the unknowns of a planteamiento in order (x, y, z…).
func nombresOrdenados(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
