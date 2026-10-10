package logica

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Resolutores returns the logic resolvers: "logica.proposicional", "logica.silogismo",
// "logica.hechos" and "logica.caballeros" (N2). b and g may be nil (then facts cannot be used or saved).
func Resolutores(b BaseHechos, g Guardar, lem nucleo.Lematizador) []nucleo.Resolutor {
	return []nucleo.Resolutor{
		&resProposicional{lem: lem},
		&resSilogismo{lem: lem},
		&resHechos{b: b, g: g, lem: lem},
		&resCaballeros{},
	}
}

// --- recognition features (cheap: one tokenization, no I/O)

type rasgos struct {
	palabras    map[string]bool
	seq         string // " w1 w2 … " normalized words and symbols
	frases      []frase
	pregunta    bool
	conclusion  bool
	simbolos    bool
	categoricas int
	recordar    bool
}

var iniciosCategorica = map[string]bool{
	"todos": true, "todas": true, "todo": true, "toda": true, "cada": true, "ningun": true, "ninguna": true,
	"ninguno": true, "algunos": true, "algunas": true, "algun": true, "alguna": true,
}

func analizarRasgos(texto string) rasgos {
	r := rasgos{palabras: map[string]bool{}}
	ts := tokenizar(texto)
	var sb strings.Builder
	sb.WriteByte(' ')
	for _, t := range ts {
		r.palabras[t.norma] = true
		sb.WriteString(t.norma)
		sb.WriteByte(' ')
		switch t.norma {
		case "->", "<->", "&", "|", "!":
			r.simbolos = true
		}
	}
	r.seq = sb.String()
	r.frases = partirFrases(ts)
	for _, f := range r.frases {
		r.pregunta = r.pregunta || f.pregunta
		r.conclusion = r.conclusion || f.conclusion
		var ws []tok
		for _, t := range f.toks {
			if !t.sim {
				ws = append(ws, t)
			}
		}
		if len(ws) > 0 {
			if iniciosCategorica[ws[0].norma] || (len(ws) > 1 && ws[0].norma == "no" && (ws[1].norma == "todos" || ws[1].norma == "hay")) {
				r.categoricas++
			}
		}
		if _, ok := quitarPrefijo(f.toks, prefijosRecordar[:9]); ok {
			r.recordar = true
		}
	}
	return r
}

func (r rasgos) tiene(frases ...string) bool {
	for _, f := range frases {
		if strings.Contains(r.seq, " "+f+" ") {
			return true
		}
	}
	return false
}

func respuestaCalculo(texto, metodo string, comp nucleo.Comprobacion) nucleo.Respuesta {
	comp.Nivel = nucleo.NivelCalculo
	if comp.Pasan == 0 && comp.Total == 0 {
		comp.Pasan, comp.Total = 1, 1
	}
	return nucleo.Respuesta{
		Texto:          texto,
		Metodo:         metodo,
		Comprobaciones: []nucleo.Comprobacion{comp},
		Nivel:          nucleo.NivelCalculo,
		Exito:          true,
	}
}

// --- logica.proposicional

type resProposicional struct{ lem nucleo.Lematizador }

func (*resProposicional) Nombre() string { return "logica.proposicional" }

func (*resProposicional) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	r := analizarRasgos(p.Texto)
	rec := nucleo.Reconocimiento{Intencion: nucleo.ILogica, Lectura: "argumento de lógica proposicional"}
	if EsCaballeros(p.Texto) {
		return rec
	}
	if _, _, ok := preguntaTautologia(p.Texto); ok {
		rec.Puntos, rec.Lectura = 0.9, "¿es siempre verdad esta fórmula?"
		return rec
	}
	switch {
	case r.simbolos:
		rec.Puntos = 0.85
	case (r.palabras["si"] || r.palabras["cuando"]) && (r.pregunta || r.conclusion) && len(r.frases) >= 2:
		rec.Puntos = 0.85
	case (r.palabras["o"] || r.palabras["ni"] || r.palabras["no"] || r.palabras["y"]) && (r.pregunta || r.conclusion) && len(r.frases) >= 2:
		rec.Puntos = 0.5
	case r.palabras["si"] && len(r.frases) >= 2:
		rec.Puntos = 0.5
	}
	if r.categoricas > 0 && !r.palabras["si"] && !r.simbolos {
		rec.Puntos = min(rec.Puntos, 0.3)
	}
	if r.recordar {
		rec.Puntos = min(rec.Puntos, 0.1)
	}
	return rec
}

