package habla

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const parrafoFunciones = `Una función es un bloque de código con nombre que hace algo.
Las funciones sirven para no repetir código. Puedes usar una función muchas veces.
Una función puede devolver un valor. Las funciones reciben parámetros extraños.`

func TestSemilla(t *testing.T) {
	h := Nuevo()
	l := h.Lista()
	if len(l) != 66 {
		t.Fatalf("quiero las 66 palabras núcleo, tengo %d", len(l))
	}
	for _, s := range l {
		if !s.Propio || s.Fuente != FuenteSemilla || s.Origen != OrigenSemilla || s.Definicion == "" || s.Ingles == "" {
			t.Errorf("semilla incompleta: %+v", s)
		}
	}
	s, ok := h.Buscar("variable")
	if !ok || s.Definicion != "una caja con nombre donde guardas un valor" || s.Ingles != "variable" {
		t.Fatalf("variable: %+v %v", s, ok)
	}
	e := h.Estado()
	if e.ConSignificado != 66 || e.DeSemilla != 66 || e.DelUsuario != 0 || e.Anclados == 0 || e.Vocabulario == 0 {
		t.Errorf("estado inicial: %+v", e)
	}
}

func TestBuscarTolerante(t *testing.T) {
	h := Nuevo()
	casos := map[string]string{
		"Función": "función", "FUNCIONES": "función", "funcion": "función", "números": "número",
		"recuerda": "recordar", "guardas": "guardar", "olvidé": "olvidar", "pequeña": "pequeño",
		"listas": "lista", "una variable": "variable", "sumando": "sumar", "aprendió": "aprender",
		"Pantallas": "pantalla", "escribiendo": "escribir",
	}
	for w, quiero := range casos {
		s, ok := h.Buscar(w)
		if !ok || !s.Propio || s.Palabra != quiero {
			t.Errorf("Buscar(%q) = %q (propio %v, ok %v), quiero %q", w, s.Palabra, s.Propio, ok, quiero)
		}
	}
	if _, ok := h.Buscar("zanahoria"); ok {
		t.Error("no debería saber qué es una zanahoria")
	}
	// anchored: "memoria" only appears inside definitions
	s, ok := h.Buscar("memoria")
	if !ok || s.Propio || s.Definicion != "" || !strings.Contains(s.Origen, "aprender") {
		t.Errorf("memoria debe estar anclada, no definida: %+v %v", s, ok)
	}
}

