package mates

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Tests of the two abilities taken from the Gnostos engine of Nyx: chains of equalities
// (mates.cadena) and resources that run out (mates.recursos).

// mejorDe returns the name of the resolver of this package that recognizes texto best.
func mejorDe(texto string) (string, float64) {
	p := &nucleo.Pregunta{Texto: texto}
	nombre, puntos := "", 0.0
	for _, r := range Resolutores(nil) {
		if rc := r.Reconoce(p); rc.Puntos > puntos {
			nombre, puntos = r.Nombre(), rc.Puntos
		}
	}
	return nombre, puntos
}

func resolverCon(t *testing.T, nombre, texto string) (nucleo.Respuesta, error) {
	t.Helper()
	r := resolutor(nombre)
	if r == nil {
		t.Fatalf("no hay resolutor %s", nombre)
	}
	ctx, cancel := context.WithTimeout(fondo, 5*time.Second)
	defer cancel()
	return r.Resolver(ctx, &nucleo.Pregunta{Texto: texto}, trazaPrueba())
}

func todoTexto(r nucleo.Respuesta) string {
	partes := []string{r.Texto, r.Entendi}
	partes = append(partes, r.Parrafos...)
	partes = append(partes, r.Plan...)
	return strings.Join(partes, "\n")
}

// ---- chains of equalities ----

func TestCadenaEjemplos(t *testing.T) {
	casos := []struct {
		texto   string
		quiere  []string
		noQuier string
	}{
		{"x=g=t=u=e=s=q=l; x=1 ¿cuánto vale l?", []string{"l = 1", "La cadena: l = q = s = e = u = t = g = x = 1", "g = x (lo dice «x=g=t=u=e=s=q=l»), así que g = 1"}, ""},
		{"si x = g, g = t y t = 5, ¿cuánto vale x?", []string{"x = 5", "x = g = t = 5"}, ""},
		{"x=g=t=u=e=s=q=l. ¿es x igual a l?", []string{"Sí: x = l", "x = g = t = u = e = s = q = l"}, ""},
		{"x = g, a = b. ¿es x igual a b?", []string{"No se puede saber", "ninguna igualdad une x con b"}, ""},
		{"x = g, g = 3, a = b, b = 4. ¿es x igual a a?", []string{"No: x = 3 y a = 4"}, ""},
		{"x = 1, l = 1, x = g. ¿es g igual a l?", []string{"Sí: g = l", "g = x = 1 = l"}, ""},
		{"a = b, b = c, a = 1, c = 2. ¿cuánto vale b?", []string{"b no tiene un valor único", "se contradicen", "1 = a = b = c = 2"}, "b = 1"},
		{"x=g=t=u=e=s=q=l; x=1; l=2 ¿cuánto vale t?", []string{"no tiene un valor único", "1 tendría que ser igual a 2"}, ""},
		{"Ana es igual a Bea y Bea es igual a 7. ¿Cuánto vale Ana?", []string{"Ana = 7"}, ""},
		{"x, g y t son iguales; t vale 3/4. ¿cuánto vale x?", []string{"x = 3/4", "≈ 0,75"}, ""},
		{"p = q = r; r = -2,5 ¿cuánto valen p y q?", []string{"p = -5/2", "q = -5/2"}, ""},
		{"x = g = h. ¿cuánto vale x?", []string{"No se puede saber cuánto vale x", "x = g = h"}, ""},
		{"x = g = h; ¿cuánto vale z?", []string{"No sé nada de z"}, ""},
		{"x = g = t; g = 2", []string{"x = g = t = 2"}, ""},
		{"x = g; x = 1; a = b; a = 3; b = 2. ¿cuánto vale g?", []string{"g = 1", "Ojo, además: los datos se contradicen: 3 = a = b = 2"}, ""},
		// x = 1 and a = 1 join both groups through the number 1, so the contradiction reaches g
		{"x = g; x = 1; a = b; a = 1; b = 2. ¿cuánto vale g?", []string{"g no tiene un valor único", "1 = a = b = 2"}, "g = 1"},
	}
	for _, c := range casos {
		if m, _ := mejorDe(c.texto); m != "mates.cadena" {
			t.Errorf("%q: lo reconoce %q", c.texto, m)
			continue
		}
		r, err := resolverCon(t, "mates.cadena", c.texto)
		if err != nil {
			t.Errorf("%q: %v", c.texto, err)
			continue
		}
		todo := todoTexto(r)
		for _, q := range c.quiere {
			if !strings.Contains(todo, q) {
				t.Errorf("%q: falta %q en:\n%s", c.texto, q, todo)
			}
		}
		if c.noQuier != "" && strings.Contains(r.Texto, c.noQuier) {
			t.Errorf("%q: no debía decir %q: %s", c.texto, c.noQuier, r.Texto)
		}
		if r.Nivel != nucleo.NivelCalculo || !r.Exito || len(r.Comprobaciones) != 1 || r.Comprobaciones[0].Pasan != r.Comprobaciones[0].Total {
			t.Errorf("%q: nivel %s éxito %v comprobaciones %+v", c.texto, r.Nivel, r.Exito, r.Comprobaciones)
		}
	}
}

