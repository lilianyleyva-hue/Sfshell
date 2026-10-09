package recetas

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"strconv"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// operaciones is the fixed table behind go_op: canonical name → Go operator.
var operaciones = map[string]string{
	"suma": "+", "resta": "-", "multiplicación": "*", "división": "/",
}

// sinonimosOp maps what people write to the canonical operation name.
var sinonimosOp = map[string]string{
	"suma": "suma", "sumar": "suma", "sumalos": "suma", "mas": "suma", "+": "suma", "adicion": "suma",
	"resta": "resta", "restar": "resta", "restalos": "resta", "menos": "resta", "-": "resta", "diferencia": "resta",
	"multiplicacion": "multiplicación", "multiplicar": "multiplicación", "multiplicalos": "multiplicación",
	"por": "multiplicación", "*": "multiplicación", "x": "multiplicación", "producto": "multiplicación",
	"division": "división", "dividir": "división", "dividelos": "división", "entre": "división", "/": "división",
	"cociente": "división",
}

// maxEntero bounds integer slots so that a template never prints for ever.
const maxEntero = 1_000_000

var funcionesPlantilla = template.FuncMap{
	"go_texto": func(s string) string { return strconv.Quote(s) },
	"go_entero": func(s string) (string, error) {
		n, err := strconv.Atoi(s)
		if err != nil {
			return "", fmt.Errorf("go_entero: %q no es un número entero", s)
		}
		return strconv.Itoa(n), nil
	},
	"go_op": func(s string) (string, error) {
		op, ok := operaciones[s]
		if !ok {
			return "", fmt.Errorf("go_op: no conozco la operación %q", s)
		}
		return op, nil
	},
}

// plantillaGo parses the template text.
func plantillaGo(p Plantilla) (*template.Template, error) {
	return template.New(p.Nombre).Funcs(funcionesPlantilla).Option("missingkey=error").Parse(p.Fuente)
}

// validarHueco checks a value against the slot's class and returns the canonical value.
func validarHueco(h Hueco, v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("falta el valor de «%s»", h.Nombre)
	}
	if !utf8.ValidString(v) || strings.ContainsAny(v, "\x00\n\r") {
		return "", fmt.Errorf("el valor de «%s» tiene caracteres que no admito", h.Nombre)
	}
	switch h.Clase {
	case "operacion":
		canon, ok := sinonimosOp[nucleo.Normalizar(v)]
		if !ok {
			return "", fmt.Errorf("no conozco la operación «%s»; puedo hacer: %s", v, strings.Join(h.Opciones, ", "))
		}
		if len(h.Opciones) > 0 && !contiene(h.Opciones, canon) {
			return "", fmt.Errorf("aquí solo puedo hacer: %s", strings.Join(h.Opciones, ", "))
		}
		return canon, nil
	case "entero":
		n, err := strconv.Atoi(v)
		if err != nil {
			return "", fmt.Errorf("«%s» no es un número entero", v)
		}
		if n < -maxEntero || n > maxEntero {
			return "", fmt.Errorf("%d es demasiado grande (el máximo es %d)", n, maxEntero)
		}
		return strconv.Itoa(n), nil
	case "texto":
		if utf8.RuneCountInString(v) > 200 {
			return "", fmt.Errorf("el texto de «%s» es demasiado largo (más de 200 caracteres)", h.Nombre)
		}
		for _, r := range v {
			if unicode.IsControl(r) {
				return "", fmt.Errorf("el valor de «%s» tiene caracteres de control", h.Nombre)
			}
		}
		return v, nil
	case "palabra":
		if utf8.RuneCountInString(v) > 40 {
			return "", fmt.Errorf("la palabra de «%s» es demasiado larga", h.Nombre)
		}
		for _, r := range v {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return "", fmt.Errorf("«%s» no es una sola palabra", v)
			}
		}
		if len(h.Opciones) > 0 && !contiene(h.Opciones, v) {
			return "", fmt.Errorf("«%s» no es una de las opciones: %s", v, strings.Join(h.Opciones, ", "))
		}
		return v, nil
	case "opcion":
		nv := nucleo.Normalizar(v)
		for _, o := range h.Opciones {
			if nucleo.Normalizar(o) == nv {
				return o, nil
			}
		}
		return "", fmt.Errorf("«%s» no es una de las opciones: %s", v, strings.Join(h.Opciones, ", "))
	}
	return "", fmt.Errorf("el hueco «%s» tiene una clase desconocida (%s)", h.Nombre, h.Clase)
}

func contiene(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// ErrFaltaHueco is wrapped by Instanciar when a slot has no value and no default.
var ErrFaltaHueco = errors.New("falta un dato")

// Instanciar fills the template: every slot value is validated against its class (missing values take
// PorDefecto), the template is executed (values reach the code only through go_texto, go_entero and
// go_op) and the result is gofmt'd.
func Instanciar(p Plantilla, valores map[string]string) (string, error) {
	datos := map[string]string{}
	for _, h := range p.Huecos {
		v, ok := valores[h.Nombre]
		if !ok || strings.TrimSpace(v) == "" {
			v = h.PorDefecto
		}
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("%w: %s", ErrFaltaHueco, h.Pregunta)
		}
		canon, err := validarHueco(h, v)
		if err != nil {
			return "", err
		}
		datos[h.Nombre] = canon
	}
	for k := range valores {
		if _, ok := datos[k]; !ok {
			return "", fmt.Errorf("la plantilla %s no tiene el dato «%s»", p.Nombre, k)
		}
	}
	t, err := plantillaGo(p)
	if err != nil {
		return "", fmt.Errorf("la plantilla %s está mal escrita: %v", p.Nombre, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, datos); err != nil {
		return "", fmt.Errorf("no pude rellenar la plantilla %s: %v", p.Nombre, err)
	}
	out, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("la plantilla %s da código que no se puede leer: %v", p.Nombre, err)
	}
	return string(out), nil
}
