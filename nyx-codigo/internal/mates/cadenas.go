package mates

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// ---- mates.cadena: chains of equalities ----
//
// The idea comes from the Gnostos engine of Nyx (ResolverCadenaGnostos): "x=g=t=u=e=s=q=l; x=1
// ¿cuánto vale l?". Here every name is a node, every number is one more node (so two different
// numbers in one group are a contradiction), the names are joined with union-find and the
// justification is the chain itself: a shortest path of stated equalities. Every result is
// recomputed by a second method (a breadth-first search over the graph of equalities) and compared.

const (
	maxTextoCadena = 4000 // longer texts are not read as chains
	maxNodosCadena = 1000 // names and numbers
	maxPasosCadena = 40   // steps shown in a justification
)

// tokC is a token of a chain text.
type tokC struct {
	tipo         byte   // 'w' word, 'n' number, '=' equal sign, 'o' operator, 's' separator
	w            string // normalized word, or the symbol
	orig         string
	num          *big.Rat
	desde, hasta int  // byte offsets in the text
	pegado       bool // written right after the previous token, without a space
	pregunta     bool // part of a question
}

// tokensCadena splits a text into words, exact numbers ("3", "-2", "2,5", "3/4"), "=" signs, other
// operators and separators. It is linear in the length of the text.
func tokensCadena(s string) []tokC {
	var out []tokC
	espacio := true
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(r) && r != '\n' {
			espacio = true
			i += n
			continue
		}
		t := tokC{desde: i, pegado: !espacio}
		espacio = false
		switch {
		case unicode.IsLetter(r):
			j := i
			for j < len(s) {
				r2, n2 := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(r2) && !unicode.IsDigit(r2) && r2 != '_' {
					break
				}
				j += n2
			}
			t.tipo, t.orig, t.w = 'w', s[i:j], nucleo.Normalizar(s[i:j])
			i = j
		case r >= '0' && r <= '9' || r == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' && antesDeNegativo(out):
			j := i
			if s[j] == '-' {
				j++
			}
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j+1 < len(s) && (s[j] == '.' || s[j] == ',') && s[j+1] >= '0' && s[j+1] <= '9' {
				j++
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
			}
			texto := strings.Replace(s[i:j], ",", ".", 1)
			if j+1 < len(s) && s[j] == '/' && s[j+1] >= '1' && s[j+1] <= '9' {
				k := j + 1
				for k < len(s) && s[k] >= '0' && s[k] <= '9' {
					k++
				}
				texto += s[j:k]
				j = k
			}
			v, ok := new(big.Rat).SetString(texto)
			if !ok || len(texto) > 60 {
				t.tipo, t.w = 'o', s[i:j] // a number that cannot be read is not a pure chain
			} else {
				t.tipo, t.num, t.w = 'n', v, s[i:j]
			}
			t.orig = s[i:j]
			i = j
		case r == '=':
			j := i
			for j < len(s) && s[j] == '=' {
				j++
			}
			t.tipo, t.w, t.orig = '=', "=", s[i:j]
			i = j
		case strings.ContainsRune("<>!:", r) && i+1 < len(s) && s[i+1] == '=':
			t.tipo, t.w, t.orig = 'o', s[i:i+2], s[i:i+2]
			i += 2
		case strings.ContainsRune("+-*/^%<>√·×÷−", r):
			t.tipo, t.w, t.orig = 'o', string(r), string(r)
			i += n
		default:
			t.tipo, t.w, t.orig = 's', string(r), string(r)
			i += n
		}
		t.hasta = i
		out = append(out, t)
	}
	marcarPreguntas(out)
	return out
}

// antesDeNegativo: a «-» right before a digit is a sign after "=", a separator or "vale", "a", "que".
func antesDeNegativo(ts []tokC) bool {
	if len(ts) == 0 {
		return true
	}
	u := ts[len(ts)-1]
	return u.tipo == '=' || u.tipo == 's' || u.tipo == 'w' && (u.w == "vale" || u.w == "valen" || u.w == "a" || u.w == "que")
}

// palabrasPregunta start a question written without «¿»; the weak ones only at the start of it.
var (
	palabrasPregunta = map[string]bool{"cuanto": true, "cuanta": true, "cuantos": true, "cuantas": true, "que": true,
		"cual": true, "cuales": true, "dime": true, "sabes": true}
	palabrasPreguntaDebiles = map[string]bool{"es": true, "son": true, "vale": true, "valen": true}
)

