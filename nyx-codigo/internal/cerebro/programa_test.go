package cerebro

import (
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// TestProgramaFrasesSinEntrada: one run with no input reads «Lo ejecuté una vez sin errores» and
// «Lo ejecuté sin entrada y escribió …», never «1 veces» or «con «(nada)»».
func TestProgramaFrasesSinEntrada(t *testing.T) {
	run := programaRun{caso: nucleo.CasoPrograma{Expectativa: nucleo.EspNinguna}, ej: nucleo.Ejecucion{Salida: "Tabla del 13:\n13 x 1 = 13\n"}, ok: true}
	cs := programaComprobaciones([]programaRun{run}, 1)
	if len(cs) != 1 || cs[0].Texto != "Lo ejecuté una vez sin errores" || cs[0].Nivel != nucleo.NivelSinFallos {
		t.Errorf("insignia: %+v", cs)
	}
	malo := run
	malo.ok = false
	if cs := programaComprobaciones([]programaRun{malo}, 1); len(cs) != 1 || cs[0].Texto != "Se cae al ejecutarlo" {
		t.Errorf("insignia de fallo: %+v", cs)
	}
	dos := programaComprobaciones([]programaRun{run, run}, 1)
	if len(dos) != 1 || dos[0].Texto != "Lo ejecuté 2 veces sin errores" {
		t.Errorf("dos ejecuciones: %+v", dos)
	}
	pp := programaPrueba{runs: []programaRun{run}}
	frase := programaFraseRun(pp)
	if !strings.HasPrefix(frase, "Lo ejecuté sin entrada y escribió «Tabla del 13:") {
		t.Errorf("frase: %q", frase)
	}
	if e := programaEntendi(pp); !strings.HasPrefix(e, "sin entrada escribe «Tabla del 13:") {
		t.Errorf("entendí: %q", e)
	}
	for _, s := range []string{frase, programaEntendi(pp)} {
		if strings.Contains(s, "(nada)") || strings.Contains(s, "(sin entrada)") {
			t.Errorf("frase con marcador de tabla: %q", s)
		}
	}
	conEntrada := programaRun{caso: nucleo.CasoPrograma{Entrada: "2\n3\n"}, ej: nucleo.Ejecucion{Salida: "La suma es 5\n"}, ok: true}
	if f := programaFraseRun(programaPrueba{runs: []programaRun{conEntrada}}); f != "Lo ejecuté con «2⏎3⏎» y escribió «La suma es 5»." {
		t.Errorf("con entrada: %q", f)
	}
	if tb := programaTabla([]programaRun{run}); tb.Filas[0][0] != "(sin entrada)" {
		t.Errorf("tabla: %q", tb.Filas[0][0])
	}
}
