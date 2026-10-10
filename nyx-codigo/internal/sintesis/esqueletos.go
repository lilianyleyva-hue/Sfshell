package sintesis

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Sketches (§4.7 step 9): the Spanish frame VERBO(MOD(OBJ)) becomes s-expressions with holes, which are
// filled from the lambda pools or the bank, cheapest first. DesdeConceptos (step 10) maps the modifiers to
// lambdas directly and adds a concept-covering enumeration when there are no examples.

// Esqueletos maps the frame to sketches over the firma's parameters ("sume los pares" → (suma (filtra ? nums))).
func Esqueletos(m *nucleo.Marco, f nucleo.Firma, r *Registro) []*Expr {
	return esqueletos(m, f, r, false)
}

type genEsq struct {
	m       *nucleo.Marco
	conMods bool
	usados  map[string]bool
	lista   string // first list parameter (not a matrix)
	elem    nucleo.Tipo
	lista2  string
	texto   string
	texto2  string
	entero  string
	entero2 string
	matriz  string
	out     []string
}

func esqueletos(m *nucleo.Marco, f nucleo.Firma, r *Registro, conMods bool) []*Expr {
	if m == nil || r == nil || len(f.Res) != 1 {
		return nil
	}
	g := &genEsq{m: m, conMods: conMods, usados: map[string]bool{}}
	for _, p := range f.Params {
		g.usados[p.Nombre] = true
		switch {
		case mismoTipo(p.Tipo, tLLI):
			if g.matriz == "" {
				g.matriz = p.Nombre
			}
		case p.Tipo.Clase == nucleo.CLista:
			if g.lista == "" {
				g.lista, g.elem = p.Nombre, *p.Tipo.Elem
			} else if g.lista2 == "" {
				g.lista2 = p.Nombre
			}
		case p.Tipo.Clase == nucleo.CString:
			if g.texto == "" {
				g.texto = p.Nombre
			} else if g.texto2 == "" {
				g.texto2 = p.Nombre
			}
		case p.Tipo.Clase == nucleo.CInt:
			if g.entero == "" {
				g.entero = p.Nombre
			} else if g.entero2 == "" {
				g.entero2 = p.Nombre
			}
		}
	}
	g.generar()
	var out []*Expr
	vistos := map[string]bool{}
	for _, s := range g.out {
		e, err := Parse(s, r, f)
		if err != nil {
			continue
		}
		k := e.String()
		if vistos[k] {
			continue
		}
		vistos[k] = true
		out = append(out, e)
	}
	return out
}

func (g *genEsq) add(formato string, a ...any) { g.out = append(g.out, fmt.Sprintf(formato, a...)) }

func (g *genEsq) varLambda() string { return nombresLambda(1, g.usados)[0] }

func numeroMod(md nucleo.Modificador) (string, bool) {
	if len(md.Numeros) == 0 {
		return "", false
	}
	x := md.Numeros[0]
	if x == math.Trunc(x) && math.Abs(x) < 1e15 {
		return strconv.Itoa(int(x)), true
	}
	return "", false
}

// cuerpoMod maps one modifier to a lambda body over variable x of type elem.
func cuerpoMod(md nucleo.Modificador, x string, elem nucleo.Tipo) (string, bool) {
	n, hayN := numeroMod(md)
	var b string
	switch elem.Clase {
	case nucleo.CInt:
		switch md.Concepto {
		case "par":
			b = "(esPar " + x + ")"
		case "impar":
			b = "(esImpar " + x + ")"
		case "positivo":
			b = "(esPositivo " + x + ")"
		case "negativo":
			b = "(esNegativo " + x + ")"
		case "cero":
			b = "(esCero " + x + ")"
		case "primo", "primos":
			b = "(esPrimo " + x + ")"
		case "mayor_que":
			if hayN {
				b = "(> " + x + " " + n + ")"
			}
		case "menor_que":
			if hayN {
				b = "(< " + x + " " + n + ")"
			}
		case "igual_a":
			if hayN {
				b = "(== " + x + " " + n + ")"
			}
		case "distinto_de":
			if hayN {
				b = "(!= " + x + " " + n + ")"
			}
		case "divisible_por":
			if hayN {
				b = "(divisible " + x + " " + n + ")"
			}
		}
	case nucleo.CRune:
		switch md.Concepto {
		case "vocal":
			b = "(esVocal " + x + ")"
		case "consonante":
			b = "(esConsonante " + x + ")"
		case "digito":
			b = "(esDigito " + x + ")"
		case "letra", "letras":
			b = "(esLetra " + x + ")"
		case "mayuscula":
			b = "(esMayus " + x + ")"
		case "minuscula":
			b = "(esMinus " + x + ")"
		case "espacio":
			b = "(esEspacio " + x + ")"
		case "igual_a":
			if rs := []rune(md.Texto); len(rs) == 1 {
				b = "(== " + x + " " + strconv.QuoteRune(rs[0]) + ")"
			}
		}
	case nucleo.CString:
		switch md.Concepto {
		case "mayor_que", "longitud_mayor":
			if hayN {
				b = "(> (largoS " + x + ") " + n + ")"
			}
		case "menor_que", "longitud_menor":
			if hayN {
				b = "(< (largoS " + x + ") " + n + ")"
			}
		case "empieza_con":
			if md.Texto != "" {
				b = "(empiezaCon " + x + " " + strconv.Quote(md.Texto) + ")"
			}
		case "termina_con":
			if md.Texto != "" {
				b = "(terminaCon " + x + " " + strconv.Quote(md.Texto) + ")"
			}
		case "palindromo":
			b = "(esPalindromoS " + x + ")"
		case "igual_a":
			if md.Texto != "" {
				b = "(== " + x + " " + strconv.Quote(md.Texto) + ")"
			}
		}
	}
	if b == "" {
		return "", false
	}
	if md.Negado {
		b = "(no " + b + ")"
	}
	return b, true
}

