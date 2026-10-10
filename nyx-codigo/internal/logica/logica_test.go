package logica

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
)

var lem = nucleotest.LematizadorSimple

func TestPONoPEsTautologia(t *testing.T) {
	f, at, _, err := AnalizarFormula("p o no p", nil, lem)
	if err != nil {
		t.Fatal(err)
	}
	if len(at.Frases) != 1 {
		t.Fatalf("átomos = %v", at.Frases)
	}
	ok, contra := Tautologia(f, len(at.Frases))
	if !ok || contra != nil {
		t.Fatalf("p o no p debería ser tautología: %v %v", ok, contra)
	}
	g, at2, _, _ := AnalizarFormula("p y no p", nil, lem)
	if ok, _ := Tautologia(g, len(at2.Frases)); ok {
		t.Fatal("p y no p no es tautología")
	}
	h, at3, _, _ := AnalizarFormula("(p -> q) & p", nil, lem)
	if ok, contra := Tautologia(h, len(at3.Frases)); ok || contra == nil {
		t.Fatal("(p -> q) & p no es tautología y debe dar contramodelo")
	}
}

func TestModusPonens(t *testing.T) {
	arg, err := AnalizarArgumento("si llueve el suelo se moja; llueve. ¿se moja?", lem)
	if err != nil {
		t.Fatal(err)
	}
	if len(arg.Premisas) != 2 || arg.Conclusion == nil {
		t.Fatalf("argumento mal leído: %+v", arg)
	}
	if got := arg.Atomos.Frases; len(got) != 2 || got[0] != "llueve" || got[1] != "el suelo se moja" {
		t.Fatalf("átomos = %q", got)
	}
	valida, contra, err := Implicacion(context.Background(), arg.Premisas, arg.Conclusion, len(arg.Atomos.Frases))
	if err != nil || !valida || contra != nil {
		t.Fatalf("modus ponens debe ser válido: %v %v %v", valida, contra, err)
	}
	if n := NombreRegla(arg.Premisas, arg.Conclusion); n != "modus ponens" {
		t.Fatalf("regla = %q", n)
	}
	nota := strings.Join(arg.Notas, "\n")
	if !strings.Contains(nota, "«se moja»") || !strings.Contains(nota, "«el suelo se moja»") {
		t.Fatalf("falta la nota del emparejamiento: %q", nota)
	}
}

func TestAfirmarConsecuente(t *testing.T) {
	arg, err := AnalizarArgumento("si llueve el suelo se moja; el suelo se moja. ¿llueve?", lem)
	if err != nil {
		t.Fatal(err)
	}
	valida, contra, err := Implicacion(context.Background(), arg.Premisas, arg.Conclusion, len(arg.Atomos.Frases))
	if err != nil || valida {
		t.Fatalf("no debe ser válido: %v %v", valida, err)
	}
	// counter-model: llueve false, se moja true
	if contra[0] || !contra[1] {
		t.Fatalf("contramodelo = %v", contra)
	}
	desc := DescribirModelo(contra, arg.Atomos, []int{0, 1})
	if !strings.Contains(desc, "«llueve» es falso") || !strings.Contains(desc, "«el suelo se moja» es verdad") {
		t.Fatalf("descripción = %q", desc)
	}
	if f := Falacia(arg.Premisas, arg.Conclusion); f != "afirmar el consecuente" {
		t.Fatalf("falacia = %q", f)
	}
	if n := NombreRegla(arg.Premisas, arg.Conclusion); n != "" {
		t.Fatalf("no debería tener nombre de regla válida: %q", n)
	}
}

