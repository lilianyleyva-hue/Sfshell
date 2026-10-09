package reparar

import (
	"context"
	"math/rand"
	"sort"

	"nyxcodigo/internal/nucleo"
)

// This file holds the small built-in case generator and shrinker used when the Reparador has no
// Azar / Reducir ports (cerebro plugs in the ones from pruebas).

// semillaSobreajuste is the fixed seed of the overfitting check, so a repair is reproducible.
const semillaSobreajuste = 20240611

// casosAzar returns n random cases for f (no expected values), reproducible from semilla.
func casosAzar(f nucleo.Firma, n int, semilla int64) []nucleo.Caso {
	rng := rand.New(rand.NewSource(semilla))
	ent := f.Entradas()
	for _, t := range ent {
		if t.Clase == nucleo.CError || !t.Codificable() {
			return nil
		}
	}
	out := make([]nucleo.Caso, 0, n)
	for i := 0; i < n; i++ {
		tam := 1 + i*12/max(n, 1) // sizes grow from 1 to 12 over the run
		e := make([]nucleo.Valor, len(ent))
		for j, t := range ent {
			e[j] = valorAzar(rng, t, tam, 0)
		}
		out = append(out, nucleo.Caso{Entradas: e, Origen: nucleo.OrigenAzar, Nota: "al azar"})
	}
	return out
}

func elem(p *nucleo.Tipo) nucleo.Tipo {
	if p == nil {
		return nucleo.Tipo{}
	}
	return *p
}

// rangoTipo returns the bounds of an integer type, narrowed to what random cases use.
func rangoTipo(t nucleo.Tipo, tam int) (int, int) {
	lim := 5 + 4*tam
	lo, hi := -lim, lim
	switch t.Nombre {
	case "int8":
		lo, hi = max(lo, -128), min(hi, 127)
	case "uint", "uint16", "uint32", "uint64", "uintptr":
		lo = 0
	case "uint8", "byte":
		lo, hi = 0, min(hi, 255)
	}
	if t.Clase == nucleo.CByte {
		lo, hi = 0, 255
	}
	return lo, hi
}

const alfabetoAzar = "aabcdeeiounrslt ñAZ1"

func valorAzar(rng *rand.Rand, t nucleo.Tipo, tam, prof int) nucleo.Valor {
	if prof > 4 {
		return nucleo.ValorCero(t)
	}
	largo := func() int {
		n := tam
		if prof > 0 {
			n = min(n, 3)
		}
		return rng.Intn(n + 1)
	}
	switch t.Clase {
	case nucleo.CInt:
		lo, hi := rangoTipo(t, tam)
		return lo + rng.Intn(hi-lo+1)
	case nucleo.CByte:
		lo, hi := rangoTipo(t, tam)
		if rng.Intn(2) == 0 {
			return int(alfabetoAzar[rng.Intn(len(alfabetoAzar))])
		}
		return lo + rng.Intn(hi-lo+1)
	case nucleo.CRune:
		rs := []rune(alfabetoAzar)
		return int(rs[rng.Intn(len(rs))])
	case nucleo.CFloat:
		return float64(rng.Intn(41)-20) / 4
	case nucleo.CBool:
		return rng.Intn(2) == 1
	case nucleo.CString:
		rs := []rune(alfabetoAzar)
		n := largo()
		b := make([]rune, n)
		for i := range b {
			b[i] = rs[rng.Intn(len(rs))]
		}
		return string(b)
	case nucleo.CLista:
		n := largo()
		if n == 0 && rng.Intn(3) == 0 {
			return []nucleo.Valor(nil)
		}
		xs := make([]nucleo.Valor, n)
		for i := range xs {
			xs[i] = valorAzar(rng, elem(t.Elem), tam, prof+1)
		}
		return xs
	case nucleo.CArreglo:
		xs := make([]nucleo.Valor, t.Largo)
		for i := range xs {
			xs[i] = valorAzar(rng, elem(t.Elem), tam, prof+1)
		}
		return xs
	case nucleo.CMapa:
		n := largo()
		m := nucleo.Mapa{}
		visto := map[string]bool{}
		for i := 0; i < n; i++ {
			k := valorAzar(rng, elem(t.Clave), tam, prof+1)
			c := nucleo.Clave(k)
			if visto[c] {
				continue
			}
			visto[c] = true
			m = append(m, nucleo.Par{K: k, V: valorAzar(rng, elem(t.Elem), tam, prof+1)})
		}
		return m.Ordenada()
	case nucleo.CStruct:
		s := make(nucleo.Estructura, len(t.Campos))
		for i, c := range t.Campos {
			s[i] = valorAzar(rng, c.Tipo, tam, prof+1)
		}
		return s
	case nucleo.CPuntero:
		if rng.Intn(4) == 0 {
			return nil
		}
		return valorAzar(rng, elem(t.Elem), tam, prof+1)
	}
	return nucleo.ValorCero(t)
}