var conceptosPredicado = map[string]bool{
	"par": true, "impar": true, "positivo": true, "negativo": true, "cero": true, "primo": true, "mayor_que": true,
	"menor_que": true, "igual_a": true, "distinto_de": true, "divisible_por": true, "vocal": true, "consonante": true,
	"digito": true, "letra": true, "mayuscula": true, "minuscula": true, "espacio": true, "empieza_con": true,
	"termina_con": true, "longitud_mayor": true, "longitud_menor": true,
}

// modsDe returns the predicate modifiers of the frame (the element counts as one: "las vocales").
func modsDe(m *nucleo.Marco) []nucleo.Modificador {
	var out []nucleo.Modificador
	tiene := map[string]bool{}
	for _, md := range m.Mods {
		if conceptosPredicado[md.Concepto] || md.Concepto == "primos" || md.Concepto == "palindromo" {
			out = append(out, md)
			tiene[md.Concepto] = true
		}
	}
	if conceptosPredicado[m.Elemento] && !tiene[m.Elemento] {
		out = append(out, nucleo.Modificador{Concepto: m.Elemento})
	}
	return out
}

// pred returns the lambda for elements of type elem: the modifiers' lambda (conMods) or a hole.
func (g *genEsq) pred(elem nucleo.Tipo) string { return g.predicado(elem, false) }

// predicado is pred, negated when negar ("elimina los pares" keeps the elements that are not even).
func (g *genEsq) predicado(elem nucleo.Tipo, negar bool) string {
	if !g.conMods {
		return "?"
	}
	mods := modsDe(g.m)
	if len(mods) == 0 {
		return "?"
	}
	x := g.varLambda()
	var cuerpos []string
	for _, md := range mods {
		b, ok := cuerpoMod(md, x, elem)
		if !ok {
			return "?"
		}
		cuerpos = append(cuerpos, b)
	}
	cuerpo := cuerpos[0]
	for _, b := range cuerpos[1:] {
		cuerpo = "(y " + cuerpo + " " + b + ")"
	}
	if negar {
		if strings.HasPrefix(cuerpo, "(no ") {
			cuerpo = strings.TrimSuffix(strings.TrimPrefix(cuerpo, "(no "), ")")
		} else {
			cuerpo = "(no " + cuerpo + ")"
		}
	}
	return "(λ " + x + " " + cuerpo + ")"
}

func (g *genEsq) hayMods() bool { return len(modsDe(g.m)) > 0 }

// fuente returns the list to work on, filtered when the frame has modifiers.
func (g *genEsq) fuentes() []string {
	if g.lista == "" {
		return nil
	}
	if g.hayMods() {
		return []string{"(filtra " + g.pred(g.elem) + " " + g.lista + ")"}
	}
	if g.conMods {
		return []string{g.lista}
	}
	return []string{g.lista, "(filtra ? " + g.lista + ")"}
}

func (g *genEsq) palabrasDe() string { return "(palabras " + g.texto + ")" }

