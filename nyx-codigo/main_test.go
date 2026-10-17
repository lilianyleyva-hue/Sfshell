package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/cerebro"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/servidor"
)

// *cerebro.Cerebro must satisfy servidor.Motor (§11).
var _ servidor.Motor = (*cerebro.Cerebro)(nil)

var clienteLocal = &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}

func estado(t *testing.T, puerto int, token string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(puerto)+"/api/estado", nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("X-Nyx-Token", token)
	}
	resp, err := clienteLocal.Do(req)
	if err != nil {
		t.Fatalf("GET /api/estado: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var e nucleo.EstadoApp
		if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
			t.Fatalf("EstadoApp: %v", err)
		}
		if e.Version != cerebro.Version || e.Internet {
			t.Fatalf("estado inesperado: %+v", e)
		}
	}
	return resp.StatusCode
}

// TestArranqueYParada builds the binary, starts it, checks the token and the instance file, and stops it
// with SIGTERM (§11).
func TestArranqueYParada(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no hay Go instalado")
	}
	if testing.Short() {
		t.Skip("prueba de extremo a extremo")
	}
	bin := filepath.Join(t.TempDir(), "nyx-codigo")
	if out, err := exec.Command(goBin, "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	// Not t.TempDir: the background warm-up may still be writing to the build cache for a moment
	// after the process exits, which would make the automatic cleanup fail.
	datos, err := os.MkdirTemp("", "nyx-codigo-main-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 20 && os.RemoveAll(datos) != nil; i++ {
			time.Sleep(100 * time.Millisecond)
		}
	})
	datos = filepath.Join(datos, "datos")

	cmd := exec.Command(bin, "-sin-navegador", "-sin-internet", "-puerto", "0", "-datos", datos)
	salida, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var errores bytes.Buffer
	cmd.Stderr = &errores
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	terminado := make(chan error, 1)
	parado := false
	t.Cleanup(func() {
		if !parado {
			cmd.Process.Kill()
			<-terminado
		}
	})

	lineas := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(salida)
		for sc.Scan() {
			lineas <- sc.Text()
		}
		close(lineas)
		terminado <- cmd.Wait()
	}()
	reURL := regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/#t=([0-9a-f]{64})`)
	var puerto int
	var token, linea string
	limite := time.After(90 * time.Second)
	for token == "" {
		select {
		case l, ok := <-lineas:
			if !ok {
				t.Fatalf("el programa terminó sin decir su dirección; stderr:\n%s", errores.String())
			}
			if m := reURL.FindStringSubmatch(l); m != nil {
				puerto, _ = strconv.Atoi(m[1])
				token, linea = m[2], l
			}
		case <-limite:
			t.Fatal("el programa no dijo su dirección en 90 s")
		}
	}
	if !strings.HasPrefix(linea, "Nyx Código está en http://127.0.0.1:") || !strings.Contains(linea, "si cierras la pestaña, vuelve a ejecutar nyx-codigo") {
		t.Errorf("mensaje de arranque inesperado: %q", linea)
	}
	go func() {
		for range lineas {
		}
	}()

	if c := estado(t, puerto, token); c != http.StatusOK {
		t.Fatalf("con la clave: %d, quiero 200", c)
	}
	if c := estado(t, puerto, ""); c != http.StatusForbidden {
		t.Fatalf("sin la clave: %d, quiero 403", c)
	}
	if c := estado(t, puerto, strings.Repeat("0", 64)); c != http.StatusForbidden {
		t.Fatalf("con una clave falsa: %d, quiero 403", c)
	}

	// The instance file describes this process and is private.
	rutaInst := filepath.Join(datos, archivoInstancia)
	fi, err := os.Stat(rutaInst)
	if err != nil {
		t.Fatalf("instancia.json: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("instancia.json tiene permisos %v, quiero 0600", fi.Mode().Perm())
	}
	if di, err := os.Stat(datos); err != nil || di.Mode().Perm() != 0o700 {
		t.Errorf("la carpeta de datos debe ser 0700: %v %v", di.Mode().Perm(), err)
	}
	inst, ok := leerInstancia(datos)
	if !ok || inst.Puerto != puerto || inst.Token != token || inst.PID != cmd.Process.Pid {
		t.Fatalf("instancia.json = %+v, quiero puerto %d, pid %d y la clave", inst, puerto, cmd.Process.Pid)
	}

	// A second start with the same data directory finds the first one and exits.
	segundo := exec.Command(bin, "-sin-navegador", "-sin-internet", "-puerto", "0", "-datos", datos)
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	out, err := exec.CommandContext(ctx, segundo.Path, segundo.Args[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("segunda instancia: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ya está abierto en http://127.0.0.1:"+strconv.Itoa(puerto)+"/#t="+token) {
		t.Fatalf("la segunda instancia debía abrir la primera: %q", out)
	}
	if c := estado(t, puerto, token); c != http.StatusOK {
		t.Fatalf("la primera instancia dejó de responder: %d", c)
	}

	// SIGTERM: graceful shutdown, instancia.json removed, tmp/ emptied.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-terminado:
		parado = true
		if err != nil {
			t.Fatalf("salida con error tras SIGTERM: %v; stderr:\n%s", err, errores.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatal("no se cerró en 20 s tras SIGTERM")
	}
	if _, err := os.Stat(rutaInst); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("instancia.json sigue ahí tras SIGTERM (%v)", err)
	}
	if es, err := os.ReadDir(filepath.Join(datos, "tmp")); err == nil && len(es) > 0 {
		t.Errorf("tmp/ no quedó vacía: %d entradas", len(es))
	}
}

func TestImpresoraArbol(t *testing.T) {
	var b bytes.Buffer
	imp := nuevaImpresora(&b)
	tr := nucleo.NuevaTraza(imp.Evento, nil)
	r := tr.Raiz()
	r.Nota("Entiendo: func SumaPares(nums []int) int")
	plan := r.Sub(nucleo.PasoPlan, "Construyo el programa")
	plan.Nota("Boceto: (suma (filtra ? nums))")
	i := plan.Sub(nucleo.PasoIntento, "Pruebo")
	i.Progreso("Probé 100 programas")
	i.Bien("Encontré: la suma de los números pares de nums")
	plan.Bien("") // printed while open: its closing line shows ✔
	c := r.Sub(nucleo.PasoComprobar, "Compruebo")
	c.Mal("Falla con [-1]")
	abierto := r.Sub(nucleo.PasoNota, "nota informativa") // notes are born closed
	_ = abierto
	sinCerrar := r.Sub(nucleo.PasoIntento, "Sigo pensando")
	_ = sinCerrar
	imp.Vaciar()
	quiero := strings.Join([]string{
		"· Entiendo: func SumaPares(nums []int) int",
		"▸ Construyo el programa",
		"  · Boceto: (suma (filtra ? nums))",
		"  ✔ Encontré: la suma de los números pares de nums",
		"✔ Construyo el programa",
		"✘ Falla con [-1]",
		"· nota informativa",
		"▸ Sigo pensando",
	}, "\n") + "\n"
	if b.String() != quiero {
		t.Fatalf("árbol:\n%s\nquiero:\n%s", b.String(), quiero)
	}

	// A parent printed because of its children shows its closing line when it fails or is renamed.
	b.Reset()
	imp = nuevaImpresora(&b)
	tr = nucleo.NuevaTraza(imp.Evento, nil)
	p := tr.Raiz().Sub(nucleo.PasoComprobar, "Compruebo")
	p.Nota("Compila")
	p.Mal("No pasa las pruebas")
	quiero = "▸ Compruebo\n  · Compila\n✘ No pasa las pruebas\n"
	if b.String() != quiero {
		t.Fatalf("árbol:\n%q\nquiero:\n%q", b.String(), quiero)
	}

	// Regression: a parent that ends well with the same title no longer stays "▸" (it looked unfinished).
	b.Reset()
	imp = nuevaImpresora(&b)
	tr = nucleo.NuevaTraza(imp.Evento, nil)
	p = tr.Raiz().Sub(nucleo.PasoComprobar, "Compruebo la receta SumaPares")
	h := p.Sub(nucleo.PasoComprobar, "Caso [1,2]")
	h.Bien("")
	p.Bien("")
	quiero = "▸ Compruebo la receta SumaPares\n  ✔ Caso [1,2]\n✔ Compruebo la receta SumaPares\n"
	if b.String() != quiero {
		t.Fatalf("árbol:\n%q\nquiero:\n%q", b.String(), quiero)
	}
	imp.Vaciar() // nothing left to print
	if b.String() != quiero {
		t.Fatalf("Vaciar repitió líneas:\n%q", b.String())
	}
}

func TestLectorResponder(t *testing.T) {
	var b bytes.Buffer
	imp := nuevaImpresora(&b)
	lec := nuevoLector(strings.NewReader("b\n\n13\notra cosa\n"))
	tr := nucleo.NuevaTraza(imp.Evento, lec.Responder(imp))
	ctx := context.Background()
	casos := []struct {
		p       nucleo.PreguntaUsuario
		quiero  string
		sinResp bool
	}{
		{nucleo.PreguntaUsuario{ID: "p1", Texto: "¿Qué debe dar con [-2]?", Opciones: []string{"4", "0", "otra cosa"}, Libre: true}, "0", false},
		{nucleo.PreguntaUsuario{ID: "p2", Texto: "¿Seguro?", Opciones: []string{"sí", "no"}, PorDefecto: "sí"}, "sí", false},
		{nucleo.PreguntaUsuario{ID: "p3", Texto: "¿Cuántos?", Libre: true}, "13", false},
		{nucleo.PreguntaUsuario{ID: "p4", Texto: "¿Y esto?", Opciones: []string{"1", "2"}, Libre: true}, "otra cosa", false},
		{nucleo.PreguntaUsuario{ID: "p5", Texto: "¿Algo más?", PorDefecto: "no"}, "no", true},
	}
	for _, c := range casos {
		s, err := tr.Preguntar(ctx, c.p)
		if s != c.quiero || errors.Is(err, nucleo.ErrSinRespuesta) != c.sinResp {
			t.Fatalf("%s: %q, %v; quiero %q (sin respuesta: %v)", c.p.ID, s, err, c.quiero, c.sinResp)
		}
	}
	if !strings.Contains(b.String(), "? ¿Qué debe dar con [-2]?\n    A) 4\n    B) 0\n    C) otra cosa\n") ||
		!strings.Contains(b.String(), "(Intro = «sí»)") || !strings.Contains(b.String(), "(sin respuesta)") ||
		!strings.Contains(b.String(), "Escribe la letra de una opción o tu respuesta: b\n") { // piped answers are echoed
		t.Fatalf("salida de las preguntas:\n%s", b.String())
	}
	// A cancelled context gives no answer.
	lec = nuevoLector(&lectorBloqueado{})
	tr = nucleo.NuevaTraza(imp.Evento, lec.Responder(imp))
	cctx, cancelar := context.WithCancel(ctx)
	cancelar()
	if s, err := tr.Preguntar(cctx, nucleo.PreguntaUsuario{PorDefecto: "x"}); s != "x" || !errors.Is(err, nucleo.ErrSinRespuesta) {
		t.Fatalf("contexto cancelado: %q %v", s, err)
	}
}

// preguntarCon asks p once with entrada as stdin and returns the answer and what was printed.
func preguntarCon(t *testing.T, p nucleo.PreguntaUsuario, entrada string) (string, error, string) {
	t.Helper()
	var b bytes.Buffer
	imp := nuevaImpresora(&b)
	lec := nuevoLector(strings.NewReader(entrada))
	s, err := nucleo.NuevaTraza(imp.Evento, lec.Responder(imp)).Preguntar(context.Background(), p)
	return s, err, b.String()
}

// TestLectorOpcionesNumericas is the regression test for options that are numbers: the options are
// labelled A) B) C) (§1), so typing 1 means the value 1 and every option can be chosen by its label.
// Before, with "1) 10 2) 4 3) 1", typing 1 gave «1» and option 1 («10») could never be chosen by number.
func TestLectorOpcionesNumericas(t *testing.T) {
	p := nucleo.PreguntaUsuario{Texto: "¿Qué debe dar Suma([1, 2, 3, 4])?", Opciones: []string{"10", "4", "1", "Otra cosa"}, Libre: true}
	for entrada, quiero := range map[string]string{
		"a\n": "10", "A\n": "10", "b)\n": "4", "(c)\n": "1", "d\n": "Otra cosa",
		"1\n": "1", "4\n": "4", "10\n": "10", "7\n": "7", "otra cosa\n": "Otra cosa",
	} {
		if s, err, _ := preguntarCon(t, p, entrada); s != quiero || err != nil {
			t.Errorf("con %q: %q %v; quiero %q", entrada, s, err, quiero)
		}
	}
	_, _, salida := preguntarCon(t, p, "a\n")
	if !strings.Contains(salida, "    A) 10\n    B) 4\n    C) 1\n    D) Otra cosa\n") || !strings.Contains(salida, "Escribe la letra de una opción o tu respuesta") {
		t.Fatalf("salida:\n%s", salida)
	}

	// Options that are letters: numbered labels, and the letter typed is the value.
	p = nucleo.PreguntaUsuario{Texto: "¿Qué letra?", Opciones: []string{"a", "b"}, Libre: true}
	for entrada, quiero := range map[string]string{"1\n": "a", "2\n": "b", "b\n": "b", "a\n": "a"} {
		if s, _, salida := preguntarCon(t, p, entrada); s != quiero || !strings.Contains(salida, "    1) a\n    2) b\n") {
			t.Errorf("letras, con %q: %q; quiero %q\n%s", entrada, s, quiero, salida)
		}
	}
	// Letters and numbers both clash: no labels, each option is typed as it is.
	p = nucleo.PreguntaUsuario{Texto: "¿Cuál?", Opciones: []string{"a", "1", "b", "2"}, Libre: true}
	if s, _, salida := preguntarCon(t, p, "1\n"); s != "1" || !strings.Contains(salida, "    - a\n    - 1\n") || !strings.Contains(salida, "Escribe una de las opciones o tu respuesta") {
		t.Errorf("sin etiquetas: %q\n%s", s, salida)
	}
	for _, ops := range [][]string{{"10", "4", "1"}, {"sí", "no"}, {"A", "x"}, {"a", "1"}, {"b", "2", "c", "3", "4"}} {
		et := etiquetasOpciones(ops)
		for i, op := range ops {
			for j, e := range et {
				if i != j && strings.EqualFold(op, e) {
					t.Errorf("%v: la etiqueta %s se lee como la opción %q", ops, e, op)
				}
			}
		}
	}

	// A closed question: the start of an option is enough, and anything else is asked again.
	p = nucleo.PreguntaUsuario{Texto: "¿Lo ejecuto en la caja segura?", Opciones: []string{"sí", "no"}, PorDefecto: "no"}
	for entrada, quiero := range map[string]string{"s\n": "sí", "si\n": "sí", "a\n": "sí", "B\n": "no", "No\n": "no", "\n": "no"} {
		if s, err, _ := preguntarCon(t, p, entrada); s != quiero || err != nil {
			t.Errorf("cerrada, con %q: %q %v; quiero %q", entrada, s, err, quiero)
		}
	}
	s, err, salida := preguntarCon(t, p, "1\nquizá\na\n")
	if s != "sí" || err != nil || !strings.Contains(salida, "«1» no es una de las opciones.") || !strings.Contains(salida, "«quizá» no es una de las opciones.") {
		t.Errorf("repregunta: %q %v\n%s", s, err, salida)
	}
	if s, err, _ := preguntarCon(t, p, "x\ny\nz\na\n"); s != "no" || !errors.Is(err, nucleo.ErrSinRespuesta) {
		t.Errorf("tres respuestas que no valen: %q %v", s, err)
	}
}

type lectorBloqueado struct{}

func (lectorBloqueado) Read([]byte) (int, error) { select {} }

func TestImprimirRespuesta(t *testing.T) {
	var b bytes.Buffer
	imprimirRespuesta(&b, nucleo.Respuesta{
		Texto:    "x = 4",
		Entendi:  "Así lo entendí: 2x+3=11",
		Parrafos: []string{"Compruebo: 2·4 + 3 = 11 ✔"},
		Plan:     []string{"Resto 3", "Divido entre 2"},
		Tablero:  &nucleo.Tablero{Celdas: [][]int{{1, 0}, {0, 2}}},
		Tablas: []nucleo.Tabla{{Titulo: "Pruebas", Cabecera: []string{"Entrada", "Obtenido"},
			Filas: [][]string{{"[1]", "1"}, {"[2]", "0"}}, Marcas: []string{"bien", "mal"}}},
		Codigo: "package solucion\n",
		Comprobaciones: []nucleo.Comprobacion{
			{Nivel: nucleo.NivelCalculo, Texto: "Comprobado sustituyendo"},
			{Nivel: nucleo.NivelTusEjemplos, Pasan: 2, Total: 3, Texto: "2/3 de tus ejemplos"},
			{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar"},
			{Nivel: nucleo.NivelFuente, Texto: "Según la documentación de Go (go doc strings.Split)"},
			{Nivel: nucleo.NivelSinFallos, Texto: "123 casos sin errores", Detalle: "semilla 996259"},
			{Nivel: nucleo.NivelPropiedades, Texto: "✔ Cumple las propiedades"},
		},
		Fuentes:     []nucleo.Fuente{{URL: "https://go.dev/doc", Titulo: "go.dev"}},
		Riesgos:     []nucleo.Violacion{{Linea: 3, Que: "usa os/exec"}},
		Aprendido:   "Lo he guardado en mi memoria como F",
		Sugerencias: []string{"¿Por qué?"},
	})
	s := b.String()
	for _, quiero := range []string{
		"x = 4\nAsí lo entendí: 2x+3=11\nCompruebo: 2·4 + 3 = 11 ✔\n",
		"1. Resto 3\n2. Divido entre 2\n",
		"1 .\n. 2\n",
		"✔  [1]",
		"✘  [2]",
		"Código:\npackage solucion\n",
		"! línea 3: usa os/exec",
		"✔ Comprobado sustituyendo\n✘ 2/3 de tus ejemplos\n· Sin comprobar\n",
		// Regression (§4.14.3): fuente and sin_fallos are not checks of the result, so no ✔ (as in the web app).
		"\n· Según la documentación de Go (go doc strings.Split)\n",
		"\n· 123 casos sin errores (semilla 996259)\n",
		"\n✔ Cumple las propiedades\n", // already marked: not marked twice
		"Fuente: go.dev — https://go.dev/doc",
		"Lo he guardado en mi memoria como F",
		"En el navegador puedes seguir con: ¿Por qué?",
	} {
		if !strings.Contains(s, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, s)
		}
	}
}

// TestImprimirRespuestaTerminal is the regression test for web-only texts and double plan numbers in the
// terminal: there is no «No es eso» button and no conversation (each -preguntar is a new session), so the
// sentence is adapted and the follow-ups are offered for the browser; the justification the web app shows
// under «¿Por qué?» is printed; plan steps that carry their numbers (puzles) are not numbered again.
func TestImprimirRespuestaTerminal(t *testing.T) {
	var b bytes.Buffer
	imprimirRespuesta(&b, nucleo.Respuesta{
		Texto:         "`SumaPares` hace esto: la suma de los números pares de nums.",
		Parrafos:      []string{cerebro.FraseSinEjemplos, "Se acabó el tiempo. Pulsa «Pensar más» para darme el doble de tiempo."},
		Plan:          []string{"1. llena la jarra de 5 l → (0, 5)", "2. vierte la jarra de 5 l en la jarra de 3 l → (3, 2)"},
		Justificacion: []string{"1. llena la jarra de 5 l → (0, 5)", "Busqué el camino más corto (BFS)"},
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Texto: "47/47 según lo que entendí"}},
		Sugerencias:    []string{"Pruébala con otros datos", "Explícala"},
	})
	s := b.String()
	for _, malo := range []string{"pulsa «No es eso»", "Pulsa «Pensar más»", "1. 1.", "2. 2.", "\nPuedes seguir con"} {
		if strings.Contains(s, malo) {
			t.Errorf("sobra %q en:\n%s", malo, s)
		}
	}
	for _, quiero := range []string{
		"comprobé que el código hace lo que entendí. Si no es eso, vuelve a preguntar con otras palabras o con ejemplos (entrada -> salida).\n",
		"En el navegador, «Pensar más» me da el doble de tiempo.",
		"\n1. llena la jarra de 5 l → (0, 5)\n2. vierte la jarra de 5 l en la jarra de 3 l → (3, 2)\n",
		"\n¿Por qué?\n  Busqué el camino más corto (BFS)\n", // the step already shown in the plan is not repeated
		"En el navegador puedes seguir con: Pruébala con otros datos · Explícala\n",
	} {
		if !strings.Contains(s, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, s)
		}
	}
	if strings.Count(s, "llena la jarra") != 1 {
		t.Errorf("el paso del plan sale repetido:\n%s", s)
	}
	// Every sentence of cerebro that names a web-only button is adapted.
	for _, frase := range []string{cerebro.FraseSinEjemplos, cerebro.FraseSinEjemplosNiAcaso} {
		if paraTerminal.Replace(frase) == frase {
			t.Errorf("no se adapta: %q", frase)
		}
	}
}

