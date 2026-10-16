package cerebro

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

// ---- helpers ----

func nuevoCerebroPrueba(t *testing.T, ej nucleo.Ejecutor) *Cerebro {
	t.Helper()
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mem.Cerrar() })
	c, err := Nuevo(Config{Semilla: 42}, Dependencias{Ej: ej, Mem: mem})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var numTarea atomic.Int64

// preguntar sends one request in a session and returns the answer and its trace.
func preguntar(t *testing.T, c *Cerebro, sesion, texto string, resp nucleo.Responder) (nucleo.Respuesta, *nucleo.Traza) {
	t.Helper()
	tr := nucleo.NuevaTraza(nil, resp)
	p := nucleo.Peticion{Tarea: "t" + itoa(numTarea.Add(1)), Sesion: sesion, Texto: texto}
	ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelar()
	return c.Responder(ctx, p, tr), tr
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// todoElTexto joins every visible text of an answer.
func todoElTexto(r nucleo.Respuesta) string {
	partes := []string{r.Texto, r.Entendi, r.Codigo, r.Aprendido}
	partes = append(partes, r.Parrafos...)
	partes = append(partes, r.Plan...)
	partes = append(partes, r.Justificacion...)
	for _, c := range r.Comprobaciones {
		partes = append(partes, c.Texto)
	}
	for _, tb := range r.Tablas {
		partes = append(partes, tb.Titulo, strings.Join(tb.Cabecera, " "))
		for _, f := range tb.Filas {
			partes = append(partes, strings.Join(f, " "))
		}
	}
	return strings.Join(partes, "\n")
}

func titulos(tr *nucleo.Traza) string {
	var out []string
	for _, p := range tr.Pasos() {
		out = append(out, p.Titulo)
	}
	return strings.Join(out, "\n")
}

// resolutorFalso recognizes texts containing "zork" (a word lengua does not know).
type resolutorFalso struct {
	nombre    string
	intencion nucleo.TipoIntencion
	puntos    float64
	resp      nucleo.Respuesta
	err       error
	llamadas  atomic.Int32
}

func (r *resolutorFalso) Nombre() string { return r.nombre }
func (r *resolutorFalso) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	if !strings.Contains(p.Texto, "zork") {
		return nucleo.Reconocimiento{}
	}
	return nucleo.Reconocimiento{Puntos: r.puntos, Intencion: r.intencion, Lectura: "lectura de " + r.nombre}
}
func (r *resolutorFalso) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	r.llamadas.Add(1)
	return r.resp, r.err
}

func verificada(texto string) nucleo.Respuesta {
	return nucleo.Respuesta{Texto: texto, Exito: true, Nivel: nucleo.NivelCalculo, Metodo: "falso",
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelCalculo, Pasan: 1, Total: 1, Texto: "Comprobado"}}}
}

// responderQueAnota answers every question with the option at index i (or "" when free text) and records them.
type responderQueAnota struct {
	mu        sync.Mutex
	preguntas []nucleo.PreguntaUsuario
	opcion    int
}

func (r *responderQueAnota) responder(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preguntas = append(r.preguntas, p)
	if r.opcion < len(p.Opciones) {
		return p.Opciones[r.opcion], nil
	}
	return p.PorDefecto, nucleo.ErrSinRespuesta
}

// ---- ranking and verification ----

func TestRankingUsaPesosDeEstrategia(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	a := &resolutorFalso{nombre: "falso.a", intencion: nucleo.ICalculo, puntos: 0.6, resp: verificada("respuesta de A")}
	b := &resolutorFalso{nombre: "falso.b", intencion: nucleo.ICalculo, puntos: 0.55, resp: verificada("respuesta de B")}
	c.resolutores = []nucleo.Resolutor{a, b}

	r, _ := preguntar(t, c, "s1", "zork 7", nil)
	if !strings.Contains(r.Texto, "respuesta de A") {
		t.Fatalf("sin estadísticas debía ganar A (más puntos): %q", r.Texto)
	}
	for i := 0; i < 12; i++ {
		c.stats.Exito("estrategia:falso.a", false)
		c.stats.Exito("estrategia:falso.b", true)
	}
	cs := c.candidatos(&nucleo.Pregunta{Texto: "zork 7"})
	if len(cs) < 2 || cs[0].nombre != "falso.b" {
		t.Fatalf("con buena tasa B debía ir primero: %+v", cs)
	}
	if cs[0].score <= cs[0].puntos || cs[1].score >= cs[1].puntos {
		t.Errorf("score = puntos × (0.5 + tasa): %+v", cs)
	}
	r, _ = preguntar(t, c, "s2", "zork 7", nil)
	if !strings.Contains(r.Texto, "respuesta de B") {
		t.Fatalf("con las estadísticas debía ganar B: %q", r.Texto)
	}
}

