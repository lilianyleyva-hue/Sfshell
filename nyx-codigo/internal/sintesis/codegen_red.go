package sintesis

import (
	"strconv"

	"nyxcodigo/internal/nucleo"
)

// Reducers and collections over fused loops (§4.7 step 12).

var nombresReductor = map[string]string{
	"maxL": "mayor", "minL": "menor", "masLarga": "masLarga", "masCorta": "masCorta",
}

func comparacionExtremo(op string, x gx, m string) string {
	switch op {
	case "maxL":
		return x.en(4) + " > " + m
	case "minL":
		return x.en(4) + " < " + m
	case "masLarga":
		return "utf8.RuneCountInString(" + x.s + ") > utf8.RuneCountInString(" + m + ")"
	}
	return "utf8.RuneCountInString(" + x.s + ") < utf8.RuneCountInString(" + m + ")"
}

func ceroSuma(t nucleo.Tipo) string {
	if t.Clase == nucleo.CFloat {
		return "0.0"
	}
	return "0"
}

func conFiltro(etapas []etapa, l *Lambda) []etapa {
	out := make([]etapa, 0, len(etapas)+1)
	out = append(out, etapas...)
	return append(out, etapa{filtro: true, lam: l})
}

// reducir generates a reducer (or a collection when e itself is a filtra/mapea chain) as one fused loop.
func (fg *funcGen) reducir(e *Expr, b *bloque, env map[int]gx, prof int) gx {
	nombre := e.Op.Nombre
	var lista, ini *Expr
	var lam *Lambda
	switch nombre {
	case "contar", "todos", "alguno":
		lam, lista = e.Lambdas[0], e.Hijos[0]
	case "pliega":
		lam, ini, lista = e.Lambdas[0], e.Hijos[0], e.Hijos[1]
	case "filtra", "mapea":
		lista = e
	default:
		lista = e.Hijos[0]
	}
	fuente, etapas := desarmar(lista)
	elemT := tI
	if lista.Tipo.Elem != nil {
		elemT = *lista.Tipo.Elem
	}
	plano := len(etapas) == 0 && !esOp(fuente, "runas") && !esOp(fuente, "rango")
	slot := fg.nPar + prof
	switch nombre {
	case "suma", "producto":
		base, ini, op := "total", ceroSuma(elemT), " += "
		if nombre == "producto" {
			base, ini, op = "producto", "1", " *= "
			if elemT.Clase == nucleo.CFloat {
				ini = "1.0"
			}
		}
		n := fg.nuevo(base)
		fg.recorrer(b, fuente, etapas, env, prof, []string{n + " := " + ini}, 1, func(cur *bloque, x gx) {
			cur.linea(n + op + x.s)
		})
		return gx{s: n, prec: 7}
	case "largo", "contar":
		if nombre == "contar" {
			etapas = conFiltro(etapas, lam)
		}
		n := fg.nuevo("cuenta")
		fg.recorrer(b, fuente, etapas, env, prof, []string{n + " := 0"}, 0, func(cur *bloque, x gx) {
			cur.linea(n + "++")
		})
		return gx{s: n, prec: 7}
	case "alguno":
		n := fg.nuevo("hay")
		fg.recorrer(b, fuente, conFiltro(etapas, lam), env, prof, []string{n + " := false"}, 0, func(cur *bloque, x gx) {
			cur.linea(n + " = true")
			cur.linea("break")
		})
		return gx{s: n, prec: 7}
	case "todos":
		n := fg.nuevo("todos")
		fg.recorrer(b, fuente, etapas, env, prof, []string{n + " := true"}, usaVar(lam.Cuerpo, slot), func(cur *bloque, x gx) {
			np := fg.negado(lam, []gx{x}, cur, env, prof)
			cur.linea("if " + np.s + " {")
			cur.linea(n + " = false")
			cur.linea("break")
			cur.linea("}")
		})
		return gx{s: n, prec: 7}
	case "frecuencias":
		n := fg.nuevo("frecuencias")
		fg.recorrer(b, fuente, etapas, env, prof, []string{n + " := make(map[string]int)"}, 1, func(cur *bloque, x gx) {
			cur.linea(n + "[" + x.s + "]++")
		})
		return gx{s: n, prec: 7}
	case "filtra", "mapea":
		n := fg.nuevo(nombreColeccion(elemT))
		fg.recorrer(b, fuente, etapas, env, prof, []string{"var " + n + " " + e.Tipo.Go()}, 1, func(cur *bloque, x gx) {
			cur.linea(n + " = append(" + n + ", " + x.s + ")")
		})
		return gx{s: n, prec: 7}
	case "pliega":
		a := fg.expr(ini, b, env, prof)
		n := fg.nuevo("acumulado")
		acc := gx{s: n, prec: 7}
		fg.recorrer(b, fuente, etapas, env, prof, []string{n + " := " + a.s}, usaVar(lam.Cuerpo, slot+1), func(cur *bloque, x gx) {
			cuerpo := lam.Cuerpo
			if cuerpo.Op != nil && cuerpo.Op.Cuerpo == nil && len(cuerpo.Hijos) == 2 &&
				(cuerpo.Op.Nombre == "+" || cuerpo.Op.Nombre == "-" || cuerpo.Op.Nombre == "*") &&
				cuerpo.Hijos[0].esVar() && cuerpo.Hijos[0].Var == slot && usaVar(cuerpo.Hijos[1], slot) == 0 {
				nenv, nprof := fg.conLambda(lam, []gx{acc, x}, env, prof)
				r := fg.expr(cuerpo.Hijos[1], cur, nenv, nprof)
				cur.linea(n + " " + cuerpo.Op.Nombre + "= " + r.s)
				return
			}
			v := fg.aplicar(lam, []gx{acc, x}, cur, env, prof)
			cur.linea(n + " = " + v.s)
		})
		return acc
	case "maxL", "minL", "masLarga", "masCorta":
		if plano {
			xs := fg.identificador(fg.expr(fuente, b, env, prof), b, nombreLista(fuente))
			fg.guarda(b, "len("+xs.s+") == 0", "con la lista vacía")
			n := fg.nuevo(nombresReductor[nombre])
			b.linea(n + " := " + xs.s + "[0]")
			el := fg.nuevo(nombreElemento(elemT, fuente))
			b.linea("for _, " + el + " := range " + xs.s + "[1:] {")
			b.linea("if " + comparacionExtremo(nombre, gx{s: el, prec: 7}, n) + " {")
			b.linea(n + " = " + el)
			b.linea("}")
			b.linea("}")
			return gx{s: n, prec: 7}
		}
		n := fg.nuevo(nombresReductor[nombre])
		vacio := fg.nuevo("ninguno")
		fg.recorrer(b, fuente, etapas, env, prof, []string{n + " := " + elemT.Cero(), vacio + " := true"}, 2, func(cur *bloque, x gx) {
			cur.linea("if " + vacio + " || " + comparacionExtremo(nombre, x, n) + " {")
			cur.linea(n + ", " + vacio + " = " + x.s + ", false")
			cur.linea("}")
		})
		fg.guarda(b, vacio, "si no hay ninguno")
		return gx{s: n, prec: 7}
	case "promedio":
		total := fg.nuevo("total")
		if plano {
			xs := fg.identificador(fg.expr(fuente, b, env, prof), b, nombreLista(fuente))
			fg.guarda(b, "len("+xs.s+") == 0", "con la lista vacía")
			b.linea(total + " := " + ceroSuma(elemT))
			el := fg.nuevo(nombreElemento(elemT, fuente))
			b.linea("for _, " + el + " := range " + xs.s + " {")
			b.linea(total + " += " + el)
			b.linea("}")
			if elemT.Clase == nucleo.CFloat {
				return gx{s: total + " / float64(len(" + xs.s + "))", prec: 5}
			}
			return gx{s: "float64(" + total + ") / float64(len(" + xs.s + "))", prec: 5}
		}
		cuenta := fg.nuevo("cuenta")
		fg.recorrer(b, fuente, etapas, env, prof, []string{total + " := " + ceroSuma(elemT), cuenta + " := 0"}, 1, func(cur *bloque, x gx) {
			cur.linea(total + " += " + x.s)
			cur.linea(cuenta + "++")
		})
		fg.guarda(b, cuenta+" == 0", "si no hay ninguno")
		if elemT.Clase == nucleo.CFloat {
			return gx{s: total + " / float64(" + cuenta + ")", prec: 5}
		}
		return gx{s: "float64(" + total + ") / float64(" + cuenta + ")", prec: 5}
	}
	fg.g.fallo("no sé escribir «%s» como bucle", nombre)
	return gx{s: "nil", prec: 7}
}

