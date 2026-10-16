package cerebro

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/nucleo"
)

// The reasoning pipeline (§4.14.2 "Reasoning"): the ranked mates/logica/puzles resolvers. Each one
// checks itself; cerebro only refuses to show an answer that its resolver could not check, unless it
// is labelled "aproximado".

// repartos are the shares of the budget for the top 3 resolvers (§4.14.1 step 7).
var repartos = []float64{0.60, 0.25, 0.15}

// intento is one resolver run.
type intento struct {
	cd  candidato
	r   nucleo.Respuesta
	err error
}

// verificada: a checked answer (Nivel calculo, tus_ejemplos or entendido), or an approximate one
// that says so.
func (it intento) verificada() bool {
	if it.err != nil || !it.r.Exito {
		return false
	}
	return nivelRazonadoValido(it.r.Nivel) || it.r.Metodo == "aproximado"
}

// honesta: no claim of success, but an answer worth showing ("No lo sé", "posible, pero con pocos datos").
func (it intento) honesta() bool {
	return it.err == nil && !it.r.Exito && strings.TrimSpace(it.r.Texto) != ""
}

func nivelRazonadoValido(n nucleo.Nivel) bool {
	return n == nucleo.NivelCalculo || n == nucleo.NivelTusEjemplos || n == nucleo.NivelEntendido
}

// razonarIntencion runs every resolver of a reasoning intent's package (used by despachar).
func (c *Cerebro) razonarIntencion(k *tarea, i nucleo.TipoIntencion) nucleo.Respuesta {
	return c.razonar(k, c.candidatosRazonar(k.q, i), false)
}

// razonar runs the resolvers: the first two at once when paralelo (half the budget each), then the
// rest one after another with the shares 60/25/15 of what is left, stopping at the first checked
// answer. Two checked answers that differ give the question "¿Cuál querías decir?".
func (c *Cerebro) razonar(k *tarea, lista []candidato, paralelo bool) nucleo.Respuesta {
	r := c.razonarSinLexico(k, lista, paralelo)
	c.ensenarSignifica(k, &r)
	return r
}

func (c *Cerebro) razonarSinLexico(k *tarea, lista []candidato, paralelo bool) nucleo.Respuesta {
	if len(lista) > 0 {
		k.intencion = lista[0].intencion
	}
	var intentos []intento
	usados := 0
	if paralelo && len(lista) >= 2 {
		a, b := c.resolverParalelo(k, lista[0], lista[1])
		intentos = append(intentos, a, b)
		usados = 2
		switch va, vb := a.verificada(), b.verificada(); {
		case va && vb && !mismaRespuesta(a.r, b.r):
			return c.terminarRazonar(k, c.cualQuerias(k, a, b), intentos)
		case va:
			return c.terminarRazonar(k, a, intentos)
		case vb:
			return c.terminarRazonar(k, b, intentos)
		}
	}
	for i := usados; i < len(lista) && i < len(repartos); i++ {
		if k.ctx.Err() != nil {
			break
		}
		suma := 0.0
		for _, x := range repartos[i:] {
			suma += x
		}
		d := time.Duration(float64(k.restante()) * repartos[i] / suma)
		if i == len(lista)-1 || i == len(repartos)-1 {
			d = k.restante() // the last one gets whatever is left
		}
		it := c.resolverUno(k, lista[i], d)
		intentos = append(intentos, it)
		if it.verificada() {
			return c.terminarRazonar(k, it, intentos)
		}
	}
	return c.sinRespuestaRazonada(k, intentos)
}

