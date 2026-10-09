package memoria

import (
	"math"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// palabrasVacias are Spanish function words left out of text search.
var palabrasVacias = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "un": true, "una": true, "unos": true, "unas": true,
	"de": true, "del": true, "a": true, "al": true, "y": true, "o": true, "e": true, "u": true, "en": true,
	"que": true, "es": true, "por": true, "para": true, "con": true, "se": true, "su": true, "sus": true,
	"lo": true, "le": true, "les": true, "me": true, "mi": true, "te": true, "tu": true, "si": true,
	"como": true, "cual": true, "this": true, "the": true, "of": true, "and": true, "to": true,
}

// tokens normalizes s (nucleo.Normalizar) and splits it into words of letters and digits, without stop words.
func tokens(s string) []string {
	campos := strings.FieldsFunc(nucleo.Normalizar(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	out := campos[:0]
	for _, c := range campos {
		if !palabrasVacias[c] {
			out = append(out, c)
		}
	}
	return out
}

// tokensNombre splits an identifier like SumaPares or suma_pares into its lowercase words.
func tokensNombre(nombre string) []string {
	var partes []string
	var actual []rune
	cerrar := func() {
		if len(actual) > 0 {
			partes = append(partes, nucleo.Normalizar(string(actual)))
			actual = actual[:0]
		}
	}
	rs := []rune(nombre)
	for i, r := range rs {
		switch {
		case r == '_' || !(unicode.IsLetter(r) || unicode.IsDigit(r)):
			cerrar()
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))):
			cerrar()
			actual = append(actual, r)
		default:
			actual = append(actual, r)
		}
	}
	cerrar()
	return partes
}

// normalizarLista normalizes words and drops empty ones and repeats (order kept).
func normalizarLista(xs []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, x := range xs {
		x = strings.TrimSpace(nucleo.Normalizar(x))
		if x == "" || visto[x] {
			continue
		}
		visto[x] = true
		out = append(out, x)
	}
	return out
}

// jaccard is |A∩B| / |A∪B| over normalized sets (0 when both are empty).
func jaccard(a, b []string) float64 {
	sa := map[string]bool{}
	for _, x := range normalizarLista(a) {
		sa[x] = true
	}
	sb := map[string]bool{}
	for _, x := range normalizarLista(b) {
		sb[x] = true
	}
	if len(sa) == 0 && len(sb) == 0 {
		return 0
	}
	comun := 0
	for x := range sa {
		if sb[x] {
			comun++
		}
	}
	return float64(comun) / float64(len(sa)+len(sb)-comun)
}

// tfidf scores a query against documents by cosine similarity of TF-IDF vectors (0..1).
// idf = ln((N+1)/(df+0.5)), so a word present in every document still counts a little.
func tfidf(consulta []string, docs [][]string) []float64 {
	n := len(docs)
	df := map[string]int{}
	for _, d := range docs {
		visto := map[string]bool{}
		for _, w := range d {
			if !visto[w] {
				visto[w] = true
				df[w]++
			}
		}
	}
	idf := func(w string) float64 { return math.Log((float64(n) + 1) / (float64(df[w]) + 0.5)) }
	vector := func(ws []string) map[string]float64 {
		v := map[string]float64{}
		for _, w := range ws {
			v[w]++
		}
		for w, tf := range v {
			v[w] = tf * idf(w)
		}
		return v
	}
	norma := func(v map[string]float64) float64 {
		s := 0.0
		for _, x := range v {
			s += x * x
		}
		return math.Sqrt(s)
	}
	q := vector(consulta)
	nq := norma(q)
	out := make([]float64, n)
	if nq == 0 {
		return out
	}
	for i, d := range docs {
		v := vector(d)
		nv := norma(v)
		if nv == 0 {
			continue
		}
		dot := 0.0
		for w, x := range q {
			dot += x * v[w]
		}
		out[i] = math.Min(1, dot/(nq*nv))
	}
	return out
}

// bm25 scores a query against documents (k1 = 1.2, b = 0.75).
func bm25(consulta []string, docs [][]string) []float64 {
	const k1, b = 1.2, 0.75
	n := len(docs)
	out := make([]float64, n)
	if n == 0 || len(consulta) == 0 {
		return out
	}
	df := map[string]int{}
	total := 0
	for _, d := range docs {
		total += len(d)
		visto := map[string]bool{}
		for _, w := range d {
			if !visto[w] {
				visto[w] = true
				df[w]++
			}
		}
	}
	medio := float64(total) / float64(n)
	if medio == 0 {
		medio = 1
	}
	q := normalizarLista(consulta)
	for i, d := range docs {
		tf := map[string]int{}
		for _, w := range d {
			tf[w]++
		}
		s := 0.0
		for _, w := range q {
			f := float64(tf[w])
			if f == 0 {
				continue
			}
			idf := math.Log(1 + (float64(n)-float64(df[w])+0.5)/(float64(df[w])+0.5))
			s += idf * f * (k1 + 1) / (f + k1*(1-b+b*float64(len(d))/medio))
		}
		out[i] = s
	}
	return out
}