func (g *genEsq) generar() {
	m := g.m
	acc := m.Accion
	L, S, N, N2 := g.lista, g.texto, g.entero, g.entero2
	sep := `" "`
	for _, md := range m.Mods {
		if md.Texto != "" && md.Concepto == "unir" {
			sep = strconv.Quote(md.Texto)
		}
	}
	palabrasObj := m.Objeto == "palabras" || m.Elemento == "palabra"
	letrasObj := m.Objeto == "letras" || m.Elemento == "letra" || m.Elemento == "vocal" || m.Elemento == "consonante" ||
		m.Elemento == "digito" || m.Elemento == "mayuscula"
	switch acc {
	case "sumar":
		for _, f := range g.fuentes() {
			g.add("(suma %s)", f)
		}
		if N != "" && N2 != "" {
			g.add("(+ %s %s)", N, N2)
		}
		if N != "" && N2 == "" {
			g.add("(suma (rango 1 (+ %s 1)))", N)
			g.add("(sumaDigitos %s)", N)
		}
	case "contar":
		if L != "" {
			if g.hayMods() || !g.conMods {
				g.add("(contar %s %s)", g.pred(g.elem), L)
			}
			g.add("(largo %s)", L)
		}
		if S != "" {
			switch {
			case palabrasObj:
				if g.hayMods() {
					g.add("(contar %s %s)", g.pred(tS), g.palabrasDe())
				}
				g.add("(largo %s)", g.palabrasDe())
			default:
				if g.hayMods() || !g.conMods {
					g.add("(contar %s (runas %s))", g.pred(tR), S)
				}
				if letrasObj && !g.hayMods() {
					g.add("(contar (λ %s (esLetra %s)) (runas %s))", g.varLambda(), g.varLambda(), S)
				}
				g.add("(largoS %s)", S)
			}
			if S != "" && g.texto2 != "" {
				g.add("(contarSub %s %s)", S, g.texto2)
			}
		}
		if N != "" {
			g.add("(numDigitos %s)", N)
		}
	case "maximo":
		for _, f := range g.fuentes() {
			g.add("(maxL %s)", f)
		}
		if N != "" && N2 != "" {
			g.add("(max2 %s %s)", N, N2)
		}
		if S != "" {
			g.add("(masLarga %s)", g.palabrasDe())
		}
	case "minimo":
		for _, f := range g.fuentes() {
			g.add("(minL %s)", f)
		}
		if N != "" && N2 != "" {
			g.add("(min2 %s %s)", N, N2)
		}
		if S != "" {
			g.add("(masCorta %s)", g.palabrasDe())
		}
	case "promedio":
		for _, f := range g.fuentes() {
			g.add("(promedio %s)", f)
		}
	case "producto", "multiplicar":
		for _, f := range g.fuentes() {
			g.add("(producto %s)", f)
		}
		if N != "" && N2 != "" {
			g.add("(* %s %s)", N, N2)
		}
	case "restar":
		if N != "" && N2 != "" {
			g.add("(- %s %s)", N, N2)
		}
		if L != "" {
			g.add("(- (maxL %s) (minL %s))", L, L)
		}
	case "dividir":
		if N != "" && N2 != "" {
			g.add("(/ %s %s)", N, N2)
		}
	case "filtrar", "eliminar":
		quita := acc == "eliminar"
		if L != "" {
			g.add("(filtra %s %s)", g.predicado(g.elem, quita), L)
		}
		if S != "" {
			if palabrasObj {
				g.add("(filtra %s %s)", g.predicado(tS, quita), g.palabrasDe())
				g.add("(unir (filtra %s %s) %s)", g.predicado(tS, quita), g.palabrasDe(), sep)
			} else {
				g.add("(deRunas (filtra %s (runas %s)))", g.predicado(tR, quita), S)
			}
		}
	case "transformar", "doble", "triple", "cuadrado", "mitad":
		x := g.varLambda()
		cuerpo := "?"
		switch acc {
		case "doble":
			cuerpo = "(* " + x + " 2)"
		case "triple":
			cuerpo = "(* " + x + " 3)"
		case "cuadrado":
			cuerpo = "(* " + x + " " + x + ")"
		case "mitad":
			cuerpo = "(/ " + x + " 2)"
		}
		if L != "" {
			if cuerpo == "?" {
				g.add("(mapea ? %s)", L)
			} else {
				g.add("(mapea (λ %s %s) %s)", x, cuerpo, L)
			}
		}
		if N != "" && cuerpo != "?" {
			g.add(strings.ReplaceAll(cuerpo, x, N))
		}
		if S != "" && acc == "transformar" {
			g.add("(unir (mapea ? %s) %s)", g.palabrasDe(), sep)
		}
	case "ordenar", "ordenar_desc":
		op := "ordenar"
		if acc == "ordenar_desc" {
			op = "ordenarDesc"
		}
		if L != "" {
			g.add("(%s %s)", op, L)
		}
		if S != "" {
			g.add("(%s %s)", op, g.palabrasDe())
			g.add("(unir (%s %s) %s)", op, g.palabrasDe(), sep)
			g.add("(deRunas (%s (runas %s)))", op, S)
		}
	case "invertir":
		if L != "" {
			g.add("(invertir %s)", L)
		}
		if S != "" {
			g.add("(invertirS %s)", S)
			g.add("(unir (invertir %s) %s)", g.palabrasDe(), sep)
		}
		if N != "" {
			g.add("(invertirNum %s)", N)
		}
	case "invertir_numero":
		if N != "" {
			g.add("(invertirNum %s)", N)
		}
	case "mayusculas":
		if S != "" {
			g.add("(mayus %s)", S)
		}
	case "minusculas":
		if S != "" {
			g.add("(minus %s)", S)
		}
	case "titulo":
		if S != "" {
			g.add("(titulo %s)", S)
		}
	case "capitalizar":
		if S != "" {
			g.add("(capitalizar %s)", S)
			g.add("(titulo %s)", S)
		}
	case "quitar_espacios":
		if S != "" {
			g.add("(quitarEspacios %s)", S)
			g.add("(recortar %s)", S)
		}
	case "palindromo":
		if S != "" {
			g.add("(esPalindromoS %s)", S)
			g.add("(esPalindromoLetras %s)", S)
		}
		if N != "" {
			g.add("(esCapicua %s)", N)
		}
		if L != "" {
			g.add("(== %s (invertir %s))", L, L)
		}
	case "factorial":
		if N != "" {
			g.add("(factorial %s)", N)
			g.add("(producto (rango 1 (+ %s 1)))", N)
		}
	case "fibonacci":
		if N != "" {
			g.add("(fib %s)", N)
		}
	case "primos", "primo":
		if N != "" {
			g.add("(filtra (λ %s (esPrimo %s)) (rango 2 (+ %s 1)))", g.varLambda(), g.varLambda(), N)
			g.add("(esPrimo %s)", N)
		}
		if L != "" {
			g.add("(filtra (λ %s (esPrimo %s)) %s)", g.varLambda(), g.varLambda(), L)
			g.add("(contar (λ %s (esPrimo %s)) %s)", g.varLambda(), g.varLambda(), L)
		}
	case "mcd":
		if N != "" && N2 != "" {
			g.add("(mcd %s %s)", N, N2)
		}
	case "mcm":
		if N != "" && N2 != "" {
			g.add("(mcm %s %s)", N, N2)
		}
	case "digitos":
		if N != "" {
			g.add("(digitos %s)", N)
			g.add("(numDigitos %s)", N)
		}
	case "suma_digitos":
		if N != "" {
			g.add("(sumaDigitos %s)", N)
		}
	case "potencia":
		if N != "" && N2 != "" {
			g.add("(pot %s %s)", N, N2)
		}
		if N != "" {
			g.add("(* %s %s)", N, N)
		}
	case "raiz":
		if N != "" {
			g.add("(raizEntera %s)", N)
		}
	case "absoluto":
		if N != "" {
			g.add("(abs %s)", N)
		}
		if L != "" {
			g.add("(mapea (λ %s (abs %s)) %s)", g.varLambda(), g.varLambda(), L)
		}
	case "binario":
		if N != "" {
			g.add("(binario %s)", N)
		}
	case "longitud":
		if L != "" {
			g.add("(largo %s)", L)
		}
		if S != "" {
			g.add("(largoS %s)", S)
			g.add("(largos %s)", g.palabrasDe())
		}
		if N != "" {
			g.add("(numDigitos %s)", N)
		}
	case "unir", "concatenar":
		if L != "" && g.elem.Clase == nucleo.CString {
			g.add("(unir %s %s)", L, sep)
			g.add("(unir %s ?)", L)
		}
		if L != "" && g.elem.Clase == nucleo.CInt {
			x := g.varLambda()
			g.add("(unir (mapea (λ %s (itoa %s)) %s) %s)", x, x, L, sep)
			g.add("(unir (mapea (λ %s (itoa %s)) %s) ?)", x, x, L)
		}
		if L != "" && g.lista2 != "" {
			g.add("(concat %s %s)", L, g.lista2)
		}
		if S != "" && g.texto2 != "" {
			g.add("(concatS %s %s)", S, g.texto2)
		}
	case "separar":
		if S != "" {
			g.add("(palabras %s)", S)
			g.add("(dividir %s ?)", S)
		}
	case "frecuencia":
		if S != "" {
			g.add("(frecuencias %s)", g.palabrasDe())
			g.add("(frecuenciaLetras %s)", S)
		}
		if L != "" && g.elem.Clase == nucleo.CString {
			g.add("(frecuencias %s)", L)
		}
	case "primero", "ultimo":
		op := "primero"
		if acc == "ultimo" {
			op = "ultimo"
		}
		for _, f := range g.fuentes() {
			g.add("(%s %s)", op, f)
		}
		if S != "" {
			g.add("(%s %s)", op, g.palabrasDe())
		}
	case "sin_repetir":
		if L != "" {
			g.add("(unicos %s)", L)
		}
		if S != "" {
			g.add("(unicos %s)", g.palabrasDe())
			g.add("(deRunas (unicos (runas %s)))", S)
		}
	case "todos", "alguno", "ninguno", "comprobar":
		if L != "" {
			p := g.pred(g.elem)
			switch acc {
			case "todos", "comprobar":
				g.add("(todos %s %s)", p, L)
			case "alguno":
				g.add("(alguno %s %s)", p, L)
			case "ninguno":
				g.add("(no (alguno %s %s))", p, L)
			}
		}
		if N != "" && g.conMods {
			x := g.varLambda()
			for _, md := range modsDe(m) {
				if b, ok := cuerpoMod(md, x, tI); ok {
					g.add(strings.ReplaceAll(b, x, N))
				}
			}
		}
		if S != "" && acc == "comprobar" {
			g.add("(todos %s (runas %s))", g.pred(tR), S)
		}
	case "contiene", "buscar", "posicion":
		if L != "" && N != "" && g.elem.Clase == nucleo.CInt {
			if acc == "posicion" {
				g.add("(indice %s %s)", L, N)
			} else {
				g.add("(contiene %s %s)", L, N)
				g.add("(indice %s %s)", L, N)
			}
		}
		if S != "" && g.texto2 != "" {
			g.add("(contieneS %s %s)", S, g.texto2)
		}
	case "a_texto":
		if N != "" {
			g.add("(itoa %s)", N)
		}
	case "a_numero":
		if S != "" {
			g.add("(atoi %s)", S)
		}
	case "rango":
		if N != "" && N2 != "" {
			g.add("(rango %s (+ %s 1))", N, N2)
		}
		if N != "" {
			g.add("(rango 1 (+ %s 1))", N)
		}
	case "acumular":
		if L != "" {
			g.add("(sumaAcumulada %s)", L)
		}
	case "transponer":
		if g.matriz != "" {
			g.add("(transpuesta %s)", g.matriz)
		}
	case "aplanar":
		if g.matriz != "" {
			g.add("(aplanar %s)", g.matriz)
		}
	case "mas_largo", "mas_corto":
		op := "masLarga"
		if acc == "mas_corto" {
			op = "masCorta"
		}
		if S != "" {
			g.add("(%s %s)", op, g.palabrasDe())
		}
		if L != "" && g.elem.Clase == nucleo.CString {
			g.add("(%s %s)", op, L)
		}
	case "repetir":
		if S != "" && N != "" {
			g.add("(repetir %s %s)", S, N)
		}
	case "reemplazar":
		if S != "" {
			g.add("(reemplazar %s ? ?)", S)
		}
	case "tomar":
		if L != "" && N != "" {
			g.add("(tomar %s %s)", L, N)
		}
	case "quitar_primeros":
		if L != "" && N != "" {
			g.add("(quitar %s %s)", L, N)
		}
	case "agregar":
		if L != "" && N != "" {
			g.add("(agrega %s %s)", L, N)
		}
	}
	if g.matriz != "" && acc == "sumar" {
		g.add("(sumaFilas %s)", g.matriz)
		g.add("(suma (aplanar %s))", g.matriz)
	}
}

