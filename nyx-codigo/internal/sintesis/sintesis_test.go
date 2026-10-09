package sintesis

import (
	"context"
	"errors"
	"go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	"go/types"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

func TestSintetizable(t *testing.T) {
	casos := map[string]bool{
		"func F(nums []int) int":              true,
		"func F(s string, n int) []string":    true,
		"func F(m [][]int) []int":             true,
		"func F(s string) map[string]int":     true,
		"func F(xs []float64) float64":        true,
		"func F(m map[string]int) int":        false,
		"func F(xs []int64) int":              false,
		"func F() int":                        false,
		"func F(a, b, c, d, e int) int":       false,
		"func F(xs [3]int) int":               false,
		"func F(m map[string][]int) []string": false,
	}
	for s, quiero := range casos {
		if got := Sintetizable(debeFirma(t, s)); got != quiero {
			t.Errorf("Sintetizable(%s) = %v, quiero %v", s, got, quiero)
		}
	}
}

func TestPrimitivasCompletas(t *testing.T) {
	r := Base()
	nombres := map[string]bool{}
	for _, p := range r.Todas() {
		nombres[p.Nombre] = true
		if p.Eval == nil {
			t.Errorf("%s no tiene Eval", p.Nombre)
		}
		if p.Frase == "" {
			t.Errorf("%s no tiene frase", p.Nombre)
		}
		for _, c := range p.Conceptos {
			if !nucleo.EsConcepto(c) {
				t.Errorf("%s usa el concepto desconocido %q", p.Nombre, c)
			}
		}
		if idTipo(p.Res) < 0 {
			t.Errorf("%s da un tipo fuera del DSL: %s", p.Nombre, p.Res.Go())
		}
	}
	if len(nombres) < 95 {
		t.Errorf("hay %d primitivas distintas; deben ser unas 100", len(nombres))
	}
	for _, alias := range []string{"filtraS", "mapeaS", "contarS", "ordenarS", "unicosS"} {
		if r.Buscar(alias) == nil {
			t.Errorf("falta el alias %s", alias)
		}
	}
	// Base returns independent registries
	r.Agregar(&Primitiva{Nombre: "soloAqui", Args: ts(tI), Res: tI, Costo: 6, Eval: func(a []V, _ []Funcion) (V, error) { return a[0], nil }})
	if Base().Buscar("soloAqui") != nil {
		t.Error("Base() comparte el registro entre llamadas")
	}
}

func TestEvaluarYParse(t *testing.T) {
	r := Base()
	casos := []struct {
		firma, prog string
		ent         []V
		quiero      V
	}{
		{"func F(nums []int) int", "(suma (filtra esPar nums))", []V{[]V{1, 2, 3, 4}}, 6},
		{"func F(nums []int) int", "(suma (filtra (λ x (esPar x)) nums))", []V{[]V{}}, 0},
		{"func F(s string) int", "(contar esVocal (runas s))", []V{"árbol"}, 2},
		{"func F(s string) string", "(masLarga (palabras s))", []V{"go es genial"}, "genial"},
		{"func F(n int) int", "(producto (rango 1 (+ n 1)))", []V{5}, 120},
		{"func F(nums []int) int", "(pliega (λ (a b) (+ a (* b b))) 0 nums)", []V{[]V{1, 2, 3}}, 14},
		{"func F(nums []int, k int) []int", "(filtra (λ x (> x k)) nums)", []V{[]V{1, 5, 9}, 4}, []V{5, 9}},
		{"func F(s string) []string", "(filtraS (λ p (> (largoS p) 3)) (palabras s))", []V{"el gato come"}, []V{"gato", "come"}},
		{"func F(xs []int) int", "(si (== (largo xs) 0) 0 (maxL xs))", []V{[]V{}}, 0},
		{"func F(s string) map[string]int", "(frecuencias (palabras s))", []V{"a b a"}, nucleo.Mapa{{K: "a", V: 2}, {K: "b", V: 1}}},
	}
	for _, c := range casos {
		f := debeFirma(t, c.firma)
		e := debeParse(t, r, f, c.prog)
		v, err := Evaluar(e, c.ent)
		if err != nil || !nucleo.Igual(v, c.quiero) {
			t.Errorf("%s con %v = %v, %v; quiero %v", c.prog, c.ent, v, err, c.quiero)
		}
		// String round-trips through Parse
		e2, err := Parse(e.String(), r, f)
		if err != nil || e2.String() != e.String() {
			t.Errorf("Parse(String()) de %s: %v, %v", e, e2, err)
		}
	}
	// ⊥ is ErrIndefinido
	f := debeFirma(t, "func F(xs []int) int")
	if _, err := Evaluar(debeParse(t, r, f, "(maxL xs)"), []V{[]V{}}); !errors.Is(err, ErrIndefinido) {
		t.Errorf("maxL de [] debería ser ⊥, da %v", err)
	}
	if _, err := Evaluar(debeParse(t, r, f, "(pot (largo xs) 100)"), []V{[]V{1, 2}}); !errors.Is(err, ErrIndefinido) {
		t.Errorf("2^100 debería ser ⊥, da %v", err)
	}
	// typed errors
	for _, malo := range []string{"(suma xs xs)", "(maxL s)", "(mayus xs)", "(suma (filtra esVocal xs))", "(", "(nada xs)"} {
		if _, err := Parse(malo, r, f); err == nil {
			t.Errorf("Parse(%q) debería fallar", malo)
		}
	}
	if Tamano(debeParse(t, r, f, "(suma (filtra esPar xs))")) != 5 {
		t.Error("Tamano de (suma (filtra esPar xs)) debería ser 5")
	}
	o := Oraculo(debeParse(t, r, f, "(suma xs)"))
	if out, err := o([]V{[]V{1, 2}}); err != nil || len(out) != 1 || out[0] != 3 {
		t.Errorf("Oraculo: %v, %v", out, err)
	}
}

func TestDescribir(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func SumaPares(nums []int) int")
	if got := Describir(debeParse(t, r, f, "(suma (filtra esPar nums))"), f); got != "la suma de los números pares de nums" {
		t.Errorf("Describir = %q", got)
	}
	casos := map[string]string{
		"func F(s string) int":      "(contar esVocal (runas s))",
		"func F(s string) []string": "(filtra (λ p (> (largoS p) 3)) (palabras s))",
		"func F(xs []int) []int":    "(filtra (λ x (> x 5)) xs)",
	}
	quiero := map[string]string{
		"func F(s string) int":      "cuántas vocales hay en s",
		"func F(s string) []string": "las palabras de más de 3 letras de s",
		"func F(xs []int) []int":    "los números mayores que 5 de xs",
	}
	for firma, prog := range casos {
		f := debeFirma(t, firma)
		if got := Describir(debeParse(t, r, f, prog), f); got != quiero[firma] {
			t.Errorf("Describir(%s) = %q, quiero %q", prog, got, quiero[firma])
		}
	}
}

func casoEj(ent []V, sal V) nucleo.Caso {
	return nucleo.Caso{Entradas: ent, Esperado: []V{sal}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario}
}

func ejemplosSumaPares() []nucleo.Caso {
	return []nucleo.Caso{
		casoEj([]V{[]V{1, 2, 3, 4}}, 6), casoEj([]V{[]V{}}, 0), casoEj([]V{[]V{5, 7}}, 0),
		casoEj([]V{[]V{2, 8, 3}}, 10), casoEj([]V{[]V{-2, 3}}, -2),
	}
}

func TestBancoSinDuplicados(t *testing.T) {
	f := debeFirma(t, "func F(nums []int) int")
	esp := Especificacion{Firma: f, Ejemplos: []nucleo.Caso{casoEj([]V{[]V{1, 2, 3}}, 99), casoEj([]V{[]V{4}}, -7)}}
	s, err := nuevaSesion(context.Background(), esp, Base(), Opciones{}.normalizar(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.construirPools(time.Now().Add(2 * time.Second))
	s.prepararMotor()
	s.mo.correr(30)
	if s.mo.distintos < 1000 {
		t.Fatalf("el banco tiene solo %d entradas", s.mo.distintos)
	}
	for tipo := 0; tipo < numTipos; tipo++ {
		vistos := map[string]int32{}
		for c := 0; c < len(s.mo.nivel[tipo]) && c <= 30; c++ {
			for _, id := range s.mo.nivel[tipo][c] {
				e := &s.mo.entradas[id]
				var sb strings.Builder
				for _, v := range e.vec {
					if esFallo(v) {
						sb.WriteString("⊥|")
					} else {
						sb.WriteString(nucleo.Clave(v) + "|")
					}
				}
				k := sb.String()
				if otro, ok := vistos[k]; ok && e.hoja == nil {
					t.Fatalf("dos entradas con la misma salida: %s y %s", s.mo.expr(otro), s.mo.expr(id))
				}
				vistos[k] = id
			}
		}
	}
	// pools are OE-reduced too
	for _, pl := range s.pools {
		if len(pl.cuerpos) > maxPorPool {
			t.Errorf("el pool %s tiene %d funciones", pl.lt.clave(), len(pl.cuerpos))
		}
	}
}

func TestMismaSemilla(t *testing.T) {
	f := debeFirma(t, "func SumaPares(nums []int) int")
	esp := Especificacion{Firma: f, Ejemplos: ejemplosSumaPares()}
	var textos [2]string
	for i := range textos {
		res, err := Sintetizar(context.Background(), esp, Base(), Opciones{Semilla: 1, Limite: 6 * time.Second}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var partes []string
		for _, s := range res.Soluciones {
			partes = append(partes, s.Expr.String())
		}
		textos[i] = strings.Join(partes, "; ")
	}
	if textos[0] != textos[1] || textos[0] == "" {
		t.Errorf("la misma semilla da resultados distintos:\n%s\n%s", textos[0], textos[1])
	}
	if !strings.HasPrefix(textos[0], "(suma (filtra (λ x (esPar x)) nums))") {
		t.Errorf("la mejor solución debería ser la suma de los pares: %s", textos[0])
	}
}

func TestLimiteTiempo(t *testing.T) {
	f := debeFirma(t, "func F(nums []int, s string) int")
	esp := Especificacion{Firma: f, Ejemplos: []nucleo.Caso{
		casoEj([]V{[]V{1, 2, 3}, "hola"}, 98761), casoEj([]V{[]V{4}, "a b"}, -7123), casoEj([]V{[]V{}, ""}, 5555),
		casoEj([]V{[]V{9, 9}, "xyz"}, 31337),
	}}
	inicio := time.Now()
	res, err := Sintetizar(context.Background(), esp, Base(), Opciones{Limite: 100 * time.Millisecond}, nil)
	dur := time.Since(inicio)
	if err != nil {
		t.Fatal(err)
	}
	if res.Motivo != "tiempo" {
		t.Errorf("Motivo = %q, quiero «tiempo»", res.Motivo)
	}
	if lim := time.Duration(float64(200*time.Millisecond) * factorCarrera); dur > lim {
		t.Errorf("con 100 ms de límite tardó %v", dur)
	}
}

func TestCancelar(t *testing.T) {
	f := debeFirma(t, "func F(nums []int) int")
	esp := Especificacion{Firma: f, Ejemplos: []nucleo.Caso{casoEj([]V{[]V{1, 2, 3}}, 98761), casoEj([]V{[]V{4}}, -7123)}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	inicio := time.Now()
	res, _ := Sintetizar(ctx, esp, Base(), Opciones{}, nil)
	if d := time.Since(inicio); d > time.Duration(float64(time.Second)*factorCarrera) {
		t.Errorf("cancelar tardó %v", d)
	}
	if len(res.Soluciones) != 0 {
		t.Errorf("no debería encontrar nada: %v", res.Soluciones[0].Expr)
	}
}

func TestSinEjemplosYTiposNoSoportados(t *testing.T) {
	f := debeFirma(t, "func F(m map[string]int) int")
	res, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: []nucleo.Caso{casoEj([]V{nucleo.Mapa{}}, 0)}}, Base(), Opciones{}, nil)
	if !errors.Is(err, nucleo.ErrNoSoportado) || res.Motivo != "tipos_no_soportados" {
		t.Errorf("tipos no soportados: %v %q", err, res.Motivo)
	}
	if _, err := Sintetizar(context.Background(), Especificacion{Firma: debeFirma(t, "func F(n int) int")}, Base(), Opciones{}, nil); err == nil {
		t.Error("sin ejemplos debería dar error")
	}
}

func TestDistinguir(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func F(x int) int")
	sols := []Solucion{{Expr: debeParse(t, r, f, "(* x x)")}, {Expr: debeParse(t, r, f, "(* x (abs x))")}}
	sondas := [][]V{{0}, {1}, {2}, {-2}, {5}}
	ent, sal, grupos, ok := Distinguir(sols, sondas)
	if !ok || len(ent) != 1 || ent[0] != -2 {
		t.Fatalf("Distinguir = %v %v %v %v; quiero [-2]", ent, sal, grupos, ok)
	}
	if len(sal) != 2 || sal[0] != 4 || sal[1] != -4 || len(grupos) != 2 {
		t.Errorf("salidas %v grupos %v", sal, grupos)
	}
	if _, _, _, ok := Distinguir(sols, [][]V{{0}, {3}}); ok {
		t.Error("no hay sonda que las distinga y dio ok")
	}
}

func TestUnificarSinAbs(t *testing.T) {
	// only arithmetic, comparisons and si: |n| needs a decision tree
	r := nuevoRegistro()
	quedan := map[string]bool{"+": true, "-": true, "*": true, "neg": true, "<": true, ">": true, "==": true,
		"esNegativo": true, "esPositivo": true, "esPar": true, "si": true}
	for _, p := range Base().Todas() {
		if quedan[p.Nombre] {
			r.Agregar(p)
		}
	}
	f := debeFirma(t, "func Absoluto(n int) int")
	esp := Especificacion{Firma: f, Ejemplos: []nucleo.Caso{
		casoEj([]V{-3}, 3), casoEj([]V{4}, 4), casoEj([]V{0}, 0), casoEj([]V{-10}, 10), casoEj([]V{7}, 7), casoEj([]V{-1}, 1),
	}}
	res, err := Sintetizar(context.Background(), esp, r, Opciones{Semilla: 1}, nil)
	if err != nil || len(res.Soluciones) == 0 {
		t.Fatalf("no encontró el valor absoluto: %v %v", err, res.Motivo)
	}
	e := res.Soluciones[0].Expr
	for _, n := range []int{-50, 50, -7, 0} {
		v, err := Evaluar(e, []V{n})
		if err != nil || v != max(n, -n) {
			t.Errorf("%s con %d = %v %v", e, n, v, err)
		}
	}
	if !strings.Contains(e.String(), "(si ") {
		t.Errorf("sin abs debería usar un condicional (Unificar): %s", e)
	}
}

func TestUnificarCondicional(t *testing.T) {
	// a target that needs a split: x*2 for evens, x+1 for odds
	r := Base()
	f := debeFirma(t, "func F(n int) int")
	var ejs []nucleo.Caso
	for _, n := range []int{2, 3, 4, 5, 10, 7, -2, -3} {
		y := n + 1
		if n%2 == 0 {
			y = n * 2
		}
		ejs = append(ejs, casoEj([]V{n}, y))
	}
	res, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: ejs}, r, Opciones{Semilla: 1}, nil)
	if err != nil || len(res.Soluciones) == 0 {
		t.Fatalf("no encontró el condicional: %v %v", err, res.Motivo)
	}
	e := res.Soluciones[0].Expr
	if ok, por := cumple(e, ejs); !ok {
		t.Fatalf("%s: %s", e, por)
	}
	if v, _ := Evaluar(e, []V{20}); v != 40 {
		t.Errorf("%s con 20 = %v", e, v)
	}
}

func TestEsqueletos(t *testing.T) {
	r := Base()
	casos := []struct {
		firma  string
		marco  nucleo.Marco
		quiero string
	}{
		{"func F(nums []int) int", nucleo.Marco{Accion: "sumar", Objeto: "numeros", Mods: []nucleo.Modificador{{Concepto: "par"}}}, "(suma (filtra ? nums))"},
		{"func F(s string) int", nucleo.Marco{Accion: "contar", Objeto: "letras", Elemento: "vocal"}, "(contar ? (runas s))"},
		{"func F(s string) string", nucleo.Marco{Accion: "mas_largo", Objeto: "palabras", Elemento: "palabra"}, "(masLarga (palabras s))"},
		{"func F(s string) []string", nucleo.Marco{Accion: "filtrar", Objeto: "palabras", Elemento: "palabra", Mods: []nucleo.Modificador{{Concepto: "longitud_mayor", Numeros: []float64{3}}}}, "(filtra ? (palabras s))"},
	}
	for _, c := range casos {
		f := debeFirma(t, c.firma)
		m := c.marco
		var textos []string
		for _, e := range Esqueletos(&m, f, r) {
			textos = append(textos, e.String())
		}
		hay := false
		for _, x := range textos {
			hay = hay || x == c.quiero
		}
		if !hay {
			t.Errorf("Esqueletos(%s) = %v; falta %s", m.Accion, textos, c.quiero)
		}
	}
}

func TestCompletarEsqueleto(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func SumaPares(nums []int) int")
	sk := debeParse(t, r, f, "(suma (filtra ? nums))")
	sols := CompletarEsqueleto(context.Background(), sk, Especificacion{Firma: f, Ejemplos: ejemplosSumaPares()}, r, Opciones{})
	if len(sols) == 0 || sols[0].Expr.String() != "(suma (filtra (λ x (esPar x)) nums))" {
		t.Fatalf("CompletarEsqueleto = %v", sols)
	}
}

func TestDesdeConceptos(t *testing.T) {
	r := Base()
	casos := []struct {
		firma  string
		marco  nucleo.Marco
		quiero string
	}{
		{"func SumaPares(nums []int) int", nucleo.Marco{Accion: "sumar", Objeto: "numeros", Mods: []nucleo.Modificador{{Concepto: "par"}}}, "(suma (filtra (λ x (esPar x)) nums))"},
		{"func F(nums []int) []int", nucleo.Marco{Accion: "filtrar", Objeto: "numeros", Mods: []nucleo.Modificador{{Concepto: "mayor_que", Numeros: []float64{5}}}}, "(filtra (λ x (> x 5)) nums)"},
		{"func F(s string) int", nucleo.Marco{Accion: "contar", Objeto: "letras", Elemento: "vocal"}, "(contar (λ x (esVocal x)) (runas s))"},
		{"func F(s string) string", nucleo.Marco{Accion: "mayusculas", Objeto: "texto"}, "(mayus s)"},
	}
	for _, c := range casos {
		m := c.marco
		sols := DesdeConceptos(context.Background(), &m, debeFirma(t, c.firma), r, 3)
		if len(sols) == 0 || sols[0].Expr.String() != c.quiero {
			var textos []string
			for _, s := range sols {
				textos = append(textos, s.Expr.String())
			}
			t.Errorf("DesdeConceptos(%s) = %v; quiero primero %s", m.Accion, textos, c.quiero)
		}
	}
}

func TestPriors(t *testing.T) {
	cont := nucleotest.NuevoContador()
	for i := 0; i < 30; i++ {
		cont.Exito("componente:esImpar", true)
	}
	prims := Base().Todas()
	sin := calcularCostos(prims, Especificacion{}, Opciones{})
	con := calcularCostos(prims, Especificacion{}, Opciones{Priors: cont})
	var impar *Primitiva
	for _, p := range prims {
		if p.Nombre == "esImpar" {
			impar = p
		}
	}
	if con[impar] >= sin[impar] {
		t.Errorf("los priors no abaratan esImpar: %d → %d", sin[impar], con[impar])
	}
	conc := calcularCostos(prims, Especificacion{Conceptos: map[string]float64{"impar": 1}}, Opciones{})
	if conc[impar] >= sin[impar] {
		t.Errorf("el concepto «impar» no abarata esImpar: %d → %d", sin[impar], conc[impar])
	}
}

// ---- codegen ----

func TestAGoFusion(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func SumaPares(nums []int) int")
	src, err := AGo(debeParse(t, r, f, "(suma (filtra esPar nums))"), f)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(src, "for "); n != 1 {
		t.Errorf("debería haber un solo bucle y hay %d:\n%s", n, src)
	}
	for _, trozo := range []string{"package solucion", "func SumaPares(nums []int) int {", "total := 0", "for _, x := range nums {", "if x%2 == 0 {", "total += x", "return total"} {
		if !strings.Contains(src, trozo) {
			t.Errorf("falta %q en:\n%s", trozo, src)
		}
	}
	// guards on partial primitives
	f2 := debeFirma(t, "func Mayor(nums []int) int")
	src, err = AGo(debeParse(t, r, f2, "(maxL nums)"), f2)
	if err != nil || !strings.Contains(src, "// con la lista vacía devuelvo 0") {
		t.Errorf("maxL sin guarda: %v\n%s", err, src)
	}
	// chains of filters and maps stay one loop
	f3 := debeFirma(t, "func F(nums []int) int")
	src, err = AGo(debeParse(t, r, f3, "(suma (mapea (λ x (* x x)) (filtra esImpar (filtra (λ x (> x 2)) nums))))"), f3)
	if err != nil || strings.Count(src, "for ") != 1 {
		t.Errorf("cadena no fusionada: %v\n%s", err, src)
	}
	if _, err := AGo(debeParse(t, r, f3, "(suma (filtra ? nums))"), f3); err == nil {
		t.Error("AGo con huecos debería fallar")
	}
}

// expresionesReferencia are programs used by the codegen tests (go/types here, the real toolchain in
// sintesis_test).
var expresionesReferencia = [][2]string{
	{"func SumaPares(nums []int) int", "(suma (filtra esPar nums))"},
	{"func Mayor(nums []int) int", "(maxL nums)"},
	{"func MayorFiltrado(nums []int) int", "(maxL (filtra (λ x (> x 2)) nums))"},
	{"func Vocales(s string) int", "(contar esVocal (runas s))"},
	{"func MasLarga(s string) string", "(masLarga (palabras s))"},
	{"func TodosPositivos(nums []int) bool", "(todos esPositivo nums)"},
	{"func CuadradosImpares(nums []int) []int", "(mapea (λ x (* x x)) (filtra esImpar nums))"},
	{"func Factorial(n int) int", "(producto (rango 1 (+ n 1)))"},
	{"func Absoluto(n int) int", "(si (< n 0) (neg n) n)"},
	{"func Media(nums []int) float64", "(promedio nums)"},
	{"func InvertirPalabras(s string) string", "(unir (invertir (palabras s)) \" \")"},
	{"func SegundoMayor(nums []int) int", "(elemento (ordenarDesc nums) 1)"},
	{"func SumaCuadrados(nums []int) int", "(pliega (λ (a b) (+ a (* b b))) 0 nums)"},
	{"func Frecuencias(s string) map[string]int", "(frecuencias (palabras s))"},
	{"func Division(a, b int) int", "(/ a b)"},
	{"func MasUno(s string) int", "(+ (atoi s) 1)"},
	{"func ParYPequenos(nums []int) bool", "(y (alguno esPar nums) (todos (λ x (< x 10)) nums))"},
	{"func SumaDeDigitos(nums []int) int", "(suma (mapea (λ x (suma (digitos x))) nums))"},
	{"func PalabrasLargas(s string) []string", "(filtra (λ p (> (largoS p) 3)) (palabras s))"},
	{"func Titulo(s string) string", "(titulo s)"},
	{"func SinVocales(s string) string", "(deRunas (filtra (λ r (no (esVocal r))) (runas s)))"},
	{"func PrimosHasta(n int) []int", "(filtra esPrimo (rango 2 (+ n 1)))"},
	{"func MediaPares(xs []int) float64", "(promedio (filtra esPar xs))"},
	{"func Clasifica(n int) string", "(si (esPar n) \"par\" (si (> n 10) \"grande\" \"impar\"))"},
	{"func Repite(s string, n int) string", "(repetir s n)"},
	{"func Ultima(s string) string", "(ultimo (palabras s))"},
	{"func MenorPositivo(nums []int) int", "(minL (filtra esPositivo nums))"},
	{"func Rango(n int) []int", "(rango 1 (+ n 1))"},
	{"func Ordenada(nums []int) bool", "(== nums (ordenar nums))"},
	{"func Comas(xs []int) string", "(unir (mapea (λ x (itoa x)) xs) \",\")"},
	{"func Cuenta(s string, c rune) int", "(contarSub s (runaTexto c))"},
	{"func Hay(nums []int, k int) bool", "(alguno (λ x (> x k)) nums)"},
	{"func SumaMayores(nums []int, k int) int", "(suma (filtra (λ x (> x k)) nums))"},
	{"func Mezcla(nums []int) int", "(si (o (esCero (largo nums)) (> (suma nums) 100)) 0 (suma (mapea (λ x (/ 100 x)) (filtra (λ x (!= x 0)) nums))))"},
	{"func Mayus(s string) string", "(deRunas (mapea aMayus (runas s)))"},
	{"func Media2(xs []float64) float64", "(promedio xs)"},
	{"func Traspuesta(m [][]int) [][]int", "(transpuesta m)"},
	{"func Largos(s string) []int", "(largos (palabras s))"},
	{"func Cifras(n int) int", "(numDigitos n)"},
	{"func Pliega(xs []int) int", "(pliega (λ (a b) (max2 a b)) -1 xs)"},
	{"func Elem(xs []int, i int) int", "(elemento xs i)"},
	{"func Resto(a, b int) bool", "(divisible a b)"},
	{"func Rangos(xs []int) []int", "(mapea (λ x (contar (λ y (> y x)) xs)) xs)"},
	{"func DobleSigno(n int) int", "(* 2 (si (esPar n) n (neg n)))"},
	{"func Sumas(m [][]int) int", "(suma (sumaFilas m))"},
	{"func SinRepetir(s string) string", "(deRunas (unicos (runas s)))"},
	{"func Mitades(xs []int) []int", "(mapea (λ x (/ x 2)) (filtra (λ x (!= x 0)) xs))"},
	{"func Dividir(xs []int, d int) []int", "(mapea (λ x (/ x d)) xs)"},
	{"func PrimeraLarga(s string) string", "(primero (filtra (λ p (> (largoS p) 3)) (palabras s)))"},
	{"func CuentaSi(xs []int) int", "(contar (λ x (y (esPar x) (> x 3))) (mapea (λ x (+ x 1)) xs))"},
	{"func Reemplaza(s string) string", "(reemplazar s \"a\" \"o\")"},
	{"func SumaTextos(xs []string) int", "(suma (mapea (λ p (atoi p)) xs))"},
	{"func Mayores(xs []int) []int", "(filtra (λ x (> (* x (largo xs)) (suma xs))) xs)"},
}

func comprobarTipos(t *testing.T, src string) {
	t.Helper()
	fset := token.NewFileSet()
	af, err := goparser.ParseFile(fset, "solucion.go", src, goparser.ParseComments)
	if err != nil {
		t.Fatalf("no se puede leer:\n%s\n%v", src, err)
	}
	conf := types.Config{Importer: importadorGo()}
	if _, err := conf.Check("solucion", fset, []*ast.File{af}, nil); err != nil {
		t.Errorf("no compila:\n%s\n%v", src, err)
	}
}

var (
	impUna sync.Once
	importador types.Importer
)

func importadorGo() types.Importer {
	impUna.Do(func() { importador = importer.ForCompiler(token.NewFileSet(), "source", nil) })
	return importador
}

func TestAGoTipos(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no hay Go instalado")
	}
	r := Base()
	for _, c := range expresionesReferencia {
		f := debeFirma(t, c[0])
		src, err := AGo(debeParse(t, r, f, c[1]), f)
		if err != nil {
			t.Errorf("%s: %v", c[1], err)
			continue
		}
		comprobarTipos(t, src)
	}
	// every primitive of the registry through a minimal program
	for _, p := range r.Todas() {
		if p.numLambdas() > 0 {
			continue
		}
		f := nucleo.Firma{Nombre: "Prueba", Res: []nucleo.Tipo{p.Res}}
		var hijos []*Expr
		for i, a := range p.Args {
			n := string(rune('a' + i))
			f.Params = append(f.Params, nucleo.Param{Nombre: n, Tipo: a})
			hijos = append(hijos, nuevaVar(i, a, n))
		}
		if len(f.Params) == 0 {
			continue
		}
		e := nuevaOp(p, hijos, nil)
		src, err := AGo(e, f)
		if err != nil {
			t.Errorf("%s: %v", e, err)
			continue
		}
		comprobarTipos(t, src)
	}
}

