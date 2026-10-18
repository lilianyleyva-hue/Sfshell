package cerebro

import (
	"fmt"
	"strings"
	"testing"

	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/saber"
)

// Regression tests for the gaps the completeness review found in follow-ups, memory, "otra forma",
// rules, source quality and habla. They use the fake sandbox and pipeline hooks, so they are fast.

// ejecutorSumaPares is a fake sandbox whose only function sums the even numbers of its list.
func ejecutorSumaPares() *nucleotest.EjecutorFalso {
	return &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func(in []nucleo.Valor) ([]nucleo.Valor, string) {
			total := 0
			xs, _ := in[0].([]nucleo.Valor)
			for _, v := range xs {
				if n, _ := v.(int); n%2 == 0 {
					total += n
				}
			}
			return []nucleo.Valor{total}, ""
		},
	}}
}

// crearFijo is a crear_funcion hook that "creates" funcionPrueba() and records each call.
func crearFijo(llamadas *[]*tarea) pipeline {
	fa := funcionPrueba()
	return func(c *Cerebro, k *tarea) nucleo.Respuesta {
		*llamadas = append(*llamadas, k)
		g := fa
		k.turno.Funcion = &g
		return nucleo.Respuesta{Texto: "Aquí la tienes.", Codigo: fa.Codigo, Exito: true, Nivel: nucleo.NivelEntendido, Metodo: "sintesis",
			Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Pasan: 10, Total: 10, Texto: "10/10 según lo que entendí"}}}
	}
}

// G1: after a function reused from memory («Ya sé hacer esto», Metodo "memoria"), «¿por qué?»,
// «explícalo con detalle», «está mal» (the «No es eso» button) and «está mal: con [2] debe dar 0» are
// follow-ups, not new requests; the memory hit also moves the crear_funcion strategy counter.
func TestSeguimientosTrasAciertoDeMemoria(t *testing.T) {
	c := nuevoCerebroPrueba(t, ejecutorSumaPares())
	r, tr := preguntar(t, c, "m1", pedidoSumaPares, nucleotest.RespuestasFijas())
	if !r.Exito || r.Aprendido == "" {
		t.Fatalf("primera vez: %s [%s]\n%s", r.Texto, r.Nivel, titulos(tr))
	}
	antes := c.stats.Usos("estrategia:crear_funcion")
	r, tr = preguntar(t, c, "m2", pedidoSumaPares, nucleotest.RespuestasFijas())
	if r.Metodo != "memoria" || !strings.Contains(r.Texto, "Ya sé hacer esto") {
		t.Fatalf("la segunda vez sale de la memoria: %s %q\n%s", r.Metodo, r.Texto, titulos(tr))
	}
	if c.stats.Usos("estrategia:crear_funcion") != antes+1 {
		t.Errorf("un acierto de memoria es crear_funcion haciendo su trabajo: usos %d → %d", antes, c.stats.Usos("estrategia:crear_funcion"))
	}
	r, tr = preguntar(t, c, "m2", "¿por qué?", nil)
	if r.Metodo != "seguimiento" || r.Texto == "No te entiendo." || !strings.Contains(todoElTexto(r), "Lo comprobé así") {
		t.Errorf("¿por qué? tras la memoria: %s %q\n%s", r.Metodo, r.Texto, titulos(tr))
	}
	var explicadas []*tarea
	c.pipes.explicar = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		explicadas = append(explicadas, k)
		return nucleo.Respuesta{Texto: "Explicado.", Exito: true, Nivel: nucleo.NivelSinComprobar}
	}
	preguntar(t, c, "m2", "explícalo con detalle", nil)
	if len(explicadas) != 1 || explicadas[0].detalle != 2 || !strings.Contains(explicadas[0].codigo, "func SumaPares") {
		t.Errorf("explícalo con detalle usa el código recordado: %d llamadas", len(explicadas))
	}
	// «No es eso» sends «está mal»: without an example it asks for one
	r, _ = preguntar(t, c, "m2", "está mal", nil)
	if r.Metodo != "seguimiento" || !strings.HasPrefix(r.Texto, "Dime un ejemplo") {
		t.Errorf("«está mal» sin ejemplo: %s %q", r.Metodo, r.Texto)
	}
	var rehechas []*tarea
	c.pipes.crearFuncion = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		rehechas = append(rehechas, k)
		return c.respuestaFallo(k, "no encontré un programa que haga lo que pides", "", "")
	}
	_, tr = preguntar(t, c, "m2", "está mal: con [2] debe dar 0", nil)
	if len(rehechas) != 1 || len(rehechas[0].ejemplosExtra) != 1 {
		t.Fatalf("«está mal: con [2] debe dar 0» repite crear_funcion con el contraejemplo: %d\n%s", len(rehechas), titulos(tr))
	}
	g, _ := c.mem.Funcion("SumaPares")
	if ces := c.contraejemplos(g.Conceptos, g.Firma); len(ces) != 1 || !nucleo.Igual(ces[0].Esperado[0], 0) {
		t.Errorf("guarda el contraejemplo para la función recordada: %+v", ces)
	}
	for _, p := range c.Pendientes() {
		if strings.Contains(p.Texto, "por qué") || strings.Contains(p.Texto, "con detalle") {
			t.Errorf("un seguimiento no es una pregunta pendiente: %+v", p)
		}
	}
}

