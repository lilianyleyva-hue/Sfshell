package puzles

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// ---- puzles.tareas: ordering tasks that depend on each other ----
//
// The idea comes from the plans of Nyx (planes.go and ConstruirPlanCausal in Gnostos v6): "ordena
// estas tareas: preparar la masa y calentar el horno; luego hornear el pan; después comer". A plan is
// a graph, not a list: "luego", "después", "antes de", "cuando termines de"… give explicit
// dependencies, and a task that uses a noun another task produces waits for it (a causal dependency,
// deduced). The tasks go in stages: the tasks of one stage do not depend on each other (‖). A cycle
// is reported, never broken at random, and the order is checked against every dependency.
// This resolver does not take the state-search puzzles of puzles.plan (jugs, river crossing, Hanoi…).

const (
	maxTextoTareas   = 3000
	maxTareas        = 100
	maxPalabrasTarea = 14
)

type tokT struct {
	w, orig      string // normalized, as written
	sep          string // separator symbol ("", ",", ";", ".", ":", "¿", "?"…)
	desde, hasta int
	pregunta     bool
}

func tokensTareas(s string) []tokT {
	var out []tokT
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r) && r != '\n':
			i += n
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			j := i
			for j < len(s) {
				r2, n2 := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(r2) && !unicode.IsDigit(r2) && r2 != '\'' && r2 != '-' {
					break
				}
				j += n2
			}
			out = append(out, tokT{w: nucleo.Normalizar(s[i:j]), orig: s[i:j], desde: i, hasta: j})
			i = j
		default:
			out = append(out, tokT{sep: string(r), orig: string(r), desde: i, hasta: i + n})
			i += n
		}
	}
	// questions: from «¿» to «?», or the sentence that ends in «?»
	dentro, inicio := false, 0
	for i := range out {
		if out[i].sep == "¿" {
			dentro = true
		}
		if dentro {
			out[i].pregunta = true
		}
		if out[i].sep == "?" {
			if !dentro {
				for k := inicio; k <= i; k++ {
					out[k].pregunta = true
				}
			}
			dentro = false
		}
		if strings.Contains(".;\n?!:", out[i].sep) && out[i].sep != "" {
			inicio = i + 1
		}
	}
	return out
}

// cabecerasTareas are the words of a request to order tasks.
var cabecerasTareas = []string{" ordena ", " ordename ", " ordenar ", " organiza ", " organizame ", " organizar ",
	" planifica ", " planificame ", " planifico ", " planea ", " en que orden ", " que orden ", " que hago primero ",
	" que va primero ", " que va antes ", " haz un plan ", " hazme un plan ", " dame un plan ", " un plan para ",
	" por donde empiezo ", " orden de las tareas ", " orden de los pasos "}

var palabrasPlanFuertes = []string{" tarea", " pasos ", " plan ", " plan:", " orden "}

// conectoresSeguidos: what follows happens after what came before.
var conectoresSeguidos = [][]string{
	{"y", "despues", "de", "eso"}, {"despues", "de", "eso"}, {"luego", "de", "eso"}, {"tras", "eso"},
	{"y", "luego"}, {"y", "despues"}, {"y", "finalmente"}, {"y", "por", "ultimo"}, {"y", "al", "final"}, {"y", "entonces"},
	{"y", "a", "continuacion"}, {"por", "ultimo"}, {"al", "final"}, {"a", "continuacion"}, {"acto", "seguido"},
	{"mas", "tarde"}, {"una", "vez", "hecho"}, {"una", "vez", "hecha"}, {"una", "vez", "hechos"}, {"una", "vez", "hechas"},
	{"una", "vez", "terminado"}, {"una", "vez", "terminada"}, {"cuando", "termines"}, {"cuando", "termine"},
	{"cuando", "terminen"}, {"cuando", "acabes"}, {"cuando", "acabe"}, {"cuando", "este", "listo"}, {"cuando", "este", "lista"},
	{"luego"}, {"despues"}, {"entonces"}, {"finalmente"}, {"seguidamente"}, {"por", "fin"}, {"segundo"}, {"tercero"},
	{"cuarto"}, {"quinto"}, {"ultimo"},
}

// conectoresInicio only mark the start ("primero …").
var conectoresInicio = [][]string{{"lo", "primero", "es"}, {"lo", "primero"}, {"antes", "que", "nada"}, {"para", "empezar"},
	{"primero"}, {"primera"}}