// ---- library learning ----

func TestAprenderAbarata(t *testing.T) {
	f := debeFirma(t, "func DoblePares(nums []int) int")
	var ejs []nucleo.Caso
	for _, c := range ejemplosSumaPares() {
		ejs = append(ejs, casoEj(c.Entradas, 2*c.Esperado[0].(int)))
	}
	esp := Especificacion{Firma: f, Ejemplos: ejs}
	antes, err := Sintetizar(context.Background(), esp, Base(), Opciones{Semilla: 1}, nil)
	if err != nil || len(antes.Soluciones) == 0 {
		t.Fatalf("sin biblioteca no encontró el doble: %v %s", err, antes.Motivo)
	}
	b := NuevaBiblioteca(nil)
	fsp := debeFirma(t, "func SumaPares(nums []int) int")
	p, err := b.Aprender("SumaPares", "la suma de los números pares de nums", debeParse(t, b.Registro(), fsp, "(suma (filtra esPar nums))"), fsp)
	if err != nil {
		t.Fatal(err)
	}
	if p.Frase != "la suma de los números pares de {0}" || p.Cuerpo == nil {
		t.Errorf("componente aprendido mal: %q", p.Frase)
	}
	despues, err := Sintetizar(context.Background(), esp, b.Registro(), Opciones{Semilla: 1}, nil)
	if err != nil || len(despues.Soluciones) == 0 {
		t.Fatalf("con biblioteca no encontró el doble: %v %s", err, despues.Motivo)
	}
	sa, sd := antes.Soluciones[0], despues.Soluciones[0]
	if !strings.Contains(sd.Expr.String(), "SumaPares") {
		t.Errorf("no usó lo aprendido: %s", sd.Expr)
	}
	if sd.Costo >= sa.Costo || despues.Explorados >= antes.Explorados {
		t.Errorf("aprender no ayudó: costo %d → %d, explorados %d → %d", sa.Costo, sd.Costo, antes.Explorados, despues.Explorados)
	}
	// the learned component generates Go as a helper call
	src, err := AGo(sd.Expr, f)
	if err != nil || !strings.Contains(src, "func sumaPares(nums []int) int") {
		t.Errorf("AGo con componente aprendido: %v\n%s", err, src)
	}
	if _, err := exec.LookPath("go"); err == nil {
		comprobarTipos(t, src)
	}
	if got := Describir(sd.Expr, f); !strings.Contains(got, "la suma de los números pares de nums") {
		t.Errorf("Describir = %q", got)
	}
	// errors
	if _, err := b.Aprender("suma", "", debeParse(t, b.Registro(), fsp, "(suma nums)"), fsp); err == nil {
		t.Error("no debería poder pisar una primitiva de base")
	}
	if _, err := b.Aprender("mal nombre", "", debeParse(t, b.Registro(), fsp, "(suma nums)"), fsp); err == nil {
		t.Error("nombre con espacios aceptado")
	}
	if !b.Olvidar("SumaPares") || b.Registro().Buscar("SumaPares") != nil {
		t.Error("Olvidar no quitó SumaPares")
	}
}

