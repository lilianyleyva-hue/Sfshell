package cerebro

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/saber"
)

// paginaCadena is a fake web page about reversing a string: a nondeterministic function (map
// iteration order) that must not pass, a good one, and a program.
const paginaCadena = `<html><head><title>Invertir una cadena en Go</title></head><body>
<h1>Invertir una cadena</h1>
<p>To reverse a string in Go convert it to a slice of runes and swap the ends while moving to the middle.</p>
<p>Reversing a string byte by byte breaks multi-byte characters, so always reverse the runes of the string.</p>
<p>Una cadena es una secuencia de bytes que no se puede cambiar.</p>
<pre>func invertMap(m map[string]int) map[int]string {
	out := make(map[int]string, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}</pre>
<pre>func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}</pre>
<pre>package main

import "fmt"

func main() {
	fmt.Println("hola")
}</pre>
</body></html>`

// webDePrueba is a fake internet on an httptest server: go.dev answers (so the app is online), DDG
// returns links to the server's own pages, and paginas maps paths to HTML.
func webDePrueba(t *testing.T, paginas map[string]string) *saber.Web {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if strings.HasPrefix(r.URL.Path, "/godev/") {
			fmt.Fprint(w, "<html><body><p>Go</p></body></html>")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/ddg/") {
			for ruta := range paginas {
				if strings.HasPrefix(ruta, "/web/") {
					fmt.Fprintf(w, `<div class="result"><a class="result__a" href="%s%s">Resultado</a><a class="result__snippet" href="#">texto</a></div>`, srv.URL, ruta)
				}
			}
			return
		}
		if p, ok := paginas[r.URL.Path]; ok {
			fmt.Fprint(w, p)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return saber.NuevaWeb(saber.Config{
		PermitirLocal:   true,
		BaseDDG:         srv.URL + "/ddg/",
		BaseGoByExample: srv.URL + "/gbe/",
		BasePkgGoDev:    srv.URL + "/pkg/",
		BaseGoDev:       srv.URL + "/godev/",
		BaseWikipedia:   srv.URL + "/wiki/%s/",
		Dormir:          func(time.Duration) {},
	})
}

// cerebroConWeb builds a Cerebro with a fake web and, when Go is installed (and not -short), the real
// sandbox with its own data dir and the shared build cache.
func cerebroConWeb(t *testing.T, web *saber.Web, conCaja bool) *Cerebro {
	t.Helper()
	goBin, _ := exec.LookPath("go")
	var ej nucleo.Ejecutor
	if conCaja && goBin != "" && !testing.Short() {
		cache := ""
		if dir, err := os.UserCacheDir(); err == nil {
			cache = filepath.Join(dir, "nyx-codigo-pruebas", "go-build")
		}
		a, err := arenero.Nuevo(arenero.Config{DirDatos: t.TempDir(), DirCache: cache, GoBin: goBin})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.Cerrar() })
		ej = a
	}
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mem.Cerrar() })
	c, err := Nuevo(Config{Semilla: 42, DirDatos: t.TempDir()}, Dependencias{Ej: ej, Mem: mem, Web: web, GoBin: goBin})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBuscarConWebFalsa(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroConWeb(t, webDePrueba(t, map[string]string{"/web/cadena": paginaCadena}), true)
	r, tr := preguntar(t, c, "s", "busca cómo invertir una cadena en Go", nil)
	todo := todoElTexto(r)
	if !r.Exito || r.Nivel != nucleo.NivelFuente || len(r.Fuentes) == 0 {
		t.Fatalf("respuesta: %+v\n%s", r, titulos(tr))
	}
	if !strings.Contains(todo, "Según 127.0.0.1: To reverse a string") {
		t.Errorf("los pasajes van con «Según <dominio>»:\n%s", todo)
	}
	if !strings.Contains(r.Codigo, "func reverse") {
		t.Errorf("código: %q", r.Codigo)
	}
	ok := false
	for _, cp := range r.Comprobaciones {
		if cp.Nivel == nucleo.NivelSinFallos && cp.Pasan == cp.Total && cp.Total > 50 {
			ok = true
		}
	}
	if !ok {
		t.Errorf("la función de la web se prueba con casos generados: %+v\n%s", r.Comprobaciones, titulos(tr))
	}
	if !strings.Contains(titulos(tr), "fallan") {
		t.Errorf("invertMap no es determinista (orden de un mapa) y no pasa:\n%s", titulos(tr))
	}
	if !strings.Contains(todo, "no sé qué debe dar") {
		t.Errorf("dice qué comprobó y qué no:\n%s", todo)
	}
}

func TestBuscarSinCajaSoloMuestra(t *testing.T) {
	c := cerebroConWeb(t, webDePrueba(t, map[string]string{"/web/cadena": paginaCadena}), false)
	r, _ := preguntar(t, c, "s", "busca cómo invertir una cadena en Go", nil)
	if r.Codigo == "" {
		t.Fatalf("muestra el código: %+v", r)
	}
	if cp, ok := badge(r, "Sin comprobar"); !ok || cp.Nivel != nucleo.NivelSinComprobar {
		t.Errorf("sin caja el código va «sin comprobar»: %+v", r.Comprobaciones)
	}
	if hs := c.mem.Todos(); len(hs) == 0 {
		t.Errorf("guarda los datos de la página con su fuente")
	} else if !strings.HasPrefix(hs[0].Fuente, "http") {
		t.Errorf("fuente del dato: %+v", hs[0])
	}
}

func TestBuscarSinInternet(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "busca cómo usar strings.Split en Go", nil)
	if r.Exito || !strings.Contains(r.Texto, "no tengo internet") {
		t.Fatalf("sin internet: %+v", r)
	}
	pend := c.mem.Pendientes()
	if len(pend) != 1 || pend[0].Motivo != "sin internet" {
		t.Errorf("queda pendiente para estudiar: %+v", pend)
	}
}