func nombreColeccion(t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CString:
		return "palabras"
	case nucleo.CRune:
		return "letras"
	}
	return "resultado"
}

// cuantificarRaiz writes todos/alguno at the root of a function with early returns.
func (fg *funcGen) cuantificarRaiz(e *Expr, b *bloque, env map[int]gx, prof int) {
	lam := e.Lambdas[0]
	fuente, etapas := desarmar(e.Hijos[0])
	if e.Op.Nombre == "alguno" {
		fg.recorrer(b, fuente, conFiltro(etapas, lam), env, prof, nil, 0, func(cur *bloque, x gx) {
			cur.linea("return true")
		})
		b.linea("return false")
		return
	}
	fg.recorrer(b, fuente, etapas, env, prof, nil, usaVar(lam.Cuerpo, fg.nPar+prof), func(cur *bloque, x gx) {
		np := fg.negado(lam, []gx{x}, cur, env, prof)
		cur.linea("if " + np.s + " {")
		cur.linea("return false")
		cur.linea("}")
	})
	b.linea("return true")
}

// generarAprendida emits the helper function of a learned component once.
func (g *generador) generarAprendida(p *Primitiva, nombre string) {
	if g.vistas["aprendida:"+nombre] {
		return
	}
	g.vistas["aprendida:"+nombre] = true
	var f nucleo.Firma
	if p.firma != nil {
		f = *p.firma
	} else {
		for i, a := range p.Args {
			f.Params = append(f.Params, nucleo.Param{Nombre: "a" + strconv.Itoa(i), Tipo: a})
		}
	}
	f.Res = []nucleo.Tipo{p.Res}
	g.ayudas = append(g.ayudas, g.funcion(nombre, f, p.Cuerpo))
}
