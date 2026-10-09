package cerebro

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This test enforces the dependency rule of §2: every package under internal/ is parsed with go/parser
// and its imports are checked against the table below. Packages that do not exist yet are skipped.
//
//	Package                                   May import (besides stdlib)
//	nucleo                                    —   (and only the stdlib packages listed in §3.1)
//	nucleo/nucleotest                         nucleo
//	instrumenta, pruebas, reparar, analisis,  nucleo
//	recetas, sintesis, lengua, mates, logica,
//	puzles, saber, memoria, servidor
//	arenero                                   nucleo, instrumenta
//	cerebro                                   everything under internal/ except servidor
//	main.go                                   cerebro, servidor, arenero, memoria, saber, nucleo
//
// Extra rules:
//   - test files (_test.go) may also import nucleo/nucleotest; only test files may;
//   - test files in an external test package (package x_test) may also import arenero and x itself;
//   - a package may import its own sub-packages, and a sub-package follows its parent's row;
//   - nothing may import a module outside the standard library, or "C";
//   - an unknown package under internal/ is an error: add it to the table first.

const modulo = "nyxcodigo"

var hojas = []string{"instrumenta", "pruebas", "reparar", "analisis", "recetas", "sintesis", "lengua",
	"mates", "logica", "puzles", "saber", "memoria", "servidor"}

// reglas: package (path under internal/) → internal packages it may import.
func reglas() map[string][]string {
	r := map[string][]string{
		"nucleo":            {},
		"nucleo/nucleotest": {"nucleo"},
		"arenero":           {"nucleo", "instrumenta"},
	}
	todos := []string{"nucleo", "arenero", "cerebro"}
	for _, h := range hojas {
		r[h] = []string{"nucleo"}
		todos = append(todos, h)
	}
	var cerebro []string
	for _, p := range todos {
		if p != "servidor" && p != "cerebro" {
			cerebro = append(cerebro, p)
		}
	}
	r["cerebro"] = cerebro
	return r
}

// reglaMain is the row for the root package (main.go).
var reglaMain = []string{"cerebro", "servidor", "arenero", "memoria", "saber", "nucleo"}

// stdNucleo is the §3.1 list: the only packages nucleo's non-test files may import.
var stdNucleo = map[string]bool{
	"context": true, "crypto/sha256": true, "encoding/hex": true, "encoding/json": true, "errors": true,
	"fmt": true, "math": true, "math/big": true, "sort": true, "strconv": true, "strings": true, "sync": true,
	"time": true, "unicode": true, "unicode/utf8": true,
}

// propietario returns the table row that governs the package at rel (path under internal/), or "".
func propietario(rel string, r map[string][]string) string {
	if _, ok := r[rel]; ok {
		return rel
	}
	partes := strings.Split(rel, "/")
	for i := len(partes) - 1; i >= 1; i-- {
		if _, ok := r[strings.Join(partes[:i], "/")]; ok {
			return strings.Join(partes[:i], "/")
		}
	}
	return ""
}

// permitido reports whether package rel may import the internal package dest.
func permitido(rel, dest string, prueba, externo bool, r map[string][]string) bool {
	if rel == dest && externo {
		return true // package x_test imports x
	}
	if strings.HasPrefix(dest, rel+"/") {
		return dest != "nucleo/nucleotest" || prueba // own sub-packages (nucleotest only from tests)
	}
	if dest == "nucleo/nucleotest" {
		return prueba
	}
	if externo && propietario(dest, r) == "arenero" {
		return true
	}
	fila := r[propietario(rel, r)]
	destProp := propietario(dest, r)
	for _, p := range fila {
		if p == destProp {
			return true
		}
	}
	return false
}

