package cerebro

import (
	"context"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/saber"
)

const gbeIndice = `<html><body><ul><li><a href="hello-world">Hello World</a></li><li><a href="values">Values</a></li></ul></body></html>`

const gbeHola = `<html><head><title>Go by Example: Hello World</title></head><body>
<p>Our first program will print the classic hello world message.</p>
<pre>package main

import "fmt"

func main() {
	fmt.Println("hello world")
}</pre></body></html>`

const gbeValores = `<html><head><title>Go by Example: Values</title></head><body>
<p>Go has various value types including strings, integers, floats and booleans.</p>
<pre>package main

import "fmt"

func main() {
	fmt.Println("go" + "lang")
	fmt.Println("1+1 =", 1+1)
}</pre></body></html>`

func estudiarTarea(t *testing.T, c *Cerebro) (nucleo.Respuesta, *nucleo.Traza) {
	t.Helper()
	tr := nucleo.NuevaTraza(nil, nil)
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelar()
	return c.Responder(ctx, nucleo.Peticion{Tarea: "estudio", Sesion: "s", Accion: "estudiar"}, tr), tr
}

func TestEstudiarConWebFalsa(t *testing.T) {
	web := webDePrueba(t, map[string]string{"/gbe/": gbeIndice, "/gbe/hello-world": gbeHola, "/gbe/values": gbeValores})
	c := cerebroConWeb(t, web, true)
	// a tiny index so the test does not walk GOROOT (TestEstudiarIndexa does)
	c.PonerIndice(&saber.IndiceStd{GoVersion: "prueba", Ejemplos: []saber.EjemploStd{{Paquete: "strings", Nombre: "ExampleFields"}}})
	for _, p := range []memoria.Pendiente{{Texto: "resuelve 2x+3=11", Motivo: "fallo"}, {Texto: "zork blorp fnord", Motivo: "fallo"}} {
		if err := c.mem.AgregarPendiente(p); err != nil {
			t.Fatal(err)
		}
	}
	r, tr := estudiarTarea(t, c)
	todo := todoElTexto(r)
	if !r.Exito || !strings.HasPrefix(r.Texto, "Aprendí") {
		t.Fatalf("informe: %+v\n%s", r, titulos(tr))
	}
	for _, quiero := range []string{"a resolver «resuelve 2x+3=11»", "2 páginas nuevas de Go by Example", "Aprendí a resolver 1 pregunta que antes no sabía"} {
		if !strings.Contains(todo, quiero) {
			t.Errorf("falta «%s»:\n%s\n%s", quiero, todo, titulos(tr))
		}
	}
	if c.ej != nil && !strings.Contains(todo, "2 programas comprobados") {
		t.Errorf("con caja ejecuta los programas seguros:\n%s\n%s", todo, titulos(tr))
	}
	pend := c.mem.Pendientes()
	if len(pend) != 1 || pend[0].Texto != "zork blorp fnord" || pend[0].Intentos < 2 {
		t.Errorf("la resuelta sale de pendientes; la otra queda con un intento más: %+v", pend)
	}
	if _, ok := c.mem.Pistas("Go by Example: Hello World"); !ok {
		t.Errorf("guarda pistas del código estudiado")
	}
	if !strings.Contains(titulos(tr), "Resolví 1 de 2") || !strings.Contains(titulos(tr), "Ahora sí: x = 4") {
		t.Errorf("traza:\n%s", titulos(tr))
	}
}

func TestEstudiarSinInternetNiCaja(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	c.PonerIndice(&saber.IndiceStd{GoVersion: "prueba"})
	r, _ := estudiarTarea(t, c)
	if !r.Exito || r.Texto != "Esta vez no aprendí nada nuevo." {
		t.Fatalf("informe: %+v", r)
	}
	todo := todoElTexto(r)
	if !strings.Contains(todo, "No tenía internet") || !strings.Contains(todo, "No tengo caja segura") {
		t.Errorf("dice qué no pudo hacer:\n%s", todo)
	}
}

func TestEstudiarIndexa(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroConWeb(t, nil, false)
	r, tr := estudiarTarea(t, c)
	if c.indice() == nil || len(c.indice().Ejemplos) == 0 {
		t.Fatalf("sin índice: %+v\n%s", r, titulos(tr))
	}
	if !strings.Contains(r.Texto, "ejemplos de la biblioteca estándar") {
		t.Errorf("informe: %q", r.Texto)
	}
}