func (rp *resProposicional) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if formula, modo, ok := preguntaTautologia(p.Texto); ok {
		return rp.tautologia(ctx, formula, modo, n)
	}
	paso := n.Sub(nucleo.PasoEntender, "Leo las premisas y la conclusión")
	arg, err := AnalizarArgumento(p.Texto, rp.lem)
	if err != nil {
		paso.Mal("No sé plantearlo como un argumento")
		return nucleo.Respuesta{}, err
	}
	nAt := len(arg.Atomos.Frases)
	nombres := arg.Atomos.Nombres(nAt)
	var leyenda []string
	for i := 0; i < nAt; i++ {
		leyenda = append(leyenda, fmt.Sprintf("%s = «%s»", nombres[i], arg.Atomos.Frases[i]))
	}
	for _, nota := range arg.Notas {
		paso.Nota("%s", nota)
	}
	var premTxt []string
	for _, f := range arg.Premisas {
		premTxt = append(premTxt, Mostrar(f, arg.Atomos))
	}
	entendi := "Así lo entendí: " + strings.Join(premTxt, "; ")
	if arg.Conclusion != nil {
		entendi += ". ¿Se sigue " + Mostrar(arg.Conclusion, arg.Atomos) + "?"
	} else {
		entendi += ". ¿Qué se sigue?"
	}
	paso.Detalle("%s", strings.Join(leyenda, ", "))
	paso.Bien("%s", entendi)

	res := nucleo.Respuesta{Entendi: entendi}
	res.Parrafos = append(res.Parrafos, "Átomos: "+strings.Join(leyenda, ", ")+".")
	res.Parrafos = append(res.Parrafos, arg.Notas...)

	comp := nucleo.Comprobacion{Texto: "Comprobado con DPLL", Comando: "Tseitin + DPLL"}
	if nAt <= maxAtomosTabla {
		comp.Texto = fmt.Sprintf("Comprobado con DPLL y con la tabla de verdad (%d filas)", 1<<nAt)
	}
	sat := n.Sub(nucleo.PasoIntento, "Busco con DPLL un caso con las premisas verdaderas")
	_, consistentes, err := Satisfacible(ctx, arg.Premisas, nAt)
	if err != nil {
		sat.Mal("DPLL no terminó")
		return nucleo.Respuesta{}, err
	}
	if !consistentes {
		sat.Mal("Las premisas se contradicen")
		out := respuestaCalculo("Las premisas se contradicen entre sí: nunca pueden ser verdad a la vez, así que de ellas se sigue cualquier cosa.", "dpll", comp)
		out.Entendi, out.Parrafos = res.Entendi, res.Parrafos
		return out, nil
	}
	if arg.Conclusion == nil {
		sat.Info("Las premisas pueden ser verdad a la vez")
		var sigue []string
		for i := 0; i < nAt; i++ {
			for _, lit := range []Formula{Atomo{i}, No{Atomo{i}}} {
				esPremisa := false
				for _, f := range arg.Premisas {
					esPremisa = esPremisa || Igual(f, lit)
				}
				if esPremisa {
					continue
				}
				v, _, err := Implicacion(ctx, arg.Premisas, lit, nAt)
				if err != nil {
					return nucleo.Respuesta{}, err
				}
				if v {
					sigue = append(sigue, Mostrar(lit, arg.Atomos))
				}
			}
		}
		texto := "De estas premisas no se sigue nada seguro sobre cada frase por separado."
		if len(sigue) > 0 {
			texto = "De las premisas se sigue: " + strings.Join(sigue, "; ") + "."
		}
		out := respuestaCalculo(texto, "dpll", comp)
		out.Entendi, out.Parrafos = res.Entendi, res.Parrafos
		return out, nil
	}
	valida, contra, err := Implicacion(ctx, arg.Premisas, arg.Conclusion, nAt)
	if err != nil {
		sat.Mal("No pude decidirlo")
		return nucleo.Respuesta{}, err
	}
	concl := Mostrar(arg.Conclusion, arg.Atomos)
	usados := atomosUsados(append(append([]Formula(nil), arg.Premisas...), arg.Conclusion)...)
	if len(usados) <= 5 {
		tb, contras, err := tablaArgumento(arg.Premisas, arg.Conclusion, arg.Atomos)
		if err == nil {
			if (contras == 0) != valida {
				return nucleo.Respuesta{}, ErrNoCoinciden
			}
			tb.Titulo = strings.Replace(tb.Titulo, "Tabla de verdad", "Tabla de verdad (premisas y conclusión)", 1)
			res.Tablas = append(res.Tablas, tb)
			c := n.Sub(nucleo.PasoComprobar, "Tabla de verdad: %d filas, %d contraejemplos", len(tb.Filas), contras)
			c.Tabla(tb)
			c.Bien("")
		}
	}
	var out nucleo.Respuesta
	if valida {
		sat.Bien("No hay ningún caso con las premisas verdaderas y la conclusión falsa")
		regla := NombreRegla(arg.Premisas, arg.Conclusion)
		texto := "Sí, se sigue: " + concl + "."
		if regla != "" {
			texto = fmt.Sprintf("Sí, se sigue: %s (es un %s).", concl, regla)
			if regla == "De Morgan" || strings.HasPrefix(regla, "simplif") || strings.HasPrefix(regla, "conjun") || strings.HasPrefix(regla, "adic") || strings.HasPrefix(regla, "doble") {
				texto = fmt.Sprintf("Sí, se sigue: %s (por %s).", concl, regla)
			}
		}
		out = respuestaCalculo(texto, "dpll", comp)
		if regla != "" {
			out.Parrafos = append(out.Parrafos, ExplicarRegla(regla))
		}
		if prueba, ok := resolucionCon(arg.Premisas, arg.Conclusion, nombres); ok {
			out.Justificacion = append([]string{"Prueba por resolución (niego la conclusión y llego a una contradicción):"}, prueba...)
			c := n.Sub(nucleo.PasoComprobar, "Prueba por resolución en %d pasos", len(prueba))
			c.Detalle("%s", strings.Join(prueba, "\n"))
			c.Bien("")
		}
	} else {
		desc := DescribirModelo(contra, arg.Atomos, usados)
		sat.Mal("Hay un caso con las premisas verdaderas y la conclusión falsa")
		opuesta, _, err := Implicacion(ctx, arg.Premisas, negar(arg.Conclusion), nAt)
		if err != nil {
			return nucleo.Respuesta{}, err
		}
		var texto string
		if opuesta {
			texto = fmt.Sprintf("No: de las premisas se sigue lo contrario, %s.", Mostrar(negar(arg.Conclusion), arg.Atomos))
		} else {
			texto = fmt.Sprintf("No se sigue: %s no se deduce de las premisas.", concl)
		}
		falacia := Falacia(arg.Premisas, arg.Conclusion)
		if falacia != "" {
			texto += " Es la falacia de " + falacia + "."
		}
		out = respuestaCalculo(texto, "dpll", comp)
		out.Parrafos = append(out.Parrafos, "Contraejemplo: si "+desc+", las premisas son verdad y la conclusión es falsa.")
		if falacia != "" {
			out.Parrafos = append(out.Parrafos, ExplicarRegla(falacia))
		}
	}
	out.Entendi = res.Entendi
	out.Parrafos = append(res.Parrafos, out.Parrafos...)
	out.Tablas = res.Tablas
	return out, nil
}

