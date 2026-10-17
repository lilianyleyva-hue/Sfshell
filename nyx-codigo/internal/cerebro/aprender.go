package cerebro

import (
	"strings"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/sintesis"
)

// Learning hooks (§4.14.1 step 9, §9). They run after the answer is written and before the trace ends;
// they never fail the request.

// Limits on lexicon proposals per request, to keep noise out of the lexicon.
const (
	maxPalabrasPropuestas  = 3
	maxConceptosPropuestos = 3
	minLargoPalabra        = 4
)

// nivelVerificado: the answer was checked against something (examples, my reading, properties, maths).
func nivelVerificado(n nucleo.Nivel) bool {
	switch n {
	case nucleo.NivelTusEjemplos, nucleo.NivelEntendido, nucleo.NivelPropiedades, nucleo.NivelCalculo:
		return true
	}
	return false
}

// nivelAlMenosEntendido: the levels at which a request's unknown words may be proposed to the lexicon.
func nivelAlMenosEntendido(n nucleo.Nivel) bool {
	return n == nucleo.NivelTusEjemplos || n == nucleo.NivelEntendido || n == nucleo.NivelCalculo
}

// metodosSinAprendizaje are answers that teach nothing about strategies (templates, stubs, acks).
var metodosSinAprendizaje = map[string]bool{
	"charla": true, "memoria": true, "en_construccion": true, "no_entiendo": true, "respuesta": true,
	"seguimiento": true, "error": true, "habla": true,
}

// aprender: always register the episode and move the strategy counters; on success propose the
// request's unknown words to the lexicon and count the program's components; on failure keep the
// question in pendientes.
func (c *Cerebro) aprender(k *tarea, r *nucleo.Respuesta) {
	defer func() { _ = recover() }()
	ok := r.Exito && nivelVerificado(r.Nivel)
	texto := k.p.Texto
	if k.turno.Texto != "" {
		texto = k.turno.Texto
	}
	c.mem.Registrar(nucleo.Episodio{
		Fecha: k.inicio, Sesion: k.p.Sesion, Tarea: k.p.Tarea, Texto: texto, Intencion: k.intencion,
		Metodo: r.Metodo, Exito: r.Exito, Nivel: r.Nivel, Ms: c.ahora().Sub(k.inicio).Milliseconds(),
	})
	if k.meta {
		return
	}
	if !metodosSinAprendizaje[r.Metodo] && k.estrategia != "" && k.estrategia != "-" {
		c.stats.Exito("estrategia:"+k.estrategia, ok)
	}
	visto := map[string]bool{k.estrategia: true}
	for _, f := range k.fallidas {
		if !visto[f] {
			visto[f] = true
			c.stats.Exito("estrategia:"+f, false)
		}
	}
	if r.Exito && nivelAlMenosEntendido(r.Nivel) && nucleo.Familia(k.intencion) == "programar" && k.q != nil {
		c.proponerPalabras(k, r)
	}
	if ok && k.programa != nil {
		conceptos := conceptosDeTarea(k)
		c.contarPrograma(k.programa, conceptos, true)
	}
	motivo := k.pendiente
	if motivo == "" && r.Nivel == nucleo.NivelFallo && !metodosSinAprendizaje[r.Metodo] {
		motivo = "fallo"
	}
	if motivo != "" {
		c.mem.AgregarPendiente(memoria.Pendiente{Texto: texto, Codigo: k.codigo, Motivo: motivo})
	}
}

// conceptosDeTarea are the concepts of the solution: its stored function's, else the request's.
func conceptosDeTarea(k *tarea) []string {
	if k.turno.Funcion != nil && len(k.turno.Funcion.Conceptos) > 0 {
		return k.turno.Funcion.Conceptos
	}
	if k.q != nil {
		return k.q.Conceptos
	}
	return nil
}

// proponerPalabras proposes each unknown lemma of the request with each concept of the solution;
// memoria confirms a pair after memoria.ConfirmarTras successes.
func (c *Cerebro) proponerPalabras(k *tarea, r *nucleo.Respuesta) {
	conceptos := conceptosDeTarea(k)
	if len(conceptos) == 0 {
		return
	}
	if len(conceptos) > maxConceptosPropuestos {
		conceptos = conceptos[:maxConceptosPropuestos]
	}
	n := 0
	for _, lema := range lengua.PalabrasDesconocidas(k.q.Tokens, k.lex) {
		if len([]rune(lema)) < minLargoPalabra || strings.ContainsAny(lema, "0123456789_") {
			continue
		}
		if n++; n > maxPalabrasPropuestas {
			break
		}
		for _, cpt := range conceptos {
			if k.turno.Propuestas == nil {
				k.turno.Propuestas = map[string][]string{}
			}
			k.turno.Propuestas[lema] = append(k.turno.Propuestas[lema], cpt)
			if c.mem.ProponerPalabra(lema, cpt) {
				k.raiz.Sub(nucleo.PasoMemoria, "Ya sé que «%s» quiere decir %s", lema, strings.ReplaceAll(cpt, "_", " ")).Bien("")
			}
		}
	}
}

// contarPrograma moves the synthesis priors: "componente:<primitiva>" for every primitive of the
// program and "asoc:<concepto>:<primitiva>" for every concept of the request.
func (c *Cerebro) contarPrograma(e *sintesis.Expr, conceptos []string, ok bool) {
	visto := map[string]bool{}
	var recorrer func(x *sintesis.Expr)
	recorrer = func(x *sintesis.Expr) {
		if x == nil {
			return
		}
		if x.Op != nil && !visto[x.Op.Nombre] {
			visto[x.Op.Nombre] = true
			c.stats.Exito("componente:"+x.Op.Nombre, ok)
			for _, cpt := range conceptos {
				c.stats.Exito("asoc:"+cpt+":"+x.Op.Nombre, ok)
			}
		}
		for _, h := range x.Hijos {
			recorrer(h)
		}
		for _, l := range x.Lambdas {
			if l != nil {
				recorrer(l.Cuerpo)
			}
		}
	}
	recorrer(e)
}

// contarDSL is contarPrograma for a stored DSL text (used by Opinar on a wrong answer).
func (c *Cerebro) contarDSL(dsl string, f nucleo.Firma, conceptos []string, ok bool) {
	e, err := sintesis.Parse(dsl, c.registro(), f)
	if err != nil {
		return
	}
	c.contarPrograma(e, conceptos, ok)
}
