// Package cerebro is the orchestrator of Nyx Código (§4.14): sessions and follow-ups, routing and
// interpretation ranking, budgets, the pipelines for every intent, the evidence and wording policy,
// and the learning hooks. It is the only package that imports the leaves. *Cerebro implements the
// servidor.Motor contract (§4.15) with exactly the same method set, plus Pendientes().
package cerebro

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/logica"
	"nyxcodigo/internal/mates"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/puzles"
	"nyxcodigo/internal/saber"
	"nyxcodigo/internal/sintesis"
)

// Version is the application version shown in /api/estado.
const Version = "1.0"

// Config holds the orchestrator's settings.
type Config struct {
	DirDatos       string
	SinInternet    bool
	Presupuestos   map[nucleo.TipoIntencion]time.Duration // overrides of the defaults in presupuesto.go
	MaxPresupuesto time.Duration                          // 120 s: the ceiling for "Pensar más"
	Semilla        int64                                  // 0 = time-based (shown in the evidence)
}

// Dependencias are the leaves with state that cerebro needs.
type Dependencias struct {
	Ej    nucleo.Ejecutor  // *arenero.Arenero (or nucleotest.EjecutorFalso in tests); nil = no sandbox
	Mem   *memoria.Almacen // nil: opened at Config.DirDatos
	Web   *saber.Web       // nil = no internet
	Std   *saber.IndiceStd // may be nil until indexed (see PonerIndice)
	GoBin string
}

// Cerebro answers requests. It is safe for concurrent use.
type Cerebro struct {
	cfg         Config
	ej          nucleo.Ejecutor
	mem         *memoria.Almacen
	web         *saber.Web
	std         atomic.Pointer[saber.IndiceStd]
	goBin       string
	stats       nucleo.Contador
	biblio      *sintesis.Biblioteca
	resolutores []nucleo.Resolutor
	pipes       pipelines
	ahora       func() time.Time
	espera      time.Duration // how long a question waits for an answer (5 min)

	mu             sync.Mutex
	sesiones       map[string]*sesion
	ultimaLimpieza time.Time

	internet   atomic.Bool
	mirandoRed atomic.Bool
	numTareas  atomic.Int64
}

// Nuevo builds the orchestrator. Without Dependencias.Mem it opens the memory at cfg.DirDatos.
func Nuevo(cfg Config, d Dependencias) (*Cerebro, error) {
	if d.Mem == nil {
		if strings.TrimSpace(cfg.DirDatos) == "" {
			return nil, errors.New("cerebro: falta la memoria (Dependencias.Mem) y la carpeta de datos")
		}
		m, err := memoria.Abrir(cfg.DirDatos)
		if err != nil {
			return nil, fmt.Errorf("cerebro: %w", err)
		}
		d.Mem = m
	}
	if cfg.MaxPresupuesto <= 0 {
		cfg.MaxPresupuesto = 120 * time.Second
	}
	c := &Cerebro{
		cfg:      cfg,
		ej:       d.Ej,
		mem:      d.Mem,
		web:      d.Web,
		goBin:    d.GoBin,
		stats:    d.Mem.Contador(),
		ahora:    time.Now,
		espera:   5 * time.Minute,
		sesiones: map[string]*sesion{},
	}
	if cfg.SinInternet {
		c.web = nil
	}
	if d.Std != nil {
		c.std.Store(d.Std)
	}
	c.biblio = sintesis.NuevaBiblioteca(sintesis.Base())
	c.biblio.Cargar(d.Mem.Funciones()) // functions without a DSL program are skipped on purpose
	c.resolutores = resolutoresPorDefecto(d.Mem)
	c.pipes = pipelinesPorDefecto()
	return c, nil
}

// resolutoresPorDefecto are the reasoning leaves; memoria is their fact base, fact store and verb store.
func resolutoresPorDefecto(m *memoria.Almacen) []nucleo.Resolutor {
	var rs []nucleo.Resolutor
	rs = append(rs, mates.Resolutores(m)...)
	rs = append(rs, logica.Resolutores(m, m, lengua.Lematizar)...)
	rs = append(rs, puzles.Resolutores(lengua.Lematizar)...)
	return rs
}

// PonerIndice installs the GOROOT example index once it has been built in the background.
func (c *Cerebro) PonerIndice(i *saber.IndiceStd) { c.std.Store(i) }

// Responder runs one request to completion, emitting the trace. It never panics: a panic becomes an
// "error interno" answer. It is the servidor.Motor implementation (§4.15).
func (c *Cerebro) Responder(ctx context.Context, p nucleo.Peticion, t *nucleo.Traza) (r nucleo.Respuesta) {
	if ctx == nil {
		ctx = context.Background()
	}
	terminada := false
	defer func() {
		if x := recover(); x != nil {
			r = respuestaErrorInterno(p, x, debug.Stack())
			if !terminada {
				t.Terminar(&r, "error")
			}
		}
	}()
	r = c.responder(ctx, p, t)
	estado := "terminada"
	if errors.Is(ctx.Err(), context.Canceled) {
		estado = "cancelada"
	}
	terminada = true
	t.Terminar(&r, estado)
	return r
}

