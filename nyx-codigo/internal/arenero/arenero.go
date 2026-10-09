// Package arenero is the sandbox of Nyx Código: it implements nucleo.Ejecutor.
//
// It does the static safety check, builds temporary modules, generates the test harness and its
// codecs, runs `go build` and `go vet`, runs binaries through the self-re-exec trampoline under
// rlimits and (when available) user+network namespaces, recovers from crashes, and parses compiler
// output.
//
// Any program that links this package gets the trampoline for free: the package init function
// checks os.Args and, when the process was started as `<self> __arenero …`, applies the limits and
// execs the target before anything else runs. main() should still call
// `if arenero.EsTrampolin() { arenero.Trampolin() }` first, as the spec asks; it is harmless twice.
package arenero

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Config sets the sandbox up. Zero fields take the defaults written next to them.
type Config struct {
	DirDatos   string        // ~/.local/share/nyx-codigo (uses DirDatos/tmp and DirDatos/cache/go-build)
	GoBin      string        // "" = LookPath("go"), then /usr/local/go/bin/go, /usr/lib/go/bin/go
	MemoriaMiB uint64        // RLIMIT_DATA, default 512
	CPU        time.Duration // RLIMIT_CPU, default 10 s
	TCompilar  time.Duration // default 60 s (cold cache), warm builds ~0.3 s
	TEjecutar  time.Duration // wall per process, default 10 s
	TCaso      time.Duration // per-case watchdog, default 2 s
	MaxSalida  int           // stdout/stderr cap per process, default 1 MiB
	Paralelo   int           // concurrent builds+runs, default 2

	// DirCache is the GOCACHE for builds; "" = DirDatos/cache/go-build. Tests point it at a shared,
	// persistent directory so that every test binary does not start with a cold cache.
	DirCache string
}

// Arenero is the sandbox. It is safe for concurrent use.
type Arenero struct {
	cfg       Config
	dirTmp    string
	dirCache  string
	goBin     string
	goRoot    string
	goVersion string
	lenguaje  string // language version for temp go.mod files: "1.24"
	goOK      bool

	trampolin  string // os.Executable(), "" when the trampoline cannot be used
	limites    bool   // the trampoline applies rlimits
	redAislada bool   // user+net namespaces work

	sem chan struct{}

	mu      sync.Mutex
	dirs    map[string]bool
	cerrado bool

	compilaciones atomic.Int64
}

var _ nucleo.Ejecutor = (*Arenero)(nil)

// Errors.
var (
	ErrCerrado = errors.New("arenero: la caja de arena ya está cerrada")
)

// Nuevo finds go, reads `go env GOVERSION GOROOT`, probes namespaces and rlimits, and empties tmp/.
//
// A missing Go toolchain is not an error: the sandbox is returned with Estado().Nivel == "sin Go" and every
// build returns nucleo.ErrSinGo, so the rest of the app (maths, logic, puzzles) keeps working.
// An error means the data directories cannot be created.
func Nuevo(c Config) (*Arenero, error) {
	if c.DirDatos == "" {
		casa, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("arenero: no sé dónde guardar los datos: %w", err)
		}
		c.DirDatos = filepath.Join(casa, ".local", "share", "nyx-codigo")
	}
	if c.MemoriaMiB == 0 {
		c.MemoriaMiB = 512
	}
	if c.CPU <= 0 {
		c.CPU = 10 * time.Second
	}
	if c.TCompilar <= 0 {
		c.TCompilar = 60 * time.Second
	}
	if c.TEjecutar <= 0 {
		c.TEjecutar = 10 * time.Second
	}
	if c.TCaso <= 0 {
		c.TCaso = 2 * time.Second
	}
	if c.MaxSalida <= 0 {
		c.MaxSalida = 1 << 20
	}
	if c.Paralelo <= 0 {
		c.Paralelo = 2
	}
	datos, err := filepath.Abs(c.DirDatos)
	if err != nil {
		return nil, fmt.Errorf("arenero: %w", err)
	}
	c.DirDatos = datos
	a := &Arenero{
		cfg:    c,
		dirTmp: filepath.Join(datos, "tmp"),
		sem:    make(chan struct{}, c.Paralelo),
		dirs:   map[string]bool{},
	}
	a.dirCache = c.DirCache
	if a.dirCache == "" {
		a.dirCache = filepath.Join(datos, "cache", "go-build")
	}
	for _, d := range []string{datos, a.dirTmp, a.dirCache} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("arenero: no puedo crear %s: %w", d, err)
		}
	}
	vaciar(a.dirTmp)

	a.goBin = buscarGo(c.GoBin)
	if a.goBin != "" {
		a.leerGoEnv()
	}
	a.lenguaje = versionLenguaje(a.goVersion)
	a.sondear()
	return a, nil
}

