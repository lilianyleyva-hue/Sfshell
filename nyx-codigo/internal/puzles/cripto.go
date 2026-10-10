package puzles

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// termino is one word of a cryptarithm with its sign (+1 on the left of "=", −1 on the right
// after moving everything to one side).
type terminoCripto struct {
	palabra []rune
	signo   int
}

// LeerCripto parses "SEND+MORE=MONEY" (also with "-" and spaces) into signed words.
func LeerCripto(expr string) ([]terminoCripto, error) {
	expr = strings.ToUpper(strings.TrimSpace(expr))
	partes := strings.Split(expr, "=")
	if len(partes) != 2 {
		return nil, fmt.Errorf("puzles: la suma de letras necesita un solo «=»: %w", nucleo.ErrNoEntiendo)
	}
	var out []terminoCripto
	for lado, texto := range partes {
		signo := 1
		if lado == 1 {
			signo = -1
		}
		actual := []rune{}
		op := 1
		cerrar := func() error {
			if len(actual) == 0 {
				return fmt.Errorf("puzles: falta una palabra en «%s»: %w", strings.TrimSpace(texto), nucleo.ErrNoEntiendo)
			}
			out = append(out, terminoCripto{palabra: actual, signo: signo * op})
			actual = []rune{}
			return nil
		}
		for _, r := range texto {
			switch {
			case unicode.IsLetter(r):
				actual = append(actual, r)
			case r == '+' || r == '-' || r == '−':
				if err := cerrar(); err != nil {
					return nil, err
				}
				op = 1
				if r != '+' {
					op = -1
				}
			case unicode.IsSpace(r):
			default:
				return nil, fmt.Errorf("puzles: no entiendo «%c» en la suma de letras: %w", r, nucleo.ErrNoEntiendo)
			}
		}
		if err := cerrar(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Cripto solves a cryptarithm column by column with carries (leading letters ≠ 0) and returns the
// first solution. See CriptoTodas for the uniqueness check.
func Cripto(ctx context.Context, expr string) (map[rune]int, error) {
	sols, _, err := CriptoTodas(ctx, expr)
	if err != nil {
		return nil, err
	}
	return sols[0], nil
}

// CriptoTodas finds up to 2 solutions (so the caller can say whether it is unique), each re-checked
// with exact arithmetic. It also returns the number of search nodes.
func CriptoTodas(ctx context.Context, expr string) ([]map[rune]int, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ts, err := LeerCripto(expr)
	if err != nil {
		return nil, 0, err
	}
	maxLen := 0
	lideres := map[rune]bool{}
	for _, t := range ts {
		maxLen = max(maxLen, len(t.palabra))
		if len(t.palabra) > 1 {
			lideres[t.palabra[0]] = true
		}
	}
	// letters in the order they first appear by column, right to left
	var letras []rune
	porColumna := make([][]rune, maxLen)
	visto := map[rune]bool{}
	for c := 0; c < maxLen; c++ {
		for _, t := range ts {
			if c < len(t.palabra) {
				r := t.palabra[len(t.palabra)-1-c]
				if !visto[r] {
					visto[r] = true
					letras = append(letras, r)
					porColumna[c] = append(porColumna[c], r)
				}
			}
		}
	}
	if len(letras) > 10 {
		return nil, 0, fmt.Errorf("puzles: hay %d letras distintas y solo hay 10 cifras: %w", len(letras), nucleo.ErrNoEntiendo)
	}
	valor := map[rune]int{}
	usado := [10]bool{}
	var sols []map[rune]int
	nodos := 0
	limite := time.Now().Add(TiempoCSP)
	var errBusca error
	var columna func(c, acarreo int) bool
	var asignar func(c, k, acarreo int) bool
	columna = func(c, acarreo int) bool {
		if c == maxLen {
			if acarreo == 0 {
				m := map[rune]int{}
				for r, v := range valor {
					m[r] = v
				}
				sols = append(sols, m)
				return len(sols) < 2
			}
			return true
		}
		return asignar(c, 0, acarreo)
	}
	asignar = func(c, k, acarreo int) bool {
		if k < len(porColumna[c]) {
			r := porColumna[c][k]
			for d := 0; d <= 9; d++ {
				if usado[d] || (d == 0 && lideres[r]) {
					continue
				}
				nodos++
				if nodos&0xFFF == 0 && (ctx.Err() != nil || time.Now().After(limite)) {
					errBusca = fmt.Errorf("puzles: sin tiempo con la suma de letras: %w", nucleo.ErrSinTiempo)
					return false
				}
				usado[d] = true
				valor[r] = d
				seguir := asignar(c, k+1, acarreo)
				usado[d] = false
				delete(valor, r)
				if !seguir {
					return false
				}
			}
			return true
		}
		s := acarreo
		for _, t := range ts {
			if c < len(t.palabra) {
				s += t.signo * valor[t.palabra[len(t.palabra)-1-c]]
			}
		}
		if s%10 != 0 {
			return true
		}
		return columna(c+1, s/10)
	}
	columna(0, 0)
	if errBusca != nil {
		return nil, nodos, errBusca
	}
	if len(sols) == 0 {
		return nil, nodos, fmt.Errorf("puzles: «%s» no tiene solución: %w", strings.TrimSpace(expr), nucleo.ErrNoEntiendo)
	}
	for _, s := range sols {
		if err := comprobarCripto(ts, s); err != nil {
			return nil, nodos, fmt.Errorf("puzles: la solución no cumple (error interno): %v", err)
		}
	}
	return sols, nodos, nil
}

func comprobarCripto(ts []terminoCripto, s map[rune]int) error {
	usados := map[int]rune{}
	for r, v := range s {
		if v < 0 || v > 9 {
			return fmt.Errorf("%c = %d no es una cifra", r, v)
		}
		if o, ok := usados[v]; ok {
			return fmt.Errorf("%c y %c valen %d", r, o, v)
		}
		usados[v] = r
	}
	total := new(big.Int)
	for _, t := range ts {
		if len(t.palabra) > 1 && s[t.palabra[0]] == 0 {
			return fmt.Errorf("%s empieza por 0", string(t.palabra))
		}
		v := valorPalabra(t.palabra, s)
		if t.signo < 0 {
			total.Sub(total, v)
		} else {
			total.Add(total, v)
		}
	}
	if total.Sign() != 0 {
		return fmt.Errorf("la cuenta no cuadra")
	}
	return nil
}

func valorPalabra(p []rune, s map[rune]int) *big.Int {
	v := new(big.Int)
	diez := big.NewInt(10)
	for _, r := range p {
		v.Mul(v, diez)
		v.Add(v, big.NewInt(int64(s[r])))
	}
	return v
}

// EscribirCripto writes the solved sum with digits: "9567 + 1085 = 10652".
func EscribirCripto(expr string, s map[rune]int) string {
	var sb strings.Builder
	for _, r := range strings.ToUpper(expr) {
		if v, ok := s[r]; ok {
			sb.WriteByte(byte('0' + v))
		} else if r == '+' || r == '-' || r == '=' {
			sb.WriteString(" " + string(r) + " ")
		} else if !unicode.IsSpace(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// TextoAsignacion lists the letters in alphabetical order: "D=7, E=5, …".
func TextoAsignacion(s map[rune]int) string {
	var ls []rune
	for r := range s {
		ls = append(ls, r)
	}
	sort.Slice(ls, func(i, j int) bool { return ls[i] < ls[j] })
	var partes []string
	for _, r := range ls {
		partes = append(partes, fmt.Sprintf("%c=%d", r, s[r]))
	}
	return strings.Join(partes, ", ")
}
