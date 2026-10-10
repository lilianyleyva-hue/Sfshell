package logica

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Knights and knaves (N2): a knight ("caballero") always tells the truth, a knave ("escudero")
// always lies. "X dice que P" becomes X ↔ P, with atom i true when person i is a knight.

// ResultadoCaballeros lists the people, what each one said as a formula, and every model.
type ResultadoCaballeros struct {
	Personas []string
	Dichos   []Formula // speaker ↔ claim, one per statement
	Textos   []string  // the statements as read
	Modelos  [][]bool  // all assignments that satisfy every statement (true = caballero)
	Notas    []string
}

const maxPersonasCaballeros = 12

var verbosDecir = map[string]bool{
	"dice": true, "afirma": true, "responde": true, "contesta": true, "asegura": true, "declara": true,
	"dijo": true, "afirmo": true, "respondio": true, "contesto": true, "aseguro": true, "grita": true,
}

var palabrasCaballero = map[string]bool{
	"caballero": true, "caballeros": true, "veraz": true, "veraces": true, "honesto": true, "honestos": true,
	"sincero": true, "sinceros": true,
}

var palabrasEscudero = map[string]bool{
	"escudero": true, "escuderos": true, "bribon": true, "bribones": true, "mentiroso": true, "mentirosos": true,
	"embustero": true, "embusteros": true, "villano": true, "villanos": true,
}

// EsCaballeros reports whether the text looks like a knights-and-knaves puzzle.
func EsCaballeros(texto string) bool {
	tipo, dice := false, false
	for _, t := range tokenizar(texto) {
		if palabrasCaballero[t.norma] || palabrasEscudero[t.norma] {
			tipo = true
		}
		if verbosDecir[t.norma] {
			dice = true
		}
	}
	return tipo && dice
}

type lectorCaballeros struct {
	personas []string
	indice   map[string]int
	notas    []string
	directo  bool // the statement being read is direct speech
}

func (l *lectorCaballeros) persona(nombre string) int {
	k := nucleo.Normalizar(nombre)
	if i, ok := l.indice[k]; ok {
		return i
	}
	l.indice[k] = len(l.personas)
	l.personas = append(l.personas, nombre)
	return len(l.personas) - 1
}

func esNombrePropio(t tok) bool {
	return !t.sim && t.mayus && !determinantesTermino[t.norma] && t.norma != "yo" && t.norma != "el"
}

// Caballeros solves a knights-and-knaves puzzle by enumerating every assignment (≤ 12 people),
// cross-checked with DPLL (satisfiable iff some model exists; a unique model is confirmed by
// showing that excluding it is unsatisfiable).
func Caballeros(ctx context.Context, texto string) (ResultadoCaballeros, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	l := &lectorCaballeros{indice: map[string]int{}}
	type dicho struct {
		quien   int
		toks    []tok
		directo bool // quoted or after ":" (direct speech)
	}
	var dichos []dicho
	for _, f := range partirFrases(tokenizar(texto)) {
		ts := f.toks
		d := -1
		for i, t := range ts {
			if verbosDecir[t.norma] && i > 0 {
				d = i
				break
			}
		}
		if d < 0 {
			continue
		}
		suj := sinDeterminantes(recortar(ts[:d]))
		if len(suj) != 1 || !(esNombrePropio(suj[0]) || utf8.RuneCountInString(suj[0].orig) == 1) {
			continue
		}
		quien := l.persona(suj[0].orig)
		directo := false
		for k := d + 1; k < len(ts) && k <= d+2; k++ {
			directo = directo || ts[k].norma == ":" || ts[k].norma == "\""
		}
		resto := sinComillas(ts[d+1:])
		resto = recortar(resto)
		for len(resto) > 0 && (resto[0].norma == "que" || resto[0].norma == ":") {
			resto = recortar(resto[1:])
		}
		if len(resto) == 0 {
			continue
		}
		dichos = append(dichos, dicho{quien: quien, toks: resto, directo: directo})
	}
	if len(dichos) == 0 {
		return ResultadoCaballeros{}, fmt.Errorf("logica: no encuentro frases del tipo «A dice que …»: %w", nucleo.ErrNoEntiendo)
	}
	// people named inside the claims (capitalized words before a copula) are registered too
	for _, d := range dichos {
		for _, t := range d.toks {
			if esNombrePropio(t) && utf8.RuneCountInString(t.orig) == 1 && !strings.ContainsAny(t.orig, "YOEU") {
				l.persona(t.orig)
			}
		}
	}
	var res ResultadoCaballeros
	for _, d := range dichos {
		l.directo = d.directo
		f, err := l.afirmacion(d.toks, d.quien)
		if err != nil {
			return ResultadoCaballeros{}, err
		}
		res.Dichos = append(res.Dichos, Equiv{Atomo{d.quien}, f})
		res.Textos = append(res.Textos, l.personas[d.quien]+" dice: «"+strings.TrimSpace(textoToks(d.toks))+"»")
	}
	res.Personas = l.personas
	res.Notas = l.notas
	n := len(l.personas)
	if n > maxPersonasCaballeros {
		return ResultadoCaballeros{}, fmt.Errorf("logica: hay %d personas; solo resuelvo hasta %d: %w", n, maxPersonasCaballeros, nucleo.ErrNoSoportado)
	}
	v := make([]bool, n)
	for mask := 0; mask < 1<<n; mask++ {
		for i := 0; i < n; i++ {
			v[i] = mask&(1<<(n-1-i)) == 0
		}
		ok := true
		for _, f := range res.Dichos {
			if !Evaluar(f, v) {
				ok = false
				break
			}
		}
		if ok {
			res.Modelos = append(res.Modelos, append([]bool(nil), v...))
		}
	}
	// cross-check with DPLL
	_, sat, err := Satisfacible(ctx, res.Dichos, n)
	if err != nil {
		return ResultadoCaballeros{}, err
	}
	if sat != (len(res.Modelos) > 0) {
		return ResultadoCaballeros{}, ErrNoCoinciden
	}
	if len(res.Modelos) == 1 {
		var lits []Formula
		for i, b := range res.Modelos[0] {
			if b {
				lits = append(lits, Atomo{i})
			} else {
				lits = append(lits, No{Atomo{i}})
			}
		}
		otro := append(append([]Formula(nil), res.Dichos...), No{conjuncion(lits)})
		if _, sat, err := Satisfacible(ctx, otro, n); err != nil {
			return ResultadoCaballeros{}, err
		} else if sat {
			return ResultadoCaballeros{}, ErrNoCoinciden
		}
	}
	return res, nil
}

