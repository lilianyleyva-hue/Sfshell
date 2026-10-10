package puzles

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// palabra is one word or number of a question.
type palabra struct {
	orig  string
	norma string
	num   int64
	esNum bool
	desde int
}

var numerosPalabra = map[string]int64{
	"cero": 0, "un": 1, "uno": 1, "una": 1, "dos": 2, "tres": 3, "cuatro": 4, "cinco": 5, "seis": 6, "siete": 7,
	"ocho": 8, "nueve": 9, "diez": 10, "once": 11, "doce": 12, "trece": 13, "catorce": 14, "quince": 15,
	"dieciseis": 16, "diecisiete": 17, "dieciocho": 18, "diecinueve": 19, "veinte": 20, "veintiuno": 21,
	"veintidos": 22, "veintitres": 23, "veinticuatro": 24, "veinticinco": 25, "treinta": 30, "cuarenta": 40,
	"cincuenta": 50, "sesenta": 60, "setenta": 70, "ochenta": 80, "noventa": 90, "cien": 100, "ciento": 100,
	"mil": 1000,
}

// palabrasDe splits a text into words and numbers ("1.000" and "1000" are numbers; "un"/"una" are
// numbers only when marcarUn is true).
func palabrasDe(texto string) []palabra {
	var out []palabra
	i := 0
	for i < len(texto) {
		r, n := utf8.DecodeRuneInString(texto[i:])
		switch {
		case unicode.IsDigit(r):
			j := i
			for j < len(texto) {
				c := texto[j]
				if c >= '0' && c <= '9' {
					j++
					continue
				}
				// thousands separator: 1.000 / 1,000
				if (c == '.' || c == ',') && esTresDigitos(texto[j+1:]) {
					j++
					continue
				}
				break
			}
			s := strings.NewReplacer(".", "", ",", "").Replace(texto[i:j])
			v, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				out = append(out, palabra{orig: texto[i:j], norma: s, num: v, esNum: true, desde: i})
			}
			i = j
		case unicode.IsLetter(r):
			j := i + n
			for j < len(texto) {
				r2, n2 := utf8.DecodeRuneInString(texto[j:])
				if !unicode.IsLetter(r2) {
					break
				}
				j += n2
			}
			w := texto[i:j]
			p := palabra{orig: w, norma: nucleo.Normalizar(w), desde: i}
			if v, ok := numerosPalabra[p.norma]; ok {
				p.num, p.esNum = v, true
			}
			out = append(out, p)
			i = j
		default:
			if !unicode.IsSpace(r) {
				out = append(out, palabra{orig: string(r), norma: string(r), desde: i})
			}
			i += n
		}
	}
	return out
}

func esTresDigitos(s string) bool {
	if len(s) < 3 {
		return false
	}
	for k := 0; k < 3; k++ {
		if s[k] < '0' || s[k] > '9' {
			return false
		}
	}
	return len(s) == 3 || s[3] < '0' || s[3] > '9'
}

// numeros returns the numbers of the text in order; "un"/"una" count only when nothing else does.
func numeros(ps []palabra) []int64 {
	var out []int64
	for _, p := range ps {
		if p.esNum && p.norma != "un" && p.norma != "una" && p.norma != "uno" {
			out = append(out, p.num)
		}
	}
	return out
}

type lectura struct {
	ps    []palabra
	texto string // " w1 w2 … " normalized
}

func nuevaLectura(t string) lectura {
	ps := palabrasDe(t)
	var sb strings.Builder
	sb.WriteByte(' ')
	for _, p := range ps {
		sb.WriteString(p.norma)
		sb.WriteByte(' ')
	}
	return lectura{ps: ps, texto: sb.String()}
}

func (l lectura) tiene(fs ...string) bool {
	for _, f := range fs {
		if strings.Contains(l.texto, " "+f+" ") {
			return true
		}
	}
	return false
}

// tieneRaiz matches words by prefix ("anagrama" matches "anagramas").
func (l lectura) tieneRaiz(prefijos ...string) bool {
	for _, p := range l.ps {
		for _, x := range prefijos {
			if strings.HasPrefix(p.norma, x) {
				return true
			}
		}
	}
	return false
}

// indice is the position of the first word equal to w (−1 if none).
func (l lectura) indice(w string, desde int) int {
	for i := desde; i < len(l.ps); i++ {
		if l.ps[i].norma == w {
			return i
		}
	}
	return -1
}

