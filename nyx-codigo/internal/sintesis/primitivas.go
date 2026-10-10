package sintesis

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// The shipped primitives (§4.7). Each has an interpreter (Eval), a Go template or a dedicated code generator
// (codegen.go), a Spanish phrase and concept tags. Polymorphic ones are registered per element type.

type opc func(p *Primitiva)

func conm(p *Primitiva)    { p.conmutativa = true }
func perez(p *Primitiva)   { p.perezosa = true }
func invol(p *Primitiva)   { p.involutiva = true }
func idem(p *Primitiva)    { p.idempotente = true }
func parc(p *Primitiva)    { p.Parcial = true }
func prec(n int) opc       { return func(p *Primitiva) { p.prec = n } }
func adj(s string) opc     { return func(p *Primitiva) { p.adjetivo = s } }
func imp(ps ...string) opc { return func(p *Primitiva) { p.Imports = append(p.Imports, ps...) } }
func ayu(nombre string) opc {
	return func(p *Primitiva) {
		p.Ayudante = ayudantes[nombre]
		p.Imports = append(p.Imports, importsAyudantes[nombre]...)
	}
}
func sust(s string, f bool) opc { return func(p *Primitiva) { p.sustantivo, p.femenino = s, f } }

type evalFn = func(a []V, l []Funcion) (V, error)

func (r *Registro) def(nombre string, args []nucleo.Tipo, res nucleo.Tipo, costo float64, conceptos []string,
	frase, goTpl string, eval evalFn, opcs ...opc) *Primitiva {
	p := &Primitiva{Nombre: nombre, Args: args, Res: res, Costo: costo, Conceptos: conceptos, Frase: frase,
		Go: goTpl, Eval: eval, prec: 7}
	for _, o := range opcs {
		o(p)
	}
	r.Agregar(p)
	return p
}

func (r *Registro) defL(nombre string, args []nucleo.Tipo, lambdas []LambdaTipo, res nucleo.Tipo, costo float64,
	conceptos []string, frase, goTpl string, eval evalFn, opcs ...opc) *Primitiva {
	p := &Primitiva{Nombre: nombre, Args: args, Lambdas: lambdas, Res: res, Costo: costo, Conceptos: conceptos,
		Frase: frase, Go: goTpl, Eval: eval, prec: 7}
	for _, o := range opcs {
		o(p)
	}
	r.Agregar(p)
	return p
}

func ts(t ...nucleo.Tipo) []nucleo.Tipo { return t }

func lt(res nucleo.Tipo, params ...nucleo.Tipo) LambdaTipo {
	return LambdaTipo{Params: params, Res: res}
}

func c(cs ...string) []string { return cs }

// int binary op with overflow check
func opEnt(f func(a, b int) (int, bool)) evalFn {
	return func(a []V, _ []Funcion) (V, error) {
		r, ok := f(ent(a[0]), ent(a[1]))
		if !ok {
			return nil, ErrIndefinido
		}
		return r, nil
	}
}

func predEnt(f func(n int) bool) evalFn {
	return func(a []V, _ []Funcion) (V, error) { return f(ent(a[0])), nil }
}

func predRuna(f func(r rune) bool) evalFn {
	return func(a []V, _ []Funcion) (V, error) { return f(rune(ent(a[0]))), nil }
}

func funTexto(f func(s string) string) evalFn {
	return func(a []V, _ []Funcion) (V, error) {
		s := f(tex(a[0]))
		if len(s) > maxTexto {
			return nil, ErrIndefinido
		}
		return s, nil
	}
}

