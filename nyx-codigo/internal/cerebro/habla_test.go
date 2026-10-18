package cerebro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/habla"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
)

func fijarFecha(c *Cerebro) {
	c.ahora = func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) }
}

func TestHablaEnsenarYPreguntar(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	fijarFecha(c)
	r, tr := preguntar(t, c, "s", "recuerda que capicúa significa palíndromo", nil)
	if !r.Exito || c.Palabras()["capicua"] != "palindromo" {
		t.Fatalf("la lógica y el léxico siguen aprendiendo: %+v\n%s", r, titulos(tr))
	}
	if !strings.Contains(r.Aprendido, "Guardo en mi diccionario que «capicúa» significa «palíndromo»") {
		t.Errorf("también va al diccionario: %q", r.Aprendido)
	}
	s, ok := c.habla.Buscar("capicúa")
	if !ok || s.Definicion != "palíndromo" || s.Origen != "me lo enseñaste tú el 17/10" {
		t.Fatalf("diccionario: %+v", s)
	}
	r, _ = preguntar(t, c, "s2", "¿qué significa capicúa?", nil)
	if !strings.Contains(nucleo.Normalizar(todoElTexto(r)), "palindromo") {
		t.Errorf("¿qué significa capicúa?: %+v", r)
	}
	// a word read in a text the user gave: said with "Creo que" and where it came from
	r, _ = preguntar(t, c, "s3", "lee esto: Un bucle es una forma de repetir código. Las funciones sirven para no repetir código.", nil)
	if !r.Exito || !strings.Contains(r.Texto, "«bucle»") || len(r.Tablas) != 1 {
		t.Fatalf("lee esto: %+v", r)
	}
	r, tr = preguntar(t, c, "s4", "¿qué es un bucle?", nil)
	if r.Metodo != "habla" || r.Texto != "Creo que «bucle» significa: una forma de repetir código." ||
		!strings.Contains(strings.Join(r.Parrafos, " "), "Lo leí en un texto que me diste el 17/10 (no lo he comprobado).") || r.Nivel != nucleo.NivelSinComprobar {
		t.Errorf("¿qué es un bucle?: %+v\n%s", r, titulos(tr))
	}
	// a seed word says it comes from Nyx's dictionary
	r, _ = preguntar(t, c, "s5", "¿Qué significa variable?", nil)
	if r.Texto != "«variable»: una caja con nombre donde guardas un valor." || !contieneParrafo(r.Parrafos, "Viene del diccionario de Nyx.") {
		t.Errorf("variable: %+v", r)
	}
	// plural and accents
	r, _ = preguntar(t, c, "s6", "que significan las funciones", nil)
	if !strings.Contains(r.Texto, "un bloque de código con nombre") {
		t.Errorf("funciones: %+v", r)
	}
}

func TestHablaPalabraDesconocida(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "¿qué significa zanahoria?", nil)
	if r.Texto != "No sé qué significa «zanahoria»." || !strings.Contains(strings.Join(r.Parrafos, " "), "Enséñamelo así: «zanahoria significa …».") ||
		len(r.Sugerencias) == 0 || r.Nivel != nucleo.NivelSinComprobar || r.Exito {
		t.Errorf("%+v", r)
	}
	// an anchored word: it says it has no definition of its own
	r, _ = preguntar(t, c, "s", "¿qué significa memoria?", nil)
	if !strings.HasPrefix(r.Texto, "No tengo una definición de «memoria»") || !strings.Contains(r.Texto, "«aprender»") {
		t.Errorf("memoria: %+v", r)
	}
}

func TestHablaRelaciones(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, tr := preguntar(t, c, "s", "¿qué relación hay entre aprender y olvidar?", nil)
	if !strings.HasPrefix(r.Texto, "Creo que «aprender» y «olvidar» son opuestos") || !strings.Contains(strings.Join(r.Parrafos, " "), "memoria") ||
		r.Nivel != nucleo.NivelSinComprobar || r.Metodo != "habla" {
		t.Fatalf("%+v\n%s", r, titulos(tr))
	}
	r, _ = preguntar(t, c, "s", "¿se parecen aprender y recordar?", nil)
	if !strings.HasPrefix(r.Texto, "Creo que sí se parecen") || !strings.Contains(r.Texto, "afines") {
		t.Errorf("%+v", r)
	}
	r, _ = preguntar(t, c, "s", "¿sumar es lo contrario de restar?", nil)
	if !strings.HasPrefix(r.Texto, "Creo que sí") || !strings.Contains(r.Texto, "opuestos") {
		t.Errorf("%+v", r)
	}
	r, _ = preguntar(t, c, "s", "¿qué relación hay entre aprender y zanahoria?", nil)
	if !strings.Contains(r.Texto, "zanahoria") || r.Exito {
		t.Errorf("no inventa una relación: %+v", r)
	}
}