// numeroTras is the first number after position i (skipping up to 3 words).
func (l lectura) numeroTras(i int) (int64, int, bool) {
	for j := i + 1; j < len(l.ps) && j <= i+4; j++ {
		if l.ps[j].esNum && l.ps[j].norma != "un" && l.ps[j].norma != "una" {
			return l.ps[j].num, j, true
		}
	}
	return 0, -1, false
}

var sinRepetir = []string{"sin repetir", "sin repeticion", "no se repiten", "no se pueden repetir", "no pueden repetirse",
	"distintos", "distintas", "diferentes", "todos distintos", "sin que se repitan", "no se repita", "no se repite"}
var conRepetir = []string{"con repeticion", "pueden repetirse", "se pueden repetir", "se repiten", "puede repetirse", "repitiendo"}

func digitos() []string {
	var out []string
	for d := 0; d <= 9; d++ {
		out = append(out, strconv.Itoa(d))
	}
	return out
}

func simbolos(prefijo string, n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("%s %d", prefijo, i))
	}
	return out
}

const letrasEspanol = "ABCDEFGHIJKLMNÑOPQRSTUVWXYZ"

// LeerConteo reads a counting question into a model and a Spanish reading of it. It knows the cues
// of §4.11: ordenar/sentar/en fila (permutations), mesa redonda (circular), elegir/grupos/comités
// (unordered), con repetición, anagramas de W, contraseñas/números de k dígitos, al menos uno,
// repartir n idénticos entre k, subir n escalones de 1 o 2, caminos en una cuadrícula a×b and
// ranges "del A al B" with divisibility, parity, primes and digits.
func LeerConteo(p *nucleo.Pregunta) (Modelo, string, error) {
	texto := p.Texto
	l := nuevaLectura(texto)
	nums := numeros(l.ps)
	noEntiendo := fmt.Errorf("puzles: no sé qué hay que contar: %w", nucleo.ErrNoEntiendo)
	repite := l.tiene(conRepetir...)
	noRepite := l.tiene(sinRepetir...)

	// anagrams
	if l.tieneRaiz("anagrama") {
		w := palabraAnagrama(texto, l)
		if w == "" {
			return Modelo{}, "", noEntiendo
		}
		m := Modelo{Ordenado: true}
		idx := map[rune]int{}
		for _, r := range w {
			if i, ok := idx[r]; ok {
				m.Multiconj[i]++
				continue
			}
			idx[r] = len(m.Alfabeto)
			m.Alfabeto = append(m.Alfabeto, string(r))
			m.Multiconj = append(m.Multiconj, 1)
		}
		m.Longitud = utf8.RuneCountInString(w)
		var reps []string
		for i, c := range m.Multiconj {
			if c > 1 {
				reps = append(reps, fmt.Sprintf("la %s %d veces", m.Alfabeto[i], c))
			}
		}
		lect := fmt.Sprintf("anagramas de %s: ordenar sus %d letras", w, m.Longitud)
		if len(reps) > 0 {
			lect += " (con " + strings.Join(reps, ", ") + ")"
		}
		return m, lect, nil
	}
	// stairs
	if l.tieneRaiz("escalon", "escalera", "peldaño") {
		n, ok := numeroAntes(l, "escalones", "peldaños", "escalon", "escaleras")
		if !ok && len(nums) > 0 {
			n, ok = nums[0], true
		}
		if !ok || n < 0 || n > 200 {
			return Modelo{}, "", noEntiendo
		}
		pasos := []int{1, 2}
		vistos := map[int]bool{}
		var otros []int
		pasado := false
		for _, p := range l.ps {
			if strings.HasPrefix(p.norma, "escal") || strings.HasPrefix(p.norma, "pelda") {
				pasado = true
				continue
			}
			if pasado && p.esNum && p.norma != "un" && p.norma != "una" && p.num > 0 && p.num <= 10 && !vistos[int(p.num)] {
				vistos[int(p.num)] = true
				otros = append(otros, int(p.num))
			}
		}
		if len(otros) >= 2 {
			sort.Ints(otros)
			pasos = otros
		}
		m := Modelo{Escalones: pasos, Longitud: int(n), Ordenado: true}
		var ps []string
		for _, x := range pasos {
			ps = append(ps, strconv.Itoa(x))
		}
		return m, fmt.Sprintf("subir %d escalones dando pasos de %s", n, strings.Join(ps, " o ")), nil
	}
	// grid paths
	if (l.tieneRaiz("cuadricula", "rejilla", "cuadriculado", "tablero", "malla")) && l.tieneRaiz("camino", "ruta", "forma", "manera") {
		if len(nums) >= 2 {
			a, b := int(nums[0]), int(nums[1])
			if a >= 0 && b >= 0 && a+b <= 40 {
				m := Modelo{Alfabeto: []string{"derecha", "abajo"}, Multiconj: []int{a, b}, Longitud: a + b, Ordenado: true}
				return m, fmt.Sprintf("caminos en una cuadrícula %d×%d: ordenar %d pasos a la derecha y %d hacia abajo", a, b, a, b), nil
			}
		}
		return Modelo{}, "", noEntiendo
	}
	// ranges "del 1 al 1000"
	if a, b, ok := leerRango(l); ok && l.tieneRaiz("cuant") {
		m := Modelo{Rango: [2]int64{a, b}}
		var conds []string
		if divs, union, ok := leerDivisores(l); ok {
			if union || len(divs) == 1 {
				m.Divisores = divs
				var ds []string
				for _, d := range divs {
					ds = append(ds, strconv.FormatInt(d, 10))
				}
				conds = append(conds, "divisibles por "+strings.Join(ds, " o por "))
			} else {
				l2 := int64(1)
				var ds []string
				for _, d := range divs {
					l2 = mcm(l2, d)
					ds = append(ds, strconv.FormatInt(d, 10))
				}
				m.Divisores = []int64{l2}
				conds = append(conds, "divisibles por "+strings.Join(ds, " y por ")+fmt.Sprintf(" (es decir, por %d)", l2))
			}
		}
		if l.tiene("pares") && len(m.Divisores) == 0 {
			m.Divisores = []int64{2}
			conds = append(conds, "pares")
		}
		if l.tiene("impares") {
			m.Filtros = append(m.Filtros, Filtro{Nombre: "impar", F: func(s []int) bool { return s[0]%2 != 0 }})
			conds = append(conds, "impares")
		}
		if l.tieneRaiz("primo") {
			m.Filtros = append(m.Filtros, Filtro{Nombre: "primo", F: func(s []int) bool { return esPrimo(int64(s[0])) }})
			conds = append(conds, "primos")
		}
		if l.tieneRaiz("cuadrados") && l.tiene("perfectos", "cuadrados") && !l.tieneRaiz("cuadricula") {
			m.Filtros = append(m.Filtros, Filtro{Nombre: "cuadrado perfecto", F: func(s []int) bool { return esCuadrado(int64(s[0])) }})
			conds = append(conds, "cuadrados perfectos")
		}
		if d, ok := leerDigito(l); ok {
			ds := strconv.Itoa(d)
			m.Filtros = append(m.Filtros, Filtro{Nombre: "contiene el " + ds, F: func(s []int) bool { return strings.Contains(strconv.Itoa(s[0]), ds) }})
			conds = append(conds, "que contienen el dígito "+ds)
		}
		lect := fmt.Sprintf("números del %d al %d", a, b)
		if len(conds) > 0 {
			lect += " " + strings.Join(conds, ", ")
		}
		return m, lect, nil
	}
	// round table
	if l.tiene("mesa redonda", "en circulo", "en un circulo", "en corro", "alrededor de una mesa", "en una rueda") {
		if len(nums) == 0 {
			return Modelo{}, "", noEntiendo
		}
		n := int(maxNum(nums))
		k := n
		if len(nums) >= 2 {
			k = int(minNum(nums))
		}
		m := Modelo{Alfabeto: simbolos("persona", n), Longitud: k, Ordenado: true, Circular: true}
		return m, fmt.Sprintf("sentar %d personas alrededor de una mesa redonda (los giros dan lo mismo)", n), nil
	}
	// share n identical things among k
	if l.tieneRaiz("repart", "distribu") && l.tiene("entre") {
		i := l.indice("entre", 0)
		k, _, ok1 := l.numeroTras(i)
		var n int64
		ok2 := false
		for _, p := range l.ps[:i] {
			if p.esNum && p.norma != "un" && p.norma != "una" {
				n, ok2 = p.num, true
			}
		}
		if !ok1 || !ok2 || k <= 0 || n < 0 || k > 30 || n > 200 {
			return Modelo{}, "", noEntiendo
		}
		m := Modelo{Alfabeto: simbolos("persona", int(k)), Longitud: int(n), Repeticion: true}
		lect := fmt.Sprintf("repartir %d cosas iguales entre %d (no importa el orden; cada cosa va a uno)", n, k)
		if l.tiene("al menos uno", "al menos una", "al menos 1", "ninguno se quede sin", "nadie se quede sin", "todos reciban") {
			for s := 0; s < int(k); s++ {
				m.Contiene = append(m.Contiene, s)
			}
			lect += ", y cada uno recibe al menos uno"
		}
		return m, lect, nil
	}
	// passwords, PINs, numbers of k digits, words of k letters
	if k, unidad, ok := leerLongitud(l); ok {
		m := Modelo{Longitud: int(k), Ordenado: true, Repeticion: !noRepite}
		lect := ""
		switch unidad {
		case "letras":
			m.Alfabeto = strings.Split(letrasEspanol, "")
			lect = fmt.Sprintf("palabras de %d letras (27 letras)", k)
		default:
			m.Alfabeto = digitos()
			lect = fmt.Sprintf("secuencias de %d cifras (del 0 al 9)", k)
			if l.tieneRaiz("numero") && !l.tieneRaiz("contraseña", "clave", "pin", "codigo", "matricula", "combinacion", "candado", "telefono") {
				m.SinCeroInicial = true
				lect = fmt.Sprintf("números de %d cifras (sin empezar por 0)", k)
			}
		}
		if noRepite {
			lect += ", sin repetir"
		} else {
			lect += ", pudiendo repetir"
		}
		if d, ok := leerAlMenos(l); ok && unidad != "letras" {
			m.Contiene = []int{d}
			lect += fmt.Sprintf(", con al menos un %d", d)
		}
		return m, lect, nil
	}
	ordenado := l.tieneRaiz("orden", "sient", "sentar", "coloc", "fila", "permut", "variacion", "podio", "cargo", "presidente", "puesto", "alinea", "cola")
	if l.tiene("no importa el orden", "sin importar el orden", "da igual el orden") {
		ordenado = false
	}
	elegir := l.tieneRaiz("elij", "elegi", "elige", "escog", "seleccion", "combinacion", "grupo", "comite", "equipo", "subconjunto", "comision", "delegacion", "toma", "sac")
	if len(nums) == 0 || (!ordenado && !elegir) {
		return Modelo{}, "", noEntiendo
	}
	n := int(maxNum(nums))
	k := n
	if len(nums) >= 2 {
		k = int(minNum(nums))
	}
	if n <= 0 || n > 1000 || k < 0 {
		return Modelo{}, "", noEntiendo
	}
	cosa := "elementos"
	m := Modelo{Alfabeto: simbolos("elemento", n), Longitud: k, Ordenado: ordenado, Repeticion: repite}
	switch {
	case ordenado && k == n && !repite:
		return m, fmt.Sprintf("ordenar %d %s (importa el orden)", n, cosa), nil
	case ordenado:
		lect := fmt.Sprintf("elegir %d de %d en orden", k, n)
		if repite {
			lect += ", pudiendo repetir"
		}
		return m, lect, nil
	}
	lect := fmt.Sprintf("elegir %d de %d sin importar el orden", k, n)
	if repite {
		lect += ", pudiendo repetir"
	}
	return m, lect, nil
}

