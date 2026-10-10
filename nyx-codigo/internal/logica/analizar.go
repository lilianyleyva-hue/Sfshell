package logica

import (
	"errors"
	"fmt"

	"nyxcodigo/internal/nucleo"
)

// Argumento is a parsed propositional argument.
type Argumento struct {
	Premisas   []Formula
	Conclusion Formula // nil if the question is "¿qué se concluye?"
	Atomos     *Atomos
	Notas      []string // "Entiendo «se moja» como «el suelo se moja»"
}

// analizador parses Spanish (and symbolic) propositional sentences into formulas.
type analizador struct {
	lem    nucleo.Lematizador
	at     *Atomos
	notas  []string
	pistas []pista // atomic segments of the other sentences, used to split "si A B" without a comma
	actual int     // index of the sentence being parsed
}

type pista struct {
	frase int
	bolsa []string
}

func nuevoAnalizador(lem nucleo.Lematizador) *analizador {
	return &analizador{lem: lem, at: &Atomos{}, actual: -1}
}

// palabrasConectivas split a sentence into atomic segments (for pistas).
var palabrasConectivas = map[string]bool{
	"si": true, "entonces": true, "o": true, "u": true, "y": true, "e": true, "ni": true, "pero": true,
	",": true, "->": true, "<->": true, "&": true, "|": true, "!": true, "(": true, ")": true, "cuando": true,
	"solo": true, "sii": true, "implica": true, "no": true, "cierto": true, "falso": true,
}

func (z *analizador) calcularPistas(fs []frase) {
	for i, f := range fs {
		var seg []tok
		vaciar := func() {
			if b := bolsaToks(seg, z.lem); len(b) > 0 {
				z.pistas = append(z.pistas, pista{frase: i, bolsa: b})
			}
			seg = nil
		}
		for _, t := range sinComillas(f.toks) {
			if palabrasConectivas[t.norma] {
				vaciar()
				continue
			}
			seg = append(seg, t)
		}
		vaciar()
	}
}

// AnalizarArgumento parses premises and a conclusion. Sentences are split on ; . newlines and
// "y además"; the conclusion is the ¿…? sentence or the one introduced by "por lo tanto",
// "entonces", "luego", "así que" or "¿se sigue que". It returns an error wrapping
// nucleo.ErrNoEntiendo when nothing can be read.
func AnalizarArgumento(texto string, lem nucleo.Lematizador) (Argumento, error) {
	z := nuevoAnalizador(lem)
	fs := partirFrases(tokenizar(texto))
	if len(fs) == 0 {
		return Argumento{}, fmt.Errorf("logica: texto vacío: %w", nucleo.ErrNoEntiendo)
	}
	z.calcularPistas(fs)
	ic := -1
	for i, f := range fs {
		if f.pregunta {
			ic = i
		}
	}
	if ic < 0 {
		for i, f := range fs {
			if f.conclusion {
				ic = i
			}
		}
	}
	arg := Argumento{Atomos: z.at}
	for i, f := range fs {
		if i == ic {
			continue
		}
		z.actual = i
		p, err := z.formula(f.toks)
		if err != nil {
			return Argumento{}, err
		}
		arg.Premisas = append(arg.Premisas, p)
	}
	if ic >= 0 && !esQueConcluye(fs[ic]) {
		z.actual = ic
		c, err := z.formula(fs[ic].toks)
		if err != nil {
			return Argumento{}, err
		}
		arg.Conclusion = c
	}
	if len(arg.Premisas) == 0 {
		return Argumento{}, fmt.Errorf("logica: no encuentro premisas: %w", nucleo.ErrNoEntiendo)
	}
	arg.Notas = z.notas
	return arg, nil
}

// AnalizarFormula parses one sentence or symbolic formula ("p o no p", "(p -> q) & p"). Atoms are
// added to a (which may be nil: a fresh Atomos is used and returned).
func AnalizarFormula(texto string, a *Atomos, lem nucleo.Lematizador) (Formula, *Atomos, []string, error) {
	z := nuevoAnalizador(lem)
	if a != nil {
		z.at = a
	}
	ts := tokenizar(texto)
	var limpio []tok
	for _, t := range ts {
		switch t.norma {
		case "¿", "?", ";", ".", "\n":
			continue
		}
		limpio = append(limpio, t)
	}
	f, err := z.formula(limpio)
	if err != nil {
		return nil, z.at, nil, err
	}
	return f, z.at, z.notas, nil
}

