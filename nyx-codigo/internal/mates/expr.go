package mates

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Expr is a parsed arithmetic expression. String renders it with "·" for products and the fewest
// parentheses.
type Expr interface{ String() string }

// Num is a number literal.
type Num struct {
	V     *big.Rat
	Texto string // as written ("3,5"); "" when it comes from a reduction
}

// Var is an unknown: x, y, n…
type Var struct{ Nombre string }

// Op is a binary operation: '+', '-', '*', '/', '^', '%' (mod).
type Op struct {
	Op   byte
	A, B Expr
}

// Neg is -A.
type Neg struct{ A Expr }

// Fact is A!.
type Fact struct{ A Expr }

// Func is a function of one argument: "√", "sen", "cos", "tan", "log", "ln", "exp", "abs".
type Func struct {
	Nombre string
	A      Expr
}

// Val is an already computed value (used while reducing step by step, and for substitution).
type Val struct{ v valor }

func (n Num) String() string { return textoRat(n.V) }
func (v Var) String() string { return v.Nombre }
func (v Val) String() string { return v.v.texto() }

func prec(e Expr) int {
	switch x := e.(type) {
	case Op:
		switch x.Op {
		case '+', '-':
			return 1
		case '*', '/', '%':
			return 2
		case '^':
			return 4
		}
	case Neg:
		return 3
	case Num:
		if !x.V.IsInt() || x.V.Sign() < 0 {
			return 2 // shown as "3/4" or "-3"
		}
	case Val:
		if x.v.conRadical() && x.v.a.Sign() != 0 {
			return 1
		}
		if x.v.signo() < 0 {
			return 3
		}
		if x.v.racional() && !x.v.a.IsInt() {
			return 2
		}
	}
	return 5
}

func envolver(e Expr, minimo int) string {
	if prec(e) < minimo {
		return "(" + e.String() + ")"
	}
	return e.String()
}

// negativoLit reports whether e is a negative literal value (written "(-2)" as a right operand).
func negativoLit(e Expr) bool {
	switch x := e.(type) {
	case Num:
		return x.V.Sign() < 0
	case Val:
		return !x.v.conRadical() && x.v.signo() < 0
	}
	return false
}

func (o Op) String() string {
	if negativoLit(o.B) && o.Op != '^' {
		izq := Op{Op: o.Op, A: o.A, B: Var{Nombre: "\x00"}}.String()
		return strings.Replace(izq, "\x00", "("+o.B.String()+")", 1)
	}
	p := prec(o)
	switch o.Op {
	case '^':
		return envolver(o.A, p+1) + "^" + envolver(o.B, p+1)
	case '-', '/', '%':
		op := string(o.Op)
		if o.Op == '%' {
			op = "mod"
		}
		sep := " "
		if o.Op == '/' {
			sep = ""
		}
		return envolver(o.A, p) + sep + op + sep + envolver(o.B, p+1)
	case '*':
		return envolver(o.A, p) + "·" + envolver(o.B, p+1)
	}
	return envolver(o.A, p) + " " + string(o.Op) + " " + envolver(o.B, p)
}

func (n Neg) String() string  { return "-" + envolver(n.A, 4) }
func (f Fact) String() string { return envolver(f.A, 5) + "!" }
func (f Func) String() string {
	switch f.Nombre {
	case "√":
		return "√" + envolver(f.A, 5)
	case "grados":
		return envolver(f.A, 5) + "°"
	}
	return f.Nombre + "(" + f.A.String() + ")"
}

// ---- lexer ----

type lexema struct {
	tipo  byte // 'n' number, 'v' variable, 'f' function, 'o' operator/symbol, '(' , ')'
	texto string
	num   *big.Rat
}

var funciones = map[string]string{
	"sen": "sen", "sin": "sen", "seno": "sen", "cos": "cos", "coseno": "cos", "tan": "tan", "tg": "tan",
	"tangente": "tan", "log": "log", "ln": "ln", "exp": "exp", "abs": "abs", "sqrt": "√", "raiz": "√",
}