// G2: «usa SumaPares para [1,2,3,4]» takes the signature of the stored function named, reuses it
// (exact name match) without asking «¿Qué recibe la función?», and runs it on that input.
func TestUsaFuncionGuardada(t *testing.T) {
	ej := ejecutorSumaPares()
	c := nuevoCerebroPrueba(t, ej)
	if _, err := c.mem.GuardarFuncion(funcionPrueba()); err != nil {
		t.Fatal(err)
	}
	r, tr := preguntar(t, c, "u", "usa SumaPares para [1,2,3,4]", nil)
	tt := titulos(tr)
	if strings.Contains(tt, "¿Qué recibe la función?") {
		t.Errorf("no pregunta qué recibe: ya lo sabe por el nombre\n%s", tt)
	}
	if r.Metodo != "memoria" || !r.Exito || !strings.HasPrefix(r.Texto, "`SumaPares([1, 2, 3, 4])` = 6") {
		t.Fatalf("usa SumaPares: %s %q [%s]\n%s", r.Metodo, r.Texto, r.Nivel, tt)
	}
	if !strings.Contains(strings.Join(r.Parrafos, " "), "Ya sé hacer esto") {
		t.Errorf("dice que la tenía en memoria: %q", r.Parrafos)
	}
	if ps := pasosDeTipo(tr, nucleo.PasoIntento); len(ps) > 0 {
		t.Errorf("no construye nada: %q", ps)
	}
	// "aplica SumaPares a la lista [2, 5, 8]" and a bare "usa SumaPares"
	r, _ = preguntar(t, c, "u2", "aplica SumaPares a la lista [2, 5, 8]", nil)
	if !strings.HasPrefix(r.Texto, "`SumaPares([2, 5, 8])` = 10") {
		t.Errorf("aplica: %q", r.Texto)
	}
	r, _ = preguntar(t, c, "u3", "usa SumaPares", nil)
	if r.Metodo != "memoria" || !strings.Contains(r.Texto, "Ya sé hacer esto") {
		t.Errorf("usa SumaPares sin datos: %s %q", r.Metodo, r.Texto)
	}
}