func TestCarpetaDatos(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	dir, err := carpetaDatos("")
	if err != nil || dir != filepath.Join(base, "nyx-codigo") {
		t.Fatalf("carpetaDatos = %q, %v", dir, err)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("permisos: %v %v", fi.Mode().Perm(), err)
	}
	t.Setenv("XDG_DATA_HOME", "relativo") // not absolute: ignored, as the XDG spec says
	t.Setenv("HOME", base)
	if dir, _ := carpetaDatos(""); dir != filepath.Join(base, ".local", "share", "nyx-codigo") {
		t.Fatalf("sin XDG_DATA_HOME válido: %q", dir)
	}
	propia := filepath.Join(base, "otra")
	if dir, _ := carpetaDatos(propia); dir != propia {
		t.Fatalf("-datos: %q", dir)
	}
	if !esCarpetaNyx(propia) {
		t.Fatal("una carpeta nueva debe quedar marcada como de Nyx Código")
	}
	if dir, err := carpetaDatos(propia); dir != propia || err != nil { // a Nyx folder is accepted again
		t.Fatalf("otra vez: %q %v", dir, err)
	}
	vacia := filepath.Join(base, "vacia")
	os.Mkdir(vacia, 0o755)
	if dir, err := carpetaDatos(vacia); dir != vacia || err != nil || !esCarpetaNyx(vacia) {
		t.Fatalf("carpeta vacía: %q %v", dir, err)
	}
}