// respuestaErrorInterno is the apology shown when something panicked inside cerebro or a leaf.
func respuestaErrorInterno(p nucleo.Peticion, x any, pila []byte) nucleo.Respuesta {
	_ = pila // kept for debugging sessions; never shown to the user
	return nucleo.Respuesta{
		Tarea:          p.Tarea,
		Texto:          "Lo siento: algo falló dentro de Nyx Código (error interno) y no pude terminar.",
		Parrafos:       []string{fmt.Sprintf("Error interno: %v", x), "Puedes intentarlo otra vez o decirlo de otra forma."},
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelFallo,
		Metodo:         "error",
	}
}

// responder is Responder without the panic guard and without Terminar.
func (c *Cerebro) responder(ctx context.Context, p nucleo.Peticion, t *nucleo.Traza) nucleo.Respuesta {
	s := c.sesion(p.Sesion)
	if r, ok := c.rutearRespuestaPendiente(s, p, t); ok {
		r.Tarea = p.Tarea
		return r
	}
	k := c.nuevaTarea(ctx, p, s, t)
	defer k.cerrar()
	r := c.rutear(k)
	c.redactar(k, &r)
	c.aprender(k, &r)
	s.registrar(k, r)
	return r
}

// Opinar records the user's verdict on a task ("¿Está bien? Sí/No"): it goes to the history, moves the
// strategy counter of that turn, raises the use count of its function and confirms its lexicon proposals
// (Sí), or stores the example as a counterexample (No, with "con [2] debe dar 0").
func (c *Cerebro) Opinar(o nucleo.Opinion) error {
	err := c.mem.Opinar(o)
	turno, ok := c.buscarTurno(o.Sesion, o.Tarea)
	if !ok {
		return err
	}
	if clave := estrategiaDe(turno); clave != "" {
		c.stats.Exito("estrategia:"+clave, o.Correcto)
	}
	if o.Correcto {
		if turno.Funcion != nil {
			c.mem.Usada(turno.Funcion.Nombre)
		}
		for lema, cs := range turno.Propuestas {
			for _, cpt := range cs {
				c.mem.ProponerPalabra(lema, cpt)
			}
		}
		return err
	}
	if turno.Funcion != nil && turno.Funcion.DSL != "" {
		c.contarDSL(turno.Funcion.DSL, turno.Funcion.Firma, turno.Funcion.Conceptos, false)
	}
	if strings.TrimSpace(o.Ejemplo) != "" {
		if f, conceptos, ok := firmaDeTurno(turno); ok {
			if caso, _, e := leerEjemplo(o.Ejemplo, f); e == nil && caso != nil {
				if e := c.mem.GuardarContraejemplo(memoria.ClaveContraejemplo(conceptos, f), f, *caso); e != nil && err == nil {
					err = e
				}
			}
		}
	}
	return err
}

// Funciones lists the learned functions; q filters them (name, description, words or concepts).
func (c *Cerebro) Funciones(q string) []nucleo.FuncionAprendida {
	todas := c.mem.Funciones()
	q = strings.TrimSpace(q)
	if q == "" {
		return todas
	}
	nq := nucleo.Normalizar(q)
	puesto := map[string]bool{}
	var out []nucleo.FuncionAprendida
	for _, f := range todas {
		texto := nucleo.Normalizar(f.Nombre + " " + f.Descripcion + " " + strings.Join(f.Palabras, " ") + " " + strings.Join(f.Conceptos, " "))
		if strings.Contains(texto, nq) {
			puesto[f.Nombre] = true
			out = append(out, f)
		}
	}
	var lemas, conceptos []string
	lex := c.lexico()
	for _, tk := range lengua.Lematizar(q) {
		if tk.Clase != "palabra" {
			continue
		}
		lemas = append(lemas, tk.Lema)
		if cpt, ok := lex.Concepto(tk.Lema); ok {
			conceptos = append(conceptos, cpt)
		}
	}
	for _, p := range c.mem.BuscarFunciones(conceptos, lemas, "", 0) {
		if p.Puntos > 0 && !puesto[p.F.Nombre] {
			puesto[p.F.Nombre] = true
			out = append(out, p.F)
		}
	}
	return out
}

// Hechos lists the stored facts; q ranks and filters them.
func (c *Cerebro) Hechos(q string) []nucleo.Hecho {
	if strings.TrimSpace(q) == "" {
		return c.mem.Todos()
	}
	return c.mem.BuscarHechos(q, 0)
}

// Palabras returns the confirmed learned words (word → concept).
func (c *Cerebro) Palabras() map[string]string { return c.mem.Palabras() }

