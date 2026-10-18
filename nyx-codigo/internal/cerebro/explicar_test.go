package cerebro

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

const explicarSumaPares = "func SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n%2 == 0 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total\n}\n"

const explicarHayRepetidos = "func HayRepetidos(xs []int) bool {\n\tfor i := 0; i < len(xs); i++ {\n\t\tfor j := i + 1; j < len(xs); j++ {\n\t\t\tif xs[i] == xs[j] {\n\t\t\t\treturn true\n\t\t\t}\n\t\t}\n\t}\n\treturn false\n}\n"

const explicarUnir = "func Unir(ps []string) string {\n\ts := \"\"\n\tfor _, p := range ps {\n\t\ts += p\n\t}\n\treturn s\n}\n"

// explicarNecesitaGo skips tests that use the real sandbox.
func explicarNecesitaGo(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("usa el arenero real: se salta con -short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sin Go")
	}
}

// tablaFrases returns the per-line table the UI reads (first header "Línea").
func tablaFrases(r nucleo.Respuesta) (nucleo.Tabla, bool) {
	for _, tb := range r.Tablas {
		if len(tb.Cabecera) >= 2 && strings.HasPrefix(nucleo.Normalizar(tb.Cabecera[0]), "linea") {
			return tb, true
		}
	}
	return nucleo.Tabla{}, false
}

func TestExplicarSumaParesConCaja(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	r, tr := preguntar(t, c, "s", "explícame este código:\n"+explicarSumaPares, nil)
	todo := todoElTexto(r)
	if !r.Exito || r.Metodo != "analisis" {
		t.Fatalf("respuesta: %+v\n%s", r, titulos(tr))
	}
	tb, ok := tablaFrases(r)
	if !ok || r.Tablas[0].Cabecera[0] != "Línea" {
		t.Fatalf("la primera tabla debe ser Línea | Frase: %+v", r.Tablas)
	}
	if tb.Filas[0][0] != "1" || len(tb.Filas) < 3 {
		t.Errorf("filas: %v", tb.Filas)
	}
	for _, quiero := range []string{"acumulador", "suma", "Estimé O(n)", "Lo ejecuté con"} {
		if !strings.Contains(todo, quiero) {
			t.Errorf("falta «%s»:\n%s", quiero, todo)
		}
	}
	if r.Codigo == "" {
		t.Errorf("la respuesta lleva el código explicado (para el medianil)")
	}
	// a verified summary carries its badge; an unverified one is never shown
	if strings.Contains(todo, "En resumen") != (r.Nivel == nucleo.NivelEntendido) {
		t.Errorf("resumen y nivel no cuadran: %s\n%s", r.Nivel, todo)
	}
	if !strings.Contains(titulos(tr), "entradas fijas") {
		t.Errorf("no observó el código:\n%s", titulos(tr))
	}

	// follow-ups: con detalle (level 2 narration has more sentences), ponle comentarios
	r2, _ := preguntar(t, c, "s", "explícalo con detalle", nil)
	tb2, ok := tablaFrases(r2)
	if !ok || len(tb2.Filas) <= len(tb.Filas) {
		t.Errorf("con detalle: %d frases, antes %d", len(tb2.Filas), len(tb.Filas))
	}
	r3, _ := preguntar(t, c, "s", "ponle comentarios", nil)
	if !strings.Contains(r3.Codigo, "// Crea el acumulador `total`") || !strings.Contains(todoElTexto(r3), "Solo he añadido comentarios") {
		t.Errorf("comentarios: %+v", r3)
	}
}

// §11: explicar of a nested-loop function says "cuadrática" (static) and the measured class agrees.
func TestExplicarCuadraticaMedida(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	r, tr := preguntar(t, c, "s", "¿es eficiente?\n"+explicarHayRepetidos, nil)
	if !strings.Contains(r.Texto, "O(n²)") || !strings.Contains(r.Texto, "cuadrática") {
		t.Fatalf("texto: %q\n%s", r.Texto, titulos(tr))
	}
	if !strings.Contains(r.Texto, "midiendo parece cuadrática") {
		t.Errorf("la medida debe coincidir: %q\n%s", r.Texto, titulos(tr))
	}
	hay := false
	for _, tb := range r.Tablas {
		if strings.HasPrefix(tb.Titulo, "Tiempos medidos") && len(tb.Filas) >= 2 {
			hay = true
		}
	}
	if !hay {
		t.Errorf("faltan los tiempos medidos: %+v", r.Tablas)
	}
}