// The chain must not take the questions of the other resolvers (the golden exam among them).
func TestCadenaNoRoba(t *testing.T) {
	ajenas := []string{"resuelve 2x+3=11", "x^2-5x+6=0", "x+y=10, x-y=2", "cuánto es 3/4+1/6", "SEND+MORE=MONEY",
		"si llueve el suelo se moja; llueve. ¿se moja?", "p -> q; p; ¿q?", "María tiene 12 caramelos y regala 5. ¿Cuántos le quedan?",
		"x := 5", "a <= b", "a != b", "x = 2y", "x = g + 1", "f(x) = 3", "2x = g", "x = 5", "x = x", "hola", "0 = 0",
		"x == y && y == z", "el 15% de 240", "Toby es un perro", "¿es primo 97?"}
	cad := resolutor("mates.cadena")
	for _, s := range ajenas {
		if rc := cad.Reconoce(&nucleo.Pregunta{Texto: s}); rc.Puntos > 0 {
			t.Errorf("mates.cadena reconoce %q con %.2f", s, rc.Puntos)
		}
	}
	// a plain system that mates.ecuacion reads stays with mates.ecuacion
	for _, s := range []string{"x = g, g = t, t = 5", "a = b"} {
		if m, _ := mejorDe(s); m != "mates.ecuacion" {
			t.Errorf("%q: lo reconoce %q y no mates.ecuacion", s, m)
		}
	}
	if rc := cad.Reconoce(&nucleo.Pregunta{Texto: "x = g", Codigo: "x = g"}); rc.Puntos > 0 {
		t.Errorf("con código no es una cadena")
	}
}

// The two methods (union-find and breadth-first search) agree on random graphs, and every path shown
// is made of stated equalities.
func TestCadenaDosMetodosAlAzar(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for caso := 0; caso < 200; caso++ {
		n := 2 + rng.Intn(30)
		var partes []string
		for k := 0; k < rng.Intn(40); k++ {
			partes = append(partes, fmt.Sprintf("v%d = v%d", rng.Intn(n), rng.Intn(n)))
		}
		for k := 0; k < rng.Intn(4); k++ {
			partes = append(partes, fmt.Sprintf("v%d = %d", rng.Intn(n), rng.Intn(3)))
		}
		if len(partes) == 0 {
			continue
		}
		d := leerCadenas(strings.Join(partes, "; "))
		if d.impuro || d.demasiados {
			t.Fatalf("%q: impuro=%v", strings.Join(partes, "; "), d.impuro)
		}
		raiz := gruposUnion(len(d.claves), d.enlaces)
		ady := adyacencia(len(d.claves), d.enlaces)
		comp := componentes(ady)
		for a := range d.claves {
			for b := range d.claves {
				if (raiz[a] == raiz[b]) != (comp[a] == comp[b]) {
					t.Fatalf("los métodos no coinciden en %s, %s", d.claves[a], d.claves[b])
				}
				if raiz[a] == raiz[b] {
					nodos, fuentes := camino(ady, a, b)
					if !caminoValido(d, nodos, fuentes) || nodos[0] != a || nodos[len(nodos)-1] != b {
						t.Fatalf("camino no válido de %s a %s: %v", d.claves[a], d.claves[b], nodos)
					}
				}
			}
		}
	}
}

