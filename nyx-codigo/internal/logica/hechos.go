package logica

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nyxcodigo/internal/nucleo"
)

// BaseHechos is the read side of the fact store (memoria implements it).
type BaseHechos interface {
	Hechos(sujeto string) []nucleo.Hecho
	Todos() []nucleo.Hecho
	Reglas() []nucleo.Regla
}

// Guardar is the write side of the fact store.
type Guardar interface {
	GuardarHecho(h nucleo.Hecho) (string, error)
	GuardarRegla(r nucleo.Regla) (string, error)
}

// Derivacion is a fact derived by forward chaining, with its provenance.
type Derivacion struct {
	Hecho nucleo.Hecho
	Desde []string // fact/rule ids
	Regla string
}

const (
	maxIterEncadenar = 1000
	maxDerivados     = 20000
)

// relacionNormal unifies the copula relations: "es", "es_una", "es_un" → "es_un".
func relacionNormal(r string) string {
	r = strings.ReplaceAll(nucleo.Normalizar(r), " ", "_")
	switch r {
	case "es", "es_un", "es_una", "ser", "son":
		return "es_un"
	}
	return r
}

func esVariable(s string) bool { return strings.HasPrefix(s, "?") }

func claveHecho(h nucleo.Hecho) string {
	return fmt.Sprintf("%s|%s|%s|%v", claveTermino(h.Sujeto), relacionNormal(h.Relacion), claveTermino(h.Objeto), h.Negado)
}

// DescribirHecho writes a fact in Spanish: "toby es perro", "toby no es gato",
// "«capicua» significa «palindromo»". The terms are the stored lemmas; see DescribirHechoCon.
func DescribirHecho(h nucleo.Hecho) string {
	return DescribirHechoCon(h, nil)
}

// DescribirHechoCon writes a fact using the words as the user wrote them when formas (see
// FormasEscritas) has them: "Toby es mamífero" instead of "toby es mamifero".
func DescribirHechoCon(h nucleo.Hecho, formas map[string]string) string {
	no := ""
	if h.Negado {
		no = "no "
	}
	s, o := formaBonita(h.Sujeto, formas), formaBonita(h.Objeto, formas)
	switch relacionNormal(h.Relacion) {
	case "es_un":
		return fmt.Sprintf("%s %ses %s", s, no, o)
	case "significa":
		return fmt.Sprintf("«%s» %ssignifica «%s»", s, no, o)
	case "tiene":
		return fmt.Sprintf("%s %stiene %s", s, no, o)
	}
	return fmt.Sprintf("%s %s%s %s", s, no, strings.ReplaceAll(h.Relacion, "_", " "), o)
}

// FormasEscritas collects, from the texts of the facts and rules (and any extra texts), how each
// word was written: normalized word → word as written ("toby" → "Toby", "mamiferos" → "mamíferos").
func FormasEscritas(b BaseHechos, extra ...string) map[string]string {
	formas := map[string]string{}
	meter := func(texto string) {
		for _, t := range tokenizar(texto) {
			if t.sim || t.norma == "" {
				continue
			}
			if _, ya := formas[t.norma]; !ya {
				formas[t.norma] = t.orig
			}
		}
	}
	if b != nil {
		for _, h := range b.Todos() {
			meter(h.Texto)
		}
		for _, r := range b.Reglas() {
			meter(r.Texto)
		}
	}
	for _, t := range extra {
		meter(t)
	}
	return formas
}