// afirmacion parses a claim: a simple claim, or simple claims joined by "y"/"o", optionally
// prefixed by "no es cierto que"/"es falso que".
func (l *lectorCaballeros) afirmacion(ts []tok, quien int) (Formula, error) {
	ts = recortar(ts)
	for _, s := range secNegacion {
		if empiezaPor(ts, s) && len(ts) > len(s) {
			f, err := l.afirmacion(ts[len(s):], quien)
			if err != nil {
				return nil, err
			}
			return No{f}, nil
		}
	}
	if f, err := l.simple(ts, quien); err == nil {
		return f, nil
	}
	for _, sep := range []map[string]bool{{"o": true, "u": true}, {"y": true, "e": true, "pero": true}} {
		partes := partirNivel0(ts, sep)
		if len(partes) < 2 {
			continue
		}
		var r Formula
		ok := true
		for _, p := range partes {
			f, err := l.simple(recortar(p), quien)
			if err != nil {
				ok = false
				break
			}
			if r == nil {
				r = f
			} else if sep["o"] {
				r = O{r, f}
			} else {
				r = Y{r, f}
			}
		}
		if ok {
			return r, nil
		}
	}
	return nil, fmt.Errorf("logica: no entiendo lo que dice %s: «%s»: %w", l.personas[quien], textoToks(ts), nucleo.ErrNoEntiendo)
}

var copulasCaballeros = map[string]bool{"es": true, "son": true, "soy": true, "somos": true, "eres": true, "sois": true, "sea": true, "sean": true}

var errNoSimple = errors.New("no es una afirmación simple")

