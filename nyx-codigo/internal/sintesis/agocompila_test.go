package sintesis_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/sintesis"
)

// TestAGoCompilaYCoincide builds the generated Go of the reference expressions with the real toolchain and
// checks that it gives what Evaluar gives on 200 random inputs each (inputs where the DSL has no result are
// left out).
func TestAGoCompilaYCoincide(t *testing.T) {
	r := sintesis.Base()
	refs := sintesis.ExpresionesReferencia()
	if len(refs) < 20 {
		t.Fatalf("hay %d expresiones de referencia; deben ser al menos 20", len(refs))
	}
	var progs []programa
	for _, ref := range refs {
		f, err := sintesis.LeerFirmaPrueba(ref[0])
		if err != nil {
			t.Fatal(err)
		}
		e, err := sintesis.Parse(ref[1], r, f)
		if err != nil {
			t.Fatalf("%s: %v", ref[1], err)
		}
		progs = append(progs, programa{f, e})
	}
	compararConGo(t, progs, 200, 50)
}

// TestAGoBanco does the same with programs taken at random from synthesis banks.
func TestAGoBanco(t *testing.T) {
	firmas := []struct {
		firma string
		ejs   [][2]string
	}{
		{"func F(nums []int) int", [][2]string{{"[1,2,3]", "7"}, {"[]", "-1"}}},
		{"func F(s string) string", [][2]string{{"\"hola mundo\"", "\"x\""}}},
		{"func F(s string) int", [][2]string{{"\"hola mundo\"", "77"}}},
		{"func F(nums []int) []int", [][2]string{{"[1,2,3]", "[9]"}}},
		{"func F(n int) bool", [][2]string{{"4", "true"}, {"5", "true"}, {"6", "false"}}},
		{"func F(s string) []string", [][2]string{{"\"a b\"", "[\"z\"]"}}},
		{"func F(a, b int) int", [][2]string{{"1; 2", "77"}}},
	}
	var progs []programa
	for i, fs := range firmas {
		f, err := sintesis.LeerFirmaPrueba(fs.firma)
		if err != nil {
			t.Fatal(err)
		}
		var ejs []nucleo.Caso
		for _, ej := range fs.ejs {
			var ent []nucleo.Valor
			for j, x := range strings.Split(ej[0], ";") {
				v, err := nucleo.ParseValor(strings.TrimSpace(x), f.Params[j].Tipo)
				if err != nil {
					t.Fatal(err)
				}
				ent = append(ent, v)
			}
			y, err := nucleo.ParseValor(ej[1], f.Res[0])
			if err != nil {
				t.Fatal(err)
			}
			ejs = append(ejs, nucleo.Caso{Entradas: ent, Esperado: []nucleo.Valor{y}})
		}
		for _, e := range sintesis.ProgramasDelBanco(f, ejs, 24, 20, int64(i+1)) {
			progs = append(progs, programa{f, e})
		}
	}
	if len(progs) < 100 {
		t.Fatalf("solo %d programas del banco", len(progs))
	}
	compararConGo(t, progs, 100, 0)
}

type programa struct {
	firma nucleo.Firma
	expr  *sintesis.Expr
}

