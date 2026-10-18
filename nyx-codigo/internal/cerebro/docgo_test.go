package cerebro

import (
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/saber"
)

func badge(r nucleo.Respuesta, texto string) (nucleo.Comprobacion, bool) {
	for _, cp := range r.Comprobaciones {
		if strings.Contains(cp.Texto, texto) {
			return cp, true
		}
	}
	return nucleo.Comprobacion{}, false
}

// Without Go nor sandbox the Spanish note still answers, labelled as my notes, and nothing claims an
// example was run.
func TestDocGoSoloNotas(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "¿para qué sirve defer?", nil)
	todo := todoElTexto(r)
	if !r.Exito || r.Nivel != nucleo.NivelFuente || !strings.Contains(r.Texto, "defer") || !strings.Contains(todo, "al final") {
		t.Fatalf("defer: %+v", r)
	}
	if _, ok := badge(r, "Según mis notas de Go"); !ok {
		t.Errorf("insignia de fuente: %+v", r.Comprobaciones)
	}
	if _, ok := badge(r, "Ejemplo comprobado"); ok || strings.Contains(todo, "Lo probé") {
		t.Errorf("sin caja no dice que probó el ejemplo: %+v", r)
	}
	if !strings.Contains(todo, "No probé el ejemplo: no tengo caja segura") {
		t.Errorf("dice por qué no lo probó:\n%s", todo)
	}

	r2, _ := preguntar(t, c, "s2", "¿qué hace strings.Zorglub?", nil)
	if r2.Exito || r2.Nivel != nucleo.NivelSinComprobar || !strings.Contains(r2.Texto, "No tengo nada") {
		t.Errorf("símbolo desconocido: %+v", r2)
	}
}

// Exam rows 25 and 26 with the real toolchain: note + go doc (English, labelled, with its pkg.go.dev
// link) + the example run and compared; plus a GOROOT example from the index.
func TestDocGoConGo(t *testing.T) {
	explicarNecesitaGo(t)
	c := cerebroExamen(t)
	c.PonerIndice(&saber.IndiceStd{GoVersion: "prueba", Ejemplos: []saber.EjemploStd{{
		Paquete: "strings", Nombre: "ExampleFields", Simbolo: "Fields", Salida: "Fields are: [\"foo\" \"bar\" \"baz\"]",
		Juego: "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Printf(\"Fields are: %q\\n\", strings.Fields(\"  foo bar  baz   \"))\n}\n",
	}}})
	r, tr := preguntar(t, c, "s", "¿qué hace strings.Fields?", nil)
	todo := todoElTexto(r)
	for _, quiero := range []string{"Separa", "Documentación de Go (en inglés)", "func Fields(s string) []string", "Lo probé y escribe: «[uno dos tres] ⏎ 3»", "ExampleFields"} {
		if !strings.Contains(todo, quiero) {
			t.Errorf("falta «%s»:\n%s\n%s", quiero, todo, titulos(tr))
		}
	}
	if r.Nivel != nucleo.NivelFuente || !r.Exito {
		t.Errorf("nivel %s", r.Nivel)
	}
	if _, ok := badge(r, "Según la documentación de Go"); !ok {
		t.Errorf("insignias: %+v", r.Comprobaciones)
	}
	if cp, ok := badge(r, "Ejemplo comprobado"); !ok || cp.Pasan != cp.Total {
		t.Errorf("insignias: %+v", r.Comprobaciones)
	}
	if len(r.Fuentes) == 0 || r.Fuentes[0].URL != "https://pkg.go.dev/strings#Fields" {
		t.Errorf("fuentes: %+v", r.Fuentes)
	}

	r2, _ := preguntar(t, c, "s2", "¿para qué sirve defer?", nil)
	if !strings.Contains(todoElTexto(r2), "al final") || r2.Nivel != nucleo.NivelFuente {
		t.Errorf("defer: %+v", r2)
	}
	if _, ok := badge(r2, "Ejemplo comprobado"); !ok {
		t.Errorf("el ejemplo de defer se ejecuta: %+v", r2.Comprobaciones)
	}
}

func TestDocGoLeerGoDoc(t *testing.T) {
	salida := "package rand // import \"math/rand\"\n\nfunc Intn(n int) int\n    Intn returns, as an int, a non-negative pseudo-random number in the\n    half-open interval [0,n).\n\n    More text.\n"
	d := docgoLeerGoDoc(salida, "rand.Intn")
	if d.firma != "func Intn(n int) int" || !strings.HasPrefix(d.texto, "Intn returns, as an int") || d.url != "https://pkg.go.dev/math/rand#Intn" {
		t.Errorf("%+v", d)
	}
	d = docgoLeerGoDoc("package strings // import \"strings\"\n\nfunc (b *Builder) WriteString(s string) (int, error)\n    WriteString appends the contents of s to b's buffer.\n", "strings.Builder.WriteString")
	if d.url != "https://pkg.go.dev/strings#Builder.WriteString" {
		t.Errorf("método: %+v", d)
	}
}

// Review quality (§0): when the note's example runs and prints something else, the answer says so and
// does not badge it as checked.
func TestDocGoEjemploQueEscribeOtraCosa(t *testing.T) {
	falso := &nucleotest.EjecutorFalso{Programa: func(fuente string, c nucleo.CasoPrograma) (nucleo.Ejecucion, error) {
		return nucleo.Ejecucion{Salida: "otra cosa\n"}, nil
	}}
	c := nuevoCerebroPrueba(t, falso)
	r, tr := preguntar(t, c, "s", "¿qué hace strings.Fields?", nil)
	todo := todoElTexto(r)
	if falso.NumLlamadas("Correr") == 0 {
		t.Fatalf("no ejecutó el ejemplo:\n%s\n%s", todo, titulos(tr))
	}
	if !strings.Contains(todo, "probé el ejemplo y escribió «otra cosa» en vez de «") {
		t.Errorf("dice lo que escribió de verdad:\n%s", todo)
	}
	if _, ok := badge(r, "Ejemplo comprobado"); ok || strings.Contains(todo, "Lo probé y escribe") {
		t.Errorf("no lo da por comprobado: %+v\n%s", r.Comprobaciones, todo)
	}
	if !r.Exito || r.Nivel != nucleo.NivelFuente {
		t.Errorf("la nota sigue valiendo como fuente: %v %s", r.Exito, r.Nivel)
	}
}
