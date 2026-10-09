package analisis

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Resumidor tries to describe a behaviour from input/output pairs. When it can, it returns a Spanish
// description and an oracle that computes the same function in-process.
type Resumidor func(ctx context.Context, f nucleo.Firma, casos []nucleo.Caso) (descripcion string, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error), ok bool)

// Igualador looks a behaviour fingerprint up in the library of known functions.
type Igualador func(huella string) (nombre string, ok bool)

// Comportamiento is what Observar learned by running the code.
type Comportamiento struct {
	Resumen    string       // "devuelve la suma de los números pares de nums"
	Verificado bool         // summary matched on all random cases
	Pruebas    int          // random cases used
	Tabla      nucleo.Tabla // sondas: entrada → salida
	IgualA     string       // "SumaPares"
	Huella     string
}

// Observar runs the probe cases, asks resumir for a summary of the observed pairs, checks the summary's
// oracle on the random cases (it is reported only on a full match) and looks the fingerprint up with igual.
// resumir and igual may be nil.
func Observar(ctx context.Context, b nucleo.Binario, f nucleo.Firma, sondas, aleatorios []nucleo.Caso, resumir Resumidor, igual Igualador, n *nucleo.Nodo) (Comportamiento, error) {
	var c Comportamiento
	if b == nil {
		return c, errors.New("analisis: no hay programa que observar")
	}
	paso := n.Sub(nucleo.PasoComprobar, "Observo qué hace con %d entradas fijas", len(sondas))
	rs, err := b.Probar(ctx, sondas, nucleo.OpcionesProbar{Variantes: []int{0}})
	if err != nil {
		paso.Mal("No pude ejecutarlo: %v", err)
		return c, err
	}
	if len(rs) == 0 || len(rs[0]) != len(sondas) {
		paso.Mal("")
		return c, errors.New("analisis: el programa no devolvió resultados")
	}
	fila := rs[0]
	c.Tabla = tablaSondas(f, sondas, fila)
	paso.Tabla(c.Tabla)
	salidas := make([][]nucleo.Valor, len(fila))
	var pares []nucleo.Caso
	for i, r := range fila {
		if r.Ejecutado && r.Panico == "" && !r.Caida && !r.Agotado && !r.SinCombustible && len(r.Obtenido) == len(f.Res) {
			salidas[i] = r.Obtenido
			pares = append(pares, nucleo.Caso{
				Entradas: sondas[i].Entradas, Esperado: r.Obtenido, Expectativa: nucleo.EspReferencia,
				Origen: sondas[i].Origen, Nota: sondas[i].Nota,
			})
		}
	}
	paso.Bien("Lo ejecuté con %d entradas fijas (%d sin fallos)", len(sondas), len(pares))
	c.Huella = nucleo.Huella(f, salidas)

	if resumir != nil && len(pares) > 0 {
		pr := n.Sub(nucleo.PasoIntento, "Busco una descripción que encaje con lo que vi")
		desc, oraculo, ok := resumir(ctx, f, pares)
		switch {
		case !ok || oraculo == nil:
			pr.Info("No encontré una descripción sencilla")
		default:
			bien, total, fallo := comprobarOraculo(ctx, b, f, sondas, fila, aleatorios, oraculo)
			c.Pruebas = total
			if fallo == "" && bien == total+len(sondas) {
				c.Resumen, c.Verificado = desc, true
				pr.Bien("En resumen: %s (comprobado con %d entradas al azar)", desc, total)
			} else {
				pr.Mal("Mi descripción («%s») no encaja: %s", desc, fallo)
			}
		}
	}
	if igual != nil {
		if nombre, ok := igual(c.Huella); ok {
			c.IgualA = nombre
			n.Nota("Hace lo mismo que %s, que ya conozco", nombre)
		}
	}
	return c, nil
}

