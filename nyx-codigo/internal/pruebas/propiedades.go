package pruebas

import (
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Property expressions follow nucleo.Propiedad: e0..eN are copies of the inputs (receiver first),
// r0..rM the results, F(...) calls the same variant, and the harness provides nyx__Ordenada,
// nyx__EsPermutacion, nyx__Subsecuencia, nyx__Contiene and nyx__Igual. Anything the five helpers do
// not cover (descending order, reversal, "no duplicates", "every element ≤ r0") is written as an inline
// Go function literal, so the property type-checks without any other helper.

// modificadores are the predicate concepts that select elements of a list.
var modificadores = map[string]bool{
	"par": true, "impar": true, "positivo": true, "negativo": true, "cero": true, "mayor_que": true,
	"menor_que": true, "igual_a": true, "distinto_de": true, "divisible_por": true, "primo": true,
	"vocal": true, "consonante": true, "digito": true, "letra": true, "mayuscula": true, "minuscula": true,
	"espacio": true, "empieza_con": true, "termina_con": true, "longitud_mayor": true, "longitud_menor": true,
}

// acciones are the action concepts (the first block of nucleo.Conceptos).
var acciones = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range nucleo.Conceptos {
		if c == "par" {
			break
		}
		m[c] = true
	}
	return m
}()

// conjunto keeps the concepts as a set and answers questions about them.
type conjunto map[string]bool

func (c conjunto) hayModificador() bool {
	for k := range c {
		if modificadores[k] {
			return true
		}
	}
	return false
}

// soloAcciones reports whether every action concept present is in permitidas.
func (c conjunto) soloAcciones(permitidas ...string) bool {
	p := map[string]bool{}
	for _, x := range permitidas {
		p[x] = true
	}
	for k := range c {
		if acciones[k] && !p[k] {
			return false
		}
	}
	return true
}