// ---- holes ----

type huecoInfo struct {
	tipo   nucleo.Tipo
	lambda *LambdaTipo // non-nil: the whole lambda is the hole
	prof   int         // enclosing lambda parameters
	vars   []int       // variable slots in scope from enclosing lambdas
	tvars  []nucleo.Tipo
}

// recorrerHuecos visits the holes of e in a fixed order (Hijos and Lambdas in Args order, depth first).
func recorrerHuecos(e *Expr, nParams int, f func(h huecoInfo)) {
	var rec func(x *Expr, prof int, vars []int, tvars []nucleo.Tipo)
	rec = func(x *Expr, prof int, vars []int, tvars []nucleo.Tipo) {
		if x == nil {
			return
		}
		if x.Hueco {
			f(huecoInfo{tipo: x.Tipo, prof: prof, vars: vars, tvars: tvars})
			return
		}
		if x.Op == nil {
			return
		}
		ih, il := 0, 0
		for i := range x.Op.Args {
			if x.Op.esLambda(i) {
				if il < len(x.Lambdas) {
					l := x.Lambdas[il]
					if l.Cuerpo == nil || l.Cuerpo.Hueco {
						lt := LambdaTipo{Params: l.Params, Res: x.Op.Lambdas[i].Res}
						f(huecoInfo{tipo: lt.Res, lambda: &lt, prof: prof, vars: vars, tvars: tvars})
					} else {
						nv := append([]int(nil), vars...)
						nt := append([]nucleo.Tipo(nil), tvars...)
						for k, t := range l.Params {
							nv = append(nv, nParams+prof+k)
							nt = append(nt, t)
						}
						rec(l.Cuerpo, prof+len(l.Params), nv, nt)
					}
				}
				il++
				continue
			}
			if ih < len(x.Hijos) {
				rec(x.Hijos[ih], prof, vars, tvars)
			}
			ih++
		}
	}
	rec(e, 0, nil, nil)
}

