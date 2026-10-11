package servidor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

const tokenPrueba = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type entorno struct {
	t      *testing.T
	srv    *httptest.Server
	s      *Servidor
	m      *motorFalso
	puerto int
}

func nuevoEntorno(t *testing.T, m *motorFalso, ajustar func(*Config)) *entorno {
	t.Helper()
	if m.grabada.Pasos == nil {
		g, err := cargarTraza("testdata/traza-sumapares.json")
		if err != nil {
			t.Fatal(err)
		}
		m.grabada = g
	}
	e := &entorno{t: t, m: m}
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { e.s.ServeHTTP(w, r) })
	e.srv = httptest.NewServer(h)
	u, _ := url.Parse(e.srv.URL)
	e.puerto, _ = strconv.Atoi(u.Port())
	c := Config{Token: tokenPrueba, Puerto: e.puerto, EsperaPregunta: 5 * time.Second}
	if ajustar != nil {
		ajustar(&c)
	}
	e.s = Nuevo(m, c).(*Servidor)
	t.Cleanup(func() {
		e.s.Cerrar()
		e.srv.Close()
	})
	return e
}

// pedir sends a request with the token (unless cab overrides it).
func (e *entorno) pedir(metodo, ruta, cuerpo string, cab map[string]string) *http.Response {
	e.t.Helper()
	var r io.Reader
	if cuerpo != "" {
		r = strings.NewReader(cuerpo)
	}
	req, err := http.NewRequest(metodo, e.srv.URL+ruta, r)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("X-Nyx-Token", tokenPrueba)
	if cuerpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cab {
		if k == "Host" {
			req.Host = v
			continue
		}
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp
}

func (e *entorno) json(metodo, ruta, cuerpo string, codigo int, destino any) {
	e.t.Helper()
	resp := e.pedir(metodo, ruta, cuerpo, nil)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != codigo {
		e.t.Fatalf("%s %s: código %d, quiero %d: %s", metodo, ruta, resp.StatusCode, codigo, b)
	}
	if destino != nil {
		if err := json.Unmarshal(b, destino); err != nil {
			e.t.Fatalf("%s %s: %v: %s", metodo, ruta, err, b)
		}
	}
}

func (e *entorno) preguntar(texto string) string {
	e.t.Helper()
	var r struct{ Tarea string }
	e.json("POST", "/api/preguntar", fmt.Sprintf(`{"sesion":"s1","texto":%q,"modo":""}`, texto), 200, &r)
	if len(r.Tarea) != 12 {
		e.t.Fatalf("id de tarea = %q", r.Tarea)
	}
	return r.Tarea
}

type eventoLeido struct {
	id    int
	tipo  string
	datos string
}

// leerSSE reads events until fin (or EOF); latidos counts heartbeat comments.
func leerSSE(t *testing.T, r io.Reader, cada func(eventoLeido)) (evs []eventoLeido, latidos int) {
	t.Helper()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var cur eventoLeido
	for sc.Scan() {
		l := sc.Text()
		switch {
		case l == "":
			if cur.tipo != "" {
				evs = append(evs, cur)
				if cada != nil {
					cada(cur)
				}
				if cur.tipo == "fin" {
					return
				}
			}
			cur = eventoLeido{}
		case strings.HasPrefix(l, ":"):
			if strings.Contains(l, "latido") {
				latidos++
			}
		case strings.HasPrefix(l, "id: "):
			cur.id, _ = strconv.Atoi(l[4:])
		case strings.HasPrefix(l, "event: "):
			cur.tipo = l[7:]
		case strings.HasPrefix(l, "data: "):
			cur.datos = l[6:]
		}
	}
	return
}

func (e *entorno) flujo(id string, cab map[string]string, query string, cada func(eventoLeido)) ([]eventoLeido, int, *http.Response) {
	e.t.Helper()
	ruta := "/api/tareas/" + id + "/eventos?t=" + tokenPrueba + query
	if cab == nil {
		cab = map[string]string{}
	}
	cab["X-Nyx-Token"] = ""
	resp := e.pedir("GET", ruta, "", cab)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, 0, resp
	}
	evs, lat := leerSSE(e.t, resp.Body, cada)
	return evs, lat, resp
}