// relacionales: "A antes de B" (A first), "después de B, A" (B first)…
var relacionales = []struct {
	f        []string
	primeroA bool // the part before the marker goes first
}{
	{[]string{"antes", "de"}, true}, {[]string{"antes", "que"}, true},
	{[]string{"despues", "de"}, false}, {[]string{"luego", "de"}, false}, {[]string{"tras"}, false},
	{[]string{"cuando", "termines", "de"}, false}, {[]string{"cuando", "acabes", "de"}, false},
	{[]string{"cuando", "termine", "de"}, false}, {[]string{"en", "cuanto", "termines", "de"}, false},
	{[]string{"una", "vez", "que", "termines", "de"}, false},
}

// rellenoTarea is dropped at the start of a task ("tengo que preparar…").
var rellenoTarea = [][]string{{"tengo", "que"}, {"tienes", "que"}, {"tenemos", "que"}, {"hay", "que"}, {"hace", "falta"},
	{"es", "necesario"}, {"voy", "a"}, {"vamos", "a"}, {"me", "toca"}, {"toca"}, {"debo"}, {"debes"}, {"debemos"},
	{"quiero"}, {"necesito"}, {"se", "debe"}, {"y"}, {"e"}, {"o"}, {"que"}, {"a"}}

var noVerboTarea = map[string]bool{}

var articuloT = map[string]bool{"el": true, "la": true, "los": true, "las": true, "un": true, "una": true, "unos": true,
	"unas": true, "mi": true, "mis": true, "tu": true, "tus": true, "su": true, "sus": true, "al": true, "del": true}

func init() {
	for _, w := range strings.Fields(`el la los las un una unos unas lo al del de en con por para sin sobre mi mis tu tus
su sus este esta estos estas ese esa eso esto todo toda todos todas otro otra otros otras mas muy que y o e ni a
estas tareas cosas pasos tarea paso cosa siguiente siguientes plan orden`) {
		noVerboTarea[w] = true
	}
}

// palabrasVacias are not nouns of a task.
var palabrasVacias = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`el la los las un una unos unas lo al del de en con por para sin sobre mi mis tu tus
su sus este esta estos estas ese esa eso esto todo toda todos todas otro otra otros otras mas muy que y o e ni a
bien mal ya luego despues antes ahora cosa cosas algo nada se me te le les nos rato poco mucho minuto minutos hora
horas vez veces dentro fuera encima debajo`) {
		palabrasVacias[w] = true
	}
}

// verbosQueGastan use things up instead of producing them ("comer el pan" does not make bread).
var verbosQueGastan = map[string]bool{"comer": true, "beber": true, "usar": true, "utilizar": true, "gastar": true,
	"tirar": true, "vender": true, "regalar": true, "servir": true, "tomar": true, "probar": true, "llevar": true,
	"entregar": true, "comerse": true, "tomarse": true, "disfrutar": true}

func esInfinitivo(w string) bool {
	switch w {
	case "ir", "ver", "dar", "oir", "ser", "reir":
		return true
	}
	if len(w) < 4 {
		return false
	}
	for _, suf := range []string{"ar", "er", "ir", "arse", "erse", "irse", "arlo", "arla", "arlos", "arlas", "erlo", "erla",
		"erlos", "erlas", "irlo", "irla", "irlos", "irlas", "arle", "erle", "irle", "arles", "erles", "irles"} {
		if strings.HasSuffix(w, suf) {
			return true
		}
	}
	return false
}