func maxNum(xs []int64) int64 {
	m := xs[0]
	for _, x := range xs {
		m = max(m, x)
	}
	return m
}

func minNum(xs []int64) int64 {
	m := xs[0]
	for _, x := range xs {
		m = min(m, x)
	}
	return m
}

func numeroAntes(l lectura, ws ...string) (int64, bool) {
	for i, p := range l.ps {
		for _, w := range ws {
			if p.norma == w && i > 0 && l.ps[i-1].esNum {
				return l.ps[i-1].num, true
			}
		}
	}
	return 0, false
}

// palabraAnagrama finds the word whose anagrams are asked: a quoted or all-capitals word, or the
// word after "anagramas de (la palabra)" / "tiene".
func palabraAnagrama(texto string, l lectura) string {
	limpiar := func(w string) string {
		w = strings.ToUpper(nucleo.Normalizar(w))
		var sb strings.Builder
		for _, r := range w {
			if unicode.IsLetter(r) {
				sb.WriteRune(unicode.ToUpper(r))
			}
		}
		return sb.String()
	}
	for _, q := range []string{"«", "\"", "“", "'"} {
		if i := strings.Index(texto, q); i >= 0 {
			cierre := map[string]string{"«": "»", "\"": "\"", "“": "”", "'": "'"}[q]
			if j := strings.Index(texto[i+len(q):], cierre); j > 0 {
				return limpiar(texto[i+len(q) : i+len(q)+j])
			}
		}
	}
	for _, p := range l.ps {
		if utf8.RuneCountInString(p.orig) >= 2 && strings.ToUpper(p.orig) == p.orig && unicode.IsLetter([]rune(p.orig)[0]) {
			return limpiar(p.orig)
		}
	}
	vacias := map[string]bool{"la": true, "palabra": true, "el": true, "nombre": true, "de": true, "tiene": true, "hay": true, "se": true, "pueden": true, "formar": true, "con": true}
	for i, p := range l.ps {
		if strings.HasPrefix(p.norma, "anagrama") {
			for j := i + 1; j < len(l.ps); j++ {
				if !vacias[l.ps[j].norma] && unicode.IsLetter([]rune(l.ps[j].orig)[0]) {
					return limpiar(l.ps[j].orig)
				}
			}
		}
	}
	return ""
}

