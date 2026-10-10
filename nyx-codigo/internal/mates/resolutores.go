package mates

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// Resolutores returns the reasoning resolvers of this package: "mates.calculo", "mates.ecuacion",
// "mates.numeros" and "mates.problema". v (may be nil) stores the verbs learned in word problems.
func Resolutores(v Verbos) []nucleo.Resolutor {
	return []nucleo.Resolutor{resCalculo{}, resEcuacion{}, resNumeros{}, &resProblema{v: v}}
}

func textoDe(p *nucleo.Pregunta) string {
	if p == nil {
		return ""
	}
	if p.Codigo != "" && strings.TrimSpace(p.Resto) != "" {
		return p.Resto
	}
	return p.Texto
}

func comprobacionCalculo(pasan, total int, texto, detalle string) nucleo.Comprobacion {
	return nucleo.Comprobacion{Nivel: nucleo.NivelCalculo, Pasan: pasan, Total: total, Texto: texto, Detalle: detalle}
}

var errSinComprobar = errors.New("no he podido comprobar el resultado, así que no lo doy por bueno")

// ---- mates.calculo ----

type resCalculo struct{}

func (resCalculo) Nombre() string { return "mates.calculo" }

func (resCalculo) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	s := limpiarCalculo(textoDe(p))
	if s == "" || len(s) > 400 {
		return nucleo.Reconocimiento{}
	}
	e, err := AnalizarExpr(s)
	if err != nil || len(variables(e)) > 0 {
		return nucleo.Reconocimiento{}
	}
	if _, soloNumero := e.(Num); soloNumero {
		return nucleo.Reconocimiento{}
	}
	puntos := 0.9
	if s != strings.TrimSpace(textoDe(p)) {
		puntos = 0.95
	}
	return nucleo.Reconocimiento{Puntos: puntos, Intencion: nucleo.ICalculo, Lectura: "una operación: " + e.String()}
}

func (resCalculo) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	s := limpiarCalculo(textoDe(p))
	e, err := AnalizarExpr(s)
	if err != nil {
		return nucleo.Respuesta{}, err
	}
	if len(variables(e)) > 0 {
		return nucleo.Respuesta{}, fmt.Errorf("%w: tiene incógnitas; escribe una ecuación (… = …)", nucleo.ErrNoEntiendo)
	}
	n.Sub(nucleo.PasoEntender, "Entiendo la operación %s", e).Info("")
	v, pasos, err := reducir(ctx, e)
	if err != nil {
		n.Sub(nucleo.PasoError, "No puedo calcularlo: %s", mensaje(err)).Mal("")
		return nucleo.Respuesta{}, err
	}
	pasos = resumirPasos(pasos)
	sub := n.Sub(nucleo.PasoIntento, "Opero paso a paso")
	for _, ps := range pasos[1:] {
		sub.Detalle("→ %s", ps)
	}
	sub.Bien("")
	if !comprobarValor(e, v, nil) {
		n.Sub(nucleo.PasoError, "Al recalcularlo por otro camino no sale lo mismo").Mal("")
		return nucleo.Respuesta{}, errSinComprobar
	}
	n.Sub(nucleo.PasoComprobar, "Lo recalculo por otro camino (decimales de 256 bits): coincide").Bien("")
	res := v.numero()
	r := nucleo.Respuesta{
		Texto:          e.String() + " = " + res.String(),
		Plan:           pasos,
		Justificacion:  pasos,
		Comprobaciones: []nucleo.Comprobacion{comprobacionCalculo(1, 1, "Comprobado por cálculo", "lo recalculé por otro camino, con decimales de 256 bits")},
		Nivel:          nucleo.NivelCalculo,
		Metodo:         "exacto",
		Exito:          true,
		Sugerencias:    []string{"¿Por qué?"},
	}
	switch {
	case v.aprox:
		r.Metodo = "aproximado"
		r.Parrafos = append(r.Parrafos, "Es un resultado aproximado: usa funciones con decimales.")
	case v.conRadical():
		r.Parrafos = append(r.Parrafos, "En decimal: ≈ "+res.Decimal())
	case !v.a.IsInt():
		r.Parrafos = append(r.Parrafos, "En decimal: ≈ "+res.Decimal())
	default:
		if c := len(v.a.Num().String()); c > 15 {
			r.Parrafos = append(r.Parrafos, "Tiene "+strconv.Itoa(len(strings.TrimPrefix(v.a.Num().String(), "-")))+" cifras.")
		}
	}
	return r, nil
}

// ---- mates.ecuacion ----

type resEcuacion struct{}

func (resEcuacion) Nombre() string { return "mates.ecuacion" }

var reInicioEcuacion = regexp.MustCompile(`(?i)^\s*(¿\s*)?((resuelve|resolver|resuelva|soluciona|solucionar|halla|hallar|calcula|calcular|encuentra|despeja\s+\w+\s+(en|de)|despeja|cu[aá]nto\s+vale\s+\w+\s+(si|en)|cu[aá]l\s+es\s+la\s+soluci[oó]n\s+de|las?\s+ecuaci[oó]n(es)?|el\s+sistema(\s+de\s+ecuaciones)?|la\s+inecuaci[oó]n|el\s+valor\s+de\s+\w+\s+en|para\s+qu[eé]\s+valores\s+de\s+\w+\s+(se\s+cumple\s+)?)\s*:?\s*)+`)

