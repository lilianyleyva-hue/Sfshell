package arenero

import (
	"errors"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// tamPila is the goroutine stack limit of the harness (debug.SetMaxStack): a runaway recursion
// without depth instrumentation dies quickly with "fatal error: stack overflow".
const tamPila = 128 << 20

// duenio says which part of the generated code a line belongs to, so build errors can be blamed.
type duenio struct {
	v int // variant index, -1 = shared code
	p int // property index, -1 = not a property
}

var comun = duenio{v: -1, p: -1}

// escritor writes generated code and remembers the owner of every line.
type escritor struct {
	sb     strings.Builder
	duenos []duenio // duenos[i] = owner of line i+1
	actual duenio
}

func (w *escritor) escribir(s string) {
	w.sb.WriteString(s)
	for i := 0; i < strings.Count(s, "\n"); i++ {
		w.duenos = append(w.duenos, w.actual)
	}
}

func (w *escritor) f(format string, a ...any) { w.escribir(fmt.Sprintf(format, a...)) }

// duenoLinea returns the owner of a 1-based line.
func (w *escritor) duenoLinea(l int) duenio {
	if l >= 1 && l <= len(w.duenos) {
		return w.duenos[l-1]
	}
	return comun
}

// opcionesArnes is what the generator needs beyond GenerarArnes's arguments.
type opcionesArnes struct {
	firma   nucleo.Firma
	props   []nucleo.Propiedad
	sufijos []string            // per variant; "" = the variant is not in this build
	nombres []map[string]string // per variant: old→new top-level names (to rename inside property expressions)
	sinProp map[[2]int]bool     // (variant, property) pairs dropped after failing to type-check
}

// arnesGenerado holds the trusted files and the line owners of the generated ones.
type arnesGenerado struct {
	archivos map[string]string
	codec    *escritor
	arnes    *escritor
	notas    []string // properties dropped before building (they do not parse)
}

const (
	archivoCodec = "solucion/zz_nyx_codec.go"
	archivoArnes = "solucion/zz_nyx_arnes.go"
)

// GenerarArnes returns the trusted files of a temp module (§4.2.1) for nVariantes variants of f, renamed with
// sufijos[k] (an empty suffix leaves variant k out of the table): "main.go", "nyxfd/nyxfd.go",
// "solucion/zz_nyx_codec.go" and "solucion/zz_nyx_arnes.go". The variant files and the support file
// (instrumenta.Soporte) are written by the caller. instr only matters through the support file; it is accepted
// for symmetry with Preparar.
func GenerarArnes(f nucleo.Firma, nVariantes int, props []nucleo.Propiedad, instr nucleo.OpcionesInstr, sufijos []string) (map[string]string, error) {
	if len(sufijos) != nVariantes {
		return nil, fmt.Errorf("arenero: hay %d sufijos para %d variantes", len(sufijos), nVariantes)
	}
	g, err := generarArnes(opcionesArnes{firma: f, props: props, sufijos: sufijos})
	if err != nil {
		return nil, err
	}
	return g.archivos, nil
}

func generarArnes(op opcionesArnes) (*arnesGenerado, error) {
	f := op.firma
	if !f.Probable() {
		return nil, nucleo.ErrNoProbable
	}
	if f.Nombre == "" {
		return nil, errors.New("arenero: la firma no tiene nombre")
	}
	g := &generador{op: op, codecs: map[string]int{}, importsCodec: map[string]bool{}, importsArnes: map[string]bool{}}
	out := &arnesGenerado{codec: &escritor{}, arnes: &escritor{}}

	// properties that do not even parse are dropped here, once for all variants
	var props []nucleo.Propiedad
	var indices []int
	for j, p := range op.props {
		if err := expresionValida(p.Expr); err != nil {
			out.notas = append(out.notas, fmt.Sprintf("quité la propiedad «%s»: %v", p.Nombre, err))
			continue
		}
		if p.Requiere != "" {
			if err := expresionValida(p.Requiere); err != nil {
				out.notas = append(out.notas, fmt.Sprintf("quité la propiedad «%s»: su condición no se entiende: %v", p.Nombre, err))
				continue
			}
		}
		props = append(props, p)
		indices = append(indices, j)
	}

	// runners (they ask for codecs, which are written to the codec file)
	cuerpo := &escritor{}
	cuerpo.actual = comun
	cuerpo.escribir("var nyx__tabla = []func(*nyx__Ejec){\n")
	for k, suf := range op.sufijos {
		if suf == "" {
			cuerpo.f("\t%d: nil,\n", k)
		} else {
			cuerpo.f("\t%d: nyx__correr_v%d,\n", k, k)
		}
	}
	cuerpo.escribir("}\n")
	for k, suf := range op.sufijos {
		if suf == "" {
			continue
		}
		g.runner(cuerpo, k, suf, props, indices)
	}

	// codec file
	cw := out.codec
	cw.actual = comun
	cw.escribir("// Código generado por Nyx Código (codificadores del arnés). No lo edites.\n\npackage solucion\n\nimport (\n\t\"sort\"\n\t\"strconv\"\n")
	for _, imp := range ordenadas(g.importsCodec) {
		cw.f("\t%q\n", imp)
	}
	cw.escribir(")\n\nvar (\n\t_ = sort.Slice\n\t_ = strconv.AppendInt\n)\n")
	for _, c := range g.trozos {
		cw.actual = c.dueno
		cw.escribir(c.texto)
	}

	// harness file: static part, then the generated body (line owners shift by the static part)
	aw := out.arnes
	aw.actual = comun
	var extra strings.Builder
	for _, imp := range ordenadas(g.importsArnes) {
		if !importaEstatico[imp] {
			fmt.Fprintf(&extra, "\t%q\n", imp)
		}
	}
	estatico := strings.NewReplacer("IMPORTACIONES_EXTRA", extra.String(), "TAMPILA", strconv.Itoa(tamPila)).Replace(plantillaArnes)
	aw.escribir(estatico)
	aw.sb.WriteString(cuerpo.sb.String())
	aw.duenos = append(aw.duenos, cuerpo.duenos...)

	out.archivos = map[string]string{
		"main.go":        fuenteMain,
		"nyxfd/nyxfd.go": fuenteNyxfd,
		archivoCodec:     cw.sb.String(),
		archivoArnes:     aw.sb.String(),
	}
	return out, nil
}

var importaEstatico = map[string]bool{
	"encoding/base64": true, "encoding/json": true, "errors": true, "fmt": true, "io": true, "math": true,
	"os": true, "reflect": true, "runtime/debug": true, "sort": true, "strconv": true, "strings": true,
	"sync": true, "sync/atomic": true, "time": true, "unicode/utf8": true,
}

func ordenadas(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// expresionValida checks that s is one Go expression (so it cannot inject statements into the harness).
func expresionValida(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("la expresión está vacía")
	}
	if _, err := parser.ParseExpr(s); err != nil {
		return fmt.Errorf("no es una expresión de Go: %v", err)
	}
	return nil
}

// renombrarExpr applies a variant's renaming to the identifiers of a property expression,
// leaving the free names (e0…, r0…, F) and the helpers alone.
func renombrarExpr(expr string, nombres map[string]string) string {
	if len(nombres) == 0 {
		return expr
	}
	fset := token.NewFileSet()
	tf := fset.AddFile("p.go", -1, len(expr))
	var s scanner.Scanner
	s.Init(tf, []byte(expr), func(token.Position, string) {}, 0)
	var sb strings.Builder
	ult := 0
	anteriorPunto := false
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT && !anteriorPunto && !nombreLibre(lit) {
			if nuevo, ok := nombres[lit]; ok {
				off := tf.Offset(pos)
				sb.WriteString(expr[ult:off])
				sb.WriteString(nuevo)
				ult = off + len(lit)
			}
		}
		anteriorPunto = tok == token.PERIOD
	}
	sb.WriteString(expr[ult:])
	return sb.String()
}