func lista(xs []int) []V {
	out := make([]V, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func registrarBase(r *Registro) {
	registrarEnteros(r)
	registrarListas(r)
	registrarTextos(r)
	registrarOtros(r)
}

func registrarEnteros(r *Registro) {
	r.def("+", ts(tI, tI), tI, 6, c("sumar"), "{0} más {1}", "{0:4} + {1:5}", opEnt(sumaSegura), conm, prec(4))
	r.def("-", ts(tI, tI), tI, 7, c("restar"), "{0} menos {1}", "{0:4} - {1:5}", opEnt(restaSegura), prec(4))
	r.def("*", ts(tI, tI), tI, 6, c("multiplicar", "doble", "triple", "cuadrado"), "{0} por {1}", "{0:5} * {1:6}",
		opEnt(multSegura), conm, prec(5))
	r.def("/", ts(tI, tI), tI, 8, c("dividir", "mitad"), "{0} entre {1}", "{0:5} / {1:6}", opEnt(func(a, b int) (int, bool) {
		if b == 0 || (b == -1 && a == -a && a != 0) {
			return 0, false
		}
		return a / b, true
	}), parc, prec(5))
	r.def("%", ts(tI, tI), tI, 8, c("dividir", "divisible_por"), "el resto de dividir {0} entre {1}", "{0:5} % {1:6}",
		opEnt(func(a, b int) (int, bool) {
			if b == 0 {
				return 0, false
			}
			if b == -1 {
				return 0, true
			}
			return a % b, true
		}), parc, prec(5))
	r.def("neg", ts(tI), tI, 8, c("restar", "negativo"), "menos {0}", "-{0:7}", func(a []V, _ []Funcion) (V, error) {
		n := ent(a[0])
		if n == -n && n != 0 {
			return nil, ErrIndefinido
		}
		return -n, nil
	}, prec(6), invol)
	r.def("abs", ts(tI), tI, 8, c("absoluto"), "el valor absoluto de {0}", "abs({0})", func(a []V, _ []Funcion) (V, error) {
		n, ok := absInt(ent(a[0]))
		if !ok {
			return nil, ErrIndefinido
		}
		return n, nil
	}, ayu("abs"), idem)
	r.def("min2", ts(tI, tI), tI, 9, c("minimo"), "el menor entre {0} y {1}", "min({0}, {1})", opEnt(func(a, b int) (int, bool) { return min(a, b), true }), conm)
	r.def("max2", ts(tI, tI), tI, 9, c("maximo"), "el mayor entre {0} y {1}", "max({0}, {1})", opEnt(func(a, b int) (int, bool) { return max(a, b), true }), conm)
	r.def("pot", ts(tI, tI), tI, 10, c("potencia", "cuadrado"), "{0} elevado a {1}", "potencia({0}, {1})", opEnt(potencia), parc, ayu("potencia"))

	cmp := func(nombre, op, frase string, costo float64, conceptos []string, f func(a, b int) bool, extra ...opc) {
		o := append([]opc{prec(3)}, extra...)
		r.def(nombre, ts(tI, tI), tB, costo, conceptos, frase, "{0:4} "+op+" {1:4}", func(a []V, _ []Funcion) (V, error) {
			return f(ent(a[0]), ent(a[1])), nil
		}, o...)
	}
	cmp("==", "==", "{0} es igual a {1}", 8, c("igual_a", "comparar"), func(a, b int) bool { return a == b }, conm)
	cmp("!=", "!=", "{0} es distinto de {1}", 9, c("distinto_de", "comparar"), func(a, b int) bool { return a != b }, conm)
	cmp("<", "<", "{0} es menor que {1}", 7, c("menor_que", "comparar"), func(a, b int) bool { return a < b })
	cmp("<=", "<=", "{0} es menor o igual que {1}", 9, c("menor_que", "comparar"), func(a, b int) bool { return a <= b })
	cmp(">", ">", "{0} es mayor que {1}", 7, c("mayor_que", "comparar"), func(a, b int) bool { return a > b })
	cmp(">=", ">=", "{0} es mayor o igual que {1}", 9, c("mayor_que", "comparar"), func(a, b int) bool { return a >= b })
	for _, t := range []nucleo.Tipo{tS} {
		r.def("==", ts(t, t), tB, 9, c("igual_a", "comparar"), "{0} es igual a {1}", "{0:4} == {1:4}", func(a []V, _ []Funcion) (V, error) {
			return tex(a[0]) == tex(a[1]), nil
		}, conm, prec(3))
		r.def("!=", ts(t, t), tB, 10, c("distinto_de", "comparar"), "{0} es distinto de {1}", "{0:4} != {1:4}", func(a []V, _ []Funcion) (V, error) {
			return tex(a[0]) != tex(a[1]), nil
		}, conm, prec(3))
	}
	for _, t := range []nucleo.Tipo{tLI, tLS} {
		r.def("==", ts(t, t), tB, 10, c("igual_a", "comparar"), "{0} es igual a {1}", "slices.Equal({0}, {1})", func(a []V, _ []Funcion) (V, error) {
			return igualExacto(a[0], a[1]), nil
		}, conm, imp("slices"))
	}

	// bool
	r.def("y", ts(tB, tB), tB, 9, nil, "{0} y {1}", "{0:2} && {1:3}", func(a []V, _ []Funcion) (V, error) {
		return boo(a[0]) && boo(a[1]), nil
	}, conm, perez, prec(2))
	r.def("o", ts(tB, tB), tB, 9, nil, "{0} o {1}", "{0:1} || {1:2}", func(a []V, _ []Funcion) (V, error) {
		return boo(a[0]) || boo(a[1]), nil
	}, conm, perez, prec(1))
	r.def("no", ts(tB), tB, 8, nil, "no es cierto que {0}", "!{0:7}", func(a []V, _ []Funcion) (V, error) {
		return !boo(a[0]), nil
	}, prec(6), invol)
	for _, t := range []nucleo.Tipo{tI, tB, tS, tR, tF, tLI, tLS} {
		r.def("si", ts(tB, t, t), t, 25, nil, "si {0}, {1}; si no, {2}", "", func(a []V, _ []Funcion) (V, error) {
			if boo(a[0]) {
				return a[1], nil
			}
			return a[2], nil
		}, perez)
	}

	// int predicates
	r.def("esPar", ts(tI), tB, 6, c("par"), "{0} es par", "{0:5}%2 == 0", predEnt(func(n int) bool { return n%2 == 0 }), prec(3), adj("pares"))
	r.def("esImpar", ts(tI), tB, 7, c("impar"), "{0} es impar", "{0:5}%2 != 0", predEnt(func(n int) bool { return n%2 != 0 }), prec(3), adj("impares"))
	r.def("esPositivo", ts(tI), tB, 7, c("positivo"), "{0} es positivo", "{0:4} > 0", predEnt(func(n int) bool { return n > 0 }), prec(3), adj("positivos"))
	r.def("esNegativo", ts(tI), tB, 7, c("negativo"), "{0} es negativo", "{0:4} < 0", predEnt(func(n int) bool { return n < 0 }), prec(3), adj("negativos"))
	r.def("esCero", ts(tI), tB, 8, c("cero"), "{0} es cero", "{0:4} == 0", predEnt(func(n int) bool { return n == 0 }), prec(3), adj("iguales a cero"))
	r.def("esPrimo", ts(tI), tB, 8, c("primo", "primos"), "{0} es primo", "esPrimo({0})", predEnt(esPrimo), ayu("esPrimo"), adj("primos"))
	r.def("divisible", ts(tI, tI), tB, 9, c("divisible_por"), "{0} es divisible por {1}", "{0:5}%{1:6} == 0", func(a []V, _ []Funcion) (V, error) {
		k := ent(a[1])
		if k == 0 {
			return nil, ErrIndefinido
		}
		if k == -1 {
			return true, nil
		}
		return ent(a[0])%k == 0, nil
	}, parc, prec(3))
	r.def("esCuadrado", ts(tI), tB, 9, c("cuadrado", "raiz"), "{0} es un cuadrado perfecto", "esCuadrado({0})", predEnt(func(n int) bool {
		if n < 0 {
			return false
		}
		s := raizEntera(n)
		return s*s == n
	}), ayu("esCuadrado"), adj("cuadrados perfectos"))
	r.def("esCapicua", ts(tI), tB, 9, c("palindromo", "invertir_numero"), "{0} es capicúa", "esCapicua({0})", predEnt(func(n int) bool {
		if n < 0 {
			return false
		}
		ds, _ := digitosDe(n)
		for i, j := 0, len(ds)-1; i < j; i, j = i+1, j-1 {
			if ds[i] != ds[j] {
				return false
			}
		}
		return true
	}), ayu("esCapicua"), adj("capicúas"))

	// number functions
	r.def("factorial", ts(tI), tI, 8, c("factorial"), "el factorial de {0}", "factorial({0})", func(a []V, _ []Funcion) (V, error) {
		n := ent(a[0])
		if n < 0 || n > 20 {
			return nil, ErrIndefinido
		}
		f := 1
		for i := 2; i <= n; i++ {
			f *= i
		}
		return f, nil
	}, parc, ayu("factorial"))
	r.def("fib", ts(tI), tI, 9, c("fibonacci"), "el término {0} de Fibonacci", "fibonacci({0})", func(a []V, _ []Funcion) (V, error) {
		n := ent(a[0])
		if n < 0 || n > 90 {
			return nil, ErrIndefinido
		}
		x, y := 0, 1
		for i := 0; i < n; i++ {
			x, y = y, x+y
		}
		return x, nil
	}, parc, ayu("fibonacci"))
	r.def("mcd", ts(tI, tI), tI, 9, c("mcd"), "el máximo común divisor de {0} y {1}", "mcd({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		x, y := ent(a[0]), ent(a[1])
		if x == -x && x != 0 || y == -y && y != 0 {
			return nil, ErrIndefinido
		}
		return mcd(x, y), nil
	}, conm, ayu("mcd"))
	r.def("mcm", ts(tI, tI), tI, 10, c("mcm"), "el mínimo común múltiplo de {0} y {1}", "mcm({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		x, y := ent(a[0]), ent(a[1])
		if x == 0 || y == 0 {
			return 0, nil
		}
		ax, ok1 := absInt(x)
		ay, ok2 := absInt(y)
		if !ok1 || !ok2 {
			return nil, ErrIndefinido
		}
		m, ok := multSegura(ax/mcd(ax, ay), ay)
		if !ok {
			return nil, ErrIndefinido
		}
		return m, nil
	}, conm, parc, ayu("mcm"))
	r.def("digitos", ts(tI), tLI, 9, c("digitos"), "las cifras de {0}", "digitos({0})", func(a []V, _ []Funcion) (V, error) {
		ds, ok := digitosDe(ent(a[0]))
		if !ok {
			return nil, ErrIndefinido
		}
		return lista(ds), nil
	}, ayu("digitos"))
	r.def("sumaDigitos", ts(tI), tI, 8, c("suma_digitos", "digitos"), "la suma de las cifras de {0}", "sumaDigitos({0})", func(a []V, _ []Funcion) (V, error) {
		ds, ok := digitosDe(ent(a[0]))
		if !ok {
			return nil, ErrIndefinido
		}
		s := 0
		for _, d := range ds {
			s += d
		}
		return s, nil
	}, ayu("sumaDigitos"))
	r.def("invertirNum", ts(tI), tI, 9, c("invertir_numero", "invertir"), "{0} con las cifras al revés", "invertirNumero({0})", func(a []V, _ []Funcion) (V, error) {
		n := ent(a[0])
		neg := n < 0
		if n == -n && n != 0 {
			return nil, ErrIndefinido
		}
		if neg {
			n = -n
		}
		r := 0
		for n > 0 {
			var ok bool
			if r, ok = multSegura(r, 10); !ok {
				return nil, ErrIndefinido
			}
			if r, ok = sumaSegura(r, n%10); !ok {
				return nil, ErrIndefinido
			}
			n /= 10
		}
		if neg {
			r = -r
		}
		return r, nil
	}, parc, ayu("invertirNumero"))
	r.def("numDigitos", ts(tI), tI, 9, c("digitos", "longitud"), "cuántas cifras tiene {0}", "numDigitos({0})", func(a []V, _ []Funcion) (V, error) {
		n, k := ent(a[0]), 0
		if n == 0 {
			return 1, nil
		}
		for n != 0 {
			k++
			n /= 10
		}
		return k, nil
	}, ayu("numDigitos"))
	r.def("raizEntera", ts(tI), tI, 9, c("raiz"), "la raíz cuadrada entera de {0}", "raizEntera({0})", func(a []V, _ []Funcion) (V, error) {
		n := ent(a[0])
		if n < 0 {
			return nil, ErrIndefinido
		}
		return raizEntera(n), nil
	}, parc, ayu("raizEntera"))
	r.def("binario", ts(tI), tS, 10, c("binario"), "{0} en binario", "strconv.FormatInt(int64({0}), 2)", func(a []V, _ []Funcion) (V, error) {
		return strconv.FormatInt(int64(ent(a[0])), 2), nil
	}, imp("strconv"))
	r.def("aDecimal", ts(tI), tF, 10, c("decimal"), "{0} como decimal", "float64({0})", func(a []V, _ []Funcion) (V, error) {
		return float64(ent(a[0])), nil
	})
}