var reDesigualdad = regexp.MustCompile(`<=|>=|=<|=>|[<>≤≥]`)

// extraerEcuaciones removes the request words and splits the equations.
func extraerEcuaciones(texto string) []string {
	s := reInicioEcuacion.ReplaceAllString(strings.TrimSpace(texto), "")
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "?.¿ "))
	var out []string
	for _, e := range partirEcuaciones(s) {
		if strings.Contains(e, "=") || reDesigualdad.MatchString(e) {
			out = append(out, strings.TrimSpace(e))
		}
	}
	return out
}

func esInecuacion(ecs []string) bool {
	return len(ecs) == 1 && reDesigualdad.MatchString(ecs[0])
}

func (resEcuacion) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	ecs := extraerEcuaciones(textoDe(p))
	if len(ecs) == 0 || len(ecs) > 10 {
		return nucleo.Reconocimiento{}
	}
	if esInecuacion(ecs) {
		partes := reDesigualdad.Split(ecs[0], -1)
		if len(partes) != 2 {
			return nucleo.Reconocimiento{}
		}
		l, err1 := AnalizarExpr(partes[0])
		r, err2 := AnalizarExpr(partes[1])
		if err1 != nil || err2 != nil || len(variables(Op{Op: '+', A: l, B: r})) == 0 {
			return nucleo.Reconocimiento{}
		}
		return nucleo.Reconocimiento{Puntos: 0.9, Intencion: nucleo.IEcuacion, Lectura: "inecuación en " + strings.Join(variables(Op{Op: '+', A: l, B: r}), ", ")}
	}
	var vars []string
	visto := map[string]bool{}
	for _, s := range ecs {
		partes := strings.Split(strings.ReplaceAll(s, "==", "="), "=")
		if len(partes) != 2 {
			return nucleo.Reconocimiento{}
		}
		l, err1 := AnalizarExpr(partes[0])
		r, err2 := AnalizarExpr(partes[1])
		if err1 != nil || err2 != nil {
			return nucleo.Reconocimiento{}
		}
		for _, v := range variables(Op{Op: '+', A: l, B: r}) {
			if !visto[v] {
				visto[v] = true
				vars = append(vars, v)
			}
		}
	}
	if len(vars) == 0 {
		return nucleo.Reconocimiento{}
	}
	lectura := "ecuación en " + vars[0]
	if len(ecs) > 1 {
		lectura = "sistema de " + strconv.Itoa(len(ecs)) + " ecuaciones con " + strings.Join(vars, ", ")
	}
	return nucleo.Reconocimiento{Puntos: 0.95, Intencion: nucleo.IEcuacion, Lectura: lectura}
}

func (resEcuacion) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	ecs := extraerEcuaciones(textoDe(p))
	if len(ecs) == 0 {
		return nucleo.Respuesta{}, fmt.Errorf("%w: no veo ninguna ecuación", nucleo.ErrNoEntiendo)
	}
	if esInecuacion(ecs) {
		n.Sub(nucleo.PasoEntender, "Entiendo la inecuación %s", ecs[0]).Info("")
		sol, pasos, err := inecuacion(ctx, ecs[0], n)
		if err != nil {
			return nucleo.Respuesta{}, err
		}
		return nucleo.Respuesta{
			Texto: sol, Plan: pasos, Justificacion: pasos,
			Parrafos:       []string{"Compruebo con valores a los dos lados del borde y en el borde: se cumple donde digo y solo ahí ✔"},
			Comprobaciones: []nucleo.Comprobacion{comprobacionCalculo(5, 5, "Comprobado con 5 valores", "valores a ambos lados del borde y en el borde")},
			Nivel:          nucleo.NivelCalculo, Metodo: "inecuacion", Exito: true, Sugerencias: []string{"¿Por qué?"},
		}, nil
	}
	n.Sub(nucleo.PasoEntender, "Entiendo %s: %s", plural(len(ecs), "la ecuación", "el sistema"), strings.Join(ecs, "; ")).Info("")
	sol, err := Ecuaciones(ctx, ecs, n)
	if err != nil {
		return nucleo.Respuesta{}, err
	}
	return respuestaSolucion(sol)
}

