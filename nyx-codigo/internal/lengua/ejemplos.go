package lengua

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// ---- code ----

var reBloque = regexp.MustCompile("(?s)```[a-zA-Z]*[ \t]*\n?(.*?)```")

// ExtraerCodigo separates Go code from the request text. Fenced blocks (```…```) win; otherwise it
// takes the largest run of lines in which at least half look like Go (func, package, :=, braces,
// for, if, return, fmt.…) and that has a clear Go line. Without code, resto == s.
func ExtraerCodigo(s string) (codigo, resto string) {
	if ms := reBloque.FindAllStringSubmatchIndex(s, -1); len(ms) > 0 {
		var partes []string
		var sb strings.Builder
		ult := 0
		for _, m := range ms {
			sb.WriteString(s[ult:m[0]])
			ult = m[1]
			partes = append(partes, strings.Trim(s[m[2]:m[3]], "\n"))
		}
		sb.WriteString(s[ult:])
		return strings.Join(partes, "\n\n"), strings.TrimSpace(sb.String())
	}
	lineas := strings.Split(s, "\n")
	if len(lineas) > 2000 {
		return "", s
	}
	tipo := make([]int, len(lineas)) // 0 blank, 1 text, 2 Go, 3 clear Go
	for i, l := range lineas {
		tipo[i] = claseLinea(l)
	}
	mejorI, mejorJ, mejorN := -1, -1, 0
	for i := range lineas {
		if tipo[i] < 2 {
			continue
		}
		goN, textoN, fuerte := 0, 0, false
		for j := i; j < len(lineas); j++ {
			switch tipo[j] {
			case 1:
				textoN++
			case 2, 3:
				goN++
				if tipo[j] == 3 {
					fuerte = true
				}
			}
			if tipo[j] >= 2 && goN >= textoN && (fuerte || goN >= 2) && goN > mejorN {
				mejorI, mejorJ, mejorN = i, j, goN
			}
		}
	}
	if mejorI < 0 {
		return "", s
	}
	codigo = strings.Join(lineas[mejorI:mejorJ+1], "\n")
	resto = strings.TrimSpace(strings.Join(append(append([]string{}, lineas[:mejorI]...), lineas[mejorJ+1:]...), "\n"))
	return codigo, resto
}

var (
	reGoFuerte = regexp.MustCompile(`^\s*(func\s|package\s|import\s*[("]|type\s+\w+\s+(struct|interface|\w)|var\s+\w+(\s+\S+)?\s*(=|$)|const\s+\w+)`)
	reGoDebil  = regexp.MustCompile(`(:=|^\s*[{}()\[\]]+[,;]?\s*$|^\s*(for|if|switch|select|case|default|return|defer|go|break|continue|else|\} else)\b|\bfmt\.|\b(strings|strconv|math|sort|os|errors)\.\w+\(|^\s*//|[+\-*/]=|\+\+\s*$|--\s*$|\{\s*$|^\s*\w+(\[[^\]]*\])?\s*=\s*[^=])`)
)

func claseLinea(l string) int {
	t := strings.TrimSpace(l)
	if t == "" {
		return 0
	}
	if reGoFuerte.MatchString(l) {
		return 3
	}
	if reGoDebil.MatchString(l) {
		// a Spanish sentence with "=" (an equation) is not Go
		if strings.Count(t, " ") >= 4 && !strings.ContainsAny(t, "{}();") && !strings.Contains(t, ":=") {
			return 1
		}
		return 2
	}
	return 1
}

// ---- examples ----

// protegido is a text where list, tuple and quoted literals are replaced by placeholders.
type protegido struct {
	texto     string
	literales []string
}

const marcaLit = '\x00'