// ---- security ----

func TestSeguridad403(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{sinPreguntar: true}, nil)
	casos := []struct {
		nombre string
		cab    map[string]string
		ruta   string
	}{
		{"sin token", map[string]string{"X-Nyx-Token": ""}, "/api/estado"},
		{"token malo", map[string]string{"X-Nyx-Token": strings.Repeat("0", 64)}, "/api/estado"},
		{"host ajeno", map[string]string{"Host": "evil.example:" + strconv.Itoa(e.puerto)}, "/api/estado"},
		{"host con otro puerto", map[string]string{"Host": "127.0.0.1:1"}, "/api/estado"},
		{"host ajeno en la página", map[string]string{"Host": "attacker.test"}, "/"},
		{"origen ajeno", map[string]string{"Origin": "http://evil.example"}, "/api/estado"},
		{"origen https", map[string]string{"Origin": "https://127.0.0.1:" + strconv.Itoa(e.puerto)}, "/api/estado"},
		{"sse sin t", map[string]string{"X-Nyx-Token": ""}, "/api/tareas/abc/eventos"},
		{"sse con cabecera pero sin t", nil, "/api/tareas/abc/eventos"},
	}
	for _, c := range casos {
		resp := e.pedir("GET", c.ruta, "", c.cab)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: código %d", c.nombre, resp.StatusCode)
		}
		if resp.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("%s: no debe haber CORS", c.nombre)
		}
	}
	// the right ones pass
	for _, cab := range []map[string]string{
		{"Origin": "http://127.0.0.1:" + strconv.Itoa(e.puerto)},
		{"Origin": "http://localhost:" + strconv.Itoa(e.puerto), "Host": "localhost:" + strconv.Itoa(e.puerto)},
		{},
	} {
		resp := e.pedir("GET", "/api/estado", "", cab)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%v: código %d", cab, resp.StatusCode)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("falta Cache-Control: no-store en /api/")
		}
	}
	// a server without a token refuses everything
	sinToken := Nuevo(&motorFalso{}, Config{Puerto: e.puerto})
	req := httptest.NewRequest("GET", "/api/estado", nil)
	req.Host = "127.0.0.1:" + strconv.Itoa(e.puerto)
	req.Header.Set("X-Nyx-Token", "")
	w := httptest.NewRecorder()
	sinToken.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("sin token configurado: %d", w.Code)
	}
}

func TestCuerpoDemasiadoGrande(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{sinPreguntar: true}, nil)
	grande := `{"sesion":"s","texto":"` + strings.Repeat("a", (1<<20)+10) + `"}`
	resp := e.pedir("POST", "/api/preguntar", grande, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("con Content-Length: %d", resp.StatusCode)
	}
	// without Content-Length (chunked)
	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte(grande))
		pw.Close()
	}()
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/preguntar", pr)
	req.Header.Set("X-Nyx-Token", tokenPrueba)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("troceado: %d", resp.StatusCode)
	}
	if len(e.m.peticiones) != 0 {
		t.Error("se creó una tarea")
	}
}

func TestPaginaYCabeceras(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{}, nil)
	resp := e.pedir("GET", "/", "", map[string]string{"X-Nyx-Token": ""})
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("GET /: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != CSP {
		t.Errorf("CSP = %q", got)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("cabeceras: %v", resp.Header)
	}
	html := string(b)
	if !strings.Contains(html, `src="/estatico/app.js"`) || !strings.Contains(html, `href="/estatico/estilo.css"`) {
		t.Error("index.html no enlaza app.js y estilo.css")
	}
	if strings.Contains(html, tokenPrueba) {
		t.Error("index.html contiene la clave")
	}
	for _, a := range []struct{ ruta, tipo string }{{"/estatico/app.js", "text/javascript"}, {"/estatico/estilo.css", "text/css"}} {
		resp := e.pedir("GET", a.ruta, "", map[string]string{"X-Nyx-Token": ""})
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), a.tipo) {
			t.Errorf("%s: %d %s", a.ruta, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	for _, ruta := range []string{"/estatico/index.html", "/estatico/..%2fservidor.go", "/otra", "/estatico/"} {
		resp := e.pedir("GET", ruta, "", map[string]string{"X-Nyx-Token": ""})
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("%s: %d", ruta, resp.StatusCode)
		}
	}
}