func nombreLibre(n string) bool {
	if n == "F" || strings.HasPrefix(strings.ToLower(n), "nyx__") {
		return true
	}
	if len(n) >= 2 && (n[0] == 'e' || n[0] == 'r') {
		if _, err := strconv.Atoi(n[1:]); err == nil {
			return true
		}
	}
	return false
}

// ---- the generator ----

type trozo struct {
	dueno duenio
	texto string
}

type generador struct {
	op           opcionesArnes
	codecs       map[string]int
	trozos       []trozo
	importsCodec map[string]bool
	importsArnes map[string]bool
}

// tieneDefinido reports whether t mentions a type declared by the candidate (renamed per variant).
func tieneDefinido(t nucleo.Tipo) bool {
	if t.Definido && t.Nombre != "" && !strings.Contains(t.Nombre, ".") {
		return true
	}
	if t.Elem != nil && tieneDefinido(*t.Elem) {
		return true
	}
	if t.Clave != nil && tieneDefinido(*t.Clave) {
		return true
	}
	for _, c := range t.Campos {
		if tieneDefinido(c.Tipo) {
			return true
		}
	}
	return false
}

// paquetesDe collects the packages of qualified type names ("time.Duration" → "time").
func paquetesDe(t nucleo.Tipo, m map[string]bool) {
	if t.Definido && strings.Contains(t.Nombre, ".") {
		m[t.Nombre[:strings.LastIndexByte(t.Nombre, '.')]] = true
	}
	if t.Elem != nil {
		paquetesDe(*t.Elem, m)
	}
	if t.Clave != nil {
		paquetesDe(*t.Clave, m)
	}
	for _, c := range t.Campos {
		paquetesDe(c.Tipo, m)
	}
}

