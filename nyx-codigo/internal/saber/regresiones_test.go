package saber

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Regression tests for the problems found in review.

// A stray "}" in a statement-mode block closes the synthetic func _ early; this used to panic.
func TestCandidatasSentenciasConLlaveSuelta(t *testing.T) {
	for _, txt := range []string{
		"fmt.Println(1)\n}\ntype T struct {",
		"x := 1\n}\nfunc otra() {\nfmt.Println(x)",
		"}\nvar v = 3\nfunc f() {",
	} {
		var cs []Candidata
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%q: pánico %v", txt, r)
				}
			}()
			cs = Candidatas(Documento{URL: "https://ejemplo.org", Bloques: []Bloque{{Tipo: "codigo", Texto: txt}}})
		}()
		for _, c := range cs {
			if c.EsPrograma && strings.Contains(c.Codigo, "func main") && strings.Contains(c.Codigo, "type T") {
				t.Errorf("%q: un programa de sentencias no debe arrastrar otras declaraciones:\n%s", txt, c.Codigo)
			}
		}
	}
	// a real statement list still becomes a program
	cs := Candidatas(Documento{Bloques: []Bloque{{Tipo: "codigo", Texto: "s := \"hola\"\nfmt.Println(strings.ToUpper(s))"}}})
	if len(cs) != 1 || !cs[0].EsPrograma || !strings.Contains(cs[0].Codigo, "func main()") {
		t.Fatalf("sentencias: %+v", cs)
	}
}

// Thousands of unclosed block elements used to make extraction quadratic.
func TestExtraerLinealConEtiquetasSinCerrar(t *testing.T) {
	medir := func(pagina string) time.Duration {
		inicio := time.Now()
		ExtraerDocumento([]byte(pagina), "https://ejemplo.org")
		return time.Since(inicio)
	}
	casos := map[string]func(n int) string{
		"div":     func(n int) string { return strings.Repeat("<div>", n) + "<p>texto</p>" },
		"div+txt": func(n int) string { return strings.Repeat("<div>a", n) },
		"span+p":  func(n int) string { return strings.Repeat("<span>", n) + strings.Repeat("<p>x</p>", n) },
		"li":      func(n int) string { return strings.Repeat("<b><li>", n) },
		"h2":      func(n int) string { return strings.Repeat("<h2><td>a", n) },
	}
	for nombre, gen := range casos {
		// 200 000 unclosed elements (about 1 MB) used to take minutes; now it is linear
		d := medir(gen(200000))
		if d > 4*time.Second {
			t.Errorf("%s: 200000 elementos tardan %v", nombre, d)
		}
	}
	// the result is still right inside a deep (but bounded) stack
	pagina := strings.Repeat("<div>", 1000) + "<h2>Título</h2><p>Un párrafo normal.</p>"
	doc := ExtraerDocumento([]byte(pagina), "https://ejemplo.org")
	if len(doc.Bloques) != 2 || doc.Bloques[0].Tipo != "titulo" || doc.Bloques[0].Texto != "Título" ||
		doc.Bloques[1].Tipo != "parrafo" || doc.Bloques[1].Texto != "Un párrafo normal." {
		t.Fatalf("bloques en una pila honda: %+v", doc.Bloques)
	}
	// a script on top of a full stack still hides its content
	pagina = strings.Repeat("<div>", 1000) + "<script>var secreto = 1;</script><p>Visible y bastante largo.</p>"
	doc = ExtraerDocumento([]byte(pagina), "https://ejemplo.org")
	for _, b := range doc.Bloques {
		if strings.Contains(b.Texto, "secreto") {
			t.Errorf("el script se coló: %+v", doc.Bloques)
		}
	}
	if len(doc.Bloques) != 1 || doc.Bloques[0].Texto != "Visible y bastante largo." {
		t.Errorf("tras el script: %+v", doc.Bloques)
	}
}

