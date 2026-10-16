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
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
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

// TestCrearWeb: synthesis cannot build base64; the web (an httptest fake) has a function that does, it is
// adapted, run on the user's example and kept with its source.
func TestCrearWeb(t *testing.T) {
	necesitaGo(t)
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
	defer srv.Close()
	goBin, _ := exec.LookPath("go")
	web := saber.NuevaWeb(saber.Config{
		PermitirLocal: true, BaseDDG: srv.URL + "/ddg/", BaseGoByExample: srv.URL + "/gobyexample/",
		BasePkgGoDev: srv.URL + "/pkg/", BaseGoDev: srv.URL + "/godev/", BaseWikipedia: srv.URL + "/wiki/%s/",
		Dormir: func(time.Duration) {}, GoBin: goBin, DirCache: t.TempDir(),
	})
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
