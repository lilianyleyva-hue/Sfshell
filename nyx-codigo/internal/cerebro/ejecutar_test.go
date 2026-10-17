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

// webEnSesion leaves a web answer with codigo in the session, as pipeBuscar does.
func webEnSesion(c *Cerebro, sesion, codigo string) {
	s := c.sesion(sesion)
	s.mu.Lock()
	defer s.mu.Unlock()
	t := Turno{Tarea: "web-" + sesion, Intencion: nucleo.IBuscar, Metodo: "web", Codigo: codigo, Texto: "busca cómo escribir un archivo en go",
		Respuesta: nucleo.Respuesta{Texto: "Esto encontré", Codigo: codigo, Metodo: "web"}}
	s.turnos = append(s.turnos, t)
	s.ctx.Codigo = codigo
	s.ctx.Intencion = nucleo.IBuscar
}

func codigoQueEscribe(marca string) string {
	return "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tos.WriteFile(" + strconvQuote(marca) + ", []byte(\"x\"), 0o600)\n\tfmt.Println(\"hola\")\n}\n"
}

// Regression (security, §7.1): web code never runs with more than PermisoAuto, even when the request comes
// confirmed: neither from the session ("ejecútalo") nor resent with the same code.
func TestEjecutarCodigoWebNuncaConPermiso(t *testing.T) {
	c := cerebroConGo(t)
	marca := filepath.Join(t.TempDir(), "marca")
	codigo := codigoQueEscribe(marca)
	webEnSesion(c, "web1", codigo)
	for _, p := range []nucleo.Peticion{
		{Sesion: "web1", Texto: "ejecútalo", Confirmado: true},
		{Sesion: "web1", Texto: "ejecútalo"},
		{Sesion: "web1", Codigo: "\n" + strings.ReplaceAll(codigo, "\t", "    "), Accion: "ejecutar", Confirmado: true},
	} {
		r, tr := pedir(t, c, p)
		if r.Exito || len(r.Riesgos) != 0 || !contiene(r, "código de internet") || r.Nivel != nucleo.NivelSinComprobar {
			t.Errorf("%+v: %s [%s] %+v\n%s\n%s", p, r.Texto, r.Nivel, r.Riesgos, strings.Join(r.Parrafos, "\n"), titulos(tr))
		}
		if _, err := os.Stat(marca); err == nil {
			t.Fatalf("el código de internet se ejecutó con permiso: %+v", p)
		}
	}
	// code derived from the web code in the session (an "arréglalo" on it) is still web code
	if !c.ejcDeLaWeb(&tarea{s: c.sesion("web1"), codigoPrevio: true}, "package main\n\nfunc main() {}\n") {
		t.Errorf("el código de la sesión viene de internet")
	}
	// the user's own code in another session is not web code
	if c.ejcDeLaWeb(&tarea{s: c.sesion("otra")}, codigo) {
		t.Errorf("en otra sesión no hay código de internet")
	}
}

func TestEjcDeLaWebPropaga(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	web := "package main\n\nimport \"os\"\n\nfunc main() {\n\tos.Remove(\"x\")\n}\n"
	webEnSesion(c, "w", web)
	s := c.sesion("w")
	arreglado := "package main\n\nimport \"os\"\n\nfunc main() {\n\t_ = os.Remove(\"x\")\n}\n"
	s.turnos = append(s.turnos, Turno{Intencion: nucleo.IReparar, Metodo: "reparacion", Codigo: arreglado, Peticion: nucleo.Peticion{Texto: "arréglalo"}})
	if !c.ejcDeLaWeb(&tarea{s: s}, arreglado) {
		t.Errorf("el arreglo de código de internet sigue siendo de internet")
	}
	mio := "package main\n\nimport \"os\"\n\nfunc main() {\n\tos.Remove(\"mi-archivo-propio\")\n}\n"
	s.turnos = append(s.turnos, Turno{Intencion: nucleo.IReparar, Metodo: "reparacion", Codigo: mio, Peticion: nucleo.Peticion{Texto: "arréglalo", Codigo: mio}})
	if c.ejcDeLaWeb(&tarea{s: s, codigoPrevio: true}, mio) {
		t.Errorf("el código que pegó el usuario no viene de internet")
	}
}