func proteger(s string) protegido {
	var p protegido
	var sb strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		fin := -1
		switch {
		case c == '[':
			fin = cierreCorchete(s, i)
		case c == '(':
			fin = cierreParentesis(s, i)
		case c == '"':
			if k := strings.IndexByte(s[i+1:], '"'); k >= 0 {
				fin = i + 1 + k
			}
		case strings.HasPrefix(s[i:], "«"):
			if k := strings.Index(s[i:], "»"); k >= 0 {
				fin = i + k + len("»") - 1
			}
		case strings.HasPrefix(s[i:], "“"):
			if k := strings.Index(s[i:], "”"); k >= 0 {
				fin = i + k + len("”") - 1
			}
		case c == '\'':
			if i+2 < len(s) {
				if k := strings.IndexByte(s[i+1:], '\''); k > 0 && k <= 4 {
					fin = i + 1 + k
				}
			}
		}
		if fin > i {
			lit := s[i : fin+1]
			// a parenthesis without a comma is plain grouping, not a tuple
			if c == '(' && !strings.Contains(lit, ",") {
				sb.WriteByte(c)
				i++
				continue
			}
			sb.WriteRune(marcaLit)
			sb.WriteString(strconv.Itoa(len(p.literales)))
			sb.WriteRune(marcaLit)
			p.literales = append(p.literales, lit)
			i = fin + 1
			continue
		}
		sb.WriteByte(c)
		i++
	}
	p.texto = sb.String()
	return p
}

var reMarca = regexp.MustCompile("\x00([0-9]+)\x00")

func (p protegido) restaurar(s string) string {
	return reMarca.ReplaceAllStringFunc(s, func(m string) string {
		k, _ := strconv.Atoi(m[1 : len(m)-1])
		if k < len(p.literales) {
			return p.literales[k]
		}
		return m
	})
}

func cierreParentesis(s string, i int) int {
	prof := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '(':
			prof++
		case ')':
			prof--
			if prof == 0 {
				return j
			}
		case '\n':
			return -1
		}
	}
	return -1
}

var (
	reFlecha      = regexp.MustCompile(`^(.*?)\s*(?:->|→|=>|⇒)\s*(.+)$`)
	reLlamada     = regexp.MustCompile(`^(?:.*[\s:])?[A-Za-z_]\w*\s*(\x00[0-9]+\x00|\([^()]*\))\s*(?:==?\s*|\s(?:da|devuelve|debe dar|deberia dar|debería dar|es)\s+)(.+)$`)
	reDeSale      = regexp.MustCompile(`(?i)(?:^|\s)de\s+(\S+)\s+(?:sale|salga|debe salir|deberia salir|debería salir)\s+(.+)$`)
	reParaDa      = regexp.MustCompile(`(?i)(?:^|\s)para\s+(\S+)\s+(?:da|devuelve|sale|debe dar|deberia dar|debería dar|es)\s+(.+)$`)
	reConDa       = regexp.MustCompile(`(?i)(?:^|\s)con\s+(\S+)\s+(debe dar|deberia dar|debería dar|tiene que dar|debe devolver|deberia devolver|debería devolver|da|devuelve|sale)\s+(.+)$`)
	rePrograma    = regexp.MustCompile(`(?i)(?:^|\s)si\s+(?:escribo|entra|meto|pongo|introduzco|le doy|tecleo|doy)\s+(.+?)\s+(?:sale|debe salir|deberia salir|debería salir|escribe|debe escribir|muestra|debe mostrar|imprime|debe imprimir|da)\s+(.+)$`)
	reConPero     = regexp.MustCompile(`(?i)\bpero\s+con\s+$`)
	reFinFrase    = regexp.MustCompile(`[\s.;:!?¡¿]+$`)
	reNumerosYEs  = regexp.MustCompile(`(?i)\s+y\s+`)
)