// sustantivos for list element types
func registrarListas(r *Registro) {
	elems := []nucleo.Tipo{tI, tS, tR, tF, tB, tLI}
	for _, e := range elems {
		le := nucleo.ListaDe(e)
		r.def("largo", ts(le), tI, 6, c("longitud", "contar"), "cuántos elementos tiene {0}", "len({0})", func(a []V, _ []Funcion) (V, error) {
			return len(lis(a[0])), nil
		})
	}
	// suma / producto
	r.def("suma", ts(tLI), tI, 6, c("sumar"), "la suma de {0}", "", func(a []V, _ []Funcion) (V, error) {
		s := 0
		for _, x := range lis(a[0]) {
			var ok bool
			if s, ok = sumaSegura(s, ent(x)); !ok {
				return nil, ErrIndefinido
			}
		}
		return s, nil
	})
	r.def("suma", ts(tLF), tF, 7, c("sumar"), "la suma de {0}", "", func(a []V, _ []Funcion) (V, error) {
		s := 0.0
		for _, x := range lis(a[0]) {
			s += flo(x)
		}
		return s, nil
	})
	r.def("producto", ts(tLI), tI, 8, c("producto", "multiplicar"), "el producto de {0}", "", func(a []V, _ []Funcion) (V, error) {
		p := 1
		for _, x := range lis(a[0]) {
			var ok bool
			if p, ok = multSegura(p, ent(x)); !ok {
				return nil, ErrIndefinido
			}
		}
		return p, nil
	}, parc)
	for _, e := range []nucleo.Tipo{tI, tF} {
		le := nucleo.ListaDe(e)
		r.def("maxL", ts(le), e, 7, c("maximo"), "el mayor de {0}", "", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			if len(xs) == 0 {
				return nil, ErrIndefinido
			}
			m := xs[0]
			for _, x := range xs[1:] {
				if menor(m, x) {
					m = x
				}
			}
			return m, nil
		}, parc)
		r.def("minL", ts(le), e, 7, c("minimo"), "el menor de {0}", "", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			if len(xs) == 0 {
				return nil, ErrIndefinido
			}
			m := xs[0]
			for _, x := range xs[1:] {
				if menor(x, m) {
					m = x
				}
			}
			return m, nil
		}, parc)
		r.def("promedio", ts(le), tF, 8, c("promedio"), "la media de {0}", "", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			if len(xs) == 0 {
				return nil, ErrIndefinido
			}
			if e.Clase == nucleo.CInt {
				s := 0
				for _, x := range xs {
					var ok bool
					if s, ok = sumaSegura(s, ent(x)); !ok {
						return nil, ErrIndefinido
					}
				}
				return float64(s) / float64(len(xs)), nil
			}
			s := 0.0
			for _, x := range xs {
				s += flo(x)
			}
			return s / float64(len(xs)), nil
		}, parc)
	}
	for _, e := range []nucleo.Tipo{tI, tS, tR, tF} {
		le := nucleo.ListaDe(e)
		r.def("ordenar", ts(le), le, 7, c("ordenar"), "{0} ordenados de menor a mayor", "", func(a []V, _ []Funcion) (V, error) {
			return ordenarValores(lis(a[0])), nil
		}, idem, imp("slices"))
		r.def("ordenarDesc", ts(le), le, 8, c("ordenar_desc"), "{0} ordenados de mayor a menor", "", func(a []V, _ []Funcion) (V, error) {
			return invertirValores(ordenarValores(lis(a[0]))), nil
		}, idem, imp("slices"))
	}
	for _, e := range []nucleo.Tipo{tI, tS, tR, tF, tB} {
		le := nucleo.ListaDe(e)
		r.def("invertir", ts(le), le, 7, c("invertir"), "{0} al revés", "", func(a []V, _ []Funcion) (V, error) {
			return invertirValores(lis(a[0])), nil
		}, invol, imp("slices"))
	}
	for _, e := range []nucleo.Tipo{tI, tS, tR} {
		e := e
		le := nucleo.ListaDe(e)
		r.def("unicos", ts(le), le, 8, c("sin_repetir"), "{0} sin repetidos", "unicos({0})", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			out := make([]V, 0, len(xs))
			visto := map[V]bool{}
			for _, x := range xs {
				if !visto[x] {
					visto[x] = true
					out = append(out, x)
				}
			}
			return out, nil
		}, idem, ayu("unicos"))
		r.def("contiene", ts(le, e), tB, 9, c("contiene", "buscar"), "{0} contiene {1}", "slices.Contains({0}, {1})", func(a []V, _ []Funcion) (V, error) {
			for _, x := range lis(a[0]) {
				if x == a[1] {
					return true, nil
				}
			}
			return false, nil
		}, imp("slices"))
		r.def("indice", ts(le, e), tI, 10, c("posicion", "buscar"), "la posición de {1} en {0}", "slices.Index({0}, {1})", func(a []V, _ []Funcion) (V, error) {
			for i, x := range lis(a[0]) {
				if x == a[1] {
					return i, nil
				}
			}
			return -1, nil
		}, imp("slices"))
		r.def("tomar", ts(le, tI), le, 10, c("tomar"), "los primeros {1} elementos de {0}", "tomar({0}, {1})", func(a []V, _ []Funcion) (V, error) {
			xs, n := lis(a[0]), ent(a[1])
			n = max(0, min(n, len(xs)))
			return xs[:n:n], nil
		}, ayu("tomar"))
		r.def("quitar", ts(le, tI), le, 10, c("quitar_primeros"), "{0} sin sus primeros {1} elementos", "quitar({0}, {1})", func(a []V, _ []Funcion) (V, error) {
			xs, n := lis(a[0]), ent(a[1])
			n = max(0, min(n, len(xs)))
			return xs[n:], nil
		}, ayu("quitar"))
		r.def("concat", ts(le, le), le, 10, c("concatenar", "unir"), "{0} seguido de {1}", "slices.Concat({0}, {1})", func(a []V, _ []Funcion) (V, error) {
			xs, ys := lis(a[0]), lis(a[1])
			if len(xs)+len(ys) > maxLargo {
				return nil, ErrIndefinido
			}
			out := make([]V, 0, len(xs)+len(ys))
			return append(append(out, xs...), ys...), nil
		}, imp("slices"))
		r.def("agrega", ts(le, e), le, 10, c("agregar"), "{0} con {1} al final", "append(slices.Clone({0}), {1})", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			if len(xs)+1 > maxLargo {
				return nil, ErrIndefinido
			}
			out := make([]V, 0, len(xs)+1)
			return append(append(out, xs...), a[1]), nil
		}, imp("slices"))
	}
	for _, e := range []nucleo.Tipo{tI, tS, tR, tF, tB} {
		le := nucleo.ListaDe(e)
		r.def("primero", ts(le), e, 8, c("primero"), "el primer elemento de {0}", "", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			if len(xs) == 0 {
				return nil, ErrIndefinido
			}
			return xs[0], nil
		}, parc)
		r.def("ultimo", ts(le), e, 8, c("ultimo"), "el último elemento de {0}", "", func(a []V, _ []Funcion) (V, error) {
			xs := lis(a[0])
			if len(xs) == 0 {
				return nil, ErrIndefinido
			}
			return xs[len(xs)-1], nil
		}, parc)
		r.def("elemento", ts(le, tI), e, 9, c("posicion"), "el elemento {1} de {0} (contando desde 0)", "", func(a []V, _ []Funcion) (V, error) {
			xs, i := lis(a[0]), ent(a[1])
			if i < 0 || i >= len(xs) {
				return nil, ErrIndefinido
			}
			return xs[i], nil
		}, parc)
	}

	// higher-order
	for _, e := range []nucleo.Tipo{tI, tR, tS, tB} {
		le := nucleo.ListaDe(e)
		r.defL("contar", ts(tipoFuncion, le), []LambdaTipo{lt(tB, e), {}}, tI, 6, c("contar"), "cuántos elementos de {1} cumplen {L0}", "",
			func(a []V, l []Funcion) (V, error) {
				n := 0
				arg := make([]V, 1)
				for _, x := range lis(a[0]) {
					arg[0] = x
					v, err := l[0](arg)
					if err != nil {
						return nil, err
					}
					if boo(v) {
						n++
					}
				}
				return n, nil
			})
		r.defL("todos", ts(tipoFuncion, le), []LambdaTipo{lt(tB, e), {}}, tB, 8, c("todos", "comprobar"), "todos los elementos de {1} cumplen {L0}", "",
			func(a []V, l []Funcion) (V, error) {
				arg := make([]V, 1)
				for _, x := range lis(a[0]) {
					arg[0] = x
					v, err := l[0](arg)
					if err != nil {
						return nil, err
					}
					if !boo(v) {
						return false, nil
					}
				}
				return true, nil
			})
		r.defL("alguno", ts(tipoFuncion, le), []LambdaTipo{lt(tB, e), {}}, tB, 8, c("alguno", "comprobar", "contiene"), "algún elemento de {1} cumple {L0}", "",
			func(a []V, l []Funcion) (V, error) {
				arg := make([]V, 1)
				for _, x := range lis(a[0]) {
					arg[0] = x
					v, err := l[0](arg)
					if err != nil {
						return nil, err
					}
					if boo(v) {
						return true, nil
					}
				}
				return false, nil
			})
	}
	for _, e := range []nucleo.Tipo{tI, tR, tS} {
		le := nucleo.ListaDe(e)
		r.defL("filtra", ts(tipoFuncion, le), []LambdaTipo{lt(tB, e), {}}, le, 6, c("filtrar"), "los elementos de {1} que cumplen {L0}", "",
			func(a []V, l []Funcion) (V, error) {
				out := []V{}
				arg := make([]V, 1)
				for _, x := range lis(a[0]) {
					arg[0] = x
					v, err := l[0](arg)
					if err != nil {
						return nil, err
					}
					if boo(v) {
						out = append(out, x)
					}
				}
				return out, nil
			})
	}
	for _, par := range [][2]nucleo.Tipo{{tI, tI}, {tI, tS}, {tS, tS}, {tS, tI}, {tR, tR}} {
		de, a := par[0], par[1]
		r.defL("mapea", ts(tipoFuncion, nucleo.ListaDe(de)), []LambdaTipo{lt(a, de), {}}, nucleo.ListaDe(a), 6, c("transformar"),
			"{L0} para cada elemento de {1}", "", func(args []V, l []Funcion) (V, error) {
				xs := lis(args[0])
				out := make([]V, len(xs))
				arg := make([]V, 1)
				for i, x := range xs {
					arg[0] = x
					v, err := l[0](arg)
					if err != nil {
						return nil, err
					}
					out[i] = v
				}
				return out, nil
			})
	}
	r.defL("pliega", ts(tipoFuncion, tI, tLI), []LambdaTipo{lt(tI, tI, tI), {}, {}}, tI, 12, c("acumular"),
		"el resultado de combinar los elementos de {2} con {L0}, empezando en {1}", "", func(a []V, l []Funcion) (V, error) {
			acc := a[0]
			arg := make([]V, 2)
			for _, x := range lis(a[1]) {
				arg[0], arg[1] = acc, x
				v, err := l[0](arg)
				if err != nil {
					return nil, err
				}
				acc = v
			}
			return acc, nil
		})
	r.def("rango", ts(tI, tI), tLI, 9, c("rango"), "los números desde {0} hasta {1} (sin incluirlo)", "rango({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		x, y := ent(a[0]), ent(a[1])
		if y <= x {
			return []V{}, nil
		}
		if d, ok := restaSegura(y, x); !ok || d > maxLargo {
			return nil, ErrIndefinido
		}
		out := make([]V, 0, y-x)
		for i := x; i < y; i++ {
			out = append(out, i)
		}
		return out, nil
	}, parc, ayu("rango"))
	r.def("sumaAcumulada", ts(tLI), tLI, 9, c("acumular", "sumar"), "las sumas acumuladas de {0}", "sumaAcumulada({0})", func(a []V, _ []Funcion) (V, error) {
		xs := lis(a[0])
		out := make([]V, len(xs))
		s := 0
		for i, x := range xs {
			var ok bool
			if s, ok = sumaSegura(s, ent(x)); !ok {
				return nil, ErrIndefinido
			}
			out[i] = s
		}
		return out, nil
	}, ayu("sumaAcumulada"))
}