// comprobarOraculo compares the oracle with the binary on the probes (already run) and the random cases.
// It returns how many cases agreed, how many random cases were used and the first disagreement.
func comprobarOraculo(ctx context.Context, b nucleo.Binario, f nucleo.Firma, sondas []nucleo.Caso, fila []nucleo.ResultadoCaso, aleatorios []nucleo.Caso, oraculo func([]nucleo.Valor) ([]nucleo.Valor, error)) (bien, total int, fallo string) {
	coincide := func(c nucleo.Caso, r nucleo.ResultadoCaso) (bool, string) {
		esperado, err := llamarOraculo(oraculo, c.Entradas)
		fallaPrograma := !r.Ejecutado || r.Panico != "" || r.Caida || r.Agotado || r.SinCombustible
		if err != nil || fallaPrograma {
			if err != nil && fallaPrograma {
				return true, ""
			}
			return false, "con " + entradasHumanas(f, c.Entradas) + " uno falla y el otro no"
		}
		if !filaIgual(esperado, r.Obtenido) {
			return false, "con " + entradasHumanas(f, c.Entradas) + " da " + salidasHumanas(f, r.Obtenido) + " y yo esperaba " + salidasHumanas(f, esperado)
		}
		return true, ""
	}
	for i, r := range fila {
		ok, por := coincide(sondas[i], r)
		if !ok {
			return bien, 0, por
		}
		bien++
	}
	if len(aleatorios) == 0 {
		return bien, 0, ""
	}
	rs, err := b.Probar(ctx, aleatorios, nucleo.OpcionesProbar{Variantes: []int{0}})
	if err != nil || len(rs) == 0 {
		return bien, 0, fmt.Sprintf("no pude ejecutar las entradas al azar (%v)", err)
	}
	for i, r := range rs[0] {
		if i >= len(aleatorios) {
			break
		}
		total++
		ok, por := coincide(aleatorios[i], r)
		if !ok {
			return bien, total, por
		}
		bien++
	}
	return bien, total, ""
}

func llamarOraculo(o func([]nucleo.Valor) ([]nucleo.Valor, error), in []nucleo.Valor) (out []nucleo.Valor, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("pánico: %v", r)
		}
	}()
	copia := make([]nucleo.Valor, len(in))
	for i, v := range in {
		copia[i] = nucleo.Copiar(v)
	}
	return o(copia)
}

