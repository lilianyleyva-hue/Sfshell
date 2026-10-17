package cerebro

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
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
	case strings.HasPrefix(r.Texto, "Lo hago más rápido"):
		if !strings.Contains(todo, "Lo medí con n = ") {
			t.Errorf("dice «más rápido» sin medirlo:\n%s\n%s", todo, titulos(tr))
		}
	case strings.HasPrefix(r.Texto, "Creo que"), strings.HasPrefix(r.Texto, "Este cambio da lo mismo"):
	default:
		t.Errorf("texto: %q\n%s", r.Texto, titulos(tr))
	}
}