func registrarTextos(r *Registro) {
	r.def("largoS", ts(tS), tI, 7, c("longitud"), "cuántas letras tiene {0}", "utf8.RuneCountInString({0})", func(a []V, _ []Funcion) (V, error) {
		return utf8.RuneCountInString(tex(a[0])), nil
	}, imp("unicode/utf8"))
	r.def("mayus", ts(tS), tS, 7, c("mayusculas"), "{0} en mayúsculas", "strings.ToUpper({0})", funTexto(strings.ToUpper), imp("strings"), idem)
	r.def("minus", ts(tS), tS, 7, c("minusculas"), "{0} en minúsculas", "strings.ToLower({0})", funTexto(strings.ToLower), imp("strings"), idem)
	r.def("titulo", ts(tS), tS, 8, c("titulo"), "{0} con cada palabra en mayúscula", "titulo({0})", funTexto(titulo), ayu("titulo"), idem)
	r.def("capitalizar", ts(tS), tS, 9, c("capitalizar"), "{0} con la primera letra en mayúscula", "capitalizar({0})", funTexto(capitalizar), ayu("capitalizar"), idem)
	r.def("invertirS", ts(tS), tS, 7, c("invertir"), "{0} al revés", "invertirTexto({0})", funTexto(invertirTexto), ayu("invertirTexto"), invol)
	r.def("palabras", ts(tS), tLS, 6, c("separar", "palabras"), "las palabras de {0}", "strings.Fields({0})", func(a []V, _ []Funcion) (V, error) {
		ps := strings.Fields(tex(a[0]))
		out := make([]V, len(ps))
		for i, p := range ps {
			out[i] = p
		}
		return out, nil
	}, imp("strings"))
	r.def("unir", ts(tLS, tS), tS, 8, c("unir", "concatenar"), "{0} unidas con {1}", "strings.Join({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		xs := lis(a[0])
		ps := make([]string, len(xs))
		total := 0
		for i, x := range xs {
			ps[i] = tex(x)
			total += len(ps[i])
		}
		if total+len(xs)*len(tex(a[1])) > maxTexto {
			return nil, ErrIndefinido
		}
		return strings.Join(ps, tex(a[1])), nil
	}, imp("strings"))
	r.def("dividir", ts(tS, tS), tLS, 9, c("separar"), "{0} partido por {1}", "strings.Split({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		ps := strings.Split(tex(a[0]), tex(a[1]))
		if len(ps) > maxLargo {
			return nil, ErrIndefinido
		}
		out := make([]V, len(ps))
		for i, p := range ps {
			out[i] = p
		}
		return out, nil
	}, imp("strings"))
	r.def("recortar", ts(tS), tS, 9, c("quitar_espacios"), "{0} sin espacios al principio ni al final", "strings.TrimSpace({0})", funTexto(strings.TrimSpace), imp("strings"), idem)
	r.def("reemplazar", ts(tS, tS, tS), tS, 11, c("reemplazar"), "{0} cambiando {1} por {2}", "strings.ReplaceAll({0}, {1}, {2})", func(a []V, _ []Funcion) (V, error) {
		s, x, y := tex(a[0]), tex(a[1]), tex(a[2])
		if x == "" && len(s) > 1000 {
			return nil, ErrIndefinido
		}
		out := strings.ReplaceAll(s, x, y)
		if len(out) > maxTexto {
			return nil, ErrIndefinido
		}
		return out, nil
	}, imp("strings"))
	r.def("repetir", ts(tS, tI), tS, 10, c("repetir"), "{0} repetido {1} veces", "strings.Repeat({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		s, n := tex(a[0]), ent(a[1])
		if n < 0 || n > 1000 || len(s)*n > maxTexto {
			return nil, ErrIndefinido
		}
		return strings.Repeat(s, n), nil
	}, parc, imp("strings"))
	r.def("contieneS", ts(tS, tS), tB, 9, c("contiene", "buscar"), "{0} contiene {1}", "strings.Contains({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		return strings.Contains(tex(a[0]), tex(a[1])), nil
	}, imp("strings"))
	r.def("empiezaCon", ts(tS, tS), tB, 9, c("empieza_con"), "{0} empieza por {1}", "strings.HasPrefix({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		return strings.HasPrefix(tex(a[0]), tex(a[1])), nil
	}, imp("strings"))
	r.def("terminaCon", ts(tS, tS), tB, 9, c("termina_con"), "{0} termina en {1}", "strings.HasSuffix({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		return strings.HasSuffix(tex(a[0]), tex(a[1])), nil
	}, imp("strings"))
	r.def("runas", ts(tS), tLR, 6, c("letras"), "las letras de {0}", "[]rune({0})", func(a []V, _ []Funcion) (V, error) {
		s := tex(a[0])
		out := make([]V, 0, len(s))
		for _, r := range s {
			out = append(out, int(r))
		}
		return out, nil
	})
	r.def("deRunas", ts(tLR), tS, 8, c("unir"), "el texto formado por {0}", "string({0})", func(a []V, _ []Funcion) (V, error) {
		xs := lis(a[0])
		rs := make([]rune, len(xs))
		for i, x := range xs {
			rs[i] = rune(ent(x))
		}
		return string(rs), nil
	})
	r.def("concatS", ts(tS, tS), tS, 8, c("concatenar", "unir"), "{0} seguido de {1}", "{0:4} + {1:5}", func(a []V, _ []Funcion) (V, error) {
		x, y := tex(a[0]), tex(a[1])
		if len(x)+len(y) > maxTexto {
			return nil, ErrIndefinido
		}
		return x + y, nil
	}, prec(4))
	r.def("itoa", ts(tI), tS, 8, c("a_texto"), "{0} como texto", "strconv.Itoa({0})", func(a []V, _ []Funcion) (V, error) {
		return strconv.Itoa(ent(a[0])), nil
	}, imp("strconv"))
	r.def("atoi", ts(tS), tI, 9, c("a_numero"), "el número escrito en {0}", "", func(a []V, _ []Funcion) (V, error) {
		n, err := strconv.Atoi(tex(a[0]))
		if err != nil {
			return nil, ErrIndefinido
		}
		return n, nil
	}, parc, imp("strconv"))
	r.def("contarSub", ts(tS, tS), tI, 10, c("contar", "buscar"), "cuántas veces aparece {1} en {0}", "strings.Count({0}, {1})", func(a []V, _ []Funcion) (V, error) {
		return strings.Count(tex(a[0]), tex(a[1])), nil
	}, imp("strings"))
	r.def("quitarEspacios", ts(tS), tS, 8, c("quitar_espacios"), "{0} sin espacios", `strings.Join(strings.Fields({0}), "")`, funTexto(func(s string) string {
		return strings.Join(strings.Fields(s), "")
	}), imp("strings"), idem)
	r.def("esPalindromoS", ts(tS), tB, 8, c("palindromo"), "{0} se lee igual al revés", "esPalindromo({0})", func(a []V, _ []Funcion) (V, error) {
		return esPalindromo(tex(a[0])), nil
	}, ayu("esPalindromo"), adj("palíndromas"))
	r.def("esPalindromoLetras", ts(tS), tB, 9, c("palindromo"), "{0} se lee igual al revés sin contar espacios ni tildes", "esPalindromoLetras({0})", func(a []V, _ []Funcion) (V, error) {
		return esPalindromoLetras(tex(a[0])), nil
	}, ayu("esPalindromoLetras"), imp("unicode"), adj("palíndromas"))

	// runes
	r.def("esVocal", ts(tR), tB, 6, c("vocal"), "{0} es una vocal", "esVocal({0})", predRuna(esVocal), ayu("esVocal"), imp("strings"), sust("vocales", true))
	r.def("esConsonante", ts(tR), tB, 7, c("consonante"), "{0} es una consonante", "esConsonante({0})", predRuna(func(r rune) bool {
		return unicode.IsLetter(r) && !esVocal(r)
	}), ayu("esConsonante"), imp("strings", "unicode"), sust("consonantes", true))
	r.def("esDigito", ts(tR), tB, 7, c("digito"), "{0} es un dígito", "unicode.IsDigit({0})", predRuna(unicode.IsDigit), imp("unicode"), sust("dígitos", false))
	r.def("esLetra", ts(tR), tB, 7, c("letra"), "{0} es una letra", "unicode.IsLetter({0})", predRuna(unicode.IsLetter), imp("unicode"), sust("letras", true))
	r.def("esMayus", ts(tR), tB, 8, c("mayuscula"), "{0} es mayúscula", "unicode.IsUpper({0})", predRuna(unicode.IsUpper), imp("unicode"), sust("mayúsculas", true))
	r.def("esMinus", ts(tR), tB, 8, c("minuscula"), "{0} es minúscula", "unicode.IsLower({0})", predRuna(unicode.IsLower), imp("unicode"), sust("minúsculas", true))
	r.def("esEspacio", ts(tR), tB, 8, c("espacio"), "{0} es un espacio", "unicode.IsSpace({0})", predRuna(unicode.IsSpace), imp("unicode"), sust("espacios", false))
	r.def("aMayus", ts(tR), tR, 8, c("mayusculas"), "{0} en mayúscula", "unicode.ToUpper({0})", func(a []V, _ []Funcion) (V, error) {
		return int(unicode.ToUpper(rune(ent(a[0])))), nil
	}, imp("unicode"), idem)
	r.def("aMinus", ts(tR), tR, 8, c("minusculas"), "{0} en minúscula", "unicode.ToLower({0})", func(a []V, _ []Funcion) (V, error) {
		return int(unicode.ToLower(rune(ent(a[0])))), nil
	}, imp("unicode"), idem)
	r.def("runaTexto", ts(tR), tS, 9, c("a_texto"), "{0} como texto", "string({0})", func(a []V, _ []Funcion) (V, error) {
		return string(rune(ent(a[0]))), nil
	})
	r.def("==", ts(tR, tR), tB, 9, c("igual_a", "comparar"), "{0} es igual a {1}", "{0:4} == {1:4}", func(a []V, _ []Funcion) (V, error) {
		return ent(a[0]) == ent(a[1]), nil
	}, conm, prec(3))

	// []string
	r.def("masLarga", ts(tLS), tS, 7, c("mas_largo"), "la palabra más larga de {0}", "", func(a []V, _ []Funcion) (V, error) {
		xs := lis(a[0])
		if len(xs) == 0 {
			return nil, ErrIndefinido
		}
		m, lm := tex(xs[0]), utf8.RuneCountInString(tex(xs[0]))
		for _, x := range xs[1:] {
			if l := utf8.RuneCountInString(tex(x)); l > lm {
				m, lm = tex(x), l
			}
		}
		return m, nil
	}, parc, imp("unicode/utf8"))
	r.def("masCorta", ts(tLS), tS, 7, c("mas_corto"), "la palabra más corta de {0}", "", func(a []V, _ []Funcion) (V, error) {
		xs := lis(a[0])
		if len(xs) == 0 {
			return nil, ErrIndefinido
		}
		m, lm := tex(xs[0]), utf8.RuneCountInString(tex(xs[0]))
		for _, x := range xs[1:] {
			if l := utf8.RuneCountInString(tex(x)); l < lm {
				m, lm = tex(x), l
			}
		}
		return m, nil
	}, parc, imp("unicode/utf8"))
	r.def("largos", ts(tLS), tLI, 8, c("longitud"), "cuántas letras tiene cada palabra de {0}", "largos({0})", func(a []V, _ []Funcion) (V, error) {
		xs := lis(a[0])
		out := make([]V, len(xs))
		for i, x := range xs {
			out[i] = utf8.RuneCountInString(tex(x))
		}
		return out, nil
	}, ayu("largos"), imp("unicode/utf8"))
}

