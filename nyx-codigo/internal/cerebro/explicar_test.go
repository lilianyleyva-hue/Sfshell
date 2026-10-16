package cerebro

import (
	"context"
	"os/exec"
	"strings"
	"testing"

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