func TestComprimir(t *testing.T) {
	b := NuevaBiblioteca(nil)
	f := debeFirma(t, "func F(nums []int) int")
	for nombre, prog := range map[string]string{
		"SumaPares":  "(suma (filtra esPar nums))",
		"CuentaPares": "(largo (filtra esPar nums))",
		"MayorPar":   "(maxL (filtra esPar nums))",
	} {
		if _, err := b.Aprender(nombre, "", debeParse(t, b.Registro(), f, prog), f); err != nil {
			t.Fatal(err)
		}
	}
	inv := b.Comprimir()
	if len(inv) != 1 {
		var nombres []string
		for _, p := range inv {
			nombres = append(nombres, p.Nombre+"="+p.Cuerpo.String())
		}
		t.Fatalf("inventos = %v; quiero exactamente filtra(esPar, …)", nombres)
	}
	p := inv[0]
	if p.Cuerpo.String() != "(filtra (λ x (esPar x)) xs)" || len(p.Args) != 1 || !mismoTipo(p.Args[0], tLI) {
		t.Errorf("invento = %s %v", p.Cuerpo, p.Args)
	}
	if p.Frase != "los números pares de {0}" {
		t.Errorf("frase del invento = %q", p.Frase)
	}
	if b.Registro().Buscar(p.Nombre) == nil {
		t.Error("el invento no está en el registro")
	}
	if again := b.Comprimir(); len(again) != 0 {
		t.Errorf("volver a comprimir inventa otra vez: %v", again)
	}
	// the invention is usable by the synthesizer and in Go
	g := debeFirma(t, "func Pares(nums []int) []int")
	e := debeParse(t, b.Registro(), g, "("+p.Nombre+" nums)")
	if v, err := Evaluar(e, []V{[]V{1, 2, 4}}); err != nil || !nucleo.Igual(v, []V{2, 4}) {
		t.Errorf("Evaluar invento = %v %v", v, err)
	}
	if src, err := AGo(e, g); err != nil || !strings.Contains(src, "func filtraEsPar(") {
		t.Errorf("AGo invento: %v\n%s", err, src)
	}
}

