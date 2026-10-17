package habla

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"math/rand"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The language model: an interpolated Kneser-Ney trigram model over words (lowercase, with accents),
// with "," ";" ":" as tokens and sentence boundaries <s> … </s>. Only the trigram counts are stored;
// bigram and unigram statistics are derived from them (with two <s> of padding every bigram and
// unigram occurrence is inside a trigram). The tables are bounded: past MaxTrigramas the lowest counts
// are pruned, and the vocabulary stops growing at MaxVocabulario (new words become <?>).

// Bounds of the model.
const (
	MaxTrigramas   = 250_000
	MaxVocabulario = 60_000
	MaxReservadas  = 200 // held-out sentences for the fluency measure
	cadaReservada  = 10  // one sentence in ten (of the ones that may be held out) is held out
	maxPalabraLM   = 40  // longer tokens become <?>
)

// Special token ids.
const (
	idInicio int32 = iota // <s>
	idFin                 // </s>
	idRaro                // <?>: code, symbols, words beyond the vocabulary
	idNum                 // <num>
	numEspeciales
)

type tri3 = [3]int32

type entrada struct {
	a, b, c int32
	n       uint32
}

type sumaDistintos struct{ suma, distintos uint32 }

// modelo is not safe for concurrent use: Habla's lock protects it.
type modelo struct {
	vocab      []string
	ids        map[string]int32
	mayus      map[int32]int32 // id → (times seen capitalized mid-sentence) − (times seen lowercase)
	tri        map[tri3]uint32
	reservadas []string
	contador   int      // sentences that could be held out, seen so far
	huellas    []uint64 // Bloom filter of the 4-grams read: tells a sentence glued from pieces
	nHuellas   int

	listo   bool // the derived tables below match tri
	porCtx  []entrada
	porFin  []entrada // sorted by (c, b, a)
	ctx2    map[[2]int32]sumaDistintos
	cont2   map[[2]int32]uint32
	suc2    []entrada // (b, c, cont2) sorted by (b, c)
	ctx1    map[int32]sumaDistintos
	cont1   []uint32
	total1  uint64
	d3, d2  float64
	bigrams int
}

func nuevoModelo() *modelo {
	m := &modelo{ids: map[string]int32{}, mayus: map[int32]int32{}, tri: map[tri3]uint32{}}
	for _, s := range []string{"<s>", "</s>", "<?>", "<num>"} {
		m.ids[s] = int32(len(m.vocab))
		m.vocab = append(m.vocab, s)
	}
	return m
}

// id returns the id of a token, adding it while the vocabulary has room.
func (m *modelo) id(t string) int32 {
	if i, ok := m.ids[t]; ok {
		return i
	}
	if len(m.vocab) >= MaxVocabulario || largo(t) > maxPalabraLM {
		return idRaro
	}
	i := int32(len(m.vocab))
	m.ids[t] = i
	m.vocab = append(m.vocab, t)
	return i
}

// tokensLM turns sentence tokens into model tokens: lowercase words, "," ";" ":", <num>, <?>.
// It also returns, for each, whether it was written capitalized away from the sentence start.
func tokensLM(ts []token) (out []string, mayus []int8) {
	for _, t := range ts {
		switch t.clase {
		case 'w':
			out = append(out, strings.ToLower(t.texto))
			m := int8(-1)
			if !t.inicio {
				if r, _ := utf8.DecodeRuneInString(t.texto); unicode.IsUpper(r) {
					m = 1
				}
			} else {
				m = 0
			}
			mayus = append(mayus, m)
		case 'n':
			out = append(out, "<num>")
			mayus = append(mayus, 0)
		case 'p':
			out = append(out, t.texto)
			mayus = append(mayus, 0)
		default:
			out = append(out, "<?>")
			mayus = append(mayus, 0)
		}
	}
	return out, mayus
}

// entrenar adds one sentence (model tokens) to the counts.
func (m *modelo) entrenar(toks []string, mayus []int8) {
	if len(toks) == 0 {
		return
	}
	seq := make([]int32, 0, len(toks)+3)
	seq = append(seq, idInicio, idInicio)
	for i, t := range toks {
		id := m.id(t)
		seq = append(seq, id)
		if id >= numEspeciales && i < len(mayus) && mayus[i] != 0 {
			m.mayus[id] += int32(mayus[i])
			if m.mayus[id] == 0 {
				delete(m.mayus, id)
			}
		}
	}
	seq = append(seq, idFin)
	for i := 2; i < len(seq); i++ {
		m.tri[tri3{seq[i-2], seq[i-1], seq[i]}]++
		if i >= 3 {
			m.anotarCuatro(seq[i-3], seq[i-2], seq[i-1], seq[i])
		}
	}
	m.listo = false
	if len(m.tri) > MaxTrigramas {
		m.podar(MaxTrigramas * 9 / 10)
	}
}