func TestVerificadaGanaANoVerificada(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	noSe := &resolutorFalso{nombre: "falso.nose", intencion: nucleo.ICalculo, puntos: 0.9,
		resp: nucleo.Respuesta{Texto: "No lo sé", Exito: false, Nivel: nucleo.NivelSinComprobar}}
	sinComprobar := &resolutorFalso{nombre: "falso.sincomprobar", intencion: nucleo.ICalculo, puntos: 0.8,
		resp: nucleo.Respuesta{Texto: "Es 41", Exito: true, Nivel: nucleo.NivelSinComprobar}}
	bueno := &resolutorFalso{nombre: "falso.bueno", intencion: nucleo.ICalculo, puntos: 0.5, resp: verificada("Es 42")}
	c.resolutores = []nucleo.Resolutor{noSe, sinComprobar, bueno}
	r, tr := preguntar(t, c, "s", "zork", nil)
	if r.Texto != "Es 42" || r.Nivel != nucleo.NivelCalculo || !r.Exito {
		t.Fatalf("debía ganar la respuesta comprobada: %+v\n%s", r, titulos(tr))
	}
	if strings.Contains(todoElTexto(r), "Es 41") {
		t.Errorf("una respuesta sin comprobar no se muestra: %+v", r)
	}
	if noSe.llamadas.Load() != 1 || sinComprobar.llamadas.Load() != 1 {
		t.Errorf("los tres resolutores debían probarse en orden")
	}

	// alone, the honest unverified answer is shown as it is
	c2 := nuevoCerebroPrueba(t, nil)
	c2.resolutores = []nucleo.Resolutor{noSe}
	r, _ = preguntar(t, c2, "s", "zork", nil)
	if r.Texto != "No lo sé" || r.Nivel != nucleo.NivelSinComprobar || r.Exito {
		t.Errorf("respuesta honesta sin comprobar: %+v", r)
	}
	// an unverified "success" is refused
	c3 := nuevoCerebroPrueba(t, nil)
	c3.resolutores = []nucleo.Resolutor{sinComprobar}
	r, _ = preguntar(t, c3, "s", "zork", nil)
	if strings.Contains(todoElTexto(r), "Es 41") || r.Exito {
		t.Errorf("debía negarse a mostrar lo que no comprobó: %+v", r)
	}
}

func TestDosVerificadasQueDifierenPreguntan(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	a := &resolutorFalso{nombre: "falso.razonar", intencion: nucleo.ICalculo, puntos: 0.8, resp: verificada("Es 5")}
	b := &resolutorFalso{nombre: "falso.saber", intencion: nucleo.IPreguntarHecho, puntos: 0.78, resp: verificada("Es 7")}
	c.resolutores = []nucleo.Resolutor{a, b}
	rq := &responderQueAnota{opcion: 1}
	r, tr := preguntar(t, c, "s", "zork", rq.responder)
	if len(rq.preguntas) != 1 || !strings.Contains(rq.preguntas[0].Texto, "¿Cuál querías decir?") {
		t.Fatalf("debía preguntar cuál: %+v\n%s", rq.preguntas, titulos(tr))
	}
	if a.llamadas.Load() != 1 || b.llamadas.Load() != 1 {
		t.Errorf("los dos debían ejecutarse (en paralelo)")
	}
	if r.Texto != "Es 7" || !strings.Contains(strings.Join(r.Parrafos, " "), "Es 5") {
		t.Errorf("debía elegir la segunda y mostrar las dos: %+v", r)
	}
	// without an answer the first one stays (a fresh brain: the first run already moved the counters)
	c = nuevoCerebroPrueba(t, nil)
	c.resolutores = []nucleo.Resolutor{a, b}
	r, _ = preguntar(t, c, "s2", "zork", nil)
	if r.Texto != "Es 5" {
		t.Errorf("sin respuesta, la primera: %+v", r)
	}
}

// ---- reasoning through the real leaves ----