func TestCrearEntradaPedida(t *testing.T) {
	f := funcionPrueba().Firma
	dos := nucleo.Firma{Nombre: "Maximo", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	casos := []struct {
		texto  string
		f      nucleo.Firma
		quiero string
	}{
		{"usa SumaPares para [1,2,3,4]", f, "[1,2,3,4]"},
		{"usa la función SumaPares con la lista [5, 6]", f, "[5,6]"},
		{"aplica Maximo a 3 y 5", dos, "3,5"},
		{"usa Maximo para 3, 9", dos, "3,9"},
		{"usa SumaPares para sumar cosas", f, ""},
		{"usa SumaPares", f, ""},
	}
	for _, cs := range casos {
		ent, ok := crearEntradaPedida(cs.texto, cs.f)
		var partes []string
		for _, v := range ent {
			partes = append(partes, strings.ReplaceAll(nucleo.Clave(v), " ", ""))
		}
		got := strings.Join(partes, ",")
		if ok != (cs.quiero != "") || (ok && !strings.Contains(got, strings.Trim(cs.quiero, "[]"))) {
			t.Errorf("%q: %v %q, quiero %q", cs.texto, ok, got, cs.quiero)
		}
	}
}

// G3: «ejecútalo con entrada 5» runs the code with stdin «5» (ejecutar), «pruébala con entrada [3,3]»
// tests it with [3,3] (probar).
func TestEjecutaloConEntrada(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	var creadas []*tarea
	c.pipes.crearFuncion = crearFijo(&creadas)
	var ejecutadas, probadas []string
	c.pipes.ejecutar = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		ejecutadas = append(ejecutadas, ejcEntradaDelTexto(k)+"|"+fmt.Sprint(strings.Contains(k.codigo, "func SumaPares")))
		return nucleo.Respuesta{Texto: "Ejecutado.", Exito: true, Nivel: nucleo.NivelSinComprobar}
	}
	c.pipes.probar = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		probadas = append(probadas, k.probarCon)
		return nucleo.Respuesta{Texto: "Probado.", Exito: true, Nivel: nucleo.NivelSinComprobar}
	}
	preguntar(t, c, "e", pedidoSumaPares, nil)
	preguntar(t, c, "e", "ejecútalo con entrada 5", nil)
	preguntar(t, c, "e", "pruébala con entrada [3,3]", nil)
	preguntar(t, c, "e", "ejecútala con [1,2]", nil)
	preguntar(t, c, "e", "pruébala con [4]", nil)
	if len(ejecutadas) != 2 || ejecutadas[0] != "5|true" || ejecutadas[1] != "[1,2]|true" {
		t.Errorf("ejecutar: %q", ejecutadas)
	}
	if len(probadas) != 2 || probadas[0] != "[3,3]" || probadas[1] != "[4]" {
		t.Errorf("probar: %q", probadas)
	}
}