// rellenar returns a copy of e with its holes replaced, in recorrerHuecos order.
func rellenar(e *Expr, rellenos []*Expr) *Expr {
	k := 0
	var rec func(x *Expr) *Expr
	rec = func(x *Expr) *Expr {
		if x == nil {
			return nil
		}
		if x.Hueco {
			r := rellenos[k]
			k++
			return r
		}
		if x.Op == nil {
			return x
		}
		c := *x
		c.Hijos = make([]*Expr, len(x.Hijos))
		c.Lambdas = make([]*Lambda, len(x.Lambdas))
		ih, il := 0, 0
		for i := range x.Op.Args {
			if x.Op.esLambda(i) {
				if il < len(x.Lambdas) {
					l := x.Lambdas[il]
					nl := &Lambda{Params: l.Params, nombres: l.nombres}
					if l.Cuerpo == nil || l.Cuerpo.Hueco {
						nl.Cuerpo = rellenos[k]
						k++
					} else {
						nl.Cuerpo = rec(l.Cuerpo)
					}
					c.Lambdas[il] = nl
				}
				il++
				continue
			}
			if ih < len(x.Hijos) {
				c.Hijos[ih] = rec(x.Hijos[ih])
			}
			ih++
		}
		return &c
	}
	return rec(e)
}