func TestArchivosIncrustados(t *testing.T) {
	f := Archivos()
	for _, n := range []string{"index.html", "app.js", "estilo.css"} {
		b, err := fs.ReadFile(f, n)
		if err != nil || len(b) < 500 {
			t.Errorf("%s: %v (%d bytes)", n, err, len(b))
		}
	}
	js, _ := fs.ReadFile(f, "app.js")
	src := string(js)
	if strings.Contains(src, ".innerHTML") || strings.Contains(src, "insertAdjacentHTML") || strings.Contains(src, "eval(") {
		t.Error("app.js no debe usar innerHTML, insertAdjacentHTML ni eval")
	}
	for _, s := range []string{"Escribe tu pregunta…", "Pensar más", "Cómo lo pienso", "Descargar .go", "¿Está bien?", "No es eso",
		"Gracias, lo tendré en cuenta", "Sin conexión con Nyx Código: ¿sigue abierto el programa?", "Pensando…", "Otra cosa"} {
		if !strings.Contains(src, s) {
			t.Errorf("falta el texto fijo %q en app.js", s)
		}
	}
	html, _ := fs.ReadFile(f, "index.html")
	for _, no := range []string{`src="http`, `href="http`, `src="//`, `href="//`, "<script>", "style=", "<style"} {
		if strings.Contains(string(html), no) {
			t.Errorf("index.html contiene %q (nada externo ni en línea)", no)
		}
	}
	css, _ := fs.ReadFile(f, "estilo.css")
	if strings.Contains(string(css), "@import") || strings.Contains(string(css), "url(http") {
		t.Error("estilo.css carga algo externo")
	}
}

// ---- tasks and SSE ----

func TestFlujoCompletoEnOrden(t *testing.T) {
	for _, terminaSolo := range []bool{false, true} {
		t.Run(fmt.Sprintf("terminaSolo=%v", terminaSolo), func(t *testing.T) {
			e := nuevoEntorno(t, &motorFalso{sinPreguntar: true, terminaSolo: terminaSolo}, nil)
			id := e.preguntar("haz una función que sume los números pares de una lista")
			evs, _, resp := e.flujo(id, nil, "", nil)
			if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
				t.Fatalf("flujo: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
			}
			if len(evs) < 10 {
				t.Fatalf("pocos eventos: %d", len(evs))
			}
			for i, ev := range evs {
				if i > 0 && ev.id <= evs[i-1].id {
					t.Errorf("ids no crecientes: %d tras %d", ev.id, evs[i-1].id)
				}
			}
			if evs[0].tipo != "paso" {
				t.Errorf("primer evento %q", evs[0].tipo)
			}
			n := len(evs)
			if evs[n-2].tipo != "respuesta" || evs[n-1].tipo != "fin" {
				t.Fatalf("final: %s, %s", evs[n-2].tipo, evs[n-1].tipo)
			}
			var p nucleo.Paso
			if err := json.Unmarshal([]byte(evs[0].datos), &p); err != nil || p.ID != 1 || p.Tipo != nucleo.PasoEntender {
				t.Errorf("paso 1 = %+v %v", p, err)
			}
			var r nucleo.Respuesta
			if err := json.Unmarshal([]byte(evs[n-2].datos), &r); err != nil || r.Texto != "Aquí la tienes." || r.Tarea != id {
				t.Errorf("respuesta = %+v %v", r, err)
			}
			var fin nucleo.Fin
			json.Unmarshal([]byte(evs[n-1].datos), &fin)
			if fin.Estado != "terminada" {
				t.Errorf("fin = %+v", fin)
			}
			fines := 0
			for _, ev := range evs {
				if ev.tipo == "fin" {
					fines++
				}
			}
			if fines != 1 {
				t.Errorf("%d eventos fin", fines)
			}
			// the task has finished: a reconnect after fin gets 204 (EventSource stops)
			_, _, resp = e.flujo(id, map[string]string{"Last-Event-ID": strconv.Itoa(evs[n-1].id)}, "", nil)
			if resp.StatusCode != http.StatusNoContent {
				t.Errorf("reconexión tras fin: %d", resp.StatusCode)
			}
			if got := e.m.peticiones[0]; got.Tarea != id || got.Sesion != "s1" {
				t.Errorf("petición = %+v", got)
			}
		})
	}
}

