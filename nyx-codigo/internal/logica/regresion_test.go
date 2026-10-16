package logica

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

// Regression tests for the problems found in review (one per bug).

func resolutorPorNombre(t *testing.T, rs []nucleo.Resolutor, nombre string) nucleo.Resolutor {
	t.Helper()
	for _, r := range rs {
		if r.Nombre() == nombre {
			return r
		}
	}
	t.Fatalf("no hay resolutor %s", nombre)
	return nil
}

func preguntar(t *testing.T, r nucleo.Resolutor, texto string) nucleo.Respuesta {
	t.Helper()
	resp, err := r.Resolver(context.Background(), &nucleo.Pregunta{Texto: texto}, nucleo.NuevaTraza(nil, nil).Raiz())
	if err != nil {
		t.Fatalf("%s(%q): %v", r.Nombre(), texto, err)
	}
	return resp
}

// A "ningún" rule used to share its id with its symmetric reading, so the re-check of the chain
// failed and every yes/no question answered with an internal error.
func TestRegresionNingunNoRompeLasPreguntas(t *testing.T) {
	b := &nucleotest.BaseHechosMemoria{}
	h := resolutorPorNombre(t, Resolutores(b, b, lem), "logica.hechos")
	preguntar(t, h, "recuerda que Toby es un perro")
	preguntar(t, h, "recuerda que todos los perros son mamíferos")
	if r := preguntar(t, h, "¿Toby es mamífero?"); r.Texto != "Sí: Toby es mamífero." {
		t.Fatalf("antes del «ningún»: %q", r.Texto)
	}
	preguntar(t, h, "recuerda que ningún gato es perro")
	preguntar(t, h, "recuerda que Michi es un gato")
	casos := map[string]string{
		"¿Toby es mamífero?":             "Sí: Toby es mamífero.",
		"¿Toby es un perro?":             "Sí: Toby es un perro.",
		"¿Toby es un gato?":              "No: Toby no es gato.",
		"¿Michi es un perro?":            "No: Michi no es perro.",
		"¿capicúa significa palíndromo?": "No lo sé",
	}
	for q, quiero := range casos {
		if r := preguntar(t, h, q); !strings.HasPrefix(r.Texto, quiero) {
			t.Errorf("%q = %q, quiero %q", q, r.Texto, quiero)
		}
	}
	// one-shot, with the facts in the question itself
	b2 := &nucleotest.BaseHechosMemoria{}
	h2 := resolutorPorNombre(t, Resolutores(b2, b2, lem), "logica.hechos")
	if r := preguntar(t, h2, "ningún gato es perro. Michi es un gato. ¿Michi es un perro?"); r.Texto != "No: Michi no es perro." {
		t.Fatalf("de una vez: %q", r.Texto)
	}
	// the re-check accepts every derivation, including the ones of the symmetric reading
	ders := Encadenar(context.Background(), b, 0)
	if len(ders) < 3 || !comprobarCadena(context.Background(), b, ders) {
		t.Fatalf("comprobarCadena rechaza %+v", ders)
	}
	// derived ids never collide with stored ones
	b3 := &nucleotest.BaseHechosMemoria{H: []nucleo.Hecho{{ID: "d1", Sujeto: "toby", Relacion: "es_un", Objeto: "perro"}}, R: b.R}
	for _, d := range Encadenar(context.Background(), b3, 0) {
		if d.Hecho.ID == "d1" {
			t.Fatalf("id repetido: %+v", d)
		}
	}
}

// Comma-separated premises in symbolic notation and two conditionals joined by "y si" used to be
// absorbed into one implication, so valid arguments got "No se sigue".
func TestRegresionVariasFormulasEnUnaFrase(t *testing.T) {
	rs := Resolutores(nil, nil, lem)
	p := resolutorPorNombre(t, rs, "logica.proposicional")
	casos := []string{
		"p → q, p. ¿q?",
		"p -> q, q -> r. ¿p -> r?",
		"si llueve, el suelo se moja y si el suelo se moja, la gente resbala. llueve. ¿la gente resbala?",
		"si llueve entonces hace frío y si hace frío entonces nieva. llueve. por lo tanto nieva",
		"si estudias apruebas y si apruebas te gradúas. estudias. ¿te gradúas?",
	}
	for _, c := range casos {
		r := preguntar(t, p, c)
		if !strings.HasPrefix(r.Texto, "Sí, se sigue") {
			t.Errorf("%q = %q (%s)", c, r.Texto, r.Entendi)
		}
	}
	arg, err := AnalizarArgumento("p → q, p. ¿q?", lem)
	if err != nil || len(arg.Premisas) != 2 {
		t.Fatalf("premisas = %v, %v", arg.Premisas, err)
	}
	if NombreRegla(arg.Premisas, arg.Conclusion) != "modus ponens" {
		t.Fatalf("regla = %q", NombreRegla(arg.Premisas, arg.Conclusion))
	}
	// a comma inside "si A, B" still joins the condition and the consequent
	arg, err = AnalizarArgumento("si llueve, el suelo se moja. llueve. ¿el suelo se moja?", lem)
	if err != nil || len(arg.Premisas) != 2 {
		t.Fatalf("si A, B: %v %v", arg.Premisas, err)
	}
	if _, ok := arg.Premisas[0].(Implica); !ok {
		t.Fatalf("si A, B no es una implicación: %v", arg.Premisas[0])
	}
}

