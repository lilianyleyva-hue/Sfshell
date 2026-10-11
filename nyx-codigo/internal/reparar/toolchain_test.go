package reparar_test

// Real-toolchain tests (§4.4 Tests): the logic fixtures of testdata/rotos are repaired with the real
// sandbox (arenero), each within 20 s. They run with a plain "go test ./internal/reparar/" and skip
// only when Go is not installed (or with -short).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/reparar"
)

var ar *arenero.Arenero

func cacheCompartida() string {
	if d, err := os.UserCacheDir(); err == nil {
		c := filepath.Join(d, "nyx-codigo-pruebas", "go-build")
		if os.MkdirAll(c, 0o700) == nil {
			return c
		}
	}
	return ""
}

func TestMain(m *testing.M) {
	if arenero.EsTrampolin() {
		arenero.Trampolin()
	}
	dir, err := os.MkdirTemp("", "nyx-reparar-prueba-")
	if err != nil {
		panic(err)
	}
	if _, err := exec.LookPath("go"); err == nil {
		ar, err = arenero.Nuevo(arenero.Config{DirDatos: dir, DirCache: cacheCompartida()})
		if err != nil {
			panic(err)
		}
	}
	codigo := m.Run()
	if ar != nil {
		ar.Cerrar()
	}
	os.RemoveAll(dir)
	os.Exit(codigo)
}

func conGo(t *testing.T) *arenero.Arenero {
	t.Helper()
	if testing.Short() {
		t.Skip("prueba con el compilador de verdad (sin -short)")
	}
	if ar == nil || !ar.Estado().GoOK {
		t.Skip("no hay Go instalado")
	}
	return ar
}

// reparar does what cerebro does: panic repair first, then logic repair on the best result.
func repararLogica(t *testing.T, r *reparar.Reparador, fx reparar.Fixture) (string, []reparar.Cambio, bool) {
	t.Helper()
	ctx := context.Background()
	fuente := fx.Fuente
	var cambios []reparar.Cambio
	ps, err := r.Panicos(ctx, fuente, fx.Firma, fx.Casos, nil)
	if err != nil {
		t.Fatalf("Panicos: %v", err)
	}
	if len(ps) > 0 {
		fuente = ps[0].Fuente
		cambios = append(cambios, ps[0].Cambios...)
		if ps[0].Pasan == ps[0].Total {
			return fuente, cambios, true
		}
	}
	res, err := r.Logica(ctx, fuente, fx.Firma, fx.Casos, nil, nil, nil)
	if err != nil {
		t.Fatalf("Logica: %v", err)
	}
	if len(res.Parches) == 0 {
		t.Logf("motivo: %s", res.Motivo)
		if res.Contraejemplo != nil {
			t.Logf("contraejemplo: %v", res.Contraejemplo.Entradas)
		}
		return fuente, cambios, false
	}
	if res.Contraejemplo == nil {
		t.Errorf("falta el contraejemplo del código original")
	}
	return res.Parches[0].Fuente, append(cambios, res.Parches[0].Cambios...), true
}

func TestFixturesLogicos(t *testing.T) {
	a := conGo(t)
	fxs, err := reparar.LeerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	a.Compilar(context.Background(), "package solucion\n\nfunc F() int { return 1 }\n") // warm the cache
	n := 0
	for _, fx := range fxs {
		if fx.Funcion == "" {
			continue
		}
		n++
		fx := fx
		t.Run(strings.TrimSuffix(fx.Nombre, ".go"), func(t *testing.T) {
			r := &reparar.Reparador{Ej: a, Stats: &nucleotest.ContadorMemoria{}}
			inicio := time.Now()
			fuente, cambios, ok := repararLogica(t, r, fx)
			d := time.Since(inicio)
			if !ok {
				t.Fatalf("no lo arregló en %v", d)
			}
			if d > 20*time.Second {
				t.Errorf("tardó %v (más de 20 s)", d)
			}
			// check the repaired code once more, on its own
			b, _, err := a.Preparar(context.Background(), nucleo.Preparacion{Variantes: []string{fuente}, Firma: fx.Firma, Instr: nucleo.InstrNormal})
			if err != nil {
				t.Fatalf("el arreglo no se prepara: %v\n%s", err, fuente)
			}
			defer b.Cerrar()
			rs, err := b.Probar(context.Background(), fx.Casos, nucleo.OpcionesProbar{})
			if err != nil {
				t.Fatal(err)
			}
			for i, x := range rs[0] {
				if !x.OK {
					t.Errorf("caso %d falla tras el arreglo: %+v\n%s", i, x, fuente)
				}
			}
			for _, c := range cambios {
				t.Logf("%s L%d %q → %q: %s", c.Regla, c.Linea, c.Antes, c.Despues, c.Porque)
			}
			t.Logf("%v", d)
		})
	}
	if n < 7 {
		t.Errorf("solo hay %d fixtures lógicos", n)
	}
}

