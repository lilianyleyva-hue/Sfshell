package puzles

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Regression tests for the problems found in review (one per bug).

func resolverConCtx(ctx context.Context, nombre, texto string) (nucleo.Respuesta, error) {
	for _, r := range Resolutores(nil) {
		if r.Nombre() == nombre {
			return r.Resolver(ctx, &nucleo.Pregunta{Texto: texto}, nucleo.NuevaTraza(nil, nil).Raiz())
		}
	}
	return nucleo.Respuesta{}, errors.New("no hay resolutor " + nombre)
}

// The round table had no bound on n: 200 000 people gave a 2 MB answer and 10⁶ people ran for
// seconds past the deadline.
func TestRegresionMesaRedondaAcotada(t *testing.T) {
	for _, n := range []string{"200000", "1000000", "999999999999"} {
		ini := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		r, err := resolverConCtx(ctx, "puzles.conteo", "¿de cuántas formas se sientan "+n+" personas en una mesa redonda?")
		cancel()
		if err == nil || !errors.Is(err, nucleo.ErrNoSoportado) || len(r.Texto) > 0 {
			t.Fatalf("%s personas: %v (%d bytes)", n, err, len(r.Texto))
		}
		if d := time.Since(ini); d > time.Second {
			t.Fatalf("%s personas tardó %v", n, d)
		}
	}
	r, err := resolverConCtx(context.Background(), "puzles.conteo", "¿de cuántas formas se sientan 1000 personas en una mesa redonda?")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Texto) > 6000 || strings.Count(r.Texto, factorial(999).String()) != 1 {
		t.Fatalf("1000 personas: %d bytes, el número debe salir una vez", len(r.Texto))
	}
}

// A timeout while checking the formula on the reduced model was reported as a disagreement.
func TestRegresionSinTiempoNoEsDesacuerdo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolverConCtx(ctx, "puzles.conteo", "¿de cuántas formas se sientan 500 personas en una mesa redonda?")
	if !errors.Is(err, nucleo.ErrSinTiempo) || strings.Contains(err.Error(), "no me fío") {
		t.Fatalf("err = %v", err)
	}
}

// "k personas en una mesa redonda de n sillas" was counted as choosing k of n people.
func TestRegresionMesaConSillas(t *testing.T) {
	casos := []struct {
		texto   string
		valor   int64
		entendi []string
	}{
		{"¿de cuántas formas se sientan 5 personas en una mesa redonda de 8 sillas?", 840, []string{"5 personas", "8 sillas"}},
		{"¿de cuántas formas se sientan 8 personas en una mesa redonda de 5 sillas?", 1344, []string{"5 de las 8 personas"}},
		{"¿de cuántas formas se sientan 5 personas en una mesa redonda?", 24, []string{"5 personas"}},
		{"¿de cuántas formas se sientan 12 personas en una mesa redonda de 20 sillas?", 3016991577600, []string{"12 personas", "20 sillas"}},
	}
	for _, c := range casos {
		r, err := resolverConCtx(context.Background(), "puzles.conteo", c.texto)
		if err != nil {
			t.Fatalf("%q: %v", c.texto, err)
		}
		if !strings.HasPrefix(r.Texto, big.NewInt(c.valor).String()+" ") {
			t.Errorf("%q = %q, quiero %d", c.texto, r.Texto, c.valor)
		}
		for _, e := range c.entendi {
			if !strings.Contains(r.Entendi, e) {
				t.Errorf("%q: entendí %q no dice %q", c.texto, r.Entendi, e)
			}
		}
	}
	// formula and brute force agree on circular multisets
	m, _, err := LeerConteo(&nucleo.Pregunta{Texto: "¿de cuántas formas se sientan 4 personas en una mesa redonda de 7 sillas?"})
	if err != nil {
		t.Fatal(err)
	}
	f, _, ok := Formula(m)
	b, err := ContarFuerza(context.Background(), m, 0)
	if !ok || err != nil || f.Cmp(b) != 0 || f.Int64() != 120 {
		t.Fatalf("fórmula %v, recuento %v (%v)", f, b, err)
	}
}

