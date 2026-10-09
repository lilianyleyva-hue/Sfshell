package reparar

import (
	"context"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"time"

	"nyxcodigo/internal/nucleo"
)

// puntaje weighs errors: unused variables and imports count half, so that a fix that leaves only
// "declared and not used" behind is progress.
func puntaje(errs []nucleo.ErrorGo) int {
	p := 0
	for _, e := range errs {
		m := primeraLinea(e.Msg)
		if strings.Contains(m, "declared and not used") || strings.Contains(m, "declared but not used") || strings.Contains(m, "imported and not used") || strings.Contains(m, "imported as") {
			p++
		} else {
			p += 2
		}
	}
	return p
}

// maxIntentosRonda caps the single-edit attempts of one backtracking round.
const maxIntentosRonda = 30

// Compilacion runs the compile-error repair loop: foreign idioms (only when the code does not compile),
// syntax edits until it parses, then type rules, each round keeping only edits that lower the error
// count (backtracking to the next-ranked edit otherwise). At the end it runs one real Ej.Compilar and one
// Ej.Vet; vet findings are translated, not fixed, except a wrong Printf verb. Without Ej, OK comes from
// the in-process go/types check.
func (r *Reparador) Compilacion(ctx context.Context, fuente string, n *nucleo.Nodo) Arreglo {
	if ctx == nil {
		ctx = context.Background()
	}
	lim := r.Limite.conDefectos()
	limite := time.Now().Add(lim.TCompilacion)
	paso := n.Sub(nucleo.PasoArreglo, "Reviso si compila")
	ar := Arreglo{Fuente: fuente}
	src := fuente
	a := analizar(src)
	if len(a.errs) > 0 {
		paso.Detalle("Encontré %d errores; el primero: %s", len(a.errs), Traducir(a.errs[0]))
		nuevo, cs := r.idiomasSeguros(src, a)
		if nuevo != src {
			src = nuevo
			ar.Cambios = append(ar.Cambios, cs...)
			for _, c := range cs {
				paso.Sub(nucleo.PasoArreglo, "%s", c.Porque).Bien("")
			}
		}
	}

	for ar.Rondas < lim.Rondas && time.Now().Before(limite) && ctx.Err() == nil {
		a = analizar(src)
		if a.sintaxis {
			nuevo, cs, rondas, ok := repararSintaxis(src, lim.Rondas-ar.Rondas, limite)
			ar.Rondas += rondas
			if rondas == 0 {
				ar.Rondas++
			}
			src = nuevo
			ar.Cambios = append(ar.Cambios, cs...)
			for _, c := range cs {
				paso.Sub(nucleo.PasoArreglo, "%s (línea %d)", c.Porque, c.Linea).Bien("")
				r.exito("regla:"+c.Regla, true)
			}
			if !ok {
				break
			}
			continue
		}
		if len(a.errs) == 0 {
			break
		}
		ar.Rondas++
		nuevo, aplicadas, ok := r.rondaTipos(a)
		if !ok {
			break
		}
		src = nuevo
		for _, e := range aplicadas {
			ar.Cambios = append(ar.Cambios, e.cambio)
			if e.suposicion != "" {
				ar.Suposiciones = append(ar.Suposiciones, e.suposicion)
			}
			paso.Sub(nucleo.PasoArreglo, "%s (línea %d)", e.cambio.Porque, e.cambio.Linea).Bien("")
		}
	}

	// gofmt the result when it parses: the inserted lines get the right indentation
	if b, err := format.Source([]byte(src)); err == nil {
		src = string(b)
	}
	ar.Fuente = src
	ar.Cambios = sinRepetirCambios(ar.Cambios)
	ar.Suposiciones = sinRepetirTextos(ar.Suposiciones)

	final := analizar(src)
	ar.Quedan = final.errs
	if r.Ej == nil {
		ar.OK = len(final.errs) == 0
	} else if len(final.errs) == 0 || !final.sintaxis {
		r.compilarDeVerdad(ctx, &ar)
	}
	if ar.OK {
		paso.Bien("Compila")
	} else if len(ar.Quedan) > 0 {
		paso.Mal("Quedan %d errores", len(ar.Quedan))
	} else {
		paso.Mal("Compila, pero go vet encontró %d avisos", len(ar.Vet))
	}
	return ar
}