// TestCarpetaDatosAjena is the regression test for "-datos ~": a folder with other files is refused, and
// neither its permissions nor its tmp/ subfolder are touched.
func TestCarpetaDatosAjena(t *testing.T) {
	casa := filepath.Join(t.TempDir(), "casa")
	if err := os.MkdirAll(filepath.Join(casa, "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(casa, 0o755)
	notas := filepath.Join(casa, "tmp", "notas.txt")
	os.WriteFile(notas, []byte("mis notas"), 0o644)
	os.WriteFile(filepath.Join(casa, "version"), []byte("v2.0 de mi proyecto\n"), 0o644) // not memoria's

	if _, err := carpetaDatos(casa); err == nil || !strings.Contains(err.Error(), "ya tiene otros archivos") {
		t.Fatalf("carpetaDatos de una carpeta ajena: %v", err)
	}
	var salErr bytes.Buffer
	if c := correr([]string{"-sin-internet", "-datos", casa, "-preguntar", "resuelve 2x+3=11"}, strings.NewReader(""), io.Discard, &salErr); c != 1 ||
		!strings.Contains(salErr.String(), "No la uso para no tocar tus archivos") {
		t.Fatalf("modo terminal con una carpeta ajena: %d %q", c, salErr.String())
	}
	if c := correr([]string{"-sin-internet", "-sin-navegador", "-puerto", "0", "-datos", casa}, strings.NewReader(""), io.Discard, io.Discard); c != 1 {
		t.Fatalf("servidor con una carpeta ajena: %d", c)
	}
	if b, err := os.ReadFile(notas); err != nil || string(b) != "mis notas" {
		t.Fatalf("tmp/notas.txt: %q %v", b, err)
	}
	if fi, _ := os.Stat(casa); fi == nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("cambió los permisos de la carpeta: %v", fi.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(casa, archivoRegistro)); err == nil {
		t.Fatal("escribió registro.log en una carpeta ajena")
	}
}

// TestCarpetaOcupada is the regression test for lost memory updates: while another process (a running
// server or a terminal question) uses the data folder, the terminal mode and a second server refuse to
// open memoria on it.
func TestCarpetaOcupada(t *testing.T) {
	datos, err := carpetaDatos(filepath.Join(t.TempDir(), "datos"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := bloquearDatos(datos)
	if err != nil {
		t.Fatal(err)
	}
	if g, err := bloquearDatos(datos); !errors.Is(err, errCarpetaOcupada) {
		if g != nil {
			g.Close()
		}
		t.Fatalf("segundo bloqueo: %v", err)
	}

	// Terminal question while a server runs: refused, with the server's address.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/estado" && r.Header.Get("X-Nyx-Token") == "clave" {
			w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	puerto := srv.Listener.Addr().(*net.TCPAddr).Port
	otra := instancia{Puerto: puerto, Token: "clave", PID: os.Getppid()}
	if err := escribirInstancia(datos, otra); err != nil {
		t.Fatal(err)
	}
	var sal, salErr bytes.Buffer
	c := correr([]string{"-sin-internet", "-datos", datos, "-preguntar", "resuelve 2x+3=11"}, strings.NewReader(""), &sal, &salErr)
	if c != 1 || !strings.Contains(salErr.String(), "ya está abierto con esta carpeta de datos en "+otra.url()) {
		t.Fatalf("modo terminal con el servidor abierto: %d\n%s\n%s", c, sal.String(), salErr.String())
	}
	if _, err := os.Stat(filepath.Join(datos, "aprendizaje.json")); err == nil {
		t.Fatal("el modo terminal abrió la memoria de otro proceso")
	}
	// A second server opens the running one instead.
	sal.Reset()
	if c := correr([]string{"-sin-internet", "-sin-navegador", "-puerto", "0", "-datos", datos}, strings.NewReader(""), &sal, io.Discard); c != 0 ||
		!strings.Contains(sal.String(), "ya está abierto en "+otra.url()) {
		t.Fatalf("segundo servidor: %d %q", c, sal.String())
	}

	// Locked without a live server (a terminal question): both modes refuse.
	os.Remove(filepath.Join(datos, archivoInstancia))
	antes := esperaOcupada
	esperaOcupada = 300 * time.Millisecond
	defer func() { esperaOcupada = antes }()
	salErr.Reset()
	if c := correr([]string{"-sin-internet", "-datos", datos, "-preguntar", "resuelve 2x+3=11"}, strings.NewReader(""), io.Discard, &salErr); c != 1 ||
		!strings.Contains(salErr.String(), "Otro Nyx Código está usando esta carpeta de datos") {
		t.Fatalf("modo terminal con la carpeta ocupada: %d %q", c, salErr.String())
	}
	salErr.Reset()
	if c := correr([]string{"-sin-internet", "-sin-navegador", "-puerto", "0", "-datos", datos}, strings.NewReader(""), io.Discard, &salErr); c != 1 ||
		!strings.Contains(salErr.String(), "Otro Nyx Código está usando esta carpeta de datos") {
		t.Fatalf("servidor con la carpeta ocupada: %d %q", c, salErr.String())
	}

	// Free again: the lock can be taken.
	f.Close()
	g, err := bloquearDatos(datos)
	if err != nil {
		t.Fatalf("tras soltarlo: %v", err)
	}
	g.Close()
}

func TestInstanciaArchivo(t *testing.T) {
	dir := t.TempDir()
	if _, ok := leerInstancia(dir); ok {
		t.Fatal("no hay archivo todavía")
	}
	yo := instancia{Puerto: 8765, Token: strings.Repeat("ab", 32), PID: os.Getpid()}
	if err := escribirInstancia(dir, yo); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, archivoInstancia)); fi == nil || fi.Mode().Perm() != 0o600 {
		t.Fatal("instancia.json debe tener permisos 0600")
	}
	if i, ok := leerInstancia(dir); !ok || i != yo {
		t.Fatalf("leerInstancia = %+v %v", i, ok)
	}
	if yo.url() != "http://127.0.0.1:8765/#t="+yo.Token {
		t.Fatalf("url: %s", yo.url())
	}
	// Our own pid is never "another instance".
	if _, ok := instanciaViva(dir); ok {
		t.Fatal("la propia instancia no cuenta como otra")
	}
	// Another process's file is left alone.
	otro := yo
	otro.PID = yo.PID + 1
	borrarInstancia(dir, otro)
	if _, ok := leerInstancia(dir); !ok {
		t.Fatal("borró el archivo de otra instancia")
	}
	borrarInstancia(dir, yo)
	if _, ok := leerInstancia(dir); ok {
		t.Fatal("no borró su propio archivo")
	}
	if !pidVivo(os.Getpid()) || pidVivo(0) || pidVivo(-1) {
		t.Fatal("pidVivo")
	}
	// A dead pid is not an instance.
	muerto := exec.Command("true")
	if err := muerto.Run(); err == nil {
		escribirInstancia(dir, instancia{Puerto: 1, Token: "x", PID: muerto.Process.Pid})
		if _, ok := instanciaViva(dir); ok {
			t.Fatal("un proceso muerto no es una instancia viva")
		}
	}
}

func TestEscucharPuertoOcupado(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupado.Close()
	p := ocupado.Addr().(*net.TCPAddr).Port
	ln, puerto, otro, err := escuchar(p)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if !otro || puerto == p || puerto == 0 {
		t.Fatalf("escuchar(%d) = %d, otro=%v", p, puerto, otro)
	}
	ln2, puerto2, otro2, err := escuchar(0)
	if err != nil {
		t.Fatal(err)
	}
	ln2.Close()
	if otro2 || puerto2 == 0 {
		t.Fatalf("escuchar(0) = %d, otro=%v", puerto2, otro2)
	}
}

func TestVaciarTmp(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "tmp", "abc", "x"), 0o700)
	os.WriteFile(filepath.Join(dir, "tmp", "f"), []byte("x"), 0o600)
	vaciarTmp(dir)
	if es, err := os.ReadDir(filepath.Join(dir, "tmp")); err != nil || len(es) != 0 {
		t.Fatalf("tmp: %v %v", es, err)
	}
}