// "¿qué significa X?" was read as the fact («que» significa «X»).
func TestRegresionQueSignifica(t *testing.T) {
	if _, _, err := LeerHecho("¿qué significa capicúa?", lem); err == nil {
		t.Fatal("«¿qué significa capicúa?» no es un hecho")
	}
	q, ok := LeerPreguntaAbierta("¿qué significa capicúa?")
	if !ok || q.Sujeto != "capicua" || q.Relacion != "significa" {
		t.Fatalf("pregunta abierta = %+v %v", q, ok)
	}
	b := &nucleotest.BaseHechosMemoria{}
	rs := Resolutores(b, b, lem)
	h := resolutorPorNombre(t, rs, "logica.hechos")
	preguntar(t, h, "recuerda que capicúa significa palíndromo")
	p := &nucleo.Pregunta{Texto: "¿qué significa capicúa?"}
	if rec := h.Reconoce(p); rec.Puntos < 0.85 {
		t.Fatalf("Reconoce = %v", rec.Puntos)
	}
	r := preguntar(t, h, "¿qué significa capicúa?")
	if !strings.Contains(r.Texto, "palíndromo") || strings.Contains(r.Texto, "«que»") || !r.Exito {
		t.Fatalf("respuesta = %+v", r)
	}
	// "¿qué es X?" with deductions, and an unknown word
	preguntar(t, h, "recuerda que Toby es un perro")
	preguntar(t, h, "recuerda que todos los perros son mamíferos")
	r = preguntar(t, h, "¿qué es Toby?")
	if !strings.Contains(r.Texto, "perro") || !strings.Contains(r.Texto, "mamífero") {
		t.Fatalf("¿qué es Toby? = %q", r.Texto)
	}
	r = preguntar(t, h, "¿qué significa ornitorrinco?")
	if r.Exito || !strings.HasPrefix(r.Texto, "No lo sé") {
		t.Fatalf("palabra desconocida = %+v", r)
	}
	// questions about Go are not claimed by the facts resolver
	for _, q := range []string{"¿qué es una goroutine?", "¿es eficiente este código?", "¿qué es defer?"} {
		if rec := h.Reconoce(&nucleo.Pregunta{Texto: q}); rec.Puntos > 0.3 {
			t.Errorf("Reconoce(%q) = %v", q, rec.Puntos)
		}
	}
}

// Deeply nested parentheses made the parser quadratic: they are now rejected quickly, and the
// largest accepted inputs stay fast.
func TestRegresionParserAcotado(t *testing.T) {
	rs := Resolutores(nil, nil, lem)
	p := resolutorPorNombre(t, rs, "logica.proposicional")
	casos := []string{
		strings.Repeat("(", 8000) + "p" + strings.Repeat(")", 8000) + " -> q. p. ¿q?",
		strings.Repeat(strings.Repeat("(", 59)+"p"+strings.Repeat(")", 59)+" y ", 15) + "q. ¿q?",
		strings.Repeat("si a ", 39) + "b. a. ¿b?",
		strings.Repeat("si llueve el suelo se moja. ", 250) + "¿se moja?",
	}
	for _, c := range casos {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		ini := time.Now()
		_, err := p.Resolver(ctx, &nucleo.Pregunta{Texto: c}, nucleo.NuevaTraza(nil, nil).Raiz())
		cancel()
		if d := time.Since(ini); d > 2*time.Second {
			t.Errorf("%d bytes tardaron %v (%v)", len(c), d, err)
		}
	}
	_, err := AnalizarArgumento(strings.Repeat("(", 8000)+"p"+strings.Repeat(")", 8000)+". ¿p?", lem)
	if err == nil {
		t.Fatal("8000 paréntesis anidados deberían rechazarse")
	}
}

