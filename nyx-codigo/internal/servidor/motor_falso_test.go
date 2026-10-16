package servidor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// motorFalso replays a recorded trace (testdata/traza-sumapares.json) through the real nucleo.Traza
// API, so the server and the UI can be exercised without cerebro.

type operacion struct {
	Op       string                  `json:"op"`
	ID       int                     `json:"id"`
	Padre    int                     `json:"padre"`
	Tipo     nucleo.TipoPaso         `json:"tipo"`
	Titulo   string                  `json:"titulo"`
	Texto    string                  `json:"texto"`
	Codigo   string                  `json:"codigo"`
	Tabla    *nucleo.Tabla           `json:"tabla"`
	Pregunta *nucleo.PreguntaUsuario `json:"pregunta"`
}

type trazaGrabada struct {
	Texto     string           `json:"texto"`
	Pasos     []operacion      `json:"pasos"`
	Respuesta nucleo.Respuesta `json:"respuesta"`
}

func cargarTraza(ruta string) (trazaGrabada, error) {
	var g trazaGrabada
	b, err := os.ReadFile(ruta)
	if err != nil {
		return g, err
	}
	return g, json.Unmarshal(b, &g)
}

type motorFalso struct {
	grabada trazaGrabada
	otras   map[string]trazaGrabada // keyword in the request text → another recorded trace (demo)
	demora  time.Duration           // between steps (demo only)

	// behaviours for tests
	terminaSolo  bool // the Motor calls Terminar itself (as cerebro does)
	esperarCtx   bool // block until ctx is done before answering
	entrar       chan struct{}
	soltar       chan struct{}
	panico       bool
	sinPreguntar bool
	ignorarCtx   bool // wait on soltar even when ctx ends (a Motor that does not notice)

	mu         sync.Mutex
	peticiones []nucleo.Peticion
	respuestas []string        // answers received through Preguntar
	errCtx     error           // ctx.Err() seen when a wait ended
	plazos     []time.Duration // time left until ctx's deadline when Responder started
	opiniones  []nucleo.Opinion
	olvidos    []string
	riesgos    []nucleo.Violacion
	funciones  []nucleo.FuncionAprendida
	pendientes []pendienteFalso
}

// pendienteFalso mimics memoria.Pendiente (no JSON tags: exported Go names).
type pendienteFalso struct {
	ID, Texto, Codigo, Motivo string
	Intentos                  int
	Fecha                     time.Time
}

func (m *motorFalso) Responder(ctx context.Context, p nucleo.Peticion, t *nucleo.Traza) nucleo.Respuesta {
	m.mu.Lock()
	m.peticiones = append(m.peticiones, p)
	if d, ok := ctx.Deadline(); ok {
		m.plazos = append(m.plazos, time.Until(d))
	}
	m.mu.Unlock()
	if m.entrar != nil {
		m.entrar <- struct{}{}
	}
	if m.panico {
		panic("se rompió algo")
	}
	if m.soltar != nil && m.ignorarCtx {
		<-m.soltar
	} else if m.soltar != nil {
		select {
		case <-m.soltar:
		case <-ctx.Done():
		}
	}
	if m.esperarCtx {
		n := t.Raiz().Sub(nucleo.PasoIntento, "Pienso sin parar")
		<-ctx.Done()
		m.mu.Lock()
		m.errCtx = ctx.Err()
		m.mu.Unlock()
		n.Mal("Me pararon")
		return nucleo.Respuesta{Texto: "Lo mejor que tenía: nada", Metodo: "falso"}
	}
	if p.Accion == "estudiar" {
		return m.estudiar(ctx, t)
	}
	grabada := m.grabada
	for clave, g := range m.otras {
		if strings.Contains(p.Texto, clave) {
			grabada = g
		}
	}
	nodos := map[int]*nucleo.Nodo{0: t.Raiz()}
	var respuestaPregunta string
	for _, op := range grabada.Pasos {
		if m.demora > 0 {
			select {
			case <-time.After(m.demora):
			case <-ctx.Done():
			}
		}
		if ctx.Err() != nil {
			break
		}
		n := nodos[op.ID]
		switch op.Op {
		case "sub":
			nodos[op.ID] = nodos[op.Padre].Sub(op.Tipo, "%s", op.Titulo)
		case "nota":
			nodos[op.Padre].Nota("%s", op.Titulo)
		case "bien":
			n.Bien("%s", op.Titulo)
		case "mal":
			n.Mal("%s", op.Titulo)
		case "info":
			n.Info("%s", op.Titulo)
		case "progreso":
			n.Progreso("%s", op.Texto)
		case "detalle":
			n.Detalle("%s", op.Texto)
		case "codigo":
			n.Codigo(op.Codigo)
		case "tabla":
			n.Tabla(*op.Tabla)
		case "pregunta":
			if m.sinPreguntar {
				continue
			}
			r, err := t.Preguntar(ctx, *op.Pregunta)
			m.mu.Lock()
			if err == nil {
				m.respuestas = append(m.respuestas, r)
			} else if ctx.Err() != nil {
				m.errCtx = ctx.Err()
			}
			m.mu.Unlock()
			respuestaPregunta = r
		}
	}
	r := grabada.Respuesta
	r.Tarea = p.Tarea
	if respuestaPregunta != "" {
		r.Parrafos = append(append([]string(nil), r.Parrafos...), fmt.Sprintf("Con [-2] da %s, como me dijiste.", respuestaPregunta))
	}
	if ctx.Err() != nil {
		m.mu.Lock()
		m.errCtx = ctx.Err()
		m.mu.Unlock()
	}
	if m.terminaSolo {
		t.Terminar(&r, "terminada")
	}
	return r
}