func TestCadenaPatologica(t *testing.T) {
	// a long chain is answered quickly, with the steps shortened
	var nombres []string
	for i := 0; i < 600; i++ {
		nombres = append(nombres, fmt.Sprintf("n%d", i))
	}
	texto := strings.Join(nombres, "=") + "; n0 = 4 ¿cuánto vale n599?"
	if len(texto) <= maxTextoCadena {
		ini := time.Now()
		r, err := resolverCon(t, "mates.cadena", texto)
		if err != nil || !strings.Contains(r.Texto, "n599 = 4") {
			t.Fatalf("cadena larga: %v %q", err, r.Texto)
		}
		if len(r.Plan) > maxPasosCadena+5 || time.Since(ini) > time.Second {
			t.Fatalf("cadena larga: %d pasos, %v", len(r.Plan), time.Since(ini))
		}
	}
	raras := []string{"", "=", "==", "= = =", "x =", "= x", strings.Repeat("x=", 3000) + "x", strings.Repeat("=", 5000),
		strings.Repeat("a = b; ", 600), "¿?", "¿x = ?", "x = 99999999999999999999999999999999999999999999999999999999999999999999",
		"x = 3/0", "x = -", "x = g ¿es x igual a", "😀 = 😀", "x = g\x00 ¿cuánto vale x?", "x=g=" + strings.Repeat("(", 200)}
	cad := resolutor("mates.cadena")
	for _, s := range raras {
		sinPanico(t, s[:min(len(s), 30)], func() {
			ini := time.Now()
			p := &nucleo.Pregunta{Texto: s}
			if cad.Reconoce(p).Puntos > 0 {
				ctx, cancel := context.WithTimeout(fondo, time.Second)
				_, _ = cad.Resolver(ctx, p, nil)
				cancel()
			}
			if d := time.Since(ini); d > 500*time.Millisecond {
				t.Errorf("%.30q tardó %v", s, d)
			}
		})
	}
}

// ---- resources that run out ----

func TestRecursosEjemplos(t *testing.T) {
	casos := []struct {
		texto  string
		quiere []string
		metodo string
	}{
		{"tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora; ¿cuánto dura?",
			[]string{"Dura 30 horas: lo primero que se acaba es el aceite", "quedan 40 metros de mecha", "90 litros ÷ 3 litros por hora = 30 horas"}, "exacto"},
		{"tengo 90 litros de aceite y gasto 3 litros por hora. ¿cuándo se acaba?", []string{"Se acaba al cabo de 30 horas"}, "exacto"},
		{"hay 40 litros de agua y se gastan 2 por día; ¿cuánto dura?", []string{"Dura 20 días"}, "exacto"},
		{"tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora, y a las 10 horas añado 50 de mecha; ¿qué se acaba primero?",
			[]string{"Se acaba primero el aceite, al cabo de 30 horas", "quedan 90 metros de mecha", "a las 10 horas quedan 80 y añades 50 → 130 metros", "75 horas"}, "exacto"},
		{"tengo 30 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora; a las 5 horas añado 90 litros de aceite. ¿qué se acaba primero?",
			[]string{"Se acaba primero el aceite, al cabo de 40 horas", "a las 5 horas quedan 15 y añades 90"}, "exacto"},
		{"tengo 30 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora; a las 5 horas añado 150 litros de aceite. ¿qué se acaba primero?",
			[]string{"Se acaba primero la mecha, al cabo de 50 horas", "quedan 30 litros de aceite"}, "exacto"},
		{"tengo 30 litros de aceite y gasto 3 por hora; a las 10 horas añado 90 litros de aceite. ¿cuánto dura?",
			[]string{"Dura 10 horas", "justo cuando ibas a añadir más"}, "exacto"},
		{"tengo 90 litros de aceite y gasto 3 por hora sin bajar de 10; ¿cuánto dura?", []string{"Dura 80/3 (≈ 26,6667) horas", "(90 litros − 10)"}, "exacto"},
		{"tengo 90 litros de aceite y 100 metros de mecha; gasto 3 litros de aceite y 2 metros de mecha por hora. ¿Cuánto dura sin bajar de 10 metros de mecha?",
			[]string{"Dura 30 horas", "el aceite"}, "exacto"},
		{"tengo 60 litros de aceite y gasto 2 por hora y 100 metros de mecha y gasto 2 por hora sin bajar de 40 metros. ¿qué se acaba primero?",
			[]string{"Se acaban a la vez el aceite y la mecha, al cabo de 30 horas"}, "exacto"},
		{"tengo 2 litros de agua y bebo 250 ml por hora. ¿cuánto dura?", []string{"Dura 8 horas"}, "exacto"},
		{"tengo 3 litros de agua y gasto 1 litro cada 2 horas. ¿cuánto dura?", []string{"Dura 6 horas"}, "exacto"},
		{"tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 1 por minuto; ¿cuánto dura?",
			[]string{"Dura 5/3 (≈ 1,6667) horas", "la mecha"}, "exacto"},
		{"Juan tiene 20 euros y gasta 3 euros por día. ¿Cuántos euros le quedan después de 4 días?", []string{"Al cabo de 4 días quedan 8 euros"}, "exacto"},
		{"tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora; ¿cuánto dura la mecha?",
			[]string{"La mecha dura 50 horas", "Pero antes, al cabo de 30 horas, se acaba el aceite"}, "exacto"},
		{"tengo noventa litros de aceite y gasto tres por hora; ¿cuánto dura?", []string{"Dura 30 horas"}, "exacto"},
		{"tengo 10 litros de aceite y gasto 0 por hora; ¿cuánto dura?", []string{"No se acaba nunca"}, "exacto"},
		{"tengo 5 litros de aceite y gasto 1 por hora sin bajar de 10; ¿cuánto dura?", []string{"No dura nada"}, "exacto"},
		{"tengo 90 litros de aceite y gasto 3*exp(-0.01*t) por hora; ¿cuánto dura?", []string{"Dura ≈ 35,6675 horas (aproximado)", "error estimado"}, "aproximado"},
		{"tengo 90 litros de aceite y gasto 3*exp(-0.1*t) por hora; ¿cuánto dura?", []string{"no se acaba"}, "aproximado"},
	}
	for _, c := range casos {
		if m, _ := mejorDe(c.texto); m != "mates.recursos" {
			t.Errorf("%q: lo reconoce %q", c.texto, m)
			continue
		}
		r, err := resolverCon(t, "mates.recursos", c.texto)
		if err != nil {
			t.Errorf("%q: %v", c.texto, err)
			continue
		}
		todo := todoTexto(r)
		for _, q := range c.quiere {
			if !strings.Contains(todo, q) {
				t.Errorf("%q: falta %q en:\n%s", c.texto, q, todo)
			}
		}
		if r.Metodo != c.metodo || r.Nivel != nucleo.NivelCalculo || !r.Exito || len(r.Comprobaciones) == 0 {
			t.Errorf("%q: método %s nivel %s éxito %v", c.texto, r.Metodo, r.Nivel, r.Exito)
		}
	}
}