var errVacia = fmt.Errorf("logica: falta una parte de la frase: %w", nucleo.ErrNoEntiendo)

func (z *analizador) formula(ts []tok) (Formula, error) {
	ts = recortar(sinComillas(ts))
	if len(ts) == 0 {
		return nil, errVacia
	}
	if err := parentesisEquilibrados(ts); err != nil {
		return nil, err
	}
	return z.equiv(ts)
}

func parentesisEquilibrados(ts []tok) error {
	p := 0
	for _, t := range ts {
		switch t.norma {
		case "(":
			p++
		case ")":
			p--
			if p < 0 {
				return fmt.Errorf("logica: sobra un paréntesis de cierre: %w", nucleo.ErrNoEntiendo)
			}
		}
	}
	if p != 0 {
		return fmt.Errorf("logica: falta cerrar un paréntesis: %w", nucleo.ErrNoEntiendo)
	}
	return nil
}

// buscarNivel0 finds the first occurrence at parenthesis depth 0, at index ≥ desde, of any of the
// sequences; it returns the index and the sequence length (−1, 0 if none).
func buscarNivel0(ts []tok, desde int, seqs ...[]string) (int, int) {
	prof := 0
	for i := 0; i < len(ts); i++ {
		switch ts[i].norma {
		case "(":
			prof++
			continue
		case ")":
			prof--
			continue
		}
		if prof != 0 || i < desde {
			continue
		}
		for _, s := range seqs {
			if empiezaPor(ts[i:], s) {
				return i, len(s)
			}
		}
	}
	return -1, 0
}

// partirNivel0 splits at every depth-0 occurrence of the single-word separators.
func partirNivel0(ts []tok, seps map[string]bool) [][]tok {
	var out [][]tok
	prof, ini := 0, 0
	for i, t := range ts {
		switch t.norma {
		case "(":
			prof++
		case ")":
			prof--
		default:
			if prof == 0 && seps[t.norma] {
				out = append(out, ts[ini:i])
				ini = i + 1
			}
		}
	}
	return append(out, ts[ini:])
}

var (
	secEquiv     = [][]string{{"si", "y", "solo", "si"}, {"si", "y", "solamente", "si"}, {"si", "y", "solo", "y", "si"}, {"sii"}, {"<->"}, {"equivale", "a"}}
	secSoloSi    = [][]string{{"solo", "si"}, {"solamente", "si"}, {"unicamente", "si"}}
	secImplica   = [][]string{{"->"}, {"implica", "que"}, {"implica"}}
	secCondicion = [][]string{{"siempre", "que"}, {"si"}, {"cuando"}, {"en", "caso", "de", "que"}}
	sepO         = map[string]bool{"o": true, "u": true, "|": true}
	sepY         = map[string]bool{"y": true, "e": true, "pero": true, "&": true, "aunque": true, "mas": false, ",": true}
	sepNi        = map[string]bool{"ni": true}
)

func (z *analizador) equiv(ts []tok) (Formula, error) {
	ts = recortar(ts)
	if len(ts) == 0 {
		return nil, errVacia
	}
	if i, l := buscarNivel0(ts, 1, secEquiv...); i > 0 {
		a, err := z.impl(ts[:i])
		if err != nil {
			return nil, err
		}
		b, err := z.equiv(ts[i+l:])
		if err != nil {
			return nil, err
		}
		return Equiv{a, b}, nil
	}
	return z.impl(ts)
}

