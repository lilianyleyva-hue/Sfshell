package lengua

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// senales are the facts about a request that the intent cues look at.
type senales struct {
	p        *nucleo.Pregunta
	n        string // normalized text without code
	calc     string // normalized like n, but "!" (factorial) kept
	crudo    string // text as written, without code
	codigo   bool
	ejemplos bool
	programa bool // program example cases
	pregunta bool
	numeros  int
	palabras int
	marco    *nucleo.Marco
}

func (s *senales) re(r *regexp.Regexp) bool { return r.MatchString(s.n) }

type senal struct {
	tipo   nucleo.TipoIntencion
	peso   float64
	motivo string
	si     func(s *senales) bool
}

func rx(p string) *regexp.Regexp { return regexp.MustCompile(p) }

var (
	reArreglar   = rx(`\b(arregla\w*|corrig\w*|corrije\w*|repara\w*|no compila|no funciona|no va|falla\w*|da mal|deberia|esta mal|bug|panic|index out of range|nil pointer|me sale|me da error|errores?|incorrect\w*|undefined|cannot use|declared and not used|not used)\b`)
	reExplicar   = rx(`\b(explica\w*|que hace|que hacen|entender|entiendo|eficiente|complejidad|como funciona|que significa este|que es este|lo que hace|comenta\w*|ponle comentarios)\b`)
	reProbar     = rx(`\b(prueba\w*|pruebas|test|tests|testea\w*|revisa\w*|tiene errores|tiene algun error|comprueba\w*|verifica\w*|casos? de prueba|y si la lista)\b`)
	reEjecutar   = rx(`\b(ejecuta\w*|corre|correlo|correla|lanza\w*|run|con (la )?entrada)\b`)
	reOptimizar  = rx(`\b(mas rapid[oa]|optimiza\w*|acelera\w*|mejora el rendimiento|mas eficiente)\b`)
	reFuncion    = rx(`\bfuncion(es)? (que|para|de|llamada|con|a la que)\b|\b(una|la|esta) funcion\b`)
	rePedirCrear = rx(`^(haz|hazme|crea|creame|escribe|escribeme|dame|necesito|quiero|programa|genera|implementa)\b`)
	reUsaFuncion = rx(`\busa [A-Za-z]\w* para\b`)
	reProgramaQ  = rx(`\bprograma (que|con|para|de|del|en el que)\b|\b(un|el|este) programa\b|\bfizz ?buzz\b|\bcalculadora\b|\badivina\w* (el|un) numero\b|\btabla de multiplicar\b`)
	reLeePide    = rx(`\b(pida|pidan|lea|lean|pregunte|pregunten|muestre|muestren|por pantalla|en pantalla|por teclado)\b`)
	reResuelve   = rx(`\b(resuelve|resolver|resuelva|despeja|despejar|ecuacion|ecuaciones|sistema de|incognita|halla x|calcula x)\b`)
	reVariable   = rx(`[0-9)][a-z]\b|[=+\-*/^(<>≤≥]\s*[a-z]\b|\b[a-z]\s*(\^|=|<|>|≤|≥|\+|-|\*|/)`)
	reDesigual   = rx(`[<>≤≥]`)
	reCalculoPal = rx(`\b(cuanto (es|da|vale|son)|calcula|calcular|cual es el resultado|resultado de|dime cuanto|opera|simplifica)\b`)
	reOperador   = rx(`\d\s*([+\-*/^×÷·]|\*\*)\s*[\d(]|\d\s*!|√\s*\d|\d\s*(mas|menos|por|entre|elevado a|elevado al|multiplicado por|dividido entre|dividido por)\s+\d|\b(raiz( cuadrada)? de|factorial de|al cuadrado|al cubo)\b`)
	reNumeros    = rx(`\b(primo|primos|factoriza\w*|descomp\w*|factores primos|factorizacion|mcd|mcm|maximo comun divisor|minimo comun multiplo|divisores|divisor|porcentaje|por ciento|multiplos? de)\b|%`)
	rePreguntaN  = rx(`\b(cuant[oa]s?|que distancia|que edad|cuantos años|cual es (el|ese|dicho) numero|que numero es|cuanto tarda|cuanto paga|cuanto cuesta|cuanto le|cuantos le|cuantas le)\b`)
	reUnNumero   = rx(`\b(un|cierto|el|dicho) numero\b.*\b(es|da|resulta|equivale|obtengo|obtiene|sale)\b|\bel (doble|triple|cuadruple) de\b|\bla mitad de un\b|\bentre los dos suman\b|\bsus edades\b`)
	reSiLogica   = rx(`\bsi\b.+(;|\bentonces\b|\.|,)`)
	reLogicaPal  = rx(`\b(se sigue|es valido|siempre verdad|siempre cierto|tautologia|contradiccion|consecuencia logica|modus|implica|equivalente|tabla de verdad)\b`)
	reConectivas = rx(`\b[pqrs] (y|o|implica) (no )?[pqrs]\b|\bno [pqrs]\b`)
	reCaballeros = rx(`\b(caballero|caballeros|bribon|bribones|escudero|escuderos|mentiroso|mentirosos|siempre miente|dice la verdad|dicen la verdad)\b`)
	reCuantif    = rx(`\b(todos|todas|algunos|algunas|algun|ningun|ninguno|ninguna)\b.*\b(son|es)\b`)
	reRecordar   = rx(`^(recuerda|aprende|apunta|memoriza|ten en cuenta|guarda) que\b|\b\w+ significa \w+`)
	reHecho      = rx(`^(es|son|esta|tiene) \w+( \w+)?$|^\w+( \w+)? (es|son|tiene) (un |una )?\w+$`)
	reGrid       = rx(`(?m)^[ \t]*([0-9.\-_*xX][ \t|]*){9}$`)
	reCripto     = rx(`[A-ZÑ]{2,}\s*\+\s*[A-ZÑ]{2,}(\s*\+\s*[A-ZÑ]{2,})*\s*=\s*[A-ZÑ]{2,}`)
	rePuzle      = rx(`\b(reinas|sudoku|cebra|zebra|einstein|colorea\w*|colores|criptoaritm\w*|cuadrado magico)\b`)
	reConteo     = rx(`\b(de cuantas (formas|maneras)|cuantas (formas|maneras)|anagramas?|probabilidad|combinaciones|permutaciones|variaciones|cuantos (numeros )?(hay )?del \d+ al \d+|cuantos .* del \d+ al \d+|elijo \w+ de|escojo \w+ de|mesa redonda|cuantos subconjuntos)\b`)
	reSecuencia  = rx(`(-?\d+\s*,\s*){3,}`)
	reSigue      = rx(`\b(siguiente|sigue|continua|que numero (va|viene|sigue)|serie|sucesion|patron|el proximo)\b`)
	rePlan       = rx(`\b(jarras?|lobo|cabra|hanoi|hanói|torres de|8-puzzle|puzzle de 8|puzle de 8|deslizante|laberinto|misioneros|canibales)\b`)
	reDocPregunta = rx(`^(como|que es|que son|para que sirve|para que sirven|para que se usa|que hace|que hacen|como uso|como se usa|como funciona|como funcionan|explicame|diferencia entre|cuando uso|cuando se usa)\b`)
	reDocConcepto = rx(`\b(defer|goroutines?|gorrutinas?|canal|canales|channels?|slices?|interfaz|interfaces|struct|structs|puntero|punteros|select|range|make|append|panic|recover|mutex|waitgroup|context|go mod|modulos?|paquetes?|package|closures?|metodos?|iota|const|rune|runas?|byte|map|mapas|for range|error|errores|nil|init|main|switch|generics?|genericos|tipos?)\b`)
	reSimboloGo  = rx(`\b[a-z]+\.[A-Z]\w*`)
	reBuscar     = rx(`\b(busca|buscar|buscame|investiga|en internet|en la web|googlea|busques)\b`)
	reComoHago   = rx(`^(como|cual es la forma de|de que forma) (leo|hago|puedo|se hace|se puede|convierto|invierto|ordeno|leer|hacer|escribo|creo|abro|recorro|uso|paso)\b`)
	reMemoria    = rx(`\b(que funciones|olvida|olvidate|funciones aprendidas|funciones guardadas|que has aprendido|que recuerdas|que te he enseñado|tu memoria|que sabes de|que palabras)\b|^que sabes$`)
	reCharla     = rx(`^(hola|buenas|buenos dias|buenas tardes|buenas noches|hey|saludos|gracias|muchas gracias|adios|hasta luego|chao)\b|\b(quien eres|que eres|eres una ia|eres un robot|eres humano|eres inteligente|que sabes hacer|que puedes hacer|en que me ayudas|ayudame|ayuda|como te llamas|como estas)\b`)
	reEstudiar   = rx(`^(estudia|ponte a estudiar|estudiar go|aprende go|aprende mas)\b`)
)