func TestExplicarOptimizarComparaConElOriginal(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	r, tr := preguntar(t, c, "s", "hazlo más rápido:\n"+explicarUnir, nil)
	if !strings.Contains(r.Codigo, "strings.Builder") || r.Diff == "" {
		t.Fatalf("reescritura: %+v\n%s", r, titulos(tr))
	}
	ok := false
	for _, cp := range r.Comprobaciones {
		if cp.Nivel == nucleo.NivelEntendido && strings.Contains(cp.Texto, "Da lo mismo que tu código") && cp.Pasan == cp.Total && cp.Total > 0 {
			ok = true
		}
	}
	if !ok || !r.Exito {
		t.Errorf("la reescritura se compara con el original antes de mostrarla: %+v", r.Comprobaciones)
	}
}

func TestExplicarSinCajaEsHonesto(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "explícame este código:\n"+explicarSumaPares, nil)
	if _, ok := tablaFrases(r); !ok {
		t.Fatalf("sin caja también explica línea a línea: %+v", r)
	}
	todo := todoElTexto(r)
	if r.Nivel != nucleo.NivelSinComprobar || len(r.Comprobaciones) != 0 || strings.Contains(todo, "En resumen") {
		t.Errorf("sin ejecutar no hay comprobaciones ni resumen: %s %+v", r.Nivel, r.Comprobaciones)
	}
	if !strings.Contains(todo, "No lo he ejecutado: no tengo caja segura") {
		t.Errorf("dice por qué no lo ejecutó:\n%s", todo)
	}
	r2 := c.Responder(context.Background(), nucleo.Peticion{Tarea: "sin-codigo", Sesion: "s2", Accion: "explicar"}, nucleo.NuevaTraza(nil, nil))
	if r2.Exito || !strings.Contains(r2.Texto, "Pega el código") {
		t.Errorf("sin código: %+v", r2)
	}
}

func TestExplicarFraseComplejidad(t *testing.T) {
	x := &explicacion{fn: analisis.InfoFunc{OGrande: "O(n²)", Motivo: "dos bucles anidados que recorren la entrada"}}
	if s := x.fraseComplejidad(); s != "Estimé O(n²) (cuadrática) mirando el código: dos bucles anidados que recorren la entrada." {
		t.Errorf("estática: %q", s)
	}
	x.medida = "cuadrática"
	if s := x.fraseComplejidad(); !strings.HasSuffix(s, "; midiendo parece cuadrática.") {
		t.Errorf("coinciden: %q", s)
	}
	x.medida = "lineal"
	if s := x.fraseComplejidad(); !strings.Contains(s, "pero midiendo parece lineal, así que no coinciden") {
		t.Errorf("no coinciden: %q", s)
	}
	x.fn.OGrande, x.medida = "O(n)", "n·log n"
	if s := x.fraseComplejidad(); !strings.Contains(s, "midiendo parece lineal o n·log n") || strings.Contains(s, "no coinciden") {
		t.Errorf("lineal y n·log n: %q", s)
	}
	if !explicarCoinciden("lineal", "n·log n") || explicarCoinciden("lineal", "cuadrática") {
		t.Errorf("lineal y n·log n se confunden al medir; lineal y cuadrática no")
	}
}