// idiomasSeguros applies Idiomas to code that does not compile. When the code parses, rewrites that
// create a new error on their own line are not applied.
func (r *Reparador) idiomasSeguros(src string, a *analisis) (string, []Cambio) {
	nuevo, cs := idiomas(src, nil)
	if nuevo == src || a.sintaxis {
		return nuevo, cs
	}
	conError := map[int]bool{}
	for _, e := range a.errs {
		conError[e.Linea] = true
	}
	b := analizar(nuevo)
	prohibidas := map[int]bool{}
	if b.sintaxis {
		for _, c := range cs {
			prohibidas[c.Linea] = true
		}
	} else {
		for _, e := range b.errs {
			if !conError[e.Linea] {
				prohibidas[e.Linea] = true
			}
		}
	}
	if len(prohibidas) == 0 {
		return nuevo, cs
	}
	nuevo, cs = idiomas(src, func(l int) bool { return !prohibidas[l] && conError[l] })
	if c := analizar(nuevo); c.sintaxis || puntaje(c.errs) > puntaje(a.errs) {
		return src, nil
	}
	return nuevo, cs
}

// rondaTipos does one round of type rules: the best non-overlapping edit for each error together;
// if the error count does not fall, single edits are tried in rank order (first error first).
func (r *Reparador) rondaTipos(a *analisis) (string, []edicion, bool) {
	antes := puntaje(a.errs)
	cands := make([][]edicion, len(a.errs))
	for i, e := range a.errs {
		cands[i] = r.candidatos(a, e)
	}
	mejora := func(nuevo string) bool {
		if nuevo == a.fuente {
			return false
		}
		b := analizar(nuevo)
		return !b.sintaxis && puntaje(b.errs) < antes
	}
	var primeras []edicion
	for _, c := range cands {
		if len(c) > 0 {
			primeras = append(primeras, c[0])
		}
	}
	if len(primeras) > 0 {
		nuevo, aplicadas := aplicar(a.fuente, primeras)
		if mejora(nuevo) {
			for _, e := range aplicadas {
				r.exito("regla:"+e.cambio.Regla, true)
			}
			return nuevo, aplicadas, true
		}
	}
	intentos := 0
	for _, c := range cands {
		for j, e := range c {
			if intentos >= maxIntentosRonda {
				return "", nil, false
			}
			intentos++
			nuevo, aplicadas := aplicar(a.fuente, []edicion{e})
			if mejora(nuevo) {
				r.exito("regla:"+e.cambio.Regla, true)
				return nuevo, aplicadas, true
			}
			if j == 0 {
				r.exito("regla:"+e.cambio.Regla, false)
			}
		}
	}
	return "", nil, false
}

var reVerbo = regexp.MustCompile(`format %(\w) has arg (.+) of wrong type (\S+)`)

// compilarDeVerdad runs one Ej.Compilar and one Ej.Vet; a wrong Printf verb is fixed and checked once more.
func (r *Reparador) compilarDeVerdad(ctx context.Context, ar *Arreglo) {
	comp := r.Ej.Compilar(ctx, ar.Fuente)
	if !comp.OK {
		ar.OK = false
		ar.Quedan = comp.Errores
		if len(ar.Quedan) == 0 {
			ar.Quedan = []nucleo.ErrorGo{{Archivo: archivoFuente, Linea: 1, Col: 1, Msg: strings.TrimSpace(comp.Texto)}}
		}
		return
	}
	ar.Quedan = nil
	vet := r.Ej.Vet(ctx, ar.Fuente)
	ar.Vet = vet.Errores
	if !vet.OK && len(vet.Errores) == 0 && strings.TrimSpace(vet.Texto) != "" {
		ar.Vet = []nucleo.ErrorGo{{Archivo: archivoFuente, Linea: 1, Col: 1, Msg: strings.TrimSpace(vet.Texto)}}
	}
	cambiado := false
	for _, e := range ar.Vet {
		if m := reVerbo.FindStringSubmatch(e.Msg); m != nil {
			if nuevo, c, ok := cambiarVerbo(ar.Fuente, e, m[1], m[2], m[3]); ok {
				ar.Fuente = nuevo
				ar.Cambios = append(ar.Cambios, c)
				cambiado = true
			}
		}
	}
	if cambiado {
		comp = r.Ej.Compilar(ctx, ar.Fuente)
		if !comp.OK {
			ar.OK = false
			ar.Quedan = comp.Errores
			return
		}
		vet = r.Ej.Vet(ctx, ar.Fuente)
		ar.Vet = vet.Errores
	}
	ar.OK = len(ar.Vet) == 0 && (vet.OK || len(vet.Errores) == 0 && strings.TrimSpace(vet.Texto) == "")
}

