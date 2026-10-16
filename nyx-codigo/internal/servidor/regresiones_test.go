package servidor

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Regression: the hard limit used to start when the task was created, so a queued task lost its
// queue time and could be reported as "cancelada" ("Lo he parado antes de empezar.") without the user
// stopping it.
func TestLimiteEmpiezaAlTenerTurno(t *testing.T) {
	const limite = 300 * time.Millisecond
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4)}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1; c.Limite = limite })
	defer close(m.soltar)
	a := e.preguntar("primera")
	<-m.entrar
	b := e.preguntar("segunda") // queued while the first one uses its whole budget
	evA, _, _ := e.flujo(a, nil, "", nil)
	evB, _, _ := e.flujo(b, nil, "", nil)
	var finA, finB nucleo.Fin
	json.Unmarshal([]byte(evA[len(evA)-1].datos), &finA)
	json.Unmarshal([]byte(evB[len(evB)-1].datos), &finB)
	if finA.Estado != "terminada" || finB.Estado != "terminada" {
		t.Fatalf("fin a = %+v, fin b = %+v", finA, finB)
	}
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evB[len(evB)-2].datos), &r)
	if strings.Contains(r.Texto, "antes de empezar") {
		t.Errorf("la segunda se dio por cancelada: %+v", r)
	}
	m.mu.Lock()
	plazos := append([]time.Duration(nil), m.plazos...)
	m.mu.Unlock()
	if len(plazos) != 2 {
		t.Fatalf("plazos = %v", plazos)
	}
	// the second task waited about 300 ms in the queue and still starts with (almost) all of it
	if plazos[1] < limite*2/3 {
		t.Errorf("la segunda empezó con %v de %v", plazos[1], limite)
	}
}

// A task that cannot get a slot within its limit ends with an honest error, not "cancelada".
func TestColaDemasiadoLarga(t *testing.T) {
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4), ignorarCtx: true}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1; c.Limite = 100 * time.Millisecond })
	defer close(m.soltar) // the stuck Motor returns only at the end
	e.preguntar("atascada")
	<-m.entrar
	b := e.preguntar("en cola")
	evB, _, _ := e.flujo(b, nil, "", nil)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evB[len(evB)-1].datos), &fin)
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evB[len(evB)-2].datos), &r)
	if fin.Estado != "error" || !strings.Contains(r.Texto, "demasiadas tareas") || r.Nivel != nucleo.NivelFallo {
		t.Errorf("fin = %+v respuesta = %+v", fin, r)
	}
}

// Cancelling a queued task still says it was stopped before starting.
func TestCancelarEnCola(t *testing.T) {
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4)}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1 })
	defer close(m.soltar)
	e.preguntar("primera")
	<-m.entrar
	b := e.preguntar("segunda")
	time.Sleep(20 * time.Millisecond)
	e.json("POST", "/api/tareas/"+b+"/cancelar", "", 204, nil)
	evB, _, _ := e.flujo(b, nil, "", nil)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evB[len(evB)-1].datos), &fin)
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evB[len(evB)-2].datos), &r)
	if fin.Estado != "cancelada" || r.Texto != "Lo he parado antes de empezar." {
		t.Errorf("fin = %+v respuesta = %+v", fin, r)
	}
}
