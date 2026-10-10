package lengua

import (
	"math/big"
	"strings"

	"nyxcodigo/internal/nucleo"
)

var valoresPalabra = map[string]int64{
	"cero": 0, "uno": 1, "un": 1, "una": 1, "dos": 2, "tres": 3, "cuatro": 4, "cinco": 5, "seis": 6, "siete": 7,
	"ocho": 8, "nueve": 9, "diez": 10, "once": 11, "doce": 12, "trece": 13, "catorce": 14, "quince": 15,
	"dieciseis": 16, "diecisiete": 17, "dieciocho": 18, "diecinueve": 19, "veinte": 20, "veintiuno": 21,
	"veintiun": 21, "veintiuna": 21, "veintidos": 22, "veintitres": 23, "veinticuatro": 24, "veinticinco": 25,
	"veintiseis": 26, "veintisiete": 27, "veintiocho": 28, "veintinueve": 29, "treinta": 30, "cuarenta": 40,
	"cincuenta": 50, "sesenta": 60, "setenta": 70, "ochenta": 80, "noventa": 90, "cien": 100, "ciento": 100,
	"doscientos": 200, "doscientas": 200, "trescientos": 300, "trescientas": 300, "cuatrocientos": 400,
	"cuatrocientas": 400, "quinientos": 500, "quinientas": 500, "seiscientos": 600, "seiscientas": 600,
	"setecientos": 700, "setecientas": 700, "ochocientos": 800, "ochocientas": 800, "novecientos": 900,
	"novecientas": 900,
}

var denominadores = map[string]int64{
	"medio": 2, "medios": 2, "media": 2, "medias": 2, "tercio": 3, "tercios": 3, "cuarto": 4, "cuartos": 4,
	"quinto": 5, "quintos": 5, "sexto": 6, "sextos": 6, "septimo": 7, "septimos": 7, "setimo": 7, "octavo": 8,
	"octavos": 8, "noveno": 9, "novenos": 9, "decimo": 10, "decimos": 10, "centesimo": 100, "centesimos": 100,
	"milesimo": 1000, "milesimos": 1000,
}

// lugar is the smallest decimal place already used by a group below 1000 (1, 10 or 100; 1000 when empty).
func lugar(g int64) int64 {
	switch {
	case g == 0:
		return 1000
	case g%10 != 0:
		return 1
	case g%100 != 0:
		return 10
	}
	return 100
}

// numeroEnPalabras reads a number written in words at toks[i] and returns it with the number of
// tokens used (0 if there is none). "un"/"una" only count before "mil", "millón" or a denominator.
func numeroEnPalabras(toks []nucleo.Token, i int) (*big.Rat, int) {
	norma := func(k int) string {
		if k < len(toks) && toks[k].Clase == "palabra" {
			return toks[k].Norma
		}
		return ""
	}
	var total, grupo int64
	hay := false
	j := i
	for j < len(toks) {
		w := norma(j)
		if v, ok := valoresPalabra[w]; ok {
			if (w == "un" || w == "una") && !hay {
				sig := norma(j + 1)
				if _, den := denominadores[sig]; !(den || sig == "mil" || sig == "millon" || sig == "millones") {
					break
				}
			}
			if v != 0 && v >= lugar(grupo) || v == 0 && hay {
				break
			}
			grupo += v
			hay = true
			j++
			if v >= 20 && v < 100 && v%10 == 0 && norma(j) == "y" {
				if u, ok := valoresPalabra[norma(j+1)]; ok && u >= 1 && u <= 9 {
					j++
				}
			}
			continue
		}
		if w == "mil" && (hay || j == i) {
			if grupo == 0 {
				grupo = 1
			}
			total += grupo * 1000
			grupo = 0
			hay = true
			j++
			continue
		}
		if (w == "millon" || w == "millones") && hay {
			total = (total + grupo) * 1_000_000
			grupo = 0
			j++
			continue
		}
		break
	}
	if !hay {
		return nil, 0
	}
	v := big.NewRat(total+grupo, 1)
	if d, ok := denominadores[norma(j)]; ok && total+grupo > 0 {
		v.Quo(v, big.NewRat(d, 1))
		j++
	} else if norma(j) == "y" && (norma(j+1) == "medio" || norma(j+1) == "media") {
		v.Add(v, big.NewRat(1, 2))
		j += 2
	}
	return v, j - i
}