// preguntaTautologia recognizes "¿es «p o no p» siempre verdad?", "¿es X una contradicción?", "¿puede
// ser verdad X?", "tabla de verdad de X". It returns the formula text and the mode.
func preguntaTautologia(texto string) (string, string, bool) {
	ts := tokenizar(texto)
	var ws []tok
	for _, t := range ts {
		switch t.norma {
		case "¿", "?", ".", ";":
			continue
		}
		ws = append(ws, t)
	}
	claves := []struct {
		seq  []string
		modo string
	}{
		{[]string{"siempre", "es", "verdad"}, "tautologia"}, {[]string{"siempre", "verdad"}, "tautologia"},
		{[]string{"siempre", "cierto"}, "tautologia"}, {[]string{"siempre", "verdadero"}, "tautologia"},
		{[]string{"siempre", "verdadera"}, "tautologia"}, {[]string{"una", "tautologia"}, "tautologia"},
		{[]string{"tautologia"}, "tautologia"}, {[]string{"una", "contradiccion"}, "contradiccion"},
		{[]string{"contradiccion"}, "contradiccion"}, {[]string{"siempre", "falso"}, "contradiccion"},
		{[]string{"siempre", "falsa"}, "contradiccion"}, {[]string{"nunca", "es", "verdad"}, "contradiccion"},
		{[]string{"satisfacible"}, "satisfacible"}, {[]string{"puede", "ser", "verdad"}, "satisfacible"},
		{[]string{"puede", "ser", "cierto"}, "satisfacible"}, {[]string{"tabla", "de", "verdad"}, "tabla"},
	}
	ini, modo, largo := -1, "", 0
	for _, c := range claves {
		for i := range ws {
			if empiezaPor(ws[i:], c.seq) {
				ini, modo, largo = i, c.modo, len(c.seq)
				break
			}
		}
		if ini >= 0 {
			break
		}
	}
	if ini < 0 {
		return "", "", false
	}
	// a quoted formula wins
	for i, t := range ws {
		if t.norma == "\"" {
			for j := i + 1; j < len(ws); j++ {
				if ws[j].norma == "\"" {
					if j > i+1 {
						return texto[ws[i+1].desde:ws[j].desde], modo, true
					}
					break
				}
			}
			break
		}
	}
	antes := ws[:ini]
	for len(antes) > 0 && (antes[0].norma == "es" || antes[0].norma == "sera" || antes[0].norma == "la" || antes[0].norma == "formula") {
		antes = antes[1:]
	}
	for len(antes) > 0 && (antes[len(antes)-1].norma == "es" || antes[len(antes)-1].norma == "una") {
		antes = antes[:len(antes)-1]
	}
	partes := antes
	if len(partes) == 0 {
		partes = ws[ini+largo:]
		for len(partes) > 0 && (partes[0].norma == "de" || partes[0].norma == ":" || partes[0].norma == "la" || partes[0].norma == "formula") {
			partes = partes[1:]
		}
	}
	if len(partes) == 0 {
		return "", "", false
	}
	return texto[partes[0].desde : partes[len(partes)-1].desde+len(partes[len(partes)-1].orig)], modo, true
}

