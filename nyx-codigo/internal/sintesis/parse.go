package sintesis

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Parse reads an s-expression program: "(suma (filtra (λ x (esPar x)) nums))". Parameters are referred to
// by name; "?" is a hole (its type comes from the position); a bare primitive name in a lambda position is
// eta-expanded ("(filtra esPar nums)"). The result must have the firma's result type when the firma has one.
func Parse(s string, r *Registro, f nucleo.Firma) (*Expr, error) {
	if r == nil {
		return nil, fmt.Errorf("sintesis: falta el registro de primitivas")
	}
	l := &lectorS{s: s}
	n, err := l.nodo()
	if err != nil {
		return nil, err
	}
	l.espacios()
	if l.i < len(l.s) {
		return nil, fmt.Errorf("sintesis: sobra texto al final: %q", l.s[l.i:])
	}
	p := &parser{r: r, cache: map[string]resultadoParse{}}
	for _, prm := range f.Params {
		p.nombres = append(p.nombres, prm.Nombre)
		p.tipos = append(p.tipos, prm.Tipo)
	}
	var esperado *nucleo.Tipo
	if len(f.Res) == 1 {
		esperado = &f.Res[0]
	}
	return p.expr(n, esperado)
}

// ---- s-expression reader ----

type nodoS struct {
	atomo string
	hijos []*nodoS
	lista bool
}

func (n *nodoS) String() string {
	if !n.lista {
		return n.atomo
	}
	partes := make([]string, len(n.hijos))
	for i, h := range n.hijos {
		partes[i] = h.String()
	}
	return "(" + strings.Join(partes, " ") + ")"
}

type lectorS struct {
	s string
	i int
}

func (l *lectorS) espacios() {
	for l.i < len(l.s) {
		r, n := utf8.DecodeRuneInString(l.s[l.i:])
		if !unicode.IsSpace(r) {
			return
		}
		l.i += n
	}
}