// goTipo spells t in variant suf (candidate-declared names get the suffix).
func goTipo(t nucleo.Tipo, suf string) string {
	if t.Definido && t.Nombre != "" {
		if strings.Contains(t.Nombre, ".") {
			return t.Nombre
		}
		return t.Nombre + suf
	}
	switch t.Clase {
	case nucleo.CLista:
		return "[]" + goTipo(elem(t.Elem), suf)
	case nucleo.CArreglo:
		return "[" + strconv.Itoa(t.Largo) + "]" + goTipo(elem(t.Elem), suf)
	case nucleo.CMapa:
		return "map[" + goTipo(elem(t.Clave), suf) + "]" + goTipo(elem(t.Elem), suf)
	case nucleo.CPuntero:
		return "*" + goTipo(elem(t.Elem), suf)
	case nucleo.CStruct:
		if len(t.Campos) == 0 {
			return "struct{}"
		}
		partes := make([]string, len(t.Campos))
		for i, c := range t.Campos {
			partes[i] = nombreCampo(c, suf) + " " + goTipo(c.Tipo, suf)
		}
		return "struct{ " + strings.Join(partes, "; ") + " }"
	}
	return t.Go()
}

// nombreCampo: an embedded field is named after its (renamed) type.
func nombreCampo(c nucleo.Campo, suf string) string {
	t := c.Tipo
	if t.Clase == nucleo.CPuntero && t.Elem != nil {
		t = *t.Elem
	}
	if t.Definido && t.Nombre == c.Nombre && !strings.Contains(t.Nombre, ".") {
		return c.Nombre + suf
	}
	return c.Nombre
}

func elem(p *nucleo.Tipo) nucleo.Tipo {
	if p == nil {
		return nucleo.Tipo{}
	}
	return *p
}

// codec returns the index N of nyx__dec_N / nyx__enc_N for t in variant k (generating it if needed).
func (g *generador) codec(t nucleo.Tipo, k int, suf string) int {
	propio := tieneDefinido(t)
	clave := "|" + t.ClaveTipo()
	dueno := comun
	if propio {
		clave = suf + clave
		dueno = duenio{v: k, p: -1}
	}
	if n, ok := g.codecs[clave]; ok {
		return n
	}
	// children first, so each function is written in one piece
	var hijos []int
	switch t.Clase {
	case nucleo.CLista, nucleo.CArreglo, nucleo.CPuntero:
		hijos = append(hijos, g.codec(elem(t.Elem), k, suf))
	case nucleo.CMapa:
		hijos = append(hijos, g.codec(elem(t.Clave), k, suf), g.codec(elem(t.Elem), k, suf))
	case nucleo.CStruct:
		for _, c := range t.Campos {
			hijos = append(hijos, g.codec(c.Tipo, k, suf))
		}
	}
	n := len(g.codecs)
	g.codecs[clave] = n
	paquetes := map[string]bool{}
	paquetesDe(t, paquetes)
	for p := range paquetes {
		g.importsCodec[p] = true
	}
	g.trozos = append(g.trozos, trozo{dueno: dueno, texto: textoCodec(t, n, suf, hijos)})
	return n
}

