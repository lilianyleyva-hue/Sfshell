package memoria

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

func abrir(t *testing.T, dir string) *Almacen {
	t.Helper()
	a, err := Abrir(dir)
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	return a
}

func reabrir(t *testing.T, a *Almacen) *Almacen {
	t.Helper()
	if err := a.Cerrar(); err != nil {
		t.Fatalf("Cerrar: %v", err)
	}
	return abrir(t, a.Dir())
}

var firmaSuma = nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}

func casosSuma(t *testing.T, filas ...[2]any) []nucleo.CasoGuardado {
	t.Helper()
	var cs []nucleo.Caso
	for _, f := range filas {
		cs = append(cs, nucleo.Caso{Entradas: []nucleo.Valor{f[0]}, Esperado: []nucleo.Valor{f[1]}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario})
	}
	gs, err := nucleo.GuardarCasos(firmaSuma, cs)
	if err != nil {
		t.Fatal(err)
	}
	return gs
}

func funcionSuma(t *testing.T) nucleo.FuncionAprendida {
	return nucleo.FuncionAprendida{
		Nombre:      "SumaPares",
		Firma:       firmaSuma,
		Codigo:      "package solucion\n\nfunc SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n%2 == 0 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total\n}\n",
		DSL:         "(sum (filter even nums))",
		Descripcion: "devuelve la suma de los números pares de nums",
		Conceptos:   []string{"sumar", "par", "numeros"},
		Palabras:    []string{"sumar", "numero", "par", "lista"},
		Casos:       casosSuma(t, [2]any{[]nucleo.Valor{1, 2, 3, 4}, 6}, [2]any{[]nucleo.Valor{}, 0}),
		Huella:      "h-sumapares",
		Nivel:       nucleo.NivelEntendido,
		Origen:      "sintesis",
	}
}

func TestAbrirCreaCarpeta(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "datos", "nyx")
	a := abrir(t, dir)
	defer a.Cerrar()
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
		t.Fatalf("carpeta: %v %v", st, err)
	}
	v, err := os.ReadFile(filepath.Join(dir, "version"))
	if err != nil || strings.TrimSpace(string(v)) != "1" {
		t.Errorf("archivo version: %q %v", v, err)
	}
	if len(a.Avisos()) != 0 {
		t.Errorf("avisos en una memoria nueva: %v", a.Avisos())
	}
	if _, err := Abrir(""); err == nil {
		t.Error("Abrir(\"\") debe fallar")
	}
}