func TestHablaDecir(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	preguntar(t, c, "s", "lee esto: Una función es un bloque de código con nombre que hace algo. Las funciones sirven para no repetir código. "+
		"Puedes usar una función muchas veces. Una función puede devolver un valor.", nil)
	r, tr := preguntar(t, c, "s", "háblame de las funciones", nil)
	if !strings.HasPrefix(r.Texto, "Esto es lo que sé de «funciones»") || !r.Exito || r.Metodo != "habla" {
		t.Fatalf("%+v\n%s", r, titulos(tr))
	}
	// what it knows comes with its source: the Go note (a source) and the dictionary
	if r.Nivel != nucleo.NivelFuente || len(r.Comprobaciones) != 1 || r.Comprobaciones[0].Texto != "Según mis notas de Go" || len(r.Justificacion) == 0 {
		t.Errorf("sello y justificación: %+v", r)
	}
	if !strings.Contains(strings.Join(r.Parrafos, " "), "«función»: un bloque de código") {
		t.Errorf("debe decir también qué significa el tema: %v", r.Parrafos)
	}
	if strings.Contains(todoElTexto(r), "propias palabras") {
		t.Errorf("no forma frases propias: %v", r.Parrafos)
	}
	r, _ = preguntar(t, c, "s", "¿por qué?", nil)
	if !strings.Contains(todoElTexto(r), "notas de Go") {
		t.Errorf("¿por qué?: %+v", r)
	}
	r, _ = preguntar(t, c, "s2", "háblame de los ornitorrincos", nil)
	if !strings.HasPrefix(r.Texto, "Todavía no sé nada de «ornitorrincos»: léeme algo o enséñame qué significa.") || r.Exito {
		t.Errorf("%+v", r)
	}
}

func TestHablaQuePalabrasSabes(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	preguntar(t, c, "s", "recuerda que capicúa significa palíndromo", nil)
	for _, q := range []string{"¿qué palabras sabes?", "¿cuántas palabras sabes?", "¿cuánto sabes hablar?"} {
		r, _ := preguntar(t, c, "s", q, nil)
		if !strings.HasPrefix(r.Texto, "Sé lo que significan") || !strings.Contains(r.Texto, "66 del diccionario de Nyx") ||
			!strings.Contains(r.Texto, "1 que me enseñaste") || len(r.Tablas) < 2 || r.Tablas[0].Titulo != "Lo que sé de palabras" {
			t.Errorf("%s: %+v", q, r)
		}
		if !strings.Contains(todoElTexto(r), "capicua") {
			t.Errorf("también enseña las palabras del léxico: %s", todoElTexto(r))
		}
	}
	if e := c.Estado(); e.Palabras < 67 {
		t.Errorf("el estado cuenta el diccionario: %d", e.Palabras)
	}
}

func TestHablaNoRobaOtrasPreguntas(t *testing.T) {
	c := nuevoCerebroPrueba(t, nil)
	r, _ := preguntar(t, c, "s", "¿qué es un slice?", nil)
	if r.Metodo == "habla" {
		t.Errorf("los conceptos de Go son de doc_go: %+v", r)
	}
	preguntar(t, c, "s2", "recuerda que Toby es un perro", nil)
	r, _ = preguntar(t, c, "s2", "¿qué es Toby?", nil)
	if r.Metodo == "habla" || !strings.Contains(nucleo.Normalizar(r.Texto), "perro") {
		t.Errorf("la lógica responde con su hecho: %+v", r)
	}
	r, _ = preguntar(t, c, "s3", "resuelve 2x+3=11", nil)
	if !strings.Contains(r.Texto, "x = 4") {
		t.Errorf("%+v", r)
	}
	r, _ = preguntar(t, c, "s4", "¿qué es 2+2?", nil)
	if r.Metodo == "habla" {
		t.Errorf("una cuenta no es una palabra: %+v", r)
	}
}

func TestHablaPersiste(t *testing.T) {
	dir := t.TempDir()
	abrir := func() (*Cerebro, *memoria.Almacen) {
		mem, err := memoria.Abrir(dir)
		if err != nil {
			t.Fatal(err)
		}
		c, err := Nuevo(Config{Semilla: 42, DirDatos: dir}, Dependencias{Mem: mem})
		if err != nil {
			t.Fatal(err)
		}
		return c, mem
	}
	c, mem := abrir()
	preguntar(t, c, "s", "zarzamora significa una fruta negra del campo", nil)
	if c.Palabras()["zarzamora"] != "una fruta negra del campo" {
		t.Errorf("pestaña Memoria: %v", c.Palabras())
	}
	if err := c.Cerrar(); err != nil {
		t.Fatal(err)
	}
	mem.Cerrar()
	if fi, err := os.Stat(filepath.Join(dir, habla.Archivo)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("habla.json: %v %v", fi, err)
	}

	c2, mem2 := abrir()
	defer mem2.Cerrar()
	r, tr := preguntar(t, c2, "s", "¿qué es una zarzamora?", nil)
	if r.Metodo != "habla" || r.Texto != "«zarzamora» significa: una fruta negra del campo." {
		t.Fatalf("el significado sobrevive a un cerebro nuevo: %+v\n%s", r, titulos(tr))
	}
	if err := c2.Olvidar("palabra", "zarzamora"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c2.habla.Buscar("zarzamora"); ok {
		t.Error("olvidar la palabra también olvida su significado")
	}
	if err := c2.Olvidar("palabra", "zarzamora"); err == nil {
		t.Error("olvidar dos veces debe fallar")
	}
}
