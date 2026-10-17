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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/saber"
	"nyxcodigo/internal/sintesis"
)

// ---- helpers ----

func necesitaGo(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("usa el arenero real: se salta con -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sin Go")
	}
}

// preguntarCrear runs one turn in a session with a responder.
func preguntarCrear(c *Cerebro, sesion, texto string, resp nucleo.Responder) (nucleo.Respuesta, *nucleo.Traza) {
	if resp == nil {
		resp = nucleotest.RespuestasFijas()
	}
	tr := nucleo.NuevaTraza(nil, resp)
	ctx, cancelar := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancelar()
	r := c.Responder(ctx, nucleo.Peticion{Sesion: sesion, Texto: texto}, tr)
	return r, tr
}

func analizarCrear(texto string) *nucleo.Pregunta {
	return lengua.Analizar(texto, "", "", lengua.LexicoBase(), nil)
}

// ---- the golden exam rows of crear (§12) ----

// filasCrear are the golden-exam rows that go through crear_funcion and crear_programa (§12).
var filasCrear = map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true,
	10: true, 11: true, 12: true, 13: true, 14: true, 15: true, 16: true, 59: true, 60: true}

// TestCrearExamen runs the exam rows #1–#16, #59 and #60 with the real arenero, recetas, sintesis and
// memoria (in a temp dir), in exam order so that memory carries over as in the full exam.
func TestCrearExamen(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	for _, f := range leerExamen(t) {
		if !filasCrear[f.num] {
			continue
		}
		f := f
		t.Run(fmt.Sprintf("%02d", f.num), func(t *testing.T) {
			responder := nucleotest.RespuestasFijas(f.fijas...)
			sesion := fmt.Sprintf("crear-%02d", f.num)
			var r nucleo.Respuesta
			t0 := time.Now()
			for i, turno := range f.turnos {
				ctx, cancelar := context.WithTimeout(context.Background(), 150*time.Second)
				r = c.Responder(ctx, nucleo.Peticion{Tarea: fmt.Sprintf("c%02d-%d", f.num, i+1), Sesion: sesion, Texto: turno}, nucleo.NuevaTraza(nil, responder))
				cancelar()
			}
			if motivo := f.comprobar(r); motivo != "" {
				t.Errorf("#%d %q: %s\nrespuesta: %s\n%s", f.num, f.turnos[len(f.turnos)-1], motivo, r.Nivel, recortar(todoElTexto(r), 1500))
			} else {
				t.Logf("#%d bien en %s: %s [%s]", f.num, time.Since(t0).Round(time.Millisecond), primeraLinea(r.Texto), r.Nivel)
			}
			if r.Exito && r.Codigo != "" && r.Metodo != "memoria" && r.Aprendido == "" && f.categoria == "crear" {
				t.Errorf("#%d: una función comprobada debe quedar guardada (Aprendido vacío)", f.num)
			}
		})
	}
}

// ---- pure parts (no Go needed) ----

func TestCrearClaves(t *testing.T) {
	fuente := "package solucion\n\nfunc SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n%2 == 0 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total\n}\n"
	cl := crearClaves(fuente, "SumaPares")
	if len(cl) != 1 || !strings.Contains(cl[0], "«n % 2 == 0»") || !strings.Contains(cl[0], "par") {
		t.Fatalf("claves = %q", cl)
	}
	if cl := crearClaves("no es go", "X"); cl != nil {
		t.Errorf("código roto: %q", cl)
	}
	bool1 := "package solucion\n\nfunc EsMayor(a, b int) bool {\n\treturn a-b > 0\n}\n"
	if cl := crearClaves(bool1, "EsMayor"); len(cl) != 1 || !strings.Contains(cl[0], "«a - b > 0»") {
		t.Errorf("return: %q", cl)
	}
}

func TestCrearNecesariosYCompatible(t *testing.T) {
	q := analizarCrear("haz una función que sume los números impares de una lista")
	cf := &crearFlujo{necesarios: crearNecesarios(q.Marco)}
	if !cf.compatible([]string{"sumar", "impar", "numeros"}) {
		t.Errorf("SumaImpares debe encajar con %v", cf.necesarios)
	}
	if cf.compatible([]string{"sumar", "par", "filtrar", "lista", "numeros"}) {
		t.Errorf("SumaPares no debe encajar con %v («ahora con impares» no puede reutilizarla)", cf.necesarios)
	}
	if crearNecesarios(nil) != nil {
		t.Error("sin marco no hay conceptos necesarios")
	}
}

