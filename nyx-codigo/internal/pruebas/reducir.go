package pruebas

import (
	"context"
	"fmt"
	"sort"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// ConOraculo returns a copy of cs where every case without an expectation gets the oracle's output as
// Esperado, with Expectativa e and Nota nota. Cases that already carry an expectation are kept as they
// are, and cases where the oracle errs (or panics) keep EspNinguna.
func ConOraculo(cs []nucleo.Caso, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error), e nucleo.Expectativa, nota string) []nucleo.Caso {
	out := make([]nucleo.Caso, len(cs))
	for i, c := range cs {
		out[i] = c
		if oraculo == nil || c.ConEsperado() || e == nucleo.EspNinguna {
			continue
		}
		copia := make([]nucleo.Valor, len(c.Entradas))
		for j, v := range c.Entradas {
			copia[j] = nucleo.Copiar(v)
		}
		r, err := llamarOraculo(oraculo, copia)
		if err != nil || r == nil {
			continue
		}
		out[i].Esperado = r
		out[i].Expectativa = e
		if nota != "" {
			out[i].Nota = nota
		}
	}
	return out
}

func llamarOraculo(o func([]nucleo.Valor) ([]nucleo.Valor, error), in []nucleo.Valor) (r []nucleo.Valor, err error) {
	defer func() {
		if p := recover(); p != nil {
			r, err = nil, fmt.Errorf("el oráculo falló: %v", p)
		}
	}()
	return o(in)
}

// maxReducciones is the cap on shrinking candidates per round (§4.3).
const maxReducciones = 40

// Reducciones returns at most 40 candidates strictly smaller than c (by the sum of nucleo.Tamano of the
// inputs): half of a list dropped, each element dropped (at most 10), each int moved toward 0 (to 0,
// halved, then one step), strings cut in half or with one rune dropped, runes turned into 'a', struct
// fields set to zero. Candidates keep c's Origen and Nota but have no expected value.
func Reducciones(c nucleo.Caso, f nucleo.Firma) []nucleo.Caso {
	ent := f.Entradas()
	tam := tamanoCaso(c.Entradas)
	visto := map[string]bool{claveEntradas(c.Entradas): true}
	var out []nucleo.Caso
	for i, v := range c.Entradas {
		var t nucleo.Tipo
		if i < len(ent) {
			t = ent[i]
		}
		for _, w := range menores(v, t, 0) {
			e := make([]nucleo.Valor, len(c.Entradas))
			for j := range c.Entradas {
				if j == i {
					e[j] = w
				} else {
					e[j] = nucleo.Copiar(c.Entradas[j])
				}
			}
			if tamanoCaso(e) >= tam {
				continue
			}
			k := claveEntradas(e)
			if visto[k] {
				continue
			}
			visto[k] = true
			out = append(out, nucleo.Caso{Entradas: e, Origen: c.Origen, Nota: c.Nota})
		}
	}
	// the most aggressive candidates come first; keep the 40 smallest when there are more
	if len(out) > maxReducciones {
		sort.SliceStable(out, func(a, b int) bool { return tamanoCaso(out[a].Entradas) < tamanoCaso(out[b].Entradas) })
		out = out[:maxReducciones]
	}
	return out
}

func tamanoCaso(e []nucleo.Valor) int {
	t := 0
	for _, v := range e {
		t += nucleo.Tamano(v)
	}
	return t
}

