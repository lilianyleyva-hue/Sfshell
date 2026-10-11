package servidor

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
)

// TestDemoUI serves the real UI with the replaying fake Motor so the page can be developed and checked
// in a browser without cerebro. It only runs when NYX_DEMO_PUERTO is set:
//
//	NYX_DEMO_PUERTO=8799 NYX_DEMO_SEGUNDOS=600 go test ./internal/servidor -run TestDemoUI -v
//
// then open http://127.0.0.1:8799/#t=<tokenPrueba>.
func TestDemoUI(t *testing.T) {
	ps := os.Getenv("NYX_DEMO_PUERTO")
	if ps == "" {
		t.Skip("NYX_DEMO_PUERTO no está definido")
	}
	puerto, err := strconv.Atoi(ps)
	if err != nil {
		t.Fatal(err)
	}
	segundos, _ := strconv.Atoi(os.Getenv("NYX_DEMO_SEGUNDOS"))
	if segundos <= 0 {
		segundos = 300
	}
	g, err := cargarTraza("testdata/traza-sumapares.json")
	if err != nil {
		t.Fatal(err)
	}
	arreglo, err := cargarTraza("testdata/traza-arreglo.json")
	if err != nil {
		t.Fatal(err)
	}
	m := &motorFalso{grabada: g, demora: 120 * time.Millisecond, otras: map[string]trazaGrabada{"arregla": arreglo}}
	f := nucleo.Firma{Nombre: "SumaPares", Params: []nucleo.Param{{Nombre: "nums", Tipo: nucleo.ListaDe(nucleo.TInt)}}, Res: []nucleo.Tipo{nucleo.TInt}}
	casos, _ := nucleo.GuardarCasos(f, []nucleo.Caso{
		{Entradas: []nucleo.Valor{[]nucleo.Valor{1, 2, 3, 4}}, Esperado: []nucleo.Valor{6}, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario},
		{Entradas: []nucleo.Valor{[]nucleo.Valor(nil)}, Esperado: []nucleo.Valor{0}, Expectativa: nucleo.EspInterpretacion, Origen: nucleo.OrigenBorde},
	})
	m.funciones = []nucleo.FuncionAprendida{{Nombre: "SumaPares", Firma: f, Casos: casos, Codigo: g.Respuesta.Codigo,
		Descripcion: "la suma de los números pares de nums", Origen: "sintesis", Nivel: nucleo.NivelEntendido, Usos: 3,
		Creada: time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)}}
	m.pendientes = []pendienteFalso{{ID: "p1", Texto: "haz una función que ordene palabras por longitud", Motivo: "se acabó el tiempo", Intentos: 1, Fecha: time.Now()}}
	h := Nuevo(m, Config{Token: tokenPrueba, Puerto: puerto})
	ln, err := net.Listen("tcp", "127.0.0.1:"+ps)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	fmt.Printf("Nyx Código (demo) en http://127.0.0.1:%d/#t=%s\n", puerto, tokenPrueba)
	time.Sleep(time.Duration(segundos) * time.Second)
	srv.Close()
}
