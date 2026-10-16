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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

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
	plan.Bien("") // same title: not printed twice
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
}

func TestLectorResponder(t *testing.T) {
	var b bytes.Buffer
	imp := nuevaImpresora(&b)
	lec := nuevoLector(strings.NewReader("2\n\n13\notra cosa\n"))
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
	if !strings.Contains(b.String(), "? ¿Qué debe dar con [-2]?\n    1) 4\n    2) 0\n    3) otra cosa\n") ||
		!strings.Contains(b.String(), "(Intro = «sí»)") || !strings.Contains(b.String(), "(sin respuesta)") {
		t.Fatalf("salida de las preguntas:\n%s", b.String())
	}
	// A number that is itself an option is taken literally.
	lec = nuevoLector(strings.NewReader("1\n"))
	tr = nucleo.NuevaTraza(imp.Evento, lec.Responder(imp))
	if s, _ := tr.Preguntar(ctx, nucleo.PreguntaUsuario{Opciones: []string{"2", "1"}}); s != "1" {
		t.Fatalf("opción numérica: %q", s)
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
		"Fuente: go.dev — https://go.dev/doc",
		"Lo he guardado en mi memoria como F",
		"Puedes seguir con: ¿Por qué?",
	} {
		if !strings.Contains(s, quiero) {
			t.Errorf("falta %q en:\n%s", quiero, s)
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
	o, err := leerOpciones(nil, io.Discard)
	if err != nil || o.puerto != puertoPorDefecto || o.sinInternet || o.sinNavegador || o.preguntar != "" {
		t.Fatalf("por defecto: %+v %v", o, err)
	}
	o, err = leerOpciones([]string{"-puerto", "0", "-sin-internet", "-sin-navegador", "-preguntar", "hola", "-datos", "/tmp/x"}, io.Discard)
	if err != nil || o.puerto != 0 || !o.sinInternet || !o.sinNavegador || o.preguntar != "hola" || o.datos != "/tmp/x" {
		t.Fatalf("opciones: %+v %v", o, err)
	}
	for _, malos := range [][]string{{"-puerto", "70000"}, {"hola"}, {"-codigo", "x.go"}, {"-nada"}} {
		if _, err := leerOpciones(malos, io.Discard); err == nil {
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