// marcarPreguntas marks the tokens of the questions: from «¿» to «?», or, without «¿», from the
// first question word of the sentence that ends in «?».
func marcarPreguntas(ts []tokC) {
	dentro := false
	inicioFrase := 0
	for i := range ts {
		t := &ts[i]
		if t.tipo == 's' && t.w == "¿" {
			dentro = true
		}
		if dentro {
			t.pregunta = true
		}
		if t.tipo == 's' && t.w == "?" {
			if !dentro {
				desde := -1
				for k := inicioFrase; k < i; k++ {
					if ts[k].tipo == 'w' && palabrasPregunta[ts[k].w] {
						desde = k
						break
					}
				}
				if desde < 0 {
					// no strong question word: the last clause, after the last comma
					desde = inicioFrase
					for k := i - 1; k > inicioFrase; k-- {
						if ts[k].tipo == 's' && ts[k].w == "," {
							desde = k + 1
							break
						}
					}
					if desde < i && ts[desde].tipo == 'w' && !palabrasPreguntaDebiles[ts[desde].w] && !operando(ts, desde) {
						desde = inicioFrase
					}
				}
				for k := desde; k <= i; k++ {
					ts[k].pregunta = true
				}
			}
			dentro = false
		}
		if t.tipo == 's' && strings.Contains(".;\n?:", t.w) {
			inicioFrase = i + 1
		}
	}
}

// noNombres are words that are never names of a chain.
var noNombres = conjuntoPal(`si y e o u ni que el la los las lo un una unos unas es son era sea sean igual iguales
a al de del en con por para vale valen cuanto cuanta cuantos cuantas cual cuales entonces tambien ademas pero
porque sabemos sabiendo se sabe dado dados dadas tenemos tengo hay mismo misma equivale equivalente dime sabes
valor como ya no y luego despues todos todas cada uno otro otra`)

func esNombreCadena(t tokC) bool {
	return t.tipo == 'w' && !noNombres[t.w] && len(t.w) <= 30
}

// datosCadena is what a chain text says.
type datosCadena struct {
	nombres  []string   // display text of each node
	claves   []string   // node keys: the normalized name, or "#" + the number
	valores  []*big.Rat // the value of a number node (nil for names)
	indice   map[string]int
	enlaces  []enlaceC
	fuentes  []string // the text of each stated chain
	cadenas  [][]int  // the nodes of each stated chain, in order
	impuro   bool     // an operator touches an equality: it is algebra, not a chain
	demasiad bool
	// recognition features
	enlacesNombres int  // links between two different names
	larga          bool // a chain with three or more members
	conPalabras    bool // "es igual a", "son iguales"…
	pregunta       preguntaCadena
}

type enlaceC struct{ a, b, fuente int }

type preguntaCadena struct {
	tipo    string // "valor" | "igualdad" | "todo" (no question) | "" (a question I do not understand)
	nodos   []int  // the names asked about
	extra   []string
	texto   string
	todos   bool
	hayPreg bool
}

func (d *datosCadena) nodo(t tokC) (int, bool) {
	clave, mostrar := t.w, t.orig
	if t.tipo == 'n' {
		clave, mostrar = "#"+t.num.RatString(), textoRat(t.num)
	}
	if k, ok := d.indice[clave]; ok {
		return k, true
	}
	if len(d.claves) >= maxNodosCadena {
		d.demasiad = true
		return 0, false
	}
	d.indice[clave] = len(d.claves)
	d.claves = append(d.claves, clave)
	d.nombres = append(d.nombres, mostrar)
	if t.tipo == 'n' {
		d.valores = append(d.valores, new(big.Rat).Set(t.num))
	} else {
		d.valores = append(d.valores, nil)
	}
	return len(d.claves) - 1, true
}

func (d *datosCadena) esNumero(k int) bool { return d.valores[k] != nil }

// frasesIgual are the word forms of "=" (longest first).
var frasesIgual = [][]string{
	{"es", "lo", "mismo", "que"}, {"vale", "lo", "mismo", "que"}, {"valen", "lo", "mismo", "que"},
	{"es", "igual", "a"}, {"es", "igual", "que"}, {"es", "equivalente", "a"}, {"son", "iguales", "a"},
	{"igual", "a"}, {"igual", "que"}, {"equivale", "a"},
}