func TestLogicaAmbigua(t *testing.T) {
	a := conGo(t)
	// with only "(2, 2) -> 4", both "a + b" and "a * b" fix "a - b"; they disagree on (3, 3)
	src := "package solucion\n\nfunc Junta(a, b int) int {\n\treturn a - b\n}\n"
	f := nucleo.Firma{Nombre: "Junta", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	casos := []nucleo.Caso{{Entradas: []nucleo.Valor{2, 2}, Esperado: []nucleo.Valor{4}, Expectativa: nucleo.EspUsuario}}
	r := &reparar.Reparador{Ej: a}
	res, err := r.Logica(context.Background(), src, f, casos, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) < 2 || res.Ambiguo == nil {
		t.Fatalf("esperaba varios arreglos y una ambigüedad: %+v", res)
	}
	if len(res.Ambiguo.Salidas) != len(res.Ambiguo.Parches) || len(res.Ambiguo.Parches) < 2 {
		t.Errorf("ambigüedad mal formada: %+v", res.Ambiguo)
	}
	t.Logf("con %v: %v (parches %v)", res.Ambiguo.Entrada, res.Ambiguo.Salidas, res.Ambiguo.Parches)
	// with an oracle the ambiguity disappears and only the sum survives
	oraculo := func(in []nucleo.Valor) ([]nucleo.Valor, error) { return []nucleo.Valor{in[0].(int) + in[1].(int)}, nil }
	res, err = r.Logica(context.Background(), src, f, casos, nil, oraculo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) == 0 || res.Ambiguo != nil {
		t.Fatalf("con referencia no debería haber ambigüedad: %+v", res)
	}
	for _, p := range res.Parches {
		if !strings.Contains(p.Fuente, "a + b") {
			t.Errorf("solo «a + b» coincide con la referencia:\n%s", p.Fuente)
		}
	}
}

// TestSoloLogica: Logica alone also fixes the fixtures that panic (it adds the panic fixes to its
// mutants), and it reports a minimized counterexample of the original when it has an oracle.
func TestSoloLogica(t *testing.T) {
	a := conGo(t)
	fxs, err := reparar.LeerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for _, fx := range fxs {
		if fx.Funcion == "" {
			continue
		}
		fx := fx
		t.Run(strings.TrimSuffix(fx.Nombre, ".go"), func(t *testing.T) {
			r := &reparar.Reparador{Ej: a}
			inicio := time.Now()
			res, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Parches) == 0 {
				t.Fatalf("no lo arregló: %s", res.Motivo)
			}
			if d := time.Since(inicio); d > 20*time.Second {
				t.Errorf("tardó %v", d)
			}
			if res.Contraejemplo == nil || len(res.Contraejemplo.Entradas) == 0 {
				t.Errorf("falta el contraejemplo")
			}
			if p := res.Parches[0]; p.Pasan != p.Total || p.Total != len(fx.Casos) {
				t.Errorf("parche %d/%d", p.Pasan, p.Total)
			}
		})
	}
}

func TestContraejemploConReferencia(t *testing.T) {
	a := conGo(t)
	fxs, err := reparar.LeerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	var fx reparar.Fixture
	for _, x := range fxs {
		if x.Funcion == "Maximo" {
			fx = x
		}
	}
	maximo := func(in []nucleo.Valor) ([]nucleo.Valor, error) {
		xs := in[0].([]nucleo.Valor)
		if len(xs) == 0 {
			return nil, context.Canceled // no answer for the empty list
		}
		m := xs[0].(int)
		for _, x := range xs {
			if x.(int) > m {
				m = x.(int)
			}
		}
		return []nucleo.Valor{m}, nil
	}
	r := &reparar.Reparador{Ej: a}
	res, err := r.Logica(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil, maximo, nil)
	if err != nil {
		t.Fatal(err)
	}
	ce := res.Contraejemplo
	if ce == nil {
		t.Fatal("falta el contraejemplo")
	}
	xs := ce.Entradas[0].([]nucleo.Valor)
	if len(xs) != 2 || !ce.ConEsperado() {
		t.Errorf("contraejemplo no minimizado: %v → %v", ce.Entradas, ce.Esperado)
	}
	if len(res.Parches) == 0 || res.Ambiguo != nil {
		t.Errorf("%+v", res)
	}
	t.Logf("falla con %v (esperado %v)", ce.Entradas, ce.Esperado)
}

func TestSegundoOrden(t *testing.T) {
	a := conGo(t)
	src := "package solucion\n\nfunc SumaPares(nums []int) int {\n\ttotal := 1\n\tfor _, n := range nums {\n\t\tif n%2 == 1 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total\n}\n"
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	var casos []nucleo.Caso
	for _, c := range []struct {
		in  []nucleo.Valor
		out int
	}{{[]nucleo.Valor{1, 2, 3, 4}, 6}, {[]nucleo.Valor{}, 0}, {[]nucleo.Valor{5, 7}, 0}, {[]nucleo.Valor{2}, 2}, {[]nucleo.Valor{4, 6, 1}, 10}} {
		casos = append(casos, nucleo.Caso{Entradas: []nucleo.Valor{c.in}, Esperado: []nucleo.Valor{c.out}, Expectativa: nucleo.EspUsuario})
	}
	r := &reparar.Reparador{Ej: a}
	inicio := time.Now()
	res, err := r.Logica(context.Background(), src, f, casos, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parches) == 0 {
		t.Fatalf("no encontró la pareja de cambios: %s (mejor %+v)", res.Motivo, res.Mejor)
	}
	p := res.Parches[0]
	if len(p.Cambios) < 2 || !strings.Contains(p.Fuente, "total := 0") {
		t.Errorf("arreglo inesperado: %+v\n%s", p.Cambios, p.Fuente)
	}
	t.Logf("%v: %+v", time.Since(inicio), p.Cambios)
}