func TestRazonarEcuacionConMates(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, tr := preguntar(t, c, "s", "resuelve 2x+3=11", nil)
	if !strings.Contains(r.Texto, "x = 4") || r.Nivel != nucleo.NivelCalculo || !r.Exito {
		t.Fatalf("resuelve 2x+3=11: %+v\n%s", r, titulos(tr))
	}
	if !strings.Contains(todoElTexto(r), "Compruebo") {
		t.Errorf("falta la comprobación: %s", todoElTexto(r))
	}
	if len(r.Sugerencias) == 0 || !strings.Contains(strings.Join(r.Sugerencias, "|"), "¿Por qué?") {
		t.Errorf("sugerencias tras una ecuación: %v", r.Sugerencias)
	}
	if c.stats.Usos("estrategia:mates.ecuacion") != 1 {
		t.Errorf("el contador de estrategia debía moverse")
	}
}

func TestYSiFuera13(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "resuelve 2x+3=11", nil)
	if !strings.Contains(r.Texto, "x = 4") {
		t.Fatalf("primer turno: %+v", r)
	}
	r, tr := preguntar(t, c, "s", "¿y si fuera 13?", nil)
	if !strings.Contains(r.Texto, "x = 5") || !strings.Contains(r.Texto, "Con 13") || r.Nivel != nucleo.NivelCalculo {
		t.Fatalf("¿y si fuera 13?: %+v\n%s", r, titulos(tr))
	}
	if !strings.Contains(titulos(tr), "2x+3=13") {
		t.Errorf("la traza debía decir el texto cambiado:\n%s", titulos(tr))
	}
	// a second change works on the changed text
	r, _ = preguntar(t, c, "s", "¿y si fuera 15?", nil)
	if !strings.Contains(r.Texto, "x = 6") {
		t.Errorf("¿y si fuera 15?: %+v", r)
	}
	// ¿por qué? re-renders the steps without changing the context
	r, _ = preguntar(t, c, "s", "¿por qué?", nil)
	if len(r.Plan) == 0 || r.Nivel != nucleo.NivelCalculo {
		t.Errorf("¿por qué?: %+v", r)
	}
	r, _ = preguntar(t, c, "s", "¿y si fuera 21?", nil)
	if !strings.Contains(r.Texto, "x = 9") {
		t.Errorf("tras ¿por qué? el contexto sigue siendo la ecuación: %+v", r)
	}
}

func TestReemplazarNumeroMismaUnidad(t *testing.T) {
	seg := segNumero("¿y si fuera a 100 km/h?", "100")
	got, viejo, ok := reemplazarNumero("un tren va a 80 km/h durante 2 horas", seg)
	if !ok || got != "un tren va a 100 km/h durante 2 horas" || viejo != "80" {
		t.Errorf("misma unidad: %q %q %v", got, viejo, ok)
	}
	seg = segNumero("¿y si fuera 13?", "13")
	got, _, _ = reemplazarNumero("resuelve 2x+3=11", seg)
	if got != "resuelve 2x+3=13" {
		t.Errorf("último número: %q", got)
	}
}

func segNumero(texto, num string) lengua.Seguimiento {
	ns := lengua.Numeros(num)
	return lengua.Seguimiento{Tipo: "cambiar_numero", Texto: texto, Numero: &ns[0]}
}

func TestHechosTobyConLogica(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	for _, s := range []string{"recuerda que Toby es un perro", "recuerda que todos los perros son mamíferos"} {
		r, tr := preguntar(t, c, "s", s, nil)
		if !r.Exito {
			t.Fatalf("%s: %+v\n%s", s, r, titulos(tr))
		}
	}
	r, tr := preguntar(t, c, "s", "¿Toby es mamífero?", nil)
	if !strings.HasPrefix(r.Texto, "Sí") || r.Nivel != nucleo.NivelCalculo || !strings.Contains(todoElTexto(r), "perro") {
		t.Fatalf("¿Toby es mamífero?: %+v\n%s", r, titulos(tr))
	}
	if len(c.Hechos("")) == 0 || len(c.Hechos("Toby")) == 0 {
		t.Errorf("los hechos debían estar en memoria")
	}
}

func TestSignificaEnsenaPalabra(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, tr := preguntar(t, c, "s", "recuerda que capicúa significa palíndromo", nil)
	if !r.Exito {
		t.Fatalf("%+v\n%s", r, titulos(tr))
	}
	if c.Palabras()["capicua"] != "palindromo" {
		t.Fatalf("debía aprender la palabra: %v\n%s", c.Palabras(), titulos(tr))
	}
	if !strings.Contains(r.Aprendido, "capicua") {
		t.Errorf("Aprendido: %q", r.Aprendido)
	}
	if con, _ := c.lexico().Concepto("capicua"); con != "palindromo" {
		t.Errorf("el léxico de la próxima petición debe conocerla: %q", con)
	}
}