// A proven impossibility was returned as "no entiendo el enunciado".
func TestRegresionImposibleEsRespuesta(t *testing.T) {
	if _, err := Cripto(context.Background(), "DOS+DOS=CUATRO"); !errors.Is(err, ErrSinSolucion) || errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Fatalf("Cripto: %v", err)
	}
	casos := []struct{ nombre, texto, quiero string }{
		{"puzles.cripto", "¿cuánto vale cada letra en DOS+DOS=CUATRO?", "no tiene solución"},
		{"puzles.sudoku", "Resuelve este sudoku:\n11.......\n.........\n.........\n.........\n.........\n.........\n.........\n.........\n.........", "no tiene solución"},
		{"puzles.cebra", "Hay dos casas. El inglés vive en la casa roja. El español vive en la casa roja. ¿Quién tiene la cebra?", "no hay solución"},
	}
	for _, c := range casos {
		r, err := resolverConCtx(context.Background(), c.nombre, c.texto)
		if err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if !strings.Contains(r.Texto, c.quiero) || r.Nivel != nucleo.NivelCalculo || !r.Exito {
			t.Fatalf("%s: %+v", c.nombre, r)
		}
	}
	// an impossible sudoku with no repeated given: the corner cell has no number left
	var tb [9][9]int
	tb[0] = [9]int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	tb[4][0] = 9
	_, _, _, err := Sudoku(context.Background(), tb, nil)
	if !errors.Is(err, ErrSinSolucion) {
		t.Fatalf("sudoku imposible: %v", err)
	}
	// a cell out of range is still a reading problem
	tb[8][8] = 12
	if _, _, _, err := Sudoku(context.Background(), tb, nil); !errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Fatalf("casilla fuera de rango: %v", err)
	}
}

// Prose lines (a legend) broke the maze; lowercase s/e were taken as start/exit.
func TestRegresionLaberintoConLeyenda(t *testing.T) {
	r, err := resolverConCtx(context.Background(), "puzles.plan", "Resuelve este laberinto:\n#####\n#S..#\n#..E#\n#####\nS es la entrada y E es la salida")
	if err != nil || len(r.Plan) != 3 {
		t.Fatalf("laberinto con leyenda: %v %v", r.Plan, err)
	}
	lab, err := Laberinto([]string{"#######", "#S.se.#", "#####E#"})
	if err != nil {
		t.Fatalf("s y e minúsculas son camino: %v", err)
	}
	plan, err := BFS(context.Background(), lab, 0)
	if err != nil || len(plan.Pasos) != 5 {
		t.Fatalf("plan = %v %v", plan.Pasos, err)
	}
	if _, err := Laberinto([]string{"#S#S#", "#.E.#"}); err == nil || !strings.Contains(err.Error(), "inicio") {
		t.Fatalf("dos S: %v", err)
	}
}

// Pasted C or Python code was claimed as a maze.
func TestRegresionCodigoNoEsLaberinto(t *testing.T) {
	codigos := []string{
		"¿por qué no compila esto?\n#include <stdio.h>\nint main() {\n  printf(\"Hola\");\n  return EXIT_SUCCESS;\n}",
		"arregla esto:\n# Suma\ndef suma(a, b):\n    # devuelve la suma\n    return a + b\nprint(suma(1, 2))  # Escribe 3",
	}
	for _, c := range codigos {
		p := &nucleo.Pregunta{Texto: c}
		for _, r := range Resolutores(nil) {
			if rec := r.Reconoce(p); rec.Puntos > 0.3 {
				t.Errorf("%s reconoce código (%v %q)", r.Nombre(), rec.Puntos, rec.Lectura)
			}
		}
	}
	p := &nucleo.Pregunta{Texto: "#####\n#S..#\n#.#E#\n#####"}
	if rec := (resPlan{}).Reconoce(p); rec.Puntos < 0.8 {
		t.Fatalf("un laberinto dibujado: %v", rec.Puntos)
	}
}