func TestReconexionLastEventID(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{sinPreguntar: true}, nil)
	id := e.preguntar("hola")
	todos, _, _ := e.flujo(id, nil, "", nil)
	desde3, _, _ := e.flujo(id, map[string]string{"Last-Event-ID": "3"}, "", nil)
	if len(desde3) == 0 || desde3[0].id != 4 {
		t.Fatalf("Last-Event-ID 3 empieza en %+v", desde3)
	}
	if len(desde3) != len(todos)-3 {
		t.Errorf("reenvió %d de %d", len(desde3), len(todos))
	}
	q, _, _ := e.flujo(id, nil, "&desde=5", nil)
	if len(q) == 0 || q[0].id != 6 {
		t.Errorf("?desde=5 empieza en %+v", q)
	}
}

func TestResponderDesbloqueaPreguntar(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{}, nil)
	id := e.preguntar("haz una función que sume los números pares de una lista")
	contestada := false
	evs, _, _ := e.flujo(id, nil, "", func(ev eventoLeido) {
		if ev.tipo != "pregunta" || contestada {
			return
		}
		var q nucleo.PreguntaUsuario
		if err := json.Unmarshal([]byte(ev.datos), &q); err != nil || q.ID != "p1" || len(q.Opciones) != 2 || !q.Libre {
			t.Errorf("pregunta = %+v %v", q, err)
		}
		e.json("POST", "/api/tareas/"+id+"/responder", `{"pregunta":"p1","respuesta":"0"}`, 204, nil)
		contestada = true
		// a second answer to the same question is refused
		resp := e.pedir("POST", "/api/tareas/"+id+"/responder", `{"pregunta":"p1","respuesta":"-2"}`, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("segunda respuesta: %d", resp.StatusCode)
		}
	})
	if !contestada {
		t.Fatal("no llegó la pregunta")
	}
	if got := e.m.respuestas; len(got) != 1 || got[0] != "0" {
		t.Errorf("el Motor recibió %v", got)
	}
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evs[len(evs)-2].datos), &r)
	if !strings.Contains(strings.Join(r.Parrafos, " "), "Con [-2] da 0") {
		t.Errorf("la respuesta no usa lo contestado: %+v", r.Parrafos)
	}
	// unknown question / unknown task
	resp := e.pedir("POST", "/api/tareas/"+id+"/responder", `{"pregunta":"p9","respuesta":"1"}`, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("pregunta inexistente: %d", resp.StatusCode)
	}
	resp = e.pedir("POST", "/api/tareas/ffffffffffff/responder", `{"pregunta":"p1","respuesta":"1"}`, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("tarea inexistente: %d", resp.StatusCode)
	}
}

func TestPreguntaSinRespuestaCaduca(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{}, func(c *Config) { c.EsperaPregunta = 50 * time.Millisecond })
	id := e.preguntar("x")
	evs, _, _ := e.flujo(id, nil, "", nil)
	if evs[len(evs)-1].tipo != "fin" {
		t.Fatal("no terminó")
	}
	if len(e.m.respuestas) != 0 {
		t.Errorf("respuestas = %v", e.m.respuestas)
	}
}

