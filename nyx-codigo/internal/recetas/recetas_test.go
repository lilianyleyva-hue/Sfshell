package recetas

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// ---- shipped content ----

func TestCargaSinErrores(t *testing.T) {
	for _, err := range erroresDeCarga() {
		t.Error(err)
	}
}

var funcionesSpec = []string{
	// numbers
	"EsPrimo", "PrimosHasta", "Factorial", "FactorialGrande", "Fibonacci", "MCD", "MCM", "SumaDigitos",
	"InvertirNumero", "EsCapicuaNumero", "Divisores", "EsPerfecto", "ABinario", "DesdeBinario", "Potencia", "RaizEntera",
	// lists
	"BusquedaBinaria", "OrdenarBurbuja", "OrdenarInsercion", "Maximo", "Minimo", "Promedio", "Mediana", "Moda",
	"SinRepetidos", "Rotar", "Intercambiar", "SegundoMayor", "SumaAcumulada", "ParesQueSuman", "Fusionar",
	// text
	"EsPalindromo", "EsAnagrama", "ContarPalabras", "FrecuenciaPalabras", "FrecuenciaLetras", "PalabraMasLarga",
	"Capitalizar", "CifradoCesar", "QuitarAcentos", "ContarVocales", "Invertir",
	// matrices
	"Transponer", "SumaMatrices", "MultiplicarMatrices", "Identidad",
	// small structs
	"Distancia", "Area", "Perimetro", "UsarPila", "UsarCola",
}

var plantillasSpec = []string{
	"dos_numeros_operacion", "calculadora", "adivina_numero", "tabla_multiplicar", "fizzbuzz", "saludar",
	"par_impar", "mayor_de_tres", "promedio_notas", "conversor_temperatura", "contar_texto", "lista_tareas",
	"piedra_papel_tijera", "primos_hasta", "invertir_texto",
}

var simbolosSpec = []string{
	"fmt.Println", "fmt.Printf", "fmt.Sprintf", "fmt.Scan", "fmt.Errorf",
	"strings.Split", "strings.Fields", "strings.Join", "strings.Contains", "strings.Index", "strings.Replace",
	"strings.ReplaceAll", "strings.ToUpper", "strings.ToLower", "strings.TrimSpace", "strings.Trim",
	"strings.HasPrefix", "strings.HasSuffix", "strings.Repeat", "strings.Builder", "strings.Count",
	"strings.Title", "strings.EqualFold",
	"strconv.Itoa", "strconv.Atoi", "strconv.ParseFloat", "strconv.FormatInt", "strconv.Quote",
	"sort.Ints", "sort.Strings", "sort.Slice",
	"slices.Sort", "slices.Contains", "slices.Index", "slices.Reverse", "slices.Max", "slices.Min",
	"maps.Keys",
	"math.Sqrt", "math.Pow", "math.Abs", "math.Max", "math.Min", "math.Floor", "math.Ceil", "math.Round",
	"math.MaxInt", "math.Inf",
	"unicode.IsLetter", "unicode.IsDigit", "unicode.IsUpper", "unicode.ToUpper", "unicode.IsSpace",
	"utf8.RuneCountInString",
	"bufio.Scanner", "bufio.NewReader",
	"os.Args", "os.Stdin", "os.ReadFile", "os.WriteFile", "os.Open", "os.Create",
	"errors.New", "errors.Is", "errors.As",
	"time.Now", "time.Since", "time.Duration", "time.Sleep",
	"rand.Intn",
	"regexp.MustCompile", "regexp.MatchString", "regexp.FindAllString",
	"json.Marshal", "json.Unmarshal",
	"bytes.Buffer", "container/heap", "sync.WaitGroup", "sync.Mutex", "http.Get", "http.ListenAndServe",
	"io.ReadAll",
	"append", "len", "cap", "make", "copy", "delete", "new", "panic", "recover", "min", "max",
}

var conceptosSpec = []string{
	"variable", "tipos", "slice", "array", "map", "struct", "puntero", "método", "interfaz", "error", "defer",
	"panic/recover", "goroutine", "canal", "select", "for/range", "switch", "función anónima/closure",
	"paquete/módulo", "go mod", "go run/build/test", "genéricos", "constante/iota", "cadena y runa", "nil",
}

