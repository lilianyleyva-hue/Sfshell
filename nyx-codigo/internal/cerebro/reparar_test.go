package cerebro

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/reparar"
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

// Regression (review): the question counts the distinct outputs ("Hay 3 arreglos…"), not always "dos".
func TestRepararDesempatarCuentaArreglos(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	inf, _ := analisis.Analizar("package solucion\n\nfunc F(xs []int) int {\n\treturn 0\n}\n")
	if len(inf.Funciones) != 1 || inf.Funciones[0].Firma == nil {
		t.Fatalf("%+v", inf)
	}
	f := *inf.Funciones[0].Firma
	val := func(s string, tp nucleo.Tipo) nucleo.Valor {
		v, err := nucleo.ParseValor(s, tp)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	preguntado := ""
	tr := nucleo.NuevaTraza(nil, func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		preguntado = p.Texto
		return "1", nil
	})
	k := c.nuevaTarea(context.Background(), nucleo.Peticion{Sesion: "desempate"}, c.sesion("desempate"), tr)
	k.q = &nucleo.Pregunta{}
	amb := &reparar.Ambiguedad{
		Entrada: []nucleo.Valor{val("[3,1]", f.Entradas()[0])},
		Parches: []int{0, 1, 2},
		Salidas: [][]nucleo.Valor{{val("3", f.Res[0])}, {val("1", f.Res[0])}, {val("5", f.Res[0])}},
	}
	j, caso, ok := c.repDesempatar(k, &repEspec{f: f}, amb)
	if !ok || j != 1 || len(caso.Esperado) != 1 {
		t.Errorf("j=%d ok=%v caso=%+v", j, ok, caso)
	}
	if !strings.Contains(preguntado, "Hay 3 arreglos") || strings.Contains(preguntado, "dos") {
		t.Errorf("pregunta: %q", preguntado)
	}
}

// Regression (review): a stored function without DSL (from a recipe) is the EspReferencia oracle, built in
// the sandbox: "comparo con «Maximo», que ya conozco".
func TestRepararReferenciaSinDSL(t *testing.T) {
	c := cerebroConGo(t)
	ref := "package solucion\n\nfunc Maximo(xs []int) int {\n\tm := xs[0]\n\tfor _, x := range xs[1:] {\n\t\tif x > m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n"
	inf, _ := analisis.Analizar(ref)
	if len(inf.Funciones) != 1 || inf.Funciones[0].Firma == nil {
		t.Fatalf("%+v", inf)
	}
	nombre, err := c.mem.GuardarFuncion(nucleo.FuncionAprendida{Nombre: "Maximo", Firma: *inf.Funciones[0].Firma, Codigo: ref,
		Descripcion: "devuelve el máximo de una lista de números", Conceptos: []string{"maximo", "lista", "numeros"},
		Palabras: []string{"maximo", "lista", "numero"}, Nivel: nucleo.NivelEntendido, Origen: "receta"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("testdata", "rotos", "max_mal.go"))
	if err != nil {
		t.Fatal(err)
	}
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "debería dar el máximo pero con [3,1,2] da 1", Codigo: string(b)})
	if !strings.Contains(strings.Join(r.Justificacion, "\n"), "Comparé con «"+nombre+"»") {
		t.Errorf("no usó «%s» como referencia: %s [%s]\n%s\n%s", nombre, r.Texto, r.Nivel, strings.Join(r.Justificacion, "\n"), titulos(tr))
	}
	referencia := false
	for _, cp := range r.Comprobaciones {
		if cp.Nivel == nucleo.NivelEntendido {
			referencia = true
		}
	}
	if !r.Exito || !strings.Contains(r.Codigo, "x > m") || !referencia {
		t.Errorf("%s [%s] %+v\n%s\n%s", r.Texto, r.Nivel, r.Comprobaciones, r.Codigo, titulos(tr))
	}
}

const tamConArchivo = "package solucion\n\nimport \"os\"\n\nfunc Tam(n int) int {\n\tb, _ := os.ReadFile(\"x\")\n\treturn n + len(b)\n}\n"

// Regression (review): reparar honours p.Confirmado: unconfirmed it returns the risks (and the code the
// dialog sends back), confirmed it tests the function with PermisoUsuario.
func TestRepararConfirmado(t *testing.T) {
	c := cerebroConGo(t)
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "arréglalo: con 3 debería dar 3", Codigo: tamConArchivo})
	if len(r.Riesgos) == 0 || r.Codigo == "" || !contiene(r, "necesito tu permiso") || contiene(r, "Pulsa «Ejecutar»") || r.Nivel != nucleo.NivelSinComprobar {
		t.Errorf("sin permiso: %s [%s] %+v\n%s\n%s", r.Texto, r.Nivel, r.Riesgos, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	r, tr = pedir(t, c, nucleo.Peticion{Texto: "arréglalo: con 3 debería dar 3", Codigo: tamConArchivo, Confirmado: true})
	if !r.Exito || r.Nivel != nucleo.NivelTusEjemplos || len(r.Riesgos) != 0 || !strings.Contains(titulos(tr), "Me diste permiso") {
		t.Errorf("con permiso: %s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	// confirmed but failing: the failure is shown honestly (no automatic repair of such code yet)
	r, tr = pedir(t, c, nucleo.Peticion{Texto: "arréglalo: con 3 debería dar 4", Codigo: tamConArchivo, Confirmado: true})
	if r.Exito || r.Nivel != nucleo.NivelFallo || !contiene(r, "Lo probé con tu permiso y falla") {
		t.Errorf("con permiso y fallo: %s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
}

// Regression (review): reparar of a whole program reads "con entrada 5" from the text, and without input
// it does not say "Ahora compila" when nothing changed.
func TestRepararProgramaConEntrada(t *testing.T) {
	c := cerebroConGo(t)
	codigo := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tvar n int\n\tfmt.Scan(&n)\n\tfmt.Println(10 / n)\n}\n"
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "arregla esto con entrada 5", Codigo: codigo})
	if !r.Exito || r.Nivel != nucleo.NivelSinFallos || !contiene(r, "«2»") {
		t.Errorf("con entrada: %s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	r, tr = pedir(t, c, nucleo.Peticion{Texto: "arregla esto", Codigo: codigo})
	if r.Nivel != nucleo.NivelFallo || strings.Contains(r.Texto, "Ahora compila") || !strings.Contains(r.Texto, "sin entrada") || !contiene(r, "lee datos") {
		t.Errorf("sin entrada: %s [%s] %+v\n%s\n%s", r.Texto, r.Nivel, r.Comprobaciones, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
}

// Regression (review): a failed run quotes the error (the last stderr line), not the input prompt.
func TestRepResultadoProgramaUltimaLinea(t *testing.T) {
	cp, frase := repResultadoPrograma(nucleo.Ejecucion{Codigo: 1, ErrSalida: "Escribe un número entero (n):\nNo entiendo n: no hay más datos\n"}, "")
	if cp != nil || !strings.Contains(frase, "«No entiendo n: no hay más datos»") || strings.Contains(frase, "Escribe un número") {
		t.Errorf("%v %q", cp, frase)
	}
}