func TestFuncionesIdaYVuelta(t *testing.T) {
	a := abrir(t, t.TempDir())
	f := funcionSuma(t)
	nombre, err := a.GuardarFuncion(f)
	if err != nil || nombre != "SumaPares" {
		t.Fatalf("GuardarFuncion = %q, %v", nombre, err)
	}
	a.Usada("SumaPares")
	a = reabrir(t, a)
	defer a.Cerrar()
	g, ok := a.Funcion("sumapares") // case-insensitive lookup
	if !ok {
		t.Fatal("no la encuentro tras reabrir")
	}
	if g.Codigo != f.Codigo || g.DSL != f.DSL || g.Firma.Go() != f.Firma.Go() || g.Usos != 1 || g.Usada.IsZero() || g.Creada.IsZero() {
		t.Errorf("función leída: %+v", g)
	}
	cs, err := nucleo.LeerCasos(g.Firma, g.Casos)
	if err != nil || len(cs) != 2 || !nucleo.Igual(cs[0].Esperado[0], 6) {
		t.Errorf("casos: %+v %v", cs, err)
	}
	if n, _, _ := a.Contar(); n != 1 {
		t.Errorf("Contar = %d", n)
	}
	if fs := a.PorHuella("h-sumapares"); len(fs) != 1 || fs[0].Nombre != "SumaPares" {
		t.Errorf("PorHuella: %+v", fs)
	}
	if err := a.OlvidarFuncion("SumaPares"); err != nil {
		t.Fatal(err)
	}
	if err := a.OlvidarFuncion("SumaPares"); err == nil {
		t.Error("olvidar dos veces debe fallar")
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	if len(a.Funciones()) != 0 {
		t.Errorf("tras olvidar: %+v", a.Funciones())
	}
}

func TestFusionPorHuella(t *testing.T) {
	a := abrir(t, t.TempDir())
	defer a.Cerrar()
	f := funcionSuma(t)
	if _, err := a.GuardarFuncion(f); err != nil {
		t.Fatal(err)
	}
	// same behaviour, different whitespace, a better level and one more case
	g := funcionSuma(t)
	g.Nombre = "SumarPares"
	g.Codigo = strings.ReplaceAll(f.Codigo, "\t", "    ")
	g.Nivel = nucleo.NivelTusEjemplos
	g.Casos = casosSuma(t, [2]any{[]nucleo.Valor{2, 2}, 4}, [2]any{[]nucleo.Valor{1, 2, 3, 4}, 6})
	g.Conceptos = []string{"sumar", "lista"}
	nombre, err := a.GuardarFuncion(g)
	if err != nil || nombre != "SumaPares" {
		t.Fatalf("la misma huella debe fusionarse en SumaPares: %q %v", nombre, err)
	}
	fs := a.Funciones()
	if len(fs) != 1 {
		t.Fatalf("hay %d funciones", len(fs))
	}
	m := fs[0]
	if m.Nivel != nucleo.NivelTusEjemplos || m.Codigo != g.Codigo || len(m.Casos) != 3 {
		t.Errorf("fusión: nivel %s, %d casos", m.Nivel, len(m.Casos))
	}
	if !reflect.DeepEqual(m.Conceptos, []string{"sumar", "par", "numeros", "lista"}) {
		t.Errorf("conceptos: %v", m.Conceptos)
	}
	// a worse level keeps the stored code
	h := funcionSuma(t)
	h.Codigo = "package solucion\n\n// otra\nfunc SumaPares(nums []int) int { return 0 }\n"
	h.Nivel = nucleo.NivelSinFallos
	if _, err := a.GuardarFuncion(h); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Funcion("SumaPares"); got.Codigo != g.Codigo || got.Nivel != nucleo.NivelTusEjemplos {
		t.Errorf("un nivel peor no debe cambiar el código: %+v", got)
	}
}

func TestChoqueDeNombres(t *testing.T) {
	a := abrir(t, t.TempDir())
	defer a.Cerrar()
	for i, quiero := range []string{"Doble", "Doble2", "Doble3"} {
		f := nucleo.FuncionAprendida{Nombre: "Doble", Firma: nucleo.Firma{Nombre: "Doble", Params: []nucleo.Param{{Nombre: "x", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}},
			Codigo: fmt.Sprintf("package solucion\n\nfunc Doble(x int) int { return x * %d }\n", i+2), Huella: fmt.Sprint("h", i)}
		nombre, err := a.GuardarFuncion(f)
		if err != nil || nombre != quiero {
			t.Errorf("GuardarFuncion %d = %q, %v; esperaba %s", i, nombre, err, quiero)
		}
	}
	if _, err := a.GuardarFuncion(nucleo.FuncionAprendida{Nombre: "X"}); err == nil {
		t.Error("sin código debe fallar")
	}
	if _, err := a.GuardarFuncion(nucleo.FuncionAprendida{Codigo: "package p"}); err == nil {
		t.Error("sin nombre debe fallar")
	}
}

func TestReemplazarFuncion(t *testing.T) {
	a := abrir(t, t.TempDir())
	defer a.Cerrar()
	f := funcionSuma(t)
	if _, err := a.GuardarFuncion(f); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		n := funcionSuma(t)
		n.Codigo = fmt.Sprintf("package solucion\n\n// versión %d\nfunc SumaPares(nums []int) int { return %d }\n", i, i)
		if err := a.ReemplazarFuncion("SumaPares", n); err != nil {
			t.Fatal(err)
		}
	}
	g, _ := a.Funcion("SumaPares")
	if len(g.Anteriores) != MaxAnteriores || !strings.Contains(g.Codigo, "versión 6") || !strings.Contains(g.Anteriores[MaxAnteriores-1], "versión 5") {
		t.Errorf("anteriores = %d, código %q", len(g.Anteriores), g.Codigo)
	}
	if !g.Creada.Equal(f.Creada) && g.Creada.IsZero() {
		t.Error("Creada debe conservarse")
	}
	if err := a.ReemplazarFuncion("NoExiste", f); err == nil {
		t.Error("reemplazar una función desconocida debe fallar")
	}
}

func TestBuscarFunciones(t *testing.T) {
	a := abrir(t, t.TempDir())
	defer a.Cerrar()
	suma := funcionSuma(t)
	pal := nucleo.FuncionAprendida{
		Nombre: "EsPalindromo", Codigo: "package solucion\n\nfunc EsPalindromo(s string) bool { return true }\n",
		Firma:     nucleo.Firma{Nombre: "EsPalindromo", Params: []nucleo.Param{{Nombre: "s", Tipo: nucleo.TString}}, Res: []nucleo.Tipo{nucleo.TBool}},
		Conceptos: []string{"palindromo", "palabra"}, Palabras: []string{"palabra", "palindromo", "decir"}, Huella: "h-pal",
	}
	impares := funcionSuma(t)
	impares.Nombre, impares.Huella = "SumaImpares", "h-impares"
	impares.Conceptos = []string{"sumar", "impar", "numeros"}
	impares.Palabras = []string{"sumar", "numero", "impar"}
	for _, f := range []nucleo.FuncionAprendida{suma, pal, impares} {
		if _, err := a.GuardarFuncion(f); err != nil {
			t.Fatal(err)
		}
	}
	r := a.BuscarFunciones([]string{"sumar", "par", "numeros"}, []string{"sumar", "numero", "par", "lista"}, "", 3)
	if len(r) < 2 || r[0].F.Nombre != "SumaPares" || r[0].Puntos < 0.99 || r[1].F.Nombre != "SumaImpares" {
		t.Fatalf("resultado: %+v", nombresPuntuadas(r))
	}
	if r[1].Puntos >= r[0].Puntos {
		t.Error("SumaImpares debe puntuar menos")
	}
	if r := a.BuscarFunciones([]string{"palindromo"}, nil, "(string)bool", 5); len(r) != 1 || r[0].F.Nombre != "EsPalindromo" {
		t.Errorf("filtro por forma: %+v", nombresPuntuadas(r))
	}
	if r := a.BuscarFunciones([]string{"palindromo"}, nil, "([]int)int", 5); len(r) != 0 {
		t.Errorf("la forma no coincide: %+v", nombresPuntuadas(r))
	}
	// a function without Palabras is found by the words of its name and description
	inv := nucleo.FuncionAprendida{Nombre: "InvertirTexto", Codigo: "package solucion\n\nfunc InvertirTexto(s string) string { return s }\n",
		Firma: pal.Firma, Descripcion: "da la vuelta a una frase", Huella: "h-inv"}
	if _, err := a.GuardarFuncion(inv); err != nil {
		t.Fatal(err)
	}
	if r := a.BuscarFunciones(nil, []string{"invertir", "frase"}, "", 1); len(r) != 1 || r[0].F.Nombre != "InvertirTexto" {
		t.Errorf("por palabras del nombre: %+v", nombresPuntuadas(r))
	}
	if r := a.BuscarFunciones(nil, nil, "", 5); len(r) != 0 {
		t.Errorf("sin consulta no hay resultados: %+v", nombresPuntuadas(r))
	}
}

func nombresPuntuadas(r []Puntuada) []string {
	var out []string
	for _, p := range r {
		out = append(out, fmt.Sprintf("%s=%.3f", p.F.Nombre, p.Puntos))
	}
	return out
}

func TestHechosYReglas(t *testing.T) {
	a := abrir(t, t.TempDir())
	id1, err := a.GuardarHecho(nucleo.Hecho{Sujeto: "toby", Relacion: "es_un", Objeto: "perro", Texto: "Toby es un perro"})
	if err != nil || id1 != "h1" {
		t.Fatalf("GuardarHecho = %q, %v", id1, err)
	}
	id2, _ := a.GuardarHecho(nucleo.Hecho{Sujeto: "luna", Relacion: "es_un", Objeto: "gato", Texto: "Luna es una gata"})
	if again, _ := a.GuardarHecho(nucleo.Hecho{Sujeto: "Toby", Relacion: "es_un", Objeto: "perro", Texto: "toby es perro"}); again != id1 {
		t.Errorf("el mismo dato no se repite: %s", again)
	}
	if _, err := a.GuardarHecho(nucleo.Hecho{Sujeto: "x"}); err == nil {
		t.Error("un dato incompleto debe fallar")
	}
	rid, err := a.GuardarRegla(nucleo.Regla{
		Si:       []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: "perro"}},
		Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "es_un", Objeto: "mamifero"},
		Texto:    "todos los perros son mamíferos",
	})
	if err != nil || rid != "r1" {
		t.Fatalf("GuardarRegla = %q, %v", rid, err)
	}
	if again, _ := a.GuardarRegla(nucleo.Regla{Si: []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: "Perro"}}, Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "es_un", Objeto: "mamifero"}}); again != rid {
		t.Errorf("la misma regla no se repite: %s", again)
	}
	a = reabrir(t, a)
	hs := a.Hechos("TOBY")
	if len(hs) != 1 || hs[0].ID != id1 || hs[0].Texto != "toby es perro" || hs[0].Fuente != "usuario" || hs[0].Confianza != 1 {
		t.Errorf("Hechos(toby) = %+v", hs)
	}
	if len(a.Todos()) != 2 || len(a.Reglas()) != 1 || a.Reglas()[0].Si[0].Objeto != "perro" {
		t.Errorf("Todos=%+v Reglas=%+v", a.Todos(), a.Reglas())
	}
	// BM25 search
	if r := a.BuscarHechos("¿qué es Luna?", 5); len(r) != 1 || r[0].ID != id2 {
		t.Errorf("BuscarHechos: %+v", r)
	}
	// contradiction: the newest statement wins
	id3, _ := a.GuardarHecho(nucleo.Hecho{Sujeto: "luna", Relacion: "es_un", Objeto: "gato", Negado: true, Texto: "Luna no es una gata"})
	if hs := a.Hechos("luna"); len(hs) != 1 || hs[0].ID != id3 || !hs[0].Negado || id3 == id2 {
		t.Errorf("contradicción: %+v", hs)
	}
	// tombstones survive reopening
	if err := a.OlvidarHecho(id1); err != nil {
		t.Fatal(err)
	}
	if err := a.OlvidarHecho(id1); err == nil {
		t.Error("olvidar dos veces debe fallar")
	}
	if err := a.OlvidarRegla(rid); err != nil {
		t.Fatal(err)
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	if hs := a.Hechos("toby"); len(hs) != 0 {
		t.Errorf("el dato olvidado volvió: %+v", hs)
	}
	if len(a.Reglas()) != 0 {
		t.Errorf("la regla olvidada volvió: %+v", a.Reglas())
	}
	// new ids never reuse old ones
	id4, _ := a.GuardarHecho(nucleo.Hecho{Sujeto: "rex", Relacion: "es_un", Objeto: "perro"})
	if id4 == id1 || id4 == id2 || id4 == id3 {
		t.Errorf("id repetido: %s", id4)
	}
}