func TestContenidoCompleto(t *testing.T) {
	rs := Funciones()
	if len(rs) < 50 {
		t.Errorf("hay %d recetas; deberían ser unas 50", len(rs))
	}
	tiene := map[string]bool{}
	for _, r := range rs {
		tiene[r.Nombre] = true
	}
	for _, n := range funcionesSpec {
		if !tiene[n] {
			t.Errorf("falta la receta %s", n)
		}
	}
	ps := Plantillas()
	tieneP := map[string]bool{}
	for _, p := range ps {
		tieneP[p.Nombre] = true
	}
	for _, n := range plantillasSpec {
		if !tieneP[n] {
			t.Errorf("falta la plantilla %s", n)
		}
	}
	ns := Notas()
	tieneN := map[string]bool{}
	for _, n := range ns {
		tieneN[n.Clave] = true
	}
	for _, n := range simbolosSpec {
		if !tieneN[n] {
			t.Errorf("falta la nota del símbolo %s", n)
		}
	}
	for _, n := range conceptosSpec {
		if !tieneN[n] {
			t.Errorf("falta la nota del concepto %s", n)
		}
	}
	if len(ns) < 105 {
		t.Errorf("hay %d notas; deberían ser unas 80 de símbolos y 25 de conceptos", len(ns))
	}
}

func TestRecetasBienFormadas(t *testing.T) {
	for _, r := range Funciones() {
		if !r.Firma.Probable() {
			t.Errorf("%s: la firma %s no se puede probar", r.Nombre, r.Firma.Go())
		}
		if len(r.Casos) < 5 {
			t.Errorf("%s: tiene %d casos", r.Nombre, len(r.Casos))
		}
		for i, c := range r.Casos {
			if c.Expectativa != nucleo.EspReferencia || c.Origen != nucleo.OrigenReceta || !c.ConEsperado() {
				t.Errorf("%s, caso %d: expectativa %q, origen %q", r.Nombre, i+1, c.Expectativa, c.Origen)
			}
		}
		if !strings.HasPrefix(r.Codigo, "package solucion") {
			t.Errorf("%s: el código debe empezar por «package solucion»", r.Nombre)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "x.go", r.Codigo, parser.ParseComments); err != nil {
			t.Errorf("%s: el código no se puede leer: %v", r.Nombre, err)
		}
		for _, p := range r.Palabras {
			if p != nucleo.Normalizar(p) {
				t.Errorf("%s: la palabra %q no está normalizada", r.Nombre, p)
			}
		}
		if f, err := FirmaDe(r.Codigo, nombreFuncion(r)); err != nil || f.Forma() != r.Firma.Forma() {
			t.Errorf("%s: FirmaDe = %v, %v", r.Nombre, f.Forma(), err)
		}
	}
}

func nombreFuncion(r Receta) string {
	if r.Firma.Receptor != nil {
		return r.Firma.Receptor.Tipo.Go() + "." + r.Firma.Nombre
	}
	return r.Firma.Nombre
}

func TestNotasBienFormadas(t *testing.T) {
	for _, n := range Notas() {
		if !strings.HasPrefix(n.Ejemplo, "package main") {
			t.Errorf("%s: el ejemplo debe ser un programa (package main)", n.Clave)
		}
		if n.Salida == "" {
			t.Errorf("%s: falta la salida del ejemplo", n.Clave)
		}
		if len(n.Palabras) == 0 {
			t.Errorf("%s: faltan palabras", n.Clave)
		}
		if k := frases(n.Resumen); k > 3 {
			t.Errorf("%s: el resumen tiene %d frases (máximo 3): %s", n.Clave, k, n.Resumen)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "x.go", n.Ejemplo, 0); err != nil {
			t.Errorf("%s: el ejemplo no se puede leer: %v", n.Clave, err)
		}
	}
}

// frases counts sentences: ". " followed by an upper-case letter, plus the last one.
func frases(s string) int {
	k := 1
	rs := []rune(strings.TrimSpace(s))
	for i := 0; i+2 < len(rs); i++ {
		if (rs[i] == '.' || rs[i] == '!' || rs[i] == '?') && rs[i+1] == ' ' && strings.ContainsRune("ABCDEFGHIJKLMNÑOPQRSTUVWXYZÁÉÍÓÚ¿¡", rs[i+2]) {
			k++
		}
	}
	return k
}

func TestCopiasIndependientes(t *testing.T) {
	a := Funciones()
	a[0].Palabras[0] = "cambiado"
	a[0].Casos[0].Entradas[0] = "cambiado"
	b := Funciones()
	if b[0].Palabras[0] == "cambiado" || b[0].Casos[0].Entradas[0] == "cambiado" {
		t.Error("Funciones debe devolver copias")
	}
	p := Plantillas()
	p[0].Huecos[0].Opciones = nil
	if len(Plantillas()[0].Huecos[0].Opciones) == 0 {
		t.Error("Plantillas debe devolver copias")
	}
}

// ---- retrieval ----

