package sintesis

import (
	"regexp"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Describir writes the program in simple Spanish: "la suma de los números pares de nums".
func Describir(e *Expr, f nucleo.Firma) string {
	d := &descriptor{nombres: map[int]string{}}
	for i, p := range f.Params {
		n := p.Nombre
		if n == "" {
			n = "la entrada " + strconv.Itoa(i+1)
		}
		d.nombres[i] = n
	}
	return d.expr(e)
}

type descriptor struct {
	nombres map[int]string
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
	return d.plantilla(e.Op.Frase, e)
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

func (d *descriptor) conVars(l *Lambda, f func() string) string {
	nombres := l.nombres
	if len(nombres) != len(l.Params) {
		nombres = nombresLambda(len(l.Params), nil)
	}
	base := varMinima(l.Cuerpo)
	viejos := map[int]string{}
	_ = base
	for i := range l.Params {
		slot := d.slotLambda(l, i)
		viejos[slot] = d.nombres[slot]
		d.nombres[slot] = nombres[i]
	}
	s := f()
	for k, v := range viejos {
		if v == "" {
			delete(d.nombres, k)
		} else {
			d.nombres[k] = v
		}
	}
	return s
}

// slotLambda finds the env slot of the i-th parameter of l from the variables its body uses.
func (d *descriptor) slotLambda(l *Lambda, i int) int {
	// parameters take the highest slots in scope: find the largest variable index used in the body
	max := -1
	var rec func(x *Expr)
	rec = func(x *Expr) {
		if x == nil {
			return
		}
		if x.esVar() && x.Var > max {
			max = x.Var
		}
		for _, h := range x.Hijos {
			rec(h)
		}
		for _, ll := range x.Lambdas {
			if ll != nil {
				rec(ll.Cuerpo)
			}
		}
	}
	rec(l.Cuerpo)
	inicio := max - len(l.Params) + 1
	if inicio < 0 {
		inicio = 0
	}
	return inicio + i
}

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
	esX := func(x *Expr) bool { return x.esVar() && usaVar(b, x.Var) > 0 && x.Var == d.slotLambda(l, 0) }
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
	if len(b.Hijos) == 2 && b.Hijos[1].EsConst {
		c := d.expr(b.Hijos[1])
		x := b.Hijos[0]
		if esX(x) {
			switch p.Nombre {
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
			switch p.Nombre {
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
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(p.Nombre) + `\b`)
		if re.MatchString(frase) {
			usado = true
			frase = re.ReplaceAllString(frase, "{"+strconv.Itoa(i)+"}")
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