// TestCrearOrdenar: the frame decides among programs ("más de 3 letras" needs largoS and >; "cada
// palabra" needs palabras; Suma(a, b) uses both parameters).
func TestCrearOrdenar(t *testing.T) {
	reg := sintesis.Base()
	casos := []struct {
		texto  string
		exprs  []string // the first one must win
		costos []int
	}{
		{"devuelve las palabras con más de 3 letras", []string{`(filtra (λ x (> (largoS x) 3)) palabras)`, `(filtra (λ x (esPalindromoS x)) palabras)`}, []int{33, 20}},
		{"cuántas veces aparece cada palabra en un texto", []string{`(frecuencias (palabras texto))`, `(frecuenciaLetras texto)`}, []int{15, 8}},
		{"haz un programa que pida dos números y los sume", []string{`(+ a b)`, `(+ a a)`}, []int{10, 10}},
		{"haz una función que sume los números pares de una lista", []string{`(suma (filtra (λ x (esPar x)) nums))`, `(+ 1 (contar (λ x (esPar x)) nums))`}, []int{22, 27}},
	}
	for _, cs := range casos {
		q := analizarCrear(cs.texto)
		if q.Firma == nil {
			t.Fatalf("%q: sin firma", cs.texto)
		}
		cf := &crearFlujo{k: &tarea{q: q}, f: *q.Firma, necesarios: crearNecesarios(q.Marco)}
		var sols []sintesis.Solucion
		for i := len(cs.exprs) - 1; i >= 0; i-- { // worst first
			e, err := sintesis.Parse(cs.exprs[i], reg, *q.Firma)
			if err != nil {
				t.Fatalf("%s: %v", cs.exprs[i], err)
			}
			sols = append(sols, sintesis.Solucion{Expr: e, Costo: cs.costos[i]})
		}
		cf.ordenar(sols, true)
		if got := sols[0].Expr.String(); got != cs.exprs[0] {
			t.Errorf("%q: gana %s, quiero %s", cs.texto, got, cs.exprs[0])
		}
	}
}

func TestCrearConstantes(t *testing.T) {
	q := analizarCrear("devuelve las palabras con más de 3 letras")
	cs := crearConstantes(q, *q.Firma, false)
	tiene3 := false
	for _, v := range cs {
		if v == 3 {
			tiene3 = true
		}
	}
	if !tiene3 {
		t.Errorf("constantes = %v, quiero el 3", cs)
	}
	q2 := analizarCrear("[1,2,3,4] -> 6; [5,7] -> 0; [] -> 0")
	if cs := crearConstantes(q2, *q2.Firma, true); len(cs) != 0 {
		t.Errorf("con ejemplos los números de los ejemplos no son constantes: %v", cs)
	}
}

func TestCrearEntendi(t *testing.T) {
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	casos := nucleo.Sondas(f)
	var rs []nucleo.ResultadoCaso
	for i, cs := range casos {
		suma := 0
		for _, v := range cs.Entradas[0].([]nucleo.Valor) {
			if n := v.(int); n%2 == 0 {
				suma += n
			}
		}
		rs = append(rs, nucleo.ResultadoCaso{Caso: i, Ejecutado: true, OK: true, Obtenido: []nucleo.Valor{suma}})
	}
	e := crearEntendi(f, casos, rs)
	if !strings.HasPrefix(e, "Así lo entendí: SumaPares([]) = 0") || strings.Count(e, ";") != 3 || !strings.Contains(e, "SumaPares([1, 2, 3, 4]) = 6") {
		t.Errorf("Entendi = %q", e)
	}
}

func TestProgramaEncaja(t *testing.T) {
	plantilla := func(nombre string) recetas.Plantilla {
		for _, p := range recetas.Plantillas() {
			if p.Nombre == nombre {
				return p
			}
		}
		t.Fatalf("no existe la plantilla %s", nombre)
		return recetas.Plantilla{}
	}
	casos := []struct {
		texto, plantilla string
		quiero           bool
	}{
		{"haz el FizzBuzz hasta 15", "fizzbuzz", true},
		{"haz el FizzBuzz hasta 15", "primos_hasta", false},
		{"programa con la tabla de multiplicar del 7", "tabla_multiplicar", true},
		{"programa que lea una lista de números y diga la suma de los pares", "par_impar", false},
		{"programa que diga si un número es par o impar", "par_impar", true},
		{"haz un juego de adivinar el número hasta 50", "primos_hasta", false},
	}
	for _, cs := range casos {
		q := analizarCrear(cs.texto)
		lemas := crearLemas(q)
		p := plantilla(cs.plantilla)
		puntos := 0.0
		for _, h := range recetas.BuscarPlantillas(lemas, 0) {
			if h.Plantilla.Nombre == cs.plantilla {
				puntos = h.Puntos
			}
		}
		if got := programaEncaja(q, p, puntos, lemas); got != cs.quiero {
			t.Errorf("%q con %s (%s): %v, quiero %v", cs.texto, cs.plantilla, decimal(puntos), got, cs.quiero)
		}
	}
}

func TestProgramaCasosPlantilla(t *testing.T) {
	var p recetas.Plantilla
	for _, x := range recetas.Plantillas() {
		if x.Nombre == "tabla_multiplicar" {
			p = x
		}
	}
	cs := programaCasosPlantilla(p, map[string]string{"numero": "7"})
	if len(cs) != 2 {
		t.Fatalf("con 7: %d casos, quiero 2 (el del 7 y el de por defecto)", len(cs))
	}
	if cs := programaCasosPlantilla(p, map[string]string{"numero": "3"}); len(cs) != 1 || !strings.Contains(cs[0].Esperado, "3 x 9 = 27") {
		t.Errorf("con 3: %+v", cs)
	}
	for _, c := range cs {
		if c.Expectativa == nucleo.EspNinguna {
			t.Errorf("los casos de plantilla comparan con lo esperado: %+v", c)
		}
	}
}

