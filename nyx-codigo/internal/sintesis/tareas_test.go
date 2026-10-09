package sintesis

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestTareas runs the task corpus (testdata/tareas.txt): at least 42 of the 50 tasks must be solved, each in
// under 3 s with seed 1, and the chosen program must also pass the held-out «prueba» cases.
func TestTareas(t *testing.T) {
	tareas, err := leerTareas("testdata/tareas.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(tareas) != 50 {
		t.Fatalf("hay %d tareas y deben ser 50", len(tareas))
	}
	if testing.Short() {
		tareas = tareas[:12]
	}
	limite := 3 * time.Second
	holgura := time.Duration(float64(limite) * factorCarrera)
	reg := Base()
	resueltas := 0
	var informe []string
	var fuentes []string
	for _, ta := range tareas {
		esp := Especificacion{Firma: ta.firma, Ejemplos: ta.ejs}
		if ta.marco != nil {
			esp.Conceptos = pesosMarco(ta.marco)
			esp.Constantes = constantesMarco(ta.marco)
			esp.Esqueletos = Esqueletos(ta.marco, ta.firma, reg)
		}
		inicio := time.Now()
		res, err := Sintetizar(context.Background(), esp, reg, Opciones{Limite: holgura, Semilla: 1}, nil)
		dur := time.Since(inicio)
		estado := "MAL"
		detalle := ""
		switch {
		case err != nil:
			detalle = err.Error()
		case len(res.Soluciones) == 0:
			detalle = fmt.Sprintf("sin solución (%s, %d explorados)", res.Motivo, res.Explorados)
		default:
			e := res.Soluciones[0].Expr
			detalle = e.String()
			if ok, por := cumple(e, ta.ejs); !ok {
				detalle += " no cumple los ejemplos: " + por
			} else if ok, por := cumple(e, ta.pruebas); !ok {
				detalle += " no generaliza: " + por
			} else if dur > holgura+holgura/10 {
				detalle += fmt.Sprintf(" tardó %v", dur)
			} else if src, err := AGo(e, ta.firma); err != nil {
				detalle += " AGo: " + err.Error()
			} else {
				fuentes = append(fuentes, src)
				estado = "bien"
				resueltas++
			}
		}
		informe = append(informe, fmt.Sprintf("%-20s %-4s %6dms %8d  %s", ta.nombre, estado, dur.Milliseconds(), res.Explorados, detalle))
	}
	t.Logf("tareas resueltas: %d de %d\n%s", resueltas, len(tareas), strings.Join(informe, "\n"))
	if _, err := exec.LookPath("go"); err == nil {
		for _, src := range fuentes {
			comprobarTipos(t, src)
		}
	}
	minimo := 42
	if testing.Short() {
		minimo = len(tareas) - 2
	}
	if resueltas < minimo {
		t.Errorf("resueltas %d de %d; hacen falta al menos %d", resueltas, len(tareas), minimo)
	}
}
