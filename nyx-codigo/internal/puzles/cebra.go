package puzles

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Zebra puzzles (N2): every category (nationality, colour, pet…) is a Distintos group of house
// positions; clues become equalities, ±1 and |Δ| = 1 relations, fixed positions and negations.

// lexicoCebra maps known values to their category (built-in vocabulary, used when the puzzle does
// not list its categories as "colores: rojo, verde, …").
var lexicoCebra = map[string][]string{
	"Nacionalidad": {"inglés", "español", "japonés", "ucraniano", "noruego", "alemán", "sueco", "danés", "francés",
		"italiano", "chino", "ruso", "americano", "mexicano", "brasileño", "argentino", "portugués", "griego", "holandés", "canadiense"},
	"Color": {"rojo", "roja", "verde", "blanco", "blanca", "amarillo", "amarilla", "azul", "marfil", "negro", "negra",
		"rosa", "gris", "morado", "morada", "violeta"},
	"Mascota": {"perro", "caracoles", "caracol", "zorro", "caballo", "cebra", "gato", "pájaros", "pájaro", "peces", "pez",
		"hámster", "tortuga", "conejo", "loro"},
	"Bebida": {"café", "té", "leche", "zumo de naranja", "jugo de naranja", "agua", "cerveza", "vino", "refresco", "zumo"},
	"Tabaco": {"kools", "chesterfield", "lucky strike", "parliament", "old gold", "pall mall", "dunhill", "blends",
		"blue master", "prince", "winston", "marlboro"},
	"Profesión": {"pintor", "escultor", "diplomático", "violinista", "médico", "abogado", "ingeniero", "profesor", "maestro", "fotógrafo"},
	"Deporte":   {"fútbol", "tenis", "baloncesto", "natación", "ciclismo", "golf", "rugby", "voleibol"},
}

var ordenCategorias = []string{"Nacionalidad", "Color", "Mascota", "Bebida", "Tabaco", "Profesión", "Deporte"}

// raizCebra compares value words: "roja"/"rojo" → "roj", "caracoles"/"caracol" → "caracol".
func raizCebra(w string) string {
	w = nucleo.Normalizar(w)
	n := utf8.RuneCountInString(w)
	switch {
	case n > 4 && strings.HasSuffix(w, "es"):
		w = w[:len(w)-2]
	case n > 3 && strings.HasSuffix(w, "s"):
		w = w[:len(w)-1]
	}
	if utf8.RuneCountInString(w) > 3 && strings.ContainsAny(w[len(w)-1:], "aeo") {
		w = w[:len(w)-1]
	}
	return w
}

func clavesFrase(frase string) []string {
	var out []string
	for _, w := range strings.Fields(nucleo.Normalizar(frase)) {
		out = append(out, raizCebra(w))
	}
	return out
}

type valorCebra struct {
	cat     int
	nombre  string   // as shown
	claves  []string // word roots
	persona bool     // a nationality: answers "¿quién…?"
}

type mencion struct {
	valor int
	pos   int // token index
}

// ResultadoCebra is the full answer of a zebra puzzle (additive helper of Cebra).
type ResultadoCebra struct {
	Tabla         nucleo.Tabla
	NoEntendidas  []string
	Respuestas    []string // answers to the "¿quién …?" questions
	Unica         bool
	Casas         int
	Restricciones int
}

type lectorCebra struct {
	cats    []string
	valores []valorCebra
	casas   int
}

func (l *lectorCebra) categoria(nombre string) int {
	for i, c := range l.cats {
		if c == nombre {
			return i
		}
	}
	l.cats = append(l.cats, nombre)
	return len(l.cats) - 1
}

func (l *lectorCebra) valor(cat int, nombre string) int {
	cl := clavesFrase(nombre)
	for i, v := range l.valores {
		if v.cat == cat && strings.Join(v.claves, " ") == strings.Join(cl, " ") {
			return i
		}
	}
	l.valores = append(l.valores, valorCebra{cat: cat, nombre: nombre, claves: cl, persona: l.cats[cat] == "Nacionalidad"})
	return len(l.valores) - 1
}

type entradaLexico struct {
	cat    string
	nombre string
	claves []string
}

