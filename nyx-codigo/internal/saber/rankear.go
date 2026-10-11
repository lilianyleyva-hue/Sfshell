package saber

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// BM25 passage ranking and fact extraction from fetched text. Web text is data: it is ranked and
// quoted ("Según <dominio>: …"), never followed as instructions.

// Pasaje is a ranked sentence with its source.
type Pasaje struct {
	Texto, URL string
	Puntos     float64
}

const (
	bm25K1      = 1.2
	bm25B       = 0.75
	minFrase    = 20 // runes
	minPalabras = 4  // a passage is a sentence, not a link or a path
	maxFrase    = 600
	pasajesStd  = 3
	confUnaVez  = 0.6
	confAcuerdo = 0.8
)

var vacias = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`el la los las un una unos unas de del al a en y o u que se su sus es son por para con sin
		como mas pero lo le les me te mi tu este esta estos estas ese esa eso hay ser fue era muy ya no si
		cual cuales donde cuando quien the a an of to in and or is are be it for on with as by this that
		at from how what which can you your use do does i`) {
		m[w] = true
	}
	return m
}()

// terminos tokenizes text for BM25: normalized (lowercase, no accents), letters and digits only,
// without stop words.
func terminos(s string) []string {
	s = nucleo.Normalizar(s)
	f := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' })
	out := f[:0]
	for _, w := range f {
		if len(w) < 2 && !(w >= "0" && w <= "9") {
			continue
		}
		if vacias[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

var reFinFrase = regexp.MustCompile(`([.!?;])\s+`)

// frases splits a paragraph into sentences.
func frases(p string) []string {
	p = strings.TrimSpace(p)
	if p == "" {
		return nil
	}
	marcado := reFinFrase.ReplaceAllString(p, "$1\x00")
	var out []string
	for _, s := range strings.Split(marcado, "\x00") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Rankear ranks the sentences of the documents' paragraphs against the query with BM25 (k1 = 1.2,
// b = 0.75), multiplied by the source weight of each document, and keeps the best n (3 when n ≤ 0).
func Rankear(consulta string, docs []Documento, n int) []Pasaje {
	if n <= 0 {
		n = pasajesStd
	}
	q := terminos(consulta)
	if len(q) == 0 {
		return nil
	}
	type frase struct {
		texto, url string
		peso       float64
		tf         map[string]int
		largo      int
	}
	var todas []frase
	df := map[string]int{}
	vistas := map[string]bool{}
	total := 0
	for _, d := range docs {
		peso := PesoDominio(d.URL)
		for _, b := range d.Bloques {
			if b.Tipo != "parrafo" {
				continue
			}
			for _, s := range frases(b.Texto) {
				if r := []rune(s); len(r) < minFrase || len(strings.Fields(s)) < minPalabras {
					continue
				} else if len(r) > maxFrase {
					s = string(r[:maxFrase]) + "…"
				}
				clave := strings.ToLower(s)
				if vistas[clave] {
					continue
				}
				vistas[clave] = true
				ts := terminos(s)
				if len(ts) == 0 {
					continue
				}
				tf := map[string]int{}
				for _, t := range ts {
					tf[t]++
				}
				for t := range tf {
					df[t]++
				}
				todas = append(todas, frase{texto: s, url: d.URL, peso: peso, tf: tf, largo: len(ts)})
				total += len(ts)
			}
		}
	}
	if len(todas) == 0 {
		return nil
	}
	N := float64(len(todas))
	media := float64(total) / N
	var out []Pasaje
	for _, f := range todas {
		puntos := 0.0
		for _, t := range q {
			tf := float64(f.tf[t])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (N-float64(df[t])+0.5)/(float64(df[t])+0.5))
			puntos += idf * tf * (bm25K1 + 1) / (tf + bm25K1*(1-bm25B+bm25B*float64(f.largo)/media))
		}
		if puntos <= 0 {
			continue
		}
		out = append(out, Pasaje{Texto: f.texto, URL: f.url, Puntos: puntos * f.peso})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Puntos > out[j].Puntos })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// ---- facts ----

var (
	reParentesis = regexp.MustCompile(`\s*\([^()]*\)`)
	reEsUn       = regexp.MustCompile(`(?i)^(.{1,60}?)\s+(?:es|era)\s+(?:un|una)\s+(.+)$`)
	reCreado     = regexp.MustCompile(`(?i)^(.{1,60}?)\s+(?:fue|ha sido)\s+(?:cread[oa]s?|diseñad[oa]s?|desarrollad[oa]s?|inventad[oa]s?)\s+por\s+(.+?)(?:\s+en\s+(?:el\s+año\s+)?(\d{4}))?\s*$`)
	reEs         = regexp.MustCompile(`(?i)^(.{1,60}?)\s+es\s+(.+)$`)
	articulos    = map[string]bool{"el": true, "la": true, "los": true, "las": true, "un": true, "una": true, "lo": true}
	cortesObjeto = map[string]bool{"que": true, "cuyo": true, "cuya": true, "creado": true, "creada": true,
		"desarrollado": true, "desarrollada": true, "diseñado": true, "diseñada": true, "con": true, "para": true,
		"basado": true, "basada": true, "y": true, "o": true, "el": true, "la": true, "en": true, "por": true,
		"inspirado": true, "inspirada": true, "usado": true, "usada": true, "como": true, "sobre": true}
)

// sujetoDe cleans the subject: no leading article, at most 4 words, lowercase.
func sujetoDe(s string) string {
	ws := strings.Fields(strings.Trim(s, " ,;:"))
	for len(ws) > 0 && articulos[strings.ToLower(ws[0])] {
		ws = ws[1:]
	}
	if len(ws) == 0 || len(ws) > 4 {
		return ""
	}
	for _, w := range ws {
		if strings.ContainsAny(w, ",;:") {
			return ""
		}
	}
	return strings.ToLower(strings.Join(ws, " "))
}

// frasNominal takes a noun with an optional "de X" complement: "lenguaje de programación concurrente…"
// → "lenguaje de programación".
func frasNominal(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ",.;:…"); i >= 0 {
		s = s[:i]
	}
	ws := strings.Fields(s)
	if len(ws) == 0 {
		return ""
	}
	out := []string{ws[0]}
	if cortesObjeto[strings.ToLower(ws[0])] {
		return ""
	}
	if len(ws) >= 3 {
		de := strings.ToLower(ws[1])
		if (de == "de" || de == "del") && !cortesObjeto[strings.ToLower(ws[2])] {
			out = append(out, ws[1], ws[2])
		}
	}
	return strings.ToLower(strings.Join(out, " "))
}

// frasCorta keeps up to n words, stopping at punctuation.
func frasCorta(s string, n int) string {
	if i := strings.IndexAny(s, ",.;:…"); i >= 0 {
		s = s[:i]
	}
	ws := strings.Fields(s)
	if len(ws) == 0 || len(ws) > n {
		return ""
	}
	return strings.ToLower(strings.Join(ws, " "))
}

func idHecho(h nucleo.Hecho) string {
	s := sha256.Sum256([]byte(h.Sujeto + "|" + h.Relacion + "|" + h.Objeto + "|" + h.Fuente))
	return "web-" + hex.EncodeToString(s[:6])
}

// ExtraerHechos finds simple facts in a document's paragraphs with patterns: "X es un/una Y …",
// "X fue creado por Y en AAAA" and "X (…) es …". With tema != "" only facts about it are kept.
// Each fact has confidence 0.6 (UnirHechos raises it to 0.8 when two sources agree).
func ExtraerHechos(d Documento, tema string) []nucleo.Hecho {
	temaN := nucleo.Normalizar(tema)
	var out []nucleo.Hecho
	vistos := map[string]bool{}
	agregar := func(sujeto, rel, objeto, texto string) {
		if sujeto == "" || objeto == "" {
			return
		}
		if temaN != "" {
			sn := nucleo.Normalizar(sujeto)
			if sn != temaN && !strings.HasPrefix(sn, temaN+" ") && !strings.HasSuffix(sn, " "+temaN) {
				return
			}
		}
		h := nucleo.Hecho{Sujeto: sujeto, Relacion: rel, Objeto: objeto, Texto: texto, Fuente: d.URL,
			Confianza: confUnaVez, Fecha: d.Fecha}
		k := sujeto + "|" + rel + "|" + objeto
		if vistos[k] {
			return
		}
		vistos[k] = true
		h.ID = idHecho(h)
		out = append(out, h)
	}
	for _, b := range d.Bloques {
		if b.Tipo != "parrafo" {
			continue
		}
		for _, f := range frases(b.Texto) {
			if len(f) > 400 {
				continue
			}
			limpia := strings.TrimSpace(reParentesis.ReplaceAllString(f, ""))
			limpia = strings.TrimRight(limpia, ".!?…")
			if m := reCreado.FindStringSubmatch(limpia); m != nil {
				suj := sujetoDe(m[1])
				autor := strings.TrimSpace(m[2])
				if len(autor) <= 120 {
					agregar(suj, "creado_por", strings.Trim(autor, " ,"), f)
				}
				if m[3] != "" {
					agregar(suj, "creado_en", m[3], f)
				}
				continue
			}
			if m := reEsUn.FindStringSubmatch(limpia); m != nil {
				agregar(sujetoDe(m[1]), "es_un", frasNominal(m[2]), f)
				continue
			}
			if m := reEs.FindStringSubmatch(limpia); m != nil && limpia != strings.TrimSpace(f) {
				// "X (…) es …": the parenthesis marks a definition
				agregar(sujetoDe(m[1]), "es", frasCorta(m[2], 4), f)
			}
		}
	}
	return out
}

// UnirHechos merges facts from several documents: the same (sujeto, relación, objeto) from two or more
// different sources gets confidence 0.8.
func UnirHechos(listas ...[]nucleo.Hecho) []nucleo.Hecho {
	type grupo struct {
		h       nucleo.Hecho
		fuentes map[string]bool
	}
	var orden []string
	grupos := map[string]*grupo{}
	for _, l := range listas {
		for _, h := range l {
			k := nucleo.Normalizar(h.Sujeto) + "|" + h.Relacion + "|" + nucleo.Normalizar(h.Objeto)
			g, ok := grupos[k]
			if !ok {
				g = &grupo{h: h, fuentes: map[string]bool{}}
				grupos[k] = g
				orden = append(orden, k)
			}
			g.fuentes[h.Fuente] = true
		}
	}
	out := make([]nucleo.Hecho, 0, len(orden))
	for _, k := range orden {
		g := grupos[k]
		h := g.h
		if len(g.fuentes) >= 2 {
			h.Confianza = confAcuerdo
		}
		out = append(out, h)
	}
	return out
}