// resolverUno runs one resolver with its own time slice, under a trace node.
func (c *Cerebro) resolverUno(k *tarea, cd candidato, d time.Duration) intento {
	nodo := k.raiz.Sub(nucleo.PasoPlan, "Lo intento como %s", cd.lectura)
	ctx, cancelar := context.WithTimeout(k.ctx, d)
	defer cancelar()
	it := intento{cd: cd}
	if e := seguro(func() error {
		it.r, it.err = cd.res.Resolver(ctx, k.q, nodo)
		return nil
	}); e != nil {
		it.err = e
	}
	if it.err == nil && !it.r.Exito && ctx.Err() != nil && strings.TrimSpace(it.r.Texto) == "" {
		it.err = fmt.Errorf("%w", nucleo.ErrSinTiempo)
	}
	switch {
	case it.verificada():
		nodo.Bien("")
	case it.honesta():
		nodo.Info("")
	case it.err == nil:
		nodo.Mal("No doy por buena esta respuesta: no la pude comprobar")
		it.err = errSinComprobar
	default:
		nodo.Mal("%s: %s", mayusculaInicial(cd.lectura), mensajeError(it.err))
	}
	return it
}

var errSinComprobar = errors.New("la respuesta no se pudo comprobar")

// resolverParalelo runs two resolvers at once, each with half the budget.
func (c *Cerebro) resolverParalelo(k *tarea, a, b candidato) (intento, intento) {
	k.raiz.Nota("Dudo entre «%s» y «%s»: pruebo las dos", a.lectura, b.lectura)
	mitad := k.restante() / 2
	var ia, ib intento
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); ia = c.resolverUno(k, a, mitad) }()
	go func() { defer wg.Done(); ib = c.resolverUno(k, b, mitad) }()
	wg.Wait()
	return ia, ib
}

// mismaRespuesta compares two answers by their first line, ignoring case and accents.
func mismaRespuesta(a, b nucleo.Respuesta) bool {
	return nucleo.Normalizar(primeraLinea(a.Texto)) == nucleo.Normalizar(primeraLinea(b.Texto))
}