var senalesTabla = []senal{
	// programming with code
	{nucleo.IReparar, 2.5, "veo código y algo que falla", func(s *senales) bool { return s.codigo && s.re(reArreglar) }},
	{nucleo.IReparar, 1.0, "hablas de un error de compilación o ejecución", func(s *senales) bool {
		return !s.codigo && s.re(rx(`\b(no compila|index out of range|nil pointer|panic|undefined|declared and not used)\b`))
	}},
	{nucleo.IExplicar, 2.5, "veo código y me pides que lo explique", func(s *senales) bool { return s.codigo && s.re(reExplicar) }},
	{nucleo.IExplicar, 0.8, "veo código", func(s *senales) bool { return s.codigo }},
	{nucleo.IProbar, 2.2, "veo código y me pides pruebas", func(s *senales) bool { return s.codigo && s.re(reProbar) }},
	{nucleo.IEjecutar, 2.5, "veo código y me pides ejecutarlo", func(s *senales) bool { return s.codigo && s.re(reEjecutar) }},
	{nucleo.IOptimizar, 2.2, "veo código y me pides que sea más rápido", func(s *senales) bool { return s.codigo && s.re(reOptimizar) }},
	// creating
	{nucleo.ICrearFuncion, 2.5, "me pides una función", func(s *senales) bool { return !s.codigo && s.re(reFuncion) && !s.re(reProgramaQ) }},
	{nucleo.ICrearFuncion, 2.2, "me das ejemplos de entrada y salida", func(s *senales) bool { return s.ejemplos && !s.codigo }},
	{nucleo.ICrearFuncion, 0.5, "me pides que cree algo", func(s *senales) bool { return s.re(rePedirCrear) && strings.Contains(s.n, "funcion") }},
	{nucleo.ICrearFuncion, 1.5, "me pides usar una función que conozco", func(s *senales) bool { return reUsaFuncion.MatchString(s.crudo) }},
	{nucleo.ICrearFuncion, 1.2, "describes una tarea de programación", func(s *senales) bool {
		return !s.codigo && s.marco != nil && s.marco.Accion != "" && s.marco.Entrada != "" && !s.marco.Programa &&
			!s.re(reVariable) && !s.re(reNumeros) && !(s.re(rePreguntaN) && s.numeros > 0) && !s.re(reUnNumero) &&
			!esCalculo(s.calc) && !s.re(reConteo)
	}},
	{nucleo.ICrearPrograma, 2.5, "me pides un programa", func(s *senales) bool { return s.re(reProgramaQ) }},
	{nucleo.ICrearPrograma, 1.5, "el programa debe pedir o mostrar datos", func(s *senales) bool { return !s.codigo && s.re(reLeePide) }},
	{nucleo.ICrearPrograma, 2.0, "me das ejemplos de lo que debe escribir", func(s *senales) bool { return s.programa }},
	// maths
	{nucleo.IEcuacion, 2.5, "veo una ecuación con incógnita", func(s *senales) bool {
		return !s.codigo && strings.Contains(s.n, "=") && s.re(reVariable) && !strings.Contains(s.n, "->") && !s.ejemplos
	}},
	{nucleo.IEcuacion, 1.0, "me pides resolver", func(s *senales) bool { return !s.codigo && s.re(reResuelve) }},
	{nucleo.IEcuacion, 2.0, "veo una inecuación", func(s *senales) bool {
		return !s.codigo && s.re(reDesigual) && s.re(reVariable) && !strings.Contains(s.n, "->") && !strings.Contains(s.n, "=>")
	}},
	{nucleo.ICalculo, 2.5, "veo una operación con números", func(s *senales) bool {
		return !s.codigo && esCalculo(s.calc)
	}},
	{nucleo.ICalculo, 1.0, "me pides un cálculo", func(s *senales) bool { return !s.codigo && s.re(reCalculoPal) && s.numeros > 0 }},
	{nucleo.INumeros, 2.5, "me preguntas por primos, divisores o porcentajes", func(s *senales) bool {
		return !s.codigo && s.re(reNumeros) && s.numeros > 0 && !s.ejemplos && !s.re(reFuncion)
	}},
	{nucleo.IProblema, 1.6, "es un problema con números y una pregunta", func(s *senales) bool {
		return !s.codigo && s.numeros > 0 && s.re(rePreguntaN) && s.palabras >= 6 && !s.re(reConteo) && !s.re(reFuncion)
	}},
	{nucleo.IProblema, 2.0, "habla de «un número» que cumple algo", func(s *senales) bool {
		return !s.codigo && s.re(reUnNumero) && !s.re(reFuncion) && !s.re(reProgramaQ)
	}},
	// logic
	{nucleo.ILogica, 2.0, "veo un «si…» y una pregunta", func(s *senales) bool {
		return !s.codigo && s.re(reSiLogica) && s.pregunta && !s.ejemplos && !s.re(reFuncion)
	}},
	{nucleo.ILogica, 2.0, "me preguntas si algo se sigue o es siempre verdad", func(s *senales) bool { return !s.codigo && s.re(reLogicaPal) }},
	{nucleo.ILogica, 1.5, "veo conectivas lógicas", func(s *senales) bool { return !s.codigo && s.re(reConectivas) }},
	{nucleo.ILogica, 2.2, "es un acertijo de caballeros y bribones", func(s *senales) bool { return s.re(reCaballeros) }},
	{nucleo.ISilogismo, 2.2, "veo «todos/algunos/ningún … son»", func(s *senales) bool {
		return !s.codigo && s.re(reCuantif) && !s.re(reFuncion) && !s.re(reProgramaQ) && !s.re(reRecordar)
	}},
	{nucleo.IRecordar, 3.0, "me pides que recuerde algo", func(s *senales) bool { return s.re(reRecordar) && !s.codigo }},
	{nucleo.IPreguntarHecho, 1.6, "me preguntas si algo es de cierta clase", func(s *senales) bool {
		return !s.codigo && s.pregunta && s.numeros == 0 && s.re(reHecho) && !s.re(reConectivas) && !s.re(reCharla)
	}},
	// puzzles, counting, sequences, planning
	{nucleo.IPuzle, 3.0, "veo un tablero de 9×9", func(s *senales) bool { return len(reGrid.FindAllString(s.crudo, -1)) >= 9 }},
	{nucleo.IPuzle, 3.0, "veo una suma de palabras (criptoaritmética)", func(s *senales) bool { return reCripto.MatchString(s.crudo) }},
	{nucleo.IPuzle, 2.5, "es un puzle conocido", func(s *senales) bool { return s.re(rePuzle) && !s.re(reFuncion) }},
	{nucleo.IConteo, 2.5, "me preguntas de cuántas formas o una probabilidad", func(s *senales) bool { return s.re(reConteo) }},
	{nucleo.ISecuencia, 2.5, "veo una lista de números y me pides el siguiente", func(s *senales) bool {
		return s.re(reSecuencia) && (s.re(reSigue) || strings.HasSuffix(strings.TrimSpace(s.crudo), "?") || strings.HasSuffix(strings.TrimSpace(s.n), "..."))
	}},
	{nucleo.IPlan, 2.5, "es un problema de pasos (jarras, río, Hanói…)", func(s *senales) bool { return s.re(rePlan) }},
	// knowledge
	{nucleo.IDocGo, 2.2, "me preguntas cómo funciona algo de Go", func(s *senales) bool {
		return !s.codigo && s.re(reDocPregunta) && (reSimboloGo.MatchString(s.crudo) || s.re(reDocConcepto))
	}},
	{nucleo.IDocGo, 0.6, "mencionas algo de Go", func(s *senales) bool {
		return !s.codigo && (reSimboloGo.MatchString(s.crudo) || s.re(rx(`\ben go\b|\bgolang\b`)))
	}},
	{nucleo.IBuscar, 2.5, "me pides que busque", func(s *senales) bool { return s.re(reBuscar) }},
	{nucleo.IBuscar, 1.0, "me preguntas cómo se hace algo", func(s *senales) bool { return !s.codigo && s.re(reComoHago) }},
	{nucleo.IMemoria, 2.5, "me preguntas por lo que sé o lo que olvido", func(s *senales) bool { return s.re(reMemoria) }},
	{nucleo.ICharla, 2.5, "es un saludo o una pregunta sobre mí", func(s *senales) bool { return s.palabras <= 8 && s.re(reCharla) }},
	{nucleo.IEstudiar, 2.5, "me pides que estudie", func(s *senales) bool { return s.re(reEstudiar) }},
}