// ---- shrinking ----

const maxReduccionesLocal = 40

func tamanoEntradas(e []nucleo.Valor) int {
	n := 0
	for _, v := range e {
		n += nucleo.Tamano(v)
	}
	return n
}

// reduccionesLocal returns up to 40 candidates strictly smaller than c, without expected values.
func reduccionesLocal(c nucleo.Caso) []nucleo.Caso {
	tam := tamanoEntradas(c.Entradas)
	visto := map[string]bool{}
	var out []nucleo.Caso
	for i, v := range c.Entradas {
		for _, w := range menoresLocal(v) {
			e := make([]nucleo.Valor, len(c.Entradas))
			for j := range c.Entradas {
				e[j] = c.Entradas[j]
				if j == i {
					e[j] = w
				}
				e[j] = nucleo.Copiar(e[j])
			}
			if tamanoEntradas(e) >= tam {
				continue
			}
			k := ""
			for _, x := range e {
				k += nucleo.Clave(x) + "\x1f"
			}
			if visto[k] {
				continue
			}
			visto[k] = true
			out = append(out, nucleo.Caso{Entradas: e, Origen: c.Origen, Nota: c.Nota})
		}
	}
	if len(out) > maxReduccionesLocal {
		sort.SliceStable(out, func(a, b int) bool { return tamanoEntradas(out[a].Entradas) < tamanoEntradas(out[b].Entradas) })
		out = out[:maxReduccionesLocal]
	}
	return out
}

func menoresLocal(v nucleo.Valor) []nucleo.Valor {
	var out []nucleo.Valor
	switch x := v.(type) {
	case int:
		if x != 0 {
			out = append(out, 0, x/2)
			if x > 0 {
				out = append(out, x-1)
			} else {
				out = append(out, x+1)
			}
		}
	case float64:
		if x != 0 {
			out = append(out, 0.0, float64(int(x)))
		}
	case string:
		rs := []rune(x)
		if len(rs) > 0 {
			out = append(out, string(rs[:len(rs)/2]), string(rs[len(rs)/2:]))
			for i := 0; i < len(rs) && i < 10; i++ {
				out = append(out, string(rs[:i])+string(rs[i+1:]))
			}
		}
	case []nucleo.Valor:
		n := len(x)
		if n > 0 {
			out = append(out, append([]nucleo.Valor{}, x[:n/2]...), append([]nucleo.Valor{}, x[n/2:]...))
			for i := 0; i < n && i < 10; i++ {
				y := append(append([]nucleo.Valor{}, x[:i]...), x[i+1:]...)
				out = append(out, y)
			}
			for i := 0; i < n && i < 10; i++ {
				for k, w := range menoresLocal(x[i]) {
					if k == 3 {
						break
					}
					y := append([]nucleo.Valor{}, x...)
					y[i] = w
					out = append(out, y)
				}
			}
		}
	case nucleo.Mapa:
		for i := range x {
			y := append(append(nucleo.Mapa{}, x[:i]...), x[i+1:]...)
			out = append(out, y)
		}
	case nucleo.Estructura:
		for i := range x {
			for _, w := range menoresLocal(x[i]) {
				y := append(nucleo.Estructura{}, x...)
				y[i] = w
				out = append(out, y)
				break
			}
		}
	}
	return out
}

// minimizarLocal is the built-in shrinker (same contract as pruebas.Minimizar).
func minimizarLocal(ctx context.Context, c nucleo.Caso, f nucleo.Firma, falla func([]nucleo.Caso) []bool, rondas int) nucleo.Caso {
	if rondas <= 0 {
		rondas = 12
	}
	actual, cambiado := c, false
	for ronda := 0; ronda < rondas && ctx.Err() == nil; ronda++ {
		cands := reduccionesLocal(actual)
		if len(cands) == 0 {
			break
		}
		fallan := falla(cands)
		mejor := -1
		for i := range cands {
			if i < len(fallan) && fallan[i] && (mejor < 0 || tamanoEntradas(cands[i].Entradas) < tamanoEntradas(cands[mejor].Entradas)) {
				mejor = i
			}
		}
		if mejor < 0 {
			break
		}
		actual, cambiado = cands[mejor], true
	}
	if !cambiado {
		return c
	}
	actual.Esperado, actual.Expectativa = nil, nucleo.EspNinguna
	return actual
}