func singularT(w string) string {
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

type tareaT struct {
	texto   string
	clave   string // normalized words, for references ("después de preparar la masa")
	verbo   string
	nombres []string
	cabeza  string
}

type depT struct {
	antes, despues int
	motivo         string
	deducida       bool
}

type planTareas struct {
	tareas   []tareaT
	deps     []depT
	etapas   [][]int
	nivel    []int
	ciclo    []int // tasks of a cycle (the first again at the end)
	conOrden bool  // some explicit order was given
	demasia  bool
}

func fraseT(ts []tokT, i int, f []string) bool {
	if i+len(f) > len(ts) {
		return false
	}
	for k, w := range f {
		if ts[i+k].w != w {
			return false
		}
	}
	return true
}

// iniciosFrases: the first words of every phrase list, so most tokens are skipped with one lookup.
var iniciosFrases = func() map[string]bool {
	m := map[string]bool{}
	for _, fs := range [][][]string{conectoresSeguidos, conectoresInicio, rellenoTarea} {
		for _, f := range fs {
			m[f[0]] = true
		}
	}
	for _, r := range relacionales {
		m[r.f[0]] = true
	}
	return m
}()

func algunaT(ts []tokT, i int, fs [][]string) int {
	if i >= len(ts) || !iniciosFrases[ts[i].w] {
		return 0
	}
	for _, f := range fs {
		if fraseT(ts, i, f) {
			return len(f)
		}
	}
	return 0
}

type lectorTareas struct {
	texto  string
	plan   *planTareas
	indice map[string]int
	hay    map[[2]int]bool // dependencies already recorded
}

// tarea adds (or finds) the task written in ts.
func (l *lectorTareas) tarea(ts []tokT) (int, bool) {
	for len(ts) > 0 {
		k := algunaT(ts, 0, rellenoTarea)
		if k == 0 {
			k = algunaT(ts, 0, conectoresInicio)
		}
		if k == 0 {
			break
		}
		ts = ts[k:]
	}
	if len(ts) == 0 || ts[0].w == "" || noVerboTarea[ts[0].w] || unicode.IsDigit(rune(ts[0].w[0])) {
		return 0, false
	}
	if len(ts) > maxPalabrasTarea {
		ts = ts[:maxPalabrasTarea]
	}
	var ws []string
	for _, t := range ts {
		ws = append(ws, t.w)
	}
	clave := strings.Join(ws, " ")
	if k, ok := l.indice[clave]; ok {
		return k, true
	}
	// "después de hornear, comer": a bare verb names the only task with that verb
	if len(ts) == 1 {
		hallada, n := 0, 0
		for k, t := range l.plan.tareas {
			if t.verbo == ts[0].w {
				hallada, n = k, n+1
			}
		}
		if n == 1 {
			return hallada, true
		}
	}
	if len(l.plan.tareas) >= maxTareas {
		l.plan.demasia = true
		return 0, false
	}
	t := tareaT{texto: strings.TrimSpace(l.texto[ts[0].desde:ts[len(ts)-1].hasta]), clave: clave, verbo: ts[0].w}
	for _, x := range ts[1:] {
		if palabrasVacias[x.w] || x.w == "" || unicode.IsDigit(rune(x.w[0])) {
			continue
		}
		t.nombres = append(t.nombres, singularT(x.w))
	}
	if len(t.nombres) > 0 && !verbosQueGastan[t.verbo] {
		t.cabeza = t.nombres[0]
	}
	l.indice[clave] = len(l.plan.tareas)
	l.plan.tareas = append(l.plan.tareas, t)
	return len(l.plan.tareas) - 1, true
}

// bloque splits a group of tasks said together: "preparar la masa y calentar el horno".
func (l *lectorTareas) bloque(ts []tokT) []int {
	var out []int
	var actual []tokT
	cerrar := func() {
		if len(actual) == 0 {
			return
		}
		if k, ok := l.tarea(actual); ok {
			dup := false
			for _, x := range out {
				dup = dup || x == k
			}
			if !dup {
				out = append(out, k)
			}
		}
		actual = nil
	}
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		corte := t.sep == "," || t.sep == "(" || t.sep == ")"
		if !corte && (t.w == "y" || t.w == "e" || t.w == "o" || t.w == "mientras") && i+1 < len(ts) {
			// "y" joins two tasks only when a verb follows ("y calentar el horno", "y calienta el
			// horno"); "la masa y el relleno" and "pan y leche" are one task
			sig := ts[i+1]
			tras := ""
			if i+2 < len(ts) {
				tras = ts[i+2].w
			}
			corte = sig.w != "" && !noVerboTarea[sig.w] && (esInfinitivo(sig.w) || articuloT[tras]) || t.w == "mientras"
		}
		if !corte && fraseT(ts, i, []string{"a", "la", "vez", "que"}) {
			cerrar()
			i += 3
			continue
		}
		if corte {
			cerrar()
			continue
		}
		if t.sep != "" {
			continue
		}
		actual = append(actual, t)
	}
	cerrar()
	return out
}

func (l *lectorTareas) depender(a, b []int, motivo string, p *planTareas) {
	if l.hay == nil {
		l.hay = map[[2]int]bool{}
	}
	for _, x := range a {
		for _, y := range b {
			if x == y || l.hay[[2]int{x, y}] {
				continue
			}
			l.hay[[2]int{x, y}] = true
			p.deps = append(p.deps, depT{antes: x, despues: y, motivo: motivo})
		}
	}
	p.conOrden = true
}

