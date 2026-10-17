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

// Regression (§0): web code that was run and failed is never shown as "Sin comprobar · compila" with
// "no lo he ejecutado". A failing program alone is offered with a fallo badge and says that it fails.
func TestBuscarProgramaQueFallaNoSeVendeComoCompila(t *testing.T) {
	explicarNecesitaGo(t)
	pagina := "<html><head><title>Índices</title></head><body><pre>package main\n\nimport \"fmt\"\n\nfunc main() {\n\tvar xs []int\n\tfmt.Println(xs[3])\n}</pre></body></html>"
	c := cerebroConWeb(t, webDePrueba(t, map[string]string{"/web/prog": pagina}), true)
	r, tr := preguntar(t, c, "s", "busca cómo leer el cuarto elemento en Go", nil)
	todo := todoElTexto(r)
	if strings.Contains(todo, "no lo he ejecutado") || strings.Contains(todo, "no sé con qué datos llamarlo") {
		t.Errorf("dice que no lo ejecutó, pero lo ejecutó y falló:\n%s\n%s", todo, titulos(tr))
	}
	if _, ok := badge(r, "Sin comprobar · compila"); ok {
		t.Errorf("un programa que falla no lleva «Sin comprobar · compila»: %+v", r.Comprobaciones)
	}
	if !strings.Contains(r.Codigo, "xs[3]") {
		t.Fatalf("sin otra cosa que enseñar, muestra el código que falla: %+v\n%s", r, titulos(tr))
	}
	if r.Nivel != nucleo.NivelFallo || r.Exito || !strings.Contains(todo, "falla") {
		t.Errorf("lo enseña como fallo: nivel %s éxito %v\n%s", r.Nivel, r.Exito, todo)
	}
	if cp, ok := badge(r, "Lo ejecuté y falla"); !ok || cp.Nivel != nucleo.NivelFallo || !strings.Contains(cp.Detalle, "index out of range") {
		t.Errorf("insignia de fallo con el motivo: %+v", r.Comprobaciones)
	}
}

// Regression (§0): a web function that fails some generated cases is not offered next to good passages,
// and the answer says that it was run and fails.
func TestBuscarFuncionQueFallaNoSeOfrece(t *testing.T) {
	explicarNecesitaGo(t)
	pagina := `<html><head><title>First element of a slice in Go</title></head><body>
<p>To get the first element of a slice in Go index it with zero, but check that the slice is not empty first.</p>
<pre>func Primero(xs []int) int {
	return xs[0]
}</pre></body></html>`
	c := cerebroConWeb(t, webDePrueba(t, map[string]string{"/web/primero": pagina}), true)
	r, tr := preguntar(t, c, "s", "busca cómo obtener el primer elemento de un slice en Go", nil)
	todo := todoElTexto(r)
	if !strings.Contains(titulos(tr), "fallan") {
		t.Fatalf("la función se prueba y falla con la lista vacía:\n%s", titulos(tr))
	}
	if strings.Contains(todo, "no lo he ejecutado") || strings.Contains(todo, "falló al probarlo") {
		t.Errorf("contradice lo que hizo:\n%s", todo)
	}
	if _, ok := badge(r, "Sin comprobar · compila"); ok {
		t.Errorf("una función que falla no lleva «Sin comprobar · compila»: %+v", r.Comprobaciones)
	}
	if strings.Contains(r.Codigo, "func Primero") {
		t.Errorf("con frases buenas no ofrece código que falla: %q", r.Codigo)
	}
	if !strings.Contains(todo, "lo probé y falla") || !strings.Contains(todo, "falla en") {
		t.Errorf("dice que lo probó y falla:\n%s", todo)
	}
	if r.Nivel != nucleo.NivelFuente || !r.Exito {
		t.Errorf("las frases siguen valiendo como fuente: %s %v", r.Nivel, r.Exito)
	}
}

func TestCuantasConcuerda(t *testing.T) {
	if got := cuantas(1, "fuente", "fuentes"); got != "1 fuente" {
		t.Errorf("singular: %q", got)
	}
	if got := cuantas(0, "página", "páginas"); got != "0 páginas" {
		t.Errorf("cero: %q", got)
	}
	if got := cuantas(3, "intento", "intentos"); got != "3 intentos" {
		t.Errorf("plural: %q", got)
	}
	if got := buscarMejores(1); got != "la mejor página" {
		t.Errorf("mejores: %q", got)
	}
}

// Regression: with one source the badge reads "Según 1 fuente de la web".
func TestBuscarUnaFuenteEnSingular(t *testing.T) {
	c := cerebroConWeb(t, webDePrueba(t, map[string]string{"/web/cadena": paginaCadena}), false)
	r, _ := preguntar(t, c, "s", "busca cómo invertir una cadena en Go", nil)
	if len(r.Fuentes) != 1 {
		t.Skipf("esta página da %d fuentes", len(r.Fuentes))
	}
	if _, ok := badge(r, "Según 1 fuente de la web"); !ok {
		t.Errorf("insignia en singular: %+v", r.Comprobaciones)
	}
}