// Malformed problems panicked or gave a false internal error.
func TestRegresionCSPEntradasMalas(t *testing.T) {
	ctx := context.Background()
	if _, _, err := Resolver(ctx, &Problema{Dom: []Dominio{DominioRango(0, 3)}, Restr: []Restriccion{{Alcance: []int{0}, Tipo: 7}}}, 1, nil); err == nil {
		t.Fatal("un tipo desconocido debería dar error")
	}
	if _, _, err := Resolver(ctx, &Problema{Dom: []Dominio{DominioRango(0, 3)}, Restr: []Restriccion{{Alcance: []int{0}, Tipo: -1, Rel: func([]int) bool { return true }}}}, 1, nil); err == nil {
		t.Fatal("un tipo negativo debería dar error")
	}
	sols, _, err := Resolver(ctx, &Problema{Dom: []Dominio{0, DominioRango(0, 2)}}, 1, nil)
	if err != nil || len(sols) != 0 {
		t.Fatalf("dominio vacío: %v %v", sols, err)
	}
	sols, _, err = Resolver(ctx, &Problema{Dom: []Dominio{DominioRango(0, 1), DominioRango(0, 1), DominioRango(0, 1)},
		Restr: []Restriccion{{Alcance: []int{0, 1, 2}, Tipo: Distintos}}}, 1, nil)
	if err != nil || len(sols) != 0 {
		t.Fatalf("tres variables, dos valores: %v %v", sols, err)
	}
}

// Zebra: "Nacionalidades:" did not mark people, and feminine nationalities were not read.
func TestRegresionCebraCategoriasYFemenino(t *testing.T) {
	texto := "Hay tres casas. Colores: roja, verde, azul. Nacionalidades: inglés, español, japonés. El inglés vive en la casa roja. " +
		"El japonés vive en la casa verde. La casa roja está a la izquierda de la casa verde. El español vive en la casa 1. ¿Quién vive en la casa azul?"
	r, err := resolverConCtx(context.Background(), "puzles.cebra", texto)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.Texto, "El español vive en la casa azul") {
		t.Fatalf("respuesta = %q", r.Texto)
	}
	res, err := ResolverCebra(context.Background(), []string{
		"Hay tres casas.", "La inglesa vive en la casa roja.", "La japonesa vive en la casa verde.",
		"La casa roja está a la izquierda de la casa verde.", "La española vive en la casa azul.",
		"La casa azul está a la izquierda de la casa roja.", "¿Quién vive en la casa verde?",
	})
	if err != nil || len(res.NoEntendidas) != 0 || !res.Unica {
		t.Fatalf("femeninos: %v %q %v", err, res.NoEntendidas, res.Unica)
	}
	if len(res.Respuestas) != 1 || !strings.Contains(res.Respuestas[0], "japonés") {
		t.Fatalf("respuestas = %q", res.Respuestas)
	}
	for _, par := range [][2]string{{"inglés", "inglesa"}, {"ingleses", "inglesas"}, {"japonés", "japonesa"}, {"francés", "francesa"}, {"roja", "rojo"}, {"verde", "verdes"}} {
		if raizCebra(par[0]) != raizCebra(par[1]) {
			t.Errorf("raíz %q = %q, %q = %q", par[0], raizCebra(par[0]), par[1], raizCebra(par[1]))
		}
	}
	// unknown values agree in gender
	res, err = ResolverCebra(context.Background(), []string{"Hay tres casas.", "El inglés vive en la casa roja.", "El noruego vive en la casa verde.", "¿Quién tiene la cebra?"})
	if err != nil {
		t.Fatal(err)
	}
	tabla := strings.ToLower(strings.Join(func() []string {
		var out []string
		for _, f := range res.Tabla.Filas {
			out = append(out, strings.Join(f, " "))
		}
		return out
	}(), " | "))
	if !strings.Contains(tabla, "nacionalidad desconocida") || strings.Contains(tabla, "nacionalidad desconocido") {
		t.Fatalf("tabla = %s", tabla)
	}
}