// respuestaSolucion writes the answer for a Solucion (it must be checked).
func respuestaSolucion(sol Solucion) (nucleo.Respuesta, error) {
	if !sol.Comprobado {
		return nucleo.Respuesta{}, errSinComprobar
	}
	r := nucleo.Respuesta{Plan: sol.Pasos, Justificacion: sol.Pasos, Nivel: nucleo.NivelCalculo, Exito: true,
		Sugerencias: []string{"¿Y si fuera otro número?", "¿Por qué?"}}
	r.Metodo = metodoDe(sol.Pasos)
	switch sol.Tipo {
	case "ninguna":
		r.Texto = "No tiene solución"
		for _, ps := range sol.Pasos {
			if strings.Contains(ps, "reales") || strings.Contains(ps, "negativo") {
				r.Texto = "No tiene soluciones reales"
			}
		}
		r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(1, 1, "Comprobado por cálculo", "la ecuación lleva a una igualdad imposible")}
	case "infinitas":
		r.Texto = "Tiene infinitas soluciones"
		if len(sol.Relacion) > 0 {
			r.Texto += ": " + strings.Join(sol.Relacion, ", ")
		}
		r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(len(sol.Comprobacion), len(sol.Comprobacion), "Comprobado por cálculo", "la ecuación se reduce a 0 = 0 o lo compruebo con soluciones concretas")}
	default:
		var partes []string
		k := 0
		for _, v := range sol.Variables {
			k = max(k, len(sol.Valores[v]))
		}
		for i := 0; i < k; i++ {
			var asign []string
			for _, v := range sol.Variables {
				if i < len(sol.Valores[v]) {
					val := sol.Valores[v][i]
					igual := " = "
					if !val.EsExacto {
						igual = " ≈ "
						asign = append(asign, v+igual+textoFloat(val.Aprox))
						continue
					}
					asign = append(asign, v+igual+val.String())
				}
			}
			partes = append(partes, strings.Join(asign, ", "))
		}
		r.Texto = strings.Join(partes, " o ")
		if sol.Tipo == "aproximada" {
			r.Texto += " (aproximado)"
		}
		for _, v := range sol.Variables {
			for _, val := range sol.Valores[v] {
				if val.EsExacto && (val.Radical != nil || val.Exacto != nil && !val.Exacto.IsInt()) {
					r.Parrafos = append(r.Parrafos, v+" = "+val.String()+" ≈ "+val.Decimal())
				}
			}
		}
		pasan := 0
		for _, c := range sol.Comprobacion {
			if strings.HasSuffix(c, "✔") {
				pasan++
			}
		}
		r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(pasan, len(sol.Comprobacion), "Comprobado sustituyendo", "sustituí cada solución en la ecuación original")}
	}
	for _, c := range sol.Comprobacion {
		r.Parrafos = append(r.Parrafos, "Compruebo: "+c)
	}
	if len(sol.Condicion) > 0 {
		r.Parrafos = append(r.Parrafos, "Con la condición "+strings.Join(sol.Condicion, ", "))
	}
	return r, nil
}

func metodoDe(pasos []string) string {
	t := strings.Join(pasos, "\n")
	switch {
	case strings.Contains(t, "Gauss"):
		return "gauss"
	case strings.Contains(t, "Durand"):
		return "durand-kerner"
	case strings.Contains(t, "Ruffini"):
		return "ruffini"
	case strings.Contains(t, "Δ"):
		return "formula_cuadratica"
	case strings.Contains(t, "Sustituyo"):
		return "sustitucion"
	}
	return "pasos"
}

// ---- mates.numeros ----

type resNumeros struct{}

func (resNumeros) Nombre() string { return "mates.numeros" }

var (
	reNumExpr      = regexp.MustCompile(`\d+(\s*[\^*+\-]\s*\(?\s*\d+\s*\)?)*!?`)
	reFactorizar   = regexp.MustCompile(`\b(factoriza\w*|descomp\w*|factores primos|factores|factorizacion)\b`)
	rePrimo        = regexp.MustCompile(`\bprimos?\b`)
	reListaPrimos  = regexp.MustCompile(`\bprimos\s+(hasta|menores\s+(que|de)|entre\s+1\s+y|del\s+1\s+al|que\s+hay\s+hasta)\b`)
	reMCD          = regexp.MustCompile(`\b(mcd|m\.c\.d|maximo comun divisor)\b`)
	reMCM          = regexp.MustCompile(`\b(mcm|m\.c\.m|minimo comun multiplo)\b`)
	reDivisores    = regexp.MustCompile(`\bdivisores\b`)
	rePorcentajeDe = regexp.MustCompile(`(-?\d+(?:[.,]\d+)?)\s*(?:%|por ciento)\s+(?:de|del)\s+(-?\d+(?:[.,]\d+)?)`)
	reQuePorc      = regexp.MustCompile(`(?:que|cual es el)\s+(?:porcentaje|tanto por ciento)\s+(?:es|representa|supone)\s+(?:el\s+)?(-?\d+(?:[.,]\d+)?)\s+(?:de|sobre|respecto a)\s+(-?\d+(?:[.,]\d+)?)`)
	reVariarPorc   = regexp.MustCompile(`\b(aumenta\w*|sube|subir|incrementa\w*|suma\w*|rebaja\w*|descuenta\w*|disminu\w*|baja|bajar|resta\w*|quita\w*)\s+(?:un|en un|el)?\s*(-?\d+(?:[.,]\d+)?)\s*(?:%|por ciento)\s+(?:a|de|sobre)?\s*(?:los|las|el|la)?\s*(-?\d+(?:[.,]\d+)?)`)
)

func (resNumeros) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	t := nucleo.Normalizar(textoDe(p))
	if !reNumExpr.MatchString(t) {
		return nucleo.Reconocimiento{}
	}
	lect := ""
	switch {
	case reFactorizar.MatchString(t):
		lect = "descomponer en factores primos"
	case reListaPrimos.MatchString(t):
		lect = "lista de números primos"
	case rePrimo.MatchString(t):
		lect = "saber si es primo"
	case reMCD.MatchString(t):
		lect = "máximo común divisor"
	case reMCM.MatchString(t):
		lect = "mínimo común múltiplo"
	case reDivisores.MatchString(t):
		lect = "divisores"
	case rePorcentajeDe.MatchString(t) || reQuePorc.MatchString(t) || reVariarPorc.MatchString(t):
		lect = "porcentaje"
	default:
		return nucleo.Reconocimiento{}
	}
	return nucleo.Reconocimiento{Puntos: 0.92, Intencion: nucleo.INumeros, Lectura: lect}
}