func lambdasHueco(e *Expr) []LambdaTipo {
	var out []LambdaTipo
	recorrerHuecos(e, 0, func(h huecoInfo) {
		if h.lambda != nil {
			out = append(out, *h.lambda)
		}
	})
	return out
}

type relleno struct {
	e     *Expr
	costo int
	conc  uint64
}

const maxRellenosExpr = 4000

// rellenos lists the candidate fillers of one hole, cheapest first.
func (s *sesion) rellenos(h huecoInfo) []relleno {
	var out []relleno
	if h.lambda != nil {
		pl := s.pools[h.lambda.clave()]
		if pl == nil {
			return nil
		}
		for _, ids := range pl.porCosto {
			for _, id := range ids {
				cuerpo := pl.cuerpos[id]
				if h.prof > 0 {
					cuerpo = desplazar(cuerpo, s.nParams, h.prof)
				}
				out = append(out, relleno{e: cuerpo, costo: pl.costos[id], conc: pl.conc[id]})
			}
		}
		return out
	}
	t := idTipo(h.tipo)
	if t < 0 {
		return nil
	}
	if h.prof == 0 && s.mo != nil && s.mo.nivelHecho > 0 {
		mo := s.mo
		for c := 0; c < len(mo.nivel[t]); c++ {
			for _, id := range mo.nivel[t][c] {
				if len(out) >= maxRellenosExpr {
					return out
				}
				out = append(out, relleno{e: mo.expr(id), costo: mo.entradas[id].costo, conc: mo.entradas[id].conc})
			}
		}
		return out
	}
	for i, v := range h.vars {
		if mismoTipo(h.tvars[i], h.tipo) {
			out = append(out, relleno{e: nuevaVar(v, h.tipo, ""), costo: costoVarLambda})
		}
	}
	if h.prof == 0 {
		for i, p := range s.firma.Params {
			if mismoTipo(p.Tipo, h.tipo) {
				out = append(out, relleno{e: nuevaVar(i, p.Tipo, p.Nombre), costo: costoParam})
			}
		}
	}
	for _, c := range s.consts {
		if mismoTipo(c.Tipo, h.tipo) {
			out = append(out, relleno{e: c, costo: costoConst})
		}
	}
	return out
}

const maxEvalEsqueleto = 300000

// completar fills the holes of sk, cheapest combinations first, and returns the fillings that satisfy every
// example (with examples) or that have some result on the probes (without examples).
func (s *sesion) completar(sk *Expr, limite time.Time, aceptar func(e *Expr, vec []V) bool) []candSol {
	if sk == nil || !mismoTipo(sk.Tipo, s.firma.Res[0]) {
		return nil
	}
	var huecos []huecoInfo
	recorrerHuecos(sk, s.nParams, func(h huecoInfo) { huecos = append(huecos, h) })
	opciones := make([][]relleno, len(huecos))
	minimos := make([]int, len(huecos)+1)
	for i, h := range huecos {
		opciones[i] = s.rellenos(h)
		if len(opciones[i]) == 0 {
			return nil
		}
	}
	for i := len(huecos) - 1; i >= 0; i-- {
		minimos[i] = minimos[i+1] + opciones[i][0].costo
	}
	fijo := s.costoExpr(sk)
	var out []candSol
	elegidos := make([]*Expr, len(huecos))
	tope := s.op.MaxCosto
	evals := 0
	var rec func(i, costo int) bool
	rec = func(i, costo int) bool {
		if i == len(huecos) {
			evals++
			s.evalEsq++
			if evals&255 == 0 && (time.Now().After(limite) || s.ctx.Err() != nil) {
				return false
			}
			e := rellenar(sk, elegidos)
			vec, ok := s.evaluarCompleto(e)
			if ok && (aceptar == nil || aceptar(e, vec)) {
				out = append(out, candSol{expr: e, costo: costo, vec: vec})
				if len(s.sols) > 0 || len(out) > 0 {
					t := int(math.Ceil(float64(costo) * 1.15))
					if t < tope {
						tope = t
					}
				}
			}
			return evals < maxEvalEsqueleto
		}
		for _, r := range opciones[i] {
			if costo+r.costo+minimos[i+1] > tope {
				break
			}
			elegidos[i] = r.e
			if !rec(i+1, costo+r.costo) {
				return false
			}
		}
		return true
	}
	rec(0, fijo)
	sort.SliceStable(out, func(i, j int) bool { return out[i].costo < out[j].costo })
	return out
}

