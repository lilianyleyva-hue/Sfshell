package lengua

import (
	"math/big"
	"strings"
	"testing"

	"nyxcodigo/internal/nucleo"
)

func TestNumeros(t *testing.T) {
	casos := []struct {
		texto string
		valor string // RatString; "nil" for an unevaluated expression
	}{
		{"dos mil trescientos cuarenta y cinco", "2345"},
		{"tres cuartos", "3/4"},
		{"3,5", "7/2"},
		{"3.5", "7/2"},
		{"la mitad", "1/2"},
		{"un tercio", "1/3"},
		{"15%", "3/20"},
		{"quince por ciento", "3/20"},
		{"veintiuno", "21"},
		{"ciento uno", "101"},
		{"un millón", "1000000"},
		{"-4", "-4"},
		{"1.000.000", "1000000"},
		{"2^61-1", "nil"},
	}
	for _, c := range casos {
		ns := Numeros(c.texto)
		if len(ns) != 1 {
			t.Errorf("Numeros(%q) = %d números, esperaba 1", c.texto, len(ns))
			continue
		}
		got := "nil"
		if ns[0].Valor != nil {
			got = ns[0].Valor.RatString()
		}
		if got != c.valor {
			t.Errorf("Numeros(%q) = %s, esperaba %s", c.texto, got, c.valor)
		}
	}
	ns := Numeros("2^61-1")
	if ns[0].Texto != "2^61-1" {
		t.Errorf("la expresión debe quedar como texto: %q", ns[0].Texto)
	}
	ns = Numeros("un tren va a 80 km/h durante 2 horas y media")
	var vs []string
	for _, n := range ns {
		vs = append(vs, n.Valor.RatString())
	}
	if strings.Join(vs, " ") != "80 2 1/2" {
		t.Errorf("tren: %v", vs)
	}
	ns = Numeros("[1,2,3] -> 6 y María tiene doce caramelos")
	if len(ns) != 2 || ns[1].Valor.Cmp(big.NewRat(12, 1)) != 0 {
		t.Errorf("números fuera de listas: %+v", ns)
	}
}

func TestLema(t *testing.T) {
	casos := map[string]string{
		"leones": "leon", "peces": "pez", "sume": "sumar", "llueve": "llover", "suma": "sumar",
		"sumando": "sumar", "pares": "par", "mamíferos": "mamifero", "cuente": "contar", "devuelva": "devolver",
		"ordene": "ordenar", "invierta": "invertir", "números": "numero", "palabras": "palabra", "vocales": "vocal",
		"letras": "letra", "pida": "pedir", "lea": "leer", "listas": "lista", "cadenas": "cadena", "perros": "perro",
		"filtre": "filtrar", "impares": "impar", "multiplique": "multiplicar", "ordenando": "ordenar",
		"pruébala": "probar", "súmalos": "sumar", "de": "de", "es": "es",
	}
	for w, want := range casos {
		if got := Lema(w); got != want {
			t.Errorf("Lema(%q) = %q, esperaba %q", w, got, want)
		}
	}
}

func TestTokenizar(t *testing.T) {
	toks := Tokenizar(`suma [1, 2, 3] y "hola mundo" con «x» usando strings.Split 3,5`)
	var clases []string
	for _, tk := range toks {
		clases = append(clases, tk.Clase+":"+tk.Texto)
	}
	want := []string{"palabra:suma", "lista:[1, 2, 3]", "palabra:y", "cita:hola mundo", "palabra:con", "cita:x",
		"palabra:usando", "palabra:strings.Split", "numero:3,5"}
	if strings.Join(clases, "|") != strings.Join(want, "|") {
		t.Errorf("Tokenizar:\n got %v\nwant %v", clases, want)
	}
	s := "¿Cuántas MANERAS hay?"
	for _, tk := range Tokenizar(s) {
		if s[tk.Desde:tk.Hasta] != tk.Texto {
			t.Errorf("desplazamientos mal en %+v", tk)
		}
	}
	lt := Lematizar("dos mil trescientos cuarenta y cinco perros")
	if len(lt) != 2 || lt[0].Clase != "numero" || lt[0].Num.RatString() != "2345" || lt[1].Lema != "perro" {
		t.Errorf("Lematizar: %+v", lt)
	}
}