func TestOtrasReglas(t *testing.T) {
	p, q, r := Atomo{0}, Atomo{1}, Atomo{2}
	casos := []struct {
		prem   []Formula
		concl  Formula
		nombre string
		falac  string
	}{
		{[]Formula{Implica{p, q}, No{q}}, No{p}, "modus tollens", ""},
		{[]Formula{Implica{p, q}, Implica{q, r}}, Implica{p, r}, "silogismo hipotético", ""},
		{[]Formula{O{p, q}, No{p}}, q, "silogismo disyuntivo", ""},
		{[]Formula{Implica{p, r}, Implica{q, r}, O{p, q}}, r, "dilema constructivo", ""},
		{[]Formula{No{Y{p, q}}}, O{No{p}, No{q}}, "De Morgan", ""},
		{[]Formula{Implica{p, q}, No{p}}, No{q}, "", "negar el antecedente"},
	}
	for _, c := range casos {
		v, _, err := Implicacion(context.Background(), c.prem, c.concl, 3)
		if err != nil {
			t.Fatal(err)
		}
		if c.nombre != "" {
			if !v || NombreRegla(c.prem, c.concl) != c.nombre {
				t.Errorf("%v ⊢ %v: válido=%v nombre=%q, quería %q", c.prem, c.concl, v, NombreRegla(c.prem, c.concl), c.nombre)
			}
		}
		if c.falac != "" && (v || Falacia(c.prem, c.concl) != c.falac) {
			t.Errorf("%v ⊢ %v: válido=%v falacia=%q", c.prem, c.concl, v, Falacia(c.prem, c.concl))
		}
	}
}

func TestPalomarCuatroEnTres(t *testing.T) {
	// variable x(i,j): pigeon i in hole j, i<4, j<3 → var i*3+j+1
	x := func(i, j int) int { return i*3 + j + 1 }
	var cnf [][]int
	for i := 0; i < 4; i++ {
		cnf = append(cnf, []int{x(i, 0), x(i, 1), x(i, 2)})
	}
	for j := 0; j < 3; j++ {
		for a := 0; a < 4; a++ {
			for b := a + 1; b < 4; b++ {
				cnf = append(cnf, []int{-x(a, j), -x(b, j)})
			}
		}
	}
	ini := time.Now()
	_, sat, err := DPLL(context.Background(), cnf, 12, 0)
	if err != nil || sat {
		t.Fatalf("palomar 4→3 debe ser insatisfacible: sat=%v err=%v", sat, err)
	}
	if d := time.Since(ini); d > time.Second {
		t.Fatalf("tardó %v", d)
	}
}

