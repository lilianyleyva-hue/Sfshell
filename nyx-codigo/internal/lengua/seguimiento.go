package lengua

import (
	"regexp"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Seguimiento is a recognized follow-up to the previous turn.
type Seguimiento struct {
	Tipo    string // "cambiar_numero" | "por_que" | "mas_simple" | "mas_detalle" | "otra_forma" | "esta_mal" | "esta_bien" | "probar_con" | "vacia" | "comentarios" | "mas_rapido" | "modificar" | "explicar_eso" | "ejecutar_eso" | "programa_eso"
	Numero  *nucleo.Numero
	Ejemplo string // "con [2] debe dar 0"
	Entrada string // "[3,3]" for probar_con
	Texto   string
}

var (
	reSegVacia     = rx(`^(y )?(si|con|cuando) (la lista|el texto|la cadena|la entrada|la frase|el arreglo|la palabra|el slice|una lista|un texto|una cadena) (esta |es |estuviera |fuera )?vaci[ao]$|^(y )?con (una lista|un texto|una cadena|un arreglo) vaci[ao]$|^(y )?(con|si es|si fuera) (\[\]|"")$`)
	reSegCambio    = rx(`^(y )?(si|que pasa si|y si) (fuera|fueran|es|son|fuese|fuesen|vale|valiera|valen|pongo|cambio .* por|en vez de .* es|el numero es|el numero fuera) (.+)$`)
	reSegYCon      = rx(`^y (con|para) (.+)$`)
	reSegPorQue    = rx(`^(y )?(por que|porque|explicame por que|dime por que|por que es asi|por que da eso|por que sale eso|como lo sabes|justificalo|demuestralo)( .{0,40})?$`)
	reSegSimple    = rx(`\b(mas (facil|simple|sencillo|claro)|no (lo )?entiendo|no me entero|para (un )?niños?|en palabras sencillas)\b`)
	reSegDetalle   = rx(`\b(mas detalle|mas detalles|con (mas )?detalle|mas a fondo|paso a paso|mas despacio|detalladamente)\b`)
	reSegOtra      = rx(`\b(de otra (forma|manera)|otra (forma|manera|solucion|version)|de otro modo|hazlo distinto|algo diferente)\b`)
	reSegMal       = rx(`^(no|esta mal|incorrecto|incorrecta|mal|eso no es|no es eso|falla|no es correcto|no es correcta|eso esta mal|te equivocas|error|no funciona|no esta bien)(\s*[,:.;!]?\s*(.*))?$`)
	reSegBien      = rx(`^(bien|muy bien|perfecto|correcto|gracias|muchas gracias|genial|vale|ok|okay|esta bien|eso es|exacto|funciona|si|si señor|estupendo|de acuerdo|buenisimo|guay)( [a-z ]{0,20})?$`)
	reSegProbar    = rx(`^(y )?(pruebala|pruebalo|prueba|probar|probala|ejecutala|ejecutalo|ejecuta|corre|correla|correlo|prueba la funcion|prueba el codigo|y) (con|para) (.+)$`)
	reSegComent    = rx(`\b(ponle comentarios|ponele comentarios|comentalo|comentala|añade comentarios|añadele comentarios|con comentarios|agrega comentarios|documentalo)\b`)
	reSegRapido    = rx(`\b(hazlo mas rapido|hazla mas rapida|mas rapido|mas rapida|optimizalo|optimizala|mas eficiente|que vaya mas rapido|acelera(lo|la))\b`)
	reSegModificar = rx(`^(ahora|y ahora|y|pero|mejor|cambialo|cambiala) (con|para|que|sin|en|de|solo|usando|sobre) (.+)$`)
	reSegExplicar  = rx(`^(explicalo|explicala|explicamelo|explicamela|explica eso|explica esto|explicame eso|explicame el codigo|explica el codigo|que hace eso|que hace esto|que hace el codigo|como funciona eso|como funciona)$`)
	reSegEjecutar  = rx(`^(ejecutalo|ejecutala|ejecuta eso|ejecutalo ahora|correlo|correla|corre eso|ejecuta el codigo|ejecuta el programa|lanzalo|pruebalo ahora)$`)
	reSegPrograma  = rx(`\b(hazlo programa|hazla programa|hazlo un programa|hazla un programa|conviertelo en (un )?programa|conviertela en (un )?programa|como programa|hazme un programa con (eso|esto|ella|esa funcion)|un programa que la use|haz un programa con ella)\b`)
	reEjemploEnMal = regexp.MustCompile(`(?i)\bcon\s+(.+?)\s+(debe dar|deberia dar|debería dar|tiene que dar|da|devuelve|sale)\s+(.+)$`)
)

// maxSeguimiento: a follow-up is a short message; longer texts are new requests.
const maxSeguimiento = 600

// DetectarSeguimiento recognizes a follow-up to the previous turn (§4.8, algorithm 5). It needs the
// session context; without it, nothing is a follow-up.
func DetectarSeguimiento(texto string, ctx *nucleo.Contexto) (Seguimiento, bool) {
	if ctx == nil || len(texto) > maxSeguimiento {
		return Seguimiento{}, false
	}
	t := strings.TrimSpace(texto)
	n := strings.TrimSpace(strings.Trim(nucleo.Normalizar(t), " .;,:"))
	if n == "" {
		return Seguimiento{}, false
	}
	s := Seguimiento{Texto: t}
	pals := len(strings.Fields(n))
	switch {
	case reSegVacia.MatchString(n):
		s.Tipo = "vacia"
	case reSegPorQue.MatchString(n):
		s.Tipo = "por_que"
	case reSegComent.MatchString(n):
		s.Tipo = "comentarios"
	case reSegPrograma.MatchString(n):
		s.Tipo = "programa_eso"
	case reSegExplicar.MatchString(n):
		s.Tipo = "explicar_eso"
	case reSegEjecutar.MatchString(n):
		s.Tipo = "ejecutar_eso"
	case reSegRapido.MatchString(n) && pals <= 6:
		s.Tipo = "mas_rapido"
	case reSegOtra.MatchString(n) && pals <= 7:
		s.Tipo = "otra_forma"
	case reSegSimple.MatchString(n) && pals <= 7:
		s.Tipo = "mas_simple"
	case reSegDetalle.MatchString(n) && pals <= 7:
		s.Tipo = "mas_detalle"
	case reSegCambio.MatchString(n) && cambioValido(n, t, reSegCambio.FindStringSubmatch(n), &s):
	case reSegProbar.MatchString(n):
		m := reSegProbar.FindStringSubmatch(n)
		ent := m[len(m)-1]
		if v, ok := valorLiteral(t); ok {
			ent = v
		}
		if nums := Numeros(ent); len(nums) == 1 && strings.TrimSpace(nums[0].Texto) == strings.TrimSpace(ent) && strings.HasPrefix(n, "y ") && !esCodigoCtx(ctx) {
			s.Tipo, s.Numero = "cambiar_numero", &nums[0]
		} else if strings.HasPrefix(n, "y ") && !literalClaro(ent) && len(Numeros(ent)) == 0 {
			s.Tipo, s.Texto = "modificar", strings.TrimPrefix(n, "y ")
		} else {
			s.Tipo, s.Entrada = "probar_con", ent
		}
	case reSegYCon.MatchString(n):
		m := reSegYCon.FindStringSubmatch(n)
		if nums := Numeros(m[2]); len(nums) > 0 {
			s.Tipo, s.Numero = "cambiar_numero", &nums[len(nums)-1]
		} else {
			s.Tipo, s.Texto = "modificar", m[1]+" "+m[2]
		}
	case reSegMal.MatchString(n) && esQueja(n, t, pals):
		s.Tipo = "esta_mal"
		if m := reEjemploEnMal.FindStringSubmatch(t); m != nil {
			s.Ejemplo = "con " + strings.TrimSpace(m[1]) + " " + strings.ToLower(m[2]) + " " + strings.TrimRight(strings.TrimSpace(m[3]), ".!")
		}
	case reSegBien.MatchString(n) && pals <= 4 && afirmacion(n):
		s.Tipo = "esta_bien"
	case reSegModificar.MatchString(n) && modificacionCorta(n, pals):
		m := reSegModificar.FindStringSubmatch(n)
		s.Tipo = "modificar"
		s.Texto = m[2] + " " + m[3]
	default:
		return Seguimiento{}, false
	}
	return s, true
}

// reCorteFrase finds a sentence break inside a message ("…; es de noche. ¿hace frío?").
var reCorteFrase = regexp.MustCompile(`[;]|[.?!]\s+\S`)

// articulosCola are the words that may surround the new value: "si fueran las 5", "y si es de 3".
var articulosCola = conjunto(`el la los las lo un una unos unas de del a al en`)

// cambioValido decides a "¿y si fuera N?" message. The part after the verb must be just a value: a
// number with at most one more word ("13", "20 personas"; two after "y si") or a literal, in a single
// short clause. "si es de noche hace frío; es de noche. ¿hace frío?", "si son las 5 Ana sale; …" and
// "si fuera rico compraría una casa" are new requests (logic, a statement), not follow-ups. Without a
// value, only "y si …"/"qué pasa si …" asks to modify the previous request.
func cambioValido(n, t string, m []string, s *Seguimiento) bool {
	cola := strings.TrimSpace(m[len(m)-1])
	if reCorteFrase.MatchString(cola) || len(strings.Fields(n)) > 9 {
		return false
	}
	conY := strings.HasPrefix(n, "y ") || strings.HasPrefix(n, "que pasa") || strings.HasPrefix(n, "y que pasa")
	extra := 1
	if conY {
		extra = 2
	}
	sobran := func(fuera string) int {
		k := 0
		for _, w := range strings.Fields(fuera) {
			if !articulosCola[w] {
				k++
			}
		}
		return k
	}
	if nums := Numeros(cola); len(nums) == 1 {
		if sobran(cola[:nums[0].Desde]+" "+cola[nums[0].Hasta:]) <= extra {
			s.Tipo, s.Numero = "cambiar_numero", &nums[0]
			return true
		}
		return false
	}
	if v, ok := valorLiteral(t); ok {
		if k := strings.Index(cola, nucleo.Normalizar(v)); k >= 0 && sobran(cola[:k]+" "+cola[k+len(nucleo.Normalizar(v)):]) <= extra {
			s.Tipo, s.Entrada = "probar_con", v
			return true
		}
		return false
	}
	if conY && len(strings.Fields(cola)) <= 5 {
		s.Tipo, s.Texto = "modificar", cola
		return true
	}
	return false
}

// noQueja are words after a leading "no" that make a new statement, not a complaint about the
// answer: "no entiendo los punteros…", "no sé cómo…", "no puedo…".
var noQueja = conjunto(`entiendo entendi se sabia puedo quiero tengo me te hay conozco recuerdo veo encuentro estoy soy`)

// esQueja decides whether a message that starts like a complaint is about the previous answer:
// it carries a counterexample ("con [2] debe dar 0"), or it is short ("no", "está mal", "no funciona",
// "no, no es eso"). A leading "no" followed by a verb of a new statement is not a complaint.
func esQueja(n, t string, pals int) bool {
	m := reSegMal.FindStringSubmatch(n)
	if m == nil {
		return false
	}
	if e := reEjemploEnMal.FindStringSubmatch(t); e != nil {
		x := strings.TrimSpace(e[1])
		if literalClaro(x) || len(Numeros(x)) == 1 && len(strings.Fields(x)) == 1 || strings.HasPrefix(x, "«") {
			return true
		}
	}
	campos := strings.Fields(n)
	if m[1] != "no" {
		return pals <= 6
	}
	if len(campos) > 1 && noQueja[strings.Trim(campos[1], ",:;.")] {
		return false
	}
	if pals <= 3 {
		return true
	}
	// "no, …" / "no: …" with a short reason
	return (strings.HasPrefix(n, "no,") || strings.HasPrefix(n, "no:")) && pals <= 6
}

// siAfirma are the words that may follow a leading "sí" in an approval ("sí, gracias", "sí, está bien");
// otherwise "si" starts a condition ("si es de noche"), not an answer.
var siAfirma = conjunto(`señor gracias perfecto vale claro eso esta exacto correcto genial bien funciona`)

func afirmacion(n string) bool {
	campos := strings.Fields(n)
	if len(campos) < 2 || strings.Trim(campos[0], ",.!") != "si" {
		return true
	}
	return strings.HasSuffix(campos[0], ",") || siAfirma[strings.Trim(campos[1], ",.!")]
}

// modificacionCorta: "ahora con impares", "y para palabras" are short; a long message or one with
// several sentences is a new request even if it starts with "y …" or "pero …".
func modificacionCorta(n string, pals int) bool {
	if reCorteFrase.MatchString(n) {
		return false
	}
	if strings.HasPrefix(n, "ahora ") || strings.HasPrefix(n, "y ahora ") || strings.HasPrefix(n, "cambial") {
		return pals <= 14
	}
	return pals <= 8
}

func esCodigoCtx(ctx *nucleo.Contexto) bool {
	return ctx != nil && (ctx.Firma != nil || ctx.Codigo != "") && nucleo.Familia(ctx.Intencion) == "programar"
}

var reLiteral = regexp.MustCompile(`(\[[^\]]*\]|"[^"]*"|«[^»]*»|“[^”]*”|\{[^}]*\})`)

// valorLiteral returns the first list/quoted/map literal written in t.
func valorLiteral(t string) (string, bool) {
	if m := reLiteral.FindString(t); m != "" {
		return m, true
	}
	return "", false
}

func literalClaro(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "[") || strings.HasPrefix(s, `"`) || strings.HasPrefix(s, "{")
}

