package memoria

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// The stored forms of facts and rules are the nucleo records themselves.
type (
	hechoGuardado = nucleo.Hecho
	reglaGuardada = nucleo.Regla
)

// claveHecho identifies a fact by its normalized triple (without the negation).
func claveHecho(h nucleo.Hecho) string {
	return nucleo.Normalizar(h.Sujeto) + "\x1f" + nucleo.Normalizar(h.Relacion) + "\x1f" + nucleo.Normalizar(h.Objeto)
}

// GuardarHecho stores a fact and returns its id ("h1", "h2"…). The same fact said again is not repeated: the
// stored one is updated (newer text, higher confidence) and its id returned. A fact that contradicts a stored
// one (same triple, opposite Negado) replaces it: the newest statement wins. A fact with a known ID replaces
// that record.
func (a *Almacen) GuardarHecho(h nucleo.Hecho) (string, error) {
	h.Sujeto, h.Relacion, h.Objeto = strings.TrimSpace(h.Sujeto), strings.TrimSpace(h.Relacion), strings.TrimSpace(h.Objeto)
	if h.Sujeto == "" || h.Relacion == "" || h.Objeto == "" {
		return "", errors.New("memoria: el dato está incompleto (falta sujeto, relación u objeto)")
	}
	if h.Fecha.IsZero() {
		h.Fecha = a.ahora()
	}
	if h.Confianza <= 0 {
		h.Confianza = 1
	}
	if h.Fuente == "" {
		h.Fuente = "usuario"
	}
	b := &a.hechos
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := a.abierto(); err != nil {
		return "", err
	}
	if h.ID != "" {
		if _, ok := b.reg[h.ID]; ok {
			if err := b.poner(a.dir, h); err != nil {
				return "", fmt.Errorf("memoria: no pude guardar el dato: %w", err)
			}
			return h.ID, nil
		}
	}
	clave := claveHecho(h)
	for _, id := range b.orden {
		g := b.reg[id]
		if claveHecho(g) != clave {
			continue
		}
		if g.Negado == h.Negado {
			if h.Texto != "" {
				g.Texto = h.Texto
			}
			if h.Confianza > g.Confianza {
				g.Confianza = h.Confianza
			}
			if err := b.poner(a.dir, g); err != nil {
				return "", fmt.Errorf("memoria: no pude guardar el dato: %w", err)
			}
			return g.ID, nil
		}
		// a contradiction: the newest statement replaces the old one
		if err := b.borrar(a.dir, g.ID); err != nil {
			return "", fmt.Errorf("memoria: no pude guardar el dato: %w", err)
		}
		break
	}
	if h.ID == "" {
		h.ID = b.nuevoID()
	}
	if err := b.poner(a.dir, h); err != nil {
		return "", fmt.Errorf("memoria: no pude guardar el dato: %w", err)
	}
	return h.ID, nil
}

// OlvidarHecho removes a fact (a tombstone in hechos.jsonl).
func (a *Almacen) OlvidarHecho(id string) error {
	b := &a.hechos
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	if _, ok := b.reg[id]; !ok {
		return fmt.Errorf("memoria: no conozco el dato %s", id)
	}
	if err := b.borrar(a.dir, id); err != nil {
		return fmt.Errorf("memoria: no pude olvidar el dato: %w", err)
	}
	return nil
}

// Hechos returns the facts about sujeto (compared after nucleo.Normalizar), oldest first.
func (a *Almacen) Hechos(sujeto string) []nucleo.Hecho {
	s := nucleo.Normalizar(strings.TrimSpace(sujeto))
	b := &a.hechos
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []nucleo.Hecho
	for _, id := range b.orden {
		if h := b.reg[id]; nucleo.Normalizar(h.Sujeto) == s {
			out = append(out, h)
		}
	}
	return out
}

// Todos returns every fact, oldest first.
func (a *Almacen) Todos() []nucleo.Hecho {
	a.hechos.mu.RLock()
	defer a.hechos.mu.RUnlock()
	return a.hechos.todos()
}

