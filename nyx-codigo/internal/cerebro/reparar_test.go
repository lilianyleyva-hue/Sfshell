package cerebro

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

// cerebroConGo is the exam's Cerebro (real arenero, real memoria, fake web); it skips without Go.
func cerebroConGo(t *testing.T) *Cerebro {
	t.Helper()
	if testing.Short() {
		t.Skip("prueba con el compilador de verdad (sin -short)")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no hay Go instalado")
	}
	c := cerebroExamen(t)
	if c.ej == nil {
		t.Skip("sin arenero")
	}
	return c
}

// pedir sends one request (with optional fixed answers to the questions Nyx asks).
func pedir(t *testing.T, c *Cerebro, p nucleo.Peticion, fijas ...string) (nucleo.Respuesta, *nucleo.Traza) {
	t.Helper()
	if p.Tarea == "" {
		p.Tarea = "t" + itoa(numTarea.Add(1))
	}
	if p.Sesion == "" {
		p.Sesion = "s-" + p.Tarea
	}
	tr := nucleo.NuevaTraza(nil, nucleotest.RespuestasFijas(fijas...))
	ctx, cancelar := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancelar()
	return c.Responder(ctx, p, tr), tr
}

func contiene(r nucleo.Respuesta, s string) bool {
	todo := plegar(todoElTexto(r) + "\n" + r.Diff + "\n" + r.CodigoTest)
	return strings.Contains(todo, plegar(s))
}

func TestRepararExamen(t *testing.T) {
	c := cerebroConGo(t)
	casos := []struct {
		nombre, texto, quiero string
		nivel                 nucleo.Nivel
	}{
		{"importar", "no compila:\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(strings.ToUpper(\"hola\"))\n}\n", "importar", nucleo.NivelSinFallos},
		{"sin_usar", "no compila:\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tx := 5\n\tfmt.Println(\"hola\")\n}\n", "nunca la usas", nucleo.NivelSinFallos},
		{"length", "arregla esto:\nfunc Suma(xs []int) int {\n\ttotal := 0\n\tfor i := 0; i < xs.length; i++ {\n\t\ttotal += xs[i]\n\t}\n\treturn total\n}\n", "len(xs)", nucleo.NivelSinFallos},
		{"maximo", "arregla esta función: debería dar el máximo, pero con [3,1,2] da 1; con [3,1,2] debería dar 3\nfunc Maximo(xs []int) int {\n\tm := xs[0]\n\tfor _, x := range xs {\n\t\tif x < m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n", "Cumple tus", nucleo.NivelTusEjemplos},
		{"indice", "me sale index out of range:\nfunc Suma(xs []int) int {\n\ttotal := 0\n\tfor i := 0; i <= len(xs); i++ {\n\t\ttotal += xs[i]\n\t}\n\treturn total\n}\n", "< len(xs)", nucleo.NivelSinFallos},
		{"printl", "no compila:\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printl(\"hola\")\n}\n", "Println", nucleo.NivelSinFallos},
	}
	for _, cs := range casos {
		t.Run(cs.nombre, func(t *testing.T) {
			inicio := time.Now()
			r, tr := pedir(t, c, nucleo.Peticion{Texto: cs.texto})
			t.Logf("%v: %s [%s]\n%s", time.Since(inicio).Round(time.Millisecond), r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"))
			if !contiene(r, cs.quiero) {
				t.Errorf("falta %q en la respuesta:\n%s\n%s\ntraza: %s", cs.quiero, todoElTexto(r), r.Diff, titulos(tr))
			}
			if fuerzaNivel[string(r.Nivel)] < fuerzaNivel[string(cs.nivel)] {
				t.Errorf("nivel %s, quiero al menos %s\n%s", r.Nivel, cs.nivel, titulos(tr))
			}
		})
	}
}

// §11: reparar fixes testdata/rotos/max_mal.go with the example "con [3,1,2] debería dar 3", and the fix
// verified with the user's example is stored.
func TestRepararMaxMal(t *testing.T) {
	c := cerebroConGo(t)
	b, err := os.ReadFile(filepath.Join("testdata", "rotos", "max_mal.go"))
	if err != nil {
		t.Fatal(err)
	}
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "arréglalo: con [3,1,2] debería dar 3", Codigo: string(b)})
	if !r.Exito || r.Nivel != nucleo.NivelTusEjemplos {
		t.Fatalf("no lo arregló: %s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	if !strings.Contains(r.Codigo, "x > m") || r.Diff == "" {
		t.Errorf("el arreglo debería cambiar «<» por «>»:\n%s\n%s", r.Codigo, r.Diff)
	}
	if !contiene(r, "Antes fallaba con [3, 1, 2]") {
		t.Errorf("falta el contraejemplo del original:\n%s", todoElTexto(r))
	}
	if len(r.Tablas) < 2 || r.Tablas[1].Titulo != "Antes y después del arreglo" {
		t.Errorf("faltan las tablas: %+v", r.Tablas)
	}
	if r.Aprendido == "" {
		t.Errorf("un arreglo comprobado con tus ejemplos se guarda")
	}
	encontrada := false
	for _, f := range c.mem.Funciones() {
		if f.Origen == "reparacion" && strings.HasPrefix(f.Nombre, "Maximo") {
			encontrada = true
		}
	}
	if !encontrada {
		t.Errorf("no está en la memoria")
	}
}