func TestCargar(t *testing.T) {
	b := NuevaBiblioteca(nil)
	f := debeFirma(t, "func DoblePares(nums []int) int")
	fs := []nucleo.FuncionAprendida{
		{Nombre: "DoblePares", Firma: f, DSL: "(* (SumaPares nums) 2)", Descripcion: "el doble de la suma de los pares de nums"},
		{Nombre: "SumaPares", Firma: debeFirma(t, "func SumaPares(nums []int) int"), DSL: "(suma (filtra (λ x (esPar x)) nums))"},
		{Nombre: "SinDSL", Firma: f},
		{Nombre: "Roto", Firma: f, DSL: "(nada nums)"},
	}
	n, errs := b.Cargar(fs)
	if n != 2 || len(errs) != 1 {
		t.Fatalf("Cargar = %d %v", n, errs)
	}
	e := debeParse(t, b.Registro(), f, "(DoblePares nums)")
	if v, err := Evaluar(e, []V{[]V{1, 2, 3, 4}}); err != nil || v != 12 {
		t.Errorf("DoblePares = %v %v", v, err)
	}
	src, err := AGo(e, nucleo.Firma{Nombre: "Usa", Params: f.Params, Res: f.Res})
	if err != nil || !strings.Contains(src, "func doblePares(") || !strings.Contains(src, "func sumaPares(") {
		t.Errorf("AGo anidado: %v\n%s", err, src)
	}
}

