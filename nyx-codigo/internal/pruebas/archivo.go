package pruebas

import (
	"go/format"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// ArchivoTest writes a table-driven _test.go file for f in package paquete (gofmt'ed). Cases with an
// expected value go into Test<Nombre>; cases without one go into Test<Nombre>SinPanico, which only checks
// that the call does not panic. Results are compared with a helper that has the same meaning as
// nucleo.Igual: nil and empty lists are equal, floats are compared within 1e-9, and for an error result
// only "there is an error or not" is checked.
func ArchivoTest(f nucleo.Firma, casos []nucleo.Caso, paquete string) string {
	if paquete == "" {
		paquete = "solucion"
	}
	ent := f.Entradas()
	desp := len(ent) - len(f.Params)

	// table field names for inputs
	reservados := map[string]bool{"nombre": true, "quiereError": true, "c": true, "t": true, "casos": true}
	campos := make([]string, len(ent))
	usados := map[string]bool{}
	for i := range ent {
		n := ""
		if i < desp {
			n = "receptor"
		} else if p := f.Params[i-desp].Nombre; p != "" && p != "_" {
			n = p
		}
		if n == "" {
			n = "e" + strconv.Itoa(i)
		}
		for reservados[n] || usados[n] || strings.HasPrefix(n, "quiero") || strings.HasPrefix(n, "obtenido") {
			n = "en" + mayusculaInicial(n)
		}
		usados[n] = true
		campos[i] = n
	}
	// results
	errIdx := -1
	if len(f.Res) > 0 && f.Res[len(f.Res)-1].Clase == nucleo.CError {
		errIdx = len(f.Res) - 1
	}
	nValores := len(f.Res)
	if errIdx >= 0 {
		nValores--
	}
	quiero := func(i int) string {
		if nValores == 1 {
			return "quiero"
		}
		return "quiero" + strconv.Itoa(i)
	}
	obtenido := func(i int) string {
		if nValores == 1 {
			return "obtenido"
		}
		return "obtenido" + strconv.Itoa(i)
	}

	// call expression and its printable form
	args := make([]string, 0, len(ent))
	formatos := make([]string, 0, len(ent))
	for i := desp; i < len(ent); i++ {
		a := "c." + campos[i]
		formatos = append(formatos, "%v")
		if f.Variadica && i == len(ent)-1 {
			a += "..."
		}
		args = append(args, a)
	}
	var llamada, mostrar string
	argsMostrar := make([]string, 0, len(ent))
	if desp > 0 {
		llamada = "c." + campos[0] + "." + f.Nombre + "(" + strings.Join(args, ", ") + ")"
		mostrar = "%v." + f.Nombre + "(" + strings.Join(formatos, ", ") + ")"
		argsMostrar = append(argsMostrar, "c."+campos[0])
	} else {
		llamada = f.Nombre + "(" + strings.Join(args, ", ") + ")"
		mostrar = f.Nombre + "(" + strings.Join(formatos, ", ") + ")"
	}
	for i := desp; i < len(ent); i++ {
		argsMostrar = append(argsMostrar, "c."+campos[i])
	}

	nombreTest := "Test" + mayusculaInicial(f.Nombre)
	if f.Receptor != nil {
		tr := f.Receptor.Tipo
		if tr.Clase == nucleo.CPuntero && tr.Elem != nil {
			tr = *tr.Elem
		}
		if tr.Nombre != "" {
			nombreTest = "Test" + mayusculaInicial(tr.Nombre) + "_" + f.Nombre
		}
	}

	// the helpers carry the test's name so that several generated files can live in one package
	sufijo := strings.TrimPrefix(nombreTest, "Test")

	var con, sin []nucleo.Caso
	for _, c := range casos {
		if len(c.Entradas) != len(ent) {
			continue
		}
		if c.ConEsperado() && len(c.Esperado) == len(f.Res) {
			con = append(con, c)
		} else {
			sin = append(sin, c)
		}
	}

	var sb strings.Builder
	sb.WriteString("// Pruebas generadas por Nyx Código.\n\n")
	sb.WriteString("package " + paquete + "\n\n")
	sb.WriteString("import (\n\t\"math\"\n\t\"reflect\"\n\t\"testing\"\n)\n\n")

	entradas := func(c nucleo.Caso) []string {
		var partes []string
		for i, v := range c.Entradas {
			partes = append(partes, campos[i]+": "+nucleo.FormatoGo(v, ent[i]))
		}
		return partes
	}
	nombreCaso := func(c nucleo.Caso, n int) string {
		if c.Nota != "" {
			return c.Nota
		}
		return "caso " + strconv.Itoa(n)
	}

	if len(con) > 0 || len(sin) == 0 {
		sb.WriteString("func " + nombreTest + "(t *testing.T) {\n")
		sb.WriteString("\tcasos := []struct {\n\t\tnombre string\n")
		for i, t := range ent {
			sb.WriteString("\t\t" + campos[i] + " " + t.Go() + "\n")
		}
		k := 0
		for i, r := range f.Res {
			if i == errIdx {
				continue
			}
			sb.WriteString("\t\t" + quiero(k) + " " + r.Go() + "\n")
			k++
		}
		if errIdx >= 0 {
			sb.WriteString("\t\tquiereError bool\n")
		}
		sb.WriteString("\t}{\n")
		for n, c := range con {
			partes := []string{"nombre: " + strconv.Quote(nombreCaso(c, n+1))}
			partes = append(partes, entradas(c)...)
			k := 0
			for i, r := range f.Res {
				if i == errIdx {
					if c.Esperado[i] != nil {
						partes = append(partes, "quiereError: true")
					}
					continue
				}
				partes = append(partes, quiero(k)+": "+nucleo.FormatoGo(c.Esperado[i], r))
				k++
			}
			sb.WriteString("\t\t{" + strings.Join(partes, ", ") + "},\n")
		}
		sb.WriteString("\t}\n")
		sb.WriteString("\tfor _, c := range casos {\n\t\tc := c\n\t\tt.Run(c.nombre, func(t *testing.T) {\n")
		var izq []string
		k = 0
		for i := range f.Res {
			if i == errIdx {
				izq = append(izq, "err")
				continue
			}
			izq = append(izq, obtenido(k))
			k++
		}
		sb.WriteString("\t\t\t" + strings.Join(izq, ", ") + " := " + llamada + "\n")
		for k := 0; k < nValores; k++ {
			sb.WriteString("\t\t\tif !iguales" + sufijo + "(" + obtenido(k) + ", c." + quiero(k) + ") {\n")
			sb.WriteString("\t\t\t\tt.Errorf(" + strconv.Quote(mostrar+" = %v; quiero %v") + ", " +
				strings.Join(argsMostrar, ", ") + sep(argsMostrar) + obtenido(k) + ", c." + quiero(k) + ")\n")
			sb.WriteString("\t\t\t}\n")
		}
		if errIdx >= 0 {
			sb.WriteString("\t\t\tif (err != nil) != c.quiereError {\n")
			sb.WriteString("\t\t\t\tt.Errorf(" + strconv.Quote(mostrar+": error = %v; ¿quiero error? %v") + ", " +
				strings.Join(argsMostrar, ", ") + sep(argsMostrar) + "err, c.quiereError)\n")
			sb.WriteString("\t\t\t}\n")
		}
		sb.WriteString("\t\t})\n\t}\n}\n\n")
	}

	if len(sin) > 0 {
		sb.WriteString("// " + nombreTest + "SinPanico solo comprueba que la función no entra en pánico.\n")
		sb.WriteString("func " + nombreTest + "SinPanico(t *testing.T) {\n")
		sb.WriteString("\tcasos := []struct {\n\t\tnombre string\n")
		for i, t := range ent {
			sb.WriteString("\t\t" + campos[i] + " " + t.Go() + "\n")
		}
		sb.WriteString("\t}{\n")
		for n, c := range sin {
			partes := []string{"nombre: " + strconv.Quote(nombreCaso(c, n+1))}
			partes = append(partes, entradas(c)...)
			sb.WriteString("\t\t{" + strings.Join(partes, ", ") + "},\n")
		}
		sb.WriteString("\t}\n")
		sb.WriteString("\tfor _, c := range casos {\n\t\tc := c\n\t\tt.Run(c.nombre, func(t *testing.T) {\n")
		blancos := make([]string, len(f.Res))
		for i := range blancos {
			blancos[i] = "_"
		}
		if len(f.Res) > 0 {
			sb.WriteString("\t\t\t" + strings.Join(blancos, ", ") + " = " + llamada + "\n")
		} else {
			sb.WriteString("\t\t\t" + llamada + "\n")
		}
		sb.WriteString("\t\t})\n\t}\n}\n\n")
	}

	sb.WriteString(strings.ReplaceAll(ayudaIguales, "XX", sufijo))
	src := sb.String()
	if b, err := format.Source([]byte(src)); err == nil {
		return string(b)
	}
	return src
}

func sep(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return ", "
}

func mayusculaInicial(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// ayudaIguales has the meaning of nucleo.Igual. It reads unexported struct fields through reflect
// without calling Interface, so it works on any value of the package.
const ayudaIguales = `// igualesXX compara como Nyx Código: nil y una lista vacía son iguales, y los decimales
// se comparan con un margen de 1e-9.
func igualesXX(a, b any) bool {
	return igualesValorXX(reflect.ValueOf(a), reflect.ValueOf(b))
}

func igualesValorXX(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return vacioXX(a) && vacioXX(b)
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Float32, reflect.Float64:
		x, y := a.Float(), b.Float()
		if math.IsNaN(x) || math.IsNaN(y) {
			return math.IsNaN(x) && math.IsNaN(y)
		}
		if x == y {
			return true
		}
		return math.Abs(x-y) <= 1e-9*math.Max(1, math.Max(math.Abs(x), math.Abs(y)))
	case reflect.Slice, reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !igualesValorXX(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		it := a.MapRange()
		for it.Next() {
			w := b.MapIndex(it.Key())
			if !w.IsValid() || !igualesValorXX(it.Value(), w) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !igualesValorXX(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() && b.IsNil()
		}
		return igualesValorXX(a.Elem(), b.Elem())
	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.String:
		return a.String() == b.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() == b.Uint()
	}
	return false
}

func vacioXX(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	}
	return false
}
`