// menciones finds the values named in a sentence (longest phrases first). With explicit categories
// only those values count; otherwise the built-in vocabulary adds new values as they appear.
func (l *lectorCebra) menciones(frase string, lexico []entradaLexico, explicito bool) []mencion {
	ws := strings.Fields(nucleo.Normalizar(strings.NewReplacer(",", " ", ".", " ", ";", " ", "?", " ", "¿", " ", ":", " ").Replace(frase)))
	raices := make([]string, len(ws))
	for i, w := range ws {
		raices[i] = raizCebra(w)
	}
	usado := make([]bool, len(ws))
	var out []mencion
	for _, e := range lexico {
		k := len(e.claves)
		for i := 0; i+k <= len(ws); i++ {
			ok := true
			for j := 0; j < k; j++ {
				if usado[i+j] || raices[i+j] != e.claves[j] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			for j := 0; j < k; j++ {
				usado[i+j] = true
			}
			var v int
			if explicito {
				v = -1
				for idx, val := range l.valores {
					if strings.Join(val.claves, " ") == strings.Join(e.claves, " ") {
						v = idx
						break
					}
				}
				if v < 0 {
					continue
				}
			} else {
				v = l.valor(l.categoria(e.cat), e.nombre)
			}
			out = append(out, mencion{valor: v, pos: i})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].pos < out[b].pos })
	return out
}

var ordinales = map[string]int{"primera": 0, "primer": 0, "segunda": 1, "tercera": 2, "cuarta": 3, "quinta": 4, "sexta": 5, "septima": 6}