func TestEnsenar(t *testing.T) {
	h := Nuevo()
	s, err := h.Ensenar("Capicúa", "que se lee igual al derecho y al revés", FuenteUsuario, "me lo enseñaste tú el 17/10")
	if err != nil || s.Palabra != "capicúa" || s.Confianza != 1 || !s.Propio {
		t.Fatalf("enseñar: %+v %v", s, err)
	}
	if b, ok := h.Buscar("capicua"); !ok || b.Origen != "me lo enseñaste tú el 17/10" {
		t.Fatalf("buscar: %+v", b)
	}
	s, err = h.Ensenar("capicúa", "palíndromo", FuenteUsuario, "me lo enseñaste tú el 18/10")
	if err != nil || s.Definicion != "palíndromo" || len(s.Anteriores) != 1 || s.Anteriores[0] != "que se lee igual al derecho y al revés" {
		t.Fatalf("la nueva definición debe guardar la anterior: %+v %v", s, err)
	}
	// a user definition replaces a seed one, keeping it
	s, err = h.Ensenar("teclado", "lo que tiene teclas", FuenteUsuario, "x")
	if err != nil || len(s.Anteriores) != 1 || s.Anteriores[0] != "con lo que escribes" {
		t.Fatalf("semilla: %+v %v", s, err)
	}
	// the web never replaces what the user taught
	if _, err := h.Ensenar("capicúa", "otra cosa", FuenteWeb, "web"); err != ErrYaEnsenada {
		t.Fatalf("la web no debe pisar al usuario: %v", err)
	}
	malos := [][2]string{
		{"", "algo"}, {"algo", ""}, {strings.Repeat("a", 41), "algo"}, {"palabra", strings.Repeat("a ", 151)},
		{"x2", "algo"}, {"hola", "hola"}, {"una frase muy larga de verdad", "algo"},
	}
	for _, m := range malos {
		if _, err := h.Ensenar(m[0], m[1], FuenteUsuario, ""); err == nil {
			t.Errorf("Ensenar(%q, %q) debía fallar", m[0], m[1])
		}
	}
	if _, err := h.Ensenar(strings.Repeat("a", 40), "una palabra larga", FuenteUsuario, ""); err != nil {
		t.Errorf("40 letras valen: %v", err)
	}
	if err := h.Olvidar("Capicúa"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Buscar("capicúa"); ok {
		t.Error("debía olvidarla")
	}
	if err := h.Olvidar("capicúa"); err != ErrNoLaSe {
		t.Errorf("olvidar dos veces: %v", err)
	}
}

func TestLeerDefiniciones(t *testing.T) {
	h := Nuevo()
	texto := `Un slice es una lista que puede crecer con append: comparte memoria.
Capicúa significa que se lee igual al revés.
Un bucle quiere decir que repites algo.
Los canales sirven para pasar valores entre goroutines.
Llamamos clausura a una función que recuerda sus variables.
defer: dejar algo para el final
¿Un perro es un animal?
Las funciones anónimas son unas funciones sin nombre.
Una variable es un nombre cualquiera.
Un módulo (archivo go.mod) es un conjunto de paquetes.
factorial: 5 -> 120
Esto es una prueba.`
	l := h.Leer(texto, FuenteLectura, "lo leí en mis notas de Go")
	if l.Frases != 12 {
		t.Errorf("frases: %d", l.Frases)
	}
	quiero := map[string]string{
		"slice":           "una lista que puede crecer con append",
		"capicúa":         "que se lee igual al revés",
		"bucle":           "que repites algo",
		"canal":           "sirve para pasar valores entre goroutines",
		"clausura":        "una función que recuerda sus variables",
		"defer":           "dejar algo para el final",
		"función anónima": "unas funciones sin nombre",
	}
	for w, d := range quiero {
		s, ok := h.Buscar(w)
		if !ok || !s.Propio || s.Definicion != d || s.Fuente != FuenteLectura || s.Confianza >= 1 || s.Origen != "lo leí en mis notas de Go" {
			t.Errorf("%s: %+v", w, s)
		}
	}
	for _, w := range []string{"perro", "factorial", "esto"} {
		if s, ok := h.Buscar(w); ok && s.Propio {
			t.Errorf("no debía aprender %q: %+v", w, s)
		}
	}
	if s, _ := h.Buscar("variable"); s.Fuente != FuenteSemilla {
		t.Errorf("leer no cambia el diccionario de Nyx: %+v", s)
	}
	if s, _ := h.Buscar("módulo"); s.Fuente != FuenteSemilla {
		t.Errorf("módulo: %+v", s)
	}
	if len(l.Nuevos) != len(quiero) {
		t.Errorf("nuevos: %d, quiero %d: %+v", len(l.Nuevos), len(quiero), l.Nuevos)
	}
}

func TestLeerNoPisaAlUsuario(t *testing.T) {
	h := Nuevo()
	if _, err := h.Ensenar("capicúa", "palíndromo", FuenteUsuario, "tú"); err != nil {
		t.Fatal(err)
	}
	l := h.Leer("Capicúa es una palabra rara. Capicúa significa otra cosa.", FuenteUsuario, "lo leí en lo que me escribiste")
	if len(l.Nuevos) != 0 {
		t.Fatalf("leer no debe pisar lo que me enseñaste: %+v", l.Nuevos)
	}
	if s, _ := h.Buscar("capicúa"); s.Definicion != "palíndromo" {
		t.Fatalf("%+v", s)
	}
	// a read definition is replaced by a better reading, never by a weaker one
	h.Leer("Un tubo es una cosa hueca.", FuenteLectura, "a")
	h.Leer("Un tubo es otra cosa distinta.", FuenteWeb, "b")
	if s, _ := h.Buscar("tubo"); s.Definicion != "una cosa hueca" {
		t.Fatalf("la web (menos fiable) no pisa una lectura: %+v", s)
	}
}

func TestRelaciones(t *testing.T) {
	h := Nuevo()
	casos := []struct{ a, b, etiqueta string }{
		{"aprender", "olvidar", "opuestos"},
		{"aprender", "recordar", "afines"},
		{"sumar", "restar", "opuestos"},
		{"recordar", "olvidar", "opuestos"},
		{"teclado", "olvidar", "sin relación"},
		{"variable", "variable", "la misma palabra"},
	}
	for _, c := range casos {
		r, ok := h.Relacion(c.a, c.b)
		if !ok || r.Etiqueta != c.etiqueta {
			t.Errorf("%s / %s: %q (coseno %.2f, ok %v), quiero %q", c.a, c.b, r.Etiqueta, r.Coseno, ok, c.etiqueta)
		}
	}
	r, _ := h.Relacion("aprender", "olvidar")
	if !contiene(r.Compartidas, "memoria") || r.PolaridadA != 1 || r.PolaridadB != -1 {
		t.Errorf("aprender/olvidar deben compartir «memoria» con signos contrarios: %+v", r)
	}
	r, _ = h.Relacion("sumar", "restar")
	if !contiene(r.Compartidas, "número") {
		t.Errorf("sumar/restar comparten «número»: %+v", r.Compartidas)
	}
	r, ok := h.Relacion("aprender", "zanahoria")
	if ok || len(r.Desconocidas) != 1 || r.Desconocidas[0] != "zanahoria" {
		t.Errorf("no debe inventar una relación: %+v %v", r, ok)
	}
	// an anchored word has an inherited vector
	r, ok = h.Relacion("memoria", "recordar")
	if !ok || r.SignificadoA.Propio || r.Etiqueta == "sin relación" {
		t.Errorf("memoria/recordar: %+v %v", r, ok)
	}
}

func contiene(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// conocida: every content word of a generated sentence must have a meaning (own or anchored).
func comprobarConocidas(t *testing.T, h *Habla, f Frase) {
	t.Helper()
	for _, w := range f.Palabras {
		if esFuncion(plegar(w)) {
			continue
		}
		if _, ok := h.Buscar(w); !ok {
			t.Errorf("la frase %q usa «%s», que no sé qué significa", f.Texto, w)
		}
	}
}

func TestDecir(t *testing.T) {
	h := Nuevo()
	h.Leer(parrafoFunciones, FuenteUsuario, "me lo diste a leer")
	for semilla := int64(1); semilla <= 20; semilla++ {
		f, ok := h.Decir(rand.New(rand.NewSource(semilla)), []string{"funciones"}, 25)
		if !ok {
			t.Fatalf("semilla %d: no dijo nada (%s)", semilla, f.Motivo)
		}
		if len(f.Palabras) > 25 || len(f.Palabras) < 2 || !strings.HasSuffix(f.Texto, ".") {
			t.Errorf("frase rara: %+v", f)
		}
		if !strings.Contains(plegar(f.Texto), "funcion") {
			t.Errorf("la frase debe hablar del tema: %q", f.Texto)
		}
		for _, w := range []string{"parametros", "extraños", "reciben"} {
			if strings.Contains(plegar(f.Texto), plegar(w)) {
				t.Errorf("usa una palabra que no conoce (%s): %q", w, f.Texto)
			}
		}
		comprobarConocidas(t, h, f)
	}
	// deterministic with the same seed
	a, _ := h.Decir(rand.New(rand.NewSource(7)), []string{"función"}, 25)
	b, _ := h.Decir(rand.New(rand.NewSource(7)), []string{"función"}, 25)
	if a.Texto != b.Texto {
		t.Errorf("con la misma semilla debe decir lo mismo: %q / %q", a.Texto, b.Texto)
	}
	if f, ok := h.Decir(nil, []string{"zanahoria"}, 25); ok || f.Motivo != "sin_lectura" {
		t.Errorf("zanahoria: %+v", f)
	}
	h.Leer("Los gatos maúllan mucho por la noche.", FuenteUsuario, "x")
	if f, ok := h.Decir(nil, []string{"gatos"}, 25); ok || f.Motivo != "sin_significado" {
		t.Errorf("no sabe qué es un gato: %+v", f)
	}
	if f, ok := h.Decir(nil, []string{"de", "la"}, 25); ok || f.Motivo != "sin_tema" {
		t.Errorf("sin tema: %+v", f)
	}
	// once it knows, it can talk about it
	if _, err := h.Ensenar("gato", "un animal pequeño que maúlla", FuenteUsuario, "x"); err != nil {
		t.Fatal(err)
	}
	h.Leer("Los gatos duermen mucho.", FuenteUsuario, "x")
	f, ok := h.Decir(rand.New(rand.NewSource(3)), []string{"gato"}, 25)
	if !ok {
		t.Fatalf("gato: %+v", f)
	}
	comprobarConocidas(t, h, f)
	// max words
	f, ok = h.Decir(rand.New(rand.NewSource(3)), []string{"función"}, 6)
	if ok && len(f.Palabras) > 6 {
		t.Errorf("más de 6 palabras: %+v", f)
	}
}

func TestDecirNuncaUsaDesconocidas(t *testing.T) {
	h := Nuevo()
	h.Leer(`El programa tiene una variable rara llamada contador.
Una variable guarda un valor y un número.
El programa usa la variable con cuidado extremo.
La variable cambia cuando el programa corre.`, FuenteUsuario, "x")
	for s := int64(0); s < 30; s++ {
		f, ok := h.Decir(rand.New(rand.NewSource(s)), []string{"variable", "programa"}, 25)
		if !ok {
			continue
		}
		comprobarConocidas(t, h, f)
		for _, w := range []string{"rara", "contador", "cuidado", "extremo", "cambia"} {
			for _, p := range f.Palabras {
				if plegar(p) == w {
					t.Errorf("usa «%s»: %q", w, f.Texto)
				}
			}
		}
	}
}

func TestDesconocidas(t *testing.T) {
	h := Nuevo()
	d := h.Desconocidas("Los gatos maúllan y el código compila con SumaPares; la memoria funciona.")
	for _, w := range []string{"gatos", "maúllan", "compila"} {
		if !contiene(d, w) {
			t.Errorf("falta %q en %v", w, d)
		}
	}
	for _, w := range []string{"código", "sumapares", "memoria", "los", "con"} {
		if contiene(d, w) {
			t.Errorf("sobra %q en %v", w, d)
		}
	}
}

func TestEstadoYFluidez(t *testing.T) {
	h := Nuevo()
	if _, err := h.Ensenar("capicúa", "palíndromo", FuenteUsuario, "x"); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "Una función puede devolver un valor número %s.\nEl programa guarda un valor en una variable.\n", strings.Repeat("y otro ", i%3))
	}
	l := h.Leer(sb.String(), FuenteLectura, "notas")
	e := h.Estado()
	if e.DelUsuario != 1 || e.ConSignificado != 67 || e.Frases != l.Entrenadas || e.Reservadas == 0 || !e.HayFluidez || e.Fluidez >= 0 || e.Ngramas == 0 {
		t.Errorf("estado: %+v (lectura %+v)", e, l)
	}
	if l.Entrenadas+e.Reservadas != l.Frases {
		t.Errorf("cada frase se entrena o se reserva: %+v %+v", l, e)
	}
}