// For constant rates the numeric integration must give the exact instant (the cross-check the
// resolver itself runs), with refills and minimums too.
func TestRecursosNumericoIgualExacto(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for caso := 0; caso < 300; caso++ {
		u := unidadesTiempo["hora"]
		var rs []*recursoR
		for k := 0; k < 1+rng.Intn(4); k++ {
			r := &recursoR{cosa: fmt.Sprintf("r%d", k), stock: big.NewRat(int64(1+rng.Intn(500)), int64(1+rng.Intn(4))),
				tasa: big.NewRat(int64(1+rng.Intn(20)), int64(1+rng.Intn(7)))}
			if rng.Intn(3) == 0 {
				r.minimo = big.NewRat(int64(rng.Intn(50)), 1)
			}
			for j := 0; j < rng.Intn(3); j++ {
				r.recargas = append(r.recargas, recargaR{t: big.NewRat(int64(rng.Intn(200)), int64(1+rng.Intn(3))), cantidad: big.NewRat(int64(1+rng.Intn(100)), 1)})
			}
			ordenarRecargas(r)
			rs = append(rs, r)
		}
		var tFin *big.Rat
		for _, r := range rs {
			if a := agotarExacto(r, u); a.t != nil && (tFin == nil || a.t.Cmp(tFin) < 0) {
				tFin = a.t
			}
		}
		pr := &problemaRecursos{recursos: rs, base: u}
		var primeros []*recursoR
		for _, r := range rs {
			if a := agotarExacto(r, u); a.t != nil && a.t.Cmp(tFin) == 0 {
				primeros = append(primeros, r)
			}
		}
		if !comprobarExacto(pr, tFin, primeros) {
			t.Fatalf("caso %d: la sustitución no cuadra con t* = %s", caso, tFin.RatString())
		}
		res, err := integrar(fondo, rs, horizonteDe(rs), 1e-9*escalaNiveles(rs))
		if err != nil {
			t.Fatalf("caso %d: %v", caso, err)
		}
		tf, _ := tFin.Float64()
		if res.nunca || math.Abs(res.t-tf) > 1e-7*math.Max(1, tf) {
			t.Fatalf("caso %d: numérico %v, exacto %v", caso, res.t, tf)
		}
	}
}