var palabrasNumero = map[string]int64{
	"cero": 0, "uno": 1, "una": 1, "dos": 2, "tres": 3, "cuatro": 4, "cinco": 5, "seis": 6, "siete": 7,
	"ocho": 8, "nueve": 9, "diez": 10, "once": 11, "doce": 12, "trece": 13, "catorce": 14, "quince": 15,
	"dieciseis": 16, "diecisiete": 17, "dieciocho": 18, "diecinueve": 19, "veinte": 20, "veintiuno": 21,
	"veintidos": 22, "veintitres": 23, "veinticuatro": 24, "veinticinco": 25, "veintiseis": 26,
	"veintisiete": 27, "veintiocho": 28, "veintinueve": 29, "treinta": 30, "cuarenta": 40, "cincuenta": 50,
	"sesenta": 60, "setenta": 70, "ochenta": 80, "noventa": 90, "cien": 100, "ciento": 100,
	"doscientos": 200, "trescientos": 300, "cuatrocientos": 400, "quinientos": 500, "seiscientos": 600,
	"setecientos": 700, "ochocientos": 800, "novecientos": 900,
}

// numeroPalabras reads a number written in words from ws[i:] ("dos mil trescientos cuarenta y cinco").
// It returns the value and how many words it used (0 if none).
func numeroPalabras(ws []string, i int) (int64, int) {
	var total, grupo int64
	j, hay := i, false
	for j < len(ws) {
		w := ws[j]
		if v, ok := palabrasNumero[w]; ok {
			if (w == "una" || w == "uno") && !hay && !(j+1 < len(ws) && ws[j+1] == "mil") {
				if w == "una" {
					break
				}
			}
			grupo += v
			hay = true
			j++
			if v >= 30 && v < 100 && j+1 < len(ws) && ws[j] == "y" {
				if u, ok := palabrasNumero[ws[j+1]]; ok && u >= 1 && u <= 9 {
					grupo += u
					j += 2
				}
			}
			continue
		}
		if w == "mil" && (hay || j == i) {
			if grupo == 0 {
				grupo = 1
			}
			total += grupo * 1000
			grupo = 0
			hay = true
			j++
			continue
		}
		if (w == "millon" || w == "millones") && hay {
			total = (total + grupo) * 1_000_000
			grupo = 0
			j++
			continue
		}
		break
	}
	if !hay {
		return 0, 0
	}
	return total + grupo, j - i
}

// palabrasOperador maps Spanish words to operators; phrases are matched first.
var frasesOperador = []struct {
	palabras []string
	op       string
}{
	{[]string{"multiplicado", "por"}, "*"}, {[]string{"dividido", "entre"}, "/"}, {[]string{"dividido", "por"}, "/"},
	{[]string{"elevado", "al", "cuadrado"}, "^2"}, {[]string{"elevado", "al", "cubo"}, "^3"},
	{[]string{"elevado", "a", "la"}, "^"}, {[]string{"elevado", "al"}, "^"}, {[]string{"elevado", "a"}, "^"},
	{[]string{"al", "cuadrado"}, "^2"}, {[]string{"al", "cubo"}, "^3"},
	{[]string{"raiz", "cuadrada", "de"}, "√"}, {[]string{"raiz", "de"}, "√"}, {[]string{"raiz", "cuadrada"}, "√"},
	{[]string{"factorial", "de"}, "fact"}, {[]string{"el", "factorial", "de"}, "fact"},
	{[]string{"mas"}, "+"}, {[]string{"menos"}, "-"}, {[]string{"por"}, "*"}, {[]string{"entre"}, "/"},
	{[]string{"mod"}, "%"}, {[]string{"modulo"}, "%"}, {[]string{"resto", "de"}, "resto"},
}

