package sintesis

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Describir writes the program in simple Spanish: "la suma de los números pares de nums".
func Describir(e *Expr, f nucleo.Firma) string {
	if demasiadoHondo(e) {
		return "un programa demasiado anidado para describirlo"
	}
	d := &descriptor{nombres: map[int]string{}, nPar: len(f.Params)}
	for i, p := range f.Params {
		n := p.Nombre
		if n == "" {
			n = "la entrada " + strconv.Itoa(i+1)
		}
		d.nombres[i] = n
	}
	return d.expr(e)
}

// descriptor walks a program keeping the names of the variables in scope: the parameters take slots
// 0..nPar-1 and the parameters of each enclosing lambda the next ones (prof of them in scope).
type descriptor struct {
	nombres map[int]string
	nPar    int
	prof    int
}

func (d *descriptor) expr(e *Expr) string {
	switch {
	case e == nil:
		return "?"
	case e.Hueco:
		return "algo"
	case e.EsConst:
		if e.Tipo.Clase == nucleo.CString {
			return strconv.Quote(tex(e.Const))
		}
		return nucleo.FormatoHumano(e.Const, e.Tipo)
	case e.Op == nil:
		if n, ok := d.nombres[e.Var]; ok {
			return n
		}
		if e.nombre != "" {
			return e.nombre
		}
		return "x"
	}
	if s, ok := d.especial(e); ok {
		return s
	}
	if s, ok := d.letras(e); ok {
		return s
	}
	if s, ok := d.rango(e); ok {
		return s
	}
	if strings.Contains(e.Op.Frase, "palabra") && len(e.Hijos) == 1 && len(e.Lambdas) == 0 {
		// "la palabra más larga de (las palabras de s)" → "la palabra más larga de s"
		if h := e.Hijos[0]; h.Op != nil && h.Op.Nombre == "palabras" && len(h.Hijos) == 1 {
			return strings.ReplaceAll(e.Op.Frase, "{0}", d.expr(h.Hijos[0]))
		}
	}
	return d.plantilla(e.Op.Frase, e)
}

// rango describes (rango a (+ n 1)) as "los números desde a hasta n".
func (d *descriptor) rango(e *Expr) (string, bool) {
	if e.Op.Nombre != "rango" || len(e.Hijos) != 2 {
		return "", false
	}
	fin := e.Hijos[1]
	if fin.Op == nil || fin.Op.Nombre != "+" || len(fin.Hijos) != 2 {
		return "", false
	}
	a, b := fin.Hijos[0], fin.Hijos[1]
	if a.EsConst {
		a, b = b, a
	}
	if !b.EsConst || ent(b.Const) != 1 || b.Tipo.Clase != nucleo.CInt {
		return "", false
	}
	return "los números desde " + d.expr(e.Hijos[0]) + " hasta " + d.expr(a), true
}

// volteada rewrites "3 < x" as "x > 3" so comparisons read with the variable first.
func volteada(e *Expr) (nombre string, x, c *Expr, ok bool) {
	if e == nil || e.Op == nil || len(e.Hijos) != 2 {
		return "", nil, nil, false
	}
	nombre, x, c = e.Op.Nombre, e.Hijos[0], e.Hijos[1]
	if c.EsConst {
		return nombre, x, c, true
	}
	if !x.EsConst {
		return "", nil, nil, false
	}
	vuelta := map[string]string{"<": ">", ">": "<", "<=": ">=", ">=": "<=", "==": "==", "!=": "!="}
	nv, hay := vuelta[nombre]
	if !hay {
		return "", nil, nil, false
	}
	return nv, c, x, true
}

var reHueco = regexp.MustCompile(`\{(L?)(\d+)\}`)

func (d *descriptor) plantilla(frase string, e *Expr) string {
	if frase == "" {
		frase = e.Op.Nombre
		for i := range e.Op.Args {
			if i == 0 {
				frase += " de {0}"
			} else {
				frase += " y {" + strconv.Itoa(i) + "}"
			}
		}
	}
	// {i} counts every argument position; map it to Hijos / Lambdas
	return reHueco.ReplaceAllStringFunc(frase, func(m string) string {
		sub := reHueco.FindStringSubmatch(m)
		i, _ := strconv.Atoi(sub[2])
		if sub[1] == "L" {
			if i < len(e.Lambdas) {
				return d.lambda(e.Lambdas[i])
			}
			return "?"
		}
		ih, il := 0, 0
		for k := range e.Op.Args {
			if e.Op.esLambda(k) {
				if k == i && il < len(e.Lambdas) {
					return d.lambda(e.Lambdas[il])
				}
				il++
				continue
			}
			if k == i && ih < len(e.Hijos) {
				return d.expr(e.Hijos[ih])
			}
			ih++
		}
		return "?"
	})
}