// igualEn returns how many tokens the "=" at ts[i] takes (0 if there is none) and whether it was
// written with words.
func igualEn(ts []tokC, i int) (int, bool) {
	if i >= len(ts) {
		return 0, false
	}
	if ts[i].tipo == '=' {
		return 1, false
	}
	for _, f := range frasesIgual {
		if i+len(f) > len(ts) {
			continue
		}
		ok := true
		for k, w := range f {
			if ts[i+k].tipo != 'w' || ts[i+k].w != w || ts[i+k].pregunta != ts[i].pregunta {
				ok = false
				break
			}
		}
		if ok {
			return len(f), true
		}
	}
	// "x vale 3"
	if ts[i].tipo == 'w' && (ts[i].w == "vale" || ts[i].w == "valen") && i+1 < len(ts) && ts[i+1].tipo == 'n' {
		return 1, true
	}
	return 0, false
}

func operando(ts []tokC, i int) bool {
	return i < len(ts) && (ts[i].tipo == 'n' || esNombreCadena(ts[i]))
}

// leerCadenas reads the stated equalities and the question. It never fails: what it cannot read
// leaves the result empty or marked impure.
func leerCadenas(texto string) *datosCadena {
	d := &datosCadena{indice: map[string]int{}}
	if len(texto) > maxTextoCadena {
		d.demasiad = true
		return d
	}
	ts := tokensCadena(texto)
	usado := make([]bool, len(ts))
	for i := 0; i < len(ts); i++ {
		if ts[i].pregunta || !operando(ts, i) {
			continue
		}
		k, palabras := igualEn(ts, i+1)
		if k == 0 || !operando(ts, i+1+k) || ts[i+1+k].pregunta {
			continue
		}
		miembros := []int{i}
		j := i
		for {
			k, pal := igualEn(ts, j+1)
			if k == 0 || !operando(ts, j+1+k) || ts[j+1+k].pregunta {
				break
			}
			palabras = palabras || pal
			for m := j + 1; m <= j+k; m++ {
				usado[m] = true
			}
			j = j + 1 + k
			miembros = append(miembros, j)
		}
		d.conPalabras = d.conPalabras || palabras
		d.agregarCadena(texto, ts, miembros)
		for _, m := range miembros {
			usado[m] = true
		}
		i = j
	}
	// "x, g y t son iguales"
	for i := 0; i+1 < len(ts); i++ {
		if ts[i].pregunta || ts[i].tipo != 'w' || ts[i].w != "son" || ts[i+1].w != "iguales" || (i+2 < len(ts) && ts[i+2].w == "a") {
			continue
		}
		var miembros []int
		for k := i - 1; k >= 0 && !ts[k].pregunta; k-- {
			switch {
			case operando(ts, k):
				miembros = append([]int{k}, miembros...)
				continue
			case ts[k].tipo == 'w' && (ts[k].w == "y" || ts[k].w == "e"), ts[k].tipo == 's' && ts[k].w == ",":
				continue
			}
			break
		}
		if len(miembros) >= 2 {
			d.conPalabras = true
			d.agregarCadena(texto, ts, miembros)
			for _, m := range miembros {
				usado[m] = true
			}
			usado[i], usado[i+1] = true, true
		}
	}
	// an operator next to an operand or an "=" that is not part of a chain means algebra
	for i, t := range ts {
		if t.pregunta {
			continue
		}
		switch {
		case t.tipo == '=' && !usado[i]:
			d.impuro = true
		case t.tipo == 'o':
			if i > 0 && (operando(ts, i-1) || ts[i-1].tipo == '=') || i+1 < len(ts) && (operando(ts, i+1) || ts[i+1].tipo == '=') {
				d.impuro = true
			}
		case t.tipo == 'w' && t.pegado && i > 0 && ts[i-1].tipo == 'n':
			d.impuro = true // "2x"
		}
	}
	d.leerPregunta(ts)
	return d
}

func (d *datosCadena) agregarCadena(texto string, ts []tokC, miembros []int) {
	var nodos []int
	for _, m := range miembros {
		k, ok := d.nodo(ts[m])
		if !ok {
			return
		}
		nodos = append(nodos, k)
	}
	fuente := len(d.fuentes)
	d.fuentes = append(d.fuentes, strings.TrimSpace(texto[ts[miembros[0]].desde:ts[miembros[len(miembros)-1]].hasta]))
	d.cadenas = append(d.cadenas, nodos)
	if len(nodos) >= 3 {
		d.larga = true
	}
	for k := 1; k < len(nodos); k++ {
		a, b := nodos[k-1], nodos[k]
		d.enlaces = append(d.enlaces, enlaceC{a: a, b: b, fuente: fuente})
		if a != b && !d.esNumero(a) && !d.esNumero(b) {
			d.enlacesNombres++
		}
	}
}

