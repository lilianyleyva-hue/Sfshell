package sintesis

import (
	"context"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"math"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Regression tests for the problems found in the review of §4.7 (one test per problem).

// ordenar, ordenarDesc and invertir change their list in place: the generated Go must copy whatever may
// share memory with a parameter (tomar and quitar return parts of their argument).
func TestRegresionAGoCopiaAntesDeCambiar(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func F(nums []int) []int")
	for _, c := range []struct{ prog, quiero string }{
		{"(concat (ordenar (tomar nums 2)) nums)", "slices.Clone(tomar(nums, 2))"},
		{"(concat (invertir (quitar nums 1)) nums)", "slices.Clone(quitar(nums, 1))"},
		{"(concat (ordenarDesc (tomar nums 3)) nums)", "slices.Clone(tomar(nums, 3))"},
		{"(ordenar nums)", "slices.Clone(nums)"},
		{"(ordenar (si (esCero (largo nums)) nums (tomar nums 1)))", "slices.Clone(valor)"},
	} {
		src, err := AGo(debeParse(t, r, f, c.prog), f)
		if err != nil {
			t.Fatalf("%s: %v", c.prog, err)
		}
		if !strings.Contains(src, c.quiero) {
			t.Errorf("%s: falta %q en:\n%s", c.prog, c.quiero, src)
		}
	}
	// a slice the code has just built is changed directly (no needless copy)
	for _, prog := range []string{"(ordenar (filtra esPar nums))", "(invertir (ordenar nums))", "(ordenar (unicos nums))"} {
		src, err := AGo(debeParse(t, r, f, prog), f)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(src, "slices.Clone("); n > 1 || prog != "(invertir (ordenar nums))" && n > 0 {
			t.Errorf("%s: copias de más:\n%s", prog, src)
		}
	}
}

// declaraciones returns how many times each top-level function is declared in src.
func declaraciones(t *testing.T, src string) map[string]int {
	t.Helper()
	af, err := goparser.ParseFile(token.NewFileSet(), "solucion.go", src, 0)
	if err != nil {
		t.Fatalf("no se puede leer:\n%s\n%v", src, err)
	}
	out := map[string]int{}
	for _, d := range af.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			out[fd.Name.Name]++
		}
	}
	return out
}

