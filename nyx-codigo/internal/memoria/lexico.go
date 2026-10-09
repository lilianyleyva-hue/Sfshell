package memoria

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// ConfirmarTras is the number of independent successes that confirm a proposed word.
const ConfirmarTras = 2

// Efectos are the meanings a word-problem verb can have (mates.Verbos).
var Efectos = map[string]bool{"sumar": true, "restar": true, "fijar": true, "transferir": true}

type datosLexico struct {
	Version     int                       `json:"version"`
	Confirmadas map[string]string         `json:"confirmadas"`
	Propuestas  map[string]map[string]int `json:"propuestas"`
	Verbos      map[string]string         `json:"verbos"`
}

type lexico struct {
	mu    sync.RWMutex
	datos datosLexico
}

func vacioLexico() datosLexico {
	return datosLexico{
		Version:     Version,
		Confirmadas: map[string]string{},
		Propuestas:  map[string]map[string]int{},
		Verbos:      map[string]string{},
	}
}

func (a *Almacen) cargarLexico() {
	var d datosLexico
	if !a.leerJSON(archivoLexico, &d, &d.Version) {
		a.lex.datos = vacioLexico()
		return
	}
	v := vacioLexico()
	for p, c := range d.Confirmadas {
		if p, c = normalizarPalabra(p), normalizarConcepto(c); p != "" && c != "" {
			v.Confirmadas[p] = c
		}
	}
	for p, m := range d.Propuestas {
		p = normalizarPalabra(p)
		if p == "" {
			continue
		}
		for c, n := range m {
			if c = normalizarConcepto(c); c != "" && n > 0 {
				if v.Propuestas[p] == nil {
					v.Propuestas[p] = map[string]int{}
				}
				v.Propuestas[p][c] += n
			}
		}
	}
	for verbo, e := range d.Verbos {
		if verbo = normalizarPalabra(verbo); verbo != "" && Efectos[e] {
			v.Verbos[verbo] = e
		}
	}
	a.lex.datos = v
}

// guardarLexico rewrites lexico.json. The caller holds the write lock.
func (a *Almacen) guardarLexico() error {
	if err := a.abierto(); err != nil {
		return err
	}
	return escribirJSON(filepath.Join(a.dir, archivoLexico), a.lex.datos)
}

// normalizarPalabra: nucleo.Normalizar and trimmed ("Capicúa" → "capicua").
func normalizarPalabra(p string) string { return strings.TrimSpace(nucleo.Normalizar(p)) }

// normalizarConcepto: normalized, spaces become "_" ("palíndromo" → "palindromo", "mayor que" → "mayor_que").
func normalizarConcepto(c string) string {
	return strings.ReplaceAll(strings.TrimSpace(nucleo.Normalizar(c)), " ", "_")
}

// Palabras returns the confirmed words: word → concept.
func (a *Almacen) Palabras() map[string]string {
	a.lex.mu.RLock()
	defer a.lex.mu.RUnlock()
	out := make(map[string]string, len(a.lex.datos.Confirmadas))
	for p, c := range a.lex.datos.Confirmadas {
		out[p] = c
	}
	return out
}

// ProponerPalabra records one success of reading palabra as concepto. The word is confirmed after
// ConfirmarTras (2) such successes; it returns whether it is confirmed now. A word already confirmed for a
// different concept (for example, taught explicitly) is not changed.
func (a *Almacen) ProponerPalabra(palabra, concepto string) (confirmada bool) {
	p, c := normalizarPalabra(palabra), normalizarConcepto(concepto)
	if p == "" || c == "" {
		return false
	}
	a.lex.mu.Lock()
	defer a.lex.mu.Unlock()
	d := &a.lex.datos
	if actual, ok := d.Confirmadas[p]; ok {
		return actual == c
	}
	if d.Propuestas[p] == nil {
		d.Propuestas[p] = map[string]int{}
	}
	d.Propuestas[p][c]++
	if d.Propuestas[p][c] >= ConfirmarTras {
		d.Confirmadas[p] = c
		delete(d.Propuestas, p)
		confirmada = true
	}
	if err := a.guardarLexico(); err != nil && !errors.Is(err, ErrCerrado) {
		a.avisar(fmt.Sprintf("No pude guardar el léxico: %v.", err))
	}
	return confirmada
}

