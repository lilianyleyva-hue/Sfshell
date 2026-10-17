package habla

import (
	"math/rand"
	"sort"
	"strings"
)

// Frase is a sentence the model built.
type Frase struct {
	Texto    string   // "las funciones sirven para no repetir código."
	Palabras []string // its words, as shown
	Puntos   float64  // log-probability per token + topic coverage − penalties
	Ancladas []string // content words it only knows anchored (no definition of their own)
	Probadas int      // candidates built
	Validas  int      // candidates that only use words whose meaning it knows
	Motivo   string   // when ok=false: "sin_tema" | "sin_lectura" | "sin_significado" | "sin_frase"
}

// Attempts of Decir.
const (
	intentosTema   = 200 // starting at the topic word and going left to a sentence start
	intentosInicio = 40  // starting at a sentence start
	maxIzquierda   = 12  // words added to the left of the topic word
	MaxPalabrasDir = 25
	margenFrase    = 0.3 // candidates this close to the best may be chosen instead
)

// Decir builds a sentence about the topic words: it starts from a topic word (going left through
// contexts that lead to it, up to a sentence start) or from a sentence start, samples the next words
// from the language model, tries several candidates and rejects every candidate with a content word
// whose meaning it does not know (function words are allowed). The best one by log-probability per
// word plus topic coverage wins. ok=false (with Motivo) when it cannot build one honestly.
func (h *Habla) Decir(rng *rand.Rand, tema []string, max int) (Frase, bool) {
	var f Frase
	if h == nil {
		f.Motivo = "sin_lectura"
		return f, false
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	if max <= 0 || max > MaxPalabrasDir {
		max = MaxPalabrasDir
	}
	temas := map[string]bool{}
	for _, t := range tema {
		for _, w := range strings.Fields(t) {
			w = plegar(w)
			if w != "" && !esVacia(w) && largo(w) >= 2 {
				temas[tallo(w)] = true
			}
		}
	}
	if len(temas) == 0 {
		f.Motivo = "sin_tema"
		return f, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.lm
	m.reconstruir()
	permitir := h.permitidos()
	var objetivos []int32
	conocidos := 0
	for i := int(numEspeciales); i < len(m.vocab); i++ {
		if temas[tallo(m.vocab[i])] {
			objetivos = append(objetivos, int32(i))
			if permitir[i] {
				conocidos++
			}
		}
	}
	switch {
	case len(objetivos) == 0:
		f.Motivo = "sin_lectura"
		return f, false
	case conocidos == 0:
		f.Motivo = "sin_significado"
		return f, false
	}
	// the meaning of the topic: sentences whose words belong to it are preferred
	sentidoTema := map[string]float64{}
	for _, t := range tema {
		for _, w := range strings.Fields(t) {
			if v, _ := h.vector(w); v != nil {
				for d, x := range v {
					if !strings.HasPrefix(d, "@") {
						sentidoTema[d] += x
					}
				}
			}
		}
	}
	mu := m.nuevoMuestreador(permitir)
	ok := func(id int32) bool { return int(id) < len(permitir) && permitir[id] }
	type candidata struct {
		ids    []int32
		puntos float64
		texto  string
	}
	var validas []candidata
	vistas := map[string]bool{}
	evaluar := func(ids []int32) {
		f.Probadas++
		c, bien := h.puntuar(ids, temas, sentidoTema, max, permitir)
		if !bien {
			return
		}
		texto := h.formatear(ids)
		if vistas[texto] {
			return
		}
		vistas[texto] = true
		f.Validas++
		validas = append(validas, candidata{ids: ids, puntos: c, texto: texto})
	}
	adelante := func(ids []int32) []int32 {
		z, a, b := idInicio, idInicio, idInicio
		switch n := len(ids); {
		case n >= 3:
			z, a, b = ids[n-3], ids[n-2], ids[n-1]
		case n == 2:
			a, b = ids[0], ids[1]
		case n == 1:
			b = ids[0]
		}
		for palabras(ids) <= max {
			c, bien := mu.siguiente(rng, z, a, b)
			if !bien {
				return nil
			}
			if c == idFin {
				return ids
			}
			ids = append(ids, c)
			z, a, b = a, b, c
		}
		return nil
	}
	for n := 0; n < intentosTema; n++ {
		t := objetivos[rng.Intn(len(objetivos))]
		if !ok(t) {
			continue
		}
		e, bien := elegirPorCuenta(rng, m.terminanEn(t), func(e entrada) bool {
			return (e.a == idInicio || ok(e.a)) && (e.b == idInicio || ok(e.b)) && !(e.a != idInicio && e.b == idInicio)
		})
		if !bien {
			continue
		}
		var izq []int32
		switch {
		case e.a == idInicio && e.b == idInicio:
		case e.a == idInicio:
			izq = []int32{e.b}
		default:
			izq = []int32{e.a, e.b}
			llego := false
			for paso := 0; paso < maxIzquierda; paso++ {
				sig := t // the word after izq[1]: prefer an x that makes a 4-gram that was read
				if len(izq) > 2 {
					sig = izq[2]
				}
				valida := func(e entrada) bool { return e.a == idInicio || ok(e.a) }
				x, bien := entrada{}, false
				if rng.Float64() < fiel {
					x, bien = elegirPorCuenta(rng, m.antecesores(izq[0], izq[1]), func(e entrada) bool {
						return valida(e) && m.leidoCuatro(e.a, izq[0], izq[1], sig)
					})
				}
				if !bien {
					x, bien = elegirPorCuenta(rng, m.antecesores(izq[0], izq[1]), valida)
				}
				if !bien {
					break
				}
				if x.a == idInicio {
					llego = true
					break
				}
				izq = append([]int32{x.a}, izq...)
			}
			if !llego {
				continue
			}
		}
		if ids := adelante(append(izq, t)); ids != nil {
			evaluar(ids)
		}
	}
	for n := 0; n < intentosInicio; n++ {
		if ids := adelante(nil); ids != nil {
			evaluar(ids)
		}
	}
	if len(validas) == 0 {
		f.Motivo = "sin_frase"
		return f, false
	}
	// the best one, or another almost as good (so "otra frase" can say something else)
	sort.Slice(validas, func(i, j int) bool {
		if validas[i].puntos != validas[j].puntos {
			return validas[i].puntos > validas[j].puntos
		}
		return validas[i].texto < validas[j].texto
	})
	n := 1
	for n < len(validas) && n < 4 && validas[n].puntos >= validas[0].puntos-margenFrase {
		n++
	}
	mejor := &validas[rng.Intn(n)]
	f.Texto, f.Puntos = mejor.texto, mejor.puntos
	visto := map[string]bool{}
	for _, id := range mejor.ids {
		w := m.vocab[id]
		if id < numEspeciales || w == "," || w == ";" || w == ":" {
			continue
		}
		f.Palabras = append(f.Palabras, m.forma(id))
		if esFuncion(plegar(w)) || visto[w] {
			continue
		}
		visto[w] = true
		if h.propio(w) == nil {
			f.Ancladas = append(f.Ancladas, w)
		}
	}
	return f, true
}

func esPuntuacion(w string) bool { return w == "," || w == ";" || w == ":" }

func palabras(ids []int32) int { return len(ids) }

// permitidos marks the vocabulary ids Decir may use: </s>, punctuation, function words and words whose
// meaning it knows (own or anchored). Cached until the dictionary or the vocabulary change.
func (h *Habla) permitidos() []bool {
	m := h.lm
	desde := int(numEspeciales)
	if h.genPermit == h.genSig && len(h.permitido) <= len(m.vocab) {
		if len(h.permitido) == len(m.vocab) {
			return h.permitido
		}
		desde = len(h.permitido) // only new words were read: the rest still holds
	}
	p := make([]bool, len(m.vocab))
	p[idFin] = true
	if desde > int(numEspeciales) {
		copy(p, h.permitido)
	}
	for i := desde; i < len(m.vocab); i++ {
		w := m.vocab[i]
		if esPuntuacion(w) {
			p[i] = true
			continue
		}
		pr, an := h.conoce(w)
		p[i] = pr || an
	}
	h.permitido, h.genPermit = p, h.genSig
	return p
}

// colgantes cannot end a sentence.
var colgantes = conjunto(`de del a al y e o u que en con para por el la los las un una unos unas lo sin sobre entre como pero si su sus mi mis`)

// puntuar scores a candidate (ids without padding); false when it must be rejected.
func (h *Habla) puntuar(ids []int32, temas map[string]bool, sentido map[string]float64, max int, permitir []bool) (float64, bool) {
	m := h.lm
	n, contenido, afines := 0, 0, 0.0
	cubiertos := map[string]bool{}
	repetidas := map[string]int{}
	alPrincipio := false
	for i, id := range ids {
		if id < numEspeciales || int(id) >= len(permitir) || !permitir[id] {
			return 0, false
		}
		w := m.vocab[id]
		if esPuntuacion(w) {
			if i == 0 || i == len(ids)-1 || esPuntuacion(m.vocab[ids[i-1]]) || (i == 1 && w == ":") {
				return 0, false // "defer: …" is a title, not a sentence
			}
			continue
		}
		n++
		st := tallo(w)
		if temas[st] {
			cubiertos[st] = true
			if n <= 3 {
				alPrincipio = true
			}
		}
		if !esFuncion(plegar(w)) {
			repetidas[st]++
		}
		if !esVacia(plegar(w)) && !temas[st] {
			contenido++
			afines += minimoF(1, sentido[st])
		}
	}
	if n < 2 || n > max || len(cubiertos) == 0 {
		return 0, false
	}
	lp, k := m.logProb(ids)
	// trigrams never read: a sentence glued from pieces is less likely to make sense
	// and 4-grams never read: each one is a place where pieces of different sentences were glued
	nuevos, empalmes := 0, 0
	z, a, b := idInicio, idInicio, idInicio
	for _, c := range append(append([]int32(nil), ids...), idFin) {
		if m.tri[tri3{a, b, c}] == 0 {
			nuevos++
		}
		if !m.leidoCuatro(z, a, b, c) {
			empalmes++
		}
		z, a, b = a, b, c
	}
	puntos := lp/float64(k) + 2*float64(len(cubiertos))/float64(len(temas)) - 1.5*float64(nuevos)/float64(k) -
		0.6*float64(empalmes)
	puntos += 0.04 * float64(minimo(n, 12)) // a little longer says more
	if alPrincipio {
		puntos += 0.5 // the sentence is about the topic, not just mentions it
	}
	if contenido > 0 {
		puntos += 0.7 * afines / float64(contenido) // its words belong to the meaning of the topic
	}
	if n < 4 {
		puntos -= 1.5
	}
	for _, r := range repetidas {
		if r >= 3 {
			puntos--
		}
	}
	if colgantes[plegar(m.vocab[ids[len(ids)-1]])] {
		puntos -= 2
	}
	return puntos, true
}

// formatear joins the tokens: punctuation sticks to the word before it, and a final period.
func (h *Habla) formatear(ids []int32) string {
	var sb strings.Builder
	for i, id := range ids {
		w := h.lm.forma(id)
		if i > 0 && !esPuntuacion(w) {
			sb.WriteByte(' ')
		}
		sb.WriteString(w)
	}
	return sb.String() + "."
}

func minimo(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minimoF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