// esCalculo: after the leading question words, only numbers, operators and number words remain.
var (
	reInicioCalculo = rx(`^(cuanto (es|da|vale|son)|calcula(me)?|calcular|dime cuanto (es|da)|cual es el resultado de|el resultado de|resultado de|opera|simplifica|evalua)\s+`)
	reSoloCalculo   = rx(`^[\d\s.,+\-*/^()!%√·×÷=]+$`)
	rePalabrasCalc  = rx(`\b(mas|menos|por|entre|elevado|al|cuadrado|cubo|a|la|raiz|cuadrada|de|factorial|multiplicado|dividido|y)\b`)
)

func esCalculo(n string) bool {
	t := reInicioCalculo.ReplaceAllString(strings.TrimSpace(n), "")
	t = strings.TrimSuffix(strings.TrimSpace(t), "=")
	if !reOperador.MatchString(t) {
		return false
	}
	t = rePalabrasCalc.ReplaceAllString(t, " ")
	return reSoloCalculo.MatchString(t)
}

func nuevasSenales(p *nucleo.Pregunta) *senales {
	resto := p.Resto
	if resto == "" && p.Codigo == "" {
		resto = p.Texto
	}
	n := p.Normal
	if n == "" {
		n = nucleo.Normalizar(resto)
	}
	s := &senales{p: p, n: n, crudo: resto, codigo: strings.TrimSpace(p.Codigo) != "",
		ejemplos: len(p.ParesTexto) > 0, programa: len(p.EjProgramas) > 0,
		pregunta: strings.Contains(p.Texto, "?"), marco: p.Marco}
	s.numeros = len(p.Numeros)
	if p.Numeros == nil {
		s.numeros = len(Numeros(resto))
	}
	s.palabras = len(strings.Fields(n))
	s.calc = strings.ReplaceAll(nucleo.Normalizar(strings.ReplaceAll(resto, "!", "‼")), "‼", "!")
	return s
}

