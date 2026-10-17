package cerebro

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/saber"
)

// TestProgramaFrasesSinEntrada: one run with no input reads «Lo ejecuté una vez sin errores» and
// «Lo ejecuté sin entrada y escribió …», never «1 veces» or «con «(nada)»».
func TestProgramaFrasesSinEntrada(t *testing.T) {
	run := programaRun{caso: nucleo.CasoPrograma{Expectativa: nucleo.EspNinguna}, ej: nucleo.Ejecucion{Salida: "Tabla del 13:\n13 x 1 = 13\n"}, ok: true}
	cs := programaComprobaciones([]programaRun{run}, 1)
	if len(cs) != 1 || cs[0].Texto != "Lo ejecuté una vez sin errores" || cs[0].Nivel != nucleo.NivelSinFallos {
		t.Errorf("insignia: %+v", cs)
	}
	malo := run
	malo.ok = false
	if cs := programaComprobaciones([]programaRun{malo}, 1); len(cs) != 1 || cs[0].Texto != "Se cae al ejecutarlo" {
		t.Errorf("insignia de fallo: %+v", cs)
	}
	dos := programaComprobaciones([]programaRun{run, run}, 1)
	if len(dos) != 1 || dos[0].Texto != "Lo ejecuté 2 veces sin errores" {
		t.Errorf("dos ejecuciones: %+v", dos)
	}
	pp := programaPrueba{runs: []programaRun{run}}
	frase := programaFraseRun(pp)
	if !strings.HasPrefix(frase, "Lo ejecuté sin entrada y escribió «Tabla del 13:") {
		t.Errorf("frase: %q", frase)
	}
	if e := programaEntendi(pp); !strings.HasPrefix(e, "sin entrada escribe «Tabla del 13:") {
		t.Errorf("entendí: %q", e)
	}
	for _, s := range []string{frase, programaEntendi(pp)} {
		if strings.Contains(s, "(nada)") || strings.Contains(s, "(sin entrada)") {
			t.Errorf("frase con marcador de tabla: %q", s)
		}
	}
	conEntrada := programaRun{caso: nucleo.CasoPrograma{Entrada: "2\n3\n"}, ej: nucleo.Ejecucion{Salida: "La suma es 5\n"}, ok: true}
	if f := programaFraseRun(programaPrueba{runs: []programaRun{conEntrada}}); f != "Lo ejecuté con «2⏎3⏎» y escribió «La suma es 5»." {
		t.Errorf("con entrada: %q", f)
	}
	if tb := programaTabla([]programaRun{run}); tb.Filas[0][0] != "(sin entrada)" {
		t.Errorf("tabla: %q", tb.Filas[0][0])
	}
}

// TestProgramaFraseSinSalida: an empty output is «como debía» only when a case said what to write.
func TestProgramaFraseSinSalida(t *testing.T) {
	nada := programaRun{caso: nucleo.CasoPrograma{Expectativa: nucleo.EspNinguna}, ok: true}
	if f := programaFraseRun(programaPrueba{runs: []programaRun{nada}}); f != "Lo ejecuté sin entrada y no escribió nada." {
		t.Errorf("sin expectativa: %q", f)
	}
	esperada := programaRun{caso: nucleo.CasoPrograma{Entrada: "0\n", Expectativa: nucleo.EspReferencia}, ok: true}
	if f := programaFraseRun(programaPrueba{runs: []programaRun{esperada}}); f != "Lo ejecuté con «0⏎» y no escribió nada (como debía)." {
		t.Errorf("con expectativa: %q", f)
	}
}

// webPrograma is a fake web (httptest) whose search finds one page with a whole Go program that
// greets without reading its input.
func webPrograma(t *testing.T, goBin string) *saber.Web {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case strings.HasPrefix(r.URL.Path, "/ddg"):
			destino := url.QueryEscape(srv.URL + "/codigo/saludo")
			fmt.Fprintf(w, `<html><body><div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=%s">Saludo en Go</a><a class="result__snippet">hola</a></div></body></html>`, destino)
		case r.URL.Path == "/codigo/saludo":
			fmt.Fprint(w, `<html><head><title>Saludo en Go</title></head><body><h1>Un saludo</h1>
<p>Este programa escribe un saludo en japonés.</p>
<pre><code>package main

import "fmt"

func main() {
	fmt.Println("Konnichiwa")
}
</code></pre></body></html>`)
		default:
			fmt.Fprint(w, "<html><body>ok</body></html>")
		}
	}))
	t.Cleanup(srv.Close)
	return saber.NuevaWeb(saber.Config{
		PermitirLocal: true, BaseDDG: srv.URL + "/ddg/", BaseGoByExample: srv.URL + "/gobyexample/",
		BasePkgGoDev: srv.URL + "/pkg/", BaseGoDev: srv.URL + "/godev/", BaseWikipedia: srv.URL + "/wiki/%s/",
		Dormir: func(time.Duration) {}, GoBin: goBin, DirCache: t.TempDir(),
	})
}

