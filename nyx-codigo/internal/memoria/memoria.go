// Package memoria keeps everything Nyx Código learns across runs (§4.13, §8): verified functions
// (biblioteca.json), facts and rules (hechos.jsonl, reglas.jsonl), the lexicon overlay and word-problem
// verbs (lexico.json), learning counters (aprendizaje.json), counterexamples (contraejemplos.json),
// web hints (pistas.json), the request history (historial/AAAA-MM-DD.jsonl) and pending questions
// (pendientes.jsonl).
//
// JSON files are rewritten crash-safely (temp file, fsync, rename, fsync of the directory). JSONL files are
// appended one record per line and tolerate a truncated last line. A corrupt file never stops the program:
// it is set aside as <name>.roto-AAAAMMDD-hhmmss, that store starts empty, and Avisos explains it.
//
// Every method is safe for concurrent use: each store has its own lock.
package memoria

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// File names inside the data directory (§8).
const (
	archivoVersion        = "version"
	archivoBiblioteca     = "biblioteca.json"
	archivoHechos         = "hechos.jsonl"
	archivoReglas         = "reglas.jsonl"
	archivoLexico         = "lexico.json"
	archivoAprendizaje    = "aprendizaje.json"
	archivoContraejemplos = "contraejemplos.json"
	archivoPistas         = "pistas.json"
	archivoPendientes     = "pendientes.jsonl"
	dirHistorial          = "historial"
)

// ErrCerrado is returned by writes after Cerrar.
var ErrCerrado = errors.New("memoria: el almacén ya está cerrado")

// Almacen is the persistent memory. Create it with Abrir and close it with Cerrar.
type Almacen struct {
	dir string

	muAvisos sync.Mutex
	avisos   []string

	muCerrado sync.RWMutex
	cerrado   bool

	fun     biblioteca
	hechos  baseJSONL[hechoGuardado]
	reglas  baseJSONL[reglaGuardada]
	lex     lexico
	apr     aprendizaje
	contra  contraejemplos
	pistas  almacenPistas
	hist    historial
	pend    baseJSONL[Pendiente]
	ahora   func() time.Time // the clock (tests may fix it)
	vigilar sync.WaitGroup
}

// Abrir opens (or creates, with permissions 0700) the memory directory and loads every store. Corrupt files
// are renamed to <name>.roto-AAAAMMDD-hhmmss and their store starts empty; it never fails because of the
// data, only when the directory itself cannot be created.
func Abrir(dir string) (*Almacen, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("memoria: falta la carpeta de datos")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("memoria: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("memoria: no puedo crear %s: %w", abs, err)
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("memoria: %s no es una carpeta", abs)
	}
	a := &Almacen{dir: abs, ahora: time.Now}
	limpiarTemporales(abs)
	a.comprobarVersion()

	a.cargarBiblioteca()
	a.hechos = baseJSONL[hechoGuardado]{nombre: archivoHechos, prefijo: "h", idDe: func(h *hechoGuardado) *string { return &h.ID }}
	cargarBase(a, &a.hechos)
	a.reglas = baseJSONL[reglaGuardada]{nombre: archivoReglas, prefijo: "r", idDe: func(r *reglaGuardada) *string { return &r.ID }}
	cargarBase(a, &a.reglas)
	a.pend = baseJSONL[Pendiente]{nombre: archivoPendientes, prefijo: "p", idDe: func(p *Pendiente) *string { return &p.ID }}
	cargarBase(a, &a.pend)
	a.cargarLexico()
	a.cargarAprendizaje()
	a.cargarContraejemplos()
	a.cargarPistas()
	return a, nil
}

// comprobarVersion writes the version file the first time and warns about a different one.
func (a *Almacen) comprobarVersion() {
	ruta := filepath.Join(a.dir, archivoVersion)
	datos, err := os.ReadFile(ruta)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := escribirAtomico(ruta, []byte(fmt.Sprintf("%d\n", Version))); err != nil {
			a.avisar(fmt.Sprintf("No pude escribir el archivo de versión de la memoria: %v.", err))
		}
	case err != nil:
		a.avisar(fmt.Sprintf("No pude leer el archivo de versión de la memoria: %v.", err))
	default:
		if v := strings.TrimSpace(string(datos)); v != fmt.Sprint(Version) {
			a.avisar(fmt.Sprintf("La memoria dice ser de la versión %q y yo uso la %d: leo lo que entiendo.", cortar(v, 20), Version))
		}
	}
}

// Dir is the memory directory.
func (a *Almacen) Dir() string { return a.dir }

// Avisos returns the Spanish warnings collected so far (corrupt files, skipped lines…), for the trace and
// /api/estado.
func (a *Almacen) Avisos() []string {
	a.muAvisos.Lock()
	defer a.muAvisos.Unlock()
	return append([]string(nil), a.avisos...)
}

func (a *Almacen) avisar(s string) {
	a.muAvisos.Lock()
	defer a.muAvisos.Unlock()
	if len(a.avisos) < 100 {
		a.avisos = append(a.avisos, s)
	}
}

// Cerrar writes the pending counters and closes the store. Reads keep working afterwards (from memory);
// writes return ErrCerrado. Calling it twice is harmless.
func (a *Almacen) Cerrar() error {
	a.muCerrado.Lock()
	if a.cerrado {
		a.muCerrado.Unlock()
		return nil
	}
	a.cerrado = true
	a.muCerrado.Unlock()
	a.pararVolcado()
	a.vigilar.Wait() // a write already in progress
	return a.volcarContadores()
}

// abierto reports ErrCerrado after Cerrar.
func (a *Almacen) abierto() error {
	a.muCerrado.RLock()
	defer a.muCerrado.RUnlock()
	if a.cerrado {
		return ErrCerrado
	}
	return nil
}

// Contar returns how many functions, facts and confirmed words are stored (for /api/estado).
func (a *Almacen) Contar() (funciones, hechos, palabras int) {
	a.fun.mu.RLock()
	funciones = len(a.fun.lista)
	a.fun.mu.RUnlock()
	a.hechos.mu.RLock()
	hechos = len(a.hechos.orden)
	a.hechos.mu.RUnlock()
	a.lex.mu.RLock()
	palabras = len(a.lex.datos.Confirmadas)
	a.lex.mu.RUnlock()
	return
}

func cortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