// Malformed input to the exported SAT functions must give an error, never a panic.
func TestRegresionDPLLEntradasMalas(t *testing.T) {
	ctx := context.Background()
	if _, _, err := DPLL(ctx, nil, -1, 0); err == nil {
		t.Fatal("nVars < 0 debería dar error")
	}
	if _, _, err := DPLL(ctx, [][]int{{math.MinInt}}, 3, 0); err == nil {
		t.Fatal("un literal fuera de rango debería dar error")
	}
	if _, _, err := DPLL(ctx, [][]int{{1}}, math.MaxInt32, 0); !errors.Is(err, nucleo.ErrNoSoportado) {
		t.Fatalf("demasiadas variables: %v", err)
	}
	if _, _, err := Satisfacible(ctx, []Formula{Atomo{-1}}, 1); err == nil {
		t.Fatal("un átomo negativo debería dar error")
	}
	if _, err := TablaVerdad([]Formula{Atomo{1 << 40}}, &Atomos{}, 0); err == nil {
		t.Fatal("un átomo enorme debería dar error")
	}
	if m, sat, err := DPLL(ctx, nil, 0, 0); err != nil || !sat || len(m) != 0 {
		t.Fatalf("CNF vacía: %v %v %v", m, sat, err)
	}
}

// Knights and knaves with feminine words, and "él" with a single person.
func TestRegresionCaballerosFemeninoYEl(t *testing.T) {
	rs := Resolutores(nil, nil, lem)
	c := resolutorPorNombre(t, rs, "logica.caballeros")
	texto := "Ana dice que Bea es escudera. Bea dice que Ana y ella son caballeras."
	if rec := c.Reconoce(&nucleo.Pregunta{Texto: texto}); rec.Puntos < 0.85 {
		t.Fatalf("Reconoce = %v", rec.Puntos)
	}
	if r := preguntar(t, c, texto); r.Texto != "Ana es caballera y Bea es escudera." {
		t.Fatalf("Ana y Bea: %q", r.Texto)
	}
	if r := preguntar(t, c, "A dice que él es escudero."); !strings.HasPrefix(r.Texto, "Es imposible") {
		t.Fatalf("A dice que él es escudero: %q", r.Texto)
	}
	if r := preguntar(t, c, "Carla dice que es mentirosa."); !strings.HasPrefix(r.Texto, "Es imposible") {
		t.Fatalf("Carla: %q", r.Texto)
	}
	// the masculine classic is unchanged
	if r := preguntar(t, c, "A dice que B es escudero. B dice que A y él son caballeros."); r.Texto != "A es caballero y B es escudero." {
		t.Fatalf("clásico: %q", r.Texto)
	}
}

// Answers used to show normalized lemmas ("toby es mamifero") and plural nouns after "es".
func TestRegresionTextosEnEspanol(t *testing.T) {
	r := resolver(t, Resolutores(nil, nil, lem), "logica.silogismo", "todos los perros son animales. ¿todos los animales son perros?")
	todo := todoTexto(r)
	if strings.Contains(todo, "es animales") || strings.Contains(todo, "ser perros") {
		t.Fatalf("plural tras «es»: %s", todo)
	}
	if !strings.Contains(todo, "algo que es animal sin ser perro") {
		t.Fatalf("contraejemplo: %s", todo)
	}
	if got := singularDe("mamíferos de sangre caliente", "mamifero de sangre caliente"); got != "mamífero de sangre caliente" {
		t.Fatalf("singularDe = %q", got)
	}
	if got := singularDe("peces", "pez"); got != "pez" {
		t.Fatalf("singularDe(peces) = %q", got)
	}
	b := &nucleotest.BaseHechosMemoria{}
	h := resolutorPorNombre(t, Resolutores(b, b, lem), "logica.hechos")
	preguntar(t, h, "recuerda que Toby es un perro")
	preguntar(t, h, "recuerda que todos los perros son mamíferos")
	preguntar(t, h, "recuerda que todos los mamíferos son animales")
	r = preguntar(t, h, "¿Toby es un animal?")
	todo = todoTexto(r)
	if !strings.Contains(todo, "así que Toby es mamífero") || strings.Contains(todo, "toby") {
		t.Fatalf("cadena sin las formas escritas: %s", todo)
	}
	if r.Entendi != "Así lo entendí: ¿Toby es animal?" {
		t.Fatalf("entendí = %q", r.Entendi)
	}
}
