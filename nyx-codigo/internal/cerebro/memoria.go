package cerebro

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Fixed templates for memory and chat (§4.14.2 "memoria / charla", §9, §13). Nothing here is a claim
// about code or maths, so these answers carry no comprobaciones and Nivel sin_comprobar.

var (
	reSaludo     = regexp.MustCompile(`^(hola|buenas|buenos dias|buenas tardes|buenas noches|hey|saludos|que tal)\b`)
	reGracias    = regexp.MustCompile(`^(gracias|muchas gracias|mil gracias)\b`)
	reAdios      = regexp.MustCompile(`^(adios|hasta luego|hasta pronto|chao|nos vemos|hasta mañana)\b`)
	reQueSabes   = regexp.MustCompile(`\b(que sabes hacer|que puedes hacer|que haces|en que me (puedes )?ayudar|en que me ayudas|para que sirves|ayuda|ayudame)\b`)
	reEresIA     = regexp.MustCompile(`\b(eres una ia|eres un robot|eres humano|eres una persona|eres inteligente|quien eres|que eres|eres chatgpt|eres un modelo|eres una inteligencia artificial|eres un llm)\b`)
	reComoApr    = regexp.MustCompile(`\b(como aprendes|como has aprendido|como recuerdas|como funciona tu memoria|que aprendes|aprendes solo|puedes aprender)\b`)
	reNoSabes    = regexp.MustCompile(`\b(que no (sabes|puedes|se) hacer|que no sabes|tus limites|tus limitaciones|que limitaciones tienes|en que fallas)\b`)
	reNombre     = regexp.MustCompile(`\bcomo te llamas\b`)
	reComoEstas  = regexp.MustCompile(`\bcomo estas\b`)
	reFunciones  = regexp.MustCompile(`\b(que funciones|funciones (sabes|conoces|aprendidas|guardadas|tienes|has aprendido)|lista de funciones|tus funciones)\b`)
	rePalabrasQ  = regexp.MustCompile(`\b(que palabras|palabras (aprendidas|sabes|nuevas|has aprendido)|tu vocabulario)\b`)
	reOlvida     = regexp.MustCompile(`^(?:olvida|olvidate de|borra de tu memoria|borra|elimina de tu memoria)\s+(.+)$`)
	reQueSabesDe = regexp.MustCompile(`\bque sabes (de|sobre)\s+(.+)$`)
	rePendientes = regexp.MustCompile(`\b(que no has (resuelto|podido)|preguntas pendientes|tus pendientes)\b`)
)

// charlaExtra recognizes the chat questions that lengua does not route ("¿cómo aprendes?",
// "¿qué no sabes hacer?"); "" when none.
func charlaExtra(normal string) string {
	switch {
	case reComoApr.MatchString(normal):
		return "aprender"
	case reNoSabes.MatchString(normal):
		return "limites"
	}
	return ""
}

func plantilla(texto string, parrafos []string, sugerencias []string, metodo string) nucleo.Respuesta {
	return nucleo.Respuesta{
		Texto:          texto,
		Parrafos:       parrafos,
		Sugerencias:    sugerencias,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         metodo,
		Exito:          true,
	}
}

var ejemplosCapacidades = []string{
	"haz una función que sume los números pares de una lista",
	"resuelve 2x+3=11",
	"¿de cuántas formas elijo 3 de 10?",
	"SEND+MORE=MONEY",
	"¿para qué sirve defer?",
}

// plantillaCharla answers greetings and questions about the app.
func (c *Cerebro) plantillaCharla(k *tarea) nucleo.Respuesta {
	n := k.q.Normal
	if n == "" {
		n = nucleo.Normalizar(k.p.Texto)
	}
	switch {
	case charlaExtra(n) == "aprender":
		return c.comoAprendo()
	case charlaExtra(n) == "limites":
		return queNoSeHacer()
	case reEresIA.MatchString(n):
		return plantilla("Soy un programa de razonamiento, no un modelo de lenguaje.",
			[]string{
				"No invento respuestas: escribo, arreglo y pruebo código Go y resuelvo cuentas, lógica y puzles con mis propios algoritmos, y compruebo cada resultado antes de enseñártelo.",
				"Cada respuesta dice cómo la comprobé. Si algo no lo pude comprobar, te lo digo.",
				"Entiendo el español con patrones y un vocabulario, no como una persona.",
			}, []string{"¿Qué sabes hacer?", "¿Cómo aprendes?"}, "charla")
	case reQueSabes.MatchString(n):
		return c.queSeHacer()
	case reNombre.MatchString(n):
		return plantilla("Me llamo Nyx Código.", []string{"Soy un programa que escribe, arregla y prueba funciones de Go, y que resuelve cuentas, lógica y puzles."}, ejemplosCapacidades[:3], "charla")
	case reComoEstas.MatchString(n):
		return plantilla("Bien, gracias. ¿En qué te ayudo?", nil, ejemplosCapacidades[:3], "charla")
	case reGracias.MatchString(n):
		return plantilla("De nada. Aquí estoy si necesitas algo más.", nil, nil, "charla")
	case reAdios.MatchString(n):
		return plantilla("¡Hasta luego! Lo que he aprendido queda guardado para la próxima vez.", nil, nil, "charla")
	case reSaludo.MatchString(n):
		return plantilla("¡Hola! Pídeme una función, pega código para arreglar o hazme una pregunta.", nil, ejemplosCapacidades[:4], "charla")
	}
	return plantilla("¡Hola! Soy Nyx Código. Puedo escribir y arreglar funciones de Go, explicar código y resolver cuentas, lógica y puzles.",
		[]string{"Mi conversación es sencilla: respondo mejor a peticiones concretas."}, ejemplosCapacidades[:3], "charla")
}

