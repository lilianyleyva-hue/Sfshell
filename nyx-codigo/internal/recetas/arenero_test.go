//go:build toolchain

package recetas_test

// These tests run the shipped knowledge through the real sandbox (arenero), as the app does:
//
//	go test -tags toolchain ./internal/recetas/
//
// They skip when Go is not installed.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/recetas"
)

var ar *arenero.Arenero

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
	dir, err := os.MkdirTemp("", "nyx-recetas-prueba-")
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

func conGo(t *testing.T) *arenero.Arenero {
	t.Helper()
	if ar == nil || !ar.Estado().GoOK {
		t.Skip("no hay Go instalado")
	}
	return ar
}

var bg = context.Background()

// Everything shipped must pass the automatic-run safety check.
func TestAreneroRevisa(t *testing.T) {
	for _, r := range recetas.Funciones() {
		if vs := arenero.Revisar(r.Codigo, nucleo.PermisoAuto); len(vs) > 0 {
			t.Errorf("receta %s: %+v", r.Nombre, vs)
		}
		if prog, _, err := recetas.Envolver(r.Codigo, r.Firma); err == nil {
			if vs := arenero.Revisar(prog, nucleo.PermisoAuto); len(vs) > 0 {
				t.Errorf("envoltorio de %s: %+v", r.Nombre, vs)
			}
		}
	}
	for _, p := range recetas.Plantillas() {
		for i, c := range p.Casos {
			src, err := recetas.Instanciar(p, c.Huecos)
			if err != nil {
				t.Errorf("%s, caso %d: %v", p.Nombre, i+1, err)
				continue
			}
			if vs := arenero.Revisar(src, nucleo.PermisoAuto); len(vs) > 0 {
				t.Errorf("plantilla %s: %+v", p.Nombre, vs)
			}
		}
	}
}

func TestAreneroRecetas(t *testing.T) {
	a := conGo(t)
	for _, r := range recetas.Funciones() {
		r := r
		t.Run(r.Nombre, func(t *testing.T) {
			t.Parallel()
			b, comp, err := a.Preparar(bg, nucleo.Preparacion{
				Variantes: []string{r.Codigo}, Firma: r.Firma, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto,
			})
			if err != nil {
				t.Fatalf("Preparar: %v\n%s\n%+v", err, comp.Texto, comp.Errores)
			}
			defer b.Cerrar()
			rs, err := b.Probar(bg, r.Casos, nucleo.OpcionesProbar{})
			if err != nil {
				t.Fatalf("Probar: %v", err)
			}
			for i, rc := range rs[0] {
				if !rc.OK {
					t.Errorf("caso %d: obtenido %v, esperado %v, pánico %q", i+1, rc.Obtenido, r.Casos[i].Esperado, rc.Panico)
				}
			}
		})
	}
}

func TestAreneroPlantillas(t *testing.T) {
	a := conGo(t)
	for _, p := range recetas.Plantillas() {
		p := p
		t.Run(p.Nombre, func(t *testing.T) {
			t.Parallel()
			programas := map[string]nucleo.Programa{}
			defer func() {
				for _, pr := range programas {
					pr.Cerrar()
				}
			}()
			for i, c := range p.Casos {
				src, err := recetas.Instanciar(p, c.Huecos)
				if err != nil {
					t.Fatalf("caso %d: %v", i+1, err)
				}
				pr, ok := programas[src]
				if !ok {
					var comp nucleo.Compilacion
					pr, comp, err = a.PrepararPrograma(bg, src, nucleo.InstrNormal, nucleo.PermisoAuto)
					if err != nil {
						t.Fatalf("PrepararPrograma: %v\n%s\n%+v", err, comp.Texto, comp.Errores)
					}
					programas[src] = pr
				}
				e, err := pr.Correr(bg, c.Caso)
				if err != nil {
					t.Fatalf("caso %d: %v", i+1, err)
				}
				if !recetas.SalidaCoincide(e.Salida, c.Caso) {
					t.Errorf("caso %d: escribió\n%s\ny esperaba (%s)\n%s\nstderr: %s", i+1, e.Salida, c.Caso.Comparar, c.Caso.Esperado, e.ErrSalida)
				}
			}
		})
	}
}

func TestAreneroEnvolverSumaPares(t *testing.T) {
	a := conGo(t)
	src := `package solucion

func SumaPares(nums []int) int {
	total := 0
	for _, n := range nums {
		if n%2 == 0 {
			total += n
		}
	}
	return total
}
`
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	prog, aCaso, err := recetas.Envolver(src, f)
	if err != nil {
		t.Fatal(err)
	}
	pr, comp, err := a.PrepararPrograma(bg, prog, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		t.Fatalf("PrepararPrograma: %v\n%s", err, comp.Texto)
	}
	defer pr.Cerrar()
	cp, ok := aCaso(nucleo.Caso{Entradas: []nucleo.Valor{[]nucleo.Valor{1, 2, 3, 4}}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario})
	if !ok {
		t.Fatal("aCaso falló")
	}
	e, err := pr.Correr(bg, cp)
	if err != nil {
		t.Fatal(err)
	}
	if e.Salida != "6\n" || !recetas.SalidaCoincide(e.Salida, cp) {
		t.Errorf("salida %q (stderr %q)", e.Salida, e.ErrSalida)
	}
	if !strings.Contains(e.ErrSalida, "Escribe una lista de números") {
		t.Errorf("stderr = %q", e.ErrSalida)
	}
}
