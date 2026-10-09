package arenero

import (
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

var actualizar = flag.Bool("actualizar", false, "reescribe los archivos .json de testdata/errores")

func TestParseErroresGolden(t *testing.T) {
	archivos, err := filepath.Glob("testdata/errores/*.txt")
	if err != nil || len(archivos) < 10 {
		t.Fatalf("faltan salidas capturadas: %d", len(archivos))
	}
	for _, a := range archivos {
		t.Run(filepath.Base(a), func(t *testing.T) {
			datos, err := os.ReadFile(a)
			if err != nil {
				t.Fatal(err)
			}
			got := ParseErrores(string(datos))
			golden := strings.TrimSuffix(a, ".txt") + ".json"
			if *actualizar {
				b, _ := json.MarshalIndent(got, "", "  ")
				if err := os.WriteFile(golden, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			b, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			var esperado []nucleo.ErrorGo
			if err := json.Unmarshal(b, &esperado); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(esperado) {
				t.Fatalf("hay %d errores y esperaba %d: %+v", len(got), len(esperado), got)
			}
			for i := range got {
				if got[i] != esperado[i] {
					t.Errorf("error %d = %+v, esperaba %+v", i, got[i], esperado[i])
				}
			}
		})
	}
}

func TestParseErroresCasos(t *testing.T) {
	es := ParseErrores("# nyxprueba/x\nx/x.go:5:25: not enough arguments in call to f\n\thave (number)\n\twant (int, int)\n")
	if len(es) != 1 || es[0].Archivo != "x/x.go" || es[0].Linea != 5 || es[0].Col != 25 ||
		es[0].Msg != "not enough arguments in call to f\nhave (number)\nwant (int, int)" {
		t.Errorf("continuación mal leída: %+v", es)
	}
	es = ParseErrores("./x.go:12:5: msg\nx.go:3: otro\n")
	if len(es) != 2 || es[0].Archivo != "x.go" || es[0].Linea != 12 || es[1].Linea != 3 || es[1].Col != 0 {
		t.Errorf("formatos mal leídos: %+v", es)
	}
	if es := ParseErrores(""); len(es) != 0 {
		t.Errorf("salida vacía dio %+v", es)
	}
}

func contiene(vs []nucleo.Violacion, sub string) bool {
	for _, v := range vs {
		if strings.Contains(v.Que, sub) {
			return true
		}
	}
	return false
}

func TestRevisarRechaza(t *testing.T) {
	casos := map[string]struct {
		src, que string
		grave    bool
	}{
		"os/exec":     {"package p\nimport \"os/exec\"\nvar _ = exec.Command", "os/exec", false},
		"net/http":    {"package p\nimport \"net/http\"\nvar _ = http.Get", "net/http", false},
		"unsafe":      {"package p\nimport \"unsafe\"\nvar _ = unsafe.Sizeof(1)", "unsafe", false},
		"cgo":         {"package p\n// #include <stdio.h>\nimport \"C\"\n", "cgo", true},
		"linkname":    {"package p\nimport _ \"unsafe\"\n//go:linkname f runtime.f\nfunc f()\n", "directiva peligrosa", true},
		"terceros":    {"package p\nimport \"github.com/x/y\"\nvar _ = y.Z", "github.com/x/y", true},
		"goroutina":   {"package p\nfunc f() { go f() }", "goroutine", false},
		"os.Remove":   {"package p\nimport \"os\"\nfunc f() { os.Remove(\"x\") }", "os.Remove", false},
		"time.Sleep":  {"package p\nimport \"time\"\nfunc f() { time.Sleep(1) }", "time.Sleep", false},
		"punto os":    {"package p\nimport . \"os\"\nvar _ = Args", "con punto", false},
		"prefijo":     {"package p\nvar nyx__x int", "nyx__", true},
		"alias os":    {"package p\nimport sis \"os\"\nfunc f() { sis.Chdir(\"/\") }", "os.Chdir", false},
		"método Fd":   {"package p\nimport \"os\"\nvar _ = os.Stdout.Fd()", "Fd", false},
		"go:build":    {"//go:build linux\n\npackage p\n", "directiva especial", false},
		"nyxprueba":   {"package p\nimport \"nyxprueba/nyxfd\"\nvar _ = nyxfd.Nonce", "nyxprueba/nyxfd", true},
		"cgo directo": {"package p\n/*\n#cgo LDFLAGS: -lm\n*/\nimport \"C\"\n", "directiva peligrosa", true},
	}
	for nombre, c := range casos {
		vs := Revisar(c.src, nucleo.PermisoAuto)
		if !contiene(vs, c.que) {
			t.Errorf("%s: no se rechazó en modo automático (%+v)", nombre, vs)
			continue
		}
		us := Revisar(c.src, nucleo.PermisoUsuario)
		if c.grave != contiene(us, c.que) {
			t.Errorf("%s: con permiso del usuario, grave=%v pero dio %+v", nombre, c.grave, us)
		}
	}
}

func TestRevisarPermite(t *testing.T) {
	src := `package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		fmt.Fprintln(os.Stdout, strings.ToUpper(sc.Text()), len(os.Args))
	}
	_ = time.Now()
	os.Exit(0)
}
`
	if vs := Revisar(src, nucleo.PermisoAuto); len(vs) != 0 {
		t.Errorf("os.Stdin con bufio.Scanner debe estar permitido: %+v", vs)
	}
	if vs := Revisar("package p\nfunc (", nucleo.PermisoAuto); vs != nil {
		t.Errorf("el código que no se puede leer no da violaciones: %+v", vs)
	}
	grande := "package p\n" + strings.Repeat("// x\n", 20000)
	if vs := Revisar(grande, nucleo.PermisoUsuario); len(vs) != 1 || !vs[0].Grave {
		t.Errorf("un archivo de más de 64 KiB es grave: %+v", vs)
	}
	vs := Revisar("package p\nimport \"os/exec\"\nvar _ = exec.Command", nucleo.PermisoAuto)
	if len(vs) != 1 || vs[0].Linea != 2 || vs[0].Paquete != "os/exec" || vs[0].Que != "usa el paquete os/exec: puede ejecutar otros programas" {
		t.Errorf("violación mal descrita: %+v", vs)
	}
}

func TestRenombrarExpr(t *testing.T) {
	nombres := map[string]string{"Punto": "Punto_v2", "F": "F_v2", "e0": "e0_v2", "Max": "Max_v2"}
	got := renombrarExpr("nyx__Igual(F([]Punto{}), e0) && r0.Max > Max", nombres)
	if got != "nyx__Igual(F([]Punto_v2{}), e0) && r0.Max > Max_v2" {
		t.Errorf("renombrarExpr = %q", got)
	}
}

func TestGenerarArnesParsea(t *testing.T) {
	punto := nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "Punto", Definido: true, Campos: []nucleo.Campo{
		{Nombre: "X", Tipo: nucleo.TInt}, {Nombre: "nombre", Tipo: nucleo.TString}}}
	firmas := []nucleo.Firma{
		{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}},
		{Nombre: "Mover", Receptor: &nucleo.Param{Nombre: "p", Tipo: nucleo.PunteroA(punto)},
			Params: []nucleo.Param{{Nombre: "d", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{punto, nucleo.TError}},
		{Nombre: "Max", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TFloat)}}, Variadica: true,
			Res: []nucleo.Tipo{nucleo.MapaDe(nucleo.TString, nucleo.ListaDe(nucleo.TInt)), nucleo.TBool}},
	}
	props := []nucleo.Propiedad{{Nombre: "p", Expr: "len(e0) >= 0", Requiere: "true"}, {Nombre: "mala", Expr: "x := 1"}}
	for _, f := range firmas {
		archivos, err := GenerarArnes(f, 3, props, nucleo.InstrNormal, []string{"_v0", "", "_v2"})
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range []string{"main.go", "nyxfd/nyxfd.go", archivoCodec, archivoArnes} {
			fset := token.NewFileSet()
			af, err := parser.ParseFile(fset, n, archivos[n], 0)
			if err != nil {
				t.Fatalf("%s de %s no se puede leer: %v\n%s", n, f.Go(), err, archivos[n])
			}
			if n == archivoArnes {
				hay := false
				ast.Inspect(af, func(x ast.Node) bool {
					if fd, ok := x.(*ast.FuncDecl); ok && fd.Name.Name == "nyx__correr_v1" {
						hay = true
					}
					return true
				})
				if hay {
					t.Error("la variante sin sufijo no debe tener corredor")
				}
				if strings.Contains(archivos[n], "\"mala\"") {
					t.Error("la propiedad que no es una expresión debe quitarse")
				}
			}
		}
	}
	if _, err := GenerarArnes(nucleo.Firma{Nombre: "F"}, 1, nil, nucleo.InstrNormal, []string{"_v0"}); err != nucleo.ErrNoProbable {
		t.Errorf("una firma sin resultados no es probable: %v", err)
	}
}