// queSeHacer is the capability list with clickable examples.
func (c *Cerebro) queSeHacer() nucleo.Respuesta {
	nf, nh, np := c.mem.Contar()
	return plantilla("Sé escribir funciones y programas de Go, arreglarlos, explicarlos y probarlos, y resolver cuentas, ecuaciones, lógica y puzles.",
		[]string{
			"Programar: «haz una función que sume los números pares de una lista», ejemplos como «[1,2,3,4] -> 6; [] -> 0», o «haz un programa que pida dos números y los sume».",
			"Código que ya tienes: pégalo y dime «no compila», «explícame este código», «hazle pruebas» o «ejecuta esto con entrada 5».",
			"Matemáticas: «cuánto es 3/4+1/6», «resuelve 2x+3=11», «factoriza 600», «el doble de un número más 3 es 11».",
			"Lógica y puzles: «si llueve el suelo se moja; llueve. ¿se moja?», un sudoku, «8 reinas», «SEND+MORE=MONEY», «jarras de 3 y 5 litros, ¿cómo saco 4?».",
			"Go: «¿para qué sirve defer?», «¿qué hace strings.Fields?».",
			fmt.Sprintf("Mi memoria: sé %d funciones, %d datos y %d palabras nuevas. Pregúntame «¿qué funciones sabes?» o enséñame con «recuerda que …».", nf, nh, np),
		}, ejemplosCapacidades, "charla")
}

// comoAprendo explains §9: memorization, statistics and composition, not generalization.
func (c *Cerebro) comoAprendo() nucleo.Respuesta {
	nf, nh, np := c.mem.Contar()
	return plantilla("Aprendo memorizando, contando y combinando; no entiendo como una persona.",
		[]string{
			"Funciones: cuando una función pasa sus pruebas, la guardo y la reutilizo («Ya sé hacer esto»). También la uso como pieza para construir otras.",
			"Palabras: si me dices «capicúa significa palíndromo», la aprendo; y una palabra nueva que aparece en dos peticiones que salen bien queda confirmada.",
			"Datos y reglas: lo que me enseñas con «recuerda que …» lo uso para razonar. Los verbos de los problemas («¿trocar suma o resta?») también los recuerdo.",
			"Errores: cuando me dices «está mal: con [2] debe dar 0», guardo ese caso y lo compruebo siempre a partir de entonces.",
			"Estadísticas: cuento qué reglas de arreglo, qué piezas y qué formas de entender salen bien, y pruebo antes las que mejor funcionan. Las preguntas que no supe resolver las reintento cuando estudio.",
			fmt.Sprintf("Ahora mismo sé %d funciones, %d datos y %d palabras nuevas. No generalizo: aprendo más rápido lo que se parece a lo que ya he visto.", nf, nh, np),
		}, []string{"¿Qué funciones sabes?", "¿Qué no sabes hacer?"}, "charla")
}

// queNoSeHacer states the known limits of §13 in simple words.
func queNoSeHacer() nucleo.Respuesta {
	return plantilla("Estas son las cosas que no sé hacer bien:",
		[]string{
			"Entiendo el español con patrones y un vocabulario: las frases largas, raras o con faltas me confunden. Si no te entiendo, te lo digo y te propongo cómo escribirlo.",
			"Construyo programas pequeños (unas diez piezas). Para cosas grandes dependo de mis recetas, de mi memoria y de la web, y a veces no lo consigo.",
			"Las pruebas no son demostraciones: cada sello dice exactamente qué comprobé. Solo arreglo fallos que están a uno o dos cambios de distancia.",
			"La web es frágil y casi todo está en inglés; el código de internet solo lo ejecuto si es seguro, y si no lo marco como «sin comprobar».",
			"En matemáticas no hago geometría ni cálculo (derivadas, integrales); en problemas con palabras solo conozco algunos tipos.",
			"No converso de cualquier tema: a la charla respondo con frases fijas.",
		}, []string{"¿Qué sabes hacer?", "¿Cómo aprendes?"}, "charla")
}