// ResolverCebra reads the clues (and questions), solves the puzzle with the CSP (uniqueness checked
// with maxSol = 2) and answers the "¿quién …?" questions.
func ResolverCebra(ctx context.Context, pistas []string) (ResultadoCebra, error) {
	l := &lectorCebra{}
	var res ResultadoCebra
	// explicit categories: "colores: rojo, verde, azul"
	explicito := false
	var frases []string
	for _, p := range pistas {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i := strings.Index(p, ":"); i > 0 && strings.Count(p[i:], ",") >= 1 && !strings.Contains(p[:i], " ") {
			cat := strings.TrimSpace(p[:i])
			c := l.categoria(mayusculaInicial(cat))
			for _, v := range strings.Split(p[i+1:], ",") {
				v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "."))
				v = strings.TrimPrefix(strings.TrimPrefix(v, "y "), "e ")
				if v != "" {
					l.valor(c, v)
				}
			}
			explicito = true
			continue
		}
		frases = append(frases, p)
	}
	var lexico []entradaLexico
	if explicito {
		for _, v := range l.valores {
			lexico = append(lexico, entradaLexico{cat: l.cats[v.cat], nombre: v.nombre, claves: v.claves})
		}
	} else {
		for _, cat := range ordenCategorias {
			for _, nombre := range lexicoCebra[cat] {
				lexico = append(lexico, entradaLexico{cat: cat, nombre: nombre, claves: clavesFrase(nombre)})
			}
		}
	}
	sort.SliceStable(lexico, func(a, b int) bool { return len(lexico[a].claves) > len(lexico[b].claves) })
	// first pass: values and the number of houses
	type pista struct {
		texto    string
		mens     []mencion
		pregunta bool
	}
	var ps []pista
	for _, f := range frases {
		norma := nucleo.Normalizar(f)
		pregunta := strings.Contains(f, "?") || strings.HasPrefix(norma, "quien") || strings.HasPrefix(norma, "que ") || strings.HasPrefix(norma, "de que")
		if strings.HasPrefix(norma, "hay ") && strings.Contains(norma, "casas") {
			for _, w := range strings.Fields(norma) {
				if v, ok := numerosPalabra[w]; ok {
					l.casas = int(v)
				}
				var x int
				if _, err := fmt.Sscanf(w, "%d", &x); err == nil {
					l.casas = x
				}
			}
			continue
		}
		ps = append(ps, pista{texto: f, mens: l.menciones(f, lexico, explicito), pregunta: pregunta})
	}
	if len(l.valores) == 0 {
		return res, fmt.Errorf("puzles: no reconozco ningún dato del acertijo: %w", nucleo.ErrNoEntiendo)
	}
	for c := range l.cats {
		cuenta := 0
		for _, v := range l.valores {
			if v.cat == c {
				cuenta++
			}
		}
		l.casas = max(l.casas, cuenta)
	}
	n := l.casas
	if n < 2 || n > 9 {
		return res, fmt.Errorf("puzles: no sé cuántas casas hay: %w", nucleo.ErrNoEntiendo)
	}
	// complete each category with unknown values
	for c := range l.cats {
		cuenta := 0
		for _, v := range l.valores {
			if v.cat == c {
				cuenta++
			}
		}
		for k := cuenta; k < n; k++ {
			l.valores = append(l.valores, valorCebra{cat: c, nombre: fmt.Sprintf("(%s desconocido)", strings.ToLower(l.cats[c])), claves: []string{fmt.Sprintf("#%d-%d", c, k)}})
		}
	}
	p := &Problema{}
	for _, v := range l.valores {
		p.Nombres = append(p.Nombres, v.nombre)
		p.Dom = append(p.Dom, DominioRango(0, n-1))
	}
	for c := range l.cats {
		var alcance []int
		for i, v := range l.valores {
			if v.cat == c {
				alcance = append(alcance, i)
			}
		}
		p.Restr = append(p.Restr, Restriccion{Alcance: alcance, Tipo: Distintos})
	}
	for _, pi := range ps {
		if pi.pregunta {
			continue
		}
		rs, ok := restriccionesPista(pi.texto, pi.mens, n)
		if !ok {
			res.NoEntendidas = append(res.NoEntendidas, pi.texto)
			continue
		}
		p.Restr = append(p.Restr, rs...)
		res.Restricciones += len(rs)
	}
	sols, _, err := Resolver(ctx, p, 2, nil)
	if err != nil && len(sols) == 0 {
		return res, err
	}
	if len(sols) == 0 {
		return res, fmt.Errorf("puzles: con estas pistas no hay solución: %w", nucleo.ErrNoEntiendo)
	}
	sol := sols[0]
	res.Unica = len(sols) == 1
	res.Casas = n
	res.Tabla = nucleo.Tabla{Titulo: "Solución", Cabecera: append([]string{"Casa"}, l.cats...)}
	if !res.Unica {
		res.Tabla.Titulo = "Una solución (hay más de una)"
	}
	for casa := 0; casa < n; casa++ {
		fila := []string{fmt.Sprint(casa + 1)}
		for c := range l.cats {
			for i, v := range l.valores {
				if v.cat == c && sol[i] == casa {
					fila = append(fila, v.nombre)
				}
			}
		}
		res.Tabla.Filas = append(res.Tabla.Filas, fila)
	}
	// answer "¿quién …?"
	for _, pi := range ps {
		if !pi.pregunta {
			continue
		}
		norma := nucleo.Normalizar(pi.texto)
		i := strings.Index(norma, "quien")
		if i < 0 || len(pi.mens) == 0 {
			continue
		}
		casa := sol[pi.mens[len(pi.mens)-1].valor]
		quien := fmt.Sprintf("el de la casa %d", casa+1)
		for idx, v := range l.valores {
			if v.persona && sol[idx] == casa {
				quien = "el " + v.nombre
			}
		}
		resto := strings.TrimSpace(strings.Trim(strings.TrimSpace(pi.texto[indiceQuien(pi.texto):]), "¿?."))
		res.Respuestas = append(res.Respuestas, quien+" "+resto)
	}
	return res, nil
}

// indiceQuien is the byte offset just after "quién"/"quien" in the original text.
func indiceQuien(s string) int {
	for _, q := range []string{"Quién", "quién", "Quien", "quien", "QUIÉN", "QUIEN"} {
		if i := strings.Index(s, q); i >= 0 {
			return i + len(q)
		}
	}
	return 0
}

func mayusculaInicial(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return strings.ToUpper(string(r)) + strings.ToLower(s[n:])
}