// Bloom filter of 4-grams: 2^22 bits (512 KiB), two probes per 4-gram, hashed over the words (ids
// change when the file is compacted). Cleared after maxHuellas insertions so it never saturates.
const (
	bitsHuellas = 1 << 22
	maxHuellas  = 1_000_000
)

func (m *modelo) hashCuatro(a, b, c, d int32) uint64 {
	h := uint64(14695981039346656037)
	for _, id := range [4]int32{a, b, c, d} {
		for _, x := range []byte(m.vocab[id]) {
			h ^= uint64(x)
			h *= 1099511628211
		}
		h ^= 0xff
		h *= 1099511628211
	}
	return h
}

func (m *modelo) anotarCuatro(a, b, c, d int32) {
	if m.huellas == nil || m.nHuellas >= maxHuellas {
		m.huellas = make([]uint64, bitsHuellas/64)
		m.nHuellas = 0
	}
	h := m.hashCuatro(a, b, c, d)
	for _, x := range [2]uint64{h & (bitsHuellas - 1), (h >> 32) & (bitsHuellas - 1)} {
		m.huellas[x/64] |= 1 << (x % 64)
	}
	m.nHuellas++
}

// leidoCuatro: the 4-gram was (probably) read.
func (m *modelo) leidoCuatro(a, b, c, d int32) bool {
	if m.huellas == nil {
		return false
	}
	h := m.hashCuatro(a, b, c, d)
	for _, x := range [2]uint64{h & (bitsHuellas - 1), (h >> 32) & (bitsHuellas - 1)} {
		if m.huellas[x/64]&(1<<(x%64)) == 0 {
			return false
		}
	}
	return true
}

// podar removes the trigrams with the lowest counts until at most objetivo remain.
func (m *modelo) podar(objetivo int) {
	if len(m.tri) <= objetivo {
		return
	}
	const tope = 64
	var hist [tope + 1]int
	for _, n := range m.tri {
		if n > tope {
			n = tope
		}
		hist[n]++
	}
	sobran := len(m.tri) - objetivo
	umbral, quitadas := uint32(0), 0
	for c := 1; c <= tope; c++ {
		if quitadas+hist[c] >= sobran {
			umbral = uint32(c)
			break
		}
		quitadas += hist[c]
	}
	// every count below umbral goes; then counts equal to umbral until enough are gone
	resto := sobran - quitadas
	for k, n := range m.tri {
		switch {
		case n < umbral:
			delete(m.tri, k)
		case n == umbral && resto > 0:
			delete(m.tri, k)
			resto--
		}
	}
	m.listo = false
}

// reconstruir derives the Kneser-Ney tables from the trigram counts. O(n log n).
func (m *modelo) reconstruir() {
	if m.listo {
		return
	}
	n := len(m.tri)
	m.porCtx = make([]entrada, 0, n)
	for k, c := range m.tri {
		m.porCtx = append(m.porCtx, entrada{k[0], k[1], k[2], c})
	}
	sort.Slice(m.porCtx, func(i, j int) bool {
		x, y := m.porCtx[i], m.porCtx[j]
		if x.a != y.a {
			return x.a < y.a
		}
		if x.b != y.b {
			return x.b < y.b
		}
		return x.c < y.c
	})
	m.porFin = append(make([]entrada, 0, n), m.porCtx...)
	sort.Slice(m.porFin, func(i, j int) bool {
		x, y := m.porFin[i], m.porFin[j]
		if x.c != y.c {
			return x.c < y.c
		}
		if x.b != y.b {
			return x.b < y.b
		}
		return x.a < y.a
	})
	m.ctx2 = make(map[[2]int32]sumaDistintos)
	var n1, n2 float64
	for _, e := range m.porCtx {
		s := m.ctx2[[2]int32{e.a, e.b}]
		s.suma += e.n
		s.distintos++
		m.ctx2[[2]int32{e.a, e.b}] = s
		if e.n == 1 {
			n1++
		} else if e.n == 2 {
			n2++
		}
	}
	m.d3 = descuento(n1, n2)
	m.cont2 = make(map[[2]int32]uint32)
	for _, e := range m.porFin {
		m.cont2[[2]int32{e.b, e.c}]++
	}
	m.bigrams = len(m.cont2)
	m.suc2 = make([]entrada, 0, len(m.cont2))
	n1, n2 = 0, 0
	for k, c := range m.cont2 {
		m.suc2 = append(m.suc2, entrada{a: k[0], b: k[1], n: c})
		if c == 1 {
			n1++
		} else if c == 2 {
			n2++
		}
	}
	m.d2 = descuento(n1, n2)
	sort.Slice(m.suc2, func(i, j int) bool {
		if m.suc2[i].a != m.suc2[j].a {
			return m.suc2[i].a < m.suc2[j].a
		}
		return m.suc2[i].b < m.suc2[j].b
	})
	m.ctx1 = make(map[int32]sumaDistintos)
	m.cont1 = make([]uint32, len(m.vocab))
	m.total1 = 0
	for _, e := range m.suc2 {
		s := m.ctx1[e.a]
		s.suma += e.n
		s.distintos++
		m.ctx1[e.a] = s
		if int(e.b) < len(m.cont1) {
			m.cont1[e.b]++
			m.total1++
		}
	}
	m.listo = true
}