func TestBuscarFunciones(t *testing.T) {
	r := BuscarFunciones([]string{"primo", "numero"}, nil, 5)
	if len(r) == 0 || r[0].Receta.Nombre != "EsPrimo" {
		t.Fatalf("BuscarFunciones(primo numero) = %v", nombresR(r))
	}
	casos := []struct {
		lemas  []string
		quiero string
	}{
		{[]string{"palindromo"}, "EsPalindromo"},
		{[]string{"contar", "vocal"}, "ContarVocales"},
		{[]string{"maximo", "lista"}, "Maximo"},
		{[]string{"ordenar", "burbuja"}, "OrdenarBurbuja"},
		{[]string{"factorial"}, "Factorial"},
		{[]string{"transponer", "matriz"}, "Transponer"},
		{[]string{"mcd", "numeros"}, "MCD"},
		{[]string{"frecuencia", "palabras"}, "FrecuenciaPalabras"},
	}
	for _, c := range casos {
		r := BuscarFunciones(c.lemas, nil, 3)
		if len(r) == 0 || r[0].Receta.Nombre != c.quiero {
			t.Errorf("BuscarFunciones(%v) = %v, quería %s primero", c.lemas, nombresR(r), c.quiero)
		}
	}
	// signature filter
	f := nucleo.Firma{Nombre: "X", Params: []nucleo.Param{{Nombre: "n", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TBool}}
	for _, p := range BuscarFunciones([]string{"primo", "lista", "numero"}, &f, 0) {
		if p.Receta.Firma.Forma() != "(int)bool" {
			t.Errorf("%s tiene la forma %s", p.Receta.Nombre, p.Receta.Firma.Forma())
		}
	}
	if r := BuscarFunciones([]string{"primo"}, &f, 1); len(r) != 1 || r[0].Receta.Nombre != "EsPrimo" {
		t.Errorf("con firma (int)bool: %v", nombresR(r))
	}
	if r := BuscarFunciones([]string{"zzzz"}, nil, 3); len(r) != 0 {
		t.Errorf("una palabra desconocida no debe encontrar nada: %v", nombresR(r))
	}
	if r := BuscarFunciones([]string{"numero"}, nil, 2); len(r) != 2 || r[0].Puntos < r[1].Puntos {
		t.Errorf("k=2 debe dar 2 resultados ordenados: %v", r)
	}
}

func nombresR(ps []Puntuada) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Receta.Nombre)
	}
	return out
}

func TestBuscarPlantillas(t *testing.T) {
	r := BuscarPlantillas([]string{"programa", "pedir", "dos", "numero", "sumar"}, 3)
	if len(r) == 0 || r[0].Plantilla.Nombre != "dos_numeros_operacion" {
		t.Fatalf("BuscarPlantillas = %v", nombresP(r))
	}
	casos := []struct {
		lemas  []string
		quiero string
	}{
		{[]string{"calculadora"}, "calculadora"},
		{[]string{"adivinar", "numero"}, "adivina_numero"},
		{[]string{"tabla", "multiplicar"}, "tabla_multiplicar"},
		{[]string{"fizzbuzz"}, "fizzbuzz"},
		{[]string{"piedra", "papel", "tijera"}, "piedra_papel_tijera"},
		{[]string{"convertir", "temperatura"}, "conversor_temperatura"},
		{[]string{"lista", "tarea"}, "lista_tareas"},
	}
	for _, c := range casos {
		r := BuscarPlantillas(c.lemas, 3)
		if len(r) == 0 || r[0].Plantilla.Nombre != c.quiero {
			t.Errorf("BuscarPlantillas(%v) = %v, quería %s", c.lemas, nombresP(r), c.quiero)
		}
	}
}

func nombresP(ps []PuntuadaPlantilla) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Plantilla.Nombre)
	}
	return out
}

func TestBuscarNotas(t *testing.T) {
	r := BuscarNotas(nil, "strings.Split", 3)
	if len(r) == 0 || r[0].Clave != "strings.Split" {
		t.Fatalf("BuscarNotas(strings.Split) = %v", clavesN(r))
	}
	if len(r) > 3 {
		t.Errorf("k=3 y devolvió %d", len(r))
	}
	for _, c := range []struct{ simbolo, quiero string }{
		{"defer", "defer"},
		{"Defer", "defer"},
		{"metodo", "método"},
		{"math/rand.Intn", "rand.Intn"},
		{"closure", "función anónima/closure"},
		{"goroutines", "goroutine"},
		{"heap.Push", "container/heap"},
		{"encoding/json.Marshal", "json.Marshal"},
		{"iota", "constante/iota"},
	} {
		r := BuscarNotas(nil, c.simbolo, 1)
		if len(r) != 1 || r[0].Clave != c.quiero {
			t.Errorf("BuscarNotas(%q) = %v, quería %s", c.simbolo, clavesN(r), c.quiero)
		}
	}
	r = BuscarNotas([]string{"leer", "archivo"}, "", 3)
	if len(r) == 0 || (r[0].Clave != "os.ReadFile" && r[0].Clave != "os.Open" && r[0].Clave != "bufio.Scanner") {
		t.Errorf("BuscarNotas(leer archivo) = %v", clavesN(r))
	}
	r = BuscarNotas([]string{"separar", "texto"}, "", 3)
	if len(r) == 0 || r[0].Clave != "strings.Split" && r[0].Clave != "strings.Fields" {
		t.Errorf("BuscarNotas(separar texto) = %v", clavesN(r))
	}
}