// restriccionesPista turns one clue into constraints.
func restriccionesPista(texto string, mens []mencion, n int) ([]Restriccion, bool) {
	norma := " " + nucleo.Normalizar(strings.NewReplacer(",", " ", ".", " ", ";", " ").Replace(texto)) + " "
	ws := strings.Fields(norma)
	neg := strings.Contains(norma, " no ") || strings.Contains(norma, " nunca ")
	buscar := func(frases ...string) int {
		for _, f := range frases {
			if i := strings.Index(norma, " "+f+" "); i >= 0 {
				return len(strings.Fields(norma[:i+1]))
			}
		}
		return -1
	}
	iz := buscar("a la izquierda de", "a la izquierda del", "a la izquierda")
	de := buscar("a la derecha de", "a la derecha del", "a la derecha")
	lado := buscar("al lado de", "al lado del", "junto a", "junto al", "de al lado", "contigua a", "contigua", "vecina de", "vecino de", "pegada a")
	// fixed position
	pos := -1
	for i, w := range ws {
		if p, ok := ordinales[w]; ok && p < n {
			pos = p
		}
		if w == "ultima" {
			pos = n - 1
		}
		if (w == "medio" || w == "central") && n%2 == 1 {
			pos = n / 2
		}
		if w == "numero" && i+1 < len(ws) {
			var x int
			if _, err := fmt.Sscanf(ws[i+1], "%d", &x); err == nil && x >= 1 && x <= n {
				pos = x - 1
			} else if v, ok := numerosPalabra[ws[i+1]]; ok && v >= 1 && int(v) <= n {
				pos = int(v) - 1
			}
		}
	}
	if len(mens) == 0 {
		return nil, false
	}
	par := func(rel func(a, b int) bool) []Restriccion {
		a, b := mens[0].valor, mens[1].valor
		if neg {
			r := rel
			rel = func(x, y int) bool { return !r(x, y) }
		}
		return []Restriccion{{Alcance: []int{a, b}, Tipo: Binaria, Rel: func(v []int) bool { return rel(v[0], v[1]) }}}
	}
	ladoDe := func(idx int) (int, int, bool) {
		var antes, despues []mencion
		for _, m := range mens {
			if m.pos < idx {
				antes = append(antes, m)
			} else {
				despues = append(despues, m)
			}
		}
		if len(antes) != 1 || len(despues) != 1 {
			return 0, 0, false
		}
		return antes[0].valor, despues[0].valor, true
	}
	switch {
	case iz >= 0 && len(mens) == 2:
		a, b, ok := ladoDe(iz)
		if !ok {
			return nil, false
		}
		mens = []mencion{{valor: a}, {valor: b}}
		return par(func(x, y int) bool { return x+1 == y }), true
	case de >= 0 && len(mens) == 2:
		a, b, ok := ladoDe(de)
		if !ok {
			return nil, false
		}
		mens = []mencion{{valor: a}, {valor: b}}
		return par(func(x, y int) bool { return x == y+1 }), true
	case lado >= 0 && len(mens) == 2:
		return par(func(x, y int) bool { return x-y == 1 || y-x == 1 }), true
	case iz >= 0 || de >= 0 || lado >= 0:
		return nil, false
	case pos >= 0 && len(mens) == 1:
		v := mens[0].valor
		d := DominioRango(pos, pos)
		if neg {
			return []Restriccion{{Alcance: []int{v}, Tipo: Func, Rel: func(x []int) bool { return x[0] != pos }}}, true
		}
		return []Restriccion{{Alcance: []int{v}, Tipo: Func, Rel: func(x []int) bool { return d.Tiene(x[0]) }}}, true
	case pos < 0 && len(mens) >= 2:
		var out []Restriccion
		for k := 1; k < len(mens); k++ {
			a, b := mens[0].valor, mens[k].valor
			if a == b {
				continue
			}
			if neg {
				out = append(out, Restriccion{Alcance: []int{a, b}, Tipo: Binaria, Rel: func(v []int) bool { return v[0] != v[1] }})
			} else {
				out = append(out, Restriccion{Alcance: []int{a, b}, Tipo: Binaria, Rel: func(v []int) bool { return v[0] == v[1] }})
			}
		}
		return out, len(out) > 0
	case pos >= 0 && len(mens) >= 2:
		// "el inglés vive en la primera casa, que es roja": all in that position
		var out []Restriccion
		for _, m := range mens {
			out = append(out, Restriccion{Alcance: []int{m.valor}, Tipo: Func, Rel: func(x []int) bool { return x[0] == pos }})
		}
		return out, !neg
	}
	return nil, false
}

// Cebra solves a zebra puzzle from its clues; it returns the solution table and the clues it could
// not read (which it lists back to the user).
func Cebra(ctx context.Context, pistas []string) (nucleo.Tabla, []string, error) {
	r, err := ResolverCebra(ctx, pistas)
	return r.Tabla, r.NoEntendidas, err
}