// ExtraerEjemplos finds input → output examples in s:
//
//	X -> Y, X → Y, X => Y; f(X) = Y, F(X) da Y; de X sale Y; para X da Y; con X (debe dar|debería dar|da) Y
//
// and program cases "si (escribo|entra|meto) X (sale|debe salir|escribe) Y". Pairs are separated by
// ";", newlines or "," outside brackets and quotes; "con X da Y" after "pero" is a bug report, not an
// example (see ExtraerFallos). Literals are returned as written ("[1,2,3]", "\"hola\"", "casa").
func ExtraerEjemplos(s string) (pares [][2]string, programa []nucleo.CasoPrograma) {
	p := proteger(s)
	for _, seg := range segmentos(p.texto) {
		if m := rePrograma.FindStringSubmatch(seg); m != nil {
			ent := p.restaurar(strings.TrimSpace(m[1]))
			sal := limpiarLado(p.restaurar(m[2]))
			programa = append(programa, nucleo.CasoPrograma{
				Entrada: entradaPrograma(ent), Esperado: quitarComillas(sal), Comparar: "contiene",
				Expectativa: nucleo.EspUsuario, Nota: "tu ejemplo " + strconv.Itoa(len(programa)+1),
			})
			continue
		}
		x, y, ok := parEjemplo(seg)
		if !ok {
			continue
		}
		x, y = limpiarLado(p.restaurar(x)), limpiarLado(p.restaurar(y))
		if x == "" || y == "" {
			continue
		}
		pares = append(pares, [2]string{x, y})
	}
	return pares, programa
}

// segmentos splits on newlines and ";", then on commas outside literals; a piece without an example
// is glued to the next one ("3, [1,2] -> 5" stays whole).
func segmentos(t string) []string {
	var out []string
	for _, linea := range strings.FieldsFunc(t, func(r rune) bool { return r == '\n' || r == ';' }) {
		piezas := strings.Split(linea, ",")
		acum := ""
		for i, pz := range piezas {
			if acum != "" {
				acum += ","
			}
			acum += pz
			if tieneEjemplo(acum) || i == len(piezas)-1 {
				out = append(out, strings.TrimSpace(acum))
				acum = ""
			}
		}
	}
	return out
}

func tieneEjemplo(s string) bool {
	if reFlecha.MatchString(s) {
		return true
	}
	_, _, ok := parEjemplo(s)
	return ok
}

func parEjemplo(seg string) (x, y string, ok bool) {
	if m := reFlecha.FindStringSubmatch(seg); m != nil {
		x = m[1]
		if k := strings.LastIndex(x, ":"); k >= 0 {
			x = x[k+1:]
		}
		x = ultimoValor(x)
		return x, m[2], strings.TrimSpace(x) != ""
	}
	if m := reLlamada.FindStringSubmatch(seg); m != nil {
		x := m[1]
		if strings.HasPrefix(x, "(") {
			x = x[1 : len(x)-1]
		}
		return x, m[2], true
	}
	if m := reDeSale.FindStringSubmatch(seg); m != nil {
		return m[1], m[2], true
	}
	if m := reParaDa.FindStringSubmatch(seg); m != nil {
		return m[1], m[2], true
	}
	if loc := reConDa.FindStringSubmatchIndex(seg); loc != nil {
		verbo := strings.ToLower(seg[loc[4]:loc[5]])
		antes := seg[:loc[2]]
		if (verbo == "da" || verbo == "devuelve" || verbo == "sale") && (reConPero.MatchString(antes) || strings.Contains(strings.ToLower(antes), "pero")) {
			return "", "", false
		}
		return seg[loc[2]:loc[3]], seg[loc[6]:loc[7]], true
	}
	return "", "", false
}

// ultimoValor keeps the trailing value of the left side of an arrow: "cuenta las vocales casa" → "casa";
// words before a protected literal are dropped; a tuple "3, [1,2]" stays whole.
func ultimoValor(x string) string {
	x = strings.TrimSpace(x)
	if k := strings.IndexRune(x, marcaLit); k >= 0 {
		if pre := strings.TrimSpace(x[:k]); k > 0 && soloPalabras(pre) {
			return strings.TrimSpace(x[k:])
		}
		return x
	}
	if strings.Contains(x, ",") {
		return x
	}
	campos := strings.Fields(x)
	if len(campos) <= 1 {
		return x
	}
	return campos[len(campos)-1]
}

