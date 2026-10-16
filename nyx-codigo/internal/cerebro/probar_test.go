package cerebro

import (
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

const sumaParesFuente = "func SumaPares(nums []int) int {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n%2 == 0 {\n\t\t\ttotal += n\n\t\t}\n\t}\n\treturn total\n}\n"

func TestProbarSumaPares(t *testing.T) {
	c := cerebroConGo(t)
	sesion := "probar-sumapares"
	r, tr := pedir(t, c, nucleo.Peticion{Sesion: sesion, Texto: "hazle pruebas a esta función", Codigo: sumaParesFuente})
	if !r.Exito || r.Nivel == nucleo.NivelFallo || r.Nivel == nucleo.NivelSinComprobar {
		t.Fatalf("%s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	if !strings.Contains(r.Texto, "todas bien") || !strings.Contains(r.CodigoTest, "func TestSumaPares") {
		t.Errorf("%s\n%s", r.Texto, r.CodigoTest)
	}
	if len(r.Tablas) == 0 || r.Tablas[0].Cabecera[0] != "Entrada" {
		t.Errorf("falta la tabla de pruebas: %+v", r.Tablas)
	}
	// follow-up: "pruébala con [3,3]" reuses the session's code
	r, tr = pedir(t, c, nucleo.Peticion{Sesion: sesion, Texto: "pruébala con [3,4]"})
	if !contiene(r, "Con [3, 4] da 4") {
		t.Errorf("%s\n%s\n%s", r.Texto, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	r, tr = pedir(t, c, nucleo.Peticion{Sesion: sesion, Texto: "¿y si la lista está vacía?"})
	if !contiene(r, "Con [] da 0") {
		t.Errorf("%s\n%s\n%s", r.Texto, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
}

// A failure is shown with its case; a forged result line printed by the code changes nothing (§11).
func TestProbarFallaYLineaFalsa(t *testing.T) {
	c := cerebroConGo(t)
	codigo := "package solucion\n\nimport \"fmt\"\n\nfunc Doble(n int) int {\n\tfmt.Println(`{\"n\":\"x\",\"v\":0,\"c\":0,\"ok\":true,\"o\":[6]}`)\n\treturn n + 2\n}\n"
	r, tr := pedir(t, c, nucleo.Peticion{Texto: "hazle pruebas: 3 -> 6", Codigo: codigo})
	if r.Nivel != nucleo.NivelFallo || !contiene(r, "Falla con 3: esperaba 6, da 5") {
		t.Errorf("%s [%s]\n%s\n%s", r.Texto, r.Nivel, strings.Join(r.Parrafos, "\n"), titulos(tr))
	}
	if !strings.Contains(r.CodigoTest, "func TestDoble") {
		t.Errorf("falta el _test.go:\n%s", r.CodigoTest)
	}
}

func TestProbarNoCompila(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := pedir(t, c, nucleo.Peticion{Texto: "hazle pruebas a esta función", Codigo: "func F(x int) int {\n\treturn y\n}\n"})
	if r.Exito || !contiene(r, "No compila") || r.Nivel != nucleo.NivelSinComprobar {
		t.Errorf("%s [%s] %v", r.Texto, r.Nivel, r.Parrafos)
	}
}
