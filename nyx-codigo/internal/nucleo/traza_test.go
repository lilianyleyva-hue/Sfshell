package nucleo

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recolector struct {
	mu  sync.Mutex
	evs []Evento
}

func (r *recolector) emitir(e Evento) {
	r.mu.Lock()
	r.evs = append(r.evs, e)
	r.mu.Unlock()
}

func (r *recolector) todos() []Evento {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Evento(nil), r.evs...)
}

func TestTrazaReceptoresNil(t *testing.T) {
	var n *Nodo
	if n.Sub(PasoPlan, "x %d", 1) != nil {
		t.Error("Sub en nil debe dar nil")
	}
	n.Nota("x")
	n.Detalle("x")
	n.Codigo("x")
	n.Tabla(Tabla{})
	n.Bien("")
	n.Mal("x")
	n.Info("x")
	n.Progreso("x")
	if n.Traza() != nil {
		t.Error("Traza() en nil")
	}
	var tr *Traza
	if tr.Raiz() != nil || tr.Pasos() != nil {
		t.Error("Traza nil")
	}
	tr.Terminar(&Respuesta{}, "terminada")
	if s, err := tr.Preguntar(context.Background(), PreguntaUsuario{PorDefecto: "A"}); s != "A" || !errors.Is(err, ErrSinRespuesta) {
		t.Errorf("Preguntar en nil: %q %v", s, err)
	}
	// the root node accepts children but ignores its own updates
	tr2 := NuevaTraza(nil, nil)
	raiz := tr2.Raiz()
	raiz.Detalle("x")
	raiz.Bien("y")
	raiz.Progreso("z")
	if len(tr2.Pasos()) != 0 {
		t.Error("la raíz no tiene paso propio")
	}
}

func TestTrazaPasosYEventos(t *testing.T) {
	r := &recolector{}
	tr := NuevaTraza(r.emitir, nil)
	plan := tr.Raiz().Sub(PasoPlan, "Plan: %s", "sumar")
	plan.Detalle("línea 1")
	plan.Detalle("línea 2")
	hijo := plan.Sub(PasoPrueba, "Pruebo %d casos", 3)
	hijo.Codigo("func F() {}")
	hijo.Tabla(Tabla{Cabecera: []string{"a"}, Filas: [][]string{{"1"}}})
	hijo.Bien("")
	plan.Mal("No lo he conseguido")
	plan.Nota("nota %d", 1)
	tr.Terminar(&Respuesta{Texto: "hecho"}, "terminada")

	pasos := tr.Pasos()
	if len(pasos) != 3 {
		t.Fatalf("hay %d pasos", len(pasos))
	}
	if pasos[0].Detalle != "línea 1\nlínea 2" || pasos[0].Estado != EstadoMal || pasos[0].Titulo != "No lo he conseguido" {
		t.Errorf("paso plan: %+v", pasos[0])
	}
	if pasos[1].Padre != pasos[0].ID || pasos[1].Estado != EstadoBien || pasos[1].Titulo != "Pruebo 3 casos" || pasos[1].Tabla == nil || pasos[1].Codigo == "" {
		t.Errorf("paso hijo: %+v", pasos[1])
	}
	if pasos[2].Tipo != PasoNota || pasos[2].Estado != EstadoInfo {
		t.Errorf("nota: %+v", pasos[2])
	}
	evs := r.todos()
	for i := 1; i < len(evs); i++ {
		if evs[i].Seq <= evs[i-1].Seq {
			t.Fatalf("Seq no crece: %d después de %d", evs[i].Seq, evs[i-1].Seq)
		}
	}
	if evs[len(evs)-2].Tipo != "respuesta" || evs[len(evs)-1].Tipo != "fin" || evs[len(evs)-1].Fin.Estado != "terminada" {
		t.Errorf("Terminar debe emitir respuesta y fin: %+v", evs[len(evs)-2:])
	}
}

