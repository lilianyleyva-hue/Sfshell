package lengua

import (
	"math/big"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// vista is the token view the frame reader works on.
type vista struct {
	toks  []nucleo.Token
	norma []string
	lema  []string
	usado []bool
	lex   *Lexico
}

func nuevaVista(tokens []nucleo.Token, l *Lexico) *vista {
	if l == nil {
		l = LexicoBase()
	}
	v := &vista{toks: tokens, lex: l, usado: make([]bool, len(tokens))}
	for _, t := range tokens {
		le := t.Lema
		if le == "" {
			if t.Clase == "palabra" {
				le = Lema(t.Texto)
			} else {
				le = t.Norma
			}
		}
		v.norma = append(v.norma, t.Norma)
		v.lema = append(v.lema, le)
	}
	return v
}

func (v *vista) n(i int) string {
	if i < 0 || i >= len(v.norma) {
		return ""
	}
	return v.norma[i]
}

func (v *vista) l(i int) string {
	if i < 0 || i >= len(v.lema) {
		return ""
	}
	return v.lema[i]
}

func (v *vista) concepto(i int) string {
	if i < 0 || i >= len(v.toks) || v.toks[i].Clase != "palabra" {
		return ""
	}
	if c, ok := v.lex.Concepto(v.lema[i]); ok {
		return c
	}
	if c, ok := v.lex.Concepto(v.norma[i]); ok {
		return c
	}
	return ""
}

// es reports whether the norma at i is one of the "|"-separated alternatives.
func (v *vista) es(i int, alts string) bool {
	w := v.n(i)
	if w == "" {
		return false
	}
	for _, a := range strings.Split(alts, "|") {
		if a == w {
			return true
		}
	}
	return false
}

// prefijo reports whether the norma at i starts with one of the "|"-separated prefixes.
func (v *vista) prefijo(i int, alts string) bool {
	w := v.n(i)
	if w == "" {
		return false
	}
	for _, a := range strings.Split(alts, "|") {
		if strings.HasPrefix(w, a) {
			return true
		}
	}
	return false
}

// frase finds the sequence of alternatives starting at or after desde; it returns the start index or -1.
func (v *vista) frase(desde int, partes ...string) int {
	for i := max(desde, 0); i+len(partes) <= len(v.norma); i++ {
		ok := true
		for k, p := range partes {
			if !v.es(i+k, p) {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// numero returns the number at token i (digits, merged words, or a single number word).
func (v *vista) numero(i int) (*big.Rat, bool) {
	if i < 0 || i >= len(v.toks) {
		return nil, false
	}
	t := v.toks[i]
	if t.Clase == "numero" && t.Num != nil {
		r := new(big.Rat).Set(t.Num)
		if i > 0 && esMenos(v.toks[i-1]) && v.toks[i-1].Hasta == t.Desde {
			r.Neg(r)
		}
		return r, true
	}
	if t.Clase == "palabra" {
		if r, k := numeroEnPalabras(v.toks, i); k > 0 {
			return r, true
		}
		if t.Norma == "n" || t.Norma == "x" {
			return nil, false
		}
	}
	return nil, false
}

// numeroTras looks for a number in the next few tokens after i (skipping articles such as "el").
func (v *vista) numeroTras(i int) (*big.Rat, int, bool) {
	for k := i + 1; k <= i+3 && k < len(v.toks); k++ {
		if r, ok := v.numero(k); ok {
			return r, k, true
		}
		if !v.es(k, "el|la|a|de|del|los|las|-") {
			break
		}
	}
	return nil, -1, false
}

// textoTras returns the quoted text, single letter or word that follows i ("por «a»", "con la letra b").
func (v *vista) textoTras(i int) (string, int, bool) {
	for k := i + 1; k <= i+4 && k < len(v.toks); k++ {
		t := v.toks[k]
		if t.Clase == "cita" {
			return t.Texto, k, true
		}
		if v.es(k, "la|el|letra|palabra|caracter|texto|cadena|un|una|los|las") {
			continue
		}
		if t.Clase == "palabra" || t.Clase == "numero" {
			return t.Texto, k, true
		}
		break
	}
	return "", -1, false
}

func ratFloat(r *big.Rat) float64 {
	f, _ := r.Float64()
	return f
}

var accionesMarco = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range nucleo.Conceptos {
		if c == "par" {
			break
		}
		m[c] = true
	}
	return m
}()

// accionesDebiles are actions that give way to any other action found in the request.
var accionesDebiles = map[string]bool{
	"buscar": true, "comprobar": true, "transformar": true, "tomar": true, "eliminar": true, "contiene": true,
	"agregar": true, "longitud": true, "primero": true, "ultimo": true, "comparar": true,
}

// verbosGenericos carry no action of their own ("devuelve el máximo" is maximo).
var verbosGenericos = conjunto(`devolver dar calcular obtener sacar hallar hacer crear escribir mostrar imprimir decir
recibir pedir leer preguntar necesitar querer poder usar programar generar construir retornar indicar determinar`)

// Marco reads the frame of a programming request: action, object, element, modifiers, output kind,
// number agreement and program inputs (§4.8, algorithm 3). It returns nil when the text has none.
func Marco(tokens []nucleo.Token, l *Lexico) *nucleo.Marco {
	v := nuevaVista(tokens, l)
	m := &nucleo.Marco{}
	v.programa(m)
	v.modificadores(m)
	v.accion(m)
	v.objeto(m)
	v.ajustarAccion(m)
	v.salida(m)
	if m.Accion == "" && m.Objeto == "" && m.Elemento == "" && len(m.Mods) == 0 && !m.Programa && m.Entrada == "" {
		return nil
	}
	return m
}

func (v *vista) programa(m *nucleo.Marco) {
	for i := range v.norma {
		switch {
		case v.es(i, "programa") && (v.es(i-1, "un|el|este|mi|tu|ese") || v.es(i+1, "que|con|para|de|del|en")) && !v.es(i+2, "funcion"):
			m.Programa = true
		case v.es(i, "pida|pidan|pregunte|pregunten|lea|lean|muestre|muestren|pide|pregunta"):
			if v.es(i, "pide|pregunta") && !v.es(i-1, "que") {
				continue
			}
			m.Programa = true
		case v.es(i, "pantalla|consola|teclado") && v.es(i-1, "en|por|la|el|del"):
			m.Programa = true
		}
		if v.es(i, "pida|pidan|pide|pregunte|pregunten|pregunta|lea|lean") {
			m.Lee = append(m.Lee, v.leeTras(i)...)
		}
		if v.es(i, "muestre|muestren|imprima|imprime|escriba|diga") && m.Muestra == "" && m.Programa {
			var ps []string
			for k := i + 1; k < len(v.norma) && k <= i+6; k++ {
				if v.toks[k].Clase == "simbolo" || v.es(k, "y|e|si|cuando") {
					break
				}
				ps = append(ps, v.norma[k])
			}
			m.Muestra = strings.Join(ps, " ")
		}
	}
}

// leeTras reads "dos números", "un número", "una lista de números", "su nombre" after a reading verb.
func (v *vista) leeTras(i int) []string {
	cuenta := 0
	for k := i + 1; k < len(v.norma) && k <= i+5; k++ {
		if r, ok := v.numero(k); ok && r.IsInt() && r.Sign() > 0 && r.Num().Int64() <= 10 {
			cuenta = int(r.Num().Int64())
			continue
		}
		if v.es(k, "un|una|uno") {
			cuenta = 1
			continue
		}
		if v.es(k, "el|la|los|las|su|sus|tu|tus|mi|mis|al|a|usuario|usuaria|de|del|varios|varias|algunos|algunas") {
			continue
		}
		var tipo string
		switch v.l(k) {
		case "numero", "entero", "edad", "año", "nota", "cantidad", "precio":
			tipo = "numero"
		case "decimal", "real":
			tipo = "decimal"
		case "palabra", "nombre", "texto", "frase", "cadena", "linea", "apellido", "mensaje":
			tipo = "texto"
		case "lista", "arreglo", "vector", "serie":
			tipo = "lista_numeros"
			if v.es(k+1, "de") && (v.prefijo(k+2, "palabra|nombre|texto|cadena")) {
				tipo = "lista_palabras"
			}
			cuenta = 1
		default:
			return nil
		}
		plural := strings.HasSuffix(v.n(k), "s") && tipo != "lista_numeros" && tipo != "lista_palabras"
		if cuenta == 0 {
			if plural {
				if tipo == "texto" {
					return []string{"lista_palabras"}
				}
				return []string{"lista_numeros"}
			}
			cuenta = 1
		}
		out := make([]string, cuenta)
		for j := range out {
			out[j] = tipo
		}
		return out
	}
	return nil
}

func (v *vista) agregarMod(m *nucleo.Marco, md nucleo.Modificador) {
	for _, x := range m.Mods {
		if x.Concepto == md.Concepto && x.Texto == md.Texto && len(x.Numeros) == len(md.Numeros) {
			return
		}
	}
	m.Mods = append(m.Mods, md)
}

func (v *vista) negado(i int) bool {
	return v.es(i-1, "no") || v.es(i-1, "sean|son|sea|es|esten") && v.es(i-2, "no")
}

func (v *vista) modificadores(m *nucleo.Marco) {
	for i := 0; i < len(v.norma); i++ {
		if v.usado[i] {
			continue
		}
		num := func(concepto string, desde int, ini int) bool {
			r, k, ok := v.numeroTras(desde)
			if !ok {
				return false
			}
			md := nucleo.Modificador{Concepto: concepto, Numeros: []float64{ratFloat(r)}, Negado: v.negado(ini)}
			v.agregarMod(m, md)
			for j := ini; j <= k; j++ {
				v.usado[j] = true
			}
			return true
		}
		switch {
		case v.es(i, "mayor|mayores|superior|superiores") && v.es(i+1, "que|a"):
			if num("mayor_que", i+1, i) {
				continue
			}
		case v.es(i, "mas") && v.es(i+1, "grande|grandes|alto|altos") && v.es(i+2, "que"):
			if num("mayor_que", i+2, i) {
				continue
			}
		case v.es(i, "menor|menores|inferior|inferiores") && v.es(i+1, "que|a"):
			if num("menor_que", i+1, i) {
				continue
			}
		case v.es(i, "mas") && v.es(i+1, "pequeño|pequeños|pequeña|pequeñas|bajo|bajos") && v.es(i+2, "que"):
			if num("menor_que", i+2, i) {
				continue
			}
		case v.es(i, "igual|iguales") && v.es(i+1, "a|que"):
			if num("igual_a", i+1, i) {
				continue
			}
			if t, k, ok := v.textoTras(i + 1); ok {
				v.agregarMod(m, nucleo.Modificador{Concepto: "igual_a", Texto: t, Negado: v.negado(i)})
				marcar(v.usado, i, k)
				continue
			}
		case v.es(i, "distinto|distintos|distinta|distintas|diferente|diferentes") && v.es(i+1, "de|a|que"):
			if num("distinto_de", i+1, i) {
				continue
			}
		case v.es(i, "divisible|divisibles") && v.es(i+1, "por|entre"):
			if num("divisible_por", i+1, i) {
				continue
			}
		case v.es(i, "multiplo|multiplos") && v.es(i+1, "de"):
			if num("divisible_por", i+1, i) {
				continue
			}
		case v.prefijo(i, "empiec|empiez|comienc|comienz|empieza|comienza|inici") && v.es(i+1, "por|con|en"):
			if t, k, ok := v.textoTras(i + 1); ok {
				v.agregarMod(m, nucleo.Modificador{Concepto: "empieza_con", Texto: t, Negado: v.negado(i) || v.es(i-2, "no")})
				marcar(v.usado, i, k)
				continue
			}
		case v.prefijo(i, "termin|acab") && v.es(i+1, "por|con|en"):
			if t, k, ok := v.textoTras(i + 1); ok {
				v.agregarMod(m, nucleo.Modificador{Concepto: "termina_con", Texto: t, Negado: v.negado(i) || v.es(i-2, "no")})
				marcar(v.usado, i, k)
				continue
			}
		case v.es(i, "mas|menos") && v.es(i+1, "de") && !v.usado[i]:
			if r, k, ok := v.numeroTras(i + 1); ok && v.es(k+1, "letras|letra|caracteres|caracter|digitos|cifras|elementos") {
				c := "longitud_mayor"
				if v.es(i, "menos") {
					c = "longitud_menor"
				}
				v.agregarMod(m, nucleo.Modificador{Concepto: c, Numeros: []float64{ratFloat(r)}})
				marcar(v.usado, i, k+1)
				if i > 0 && v.es(i-1, "con|de|que tengan") {
					v.usado[i-1] = true
				}
				continue
			}
		case v.es(i, "sin") && v.es(i+1, "repetir|repetidos|repetidas|repeticiones|duplicados|duplicadas"):
			v.agregarMod(m, nucleo.Modificador{Concepto: "sin_repetir"})
			marcar(v.usado, i, i+1)
			continue
		case v.es(i, "no") && v.es(i+1, "repetidos|repetidas|duplicados"):
			v.agregarMod(m, nucleo.Modificador{Concepto: "sin_repetir"})
			marcar(v.usado, i, i+1)
			continue
		}
		if v.toks[i].Clase != "palabra" {
			continue
		}
		switch c := v.concepto(i); c {
		case "par", "impar", "positivo", "negativo", "primo":
			if v.es(i, "par") && v.es(i-1, "un") && !v.es(i+1, "o") { // "un par de" = a couple
				continue
			}
			v.agregarMod(m, nucleo.Modificador{Concepto: c, Negado: v.negado(i)})
			v.usado[i] = true
		}
	}
}

func marcar(u []bool, a, b int) {
	for j := a; j <= b && j < len(u); j++ {
		if j >= 0 {
			u[j] = true
		}
	}
}

func (v *vista) quitarMod(m *nucleo.Marco, c string) {
	out := m.Mods[:0]
	for _, x := range m.Mods {
		if x.Concepto != c {
			out = append(out, x)
		}
	}
	m.Mods = out
}

func (v *vista) tieneMod(m *nucleo.Marco, c string) bool {
	for _, x := range m.Mods {
		if x.Concepto == c {
			return true
		}
	}
	return false
}

func (v *vista) accion(m *nucleo.Marco) {
	fija := func(c string, desde, hasta int) {
		if m.Accion == "" {
			m.Accion = c
			marcar(v.usado, desde, hasta)
		}
	}
	if i := v.frase(0, "maximo|mayor", "comun", "divisor"); i >= 0 {
		fija("mcd", i, i+2)
	}
	if i := v.frase(0, "minimo|menor", "comun", "multiplo"); i >= 0 {
		fija("mcm", i, i+2)
	}
	if i := v.frase(0, "primera|primeras", "letra|letras"); i >= 0 && v.frase(0, "mayuscula|mayusculas") >= 0 {
		fija("titulo", i, i+1)
	}
	if i := v.frase(0, "esta|este|estan|es|sea|son|sean|estuviera", "ordenada|ordenado|ordenadas|ordenados"); i >= 0 {
		fija("comprobar", i, i+1)
	}
	for i := range v.norma {
		switch {
		case v.es(i, "mcd"):
			fija("mcd", i, i)
		case v.es(i, "mcm"):
			fija("mcm", i, i)
		case v.es(i, "mas") && v.prefijo(i+1, "larg"):
			fija("mas_largo", i, i+1)
		case v.es(i, "mas") && v.prefijo(i+1, "cort"):
			fija("mas_corto", i, i+1)
		case v.es(i, "mas") && v.es(i+1, "grande|grandes|alto|alta|altos|altas") && !v.es(i+2, "que"):
			fija("maximo", i, i+1)
		case v.es(i, "mas") && v.es(i+1, "pequeño|pequeña|pequeños|pequeñas|bajo|baja|bajos|bajas|chico|chica") && !v.es(i+2, "que"):
			fija("minimo", i, i+1)
		case v.es(i, "cuantas|cuantos") && v.es(i+1, "veces"):
			if v.frase(i, "cada") >= 0 {
				fija("frecuencia", i, i+1)
			} else {
				fija("contar", i, i+1)
			}
		case v.es(i, "primera|primeras") && v.es(i+1, "letra|letras") && v.frase(i, "mayuscula|mayusculas") >= 0:
			fija("titulo", i, i+1)
		case v.es(i, "cada") && v.es(i+1, "palabra") && v.frase(i, "mayuscula|mayusculas") >= 0:
			fija("titulo", i, i+1)
		case v.es(i, "a|en") && v.es(i+1, "mayusculas|mayuscula"):
			fija("mayusculas", i, i+1)
		case v.es(i, "a|en") && v.es(i+1, "minusculas|minuscula"):
			fija("minusculas", i, i+1)
		case v.prefijo(i, "quit|elimin|borr|sac") && v.es(i+1, "los|el|todos") && v.es(i+2, "espacios|espacio"):
			fija("quitar_espacios", i, i+2)
		case v.es(i, "de") && v.es(i+1, "mayor") && v.es(i+2, "a") && v.es(i+3, "menor"):
			m.Accion = "ordenar_desc"
			marcar(v.usado, i, i+3)
		case v.es(i, "de") && v.es(i+1, "menor") && v.es(i+2, "a") && v.es(i+3, "mayor"):
			fija("ordenar", i, i+3)
		case v.es(i, "al") && v.es(i+1, "reves"):
			if m.Accion == "ordenar" {
				m.Accion = "ordenar_desc"
			}
			fija("invertir", i, i+1)
		case v.es(i, "la|una") && v.es(i+1, "vuelta"):
			fija("invertir", i, i+1)
		case v.es(i, "el|la") && v.es(i+1, "numero|cantidad") && v.es(i+2, "de"):
			fija("contar", i, i+1)
		case v.es(i, "es|sea|son|sean") && v.concepto(i+1) == "palindromo":
			fija("palindromo", i, i+1)
		case v.concepto(i) == "palindromo":
			fija("palindromo", i, i)
		case v.concepto(i) == "anagrama":
			fija("anagrama", i, i)
		case v.es(i, "factorial"):
			fija("factorial", i, i)
		case v.es(i, "fibonacci"):
			fija("fibonacci", i, i)
		case v.es(i, "a") && v.es(i+1, "binario"):
			fija("binario", i, i+1)
		case v.es(i, "suma") && v.es(i+1, "de") && v.es(i+2, "sus|los") && v.es(i+3, "digitos|cifras"):
			fija("suma_digitos", i, i+3)
		}
	}
	// strong actions from the lexicon, in order; weak ones only if nothing else
	debil := ""
	debilI := -1
	for i := range v.norma {
		if m.Accion != "" {
			break
		}
		if v.usado[i] || verbosGenericos[v.l(i)] {
			continue
		}
		c := v.concepto(i)
		if !accionesMarco[c] {
			continue
		}
		if v.es(i, "mayor|menor|mayores|menores") && v.es(i+1, "que|a") {
			continue
		}
		if accionesDebiles[c] {
			if debil == "" {
				debil, debilI = c, i
			}
			continue
		}
		m.Accion = c
		v.usado[i] = true
	}
	if m.Accion == "" && debil != "" {
		m.Accion = debil
		v.usado[debilI] = true
	}
	// "diga si …", "comprueba si …", "es …?"
	if m.Accion == "" || m.Accion == "buscar" {
		if v.frase(0, "diga|dice|di|dime|indique|indica|comprueba|compruebe|verifique|verifica|determine|determina|sepa|saber|devuelva|devuelve", "si") >= 0 ||
			v.frase(0, "si", "es|son|esta|estan|hay|tiene|contiene") >= 0 {
			if m.Accion == "" {
				m.Accion = "comprobar"
			}
		}
	}
}

// ajustarAccion settles cases that depend on the object and modifiers.
func (v *vista) ajustarAccion(m *nucleo.Marco) {
	switch {
	case m.Accion == "" && v.tieneMod(m, "primo") && (m.Entrada == "numero" || v.frase(0, "hasta|menores|menor") >= 0) && len(m.Mods) == 1:
		m.Accion = "primos"
		v.quitarMod(m, "primo")
		if m.Entrada == "" || m.Entrada == "lista_numeros" {
			m.Entrada, m.Objeto = "numero", "numero"
		}
	case m.Accion == "eliminar" && v.tieneMod(m, "sin_repetir"):
		m.Accion = "sin_repetir"
		v.quitarMod(m, "sin_repetir")
	case m.Accion == "eliminar" && v.frase(0, "repetidos|repetidas|duplicados|duplicadas") >= 0:
		m.Accion = "sin_repetir"
	case m.Accion == "eliminar" && len(m.Mods) > 0:
		// "quita los pares" keeps the others
		m.Accion = "filtrar"
		for k := range m.Mods {
			m.Mods[k].Negado = !m.Mods[k].Negado
		}
	case m.Accion == "" && len(m.Mods) > 0 && !(len(m.Mods) == 1 && m.Mods[0].Concepto == "sin_repetir"):
		m.Accion = "filtrar"
	case m.Accion == "" && v.tieneMod(m, "sin_repetir"):
		m.Accion = "sin_repetir"
		v.quitarMod(m, "sin_repetir")
	case m.Accion == "comprobar" && v.tieneMod(m, "primo") && m.Entrada == "":
		m.Entrada, m.Objeto = "numero", "numero"
	}
	if m.Accion == "" && (v.frase(0, "cuantas|cuantos") >= 0) && (m.Elemento != "" || m.Objeto != "") {
		m.Accion = "contar"
	}
	if (m.Accion == "mcd" || m.Accion == "mcm") && m.Entrada == "" || (m.Accion == "mcd" || m.Accion == "mcm") && m.Entrada == "numero" {
		m.Entrada, m.Objeto = "dos_numeros", "numeros"
	}
	if m.Accion == "maximo" || m.Accion == "minimo" {
		// "el mayor de dos números"
		if m.Entrada == "lista_numeros" && v.frase(0, "dos", "numeros|enteros") >= 0 {
			m.Entrada = "dos_numeros"
		}
	}
	if m.Entrada == "" {
		switch m.Accion {
		case "factorial", "fibonacci", "primos", "digitos", "suma_digitos", "invertir_numero", "binario", "raiz", "absoluto", "rango":
			m.Entrada, m.Objeto = "numero", "numero"
		case "mayusculas", "minusculas", "titulo", "capitalizar", "quitar_espacios", "palindromo", "anagrama":
			m.Entrada = "texto"
		}
	}
	if m.Accion == "palindromo" && m.Entrada == "numero" {
		m.Entrada = "numero"
	}
}

// sustantivo classifies a noun lemma: lista, numero, decimal, palabra, frase, texto, letra, matriz, mapa.
func sustantivo(lema string) string {
	switch lema {
	case "lista", "arreglo", "vector", "slice", "array", "secuencia", "coleccion":
		return "lista"
	case "numero", "entero", "valor":
		return "numero"
	case "decimal", "real":
		return "decimal"
	case "palabra", "nombre":
		return "palabra"
	case "frase", "oracion":
		return "frase"
	case "texto", "cadena", "string":
		return "texto"
	case "letra", "caracter", "simbolo", "runa":
		return "letra"
	case "matriz", "tabla":
		return "matriz"
	case "mapa", "diccionario":
		return "mapa"
	}
	return ""
}

var elementosPredicado = map[string]string{
	"vocal": "vocal", "consonante": "consonante", "digito": "digito", "cifra": "digito", "mayuscula": "mayuscula",
	"minuscula": "minuscula", "espacio": "espacio",
}

func (v *vista) objeto(m *nucleo.Marco) {
	type mencion struct {
		clase  string
		plural bool
		i      int
	}
	var ms []mencion
	for i := range v.norma {
		if v.toks[i].Clase != "palabra" || v.usado[i] && !v.es(i, "numero|numeros|palabra|palabras") {
			continue
		}
		if v.es(i, "tabla") && v.es(i+1, "de") && v.prefijo(i+2, "multiplicar") {
			continue
		}
		c := sustantivo(v.l(i))
		if c == "" {
			if e, ok := elementosPredicado[v.l(i)]; ok && !(m.Accion == "mayusculas" || m.Accion == "minusculas" || m.Accion == "titulo") {
				if m.Elemento == "" {
					m.Elemento = e
				}
			}
			continue
		}
		if v.es(i, "numero") && v.es(i-1, "el") && v.es(i+1, "de") && m.Accion == "contar" {
			continue // "el número de vocales"
		}
		ms = append(ms, mencion{clase: c, plural: strings.HasSuffix(v.n(i), "s") && c != "matriz", i: i})
	}
	tiene := func(c string, plural int) (mencion, bool) {
		for _, x := range ms {
			if x.clase == c && (plural < 0 || plural == 0 && !x.plural || plural == 1 && x.plural) {
				return x, true
			}
		}
		return mencion{}, false
	}
	accPalabra := map[string]bool{"contar": true, "frecuencia": true, "mas_largo": true, "mas_corto": true, "separar": true,
		"titulo": true, "capitalizar": true, "maximo": true, "minimo": true, "primero": true, "ultimo": true, "longitud": true}
	if x, ok := tiene("lista", -1); ok {
		m.Objeto, m.Entrada = "lista", "lista_numeros"
		// "lista de X"
		desc := ""
		for k := x.i + 1; k < len(v.norma) && k <= x.i+3; k++ {
			if s := sustantivo(v.l(k)); s != "" && s != "lista" {
				desc = s
				break
			}
		}
		if desc == "" {
			if _, ok := tiene("palabra", 1); ok {
				desc = "palabra"
			} else if _, ok := tiene("decimal", -1); ok {
				desc = "decimal"
			} else if _, ok := tiene("texto", 1); ok {
				desc = "palabra"
			}
		}
		switch desc {
		case "palabra", "texto", "frase":
			m.Entrada = "lista_palabras"
			m.Elemento = cond(m.Elemento == "", "palabra", m.Elemento)
		case "decimal":
			m.Entrada = "lista_decimales"
			m.Elemento = cond(m.Elemento == "", "numero", m.Elemento)
		default:
			if _, ok := tiene("numero", -1); ok {
				m.Elemento = cond(m.Elemento == "", "numero", m.Elemento)
			}
		}
	} else if _, ok := tiene("matriz", -1); ok {
		m.Objeto, m.Entrada = "matriz", "matriz"
	} else if _, ok := tiene("frase", -1); ok {
		m.Objeto, m.Entrada = "frase", "texto"
		if _, ok := tiene("palabra", -1); ok {
			m.Elemento = cond(m.Elemento == "", "palabra", m.Elemento)
		}
	} else if _, ok := tiene("texto", -1); ok {
		m.Objeto, m.Entrada = "texto", "texto"
		if _, ok := tiene("palabra", -1); ok {
			m.Elemento = cond(m.Elemento == "", "palabra", m.Elemento)
		}
	} else if x, ok := tiene("palabra", -1); ok {
		switch {
		case accPalabra[m.Accion] && (x.plural || m.Accion == "mas_largo" || m.Accion == "mas_corto"):
			m.Elemento = cond(m.Elemento == "", "palabra", m.Elemento)
			if x.plural && m.Accion != "mas_largo" && m.Accion != "mas_corto" && m.Accion != "contar" && m.Accion != "frecuencia" && m.Accion != "separar" && m.Accion != "titulo" && m.Accion != "capitalizar" {
				m.Objeto, m.Entrada = "palabras", "lista_palabras"
			} else {
				m.Entrada = "texto"
			}
		case x.plural:
			m.Objeto, m.Entrada = "palabras", "lista_palabras"
			m.Elemento = cond(m.Elemento == "", "palabra", m.Elemento)
		default:
			m.Objeto, m.Entrada = "palabra", "texto"
		}
	} else if _, ok := tiene("decimal", 1); ok {
		m.Objeto, m.Entrada, m.Elemento = "numeros", "lista_decimales", cond(m.Elemento == "", "numero", m.Elemento)
	} else if x, ok := tiene("numero", 1); ok {
		if v.es(x.i-1, "dos") || v.n(x.i-1) == "2" {
			m.Objeto, m.Entrada = "numeros", "dos_numeros"
		} else {
			m.Objeto, m.Entrada = "numeros", "lista_numeros"
			m.Elemento = cond(m.Elemento == "", "numero", m.Elemento)
		}
	} else if _, ok := tiene("numero", 0); ok {
		m.Objeto, m.Entrada = "numero", "numero"
	} else if _, ok := tiene("decimal", 0); ok {
		m.Objeto, m.Entrada = "numero", "decimal"
	} else if x, ok := tiene("letra", -1); ok {
		if x.plural {
			m.Elemento = cond(m.Elemento == "", "letra", m.Elemento)
			m.Entrada = "texto"
		} else {
			m.Objeto, m.Entrada = "letra", "letra"
		}
	} else if _, ok := tiene("mapa", -1); ok {
		m.Objeto = "mapa"
	}
	if m.Entrada == "" && m.Elemento != "" && m.Elemento != "numero" {
		m.Entrada = "texto"
	}
	// explicit input: "recibe una lista de enteros"
	for i := range v.norma {
		if v.es(i, "recibe|reciba|toma|tome|dada|dado|dadas|dados") {
			for k := i + 1; k < len(v.norma) && k <= i+3; k++ {
				switch sustantivo(v.l(k)) {
				case "lista":
					m.Entrada = "lista_numeros"
					if v.es(k+1, "de") && sustantivo(v.l(k+2)) == "palabra" || sustantivo(v.l(k+2)) == "texto" {
						m.Entrada = "lista_palabras"
					} else if v.es(k+1, "de") && sustantivo(v.l(k+2)) == "decimal" {
						m.Entrada = "lista_decimales"
					}
				case "texto", "frase", "palabra":
					if strings.HasSuffix(v.n(k), "s") {
						m.Entrada = "lista_palabras"
					} else {
						m.Entrada = "texto"
					}
				case "numero":
					if strings.HasSuffix(v.n(k), "s") {
						m.Entrada = "lista_numeros"
						if v.es(k-1, "dos") {
							m.Entrada = "dos_numeros"
						}
					} else {
						m.Entrada = "numero"
					}
				default:
					continue
				}
				break
			}
		}
	}
	// number agreement for superlatives: "la palabra más larga" / "las palabras más largas"
	for i := range v.norma {
		if v.es(i, "mas") && v.prefijo(i+1, "larg|cort|grande|pequeñ|alt|baj") {
			if strings.HasSuffix(v.n(i+1), "s") || strings.HasSuffix(v.n(i-1), "s") && sustantivo(v.l(i-1)) != "" {
				m.Plural = true
			}
		}
	}
}

func cond(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func (v *vista) salida(m *nucleo.Marco) {
	texto := m.Entrada == "texto"
	lista := strings.HasPrefix(m.Entrada, "lista")
	switch m.Accion {
	case "sumar", "contar", "longitud", "producto", "multiplicar", "restar", "dividir", "factorial", "fibonacci", "mcd",
		"mcm", "suma_digitos", "potencia", "invertir_numero", "posicion", "a_numero":
		m.Salida = "numero"
		if m.Entrada == "lista_decimales" && m.Accion != "contar" && m.Accion != "longitud" && m.Accion != "posicion" {
			m.Salida = "decimal"
		}
	case "absoluto", "raiz", "doble", "triple", "mitad", "cuadrado", "redondear":
		m.Salida = cond(lista, "lista", "numero")
		if m.Accion == "raiz" && !lista {
			m.Salida = "decimal"
		}
	case "maximo", "minimo", "primero", "ultimo":
		switch {
		case m.Plural:
			m.Salida = "lista"
		case m.Elemento == "palabra" || m.Entrada == "lista_palabras" || texto:
			m.Salida = "texto"
		case m.Entrada == "lista_decimales":
			m.Salida = "decimal"
		default:
			m.Salida = "numero"
		}
	case "mas_largo", "mas_corto":
		m.Salida = cond(m.Plural, "lista", "texto")
	case "comprobar", "palindromo", "anagrama", "contiene", "todos", "alguno", "ninguno", "comparar":
		m.Salida = "booleano"
	case "promedio":
		m.Salida = "decimal"
	case "frecuencia":
		m.Salida = "mapa"
	case "filtrar", "ordenar", "ordenar_desc", "sin_repetir", "transformar", "eliminar", "agregar", "acumular", "tomar",
		"quitar_primeros", "intercambiar":
		m.Salida = cond(texto && m.Elemento != "palabra", "texto", "lista")
		if texto && m.Elemento == "palabra" && (m.Accion == "ordenar" || m.Accion == "ordenar_desc" || m.Accion == "filtrar") {
			m.Salida = "lista"
		}
	case "invertir":
		switch {
		case texto:
			m.Salida = "texto"
		case m.Entrada == "numero":
			m.Salida = "numero"
		default:
			m.Salida = "lista"
		}
	case "primos", "digitos", "separar", "rango", "transponer", "aplanar":
		m.Salida = "lista"
	case "mayusculas", "minusculas", "titulo", "capitalizar", "quitar_espacios", "reemplazar", "repetir", "unir",
		"concatenar", "a_texto", "binario":
		m.Salida = "texto"
	}
}

// FirmaProbable proposes a signature from the frame and/or the example pairs. Examples win over the
// frame when they disagree; the frame gives the name and parameter names. It returns nil when neither
// gives one.
func FirmaProbable(m *nucleo.Marco, pares [][2]string) *nucleo.Firma {
	fm := firmaDeMarco(m)
	if len(pares) > 0 {
		if fm != nil {
			if _, f, err := TiparEjemplos(pares, fm); err == nil && coincideInferida(pares, fm) {
				return f
			}
		}
		if _, f, err := TiparEjemplos(pares, nil); err == nil {
			if m != nil && m.Accion != "" {
				f.Nombre = NombreFuncion(m)
			}
			return f
		}
	}
	return fm
}

// coincideInferida reports whether the types inferred from the examples unify with f's.
func coincideInferida(pares [][2]string, f *nucleo.Firma) bool {
	_, g, err := TiparEjemplos(pares, nil)
	if err != nil {
		return true // the examples alone cannot be typed; trust the frame
	}
	a, b := f.Entradas(), g.Entradas()
	if len(a) != len(b) || len(f.Res) != len(g.Res) {
		return false
	}
	for i := range a {
		if _, ok := nucleo.Unificar(a[i], b[i]); !ok && !(a[i].Clase == nucleo.CString || b[i].Clase == nucleo.CString && a[i].Clase == nucleo.CRune) {
			return false
		}
	}
	for i := range f.Res {
		if _, ok := nucleo.Unificar(f.Res[i], g.Res[i]); !ok {
			return false
		}
	}
	return true
}

func tipoEntrada(e string) ([]nucleo.Param, bool) {
	switch e {
	case "lista_numeros":
		return []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, true
	case "lista_decimales":
		return []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TFloat)}}, true
	case "lista_palabras":
		return []nucleo.Param{{Nombre: "palabras", Tipo: nucleo.ListaDe(nucleo.TString)}}, true
	case "texto":
		return []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, true
	case "numero":
		return []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, true
	case "decimal":
		return []nucleo.Param{{Nombre: "x", Tipo: nucleo.TFloat}}, true
	case "dos_numeros":
		return []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, true
	case "letra":
		return []nucleo.Param{{Nombre: "c", Tipo: nucleo.TRune}}, true
	case "matriz":
		return []nucleo.Param{{Nombre: "matriz", Tipo: nucleo.ListaDe(nucleo.ListaDe(nucleo.TInt))}}, true
	}
	return nil, false
}

func firmaDeMarco(m *nucleo.Marco) *nucleo.Firma {
	if m == nil {
		return nil
	}
	entrada := m.Entrada
	var params []nucleo.Param
	if m.Programa && len(m.Lee) > 0 {
		var ts []nucleo.Tipo
		for _, x := range m.Lee {
			ps, ok := tipoEntrada(x)
			if !ok {
				return nil
			}
			ts = append(ts, ps[0].Tipo)
		}
		params = paramsPorTipo(ts)
		if len(m.Lee) == 1 {
			entrada = m.Lee[0]
		} else if len(m.Lee) == 2 && m.Lee[0] == "numero" && m.Lee[1] == "numero" {
			entrada = "dos_numeros"
		}
	} else {
		ps, ok := tipoEntrada(entrada)
		if !ok {
			return nil
		}
		params = ps
		if entrada == "texto" {
			switch m.Objeto {
			case "frase":
				params[0].Nombre = "frase"
			case "palabra":
				params[0].Nombre = "palabra"
			case "texto":
				params[0].Nombre = "texto"
			}
		}
	}
	elem := nucleo.TInt
	switch entrada {
	case "lista_decimales", "decimal":
		elem = nucleo.TFloat
	case "lista_palabras":
		elem = nucleo.TString
	}
	var res nucleo.Tipo
	switch m.Salida {
	case "numero":
		res = nucleo.TInt
		if m.Accion == "promedio" || elem.Clase == nucleo.CFloat && m.Accion != "contar" && m.Accion != "longitud" && m.Accion != "posicion" {
			res = nucleo.TFloat
		}
	case "decimal":
		res = nucleo.TFloat
	case "booleano":
		res = nucleo.TBool
	case "texto":
		res = nucleo.TString
	case "lista":
		switch {
		case strings.HasPrefix(entrada, "lista") || entrada == "matriz" && m.Accion != "aplanar" && m.Accion != "transponer":
			res = params[0].Tipo
		case entrada == "matriz" && m.Accion == "transponer":
			res = params[0].Tipo
		case entrada == "matriz":
			res = nucleo.ListaDe(nucleo.TInt)
		case entrada == "texto":
			res = nucleo.ListaDe(nucleo.TString)
		default:
			res = nucleo.ListaDe(elem)
		}
	case "mapa":
		switch {
		case m.Elemento == "palabra" || entrada == "lista_palabras":
			res = nucleo.MapaDe(nucleo.TString, nucleo.TInt)
		case entrada == "texto":
			res = nucleo.MapaDe(nucleo.TRune, nucleo.TInt)
		default:
			res = nucleo.MapaDe(elem, nucleo.TInt)
		}
	default:
		return nil
	}
	return &nucleo.Firma{Nombre: NombreFuncion(m), Params: params, Res: []nucleo.Tipo{res}}
}

var nombresAccion = map[string]string{
	"sumar": "Suma", "restar": "Resta", "multiplicar": "Multiplica", "dividir": "Divide", "contar": "Cuenta",
	"filtrar": "Filtra", "transformar": "Transforma", "ordenar": "Ordena", "ordenar_desc": "OrdenaDesc",
	"invertir": "Invierte", "maximo": "Maximo", "minimo": "Minimo", "promedio": "Promedio", "producto": "Producto",
	"unir": "Une", "separar": "Separa", "mayusculas": "Mayusculas", "minusculas": "Minusculas", "titulo": "Titulo",
	"capitalizar": "Capitaliza", "quitar_espacios": "QuitaEspacios", "reemplazar": "Reemplaza", "repetir": "Repite",
	"buscar": "Busca", "contiene": "Contiene", "posicion": "Posicion", "primero": "Primero", "ultimo": "Ultimo",
	"tomar": "Toma", "quitar_primeros": "QuitaPrimeros", "sin_repetir": "SinRepetir", "frecuencia": "Frecuencias",
	"longitud": "Longitud", "todos": "Todos", "alguno": "Alguno", "ninguno": "Ninguno", "comprobar": "Es",
	"a_texto": "ATexto", "a_numero": "ANumero", "factorial": "Factorial", "fibonacci": "Fibonacci", "primos": "Primos",
	"mcd": "Mcd", "mcm": "Mcm", "digitos": "Digitos", "suma_digitos": "SumaDigitos", "invertir_numero": "InvierteNumero",
	"potencia": "Potencia", "cuadrado": "Cuadrados", "raiz": "Raiz", "absoluto": "Absoluto", "doble": "Doble",
	"triple": "Triple", "mitad": "Mitad", "rango": "Rango", "acumular": "Acumula", "transponer": "Transpone",
	"aplanar": "Aplana", "binario": "Binario", "palindromo": "EsPalindromo", "anagrama": "EsAnagrama",
	"concatenar": "Concatena", "agregar": "Agrega", "eliminar": "Elimina", "intercambiar": "Intercambia",
	"redondear": "Redondea", "comparar": "Compara",
}

var nombresMod = map[string][2]string{ // plural, singular
	"par": {"Pares", "Par"}, "impar": {"Impares", "Impar"}, "positivo": {"Positivos", "Positivo"},
	"negativo": {"Negativos", "Negativo"}, "primo": {"Primos", "Primo"}, "cero": {"Ceros", "Cero"},
	"mayor_que": {"Mayores", "Mayor"}, "menor_que": {"Menores", "Menor"}, "igual_a": {"Iguales", "Igual"},
	"distinto_de": {"Distintos", "Distinto"}, "divisible_por": {"Divisibles", "Divisible"},
	"sin_repetir": {"SinRepetir", "SinRepetir"}, "empieza_con": {"QueEmpiezan", "QueEmpieza"},
	"termina_con": {"QueTerminan", "QueTermina"}, "longitud_mayor": {"Largas", "Larga"},
	"longitud_menor": {"Cortas", "Corta"}, "vocal": {"Vocales", "Vocal"}, "consonante": {"Consonantes", "Consonante"},
	"digito": {"Digitos", "Digito"}, "mayuscula": {"Mayusculas", "Mayuscula"}, "minuscula": {"Minusculas", "Minuscula"},
	"letra": {"Letras", "Letra"}, "palabra": {"Palabras", "Palabra"}, "espacio": {"Espacios", "Espacio"},
}

// NombreFuncion builds a Go name from the frame: "SumaPares", "CuentaVocales", "PalabraMasLarga", "EsPrimo".
func NombreFuncion(m *nucleo.Marco) string {
	if m == nil || m.Accion == "" {
		return "Funcion"
	}
	var sb strings.Builder
	switch m.Accion {
	case "mas_largo", "mas_corto":
		suf := map[bool]string{false: "Larga", true: "Largo"}
		if m.Accion == "mas_corto" {
			suf = map[bool]string{false: "Corta", true: "Corto"}
		}
		elem := m.Elemento
		if elem == "" {
			elem = "palabra"
		}
		base := map[string]string{"palabra": "Palabra", "numero": "Numero", "letra": "Letra", "frase": "Frase"}[elem]
		if base == "" {
			base = "Palabra"
		}
		masc := elem == "numero"
		if m.Plural {
			sb.WriteString(base + "sMas" + suf[masc] + "s")
		} else {
			sb.WriteString(base + "Mas" + suf[masc])
		}
		return sb.String()
	case "comprobar":
		sb.WriteString("Es")
		for _, md := range m.Mods {
			if n, ok := nombresMod[md.Concepto]; ok {
				if md.Negado {
					sb.WriteString("No")
				}
				sb.WriteString(n[1])
			}
		}
		if sb.Len() == 2 {
			sb.WriteString("Valido")
		}
		return sb.String()
	}
	base, ok := nombresAccion[m.Accion]
	if !ok {
		base = "Funcion"
	}
	sb.WriteString(base)
	puesto := false
	for _, md := range m.Mods {
		if n, ok := nombresMod[md.Concepto]; ok {
			if md.Negado {
				sb.WriteString("No")
			}
			sb.WriteString(n[0])
			puesto = true
		}
	}
	if n, ok := nombresMod[m.Elemento]; ok && !v0tieneMod(m, m.Elemento) &&
		(m.Elemento != "palabra" || m.Accion == "contar" || m.Accion == "frecuencia" || m.Accion == "invertir" || m.Accion == "ordenar") {
		sb.WriteString(n[0])
		puesto = true
	}
	if !puesto && m.Entrada == "texto" && (m.Accion == "invertir" || m.Accion == "ordenar" || m.Accion == "contar" || m.Accion == "longitud") {
		switch m.Objeto {
		case "frase":
			sb.WriteString("Frase")
		case "palabra":
			sb.WriteString("Palabra")
		default:
			sb.WriteString("Texto")
		}
	}
	return limpiarNombre(sb.String())
}

func limpiarNombre(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			sb.WriteRune(r)
		}
	}
	if sb.Len() == 0 {
		return "Funcion"
	}
	return sb.String()
}

func v0tieneMod(m *nucleo.Marco, c string) bool {
	for _, x := range m.Mods {
		if x.Concepto == c {
			return true
		}
	}
	return false
}
