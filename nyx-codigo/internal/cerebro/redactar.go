package cerebro

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
)

// Verification and wording policy (§4.14.3). Every answer goes through redactar before it is shown.

// Length limits of the bubble.
const (
	MaxPrimeraLinea = 160 // runes in Respuesta.Texto
	MaxParrafos     = 6
	MaxFilas        = 12 // rows of a table in the bubble (test and per-line tables are kept whole)
)

// Sentences of the evidence policy.
const (
	FraseSinEjemplos        = "No me diste ejemplos: comprobé que el código hace lo que entendí. Si no es eso, pulsa «No es eso»."
	FraseSinEjemplosNiAcaso = "No me diste ejemplos y no he podido comprobar qué debe dar. Si no es eso, pulsa «No es eso»."
)

// ordenNiveles is the fixed badge order; failing groups go first so they cannot be missed.
var ordenNiveles = map[nucleo.Nivel]int{
	nucleo.NivelFallo:        0,
	nucleo.NivelTusEjemplos:  1,
	nucleo.NivelEntendido:    2,
	nucleo.NivelPropiedades:  3,
	nucleo.NivelSinFallos:    4,
	nucleo.NivelCalculo:      5,
	nucleo.NivelFuente:       6,
	nucleo.NivelSinComprobar: 7,
}

// afirmar enforces the wording rule for claims about behaviour: a claim without a check, or whose check
// is sin_comprobar (or failed), starts with "Creo que".
func afirmar(texto string, comp *nucleo.Comprobacion) string {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return texto
	}
	if comp != nil && comp.Nivel != "" && comp.Nivel != nucleo.NivelSinComprobar && comp.Nivel != nucleo.NivelFallo {
		return texto
	}
	if strings.HasPrefix(nucleo.Normalizar(texto), "creo que") {
		return texto
	}
	if empiezaConNombre(texto) {
		return "Creo que " + texto
	}
	r, n := utf8.DecodeRuneInString(texto)
	return "Creo que " + strings.ToLower(string(r)) + texto[n:]
}

// empiezaConNombre: the first word looks like an identifier or proper name (has inner capitals or digits).
func empiezaConNombre(s string) bool {
	w := strings.Fields(s)
	if len(w) == 0 {
		return false
	}
	p := w[0]
	if p == "Go" {
		return true
	}
	for i, r := range p {
		if i > 0 && (r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '(') {
			return true
		}
	}
	return false
}

// textoBadge is the default badge text of a comprobación (the UI adds the ✔).
func textoBadge(c nucleo.Comprobacion) string {
	switch c.Nivel {
	case nucleo.NivelTusEjemplos:
		if c.Total == 1 {
			return "Cumple tu ejemplo"
		}
		return fmt.Sprintf("Cumple tus %d ejemplos", c.Total)
	case nucleo.NivelEntendido:
		return fmt.Sprintf("%d/%d según lo que entendí", c.Pasan, c.Total)
	case nucleo.NivelPropiedades:
		return fmt.Sprintf("Propiedades %d/%d", c.Pasan, c.Total)
	case nucleo.NivelSinFallos:
		return fmt.Sprintf("Sin errores en %d pruebas", c.Total)
	case nucleo.NivelCalculo:
		return "Comprobado por cálculo"
	case nucleo.NivelFuente:
		return "Según la documentación"
	case nucleo.NivelFallo:
		return "No lo he conseguido"
	}
	return "Sin comprobar"
}

// redactar applies the evidence and wording policy to an answer before it is shown.
func (c *Cerebro) redactar(k *tarea, r *nucleo.Respuesta) {
	r.Tarea = k.p.Tarea
	if r.Comprobaciones == nil {
		r.Comprobaciones = []nucleo.Comprobacion{}
	}
	for i := range r.Comprobaciones {
		if r.Comprobaciones[i].Nivel == "" {
			r.Comprobaciones[i].Nivel = nucleo.NivelSinComprobar // a check that does not say what it is counts as none
		}
		if strings.TrimSpace(r.Comprobaciones[i].Texto) == "" {
			r.Comprobaciones[i].Texto = textoBadge(r.Comprobaciones[i])
		}
	}
	sort.SliceStable(r.Comprobaciones, func(i, j int) bool {
		return rangoNivel(r.Comprobaciones[i].Nivel) < rangoNivel(r.Comprobaciones[j].Nivel)
	})
	r.Nivel = nivelHonesto(k, *r)
	if r.Nivel == nucleo.NivelFallo {
		r.Exito = false
	}
	if esCreacion(k.intencion) && r.Exito && r.Codigo != "" && !hayNivel(r.Comprobaciones, nucleo.NivelTusEjemplos) {
		frase := FraseSinEjemplosNiAcaso
		if hayNivel(r.Comprobaciones, nucleo.NivelEntendido) {
			frase = FraseSinEjemplos
		}
		if !contieneParrafo(r.Parrafos, frase) {
			r.Parrafos = append(r.Parrafos, frase)
		}
	}
	if len(r.Sugerencias) == 0 {
		r.Sugerencias = sugerencias(k, *r)
	}
	limitar(r)
}

func rangoNivel(n nucleo.Nivel) int {
	if v, ok := ordenNiveles[n]; ok {
		return v
	}
	return len(ordenNiveles)
}

func hayNivel(cs []nucleo.Comprobacion, n nucleo.Nivel) bool {
	for _, c := range cs {
		if c.Nivel == n {
			return true
		}
	}
	return false
}