func TestComprimirCadaCinco(t *testing.T) {
	b := NuevaBiblioteca(nil)
	f := debeFirma(t, "func F(s string) int")
	progs := []string{
		"(largo (filtra (λ p (> (largoS p) 3)) (palabras s)))",
		"(suma (largos (filtra (λ p (> (largoS p) 3)) (palabras s))))",
		"(largoS (unir (filtra (λ p (> (largoS p) 3)) (palabras s)) \"\"))",
		"(largoS s)",
		"(largo (palabras s))",
	}
	for i, p := range progs {
		if _, err := b.Aprender("Prog"+string(rune('A'+i)), "", debeParse(t, b.Registro(), f, p), f); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.Inventos()) == 0 {
		t.Fatal("tras 5 aprendidas debería haber comprimido")
	}
	for _, p := range b.Inventos() {
		t.Logf("invento %s = %s (%s)", p.Nombre, p.Cuerpo, p.Frase)
	}
}

func TestExcluirOtraForma(t *testing.T) {
	f := debeFirma(t, "func SumaPares(nums []int) int")
	esp := Especificacion{Firma: f, Ejemplos: ejemplosSumaPares()}
	primera, err := Sintetizar(context.Background(), esp, Base(), Opciones{Semilla: 1}, nil)
	if err != nil || len(primera.Soluciones) == 0 {
		t.Fatal(err)
	}
	esp.Excluir = []string{primera.Soluciones[0].Expr.String()}
	otra, err := Sintetizar(context.Background(), esp, Base(), Opciones{Semilla: 1}, nil)
	if err != nil || len(otra.Soluciones) == 0 {
		t.Fatalf("otra forma: %v %s", err, otra.Motivo)
	}
	for _, s := range otra.Soluciones {
		if s.Expr.String() == esp.Excluir[0] {
			t.Errorf("devolvió el programa excluido %s", s.Expr)
		}
	}
	if ok, por := cumple(otra.Soluciones[0].Expr, esp.Ejemplos); !ok {
		t.Errorf("la otra forma no cumple: %s", por)
	}
}

