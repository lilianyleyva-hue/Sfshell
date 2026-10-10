package logica

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Cuantificador is the form of a categorical sentence.
type Cuantificador int

const (
	Todos     Cuantificador = iota // todos los A son B
	Ningun                         // ningún A es B
	Algunos                        // algunos A son B
	AlgunosNo                      // algunos A no son B
	Individuo                      // Sócrates es (un) B
)

// Categorica is one categorical sentence. Negada (additive) only applies to Individuo
// ("Sócrates no es inmortal"). FraseSujeto/FrasePredicado (additive) keep the words as written.
type Categorica struct {
	Cuant     Cuantificador
	Sujeto    string // lemma: "perro"
	Predicado string // lemma: "mamifero"

	Negada         bool
	FraseSujeto    string
	FrasePredicado string
}

// Letra is the classic vowel of the form: A, E, I, O (an individual counts as A, or E when negated).
func (c Categorica) Letra() string {
	switch c.Cuant {
	case Todos:
		return "A"
	case Ningun:
		return "E"
	case Algunos:
		return "I"
	case AlgunosNo:
		return "O"
	}
	if c.Negada {
		return "E"
	}
	return "A"
}

func (c Categorica) nombreSujeto() string {
	if c.FraseSujeto != "" {
		return c.FraseSujeto
	}
	return c.Sujeto
}

func (c Categorica) nombrePredicado() string {
	if c.FrasePredicado != "" {
		return c.FrasePredicado
	}
	return c.Predicado
}

// String writes the sentence in Spanish: "todos los A son B", "ningún A es B"…
func (c Categorica) String() string {
	s, p := c.nombreSujeto(), c.nombrePredicado()
	switch c.Cuant {
	case Todos:
		return fmt.Sprintf("todos los %s son %s", s, p)
	case Ningun:
		return fmt.Sprintf("ningún %s es %s", s, p)
	case Algunos:
		return fmt.Sprintf("algunos %s son %s", s, p)
	case AlgunosNo:
		return fmt.Sprintf("algunos %s no son %s", s, p)
	}
	if c.Negada {
		return fmt.Sprintf("%s no es %s", s, p)
	}
	return fmt.Sprintf("%s es %s", s, p)
}

var copulas = map[string]bool{"son": true, "es": true, "somos": true, "sea": true, "sean": true, "soy": true, "eres": true, "sois": true}

var determinantesTermino = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "un": true, "una": true, "unos": true, "unas": true, "lo": true,
}

var prefijosCategorica = [][]string{
	{"recuerda", "que"}, {"aprende", "que"}, {"ten", "en", "cuenta", "que"}, {"sabemos", "que"}, {"sabiendo", "que"},
	{"y", "si"}, {"y"},
}