// leerPregunta finds what is asked: the value of some names, or whether two names are equal.
func (d *datosCadena) leerPregunta(ts []tokC) {
	var palabras []string
	var nombres []int
	var partes []string
	igual := false
	for _, t := range ts {
		if !t.pregunta {
			continue
		}
		d.pregunta.hayPreg = true
		partes = append(partes, t.orig)
		switch {
		case t.tipo == '=':
			igual = true
		case t.tipo == 'w':
			palabras = append(palabras, t.w)
			if k, ok := d.indice[t.w]; ok && !d.esNumero(k) {
				repetido := false
				for _, x := range nombres {
					repetido = repetido || x == k
				}
				if !repetido {
					nombres = append(nombres, k)
				}
			} else if esNombreCadena(t) && utf8.RuneCountInString(t.w) <= 2 {
				d.pregunta.extra = append(d.pregunta.extra, t.orig)
			}
		}
	}
	d.pregunta.texto = strings.TrimSpace(strings.Join(partes, " "))
	sec := " " + strings.Join(palabras, " ") + " "
	d.pregunta.nodos = nombres
	d.pregunta.todos = strings.Contains(sec, " todos ") || strings.Contains(sec, " todas ") || strings.Contains(sec, " cada ")
	switch {
	case !d.pregunta.hayPreg:
		d.pregunta.tipo = "todo"
	case igual || strings.Contains(sec, " igual ") || strings.Contains(sec, " iguales ") || strings.Contains(sec, " mismo ") ||
		strings.Contains(sec, " equivale "):
		if len(nombres)+len(d.pregunta.extra) >= 2 {
			d.pregunta.tipo = "igualdad"
		}
	case strings.Contains(sec, " vale ") || strings.Contains(sec, " valen ") || strings.Contains(sec, " valor ") ||
		strings.Contains(sec, " cuanto ") || strings.Contains(sec, " cuanta ") || strings.Contains(sec, " que es "):
		if len(nombres)+len(d.pregunta.extra) > 0 || d.pregunta.todos {
			d.pregunta.tipo = "valor"
		}
	}
}

// ---- the two methods ----

// grupos joins the nodes with union-find (union by size, path halving).
func gruposUnion(n int, enlaces []enlaceC) []int {
	padre := make([]int, n)
	tam := make([]int, n)
	for i := range padre {
		padre[i], tam[i] = i, 1
	}
	raiz := func(x int) int {
		for padre[x] != x {
			padre[x] = padre[padre[x]]
			x = padre[x]
		}
		return x
	}
	for _, e := range enlaces {
		a, b := raiz(e.a), raiz(e.b)
		if a == b {
			continue
		}
		if tam[a] < tam[b] {
			a, b = b, a
		}
		padre[b] = a
		tam[a] += tam[b]
	}
	out := make([]int, n)
	for i := range out {
		out[i] = raiz(i)
	}
	return out
}

type aristaC struct{ a, fuente int }

func adyacencia(n int, enlaces []enlaceC) [][]aristaC {
	ady := make([][]aristaC, n)
	for _, e := range enlaces {
		ady[e.a] = append(ady[e.a], aristaC{e.b, e.fuente})
		ady[e.b] = append(ady[e.b], aristaC{e.a, e.fuente})
	}
	return ady
}

// componentes labels the nodes by breadth-first search (the second method).
func componentes(ady [][]aristaC) []int {
	comp := make([]int, len(ady))
	for i := range comp {
		comp[i] = -1
	}
	cola := make([]int, 0, len(ady))
	for s := range ady {
		if comp[s] >= 0 {
			continue
		}
		comp[s] = s
		cola = append(cola[:0], s)
		for len(cola) > 0 {
			x := cola[0]
			cola = cola[1:]
			for _, e := range ady[x] {
				if comp[e.a] < 0 {
					comp[e.a] = s
					cola = append(cola, e.a)
				}
			}
		}
	}
	return comp
}

// camino is a shortest path of stated equalities from s to t: the nodes and the source of each link.
func camino(ady [][]aristaC, s, t int) ([]int, []int) {
	prev := make([]int, len(ady))
	fuente := make([]int, len(ady))
	for i := range prev {
		prev[i] = -2
	}
	prev[s] = -1
	cola := []int{s}
	for len(cola) > 0 && prev[t] == -2 {
		x := cola[0]
		cola = cola[1:]
		for _, e := range ady[x] {
			if prev[e.a] == -2 {
				prev[e.a], fuente[e.a] = x, e.fuente
				cola = append(cola, e.a)
			}
		}
	}
	if prev[t] == -2 {
		return nil, nil
	}
	var nodos, fuentes []int
	for x := t; x != -1; x = prev[x] {
		nodos = append(nodos, x)
		fuentes = append(fuentes, fuente[x])
	}
	for i, j := 0, len(nodos)-1; i < j; i, j = i+1, j-1 {
		nodos[i], nodos[j] = nodos[j], nodos[i]
		fuentes[i], fuentes[j] = fuentes[j], fuentes[i]
	}
	return nodos, fuentes[1:] // fuentes[k] is the source of the link nodos[k] — nodos[k+1]
}

