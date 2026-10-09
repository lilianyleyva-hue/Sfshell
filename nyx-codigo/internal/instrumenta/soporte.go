package instrumenta

import (
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// MaxEventosDefecto is used when OpcionesInstr.MaxEventos is 0.
const MaxEventosDefecto = 200

// MaxRunasValor caps the text of one traced value.
const MaxRunasValor = 60

// Messages of the panic types (Spanish; also used by arenero to recognise them in a crashed program).
const (
	MensajeSinComb = "se acabó el combustible: demasiados pasos (¿un bucle sin fin?)"
	MensajeHondo   = "demasiadas llamadas anidadas (¿una recursión sin fin?)"
)

// Soporte returns the support file for package `paquete`: counters (plain int64; auto-run code has no goroutines),
// coverage array [n]uint32, event buffer, the panic types, and nyx__Reiniciar(fuel int64) used by the harness per case.
//
// Exported to the rest of the package (all names start with Prefijo):
//
//	nyx__SinComb, nyx__Hondo         panic types (they implement error)
//	nyx__f, nyx__p, nyx__maxPila     fuel left, current depth, depth limit
//	nyx__c                           coverage counters, one per statement id
//	nyx__ev(linea, nombre, valor)    records one variable event
//	nyx__Reiniciar(fuel)             resets fuel, depth, coverage and events (once per case)
//	nyx__Cubiertas() []int32         ids of the statements executed since the last reset
//	nyx__Eventos() []nyx__Evento     events recorded since the last reset
//
// When op.Combustible is 0 the fuel counter starts very high, so code that was not instrumented
// for fuel never sees it run out.
func Soporte(paquete string, op nucleo.OpcionesInstr, nSentencias int) string {
	if nSentencias < 0 {
		nSentencias = 0
	}
	combustible := op.Combustible
	if combustible <= 0 {
		combustible = 1 << 62
	}
	maxPila := op.MaxPila
	if maxPila <= 0 {
		maxPila = 1 << 30
	}
	maxEventos := op.MaxEventos
	if maxEventos <= 0 {
		maxEventos = MaxEventosDefecto
	}
	r := strings.NewReplacer(
		"PAQUETE", paquete,
		"COMBUSTIBLE", strconv.FormatInt(combustible, 10),
		"MAXPILA", strconv.Itoa(maxPila),
		"NSENTENCIAS", strconv.Itoa(nSentencias),
		"MAXEVENTOS", strconv.Itoa(maxEventos),
		"MAXRUNAS", strconv.Itoa(MaxRunasValor),
		"MSGSINCOMB", strconv.Quote(MensajeSinComb),
		"MSGHONDO", strconv.Quote(MensajeHondo),
	)
	return r.Replace(plantillaSoporte)
}

const plantillaSoporte = `// Código generado por Nyx Código (soporte de instrumentación). No lo edites.

package PAQUETE

import (
	"fmt"
	"unicode/utf8"
)

type nyx__SinComb struct{}

func (nyx__SinComb) Error() string { return MSGSINCOMB }

type nyx__Hondo struct{}

func (nyx__Hondo) Error() string { return MSGHONDO }

type nyx__Evento struct {
	Linea int
	Var   string
	Valor string
}

var (
	nyx__f          int64 = COMBUSTIBLE
	nyx__p          int
	nyx__maxPila    int = MAXPILA
	nyx__c          [NSENTENCIAS]uint32
	nyx__evs        []nyx__Evento
	nyx__maxEventos int = MAXEVENTOS
)

func nyx__ev(linea int, nombre string, v any) {
	if len(nyx__evs) >= nyx__maxEventos {
		return
	}
	s := fmt.Sprint(v)
	if utf8.RuneCountInString(s) > MAXRUNAS {
		n := 0
		for i := range s {
			if n == MAXRUNAS-1 {
				s = s[:i] + "…"
				break
			}
			n++
		}
	}
	nyx__evs = append(nyx__evs, nyx__Evento{Linea: linea, Var: nombre, Valor: s})
}

func nyx__Reiniciar(fuel int64) {
	nyx__f = fuel
	nyx__p = 0
	for i := range nyx__c {
		nyx__c[i] = 0
	}
	nyx__evs = nyx__evs[:0]
}

func nyx__Cubiertas() []int32 {
	var out []int32
	for i, n := range nyx__c {
		if n > 0 {
			out = append(out, int32(i))
		}
	}
	return out
}

func nyx__Eventos() []nyx__Evento {
	return append([]nyx__Evento(nil), nyx__evs...)
}

var (
	_ = nyx__ev
	_ = nyx__Reiniciar
	_ = nyx__Cubiertas
	_ = nyx__Eventos
	_ = nyx__SinComb{}
	_ = nyx__Hondo{}
)
`