// Helper names must never collide: learned components named like a shipped helper, and main functions
// named like a shipped helper.
func TestRegresionAGoNombresDeAyudantes(t *testing.T) {
	hayGo := true
	if _, err := exec.LookPath("go"); err != nil {
		hayGo = false
	}
	b := NuevaBiblioteca(nil)
	reg := b.Registro()
	aprender := func(nombre, firma, prog string) {
		f := debeFirma(t, firma)
		if _, err := b.Aprender(nombre, "", debeParse(t, reg, f, prog), f); err != nil {
			t.Fatalf("Aprender %s: %v", nombre, err)
		}
	}
	aprender("Fibonacci", "func Fibonacci(n int) int", "(fib n)")
	aprender("EsPalindromo", "func EsPalindromo(s string) bool", "(esPalindromoS s)")
	aprender("Factorial", "func Factorial(n int) int", "(factorial n)")
	aprender("invertirNumero", "func invertirNumero(n int) int", "(invertirNum n)")
	casos := []struct{ firma, prog, ayudante string }{
		{"func F(m int) int", "(+ (Fibonacci m) 1)", "fibonacci"},
		{"func F(s string) bool", "(no (EsPalindromo s))", "esPalindromo"},
		{"func F(m int) int", "(Factorial (+ m 1))", "factorial"},
		{"func F(m int) int", "(invertirNumero m)", "invertirNumero"},
		{"func F(m int) int", "(+ (Fibonacci m) (fib m))", "fibonacci"},
		// the main function has the name of a shipped helper
		{"func fibonacci(n int) int", "(fib n)", "fibonacciAux"},
		{"func esPrimo(n int) bool", "(esPrimo n)", "esPrimoAux"},
		{"func tomar(xs []int, n int) []int", "(tomar xs n)", "tomarAux"},
		{"func factorial(n int) int", "(+ (Factorial n) 1)", "factorialAux"},
	}
	for _, c := range casos {
		f := debeFirma(t, c.firma)
		e := debeParse(t, reg, f, c.prog)
		src, err := AGo(e, f)
		if err != nil {
			t.Errorf("%s: %v", c.prog, err)
			continue
		}
		for nombre, n := range declaraciones(t, src) {
			if n > 1 {
				t.Errorf("%s: «%s» declarada %d veces:\n%s", c.prog, nombre, n, src)
			}
		}
		if !strings.Contains(src, "func "+c.ayudante+"(") {
			t.Errorf("%s: falta el ayudante %s:\n%s", c.prog, c.ayudante, src)
		}
		if hayGo {
			comprobarTipos(t, src)
		}
	}
	// a learned helper must call the shipped helper, not itself
	f := debeFirma(t, "func F(m int) int")
	src, err := AGo(debeParse(t, reg, f, "(Fibonacci m)"), f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(src, "return fibonacci2(n)") || !strings.Contains(src, "func fibonacci2(n int) int {\n\treturn fibonacci(n)\n}") {
		t.Errorf("el componente aprendido no llama al ayudante:\n%s", src)
	}
	// a main function named like a package the code uses: the package gets another name
	g := debeFirma(t, "func strings(s string) []string")
	src, err = AGo(debeParse(t, reg, g, "(palabras s)"), g)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, `goStrings "strings"`) || !strings.Contains(src, "goStrings.Fields(s)") {
		t.Errorf("sin alias para strings:\n%s", src)
	}
	if hayGo {
		comprobarTipos(t, src)
	}
	// a main function named like a builtin the code needs cannot be written: an error, not broken code
	h := debeFirma(t, "func max(xs []int, n int) []int")
	if src, err := AGo(debeParse(t, reg, h, "(tomar xs n)"), h); err == nil {
		t.Errorf("max como nombre debería dar error:\n%s", src)
	} else if !strings.Contains(err.Error(), "«max»") {
		t.Errorf("error poco claro: %v", err)
	}
	// …but when the code does not use it, the name is fine
	src, err = AGo(debeParse(t, reg, debeFirma(t, "func max(xs []int) int"), "(maxL xs)"), debeFirma(t, "func max(xs []int) int"))
	if err != nil {
		t.Errorf("max sin usar el max de Go: %v", err)
	} else if hayGo {
		comprobarTipos(t, src)
	}
}

// Opciones.Limite and the context must hold with large inputs: the clock is read according to how long the
// candidates take, and inputs beyond what the DSL may build are refused.
func TestRegresionLimiteConEntradasGrandes(t *testing.T) {
	margen := func(d time.Duration) time.Duration { return time.Duration(float64(d) * factorCarrera) }
	// 1 MB of text: refused at once
	f := debeFirma(t, "func F(s string) int")
	grande := strings.Repeat("hola mundo ", 100000)
	inicio := time.Now()
	_, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: []nucleo.Caso{casoEj([]V{grande}, 3)}},
		Base(), Opciones{Limite: time.Second}, nil)
	if err == nil || !strings.Contains(err.Error(), "demasiado grande") {
		t.Errorf("1 MB de texto: err = %v", err)
	}
	if d := time.Since(inicio); d > margen(200*time.Millisecond) {
		t.Errorf("rechazar 1 MB tardó %v", d)
	}
	// 96 KB of text (allowed): the 100 ms limit holds
	medio := strings.Repeat("abcdefgh ", 96*1024/9)
	casos := []struct {
		firma string
		ejs   []nucleo.Caso
	}{
		{"func F(s string) int", []nucleo.Caso{casoEj([]V{medio}, 98761), casoEj([]V{"ab"}, -7)}},
		{"func F(n int) []int", []nucleo.Caso{casoEj([]V{9000}, []V{1}), casoEj([]V{8999}, []V{2, 3})}},
		{"func F(nums []int) []int", []nucleo.Caso{casoEj([]V{secuencia(9000)}, []V{1}), casoEj([]V{secuencia(5000)}, []V{2, 3})}},
	}
	for _, c := range casos {
		f := debeFirma(t, c.firma)
		inicio := time.Now()
		res, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: c.ejs}, Base(),
			Opciones{Limite: 100 * time.Millisecond}, nil)
		d := time.Since(inicio)
		if err != nil {
			t.Errorf("%s: %v", c.firma, err)
			continue
		}
		if res.Motivo != "tiempo" && res.Motivo != "banco_lleno" {
			t.Errorf("%s: Motivo = %q", c.firma, res.Motivo)
		}
		if d > margen(200*time.Millisecond) {
			t.Errorf("%s: con 100 ms de límite tardó %v (Motivo %s, %d explorados)", c.firma, d, res.Motivo, res.Explorados)
		}
	}
	// cancelling the context at 100 ms stops it soon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)
	inicio = time.Now()
	f = debeFirma(t, "func F(s string) int")
	if _, err := Sintetizar(ctx, Especificacion{Firma: f, Ejemplos: casos[0].ejs}, Base(), Opciones{Limite: 6 * time.Second}, nil); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(inicio); d > margen(250*time.Millisecond) {
		t.Errorf("cancelar a los 100 ms tardó %v", d)
	}
}