// conVars describes something inside lambda l (an argument of the node being described): l's parameters
// take the slots after those in scope, named as l names them.
func (d *descriptor) conVars(l *Lambda, f func() string) string {
	nombres := l.nombres
	if len(nombres) != len(l.Params) {
		nombres = nombresLambda(len(l.Params), nil)
	}
	viejos := map[int]string{}
	for i := range l.Params {
		slot := d.slotLambda(i)
		viejos[slot] = d.nombres[slot]
		d.nombres[slot] = nombres[i]
	}
	d.prof += len(l.Params)
	s := f()
	d.prof -= len(l.Params)
	for k, v := range viejos {
		if v == "" {
			delete(d.nombres, k)
		} else {
			d.nombres[k] = v
		}
	}
	return s
}

// slotLambda is the env slot of the i-th parameter of a lambda argument of the node being described.
func (d *descriptor) slotLambda(i int) int { return d.nPar + d.prof + i }

func (d *descriptor) lambda(l *Lambda) string {
	if l == nil || l.Cuerpo == nil || l.Cuerpo.Hueco {
		return "algo"
	}
	return d.conVars(l, func() string { return d.expr(l.Cuerpo) })
}

type sustantivoTipo struct {
	plural string
	fem    bool
}

func sustantivoDe(t nucleo.Tipo) sustantivoTipo {
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat:
		return sustantivoTipo{"números", false}
	case nucleo.CString:
		return sustantivoTipo{"palabras", true}
	case nucleo.CRune:
		return sustantivoTipo{"caracteres", false}
	case nucleo.CBool:
		return sustantivoTipo{"valores", false}
	case nucleo.CLista:
		return sustantivoTipo{"listas", true}
	}
	return sustantivoTipo{"elementos", false}
}

func articulo(fem bool) string {
	if fem {
		return "las"
	}
	return "los"
}

// cualidad describes a one-parameter predicate lambda as an adjective ("pares", "mayores que 5") or, for
// rune predicates, as a noun ("vocales").
func (d *descriptor) cualidad(l *Lambda) (adj string, sust *sustantivoTipo, ok bool) {
	if l == nil || len(l.Params) != 1 || l.Cuerpo == nil || l.Cuerpo.Op == nil {
		return "", nil, false
	}
	b := l.Cuerpo
	esX := func(x *Expr) bool { return x.esVar() && x.Var == d.slotLambda(0) }
	negar := false
	if b.Op.Nombre == "no" && len(b.Hijos) == 1 && b.Hijos[0].Op != nil {
		negar = true
		b = b.Hijos[0]
	}
	pre := ""
	if negar {
		pre = "no "
	}
	p := b.Op
	if len(b.Hijos) == 1 && esX(b.Hijos[0]) {
		if p.sustantivo != "" {
			s := sustantivoTipo{pre + p.sustantivo, p.femenino}
			if negar {
				s.plural = "caracteres que no son " + p.sustantivo
				s.fem = false
			}
			return "", &s, true
		}
		if p.adjetivo != "" {
			return pre + p.adjetivo, nil, true
		}
	}
	if nombre, x, ce, hay := volteada(b); hay {
		c := d.expr(ce)
		if esX(x) {
			switch nombre {
			case ">":
				return pre + "mayores que " + c, nil, true
			case "<":
				return pre + "menores que " + c, nil, true
			case ">=":
				return pre + "mayores o iguales que " + c, nil, true
			case "<=":
				return pre + "menores o iguales que " + c, nil, true
			case "==":
				return pre + "iguales a " + c, nil, true
			case "!=":
				return pre + "distintos de " + c, nil, true
			case "divisible":
				return pre + "divisibles por " + c, nil, true
			case "empiezaCon":
				return pre + "que empiezan por " + c, nil, true
			case "terminaCon":
				return pre + "que terminan en " + c, nil, true
			}
		}
		if x.Op != nil && x.Op.Nombre == "largoS" && len(x.Hijos) == 1 && esX(x.Hijos[0]) {
			switch nombre {
			case ">":
				return pre + "de más de " + c + " letras", nil, true
			case ">=":
				return pre + "de " + c + " letras o más", nil, true
			case "<":
				return pre + "de menos de " + c + " letras", nil, true
			case "<=":
				return pre + "de " + c + " letras o menos", nil, true
			case "==":
				return pre + "de " + c + " letras", nil, true
			}
		}
	}
	return "", nil, false
}