func TestDPLLContraFuerzaBruta(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	sats := 0
	for k := 0; k < 500; k++ {
		n := 12
		m := 30 + rng.Intn(30)
		cnf := make([][]int, m)
		for i := range cnf {
			for j := 0; j < 3; j++ {
				l := rng.Intn(n) + 1
				if rng.Intn(2) == 0 {
					l = -l
				}
				cnf[i] = append(cnf[i], l)
			}
		}
		modelo, sat, err := DPLL(context.Background(), cnf, n, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, satF := satFuerzaCNF(cnf, n)
		if sat != satF {
			t.Fatalf("instancia %d: DPLL=%v fuerza bruta=%v", k, sat, satF)
		}
		if sat {
			sats++
			for _, c := range cnf {
				ok := false
				for _, l := range c {
					ok = ok || (l > 0 && modelo[l-1]) || (l < 0 && !modelo[-l-1])
				}
				if !ok {
					t.Fatalf("instancia %d: el modelo no cumple %v", k, c)
				}
			}
		}
	}
	if sats == 0 || sats == 500 {
		t.Fatalf("las instancias deberían mezclar SAT y UNSAT (sat=%d)", sats)
	}
}

func TestTseitinEquisatisfacible(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var gen func(prof int) Formula
	gen = func(prof int) Formula {
		if prof == 0 || rng.Intn(4) == 0 {
			return Atomo{rng.Intn(4)}
		}
		switch rng.Intn(5) {
		case 0:
			return No{gen(prof - 1)}
		case 1:
			return Y{gen(prof - 1), gen(prof - 1)}
		case 2:
			return O{gen(prof - 1), gen(prof - 1)}
		case 3:
			return Implica{gen(prof - 1), gen(prof - 1)}
		}
		return Equiv{gen(prof - 1), gen(prof - 1)}
	}
	for k := 0; k < 300; k++ {
		f := gen(4)
		_, satD, err := Satisfacible(context.Background(), []Formula{f}, 4)
		if err != nil {
			t.Fatal(err)
		}
		_, satF := fuerzaBruta([]Formula{f}, 4)
		if satD != satF {
			t.Fatalf("%v: DPLL=%v tabla=%v", f, satD, satF)
		}
	}
}

func TestResolucion(t *testing.T) {
	p, q := Atomo{0}, Atomo{1}
	pasos, ok := Resolucion([]Formula{Implica{p, q}, p}, q, 2)
	if !ok {
		t.Fatalf("debería encontrar la prueba: %v", pasos)
	}
	ultimo := pasos[len(pasos)-1]
	if !strings.Contains(ultimo, "□") {
		t.Fatalf("la prueba debe terminar en la cláusula vacía: %v", pasos)
	}
	if _, ok := Resolucion([]Formula{Implica{p, q}, q}, p, 2); ok {
		t.Fatal("afirmar el consecuente no se prueba por resolución")
	}
}

func TestTablaVerdad(t *testing.T) {
	arg, err := AnalizarArgumento("si llueve, el suelo se moja. llueve. por lo tanto el suelo se moja", lem)
	if err != nil {
		t.Fatal(err)
	}
	tb, err := TablaVerdad(append(arg.Premisas, arg.Conclusion), arg.Atomos, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Filas) != 4 || len(tb.Cabecera) != 5 {
		t.Fatalf("tabla = %+v", tb)
	}
	if !strings.Contains(tb.Titulo, "«llueve»") {
		t.Fatalf("título = %q", tb.Titulo)
	}
	_, contras, err := tablaArgumento(arg.Premisas, arg.Conclusion, arg.Atomos)
	if err != nil || contras != 0 {
		t.Fatalf("contraejemplos = %d, %v", contras, err)
	}
	var muchas []Formula
	for i := 0; i < 13; i++ {
		muchas = append(muchas, Atomo{i})
	}
	if _, err := TablaVerdad(muchas, nil, 0); err == nil {
		t.Fatal("13 átomos deben dar error")
	}
}

func TestAnalizarConectivas(t *testing.T) {
	casos := []struct {
		texto string
		forma string
	}{
		{"si llueve, entonces hace frío", "p → q"},
		{"llueve si y solo si hace frío", "p ↔ q"},
		{"llueve solo si hace frío", "p → q"},
		{"o llueve o hace frío", "p ∨ q"},
		{"ni llueve ni hace frío", "¬p ∧ ¬q"},
		{"llueve pero no hace frío", "p ∧ ¬q"},
		{"no es cierto que llueva y haga frío", "¬(p ∧ q)"},
		{"el suelo no se moja", "¬p"},
		{"(p -> q) & !q", "(p → q) ∧ ¬q"},
		{"p ∨ ¬p", "p ∨ ¬p"},
		{"hace frío si llueve", "q → p"},
	}
	for _, c := range casos {
		f, at, _, err := AnalizarFormula(c.texto, nil, lem)
		if err != nil {
			t.Errorf("%q: %v", c.texto, err)
			continue
		}
		if got := EscribirCon(f, at.Nombres(len(at.Frases))); got != c.forma {
			t.Errorf("%q → %q, quería %q (átomos %q)", c.texto, got, c.forma, at.Frases)
		}
	}
}

func TestBuscarAtomo(t *testing.T) {
	a := &Atomos{}
	i, _ := a.Obtener("el suelo se moja", lem)
	j, nota, ok := a.Buscar("se moja", lem)
	if !ok || j != i || !strings.Contains(nota, "Entiendo «se moja» como «el suelo se moja»") {
		t.Fatalf("Buscar = %d %q %v", j, nota, ok)
	}
	if _, _, ok := a.Buscar("hace sol", lem); ok {
		t.Fatal("«hace sol» no debería emparejar")
	}
	k, nota := a.Obtener("el suelo se moja", lem)
	if k != i || nota != "" {
		t.Fatalf("emparejamiento exacto sin nota: %d %q", k, nota)
	}
}

func cat(t *testing.T, s string) Categorica {
	t.Helper()
	c, err := AnalizarCategorica(s, lem)
	if err != nil {
		t.Fatalf("%q: %v", s, err)
	}
	return c
}

func TestBarbara(t *testing.T) {
	prem := []Categorica{cat(t, "todos los M son P"), cat(t, "todos los S son M")}
	concl := cat(t, "todos los S son P")
	r, err := Silogismo(context.Background(), prem, &concl)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Valido || r.Nombre != "Barbara" {
		t.Fatalf("Barbara: %+v", r)
	}
}

func TestSilogismoInvalido(t *testing.T) {
	prem := []Categorica{cat(t, "todos los A son B"), cat(t, "algunos B son C")}
	concl := cat(t, "algunos A son C")
	r, err := Silogismo(context.Background(), prem, &concl)
	if err != nil {
		t.Fatal(err)
	}
	if r.Valido || r.Contraejemplo == "" {
		t.Fatalf("no debería seguirse: %+v", r)
	}
	if !strings.Contains(r.Contraejemplo, "B") || !strings.Contains(r.Contraejemplo, "C") {
		t.Fatalf("contraejemplo = %q", r.Contraejemplo)
	}
}

func TestModosClasicos(t *testing.T) {
	casos := []struct {
		p1, p2, c, nombre string
	}{
		{"ningún M es P", "todos los S son M", "ningún S es P", "Celarent"},
		{"todos los M son P", "algunos S son M", "algunos S son P", "Darii"},
		{"ningún M es P", "algunos S son M", "algunos S no son P", "Ferio"},
		{"ningún P es M", "todos los S son M", "ningún S es P", "Cesare"},
		{"todos los P son M", "algunos S no son M", "algunos S no son P", "Baroco"},
		{"todos los M son P", "todos los M son S", "algunos S son P", "Darapti"},
	}
	for _, c := range casos {
		prem := []Categorica{cat(t, c.p1), cat(t, c.p2)}
		concl := cat(t, c.c)
		r, err := Silogismo(context.Background(), prem, &concl)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Valido || r.Nombre != c.nombre {
			t.Errorf("%s; %s ⊢ %s: válido=%v nombre=%q, quería %s", c.p1, c.p2, c.c, r.Valido, r.Nombre, c.nombre)
		}
	}
}

func TestSocrates(t *testing.T) {
	prem := []Categorica{cat(t, "todos los hombres son mortales"), cat(t, "Sócrates es un hombre")}
	concl := cat(t, "¿Sócrates es mortal?")
	if concl.Cuant != Individuo || concl.FraseSujeto != "Sócrates" {
		t.Fatalf("conclusión = %+v", concl)
	}
	r, err := Silogismo(context.Background(), prem, &concl)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Valido {
		t.Fatalf("Sócrates es mortal debería seguirse: %+v", r)
	}
	if !strings.HasPrefix(r.Nombre, "Barbara") {
		t.Fatalf("nombre = %q", r.Nombre)
	}
}

func TestQueSeConcluye(t *testing.T) {
	prem := []Categorica{cat(t, "todos los perros son mamíferos"), cat(t, "todos los mamíferos son animales")}
	r, err := Silogismo(context.Background(), prem, nil)
	if err != nil {
		t.Fatal(err)
	}
	encontrada := false
	for _, c := range r.Conclusiones {
		if strings.Contains(c, "perros") && strings.Contains(c, "animales") && strings.HasPrefix(c, "todos") {
			encontrada = true
		}
	}
	if !encontrada {
		t.Fatalf("conclusiones = %q", r.Conclusiones)
	}
}

func TestCadenaToby(t *testing.T) {
	b := &nucleotest.BaseHechosMemoria{}
	h, _, err := LeerHecho("recuerda que Toby es un perro", lem)
	if err != nil || h == nil {
		t.Fatal(err)
	}
	if h.Sujeto != "toby" || h.Relacion != "es_un" || h.Objeto != "perro" {
		t.Fatalf("hecho = %+v", h)
	}
	b.GuardarHecho(*h)
	_, r, err := LeerHecho("recuerda que todos los perros son mamíferos", lem)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if r.Si[0].Objeto != "perro" || r.Entonces.Objeto != "mamifero" || r.Si[0].Sujeto != "?x" {
		t.Fatalf("regla = %+v", r)
	}
	b.GuardarRegla(*r)
	ders := Encadenar(context.Background(), b, 0)
	if len(ders) != 1 || len(ders[0].Desde) != 2 {
		t.Fatalf("derivaciones = %+v", ders)
	}
	q, _, _ := LeerHecho("¿Toby es mamífero?", lem)
	resp, cadena := Consultar(context.Background(), b, *q)
	if resp != "si" || len(cadena) != 2 {
		t.Fatalf("respuesta = %q, cadena = %q", resp, cadena)
	}
	if !strings.Contains(cadena[0], "perro") {
		t.Fatalf("cadena = %q", cadena)
	}
	q2, _, _ := LeerHecho("¿Toby es un gato?", lem)
	if resp, _ := Consultar(context.Background(), b, *q2); resp != "no_se" {
		t.Fatalf("gato: %q", resp)
	}
	_, r2, _ := LeerHecho("ningún perro es un gato", lem)
	b.GuardarRegla(*r2)
	if resp, cad := Consultar(context.Background(), b, *q2); resp != "no" || len(cad) == 0 {
		t.Fatalf("con «ningún perro es un gato»: %q %q", resp, cad)
	}
}

func TestCaballeros(t *testing.T) {
	r, err := Caballeros(context.Background(), "A dice que B es un escudero. B dice que A y él son escuderos. ¿Qué es cada uno?")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Modelos) != 1 {
		t.Fatalf("modelos = %v (dichos %v)", r.Modelos, r.Dichos)
	}
	if got := DescribirCaballeros(r.Personas, r.Modelos[0]); got != "A es caballero y B es escudero" {
		t.Fatalf("solución = %q", got)
	}
	r2, err := Caballeros(context.Background(), "A dice: «B es escudero». B dice: «A y yo somos del mismo tipo».")
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Modelos) != 1 || DescribirCaballeros(r2.Personas, r2.Modelos[0]) != "A es caballero y B es escudero" {
		t.Fatalf("segundo: %v", r2.Modelos)
	}
	r3, err := Caballeros(context.Background(), "A dice que B es escudero. B dice que A es escudero.")
	if err != nil {
		t.Fatal(err)
	}
	if len(r3.Modelos) != 2 {
		t.Fatalf("tercero: %v", r3.Modelos)
	}
}

