package mates

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Regression tests for the problems found in the review of this package.

func trazaPrueba() *nucleo.Nodo { return nucleo.NuevaTraza(nil, nil).Raiz() }

// sinPanico runs f and turns a panic into a test error.
func sinPanico(t *testing.T, nombre string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: pánico: %v", nombre, r)
		}
	}()
	f()
}

func TestRevisionInfinitosYNaN(t *testing.T) {
	for _, s := range []string{"sen(10^400)", "exp(1000)", "(10^400+1)^(1/3)", "cos(10^400)", "ln(10^400)*exp(800)"} {
		p := &nucleo.Pregunta{Texto: s}
		sinPanico(t, s, func() {
			ctx, cancel := context.WithTimeout(fondo, 2*time.Second)
			defer cancel()
			r, err := resCalculo{}.Resolver(ctx, p, trazaPrueba())
			if err == nil && (strings.Contains(r.Texto, "NaN") || strings.Contains(r.Texto, "Inf")) {
				t.Errorf("%q dio %q", s, r.Texto)
			}
		})
		sinPanico(t, s+" (comprobarValor)", func() {
			e, err := AnalizarExpr(s)
			if err != nil {
				t.Fatalf("%q: %v", s, err)
			}
			comprobarValor(e, vAprox(1), nil)
		})
	}
}

func TestRevisionRaizEnteraAcotada(t *testing.T) {
	for _, s := range []string{"((2^99999)^2+1)^(1/3)", "(2^99999+1)^(1/3)", "(2^150000)^(1/7)"} {
		sinPanico(t, s, func() {
			ctx, cancel := context.WithTimeout(fondo, 500*time.Millisecond)
			defer cancel()
			e, err := AnalizarExpr(s)
			if err != nil {
				t.Fatalf("%q: %v", s, err)
			}
			ini := time.Now()
			reducir(ctx, e)
			if d := time.Since(ini); d > 2*time.Second {
				t.Errorf("%q tardó %v", s, d)
			}
		})
	}
	// exact roots still work
	if got := calcular(t, "(2^300)^(1/3)").String(); got != new(big.Int).Lsh(big.NewInt(1), 100).String() {
		t.Errorf("(2^300)^(1/3) = %s", got)
	}
	if r, ok := raizEntera(big.NewInt(1000), 3); !ok || r.Int64() != 10 {
		t.Errorf("raíz cúbica de 1000 = %v %v", r, ok)
	}
	if _, ok := raizEntera(big.NewInt(1001), 3); ok {
		t.Errorf("1001 no es un cubo")
	}
}

func TestRevisionFactorizarRespetaPlazo(t *testing.T) {
	f3000 := new(big.Int).MulRange(1, 3000)
	ctx, cancel := context.WithTimeout(fondo, 300*time.Millisecond)
	defer cancel()
	ini := time.Now()
	fs, err := Factorizar(ctx, f3000)
	if d := time.Since(ini); d > time.Second {
		t.Errorf("Factorizar(3000!) tardó %v con un plazo de 300 ms", d)
	}
	if err == nil && producto(fs).Cmp(f3000) != 0 {
		t.Errorf("Factorizar(3000!) no da 3000! al multiplicar")
	}
	// a product of two primes near 2^120 cannot be finished in 50 ms: the deadline wins
	p1, _ := new(big.Int).SetString("1329227995784915872903807060280345027", 10) // about 2^120
	p1 = siguientePrimo(p1)
	p2 := siguientePrimo(new(big.Int).Add(p1, big.NewInt(1000)))
	n := new(big.Int).Mul(p1, p2)
	ctx2, cancel2 := context.WithTimeout(fondo, 50*time.Millisecond)
	defer cancel2()
	ini = time.Now()
	_, err = Factorizar(ctx2, n)
	if d := time.Since(ini); d > 500*time.Millisecond {
		t.Errorf("Factorizar(p·q) tardó %v con un plazo de 50 ms", d)
	}
	if !errors.Is(err, nucleo.ErrSinTiempo) {
		t.Errorf("Factorizar(p·q) con 50 ms: %v", err)
	}
	// the resolver reads «3000!» as a factorial and answers quickly
	r, err := resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: "factoriza 3000!"}, trazaPrueba())
	if err != nil || !strings.HasPrefix(r.Texto, "3000! = 2^2993 · 3^1498") {
		t.Errorf("factoriza 3000!: %q %v", recortar(r.Texto), err)
	}
	r, err = resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: "¿es primo 3000!?"}, trazaPrueba())
	if err != nil || !strings.HasPrefix(r.Texto, "No") {
		t.Errorf("¿es primo 3000!?: %q %v", r.Texto, err)
	}
	// «¡…!» is an exclamation, not a factorial
	r, err = resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: "¡factoriza 360!"}, trazaPrueba())
	if err != nil || r.Texto != "360 = 2^3 · 3^2 · 5" {
		t.Errorf("¡factoriza 360!: %q %v", r.Texto, err)
	}
}

