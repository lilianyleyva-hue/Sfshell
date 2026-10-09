package nucleo

import (
	"encoding/json"
	"testing"
)

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"¿Cuántas MANERAS?":          "cuantas maneras",
		"año":                        "año",
		"AÑO":                        "año",
		"  ¡Hola,   Mundo!  ":        "hola, mundo",
		"¿y si fuera 13?":            "y si fuera 13",
		"pingüino ÁÉÍÓÚ":             "pinguino aeiou",
		"«cita» y “otra”":            `"cita" y "otra"`,
		"línea\n\tsiguiente palabra": "linea siguiente palabra",
		"":                           "",
	}
	for entrada, want := range casos {
		if got := Normalizar(entrada); got != want {
			t.Errorf("Normalizar(%q) = %q, esperaba %q", entrada, got, want)
		}
	}
}

func TestConceptos(t *testing.T) {
	visto := map[string]bool{}
	for _, c := range Conceptos {
		if visto[c] {
			t.Errorf("concepto repetido: %s", c)
		}
		visto[c] = true
		for _, r := range c {
			if !(r >= 'a' && r <= 'z') && r != '_' {
				t.Errorf("concepto %q: solo minúsculas sin acentos y _", c)
			}
		}
		if !EsConcepto(c) {
			t.Errorf("EsConcepto(%q) = false", c)
		}
	}
	for _, c := range []string{"sumar", "par", "palindromo", "numeros"} {
		if !EsConcepto(c) {
			t.Errorf("falta el concepto %q", c)
		}
	}
	if EsConcepto("volar") {
		t.Error("volar no es un concepto")
	}
}

func TestFamilia(t *testing.T) {
	todas := []TipoIntencion{ICrearFuncion, ICrearPrograma, IReparar, IExplicar, IProbar, IEjecutar, IOptimizar, IDocGo,
		ICalculo, IEcuacion, IProblema, INumeros, ILogica, ISilogismo, IRecordar, IPreguntarHecho, IPuzle, IConteo,
		ISecuencia, IPlan, IBuscar, IMemoria, ICharla, IEstudiar}
	for _, i := range todas {
		if Familia(i) == "" {
			t.Errorf("la intención %s no tiene familia", i)
		}
	}
	if Familia(IReparar) != "programar" || Familia(IEcuacion) != "razonar" || Familia(IBuscar) != "saber" || Familia(ICharla) != "charla" {
		t.Error("familias incorrectas")
	}
	if Familia("otra") != "" {
		t.Error("una intención desconocida no tiene familia")
	}
}

func TestJSONDeContrato(t *testing.T) {
	// the JSON names are part of the contract with the web page (§5)
	r := Respuesta{Texto: "x", Nivel: NivelTusEjemplos, Comprobaciones: []Comprobacion{{Nivel: NivelPropiedades, Pasan: 2, Total: 2, Texto: "ok"}}}
	datos, err := json.Marshal(Evento{Seq: 1, Tipo: "respuesta", Respuesta: &r})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(datos, &m); err != nil {
		t.Fatal(err)
	}
	resp := m["respuesta"].(map[string]any)
	if resp["nivel"] != "tus_ejemplos" || resp["comprobaciones"].([]any)[0].(map[string]any)["pasan"] != 2.0 {
		t.Errorf("JSON de Respuesta: %s", datos)
	}
	f := Firma{Nombre: "F", Params: []Param{{"x", TInt}}, Res: []Tipo{TInt}}
	datos, _ = json.Marshal(f)
	if string(datos) != `{"nombre":"F","params":[{"nombre":"x","tipo":{"clase":1,"nombre":"int"}}],"res":[{"clase":1,"nombre":"int"}]}` {
		t.Errorf("JSON de Firma: %s", datos)
	}
}