func secuencia(n int) []V {
	out := make([]V, n)
	for i := range out {
		out[i] = (i*7919)%2001 - 1000
	}
	return out
}

// The bank is bounded in memory, not only in entries: every entry caches a whole output vector.
func TestRegresionBancoMemoriaAcotada(t *testing.T) {
	if testing.Short() {
		t.Skip("tarda unos segundos")
	}
	f := debeFirma(t, "func F(nums []int) []int")
	esp := Especificacion{Firma: f, Ejemplos: []nucleo.Caso{
		casoEj([]V{secuencia(10000)}, []V{7, 7, 7}), casoEj([]V{secuencia(9999)}, []V{8}),
	}}
	runtime.GC()
	var pico uint64
	var mu sync.Mutex
	parar := make(chan struct{})
	hecho := make(chan struct{})
	go func() {
		defer close(hecho)
		var ms runtime.MemStats
		for {
			select {
			case <-parar:
				return
			case <-time.After(20 * time.Millisecond):
			}
			runtime.ReadMemStats(&ms)
			mu.Lock()
			pico = max(pico, ms.HeapAlloc)
			mu.Unlock()
		}
	}()
	s, err := nuevaSesion(context.Background(), esp, Base(), Opciones{}.normalizar(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.construirPools(time.Now().Add(time.Second))
	s.prepararMotor()
	s.mo.limite = time.Now().Add(time.Duration(float64(20*time.Second) * factorCarrera))
	s.mo.correr(90)
	close(parar)
	<-hecho
	if s.mo.motivo != "banco_lleno" {
		t.Errorf("Motivo = %q (%d entradas, %d bytes)", s.mo.motivo, len(s.mo.entradas), s.mo.bytes)
	}
	if s.mo.bytes > maxBytesBanco+pesoVector(make([]V, s.mo.m))+int64(s.mo.m)*pesoValor(secuencia(maxLargo)) {
		t.Errorf("el banco estima %d bytes; el tope es %d", s.mo.bytes, maxBytesBanco)
	}
	mu.Lock()
	defer mu.Unlock()
	if pico > 600<<20 {
		t.Errorf("el montón llegó a %d MB", pico>>20)
	}
	t.Logf("%d entradas, %d MB estimados, pico del montón %d MB", len(s.mo.entradas), s.mo.bytes>>20, pico>>20)
}

// Deep nesting must give an error, never a stack overflow (which ends the process).
func TestRegresionParseProfundo(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func F(n int) int")
	const hondo = 1000000
	if _, err := Parse(strings.Repeat("(neg ", hondo)+"n"+strings.Repeat(")", hondo), r, f); err == nil {
		t.Error("un millón de paréntesis anidados debería dar error")
	}
	if _, err := Parse("(suma "+strings.Repeat("[", hondo)+strings.Repeat("]", hondo)+")", r, f); err == nil {
		t.Error("un millón de corchetes anidados debería dar error")
	}
	// moderate nesting still works
	e, err := Parse(strings.Repeat("(neg ", 150)+"n"+strings.Repeat(")", 150), r, f)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := Evaluar(e, []V{3}); err != nil || v != 3 {
		t.Errorf("150 neg = %v %v", v, err)
	}
	// deep trees built in code: errors, short ones
	var x *Expr = nuevaVar(0, tI, "n")
	neg := r.buscarTipos("neg", tI)
	for i := 0; i < 100000; i++ {
		x = nuevaOp(neg, []*Expr{x}, nil)
	}
	if _, err := AGo(x, f); err == nil || len(err.Error()) > 300 {
		t.Errorf("AGo de un árbol hondo: %v", err)
	}
	if _, err := Evaluar(x, []V{1}); err == nil {
		t.Error("Evaluar de un árbol hondo debería dar error")
	}
	if _, err := Oraculo(x)([]V{1}); err == nil {
		t.Error("Oraculo de un árbol hondo debería dar error")
	}
	if d := Describir(x, f); d == "" {
		t.Error("Describir de un árbol hondo vacío")
	}
	if _, err := NuevaBiblioteca(nil).Aprender("Hondo", "", x, f); err == nil {
		t.Error("Aprender de un árbol hondo debería dar error")
	}
	// Cargar from memoria with a crafted entry: an error, not a crash
	_, errs := NuevaBiblioteca(nil).Cargar([]nucleo.FuncionAprendida{{Nombre: "Malo", Firma: f,
		DSL: strings.Repeat("(neg ", hondo) + "n" + strings.Repeat(")", hondo)}})
	if len(errs) != 1 {
		t.Errorf("Cargar: %v", errs)
	}
}

// The defaults 0 1 2 10 "" " " "," are always in the constant pool.
func TestRegresionConstantesPorDefecto(t *testing.T) {
	f := debeFirma(t, "func F(s string) string")
	var dadas []V
	for i := 100; i < 112; i++ {
		dadas = append(dadas, i)
	}
	for _, x := range []string{"p", "q", "r", "s", "t", "u", "v", "w", "z"} {
		dadas = append(dadas, x)
	}
	esp := Especificacion{Firma: f, Ejemplos: []nucleo.Caso{casoEj([]V{"a.b:c!d?e-f_g/h+i=j"}, "x")}, Constantes: dadas}
	s, err := nuevaSesion(context.Background(), esp, Base(), Opciones{}.normalizar(), true)
	if err != nil {
		t.Fatal(err)
	}
	hay := map[string]bool{}
	for _, c := range s.consts {
		hay[c.String()] = true
	}
	for _, quiero := range []string{"0", "1", "2", "10", `""`, `" "`, `","`} {
		if !hay[quiero] {
			t.Errorf("falta la constante %s en %v", quiero, hay)
		}
	}
	// with the text constants filling the cap, joining with a space is still found
	g := debeFirma(t, "func F(xs []string) string")
	esp = Especificacion{Firma: g, Ejemplos: []nucleo.Caso{
		casoEj([]V{[]V{"a", "b"}}, "a b"), casoEj([]V{[]V{"c", "d", "e"}}, "c d e"), casoEj([]V{[]V{"hola"}}, "hola"),
	}, Constantes: []V{"uno", "dos", "tres", "cuatro", "cinco", "seis", "siete", "ocho"}}
	res, err := Sintetizar(context.Background(), esp, Base(), Opciones{Limite: time.Duration(float64(3*time.Second) * factorCarrera), Semilla: 1}, nil)
	if err != nil || len(res.Soluciones) == 0 || res.Soluciones[0].Expr.String() != `(unir xs " ")` {
		t.Errorf("unir con espacio: %v %v %s", res.Soluciones, err, res.Motivo)
	}
}

// Parameter names inside Spanish words ("s" in "más", "n" in "también") are not parameters.
func TestRegresionFraseAprendidaAcentos(t *testing.T) {
	b := NuevaBiblioteca(nil)
	f := debeFirma(t, "func Larga(s string) string")
	p, err := b.Aprender("Larga", "la palabra más larga de s", debeParse(t, b.Registro(), f, "(masLarga (palabras s))"), f)
	if err != nil {
		t.Fatal(err)
	}
	if p.Frase != "la palabra más larga de {0}" {
		t.Errorf("Frase = %q", p.Frase)
	}
	g := debeFirma(t, "func F(texto string) string")
	if got := Describir(debeParse(t, b.Registro(), g, "(Larga texto)"), g); got != "la palabra más larga de texto" {
		t.Errorf("Describir = %q", got)
	}
	h := debeFirma(t, "func Doble(n int) int")
	if got := fraseAprendida("Doble", "el doble de n también", h); got != "el doble de {0} también" {
		t.Errorf("fraseAprendida = %q", got)
	}
	if got := fraseAprendida("Doble", "n por dos, y n", h); got != "{0} por dos, y {0}" {
		t.Errorf("fraseAprendida = %q", got)
	}
}

// Lambda parameters are named by their place in scope, not guessed from the variables the body uses.
func TestRegresionDescribirLambdas(t *testing.T) {
	r := Base()
	casos := []struct{ firma, prog, quiero string }{
		{"func F(nums []int) int", "(pliega (λ (a b) (+ a 1)) 0 nums)", "con a más 1, empezando en 0"},
		{"func F(nums []int, k int) int", "(pliega (λ (a b) (+ a k)) 0 nums)", "con a más k, empezando en 0"},
		{"func F(nums []int, k int) []int", "(mapea (λ y k) nums)", "k para cada y de nums"},
		{"func F(nums []int) int", "(pliega (λ (a b) (+ a (* b b))) 0 nums)", "con a más b por b"},
		{"func F(nums []int, k int) []int", "(filtra (λ x (> k 3)) nums)", "tales que k es mayor que 3"},
	}
	for _, c := range casos {
		f := debeFirma(t, c.firma)
		got := Describir(debeParse(t, r, f, c.prog), f)
		if !strings.Contains(got, c.quiero) {
			t.Errorf("Describir(%s) = %q; quiero que contenga %q", c.prog, got, c.quiero)
		}
		if strings.Contains(got, "nums") && strings.Count(got, "nums") != strings.Count(c.prog, "nums") {
			t.Errorf("Describir(%s) = %q cambia el nombre de nums", c.prog, got)
		}
	}
	// nested lambdas
	f := debeFirma(t, "func F(nums []int) []int")
	got := Describir(debeParse(t, r, f, "(mapea (λ x (contar (λ y (> y x)) nums)) nums)"), f)
	if !strings.Contains(got, "y es mayor que x") && !strings.Contains(got, "mayores que x") {
		t.Errorf("lambdas anidadas: %q", got)
	}
}

// A fused counting loop over (rango a (+ n 1)) must stay short where the interpreter has no result.
func TestRegresionAGoRangoAcotado(t *testing.T) {
	r := Base()
	f := debeFirma(t, "func F(n int) int")
	casos := []struct{ prog, guarda, bucle string }{
		{"(suma (rango 1 (+ n 1)))", "if n > 10000 {", "for i := 1; i <= n; i++ {"},
		{"(producto (rango 1 (+ n 1)))", "if n > 10000 {", "for i := 1; i <= n; i++ {"},
		{"(contar esPar (rango 0 n))", "if n > 10000 {", "for i := 0; i < n; i++ {"},
		{"(suma (rango -5 (* n 2)))", "if hasta > 9995 {", "for i := -5; i < hasta; i++ {"},
	}
	for _, c := range casos {
		src, err := AGo(debeParse(t, r, f, c.prog), f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(src, c.guarda) || !strings.Contains(src, c.bucle) {
			t.Errorf("%s: falta %q o %q:\n%s", c.prog, c.guarda, c.bucle, src)
		}
		if v, err := Evaluar(debeParse(t, r, f, c.prog), []V{math.MaxInt}); err == nil {
			t.Errorf("%s con MaxInt = %v; el intérprete no debería dar resultado", c.prog, v)
		}
	}
	// a start that is not a constant uses the bounded rango helper
	g := debeFirma(t, "func F(a, b int) int")
	src, err := AGo(debeParse(t, r, g, "(suma (rango a (+ b 1)))"), g)
	if err != nil || !strings.Contains(src, "range rango(a, b+1)") || !strings.Contains(src, "len(out) < 10000") {
		t.Errorf("rango con inicio variable: %v\n%s", err, src)
	}
}

// Decision trees with three examples: abs without the abs primitive.
func TestRegresionUnificarTresEjemplos(t *testing.T) {
	r := nuevoRegistro()
	quedan := map[string]bool{"+": true, "-": true, "*": true, "neg": true, "<": true, ">": true, "==": true,
		"esNegativo": true, "esPositivo": true, "esPar": true, "si": true}
	for _, p := range Base().Todas() {
		if quedan[p.Nombre] {
			r.Agregar(p)
		}
	}
	f := debeFirma(t, "func Absoluto(n int) int")
	ejs := []nucleo.Caso{casoEj([]V{-3}, 3), casoEj([]V{4}, 4), casoEj([]V{0}, 0)}
	s, err := nuevaSesion(context.Background(), Especificacion{Firma: f, Ejemplos: ejs}, r, Opciones{}.normalizar(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.construirPools(time.Now().Add(time.Second))
	s.prepararMotor()
	s.mo.correr(20)
	un := &unificador{mo: s.mo}
	a := un.intentar()
	if a == nil || a.hoja() {
		t.Fatalf("Unificar no construyó un árbol con 3 ejemplos")
	}
	e := un.expr(a, r.buscarTipos("si", tB, tI, tI))
	for _, n := range []int{-50, 50, -7, 0, 1} {
		if v, err := Evaluar(e, []V{n}); err != nil || v != max(n, -n) {
			t.Errorf("%s con %d = %v %v", e, n, v, err)
		}
	}
	// a leaf that satisfies a single example would memorize it: no tree for unrelated outputs
	g := debeFirma(t, "func F(n int) int")
	s, err = nuevaSesion(context.Background(), Especificacion{Firma: g, Ejemplos: []nucleo.Caso{
		casoEj([]V{1}, 917), casoEj([]V{2}, -443), casoEj([]V{3}, 5081)}}, r, Opciones{}.normalizar(), true)
	if err != nil {
		t.Fatal(err)
	}
	s.construirPools(time.Now().Add(time.Second))
	s.prepararMotor()
	s.mo.correr(20)
	if a := (&unificador{mo: s.mo}).intentar(); a != nil {
		t.Errorf("árbol que memoriza los ejemplos: %+v", a)
	}
}

// User-facing messages: no internal names, singular for one example, short AGo errors.
func TestRegresionMensajes(t *testing.T) {
	f := debeFirma(t, "func F(n int) int")
	_, err := Sintetizar(context.Background(), Especificacion{Firma: f}, Base(), Opciones{}, nil)
	if err == nil || strings.Contains(err.Error(), "DesdeConceptos") {
		t.Errorf("sin ejemplos: %v", err)
	}
	var titulos []string
	var mu sync.Mutex
	tr := nucleo.NuevaTraza(func(ev nucleo.Evento) {
		if ev.Tipo == "paso" && ev.Paso != nil {
			mu.Lock()
			titulos = append(titulos, ev.Paso.Titulo)
			mu.Unlock()
		}
	}, nil)
	if _, err := Sintetizar(context.Background(), Especificacion{Firma: f, Ejemplos: []nucleo.Caso{casoEj([]V{3}, 6)}},
		Base(), Opciones{Limite: 2 * time.Second}, tr.Raiz()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(titulos) == 0 || titulos[0] != "Busco un programa que cumpla el ejemplo" {
		t.Errorf("títulos = %q", titulos)
	}
	if got := textoEjemplos(3); got != "los 3 ejemplos" {
		t.Errorf("textoEjemplos(3) = %q", got)
	}
	// an AGo error never carries the generated source
	if _, err := terminarFuente("package solucion\n\nfunc F( {\n"+strings.Repeat("x := 1\n", 10000)+"}\n", "F"); err == nil || len(err.Error()) > 300 {
		t.Errorf("error de AGo demasiado largo (%d bytes)", len(err.Error()))
	}
}
