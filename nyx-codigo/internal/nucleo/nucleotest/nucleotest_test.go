package nucleotest

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

var firmaSuma = nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}

func sumaPares(in []nucleo.Valor) ([]nucleo.Valor, string) {
	t := 0
	for _, x := range in[0].([]nucleo.Valor) {
		if x.(int)%2 == 0 {
			t += x.(int)
		}
	}
	return []nucleo.Valor{t}, ""
}

func TestEjecutorFalsoProbar(t *testing.T) {
	e := &EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func(in []nucleo.Valor) ([]nucleo.Valor, string) { return []nucleo.Valor{0}, "" }, // wrong
		1: func(in []nucleo.Valor) ([]nucleo.Valor, string) { return nil, "index out of range [0] with length 0" },
		3: sumaPares, // "variant 3 is the correct mutant"
		4: func(in []nucleo.Valor) ([]nucleo.Valor, string) { panic("boom") },
	}}
	src := "package solucion\n\nfunc SumaPares(nums []int) int { return 0 }\n"
	b, comp, err := e.Preparar(context.Background(), nucleo.Preparacion{Variantes: []string{src, src, src, src, src}, Firma: firmaSuma})
	if err != nil || !comp.OK {
		t.Fatalf("Preparar: %v %+v", err, comp)
	}
	defer b.Cerrar()
	if b.NumVariantes() != 5 {
		t.Fatalf("NumVariantes = %d", b.NumVariantes())
	}
	casos := []nucleo.Caso{
		{Entradas: []nucleo.Valor{[]nucleo.Valor{1, 2, 3, 4}}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario},
		{Entradas: []nucleo.Valor{[]nucleo.Valor{}}, Esperado: []nucleo.Valor{0}, Expectativa: nucleo.EspUsuario},
		{Entradas: []nucleo.Valor{[]nucleo.Valor{2}}, Origen: nucleo.OrigenAzar},
	}
	res, err := b.Probar(context.Background(), casos, nucleo.OpcionesProbar{Repetir: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 5 || len(res[0]) != 3 {
		t.Fatalf("forma de resultados: %d×%d", len(res), len(res[0]))
	}
	if res[0][0].OK || !res[0][1].OK || !res[0][2].OK {
		t.Errorf("variante 0: %+v", res[0])
	}
	if res[1][0].OK || res[1][0].Panico == "" {
		t.Errorf("variante 1 debería entrar en pánico: %+v", res[1][0])
	}
	if res[2][0].Ejecutado {
		t.Error("la variante 2 no tiene función: no se ejecuta")
	}
	for c := range casos {
		if !res[3][c].OK || !res[3][c].Ejecutado {
			t.Errorf("la variante 3 es la correcta: caso %d %+v", c, res[3][c])
		}
	}
	if res[3][0].Obtenido[0] != 6 {
		t.Errorf("Obtenido = %v", res[3][0].Obtenido)
	}
	if res[4][0].Panico != "boom" {
		t.Errorf("un pánico real se convierte en Panico: %+v", res[4][0])
	}
	// PararAlFallar and a subset of variants
	res, err = b.Probar(context.Background(), casos, nucleo.OpcionesProbar{PararAlFallar: true, Variantes: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0][0].Ejecutado || res[0][1].Ejecutado || res[3][0].Ejecutado {
		t.Errorf("PararAlFallar / Variantes: %+v %+v", res[0], res[3][0])
	}
	if e.NumLlamadas("Preparar") != 1 || e.NumLlamadas("Probar") != 2 {
		t.Errorf("Llamadas = %v", e.Llamadas)
	}
	if _, err := b.Probar(context.Background(), casos, nucleo.OpcionesProbar{Variantes: []int{9}}); err == nil {
		t.Error("una variante inexistente debe dar error")
	}
}

func TestEjecutorFalsoNoDeterminismoYPropiedades(t *testing.T) {
	var mu sync.Mutex
	n := 0
	e := &EjecutorFalso{
		Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
			0: func(in []nucleo.Valor) ([]nucleo.Valor, string) {
				mu.Lock()
				defer mu.Unlock()
				n++
				return []nucleo.Valor{n}, ""
			},
			1: sumaPares,
		},
		EvalProp: func(p nucleo.Propiedad, in, out []nucleo.Valor) bool { return out[0].(int) >= 0 },
	}
	b, _, err := e.Preparar(context.Background(), nucleo.Preparacion{Firma: firmaSuma, Props: []nucleo.Propiedad{{Nombre: "no_negativa", Expr: "r0 >= 0"}}})
	if err != nil {
		t.Fatal(err)
	}
	casos := []nucleo.Caso{{Entradas: []nucleo.Valor{[]nucleo.Valor{-2}}}}
	res, err := b.Probar(context.Background(), casos, nucleo.OpcionesProbar{Repetir: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !res[0][0].NoDeterminista || res[0][0].OK {
		t.Errorf("la variante 0 no es determinista: %+v", res[0][0])
	}
	if res[1][0].OK || res[1][0].PropFallida != "no_negativa" {
		t.Errorf("la suma de [-2] es -2: falla la propiedad: %+v", res[1][0])
	}
	// inputs are copied: a mutating function does not change the case
	e.Funcs[2] = func(in []nucleo.Valor) ([]nucleo.Valor, string) {
		in[0].([]nucleo.Valor)[0] = 99
		return []nucleo.Valor{0}, ""
	}
	b2, _, _ := e.Preparar(context.Background(), nucleo.Preparacion{Firma: firmaSuma})
	if _, err := b2.Probar(context.Background(), casos, nucleo.OpcionesProbar{Variantes: []int{2}}); err != nil {
		t.Fatal(err)
	}
	if casos[0].Entradas[0].([]nucleo.Valor)[0] != -2 {
		t.Error("Probar no debe modificar los casos")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b2.Probar(ctx, casos, nucleo.OpcionesProbar{}); !errors.Is(err, context.Canceled) {
		t.Errorf("con ctx cancelado: %v", err)
	}
}

func TestEjecutorFalsoErroresDePreparar(t *testing.T) {
	e := &EjecutorFalso{}
	malaFirma := nucleo.Firma{Nombre: "F", Params: []nucleo.Param{{Nombre: "c", Tipo: nucleo.TError}}, Res: []nucleo.Tipo{nucleo.TInt}}
	if _, _, err := e.Preparar(context.Background(), nucleo.Preparacion{Firma: malaFirma}); !errors.Is(err, nucleo.ErrNoProbable) {
		t.Errorf("firma no probable: %v", err)
	}
	inseguro := "package solucion\n\nimport \"os/exec\"\n\nfunc SumaPares(nums []int) int { exec.Command(\"ls\"); return 0 }\n"
	if _, _, err := e.Preparar(context.Background(), nucleo.Preparacion{Variantes: []string{inseguro}, Firma: firmaSuma}); !errors.Is(err, nucleo.ErrInseguro) {
		t.Errorf("os/exec en modo automático: %v", err)
	}
	if _, _, err := e.Preparar(context.Background(), nucleo.Preparacion{Variantes: []string{inseguro}, Firma: firmaSuma, Permiso: nucleo.PermisoUsuario}); err != nil {
		t.Errorf("os/exec con permiso del usuario: %v", err)
	}
}

func TestEjecutorFalsoRevisar(t *testing.T) {
	e := &EjecutorFalso{}
	casos := map[string]bool{ // source → has violations in auto mode
		"package x\nimport \"os/exec\"\n":                                             true,
		"package x\nimport \"net/http\"\n":                                            true,
		"package x\nimport \"unsafe\"\n":                                              true,
		"package x\nimport \"C\"\n":                                                   true,
		"package x\nimport \"github.com/x/y\"\n":                                      true,
		"package x\n//go:linkname f runtime.f\nfunc f()\n":                            true,
		"package x\nfunc f() { go f() }\n":                                            true,
		"package x\nimport . \"os\"\n":                                                true,
		"package x\nvar nyx__x int\n":                                                 true,
		"package x\nimport (\"bufio\"; \"os\")\nvar s = bufio.NewScanner(os.Stdin)\n": false,
		"package x\nimport \"strings\"\nvar _ = strings.ToUpper\n":                    false,
	}
	for src, malo := range casos {
		vs := e.Revisar(src, nucleo.PermisoAuto)
		if (len(vs) > 0) != malo {
			t.Errorf("Revisar(%q) = %+v", src, vs)
		}
		for _, v := range vs {
			if v.Que == "" {
				t.Errorf("violación sin explicación: %+v", v)
			}
		}
	}
	if vs := e.Revisar("package x\nimport \"os/exec\"\n", nucleo.PermisoUsuario); len(vs) != 0 {
		t.Errorf("os/exec con permiso del usuario no es grave: %+v", vs)
	}
	if vs := e.Revisar("package x\nimport \"C\"\n", nucleo.PermisoUsuario); len(vs) == 0 || !vs[0].Grave {
		t.Errorf("cgo siempre es grave: %+v", vs)
	}
	e.Violaciones = []nucleo.Violacion{{Linea: 1, Que: "guionizada"}}
	if vs := e.Revisar("package x", nucleo.PermisoAuto); len(vs) != 1 || vs[0].Que != "guionizada" {
		t.Errorf("Violaciones guionizadas: %+v", vs)
	}
}

func TestEjecutorFalsoCompilar(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("sin Go instalado: el falso usa los datos de exportación de la biblioteca estándar")
	}
	e := &EjecutorFalso{}
	ctx := context.Background()
	inicio := time.Now()
	bien := "package solucion\n\nimport \"strings\"\n\nfunc Mayus(s string) string { return strings.ToUpper(s) }\n"
	if c := e.Compilar(ctx, bien); !c.OK {
		t.Fatalf("debería compilar: %+v", c)
	}
	casos := []struct {
		src   string
		linea int
		msg   string
	}{
		{"package solucion\n\nfunc F() int {\n\tx := 1\n\treturn 2\n}\n", 4, "declared and not used: x"},
		{"package solucion\n\nfunc F(s string) string {\n\treturn strings.ToUpper(s)\n}\n", 4, "undefined: strings"},
		{"package solucion\n\nfunc F() int {\n\treturn \"x\"\n}\n", 4, "cannot use"},
		{"package solucion\n\nfunc F() int {\n\treturn 1 +\n}\n", 5, "expected operand"},
		{"package main\n\nfunc f() {}\n", 1, "function main is undeclared"},
	}
	for _, c := range casos {
		r := e.Compilar(ctx, c.src)
		if r.OK || len(r.Errores) == 0 {
			t.Errorf("no debería compilar:\n%s", c.src)
			continue
		}
		if r.Errores[0].Linea != c.linea || !strings.Contains(r.Errores[0].Msg, c.msg) {
			t.Errorf("error %+v, esperaba línea %d con %q", r.Errores[0], c.linea, c.msg)
		}
		if !strings.Contains(r.Texto, "./x.go:") {
			t.Errorf("Texto en estilo del compilador: %q", r.Texto)
		}
	}
	if v := e.Vet(ctx, bien); !v.OK || !strings.Contains(v.Comando, "vet") {
		t.Errorf("Vet: %+v", v)
	}
	if time.Since(inicio) > 20*time.Second {
		t.Errorf("Compilar del falso es demasiado lento: %v", time.Since(inicio))
	}
}

func TestEjecutorFalsoProgramaYEstado(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hola\") }\n"
	e := &EjecutorFalso{}
	if _, err := exec.LookPath("go"); err == nil {
		p, comp, err := e.PrepararPrograma(context.Background(), src, nucleo.OpcionesInstr{}, nucleo.PermisoAuto)
		if err != nil || !comp.OK {
			t.Fatalf("PrepararPrograma: %v %+v", err, comp)
		}
		if _, err := p.Correr(context.Background(), nucleo.CasoPrograma{}); err == nil {
			t.Error("sin Programa definido, Correr debe dar un error honesto")
		}
		e.Programa = func(fuente string, c nucleo.CasoPrograma) (nucleo.Ejecucion, error) {
			return nucleo.Ejecucion{Salida: "La suma es 5\n"}, nil
		}
		if ej, err := p.Correr(context.Background(), nucleo.CasoPrograma{Entrada: "2\n3\n"}); err != nil || ej.Salida != "La suma es 5\n" {
			t.Errorf("Correr: %+v %v", ej, err)
		}
		if _, _, err := e.PrepararPrograma(context.Background(), "package main\nfunc main() { x := 1 }\n", nucleo.OpcionesInstr{}, nucleo.PermisoAuto); !errors.Is(err, nucleo.ErrNoCompila) {
			t.Errorf("un programa que no compila: %v", err)
		}
	}
	if st := e.Estado(); !st.GoOK || st.Nivel == "" {
		t.Errorf("Estado: %+v", st)
	}
	e.EstadoFijo = &nucleo.EstadoArenero{Nivel: "sin Go"}
	if e.Estado().Nivel != "sin Go" {
		t.Error("EstadoFijo")
	}
}

func TestContadorMemoria(t *testing.T) {
	var c nucleo.Contador = NuevoContador()
	if c.Tasa("x") != 0.5 || c.Usos("x") != 0 {
		t.Error("contador vacío: tasa 1/2")
	}
	c.Exito("x", true)
	c.Exito("x", true)
	c.Exito("x", false)
	if c.Usos("x") != 3 || c.Tasa("x") != 3.0/5.0 {
		t.Errorf("tasa %v usos %d", c.Tasa("x"), c.Usos("x"))
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Exito("y", j%2 == 0)
				_ = c.Tasa("y")
			}
		}()
	}
	wg.Wait()
	if c.Usos("y") != 800 {
		t.Errorf("usos concurrentes: %d", c.Usos("y"))
	}
	var cero ContadorMemoria
	cero.Exito("z", true)
	if cero.Usos("z") != 1 {
		t.Error("el valor cero de ContadorMemoria también funciona")
	}
}

// baseHechos and guardar mirror the shapes of logica.BaseHechos and logica.Guardar.
type baseHechos interface {
	Hechos(sujeto string) []nucleo.Hecho
	Todos() []nucleo.Hecho
	Reglas() []nucleo.Regla
}

type guardar interface {
	GuardarHecho(h nucleo.Hecho) (string, error)
	GuardarRegla(r nucleo.Regla) (string, error)
}

func TestBaseHechosMemoria(t *testing.T) {
	b := &BaseHechosMemoria{H: []nucleo.Hecho{{ID: "a", Sujeto: "toby", Relacion: "es_un", Objeto: "perro"}}}
	var _ baseHechos = BaseHechosMemoria{}
	var bh baseHechos = b
	var g guardar = b
	id, _ := g.GuardarHecho(nucleo.Hecho{Sujeto: "Toby", Relacion: "tiene", Objeto: "collar"})
	rid, _ := g.GuardarRegla(nucleo.Regla{Si: []nucleo.Hecho{{Sujeto: "?x", Relacion: "es_un", Objeto: "perro"}},
		Entonces: nucleo.Hecho{Sujeto: "?x", Relacion: "es_un", Objeto: "mamifero"}})
	if id != "h2" || rid != "r1" {
		t.Errorf("ids: %s %s", id, rid)
	}
	if len(bh.Hechos("toby")) != 2 || len(bh.Hechos("misi")) != 0 || len(bh.Todos()) != 2 || len(bh.Reglas()) != 1 {
		t.Errorf("consultas: %+v", b)
	}
}

func TestLematizadorSimple(t *testing.T) {
	texto := "¿Los perros mayores suman 3,5 y «dos» números [1, 2]?"
	toks := LematizadorSimple(texto)
	var lemas, clases []string
	for _, tk := range toks {
		lemas = append(lemas, tk.Lema)
		clases = append(clases, tk.Clase)
		if texto[tk.Desde:tk.Hasta] != tk.Texto {
			t.Errorf("desplazamientos de %q: %q", tk.Texto, texto[tk.Desde:tk.Hasta])
		}
	}
	if got := strings.Join(lemas, " "); got != "¿ los perro mayor suman 3,5 y dos numero [1, 2] ?" {
		t.Errorf("lemas: %q", got)
	}
	if got := strings.Join(clases, " "); got != "simbolo palabra palabra palabra palabra numero palabra cita palabra lista simbolo" {
		t.Errorf("clases: %q", got)
	}
	if toks[5].Num == nil || toks[5].Num.FloatString(1) != "3.5" {
		t.Errorf("número 3,5: %+v", toks[5])
	}
	verbos := LematizadorSimple("suma resta come llueve Toby es un perro siete")
	var vs []string
	for _, tk := range verbos {
		vs = append(vs, tk.Lema)
	}
	if got := strings.Join(vs, " "); got != "sumar restar comer lluever toby es un perro siete" {
		t.Errorf("lemas de verbos: %q", got)
	}
	if verbos[len(verbos)-1].Clase != "numero" || verbos[len(verbos)-1].Num.RatString() != "7" {
		t.Errorf("número en palabras: %+v", verbos[len(verbos)-1])
	}
	var lem nucleo.Lematizador = LematizadorSimple
	if len(lem("")) != 0 {
		t.Error("texto vacío")
	}
}

func TestRespuestasFijas(t *testing.T) {
	r := RespuestasFijas("A", "B")
	ctx := context.Background()
	p := nucleo.PreguntaUsuario{PorDefecto: "Z"}
	if s, err := r(ctx, p); s != "A" || err != nil {
		t.Errorf("1: %q %v", s, err)
	}
	if s, err := r(ctx, p); s != "B" || err != nil {
		t.Errorf("2: %q %v", s, err)
	}
	if s, err := r(ctx, p); s != "Z" || !errors.Is(err, nucleo.ErrSinRespuesta) {
		t.Errorf("3: %q %v", s, err)
	}
	tr := nucleo.NuevaTraza(nil, RespuestasFijas("resta"))
	if s, err := tr.Preguntar(ctx, nucleo.PreguntaUsuario{Texto: "¿«trocar» suma o resta?"}); s != "resta" || err != nil {
		t.Errorf("con Traza: %q %v", s, err)
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := RespuestasFijas("x")(c, p); !errors.Is(err, context.Canceled) {
		t.Errorf("ctx cancelado: %v", err)
	}
}