func TestExplicarGenerador(t *testing.T) {
	lista := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}, {Nombre: "x", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TBool}}
	gen := explicarGenerador(lista, 1000)
	cs, ok := gen(100)
	if !ok || len(cs.Entradas[0].([]nucleo.Valor)) != 100 || cs.Entradas[1] != -1 {
		t.Fatalf("caso: %v %v", ok, cs.Entradas)
	}
	if _, ok := gen(10000); ok {
		t.Errorf("respeta el tamaño máximo")
	}
	vistos := map[nucleo.Valor]bool{}
	for _, v := range cs.Entradas[0].([]nucleo.Valor) {
		vistos[v] = true
	}
	if len(vistos) != 100 {
		t.Errorf("los valores son distintos, para que un bucle que busca repetidos llegue al final")
	}
	texto := nucleo.Firma{Nombre: "G", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TInt}}
	if cs, _ := explicarGenerador(texto, 1000)(50); len(cs.Entradas[0].(string)) != 50 {
		t.Errorf("texto de 50 letras: %v", cs.Entradas)
	}
	dos := nucleo.Firma{Nombre: "H", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	mapa := nucleo.Firma{Nombre: "M", Params: []nucleo.Param{{Nombre: "m", Tipo: nucleo.MapaDe(nucleo.TString, nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	if explicarGenerador(dos, 1000) != nil || explicarGenerador(mapa, 1000) != nil {
		t.Errorf("sin una entrada que crezca no se mide")
	}
}

func TestExplicarFrasesYRoles(t *testing.T) {
	s := explicarFraseRoles(map[string]string{"total": "acumulador", "i": "índice", "j": "índice", "n": "auxiliar"}, 0)
	if s != "Papel de cada variable: `total` es el acumulador (sirve para ir juntando el resultado); `i` y `j` son índices (sirven para marcar la posición por la que va)." {
		t.Errorf("roles: %q", s)
	}
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	r := nucleo.ResultadoCaso{Ejecutado: true, OK: true, Obtenido: []nucleo.Valor{6}, Eventos: []nucleo.EventoVar{
		{Linea: 2, Var: "total", Valor: "0"}, {Linea: 3, Var: "n", Valor: "1"}, {Linea: 3, Var: "n", Valor: "2"},
		{Linea: 5, Var: "total", Valor: "2"}, {Linea: 3, Var: "n", Valor: "4"}, {Linea: 5, Var: "total", Valor: "6"}}}
	got := explicarFraseTraza(f, "nums = [1, 2, 4]", r, map[string]string{"total": "acumulador"})
	if got != "Lo ejecuté con nums = [1, 2, 4]: `total` pasa por 0 → 2 → 6, `n` pasa por 1 → 2 → 4; devuelve 6." {
		t.Errorf("traza: %q", got)
	}
}

// Regression: sub-millisecond timings are noise; no class is reported from them.
func TestExplicarClasificarMedidasPequenas(t *testing.T) {
	ruido := [][]analisis.Medida{
		{{N: 100, Micros: 0}, {N: 1000, Micros: 0}, {N: 10000, Micros: 4}, {N: 100000, Micros: 178}},
		{{N: 100, Micros: 0}, {N: 1000, Micros: 1}, {N: 10000, Micros: 12}, {N: 100000, Micros: 275}},
		{{N: 10000, Micros: 4}, {N: 100000, Micros: 1200}},
	}
	for _, ms := range ruido {
		if cl, nota := explicarClasificarMedidas(ms); cl != "" || nota == "" {
			t.Errorf("%v: clase %q nota %q", ms, cl, nota)
		}
	}
	if _, nota := explicarClasificarMedidas(ruido[0]); !strings.Contains(nota, "va tan rápido") || !strings.Contains(nota, "178 µs") {
		t.Errorf("nota: %q", nota)
	}
	if cl, _ := explicarClasificarMedidas([]analisis.Medida{{N: 100, Micros: 3}, {N: 1000, Micros: 400}, {N: 10000, Micros: 40000}}); cl != "cuadrática" {
		t.Errorf("cuadrática: %q", cl)
	}
	if cl, _ := explicarClasificarMedidas([]analisis.Medida{{N: 1000, Micros: 150}, {N: 10000, Micros: 1500}, {N: 100000, Micros: 15000}}); cl != "lineal" {
		t.Errorf("lineal: %q", cl)
	}
}

// Regression: a fast linear function is never reported as "midiendo parece cuadrática".
func TestExplicarLinealRapidaNoSeMideMal(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	const suma = "func Suma(xs []int) int {\n\ttotal := 0\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn total\n}\n"
	for i := 0; i < 2; i++ {
		r, tr := preguntar(t, c, "s"+itoa(int64(i)), "¿es eficiente?\n"+suma, nil)
		if !strings.Contains(r.Texto, "Estimé O(n)") {
			t.Fatalf("texto: %q\n%s", r.Texto, titulos(tr))
		}
		if strings.Contains(r.Texto, "no coinciden") || strings.Contains(r.Texto, "cuadrática") || strings.Contains(r.Texto, "cúbica") {
			t.Errorf("medida falsa: %q\n%s", r.Texto, titulos(tr))
		}
	}
}

// Regression: measuring the growth has its own time limit and says why it stopped.
func TestExplicarMedirTieneLimite(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	const cubo = "func Cubo(n int) int {\n\ts := 0\n\tfor i := 0; i < n*n*n; i++ {\n\t\ts++\n\t}\n\treturn s\n}\n"
	r, tr := preguntar(t, c, "s", "¿es eficiente?\n"+cubo, nil)
	// the measuring step lasts at most explicarMaxMedir (plus slack), measured from the step before it
	pasos := tr.Pasos()
	for i, p := range pasos {
		if strings.HasPrefix(p.Titulo, "Mido cuánto tarda con entradas") && i > 0 {
			if d := time.Duration(p.Ms-pasos[i-1].Ms) * time.Millisecond; d > explicarMaxMedir+2*time.Second {
				t.Errorf("medir duró %v: tiene su propio límite\n%s", d, titulos(tr))
			}
		}
	}
	if strings.Contains(r.Texto, "no conseguí medidas suficientes") {
		t.Errorf("motivo equivocado: %q\n%s", r.Texto, titulos(tr))
	}
	if !strings.Contains(r.Texto, "midiendo") && !strings.Contains(r.Texto, "paré de medir") && !strings.Contains(r.Texto, "no lo he medido") {
		t.Errorf("dice qué pasó al medir: %q", r.Texto)
	}
	if strings.Contains(r.Texto, "midiendo parece cúbica") && !strings.Contains(r.Texto, "crece más deprisa") {
		t.Errorf("si mide más de lo estimado lo dice así: %q", r.Texto)
	}
}

// Regression: estimates other than the four common ones are named, measured when ≥ O(n), or the answer
// says why not.
func TestExplicarOrdenesRaros(t *testing.T) {
	casos := []struct {
		o     string
		clase string
		maxN  int
		nota  bool
	}{
		{"O(n√n)", "entre lineal y cuadrática", 100000, false},
		{"O(n² log n)", "algo más que cuadrática", 10000, false},
		{"O(n⁴)", "polinómica de grado 4", 0, true},
		{"O(2ⁿ)?", "exponencial", 0, true},
		{"O(n log² n)", "algo más que lineal", 100000, false},
		{"O(log² n)", "logarítmica", 0, false},
		{"O(√n)", "raíz cuadrada de n", 0, false},
		{"O(n³)", "cúbica", 1000, false},
	}
	for _, c := range casos {
		if got := explicarClase(c.o); got != c.clase {
			t.Errorf("%s: clase %q, quiero %q", c.o, got, c.clase)
		}
		maxN, nota := explicarMaxN(c.o)
		if maxN != c.maxN || (nota != "") != c.nota {
			t.Errorf("%s: maxN %d nota %q", c.o, maxN, nota)
		}
	}
	x := &explicacion{fn: analisis.InfoFunc{OGrande: "O(n⁴)"}}
	_, x.notaMedida = explicarMaxN("O(n⁴)")
	if s := x.fraseComplejidad(); !strings.Contains(s, "(no lo he medido: crece tan deprisa") {
		t.Errorf("dice por qué no mide: %q", s)
	}
	x = &explicacion{fn: analisis.InfoFunc{OGrande: "O(n√n)"}, medida: "cuadrática"}
	if s := x.fraseComplejidad(); strings.Contains(s, "no coinciden") || !strings.Contains(s, "parecida") {
		t.Errorf("n√n medida como cuadrática es parecida: %q", s)
	}
	x = &explicacion{fn: analisis.InfoFunc{OGrande: "O(n)"}, medida: "cúbica"}
	if s := x.fraseComplejidad(); !strings.Contains(s, "pero midiendo parece cúbica, así que no coinciden: crece más deprisa") {
		t.Errorf("más deprisa: %q", s)
	}
}

// Regression (§0, N3): "más rápido" is said only when the timing shows it.
func TestExplicarVeredictoTiempo(t *testing.T) {
	if s, ok := explicarVeredictoTiempo(10000, 45000, 2000, false); !ok || s != "Lo medí con n = 10000: tu código tarda 45,0 ms y el nuevo 2,0 ms." {
		t.Errorf("más rápido: %q %v", s, ok)
	}
	if s, ok := explicarVeredictoTiempo(10000, 300, 100, false); ok || !strings.Contains(s, "demasiado poco") {
		t.Errorf("tiempos de ruido: %q %v", s, ok)
	}
	if s, ok := explicarVeredictoTiempo(10000, 5000, 4900, false); ok || !strings.Contains(s, "no he visto que vaya más rápido") {
		t.Errorf("igual de rápido: %q %v", s, ok)
	}
	if s, ok := explicarVeredictoTiempo(10000, 2_000_000, 2000, true); !ok || !strings.Contains(s, "seguía sin terminar a los 2 s") {
		t.Errorf("el original no termina: %q %v", s, ok)
	}
	if s, ok := explicarVeredictoTiempo(10000, 500, 2000, true); ok || !strings.Contains(s, "no sé cuál") {
		t.Errorf("cortado demasiado pronto: %q %v", s, ok)
	}
	if got := explicarTiempo(2_000_000); got != "2 s" {
		t.Errorf("tiempo: %q", got)
	}
}

func TestExplicarOptimizarNoAfirmaSinMedir(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	r, tr := preguntar(t, c, "s", "hazlo más rápido:\n"+explicarUnir, nil)
	todo := todoElTexto(r)
	switch {
	case strings.HasPrefix(r.Texto, "Así `Unir` va más rápido"):
		if !strings.Contains(todo, "Lo medí con n = ") {
			t.Errorf("dice «más rápido» sin medirlo:\n%s\n%s", todo, titulos(tr))
		}
	case strings.HasPrefix(r.Texto, "Creo que"), strings.HasPrefix(r.Texto, "Este cambio en `Unir` da lo mismo"):
	default:
		t.Errorf("texto: %q\n%s", r.Texto, titulos(tr))
	}
}

// ---- regression tests for the review of explicar ----

// ejecutorEspia records what is prepared and every case run, on top of any Ejecutor.
type ejecutorEspia struct {
	nucleo.Ejecutor
	mu         sync.Mutex
	preparados []nucleo.Preparacion
	casos      []nucleo.Caso
}

func (e *ejecutorEspia) Preparar(ctx context.Context, p nucleo.Preparacion) (nucleo.Binario, nucleo.Compilacion, error) {
	e.mu.Lock()
	e.preparados = append(e.preparados, p)
	e.mu.Unlock()
	b, comp, err := e.Ejecutor.Preparar(ctx, p)
	if err != nil || b == nil {
		return b, comp, err
	}
	return &binarioEspia{Binario: b, e: e}, comp, nil
}

type binarioEspia struct {
	nucleo.Binario
	e *ejecutorEspia
}

func (b *binarioEspia) Probar(ctx context.Context, casos []nucleo.Caso, op nucleo.OpcionesProbar) ([][]nucleo.ResultadoCaso, error) {
	b.e.mu.Lock()
	b.e.casos = append(b.e.casos, casos...)
	b.e.mu.Unlock()
	return b.Binario.Probar(ctx, casos, op)
}

// azarCorridos counts the random cases that were run to check a summary (growth inputs are left out).
func (e *ejecutorEspia) azarCorridos() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.casos {
		if c.Origen == nucleo.OrigenAzar && !strings.HasPrefix(c.Nota, "tamaño") {
			n++
		}
	}
	return n
}

func espiaFalso(funcs map[int]func([]nucleo.Valor) ([]nucleo.Valor, string)) *ejecutorEspia {
	return &ejecutorEspia{Ejecutor: &nucleotest.EjecutorFalso{Funcs: funcs}}
}

var reAzarDicho = regexp.MustCompile(`y (\d+) al azar`)

// azarHonesto: the "¿por qué?" text never claims more random cases than really ran.
func azarHonesto(t *testing.T, r nucleo.Respuesta, e *ejecutorEspia, tr *nucleo.Traza) {
	t.Helper()
	corridos := e.azarCorridos()
	just := strings.Join(r.Justificacion, "\n")
	m := reAzarDicho.FindStringSubmatch(just)
	switch {
	case corridos == 0 && strings.Contains(just, "al azar"):
		t.Errorf("no ejecutó ninguna entrada al azar y dice que sí:\n%s\n%s", just, titulos(tr))
	case m != nil:
		if n, _ := strconv.Atoi(m[1]); n > corridos || n == 0 {
			t.Errorf("dice %d al azar y corrieron %d:\n%s", n, corridos, just)
		}
	}
}

// Review bug: the Justificacion said the random cases ran even when the summarizer found nothing (they
// only run to check a summary).
func TestExplicarJustificacionSoloLoQueCorrio(t *testing.T) {
	hash := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		s, _ := in[0].(string)
		h := 7
		for _, r := range s {
			h = h*31 + int(r)%13
		}
		return []nucleo.Valor{h}, ""
	}
	e := espiaFalso(map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: hash})
	c := nuevoCerebroPrueba(t, e)
	const codigo = "func H(s string) int {\n\th := 7\n\tfor _, r := range s {\n\t\th = h*31 + int(r)%13\n\t}\n\treturn h\n}\n"
	r, tr := preguntar(t, c, "s", "explícame este código:\n"+codigo, nil)
	if !strings.Contains(strings.Join(r.Justificacion, "\n"), "Lo compilé en la caja segura y lo ejecuté con") {
		t.Fatalf("dice con qué lo ejecutó: %+v\n%s", r.Justificacion, titulos(tr))
	}
	azarHonesto(t, r, e, tr)
	if !strings.Contains(r.Texto, "En resumen") && strings.Contains(strings.Join(r.Justificacion, "\n"), "coincide en todas") {
		t.Errorf("sin resumen no dice que coincide: %+v", r.Justificacion)
	}

	// with a summary that fits, the random cases do run, and they are 300 (§4.5)
	pares := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		xs, _ := in[0].([]nucleo.Valor)
		total := 0
		for _, v := range xs {
			if n, ok := v.(int); ok && n%2 == 0 {
				total += n
			}
		}
		return []nucleo.Valor{total}, ""
	}
	e2 := espiaFalso(map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: pares})
	c2 := nuevoCerebroPrueba(t, e2)
	r2, tr2 := preguntar(t, c2, "s", "explícame este código:\n"+explicarSumaPares, nil)
	azarHonesto(t, r2, e2, tr2)
	if strings.Contains(r2.Texto, "En resumen") && !strings.Contains(r2.Texto, "lo comprobé con 300 entradas al azar") {
		t.Errorf("el resumen se comprueba con 300 entradas al azar: %q", r2.Texto)
	}
}

