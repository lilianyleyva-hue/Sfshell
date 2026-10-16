package servidor

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Contracts between the Motor (cerebro) and the page (web/app.js) that nucleo.Respuesta and
// nucleo.Opinion do not spell out with their own fields. The page follows them exactly; a Motor that
// wants these features must follow them too.
//
// 1. Explanation phrases in the gutter (§6.2, Código tab: "each Frase shows next to its line in the
//    gutter on hover"). nucleo.Respuesta has no field for frases, so they travel as one entry of
//    Respuesta.Tablas built with TablaFrases:
//      - Cabecera[0] is CabeceraFrases ("Línea"); the page accepts any first header that, lowercased and
//        without accents, starts with "linea", and needs at least two columns;
//      - each row is [line number in the code the user sent (1-based, decimal), phrase, …]; extra
//        columns are joined to the phrase with " — "; rows whose first cell is not a positive number
//        are ignored; several rows for the same line are joined with a space.
//    The table is also shown as a normal table in the answer, so it doubles as a readable fallback.
//    FrasesDeTablas is the Go twin of the page's reading (esTablaFrases/actualizarExplicacion).
//
// 2. "¿Está bien? [Sí] [No]" (§6.2). Both buttons post /api/opinion at once:
//    {sesion, tarea, correcto: true|false, comentario: "", ejemplo: ""}. After "No" the page offers the
//    optional field "¿Qué debería dar?"; if the user fills it, a second opinion for the same tarea is
//    posted with correcto=false and ejemplo set. Motor.Opinar must treat a later opinion of the same
//    sesion+tarea that only adds an ejemplo as a complement of the first one (attach the example), not
//    as a second negative vote.

// CabeceraFrases is the first header of the explanation-phrases table.
const CabeceraFrases = "Línea"

// TituloFrases is the title the page shows above that table.
const TituloFrases = "Cómo lo leo, línea a línea"

// FraseLinea is one explanation phrase tied to a line of the user's code (analisis.Frase without the
// level; servidor imports only nucleo).
type FraseLinea struct {
	Linea int
	Texto string
}

// TablaFrases builds the table that carries explanation phrases to the page (contract 1). Phrases with
// a line below 1 or no text are left out; the rest keep their order, stably sorted by line.
func TablaFrases(frases []FraseLinea) nucleo.Tabla {
	t := nucleo.Tabla{Titulo: TituloFrases, Cabecera: []string{CabeceraFrases, "Qué hace"}, Filas: [][]string{}}
	buenas := make([]FraseLinea, 0, len(frases))
	for _, f := range frases {
		if f.Linea >= 1 && strings.TrimSpace(f.Texto) != "" {
			buenas = append(buenas, f)
		}
	}
	sort.SliceStable(buenas, func(i, j int) bool { return buenas[i].Linea < buenas[j].Linea })
	for _, f := range buenas {
		t.Filas = append(t.Filas, []string{strconv.Itoa(f.Linea), strings.TrimSpace(f.Texto)})
	}
	return t
}

// EsTablaFrases reports whether t carries explanation phrases (same rule as the page).
func EsTablaFrases(t nucleo.Tabla) bool {
	return len(t.Cabecera) >= 2 && strings.HasPrefix(sinTildes(t.Cabecera[0]), "linea")
}

// FrasesDeTablas reads the phrases per line the way the page does: the first phrases table wins.
func FrasesDeTablas(ts []nucleo.Tabla) map[int]string {
	for _, t := range ts {
		if !EsTablaFrases(t) {
			continue
		}
		out := map[int]string{}
		for _, f := range t.Filas {
			if len(f) == 0 {
				continue
			}
			n := enteroInicial(f[0])
			if n <= 0 {
				continue
			}
			frase := strings.Join(f[1:], " — ")
			if previa, ok := out[n]; ok {
				frase = previa + " " + frase
			}
			out[n] = frase
		}
		return out
	}
	return nil
}

// enteroInicial mirrors JavaScript's parseInt(s, 10) for the cases that matter here: optional leading
// spaces and sign, then decimal digits; anything else gives 0.
func enteroInicial(s string) int {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	signo := 1
	if strings.HasPrefix(s, "-") {
		signo, s = -1, s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' || n > 1e8 {
			break
		}
		n = n*10 + int(r-'0')
	}
	return signo * n
}

// sinTildes lowercases s and drops the accents of Spanish vowels (as normal() in app.js does).
func sinTildes(s string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u")
	return strings.ToLower(r.Replace(s))
}