// With no spec and no failures it says so plainly instead of claiming the code is right.
func TestRepararSinEspecificacion(t *testing.T) {
	c := cerebroConGo(t)
	r, _ := pedir(t, c, nucleo.Peticion{Texto: "arregla esto", Codigo: "func Raro(a, b int) int {\n\treturn a*3 - b\n}\n"})
	if !strings.Contains(r.Texto, "no sé qué debe dar") || r.Nivel != nucleo.NivelSinFallos {
		t.Errorf("%s [%s]", r.Texto, r.Nivel)
	}
}

// Without code it asks for it, and without an answer it says what it needs.
func TestRepararPideCodigo(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	k := &tarea{ctx: context.Background(), ctxPadre: context.Background(), p: nucleo.Peticion{Texto: "arréglalo"},
		q: &nucleo.Pregunta{}, s: c.sesion("x"), t: nucleo.NuevaTraza(nil, nil), turno: &Turno{}, lex: c.lexico()}
	k.raiz = k.t.Raiz()
	r := pipeReparar(c, k)
	if r.Exito || !strings.Contains(r.Texto, "Necesito el código") {
		t.Errorf("%+v", r)
	}
}

func TestRepCompletar(t *testing.T) {
	s := repCompletar("func Suma(xs []int) int {\n\treturn 0\n}")
	if s.paquete != "solucion" || !strings.HasPrefix(s.fuente, "package solucion; func Suma") || s.desfase != 0 {
		t.Errorf("%+v", s)
	}
	s = repCompletar("x := 3\nfmt.Println(strings.ToUpper(\"a\"), x)")
	if s.paquete != "main" || !strings.Contains(s.fuente, "func main()") || strings.Join(s.importes, ",") != "fmt,strings" {
		t.Errorf("%+v", s)
	}
	s = repCompletar("package main\n\nfunc main() {}\n")
	if s.paquete != "main" || s.fuente != "package main\n\nfunc main() {}\n" {
		t.Errorf("%+v", s)
	}
	if got := strings.Join(repPartirNombre("SumaParesDe_lista2"), " "); got != "suma pares de lista" {
		t.Errorf("%q", got)
	}
	if got := repPartirComas(`[1,2], "a,b", 3`); len(got) != 3 {
		t.Errorf("%q", got)
	}
}

// Two fixes that disagree: it asks "¿esperas A o B?" and the answer picks the fix (and becomes a case).
func TestRepararPreguntaCuandoHayDosArreglos(t *testing.T) {
	c := cerebroConGo(t)
	codigo := "func Junta(a, b int) int {\n\treturn a - b\n}\n"
	var codigos []string
	for opcion := 0; opcion < 2; opcion++ {
		preguntado := ""
		elegir := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
			preguntado = p.Texto
			if len(p.Opciones) <= opcion {
				return "", context.Canceled
			}
			return p.Opciones[opcion], nil
		}
		tr := nucleo.NuevaTraza(nil, elegir)
		ctx, cancelar := context.WithTimeout(context.Background(), 120*time.Second)
		r := c.Responder(ctx, nucleo.Peticion{Tarea: "amb" + itoa(int64(opcion)), Sesion: "amb" + itoa(int64(opcion)), Texto: "arregla esto: (2, 2) -> 4", Codigo: codigo}, tr)
		cancelar()
		if !strings.Contains(preguntado, "¿esperas") {
			t.Skipf("no preguntó (solo encontró un arreglo): %s\n%s", r.Texto, titulos(tr))
		}
		if !r.Exito || r.Nivel != nucleo.NivelTusEjemplos {
			t.Fatalf("opción %d de «%s»: %s [%s]\n%s\n%s", opcion, preguntado, r.Texto, r.Nivel, r.Codigo, titulos(tr))
		}
		codigos = append(codigos, r.Codigo)
	}
	if codigos[0] == codigos[1] {
		t.Errorf("cada respuesta debería elegir un arreglo distinto:\n%s", codigos[0])
	}
}