// enteros reads the integers written in the text (expressions such as 2^61-1 are evaluated).
func enterosDe(ctx context.Context, t string) ([]*big.Int, []string, error) {
	var out []*big.Int
	var txt []string
	for _, m := range reNumExpr.FindAllString(t, -1) {
		m = strings.TrimSpace(strings.TrimRight(m, "+-*^ "))
		e, err := AnalizarExpr(m)
		if err != nil {
			continue
		}
		v, err := evaluar(ctx, e, nil)
		if err != nil {
			return nil, nil, err
		}
		if !v.entero() {
			continue
		}
		out = append(out, new(big.Int).Set(v.a.Num()))
		txt = append(txt, m)
	}
	return out, txt, nil
}

func (rn resNumeros) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	t := nucleo.Normalizar(textoDe(p))
	switch {
	case rePorcentajeDe.MatchString(t) || reQuePorc.MatchString(t) || reVariarPorc.MatchString(t):
		if !(reFactorizar.MatchString(t) || reMCD.MatchString(t) || reMCM.MatchString(t)) {
			return porcentajes(t, n)
		}
	}
	nums, textos, err := enterosDe(ctx, t)
	if err != nil {
		return nucleo.Respuesta{}, err
	}
	if len(nums) == 0 {
		return nucleo.Respuesta{}, fmt.Errorf("%w: no veo ningún número entero", nucleo.ErrNoEntiendo)
	}
	switch {
	case reFactorizar.MatchString(t):
		return factorizar(ctx, nums[0], textos[0], n)
	case reListaPrimos.MatchString(t):
		return listaPrimos(nums[len(nums)-1], n)
	case rePrimo.MatchString(t):
		return primo(ctx, nums[0], textos[0], n)
	case reMCD.MatchString(t):
		return mcd(nums, n)
	case reMCM.MatchString(t):
		return mcm(nums, n)
	case reDivisores.MatchString(t):
		return divisores(nums[0], n)
	}
	return nucleo.Respuesta{}, fmt.Errorf("%w: no sé qué quieres saber de ese número", nucleo.ErrNoEntiendo)
}

func respNum(texto, metodo string, pasos []string, comp nucleo.Comprobacion) nucleo.Respuesta {
	return nucleo.Respuesta{Texto: texto, Plan: pasos, Justificacion: pasos, Comprobaciones: []nucleo.Comprobacion{comp},
		Nivel: nucleo.NivelCalculo, Metodo: metodo, Exito: true, Sugerencias: []string{"¿Por qué?"}}
}

func factorizar(ctx context.Context, x *big.Int, txt string, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	sub := n.Sub(nucleo.PasoIntento, "Descompongo %s: divido entre primos hasta un millón y luego uso Pollard-Brent", x)
	fs, err := Factorizar(ctx, x)
	if err != nil {
		sub.Mal("No he podido descomponerlo: %s", mensaje(err))
		return nucleo.Respuesta{}, err
	}
	sub.Bien("")
	// check: the product gives x back and every factor is prime
	ok := producto(fs).Cmp(new(big.Int).Abs(x)) == 0
	for _, f := range fs {
		ok = ok && EsPrimo(f.Primo)
	}
	if !ok {
		return nucleo.Respuesta{}, errSinComprobar
	}
	n.Sub(nucleo.PasoComprobar, "Compruebo: multiplico los factores y sale %s; cada factor es primo", new(big.Int).Abs(x)).Bien("")
	pasos := []string{"Divido entre los primos pequeños y, si queda un trozo grande, busco factores con Pollard-Brent"}
	if len(fs) == 0 {
		return respNum("El 1 no tiene factores primos", "factorizacion", pasos,
			comprobacionCalculo(1, 1, "Comprobado por cálculo", "el 1 no es primo ni compuesto")), nil
	}
	texto := new(big.Int).Abs(x).String() + " = " + textoFactores(fs)
	if len(fs) == 1 && fs[0].Exp == 1 {
		texto = new(big.Int).Abs(x).String() + " es primo: no se puede descomponer más"
	}
	if x.Sign() < 0 {
		texto = "-" + strings.TrimPrefix(texto, "") + " (con signo menos delante)"
	}
	return respNum(texto, "factorizacion", pasos, comprobacionCalculo(len(fs)+1, len(fs)+1, "Comprobado por cálculo",
		"multipliqué los factores y comprobé que cada uno es primo (Miller-Rabin, 20 rondas)")), nil
}

