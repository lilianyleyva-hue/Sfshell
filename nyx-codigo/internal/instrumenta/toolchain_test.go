package instrumenta_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"nyxcodigo/internal/instrumenta"
	"nyxcodigo/internal/nucleo"
)

const candidato = `package main

func Bucle() int {
	n := 0
	for {
		n++
	}
}

func Profundo(n int) int {
	if n == 0 {
		return 0
	}
	return 1 + Profundo(n-1)
}

func Clasifica(n int) string {
	if n == 0 {
		return "cero"
	} else {
		return "otro"
	}
}
`

const conductor = `package main

import (
	"fmt"
	"os"
)

func probar(f func()) (resultado string) {
	defer func() {
		switch r := recover().(type) {
		case nil:
			resultado = "sin pánico"
		case nyx__SinComb:
			resultado = "SinComb: " + r.Error()
		case nyx__Hondo:
			resultado = "Hondo: " + r.Error()
		default:
			resultado = fmt.Sprint("otro pánico: ", r)
		}
	}()
	f()
	return
}

func main() {
	switch os.Args[1] {
	case "bucle":
		nyx__Reiniciar(100000)
		fmt.Println(probar(func() { Bucle() }))
	case "hondo":
		nyx__Reiniciar(1 << 40)
		fmt.Println(probar(func() { Profundo(1 << 30) }))
	case "cobertura":
		nyx__Reiniciar(1000)
		fmt.Println(Clasifica(0), nyx__Cubiertas())
	}
}
`

// TestConGoDeVerdad builds the instrumented code with the real toolchain and checks fuel, depth and coverage.
func TestConGoDeVerdad(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no hay Go instalado")
	}
	op := nucleo.OpcionesInstr{Combustible: 100000, MaxPila: 1000, Cobertura: true}
	r, err := instrumenta.Instrumentar(candidato, op)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archivos := map[string]string{
		"go.mod":       "module prueba\n\ngo 1.22\n",
		"candidato.go": r.Fuente,
		"soporte.go":   instrumenta.Soporte("main", op, len(r.Lineas)),
		"conductor.go": conductor,
	}
	for n, c := range archivos {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog := filepath.Join(dir, "prog")
	build := exec.Command(goBin, "build", "-o", prog, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOPROXY=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("no compila: %v\n%s\n%s", err, out, r.Fuente)
	}
	correr := func(arg string) string {
		out, err := exec.Command(prog, arg).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", arg, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if got := correr("bucle"); !strings.HasPrefix(got, "SinComb: "+instrumenta.MensajeSinComb) {
		t.Errorf("for {} debería quedarse sin combustible: %q", got)
	}
	if got := correr("hondo"); !strings.HasPrefix(got, "Hondo: ") {
		t.Errorf("la recursión profunda debería dar Hondo: %q", got)
	}
	// Clasifica: if(18) return "cero"(19) return "otro"(21); input 0 never reaches the else branch.
	got := correr("cobertura")
	idElse := -1
	for id, l := range r.Lineas {
		if l == 21 {
			idElse = id
		}
	}
	if idElse < 0 {
		t.Fatalf("no encuentro la sentencia de la rama else en %v", r.Lineas)
	}
	if !strings.HasPrefix(got, "cero [") || strings.Contains(got, " "+itoa(idElse)+"]") || strings.Contains(got, "["+itoa(idElse)+" ") {
		t.Errorf("la rama else no debería estar cubierta con 0: %q (id else = %d, Lineas %v)", got, idElse, r.Lineas)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
