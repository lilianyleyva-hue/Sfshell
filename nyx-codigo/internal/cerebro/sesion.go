package cerebro

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Sessions (§4.14.1 step 1): in memory, at most MaxTurnos turns, forgotten after Caducidad idle.
const (
	MaxTurnos = 30
	Caducidad = 24 * time.Hour
)

// Turno is one answered request of a session.
type Turno struct {
	Tarea     string
	Pregunta  *nucleo.Pregunta
	Intencion nucleo.TipoIntencion
	Respuesta nucleo.Respuesta
	Funcion   *nucleo.FuncionAprendida // produced/used function
	Expr      string                   // DSL program, if any
	Excluidos []string                 // programs (or resolver names, for reasoning) already shown, for "otra forma"
	Metodo    string
	Resolutor string
	Fecha     time.Time

	Peticion    nucleo.Peticion     // the request as received ("Pensar más" re-runs it)
	Texto       string              // the question this turn answered (after follow-up rewriting)
	Codigo      string              // code worked on or produced
	Presupuesto time.Duration       // budget used ("Pensar más" doubles it)
	Propuestas  map[string][]string // lexicon proposals made (lemma → concepts), confirmed by "está bien"
	Seguimiento string              // follow-up type that produced this turn ("" = a new request)
	Meta        bool                // only talks about the previous turn (por qué, está bien…): not a new context
}

type sesion struct {
	id         string
	mu         sync.Mutex
	turnos     []Turno
	ctx        nucleo.Contexto
	usada      time.Time
	pendientes []*preguntaPendiente
}

// preguntaPendiente is a question that a running task of this session is waiting on. A new request in
// the same session whose text answers it is routed here instead of starting a task (§4.14.4).
type preguntaPendiente struct {
	tarea    string
	pregunta nucleo.PreguntaUsuario
	tipo     *nucleo.Tipo // expected type of a typed answer, or nil
	ch       chan string  // buffered (1)
	usada    atomic.Bool
}

// acepta reports whether the typed text answers the question: it equals an option (ignoring case,
// accents and final punctuation) or parses as a value of the expected type.
func (pp *preguntaPendiente) acepta(texto string) (string, bool) {
	t := strings.TrimSpace(texto)
	n := strings.Trim(nucleo.Normalizar(t), " .;,:")
	if n == "" {
		return "", false
	}
	for _, o := range pp.pregunta.Opciones {
		if strings.Trim(nucleo.Normalizar(o), " .;,:") == n {
			return o, true
		}
	}
	if pp.tipo != nil {
		if _, err := nucleo.ParseValor(t, *pp.tipo); err == nil {
			return t, true
		}
	}
	return "", false
}

// entregar delivers an answer once; false if it was already answered.
func (pp *preguntaPendiente) entregar(s string) bool {
	if !pp.usada.CompareAndSwap(false, true) {
		return false
	}
	pp.ch <- s
	return true
}

// sesion returns the session with that id, creating it, and forgets sessions idle for Caducidad.
func (c *Cerebro) sesion(id string) *sesion {
	ahora := c.ahora()
	c.mu.Lock()
	defer c.mu.Unlock()
	if ahora.Sub(c.ultimaLimpieza) > time.Minute {
		c.ultimaLimpieza = ahora
		for k, s := range c.sesiones {
			s.mu.Lock()
			viejo := ahora.Sub(s.usada) > Caducidad && len(s.pendientes) == 0
			s.mu.Unlock()
			if viejo {
				delete(c.sesiones, k)
			}
		}
	}
	s := c.sesiones[id]
	if s != nil {
		s.mu.Lock()
		if ahora.Sub(s.usada) > Caducidad && len(s.pendientes) == 0 {
			s.turnos, s.ctx = nil, nucleo.Contexto{}
		}
		s.usada = ahora
		s.mu.Unlock()
		return s
	}
	s = &sesion{id: id, usada: ahora}
	c.sesiones[id] = s
	return s
}