func TestTrazaYProgreso(t *testing.T) {
	tr := nucleo.NuevaTraza(nil, nil)
	f := debeFirma(t, "func SumaPares(nums []int) int")
	if _, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: ejemplosSumaPares()}, Base(), Opciones{}, tr.Raiz()); err != nil {
		t.Fatal(err)
	}
	pasos := tr.Pasos()
	if len(pasos) == 0 || !strings.Contains(pasos[0].Titulo, "Encontré") || pasos[0].Estado != nucleo.EstadoBien {
		t.Errorf("pasos = %+v", pasos)
	}
}

func TestConcurrencia(t *testing.T) {
	b := NuevaBiblioteca(nil)
	f := debeFirma(t, "func SumaPares(nums []int) int")
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: ejemplosSumaPares()}, b.Registro(), Opciones{Limite: 3 * time.Second}, nil)
			if err != nil || len(res.Soluciones) == 0 {
				t.Errorf("síntesis concurrente: %v %s", err, res.Motivo)
			}
		}()
	}
	for i, prog := range []string{"(suma (filtra esPar nums))", "(largo (filtra esImpar nums))", "(maxL (filtra esPar nums))"} {
		if _, err := b.Aprender("Comp"+string(rune('A'+i)), "", debeParse(t, b.Registro(), f, prog), f); err != nil {
			t.Error(err)
		}
	}
	b.Comprimir()
	wg.Wait()
}