// parte reads one part of a sentence, with its "antes de" / "después de" if it has one, and returns
// its blocks in order.
func (l *lectorTareas) parte(ts []tokT) [][]int {
	for i := range ts {
		for _, r := range relacionales {
			if !iniciosFrases[ts[i].w] || !fraseT(ts, i, r.f) {
				continue
			}
			marca := strings.Join(r.f, " ")
			motivo := "lo dices con «" + marca + "»"
			if marca == "despues de" || marca == "antes de" {
				motivo = "lo dices con «" + strings.Replace(marca, "despues", "después", 1) + "»"
			}
			antes, despues := ts[:i], ts[i+len(r.f):]
			if i == 0 {
				// "antes de B, A" / "después de B, A": the comma ends B
				coma := -1
				for k, t := range despues {
					if t.sep == "," {
						coma = k
						break
					}
				}
				if coma < 0 {
					return [][]int{l.bloque(despues)}
				}
				b, a := l.bloque(despues[:coma]), l.bloque(despues[coma+1:])
				if r.primeroA { // antes de B, A → A then B
					l.depender(a, b, motivo, l.plan)
					return [][]int{a, b}
				}
				l.depender(b, a, motivo, l.plan)
				return [][]int{b, a}
			}
			a, b := l.bloque(antes), l.bloque(despues)
			if r.primeroA { // A antes de B
				l.depender(a, b, motivo, l.plan)
				return [][]int{a, b}
			}
			l.depender(b, a, motivo, l.plan) // A después de B
			return [][]int{b, a}
		}
	}
	return [][]int{l.bloque(ts)}
}

// leerTareas reads the tasks and their explicit order. It is linear in the text, apart from the
// dependencies themselves (at most maxTareas² of them).
func leerTareas(texto string) *planTareas {
	if len(texto) > maxTextoTareas {
		return &planTareas{demasia: true}
	}
	return leerTareasDe(texto, tokensTareas(texto))
}

func leerTareasDe(texto string, ts []tokT) *planTareas {
	p := &planTareas{}
	l := &lectorTareas{texto: texto, plan: p, indice: map[string]int{}}
	// the header ends at the first ":" when the words before it ask for an order
	cuerpo := ts
	for i, t := range ts {
		if t.sep == ":" {
			antes := " "
			for _, x := range ts[:i] {
				if x.w != "" {
					antes += x.w + " "
				}
			}
			if contiene(antes, cabecerasTareas...) || contiene(antes, palabrasPlanFuertes...) {
				cuerpo = ts[i+1:]
			}
			break
		}
	}
	var sinPregunta []tokT
	for _, t := range cuerpo {
		if !t.pregunta {
			sinPregunta = append(sinPregunta, t)
		}
	}
	l.leerCuerpo(quitarCabecera(sinPregunta))
	if len(p.tareas) < 2 && len(sinPregunta) < len(cuerpo) {
		// everything was inside the question: "¿qué hago primero, lavar los platos o barrer?"
		*p = planTareas{}
		l.indice, l.hay = map[string]int{}, nil
		l.leerCuerpo(quitarCabecera(cuerpo))
	}
	return p
}

// quitarCabecera drops "ordena estas tareas", "en qué orden hago esto", "qué hago primero" at the start.
func quitarCabecera(ts []tokT) []tokT {
	for len(ts) > 0 && ts[0].sep != "" {
		ts = ts[1:]
	}
	texto := " "
	for i := 0; i < len(ts) && i < 12; i++ {
		texto += ts[i].w + " "
		if !contiene(texto, cabecerasTareas...) {
			continue
		}
		inicio := i + 1
		// "ordena estas tareas", "en qué orden hago esto"
		for inicio < len(ts) && (noVerboTarea[ts[inicio].w] || ts[inicio].w == "hago" || ts[inicio].w == "haces" ||
			ts[inicio].w == "hacer" || ts[inicio].w == "en" || ts[inicio].sep != "") {
			inicio++
			if ts[inicio-1].sep == "," || ts[inicio-1].sep == ":" {
				break
			}
		}
		return ts[inicio:]
	}
	return ts
}

func (l *lectorTareas) leerCuerpo(ts []tokT) {
	p := l.plan
	var ultimo []int // the last block of the previous part
	for i := 0; i < len(ts); {
		// one sentence
		j := i
		for j < len(ts) && !finOracion(ts[j]) {
			j++
		}
		oracion := ts[i:j]
		// split by sequential connectors
		var partes [][]tokT
		var motivos []string
		inicio := 0
		motivo := ""
		for k := 0; k < len(oracion); {
			if n := algunaT(oracion, k, conectoresSeguidos); n > 0 && !(k+n < len(oracion) && oracion[k+n].w == "de") {
				partes = append(partes, oracion[inicio:k])
				motivos = append(motivos, motivo)
				motivo = "lo dices con «" + textoOriginal(oracion[k:k+n]) + "»"
				k += n
				for k < len(oracion) && oracion[k].sep == "," {
					k++
				}
				inicio = k
				continue
			}
			k++
		}
		partes = append(partes, oracion[inicio:])
		motivos = append(motivos, motivo)
		for pi, parte := range partes {
			bloques := l.parte(parte)
			var vivos [][]int
			for _, b := range bloques {
				if len(b) > 0 {
					vivos = append(vivos, b)
				}
			}
			if len(vivos) == 0 {
				continue
			}
			if motivos[pi] != "" && ultimo != nil {
				l.depender(ultimo, vivos[0], motivos[pi], p)
			}
			ultimo = vivos[len(vivos)-1]
		}
		if p.demasia {
			return
		}
		i = j + 1
	}
}

