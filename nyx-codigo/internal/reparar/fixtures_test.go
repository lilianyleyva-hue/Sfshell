package reparar

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Fixture is one file of testdata/rotos with its header read. It is exported (in a _test.go file) so
// that the real-toolchain tests in package reparar_test can use it.
type Fixture struct {
	Nombre  string
	Fuente  string
	Compila bool          // "// quiero: compila"
	Funcion string        // "// funcion: Maximo"
	Firma   nucleo.Firma  // built from the source for Funcion
	Casos   []nucleo.Caso // "// casos: [1,2,3] -> 6; [] -> 0"
}

// LeerFixtures reads testdata/rotos/*.go, sorted by name.
func LeerFixtures() ([]Fixture, error) {
	archivos, err := filepath.Glob(filepath.Join("testdata", "rotos", "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(archivos)
	var out []Fixture
	for _, a := range archivos {
		b, err := os.ReadFile(a)
		if err != nil {
			return nil, err
		}
		fx := Fixture{Nombre: filepath.Base(a), Fuente: string(b)}
		var casos string
		for _, l := range strings.Split(fx.Fuente, "\n") {
			l = strings.TrimSpace(l)
			if !strings.HasPrefix(l, "//") {
				break
			}
			l = strings.TrimSpace(strings.TrimPrefix(l, "//"))
			switch {
			case l == "quiero: compila":
				fx.Compila = true
			case strings.HasPrefix(l, "funcion:"):
				fx.Funcion = strings.TrimSpace(strings.TrimPrefix(l, "funcion:"))
			case strings.HasPrefix(l, "casos:"):
				casos = strings.TrimSpace(strings.TrimPrefix(l, "casos:"))
			}
		}
		if fx.Funcion != "" {
			fx.Firma, err = firmaDe(fx.Fuente, fx.Funcion)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", fx.Nombre, err)
			}
			fx.Casos, err = leerCasos(casos, fx.Firma)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", fx.Nombre, err)
			}
		}
		out = append(out, fx)
	}
	return out, nil
}

// firmaDe builds the signature of a plain function with basic, slice and map types.
func firmaDe(src, nombre string) (nucleo.Firma, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		return nucleo.Firma{}, err
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != nombre || fd.Recv != nil {
			continue
		}
		firma := nucleo.Firma{Nombre: nombre}
		for _, campo := range fd.Type.Params.List {
			t, err := nucleo.ParseTipo(types.ExprString(campo.Type))
			if err != nil {
				return firma, err
			}
			for _, n := range campo.Names {
				firma.Params = append(firma.Params, nucleo.Param{Nombre: n.Name, Tipo: t})
			}
		}
		if fd.Type.Results != nil {
			for _, campo := range fd.Type.Results.List {
				t, err := nucleo.ParseTipo(types.ExprString(campo.Type))
				if err != nil {
					return firma, err
				}
				n := max(len(campo.Names), 1)
				for i := 0; i < n; i++ {
					firma.Res = append(firma.Res, t)
				}
			}
		}
		return firma, nil
	}
	return nucleo.Firma{}, fmt.Errorf("no encuentro la función %s", nombre)
}

// leerCasos reads "in -> out; in -> out" (a single input per case is enough for the fixtures).
func leerCasos(s string, f nucleo.Firma) ([]nucleo.Caso, error) {
	var out []nucleo.Caso
	ent := f.Entradas()
	for i, parte := range strings.Split(s, ";") {
		lados := strings.SplitN(parte, "->", 2)
		if len(lados) != 2 || len(ent) != 1 || len(f.Res) != 1 {
			return nil, fmt.Errorf("caso %q: no lo entiendo", parte)
		}
		e, err := nucleo.ParseValor(lados[0], ent[0])
		if err != nil {
			return nil, err
		}
		r, err := nucleo.ParseValor(lados[1], f.Res[0])
		if err != nil {
			return nil, err
		}
		out = append(out, nucleo.Caso{Entradas: []nucleo.Valor{e}, Esperado: []nucleo.Valor{r}, Expectativa: nucleo.EspUsuario,
			Origen: nucleo.OrigenUsuario, Nota: fmt.Sprintf("tu ejemplo %d", i+1)})
	}
	return out, nil
}