func TestCancelar(t *testing.T) {
	m := &motorFalso{esperarCtx: true, entrar: make(chan struct{}, 1)}
	e := nuevoEntorno(t, m, nil)
	id := e.preguntar("piensa mucho")
	<-m.entrar
	e.json("POST", "/api/tareas/"+id+"/cancelar", "", 204, nil)
	evs, _, _ := e.flujo(id, nil, "", nil)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evs[len(evs)-1].datos), &fin)
	if fin.Estado != "cancelada" {
		t.Errorf("fin = %+v", fin)
	}
	m.mu.Lock()
	err := m.errCtx
	m.mu.Unlock()
	if err != context.Canceled {
		t.Errorf("el Motor vio %v", err)
	}
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evs[len(evs)-2].datos), &r)
	if r.Texto != "Lo mejor que tenía: nada" {
		t.Errorf("respuesta parcial = %+v", r)
	}
	resp := e.pedir("POST", "/api/tareas/000000000000/cancelar", "", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("cancelar inexistente: %d", resp.StatusCode)
	}
}

func TestLimiteDuro(t *testing.T) {
	m := &motorFalso{esperarCtx: true}
	e := nuevoEntorno(t, m, func(c *Config) { c.Limite = 80 * time.Millisecond })
	id := e.preguntar("no acabes nunca")
	evs, _, _ := e.flujo(id, nil, "", nil)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evs[len(evs)-1].datos), &fin)
	if fin.Estado != "terminada" {
		t.Errorf("fin = %+v", fin)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errCtx != context.DeadlineExceeded {
		t.Errorf("el Motor vio %v", m.errCtx)
	}
}

func TestPanicoDelMotor(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{panico: true}, nil)
	id := e.preguntar("rompe")
	evs, _, _ := e.flujo(id, nil, "", nil)
	if len(evs) != 2 {
		t.Fatalf("eventos = %+v", evs)
	}
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evs[0].datos), &r)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evs[1].datos), &fin)
	if fin.Estado != "error" || !strings.Contains(r.Texto, "Lo siento") || !strings.Contains(strings.Join(r.Parrafos, ""), "se rompió algo") {
		t.Errorf("respuesta = %+v fin = %+v", r, fin)
	}
	if r.Nivel != nucleo.NivelFallo || r.Comprobaciones == nil {
		t.Errorf("nivel = %q", r.Nivel)
	}
}

func TestLatido(t *testing.T) {
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 1)}
	e := nuevoEntorno(t, m, func(c *Config) { c.Latido = 20 * time.Millisecond })
	id := e.preguntar("x")
	<-m.entrar
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(m.soltar)
	}()
	_, latidos, _ := e.flujo(id, nil, "", nil)
	if latidos < 2 {
		t.Errorf("latidos = %d", latidos)
	}
}

func TestColaMaxTareas(t *testing.T) {
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4)}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1 })
	a := e.preguntar("primera")
	<-m.entrar
	b := e.preguntar("segunda")
	select {
	case <-m.entrar:
		t.Fatal("la segunda tarea empezó sin esperar")
	case <-time.After(100 * time.Millisecond):
	}
	close(m.soltar)
	evA, _, _ := e.flujo(a, nil, "", nil)
	evB, _, _ := e.flujo(b, nil, "", nil)
	if evA[len(evA)-1].tipo != "fin" || evB[len(evB)-1].tipo != "fin" {
		t.Fatal("no terminaron")
	}
	var p nucleo.Paso
	json.Unmarshal([]byte(evB[0].datos), &p)
	if !strings.Contains(p.Titulo, "Espero mi turno") {
		t.Errorf("la segunda no avisó de la cola: %+v", p)
	}
}

func TestRetener(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{sinPreguntar: true}, func(c *Config) { c.Retener = time.Minute })
	ahora := time.Now()
	e.s.ahora = func() time.Time { return ahora }
	id := e.preguntar("vieja")
	e.flujo(id, nil, "", nil)
	ahora = ahora.Add(2 * time.Minute)
	e.preguntar("nueva") // creating a task sweeps old ones
	resp := e.pedir("GET", "/api/tareas/"+id+"/eventos?t="+tokenPrueba, "", map[string]string{"X-Nyx-Token": ""})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("tarea vieja: %d", resp.StatusCode)
	}
}

