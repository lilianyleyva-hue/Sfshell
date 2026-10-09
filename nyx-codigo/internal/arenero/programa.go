package arenero

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/instrumenta"
	"nyxcodigo/internal/nucleo"
)

// limpiarSalida rewrites the paths of the temp module so the user sees "x.go:3:2".
func limpiarSalida(s string) string {
	s = strings.ReplaceAll(s, "./x/x.go:", "x.go:")
	s = strings.ReplaceAll(s, "x/x.go:", "x.go:")
	s = strings.ReplaceAll(s, "./x.go:", "x.go:")
	return recortarTexto(s)
}

func erroresDe(salida string) []nucleo.ErrorGo {
	es := ParseErrores(salida)
	for i := range es {
		if es[i].Archivo != "" {
			es[i].Archivo = filepath.Base(es[i].Archivo)
		}
	}
	return es
}

// Compilar builds one file (no harness): <tmp>/x/x.go with its own package clause. A main package is built
// as a binary, anything else as a library. Exactly one build.
func (a *Arenero) Compilar(ctx context.Context, fuente string) nucleo.Compilacion {
	inicio := time.Now()
	// -o always: a main package built as ./x would collide with the directory x (for a library it only
	// writes the package archive)
	args := []string{"-gcflags=-e", "-o", "prog", "./x"}
	comp := nucleo.Compilacion{Comando: "go build -gcflags=-e -o prog ./x"}
	if !a.goOK {
		comp.Texto = nucleo.ErrSinGo.Error()
		comp.Errores = []nucleo.ErrorGo{{Msg: "no encuentro Go instalado"}}
		return comp
	}
	dir, err := a.nuevoDir()
	if err != nil {
		comp.Texto = err.Error()
		comp.Errores = []nucleo.ErrorGo{{Msg: err.Error()}}
		return comp
	}
	defer a.borrarDir(dir)
	if err := escribirArchivos(dir, map[string]string{"go.mod": a.goMod(), "x/x.go": fuente}); err != nil {
		comp.Texto = err.Error()
		comp.Errores = []nucleo.ErrorGo{{Msg: err.Error()}}
		return comp
	}
	salida, ok, err := a.goBuild(ctx, dir, args...)
	comp.Ms = time.Since(inicio).Milliseconds()
	comp.Texto = limpiarSalida(salida)
	if err != nil {
		comp.Errores = []nucleo.ErrorGo{{Msg: err.Error()}}
		if comp.Texto == "" {
			comp.Texto = err.Error()
		}
		return comp
	}
	comp.OK = ok
	if !ok {
		comp.Errores = erroresDe(salida)
	}
	return comp
}

// Vet runs `go vet` on one file (package solucion or main, or whatever it declares).
func (a *Arenero) Vet(ctx context.Context, fuente string) nucleo.Compilacion {
	inicio := time.Now()
	comp := nucleo.Compilacion{Comando: "go vet ./x"}
	if !a.goOK {
		comp.Texto = nucleo.ErrSinGo.Error()
		comp.Errores = []nucleo.ErrorGo{{Msg: "no encuentro Go instalado"}}
		return comp
	}
	dir, err := a.nuevoDir()
	if err != nil {
		comp.Texto = err.Error()
		comp.Errores = []nucleo.ErrorGo{{Msg: err.Error()}}
		return comp
	}
	defer a.borrarDir(dir)
	if err := escribirArchivos(dir, map[string]string{"go.mod": a.goMod(), "x/x.go": fuente}); err != nil {
		comp.Texto = err.Error()
		comp.Errores = []nucleo.ErrorGo{{Msg: err.Error()}}
		return comp
	}
	salida, ok, err := a.goCmd(ctx, dir, "vet", "./x")
	comp.Ms = time.Since(inicio).Milliseconds()
	comp.Texto = limpiarSalida(salida)
	if err != nil {
		comp.Errores = []nucleo.ErrorGo{{Msg: err.Error()}}
		if comp.Texto == "" {
			comp.Texto = err.Error()
		}
		return comp
	}
	comp.OK = ok
	if !ok {
		comp.Errores = erroresDe(salida)
	}
	return comp
}

// programa is a built whole program (nucleo.Programa).
type programa struct {
	a       *Arenero
	dir     string
	prog    string
	conFuel bool
	mu      sync.Mutex
	cerrado bool
}

var _ nucleo.Programa = (*programa)(nil)