func TestNoSePlantearlo(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	fallo := &resolutorFalso{nombre: "falso.x", intencion: nucleo.ICalculo, puntos: 0.9, err: errors.Join(nucleo.ErrNoEntiendo)}
	c.resolutores = []nucleo.Resolutor{fallo}
	r, _ := preguntar(t, c, "s", "zork", nil)
	if r.Exito || r.Nivel != nucleo.NivelFallo || !strings.Contains(r.Parrafos[len(r.Parrafos)-1], "No sé plantearlo. Prueba a escribirlo así:") {
		t.Fatalf("debía terminar con «No sé plantearlo»: %+v", r)
	}
	if len(c.Pendientes()) != 1 {
		t.Errorf("un fallo va a pendientes: %v", c.Pendientes())
	}
	if c.stats.Tasa("estrategia:falso.x") >= 0.5 {
		t.Errorf("el resolutor que falla baja su tasa")
	}
}

func TestOtraFormaRazonada(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	a := &resolutorFalso{nombre: "falso.a", intencion: nucleo.ICalculo, puntos: 0.9, resp: verificada("Así: A")}
	b := &resolutorFalso{nombre: "falso.b", intencion: nucleo.ICalculo, puntos: 0.5, resp: verificada("Así: B")}
	c.resolutores = []nucleo.Resolutor{a, b}
	r, _ := preguntar(t, c, "s", "zork 3", nil)
	if r.Texto != "Así: A" {
		t.Fatalf("%+v", r)
	}
	r, _ = preguntar(t, c, "s", "otra forma", nil)
	if r.Texto != "Así: B" {
		t.Fatalf("otra forma debía usar el otro resolutor: %+v", r)
	}
	r, _ = preguntar(t, c, "s", "de otra forma", nil)
	if !strings.Contains(r.Texto, "No conozco otra forma") {
		t.Fatalf("sin más resolutores: %+v", r)
	}
}

// ---- sessions and questions ----

