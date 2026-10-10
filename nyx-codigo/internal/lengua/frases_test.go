package lengua

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

// fila is one line of testdata/frases.tsv:
// texto ⇥ intención ⇥ firma (Forma) ⇥ marco (k=v;…) ⇥ ejemplos (pares | prog:N). "-" skips a check.
type fila struct {
	linea                                    int
	texto, intencion, firma, marco, ejemplos string
}

func leerFrases(t *testing.T) []fila {
	t.Helper()
	f, err := os.Open("testdata/frases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []fila
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		l := sc.Text()
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			continue
		}
		c := strings.Split(l, "\t")
		if len(c) != 5 {
			t.Fatalf("frases.tsv:%d: %d columnas, esperaba 5", n, len(c))
		}
		texto := strings.NewReplacer(`\n`, "\n", `\t`, "\t").Replace(c[0])
		out = append(out, fila{n, texto, c[1], c[2], c[3], c[4]})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// comprobarFila returns "" when the analysis matches the row, else what differs.
func comprobarFila(f fila) string {
	p := Analizar(f.texto, "", "", nil, nil)
	var fallos []string
	if f.intencion != "-" {
		got := "(ninguna)"
		if len(p.Intenciones) > 0 {
			got = string(p.Intenciones[0].Tipo)
		}
		if got != f.intencion {
			fallos = append(fallos, "intención "+got)
		}
	}
	if f.firma != "-" {
		got := "(nil)"
		if p.Firma != nil {
			got = p.Firma.Forma()
		}
		if got != f.firma {
			fallos = append(fallos, "firma "+got)
		}
	}
	if f.marco != "-" {
		if p.Marco == nil {
			fallos = append(fallos, "sin marco")
		} else {
			for _, kv := range strings.Split(f.marco, ";") {
				k, v, _ := strings.Cut(kv, "=")
				if got := campoMarco(p.Marco, k); got != v {
					fallos = append(fallos, k+"="+got)
				}
			}
		}
	}
	if f.ejemplos != "-" {
		if strings.HasPrefix(f.ejemplos, "prog:") {
			n, _ := strconv.Atoi(strings.TrimPrefix(f.ejemplos, "prog:"))
			if len(p.EjProgramas) != n {
				fallos = append(fallos, "casos de programa "+strconv.Itoa(len(p.EjProgramas)))
			}
		} else {
			n, _ := strconv.Atoi(f.ejemplos)
			if len(p.ParesTexto) != n {
				fallos = append(fallos, "ejemplos "+strconv.Itoa(len(p.ParesTexto)))
			} else if n > 0 && p.Firma != nil && len(p.Ejemplos) != n {
				fallos = append(fallos, "ejemplos tipados "+strconv.Itoa(len(p.Ejemplos)))
			}
		}
	}
	return strings.Join(fallos, ", ")
}

func campoMarco(m *nucleo.Marco, k string) string {
	switch k {
	case "accion":
		return m.Accion
	case "objeto":
		return m.Objeto
	case "elemento":
		return m.Elemento
	case "salida":
		return m.Salida
	case "entrada":
		return m.Entrada
	case "plural":
		return strconv.FormatBool(m.Plural)
	case "programa":
		return strconv.FormatBool(m.Programa)
	case "lee":
		return strings.Join(m.Lee, ",")
	case "nombre":
		return NombreFuncion(m)
	case "mods":
		var cs []string
		for _, md := range m.Mods {
			c := md.Concepto
			if md.Negado {
				c = "!" + c
			}
			cs = append(cs, c)
		}
		return strings.Join(cs, ",")
	}
	return "?" + k
}

func TestFrases(t *testing.T) {
	filas := leerFrases(t)
	if len(filas) < 120 {
		t.Fatalf("frases.tsv tiene %d frases, deben ser al menos 120", len(filas))
	}
	pasan := 0
	for _, f := range filas {
		if msg := comprobarFila(f); msg != "" {
			t.Logf("frases.tsv:%d %q: %s", f.linea, f.texto, msg)
		} else {
			pasan++
		}
	}
	pct := 100 * float64(pasan) / float64(len(filas))
	t.Logf("pasan %d de %d (%.1f %%)", pasan, len(filas), pct)
	if pct < 90 {
		t.Errorf("solo pasan %.1f %% de las frases; el mínimo es 90 %%", pct)
	}
}

// The spec's named cases must always pass, whatever the global rate.
func TestFrasesObligatorias(t *testing.T) {
	p := Analizar("resuelve 2x+3=11", "", "", nil, nil)
	if p.Intenciones[0].Tipo != nucleo.IEcuacion {
		t.Errorf("resuelve 2x+3=11 → %v", p.Intenciones[0].Tipo)
	}
	p = Analizar("[1,2,3] -> 6", "", "", nil, nil)
	if len(p.ParesTexto) != 1 || p.Firma == nil || p.Firma.Forma() != "([]int)int" || len(p.Ejemplos) != 1 {
		t.Errorf("[1,2,3] -> 6: pares %v firma %v", p.ParesTexto, p.Firma)
	}
	if p.Ejemplos[0].Expectativa != nucleo.EspUsuario {
		t.Errorf("los ejemplos deben ser EspUsuario")
	}
	p = Analizar("devuelve la palabra más larga de una frase", "", "", nil, nil)
	if p.Firma == nil || p.Firma.Res[0].Go() != "string" {
		t.Errorf("la palabra más larga → %v", p.Firma)
	}
	p = Analizar("devuelve las palabras más largas de una frase", "", "", nil, nil)
	if p.Firma == nil || p.Firma.Res[0].Go() != "[]string" {
		t.Errorf("las palabras más largas → %v", p.Firma)
	}
	p = Analizar("haz un programa que pida dos números y los sume", "", "", nil, nil)
	if p.Intenciones[0].Tipo != nucleo.ICrearPrograma || p.Marco == nil || strings.Join(p.Marco.Lee, ",") != "numero,numero" {
		t.Errorf("programa que pida dos números: %v %+v", p.Intenciones, p.Marco)
	}
}