// vaciar removes everything inside dir (the startup sweep of tmp/).
func vaciar(dir string) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entradas {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

func buscarGo(dado string) string {
	candidatos := []string{}
	if dado != "" {
		candidatos = append(candidatos, dado)
	} else {
		if p, err := exec.LookPath("go"); err == nil {
			candidatos = append(candidatos, p)
		}
		candidatos = append(candidatos, "/usr/local/go/bin/go", "/usr/lib/go/bin/go")
	}
	for _, p := range candidatos {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
			return p
		}
	}
	return ""
}

func (a *Arenero) leerGoEnv() {
	dir, err := os.MkdirTemp(a.dirTmp, "env-")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, a.goBin, "env", "GOVERSION", "GOROOT")
	cmd.Dir = dir
	cmd.Env = a.entornoGo(dir)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	lineas := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lineas) >= 2 {
		a.goVersion = strings.TrimSpace(lineas[0])
		a.goRoot = strings.TrimSpace(lineas[1])
		a.goOK = strings.HasPrefix(a.goVersion, "go")
	}
}

// versionLenguaje turns "go1.24.7" into "1.24" (fallback "1.22").
func versionLenguaje(v string) string {
	v = strings.TrimPrefix(v, "go")
	partes := strings.SplitN(v, ".", 3)
	if len(partes) < 2 || partes[0] != "1" {
		return "1.22"
	}
	menor := partes[1]
	for i, r := range menor {
		if r < '0' || r > '9' {
			menor = menor[:i]
			break
		}
	}
	if menor == "" {
		return "1.22"
	}
	return "1." + menor
}

// entornoGo is the stripped environment of every go command (nothing inherited). TMPDIR is a
// subdirectory of the module: the go command ignores a go.mod that sits exactly in the temp root.
func (a *Arenero) entornoGo(tmp string) []string {
	_ = os.MkdirAll(filepath.Join(tmp, ".tmp"), 0o700)
	return []string{
		"PATH=" + filepath.Dir(a.goBin) + ":/usr/bin:/bin",
		"HOME=" + tmp,
		"GOPATH=" + filepath.Join(tmp, "gopath"),
		"GOCACHE=" + a.dirCache,
		"GOPROXY=off",
		"GOTOOLCHAIN=local",
		"GOFLAGS=-mod=mod",
		"GOWORK=off",
		"GOENV=off",
		"CGO_ENABLED=0",
		"GOTELEMETRY=off",
		"LANG=C.UTF-8",
		"TMPDIR=" + filepath.Join(tmp, ".tmp"),
	}
}

// entornoEjecucion is the environment of every sandboxed run.
func entornoEjecucion(tmp, nonce string) []string {
	env := []string{
		"GOMAXPROCS=2",
		"GOMEMLIMIT=256MiB",
		"HOME=" + tmp,
		"TMPDIR=" + tmp,
		"LANG=C.UTF-8",
	}
	if nonce != "" {
		env = append(env, "NYX__NONCE="+nonce)
	}
	return env
}