// PrepararPrograma normalizes to package main, instruments for fuel when instr.Combustible > 0 and the code has
// no goroutines, then builds it once. Correr runs it through the trampoline.
func (a *Arenero) PrepararPrograma(ctx context.Context, fuente string, instr nucleo.OpcionesInstr, p nucleo.Permiso) (nucleo.Programa, nucleo.Compilacion, error) {
	inicio := time.Now()
	comp := nucleo.Compilacion{Comando: "go build -gcflags=-e -o prog ."}
	terminar := func(pr nucleo.Programa, err error) (nucleo.Programa, nucleo.Compilacion, error) {
		comp.Ms = time.Since(inicio).Milliseconds()
		return pr, comp, err
	}
	if !a.goOK {
		return terminar(nil, nucleo.ErrSinGo)
	}
	if vs := Revisar(fuente, p); len(vs) > 0 {
		for _, v := range vs {
			comp.Errores = append(comp.Errores, nucleo.ErrorGo{Archivo: "x.go", Linea: v.Linea, Msg: v.Que})
		}
		return terminar(nil, nucleo.ErrInseguro)
	}
	if errs := erroresSintaxis(fuente, "x.go"); len(errs) > 0 {
		comp.Errores = errs
		var sb strings.Builder
		for _, e := range errs {
			sb.WriteString(e.Archivo + ":" + itoa(e.Linea) + ":" + itoa(e.Col) + ": " + e.Msg + "\n")
		}
		comp.Texto = sb.String()
		return terminar(nil, nucleo.ErrNoCompila)
	}
	src, err := instrumenta.CambiarPaquete(fuente, "main")
	if err != nil {
		comp.Errores = []nucleo.ErrorGo{{Archivo: "x.go", Linea: 1, Msg: err.Error()}}
		return terminar(nil, nucleo.ErrNoCompila)
	}
	archivos := map[string]string{"go.mod": a.goMod()}
	conFuel := instr.Combustible > 0 && !instrumenta.TieneGoroutinas(src)
	if conFuel {
		op := nucleo.OpcionesInstr{Combustible: instr.Combustible, MaxPila: instr.MaxPila}
		r, err := instrumenta.Instrumentar(src, op)
		if err != nil {
			comp.Errores = []nucleo.ErrorGo{{Archivo: "x.go", Linea: 1, Msg: err.Error()}}
			return terminar(nil, nucleo.ErrNoCompila)
		}
		src = r.Fuente
		archivos["zz_nyx_soporte.go"] = instrumenta.Soporte("main", op, 0)
	}
	archivos["x.go"] = src
	dir, err := a.nuevoDir()
	if err != nil {
		return terminar(nil, err)
	}
	if err := escribirArchivos(dir, archivos); err != nil {
		a.borrarDir(dir)
		return terminar(nil, err)
	}
	salida, ok, err := a.goBuild(ctx, dir, "-gcflags=-e", "-o", "prog", ".")
	comp.Texto = limpiarSalida(salida)
	if err != nil {
		a.borrarDir(dir)
		return terminar(nil, err)
	}
	if !ok {
		a.borrarDir(dir)
		comp.Errores = erroresDe(salida)
		return terminar(nil, nucleo.ErrNoCompila)
	}
	comp.OK = true
	return terminar(&programa{a: a, dir: dir, prog: filepath.Join(dir, "prog"), conFuel: conFuel}, nil)
}

// Correr runs the program once with stdin = c.Entrada and args = c.Args. Comparing the output is the caller's job.
func (p *programa) Correr(ctx context.Context, c nucleo.CasoPrograma) (nucleo.Ejecucion, error) {
	p.mu.Lock()
	cerrado := p.cerrado
	p.mu.Unlock()
	if cerrado {
		return nucleo.Ejecucion{}, ErrCerrado
	}
	if err := p.a.tomar(ctx); err != nil {
		return nucleo.Ejecucion{}, err
	}
	defer p.a.soltar()
	o := orden{
		dir: p.dir, bin: p.prog, args: c.Args, stdin: strings.NewReader(c.Entrada),
		env: entornoEjecucion(p.dir, ""), pared: p.a.cfg.TEjecutar, aislar: true,
	}
	f := p.a.ejecutar(ctx, o, nil)
	if f.err != nil {
		return nucleo.Ejecucion{}, f.err
	}
	e := nucleo.Ejecucion{
		Salida:    f.salida.String(),
		ErrSalida: f.errSalida.String(),
		Codigo:    f.codigo,
		Senal:     f.senal,
		Agotado:   f.agotado,
		Recortado: f.salida.Recortado() || f.errSalida.Recortado(),
		Ms:        f.ms,
		Comando:   f.comando,
	}
	if c.Entrada != "" {
		e.Comando += " < entrada"
	}
	if p.conFuel && strings.Contains(f.errSalida.Cola(tamCola), instrumenta.MensajeSinComb) {
		e.SinCombustible = true
	}
	return e, nil
}

func (p *programa) Cerrar() error {
	p.mu.Lock()
	ya := p.cerrado
	p.cerrado = true
	p.mu.Unlock()
	if !ya {
		p.a.borrarDir(p.dir)
	}
	return nil
}

// Calentar builds one program importing every allowlisted package and the harness packages, so the
// build cache (GOCACHE) is warm. It is meant to run once at startup, in the background.
func (a *Arenero) Calentar(ctx context.Context) error {
	if !a.goOK {
		return nucleo.ErrSinGo
	}
	paquetes := map[string]bool{}
	for p := range PermitidosAuto {
		paquetes[p] = true
	}
	for p := range importaEstatico {
		paquetes[p] = true
	}
	paquetes["bufio"] = true
	paquetes["syscall"] = true
	lista := make([]string, 0, len(paquetes))
	for p := range paquetes {
		lista = append(lista, p)
	}
	sort.Strings(lista)
	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n")
	for _, p := range lista {
		sb.WriteString("\t_ \"" + p + "\"\n")
	}
	sb.WriteString(")\n\nfunc main() {}\n")
	dir, err := a.nuevoDir()
	if err != nil {
		return err
	}
	defer a.borrarDir(dir)
	if err := escribirArchivos(dir, map[string]string{"go.mod": a.goMod(), "main.go": sb.String()}); err != nil {
		return err
	}
	salida, ok, err := a.goCmd(ctx, dir, "build", "-trimpath", "-o", "prog", ".")
	if err != nil {
		return err
	}
	if !ok {
		return &errorCalentar{salida: salida}
	}
	return nil
}

type errorCalentar struct{ salida string }

func (e *errorCalentar) Error() string {
	return "arenero: no pude preparar Go: " + strings.TrimSpace(recortarTexto(e.salida))
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
