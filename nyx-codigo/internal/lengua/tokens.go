package lengua

import (
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Tokenizar splits s into tokens: words (Go identifiers such as strings.Split stay whole), numbers
// written with digits ("12", "3,5", "3.5", "1.000.000"), list literals "[…]" and quoted text ("…",
// «…», “…”, 'x') as single tokens, and one-character symbols. Lema is left empty (see Lematizar).
// Desde/Hasta are byte offsets in s; a quoted token's Texto excludes the quotes.
func Tokenizar(s string) []nucleo.Token {
	var out []nucleo.Token
	i := 0
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r):
			i += n
		case r >= '0' && r <= '9':
			j, v := leerDigitos(s, i)
			txt := s[i:j]
			out = append(out, nucleo.Token{Texto: txt, Norma: txt, Clase: "numero", Num: v, Desde: i, Hasta: j})
			i = j
		case unicode.IsLetter(r) || r == '_':
			j := finPalabra(s, i)
			txt := s[i:j]
			out = append(out, nucleo.Token{Texto: txt, Norma: nucleo.Normalizar(txt), Clase: "palabra", Desde: i, Hasta: j})
			i = j
		case r == '"' || r == '«' || r == '“' || r == '”':
			cierre := map[rune]string{'"': `"`, '«': "»", '“': "”", '”': "”"}[r]
			k := strings.Index(s[i+n:], cierre)
			if k < 0 && r == '“' {
				k = strings.Index(s[i+n:], `"`)
				cierre = `"`
			}
			if k < 0 {
				out = append(out, simbolo(s, i, n))
				i += n
				continue
			}
			ini, fin := i+n, i+n+k
			txt := s[ini:fin]
			out = append(out, nucleo.Token{Texto: txt, Norma: nucleo.Normalizar(txt), Clase: "cita", Desde: ini, Hasta: fin})
			i = fin + len(cierre)
		case r == '\'':
			// 'x' (one rune) is a quoted character; anything else is a symbol (apostrophe)
			if r2, n2 := utf8.DecodeRuneInString(s[i+n:]); i+n < len(s) && r2 != '\'' && strings.HasPrefix(s[i+n+n2:], "'") {
				ini, fin := i+n, i+n+n2
				out = append(out, nucleo.Token{Texto: s[ini:fin], Norma: nucleo.Normalizar(s[ini:fin]), Clase: "cita", Desde: ini, Hasta: fin})
				i = fin + 1
				continue
			}
			out = append(out, simbolo(s, i, n))
			i += n
		case r == '[':
			j := cierreCorchete(s, i)
			if j < 0 {
				out = append(out, simbolo(s, i, n))
				i += n
				continue
			}
			txt := s[i : j+1]
			out = append(out, nucleo.Token{Texto: txt, Norma: nucleo.Normalizar(txt), Clase: "lista", Desde: i, Hasta: j + 1})
			i = j + 1
		default:
			out = append(out, simbolo(s, i, n))
			i += n
		}
	}
	return out
}

func simbolo(s string, i, n int) nucleo.Token {
	txt := s[i : i+n]
	norma := nucleo.Normalizar(txt)
	if norma == "" {
		norma = txt
	}
	return nucleo.Token{Texto: txt, Norma: norma, Lema: norma, Clase: "simbolo", Desde: i, Hasta: i + n}
}