// caminoValido checks a path link by link against the stated equalities.
func caminoValido(d *datosCadena, nodos, fuentes []int) bool {
	if len(nodos) == 0 || len(fuentes) != len(nodos)-1 {
		return false
	}
	for k := 1; k < len(nodos); k++ {
		ok := false
		for _, e := range d.enlaces {
			if e.fuente == fuentes[k-1] && (e.a == nodos[k-1] && e.b == nodos[k] || e.b == nodos[k-1] && e.a == nodos[k]) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// ---- the resolver ----

type resCadena struct{}

func (resCadena) Nombre() string { return "mates.cadena" }

func (resCadena) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	if p == nil || strings.TrimSpace(p.Codigo) != "" {
		return nucleo.Reconocimiento{}
	}
	t := textoDe(p)
	if len(t) > maxTextoCadena {
		return nucleo.Reconocimiento{}
	}
	n := " " + nucleo.Normalizar(t) + " "
	if !strings.Contains(t, "=") && !strings.Contains(n, " igual") && !strings.Contains(n, " lo mismo que ") && !strings.Contains(n, " equivale") {
		return nucleo.Reconocimiento{}
	}
	d := leerCadenas(t)
	if d.impuro || d.demasiad || d.enlacesNombres == 0 {
		return nucleo.Reconocimiento{}
	}
	rec := nucleo.Reconocimiento{Puntos: 0.9, Intencion: nucleo.IEcuacion, Lectura: "una cadena de igualdades"}
	if d.larga || d.conPalabras || d.pregunta.tipo == "valor" || d.pregunta.tipo == "igualdad" {
		rec.Puntos = 0.95
	}
	if d.pregunta.tipo == "igualdad" {
		rec.Intencion = nucleo.ILogica
	}
	// a plain system that mates.ecuacion already reads is its question
	if (resEcuacion{}).Reconoce(p).Puntos > 0 {
		rec.Puntos = 0.9
	}
	return rec
}

func (resCadena) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	t := textoDe(p)
	d := leerCadenas(t)
	switch {
	case d.demasiad:
		return nucleo.Respuesta{}, fmt.Errorf("%w: hay demasiados nombres (como mucho %d)", nucleo.ErrNoSoportado, maxNodosCadena)
	case d.impuro:
		return nucleo.Respuesta{}, fmt.Errorf("%w: hay operaciones además de igualdades; escríbelo como ecuación", nucleo.ErrNoEntiendo)
	case d.enlacesNombres == 0:
		return nucleo.Respuesta{}, fmt.Errorf("%w: no veo ninguna cadena de igualdades (como «x = g = t; x = 1»)", nucleo.ErrNoEntiendo)
	}
	leer := n.Sub(nucleo.PasoEntender, "Leo %d %s entre %d nombres y números", len(d.enlaces), plural(len(d.enlaces), "igualdad", "igualdades"), len(d.claves))
	for _, f := range d.fuentes {
		leer.Detalle("%s", f)
	}
	if d.pregunta.texto != "" {
		leer.Detalle("Pregunta: %s", d.pregunta.texto)
	}
	leer.Info("")
	if ctx.Err() != nil {
		return nucleo.Respuesta{}, nucleo.ErrSinTiempo
	}

	// method 1: union-find
	uf := n.Sub(nucleo.PasoIntento, "Junto los nombres iguales en grupos (unión-búsqueda)")
	raiz := gruposUnion(len(d.claves), d.enlaces)
	miembros := map[int][]int{}
	var orden []int
	for k, r := range raiz {
		if _, ok := miembros[r]; !ok {
			orden = append(orden, r)
		}
		miembros[r] = append(miembros[r], k)
	}
	numeros := map[int][]int{} // group → its number nodes
	for _, r := range orden {
		for _, k := range miembros[r] {
			if d.esNumero(k) {
				numeros[r] = append(numeros[r], k)
			}
		}
	}
	for i, r := range orden {
		if i == 20 {
			uf.Detalle("… y %d grupos más", len(orden)-20)
			break
		}
		uf.Detalle("%s", d.textoGrupo(miembros[r]))
	}
	uf.Bien("%d %s", len(orden), plural(len(orden), "grupo", "grupos"))

	// method 2: breadth-first search over the graph of equalities
	comp := n.Sub(nucleo.PasoComprobar, "Lo repito por otro camino: recorro el grafo de igualdades (búsqueda en anchura)")
	ady := adyacencia(len(d.claves), d.enlaces)
	bfs := componentes(ady)
	ida, vuelta := map[int]int{}, map[int]int{}
	coinciden := true
	for k := range d.claves {
		if a, ok := ida[raiz[k]]; ok && a != bfs[k] {
			coinciden = false
		}
		if b, ok := vuelta[bfs[k]]; ok && b != raiz[k] {
			coinciden = false
		}
		ida[raiz[k]], vuelta[bfs[k]] = bfs[k], raiz[k]
	}
	if !coinciden {
		comp.Mal("Los dos métodos no dan los mismos grupos")
		return nucleo.Respuesta{}, errSinComprobar
	}
	comp.Bien("Salen los mismos %d %s", len(orden), plural(len(orden), "grupo", "grupos"))

	s := &salidaCadena{d: d, ady: ady, raiz: raiz, miembros: miembros, numeros: numeros}
	var r nucleo.Respuesta
	var err error
	switch d.pregunta.tipo {
	case "igualdad":
		r, err = s.igualdad(n)
	case "valor":
		r, err = s.valores(n)
	default:
		r, err = s.todo(orden, n)
	}
	if err != nil {
		return nucleo.Respuesta{}, err
	}
	// contradictions in groups that were not asked about are reported too
	for _, g := range orden {
		if len(numeros[g]) >= 2 && !s.contado[g] {
			if txt, ok := s.contradiccion(g); ok {
				r.Parrafos = append(r.Parrafos, "Ojo, además: "+txt)
			}
		}
	}
	r.Entendi = "Así lo entendí: " + recortar(strings.Join(d.fuentes, "; "))
	if d.pregunta.texto != "" {
		r.Entendi += "; pregunta: " + d.pregunta.texto
	}
	r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(s.comprobados, s.comprobados, "Comprobado por otro camino",
		"agrupé los nombres con unión-búsqueda y lo repetí recorriendo el grafo de igualdades: salen los mismos grupos, y cada cadena que enseño está hecha de igualdades que me diste")}
	r.Nivel, r.Exito, r.Metodo = nucleo.NivelCalculo, true, "union-busqueda"
	r.Sugerencias = []string{"¿Por qué?"}
	r.Justificacion = r.Plan
	return r, nil
}

