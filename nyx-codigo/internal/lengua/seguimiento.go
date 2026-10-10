package lengua

import (
	"regexp"
	"strings"

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

// DetectarSeguimiento recognizes a follow-up to the previous turn (§4.8, algorithm 5). It needs the
// session context; without it, nothing is a follow-up.
func DetectarSeguimiento(texto string, ctx *nucleo.Contexto) (Seguimiento, bool) {
	if ctx == nil {
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
	case reSegCambio.MatchString(n):
		m := reSegCambio.FindStringSubmatch(n)
		cola := m[len(m)-1]
		if nums := Numeros(cola); len(nums) > 0 {
			s.Tipo = "cambiar_numero"
			s.Numero = &nums[len(nums)-1]
		} else if v, ok := valorLiteral(t); ok {
			s.Tipo, s.Entrada = "probar_con", v
		} else {
			s.Tipo = "modificar"
			s.Texto = cola
		}
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
	case reSegMal.MatchString(n) && (pals <= 3 || strings.Contains(n, "con ") || reSegMal.FindStringSubmatch(n)[1] != "no" || strings.HasPrefix(n, "no,") || strings.HasPrefix(n, "no:")):
		s.Tipo = "esta_mal"
		if m := reEjemploEnMal.FindStringSubmatch(t); m != nil {
			s.Ejemplo = "con " + strings.TrimSpace(m[1]) + " " + strings.ToLower(m[2]) + " " + strings.TrimRight(strings.TrimSpace(m[3]), ".!")
		}
	case reSegBien.MatchString(n) && pals <= 4:
		s.Tipo = "esta_bien"
	case reSegModificar.MatchString(n):
		m := reSegModificar.FindStringSubmatch(n)
		s.Tipo = "modificar"
		s.Texto = m[2] + " " + m[3]
	default:
		return Seguimiento{}, false
	}
	return s, true
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
	r.Texto = strings.TrimSpace(anterior.Texto + "; " + nueva.Texto)
	r.Normal = nucleo.Normalizar(r.Texto)
	r.Tokens = append(append([]nucleo.Token(nil), anterior.Tokens...), nueva.Tokens...)
	if nueva.Codigo != "" {
		r.Codigo = nueva.Codigo
	}
	if nueva.Modo != "" {
		r.Modo = nueva.Modo
	}
	if len(nueva.Numeros) > 0 {
		r.Numeros = nueva.Numeros
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
		v := &vista{}
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