func (z *analizador) impl(ts []tok) (Formula, error) {
	ts = recortar(ts)
	if len(ts) == 0 {
		return nil, errVacia
	}
	// "no es cierto que A y B" denies the whole rest of the sentence
	for _, s := range secNegacion {
		if empiezaPor(ts, s) && len(ts) > len(s) {
			f, err := z.impl(ts[len(s):])
			if err != nil {
				return nil, err
			}
			return No{f}, nil
		}
	}
	for _, seq := range secCondicion {
		if !empiezaPor(ts, seq) || len(ts) <= len(seq) {
			continue
		}
		cuerpo := ts[len(seq):]
		var a, b []tok
		if i, l := buscarNivel0(cuerpo, 1, []string{"entonces"}, []string{",", "entonces"}, []string{"->"}); i > 0 {
			a, b = cuerpo[:i], cuerpo[i+l:]
		} else if i, _ := buscarNivel0(cuerpo, 1, []string{","}); i > 0 {
			a, b = cuerpo[:i], cuerpo[i+1:]
		} else if len(cuerpo) >= 2 {
			i := z.partirSinComa(cuerpo)
			a, b = cuerpo[:i], cuerpo[i:]
			z.notas = append(z.notas, fmt.Sprintf("Sin coma, separo así: «%s» → «%s»", textoToks(a), textoToks(b)))
		} else {
			return nil, fmt.Errorf("logica: la condición «%s» no dice qué pasa: %w", textoToks(ts), nucleo.ErrNoEntiendo)
		}
		fa, err := z.or(a)
		if err != nil {
			return nil, err
		}
		fb, err := z.impl(b)
		if err != nil {
			return nil, err
		}
		return Implica{fa, fb}, nil
	}
	if i, l := buscarNivel0(ts, 1, secImplica...); i > 0 {
		a, err := z.or(ts[:i])
		if err != nil {
			return nil, err
		}
		b, err := z.impl(ts[i+l:])
		if err != nil {
			return nil, err
		}
		return Implica{a, b}, nil
	}
	if i, l := buscarNivel0(ts, 1, secSoloSi...); i > 0 {
		a, err := z.or(ts[:i])
		if err != nil {
			return nil, err
		}
		b, err := z.or(ts[i+l:])
		if err != nil {
			return nil, err
		}
		return Implica{a, b}, nil
	}
	// "B si A" / "B cuando A"
	if i, l := buscarNivel0(ts, 1, []string{"si"}, []string{"cuando"}, []string{"siempre", "que"}); i > 0 && i+l < len(ts) {
		b, err := z.or(ts[:i])
		if err != nil {
			return nil, err
		}
		a, err := z.or(ts[i+l:])
		if err != nil {
			return nil, err
		}
		return Implica{a, b}, nil
	}
	return z.or(ts)
}

// determinantes and pronouns that usually start the consequent of "si A B".
var iniciosSujeto = map[string]int{
	"el": 2, "la": 2, "los": 2, "las": 2, "un": 2, "una": 2, "unos": 2, "unas": 2, "mi": 2, "tu": 2,
	"su": 2, "mis": 2, "sus": 2, "yo": 2, "ella": 2, "ellos": 2, "ellas": 2, "nosotros": 2, "usted": 2,
	"ustedes": 2, "todo": 1, "todos": 1, "no": 1, "se": 1, "me": 1, "te": 1, "nos": 1, "hay": 1,
}

// partirSinComa chooses where "si A B" splits when there is no comma or "entonces": a side that
// matches a segment of another sentence scores 3, a side that contains one scores 1, and a
// consequent that starts with a determiner or pronoun scores 2. Ties go to the earliest split.
func (z *analizador) partirSinComa(ts []tok) int {
	mejor, mejorP := 1, -1
	for i := 1; i < len(ts); i++ {
		izq, der := bolsaToks(ts[:i], z.lem), bolsaToks(ts[i:], z.lem)
		if len(izq) == 0 || len(der) == 0 {
			continue
		}
		p := 0
		for _, h := range z.pistas {
			if h.frase == z.actual {
				continue
			}
			switch {
			case igualBolsa(izq, h.bolsa):
				p += 3
			case incluye(izq, h.bolsa):
				p++
			}
			switch {
			case igualBolsa(der, h.bolsa):
				p += 3
			case incluye(der, h.bolsa):
				p++
			}
		}
		p += iniciosSujeto[ts[i].norma]
		if ts[i].mayus && !ts[i].sim {
			p++
		}
		if _, ok := iniciosSujeto[ts[i-1].norma]; ok && iniciosSujeto[ts[i-1].norma] == 2 {
			p -= 2 // never leave a determiner at the end of the condition
		}
		if p > mejorP {
			mejor, mejorP = i, p
		}
	}
	return mejor
}