// AnalizarCategorica reads one categorical sentence: "todos los A son B", "todo A es B", "ningún A
// es B", "no hay A que sean B", "algunos A son B", "hay A que son B", "algunos A no son B", "no
// todos los A son B", "los A son B" (= todos) and "Sócrates es (un) B" / "Toby no es un gato"
// (individuals). Terms are stored as lemmas; the words as written go to FraseSujeto/FrasePredicado.
func AnalizarCategorica(frase string, lem nucleo.Lematizador) (Categorica, error) {
	var ts []tok
	for _, t := range tokenizar(frase) {
		if t.sim {
			continue
		}
		ts = append(ts, t)
	}
	for {
		r, ok := quitarPrefijo(ts, prefijosCategorica)
		if !ok {
			r, ok = quitarPrefijo(ts, marcadoresConclusion)
		}
		if !ok {
			r, ok = quitarPrefijo(ts, marcadoresPregunta)
		}
		if !ok {
			break
		}
		ts = r
	}
	noEntiendo := fmt.Errorf("logica: «%s» no es una frase del tipo «todos los A son B»: %w", strings.TrimSpace(frase), nucleo.ErrNoEntiendo)
	if len(ts) < 3 {
		return Categorica{}, noEntiendo
	}
	c := -1
	for i, t := range ts {
		if copulas[t.norma] {
			c = i
			break
		}
	}
	if c < 1 || c == len(ts)-1 {
		return Categorica{}, noEntiendo
	}
	negCopula := ts[c-1].norma == "no"
	finSujeto := c
	if negCopula {
		finSujeto = c - 1
	}
	pred := ts[c+1:]
	w0 := ts[0].norma
	var cat Categorica
	var suj []tok
	switch {
	case w0 == "todos" || w0 == "todas" || w0 == "todo" || w0 == "toda" || w0 == "cada":
		suj = ts[1:finSujeto]
		cat.Cuant = Todos
		if negCopula {
			cat.Cuant = Ningun
		}
	case strings.HasPrefix(w0, "ningun") || w0 == "ninguna" || w0 == "ningunas" || w0 == "nadie":
		suj = ts[1:finSujeto]
		cat.Cuant = Ningun
	case w0 == "no" && len(ts) > 1 && (ts[1].norma == "todos" || ts[1].norma == "todas" || ts[1].norma == "todo" || ts[1].norma == "toda"):
		suj = ts[2:finSujeto]
		cat.Cuant = AlgunosNo
	case w0 == "no" && len(ts) > 1 && (ts[1].norma == "hay" || ts[1].norma == "existe" || ts[1].norma == "existen"):
		suj = hastaQue(ts[2:finSujeto])
		cat.Cuant = Ningun
	case w0 == "algunos" || w0 == "algunas" || w0 == "algun" || w0 == "alguna" || w0 == "alguno" || w0 == "ciertos" || w0 == "ciertas":
		suj = ts[1:finSujeto]
		cat.Cuant = Algunos
		if negCopula {
			cat.Cuant = AlgunosNo
		}
	case w0 == "hay" || w0 == "existen" || w0 == "existe":
		suj = hastaQue(ts[1:finSujeto])
		cat.Cuant = Algunos
		if negCopula {
			cat.Cuant = AlgunosNo
		}
	case (w0 == "los" || w0 == "las") && ts[c].norma == "son":
		suj = ts[1:finSujeto]
		cat.Cuant = Todos
		if negCopula {
			cat.Cuant = Ningun
		}
	default:
		suj = ts[:finSujeto]
		cat.Cuant = Individuo
		cat.Negada = negCopula
	}
	suj = sinDeterminantes(suj)
	pred = sinDeterminantes(pred)
	if len(suj) == 0 || len(pred) == 0 || len(suj) > 4 || len(pred) > 4 {
		return Categorica{}, noEntiendo
	}
	if cat.Cuant == Individuo {
		cat.Sujeto = nucleo.Normalizar(textoToks(suj))
		cat.FraseSujeto = textoOriginal(suj)
	} else {
		cat.Sujeto = lemaFrase(suj, lem)
		cat.FraseSujeto = textoOriginal(suj)
	}
	cat.Predicado = lemaFrase(pred, lem)
	cat.FrasePredicado = textoOriginal(pred)
	return cat, nil
}

// hastaQue cuts "A que" → "A" ("hay perros que son blancos").
func hastaQue(ts []tok) []tok {
	for i, t := range ts {
		if t.norma == "que" {
			return ts[:i]
		}
	}
	return ts
}

func sinDeterminantes(ts []tok) []tok {
	var out []tok
	for _, t := range ts {
		if determinantesTermino[t.norma] {
			continue
		}
		out = append(out, t)
	}
	return out
}

func lemaFrase(ts []tok, lem nucleo.Lematizador) string {
	var ws []string
	for _, t := range ts {
		if len(t.orig) == 1 && t.mayus {
			ws = append(ws, t.orig)
			continue
		}
		ws = append(ws, lemaTermino(t.orig, lem))
	}
	return strings.Join(ws, " ")
}

// textoOriginal keeps names and single capital letters as written, lowercasing the rest.
func textoOriginal(ts []tok) string {
	var ws []string
	for _, t := range ts {
		if t.mayus && (utf8.RuneCountInString(t.orig) == 1 || len(ws) == 0) {
			ws = append(ws, t.orig)
		} else {
			ws = append(ws, strings.ToLower(t.orig))
		}
	}
	return strings.Join(ws, " ")
}

// claveTermino compares terms: word roots ("hombres" = "hombre").
func claveTermino(s string) string {
	var ws []string
	for _, w := range strings.Fields(s) {
		ws = append(ws, raizTermino(w))
	}
	return strings.Join(ws, " ")
}

