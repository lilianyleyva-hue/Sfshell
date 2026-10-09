package pruebas

import (
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// resultadoDe returns the result of case i: rs is either aligned with casos, or indexed by ResultadoCaso.Caso.
func resultados(casos []nucleo.Caso, rs []nucleo.ResultadoCaso) []*nucleo.ResultadoCaso {
	out := make([]*nucleo.ResultadoCaso, len(casos))
	if len(rs) == len(casos) {
		alineado := true
		for i := range rs {
			if rs[i].Caso != i && rs[i].Caso != 0 {
				alineado = false
				break
			}
		}
		if alineado {
			for i := range rs {
				out[i] = &rs[i]
			}
			return out
		}
	}
	for i := range rs {
		if c := rs[i].Caso; c >= 0 && c < len(out) && out[c] == nil {
			out[c] = &rs[i]
		}
	}
	return out
}

// terminado: the case ran to the end (no panic, crash, timeout, fuel or nondeterminism).
func terminado(r *nucleo.ResultadoCaso) bool {
	return r != nil && r.Ejecutado && r.Panico == "" && !r.Caida && !r.Agotado && !r.SinCombustible && !r.NoDeterminista
}

// cumpleEsperado: the case ran and its outputs match the expected values.
func cumpleEsperado(c nucleo.Caso, r *nucleo.ResultadoCaso) bool {
	if !terminado(r) {
		return false
	}
	if r.OK {
		return true
	}
	if !c.ConEsperado() || len(r.Obtenido) != len(c.Esperado) {
		return false
	}
	for i := range c.Esperado {
		if !nucleo.Igual(r.Obtenido[i], c.Esperado[i]) {
			return false
		}
	}
	return true
}

// Comprobaciones groups the results by Expectativa and counts the checked properties (§4.3):
//
//	EspUsuario        → tus_ejemplos "Cumple tus N ejemplos" / "Falla N de tus M ejemplos"
//	EspInterpretacion → entendido    "N/M pruebas según lo que entendí"
//	EspReferencia     → entendido    "N/M pruebas comparando con <nota>" (one badge per nota)
//	properties        → propiedades  "Propiedades: N/M"
//	everything else   → sin_fallos   "N casos sin errores"
//
// A group with failures gets Nivel fallo. A case that was never run counts as not passing.
// Detalle says the seed and how many cases came from each origin.
func Comprobaciones(casos []nucleo.Caso, rs []nucleo.ResultadoCaso, props []nucleo.Propiedad, semilla int64) []nucleo.Comprobacion {
	res := resultados(casos, rs)
	detalle := detalleOrigenes(casos, semilla)

	var usuario, interp, ninguna [2]int // pasan, total
	type grupoRef struct {
		nota         string
		pasan, total int
	}
	var refs []*grupoRef
	porNota := map[string]*grupoRef{}
	propPasan, propTotal := 0, 0

	for i, c := range casos {
		r := res[i]
		switch {
		case c.ConEsperado() && c.Expectativa == nucleo.EspUsuario:
			usuario[1]++
			if cumpleEsperado(c, r) {
				usuario[0]++
			}
		case c.ConEsperado() && c.Expectativa == nucleo.EspInterpretacion:
			interp[1]++
			if cumpleEsperado(c, r) {
				interp[0]++
			}
		case c.ConEsperado() && c.Expectativa == nucleo.EspReferencia:
			g := porNota[c.Nota]
			if g == nil {
				g = &grupoRef{nota: c.Nota}
				porNota[c.Nota] = g
				refs = append(refs, g)
			}
			g.total++
			if cumpleEsperado(c, r) {
				g.pasan++
			}
		default:
			ninguna[1]++
			if terminado(r) {
				ninguna[0]++
			}
		}
		// properties are evaluated on cases that ran to the end and whose expectation (if any) held
		if len(props) > 0 && terminado(r) && (!c.ConEsperado() || cumpleEsperado(c, r)) {
			propTotal++
			if r.PropFallida == "" {
				propPasan++
			}
		}
	}

	var out []nucleo.Comprobacion
	nueva := func(nivel nucleo.Nivel, pasan, total int, texto string) {
		if pasan < total {
			nivel = nucleo.NivelFallo
		}
		out = append(out, nucleo.Comprobacion{Nivel: nivel, Pasan: pasan, Total: total, Texto: texto, Detalle: detalle})
	}
	if usuario[1] > 0 {
		var texto string
		switch {
		case usuario[0] == usuario[1] && usuario[1] == 1:
			texto = "Cumple tu ejemplo"
		case usuario[0] == usuario[1]:
			texto = "Cumple tus " + strconv.Itoa(usuario[1]) + " ejemplos"
		case usuario[1] == 1:
			texto = "Falla tu ejemplo"
		default:
			texto = "Falla " + strconv.Itoa(usuario[1]-usuario[0]) + " de tus " + strconv.Itoa(usuario[1]) + " ejemplos"
		}
		nueva(nucleo.NivelTusEjemplos, usuario[0], usuario[1], texto)
	}
	if interp[1] > 0 {
		nueva(nucleo.NivelEntendido, interp[0], interp[1],
			strconv.Itoa(interp[0])+"/"+strconv.Itoa(interp[1])+" pruebas según lo que entendí")
	}
	for _, g := range refs {
		con := g.nota
		if con == "" {
			con = "otra versión"
		}
		nueva(nucleo.NivelEntendido, g.pasan, g.total,
			strconv.Itoa(g.pasan)+"/"+strconv.Itoa(g.total)+" pruebas comparando con "+con)
	}
	if len(props) > 0 && propTotal > 0 {
		nueva(nucleo.NivelPropiedades, propPasan, propTotal,
			"Propiedades: "+strconv.Itoa(propPasan)+"/"+strconv.Itoa(propTotal))
	}
	if ninguna[1] > 0 {
		texto := strconv.Itoa(ninguna[0]) + " casos sin errores"
		switch {
		case ninguna[0] == 1 && ninguna[1] == 1:
			texto = "1 caso sin errores"
		case ninguna[0] < ninguna[1]:
			texto = strconv.Itoa(ninguna[1]-ninguna[0]) + " de " + strconv.Itoa(ninguna[1]) + " casos fallan"
		}
		nueva(nucleo.NivelSinFallos, ninguna[0], ninguna[1], texto)
	}
	return out
}

func detalleOrigenes(casos []nucleo.Caso, semilla int64) string {
	cuenta := map[nucleo.Origen]int{}
	var orden []nucleo.Origen
	for _, c := range casos {
		if cuenta[c.Origen] == 0 {
			orden = append(orden, c.Origen)
		}
		cuenta[c.Origen]++
	}
	partes := []string{"semilla " + strconv.FormatInt(semilla, 10)}
	var trozos []string
	for _, o := range orden {
		trozos = append(trozos, strconv.Itoa(cuenta[o])+" "+nombreOrigen(o, cuenta[o] != 1))
	}
	if len(trozos) > 0 {
		partes = append(partes, strings.Join(trozos, ", "))
	}
	return strings.Join(partes, "; ")
}

func nombreOrigen(o nucleo.Origen, plural bool) string {
	switch o {
	case nucleo.OrigenUsuario:
		if plural {
			return "tuyos"
		}
		return "tuyo"
	case nucleo.OrigenContraejemplo:
		if plural {
			return "contraejemplos"
		}
		return "contraejemplo"
	case nucleo.OrigenBorde:
		return "de borde"
	case nucleo.OrigenAzar:
		return "al azar"
	case nucleo.OrigenSonda:
		if plural {
			return "sondas"
		}
		return "sonda"
	case nucleo.OrigenReceta:
		return "de la receta"
	case nucleo.OrigenWeb:
		return "de la web"
	}
	if o == "" {
		return "sin origen"
	}
	return string(o)
}

// NivelGlobal returns fallo if any group fails; otherwise the best honest level in the order
// tus_ejemplos, entendido, propiedades, sin_fallos (and then any other level present). With no
// comprobaciones it returns sin_comprobar.
func NivelGlobal(cs []nucleo.Comprobacion) nucleo.Nivel {
	if len(cs) == 0 {
		return nucleo.NivelSinComprobar
	}
	hay := map[nucleo.Nivel]bool{}
	for _, c := range cs {
		if c.Nivel == nucleo.NivelFallo || c.Pasan < c.Total {
			return nucleo.NivelFallo
		}
		hay[c.Nivel] = true
	}
	for _, n := range []nucleo.Nivel{nucleo.NivelTusEjemplos, nucleo.NivelEntendido, nucleo.NivelPropiedades,
		nucleo.NivelSinFallos, nucleo.NivelCalculo, nucleo.NivelFuente} {
		if hay[n] {
			return n
		}
	}
	return cs[0].Nivel
}

// Tabla shows at most max cases (failures first, then the user's examples) with the columns
// Entrada | Esperado | Obtenido | ✔/✘ | Origen. max ≤ 0 means 20.
func Tabla(f nucleo.Firma, casos []nucleo.Caso, rs []nucleo.ResultadoCaso, max int) nucleo.Tabla {
	if max <= 0 {
		max = 20
	}
	res := resultados(casos, rs)
	ent := f.Entradas()
	type fila struct {
		i    int
		ok   bool
		prio int
	}
	filas := make([]fila, len(casos))
	for i, c := range casos {
		ok := false
		if c.ConEsperado() {
			ok = cumpleEsperado(c, res[i]) && (res[i].OK || res[i].PropFallida == "")
		} else {
			ok = terminado(res[i]) && res[i].PropFallida == ""
		}
		prio := 3
		switch c.Expectativa {
		case nucleo.EspUsuario:
			prio = 0
		case nucleo.EspInterpretacion:
			prio = 1
		case nucleo.EspReferencia:
			prio = 2
		}
		filas[i] = fila{i: i, ok: ok, prio: prio}
	}
	sort.SliceStable(filas, func(a, b int) bool {
		if filas[a].ok != filas[b].ok {
			return !filas[a].ok
		}
		return filas[a].prio < filas[b].prio
	})
	if len(filas) > max {
		filas = filas[:max]
	}
	tb := nucleo.Tabla{Cabecera: []string{"Entrada", "Esperado", "Obtenido", "✔/✘", "Origen"}}
	for _, fl := range filas {
		c := casos[fl.i]
		r := res[fl.i]
		entrada := make([]string, len(c.Entradas))
		for j, v := range c.Entradas {
			var t nucleo.Tipo
			if j < len(ent) {
				t = ent[j]
			}
			entrada[j] = nucleo.FormatoHumano(v, t)
		}
		esperado := "—"
		if c.ConEsperado() {
			esperado = mostrarValores(c.Esperado, f.Res)
		}
		marca, estado := "✔", "bien"
		if !fl.ok {
			marca, estado = "✘", "mal"
		}
		tb.Filas = append(tb.Filas, []string{strings.Join(entrada, ", "), esperado, mostrarObtenido(r, f.Res), marca, origenCaso(c)})
		tb.Marcas = append(tb.Marcas, estado)
	}
	return tb
}

func mostrarValores(vs []nucleo.Valor, ts []nucleo.Tipo) string {
	partes := make([]string, len(vs))
	for i, v := range vs {
		var t nucleo.Tipo
		if i < len(ts) {
			t = ts[i]
		}
		partes[i] = nucleo.FormatoHumano(v, t)
	}
	return strings.Join(partes, ", ")
}

func mostrarObtenido(r *nucleo.ResultadoCaso, ts []nucleo.Tipo) string {
	switch {
	case r == nil || !r.Ejecutado:
		return "sin ejecutar"
	case r.Agotado:
		return "tardó demasiado"
	case r.SinCombustible:
		return "no terminó (¿bucle infinito?)"
	case r.Caida:
		if r.Panico != "" {
			return "el programa se cayó: " + r.Panico
		}
		return "el programa se cayó"
	case r.Panico != "":
		return "pánico: " + r.Panico
	}
	s := mostrarValores(r.Obtenido, ts)
	if r.NoDeterminista {
		s += " (cambia de una vez a otra)"
	}
	if r.PropFallida != "" {
		s += " (no cumple «" + r.PropFallida + "»)"
	}
	return s
}

func origenCaso(c nucleo.Caso) string {
	if c.Nota != "" {
		return c.Nota
	}
	return nombreOrigen(c.Origen, false)
}
