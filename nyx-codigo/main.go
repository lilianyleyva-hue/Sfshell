// Command nyx-codigo is Nyx Código: a Spanish web app on 127.0.0.1 that writes, repairs, explains and
// tests Go code and solves maths, logic and puzzle questions, checking every result before showing it.
//
// Flags (§4.16):
//
//	-puerto int       default 8765; if busy, a random free port (0 = always a random one)
//	-datos string     default $XDG_DATA_HOME/nyx-codigo or ~/.local/share/nyx-codigo (0700)
//	-sin-internet     disable all network access
//	-sin-navegador    do not open the browser
//	-preguntar "…"    CLI mode: answer one question, print the trace as an indented tree and the answer, exit
//	-codigo archivo   with -preguntar: the Go file the question is about (additive)
//	-instalar         write the .desktop launcher and exit
//	-version          print the version and exit
//
// Startup: sandbox trampoline, single instance (<datos>/instancia.json), memoria → arenero → saber →
// cerebro → servidor, listen on 127.0.0.1, background warm-up, browser, graceful shutdown.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/cerebro"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/saber"
	"nyxcodigo/internal/servidor"
)

const (
	puertoPorDefecto = 8765
	archivoInstancia = "instancia.json"
	archivoRegistro  = "registro.log"
	maxRegistro      = 5 << 20 // registro.log is rotated at 5 MB (one old copy)
	esperaApagado    = 5 * time.Second
	esperaIndiceCLI  = 3 * time.Second
)