// verboPara chooses a Printf verb for a Go type name.
func verboPara(tipo string) string {
	switch {
	case tipo == "string":
		return "s"
	case tipo == "bool":
		return "t"
	case strings.HasPrefix(tipo, "int") || strings.HasPrefix(tipo, "uint"):
		return "d"
	}
	return "v"
}

// cambiarVerbo fixes "fmt.Printf format %d has arg x of wrong type string" on line e.Linea.
func cambiarVerbo(src string, e nucleo.ErrorGo, verbo, arg, tipo string) (string, Cambio, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, archivoFuente, src, 0)
	if err != nil {
		return "", Cambio{}, false
	}
	var resultado string
	var cambio Cambio
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || resultado != "" || fset.Position(c.Pos()).Line > e.Linea || fset.Position(c.End()).Line < e.Linea {
			return resultado == ""
		}
		// the format is the first string literal argument
		fi := -1
		for i, x := range c.Args {
			if bl, ok := x.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				fi = i
				break
			}
		}
		if fi < 0 {
			return true
		}
		ai := -1
		for i := fi + 1; i < len(c.Args); i++ {
			ini, fin := fset.Position(c.Args[i].Pos()).Offset, fset.Position(c.Args[i].End()).Offset
			if strings.TrimSpace(src[ini:fin]) == strings.TrimSpace(arg) {
				ai = i - fi - 1
				break
			}
		}
		if ai < 0 {
			return true
		}
		lit := c.Args[fi].(*ast.BasicLit)
		formato, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		nuevoFormato, ok := reemplazarVerbo(formato, ai, verbo, verboPara(tipo))
		if !ok {
			return true
		}
		ini, fin := fset.Position(lit.Pos()).Offset, fset.Position(lit.End()).Offset
		nuevaLit := strconv.Quote(nuevoFormato)
		if strings.HasPrefix(lit.Value, "`") && !strings.Contains(nuevoFormato, "`") {
			nuevaLit = "`" + nuevoFormato + "`"
		}
		resultado = src[:ini] + nuevaLit + src[fin:]
		cambio = Cambio{Regla: "verbo_printf", Linea: e.Linea, Antes: textoLinea(src, e.Linea), Despues: textoLinea(resultado, e.Linea),
			Porque: "en el Printf, «%" + verbo + "» no sirve para un valor de tipo " + tipo + ": uso «%" + verboPara(tipo) + "»"}
		return false
	})
	return resultado, cambio, resultado != ""
}

// reemplazarVerbo changes the verb of the k-th argument in a Printf format.
func reemplazarVerbo(formato string, k int, viejo, nuevo string) (string, bool) {
	n := 0
	for i := 0; i < len(formato); i++ {
		if formato[i] != '%' {
			continue
		}
		if i+1 < len(formato) && formato[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		for j < len(formato) && strings.IndexByte("+-# 0123456789.*[]", formato[j]) >= 0 {
			j++
		}
		if j >= len(formato) {
			return "", false
		}
		if n == k {
			if string(formato[j]) != viejo {
				return "", false
			}
			return formato[:j] + nuevo + formato[j+1:], true
		}
		n++
		i = j
	}
	return "", false
}

func sinRepetirCambios(cs []Cambio) []Cambio {
	visto := map[string]bool{}
	var out []Cambio
	for _, c := range cs {
		k := c.Regla + "\x00" + c.Antes + "\x00" + c.Despues
		if c.Regla != "importar_paquete" {
			k += "\x00" + strconv.Itoa(c.Linea)
		}
		if visto[k] {
			continue
		}
		visto[k] = true
		out = append(out, c)
	}
	return out
}

func sinRepetirTextos(xs []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !visto[x] {
			visto[x] = true
			out = append(out, x)
		}
	}
	return out
}