func TestDetectarSeguimiento(t *testing.T) {
	ecu := &nucleo.Contexto{Intencion: nucleo.IEcuacion, Texto: "resuelve 2x+3=11"}
	cod := &nucleo.Contexto{Intencion: nucleo.ICrearFuncion, Texto: "haz una función que sume los pares",
		Codigo: "func SumaPares(nums []int) int { return 0 }", Firma: &nucleo.Firma{Nombre: "SumaPares"}}
	casos := []struct {
		texto   string
		ctx     *nucleo.Contexto
		tipo    string // "" = not a follow-up
		numero  string
		entrada string
		ejemplo string
	}{
		{"¿y si fuera 13?", ecu, "cambiar_numero", "13", "", ""},
		{"y si son 20", ecu, "cambiar_numero", "20", "", ""},
		{"¿y si es -3?", ecu, "cambiar_numero", "-3", "", ""},
		{"y con 7", ecu, "cambiar_numero", "7", "", ""},
		{"¿por qué?", ecu, "por_que", "", "", ""},
		{"por que", cod, "por_que", "", "", ""},
		{"explícalo más simple", cod, "mas_simple", "", "", ""},
		{"más fácil", cod, "mas_simple", "", "", ""},
		{"con más detalle", cod, "mas_detalle", "", "", ""},
		{"de otra forma", cod, "otra_forma", "", "", ""},
		{"hazlo de otra manera", cod, "otra_forma", "", "", ""},
		{"está mal: con [2] debe dar 0", cod, "esta_mal", "", "", "con [2] debe dar 0"},
		{"está mal: con [2] da 2", cod, "esta_mal", "", "", "con [2] da 2"},
		{"no", cod, "esta_mal", "", "", ""},
		{"incorrecto", cod, "esta_mal", "", "", ""},
		{"perfecto", cod, "esta_bien", "", "", ""},
		{"gracias", cod, "esta_bien", "", "", ""},
		{"pruébala con [3,3]", cod, "probar_con", "", "[3,3]", ""},
		{`prueba con "hola"`, cod, "probar_con", "", `"hola"`, ""},
		{"y si la lista está vacía", cod, "vacia", "", "", ""},
		{"¿y con una lista vacía?", cod, "vacia", "", "", ""},
		{"ponle comentarios", cod, "comentarios", "", "", ""},
		{"hazlo más rápido", cod, "mas_rapido", "", "", ""},
		{"ahora con impares", cod, "modificar", "", "", ""},
		{"y para palabras", cod, "modificar", "", "", ""},
		{"ahora que devuelva el máximo", cod, "modificar", "", "", ""},
		{"explícalo", cod, "explicar_eso", "", "", ""},
		{"ejecútalo", cod, "ejecutar_eso", "", "", ""},
		{"hazlo programa", cod, "programa_eso", "", "", ""},
		{"haz una función que sume los números pares de una lista", cod, "", "", "", ""},
		{"resuelve x^2=4", ecu, "", "", "", ""},
		{"¿por qué?", nil, "", "", "", ""},
	}
	if len(casos) < 30 {
		t.Fatalf("faltan casos: %d", len(casos))
	}
	for _, c := range casos {
		s, ok := DetectarSeguimiento(c.texto, c.ctx)
		if c.tipo == "" {
			if ok {
				t.Errorf("%q no es un seguimiento, salió %q", c.texto, s.Tipo)
			}
			continue
		}
		if !ok || s.Tipo != c.tipo {
			t.Errorf("%q → %q (%v), esperaba %q", c.texto, s.Tipo, ok, c.tipo)
			continue
		}
		if c.numero != "" && (s.Numero == nil || s.Numero.Valor.RatString() != c.numero) {
			t.Errorf("%q: número %+v, esperaba %s", c.texto, s.Numero, c.numero)
		}
		if c.entrada != "" && s.Entrada != c.entrada {
			t.Errorf("%q: entrada %q, esperaba %q", c.texto, s.Entrada, c.entrada)
		}
		if c.ejemplo != "" && s.Ejemplo != c.ejemplo {
			t.Errorf("%q: ejemplo %q, esperaba %q", c.texto, s.Ejemplo, c.ejemplo)
		}
	}
}