// descuento is the Kneser-Ney discount n1/(n1+2·n2), kept in [0.3, 0.9].
func descuento(n1, n2 float64) float64 {
	if n1+n2 == 0 {
		return 0.75
	}
	d := n1 / (n1 + 2*n2)
	return math.Max(0.3, math.Min(0.9, d))
}

// p1 is the continuation unigram probability with add-one smoothing over the vocabulary (no <s>).
func (m *modelo) p1(c int32) float64 {
	v := float64(len(m.vocab) - 1)
	n := 0.0
	if int(c) < len(m.cont1) {
		n = float64(m.cont1[c])
	}
	return (n + 1) / (float64(m.total1) + v)
}

func (m *modelo) p2(c, b int32) float64 {
	s := m.ctx1[b]
	if s.suma == 0 {
		return m.p1(c)
	}
	n := float64(m.cont2[[2]int32{b, c}])
	t := float64(s.suma)
	return math.Max(n-m.d2, 0)/t + m.d2*float64(s.distintos)/t*m.p1(c)
}

// p is P_KN(c | a b).
func (m *modelo) p(c, a, b int32) float64 {
	s := m.ctx2[[2]int32{a, b}]
	if s.suma == 0 {
		return m.p2(c, b)
	}
	n := float64(m.tri[tri3{a, b, c}])
	t := float64(s.suma)
	return math.Max(n-m.d3, 0)/t + m.d3*float64(s.distintos)/t*m.p2(c, b)
}

// logProb is the natural-log probability of a sentence of ids (without padding; </s> is added) and the
// number of predicted tokens.
func (m *modelo) logProb(ids []int32) (float64, int) {
	m.reconstruir()
	a, b := idInicio, idInicio
	lp := 0.0
	for _, c := range append(append([]int32(nil), ids...), idFin) {
		lp += math.Log(m.p(c, a, b))
		a, b = b, c
	}
	return lp, len(ids) + 1
}

// idsDe maps model tokens to ids without adding words (unknown → <?>).
func (m *modelo) idsDe(toks []string) []int32 {
	out := make([]int32, len(toks))
	for i, t := range toks {
		if id, ok := m.ids[t]; ok {
			out[i] = id
		} else {
			out[i] = idRaro
		}
	}
	return out
}

// fluidez is the average log-probability per token of the held-out sentences (ok=false without any).
func (m *modelo) fluidez() (float64, bool) {
	if len(m.reservadas) == 0 || len(m.tri) == 0 {
		return 0, false
	}
	total, n := 0.0, 0
	for _, f := range m.reservadas {
		toks, _ := tokensLM(tokenizar(f))
		lp, k := m.logProb(m.idsDe(toks))
		total += lp
		n += k
	}
	if n == 0 {
		return 0, false
	}
	return total / float64(n), true
}

// rango returns [i, j) of the entries of s (sorted by the key) whose key equals k.
func rango(s []entrada, menor func(e entrada) int) (int, int) {
	i := sort.Search(len(s), func(x int) bool { return menor(s[x]) >= 0 })
	j := sort.Search(len(s), func(x int) bool { return menor(s[x]) > 0 })
	return i, j
}

func cmp32(x, y int32) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// sucesores3 are the trigrams (a, b, ·).
func (m *modelo) sucesores3(a, b int32) []entrada {
	i, j := rango(m.porCtx, func(e entrada) int {
		if c := cmp32(e.a, a); c != 0 {
			return c
		}
		return cmp32(e.b, b)
	})
	return m.porCtx[i:j]
}