func (rp *resProposicional) tautologia(ctx context.Context, formula, modo string, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	paso := n.Sub(nucleo.PasoEntender, "Leo la fórmula «%s»", strings.TrimSpace(formula))
	f, at, notas, err := AnalizarFormula(formula, nil, rp.lem)
	if err != nil {
		paso.Mal("No entiendo la fórmula")
		return nucleo.Respuesta{}, err
	}
	for _, nota := range notas {
		paso.Nota("%s", nota)
	}
	nAt := len(at.Frases)
	nombres := at.Nombres(nAt)
	simb := EscribirCon(f, nombres)
	paso.Bien("Fórmula: %s", simb)
	texto := "«" + strings.TrimSpace(formula) + "»"
	comp := nucleo.Comprobacion{Texto: "Comprobado con DPLL", Comando: "Tseitin + DPLL"}
	var tb *nucleo.Tabla
	if nAt <= maxAtomosTabla {
		t, err := TablaVerdad([]Formula{f}, at, maxAtomosTabla)
		if err == nil {
			tb = &t
			comp.Texto = fmt.Sprintf("Comprobado con DPLL y con la tabla de verdad (%d filas)", len(t.Filas))
		}
	}
	verdaderas := -1
	if tb != nil {
		verdaderas = 0
		for _, fila := range tb.Filas {
			if fila[len(fila)-1] == "V" {
				verdaderas++
			}
		}
	}
	usados := atomosUsados(f)
	var out nucleo.Respuesta
	switch modo {
	case "contradiccion":
		siempreFalsa, modelo := Tautologia(No{f}, nAt)
		if verdaderas >= 0 && siempreFalsa != (verdaderas == 0) {
			return nucleo.Respuesta{}, ErrNoCoinciden
		}
		if siempreFalsa {
			out = respuestaCalculo(fmt.Sprintf("Sí, %s es una contradicción: nunca es verdad.", texto), "dpll", comp)
		} else {
			out = respuestaCalculo(fmt.Sprintf("No, %s no es una contradicción: es verdad cuando %s.", texto, DescribirModelo(modelo, at, usados)), "dpll", comp)
		}
	case "satisfacible":
		modelo, sat, err := Satisfacible(ctx, []Formula{f}, nAt)
		if err != nil {
			return nucleo.Respuesta{}, err
		}
		if verdaderas >= 0 && sat != (verdaderas > 0) {
			return nucleo.Respuesta{}, ErrNoCoinciden
		}
		if sat {
			out = respuestaCalculo(fmt.Sprintf("Sí, %s puede ser verdad: por ejemplo, cuando %s.", texto, DescribirModelo(modelo, at, usados)), "dpll", comp)
		} else {
			out = respuestaCalculo(fmt.Sprintf("No, %s nunca puede ser verdad: es una contradicción.", texto), "dpll", comp)
		}
	default:
		taut, contra := Tautologia(f, nAt)
		if verdaderas >= 0 && taut != (verdaderas == len(tb.Filas)) {
			return nucleo.Respuesta{}, ErrNoCoinciden
		}
		switch {
		case taut:
			out = respuestaCalculo(fmt.Sprintf("Sí, %s es siempre verdad: es una tautología.", texto), "dpll", comp)
		case contra != nil:
			out = respuestaCalculo(fmt.Sprintf("No, %s no es siempre verdad: es falso cuando %s.", texto, DescribirModelo(contra, at, usados)), "dpll", comp)
		default:
			out = respuestaCalculo(fmt.Sprintf("No, %s no es siempre verdad.", texto), "dpll", comp)
		}
		if modo == "tabla" {
			out.Texto = "Esta es la tabla de verdad de " + texto + ". " + out.Texto
		}
	}
	if tb != nil && (len(usados) <= 5 || modo == "tabla") {
		out.Tablas = append(out.Tablas, *tb)
		c := n.Sub(nucleo.PasoComprobar, "Tabla de verdad: %d de %d filas verdaderas", verdaderas, len(tb.Filas))
		c.Tabla(*tb)
		c.Bien("")
	}
	out.Entendi = "Así lo entendí: " + simb
	if ley := at.Leyenda(usados, nombres); ley != "" {
		out.Entendi += " (" + ley + ")"
	}
	return out, nil
}