// evaluarCompleto runs e on every input; with examples, it fails fast on the first mismatch.
func (s *sesion) evaluarCompleto(e *Expr) (vec []V, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			vec, ok = nil, false
		}
	}()
	ev := compilar(e, 0)
	vec = make([]V, len(s.envs))
	algun := false
	for j, env := range s.envs {
		v, err := ev(env)
		if err != nil {
			if j < s.nEj {
				return nil, false
			}
			vec[j] = bottom
			continue
		}
		if j < s.nEj && !nucleo.Igual(v, s.esperado[j]) {
			return nil, false
		}
		vec[j] = v
		algun = true
	}
	return vec, algun
}

func (s *sesion) probarEsqueletos(sks []*Expr, limite time.Time) {
	conExpr := false
	for _, sk := range sks {
		recorrerHuecos(sk, s.nParams, func(h huecoInfo) {
			if h.lambda == nil && h.prof == 0 {
				conExpr = true
			}
		})
	}
	if conExpr && s.mo != nil {
		viejo := s.mo.limite
		s.mo.limite = limite
		s.mo.correr(min(20, s.op.MaxCosto))
		s.mo.limite = viejo
		if s.mo.parar && s.mo.motivo == "tiempo" && s.ctx.Err() == nil {
			s.mo.parar = false // only the sketch budget ran out
		}
	}
	for _, sk := range sks {
		if time.Now().After(limite) {
			break
		}
		for _, c := range s.completar(sk, limite, nil) {
			s.agregarSol(c.expr, c.costo, c.vec)
		}
	}
}

// CompletarEsqueleto fills the holes of one sketch from the lambda pools or the bank, cheapest first,
// within 1 s, and returns the fillings that satisfy every example.
func CompletarEsqueleto(ctx context.Context, sk *Expr, esp Especificacion, r *Registro, op Opciones) []Solucion {
	op = op.normalizar()
	if sk == nil || len(esp.Ejemplos) == 0 || !Sintetizable(esp.Firma) {
		return nil
	}
	limite := min(op.Limite, 2*time.Second)
	ctx, cancel := context.WithTimeout(ctx, limite)
	defer cancel()
	s, err := nuevaSesion(ctx, esp, r, op, true)
	if err != nil {
		return nil
	}
	s.extraLT = lambdasHueco(sk)
	s.construirPools(time.Now().Add(limite / 2))
	s.prepararMotor()
	s.probarEsqueletos([]*Expr{sk}, time.Now().Add(time.Second))
	return s.resultado("encontrado", time.Now()).Soluciones
}

// ---- DesdeConceptos ----

func pesosMarco(m *nucleo.Marco) map[string]float64 {
	w := map[string]float64{}
	if m.Accion != "" {
		w[m.Accion] = 1
	}
	for _, md := range m.Mods {
		if md.Concepto != "" && w[md.Concepto] < 0.8 {
			w[md.Concepto] = 0.8
		}
	}
	if m.Elemento != "" && w[m.Elemento] < 0.8 {
		w[m.Elemento] = 0.8
	}
	if m.Objeto != "" && w[m.Objeto] == 0 {
		w[m.Objeto] = 0.3
	}
	return w
}

func constantesMarco(m *nucleo.Marco) []V {
	var out []V
	for _, md := range m.Mods {
		for _, x := range md.Numeros {
			if x == math.Trunc(x) && math.Abs(x) < 1e15 {
				out = append(out, int(x))
			} else {
				out = append(out, x)
			}
		}
		if md.Texto != "" {
			out = append(out, md.Texto)
		}
	}
	return out
}

// sondasMarco builds probe inputs around the frame's numbers (so "mayores que 5" is told apart from
// "mayores que 0"), then fills up with the default probes.
func sondasMarco(m *nucleo.Marco, f nucleo.Firma) [][]nucleo.Valor {
	var nums []int
	for _, x := range constantesMarco(m) {
		if n, ok := x.(int); ok && n > -1000 && n < 1000 {
			nums = append(nums, n)
		}
	}
	var out [][]nucleo.Valor
	for i, n := range nums {
		if i >= 2 {
			break
		}
		fila := make([]nucleo.Valor, len(f.Params))
		ok := true
		for j, p := range f.Params {
			switch {
			case mismoTipo(p.Tipo, tLI):
				fila[j] = []nucleo.Valor{n - 1, n, n + 1, 2*n + 3, -n - 2}
			case mismoTipo(p.Tipo, tI):
				fila[j] = n + 1
			case mismoTipo(p.Tipo, tS):
				fila[j] = "un " + strings.Repeat("a", max(n, 0)) + " " + strings.Repeat("b", max(n+1, 1)) + " c"
			case mismoTipo(p.Tipo, tLS):
				fila[j] = []nucleo.Valor{strings.Repeat("a", max(n, 0)), strings.Repeat("b", max(n+1, 1)), "c"}
			default:
				ok = false
			}
		}
		if ok {
			out = append(out, fila)
		}
	}
	return append(out, SondasPorDefecto(f, nil, maxSondas-len(out))...)
}