// G4: «ahora con impares» changes the request that created the code, also after follow-ups about that
// code and after a «No te entiendo»; after «hazlo programa» the new version is a program again.
func TestModificarTrasSeguimientos(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	var creadas []*tarea
	c.pipes.crearFuncion = crearFijo(&creadas)
	c.pipes.probar = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		return nucleo.Respuesta{Texto: "Probado.", Exito: true, Nivel: nucleo.NivelSinComprobar, Metodo: "pruebas"}
	}
	var programas []*tarea
	c.pipes.crearPrograma = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		programas = append(programas, k)
		return nucleo.Respuesta{Texto: "Programa.", Codigo: "package main\n\nfunc main() {}\n", Exito: true, Nivel: nucleo.NivelEntendido,
			Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Pasan: 1, Total: 1}}}
	}
	preguntar(t, c, "s", pedidoSumaPares, nil)
	preguntar(t, c, "s", "pruébala con [3,3]", nil)
	if r, _ := preguntar(t, c, "s", "asdf qwer zxcv", nil); r.Metodo != "no_entiendo" {
		t.Fatalf("quería un «no te entiendo»: %+v", r)
	}
	r, tr := preguntar(t, c, "s", "¿por qué?", nil)
	if r.Metodo != "seguimiento" {
		t.Errorf("«¿por qué?» tras un malentendido habla del turno de antes: %s %q\n%s", r.Metodo, r.Texto, titulos(tr))
	}
	r, tr = preguntar(t, c, "s", "ahora con impares", nil)
	if len(creadas) != 2 {
		t.Fatalf("«ahora con impares» tras pruébala y un malentendido: %d llamadas, %q\n%s", len(creadas), r.Texto, titulos(tr))
	}
	if q := creadas[1].q.Texto; !strings.Contains(q, "sume los números pares") || !strings.Contains(q, "impares") {
		t.Errorf("se une con la petición que creó el código: %q", q)
	}
	preguntar(t, c, "s", "hazlo programa", nil)
	if len(programas) != 1 || !programas[0].envolver {
		t.Fatalf("hazlo programa: %d", len(programas))
	}
	preguntar(t, c, "s", "ahora con negativos", nil)
	if len(programas) != 2 || len(creadas) != 2 {
		t.Fatalf("tras «hazlo programa», la nueva versión es un programa: %d programas, %d funciones", len(programas), len(creadas))
	}
	k := programas[1]
	if k.envolver || !k.sinPlantillas || !strings.Contains(k.q.Texto, "impares") || !strings.Contains(k.q.Texto, "negativos") {
		t.Errorf("programa nuevo desde la última petición: envolver=%v sinPlantillas=%v %q", k.envolver, k.sinPlantillas, k.q.Texto)
	}
	// a request of another kind in between ends the line: nothing to modify
	preguntar(t, c, "s2", pedidoSumaPares, nil)
	preguntar(t, c, "s2", "resuelve 2x+3=11", nil)
	n := len(creadas)
	preguntar(t, c, "s2", "ahora con impares", nil)
	if len(creadas) != n {
		t.Errorf("tras una ecuación, «ahora con impares» no rehace la función")
	}
}

// G6: «otra forma» with nothing new is not a failure: no «No lo he conseguido», nothing in pendientes,
// no strategy loses points, and «¿por qué?» still talks about the answer that was solved.
func TestOtraFormaSinOtraEsHonesta(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	var creadas []*tarea
	primera := crearFijo(&creadas)
	c.pipes.crearFuncion = func(c *Cerebro, k *tarea) nucleo.Respuesta {
		if len(creadas) == 0 {
			return primera(c, k)
		}
		creadas = append(creadas, k)
		r := c.respuestaFallo(k, "no encontré un programa que haga lo que pides", "", "")
		r.Metodo = "sintesis"
		return r
	}
	preguntar(t, c, "s", "haz una función que devuelva el mayor de dos números", nil)
	usos := c.stats.Usos("estrategia:crear_funcion")
	r, tr := preguntar(t, c, "s", "otra forma", nil)
	if len(creadas) != 2 {
		t.Fatalf("otra forma vuelve a intentarlo: %d", len(creadas))
	}
	if r.Texto != "No conozco otra forma de hacerlo: solo sé hacerlo como te mostré." || r.Exito || r.Nivel != nucleo.NivelSinComprobar ||
		strings.Contains(todoElTexto(r), "No lo he conseguido") {
		t.Errorf("otra forma sin otra: %q [%s] %v\n%s", r.Texto, r.Nivel, r.Exito, titulos(tr))
	}
	if ps := c.Pendientes(); len(ps) != 0 {
		t.Errorf("lo ya resuelto no va a pendientes: %+v", ps)
	}
	if c.stats.Usos("estrategia:crear_funcion") != usos {
		t.Errorf("no mueve la estrategia")
	}
	r, _ = preguntar(t, c, "s", "¿por qué?", nil)
	if r.Metodo != "seguimiento" || !strings.Contains(todoElTexto(r), "10/10 según lo que entendí") {
		t.Errorf("¿por qué? habla de la respuesta resuelta: %+v", r)
	}
	// the same code shown again as a success is not "another way" either
	c.pipes.crearFuncion = crearFijo(&creadas)
	r, _ = preguntar(t, c, "s", "otra forma", nil)
	if !strings.HasPrefix(r.Texto, "No conozco otra forma") {
		t.Errorf("el mismo código no es otra forma: %q", r.Texto)
	}
}