// leerRango finds "del A al B", "de A a B", "entre A y B".
func leerRango(l lectura) (int64, int64, bool) {
	for i := 0; i+3 < len(l.ps); i++ {
		a, b, c, d := l.ps[i], l.ps[i+1], l.ps[i+2], l.ps[i+3]
		if (a.norma == "del" || a.norma == "de" || a.norma == "desde") && b.esNum && (c.norma == "al" || c.norma == "a" || c.norma == "hasta") && d.esNum {
			return min(b.num, d.num), max(b.num, d.num), true
		}
		if a.norma == "entre" && b.esNum && (c.norma == "y" || c.norma == "e") && d.esNum {
			return min(b.num, d.num), max(b.num, d.num), true
		}
	}
	return 0, 0, false
}

// leerDivisores reads "divisibles por 3 o (por) 5" / "múltiplos de 3 y de 5".
func leerDivisores(l lectura) ([]int64, bool, bool) {
	for i, p := range l.ps {
		if !(strings.HasPrefix(p.norma, "divisible") || strings.HasPrefix(p.norma, "multiplo")) {
			continue
		}
		var divs []int64
		union := true
		for j := i + 1; j < len(l.ps); j++ {
			q := l.ps[j]
			switch {
			case q.esNum && q.norma != "un" && q.norma != "una":
				if q.num > 0 {
					divs = append(divs, q.num)
				}
			case q.norma == "por" || q.norma == "de" || q.norma == "entre":
			case q.norma == "o" || q.norma == "u" || q.norma == ",":
				union = true
			case q.norma == "y" || q.norma == "e":
				union = false
			case q.norma == "a" && j+1 < len(l.ps) && l.ps[j+1].norma == "la":
				j++
			case q.norma == "vez" || q.norma == "la" || q.norma == "ambos":
			default:
				j = len(l.ps)
			}
		}
		if len(divs) > 0 {
			return divs, union, true
		}
	}
	return nil, false, false
}