func TestPreguntaPendienteRespondidaEscribiendo(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	var recibida atomic.Value
	c.pipes.crearFuncion = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		v, s, ok := c.preguntarValor(k, "¿Qué debe dar SumaPares([-2])?", []string{"-2", "0"}, nucleo.TInt)
		recibida.Store(s)
		if !ok {
			return nucleo.Respuesta{Texto: "sin respuesta"}
		}
		return nucleo.Respuesta{Texto: "Me dijiste " + nucleo.FormatoHumano(v, nucleo.TInt), Exito: true}
	}
	// the first task's trace has a responder that never answers (the UI card stays open)
	bloquea := func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		<-ctx.Done()
		return p.PorDefecto, nucleo.ErrSinRespuesta
	}
	hecho := make(chan nucleo.Respuesta, 1)
	go func() {
		tr := nucleo.NuevaTraza(nil, bloquea)
		hecho <- c.Responder(context.Background(), nucleo.Peticion{Tarea: "a", Sesion: "s", Texto: "haz una función que sume los números pares de una lista"}, tr)
	}()
	s := c.sesion("s")
	limite := time.Now().Add(5 * time.Second)
	for {
		s.mu.Lock()
		n := len(s.pendientes)
		s.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(limite) {
			t.Fatal("la pregunta nunca quedó pendiente")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// text that is not an answer starts a normal task
	if _, ok := s.pendienteQueAcepta("hola"); ok != "" {
		t.Errorf("«hola» no responde a la pregunta")
	}
	r, _ := preguntar(t, c, "s", "0", nil)
	if !strings.Contains(r.Texto, "Lo tomo como respuesta") {
		t.Fatalf("el texto debía ir a la pregunta pendiente: %+v", r)
	}
	select {
	case r = <-hecho:
	case <-time.After(5 * time.Second):
		t.Fatal("la primera tarea no recibió la respuesta")
	}
	if r.Texto != "Me dijiste 0" {
		t.Fatalf("respuesta de la primera tarea: %+v (recibida %v)", r, recibida.Load())
	}
	if n := s.numTurnos(); n != 1 {
		t.Errorf("la respuesta escrita no es un turno nuevo: %d turnos", n)
	}
}

func TestSesionTurnosYCaducidad(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	ahora := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	c.ahora = func() time.Time { return ahora }
	for i := 0; i < MaxTurnos+5; i++ {
		preguntar(t, c, "s", "hola", nil)
	}
	if n := c.sesion("s").numTurnos(); n != MaxTurnos {
		t.Errorf("como mucho %d turnos: %d", MaxTurnos, n)
	}
	ahora = ahora.Add(Caducidad + time.Hour)
	if n := c.sesion("s").numTurnos(); n != 0 {
		t.Errorf("tras 24 h sin uso la sesión empieza de cero: %d", n)
	}
}

// ---- wording policy ----

func TestAfirmarCreoQue(t *testing.T) {
	casos := []struct {
		texto string
		comp  *nucleo.Comprobacion
		want  string
	}{
		{"Devuelve la suma de los pares", nil, "Creo que devuelve la suma de los pares"},
		{"Devuelve 6", &nucleo.Comprobacion{Nivel: nucleo.NivelSinComprobar}, "Creo que devuelve 6"},
		{"Devuelve 6", &nucleo.Comprobacion{Nivel: nucleo.NivelFallo}, "Creo que devuelve 6"},
		{"Devuelve 6", &nucleo.Comprobacion{Nivel: nucleo.NivelEntendido}, "Devuelve 6"},
		{"Devuelve 6", &nucleo.Comprobacion{Nivel: nucleo.NivelCalculo}, "Devuelve 6"},
		{"SumaPares devuelve 6", nil, "Creo que SumaPares devuelve 6"},
		{"Creo que es cuadrática", nil, "Creo que es cuadrática"},
		{"", nil, ""},
	}
	for _, cs := range casos {
		if got := afirmar(cs.texto, cs.comp); got != cs.want {
			t.Errorf("afirmar(%q) = %q, quiero %q", cs.texto, got, cs.want)
		}
	}
}

func TestRedactarPolitica(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	k := c.nuevaTarea(context.Background(), nucleo.Peticion{Tarea: "x"}, c.sesion("s"), nil)
	k.intencion = nucleo.ICrearFuncion
	var filas [][]string
	for i := 0; i < 20; i++ {
		filas = append(filas, []string{"a", "b"})
	}
	r := nucleo.Respuesta{
		Texto:    strings.Repeat("palabra ", 40),
		Parrafos: []string{"1", "2", "3", "4", "5", "6", "7", "8"},
		Codigo:   "package solucion",
		Exito:    true,
		Comprobaciones: []nucleo.Comprobacion{
			{Nivel: nucleo.NivelPropiedades, Pasan: 200, Total: 200},
			{Nivel: nucleo.NivelEntendido, Pasan: 264, Total: 264},
		},
		Nivel:  nucleo.NivelTusEjemplos, // claimed, but no comprobación backs it
		Tablas: []nucleo.Tabla{{Cabecera: []string{"x", "y"}, Filas: filas}, {Cabecera: []string{"Entrada", "Esperado", "Obtenido"}, Filas: filas}},
	}
	c.redactar(k, &r)
	if n := len([]rune(r.Texto)); n > MaxPrimeraLinea {
		t.Errorf("primera línea de %d runas", n)
	}
	if len(r.Parrafos) != MaxParrafos {
		t.Errorf("párrafos: %d", len(r.Parrafos))
	}
	if r.Comprobaciones[0].Nivel != nucleo.NivelEntendido || r.Comprobaciones[0].Texto == "" {
		t.Errorf("orden fijo de sellos y texto por defecto: %+v", r.Comprobaciones)
	}
	if r.Nivel != nucleo.NivelEntendido {
		t.Errorf("el nivel no puede subir sin comprobación: %s", r.Nivel)
	}
	if len(r.Tablas[0].Filas) != MaxFilas || len(r.Tablas[1].Filas) != 20 {
		t.Errorf("tablas: %d y %d filas", len(r.Tablas[0].Filas), len(r.Tablas[1].Filas))
	}
	if !strings.Contains(strings.Join(r.Parrafos, "\n"), "No me diste ejemplos: comprobé que el código hace lo que entendí.") {
		t.Errorf("falta la frase sin ejemplos: %q", r.Parrafos)
	}
	if !strings.Contains(strings.Join(r.Sugerencias, "|"), "Pruébala con otros datos") {
		t.Errorf("sugerencias tras código: %v", r.Sugerencias)
	}
	// a failing group makes the whole answer a failure
	r2 := nucleo.Respuesta{Texto: "x", Exito: true, Nivel: nucleo.NivelTusEjemplos,
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelTusEjemplos, Pasan: 3, Total: 3}, {Nivel: nucleo.NivelFallo, Pasan: 1, Total: 2}}}
	c.redactar(k, &r2)
	if r2.Nivel != nucleo.NivelFallo || r2.Exito || r2.Comprobaciones[0].Nivel != nucleo.NivelFallo {
		t.Errorf("con un grupo que falla: %+v", r2)
	}
}