func TestEscritorio(t *testing.T) {
	s := textoEscritorio("/home/ana/nyx-codigo/nyx-codigo")
	for _, quiero := range []string{"[Desktop Entry]\n", "Exec=/home/ana/nyx-codigo/nyx-codigo\n",
		"Icon=utilities-terminal\n", "Categories=Development;Education;\n", "Name=Nyx Código\n"} {
		if !strings.Contains(s, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, s)
		}
	}
	if c := citarExec("/home/ana/mis programas/nyx-codigo"); c != `"/home/ana/mis programas/nyx-codigo"` {
		t.Errorf("citarExec: %s", c)
	}
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	ruta, err := instalarEscritorio()
	if err != nil || ruta != filepath.Join(base, "applications", "nyx-codigo.desktop") {
		t.Fatalf("instalarEscritorio = %q, %v", ruta, err)
	}
	if b, err := os.ReadFile(ruta); err != nil || !strings.Contains(string(b), "Exec=") {
		t.Fatalf("archivo .desktop: %v", err)
	}
}

func TestOpcionesYVersion(t *testing.T) {
	o, err := leerOpciones(nil, io.Discard, io.Discard)
	if err != nil || o.puerto != puertoPorDefecto || o.sinInternet || o.sinNavegador || o.preguntar != "" {
		t.Fatalf("por defecto: %+v %v", o, err)
	}
	o, err = leerOpciones([]string{"-puerto", "0", "-sin-internet", "-sin-navegador", "-preguntar", "hola", "-datos", "/tmp/x"}, io.Discard, io.Discard)
	if err != nil || o.puerto != 0 || !o.sinInternet || !o.sinNavegador || o.preguntar != "hola" || o.datos != "/tmp/x" {
		t.Fatalf("opciones: %+v %v", o, err)
	}
	for _, malos := range [][]string{{"-puerto", "70000"}, {"hola"}, {"-codigo", "x.go"}, {"-nada"}} {
		if _, err := leerOpciones(malos, io.Discard, io.Discard); err == nil {
			t.Errorf("%v debía fallar", malos)
		}
	}
	var sal bytes.Buffer
	if c := correr([]string{"-version"}, strings.NewReader(""), &sal, io.Discard); c != 0 || !strings.HasPrefix(sal.String(), "Nyx Código "+cerebro.Version) {
		t.Fatalf("-version: %d %q", c, sal.String())
	}
	if c := correr([]string{"-h"}, strings.NewReader(""), io.Discard, io.Discard); c != 0 {
		t.Fatalf("-h: %d", c)
	}
	if c := correr([]string{"sobra"}, strings.NewReader(""), io.Discard, io.Discard); c != 2 {
		t.Fatalf("argumento de más: %d", c)
	}
}