// salidaCadena writes the answers once both methods agree.
type salidaCadena struct {
	d           *datosCadena
	ady         [][]aristaC
	raiz        []int
	miembros    map[int][]int
	numeros     map[int][]int
	contado     map[int]bool
	comprobados int
}

func (d *datosCadena) textoGrupo(nodos []int) string {
	var ps []string
	for i, k := range nodos {
		if i == 12 {
			ps = append(ps, fmt.Sprintf("… (%d más)", len(nodos)-12))
			break
		}
		ps = append(ps, d.nombres[k])
	}
	return strings.Join(ps, " = ")
}

// textoCamino writes a path as "l = q = s = … = 1", shortened in the middle when it is long.
func (d *datosCadena) textoCamino(nodos []int) string {
	var ps []string
	for i, k := range nodos {
		if len(nodos) > maxPasosCadena && i == 20 {
			ps = append(ps, "…")
		}
		if len(nodos) > maxPasosCadena && i >= 20 && i < len(nodos)-10 {
			continue
		}
		ps = append(ps, d.nombres[k])
	}
	return strings.Join(ps, " = ")
}

func (s *salidaCadena) marcar(g int) {
	if s.contado == nil {
		s.contado = map[int]bool{}
	}
	s.contado[g] = true
}

// pasos writes the steps along a path: from a number to a name the value travels; between two names
// the equality does.
func (s *salidaCadena) pasos(nodos, fuentes []int) []string {
	d := s.d
	var out []string
	desdeNumero := d.esNumero(nodos[0])
	primero := d.nombres[nodos[0]]
	for k := 1; k < len(nodos); k++ {
		if len(nodos) > maxPasosCadena && k > 20 && k < len(nodos)-10 {
			if k == 21 {
				out = append(out, fmt.Sprintf("… (%d pasos más) …", len(nodos)-31))
			}
			continue
		}
		a, b := d.nombres[nodos[k-1]], d.nombres[nodos[k]]
		f := "«" + recortar(d.fuentes[fuentes[k-1]]) + "»"
		switch {
		case desdeNumero && k == 1:
			out = append(out, fmt.Sprintf("%s = %s (lo dice %s)", b, a, f))
		case desdeNumero:
			out = append(out, fmt.Sprintf("%s = %s (lo dice %s), así que %s = %s", b, a, f, b, primero))
		case k == 1:
			out = append(out, fmt.Sprintf("%s = %s (lo dice %s)", a, b, f))
		default:
			out = append(out, fmt.Sprintf("%s = %s (lo dice %s), así que %s = %s", a, b, f, primero, b))
		}
	}
	return out
}

