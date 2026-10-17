package puzles

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Tests of puzles.tareas: plans with dependencies (the idea of planes.go and the causal plan of
// Gnostos v6 in Nyx).

func TestTareasEjemplos(t *testing.T) {
	casos := []struct {
		texto  string
		plan   []string
		quiere []string
	}{
		{"ordena estas tareas: preparar la masa y calentar el horno; luego hornear el pan; después comer",
			[]string{"preparar la masa ‖ calentar el horno", "hornear el pan", "comer"},
			[]string{"Orden en 3 etapas", "lo dices con «luego»", "las 3 dependencias"}},
		{"primero lavar la ropa, luego tender la ropa y después planchar la ropa. ¿en qué orden?",
			[]string{"lavar la ropa", "tender la ropa", "planchar la ropa"}, nil},
		{"organiza estas tareas: prepara la masa y calienta el horno; luego hornea el pan",
			[]string{"prepara la masa ‖ calienta el horno", "hornea el pan"}, nil},
		{"ordena estas tareas: después de hornear el pan, comer; preparar la masa antes de hornear el pan",
			[]string{"preparar la masa", "hornear el pan", "comer"}, []string{"lo dices con «después de»", "lo dices con «antes de»"}},
		{"ordena estas tareas: comer cuando termines de hornear el pan; hornear el pan después de calentar el horno",
			[]string{"calentar el horno", "hornear el pan", "comer"}, nil},
		{"dame un plan: comprar pan y leche; luego desayunar",
			[]string{"comprar pan y leche", "desayunar"}, nil},
		// causal: who uses a noun waits for who produced it, even without a connector
		{"¿en qué orden hago esto? comprar harina, hacer la masa con la harina, hornear la masa",
			[]string{"comprar harina", "hacer la masa con la harina", "hornear la masa"},
			[]string{"usa «harina», que sale de «comprar harina»", "no lo dices, lo deduzco"}},
		// through an intermediate stage (the test of Gnostos v6: "subir árbol seco" waits for "juntar árbol seco")
		{"ordena estas tareas: subir la leña al faro; reparar la puerta; juntar la leña seca",
			[]string{"reparar la puerta ‖ juntar la leña seca", "subir la leña al faro"}, []string{"usa «leña», que sale de «juntar la leña seca»"}},
		{"ordena estas tareas: preparar la masa, estirar la masa y cortar la masa",
			[]string{"preparar la masa", "estirar la masa ‖ cortar la masa"}, nil},
		{"ordena estas tareas: hacer la masa; hornear la masa", []string{"hacer la masa", "hornear la masa"}, nil},
		// without a verb that makes it, the earlier task with the noun counts as its maker (Gnostos v6)
		{"ordena estas tareas: subir la caja; vaciar la caja", []string{"subir la caja", "vaciar la caja"}, nil},
		{"¿qué hago primero, lavar los platos o barrer?", []string{"lavar los platos ‖ barrer"}, []string{"Ninguna depende de otra"}},
		// eating uses the bread that baking makes, even when it is written first
		{"ordena estas tareas: comer el pan y hornear el pan", []string{"hornear el pan", "comer el pan"}, nil},
		// the order that is said wins over a deduced one
		{"ordena estas tareas: comer el pan antes de hornear el pan", []string{"comer el pan", "hornear el pan"}, nil},
	}
	for _, c := range casos {
		if m := mejor(c.texto); m != "puzles.tareas" {
			t.Errorf("%q: lo reconoce %q", c.texto, m)
			continue
		}
		r := resolver(t, "puzles.tareas", c.texto)
		if strings.Join(r.Plan, " | ") != strings.Join(c.plan, " | ") {
			t.Errorf("%q: plan %q, quería %q", c.texto, r.Plan, c.plan)
		}
		todo := r.Texto + "\n" + strings.Join(r.Parrafos, "\n")
		for _, q := range c.quiere {
			if !strings.Contains(todo, q) {
				t.Errorf("%q: falta %q en:\n%s", c.texto, q, todo)
			}
		}
		if r.Nivel != nucleo.NivelCalculo || !r.Exito || r.Comprobaciones[0].Pasan != r.Comprobaciones[0].Total {
			t.Errorf("%q: nivel %s éxito %v %+v", c.texto, r.Nivel, r.Exito, r.Comprobaciones)
		}
	}
}

func TestTareasCiclos(t *testing.T) {
	for _, c := range []struct {
		texto  string
		quiere []string
	}{
		{"ordena estas tareas: pintar la pared antes de lijar la pared; lijar la pared antes de pintar la pared",
			[]string{"«pintar la pared» va antes que «lijar la pared»", "«lijar la pared» va antes que «pintar la pared»"}},
		{"ordena estas tareas: a: lavar antes de secar; secar antes de planchar; planchar antes de lavar",
			[]string{"«lavar» va antes que «secar»", "«secar» va antes que «planchar»", "«planchar» va antes que «lavar»"}},
	} {
		r := resolver(t, "puzles.tareas", c.texto)
		if !strings.Contains(r.Texto, "hay un ciclo") || r.Metodo != "ciclo" || r.Nivel != nucleo.NivelCalculo {
			t.Errorf("%q: %q (%s)", c.texto, r.Texto, r.Metodo)
		}
		todo := strings.Join(r.Parrafos, "\n")
		for _, q := range c.quiere {
			if !strings.Contains(todo, q) {
				t.Errorf("%q: falta %q en %s", c.texto, q, todo)
			}
		}
	}
}

// The check catches an order that breaks a dependency.
func TestTareasComprobacion(t *testing.T) {
	pt := leerTareas("ordena estas tareas: preparar la masa; luego hornear el pan; después comer")
	if !pt.ordenar(context.Background()) || !pt.comprobarOrden() {
		t.Fatalf("el orden bueno no pasa la comprobación: %+v", pt)
	}
	pt.nivel[0], pt.nivel[2] = pt.nivel[2], pt.nivel[0]
	pt.etapas[0], pt.etapas[2] = pt.etapas[2], pt.etapas[0]
	if pt.comprobarOrden() {
		t.Fatal("un orden al revés pasa la comprobación")
	}
}

// puzles.tareas does not take other questions (the golden exam's are checked by TestResolutoresExamen).
func TestTareasNoRoba(t *testing.T) {
	for _, s := range []string{"ordena alfabéticamente: pera, manzana, uva", "ordena la lista 3, 1, 2", "ordena [3,1,2]",
		"haz una función que primero ordene la lista y luego quite los repetidos", "el lobo, la cabra y la col",
		"jarras de 3 y 5 litros, ¿cómo saco 4?", "torres de Hanói con 3 discos", "si llueve el suelo se moja; llueve. ¿se moja?",
		"primero lavar la ropa y luego tenderla", "x=g=t; x=1 ¿cuánto vale t?", "ordena: 5, 3, 8",
		"tengo 90 litros de aceite y gasto 3 por hora; ¿cuánto dura?", "programa que ordene las tareas: leer y escribir",
		"hola", "¿qué orden tiene un grupo?"} {
		p := &nucleo.Pregunta{Texto: s}
		if rc := (resTareas{}).Reconoce(p); rc.Puntos > 0 {
			t.Errorf("puzles.tareas reconoce %q con %.2f", s, rc.Puntos)
		}
	}
	if rc := (resTareas{}).Reconoce(&nucleo.Pregunta{Texto: "ordena estas tareas: a y b", Codigo: "func main() {}"}); rc.Puntos > 0 {
		t.Error("con código no es un plan")
	}
}

func TestTareasPatologicas(t *testing.T) {
	var muchas []string
	for i := 0; i < 150; i++ {
		muchas = append(muchas, fmt.Sprintf("hacer cosa%d", i))
	}
	casos := []string{
		"ordena estas tareas: " + strings.Join(muchas, " y luego "),
		"ordena estas tareas: " + strings.Join(muchas[:60], ", ") + "; luego " + strings.Join(muchas[60:120], ", "),
		"ordena estas tareas: " + strings.Repeat("luego ", 500),
		"ordena estas tareas: " + strings.Repeat("antes de ", 300) + "comer",
		"ordena estas tareas: " + strings.Repeat(";", 2000),
		"ordena estas tareas:",
		"ordena estas tareas: ¿¿¿???",
		"ordena estas tareas: comer y comer y comer; luego comer",
		"ordena estas tareas: " + strings.Repeat("a", 5000),
		"ordena estas tareas: preparar\x00 la masa; luego 😀 hornear",
	}
	for _, s := range casos {
		ini := time.Now()
		func() {
			defer func() {
				if x := recover(); x != nil {
					t.Errorf("%.40q: pánico %v", s, x)
				}
			}()
			p := &nucleo.Pregunta{Texto: s}
			(resTareas{}).Reconoce(p)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			r, err := (resTareas{}).Resolver(ctx, p, nucleo.NuevaTraza(nil, nil).Raiz())
			if err == nil && r.Nivel == nucleo.NivelCalculo && r.Metodo == "orden_topologico" && len(r.Plan) == 0 {
				t.Errorf("%.40q: comprobado sin plan", s)
			}
		}()
		if d := time.Since(ini); d > time.Second {
			t.Errorf("%.40q tardó %v", s, d)
		}
	}
	// the chained 100 tasks (the cap) give 100 stages
	pt := leerTareas("ordena estas tareas: " + strings.Join(muchas[:100], " y luego "))
	if !pt.ordenar(context.Background()) || len(pt.etapas) != 100 || !pt.comprobarOrden() {
		t.Fatalf("100 tareas encadenadas: %d etapas", len(pt.etapas))
	}
	// Reconoce stays cheap
	largo := "ordena estas tareas: " + strings.Repeat("hacer una cosa, ", 300)
	p := &nucleo.Pregunta{Texto: largo[:maxTextoTareas]}
	ini := time.Now()
	for i := 0; i < 20; i++ {
		(resTareas{}).Reconoce(p)
	}
	if d := time.Since(ini) / 20; d > 5*time.Millisecond {
		t.Errorf("Reconoce tardó %v", d)
	}
}