// TestOpcionesEnEspanol is the regression test for the help and the flag errors, which the flag package
// writes in English ("(default 8765)", "invalid value … parse error", "flag provided but not defined").
func TestOpcionesEnEspanol(t *testing.T) {
	ingles := regexp.MustCompile(`default|invalid|flag |parse error|provided|needs an argument|\bint\b|\bstring\b|Usage`)
	var sal, salErr bytes.Buffer
	if c := correr([]string{"-h"}, strings.NewReader(""), &sal, &salErr); c != 0 || salErr.Len() != 0 {
		t.Fatalf("-h: %d %q", c, salErr.String())
	}
	ayuda := sal.String()
	if m := ingles.FindString(ayuda); m != "" {
		t.Errorf("la ayuda tiene %q en inglés:\n%s", m, ayuda)
	}
	for _, quiero := range []string{"Uso: nyx-codigo [opciones]", "-puerto número", "por defecto 8765", "-datos carpeta",
		"-preguntar \"pregunta\"", "-codigo archivo.go", "-sin-internet", "-sin-navegador", "-instalar", "-version"} {
		if !strings.Contains(ayuda, quiero) {
			t.Errorf("la ayuda no dice %q:\n%s", quiero, ayuda)
		}
	}
	for _, c := range []struct {
		args   []string
		quiero string
	}{
		{[]string{"-puerto", "abc"}, "«abc» no es un puerto: usa un número entre 0 y 65535."},
		{[]string{"-nada"}, "No conozco la opción -nada."},
		{[]string{"-puerto"}, "A la opción -puerto le falta su valor: escribe -puerto número."},
		{[]string{"-datos"}, "A la opción -datos le falta su valor: escribe -datos carpeta."},
		{[]string{"-sin-internet=quizá"}, "La opción -sin-internet no lleva valor: escribe solo -sin-internet."},
		{[]string{"---x"}, "No entiendo «---x»."},
		{[]string{"-puerto", "-1"}, "El puerto -1 no vale"},
	} {
		sal.Reset()
		salErr.Reset()
		if code := correr(c.args, strings.NewReader(""), &sal, &salErr); code != 2 || sal.Len() != 0 {
			t.Errorf("%v: salida %d, %q", c.args, code, sal.String())
		}
		e := salErr.String()
		if !strings.Contains(e, c.quiero) || !strings.Contains(e, "Escribe «nyx-codigo -h» para ver las opciones.") {
			t.Errorf("%v: %q; quiero %q", c.args, e, c.quiero)
		}
		if m := ingles.FindString(e); m != "" {
			t.Errorf("%v: %q en inglés: %q", c.args, m, e)
		}
	}
}