func lexer(s string) ([]lexema, error) {
	// normalize: lowercase, no accents (keeping symbols)
	s = strings.NewReplacer("×", "*", "·", "*", "÷", "/", "−", "-", "–", "-", "**", "^", "²", "^2", "³", "^3",
		"≤", "<=", "≥", ">=", "π", " pi ", ":", "/").Replace(s)
	var out []lexema
	var palabras []string // pending word run
	vaciar := func() error {
		for k := 0; k < len(palabras); {
			w := palabras[k]
			if v, n := numeroPalabras(palabras, k); n > 0 {
				out = append(out, lexema{tipo: 'n', texto: strings.Join(palabras[k:k+n], " "), num: big.NewRat(v, 1)})
				k += n
				continue
			}
			hecho := false
			for _, f := range frasesOperador {
				if k+len(f.palabras) > len(palabras) {
					continue
				}
				ok := true
				for j, p := range f.palabras {
					if palabras[k+j] != p {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				switch {
				case f.op == "^2" || f.op == "^3":
					out = append(out, lexema{tipo: 'o', texto: "^"}, lexema{tipo: 'n', texto: f.op[1:], num: big.NewRat(int64(f.op[1]-'0'), 1)})
				case f.op == "√":
					out = append(out, lexema{tipo: 'f', texto: "√"})
				case f.op == "fact":
					out = append(out, lexema{tipo: 'f', texto: "fact"})
				case f.op == "resto":
					out = append(out, lexema{tipo: 'f', texto: "resto"})
				default:
					out = append(out, lexema{tipo: 'o', texto: f.op})
				}
				k += len(f.palabras)
				hecho = true
				break
			}
			if hecho {
				continue
			}
			switch {
			case w == "pi":
				out = append(out, lexema{tipo: 'n', texto: "π", num: nil})
			case funciones[w] != "":
				out = append(out, lexema{tipo: 'f', texto: funciones[w]})
			case w == "de" && len(out) > 0 && out[len(out)-1].tipo == 'o' && out[len(out)-1].texto == "%":
				out = append(out, lexema{tipo: 'o', texto: "de"})
			case w == "y" && len(out) > 0 && out[len(out)-1].tipo == 'f' && out[len(out)-1].texto == "resto":
				out = append(out, lexema{tipo: 'o', texto: "%"})
			case len(w) <= 2 && esLetras(w) && !palabrasCortas[w]:
				for _, r := range w {
					out = append(out, lexema{tipo: 'v', texto: string(r)})
				}
			default:
				return fmt.Errorf("%w: no sé qué es «%s» en una operación", nucleo.ErrNoEntiendo, w)
			}
			k++
		}
		palabras = palabras[:0]
		return nil
	}
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case unicode.IsSpace(r):
			i += n
		case unicode.IsLetter(r):
			j := i
			for j < len(s) {
				r2, n2 := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsLetter(r2) {
					break
				}
				j += n2
			}
			palabras = append(palabras, nucleo.Normalizar(s[i:j]))
			i = j
		case r >= '0' && r <= '9' || r == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9':
			if err := vaciar(); err != nil {
				return nil, err
			}
			j := i
			for j < len(s) && (s[j] >= '0' && s[j] <= '9') {
				j++
			}
			// decimal point or comma: "3.5", "3,5" (not "3, 5")
			if j+1 < len(s) && (s[j] == '.' || s[j] == ',') && s[j+1] >= '0' && s[j+1] <= '9' {
				k := j + 1
				for k < len(s) && s[k] >= '0' && s[k] <= '9' {
					k++
				}
				j = k
			}
			txt := s[i:j]
			v, ok := new(big.Rat).SetString(strings.Replace(txt, ",", ".", 1))
			if !ok {
				return nil, fmt.Errorf("%w: el número «%s» está mal escrito", nucleo.ErrNoEntiendo, txt)
			}
			out = append(out, lexema{tipo: 'n', texto: txt, num: v})
			i = j
		default:
			if err := vaciar(); err != nil {
				return nil, err
			}
			switch r {
			case '+', '-', '*', '/', '^', '!', '%', '√', '°':
				out = append(out, lexema{tipo: 'o', texto: string(r)})
			case '(', '[', '{':
				out = append(out, lexema{tipo: '(', texto: "("})
			case ')', ']', '}':
				out = append(out, lexema{tipo: ')', texto: ")"})
			case '?', '¿', '=':
				// question marks around a calculation are ignored; "=" at the end ("2+2=") too
				if r == '=' && strings.TrimSpace(s[i+n:]) != "" {
					return nil, fmt.Errorf("%w: hay un «=»: eso es una ecuación", nucleo.ErrNoEntiendo)
				}
			default:
				return nil, fmt.Errorf("%w: no sé qué es «%c» en una operación", nucleo.ErrNoEntiendo, r)
			}
			i += n
		}
	}
	if err := vaciar(); err != nil {
		return nil, err
	}
	return out, nil
}

