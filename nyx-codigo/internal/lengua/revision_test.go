package lengua

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math/big"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Regression tests for the problems found in the review of this package.

// firmaCompila type-checks "func … { panic(0) }" built from f.
func firmaCompila(t *testing.T, f *nucleo.Firma) {
	t.Helper()
	src := "package p\n" + f.Go() + " { panic(0) }\n"
	fs := token.NewFileSet()
	arch, err := parser.ParseFile(fs, "p.go", src, 0)
	if err != nil {
		t.Errorf("%s: no se puede leer: %v", f.Go(), err)
		return
	}
	conf := types.Config{Error: func(error) {}}
	if _, err := conf.Check("p", fs, []*ast.File{arch}, nil); err != nil {
		t.Errorf("%s: no compila: %v", f.Go(), err)
	}
}

func TestRevisionNombresDeParametrosDistintos(t *testing.T) {
	casos := map[string]string{
		"(3, 5, true)":            "func Funcion(a, b int, ok bool) int",
		"(1, 2, 3, 'x')":          "func Funcion(a, b, c int, r rune) int",
		"(true, 3, 5)":            "func Funcion(ok bool, a, b int) int",
		`("a", "b", 's', 3)`:      "func Funcion(s, t string, c rune, n int) int",
		"(1.5, 2.5, 3, 4)":        "func Funcion(x, y float64, a, b int) int",
		`([1], [2], "s", "t", 4)`: "func Funcion(nums1, nums2 []int, s, t string, n int) int",
	}
	for entrada, want := range casos {
		_, f, err := TiparEjemplos([][2]string{{entrada, "8"}}, nil)
		if err != nil {
			t.Errorf("%s: %v", entrada, err)
			continue
		}
		if got := f.Go(); got != want {
			t.Errorf("%s → %s, esperaba %s", entrada, got, want)
		}
		firmaCompila(t, f)
	}
	// every combination of up to four basic types gets distinct names
	basicos := []nucleo.Tipo{nucleo.TInt, nucleo.TBool, nucleo.TRune, nucleo.TString, nucleo.TFloat,
		nucleo.ListaDe(nucleo.TInt), nucleo.MapaDe(nucleo.TString, nucleo.TInt)}
	var rec func(ts []nucleo.Tipo)
	rec = func(ts []nucleo.Tipo) {
		if len(ts) > 0 {
			visto := map[string]bool{}
			for _, p := range paramsPorTipo(ts) {
				if visto[p.Nombre] || p.Nombre == "" {
					t.Fatalf("%v: nombre repetido o vacío %q", ts, p.Nombre)
				}
				visto[p.Nombre] = true
			}
		}
		if len(ts) == 4 {
			return
		}
		for _, b := range basicos {
			rec(append(append([]nucleo.Tipo(nil), ts...), b))
		}
	}
	rec(nil)
}

func TestRevisionFirmasDeMarco(t *testing.T) {
	casos := map[string]string{
		"función que cuente cuántas veces aparece cada palabra":       "func FrecuenciasPalabras(s string) map[string]int",
		"función que cuente cuántas veces aparece cada letra":         "func FrecuenciasLetras(s string) map[rune]int",
		"función que devuelva la letra más repetida":                  "func LetraMasRepetida(s string) rune",
		"función que devuelva la palabra más repetida de una frase":   "func PalabraMasRepetida(frase string) string",
		"función que devuelva el número más repetido de una lista":    "func NumeroMasRepetido(nums []int) int",
		"función que quite los repetidos de una lista sin repetir":    "func SinRepetir(nums []int) []int",
		"función que devuelva la letra que más se repite en un texto": "func LetraMasRepetida(texto string) rune",
	}
	for texto, want := range casos {
		p := Analizar(texto, "", "", nil, nil)
		if p.Firma == nil {
			t.Errorf("%q: sin firma", texto)
			continue
		}
		if got := p.Firma.Go(); got != want {
			t.Errorf("%q → %s, esperaba %s", texto, got, want)
		}
		firmaCompila(t, p.Firma)
	}
	// a follow-up keeps the one-element output of "la letra más repetida"
	a := Analizar("función que devuelva la letra más repetida", "", "", nil, nil)
	b := Analizar("ahora de una frase", "", "", nil, nil)
	if r := Fusionar(a, b); r.Firma == nil || r.Firma.Res[0].Go() != "rune" {
		t.Errorf("tras fusionar: %+v", r.Firma)
	}
}