// --- logica.silogismo

type resSilogismo struct{ lem nucleo.Lematizador }

func (*resSilogismo) Nombre() string { return "logica.silogismo" }

func (*resSilogismo) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	r := analizarRasgos(p.Texto)
	rec := nucleo.Reconocimiento{Intencion: nucleo.ISilogismo, Lectura: "silogismo (todos, algunos, ningún)"}
	if r.recordar || EsCaballeros(p.Texto) {
		return rec
	}
	if r.categoricas >= 1 && len(r.frases) >= 2 {
		rec.Puntos = 0.6
		if r.pregunta || r.conclusion {
			rec.Puntos = 0.85
		}
		if r.palabras["si"] || r.simbolos {
			rec.Puntos = 0.4
		}
	} else if r.categoricas >= 1 && r.tiene("se concluye", "se deduce", "se sigue") {
		rec.Puntos = 0.7
	}
	return rec
}

// leerSilogismo splits the text into categorical premises and an optional conclusion.
func leerSilogismo(texto string, lem nucleo.Lematizador) ([]Categorica, *Categorica, bool, error) {
	var prem []Categorica
	var concl *Categorica
	queConcluye := false
	for _, f := range partirFrases(tokenizar(texto)) {
		if esQueConcluye(f) {
			queConcluye = true
			continue
		}
		// a sentence may hold several premises separated by commas or "y"
		var cats []Categorica
		if c, err := AnalizarCategorica(fraseTexto(texto, f.toks), lem); err == nil {
			cats = []Categorica{c}
		} else {
			for _, parte := range partirNivel0(f.toks, map[string]bool{",": true, "y": true}) {
				parte = recortar(parte)
				if len(parte) == 0 {
					continue
				}
				c, err := AnalizarCategorica(fraseTexto(texto, parte), lem)
				if err != nil {
					return nil, nil, false, err
				}
				cats = append(cats, c)
			}
		}
		if f.pregunta || f.conclusion {
			if len(cats) != 1 {
				return nil, nil, false, fmt.Errorf("logica: la conclusión debe ser una sola frase: %w", nucleo.ErrNoEntiendo)
			}
			c := cats[0]
			concl = &c
			continue
		}
		prem = append(prem, cats...)
	}
	if len(prem) == 0 {
		return nil, nil, false, fmt.Errorf("logica: no encuentro premisas: %w", nucleo.ErrNoEntiendo)
	}
	return prem, concl, queConcluye, nil
}

// fraseTexto recovers the original text of some tokens.
func fraseTexto(texto string, ts []tok) string {
	if len(ts) == 0 {
		return ""
	}
	ult := ts[len(ts)-1]
	return texto[ts[0].desde : ult.desde+len(ult.orig)]
}

