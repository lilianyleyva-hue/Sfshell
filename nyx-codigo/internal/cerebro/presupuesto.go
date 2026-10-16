package cerebro

import (
	"context"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Default budgets per intent (§4.14.1). Reasoning intents share presupuestoRazonar.
var presupuestosBase = map[nucleo.TipoIntencion]time.Duration{
	nucleo.ICrearFuncion:  30 * time.Second,
	nucleo.ICrearPrograma: 30 * time.Second,
	nucleo.IReparar:       30 * time.Second,
	nucleo.IExplicar:      20 * time.Second,
	nucleo.IProbar:        20 * time.Second,
	nucleo.IOptimizar:     20 * time.Second,
	nucleo.IBuscar:        20 * time.Second,
	nucleo.IDocGo:         20 * time.Second,
	nucleo.IEjecutar:      15 * time.Second,
	nucleo.IEstudiar:      10 * time.Minute,
	nucleo.IMemoria:       10 * time.Second,
	nucleo.ICharla:        10 * time.Second,
}

const presupuestoRazonar = 10 * time.Second

// presupuestoDe is the configured budget of an intent (Config.Presupuestos first, then the defaults).
func (c *Cerebro) presupuestoDe(i nucleo.TipoIntencion) time.Duration {
	if d, ok := c.cfg.Presupuestos[i]; ok && d > 0 {
		return d
	}
	if d, ok := presupuestosBase[i]; ok {
		return d
	}
	return presupuestoRazonar
}

// presupuestoTarea is the budget for this task: the request's own (Peticion.Presupuesto), else the
// intent's; "Pensar más" doubles the previous run's budget (or the default) up to MaxPresupuesto.
// A budget that is larger than MaxPresupuesto by default (estudiar) is never cut.
func (c *Cerebro) presupuestoTarea(k *tarea, i nucleo.TipoIntencion) time.Duration {
	if k.p.Presupuesto > 0 {
		return k.p.Presupuesto
	}
	base := c.presupuestoDe(i)
	if !k.pensarMas {
		return base
	}
	d := 2 * base
	if k.presupuestoPrevio > 0 {
		d = 2 * k.presupuestoPrevio
	}
	tope := c.cfg.MaxPresupuesto
	if base > tope {
		tope = base
	}
	if d > tope {
		d = tope
	}
	return d
}

// reloj is a context whose deadline can be paused: the budget does not run while the user is
// thinking about a question. When the budget is spent, Err() is context.DeadlineExceeded; when the
// request is cancelled, it is the parent's error.
type reloj struct {
	padre    context.Context
	mu       sync.Mutex
	hecho    chan struct{}
	err      error
	timer    *time.Timer
	limite   time.Time     // while running
	restante time.Duration // while paused
	pausas   int
}

var _ context.Context = (*reloj)(nil)

func nuevoReloj(padre context.Context, d time.Duration) *reloj {
	r := &reloj{padre: padre, hecho: make(chan struct{}), restante: d}
	r.mu.Lock()
	r.arrancar()
	r.mu.Unlock()
	go func() {
		select {
		case <-padre.Done():
			r.cerrar(padre.Err())
		case <-r.hecho:
		}
	}()
	return r
}

// arrancar starts the timer with the remaining time; caller holds mu.
func (r *reloj) arrancar() {
	if r.err != nil {
		return
	}
	r.limite = time.Now().Add(r.restante)
	r.timer = time.AfterFunc(r.restante, func() { r.cerrar(context.DeadlineExceeded) })
}

func (r *reloj) cerrar(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return
	}
	r.err = err
	if r.timer != nil {
		r.timer.Stop()
	}
	close(r.hecho)
}

// Pausar stops the budget clock until the returned function is called (pauses nest).
func (r *reloj) Pausar() (reanudar func()) {
	r.mu.Lock()
	if r.pausas == 0 && r.err == nil {
		if r.timer != nil {
			r.timer.Stop()
		}
		r.restante = time.Until(r.limite)
		if r.restante < 0 {
			r.restante = 0
		}
	}
	r.pausas++
	r.mu.Unlock()
	var una sync.Once
	return func() {
		una.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.pausas--
			if r.pausas == 0 {
				r.arrancar()
			}
		})
	}
}

// Restante is the budget left.
func (r *reloj) Restante() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0
	}
	if r.pausas > 0 {
		return r.restante
	}
	if d := time.Until(r.limite); d > 0 {
		return d
	}
	return 0
}

// Cancelar ends the clock (the task is over).
func (r *reloj) Cancelar() { r.cerrar(context.Canceled) }

func (r *reloj) Deadline() (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var d time.Time
	if r.pausas > 0 {
		d = time.Now().Add(r.restante)
	} else {
		d = r.limite
	}
	if pd, ok := r.padre.Deadline(); ok && pd.Before(d) {
		d = pd
	}
	return d, true
}

func (r *reloj) Done() <-chan struct{} { return r.hecho }

func (r *reloj) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *reloj) Value(k any) any { return r.padre.Value(k) }