func TestEjecutarRiesgos(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{sinPreguntar: true}, nil)
	codigo := `package main\nimport \"os/exec\"\nfunc main() { exec.Command(\"ls\").Run() }`
	var conflicto struct{ Riesgos []nucleo.Violacion }
	e.json("POST", "/api/codigo/ejecutar", `{"sesion":"s","codigo":"`+codigo+`"}`, 409, &conflicto)
	if len(conflicto.Riesgos) != 1 || conflicto.Riesgos[0].Paquete != "os/exec" {
		t.Errorf("riesgos = %+v", conflicto.Riesgos)
	}
	if len(e.m.peticiones) != 0 {
		t.Error("409 no debe crear tarea")
	}
	var r struct{ Tarea string }
	e.json("POST", "/api/codigo/ejecutar", `{"sesion":"s","codigo":"`+codigo+`","entrada":"5","confirmado":true}`, 200, &r)
	e.flujo(r.Tarea, nil, "", nil)
	p := e.m.peticiones[0]
	if p.Accion != "ejecutar" || !p.Confirmado || p.Entrada != "5" || p.Tarea != r.Tarea {
		t.Errorf("petición = %+v", p)
	}
	// safe code runs without confirmation
	e.json("POST", "/api/codigo/ejecutar", `{"sesion":"s","codigo":"package main\nfunc main() {}"}`, 200, &r)
	// other actions, unknown action, missing code
	e.json("POST", "/api/codigo/explicar", `{"sesion":"s","codigo":"func f() {}"}`, 200, &r)
	e.json("POST", "/api/codigo/borrar", `{"codigo":"x"}`, 404, nil)
	e.json("POST", "/api/codigo/probar", `{"codigo":"  "}`, 400, nil)
	e.json("POST", "/api/preguntar", `{"texto":""}`, 400, nil)
	e.json("POST", "/api/preguntar", `no es json`, 400, nil)
}

func TestPensarMasYEstudiar(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{sinPreguntar: true}, nil)
	id := e.preguntar("haz una función que sume los pares")
	e.flujo(id, nil, "", nil)
	var r struct{ Tarea string }
	e.json("POST", "/api/tareas/"+id+"/pensar-mas", "", 200, &r)
	if r.Tarea == id || r.Tarea == "" {
		t.Fatalf("tarea nueva = %q", r.Tarea)
	}
	e.flujo(r.Tarea, nil, "", nil)
	p := e.m.peticiones[1]
	if p.Accion != "pensar_mas" || p.Previa != id || p.Texto != "haz una función que sume los pares" || p.Sesion != "s1" {
		t.Errorf("pensar más = %+v", p)
	}
	e.json("POST", "/api/tareas/123456789012/pensar-mas", "", 404, nil)
	e.json("POST", "/api/estudiar", "", 200, &r)
	e.flujo(r.Tarea, nil, "", nil)
	if p := e.m.peticiones[2]; p.Accion != "estudiar" {
		t.Errorf("estudiar = %+v", p)
	}
}

func TestOpinion(t *testing.T) {
	e := nuevoEntorno(t, &motorFalso{}, nil)
	e.json("POST", "/api/opinion", `{"sesion":"s","tarea":"abc","correcto":false,"comentario":"no","ejemplo":"con [2] debe dar 0"}`, 204, nil)
	if len(e.m.opiniones) != 1 || e.m.opiniones[0].Ejemplo != "con [2] debe dar 0" || e.m.opiniones[0].Correcto {
		t.Errorf("opiniones = %+v", e.m.opiniones)
	}
	e.json("POST", "/api/opinion", `{"sesion":"s"}`, 500, nil)
}