// PrepararPrograma and Correr of ejecutorConPlazo (crear_test.go) record their deadlines too.
func (e *ejecutorConPlazo) PrepararPrograma(ctx context.Context, fuente string, instr nucleo.OpcionesInstr, p nucleo.Permiso) (nucleo.Programa, nucleo.Compilacion, error) {
	e.anotar(ctx)
	pr, comp, err := e.Ejecutor.PrepararPrograma(ctx, fuente, instr, p)
	if pr != nil {
		pr = programaConPlazo{pr, e}
	}
	return pr, comp, err
}

func (e *ejecutorConPlazo) Compilar(ctx context.Context, fuente string) nucleo.Compilacion {
	e.anotar(ctx)
	return e.Ejecutor.Compilar(ctx, fuente)
}

func (e *ejecutorConPlazo) Vet(ctx context.Context, fuente string) nucleo.Compilacion {
	e.anotar(ctx)
	return e.Ejecutor.Vet(ctx, fuente)
}

type programaConPlazo struct {
	nucleo.Programa
	e *ejecutorConPlazo
}

func (p programaConPlazo) Correr(ctx context.Context, c nucleo.CasoPrograma) (nucleo.Ejecucion, error) {
	p.e.anotar(ctx)
	return p.Programa.Correr(ctx, c)
}

// TestProgramaWebDentroDeSuPresupuesto: the web program is searched, built, vetted and run within the
// web stage's 35 % of the budget (programaProbar reads k.ctx), and its single run is told plainly.
func TestProgramaWebDentroDeSuPresupuesto(t *testing.T) {
	necesitaGo(t)
	goBin, _ := exec.LookPath("go")
	cache := ""
	if dir, err := os.UserCacheDir(); err == nil {
		cache = filepath.Join(dir, "nyx-codigo-pruebas", "go-build")
	}
	a, err := arenero.Nuevo(arenero.Config{DirDatos: t.TempDir(), DirCache: cache, GoBin: goBin})
	if err != nil {
		t.Skipf("sin arenero: %v", err)
	}
	defer a.Cerrar()
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Cerrar()
	ej := &ejecutorConPlazo{Ejecutor: a}
	c, err := Nuevo(Config{Semilla: 42}, Dependencias{Ej: ej, Mem: mem, Web: webPrograma(t, goBin), GoBin: goBin})
	if err != nil {
		t.Fatal(err)
	}
	presupuesto := 40 * time.Second
	k := &tarea{q: analizarCrear("haz un programa que salude en japonés"), s: &sesion{id: "pw"},
		t: nucleo.NuevaTraza(nil, nucleotest.RespuestasFijas()), turno: &Turno{}, presupuesto: presupuesto, semilla: 42}
	k.q.ConsultaWeb = "programa saludo japones golang"
	k.raiz = k.t.Raiz()
	k.ctx, k.ctxPadre = context.Background(), context.Background()
	r, ok := programaWeb(c, k)
	if !ok {
		t.Fatalf("no usó el programa de la web:\n%s", titulos(k.t))
	}
	if k.ctx != context.Background() {
		t.Error("programaWeb debe devolver k.ctx como estaba")
	}
	if r.Nivel != nucleo.NivelSinComprobar || !contieneParrafo(r.Parrafos, "Lo ejecuté una vez y escribió «Konnichiwa».") {
		t.Errorf("respuesta: %s %q", r.Nivel, r.Parrafos)
	}
	ej.mu.Lock()
	defer ej.mu.Unlock()
	if len(ej.plazos) < 4 { // Compilar, Vet, PrepararPrograma, Vet, Correr
		t.Fatalf("llamadas al arenero: %v\n%s", ej.plazos, titulos(k.t))
	}
	tope := presupuesto*35/100 + 100*time.Millisecond
	for i, d := range ej.plazos {
		if d < 0 || d > tope {
			t.Errorf("llamada %d al arenero con plazo %v (tope %v)", i, d, tope)
		}
	}
}