func contieneParrafo(ps []string, s string) bool {
	for _, p := range ps {
		if p == s {
			return true
		}
	}
	return false
}

func esCreacion(i nucleo.TipoIntencion) bool {
	return i == nucleo.ICrearFuncion || i == nucleo.ICrearPrograma
}

// nivelHonesto is the overall level the evidence supports: code → pruebas.NivelGlobal of the
// comprobaciones; reasoning → the resolver's level if a comprobación backs it; docs → fuente when there
// are sources. A level no comprobación supports is lowered (never raised).
func nivelHonesto(k *tarea, r nucleo.Respuesta) nucleo.Nivel {
	cs := r.Comprobaciones
	global := pruebas.NivelGlobal(cs)
	if global == nucleo.NivelFallo {
		return nucleo.NivelFallo
	}
	switch r.Nivel {
	case "":
		if nucleo.Familia(k.intencion) == "programar" || len(cs) > 0 {
			return global
		}
		if len(r.Fuentes) > 0 {
			return nucleo.NivelFuente
		}
		return nucleo.NivelSinComprobar
	case nucleo.NivelFallo, nucleo.NivelSinComprobar:
		return r.Nivel
	case nucleo.NivelFuente:
		if len(r.Fuentes) > 0 || hayNivel(cs, nucleo.NivelFuente) {
			return r.Nivel
		}
		return global
	}
	if hayNivel(cs, r.Nivel) {
		if nucleo.Familia(k.intencion) == "programar" && r.Codigo != "" {
			return global // the code's level is the global one (a failing group already returned fallo)
		}
		return r.Nivel
	}
	return global
}

// sugerencias are the default follow-up chips (§4.14.3).
func sugerencias(k *tarea, r nucleo.Respuesta) []string {
	if !r.Exito {
		return nil
	}
	if r.Codigo != "" && nucleo.Familia(k.intencion) == "programar" {
		if k.intencion == nucleo.ICrearPrograma {
			return []string{"Explícalo", "Ponle comentarios"}
		}
		return []string{"Pruébala con otros datos", "Explícala", "Hazla programa", "Ponle comentarios"}
	}
	if k.intencion == nucleo.IEcuacion {
		var out []string
		if k.q != nil && len(k.q.Numeros) > 0 {
			ult := k.q.Numeros[len(k.q.Numeros)-1]
			if ult.Valor != nil && ult.Valor.IsInt() {
				n := ult.Valor.Num().Int64() + 2
				out = append(out, fmt.Sprintf("¿Y si fuera %d?", n))
			}
		}
		return append(out, "¿Por qué?")
	}
	if len(r.Justificacion) > 0 || len(r.Plan) > 0 {
		return []string{"¿Por qué?"}
	}
	return nil
}

// limitar enforces the length limits: first line ≤ 160 runes (the rest becomes the first paragraph),
// at most 6 paragraphs (extra ones are joined into the last), at most 12 rows per table in the bubble
// (tables of test cases and per-line explanations are kept whole: the Pruebas tab and the editor
// gutter use them).
func limitar(r *nucleo.Respuesta) {
	texto := strings.TrimSpace(r.Texto)
	resto := ""
	if i := strings.IndexByte(texto, '\n'); i >= 0 {
		texto, resto = strings.TrimSpace(texto[:i]), strings.TrimSpace(texto[i+1:])
	}
	if utf8.RuneCountInString(texto) > MaxPrimeraLinea {
		runas := []rune(texto)
		corte := MaxPrimeraLinea - 1
		for j := corte; j > MaxPrimeraLinea/2; j-- {
			if runas[j] == ' ' {
				corte = j
				break
			}
		}
		resto = strings.TrimSpace("…" + strings.TrimSpace(string(runas[corte:])) + "\n" + resto)
		texto = strings.TrimSpace(string(runas[:corte])) + "…"
	}
	r.Texto = texto
	if resto != "" {
		r.Parrafos = append([]string{resto}, r.Parrafos...)
	}
	if len(r.Parrafos) > MaxParrafos {
		ultimo := strings.Join(r.Parrafos[MaxParrafos-1:], "\n")
		r.Parrafos = append(r.Parrafos[:MaxParrafos-1:MaxParrafos-1], ultimo)
	}
	for i := range r.Tablas {
		tb := &r.Tablas[i]
		if len(tb.Filas) <= MaxFilas || tablaCompleta(*tb) {
			continue
		}
		total := len(tb.Filas)
		tb.Filas = tb.Filas[:MaxFilas]
		if len(tb.Marcas) > MaxFilas {
			tb.Marcas = tb.Marcas[:MaxFilas]
		}
		nota := fmt.Sprintf("(%d de %d filas)", MaxFilas, total)
		if tb.Titulo == "" {
			tb.Titulo = nota
		} else {
			tb.Titulo += " " + nota
		}
	}
}

// tablaCompleta: tables the UI reads in full (test cases; per-line explanation).
func tablaCompleta(tb nucleo.Tabla) bool {
	tiene := func(s string) bool {
		for _, h := range tb.Cabecera {
			if nucleo.Normalizar(h) == nucleo.Normalizar(s) {
				return true
			}
		}
		return false
	}
	if len(tb.Cabecera) > 0 && nucleo.Normalizar(tb.Cabecera[0]) == "linea" {
		return true
	}
	return tiene("Entrada") && (tiene("Obtenido") || tiene("Esperado"))
}