// menores lists smaller versions of one value of type t (t may be invalid: then the dynamic form decides).
func menores(v nucleo.Valor, t nucleo.Tipo, prof int) []nucleo.Valor {
	if prof > 4 {
		return nil
	}
	if t.Clase == nucleo.CPuntero {
		if v == nil || t.Elem == nil {
			return nil
		}
		return append([]nucleo.Valor{nil}, menores(v, *t.Elem, prof+1)...)
	}
	var out []nucleo.Valor
	switch x := v.(type) {
	case nil:
		return nil
	case bool:
		if x {
			out = append(out, false)
		}
	case int:
		if t.Clase == nucleo.CRune || t.Clase == nucleo.CByte {
			if x != 'a' && x > 'a' {
				out = append(out, int('a'))
			}
			if x != 0 {
				out = append(out, x/2)
			}
			break
		}
		if x != 0 {
			out = append(out, 0)
			if x/2 != 0 {
				out = append(out, x/2)
			}
			if x > 0 {
				out = append(out, x-1)
			} else {
				out = append(out, x+1)
			}
		}
	case float64:
		if x != 0 {
			out = append(out, 0.0)
			if h := float64(int64(x)); h != x {
				out = append(out, h) // drop the decimals
			}
			out = append(out, x/2)
		}
	case string:
		if x == "" {
			break
		}
		runas := []rune(x)
		out = append(out, "")
		if len(runas) > 1 {
			out = append(out, string(runas[:len(runas)/2]), string(runas[len(runas)/2:]))
		}
		for i := 0; i < len(runas) && i < 10; i++ {
			out = append(out, string(runas[:i])+string(runas[i+1:]))
		}
		// non-ASCII runes turned into 'a' (fewer bytes)
		cambiada := false
		for i, r := range runas {
			if utf8.RuneLen(r) > 1 {
				runas[i] = 'a'
				cambiada = true
			}
		}
		if cambiada {
			out = append(out, string(runas))
		}
	case []nucleo.Valor:
		if x == nil {
			break
		}
		var e nucleo.Tipo
		if t.Elem != nil {
			e = *t.Elem
		}
		fijo := t.Clase == nucleo.CArreglo
		if !fijo {
			if len(x) == 0 {
				out = append(out, []nucleo.Valor(nil))
				break
			}
			if len(x) > 1 {
				out = append(out, copiaLista(x[:len(x)/2]), copiaLista(x[len(x)/2:]))
			}
			for i := 0; i < len(x) && i < 10; i++ {
				out = append(out, append(copiaLista(x[:i]), copiaLista(x[i+1:])...))
			}
		}
		for i := 0; i < len(x) && i < 10; i++ {
			for _, w := range menores(x[i], e, prof+1) {
				c := copiaLista(x)
				c[i] = w
				out = append(out, c)
			}
		}
	case nucleo.Mapa:
		if x == nil {
			break
		}
		var vt nucleo.Tipo
		if t.Elem != nil {
			vt = *t.Elem
		}
		if len(x) == 0 {
			out = append(out, nucleo.Mapa(nil))
			break
		}
		if len(x) > 1 {
			out = append(out, copiaMapa(x[:len(x)/2]), copiaMapa(x[len(x)/2:]))
		}
		for i := 0; i < len(x) && i < 10; i++ {
			out = append(out, append(copiaMapa(x[:i]), copiaMapa(x[i+1:])...))
		}
		for i := 0; i < len(x) && i < 5; i++ {
			for _, w := range menores(x[i].V, vt, prof+1) {
				c := copiaMapa(x)
				c[i].V = w
				out = append(out, c)
			}
		}
	case nucleo.Estructura:
		for i := range x {
			var ft nucleo.Tipo
			if i < len(t.Campos) {
				ft = t.Campos[i].Tipo
			} else {
				continue
			}
			cero := nucleo.ValorCero(ft)
			if !nucleo.Igual(x[i], cero) {
				c := nucleo.Copiar(x).(nucleo.Estructura)
				c[i] = cero
				out = append(out, c)
			}
			for _, w := range menores(x[i], ft, prof+1) {
				c := nucleo.Copiar(x).(nucleo.Estructura)
				c[i] = w
				out = append(out, c)
			}
		}
	}
	return out
}

func copiaLista(x []nucleo.Valor) []nucleo.Valor {
	c := make([]nucleo.Valor, len(x))
	for i, v := range x {
		c[i] = nucleo.Copiar(v)
	}
	return c
}

func copiaMapa(x nucleo.Mapa) nucleo.Mapa {
	c := make(nucleo.Mapa, len(x))
	for i, p := range x {
		c[i] = nucleo.Par{K: nucleo.Copiar(p.K), V: nucleo.Copiar(p.V)}
	}
	return c
}

// Minimizar shrinks a failing case. Each round computes Reducciones, runs all of them in ONE call to
// falla (one Probar, no rebuild), and keeps the smallest failing candidate by Tamano. It stops after
// rondas rounds (12 when rondas ≤ 0), when nothing smaller fails, or when ctx is done. A shrunk case has
// no expected value (the original one no longer applies); an unchanged case is returned as given.
func Minimizar(ctx context.Context, c nucleo.Caso, f nucleo.Firma, falla func([]nucleo.Caso) []bool, rondas int) nucleo.Caso {
	if rondas <= 0 {
		rondas = 12
	}
	actual := c
	cambiado := false
	for ronda := 0; ronda < rondas; ronda++ {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		cands := Reducciones(actual, f)
		if len(cands) == 0 {
			break
		}
		fallan := falla(cands)
		mejor := -1
		for i := range cands {
			if i >= len(fallan) || !fallan[i] {
				continue
			}
			if mejor < 0 || tamanoCaso(cands[i].Entradas) < tamanoCaso(cands[mejor].Entradas) {
				mejor = i
			}
		}
		if mejor < 0 {
			break
		}
		actual = cands[mejor]
		cambiado = true
	}
	if !cambiado {
		return c
	}
	actual.Esperado = nil
	actual.Expectativa = nucleo.EspNinguna
	return actual
}