// TestCrearSinArenero: without a sandbox nothing is claimed as checked and nothing is stored.
func TestCrearSinArenero(t *testing.T) {
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Cerrar()
	c, err := Nuevo(Config{Semilla: 7}, Dependencias{Mem: mem})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := preguntarCrear(c, "s", "haz una función que sume los números pares de una lista", nil)
	if r.Nivel != nucleo.NivelSinComprobar || r.Exito || r.Codigo == "" || r.Aprendido != "" {
		t.Errorf("sin arenero: nivel %s éxito %v aprendido %q código %d bytes", r.Nivel, r.Exito, r.Aprendido, len(r.Codigo))
	}
	if !strings.Contains(r.Texto, "Creo que") {
		t.Errorf("lo no comprobado empieza por «Creo que»: %q", r.Texto)
	}
	if n, _, _ := mem.Contar(); n != 0 {
		t.Errorf("guardó %d funciones sin comprobarlas", n)
	}
	r2, _ := preguntarCrear(c, "s2", "haz un programa que pida dos números y los sume", nil)
	if r2.Nivel != nucleo.NivelSinComprobar || !strings.Contains(r2.Texto+strings.Join(r2.Parrafos, " "), "go run main.go") {
		t.Errorf("programa sin arenero: %s %q %q", r2.Nivel, r2.Texto, r2.Parrafos)
	}
}

// TestCrearSinFirma: no signature and no answer → an honest question for the user, nothing invented.
func TestCrearSinFirma(t *testing.T) {
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Cerrar()
	c, err := Nuevo(Config{Semilla: 7}, Dependencias{Mem: mem})
	if err != nil {
		t.Fatal(err)
	}
	var preguntas []string
	resp := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		preguntas = append(preguntas, p.Texto)
		return "", nucleo.ErrSinRespuesta
	}
	k := &tarea{q: &nucleo.Pregunta{Texto: "haz una función"}, s: &sesion{id: "x"}, t: nucleo.NuevaTraza(nil, resp), turno: &Turno{}}
	k.raiz = k.t.Raiz()
	k.ctx, k.ctxPadre = context.Background(), context.Background()
	r := pipeCrearFuncion(c, k)
	if r.Exito || r.Codigo != "" || !strings.Contains(r.Texto, "No sé qué datos recibe") {
		t.Errorf("sin firma: %+v", r)
	}
	if len(preguntas) != 1 || preguntas[0] != "¿Qué recibe la función?" {
		t.Errorf("debía preguntar qué recibe la función: %q", preguntas)
	}
}

// ---- with the real sandbox ----

// TestCrearPreguntaDistinguir: two programs fit "[3,1,2] -> 3"; Nyx asks what another input must give
// and the answer (the maximum) becomes a user example.
func TestCrearPreguntaDistinguir(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	reLista := regexp.MustCompile(`\(\[([^\]]*)\]\)`)
	preguntas := 0
	resp := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		preguntas++
		m := reLista.FindStringSubmatch(p.Texto)
		if m == nil {
			return "", nucleo.ErrSinRespuesta
		}
		mayor := 0
		for i, s := range strings.Split(m[1], ",") {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", nucleo.ErrSinRespuesta
			}
			if i == 0 || n > mayor {
				mayor = n
			}
		}
		return strconv.Itoa(mayor), nil
	}
	r, tr := preguntarCrear(c, "d", "[3,1,2] -> 3", resp)
	if preguntas == 0 {
		t.Fatalf("no preguntó nada:\n%s", titulos(tr))
	}
	if r.Nivel != nucleo.NivelTusEjemplos || !r.Exito {
		t.Fatalf("nivel %s: %s", r.Nivel, todoElTexto(r))
	}
	if !strings.Contains(todoElTexto(r), "tus "+strconv.Itoa(1+preguntas)+" ejemplos") {
		t.Errorf("cada respuesta es un ejemplo tuyo más:\n%s", recortar(todoElTexto(r), 1200))
	}
	if !strings.Contains(strings.Join(r.Justificacion, "\n"), "maxL") {
		t.Errorf("debía quedarse con el máximo: %q", r.Justificacion)
	}
}