func clavesN(ns []Nota) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Clave)
	}
	return out
}

// ---- templates ----

func plantilla(t *testing.T, nombre string) Plantilla {
	t.Helper()
	for _, p := range Plantillas() {
		if p.Nombre == nombre {
			return p
		}
	}
	t.Fatalf("no encuentro la plantilla %s", nombre)
	return Plantilla{}
}

func TestInstanciarRechazaInyeccion(t *testing.T) {
	p := plantilla(t, "tabla_multiplicar")
	if _, err := Instanciar(p, map[string]string{"numero": "; os.Remove(\"x\")"}); err == nil {
		t.Error("debe rechazar código en un hueco entero")
	}
	if _, err := Instanciar(p, map[string]string{"numero": "7; os.Exit(1)"}); err == nil {
		t.Error("debe rechazar código detrás de un número")
	}
	if _, err := Instanciar(p, map[string]string{"numero": "99999999999"}); err == nil {
		t.Error("debe rechazar números enormes")
	}
	if _, err := Instanciar(p, map[string]string{"otro": "3"}); err == nil {
		t.Error("debe rechazar huecos que no existen")
	}
	src, err := Instanciar(p, map[string]string{"numero": " 9 "})
	if err != nil || !strings.Contains(src, "n := 9") {
		t.Errorf("Instanciar(9) = %v\n%s", err, src)
	}
	// text slots are quoted: the injected code stays inside a string literal
	s := plantilla(t, "saludar")
	malo := `"); os.Remove("x"); fmt.Println("`
	src, err = Instanciar(s, map[string]string{"saludo": malo})
	if err != nil {
		t.Fatalf("Instanciar(saludo) = %v", err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatalf("no se puede leer: %v\n%s", err, src)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Remove" {
			t.Errorf("el texto se ha convertido en código:\n%s", src)
		}
		return true
	})
	if !strings.Contains(src, `"\"); os.Remove(\"x\"); fmt.Println(\""`) {
		t.Errorf("el texto debe ir entre comillas:\n%s", src)
	}
	if _, err := Instanciar(s, map[string]string{"saludo": "hola\nmundo"}); err == nil {
		t.Error("debe rechazar saltos de línea en un texto")
	}
}

func TestInstanciarOperacion(t *testing.T) {
	p := plantilla(t, "dos_numeros_operacion")
	for _, c := range []struct{ valor, op string }{
		{"suma", "+"}, {"restar", "-"}, {"multiplicación", "*"}, {"multiplicacion", "*"}, {"por", "*"}, {"división", "/"}, {"entre", "/"},
	} {
		src, err := Instanciar(p, map[string]string{"op": c.valor})
		if err != nil || !strings.Contains(src, "resultado := a "+c.op+" b") {
			t.Errorf("op=%s: %v\n%s", c.valor, err, src)
		}
	}
	if _, err := Instanciar(p, map[string]string{"op": "potencia"}); err == nil {
		t.Error("debe rechazar una operación desconocida")
	}
	if _, err := Instanciar(p, map[string]string{"op": "+ b; os.Exit(1); _ = a"}); err == nil {
		t.Error("debe rechazar código en una operación")
	}
	src, err := Instanciar(p, nil)
	if err != nil || !strings.Contains(src, "a + b") {
		t.Errorf("sin valores debe usar la suma: %v", err)
	}
	if !strings.Contains(src, `"La suma es"`) {
		t.Errorf("falta el texto de la suma:\n%s", src)
	}
}