func siguientePrimo(x *big.Int) *big.Int {
	p := new(big.Int).Set(x)
	if p.Bit(0) == 0 {
		p.Add(p, big.NewInt(1))
	}
	for !p.ProbablyPrime(20) {
		p.Add(p, big.NewInt(2))
	}
	return p
}

func TestRevisionEsPrimoCtx(t *testing.T) {
	m61 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 61), big.NewInt(1))
	m127 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	carmichael := big.NewInt(561)
	casos := []struct {
		x    *big.Int
		want bool
	}{{m61, true}, {m127, true}, {new(big.Int).Add(m127, big.NewInt(2)), false}, {carmichael, false},
		{new(big.Int).Mul(m61, m127), false}, {big.NewInt(2), true}, {big.NewInt(1), false}}
	for _, c := range casos {
		got, err := esPrimoCtx(fondo, c.x)
		if err != nil || got != c.want || EsPrimo(c.x) != c.want {
			t.Errorf("esPrimoCtx(%s) = %v %v, esperaba %v", c.x, got, err, c.want)
		}
	}
	vencido, cancel := context.WithCancel(fondo)
	cancel()
	if _, err := esPrimoCtx(vencido, m127); !errors.Is(err, nucleo.ErrSinTiempo) {
		t.Errorf("con el contexto cancelado: %v", err)
	}
}

func TestRevisionPolinomiosAcotados(t *testing.T) {
	casos := []string{
		"resuelve (a+b+c+d+e)^40=0",
		"resuelve (x+1)^64*(x+2)^64*(x+3)^64*(x+4)^64*(x+5)^64*(x+6)^64*(x+7)^64*(x+8)^64=1",
		"resuelve (x+y)^60*(x-y)^60=1",
		"resuelve x - y = 1, (x+y+1)^60 = 2",
	}
	for _, s := range casos {
		ctx, cancel := context.WithTimeout(fondo, 500*time.Millisecond)
		ini := time.Now()
		_, err := resEcuacion{}.Resolver(ctx, &nucleo.Pregunta{Texto: s}, trazaPrueba())
		d := time.Since(ini)
		cancel()
		if d > time.Second {
			t.Errorf("%q tardó %v con un plazo de 500 ms", s, d)
		}
		if err == nil {
			if !strings.Contains(s, "(x+y+1)^60") {
				t.Errorf("%q no debería resolverse", s)
			}
		} else if !errors.Is(err, nucleo.ErrNoSoportado) && !errors.Is(err, nucleo.ErrSinTiempo) {
			t.Errorf("%q: error inesperado %v", s, err)
		}
	}
	// Durand-Kerner stops when the context ends
	cs := make([]*big.Rat, 101)
	for i := range cs {
		cs[i] = big.NewRat(int64(i%7+1), 1)
	}
	vencido, cancel := context.WithCancel(fondo)
	cancel()
	if _, err := durandKerner(vencido, cs); !errors.Is(err, nucleo.ErrSinTiempo) {
		t.Errorf("durandKerner con el contexto cancelado: %v", err)
	}
	// a moderate expansion still works, also through substitution
	sol, err := Ecuaciones(fondo, []string{"x - y = 1, (x+y)^2 = 9"}, nil)
	if err != nil || !sol.Comprobado || len(sol.Valores["y"]) != 2 {
		t.Errorf("x - y = 1, (x+y)^2 = 9: %+v %v", sol, err)
	}
	sol, err = Ecuaciones(fondo, []string{"(x+1)^5 = 32"}, nil)
	if err != nil || !sol.Comprobado || len(sol.Valores["x"]) == 0 || sol.Valores["x"][0].String() != "1" {
		t.Errorf("(x+1)^5 = 32: %+v %v", sol, err)
	}
}

func TestRevisionPrimoTrazaHonesta(t *testing.T) {
	r, err := resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: "¿es primo 97?"}, trazaPrueba())
	if err != nil || !strings.HasPrefix(r.Texto, "Sí") {
		t.Fatalf("97: %q %v", r.Texto, err)
	}
	if !strings.Contains(strings.Join(r.Plan, "\n"), "√97 ≈ 9") {
		t.Errorf("97: el plan no cuenta la división: %q", r.Plan)
	}
	// the division really happens: a composite that Miller-Rabin rejects gets its divisor named
	r, err = resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: "¿es primo 91?"}, trazaPrueba())
	if err != nil || r.Texto != "No, 91 no es primo: 91 = 7 · 13" {
		t.Errorf("91: %q %v", r.Texto, err)
	}
	if d, ok := tanteoCompleto(1_000_000_007); ok {
		t.Errorf("10⁹+7 es primo, pero tanteoCompleto encontró %d", d)
	}
	if d, ok := tanteoCompleto(999_983 * 999_979); !ok || d != 999_979 {
		t.Errorf("999983·999979: %d %v", d, ok)
	}
	for _, s := range []string{"¿es primo 2?", "¿es primo 3?", "¿es primo 1000000007?"} {
		r, err := resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: s}, trazaPrueba())
		if err != nil || !strings.HasPrefix(r.Texto, "Sí") {
			t.Errorf("%s: %q %v", s, r.Texto, err)
		}
	}
	// numbers above 40 bits are not said to be divided by everything up to their root
	r, err = resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: "¿es primo 2^61-1?"}, trazaPrueba())
	if err != nil || !strings.HasPrefix(r.Texto, "Sí") || strings.Contains(strings.Join(r.Plan, " "), "hasta √") {
		t.Errorf("2^61-1: %q %q %v", r.Texto, r.Plan, err)
	}
}