// ResultadoSilogismo is the verdict on a syllogism.
type ResultadoSilogismo struct {
	Valido        bool
	Nombre        string   // "Barbara", "Celarent", "Darii", "Ferio", …
	Contraejemplo string   // Spanish description of a Venn model
	Conclusiones  []string // for "¿qué se concluye?"
	Pasos         []string
}

// modosValidos are the 24 valid moods, by figure and letters (major, minor, conclusion).
var modosValidos = map[string]string{
	"1AAA": "Barbara", "1EAE": "Celarent", "1AII": "Darii", "1EIO": "Ferio", "1AAI": "Barbari", "1EAO": "Celaront",
	"2EAE": "Cesare", "2AEE": "Camestres", "2EIO": "Festino", "2AOO": "Baroco", "2EAO": "Cesaro", "2AEO": "Camestros",
	"3AAI": "Darapti", "3IAI": "Disamis", "3AII": "Datisi", "3EAO": "Felapton", "3OAO": "Bocardo", "3EIO": "Ferison",
	"4AAI": "Bramantip", "4AEE": "Camenes", "4IAI": "Dimaris", "4EAO": "Fesapo", "4EIO": "Fresison", "4AEO": "Camenos",
}

// venn holds the terms of a syllogism and encodes sentences over its 2^k regions.
type venn struct {
	claves     []string
	lemas      []string
	nombres    []string
	individuos map[int]bool
	k          int
	nRegiones  int
}

const maxTerminosSilogismo = 6

func nuevoVenn(cs []Categorica) (*venn, error) {
	v := &venn{individuos: map[int]bool{}}
	indice := map[string]int{}
	agregar := func(lema, frase string) int {
		k := claveTermino(lema)
		if i, ok := indice[k]; ok {
			return i
		}
		indice[k] = len(v.claves)
		v.claves = append(v.claves, k)
		v.lemas = append(v.lemas, lema)
		nombre := frase
		if nombre == "" {
			nombre = lema
		}
		v.nombres = append(v.nombres, nombre)
		return len(v.claves) - 1
	}
	for _, c := range cs {
		s := agregar(c.Sujeto, c.FraseSujeto)
		agregar(c.Predicado, c.FrasePredicado)
		if c.Cuant == Individuo {
			v.individuos[s] = true
		}
	}
	v.k = len(v.claves)
	if v.k > maxTerminosSilogismo {
		return nil, fmt.Errorf("logica: hay %d términos; solo razono con %d o menos: %w", v.k, maxTerminosSilogismo, nucleo.ErrNoSoportado)
	}
	v.nRegiones = 1 << v.k
	return v, nil
}

func (v *venn) termino(lema string) int {
	k := claveTermino(lema)
	for i, c := range v.claves {
		if c == k {
			return i
		}
	}
	return -1
}

// clausulas encodes c (or its negation) as CNF over region variables (region r ↔ variable r+1,
// true = the region has something in it).
func (v *venn) clausulas(c Categorica, negada bool) [][]int {
	a, b := v.termino(c.Sujeto), v.termino(c.Predicado)
	en := func(r, t int) bool { return r&(1<<t) != 0 }
	vacias := func(cond func(r int) bool) [][]int {
		var out [][]int
		for r := 0; r < v.nRegiones; r++ {
			if cond(r) {
				out = append(out, []int{-(r + 1)})
			}
		}
		return out
	}
	alguna := func(cond func(r int) bool) [][]int {
		var cl []int
		for r := 0; r < v.nRegiones; r++ {
			if cond(r) {
				cl = append(cl, r+1)
			}
		}
		return [][]int{cl}
	}
	aNoB := func(r int) bool { return en(r, a) && !en(r, b) }
	aYB := func(r int) bool { return en(r, a) && en(r, b) }
	cuant := c.Cuant
	if cuant == Individuo {
		// with exactly one region for the individual, "s es B" = no s-region outside B
		if c.Negada != negada {
			return alguna(aNoB)
		}
		return vacias(aNoB)
	}
	if negada {
		switch cuant {
		case Todos:
			cuant = AlgunosNo
		case Ningun:
			cuant = Algunos
		case Algunos:
			cuant = Ningun
		case AlgunosNo:
			cuant = Todos
		}
	}
	switch cuant {
	case Todos:
		return vacias(aNoB)
	case Ningun:
		return vacias(aYB)
	case Algunos:
		return alguna(aYB)
	default:
		return alguna(aNoB)
	}
}