// revisarDependencias checks the module rooted at raiz and returns one Spanish line per violation.
func revisarDependencias(raiz string) ([]string, error) {
	r := reglas()
	var malas []string
	fset := token.NewFileSet()
	revisarArchivo := func(ruta, rel string, esMain bool) error {
		f, err := parser.ParseFile(fset, ruta, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("no puedo leer %s: %v", ruta, err)
		}
		prueba := strings.HasSuffix(ruta, "_test.go")
		externo := strings.HasSuffix(f.Name.Name, "_test")
		relRuta, _ := filepath.Rel(raiz, ruta)
		for _, im := range f.Imports {
			imp, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				return err
			}
			mal := func(por string) {
				malas = append(malas, fmt.Sprintf("%s importa %q: %s", filepath.ToSlash(relRuta), imp, por))
			}
			switch {
			case imp == "C":
				mal("cgo no está permitido")
			case imp == modulo || strings.HasPrefix(imp, modulo+"/"):
				if !strings.HasPrefix(imp, modulo+"/internal/") {
					mal("solo se pueden importar paquetes de internal/")
					continue
				}
				dest := strings.TrimPrefix(imp, modulo+"/internal/")
				if propietario(dest, r) == "" {
					mal("el paquete importado no está en la tabla de §2")
					continue
				}
				if esMain {
					ok := dest == "nucleo/nucleotest" && prueba
					for _, p := range reglaMain {
						if propietario(dest, r) == p {
							ok = true
						}
					}
					if prueba && propietario(dest, r) == "arenero" {
						ok = true
					}
					if !ok {
						mal("main.go solo puede importar cerebro, servidor, arenero, memoria, saber y nucleo")
					}
					continue
				}
				if !permitido(rel, dest, prueba, externo, r) {
					mal(fmt.Sprintf("la regla de §2 no lo permite para %s", propietario(rel, r)))
				}
			case strings.Contains(strings.SplitN(imp, "/", 2)[0], "."):
				mal("solo se permite la biblioteca estándar (sin módulos de terceros)")
			default:
				if !esMain && rel == "nucleo" && !prueba && !stdNucleo[imp] {
					mal("nucleo solo puede importar la lista de §3.1")
				}
			}
		}
		return nil
	}

	interno := filepath.Join(raiz, "internal")
	err := filepath.WalkDir(interno, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		nombre := d.Name()
		if ruta != interno && (nombre == "testdata" || strings.HasPrefix(nombre, ".") || strings.HasPrefix(nombre, "_")) {
			return filepath.SkipDir
		}
		archivos, err := filepath.Glob(filepath.Join(ruta, "*.go"))
		if err != nil || len(archivos) == 0 {
			return err
		}
		rel, _ := filepath.Rel(interno, ruta)
		rel = filepath.ToSlash(rel)
		if propietario(rel, r) == "" {
			malas = append(malas, fmt.Sprintf("internal/%s: paquete desconocido; añádelo a la tabla de §2 y a deps_test.go", rel))
			return nil
		}
		sort.Strings(archivos)
		for _, a := range archivos {
			if err := revisarArchivo(a, rel, false); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	raices, err := filepath.Glob(filepath.Join(raiz, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(raices)
	for _, a := range raices {
		if err := revisarArchivo(a, "", true); err != nil {
			return nil, err
		}
	}
	return malas, nil
}

// raizModulo finds the directory of go.mod for module nyxcodigo, going up from the working directory.
func raizModulo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		datos, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(datos), "module "+modulo+"\n") {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatal("no encuentro el go.mod del módulo nyxcodigo")
		}
		dir = padre
	}
}

func TestDependencias(t *testing.T) {
	raiz := raizModulo(t)
	malas, err := revisarDependencias(raiz)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range malas {
		t.Error(m)
	}
	if _, err := os.Stat(filepath.Join(raiz, "internal", "nucleo")); err != nil {
		t.Error("falta internal/nucleo")
	}
}