// Fusionar merges a follow-up request into the previous one: slots present in nueva override anterior.
// Modifiers of the same family replace each other (par ↔ impar, positivo ↔ negativo, mayor_que ↔
// menor_que…); others are added. The examples of anterior are kept only when the frame did not change.
func Fusionar(anterior *nucleo.Pregunta, nueva *nucleo.Pregunta) *nucleo.Pregunta {
	if anterior == nil {
		return nueva
	}
	if nueva == nil {
		return anterior
	}
	r := *anterior
	// Texto is "anterior; nueva" and every token and number offset indexes it (nucleo contract):
	// the new request's offsets move by where it starts, the old ones by the trimmed leading space.
	pegado := anterior.Texto + "; " + nueva.Texto
	izq := len(pegado) - len(strings.TrimLeftFunc(pegado, unicode.IsSpace))
	r.Texto = strings.TrimSpace(pegado)
	desplA, desplN := -izq, len(anterior.Texto)+2-izq
	if strings.TrimSpace(anterior.Texto) == "" {
		r.Texto = strings.TrimSpace(nueva.Texto)
		desplN = -(len(nueva.Texto) - len(strings.TrimLeftFunc(nueva.Texto, unicode.IsSpace)))
	}
	r.Tokens = make([]nucleo.Token, 0, len(anterior.Tokens)+len(nueva.Tokens))
	r.Tokens = append(r.Tokens, moverTokens(anterior.Tokens, desplA, len(r.Texto))...)
	r.Tokens = append(r.Tokens, moverTokens(nueva.Tokens, desplN, len(r.Texto))...)
	switch {
	case strings.TrimSpace(anterior.Resto) == "":
		r.Resto = nueva.Resto
	case strings.TrimSpace(nueva.Resto) == "":
		r.Resto = anterior.Resto
	default:
		r.Resto = strings.TrimSpace(anterior.Resto) + "; " + strings.TrimSpace(nueva.Resto)
	}
	r.Normal = nucleo.Normalizar(r.Resto)
	if nueva.Codigo != "" {
		r.Codigo = nueva.Codigo
	}
	if nueva.Modo != "" {
		r.Modo = nueva.Modo
	}
	r.Numeros = moverNumeros(anterior.Numeros, desplA, len(r.Texto))
	if len(nueva.Numeros) > 0 {
		r.Numeros = moverNumeros(nueva.Numeros, desplN, len(r.Texto))
	}
	cambio := false
	if anterior.Marco != nil || nueva.Marco != nil {
		var m nucleo.Marco
		if anterior.Marco != nil {
			m = *anterior.Marco
			m.Mods = append([]nucleo.Modificador(nil), anterior.Marco.Mods...)
			m.Lee = append([]string(nil), anterior.Marco.Lee...)
		}
		if n := nueva.Marco; n != nil {
			sustituye := func(dst *string, src string) {
				if src != "" && src != *dst {
					*dst = src
					cambio = true
				}
			}
			// an action that only came from the modifiers ("ahora con impares" → filtrar) does not replace
			if n.Accion != "" && !(n.Accion == "filtrar" && len(n.Mods) > 0 && !tieneVerbo(nueva)) {
				if n.Accion != m.Accion {
					m.Salida = n.Salida // the new action's own output (salida recomputes the rest)
				}
				sustituye(&m.Accion, n.Accion)
			}
			if n.Entrada != "" {
				sustituye(&m.Entrada, n.Entrada)
				sustituye(&m.Objeto, n.Objeto)
				if n.Elemento != "" {
					sustituye(&m.Elemento, n.Elemento)
				}
			} else {
				sustituye(&m.Objeto, n.Objeto)
				sustituye(&m.Elemento, n.Elemento)
			}
			if n.Plural != m.Plural && (n.Accion == "mas_largo" || n.Accion == "mas_corto" || n.Accion == "maximo" || n.Accion == "minimo") {
				m.Plural = n.Plural
				cambio = true
			}
			for _, md := range n.Mods {
				m.Mods = reemplazarMod(m.Mods, md)
				cambio = true
			}
			if n.Programa {
				m.Programa = true
				if len(n.Lee) > 0 {
					m.Lee = n.Lee
				}
				if n.Muestra != "" {
					m.Muestra = n.Muestra
				}
				cambio = true
			}
		}
		// "la letra más repetida" keeps its one-element output (see salida)
		v := &vista{masFrec: m.Accion == "frecuencia" && m.Salida != "" && m.Salida != "mapa"}
		v.salida(&m)
		r.Marco = &m
	}
	switch {
	case len(nueva.ParesTexto) > 0:
		r.ParesTexto = nueva.ParesTexto
		r.Ejemplos = nueva.Ejemplos
	case cambio:
		r.ParesTexto, r.Ejemplos = nil, nil
	}
	if len(nueva.EjProgramas) > 0 {
		r.EjProgramas = nueva.EjProgramas
	} else if cambio {
		r.EjProgramas = nil
	}
	if nueva.Firma != nil && nueva.Marco == nil {
		r.Firma = nueva.Firma
	} else {
		r.Firma = FirmaProbable(r.Marco, r.ParesTexto)
		if r.Firma == nil {
			r.Firma = anterior.Firma
		}
	}
	if r.Firma != nil && len(r.ParesTexto) > 0 {
		if casos, f, err := TiparEjemplos(r.ParesTexto, r.Firma); err == nil {
			r.Ejemplos, r.Firma = casos, f
		}
	}
	r.Conceptos = conceptosDe(r.Marco, r.Tokens, nil)
	r.Intenciones = anterior.Intenciones
	r.ConsultaWeb = ConsultaIngles(&r, nil)
	return &r
}