// estudiar replays a short study session with N/M progress.
func (m *motorFalso) estudiar(ctx context.Context, t *nucleo.Traza) nucleo.Respuesta {
	n := t.Raiz().Sub(nucleo.PasoBusqueda, "Leo gobyexample")
	const total = 8
	for i := 1; i <= total && ctx.Err() == nil; i++ {
		n.Progreso("%d/%d", i, total)
		if m.demora > 0 {
			time.Sleep(3 * m.demora)
		}
	}
	n.Bien("Leí %d páginas de gobyexample", total)
	t.Raiz().Sub(nucleo.PasoResultado, "Aprendí a resolver 1 pregunta que antes no sabía").Bien("")
	return nucleo.Respuesta{Texto: "Aprendí 1 cosa nueva: ordenar palabras por longitud.", Metodo: "estudiar", Exito: true,
		Nivel: nucleo.NivelEntendido}
}

func (m *motorFalso) Opinar(o nucleo.Opinion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o.Tarea == "" {
		return errors.New("falta la tarea")
	}
	m.opiniones = append(m.opiniones, o)
	return nil
}

func (m *motorFalso) Funciones(q string) []nucleo.FuncionAprendida {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []nucleo.FuncionAprendida
	for _, f := range m.funciones {
		if q == "" || strings.Contains(strings.ToLower(f.Nombre+" "+f.Descripcion), strings.ToLower(q)) {
			out = append(out, f)
		}
	}
	return out
}

func (m *motorFalso) Hechos(q string) []nucleo.Hecho {
	if q == "nada" {
		return nil
	}
	return []nucleo.Hecho{{ID: "h1", Sujeto: "toby", Relacion: "es_un", Objeto: "perro", Texto: "Toby es un perro",
		Fuente: "usuario", Confianza: 1, Fecha: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)}}
}

func (m *motorFalso) Palabras() map[string]string {
	return map[string]string{"capicua": "palindromo"}
}

func (m *motorFalso) Olvidar(tipo, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == "NoExiste" {
		return errors.New("no conozco «NoExiste»")
	}
	m.olvidos = append(m.olvidos, tipo+":"+id)
	return nil
}

func (m *motorFalso) Riesgos(codigo string) []nucleo.Violacion {
	if strings.Contains(codigo, "os/exec") {
		return []nucleo.Violacion{{Linea: 3, Paquete: "os/exec", Que: "usa el paquete os/exec: puede ejecutar otros programas"}}
	}
	if strings.Contains(codigo, `import "C"`) {
		return []nucleo.Violacion{{Linea: 2, Paquete: "C", Que: "usa cgo", Grave: true}}
	}
	return nil
}

func (m *motorFalso) Estado() nucleo.EstadoApp {
	m.mu.Lock()
	n := len(m.funciones)
	m.mu.Unlock()
	return nucleo.EstadoApp{Version: "prueba", Internet: true, Funciones: n, Hechos: 120, Palabras: 1,
		Arenero: nucleo.EstadoArenero{GoVersion: "go1.24.7", GoOK: true, RedAislada: true, Limites: true, Nivel: "completa"}}
}

func (m *motorFalso) Pendientes() []pendienteFalso {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pendientes
}