func TestRevisionSeguimientosFalsos(t *testing.T) {
	cod := &nucleo.Contexto{Intencion: nucleo.ICrearFuncion, Texto: "haz una función que sume los pares",
		Codigo: "func SumaPares(nums []int) int { return 0 }", Firma: &nucleo.Firma{Nombre: "SumaPares"}}
	nuevas := []string{
		"si es de noche hace frío; es de noche. ¿hace frío?",
		"si son las 5 Ana sale; son las 5. ¿sale Ana?",
		"no entiendo los punteros, explícame con un ejemplo",
		"si fuera rico compraría una casa",
		"si llueve el suelo se moja; llueve. ¿se moja?",
		"no sé cómo se usa un mapa en Go",
		"y qué es una goroutine? explícamelo con detalle y con ejemplos de código",
		"si son las 5 Ana sale",
		"si es de noche",
		"si llueve me mojo",
	}
	for _, s := range nuevas {
		if sg, ok := DetectarSeguimiento(s, cod); ok {
			t.Errorf("%q no es un seguimiento, salió %q", s, sg.Tipo)
		}
	}
	siguen := map[string]string{
		"¿y si fuera 13?":              "cambiar_numero",
		"si fueran 20 personas":        "cambiar_numero",
		"¿y si fuera con palabras?":    "modificar",
		"no, con [2] debe dar 0":       "esta_mal",
		"no funciona":                  "esta_mal",
		"está mal":                     "esta_mal",
		"no, no es eso":                "esta_mal",
		"y si es [1,2,3]":              "probar_con",
		"ahora con impares":            "modificar",
		"no entiendo":                  "mas_simple",
		"falla con la lista vacía":     "esta_mal",
		"incorrecto, con [1,1] da 1":   "esta_mal",
		"ahora que devuelva el mínimo": "modificar",
		"y para palabras":              "modificar",
		"pruébala con [3,3]":           "probar_con",
		"explícalo":                    "explicar_eso",
	}
	for s, want := range siguen {
		if sg, ok := DetectarSeguimiento(s, cod); !ok || sg.Tipo != want {
			t.Errorf("%q → %q (%v), esperaba %q", s, sg.Tipo, ok, want)
		}
	}
	if _, ok := DetectarSeguimiento(strings.Repeat("y con 5 ", 200), cod); ok {
		t.Errorf("un texto largo no es un seguimiento")
	}
}

func TestRevisionFusionarDesplazamientos(t *testing.T) {
	comprobar := func(r *nucleo.Pregunta) {
		t.Helper()
		for _, tk := range r.Tokens {
			if tk.Desde < 0 || tk.Hasta > len(r.Texto) || tk.Desde > tk.Hasta {
				t.Errorf("token %q [%d:%d] fuera de %q", tk.Texto, tk.Desde, tk.Hasta, r.Texto)
				continue
			}
			if got := r.Texto[tk.Desde:tk.Hasta]; got != tk.Texto {
				t.Errorf("token %q [%d:%d] apunta a %q en %q", tk.Texto, tk.Desde, tk.Hasta, got, r.Texto)
			}
		}
		for _, n := range r.Numeros {
			if n.Desde < 0 || n.Hasta > len(r.Texto) || r.Texto[n.Desde:n.Hasta] != n.Texto {
				t.Errorf("número %q [%d:%d] mal en %q", n.Texto, n.Desde, n.Hasta, r.Texto)
			}
		}
	}
	a := Analizar("haz una función que sume los números pares", "", "", nil, nil)
	b := Analizar("ahora con impares", "", "", nil, nil)
	r := Fusionar(a, b)
	comprobar(r)
	if r.Texto != "haz una función que sume los números pares; ahora con impares" {
		t.Errorf("texto: %q", r.Texto)
	}
	if r.Resto != r.Texto || r.Normal != nucleo.Normalizar(r.Texto) {
		t.Errorf("resto %q normal %q", r.Resto, r.Normal)
	}
	// numbers of the new request, quoted text, and leading spaces
	a = Analizar("  función que devuelva los números mayores que 5 de una lista", "", "", nil, nil)
	b = Analizar(`ahora mayores que 10 y que empiecen por "x"`, "", "", nil, nil)
	r = Fusionar(a, b)
	comprobar(r)
	if len(r.Numeros) != 1 || r.Numeros[0].Valor.Cmp(big.NewRat(10, 1)) != 0 {
		t.Errorf("números: %+v", r.Numeros)
	}
	// the request with code: Resto keeps only the text
	a = Analizar("explica esto:\n```go\nfunc F() int { return 1 }\n```", "", "", nil, nil)
	b = Analizar("ahora más simple", "", "", nil, nil)
	r = Fusionar(a, b)
	comprobar(r)
	if strings.Contains(r.Resto, "func F") || !strings.Contains(r.Resto, "ahora más simple") {
		t.Errorf("resto con código: %q", r.Resto)
	}
}