func textoCodec(t nucleo.Tipo, n int, suf string, hijos []int) string {
	T := goTipo(t, suf)
	var sb strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&sb, format, a...) }
	p("\n// %s\n", T)
	p("func nyx__dec_%d(x any) (nyx__z nyx__T%d, err error) {\n", n, n)
	switch t.Clase {
	case nucleo.CInt, nucleo.CRune, nucleo.CByte:
		if sinSigno(t) {
			p("\tv, err := nyx__decNatural(x)\n")
		} else {
			p("\tv, err := nyx__decEntero(x)\n")
		}
		p("\treturn %s(v), err\n", T)
	case nucleo.CFloat:
		p("\tv, err := nyx__decDecimal(x)\n\treturn %s(v), err\n", T)
	case nucleo.CBool:
		p("\tv, err := nyx__decLogico(x)\n\treturn %s(v), err\n", T)
	case nucleo.CString:
		p("\tv, err := nyx__decTexto(x)\n\treturn %s(v), err\n", T)
	case nucleo.CLista:
		p("\txs, esNil, err := nyx__decLista(x)\n\tif err != nil || esNil {\n\t\treturn nil, err\n\t}\n")
		p("\tout := make(%s, len(xs))\n\tfor i, y := range xs {\n\t\tif out[i], err = nyx__dec_%d(y); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t}\n\treturn out, nil\n", T, hijos[0])
	case nucleo.CArreglo:
		p("\tvar out %s\n\txs, _, err := nyx__decLista(x)\n\tif err == nil {\n\t\terr = nyx__largo(xs, %d)\n\t}\n\tif err != nil {\n\t\treturn out, err\n\t}\n", T, t.Largo)
		p("\tfor i, y := range xs {\n\t\tif out[i], err = nyx__dec_%d(y); err != nil {\n\t\t\treturn out, err\n\t\t}\n\t}\n\treturn out, nil\n", hijos[0])
	case nucleo.CMapa:
		p("\txs, esNil, err := nyx__decLista(x)\n\tif err != nil || esNil {\n\t\treturn nil, err\n\t}\n")
		p("\tout := make(%s, len(xs))\n\tfor _, y := range xs {\n\t\tk, v, err := nyx__decPar(y)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n", T)
		p("\t\tck, err := nyx__dec_%d(k)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tcv, err := nyx__dec_%d(v)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tout[ck] = cv\n\t}\n\treturn out, nil\n", hijos[0], hijos[1])
	case nucleo.CStruct:
		p("\txs, _, err := nyx__decLista(x)\n\tif err == nil {\n\t\terr = nyx__largo(xs, %d)\n\t}\n\tif err != nil {\n\t\treturn nyx__z, err\n\t}\n", len(t.Campos))
		for i := range t.Campos {
			p("\tc%d, err := nyx__dec_%d(xs[%d])\n\tif err != nil {\n\t\treturn nyx__z, err\n\t}\n", i, hijos[i], i)
		}
		p("\treturn %s{", T)
		for i, c := range t.Campos {
			if i > 0 {
				p(", ")
			}
			p("%s: c%d", nombreCampo(c, suf), i)
		}
		p("}, nil\n")
	case nucleo.CPuntero:
		p("\tif x == nil {\n\t\treturn nil, nil\n\t}\n\tv, err := nyx__dec_%d(x)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn &v, nil\n", hijos[0])
	default:
		p("\treturn nyx__z, nyx__malTipo(x, %q)\n", "un tipo que sé leer")
	}
	p("}\n\n")
	p("func nyx__enc_%d(b []byte, v nyx__T%d) []byte {\n", n, n)
	switch t.Clase {
	case nucleo.CInt, nucleo.CRune, nucleo.CByte:
		if sinSigno(t) {
			p("\treturn strconv.AppendUint(b, uint64(v), 10)\n")
		} else {
			p("\treturn strconv.AppendInt(b, int64(v), 10)\n")
		}
	case nucleo.CFloat:
		bits := 64
		if t.Nombre == "float32" && !t.Definido {
			bits = 32
		}
		p("\treturn nyx__encDecimal(b, float64(v), %d)\n", bits)
	case nucleo.CBool:
		p("\treturn strconv.AppendBool(b, bool(v))\n")
	case nucleo.CString:
		p("\treturn nyx__encTexto(b, string(v))\n")
	case nucleo.CLista:
		p("\tif v == nil {\n\t\treturn append(b, \"null\"...)\n\t}\n")
		fallthrough
	case nucleo.CArreglo:
		p("\tb = append(b, '[')\n\tfor i, e := range v {\n\t\tif i > 0 {\n\t\t\tb = append(b, ',')\n\t\t}\n\t\tb = nyx__enc_%d(b, e)\n\t}\n\treturn append(b, ']')\n", hijos[0])
	case nucleo.CMapa:
		K := goTipo(elem(t.Clave), suf)
		menor := "ks[i] < ks[j]"
		if elem(t.Clave).Clase == nucleo.CBool {
			menor = "!bool(ks[i]) && bool(ks[j])"
		}
		p("\tif v == nil {\n\t\treturn append(b, \"null\"...)\n\t}\n\tks := make([]%s, 0, len(v))\n\tfor k := range v {\n\t\tks = append(ks, k)\n\t}\n", K)
		p("\tsort.Slice(ks, func(i, j int) bool { return %s })\n", menor)
		p("\tb = append(b, '[')\n\tfor i, k := range ks {\n\t\tif i > 0 {\n\t\t\tb = append(b, ',')\n\t\t}\n\t\tb = append(b, '[')\n\t\tb = nyx__enc_%d(b, k)\n\t\tb = append(b, ',')\n\t\tb = nyx__enc_%d(b, v[k])\n\t\tb = append(b, ']')\n\t}\n\treturn append(b, ']')\n", hijos[0], hijos[1])
	case nucleo.CStruct:
		p("\tb = append(b, '[')\n")
		for i, c := range t.Campos {
			if i > 0 {
				p("\tb = append(b, ',')\n")
			}
			p("\tb = nyx__enc_%d(b, v.%s)\n", hijos[i], nombreCampo(c, suf))
		}
		p("\treturn append(b, ']')\n")
	case nucleo.CPuntero:
		p("\tif v == nil {\n\t\treturn append(b, \"null\"...)\n\t}\n\treturn nyx__enc_%d(b, *v)\n", hijos[0])
	default:
		p("\treturn append(b, \"null\"...)\n")
	}
	p("}\n")
	// the named result and the type alias keep the generated text short and readable
	return strings.ReplaceAll(sb.String(), fmt.Sprintf("nyx__T%d", n), T)
}