// TestDependenciasDetectaViolaciones checks the checker itself on a synthetic module.
func TestDependenciasDetectaViolaciones(t *testing.T) {
	raiz := t.TempDir()
	escribir := func(rel, contenido string) {
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	imp := func(pkg string, rutas ...string) string {
		var sb strings.Builder
		sb.WriteString("package " + pkg + "\n\nimport (\n")
		for _, r := range rutas {
			sb.WriteString("\t_ " + strconv.Quote(r) + "\n")
		}
		sb.WriteString(")\n")
		return sb.String()
	}
	escribir("go.mod", "module nyxcodigo\n\ngo 1.22\n")
	// allowed
	escribir("internal/nucleo/a.go", imp("nucleo", "strings", "math/big"))
	escribir("internal/nucleo/a_test.go", imp("nucleo", "os", "math/rand"))
	escribir("internal/nucleo/nucleotest/f.go", imp("nucleotest", "go/types", "nyxcodigo/internal/nucleo"))
	escribir("internal/arenero/a.go", imp("arenero", "os/exec", "nyxcodigo/internal/nucleo", "nyxcodigo/internal/instrumenta"))
	escribir("internal/pruebas/p_test.go", imp("pruebas_test", "nyxcodigo/internal/pruebas", "nyxcodigo/internal/arenero", "nyxcodigo/internal/nucleo/nucleotest"))
	escribir("internal/lengua/datos/d.go", imp("datos", "nyxcodigo/internal/nucleo"))
	escribir("internal/lengua/l.go", imp("lengua", "nyxcodigo/internal/lengua/datos"))
	escribir("internal/cerebro/c.go", imp("cerebro", "nyxcodigo/internal/sintesis", "nyxcodigo/internal/arenero", "nyxcodigo/internal/saber"))
	escribir("internal/servidor/web/nota.txt", "sin código Go")
	escribir("internal/mates/testdata/x.go", imp("x", "github.com/ignorado/porque/testdata"))
	escribir("main.go", imp("main", "nyxcodigo/internal/cerebro", "nyxcodigo/internal/servidor", "flag"))
	// forbidden
	escribir("internal/nucleo/b.go", imp("nucleo", "os"))
	escribir("internal/mates/m.go", imp("mates", "nyxcodigo/internal/lengua"))
	escribir("internal/pruebas/q_test.go", imp("pruebas", "nyxcodigo/internal/arenero"))
	escribir("internal/sintesis/s.go", imp("sintesis", "nyxcodigo/internal/nucleo/nucleotest"))
	escribir("internal/saber/w.go", imp("saber", "github.com/x/y"))
	escribir("internal/cerebro/d.go", imp("cerebro", "nyxcodigo/internal/servidor"))
	escribir("internal/logica/c.go", imp("logica", "C"))
	escribir("internal/desconocido/z.go", imp("desconocido"))
	escribir("internal/memoria/n.go", imp("memoria", "nyxcodigo/internal/nucleo/otro"))
	escribir("internal/reparar/r.go", imp("reparar", "nyxcodigo/internal/analisis"))
	escribir("raiz.go", imp("main", "nyxcodigo/internal/sintesis"))

	malas, err := revisarDependencias(raiz)
	if err != nil {
		t.Fatal(err)
	}
	esperadas := []string{
		`internal/nucleo/b.go importa "os"`,
		`internal/mates/m.go importa "nyxcodigo/internal/lengua"`,
		`internal/pruebas/q_test.go importa "nyxcodigo/internal/arenero"`,
		`internal/sintesis/s.go importa "nyxcodigo/internal/nucleo/nucleotest"`,
		`internal/saber/w.go importa "github.com/x/y"`,
		`internal/cerebro/d.go importa "nyxcodigo/internal/servidor"`,
		`internal/logica/c.go importa "C"`,
		`internal/desconocido: paquete desconocido`,
		`internal/reparar/r.go importa "nyxcodigo/internal/analisis"`,
		`raiz.go importa "nyxcodigo/internal/sintesis"`,
	}
	todas := strings.Join(malas, "\n")
	for _, e := range esperadas {
		if !strings.Contains(todas, e) {
			t.Errorf("no se detectó: %s\nviolaciones:\n%s", e, todas)
		}
	}
	// nucleo/otro inherits nucleo's row (memoria may import nucleo and its sub-packages), so it is allowed
	if len(malas) != len(esperadas) {
		t.Errorf("hay %d violaciones y esperaba %d:\n%s", len(malas), len(esperadas), todas)
	}
}