// ultimo returns the last turn that is not a meta turn nor a "no te entiendo": a misunderstanding is
// not the context the next follow-up talks about.
func (s *sesion) ultimo() (Turno, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.turnos) - 1; i >= 0; i-- {
		if !s.turnos[i].Meta && !noEntendido(s.turnos[i]) {
			return s.turnos[i], true
		}
	}
	return Turno{}, false
}

// noEntendido: the turn was the "no te entiendo" template.
func noEntendido(t Turno) bool {
	return t.Metodo == "no_entiendo" || t.Respuesta.Metodo == "no_entiendo"
}

// baseCreacion finds the request that created the code the conversation is about, for "ahora con
// impares": the newest creation turn, walking back over meta turns, misunderstandings and follow-ups
// about that code (pruébala, explícalo, hazlo programa…). comoPrograma: the code was turned into a
// program after it ("hazlo programa"). ok=false when another kind of request came in between.
func (s *sesion) baseCreacion() (base Turno, comoPrograma, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.turnos) - 1; i >= 0; i-- {
		t := s.turnos[i]
		switch {
		case t.Meta || noEntendido(t):
			continue
		case esCreacion(t.Intencion) && t.Pregunta != nil && t.Seguimiento != "programa_eso":
			return t, comoPrograma, true
		case t.Seguimiento != "" && nucleo.Familia(t.Intencion) == "programar":
			if t.Seguimiento == "programa_eso" {
				comoPrograma = true
			}
			continue
		}
		return Turno{}, false, false
	}
	return Turno{}, false, false
}

// contexto returns a copy of the session context (for lengua).
func (s *sesion) contexto() nucleo.Contexto {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.ctx
	c.Numeros = append([]nucleo.Numero(nil), s.ctx.Numeros...)
	return c
}

func (s *sesion) buscarTurno(tarea string) (Turno, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.turnos) - 1; i >= 0; i-- {
		if s.turnos[i].Tarea == tarea {
			return s.turnos[i], true
		}
	}
	return Turno{}, false
}

func (s *sesion) numTurnos() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.turnos)
}

// registrar appends the task's turn (at most MaxTurnos are kept) and, unless it is a meta turn,
// updates the session context that lengua uses for follow-ups.
func (s *sesion) registrar(k *tarea, r nucleo.Respuesta) {
	t := *k.turno
	t.Tarea = k.p.Tarea
	t.Peticion = k.p
	t.Respuesta = r
	t.Intencion = k.intencion
	t.Presupuesto = k.presupuesto
	t.Fecha = k.inicio
	t.Meta = k.meta
	if t.Pregunta == nil {
		t.Pregunta = k.q
	}
	if t.Texto == "" {
		t.Texto = k.p.Texto
	}
	if t.Metodo == "" {
		t.Metodo = r.Metodo
	}
	if r.Codigo != "" {
		t.Codigo = r.Codigo
	} else if t.Codigo == "" {
		t.Codigo = k.codigo
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turnos = append(s.turnos, t)
	if len(s.turnos) > MaxTurnos {
		s.turnos = append([]Turno(nil), s.turnos[len(s.turnos)-MaxTurnos:]...)
	}
	if t.Meta || noEntendido(t) {
		return
	}
	ctx := s.ctx
	ctx.Intencion = t.Intencion
	ctx.Texto = t.Texto
	if t.Codigo != "" {
		ctx.Codigo = t.Codigo
	}
	if t.Pregunta != nil {
		ctx.Marco = t.Pregunta.Marco
		ctx.Numeros = append([]nucleo.Numero(nil), t.Pregunta.Numeros...)
		if t.Pregunta.Firma != nil && nucleo.Familia(t.Intencion) == "programar" {
			f := *t.Pregunta.Firma
			ctx.Firma = &f
		}
	}
	if t.Funcion != nil {
		f := t.Funcion.Firma
		ctx.Firma = &f
		ctx.Funcion = t.Funcion.Nombre
	}
	s.ctx = ctx
}

func (s *sesion) ponerPendiente(pp *preguntaPendiente) {
	s.mu.Lock()
	s.pendientes = append(s.pendientes, pp)
	s.mu.Unlock()
}

func (s *sesion) quitarPendiente(pp *preguntaPendiente) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, x := range s.pendientes {
		if x == pp {
			s.pendientes = append(s.pendientes[:i], s.pendientes[i+1:]...)
			return
		}
	}
}