// ---- templates ----

func TestPlantillasCharla(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	casos := map[string]string{
		"hola":                 "Hola",
		"¿qué sabes hacer?":    "funciones",
		"¿eres una IA?":        "no un modelo de lenguaje",
		"¿cómo aprendes?":      "memorizando",
		"¿qué no sabes hacer?": "no sé hacer",
		"¿cómo te llamas?":     "Nyx Código",
	}
	for texto, quiero := range casos {
		r, tr := preguntar(t, c, "s-"+texto, texto, nil)
		if !strings.Contains(todoElTexto(r), quiero) {
			t.Errorf("%q: quiero %q en %q\n%s", texto, quiero, todoElTexto(r), titulos(tr))
		}
		if len(r.Comprobaciones) != 0 || r.Nivel != nucleo.NivelSinComprobar {
			t.Errorf("%q: una plantilla no dice que comprobó nada: %+v", texto, r)
		}
	}
	if len(c.Pendientes()) != 0 {
		t.Errorf("la charla no va a pendientes: %v", c.Pendientes())
	}
}

func TestSeguimientoNoSeAplicaAPlantillas(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	preguntar(t, c, "s", "hola", nil)
	r, _ := preguntar(t, c, "s", "gracias", nil)
	if !strings.Contains(r.Texto, "De nada") {
		t.Errorf("«gracias» tras un saludo es charla: %+v", r)
	}
	r, _ = preguntar(t, c, "s", "¿por qué?", nil)
	if r.Metodo == "seguimiento" {
		t.Errorf("«¿por qué?» tras un saludo no repasa nada: %+v", r)
	}
}

func TestNoTeEntiendo(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "asdf qwer zxcv", nil)
	if r.Texto != "No te entiendo." || len(r.Sugerencias) != 3 || !strings.Contains(strings.Join(r.Parrafos, " "), "Prueba a escribirlo así") {
		t.Fatalf("plantilla «no te entiendo»: %+v", r)
	}
	if len(c.Pendientes()) != 1 {
		t.Errorf("va a pendientes: %v", c.Pendientes())
	}
}

func funcionPrueba() nucleo.FuncionAprendida {
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	return nucleo.FuncionAprendida{
		Nombre: "SumaPares", Firma: f, Nivel: nucleo.NivelEntendido, Origen: "sintesis",
		Codigo:      "package solucion\n\nfunc SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n%2 == 0 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total\n}\n",
		Descripcion: "la suma de los números pares de nums", Conceptos: []string{"sumar", "par"},
		Huella: "h1", Creada: time.Now(), Usada: time.Now(),
	}
}

func TestMemoriaFuncionesYOlvidar(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "¿qué funciones sabes?", nil)
	if !strings.Contains(r.Texto, "ninguna función") {
		t.Errorf("sin funciones: %+v", r)
	}
	if _, err := c.mem.GuardarFuncion(funcionPrueba()); err != nil {
		t.Fatal(err)
	}
	r, _ = preguntar(t, c, "s", "¿qué funciones sabes?", nil)
	if len(r.Tablas) != 1 || r.Tablas[0].Filas[0][0] != "SumaPares" {
		t.Fatalf("tabla de funciones: %+v", r)
	}
	if fs := c.Funciones("pares"); len(fs) != 1 {
		t.Errorf("Funciones(q): %d", len(fs))
	}
	// without confirmation nothing is forgotten
	r, _ = preguntar(t, c, "s", "olvida SumaPares", nil)
	if _, ok := c.mem.Funcion("SumaPares"); !ok || !strings.Contains(r.Texto, "confirmes") {
		t.Fatalf("sin confirmar no se olvida: %+v", r)
	}
	r, _ = preguntar(t, c, "s", "olvida SumaPares", nucleotest.RespuestasFijas("Sí, olvídalo"))
	if _, ok := c.mem.Funcion("SumaPares"); ok || !strings.Contains(r.Texto, "he olvidado") {
		t.Fatalf("con confirmación se olvida: %+v", r)
	}
	if err := c.Olvidar("cosa", "x"); err == nil {
		t.Errorf("tipo desconocido")
	}
}