// --- resolvers (golden exam rows 38-43)

func resolver(t *testing.T, rs []nucleo.Resolutor, nombre, texto string) nucleo.Respuesta {
	t.Helper()
	for _, r := range rs {
		if r.Nombre() != nombre {
			continue
		}
		p := &nucleo.Pregunta{Texto: texto}
		tr := nucleo.NuevaTraza(nil, nil)
		resp, err := r.Resolver(context.Background(), p, tr.Raiz())
		if err != nil {
			t.Fatalf("%s(%q): %v", nombre, texto, err)
		}
		return resp
	}
	t.Fatalf("no hay resolutor %s", nombre)
	return nucleo.Respuesta{}
}

func mejor(rs []nucleo.Resolutor, texto string) string {
	p := &nucleo.Pregunta{Texto: texto}
	nombre, puntos := "", 0.0
	for _, r := range rs {
		if rec := r.Reconoce(p); rec.Puntos > puntos {
			nombre, puntos = r.Nombre(), rec.Puntos
		}
	}
	return nombre
}

func todoTexto(r nucleo.Respuesta) string {
	return r.Texto + "\n" + strings.Join(r.Parrafos, "\n") + "\n" + strings.Join(r.Justificacion, "\n")
}

func TestResolutoresExamen(t *testing.T) {
	b := &nucleotest.BaseHechosMemoria{}
	rs := Resolutores(b, b, lem)
	casos := []struct {
		texto, resolutor string
		quiere           []string
	}{
		{"si llueve el suelo se moja; llueve. ¿se moja?", "logica.proposicional", []string{"Sí", "modus ponens"}},
		{"si llueve el suelo se moja; el suelo se moja. ¿llueve?", "logica.proposicional", []string{"afirmar el consecuente"}},
		{"todos los A son B, algunos B son C, ¿algunos A son C?", "logica.silogismo", []string{"No se sigue"}},
		{"todos los hombres son mortales; Sócrates es un hombre; ¿Sócrates es mortal?", "logica.silogismo", []string{"Sí"}},
		{"¿es «p o no p» siempre verdad?", "logica.proposicional", []string{"siempre"}},
		{"A dice que B es un escudero. B dice que A y él son escuderos.", "logica.caballeros", []string{"A es caballero y B es escudero"}},
	}
	for _, c := range casos {
		if m := mejor(rs, c.texto); m != c.resolutor {
			t.Errorf("%q: reconoce %q, quería %q", c.texto, m, c.resolutor)
		}
		r := resolver(t, rs, c.resolutor, c.texto)
		if r.Nivel != nucleo.NivelCalculo || !r.Exito {
			t.Errorf("%q: nivel %q éxito %v", c.texto, r.Nivel, r.Exito)
		}
		for _, q := range c.quiere {
			if !strings.Contains(todoTexto(r), q) {
				t.Errorf("%q: falta %q en %q", c.texto, q, todoTexto(r))
			}
		}
	}
	// facts across three turns (#43)
	for _, texto := range []string{"recuerda que Toby es un perro", "recuerda que todos los perros son mamíferos"} {
		if m := mejor(rs, texto); m != "logica.hechos" {
			t.Errorf("%q: reconoce %q", texto, m)
		}
		r := resolver(t, rs, "logica.hechos", texto)
		if !r.Exito || r.Aprendido == "" {
			t.Errorf("%q: %+v", texto, r)
		}
	}
	if len(b.H) != 1 || len(b.R) != 1 {
		t.Fatalf("base = %+v", b)
	}
	texto := "¿Toby es mamífero?"
	if m := mejor(rs, texto); m != "logica.hechos" {
		t.Errorf("%q: reconoce %q", texto, m)
	}
	r := resolver(t, rs, "logica.hechos", texto)
	if !strings.Contains(todoTexto(r), "Sí") || !strings.Contains(todoTexto(r), "perro") || r.Nivel != nucleo.NivelCalculo {
		t.Fatalf("Toby: %+v", r)
	}
}

