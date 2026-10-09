package nucleo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// The reasoning trace (§3.6). The trace is a tree of Paso. All *Nodo methods are safe on a nil
// receiver, so solvers run in tests without a UI. Progreso is coalesced to at most 4 events per second
// per node. A task has at most 2000 steps; after that, Sub returns nil and a single note
// "(más pasos omitidos)" is emitted.
//
// emitir is called with the trace's mutex held: it must not call back into the trace.

type TipoPaso string

const (
	PasoEntender  TipoPaso = "entender"
	PasoPlan      TipoPaso = "plan"
	PasoRecuerdo  TipoPaso = "recuerdo"
	PasoIntento   TipoPaso = "intento"
	PasoPrueba    TipoPaso = "prueba"
	PasoError     TipoPaso = "error"
	PasoArreglo   TipoPaso = "arreglo"
	PasoBusqueda  TipoPaso = "busqueda"
	PasoComprobar TipoPaso = "comprobar"
	PasoPregunta  TipoPaso = "pregunta"
	PasoMemoria   TipoPaso = "memoria"
	PasoNota      TipoPaso = "nota"
	PasoDuda      TipoPaso = "duda"
	PasoResultado TipoPaso = "resultado"
)

type Estado string

const (
	EstadoAbierto Estado = "abierto"
	EstadoBien    Estado = "bien"
	EstadoMal     Estado = "mal"
	EstadoInfo    Estado = "info"
)

type Tabla struct {
	Titulo   string     `json:"titulo,omitempty"`
	Cabecera []string   `json:"cabecera"`
	Filas    [][]string `json:"filas"`
	Marcas   []string   `json:"marcas,omitempty"` // per row: "bien" | "mal" | ""
}

type Paso struct {
	ID       int      `json:"id"`
	Padre    int      `json:"padre"` // 0 = root
	Tipo     TipoPaso `json:"tipo"`
	Titulo   string   `json:"titulo"`
	Detalle  string   `json:"detalle,omitempty"`
	Codigo   string   `json:"codigo,omitempty"`
	Tabla    *Tabla   `json:"tabla,omitempty"`
	Estado   Estado   `json:"estado"`
	Progreso string   `json:"progreso,omitempty"`
	Ms       int64    `json:"ms"` // ms since task start when last updated
}

type PreguntaUsuario struct {
	ID         string   `json:"id"`
	Texto      string   `json:"texto"`              // "¿Qué debe dar con [-2]?"
	Opciones   []string `json:"opciones,omitempty"` // button labels
	Libre      bool     `json:"libre"`              // free text allowed
	PorDefecto string   `json:"porDefecto,omitempty"`
}

// Evento is what the trace emits; servidor serializes it as SSE (§5.2).
type Evento struct {
	Seq       int              `json:"seq"`
	Tipo      string           `json:"tipo"` // "paso" | "actualiza" | "pregunta" | "respuesta" | "fin"
	Paso      *Paso            `json:"paso,omitempty"`
	Pregunta  *PreguntaUsuario `json:"pregunta,omitempty"`
	Respuesta *Respuesta       `json:"respuesta,omitempty"`
	Fin       *Fin             `json:"fin,omitempty"`
}

type Fin struct {
	Estado string `json:"estado"` // "terminada" | "cancelada" | "error"
	Ms     int64  `json:"ms"`
}

var ErrSinRespuesta = errors.New("nucleo: nadie respondió a la pregunta")

type Responder func(ctx context.Context, p PreguntaUsuario) (string, error)

type Traza struct {
	mu        sync.Mutex
	emitir    func(Evento)
	responder Responder
	inicio    time.Time
	seq, ids  int
	pasos     []Paso
	ultimoPro map[int]time.Time
	lleno     bool
	limite    int // 2000
}

// NuevaTraza: emitir and responder may be nil (tests). Pasos are always recorded for inspection.
func NuevaTraza(emitir func(Evento), responder Responder) *Traza {
	return &Traza{emitir: emitir, responder: responder, inicio: time.Now(), ultimoPro: map[int]time.Time{}, limite: 2000}
}

type Nodo struct {
	t  *Traza
	id int
}

func (t *Traza) Raiz() *Nodo {
	if t == nil {
		return nil
	}
	return &Nodo{t: t, id: 0}
}

func (t *Traza) Pasos() []Paso {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Paso(nil), t.pasos...)
}

func (t *Traza) enviar(e Evento) { // caller holds mu
	t.seq++
	e.Seq = t.seq
	if t.emitir != nil {
		t.emitir(e)
	}
}

func (t *Traza) ms() int64 { return time.Since(t.inicio).Milliseconds() }

