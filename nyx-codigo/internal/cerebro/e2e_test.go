package cerebro

import (
	"context"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

// End-to-end checks of §11 and §4.14.5. The other §11 checks live next to their pipelines, with the real
// arenero and the fake web: TestRepararMaxMal (testdata/rotos/max_mal.go with «con [3,1,2] debería dar
// 3»), TestExplicarCuadraticaMedida (static «cuadrática» agrees with the measured class),
// TestEjecutarRiesgosSinEjecutar (os.ReadFile returns Riesgos and runs nothing) and
// TestProbarFallaYLineaFalsa (a forged result line on stdout changes nothing).

const pedidoSumaPares = "haz una función que sume los números pares de una lista"

// pasosDeTipo counts the trace steps of one type.
func pasosDeTipo(tr *nucleo.Traza, tipo nucleo.TipoPaso) []string {
	var out []string
	for _, p := range tr.Pasos() {
		if p.Tipo == tipo {
			out = append(out, p.Titulo)
		}
	}
	return out
}

// The pair-sum request is solved, checked and stored; asked again in another session, the trace says
// «Ya sé hacer esto», nothing is synthesized and it finishes in under 1 s.
func TestE2ESumaParesGuardaYRecuerda(t *testing.T) {
	c := cerebroConGo(t)
	r, tr := pedir(t, c, nucleo.Peticion{Texto: pedidoSumaPares, Sesion: "e2e-1"})
	if !r.Exito || !strings.Contains(r.Codigo, "%2 == 0") {
		t.Fatalf("no lo resolvió: %s [%s]\n%s\n%s", r.Texto, r.Nivel, r.Codigo, titulos(tr))
	}
	guardada := false
	for _, f := range c.mem.Funciones() {
		if f.Nombre == "SumaPares" {
			guardada = true
		}
	}
	if !guardada {
		t.Fatalf("SumaPares no quedó en la memoria: %q\n%s", r.Aprendido, titulos(tr))
	}

	inicio := time.Now()
	r, tr = pedir(t, c, nucleo.Peticion{Texto: pedidoSumaPares, Sesion: "e2e-2"})
	tarda := time.Since(inicio)
	t.Logf("recordar tardó %s", tarda.Round(time.Millisecond))
	if !r.Exito || !strings.Contains(titulos(tr)+"\n"+r.Texto, "Ya sé hacer esto") {
		t.Fatalf("la segunda vez debía recordarlo: %s [%s]\n%s", r.Texto, r.Nivel, titulos(tr))
	}
	if fuerzaNivel[string(r.Nivel)] < fuerzaNivel[string(nucleo.NivelEntendido)] {
		t.Errorf("lo recordado se vuelve a comprobar: nivel %s", r.Nivel)
	}
	if ps := pasosDeTipo(tr, nucleo.PasoIntento); len(ps) > 0 {
		t.Errorf("con la memoria no hace falta construir nada: %q", ps)
	}
	// The 1 s budget is for a warm, idle machine. When go test builds other packages at the same time, a
	// warm build of the same function is slower too, and the budget grows by that factor.
	limite := time.Second
	if ref := tiempoPreparar(t, c, "SumaPares"); ref > 300*time.Millisecond {
		limite = time.Duration(float64(time.Second) * float64(ref) / float64(300*time.Millisecond))
		t.Logf("el arenero va lento (preparar tarda %s): doy %s", ref.Round(time.Millisecond), limite.Round(time.Millisecond))
	}
	if tarda >= limite {
		t.Errorf("recordar debe tardar menos de %s y tardó %s\n%s", limite.Round(time.Millisecond), tarda.Round(time.Millisecond), titulos(tr))
	}
}

// tiempoPreparar measures one warm build of a stored function in the real sandbox.
func tiempoPreparar(t *testing.T, c *Cerebro, nombre string) time.Duration {
	t.Helper()
	for _, f := range c.mem.Funciones() {
		if f.Nombre != nombre {
			continue
		}
		ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancelar()
		inicio := time.Now()
		b, _, err := c.ej.Preparar(ctx, nucleo.Preparacion{Variantes: []string{f.Codigo}, Firma: f.Firma, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
		d := time.Since(inicio)
		if err != nil {
			t.Fatalf("preparar %s: %v", nombre, err)
		}
		b.Cerrar()
		return d
	}
	t.Fatalf("%s no está en la memoria", nombre)
	return 0
}

// A memory hit answers without synthesis: the fake sandbox's call log shows one Preparar (the re-check
// of the stored function) and the trace no construction step (§4.14.5).
func TestMemoriaSinSintesis(t *testing.T) {
	ej := &nucleotest.EjecutorFalso{Funcs: map[int]func([]nucleo.Valor) ([]nucleo.Valor, string){
		0: func(in []nucleo.Valor) ([]nucleo.Valor, string) {
			total := 0
			xs, _ := in[0].([]nucleo.Valor)
			for _, v := range xs {
				if n, _ := v.(int); n%2 == 0 {
					total += n
				}
			}
			return []nucleo.Valor{total}, ""
		},
	}}
	c := nuevoCerebroPrueba(t, ej)
	r, tr := preguntar(t, c, "m-1", pedidoSumaPares, nucleotest.RespuestasFijas())
	if !r.Exito || r.Aprendido == "" {
		t.Fatalf("la primera vez debía resolverlo y guardarlo: %s [%s] %q\n%s", r.Texto, r.Nivel, r.Aprendido, titulos(tr))
	}
	antes := ej.NumLlamadas("Preparar")
	r, tr = preguntar(t, c, "m-2", pedidoSumaPares, nucleotest.RespuestasFijas())
	if !r.Exito || !strings.Contains(titulos(tr)+"\n"+r.Texto, "Ya sé hacer esto") {
		t.Fatalf("la segunda vez debía salir de la memoria: %s [%s]\n%s", r.Texto, r.Nivel, titulos(tr))
	}
	if n := ej.NumLlamadas("Preparar") - antes; n != 1 {
		t.Errorf("un acierto de memoria prepara una vez (para comprobarla) y preparó %d veces\n%s", n, titulos(tr))
	}
	if ps := pasosDeTipo(tr, nucleo.PasoIntento); len(ps) > 0 {
		t.Errorf("no debía sintetizar nada: %q", ps)
	}
}
