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

// DescribirHecho writes a fact in Spanish: "toby es un perro", "toby no es un gato",
// "«capicúa» significa «palíndromo»".
func DescribirHecho(h nucleo.Hecho) string {
	no := ""
	if h.Negado {
		no = "no "
	}
	switch relacionNormal(h.Relacion) {
	case "es_un":
		return fmt.Sprintf("%s %ses %s", h.Sujeto, no, h.Objeto)
	case "significa":
		return fmt.Sprintf("«%s» %ssignifica «%s»", h.Sujeto, no, h.Objeto)
	case "tiene":
		return fmt.Sprintf("%s %stiene %s", h.Sujeto, no, h.Objeto)
	}
	return fmt.Sprintf("%s %s%s %s", h.Sujeto, no, strings.ReplaceAll(h.Relacion, "_", " "), h.Objeto)
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
				h.ID = fmt.Sprintf("d%d", len(ders)+1)
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
	ders := Encadenar(ctx, b, maxIterEncadenar)
	hechoPorID := map[string]nucleo.Hecho{}
	var todos []nucleo.Hecho
	for i, h := range b.Todos() {
		if h.ID == "" {
			h.ID = fmt.Sprintf("h?%d", i+1)
		}
		hechoPorID[h.ID] = h
		todos = append(todos, h)
	}
	derPorID := map[string]Derivacion{}
	for _, d := range ders {
		derPorID[d.Hecho.ID] = d
		todos = append(todos, d.Hecho)
	}
	reglaPorID := map[string]nucleo.Regla{}
	for _, r := range reglasEfectivas(b.Reglas()) {
		if _, ya := reglaPorID[r.ID]; !ya {
			reglaPorID[r.ID] = r
		}
	}
	buscar := func(neg bool) (nucleo.Hecho, bool) {
		x := q
		x.Negado = neg
		k := claveHecho(x)
		for _, h := range todos {
			if claveHecho(h) == k {
				return h, true
			}
		}
		return nucleo.Hecho{}, false
	}
	explicar := func(h nucleo.Hecho) []string {
		var lineas []string
		visto := map[string]bool{}
		var rec func(id string, final bool)
		rec = func(id string, final bool) {
			if visto[id] {
				return
			}
			visto[id] = true
			if d, ok := derPorID[id]; ok {
				for _, s := range d.Desde {
					rec(s, false)
				}
				if !final {
					lineas = append(lineas, "así que "+DescribirHecho(d.Hecho))
				}
				return
			}
			if h, ok := hechoPorID[id]; ok {
				lineas = append(lineas, textoHechoBase(h))
				return
			}
			if r, ok := reglaPorID[id]; ok {
				lineas = append(lineas, DescribirRegla(r))
			}
		}
		rec(h.ID, true)
		if len(lineas) == 0 {
			lineas = append(lineas, textoHechoBase(h))
		}
		return lineas
	}
	if h, ok := buscar(q.Negado); ok {
		return "si", explicar(h)
	}
	if h, ok := buscar(!q.Negado); ok {
		return "no", explicar(h)
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
	reglas := map[string]nucleo.Regla{}
	for _, r := range reglasEfectivas(b.Reglas()) {
		reglas[r.ID] = r
	}
	for _, d := range ders {
		if len(d.Desde) == 0 {
			return false
		}
		r, ok := reglas[d.Desde[len(d.Desde)-1]]
		if !ok || len(r.Si) != len(d.Desde)-1 {
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
		for _, m := range emparejar(r.Si, fuentes) {
			if h, ok := instanciar(r.Entonces, m.vars); ok && claveHecho(h) == claveHecho(d.Hecho) {
				encontrado = true
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