// §4.5 / §1 #5: the summary is checked on 300 random cases, spread over the generated sizes.
func TestExplicarTrescientasAlAzar(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	k := c.nuevaTarea(context.Background(), nucleo.Peticion{}, c.sesion("s"), nucleo.NuevaTraza(nil, nil))
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	todos := c.generarCasos(k, f, nil, -1, explicarAzarPedidos)
	cs := explicarMuestra(todos, explicarAzar)
	if len(cs) != 300 {
		t.Fatalf("%d casos al azar (de %d generados), quiero 300", len(cs), len(todos))
	}
	if cs[len(cs)-1].Nota == todos[explicarAzar-1].Nota && len(todos) > explicarAzar {
		t.Errorf("la muestra llega hasta los casos más grandes")
	}
	if got := explicarMuestra(todos[:10], 300); len(got) != 10 {
		t.Errorf("con menos casos los deja todos: %d", len(got))
	}
}

const explicarSumaYUnir = "func Suma(xs []int) int {\n\ttotal := 0\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn total\n}\n\nfunc Unir(ps []string) string {\n\ts := \"\"\n\tfor _, p := range ps {\n\t\ts += p\n\t}\n\treturn s\n}\n"

// Review bug: "hazlo más rápido" compared and timed the first function of the file, not the one the
// rewrite changed.
func TestExplicarOptimizaLaFuncionQueCambia(t *testing.T) {
	unir := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		xs, _ := in[0].([]nucleo.Valor)
		var sb strings.Builder
		for _, v := range xs {
			s, _ := v.(string)
			sb.WriteString(s)
		}
		return []nucleo.Valor{sb.String()}, ""
	}
	e := espiaFalso(map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: unir, 1: unir})
	c := nuevoCerebroPrueba(t, e)
	r, tr := preguntar(t, c, "s", "hazlo más rápido:\n"+explicarSumaYUnir, nil)
	if !strings.Contains(r.Codigo, "strings.Builder") {
		t.Fatalf("reescritura: %+v\n%s", r, titulos(tr))
	}
	comparoUnir := false
	for _, p := range e.preparados {
		if p.Firma.Nombre == "Suma" {
			t.Errorf("compiló Suma, que no cambia: %+v", p.Firma)
		}
		if p.Firma.Nombre == "Unir" && len(p.Variantes) == 2 && strings.Contains(p.Variantes[0], "func Unir") && strings.Contains(p.Variantes[1], "strings.Builder") {
			comparoUnir = true
		}
	}
	if !comparoUnir {
		t.Errorf("compara la Unir nueva con la original: %+v", e.preparados)
	}
	if !strings.Contains(r.Texto, "`Unir`") || strings.Contains(r.Texto, "Suma") {
		t.Errorf("habla de la función que cambia: %q", r.Texto)
	}
	if _, ok := badge(r, "Da lo mismo que tu código"); !ok {
		t.Errorf("insignia de la comparación: %+v", r.Comprobaciones)
	}
	if !strings.Contains(titulos(tr), "El cambio está en `Unir`") {
		t.Errorf("la traza dice qué función compara:\n%s", titulos(tr))
	}
	// wording: no "Lo hago más rápido: El texto …" (capital letter after a colon, the problem as the change)
	todo := todoElTexto(r)
	if strings.Contains(todo, "rápido: El") || strings.Contains(todo, "Lo hago más rápido") {
		t.Errorf("redacción:\n%s", todo)
	}
	if !strings.Contains(todo, "Lo lento está en la línea 12, dentro de `Unir`.") {
		t.Errorf("dice dónde está lo lento:\n%s", todo)
	}
}