func ordenarRecargas(r *recursoR) {
	for i := 1; i < len(r.recargas); i++ {
		for j := i; j > 0 && r.recargas[j].t.Cmp(r.recargas[j-1].t) < 0; j-- {
			r.recargas[j], r.recargas[j-1] = r.recargas[j-1], r.recargas[j]
		}
	}
}

// The adaptive RK4 against analytic solutions of rates that depend on t.
func TestRecursosRK4Analitico(t *testing.T) {
	casos := []struct {
		texto     string
		analitico float64
	}{
		{"tengo 90 litros de aceite y gasto 3*exp(-0.01*t) por hora; ¿cuánto dura?", -100 * math.Log(0.7)},
		{"tengo 100 litros de agua y gasto 0.5*t por hora; ¿cuánto dura?", 20},
		{"tengo 1 litro de agua y gasto sen(t) por hora; ¿cuánto dura?", math.Pi / 2},
		{"tengo 50 kilos de grano y gasto 2+t/10 por día; ¿cuánto dura?", -20 + math.Sqrt(400+1000)},
	}
	for _, c := range casos {
		pr, err := leerRecursos(c.texto)
		if err != nil {
			t.Fatalf("%q: %v", c.texto, err)
		}
		if !pr.numerico {
			t.Fatalf("%q: debería ser numérico", c.texto)
		}
		res, err := integrar(fondo, pr.recursos, horizonteDe(pr.recursos), 1e-10)
		if err != nil || res.nunca || math.Abs(res.t-c.analitico) > 1e-5 {
			t.Errorf("%q: t* = %v (%v), analítico %v", c.texto, res.t, err, c.analitico)
		}
		r, err := resolverCon(t, "mates.recursos", c.texto)
		if err != nil || r.Metodo != "aproximado" || !strings.Contains(r.Texto, "aproximado") || !strings.Contains(r.Texto, textoFloat(c.analitico)) {
			t.Errorf("%q: %v %q (quería ≈ %s)", c.texto, err, r.Texto, textoFloat(c.analitico))
		}
	}
}

func TestRecursosLectura(t *testing.T) {
	pr, err := leerRecursos("tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora, y a las 10 horas añado 50 de mecha; ¿cuánto dura?")
	if err != nil {
		t.Fatal(err)
	}
	if len(pr.recursos) != 2 || pr.pregunta != "dura" || pr.base.uno != "hora" {
		t.Fatalf("lectura: %+v", pr)
	}
	a, m := pr.recursos[0], pr.recursos[1]
	if a.cosa != "aceite" || a.stock.Cmp(big.NewRat(90, 1)) != 0 || a.tasa.Cmp(big.NewRat(3, 1)) != 0 || len(a.recargas) != 0 {
		t.Errorf("aceite: %+v", a)
	}
	if m.cosa != "mecha" || m.stock.Cmp(big.NewRat(100, 1)) != 0 || m.tasa.Cmp(big.NewRat(2, 1)) != 0 || len(m.recargas) != 1 ||
		m.recargas[0].t.Cmp(big.NewRat(10, 1)) != 0 || m.recargas[0].cantidad.Cmp(big.NewRat(50, 1)) != 0 {
		t.Errorf("mecha: %+v", m)
	}
	// honest errors
	for texto, quiere := range map[string]error{
		"tengo 90 litros de aceite y gasto 3 litros; ¿cuánto dura?":                                  nucleo.ErrNoEntiendo,
		"tengo 90 litros de aceite y gasto 3 por hora y cada 10 horas añado 5 litros; ¿cuánto dura?": nucleo.ErrNoSoportado,
		"gasto 3 litros de aceite por hora; ¿cuánto dura?":                                           nucleo.ErrNoEntiendo,
		"tengo 90 litros de aceite y gasto 3*x por hora; ¿cuánto dura?":                              nucleo.ErrNoEntiendo,
		"tengo 90 litros de aceite y gasto 3 por hora y añado 5 litros; ¿cuánto dura?":               nucleo.ErrNoEntiendo,
	} {
		if _, err := leerRecursos(texto); !errors.Is(err, quiere) {
			t.Errorf("%q: %v, quería %v", texto, err, quiere)
		}
	}
}