// formaBonita rewrites a stored term with the written forms: a word is replaced by how it was
// written, or by the singular of a written plural ("mamíferos" → "mamífero"). Names keep their
// capital; the rest is lowercased.
func formaBonita(termino string, formas map[string]string) string {
	if len(formas) == 0 || esVariable(termino) {
		return termino
	}
	var out []string
	for _, w := range strings.Fields(termino) {
		n := nucleo.Normalizar(w)
		switch {
		case formas[n] != "":
			out = append(out, formas[n])
		case formas[n+"s"] != "":
			f := formas[n+"s"]
			out = append(out, f[:len(f)-1])
		case formas[n+"es"] != "":
			f := formas[n+"es"]
			out = append(out, f[:len(f)-2])
		default:
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// DescribirRegla writes a rule in Spanish (its Texto when it has one).
func DescribirRegla(r nucleo.Regla) string {
	if r.Texto != "" {
		return r.Texto
	}
	if len(r.Si) == 1 && esVariable(r.Si[0].Sujeto) && relacionNormal(r.Si[0].Relacion) == "es_un" &&
		relacionNormal(r.Entonces.Relacion) == "es_un" && r.Entonces.Sujeto == r.Si[0].Sujeto {
		if r.Entonces.Negado {
			return fmt.Sprintf("ningún %s es %s", r.Si[0].Objeto, r.Entonces.Objeto)
		}
		return fmt.Sprintf("todo %s es %s", r.Si[0].Objeto, r.Entonces.Objeto)
	}
	var si []string
	for _, h := range r.Si {
		si = append(si, DescribirHecho(h))
	}
	return "si " + strings.Join(si, " y ") + ", entonces " + DescribirHecho(r.Entonces)
}

type emparejamiento struct {
	vars map[string]string
	ids  []string
}

// emparejar finds every way to match the patterns against the facts.
func emparejar(patrones []nucleo.Hecho, hechos []nucleo.Hecho) []emparejamiento {
	var out []emparejamiento
	var rec func(k int, vars map[string]string, ids []string)
	rec = func(k int, vars map[string]string, ids []string) {
		if len(out) > maxDerivados {
			return
		}
		if k == len(patrones) {
			cp := map[string]string{}
			for a, b := range vars {
				cp[a] = b
			}
			out = append(out, emparejamiento{vars: cp, ids: append([]string(nil), ids...)})
			return
		}
		p := patrones[k]
		rel := relacionNormal(p.Relacion)
		for _, h := range hechos {
			if relacionNormal(h.Relacion) != rel || h.Negado != p.Negado {
				continue
			}
			nuevas := map[string]string{}
			ok := true
			for _, par := range [][2]string{{p.Sujeto, h.Sujeto}, {p.Objeto, h.Objeto}} {
				pat, val := par[0], par[1]
				if esVariable(pat) {
					if v, ya := vars[pat]; ya {
						ok = ok && claveTermino(v) == claveTermino(val)
					} else if v, ya := nuevas[pat]; ya {
						ok = ok && claveTermino(v) == claveTermino(val)
					} else {
						nuevas[pat] = val
					}
				} else {
					ok = ok && claveTermino(pat) == claveTermino(val)
				}
			}
			if !ok {
				continue
			}
			for a, b := range nuevas {
				vars[a] = b
			}
			rec(k+1, vars, append(ids, h.ID))
			for a := range nuevas {
				delete(vars, a)
			}
		}
	}
	rec(0, map[string]string{}, nil)
	return out
}

func instanciar(h nucleo.Hecho, vars map[string]string) (nucleo.Hecho, bool) {
	out := h
	if esVariable(h.Sujeto) {
		v, ok := vars[h.Sujeto]
		if !ok {
			return out, false
		}
		out.Sujeto = v
	}
	if esVariable(h.Objeto) {
		v, ok := vars[h.Objeto]
		if !ok {
			return out, false
		}
		out.Objeto = v
	}
	return out, true
}

// reglasEfectivas adds the symmetric reading of "ningún A es B" rules (then "ningún B es A").
// The symmetric rule keeps the id of the stored rule (so provenance names real ids): several
// effective rules may share one id, and comprobarCadena tries each of them.
func reglasEfectivas(rs []nucleo.Regla) []nucleo.Regla {
	out := make([]nucleo.Regla, 0, len(rs))
	for i, r := range rs {
		if r.ID == "" {
			r.ID = fmt.Sprintf("r?%d", i+1)
		}
		out = append(out, r)
		if len(r.Si) == 1 && r.Entonces.Negado && !r.Si[0].Negado && esVariable(r.Si[0].Sujeto) &&
			r.Si[0].Sujeto == r.Entonces.Sujeto && relacionNormal(r.Si[0].Relacion) == "es_un" &&
			relacionNormal(r.Entonces.Relacion) == "es_un" && !esVariable(r.Si[0].Objeto) && !esVariable(r.Entonces.Objeto) {
			sim := r
			sim.Si = []nucleo.Hecho{{Sujeto: r.Si[0].Sujeto, Relacion: "es_un", Objeto: r.Entonces.Objeto}}
			sim.Entonces = nucleo.Hecho{Sujeto: r.Si[0].Sujeto, Relacion: "es_un", Objeto: r.Si[0].Objeto, Negado: true}
			out = append(out, sim)
		}
	}
	return out
}

// Encadenar runs forward chaining over the facts and rules to a fixpoint (≤ maxIter rounds, at
// most 1 000). Each derived fact gets an id "d1", "d2"… and records the ids it came from.
func Encadenar(ctx context.Context, b BaseHechos, maxIter int) []Derivacion {
	if b == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if maxIter <= 0 || maxIter > maxIterEncadenar {
		maxIter = maxIterEncadenar
	}
	var hechos []nucleo.Hecho
	conocidos := map[string]bool{}
	for i, h := range b.Todos() {
		if h.ID == "" {
			h.ID = fmt.Sprintf("h?%d", i+1)
		}
		hechos = append(hechos, h)
		conocidos[claveHecho(h)] = true
	}
	reglas := reglasEfectivas(b.Reglas())
	usados := map[string]bool{}
	for _, h := range hechos {
		usados[h.ID] = true
	}
	for _, r := range reglas {
		usados[r.ID] = true
	}
	siguiente := 0
	nuevoID := func() string {
		for {
			siguiente++
			id := fmt.Sprintf("d%d", siguiente)
			if !usados[id] {
				usados[id] = true
				return id
			}
		}
	}
	var ders []Derivacion
	for it := 0; it < maxIter && ctx.Err() == nil; it++ {
		nuevos := 0
		for _, r := range reglas {
			if len(r.Si) == 0 {
				continue
			}
			for _, m := range emparejar(r.Si, hechos) {
				h, ok := instanciar(r.Entonces, m.vars)
				if !ok {
					continue
				}
				k := claveHecho(h)
				if conocidos[k] {
					continue
				}
				conocidos[k] = true
				h.ID = nuevoID()
				h.Fuente = "deducido"
				h.Texto = DescribirHecho(h)
				h.Confianza = 1
				ders = append(ders, Derivacion{Hecho: h, Desde: append(append([]string(nil), m.ids...), r.ID), Regla: DescribirRegla(r)})
				hechos = append(hechos, h)
				nuevos++
				if len(ders) >= maxDerivados {
					return ders
				}
			}
		}
		if nuevos == 0 {
			break
		}
	}
	return ders
}

// Consultar answers a yes/no question about a fact: "si" when it is known or derived, "no" when the
// opposite (an explicit negated fact) is known or derived, and "no_se" otherwise. The chain lists
// the facts and rules used, in order ("toby es un perro", "todos los perros son mamíferos").
func Consultar(ctx context.Context, b BaseHechos, q nucleo.Hecho) (respuesta string, cadena []string) {
	if b == nil {
		return "no_se", nil
	}
	return nuevoConsultor(ctx, b).consultar(q)
}

// consultor holds one forward-chaining run and answers several questions over it.
type consultor struct {
	todos      []nucleo.Hecho
	hechoPorID map[string]nucleo.Hecho
	derPorID   map[string]Derivacion
	reglaPorID map[string]nucleo.Regla
	formas     map[string]string
	ders       []Derivacion
}

func nuevoConsultor(ctx context.Context, b BaseHechos) *consultor {
	c := &consultor{
		hechoPorID: map[string]nucleo.Hecho{},
		derPorID:   map[string]Derivacion{},
		reglaPorID: map[string]nucleo.Regla{},
		formas:     FormasEscritas(b),
	}
	c.ders = Encadenar(ctx, b, maxIterEncadenar)
	for i, h := range b.Todos() {
		if h.ID == "" {
			h.ID = fmt.Sprintf("h?%d", i+1)
		}
		c.hechoPorID[h.ID] = h
		c.todos = append(c.todos, h)
	}
	for _, d := range c.ders {
		c.derPorID[d.Hecho.ID] = d
		c.todos = append(c.todos, d.Hecho)
	}
	for _, r := range reglasEfectivas(b.Reglas()) {
		if _, ya := c.reglaPorID[r.ID]; !ya {
			c.reglaPorID[r.ID] = r
		}
	}
	return c
}

func (c *consultor) buscar(q nucleo.Hecho, neg bool) (nucleo.Hecho, bool) {
	x := q
	x.Negado = neg
	k := claveHecho(x)
	for _, h := range c.todos {
		if claveHecho(h) == k {
			return h, true
		}
	}
	return nucleo.Hecho{}, false
}

func (c *consultor) explicar(h nucleo.Hecho) []string {
	var lineas []string
	visto := map[string]bool{}
	var rec func(id string, final bool)
	rec = func(id string, final bool) {
		if visto[id] {
			return
		}
		visto[id] = true
		if d, ok := c.derPorID[id]; ok {
			for _, s := range d.Desde {
				rec(s, false)
			}
			if !final {
				lineas = append(lineas, "así que "+DescribirHechoCon(d.Hecho, c.formas))
			}
			return
		}
		if h, ok := c.hechoPorID[id]; ok {
			lineas = append(lineas, textoHechoBase(h))
			return
		}
		if r, ok := c.reglaPorID[id]; ok {
			lineas = append(lineas, DescribirRegla(r))
		}
	}
	rec(h.ID, true)
	if len(lineas) == 0 {
		lineas = append(lineas, textoHechoBase(h))
	}
	return lineas
}

func (c *consultor) consultar(q nucleo.Hecho) (string, []string) {
	if h, ok := c.buscar(q, q.Negado); ok {
		return "si", c.explicar(h)
	}
	if h, ok := c.buscar(q, !q.Negado); ok {
		return "no", c.explicar(h)
	}
	return "no_se", nil
}

func textoHechoBase(h nucleo.Hecho) string {
	t := strings.TrimSpace(h.Texto)
	if t == "" {
		t = DescribirHecho(h)
	}
	if strings.HasPrefix(h.Fuente, "http") {
		t += " (según " + h.Fuente + ")"
	}
	return t
}

var prefijosRecordar = [][]string{
	{"recuerda", "que"}, {"aprende", "que"}, {"ten", "en", "cuenta", "que"}, {"apunta", "que"}, {"guarda", "que"},
	{"memoriza", "que"}, {"has", "de", "saber", "que"}, {"debes", "saber", "que"}, {"que", "sepas", "que"},
	{"recuerda"}, {"aprende"},
}

// QuitarRecordar strips "recuerda que", "aprende que"… and reports whether it was there.
func QuitarRecordar(texto string) (string, bool) {
	ts := tokenizar(texto)
	r, ok := quitarPrefijo(ts, prefijosRecordar)
	if !ok || len(r) == 0 {
		return strings.TrimSpace(texto), false
	}
	return strings.TrimSpace(texto[r[0].desde:]), true
}

// LeerHecho reads one sentence as a fact or a rule: "Toby es un perro" → (toby, es_un, perro);
// "todos los perros son mamíferos" → ?x es_un perro ⇒ ?x es_un mamifero; "ningún gato es perro" →
// a negated rule; "X significa Y"; "X tiene Y". "recuerda que" is stripped first.
func LeerHecho(texto string, lem nucleo.Lematizador) (*nucleo.Hecho, *nucleo.Regla, error) {
	limpio, _ := QuitarRecordar(texto)
	limpio = strings.TrimRight(strings.TrimSpace(limpio), ".;!?")
	ahora := time.Now()
	ts := tokenizar(limpio)
	var ws []tok
	for _, t := range ts {
		if !t.sim {
			ws = append(ws, t)
		}
	}
	if len(ws) > 0 && interrogativos[ws[0].norma] {
		return nil, nil, fmt.Errorf("logica: «%s» es una pregunta abierta, no un hecho: %w", limpio, nucleo.ErrNoEntiendo)
	}
	for i, t := range ws {
		if i > 0 && i < len(ws)-1 && (t.norma == "significa" || (t.norma == "quiere" && i+2 < len(ws) && ws[i+1].norma == "decir")) {
			fin := i + 1
			if t.norma == "quiere" {
				fin = i + 2
			}
			suj := nucleo.Normalizar(textoToks(sinDeterminantes(ws[:i])))
			obj := nucleo.Normalizar(textoToks(sinDeterminantes(ws[fin:])))
			if suj == "" || obj == "" {
				break
			}
			h := nucleo.Hecho{Sujeto: suj, Relacion: "significa", Objeto: obj, Texto: limpio, Fuente: "usuario", Confianza: 1, Fecha: ahora}
			return &h, nil, nil
		}
	}
	for i, t := range ws {
		if i > 0 && i < len(ws)-1 && (t.norma == "tiene" || t.norma == "tienen") {
			neg := ws[i-1].norma == "no"
			fin := i
			if neg {
				fin = i - 1
			}
			suj := sinDeterminantes(ws[:fin])
			obj := sinDeterminantes(ws[i+1:])
			if len(suj) == 0 || len(obj) == 0 || len(suj) > 3 {
				break
			}
			if suj[0].norma == "todos" || suj[0].norma == "todas" || suj[0].norma == "todo" || suj[0].norma == "cada" {
				gen := sinDeterminantes(suj[1:])
				if len(gen) == 0 {
					break
				}
				r := nucleo.Regla{
					Si:       []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: lemaFrase(gen, lem)}},
					Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "tiene", Objeto: lemaFrase(obj, lem), Negado: neg},
					Texto:    limpio, Fuente: "usuario", Fecha: ahora,
				}
				return nil, &r, nil
			}
			h := nucleo.Hecho{Sujeto: nucleo.Normalizar(textoToks(suj)), Relacion: "tiene", Objeto: lemaFrase(obj, lem), Negado: neg,
				Texto: limpio, Fuente: "usuario", Confianza: 1, Fecha: ahora}
			return &h, nil, nil
		}
	}
	c, err := AnalizarCategorica(limpio, lem)
	if err != nil {
		return nil, nil, err
	}
	switch c.Cuant {
	case Individuo:
		h := nucleo.Hecho{Sujeto: c.Sujeto, Relacion: "es_un", Objeto: c.Predicado, Negado: c.Negada, Texto: limpio,
			Fuente: "usuario", Confianza: 1, Fecha: ahora}
		return &h, nil, nil
	case Todos, Ningun:
		r := nucleo.Regla{
			Si:       []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: c.Sujeto}},
			Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "es_un", Objeto: c.Predicado, Negado: c.Cuant == Ningun},
			Texto:    limpio, Fuente: "usuario", Fecha: ahora,
		}
		return nil, &r, nil
	}
	return nil, nil, fmt.Errorf("logica: solo recuerdo reglas con «todos» o «ningún», no con «algunos»: %w", nucleo.ErrNoSoportado)
}