// familiaModo maps the UI mode chip to the intent family it boosts.
func familiaModo(modo string, t nucleo.TipoIntencion) bool {
	switch modo {
	case "programar":
		return t == nucleo.ICrearFuncion || t == nucleo.ICrearPrograma
	case "arreglar":
		return t == nucleo.IReparar
	case "explicar":
		return t == nucleo.IExplicar
	case "mates":
		return t == nucleo.ICalculo || t == nucleo.IEcuacion || t == nucleo.INumeros || t == nucleo.IProblema
	case "logica":
		return t == nucleo.ILogica || t == nucleo.ISilogismo || t == nucleo.IPreguntarHecho || t == nucleo.IRecordar
	case "puzles":
		return t == nucleo.IPuzle || t == nucleo.IConteo || t == nucleo.ISecuencia || t == nucleo.IPlan
	case "buscar":
		return t == nucleo.IBuscar || t == nucleo.IDocGo
	}
	return false
}

// Intenciones scores every intent with the weighted cues of senalesTabla, squashed to 0..1
// (1 − e^(−suma)); the UI mode chip doubles its family's sums. Best first; zero scores are dropped.
func Intenciones(p *nucleo.Pregunta) []nucleo.Candidata {
	if p == nil {
		return nil
	}
	s := nuevasSenales(p)
	suma := map[nucleo.TipoIntencion]float64{}
	motivo := map[nucleo.TipoIntencion]string{}
	mejor := map[nucleo.TipoIntencion]float64{}
	var orden []nucleo.TipoIntencion
	for _, c := range senalesTabla {
		if !c.si(s) {
			continue
		}
		if _, ok := suma[c.tipo]; !ok {
			orden = append(orden, c.tipo)
		}
		suma[c.tipo] += c.peso
		if c.peso > mejor[c.tipo] {
			mejor[c.tipo], motivo[c.tipo] = c.peso, c.motivo
		}
	}
	out := make([]nucleo.Candidata, 0, len(orden))
	for _, t := range orden {
		w := suma[t]
		if p.Modo != "" && familiaModo(p.Modo, t) {
			w *= 2
		}
		out = append(out, nucleo.Candidata{Tipo: t, Puntos: 1 - math.Exp(-w), Motivo: motivo[t]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Puntos > out[j].Puntos })
	return out
}
