package saber

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

func leerFixture(t *testing.T, nombre string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", nombre))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// relojFalso is an injectable clock whose Dormir advances time instantly.
type relojFalso struct {
	mu       sync.Mutex
	t        time.Time
	dormidos []time.Duration
}

func nuevoRelojFalso() *relojFalso {
	return &relojFalso{t: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (r *relojFalso) Ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *relojFalso) Dormir(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dormidos = append(r.dormidos, d)
	r.t = r.t.Add(d)
}

func webPrueba(t *testing.T, base string, r *relojFalso) *Web {
	t.Helper()
	c := Config{
		DirCache: t.TempDir(), PermitirLocal: true, Agente: "NyxCodigo/1.0",
		BaseDDG: base + "/html/", BaseGoByExample: base + "/gbe/", BasePkgGoDev: base + "/pkg/",
		BaseGoDev: base + "/godev/", BaseWikipedia: base + "/wiki/%s/",
	}
	if r != nil {
		c.Reloj = r.Ahora
		c.Dormir = r.Dormir
	} else {
		c.Dormir = func(time.Duration) {}
	}
	return NuevaWeb(c)
}

// ---- DuckDuckGo ----

func TestExtraerEnlacesDDG(t *testing.T) {
	enlaces := ExtraerEnlacesDDG(leerFixture(t, "ddg.html"))
	if len(enlaces) < 8 {
		t.Fatalf("pocos enlaces: %d", len(enlaces))
	}
	quiero := []string{
		"https://www.geeksforgeeks.org/go-language/how-to-reverse-a-string-in-golang/",
		"https://stackoverflow.com/questions/1752414/how-to-reverse-a-string-in-go",
		"https://pkg.go.dev/golang.org/x/example/hello/reverse",
	}
	for i, q := range quiero {
		if enlaces[i].URL != q {
			t.Errorf("enlace %d = %q, quiero %q", i, enlaces[i].URL, q)
		}
	}
	if enlaces[1].Titulo != "How to reverse a string in Go? - Stack Overflow" {
		t.Errorf("título = %q", enlaces[1].Titulo)
	}
	if enlaces[1].Dominio != "stackoverflow.com" || enlaces[1].Peso != 0.8 {
		t.Errorf("dominio/peso = %q %v", enlaces[1].Dominio, enlaces[1].Peso)
	}
	if !strings.Contains(enlaces[3].Resumen, "Let's explore") {
		t.Errorf("resumen sin decodificar: %q", enlaces[3].Resumen)
	}
	if enlaces[0].Peso != 0.6 || enlaces[2].Peso != 1.0 {
		t.Errorf("pesos: %v %v", enlaces[0].Peso, enlaces[2].Peso)
	}
}

func TestExtraerEnlacesDDGAnunciosYDuplicados(t *testing.T) {
	pagina := `<div class="result result--ad"><a class="result__a" href="https://duckduckgo.com/y.js?ad_domain=x&amp;u3=1">Anuncio</a></div>
<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgobyexample.com%2Fmaps&amp;rut=1">Go by <b>Example</b>: Maps</a>
<a class="result__snippet" href="#">Maps are <b>Go</b>&#x27;s built-in</a></div>
<div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgobyexample.com%2Fmaps&amp;rut=2">Otra vez</a></div>
<div class="result"><a class="result__a" href="https://go.dev/blog/maps">Directo</a></div>`
	en := ExtraerEnlacesDDG([]byte(pagina))
	if len(en) != 2 {
		t.Fatalf("enlaces = %+v", en)
	}
	if en[0].URL != "https://gobyexample.com/maps" || en[0].Titulo != "Go by Example: Maps" || en[0].Resumen != "Maps are Go's built-in" {
		t.Errorf("primero = %+v", en[0])
	}
	if en[1].URL != "https://go.dev/blog/maps" {
		t.Errorf("segundo = %+v", en[1])
	}
}

func TestBuscarOrdenaPorPesoYUsaAgente(t *testing.T) {
	pagina := leerFixture(t, "ddg.html")
	var agente, consulta atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agente.Store(r.Header.Get("User-Agent"))
		consulta.Store(r.URL.Query().Get("q"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(pagina)
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	en, err := w.Buscar(context.Background(), "invertir  texto go", 5)
	if err != nil {
		t.Fatal(err)
	}
	if agente.Load() != "NyxCodigo/1.0" {
		t.Errorf("User-Agent = %v", agente.Load())
	}
	if consulta.Load() != "invertir texto go" {
		t.Errorf("q = %v", consulta.Load())
	}
	if len(en) != 5 {
		t.Fatalf("max no respetado: %d", len(en))
	}
	for i := 1; i < len(en); i++ {
		if en[i].Peso > en[i-1].Peso {
			t.Errorf("no ordenado por peso: %v", en)
		}
	}
	if en[0].Dominio != "pkg.go.dev" || en[2].Dominio != "stackoverflow.com" {
		t.Errorf("orden: %s %s %s", en[0].Dominio, en[1].Dominio, en[2].Dominio)
	}
}

// ---- extraction ----

func TestExtraerDocumentoQuitaScriptYDecodifica(t *testing.T) {
	pagina := `<html><head><title>Prueba &amp; más</title><script>alert("<p>malo</p>")</script></head>
<body><nav><p>Menú de navegación del sitio</p></nav><h1>Título</h1>
<p>Si a &lt; b entonces   b &gt; a.</p><pre>for i := 0; i &lt; 3; i++ {
	fmt.Println(i)
}</pre><footer><p>Pie de página con texto legal</p></footer></body></html>`
	d := ExtraerDocumento([]byte(pagina), "https://ejemplo.org/x")
	if d.Titulo != "Prueba & más" {
		t.Errorf("título = %q", d.Titulo)
	}
	todo := fmt.Sprint(d.Bloques)
	for _, no := range []string{"alert", "malo", "navegación", "Pie de página"} {
		if strings.Contains(todo, no) {
			t.Errorf("no debería contener %q: %s", no, todo)
		}
	}
	if len(d.Bloques) != 3 {
		t.Fatalf("bloques = %+v", d.Bloques)
	}
	if d.Bloques[0].Tipo != "titulo" || d.Bloques[1].Texto != "Si a < b entonces b > a." {
		t.Errorf("bloques = %+v", d.Bloques)
	}
	if d.Bloques[2].Tipo != "codigo" || !strings.Contains(d.Bloques[2].Texto, "i < 3") || d.Bloques[2].Contexto != "Si a < b entonces b > a." {
		t.Errorf("código = %+v", d.Bloques[2])
	}
}

func TestLimpiarCodigo(t *testing.T) {
	numerado := "1  func f() int {\n2      return 1\n3  }"
	if got := limpiarCodigo(numerado); got != "func f() int {\n    return 1\n}" {
		t.Errorf("números de línea: %q", got)
	}
	shell := "$ go run hola.go\nhola"
	if got := limpiarCodigo(shell); got != "go run hola.go\nhola" {
		t.Errorf("prompt: %q", got)
	}
	normal := "x := []int{\n1, 2}\n"
	if got := limpiarCodigo(normal); got != "x := []int{\n1, 2}" {
		t.Errorf("no debería tocar: %q", got)
	}
}

func TestStackOverflow(t *testing.T) {
	d := ExtraerDocumento(leerFixture(t, "stackoverflow.html"), "https://stackoverflow.com/questions/1752414")
	if d.Titulo != "How to reverse a string in Go? - Stack Overflow" {
		t.Errorf("título = %q", d.Titulo)
	}
	var codigos []Bloque
	for _, b := range d.Bloques {
		if b.Tipo == "codigo" {
			codigos = append(codigos, b)
		}
		if strings.Contains(b.Texto, "no soy código") || strings.Contains(b.Texto, "Site design") || strings.Contains(b.Texto, "Questions") {
			t.Errorf("bloque que debía quitarse: %+v", b)
		}
	}
	if len(codigos) != 3 {
		t.Fatalf("quiero 3 bloques de código, hay %d: %+v", len(codigos), codigos)
	}
	if codigos[0].Votos != 140 || codigos[1].Votos != 95 || codigos[2].Votos != 30 {
		t.Errorf("votos = %d %d %d", codigos[0].Votos, codigos[1].Votos, codigos[2].Votos)
	}
	if !strings.Contains(codigos[1].Texto, "i < j") || !strings.HasPrefix(codigos[1].Texto, "func reverse(s string) string {") {
		t.Errorf("código 2 = %q", codigos[1].Texto)
	}
	if !strings.Contains(codigos[1].Contexto, "rune is a builtin type") {
		t.Errorf("contexto = %q", codigos[1].Contexto)
	}
	cs := Candidatas(d)
	var rev, prog, revProg *Candidata
	for i := range cs {
		c := &cs[i]
		switch {
		case c.Funcion == "reverse":
			rev = c
		case c.EsPrograma && strings.Contains(c.Codigo, "func Reverse"):
			prog = c
		case c.Funcion == "Reverse":
			revProg = c
		}
	}
	if rev == nil || rev.Firma == nil {
		t.Fatalf("falta reverse: %+v", cs)
	}
	if rev.Firma.Go() != "func reverse(s string) string" || rev.Votos != 95 || rev.EsPrograma {
		t.Errorf("reverse = %s votos %d", rev.Firma.Go(), rev.Votos)
	}
	if !strings.HasPrefix(rev.Codigo, "package solucion") {
		t.Errorf("código normalizado:\n%s", rev.Codigo)
	}
	if prog == nil || prog.Votos != 30 || !strings.Contains(prog.Codigo, "func main()") {
		t.Fatalf("falta el programa: %+v", cs)
	}
	if revProg == nil || revProg.Firma == nil || revProg.Firma.Go() != "func Reverse(s string) string" {
		t.Fatalf("falta Reverse: %+v", revProg)
	}
	if strings.Contains(revProg.Codigo, `"fmt"`) || strings.Contains(revProg.Codigo, "func main") {
		t.Errorf("la función no debe llevar main ni imports sin usar:\n%s", revProg.Codigo)
	}
	// the statement snippet of the question becomes a program that imports fmt
	var preg *Candidata
	for i := range cs {
		if cs[i].EsPrograma && cs[i].Votos == 140 {
			preg = &cs[i]
		}
	}
	if preg == nil || !strings.Contains(preg.Codigo, `import "fmt"`) {
		t.Errorf("programa de la pregunta: %+v", preg)
	}
	for _, c := range cs {
		if _, err := parser.ParseFile(token.NewFileSet(), "", c.Codigo, 0); err != nil {
			t.Errorf("candidata que no se analiza: %v\n%s", err, c.Codigo)
		}
	}
	// dedupe: the same document twice gives the same candidates
	d2 := d
	d2.Bloques = append(append([]Bloque(nil), d.Bloques...), d.Bloques...)
	if n := len(Candidatas(d2)); n != len(cs) {
		t.Errorf("duplicados: %d vs %d", n, len(cs))
	}
}

// ---- gobyexample ----

func servidorFixtures(t *testing.T, golpes *int64) *httptest.Server {
	t.Helper()
	rutas := map[string]struct{ archivo, tipo string }{
		"/gbe/":                 {"gobyexample-indice.html", "text/html"},
		"/gbe/string-functions": {"gobyexample-string-functions.html", "text/html"},
		"/pkg/strings":          {"pkggodev-strings.html", "text/html; charset=utf-8"},
		"/wiki/es/Go_(lenguaje_de_programación)": {"wikipedia-go.json", "application/json"},
		"/so": {"stackoverflow.html", "text/html"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if golpes != nil {
			atomic.AddInt64(golpes, 1)
		}
		f, ok := rutas[r.URL.Path]
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/gbe/") {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, "<html><head><title>Go by Example: %s</title></head><body><p>Página %s de prueba con texto suficiente.</p></body></html>", r.URL.Path, r.URL.Path)
				return
			}
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", f.tipo)
		w.Write(leerFixture(t, f.archivo))
	})
	return httptest.NewServer(mux)
}

func TestGoByExample(t *testing.T) {
	srv := servidorFixtures(t, nil)
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	ctx := context.Background()
	temas, err := w.TemasGoByExample(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(temas) < 50 {
		t.Fatalf("pocos temas: %d", len(temas))
	}
	if temas[0].Titulo != "Hello World" || temas[0].URL != srv.URL+"/gbe/hello-world" {
		t.Errorf("primer tema = %+v", temas[0])
	}
	d, err := w.Traer(ctx, srv.URL+"/gbe/string-functions")
	if err != nil {
		t.Fatal(err)
	}
	if d.Titulo != "Go by Example: String Functions" {
		t.Errorf("título = %q", d.Titulo)
	}
	var programa string
	for _, b := range d.Bloques {
		if b.Tipo == "codigo" && strings.HasPrefix(b.Texto, "package main") {
			programa = b.Texto
		}
	}
	if programa == "" || !strings.Contains(programa, `s.Contains("test", "es")`) {
		t.Fatalf("no unió los segmentos:\n%s", programa)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", programa, 0); err != nil {
		t.Fatalf("el programa unido no se analiza: %v\n%s", err, programa)
	}
	cs := Candidatas(d)
	if len(cs) == 0 || !cs[0].EsPrograma {
		t.Fatalf("candidatas = %+v", cs)
	}
	if !strings.Contains(strings.Join(cs[0].Imports, " "), "strings") {
		t.Errorf("imports = %v", cs[0].Imports)
	}
}

func TestEstudiar(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/gbe/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/gbe/":
			fmt.Fprint(w, `<p>Intro <a href="hello-world">first example</a></p><ul><li><a href="hello-world">Hello World</a></li><li><a href="values">Values</a></li><li><a href="falta">Falta</a></li></ul><a href="https://go.dev">Go</a>`)
		case "/gbe/hello-world":
			fmt.Fprint(w, `<title>Go by Example: Hello World</title><pre>package main</pre><pre>import "fmt"</pre><pre>func main() {
    fmt.Println("hello world")
}</pre><pre>$ go run hello-world.go
hello world</pre>`)
		case "/gbe/values":
			fmt.Fprint(w, `<title>Go by Example: Values</title><p>Go has various value types including strings, integers.</p>`)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	var notas []string
	docs, err := w.Estudiar(context.Background(), func(hecho, total int, nota string) {
		if total != 3 {
			t.Errorf("total = %d", total)
		}
		notas = append(notas, fmt.Sprintf("%d/%d %s", hecho, total, nota))
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || len(notas) != 3 {
		t.Fatalf("docs=%d notas=%v", len(docs), notas)
	}
	if !strings.Contains(notas[2], "No pude leer") {
		t.Errorf("notas = %v", notas)
	}
	var prog string
	for _, b := range docs[0].Bloques {
		if b.Tipo == "codigo" && strings.HasPrefix(b.Texto, "package main") {
			prog = b.Texto
		}
	}
	if !strings.Contains(prog, "fmt.Println") || strings.Contains(prog, "go run") {
		t.Errorf("programa = %q", prog)
	}
}

// ---- pkg.go.dev and Wikipedia ----

func TestPkgGoDev(t *testing.T) {
	srv := servidorFixtures(t, nil)
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	d, err := w.PkgGoDev(context.Background(), "strings.Split")
	if err != nil {
		t.Fatal(err)
	}
	todo := fmt.Sprint(d.Bloques)
	if !strings.Contains(todo, "Split slices s into all substrings separated by sep") {
		t.Errorf("falta la descripción: %s", todo)
	}
	if !strings.Contains(todo, "func Split(s, sep string) []string") {
		t.Errorf("falta la declaración: %s", todo)
	}
	if strings.Contains(todo, "SplitAfter slices") || strings.Contains(todo, "Package strings implements") {
		t.Errorf("se pasó de la sección: %s", todo)
	}
	if !strings.HasSuffix(d.URL, "/pkg/strings#Split") {
		t.Errorf("url = %q", d.URL)
	}
	ov, err := w.PkgGoDev(context.Background(), "strings")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fmt.Sprint(ov.Bloques), "Package strings implements simple functions") {
		t.Errorf("resumen del paquete: %+v", ov.Bloques)
	}
	if _, err := w.PkgGoDev(context.Background(), "strings.NoExiste"); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("símbolo inexistente: %v", err)
	}
	if _, err := w.PkgGoDev(context.Background(), "../etc"); err == nil {
		t.Error("nombre no válido aceptado")
	}
}

func TestPartirSimbolo(t *testing.T) {
	casos := map[string][2]string{
		"strings.Split":               {"strings", "Split"},
		"math/rand.Intn":              {"math/rand", "Intn"},
		"strings.Builder.WriteString": {"strings", "Builder.WriteString"},
		"strings":                     {"strings", ""},
		"encoding/json":               {"encoding/json", ""},
	}
	for in, q := range casos {
		p, a, err := partirSimbolo(in)
		if err != nil || p != q[0] || a != q[1] {
			t.Errorf("%s → %q %q %v", in, p, a, err)
		}
	}
}

func TestWikipedia(t *testing.T) {
	srv := servidorFixtures(t, nil)
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	r, err := w.Wikipedia(context.Background(), "Go (lenguaje de programación)", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Titulo != "Go (lenguaje de programación)" || !strings.HasPrefix(r.Texto, "Go es un lenguaje") ||
		!strings.HasPrefix(r.URL, "https://es.wikipedia.org/wiki/") {
		t.Errorf("resumen = %+v", r)
	}
	if _, err := w.Wikipedia(context.Background(), "Nada", "es"); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("página inexistente: %v", err)
	}
	if _, err := w.Wikipedia(context.Background(), "Go", "es.evil.com/x"); err == nil {
		t.Error("idioma no válido aceptado")
	}
	hs := ExtraerHechos(DocumentoDeResumen(r), "go")
	if len(hs) == 0 {
		t.Fatal("sin hechos")
	}
}

// ---- client guarantees ----

func TestCacheSegundoTraerSinGolpes(t *testing.T) {
	var golpes int64
	srv := servidorFixtures(t, &golpes)
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	u := srv.URL + "/so"
	d1, err := w.Traer(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := w.Traer(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if golpes != 1 {
		t.Errorf("golpes = %d, quiero 1", golpes)
	}
	if d1.Titulo != d2.Titulo || len(d1.Bloques) != len(d2.Bloques) {
		t.Error("la caché cambió el documento")
	}
	// a new client on the same directory reads the disk cache: zero hits
	w2 := NuevaWeb(w.Config())
	if _, err := w2.Traer(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if golpes != 1 {
		t.Errorf("golpes tras reabrir = %d", golpes)
	}
	// the cache stores extracted documents, never raw HTML
	filepath.WalkDir(w.Config().DirCache, func(ruta string, de os.DirEntry, err error) error {
		if err == nil && !de.IsDir() {
			b, _ := os.ReadFile(ruta)
			if strings.Contains(string(b), "<script") || strings.Contains(string(b), "<div") {
				t.Errorf("la caché guarda HTML crudo: %s", ruta)
			}
		}
		return nil
	})
	// expiry: 31 days later the page is fetched again
	r := nuevoRelojFalso()
	c := w.Config()
	c.Reloj = r.Ahora
	w3 := NuevaWeb(c)
	w3.cache.escribir("doc:"+u, entradaCache{Documento: &d1})
	r.Dormir(31 * 24 * time.Hour)
	if _, err := w3.Traer(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if golpes != 2 {
		t.Errorf("tras caducar golpes = %d", golpes)
	}
}

func TestDesalojarCache(t *testing.T) {
	dir := t.TempDir()
	c := nuevaCache(dir, time.Now)
	grande := strings.Repeat("x", 4000)
	for i := 0; i < 10; i++ {
		c.escribir(fmt.Sprintf("doc:%d", i), entradaCache{Documento: &Documento{Titulo: grande}})
		ruta := c.ruta(fmt.Sprintf("doc:%d", i))
		ts := time.Now().Add(time.Duration(i-20) * time.Hour)
		os.Chtimes(ruta, ts, ts)
	}
	c.desalojar(20000)
	var total int64
	quedan := 0
	filepath.WalkDir(dir, func(ruta string, de os.DirEntry, err error) error {
		if err == nil && !de.IsDir() {
			info, _ := de.Info()
			total += info.Size()
			quedan++
		}
		return nil
	})
	if total > 20000 || quedan == 0 || quedan == 10 {
		t.Errorf("total=%d quedan=%d", total, quedan)
	}
	if _, err := os.Stat(c.ruta("doc:9")); err != nil {
		t.Error("borró el más nuevo")
	}
	if _, err := os.Stat(c.ruta("doc:0")); err == nil {
		t.Error("no borró el más viejo")
	}
}

func TestTope2MiB(t *testing.T) {
	grande := strings.Repeat("a", 3<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if r.URL.Path == "/con-largo" {
			w.Header().Set("Content-Length", fmt.Sprint(len(grande)))
			w.Write([]byte(grande))
			return
		}
		for i := 0; i < 3; i++ { // chunked, no Content-Length
			w.Write([]byte(grande[:1<<20]))
			w.(http.Flusher).Flush()
		}
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	for _, p := range []string{"/con-largo", "/troceado"} {
		if _, err := w.Traer(context.Background(), srv.URL+p); !errors.Is(err, ErrDemasiadoGrande) {
			t.Errorf("%s: err = %v", p, err)
		}
	}
}

func TestTipoDeContenido(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG"))
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	if _, err := w.Traer(context.Background(), srv.URL+"/x.png"); !errors.Is(err, ErrTipoContenido) {
		t.Errorf("err = %v", err)
	}
}

func TestRedirecciones(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var n int
		fmt.Sscanf(r.URL.Path, "/r/%d", &n)
		if n < 10 {
			http.Redirect(w, r, fmt.Sprintf("/r/%d", n+1), http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<p>fin del camino de redirecciones</p>")
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, nil)
	if _, err := w.Traer(context.Background(), srv.URL+"/r/0"); err == nil || !strings.Contains(err.Error(), "redirecciones") {
		t.Errorf("10 redirecciones: %v", err)
	}
	if _, err := w.Traer(context.Background(), srv.URL+"/r/7"); err != nil {
		t.Errorf("3 redirecciones: %v", err)
	}
}

func TestGuardiaSSRF(t *testing.T) {
	w := NuevaWeb(Config{Dormir: func(time.Duration) {}})
	ctx := context.Background()
	for _, u := range []string{"https://127.0.0.1/", "http://10.0.0.1/", "https://10.0.0.1/", "https://localhost/x",
		"https://[::1]/", "https://169.254.169.254/latest", "https://192.168.1.1/", "ftp://go.dev/", "https://user:pw@go.dev/",
		"http://go.dev/", "https://0.0.0.0/", "https://100.64.0.1/"} {
		if _, err := w.Traer(ctx, u); !errors.Is(err, ErrBloqueado) {
			t.Errorf("%s: err = %v", u, err)
		}
	}
	// the dialer itself refuses names that resolve to private addresses
	d := &net.Dialer{Timeout: time.Second}
	if _, err := w.marcar(ctx, d, "tcp", "localhost:443"); !errors.Is(err, ErrBloqueado) {
		t.Errorf("marcar localhost: %v", err)
	}
	if _, err := w.marcar(ctx, d, "tcp", "127.0.0.1:443"); !errors.Is(err, ErrBloqueado) {
		t.Errorf("marcar 127.0.0.1: %v", err)
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700::1111", "151.101.1.69"} {
		if IPProhibida(net.ParseIP(ip)) {
			t.Errorf("%s debería estar permitida", ip)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.1.1", "::1", "fe80::1", "fc00::1", "224.0.0.1", "0.0.0.0", "::"} {
		if !IPProhibida(net.ParseIP(ip)) {
			t.Errorf("%s debería estar prohibida", ip)
		}
	}
}

func TestLimiteDDG(t *testing.T) {
	r := nuevoRelojFalso()
	var mu sync.Mutex
	var horas []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		horas = append(horas, r.Ahora())
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2F">Go</a>`)
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, r)
	for _, q := range []string{"uno", "dos", "tres"} {
		if _, err := w.Buscar(context.Background(), q, 3); err != nil {
			t.Fatal(err)
		}
	}
	if len(horas) != 3 {
		t.Fatalf("peticiones = %d", len(horas))
	}
	for i := 1; i < len(horas); i++ {
		if d := horas[i].Sub(horas[i-1]); d < IntervaloDDG {
			t.Errorf("búsquedas separadas solo %v", d)
		}
	}
	// a cached query does not count and does not wait
	antes := len(r.dormidos)
	if _, err := w.Buscar(context.Background(), "uno", 3); err != nil {
		t.Fatal(err)
	}
	if len(r.dormidos) != antes || len(horas) != 3 {
		t.Error("la búsqueda en caché esperó o pidió de nuevo")
	}
	// daily cap
	w.mu.Lock()
	w.cuentaDDG = MaxDiaDDG
	w.mu.Unlock()
	if _, err := w.Buscar(context.Background(), "cuatro", 3); !errors.Is(err, ErrLimiteDiario) {
		t.Errorf("límite diario: %v", err)
	}
	r.Dormir(24 * time.Hour)
	if _, err := w.Buscar(context.Background(), "cinco", 3); err != nil {
		t.Errorf("al día siguiente: %v", err)
	}
}

func TestLimiteOtrosHosts(t *testing.T) {
	r := nuevoRelojFalso()
	w := NuevaWeb(Config{Reloj: r.Ahora, Dormir: r.Dormir})
	esperas := []time.Duration{}
	for i := 0; i < 3; i++ {
		d, err := w.reservar("go.dev", IntervaloOtros, false)
		if err != nil {
			t.Fatal(err)
		}
		esperas = append(esperas, d)
	}
	if esperas[0] != 0 || esperas[1] != 500*time.Millisecond || esperas[2] != time.Second {
		t.Errorf("esperas = %v", esperas)
	}
}

func TestSinInternet(t *testing.T) {
	w := NuevaWeb(Config{SinInternet: true})
	ctx := context.Background()
	if _, err := w.Buscar(ctx, "go", 3); !errors.Is(err, nucleo.ErrSinInternet) {
		t.Errorf("Buscar: %v", err)
	}
	if _, err := w.Traer(ctx, "https://go.dev/"); !errors.Is(err, nucleo.ErrSinInternet) {
		t.Errorf("Traer: %v", err)
	}
	if _, err := w.PkgGoDev(ctx, "strings.Split"); !errors.Is(err, nucleo.ErrSinInternet) {
		t.Errorf("PkgGoDev: %v", err)
	}
	if _, err := w.Wikipedia(ctx, "Go", "es"); !errors.Is(err, nucleo.ErrSinInternet) {
		t.Errorf("Wikipedia: %v", err)
	}
	if _, err := w.TemasGoByExample(ctx); !errors.Is(err, nucleo.ErrSinInternet) {
		t.Errorf("Temas: %v", err)
	}
	if _, err := w.Estudiar(ctx, nil); !errors.Is(err, nucleo.ErrSinInternet) {
		t.Errorf("Estudiar: %v", err)
	}
	if w.Disponible(ctx) {
		t.Error("Disponible sin internet")
	}
}

func TestDisponible(t *testing.T) {
	var golpes int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&golpes, 1)
		if r.Method != http.MethodHead {
			t.Errorf("método %s", r.Method)
		}
	}))
	defer srv.Close()
	r := nuevoRelojFalso()
	w := webPrueba(t, srv.URL, r)
	if !w.Disponible(context.Background()) || !w.Disponible(context.Background()) {
		t.Fatal("no disponible")
	}
	if golpes != 1 {
		t.Errorf("golpes = %d (debía usar la caché de 5 min)", golpes)
	}
	r.Dormir(6 * time.Minute)
	w.Disponible(context.Background())
	if golpes != 2 {
		t.Errorf("golpes = %d tras 6 min", golpes)
	}
}

// ---- adaptation and mining ----

func comprobarTipos(t *testing.T, src string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.go", src, 0)
	if err != nil {
		t.Fatalf("no se analiza: %v\n%s", err, src)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("solucion", fset, []*ast.File{f}, nil); err != nil {
		t.Fatalf("no pasa go/types: %v\n%s", err, src)
	}
}

func firma(t *testing.T, decl string) nucleo.Firma {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "package p\n"+decl+" { panic(0) }", 0)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := firmaDe(f.Decls[0].(*ast.FuncDecl), tiposDeclarados(f))
	if err != nil {
		t.Fatal(err)
	}
	return fi
}

func candidataDe(t *testing.T, src, funcion string) Candidata {
	t.Helper()
	cs := Candidatas(Documento{URL: "https://ejemplo.org", Bloques: []Bloque{{Tipo: "codigo", Texto: src}}})
	for _, c := range cs {
		if c.Funcion == funcion {
			return c
		}
	}
	t.Fatalf("no hay candidata %s en %+v", funcion, cs)
	return Candidata{}
}

func TestAdaptarSumEven(t *testing.T) {
	c := candidataDe(t, `func sumEven(arr []int64) int64 {
	var s int64
	for _, x := range arr {
		if x%2 == 0 {
			s += x
		}
	}
	return s
}`, "sumEven")
	ads := Adaptar(c, firma(t, "func SumaPares(nums []int) int"))
	if len(ads) != 1 {
		t.Fatalf("adaptaciones = %+v", ads)
	}
	a := ads[0]
	comprobarTipos(t, a.Fuente)
	if !strings.Contains(a.Fuente, "func SumaPares(nums []int) int {") || !strings.Contains(a.Fuente, "sumEven(") {
		t.Errorf("fuente:\n%s", a.Fuente)
	}
	pasos := strings.Join(a.Pasos, "; ")
	if !strings.Contains(pasos, "convierto nums ([]int) en []int64") || !strings.Contains(pasos, "convierto el resultado de int64 a int") {
		t.Errorf("pasos = %s", pasos)
	}
}

func TestAdaptarCasos(t *testing.T) {
	casos := []struct {
		nombre, src, funcion, objetivo, paso string
	}{
		{"renombrar", "func reverse(s string) string {\n\tr := []rune(s)\n\tfor i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {\n\t\tr[i], r[j] = r[j], r[i]\n\t}\n\treturn string(r)\n}",
			"reverse", "func Invertir(s string) string", "renombro reverse a Invertir"},
		{"error", "import \"strconv\"\n\nfunc parse(s string) (int, error) {\n\treturn strconv.Atoi(s)\n}", "parse", "func Numero(s string) int", "quito el error final"},
		{"n extra", "func sum(arr []int, n int) int {\n\tt := 0\n\tfor i := 0; i < n; i++ {\n\t\tt += arr[i]\n\t}\n\treturn t\n}", "sum", "func Suma(xs []int) int", "len(xs)"},
		{"permutación", "import \"strings\"\n\nfunc repeat(n int, s string) string {\n\treturn strings.Repeat(s, n)\n}", "repeat", "func Repetir(s string, veces int) string", "orden de los parámetros"},
		{"método", "type Stack []int\n\nfunc (s Stack) Sum() int {\n\tt := 0\n\tfor _, x := range s {\n\t\tt += x\n\t}\n\treturn t\n}", "Sum", "func Suma(nums []int) int", "método Sum del tipo Stack"},
		{"puntero", "type Pila []int\n\nfunc (p *Pila) Len() int { return len(*p) }", "Len", "func Largo(nums []int) int", "método Len"},
		{"bytes", "func up(b []byte) []byte {\n\tfor i := range b {\n\t\tif b[i] >= 'a' && b[i] <= 'z' {\n\t\t\tb[i] -= 32\n\t\t}\n\t}\n\treturn b\n}", "up", "func Mayus(s string) string", "convierto s (string) en []byte"},
		{"runa", "func isVowel(c int32) bool {\n\treturn c == 'a' || c == 'e'\n}", "isVowel", "func EsVocal(r rune) bool", "convierto r (rune) en int32"},
		{"float32", "func half(x float32) float32 { return x / 2 }", "half", "func Mitad(x float64) float64", "float32"},
		{"mismo nombre", "func Suma(a, b int64) int64 { return a + b }", "Suma", "func Suma(a, b int) int", "envuelvo SumaWeb en Suma"},
		{"variádica", "func max(xs ...int) int {\n\tm := xs[0]\n\tfor _, x := range xs {\n\t\tif x > m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}", "max", "func Maximo(nums []int) int", "envuelvo max en Maximo"},
		{"mapa", "func count(words []string) map[string]int64 {\n\tm := map[string]int64{}\n\tfor _, w := range words {\n\t\tm[w]++\n\t}\n\treturn m\n}", "count", "func Frecuencia(ps []string) map[string]int", "convierto el resultado"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cand := candidataDe(t, c.src, c.funcion)
			obj := firma(t, c.objetivo)
			ads := Adaptar(cand, obj)
			if len(ads) == 0 {
				t.Fatalf("sin adaptaciones para %s", c.objetivo)
			}
			if len(ads) > maxAdaptaciones {
				t.Errorf("más de 6: %d", len(ads))
			}
			a := ads[0]
			comprobarTipos(t, a.Fuente)
			if !strings.Contains(a.Fuente, obj.Go()) && !strings.Contains(a.Fuente, "func "+obj.Nombre+"(") {
				t.Errorf("falta la función objetivo:\n%s", a.Fuente)
			}
			if !strings.Contains(strings.Join(a.Pasos, "; "), c.paso) {
				t.Errorf("pasos = %v, quiero %q\n%s", a.Pasos, c.paso, a.Fuente)
			}
		})
	}
}

func TestAdaptarImposible(t *testing.T) {
	cand := candidataDe(t, "func f(s string) string { return s }", "f")
	if ads := Adaptar(cand, firma(t, "func G(n int) int")); len(ads) != 0 {
		t.Errorf("no debería adaptar: %+v", ads)
	}
	prog := Candidata{EsPrograma: true, Codigo: "package main\nfunc main() {}"}
	if ads := Adaptar(prog, firma(t, "func G(n int) int")); ads != nil {
		t.Error("los programas no se adaptan")
	}
	// two ints: both orders are offered, identity first
	c2 := candidataDe(t, "func resta(a, b int) int { return a - b }", "resta")
	ads := Adaptar(c2, firma(t, "func Resta(x, y int) int"))
	if len(ads) != 2 || !strings.HasPrefix(ads[0].Pasos[0], "renombro") {
		t.Errorf("adaptaciones = %+v", ads)
	}
}

func TestMinar(t *testing.T) {
	p := Minar([]string{
		"func f(s string) int { return len(strings.Fields(s)) }",
		"package main\nimport \"strings\"\nfunc g(s string) []string { x := strings.Fields(s); n := 2; n %= 3; return x[:n%2] }",
		"no es código {",
	})
	if p.Llamadas["strings.Fields"] != 2 {
		t.Errorf("llamadas = %v", p.Llamadas)
	}
	if p.Operadores["%"] != 2 {
		t.Errorf("operadores = %v", p.Operadores)
	}
	encontrado := false
	for _, c := range p.Constantes {
		if c == 2 {
			encontrado = true
		}
	}
	if !encontrado {
		t.Errorf("constantes = %v", p.Constantes)
	}
}

// ---- ranking and facts ----

func TestRankear(t *testing.T) {
	docs := []Documento{
		{URL: "https://blog.ejemplo.com/a", Bloques: []Bloque{{Tipo: "parrafo", Texto: "El clima de hoy es soleado y agradable. Los gatos duermen mucho durante el día."}}},
		{URL: "https://go.dev/doc", Bloques: []Bloque{{Tipo: "parrafo", Texto: "Para invertir un mapa en Go se recorre con range y se intercambian claves y valores. Otra frase sin relación con nada de lo pedido."}}},
		{URL: "https://otro.ejemplo.com/b", Bloques: []Bloque{{Tipo: "codigo", Texto: "invertir mapa mapa mapa"}}},
	}
	ps := Rankear("cómo invertir un mapa en Go", docs, 3)
	if len(ps) == 0 || !strings.HasPrefix(ps[0].Texto, "Para invertir un mapa") || ps[0].URL != "https://go.dev/doc" {
		t.Fatalf("pasajes = %+v", ps)
	}
	for _, p := range ps {
		if strings.Contains(p.Texto, "gatos") {
			t.Errorf("pasaje irrelevante: %+v", p)
		}
	}
	if Rankear("de la el", docs, 3) != nil {
		t.Error("una consulta vacía no debería dar pasajes")
	}
}

func TestExtraerHechos(t *testing.T) {
	d := Documento{URL: "https://es.wikipedia.org/wiki/Go", Bloques: []Bloque{{Tipo: "parrafo",
		Texto: "Go es un lenguaje de programación concurrente y compilado inspirado en la sintaxis de C. Go fue creado por Robert Griesemer, Rob Pike y Ken Thompson en 2009. Go (también llamado Golang) es software libre. Python es un lenguaje interpretado."}}}
	hs := ExtraerHechos(d, "go")
	quiero := map[string]bool{
		"go|es_un|lenguaje de programación":                       false,
		"go|creado_por|Robert Griesemer, Rob Pike y Ken Thompson": false,
		"go|creado_en|2009":                                       false,
		"go|es|software libre":                                    false,
	}
	for _, h := range hs {
		k := h.Sujeto + "|" + h.Relacion + "|" + h.Objeto
		if _, ok := quiero[k]; ok {
			quiero[k] = true
		} else {
			t.Errorf("hecho inesperado %q", k)
		}
		if h.Confianza != 0.6 || h.Fuente != d.URL || h.ID == "" || h.Texto == "" {
			t.Errorf("hecho incompleto: %+v", h)
		}
	}
	for k, ok := range quiero {
		if !ok {
			t.Errorf("falta %q en %+v", k, hs)
		}
	}
	todos := ExtraerHechos(d, "")
	if len(todos) != len(hs)+1 {
		t.Errorf("sin tema: %d hechos", len(todos))
	}
	otra := d
	otra.URL = "https://go.dev/doc/faq"
	unidos := UnirHechos(hs, ExtraerHechos(otra, "go"))
	if len(unidos) != len(hs) {
		t.Fatalf("unidos = %d", len(unidos))
	}
	for _, h := range unidos {
		if h.Confianza != 0.8 {
			t.Errorf("acuerdo de dos fuentes sin subir confianza: %+v", h)
		}
	}
}

// ---- local Go knowledge (real toolchain) ----

func goroot(t *testing.T) (string, string) {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go no está instalado")
	}
	out, err := exec.Command(gobin, "env", "GOROOT", "GOVERSION").Output()
	if err != nil {
		t.Skip("go env falló")
	}
	ls := strings.Fields(string(out))
	if len(ls) < 2 {
		t.Skip("go env sin datos")
	}
	return ls[0], ls[1]
}

func TestIndexarGOROOT(t *testing.T) {
	root, version := goroot(t)
	dir := t.TempDir()
	inicio := time.Now()
	ix, err := IndexarGOROOT(root, version, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d ejemplos en %v", len(ix.Ejemplos), time.Since(inicio))
	es := ix.Buscar(nil, "strings.Split", 3)
	if len(es) == 0 || es[0].Nombre != "ExampleSplit" || es[0].Paquete != "strings" {
		t.Fatalf("Buscar strings.Split = %+v", es)
	}
	if !strings.Contains(es[0].Salida, `["a" "b" "c"]`) {
		t.Errorf("salida = %q", es[0].Salida)
	}
	prog := ix.Programa(es[0])
	if !strings.Contains(prog, "package main") || !strings.Contains(prog, "func main()") {
		t.Errorf("programa:\n%s", prog)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", prog, 0); err != nil {
		t.Errorf("programa no se analiza: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, archivoIndice)); err != nil {
		t.Errorf("no guardó la caché: %v", err)
	}
	inicio = time.Now()
	ix2, err := IndexarGOROOT(root, version, dir)
	if err != nil || len(ix2.Ejemplos) != len(ix.Ejemplos) {
		t.Fatalf("desde caché: %v", err)
	}
	t.Logf("desde caché en %v", time.Since(inicio))
	if r := ix2.Buscar([]string{"WriteString"}, "strings.Builder", 2); len(r) == 0 {
		t.Error("búsqueda por palabras vacía")
	}
	// method examples keep "Type.Method"
	if r := ix2.Buscar(nil, "strings.Builder", 1); len(r) == 0 || r[0].Simbolo != "Builder" {
		t.Errorf("Builder = %+v", r)
	}
}

func TestProgramaSinJuego(t *testing.T) {
	ix := &IndiceStd{}
	e := EjemploStd{Paquete: "strings", Nombre: "ExampleFoo", Imports: []string{`"fmt"`, `"strings"`, `"os"`},
		Codigo: "func ExampleFoo() {\n\tfmt.Println(strings.ToUpper(\"a\"))\n}"}
	p := ix.Programa(e)
	comprobarTiposMain(t, p)
	if strings.Contains(p, `"os"`) {
		t.Errorf("import sin usar:\n%s", p)
	}
	e.Codigo = "func ExampleFoo() {\n\tfmt.Println(Interna())\n}"
	if ix.Programa(e) != "" {
		t.Error("un ejemplo interno no se puede ejecutar solo")
	}
}

func comprobarTiposMain(t *testing.T, src string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.go", src, 0)
	if err != nil {
		t.Fatalf("no se analiza: %v\n%s", err, src)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("main", fset, []*ast.File{f}, nil); err != nil {
		t.Fatalf("no pasa go/types: %v\n%s", err, src)
	}
}

func TestGoDoc(t *testing.T) {
	goroot(t)
	ctx := context.Background()
	s, err := GoDoc(ctx, "", "strings.Split")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "Split") {
		t.Errorf("go doc = %q", s)
	}
	s, err = GoDoc(ctx, "", "strings")
	if err != nil || !strings.Contains(s, "func Split(") {
		t.Errorf("go doc strings = %q %v", s, err)
	}
	if _, err := GoDoc(ctx, "", "strings.NoExisteNada"); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("inexistente: %v", err)
	}
	if _, err := GoDoc(ctx, "", "-u"); err == nil {
		t.Error("aceptó una opción como símbolo")
	}
}

func TestGoDocSinGo(t *testing.T) {
	if _, err := GoDoc(context.Background(), "/no/existe/go", "strings.Split"); err == nil {
		t.Error("sin binario debería fallar")
	}
}