// base: each individual is in exactly one region; with importe, every general term is non-empty.
func (v *venn) base(importe bool) [][]int {
	var out [][]int
	for t := 0; t < v.k; t++ {
		var rs []int
		for r := 0; r < v.nRegiones; r++ {
			if r&(1<<t) != 0 {
				rs = append(rs, r+1)
			}
		}
		if v.individuos[t] {
			out = append(out, rs)
			for i := 0; i < len(rs); i++ {
				for j := i + 1; j < len(rs); j++ {
					out = append(out, []int{-rs[i], -rs[j]})
				}
			}
		} else if importe {
			out = append(out, rs)
		}
	}
	return out
}

// decidir runs DPLL (and brute force when there are ≤ 16 regions) on the clauses.
func (v *venn) decidir(ctx context.Context, cnf [][]int) ([]bool, bool, error) {
	modelo, sat, err := DPLL(ctx, cnf, v.nRegiones, MaxDecisiones)
	if err != nil {
		return nil, false, err
	}
	if v.nRegiones <= 16 {
		if _, s := satFuerzaCNF(cnf, v.nRegiones); s != sat {
			return nil, false, ErrNoCoinciden
		}
	}
	return modelo, sat, nil
}

// satFuerzaCNF checks a CNF by enumerating all assignments (n ≤ 20).
func satFuerzaCNF(cnf [][]int, n int) ([]bool, bool) {
	v := make([]bool, n)
	for mask := 0; mask < 1<<n; mask++ {
		for i := 0; i < n; i++ {
			v[i] = mask&(1<<i) != 0
		}
		ok := true
		for _, c := range cnf {
			sat := false
			for _, l := range c {
				if (l > 0 && v[l-1]) || (l < 0 && !v[-l-1]) {
					sat = true
					break
				}
			}
			if !sat {
				ok = false
				break
			}
		}
		if ok {
			return append([]bool(nil), v...), true
		}
	}
	return nil, false
}

func (v *venn) describirModelo(m []bool) string {
	var partes []string
	for r := 1; r < v.nRegiones; r++ {
		if r >= len(m) || !m[r] {
			continue
		}
		var dentro, fuera []string
		ind := -1
		for t := 0; t < v.k; t++ {
			if r&(1<<t) != 0 {
				if v.individuos[t] {
					ind = t
					continue
				}
				dentro = append(dentro, v.nombres[t])
			} else if !v.individuos[t] {
				fuera = append(fuera, v.nombres[t])
			}
		}
		var s string
		if ind >= 0 {
			s = v.nombres[ind]
			if len(dentro) > 0 {
				s += " es " + strings.Join(dentro, " y ")
				if len(fuera) > 0 {
					s += " pero no es " + strings.Join(fuera, " ni ")
				}
			} else {
				s += " no es " + strings.Join(fuera, " ni ")
			}
		} else {
			if len(dentro) == 0 {
				continue
			}
			s = "algo que es " + strings.Join(dentro, " y ")
			if len(fuera) > 0 {
				s += " sin ser " + strings.Join(fuera, " ni ")
			}
		}
		partes = append(partes, s)
	}
	if len(partes) == 0 {
		return "un mundo donde no hay nada"
	}
	return "puede haber " + unirY(partes)
}

func describirClausulasVenn(c Categorica) string {
	switch c.Cuant {
	case Todos:
		return fmt.Sprintf("«%s»: están vacías las zonas de %s fuera de %s", c, c.nombreSujeto(), c.nombrePredicado())
	case Ningun:
		return fmt.Sprintf("«%s»: están vacías las zonas comunes a %s y %s", c, c.nombreSujeto(), c.nombrePredicado())
	case Algunos:
		return fmt.Sprintf("«%s»: alguna zona común a %s y %s tiene algo", c, c.nombreSujeto(), c.nombrePredicado())
	case AlgunosNo:
		return fmt.Sprintf("«%s»: alguna zona de %s fuera de %s tiene algo", c, c.nombreSujeto(), c.nombrePredicado())
	}
	if c.Negada {
		return fmt.Sprintf("«%s»: %s está en una sola zona, fuera de %s", c, c.nombreSujeto(), c.nombrePredicado())
	}
	return fmt.Sprintf("«%s»: %s está en una sola zona, dentro de %s", c, c.nombreSujeto(), c.nombrePredicado())
}