// plantillaMemoria answers "¿qué funciones sabes?", "olvida X", "¿qué palabras sabes?", "¿qué sabes de X?".
func (c *Cerebro) plantillaMemoria(k *tarea) nucleo.Respuesta {
	n := k.q.Normal
	if n == "" {
		n = nucleo.Normalizar(k.p.Texto)
	}
	n = strings.Trim(n, " .")
	switch {
	case reOlvida.MatchString(n):
		return c.olvidarConfirmando(k, strings.TrimSpace(reOlvida.FindStringSubmatch(n)[1]))
	case reFunciones.MatchString(n):
		return c.tablaFunciones()
	case rePalabrasQ.MatchString(n):
		return c.tablaPalabras()
	case rePendientes.MatchString(n):
		return c.listaPendientes()
	case reQueSabesDe.MatchString(n):
		return c.queSeDe(strings.TrimSpace(reQueSabesDe.FindStringSubmatch(n)[2]))
	}
	nf, nh, np := c.mem.Contar()
	return plantilla(fmt.Sprintf("Sé %d funciones, %d datos y %d palabras nuevas.", nf, nh, np),
		[]string{"Pregúntame «¿qué funciones sabes?», «¿qué palabras sabes?» o «¿qué sabes de Toby?». Para borrar algo, dime «olvida …»."},
		[]string{"¿Qué funciones sabes?", "¿Qué palabras sabes?"}, "memoria")
}

func (c *Cerebro) tablaFunciones() nucleo.Respuesta {
	fs := c.mem.Funciones()
	if len(fs) == 0 {
		return plantilla("Todavía no he aprendido ninguna función.",
			[]string{"Pídeme una («haz una función que …») y, si pasa sus pruebas, la guardaré."},
			[]string{"haz una función que sume los números pares de una lista"}, "memoria")
	}
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].Usos > fs[j].Usos })
	tb := nucleo.Tabla{Titulo: "Funciones que sé", Cabecera: []string{"Nombre", "Qué hace", "Firma", "Comprobada", "Usos"}}
	for _, f := range fs {
		tb.Filas = append(tb.Filas, []string{f.Nombre, f.Descripcion, f.Firma.Go(), etiquetaNivel(f.Nivel), strconv.Itoa(f.Usos)})
	}
	texto := fmt.Sprintf("Sé %d funciones.", len(fs))
	if len(fs) == 1 {
		texto = "Sé 1 función."
	}
	r := plantilla(texto, []string{"Puedes usarlas con «usa " + fs[0].Nombre + " para …» o borrarlas con «olvida " + fs[0].Nombre + "»."}, nil, "memoria")
	r.Tablas = []nucleo.Tabla{tb}
	return r
}

func (c *Cerebro) tablaPalabras() nucleo.Respuesta {
	ps := c.mem.Palabras()
	if len(ps) == 0 {
		return plantilla("Todavía no he aprendido palabras nuevas.",
			[]string{"Enséñame una con «recuerda que capicúa significa palíndromo»."}, nil, "memoria")
	}
	var claves []string
	for p := range ps {
		claves = append(claves, p)
	}
	sort.Strings(claves)
	tb := nucleo.Tabla{Titulo: "Palabras que he aprendido", Cabecera: []string{"Palabra", "Quiere decir"}}
	for _, p := range claves {
		tb.Filas = append(tb.Filas, []string{p, strings.ReplaceAll(ps[p], "_", " ")})
	}
	r := plantilla(fmt.Sprintf("He aprendido %d palabras.", len(ps)), nil, nil, "memoria")
	r.Tablas = []nucleo.Tabla{tb}
	return r
}

func (c *Cerebro) listaPendientes() nucleo.Respuesta {
	ps := c.mem.Pendientes()
	if len(ps) == 0 {
		return plantilla("No tengo preguntas pendientes.", nil, nil, "memoria")
	}
	tb := nucleo.Tabla{Titulo: "Preguntas que no supe resolver", Cabecera: []string{"Pregunta", "Motivo", "Intentos"}}
	for _, p := range ps {
		tb.Filas = append(tb.Filas, []string{recortar(p.Texto, 80), p.Motivo, strconv.Itoa(p.Intentos)})
	}
	r := plantilla(fmt.Sprintf("Tengo %d preguntas pendientes.", len(ps)), []string{"Las reintento cuando estudio (botón «Estudiar Go»)."}, nil, "memoria")
	r.Tablas = []nucleo.Tabla{tb}
	return r
}

