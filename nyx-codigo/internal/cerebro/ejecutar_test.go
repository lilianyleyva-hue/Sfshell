package cerebro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

func TestEjecutarConEntrada(t *testing.T) {
	c := cerebroConGo(t)
	codigo := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tvar n int\n\tfmt.Scan(&n)\n\tfmt.Println(n * n)\n}\n"
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "ejecuta esto con entrada 5\n" + codigo})
	if !r.Exito || !contiene(r, "«25»") || r.Nivel != nucleo.NivelSinFallos || len(r.Riesgos) != 0 {
		t.Errorf("%s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
}

// A function without main is wrapped into a program; "con [3,1,2]" becomes its input.
func TestEjecutarFuncionEnvuelta(t *testing.T) {
	c := cerebroConGo(t)
	codigo := "func Mayor(xs []int) int {\n\tm := xs[0]\n\tfor _, x := range xs {\n\t\tif x > m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n"
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "ejecútala con [3,1,2]", Codigo: codigo})
	if !r.Exito || !contiene(r, "«3»") || !strings.Contains(r.Codigo, "func main()") {
		t.Errorf("%s [%s]\n%s\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), r.Codigo, titulos(tr))
	}
}

// §11: code using os.ReadFile returns Riesgos and nothing runs until the user confirms.
func TestEjecutarRiesgosSinEjecutar(t *testing.T) {
	c := cerebroConGo(t)
	marca := filepath.Join(t.TempDir(), "marca")
	codigo := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tos.WriteFile(" + strconvQuote(marca) + ", []byte(\"x\"), 0o600)\n\tb, err := os.ReadFile(\"main.go\")\n\tfmt.Println(len(b), err != nil)\n}\n"
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "ejecuta esto", Codigo: codigo})
	if len(r.Riesgos) == 0 || r.Exito || !contiene(r, "necesito tu permiso") {
		t.Errorf("%s [%s] %+v\n%s", r.Texto, r.Nivel, r.Riesgos, titulos(tr))
	}
	if ts := titulos(tr); strings.Contains(ts, "Compilo y ejecuto") || strings.Contains(ts, "Terminó") {
		t.Errorf("no debe ejecutar nada sin permiso:\n%s", titulos(tr))
	}
	if _, err := os.Stat(marca); err == nil {
		t.Errorf("el programa llegó a ejecutarse")
	}
	// confirmed: it runs inside the sandbox (its working directory is a fresh temp dir)
	r, tr = pedir(t, c, nucleo.Peticion{Texto: "ejecuta esto", Codigo: codigo, Confirmado: true})
	if !r.Exito || !strings.Contains(r.Texto, "Lo ejecuté") || len(r.Riesgos) != 0 {
		t.Errorf("con permiso sí se ejecuta: %s\n%s", r.Texto, titulos(tr))
	}
}

func TestEjecutarEntradaDelTexto(t *testing.T) {
	for texto, quiero := range map[string]string{
		"ejecuta esto con entrada 5":     "5",
		"ejecútalo con la entrada: hola": "hola",
		"ejecútala con [3,1,2]":          "[3,1,2]",
		"ejecútalo con cuidado":          "",
	} {
		k := &tarea{q: &nucleo.Pregunta{Resto: texto}}
		if got := ejcEntradaDelTexto(k); got != quiero {
			t.Errorf("%q: %q, quiero %q", texto, got, quiero)
		}
	}
}

func strconvQuote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\"" }
