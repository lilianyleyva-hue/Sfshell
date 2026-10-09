package arenero

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"math/bits"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nyxcodigo/internal/instrumenta"
	"nyxcodigo/internal/nucleo"
)

const (
	comandoPreparar = "go build -trimpath -gcflags=nyxprueba/...=-e -o prog . && arnés (fd 3/4)"
	maxTexto        = 64 << 10
)

// variante is one candidate file inside a batch.
type variante struct {
	k       int
	sufijo  string
	fuente  string // renamed + instrumented, package solucion
	lineas  []int
	nombres map[string]string
	fuera   bool
	motivo  string // why it is out: "inseguro" | "no compila"
	errores []nucleo.ErrorGo
}

func (v *variante) archivo() string { return "v" + strconv.Itoa(v.k) + ".go" }

// Preparar builds every variant into one harness binary (§4.2.2). Variants that are not safe for p.Permiso or
// that do not compile are left out and never run (their cases come back with Ejecutado=false); their errors are
// in Compilacion.Errores with Archivo "v<k>.go". Compilacion.OK is true when at least one variant was built.
// The error is ErrNoProbable, ErrInseguro (no variant is safe), ErrNoCompila (no variant compiles), ErrSinGo
// or the context's error.
func (a *Arenero) Preparar(ctx context.Context, p nucleo.Preparacion) (nucleo.Binario, nucleo.Compilacion, error) {
	inicio := time.Now()
	comp := nucleo.Compilacion{Comando: comandoPreparar}
	terminar := func(b nucleo.Binario, err error) (nucleo.Binario, nucleo.Compilacion, error) {
		comp.Ms = time.Since(inicio).Milliseconds()
		return b, comp, err
	}
	if !a.goOK {
		return terminar(nil, nucleo.ErrSinGo)
	}
	if !p.Firma.Probable() {
		return terminar(nil, nucleo.ErrNoProbable)
	}
	if len(p.Variantes) == 0 {
		return terminar(nil, errors.New("arenero: no hay ninguna variante que preparar"))
	}
	if err := ctx.Err(); err != nil {
		return terminar(nil, err)
	}
	vars := prepararVariantes(p)
	maxN := 0
	for _, v := range vars {
		if !v.fuera && len(v.lineas) > maxN {
			maxN = len(v.lineas)
		}
	}
	if !p.Instr.Cobertura {
		maxN = 0
	}
	dir, err := a.nuevoDir()
	if err != nil {
		return terminar(nil, err)
	}
	listo := false
	defer func() {
		if !listo {
			a.borrarDir(dir)
		}
	}()
	base := map[string]string{
		"go.mod":                     a.goMod(),
		"solucion/zz_nyx_soporte.go": instrumenta.Soporte("solucion", p.Instr, maxN),
	}
	if err := escribirArchivos(dir, base); err != nil {
		return terminar(nil, fmt.Errorf("arenero: no puedo escribir el módulo temporal: %w", err))
	}
	lote := &lote{a: a, dir: dir, vars: vars, op: opcionesArnes{firma: p.Firma, props: p.Props, sinProp: map[[2]int]bool{}}}
	ok, err := lote.construir(ctx)
	comp.Texto = recortarTexto(lote.texto.String())
	for _, v := range vars {
		if v.fuera && v.motivo == "no compila" {
			comp.Errores = append(comp.Errores, v.errores...)
		}
	}
	if err != nil {
		return terminar(nil, err)
	}
	if !ok {
		inseguras := 0
		for _, v := range vars {
			if v.motivo == "inseguro" {
				inseguras++
			}
		}
		if inseguras == len(vars) {
			return terminar(nil, nucleo.ErrInseguro)
		}
		return terminar(nil, nucleo.ErrNoCompila)
	}
	comp.OK = true
	b := &binario{
		a:       a,
		dir:     dir,
		prog:    filepath.Join(dir, "prog"),
		firma:   p.Firma,
		n:       len(vars),
		vars:    vars,
		instr:   p.Instr,
		permiso: p.Permiso,
	}
	listo = true
	return terminar(b, nil)
}

