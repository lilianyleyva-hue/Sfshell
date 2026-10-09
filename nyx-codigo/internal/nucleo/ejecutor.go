package nucleo

import (
	"context"
	"errors"
	"time"
)

// The sandbox contract (§3.5). arenero implements it; nucleotest.EjecutorFalso fakes it for tests.

type Permiso int

const (
	PermisoAuto    Permiso = iota // anything run automatically: synthesized, recipe, web, user code under test without confirmation
	PermisoUsuario                // user explicitly confirmed (Ejecutar button / confirmation dialog)
)

type Violacion struct {
	Linea   int    `json:"linea"`
	Paquete string `json:"paquete,omitempty"`
	Que     string `json:"que"`   // Spanish: "usa el paquete os/exec: puede ejecutar otros programas"
	Grave   bool   `json:"grave"` // true: never runnable (cgo, linkname); false: runnable with PermisoUsuario
}

type ErrorGo struct {
	Archivo string `json:"archivo,omitempty"`
	Linea   int    `json:"linea"`
	Col     int    `json:"col"`
	Msg     string `json:"msg"`
}

type Compilacion struct {
	OK      bool      `json:"ok"`
	Errores []ErrorGo `json:"errores,omitempty"`
	Texto   string    `json:"texto,omitempty"` // raw compiler/vet output (capped 64 KiB)
	Comando string    `json:"comando"`         // e.g. "go build -gcflags=-e ./..." (shown in Evidencia)
	Ms      int64     `json:"ms"`
}

type Ejecucion struct {
	Salida, ErrSalida string
	Codigo            int    // exit code; -1 if killed
	Senal             string // "SIGKILL", "SIGXCPU"… or ""
	Agotado           bool   // wall timeout
	Recortado         bool   // output capped
	SinCombustible    bool   // fuel panic detected
	Ms                int64
	Comando           string
}

type EventoVar struct {
	Linea int    `json:"linea"`
	Var   string `json:"var"`
	Valor string `json:"valor"` // fmt.Sprint, ≤ 60 runes
}

type ResultadoCaso struct {
	Variante, Caso int
	Ejecutado      bool // false if never reached (variant stopped early, or too many crashes)
	OK             bool // no panic/crash AND (Esperado matches if any) AND all properties hold AND deterministic
	Obtenido       []Valor
	Impreso        string // what the function printed to stdout during this case (≤ 4 KiB)
	Panico         string // recovered panic message
	PropFallida    string // name of the first failing property
	SinCombustible bool
	Caida          bool // process died during this case (fatal error, stack overflow, OOM, signal)
	Agotado        bool // per-case watchdog fired
	NoDeterminista bool // two runs gave different results (OpcionesProbar.Repetir ≥ 2)
	Micros         int64
	Cubiertas      []int32     // statement ids executed (coverage builds)
	Eventos        []EventoVar // variable trace (trace builds)
}

type OpcionesInstr struct {
	Combustible int64 // steps per case; 0 = no fuel instrumentation
	MaxPila     int   // max call depth; 0 = no depth check
	Cobertura   bool
	Variables   bool
	MaxEventos  int // default 200
}

var InstrNormal = OpcionesInstr{Combustible: 5_000_000, MaxPila: 10_000}

type Preparacion struct {
	Variantes []string // each is one complete Go file (any package name; normalized to solucion)
	Firma     Firma    // function or method under test, named as in the sources
	Props     []Propiedad
	Instr     OpcionesInstr
	Permiso   Permiso
}

type OpcionesProbar struct {
	PararAlFallar bool          // a variant stops at its first failing case (mutation repair)
	Variantes     []int         // nil = all
	Repetir       int           // ≥2 runs every case that many times and sets NoDeterminista
	TiempoCaso    time.Duration // per-case watchdog; 0 = default (2 s)
}

type EstadoArenero struct {
	GoVersion  string `json:"goVersion"` // "go1.24.7"
	GoBin      string `json:"goBin"`
	GoOK       bool   `json:"goOK"`
	RedAislada bool   `json:"redAislada"` // user+net namespaces available
	Limites    bool   `json:"limites"`    // rlimits applied by the trampoline
	Nivel      string `json:"nivel"`      // "completa" | "básica" | "sin Go"
}

type Ejecutor interface {
	Revisar(fuente string, p Permiso) []Violacion
	Compilar(ctx context.Context, fuente string) Compilacion // any package; package main → binary, else library build
	Vet(ctx context.Context, fuente string) Compilacion
	Preparar(ctx context.Context, p Preparacion) (Binario, Compilacion, error)
	PrepararPrograma(ctx context.Context, fuente string, instr OpcionesInstr, p Permiso) (Programa, Compilacion, error)
	Estado() EstadoArenero
}

type Binario interface {
	Probar(ctx context.Context, casos []Caso, op OpcionesProbar) ([][]ResultadoCaso, error) // [variante][caso]
	Lineas(variante int) []int                                                              // statement id → line in that variant's original source
	NumVariantes() int
	Cerrar() error
}

type Programa interface {
	Correr(ctx context.Context, c CasoPrograma) (Ejecucion, error)
	Cerrar() error
}

var (
	ErrNoProbable = errors.New("nucleo: la firma no se puede probar automáticamente")
	ErrInseguro   = errors.New("nucleo: el código no pasa la revisión de seguridad")
	ErrNoCompila  = errors.New("nucleo: no compila")
	ErrSinGo      = errors.New("nucleo: no encuentro Go instalado")
)