// TestPreguntaVacia is the regression test for -preguntar "": it is a mistake, not "no question" (which
// started the server and opened the browser, and blocked a script that ran -preguntar "$Q").
func TestPreguntaVacia(t *testing.T) {
	datos := filepath.Join(t.TempDir(), "datos")
	for _, q := range []string{"", "   "} {
		hecho := make(chan int, 1)
		var sal, salErr bytes.Buffer
		go func() {
			hecho <- correr([]string{"-sin-internet", "-sin-navegador", "-puerto", "0", "-datos", datos, "-preguntar", q}, strings.NewReader(""), &sal, &salErr)
		}()
		select {
		case c := <-hecho:
			if c != 2 || !strings.Contains(salErr.String(), "La pregunta está vacía") || strings.Contains(sal.String(), "Nyx Código está en") {
				t.Fatalf("-preguntar %q: %d\n%s\n%s", q, c, sal.String(), salErr.String())
			}
		case <-time.After(20 * time.Second):
			t.Fatalf("-preguntar %q arrancó el servidor", q)
		}
	}
	if _, err := os.Stat(datos); err == nil {
		t.Fatal("con una pregunta vacía no debe crear la carpeta de datos")
	}
	if _, err := leerOpciones([]string{"-preguntar", "hola", "-codigo", ""}, io.Discard, io.Discard); err == nil {
		t.Fatal("-codigo \"\" debía fallar")
	}
}

// TestLeerCodigo is the regression test for -codigo without a size limit (/dev/zero filled the memory):
// it reads at most 1 MiB, like the web app's request bodies, and says what is wrong in Spanish.
func TestLeerCodigo(t *testing.T) {
	dir := t.TempDir()
	bueno := filepath.Join(dir, "bueno.go")
	os.WriteFile(bueno, []byte("package main\n"), 0o644)
	if s, err := leerCodigo(bueno); err != nil || s != "package main\n" {
		t.Fatalf("archivo bueno: %q %v", s, err)
	}
	grande := filepath.Join(dir, "grande.go")
	os.WriteFile(grande, bytes.Repeat([]byte("a"), maxCodigo+1), 0o644)
	justo := filepath.Join(dir, "justo.go")
	os.WriteFile(justo, bytes.Repeat([]byte("a"), maxCodigo), 0o644)
	binario := filepath.Join(dir, "binario")
	os.WriteFile(binario, []byte{0xff, 0xfe, 0x00}, 0o644)
	for ruta, quiero := range map[string]string{
		grande: "es demasiado grande (el máximo es 1 MiB)", dir: "es una carpeta, no un archivo",
		binario: "no es un archivo de texto", filepath.Join(dir, "no.go"): "no existe",
	} {
		if _, err := leerCodigo(ruta); err == nil || motivoError(err) != quiero {
			t.Errorf("%s: %v; quiero %q", filepath.Base(ruta), err, quiero)
		}
	}
	if _, err := leerCodigo(justo); err != nil {
		t.Errorf("1 MiB justo: %v", err)
	}
	if _, err := os.Stat("/dev/zero"); err == nil {
		inicio := time.Now()
		if _, err := leerCodigo("/dev/zero"); !errors.Is(err, errCodigoGrande) {
			t.Errorf("/dev/zero: %v", err)
		}
		if d := time.Since(inicio); d > 10*time.Second {
			t.Errorf("/dev/zero tardó %v", d)
		}
	}
	// Through correr: the message, and nothing is created.
	datos := filepath.Join(dir, "datos")
	var salErr bytes.Buffer
	if c := correr([]string{"-sin-internet", "-datos", datos, "-preguntar", "explica esto", "-codigo", grande}, strings.NewReader(""), io.Discard, &salErr); c != 1 ||
		salErr.String() != "No puedo leer "+grande+": es demasiado grande (el máximo es 1 MiB).\n" {
		t.Fatalf("correr: %d %q", c, salErr.String())
	}
	if _, err := os.Stat(datos); err == nil {
		t.Fatal("con un archivo que no vale no debe crear la carpeta de datos")
	}
}