// Propiedades returns the concept properties that fit the signature's types (§4.3 table). Properties
// that call F are only given for plain functions (no receiver). When the last result is an error, every
// property holds trivially for cases that return a non-nil error.
func Propiedades(conceptos []string, f nucleo.Firma) []nucleo.Propiedad {
	c := conjunto{}
	for _, k := range conceptos {
		c[k] = true
	}
	ent := f.Entradas()
	res := f.Res
	if len(res) == 0 || res[0].Clase == nucleo.CError {
		return nil
	}
	errIdx := -1
	if len(res) >= 2 && res[len(res)-1].Clase == nucleo.CError {
		errIdx = len(res) - 1
	}
	r0 := res[0]
	desp := len(ent) - len(f.Params)
	conF := f.Receptor == nil
	unaEntrada := len(ent) == 1

	var out []nucleo.Propiedad
	agregar := func(nombre, explicacion, expr, requiere string) {
		if errIdx >= 0 {
			expr = "r" + strconv.Itoa(errIdx) + " != nil || (" + expr + ")"
		}
		for _, p := range out {
			if p.Nombre == nombre {
				return
			}
		}
		out = append(out, nucleo.Propiedad{Nombre: nombre, Explicacion: explicacion, Expr: expr, Requiere: requiere})
	}

	// index of the first input with exactly the result's type
	mismo := -1
	for i, t := range ent {
		if t.Igual(r0) {
			mismo = i
			break
		}
	}
	// index of the first list input
	lista := -1
	for i, t := range ent {
		if t.Clase == nucleo.CLista && t.Elem != nil {
			lista = i
			break
		}
	}
	e := func(i int) string { return "e" + strconv.Itoa(i) }

	// ordenar / ordenar_desc
	if mismo >= 0 && r0.Clase == nucleo.CLista && r0.Elem != nil && ordenable(*r0.Elem) {
		en := e(mismo)
		perm := !c.hayModificador() && c.soloAcciones("ordenar", "ordenar_desc")
		if c["ordenar"] {
			expr := "nyx__Ordenada(r0)"
			if perm {
				expr += " && nyx__EsPermutacion(" + en + ", r0)"
			}
			agregar("ordenada", "el resultado está ordenado de menor a mayor y tiene los mismos elementos", expr, "")
		}
		if c["ordenar_desc"] {
			expr := "func() bool { for i := 1; i < len(r0); i++ { if r0[i-1] < r0[i] { return false } }; return true }()"
			if perm {
				expr += " && nyx__EsPermutacion(" + en + ", r0)"
			}
			agregar("ordenada_desc", "el resultado está ordenado de mayor a menor y tiene los mismos elementos", expr, "")
		}
	}

	// invertir: F(r0) gives back the input
	if c["invertir"] && conF && unaEntrada && mismo == 0 && (r0.Clase == nucleo.CLista || r0.Clase == nucleo.CString) &&
		!c.hayModificador() && c.soloAcciones("invertir") {
		agregar("inversa", "si lo inviertes otra vez vuelves a la entrada", "nyx__Igual("+llamada(f, desp, 0, "r0")+", e0) && len(r0) == len(e0)", "")
	}

	// filtrar, or a predicate modifier on a list: the result is a subsequence of the input
	if mismo >= 0 && r0.Clase == nucleo.CLista && (c["filtrar"] || c.hayModificador()) &&
		c.soloAcciones("filtrar", "sin_repetir", "tomar", "quitar_primeros", "eliminar") {
		agregar("subsecuencia", "el resultado solo tiene elementos de la entrada, en el mismo orden", "nyx__Subsecuencia(r0, "+e(mismo)+")", "")
	}

	// sumar: F(a ++ b) == F(a) + F(b) on a split of the input
	if c["sumar"] && conF && unaEntrada && lista == 0 && numerico(*ent[0].Elem) && numerico(r0) &&
		c.soloAcciones("sumar", "filtrar", "doble", "triple", "cuadrado", "absoluto", "transformar") {
		tl := ent[0].Go()
		a := "append(" + tl + "(nil), e0[:len(e0)/2]...)"
		b := "append(" + tl + "(nil), e0[len(e0)/2:]...)"
		agregar("suma_partida", "sumar las dos mitades por separado da lo mismo que sumarlo todo",
			"nyx__Igual(r0, "+llamada(f, desp, 0, a)+"+"+llamada(f, desp, 0, b)+")", "")
	}

	// maximo / minimo
	if unaEntrada && lista == 0 && ent[0].Elem.Igual(r0) && ordenable(r0) && !c.hayModificador() {
		if c["maximo"] && c.soloAcciones("maximo") {
			agregar("maximo", "el resultado está en la lista y ningún elemento es mayor",
				"nyx__Contiene(e0, r0) && func() bool { for _, x := range e0 { if x > r0 { return false } }; return true }()", "len(e0) > 0")
		}
		if c["minimo"] && c.soloAcciones("minimo") {
			agregar("minimo", "el resultado está en la lista y ningún elemento es menor",
				"nyx__Contiene(e0, r0) && func() bool { for _, x := range e0 { if x < r0 { return false } }; return true }()", "len(e0) > 0")
		}
	}

	// contar: 0 <= r0 <= len(input)
	if c["contar"] && r0.Clase == nucleo.CInt {
		k := -1
		switch {
		case unaEntrada && (ent[0].Clase == nucleo.CLista || ent[0].Clase == nucleo.CString || ent[0].Clase == nucleo.CMapa):
			k = 0
		case len(ent) == 2 && ent[0].Clase == nucleo.CLista && ent[0].Elem != nil && ent[0].Elem.Igual(ent[1]):
			k = 0
		case len(ent) == 2 && ent[1].Clase == nucleo.CLista && ent[1].Elem != nil && ent[1].Elem.Igual(ent[0]):
			k = 1
		}
		if k >= 0 {
			agregar("contar_rango", "la cuenta está entre 0 y el tamaño de la entrada", "0 <= r0 && r0 <= len("+e(k)+")", "")
		}
	}

	// sin_repetir: idempotent and without duplicates
	if c["sin_repetir"] && unaEntrada && mismo == 0 && c.soloAcciones("sin_repetir") && !c.hayModificador() {
		switch {
		case r0.Clase == nucleo.CLista && r0.Elem != nil && comparable(*r0.Elem):
			if conF {
				agregar("idempotente", "aplicarlo dos veces da lo mismo que una", "nyx__Igual("+llamada(f, desp, 0, "r0")+", r0)", "")
			}
			agregar("sin_duplicados", "el resultado no tiene elementos repetidos",
				"func() bool { vistos := map["+r0.Elem.Go()+"]bool{}; for _, x := range r0 { if vistos[x] { return false }; vistos[x] = true }; return true }()", "")
		case r0.Clase == nucleo.CString:
			if conF {
				agregar("idempotente", "aplicarlo dos veces da lo mismo que una", llamada(f, desp, 0, "r0")+" == r0", "")
			}
			agregar("sin_duplicados", "el resultado no tiene letras repetidas",
				"func() bool { vistas := map[rune]bool{}; for _, x := range r0 { if vistas[x] { return false }; vistas[x] = true }; return true }()", "")
		}
	}

	// palindromo: the answer does not change when the input is reversed
	if c["palindromo"] && conF && unaEntrada && r0.Clase == nucleo.CBool {
		var inv string
		switch {
		case ent[0].Clase == nucleo.CString && !ent[0].Definido:
			inv = "func() string { r := []rune(e0); for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 { r[i], r[j] = r[j], r[i] }; return string(r) }()"
		case ent[0].Clase == nucleo.CLista:
			tl := ent[0].Go()
			inv = "func() " + tl + " { r := append(" + tl + "(nil), e0...); for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 { r[i], r[j] = r[j], r[i] }; return r }()"
		}
		if inv != "" {
			agregar("simetrica", "da lo mismo con la entrada al revés", llamada(f, desp, 0, inv)+" == r0", "")
		}
	}

	// idempotent text transformations
	for _, k := range []string{"mayusculas", "minusculas", "titulo", "quitar_espacios"} {
		if !c[k] || !conF || !unaEntrada || mismo != 0 {
			continue
		}
		switch {
		case r0.Clase == nucleo.CString:
			agregar("idempotente", "aplicarlo dos veces da lo mismo que una", llamada(f, desp, 0, "r0")+" == r0", "")
		case r0.Clase == nucleo.CLista && r0.Elem != nil && r0.Elem.Clase == nucleo.CString:
			agregar("idempotente", "aplicarlo dos veces da lo mismo que una", "nyx__Igual("+llamada(f, desp, 0, "r0")+", r0)", "")
		}
	}

	if c["longitud"] && r0.Clase == nucleo.CInt {
		agregar("longitud_no_negativa", "una longitud nunca es negativa", "r0 >= 0", "")
	}
	if c["absoluto"] && (r0.Clase == nucleo.CInt || r0.Clase == nucleo.CFloat) && c.soloAcciones("absoluto") {
		agregar("absoluto_no_negativo", "un valor absoluto nunca es negativo", "r0 >= 0", "")
	}
	return out
}

// llamada writes F(...) with argument i replaced by arg and the other inputs passed through
// (variadic last parameters get "...").
func llamada(f nucleo.Firma, desp, i int, arg string) string {
	ent := f.Entradas()
	partes := make([]string, len(ent)-desp)
	for j := desp; j < len(ent); j++ {
		a := "e" + strconv.Itoa(j)
		if j == i {
			a = arg
		}
		if f.Variadica && j == len(ent)-1 {
			a += "..."
		}
		partes[j-desp] = a
	}
	return "F(" + strings.Join(partes, ", ") + ")"
}

// ordenable: values of t can be compared with < in Go.
func ordenable(t nucleo.Tipo) bool {
	switch t.Clase {
	case nucleo.CInt, nucleo.CFloat, nucleo.CString, nucleo.CRune, nucleo.CByte:
		return true
	}
	return false
}

// comparable: values of t can be map keys in Go.
func comparable(t nucleo.Tipo) bool {
	return ordenable(t) || t.Clase == nucleo.CBool
}

func numerico(t nucleo.Tipo) bool {
	return t.Clase == nucleo.CInt || t.Clase == nucleo.CFloat
}
