package arenero

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// limites are passed to the trampoline as JSON in os.Args[2].
type limites struct {
	Datos    uint64 `json:"datos"`    // RLIMIT_DATA bytes
	CPU      uint64 `json:"cpu"`      // RLIMIT_CPU seconds (soft; hard = soft+1)
	Archivo  uint64 `json:"archivo"`  // RLIMIT_FSIZE bytes
	Abiertos uint64 `json:"abiertos"` // RLIMIT_NOFILE
}

func (a *Arenero) limitesEjecucion() limites {
	cpu := uint64(a.cfg.CPU / time.Second)
	if cpu == 0 {
		cpu = 1
	}
	return limites{
		Datos:    a.cfg.MemoriaMiB << 20,
		CPU:      cpu,
		Archivo:  1 << 20,
		Abiertos: 64,
	}
}

// orden describes one sandboxed run.
type orden struct {
	dir    string
	bin    string
	args   []string
	stdin  io.Reader // nil = /dev/null
	extra  []*os.File
	env    []string
	pared  time.Duration
	aislar bool // use the namespaces if the probe said they work

	directo   bool // a trusted tool (the go command): no trampoline, no namespaces
	maxSalida int  // 0 = Config.MaxSalida
}

// fin is how a run ended.
type fin struct {
	salida, errSalida *limitado
	codigo            int    // exit code; -1 if killed by a signal
	senal             string // "SIGKILL"… or ""
	agotado           bool   // our wall timer killed it
	ms                int64
	comando           string
	err               error // the process could not be started
}

// limitado is a capped writer: it keeps the first max bytes, remembers the last 4 KiB, and never blocks
// or fails the writer.
type limitado struct {
	mu        sync.Mutex
	max       int
	buf       []byte
	cola      []byte
	recortado bool
}

const tamCola = 4 << 10

func nuevoLimitado(max int) *limitado { return &limitado{max: max} }

func (l *limitado) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if libre := l.max - len(l.buf); libre > 0 {
		n := min(libre, len(p))
		l.buf = append(l.buf, p[:n]...)
		if n < len(p) {
			l.recortado = true
		}
	} else if len(p) > 0 {
		l.recortado = true
	}
	l.cola = append(l.cola, p...)
	if len(l.cola) > tamCola {
		l.cola = append(l.cola[:0], l.cola[len(l.cola)-tamCola:]...)
	}
	return len(p), nil
}

func (l *limitado) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return string(l.buf)
}

// Cola returns the last n bytes written (even past the cap).
func (l *limitado) Cola(n int) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.cola
	if len(c) > n {
		c = c[len(c)-n:]
	}
	return strings.ToValidUTF8(string(c), "")
}

func (l *limitado) Recortado() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.recortado
}

// ejecutar starts o.bin through the trampoline, calls mientras (in this goroutine) while the process runs,
// then waits for it. The wall timer and ctx kill the whole process group.
func (a *Arenero) ejecutar(ctx context.Context, o orden, mientras func()) fin {
	max := o.maxSalida
	if max <= 0 {
		max = a.cfg.MaxSalida
	}
	f := fin{salida: nuevoLimitado(max), errSalida: nuevoLimitado(max), codigo: -1}
	var cmd *exec.Cmd
	if !o.directo && a.limites && a.trampolin != "" {
		lim, _ := json.Marshal(a.limitesEjecucion())
		args := append([]string{argTrampolin, string(lim), o.bin}, o.args...)
		cmd = exec.Command(a.trampolin, args...)
	} else {
		cmd = exec.Command(o.bin, o.args...)
	}
	f.comando = comandoVisible(o)
	cmd.Dir = o.dir
	cmd.Env = o.env
	if cmd.Env == nil {
		cmd.Env = []string{}
	}
	cmd.Stdin = o.stdin
	cmd.Stdout = f.salida
	cmd.Stderr = f.errSalida
	cmd.ExtraFiles = o.extra
	cmd.SysProcAttr = atributosProceso(!o.directo && o.aislar && a.redAislada)
	cmd.WaitDelay = time.Second
	if err := ctx.Err(); err != nil {
		f.err = err
		cerrarTodos(o.extra)
		return f
	}
	inicio := time.Now()
	if err := cmd.Start(); err != nil {
		f.err = fmt.Errorf("arenero: no pude arrancar el programa: %w", err)
		cerrarTodos(o.extra)
		if mientras != nil {
			mientras()
		}
		return f
	}
	cerrarTodos(o.extra) // the child has its own copies
	pgid := cmd.Process.Pid
	var agotado atomic.Bool
	matar := func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) }
	pared := o.pared
	if pared <= 0 {
		pared = a.cfg.TEjecutar
	}
	reloj := time.AfterFunc(pared, func() {
		agotado.Store(true)
		matar()
	})
	parar := context.AfterFunc(ctx, matar)
	if mientras != nil {
		mientras()
	}
	errWait := cmd.Wait()
	reloj.Stop()
	parar()
	matar() // anything left in the group (user programs that started children)
	f.ms = time.Since(inicio).Milliseconds()
	f.agotado = agotado.Load()
	if cmd.ProcessState != nil {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			f.codigo = -1
			f.senal = nombreSenal(ws.Signal())
		} else {
			f.codigo = cmd.ProcessState.ExitCode()
		}
	} else if errWait != nil {
		f.err = errWait
	}
	if ctx.Err() != nil && f.err == nil && f.codigo != 0 {
		f.err = ctx.Err()
	}
	return f
}