func primeraLinea(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// cualQuerias asks which of two checked but different answers the user meant, and keeps the other as
// a paragraph so both are shown. Without an answer the first (better ranked) one stays.
func (c *Cerebro) cualQuerias(k *tarea, a, b intento) intento {
	oa := recortar(mayusculaInicial(a.cd.lectura)+": "+primeraLinea(a.r.Texto), 90)
	ob := recortar(mayusculaInicial(b.cd.lectura)+": "+primeraLinea(b.r.Texto), 90)
	if oa == ob {
		oa, ob = "1) "+oa, "2) "+ob
	}
	elegida, otra := a, b
	if resp, ok := c.preguntar(k, "Lo he resuelto de dos formas y dan distinto. ¿Cuál querías decir?", []string{oa, ob}); ok &&
		nucleo.Normalizar(resp) == nucleo.Normalizar(ob) {
		elegida, otra = b, a
	}
	elegida.r.Parrafos = append(elegida.r.Parrafos,
		fmt.Sprintf("Si querías decir otra cosa (%s): %s", otra.cd.lectura, primeraLinea(otra.r.Texto)))
	return elegida
}

// terminarRazonar records the winner and the failed strategies.
func (c *Cerebro) terminarRazonar(k *tarea, it intento, intentos []intento) nucleo.Respuesta {
	k.turno.Resolutor = it.cd.nombre
	k.estrategia = it.cd.nombre
	k.intencion = it.cd.intencion
	k.turno.Metodo = it.r.Metodo
	for _, x := range intentos {
		if x.cd.nombre != it.cd.nombre && !x.verificada() {
			k.fallidas = append(k.fallidas, x.cd.nombre)
		}
	}
	r := it.r
	if r.Comprobaciones == nil {
		r.Comprobaciones = []nucleo.Comprobacion{}
	}
	return r
}

// ejemplosRazonar are the "Prueba a escribirlo así" formats per reasoning intent.
var ejemplosRazonar = map[nucleo.TipoIntencion][2]string{
	nucleo.ICalculo:        {"cuánto es 3/4+1/6", "el 15% de 240"},
	nucleo.IEcuacion:       {"resuelve 2x+3=11", "x+y=10, x-y=2"},
	nucleo.IProblema:       {"el doble de un número más 3 es 11", "María tiene 12 caramelos y regala 5. ¿Cuántos le quedan?"},
	nucleo.INumeros:        {"factoriza 600", "¿es primo 97?"},
	nucleo.ILogica:         {"si llueve el suelo se moja; llueve. ¿se moja?", "¿es «p o no p» siempre verdad?"},
	nucleo.ISilogismo:      {"todos los hombres son mortales; Sócrates es un hombre; ¿Sócrates es mortal?", "todos los A son B, algunos B son C, ¿algunos A son C?"},
	nucleo.IRecordar:       {"recuerda que Toby es un perro", "recuerda que todos los perros son mamíferos"},
	nucleo.IPreguntarHecho: {"¿Toby es mamífero?", "¿Toby es un perro?"},
	nucleo.IPuzle:          {"SEND+MORE=MONEY", "¿cuántas soluciones tiene el problema de las 8 reinas?"},
	nucleo.IConteo:         {"¿de cuántas formas elijo 3 de 10?", "probabilidad de sacar 2 caras al lanzar 3 monedas"},
	nucleo.ISecuencia:      {"2, 5, 10, 17, ¿siguiente?", "1, 1, 2, 3, 5, 8, ¿siguiente?"},
	nucleo.IPlan:           {"jarras de 3 y 5 litros, ¿cómo saco 4?", "torres de Hanói con 3 discos"},
}

// sinRespuestaRazonada: no resolver gave a checked answer. An honest unchecked answer is shown as
// is; otherwise the failure says why ("No sé plantearlo. Prueba a escribirlo así: …").
func (c *Cerebro) sinRespuestaRazonada(k *tarea, intentos []intento) nucleo.Respuesta {
	for _, it := range intentos {
		if it.honesta() {
			r := c.terminarRazonar(k, it, intentos)
			if r.Nivel == "" || nivelRazonadoValido(r.Nivel) {
				r.Nivel = nucleo.NivelSinComprobar
			}
			return r
		}
	}
	for _, it := range intentos {
		k.fallidas = append(k.fallidas, it.cd.nombre)
	}
	var tiempo, noSoportado, otros []string
	noEntiendo := 0
	for _, it := range intentos {
		switch {
		case errors.Is(it.err, nucleo.ErrSinTiempo), errors.Is(it.err, context.DeadlineExceeded):
			tiempo = append(tiempo, it.cd.lectura)
		case errors.Is(it.err, nucleo.ErrNoSoportado):
			noSoportado = append(noSoportado, mensajeError(it.err))
		case errors.Is(it.err, nucleo.ErrNoEntiendo):
			noEntiendo++
		default:
			otros = append(otros, mensajeError(it.err))
		}
	}
	ej := ejemplosRazonar[k.intencion]
	if ej[0] == "" {
		ej = ejemplosRazonar[nucleo.IEcuacion]
	}
	r := nucleo.Respuesta{
		Texto:          "No he podido resolverlo.",
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelFallo,
		Metodo:         "razonar",
	}
	switch {
	case errors.Is(k.ctxPadre.Err(), context.Canceled):
		r.Texto = "Lo he parado antes de terminar."
		k.pendiente = ""
		return r
	case len(tiempo) > 0:
		r.Texto = "Se me acabó el tiempo antes de terminar."
		r.Parrafos = append(r.Parrafos, "Pulsa «Pensar más» para darme el doble de tiempo.")
		k.pendiente = "sin_tiempo"
	case len(noSoportado) > 0:
		r.Texto = "Todavía no sé resolver esto."
		r.Parrafos = append(r.Parrafos, mayusculaInicial(noSoportado[0])+".")
		k.pendiente = "no_soportado"
	case len(otros) > 0:
		r.Parrafos = append(r.Parrafos, mayusculaInicial(otros[0])+".")
		k.pendiente = "error"
	default:
		k.pendiente = "no_entiendo"
	}
	r.Parrafos = append(r.Parrafos, fmt.Sprintf("No sé plantearlo. Prueba a escribirlo así: «%s» o «%s».", ej[0], ej[1]))
	return r
}

// reSignifica finds "X significa Y" / "X quiere decir Y" in normalized text.
var reSignifica = regexp.MustCompile(`(?:^|[.;:,]\s*|\bque\s+)"?([\pL][\pL_-]*)"?\s+(?:significa|quiere decir)\s+(?:lo mismo que\s+)?"?([\pL][\pL_-]*(?:\s[\pL][\pL_-]*)?)"?\s*(?:[.;,]|$)`)

var noPalabra = map[string]bool{"que": true, "lo": true, "eso": true, "esto": true, "esa": true, "ese": true, "esta": true, "este": true, "no": true}

// ensenarSignifica: "X significa Y" also teaches the lexicon (EnsenarPalabra) when Y is a known
// concept word (§4.14.2 Reasoning). If logic could not store it, this alone answers.
func (c *Cerebro) ensenarSignifica(k *tarea, r *nucleo.Respuesta) {
	if k.q == nil || strings.Contains(k.q.Texto, "?") {
		return
	}
	texto := nucleo.Normalizar(k.q.Texto)
	var aprendidas []string
	for _, m := range reSignifica.FindAllStringSubmatch(texto, -1) {
		x, y := m[1], strings.TrimSpace(m[2])
		if noPalabra[x] || x == y {
			continue
		}
		concepto := conceptoDePalabras(y, k.lex)
		if concepto == "" {
			continue
		}
		if err := c.mem.EnsenarPalabra(x, concepto); err != nil {
			k.raiz.Sub(nucleo.PasoMemoria, "No pude aprender la palabra «%s»: %s", x, mensajeError(err)).Mal("")
			continue
		}
		k.raiz.Sub(nucleo.PasoMemoria, "Aprendo una palabra: «%s» quiere decir %s", x, strings.ReplaceAll(concepto, "_", " ")).Bien("")
		aprendidas = append(aprendidas, fmt.Sprintf("«%s» quiere decir %s", x, strings.ReplaceAll(concepto, "_", " ")))
	}
	if len(aprendidas) == 0 {
		return
	}
	k.lex = c.lexico()
	frase := "Ya sé que " + strings.Join(aprendidas, " y que ")
	if r.Exito {
		if r.Aprendido != "" {
			r.Aprendido += ". " + frase
		} else {
			r.Aprendido = frase
		}
		return
	}
	k.pendiente = ""
	k.fallidas = nil
	k.estrategia = "lexico"
	*r = nucleo.Respuesta{
		Texto:          "Entendido: " + strings.Join(aprendidas, "; ") + ".",
		Aprendido:      frase,
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Pasan: len(aprendidas), Total: len(aprendidas), Texto: "Guardado en mi memoria"}},
		Nivel:          nucleo.NivelEntendido,
		Metodo:         "lexico",
		Exito:          true,
	}
}

