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

// construirYCorrer instruments candidato, builds it with the support file and conductor, and runs it.
func construirYCorrer(t *testing.T, candidato, conductor string, op nucleo.OpcionesInstr, args ...string) (string, instrumenta.Resultado) {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no hay Go instalado")
	}
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
	out, err := exec.Command(prog, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return string(out), r
}

const conductorTraza = `package main

import "fmt"

func main() {
	nyx__Reiniciar(1000000)
	r := F(5)
	fmt.Println("resultado", r)
	fmt.Println("combustible", nyx__f)
	for _, id := range nyx__Cubiertas() {
		fmt.Println("cubierta", id)
	}
	for _, e := range nyx__Eventos() {
		fmt.Printf("evento %d %s=%s\n", e.Linea, e.Var, e.Valor)
	}
}
`

// lineasCubiertas returns the original lines of the covered statement ids printed by conductorTraza.
func lineasCubiertas(t *testing.T, out string, r instrumenta.Resultado) map[int]bool {
	t.Helper()
	m := map[int]bool{}
	for _, l := range strings.Split(out, "\n") {
		if s, ok := strings.CutPrefix(l, "cubierta "); ok {
			id := 0
			for _, c := range s {
				id = id*10 + int(c-'0')
			}
			m[r.Lineas[id]] = true
		}
	}
	return m
}

// Regression: a goto target is covered.
func TestCoberturaDeGoto(t *testing.T) {
	cand := "package main\n\nfunc F(n int) int {\n\tif n > 0 {\n\t\tgoto fin\n\t}\n\tn = -n\nfin:\n\treturn n\n}\n"
	out, r := construirYCorrer(t, cand, conductorTraza, nucleo.OpcionesInstr{Combustible: 1000, Cobertura: true})
	c := lineasCubiertas(t, out, r)
	if !c[4] || !c[9] || c[7] {
		t.Errorf("con 5 se ejecutan las líneas 4, 5 y 9 (no la 7): %v\n%s", c, out)
	}
}

// Regression: tracing variables never runs the program's own String/Error methods, never recurses, and does
// not spend fuel; standard-library values still show through their String method.
func TestTrazaNoLlamaMetodos(t *testing.T) {
	cand := `package main

import (
	"fmt"
	"time"
)

type P struct{ X int }

func (p P) String() string {
	s := fmt.Sprint(p.X)
	q := p
	return "(" + s + fmt.Sprint(q.X) + ")"
}

type E struct{ msg string }

func (e *E) Error() string { return "E:" + e.msg }

func F(n int) int {
	p := P{n}
	var err error = &E{"malo"}
	d := time.Duration(n) * time.Second
	m := map[string]int{"b": 2, "a": 1}
	xs := []P{{1}, {2}}
	_, _, _, _ = err, d, m, xs
	return p.X
}
`
	op := nucleo.OpcionesInstr{Combustible: 1000000, Cobertura: true, Variables: true}
	out, r := construirYCorrer(t, cand, conductorTraza, op)
	if !strings.Contains(out, "resultado 5") {
		t.Fatalf("salida: %s", out)
	}
	c := lineasCubiertas(t, out, r)
	for _, l := range []int{11, 12, 13, 18} {
		if c[l] {
			t.Errorf("la línea %d (String/Error) no se ejecuta en F, pero sale cubierta: %v", l, c)
		}
	}
	for _, mal := range []string{"evento 11 ", "evento 12 ", "E:malo", "(55)"} {
		if strings.Contains(out, mal) {
			t.Errorf("la traza ejecutó un método del programa (%q):\n%s", mal, out)
		}
	}
	for _, bien := range []string{"evento 21 p={5}", "evento 22 err=&{malo}", "evento 23 d=5s", "evento 24 m=map[a:1 b:2]", "evento 25 xs=[{1} {2}]"} {
		if !strings.Contains(out, bien+"\n") {
			t.Errorf("falta %q:\n%s", bien, out)
		}
	}
	// the same program without tracing spends exactly the same fuel
	out2, _ := construirYCorrer(t, cand, conductorTraza, nucleo.OpcionesInstr{Combustible: 1000000, Cobertura: true})
	comb := func(s string) string {
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(l, "combustible ") {
				return l
			}
		}
		return ""
	}
	if comb(out) != comb(out2) || comb(out) == "" {
		t.Errorf("la traza gastó combustible: %q contra %q", comb(out), comb(out2))
	}
}

// Regression: a standard-library value whose String/Error calls back into the program (errors.Join) is shown,
// but the program's code it runs leaves no events, no coverage and spends no fuel.
func TestTrazaGuardaReentrada(t *testing.T) {
	cand := `package main

import "errors"

type R struct{ n int }

func (e *R) Error() string {
	r := e
	return "R" + string(rune('0'+r.n))
}

func F(n int) int {
	j := errors.Join(&R{n})
	_ = j
	return n
}
`
	out, r := construirYCorrer(t, cand, conductorTraza, nucleo.OpcionesInstr{Combustible: 1000000, Cobertura: true, Variables: true})
	if !strings.Contains(out, "evento 13 j=R5\n") || strings.Contains(out, "evento 8 ") {
		t.Errorf("eventos:\n%s", out)
	}
	if c := lineasCubiertas(t, out, r); c[8] || c[9] {
		t.Errorf("Error() no la llama F: %v", c)
	}
	out2, _ := construirYCorrer(t, cand, conductorTraza, nucleo.OpcionesInstr{Combustible: 1000000, Cobertura: true})
	if !strings.Contains(out, "combustible 999999\n") || !strings.Contains(out2, "combustible 999999\n") {
		t.Errorf("combustible:\n%s\n%s", out, out2)
	}
}