// simple parses "<sujetos> [no] <cópula> [un|una|de|del] <tipo>".
func (l *lectorCaballeros) simple(ts []tok, quien int) (Formula, error) {
	c := -1
	for i, t := range ts {
		if copulasCaballeros[t.norma] {
			if c >= 0 {
				return nil, errNoSimple
			}
			c = i
		}
	}
	if c < 0 || c == len(ts)-1 {
		return nil, errNoSimple
	}
	neg := c > 0 && ts[c-1].norma == "no"
	fin := c
	if neg {
		fin = c - 1
	}
	sujetos := recortar(ts[:fin])
	// who: a list of people and how they are combined ("y" all, "o" any, "uno" exactly one…)
	modo := "y"
	var gente []int
	todos := func() []int {
		var out []int
		for i := range l.personas {
			out = append(out, i)
		}
		return out
	}
	textoSuj := " " + nucleo.Normalizar(textoToks(sujetos)) + " "
	switch {
	case len(sujetos) == 0:
		if ts[c].norma == "somos" {
			gente = todos()
		} else {
			gente = []int{quien}
		}
	case strings.Contains(textoSuj, " al menos uno ") || strings.Contains(textoSuj, " alguno ") || strings.Contains(textoSuj, " uno de "):
		gente, modo = todos(), "o"
		if strings.Contains(textoSuj, " exactamente uno ") || strings.Contains(textoSuj, " solo uno ") || strings.Contains(textoSuj, " solamente uno ") {
			modo = "uno"
		}
	case strings.Contains(textoSuj, " ninguno ") || strings.Contains(textoSuj, " nadie "):
		gente, modo = todos(), "ninguno"
	case strings.Contains(textoSuj, " todos ") || strings.Contains(textoSuj, " ambos ") || strings.Contains(textoSuj, " los dos ") || strings.TrimSpace(textoSuj) == "nosotros":
		gente = todos()
	default:
		for _, t := range sujetos {
			switch {
			case t.norma == "y" || t.norma == "e" || t.sim:
			case t.norma == "o" || t.norma == "u":
				modo = "o"
			case t.norma == "yo":
				gente = append(gente, quien)
			case t.norma == "el" || t.norma == "ella":
				// reported speech "B dice que A y él…": él is the speaker when someone else is named
				otros := false
				for _, u := range sujetos {
					otros = otros || u.norma == "yo" || esNombrePropio(u) || (utf8.RuneCountInString(u.orig) == 1 && u.mayus)
				}
				if otros && !l.directo {
					gente = append(gente, quien)
				} else if len(l.personas) == 2 {
					gente = append(gente, 1-quien)
				} else {
					return nil, fmt.Errorf("logica: no sé a quién se refiere «%s»: %w", t.orig, nucleo.ErrNoEntiendo)
				}
			case esNombrePropio(t) || utf8.RuneCountInString(t.orig) == 1:
				i, ok := l.indice[nucleo.Normalizar(t.orig)]
				if !ok {
					i = l.persona(t.orig)
				}
				gente = append(gente, i)
			default:
				return nil, errNoSimple
			}
		}
	}
	if len(gente) == 0 {
		return nil, errNoSimple
	}
	var pred []string
	for _, t := range ts[c+1:] {
		if t.sim || determinantesTermino[t.norma] || t.norma == "de" || t.norma == "del" {
			continue
		}
		pred = append(pred, t.norma)
	}
	if len(pred) == 0 {
		return nil, errNoSimple
	}
	textoPred := " " + strings.Join(pred, " ") + " "
	var f Formula
	switch {
	case palabrasCaballero[pred[0]] || palabrasEscudero[pred[0]]:
		lit := func(i int) Formula {
			if palabrasCaballero[pred[0]] {
				return Atomo{i}
			}
			return No{Atomo{i}}
		}
		var lits []Formula
		for _, i := range gente {
			lits = append(lits, lit(i))
		}
		switch modo {
		case "o":
			f = disyuncion(lits)
		case "uno":
			f = exactamenteUno(lits)
		case "ninguno":
			var ns []Formula
			for _, x := range lits {
				ns = append(ns, negar(x))
			}
			f = conjuncion(ns)
		default:
			f = conjuncion(lits)
		}
	case strings.Contains(textoPred, " mismo ") || strings.Contains(textoPred, " misma ") || strings.Contains(textoPred, " iguales "):
		if len(gente) < 2 {
			return nil, errNoSimple
		}
		var eqs []Formula
		for k := 1; k < len(gente); k++ {
			eqs = append(eqs, Equiv{Atomo{gente[0]}, Atomo{gente[k]}})
		}
		f = conjuncion(eqs)
	case strings.Contains(textoPred, " distint") || strings.Contains(textoPred, " diferente"):
		if len(gente) != 2 {
			return nil, errNoSimple
		}
		f = No{Equiv{Atomo{gente[0]}, Atomo{gente[1]}}}
	default:
		return nil, errNoSimple
	}
	if neg {
		f = No{f}
	}
	return f, nil
}

func disyuncion(fs []Formula) Formula {
	if len(fs) == 0 {
		return nil
	}
	r := fs[0]
	for _, f := range fs[1:] {
		r = O{r, f}
	}
	return r
}

// exactamenteUno: at least one, and no two together.
func exactamenteUno(fs []Formula) Formula {
	partes := []Formula{disyuncion(fs)}
	for i := 0; i < len(fs); i++ {
		for j := i + 1; j < len(fs); j++ {
			partes = append(partes, No{Y{fs[i], fs[j]}})
		}
	}
	return conjuncion(partes)
}

// DescribirCaballeros writes one model: "A es caballero y B es escudero".
func DescribirCaballeros(personas []string, m []bool) string {
	var partes []string
	for i, p := range personas {
		if i < len(m) && m[i] {
			partes = append(partes, p+" es caballero")
		} else {
			partes = append(partes, p+" es escudero")
		}
	}
	return unirY(partes)
}