func (rs *resSilogismo) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	paso := n.Sub(nucleo.PasoEntender, "Leo las frases con «todos», «algunos» y «ningún»")
	prem, concl, _, err := leerSilogismo(p.Texto, rs.lem)
	if err != nil {
		paso.Mal("No sé plantearlo como un silogismo")
		return nucleo.Respuesta{}, err
	}
	var premTxt []string
	for _, c := range prem {
		premTxt = append(premTxt, c.String())
	}
	entendi := "Así lo entendí: " + strings.Join(premTxt, "; ")
	if concl != nil {
		entendi += ". ¿" + mayuscula(concl.String()) + "?"
	} else {
		entendi += ". ¿Qué se concluye?"
	}
	paso.Bien("%s", entendi)
	venn := n.Sub(nucleo.PasoIntento, "Razono con un diagrama de Venn y DPLL")
	r, err := Silogismo(ctx, prem, concl)
	if err != nil {
		venn.Mal("No pude decidirlo")
		return nucleo.Respuesta{}, err
	}
	venn.Detalle("%s", strings.Join(r.Pasos, "\n"))
	terminos := map[string]bool{}
	for _, c := range append(append([]Categorica(nil), prem...), derefs(concl)...) {
		terminos[claveTermino(c.Sujeto)] = true
		terminos[claveTermino(c.Predicado)] = true
	}
	comp := nucleo.Comprobacion{Texto: "Comprobado con DPLL sobre las zonas de un diagrama de Venn", Comando: "Venn + DPLL"}
	if len(terminos) <= 4 {
		comp.Texto += " y probando todas las combinaciones"
	}
	var out nucleo.Respuesta
	switch {
	case concl == nil:
		venn.Info("Busco qué se sigue")
		texto := "No se sigue ninguna frase nueva que relacione estos términos."
		if len(r.Conclusiones) > 0 {
			texto = "Se sigue: " + strings.Join(r.Conclusiones, "; ") + "."
		}
		out = respuestaCalculo(texto, "venn", comp)
	case r.Valido:
		venn.Bien("La conclusión se sigue")
		texto := "Sí, se sigue: " + concl.String() + "."
		if concl.Cuant == Individuo {
			texto = "Sí: " + concl.String() + "."
		}
		if r.Nombre != "" {
			texto = strings.TrimSuffix(texto, ".") + " (es un silogismo en " + r.Nombre + ")."
		}
		out = respuestaCalculo(texto, "venn", comp)
	default:
		venn.Mal("La conclusión no se sigue")
		texto := "No se sigue: «" + concl.String() + "» no se deduce de las premisas."
		out = respuestaCalculo(texto, "venn", comp)
		if r.Contraejemplo != "" {
			out.Parrafos = append(out.Parrafos, "Contraejemplo: "+r.Contraejemplo+". Así las premisas son verdad y la conclusión es falsa.")
		}
	}
	out.Entendi = entendi
	out.Justificacion = r.Pasos
	return out, nil
}

func derefs(c *Categorica) []Categorica {
	if c == nil {
		return nil
	}
	return []Categorica{*c}
}

func mayuscula(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// --- logica.hechos

type resHechos struct {
	b   BaseHechos
	g   Guardar
	lem nucleo.Lematizador
}

func (*resHechos) Nombre() string { return "logica.hechos" }

func (rh *resHechos) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	r := analizarRasgos(p.Texto)
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPreguntarHecho, Lectura: "pregunta sobre lo que sé"}
	if EsCaballeros(p.Texto) {
		return rec
	}
	if r.recordar || (r.tiene("significa", "quiere decir") && !r.pregunta) {
		rec.Intencion, rec.Lectura = nucleo.IRecordar, "recordar un hecho o una regla"
		rec.Puntos = 0.9
		if rh.g == nil {
			rec.Puntos = 0.3
		}
		return rec
	}
	if r.pregunta && len(r.frases) == 1 && !r.palabras["si"] && !r.palabras["o"] && !r.simbolos && (r.palabras["es"] || r.palabras["tiene"] || r.palabras["significa"]) {
		if rh.b == nil {
			rec.Puntos = 0.2
			return rec
		}
		rec.Puntos = 0.6
		if h, _, err := LeerHecho(p.Texto, rh.lem); err == nil && h != nil && len(rh.b.Hechos(h.Sujeto)) > 0 {
			rec.Puntos = 0.85
		}
		return rec
	}
	if r.pregunta && len(r.frases) >= 2 && !r.palabras["si"] && !r.simbolos {
		rec.Puntos = 0.45
	}
	return rec
}