func (t *Traza) nuevo(padre int, tipo TipoPaso, titulo string) *Nodo {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.pasos) >= t.limite {
		if !t.lleno {
			t.lleno = true
			t.ids++
			p := Paso{ID: t.ids, Padre: 0, Tipo: PasoNota, Titulo: "(más pasos omitidos)", Estado: EstadoInfo, Ms: t.ms()}
			t.pasos = append(t.pasos, p)
			t.enviar(Evento{Tipo: "paso", Paso: &p})
		}
		return nil
	}
	t.ids++
	p := Paso{ID: t.ids, Padre: padre, Tipo: tipo, Titulo: titulo, Estado: EstadoAbierto, Ms: t.ms()}
	if tipo == PasoNota {
		p.Estado = EstadoInfo
	}
	t.pasos = append(t.pasos, p)
	t.enviar(Evento{Tipo: "paso", Paso: &p})
	return &Nodo{t: t, id: p.ID}
}

func (t *Traza) cambiar(id int, f func(p *Paso)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := len(t.pasos) - 1; i >= 0; i-- {
		if t.pasos[i].ID == id {
			f(&t.pasos[i])
			t.pasos[i].Ms = t.ms()
			p := t.pasos[i]
			t.enviar(Evento{Tipo: "actualiza", Paso: &p})
			return
		}
	}
}

func (n *Nodo) Sub(tipo TipoPaso, formato string, a ...any) *Nodo {
	if n == nil {
		return nil
	}
	return n.t.nuevo(n.id, tipo, fmt.Sprintf(formato, a...))
}
func (n *Nodo) Nota(formato string, a ...any) { n.Sub(PasoNota, formato, a...) }
func (n *Nodo) Detalle(formato string, a ...any) {
	if n == nil || n.id == 0 {
		return
	}
	s := fmt.Sprintf(formato, a...)
	n.t.cambiar(n.id, func(p *Paso) {
		if p.Detalle != "" {
			p.Detalle += "\n"
		}
		p.Detalle += s
	})
}
func (n *Nodo) Codigo(src string) {
	if n == nil || n.id == 0 {
		return
	}
	n.t.cambiar(n.id, func(p *Paso) { p.Codigo = src })
}
func (n *Nodo) Tabla(tb Tabla) {
	if n == nil || n.id == 0 {
		return
	}
	n.t.cambiar(n.id, func(p *Paso) { p.Tabla = &tb })
}
func (n *Nodo) cerrar(e Estado, formato string, a ...any) {
	if n == nil || n.id == 0 {
		return
	}
	s := ""
	if formato != "" {
		s = fmt.Sprintf(formato, a...)
	}
	n.t.cambiar(n.id, func(p *Paso) {
		p.Estado = e
		p.Progreso = ""
		if s != "" {
			p.Titulo = s
		}
	})
}
func (n *Nodo) Bien(formato string, a ...any) { n.cerrar(EstadoBien, formato, a...) } // "" keeps the title
func (n *Nodo) Mal(formato string, a ...any)  { n.cerrar(EstadoMal, formato, a...) }
func (n *Nodo) Info(formato string, a ...any) { n.cerrar(EstadoInfo, formato, a...) }
func (n *Nodo) Progreso(formato string, a ...any) {
	if n == nil || n.id == 0 {
		return
	}
	n.t.mu.Lock()
	ult := n.t.ultimoPro[n.id]
	if time.Since(ult) < 250*time.Millisecond {
		n.t.mu.Unlock()
		return
	}
	n.t.ultimoPro[n.id] = time.Now()
	n.t.mu.Unlock()
	s := fmt.Sprintf(formato, a...)
	n.t.cambiar(n.id, func(p *Paso) { p.Progreso = s })
}
func (n *Nodo) Traza() *Traza {
	if n == nil {
		return nil
	}
	return n.t
}

// Preguntar emits a "pregunta" event and blocks until answered, ctx is done, or no responder exists.
// Without a responder it returns (p.PorDefecto, ErrSinRespuesta) immediately.
func (t *Traza) Preguntar(ctx context.Context, p PreguntaUsuario) (string, error) {
	if t == nil || t.responder == nil {
		return p.PorDefecto, ErrSinRespuesta
	}
	t.mu.Lock()
	t.enviar(Evento{Tipo: "pregunta", Pregunta: &p})
	t.mu.Unlock()
	return t.responder(ctx, p)
}

// Terminar emits "respuesta" (if r != nil) and then "fin".
func (t *Traza) Terminar(r *Respuesta, estado string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if r != nil {
		t.enviar(Evento{Tipo: "respuesta", Respuesta: r})
	}
	t.enviar(Evento{Tipo: "fin", Fin: &Fin{Estado: estado, Ms: t.ms()}})
}