func primo(ctx context.Context, x *big.Int, txt string, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	etiqueta := x.String()
	if txt != x.String() && len(etiqueta) > 30 {
		etiqueta = txt
	} else if txt != x.String() {
		etiqueta = txt + " = " + x.String()
	}
	if x.Cmp(big.NewInt(2)) < 0 {
		return respNum("No, "+x.String()+" no es primo: los primos empiezan en el 2", "primalidad",
			[]string{"Un primo es un número mayor que 1 que solo se divide entre 1 y él mismo"},
			comprobacionCalculo(1, 1, "Comprobado por cálculo", "por definición")), nil
	}
	sub := n.Sub(nucleo.PasoIntento, "Aplico Miller-Rabin con 20 rondas a %s", etiqueta)
	if EsPrimo(x) {
		sub.Bien("")
		pasos := []string{"Prueba de Miller-Rabin con 20 bases: ninguna demuestra que sea compuesto"}
		if x.BitLen() <= 40 {
			// small: confirm by trial division as well
			r := new(big.Int).Sqrt(x)
			pasos = append(pasos, "Además lo divido entre todos los números hasta √"+x.String()+" ≈ "+r.String()+": ninguno lo divide")
		}
		return respNum("Sí, "+etiqueta+" es primo (Miller-Rabin, 20 rondas)", "miller-rabin", pasos,
			comprobacionCalculo(20, 20, "Comprobado por cálculo", "Miller-Rabin con 20 rondas")), nil
	}
	sub.Info("No es primo")
	pasos := []string{"Miller-Rabin encuentra una base que demuestra que es compuesto"}
	ctx2, cancel := context.WithTimeout(ctx, tiempoFactorizar)
	defer cancel()
	if fs, err := Factorizar(ctx2, x); err == nil && len(fs) > 0 && producto(fs).Cmp(x) == 0 {
		pasos = append(pasos, x.String()+" = "+textoFactores(fs))
		return respNum("No, "+etiqueta+" no es primo: "+x.String()+" = "+textoFactores(fs), "miller-rabin", pasos,
			comprobacionCalculo(1, 1, "Comprobado por cálculo", "encontré sus factores y los multipliqué")), nil
	}
	return respNum("No, "+etiqueta+" no es primo", "miller-rabin", pasos,
		comprobacionCalculo(1, 1, "Comprobado por cálculo", "la prueba de Miller-Rabin demuestra que es compuesto")), nil
}

const maxCriba = 1_000_000

func listaPrimos(hasta *big.Int, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if !hasta.IsInt64() || hasta.Int64() > maxCriba {
		return nucleo.Respuesta{}, fmt.Errorf("%w: solo hago la lista de primos hasta un millón", nucleo.ErrNoSoportado)
	}
	m := int(hasta.Int64())
	compuesto := make([]bool, max(m+1, 2))
	var ps []string
	for i := 2; i <= m; i++ {
		if compuesto[i] {
			continue
		}
		ps = append(ps, strconv.Itoa(i))
		for j := i * i; j <= m; j += i {
			compuesto[j] = true
		}
	}
	// check each listed number with Miller-Rabin
	for _, s := range ps {
		v, _ := new(big.Int).SetString(s, 10)
		if !EsPrimo(v) {
			return nucleo.Respuesta{}, errSinComprobar
		}
	}
	n.Sub(nucleo.PasoIntento, "Criba de Eratóstenes hasta %d: %d primos", m, len(ps)).Bien("")
	mostrar := ps
	cola := ""
	if len(mostrar) > 100 {
		mostrar = ps[:100]
		cola = ", …"
	}
	texto := fmt.Sprintf("Hay %d primos hasta %d: %s%s", len(ps), m, strings.Join(mostrar, ", "), cola)
	return respNum(texto, "criba", []string{"Criba de Eratóstenes: tacho los múltiplos de cada primo"},
		comprobacionCalculo(len(ps), len(ps), "Comprobado por cálculo", "cada número de la lista pasa Miller-Rabin")), nil
}

func mcd(nums []*big.Int, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if len(nums) < 2 {
		return nucleo.Respuesta{}, fmt.Errorf("%w: el mcd necesita al menos dos números", nucleo.ErrNoEntiendo)
	}
	var pasos []string
	g := new(big.Int).Abs(nums[0])
	for _, x := range nums[1:] {
		a, b := new(big.Int).Set(g), new(big.Int).Abs(x)
		if a.Cmp(b) < 0 {
			a, b = b, a
		}
		for b.Sign() != 0 && len(pasos) < 40 {
			q, r := new(big.Int).QuoRem(a, b, new(big.Int))
			pasos = append(pasos, fmt.Sprintf("%s = %s·%s + %s", a, q, b, r))
			a, b = b, r
		}
		g = MCD(g, x)
	}
	// check: g divides every number, and the quotients have no common factor
	ok := g.Sign() != 0
	var cocientes []*big.Int
	for _, x := range nums {
		if ok {
			q, r := new(big.Int).QuoRem(x, g, new(big.Int))
			ok = r.Sign() == 0
			cocientes = append(cocientes, q)
		}
	}
	if ok {
		h := new(big.Int)
		for _, q := range cocientes {
			h = MCD(h, q)
		}
		ok = h.Cmp(big.NewInt(1)) == 0
	}
	if !ok {
		return nucleo.Respuesta{}, errSinComprobar
	}
	n.Sub(nucleo.PasoIntento, "Algoritmo de Euclides").Bien("mcd = %s", g)
	n.Sub(nucleo.PasoComprobar, "Compruebo: %s divide a todos y los cocientes no tienen factores comunes", g).Bien("")
	pasos = append([]string{"Algoritmo de Euclides: divido y me quedo con el resto hasta que da 0"}, pasos...)
	return respNum("mcd("+listaInts(nums)+") = "+g.String(), "euclides", pasos,
		comprobacionCalculo(len(nums)+1, len(nums)+1, "Comprobado por cálculo", "divide a todos y los cocientes no tienen divisores comunes")), nil
}