func (rh *resHechos) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	fs := partirFrases(tokenizar(p.Texto))
	if len(fs) == 0 {
		return nucleo.Respuesta{}, fmt.Errorf("logica: texto vacío: %w", nucleo.ErrNoEntiendo)
	}
	var pregunta *frase
	var hechos []nucleo.Hecho
	var reglas []nucleo.Regla
	recordar := false
	_, recordar = QuitarRecordar(p.Texto)
	paso := n.Sub(nucleo.PasoEntender, "Leo los hechos y las reglas")
	for i := range fs {
		f := fs[i]
		if f.pregunta {
			pregunta = &fs[i]
			continue
		}
		h, r, err := LeerHecho(fraseTexto(p.Texto, f.toks), rh.lem)
		if err != nil {
			paso.Mal("No entiendo «%s»", fraseTexto(p.Texto, f.toks))
			return nucleo.Respuesta{}, err
		}
		if h != nil {
			hechos = append(hechos, *h)
			paso.Nota("Hecho: %s", DescribirHecho(*h))
		}
		if r != nil {
			reglas = append(reglas, *r)
			paso.Nota("Regla: %s", DescribirRegla(*r))
		}
	}
	if pregunta == nil {
		if !recordar && !strings.Contains(nucleo.Normalizar(p.Texto), "significa") {
			paso.Mal("No hay nada que recordar ni preguntar")
			return nucleo.Respuesta{}, fmt.Errorf("logica: no hay pregunta: %w", nucleo.ErrNoEntiendo)
		}
		return rh.guardar(hechos, reglas, paso)
	}
	paso.Bien("Entendido")
	q, r, err := LeerHecho(fraseTexto(p.Texto, pregunta.toks), rh.lem)
	if err != nil || q == nil {
		if err == nil && r != nil {
			err = fmt.Errorf("logica: la pregunta es una regla general; eso lo razono como silogismo: %w", nucleo.ErrNoSoportado)
		}
		return nucleo.Respuesta{}, err
	}
	base := baseUnida{b: rh.b, h: numerar(hechos, "t"), r: numerarReglas(reglas)}
	consulta := n.Sub(nucleo.PasoIntento, "Encadeno hechos y reglas hasta llegar a «%s»", DescribirHecho(*q))
	resp, cadena := Consultar(ctx, base, *q)
	preguntaTxt := strings.TrimSpace(fraseTexto(p.Texto, pregunta.toks))
	afirmacion := mayuscula(preguntaTxt)
	switch resp {
	case "si", "no":
		ders := Encadenar(ctx, base, maxIterEncadenar)
		if !comprobarCadena(ctx, base, ders) {
			consulta.Mal("La cadena no se sostiene")
			return nucleo.Respuesta{}, errors.New("logica: la cadena de deducción no se pudo comprobar (error interno)")
		}
		consulta.Bien("Llego en %d pasos", len(cadena))
		consulta.Detalle("%s", strings.Join(cadena, "\n"))
		var texto string
		if resp == "si" {
			texto = "Sí: " + afirmacion + "."
		} else {
			texto = "No: " + DescribirHecho(negado(*q)) + "."
		}
		comp := nucleo.Comprobacion{Texto: fmt.Sprintf("Deducido de lo que sé, comprobando cada paso (%d)", len(cadena)), Comando: "encadenamiento hacia delante"}
		out := respuestaCalculo(texto, "encadenamiento", comp)
		out.Parrafos = []string{"Lo sé porque: " + strings.Join(cadena, "; ") + "."}
		out.Justificacion = cadena
		out.Entendi = "Así lo entendí: ¿" + DescribirHecho(*q) + "?"
		return out, nil
	}
	consulta.Info("No llego a ninguna conclusión")
	return nucleo.Respuesta{
		Texto:          "No lo sé: con lo que sé no puedo deducir si " + DescribirHecho(*q) + " ni lo contrario.",
		Entendi:        "Así lo entendí: ¿" + DescribirHecho(*q) + "?",
		Metodo:         "encadenamiento",
		Nivel:          nucleo.NivelSinComprobar,
		Exito:          false,
		Parrafos:       []string{"Puedes enseñármelo con «recuerda que …»."},
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelSinComprobar, Texto: "Sin datos suficientes"}},
	}, nil
}

func negado(h nucleo.Hecho) nucleo.Hecho {
	h.Negado = !h.Negado
	return h
}

func numerar(hs []nucleo.Hecho, prefijo string) []nucleo.Hecho {
	out := append([]nucleo.Hecho(nil), hs...)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = fmt.Sprintf("%s%d", prefijo, i+1)
		}
	}
	return out
}

func numerarReglas(rs []nucleo.Regla) []nucleo.Regla {
	out := append([]nucleo.Regla(nil), rs...)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = fmt.Sprintf("tr%d", i+1)
		}
	}
	return out
}