// Regression (review): the Riesgos answer carries the code the dialog sends back, also when the code was
// pasted in the message or taken from the session.
func TestEjecutarRiesgosLlevanCodigo(t *testing.T) {
	c := cerebroConGo(t)
	codigo := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tb, err := os.ReadFile(\"main.go\")\n\tfmt.Println(len(b), err != nil)\n}\n"
	r, tr := pedir(t, c, nucleo.Peticion{Sesion: "riesgos", Texto: "ejecuta esto\n" + codigo})
	if len(r.Riesgos) == 0 || !strings.Contains(r.Codigo, "os.ReadFile") {
		t.Errorf("en el mensaje: %s %+v %q\n%s", r.Texto, r.Riesgos, r.Codigo, titulos(tr))
	}
	r, tr = pedir(t, c, nucleo.Peticion{Sesion: "riesgos", Texto: "ejecútalo"})
	if len(r.Riesgos) == 0 || !strings.Contains(r.Codigo, "os.ReadFile") {
		t.Errorf("de la sesión: %s %+v %q\n%s", r.Texto, r.Riesgos, r.Codigo, titulos(tr))
	}
}

// Regression (review): with a grave violation the first answer already says it never runs (no "Si lo
// confirmas", no dialog); cgo is reported as cgo, not as a missing package.
func TestEjecutarGraveDesdeElPrincipio(t *testing.T) {
	c := cerebroConGo(t)
	marca := filepath.Join(t.TempDir(), "marca")
	link := "package main\n\nimport (\n\t\"os\"\n\t_ \"unsafe\"\n)\n\n//go:linkname nanotime runtime.nanotime\nfunc nanotime() int64\n\nfunc main() {\n\tos.WriteFile(" + strconvQuote(marca) + ", []byte(\"x\"), 0o600)\n}\n"
	cgo := "package main\n\n// #include <stdio.h>\nimport \"C\"\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hola\")\n}\n"
	for nombre, codigo := range map[string]string{"linkname": link, "cgo": cgo} {
		for _, conf := range []bool{false, true} {
			r, tr := pedir(t, c, nucleo.Peticion{Texto: "ejecuta esto", Codigo: codigo, Confirmado: conf})
			if !strings.HasPrefix(r.Texto, "No lo puedo ejecutar") || contiene(r, "Si lo confirmas") || len(r.Riesgos) != 0 || r.Exito {
				t.Errorf("%s (confirmado %v): %s %+v\n%s\n%s", nombre, conf, r.Texto, r.Riesgos, strings.Join(r.Parrafos, "\n"), titulos(tr))
			}
			if nombre == "cgo" && (!contiene(r, "cgo") || contiene(r, "No encuentro el paquete")) {
				t.Errorf("cgo: %s\n%s", r.Texto, strings.Join(r.Parrafos, "\n"))
			}
		}
	}
	if _, err := os.Stat(marca); err == nil {
		t.Errorf("se ejecutó")
	}
}

// Regression (review): a wrapped function is not run with empty stdin when nobody gives its data, and a
// program that ends with an error is not an Exito.
func TestEjecutarSinDatosYConError(t *testing.T) {
	c := cerebroConGo(t)
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "ejecútala", Codigo: "func Doble(n int) int {\n\treturn 2 * n\n}\n"})
	if r.Exito || !strings.Contains(r.Texto, "necesito sus datos") || strings.Contains(titulos(tr), "Compilo y ejecuto") || !strings.Contains(r.Codigo, "func main()") {
		t.Errorf("sin datos: %s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	codigo := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Fprintln(os.Stderr, \"primero\")\n\tfmt.Fprintln(os.Stderr, \"algo salió mal\")\n\tos.Exit(3)\n}\n"
	r, tr = pedir(t, c, nucleo.Peticion{Texto: "ejecuta esto", Codigo: codigo})
	if r.Exito || r.Nivel != nucleo.NivelFallo || !contiene(r, "«algo salió mal»") {
		t.Errorf("con error: %s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
}