// fuente describes the list a higher-order primitive walks: (runas s) is just "s".
func (d *descriptor) fuente(xs *Expr) (desc string, elem nucleo.Tipo) {
	if xs.Op != nil && xs.Op.Nombre == "runas" && len(xs.Hijos) == 1 {
		return d.expr(xs.Hijos[0]), tR
	}
	if xs.Op != nil && xs.Op.Nombre == "palabras" && len(xs.Hijos) == 1 {
		return d.expr(xs.Hijos[0]), tS
	}
	if xs.Tipo.Elem != nil {
		elem = *xs.Tipo.Elem
	}
	return d.expr(xs), elem
}

func (d *descriptor) especial(e *Expr) (string, bool) {
	p := e.Op
	if len(e.Lambdas) != 1 || len(e.Hijos) == 0 {
		return "", false
	}
	l := e.Lambdas[0]
	xs := e.Hijos[len(e.Hijos)-1]
	src, elem := d.fuente(xs)
	sust := sustantivoDe(elem)
	adj, sn, ok := d.cualidad(l)
	grupo := ""
	if ok {
		if sn != nil {
			grupo = sn.plural
			sust.fem = sn.fem
		} else {
			grupo = sust.plural + " " + adj
		}
	}
	nombreX := "x"
	if len(l.nombres) > 0 {
		nombreX = l.nombres[0]
	}
	switch p.Nombre {
	case "filtra":
		if ok {
			return articulo(sust.fem) + " " + grupo + " de " + src, true
		}
		return articulo(sust.fem) + " " + sust.plural + " " + nombreX + " de " + src + " tales que " + d.lambda(l), true
	case "contar":
		if ok {
			cuantos := "cuántos "
			if sust.fem {
				cuantos = "cuántas "
			}
			return cuantos + grupo + " hay en " + src, true
		}
		return "cuántos elementos " + nombreX + " de " + src + " cumplen que " + d.lambda(l), true
	case "todos":
		if ok && sn == nil {
			return "todos los elementos de " + src + " son " + adj, true
		}
		return "todos los elementos " + nombreX + " de " + src + " cumplen que " + d.lambda(l), true
	case "alguno":
		if ok {
			return "hay " + grupo + " en " + src, true
		}
		return "algún elemento " + nombreX + " de " + src + " cumple que " + d.lambda(l), true
	case "mapea":
		return d.lambda(l) + " para cada " + nombreX + " de " + src, true
	}
	return "", false
}

// fraseAprendida turns a description into a phrase template: parameter names become {i}.
func fraseAprendida(nombre, descripcion string, f nucleo.Firma) string {
	frase := strings.TrimSpace(descripcion)
	usado := false
	for i, p := range f.Params {
		if p.Nombre == "" {
			continue
		}
		if nueva, hay := cambiarPalabra(frase, p.Nombre, "{"+strconv.Itoa(i)+"}"); hay {
			usado = true
			frase = nueva
		}
	}
	if frase == "" || !usado {
		partes := make([]string, len(f.Params))
		for i := range f.Params {
			partes[i] = "{" + strconv.Itoa(i) + "}"
		}
		return nombre + "(" + strings.Join(partes, ", ") + ")"
	}
	return frase
}

// cambiarPalabra replaces the whole-word occurrences of palabra in texto (Unicode-aware: "s" in "más" or
// "n" in "también" is not a word of its own) and reports whether there was one.
func cambiarPalabra(texto, palabra, por string) (string, bool) {
	if palabra == "" {
		return texto, false
	}
	esLetra := func(r rune) bool {
		return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
	}
	var sb strings.Builder
	hay := false
	for i := 0; i < len(texto); {
		j := strings.Index(texto[i:], palabra)
		if j < 0 {
			sb.WriteString(texto[i:])
			break
		}
		j += i
		fin := j + len(palabra)
		antes, _ := utf8.DecodeLastRuneInString(texto[:j])
		despues, _ := utf8.DecodeRuneInString(texto[fin:])
		sb.WriteString(texto[i:j])
		if (j == 0 || !esLetra(antes)) && (fin == len(texto) || !esLetra(despues)) {
			sb.WriteString(por)
			hay = true
		} else {
			sb.WriteString(palabra)
		}
		i = fin
	}
	return sb.String(), hay
}

// letras describes comparisons of a word length with a number: "p tiene más de 3 letras".
func (d *descriptor) letras(e *Expr) (string, bool) {
	nombre, lx, ce, ok := volteada(e)
	if !ok || lx.Op == nil || lx.Op.Nombre != "largoS" || len(lx.Hijos) != 1 {
		return "", false
	}
	x, c := d.expr(lx.Hijos[0]), d.expr(ce)
	switch nombre {
	case ">":
		return x + " tiene más de " + c + " letras", true
	case "<":
		return x + " tiene menos de " + c + " letras", true
	case ">=":
		return x + " tiene " + c + " letras o más", true
	case "<=":
		return x + " tiene " + c + " letras o menos", true
	case "==":
		return x + " tiene " + c + " letras", true
	}
	return "", false
}