func igualBolsa(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (z *analizador) or(ts []tok) (Formula, error) {
	ts = recortar(ts)
	if len(ts) == 0 {
		return nil, errVacia
	}
	// "o A o B", "o bien A o bien B"
	if ts[0].norma == "o" || ts[0].norma == "u" {
		ts = ts[1:]
		if len(ts) > 0 && ts[0].norma == "bien" {
			ts = ts[1:]
		}
	}
	partes := partirNivel0(ts, sepO)
	if len(partes) == 1 {
		return z.and(ts)
	}
	var r Formula
	for _, p := range partes {
		p = recortar(p)
		if len(p) > 0 && p[0].norma == "bien" {
			p = p[1:]
		}
		f, err := z.and(p)
		if err != nil {
			return nil, err
		}
		if r == nil {
			r = f
		} else {
			r = O{r, f}
		}
	}
	return r, nil
}

func (z *analizador) and(ts []tok) (Formula, error) {
	ts = recortar(ts)
	if len(ts) == 0 {
		return nil, errVacia
	}
	// "ni A ni B" → ¬A ∧ ¬B; "no A ni B" → ¬A ∧ ¬B
	if partes := partirNivel0(ts, sepNi); len(partes) > 1 {
		var r Formula
		for k, p := range partes {
			p = recortar(p)
			if k == 0 && len(p) == 0 {
				continue
			}
			f, err := z.and(p)
			if err != nil {
				return nil, err
			}
			if k > 0 {
				f = negar(f)
			}
			if r == nil {
				r = f
			} else {
				r = Y{r, f}
			}
		}
		if r == nil {
			return nil, errVacia
		}
		return r, nil
	}
	partes := partirNivel0(ts, sepY)
	var r Formula
	for _, p := range partes {
		p = recortar(p)
		if len(p) == 0 {
			continue
		}
		f, err := z.not(p)
		if err != nil {
			return nil, err
		}
		if r == nil {
			r = f
		} else {
			r = Y{r, f}
		}
	}
	if r == nil {
		return nil, errVacia
	}
	return r, nil
}

var secNegacion = [][]string{
	{"no", "es", "cierto", "que"}, {"no", "es", "verdad", "que"}, {"es", "falso", "que"}, {"no", "ocurre", "que"},
	{"es", "mentira", "que"}, {"no", "se", "cumple", "que"}, {"no", "pasa", "que"},
}
var secAfirmacion = [][]string{{"es", "cierto", "que"}, {"es", "verdad", "que"}, {"ocurre", "que"}, {"se", "cumple", "que"}}

func (z *analizador) not(ts []tok) (Formula, error) {
	ts = recortar(ts)
	if len(ts) == 0 {
		return nil, errVacia
	}
	for _, s := range secNegacion {
		if empiezaPor(ts, s) && len(ts) > len(s) {
			f, err := z.not(ts[len(s):])
			if err != nil {
				return nil, err
			}
			return No{f}, nil
		}
	}
	for _, s := range secAfirmacion {
		if empiezaPor(ts, s) && len(ts) > len(s) {
			return z.not(ts[len(s):])
		}
	}
	if ts[0].norma == "!" || (ts[0].norma == "no" && len(ts) > 1 && ts[1].norma == "(") {
		f, err := z.not(ts[1:])
		if err != nil {
			return nil, err
		}
		return No{f}, nil
	}
	if ts[0].norma == "(" && cierreDe(ts, 0) == len(ts)-1 {
		return z.equiv(ts[1 : len(ts)-1])
	}
	return z.clausula(ts)
}

func cierreDe(ts []tok, i int) int {
	prof := 0
	for j := i; j < len(ts); j++ {
		switch ts[j].norma {
		case "(":
			prof++
		case ")":
			prof--
			if prof == 0 {
				return j
			}
		}
	}
	return -1
}

// clausula turns an atomic clause into an atom; "no"/"nunca" inside it negates the literal.
func (z *analizador) clausula(ts []tok) (Formula, error) {
	neg := false
	var ws []tok
	for _, t := range ts {
		if t.sim {
			if t.norma == "!" {
				neg = !neg
			}
			continue
		}
		if t.norma == "no" || t.norma == "nunca" {
			neg = !neg
			continue
		}
		ws = append(ws, t)
	}
	for len(ws) > 1 && ws[0].norma == "que" {
		ws = ws[1:]
	}
	if len(ws) == 0 {
		return nil, errVacia
	}
	frase := textoToks(ws)
	if len(ws) == 1 && esLetra(ws[0].orig) {
		frase = ws[0].orig
	}
	bolsa := bolsaToks(ws, z.lem)
	i, nota, ok := z.at.buscarBolsa(bolsa, frase)
	if !ok {
		i = z.at.agregar(frase, bolsa)
	} else if nota != "" {
		z.notas = append(z.notas, nota)
	}
	var f Formula = Atomo{i}
	if neg {
		f = No{f}
	}
	return f, nil
}

// ErrSinConclusion is returned by checks that need a conclusion.
var ErrSinConclusion = errors.New("logica: no hay conclusión")