// TestModoPregunta runs the CLI mode in-process on a reasoning question (no Go build needed).
func TestModoPregunta(t *testing.T) {
	if testing.Short() {
		t.Skip("abre la memoria y la caja de arena")
	}
	datos := t.TempDir()
	var sal, salErr bytes.Buffer
	c := correr([]string{"-sin-internet", "-datos", datos, "-preguntar", "resuelve 2x+3=11"}, strings.NewReader(""), &sal, &salErr)
	if c != 0 {
		t.Fatalf("salida %d\n%s\n%s", c, sal.String(), salErr.String())
	}
	s := sal.String()
	if !strings.Contains(s, "x = 4") || !strings.Contains(s, "✔") || !strings.Contains(s, strings.Repeat("─", 40)) {
		t.Fatalf("respuesta:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(datos, archivoRegistro)); err != nil {
		t.Errorf("registro.log: %v", err)
	}
	if _, err := os.Stat(filepath.Join(datos, archivoInstancia)); err == nil {
		t.Error("el modo terminal no debe escribir instancia.json")
	}
}

// TestPeticionConfirmable is the regression test for the terminal mode's risk confirmation: an answer
// with risks gives a request to resend with Confirmado, never for grave risks or twice.
func TestPeticionConfirmable(t *testing.T) {
	p := nucleo.Peticion{Tarea: "cli-1", Sesion: "cli", Texto: "ejecuta esto", Codigo: "package main\n"}
	if _, ok := peticionConfirmable(p, nucleo.Respuesta{}); ok {
		t.Fatal("sin riesgos no hay nada que confirmar")
	}
	r := nucleo.Respuesta{Riesgos: []nucleo.Violacion{{Linea: 4, Que: "lee archivos"}}, Codigo: "package main\n"}
	q, ok := peticionConfirmable(p, r)
	if !ok || !q.Confirmado || q.Texto != p.Texto || q.Codigo != p.Codigo || q.Sesion != "cli" {
		t.Fatalf("con riesgos: %+v %v", q, ok)
	}
	if _, ok := peticionConfirmable(q, r); ok {
		t.Fatal("ya confirmada: no se pregunta otra vez")
	}
	grave := nucleo.Respuesta{Riesgos: []nucleo.Violacion{{Que: "usa cgo", Grave: true}}, Codigo: "x"}
	if _, ok := peticionConfirmable(p, grave); ok {
		t.Fatal("un riesgo grave nunca se ejecuta")
	}
	// Without code in the request, the answer's code is run (as the web app's dialog does).
	q, ok = peticionConfirmable(nucleo.Peticion{Sesion: "cli", Texto: "ejecútala"}, r)
	if !ok || q.Accion != "ejecutar" || q.Codigo != r.Codigo || !q.Confirmado {
		t.Fatalf("sin código en la petición: %+v %v", q, ok)
	}
	if _, ok := peticionConfirmable(nucleo.Peticion{Texto: "x"}, nucleo.Respuesta{Riesgos: r.Riesgos}); ok {
		t.Fatal("sin código no hay nada que ejecutar")
	}
	for s, quiero := range map[string]bool{"sí": true, " Si ": true, "s": true, "no": false, "": false, "2": false} {
		if esSi(s) != quiero {
			t.Errorf("esSi(%q) = %v", s, !quiero)
		}
	}
}

// TestModoPreguntaConfirma runs risky code in the terminal mode: it asks, and with "a" (sí) runs it.
func TestModoPreguntaConfirma(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no hay Go instalado")
	}
	if testing.Short() {
		t.Skip("compila código en la caja de arena")
	}
	datos, err := carpetaDatos(filepath.Join(t.TempDir(), "datos"))
	if err != nil {
		t.Fatal(err)
	}
	compartirCache(datos)
	lee := filepath.Join(t.TempDir(), "lee.go")
	os.WriteFile(lee, []byte("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\t_, err := os.ReadFile(\"/no/existe/nada.txt\")\n\tfmt.Println(\"leído:\", err != nil)\n}\n"), 0o644)
	run := func(entrada string) (int, string) {
		var sal, salErr bytes.Buffer
		c := correr([]string{"-sin-internet", "-datos", datos, "-preguntar", "ejecuta esto", "-codigo", lee}, strings.NewReader(entrada), &sal, &salErr)
		return c, sal.String() + salErr.String()
	}
	c, s := run("b\n")
	if c != 1 || !strings.Contains(s, "? ¿Lo ejecuto en la caja segura?") || !strings.Contains(s, "No lo ejecuto.") || strings.Contains(s, "leído: true") {
		t.Fatalf("con «no»: %d\n%s", c, s)
	}
	c, s = run("a\n")
	if c != 0 || !strings.Contains(s, "? ¿Lo ejecuto en la caja segura?") || !strings.Contains(s, "leído: true") {
		t.Fatalf("con «sí»: %d\n%s", c, s)
	}
}

// TestReadme is the regression test for the README's commands: "go build" alone makes a binary named
// after the module (nyxcodigo), so the README must build with -o nyx-codigo; it must say how to close an
// instance started from the menu, and that requests may be searched on DuckDuckGo.
func TestReadme(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, quiero := range []string{"go build -o nyx-codigo && ./nyx-codigo", "pkill nyx-codigo", "DuckDuckGo", "-sin-internet", "¿Lo ejecuto en la caja segura?"} {
		if !strings.Contains(s, quiero) {
			t.Errorf("README.md no dice %q", quiero)
		}
	}
	if regexp.MustCompile(`go build( &&|\n|$)`).MatchString(s) {
		t.Error("README.md tiene un «go build» sin -o nyx-codigo")
	}
	// Regression: the privacy section must not name a site the app never contacts. Wikipedia is only read
	// if cerebro calls saber's Web.Wikipedia.
	if strings.Contains(s, "Wikipedia") && !cerebroLlama(t, ".Wikipedia(") {
		t.Error("README.md dice que lee Wikipedia, pero cerebro nunca llama a Web.Wikipedia")
	}
	if strings.Contains(s, "Puedes seguir con") || !strings.Contains(s, "contesta `a` (sí) o `b` (no)") {
		t.Error("README.md debe explicar las respuestas con letra del modo terminal")
	}
}

