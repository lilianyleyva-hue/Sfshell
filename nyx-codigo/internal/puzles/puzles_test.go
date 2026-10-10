package puzles

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

const sudokuInkala = `8........
..36.....
.7..9.2..
.5...7...
....457..
...1...3.
..1....68
..85...1.
.9....4..`

const sudokuFacil = `53..7....
6..195...
.98....6.
8...6...3
4..8.3..1
7...2...6
.6....28.
...419..5
....8..79`

func TestSudokuInkala(t *testing.T) {
	tab, err := LeerSudoku(sudokuInkala)
	if err != nil {
		t.Fatal(err)
	}
	ini := time.Now()
	sol, unica, est, err := Sudoku(context.Background(), tab, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(ini); d > time.Second {
		t.Fatalf("tardó %v", d)
	}
	if !unica {
		t.Fatal("la solución debería ser única")
	}
	if err := ValidarSudoku(sol, true); err != nil {
		t.Fatal(err)
	}
	if got := sol[0]; got != [9]int{8, 1, 2, 7, 5, 3, 6, 4, 9} {
		t.Fatalf("primera fila = %v", got)
	}
	if est.Nodos == 0 {
		t.Fatal("el sudoku difícil debería necesitar búsqueda")
	}
}

func TestSudokuFacilSoloTecnicas(t *testing.T) {
	tab, err := LeerSudoku(sudokuFacil)
	if err != nil {
		t.Fatal(err)
	}
	tr := nucleo.NuevaTraza(nil, nil)
	sol, unica, est, det, err := SudokuDetallado(context.Background(), tab, tr.Raiz())
	if err != nil {
		t.Fatal(err)
	}
	if !det.SoloTecnicas || est.Retrocesos != 0 || !unica {
		t.Fatalf("solo técnicas=%v retrocesos=%d única=%v", det.SoloTecnicas, est.Retrocesos, unica)
	}
	if err := ValidarSudoku(sol, true); err != nil {
		t.Fatal(err)
	}
	encontrado := false
	for _, p := range tr.Pasos() {
		if strings.Contains(p.Titulo, "solo puede ser") || strings.Contains(p.Titulo, "solo cabe") {
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatal("la traza debería explicar las técnicas en español")
	}
}

func TestLeerSudoku(t *testing.T) {
	una := strings.ReplaceAll(sudokuFacil, "\n", "")
	if _, err := LeerSudoku("resuelve: " + una); err != nil {
		t.Fatalf("81 seguidas: %v", err)
	}
	conMarcos := "5 3 . | . 7 . | . . .\n6 . . | 1 9 5 | . . .\n. 9 8 | . . . | . 6 .\n------+-------+------\n8 . . | . 6 . | . . 3\n4 . . | 8 . 3 | . . 1\n7 . . | . 2 . | . . 6\n------+-------+------\n. 6 . | . . . | 2 8 .\n. . . | 4 1 9 | . . 5\n. . . | . 8 . | . 7 9"
	if _, err := LeerSudoku(conMarcos); err != nil {
		t.Fatalf("con marcos: %v", err)
	}
	if _, err := LeerSudoku("hola"); err == nil {
		t.Fatal("texto sin sudoku debe fallar")
	}
	malo := strings.Replace(sudokuFacil, "53..7....", "55..7....", 1)
	tab, _ := LeerSudoku(malo)
	if _, _, _, err := Sudoku(context.Background(), tab, nil); err == nil {
		t.Fatal("un sudoku con repetidos debe fallar")
	}
}

func TestReinas(t *testing.T) {
	c, una, err := Reinas(context.Background(), 8, true)
	if err != nil || c != 92 || ComprobarReinas(una) != nil {
		t.Fatalf("8 reinas: %d %v %v", c, una, err)
	}
	ini := time.Now()
	_, una, err = Reinas(context.Background(), 20, false)
	if err != nil || len(una) != 20 || ComprobarReinas(una) != nil {
		t.Fatalf("20 reinas: %v %v", una, err)
	}
	if d := time.Since(ini); d > time.Second {
		t.Fatalf("20 reinas tardó %v", d)
	}
	_, una, err = Reinas(context.Background(), 60, false)
	if err != nil || ComprobarReinas(una) != nil {
		t.Fatalf("60 reinas (min-conflicts): %v", err)
	}
	if c, _, _ := Reinas(context.Background(), 3, true); c != 0 {
		t.Fatalf("3 reinas: %d", c)
	}
}

func TestCripto(t *testing.T) {
	s, err := Cripto(context.Background(), "SEND+MORE=MONEY")
	if err != nil {
		t.Fatal(err)
	}
	if got := EscribirCripto("SEND+MORE=MONEY", s); got != "9567 + 1085 = 10652" {
		t.Fatalf("SEND+MORE=MONEY → %q", got)
	}
	sols, _, err := CriptoTodas(context.Background(), "SEND + MORE = MONEY")
	if err != nil || len(sols) != 1 {
		t.Fatalf("debería ser única: %d %v", len(sols), err)
	}
	if _, err := Cripto(context.Background(), "A+A=B+C"); err == nil {
		// A+A=B+C has solutions; just check it does not crash
	}
	if _, err := Cripto(context.Background(), "AB+AB=AB"); err == nil {
		t.Fatal("AB+AB=AB no tiene solución")
	}
}

func TestCSPSumaYFunc(t *testing.T) {
	p := &Problema{
		Dom: []Dominio{DominioRango(1, 9), DominioRango(1, 9), DominioRango(1, 9)},
		Restr: []Restriccion{
			{Alcance: []int{0, 1, 2}, Tipo: SumaIgual, K: 6},
			{Alcance: []int{0, 1, 2}, Tipo: Distintos},
			{Alcance: []int{0, 1}, Tipo: Binaria, Rel: func(v []int) bool { return v[0] < v[1] }},
			{Alcance: []int{1, 2}, Tipo: Func, Rel: func(v []int) bool { return v[0] < v[1] }},
		},
	}
	sols, _, err := Resolver(context.Background(), p, 10, nil)
	if err != nil || len(sols) != 1 || sols[0][0] != 1 || sols[0][1] != 2 || sols[0][2] != 3 {
		t.Fatalf("sols = %v, %v", sols, err)
	}
}

func TestPlanes(t *testing.T) {
	ctx := context.Background()
	casos := []struct {
		nombre string
		p      ProblemaPlan
		pasos  int
	}{
		{"jarras 3 y 5 → 4", Jarras([]int{3, 5}, 4), 6},
		{"lobo, cabra y col", Rio(), 7},
		{"misioneros 3/3", Misioneros(3, 3, 2), 11},
	}
	for _, c := range casos {
		plan, err := BFS(ctx, c.p, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if len(plan.Pasos) != c.pasos || !Simular(c.p, plan) {
			t.Fatalf("%s: %d pasos (quería %d), simula=%v: %v", c.nombre, len(plan.Pasos), c.pasos, Simular(c.p, plan), plan.Pasos)
		}
		planA, err := AEstrella(ctx, c.p, 0)
		if err != nil || len(planA.Pasos) != c.pasos || !Simular(c.p, planA) {
			t.Fatalf("%s con A*: %d pasos, %v", c.nombre, len(planA.Pasos), err)
		}
	}
	// a tampered plan must not simulate
	plan, _ := BFS(ctx, Jarras([]int{3, 5}, 4), 0)
	plan.Pasos[0] = "llena la jarra de 3 l"
	if Simular(Jarras([]int{3, 5}, 4), plan) {
		t.Fatal("un plan alterado no debe pasar la simulación")
	}
	if _, err := BFS(ctx, Jarras([]int{2, 4}, 3), 0); !errors.Is(err, ErrSinPlan) {
		t.Fatalf("jarras 2 y 4 → 3 es imposible: %v", err)
	}
}

func TestHanoi(t *testing.T) {
	movs, total := Hanoi(3)
	if len(movs) != 7 || total.Int64() != 7 {
		t.Fatalf("Hanoi(3) = %d, %v", len(movs), total)
	}
	if err := ComprobarHanoi(3, movs); err != nil {
		t.Fatal(err)
	}
	movs, total = Hanoi(64)
	if movs != nil || total.String() != "18446744073709551615" {
		t.Fatalf("Hanoi(64) = %v", total)
	}
}

func TestPuzzle8(t *testing.T) {
	ctx := context.Background()
	// optimal depth checked against BFS, which is exhaustive
	ini := [9]int{8, 1, 3, 4, 0, 2, 7, 6, 5}
	p, ok := Puzzle8(ini)
	if !ok {
		t.Fatal("debería ser resoluble")
	}
	bfs, err := BFS(ctx, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	ida, err := IDAEstrella(ctx, p, 0)
	if err != nil || len(ida.Pasos) != len(bfs.Pasos) || !Simular(p, ida) {
		t.Fatalf("IDA* %d pasos, BFS %d: %v", len(ida.Pasos), len(bfs.Pasos), err)
	}
	// the hardest instances need 31 moves
	dificil, _ := Puzzle8([9]int{8, 6, 7, 2, 5, 4, 3, 0, 1})
	plan, err := AEstrella(ctx, dificil, 0)
	if err != nil || len(plan.Pasos) != 31 || !Simular(dificil, plan) {
		t.Fatalf("8-puzzle difícil: %d pasos, %v", len(plan.Pasos), err)
	}
	if _, ok := Puzzle8([9]int{2, 1, 3, 4, 5, 6, 7, 8, 0}); ok {
		t.Fatal("cambiar dos fichas lo hace irresoluble (paridad)")
	}
}

func TestLaberinto(t *testing.T) {
	lab, err := Laberinto([]string{
		"#########",
		"#S..#...#",
		"#.#.#.#.#",
		"#.#...#E#",
		"#########",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := AEstrella(context.Background(), lab, 0)
	if err != nil || !Simular(lab, plan) {
		t.Fatal(err)
	}
	if len(plan.Pasos) != 12 {
		t.Fatalf("pasos = %d: %v", len(plan.Pasos), plan.Pasos)
	}
}

func conteo(t *testing.T, texto string) (*big.Int, *big.Int, Modelo) {
	t.Helper()
	m, _, err := LeerConteo(&nucleo.Pregunta{Texto: texto})
	if err != nil {
		t.Fatalf("%q: %v", texto, err)
	}
	f, _, ok := Formula(m)
	if !ok {
		f = nil
	}
	c, err := ContarFuerza(context.Background(), m, LimiteFuerza)
	if err != nil {
		t.Fatalf("%q: %v", texto, err)
	}
	return f, c, m
}

func TestConteo(t *testing.T) {
	casos := []struct {
		texto string
		v     int64
	}{
		{"¿cuántos anagramas tiene CASA?", 12},
		{"¿de cuántas formas elijo 3 de 10?", 120},
		{"¿cuántos PIN de 4 dígitos sin repetir hay?", 5040},
		{"¿de cuántas formas puedo repartir 10 caramelos iguales entre 3 niños?", 66},
		{"¿de cuántas formas puedo subir 10 escalones de 1 o 2 en 2?", 89},
		{"¿de cuántas formas se sientan 5 personas en una mesa redonda?", 24},
		{"¿cuántos números del 1 al 1000 son divisibles por 3 o por 5?", 467},
		{"¿cuántos caminos hay en una cuadrícula de 3 por 3?", 20},
		{"¿de cuántas formas se pueden ordenar 5 libros?", 120},
		{"¿cuántos números de 3 cifras hay?", 900},
		{"¿cuántas contraseñas de 3 dígitos tienen al menos un 7?", 271},
		{"¿cuántos números del 1 al 100 son divisibles por 2 y por 3?", 16},
	}
	for _, c := range casos {
		f, n, _ := conteo(t, c.texto)
		if n.Int64() != c.v {
			t.Errorf("%q: recuento %v, quería %d", c.texto, n, c.v)
		}
		if f != nil && f.Cmp(n) != 0 {
			t.Errorf("%q: fórmula %v ≠ recuento %v", c.texto, f, n)
		}
		if f == nil {
			t.Errorf("%q: debería tener fórmula", c.texto)
		}
	}
	// primes only by brute force
	m, _, err := LeerConteo(&nucleo.Pregunta{Texto: "¿cuántos primos hay del 1 al 1000?"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := Formula(m); ok {
		t.Fatal("los primos no tienen fórmula aquí")
	}
	if c, _ := ContarFuerza(context.Background(), m, 0); c.Int64() != 168 {
		t.Fatalf("primos hasta 1000 = %v", c)
	}
}

func TestProbabilidad(t *testing.T) {
	casos := []struct{ texto, p string }{
		{"probabilidad de sacar 2 caras al lanzar 3 monedas", "3/8"},
		{"probabilidad de que la suma sea 7 al tirar dos dados", "1/6"},
		{"probabilidad de sacar al menos un 6 con 4 dados", "671/1296"},
		{"probabilidad de sacar al menos 1 cara al lanzar 3 monedas", "7/8"},
	}
	for _, c := range casos {
		m, ex, _, formula, err := LeerProbabilidad(&nucleo.Pregunta{Texto: c.texto})
		if err != nil {
			t.Fatalf("%q: %v", c.texto, err)
		}
		pr, err := Probabilidad(context.Background(), m, ex)
		if err != nil || pr.RatString() != c.p {
			t.Errorf("%q: %v %v (quería %s)", c.texto, pr, err, c.p)
		}
		if formula == nil || formula.Cmp(pr) != 0 {
			t.Errorf("%q: fórmula %v ≠ recuento %v", c.texto, formula, pr)
		}
	}
}

func ratsDe(xs ...int64) []*big.Rat {
	var out []*big.Rat
	for _, x := range xs {
		out = append(out, big.NewRat(x, 1))
	}
	return out
}

func TestSecuencias(t *testing.T) {
	casos := []struct {
		seq       []int64
		siguiente string
		confianza string
		texto     string
	}{
		{[]int64{2, 4, 8, 16}, "32", "alta", ""},
		{[]int64{1, 1, 2, 3, 5, 8}, "13", "alta", "dos anteriores"},
		{[]int64{2, 5, 10, 17}, "26", "media", "diferencias"},
		{[]int64{3, 5, 9, 17}, "33", "", ""},
		{[]int64{2, 3, 5, 7, 11}, "13", "alta", "primos"},
		{[]int64{1, 4, 9, 16, 25}, "36", "alta", "cuadrados"},
		{[]int64{5, 8, 11, 14}, "17", "alta", "suma 3"},
		{[]int64{1, 10, 2, 20, 3, 30}, "4", "", "intercaladas"},
		{[]int64{1, 2, 3}, "4", "media", "suma 1"},
	}
	for _, c := range casos {
		r, err := Siguiente(ratsDe(c.seq...), 1)
		if err != nil {
			t.Errorf("%v: %v", c.seq, err)
			continue
		}
		if TextoRat(r.Siguientes[0]) != c.siguiente {
			t.Errorf("%v → %v (%s), quería %s", c.seq, r.Siguientes[0], r.Descripcion, c.siguiente)
		}
		if c.confianza != "" && r.Confianza != c.confianza {
			t.Errorf("%v: confianza %q, quería %q", c.seq, r.Confianza, c.confianza)
		}
		if c.texto != "" && !strings.Contains(r.Descripcion, c.texto) {
			t.Errorf("%v: descripción %q sin %q", c.seq, r.Descripcion, c.texto)
		}
	}
	r, err := Siguiente(ratsDe(1, 2), 1)
	if !errors.Is(err, ErrPocosDatos) || r.Confianza != "pocos datos" || len(r.Siguientes) != 0 {
		t.Fatalf("1,2: %+v %v", r, err)
	}
	r, err = Siguiente([]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 4), big.NewRat(1, 8), big.NewRat(1, 16)}, 2)
	if err != nil || r.Siguientes[0].RatString() != "1/32" || len(r.Siguientes) != 2 {
		t.Fatalf("1/2, 1/4…: %+v %v", r, err)
	}
}

func TestCebra(t *testing.T) {
	pistas := []string{
		"Hay cinco casas.",
		"El inglés vive en la casa roja.",
		"El español tiene un perro.",
		"En la casa verde se bebe café.",
		"El ucraniano bebe té.",
		"La casa verde está inmediatamente a la derecha de la casa de marfil.",
		"El que fuma Old Gold tiene caracoles.",
		"En la casa amarilla se fuma Kools.",
		"En la casa del medio se bebe leche.",
		"El noruego vive en la primera casa.",
		"El que fuma Chesterfield vive al lado del que tiene el zorro.",
		"En la casa de al lado de donde está el caballo se fuma Kools.",
		"El que fuma Lucky Strike bebe zumo de naranja.",
		"El japonés fuma Parliament.",
		"El noruego vive al lado de la casa azul.",
		"¿Quién bebe agua?",
		"¿Quién tiene la cebra?",
	}
	r, err := ResolverCebra(context.Background(), pistas)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.NoEntendidas) != 0 {
		t.Fatalf("no entendidas: %q", r.NoEntendidas)
	}
	if !r.Unica {
		t.Fatal("debería ser única")
	}
	todo := strings.Join(r.Respuestas, "; ")
	if !strings.Contains(todo, "el japonés tiene la cebra") || !strings.Contains(todo, "el noruego bebe agua") {
		t.Fatalf("respuestas = %q\n%v", r.Respuestas, r.Tabla)
	}
	tb, no, err := Cebra(context.Background(), append(pistas, "El marciano baila tango con la luna."))
	if err != nil || len(tb.Filas) != 5 || len(no) != 1 {
		t.Fatalf("pista no entendida: %v %q", err, no)
	}
}

func TestColorear(t *testing.T) {
	aristas := [][2]string{{"A", "B"}, {"B", "C"}, {"C", "A"}, {"C", "D"}}
	m, err := Colorear(context.Background(), aristas, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range aristas {
		if m[a[0]] == m[a[1]] {
			t.Fatalf("%v mismo color", a)
		}
	}
	if _, err := Colorear(context.Background(), aristas, 2); !errors.Is(err, ErrSinColoreo) {
		t.Fatalf("un triángulo no se colorea con 2: %v", err)
	}
	k, _, err := NumeroCromatico(context.Background(), aristas, 5)
	if err != nil || k != 3 {
		t.Fatalf("número cromático = %d %v", k, err)
	}
}

// --- resolvers (golden exam rows 44-55)

func resolver(t *testing.T, nombre, texto string) nucleo.Respuesta {
	t.Helper()
	for _, r := range Resolutores(nucleotest.LematizadorSimple) {
		if r.Nombre() != nombre {
			continue
		}
		tr := nucleo.NuevaTraza(nil, nil)
		resp, err := r.Resolver(context.Background(), &nucleo.Pregunta{Texto: texto}, tr.Raiz())
		if err != nil {
			t.Fatalf("%s(%q): %v", nombre, texto, err)
		}
		return resp
	}
	t.Fatalf("no hay resolutor %s", nombre)
	return nucleo.Respuesta{}
}

func mejor(texto string) string {
	p := &nucleo.Pregunta{Texto: texto}
	nombre, puntos := "", 0.0
	for _, r := range Resolutores(nil) {
		if rec := r.Reconoce(p); rec.Puntos > puntos {
			nombre, puntos = r.Nombre(), rec.Puntos
		}
	}
	return nombre
}

func TestResolutoresExamen(t *testing.T) {
	casos := []struct {
		texto, resolutor string
		quiere           []string
	}{
		{sudokuFacil, "puzles.sudoku", []string{"única"}},
		{"¿cuántas soluciones tiene el problema de las 8 reinas?", "puzles.reinas", []string{"92"}},
		{"SEND+MORE=MONEY", "puzles.cripto", []string{"9567"}},
		{"jarras de 3 y 5 litros, ¿cómo saco 4?", "puzles.plan", []string{"6"}},
		{"el lobo, la cabra y la col", "puzles.plan", []string{"7"}},
		{"torres de Hanói con 3 discos", "puzles.plan", []string{"7"}},
		{"¿de cuántas formas elijo 3 de 10?", "puzles.conteo", []string{"120", "conté"}},
		{"¿cuántos anagramas tiene CASA?", "puzles.conteo", []string{"12"}},
		{"probabilidad de sacar 2 caras al lanzar 3 monedas", "puzles.probabilidad", []string{"3/8"}},
		{"2, 5, 10, 17, ¿siguiente?", "puzles.secuencia", []string{"26"}},
		{"¿cuántos números del 1 al 1000 son divisibles por 3 o por 5?", "puzles.conteo", []string{"467"}},
		{"¿de cuántas formas se sientan 5 personas en una mesa redonda?", "puzles.conteo", []string{"24"}},
		{"colorea este mapa con 3 colores: A-B, B-C, C-A, C-D", "puzles.colorear", []string{"3 colores"}},
		{"misioneros y caníbales: 3 misioneros y 3 caníbales con una barca de 2", "puzles.plan", []string{"11"}},
		{"resuelve el puzzle 1 2 3 4 5 6 0 7 8", "puzles.plan", []string{"2 pasos"}},
	}
	for _, c := range casos {
		if m := mejor(c.texto); m != c.resolutor {
			t.Errorf("%q: reconoce %q, quería %q", c.texto, m, c.resolutor)
		}
		r := resolver(t, c.resolutor, c.texto)
		if r.Nivel != nucleo.NivelCalculo || !r.Exito {
			t.Errorf("%q: nivel %q éxito %v", c.texto, r.Nivel, r.Exito)
		}
		todo := r.Texto + "\n" + strings.Join(r.Parrafos, "\n") + "\n" + strings.Join(r.Plan, "\n")
		for _, q := range c.quiere {
			if !strings.Contains(todo, q) {
				t.Errorf("%q: falta %q en %q", c.texto, q, r.Texto)
			}
		}
	}
	r := resolver(t, "puzles.plan", "jarras de 3 y 5 litros, ¿cómo saco 4?")
	if len(r.Plan) != 6 {
		t.Fatalf("plan de las jarras = %q", r.Plan)
	}
	r = resolver(t, "puzles.secuencia", "1, 2, ¿siguiente?")
	if r.Exito || !strings.Contains(r.Texto, "pocos datos") {
		t.Fatalf("1, 2: %+v", r)
	}
	r = resolver(t, "puzles.sudoku", sudokuFacil)
	if r.Tablero == nil || !r.Tablero.Fijas[0][0] || r.Tablero.Celdas[0][2] != 4 {
		t.Fatalf("tablero = %+v", r.Tablero)
	}
	r = resolver(t, "puzles.conteo", "¿cuántas contraseñas de 8 dígitos hay?")
	if !strings.Contains(r.Texto, "100000000") || !strings.Contains(r.Texto, "más pequeño") {
		t.Fatalf("espacio grande: %q", r.Texto)
	}
}

func TestReconoceRapido(t *testing.T) {
	p := &nucleo.Pregunta{Texto: sudokuInkala + "\n¿cuántas formas? SEND+MORE=MONEY jarras 2, 4, 8"}
	ini := time.Now()
	for i := 0; i < 20; i++ {
		for _, r := range Resolutores(nil) {
			r.Reconoce(p)
		}
	}
	if d := time.Since(ini) / 20; d > 5*time.Millisecond {
		t.Fatalf("Reconoce tarda %v", d)
	}
	if m := mejor("haz una función que sume los pares"); m != "" {
		t.Fatalf("no es un puzle: %q", m)
	}
}
