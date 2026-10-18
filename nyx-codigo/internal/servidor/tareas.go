package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// The task registry: each request runs as a tarea with an append-only event buffer. Subscribers wait
// for new events on a channel that is closed and replaced on every append (a broadcast that, unlike
// sync.Cond, can be combined with the heartbeat timer and the request context in one select).

// gracia is how long the server waits for the Motor to notice a cancellation before it closes the
// stream itself.
const gracia = 5 * time.Second

type eventoSSE struct {
	seq   int
	tipo  string
	datos []byte
}

type tarea struct {
	id       string
	peticion nucleo.Peticion
	creada   time.Time
	cancel   context.CancelFunc
	limite   time.Duration // hard limit, counted from the moment the task gets a run slot

	mu        sync.Mutex
	eventos   []eventoSSE
	cambio    chan struct{}
	terminada bool
	finEn     time.Time
	cancelada bool
	preguntas map[string]chan string
	// respondidas are the question channels that already took their answer: the Motor may read the
	// answer before its deferred cleanup removes the question, and a second /responder in between must
	// still be refused (409), not accepted into an empty channel and lost.
	respondidas map[chan string]bool
	ahora       func() time.Time
}

// agregar is the trace's emitir: it is called with the trace's mutex held and never calls back into it.
func (t *tarea) agregar(e nucleo.Evento) {
	var v any
	switch e.Tipo {
	case "paso", "actualiza":
		v = e.Paso
	case "pregunta":
		v = e.Pregunta
	case "respuesta":
		v = e.Respuesta
	case "fin":
		v = e.Fin
	default:
		return
	}
	datos, err := json.Marshal(v)
	if err != nil {
		datos = []byte(`{}`)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.eventos) >= maxEventos && (e.Tipo == "paso" || e.Tipo == "actualiza") {
		return
	}
	t.eventos = append(t.eventos, eventoSSE{seq: e.Seq, tipo: e.Tipo, datos: datos})
	switch e.Tipo {
	case "pregunta":
		if e.Pregunta != nil && t.preguntas[e.Pregunta.ID] == nil {
			// registered here, before the Motor waits, so a fast answer is never lost
			t.preguntas[e.Pregunta.ID] = make(chan string, 1)
		}
	case "fin":
		if !t.terminada {
			t.terminada = true
			t.finEn = t.ahora()
		}
	}
	close(t.cambio)
	t.cambio = make(chan struct{})
}

// desde returns the events with Seq > n, whether the task has finished, and the channel that will be
// closed on the next event.
func (t *tarea) desde(n int) ([]eventoSSE, bool, <-chan struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	i := sort.Search(len(t.eventos), func(i int) bool { return t.eventos[i].seq > n })
	return append([]eventoSSE(nil), t.eventos[i:]...), t.terminada, t.cambio
}

func (t *tarea) yaTermino() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.terminada
}

func (t *tarea) fueCancelada() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancelada
}

func (t *tarea) marcarCancelada() {
	t.mu.Lock()
	if !t.terminada {
		t.cancelada = true
	}
	t.mu.Unlock()
	t.cancel()
}

func (t *tarea) cancelar() { t.marcarCancelada() }

// esperarRespuesta is the trace's responder: it waits for /responder, ctx, or the timeout.
func (t *tarea) esperarRespuesta(ctx context.Context, p nucleo.PreguntaUsuario, espera time.Duration) (string, error) {
	t.mu.Lock()
	ch := t.preguntas[p.ID]
	if ch == nil {
		ch = make(chan string, 1)
		t.preguntas[p.ID] = ch
	}
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		if t.preguntas[p.ID] == ch {
			delete(t.preguntas, p.ID)
		}
		delete(t.respondidas, ch)
		t.mu.Unlock()
	}()
	reloj := time.NewTimer(espera)
	defer reloj.Stop()
	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return p.PorDefecto, fmt.Errorf("%w: %w", nucleo.ErrSinRespuesta, ctx.Err())
	case <-reloj.C:
		return p.PorDefecto, nucleo.ErrSinRespuesta
	}
}

// contestar delivers an answer; false when no such question is pending (or it was already answered).
func (t *tarea) contestar(id, respuesta string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	ch := t.preguntas[id]
	if ch == nil || t.respondidas[ch] {
		return false
	}
	select {
	case ch <- respuesta:
		if t.respondidas == nil {
			t.respondidas = map[chan string]bool{}
		}
		t.respondidas[ch] = true
		return true
	default:
		return false
	}
}

// ---- running ----

// limpiar forgets finished tasks older than Retener.
func (s *Servidor) limpiar() {
	ahora := s.ahora()
	s.mu.Lock()
	var viejas []*tarea
	for id, t := range s.tareas {
		t.mu.Lock()
		if t.terminada && ahora.Sub(t.finEn) > s.c.Retener {
			viejas = append(viejas, t)
			delete(s.tareas, id)
		}
		t.mu.Unlock()
	}
	s.mu.Unlock()
	for _, t := range viejas {
		t.cancel()
	}
}

