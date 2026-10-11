package reparar

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo/nucleotest"
)

func TestZZTiempos(t *testing.T) {
	for _, n := range []int{2000, 8000} {
		var sb strings.Builder
		sb.WriteString("package solucion\n\nfunc F(xs []int) int {\n")
		for i := 0; i < n; i++ {
			sb.WriteString("\tlet x" + strconv.Itoa(i) + " = xs.length;\n")
		}
		sb.WriteString("\treturn 0\n}\n")
		inicio := time.Now()
		_, cs := Idiomas(sb.String())
		t.Logf("Idiomas %d líneas: %v, %d cambios", n, time.Since(inicio), len(cs))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		r := &Reparador{Ej: &nucleotest.EjecutorFalso{}, Limite: Limites{TCompilacion: time.Second}}
		inicio = time.Now()
		ar := r.Compilacion(ctx, sb.String(), nil)
		cancel()
		t.Logf("Compilacion %d líneas: %v ok=%v rondas=%d", n, time.Since(inicio), ar.OK, ar.Rondas)
	}
}