// sucesores2 are the continuation bigrams (b, ·): entrada{a: b, b: c, n: N1+(• b c)}.
func (m *modelo) sucesores2(b int32) []entrada {
	i, j := rango(m.suc2, func(e entrada) int { return cmp32(e.a, b) })
	return m.suc2[i:j]
}

// antecesores are the trigrams (·, b, c).
func (m *modelo) antecesores(b, c int32) []entrada {
	i, j := rango(m.porFin, func(e entrada) int {
		if x := cmp32(e.c, c); x != 0 {
			return x
		}
		return cmp32(e.b, b)
	})
	return m.porFin[i:j]
}

// terminanEn are the trigrams (·, ·, c).
func (m *modelo) terminanEn(c int32) []entrada {
	i, j := rango(m.porFin, func(e entrada) int { return cmp32(e.c, c) })
	return m.porFin[i:j]
}

// muestreador samples continuations restricted to the allowed tokens.
type muestreador struct {
	m        *modelo
	permitir []bool
}

func (m *modelo) nuevoMuestreador(permitir []bool) *muestreador {
	m.reconstruir()
	return &muestreador{m: m, permitir: permitir}
}

func (s *muestreador) ok(c int32) bool { return int(c) < len(s.permitir) && s.permitir[c] }

// siguiente samples the token after (z, a, b) along what was read: usually (fiel) a word that
// continues a real 4-gram, else one of the trigrams (a, b, ·) by their counts, or (one time in ten, or
// when there is none) one of the bigrams (b, ·) by their continuation counts. It never jumps to an
// unrelated word: with nothing allowed after (a, b) it fails.
func (s *muestreador) siguiente(rng *rand.Rand, z, a, b int32) (int32, bool) {
	m := s.m
	tri := m.sucesores3(a, b)
	if rng.Float64() < fiel {
		if e, ok := elegirPorCuenta(rng, tri, func(e entrada) bool { return s.ok(e.c) && m.leidoCuatro(z, a, b, e.c) }); ok {
			return e.c, true
		}
	}
	if rng.Float64() >= 0.1 {
		if e, ok := elegirPorCuenta(rng, tri, func(e entrada) bool { return s.ok(e.c) }); ok {
			return e.c, true
		}
	}
	if e, ok := elegirPorCuenta(rng, m.sucesores2(b), func(e entrada) bool { return s.ok(e.b) }); ok {
		return e.b, true
	}
	if e, ok := elegirPorCuenta(rng, tri, func(e entrada) bool { return s.ok(e.c) }); ok {
		return e.c, true
	}
	return 0, false
}

// fiel is how often a step follows a 4-gram that was read (the rest joins pieces: its own words).
const fiel = 0.85

// elegirPorCuenta picks one entry with probability proportional to its count among those accepted.
func elegirPorCuenta(rng *rand.Rand, es []entrada, acepta func(e entrada) bool) (entrada, bool) {
	total := 0.0
	for _, e := range es {
		if acepta(e) {
			total += float64(e.n)
		}
	}
	if total == 0 {
		return entrada{}, false
	}
	u := rng.Float64() * total
	for _, e := range es {
		if !acepta(e) {
			continue
		}
		u -= float64(e.n)
		if u <= 0 {
			return e, true
		}
	}
	for i := len(es) - 1; i >= 0; i-- {
		if acepta(es[i]) {
			return es[i], true
		}
	}
	return entrada{}, false
}

// reservar decides whether a sentence is held out (not trained) for the fluency measure.
func (m *modelo) reservar(frase string) bool {
	m.contador++
	if m.contador%cadaReservada != 0 {
		return false
	}
	m.reservadas = append(m.reservadas, frase)
	if len(m.reservadas) > MaxReservadas {
		m.reservadas = append([]string(nil), m.reservadas[len(m.reservadas)-MaxReservadas:]...)
	}
	return true
}

// ---- persistence ----

type archivoModelo struct {
	Vocabulario []string `json:"vocabulario"`
	Trigramas   []uint32 `json:"trigramas"` // a, b, c, count, a, b, c, count… (ids into Vocabulario)
	Mayusculas  []string `json:"mayusculas,omitempty"`
	Reservadas  []string `json:"reservadas,omitempty"`
	Contador    int      `json:"contador"`
	Huellas     string   `json:"huellas,omitempty"` // base64 of the 4-gram Bloom filter (little endian)
	NHuellas    int      `json:"nHuellas,omitempty"`
}