func TestExplicarCambiadas(t *testing.T) {
	antes := "package main\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() int { return 2 }\n"
	despues := "package main\n\nfunc A() int {\n\treturn 1\n}\n\nfunc B() int {\n\t// dos\n\treturn 3\n}\n"
	if got := explicarCambiadas(antes, despues); len(got) != 1 || got[0] != "B" {
		t.Errorf("solo cambia B: %v", got)
	}
	if got := explicarCambiadas(antes, "package main\n\nfunc A() int { return 1 }\n\nfunc B() int {\n\treturn 2\n}\n"); len(got) != 0 {
		t.Errorf("cambiar el formato o los comentarios no es un cambio: %v", got)
	}
	metodo := "type P struct{ x int }\n\nfunc (p *P) Doble() int { return p.x * 2 }\n"
	if got := explicarCambiadas(metodo, strings.Replace(metodo, "* 2", "+ p.x", 1)); len(got) != 1 || got[0] != "P.Doble" {
		t.Errorf("métodos con el nombre de analisis: %v", got)
	}
}

// Review N2: a first message that carries code honours "ponle comentarios" and "con detalle" (§4.14.2).
func TestExplicarPeticionConComentariosYDetalle(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, tr := preguntar(t, c, "a", "ponle comentarios a este código:\n"+explicarSumaPares, nil)
	if !strings.Contains(r.Codigo, "//") || !strings.Contains(todoElTexto(r), "comentarios") {
		t.Errorf("comentarios en el primer mensaje: %+v\n%s", r, titulos(tr))
	}
	normal, _ := preguntar(t, c, "b", "explícame este código:\n"+explicarSumaPares, nil)
	detalle, tr2 := preguntar(t, c, "c", "explícalo con detalle:\n"+explicarSumaPares, nil)
	tn, ok1 := tablaFrases(normal)
	td, ok2 := tablaFrases(detalle)
	if !ok1 || !ok2 || len(td.Filas) <= len(tn.Filas) {
		t.Errorf("con detalle da más frases: %d, normal %d\n%s", len(td.Filas), len(tn.Filas), titulos(tr2))
	}
	if !strings.Contains(strings.Join(detalle.Sugerencias, " "), "más simple") {
		t.Errorf("tras «con detalle» sugiere «más simple»: %v", detalle.Sugerencias)
	}
}