func TestPlantillaInsegura(t *testing.T) {
	for _, fuente := range []string{
		"package main\n\nfunc main() { _ = {{.n}} }\n",
		"package main\n\nfunc main() { _ = {{printf \"%s\" .n}} }\n",
		"package main\n\n{{$x := .n}}func main() { _ = {{$x}} }\n",
		"package main\n\n{{if .n}}func main() { _ = {{.n | printf \"%s\"}} }{{end}}\n",
	} {
		p := Plantilla{Nombre: "x", Huecos: []Hueco{{Nombre: "n", Clase: "entero", PorDefecto: "1"}}, Fuente: fuente}
		if _, err := Instanciar(p, nil); err == nil {
			t.Errorf("debe rechazar la plantilla %q", fuente)
		}
	}
	p := Plantilla{Nombre: "x", Huecos: []Hueco{{Nombre: "n", Clase: "entero", PorDefecto: "1"}},
		Fuente: "package main\n\n{{if eq .n \"1\"}}// uno\n{{end}}func main() { _ = {{.n | go_entero}} }\n"}
	if src, err := Instanciar(p, nil); err != nil || !strings.Contains(src, "_ = 1") {
		t.Errorf("plantilla segura: %v\n%s", err, src)
	}
}

func TestInstanciarFaltaHueco(t *testing.T) {
	p := Plantilla{Nombre: "x", Huecos: []Hueco{{Nombre: "n", Clase: "entero", Pregunta: "¿Cuántos?"}},
		Fuente: "package main\n\nfunc main() { _ = {{go_entero .n}} }\n"}
	_, err := Instanciar(p, nil)
	if !errors.Is(err, ErrFaltaHueco) || !strings.Contains(err.Error(), "¿Cuántos?") {
		t.Errorf("err = %v", err)
	}
	src, err := Instanciar(p, map[string]string{"n": "-3"})
	if err != nil || !strings.Contains(src, "_ = -3") {
		t.Errorf("Instanciar = %v\n%s", err, src)
	}
	p.Huecos[0].Clase = "rara"
	if _, err := Instanciar(p, map[string]string{"n": "3"}); err == nil {
		t.Error("una clase desconocida debe dar error")
	}
	p.Huecos[0] = Hueco{Nombre: "w", Clase: "palabra"}
	p.Fuente = "package main\n\nfunc main() { _ = {{go_texto .w}} }\n"
	if _, err := Instanciar(p, map[string]string{"w": "dos palabras"}); err == nil {
		t.Error("una palabra no puede tener espacios")
	}
	if src, err := Instanciar(p, map[string]string{"w": "árbol"}); err != nil || !strings.Contains(src, `"árbol"`) {
		t.Errorf("palabra: %v\n%s", err, src)
	}
}

func TestInstanciarTodas(t *testing.T) {
	for _, p := range Plantillas() {
		if len(p.Casos) < 2 {
			t.Errorf("%s: tiene %d casos", p.Nombre, len(p.Casos))
		}
		for i, c := range p.Casos {
			src, err := Instanciar(p, c.Huecos)
			if err != nil {
				t.Errorf("%s, caso %d: %v", p.Nombre, i+1, err)
				continue
			}
			if _, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0); err != nil {
				t.Errorf("%s, caso %d: %v", p.Nombre, i+1, err)
			}
			if c.Caso.Comparar == "" || c.Caso.Expectativa != nucleo.EspReferencia {
				t.Errorf("%s, caso %d: comparar %q, expectativa %q", p.Nombre, i+1, c.Caso.Comparar, c.Caso.Expectativa)
			}
		}
		for _, h := range p.Huecos {
			if h.Pregunta == "" || h.PorDefecto == "" {
				t.Errorf("%s: el hueco %s necesita pregunta y valor por defecto", p.Nombre, h.Nombre)
			}
			if _, err := validarHueco(h, h.PorDefecto); err != nil {
				t.Errorf("%s: el valor por defecto de %s no vale: %v", p.Nombre, h.Nombre, err)
			}
		}
	}
}

// ---- envoltorio ----

const sumaPares = `package solucion

// SumaPares suma los números pares de nums.
func SumaPares(nums []int) int {
	total := 0
	for _, n := range nums {
		if n%2 == 0 {
			total += n
		}
	}
	return total
}
`

func firmaSumaPares() nucleo.Firma {
	return nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
}

