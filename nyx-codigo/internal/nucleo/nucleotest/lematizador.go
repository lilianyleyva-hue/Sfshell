package nucleotest

import (
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// numerosEnPalabras are the number words LematizadorSimple understands ("un"/"una" are left as words).
var numerosEnPalabras = map[string]int64{
	"cero": 0, "uno": 1, "dos": 2, "tres": 3, "cuatro": 4, "cinco": 5, "seis": 6, "siete": 7,
	"ocho": 8, "nueve": 9, "diez": 10, "once": 11, "doce": 12, "veinte": 20, "cien": 100, "mil": 1000,
}

// sinLema are short words left untouched by the crude suffix rules.
var sinLema = map[string]bool{
	"es": true, "son": true, "mas": true, "menos": true, "pues": true, "tres": true, "seis": true,
	"dos": true, "los": true, "las": true, "les": true, "nos": true, "sus": true, "tus": true, "mis": true,
	"este": true, "ese": true, "que": true, "de": true, "se": true, "le": true, "la": true, "una": true,
	"entre": true, "sobre": true, "siempre": true, "nunca": true, "nadie": true, "alguna": true, "ninguna": true,
	"toda": true, "todas": true, "todos": true, "algunas": true, "algunos": true, "ningunos": true, "si": true, "no": true,
	"entonces": true, "tambien": true, "luego": true, "cada": true, "nada": true, "otra": true, "otras": true,
}

// LematizadorSimple tokenizes for leaf tests: nucleo.Normalizar, split on spaces/punctuation, strip
// plural -s/-es, verb endings -a/-e → -ar/-er (crude). Digits ("12", "3,5", "3.5") and the number
// words cero…doce, veinte, cien, mil become "numero" tokens with Num; text in quotes ("…", «…», “…”)
// becomes one "cita" token (Texto without the quotes); "[…]" becomes one "lista" token; any other
// punctuation is a one-character "simbolo" token. Desde/Hasta are byte offsets in texto.
func LematizadorSimple(texto string) []nucleo.Token {
	var out []nucleo.Token
	i := 0
	for i < len(texto) {
		r, n := utf8.DecodeRuneInString(texto[i:])
		switch {
		case unicode.IsSpace(r):
			i += n
		case unicode.IsDigit(r):
			j := i + n
			for j < len(texto) {
				c := texto[j]
				if c >= '0' && c <= '9' {
					j++
					continue
				}
				if (c == '.' || c == ',') && j+1 < len(texto) && texto[j+1] >= '0' && texto[j+1] <= '9' {
					j++
					continue
				}
				break
			}
			s := texto[i:j]
			num, ok := new(big.Rat).SetString(strings.Replace(s, ",", ".", 1))
			if !ok {
				num = nil
			}
			out = append(out, nucleo.Token{Texto: s, Norma: s, Lema: s, Clase: "numero", Num: num, Desde: i, Hasta: j})
			i = j
		case unicode.IsLetter(r) || r == '_':
			j := i + n
			for j < len(texto) {
				r2, n2 := utf8.DecodeRuneInString(texto[j:])
				if !unicode.IsLetter(r2) && !unicode.IsDigit(r2) && r2 != '_' {
					break
				}
				j += n2
			}
			s := texto[i:j]
			norma := nucleo.Normalizar(s)
			t := nucleo.Token{Texto: s, Norma: norma, Lema: lemaSimple(norma), Clase: "palabra", Desde: i, Hasta: j}
			if v, ok := numerosEnPalabras[norma]; ok {
				t.Clase, t.Lema, t.Num = "numero", norma, big.NewRat(v, 1)
			}
			out = append(out, t)
			i = j
		case r == '"' || r == '«' || r == '“':
			cierre := map[rune]rune{'"': '"', '«': '»', '“': '”'}[r]
			k := strings.IndexRune(texto[i+n:], cierre)
			if k < 0 {
				out = append(out, simbolo(texto, i, n))
				i += n
				continue
			}
			ini, fin := i+n, i+n+k
			s := texto[ini:fin]
			norma := nucleo.Normalizar(s)
			out = append(out, nucleo.Token{Texto: s, Norma: norma, Lema: norma, Clase: "cita", Desde: ini, Hasta: fin})
			i = fin + utf8.RuneLen(cierre)
		case r == '[':
			prof, j := 0, i
			for ; j < len(texto); j++ {
				if texto[j] == '[' {
					prof++
				} else if texto[j] == ']' {
					prof--
					if prof == 0 {
						break
					}
				}
			}
			if j >= len(texto) {
				out = append(out, simbolo(texto, i, n))
				i += n
				continue
			}
			s := texto[i : j+1]
			out = append(out, nucleo.Token{Texto: s, Norma: nucleo.Normalizar(s), Lema: nucleo.Normalizar(s), Clase: "lista", Desde: i, Hasta: j + 1})
			i = j + 1
		default:
			out = append(out, simbolo(texto, i, n))
			i += n
		}
	}
	return out
}

func simbolo(texto string, i, n int) nucleo.Token {
	s := texto[i : i+n]
	norma := nucleo.Normalizar(s)
	if norma == "" { // ¿ and ¡ normalize to nothing
		norma = s
	}
	return nucleo.Token{Texto: s, Norma: norma, Lema: norma, Clase: "simbolo", Desde: i, Hasta: i + n}
}

// lemaSimple applies the crude rules: plural -es/-s, then (only if no plural was removed)
// -a → -ar and -e → -er for words of 4+ letters.
func lemaSimple(w string) string {
	if sinLema[w] || utf8.RuneCountInString(w) < 3 {
		return w
	}
	n := utf8.RuneCountInString(w)
	switch {
	case strings.HasSuffix(w, "es") && n > 4 && !strings.ContainsRune("aeiou", rune(w[len(w)-3])):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "s") && n > 3:
		return w[:len(w)-1]
	}
	if n >= 4 && (strings.HasSuffix(w, "a") || strings.HasSuffix(w, "e")) {
		return w + "r"
	}
	return w
}