// Olvidar forgets a function, a fact (or rule), or a learned word. tipo: "funcion" | "hecho" | "regla" | "palabra".
func (c *Cerebro) Olvidar(tipo, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("cerebro: falta qué olvidar")
	}
	switch tipo {
	case "funcion", "función":
		if err := c.mem.OlvidarFuncion(id); err != nil {
			return err
		}
		c.biblio.Olvidar(id)
		return nil
	case "hecho":
		if err := c.mem.OlvidarHecho(id); err != nil {
			if strings.HasPrefix(id, "r") {
				return c.mem.OlvidarRegla(id)
			}
			return err
		}
		return nil
	case "regla":
		return c.mem.OlvidarRegla(id)
	case "palabra":
		return c.mem.OlvidarPalabra(id)
	}
	return fmt.Errorf("cerebro: no sé olvidar cosas de tipo %q (funcion, hecho o palabra)", tipo)
}

// Riesgos lists what the code would need permission for (the run-confirmation dialog). Empty: it may run
// automatically under PermisoAuto.
func (c *Cerebro) Riesgos(codigo string) []nucleo.Violacion {
	if c.ej == nil || strings.TrimSpace(codigo) == "" {
		return nil
	}
	return c.ej.Revisar(codigo, nucleo.PermisoAuto)
}

// Estado reports the app status for the header chips.
func (c *Cerebro) Estado() nucleo.EstadoApp {
	e := nucleo.EstadoApp{Version: Version}
	if c.ej != nil {
		e.Arenero = c.ej.Estado()
		if a, ok := c.ej.(interface{ Aviso() string }); ok {
			if s := a.Aviso(); s != "" {
				e.Avisos = append(e.Avisos, s)
			}
		}
	} else {
		e.Arenero = nucleo.EstadoArenero{Nivel: "sin Go"}
		e.Avisos = append(e.Avisos, "No tengo caja segura: no puedo compilar ni ejecutar código.")
	}
	e.Internet = c.hayInternet()
	e.Funciones, e.Hechos, e.Palabras = c.mem.Contar()
	e.Avisos = append(e.Avisos, c.mem.Avisos()...)
	return e
}

// Pendientes returns the questions the app could not answer (servidor finds this method by reflection).
func (c *Cerebro) Pendientes() []memoria.Pendiente { return c.mem.Pendientes() }

// hayInternet returns the last known connectivity and refreshes it in the background (never blocks).
func (c *Cerebro) hayInternet() bool {
	if c.web == nil {
		return false
	}
	if c.mirandoRed.CompareAndSwap(false, true) {
		go func() {
			defer c.mirandoRed.Store(false)
			defer func() { _ = recover() }()
			ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelar()
			c.internet.Store(c.web.Disponible(ctx))
		}()
	}
	return c.internet.Load()
}

// enLinea reports whether the web may be used now (online and not disabled).
func (c *Cerebro) enLinea(ctx context.Context) bool {
	if c.web == nil {
		return false
	}
	ok := c.web.Disponible(ctx)
	c.internet.Store(ok)
	return ok
}

// lexico is the base lexicon plus the confirmed learned words.
func (c *Cerebro) lexico() *lengua.Lexico {
	return lengua.LexicoBase().ConAprendidas(c.mem.Palabras())
}

// resolutor finds a reasoning resolver by name.
func (c *Cerebro) resolutor(nombre string) nucleo.Resolutor {
	for _, r := range c.resolutores {
		if r.Nombre() == nombre {
			return r
		}
	}
	return nil
}

// buscarTurno finds a task's turn, first in the given session, then in all of them.
func (c *Cerebro) buscarTurno(sesionID, tareaID string) (Turno, bool) {
	if tareaID == "" {
		return Turno{}, false
	}
	c.mu.Lock()
	s := c.sesiones[sesionID]
	var todas []*sesion
	for _, x := range c.sesiones {
		todas = append(todas, x)
	}
	c.mu.Unlock()
	if s != nil {
		if t, ok := s.buscarTurno(tareaID); ok {
			return t, true
		}
	}
	sort.Slice(todas, func(i, j int) bool { return todas[i].id < todas[j].id })
	for _, x := range todas {
		if t, ok := x.buscarTurno(tareaID); ok {
			return t, true
		}
	}
	return Turno{}, false
}

// estrategiaDe is the strategy counter name of a turn ("mates.ecuacion", "crear_funcion").
func estrategiaDe(t Turno) string {
	if t.Resolutor != "" {
		return t.Resolutor
	}
	return string(t.Intencion)
}

// firmaDeTurno returns the signature and concepts a turn worked with.
func firmaDeTurno(t Turno) (nucleo.Firma, []string, bool) {
	if t.Funcion != nil && t.Funcion.Firma.Nombre != "" {
		return t.Funcion.Firma, t.Funcion.Conceptos, true
	}
	if t.Pregunta != nil && t.Pregunta.Firma != nil {
		return *t.Pregunta.Firma, t.Pregunta.Conceptos, true
	}
	return nucleo.Firma{}, nil, false
}

// seguro runs f and turns a panic into an error (for goroutines, which Responder's recover cannot see).
func seguro(f func() error) (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("error interno: %v", x)
		}
	}()
	return f()
}