func TestEnvolverSumaPares(t *testing.T) {
	prog, aCaso, err := Envolver(sumaPares, firmaSumaPares())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(prog, "package main") || !strings.Contains(prog, "func main()") || !strings.Contains(prog, "func SumaPares(") {
		t.Fatalf("programa raro:\n%s", prog)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", prog, 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prog, "os.Stderr") || !strings.Contains(prog, "Escribe una lista de números enteros separados por espacios") {
		t.Errorf("las preguntas deben ir en español a stderr:\n%s", prog)
	}
	c := nucleo.Caso{Entradas: []nucleo.Valor{[]nucleo.Valor{1, 2, 3, 4}}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario}
	cp, ok := aCaso(c)
	if !ok || cp.Entrada != "1 2 3 4\n" || cp.Esperado != "6\n" || cp.Comparar != "lineas" || cp.Expectativa != nucleo.EspUsuario {
		t.Errorf("aCaso = %+v, %v", cp, ok)
	}
	// without the package clause it works too
	sinPaquete := strings.TrimPrefix(sumaPares, "package solucion\n")
	if _, _, err := Envolver(sinPaquete, firmaSumaPares()); err != nil {
		t.Errorf("sin package: %v", err)
	}
	// and it really prints 6 for «1 2 3 4»
	m := nuevoModulo(t)
	m.escribir("sumapares/main.go", prog)
	bin := m.construir()
	out, errOut, err := ejecutar(filepath.Join(bin, "sumapares"), nil, "1 2 3 4\n")
	if err != nil || out != "6\n" {
		t.Errorf("salida %q, error %v, stderr %q", out, err, errOut)
	}
	if !strings.Contains(errOut, "Escribe una lista") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestEnvolverErrores(t *testing.T) {
	f := firmaSumaPares()
	if _, _, err := Envolver("", f); err == nil {
		t.Error("sin código debe fallar")
	}
	if _, _, err := Envolver("package x\nfunc Otra() int { return 1 }\n", f); err == nil {
		t.Error("sin la función debe fallar")
	}
	if _, _, err := Envolver(sumaPares+"\nfunc main() {}\n", f); err == nil {
		t.Error("con main debe fallar")
	}
	g := f
	g.Res = nil
	if _, _, err := Envolver(sumaPares, g); err == nil {
		t.Error("sin resultados debe fallar")
	}
	g = f
	g.Receptor = &nucleo.Param{Nombre: "p", Tipo: nucleo.Tipo{Clase: nucleo.CStruct, Nombre: "P", Definido: true}}
	if _, _, err := Envolver(sumaPares, g); err == nil {
		t.Error("un método debe fallar")
	}
	g = f
	g.Params = []nucleo.Param{{Nombre: "m", Tipo: nucleo.MapaDe(nucleo.TString, nucleo.TInt)}}
	if _, _, err := Envolver(sumaPares, g); err == nil || !strings.Contains(err.Error(), "map[string]int") {
		t.Errorf("un mapa como entrada debe fallar: %v", err)
	}
}

func TestEnvolverCasos(t *testing.T) {
	f := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{
		{Nombre: "s", Tipo: nucleo.TString}, {Nombre: "ws", Tipo: nucleo.ListaDe(nucleo.TString)}, {Nombre: "x", Tipo: nucleo.TFloat},
	}, Res: []nucleo.Tipo{nucleo.ListaDe(nucleo.TFloat), nucleo.TError}}
	src := "package p\n\nfunc F(s string, ws []string, x float64) ([]float64, error) { return nil, nil }\n"
	_, aCaso, err := Envolver(src, f)
	if err != nil {
		t.Fatal(err)
	}
	c := nucleo.Caso{Entradas: []nucleo.Valor{"hola mundo", []nucleo.Valor{"a", "b"}, 2.5}, Esperado: []nucleo.Valor{[]nucleo.Valor{1.5, 2.0}, nil}, Expectativa: nucleo.EspReferencia}
	cp, ok := aCaso(c)
	if !ok || cp.Entrada != "hola mundo\na b\n2.5\n" || cp.Esperado != "1.5 2\n" || cp.Comparar != "numeros" {
		t.Errorf("aCaso = %+v, %v", cp, ok)
	}
	c.Esperado = []nucleo.Valor{nil, nucleo.ErrorV("mal")}
	cp, ok = aCaso(c)
	if !ok || cp.Esperado != "error:" || cp.Comparar != "contiene" {
		t.Errorf("con error: %+v, %v", cp, ok)
	}
	c.Entradas[1] = []nucleo.Valor{"dos palabras"}
	if _, ok := aCaso(c); ok {
		t.Error("una palabra con espacios no se puede escribir en una línea de palabras")
	}
	c.Entradas[1] = []nucleo.Valor{"a"}
	c.Entradas[0] = "dos\nlíneas"
	if _, ok := aCaso(c); ok {
		t.Error("un texto con salto de línea no cabe en una línea")
	}
	c = nucleo.Caso{Entradas: []nucleo.Valor{"x", []nucleo.Valor{}, 1.0}}
	if cp, ok := aCaso(c); !ok || cp.Expectativa != nucleo.EspNinguna || cp.Esperado != "" {
		t.Errorf("sin esperado: %+v %v", cp, ok)
	}
}

// ---- renaming ----

func receta(t *testing.T, nombre string) Receta {
	t.Helper()
	for _, r := range Funciones() {
		if r.Nombre == nombre {
			return r
		}
	}
	t.Fatalf("no encuentro la receta %s", nombre)
	return Receta{}
}