// leerDigito reads "contienen el (dígito|número) 7" / "tienen algún 7" / "con el 7".
func leerDigito(l lectura) (int, bool) {
	for i, p := range l.ps {
		if strings.HasPrefix(p.norma, "contien") || p.norma == "tienen" || p.norma == "llevan" || p.norma == "con" {
			if v, _, ok := l.numeroTras(i); ok && v >= 0 && v <= 9 {
				if i+1 < len(l.ps) && (l.ps[i+1].norma == "el" || l.ps[i+1].norma == "algun" || l.ps[i+1].norma == "un" || strings.HasPrefix(l.ps[i+1].norma, "digito") || strings.HasPrefix(l.ps[i+1].norma, "cifra")) {
					return int(v), true
				}
			}
		}
	}
	return 0, false
}

// leerLongitud reads "de k dígitos/cifras/letras" for passwords, PINs and numbers.
func leerLongitud(l lectura) (int64, string, bool) {
	for i, p := range l.ps {
		var unidad string
		switch {
		case strings.HasPrefix(p.norma, "digito") || strings.HasPrefix(p.norma, "cifra") || strings.HasPrefix(p.norma, "numero") && i > 0 && l.ps[i-1].esNum && l.tieneRaiz("contraseña", "clave", "pin", "codigo", "candado"):
			unidad = "digitos"
		case strings.HasPrefix(p.norma, "letra"):
			unidad = "letras"
		default:
			continue
		}
		if i > 0 && l.ps[i-1].esNum && l.ps[i-1].num > 0 && l.ps[i-1].num <= 12 {
			return l.ps[i-1].num, unidad, true
		}
	}
	return 0, "", false
}