func TestRevisionTextosPatologicosLineales(t *testing.T) {
	// The time must grow linearly: four times the text may cost at most about four times as much
	// (a quadratic scan costs sixteen). The best of three runs is taken to smooth the noise, and the
	// fixed slack absorbs a busy machine (the whole suite runs packages in parallel); the absolute
	// bound only catches gross regressions (the race detector makes everything slower).
	escala := time.Duration(1)
	if conRace {
		escala = 15
	}
	trabajo := func(s string) time.Duration {
		mejor := time.Duration(1 << 62)
		for k := 0; k < 3; k++ {
			ini := time.Now()
			Tokenizar(s)
			ExtraerEjemplos(s)
			mejor = min(mejor, time.Since(ini))
		}
		return mejor
	}
	patrones := []struct {
		p string
		n int
	}{
		{"[", 6_000}, {"(", 6_000}, {"«", 3_000}, {"“", 3_000}, {`["`, 3_000}, {`[" \"`, 2_000},
		{"1,", 3_000}, {"cuantas veces cada ", 400}, {"a -> b, ", 1_000}, {"(1, ", 2_000},
	}
	for _, c := range patrones {
		t1 := trabajo(strings.Repeat(c.p, c.n))
		grande := strings.Repeat(c.p, 4*c.n)
		t4 := trabajo(grande)
		if t4 > 8*t1+40*time.Millisecond*escala {
			t.Errorf("%q: ×%d tardó %v y ×%d tardó %v: no crece de forma lineal", c.p, c.n, t1, 4*c.n, t4)
		}
		ini := time.Now()
		Analizar(grande, "", "", nil, nil)
		if d := time.Since(ini); d > time.Second*escala {
			t.Errorf("Analizar(%q×%d) tardó %v", c.p, 4*c.n, d)
		}
	}
	// the linear scanners agree with the direct ones on ordinary text
	for _, s := range []string{`suma [1, [2, 3], "]"] y [4]`, `[[x] [y`, `a "[b]" [c] «d» “e”`, `f(2, (3, 4)) y g(1`} {
		c := nuevosCorchetes(s)
		par := parentesisPorLinea(s)
		for i := 0; i < len(s); i++ {
			if s[i] == '[' {
				if got, want := c.cierre(i), cierreCorchete(s, i); got != want {
					t.Errorf("%q: corchete en %d → %d, esperaba %d", s, i, got, want)
				}
			}
			if s[i] == '(' {
				if got, want := par[i], cierreParentesis(s, i); got != want {
					t.Errorf("%q: paréntesis en %d → %d, esperaba %d", s, i, got, want)
				}
			}
		}
	}
	toks := Tokenizar(`suma [1,2] y "hola" con «x» y “y”`)
	var clases []string
	for _, tk := range toks {
		clases = append(clases, tk.Clase)
	}
	if got := strings.Join(clases, " "); got != "palabra lista palabra cita palabra cita palabra cita" {
		t.Errorf("clases: %s", got)
	}
}

func TestRevisionNumerosSinDesbordar(t *testing.T) {
	casos := map[string]string{
		"dos millones millones millones millones": "2000000",
		"dos mil millones":                        "2000000000",
		"trescientos mil millones":                "300000000000",
		"un millón trescientos mil":               "1300000",
		"mil mil":                                 "1000",
		"novecientos noventa y nueve mil novecientos noventa y nueve millones": "999999000000",
	}
	for texto, want := range casos {
		ns := Numeros(texto)
		if len(ns) == 0 || ns[0].Valor.RatString() != want {
			t.Errorf("Numeros(%q) = %+v, esperaba %s primero", texto, ns, want)
		}
		for _, n := range ns {
			if n.Valor.Sign() < 0 || n.Valor.Cmp(big.NewRat(1_000_000_000_000, 1)) >= 0 {
				t.Errorf("Numeros(%q): valor fuera de rango %s", texto, n.Valor.RatString())
			}
		}
	}
	for _, tk := range Lematizar("dos millones millones millones millones") {
		if tk.Num != nil && tk.Num.Sign() < 0 {
			t.Errorf("Lematizar: %s", tk.Num.RatString())
		}
	}
}