func TestRenombrar(t *testing.T) {
	r := receta(t, "EsPrimo")
	n := Renombrar(r, "EsNumeroPrimo")
	if n.Nombre != "EsNumeroPrimo" || n.Firma.Nombre != "EsNumeroPrimo" {
		t.Errorf("nombre = %s / %s", n.Nombre, n.Firma.Nombre)
	}
	if !strings.Contains(n.Codigo, "func EsNumeroPrimo(n int) bool") || strings.Contains(n.Codigo, "func EsPrimo(") {
		t.Errorf("código:\n%s", n.Codigo)
	}
	if !strings.Contains(n.Codigo, "// EsNumeroPrimo dice") {
		t.Errorf("el comentario también cambia:\n%s", n.Codigo)
	}
	if r.Nombre != "EsPrimo" || !strings.Contains(r.Codigo, "func EsPrimo(") {
		t.Error("el original no debe cambiar")
	}
	if f, err := FirmaDe(n.Codigo, "EsNumeroPrimo"); err != nil || f.Forma() != "(int)bool" {
		t.Errorf("FirmaDe = %v %v", f, err)
	}
	// invalid names leave it unchanged
	for _, malo := range []string{"", "1x", "func", "con espacio"} {
		if x := Renombrar(r, malo); x.Nombre != "EsPrimo" {
			t.Errorf("Renombrar(%q) cambió el nombre", malo)
		}
	}
	// recursive calls are renamed too
	for _, rc := range Funciones() {
		if strings.Count(rc.Codigo, rc.Firma.Nombre+"(") >= 2 && rc.Firma.Receptor == nil {
			x := Renombrar(rc, "Nuevo"+rc.Nombre)
			if strings.Contains(x.Codigo, rc.Firma.Nombre+"(") && !strings.Contains(x.Codigo, "Nuevo"+rc.Firma.Nombre+"(") {
				t.Errorf("%s: quedan llamadas al nombre viejo:\n%s", rc.Nombre, x.Codigo)
			}
			if _, err := FirmaDe(x.Codigo, "Nuevo"+rc.Firma.Nombre); err != nil {
				t.Errorf("%s: %v", rc.Nombre, err)
			}
		}
	}
	// a method
	a := receta(t, "Area")
	x := Renombrar(a, "Superficie")
	if !strings.Contains(x.Codigo, ") Superficie() float64") || x.Firma.Nombre != "Superficie" {
		t.Errorf("método:\n%s", x.Codigo)
	}
}

// ---- signatures ----

func TestFirmaDe(t *testing.T) {
	src := `package p

type Punto struct{ X, Y float64 }

type Celsius float64

func A(xs []int, m map[string][]int, p *Punto, c Celsius, v ...string) (int, error) { return 0, nil }
func B(ch chan int) int { return 0 }
func (p Punto) Norma() float64 { return 0 }
`
	f, err := FirmaDe(src, "A")
	if err != nil || f.Forma() != "([]int,map[string][]int,*Punto,Celsius,...string)(int,error)" || !f.Variadica {
		t.Errorf("A: %s %v", f.Forma(), err)
	}
	if _, err := FirmaDe(src, "B"); err == nil || !strings.Contains(err.Error(), "canales") {
		t.Errorf("B: %v", err)
	}
	m, err := FirmaDe(src, "Punto.Norma")
	if err != nil || m.Receptor == nil || m.Forma() != "(Punto)float64" {
		t.Errorf("Punto.Norma: %v %v", m.Forma(), err)
	}
	if _, err := FirmaDe(src, "C"); err == nil {
		t.Error("C no existe")
	}
}

// ---- toolchain: everything shipped compiles and does what it says ----

func TestRecetasConGo(t *testing.T) {
	t.Parallel()
	m := nuevoModulo(t)
	rs := Funciones()
	for i, r := range rs {
		d := nombreDir("r", i, r.Nombre)
		m.escribir(d+"/solucion.go", r.Codigo)
		m.escribir(d+"/soporte_test.go", soporteComparar)
		m.escribir(d+"/solucion_test.go", pruebaDeReceta(r))
	}
	if out, err := m.goCmd("test", "-count=1", "./..."); err != nil {
		t.Errorf("las recetas no pasan sus casos: %v\n%s", err, recortarTexto(out, 30000))
	}
	m.vet()
}