// compararConGo compiles every program with AGo into one module and compares the Go results with Evaluar.
func compararConGo(t *testing.T, progs []programa, porPrograma, minimoValidos int) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no hay Go instalado")
	}
	if testing.Short() {
		t.Skip("compila con Go de verdad")
	}
	dir := t.TempDir()
	escribir(t, filepath.Join(dir, "go.mod"), "module prueba\n\ngo 1.22\n")
	type esperado struct {
		prog int
		ent  []nucleo.Valor
		sal  nucleo.Valor
	}
	var esperados []esperado
	var imports, llamadas strings.Builder
	rnd := rand.New(rand.NewSource(1))
	for i, pr := range progs {
		f, e := pr.firma, pr.expr
		f.Nombre = "Funcion"
		src, err := sintesis.AGo(e, f)
		if err != nil {
			t.Fatalf("%s: %v", e, err)
		}
		pkg := fmt.Sprintf("p%03d", i)
		if err := os.MkdirAll(filepath.Join(dir, pkg), 0o755); err != nil {
			t.Fatal(err)
		}
		escribir(t, filepath.Join(dir, pkg, "solucion.go"), src)
		fmt.Fprintf(&imports, "\t%s \"prueba/%s\"\n", pkg, pkg)
		validos := 0
		for intento := 0; intento < 10*porPrograma && validos < porPrograma; intento++ {
			ent := make([]nucleo.Valor, len(f.Params))
			for j, p := range f.Params {
				ent[j] = azar(rnd, p.Tipo)
			}
			v, err := sintesis.Evaluar(e, ent)
			if err != nil {
				continue // ⊥: left out of the differential test
			}
			validos++
			esperados = append(esperados, esperado{prog: i, ent: ent, sal: v})
			args := make([]string, len(ent))
			for j, x := range ent {
				args[j] = nucleo.FormatoGo(x, f.Params[j].Tipo)
			}
			fmt.Fprintf(&llamadas, "\tfmt.Println(cod(%s.Funcion(%s)))\n", pkg, strings.Join(args, ", "))
		}
		if validos < minimoValidos {
			t.Errorf("%s: solo %d entradas con resultado", e, validos)
		}
	}
	escribir(t, filepath.Join(dir, "main.go"), fmt.Sprintf(mainPrueba, imports.String(), llamadas.String()))
	cmd := exec.Command(goBin, "build", "-o", "prog", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=mod", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	run := exec.Command(filepath.Join(dir, "prog"))
	run.Dir = dir
	inicio := time.Now()
	out, err := run.Output()
	if err != nil {
		t.Fatalf("el programa falló: %v", err)
	}
	t.Logf("%d programas, %d casos en %v", len(progs), len(esperados), time.Since(inicio))
	lineas := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lineas) != len(esperados) {
		t.Fatalf("%d líneas y %d casos", len(lineas), len(esperados))
	}
	fallos := 0
	for k, es := range esperados {
		pr := progs[es.prog]
		got, err := nucleo.DecodificarJSON(json.RawMessage(lineas[k]), pr.firma.Res[0])
		if err != nil {
			t.Fatalf("no puedo leer %q: %v", lineas[k], err)
		}
		if !nucleo.Igual(got, es.sal) {
			fallos++
			if fallos <= 10 {
				t.Errorf("%s con %v: Go da %v y Evaluar %v", pr.expr, es.ent, got, es.sal)
			}
		}
	}
}

func escribir(t *testing.T, ruta, texto string) {
	t.Helper()
	if err := os.WriteFile(ruta, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
}

const letras = "aeiouáéñbcdlmnrstAEZ "

func azar(r *rand.Rand, t nucleo.Tipo) nucleo.Valor {
	switch t.Clase {
	case nucleo.CInt:
		if r.Intn(10) == 0 {
			return r.Intn(2001) - 1000
		}
		return r.Intn(61) - 30
	case nucleo.CRune:
		rs := []rune(letras + "09")
		return int(rs[r.Intn(len(rs))])
	case nucleo.CFloat:
		return float64(r.Intn(81)-40) / 4
	case nucleo.CBool:
		return r.Intn(2) == 0
	case nucleo.CString:
		if r.Intn(6) == 0 {
			return fmt.Sprint(r.Intn(2001) - 1000)
		}
		rs := []rune(letras)
		n := r.Intn(14)
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteRune(rs[r.Intn(len(rs))])
		}
		return sb.String()
	case nucleo.CLista:
		n := r.Intn(7)
		if n == 0 && r.Intn(2) == 0 {
			return nil
		}
		xs := make([]nucleo.Valor, n)
		for i := range xs {
			if t.Elem.Clase == nucleo.CInt {
				xs[i] = r.Intn(41) - 20
			} else {
				xs[i] = azar(r, *t.Elem)
			}
		}
		if t.Elem.Clase == nucleo.CLista && n > 0 {
			// rectangular matrices
			ancho := r.Intn(4)
			for i := range xs {
				fila := make([]nucleo.Valor, ancho)
				for j := range fila {
					fila[j] = r.Intn(21) - 10
				}
				xs[i] = fila
			}
		}
		return xs
	}
	return nil
}

const mainPrueba = `package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

%s)

func cod(v any) string { return codR(reflect.ValueOf(v)) }

func codR(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			return "null"
		}
		ks := v.MapKeys()
		sort.Slice(ks, func(i, j int) bool { return ks[i].String() < ks[j].String() })
		var partes []string
		for _, k := range ks {
			partes = append(partes, "["+codR(k)+","+codR(v.MapIndex(k))+"]")
		}
		return "[" + strings.Join(partes, ",") + "]"
	case reflect.Slice:
		if v.IsNil() {
			return "null"
		}
		partes := make([]string, v.Len())
		for i := range partes {
			partes[i] = codR(v.Index(i))
		}
		return "[" + strings.Join(partes, ",") + "]"
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64)
	case reflect.String:
		b, _ := json.Marshal(v.String())
		return string(b)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	}
	return strconv.FormatInt(v.Int(), 10)
}

func main() {
%s}
`