// leerAlMenos reads "al menos un 7".
func leerAlMenos(l lectura) (int, bool) {
	for i := 0; i+2 < len(l.ps); i++ {
		if l.ps[i].norma == "al" && l.ps[i+1].norma == "menos" {
			if v, _, ok := l.numeroTras(i + 1); ok && v >= 0 && v <= 9 {
				return int(v), true
			}
		}
	}
	return 0, false
}

func esPrimo(n int64) bool {
	if n < 2 {
		return false
	}
	if n%2 == 0 {
		return n == 2
	}
	for d := int64(3); d*d <= n; d += 2 {
		if n%d == 0 {
			return false
		}
	}
	return true
}

func esCuadrado(n int64) bool {
	if n < 0 {
		return false
	}
	r := int64(0)
	for r*r < n {
		r++
	}
	return r*r == n
}

// --- probability

// LeerProbabilidad reads "probabilidad de k caras en n monedas", "… al menos k caras …", "suma s con
// dos dados", "sacar un 6 con un dado", "al menos un 6 con 4 dados". It returns the space of equally
// likely outcomes, the success filter, a reading and, when there is one, a closed formula.
func LeerProbabilidad(p *nucleo.Pregunta) (Modelo, Filtro, string, *big.Rat, error) {
	l := nuevaLectura(p.Texto)
	noEntiendo := fmt.Errorf("puzles: no sé qué probabilidad calcular: %w", nucleo.ErrNoEntiendo)
	nums := numeros(l.ps)
	alMenos := l.tiene("al menos", "como minimo", "por lo menos")
	comoMucho := l.tiene("como mucho", "como maximo", "a lo sumo")
	if l.tieneRaiz("moneda") {
		n, ok := numeroAntes(l, "monedas", "moneda", "veces", "lanzamientos", "tiradas")
		if !ok {
			n = 1
			if len(nums) >= 2 {
				n = maxNum(nums)
			}
		}
		lado := "cara"
		if l.tieneRaiz("cruz", "cruces", "sello") && !l.tieneRaiz("cara") {
			lado = "cruz"
		}
		k, ok := numeroAntes(l, "caras", "cara", "cruces", "cruz", "sellos")
		if !ok {
			k = 1
		}
		ladoTxt := lado
		if k != 1 {
			ladoTxt = map[string]string{"cara": "caras", "cruz": "cruces"}[lado]
		}
		if n <= 0 || n > 20 || k < 0 || k > n {
			return Modelo{}, Filtro{}, "", nil, noEntiendo
		}
		m := Modelo{Alfabeto: []string{"cara", "cruz"}, Longitud: int(n), Ordenado: true, Repeticion: true}
		obj := 0
		if lado == "cruz" {
			obj = 1
		}
		kk := int(k)
		cuenta := func(s []int) int {
			c := 0
			for _, x := range s {
				if x == obj {
					c++
				}
			}
			return c
		}
		f := Filtro{Nombre: fmt.Sprintf("exactamente %d %s", k, ladoTxt), F: func(s []int) bool { return cuenta(s) == kk }}
		formula := new(big.Rat)
		total := potencia(2, int(n))
		acumular := func(i int) { formula.Add(formula, new(big.Rat).SetFrac(combinaciones(int(n), i), total)) }
		switch {
		case alMenos:
			f = Filtro{Nombre: fmt.Sprintf("al menos %d %s", k, ladoTxt), F: func(s []int) bool { return cuenta(s) >= kk }}
			for i := kk; i <= int(n); i++ {
				acumular(i)
			}
		case comoMucho:
			f = Filtro{Nombre: fmt.Sprintf("como mucho %d %s", k, ladoTxt), F: func(s []int) bool { return cuenta(s) <= kk }}
			for i := 0; i <= kk; i++ {
				acumular(i)
			}
		default:
			acumular(kk)
		}
		return m, f, fmt.Sprintf("lanzar %d monedas: %s", n, f.Nombre), formula, nil
	}
	if l.tieneRaiz("dado") {
		n, ok := numeroAntes(l, "dados", "dado")
		if !ok {
			n = 1
			if l.tiene("dos dados") {
				n = 2
			}
		}
		if n <= 0 || n > 8 {
			return Modelo{}, Filtro{}, "", nil, noEntiendo
		}
		m := Modelo{Alfabeto: []string{"1", "2", "3", "4", "5", "6"}, Longitud: int(n), Ordenado: true, Repeticion: true}
		suma := func(s []int) int {
			t := 0
			for _, x := range s {
				t += x + 1
			}
			return t
		}
		total := potencia(6, int(n))
		// distribution of sums by DP (independent of the brute-force count)
		dist := map[int]*big.Int{0: big.NewInt(1)}
		for i := 0; i < int(n); i++ {
			nd := map[int]*big.Int{}
			for s, c := range dist {
				for cara := 1; cara <= 6; cara++ {
					if nd[s+cara] == nil {
						nd[s+cara] = new(big.Int)
					}
					nd[s+cara].Add(nd[s+cara], c)
				}
			}
			dist = nd
		}
		if l.tieneRaiz("suma", "sumen", "sume", "total") {
			var s int64 = -1
			for i, p := range l.ps {
				if strings.HasPrefix(p.norma, "sum") || p.norma == "total" {
					if v, _, ok := l.numeroTras(i); ok {
						s = v
						break
					}
				}
			}
			if s < 0 {
				for _, x := range nums {
					if x != n {
						s = x
					}
				}
			}
			if s < 0 {
				return Modelo{}, Filtro{}, "", nil, noEntiendo
			}
			ss := int(s)
			f := Filtro{Nombre: fmt.Sprintf("que la suma sea %d", s), F: func(x []int) bool { return suma(x) == ss }}
			formula := new(big.Rat)
			if c := dist[ss]; c != nil {
				formula.SetFrac(c, total)
			}
			if alMenos {
				f = Filtro{Nombre: fmt.Sprintf("que la suma sea al menos %d", s), F: func(x []int) bool { return suma(x) >= ss }}
				formula = new(big.Rat)
				for v, c := range dist {
					if v >= ss {
						formula.Add(formula, new(big.Rat).SetFrac(c, total))
					}
				}
			}
			return m, f, fmt.Sprintf("tirar %d dados: %s", n, f.Nombre), formula, nil
		}
		// a given face: "sacar un 6", "al menos un 6"
		var cara int64 = -1
		for i, p := range l.ps {
			if (p.norma == "un" || p.norma == "el" || p.norma == "algun") && i+1 < len(l.ps) && l.ps[i+1].esNum && l.ps[i+1].num >= 1 && l.ps[i+1].num <= 6 {
				cara = l.ps[i+1].num
			}
		}
		if cara < 0 {
			for _, x := range nums {
				if x >= 1 && x <= 6 && x != n {
					cara = x
				}
			}
		}
		if cara < 0 {
			return Modelo{}, Filtro{}, "", nil, noEntiendo
		}
		obj := int(cara) - 1
		tiene := func(s []int) bool {
			for _, x := range s {
				if x == obj {
					return true
				}
			}
			return false
		}
		f := Filtro{Nombre: fmt.Sprintf("al menos un %d", cara), F: tiene}
		// 1 − (5/6)^n
		sin := new(big.Rat).SetFrac(potencia(5, int(n)), total)
		formula := new(big.Rat).Sub(big.NewRat(1, 1), sin)
		if n == 1 {
			f.Nombre = fmt.Sprintf("sacar un %d", cara)
		}
		return m, f, fmt.Sprintf("tirar %d dados: %s", n, f.Nombre), formula, nil
	}
	return Modelo{}, Filtro{}, "", nil, noEntiendo
}