func finOracion(t tokT) bool { return t.sep != "" && strings.Contains(".;\n!:¿?", t.sep) }

func textoOriginal(ts []tokT) string {
	var ws []string
	for _, t := range ts {
		ws = append(ws, t.orig)
	}
	return strings.Join(ws, " ")
}

// ---- ordering ----

// ordenKahn returns a topological order (ties by order of appearance) and the tasks left in a cycle.
func ordenKahn(n int, deps []depT) ([]int, []int) {
	entran := make([]int, n)
	salen := make([][]int, n)
	for _, d := range deps {
		entran[d.despues]++
		salen[d.antes] = append(salen[d.antes], d.despues)
	}
	var listos, orden []int
	for i := 0; i < n; i++ {
		if entran[i] == 0 {
			listos = append(listos, i)
		}
	}
	for len(listos) > 0 {
		sort.Ints(listos)
		x := listos[0]
		listos = listos[1:]
		orden = append(orden, x)
		for _, y := range salen[x] {
			entran[y]--
			if entran[y] == 0 {
				listos = append(listos, y)
			}
		}
	}
	var resto []int
	for i := 0; i < n; i++ {
		if entran[i] > 0 {
			resto = append(resto, i)
		}
	}
	return orden, resto
}

// buscarCiclo finds a cycle among the tasks left by Kahn (each of them has a predecessor there).
func buscarCiclo(resto []int, deps []depT) []int {
	en := map[int]bool{}
	for _, x := range resto {
		en[x] = true
	}
	pred := map[int]int{}
	for _, d := range deps {
		if en[d.antes] && en[d.despues] {
			pred[d.despues] = d.antes
		}
	}
	// walking back through predecessors must repeat a task: that loop is the cycle
	visto := map[int]int{}
	x := resto[0]
	for paso := 0; ; paso++ {
		if k, ok := visto[x]; ok {
			var camino []int
			for y, i := x, 0; i < paso-k; i++ {
				camino = append(camino, y)
				y = pred[y]
			}
			// camino goes backwards: reverse it so each task goes before the next
			for a, b := 0, len(camino)-1; a < b; a, b = a+1, b-1 {
				camino[a], camino[b] = camino[b], camino[a]
			}
			return camino
		}
		visto[x] = paso
		x = pred[x]
	}
}

func alcanzable(n int, deps []depT, desde, hasta int) bool {
	salen := make([][]int, n)
	for _, d := range deps {
		salen[d.antes] = append(salen[d.antes], d.despues)
	}
	visto := make([]bool, n)
	pila := []int{desde}
	for len(pila) > 0 {
		x := pila[len(pila)-1]
		pila = pila[:len(pila)-1]
		if x == hasta {
			return true
		}
		if visto[x] {
			continue
		}
		visto[x] = true
		pila = append(pila, salen[x]...)
	}
	return false
}