func mcm(nums []*big.Int, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if len(nums) < 2 {
		return nucleo.Respuesta{}, fmt.Errorf("%w: el mcm necesita al menos dos números", nucleo.ErrNoEntiendo)
	}
	m := new(big.Int).Abs(nums[0])
	var pasos []string
	for _, x := range nums[1:] {
		g := MCD(m, x)
		nuevo := MCM(m, x)
		pasos = append(pasos, fmt.Sprintf("mcm(%s, %s) = %s·%s / mcd = %s / %s = %s", m, new(big.Int).Abs(x), m, new(big.Int).Abs(x),
			new(big.Int).Mul(m, new(big.Int).Abs(x)), g, nuevo))
		m = nuevo
	}
	ok := m.Sign() != 0
	for _, x := range nums {
		ok = ok && new(big.Int).Mod(m, new(big.Int).Abs(x)).Sign() == 0
	}
	// minimal: m/p is not a common multiple for any prime p of m (small m only)
	if ok && m.BitLen() <= 62 {
		ctx, cancel := context.WithTimeout(context.Background(), tiempoFactorizar)
		fs, err := Factorizar(ctx, m)
		cancel()
		if err == nil {
			for _, f := range fs {
				menor := new(big.Int).Quo(m, f.Primo)
				todos := true
				for _, x := range nums {
					if new(big.Int).Mod(menor, new(big.Int).Abs(x)).Sign() != 0 {
						todos = false
					}
				}
				if todos {
					ok = false
				}
			}
		}
	}
	if !ok {
		return nucleo.Respuesta{}, errSinComprobar
	}
	n.Sub(nucleo.PasoIntento, "mcm(a, b) = a·b / mcd(a, b)").Bien("mcm = %s", m)
	n.Sub(nucleo.PasoComprobar, "Compruebo: %s es múltiplo de todos y ninguno menor lo es", m).Bien("")
	return respNum("mcm("+listaInts(nums)+") = "+m.String(), "mcm", pasos,
		comprobacionCalculo(len(nums)+1, len(nums)+1, "Comprobado por cálculo", "es múltiplo de todos y ninguno menor lo es")), nil
}

func listaInts(xs []*big.Int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = x.String()
	}
	return strings.Join(s, ", ")
}

func divisores(x *big.Int, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	ds, err := Divisores(x)
	if err != nil {
		return nucleo.Respuesta{}, err
	}
	abs := new(big.Int).Abs(x)
	for _, d := range ds {
		if new(big.Int).Mod(abs, d).Sign() != 0 {
			return nucleo.Respuesta{}, errSinComprobar
		}
	}
	n.Sub(nucleo.PasoIntento, "Descompongo %s en primos y combino los factores", abs).Bien("%d divisores", len(ds))
	n.Sub(nucleo.PasoComprobar, "Compruebo que cada uno divide a %s", abs).Bien("")
	return respNum(fmt.Sprintf("Los divisores de %s son: %s (%d divisores)", abs, listaInts(ds), len(ds)), "divisores",
		[]string{"Descompongo en factores primos y multiplico todas las combinaciones"},
		comprobacionCalculo(len(ds), len(ds), "Comprobado por cálculo", "cada uno divide al número")), nil
}

func ratDeTexto(s string) *big.Rat {
	r, _ := new(big.Rat).SetString(strings.Replace(s, ",", ".", 1))
	return r
}

func porcentajes(t string, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	cien := big.NewRat(100, 1)
	if m := reVariarPorc.FindStringSubmatch(t); m != nil {
		p, base := ratDeTexto(m[2]), ratDeTexto(m[3])
		if p == nil || base == nil {
			return nucleo.Respuesta{}, nucleo.ErrNoEntiendo
		}
		sube := !regexp.MustCompile(`^(rebaj|descuent|disminu|baja|rest|quit)`).MatchString(m[1])
		factor := new(big.Rat).Quo(p, cien)
		cambio := new(big.Rat).Mul(base, factor)
		res := new(big.Rat).Set(base)
		verbo, signo := "Aumento", "+"
		if sube {
			res.Add(res, cambio)
		} else {
			res.Sub(res, cambio)
			verbo, signo = "Rebajo", "-"
		}
		// check: the change over the base is the percentage
		chk := new(big.Rat).Sub(res, base)
		chk.Abs(chk)
		if new(big.Rat).Quo(chk, base).Cmp(factor) != 0 {
			return nucleo.Respuesta{}, errSinComprobar
		}
		pasos := []string{fmt.Sprintf("El %s %% de %s es %s·%s/100 = %s", textoRat(p), textoRat(base), textoRat(base), textoRat(p), textoRat(cambio)),
			fmt.Sprintf("%s: %s %s %s = %s", verbo, textoRat(base), signo, textoRat(cambio), textoRat(res))}
		n.Sub(nucleo.PasoIntento, "%s", pasos[1]).Bien("")
		r := respNum(fmt.Sprintf("%s un %s %% a %s da %s", verbo, textoRat(p), textoRat(base), textoRat(res)), "porcentaje", pasos,
			comprobacionCalculo(1, 1, "Comprobado por cálculo", "la diferencia dividida entre el total da el porcentaje"))
		agregarDecimal(&r, res)
		return r, nil
	}
	if m := reQuePorc.FindStringSubmatch(t); m != nil {
		a, b := ratDeTexto(m[1]), ratDeTexto(m[2])
		if a == nil || b == nil || b.Sign() == 0 {
			return nucleo.Respuesta{}, nucleo.ErrNoEntiendo
		}
		res := new(big.Rat).Mul(new(big.Rat).Quo(a, b), cien)
		if new(big.Rat).Quo(new(big.Rat).Mul(res, b), cien).Cmp(a) != 0 {
			return nucleo.Respuesta{}, errSinComprobar
		}
		pasos := []string{fmt.Sprintf("%s/%s·100 = %s", textoRat(a), textoRat(b), textoRat(res))}
		n.Sub(nucleo.PasoIntento, "%s", pasos[0]).Bien("")
		r := respNum(fmt.Sprintf("%s es el %s %% de %s", textoRat(a), textoRat(res), textoRat(b)), "porcentaje", pasos,
			comprobacionCalculo(1, 1, "Comprobado por cálculo", "el "+textoRat(res)+" % de "+textoRat(b)+" da "+textoRat(a)))
		agregarDecimal(&r, res)
		return r, nil
	}
	if m := rePorcentajeDe.FindStringSubmatch(t); m != nil {
		p, base := ratDeTexto(m[1]), ratDeTexto(m[2])
		if p == nil || base == nil {
			return nucleo.Respuesta{}, nucleo.ErrNoEntiendo
		}
		res := new(big.Rat).Quo(new(big.Rat).Mul(p, base), cien)
		if base.Sign() != 0 && new(big.Rat).Mul(new(big.Rat).Quo(res, base), cien).Cmp(p) != 0 {
			return nucleo.Respuesta{}, errSinComprobar
		}
		pasos := []string{fmt.Sprintf("%s·%s/100 = %s", textoRat(base), textoRat(p), textoRat(res))}
		n.Sub(nucleo.PasoIntento, "%s", pasos[0]).Bien("")
		n.Sub(nucleo.PasoComprobar, "Compruebo: %s/%s·100 = %s", textoRat(res), textoRat(base), textoRat(p)).Bien("")
		r := respNum(fmt.Sprintf("El %s %% de %s es %s", textoRat(p), textoRat(base), textoRat(res)), "porcentaje", pasos,
			comprobacionCalculo(1, 1, "Comprobado por cálculo", "dividí el resultado entre el total y sale el porcentaje"))
		agregarDecimal(&r, res)
		return r, nil
	}
	return nucleo.Respuesta{}, fmt.Errorf("%w: no entiendo el porcentaje", nucleo.ErrNoEntiendo)
}