// sondear probes the trampoline (rlimits) and the namespaces by running `true` through them.
func (a *Arenero) sondear() {
	yo, err := os.Executable()
	if err != nil {
		return
	}
	verdad := ""
	for _, p := range []string{"/bin/true", "/usr/bin/true"} {
		if _, err := os.Stat(p); err == nil {
			verdad = p
			break
		}
	}
	if verdad == "" {
		if p, err := exec.LookPath("true"); err == nil {
			verdad = p
		}
	}
	if verdad == "" {
		return
	}
	a.trampolin = yo
	a.limites = true
	dir, err := os.MkdirTemp(a.dirTmp, "sonda-")
	if err != nil {
		a.trampolin, a.limites = "", false
		return
	}
	defer os.RemoveAll(dir)
	ctx, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	o := orden{dir: dir, bin: verdad, env: entornoEjecucion(dir, ""), pared: 5 * time.Second}
	if r := a.ejecutar(ctx, o, nil); r.err != nil || r.codigo != 0 {
		a.trampolin, a.limites = "", false
	}
	if !puedeAislar() {
		return
	}
	a.redAislada = true
	o.aislar = true
	if r := a.ejecutar(ctx, o, nil); r.err != nil || r.codigo != 0 {
		a.redAislada = false
	}
}

// Estado reports what the sandbox can do.
func (a *Arenero) Estado() nucleo.EstadoArenero {
	e := nucleo.EstadoArenero{
		GoVersion:  a.goVersion,
		GoBin:      a.goBin,
		GoOK:       a.goOK,
		RedAislada: a.redAislada,
		Limites:    a.limites,
	}
	switch {
	case !a.goOK:
		e.Nivel = "sin Go"
	case a.redAislada && a.limites:
		e.Nivel = "completa"
	default:
		e.Nivel = "básica"
	}
	return e
}

// Aviso is the Spanish sentence for the trace when isolation is incomplete ("" when complete).
func (a *Arenero) Aviso() string {
	switch {
	case !a.goOK:
		return "No encuentro Go instalado: no puedo compilar ni ejecutar código."
	case !a.limites:
		return "No puedo poner límites de memoria y tiempo a los programas; solo ejecuto código con la lista segura."
	case !a.redAislada:
		return "Sin aislamiento de red; solo ejecuto código con la lista segura."
	}
	return ""
}

// GoRoot is the GOROOT of the toolchain in use ("" without Go).
func (a *Arenero) GoRoot() string { return a.goRoot }

// Compilaciones counts the `go build` runs made so far (tests use it to check that Probar never rebuilds).
func (a *Arenero) Compilaciones() int64 { return a.compilaciones.Load() }

// Cerrar removes every temp directory still alive. Binaries and programs stop working afterwards.
func (a *Arenero) Cerrar() error {
	a.mu.Lock()
	a.cerrado = true
	dirs := make([]string, 0, len(a.dirs))
	for d := range a.dirs {
		dirs = append(dirs, d)
	}
	a.dirs = map[string]bool{}
	a.mu.Unlock()
	var primero error
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil && primero == nil {
			primero = err
		}
	}
	return primero
}

// nuevoDir creates DirDatos/tmp/<16 hex> (0700).
func (a *Arenero) nuevoDir() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cerrado {
		return "", ErrCerrado
	}
	for i := 0; i < 5; i++ {
		d := filepath.Join(a.dirTmp, aleatorio(8))
		if err := os.Mkdir(d, 0o700); err == nil {
			a.dirs[d] = true
			return d, nil
		} else if !os.IsExist(err) {
			return "", fmt.Errorf("arenero: no puedo crear la carpeta temporal: %w", err)
		}
	}
	return "", errors.New("arenero: no puedo crear la carpeta temporal")
}

func (a *Arenero) borrarDir(d string) {
	a.mu.Lock()
	delete(a.dirs, d)
	a.mu.Unlock()
	_ = os.RemoveAll(d)
}

// aleatorio returns 2n random hex digits.
func aleatorio(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail on Linux; keep going with the time as a last resort
		t := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(t >> (8 * (i % 8)))
		}
	}
	return hex.EncodeToString(b)
}

// tomar takes a slot of the build-or-run semaphore.
func (a *Arenero) tomar(ctx context.Context) error {
	select {
	case a.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Arenero) soltar() { <-a.sem }

// escribirArchivos writes rel path → content under dir.
func escribirArchivos(dir string, archivos map[string]string) error {
	for rel, contenido := range archivos {
		ruta := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (a *Arenero) goMod() string {
	return "module nyxprueba\n\ngo " + a.lenguaje + "\n"
}