// Silogismo decides whether the categorical premises entail the conclusion, on Venn regions with
// DPLL (cross-checked by brute force for ≤ 4 terms). It first uses the modern reading (a "todos"
// sentence does not claim that something exists); if that fails it tries the classical reading
// (every general term is non-empty) and says so. With concl == nil it lists what follows.
func Silogismo(ctx context.Context, prem []Categorica, concl *Categorica) (ResultadoSilogismo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(prem) == 0 {
		return ResultadoSilogismo{}, fmt.Errorf("logica: no hay premisas: %w", nucleo.ErrNoEntiendo)
	}
	todas := append([]Categorica(nil), prem...)
	if concl != nil {
		todas = append(todas, *concl)
	}
	v, err := nuevoVenn(todas)
	if err != nil {
		return ResultadoSilogismo{}, err
	}
	var res ResultadoSilogismo
	res.Pasos = append(res.Pasos, fmt.Sprintf("Dibujo un diagrama de Venn con %d términos (%s): %d zonas.", v.k, strings.Join(v.nombres, ", "), v.nRegiones))
	var cnfPrem [][]int
	for _, p := range prem {
		cnfPrem = append(cnfPrem, v.clausulas(p, false)...)
		res.Pasos = append(res.Pasos, describirClausulasVenn(p)+".")
	}
	// consistent premises?
	if _, sat, err := v.decidir(ctx, append(append([][]int(nil), cnfPrem...), v.base(false)...)); err != nil {
		return ResultadoSilogismo{}, err
	} else if !sat {
		res.Pasos = append(res.Pasos, "Las premisas se contradicen entre sí: de ellas se sigue cualquier cosa.")
		if concl != nil {
			res.Valido = true
		}
		return res, nil
	}
	if concl == nil {
		res.Conclusiones, err = v.conclusiones(ctx, prem, cnfPrem)
		if err != nil {
			return ResultadoSilogismo{}, err
		}
		if len(res.Conclusiones) == 0 {
			res.Pasos = append(res.Pasos, "Ninguna frase nueva sobre estos términos se sigue de las premisas.")
		} else {
			res.Pasos = append(res.Pasos, "Se sigue: "+strings.Join(res.Conclusiones, "; ")+".")
		}
		return res, nil
	}
	neg := v.clausulas(*concl, true)
	res.Pasos = append(res.Pasos, fmt.Sprintf("Busco con DPLL una forma de llenar las zonas donde las premisas sean verdad y «%s» sea falso.", *concl))
	moderno := append(append(append([][]int(nil), cnfPrem...), neg...), v.base(false)...)
	_, sat, err := v.decidir(ctx, moderno)
	if err != nil {
		return ResultadoSilogismo{}, err
	}
	if !sat {
		res.Valido = true
		res.Pasos = append(res.Pasos, "No existe: la conclusión se sigue.")
	} else {
		clasico := append(moderno, v.base(true)...)
		m2, sat2, err := v.decidir(ctx, clasico)
		if err != nil {
			return ResultadoSilogismo{}, err
		}
		if !sat2 {
			res.Valido = true
			res.Pasos = append(res.Pasos, "Solo se sigue si suponemos que existe al menos un ejemplar de cada término (como en la lógica clásica).")
		} else {
			res.Contraejemplo = v.describirModelo(m2)
			res.Pasos = append(res.Pasos, "Sí existe: "+res.Contraejemplo+". Ahí las premisas son verdad y la conclusión es falsa.")
		}
	}
	if res.Valido {
		res.Nombre = nombreModo(prem, *concl)
		if res.Nombre != "" {
			res.Pasos = append(res.Pasos, "Es un silogismo en "+res.Nombre+".")
		}
	}
	return res, nil
}