// Regression (§4.14.2 estudiar step 4): a question parked "sin internet" by buscar is solved when the
// retry answers it from a source (Nivel fuente), and leaves pendientes. Without a sandbox the report
// does not say that 0 programs passed go vet.
func TestEstudiarResuelvePendienteDeBuscar(t *testing.T) {
	pagina := `<html><head><title>Split a string in Go</title></head><body>
<p>To divide a string in Go use strings.Split: it splits the string around each separator and returns a slice of strings.</p>
<p>strings.Fields divides a string around runs of white space.</p></body></html>`
	web := webDePrueba(t, map[string]string{"/web/split": pagina, "/gbe/": gbeIndice, "/gbe/hello-world": gbeHola, "/gbe/values": gbeValores})
	c := cerebroConWeb(t, web, false)
	c.PonerIndice(&saber.IndiceStd{GoVersion: "prueba", Ejemplos: []saber.EjemploStd{{Paquete: "strings", Nombre: "ExampleFields"}}})
	if err := c.mem.AgregarPendiente(memoria.Pendiente{Texto: "busca cómo dividir un texto en Go", Motivo: "sin internet"}); err != nil {
		t.Fatal(err)
	}
	r, tr := estudiarTarea(t, c)
	tt, todo := titulos(tr), todoElTexto(r)
	if pend := c.mem.Pendientes(); len(pend) != 0 {
		t.Errorf("la pregunta respondida con fuentes sale de pendientes: %+v\n%s", pend, tt)
	}
	if !strings.Contains(tt, "Resolví 1 de 1") || !strings.Contains(todo, "Aprendí a resolver 1 pregunta") {
		t.Errorf("cuenta como resuelta:\n%s\n%s", tt, todo)
	}
	if strings.Contains(tt, "pasan go vet") || strings.Contains(todo, "0 programas comprobados") {
		t.Errorf("sin caja no dice que pasó go vet:\n%s\n%s", tt, todo)
	}
	if !strings.Contains(todo, "sin ejecutar sus programas") {
		t.Errorf("dice que no ejecutó los programas:\n%s", todo)
	}
	if strings.Contains(tt, "1 ejemplos") || strings.Contains(tt, "1 intentos") || strings.Contains(tt, "1 preguntas") {
		t.Errorf("concordancia de número:\n%s", tt)
	}
}

func TestEstudiarResuelta(t *testing.T) {
	casos := []struct {
		r    nucleo.Respuesta
		pend string
		ok   bool
	}{
		{nucleo.Respuesta{Exito: true, Nivel: nucleo.NivelFuente}, "", true},
		{nucleo.Respuesta{Exito: true, Nivel: nucleo.NivelCalculo}, "", true},
		{nucleo.Respuesta{Exito: true, Nivel: nucleo.NivelFuente}, "búsqueda sin respuesta clara", false},
		{nucleo.Respuesta{Exito: true, Nivel: nucleo.NivelSinComprobar}, "", false},
		{nucleo.Respuesta{Exito: false, Nivel: nucleo.NivelFuente}, "", false},
	}
	for i, c := range casos {
		if got := estudiarResuelta(c.r, c.pend); got != c.ok {
			t.Errorf("%d: %v, quiero %v", i, got, c.ok)
		}
	}
}

// Review quality: reading the same Go by Example pages again is not "something new learned".
func TestEstudiarDosVecesNoCuentaLoRepasado(t *testing.T) {
	web := webDePrueba(t, map[string]string{"/gbe/": gbeIndice, "/gbe/hello-world": gbeHola, "/gbe/values": gbeValores})
	c := cerebroConWeb(t, web, false)
	c.PonerIndice(&saber.IndiceStd{GoVersion: "prueba", Ejemplos: []saber.EjemploStd{{Paquete: "strings", Nombre: "ExampleFields"}}})
	if err := c.mem.AgregarPendiente(memoria.Pendiente{Texto: "zork blorp fnord", Motivo: "fallo"}); err != nil {
		t.Fatal(err)
	}
	r1, tr1 := estudiarTarea(t, c)
	if !strings.HasPrefix(r1.Texto, "Aprendí 1 cosa nueva: 2 páginas nuevas de Go by Example") {
		t.Fatalf("primera vez: %q\n%s", r1.Texto, titulos(tr1))
	}
	r2, tr2 := estudiarTarea(t, c)
	if r2.Texto != "Esta vez no aprendí nada nuevo." {
		t.Errorf("segunda vez, mismas páginas y nada resuelto: %q\n%s", r2.Texto, titulos(tr2))
	}
	if todo := todoElTexto(r2); !strings.Contains(todo, "Repasé 2 páginas de Go by Example que ya había estudiado: no traían nada nuevo.") {
		t.Errorf("dice que repasó lo que ya sabía:\n%s", todo)
	}
}
