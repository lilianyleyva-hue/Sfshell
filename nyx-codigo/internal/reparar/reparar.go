// Package reparar repairs Go code: it rewrites habits from other languages (Idiomas), runs the
// compile-error repair loop (syntax edits, then type-checked rules), repairs panics, and does fault
// localization (Ochiai) plus mutation repair with an overfitting check. It also translates compiler
// errors and panics into simple Spanish and produces unified diffs.
//
// It imports only the standard library and internal/nucleo (§2, §4.4).
package reparar

import (
	"context"
	"errors"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Limites bounds the work of a Reparador. Zero fields take the defaults.
type Limites struct {
	Rondas       int           // compile loop, default 10
	TCompilacion time.Duration // default 15 s
	Mutantes     int           // single mutants, default 400
	Pares        int           // second-order pairs, default 1500
	TLogica      time.Duration // default 25 s
}

func (l Limites) conDefectos() Limites {
	if l.Rondas <= 0 {
		l.Rondas = 10
	}
	if l.TCompilacion <= 0 {
		l.TCompilacion = 15 * time.Second
	}
	if l.Mutantes <= 0 {
		l.Mutantes = 400
	}
	if l.Pares <= 0 {
		l.Pares = 1500
	}
	if l.TLogica <= 0 {
		l.TLogica = 25 * time.Second
	}
	return l
}

// Reparador holds the sandbox, the learning statistics and the limits.
//
// Reducir and Azar are optional ports (cerebro may plug pruebas.Minimizar and a pruebas generator in);
// when nil, a small built-in shrinker and generator are used.
type Reparador struct {
	Ej     nucleo.Ejecutor
	Stats  nucleo.Contador // keys "regla:<nombre>", "mutacion:<operador>"
	Limite Limites

	// Reducir minimizes a failing case (same signature as pruebas.Minimizar).
	Reducir func(ctx context.Context, c nucleo.Caso, f nucleo.Firma, falla func([]nucleo.Caso) []bool, rondas int) nucleo.Caso
	// Azar returns n random cases for f, without expected values (used by the overfitting check).
	Azar func(f nucleo.Firma, n int) []nucleo.Caso
}

// Cambio is one edit, explained in Spanish.
type Cambio struct {
	Regla   string `json:"regla"` // "importar_paquete", "levenshtein", "menor_a_menor_igual"…
	Linea   int    `json:"linea"`
	Antes   string `json:"antes"`
	Despues string `json:"despues"`
	Porque  string `json:"porque"` // Spanish: "faltaba importar «strings»"
}

// Arreglo is the result of the compile-error repair loop.
type Arreglo struct {
	Fuente       string
	Cambios      []Cambio
	OK           bool // compiles (go build) and vet has no errors
	Rondas       int
	Quedan       []nucleo.ErrorGo // remaining errors
	Suposiciones []string         // "puse «return 0» al final: es una suposición"
	Vet          []nucleo.ErrorGo
}

// Parche is a candidate repair that was run against the cases.
type Parche struct {
	Fuente   string
	Cambios  []Cambio
	Pasan    int
	Total    int
	Sospecha float64 // suspiciousness of the edited statement
}

// Ambiguedad: two plausible patches disagree on an input.
type Ambiguedad struct {
	Entrada []nucleo.Valor
	Salidas [][]nucleo.Valor // per candidate patch
	Parches []int            // indices into ResultadoLogica.Parches
}

// ResultadoLogica is the result of logic repair.
type ResultadoLogica struct {
	Parches       []Parche     // passing all cases, ranked
	Contraejemplo *nucleo.Caso // minimized failing case of the ORIGINAL code
	Ambiguo       *Ambiguedad  // two plausible patches disagree on a random input
	Mejor         *Parche      // best partial (most tests) when no full patch
	Motivo        string       // Spanish reason when nothing found
}

// Mutante is one single-edit variant of a source file.
type Mutante struct {
	Fuente   string
	Cambio   Cambio
	Operador string
}

// ErrSintaxis is returned by ChequeoTipos when the source does not parse; the syntax errors are
// returned too, as ErrorGo values.
var ErrSintaxis = errors.New("reparar: el código tiene errores de sintaxis")

func (r *Reparador) tasa(clave string) float64 {
	if r == nil || r.Stats == nil {
		return 0.5
	}
	return r.Stats.Tasa(clave)
}

func (r *Reparador) exito(clave string, ok bool) {
	if r != nil && r.Stats != nil {
		r.Stats.Exito(clave, ok)
	}
}
