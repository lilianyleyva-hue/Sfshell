package recetas

// Regression tests for the problems a reviewer confirmed (§4.6): Renombrar must leave the recipe alone when
// the new name clashes with anything in the file, and Envolver must generate a program that compiles
// whatever the wrapped code is called.

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// Renombrar used to rename into a field of the receiver, an import name or init, giving code that did not
// compile while reporting the new name.
func TestRegresionRenombrarConflictos(t *testing.T) {
	casos := []struct{ receta, nombre, motivo string }{
		{"Area", "Ancho", "campo del receptor"},
		{"Area", "Perimetro", "otro método del mismo tipo"},
		{"Distancia", "math", "nombre de un import"},
		{"Distancia", "init", "init"},
		{"Distancia", "main", "main"},
		{"EsPrimo", "len", "nombre predeclarado"},
		{"EsPrimo", "n", "parámetro de la propia función"},
	}
	for _, c := range casos {
		r := receta(t, c.receta)
		out := Renombrar(r, c.nombre)
		if out.Nombre != r.Nombre || out.Firma.Nombre != r.Firma.Nombre || out.Codigo != r.Codigo {
			t.Errorf("Renombrar(%s, %q) (%s) debía dejar la receta igual; dio %s:\n%s", c.receta, c.nombre, c.motivo, out.Nombre, out.Codigo)
		}
	}
	// and a free name still works, with code that parses and keeps the signature
	out := Renombrar(receta(t, "Distancia"), "Separacion")
	if out.Nombre != "Separacion" || !strings.Contains(out.Codigo, "func Separacion(") {
		t.Fatalf("Renombrar libre:\n%s", out.Codigo)
	}
	if _, err := FirmaDe(out.Codigo, "Separacion"); err != nil {
		t.Error(err)
	}
}

