package recetas

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// indice is a TF-IDF index over small documents of lemmas.
type indice struct {
	docs []map[string]float64 // term → weight (1+log tf)·idf, normalized to length 1
	idf  map[string]float64
}

// vacias are words that carry no meaning for retrieval.
var vacias = map[string]bool{
	"de": true, "la": true, "el": true, "los": true, "las": true, "un": true, "una": true, "y": true, "o": true,
	"a": true, "en": true, "con": true, "por": true, "para": true, "que": true, "se": true, "es": true,
	"del": true, "al": true, "lo": true, "como": true, "su": true, "sus": true, "mi": true, "me": true,
	"haz": true, "hacer": true, "quiero": true, "dame": true, "funcion": true, "codigo": true, "go": true,
}

// raiz is a crude Spanish stem: normalized, without the plural ending.
func raiz(w string) string {
	w = nucleo.Normalizar(w)
	switch {
	case len(w) > 4 && strings.HasSuffix(w, "ces"):
		return w[:len(w)-3] + "z" // luces → luz
	case len(w) > 4 && strings.HasSuffix(w, "es") && !strings.ContainsRune("aeiou", rune(w[len(w)-3])):
		return w[:len(w)-2] // pares → par, vocales → vocal
	case len(w) > 3 && strings.HasSuffix(w, "s"):
		return w[:len(w)-1]
	}
	return w
}

// terminos turns words (lemmas, concepts, identifiers) into stems; concepts like "suma_digitos" and
// CamelCase names are split too.
func terminos(ws []string) []string {
	var out []string
	for _, w := range ws {
		for _, parte := range partirPalabra(w) {
			if parte == "" || vacias[parte] {
				continue
			}
			out = append(out, raiz(parte))
		}
	}
	return out
}

// partirPalabra splits "suma_digitos" → [suma digitos], "EsPrimo" → [es primo], "strings.Split" →
// [strings split], and keeps the whole lowercased word too when it was split.
func partirPalabra(w string) []string {
	w = strings.TrimSpace(w)
	if w == "" {
		return nil
	}
	var partes []string
	var actual []rune
	cortar := func() {
		if len(actual) > 0 {
			partes = append(partes, strings.ToLower(string(actual)))
			actual = actual[:0]
		}
	}
	rs := []rune(w)
	for i, r := range rs {
		switch {
		case r == '_' || r == '.' || r == '/' || r == '-' || unicode.IsSpace(r):
			cortar()
		case unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1]))):
			cortar()
			actual = append(actual, r)
		default:
			actual = append(actual, r)
		}
	}
	cortar()
	if len(partes) > 1 {
		partes = append(partes, strings.ToLower(strings.NewReplacer("_", "", ".", "", "/", "", "-", "").Replace(w)))
	}
	return partes
}

func nuevoIndice(docs [][]string) *indice {
	ix := &indice{idf: map[string]float64{}}
	df := map[string]int{}
	tfs := make([]map[string]int, len(docs))
	for i, d := range docs {
		tf := map[string]int{}
		for _, t := range terminos(d) {
			tf[t]++
		}
		tfs[i] = tf
		for t := range tf {
			df[t]++
		}
	}
	n := float64(len(docs))
	for t, k := range df {
		ix.idf[t] = math.Log((n+1)/(float64(k)+1)) + 1
	}
	for _, tf := range tfs {
		v := map[string]float64{}
		ts := make([]string, 0, len(tf))
		for t := range tf {
			ts = append(ts, t)
		}
		sort.Strings(ts)
		norma := 0.0
		for _, t := range ts {
			w := (1 + math.Log(float64(tf[t]))) * ix.idf[t]
			v[t] = w
			norma += w * w
		}
		norma = math.Sqrt(norma)
		for t := range v {
			v[t] /= norma
		}
		ix.docs = append(ix.docs, v)
	}
	return ix
}

// puntuar returns the cosine similarity of the query with every document.
func (ix *indice) puntuar(consulta []string) []float64 {
	out := make([]float64, len(ix.docs))
	q := map[string]float64{}
	var ts []string // sorted, so that the sums (and ties) are the same on every call
	for _, t := range terminos(consulta) {
		if idf, ok := ix.idf[t]; ok {
			if _, ya := q[t]; !ya {
				ts = append(ts, t)
			}
			q[t] = idf
		}
	}
	sort.Strings(ts)
	norma := 0.0
	for _, t := range ts {
		norma += q[t] * q[t]
	}
	if norma == 0 {
		return out
	}
	norma = math.Sqrt(norma)
	for i, d := range ix.docs {
		s := 0.0
		for _, t := range ts {
			s += q[t] * d[t]
		}
		out[i] = s / norma
	}
	return out
}

func indiceFunciones(rs []Receta) *indice {
	docs := make([][]string, len(rs))
	for i, r := range rs {
		docs[i] = append(append(append([]string(nil), r.Palabras...), r.Conceptos...), r.Nombre)
	}
	return nuevoIndice(docs)
}