func TestTrazaProgresoAgrupado(t *testing.T) {
	r := &recolector{}
	tr := NuevaTraza(r.emitir, nil)
	n := tr.Raiz().Sub(PasoIntento, "Busco")
	inicio := time.Now()
	for i := 0; i < 1000; i++ {
		n.Progreso("probé %d", i)
		time.Sleep(time.Second / 1000)
		if time.Since(inicio) > time.Second {
			break
		}
	}
	act := 0
	for _, e := range r.todos() {
		if e.Tipo == "actualiza" {
			act++
		}
	}
	if act == 0 || act > 5 {
		t.Fatalf("1000 llamadas a Progreso en 1 s dieron %d eventos actualiza (máximo 5)", act)
	}
	n.Bien("")
	if p := tr.Pasos()[0]; p.Progreso != "" {
		t.Errorf("cerrar un paso borra el progreso: %q", p.Progreso)
	}
}

func TestTrazaLimite(t *testing.T) {
	r := &recolector{}
	tr := NuevaTraza(r.emitir, nil)
	raiz := tr.Raiz()
	var ultimo *Nodo
	for i := 0; i < 2500; i++ {
		if n := raiz.Sub(PasoIntento, "intento %d", i); n != nil {
			ultimo = n
		}
	}
	pasos := tr.Pasos()
	if len(pasos) != 2001 {
		t.Fatalf("hay %d pasos; el límite es 2000 más la nota", len(pasos))
	}
	if pasos[2000].Titulo != "(más pasos omitidos)" {
		t.Errorf("última nota: %+v", pasos[2000])
	}
	notas := 0
	for _, e := range r.todos() {
		if e.Paso != nil && e.Paso.Titulo == "(más pasos omitidos)" {
			notas++
		}
	}
	if notas != 1 {
		t.Errorf("la nota de límite se emitió %d veces", notas)
	}
	ultimo.Bien("sigue funcionando") // updating an existing step still works
	if raiz.Sub(PasoNota, "más") != nil {
		t.Error("Sub tras el límite debe dar nil")
	}
}

func TestTrazaPreguntar(t *testing.T) {
	tr := NuevaTraza(nil, nil)
	s, err := tr.Preguntar(context.Background(), PreguntaUsuario{ID: "p1", Texto: "¿Qué debe dar con [-2]?", PorDefecto: "B"})
	if !errors.Is(err, ErrSinRespuesta) || s != "B" {
		t.Errorf("sin responder: %q %v", s, err)
	}
	r := &recolector{}
	tr = NuevaTraza(r.emitir, func(ctx context.Context, p PreguntaUsuario) (string, error) {
		return "A) " + p.Opciones[0], nil
	})
	s, err = tr.Preguntar(context.Background(), PreguntaUsuario{ID: "p2", Opciones: []string{"4", "0"}})
	if err != nil || s != "A) 4" {
		t.Errorf("con responder: %q %v", s, err)
	}
	if evs := r.todos(); len(evs) != 1 || evs[0].Tipo != "pregunta" || evs[0].Pregunta.ID != "p2" {
		t.Errorf("Preguntar debe emitir el evento pregunta: %+v", evs)
	}
}

func TestTrazaConcurrente(t *testing.T) {
	r := &recolector{}
	tr := NuevaTraza(r.emitir, func(ctx context.Context, p PreguntaUsuario) (string, error) { return "sí", nil })
	raiz := tr.Raiz()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			n := raiz.Sub(PasoIntento, "goroutine %d", g)
			for i := 0; i < 100; i++ {
				h := n.Sub(PasoPrueba, "caso %d", i)
				h.Detalle("d")
				h.Progreso("p %d", i)
				if i%2 == 0 {
					h.Bien("")
				} else {
					h.Mal("")
				}
			}
			_, _ = tr.Preguntar(context.Background(), PreguntaUsuario{ID: "x"})
			_ = tr.Pasos()
		}(g)
	}
	wg.Wait()
	tr.Terminar(nil, "terminada")
	evs := r.todos()
	for i := 1; i < len(evs); i++ {
		if evs[i].Seq != evs[i-1].Seq+1 {
			t.Fatalf("Seq debe crecer de uno en uno: %d → %d", evs[i-1].Seq, evs[i].Seq)
		}
	}
	if len(tr.Pasos()) != 8*101 {
		t.Errorf("hay %d pasos, esperaba %d", len(tr.Pasos()), 8*101)
	}
}