// finPalabra returns the end of the word starting at i. A dot between ASCII letters is kept, so
// "strings.Split" and "fmt.Println" are one token.
func finPalabra(s string, i int) int {
	j := i
	for j < len(s) {
		r, n := utf8.DecodeRuneInString(s[j:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			j += n
			continue
		}
		if r == '.' && j > i && j+1 < len(s) && esLetraASCII(s[j-1]) && esLetraASCII(s[j+1]) && esIdentASCII(s[i:j]) {
			j++
			continue
		}
		break
	}
	return j
}

func esLetraASCII(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func esIdentASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(esLetraASCII(c) || c >= '0' && c <= '9' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// cierreCorchete returns the index of the "]" closing the "[" at i, or -1.
func cierreCorchete(s string, i int) int {
	prof := 0
	enCita := byte(0)
	for j := i; j < len(s); j++ {
		c := s[j]
		if enCita != 0 {
			if c == '\\' {
				j++
				continue
			}
			if c == enCita {
				enCita = 0
			}
			continue
		}
		switch c {
		case '"':
			enCita = c
		case '[':
			prof++
		case ']':
			prof--
			if prof == 0 {
				return j
			}
		}
	}
	return -1
}

func esDigito(c byte) bool { return c >= '0' && c <= '9' }

// leerDigitos reads a number written with digits starting at i: "12", "3,5", "3.5", "1.000.000".
// A comma is a decimal comma only when it is not part of a comma-separated list ("1,2,3").
func leerDigitos(s string, i int) (int, *big.Rat) {
	j := i
	for j < len(s) && esDigito(s[j]) {
		j++
	}
	entero := s[i:j]
	// thousands "1.000.000" (two or more groups of three)
	if j+4 <= len(s) && s[j] == '.' && len(entero) <= 3 {
		k, grupos := j, 0
		for k+4 <= len(s) && s[k] == '.' && esDigito(s[k+1]) && esDigito(s[k+2]) && esDigito(s[k+3]) && (k+4 == len(s) || !esDigito(s[k+4])) {
			k += 4
			grupos++
		}
		if grupos >= 2 {
			v, _ := new(big.Rat).SetString(strings.ReplaceAll(s[i:k], ".", ""))
			return k, v
		}
	}
	if j+1 < len(s) && (s[j] == '.' || s[j] == ',') && esDigito(s[j+1]) {
		k := j + 1
		for k < len(s) && esDigito(s[k]) {
			k++
		}
		lista := s[j] == ',' && (k+1 < len(s) && s[k] == ',' && esDigito(s[k+1]) || i > 0 && s[i-1] == ',')
		if !lista {
			v, ok := new(big.Rat).SetString(entero + "." + s[j+1:k])
			if ok {
				return k, v
			}
		}
	}
	v, _ := new(big.Rat).SetString(entero)
	return j, v
}

// Lematizar tokenizes texto (Tokenizar), fills Lema, and joins numbers written in words into one
// "numero" token with Num ("dos mil trescientos cuarenta y cinco", "tres cuartos", "un tercio").
// It satisfies nucleo.Lematizador.
func Lematizar(texto string) []nucleo.Token {
	toks := Tokenizar(texto)
	out := make([]nucleo.Token, 0, len(toks))
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.Clase == "palabra" {
			if v, k := numeroEnPalabras(toks, i); k > 0 {
				ult := toks[i+k-1]
				txt := texto[t.Desde:ult.Hasta]
				out = append(out, nucleo.Token{Texto: txt, Norma: nucleo.Normalizar(txt), Lema: v.RatString(), Clase: "numero",
					Num: v, Desde: t.Desde, Hasta: ult.Hasta})
				i += k - 1
				continue
			}
			t.Lema = Lema(t.Texto)
		} else if t.Lema == "" {
			t.Lema = t.Norma
		}
		out = append(out, t)
	}
	return out
}

var _ nucleo.Lematizador = Lematizar

// Lema returns the dictionary form of a Spanish word, normalized (no accents, lowercase):
// sume/suma/sumando → sumar, pares → par, leones → leon, peces → pez, mamíferos → mamifero,
// llueve → llover. It uses an irregular table plus suffix rules checked against the known lemmas
// (lexicon, verbs, common words); unknown words get the plain plural rules.
func Lema(palabra string) string {
	w := nucleo.Normalizar(strings.TrimSpace(palabra))
	if w == "" || strings.ContainsAny(w, "._0123456789 ") {
		return w
	}
	if v, ok := irregulares[w]; ok {
		return v
	}
	if funcionales[w] || esConocido(w) {
		return w
	}
	if r := lemaNombre(w); r != "" {
		return r
	}
	if r := lemaVerbo(w); r != "" {
		return r
	}
	// enclitic pronouns: súmalos, pruébala, ordénalas
	for _, suf := range []string{"melo", "selo", "los", "las", "les", "lo", "la", "le", "me", "te", "nos"} {
		if strings.HasSuffix(w, suf) && utf8.RuneCountInString(w)-len(suf) >= 3 {
			base := w[:len(w)-len(suf)]
			if v, ok := irregulares[base]; ok {
				return v
			}
			if verbosConocidos[base] {
				return base
			}
			if r := lemaVerbo(base); r != "" {
				return r
			}
		}
	}
	return pluralPorDefecto(w)
}

func esConocido(w string) bool { return esLemaBase(w) || verbosConocidos[w] || comunes[w] }

// lemaNombre undoes plural and feminine endings when the result is a known lemma.
func lemaNombre(w string) string {
	for _, c := range candidatosNombre(w) {
		if esLemaBase(c) || comunes[c] {
			return c
		}
		if strings.HasSuffix(c, "a") {
			if m := c[:len(c)-1] + "o"; esLemaBase(m) || comunes[m] {
				return m
			}
		}
	}
	return ""
}

// candidatosNombre lists the singular forms w could come from, most likely first (w itself included).
func candidatosNombre(w string) []string {
	out := []string{w}
	n := len(w)
	if strings.HasSuffix(w, "ces") && n > 4 {
		out = append(out, w[:n-3]+"z", w[:n-1])
	}
	if strings.HasSuffix(w, "es") && n > 3 {
		out = append(out, w[:n-2], w[:n-1])
	} else if strings.HasSuffix(w, "s") && n > 3 {
		out = append(out, w[:n-1])
	}
	return out
}

// pluralPorDefecto: -ces → -z; -es after a consonant other than s → strip "es"; other -s → strip "s".
func pluralPorDefecto(w string) string {
	n := len(w)
	if n <= 3 || !strings.HasSuffix(w, "s") {
		return w
	}
	if strings.HasSuffix(w, "ces") && n > 4 {
		return w[:n-3] + "z"
	}
	if strings.HasSuffix(w, "es") && n > 4 {
		c := w[n-3]
		if !strings.ContainsRune("aeiousñ", rune(c)) {
			return w[:n-2]
		}
	}
	if c := w[n-2]; strings.ContainsRune("aeiou", rune(c)) {
		return w[:n-1]
	}
	return w
}

// sufijosVerbo are conjugation endings, longest first.
var sufijosVerbo = []string{
	"ariamos", "eriamos", "iriamos", "aremos", "eremos", "iremos", "abamos", "iendo", "yendo",
	"arian", "erian", "irian", "aron", "ieron", "aban", "aste", "iste", "ados", "adas", "idos", "idas",
	"aria", "eria", "iria", "aran", "eran", "iran", "ando", "amos", "emos", "imos", "aras", "eras",
	"ado", "ada", "ido", "ida", "aba", "ian", "ias", "ara", "era", "ira", "are", "ere", "ire", "ais", "eis",
	"ia", "io", "as", "es", "an", "en", "a", "e", "o",
}

// lemaVerbo returns the known infinitive a conjugated form comes from, or "".
// It undoes stem changes (ue→o, ie→e, i→e) and spelling changes (qu→c, gu→g, c→z, j→g, y→ø).
func lemaVerbo(w string) string {
	if verbosConocidos[w] {
		return w
	}
	for _, suf := range sufijosVerbo {
		if !strings.HasSuffix(w, suf) || len(w)-len(suf) < 1 {
			continue
		}
		raiz := w[:len(w)-len(suf)]
		for _, r := range variantesRaiz(raiz) {
			for _, inf := range []string{"ar", "er", "ir"} {
				if verbosConocidos[r+inf] {
					return r + inf
				}
			}
		}
	}
	return ""
}

func variantesRaiz(r string) []string {
	out := []string{r}
	add := func(s string) {
		for _, x := range out {
			if x == s {
				return
			}
		}
		out = append(out, s)
	}
	orto := func(s string) []string {
		v := []string{s}
		switch {
		case strings.HasSuffix(s, "qu"):
			v = append(v, s[:len(s)-2]+"c")
		case strings.HasSuffix(s, "gu"):
			v = append(v, s[:len(s)-1])
		case strings.HasSuffix(s, "c"):
			v = append(v, s[:len(s)-1]+"z")
		case strings.HasSuffix(s, "j"):
			v = append(v, s[:len(s)-1]+"g")
		case strings.HasSuffix(s, "y"):
			v = append(v, s[:len(s)-1])
		case strings.HasSuffix(s, "zc"):
			v = append(v, s[:len(s)-2]+"c")
		}
		return v
	}
	for _, a := range orto(r) {
		add(a)
		if k := strings.LastIndex(a, "ue"); k >= 0 {
			add(a[:k] + "o" + a[k+2:])
		}
		if k := strings.LastIndex(a, "ie"); k >= 0 {
			add(a[:k] + "e" + a[k+2:])
		}
		if k := strings.LastIndex(a, "i"); k >= 0 {
			add(a[:k] + "e" + a[k+1:])
		}
	}
	return out
}