// cerebroLlama reports whether a non-test file of internal/cerebro contains llamada.
func cerebroLlama(t *testing.T, llamada string) bool {
	t.Helper()
	rutas, err := filepath.Glob(filepath.Join("internal", "cerebro", "*.go"))
	if err != nil || len(rutas) == 0 {
		t.Fatalf("no encuentro internal/cerebro: %v", err)
	}
	for _, r := range rutas {
		if strings.HasSuffix(r, "_test.go") {
			continue
		}
		if b, err := os.ReadFile(r); err == nil && strings.Contains(string(b), llamada) {
			return true
		}
	}
	return false
}

func TestRegistroRota(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), archivoRegistro)
	r := abrirRegistro(ruta)
	if r == nil {
		t.Fatal("abrirRegistro")
	}
	defer r.Cerrar()
	r.tam = maxRegistro - 3
	r.Write([]byte("hola\n"))
	if b, err := os.ReadFile(ruta); err != nil || string(b) != "hola\n" {
		t.Fatalf("tras rotar: %q %v", b, err)
	}
	if _, err := os.Stat(ruta + ".1"); err != nil {
		t.Fatalf("copia vieja: %v", err)
	}
}

// compartirCache points <datos>/cache/go-build at a build cache shared between test runs (a cold cache
// takes long).
func compartirCache(datos string) {
	if c, err := os.UserCacheDir(); err == nil {
		comun := filepath.Join(c, "nyx-codigo-pruebas", "go-build")
		if os.MkdirAll(comun, 0o700) == nil && os.MkdirAll(filepath.Join(datos, "cache"), 0o700) == nil {
			os.Symlink(comun, filepath.Join(datos, "cache", "go-build"))
		}
	}
}

// romperAprendizaje puts a non-empty folder where memoria writes its learning counters, so that writing
// them fails (rename onto a non-empty directory).
func romperAprendizaje(t *testing.T, datos string) {
	t.Helper()
	ruta := filepath.Join(datos, "aprendizaje.json")
	if err := os.RemoveAll(ruta); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ruta, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
}

// TestMotorCerrarError: cerrar returns what memoria could not save, also when it is called again
// (memoria.Cerrar itself returns nil the second time).
func TestMotorCerrarError(t *testing.T) {
	if testing.Short() {
		t.Skip("abre la memoria y la caja de arena")
	}
	datos, err := carpetaDatos(filepath.Join(t.TempDir(), "datos"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := construir(opciones{sinInternet: true}, datos, arenero.Config{DirDatos: t.TempDir(), DirCache: filepath.Join(t.TempDir(), "cache")})
	if err != nil {
		t.Fatal(err)
	}
	romperAprendizaje(t, datos)
	m.mem.Contador().Exito("estrategia:prueba", true)
	err = m.cerrar()
	if err == nil {
		t.Fatal("cerrar no avisó de que no pudo guardar aprendizaje.json")
	}
	if txt := textoErrorCierre(err); txt != "no pude guardar aprendizaje.json (ya hay algo con ese nombre que no puedo reemplazar)" &&
		txt != "no pude guardar aprendizaje.json (en su lugar hay una carpeta)" {
		t.Errorf("textoErrorCierre = %q", txt)
	}
	if err2 := m.cerrar(); err2 != err {
		t.Errorf("la segunda llamada devolvió %v", err2)
	}
}

// TestCierreAvisaSiNoGuarda is the regression test for the shutdown message (§0): when memoria cannot
// write its pending counters, the server must not say «Tu memoria está guardada» nor exit 0.
func TestCierreAvisaSiNoGuarda(t *testing.T) {
	if testing.Short() {
		t.Skip("arranca el servidor")
	}
	// Not t.TempDir: the background warm-up may still be writing to the build cache for a moment.
	base, err := os.MkdirTemp("", "nyx-codigo-cierre-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 20 && os.RemoveAll(base) != nil; i++ {
			time.Sleep(100 * time.Millisecond)
		}
	})
	datos, err := carpetaDatos(filepath.Join(base, "datos"))
	if err != nil {
		t.Fatal(err)
	}
	compartirCache(datos)

	ctx, parar := context.WithCancel(context.Background())
	defer parar()
	antes := contextoSenales
	contextoSenales = func() (context.Context, context.CancelFunc) { return context.WithCancel(ctx) }
	defer func() { contextoSenales = antes }()

	var sal, salErr bytes.Buffer
	hecho := make(chan int, 1)
	go func() {
		hecho <- correr([]string{"-sin-internet", "-sin-navegador", "-puerto", "0", "-datos", datos}, strings.NewReader(""), &sal, &salErr)
	}()
	var inst instancia
	for limite := time.Now().Add(90 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if i, ok := leerInstancia(datos); ok {
			inst = i
			break
		}
		select {
		case c := <-hecho:
			t.Fatalf("el servidor terminó antes de tiempo: %d\n%s\n%s", c, sal.String(), salErr.String())
		default:
		}
		if time.Now().After(limite) {
			parar()
			<-hecho
			t.Fatal("el servidor no escribió instancia.json en 90 s")
		}
	}

	// memoria is open; from now on its learning counters cannot be written.
	romperAprendizaje(t, datos)
	base1 := "http://127.0.0.1:" + strconv.Itoa(inst.Puerto)
	req, _ := http.NewRequest(http.MethodPost, base1+"/api/preguntar", strings.NewReader(`{"sesion":"abc","texto":"resuelve 2x+3=11"}`))
	req.Header.Set("X-Nyx-Token", inst.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := clienteLocal.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var tarea struct {
		Tarea string `json:"tarea"`
	}
	json.NewDecoder(resp.Body).Decode(&tarea)
	resp.Body.Close()
	if tarea.Tarea == "" {
		t.Fatalf("POST /api/preguntar: %d", resp.StatusCode)
	}
	flujo, err := (&http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 60 * time.Second}).Get(base1 + "/api/tareas/" + tarea.Tarea + "/eventos?t=" + inst.Token)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(flujo.Body)
	fin := false
	for !fin && sc.Scan() {
		fin = sc.Text() == "event: fin"
	}
	flujo.Body.Close()
	if !fin {
		t.Fatalf("la tarea no terminó: %v", sc.Err())
	}

	parar() // as SIGTERM does
	select {
	case c := <-hecho:
		if c != 1 {
			t.Errorf("salida %d, quiero 1", c)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no se cerró en 30 s")
	}
	if strings.Contains(sal.String(), "Tu memoria está guardada") {
		t.Errorf("dice que guardó la memoria sin haberlo hecho:\n%s", sal.String())
	}
	if !strings.Contains(salErr.String(), "Nyx Código se ha cerrado, pero no pude guardar aprendizaje.json (") {
		t.Errorf("no avisa de lo que no pudo guardar:\n%s", salErr.String())
	}
	if _, err := os.Stat(filepath.Join(datos, archivoInstancia)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("instancia.json sigue ahí (%v)", err)
	}
}
