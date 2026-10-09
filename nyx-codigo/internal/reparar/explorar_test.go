package reparar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo/nucleotest"
)

func TestExplorar(t *testing.T) {
	if os.Getenv("EXPLORAR") == "" {
		t.Skip()
	}
	archivos, _ := filepath.Glob("testdata/rotos/*.go")
	for _, a := range archivos {
		b, _ := os.ReadFile(a)
		src := string(b)
		if !strings.Contains(src, "quiero: compila") {
			continue
		}
		r := &Reparador{Ej: &nucleotest.EjecutorFalso{}}
		t0 := time.Now()
		ar := r.Compilacion(context.Background(), src, nil)
		t.Logf("==== %s ok=%v rondas=%d %v", filepath.Base(a), ar.OK, ar.Rondas, time.Since(t0))
		for _, c := range ar.Cambios {
			t.Logf("   %s L%d %q -> %q : %s", c.Regla, c.Linea, c.Antes, c.Despues, c.Porque)
		}
		for _, q := range ar.Quedan {
			t.Logf("   QUEDA %d: %s | %s", q.Linea, q.Msg, Traducir(q))
		}
		if !ar.OK {
			t.Logf("%s", ar.Fuente)
		}
	}
}