func TestCompactacion(t *testing.T) {
	dir := t.TempDir()
	a := abrir(t, dir)
	var ids []string
	for i := 0; i < 10; i++ {
		id, err := a.GuardarHecho(nucleo.Hecho{Sujeto: fmt.Sprint("s", i), Relacion: "es", Objeto: "x"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids[:6] {
		if err := a.OlvidarHecho(id); err != nil {
			t.Fatal(err)
		}
	}
	a = reabrir(t, a) // 16 lines, 12 dead: compacted
	defer a.Cerrar()
	datos, err := os.ReadFile(filepath.Join(dir, "hechos.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lineas := strings.Split(strings.TrimSpace(string(datos)), "\n")
	if len(lineas) != 5 || !strings.Contains(lineas[0], `"version":1`) || strings.Contains(string(datos), "borrado") {
		t.Errorf("tras compactar:\n%s", datos)
	}
	if len(a.Todos()) != 4 {
		t.Errorf("quedan %d datos", len(a.Todos()))
	}
	if id, _ := a.GuardarHecho(nucleo.Hecho{Sujeto: "nuevo", Relacion: "es", Objeto: "x"}); id != "h11" {
		t.Errorf("el contador de ids se pierde al compactar: %s", id)
	}
}

func TestLineaCortadaJSONL(t *testing.T) {
	dir := t.TempDir()
	contenido := `{"version":1}
{"id":"h1","sujeto":"toby","relacion":"es_un","objeto":"perro","texto":"","fuente":"usuario","confianza":1,"fecha":"2026-01-01T00:00:00Z","usos":0}
esto no es json
{"id":"h2","sujeto":"luna","relacion":"es_un","objeto":"gato","texto":"","fuente":"usuario","confianza":1,"fecha":"2026-01-01T00:00:00Z","usos":0}
{"id":"h3","sujeto":"re`
	if err := os.WriteFile(filepath.Join(dir, "hechos.jsonl"), []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
	a := abrir(t, dir)
	if len(a.Todos()) != 2 {
		t.Fatalf("datos: %+v", a.Todos())
	}
	if av := a.Avisos(); len(av) != 1 || !strings.Contains(av[0], "hechos.jsonl") || !strings.Contains(av[0], "2 línea") {
		t.Errorf("avisos: %v", av)
	}
	id, err := a.GuardarHecho(nucleo.Hecho{Sujeto: "rex", Relacion: "es_un", Objeto: "perro"})
	if err != nil || id != "h3" {
		t.Fatalf("%q %v", id, err)
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	if len(a.Todos()) != 3 || len(a.Avisos()) != 0 {
		t.Errorf("tras reabrir: %+v %v", a.Todos(), a.Avisos())
	}
}

func TestLineaCortadaSinCompactar(t *testing.T) {
	// a truncated last line is tolerated by the reader itself
	dir := t.TempDir()
	ruta := filepath.Join(dir, "x.jsonl")
	os.WriteFile(ruta, []byte("{\"version\":1}\n{\"id\":\"p1\",\"texto\":\"a\"}\n{\"id\":\"p2\",\"tex"), 0o600)
	l, err := leerJSONL(ruta)
	if err != nil || len(l.lineas) != 1 || l.malas != 1 || !l.sinSalto || l.cabecera.Version != 1 {
		t.Errorf("%+v %v", l, err)
	}
}

func TestEscrituraInterrumpida(t *testing.T) {
	dir := t.TempDir()
	a := abrir(t, dir)
	if _, err := a.GuardarFuncion(funcionSuma(t)); err != nil {
		t.Fatal(err)
	}
	antes, _ := os.ReadFile(filepath.Join(dir, "biblioteca.json"))
	defer func() { antesDeRenombrar = func(string, string) error { return nil } }()
	antesDeRenombrar = func(tmp, final string) error { return errors.New("corte de luz") }
	otra := funcionSuma(t)
	otra.Nombre, otra.Huella = "Otra", "h-otra"
	if _, err := a.GuardarFuncion(otra); err == nil {
		t.Fatal("la escritura interrumpida debe dar error")
	}
	if _, err := os.Stat(filepath.Join(dir, "biblioteca.json.tmp")); err != nil {
		t.Fatalf("el .tmp debería seguir ahí: %v", err)
	}
	if len(a.Funciones()) != 1 {
		t.Errorf("la memoria no debe quedarse con lo que no se guardó: %d", len(a.Funciones()))
	}
	despues, _ := os.ReadFile(filepath.Join(dir, "biblioteca.json"))
	if string(antes) != string(despues) {
		t.Error("el archivo viejo cambió")
	}
	antesDeRenombrar = func(string, string) error { return nil }
	a.Cerrar()
	b := abrir(t, dir)
	defer b.Cerrar()
	if fs := b.Funciones(); len(fs) != 1 || fs[0].Nombre != "SumaPares" {
		t.Errorf("tras el corte: %+v", fs)
	}
	if _, err := os.Stat(filepath.Join(dir, "biblioteca.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("Abrir debe quitar el .tmp abandonado: %v", err)
	}
	if len(b.Avisos()) != 0 {
		t.Errorf("avisos: %v", b.Avisos())
	}
}

func TestBibliotecaRota(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "biblioteca.json"), []byte(`{"version":1,"funciones":[{"nombre":"X",`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lexico.json"), []byte(`{"version":7,"confirmadas":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a := abrir(t, dir)
	defer a.Cerrar()
	if len(a.Funciones()) != 0 {
		t.Error("una biblioteca rota debe empezar vacía")
	}
	rotos, _ := filepath.Glob(filepath.Join(dir, "biblioteca.json.roto-*"))
	if len(rotos) != 1 || !strings.HasPrefix(filepath.Base(rotos[0]), "biblioteca.json.roto-"+time.Now().Format("20060102")) {
		t.Errorf("archivo apartado: %v", rotos)
	}
	if _, err := os.Stat(filepath.Join(dir, "biblioteca.json")); !os.IsNotExist(err) {
		t.Error("el archivo roto debe haberse movido")
	}
	av := strings.Join(a.Avisos(), "\n")
	if !strings.Contains(av, "biblioteca.json estaba dañado") || !strings.Contains(av, "lexico.json es de una versión más nueva") {
		t.Errorf("avisos: %v", av)
	}
	// the store keeps working
	if _, err := a.GuardarFuncion(funcionSuma(t)); err != nil {
		t.Fatal(err)
	}
	if err := a.EnsenarPalabra("capicúa", "palíndromo"); err != nil {
		t.Fatal(err)
	}
}

func TestLexicoYVerbos(t *testing.T) {
	a := abrir(t, t.TempDir())
	if a.ProponerPalabra("Pares", "par") {
		t.Error("la primera propuesta no confirma")
	}
	if !a.ProponerPalabra("pares", "par") {
		t.Error("la segunda propuesta confirma")
	}
	if !a.ProponerPalabra("pares", "par") {
		t.Error("ya estaba confirmada")
	}
	if err := a.EnsenarPalabra("Capicúa", "palíndromo"); err != nil {
		t.Fatal(err)
	}
	if a.ProponerPalabra("capicua", "numero") {
		t.Error("una palabra enseñada no cambia por propuestas")
	}
	a.ProponerPalabra("suelto", "numero") // one proposal survives reopening
	if err := a.Aprender("Trocar", "restar"); err != nil {
		t.Fatal(err)
	}
	if err := a.Aprender("zumbar", "bailar"); err == nil {
		t.Error("un efecto desconocido debe fallar")
	}
	if err := a.EnsenarPalabra("", "x"); err == nil {
		t.Error("falta la palabra")
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	quiero := map[string]string{"pares": "par", "capicua": "palindromo"}
	if p := a.Palabras(); !reflect.DeepEqual(p, quiero) {
		t.Errorf("Palabras = %v", p)
	}
	if !a.ProponerPalabra("suelto", "numero") {
		t.Error("la propuesta guardada cuenta tras reabrir")
	}
	if e, ok := a.Efecto("trocar"); !ok || e != "restar" {
		t.Errorf("Efecto = %q %v", e, ok)
	}
	if _, ok := a.Efecto("volar"); ok {
		t.Error("verbo desconocido")
	}
	if _, _, n := a.Contar(); n != 3 {
		t.Errorf("Contar palabras = %d", n)
	}
	if err := a.OlvidarPalabra("capicúa"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Palabras()["capicua"]; ok {
		t.Error("olvidada")
	}
}

func TestContador(t *testing.T) {
	dir := t.TempDir()
	a := abrir(t, dir)
	c := a.Contador()
	if c.Tasa("regla:importar") != 0.5 || c.Usos("regla:importar") != 0 {
		t.Fatal("una clave nueva vale 0.5")
	}
	anterior := c.Tasa("regla:importar")
	for i := 0; i < 5; i++ {
		c.Exito("regla:importar", true)
		ahora := c.Tasa("regla:importar")
		if ahora <= anterior {
			t.Errorf("la tasa debe subir: %v → %v", anterior, ahora)
		}
		anterior = ahora
	}
	if math.Abs(anterior-6.0/7.0) > 1e-12 || c.Usos("regla:importar") != 5 {
		t.Errorf("tras 5 éxitos: %v (%d usos)", anterior, c.Usos("regla:importar"))
	}
	c.Exito("asoc:par:esPar", true)
	c.Exito("asoc:par:esPar", true)
	c.Exito("asoc:par:resto", false)
	c.Exito("asoc:parcial:x", true)
	as := a.Asociaciones("par")
	if len(as) != 2 || as["esPar"] != 0.75 || as["resto"] != 1.0/3.0 {
		t.Errorf("Asociaciones = %v", as)
	}
	a = reabrir(t, a) // Cerrar writes the counters
	defer a.Cerrar()
	if a.Contador().Usos("regla:importar") != 5 || a.Contador().Tasa("asoc:par:resto") != 1.0/3.0 {
		t.Error("los contadores no sobrevivieron")
	}
}

func TestContadorVolcadoPeriodico(t *testing.T) {
	viejo := IntervaloVolcado
	IntervaloVolcado = 50 * time.Millisecond
	defer func() { IntervaloVolcado = viejo }()
	dir := t.TempDir()
	a := abrir(t, dir)
	defer a.Cerrar()
	a.Contador().Exito("estrategia:mates.ecuacion", true)
	ruta := filepath.Join(dir, "aprendizaje.json")
	limite := time.Now().Add(5 * time.Second)
	for {
		if datos, err := os.ReadFile(ruta); err == nil && strings.Contains(string(datos), `"estrategia:mates.ecuacion":[1,1]`) {
			break
		}
		if time.Now().After(limite) {
			t.Fatal("los contadores no se escribieron solos")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestContraejemplos(t *testing.T) {
	a := abrir(t, t.TempDir())
	clave := ClaveContraejemplo([]string{"sumar", "par", "sumar"}, firmaSuma)
	if clave != "par,sumar|([]int)int" {
		t.Errorf("clave = %q", clave)
	}
	c := nucleo.Caso{Entradas: []nucleo.Valor{[]nucleo.Valor{2}}, Esperado: []nucleo.Valor{2}}
	if err := a.GuardarContraejemplo(clave, firmaSuma, c); err != nil {
		t.Fatal(err)
	}
	c2 := nucleo.Caso{Entradas: []nucleo.Valor{[]nucleo.Valor{2}}, Esperado: []nucleo.Valor{4}} // same input, newer answer
	if err := a.GuardarContraejemplo(clave, firmaSuma, c2); err != nil {
		t.Fatal(err)
	}
	if err := a.GuardarContraejemplo(clave, firmaSuma, nucleo.Caso{Entradas: []nucleo.Valor{"x"}}); err == nil {
		t.Error("un caso que no encaja con la firma debe fallar")
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	cs := a.Contraejemplos(clave, firmaSuma)
	if len(cs) != 1 || !nucleo.Igual(cs[0].Esperado[0], 4) || cs[0].Origen != nucleo.OrigenContraejemplo || cs[0].Expectativa != nucleo.EspUsuario {
		t.Errorf("contraejemplos: %+v", cs)
	}
	if cs := a.Contraejemplos("otra", firmaSuma); len(cs) != 0 {
		t.Errorf("clave desconocida: %+v", cs)
	}
}

func TestPistas(t *testing.T) {
	a := abrir(t, t.TempDir())
	p := nucleo.Pistas{Llamadas: map[string]int{"strings.Fields": 3}, Operadores: map[string]int{"%": 2}, Constantes: []nucleo.Valor{2, 0.5, " ", true, []nucleo.Valor{1}}}
	if err := a.GuardarPistas("golang sum even numbers", p); err != nil {
		t.Fatal(err)
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	q, ok := a.Pistas("Golang  SUM even numbers")
	if !ok || q.Llamadas["strings.Fields"] != 3 || q.Operadores["%"] != 2 || len(q.Constantes) != 4 {
		t.Fatalf("pistas: %+v %v", q, ok)
	}
	if _, esInt := q.Constantes[0].(int); !esInt || q.Constantes[1] != 0.5 || q.Constantes[2] != " " || q.Constantes[3] != true {
		t.Errorf("constantes: %#v", q.Constantes)
	}
	if _, ok := a.Pistas("otra cosa"); ok {
		t.Error("consulta desconocida")
	}
}

func TestHistorial(t *testing.T) {
	a := abrir(t, t.TempDir())
	defer a.Cerrar()
	hoy := time.Now()
	if err := a.Registrar(nucleo.Episodio{Fecha: hoy, Sesion: "s1", Tarea: "t1", Texto: "suma pares", Intencion: nucleo.ICrearFuncion, Metodo: "sintesis", Exito: true, Nivel: nucleo.NivelEntendido, Ms: 900}); err != nil {
		t.Fatal(err)
	}
	if err := a.Registrar(nucleo.Episodio{Tarea: "t2", Texto: "hola", Exito: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.Opinar(nucleo.Opinion{Sesion: "s1", Tarea: "t1", Correcto: false, Ejemplo: "con [2] debe dar 0"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Opinar(nucleo.Opinion{}); err == nil {
		t.Error("una opinión sin tarea debe fallar")
	}
	ruta := filepath.Join(a.Dir(), "historial", hoy.Format("2006-01-02")+".jsonl")
	datos, err := os.ReadFile(ruta)
	if err != nil || !strings.HasPrefix(string(datos), `{"version":1}`) || !strings.Contains(string(datos), `"opinion":"mal"`) {
		t.Fatalf("archivo: %s %v", datos, err)
	}
	es, err := a.Historial(hoy)
	if err != nil || len(es) != 2 || es[0].Opinion != "mal" || es[1].Opinion != "" || es[0].Metodo != "sintesis" {
		t.Errorf("Historial = %+v %v", es, err)
	}
	if es, err := a.Historial(hoy.AddDate(0, 0, -3)); err != nil || len(es) != 0 {
		t.Errorf("día sin historial: %+v %v", es, err)
	}
}

func TestPendientes(t *testing.T) {
	a := abrir(t, t.TempDir())
	if err := a.AgregarPendiente(Pendiente{Texto: "¿cuántos primos hay hasta 100?", Motivo: "sin_tiempo"}); err != nil {
		t.Fatal(err)
	}
	if err := a.AgregarPendiente(Pendiente{Texto: "cuantos primos hay hasta 100", Motivo: "no_entiendo"}); err != nil {
		t.Fatal(err)
	}
	if err := a.AgregarPendiente(Pendiente{Texto: "haz un juego de ajedrez"}); err != nil {
		t.Fatal(err)
	}
	if err := a.AgregarPendiente(Pendiente{}); err == nil {
		t.Error("vacía")
	}
	ps := a.Pendientes()
	if len(ps) != 2 || ps[0].ID != "p1" || ps[0].Intentos != 2 || ps[0].Motivo != "no_entiendo" || ps[0].Fecha.IsZero() || ps[1].Intentos != 1 {
		t.Fatalf("pendientes: %+v", ps)
	}
	if err := a.ResolverPendiente("p1"); err != nil {
		t.Fatal(err)
	}
	if err := a.ResolverPendiente("p1"); err == nil {
		t.Error("resolver dos veces debe fallar")
	}
	a = reabrir(t, a)
	defer a.Cerrar()
	ps = a.Pendientes()
	if len(ps) != 1 || ps[0].Texto != "haz un juego de ajedrez" {
		t.Errorf("tras reabrir: %+v", ps)
	}
	ps[0].Intentos = 5
	if err := a.AgregarPendiente(ps[0]); err != nil {
		t.Fatal(err)
	}
	if got := a.Pendientes(); len(got) != 1 || got[0].Intentos != 5 {
		t.Errorf("reemplazo por id: %+v", got)
	}
}

func TestCerrado(t *testing.T) {
	a := abrir(t, t.TempDir())
	if err := a.Cerrar(); err != nil {
		t.Fatal(err)
	}
	if err := a.Cerrar(); err != nil {
		t.Errorf("cerrar dos veces: %v", err)
	}
	if _, err := a.GuardarFuncion(funcionSuma(t)); !errors.Is(err, ErrCerrado) {
		t.Errorf("GuardarFuncion tras Cerrar: %v", err)
	}
	if _, err := a.GuardarHecho(nucleo.Hecho{Sujeto: "a", Relacion: "es", Objeto: "b"}); !errors.Is(err, ErrCerrado) {
		t.Errorf("GuardarHecho tras Cerrar: %v", err)
	}
	if err := a.Registrar(nucleo.Episodio{Tarea: "t"}); !errors.Is(err, ErrCerrado) {
		t.Errorf("Registrar tras Cerrar: %v", err)
	}
	a.Contador().Exito("x", true) // must not panic or start timers
	a.Usada("nada")
}

func TestEscritoresConcurrentes(t *testing.T) {
	dir := t.TempDir()
	a := abrir(t, dir)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f := funcionSuma(t)
			f.Nombre = fmt.Sprintf("F%d", i)
			f.Huella = fmt.Sprintf("h%d", i)
			if _, err := a.GuardarFuncion(f); err != nil {
				t.Error(err)
			}
			if _, err := a.GuardarHecho(nucleo.Hecho{Sujeto: fmt.Sprint("s", i), Relacion: "es", Objeto: "x"}); err != nil {
				t.Error(err)
			}
			a.Contador().Exito("componente:suma", i%2 == 0)
			a.ProponerPalabra(fmt.Sprint("w", i%10), "numero")
			if err := a.Registrar(nucleo.Episodio{Tarea: fmt.Sprint("t", i), Exito: true}); err != nil {
				t.Error(err)
			}
			if err := a.AgregarPendiente(Pendiente{Texto: fmt.Sprint("pregunta ", i)}); err != nil {
				t.Error(err)
			}
			a.Usada("SumaPares")
			_ = a.BuscarFunciones([]string{"sumar"}, []string{"suma"}, "", 3)
			_ = a.Hechos("s1")
			_ = a.Palabras()
			_, _, _ = a.Contar()
		}(i)
	}
	wg.Wait()
	a = reabrir(t, a)
	defer a.Cerrar()
	f, h, p := a.Contar()
	if f != 100 || h != 100 || p != 10 {
		t.Errorf("Contar = %d %d %d", f, h, p)
	}
	if n := a.Contador().Usos("componente:suma"); n != 100 {
		t.Errorf("Usos = %d", n)
	}
	if len(a.Pendientes()) != 100 {
		t.Errorf("pendientes = %d", len(a.Pendientes()))
	}
	es, err := a.Historial(time.Now())
	if err != nil || len(es) != 100 {
		t.Errorf("historial = %d %v", len(es), err)
	}
	if len(a.Avisos()) != 0 {
		t.Errorf("avisos: %v", a.Avisos())
	}
}

func TestTokens(t *testing.T) {
	if got := tokensNombre("SumaParesHTTPRápido_x2"); !reflect.DeepEqual(got, []string{"suma", "pares", "http", "rapido", "x2"}) {
		t.Errorf("tokensNombre = %v", got)
	}
	if got := tokens("¿Cuántos números PARES hay en la lista?"); !reflect.DeepEqual(got, []string{"cuantos", "numeros", "pares", "hay", "lista"}) {
		t.Errorf("tokens = %v", got)
	}
	if j := jaccard([]string{"a", "b"}, []string{"b", "c"}); math.Abs(j-1.0/3.0) > 1e-12 {
		t.Errorf("jaccard = %v", j)
	}
}

// The shapes other leaves receive at construction (§4.10 mates.Verbos, §4.11 logica.BaseHechos and
// logica.Guardar): Almacen must satisfy them without adapters.
var (
	_ interface {
		Hechos(sujeto string) []nucleo.Hecho
		Todos() []nucleo.Hecho
		Reglas() []nucleo.Regla
	} = (*Almacen)(nil)
	_ interface {
		GuardarHecho(h nucleo.Hecho) (string, error)
		GuardarRegla(r nucleo.Regla) (string, error)
	} = (*Almacen)(nil)
	_ interface {
		Efecto(verbo string) (string, bool)
		Aprender(verbo, efecto string) error
	} = (*Almacen)(nil)
)