// Numeros finds the numbers in s, in order: digits ("12", "3,5" = 7/2, "-4"), words ("dos mil
// trescientos cuarenta y cinco" = 2345, "tres cuartos" = 3/4, "un tercio", "la mitad" = 1/2,
// "y media"), percentages ("15%" and "15 por ciento" = 3/20, Texto keeps the sign) and powers written
// as expressions ("2^61-1"), which are returned as text with a nil Valor (not evaluated).
func Numeros(s string) []nucleo.Numero {
	toks := Tokenizar(s)
	var out []nucleo.Numero
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.Clase {
		case "numero":
			// expression with a power: 2^61-1, 10^9
			if fin, ok := finPotencia(s, toks, i); ok {
				out = append(out, nucleo.Numero{Valor: nil, Desde: t.Desde, Hasta: toks[fin].Hasta, Texto: s[t.Desde:toks[fin].Hasta]})
				i = fin
				continue
			}
			v := new(big.Rat).Set(t.Num)
			desde, hasta := t.Desde, t.Hasta
			if i > 0 && esMenos(toks[i-1]) && toks[i-1].Hasta == t.Desde && (i == 1 || !operando(toks[i-2])) {
				v.Neg(v)
				desde = toks[i-1].Desde
			}
			// percentages
			if i+1 < len(toks) && toks[i+1].Texto == "%" {
				v.Quo(v, big.NewRat(100, 1))
				hasta = toks[i+1].Hasta
				i++
			} else if i+2 < len(toks) && toks[i+1].Norma == "por" && toks[i+2].Norma == "ciento" {
				v.Quo(v, big.NewRat(100, 1))
				hasta = toks[i+2].Hasta
				i += 2
			}
			out = append(out, nucleo.Numero{Valor: v, Desde: desde, Hasta: hasta, Texto: s[desde:hasta]})
		case "palabra":
			if t.Norma == "mitad" && i > 0 && (toks[i-1].Norma == "la" || toks[i-1].Norma == "una") {
				out = append(out, nucleo.Numero{Valor: big.NewRat(1, 2), Desde: toks[i-1].Desde, Hasta: t.Hasta, Texto: s[toks[i-1].Desde:t.Hasta]})
				continue
			}
			if (t.Norma == "media" || t.Norma == "medio") && i > 0 && toks[i-1].Norma == "y" {
				// "dos horas y media": the half is a number of its own
				out = append(out, nucleo.Numero{Valor: big.NewRat(1, 2), Desde: toks[i-1].Desde, Hasta: t.Hasta, Texto: s[toks[i-1].Desde:t.Hasta]})
				continue
			}
			v, k := numeroEnPalabras(toks, i)
			if k == 0 {
				continue
			}
			fin := i + k - 1
			hasta := toks[fin].Hasta
			if fin+2 < len(toks) && toks[fin+1].Norma == "por" && toks[fin+2].Norma == "ciento" {
				v.Quo(v, big.NewRat(100, 1))
				hasta = toks[fin+2].Hasta
				fin += 2
			}
			out = append(out, nucleo.Numero{Valor: v, Desde: t.Desde, Hasta: hasta, Texto: s[t.Desde:hasta]})
			i = fin
		}
	}
	return out
}

func esMenos(t nucleo.Token) bool { return t.Texto == "-" || t.Texto == "−" }

// operando reports whether t ends an operand (so a following "-" is a binary minus).
func operando(t nucleo.Token) bool {
	return t.Clase == "numero" || t.Clase == "palabra" && len([]rune(t.Texto)) == 1 || t.Texto == ")"
}

// finPotencia: if toks[i] starts a run like 2^61-1 (digits and + - * / ^ without spaces, with a "^"),
// it returns the index of the last token of the run.
func finPotencia(s string, toks []nucleo.Token, i int) (int, bool) {
	fin, potencia := i, false
	for k := i + 1; k+1 < len(toks); k += 2 {
		op, sig := toks[k], toks[k+1]
		if op.Desde != toks[k-1].Hasta || sig.Desde != op.Hasta || sig.Clase != "numero" {
			break
		}
		if !strings.Contains("^+-*/", op.Texto) || op.Texto == "" {
			break
		}
		if op.Texto == "^" {
			potencia = true
		}
		fin = k + 1
	}
	return fin, potencia
}