func sinSigno(t nucleo.Tipo) bool {
	if t.Clase == nucleo.CByte {
		return true
	}
	switch t.Nombre {
	case "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte":
		return true
	}
	return false
}

// llamada spells the call of variant k with the given argument names.
func llamada(f nucleo.Firma, suf string, args []string) string {
	desp := 0
	var obj string
	if f.Receptor != nil {
		obj = args[0] + "." + f.Nombre
		desp = 1
	} else {
		obj = f.Nombre + suf
	}
	resto := append([]string(nil), args[desp:]...)
	if f.Variadica && len(resto) > 0 {
		resto[len(resto)-1] += "..."
	}
	return obj + "(" + strings.Join(resto, ", ") + ")"
}

// referenciaF is what F means in the properties of variant k.
func referenciaF(f nucleo.Firma, suf string) string {
	if f.Receptor == nil {
		return f.Nombre + suf
	}
	t := f.Receptor.Tipo
	if t.Clase == nucleo.CPuntero {
		return "(" + goTipo(t, suf) + ")." + f.Nombre
	}
	return goTipo(t, suf) + "." + f.Nombre
}

func (g *generador) decResultado(t nucleo.Tipo, k int, suf string) string {
	if t.Clase == nucleo.CError {
		return "nyx__decError"
	}
	return "nyx__dec_" + strconv.Itoa(g.codec(t, k, suf))
}

func (g *generador) encResultado(t nucleo.Tipo, k int, suf string, v string) string {
	if t.Clase == nucleo.CError {
		return "nyx__encError(nil, " + v + ")"
	}
	return "nyx__enc_" + strconv.Itoa(g.codec(t, k, suf)) + "(nil, " + v + ")"
}

