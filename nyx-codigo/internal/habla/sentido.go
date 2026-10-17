package habla

import (
	"math"
	"sort"
	"strings"
)

// Meaning vectors and relations between words. The ideas come from Nyx's Gnostos v5/v6: the vector
// of a word is made of its definition (not of its letters); a word without a definition that other
// definitions name inherits the average of their vectors; the polarity of a definition (gain or lose
// stems) separates opposites from neighbours of the same topic.

// gana and pierde are the stems of words that add or take away.
var (
	gana = tallos(`traer guardar quedar poner dar ganar crear abrir sumar juntar unir añadir agregar aumentar
		llenar encender recibir construir aprender recordar`)
	pierde = tallos(`perder quitar olvidar cerrar romper borrar restar sacar vaciar apagar dejar eliminar
		disminuir destruir gastar soltar tirar faltar`)
)

func tallos(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[tallo(w)] = true
	}
	return m
}

// vectorPropio: weight 2 for the word itself, 1 for each content word of its definition, 1.5 for the
// words of the dictionary it names and ½ for the words of their definitions (one level only).
func (h *Habla) vectorPropio(s *Significado) map[string]float64 {
	v := map[string]float64{"@" + clave(s.Palabra): 2}
	var nombrados []*Significado
	visto := map[*Significado]bool{s: true}
	for _, t := range tokenizar(s.Definicion) {
		if t.clase != 'w' || esVacia(t.pleg) || largo(t.pleg) < 3 {
			continue
		}
		v[tallo(t.pleg)]++
		if o := h.propioEstricto(t.pleg); o != nil && !visto[o] {
			visto[o] = true
			v["@"+clave(o.Palabra)] += 1.5
			nombrados = append(nombrados, o)
		}
	}
	for _, o := range nombrados {
		for _, t := range tokenizar(o.Definicion) {
			if t.clase == 'w' && !esVacia(t.pleg) && largo(t.pleg) >= 3 {
				v[tallo(t.pleg)] += 0.5
			}
		}
	}
	return v
}

// vector is the meaning vector of a word: its own, or the average of the definitions that name it
// (anclado); nil when it knows neither.
func (h *Habla) vector(palabra string) (map[string]float64, *Significado) {
	if s := h.propio(palabra); s != nil {
		return h.vectorPropio(s), s
	}
	des := h.anclado(palabra)
	if len(des) == 0 {
		return nil, nil
	}
	if len(des) > maxAnclas {
		des = des[:maxAnclas]
	}
	suma := map[string]float64{}
	for _, k := range des {
		for d, x := range h.vectorPropio(h.sig[k]) {
			suma[d] += x
		}
	}
	for d := range suma {
		suma[d] /= float64(len(des))
	}
	return suma, nil
}

// polaridad: +1 when the definition adds (traer, guardar, crear…), −1 when it takes away (perder,
// quitar, olvidar…), 0 otherwise or without a definition.
func polaridad(s *Significado) int {
	if s == nil {
		return 0
	}
	p := 0
	for _, t := range tokenizar(s.Definicion) {
		if t.clase != 'w' {
			continue
		}
		st := tallo(t.pleg)
		if gana[st] {
			p++
		}
		if pierde[st] {
			p--
		}
	}
	switch {
	case p > 0:
		return 1
	case p < 0:
		return -1
	}
	return 0
}

// Relacion is how two words relate by their meaning.
type Relacion struct {
	A, B         string
	Coseno       float64
	Etiqueta     string   // "afines" | "opuestos" | "vecinos de tema" | "sin relación" | "la misma palabra"
	Compartidas  []string // the words of their meanings that justify it (at most 5)
	PolaridadA   int
	PolaridadB   int
	SignificadoA Significado // Propio false when only anchored
	SignificadoB Significado
	Desconocidas []string // the words it does not know (ok=false)
}

// Relacion compares two words by their meaning vectors. ok=false (with Desconocidas) when it does not
// know what one of them means: it does not invent a relation.
func (h *Habla) Relacion(a, b string) (Relacion, bool) {
	r := Relacion{A: strings.TrimSpace(a), B: strings.TrimSpace(b)}
	if h == nil {
		return r, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	va, sa := h.vector(a)
	vb, sb := h.vector(b)
	if va == nil {
		r.Desconocidas = append(r.Desconocidas, r.A)
	}
	if vb == nil {
		r.Desconocidas = append(r.Desconocidas, r.B)
	}
	if len(r.Desconocidas) > 0 {
		return r, false
	}
	r.SignificadoA, r.SignificadoB = h.significadoDe(a, sa), h.significadoDe(b, sb)
	r.PolaridadA, r.PolaridadB = polaridad(sa), polaridad(sb)
	if sa != nil && sa == sb || clave(sinArticulo(a)) == clave(sinArticulo(b)) {
		r.Coseno, r.Etiqueta = 1, "la misma palabra"
		return r, true
	}
	dot, na, nb := 0.0, 0.0, 0.0
	type dim struct {
		d string
		x float64
	}
	var comunes []dim
	for d, x := range va {
		na += x * x
		if y, ok := vb[d]; ok {
			dot += x * y
			comunes = append(comunes, dim{d, x * y})
		}
	}
	for _, y := range vb {
		nb += y * y
	}
	if na > 0 && nb > 0 {
		r.Coseno = dot / math.Sqrt(na*nb)
	}
	sort.Slice(comunes, func(i, j int) bool {
		if comunes[i].x != comunes[j].x {
			return comunes[i].x > comunes[j].x
		}
		return comunes[i].d < comunes[j].d
	})
	h.indexar()
	visto := map[string]bool{}
	for _, c := range comunes {
		w := h.mostrar(c.d)
		if w == "" || visto[w] {
			continue
		}
		visto[w] = true
		r.Compartidas = append(r.Compartidas, w)
		if len(r.Compartidas) == 5 {
			break
		}
	}
	pa, pb := r.PolaridadA, r.PolaridadB
	switch {
	case r.Coseno > 0.1 && pa != 0 && pa == -pb:
		r.Etiqueta = "opuestos"
	case r.Coseno > 0.1 && pa != 0 && pa == pb:
		r.Etiqueta = "afines"
	case r.Coseno >= 0.45:
		r.Etiqueta = "afines"
	case r.Coseno > 0.1:
		r.Etiqueta = "vecinos de tema"
	default:
		r.Etiqueta = "sin relación"
	}
	return r, true
}

// mostrar turns a vector dimension into a word to show.
func (h *Habla) mostrar(d string) string {
	if strings.HasPrefix(d, "@") {
		if s := h.sig[d[1:]]; s != nil {
			return s.Palabra
		}
		return d[1:]
	}
	if w, ok := h.muestra[d]; ok {
		return w
	}
	return d
}

func (h *Habla) significadoDe(palabra string, s *Significado) Significado {
	if s != nil {
		return copiar(*s)
	}
	var nombres []string
	for i, k := range h.anclado(palabra) {
		if i == 6 {
			break
		}
		nombres = append(nombres, h.sig[k].Palabra)
	}
	return Significado{Palabra: strings.ToLower(sinArticulo(strings.TrimSpace(palabra))), Conceptos: nombres,
		Fuente: "anclada", Confianza: 0.3, Origen: "aparece en lo que significan " + listaComillas(nombres)}
}