func TestMemoria(t *testing.T) {
	m := &motorFalso{}
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	casos, err := nucleo.GuardarCasos(f, []nucleo.Caso{
		{Entradas: []nucleo.Valor{[]nucleo.Valor{1, 2, 3, 4}}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario},
		{Entradas: []nucleo.Valor{[]nucleo.Valor(nil)}, Origen: nucleo.OrigenBorde},
	})
	if err != nil {
		t.Fatal(err)
	}
	m.funciones = []nucleo.FuncionAprendida{{Nombre: "SumaPares", Firma: f, Casos: casos, Descripcion: "la suma de los números pares", Origen: "sintesis", Nivel: nucleo.NivelEntendido}}
	m.pendientes = []pendienteFalso{{ID: "p1", Texto: "haz algo difícil", Motivo: "sin tiempo", Intentos: 2}}
	e := nuevoEntorno(t, m, nil)

	var fs []map[string]any
	e.json("GET", "/api/memoria/funciones?q=suma", "", 200, &fs)
	if len(fs) != 1 || fs[0]["firmaGo"] != "func SumaPares(nums []int) int" {
		t.Fatalf("funciones = %+v", fs)
	}
	cs := fs[0]["casos"].([]any)
	c0 := cs[0].(map[string]any)
	if c0["entrada"] != "SumaPares([1, 2, 3, 4])" || c0["esperado"] != "6" || c0["origen"] != "usuario" {
		t.Errorf("caso 0 = %+v", c0)
	}
	if c1 := cs[1].(map[string]any); c1["entrada"] != "SumaPares([])" || c1["esperado"] != nil {
		t.Errorf("caso 1 = %+v", c1)
	}
	e.json("GET", "/api/memoria/funciones?q=zzz", "", 200, &fs)
	if len(fs) != 0 {
		t.Errorf("filtro: %+v", fs)
	}
	var hs []nucleo.Hecho
	e.json("GET", "/api/memoria/hechos?q=nada", "", 200, &hs)
	if hs == nil || len(hs) != 0 {
		t.Errorf("hechos vacíos deben ser [] : %v", hs)
	}
	e.json("GET", "/api/memoria/hechos", "", 200, &hs)
	if len(hs) != 1 || hs[0].Sujeto != "toby" {
		t.Errorf("hechos = %+v", hs)
	}
	var pal map[string]string
	e.json("GET", "/api/memoria/palabras", "", 200, &pal)
	if pal["capicua"] != "palindromo" {
		t.Errorf("palabras = %v", pal)
	}
	var pend []map[string]any
	e.json("GET", "/api/memoria/pendientes", "", 200, &pend)
	if len(pend) != 1 || pend[0]["Texto"] != "haz algo difícil" {
		t.Errorf("pendientes = %+v", pend)
	}
	e.json("DELETE", "/api/memoria/funciones/SumaPares", "", 204, nil)
	e.json("DELETE", "/api/memoria/hechos/h1", "", 204, nil)
	e.json("DELETE", "/api/memoria/palabras/capicua", "", 204, nil)
	e.json("DELETE", "/api/memoria/funciones/NoExiste", "", 404, nil)
	if strings.Join(m.olvidos, ",") != "funcion:SumaPares,hecho:h1,palabra:capicua" {
		t.Errorf("olvidos = %v", m.olvidos)
	}
	var est nucleo.EstadoApp
	e.json("GET", "/api/estado", "", 200, &est)
	if est.Arenero.GoVersion != "go1.24.7" || est.Funciones != 1 {
		t.Errorf("estado = %+v", est)
	}
}

func TestPendientesSinMetodo(t *testing.T) {
	type soloMotor struct{ Motor }
	s := nuevo(soloMotor{&motorFalso{}}, Config{Token: tokenPrueba, Puerto: 80})
	req := httptest.NewRequest("GET", "/api/memoria/pendientes", nil)
	req.Host = "127.0.0.1:80"
	req.Header.Set("X-Nyx-Token", tokenPrueba)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("pendientes sin método: %d %s", w.Code, w.Body.String())
	}
}