// conceptoDePalabras maps "palíndromo" or "número primo" to a concept of nucleo.Conceptos ("" if none).
func conceptoDePalabras(y string, lex *lengua.Lexico) string {
	y = strings.TrimSpace(nucleo.Normalizar(y))
	if c := strings.ReplaceAll(y, " ", "_"); nucleo.EsConcepto(c) {
		return c
	}
	palabras := strings.Fields(y)
	for i := len(palabras) - 1; i >= 0; i-- {
		p := palabras[i]
		if nucleo.EsConcepto(p) {
			return p
		}
		if lex != nil {
			if c, ok := lex.Concepto(lengua.Lema(p)); ok {
				return c
			}
			if c, ok := lex.Concepto(p); ok {
				return c
			}
		}
	}
	return ""
}

// rerazonarCon re-runs one named resolver on a new question (follow-ups "¿y si fuera 13?" and
// "otra forma"), falling back to the ranking when the resolver no longer recognizes it.
func (c *Cerebro) rerazonarCon(k *tarea, nombre string, i nucleo.TipoIntencion, excluir map[string]bool) nucleo.Respuesta {
	lista := c.candidatosRazonar(k.q, i)
	if nombre != "" {
		lista = primeroElResolutor(lista, nombre)
	}
	var filtrada []candidato
	for _, cd := range lista {
		if !excluir[cd.nombre] {
			filtrada = append(filtrada, cd)
		}
	}
	return c.conPresupuesto(k, i, func() nucleo.Respuesta { return c.razonar(k, filtrada, false) })
}