func indicePlantillas(ps []Plantilla) *indice {
	docs := make([][]string, len(ps))
	for i, p := range ps {
		docs[i] = append(append([]string(nil), p.Palabras...), p.Nombre)
	}
	return nuevoIndice(docs)
}

func indiceNotas(ns []Nota, alias [][]string) *indice {
	docs := make([][]string, len(ns))
	for i, n := range ns {
		docs[i] = append(append([]string(nil), n.Palabras...), n.Clave)
		if i < len(alias) {
			docs[i] = append(docs[i], alias[i]...)
		}
	}
	return nuevoIndice(docs)
}

type puesto struct {
	i      int
	puntos float64
}

func mejores(puntos []float64, k int, valido func(i int) bool) []puesto {
	var ps []puesto
	for i, p := range puntos {
		if p > 0 && (valido == nil || valido(i)) {
			ps = append(ps, puesto{i, p})
		}
	}
	sort.SliceStable(ps, func(a, b int) bool { return ps[a].puntos > ps[b].puntos })
	if k > 0 && len(ps) > k {
		ps = ps[:k]
	}
	return ps
}

// BuscarFunciones ranks the recipes by TF-IDF cosine over Palabras+Conceptos (and the name). When f is
// not nil, recipes whose Forma() differs from f.Forma() score 0 and are left out. k ≤ 0 means no limit.
// The recipes returned are copies.
func BuscarFunciones(lemas []string, f *nucleo.Firma, k int) []Puntuada {
	c := cargado()
	puntos := c.idxFunciones.puntuar(lemas)
	var forma string
	if f != nil {
		forma = f.Forma()
	}
	var out []Puntuada
	for _, p := range mejores(puntos, k, func(i int) bool { return f == nil || c.funciones[i].Firma.Forma() == forma }) {
		r := copiarReceta(c.funciones[p.i])
		out = append(out, Puntuada{Receta: &r, Puntos: p.puntos})
	}
	return out
}

// BuscarPlantillas ranks the program templates by TF-IDF cosine over their Palabras (and name).
func BuscarPlantillas(lemas []string, k int) []PuntuadaPlantilla {
	c := cargado()
	var out []PuntuadaPlantilla
	for _, p := range mejores(c.idxPlantillas.puntuar(lemas), k, nil) {
		pl := copiarPlantilla(c.plantillas[p.i])
		out = append(out, PuntuadaPlantilla{Plantilla: &pl, Puntos: p.puntos})
	}
	return out
}

// BuscarNotas returns up to k notes (k ≤ 0: no limit): the note for simbolo first, when there is one
// ("strings.Split", "defer"; also "math/rand.Intn", "Defer", "closure", "heap.Push" through the notes'
// aliases), then the best TF-IDF matches for lemas and the symbol's words.
func BuscarNotas(lemas []string, simbolo string, k int) []Nota {
	c := cargado()
	var out []Nota
	usado := map[int]bool{}
	if i := c.notaExacta(simbolo); i >= 0 {
		out = append(out, c.notas[i])
		usado[i] = true
	}
	consulta := append([]string(nil), lemas...)
	if simbolo != "" {
		consulta = append(consulta, simbolo)
	}
	for _, p := range mejores(c.idxNotas.puntuar(consulta), 0, func(i int) bool { return !usado[i] }) {
		if k > 0 && len(out) >= k {
			break
		}
		out = append(out, c.notas[p.i])
	}
	if k > 0 && len(out) > k {
		out = out[:k]
	}
	for i := range out {
		out[i].Relacionados = append([]string(nil), out[i].Relacionados...)
		out[i].Palabras = append([]string(nil), out[i].Palabras...)
	}
	return out
}

// notaExacta finds the note for a symbol or concept: exact Clave, then the part after the last "/"
// ("math/rand.Intn" → "rand.Intn"), then the same ignoring case and accents, then the aliases, then the
// singular of a plain word ("goroutines"). It returns -1 when there is none.
func (c *contenido) notaExacta(simbolo string) int {
	s := strings.TrimSuffix(strings.TrimSpace(simbolo), "()")
	if s == "" {
		return -1
	}
	cands := []string{s}
	if i := strings.LastIndex(s, "/"); i >= 0 && i+1 < len(s) && strings.Contains(s[i+1:], ".") {
		cands = append(cands, s[i+1:])
	}
	for _, x := range cands {
		for i, n := range c.notas {
			if n.Clave == x {
				return i
			}
		}
	}
	for _, x := range cands {
		nx := nucleo.Normalizar(x)
		for i, n := range c.notas {
			if nucleo.Normalizar(n.Clave) == nx {
				return i
			}
		}
		if i, ok := c.alias[nx]; ok {
			return i
		}
	}
	if !strings.ContainsAny(s, " ./") {
		rs := raiz(s)
		for i, n := range c.notas {
			if !strings.ContainsAny(n.Clave, " ./") && raiz(n.Clave) == rs {
				return i
			}
		}
	}
	return -1
}