// prepararVariantes checks, normalizes, renames and instruments every variant.
func prepararVariantes(p nucleo.Preparacion) []*variante {
	vars := make([]*variante, len(p.Variantes))
	for k, src := range p.Variantes {
		v := &variante{k: k, sufijo: "_v" + strconv.Itoa(k)}
		vars[k] = v
		if vs := Revisar(src, p.Permiso); len(vs) > 0 {
			v.fuera, v.motivo = true, "inseguro"
			for _, x := range vs {
				v.errores = append(v.errores, nucleo.ErrorGo{Archivo: v.archivo(), Linea: x.Linea, Msg: x.Que})
			}
			continue
		}
		if errs := erroresSintaxis(src, v.archivo()); len(errs) > 0 {
			v.fuera, v.motivo, v.errores = true, "no compila", errs
			continue
		}
		norm, err := instrumenta.CambiarPaquete(src, "solucion")
		if err == nil {
			var ren string
			ren, v.nombres, err = instrumenta.Renombrar(norm, v.sufijo)
			if err == nil {
				op := p.Instr
				if instrumenta.TieneGoroutinas(src) {
					// counters are not atomic: with goroutines only the watchdog and the wall timeout apply
					op.Combustible, op.MaxPila, op.Variables = 0, 0, false
				}
				var r instrumenta.Resultado
				if r, err = instrumenta.Instrumentar(ren, op); err == nil {
					v.fuente, v.lineas = r.Fuente, r.Lineas
				}
			}
		}
		if err != nil {
			v.fuera, v.motivo = true, "no compila"
			v.errores = []nucleo.ErrorGo{{Archivo: v.archivo(), Linea: 1, Col: 1, Msg: err.Error()}}
		}
	}
	return vars
}

// erroresSintaxis returns the parse errors of src, named after archivo.
func erroresSintaxis(src, archivo string) []nucleo.ErrorGo {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, archivo, src, parser.AllErrors|parser.SkipObjectResolution)
	if err == nil {
		return nil
	}
	var lista scanner.ErrorList
	if errors.As(err, &lista) {
		out := make([]nucleo.ErrorGo, 0, len(lista))
		for _, e := range lista {
			out = append(out, nucleo.ErrorGo{Archivo: archivo, Linea: e.Pos.Line, Col: e.Pos.Column, Msg: e.Msg})
		}
		return out
	}
	return []nucleo.ErrorGo{{Archivo: archivo, Linea: 1, Col: 1, Msg: err.Error()}}
}

func recortarTexto(s string) string {
	if len(s) <= maxTexto {
		return s
	}
	return s[:maxTexto] + "\n…(recortado)"
}

// lote builds a batch, blaming and removing guilty variants and properties.
type lote struct {
	a     *Arenero
	dir   string
	vars  []*variante
	op    opcionesArnes
	texto strings.Builder
}

func (l *lote) activos() []int {
	var out []int
	for _, v := range l.vars {
		if !v.fuera {
			out = append(out, v.k)
		}
	}
	return out
}

var reArchivoVariante = regexp.MustCompile(`^(?:.*/)?v(\d+)\.go$`)

// construir builds until it succeeds (true) or nothing is left (false).
func (l *lote) construir(ctx context.Context) (bool, error) {
	for ronda := 0; ; ronda++ {
		activos := l.activos()
		if len(activos) == 0 {
			return false, nil
		}
		ok, errs, gen, err := l.compilar(ctx, activos)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		if len(activos) == 1 {
			l.echar(activos[0], errs)
			return false, nil
		}
		culpables, malas := l.atribuir(errs, gen)
		if len(culpables)+len(malas) == 0 || ronda >= 4 {
			return l.bisectar(ctx, activos)
		}
		for k, es := range culpables {
			l.echar(k, es)
		}
		for par, msg := range malas {
			l.op.sinProp[par] = true
			nombre := ""
			if par[1] < len(l.op.props) {
				nombre = l.op.props[par[1]].Nombre
			}
			fmt.Fprintf(&l.texto, "Nota: quité la propiedad «%s» de la variante %d porque no compila: %s\n", nombre, par[0], msg)
		}
	}
}

