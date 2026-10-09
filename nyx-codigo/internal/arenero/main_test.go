package arenero_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"nyxcodigo/internal/arenero"
)

// ar is shared by every real-toolchain test of this package (nil without Go).
var ar *arenero.Arenero

// cacheCompartida is a persistent GOCACHE for tests, so each run does not start cold.
func cacheCompartida() string {
	if d, err := os.UserCacheDir(); err == nil {
		c := filepath.Join(d, "nyx-codigo-pruebas", "go-build")
		if os.MkdirAll(c, 0o700) == nil {
			return c
		}
	}
	return ""
}

func TestMain(m *testing.M) {
	if arenero.EsTrampolin() {
		arenero.Trampolin()
	}
	dir, err := os.MkdirTemp("", "nyx-arenero-prueba-")
	if err != nil {
		panic(err)
	}
	if _, err := exec.LookPath("go"); err == nil {
		ar, err = arenero.Nuevo(arenero.Config{DirDatos: dir, DirCache: cacheCompartida()})
		if err != nil {
			panic(err)
		}
	}
	codigo := m.Run()
	if ar != nil {
		ar.Cerrar()
	}
	os.RemoveAll(dir)
	os.Exit(codigo)
}

// conGo returns the shared sandbox or skips the test.
func conGo(t *testing.T) *arenero.Arenero {
	t.Helper()
	if ar == nil || !ar.Estado().GoOK {
		t.Skip("no hay Go instalado")
	}
	return ar
}