// ---- follow-ups through the pipeline hooks ----

func TestEstaMalGuardaContraejemploYRepite(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	fa := funcionPrueba()
	var llamadas []*tarea
	c.pipes.crearFuncion = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		llamadas = append(llamadas, k)
		g := fa
		k.turno.Funcion = &g
		return nucleo.Respuesta{Texto: "Aquí la tienes.", Codigo: fa.Codigo, Exito: true, Nivel: nucleo.NivelEntendido,
			Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Pasan: 10, Total: 10}}}
	}
	r, _ := preguntar(t, c, "s", "haz una función que sume los números pares de una lista", nil)
	if !r.Exito || len(llamadas) != 1 {
		t.Fatalf("primer turno: %+v", r)
	}
	r, tr := preguntar(t, c, "s", "está mal: con [2] debe dar 0", nil)
	if len(llamadas) != 2 {
		t.Fatalf("debía repetir el pipeline: %d llamadas\n%s", len(llamadas), titulos(tr))
	}
	k := llamadas[1]
	if len(k.ejemplosExtra) != 1 || k.ejemplosExtra[0].Expectativa != nucleo.EspUsuario || k.ejemplosExtra[0].Origen != nucleo.OrigenContraejemplo {
		t.Errorf("el contraejemplo llega al pipeline: %+v", k.ejemplosExtra)
	}
	ces := c.contraejemplos(fa.Conceptos, fa.Firma)
	if len(ces) != 1 || !nucleo.Igual(ces[0].Esperado[0], 0) {
		t.Fatalf("contraejemplo guardado: %+v", ces)
	}
	if !strings.Contains(titulos(tr), "Guardo el contraejemplo: con [2] debe dar 0") {
		t.Errorf("traza:\n%s", titulos(tr))
	}
	// "ahora con impares" fuses the request and runs the pipeline again
	preguntar(t, c, "s", "ahora con impares", nil)
	if len(llamadas) != 3 || !strings.Contains(llamadas[2].q.Texto, "impares") {
		t.Errorf("modificar: %d llamadas", len(llamadas))
	}
	// "está bien" is bookkeeping only
	r, _ = preguntar(t, c, "s", "está bien", nil)
	if r.Texto != "Gracias, lo tendré en cuenta." || len(llamadas) != 3 {
		t.Errorf("está bien: %+v", r)
	}
}

func TestEnConstruccionEsHonesto(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "haz una función que sume los números pares de una lista", nil)
	if r.Exito || r.Nivel != nucleo.NivelSinComprobar || !strings.Contains(r.Texto, "Todavía no sé hacer esto") || len(r.Comprobaciones) != 0 {
		t.Fatalf("un pipeline sin hacer responde con honestidad: %+v", r)
	}
	if c.stats.Usos("estrategia:crear_funcion") != 0 {
		t.Errorf("un pipeline sin hacer no mueve las estadísticas")
	}
}

func TestErrorInterno(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	c.pipes.crearFuncion = func(c *Cerebro, k *tarea) nucleo.Respuesta { panic("roto") }
	var fin string
	tr := nucleo.NuevaTraza(func(e nucleo.Evento) {
		if e.Fin != nil {
			fin = e.Fin.Estado
		}
	}, nil)
	r := c.Responder(context.Background(), nucleo.Peticion{Tarea: "x", Sesion: "s", Texto: "haz una función que sume los números pares de una lista"}, tr)
	if !strings.Contains(r.Texto, "error interno") || r.Nivel != nucleo.NivelFallo || fin != "error" {
		t.Fatalf("error interno: %+v (fin %q)", r, fin)
	}
}