// G5: «otra forma» after a template program never shows the same template again.
func TestOtraFormaNoRepitePlantilla(t *testing.T) {
	ej := &nucleotest.EjecutorFalso{Programa: func(fuente string, cp nucleo.CasoPrograma) (nucleo.Ejecucion, error) {
		return nucleo.Ejecucion{Salida: cp.Esperado}, nil // the template does what its cases expect
	}}
	c := nuevoCerebroPrueba(t, ej)
	r1, tr := preguntar(t, c, "p", "programa que dice si un número es par o impar", nucleotest.RespuestasFijas())
	if r1.Metodo != "plantilla" || !r1.Exito {
		t.Fatalf("plantilla: %s %q\n%s", r1.Metodo, r1.Texto, titulos(tr))
	}
	prev, _ := c.sesion("p").ultimo()
	if len(prev.Excluidos) != 1 || !strings.HasPrefix(prev.Excluidos[0], "plantilla:") {
		t.Fatalf("el turno recuerda la plantilla enseñada: %q", prev.Excluidos)
	}
	r2, tr := preguntar(t, c, "p", "otra forma", nucleotest.RespuestasFijas())
	t.Logf("otra forma: %s %q [%s]", r2.Metodo, r2.Texto, r2.Nivel)
	if r2.Exito && ejcCompactar(r2.Codigo) == ejcCompactar(r1.Codigo) {
		t.Errorf("enseña el mismo programa como si fuera otro:\n%s", titulos(tr))
	}
	if !r2.Exito && r2.Texto != "No conozco otra forma de hacerlo: solo sé hacerlo como te mostré." {
		t.Errorf("sin otra forma lo dice: %q", r2.Texto)
	}
	if !strings.Contains(titulos(tr), "ya te la enseñé") {
		t.Errorf("salta la plantilla ya enseñada:\n%s", titulos(tr))
	}
	if ps := c.Pendientes(); len(ps) != 0 {
		t.Errorf("nada a pendientes: %+v", ps)
	}
}

// G6 (study): a pending question that memory already answers leaves the list, but is not reported as
// something learned now.
func TestEstudiarNoPresumeDeLoQueYaSabia(t *testing.T) {
	c := nuevoCerebroPrueba(t, ejecutorSumaPares())
	c.PonerIndice(&saber.IndiceStd{GoVersion: "prueba"})
	if r, _ := preguntar(t, c, "s", pedidoSumaPares, nucleotest.RespuestasFijas()); !r.Exito || r.Aprendido == "" {
		t.Fatalf("la resuelve y la guarda: %+v", r)
	}
	if err := c.mem.AgregarPendiente(memoria.Pendiente{Texto: pedidoSumaPares, Motivo: "no encontré un programa que haga lo que pides"}); err != nil {
		t.Fatal(err)
	}
	r, tr := estudiarTarea(t, c)
	todo := todoElTexto(r)
	if strings.Contains(todo, "a resolver «") || strings.Contains(todo, "que antes no sabía") {
		t.Errorf("no presume de aprender lo que ya sabía:\n%s\n%s", todo, titulos(tr))
	}
	if !strings.Contains(todo, "1 pregunta de las pendientes ya la sabía resolver") {
		t.Errorf("dice que ya la sabía:\n%s", todo)
	}
	if ps := c.mem.Pendientes(); len(ps) != 0 {
		t.Errorf("sale de pendientes: %+v", ps)
	}
}