func soloPalabras(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) || r == ' ' || r == ':') {
			return false
		}
	}
	return true
}

func limpiarLado(s string) string {
	s = strings.TrimSpace(s)
	s = reFinFrase.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func quitarComillas(s string) string {
	for _, q := range [][2]string{{`"`, `"`}, {"«", "»"}, {"“", "”"}, {"'", "'"}} {
		if len(s) >= len(q[0])+len(q[1]) && strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) {
			return s[len(q[0]) : len(s)-len(q[1])]
		}
	}
	return s
}

// entradaPrograma turns "2 y 3", "«2⏎3»" or "5" into stdin lines.
func entradaPrograma(x string) string {
	x = quitarComillas(strings.TrimSpace(x))
	x = strings.ReplaceAll(x, "⏎", "\n")
	if !strings.Contains(x, "\n") {
		x = reNumerosYEs.ReplaceAllString(x, "\n")
	}
	if !strings.HasSuffix(x, "\n") {
		x += "\n"
	}
	return x
}

// Fallo is a bug report found in the text: "con [3,1,2] da 1" (Obtenido) or "con [3,1,2] debería dar 3" (Esperado).
type Fallo struct {
	Entrada  string
	Obtenido string // what the code gives now ("" if not said)
	Esperado string // what it should give ("" if not said)
}

var reFalloDa = regexp.MustCompile(`(?i)con\s+(\S+)\s+(?:me\s+)?(da|devuelve|sale|sale|imprime|debe dar|deberia dar|debería dar|tiene que dar)\s+([^\s,;.]+(?:\s*[^\s,;.]+)?)`)

// ExtraerFallos reads "debería dar el máximo pero con [3,1,2] da 1" style reports (additive helper for
// cerebro's repair spec): each "con X da Y" gives Obtenido, each "con X debe/debería dar Y" gives Esperado.
func ExtraerFallos(s string) []Fallo {
	p := proteger(s)
	var out []Fallo
	for _, m := range reFalloDa.FindAllStringSubmatch(p.texto, -1) {
		f := Fallo{Entrada: limpiarLado(p.restaurar(m[1]))}
		val := limpiarLado(p.restaurar(strings.Fields(m[3])[0]))
		if strings.HasPrefix(strings.ToLower(m[2]), "deb") || strings.HasPrefix(strings.ToLower(m[2]), "tiene") {
			f.Esperado = val
		} else {
			f.Obtenido = val
		}
		out = append(out, f)
	}
	return out
}

// ---- typing ----