// BuscarHechos ranks facts by BM25 of the query over their text (or "sujeto relación objeto" when a fact has
// no text). It returns at most k (k ≤ 0: all) facts with a positive score, best first.
func (a *Almacen) BuscarHechos(q string, k int) []nucleo.Hecho {
	a.hechos.mu.RLock()
	todos := a.hechos.todos()
	a.hechos.mu.RUnlock()
	docs := make([][]string, len(todos))
	for i, h := range todos {
		docs[i] = tokens(h.Texto + " " + h.Sujeto + " " + strings.ReplaceAll(h.Relacion, "_", " ") + " " + h.Objeto)
	}
	puntos := bm25(tokens(q), docs)
	orden := make([]int, 0, len(todos))
	for i, p := range puntos {
		if p > 0 {
			orden = append(orden, i)
		}
	}
	sort.SliceStable(orden, func(i, j int) bool { return puntos[orden[i]] > puntos[orden[j]] })
	if k > 0 && len(orden) > k {
		orden = orden[:k]
	}
	out := make([]nucleo.Hecho, len(orden))
	for i, j := range orden {
		out[i] = todos[j]
	}
	return out
}

// claveRegla identifies a rule by its normalized conditions and conclusion.
func claveRegla(r nucleo.Regla) string {
	partes := make([]string, 0, len(r.Si))
	for _, h := range r.Si {
		partes = append(partes, claveHecho(h)+fmt.Sprint(h.Negado))
	}
	sort.Strings(partes)
	return strings.Join(partes, "\x1e") + "\x1d" + claveHecho(r.Entonces) + fmt.Sprint(r.Entonces.Negado)
}

// GuardarRegla stores a rule and returns its id ("r1", "r2"…). The same rule said again returns the stored id.
func (a *Almacen) GuardarRegla(r nucleo.Regla) (string, error) {
	if len(r.Si) == 0 || strings.TrimSpace(r.Entonces.Relacion) == "" {
		return "", errors.New("memoria: la regla está incompleta (falta la condición o la conclusión)")
	}
	if r.Fecha.IsZero() {
		r.Fecha = a.ahora()
	}
	if r.Fuente == "" {
		r.Fuente = "usuario"
	}
	r.Si = append([]nucleo.Hecho(nil), r.Si...)
	b := &a.reglas
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := a.abierto(); err != nil {
		return "", err
	}
	if r.ID != "" {
		if _, ok := b.reg[r.ID]; ok {
			if err := b.poner(a.dir, r); err != nil {
				return "", fmt.Errorf("memoria: no pude guardar la regla: %w", err)
			}
			return r.ID, nil
		}
	}
	clave := claveRegla(r)
	for _, id := range b.orden {
		if claveRegla(b.reg[id]) == clave {
			return id, nil
		}
	}
	if r.ID == "" {
		r.ID = b.nuevoID()
	}
	if err := b.poner(a.dir, r); err != nil {
		return "", fmt.Errorf("memoria: no pude guardar la regla: %w", err)
	}
	return r.ID, nil
}

// OlvidarRegla removes a rule (a tombstone in reglas.jsonl).
func (a *Almacen) OlvidarRegla(id string) error {
	b := &a.reglas
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	if _, ok := b.reg[id]; !ok {
		return fmt.Errorf("memoria: no conozco la regla %s", id)
	}
	if err := b.borrar(a.dir, id); err != nil {
		return fmt.Errorf("memoria: no pude olvidar la regla: %w", err)
	}
	return nil
}

// Reglas returns every rule, oldest first.
func (a *Almacen) Reglas() []nucleo.Regla {
	a.reglas.mu.RLock()
	defer a.reglas.mu.RUnlock()
	out := a.reglas.todos()
	for i := range out {
		out[i].Si = append([]nucleo.Hecho(nil), out[i].Si...)
	}
	return out
}