// G7: rules taught with «recuerda que todos los X son Y» are listed with the facts (Memoria tab,
// GET /api/memoria/hechos), counted, and can be forgotten by name or through the tab's id.
func TestReglasSeListanYSeOlvidan(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	preguntar(t, c, "s", "recuerda que Toby es un perro", nil)
	if r, _ := preguntar(t, c, "s", "recuerda que todos los perros son mamíferos", nil); !r.Exito || len(c.mem.Reglas()) != 1 {
		t.Fatalf("regla guardada: %+v %v", r, c.mem.Reglas())
	}
	regla := c.mem.Reglas()[0]
	var listada *nucleo.Hecho
	for _, h := range c.Hechos("") {
		if h.ID == regla.ID {
			h := h
			listada = &h
		}
	}
	if listada == nil || !strings.Contains(nucleo.Normalizar(listada.Texto), "perro") || !strings.Contains(nucleo.Normalizar(listada.Texto), "mamifero") {
		t.Fatalf("la regla aparece en Hechos(\"\"): %+v", c.Hechos(""))
	}
	if hs := c.Hechos("mamíferos"); len(hs) == 0 || hs[len(hs)-1].ID != regla.ID {
		t.Errorf("Hechos(q) también busca en las reglas: %+v", hs)
	}
	if e := c.Estado(); e.Hechos != 2 {
		t.Errorf("el estado cuenta el dato y la regla: %d", e.Hechos)
	}
	if r, _ := preguntar(t, c, "s", "¿Toby es mamífero?", nil); !strings.HasPrefix(r.Texto, "Sí") {
		t.Fatalf("con la regla: %q", r.Texto)
	}
	r, tr := preguntar(t, c, "s", "olvida que todos los perros son mamíferos", nucleotest.RespuestasFijas("Sí, olvídalo"))
	if len(c.mem.Reglas()) != 0 || !strings.Contains(r.Texto, "he olvidado la regla") {
		t.Fatalf("olvidar la regla: %q\n%s", r.Texto, titulos(tr))
	}
	if len(c.mem.Todos()) != 1 {
		t.Errorf("el dato de Toby sigue: %+v", c.mem.Todos())
	}
	if r, _ := preguntar(t, c, "s", "¿Toby es mamífero?", nil); strings.HasPrefix(r.Texto, "Sí") {
		t.Errorf("sin la regla ya no lo sabe: %q", r.Texto)
	}
	// the Memoria tab's «Olvidar» sends DELETE /api/memoria/hechos/<id> → Olvidar("hecho", id)
	preguntar(t, c, "s", "recuerda que ningún gato es un perro", nil)
	if len(c.mem.Reglas()) != 1 {
		t.Fatalf("segunda regla: %v", c.mem.Reglas())
	}
	id := c.mem.Reglas()[0].ID
	if err := c.Olvidar("hecho", id); err != nil || len(c.mem.Reglas()) != 0 {
		t.Errorf("Olvidar(hecho, %s): %v %v", id, err, c.mem.Reglas())
	}
}

func TestBuscarReglaPorSuFrase(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	todos := nucleo.Regla{Si: []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: "perro"}}, Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "es_un", Objeto: "mamifero"}}
	ninguno := nucleo.Regla{Si: []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: "gato"}}, Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "es_un", Objeto: "perro", Negado: true}}
	for _, r := range []nucleo.Regla{todos, ninguno} {
		if _, err := c.mem.GuardarRegla(r); err != nil {
			t.Fatal(err)
		}
	}
	casos := map[string]string{
		"todos los perros son mamíferos": "mamifero",
		"los perros son mamiferos":       "mamifero",
		"ningún gato es un perro":        "perro",
		"todos los gatos son perros":     "",
		"todos los perros son animales":  "",
		"Toby es un perro":               "",
	}
	for texto, quiero := range casos {
		r, ok := c.buscarRegla(texto)
		if ok != (quiero != "") || ok && r.Entonces.Objeto != quiero {
			t.Errorf("%q: %v %+v", texto, ok, r.Entonces)
		}
	}
}