func (c *Cerebro) queSeDe(tema string) nucleo.Respuesta {
	hs := c.mem.BuscarHechos(tema, 12)
	var fs []string
	for _, f := range c.Funciones(tema) {
		fs = append(fs, f.Nombre)
	}
	if len(hs) == 0 && len(fs) == 0 {
		return plantilla(fmt.Sprintf("No sé nada de «%s».", tema), []string{"Puedes enseñármelo con «recuerda que …»."}, nil, "memoria")
	}
	var ps []string
	for _, h := range hs {
		t := h.Texto
		if t == "" {
			t = h.Sujeto + " " + strings.ReplaceAll(h.Relacion, "_", " ") + " " + h.Objeto
		}
		ps = append(ps, t+" ("+fuenteHecho(h)+")")
	}
	if len(fs) > 0 {
		ps = append(ps, "Funciones relacionadas: "+strings.Join(fs, ", ")+".")
	}
	return plantilla(fmt.Sprintf("Esto es lo que sé de «%s»:", tema), ps, nil, "memoria")
}

func fuenteHecho(h nucleo.Hecho) string {
	if h.Fuente == "" || h.Fuente == "usuario" {
		return "me lo dijiste tú"
	}
	return "según " + h.Fuente
}

// olvidarConfirmando finds what "olvida X" names (a function, a learned word or a fact), asks for
// confirmation with buttons and then forgets it.
func (c *Cerebro) olvidarConfirmando(k *tarea, que string) nucleo.Respuesta {
	que = strings.TrimSpace(strings.Trim(que, "\"'«» ."))
	for _, pre := range []string{"la funcion ", "funcion ", "la palabra ", "palabra ", "el dato ", "el hecho "} {
		que = strings.TrimPrefix(que, pre)
	}
	if que == "" {
		return plantilla("¿Qué quieres que olvide?", []string{"Dime «olvida SumaPares», «olvida la palabra capicúa» u «olvida que Toby es un perro»."}, nil, "memoria")
	}
	tipo, id, descripcion := "", "", ""
	for _, f := range c.mem.Funciones() {
		if nucleo.Normalizar(f.Nombre) == que {
			tipo, id, descripcion = "funcion", f.Nombre, "la función "+f.Nombre
			break
		}
	}
	if tipo == "" {
		if _, ok := c.mem.Palabras()[que]; ok {
			tipo, id, descripcion = "palabra", que, "la palabra «"+que+"»"
		}
	}
	if tipo == "" {
		dato := strings.TrimPrefix(que, "que ")
		if hs := c.mem.BuscarHechos(dato, 1); len(hs) > 0 {
			t := hs[0].Texto
			if t == "" {
				t = hs[0].Sujeto + " " + strings.ReplaceAll(hs[0].Relacion, "_", " ") + " " + hs[0].Objeto
			}
			tipo, id, descripcion = "hecho", hs[0].ID, "el dato «"+t+"»"
		}
	}
	if tipo == "" {
		return plantilla(fmt.Sprintf("No encuentro «%s» en mi memoria.", que),
			[]string{"Mira la pestaña Memoria o pregúntame «¿qué funciones sabes?»."}, nil, "memoria")
	}
	resp, ok := c.preguntar(k, fmt.Sprintf("¿Olvido %s? No se puede deshacer.", descripcion), []string{"Sí, olvídalo", "No"})
	if !ok {
		return plantilla("No he olvidado nada: necesito que me lo confirmes.", nil, nil, "memoria")
	}
	if !strings.HasPrefix(nucleo.Normalizar(resp), "si") {
		return plantilla("Vale, no olvido nada.", nil, nil, "memoria")
	}
	if err := c.Olvidar(tipo, id); err != nil {
		r := plantilla("No he podido olvidarlo: "+mensajeError(err)+".", nil, nil, "memoria")
		r.Exito = false
		return r
	}
	k.raiz.Sub(nucleo.PasoMemoria, "Olvido %s", descripcion).Bien("")
	return plantilla(fmt.Sprintf("Hecho: he olvidado %s.", descripcion), nil, nil, "memoria")
}

// etiquetaNivel is a short Spanish name of a level (tables).
func etiquetaNivel(n nucleo.Nivel) string {
	switch n {
	case nucleo.NivelTusEjemplos:
		return "con tus ejemplos"
	case nucleo.NivelEntendido:
		return "según lo que entendí"
	case nucleo.NivelPropiedades:
		return "propiedades"
	case nucleo.NivelSinFallos:
		return "sin errores"
	case nucleo.NivelCalculo:
		return "por cálculo"
	case nucleo.NivelFuente:
		return "según una fuente"
	case nucleo.NivelFallo:
		return "falla"
	}
	return "sin comprobar"
}