func TestPlantillasConGo(t *testing.T) {
	t.Parallel()
	m := nuevoModulo(t)
	type prueba struct {
		bin  string
		caso CasoPlantilla
		pl   string
		i    int
	}
	var pruebas []prueba
	k := 0
	for _, p := range Plantillas() {
		hechos := map[string]string{} // instance key → dir
		for i, c := range p.Casos {
			src, err := Instanciar(p, c.Huecos)
			if err != nil {
				t.Errorf("%s, caso %d: %v", p.Nombre, i+1, err)
				continue
			}
			d, ok := hechos[src]
			if !ok {
				d = nombreDir("p", k, p.Nombre)
				k++
				hechos[src] = d
				m.escribir(d+"/main.go", src)
			}
			pruebas = append(pruebas, prueba{bin: d, caso: c, pl: p.Nombre, i: i + 1})
		}
		// the defaults must work too
		if src, err := Instanciar(p, nil); err == nil {
			if _, ok := hechos[src]; !ok {
				d := nombreDir("p", k, p.Nombre)
				k++
				m.escribir(d+"/main.go", src)
			}
		} else {
			t.Errorf("%s: sin valores: %v", p.Nombre, err)
		}
	}
	dir := m.construir()
	m.vet()
	for _, pr := range pruebas {
		out, errOut, err := ejecutar(filepath.Join(dir, pr.bin), pr.caso.Caso.Args, pr.caso.Caso.Entrada)
		if err != nil && !strings.Contains(err.Error(), "exit status") {
			t.Errorf("%s, caso %d: %v", pr.pl, pr.i, err)
			continue
		}
		if !salidaCoincide(out, pr.caso.Caso) {
			t.Errorf("%s, caso %d (%s): escribió\n%s\ny esperaba (%s)\n%s\nstderr: %s", pr.pl, pr.i, pr.caso.Caso.Nota, out, pr.caso.Caso.Comparar, pr.caso.Caso.Esperado, errOut)
		}
	}
}

func TestNotasConGo(t *testing.T) {
	t.Parallel()
	m := nuevoModulo(t)
	ns := Notas()
	dirs := make([]string, len(ns))
	for i, n := range ns {
		dirs[i] = nombreDir("n", i, n.Clave)
		m.escribir(dirs[i]+"/main.go", n.Ejemplo)
	}
	dir := m.construir()
	m.vet()
	for i, n := range ns {
		out, errOut, err := ejecutar(filepath.Join(dir, dirs[i]), nil, "")
		if err != nil {
			t.Errorf("%s: %v\n%s", n.Clave, err, errOut)
			continue
		}
		if out != n.Salida {
			t.Errorf("%s: escribió\n%q\ny la nota dice\n%q", n.Clave, out, n.Salida)
		}
	}
}

// TestEnvoltorioConGo wraps every recipe that is a plain function into a program and runs its cases
// through stdin/stdout.
func TestEnvoltorioConGo(t *testing.T) {
	t.Parallel()
	m := nuevoModulo(t)
	type prueba struct {
		bin, receta string
		i           int
		caso        nucleo.CasoPrograma
	}
	var pruebas []prueba
	envueltas := 0
	for i, r := range Funciones() {
		prog, aCaso, err := Envolver(r.Codigo, r.Firma)
		if err != nil {
			if r.Firma.Receptor == nil && !strings.Contains(err.Error(), "no sé leer") {
				t.Errorf("%s: %v", r.Nombre, err)
			}
			continue
		}
		envueltas++
		d := nombreDir("e", i, r.Nombre)
		m.escribir(d+"/main.go", prog)
		for j, c := range r.Casos {
			cp, ok := aCaso(c)
			if !ok {
				continue
			}
			pruebas = append(pruebas, prueba{bin: d, receta: r.Nombre, i: j + 1, caso: cp})
		}
	}
	if envueltas < 40 {
		t.Errorf("solo se pudieron envolver %d recetas", envueltas)
	}
	dir := m.construir()
	m.vet()
	for _, pr := range pruebas {
		out, errOut, err := ejecutar(filepath.Join(dir, pr.bin), nil, pr.caso.Entrada)
		if err != nil {
			t.Errorf("%s, caso %d: %v\n%s", pr.receta, pr.i, err, errOut)
			continue
		}
		if !salidaCoincide(out, pr.caso) {
			t.Errorf("%s, caso %d: con la entrada %q escribió %q y esperaba %q (%s)", pr.receta, pr.i, pr.caso.Entrada, out, pr.caso.Esperado, pr.caso.Comparar)
		}
	}
}

func TestBusquedaDeterminista(t *testing.T) {
	a := nombresR(BuscarFunciones([]string{"numero"}, nil, 0))
	for i := 0; i < 5; i++ {
		b := nombresR(BuscarFunciones([]string{"numero"}, nil, 0))
		if strings.Join(a, ",") != strings.Join(b, ",") {
			t.Fatal("el orden cambia entre llamadas")
		}
	}
}