// Extraction honours ctx, so Traer's timeout can stop it.
func TestExtraerRespetaContexto(t *testing.T) {
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	pagina := strings.Repeat("<p>uno dos tres</p>", 5000)
	if _, err := extraer(ctx, []byte(pagina), "https://ejemplo.org", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if d, err := extraer(context.Background(), []byte(pagina), "https://ejemplo.org", ""); err != nil || len(d.Bloques) == 0 {
		t.Fatalf("sin cancelar: %v %d", err, len(d.Bloques))
	}
}

// An empty result page (DuckDuckGo's 202 bot check) must not be cached for 7 days.
func TestBuscarNoGuardaResultadoVacio(t *testing.T) {
	buena := leerFixture(t, "ddg.html")
	var golpes, modo int64 // modo 0: 202 bot check, 1: 200 without results, 2: real page
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&golpes, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch atomic.LoadInt64(&modo) {
		case 0:
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`<html><body><div class="anomaly-modal">Unfortunately, bots use DuckDuckGo too.</div></body></html>`))
		case 1:
			w.Write([]byte(`<html><body><div class="no-results">No results.</div></body></html>`))
		default:
			w.Write(buena)
		}
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, nuevoRelojFalso())
	ctx := context.Background()
	en, err := w.Buscar(ctx, "invertir texto go", 5)
	if !errors.Is(err, ErrBuscadorOcupado) || len(en) != 0 {
		t.Fatalf("202: %v %v", en, err)
	}
	atomic.StoreInt64(&modo, 1)
	en, err = w.Buscar(ctx, "invertir texto go", 5)
	if err != nil || len(en) != 0 {
		t.Fatalf("sin resultados: %v %v", en, err)
	}
	atomic.StoreInt64(&modo, 2)
	en, err = w.Buscar(ctx, "invertir texto go", 5)
	if err != nil || len(en) == 0 {
		t.Fatalf("después del 202 sigue vacío: %v %v", en, err)
	}
	if g := atomic.LoadInt64(&golpes); g != 3 {
		t.Errorf("golpes = %d, quería 3 (nada vacío en caché)", g)
	}
	// a non-empty answer is cached
	if en2, err := w.Buscar(ctx, "invertir texto go", 5); err != nil || len(en2) != len(en) || atomic.LoadInt64(&golpes) != 3 {
		t.Errorf("caché: %d %v golpes=%d", len(en2), err, atomic.LoadInt64(&golpes))
	}
}

// Shorthand numeric IPv4 hosts must not slip past the guard (they matter when a proxy resolves them).
func TestGuardiaSSRFAtajosNumericos(t *testing.T) {
	w := NuevaWeb(Config{Dormir: func(time.Duration) {}})
	ctx := context.Background()
	for _, u := range []string{"https://127.1/", "https://0x7f000001/", "https://2130706433/", "https://0177.0.0.1/",
		"https://127.0.0.1./", "https://0x7f.1/", "https://10.1/", "https://[fe80::1%25eth0]/", "https://[::ffff:127.0.0.1]/",
		"https://1.2.3.0x10/"} {
		if _, err := w.Traer(ctx, u); !errors.Is(err, ErrBloqueado) {
			t.Errorf("%s: err = %v", u, err)
		}
	}
	for _, h := range []string{"go.dev", "1e100.net", "123.example.com", "0x.example.org", "pkg.go.dev", "x0x1.com"} {
		if hostNumerico(h) {
			t.Errorf("%s no es numérico", h)
		}
	}
	for _, h := range []string{"127.1", "0x7f000001", "2130706433", "0177.0.0.1", "1.2.3.4.", "0x"} {
		if !hostNumerico(h) {
			t.Errorf("%s es numérico", h)
		}
	}
	// comprobarDestino (the proxy path) refuses them too, whether or not a proxy is configured
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	req.URL.Host = "127.1"
	if p, _ := http.ProxyFromEnvironment(req); p != nil {
		if err := w.comprobarDestino(ctx, req); !errors.Is(err, ErrBloqueado) {
			t.Errorf("comprobarDestino 127.1 con proxy: %v", err)
		}
	}
}