// ordenar adds the causal dependencies and computes the stages. It returns false when there is a cycle.
//
// Causal dependencies: a task that uses a noun waits for the tasks that make it. A task makes the noun
// of its object when its verb is one that makes or gets things ("comprar harina", "juntar la leña");
// a task that only uses the noun waits for every maker, even one written later ("subir la leña al
// faro" waits for "juntar la leña"), and a maker waits only for the makers written before it. When
// no verb of that kind names the noun, the earlier task with that noun counts as its maker (as in
// Gnostos). A deduced dependency is never added against the order that was said (no cycles).
func (p *planTareas) ordenar(ctx context.Context) bool {
	n := len(p.tareas)
	orden, resto := ordenKahn(n, p.deps)
	if len(resto) > 0 {
		p.ciclo = buscarCiclo(resto, p.deps)
		return false
	}
	pos := make([]int, n)
	for i, v := range orden {
		pos[v] = i
	}
	hacen := map[string][]int{} // noun → tasks whose verb makes it
	for v, t := range p.tareas {
		if t.cabeza != "" && verboQueHace(t.verbo) {
			hacen[t.cabeza] = append(hacen[t.cabeza], v)
		}
	}
	deducir := func(u, v int, nom string) {
		if u == v || alcanzable(n, p.deps, u, v) || alcanzable(n, p.deps, v, u) {
			return
		}
		p.deps = append(p.deps, depT{antes: u, despues: v, deducida: true,
			motivo: fmt.Sprintf("usa «%s», que sale de «%s»", nom, p.tareas[u].texto)})
	}
	anterior := map[string]int{} // noun → the last task in order whose object starts with it
	for _, v := range orden {
		if ctx.Err() != nil {
			break
		}
		t := p.tareas[v]
		for _, nom := range t.nombres {
			if ps := hacen[nom]; len(ps) > 0 {
				haceEsta := false
				for _, u := range ps {
					haceEsta = haceEsta || u == v
				}
				for _, u := range ps {
					if !haceEsta || pos[u] < pos[v] {
						deducir(u, v, nom)
					}
				}
				continue
			}
			if u, ok := anterior[nom]; ok {
				deducir(u, v, nom)
			}
		}
		if t.cabeza != "" {
			anterior[t.cabeza] = v
		}
	}
	orden, resto = ordenKahn(n, p.deps)
	if len(resto) > 0 { // cannot happen: a deduced dependency never closes a cycle
		p.ciclo = buscarCiclo(resto, p.deps)
		return false
	}
	p.nivel = make([]int, n)
	for _, v := range orden {
		for _, d := range p.deps {
			if d.despues == v && p.nivel[d.antes]+1 > p.nivel[v] {
				p.nivel[v] = p.nivel[d.antes] + 1
			}
		}
	}
	maxNivel := 0
	for _, x := range p.nivel {
		maxNivel = max(maxNivel, x)
	}
	p.etapas = make([][]int, maxNivel+1)
	for _, v := range orden {
		p.etapas[p.nivel[v]] = append(p.etapas[p.nivel[v]], v)
	}
	return true
}

// raicesQueHacen are the stems of verbs that make or get the thing they name (infinitive and
// imperative stems).
var raicesQueHacen = map[string]bool{}

func init() {
	// only verbs that create or obtain: "cortar", "lavar" or "calentar" change a thing that already exists
	for _, w := range strings.Fields(`hac haz prepar junt recog reun compr consegu consig tra busc cocin horne fabric constru
cre escrib coc cuec fre fri imprim descarg ped pid encarg obten obtien gan cos dibuj cog elabor produc produz gener
redact grab arm diseñ program pesc caz cultiv cosech`) {
		raicesQueHacen[w] = true
	}
}

// verboQueHace: "comprar", "compra", "juntar", "haz"…
func verboQueHace(w string) bool {
	if raicesQueHacen[w] {
		return true
	}
	for _, suf := range []string{"ar", "er", "ir", "a", "e"} {
		if strings.HasSuffix(w, suf) && raicesQueHacen[strings.TrimSuffix(w, suf)] {
			return true
		}
	}
	return false
}

// comprobarOrden: every task in exactly one stage, every dependency goes to a later stage, and a
// second order (depth-first) also respects every dependency.
func (p *planTareas) comprobarOrden() bool {
	n := len(p.tareas)
	vista := make([]int, n)
	for e, et := range p.etapas {
		if len(et) == 0 {
			return false
		}
		for _, v := range et {
			vista[v]++
			if p.nivel[v] != e {
				return false
			}
		}
	}
	for _, c := range vista {
		if c != 1 {
			return false
		}
	}
	for _, d := range p.deps {
		if p.nivel[d.antes] >= p.nivel[d.despues] {
			return false
		}
	}
	// depth-first order (reverse post-order)
	salen := make([][]int, n)
	for _, d := range p.deps {
		salen[d.antes] = append(salen[d.antes], d.despues)
	}
	estado := make([]byte, n)
	var post []int
	for s := 0; s < n; s++ {
		if estado[s] != 0 {
			continue
		}
		type marco struct{ v, k int }
		pila := []marco{{s, 0}}
		estado[s] = 1
		for len(pila) > 0 {
			m := &pila[len(pila)-1]
			if m.k < len(salen[m.v]) {
				w := salen[m.v][m.k]
				m.k++
				switch estado[w] {
				case 0:
					estado[w] = 1
					pila = append(pila, marco{w, 0})
				case 1:
					return false // a cycle
				}
				continue
			}
			estado[m.v] = 2
			post = append(post, m.v)
			pila = pila[:len(pila)-1]
		}
	}
	pos := make([]int, n)
	for i, v := range post {
		pos[v] = n - 1 - i
	}
	for _, d := range p.deps {
		if pos[d.antes] >= pos[d.despues] {
			return false
		}
	}
	return len(post) == n
}

// ---- the resolver ----

type resTareas struct{}

func (resTareas) Nombre() string { return "puzles.tareas" }

var palabrasProgramacion = []string{" funcion", " programa ", " programar", " codigo", " func ", " script", " algoritmo",
	" variable", " slice", " array"}