func TestLimites(t *testing.T) {
	// trigram table: about 520 000 trigrams go in, the table never passes the bound
	m := nuevoModelo()
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 40000; i++ {
		toks := make([]string, 12)
		for j := range toks {
			toks[j] = fmt.Sprintf("p%d", rng.Intn(20000))
		}
		m.entrenar(toks, nil)
		if len(m.tri) > MaxTrigramas {
			t.Fatalf("la tabla pasa del límite: %d", len(m.tri))
		}
	}
	if len(m.tri) < MaxTrigramas/2 {
		t.Errorf("podó demasiado: %d", len(m.tri))
	}
	// vocabulary
	v := nuevoModelo()
	for i := 0; i < MaxVocabulario+10; i++ {
		v.id(fmt.Sprintf("w%d", i))
	}
	if len(v.vocab) != MaxVocabulario || v.id("otra") != idRaro || v.id(strings.Repeat("a", 41)) != idRaro {
		t.Fatalf("vocabulario: %d", len(v.vocab))
	}
	// a read is capped at 1 MiB
	h := Nuevo()
	frase := "Una función hace algo con un valor.\n"
	l := h.Leer(strings.Repeat(frase, (2<<20)/len(frase)), FuenteUsuario, "x")
	if l.Frases > (MaxLectura/len(frase))+1 || l.Frases < (MaxLectura/len(frase))-1 {
		t.Errorf("leí %d frases de un texto de 2 MiB", l.Frases)
	}
	// dictionary size
	d := Nuevo()
	for i := 0; len(d.sig) < MaxSignificados; i++ {
		d.mu.Lock()
		d.sig[fmt.Sprintf("k%d", i)] = &Significado{Palabra: fmt.Sprintf("k%d", i), Definicion: "x"}
		d.mu.Unlock()
	}
	if _, err := d.Ensenar("nuevapalabra", "algo nuevo", FuenteUsuario, ""); err != ErrLleno {
		t.Errorf("diccionario lleno: %v", err)
	}
}

