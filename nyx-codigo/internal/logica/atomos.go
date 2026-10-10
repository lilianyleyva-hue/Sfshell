package logica

import (
	"fmt"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Atomos maps atom indices to the Spanish phrases they stand for.
type Atomos struct {
	Frases []string // atom index → canonical phrase ("el suelo se moja")
	Claves []string // lemma-bag key: sorted word roots joined by spaces
}

// vaciasAtomo are dropped from a clause's lemma bag: determiners, "se", "que", auxiliaries.
var vaciasAtomo = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "lo": true, "un": true, "una": true, "unos": true,
	"unas": true, "se": true, "que": true, "es": true, "son": true, "esta": true, "estan": true, "ha": true,
	"han": true, "he": true, "has": true, "hemos": true, "fue": true, "fueron": true, "era": true, "eran": true,
	"sera": true, "seran": true, "sea": true, "sean": true, "no": true, "nunca": true, "al": true, "del": true,
}

// bolsaPalabras computes the lemma bag of some words (already split).
func bolsaPalabras(palabras []string, lem nucleo.Lematizador) []string {
	vistos := map[string]bool{}
	var out []string
	for _, w := range palabras {
		n := nucleo.Normalizar(w)
		if n == "" || vaciasAtomo[n] {
			continue
		}
		r := raizTermino(lematizar(w, lem))
		if r == "" || vistos[r] {
			continue
		}
		vistos[r] = true
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func bolsaToks(ts []tok, lem nucleo.Lematizador) []string {
	var ws []string
	for _, t := range ts {
		if !t.sim {
			ws = append(ws, t.orig)
		}
	}
	return bolsaPalabras(ws, lem)
}

// bolsaFrase tokenizes a phrase and computes its lemma bag.
func bolsaFrase(frase string, lem nucleo.Lematizador) []string {
	return bolsaToks(tokenizar(frase), lem)
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	en := map[string]bool{}
	for _, x := range a {
		en[x] = true
	}
	comun := 0
	union := len(a)
	for _, y := range b {
		if en[y] {
			comun++
		} else {
			union++
		}
	}
	return float64(comun) / float64(union)
}

// incluye reports whether every element of sub is in sup.
func incluye(sup, sub []string) bool {
	en := map[string]bool{}
	for _, x := range sup {
		en[x] = true
	}
	for _, y := range sub {
		if !en[y] {
			return false
		}
	}
	return true
}

// Buscar looks for the atom that the phrase names: first an exact key match, then a unique atom with
// Jaccard ≥ 0.6, then a unique atom whose bag contains the phrase's bag ("se moja" ⊆ "el suelo se
// moja"). Non-exact matches return a Spanish note ("Entiendo «se moja» como «el suelo se moja»").
// It reports false when no atom matches; the phrase is not added (see Obtener).
func (a *Atomos) Buscar(frase string, lem nucleo.Lematizador) (int, string, bool) {
	return a.buscarBolsa(bolsaFrase(frase, lem), strings.TrimSpace(frase))
}

func (a *Atomos) buscarBolsa(bolsa []string, frase string) (int, string, bool) {
	if a == nil {
		return -1, "", false
	}
	clave := strings.Join(bolsa, " ")
	for i, c := range a.Claves {
		if c == clave {
			return i, "", true
		}
	}
	if len(bolsa) == 0 {
		return -1, "", false
	}
	nota := func(i int) string {
		return fmt.Sprintf("Entiendo «%s» como «%s»", frase, a.Frases[i])
	}
	cand, n := -1, 0
	for i, c := range a.Claves {
		if jaccard(bolsa, strings.Fields(c)) >= 0.6 {
			cand, n = i, n+1
		}
	}
	if n == 1 {
		return cand, nota(cand), true
	}
	cand, n = -1, 0
	for i, c := range a.Claves {
		if incluye(strings.Fields(c), bolsa) {
			cand, n = i, n+1
		}
	}
	if n == 1 {
		return cand, nota(cand), true
	}
	return -1, "", false
}

// agregar appends a new atom.
func (a *Atomos) agregar(frase string, bolsa []string) int {
	a.Frases = append(a.Frases, frase)
	a.Claves = append(a.Claves, strings.Join(bolsa, " "))
	return len(a.Frases) - 1
}

// Obtener returns the atom for the phrase, adding a new one when Buscar finds none. The note is
// non-empty for non-exact matches.
func (a *Atomos) Obtener(frase string, lem nucleo.Lematizador) (int, string) {
	b := bolsaFrase(frase, lem)
	if i, nota, ok := a.buscarBolsa(b, strings.TrimSpace(frase)); ok {
		return i, nota
	}
	return a.agregar(strings.TrimSpace(frase), b), ""
}

// Nombres gives every atom a short name for tables and proofs: atoms written as single letters keep
// their letter; the others get p, q, r… (skipping letters already taken).
func (a *Atomos) Nombres(n int) []string {
	out := make([]string, n)
	usadas := map[string]bool{}
	if a != nil {
		for i := 0; i < n && i < len(a.Frases); i++ {
			if esLetra(a.Frases[i]) {
				out[i] = a.Frases[i]
				usadas[strings.ToLower(a.Frases[i])] = true
			}
		}
	}
	sig := 0
	for i := range out {
		if out[i] != "" {
			continue
		}
		for {
			nombre := NombreAtomo(sig)
			sig++
			if !usadas[nombre] {
				out[i] = nombre
				usadas[nombre] = true
				break
			}
		}
	}
	return out
}

// Leyenda is "p = «llueve», q = «el suelo se moja»" for the atoms used (empty if all are letters).
func (a *Atomos) Leyenda(usados []int, nombres []string) string {
	if a == nil {
		return ""
	}
	var partes []string
	for _, i := range usados {
		if i < len(a.Frases) && i < len(nombres) && !esLetra(a.Frases[i]) {
			partes = append(partes, nombres[i]+" = «"+a.Frases[i]+"»")
		}
	}
	return strings.Join(partes, ", ")
}

func esLetra(s string) bool {
	return len(s) == 1 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}