// lanzar registers a task and starts it in the background.
func (s *Servidor) lanzar(p nucleo.Peticion) string {
	s.limpiar()
	id := nuevoID()
	p.Tarea = id
	limite := s.c.Limite
	if p.Accion == "estudiar" {
		limite = s.c.LimiteEstudio
	}
	// only a cancel here: the hard limit starts in correr, once the task has a slot, so time spent
	// queued behind MaxTareas running tasks does not eat into its budget
	ctx, cancel := context.WithCancel(context.Background())
	t := &tarea{id: id, peticion: p, creada: s.ahora(), cancel: cancel, limite: limite, cambio: make(chan struct{}),
		preguntas: map[string]chan string{}, ahora: s.ahora}
	tr := nucleo.NuevaTraza(t.agregar, func(c context.Context, q nucleo.PreguntaUsuario) (string, error) {
		return t.esperarRespuesta(c, q, s.c.EsperaPregunta)
	})
	s.mu.Lock()
	s.tareas[id] = t
	s.mu.Unlock()
	go s.correr(ctx, t, tr)
	return id
}

type resultado struct {
	r      nucleo.Respuesta
	panico any
}

func (s *Servidor) correr(base context.Context, t *tarea, tr *nucleo.Traza) {
	defer t.cancel()
	// wait for a free slot (MaxTareas running at once). base ends only when the user (or Cerrar)
	// cancels; a task that waits more than twice its own limit (a running task normally frees its slot
	// within one limit) gives up with an honest error instead.
	select {
	case s.cupos <- struct{}{}:
	default:
		tr.Raiz().Nota("Espero mi turno: hay otras tareas en marcha")
		cola := time.NewTimer(2 * t.limite)
		select {
		case s.cupos <- struct{}{}:
			cola.Stop()
		case <-base.Done():
			cola.Stop()
			s.terminar(base, t, tr, nucleo.Respuesta{Texto: "Lo he parado antes de empezar."}, "cancelada")
			return
		case <-cola.C:
			s.terminar(base, t, tr, nucleo.Respuesta{
				Texto: "Había demasiadas tareas en marcha y no llegué a empezar. Prueba otra vez dentro de un rato.",
				Nivel: nucleo.NivelFallo, Metodo: "error",
			}, "error")
			return
		}
	}
	// the hard limit (§4.15) runs from here, when the task really starts
	ctx, cancelLimite := context.WithTimeout(base, t.limite)
	defer cancelLimite()
	hecho := make(chan resultado, 1)
	go func() {
		defer func() { <-s.cupos }() // the slot is freed only when the Motor really returns
		defer func() {
			if x := recover(); x != nil {
				hecho <- resultado{panico: x}
			}
		}()
		hecho <- resultado{r: s.m.Responder(ctx, t.peticion, tr)}
	}()
	var res resultado
	select {
	case res = <-hecho:
	case <-ctx.Done():
		select {
		case res = <-hecho:
		case <-time.After(gracia):
			s.terminar(ctx, t, tr, nucleo.Respuesta{Texto: "No pude pararme a tiempo; lo dejo aquí.", Nivel: nucleo.NivelFallo, Metodo: "error"}, "cancelada")
			return
		}
	}
	estado := "terminada"
	r := res.r
	if res.panico != nil {
		estado = "error"
		r = nucleo.Respuesta{
			Texto:    "Lo siento: algo falló dentro de Nyx Código y no pude terminar.",
			Parrafos: []string{fmt.Sprintf("Error interno: %v", res.panico)},
			Nivel:    nucleo.NivelFallo, Metodo: "error",
		}
	} else if t.fueCancelada() {
		estado = "cancelada"
	}
	s.terminar(ctx, t, tr, r, estado)
}

// terminar emits "respuesta" and "fin" unless the Motor already did (cerebro calls Terminar itself).
func (s *Servidor) terminar(ctx context.Context, t *tarea, tr *nucleo.Traza, r nucleo.Respuesta, estado string) {
	if t.yaTermino() {
		return
	}
	if r.Tarea == "" {
		r.Tarea = t.id
	}
	if r.Comprobaciones == nil {
		r.Comprobaciones = []nucleo.Comprobacion{}
	}
	if r.Texto == "" {
		switch {
		case estado == "cancelada":
			r.Texto = "Lo he parado."
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			r.Texto = "Se me acabó el tiempo y no llegué a una respuesta."
		}
	}
	if r.Nivel == "" && !r.Exito {
		r.Nivel = nucleo.NivelSinComprobar
	}
	tr.Terminar(&r, estado)
}

// ---- SSE ----

func (s *Servidor) eventos(w http.ResponseWriter, r *http.Request) {
	t := s.tarea(r.PathValue("id"))
	if t == nil {
		errorJSON(w, http.StatusNotFound, "Esa tarea ya no existe")
		return
	}
	desde := 0
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		desde, _ = strconv.Atoi(v)
	} else if v := r.URL.Query().Get("desde"); v != "" {
		desde, _ = strconv.Atoi(v)
	}
	evs, fin, cambio := t.desde(desde)
	if fin && len(evs) == 0 {
		// nothing left: 204 tells EventSource not to reconnect
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, "retry: 2000\n\n"); err != nil {
		return
	}
	latido := time.NewTicker(s.c.Latido)
	defer latido.Stop()
	for {
		for _, e := range evs {
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.seq, e.tipo, e.datos); err != nil {
				return
			}
			desde = e.seq
			if e.tipo == "fin" {
				rc.Flush()
				return
			}
		}
		if err := rc.Flush(); err != nil {
			return
		}
		select {
		case <-cambio:
		case <-latido.C:
			if _, err := io.WriteString(w, ": latido\n\n"); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
		evs, _, cambio = t.desde(desde)
	}
}