// TestProgramasDelBanco: every program the search builds can be printed, read back, described and written
// in Go.
func TestProgramasDelBanco(t *testing.T) {
	firmas := map[string]nucleo.Caso{
		"func F(nums []int) int":        casoEj([]V{[]V{1, 2, 3}}, 7),
		"func F(s string) string":       casoEj([]V{"hola mundo"}, "x"),
		"func F(s string, c rune) bool": casoEj([]V{"hola", int('o')}, true),
		"func F(xs []string) []string":  casoEj([]V{[]V{"a", "bb"}}, []V{"z"}),
		"func F(xs []float64) float64":  casoEj([]V{[]V{1.5, 2.0}}, 0.25),
		"func F(m [][]int) []int":       casoEj([]V{[]V{[]V{1, 2}, []V{3, 4}}}, []V{9}),
		"func F(a, b int) []int":        casoEj([]V{2, 5}, []V{1}),
	}
	r := Base()
	for firma, ej := range firmas {
		f := debeFirma(t, firma)
		s, err := nuevaSesion(context.Background(), Especificacion{Firma: f, Ejemplos: []nucleo.Caso{ej}}, r, Opciones{}.normalizar(), true)
		if err != nil {
			t.Fatal(err)
		}
		s.construirPools(time.Now().Add(time.Second))
		s.prepararMotor()
		s.mo.correr(22)
		n := 0
		for id := range s.mo.entradas {
			en := &s.mo.entradas[id]
			if en.tipo != s.objetivo || en.hoja != nil {
				continue
			}
			n++
			e := s.mo.expr(int32(id))
			txt := e.String()
			e2, err := Parse(txt, r, f)
			if err != nil || e2.String() != txt {
				t.Errorf("%s: no se lee de vuelta: %v", txt, err)
				continue
			}
			if d := Describir(e, f); d == "" || strings.Contains(d, "?") {
				t.Errorf("%s: descripción mala %q", txt, d)
			}
			if _, err := AGo(e, f); err != nil {
				t.Errorf("%s: %v", txt, err)
			}
		}
		if n < 10 {
			t.Errorf("%s: solo %d programas", firma, n)
		}
	}
}