// Map colouring read only single-word region names.
func TestRegresionColorearNombresLargos(t *testing.T) {
	a, err := LeerAristas("colorea este mapa con 3 colores: Castilla y León - Madrid, Madrid - Castilla-La Mancha, Castilla-La Mancha - Castilla y León")
	if err != nil {
		t.Fatal(err)
	}
	quiero := [][2]string{{"Castilla y León", "Madrid"}, {"Madrid", "Castilla-La Mancha"}, {"Castilla-La Mancha", "Castilla y León"}}
	if len(a) != 3 {
		t.Fatalf("aristas = %q", a)
	}
	for i := range quiero {
		if a[i] != quiero[i] {
			t.Fatalf("aristas = %q", a)
		}
	}
	a, err = LeerAristas("colorea el mapa: la Región de Murcia - Andalucía, Galicia-Asturias y Asturias-Castilla y León")
	if err != nil || len(a) != 3 || a[0][0] != "Región de Murcia" || a[2][1] != "Castilla y León" {
		t.Fatalf("aristas = %q %v", a, err)
	}
	if _, err := LeerAristas("colorea este mapa con 3 colores: Castilla-La Mancha, Castilla-León, Madrid-Castilla-La Mancha"); !errors.Is(err, nucleo.ErrNoEntiendo) {
		t.Fatalf("una frontera ambigua: %v", err)
	}
	for _, texto := range []string{"Colorea A-B, B-C, C-A con 3 colores", "colorea el grafo A-B B-C C-A con 3 colores", "colorea este grafo:\nA: B, C\nB: C"} {
		a, err := LeerAristas(texto)
		if err != nil || len(a) != 3 {
			t.Errorf("%q: %q %v", texto, a, err)
		}
	}
	r, err := resolverConCtx(context.Background(), "puzles.colorear", "colorea este mapa con 3 colores: Castilla y León - Madrid, Madrid - Castilla-La Mancha, Castilla-La Mancha - Castilla y León")
	if err != nil || !strings.Contains(r.Texto, "Castilla y León") || !strings.Contains(r.Entendi, "Castilla-La Mancha") {
		t.Fatalf("colorear: %q %q %v", r.Texto, r.Entendi, err)
	}
}

// Spanish shown to users: no "-1 veces", no repeated column letters for big boards.
func TestRegresionTextos(t *testing.T) {
	r, err := resolverConCtx(context.Background(), "puzles.secuencia", "1, 0, 1, 0, 1, 0, ¿siguiente?")
	if err != nil || strings.Contains(r.Texto, "-1 veces") || !strings.HasPrefix(r.Texto, "El siguiente es 1") {
		t.Fatalf("secuencia: %q %v", r.Texto, err)
	}
	if d := describirRecurrencia([]*big.Rat{big.NewRat(-1, 1)}, new(big.Rat)); d != "cada término es el anterior cambiado de signo" {
		t.Fatalf("recurrencia = %q", d)
	}
	if d := describirRecurrencia([]*big.Rat{big.NewRat(2, 1), big.NewRat(-1, 1)}, big.NewRat(-3, 1)); d != "cada término es el doble del anterior menos el de dos antes menos 3" {
		t.Fatalf("recurrencia = %q", d)
	}
	cols := make([]int, 30)
	for i := range cols {
		cols[i] = (i*7 + 3) % 30
	}
	if txt := textoReinas(cols); !strings.HasPrefix(txt, "fila 1: columna 4") {
		t.Fatalf("30 columnas: %q", txt)
	}
	if txt := textoReinas([]int{0, 4, 7, 5, 2, 6, 1, 3}); txt != "a1, e2, h3, f4, c5, g6, b7, d8" {
		t.Fatalf("8 reinas: %q", txt)
	}
}

// Counting all queens was advertised up to 17 but 16 and 17 always ran out of time; perfect
// squares were checked with a linear scan.
func TestRegresionLimitesAnunciados(t *testing.T) {
	ini := time.Now()
	if _, _, err := Reinas(context.Background(), maxReinasContar+1, true); !errors.Is(err, nucleo.ErrNoSoportado) {
		t.Fatalf("%d reinas: %v", maxReinasContar+1, err)
	}
	if time.Since(ini) > 100*time.Millisecond {
		t.Fatal("el rechazo debe ser inmediato")
	}
	if c, _, err := Reinas(context.Background(), 12, true); err != nil || c != 14200 {
		t.Fatalf("12 reinas: %d %v", c, err)
	}
	for _, x := range []int64{0, 1, 4, 99_980_001, 1 << 62, 3_037_000_499 * 3_037_000_499} {
		if !esCuadrado(x) {
			t.Errorf("%d es un cuadrado", x)
		}
	}
	for _, x := range []int64{2, 3, 99_980_002, 1<<62 + 1, -4} {
		if esCuadrado(x) {
			t.Errorf("%d no es un cuadrado", x)
		}
	}
	ini = time.Now()
	r, err := resolverConCtx(context.Background(), "puzles.conteo", "¿cuántos números del 1 al 10000000 son cuadrados perfectos?")
	if err != nil || !strings.HasPrefix(r.Texto, "Hay 3162") {
		t.Fatalf("cuadrados: %q %v", r.Texto, err)
	}
	if d := time.Since(ini); d > 5*time.Second {
		t.Fatalf("cuadrados tardó %v", d)
	}
}