func (l *lectorS) nodo() (*nodoS, error) {
	l.espacios()
	if l.i >= len(l.s) {
		return nil, fmt.Errorf("sintesis: la expresión está incompleta")
	}
	switch l.s[l.i] {
	case '(':
		l.i++
		n := &nodoS{lista: true}
		for {
			l.espacios()
			if l.i >= len(l.s) {
				return nil, fmt.Errorf("sintesis: falta «)»")
			}
			if l.s[l.i] == ')' {
				l.i++
				return n, nil
			}
			h, err := l.nodo()
			if err != nil {
				return nil, err
			}
			n.hijos = append(n.hijos, h)
		}
	case ')':
		return nil, fmt.Errorf("sintesis: sobra «)»")
	case '"', '\'':
		q := l.s[l.i]
		j := l.i + 1
		for j < len(l.s) && l.s[j] != q {
			if l.s[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(l.s) {
			return nil, fmt.Errorf("sintesis: falta cerrar las comillas")
		}
		tok := l.s[l.i : j+1]
		l.i = j + 1
		return &nodoS{atomo: tok}, nil
	case '[', '{':
		prof := 0
		j := l.i
		for j < len(l.s) {
			ch := l.s[j]
			switch ch {
			case '[', '{':
				prof++
			case ']', '}':
				prof--
			case '"', '\'':
				k := j + 1
				for k < len(l.s) && l.s[k] != ch {
					if l.s[k] == '\\' {
						k++
					}
					k++
				}
				j = k
			}
			j++
			if prof == 0 {
				break
			}
		}
		if prof != 0 {
			return nil, fmt.Errorf("sintesis: falta cerrar «%c»", l.s[l.i])
		}
		tok := l.s[l.i:j]
		l.i = j
		return &nodoS{atomo: tok}, nil
	}
	j := l.i
	for j < len(l.s) {
		r, n := utf8.DecodeRuneInString(l.s[j:])
		if unicode.IsSpace(r) || r == '(' || r == ')' {
			break
		}
		j += n
	}
	tok := l.s[l.i:j]
	l.i = j
	return &nodoS{atomo: tok}, nil
}

// ---- typing and resolution ----

type parser struct {
	r       *Registro
	nombres []string // scope: index = variable slot
	tipos   []nucleo.Tipo
	cache   map[string]resultadoParse
}

type resultadoParse struct {
	e   *Expr
	err error
}

func esLambdaCabeza(s string) bool { return s == "λ" || s == "lambda" || s == `\` || s == "fn" }

func (p *parser) firmaAlcance() string {
	var sb strings.Builder
	for i, n := range p.nombres {
		sb.WriteString(n)
		sb.WriteString(":")
		sb.WriteString(p.tipos[i].ClaveTipo())
		sb.WriteString(";")
	}
	return sb.String()
}

func (p *parser) expr(n *nodoS, esperado *nucleo.Tipo) (*Expr, error) {
	clave := fmt.Sprintf("%p|%s|", n, p.firmaAlcance())
	if esperado != nil {
		clave += esperado.ClaveTipo()
	}
	if r, ok := p.cache[clave]; ok {
		return r.e, r.err
	}
	e, err := p.exprSinCache(n, esperado)
	if err == nil && esperado != nil && !mismoTipo(e.Tipo, *esperado) {
		err = fmt.Errorf("sintesis: %s da %s y aquí hace falta %s", n, e.Tipo.Go(), esperado.Go())
		e = nil
	}
	p.cache[clave] = resultadoParse{e, err}
	return e, err
}

func (p *parser) exprSinCache(n *nodoS, esperado *nucleo.Tipo) (*Expr, error) {
	if !n.lista {
		return p.atomo(n.atomo, esperado)
	}
	if len(n.hijos) == 0 {
		return nil, fmt.Errorf("sintesis: «()» vacío")
	}
	cab := n.hijos[0]
	if cab.lista {
		return nil, fmt.Errorf("sintesis: %s no es el nombre de una primitiva", cab)
	}
	if esLambdaCabeza(cab.atomo) {
		return nil, fmt.Errorf("sintesis: aquí no puede ir una función (λ)")
	}
	cands := p.r.instancias(cab.atomo)
	if len(cands) == 0 {
		return nil, fmt.Errorf("sintesis: no conozco la primitiva «%s»", cab.atomo)
	}
	args := n.hijos[1:]
	var primerErr error
	for _, prim := range cands {
		if len(prim.Args) != len(args) {
			if primerErr == nil {
				primerErr = fmt.Errorf("sintesis: «%s» recibe %d argumentos y hay %d", cab.atomo, len(prim.Args), len(args))
			}
			continue
		}
		if esperado != nil && !mismoTipo(prim.Res, *esperado) {
			if primerErr == nil {
				primerErr = fmt.Errorf("sintesis: «%s» da %s y aquí hace falta %s", cab.atomo, prim.Res.Go(), esperado.Go())
			}
			continue
		}
		e, err := p.aplicar(prim, args)
		if err == nil {
			return e, nil
		}
		if primerErr == nil || strings.Contains(primerErr.Error(), "argumentos") || strings.Contains(primerErr.Error(), "aquí hace falta") {
			primerErr = err
		}
	}
	return nil, primerErr
}

func (p *parser) aplicar(prim *Primitiva, args []*nodoS) (*Expr, error) {
	e := &Expr{Op: prim, Var: -1, Tipo: prim.Res}
	for i, a := range args {
		if prim.esLambda(i) {
			continue
		}
		t := prim.Args[i]
		h, err := p.expr(a, &t)
		if err != nil {
			return nil, err
		}
		e.Hijos = append(e.Hijos, h)
	}
	for i, a := range args {
		if !prim.esLambda(i) {
			continue
		}
		l, err := p.lambda(a, prim.Lambdas[i])
		if err != nil {
			return nil, err
		}
		e.Lambdas = append(e.Lambdas, l)
	}
	return e, nil
}

func (p *parser) lambda(n *nodoS, lt LambdaTipo) (*Lambda, error) {
	base := len(p.nombres)
	if !n.lista {
		if strings.HasPrefix(n.atomo, "?") {
			return &Lambda{Params: lt.Params, Cuerpo: nuevoHueco(lt.Res)}, nil
		}
		// eta: a bare primitive name
		for _, prim := range p.r.instancias(n.atomo) {
			if len(prim.Args) != len(lt.Params) || prim.numLambdas() > 0 || !mismoTipo(prim.Res, lt.Res) {
				continue
			}
			ok := true
			for i, a := range prim.Args {
				if !mismoTipo(a, lt.Params[i]) {
					ok = false
				}
			}
			if !ok {
				continue
			}
			usados := map[string]bool{}
			for _, nm := range p.nombres {
				usados[nm] = true
			}
			nombres := nombresLambda(len(lt.Params), usados)
			cuerpo := &Expr{Op: prim, Var: -1, Tipo: prim.Res}
			for i, t := range lt.Params {
				cuerpo.Hijos = append(cuerpo.Hijos, nuevaVar(base+i, t, nombres[i]))
			}
			return &Lambda{Params: lt.Params, Cuerpo: cuerpo, nombres: nombres}, nil
		}
		return nil, fmt.Errorf("sintesis: aquí hace falta una función de %s y «%s» no lo es", lt.clave(), n.atomo)
	}
	if len(n.hijos) < 3 || n.hijos[0].lista || !esLambdaCabeza(n.hijos[0].atomo) {
		return nil, fmt.Errorf("sintesis: aquí hace falta una función (λ …) y hay %s", n)
	}
	var nombres []string
	if n.hijos[1].lista {
		if len(n.hijos) != 3 {
			return nil, fmt.Errorf("sintesis: λ mal escrita: %s", n)
		}
		for _, h := range n.hijos[1].hijos {
			if h.lista {
				return nil, fmt.Errorf("sintesis: λ mal escrita: %s", n)
			}
			nombres = append(nombres, h.atomo)
		}
	} else {
		for _, h := range n.hijos[1 : len(n.hijos)-1] {
			if h.lista {
				return nil, fmt.Errorf("sintesis: λ mal escrita: %s", n)
			}
			nombres = append(nombres, h.atomo)
		}
	}
	if len(nombres) != len(lt.Params) {
		return nil, fmt.Errorf("sintesis: la función necesita %d parámetros y tiene %d", len(lt.Params), len(nombres))
	}
	viejosN, viejosT := p.nombres, p.tipos
	p.nombres = append(append([]string(nil), p.nombres...), nombres...)
	p.tipos = append(append([]nucleo.Tipo(nil), p.tipos...), lt.Params...)
	defer func() { p.nombres, p.tipos = viejosN, viejosT }()
	res := lt.Res
	cuerpo, err := p.expr(n.hijos[len(n.hijos)-1], &res)
	if err != nil {
		return nil, err
	}
	return &Lambda{Params: lt.Params, Cuerpo: cuerpo, nombres: nombres}, nil
}

func (p *parser) atomo(a string, esperado *nucleo.Tipo) (*Expr, error) {
	if strings.HasPrefix(a, "?") {
		if esperado == nil {
			return nil, fmt.Errorf("sintesis: no sé de qué tipo es el hueco «%s»", a)
		}
		return nuevoHueco(*esperado), nil
	}
	for i := len(p.nombres) - 1; i >= 0; i-- {
		if p.nombres[i] == a {
			return nuevaVar(i, p.tipos[i], a), nil
		}
	}
	switch a {
	case "true", "verdadero":
		return nuevaConst(true, tB), nil
	case "false", "falso":
		return nuevaConst(false, tB), nil
	}
	if strings.HasPrefix(a, `"`) {
		s, err := strconv.Unquote(a)
		if err != nil {
			return nil, fmt.Errorf("sintesis: texto mal escrito: %s", a)
		}
		return nuevaConst(s, tS), nil
	}
	if strings.HasPrefix(a, "'") {
		s, err := strconv.Unquote(a)
		if err != nil || utf8.RuneCountInString(s) != 1 {
			return nil, fmt.Errorf("sintesis: carácter mal escrito: %s", a)
		}
		r, _ := utf8.DecodeRuneInString(s)
		return nuevaConst(int(r), tR), nil
	}
	if strings.HasPrefix(a, "[") || strings.HasPrefix(a, "{") {
		if esperado != nil {
			v, err := nucleo.ParseValor(a, *esperado)
			if err != nil {
				return nil, fmt.Errorf("sintesis: %v", err)
			}
			return nuevaConst(v, *esperado), nil
		}
		v, t, err := nucleo.Inferir(a)
		if err != nil {
			return nil, fmt.Errorf("sintesis: %v", err)
		}
		if idTipo(t) < 0 {
			return nil, fmt.Errorf("sintesis: no sé de qué tipo es %s", a)
		}
		return nuevaConst(v, t), nil
	}
	if i, err := strconv.Atoi(a); err == nil {
		if esperado != nil && esperado.Clase == nucleo.CFloat {
			return nuevaConst(float64(i), tF), nil
		}
		return nuevaConst(i, tI), nil
	}
	if f, err := strconv.ParseFloat(a, 64); err == nil && strings.ContainsAny(a, ".eE") {
		return nuevaConst(f, tF), nil
	}
	if len(p.r.instancias(a)) > 0 {
		return nil, fmt.Errorf("sintesis: «%s» es una primitiva: escríbela entre paréntesis con sus argumentos", a)
	}
	return nil, fmt.Errorf("sintesis: no conozco «%s»", a)
}