func filaIgual(a, b []nucleo.Valor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !nucleo.Igual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func entradasHumanas(f nucleo.Firma, in []nucleo.Valor) string {
	ts := f.Entradas()
	s := ""
	for i, v := range in {
		if i > 0 {
			s += ", "
		}
		var t nucleo.Tipo
		if i < len(ts) {
			t = ts[i]
		}
		s += nucleo.FormatoHumano(v, t)
	}
	return s
}

func salidasHumanas(f nucleo.Firma, out []nucleo.Valor) string {
	s := ""
	for i, v := range out {
		if i > 0 {
			s += ", "
		}
		var t nucleo.Tipo
		if i < len(f.Res) {
			t = f.Res[i]
		}
		s += nucleo.FormatoHumano(v, t)
	}
	return s
}

// tablaSondas builds the probe table: one column per input, then the result.
func tablaSondas(f nucleo.Firma, sondas []nucleo.Caso, fila []nucleo.ResultadoCaso) nucleo.Tabla {
	tb := nucleo.Tabla{Titulo: "Lo que hace con entradas fijas"}
	if f.Receptor != nil {
		nombre := f.Receptor.Nombre
		if nombre == "" {
			nombre = "receptor"
		}
		tb.Cabecera = append(tb.Cabecera, nombre)
	}
	for i, p := range f.Params {
		nombre := p.Nombre
		if nombre == "" {
			nombre = "entrada " + strconv.Itoa(i+1)
		}
		tb.Cabecera = append(tb.Cabecera, nombre)
	}
	tb.Cabecera = append(tb.Cabecera, "resultado")
	for i, c := range sondas {
		var celdas []string
		ts := f.Entradas()
		for j, v := range c.Entradas {
			var t nucleo.Tipo
			if j < len(ts) {
				t = ts[j]
			}
			celdas = append(celdas, nucleo.FormatoHumano(v, t))
		}
		marca := ""
		if i < len(fila) {
			r := fila[i]
			switch {
			case !r.Ejecutado:
				celdas = append(celdas, "(no se ejecutó)")
				marca = "mal"
			case r.Panico != "":
				celdas = append(celdas, "pánico: "+r.Panico)
				marca = "mal"
			case r.Caida:
				celdas = append(celdas, "el programa se cayó")
				marca = "mal"
			case r.Agotado || r.SinCombustible:
				celdas = append(celdas, "tardó demasiado")
				marca = "mal"
			default:
				celdas = append(celdas, salidasHumanas(f, r.Obtenido))
			}
		} else {
			celdas = append(celdas, "?")
		}
		tb.Filas = append(tb.Filas, celdas)
		tb.Marcas = append(tb.Marcas, marca)
	}
	return tb
}

// TrazaVariables runs one case on a binary built with OpcionesInstr.Variables and returns the events.
func TrazaVariables(ctx context.Context, b nucleo.Binario, c nucleo.Caso) ([]nucleo.EventoVar, error) {
	if b == nil {
		return nil, errors.New("analisis: no hay programa que trazar")
	}
	rs, err := b.Probar(ctx, []nucleo.Caso{c}, nucleo.OpcionesProbar{Variantes: []int{0}})
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 || len(rs[0]) == 0 || !rs[0][0].Ejecutado {
		return nil, errors.New("analisis: el caso no llegó a ejecutarse")
	}
	r := rs[0][0]
	if r.Caida {
		return r.Eventos, errors.New("analisis: el programa se cayó durante la traza")
	}
	return r.Eventos, nil
}

// Medida is one timing of Crecimiento.
type Medida struct {
	N      int
	Micros int64
}

var tamanosCrecimiento = []int{100, 1000, 10000, 100000}

// Crecimiento measures the running time at n = 10², 10³, 10⁴, 10⁵ (median of 3 runs each) and classifies
// the growth. It MUST receive a binary built WITHOUT fuel/coverage instrumentation (OpcionesInstr{}).
// It stops early when a run takes more than 1 s or the watchdog (3 s) fires.
func Crecimiento(ctx context.Context, b nucleo.Binario, caso func(n int) (nucleo.Caso, bool), n *nucleo.Nodo) (clase string, m []Medida, err error) {
	if b == nil || caso == nil {
		return "", nil, errors.New("analisis: faltan el programa o el generador de casos")
	}
	paso := n.Sub(nucleo.PasoPrueba, "Mido cuánto tarda con entradas cada vez más grandes")
	for _, tam := range tamanosCrecimiento {
		if err := ctx.Err(); err != nil {
			return "", m, err
		}
		c, ok := caso(tam)
		if !ok {
			break
		}
		casos := []nucleo.Caso{c, c, c}
		rs, err := b.Probar(ctx, casos, nucleo.OpcionesProbar{Variantes: []int{0}, TiempoCaso: 3 * time.Second})
		if err != nil {
			paso.Mal("No pude medir: %v", err)
			return "", m, err
		}
		if len(rs) == 0 {
			break
		}
		var tiempos []int64
		parar := false
		for _, r := range rs[0] {
			if !r.Ejecutado || r.Agotado || r.Caida || r.Micros > 3_000_000 {
				parar = true // the watchdog (3 s) fired
				continue
			}
			tiempos = append(tiempos, r.Micros)
			if r.Micros > 1_000_000 {
				parar = true
			}
		}
		if len(tiempos) > 0 {
			sort.Slice(tiempos, func(i, j int) bool { return tiempos[i] < tiempos[j] })
			m = append(m, Medida{N: tam, Micros: tiempos[len(tiempos)/2]})
			paso.Progreso("n = %d: %d µs", tam, tiempos[len(tiempos)/2])
		}
		if parar {
			break
		}
	}
	if len(m) < 2 {
		paso.Mal("No tengo medidas suficientes")
		return "", m, errors.New("analisis: no tengo medidas suficientes para ver cómo crece")
	}
	clase = ClasificarCrecimiento(m)
	tb := nucleo.Tabla{Titulo: "Tiempos medidos", Cabecera: []string{"n", "tiempo (µs)"}}
	for _, x := range m {
		tb.Filas = append(tb.Filas, []string{strconv.Itoa(x.N), strconv.FormatInt(x.Micros, 10)})
	}
	paso.Tabla(tb)
	paso.Bien("Crece de forma %s", clase)
	return clase, m, nil
}

// ClasificarCrecimiento fits the slope of log t over log n (after subtracting the time at the smallest
// n, the process overhead) and says "constante", "lineal", "n·log n", "cuadrática", "cúbica" or "explosiva".
func ClasificarCrecimiento(m []Medida) string {
	ms := append([]Medida(nil), m...)
	sort.Slice(ms, func(i, j int) bool { return ms[i].N < ms[j].N })
	var limpio []Medida
	for _, x := range ms {
		if x.N > 0 && (len(limpio) == 0 || limpio[len(limpio)-1].N != x.N) {
			limpio = append(limpio, x)
		}
	}
	ms = limpio
	if len(ms) < 2 {
		return "constante"
	}
	base := float64(ms[0].Micros)
	if base < 0 {
		base = 0
	}
	n0 := float64(ms[0].N)
	var xs, ys []float64
	var ns []float64
	for _, x := range ms[1:] {
		d := float64(x.Micros) - base
		if d <= 0 {
			continue
		}
		xs = append(xs, math.Log(float64(x.N)))
		ys = append(ys, math.Log(d))
		ns = append(ns, float64(x.N))
	}
	// the extra time is tiny compared with the time at the smallest n: constant
	ultimo := float64(ms[len(ms)-1].Micros)
	if len(xs) == 0 || ultimo-base <= math.Max(0.25*base, 20) {
		return "constante"
	}
	var k float64
	if len(xs) >= 2 {
		k = pendiente(xs, ys)
	} else {
		// one point after the subtraction: use the raw slope between the two ends
		t0 := math.Max(float64(ms[0].Micros), 1)
		t1 := math.Max(float64(ms[len(ms)-1].Micros), 1)
		k = math.Log(t1/t0) / math.Log(float64(ms[len(ms)-1].N)/n0)
	}
	switch {
	case k < 0.25:
		return "constante"
	case k < 1.3:
		if len(xs) >= 2 {
			lin := errorModelo(ns, ys, n0, func(n float64) float64 { return n })
			nlog := errorModelo(ns, ys, n0, func(n float64) float64 { return n * math.Log(n) })
			if nlog < 0.9*lin {
				return "n·log n"
			}
		}
		return "lineal"
	case k < 2.4:
		return "cuadrática"
	case k < 3.4:
		return "cúbica"
	}
	return "explosiva"
}

// pendiente is the least-squares slope of y over x.
func pendiente(xs, ys []float64) float64 {
	var sx, sy, sxx, sxy float64
	n := float64(len(xs))
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		sxy += xs[i] * ys[i]
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return 0
	}
	return (n*sxy - sx*sy) / den
}

// errorModelo is the squared error of log d ≈ log a + log(g(n) − g(n0)), with a fitted.
func errorModelo(ns, ys []float64, n0 float64, g func(float64) float64) float64 {
	res := make([]float64, len(ns))
	media := 0.0
	for i, n := range ns {
		v := g(n) - g(n0)
		if v <= 0 {
			return math.Inf(1)
		}
		res[i] = ys[i] - math.Log(v)
		media += res[i]
	}
	media /= float64(len(ns))
	e := 0.0
	for _, r := range res {
		e += (r - media) * (r - media)
	}
	return e
}
