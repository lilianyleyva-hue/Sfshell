package recetas

// Helpers for the tests that use the real Go toolchain directly (no sandbox): they write temporary
// modules, build every program at once with `go build ./...` and run the binaries. They skip when
// exec.LookPath("go") fails.

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

var (
	versionUnaVez sync.Once
	versionGo     string // "1.24"
)

// goParaPruebas returns the go binary or skips the test.
func goParaPruebas(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no hay Go instalado")
	}
	versionUnaVez.Do(func() {
		out, err := exec.Command(bin, "env", "GOVERSION").Output()
		if err != nil {
			return
		}
		v := strings.TrimPrefix(strings.TrimSpace(string(out)), "go")
		partes := strings.SplitN(v, ".", 3)
		if len(partes) >= 2 {
			versionGo = partes[0] + "." + strings.TrimRightFunc(partes[1], func(r rune) bool { return r < '0' || r > '9' })
		}
	})
	if versionGo == "" {
		t.Skip("no sé qué versión de Go hay instalada")
	}
	return bin
}

// moduloPrueba is a temporary module.
type moduloPrueba struct {
	t   *testing.T
	go_ string
	dir string
}

func nuevoModulo(t *testing.T) *moduloPrueba {
	t.Helper()
	bin := goParaPruebas(t)
	m := &moduloPrueba{t: t, go_: bin, dir: t.TempDir()}
	m.escribir("go.mod", "module prueba\n\ngo "+versionGo+"\n")
	return m
}