func (rh *resHechos) guardar(hechos []nucleo.Hecho, reglas []nucleo.Regla, paso *nucleo.Nodo) (nucleo.Respuesta, error) {
	if rh.g == nil {
		paso.Mal("No tengo memoria donde guardarlo")
		return nucleo.Respuesta{}, fmt.Errorf("logica: no tengo memoria para hechos: %w", nucleo.ErrNoSoportado)
	}
	var dichos []string
	for _, h := range hechos {
		if _, err := rh.g.GuardarHecho(h); err != nil {
			paso.Mal("No pude guardarlo")
			return nucleo.Respuesta{}, err
		}
		dichos = append(dichos, h.Texto)
	}
	for _, r := range reglas {
		if _, err := rh.g.GuardarRegla(r); err != nil {
			paso.Mal("No pude guardarlo")
			return nucleo.Respuesta{}, err
		}
		dichos = append(dichos, DescribirRegla(r))
	}
	if len(dichos) == 0 {
		paso.Mal("No hay nada que recordar")
		return nucleo.Respuesta{}, fmt.Errorf("logica: no hay nada que recordar: %w", nucleo.ErrNoEntiendo)
	}
	paso.Bien("Lo guardo en mi memoria")
	return nucleo.Respuesta{
		Texto:     "Lo recuerdo: " + strings.Join(dichos, "; ") + ".",
		Aprendido: "Lo he guardado en mi memoria: " + strings.Join(dichos, "; "),
		Metodo:    "hechos",
		Nivel:     nucleo.NivelEntendido,
		Exito:     true,
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelEntendido, Pasan: len(dichos), Total: len(dichos),
			Texto: "Guardado en mi memoria"}},
	}, nil
}

// --- logica.caballeros (N2)

type resCaballeros struct{}

func (*resCaballeros) Nombre() string { return "logica.caballeros" }

func (*resCaballeros) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.ILogica, Lectura: "caballeros (dicen la verdad) y escuderos (mienten)"}
	if EsCaballeros(p.Texto) {
		rec.Puntos = 0.9
	}
	return rec
}

func (*resCaballeros) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	paso := n.Sub(nucleo.PasoEntender, "Leo quién dice qué")
	r, err := Caballeros(ctx, p.Texto)
	if err != nil {
		paso.Mal("No entiendo el enunciado")
		return nucleo.Respuesta{}, err
	}
	nombres := r.Personas
	var lineas []string
	for i, d := range r.Dichos {
		lineas = append(lineas, r.Textos[i]+" → "+EscribirCon(d, nombres))
	}
	paso.Detalle("%s", strings.Join(lineas, "\n"))
	paso.Bien("Cada uno dice la verdad si y solo si es caballero")
	nP := len(nombres)
	busca := n.Sub(nucleo.PasoIntento, "Pruebo las %d combinaciones posibles", 1<<nP)
	tb := nucleo.Tabla{Titulo: "Combinaciones (V = caballero, F = escudero)", Cabecera: append(append([]string(nil), nombres...), "¿cumple?")}
	if nP <= 4 {
		v := make([]bool, nP)
		for mask := 0; mask < 1<<nP; mask++ {
			var fila []string
			for i := 0; i < nP; i++ {
				v[i] = mask&(1<<(nP-1-i)) == 0
				fila = append(fila, vf(v[i]))
			}
			ok := true
			for _, d := range r.Dichos {
				ok = ok && Evaluar(d, v)
			}
			marca := "mal"
			cumple := "no"
			if ok {
				marca, cumple = "bien", "sí"
			}
			tb.Filas = append(tb.Filas, append(fila, cumple))
			tb.Marcas = append(tb.Marcas, marca)
		}
		busca.Tabla(tb)
	}
	comp := nucleo.Comprobacion{Texto: fmt.Sprintf("Comprobé las %d combinaciones y lo confirmé con DPLL", 1<<nP), Comando: "enumeración + DPLL"}
	var out nucleo.Respuesta
	switch len(r.Modelos) {
	case 0:
		busca.Mal("Ninguna combinación cumple")
		out = respuestaCalculo("Es imposible: ninguna combinación de caballeros y escuderos cumple lo que dicen.", "enumeracion", comp)
	case 1:
		busca.Bien("Solo una combinación cumple")
		out = respuestaCalculo(mayuscula(DescribirCaballeros(nombres, r.Modelos[0]))+".", "enumeracion", comp)
		out.Parrafos = []string{"Es la única combinación en la que cada caballero dice la verdad y cada escudero miente."}
	default:
		busca.Info("Hay %d combinaciones posibles", len(r.Modelos))
		var ops []string
		for _, m := range r.Modelos {
			ops = append(ops, DescribirCaballeros(nombres, m))
		}
		out = respuestaCalculo(fmt.Sprintf("No se puede saber: hay %d soluciones posibles.", len(r.Modelos)), "enumeracion", comp)
		out.Parrafos = ops
	}
	if nP <= 4 {
		out.Tablas = []nucleo.Tabla{tb}
	}
	out.Entendi = "Así lo entendí: " + strings.Join(lineas, "; ")
	out.Parrafos = append(out.Parrafos, r.Notas...)
	return out, nil
}