// Windows-1252 bytes 0x80-0x9F are curly quotes, dashes and €, not C1 controls; <meta charset> counts.
func TestAUTF8Windows1252(t *testing.T) {
	casos := []struct{ entrada, charset, quiero string }{
		{"\x93hola\x94 \x80", "windows-1252", "“hola” €"},
		{"\x93hola\x94 \x96 caf\xe9", "ISO-8859-1", "“hola” – café"},
		{"\xa4 \xbd caf\xe9", "iso-8859-15", "€ œ café"},
		{"ya es UTF-8: café “x”", "windows-1252", "ya es UTF-8: café “x”"},
		{"caf\xe9", "koi8-r", "caf\xe9"},
	}
	for _, c := range casos {
		if got := string(aUTF8([]byte(c.entrada), c.charset)); got != c.quiero {
			t.Errorf("aUTF8(%q, %s) = %q, quiero %q", c.entrada, c.charset, got, c.quiero)
		}
	}
	for pagina, quiero := range map[string]string{
		`<html><head><META CHARSET="Windows-1252"></head>`:                         "Windows-1252",
		`<meta http-equiv="Content-Type" content="text/html; charset=iso-8859-1">`: "iso-8859-1",
		`<meta charset=utf-8>`: "utf-8",
		`<p>sin meta</p>`:      "",
		strings.Repeat(" ", 5000) + `<meta charset="windows-1252">`:                                              "",
		`<meta name="description" content="x"><meta http-equiv=content-type content='text/html;charset=latin1'>`: "latin1",
	} {
		if got := charsetMeta([]byte(pagina)); !strings.EqualFold(got, quiero) {
			t.Errorf("charsetMeta(%.60q) = %q, quiero %q", pagina, got, quiero)
		}
	}
	// end to end: a page whose charset is only in <meta>
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><head><meta charset=\"windows-1252\"><title>Prueba</title></head><body>" +
			"<p>Dijo \x93hola\x94 y pag\xf3 5 \x80 \x97 un p\xe1rrafo largo.</p></body></html>"))
	}))
	defer srv.Close()
	w := webPrueba(t, srv.URL, nuevoRelojFalso())
	d, err := w.Traer(context.Background(), srv.URL+"/pagina")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Bloques) == 0 || d.Bloques[0].Texto != "Dijo “hola” y pagó 5 € — un párrafo largo." {
		t.Fatalf("bloques = %+v", d.Bloques)
	}
}

// "Buffer.Grow" (type and method, no package) finds bytes.ExampleBuffer_Grow.
func TestIndiceBuscarMetodoSinPaquete(t *testing.T) {
	ix := &IndiceStd{Ejemplos: []EjemploStd{
		{Paquete: "bytes", Nombre: "ExampleBuffer_Grow", Simbolo: "Buffer.Grow"},
		{Paquete: "strings", Nombre: "ExampleSplit", Simbolo: "Split"},
		{Paquete: "strings", Nombre: "ExampleBuilder", Simbolo: "Builder"},
	}}
	ix.preparar()
	for simbolo, quiero := range map[string]string{
		"Buffer.Grow": "ExampleBuffer_Grow", "bytes.Buffer.Grow": "ExampleBuffer_Grow", "buffer.grow": "ExampleBuffer_Grow",
		"Split": "ExampleSplit", "strings.Split": "ExampleSplit", "Builder": "ExampleBuilder",
	} {
		r := ix.Buscar(nil, simbolo, 3)
		if len(r) == 0 || r[0].Nombre != quiero {
			t.Errorf("Buscar(%q) = %+v, quiero %s", simbolo, r, quiero)
		}
	}
	if r := ix.Buscar(nil, "Buffer.Nada", 3); len(r) != 0 {
		t.Errorf("Buffer.Nada = %+v", r)
	}
}