func TestFusionar(t *testing.T) {
	lex := LexicoBase()
	ant := Analizar("haz una función que sume los números pares de una lista", "", "", lex, nil)
	nueva := Analizar("ahora con impares", "", "", lex, nil)
	r := Fusionar(ant, nueva)
	if r.Marco == nil || r.Marco.Accion != "sumar" || r.Marco.Entrada != "lista_numeros" {
		t.Fatalf("marco fusionado: %+v", r.Marco)
	}
	if len(r.Marco.Mods) != 1 || r.Marco.Mods[0].Concepto != "impar" {
		t.Errorf("los modificadores deben ser [impar]: %+v", r.Marco.Mods)
	}
	if r.Firma == nil || r.Firma.Forma() != "([]int)int" || r.Firma.Nombre != "SumaImpares" {
		t.Errorf("firma fusionada: %+v", r.Firma)
	}
	if ant.Marco.Mods[0].Concepto != "par" {
		t.Errorf("Fusionar no debe cambiar la pregunta anterior")
	}
	// a different action replaces the old one; modifiers of other families stay
	ant = Analizar("función que sume los números mayores que 5 de una lista", "", "", lex, nil)
	r = Fusionar(ant, Analizar("ahora que cuente los pares", "", "", lex, nil))
	if r.Marco.Accion != "contar" || len(r.Marco.Mods) != 2 {
		t.Errorf("cambiar acción: %+v", r.Marco)
	}
	// examples of the new turn win
	ant = Analizar("[1,2] -> 3", "", "", lex, nil)
	r = Fusionar(ant, Analizar("[5] -> 5; [] -> 0", "", "", lex, nil))
	if len(r.ParesTexto) != 2 {
		t.Errorf("ejemplos nuevos: %v", r.ParesTexto)
	}
	if Fusionar(nil, nueva) != nueva || Fusionar(ant, nil) != ant {
		t.Errorf("Fusionar con nil")
	}
}

func TestConsultaIngles(t *testing.T) {
	lex := LexicoBase()
	q := ConsultaIngles(Analizar("invertir una cadena", "", "", lex, nil), lex)
	if !strings.Contains(q, "reverse") || !strings.Contains(q, "string") || !strings.Contains(q, "golang") {
		t.Errorf("ConsultaIngles(invertir una cadena) = %q", q)
	}
	q = ConsultaIngles(Analizar("busca cómo ordenar un mapa en Go", "", "", lex, nil), lex)
	if !strings.Contains(q, "sort") || !strings.Contains(q, "map") || strings.Contains(q, "search") {
		t.Errorf("busca cómo ordenar un mapa = %q", q)
	}
	q = ConsultaIngles(Analizar("¿cómo uso strings.Split?", "", "", lex, nil), lex)
	if !strings.Contains(q, "strings.Split") {
		t.Errorf("símbolo de Go: %q", q)
	}
	q = ConsultaIngles(Analizar("haz una función que sume los números pares de una lista", "", "", lex, nil), lex)
	if !strings.Contains(q, "func") || !strings.Contains(q, "even") || !strings.Contains(q, "slice") {
		t.Errorf("petición de código: %q", q)
	}
}