func registrarOtros(r *Registro) {
	r.def("frecuencias", ts(tLS), tMSI, 7, c("frecuencia"), "cuántas veces aparece cada palabra de {0}", "", func(a []V, _ []Funcion) (V, error) {
		return frecuencias(lis(a[0])), nil
	})
	r.def("frecuenciaLetras", ts(tS), tMSI, 8, c("frecuencia", "letras"), "cuántas veces aparece cada letra de {0}", "frecuenciaLetras({0})", func(a []V, _ []Funcion) (V, error) {
		var xs []V
		for _, r := range tex(a[0]) {
			if unicode.IsLetter(r) {
				xs = append(xs, string(unicode.ToLower(r)))
			}
		}
		return frecuencias(xs), nil
	}, ayu("frecuenciaLetras"), imp("unicode"))

	// matrices (N2)
	r.def("transpuesta", ts(tLLI), tLLI, 9, c("transponer", "matriz"), "la traspuesta de {0}", "transpuesta({0})", func(a []V, _ []Funcion) (V, error) {
		m := lis(a[0])
		if len(m) == 0 {
			return []V{}, nil
		}
		ancho := len(lis(m[0]))
		for _, fila := range m {
			if len(lis(fila)) != ancho {
				return nil, ErrIndefinido
			}
		}
		out := make([]V, ancho)
		for j := 0; j < ancho; j++ {
			col := make([]V, len(m))
			for i, fila := range m {
				col[i] = lis(fila)[j]
			}
			out[j] = col
		}
		return out, nil
	}, parc, ayu("transpuesta"), invol)
	r.def("sumaFilas", ts(tLLI), tLI, 9, c("sumar", "matriz"), "la suma de cada fila de {0}", "sumaFilas({0})", func(a []V, _ []Funcion) (V, error) {
		m := lis(a[0])
		out := make([]V, len(m))
		for i, fila := range m {
			s := 0
			for _, x := range lis(fila) {
				var ok bool
				if s, ok = sumaSegura(s, ent(x)); !ok {
					return nil, ErrIndefinido
				}
			}
			out[i] = s
		}
		return out, nil
	}, ayu("sumaFilas"))
	r.def("aplanar", ts(tLLI), tLI, 9, c("aplanar", "matriz"), "todos los números de {0} en una sola lista", "aplanar({0})", func(a []V, _ []Funcion) (V, error) {
		out := []V{}
		for _, fila := range lis(a[0]) {
			out = append(out, lis(fila)...)
			if len(out) > maxLargo {
				return nil, ErrIndefinido
			}
		}
		return out, nil
	}, ayu("aplanar"))
	r.def("diagonal", ts(tLLI), tLI, 9, c("matriz"), "la diagonal de {0}", "diagonal({0})", func(a []V, _ []Funcion) (V, error) {
		m := lis(a[0])
		out := make([]V, len(m))
		for i, fila := range m {
			f := lis(fila)
			if i >= len(f) {
				return nil, ErrIndefinido
			}
			out[i] = f[i]
		}
		return out, nil
	}, parc, ayu("diagonal"))

	// aliases named in the spec
	r.alias("ordenarS", r.buscarTipos("ordenar", tLS))
	r.alias("unicosS", r.buscarTipos("unicos", tLS))
	r.alias("filtraS", r.buscarTipos("filtra", tLS))
	r.alias("contarS", r.buscarTipos("contar", tLS))
	r.alias("mapeaS", r.buscarTipos("mapea", tLS))
}

func frecuencias(xs []V) nucleo.Mapa {
	cuenta := map[string]int{}
	var orden []string
	for _, x := range xs {
		s := tex(x)
		if _, ok := cuenta[s]; !ok {
			orden = append(orden, s)
		}
		cuenta[s]++
	}
	m := make(nucleo.Mapa, 0, len(orden))
	for _, s := range orden {
		m = append(m, nucleo.Par{K: s, V: cuenta[s]})
	}
	return m.Ordenada()
}