// palabrasCortas are Spanish words that are never read as unknowns ("un" is not u·n).
var palabrasCortas = map[string]bool{"un": true, "el": true, "la": true, "lo": true, "de": true, "es": true,
	"en": true, "si": true, "se": true, "al": true, "le": true, "me": true, "te": true, "mi": true, "tu": true,
	"su": true, "ya": true, "no": true, "da": true, "va": true, "ha": true, "he": true, "os": true, "ni": true}

func esLetras(w string) bool {
	for _, r := range w {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return w != ""
}

// ---- Pratt parser ----

type parser struct {
	lx []lexema
	i  int
}

func (p *parser) mirar() *lexema {
	if p.i < len(p.lx) {
		return &p.lx[p.i]
	}
	return nil
}

func (p *parser) es(tipo byte, texto string) bool {
	l := p.mirar()
	return l != nil && l.tipo == tipo && (texto == "" || l.texto == texto)
}

// AnalizarExpr parses an arithmetic expression (Pratt): + - * / ^ ! % mod √ ( ), implicit
// multiplication (2x, 3(x+1), (a)(b)), Spanish words ("más", "menos", "por", "entre", "elevado a",
// "al cuadrado", "raíz de", "factorial de"), numbers in words ("dos mil"), and "3,5" with a decimal comma.
// "N% de E" is N/100·E; a "%" between two operands is the remainder.
func AnalizarExpr(s string) (Expr, error) {
	lx, err := lexer(s)
	if err != nil {
		return nil, err
	}
	if len(lx) == 0 {
		return nil, fmt.Errorf("%w: no hay ninguna operación", nucleo.ErrNoEntiendo)
	}
	p := &parser{lx: lx}
	e, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	if l := p.mirar(); l != nil {
		return nil, fmt.Errorf("%w: me sobra «%s» al final", nucleo.ErrNoEntiendo, l.texto)
	}
	return e, nil
}

// infijo returns the binding powers of the operator at the cursor (0 if there is none). Implicit
// multiplication has the power of "*".
func (p *parser) infijo() (op byte, izq, der int, implicito bool) {
	l := p.mirar()
	if l == nil {
		return 0, 0, 0, false
	}
	switch l.tipo {
	case 'o':
		switch l.texto {
		case "+", "-":
			return l.texto[0], 10, 11, false
		case "*", "/":
			return l.texto[0], 20, 21, false
		case "%":
			// remainder only when an operand follows
			if p.i+1 < len(p.lx) && (p.lx[p.i+1].tipo == 'n' || p.lx[p.i+1].tipo == 'v' || p.lx[p.i+1].tipo == '(') {
				return '%', 20, 21, false
			}
			return 0, 0, 0, false
		case "^":
			return '^', 31, 30, false // right associative
		case "√":
			return '*', 20, 21, true
		}
	case 'n', 'v', '(', 'f':
		return '*', 20, 21, true
	}
	return 0, 0, 0, false
}

func (p *parser) expr(minimo int) (Expr, error) {
	izq, err := p.prefijo()
	if err != nil {
		return nil, err
	}
	for {
		// postfix: ! and % (percent) and ° (degrees)
		if l := p.mirar(); l != nil && l.tipo == 'o' && (l.texto == "!" || l.texto == "°" || l.texto == "%") {
			if l.texto == "%" {
				if op, _, _, _ := p.infijo(); op == '%' {
					goto infijo
				}
			}
			if 40 < minimo {
				break
			}
			p.i++
			switch l.texto {
			case "!":
				izq = Fact{A: izq}
			case "°":
				izq = Func{Nombre: "grados", A: izq}
			case "%":
				izq = Op{Op: '/', A: izq, B: Num{V: big.NewRat(100, 1)}}
				// "15% de 240"
				if p.esPalabraDe() {
					p.i++
					der, err := p.expr(21)
					if err != nil {
						return nil, err
					}
					izq = Op{Op: '*', A: izq, B: der}
				}
			}
			continue
		}
	infijo:
		op, bi, bd, implicito := p.infijo()
		if op == 0 || bi < minimo {
			break
		}
		if !implicito {
			p.i++
		}
		der, err := p.expr(bd)
		if err != nil {
			return nil, err
		}
		izq = Op{Op: op, A: izq, B: der}
	}
	return izq, nil
}

// esPalabraDe: "de" is not a lexeme; the lexer rejects it, so "N% de E" is rewritten before parsing.
func (p *parser) esPalabraDe() bool { return p.es('o', "de") }

func (p *parser) prefijo() (Expr, error) {
	l := p.mirar()
	if l == nil {
		return nil, fmt.Errorf("%w: la operación está incompleta", nucleo.ErrNoEntiendo)
	}
	p.i++
	switch l.tipo {
	case 'n':
		if l.num == nil { // π
			return Val{v: vAprox(math.Pi)}, nil
		}
		return Num{V: l.num, Texto: l.texto}, nil
	case 'v':
		return Var{Nombre: l.texto}, nil
	case '(':
		e, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		if !p.es(')', "") {
			return nil, fmt.Errorf("%w: falta cerrar un paréntesis", nucleo.ErrNoEntiendo)
		}
		p.i++
		return e, nil
	case 'f':
		switch l.texto {
		case "fact":
			a, err := p.expr(35)
			if err != nil {
				return nil, err
			}
			return Fact{A: a}, nil
		case "resto":
			a, err := p.expr(21)
			if err != nil {
				return nil, err
			}
			if !p.es('o', "%") && !p.es('o', "/") {
				return nil, fmt.Errorf("%w: «el resto de» necesita «entre»", nucleo.ErrNoEntiendo)
			}
			p.i++
			b, err := p.expr(21)
			if err != nil {
				return nil, err
			}
			return Op{Op: '%', A: a, B: b}, nil
		}
		a, err := p.expr(35)
		if err != nil {
			return nil, err
		}
		return Func{Nombre: l.texto, A: a}, nil
	case 'o':
		switch l.texto {
		case "-":
			a, err := p.expr(25)
			if err != nil {
				return nil, err
			}
			if n, ok := a.(Num); ok {
				return Num{V: new(big.Rat).Neg(n.V), Texto: "-" + n.Texto}, nil
			}
			return Neg{A: a}, nil
		case "+":
			return p.expr(25)
		case "√":
			a, err := p.expr(35)
			if err != nil {
				return nil, err
			}
			return Func{Nombre: "√", A: a}, nil
		}
	}
	return nil, fmt.Errorf("%w: no esperaba «%s»", nucleo.ErrNoEntiendo, l.texto)
}

// variables lists the unknowns of e in order of appearance.
func variables(e Expr) []string {
	var out []string
	visto := map[string]bool{}
	var rec func(Expr)
	rec = func(e Expr) {
		switch x := e.(type) {
		case Var:
			if !visto[x.Nombre] {
				visto[x.Nombre] = true
				out = append(out, x.Nombre)
			}
		case Op:
			rec(x.A)
			rec(x.B)
		case Neg:
			rec(x.A)
		case Fact:
			rec(x.A)
		case Func:
			rec(x.A)
		}
	}
	rec(e)
	return out
}

// sustituir replaces variables by values.
func sustituir(e Expr, vals map[string]valor) Expr {
	switch x := e.(type) {
	case Var:
		if v, ok := vals[x.Nombre]; ok {
			return Val{v: v}
		}
	case Op:
		return Op{Op: x.Op, A: sustituir(x.A, vals), B: sustituir(x.B, vals)}
	case Neg:
		return Neg{A: sustituir(x.A, vals)}
	case Fact:
		return Fact{A: sustituir(x.A, vals)}
	case Func:
		return Func{Nombre: x.Nombre, A: sustituir(x.A, vals)}
	}
	return e
}