func TestCompilacionReal(t *testing.T) {
	a := conGo(t)
	fxs, err := reparar.LeerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	r := &reparar.Reparador{Ej: a}
	for _, fx := range fxs {
		if !fx.Compila {
			continue
		}
		ar := r.Compilacion(context.Background(), fx.Fuente, nil)
		if !ar.OK {
			t.Errorf("%s: no compila con go build: %+v\n%s", fx.Nombre, ar.Quedan, ar.Fuente)
		}
	}
}

// TestCompilacionSinDeclararReal (regression): an undeclared variable is declared, not renamed to
// another variable in scope; the repaired code compiles, vets clean and computes what was written.
func TestCompilacionSinDeclararReal(t *testing.T) {
	a := conGo(t)
	casos := []struct {
		src   string
		firma nucleo.Firma
		caso  nucleo.Caso
	}{
		{"package solucion\n\nfunc F(a, b int) int {\n\tc = a + b\n\treturn c * a\n}\n",
			nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}},
			nucleo.Caso{Entradas: []nucleo.Valor{2, 3}, Esperado: []nucleo.Valor{10}, Expectativa: nucleo.EspUsuario}},
		{"package solucion\n\nfunc Max(xs []int) int {\n\tm = xs[0]\n\tfor _, x := range xs {\n\t\tif x > m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n",
			nucleo.Firma{Nombre: "Max", Params: []nucleo.Param{{Nombre: "xs", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}},
			nucleo.Caso{Entradas: []nucleo.Valor{[]nucleo.Valor{3, 1, 7, 2}}, Esperado: []nucleo.Valor{7}, Expectativa: nucleo.EspUsuario}},
	}
	r := &reparar.Reparador{Ej: a}
	for _, c := range casos {
		res := r.Compilacion(context.Background(), c.src, nil)
		if !res.OK || len(res.Vet) > 0 {
			t.Errorf("no quedó bien: ok=%v vet=%v quedan=%v\n%s", res.OK, res.Vet, res.Quedan, res.Fuente)
			continue
		}
		b, _, err := a.Preparar(context.Background(), nucleo.Preparacion{Variantes: []string{res.Fuente}, Firma: c.firma, Instr: nucleo.InstrNormal})
		if err != nil {
			t.Fatalf("%v\n%s", err, res.Fuente)
		}
		rs, err := b.Probar(context.Background(), []nucleo.Caso{c.caso}, nucleo.OpcionesProbar{})
		b.Cerrar()
		if err != nil {
			t.Fatal(err)
		}
		if !rs[0][0].OK {
			t.Errorf("el arreglo calcula otra cosa: %v → %v (quiero %v)\n%s", c.caso.Entradas, rs[0][0].Obtenido, c.caso.Esperado, res.Fuente)
		}
	}
}

// TestPanicosNoRompeLoQuePasabaReal (regression): no panic patch breaks a case the original passed
// ("" → true for EsPalindromo), and none is the bogus guard «if len(s) <= k».
func TestPanicosNoRompeLoQuePasabaReal(t *testing.T) {
	a := conGo(t)
	fxs, err := reparar.LeerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for _, fx := range fxs {
		if fx.Funcion == "" {
			continue
		}
		r := &reparar.Reparador{Ej: a}
		ps, err := r.Panicos(context.Background(), fx.Fuente, fx.Firma, fx.Casos, nil)
		if err != nil {
			t.Fatalf("%s: %v", fx.Nombre, err)
		}
		if len(ps) == 0 {
			continue
		}
		b, _, err := a.Preparar(context.Background(), nucleo.Preparacion{Variantes: append([]string{fx.Fuente}, fuentes(ps)...), Firma: fx.Firma, Instr: nucleo.InstrNormal})
		if err != nil {
			t.Fatalf("%s: %v", fx.Nombre, err)
		}
		rs, err := b.Probar(context.Background(), fx.Casos, nucleo.OpcionesProbar{})
		b.Cerrar()
		if err != nil {
			t.Fatal(err)
		}
		for k, p := range ps {
			if strings.Contains(p.Fuente, "len(s) <=") || strings.Contains(p.Fuente, "len(xs) <=") {
				t.Errorf("%s: guarda falsa:\n%s", fx.Nombre, p.Fuente)
			}
			for i, x := range rs[0] {
				if x.OK && !rs[k+1][i].OK {
					t.Errorf("%s: el parche %d rompe el caso %d, que pasaba:\n%s", fx.Nombre, k, i, p.Fuente)
				}
			}
		}
	}
}

func fuentes(ps []reparar.Parche) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Fuente)
	}
	return out
}