func cerrarTodos(fs []*os.File) {
	for _, x := range fs {
		if x != nil {
			_ = x.Close()
		}
	}
}

func comandoVisible(o orden) string {
	partes := []string{"./" + baseNombre(o.bin)}
	for _, a := range o.args {
		partes = append(partes, citar(a))
	}
	return strings.Join(partes, " ")
}

func baseNombre(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func citar(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

var nombresSenal = map[syscall.Signal]string{
	syscall.SIGKILL: "SIGKILL", syscall.SIGXCPU: "SIGXCPU", syscall.SIGXFSZ: "SIGXFSZ",
	syscall.SIGSEGV: "SIGSEGV", syscall.SIGABRT: "SIGABRT", syscall.SIGBUS: "SIGBUS",
	syscall.SIGILL: "SIGILL", syscall.SIGFPE: "SIGFPE", syscall.SIGTERM: "SIGTERM",
	syscall.SIGPIPE: "SIGPIPE", syscall.SIGINT: "SIGINT", syscall.SIGQUIT: "SIGQUIT",
	syscall.SIGTRAP: "SIGTRAP", syscall.SIGSYS: "SIGSYS",
}

func nombreSenal(s syscall.Signal) string {
	if n, ok := nombresSenal[s]; ok {
		return n
	}
	return fmt.Sprintf("señal %d", int(s))
}

// explicarSenal turns a signal into simple Spanish.
func explicarSenal(s string) string {
	switch s {
	case "SIGKILL":
		return "el sistema lo detuvo a la fuerza (SIGKILL): tardaba demasiado o usaba demasiada memoria"
	case "SIGXCPU":
		return "usó demasiado tiempo de procesador (SIGXCPU)"
	case "SIGXFSZ":
		return "intentó escribir un archivo demasiado grande (SIGXFSZ)"
	case "SIGSEGV":
		return "accedió a memoria prohibida (SIGSEGV)"
	case "SIGABRT":
		return "se abortó (SIGABRT)"
	}
	return "terminó por la señal " + s
}

// ---- trampoline ----

const argTrampolin = "__arenero"

// EsTrampolin reports whether this process was started as the sandbox trampoline (os.Args[1] == "__arenero").
func EsTrampolin() bool { return len(os.Args) > 1 && os.Args[1] == argTrampolin }

// Trampolin parses the limits JSON from os.Args[2], sets the rlimits, and replaces the process with
// os.Args[3] (args os.Args[3:], same environment). It never returns: on failure it writes a Spanish
// message to stderr and exits with code 125 (limits) or 126 (exec).
func Trampolin() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "arenero: el trampolín necesita los límites y el programa")
		os.Exit(125)
	}
	var l limites
	if err := json.Unmarshal([]byte(os.Args[2]), &l); err != nil {
		fmt.Fprintln(os.Stderr, "arenero: no entiendo los límites:", err)
		os.Exit(125)
	}
	if err := aplicarLimites(l); err != nil {
		fmt.Fprintln(os.Stderr, "arenero: no pude poner los límites:", err)
		os.Exit(125)
	}
	err := syscall.Exec(os.Args[3], os.Args[3:], os.Environ())
	fmt.Fprintf(os.Stderr, "arenero: no pude ejecutar %s: %v\n", os.Args[3], err)
	os.Exit(126)
}

func init() {
	if EsTrampolin() {
		Trampolin()
	}
}

var errSinSoporte = errors.New("arenero: este sistema no permite poner límites a los procesos")