func TestPersistencia(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "datos")
	h, err := Abrir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, Archivo))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("habla.json: %v %v", fi, err)
	}
	if di, _ := os.Stat(dir); di.Mode().Perm() != 0o700 {
		t.Errorf("carpeta: %v", di.Mode().Perm())
	}
	if _, err := h.Ensenar("capicúa", "palíndromo", FuenteUsuario, "me lo enseñaste tú el 17/10"); err != nil {
		t.Fatal(err)
	}
	h.Leer(parrafoFunciones, FuenteUsuario, "x")
	if err := h.Olvidar("teclado"); err != nil {
		t.Fatal(err)
	}
	h.MarcarLeida("notas_go", "v1")
	if err := h.Guardar(); err != nil {
		t.Fatal(err)
	}
	antes := h.Estado()
	fa, _ := h.Decir(rand.New(rand.NewSource(5)), []string{"funciones"}, 25)

	g, err := Abrir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := g.Buscar("capicua"); !ok || s.Definicion != "palíndromo" || s.Origen != "me lo enseñaste tú el 17/10" {
		t.Errorf("capicúa: %+v", s)
	}
	if _, ok := g.Buscar("teclado"); ok {
		t.Error("teclado estaba olvidado")
	}
	if g.Leida("notas_go") != "v1" {
		t.Error("lecturas")
	}
	despues := g.Estado()
	if despues.Vocabulario != antes.Vocabulario || despues.Frases != antes.Frases || despues.Ngramas != antes.Ngramas || despues.ConSignificado != antes.ConSignificado {
		t.Errorf("estado: %+v / %+v", antes, despues)
	}
	fb, _ := g.Decir(rand.New(rand.NewSource(5)), []string{"funciones"}, 25)
	if fa.Texto != fb.Texto || fa.Texto == "" {
		t.Errorf("misma frase tras guardar: %q / %q", fa.Texto, fb.Texto)
	}
	if len(g.Avisos()) != 0 {
		t.Errorf("avisos: %v", g.Avisos())
	}
	if _, err := os.Stat(filepath.Join(dir, Archivo+".tmp")); err == nil {
		t.Error("quedó el temporal")
	}
	// GuardarSiHaceFalta: reading alone waits, teaching saves now
	g.Leer("El programa corre en la consola.", FuenteUsuario, "x")
	if err := g.GuardarSiHaceFalta(time.Hour); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	esperando := g.sucio
	g.mu.Unlock()
	if !esperando {
		t.Error("lo leído debía esperar al intervalo")
	}
	if _, err := g.Ensenar("bucle", "repetir algo", FuenteUsuario, "x"); err != nil {
		t.Fatal(err)
	}
	if err := g.GuardarSiHaceFalta(time.Hour); err != nil {
		t.Fatal(err)
	}
	k, _ := Abrir(dir)
	if _, ok := k.Buscar("bucle"); !ok {
		t.Error("lo enseñado se guarda enseguida")
	}
}

