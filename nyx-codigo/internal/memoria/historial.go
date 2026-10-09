package memoria

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// MaxPendientes is the number of pending questions kept (the oldest are dropped).
const MaxPendientes = 300

type historial struct {
	mu sync.Mutex
}

// opinionGuardada is the line appended to the history for a "bien"/"mal" verdict.
type opinionGuardada struct {
	Tarea      string    `json:"tarea"`
	Sesion     string    `json:"sesion,omitempty"`
	Opinion    string    `json:"opinion"` // "bien" | "mal"
	Comentario string    `json:"comentario,omitempty"`
	Ejemplo    string    `json:"ejemplo,omitempty"`
	Fecha      time.Time `json:"fecha"`
}

func (a *Almacen) rutaHistorial(dia time.Time) string {
	return filepath.Join(a.dir, dirHistorial, dia.Local().Format("2006-01-02")+".jsonl")
}

func (a *Almacen) anexarHistorial(dia time.Time, r any) error {
	a.hist.mu.Lock()
	defer a.hist.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(a.dir, dirHistorial), 0o700); err != nil {
		return err
	}
	return anexarJSONL(a.rutaHistorial(dia), r)
}

// Registrar appends one request to historial/AAAA-MM-DD.jsonl (the day of e.Fecha; now when it is zero).
func (a *Almacen) Registrar(e nucleo.Episodio) error {
	if e.Fecha.IsZero() {
		e.Fecha = a.ahora()
	}
	if err := a.anexarHistorial(e.Fecha, e); err != nil {
		return fmt.Errorf("memoria: no pude guardar el historial: %w", err)
	}
	return nil
}

// Opinar appends the user's verdict on a task ({"tarea":…,"opinion":"bien"|"mal"}) to today's history.
func (a *Almacen) Opinar(o nucleo.Opinion) error {
	if strings.TrimSpace(o.Tarea) == "" {
		return errors.New("memoria: la opinión no dice de qué tarea es")
	}
	r := opinionGuardada{Tarea: o.Tarea, Sesion: o.Sesion, Opinion: "mal", Comentario: o.Comentario, Ejemplo: o.Ejemplo, Fecha: a.ahora()}
	if o.Correcto {
		r.Opinion = "bien"
	}
	if err := a.anexarHistorial(r.Fecha, r); err != nil {
		return fmt.Errorf("memoria: no pude guardar la opinión: %w", err)
	}
	return nil
}

// Historial returns the requests of one day, oldest first, with the user's verdicts ("bien"/"mal") filled in.
// Damaged lines are skipped.
func (a *Almacen) Historial(dia time.Time) ([]nucleo.Episodio, error) {
	a.hist.mu.Lock()
	l, err := leerJSONL(a.rutaHistorial(dia))
	a.hist.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("memoria: no pude leer el historial: %w", err)
	}
	var out []nucleo.Episodio
	opiniones := map[string]string{}
	for _, linea := range l.lineas {
		var sonda struct {
			Exito   *bool  `json:"exito"`
			Tarea   string `json:"tarea"`
			Opinion string `json:"opinion"`
		}
		if json.Unmarshal(linea, &sonda) != nil {
			continue
		}
		if sonda.Exito == nil {
			if sonda.Tarea != "" && sonda.Opinion != "" {
				opiniones[sonda.Tarea] = sonda.Opinion
			}
			continue
		}
		var e nucleo.Episodio
		if json.Unmarshal(linea, &e) == nil {
			out = append(out, e)
		}
	}
	for i := range out {
		if op, ok := opiniones[out[i].Tarea]; ok {
			out[i].Opinion = op
		}
	}
	return out, nil
}

// ---- pending questions ----

// Pendiente is a question the app could not answer, kept for study mode.
type Pendiente struct {
	ID       string    `json:"id"`
	Texto    string    `json:"texto"`
	Codigo   string    `json:"codigo,omitempty"`
	Motivo   string    `json:"motivo,omitempty"`
	Intentos int       `json:"intentos"`
	Fecha    time.Time `json:"fecha"`
}

// Pendientes returns the pending questions, oldest first.
func (a *Almacen) Pendientes() []Pendiente {
	a.pend.mu.RLock()
	defer a.pend.mu.RUnlock()
	return a.pend.todos()
}

// AgregarPendiente saves a question that could not be answered. With the ID of a stored one, it replaces it.
// Without an ID, the same question (same normalized text and code) is not repeated: its Intentos goes up and
// its Motivo is updated. New questions get an id ("p1", "p2"…), Fecha = now and at least 1 attempt.
func (a *Almacen) AgregarPendiente(p Pendiente) error {
	if strings.TrimSpace(p.Texto) == "" && strings.TrimSpace(p.Codigo) == "" {
		return errors.New("memoria: la pregunta pendiente está vacía")
	}
	b := &a.pend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	if p.ID != "" {
		if viejo, ok := b.reg[p.ID]; ok {
			if p.Fecha.IsZero() {
				p.Fecha = viejo.Fecha
			}
			if err := b.poner(a.dir, p); err != nil {
				return fmt.Errorf("memoria: no pude guardar la pregunta pendiente: %w", err)
			}
			return nil
		}
	}
	clave := nucleo.Normalizar(p.Texto) + "\x1f" + strings.TrimSpace(p.Codigo)
	if p.ID == "" {
		for _, id := range b.orden {
			v := b.reg[id]
			if nucleo.Normalizar(v.Texto)+"\x1f"+strings.TrimSpace(v.Codigo) != clave {
				continue
			}
			v.Intentos++
			if p.Intentos > v.Intentos {
				v.Intentos = p.Intentos
			}
			if p.Motivo != "" {
				v.Motivo = p.Motivo
			}
			if err := b.poner(a.dir, v); err != nil {
				return fmt.Errorf("memoria: no pude guardar la pregunta pendiente: %w", err)
			}
			return nil
		}
		p.ID = b.nuevoID()
	}
	if p.Fecha.IsZero() {
		p.Fecha = a.ahora()
	}
	if p.Intentos < 1 {
		p.Intentos = 1
	}
	if err := b.poner(a.dir, p); err != nil {
		return fmt.Errorf("memoria: no pude guardar la pregunta pendiente: %w", err)
	}
	for len(b.orden) > MaxPendientes {
		if err := b.borrar(a.dir, b.orden[0]); err != nil {
			break
		}
	}
	return nil
}

// ResolverPendiente removes a pending question (it was answered, or the user dismissed it).
func (a *Almacen) ResolverPendiente(id string) error {
	b := &a.pend
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	if _, ok := b.reg[id]; !ok {
		return fmt.Errorf("memoria: no hay ninguna pregunta pendiente %s", id)
	}
	if err := b.borrar(a.dir, id); err != nil {
		return fmt.Errorf("memoria: no pude quitar la pregunta pendiente: %w", err)
	}
	return nil
}