// Envolver used fixed names (entrada, e0/v0/r0, err, envLinea, fmt…) without checking them against the
// wrapped code, so a function or helper with one of those names gave a program that did not compile.
func TestRegresionEnvolverNombres(t *testing.T) {
	entero := nucleo.TInt
	doble := func(nombre string) nucleo.Firma {
		return nucleo.Firma{Nombre: nombre, Params: []nucleo.Param{{Nombre: "x", Tipo: entero}}, Res: []nucleo.Tipo{entero}}
	}
	type caso struct {
		nombre string // directory
		fuente string
		firma  nucleo.Firma
		dentro []string // texts the program must contain
		fuera  []string // texts it must not contain
	}
	var casos []caso
	for _, n := range []string{"entrada", "err", "e0", "v0", "r0", "x", "i", "fila", "f", "envLinea", "envUnir", "envoltorio"} {
		casos = append(casos, caso{nombre: "f_" + n, fuente: "package solucion\n\nfunc " + n + "(x int) int { return x * 2 }\n", firma: doble(n)})
	}
	for _, n := range []string{"fmt", "strings", "bufio", "os", "strconv"} {
		casos = append(casos, caso{nombre: "f_" + n, fuente: "package solucion\n\nfunc " + n + "(x int) int { return x * 2 }\n", firma: doble(n),
			dentro: []string{"env" + strings.ToUpper(n[:1]) + n[1:] + " \"" + n + "\""}})
	}
	casos = append(casos,
		caso{nombre: "ayudante_env", fuente: "package solucion\n\nfunc envLinea(x int) int { return x }\n\nfunc Doble(x int) int { return envLinea(x) * 2 }\n", firma: doble("Doble"),
			dentro: []string{"func nyxLinea("}, fuera: []string{"func envLinea(r "}},
		caso{nombre: "var_entrada", fuente: "package solucion\n\nvar entrada = 3\n\nfunc Doble(x int) int { return entrada * x }\n", firma: doble("Doble")},
		caso{nombre: "tipo_env", fuente: "package solucion\n\ntype envNumero int\n\nfunc Doble(x envNumero) envNumero { return x * 2 }\n",
			firma:  nucleo.Firma{Nombre: "Doble", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.Tipo{Clase: nucleo.CInt, Nombre: "envNumero", Definido: true}}}, Res: []nucleo.Tipo{{Clase: nucleo.CInt, Nombre: "envNumero", Definido: true}}},
			dentro: []string{"envNumero(", "nyxEntero("}},
		caso{nombre: "alias_fmt", fuente: "package solucion\n\nimport f \"fmt\"\n\nfunc Doble(x int) int { _ = f.Sprint(x); return x * 2 }\n", firma: doble("Doble"),
			dentro: []string{"f \"fmt\"", "\"fmt\"\n", "fmt.Fprintln("}, fuera: []string{"f.Fprintln("}},
		caso{nombre: "alias_s", fuente: "package solucion\n\nimport s \"strings\"\n\nfunc Largo(xs []string) int { return len(s.Join(xs, \"\")) }\n",
			firma: nucleo.Firma{Nombre: "Largo", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TString)}}, Res: []nucleo.Tipo{entero}}},
		caso{nombre: "lista_x", fuente: "package solucion\n\ntype x []int\n\nfunc Suma(xs x) int {\n\tt := 0\n\tfor _, v := range xs {\n\t\tt += v\n\t}\n\treturn t\n}\n",
			firma: nucleo.Firma{Nombre: "Suma", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.Tipo{Clase: nucleo.CLista, Nombre: "x", Definido: true, Elem: &entero}}}, Res: []nucleo.Tipo{entero}}},
	)
	programas := map[string]string{}
	for _, c := range casos {
		prog, aCaso, err := Envolver(c.fuente, c.firma)
		if err != nil {
			t.Errorf("%s: %v", c.nombre, err)
			continue
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "main.go", prog, 0); err != nil {
			t.Errorf("%s: el programa no se puede leer: %v\n%s", c.nombre, err, prog)
			continue
		}
		for _, d := range c.dentro {
			if !strings.Contains(prog, d) {
				t.Errorf("%s: falta %q:\n%s", c.nombre, d, prog)
			}
		}
		for _, f := range c.fuera {
			if strings.Contains(prog, f) {
				t.Errorf("%s: sobra %q:\n%s", c.nombre, f, prog)
			}
		}
		if _, ok := aCaso(nucleo.Caso{Entradas: []nucleo.Valor{3}, Esperado: []nucleo.Valor{6}}); !ok && c.nombre != "lista_x" && c.nombre != "alias_s" {
			t.Errorf("%s: aCaso falló", c.nombre)
		}
		programas[c.nombre] = prog
	}
	// a redeclared predeclared name is an honest error, not a broken program
	if _, _, err := Envolver("package solucion\n\nfunc len(x int) int { return x }\n\nfunc Doble(x int) int { return len(x) * 2 }\n", doble("Doble")); err == nil || !strings.Contains(err.Error(), "len") {
		t.Errorf("declarar len debe dar un error claro: %v", err)
	}
	// and every program really compiles and works (real toolchain; skipped without go)
	m := nuevoModulo(t)
	for nombre, prog := range programas {
		m.escribir("nombres/"+nombre+"/main.go", prog)
	}
	bin := m.construir()
	m.vet()
	for _, nombre := range []string{"f_entrada", "f_envLinea", "f_fmt", "ayudante_env", "tipo_env"} {
		out, errOut, err := ejecutar(bin+"/"+nombre, nil, "21\n")
		if err != nil || out != "42\n" {
			t.Errorf("%s: salida %q, error %v, stderr %q", nombre, out, err, errOut)
		}
	}
	if out, _, err := ejecutar(bin+"/lista_x", nil, "1 2 3\n"); err != nil || out != "6\n" {
		t.Errorf("lista_x: salida %q, error %v", out, err)
	}
	if out, _, err := ejecutar(bin+"/alias_s", nil, "ab cde\n"); err != nil || out != "5\n" {
		t.Errorf("alias_s: salida %q, error %v", out, err)
	}
}