func TestLexico(t *testing.T) {
	lex := LexicoBase()
	n := 0
	for _, lema := range lex.Lemas() {
		n++
		if c, ok := lex.Concepto(lema); ok && !nucleo.EsConcepto(c) {
			t.Errorf("el lema %q tiene el concepto %q, que no está en nucleo.Conceptos", lema, c)
		}
	}
	if n < 250 {
		t.Errorf("el léxico tiene %d entradas; deberían ser unas 300", n)
	}
	if c, _ := lex.Concepto("invertir"); c != "invertir" {
		t.Errorf("invertir → %q", c)
	}
	if en, _ := lex.Ingles("cadena"); en != "string" {
		t.Errorf("cadena → %q", en)
	}
	if _, ok := lex.Concepto("trocar"); ok {
		t.Errorf("trocar no debería conocerse")
	}
	l2 := lex.ConAprendidas(map[string]string{"capicúa": "palindromo", "rarísimo": "no_es_concepto"})
	if c, ok := l2.Concepto("capicua"); !ok || c != "palindromo" {
		t.Errorf("capicúa aprendida: %q %v", c, ok)
	}
	if _, ok := l2.Concepto("rarisimo"); ok {
		t.Errorf("un concepto desconocido no se aprende")
	}
	l2.Aprender("trocar", "restar")
	if c, _ := l2.Concepto("trocar"); c != "restar" {
		t.Errorf("Aprender: %q", c)
	}
	if _, ok := lex.Concepto("trocar"); ok {
		t.Errorf("ConAprendidas debe devolver una copia")
	}
	if en, ok := l2.Ingles("trocar"); !ok || en != "subtract" {
		t.Errorf("Ingles de una palabra aprendida: %q", en)
	}
	// "recuerda que capicúa significa palíndromo" then a request with the new word
	p := Analizar("función que diga si una palabra es capicúa", "", "", l2, nil)
	if p.Marco == nil || p.Marco.Accion != "palindromo" {
		t.Errorf("palabra aprendida en un marco: %+v", p.Marco)
	}
}

func TestPalabrasDesconocidas(t *testing.T) {
	lex := LexicoBase()
	got := PalabrasDesconocidas(Lematizar("haz una función que trocee las zarandajas de una lista"), lex)
	if strings.Join(got, ",") != "trocee,zarandaja" && strings.Join(got, ",") != "zarandaja" {
		t.Errorf("desconocidas: %v", got)
	}
	if got := PalabrasDesconocidas(Lematizar("suma los números pares de una lista"), lex); len(got) != 0 {
		t.Errorf("todas son conocidas: %v", got)
	}
}

func TestExtraerCodigo(t *testing.T) {
	c, r := ExtraerCodigo("arregla esto:\n```go\nfunc f() int {\n\treturn 1\n}\n```\ngracias")
	if c != "func f() int {\n\treturn 1\n}" || r != "arregla esto:\n\ngracias" {
		t.Errorf("bloque: %q / %q", c, r)
	}
	c, r = ExtraerCodigo("no compila\nfunc f(xs []int) int {\n\tt := 0\n\treturn t\n}")
	if !strings.HasPrefix(c, "func f") || r != "no compila" {
		t.Errorf("líneas: %q / %q", c, r)
	}
	c, r = ExtraerCodigo("resuelve 2x+3=11")
	if c != "" || r != "resuelve 2x+3=11" {
		t.Errorf("sin código: %q / %q", c, r)
	}
	c, _ = ExtraerCodigo("haz una función que sume los números pares de una lista")
	if c != "" {
		t.Errorf("una frase no es código: %q", c)
	}
	p := Analizar("explica:\nfunc f() {}\n", "func g() {}", "", nil, nil)
	if p.Codigo != "func g() {}" {
		t.Errorf("el código de la caja gana: %q", p.Codigo)
	}
	p = Analizar("no compila\nfunc f(xs []int) int {\n\tt := 0\n\treturn t\n}", "", "", nil, nil)
	for _, tk := range p.Tokens {
		if tk.Texto == "func" || tk.Texto == "return" {
			t.Errorf("el código no debe tokenizarse: %+v", tk)
		}
		if p.Texto[tk.Desde:tk.Hasta] != tk.Texto {
			t.Errorf("desplazamiento mal: %+v", tk)
		}
	}
}

