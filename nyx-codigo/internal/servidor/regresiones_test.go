package servidor

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Regression: the hard limit used to start when the task was created, so a queued task lost its
// queue time and could be reported as "cancelada" ("Lo he parado antes de empezar.") without the user
// stopping it.
func TestLimiteEmpiezaAlTenerTurno(t *testing.T) {
	const limite = 300 * time.Millisecond
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4)}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1; c.Limite = limite })
	defer close(m.soltar)
	a := e.preguntar("primera")
	<-m.entrar
	b := e.preguntar("segunda") // queued while the first one uses its whole budget
	evA, _, _ := e.flujo(a, nil, "", nil)
	evB, _, _ := e.flujo(b, nil, "", nil)
	var finA, finB nucleo.Fin
	json.Unmarshal([]byte(evA[len(evA)-1].datos), &finA)
	json.Unmarshal([]byte(evB[len(evB)-1].datos), &finB)
	if finA.Estado != "terminada" || finB.Estado != "terminada" {
		t.Fatalf("fin a = %+v, fin b = %+v", finA, finB)
	}
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evB[len(evB)-2].datos), &r)
	if strings.Contains(r.Texto, "antes de empezar") {
		t.Errorf("la segunda se dio por cancelada: %+v", r)
	}
	m.mu.Lock()
	plazos := append([]time.Duration(nil), m.plazos...)
	m.mu.Unlock()
	if len(plazos) != 2 {
		t.Fatalf("plazos = %v", plazos)
	}
	// the second task waited about 300 ms in the queue and still starts with (almost) all of it
	if plazos[1] < limite*2/3 {
		t.Errorf("la segunda empezó con %v de %v", plazos[1], limite)
	}
}

// A task that cannot get a slot within its limit ends with an honest error, not "cancelada".
func TestColaDemasiadoLarga(t *testing.T) {
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4), ignorarCtx: true}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1; c.Limite = 100 * time.Millisecond })
	defer close(m.soltar) // the stuck Motor returns only at the end
	e.preguntar("atascada")
	<-m.entrar
	b := e.preguntar("en cola")
	evB, _, _ := e.flujo(b, nil, "", nil)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evB[len(evB)-1].datos), &fin)
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evB[len(evB)-2].datos), &r)
	if fin.Estado != "error" || !strings.Contains(r.Texto, "demasiadas tareas") || r.Nivel != nucleo.NivelFallo {
		t.Errorf("fin = %+v respuesta = %+v", fin, r)
	}
}

// Cancelling a queued task still says it was stopped before starting.
func TestCancelarEnCola(t *testing.T) {
	m := &motorFalso{sinPreguntar: true, soltar: make(chan struct{}), entrar: make(chan struct{}, 4)}
	e := nuevoEntorno(t, m, func(c *Config) { c.MaxTareas = 1 })
	defer close(m.soltar)
	e.preguntar("primera")
	<-m.entrar
	b := e.preguntar("segunda")
	time.Sleep(20 * time.Millisecond)
	e.json("POST", "/api/tareas/"+b+"/cancelar", "", 204, nil)
	evB, _, _ := e.flujo(b, nil, "", nil)
	var fin nucleo.Fin
	json.Unmarshal([]byte(evB[len(evB)-1].datos), &fin)
	var r nucleo.Respuesta
	json.Unmarshal([]byte(evB[len(evB)-2].datos), &r)
	if fin.Estado != "cancelada" || r.Texto != "Lo he parado antes de empezar." {
		t.Errorf("fin = %+v respuesta = %+v", fin, r)
	}
}

// ---- page (web/app.js) regressions ----