// echar takes variant k out with its build errors.
func (l *lote) echar(k int, errs []nucleo.ErrorGo) {
	v := l.vars[k]
	v.fuera, v.motivo = true, "no compila"
	re := regexp.MustCompile(regexp.QuoteMeta(v.sufijo) + `([^0-9]|$)`)
	for _, e := range errs {
		e.Msg = re.ReplaceAllString(e.Msg, "$1")
		if !reArchivoVariante.MatchString(e.Archivo) {
			// an error in the generated harness: the variant does not fit the signature
			e.Linea, e.Col = 0, 0
			e.Msg = "no encaja con la firma " + l.op.firma.Go() + ": " + e.Msg
		}
		e.Archivo = v.archivo()
		v.errores = append(v.errores, e)
	}
	if len(v.errores) == 0 {
		v.errores = []nucleo.ErrorGo{{Archivo: v.archivo(), Msg: "no compila junto con el arnés de pruebas"}}
	}
}

// atribuir blames build errors on variants (by file or by generated line) and on properties.
func (l *lote) atribuir(errs []nucleo.ErrorGo, gen *arnesGenerado) (map[int][]nucleo.ErrorGo, map[[2]int]string) {
	culpables := map[int][]nucleo.ErrorGo{}
	malas := map[[2]int]string{}
	for _, e := range errs {
		if m := reArchivoVariante.FindStringSubmatch(e.Archivo); m != nil {
			k, _ := strconv.Atoi(m[1])
			if k >= 0 && k < len(l.vars) {
				culpables[k] = append(culpables[k], e)
			}
			continue
		}
		var d duenio = comun
		switch {
		case strings.HasSuffix(e.Archivo, filepath.Base(archivoArnes)) && gen != nil:
			d = gen.arnes.duenoLinea(e.Linea)
		case strings.HasSuffix(e.Archivo, filepath.Base(archivoCodec)) && gen != nil:
			d = gen.codec.duenoLinea(e.Linea)
		}
		switch {
		case d.v >= 0 && d.p >= 0:
			if _, ya := malas[[2]int{d.v, d.p}]; !ya {
				malas[[2]int{d.v, d.p}] = e.Msg
			}
		case d.v >= 0:
			culpables[d.v] = append(culpables[d.v], e)
		}
	}
	return culpables, malas
}