func agregarDecimal(r *nucleo.Respuesta, v *big.Rat) {
	if !v.IsInt() {
		f, _ := v.Float64()
		r.Parrafos = append(r.Parrafos, "En decimal: ≈ "+textoFloat(f))
	}
}

// ---- mates.problema ----

type resProblema struct {
	v Verbos
}

func (*resProblema) Nombre() string { return "mates.problema" }

var reTieneIgualdadSimbolos = regexp.MustCompile(`[=<>≤≥]`)

func (rp *resProblema) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	t := textoDe(p)
	if len(t) > 2000 || reTieneIgualdadSimbolos.MatchString(t) {
		return nucleo.Reconocimiento{}
	}
	pls, err := Plantear(p, rp.v)
	if err != nil || len(pls) == 0 {
		return nucleo.Reconocimiento{}
	}
	pl := pls[0]
	if len(pl.Desconocido) > 0 {
		return nucleo.Reconocimiento{Puntos: 0.6, Intencion: nucleo.IProblema, Lectura: "problema con un verbo que no conozco: «" + pl.Desconocido[0] + "»"}
	}
	lect := map[string]string{"traduccion": "problema de «un número»", "relaciones": "problema de edades o reparto",
		"situacion": "problema de cantidades que cambian", "movimiento": "problema de velocidad, distancia y tiempo",
		"precio": "problema de precios"}[pl.Tipo]
	return nucleo.Reconocimiento{Puntos: 0.88, Intencion: nucleo.IProblema, Lectura: lect}
}

// verbosMemoria overlays verbs answered in this run on top of the persistent store.
type verbosMemoria struct {
	mu   sync.Mutex
	base Verbos
	m    map[string]string
}

func (vm *verbosMemoria) Efecto(verbo string) (string, bool) {
	vm.mu.Lock()
	e, ok := vm.m[verbo]
	vm.mu.Unlock()
	if ok {
		return e, true
	}
	if vm.base != nil {
		return vm.base.Efecto(verbo)
	}
	return "", false
}

func (vm *verbosMemoria) Aprender(verbo, efecto string) error {
	vm.mu.Lock()
	vm.m[verbo] = efecto
	vm.mu.Unlock()
	// "no cambia nada" is only remembered for this problem: the store keeps the four real effects
	if vm.base != nil && efecto != "nada" {
		return vm.base.Aprender(verbo, efecto)
	}
	return nil
}

// efectoRespuesta maps the button answer to an effect.
func efectoRespuesta(r string) (string, bool) {
	r = nucleo.Normalizar(r)
	switch {
	case strings.Contains(r, "no cambia") || r == "nada" || strings.Contains(r, "ninguno"):
		return "nada", true
	case strings.HasPrefix(r, "sum") || strings.Contains(r, "suma") || r == "mas" || r == "+":
		return "sumar", true
	case strings.HasPrefix(r, "rest") || strings.Contains(r, "resta") || r == "menos" || r == "-":
		return "restar", true
	case strings.Contains(r, "fija") || strings.Contains(r, "tiene"):
		return "fijar", true
	case strings.Contains(r, "da") || strings.Contains(r, "transfier") || strings.Contains(r, "pasa"):
		return "transferir", true
	}
	return "", false
}

