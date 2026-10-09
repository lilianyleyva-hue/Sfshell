package sintesis

import (
	"math"

	"nyxcodigo/internal/nucleo"
)

// Distinguir picks the probe on which the solutions split into the most groups (largest partition entropy).
// It returns that input, one representative output per group and the groups (indexes into sols). ok is
// false when every probe gives the same output for all solutions. A solution without result on a probe
// forms its own group, whose representative output is nucleo.ErrorV("sin resultado"); probes where some
// solution has no result are used only when no other probe splits the solutions.
func Distinguir(sols []Solucion, sondas [][]nucleo.Valor) (entrada []nucleo.Valor, salidas []nucleo.Valor, grupos [][]int, ok bool) {
	if len(sols) < 2 || len(sondas) == 0 {
		return nil, nil, nil, false
	}
	oraculos := make([]func([]nucleo.Valor) ([]nucleo.Valor, error), len(sols))
	for i, s := range sols {
		oraculos[i] = Oraculo(s.Expr)
	}
	mejorH := -1.0
	mejorFallos := true
	for _, sd := range sondas {
		var claves []string
		var reps []nucleo.Valor
		var gs [][]int
		fallos := false
		for i, o := range oraculos {
			out, err := o(sd)
			var v nucleo.Valor
			k := "⊥"
			if err != nil || len(out) != 1 {
				fallos = true
				v = nucleo.ErrorV("sin resultado")
			} else {
				v = out[0]
				k = "=" + nucleo.Clave(v)
			}
			g := -1
			for j, c := range claves {
				if c == k {
					g = j
					break
				}
			}
			if g < 0 {
				claves = append(claves, k)
				reps = append(reps, v)
				gs = append(gs, nil)
				g = len(gs) - 1
			}
			gs[g] = append(gs[g], i)
		}
		if len(gs) < 2 {
			continue
		}
		h := 0.0
		for _, g := range gs {
			p := float64(len(g)) / float64(len(sols))
			h -= p * math.Log2(p)
		}
		mejora := false
		switch {
		case entrada == nil:
			mejora = true
		case mejorFallos && !fallos:
			mejora = true
		case fallos && !mejorFallos:
			mejora = false
		default:
			mejora = h > mejorH+1e-12
		}
		if mejora {
			entrada, salidas, grupos, mejorH, mejorFallos = sd, reps, gs, h, fallos
		}
	}
	if entrada == nil {
		return nil, nil, nil, false
	}
	return entrada, salidas, grupos, true
}