// exportar writes a compact copy: only the words still used by some trigram, renumbered.
func (m *modelo) exportar() archivoModelo {
	usado := make([]bool, len(m.vocab))
	for k := range m.tri {
		usado[k[0]], usado[k[1]], usado[k[2]] = true, true, true
	}
	nuevo := make([]int32, len(m.vocab))
	var voc []string
	for i, w := range m.vocab {
		if i < int(numEspeciales) || usado[i] {
			nuevo[i] = int32(len(voc))
			voc = append(voc, w)
		}
	}
	claves := make([]tri3, 0, len(m.tri))
	for k := range m.tri {
		claves = append(claves, k)
	}
	sort.Slice(claves, func(i, j int) bool {
		x, y := claves[i], claves[j]
		if x[0] != y[0] {
			return x[0] < y[0]
		}
		if x[1] != y[1] {
			return x[1] < y[1]
		}
		return x[2] < y[2]
	})
	tri := make([]uint32, 0, 4*len(claves))
	for _, k := range claves {
		tri = append(tri, uint32(nuevo[k[0]]), uint32(nuevo[k[1]]), uint32(nuevo[k[2]]), m.tri[k])
	}
	var may []string
	for id, v := range m.mayus {
		if v > 0 && int(id) < len(m.vocab) && usado[id] {
			may = append(may, m.vocab[id])
		}
	}
	sort.Strings(may)
	a := archivoModelo{Vocabulario: voc, Trigramas: tri, Mayusculas: may,
		Reservadas: append([]string(nil), m.reservadas...), Contador: m.contador}
	if m.huellas != nil {
		b := make([]byte, 8*len(m.huellas))
		for i, w := range m.huellas {
			binary.LittleEndian.PutUint64(b[8*i:], w)
		}
		a.Huellas, a.NHuellas = base64.StdEncoding.EncodeToString(b), m.nHuellas
	}
	return a
}

// importar rebuilds a model from its file form; ok=false when it is inconsistent.
func importar(a archivoModelo) (*modelo, bool) {
	m := nuevoModelo()
	if len(a.Vocabulario) < int(numEspeciales) || len(a.Trigramas)%4 != 0 || len(a.Vocabulario) > MaxVocabulario {
		return m, len(a.Vocabulario) == 0 && len(a.Trigramas) == 0
	}
	for i := 0; i < int(numEspeciales); i++ {
		if a.Vocabulario[i] != m.vocab[i] {
			return nuevoModelo(), false
		}
	}
	for _, w := range a.Vocabulario[numEspeciales:] {
		if _, ya := m.ids[w]; ya || w == "" {
			return nuevoModelo(), false
		}
		m.ids[w] = int32(len(m.vocab))
		m.vocab = append(m.vocab, w)
	}
	v := uint32(len(m.vocab))
	for i := 0; i+3 < len(a.Trigramas); i += 4 {
		x, y, z, n := a.Trigramas[i], a.Trigramas[i+1], a.Trigramas[i+2], a.Trigramas[i+3]
		if x >= v || y >= v || z >= v || n == 0 {
			return nuevoModelo(), false
		}
		m.tri[tri3{int32(x), int32(y), int32(z)}] += n
	}
	for _, w := range a.Mayusculas {
		if id, ok := m.ids[w]; ok {
			m.mayus[id] = 1
		}
	}
	for _, f := range a.Reservadas {
		if len(m.reservadas) < MaxReservadas && largo(f) <= maxFraseRunas {
			m.reservadas = append(m.reservadas, f)
		}
	}
	m.contador = a.Contador
	if a.Huellas != "" {
		if b, err := base64.StdEncoding.DecodeString(a.Huellas); err == nil && len(b) == bitsHuellas/8 {
			m.huellas = make([]uint64, bitsHuellas/64)
			for i := range m.huellas {
				m.huellas[i] = binary.LittleEndian.Uint64(b[8*i:])
			}
			m.nHuellas = a.NHuellas
		}
	}
	if len(m.tri) > MaxTrigramas {
		m.podar(MaxTrigramas * 9 / 10)
	}
	return m, true
}

// forma is how a token is shown: "go" → "Go" when it is usually written capitalized.
func (m *modelo) forma(id int32) string {
	w := m.vocab[id]
	if m.mayus[id] > 0 {
		r := []rune(w)
		if len(r) > 0 {
			return strings.ToUpper(string(r[0])) + string(r[1:])
		}
	}
	return w
}