// moverTokens copies tokens with their offsets moved by d; tokens that would fall outside a text of
// length n (they cannot, for offsets that indexed the original texts) are dropped.
func moverTokens(ts []nucleo.Token, d, n int) []nucleo.Token {
	out := make([]nucleo.Token, 0, len(ts))
	for _, t := range ts {
		t.Desde += d
		t.Hasta += d
		if t.Desde >= 0 && t.Hasta <= n && t.Desde <= t.Hasta {
			out = append(out, t)
		}
	}
	return out
}

func moverNumeros(ns []nucleo.Numero, d, n int) []nucleo.Numero {
	out := make([]nucleo.Numero, 0, len(ns))
	for _, x := range ns {
		x.Desde += d
		x.Hasta += d
		if x.Desde >= 0 && x.Hasta <= n && x.Desde <= x.Hasta {
			out = append(out, x)
		}
	}
	return out
}

// familiaMod groups modifiers that exclude each other.
func familiaMod(c string) string {
	switch c {
	case "par", "impar":
		return "paridad"
	case "positivo", "negativo", "cero":
		return "signo"
	case "mayor_que", "menor_que", "igual_a", "distinto_de":
		return "comparacion"
	case "longitud_mayor", "longitud_menor":
		return "longitud"
	case "vocal", "consonante", "digito", "mayuscula", "minuscula", "letra":
		return "clase_letra"
	}
	return c
}

func reemplazarMod(ms []nucleo.Modificador, md nucleo.Modificador) []nucleo.Modificador {
	f := familiaMod(md.Concepto)
	out := make([]nucleo.Modificador, 0, len(ms)+1)
	for _, x := range ms {
		if familiaMod(x.Concepto) != f {
			out = append(out, x)
		}
	}
	return append(out, md)
}
