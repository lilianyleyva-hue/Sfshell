package cerebro

import (
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// The reasoning taken from the Gnostos engine of Nyx, asked through the real Cerebro: chains of
// equalities (mates.cadena), resources that run out (mates.recursos) and plans with dependencies
// (puzles.tareas). Each question gets a new Cerebro, so every strategy starts with the same weight.

func TestGnostosPorElCerebro(t *testing.T) {
	casos := []struct {
		texto, resolutor, metodo string
		quiere                   []string
	}{
		{"x=g=t=u=e=s=q=l; x=1 ¿cuánto vale l?", "mates.cadena", "union-busqueda",
			[]string{"l = 1", "La cadena: l = q = s = e = u = t = g = x = 1", "así que g = 1", "Comprobado por otro camino"}},
		{"si x = g, g = t y t = 5, ¿cuánto vale x?", "mates.cadena", "union-busqueda", []string{"x = 5", "x = g = t = 5"}},
		{"x=g=t=u=e=s=q=l. ¿es x igual a l?", "mates.cadena", "union-busqueda", []string{"Sí: x = l"}},
		{"x=g=t=u=e=s=q=l; x=1; l=2 ¿cuánto vale t?", "mates.cadena", "union-busqueda",
			[]string{"t no tiene un valor único", "se contradicen", "1 tendría que ser igual a 2"}},
		{"tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora; ¿cuánto dura?", "mates.recursos", "exacto",
			[]string{"Dura 30 horas: lo primero que se acaba es el aceite", "quedan 40 metros de mecha", "Runge-Kutta"}},
		{"tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora, y a las 10 horas añado 50 de mecha; ¿qué se acaba primero?",
			"mates.recursos", "exacto", []string{"Se acaba primero el aceite, al cabo de 30 horas", "quedan 90 metros de mecha"}},
		{"tengo 90 litros de aceite y gasto 3 litros por hora sin bajar de 10. ¿cuándo se acaba?", "mates.recursos", "exacto",
			[]string{"Se acaba al cabo de 80/3 (≈ 26,6667) horas"}},
		{"tengo 90 litros de aceite y gasto 3*exp(-0.01*t) por hora; ¿cuánto dura?", "mates.recursos", "aproximado",
			[]string{"Dura ≈ 35,6675 horas (aproximado)", "error estimado"}},
		{"ordena estas tareas: preparar la masa y calentar el horno; luego hornear el pan; después comer", "puzles.tareas", "orden_topologico",
			[]string{"1. preparar la masa ‖ calentar el horno", "2. hornear el pan", "3. comer", "lo dices con «luego»"}},
		{"¿en qué orden hago esto? comprar harina, hacer la masa con la harina, hornear la masa", "puzles.tareas", "orden_topologico",
			[]string{"1. comprar harina", "2. hacer la masa con la harina", "3. hornear la masa", "lo deduzco"}},
		{"ordena estas tareas: pintar la pared antes de lijar la pared; lijar la pared antes de pintar la pared", "puzles.tareas", "ciclo",
			[]string{"No se pueden ordenar: hay un ciclo"}},
	}
	for _, cs := range casos {
		c := nuevoCerebroPrueba(t, nil)
		r, tr := preguntar(t, c, "s", cs.texto, nil)
		todo := todoElTexto(r)
		if !r.Exito || r.Nivel != nucleo.NivelCalculo || r.Metodo != cs.metodo {
			t.Errorf("%q: éxito %v nivel %s método %s: %s\n%s", cs.texto, r.Exito, r.Nivel, r.Metodo, r.Texto, titulos(tr))
			continue
		}
		for _, q := range cs.quiere {
			if !strings.Contains(todo, q) {
				t.Errorf("%q: falta %q en:\n%s", cs.texto, q, todo)
			}
		}
		if c.stats.Usos("estrategia:"+cs.resolutor) != 1 {
			t.Errorf("%q: no lo contestó %s\n%s", cs.texto, cs.resolutor, titulos(tr))
		}
		if !strings.Contains(titulos(tr), "Lo intento como") {
			t.Errorf("%q: falta la traza del razonamiento:\n%s", cs.texto, titulos(tr))
		}
	}
}

// The questions of the resolvers that were already there keep going to them.
func TestGnostosNoRobaPreguntas(t *testing.T) {
	casos := []struct{ texto, resolutor, quiere string }{
		{"María tiene 12 caramelos y regala 5. ¿Cuántos le quedan?", "mates.problema", "7"},
		{"x+y=10, x-y=2", "mates.ecuacion", "x = 6"},
		{"jarras de 3 y 5 litros, ¿cómo saco 4?", "puzles.plan", "6 pasos"},
		{"el lobo, la cabra y la col", "puzles.plan", "7 pasos"},
		{"si llueve el suelo se moja; llueve. ¿se moja?", "logica.proposicional", "Sí"},
	}
	for _, cs := range casos {
		c := nuevoCerebroPrueba(t, nil)
		r, tr := preguntar(t, c, "s", cs.texto, nil)
		if c.stats.Usos("estrategia:"+cs.resolutor) != 1 || !strings.Contains(r.Texto, cs.quiere) {
			t.Errorf("%q: %q\n%s", cs.texto, r.Texto, titulos(tr))
		}
		for _, nuevo := range []string{"mates.cadena", "mates.recursos", "puzles.tareas"} {
			if c.stats.Usos("estrategia:"+nuevo) != 0 {
				t.Errorf("%q: lo intentó %s", cs.texto, nuevo)
			}
		}
	}
}
