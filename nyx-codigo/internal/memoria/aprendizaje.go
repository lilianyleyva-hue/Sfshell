package memoria

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// IntervaloVolcado is the longest time counters stay only in memory (they are written at most this often,
// and at Cerrar).
var IntervaloVolcado = 5 * time.Second

type datosAprendizaje struct {
	Version    int               `json:"version"`
	Contadores map[string][2]int `json:"contadores"` // key → [éxitos, intentos]
}

type aprendizaje struct {
	mu        sync.Mutex
	m         map[string][2]int
	sucio     bool
	ultimo    time.Time   // last write
	reloj     *time.Timer // pending write, if any
	parado    bool        // Cerrar was called: no more timers
	escritura sync.Mutex  // serializes file writes
}

func (a *Almacen) cargarAprendizaje() {
	a.apr.m = map[string][2]int{}
	a.apr.ultimo = a.ahora()
	var d datosAprendizaje
	if !a.leerJSON(archivoAprendizaje, &d, &d.Version) {
		return
	}
	for k, v := range d.Contadores {
		if k == "" || v[0] < 0 || v[1] < 0 {
			continue
		}
		if v[0] > v[1] {
			v[0] = v[1]
		}
		a.apr.m[k] = v
	}
}

// contador is the nucleo.Contador view of the learning counters.
type contador struct{ a *Almacen }

var _ nucleo.Contador = contador{}

// Contador returns the learning statistics (nucleo.Contador). Keys are namespaced: "regla:", "mutacion:",
// "componente:", "asoc:<concepto>:<componente>", "estrategia:", "dominio:".
func (a *Almacen) Contador() nucleo.Contador { return contador{a} }

// Exito records one attempt and whether it worked.
func (c contador) Exito(clave string, ok bool) {
	if clave == "" {
		return
	}
	a := c.a
	a.apr.mu.Lock()
	defer a.apr.mu.Unlock()
	v := a.apr.m[clave]
	if ok {
		v[0]++
	}
	v[1]++
	a.apr.m[clave] = v
	a.apr.sucio = true
	a.programarVolcado()
}

// Tasa is the Beta mean (éxitos+1)/(intentos+2): 0.5 for an unknown key.
func (c contador) Tasa(clave string) float64 {
	a := c.a
	a.apr.mu.Lock()
	defer a.apr.mu.Unlock()
	v := a.apr.m[clave]
	return float64(v[0]+1) / float64(v[1]+2)
}

// Usos is the number of attempts recorded for clave.
func (c contador) Usos(clave string) int {
	a := c.a
	a.apr.mu.Lock()
	defer a.apr.mu.Unlock()
	return a.apr.m[clave][1]
}

// Asociaciones returns, for the counters "asoc:<concepto>:<componente>", componente → Tasa.
func (a *Almacen) Asociaciones(concepto string) map[string]float64 {
	prefijo := "asoc:" + concepto + ":"
	a.apr.mu.Lock()
	defer a.apr.mu.Unlock()
	out := map[string]float64{}
	for k, v := range a.apr.m {
		if strings.HasPrefix(k, prefijo) && len(k) > len(prefijo) {
			out[k[len(prefijo):]] = float64(v[0]+1) / float64(v[1]+2)
		}
	}
	return out
}

// programarVolcado arms one timer that writes the counters, no sooner than IntervaloVolcado after the last
// write. The caller holds a.apr.mu.
func (a *Almacen) programarVolcado() {
	if a.apr.reloj != nil || a.apr.parado {
		return
	}
	espera := IntervaloVolcado - a.ahora().Sub(a.apr.ultimo)
	if espera < 0 {
		espera = 0
	}
	a.vigilar.Add(1)
	a.apr.reloj = time.AfterFunc(espera, a.volcadoProgramado)
}

func (a *Almacen) volcadoProgramado() {
	defer a.vigilar.Done()
	a.apr.mu.Lock()
	a.apr.reloj = nil
	a.apr.mu.Unlock()
	if err := a.volcarContadores(); err != nil {
		a.avisar(fmt.Sprintf("No pude guardar las estadísticas de aprendizaje: %v.", err))
	}
}

// pararVolcado stops the timer for good (Cerrar). A callback already running finishes on its own.
func (a *Almacen) pararVolcado() {
	a.apr.mu.Lock()
	defer a.apr.mu.Unlock()
	a.apr.parado = true
	if a.apr.reloj != nil {
		if a.apr.reloj.Stop() {
			a.vigilar.Done() // its function will never run
		}
		a.apr.reloj = nil
	}
}

// volcarContadores writes aprendizaje.json if anything changed since the last write.
func (a *Almacen) volcarContadores() error {
	a.apr.escritura.Lock()
	defer a.apr.escritura.Unlock()
	a.apr.mu.Lock()
	if !a.apr.sucio {
		a.apr.mu.Unlock()
		return nil
	}
	copia := make(map[string][2]int, len(a.apr.m))
	for k, v := range a.apr.m {
		copia[k] = v
	}
	a.apr.sucio = false
	a.apr.ultimo = a.ahora()
	a.apr.mu.Unlock()
	err := escribirJSON(filepath.Join(a.dir, archivoAprendizaje), datosAprendizaje{Version: Version, Contadores: copia})
	if err != nil {
		a.apr.mu.Lock()
		a.apr.sucio = true
		a.apr.mu.Unlock()
	}
	return err
}