// TestCrearSeguimientos: otra forma shows a different program; está mal with an example re-runs with it
// as a user example; hazlo programa wraps the function and runs the program.
func TestCrearSeguimientos(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	r1, _ := preguntarCrear(c, "s", "devuelve las palabras con más de 3 letras", nil)
	if !r1.Exito || r1.Nivel != nucleo.NivelEntendido || r1.Entendi == "" {
		t.Fatalf("primera: %s %s", r1.Nivel, todoElTexto(r1))
	}
	p1 := programaDe(r1)
	r2, tr2 := preguntarCrear(c, "s", "otra forma", nil)
	if !r2.Exito || programaDe(r2) == "" || programaDe(r2) == p1 {
		t.Errorf("otra forma: %q después de %q (%s)\n%s", programaDe(r2), p1, r2.Nivel, titulos(tr2))
	}
	r3, tr3 := preguntarCrear(c, "s", `está mal: con ["casa", "árbol", "sol"] debe dar ["casa", "árbol"]`, nil)
	todo := todoElTexto(r3)
	if !r3.Exito || !strings.Contains(todo, "Cumple tu ejemplo") || r3.Nivel != nucleo.NivelTusEjemplos {
		t.Errorf("está mal: %s\n%s\n%s", r3.Nivel, recortar(todo, 1500), titulos(tr3))
	}
	r4, _ := preguntarCrear(c, "s", "hazlo programa", nil)
	if !strings.Contains(r4.Codigo, "package main") || !strings.Contains(todoElTexto(r4), "go run main.go") || !r4.Exito {
		t.Errorf("hazlo programa: %s %q\n%s", r4.Nivel, r4.Texto, recortar(r4.Codigo, 400))
	}
	if r4.Nivel != nucleo.NivelEntendido {
		t.Errorf("el programa se compara con la función: nivel %s", r4.Nivel)
	}
}

func programaDe(r nucleo.Respuesta) string {
	for _, l := range r.Justificacion {
		if strings.HasPrefix(l, "Mi programa: ") {
			return strings.TrimPrefix(l, "Mi programa: ")
		}
	}
	return ""
}

// TestProgramaListaDePares: "la suma de los pares" is not the par_impar template: the function is
// built (or reused) and wrapped, and the program is run against the function's cases.
func TestProgramaListaDePares(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	r, tr := preguntarCrear(c, "p", "programa que lea una lista de números y diga la suma de los pares", nil)
	if !r.Exito || !strings.Contains(r.Codigo, "package main") || !strings.Contains(r.Codigo, "%2 == 0") {
		t.Fatalf("%s %q\n%s\n%s", r.Nivel, r.Texto, recortar(r.Codigo, 600), titulos(tr))
	}
	if strings.Contains(r.Codigo, "es impar") || r.Metodo == "plantilla" {
		t.Errorf("usó la plantilla par_impar")
	}
	if !strings.Contains(strings.Join(r.Parrafos, "\n"), FraseGoRun) {
		t.Errorf("falta cómo ejecutarlo: %q", r.Parrafos)
	}
}

// TestProgramaPlantillaPregunta: a missing operation is asked with the template's buttons.
func TestProgramaPlantillaPregunta(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	r, tr := preguntarCrear(c, "q", "una calculadora", nucleotest.RespuestasFijas())
	if !r.Exito || r.Metodo != "plantilla" || !strings.Contains(todoElTexto(r), "go run main.go") {
		t.Fatalf("%s %s\n%s\n%s", r.Nivel, r.Metodo, recortar(todoElTexto(r), 800), titulos(tr))
	}
	if r.Nivel != nucleo.NivelEntendido && r.Nivel != nucleo.NivelSinFallos {
		t.Errorf("nivel %s", r.Nivel)
	}
}