// mates.problema leaves the resources to mates.recursos, and mates.recursos takes nothing else.
func TestRecursosNoRoba(t *testing.T) {
	for _, s := range []string{"María tiene 12 caramelos y regala 5. ¿Cuántos le quedan?",
		"un tren va a 80 km/h durante 2 horas y media, ¿qué distancia recorre?", "el doble de un número más 3 es 11",
		"Ana tiene el triple de la edad de Luis y entre los dos suman 48", "resuelve 2x+3=11", "hola",
		"un coche gasta 6 litros cada 100 km, ¿cuánto gasta en 300 km?", "x=g=t; x=1 ¿cuánto vale t?"} {
		if rc := (resRecursos{}).Reconoce(&nucleo.Pregunta{Texto: s}); rc.Puntos > 0 {
			t.Errorf("mates.recursos reconoce %q con %.2f", s, rc.Puntos)
		}
	}
	texto := "tengo 90 litros de aceite y gasto 3 por hora; ¿cuánto dura?"
	if rc := resolutor("mates.problema").Reconoce(&nucleo.Pregunta{Texto: texto}); rc.Puntos > 0 {
		t.Errorf("mates.problema no debe tomar %q", texto)
	}
	if _, err := resolutor("mates.problema").Resolver(fondo, &nucleo.Pregunta{Texto: texto}, nil); !errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Errorf("mates.problema debía rechazar los recursos: %v", err)
	}
}

func TestRecursosPatologicos(t *testing.T) {
	raras := []string{
		"tengo 90 litros de aceite y gasto 1/(t-5) por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto exp(t) por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto exp(1000*t) por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto -3 por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto 3/0 por hora; ¿cuánto dura?",
		"tengo 10^400 litros de aceite y gasto 3 por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto 10^-300 por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto 0.000000001*t por segundo; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto sen(t)^2 por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto 3 por hora " + strings.Repeat("y a las 1 horas añado 1 litro de aceite ", 120) + "; ¿cuánto dura?",
		strings.Repeat("tengo 90 litros de aceite y gasto 3 por hora; ", 100) + "¿cuánto dura?",
		strings.Repeat("gasto ", 600) + "por hora",
		"gasto por hora ¿cuánto dura?", "tengo gasto hora dura", "tengo 1 hora y gasto 2 por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto (((((((((((t))))))))))) por hora; ¿cuánto dura?",
		"tengo 90 litros de aceite y gasto 3 por hora sin bajar de 100000; ¿cuánto queda a las 5 horas?",
	}
	r := resolutor("mates.recursos")
	for _, s := range raras {
		sinPanico(t, s[:min(len(s), 50)], func() {
			ini := time.Now()
			ctx, cancel := context.WithTimeout(fondo, 3*time.Second)
			defer cancel()
			p := &nucleo.Pregunta{Texto: s}
			r.Reconoce(p)
			resp, err := r.Resolver(ctx, p, trazaPrueba())
			if err == nil && (strings.Contains(resp.Texto, "NaN") || strings.Contains(resp.Texto, "Inf")) {
				t.Errorf("%.50q: %q", s, resp.Texto)
			}
			if d := time.Since(ini); d > 3500*time.Millisecond {
				t.Errorf("%.50q tardó %v", s, d)
			}
		})
	}
	// Reconoce stays cheap on the longest accepted text
	texto := strings.Repeat("tengo 9 litros de aceite y gasto 3 por hora; ", 100)[:maxTextoRecursos]
	ini := time.Now()
	for i := 0; i < 20; i++ {
		r.Reconoce(&nucleo.Pregunta{Texto: texto})
		resolutor("mates.cadena").Reconoce(&nucleo.Pregunta{Texto: texto})
	}
	if d := time.Since(ini) / 40; d > 5*time.Millisecond {
		t.Errorf("Reconoce tardó %v", d)
	}
}

// A rate with a singularity cannot be integrated honestly: the two grids disagree and nothing is
// claimed.
func TestRecursosSingularidad(t *testing.T) {
	r, err := resolverCon(t, "mates.recursos", "tengo 90 litros de aceite y gasto 1/(t-5) por hora; ¿cuánto dura?")
	if err == nil {
		t.Fatalf("una tasa con una singularidad no debía dar respuesta: %q", r.Texto)
	}
	r, err = resolverCon(t, "mates.recursos", "tengo 10^400 litros de aceite y gasto 3 por hora; ¿cuánto dura?")
	if err != nil || !strings.Contains(r.Texto, "3,33333·10^399") || r.Comprobaciones[0].Texto != "Comprobado sustituyendo" {
		t.Fatalf("números enormes: %v %q %+v", err, r.Texto, r.Comprobaciones)
	}
}