// soloPalabras: the normalized words between spaces, without punctuation.
func soloPalabras(ts []tokT) string {
	var sb strings.Builder
	sb.WriteByte(' ')
	for _, t := range ts {
		if t.w != "" {
			sb.WriteString(t.w)
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

func (resTareas) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPlan, Lectura: "ordenar tareas que dependen unas de otras"}
	if p == nil || strings.TrimSpace(p.Codigo) != "" || len(p.Texto) > maxTextoTareas {
		return rec
	}
	// recognizing needs only the start of a long text (Resolver reads it all)
	texto := p.Texto
	if len(texto) > 800 {
		corte := 800
		for corte > 0 && !utf8.RuneStart(texto[corte]) {
			corte--
		}
		texto = texto[:corte]
	}
	ts := tokensTareas(texto)
	t := soloPalabras(ts)
	if !contiene(t, cabecerasTareas...) || contiene(" "+nucleo.Normalizar(p.Texto)+" ", palabrasProgramacion...) || strings.ContainsAny(p.Texto, "=[]{}") {
		return rec
	}
	if (resPlan{}).Reconoce(p).Puntos > 0 {
		return rec // jugs, river crossings, Hanoi…: puzles.plan
	}
	pt := leerTareasDe(texto, ts)
	if pt.demasia || len(pt.tareas) < 2 {
		return rec
	}
	infinitivos := 0
	for _, x := range pt.tareas {
		if esInfinitivo(x.verbo) {
			infinitivos++
		}
	}
	fuerte := contiene(t, palabrasPlanFuertes...)
	switch {
	case 2*infinitivos >= len(pt.tareas) && fuerte:
		rec.Puntos = 0.92
	case 2*infinitivos >= len(pt.tareas) || fuerte && pt.conOrden:
		rec.Puntos = 0.88
	}
	return rec
}

func (resTareas) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	pt := leerTareas(p.Texto)
	switch {
	case pt.demasia:
		return nucleo.Respuesta{}, fmt.Errorf("puzles: como mucho ordeno %d tareas: %w", maxTareas, nucleo.ErrNoSoportado)
	case len(pt.tareas) < 2:
		return nucleo.Respuesta{}, fmt.Errorf("puzles: no veo al menos dos tareas (escríbelas así: «preparar la masa y calentar el horno; luego hornear el pan»): %w", nucleo.ErrNoEntiendo)
	}
	leer := n.Sub(nucleo.PasoEntender, "Leo %d tareas y %s", len(pt.tareas), plural(len(pt.deps), "dependencia que me dices", "dependencias que me dices"))
	for _, d := range pt.deps {
		leer.Detalle("«%s» antes que «%s» (%s)", pt.tareas[d.antes].texto, pt.tareas[d.despues].texto, d.motivo)
	}
	leer.Info("")
	var tareas []string
	for _, t := range pt.tareas {
		tareas = append(tareas, t.texto)
	}
	entendi := "Así lo entendí: " + strings.Join(tareas, "; ")
	ordenar := n.Sub(nucleo.PasoIntento, "Ordeno el grafo de dependencias por etapas (orden topológico)")
	if !pt.ordenar(ctx) {
		ordenar.Mal("Hay un ciclo")
		return respuestaCiclo(pt, entendi, n)
	}
	if ctx.Err() != nil {
		return nucleo.Respuesta{}, nucleo.ErrSinTiempo
	}
	deducidas := 0
	for _, d := range pt.deps {
		if d.deducida {
			deducidas++
			ordenar.Detalle("Deduzco: «%s» espera a «%s» (%s)", pt.tareas[d.despues].texto, pt.tareas[d.antes].texto, d.motivo)
		}
	}
	ordenar.Bien("%s", plural(len(pt.etapas), "etapa", "etapas"))
	comp := n.Sub(nucleo.PasoComprobar, "Compruebo que el orden respeta %s", lasDependencias(len(pt.deps)))
	if !pt.comprobarOrden() {
		comp.Mal("No las respeta")
		return nucleo.Respuesta{}, fmt.Errorf("puzles: el orden no respeta las dependencias (error interno)")
	}
	comp.Bien("Las respeta todas, y otro orden (en profundidad) también")

	var lineas, pasos []string // lineas carry the stage number for the sentence; Plan is numbered when shown
	r := respuestaCalculo("", "orden_topologico", "Comprobé que el orden respeta "+lasDependencias(len(pt.deps)))
	r.Comprobaciones[0].Pasan, r.Comprobaciones[0].Total = len(pt.deps), len(pt.deps)
	r.Comprobaciones[0].Detalle = "cada tarea está en una sola etapa, cada dependencia va a una etapa posterior y un segundo orden (en profundidad) también las respeta"
	tb := nucleo.Tabla{Titulo: "Tareas", Cabecera: []string{"Etapa", "Tarea", "Espera a"}}
	for e, et := range pt.etapas {
		var nombres []string
		for _, v := range et {
			nombres = append(nombres, pt.tareas[v].texto)
			var espera []string
			for _, d := range pt.deps {
				if d.despues == v {
					espera = append(espera, pt.tareas[d.antes].texto)
				}
			}
			tb.Filas = append(tb.Filas, []string{fmt.Sprint(e + 1), pt.tareas[v].texto, strings.Join(espera, ", ")})
		}
		lineas = append(lineas, fmt.Sprintf("%d. %s", e+1, strings.Join(nombres, " ‖ ")))
		pasos = append(pasos, strings.Join(nombres, " ‖ "))
	}
	r.Plan = pasos
	r.Tablas = []nucleo.Tabla{tb}
	if len(pt.etapas) == 1 {
		r.Texto = "Ninguna depende de otra: puedes hacerlas en cualquier orden, o a la vez: " + strings.Join(tareas, " ‖ ") + "."
	} else {
		r.Texto = fmt.Sprintf("Orden en %d etapas: %s.", len(pt.etapas), strings.Join(lineas, "; "))
	}
	if strings.Contains(strings.Join(lineas, ""), "‖") {
		r.Parrafos = append(r.Parrafos, "‖ quiere decir que esas tareas no dependen una de otra: se pueden hacer a la vez o en cualquier orden.")
	}
	var porque []string
	for _, d := range pt.deps {
		txt := fmt.Sprintf("«%s» va después de «%s» (%s", pt.tareas[d.despues].texto, pt.tareas[d.antes].texto, d.motivo)
		if d.deducida {
			txt += "; no lo dices, lo deduzco"
		}
		porque = append(porque, txt+")")
	}
	if len(porque) > 30 {
		porque = append(porque[:30], fmt.Sprintf("… y %d dependencias más", len(porque)-30))
	}
	r.Justificacion = porque
	if len(porque) > 0 {
		r.Parrafos = append(r.Parrafos, "Por qué: "+strings.Join(porque, "; ")+".")
	}
	r.Parrafos = append(r.Parrafos, "Compruebo que el orden respeta "+lasDependencias(len(pt.deps))+" y que cada tarea sale una sola vez ✔")
	if deducidas > 0 {
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("%s: no me lo dices, lo deduzco porque una tarea usa algo que sale de otra.", mayus(plural(deducidas, "dependencia deducida", "dependencias deducidas"))))
	}
	r.Entendi = entendi
	r.Sugerencias = []string{"¿Por qué?"}
	return r, nil
}

