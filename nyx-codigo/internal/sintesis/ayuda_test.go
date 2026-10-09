package sintesis

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// Helpers shared by the internal tests: a tiny signature reader and the task-file reader.

// leerFirma reads "func Nombre(a, b int, xs []int) res".
func leerFirma(s string) (nucleo.Firma, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "func "))
	i := strings.Index(s, "(")
	j := strings.LastIndex(s, ")")
	if i < 0 || j < i {
		return nucleo.Firma{}, fmt.Errorf("firma mal escrita: %q", s)
	}
	f := nucleo.Firma{Nombre: strings.TrimSpace(s[:i])}
	var pendientes []string
	for _, parte := range strings.Split(s[i+1:j], ",") {
		parte = strings.TrimSpace(parte)
		if parte == "" {
			continue
		}
		campos := strings.Fields(parte)
		if len(campos) == 1 {
			pendientes = append(pendientes, campos[0])
			continue
		}
		t, err := nucleo.ParseTipo(strings.Join(campos[1:], " "))
		if err != nil {
			return f, err
		}
		for _, n := range append(pendientes, campos[0]) {
			f.Params = append(f.Params, nucleo.Param{Nombre: n, Tipo: t})
		}
		pendientes = nil
	}
	if len(pendientes) > 0 {
		return f, fmt.Errorf("faltan tipos en %q", s)
	}
	res := strings.TrimSpace(s[j+1:])
	if res != "" {
		t, err := nucleo.ParseTipo(res)
		if err != nil {
			return f, err
		}
		f.Res = []nucleo.Tipo{t}
	}
	return f, nil
}

func debeFirma(t testing.TB, s string) nucleo.Firma {
	t.Helper()
	f, err := leerFirma(s)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// partirFuera splits s at sep outside quotes and brackets.
func partirFuera(s, sep string) []string {
	var out []string
	prof := 0
	var comilla byte
	ini := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if comilla != 0 {
			if ch == '\\' {
				i++
			} else if ch == comilla {
				comilla = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			comilla = ch
		case '[', '{', '(':
			prof++
		case ']', '}', ')':
			prof--
		default:
			if prof == 0 && strings.HasPrefix(s[i:], sep) {
				out = append(out, s[ini:i])
				i += len(sep) - 1
				ini = i + 1
			}
		}
	}
	return append(out, s[ini:])
}

func leerCaso(f nucleo.Firma, s string) (nucleo.Caso, error) {
	lados := partirFuera(s, "=>")
	if len(lados) != 2 {
		return nucleo.Caso{}, fmt.Errorf("falta «=>» en %q", s)
	}
	ents := partirFuera(lados[0], ";")
	if len(ents) != len(f.Params) {
		return nucleo.Caso{}, fmt.Errorf("%q: %d entradas, la firma tiene %d", s, len(ents), len(f.Params))
	}
	var c nucleo.Caso
	for i, e := range ents {
		v, err := nucleo.ParseValor(strings.TrimSpace(e), f.Params[i].Tipo)
		if err != nil {
			return c, fmt.Errorf("%q: %v", e, err)
		}
		c.Entradas = append(c.Entradas, v)
	}
	y, err := nucleo.ParseValor(strings.TrimSpace(lados[1]), f.Res[0])
	if err != nil {
		return c, fmt.Errorf("%q: %v", lados[1], err)
	}
	c.Esperado = []nucleo.Valor{y}
	c.Expectativa = nucleo.EspUsuario
	c.Origen = nucleo.OrigenUsuario
	return c, nil
}

func leerMarco(s string) (*nucleo.Marco, error) {
	m := &nucleo.Marco{}
	for _, campo := range partirFuera(strings.TrimSpace(s), " ") {
		campo = strings.TrimSpace(campo)
		if campo == "" {
			continue
		}
		k, v, ok := strings.Cut(campo, "=")
		if !ok {
			return nil, fmt.Errorf("campo de marco mal escrito: %q", campo)
		}
		switch k {
		case "accion":
			m.Accion = v
		case "objeto":
			m.Objeto = v
		case "elemento":
			m.Elemento = v
		case "salida":
			m.Salida = v
		case "mod":
			md := nucleo.Modificador{}
			c, arg, hay := strings.Cut(v, ":")
			md.Concepto = c
			if strings.HasPrefix(c, "!") {
				md.Concepto, md.Negado = c[1:], true
			}
			if hay {
				if x, err := strconv.ParseFloat(arg, 64); err == nil {
					md.Numeros = []float64{x}
				} else if u, err := strconv.Unquote(arg); err == nil {
					md.Texto = u
				} else {
					md.Texto = arg
				}
			}
			m.Mods = append(m.Mods, md)
		default:
			return nil, fmt.Errorf("campo de marco desconocido: %q", k)
		}
	}
	return m, nil
}

type tarea struct {
	nombre  string
	firma   nucleo.Firma
	ejs     []nucleo.Caso
	pruebas []nucleo.Caso
	marco   *nucleo.Marco
}

func leerTareas(ruta string) ([]tarea, error) {
	fh, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	var out []tarea
	var cur *tarea
	sc := bufio.NewScanner(fh)
	n := 0
	for sc.Scan() {
		n++
		lin := strings.TrimSpace(sc.Text())
		if lin == "" || strings.HasPrefix(lin, "#") {
			continue
		}
		if strings.HasPrefix(lin, "==") {
			out = append(out, tarea{nombre: strings.TrimSpace(lin[2:])})
			cur = &out[len(out)-1]
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("línea %d: fuera de una tarea", n)
		}
		clave, valor, ok := strings.Cut(lin, ":")
		if !ok {
			return nil, fmt.Errorf("línea %d: falta «:»", n)
		}
		valor = strings.TrimSpace(valor)
		switch clave {
		case "firma":
			f, err := leerFirma(valor)
			if err != nil {
				return nil, fmt.Errorf("línea %d: %v", n, err)
			}
			cur.firma = f
		case "ej", "prueba":
			c, err := leerCaso(cur.firma, valor)
			if err != nil {
				return nil, fmt.Errorf("línea %d: %v", n, err)
			}
			if clave == "ej" {
				cur.ejs = append(cur.ejs, c)
			} else {
				cur.pruebas = append(cur.pruebas, c)
			}
		case "marco":
			m, err := leerMarco(valor)
			if err != nil {
				return nil, fmt.Errorf("línea %d: %v", n, err)
			}
			cur.marco = m
		default:
			return nil, fmt.Errorf("línea %d: clave desconocida %q", n, clave)
		}
	}
	return out, sc.Err()
}

func debeParse(t testing.TB, r *Registro, f nucleo.Firma, s string) *Expr {
	t.Helper()
	e, err := Parse(s, r, f)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return e
}

// cumple reports whether e gives the expected value on every case.
func cumple(e *Expr, casos []nucleo.Caso) (bool, string) {
	for _, c := range casos {
		v, err := Evaluar(e, c.Entradas)
		if err != nil {
			return false, fmt.Sprintf("con %v: %v", c.Entradas, err)
		}
		if !nucleo.Igual(v, c.Esperado[0]) {
			return false, fmt.Sprintf("con %v da %v y debía dar %v", c.Entradas, v, c.Esperado[0])
		}
	}
	return true, ""
}