const explicarVocales = "func Vocales(s string) int {\n\tn := 0\n\tfor _, r := range s {\n\t\tif strings.ContainsRune(\"aeiou\", r) {\n\t\t\tn++\n\t\t}\n\t}\n\treturn n\n}\n"

// Review N2: a pasted function that forgot its import runs completed, and the answer says what was
// missing (in Spanish).
func TestExplicarSinImportSeEjecuta(t *testing.T) {
	vocales := func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		s, _ := in[0].(string)
		n := 0
		for _, r := range s {
			if strings.ContainsRune("aeiou", r) {
				n++
			}
		}
		return []nucleo.Valor{n}, ""
	}
	e := espiaFalso(map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){0: vocales})
	c := nuevoCerebroPrueba(t, e)
	r, tr := preguntar(t, c, "s", "explícame este código:\n"+explicarVocales, nil)
	todo := todoElTexto(r)
	importa := false
	for _, p := range e.preparados {
		if len(p.Variantes) > 0 && strings.Contains(p.Variantes[0], `"strings"`) {
			importa = true
		}
	}
	if !importa {
		t.Fatalf("lo compila con el import que falta:\n%s\n%s", todo, titulos(tr))
	}
	for _, quiero := range []string{"Le falta importar `strings`", "Para ejecutarlo se lo añadí", "Lo ejecuté con s = "} {
		if !strings.Contains(todo, quiero) {
			t.Errorf("falta «%s»:\n%s", quiero, todo)
		}
	}
	for _, no := range []string{"undefined", "No lo he ejecutado", "así que no lo he ejecutado"} {
		if strings.Contains(todo, no) {
			t.Errorf("sobra «%s»:\n%s", no, todo)
		}
	}

	// without a sandbox it says what is missing and that it did not run it
	c2 := nuevoCerebroPrueba(t, nil)
	r2, _ := preguntar(t, c2, "s", "explícame este código:\n"+explicarVocales, nil)
	todo2 := todoElTexto(r2)
	if !strings.Contains(todo2, "Le falta importar `strings`") || !strings.Contains(todo2, "No lo he ejecutado: no tengo caja segura") || strings.Contains(todo2, "se lo añadí") {
		t.Errorf("sin caja:\n%s", todo2)
	}
}