func (rp *resProblema) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	vm := &verbosMemoria{base: rp.v, m: map[string]string{}}
	var pls []Planteamiento
	var err error
	for ronda := 0; ronda < 5; ronda++ {
		pls, err = Plantear(p, vm)
		if err != nil {
			return nucleo.Respuesta{}, fmt.Errorf("%w. No sé plantearlo. Prueba a escribirlo así: «el doble de un número más 3 es 11»", err)
		}
		var descon []string
		for _, pl := range pls {
			descon = append(descon, pl.Desconocido...)
		}
		if len(descon) == 0 {
			break
		}
		for _, verbo := range descon {
			preg := nucleo.PreguntaUsuario{ID: "verbo:" + verbo, Texto: "¿«" + verbo + "» suma o resta?",
				Opciones: []string{"suma", "resta", "no cambia nada"}}
			n.Sub(nucleo.PasoPregunta, "No conozco el verbo «%s»: ¿suma o resta?", verbo)
			resp, err := n.Traza().Preguntar(ctx, preg)
			if err != nil {
				return nucleo.Respuesta{}, fmt.Errorf("%w: no sé si «%s» suma o resta", nucleo.ErrNoEntiendo, verbo)
			}
			efecto, ok := efectoRespuesta(resp)
			if !ok {
				return nucleo.Respuesta{}, fmt.Errorf("%w: no entiendo la respuesta «%s»", nucleo.ErrNoEntiendo, resp)
			}
			if err := vm.Aprender(verbo, efecto); err != nil {
				n.Nota("No he podido guardar el verbo «%s»: %s", verbo, err)
			} else {
				n.Sub(nucleo.PasoMemoria, "Aprendo que «%s» significa %s", verbo, map[string]string{"sumar": "sumar", "restar": "restar", "nada": "que no cambia nada", "fijar": "tener", "transferir": "dar"}[efecto]).Bien("")
			}
		}
	}
	ultimo := nucleo.ErrNoEntiendo
	for _, pl := range pls {
		if len(pl.Ecuaciones) == 0 {
			continue
		}
		sub := n.Sub(nucleo.PasoPlan, "Lo planteo así")
		for _, m := range pl.Mapeo {
			sub.Detalle("%s", m)
		}
		sub.Info("")
		sol, err := Ecuaciones(ctx, pl.Ecuaciones, n)
		if err != nil {
			ultimo = err
			continue
		}
		if !sol.Comprobado || sol.Tipo != "unica" {
			ultimo = errSinComprobar
			if sol.Tipo == "ninguna" || sol.Tipo == "infinitas" {
				ultimo = fmt.Errorf("%w: el problema no tiene una solución única", nucleo.ErrNoEntiendo)
			}
			continue
		}
		return respuestaProblema(pl, sol), nil
	}
	return nucleo.Respuesta{}, ultimo
}

func respuestaProblema(pl Planteamiento, sol Solucion) nucleo.Respuesta {
	valor := func(v string) string {
		vs := sol.Valores[v]
		if len(vs) == 0 {
			return "?"
		}
		s := vs[0].String()
		if vs[0].Exacto != nil && !vs[0].Exacto.IsInt() {
			s += " (≈ " + vs[0].Decimal() + ")"
		}
		return s
	}
	var texto string
	switch pl.Tipo {
	case "relaciones":
		var partes []string
		for _, v := range nombresOrdenados(pl.Incognitas) {
			quien := pl.Incognitas[v]
			for _, pre := range []string{"la edad de ", "lo que tiene "} {
				quien = strings.TrimPrefix(quien, pre)
			}
			parte := quien + " tiene " + valor(v)
			if pl.Unidad != "" {
				parte += " " + pl.Unidad
			}
			partes = append(partes, parte)
		}
		texto = strings.Join(partes, " y ")
	case "traduccion":
		if len(pl.Incognitas) > 1 {
			var partes []string
			for _, v := range nombresOrdenados(pl.Incognitas) {
				partes = append(partes, v+" = "+valor(v))
			}
			texto = fmt.Sprintf(pl.Respuesta, strings.Join(partes, ", "))
		} else {
			texto = fmt.Sprintf(pl.Respuesta, valor(pl.Pregunta))
		}
	default:
		texto = fmt.Sprintf(pl.Respuesta, valor(pl.Pregunta))
	}
	r := nucleo.Respuesta{Texto: texto, Nivel: nucleo.NivelCalculo, Exito: true, Metodo: "problema." + pl.Tipo,
		Sugerencias: []string{"¿Por qué?", "¿Y si fuera otro número?"}}
	r.Parrafos = append(r.Parrafos, "Lo planteo así:")
	r.Parrafos = append(r.Parrafos, pl.Mapeo...)
	var ecs []string
	for _, e := range pl.Ecuaciones {
		ecs = append(ecs, strings.ReplaceAll(e, "*", "·"))
	}
	r.Parrafos = append(r.Parrafos, "Ecuación: "+strings.Join(ecs, "; "))
	for _, c := range sol.Comprobacion {
		r.Parrafos = append(r.Parrafos, "Compruebo: "+c)
	}
	r.Plan = append(append([]string{}, pl.Mapeo...), sol.Pasos...)
	r.Justificacion = r.Plan
	pasan := 0
	for _, c := range sol.Comprobacion {
		if strings.HasSuffix(c, "✔") {
			pasan++
		}
	}
	r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(pasan, len(sol.Comprobacion), "Comprobado sustituyendo",
		"sustituí la solución en las ecuaciones del planteamiento")}
	return r
}
