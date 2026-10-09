//go:build toolchain

package analisis_test

// Observation through the real sandbox:
//
//	go test -tags toolchain ./internal/analisis/
//
// They skip when Go is not installed.

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/nucleo"
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
	dir, err := os.MkdirTemp("", "nyx-analisis-prueba-")
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

const sumaPares = `package solucion

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

const parejas = `package solucion

// Parejas mezcla cada pareja de elementos (dos bucles anidados) con algo de trabajo en cada una, para que
// los tiempos sean grandes comparados con el ruido.
func Parejas(nums []int) int {
	n := 0
	for i := 0; i < len(nums); i++ {
		for j := 0; j < len(nums); j++ {
			x := nums[i]*31 + nums[j]
			for k := 0; k < 8; k++ {
				x = x*1103515245 + 12345
			}
			if x == 0 {
				n++
			}
		}
	}
	return n
}
`

func firma(nombre string) nucleo.Firma {
	return nucleo.Firma{Nombre: nombre, Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
}

func lista(xs ...int) nucleo.Valor {
	out := make([]nucleo.Valor, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func preparar(t *testing.T, src string, f nucleo.Firma, instr nucleo.OpcionesInstr) nucleo.Binario {
	t.Helper()
	b, comp, err := conGo(t).Preparar(bg, nucleo.Preparacion{Variantes: []string{src}, Firma: f, Instr: instr, Permiso: nucleo.PermisoAuto})
	if err != nil {
		t.Fatalf("Preparar: %v\n%s", err, comp.Texto)
	}
	t.Cleanup(func() { b.Cerrar() })
	return b
}

func TestAreneroObservar(t *testing.T) {
	f := firma("SumaPares")
	b := preparar(t, sumaPares, f, nucleo.InstrNormal)
	az := rand.New(rand.NewSource(1))
	var aleatorios []nucleo.Caso
	for i := 0; i < 300; i++ {
		xs := make([]int, az.Intn(12))
		for j := range xs {
			xs[j] = az.Intn(201) - 100
		}
		aleatorios = append(aleatorios, nucleo.Caso{Entradas: []nucleo.Valor{lista(xs...)}, Origen: nucleo.OrigenAzar})
	}
	oraculo := func(in []nucleo.Valor) ([]nucleo.Valor, error) {
		total := 0
		for _, v := range in[0].([]nucleo.Valor) {
			if n := v.(int); n%2 == 0 {
				total += n
			}
		}
		return []nucleo.Valor{total}, nil
	}
	resumir := func(ctx context.Context, f nucleo.Firma, casos []nucleo.Caso) (string, func([]nucleo.Valor) ([]nucleo.Valor, error), bool) {
		return "devuelve la suma de los números pares de nums", oraculo, true
	}
	sondas := nucleo.Sondas(f)
	c, err := analisis.Observar(bg, b, f, sondas, aleatorios, resumir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Verificado || c.Pruebas != 300 || len(c.Tabla.Filas) != len(sondas) || c.Huella == "" {
		t.Errorf("Observar = %+v", c)
	}
	// a wrong summary is not reported
	malo := func(ctx context.Context, f nucleo.Firma, casos []nucleo.Caso) (string, func([]nucleo.Valor) ([]nucleo.Valor, error), bool) {
		return "devuelve 0", func([]nucleo.Valor) ([]nucleo.Valor, error) { return []nucleo.Valor{0}, nil }, true
	}
	c, err = analisis.Observar(bg, b, f, sondas, aleatorios, malo, nil, nil)
	if err != nil || c.Verificado || c.Resumen != "" {
		t.Errorf("resumen falso: %+v %v", c, err)
	}
}

func TestAreneroTrazaVariables(t *testing.T) {
	f := firma("SumaPares")
	b := preparar(t, sumaPares, f, nucleo.OpcionesInstr{Variables: true})
	ev, err := analisis.TrazaVariables(bg, b, nucleo.Caso{Entradas: []nucleo.Valor{lista(1, 2, 3, 4)}, Origen: nucleo.OrigenUsuario})
	if err != nil {
		t.Fatal(err)
	}
	vistos := map[string]bool{}
	for _, e := range ev {
		if e.Var == "total" {
			vistos[e.Valor] = true
		}
	}
	if !vistos["0"] || !vistos["2"] || !vistos["6"] {
		t.Errorf("la traza de total no tiene 0, 2 y 6: %+v", ev)
	}
}

func TestAreneroCrecimiento(t *testing.T) {
	f := firma("Parejas")
	b := preparar(t, parejas, f, nucleo.OpcionesInstr{})
	caso := func(n int) (nucleo.Caso, bool) {
		xs := make([]int, n)
		for i := range xs {
			xs[i] = i + 1
		}
		return nucleo.Caso{Entradas: []nucleo.Valor{lista(xs...)}, Origen: nucleo.OrigenAzar}, true
	}
	clase, m, err := analisis.Crecimiento(bg, b, caso, nil)
	if err != nil {
		t.Fatalf("Crecimiento: %v (%+v)", err, m)
	}
	if clase != "cuadrática" {
		t.Errorf("clase = %s, medidas %+v", clase, m)
	}
}