// nombreModo gives the classic name of a two-premise syllogism (or "" when it has none).
func nombreModo(prem []Categorica, concl Categorica) string {
	if len(prem) != 2 {
		return ""
	}
	s, p := claveTermino(concl.Sujeto), claveTermino(concl.Predicado)
	tiene := func(c Categorica, t string) bool {
		return claveTermino(c.Sujeto) == t || claveTermino(c.Predicado) == t
	}
	var mayor, menor Categorica
	switch {
	case tiene(prem[0], p) && tiene(prem[1], s) && !tiene(prem[0], s):
		mayor, menor = prem[0], prem[1]
	case tiene(prem[1], p) && tiene(prem[0], s) && !tiene(prem[1], s):
		mayor, menor = prem[1], prem[0]
	default:
		return ""
	}
	var m string
	if claveTermino(mayor.Sujeto) == p {
		m = claveTermino(mayor.Predicado)
	} else {
		m = claveTermino(mayor.Sujeto)
	}
	if !tiene(menor, m) || m == s || m == p {
		return ""
	}
	mSujMayor := claveTermino(mayor.Sujeto) == m
	mSujMenor := claveTermino(menor.Sujeto) == m
	var figura string
	switch {
	case mSujMayor && !mSujMenor:
		figura = "1"
	case !mSujMayor && !mSujMenor:
		figura = "2"
	case mSujMayor && mSujMenor:
		figura = "3"
	default:
		figura = "4"
	}
	clave := figura + mayor.Letra() + menor.Letra() + concl.Letra()
	nombre := modosValidos[clave]
	if nombre != "" && (mayor.Cuant == Individuo || menor.Cuant == Individuo || concl.Cuant == Individuo) {
		nombre += " (con un individuo)"
	}
	return nombre
}

// conclusiones enumerates the four forms over ordered pairs of terms and keeps those that follow
// (classical reading) but do not follow from a single premise, dropping weaker or repeated ones.
func (v *venn) conclusiones(ctx context.Context, prem []Categorica, cnfPrem [][]int) ([]string, error) {
	type cand struct {
		c     Categorica
		texto string
	}
	sigue := func(base [][]int, c Categorica) (bool, error) {
		cnf := append(append(append([][]int(nil), base...), v.clausulas(c, true)...), v.base(true)...)
		_, sat, err := v.decidir(ctx, cnf)
		return !sat, err
	}
	var buenas []Categorica
	for a := 0; a < v.k; a++ {
		for b := 0; b < v.k; b++ {
			if a == b || v.individuos[b] {
				continue
			}
			formas := []Categorica{{Cuant: Todos}, {Cuant: Ningun}, {Cuant: Algunos}, {Cuant: AlgunosNo}}
			if v.individuos[a] {
				formas = []Categorica{{Cuant: Individuo}, {Cuant: Individuo, Negada: true}}
			}
			for _, f := range formas {
				f.Sujeto, f.Predicado = v.lemas[a], v.lemas[b]
				f.FraseSujeto, f.FrasePredicado = v.nombres[a], v.nombres[b]
				ok, err := sigue(cnfPrem, f)
				if err != nil {
					return nil, err
				}
				if !ok {
					continue
				}
				trivial := false
				for _, p := range prem {
					t, err := sigue(v.clausulas(p, false), f)
					if err != nil {
						return nil, err
					}
					if t {
						trivial = true
						break
					}
				}
				if !trivial {
					buenas = append(buenas, f)
				}
			}
		}
	}
	mismo := func(x, y Categorica, cu Cuantificador, cruzado bool) bool {
		if y.Cuant != cu {
			return false
		}
		if cruzado {
			return x.Sujeto == y.Predicado && x.Predicado == y.Sujeto
		}
		return x.Sujeto == y.Sujeto && x.Predicado == y.Predicado
	}
	var out []string
	for i, c := range buenas {
		redundante := false
		for j, d := range buenas {
			if i == j {
				continue
			}
			switch c.Cuant {
			case Algunos:
				redundante = redundante || mismo(c, d, Todos, false) || mismo(c, d, Todos, true) || (j < i && mismo(c, d, Algunos, true))
			case AlgunosNo:
				redundante = redundante || mismo(c, d, Ningun, false) || mismo(c, d, Ningun, true)
			case Ningun:
				redundante = redundante || (j < i && mismo(c, d, Ningun, true))
			}
		}
		if !redundante {
			out = append(out, c.String())
		}
	}
	return out, nil
}

// errNoCategorica marks text that is not a list of categorical sentences.
var errNoCategorica = errors.New("logica: no es un silogismo")