// webBase64 is a fake web (httptest) whose search finds one page with a Go function that encodes base64.
func webBase64(t *testing.T, goBin string) *saber.Web {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/ddg"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			destino := url.QueryEscape(srv.URL + "/codigo/base64")
			fmt.Fprintf(w, `<html><body><div class="result"><a class="result__a" href="//duckduckgo.com/l/?uddg=%s">Base64 en Go</a><a class="result__snippet">encode</a></div></body></html>`, destino)
		case r.URL.Path == "/codigo/base64":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<html><head><title>Base64 en Go</title></head><body><h1>Codificar en base64</h1>
<p>Para codificar un texto en base64 usa el paquete encoding/base64 de la biblioteca estándar.</p>
<pre><code>import "encoding/base64"

func ToBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
</code></pre></body></html>`)
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
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

// TestCrearWeb: synthesis cannot build base64; the web (an httptest fake) has a function that does, it is
// adapted, run on the user's example and kept with its source.
func TestCrearWeb(t *testing.T) {
	necesitaGo(t)
	goBin, _ := exec.LookPath("go")
	web := webBase64(t, goBin)
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
	c, err := Nuevo(Config{Semilla: 42, Presupuestos: map[nucleo.TipoIntencion]time.Duration{nucleo.ICrearFuncion: 20 * time.Second}},
		Dependencias{Ej: a, Mem: mem, Web: web, GoBin: goBin})
	if err != nil {
		t.Fatal(err)
	}
	r, tr := preguntarCrear(c, "w", "codifica en base64: hola -> aG9sYQ==", nil)
	if r.Metodo != "web" || !r.Exito || r.Nivel != nucleo.NivelTusEjemplos || len(r.Fuentes) == 0 {
		t.Fatalf("%s %s %v\n%s\n%s", r.Metodo, r.Nivel, r.Exito, recortar(todoElTexto(r), 1200), titulos(tr))
	}
	if !strings.Contains(r.Codigo, "base64") || !strings.Contains(r.Fuentes[0].URL, "/codigo/base64") {
		t.Errorf("código o fuente: %q %+v", recortar(r.Codigo, 300), r.Fuentes)
	}
	if fs := mem.Funciones(); len(fs) == 0 || fs[0].Origen != "web" || fs[0].Fuente == "" {
		t.Errorf("debía guardarla con su origen web: %+v", fs)
	}
}

// ---- regression tests of the review fixes ----

// flujoPuro builds a crearFlujo for texto with no sandbox (c.ej nil): synthesis runs, codegen is not
// checked (sinComprobar), so it needs no Go.
func flujoPuro(t *testing.T, texto string, resp nucleo.Responder, presupuesto time.Duration) *crearFlujo {
	t.Helper()
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mem.Cerrar() })
	c, err := Nuevo(Config{Semilla: 7}, Dependencias{Mem: mem})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		resp = nucleotest.RespuestasFijas()
	}
	k := &tarea{q: analizarCrear(texto), s: &sesion{id: "x"}, t: nucleo.NuevaTraza(nil, resp), turno: &Turno{},
		presupuesto: presupuesto, semilla: 7}
	k.raiz = k.t.Raiz()
	k.ctx, k.ctxPadre = context.Background(), context.Background()
	cf := &crearFlujo{c: c, k: k}
	if _, ok := cf.entender(); !ok {
		t.Fatalf("no entendió %q", texto)
	}
	return cf
}

var reListaPregunta = regexp.MustCompile(`\(\[([^\]]*)\]\)`)

// sumaMenosLargo answers "¿Qué debe dar Funcion([…])?" with sum(nums) − len(nums).
func sumaMenosLargo(texto string) (string, bool) {
	m := reListaPregunta.FindStringSubmatch(texto)
	if m == nil {
		return "", false
	}
	suma, n := 0, 0
	for _, s := range strings.Split(m[1], ",") {
		if strings.TrimSpace(s) == "" {
			continue
		}
		x, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return "", false
		}
		suma += x
		n++
	}
	return strconv.Itoa(suma - n), true
}

// TestCrearPreguntaPausaSintesis: the synthesis clock does not run while the user thinks. The first
// answer comes after the clock (lim+1 s) would have run out and matches none of the offered outputs, so
// synthesis runs again and must still find sum − len.
func TestCrearPreguntaPausaSintesis(t *testing.T) {
	if testing.Short() {
		t.Skip("espera unos segundos: se salta con -short")
	}
	var mu sync.Mutex
	preguntas := 0
	resp := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		s, ok := sumaMenosLargo(p.Texto)
		if !ok {
			return "", nucleo.ErrSinRespuesta
		}
		mu.Lock()
		preguntas++
		primera := preguntas == 1
		mu.Unlock()
		if primera {
			select {
			case <-time.After(3500 * time.Millisecond):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return s, nil
	}
	cf := flujoPuro(t, "[3,1,2] -> 3", resp, 4*time.Second) // lim = 1.8 s, the clock 2.8 s
	h, ok := cf.sintesis()
	if !ok || !h.sinComprobar {
		t.Fatalf("no encontró el programa tras pensar el usuario 3,5 s (%d preguntas): %+v\n%s", preguntas, h.resp, titulos(cf.k.t))
	}
	todo := strings.Join(h.resp.Parrafos, "\n")
	if !strings.Contains(todo, "suma") || !strings.Contains(todo, "largo") {
		t.Errorf("debía ser suma − largo: %q", todo)
	}
	if cf.relojSint != nil {
		t.Error("el reloj de la síntesis debe quedar suelto al terminar")
	}
}

// TestCrearPreguntaSalidaPausaReloj: preguntarSalida pauses the synthesis clock.
func TestCrearPreguntaSalidaPausaReloj(t *testing.T) {
	resp := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		time.Sleep(300 * time.Millisecond)
		return "0", nil
	}
	cf := flujoPuro(t, "[3,1,2] -> 3", resp, 4*time.Second)
	r := nuevoReloj(context.Background(), 150*time.Millisecond)
	defer r.Cancelar()
	cf.relojSint = r
	v, como := cf.preguntarSalida([]nucleo.Valor{[]nucleo.Valor{0}}, []nucleo.Valor{1, 0})
	if como != salidaValor || !nucleo.Igual(v, 0) {
		t.Fatalf("respuesta %v (%d)", v, como)
	}
	if r.Err() != nil {
		t.Errorf("el reloj de la síntesis corrió mientras el usuario pensaba: %v", r.Err())
	}
}

// TestCrearOtraCosa: «Otra cosa» is not "no answer": Nyx asks for the value as free text; if none comes,
// no program that gives one of the rejected outputs is shown, and there is no «Había otra forma…».
func TestCrearOtraCosa(t *testing.T) {
	type pregunta struct {
		texto    string
		opciones []string
	}
	var mu sync.Mutex
	var hechas []pregunta
	resp := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		mu.Lock()
		hechas = append(hechas, pregunta{p.Texto, p.Opciones})
		mu.Unlock()
		if strings.HasPrefix(p.Texto, "¿Qué debe dar") {
			return "Otra cosa", nil
		}
		return "", nucleo.ErrSinRespuesta
	}
	cf := flujoPuro(t, "[3,1,2] -> 3", resp, 4*time.Second)
	h, ok := cf.sintesis()
	if len(hechas) < 2 || !strings.HasPrefix(hechas[1].texto, "Escribe qué debe dar") {
		t.Fatalf("tras «Otra cosa» debía pedir el valor escribiendo: %+v", hechas)
	}
	for _, d := range cf.dudas {
		if strings.Contains(d, "Había otra forma") {
			t.Errorf("tras «Otra cosa» no vale «%s»", d)
		}
	}
	if !ok {
		return // no other program: the honest failure follows
	}
	prog := programaDe(h.resp)
	if prog == "" {
		for _, p := range h.resp.Parrafos {
			if strings.HasPrefix(p, "Mi programa: ") {
				prog = strings.TrimPrefix(p, "Mi programa: ")
			}
		}
	}
	e, err := sintesis.Parse(prog, cf.c.registro(), cf.f)
	if err != nil {
		t.Fatalf("programa %q: %v", prog, err)
	}
	m := reListaPregunta.FindStringSubmatch(hechas[0].texto)
	ent, err := nucleo.ParseValor("["+m[1]+"]", cf.f.Entradas()[0])
	if err != nil {
		t.Fatal(err)
	}
	out, err := sintesis.Oraculo(e)([]nucleo.Valor{ent})
	if err != nil || len(out) != 1 {
		t.Fatalf("%s con %v: %v %v", prog, ent, out, err)
	}
	for _, o := range hechas[0].opciones {
		if o == nucleo.FormatoHumano(out[0], cf.f.Res[0]) {
			t.Errorf("%s da %s con %v, que el usuario rechazó con «Otra cosa»", prog, o, ent)
		}
	}
}

// TestCrearOtraCosaEscrita: «Otra cosa» then a typed value makes it a user example.
func TestCrearOtraCosaEscrita(t *testing.T) {
	resp := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		if strings.HasPrefix(p.Texto, "¿Qué debe dar") {
			return "Otra cosa", nil
		}
		if s, ok := sumaMenosLargo(p.Texto); ok {
			return s, nil
		}
		return "", nucleo.ErrSinRespuesta
	}
	cf := flujoPuro(t, "[3,1,2] -> 3", resp, 4*time.Second)
	v, como := cf.preguntarSalida([]nucleo.Valor{[]nucleo.Valor{0}}, []nucleo.Valor{1, 0})
	if como != salidaValor || !nucleo.Igual(v, -1) {
		t.Fatalf("debía tomar el valor escrito -1: %v (%d)", v, como)
	}
	// A text result: «Otra cosa» is never taken as the value "Otra cosa".
	cf2 := flujoPuro(t, `"hola" -> "HOLA"`, func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		if strings.HasPrefix(p.Texto, "¿Qué debe dar") {
			return "Otra cosa", nil
		}
		return "", nucleo.ErrSinRespuesta
	}, 4*time.Second)
	if _, como := cf2.preguntarSalida([]nucleo.Valor{"a"}, []nucleo.Valor{"A", "a"}); como != salidaOtraCosa {
		t.Errorf("«Otra cosa» sin valor: %d", como)
	}
}

// TestCrearEtiquetasMemoria: stored cases of a function kept with less than entendido, and its own
// outputs, only count as "da lo mismo que cuando la guardé" (sin_fallos), never as entendido.
func TestCrearEtiquetasMemoria(t *testing.T) {
	caso := func(nota string) nucleo.Caso {
		return nucleo.Caso{Entradas: []nucleo.Valor{"a"}, Esperado: []nucleo.Valor{"YQ=="}, Expectativa: nucleo.EspReferencia, Nota: nota}
	}
	web := []nucleo.Caso{caso(crearNotaGuardada), caso("otra fuente")}
	crearEtiquetarGuardados(web, nucleo.NivelSinFallos)
	for _, c := range web {
		if c.Nota != crearNotaGuardada {
			t.Errorf("función sin_fallos: %q", c.Nota)
		}
	}
	buena := []nucleo.Caso{caso(crearNotaGuardada), caso("lo que entendí")}
	crearEtiquetarGuardados(buena, nucleo.NivelEntendido)
	if buena[0].Nota != crearNotaGuardada || buena[1].Nota != crearNotaMemoria {
		t.Errorf("función entendido: %q %q", buena[0].Nota, buena[1].Nota)
	}
	rs := []nucleo.ResultadoCaso{{Caso: 0, Ejecutado: true, OK: true, Obtenido: []nucleo.Valor{"YQ=="}},
		{Caso: 1, Ejecutado: true, OK: true, Obtenido: []nucleo.Valor{"YQ=="}}}
	v := verificacion{Casos: web, Resultados: rs}
	v.Comprobaciones = pruebas.Comprobaciones(web, rs, nil, 0)
	v = crearRebajarRegresion(v)
	if v.Nivel != nucleo.NivelSinFallos || len(v.Comprobaciones) != 1 || !strings.Contains(v.Comprobaciones[0].Texto, "dan lo mismo que cuando la guardé") {
		t.Errorf("nivel %s %+v", v.Nivel, v.Comprobaciones)
	}
}

// TestCrearRespuestaWebSinConsenso: a web answer at sin_fallos (no example, no other source) is not
// stored, says what it does only with «Creo que», and never stores the request as its description.
func TestCrearRespuestaWebSinConsenso(t *testing.T) {
	cf := flujoPuro(t, `"hola" -> "aG9sYQ=="`, nil, 4*time.Second)
	v := verificacion{Nivel: nucleo.NivelSinFallos, Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelSinFallos, Pasan: 5, Total: 5, Texto: "5 casos sin errores"}}}
	fuente := "package solucion\n\nfunc Funcion(texto string) string { return texto }\n"
	h := cf.respuesta("web", "Aquí tienes `Funcion`.", fuente, cf.f, v, nil, "devuelve el texto", nil)
	if h.resp.Aprendido != "" || len(cf.c.mem.Funciones()) != 0 {
		t.Errorf("guardó una función web sin consenso: %q", h.resp.Aprendido)
	}
	if !contieneParrafo(h.resp.Parrafos, "Creo que hace esto: devuelve el texto.") {
		t.Errorf("lo no comprobado empieza por «Creo que»: %q", h.resp.Parrafos)
	}
	cf.webConsenso = true
	v.Nivel = nucleo.NivelEntendido
	v.Comprobaciones = []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Pasan: 5, Total: 5, Texto: "5/5 pruebas comparando con otra fuente"}}
	h = cf.respuesta("web", "Aquí tienes `Funcion`.", fuente, cf.f, v, nil, "", nil)
	fs := cf.c.mem.Funciones()
	if h.resp.Aprendido == "" || len(fs) != 1 {
		t.Fatalf("con consenso se guarda: %q %d", h.resp.Aprendido, len(fs))
	}
	if fs[0].Descripcion != "" {
		t.Errorf("la descripción guardada no puede ser la petición: %q", fs[0].Descripcion)
	}
}

// TestCrearMemoriaWebNoSubeNivel: a web function stored at sin_fallos, with cases that are only its own
// outputs, comes back from memoria at sin_fallos, with no «comprobé que el código hace lo que entendí»
// and without the request shown as "Lo que hace".
func TestCrearMemoriaWebNoSubeNivel(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	texto := "haz una función que pase un texto a base64"
	q := analizarCrear(texto)
	f := nucleo.Firma{Nombre: "TextoBase64", Params: []nucleo.Param{{Nombre: "texto", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TString}}
	if q.Firma != nil {
		f.Params = q.Firma.Params
	}
	var casos []nucleo.Caso
	for _, x := range [][2]string{{"", ""}, {"a", "YQ=="}, {"hola", "aG9sYQ=="}, {"Go", "R28="}} {
		casos = append(casos, nucleo.Caso{Entradas: []nucleo.Valor{x[0]}, Esperado: []nucleo.Valor{x[1]},
			Expectativa: nucleo.EspReferencia, Origen: nucleo.OrigenBorde, Nota: crearNotaGuardada})
	}
	gs, err := nucleo.GuardarCasos(f, casos)
	if err != nil {
		t.Fatal(err)
	}
	param := f.Params[0].Nombre
	_, err = c.mem.GuardarFuncion(nucleo.FuncionAprendida{
		Nombre: f.Nombre, Firma: f, Descripcion: texto, Nivel: nucleo.NivelSinFallos, Origen: "web",
		Codigo: "package solucion\n\nimport \"encoding/base64\"\n\nfunc TextoBase64(" + param + " string) string {\n\treturn base64.StdEncoding.EncodeToString([]byte(" + param + "))\n}\n",
		Casos:  gs, Conceptos: q.Conceptos, Palabras: lemasDe(q), Creada: time.Now(), Usada: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	r, tr := preguntarCrear(c, "m", texto, nil)
	if r.Metodo != "memoria" {
		t.Fatalf("no la tomó de la memoria (%s): %s", r.Metodo, titulos(tr))
	}
	todo := todoElTexto(r)
	if r.Nivel != nucleo.NivelSinFallos || hayNivel(r.Comprobaciones, nucleo.NivelEntendido) {
		t.Errorf("nivel %s, debía ser sin_fallos:\n%s", r.Nivel, todo)
	}
	if strings.Contains(todo, FraseSinEjemplos) || strings.Contains(todo, "Lo que hace: haz una") {
		t.Errorf("afirma lo que no comprobó:\n%s", todo)
	}
	if !strings.Contains(todo, "dan lo mismo que cuando la guardé") {
		t.Errorf("falta la comprobación de regresión:\n%s", todo)
	}
}

// TestCrearFalloReducido: the failure shows the minimized failing case (pruebas.Minimizar on the same
// binary, with the program's oracle for the smaller inputs), not the first big one.
func TestCrearFalloReducido(t *testing.T) {
	necesitaGo(t)
	c := cerebroExamen(t)
	k := &tarea{q: analizarCrear("suma los números de una lista"), s: &sesion{id: "r"}, t: nucleo.NuevaTraza(nil, nucleotest.RespuestasFijas()),
		turno: &Turno{}, presupuesto: 30 * time.Second, semilla: 42}
	k.raiz = k.t.Raiz()
	k.ctx, k.ctxPadre = context.Background(), context.Background()
	f := nucleo.Firma{Nombre: "Suma", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	cf := &crearFlujo{c: c, k: k, f: f}
	e, err := sintesis.Parse("(suma nums)", c.registro(), f)
	if err != nil {
		t.Fatal(err)
	}
	oraculo := sintesis.Oraculo(e)
	fuente := "package solucion\n\nfunc Suma(nums []int) int {\n\tt := 0\n\tfor i, n := range nums {\n\t\tif i < 3 {\n\t\t\tt += n\n\t\t}\n\t}\n\treturn t\n}\n"
	casos := cf.generados(f, oraculo, 64, 200)
	v, err := cf.comprobar("Compruebo", fuente, f, casos, nil, map[string]crearOraculo{"lo que entendí": oraculo})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Fallos) == 0 {
		t.Fatal("el código roto pasó")
	}
	cf.anotarIntento(fuente, f, v)
	primero := crearFrasefallo(f, v.Casos[v.Fallos[0]], v.Resultados[v.Fallos[0]])
	contra := cf.mejor.contra
	m := regexp.MustCompile(`^Falla con \[([^\]]*)\]: esperaba (-?\d+), da (-?\d+)\.$`).FindStringSubmatch(contra)
	if m == nil {
		t.Fatalf("frase %q (la primera era %q)", contra, primero)
	}
	if n := len(strings.Split(m[1], ",")); n != 4 {
		t.Errorf("el fallo reducido debe tener 4 números (falla desde el cuarto): %q (la primera: %q)", contra, primero)
	}
	if r := cf.fallo().resp; !contieneParrafo(r.Parrafos, contra) {
		t.Errorf("la respuesta de fallo no muestra el caso reducido: %q", r.Parrafos)
	}
}

// ejecutorConPlazo records how much time the contexts of Preparar and Probar allow.
type ejecutorConPlazo struct {
	nucleo.Ejecutor
	mu     sync.Mutex
	plazos []time.Duration // -1: no deadline
}

func (e *ejecutorConPlazo) anotar(ctx context.Context) {
	d := time.Duration(-1)
	if dl, ok := ctx.Deadline(); ok {
		d = time.Until(dl)
	}
	e.mu.Lock()
	e.plazos = append(e.plazos, d)
	e.mu.Unlock()
}

func (e *ejecutorConPlazo) Preparar(ctx context.Context, p nucleo.Preparacion) (nucleo.Binario, nucleo.Compilacion, error) {
	e.anotar(ctx)
	b, comp, err := e.Ejecutor.Preparar(ctx, p)
	if b != nil {
		b = binarioConPlazo{b, e}
	}
	return b, comp, err
}

type binarioConPlazo struct {
	nucleo.Binario
	e *ejecutorConPlazo
}

func (b binarioConPlazo) Probar(ctx context.Context, casos []nucleo.Caso, op nucleo.OpcionesProbar) ([][]nucleo.ResultadoCaso, error) {
	b.e.anotar(ctx)
	return b.Binario.Probar(ctx, casos, op)
}

// TestCrearWebDentroDeSuPresupuesto: building, running and checking web code stay within the web
// stage's 35 % of the budget.
func TestCrearWebDentroDeSuPresupuesto(t *testing.T) {
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
	c, err := Nuevo(Config{Semilla: 42}, Dependencias{Ej: ej, Mem: mem, Web: webBase64(t, goBin), GoBin: goBin})
	if err != nil {
		t.Fatal(err)
	}
	presupuesto := 40 * time.Second
	k := &tarea{q: analizarCrear("codifica en base64: hola -> aG9sYQ=="), s: &sesion{id: "w"}, t: nucleo.NuevaTraza(nil, nucleotest.RespuestasFijas()),
		turno: &Turno{}, presupuesto: presupuesto, semilla: 42}
	k.raiz = k.t.Raiz()
	k.ctx, k.ctxPadre = context.Background(), context.Background()
	cf := &crearFlujo{c: c, k: k}
	if _, ok := cf.entender(); !ok {
		t.Fatal("no entendió")
	}
	cf.web()
	if k.ctx != context.Background() {
		t.Error("web debe devolver k.ctx como estaba")
	}
	ej.mu.Lock()
	defer ej.mu.Unlock()
	if len(ej.plazos) == 0 {
		t.Fatalf("no compiló nada de la web:\n%s", titulos(k.t))
	}
	tope := presupuesto*35/100 + 100*time.Millisecond
	for i, d := range ej.plazos {
		if d < 0 || d > tope {
			t.Errorf("llamada %d al arenero con plazo %v (tope %v)", i, d, tope)
		}
	}
}