func leerAppJS(t *testing.T) string {
	t.Helper()
	b, err := fs.ReadFile(Archivos(), "app.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// trozo returns the source of app.js from the first occurrence of desde up to the first occurrence of
// hasta after it.
func trozo(t *testing.T, src, desde, hasta string) string {
	t.Helper()
	i := strings.Index(src, desde)
	if i < 0 {
		t.Fatalf("no encuentro %q en app.js", desde)
	}
	j := strings.Index(src[i+len(desde):], hasta)
	if j < 0 {
		t.Fatalf("no encuentro %q tras %q en app.js", hasta, desde)
	}
	return src[i : i+len(desde)+j]
}

// Regression: "¿Está bien? [No]" used to post nothing unless the optional field was then sent (§6.2:
// both buttons post /api/opinion; the field is optional).
func TestBotonNoMandaOpinionEnElActo(t *testing.T) {
	src := leerAppJS(t)
	no := trozo(t, src, "var bNo = boton(", "}, 'secundario pequeno');\n    fila.appendChild(bSi);")
	envio := strings.Index(no, "opinar(false, '')")
	campo := strings.Index(no, "el('input'")
	if envio < 0 || campo < 0 || envio > campo {
		t.Fatalf("«No» debe mandar opinar(false, '') antes de abrir el campo opcional:\n%s", no)
	}
	opinar := trozo(t, src, "function opinar(correcto, ejemplo)", "\n    }\n")
	if !strings.Contains(opinar, "'/api/opinion'") {
		t.Errorf("opinar no manda /api/opinion:\n%s", opinar)
	}
	// the optional field only adds a second opinion with the example
	if !strings.Contains(no, "opinar(false, ejemplo)") {
		t.Errorf("el campo opcional no manda el ejemplo:\n%s", no)
	}
}

// Regression: while a question card was open, chips, «No es eso» and «Reintentar» were sent as the
// answer to that question. Only the main box (opciones.responde) answers it now.
func TestSoloElCuadroPrincipalContestaPreguntas(t *testing.T) {
	src := leerAppJS(t)
	env := trozo(t, src, "function enviarTexto(texto, codigo, opciones) {", "texto = (texto || '').trim();")
	if !strings.Contains(env, "opciones.responde") || !strings.Contains(env, "preguntaAbierta.contestar(texto)") {
		t.Fatalf("enviarTexto debe contestar la pregunta abierta solo con opciones.responde:\n%s", env)
	}
	if n := strings.Count(src, "responde: true"); n != 1 {
		t.Errorf("responde: true aparece %d veces; solo el formulario principal debe usarlo", n)
	}
	form := trozo(t, src, "form.addEventListener('submit'", "});")
	if !strings.Contains(form+"});", "enviarTexto(texto, codigo, { responde: true })") {
		t.Errorf("el formulario principal no contesta preguntas:\n%s", form)
	}
	noEs := trozo(t, src, "function noEsEso() {", "}")
	if strings.Contains(noEs, "responde") || !strings.Contains(noEs, "enviarTexto(TEXTOS.estaMal)") {
		t.Errorf("«No es eso» debe ser un mensaje nuevo:\n%s", noEs)
	}
}

// Regression: the gutter phrases depended on an unwritten convention. contrato.go now writes it down
// (TablaFrases / FrasesDeTablas) and the page must read the same table.
func TestContratoFrasesDelMedianil(t *testing.T) {
	tabla := TablaFrases([]FraseLinea{
		{Linea: 3, Texto: "devuelve la suma"},
		{Linea: 1, Texto: "empieza la función"},
		{Linea: 0, Texto: "sin línea"},
		{Linea: 2, Texto: "  "},
		{Linea: 3, Texto: "y termina"},
	})
	if tabla.Cabecera[0] != CabeceraFrases || !EsTablaFrases(tabla) || len(tabla.Filas) != 3 || tabla.Filas[0][0] != "1" {
		t.Fatalf("tabla = %+v", tabla)
	}
	r := nucleo.Respuesta{Tablas: []nucleo.Tabla{{Cabecera: []string{"Paso", "Valor"}, Filas: [][]string{{"1", "x"}}}, tabla}}
	got := FrasesDeTablas(r.Tablas)
	if len(got) != 2 || got[1] != "empieza la función" || got[3] != "devuelve la suma y termina" {
		t.Errorf("frases = %#v", got)
	}
	// same header rule as the page: lowercased, no accents, starts with "linea"
	for cab, quiero := range map[string]bool{"Línea": true, "LINEA": true, "líneas del código": true, "Paso": false, "": false} {
		if g := EsTablaFrases(nucleo.Tabla{Cabecera: []string{cab, "Frase"}}); g != quiero {
			t.Errorf("EsTablaFrases(%q) = %v", cab, g)
		}
	}
	if EsTablaFrases(nucleo.Tabla{Cabecera: []string{"Línea"}}) {
		t.Error("una sola columna no es tabla de frases")
	}
	if g := FrasesDeTablas([]nucleo.Tabla{{Cabecera: []string{"Línea", "Frase"}, Filas: [][]string{{"12 ", "a", "b"}, {"x", "no"}, {"-1", "no"}, {}}}}); len(g) != 1 || g[12] != "a — b" {
		t.Errorf("filas raras = %#v", g)
	}
	src := leerAppJS(t)
	fr := trozo(t, src, "function esTablaFrases(x) {", "}")
	if !strings.Contains(fr, "normal(x.cabecera[0]).indexOf('linea') === 0") || !strings.Contains(fr, "x.cabecera.length >= 2") {
		t.Errorf("app.js ya no lee la tabla del contrato:\n%s", fr)
	}
	if !strings.Contains(src, "f.slice(1).join(' — ')") || !strings.Contains(src, "parseInt(f[0], 10)") {
		t.Error("app.js ya no une las frases como dice contrato.go")
	}
	b, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(b), `"cabecera":["Línea","Qué hace"]`) {
		t.Errorf("json = %s %v", b, err)
	}
}

// app.js still parses (only when node is installed).
func TestAppJSSintaxis(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node no está instalado")
	}
	dir := t.TempDir()
	ruta := filepath.Join(dir, "app.js")
	if err := os.WriteFile(ruta, []byte(leerAppJS(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, "--check", ruta).CombinedOutput(); err != nil {
		t.Fatalf("node --check: %v\n%s", err, out)
	}
}