func TestResolutorQueSeConcluye(t *testing.T) {
	rs := Resolutores(nil, nil, lem)
	r := resolver(t, rs, "logica.proposicional", "si llueve, hace frío. llueve. ¿qué se concluye?")
	if !strings.Contains(r.Texto, "«hace frío»") {
		t.Fatalf("texto = %q", r.Texto)
	}
	r2 := resolver(t, rs, "logica.proposicional", "si llueve, el suelo se moja. el suelo no se moja. ¿llueve?")
	if !strings.Contains(r2.Texto, "lo contrario") {
		t.Fatalf("modus tollens a la pregunta: %q", r2.Texto)
	}
	r3 := resolver(t, rs, "logica.proposicional", "¿es «p y no p» una contradicción?")
	if !strings.HasPrefix(r3.Texto, "Sí") {
		t.Fatalf("contradicción: %q", r3.Texto)
	}
}

func TestReconoceRapido(t *testing.T) {
	rs := Resolutores(&nucleotest.BaseHechosMemoria{}, nil, lem)
	p := &nucleo.Pregunta{Texto: "si llueve el suelo se moja; llueve. ¿se moja? todos los A son B, algunos B son C"}
	ini := time.Now()
	for i := 0; i < 20; i++ {
		for _, r := range rs {
			r.Reconoce(p)
		}
	}
	if d := time.Since(ini) / 20; d > 5*time.Millisecond {
		t.Fatalf("Reconoce tarda %v", d)
	}
	if rec := rs[0].Reconoce(&nucleo.Pregunta{Texto: "haz una función que sume"}); rec.Puntos > 0.2 {
		t.Fatalf("no es lógica: %v", rec.Puntos)
	}
}