func (m *moduloPrueba) escribir(rel, contenido string) {
	m.t.Helper()
	p := filepath.Join(m.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

// goCmd runs the go command in the module and returns its combined output.
func (m *moduloPrueba) goCmd(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.go_, args...)
	cmd.Dir = m.dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GO111MODULE=on", "GOFLAGS=", "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// construir builds every main package into dir/bin and returns that directory.
func (m *moduloPrueba) construir() string {
	m.t.Helper()
	bin := filepath.Join(m.dir, "bin")
	if out, err := m.goCmd("build", "-o", bin+string(filepath.Separator), "./..."); err != nil {
		m.t.Fatalf("go build falló: %v\n%s", err, recortarTexto(out, 20000))
	}
	return bin
}

// vet runs go vet over the whole module.
func (m *moduloPrueba) vet() {
	m.t.Helper()
	if out, err := m.goCmd("vet", "./..."); err != nil {
		m.t.Errorf("go vet se queja: %v\n%s", err, recortarTexto(out, 20000))
	}
}

func recortarTexto(s string, n int) string {
	if len(s) > n {
		return s[:n] + "\n…"
	}
	return s
}

// ejecutar runs a binary with args and stdin, with a timeout.
func ejecutar(bin string, args []string, entrada string) (salida string, errSalida string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(entrada)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.Dir = filepath.Dir(bin)
	err = cmd.Run()
	if ctx.Err() != nil {
		err = fmt.Errorf("tardó demasiado: %w", ctx.Err())
	}
	return out.String(), errb.String(), err
}

// ---- program output comparison (same rules as nucleo.CasoPrograma.Comparar) ----

var reNumeroPrueba = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`)

// salidaCoincide compares a program's stdout with the case's expected text:
// exacto, lineas (default), contiene, numeros.
func salidaCoincide(obtenida string, c nucleo.CasoPrograma) bool {
	o := strings.ReplaceAll(obtenida, "\r\n", "\n")
	e := strings.ReplaceAll(c.Esperado, "\r\n", "\n")
	switch c.Comparar {
	case "exacto":
		return o == e
	case "contiene":
		return strings.Contains(o, e)
	case "numeros":
		a, b := reNumeroPrueba.FindAllString(o, -1), reNumeroPrueba.FindAllString(e, -1)
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			x, err1 := strconv.ParseFloat(a[i], 64)
			y, err2 := strconv.ParseFloat(b[i], 64)
			if err1 != nil || err2 != nil {
				if a[i] != b[i] {
					return false
				}
				continue
			}
			if math.Abs(x-y) > 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y))) {
				return false
			}
		}
		return true
	}
	return lineasNormales(o) == lineasNormales(e)
}

func lineasNormales(s string) string {
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		ls[i] = strings.TrimRight(l, " \t")
	}
	for len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return strings.Join(ls, "\n")
}

// ---- recipe test files ----

// soporteComparar is the comparison helper of the generated recipe tests: deep equality where a nil
// slice equals an empty one, floats are equal within 1e-9 (relative) and two non-nil errors are equal.
const soporteComparar = `package solucion

import (
	"math"
	"reflect"
	"testing"
)

var nyxTipoError = reflect.TypeOf((*error)(nil)).Elem()

func nyxComparar[T any](t *testing.T, donde string, obtenido, esperado T) {
	t.Helper()
	if !nyxIgual(reflect.ValueOf(&obtenido).Elem(), reflect.ValueOf(&esperado).Elem()) {
		t.Errorf("%s: da %#v y debería dar %#v", donde, obtenido, esperado)
	}
}

func nyxIgual(a, b reflect.Value) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	switch a.Kind() {
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() && b.IsNil()
		}
		if a.Type() == nyxTipoError {
			return true
		}
		if a.Elem().Type() != b.Elem().Type() {
			return false
		}
		return nyxIgual(a.Elem(), b.Elem())
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() && b.IsNil()
		}
		return nyxIgual(a.Elem(), b.Elem())
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !nyxIgual(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		it := a.MapRange()
		for it.Next() {
			v := b.MapIndex(it.Key())
			if !v.IsValid() || !nyxIgual(it.Value(), v) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !nyxIgual(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Float32, reflect.Float64:
		x, y := a.Float(), b.Float()
		if math.IsNaN(x) || math.IsNaN(y) {
			return math.IsNaN(x) && math.IsNaN(y)
		}
		if x == y {
			return true
		}
		if math.IsInf(x, 0) || math.IsInf(y, 0) {
			return false
		}
		return math.Abs(x-y) <= 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y)))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() == b.Uint()
	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.String:
		return a.String() == b.String()
	}
	return false
}
`

// pruebaDeReceta writes a _test.go file (package solucion) that calls the recipe on every case and
// compares the results.
func pruebaDeReceta(r Receta) string {
	var sb strings.Builder
	sb.WriteString("package solucion\n\nimport (\n\t\"errors\"\n\t\"math\"\n\t\"testing\"\n)\n\nvar _ = errors.New\nvar _ = math.Inf\n\n")
	sb.WriteString("func TestReceta(t *testing.T) {\n")
	f := r.Firma
	ent := f.Entradas()
	desp := len(ent) - len(f.Params)
	for i, c := range r.Casos {
		sb.WriteString("\t{\n")
		for j, tp := range ent {
			fmt.Fprintf(&sb, "\t\tvar e%d %s = %s\n", j, tp.Go(), nucleo.FormatoGo(c.Entradas[j], tp))
		}
		llamada := f.Nombre
		if f.Receptor != nil {
			llamada = "e0." + f.Nombre
		}
		var args []string
		for j := desp; j < len(ent); j++ {
			a := "e" + strconv.Itoa(j)
			if f.Variadica && j == len(ent)-1 {
				a += "..."
			}
			args = append(args, a)
		}
		var res []string
		for k := range f.Res {
			res = append(res, "r"+strconv.Itoa(k))
		}
		fmt.Fprintf(&sb, "\t\t%s := %s(%s)\n", strings.Join(res, ", "), llamada, strings.Join(args, ", "))
		for k, tp := range f.Res {
			fmt.Fprintf(&sb, "\t\tvar x%d %s = %s\n", k, tp.Go(), nucleo.FormatoGo(c.Esperado[k], tp))
			donde := fmt.Sprintf("%s, caso %d, resultado %d", r.Nombre, i+1, k+1)
			fmt.Fprintf(&sb, "\t\tnyxComparar(t, %q, r%d, x%d)\n", donde, k, k)
		}
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// nombreDir turns a name into a directory name that is also a valid binary name.
func nombreDir(prefijo string, i int, nombre string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(nombre) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	return fmt.Sprintf("%s%03d_%s", prefijo, i, sb.String())
}