// runner writes nyx__correr_v<k>.
func (g *generador) runner(w *escritor, k int, suf string, props []nucleo.Propiedad, indices []int) {
	f := g.op.firma
	ent := f.Entradas()
	res := f.Res
	for _, t := range ent {
		paquetesDe(t, g.importsArnes)
	}
	for _, t := range res {
		paquetesDe(t, g.importsArnes)
	}
	w.actual = duenio{v: k, p: -1}
	w.f("\n// variante %d\nfunc nyx__correr_v%d(nyx__x *nyx__Ejec) {\n\tvar nyx__err error\n\t_ = nyx__err\n", k, k)
	w.f("\tif len(nyx__x.E) != %d {\n\t\tnyx__x.Dec(errors.New(\"el caso no tiene %d entradas\"))\n\t\treturn\n\t}\n", len(ent), len(ent))
	decs := make([]string, len(ent))
	a := make([]string, len(ent))
	for i, t := range ent {
		decs[i] = "nyx__dec_" + strconv.Itoa(g.codec(t, k, suf))
		a[i] = "nyx__a" + strconv.Itoa(i)
		w.f("\tvar %s %s\n\tif %s, nyx__err = %s(nyx__x.E[%d]); nyx__x.Dec(nyx__err) {\n\t\treturn\n\t}\n", a[i], goTipo(t, suf), a[i], decs[i], i)
	}
	rs := make([]string, len(res))
	for i, t := range res {
		rs[i] = "r" + strconv.Itoa(i)
		w.f("\tvar %s %s\n", rs[i], goTipoResultado(t, suf))
	}
	w.f("\tif !nyx__x.Llamar(func() { %s = %s }) {\n\t\treturn\n\t}\n", strings.Join(rs, ", "), llamada(f, suf, a))
	encs := make([]string, len(res))
	for i, t := range res {
		encs[i] = g.encResultado(t, k, suf, rs[i])
	}
	w.f("\tnyx__x.O = [][]byte{%s}\n", strings.Join(encs, ", "))
	w.f("\tif nyx__x.ConR {\n\t\tif len(nyx__x.R) != %d {\n\t\t\tnyx__x.Distinto = true\n\t\t} else {\n", len(res))
	for i, t := range res {
		w.f("\t\t\tif q, err := %s(nyx__x.R[%d]); err != nil || !nyx__Igual(%s, q) {\n\t\t\t\tnyx__x.Distinto = true\n\t\t\t}\n", g.decResultado(t, k, suf), i, rs[i])
	}
	w.escribir("\t\t}\n\t}\n")

	// properties
	var activas []int
	for i := range props {
		if !g.op.sinProp[[2]int{k, indices[i]}] {
			activas = append(activas, i)
		}
	}
	if len(activas) > 0 {
		w.escribir("\t{\n")
		es := make([]string, len(ent))
		for i, t := range ent {
			es[i] = "e" + strconv.Itoa(i)
			w.f("\t\tvar %s %s\n\t\tif %s, nyx__err = %s(nyx__x.E[%d]); nyx__x.Dec(nyx__err) {\n\t\t\treturn\n\t\t}\n", es[i], goTipo(t, suf), es[i], decs[i], i)
		}
		w.f("\t\tF := %s\n", referenciaF(f, suf))
		usados := append(append([]string{"F"}, es...), rs...)
		w.f("\t\t_ = []any{%s}\n", strings.Join(usados, ", "))
		var nombres map[string]string
		if k < len(g.op.nombres) {
			nombres = g.op.nombres[k]
		}
		for _, i := range activas {
			p := props[i]
			w.actual = duenio{v: k, p: indices[i]}
			w.f("\t\tnyx__x.Prop(%q, func() (bool, bool) {\n", p.Nombre)
			if p.Requiere != "" {
				w.f("\t\t\tif !(%s) {\n\t\t\t\treturn false, true\n\t\t\t}\n", unaLinea(renombrarExpr(p.Requiere, nombres)))
			}
			w.f("\t\t\treturn (%s), false\n\t\t})\n", unaLinea(renombrarExpr(p.Expr, nombres)))
			w.actual = duenio{v: k, p: -1}
		}
		w.escribir("\t}\n")
	}

	// determinism
	w.escribir("\tfor nyx__i := 1; nyx__i < nyx__x.Repetir; nyx__i++ {\n")
	bs := make([]string, len(ent))
	for i, t := range ent {
		bs[i] = "nyx__b" + strconv.Itoa(i)
		w.f("\t\tvar %s %s\n\t\tif %s, nyx__err = %s(nyx__x.E[%d]); nyx__x.Dec(nyx__err) {\n\t\t\treturn\n\t\t}\n", bs[i], goTipo(t, suf), bs[i], decs[i], i)
	}
	ss := make([]string, len(res))
	encs2 := make([]string, len(res))
	for i, t := range res {
		ss[i] = "nyx__s" + strconv.Itoa(i)
		encs2[i] = g.encResultado(t, k, suf, ss[i])
	}
	w.f("\t\tnyx__x.Repetida(func() [][]byte {\n\t\t\t%s := %s\n\t\t\treturn [][]byte{%s}\n\t\t})\n\t}\n}\n", strings.Join(ss, ", "), llamada(f, suf, bs), strings.Join(encs2, ", "))
}

func goTipoResultado(t nucleo.Tipo, suf string) string {
	if t.Clase == nucleo.CError && !t.Definido {
		return "error"
	}
	return goTipo(t, suf)
}

// unaLinea keeps a property on one line, so its build errors land on its own lines.
func unaLinea(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}