func lasDependencias(n int) string {
	switch n {
	case 0:
		return "todo (no hay dependencias)"
	case 1:
		return "la única dependencia"
	}
	return fmt.Sprintf("las %d dependencias", n)
}

// respuestaCiclo explains why the tasks cannot be ordered, after checking the cycle link by link.
func respuestaCiclo(pt *planTareas, entendi string, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	c := pt.ciclo
	if len(c) < 2 {
		return nucleo.Respuesta{}, fmt.Errorf("puzles: no encuentro el ciclo (error interno)")
	}
	comp := n.Sub(nucleo.PasoComprobar, "Compruebo el ciclo enlace a enlace")
	var enlaces []string
	for i := range c {
		a, b := c[i], c[(i+1)%len(c)]
		hallada := ""
		for _, d := range pt.deps {
			if d.antes == a && d.despues == b {
				hallada = d.motivo
				break
			}
		}
		if hallada == "" {
			comp.Mal("Un enlace no es una dependencia")
			return nucleo.Respuesta{}, fmt.Errorf("puzles: el ciclo no se sostiene (error interno)")
		}
		enlaces = append(enlaces, fmt.Sprintf("«%s» va antes que «%s» (%s)", pt.tareas[a].texto, pt.tareas[b].texto, hallada))
	}
	comp.Bien("Cada enlace es una dependencia que me diste")
	r := respuestaCalculo("No se pueden ordenar: hay un ciclo, unas tareas se esperan a otras en redondo.", "ciclo",
		"Comprobé el ciclo enlace a enlace: cada uno es una dependencia que me diste")
	r.Parrafos = []string{"El ciclo: " + strings.Join(enlaces, "; ") + ".", "Revisa alguna de esas dependencias: con un ciclo ninguna de esas tareas puede empezar."}
	r.Justificacion = enlaces
	r.Entendi = entendi
	return r, nil
}