func TestExtraerEjemplos(t *testing.T) {
	casos := []struct {
		texto string
		pares string
		prog  int
	}{
		{"[1,2,3,4] -> 6; [5,7] -> 0; [] -> 0", "[1,2,3,4]=6|[5,7]=0|[]=0", 0},
		{"cuenta las vocales: casa -> 2, árbol -> 2", "casa=2|árbol=2", 0},
		{`de "go es genial" sale "Go Es Genial"`, `"go es genial"="Go Es Genial"`, 0},
		{"f(3) = 9, f(4) = 16", "3=9|4=16", 0},
		{"Suma(2, 3) da 5", "2, 3=5", 0},
		{"para 5 da 25", "5=25", 0},
		{"con [1,2] debe dar 3", "[1,2]=3", 0},
		{"(3, [1,2]) -> 5\n(0, []) -> 0", "(3, [1,2])=5|(0, [])=0", 0},
		{"[3,1,2] => [1,2,3]", "[3,1,2]=[1,2,3]", 0},
		{"si escribo 5 debe salir 25", "", 1},
		{"debería dar el máximo pero con [3,1,2] da 1", "", 0},
		{"resuelve 2x+3=11", "", 0},
	}
	for _, c := range casos {
		pares, prog := ExtraerEjemplos(c.texto)
		var ps []string
		for _, p := range pares {
			ps = append(ps, p[0]+"="+p[1])
		}
		if strings.Join(ps, "|") != c.pares || len(prog) != c.prog {
			t.Errorf("ExtraerEjemplos(%q) = %v, %d programas", c.texto, ps, len(prog))
		}
	}
	_, prog := ExtraerEjemplos("si escribo 2 y 3 debe salir 5")
	if len(prog) != 1 || prog[0].Entrada != "2\n3\n" || prog[0].Esperado != "5" {
		t.Errorf("caso de programa: %+v", prog)
	}
	fs := ExtraerFallos("debería dar el máximo pero con [3,1,2] da 1")
	if len(fs) != 1 || fs[0].Entrada != "[3,1,2]" || fs[0].Obtenido != "1" {
		t.Errorf("ExtraerFallos: %+v", fs)
	}
}

func TestTiparEjemplos(t *testing.T) {
	casos, f, err := TiparEjemplos([][2]string{{"[1,2,3,4]", "6"}, {"[]", "0"}}, nil)
	if err != nil || f.Forma() != "([]int)int" || len(casos) != 2 {
		t.Fatalf("inferir: %v %v", f, err)
	}
	if !nucleo.Igual(casos[0].Esperado[0], 6) || casos[0].Expectativa != nucleo.EspUsuario || casos[0].Origen != nucleo.OrigenUsuario {
		t.Errorf("caso: %+v", casos[0])
	}
	_, f, err = TiparEjemplos([][2]string{{"(3, [1,2])", "5"}, {"0, []", "0"}}, nil)
	if err != nil || f.Forma() != "(int,[]int)int" {
		t.Errorf("tupla: %v %v", f, err)
	}
	_, f, err = TiparEjemplos([][2]string{{"[1,2]", "1.5"}, {"[4]", "4"}}, nil)
	if err != nil || f.Forma() != "([]int)float64" {
		t.Errorf("unificar int y float: %v %v", f, err)
	}
	firma := &nucleo.Firma{Nombre: "Suma", Params: []nucleo.Param{{Nombre: "a", Tipo: nucleo.TInt}, {Nombre: "b", Tipo: nucleo.TInt}}, Res: []nucleo.Tipo{nucleo.TInt}}
	casos, _, err = TiparEjemplos([][2]string{{"2, 3", "5"}, {"(1, 1)", "2"}}, firma)
	if err != nil || len(casos) != 2 || !nucleo.Igual(casos[1].Entradas[1], 1) {
		t.Errorf("con firma: %v %v", casos, err)
	}
	if _, _, err := TiparEjemplos([][2]string{{"[1,2]", "x"}}, firma); err == nil {
		t.Errorf("número de datos distinto debe dar error")
	}
	if _, _, err := TiparEjemplos(nil, nil); err == nil {
		t.Errorf("sin ejemplos debe dar error")
	}
}

func TestIntencionesModo(t *testing.T) {
	p := Analizar("¿cuántos números del 1 al 1000 son divisibles por 3 o por 5?", "", "", nil, nil)
	sin := p.Intenciones[0].Puntos
	p = Analizar("¿cuántos números del 1 al 1000 son divisibles por 3 o por 5?", "", "puzles", nil, nil)
	if p.Intenciones[0].Tipo != nucleo.IConteo || p.Intenciones[0].Puntos <= sin {
		t.Errorf("el modo debe reforzar su familia: %v", p.Intenciones)
	}
	for _, c := range p.Intenciones {
		if c.Puntos <= 0 || c.Puntos > 1 || c.Motivo == "" {
			t.Errorf("candidata mal formada: %+v", c)
		}
	}
	if Intenciones(nil) != nil {
		t.Errorf("Intenciones(nil)")
	}
}