// bisectar finds the variants that break the build when their errors cannot be blamed directly.
// It spends at most log2(K)+1 builds per culprit search and two searches in total.
func (l *lote) bisectar(ctx context.Context, activos []int) (bool, error) {
	presupuesto := 2 * (bits.Len(uint(len(activos))) + 1)
	pendientes := append([]int(nil), activos...)
	for presupuesto > 0 && len(pendientes) > 0 {
		conjunto := pendientes
		var ultimos []nucleo.ErrorGo
		for len(conjunto) > 1 && presupuesto > 0 {
			mitad := conjunto[:len(conjunto)/2]
			ok, errs, _, err := l.compilar(ctx, mitad)
			presupuesto--
			if err != nil {
				return false, err
			}
			if ok {
				conjunto = conjunto[len(conjunto)/2:]
			} else {
				conjunto, ultimos = mitad, errs
			}
		}
		if len(conjunto) != 1 {
			break
		}
		l.echar(conjunto[0], ultimos)
		pendientes = quitar(pendientes, conjunto[0])
		if len(pendientes) == 0 {
			return false, nil
		}
		ok, _, _, err := l.compilar(ctx, pendientes)
		presupuesto--
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	// give up: whatever is left does not build
	for _, k := range pendientes {
		if !l.vars[k].fuera {
			l.echar(k, nil)
		}
	}
	return false, nil
}

func quitar(xs []int, x int) []int {
	out := make([]int, 0, len(xs))
	for _, y := range xs {
		if y != x {
			out = append(out, y)
		}
	}
	return out
}

// compilar writes the variants in `incluidos` plus a matching harness, and runs go build once.
func (l *lote) compilar(ctx context.Context, incluidos []int) (bool, []nucleo.ErrorGo, *arnesGenerado, error) {
	dentro := map[int]bool{}
	for _, k := range incluidos {
		dentro[k] = true
	}
	op := l.op
	op.sufijos = make([]string, len(l.vars))
	op.nombres = make([]map[string]string, len(l.vars))
	archivos := map[string]string{}
	for _, v := range l.vars {
		ruta := filepath.Join(l.dir, "solucion", v.archivo())
		if dentro[v.k] {
			op.sufijos[v.k] = v.sufijo
			op.nombres[v.k] = v.nombres
			archivos["solucion/"+v.archivo()] = v.fuente
		} else {
			_ = os.Remove(ruta)
		}
	}
	gen, err := generarArnes(op)
	if err != nil {
		return false, nil, nil, err
	}
	for _, n := range gen.notas {
		fmt.Fprintf(&l.texto, "Nota: %s\n", n)
	}
	for rel, c := range gen.archivos {
		archivos[rel] = c
	}
	if err := escribirArchivos(l.dir, archivos); err != nil {
		return false, nil, nil, fmt.Errorf("arenero: no puedo escribir el módulo temporal: %w", err)
	}
	salida, ok, err := l.a.goBuild(ctx, l.dir, "-gcflags=nyxprueba/...=-e", "-o", "prog", ".")
	if err != nil {
		return false, nil, nil, err
	}
	if ok {
		return true, nil, gen, nil
	}
	l.texto.WriteString(salida)
	if !strings.HasSuffix(salida, "\n") {
		l.texto.WriteByte('\n')
	}
	return false, ParseErrores(salida), gen, nil
}

// goBuild runs `go build -trimpath <args>` in dir. ok is false when the build failed; err is only for
// problems that are not the code's fault (no Go, timeout, cancelled).
func (a *Arenero) goBuild(ctx context.Context, dir string, args ...string) (string, bool, error) {
	a.compilaciones.Add(1)
	return a.goCmd(ctx, dir, append([]string{"build", "-trimpath"}, args...)...)
}

// goCmd runs the go command with the stripped environment and the TCompilar timeout.
func (a *Arenero) goCmd(ctx context.Context, dir string, args ...string) (string, bool, error) {
	if err := a.tomar(ctx); err != nil {
		return "", false, err
	}
	defer a.soltar()
	ctx2, cancelar := context.WithTimeout(ctx, a.cfg.TCompilar)
	defer cancelar()
	o := orden{dir: dir, bin: a.goBin, args: args, env: a.entornoGo(dir), pared: a.cfg.TCompilar}
	f := a.ejecutarDirecto(ctx2, o)
	salida := f.salida.String() + f.errSalida.String()
	if f.err != nil {
		if ctx.Err() != nil {
			return salida, false, ctx.Err()
		}
		if ctx2.Err() != nil || f.agotado {
			return salida, false, fmt.Errorf("arenero: la compilación tardó más de %v", a.cfg.TCompilar)
		}
		return salida, false, f.err
	}
	if f.agotado {
		return salida, false, fmt.Errorf("arenero: la compilación tardó más de %v", a.cfg.TCompilar)
	}
	return salida, f.codigo == 0, nil
}

// ejecutarDirecto runs a trusted tool (the go command) without the trampoline or namespaces,
// in its own process group so a timeout kills the compiler too.
func (a *Arenero) ejecutarDirecto(ctx context.Context, o orden) fin {
	o.directo = true
	o.maxSalida = 4 << 20
	return a.ejecutar(ctx, o, nil)
}
