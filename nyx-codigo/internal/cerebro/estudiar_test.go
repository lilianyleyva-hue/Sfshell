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
	for _, quiero := range []string{"a resolver «resuelve 2x+3=11»", "2 páginas de Go by Example", "Aprendí a resolver 1 pregunta que antes no sabía"} {
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