// EnsenarPalabra confirms a word immediately ("recuerda que capicúa significa palíndromo").
func (a *Almacen) EnsenarPalabra(palabra, concepto string) error {
	p, c := normalizarPalabra(palabra), normalizarConcepto(concepto)
	if p == "" || c == "" {
		return errors.New("memoria: falta la palabra o su significado")
	}
	a.lex.mu.Lock()
	defer a.lex.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	d := &a.lex.datos
	antes, habia := d.Confirmadas[p]
	propuestas := d.Propuestas[p]
	d.Confirmadas[p] = c
	delete(d.Propuestas, p)
	if err := a.guardarLexico(); err != nil {
		if habia {
			d.Confirmadas[p] = antes
		} else {
			delete(d.Confirmadas, p)
		}
		if propuestas != nil {
			d.Propuestas[p] = propuestas
		}
		return fmt.Errorf("memoria: no pude guardar el léxico: %w", err)
	}
	return nil
}

// OlvidarPalabra removes a confirmed word and its proposals.
func (a *Almacen) OlvidarPalabra(palabra string) error {
	p := normalizarPalabra(palabra)
	a.lex.mu.Lock()
	defer a.lex.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	d := &a.lex.datos
	antes, habia := d.Confirmadas[p]
	propuestas := d.Propuestas[p]
	if !habia && propuestas == nil {
		return fmt.Errorf("memoria: no conozco la palabra %q", palabra)
	}
	delete(d.Confirmadas, p)
	delete(d.Propuestas, p)
	if err := a.guardarLexico(); err != nil {
		if habia {
			d.Confirmadas[p] = antes
		}
		if propuestas != nil {
			d.Propuestas[p] = propuestas
		}
		return fmt.Errorf("memoria: no pude guardar el léxico: %w", err)
	}
	return nil
}

// Efecto returns what a word-problem verb does: "sumar", "restar", "fijar" or "transferir" (mates.Verbos).
func (a *Almacen) Efecto(verbo string) (string, bool) {
	a.lex.mu.RLock()
	defer a.lex.mu.RUnlock()
	e, ok := a.lex.datos.Verbos[normalizarPalabra(verbo)]
	return e, ok
}

// Aprender remembers the answer to "¿«trocar» suma o resta?". efecto must be one of Efectos.
func (a *Almacen) Aprender(verbo, efecto string) error {
	v := normalizarPalabra(verbo)
	e := normalizarPalabra(efecto)
	if v == "" {
		return errors.New("memoria: falta el verbo")
	}
	if !Efectos[e] {
		return fmt.Errorf("memoria: no sé qué efecto es %q (puede ser sumar, restar, fijar o transferir)", efecto)
	}
	a.lex.mu.Lock()
	defer a.lex.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	d := &a.lex.datos
	antes, habia := d.Verbos[v]
	d.Verbos[v] = e
	if err := a.guardarLexico(); err != nil {
		if habia {
			d.Verbos[v] = antes
		} else {
			delete(d.Verbos, v)
		}
		return fmt.Errorf("memoria: no pude guardar el léxico: %w", err)
	}
	return nil
}

// Verbos returns every learned verb: verb → effect.
func (a *Almacen) Verbos() map[string]string {
	a.lex.mu.RLock()
	defer a.lex.mu.RUnlock()
	out := make(map[string]string, len(a.lex.datos.Verbos))
	for v, e := range a.lex.datos.Verbos {
		out[v] = e
	}
	return out
}