func TestArchivoRoto(t *testing.T) {
	for nombre, contenido := range map[string]string{
		"basura":  "{esto no es json",
		"version": `{"version": 99, "significados": []}`,
	} {
		t.Run(nombre, func(t *testing.T) {
			dir := t.TempDir()
			ruta := filepath.Join(dir, Archivo)
			if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
				t.Fatal(err)
			}
			h, err := Abrir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if b, err := os.ReadFile(ruta + ".roto"); err != nil || string(b) != contenido {
				t.Errorf("copia del roto: %q %v", b, err)
			}
			if len(h.Avisos()) == 0 || !strings.Contains(h.Avisos()[0], "habla.json.roto") {
				t.Errorf("avisos: %v", h.Avisos())
			}
			if len(h.Lista()) != 66 {
				t.Errorf("debe empezar con la semilla: %d", len(h.Lista()))
			}
			if _, err := os.Stat(ruta); err != nil {
				t.Errorf("debe escribir uno nuevo: %v", err)
			}
		})
	}
}

func TestModeloRoto(t *testing.T) {
	dir := t.TempDir()
	a := archivo{Version: Version, Significados: []Significado{{Palabra: "capicúa", Definicion: "palíndromo", Fuente: FuenteUsuario, Confianza: 1}},
		Modelo: archivoModelo{Vocabulario: []string{"<s>", "</s>", "<?>", "<num>", "hola"}, Trigramas: []uint32{0, 0, 99, 1}}}
	b, _ := json.Marshal(a)
	if err := os.WriteFile(filepath.Join(dir, Archivo), b, 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := Abrir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := h.Buscar("capicúa"); !ok || s.Definicion != "palíndromo" {
		t.Errorf("los significados se conservan: %+v", s)
	}
	if len(h.Avisos()) != 1 || !strings.Contains(h.Avisos()[0], "desde cero") {
		t.Errorf("avisos: %v", h.Avisos())
	}
	if len(h.Lista()) != 67 {
		t.Errorf("la semilla se completa: %d", len(h.Lista()))
	}
}

func TestCarrera(t *testing.T) {
	h, err := Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g)))
			for i := 0; i < 15; i++ {
				switch (g + i) % 7 {
				case 0:
					h.Leer(parrafoFunciones, FuenteUsuario, "x")
				case 1:
					_, _ = h.Ensenar(fmt.Sprintf("palabra%c", 'a'+g), "algo que se enseña", FuenteUsuario, "x")
				case 2:
					h.Buscar("funciones")
				case 3:
					h.Relacion("aprender", "olvidar")
				case 4:
					h.Decir(rng, []string{"función"}, 25)
				case 5:
					h.Estado()
					h.Desconocidas("los gatos maúllan")
				case 6:
					_ = h.Guardar()
					h.Lista()
				}
			}
		}(g)
	}
	wg.Wait()
	if err := h.Guardar(); err != nil {
		t.Fatal(err)
	}
}

func TestTextoAyudas(t *testing.T) {
	for w, quiero := range map[string]string{
		"memoria": "memor", "memorias": "memor", "guardar": "guard", "guardas": "guard", "quedarte": "qued",
		"funciones": "funci", "función": "funci", "traer": "tra", "trae": "tra", "números": "numer",
	} {
		if g := tallo(w); g != quiero {
			t.Errorf("tallo(%q) = %q, quiero %q", w, g, quiero)
		}
	}
	for w, quiero := range map[string]string{
		"funciones": "función", "variables": "variable", "papeles": "papel", "lápices": "lápiz", "valores": "valor",
		"clases": "clase", "casa": "casa", "nombres": "nombre",
	} {
		if g := singular(w); g != quiero {
			t.Errorf("singular(%q) = %q, quiero %q", w, g, quiero)
		}
	}
	fs := partirFrases("Usa strings.Fields. Luego 3.5 es un número\n- defer: dejar algo\n\n¿Qué es?")
	if len(fs) != 4 || fs[0] != "Usa strings.Fields." || fs[2] != "defer: dejar algo" {
		t.Errorf("frases: %q", fs)
	}
}