// contradiccion explains why a group with two different numbers is impossible.
func (s *salidaCadena) contradiccion(g int) (string, bool) {
	d := s.d
	ns := s.numeros[g]
	if len(ns) < 2 {
		return "", false
	}
	nodos, fuentes := camino(s.ady, ns[0], ns[1])
	if !caminoValido(d, nodos, fuentes) || s.raiz[ns[0]] != s.raiz[ns[1]] || d.valores[ns[0]].Cmp(d.valores[ns[1]]) == 0 {
		return "", false
	}
	s.marcar(g)
	s.comprobados++
	return fmt.Sprintf("los datos se contradicen: %s, así que %s tendría que ser igual a %s, y no lo es.",
		d.textoCamino(nodos), d.nombres[ns[0]], d.nombres[ns[1]]), true
}

func (s *salidaCadena) valores(n *nucleo.Nodo) (nucleo.Respuesta, error) {
	d := s.d
	var r nucleo.Respuesta
	objetivos := d.pregunta.nodos
	if d.pregunta.todos && len(objetivos) == 0 {
		for k := range d.claves {
			if !d.esNumero(k) {
				objetivos = append(objetivos, k)
			}
		}
	}
	var textos []string
	for _, x := range d.pregunta.extra {
		textos = append(textos, fmt.Sprintf("No sé nada de %s: no aparece en ninguna igualdad", x))
	}
	for i, k := range objetivos {
		if i == 50 {
			textos = append(textos, fmt.Sprintf("… y %d nombres más", len(objetivos)-50))
			break
		}
		g := s.raiz[k]
		ns := s.numeros[g]
		nombre := d.nombres[k]
		switch len(ns) {
		case 0:
			s.comprobados++
			grupo := d.textoGrupo(s.miembros[g])
			if len(s.miembros[g]) == 1 {
				textos = append(textos, fmt.Sprintf("No se puede saber cuánto vale %s: ningún dato le da un número", nombre))
				continue
			}
			textos = append(textos, fmt.Sprintf("No se puede saber cuánto vale %s: %s, pero ningún dato le da un número a ese grupo", nombre, grupo))
			n.Sub(nucleo.PasoResultado, "%s no tiene ningún número en su grupo", nombre).Info("")
		case 1:
			nodos, fuentes := camino(s.ady, ns[0], k)
			if !caminoValido(d, nodos, fuentes) || d.valores[ns[0]] == nil {
				n.Sub(nucleo.PasoError, "La cadena de %s no se sostiene", nombre).Mal("")
				return nucleo.Respuesta{}, errSinComprobar
			}
			s.comprobados++
			v := d.valores[ns[0]]
			valor := textoRat(v)
			if !v.IsInt() {
				f, _ := v.Float64()
				valor += " (≈ " + textoFloat(f) + ")"
			}
			textos = append(textos, nombre+" = "+valor)
			invertido := append([]int(nil), nodos...)
			for a, b := 0, len(invertido)-1; a < b; a, b = a+1, b-1 {
				invertido[a], invertido[b] = invertido[b], invertido[a]
			}
			r.Parrafos = append(r.Parrafos, "La cadena: "+d.textoCamino(invertido))
			if len(objetivos) > 1 {
				r.Plan = append(r.Plan, "Para "+nombre+":")
			}
			r.Plan = append(r.Plan, s.pasos(nodos, fuentes)...)
			paso := n.Sub(nucleo.PasoIntento, "Llevo el valor por la cadena hasta %s", nombre)
			paso.Detalle("%s", d.textoCamino(invertido))
			paso.Bien("%s = %s", nombre, textoRat(v))
		default:
			txt, ok := s.contradiccion(g)
			if !ok {
				return nucleo.Respuesta{}, errSinComprobar
			}
			n.Sub(nucleo.PasoError, "Contradicción en el grupo de %s", nombre).Mal("")
			textos = append(textos, fmt.Sprintf("%s no tiene un valor único: %s", nombre, txt))
		}
		s.marcar(g)
	}
	if len(textos) == 0 {
		return nucleo.Respuesta{}, fmt.Errorf("%w: no sé por qué nombre me preguntas", nucleo.ErrNoEntiendo)
	}
	r.Texto = strings.Join(textos, "; ")
	return r, nil
}