// baseUnida overlays temporary facts and rules (from the question itself) on a stored base.
type baseUnida struct {
	b BaseHechos
	h []nucleo.Hecho
	r []nucleo.Regla
}

func (u baseUnida) Hechos(sujeto string) []nucleo.Hecho {
	var out []nucleo.Hecho
	k := claveTermino(sujeto)
	for _, h := range u.Todos() {
		if claveTermino(h.Sujeto) == k {
			out = append(out, h)
		}
	}
	return out
}

func (u baseUnida) Todos() []nucleo.Hecho {
	var out []nucleo.Hecho
	if u.b != nil {
		out = append(out, u.b.Todos()...)
	}
	return append(out, u.h...)
}

func (u baseUnida) Reglas() []nucleo.Regla {
	var out []nucleo.Regla
	if u.b != nil {
		out = append(out, u.b.Reglas()...)
	}
	return append(out, u.r...)
}

// comprobarCadena re-checks a "si"/"no" answer: every derived fact on the path must follow from its
// sources by its rule. It re-runs the chaining on a copy and verifies each derivation's sources exist.
func comprobarCadena(ctx context.Context, b BaseHechos, ders []Derivacion) bool {
	ids := map[string]nucleo.Hecho{}
	for i, h := range b.Todos() {
		if h.ID == "" {
			h.ID = fmt.Sprintf("h?%d", i+1)
		}
		ids[h.ID] = h
	}
	// several effective rules may share an id (a "ningún" rule and its symmetric reading)
	reglas := map[string][]nucleo.Regla{}
	for _, r := range reglasEfectivas(b.Reglas()) {
		reglas[r.ID] = append(reglas[r.ID], r)
	}
	for _, d := range ders {
		if len(d.Desde) == 0 {
			return false
		}
		var fuentes []nucleo.Hecho
		for _, id := range d.Desde[:len(d.Desde)-1] {
			h, ok := ids[id]
			if !ok {
				return false
			}
			fuentes = append(fuentes, h)
		}
		encontrado := false
		for _, r := range reglas[d.Desde[len(d.Desde)-1]] {
			if len(r.Si) != len(fuentes) {
				continue
			}
			for _, m := range emparejar(r.Si, fuentes) {
				if h, ok := instanciar(r.Entonces, m.vars); ok && claveHecho(h) == claveHecho(d.Hecho) {
					encontrado = true
					break
				}
			}
			if encontrado {
				break
			}
		}
		if !encontrado {
			return false
		}
		ids[d.Hecho.ID] = d.Hecho
	}
	return ctx.Err() == nil
}