func TestLimitado(t *testing.T) {
	l := nuevoLimitado(10)
	l.Write([]byte("0123456789abc"))
	l.Write([]byte("def"))
	if l.String() != "0123456789" || !l.Recortado() || l.Cola(4) != "cdef" {
		t.Errorf("limitado: %q %v %q", l.String(), l.Recortado(), l.Cola(4))
	}
}

func TestVersionLenguaje(t *testing.T) {
	for in, esp := range map[string]string{"go1.24.7": "1.24", "go1.22": "1.22", "": "1.22", "go1.23rc1": "1.23", "devel": "1.22"} {
		if got := versionLenguaje(in); got != esp {
			t.Errorf("versionLenguaje(%q) = %q, esperaba %q", in, got, esp)
		}
	}
}

func TestExtraerCaida(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("runtime: goroutine stack exceeds 1000000000-byte limit\nruntime: sp=0xc020160398 stack=[0xc020160000, 0xc040160000]\nfatal error: stack overflow\n\nruntime stack:\n")
	for i := 0; i < 400; i++ {
		sb.WriteString("goroutine 4 gp=0xc000002fc0 m=nil [GC scavenge wait]:\n\truntime.gopark(...)\n")
	}
	got := extraerCaida(sb.String())
	if !strings.HasPrefix(got, "runtime: goroutine stack exceeds") || !strings.Contains(got, "fatal error: stack overflow") || len(got) > maxCaida {
		t.Errorf("pila: %d bytes, %q…", len(got), cortar(got, 200))
	}
	oom := "basura previa\nfatal error: runtime: out of memory\n\nruntime stack:\nruntime.throw()\n"
	if got := extraerCaida(oom); !strings.HasPrefix(got, "fatal error: runtime: out of memory") {
		t.Errorf("memoria: %q", got)
	}
	if got := extraerCaida("hola\nadiós\n"); got != "" {
		t.Errorf("sin línea de caída: %q", got)
	}
	larga := "panic: " + strings.Repeat("x", 3*maxCaida)
	if got := extraerCaida(larga); len(got) != maxCaida || !strings.HasPrefix(got, "panic: x") {
		t.Errorf("una sola línea larga se recorta: %d", len(got))
	}
	f := fin{errSalida: nuevoLimitado(1 << 20), codigo: 2}
	f.errSalida.Write([]byte(sb.String()))
	if r := resumenCaida(f); !strings.HasPrefix(r, "runtime: goroutine stack exceeds") || !strings.HasSuffix(r, "(el programa se cayó (código de salida 2))") {
		t.Errorf("resumen: %q", cortar(r, 300))
	}
}