// Review bug: the trace's "Línea" column used the gofmt'd file's lines for code that is not gofmt'd.
func TestExplicarMapaLineas(t *testing.T) {
	const codigo = "func SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums { if n%2 == 0 { total += n } }\n\treturn total\n}\n"
	fuente, err := explicarFuente(codigo, "SumaPares")
	if err != nil {
		t.Fatal(err)
	}
	m := explicarMapaLineas(codigo, fuente, "SumaPares")
	quiero := map[string]int{"total := 0": 2, "total += n": 3, "return total": 4, "if n%2 == 0": 3}
	for i, l := range strings.Split(fuente, "\n") {
		for trozo, lu := range quiero {
			if strings.Contains(l, trozo) && m[i+1] != lu {
				t.Errorf("«%s» está en la línea %d del archivo compilado: la del usuario es %d, no %d", trozo, i+1, lu, m[i+1])
			}
		}
	}
	const metodo = "type P struct{ x int }\nfunc (p *P) Doble() int { y := p.x; return y * 2 }\n"
	fm, err := explicarFuente(metodo, "P.Doble")
	if err != nil {
		t.Fatal(err)
	}
	mm := explicarMapaLineas(metodo, fm, "P.Doble")
	for i, l := range strings.Split(fm, "\n") {
		if strings.Contains(l, "return y * 2") && mm[i+1] != 2 {
			t.Errorf("método: %v\n%s", mm, fm)
		}
	}
}