func TestAccionesYPensarMas(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	var presupuestos []time.Duration
	var pensar []bool
	c.pipes.reparar = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		presupuestos = append(presupuestos, k.presupuesto)
		pensar = append(pensar, k.pensarMas)
		if k.codigo != "package main" {
			t.Errorf("el código de la petición: %q", k.codigo)
		}
		return nucleo.Respuesta{Texto: "arreglado", Exito: true}
	}
	tr := nucleo.NuevaTraza(nil, nil)
	c.Responder(context.Background(), nucleo.Peticion{Tarea: "r1", Sesion: "s", Accion: "arreglar", Codigo: "package main"}, tr)
	c.Responder(context.Background(), nucleo.Peticion{Tarea: "r2", Sesion: "s", Accion: "pensar_mas", Previa: "r1"}, nucleo.NuevaTraza(nil, nil))
	c.Responder(context.Background(), nucleo.Peticion{Tarea: "r3", Sesion: "s", Accion: "pensar_mas", Previa: "r2"}, nucleo.NuevaTraza(nil, nil))
	c.Responder(context.Background(), nucleo.Peticion{Tarea: "r4", Sesion: "s", Accion: "pensar_mas", Previa: "r3"}, nucleo.NuevaTraza(nil, nil))
	want := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 120 * time.Second}
	if len(presupuestos) != 4 {
		t.Fatalf("llamadas: %v", presupuestos)
	}
	for i := range want {
		if presupuestos[i] != want[i] {
			t.Errorf("presupuesto %d: %v, quiero %v", i, presupuestos[i], want[i])
		}
	}
	if pensar[0] || !pensar[1] {
		t.Errorf("pensarMas: %v", pensar)
	}
}

func TestRelojSePausa(t *testing.T) {
	r := nuevoReloj(context.Background(), 60*time.Millisecond)
	reanudar := r.Pausar()
	time.Sleep(120 * time.Millisecond)
	if r.Err() != nil {
		t.Fatal("el presupuesto no corre mientras se espera al usuario")
	}
	reanudar()
	reanudar() // idempotent
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("el presupuesto debía acabarse")
	}
	if !errors.Is(r.Err(), context.DeadlineExceeded) {
		t.Errorf("Err: %v", r.Err())
	}
	padre, cancelar := context.WithCancel(context.Background())
	r2 := nuevoReloj(padre, time.Hour)
	cancelar()
	<-r2.Done()
	if !errors.Is(r2.Err(), context.Canceled) {
		t.Errorf("cancelar la petición cancela el presupuesto: %v", r2.Err())
	}
}

func TestEstadoRiesgosYOpinar(t *testing.T) {
	ej := &nucleotest.EjecutorFalso{}
	c := nuevoCerebroPrueba(t, ej)
	e := c.Estado()
	if e.Version != Version || e.Internet || e.Arenero.Nivel == "" {
		t.Errorf("estado: %+v", e)
	}
	if v := c.Riesgos("package main\n\nimport \"os/exec\"\n\nfunc main() { exec.Command(\"ls\").Run() }\n"); len(v) == 0 {
		t.Errorf("os/exec debía dar riesgos")
	}
	if v := c.Riesgos("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n"); len(v) != 0 {
		t.Errorf("fmt no tiene riesgos: %+v", v)
	}
	r, _ := preguntar(t, c, "s", "resuelve 2x+3=11", nil)
	antes := c.stats.Tasa("estrategia:mates.ecuacion")
	if err := c.Opinar(nucleo.Opinion{Sesion: "s", Tarea: r.Tarea, Correcto: true}); err != nil {
		t.Fatal(err)
	}
	if c.stats.Tasa("estrategia:mates.ecuacion") <= antes {
		t.Errorf("una opinión buena sube la tasa")
	}
}

func TestFlechasDatosOLogica(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	casos := map[string]string{
		"[1,2,3,4] -> 6; [5,7] -> 0; [] -> 0":                     "crear_funcion",
		"primos hasta n: 10 -> [2,3,5,7]":                         "crear_funcion",
		"invierte una cadena: hola -> aloh":                       "crear_funcion",
		"función que diga si una palabra es capicúa: ana -> true": "crear_funcion",
		"p -> q; p; ¿q?":                                          "logica.proposicional",
		"a -> b; no b. ¿a?":                                       "logica.proposicional",
	}
	for texto, quiero := range casos {
		k := c.nuevaTarea(context.Background(), nucleo.Peticion{Texto: texto}, c.sesion("x"), nil)
		cs := c.candidatos(c.analizar(k, texto, ""))
		if len(cs) == 0 || cs[0].nombre != quiero {
			t.Errorf("%q: gana %+v, quiero %s", texto, cs, quiero)
			continue
		}
		if len(cs) > 1 && cs[1].familia != cs[0].familia && cs[1].score >= (1-margenDuda)*cs[0].score {
			t.Errorf("%q: no debería hacer falta preguntar: %+v", texto, cs[:2])
		}
	}
}
