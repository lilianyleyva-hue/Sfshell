package puzles

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// NombresColores are the names used for colours 0, 1, 2…
var NombresColores = []string{"rojo", "verde", "azul", "amarillo", "morado", "naranja", "rosa", "gris", "marrón", "negro"}

// Colorear colours the graph given by its edges with k colours so that neighbours differ (N2). The
// result maps every node to a colour 0..k−1 and is re-checked on every edge.
func Colorear(ctx context.Context, aristas [][2]string, k int) (map[string]int, error) {
	if k <= 0 || k > 64 {
		return nil, fmt.Errorf("puzles: el número de colores debe estar entre 1 y 64: %w", nucleo.ErrNoEntiendo)
	}
	indice := map[string]int{}
	p := &Problema{}
	nodo := func(s string) int {
		if i, ok := indice[s]; ok {
			return i
		}
		indice[s] = len(p.Nombres)
		p.Nombres = append(p.Nombres, s)
		p.Dom = append(p.Dom, DominioRango(0, k-1))
		return len(p.Nombres) - 1
	}
	for _, a := range aristas {
		x, y := nodo(a[0]), nodo(a[1])
		if x == y {
			return nil, fmt.Errorf("puzles: «%s» no puede ser vecino de sí mismo", a[0])
		}
		p.Restr = append(p.Restr, Restriccion{Alcance: []int{x, y}, Tipo: Binaria, Rel: func(v []int) bool { return v[0] != v[1] }})
	}
	if len(p.Nombres) == 0 {
		return nil, fmt.Errorf("puzles: no hay regiones que colorear: %w", nucleo.ErrNoEntiendo)
	}
	// symmetry breaking: the first node takes colour 0
	p.Dom[0] = DominioRango(0, 0)
	sols, _, err := Resolver(ctx, p, 1, nil)
	if err != nil && len(sols) == 0 {
		return nil, err
	}
	if len(sols) == 0 {
		return nil, fmt.Errorf("puzles: no se puede colorear con %d colores: %w", k, ErrSinColoreo)
	}
	out := map[string]int{}
	for nombre, i := range indice {
		out[nombre] = sols[0][i]
	}
	for _, a := range aristas {
		if out[a[0]] == out[a[1]] {
			return nil, fmt.Errorf("puzles: %s y %s quedaron del mismo color (error interno)", a[0], a[1])
		}
	}
	return out, nil
}

// ErrSinColoreo marks a graph that cannot be coloured with the given number of colours.
var ErrSinColoreo = errors.New("no hay coloreo posible")

// NumeroCromatico finds the fewest colours that work (trying 1, 2, … up to max).
func NumeroCromatico(ctx context.Context, aristas [][2]string, max int) (int, map[string]int, error) {
	for k := 1; k <= max; k++ {
		m, err := Colorear(ctx, aristas, k)
		if err == nil {
			return k, m, nil
		}
		if !esSinColoreo(err) {
			return 0, nil, err
		}
	}
	return 0, nil, fmt.Errorf("puzles: hacen falta más de %d colores: %w", max, ErrSinColoreo)
}

func esSinColoreo(err error) bool { return errors.Is(err, ErrSinColoreo) }

// --- reading the borders

// conectoresNombre may sit inside a region name ("Región de Murcia", "Castilla y León").
var conectoresNombre = map[string]bool{"de": true, "del": true, "la": true, "las": true, "los": true, "el": true, "y": true}

// palabrasIntro mark the "colorea este mapa con 3 colores:" part before the list.
var palabrasIntro = []string{"colore", "pint", "mapa", "grafo", "fronter", "vecin", "region", "arista", "conexion", "colores", "frontera"}