// TiparEjemplos types the example pairs. With f, each side is read with f's types (a multi-argument
// input is "(3, [1,2])" or "3, [1,2]"). Without f, the types are inferred with nucleo.Inferir and
// unified across pairs (an empty list takes the element type of the others, []int if none);
// a parenthesized tuple gives several parameters. Cases are EspUsuario / OrigenUsuario.
func TiparEjemplos(pares [][2]string, f *nucleo.Firma) ([]nucleo.Caso, *nucleo.Firma, error) {
	if len(pares) == 0 {
		return nil, f, fmt.Errorf("no hay ejemplos")
	}
	if f != nil {
		ent := f.Entradas()
		casos := make([]nucleo.Caso, 0, len(pares))
		for i, pr := range pares {
			args := partirArgs(pr[0], len(ent))
			if len(args) != len(ent) {
				return nil, nil, fmt.Errorf("el ejemplo %d tiene %d datos y la función recibe %d", i+1, len(args), len(ent))
			}
			c := nucleo.Caso{Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario, Nota: "tu ejemplo " + strconv.Itoa(i+1)}
			for k, a := range args {
				v, err := nucleo.ParseValor(a, ent[k])
				if err != nil {
					return nil, nil, fmt.Errorf("ejemplo %d: %w", i+1, err)
				}
				c.Entradas = append(c.Entradas, v)
			}
			res := partirArgs(pr[1], len(f.Res))
			if len(res) != len(f.Res) {
				return nil, nil, fmt.Errorf("el ejemplo %d da %d resultados y la función devuelve %d", i+1, len(res), len(f.Res))
			}
			for k, a := range res {
				v, err := nucleo.ParseValor(a, f.Res[k])
				if err != nil {
					return nil, nil, fmt.Errorf("ejemplo %d: %w", i+1, err)
				}
				c.Esperado = append(c.Esperado, v)
			}
			casos = append(casos, c)
		}
		cp := *f
		return casos, &cp, nil
	}
	// infer
	n := -1
	var filas [][]string
	for _, pr := range pares {
		args := partirTupla(pr[0])
		if n >= 0 && len(args) != n {
			return nil, nil, fmt.Errorf("los ejemplos no tienen el mismo número de datos de entrada")
		}
		n = len(args)
		filas = append(filas, append(args, pr[1]))
	}
	cols := n + 1
	tipos := make([]nucleo.Tipo, cols)
	vals := make([][]nucleo.Valor, len(filas))
	for k := 0; k < cols; k++ {
		t, vs, err := tiparColumna(filas, k)
		if err != nil {
			return nil, nil, err
		}
		tipos[k] = t
		for i := range filas {
			vals[i] = append(vals[i], vs[i])
		}
	}
	firma := &nucleo.Firma{Nombre: "Funcion", Res: []nucleo.Tipo{tipos[n]}}
	firma.Params = paramsPorTipo(tipos[:n])
	casos := make([]nucleo.Caso, len(filas))
	for i := range filas {
		casos[i] = nucleo.Caso{Entradas: vals[i][:n], Esperado: vals[i][n:], Expectativa: nucleo.EspUsuario,
			Origen: nucleo.OrigenUsuario, Nota: "tu ejemplo " + strconv.Itoa(i+1)}
	}
	return casos, firma, nil
}

// tiparColumna infers and unifies column k; on a conflict it reads the whole column as text.
func tiparColumna(filas [][]string, k int) (nucleo.Tipo, []nucleo.Valor, error) {
	var t nucleo.Tipo
	vs := make([]nucleo.Valor, len(filas))
	ok := true
	for i, f := range filas {
		v, ti, err := nucleo.Inferir(f[k])
		if err != nil {
			ok = false
			break
		}
		u, bien := nucleo.Unificar(t, ti)
		if !bien {
			ok = false
			break
		}
		t, vs[i] = u, v
	}
	if ok {
		t = completarTipo(t)
		for i, f := range filas {
			v, err := nucleo.ParseValor(f[k], t)
			if err != nil {
				ok = false
				break
			}
			vs[i] = v
		}
	}
	if ok {
		return t, vs, nil
	}
	// mixed literals (a word that reads as a bool, "si"/"no"…): everything as text
	for i, f := range filas {
		vs[i] = quitarComillas(strings.TrimSpace(f[k]))
	}
	return nucleo.TString, vs, nil
}

// completarTipo fills unknown element types with int ([] → []int).
func completarTipo(t nucleo.Tipo) nucleo.Tipo {
	switch t.Clase {
	case nucleo.CInvalida:
		return nucleo.TInt
	case nucleo.CLista, nucleo.CArreglo, nucleo.CPuntero:
		e := nucleo.TInt
		if t.Elem != nil {
			e = completarTipo(*t.Elem)
		}
		t.Elem = &e
	case nucleo.CMapa:
		k, e := nucleo.TString, nucleo.TInt
		if t.Clave != nil && t.Clave.Clase != nucleo.CInvalida {
			k = *t.Clave
		}
		if t.Elem != nil {
			e = completarTipo(*t.Elem)
		}
		t.Clave, t.Elem = &k, &e
	}
	return t
}