func (s *salidaCadena) igualdad(n *nucleo.Nodo) (nucleo.Respuesta, error) {
	d := s.d
	var r nucleo.Respuesta
	if len(d.pregunta.nodos) < 2 {
		falta := strings.Join(d.pregunta.extra, " ni de ")
		s.comprobados++
		r.Texto = fmt.Sprintf("No se puede saber: no sé nada de %s, no aparece en ninguna igualdad.", falta)
		return r, nil
	}
	a, b := d.pregunta.nodos[0], d.pregunta.nodos[1]
	na, nb := d.nombres[a], d.nombres[b]
	ga, gb := s.raiz[a], s.raiz[b]
	s.marcar(ga)
	s.marcar(gb)
	if ga == gb {
		nodos, fuentes := camino(s.ady, a, b)
		if !caminoValido(d, nodos, fuentes) {
			return nucleo.Respuesta{}, errSinComprobar
		}
		s.comprobados++
		r.Texto = fmt.Sprintf("Sí: %s = %s.", na, nb)
		r.Parrafos = append(r.Parrafos, "La cadena: "+d.textoCamino(nodos))
		r.Plan = s.pasos(nodos, fuentes)
		r.Plan = append(r.Plan, fmt.Sprintf("Así que %s = %s", na, nb))
		n.Sub(nucleo.PasoIntento, "Busco una cadena de igualdades de %s a %s", na, nb).Bien("%s", d.textoCamino(nodos))
		if len(s.numeros[ga]) >= 2 {
			s.contado[ga] = false // reported below as a contradiction
		}
		return r, nil
	}
	va, vb := s.valorUnico(ga), s.valorUnico(gb)
	n.Sub(nucleo.PasoIntento, "Busco una cadena de igualdades de %s a %s", na, nb).Info("No hay ninguna")
	s.comprobados++
	if va != nil && vb != nil && va.Cmp(vb) != 0 {
		r.Texto = fmt.Sprintf("No: %s = %s y %s = %s.", na, textoRat(va), nb, textoRat(vb))
		return r, nil
	}
	r.Texto = fmt.Sprintf("No se puede saber: ninguna igualdad une %s con %s.", na, nb)
	r.Parrafos = append(r.Parrafos, "El grupo de "+na+": "+d.textoGrupo(s.miembros[ga])+".", "El grupo de "+nb+": "+d.textoGrupo(s.miembros[gb])+".")
	return r, nil
}

func (s *salidaCadena) valorUnico(g int) *big.Rat {
	if ns := s.numeros[g]; len(ns) == 1 {
		return s.d.valores[ns[0]]
	}
	return nil
}

// todo: no question, so every group is shown with its value.
func (s *salidaCadena) todo(orden []int, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	d := s.d
	var r nucleo.Respuesta
	var lineas []string
	for _, g := range orden {
		nombres := 0
		for _, k := range s.miembros[g] {
			if !d.esNumero(k) {
				nombres++
			}
		}
		if nombres == 0 {
			continue
		}
		s.marcar(g)
		s.comprobados++
		switch ns := s.numeros[g]; len(ns) {
		case 0:
			lineas = append(lineas, d.textoGrupo(s.miembros[g])+" (sin número)")
		case 1:
			lineas = append(lineas, d.textoGrupo(s.miembros[g]))
		default:
			txt, ok := s.contradiccion(g)
			if !ok {
				return nucleo.Respuesta{}, errSinComprobar
			}
			lineas = append(lineas, mayusculaInicial(txt))
		}
	}
	if d.pregunta.hayPreg {
		r.Parrafos = append(r.Parrafos, "No he entendido la pregunta («"+d.pregunta.texto+"»), así que te digo lo que se deduce de las igualdades.")
	}
	r.Texto = "Grupos de nombres iguales: " + strings.Join(lineas, "; ")
	n.Sub(nucleo.PasoResultado, "%d %s", len(lineas), plural(len(lineas), "grupo", "grupos")).Info("")
	return r, nil
}

func mayusculaInicial(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}