// esPalabraIntro: a verb of the request ("Colorea A-B…"), never part of a region name.
func esPalabraIntro(w string) bool {
	n := nucleo.Normalizar(w)
	for _, p := range []string{"colore", "pint", "dibuj", "resuelv"} {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

func esPalabraNombre(w string) bool {
	r, _ := utf8.DecodeRuneInString(w)
	return unicode.IsUpper(r) || unicode.IsDigit(r)
}

// recortarNombre keeps the region name next to the separator: the run of capitalized words (and
// connectors inside it) closest to it, or the single nearest word when nothing is capitalized.
// haciaAtras: the name is the end of izq ("colorea el grafo A" → "A").
func recortarNombre(texto string, haciaAtras bool) string {
	ws := strings.Fields(strings.Trim(strings.TrimSpace(texto), ".¿?¡!\"«»"))
	if len(ws) <= 1 {
		return strings.Join(ws, "")
	}
	orden := make([]int, len(ws))
	for i := range ws {
		if haciaAtras {
			orden[i] = len(ws) - 1 - i
		} else {
			orden[i] = i
		}
	}
	var run []int
	for _, i := range orden {
		w := ws[i]
		if esPalabraIntro(w) {
			break
		}
		if esPalabraNombre(w) || (conectoresNombre[strings.ToLower(w)] && len(run) > 0) {
			run = append(run, i)
			continue
		}
		break
	}
	// a name does not start or end with a connector
	for len(run) > 0 && conectoresNombre[strings.ToLower(ws[run[len(run)-1]])] {
		run = run[:len(run)-1]
	}
	if len(run) == 0 {
		return ws[orden[0]]
	}
	ini, fin := run[0], run[len(run)-1]
	if ini > fin {
		ini, fin = fin, ini
	}
	return strings.Join(ws[ini:fin+1], " ")
}

// partirArista splits one "A-B" item; it returns ok=false when there is no separator, and an error
// when there are several hyphens and it cannot tell where one name ends.
func partirArista(item string) ([][2]string, bool, error) {
	for _, sep := range []string{"<->", " – ", " — ", " - ", "–", "—"} {
		if i := strings.Index(item, sep); i >= 0 {
			a, b := recortarNombre(item[:i], true), recortarNombre(item[i+len(sep):], false)
			if a == "" || b == "" {
				return nil, false, nil
			}
			return [][2]string{{a, b}}, true, nil
		}
	}
	switch strings.Count(item, "-") {
	case 0:
		return nil, false, nil
	case 1:
		i := strings.Index(item, "-")
		a, b := recortarNombre(item[:i], true), recortarNombre(item[i+1:], false)
		if a == "" || b == "" {
			return nil, false, nil
		}
		return [][2]string{{a, b}}, true, nil
	}
	// "A-B B-C": every word with a hyphen is one border
	var out [][2]string
	for _, w := range strings.Fields(item) {
		w = strings.Trim(w, ".")
		switch strings.Count(w, "-") {
		case 0:
			continue
		case 1:
			i := strings.Index(w, "-")
			if i == 0 || i == len(w)-1 {
				return nil, false, errAmbigua(item)
			}
			out = append(out, [2]string{w[:i], w[i+1:]})
		default:
			return nil, false, errAmbigua(item)
		}
	}
	return out, len(out) > 0, nil
}

func errAmbigua(item string) error {
	return fmt.Errorf("puzles: no sé dónde acaba cada región en «%s»: escribe un guion con espacios entre las dos regiones (por ejemplo «Madrid - Castilla-La Mancha»): %w", strings.TrimSpace(item), nucleo.ErrNoEntiendo)
}

// partirLista splits a list on commas and semicolons, and on " y " / " e " when both sides are
// borders ("A-B y B-C", "Galicia-Castilla y León y Asturias-León") or when the list is of plain
// names (adjacency "A: B, C y D").
func partirLista(texto string, nombres bool) []string {
	var out []string
	for _, parte := range strings.FieldsFunc(texto, func(r rune) bool { return r == ',' || r == ';' }) {
		for _, t := range partirY(parte, nombres) {
			if t = strings.TrimSpace(strings.Trim(strings.TrimSpace(t), ".")); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// partirY splits t at an " y "/" e " that separates two borders: both sides have a separator and the
// right side reaches its separator without another "y" (so "Castilla y León" stays one name).
func partirY(t string, nombres bool) []string {
	esSep := func(r rune) bool { return r == '-' || r == '–' || r == '—' }
	for i := 0; i+3 <= len(t); i++ {
		if t[i] != ' ' || !(strings.HasPrefix(t[i:], " y ") || strings.HasPrefix(t[i:], " e ")) {
			continue
		}
		izq, der := t[:i], t[i+3:]
		if nombres {
			return append(partirY(izq, true), partirY(der, true)...)
		}
		j := strings.IndexFunc(der, esSep)
		if !strings.ContainsFunc(izq, esSep) || j < 0 {
			continue
		}
		antes := " " + der[:j] + " "
		if strings.Contains(antes, " y ") || strings.Contains(antes, " e ") {
			continue
		}
		return append(partirY(izq, false), partirY(der, false)...)
	}
	return []string{t}
}

// LeerAristas reads the borders of a map or graph: "A-B, B-C y C-A" (names may have several words,
// "Castilla y León - Madrid"), or adjacency lines "A: B, C". It returns an error when a border is
// ambiguous ("Madrid-Castilla-La Mancha").
func LeerAristas(texto string) ([][2]string, error) {
	var out [][2]string
	visto := map[[2]string]bool{}
	agregar := func(a, b string) {
		a, b = strings.TrimSpace(a), strings.TrimSpace(b)
		if a == "" || b == "" || a == b {
			return
		}
		k := [2]string{a, b}
		if b < a {
			k = [2]string{b, a}
		}
		if !visto[k] {
			visto[k] = true
			out = append(out, [2]string{a, b})
		}
	}
	tieneSep := func(s string) bool { return strings.ContainsAny(s, "-–—") }
	for _, linea := range strings.Split(texto, "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		cuerpo := linea
		if i := strings.Index(linea, ":"); i >= 0 {
			cabeza, resto := strings.TrimSpace(linea[:i]), linea[i+1:]
			intro := false
			for _, w := range palabrasIntro {
				intro = intro || strings.Contains(nucleo.Normalizar(cabeza), w)
			}
			if !intro && !tieneSep(cabeza) && !tieneSep(resto) && cabeza != "" && len(strings.Fields(cabeza)) <= 4 {
				for _, b := range partirLista(resto, true) {
					agregar(cabeza, b)
				}
				continue
			}
			cuerpo = resto
		}
		for _, item := range partirLista(cuerpo, false) {
			pares, ok, err := partirArista(item)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			for _, p := range pares {
				agregar(p[0], p[1])
			}
		}
	}
	return out, nil
}