func main() {
	if arenero.EsTrampolin() {
		arenero.Trampolin()
	}
	os.Exit(correr(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// opciones are the parsed flags.
type opciones struct {
	puerto       int
	datos        string
	sinInternet  bool
	sinNavegador bool
	preguntar    string
	codigo       string
	instalar     bool
	version      bool
}

func leerOpciones(args []string, salErr io.Writer) (opciones, error) {
	var o opciones
	fs := flag.NewFlagSet("nyx-codigo", flag.ContinueOnError)
	fs.SetOutput(salErr)
	fs.IntVar(&o.puerto, "puerto", puertoPorDefecto, "puerto local (si está ocupado, uso otro libre; 0 = cualquiera)")
	fs.StringVar(&o.datos, "datos", "", "carpeta de datos (por defecto $XDG_DATA_HOME/nyx-codigo o ~/.local/share/nyx-codigo)")
	fs.BoolVar(&o.sinInternet, "sin-internet", false, "no usar internet para nada")
	fs.BoolVar(&o.sinNavegador, "sin-navegador", false, "no abrir el navegador")
	fs.StringVar(&o.preguntar, "preguntar", "", "modo terminal: responde una pregunta, enseña cómo pensó y termina")
	fs.StringVar(&o.codigo, "codigo", "", "con -preguntar: archivo .go sobre el que preguntas")
	fs.BoolVar(&o.instalar, "instalar", false, "crea el acceso directo del menú de aplicaciones y termina")
	fs.BoolVar(&o.version, "version", false, "muestra la versión y termina")
	fs.Usage = func() {
		fmt.Fprintln(salErr, "Uso: nyx-codigo [opciones]")
		fmt.Fprintln(salErr, "Abre Nyx Código en tu navegador (http://127.0.0.1:8765). Opciones:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(salErr, "No entiendo «%s». Si quieres preguntar algo, usa -preguntar \"tu pregunta\".\n", strings.Join(fs.Args(), " "))
		return o, errors.New("argumentos de más")
	}
	if o.puerto < 0 || o.puerto > 65535 {
		fmt.Fprintf(salErr, "El puerto %d no vale: usa un número entre 0 y 65535.\n", o.puerto)
		return o, errors.New("puerto no válido")
	}
	if o.codigo != "" && o.preguntar == "" {
		fmt.Fprintln(salErr, "-codigo solo sirve junto con -preguntar.")
		return o, errors.New("-codigo sin -preguntar")
	}
	return o, nil
}

// correr is main without os.Exit, so tests can call it.
func correr(args []string, entrada io.Reader, sal, salErr io.Writer) int {
	o, err := leerOpciones(args, salErr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if o.version {
		fmt.Fprintf(sal, "Nyx Código %s (%s, %s/%s)\n", cerebro.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return 0
	}
	if o.instalar {
		ruta, err := instalarEscritorio()
		if err != nil {
			fmt.Fprintf(salErr, "No pude crear el acceso directo: %v\n", err)
			return 1
		}
		fmt.Fprintf(sal, "Listo: he creado %s. Busca «Nyx Código» en el menú de aplicaciones.\n", ruta)
		return 0
	}
	datos, err := carpetaDatos(o.datos)
	if err != nil {
		fmt.Fprintf(salErr, "No puedo usar la carpeta de datos: %v\n", err)
		return 1
	}
	reg := abrirRegistro(filepath.Join(datos, archivoRegistro))
	if reg != nil {
		log.SetOutput(reg)
		defer reg.Cerrar()
	} else {
		log.SetOutput(io.Discard)
	}
	if o.preguntar != "" {
		return modoPregunta(o, datos, entrada, sal, salErr)
	}
	return servir(o, datos, sal, salErr)
}

// ---------------------------------------------------------------------------------------------
// Data directory, log and instance file

// carpetaDatos resolves the data directory and makes sure it exists with mode 0700.
func carpetaDatos(pedida string) (string, error) {
	dir := pedida
	if dir == "" {
		if x := os.Getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
			dir = filepath.Join(x, "nyx-codigo")
		} else {
			casa, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("no sé cuál es tu carpeta personal: %w", err)
			}
			dir = filepath.Join(casa, ".local", "share", "nyx-codigo")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(abs, 0o700); err != nil {
		return "", err
	}
	return abs, nil
}

// registro is the app log (registro.log), rotated at maxRegistro with one old copy.
type registro struct {
	mu   sync.Mutex
	ruta string
	f    *os.File
	tam  int64
}

func abrirRegistro(ruta string) *registro {
	f, err := os.OpenFile(ruta, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	r := &registro{ruta: ruta, f: f}
	if fi, err := f.Stat(); err == nil {
		r.tam = fi.Size()
	}
	return r
}

func (r *registro) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return len(b), nil
	}
	if r.tam+int64(len(b)) > maxRegistro && r.tam > 0 {
		r.f.Close()
		os.Rename(r.ruta, r.ruta+".1")
		f, err := os.OpenFile(r.ruta, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			r.f = nil
			return len(b), nil
		}
		r.f, r.tam = f, 0
	}
	n, err := r.f.Write(b)
	r.tam += int64(n)
	return n, err
}

func (r *registro) Cerrar() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f != nil {
		r.f.Close()
		r.f = nil
	}
}

// instancia is <datos>/instancia.json (0600), removed on exit.
type instancia struct {
	Puerto int    `json:"puerto"`
	Token  string `json:"token"`
	PID    int    `json:"pid"`
}

func (i instancia) url() string {
	return fmt.Sprintf("http://127.0.0.1:%d/#t=%s", i.Puerto, i.Token)
}

func leerInstancia(datos string) (instancia, bool) {
	var i instancia
	b, err := os.ReadFile(filepath.Join(datos, archivoInstancia))
	if err != nil || json.Unmarshal(b, &i) != nil || i.Puerto <= 0 || i.Token == "" {
		return instancia{}, false
	}
	return i, true
}

// escribirInstancia writes the file atomically (temp → fsync → rename) with mode 0600.
func escribirInstancia(datos string, i instancia) error {
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(datos, ".instancia-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(datos, archivoInstancia))
}

// borrarInstancia removes instancia.json only when it still describes this process.
func borrarInstancia(datos string, propia instancia) {
	if i, ok := leerInstancia(datos); ok && (i.PID != propia.PID || i.Token != propia.Token) {
		return
	}
	os.Remove(filepath.Join(datos, archivoInstancia))
}

// pidVivo reports whether a process with that pid exists.
func pidVivo(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// instanciaViva returns the running instance described by instancia.json, if its process is alive and
// its server answers GET /api/estado with the token.
func instanciaViva(datos string) (instancia, bool) {
	i, ok := leerInstancia(datos)
	if !ok || i.PID == os.Getpid() || !pidVivo(i.PID) {
		return instancia{}, false
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/estado", i.Puerto), nil)
	if err != nil {
		return instancia{}, false
	}
	req.Header.Set("X-Nyx-Token", i.Token)
	cli := &http.Client{Transport: &http.Transport{Proxy: nil}} // never through a proxy: it is our own loopback server
	resp, err := cli.Do(req)
	if err != nil {
		return instancia{}, false
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	return i, resp.StatusCode == http.StatusOK
}

// vaciarTmp empties <datos>/tmp (sandbox temp modules).
func vaciarTmp(datos string) {
	dir := filepath.Join(datos, "tmp")
	es, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range es {
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

// nuevoToken returns 32 random bytes in hex.
func nuevoToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// escuchar listens on 127.0.0.1:puerto; when that port is busy it takes a random free one.
func escuchar(puerto int) (net.Listener, int, bool, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(puerto)))
	otro := false
	if err != nil && puerto != 0 {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		otro = true
	}
	if err != nil {
		return nil, 0, false, err
	}
	return ln, ln.Addr().(*net.TCPAddr).Port, otro, nil
}

// abrirNavegador runs xdg-open; it reports false when there is no xdg-open.
func abrirNavegador(url string) bool {
	ruta, err := exec.LookPath("xdg-open")
	if err != nil {
		return false
	}
	cmd := exec.Command(ruta, url)
	if err := cmd.Start(); err != nil {
		log.Printf("xdg-open: %v", err)
		return false
	}
	go cmd.Wait()
	return true
}

// ---------------------------------------------------------------------------------------------
// Server mode

// motor is the app's state shared by both modes.
type motor struct {
	mem *memoria.Almacen
	are *arenero.Arenero // nil when the sandbox could not be created
	web *saber.Web       // nil with -sin-internet
	cer *cerebro.Cerebro
}

// construir opens memoria → arenero → saber → cerebro (§4.16 step 4).
func construir(o opciones, datos string, cfgAre arenero.Config) (*motor, error) {
	mem, err := memoria.Abrir(datos)
	if err != nil {
		return nil, fmt.Errorf("no puedo abrir mi memoria en %s: %w", datos, err)
	}
	m := &motor{mem: mem}
	are, err := arenero.Nuevo(cfgAre)
	if err != nil {
		log.Printf("arenero: %v", err)
	} else {
		m.are = are
	}
	goBin := ""
	var ej nucleo.Ejecutor // stays a nil interface without a sandbox
	if m.are != nil {
		ej = m.are
		goBin = m.are.Estado().GoBin
	}
	if !o.sinInternet {
		m.web = saber.NuevaWeb(saber.Config{DirCache: filepath.Join(datos, "web"), GoBin: goBin})
	}
	m.cer, err = cerebro.Nuevo(cerebro.Config{DirDatos: datos, SinInternet: o.sinInternet},
		cerebro.Dependencias{Ej: ej, Mem: mem, Web: m.web, GoBin: goBin})
	if err != nil {
		m.cerrar()
		return nil, err
	}
	return m, nil
}

// indexar builds (or loads) the GOROOT example index and hands it to cerebro.
func (m *motor) indexar(datos string) {
	if m.are == nil {
		return
	}
	e := m.are.Estado()
	if !e.GoOK || m.are.GoRoot() == "" {
		return
	}
	idx, err := saber.IndexarGOROOT(m.are.GoRoot(), e.GoVersion, datos)
	if err != nil {
		log.Printf("índice de GOROOT: %v", err)
		return
	}
	m.cer.PonerIndice(idx)
}

func (m *motor) cerrar() {
	if err := m.mem.Cerrar(); err != nil {
		log.Printf("memoria.Cerrar: %v", err)
	}
	if m.are != nil {
		m.are.Cerrar()
	}
}

func servir(o opciones, datos string, sal, salErr io.Writer) int {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	// Single instance: if another Nyx Código already serves this data directory, open it instead.
	if i, ok := instanciaViva(datos); ok {
		fmt.Fprintf(sal, "Nyx Código ya está abierto en %s\n", i.url())
		if !o.sinNavegador && !abrirNavegador(i.url()) {
			fmt.Fprintln(sal, "No encuentro xdg-open: abre esa dirección en tu navegador.")
		}
		return 0
	}

	m, err := construir(o, datos, arenero.Config{DirDatos: datos})
	if err != nil {
		fmt.Fprintf(salErr, "Nyx Código no puede arrancar: %v\n", err)
		return 1
	}
	defer m.cerrar()

	ln, puerto, otro, err := escuchar(o.puerto)
	if err != nil {
		fmt.Fprintf(salErr, "No puedo abrir un puerto en 127.0.0.1: %v\n", err)
		return 1
	}
	if otro {
		fmt.Fprintf(sal, "El puerto %d está ocupado; uso el %d.\n", o.puerto, puerto)
	}
	token, err := nuevoToken()
	if err != nil {
		ln.Close()
		fmt.Fprintf(salErr, "No puedo crear la clave de acceso: %v\n", err)
		return 1
	}
	h := servidor.Nuevo(m.cer, servidor.Config{Token: token, Puerto: puerto})
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, ErrorLog: log.Default()}
	errServir := make(chan error, 1)
	go func() { errServir <- srv.Serve(ln) }()

	yo := instancia{Puerto: puerto, Token: token, PID: os.Getpid()}
	if err := escribirInstancia(datos, yo); err != nil {
		log.Printf("instancia.json: %v", err)
	}
	defer borrarInstancia(datos, yo)

	// Background warm-up: build cache, GOROOT examples, internet check.
	fondo, pararFondo := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	enFondo := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if x := recover(); x != nil {
					log.Printf("tarea de fondo: %v", x)
				}
			}()
			f()
		}()
	}
	if m.are != nil {
		enFondo(func() {
			if err := m.are.Calentar(fondo); err != nil && fondo.Err() == nil {
				log.Printf("Calentar: %v", err)
			}
		})
		enFondo(func() { m.indexar(datos) })
	}
	if m.web != nil {
		enFondo(func() { m.web.Disponible(fondo) })
	}

	url := yo.url()
	fmt.Fprintf(sal, "Nyx Código está en %s — si cierras la pestaña, vuelve a ejecutar nyx-codigo\n", url)
	if !o.sinNavegador && !abrirNavegador(url) {
		fmt.Fprintln(sal, "No encuentro xdg-open: abre esa dirección en tu navegador.")
	}
	log.Printf("Nyx Código %s escucha en 127.0.0.1:%d (pid %d)", cerebro.Version, puerto, os.Getpid())

	salida := 0
	select {
	case <-ctx.Done():
	case err := <-errServir:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(salErr, "El servidor se paró: %v\n", err)
			salida = 1
		}
	}
	parar()

	// Graceful shutdown (§4.16 step 9).
	if s, ok := h.(*servidor.Servidor); ok {
		s.Cerrar() // cancels running tasks so their event streams end
	}
	pararFondo()
	c, cancelar := context.WithTimeout(context.Background(), esperaApagado)
	if err := srv.Shutdown(c); err != nil {
		srv.Close()
	}
	cancelar()
	esperar(&wg, 3*time.Second)
	m.cerrar()
	borrarInstancia(datos, yo)
	vaciarTmp(datos)
	fmt.Fprintln(sal, "Nyx Código se ha cerrado. Tu memoria está guardada.")
	return salida
}

// esperar waits for wg at most d.
func esperar(wg *sync.WaitGroup, d time.Duration) {
	hecho := make(chan struct{})
	go func() { wg.Wait(); close(hecho) }()
	select {
	case <-hecho:
	case <-time.After(d):
	}
}

// ---------------------------------------------------------------------------------------------
// CLI mode (-preguntar)

func modoPregunta(o opciones, datos string, entrada io.Reader, sal, salErr io.Writer) int {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	codigo := ""
	if o.codigo != "" {
		b, err := os.ReadFile(o.codigo)
		if err != nil {
			fmt.Fprintf(salErr, "No puedo leer %s: %v\n", o.codigo, err)
			return 1
		}
		codigo = string(b)
	}
	// The sandbox gets its own temp folder, so a running server instance (which owns <datos>/tmp)
	// is not disturbed; the build cache is shared.
	tmpAre, err := os.MkdirTemp("", "nyx-codigo-cli-")
	if err != nil {
		fmt.Fprintf(salErr, "No puedo crear una carpeta temporal: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpAre)
	m, err := construir(o, datos, arenero.Config{DirDatos: tmpAre, DirCache: filepath.Join(datos, "cache", "go-build")})
	if err != nil {
		fmt.Fprintf(salErr, "Nyx Código no puede arrancar: %v\n", err)
		return 1
	}
	defer m.cerrar()

	// The GOROOT index is usually cached on disk; wait a little for it, then carry on without it.
	listo := make(chan struct{})
	go func() {
		defer close(listo)
		defer func() { recover() }()
		m.indexar(datos)
	}()
	select {
	case <-listo:
	case <-time.After(esperaIndiceCLI):
	}

	imp := nuevaImpresora(sal)
	lec := nuevoLector(entrada)
	tr := nucleo.NuevaTraza(imp.Evento, lec.Responder(imp))
	id, _ := nuevoToken()
	p := nucleo.Peticion{Tarea: "cli-" + id[:12], Sesion: "cli", Texto: o.preguntar, Codigo: codigo}
	r := m.cer.Responder(ctx, p, tr)
	imp.Vaciar()
	imprimirRespuesta(sal, r)
	if r.Exito {
		return 0
	}
	return 1
}

// impresora prints trace events as an indented tree: "  "×depth + marker (▸ ✔ ✘ ·) + title.
// An open step is printed when it closes (with its final title), when a child of it is printed,
// before a question, or at the end.
type impresora struct {
	mu    sync.Mutex
	w     io.Writer
	pasos map[int]*pasoCLI
	orden []int
}

type pasoCLI struct {
	id, padre, prof int
	titulo          string
	estado          nucleo.Estado
	impreso         bool
	tituloImpreso   string
}

func nuevaImpresora(w io.Writer) *impresora {
	return &impresora{w: w, pasos: map[int]*pasoCLI{}}
}

func marcador(e nucleo.Estado) string {
	switch e {
	case nucleo.EstadoBien:
		return "✔"
	case nucleo.EstadoMal:
		return "✘"
	case nucleo.EstadoInfo:
		return "·"
	}
	return "▸"
}

func unaLinea(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", " "))
	return s
}

// Evento is the trace's emitir. It never calls back into the trace.
func (im *impresora) Evento(e nucleo.Evento) {
	im.mu.Lock()
	defer im.mu.Unlock()
	switch e.Tipo {
	case "paso", "actualiza":
		if e.Paso == nil {
			return
		}
		pp := *e.Paso
		p, ok := im.pasos[pp.ID]
		if !ok {
			p = &pasoCLI{id: pp.ID, padre: pp.Padre, estado: nucleo.EstadoAbierto}
			if q, ok := im.pasos[pp.Padre]; ok && pp.Padre != 0 {
				p.prof = q.prof + 1
			}
			im.pasos[pp.ID] = p
			im.orden = append(im.orden, pp.ID)
		}
		antes := p.estado
		if t := unaLinea(pp.Titulo); t != "" {
			p.titulo = t
		}
		p.estado = pp.Estado
		if p.estado == nucleo.EstadoAbierto {
			return // printed later: when it closes, gets a child, or at the end
		}
		switch {
		case !p.impreso:
			im.imprimir(p)
		case antes == nucleo.EstadoAbierto && (p.estado == nucleo.EstadoMal || p.titulo != p.tituloImpreso):
			im.linea(p)
		}
	case "pregunta":
		im.vaciar()
	}
}

// imprimir prints p after its unprinted ancestors.
func (im *impresora) imprimir(p *pasoCLI) {
	if q, ok := im.pasos[p.padre]; ok && p.padre != 0 && !q.impreso {
		im.imprimir(q)
	}
	im.linea(p)
}

func (im *impresora) linea(p *pasoCLI) {
	p.impreso = true
	p.tituloImpreso = p.titulo
	fmt.Fprintf(im.w, "%s%s %s\n", strings.Repeat("  ", p.prof), marcador(p.estado), p.titulo)
}

func (im *impresora) vaciar() {
	for _, id := range im.orden {
		if p := im.pasos[id]; !p.impreso {
			im.imprimir(p)
		}
	}
}

// Vaciar prints every step not printed yet (called before the answer).
func (im *impresora) Vaciar() {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.vaciar()
}

func (im *impresora) escribir(f func(w io.Writer)) {
	im.mu.Lock()
	defer im.mu.Unlock()
	f(im.w)
}

// lector answers the trace's questions from stdin, one line per answer.
type lector struct {
	una    sync.Once
	r      io.Reader
	lineas chan string
}

func nuevoLector(r io.Reader) *lector { return &lector{r: r, lineas: make(chan string)} }

func (l *lector) empezar() {
	l.una.Do(func() {
		go func() {
			defer close(l.lineas)
			sc := bufio.NewScanner(l.r)
			sc.Buffer(make([]byte, 64*1024), 1<<20)
			for sc.Scan() {
				l.lineas <- sc.Text()
			}
		}()
	})
}

// Responder returns the nucleo.Responder: it shows the question and its options, and reads a line.
// A number picks an option; an empty line takes the default; end of input means no answer.
func (l *lector) Responder(imp *impresora) nucleo.Responder {
	return func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		imp.escribir(func(w io.Writer) {
			fmt.Fprintf(w, "? %s\n", unaLinea(p.Texto))
			for i, op := range p.Opciones {
				fmt.Fprintf(w, "    %d) %s\n", i+1, op)
			}
			pista := "Escribe tu respuesta"
			if len(p.Opciones) > 0 {
				pista = "Escribe el número de una opción"
				if p.Libre {
					pista += " o tu respuesta"
				}
			}
			if p.PorDefecto != "" {
				pista += fmt.Sprintf(" (Intro = «%s»)", p.PorDefecto)
			}
			fmt.Fprintf(w, "  %s: ", pista)
		})
		l.empezar()
		var s string
		var ok bool
		select {
		case s, ok = <-l.lineas:
		case <-ctx.Done():
			imp.escribir(func(w io.Writer) { fmt.Fprintln(w) })
			return p.PorDefecto, nucleo.ErrSinRespuesta
		}
		if !ok {
			imp.escribir(func(w io.Writer) { fmt.Fprintln(w, "(sin respuesta)") })
			return p.PorDefecto, nucleo.ErrSinRespuesta
		}
		s = strings.TrimSpace(s)
		if s == "" {
			if p.PorDefecto == "" {
				return "", nucleo.ErrSinRespuesta
			}
			return p.PorDefecto, nil
		}
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(p.Opciones) {
			// A number names an option, unless the number itself is one of the options.
			esOpcion := false
			for _, op := range p.Opciones {
				if op == s {
					esOpcion = true
				}
			}
			if !esOpcion {
				return p.Opciones[n-1], nil
			}
		}
		return s, nil
	}
}

// imprimirRespuesta prints the final answer below the trace.
func imprimirRespuesta(w io.Writer, r nucleo.Respuesta) {
	fmt.Fprintln(w, strings.Repeat("─", 40))
	if t := strings.TrimSpace(r.Texto); t != "" {
		fmt.Fprintln(w, t)
	}
	if r.Entendi != "" {
		fmt.Fprintln(w, r.Entendi)
	}
	for _, p := range r.Parrafos {
		if strings.TrimSpace(p) != "" {
			fmt.Fprintln(w, p)
		}
	}
	if len(r.Plan) > 0 {
		fmt.Fprintln(w)
		for i, s := range r.Plan {
			fmt.Fprintf(w, "%d. %s\n", i+1, s)
		}
	}
	if r.Tablero != nil && len(r.Tablero.Celdas) > 0 {
		fmt.Fprintln(w)
		imprimirTablero(w, r.Tablero)
	}
	for _, tb := range r.Tablas {
		fmt.Fprintln(w)
		imprimirTabla(w, tb)
	}
	if r.Codigo != "" {
		fmt.Fprintln(w, "\nCódigo:")
		fmt.Fprintln(w, strings.TrimRight(r.Codigo, "\n"))
	}
	if r.Diff != "" {
		fmt.Fprintln(w, "\nCambios:")
		fmt.Fprintln(w, strings.TrimRight(r.Diff, "\n"))
	}
	if r.CodigoTest != "" {
		fmt.Fprintln(w, "\nPruebas (archivo _test.go):")
		fmt.Fprintln(w, strings.TrimRight(r.CodigoTest, "\n"))
	}
	if len(r.Riesgos) > 0 {
		fmt.Fprintln(w, "\nRiesgos:")
		for _, v := range r.Riesgos {
			if v.Linea > 0 {
				fmt.Fprintf(w, "  ! línea %d: %s\n", v.Linea, v.Que)
			} else {
				fmt.Fprintf(w, "  ! %s\n", v.Que)
			}
		}
	}
	if len(r.Comprobaciones) > 0 {
		fmt.Fprintln(w)
		for _, c := range r.Comprobaciones {
			texto := c.Texto
			if texto == "" {
				texto = string(c.Nivel)
			}
			if c.Detalle != "" {
				texto += " (" + c.Detalle + ")"
			}
			fmt.Fprintf(w, "%s %s\n", marcaComprobacion(c), texto)
		}
	}
	for _, f := range r.Fuentes {
		t := f.Titulo
		if t == "" {
			t = f.URL
		}
		fmt.Fprintf(w, "Fuente: %s — %s\n", t, f.URL)
	}
	if r.Aprendido != "" {
		fmt.Fprintln(w, r.Aprendido)
	}
	if len(r.Sugerencias) > 0 {
		fmt.Fprintf(w, "Puedes seguir con: %s\n", strings.Join(r.Sugerencias, " · "))
	}
}

func marcaComprobacion(c nucleo.Comprobacion) string {
	switch {
	case c.Nivel == nucleo.NivelFallo || (c.Total > 0 && c.Pasan < c.Total):
		return "✘"
	case c.Nivel == nucleo.NivelSinComprobar || c.Nivel == "":
		return "·"
	}
	return "✔"
}

func imprimirTablero(w io.Writer, t *nucleo.Tablero) {
	for _, fila := range t.Celdas {
		partes := make([]string, len(fila))
		for i, v := range fila {
			if v == 0 {
				partes[i] = "."
			} else {
				partes[i] = strconv.Itoa(v)
			}
		}
		fmt.Fprintln(w, strings.Join(partes, " "))
	}
}

func imprimirTabla(w io.Writer, tb nucleo.Tabla) {
	if tb.Titulo != "" {
		fmt.Fprintln(w, tb.Titulo)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	marcas := len(tb.Marcas) > 0
	for _, c := range tb.Cabecera {
		if strings.Contains(c, "✔") {
			marcas = false // the table already has its own ✔/✘ column
		}
	}
	if len(tb.Cabecera) > 0 {
		pre := ""
		if marcas {
			pre = "\t"
		}
		fmt.Fprintln(tw, pre+strings.Join(tb.Cabecera, "\t"))
	}
	for i, fila := range tb.Filas {
		celdas := make([]string, len(fila))
		for j, c := range fila {
			celdas[j] = unaLinea(c)
		}
		pre := ""
		if marcas {
			m := ""
			if i < len(tb.Marcas) {
				switch tb.Marcas[i] {
				case "bien":
					m = "✔"
				case "mal":
					m = "✘"
				}
			}
			pre = m + "\t"
		}
		fmt.Fprintln(tw, pre+strings.Join(celdas, "\t"))
	}
	tw.Flush()
}

// ---------------------------------------------------------------------------------------------
// -instalar

// instalarEscritorio writes ~/.local/share/applications/nyx-codigo.desktop.
func instalarEscritorio() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" || !filepath.IsAbs(base) {
		casa, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(casa, ".local", "share")
	}
	dir := filepath.Join(base, "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ruta := filepath.Join(dir, "nyx-codigo.desktop")
	return ruta, os.WriteFile(ruta, []byte(textoEscritorio(exe)), 0o644)
}

// textoEscritorio is the .desktop file for the binary at exe.
func textoEscritorio(exe string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Nyx Código\n" +
		"Comment=Escribe, arregla, explica y prueba código Go; resuelve problemas de mates, lógica y puzles\n" +
		"Exec=" + citarExec(exe) + "\n" +
		"Icon=utilities-terminal\n" +
		"Terminal=false\n" +
		"Categories=Development;Education;\n"
}

// citarExec quotes a path for the Exec key of a .desktop file when it needs it.
func citarExec(s string) string {
	if !strings.ContainsAny(s, " \t\"'\\$`<>~|&;*?#()%") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", `$`, `\\$`, `%`, `%%`)
	return `"` + r.Replace(s) + `"`
}