// G8: checked web snippets move their site's "dominio:" counter, and the counters reorder the search
// results and the passages (no history keeps saber's order).
func TestCalidadDeFuentes(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	es := []saber.Enlace{{URL: "https://malo.example/a"}, {URL: "https://bueno.example/b"}, {URL: "https://otro.example/c"}}
	if got := c.ordenarEnlaces(es); got[0].URL != es[0].URL || got[1].URL != es[1].URL || got[2].URL != es[2].URL {
		t.Errorf("sin historia se queda el orden: %+v", got)
	}
	for i := 0; i < 6; i++ {
		c.contarDominio("https://malo.example/x", false)
		c.contarDominio("https://bueno.example/y", true)
	}
	if c.stats.Usos("dominio:malo.example") != 6 || c.confianzaDominio("https://malo.example/z") >= 1 || c.confianzaDominio("https://bueno.example/") <= 1 {
		t.Fatalf("contadores: %d %v %v", c.stats.Usos("dominio:malo.example"), c.confianzaDominio("https://malo.example/z"), c.confianzaDominio("https://bueno.example/"))
	}
	if got := c.ordenarEnlaces(es); got[0].URL != es[1].URL || got[2].URL != es[0].URL {
		t.Errorf("la web de código que funcionó va primero y la que falló al final: %+v", got)
	}
	docs := []saber.Documento{
		{URL: "https://malo.example/p", Bloques: []saber.Bloque{{Tipo: "parrafo", Texto: "To reverse a string in Go convert it to a slice of runes and swap them from both ends."}}},
		{URL: "https://bueno.example/p", Bloques: []saber.Bloque{{Tipo: "parrafo", Texto: "To reverse a string in Go you can turn it into runes and swap the runes from both ends."}}},
	}
	if ps := saber.Rankear("reverse a string in Go", docs, 1); len(ps) != 1 || ps[0].URL != "https://malo.example/p" {
		t.Fatalf("sin los contadores ganaría la otra frase: %+v", ps)
	}
	ps := c.rankear("reverse a string in Go", docs, 1)
	if len(ps) != 1 || ps[0].URL != "https://bueno.example/p" {
		t.Errorf("rankear prefiere la fuente fiable: %+v", ps)
	}
	// buscar: a web function that crashes on generated cases counts against its site
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func(in []nucleo.Valor) ([]nucleo.Valor, string) {
			xs, _ := in[0].([]nucleo.Valor)
			if len(xs) == 0 {
				return nil, "runtime error: index out of range [0] with length 0"
			}
			return []nucleo.Valor{xs[0]}, ""
		},
	}}
	pagina := `<html><head><title>First element of a slice in Go</title></head><body>
<p>To get the first element of a slice in Go index it with zero, but check that the slice is not empty first.</p>
<pre>func Primero(xs []int) int {
	return xs[0]
}</pre></body></html>`
	web := webDePrueba(t, map[string]string{"/web/primero": pagina})
	cw := cerebroConWeb(t, web, false)
	cw.ej = ej
	r, tr := preguntar(t, cw, "s", "busca cómo obtener el primer elemento de un slice en Go", nil)
	if cw.stats.Usos("dominio:127.0.0.1") == 0 || cw.confianzaDominio("http://127.0.0.1/") >= 1 {
		t.Errorf("el código que falla cuenta contra su web: usos %d\n%s\n%s", cw.stats.Usos("dominio:127.0.0.1"), r.Texto, titulos(tr))
	}
}

// G9: «háblame de X» says what it knows, with sources; it never shows a sentence of its own making.
func TestHablaNoInventaFrases(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	preguntar(t, c, "s", "lee esto: Una función se puede escribir en él. Hay que crearlo con make. Una función es un bloque de código con nombre que hace algo.", nil)
	r, tr := preguntar(t, c, "s", "háblame de las funciones", nil)
	todo := todoElTexto(r)
	if strings.Contains(todo, "propias palabras") || strings.Contains(todo, "make") || strings.Contains(r.Texto, "Creo que una") {
		t.Errorf("no forma frases con lo leído:\n%s\n%s", todo, titulos(tr))
	}
	if !strings.HasPrefix(r.Texto, "Esto es lo que sé de «funciones»") || !strings.Contains(todo, "Según mis notas de Go") || r.Nivel != nucleo.NivelFuente {
		t.Errorf("dice lo que sabe y de dónde: %q [%s]\n%s", r.Texto, r.Nivel, todo)
	}
}
