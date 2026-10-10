// Package lengua is the Spanish understanding of Nyx Código: normalization, tokens, lemmas, numbers
// written in words, code and example extraction, frames, signatures and names, intent scores,
// follow-ups, English web queries and the embedded lexicon (§4.8).
//
// It imports only the standard library and internal/nucleo.
package lengua

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

//go:embed lexico.json
var lexicoJSON []byte

type entrada struct {
	Concepto string `json:"c"`  // a nucleo.Conceptos entry, or "" (English only)
	Ingles   string `json:"en"` // English word(s) for web queries
}

type archivoLexico struct {
	Version  int                `json:"version"`
	Palabras map[string]entrada `json:"palabras"`
}

// Lexico maps Spanish lemmas to concepts (nucleo.Conceptos) and to English words. It is the embedded
// base lexicon (lexico.json) plus an overlay of learned words. It is safe for concurrent use.
type Lexico struct {
	base *archivoLexico // shared, read-only

	mu        sync.RWMutex
	aprendido map[string]string // lemma → concept
}

var (
	baseUnaVez sync.Once
	baseLex    *archivoLexico
)

func cargarBase() *archivoLexico {
	baseUnaVez.Do(func() {
		var a archivoLexico
		if err := json.Unmarshal(lexicoJSON, &a); err != nil {
			panic("lengua: lexico.json no es válido: " + err.Error())
		}
		norm := make(map[string]entrada, len(a.Palabras))
		for p, e := range a.Palabras {
			norm[nucleo.Normalizar(p)] = e
		}
		a.Palabras = norm
		baseLex = &a
	})
	return baseLex
}

// LexicoBase returns a new lexicon with the embedded entries and no learned words.
func LexicoBase() *Lexico {
	return &Lexico{base: cargarBase(), aprendido: map[string]string{}}
}

// ConAprendidas returns a copy of l with the learned words m (word → concept, as memoria stores them)
// added on top. Words are lemmatized; concepts outside nucleo.Conceptos are ignored.
func (l *Lexico) ConAprendidas(m map[string]string) *Lexico {
	n := &Lexico{base: cargarBase(), aprendido: map[string]string{}}
	if l != nil {
		l.mu.RLock()
		for k, v := range l.aprendido {
			n.aprendido[k] = v
		}
		l.mu.RUnlock()
	}
	for p, c := range m {
		n.Aprender(p, c)
	}
	return n
}

// Aprender adds a learned word: "capicúa" → "palindromo". Both the written form and its lemma are
// stored. A concept outside nucleo.Conceptos is ignored.
func (l *Lexico) Aprender(palabra, concepto string) {
	if l == nil {
		return
	}
	c := strings.ReplaceAll(strings.TrimSpace(nucleo.Normalizar(concepto)), " ", "_")
	if !nucleo.EsConcepto(c) {
		if lc := Lema(c); nucleo.EsConcepto(lc) {
			c = lc
		} else if e, ok := cargarBase().Palabras[lc]; ok && e.Concepto != "" {
			c = e.Concepto
		} else {
			return
		}
	}
	p := strings.TrimSpace(nucleo.Normalizar(palabra))
	if p == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.aprendido == nil {
		l.aprendido = map[string]string{}
	}
	l.aprendido[p] = c
	l.aprendido[Lema(p)] = c
}

// Concepto returns the concept of a lemma (learned words first).
func (l *Lexico) Concepto(lema string) (string, bool) {
	k := nucleo.Normalizar(lema)
	if l != nil {
		l.mu.RLock()
		c, ok := l.aprendido[k]
		l.mu.RUnlock()
		if ok {
			return c, true
		}
	}
	e, ok := cargarBase().Palabras[k]
	if !ok || e.Concepto == "" {
		return "", false
	}
	return e.Concepto, true
}

// Ingles returns the English translation of a lemma. A learned word is translated through its concept.
func (l *Lexico) Ingles(lema string) (string, bool) {
	k := nucleo.Normalizar(lema)
	b := cargarBase()
	if e, ok := b.Palabras[k]; ok && e.Ingles != "" {
		return e.Ingles, true
	}
	if l != nil {
		l.mu.RLock()
		c, ok := l.aprendido[k]
		l.mu.RUnlock()
		if ok {
			if e, ok := b.Palabras[c]; ok && e.Ingles != "" {
				return e.Ingles, true
			}
			return strings.ReplaceAll(c, "_", " "), true
		}
	}
	return "", false
}

// conoce reports whether the lemma is in the lexicon (base or learned).
func (l *Lexico) conoce(lema string) bool {
	k := nucleo.Normalizar(lema)
	if _, ok := cargarBase().Palabras[k]; ok {
		return true
	}
	if l != nil {
		l.mu.RLock()
		_, ok := l.aprendido[k]
		l.mu.RUnlock()
		return ok
	}
	return false
}

// Lemas returns the base lexicon's lemmas, sorted (for tests and the UI).
func (l *Lexico) Lemas() []string {
	b := cargarBase()
	out := make([]string, 0, len(b.Palabras))
	for k := range b.Palabras {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// esLemaBase reports whether w is a lemma of the embedded lexicon.
func esLemaBase(w string) bool {
	_, ok := cargarBase().Palabras[w]
	return ok
}

// PalabrasDesconocidas returns the lemmas of word tokens that the lexicon does not know and that are
// not function words or common verbs, without repeats, in order. cerebro proposes them as new words.
func PalabrasDesconocidas(tokens []nucleo.Token, l *Lexico) []string {
	var out []string
	visto := map[string]bool{}
	for _, t := range tokens {
		if t.Clase != "palabra" {
			continue
		}
		lema := t.Lema
		if lema == "" {
			lema = Lema(t.Texto)
		}
		if lema == "" || visto[lema] || len([]rune(lema)) < 3 {
			continue
		}
		if funcionales[lema] || funcionales[t.Norma] || verbosConocidos[lema] || l.conoce(lema) || comunes[lema] {
			continue
		}
		if strings.ContainsAny(lema, "._0123456789") {
			continue
		}
		visto[lema] = true
		out = append(out, lema)
	}
	return out
}