func TestExplicarTrazaLineasDelUsuario(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	const codigo = "func SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums { if n%2 == 0 { total += n } }\n\treturn total\n}\n"
	r, tr := preguntar(t, c, "s", "explícame este código:\n"+codigo, nil)
	hay := false
	for _, tb := range r.Tablas {
		if !strings.HasPrefix(tb.Titulo, "Así cambian las variables") {
			continue
		}
		for _, f := range tb.Filas {
			if f[2] != "total" {
				continue
			}
			hay = true
			if f[1] != "2" && f[1] != "3" {
				t.Errorf("`total` cambia en las líneas 2 y 3 del código, no en la %s: %v", f[1], tb.Filas)
			}
		}
	}
	if !hay {
		t.Errorf("sin traza de `total`: %+v\n%s", r.Tablas, titulos(tr))
	}
}

// Review quality: compiler errors in simple Spanish (reparar.Traducir); code that cannot be parsed is
// not narrated (the parser's BadExpr would reach the gutter).
func TestExplicarErroresEnEspanol(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	const roto = "func F(xs []int) int {\n\ts := 0\n\tfor i := 0; i < len(xs) i++ {\n\t\ts += xs[i]\n\t}\n\treturn s\n}\n"
	r, _ := preguntar(t, c, "a", "explícame este código:\n"+roto, nil)
	todo := todoElTexto(r)
	if _, ok := tablaFrases(r); ok || strings.Contains(todo, "BadExpr") || strings.Contains(todo, "expected") {
		t.Errorf("no narra código que no se puede leer:\n%s", todo)
	}
	if !strings.Contains(todo, "no se puede leer") || !strings.Contains(todo, "en la línea 3") || r.Exito {
		t.Errorf("dice en español por qué:\n%s", todo)
	}
	const tipos = "func F() int {\n\treturn totl\n}\n"
	r2, _ := preguntar(t, c, "b", "explícame este código:\n"+tipos, nil)
	todo2 := todoElTexto(r2)
	if !strings.Contains(todo2, "Ojo: así como está no compila (en la línea 2, ") || strings.Contains(todo2, "undefined") {
		t.Errorf("error de tipos en español:\n%s", todo2)
	}
}

// Review quality: "excelente" does not ask whether the code is slow.
func TestExplicarPideEficienciaPalabras(t *testing.T) {
	casos := map[string]bool{
		"explícame esta función excelente": false,
		"explícame este código":            false,
		"¿es eficiente?":                   true,
		"¿es lento con listas grandes?":    true,
		"¿cuánto tarda?":                   true,
		"¿qué complejidad tiene?":          true,
		"¿es O(n)?":                        true,
		"¿cómo de rápida es?":              true,
	}
	for texto, quiero := range casos {
		if got := explicarPideEficiencia(&tarea{p: nucleo.Peticion{Texto: texto}}); got != quiero {
			t.Errorf("%q: %v, quiero %v", texto, got, quiero)
		}
	}
}

func TestExplicarErrorTexto(t *testing.T) {
	got := explicarErrorTexto(nucleo.ErrorGo{Linea: 4, Msg: "undefined: strings"})
	if !strings.HasPrefix(got, "en la línea 4, ") || strings.Contains(got, "undefined") || strings.HasSuffix(got, ".") || strings.Contains(got, "(línea") {
		t.Errorf("%q", got)
	}
	if got := explicarMinuscula("Go es"); got != "Go es" {
		t.Errorf("Go: %q", got)
	}
	if got := explicarImports([]string{"strings", "os"}); got != "`strings` y `os`" {
		t.Errorf("imports: %q", got)
	}
}

func TestExplicarLeerPeticion(t *testing.T) {
	casos := []struct {
		texto       string
		comentarios bool
		detalle     int
	}{
		{"ponle comentarios a este código", true, 0},
		{"coméntalo, por favor", true, 0},
		{"añádele un comentario", true, 0},
		{"explícalo con comentarios", true, 0},
		{"explícalo con detalle", false, 2},
		{"explícame este bucle paso a paso", false, 2},
		{"explícalo más simple", false, 1},
		{"explícame este código", false, 0},
		{"este código tiene comentarios, explícamelo", false, 0},
	}
	for _, c := range casos {
		k := &tarea{p: nucleo.Peticion{Texto: c.texto}}
		explicarLeerPeticion(k)
		if k.comentarios != c.comentarios || k.detalle != c.detalle {
			t.Errorf("%q: comentarios %v detalle %d", c.texto, k.comentarios, k.detalle)
		}
	}
	// a follow-up already decided: the text does not undo it
	k := &tarea{p: nucleo.Peticion{Texto: "explícalo más simple"}, detalle: 2}
	explicarLeerPeticion(k)
	if k.detalle != 2 {
		t.Errorf("respeta el seguimiento: %d", k.detalle)
	}
}
