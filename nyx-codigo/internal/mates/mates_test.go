package mates

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

var fondo = context.Background()

func calcular(t *testing.T, s string) Numero {
	t.Helper()
	v, err := Calcular(fondo, s, nil)
	if err != nil {
		t.Fatalf("Calcular(%q): %v", s, err)
	}
	return v
}

func TestCalcularExacto(t *testing.T) {
	casos := map[string]string{
		"1/3+1/6":                 "1/2",
		"3/4+1/6":                 "11/12",
		"cuánto es 3/4+1/6":       "11/12",
		"(2+3)*4":                 "20",
		"2+3*4":                   "14",
		"2^10":                    "1024",
		"√50":                     "5√2",
		"raíz cuadrada de 50":     "5√2",
		"√2·√2":                   "2",
		"(1+√5)/2":                "(1 + √5)/2",
		"(√3+1)^2":                "4 + 2√3",
		"3,5 + 1":                 "9/2",
		"dos más tres por cuatro": "14",
		"15% de 240":              "36",
		"17 mod 5":                "2",
		"-3^2":                    "-9",
		"2^-1":                    "1/2",
		"8^(1/3)":                 "2",
		"7 entre 2":               "7/2",
		"5!":                      "120",
		"log(1000)":               "3",
		"2·(3 + 4)":               "14",
		"6:3":                     "2",
		"10 - 2 - 3":              "5",
		"2^3^2":                   "512",
	}
	for s, want := range casos {
		if got := calcular(t, s).String(); got != want {
			t.Errorf("Calcular(%q) = %s, esperaba %s", s, got, want)
		}
	}
	if d := len(calcular(t, "100!").Exacto.Num().String()); d != 158 {
		t.Errorf("100! tiene %d cifras, deberían ser 158", d)
	}
	if got := calcular(t, "sen(30°)"); got.EsExacto || got.String() != "≈ 0,5" {
		t.Errorf("sen 30° = %v", got)
	}
}

func TestCalcularErrores(t *testing.T) {
	inicio := time.Now()
	_, err := Calcular(fondo, "2^(10^9)", nil)
	if !errors.Is(err, ErrDemasiadoGrande) {
		t.Errorf("2^(10^9) debería ser demasiado grande: %v", err)
	}
	if d := time.Since(inicio); d > 100*time.Millisecond {
		t.Errorf("2^(10^9) tardó %v en rechazarse", d)
	}
	if _, err := Calcular(fondo, "5001!", nil); !errors.Is(err, ErrDemasiadoGrande) {
		t.Errorf("5001! debería ser demasiado grande: %v", err)
	}
	if _, err := Calcular(fondo, "1/0", nil); !errors.Is(err, ErrDivisionCero) {
		t.Errorf("1/0: %v", err)
	}
	if _, err := Calcular(fondo, "patata frita", nil); !errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Errorf("galimatías: %v", err)
	}
	if _, err := Calcular(fondo, "2x+1", nil); !errors.Is(err, ErrIncognita) {
		t.Errorf("con incógnita: %v", err)
	}
	if _, err := Calcular(fondo, "√-4", nil); err == nil {
		t.Errorf("√-4 debería fallar")
	}
}

func TestPasosReduccion(t *testing.T) {
	tr := nucleo.NuevaTraza(nil, nil)
	e, err := AnalizarExpr("(2+3)*4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluar(fondo, e, tr.Raiz()); err != nil {
		t.Fatal(err)
	}
	var detalle string
	for _, p := range tr.Pasos() {
		detalle += p.Titulo + "\n" + p.Detalle + "\n"
	}
	if !strings.Contains(detalle, "→ 5·4") || !strings.Contains(detalle, "→ 20") {
		t.Errorf("faltan los pasos (2+3)·4 → 5·4 → 20:\n%s", detalle)
	}
	// long reductions are collapsed
	_, pasos, err := reducir(fondo, mustExpr(t, "1+2+3+4+5+6+7+8+9+10+11+12+13+14+15+16+17+18+19+20+21+22+23+24+25+26+27+28+29+30+31+32+33+34+35"))
	if err != nil {
		t.Fatal(err)
	}
	if r := resumirPasos(pasos); len(r) > maxPasosVisibles+1 || !strings.Contains(strings.Join(r, "|"), "pasos más") {
		t.Errorf("los pasos no se resumen: %d", len(r))
	}
}