// pendienteQueAcepta returns the newest pending question that the text answers.
func (s *sesion) pendienteQueAcepta(texto string) (*preguntaPendiente, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.pendientes) - 1; i >= 0; i-- {
		if r, ok := s.pendientes[i].acepta(texto); ok {
			return s.pendientes[i], r
		}
	}
	return nil, ""
}

// rutearRespuestaPendiente: when a task of this session is waiting for an answer and the new text
// answers it, the text goes to that task instead of starting a new one (§4.14.4, last paragraph).
func (c *Cerebro) rutearRespuestaPendiente(s *sesion, p nucleo.Peticion, t *nucleo.Traza) (nucleo.Respuesta, bool) {
	if p.Accion != "" || strings.TrimSpace(p.Codigo) != "" {
		return nucleo.Respuesta{}, false
	}
	pp, valor := s.pendienteQueAcepta(p.Texto)
	if pp == nil || !pp.entregar(valor) {
		return nucleo.Respuesta{}, false
	}
	t.Raiz().Sub(nucleo.PasoNota, "Es tu respuesta a mi pregunta «%s»: «%s»", pp.pregunta.Texto, valor)
	return nucleo.Respuesta{
		Texto:          fmt.Sprintf("Lo tomo como respuesta a mi pregunta: «%s».", valor),
		Parrafos:       []string{"Sigo con la tarea que estaba esperando tu respuesta."},
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "respuesta",
		Exito:          true,
	}, true
}

// preguntarCon asks the user through the trace and waits (at most c.espera, with the budget paused).
// A typed answer in the same session (see rutearRespuestaPendiente) also counts. ok=false: nobody
// answered (no responder, timeout or cancel); then the caller uses its default.
func (c *Cerebro) preguntarCon(k *tarea, pu nucleo.PreguntaUsuario, tipo *nucleo.Tipo) (string, bool) {
	if pu.ID == "" {
		pu.ID = fmt.Sprintf("p%d", k.numPreguntas.Add(1))
	}
	nodo := k.raiz.Sub(nucleo.PasoPregunta, "%s", pu.Texto)
	reanudar := k.pausar()
	defer reanudar()
	pp := &preguntaPendiente{tarea: k.p.Tarea, pregunta: pu, tipo: tipo, ch: make(chan string, 1)}
	k.s.ponerPendiente(pp)
	defer k.s.quitarPendiente(pp)

	ctx, cancelar := context.WithTimeout(k.ctxPadre, c.espera)
	defer cancelar()
	type res struct {
		s   string
		err error
	}
	out := make(chan res, 1)
	go func() {
		var r res
		err := seguro(func() error {
			s, e := k.t.Preguntar(ctx, pu)
			r = res{s, e}
			return nil
		})
		if err != nil {
			r.err = err
		}
		out <- r
	}()
	var r res
	select {
	case r = <-out:
		if r.err == nil && !pp.usada.CompareAndSwap(false, true) {
			// answered by typing at the same moment: that answer wins
			r = res{s: <-pp.ch}
		}
	case s := <-pp.ch:
		cancelar()
		r = res{s: s}
	}
	if r.err != nil {
		nodo.Info("Nadie respondió: sigo con lo más probable")
		return pu.PorDefecto, false
	}
	nodo.Bien("Me respondiste: %s", r.s)
	return r.s, true
}