func TestRevisionProblemasSituacion(t *testing.T) {
	casos := map[string]string{
		"Pedro tiene 5 manzanas. Compra 3 bolsas de 4 manzanas. ¿Cuántas manzanas tiene?":          "17",
		"Pepe tiene 7 lapices. Su hermana tiene 4 lapices. ¿Cuantos lapices tienen entre los dos?": "11",
		"Hay 24 caramelos y los reparto entre 4 niños. ¿Cuántos le tocan a cada uno?":              "6",
		"Ana tiene 5 años menos que Luis y Luis tiene 20 años. ¿Cuántos años tiene Ana?":           "15",
	}
	for texto, want := range casos {
		r, err := (&resProblema{}).Resolver(fondo, &nucleo.Pregunta{Texto: texto}, trazaPrueba())
		if err != nil {
			t.Errorf("%q: %v", texto, err)
			continue
		}
		if !strings.Contains(r.Texto, " "+want+" ") {
			t.Errorf("%q → %q, esperaba %s", texto, r.Texto, want)
		}
	}
}

func TestRevisionDominioDenominador(t *testing.T) {
	sol, err := Ecuaciones(fondo, []string{"x/x=1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sol.Tipo != "infinitas" || fmt.Sprint(sol.Condicion) != "[x ≠ 0]" || len(sol.Relacion) != 1 ||
		!strings.Contains(sol.Relacion[0], "x ≠ 0") {
		t.Errorf("x/x=1: %+v", sol)
	}
	sol, err = Ecuaciones(fondo, []string{"1/x - 1/x + 2 = 2"}, nil)
	if err != nil || sol.Tipo != "infinitas" || fmt.Sprint(sol.Condicion) != "[x ≠ 0]" {
		t.Errorf("1/x - 1/x + 2 = 2: %+v %v", sol, err)
	}
	sol, err = Ecuaciones(fondo, []string{"(x^2-1)/(x-1) = 2"}, nil)
	if err != nil || sol.Tipo != "ninguna" {
		t.Errorf("(x^2-1)/(x-1) = 2: %+v %v", sol, err)
	}
	// without unknowns in the denominator nothing changes
	sol, err = Ecuaciones(fondo, []string{"x - x = 0"}, nil)
	if err != nil || sol.Tipo != "infinitas" || len(sol.Condicion) != 0 {
		t.Errorf("x - x = 0: %+v %v", sol, err)
	}
}

func TestRevisionMcdMcmConCero(t *testing.T) {
	casos := map[string]string{
		"mcm de 0 y 5":   "mcm(0, 5) = 0",
		"mcm de 4 y 0":   "mcm(4, 0) = 0",
		"mcd de 0 y 0":   "mcd(0, 0) = 0",
		"mcd de 0 y 5":   "mcd(0, 5) = 5",
		"mcm de 4 y 6":   "mcm(4, 6) = 12",
		"mcd de 12 y 18": "mcd(12, 18) = 6",
	}
	for texto, want := range casos {
		r, err := resNumeros{}.Resolver(fondo, &nucleo.Pregunta{Texto: texto}, trazaPrueba())
		if err != nil || !strings.HasPrefix(r.Texto, want) || !r.Exito {
			t.Errorf("%q → %q %v, esperaba %q", texto, r.Texto, err, want)
		}
	}
	if MCM(big.NewInt(0), big.NewInt(5)).Sign() != 0 || MCD(big.NewInt(0), big.NewInt(0)).Sign() != 0 {
		t.Errorf("MCM/MCD con cero")
	}
}

// An unknown stem-changing verb is asked about by its real infinitive (trueca → trocar, not truecar).
func TestInfinitivoProbableDiptongo(t *testing.T) {
	casos := map[string]string{"trueca": "trocar", "truecan": "trocar", "troca": "trocar", "cierra": "cerrar",
		"frecuenta": "frecuentar", "bebe": "beber", "cuestiona": "cuestionar"}
	for w, quiero := range casos {
		if got := infinitivoProbable(w); got != quiero {
			t.Errorf("infinitivoProbable(%q) = %q, quiero %q", w, got, quiero)
		}
	}
}