func mustExpr(t *testing.T, s string) Expr {
	t.Helper()
	e, err := AnalizarExpr(s)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestNumeroString(t *testing.T) {
	casos := []struct {
		n    Numero
		want string
	}{
		{Numero{Exacto: big.NewRat(11, 12), EsExacto: true}, "11/12"},
		{Numero{Exacto: big.NewRat(4, 1), EsExacto: true}, "4"},
		{Numero{Mas: big.NewRat(3, 1), Radical: &Radical{Coef: big.NewRat(2, 1), Radicando: big.NewInt(5)}, EsExacto: true}, "3 + 2√5"},
		{Numero{Mas: big.NewRat(5, 2), Radical: &Radical{Coef: big.NewRat(-1, 2), Radicando: big.NewInt(13)}, EsExacto: true}, "(5 - √13)/2"},
		{Numero{Mas: new(big.Rat), Radical: &Radical{Coef: big.NewRat(1, 1), Radicando: big.NewInt(2)}, EsExacto: true}, "√2"},
		{Numero{Aprox: 1.41421356}, "≈ 1,4142"},
		{Numero{Aprox: 2.5}, "≈ 2,5"},
	}
	for _, c := range casos {
		if got := c.n.String(); got != c.want {
			t.Errorf("String() = %q, esperaba %q", got, c.want)
		}
	}
}

func valores(t *testing.T, sol Solucion, v string) []string {
	t.Helper()
	var out []string
	for _, n := range sol.Valores[v] {
		out = append(out, n.String())
	}
	return out
}

func TestEcuacionLineal(t *testing.T) {
	tr := nucleo.NuevaTraza(nil, nil)
	sol, err := Ecuaciones(fondo, []string{"2x+3=11"}, tr.Raiz())
	if err != nil {
		t.Fatal(err)
	}
	if got := valores(t, sol, "x"); len(got) != 1 || got[0] != "4" || sol.Tipo != "unica" || !sol.Comprobado {
		t.Errorf("2x+3=11 → %v %s %v", got, sol.Tipo, sol.Comprobado)
	}
	traza := ""
	for _, p := range tr.Pasos() {
		traza += strings.ToLower(p.Titulo) + "\n"
	}
	if !strings.Contains(traza, "resto 3") || !strings.Contains(traza, "divido entre 2") {
		t.Errorf("la traza no tiene «resto 3»:\n%s", traza)
	}
	if !strings.Contains(strings.Join(sol.Comprobacion, ""), "2·4 + 3 = 11 ✔") {
		t.Errorf("comprobación: %v", sol.Comprobacion)
	}
	for ec, want := range map[string]string{"3(x+1) = 12": "3", "x/2 + 1 = 4": "6", "11 = 2x + 3": "4", "5x - 2 = 13": "3",
		"2x + 1 = x - 4": "-5", "x/3 + x/6 = 1": "2"} {
		sol, err := Ecuaciones(fondo, []string{ec}, nil)
		if err != nil || len(sol.Valores["x"]) != 1 || sol.Valores["x"][0].String() != want || !sol.Comprobado {
			t.Errorf("%s → %v (%v)", ec, sol.Valores, err)
		}
	}
}

func TestEcuacionSinOInfinitas(t *testing.T) {
	sol, err := Ecuaciones(fondo, []string{"0x=5"}, nil)
	if err != nil || sol.Tipo != "ninguna" {
		t.Errorf("0x=5 → %s %v", sol.Tipo, err)
	}
	sol, err = Ecuaciones(fondo, []string{"0x=0"}, nil)
	if err != nil || sol.Tipo != "infinitas" {
		t.Errorf("0x=0 → %s %v", sol.Tipo, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^2 + 1 = 0"}, nil)
	if err != nil || sol.Tipo != "ninguna" {
		t.Errorf("x²+1=0 → %s %v", sol.Tipo, err)
	}
}

func TestEcuacionCuadratica(t *testing.T) {
	sol, err := Ecuaciones(fondo, []string{"x^2-5x+6=0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "2,3" || !sol.Comprobado {
		t.Errorf("x^2-5x+6=0 → %v %v", sol.Valores, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^2=2"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "-√2,√2" || !sol.Comprobado {
		t.Errorf("x^2=2 → %v %v", sol.Valores, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^2 - 2x - 1 = 0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "1 - √2,1 + √2" || !sol.Comprobado {
		t.Errorf("x^2-2x-1=0 → %v %v", valores(t, sol, "x"), err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^2 + x - 1 = 0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "(-1 - √5)/2,(-1 + √5)/2" || !sol.Comprobado {
		t.Errorf("x^2+x-1=0 → %v %v", valores(t, sol, "x"), err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^2 - 4x + 4 = 0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "2" || sol.Tipo != "unica" {
		t.Errorf("raíz doble → %v %s", valores(t, sol, "x"), sol.Tipo)
	}
}

func TestEcuacionGradoAlto(t *testing.T) {
	sol, err := Ecuaciones(fondo, []string{"x^3 - 6x^2 + 11x - 6 = 0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "1,2,3" || !sol.Comprobado {
		t.Errorf("cúbica → %v %v", sol.Valores, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^4 - 5x^2 + 4 = 0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "-2,-1,1,2" {
		t.Errorf("cuártica → %v %v", sol.Valores, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^3 - 2x = 0"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "-√2,0,√2" {
		t.Errorf("x^3-2x → %v %v", valores(t, sol, "x"), err)
	}
	sol, err = Ecuaciones(fondo, []string{"x^5 - x - 1 = 0"}, nil)
	if err != nil || sol.Tipo != "aproximada" || !sol.Comprobado || len(sol.Valores["x"]) != 1 {
		t.Fatalf("quíntica → %v %s %v", sol.Valores, sol.Tipo, err)
	}
	if x := sol.Valores["x"][0].Aprox; x < 1.1672 || x > 1.1674 {
		t.Errorf("raíz de x^5-x-1 ≈ %v", x)
	}
}

func TestEcuacionConDenominador(t *testing.T) {
	sol, err := Ecuaciones(fondo, []string{"(x^2-1)/(x-1) = 3"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "2" || len(sol.Condicion) == 0 || sol.Condicion[0] != "x ≠ 1" {
		t.Errorf("con denominador → %v %v %v", sol.Valores, sol.Condicion, err)
	}
}

func TestSistemas(t *testing.T) {
	sol, err := Ecuaciones(fondo, []string{"x+y=10, x-y=2"}, nil)
	if err != nil || valores(t, sol, "x")[0] != "6" || valores(t, sol, "y")[0] != "4" || !sol.Comprobado {
		t.Errorf("x+y=10, x-y=2 → %v %v", sol.Valores, err)
	}
	if !strings.Contains(strings.Join(sol.Pasos, "\n"), "F2 → F2 - F1") {
		t.Errorf("faltan las operaciones de fila: %v", sol.Pasos)
	}
	sol, err = Ecuaciones(fondo, []string{"x + y + z = 6", "x - y = 0", "2z = 6"}, nil)
	if err != nil || valores(t, sol, "z")[0] != "3" || valores(t, sol, "x")[0] != "3/2" {
		t.Errorf("3×3 → %v %v", sol.Valores, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x+y=1; 2x+2y=3"}, nil)
	if err != nil || sol.Tipo != "ninguna" {
		t.Errorf("incompatible → %s %v", sol.Tipo, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x+y=1, 2x+2y=2"}, nil)
	if err != nil || sol.Tipo != "infinitas" || len(sol.Relacion) != 1 || !sol.Comprobado {
		t.Errorf("indeterminado → %s %v %v", sol.Tipo, sol.Relacion, err)
	}
	sol, err = Ecuaciones(fondo, []string{"x+y=10 y x*y=21"}, nil)
	if err != nil || strings.Join(valores(t, sol, "x"), ",") != "7,3" || !sol.Comprobado {
		t.Errorf("sustitución → %v %v", sol.Valores, err)
	}
	if _, err := Ecuaciones(fondo, []string{"x*y=1, x*x+y*y=4"}, nil); !errors.Is(err, nucleo.ErrNoSoportado) {
		t.Errorf("no lineal sin ecuación lineal: %v", err)
	}
}

func TestInecuacion(t *testing.T) {
	casos := map[string]string{
		"2x-1 > 5":      "x > 3",
		"-2x + 4 <= 10": "x ≥ -3",
		"x/3 < 2":       "x < 6",
		"3 > x":         "x < 3",
		"5 - x ≥ 2x":    "x ≤ 5/3",
		"0x > -1":       "se cumple para cualquier x",
		"0x > 1":        "no se cumple para ningún x",
	}
	for s, want := range casos {
		got, err := Inecuacion(fondo, s, nil)
		if err != nil || got != want {
			t.Errorf("Inecuacion(%q) = %q, %v; esperaba %q", s, got, err, want)
		}
	}
	tr := nucleo.NuevaTraza(nil, nil)
	if _, err := Inecuacion(fondo, "-2x > 4", tr.Raiz()); err != nil {
		t.Fatal(err)
	}
	traza := ""
	for _, p := range tr.Pasos() {
		traza += p.Titulo + "\n"
	}
	if !strings.Contains(traza, "doy la vuelta al signo") {
		t.Errorf("al dividir entre negativo hay que decirlo:\n%s", traza)
	}
	if _, err := Inecuacion(fondo, "x^2 > 4", nil); !errors.Is(err, nucleo.ErrNoSoportado) {
		t.Errorf("no lineal: %v", err)
	}
}

func TestPrimosYFactores(t *testing.T) {
	inicio := time.Now()
	fs, err := Factorizar(fondo, big.NewInt(600851475143))
	if err != nil || textoFactores(fs) != "71 · 839 · 1471 · 6857" {
		t.Errorf("600851475143 → %s %v", textoFactores(fs), err)
	}
	if d := time.Since(inicio); d > time.Second {
		t.Errorf("tardó %v", d)
	}
	m := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 61), big.NewInt(1))
	if !EsPrimo(m) {
		t.Errorf("2^61-1 es primo")
	}
	if EsPrimo(big.NewInt(91)) || EsPrimo(big.NewInt(1)) || !EsPrimo(big.NewInt(97)) {
		t.Errorf("EsPrimo mal")
	}
	// Pollard-Brent: product of two primes above the trial-division limit
	x, _ := new(big.Int).SetString("1000000016000000063", 10)
	fs, err = Factorizar(fondo, x)
	if err != nil || textoFactores(fs) != "1000000007 · 1000000009" {
		t.Errorf("Pollard-Brent → %s %v", textoFactores(fs), err)
	}
	fs, err = Factorizar(fondo, big.NewInt(360))
	if err != nil || textoFactores(fs) != "2^3 · 3^2 · 5" {
		t.Errorf("360 → %s", textoFactores(fs))
	}
	if MCD(big.NewInt(48), big.NewInt(18)).Int64() != 6 || MCM(big.NewInt(4), big.NewInt(6)).Int64() != 12 {
		t.Errorf("mcd/mcm")
	}
	ds, err := Divisores(big.NewInt(36))
	if err != nil || listaInts(ds) != "1, 2, 3, 4, 6, 9, 12, 18, 36" {
		t.Errorf("divisores de 36: %v", ds)
	}
	if _, err := Divisores(big.NewInt(0)); err == nil {
		t.Errorf("el 0 no tiene lista de divisores")
	}
}

func plantearUno(t *testing.T, texto string, v Verbos) (Planteamiento, Solucion) {
	t.Helper()
	pls, err := Plantear(&nucleo.Pregunta{Texto: texto}, v)
	if err != nil || len(pls) == 0 {
		t.Fatalf("Plantear(%q): %v", texto, err)
	}
	sol, err := Ecuaciones(fondo, pls[0].Ecuaciones, nil)
	if err != nil || !sol.Comprobado {
		t.Fatalf("%q: %v %v", texto, pls[0].Ecuaciones, err)
	}
	return pls[0], sol
}

func TestProblemas(t *testing.T) {
	pl, sol := plantearUno(t, "el doble de un número más 3 es 11", nil)
	if valores(t, sol, "x")[0] != "4" || !strings.Contains(strings.Join(pl.Mapeo, "\n"), "«el doble de un número» → 2·x") {
		t.Errorf("doble: %v %v", sol.Valores, pl.Mapeo)
	}
	_, sol = plantearUno(t, "María tiene 12 caramelos y regala 5. ¿Cuántos le quedan?", nil)
	if valores(t, sol, "x")[0] != "7" {
		t.Errorf("caramelos: %v", sol.Valores)
	}
	pl, sol = plantearUno(t, "un tren va a 80 km/h durante 2 horas y media, ¿qué distancia recorre?", nil)
	if valores(t, sol, "x")[0] != "200" || pl.Unidad != "km" {
		t.Errorf("tren: %v %q", sol.Valores, pl.Unidad)
	}
	_, sol = plantearUno(t, "Ana tiene el triple de la edad de Luis y entre los dos suman 48", nil)
	if valores(t, sol, "x")[0] != "36" || valores(t, sol, "y")[0] != "12" {
		t.Errorf("edades: %v", sol.Valores)
	}
	casos := map[string]string{
		"el triple de un número menos 4 es 20":                                             "8",
		"la suma de un número y su doble es 30":                                            "10",
		"si a un número le sumo 5 obtengo 12":                                              "7",
		"la suma de dos números consecutivos es 15":                                        "7",
		"Pedro tiene 20 euros y gasta 8. ¿Cuánto le queda?":                                "12",
		"Luis tiene 7 cromos y Ana le da 5. ¿Cuántos tiene Luis?":                          "12",
		"Ana tiene 15 canicas y le da 4 a Luis, que tenía 3. ¿Cuántas tiene Luis?":         "7",
		"En una caja hay 20 manzanas y se comen 7. ¿Cuántas quedan?":                       "13",
		"un coche recorre 300 km a 60 km/h. ¿Cuánto tarda?":                                "5",
		"una camisa cuesta 40 euros y tiene un descuento del 25%. ¿Cuánto cuesta?":         "30",
		"compro 3 kilos de manzanas a 2 euros el kilo. ¿Cuánto pago?":                      "6",
		"reparte 20 caramelos entre 4 amigos. ¿Cuántos le tocan a cada uno?":               "5",
		"Juan tiene 3 peces y compra 4. ¿Cuántos tiene ahora?":                             "7",
		"Ana compra 3 cajas con 12 caramelos cada una. ¿Cuántos caramelos tiene?":          "36",
		"Luis tiene 5 bolsas de 4 canicas y pierde 3 canicas. ¿Cuántas canicas le quedan?": "17",
		"compro 500 gramos de queso a 12 euros el kilo. ¿Cuánto pago?":                     "6",
		"un ciclista va a 20 km/h. ¿Cuántos minutos tarda en recorrer 5 km?":               "15",
		"un coche va a 90 km/h durante 20 minutos. ¿Qué distancia recorre?":                "30",
		"un tren recorre 150 km en 2 horas. ¿A qué velocidad va?":                          "75",
		"un corredor va a 5 m/s durante 2 minutos. ¿Cuántos metros recorre?":               "600",
	}
	for texto, want := range casos {
		_, sol := plantearUno(t, texto, nil)
		if got := valores(t, sol, "x"); len(got) == 0 || got[0] != want {
			t.Errorf("%q → %v, esperaba %s", texto, got, want)
		}
	}
	_, sol = plantearUno(t, "Dentro de 5 años, Ana tendrá el doble de la edad de Luis. Ana tiene 25 años", nil)
	if valores(t, sol, "y")[0] != "10" {
		t.Errorf("dentro de 5 años: %v", sol.Valores)
	}
	if _, err := Plantear(&nucleo.Pregunta{Texto: "blablá patata frita"}, nil); !errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Errorf("galimatías: %v", err)
	}
}

// verbosFalsos is an in-memory Verbos.
type verbosFalsos struct {
	mu sync.Mutex
	m  map[string]string
}

func (v *verbosFalsos) Efecto(verbo string) (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	e, ok := v.m[verbo]
	return e, ok
}

func (v *verbosFalsos) Aprender(verbo, efecto string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.m[verbo] = efecto
	return nil
}

func TestVerboDesconocido(t *testing.T) {
	v := &verbosFalsos{m: map[string]string{}}
	texto := "Juan tiene 10 canicas y troca 3. ¿Cuántas le quedan?"
	pls, err := Plantear(&nucleo.Pregunta{Texto: texto}, v)
	if err != nil || len(pls) == 0 || len(pls[0].Desconocido) != 1 || pls[0].Desconocido[0] != "trocar" {
		t.Fatalf("debería no conocer «trocar»: %+v %v", pls, err)
	}
	var mu sync.Mutex
	var preguntas []nucleo.PreguntaUsuario
	tr := nucleo.NuevaTraza(func(e nucleo.Evento) {
		if e.Pregunta != nil {
			mu.Lock()
			preguntas = append(preguntas, *e.Pregunta)
			mu.Unlock()
		}
	}, nucleotest.RespuestasFijas("resta"))
	var prob nucleo.Resolutor
	for _, r := range Resolutores(v) {
		if r.Nombre() == "mates.problema" {
			prob = r
		}
	}
	r, err := prob.Resolver(fondo, &nucleo.Pregunta{Texto: texto}, tr.Raiz())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Texto, "7") || r.Nivel != nucleo.NivelCalculo {
		t.Errorf("respuesta: %q", r.Texto)
	}
	if len(preguntas) != 1 || preguntas[0].Texto != "¿«trocar» suma o resta?" || strings.Join(preguntas[0].Opciones, "|") != "suma|resta|no cambia nada" {
		t.Errorf("pregunta: %+v", preguntas)
	}
	if e, ok := v.Efecto("trocar"); !ok || e != "restar" {
		t.Errorf("no se aprendió el verbo: %q %v", e, ok)
	}
	// learned: no question the second time
	tr2 := nucleo.NuevaTraza(nil, nil)
	r, err = prob.Resolver(fondo, &nucleo.Pregunta{Texto: "Eva tiene 9 cromos y troca 4. ¿Cuántos le quedan?"}, tr2.Raiz())
	if err != nil || !strings.Contains(r.Texto, "5") {
		t.Errorf("segunda vez: %q %v", r.Texto, err)
	}
	// nobody answers: an honest error
	if _, err := Resolutores(nil)[3].Resolver(fondo, &nucleo.Pregunta{Texto: texto}, nucleo.NuevaTraza(nil, nil).Raiz()); !errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Errorf("sin respuesta: %v", err)
	}
}

func resolutor(nombre string) nucleo.Resolutor {
	for _, r := range Resolutores(nil) {
		if r.Nombre() == nombre {
			return r
		}
	}
	return nil
}

// TestResolutores runs the golden-exam questions of this package through the best resolver.
func TestResolutores(t *testing.T) {
	casos := []struct {
		texto, resolutor string
		contiene         []string
	}{
		{"resuelve 2x+3=11", "mates.ecuacion", []string{"x = 4", "Compruebo"}},
		{"x^2-5x+6=0", "mates.ecuacion", []string{"2", "3"}},
		{"x+y=10, x-y=2", "mates.ecuacion", []string{"x = 6", "y = 4"}},
		{"cuánto es 3/4+1/6", "mates.calculo", []string{"11/12"}},
		{"factoriza 600851475143", "mates.numeros", []string{"6857"}},
		{"¿es primo 97?", "mates.numeros", []string{"Sí"}},
		{"mcd de 48 y 18", "mates.numeros", []string{"6"}},
		{"el 15% de 240", "mates.numeros", []string{"36"}},
		{"el doble de un número más 3 es 11", "mates.problema", []string{"4", "2·x"}},
		{"María tiene 12 caramelos y regala 5. ¿Cuántos le quedan?", "mates.problema", []string{"7"}},
		{"un tren va a 80 km/h durante 2 horas y media, ¿qué distancia recorre?", "mates.problema", []string{"200"}},
		{"Ana tiene el triple de la edad de Luis y entre los dos suman 48", "mates.problema", []string{"36", "12"}},
		{"¿es primo 2^61-1?", "mates.numeros", []string{"Sí"}},
		{"¿es primo 91?", "mates.numeros", []string{"No", "7 · 13"}},
		{"2x-1 > 5", "mates.ecuacion", []string{"x > 3"}},
		{"100!", "mates.calculo", []string{"158 cifras"}},
		{"¿cuál es el mcm de 4 y 6?", "mates.numeros", []string{"12"}},
		{"divisores de 36", "mates.numeros", []string{"1, 2, 3, 4, 6, 9, 12, 18, 36"}},
		{"¿qué porcentaje es 30 de 120?", "mates.numeros", []string{"25 %"}},
		{"rebaja un 20% a 50", "mates.numeros", []string{"40"}},
		{"primos hasta 30", "mates.numeros", []string{"29"}},
		{"raíz cuadrada de 50", "mates.calculo", []string{"5√2"}},
		{"resuelve 2x+3=13", "mates.ecuacion", []string{"x = 5"}},
	}
	rs := Resolutores(nil)
	for _, c := range casos {
		p := &nucleo.Pregunta{Texto: c.texto}
		var mejor nucleo.Resolutor
		var puntos float64
		for _, r := range rs {
			inicio := time.Now()
			rc := r.Reconoce(p)
			if d := time.Since(inicio); d > 5*time.Millisecond {
				t.Errorf("%s.Reconoce(%q) tardó %v", r.Nombre(), c.texto, d)
			}
			if rc.Puntos > puntos {
				mejor, puntos = r, rc.Puntos
			}
		}
		if mejor == nil || mejor.Nombre() != c.resolutor {
			t.Errorf("%q: lo reconoce %v, esperaba %s", c.texto, mejor, c.resolutor)
			continue
		}
		resp, err := mejor.Resolver(fondo, p, nucleo.NuevaTraza(nil, nil).Raiz())
		if err != nil {
			t.Errorf("%q: %v", c.texto, err)
			continue
		}
		todo := resp.Texto + "\n" + strings.Join(resp.Parrafos, "\n")
		for _, s := range c.contiene {
			if !strings.Contains(todo, s) {
				t.Errorf("%q: la respuesta no contiene %q:\n%s", c.texto, s, todo)
			}
		}
		if resp.Nivel != nucleo.NivelCalculo || !resp.Exito || len(resp.Comprobaciones) == 0 {
			t.Errorf("%q: nivel %s, comprobaciones %v", c.texto, resp.Nivel, resp.Comprobaciones)
		}
	}
	// not mine
	for _, s := range []string{"hola", "si llueve el suelo se moja; llueve. ¿se moja?", "haz una función que sume los números pares de una lista",
		"2, 5, 10, 17, ¿siguiente?", "SEND+MORE=MONEY", "¿de cuántas formas elijo 3 de 10?"} {
		for _, r := range rs {
			if rc := r.Reconoce(&nucleo.Pregunta{Texto: s}); rc.Puntos >= 0.5 {
				t.Errorf("%s reconoce %q con %.2f", r.Nombre(), s, rc.Puntos)
			}
		}
	}
}

func TestGalimatias(t *testing.T) {
	for _, r := range Resolutores(nil) {
		_, err := r.Resolver(fondo, &nucleo.Pregunta{Texto: "flurb glorp zzzt"}, nil)
		if !errors.Is(err, nucleo.ErrNoEntiendo) {
			t.Errorf("%s con galimatías: %v", r.Nombre(), err)
		}
	}
	if r := resolutor("mates.calculo"); r == nil {
		t.Fatal("falta mates.calculo")
	}
}

func TestPartirEcuaciones(t *testing.T) {
	casos := map[string]string{
		"x+y=10, x-y=2":    "x+y=10|x-y=2",
		"x+y=10; x-y=2":    "x+y=10|x-y=2",
		"x+y=10 y x-y=2":   "x+y=10|x-y=2",
		"x = 3,5":          "x = 3,5",
		"resuelve 2x+3=11": "resuelve 2x+3=11",
	}
	for s, want := range casos {
		if got := strings.Join(partirEcuaciones(s), "|"); got != want {
			t.Errorf("partirEcuaciones(%q) = %q", s, got)
		}
	}
	if got := strings.Join(extraerEcuaciones("resuelve el sistema: x+y=10, x-y=2"), "|"); got != "x+y=10|x-y=2" {
		t.Errorf("extraerEcuaciones: %q", got)
	}
}

func TestEntradasRaras(t *testing.T) {
	raras := []string{"", "   ", "=", "==", "x=", "=x", "()", ")(", "2^", "√", "!", "%", "x/0 = 1", "1/(x-x) = 2",
		"x^100 = 1", "x^41 = 2", "x^2^2^2 = 16", "(x+1)^-1 = 2", "2^(1/3) = x", "x = x", "x = x + 1", "0 = 0",
		"a+b+c+d+e+f+g = 1", "¿?", "😀", "1,2,3", "x < y", "x < 2 < 3", "2^(2^30)", "10^10^10", "0^-1", "(-8)^(1/3)",
		"(-4)^(1/2)", "1e5", ".5 + .5", "x^2 = -0", "sen(x) = 1", strings.Repeat("(", 500) + "1" + strings.Repeat(")", 500),
		strings.Repeat("1+", 3000) + "1", "María tiene", "¿Cuántos le quedan?", "tiene 5 y da 7. ¿Cuántos quedan?"}
	rs := Resolutores(nil)
	for _, s := range raras {
		ctx, cancel := context.WithTimeout(fondo, 2*time.Second)
		_, _ = Calcular(ctx, s, nil)
		_, _ = Ecuaciones(ctx, []string{s}, nil)
		_, _ = Inecuacion(ctx, s, nil)
		_, _ = Plantear(&nucleo.Pregunta{Texto: s}, nil)
		for _, r := range rs {
			p := &nucleo.Pregunta{Texto: s}
			if r.Reconoce(p).Puntos > 0 {
				_, _ = r.Resolver(ctx, p, nil)
			}
		}
		cancel()
	}
}