func TestNombreFuncion(t *testing.T) {
	casos := map[string]string{
		"haz una función que sume los números pares de una lista":     "SumaPares",
		"función que cuente las vocales de un texto":                  "CuentaVocales",
		"devuelve la palabra más larga de una frase":                  "PalabraMasLarga",
		"función que diga si un número es primo":                      "EsPrimo",
		"función que devuelva los números mayores que 5 de una lista": "FiltraMayores",
		"función que ordene una lista de números de mayor a menor":    "OrdenaDesc",
		"función que quite los repetidos de una lista":                "SinRepetir",
		"función que devuelva el factorial de un número":              "Factorial",
		"cuántas veces aparece cada palabra en un texto":              "FrecuenciasPalabras",
		"función que sume los números que no sean pares de una lista": "SumaNoPares",
	}
	for texto, want := range casos {
		p := Analizar(texto, "", "", nil, nil)
		if got := NombreFuncion(p.Marco); got != want {
			t.Errorf("NombreFuncion(%q) = %q, esperaba %q", texto, got, want)
		}
	}
	if NombreFuncion(nil) != "Funcion" {
		t.Errorf("NombreFuncion(nil)")
	}
}

func TestAnalizarCompleto(t *testing.T) {
	p := Analizar("haz una función que sume los números pares de una lista: [1,2,3,4] -> 6", "", "", nil, nil)
	if p.Normal == "" || len(p.Tokens) == 0 || p.Marco == nil || p.Firma == nil || len(p.Ejemplos) != 1 {
		t.Fatalf("análisis incompleto: %+v", p)
	}
	if p.Firma.Nombre != "SumaPares" || p.Firma.Go() != "func SumaPares(nums []int) int" {
		t.Errorf("firma: %s", p.Firma.Go())
	}
	if strings.Join(p.Conceptos, ",") != "sumar,par,numero,lista" {
		t.Errorf("conceptos: %v", p.Conceptos)
	}
	for _, c := range p.Conceptos {
		if !nucleo.EsConcepto(c) {
			t.Errorf("concepto desconocido %q", c)
		}
	}
	if p.ConsultaWeb == "" {
		t.Errorf("falta la consulta web")
	}
}

func TestEjemploLlamadaLista(t *testing.T) {
	pares, _ := ExtraerEjemplos("Max([3,1,2]) = 3; Max([5]) = 5")
	if len(pares) != 2 || pares[0][0] != "[3,1,2]" {
		t.Fatalf("pares: %v", pares)
	}
	_, f, err := TiparEjemplos(pares, nil)
	if err != nil || f.Forma() != "([]int)int" {
		t.Errorf("firma: %v %v", f, err)
	}
}

func TestEntradasRaras(t *testing.T) {
	ctx := &nucleo.Contexto{Intencion: nucleo.ICrearFuncion, Codigo: "func f() {}"}
	raras := []string{"", "   ", "¿?", "[1,2", `"sin cerrar`, "«", "->", "-> 3", "[] ->", "((((", "]]]]", "😀 hola 😀",
		"1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20", strings.Repeat("palabra ", 3000), "```go\n", "```",
		"f() = ", "con  da ", "si escribo  sale ", "mil mil mil millones", "un un un", "y si fuera", "y con",
		"está mal: con [ da", "'", "''", "'ab'", "\x00\x01", "x^^^2", "%%%", "3,,5", "1.000.000.000.000.000.000.000"}
	for _, s := range raras {
		p := Analizar(s, "", "", nil, ctx)
		if p == nil {
			t.Fatalf("Analizar(%q) = nil", s)
		}
		_ = Fusionar(p, Analizar(s, "", "", nil, nil))
		DetectarSeguimiento(s, ctx)
		Numeros(s)
		ExtraerEjemplos(s)
		ExtraerFallos(s)
		PalabrasDesconocidas(Lematizar(s), nil)
	}
}