// DesdeConceptos returns up to k programs for a request without examples: the frame's sketches with the
// modifiers as lambdas, then the smallest programs whose primitives cover every action and modifier concept
// (at most 50 000 candidates or 1 s). Cheapest first.
func DesdeConceptos(ctx context.Context, m *nucleo.Marco, f nucleo.Firma, r *Registro, k int) []Solucion {
	if m == nil || !Sintetizable(f) {
		return nil
	}
	if r == nil {
		r = Base()
	}
	if k <= 0 {
		k = 5
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	esp := Especificacion{Firma: f, Conceptos: pesosMarco(m), Constantes: constantesMarco(m), Sondas: sondasMarco(m, f)}
	op := Opciones{MaxCosto: 70, MaxBanco: 150000}.normalizar()
	s, err := nuevaSesion(ctx, esp, r, op, true)
	if err != nil || len(s.envs) == 0 {
		return nil
	}
	// required concepts: those some primitive can provide
	disponibles := map[string]bool{}
	for _, p := range s.prims {
		for _, c := range p.Conceptos {
			disponibles[c] = true
		}
	}
	var req []string
	for _, c := range append([]string{m.Accion}, func() []string {
		var cs []string
		for _, md := range modsDe(m) {
			cs = append(cs, md.Concepto)
		}
		return cs
	}()...) {
		if c != "" && disponibles[c] && len(req) < 64 {
			dup := false
			for _, x := range req {
				dup = dup || x == c
			}
			if !dup {
				req = append(req, c)
			}
		}
	}
	s.reqConc = req
	if s.reqConc == nil {
		s.reqConc = []string{}
	}
	todo := uint64(1)<<uint(len(req)) - 1
	sks := esqueletos(m, f, r, true)
	for _, sk := range sks {
		s.extraLT = append(s.extraLT, lambdasHueco(sk)...)
	}
	s.construirPools(time.Now().Add(500 * time.Millisecond))
	s.prepararMotor()
	cubre := func(e *Expr) int {
		tiene := map[string]bool{}
		for _, c := range conceptosDe(e) {
			tiene[c] = true
		}
		n := 0
		for _, c := range req {
			if tiene[c] {
				n++
			}
		}
		return n
	}
	constante := func(vec []V) bool {
		for _, v := range vec[1:] {
			if !igualExacto(v, vec[0]) {
				return false
			}
		}
		return len(s.envs) > 1
	}
	type candDC struct {
		candSol
		esq       bool
		cubre     int
		constante bool
	}
	var cands []candDC
	agregar := func(c candSol, esq bool) {
		n := candDC{candSol: c, esq: esq, cubre: cubre(c.expr), constante: constante(c.vec)}
		for i, o := range cands {
			if igualVec(o.vec, c.vec) {
				if n.cubre > o.cubre || n.cubre == o.cubre && (n.esq && !o.esq || n.esq == o.esq && c.costo < o.costo) {
					cands[i] = n
				}
				return
			}
		}
		cands = append(cands, n)
	}
	limiteEsq := time.Now().Add(500 * time.Millisecond)
	for _, sk := range sks {
		for _, c := range s.completar(sk, limiteEsq, nil) {
			agregar(c, true)
			break // the cheapest filling of each sketch
		}
	}
	if len(req) > 0 {
		mo := s.mo
		mo.limite = time.Now().Add(time.Second)
		visto := 0
		completos := 0
		for c := 1; c <= op.MaxCosto && !mo.parar && mo.explorados < 50000; c++ {
			mo.correr(c)
			for _, id := range mo.objetivos[visto:] {
				e := &mo.entradas[id]
				if e.conc&todo != todo {
					continue
				}
				x := mo.expr(id)
				if cubre(x) == len(req) {
					agregar(candSol{expr: x, costo: e.costo, vec: e.vec}, false)
					completos++
				}
			}
			visto = len(mo.objetivos)
			if completos >= 3*k {
				break
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.cubre != b.cubre {
			return a.cubre > b.cubre
		}
		if a.esq != b.esq {
			return a.esq
		}
		if a.constante != b.constante {
			return !a.constante
		}
		if a.costo != b.costo {
			return a.costo < b.costo
		}
		return Tamano(a.expr) < Tamano(b.expr)
	})
	var out []Solucion
	for _, c := range cands {
		if len(out) >= k {
			break
		}
		out = append(out, Solucion{Expr: c.expr, Costo: c.costo})
	}
	return out
}