// interrogativos start an open question ("¿qué significa capicúa?"), never the subject of a fact.
var interrogativos = map[string]bool{
	"que": true, "quien": true, "quienes": true, "cual": true, "cuales": true, "como": true, "donde": true,
	"cuando": true, "cuanto": true, "cuanta": true, "cuantos": true, "cuantas": true, "por": true, "porque": true,
}

// PreguntaAbierta is an open question about stored facts: "¿qué significa capicúa?" →
// {Sujeto: "capicua", Relacion: "significa"}; "¿qué es Toby?" → es_un; "¿qué tiene Toby?" → tiene.
type PreguntaAbierta struct {
	Sujeto   string // as stored (normalized)
	Escrito  string // as written ("capicúa")
	Relacion string // "significa", "es_un" or "tiene"
}

// LeerPreguntaAbierta recognizes "qué significa X", "qué quiere decir X", "cuál es el significado
// de X", "qué es X", "quién es X" and "qué tiene X" (with or without ¿?).
func LeerPreguntaAbierta(texto string) (PreguntaAbierta, bool) {
	var ws []tok
	for _, t := range tokenizar(texto) {
		if !t.sim {
			ws = append(ws, t)
		}
	}
	formas := []struct {
		seq []string
		rel string
	}{
		{[]string{"que", "significa"}, "significa"}, {[]string{"que", "quiere", "decir"}, "significa"},
		{[]string{"que", "significado", "tiene"}, "significa"}, {[]string{"cual", "es", "el", "significado", "de"}, "significa"},
		{[]string{"que", "es", "lo", "que", "significa"}, "significa"},
		{[]string{"que", "cosa", "es"}, "es_un"}, {[]string{"que", "es"}, "es_un"}, {[]string{"quien", "es"}, "es_un"},
		{[]string{"que", "son"}, "es_un"}, {[]string{"que", "tiene"}, "tiene"}, {[]string{"que", "tienen"}, "tiene"},
	}
	for _, f := range formas {
		if !empiezaPor(ws, f.seq) {
			continue
		}
		suj := sinDeterminantes(ws[len(f.seq):])
		if len(suj) == 0 || len(suj) > 4 {
			return PreguntaAbierta{}, false
		}
		return PreguntaAbierta{Sujeto: nucleo.Normalizar(textoToks(suj)), Escrito: textoOriginal(suj), Relacion: f.rel}, true
	}
	return PreguntaAbierta{}, false
}

// ResponderAbierta lists what is known (stored or deduced) about q: the matching facts, and for each
// deduced one the chain that leads to it.
func ResponderAbierta(ctx context.Context, b BaseHechos, q PreguntaAbierta) (hechos []nucleo.Hecho, cadenas [][]string) {
	if b == nil {
		return nil, nil
	}
	c := nuevoConsultor(ctx, b)
	k, rel := claveTermino(q.Sujeto), relacionNormal(q.Relacion)
	visto := map[string]bool{}
	for _, h := range c.todos {
		if h.Negado || relacionNormal(h.Relacion) != rel || claveTermino(h.Sujeto) != k || visto[claveHecho(h)] {
			continue
		}
		visto[claveHecho(h)] = true
		hechos = append(hechos, h)
		cadenas = append(cadenas, c.explicar(h))
	}
	return hechos, cadenas
}
