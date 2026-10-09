package pruebas

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"nyxcodigo/internal/nucleo"
)

// evaluador is a tiny interpreter for property expressions, enough for tests: the five harness helpers
// (nyx__Ordenada, nyx__EsPermutacion, nyx__Subsecuencia, nyx__Contiene, nyx__Igual), F(...), len,
// append with "...", conversions of nil, slicing, comparisons, + - / and the boolean operators.
// Function literals are not supported (evaluar returns an error).
type evaluador struct {
	vars map[string]nucleo.Valor
	F    func(args []nucleo.Valor) nucleo.Valor
}

func evaluarProp(expr string, entradas, salidas []nucleo.Valor, F func([]nucleo.Valor) nucleo.Valor) (bool, error) {
	ev := evaluador{vars: map[string]nucleo.Valor{}, F: F}
	for i, v := range entradas {
		ev.vars["e"+strconv.Itoa(i)] = nucleo.Copiar(v)
	}
	for i, v := range salidas {
		ev.vars["r"+strconv.Itoa(i)] = v
	}
	x, err := parser.ParseExpr(expr)
	if err != nil {
		return false, err
	}
	v, err := ev.eval(x)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("la propiedad no da un bool: %v", v)
	}
	return b, nil
}

func (ev evaluador) eval(x ast.Expr) (nucleo.Valor, error) {
	switch n := x.(type) {
	case *ast.ParenExpr:
		return ev.eval(n.X)
	case *ast.Ident:
		if n.Name == "nil" {
			return nil, nil
		}
		if n.Name == "true" || n.Name == "false" {
			return n.Name == "true", nil
		}
		v, ok := ev.vars[n.Name]
		if !ok {
			return nil, fmt.Errorf("variable desconocida %s", n.Name)
		}
		return v, nil
	case *ast.BasicLit:
		if n.Kind == token.INT {
			i, err := strconv.Atoi(n.Value)
			return i, err
		}
		return nil, fmt.Errorf("literal no soportado %s", n.Value)
	case *ast.UnaryExpr:
		v, err := ev.eval(n.X)
		if err != nil {
			return nil, err
		}
		if n.Op == token.NOT {
			return !v.(bool), nil
		}
		return nil, fmt.Errorf("operador %s no soportado", n.Op)
	case *ast.BinaryExpr:
		a, err := ev.eval(n.X)
		if err != nil {
			return nil, err
		}
		switch n.Op {
		case token.LAND:
			if !a.(bool) {
				return false, nil
			}
			return ev.eval(n.Y)
		case token.LOR:
			if a.(bool) {
				return true, nil
			}
			return ev.eval(n.Y)
		}
		b, err := ev.eval(n.Y)
		if err != nil {
			return nil, err
		}
		switch n.Op {
		case token.EQL:
			return nucleo.Igual(a, b), nil
		case token.NEQ:
			return !nucleo.Igual(a, b), nil
		case token.LSS:
			return nucleo.Comparar(a, b) < 0, nil
		case token.LEQ:
			return nucleo.Comparar(a, b) <= 0, nil
		case token.GTR:
			return nucleo.Comparar(a, b) > 0, nil
		case token.GEQ:
			return nucleo.Comparar(a, b) >= 0, nil
		case token.ADD:
			if fa, ok := a.(float64); ok {
				return fa + b.(float64), nil
			}
			return a.(int) + b.(int), nil
		case token.SUB:
			return a.(int) - b.(int), nil
		case token.QUO:
			return a.(int) / b.(int), nil
		}
		return nil, fmt.Errorf("operador %s no soportado", n.Op)
	case *ast.SliceExpr:
		v, err := ev.eval(n.X)
		if err != nil {
			return nil, err
		}
		xs, _ := v.([]nucleo.Valor)
		lo, hi := 0, len(xs)
		if n.Low != nil {
			l, err := ev.eval(n.Low)
			if err != nil {
				return nil, err
			}
			lo = l.(int)
		}
		if n.High != nil {
			h, err := ev.eval(n.High)
			if err != nil {
				return nil, err
			}
			hi = h.(int)
		}
		return append([]nucleo.Valor(nil), xs[lo:hi]...), nil
	case *ast.CallExpr:
		if _, ok := n.Fun.(*ast.ArrayType); ok { // []T(nil)
			return []nucleo.Valor(nil), nil
		}
		id, ok := n.Fun.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("llamada no soportada")
		}
		if _, ok := n.Fun.(*ast.FuncLit); ok {
			return nil, fmt.Errorf("funciones literales no soportadas")
		}
		args := make([]nucleo.Valor, len(n.Args))
		for i, a := range n.Args {
			v, err := ev.eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		lista := func(i int) []nucleo.Valor { xs, _ := args[i].([]nucleo.Valor); return xs }
		switch id.Name {
		case "len":
			switch x := args[0].(type) {
			case string:
				return len(x), nil
			case nucleo.Mapa:
				return len(x), nil
			}
			return len(lista(0)), nil
		case "append":
			out := append([]nucleo.Valor(nil), lista(0)...)
			if n.Ellipsis.IsValid() {
				return append(out, lista(1)...), nil
			}
			return append(out, args[1:]...), nil
		case "F":
			return ev.F(args), nil
		case "nyx__Igual":
			return nucleo.Igual(args[0], args[1]), nil
		case "nyx__Ordenada":
			xs := lista(0)
			for i := 1; i < len(xs); i++ {
				if nucleo.Comparar(xs[i-1], xs[i]) > 0 {
					return false, nil
				}
			}
			return true, nil
		case "nyx__EsPermutacion":
			a, b := lista(0), lista(1)
			if len(a) != len(b) {
				return false, nil
			}
			cuenta := map[string]int{}
			for _, v := range a {
				cuenta[nucleo.Clave(v)]++
			}
			for _, v := range b {
				cuenta[nucleo.Clave(v)]--
				if cuenta[nucleo.Clave(v)] < 0 {
					return false, nil
				}
			}
			return true, nil
		case "nyx__Subsecuencia":
			sub, todo := lista(0), lista(1)
			j := 0
			for _, v := range todo {
				if j < len(sub) && nucleo.Igual(sub[j], v) {
					j++
				}
			}
			return j == len(sub), nil
		case "nyx__Contiene":
			for _, v := range lista(0) {
				if nucleo.Igual(v, args[1]) {
					return true, nil
				}
			}
			return false, nil
		}
		return nil, fmt.Errorf("función %s no soportada", id.Name)
	case *ast.FuncLit:
		return nil, fmt.Errorf("funciones literales no soportadas")
	}
	return nil, fmt.Errorf("expresión no soportada %T", x)
}