// partirTupla splits "(3, [1,2])" into its parts; "3, [1,2]" (not all numbers) as well; anything else is one value.
func partirTupla(x string) []string {
	x = strings.TrimSpace(x)
	if strings.HasPrefix(x, "(") && strings.HasSuffix(x, ")") && cierreParentesis(x, 0) == len(x)-1 {
		if ps := partirNivel(x[1 : len(x)-1]); len(ps) > 1 {
			return ps
		}
	}
	ps := partirNivel(x)
	if len(ps) > 1 {
		todosNum := true
		for _, p := range ps {
			if _, t, err := nucleo.Inferir(p); err != nil || (t.Clase != nucleo.CInt && t.Clase != nucleo.CFloat) {
				todosNum = false
			}
		}
		if !todosNum {
			return ps
		}
	}
	return []string{x}
}

// partirArgs splits x into n arguments when n > 1.
func partirArgs(x string, n int) []string {
	x = strings.TrimSpace(x)
	if n <= 1 {
		if strings.HasPrefix(x, "(") && strings.HasSuffix(x, ")") && cierreParentesis(x, 0) == len(x)-1 && n == 1 {
			if ps := partirNivel(x[1 : len(x)-1]); len(ps) == 1 {
				return ps
			}
		}
		return []string{x}
	}
	if strings.HasPrefix(x, "(") && strings.HasSuffix(x, ")") && cierreParentesis(x, 0) == len(x)-1 {
		x = x[1 : len(x)-1]
	}
	ps := partirNivel(x)
	if len(ps) != n {
		// "3 y 4"
		if alt := reNumerosYEs.Split(x, -1); len(alt) == n {
			return alt
		}
	}
	return ps
}

// partirNivel splits on commas at depth 0 (outside brackets, parentheses, braces and quotes).
func partirNivel(s string) []string {
	var out []string
	prof, ini := 0, 0
	var cita byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if cita != 0 {
			if c == '\\' {
				i++
			} else if c == cita {
				cita = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			cita = c
		case '[', '(', '{':
			prof++
		case ']', ')', '}':
			prof--
		case ',':
			if prof == 0 {
				out = append(out, strings.TrimSpace(s[ini:i]))
				ini = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[ini:]))
	return out
}

// paramsPorTipo names parameters idiomatically: nums, palabras, s, n, c, matriz, m; repeats become a, b….
func paramsPorTipo(ts []nucleo.Tipo) []nucleo.Param {
	out := make([]nucleo.Param, len(ts))
	cuenta := map[string]int{}
	nombres := make([]string, len(ts))
	for i, t := range ts {
		nombres[i] = nombreParam(t)
		cuenta[nombres[i]]++
	}
	letras := map[string][]string{
		"n": {"a", "b", "c", "d", "e", "f"}, "s": {"s", "t", "u", "v", "w", "z"}, "x": {"x", "y", "z", "u", "v", "w"},
	}
	usados := map[string]int{}
	for i, t := range ts {
		nom := nombres[i]
		if cuenta[nom] > 1 {
			k := usados[nom]
			usados[nom]++
			if ls, ok := letras[nom]; ok && k < len(ls) {
				nom = ls[k]
			} else {
				nom += strconv.Itoa(k + 1)
			}
		}
		out[i] = nucleo.Param{Nombre: nom, Tipo: t}
	}
	return out
}

func nombreParam(t nucleo.Tipo) string {
	switch t.Clase {
	case nucleo.CInt:
		return "n"
	case nucleo.CFloat:
		return "x"
	case nucleo.CString:
		return "s"
	case nucleo.CRune, nucleo.CByte:
		return "c"
	case nucleo.CBool:
		return "b"
	case nucleo.CMapa:
		return "m"
	case nucleo.CLista, nucleo.CArreglo:
		if t.Elem != nil {
			switch t.Elem.Clase {
			case nucleo.CInt, nucleo.CFloat:
				return "nums"
			case nucleo.CString:
				return "palabras"
			case nucleo.CLista:
				return "matriz"
			case nucleo.CRune, nucleo.CByte:
				return "letras"
			}
		}
		return "xs"
	}
	return "v"
}
