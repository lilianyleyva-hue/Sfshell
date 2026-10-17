package cerebro

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/reparar"
	"nyxcodigo/internal/sintesis"
)

// pipeReparar is the pipeline to arreglar código (reparar, §4.14.2): compile errors with rule-based fixes,
// then the target function is tested against the spec (the user's examples, "con X da Y" reports,
// counterexamples, concept properties and a stored reference) plus generated cases, and the failures are
// repaired (panics, then fault localization and mutation), each fix checked by running the tests again.
func pipeReparar(c *Cerebro, k *tarea) (resp nucleo.Respuesta) {
	codigo := c.repCodigo(k, "Pega el código que quieres que arregle")
	if codigo == "" {
		return repSinCodigo("arreglarlo")
	}
	plan := k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) que compile 2) elegir la función 3) probarla 4) arreglar lo que falle")
	plan.Info("")
	src := repCompletar(codigo)
	if src.sinMain {
		k.raiz.Nota("Es `package main` sin `func main`: lo trato como un archivo de funciones")
		defer func() { src.volverMain(&resp) }()
	}
	base := repFormatear(src.fuente)

	// 1-2. compile errors, explained, and the rule-based repair loop
	var errores []string
	if errs, _ := reparar.ChequeoTipos(src.fuente); len(errs) > 0 {
		n := k.raiz.Sub(nucleo.PasoError, "No compila: %d errores", len(errs))
		for _, e := range errs {
			t := reparar.Traducir(src.linea(e))
			errores = append(errores, t)
			n.Detalle("%s", t)
		}
		n.Mal("")
	}
	var cambios []reparar.Cambio
	for _, imp := range src.importes {
		cambios = append(cambios, reparar.Cambio{Regla: "importar_paquete", Despues: "import \"" + imp + "\"", Porque: "faltaba importar «" + imp + "»"})
	}
	ar := c.reparador(k).Compilacion(k.ctx, src.fuente, k.raiz)
	for _, cb := range ar.Cambios {
		cb.Linea = src.lineaUsuario(cb.Linea)
		cambios = append(cambios, cb)
	}
	if !ar.OK && len(ar.Quedan) > 0 {
		motivo := "no consigo que compile"
		contra := "Queda este error: " + reparar.Traducir(src.linea(ar.Quedan[0]))
		r := c.respuestaFallo(k, motivo, ar.Fuente, contra)
		r.Diff = reparar.Diff(base, ar.Fuente)
		r.Parrafos = append(repParrafosCambios(errores, cambios, ar.Suposiciones), r.Parrafos...)
		r.Metodo = "reparacion"
		return r
	}
	actual := ar.Fuente
	var notas []string
	for _, v := range ar.Vet {
		notas = append(notas, "go vet dice: "+reparar.Traducir(src.linea(v)))
	}
	if c.ej == nil {
		return repSinCaja(k, base, actual, errores, cambios, ar.Suposiciones)
	}

	// 3. the target function
	inf, _ := analisis.Analizar(actual)
	fn, hay := c.repElegir(k, actual, inf)
	if !hay {
		return c.repPrograma(k, base, actual, errores, cambios, ar.Suposiciones, notas)
	}
	if fn.Firma == nil || !fn.Firma.Probable() {
		motivo := fn.NoProbable
		if motivo == "" {
			motivo = "sus tipos no los sé generar"
		}
		k.raiz.Sub(nucleo.PasoNota, "No puedo probar «%s» solo: %s", fn.Nombre, motivo).Info("")
		r := repRespuestaCompila(base, actual, errores, cambios, ar.Suposiciones)
		r.Parrafos = append(r.Parrafos, "No puedo probar «"+fn.Nombre+"» automáticamente: "+motivo+".")
		return r
	}
	f := *fn.Firma
	k.q = repConFirma(k.q, &f)
	permiso := nucleo.PermisoAuto
	if v := c.ej.Revisar(actual, nucleo.PermisoAuto); len(v) > 0 {
		pr, ok := c.ejcPermiso(k, actual, v, "probarlo")
		if !ok {
			r := repRespuestaCompila(base, actual, errores, cambios, ar.Suposiciones)
			r.Parrafos = append(append(r.Parrafos, pr.Texto), pr.Parrafos...)
			r.Riesgos = pr.Riesgos
			return r
		}
		permiso = nucleo.PermisoUsuario
	}

	// 4. the spec and the generated cases
	e := c.repEspec(k, f, fn.Nombre)
	defer e.cerrar()
	repRecordarEspec(k, e)
	casos := e.casos(c, k)

	// 5. run the original
	bOrig, rsOrig, err := c.repCorrerCasos(k, k.raiz, "Pruebo «"+f.Nombre+"» con "+fmt.Sprint(len(casos))+" casos", actual, e, casos, permiso)
	if err != nil {
		r := repRespuestaCompila(base, actual, errores, cambios, ar.Suposiciones)
		r.Parrafos = append(r.Parrafos, "No pude probarlo: "+mensajeError(err)+".")
		return r
	}
	defer bOrig.Cerrar()
	fallos := repFallos(rsOrig, len(casos))
	if len(fallos) == 0 {
		return c.repSinFallos(k, e, base, actual, casos, rsOrig, errores, cambios, ar.Suposiciones, notas)
	}

	// 6. repair: panics first, then logic (with the reference oracle when there is one)
	contra := c.repContraejemplo(k, bOrig, e, casos, rsOrig, fallos)
	if permiso != nucleo.PermisoAuto {
		// the repair search (reparar.Panicos/Logica) only builds code that passes the automatic check
		k.raiz.Sub(nucleo.PasoNota, "Con código que necesita tu permiso no busco el arreglo solo").Info("")
		comps := e.comprobaciones(k, casos, rsOrig)
		r := repRespuestaCompila(base, actual, errores, cambios, ar.Suposiciones)
		r.Texto = fmt.Sprintf("Lo probé con tu permiso y falla en %d de %d casos.", len(fallos), len(casos))
		r.Comprobaciones, r.Nivel, r.Exito = comps, pruebas.NivelGlobal(comps), false
		if contra != "" {
			r.Parrafos = append(r.Parrafos, contra)
		}
		r.Parrafos = append(r.Parrafos, "Todavía no sé buscar el arreglo solo en código que necesita tu permiso: arregla el fallo y vuelve a pedirme que lo pruebe.")
		r.Parrafos = append(r.Parrafos, notas...)
		r.Tablas = []nucleo.Tabla{pruebas.Tabla(f, casos, rsOrig, 0)}
		r.Justificacion = repJustificacion(k, e, casos, comps, cambios)
		return r
	}
	nuevo, cambiosLogica, cerrado, casos, notasRep := c.repLogica(k, e, actual, casos)
	notas = append(notas, notasRep...)
	if !cerrado {
		mejor := nuevo
		if mejor == "" {
			mejor = actual
		}
		r := c.respuestaFallo(k, "no encontré un arreglo que pase todas las pruebas", mejor, contra)
		r.Diff = reparar.Diff(base, mejor)
		r.Parrafos = append(repParrafosCambios(errores, append(cambios, cambiosLogica...), ar.Suposiciones), r.Parrafos...)
		r.Tablas = []nucleo.Tabla{pruebas.Tabla(f, casos, repAlinear(rsOrig, len(casos)), 0)}
		r.Metodo = "reparacion"
		return r
	}
	cambios = append(cambios, cambiosLogica...)

	// 7. check the repaired code once more, on its own
	bNuevo, rsNuevo, err := c.repCorrerCasos(k, k.raiz, "Compruebo el código arreglado", nuevo, e, casos, nucleo.PermisoAuto)
	if err != nil {
		r := c.respuestaFallo(k, "no pude comprobar el arreglo: "+mensajeError(err), nuevo, contra)
		r.Metodo = "reparacion"
		return r
	}
	bNuevo.Cerrar()
	comps := e.comprobaciones(k, casos, rsNuevo)
	nivel := pruebas.NivelGlobal(comps)

	r := nucleo.Respuesta{
		Codigo:         nuevo,
		Diff:           reparar.Diff(base, nuevo),
		Comprobaciones: comps,
		Nivel:          nivel,
		Metodo:         "reparacion",
		Exito:          nivel != nucleo.NivelFallo,
	}
	switch {
	case r.Exito && nivel == nucleo.NivelTusEjemplos:
		r.Texto = "Lo he arreglado y ahora cumple tus ejemplos: " + repResumenCambios(cambios)
	case r.Exito:
		r.Texto = "Lo he arreglado: " + repResumenCambios(cambios)
	default:
		r.Texto = "Lo he cambiado, pero todavía falla alguna prueba."
	}
	r.Parrafos = repParrafosCambios(errores, cambios, ar.Suposiciones)
	if contra != "" {
		r.Parrafos = append(r.Parrafos, contra)
	}
	r.Parrafos = append(r.Parrafos, notas...)
	r.Tablas = []nucleo.Tabla{pruebas.Tabla(f, casos, rsNuevo, 0), repTablaAntesDespues(f, casos, rsOrig, rsNuevo)}
	r.Justificacion = repJustificacion(k, e, casos, comps, cambios)
	if r.Exito && nivel == nucleo.NivelTusEjemplos && f.Receptor == nil {
		if frase := c.repGuardar(k, e, fn.Nombre, nuevo, nivel); frase != "" {
			r.Aprendido = frase
		}
	}
	return r
}

// ---- getting the code ----

// repCodigo returns the code to work on, asking for it when there is none.
func (c *Cerebro) repCodigo(k *tarea, pide string) string {
	if strings.TrimSpace(k.codigo) != "" {
		return k.codigo
	}
	resp, ok := c.preguntar(k, pide, nil)
	if !ok || strings.TrimSpace(resp) == "" {
		return ""
	}
	codigo, _ := lengua.ExtraerCodigo(resp)
	if strings.TrimSpace(codigo) == "" {
		codigo = resp
	}
	k.codigo = codigo
	return codigo
}

// repSinCodigo is the honest answer when there is no code to work on.
func repSinCodigo(para string) nucleo.Respuesta {
	return nucleo.Respuesta{
		Texto:          "Necesito el código para " + para + ".",
		Parrafos:       []string{"Pégalo en el cuadro de código (o en el mensaje) y vuelve a pedírmelo."},
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "sin_codigo",
	}
}

// repFuente is the user's code made into a complete Go file.
type repFuente struct {
	fuente   string   // complete file
	paquete  string   // its package name
	desfase  int      // lines added before the user's first line
	importes []string // imports added while wrapping bare statements (reported as changes)
	// sinMain: the user wrote `package main` with no func main (a file of functions). It is checked and
	// tested as `package solucion` (same line, so line numbers do not move) and shown back as main.
	sinMain bool
}

// paqueteUsuario is the package name to show the user (and to use in the generated _test.go).
func (s repFuente) paqueteUsuario() string {
	if s.sinMain {
		return "main"
	}
	return s.paquete
}

var (
	reClausulaMain     = regexp.MustCompile(`(?m)^(\s*package\s+)main\b`)
	reClausulaSolucion = regexp.MustCompile(`(?m)^(\s*)package solucion\b`)
	reDiffSolucion     = regexp.MustCompile(`(?m)^([ +-]\s*)package solucion\b`)
)

// volverMain puts `package main` back in the code shown to the user when the file was a package main
// without func main (repFuente.sinMain); the checks ran on it as package solucion.
func (s repFuente) volverMain(r *nucleo.Respuesta) {
	if !s.sinMain {
		return
	}
	cambiar := func(re *regexp.Regexp, txt string) string {
		hecho := false
		return re.ReplaceAllStringFunc(txt, func(m string) string {
			if hecho {
				return m
			}
			hecho = true
			return strings.Replace(m, "package solucion", "package main", 1)
		})
	}
	r.Codigo = cambiar(reClausulaSolucion, r.Codigo)
	r.CodigoTest = cambiar(reClausulaSolucion, r.CodigoTest)
	r.Diff = reDiffSolucion.ReplaceAllStringFunc(r.Diff, func(m string) string {
		return strings.Replace(m, "package solucion", "package main", 1)
	})
}

var (
	reClausulaPaquete = regexp.MustCompile(`(?m)^\s*package\s+(\w+)`)
	reFuncMain        = regexp.MustCompile(`(?m)^\s*func\s+main\s*\(\s*\)`)
	reImport          = regexp.MustCompile(`"([a-z0-9_/]+)"`)
)

// repCompletar adds what a snippet lacks to be a file: the package clause (on the user's first line, so
// line numbers do not move), or func main around bare statements (analisis.Normalizar).
func repCompletar(codigo string) repFuente {
	src := strings.TrimSpace(strings.ReplaceAll(codigo, "\r\n", "\n")) + "\n"
	if m := reClausulaPaquete.FindStringSubmatch(src); m != nil {
		if m[1] == "main" && !reFuncMain.MatchString(src) && !repTieneMainAST(src) {
			loc := reClausulaMain.FindStringSubmatchIndex(src)
			if loc != nil {
				fuente := src[:loc[3]] + "solucion" + src[loc[1]:]
				return repFuente{fuente: fuente, paquete: "solucion", sinMain: true}
			}
		}
		return repFuente{fuente: src, paquete: m[1]}
	}
	paquete := "solucion"
	if reFuncMain.MatchString(src) {
		paquete = "main"
	}
	cand := "package " + paquete + "; " + src
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", cand, 0); err == nil || repEmpiezaConDeclaracion(src) {
		return repFuente{fuente: cand, paquete: paquete}
	}
	if out, err := analisis.Normalizar(src, ""); err == nil {
		r := repFuente{fuente: out, paquete: "main"}
		antes := map[string]bool{}
		for _, m := range reImport.FindAllStringSubmatch(src, -1) {
			antes[m[1]] = true
		}
		if f, err := parser.ParseFile(token.NewFileSet(), "x.go", out, parser.ImportsOnly); err == nil {
			for _, im := range f.Imports {
				ruta := strings.Trim(im.Path.Value, "\"")
				if !antes[ruta] {
					r.importes = append(r.importes, ruta)
				}
			}
		}
		return r
	}
	return repFuente{fuente: "package main; func main() {\n" + src + "}\n", paquete: "main", desfase: 1}
}

// repTieneMainAST: the file parses and declares func main (catches spellings the regexp misses).
// A file that does not parse counts as having one, so its own errors are reported as they are.
func repTieneMainAST(src string) bool {
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		return true
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "main" {
			return true
		}
	}
	return false
}

// repEmpiezaConDeclaracion: the snippet starts (after comments) with a top-level declaration keyword.
func repEmpiezaConDeclaracion(src string) bool {
	for _, l := range strings.Split(src, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "//") {
			continue
		}
		if strings.HasPrefix(l, "func(") {
			return false // a function literal is a statement
		}
		for _, p := range []string{"func ", "type ", "import ", "import(", "var ", "const "} {
			if strings.HasPrefix(l, p) {
				return true
			}
		}
		return false
	}
	return false
}

// linea moves an error's line back to the user's numbering.
func (s repFuente) linea(e nucleo.ErrorGo) nucleo.ErrorGo {
	e.Linea = s.lineaUsuario(e.Linea)
	return e
}

func (s repFuente) lineaUsuario(l int) int {
	if l > s.desfase {
		return l - s.desfase
	}
	return l
}

// repFormatear gofmts a source when it parses (the diff base and the shown code then look alike).
func repFormatear(src string) string {
	if b, err := format.Source([]byte(src)); err == nil {
		return string(b)
	}
	return src
}

// repConFirma returns a copy of the question with the code's signature (follow-ups such as "pruébala con
// [3,3]" or "está mal" then read values with the right types).
func repConFirma(q *nucleo.Pregunta, f *nucleo.Firma) *nucleo.Pregunta {
	if q == nil {
		return &nucleo.Pregunta{Firma: f}
	}
	nq := *q
	if f != nil {
		g := *f
		nq.Firma = &g
	}
	return &nq
}

// repRecordarEspec keeps the spec's typed examples and concepts in the turn's question, so "está mal: con
// [2] debe dar 0" adds to them with the same types and stores the counterexample under the same key.
func repRecordarEspec(k *tarea, e *repEspec) {
	k.q.Ejemplos = append([]nucleo.Caso(nil), e.usuario...)
	k.q.Conceptos = append([]string(nil), e.conceptos...)
}

// ---- choosing the function ----

// repElegir picks the target function: the one named in the text, else the only one besides main, else
// the only one main calls, else it asks. false: there is no function besides main and init.
func (c *Cerebro) repElegir(k *tarea, fuente string, inf analisis.Informe) (analisis.InfoFunc, bool) {
	var fs []analisis.InfoFunc
	for _, f := range inf.Funciones {
		if f.Nombre == "main" || f.Nombre == "init" {
			continue
		}
		fs = append(fs, f)
	}
	if len(fs) == 0 {
		return analisis.InfoFunc{}, false
	}
	if len(fs) == 1 {
		return fs[0], true
	}
	texto := k.q.Resto
	if strings.TrimSpace(texto) == "" {
		texto = k.p.Texto
	}
	for _, sensible := range []bool{true, false} {
		var hits []analisis.InfoFunc
		for _, f := range fs {
			nombre := f.Nombre
			if i := strings.LastIndexByte(nombre, '.'); i >= 0 {
				nombre = nombre[i+1:]
			}
			patron := `\b` + regexp.QuoteMeta(nombre) + `\b`
			if !sensible {
				patron = `(?i)` + patron
			}
			if regexp.MustCompile(patron).MatchString(texto) {
				hits = append(hits, f)
			}
		}
		if len(hits) == 1 {
			return hits[0], true
		}
	}
	if llamadas := repLlamadasDeMain(fuente, fs); len(llamadas) == 1 {
		return llamadas[0], true
	}
	var nombres []string
	for _, f := range fs {
		nombres = append(nombres, f.Nombre)
	}
	if resp, ok := c.preguntar(k, "¿Qué función quieres que mire?", nombres); ok {
		for _, f := range fs {
			if strings.EqualFold(strings.TrimSpace(resp), f.Nombre) {
				return f, true
			}
		}
	}
	for _, f := range fs {
		if f.Firma != nil && f.Firma.Probable() {
			return f, true
		}
	}
	return fs[0], true
}

// repLlamadasDeMain: the listed functions that func main calls.
func repLlamadasDeMain(fuente string, fs []analisis.InfoFunc) []analisis.InfoFunc {
	archivo, err := parser.ParseFile(token.NewFileSet(), "x.go", fuente, 0)
	if err != nil {
		return nil
	}
	porNombre := map[string]analisis.InfoFunc{}
	for _, f := range fs {
		porNombre[f.Nombre] = f
		if i := strings.LastIndexByte(f.Nombre, '.'); i >= 0 {
			porNombre[f.Nombre[i+1:]] = f
		}
	}
	visto := map[string]bool{}
	var out []analisis.InfoFunc
	for _, d := range archivo.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name.Name != "main" || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			ll, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			nombre := ""
			switch fun := ll.Fun.(type) {
			case *ast.Ident:
				nombre = fun.Name
			case *ast.SelectorExpr:
				nombre = fun.Sel.Name
			}
			if f, ok := porNombre[nombre]; ok && !visto[f.Nombre] {
				visto[f.Nombre] = true
				out = append(out, f)
			}
			return true
		})
	}
	return out
}

// ---- the spec ----

// repEspec is what the code must do: the user's cases, the reported wrong outputs, concept properties and
// a stored reference function.
type repEspec struct {
	f          nucleo.Firma
	conceptos  []string
	usuario    []nucleo.Caso      // EspUsuario: examples, "debería dar", counterexamples, answers
	mencion    []nucleo.Caso      // inputs mentioned without an expected value (reference value if any)
	props      []nucleo.Propiedad // concept properties
	negativas  []repNegativa      // "con [3,1,2] da 1": it must not give 1 there
	oraculo    func([]nucleo.Valor) ([]nucleo.Valor, error)
	referencia string      // name of the stored function used as the reference
	ref        *repCajaRef // the reference built in the sandbox (a stored function without DSL), or nil
	extra      []nucleo.Caso
}

// cerrar frees the reference's binary, if any.
func (e *repEspec) cerrar() {
	if e.ref != nil {
		e.ref.cerrar()
	}
}

type repNegativa struct {
	entradas []nucleo.Valor
	no       []nucleo.Valor
	prop     nucleo.Propiedad
	texto    string // "Con [3,1,2] ya no da 1"
}

// repEspec builds the spec of function nombre with signature f (§4.14.2 reparar step 4).
func (c *Cerebro) repEspec(k *tarea, f nucleo.Firma, nombre string) *repEspec {
	n := k.raiz.Sub(nucleo.PasoEntender, "Qué debe hacer «%s»", nombre)
	e := &repEspec{f: f}
	e.conceptos = repConceptos(k, nombre)
	visto := map[string]bool{}
	agregar := func(cs ...nucleo.Caso) {
		for _, cs := range cs {
			if len(cs.Entradas) != len(f.Entradas()) || len(cs.Esperado) != len(f.Res) {
				continue
			}
			clave := repClave(cs.Entradas)
			if visto[clave] {
				continue
			}
			visto[clave] = true
			cs.Expectativa = nucleo.EspUsuario
			if cs.Origen == "" {
				cs.Origen = nucleo.OrigenUsuario
			}
			e.usuario = append(e.usuario, cs)
		}
	}
	if k.q.Firma != nil && k.q.Firma.Forma() == f.Forma() && len(k.q.Ejemplos) > 0 {
		agregar(k.q.Ejemplos...)
	} else if len(k.q.ParesTexto) > 0 {
		if cs, _, err := lengua.TiparEjemplos(k.q.ParesTexto, &f); err == nil {
			agregar(cs...)
		} else {
			for _, par := range k.q.ParesTexto {
				if cs, _, err := lengua.TiparEjemplos([][2]string{par}, &f); err == nil {
					agregar(cs...)
				}
			}
		}
	}
	agregar(k.ejemplosExtra...)
	texto := k.q.Resto
	if strings.TrimSpace(texto) == "" {
		texto = k.p.Texto
	}
	fallos := lengua.ExtraerFallos(texto)
	for _, fl := range fallos {
		if fl.Esperado == "" {
			continue
		}
		if cs, _, err := lengua.TiparEjemplos([][2]string{{fl.Entrada, fl.Esperado}}, &f); err == nil {
			agregar(cs...)
		}
	}
	agregar(c.contraejemplos(e.conceptos, f)...)

	e.oraculo, e.referencia, e.ref = c.repOraculo(k, f, e.conceptos)
	for _, fl := range fallos {
		if fl.Obtenido == "" {
			continue
		}
		cs, _, err := lengua.TiparEjemplos([][2]string{{fl.Entrada, fl.Obtenido}}, &f)
		if err != nil || len(cs) == 0 || visto[repClave(cs[0].Entradas)] {
			continue
		}
		visto[repClave(cs[0].Entradas)] = true
		caso := nucleo.Caso{Entradas: cs[0].Entradas, Origen: nucleo.OrigenUsuario, Nota: "tu caso"}
		if e.oraculo != nil {
			caso = pruebas.ConOraculo([]nucleo.Caso{caso}, e.oraculo, nucleo.EspReferencia, e.referencia)[0]
		}
		e.mencion = append(e.mencion, caso)
		if len(f.Res) == 1 && !caso.ConEsperado() {
			e.negativas = append(e.negativas, repNegativaDe(f, cs[0], len(e.negativas)))
		}
	}
	e.props = pruebas.Propiedades(e.conceptos, f)

	switch {
	case len(e.usuario) > 0:
		n.Detalle("Tus ejemplos: %d", len(e.usuario))
	}
	for _, ng := range e.negativas {
		n.Detalle("%s", ng.texto)
	}
	if len(e.props) > 0 {
		n.Detalle("Propiedades de «%s»: %d", strings.Join(e.conceptos, ", "), len(e.props))
	}
	if e.oraculo != nil {
		n.Detalle("Comparo con «%s», que ya conozco", e.referencia)
	}
	if e.vacia() {
		n.Info("No sé qué debe dar: solo miro que no se rompa")
	} else {
		n.Info("")
	}
	return e
}

// vacia: nothing says what the function must give (only no-panic and determinism can be checked).
func (e *repEspec) vacia() bool {
	return len(e.usuario) == 0 && len(e.negativas) == 0 && len(e.props) == 0 && e.oraculo == nil && !repHayEsperado(e.mencion)
}

func repHayEsperado(cs []nucleo.Caso) bool {
	for _, c := range cs {
		if c.ConEsperado() {
			return true
		}
	}
	return false
}

// todasProps: the concept properties plus the "it must not give X here" ones.
func (e *repEspec) todasProps() []nucleo.Propiedad {
	out := append([]nucleo.Propiedad(nil), e.props...)
	for _, n := range e.negativas {
		out = append(out, n.prop)
	}
	return out
}

// casos: the user's cases, the mentioned inputs, extra cases, and the generated ones (with the
// reference's output when there is a reference).
func (e *repEspec) casos(c *Cerebro, k *tarea) []nucleo.Caso {
	var out []nucleo.Caso
	out = append(out, e.usuario...)
	out = append(out, e.mencion...)
	out = append(out, e.extra...)
	gen := c.generarCasos(k, e.f, out, 0, 0)
	if e.ref != nil {
		ins := make([][]nucleo.Valor, len(gen))
		for i, cs := range gen {
			ins[i] = cs.Entradas
		}
		e.ref.correr(ins) // one sandbox run for all the generated inputs
	}
	if e.oraculo != nil {
		gen = pruebas.ConOraculo(gen, e.oraculo, nucleo.EspReferencia, e.referencia)
	}
	visto := map[string]bool{}
	for _, cs := range out {
		visto[repClave(cs.Entradas)] = true
	}
	for _, cs := range gen {
		if !visto[repClave(cs.Entradas)] {
			visto[repClave(cs.Entradas)] = true
			out = append(out, cs)
		}
	}
	return out
}

// comprobaciones are the badges of a run: pruebas.Comprobaciones (the concept properties only count when
// there are some) plus one badge per "it must not give X" check.
func (e *repEspec) comprobaciones(k *tarea, casos []nucleo.Caso, rs []nucleo.ResultadoCaso) []nucleo.Comprobacion {
	var props []nucleo.Propiedad
	if len(e.props) > 0 {
		props = e.todasProps()
	}
	out := pruebas.Comprobaciones(casos, rs, props, k.semilla)
	for _, ng := range e.negativas {
		cp := nucleo.Comprobacion{Nivel: nucleo.NivelPropiedades, Pasan: 0, Total: 1, Texto: ng.texto}
		for i, cs := range casos {
			if i < len(rs) && repClave(cs.Entradas) == repClave(ng.entradas) {
				r := rs[i]
				if r.Ejecutado && r.Panico == "" && !r.Caida && !r.Agotado && !r.SinCombustible && !repIgualValores(r.Obtenido, ng.no) {
					cp.Pasan = 1
				}
				break
			}
		}
		if cp.Pasan < cp.Total {
			cp.Nivel = nucleo.NivelFallo
		}
		out = append(out, cp)
	}
	return out
}

func repNegativaDe(f nucleo.Firma, cs nucleo.Caso, i int) repNegativa {
	ent := f.Entradas()
	var req []string
	for j, v := range cs.Entradas {
		req = append(req, fmt.Sprintf("nyx__Igual(e%d, %s)", j, nucleo.FormatoGo(v, ent[j])))
	}
	no := nucleo.FormatoGo(cs.Esperado[0], f.Res[0])
	texto := "Con " + repEntradas(f, cs.Entradas) + " ya no da " + repValores(cs.Esperado, f.Res)
	return repNegativa{
		entradas: cs.Entradas,
		no:       cs.Esperado,
		texto:    texto,
		prop: nucleo.Propiedad{
			Nombre:      fmt.Sprintf("no_da_%d", i+1),
			Explicacion: "con " + repEntradas(f, cs.Entradas) + " no debe dar " + repValores(cs.Esperado, f.Res),
			Expr:        "!nyx__Igual(r0, " + no + ")",
			Requiere:    strings.Join(req, " && "),
		},
	}
}

// repConceptos: the request's concepts, or those of the function name's words (Maximo → maximo).
func repConceptos(k *tarea, nombre string) []string {
	var out []string
	visto := map[string]bool{}
	agregar := func(cs ...string) {
		for _, c := range cs {
			if c != "" && !visto[c] && nucleo.EsConcepto(c) {
				visto[c] = true
				out = append(out, c)
			}
		}
	}
	agregar(k.q.Conceptos...)
	if len(repAcciones(out)) > 0 {
		return out
	}
	if i := strings.LastIndexByte(nombre, '.'); i >= 0 {
		nombre = nombre[i+1:]
	}
	palabras := repPartirNombre(nombre)
	for _, tk := range lengua.Lematizar(strings.Join(palabras, " ")) {
		if tk.Clase != "palabra" {
			continue
		}
		if cpt, ok := k.lex.Concepto(tk.Lema); ok {
			agregar(cpt)
		} else {
			agregar(tk.Lema)
		}
	}
	return out
}

// repPartirNombre splits SumaPares / suma_pares into words.
func repPartirNombre(s string) []string {
	var out []string
	var w []rune
	for i, r := range []rune(s) {
		if r == '_' || r >= '0' && r <= '9' {
			if len(w) > 0 {
				out = append(out, strings.ToLower(string(w)))
				w = nil
			}
			continue
		}
		if i > 0 && r >= 'A' && r <= 'Z' && len(w) > 0 {
			out = append(out, strings.ToLower(string(w)))
			w = nil
		}
		w = append(w, r)
	}
	if len(w) > 0 {
		out = append(out, strings.ToLower(string(w)))
	}
	return out
}

var repObjetos = map[string]bool{"numero": true, "numeros": true, "decimal": true, "lista": true, "texto": true,
	"palabra": true, "palabras": true, "letras": true, "frase": true, "matriz": true, "mapa": true}

// repAcciones: the concepts that say what is done (not the object types).
func repAcciones(cs []string) []string {
	var out []string
	for _, c := range cs {
		if !repObjetos[c] {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// repOraculo looks for a stored function with the same action concepts and the same types to use as the
// EspReferencia oracle (§4.14.2 reparar step 4). A synthesized one has its DSL program evaluated in
// process (inputs where it is undefined are left out); any other one (from a recipe, a repair, the web)
// is built in the sandbox under PermisoAuto and run on the inputs (the caller closes it with
// repEspec.cerrar). Only functions verified beyond "no se rompe" are trusted as a reference.
func (c *Cerebro) repOraculo(k *tarea, f nucleo.Firma, conceptos []string) (func([]nucleo.Valor) ([]nucleo.Valor, error), string, *repCajaRef) {
	acc := repAcciones(conceptos)
	if len(acc) == 0 || len(f.Res) != 1 || f.Receptor != nil {
		return nil, "", nil
	}
	for _, h := range c.mem.BuscarFunciones(conceptos, lemasDe(k.q), f.Forma(), 5) {
		g := h.F
		if strings.Join(repAcciones(g.Conceptos), ",") != strings.Join(acc, ",") {
			continue
		}
		if g.DSL == "" {
			if ref := c.repCajaReferencia(k, g); ref != nil {
				return ref.oraculo, g.Nombre, ref
			}
			continue
		}
		e, err := sintesis.Parse(g.DSL, c.registro(), g.Firma)
		if err != nil {
			continue
		}
		return func(in []nucleo.Valor) ([]nucleo.Valor, error) {
			copia := make([]nucleo.Valor, len(in))
			for i, v := range in {
				copia[i] = nucleo.Copiar(v)
			}
			v, err := sintesis.Evaluar(e, copia)
			if err != nil {
				return nil, err
			}
			return []nucleo.Valor{v}, nil
		}, g.Nombre, nil
	}
	return nil, "", nil
}

// repCajaRef is a stored function built in the sandbox and used as the reference oracle. Its outputs are
// remembered by input, so the generated cases cost one run. reparar.Logica asks for inputs one at a time
// (each one a sandbox run of ~10 ms), so at most maxSueltasRef such runs are made; past that an unknown
// input is "undefined" and its case keeps only the other checks.
type repCajaRef struct {
	ctx      context.Context
	b        nucleo.Binario
	mu       sync.Mutex
	salidas  map[string][]nucleo.Valor // input key → output (absent: it fails or never ends there)
	conocido map[string]bool
	sueltas  int // single-input runs made
}

const maxSueltasRef = 64

var errReferenciaIndefinida = fmt.Errorf("la referencia no da un resultado para esa entrada")

// repCajaReferencia builds g in the sandbox, or returns nil when it cannot be used as a reference.
func (c *Cerebro) repCajaReferencia(k *tarea, g nucleo.FuncionAprendida) *repCajaRef {
	switch g.Nivel {
	case nucleo.NivelTusEjemplos, nucleo.NivelEntendido, nucleo.NivelPropiedades:
	default:
		return nil
	}
	if c.ej == nil || strings.TrimSpace(g.Codigo) == "" || !g.Firma.Probable() || len(c.ej.Revisar(g.Codigo, nucleo.PermisoAuto)) > 0 {
		return nil
	}
	b, _, err := c.ej.Preparar(k.ctx, nucleo.Preparacion{Variantes: []string{g.Codigo}, Firma: g.Firma, Instr: nucleo.InstrNormal, Permiso: nucleo.PermisoAuto})
	if err != nil {
		return nil
	}
	return &repCajaRef{ctx: k.ctx, b: b, salidas: map[string][]nucleo.Valor{}, conocido: map[string]bool{}}
}

// correr runs the reference on the inputs it has not seen yet, all in one sandbox run.
func (r *repCajaRef) correr(ins [][]nucleo.Valor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.correrSinCerrojo(ins)
}

func (r *repCajaRef) correrSinCerrojo(ins [][]nucleo.Valor) {
	if r.b == nil {
		return
	}
	var casos []nucleo.Caso
	var claves []string
	nuevo := map[string]bool{}
	for _, in := range ins {
		cl := repClave(in)
		if r.conocido[cl] || nuevo[cl] {
			continue
		}
		nuevo[cl] = true
		copia := make([]nucleo.Valor, len(in))
		for i, v := range in {
			copia[i] = nucleo.Copiar(v)
		}
		casos = append(casos, nucleo.Caso{Entradas: copia})
		claves = append(claves, cl)
	}
	if len(casos) == 0 {
		return
	}
	ctx, cancelar := context.WithTimeout(r.ctx, 20*time.Second)
	defer cancelar()
	var rs []nucleo.ResultadoCaso
	if rr, err := r.b.Probar(ctx, casos, nucleo.OpcionesProbar{}); err == nil && len(rr) > 0 {
		rs = repAlinear(rr[0], len(casos))
	}
	for i, cl := range claves {
		r.conocido[cl] = true
		if i < len(rs) {
			x := rs[i]
			if x.Ejecutado && x.Panico == "" && !x.Caida && !x.Agotado && !x.SinCombustible && !x.NoDeterminista && len(x.Obtenido) > 0 {
				r.salidas[cl] = x.Obtenido
			}
		}
	}
}

// oraculo is the reference's output for in (an error where it is undefined).
func (r *repCajaRef) oraculo(in []nucleo.Valor) ([]nucleo.Valor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cl := repClave(in)
	if !r.conocido[cl] && r.sueltas < maxSueltasRef {
		r.sueltas++
		r.correrSinCerrojo([][]nucleo.Valor{in})
	}
	v, ok := r.salidas[cl]
	if !ok {
		return nil, errReferenciaIndefinida
	}
	out := make([]nucleo.Valor, len(v))
	for i, x := range v {
		out[i] = nucleo.Copiar(x)
	}
	return out, nil
}

func (r *repCajaRef) cerrar() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.b != nil {
		r.b.Cerrar()
		r.b = nil
	}
}

// ---- running ----

// repCorrerCasos builds fuente with the spec's properties and runs the cases. The caller closes the binary.
// permiso is PermisoUsuario only after the user confirmed the risks (ejcPermiso).
func (c *Cerebro) repCorrerCasos(k *tarea, nodo *nucleo.Nodo, titulo, fuente string, e *repEspec, casos []nucleo.Caso, permiso nucleo.Permiso) (nucleo.Binario, []nucleo.ResultadoCaso, error) {
	n := nodo.Sub(nucleo.PasoPrueba, "%s", titulo)
	b, _, err := c.preparar(k, n, nucleo.Preparacion{Variantes: []string{fuente}, Firma: e.f, Props: e.todasProps(), Instr: nucleo.InstrNormal, Permiso: permiso})
	if err != nil {
		n.Mal("No lo pude probar")
		return nil, nil, err
	}
	rs, err := c.probarCasos(k, n, b, casos, nucleo.OpcionesProbar{})
	if err != nil || len(rs) == 0 {
		b.Cerrar()
		n.Mal("No lo pude probar")
		if err == nil {
			err = fmt.Errorf("el arenero no devolvió resultados")
		}
		return nil, nil, err
	}
	out := repAlinear(rs[0], len(casos))
	if f := repFallos(out, len(casos)); len(f) == 0 {
		n.Bien("")
	} else {
		n.Mal("Fallan %d de %d casos", len(f), len(casos))
	}
	return b, out, nil
}

// repAlinear returns the results indexed by case.
func repAlinear(rs []nucleo.ResultadoCaso, n int) []nucleo.ResultadoCaso {
	out := make([]nucleo.ResultadoCaso, n)
	puesto := make([]bool, n)
	for i, r := range rs {
		j := r.Caso
		if j < 0 || j >= n || puesto[j] {
			j = i
		}
		if j < n && !puesto[j] {
			out[j], puesto[j] = r, true
		}
	}
	return out
}

func repFallos(rs []nucleo.ResultadoCaso, n int) []int {
	var out []int
	for i := 0; i < n; i++ {
		if i >= len(rs) || !rs[i].Ejecutado || !rs[i].OK {
			out = append(out, i)
		}
	}
	return out
}

// repLogica repairs the failing code: panic rules first, then Logica on the result. With two plausible
// fixes that disagree it asks "¿esperas 3 o 1?" and the answer becomes a case. It returns the new source,
// its changes, whether every case passes, the (possibly extended) cases and notes for the answer.
func (c *Cerebro) repLogica(k *tarea, e *repEspec, fuente string, casos []nucleo.Caso) (string, []reparar.Cambio, bool, []nucleo.Caso, []string) {
	rep := c.reparador(k)
	if d := k.restante() - 4*time.Second; d < 25*time.Second {
		rep.Limite.TLogica = max(d, 3*time.Second)
	}
	n := k.raiz.Sub(nucleo.PasoArreglo, "Busco el fallo y lo arreglo")
	actual := fuente
	var cambios []reparar.Cambio
	var notas []string
	if ps, err := rep.Panicos(k.ctx, actual, e.f, casos, n); err == nil && len(ps) > 0 {
		actual = ps[0].Fuente
		cambios = append(cambios, ps[0].Cambios...)
		if ps[0].Pasan == ps[0].Total && len(e.negativas) == 0 && len(e.props) == 0 {
			n.Bien("Arreglado: ya no se rompe")
			return actual, cambios, true, casos, notas
		}
	}
	res, err := rep.Logica(k.ctx, actual, e.f, casos, e.todasProps(), e.oraculo, n)
	if err != nil {
		n.Mal("No pude buscar el arreglo: %s", mensajeError(err))
		return actual, cambios, false, casos, notas
	}
	if len(res.Parches) == 0 {
		if res.Contraejemplo == nil && strings.Contains(res.Motivo, "ya pasa") {
			n.Bien("Arreglado: ya pasa las %d pruebas", len(casos))
			return actual, cambios, true, casos, notas
		}
		if res.Mejor != nil {
			actual = res.Mejor.Fuente
			cambios = append(cambios, res.Mejor.Cambios...)
		}
		if res.Motivo != "" {
			notas = append(notas, mayusculaInicial(res.Motivo)+".")
		}
		n.Mal("No encontré un arreglo que pase todas las pruebas")
		return actual, cambios, false, casos, notas
	}
	elegido := 0
	if amb := res.Ambiguo; amb != nil && len(amb.Parches) >= 2 {
		j, caso, ok := c.repDesempatar(k, e, amb)
		if ok {
			casos = append(casos, caso)
			e.usuario = append(e.usuario, caso)
			if j >= 0 && j < len(res.Parches) {
				elegido = j
			} else if res2, err := rep.Logica(k.ctx, actual, e.f, casos, e.todasProps(), e.oraculo, n); err == nil && len(res2.Parches) > 0 {
				res = res2
				elegido = 0
			} else {
				n.Mal("Ningún arreglo da lo que esperas")
				return actual, cambios, false, casos, notas
			}
		} else {
			otro := amb.Parches[1]
			if amb.Parches[0] != elegido {
				otro = amb.Parches[0]
			}
			notas = append(notas, fmt.Sprintf("Ojo: hay otro arreglo que da distinto con %s (%s en vez de %s). Me quedo con el primero; si no es eso, dime qué debe dar.",
				repEntradas(e.f, amb.Entrada), repSalida(e.f, repSalidaDe(amb, otro)), repSalida(e.f, repSalidaDe(amb, elegido))))
		}
	}
	p := res.Parches[elegido]
	n.Bien("Arreglado: pasa los %d casos", p.Total)
	return p.Fuente, append(cambios, p.Cambios...), true, casos, notas
}

func repSalidaDe(amb *reparar.Ambiguedad, parche int) []nucleo.Valor {
	for i, p := range amb.Parches {
		if p == parche && i < len(amb.Salidas) {
			return amb.Salidas[i]
		}
	}
	return nil
}

// repDesempatar asks which output the user expects. It returns the index of the matching patch (-1 when
// none matches) and the answer as a new case.
func (c *Cerebro) repDesempatar(k *tarea, e *repEspec, amb *reparar.Ambiguedad) (int, nucleo.Caso, bool) {
	var opciones []string
	visto := map[string]bool{}
	for i := range amb.Parches {
		if i >= len(amb.Salidas) || amb.Salidas[i] == nil {
			continue
		}
		s := repValores(amb.Salidas[i], e.f.Res)
		if !visto[s] {
			visto[s] = true
			opciones = append(opciones, s)
		}
	}
	if len(opciones) < 2 {
		return -1, nucleo.Caso{}, false
	}
	texto := fmt.Sprintf("Hay %d arreglos que dan distinto con %s: ¿esperas %s?", len(opciones), repEntradas(e.f, amb.Entrada), strings.Join(opciones, " o "))
	var valores []nucleo.Valor
	if len(e.f.Res) == 1 {
		v, _, ok := c.preguntarValor(k, texto, opciones, e.f.Res[0])
		if !ok {
			return -1, nucleo.Caso{}, false
		}
		valores = []nucleo.Valor{v}
	} else {
		resp, ok := c.preguntar(k, texto, opciones)
		if !ok {
			return -1, nucleo.Caso{}, false
		}
		for i := range amb.Parches {
			if i < len(amb.Salidas) && amb.Salidas[i] != nil && repValores(amb.Salidas[i], e.f.Res) == strings.TrimSpace(resp) {
				valores = amb.Salidas[i]
				break
			}
		}
		if valores == nil {
			return -1, nucleo.Caso{}, false
		}
	}
	caso := nucleo.Caso{Entradas: amb.Entrada, Esperado: valores, Expectativa: nucleo.EspUsuario, Origen: nucleo.OrigenUsuario, Nota: "tu respuesta"}
	for i, p := range amb.Parches {
		if i < len(amb.Salidas) && repIgualValores(amb.Salidas[i], valores) {
			return p, caso, true
		}
	}
	return -1, caso, true
}

// repContraejemplo describes the first failing case of the original code (the user's first; a panic
// is minimized).
func (c *Cerebro) repContraejemplo(k *tarea, b nucleo.Binario, e *repEspec, casos []nucleo.Caso, rs []nucleo.ResultadoCaso, fallos []int) string {
	caso, r, ok := c.repContraejemploCaso(k, b, e, casos, rs, fallos)
	if !ok {
		return ""
	}
	return "Antes " + repFrase(e, caso, r, true) + "."
}

// repContraejemploCaso picks the failing case to show and its result.
func (c *Cerebro) repContraejemploCaso(k *tarea, b nucleo.Binario, e *repEspec, casos []nucleo.Caso, rs []nucleo.ResultadoCaso, fallos []int) (nucleo.Caso, nucleo.ResultadoCaso, bool) {
	if len(fallos) == 0 {
		return nucleo.Caso{}, nucleo.ResultadoCaso{}, false
	}
	i := fallos[0]
	for _, j := range fallos {
		if casos[j].Expectativa == nucleo.EspUsuario {
			i = j
			break
		}
	}
	caso, r := casos[i], rs[i]
	if r.Panico != "" && !casos[i].ConEsperado() {
		ctx, cancelar := context.WithTimeout(k.ctx, 3*time.Second)
		reducido := pruebas.Minimizar(ctx, caso, e.f, func(cs []nucleo.Caso) []bool {
			out := make([]bool, len(cs))
			rr, err := b.Probar(ctx, cs, nucleo.OpcionesProbar{})
			if err != nil || len(rr) == 0 {
				return out
			}
			for j, x := range repAlinear(rr[0], len(cs)) {
				out[j] = x.Panico != "" || x.Caida
			}
			return out
		}, 30)
		cancelar()
		if rr, err := b.Probar(k.ctx, []nucleo.Caso{reducido}, nucleo.OpcionesProbar{}); err == nil && len(rr) > 0 && len(rr[0]) > 0 && rr[0][0].Panico != "" {
			caso, r = reducido, rr[0][0]
		}
	}
	return caso, r, true
}

// repFrase describes a failing case: "falla con [-1]: esperaba 0, da -1" (pasado: "fallaba … daba").
func repFrase(e *repEspec, caso nucleo.Caso, r nucleo.ResultadoCaso, pasado bool) string {
	falla, da, rompe := "falla", "da", "se rompe"
	if pasado {
		falla, da, rompe = "fallaba", "daba", "se rompía"
	}
	ent := repEntradas(e.f, caso.Entradas)
	switch {
	case !r.Ejecutado:
		return falla + " con " + ent + ": no llegó a ejecutarse"
	case r.SinCombustible || r.Agotado:
		return falla + " con " + ent + ": no termina (parece un bucle sin fin)"
	case r.Panico != "" || r.Caida:
		return falla + " con " + ent + ": " + rompe + " (" + repMinuscula(strings.TrimSuffix(reparar.TraducirPanico(primeraLinea(r.Panico)), ".")) + ")"
	case r.NoDeterminista:
		return falla + " con " + ent + ": cada vez " + da + " una cosa distinta"
	case caso.ConEsperado() && !repIgualValores(r.Obtenido, caso.Esperado):
		esperaba := "esperaba "
		if caso.Expectativa == nucleo.EspReferencia && caso.Nota != "" {
			esperaba = "según " + caso.Nota + " debía dar "
		}
		return falla + " con " + ent + ": " + esperaba + repValores(caso.Esperado, e.f.Res) + ", " + da + " " + repValores(r.Obtenido, e.f.Res)
	case r.PropFallida != "":
		por := r.PropFallida
		for _, p := range e.todasProps() {
			if p.Nombre == r.PropFallida {
				por = p.Explicacion
			}
		}
		return falla + " con " + ent + ": " + da + " " + repValores(r.Obtenido, e.f.Res) + " y no cumple «" + por + "»"
	}
	return falla + " con " + ent
}

// ---- answers ----

// repSinFallos: the code compiles and passes everything it was tested with.
func (c *Cerebro) repSinFallos(k *tarea, e *repEspec, base, actual string, casos []nucleo.Caso, rs []nucleo.ResultadoCaso, errores []string, cambios []reparar.Cambio, suposiciones, notas []string) nucleo.Respuesta {
	comps := e.comprobaciones(k, casos, rs)
	r := nucleo.Respuesta{
		Codigo:         actual,
		Diff:           reparar.Diff(base, actual),
		Comprobaciones: comps,
		Nivel:          pruebas.NivelGlobal(comps),
		Tablas:         []nucleo.Tabla{pruebas.Tabla(e.f, casos, rs, 0)},
		Metodo:         "reparacion",
		Exito:          true,
	}
	switch {
	case len(cambios) > 0:
		r.Texto = "Lo he arreglado: " + repResumenCambios(cambios)
	case e.vacia():
		r.Texto = fmt.Sprintf("Compila y no encontré fallos en %d pruebas, pero no sé qué debe dar. Dame un ejemplo (entrada -> salida) y lo compruebo.", len(casos))
	default:
		r.Texto = fmt.Sprintf("No encontré nada que arreglar: compila y pasa las %d pruebas.", len(casos))
	}
	if len(cambios) > 0 && e.vacia() {
		notas = append(notas, fmt.Sprintf("No encontré fallos en %d pruebas, pero no sé qué debe dar. Dame un ejemplo (entrada -> salida) y lo compruebo.", len(casos)))
	}
	r.Parrafos = append(repParrafosCambios(errores, cambios, suposiciones), notas...)
	r.Justificacion = repJustificacion(k, e, casos, comps, cambios)
	return r
}

// repRespuestaCompila: it compiles but could not be tested (said plainly, without badges).
func repRespuestaCompila(base, actual string, errores []string, cambios []reparar.Cambio, suposiciones []string) nucleo.Respuesta {
	r := nucleo.Respuesta{
		Codigo:         actual,
		Diff:           reparar.Diff(base, actual),
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "reparacion",
		Exito:          true,
		Parrafos:       repParrafosCambios(errores, cambios, suposiciones),
	}
	if len(cambios) > 0 {
		r.Texto = "Ahora compila: " + repResumenCambios(cambios)
	} else {
		r.Texto = "Compila: no encontré errores de compilación."
	}
	return r
}

// repSinCaja: without a sandbox only the in-process type check backs the fix.
func repSinCaja(k *tarea, base, actual string, errores []string, cambios []reparar.Cambio, suposiciones []string) nucleo.Respuesta {
	k.raiz.Sub(nucleo.PasoNota, "No tengo caja segura: no puedo ejecutar las pruebas").Info("")
	r := repRespuestaCompila(base, actual, errores, cambios, suposiciones)
	r.Parrafos = append(r.Parrafos, "No he podido ejecutarlo para probarlo: no tengo caja segura (falta Go).")
	return r
}

// repPrograma: a program without functions to test: it is run once (stdin from the request or "con
// entrada 5" in its text) when the safety check allows it, or after the user confirmed the risks.
func (c *Cerebro) repPrograma(k *tarea, base, actual string, errores []string, cambios []reparar.Cambio, suposiciones, notas []string) nucleo.Respuesta {
	r := repRespuestaCompila(base, actual, errores, cambios, suposiciones)
	r.Parrafos = append(r.Parrafos, notas...)
	if !reFuncMain.MatchString(actual) {
		return r
	}
	permiso := nucleo.PermisoAuto
	if v := c.ej.Revisar(actual, nucleo.PermisoAuto); len(v) > 0 {
		pr, ok := c.ejcPermiso(k, actual, v, "ejecutarlo")
		if !ok {
			r.Parrafos = append(append(r.Parrafos, pr.Texto), pr.Parrafos...)
			r.Riesgos = pr.Riesgos
			return r
		}
		permiso = nucleo.PermisoUsuario
	}
	caso := nucleo.CasoPrograma{Entrada: k.p.Entrada, Args: k.p.Args}
	if caso.Entrada == "" {
		if pedida := ejcEntradaExplicita(k); pedida != "" {
			caso.Entrada = pedida + "\n"
		}
	}
	ej, err := c.repCorrerPrograma(k, k.raiz, actual, caso, permiso)
	if err != nil {
		r.Parrafos = append(r.Parrafos, "No pude ejecutarlo: "+mensajeError(err)+".")
		return r
	}
	cp, frase := repResultadoPrograma(ej, caso.Entrada)
	r.Parrafos = append(r.Parrafos, frase)
	if cp != nil {
		r.Comprobaciones = []nucleo.Comprobacion{*cp}
		r.Nivel = cp.Nivel
		return r
	}
	falla := "Al ejecutarlo falla"
	if caso.Entrada == "" {
		falla = "Al ejecutarlo sin entrada falla"
	}
	r.Comprobaciones = []nucleo.Comprobacion{{Nivel: nucleo.NivelFallo, Pasan: 0, Total: 1, Texto: falla, Comando: ej.Comando}}
	r.Nivel = nucleo.NivelFallo
	r.Exito = false
	switch {
	case len(cambios) > 0:
		r.Texto = "Ahora compila, pero al ejecutarlo falla."
	case caso.Entrada == "":
		r.Texto = "Compila, pero al ejecutarlo sin entrada falla."
	default:
		r.Texto = "Compila, pero al ejecutarlo falla."
	}
	if caso.Entrada == "" && ejcLeeEntrada(actual) {
		r.Parrafos = append(r.Parrafos, "Ojo: el programa lee datos y no me diste ninguno, así que puede que el fallo sea eso. Dime con qué entrada lo pruebo, por ejemplo «arréglalo con entrada 5».")
	}
	return r
}

// repCorrerPrograma builds a whole program and runs it once.
func (c *Cerebro) repCorrerPrograma(k *tarea, nodo *nucleo.Nodo, fuente string, caso nucleo.CasoPrograma, permiso nucleo.Permiso) (nucleo.Ejecucion, error) {
	instr := nucleo.InstrNormal
	if permiso == nucleo.PermisoUsuario {
		instr = nucleo.OpcionesInstr{}
	}
	n := nodo.Sub(nucleo.PasoPrueba, "Compilo y ejecuto el programa")
	p, comp, err := c.ej.PrepararPrograma(k.ctx, fuente, instr, permiso)
	if err != nil {
		n.Mal("No pude prepararlo: %s", resumenErrores(comp, err))
		return nucleo.Ejecucion{}, err
	}
	defer p.Cerrar()
	ej, err := p.Correr(k.ctx, caso)
	if err != nil {
		n.Mal("No pude ejecutarlo: %s", mensajeError(err))
		return ej, err
	}
	if _, frase := repResultadoPrograma(ej, caso.Entrada); frase != "" {
		n.Detalle("%s", frase)
	}
	if ej.Comando != "" {
		n.Detalle("%s", ej.Comando)
	}
	if repTerminoBien(ej) {
		n.Bien("Terminó sin errores (%s)", milis(ej.Ms))
	} else {
		n.Mal("No terminó bien")
	}
	return ej, nil
}

func repTerminoBien(ej nucleo.Ejecucion) bool {
	return ej.Codigo == 0 && ej.Senal == "" && !ej.Agotado && !ej.SinCombustible
}

// repResultadoPrograma describes one run; the comprobación is non-nil when it ended without errors.
func repResultadoPrograma(ej nucleo.Ejecucion, entrada string) (*nucleo.Comprobacion, string) {
	con := ""
	if entrada != "" {
		con = " con «" + recortar(strings.ReplaceAll(strings.TrimRight(entrada, "\n"), "\n", "⏎"), 60) + "⏎»"
	}
	salida := strings.TrimRight(ej.Salida, "\n")
	frase := "Lo ejecuté" + con
	switch {
	case salida == "":
		frase += " y no escribió nada"
	case !strings.Contains(salida, "\n") && len([]rune(salida)) <= 80:
		frase += " y escribió «" + salida + "»"
	default:
		frase += " y escribió:\n" + recortar(salida, 2000)
	}
	if ej.Recortado {
		frase += "\n(la salida era muy larga y la corté)"
	}
	switch {
	case ej.Agotado:
		frase += ". Lo paré porque tardaba demasiado."
	case ej.SinCombustible:
		frase += ". Lo paré: daba demasiados pasos (¿un bucle sin fin?)."
	case strings.Contains(ej.ErrSalida, "panic:"):
		i := strings.Index(ej.ErrSalida, "panic:")
		frase += ". Se rompió: " + reparar.TraducirPanico(primeraLinea(strings.TrimSpace(strings.TrimPrefix(ej.ErrSalida[i:], "panic:"))))
	case ej.Senal != "":
		frase += ". Lo cortó el sistema (" + ej.Senal + ")."
	case ej.Codigo != 0:
		frase += fmt.Sprintf(". Terminó con código de error %d.", ej.Codigo)
		if s := repUltimaLinea(ej.ErrSalida); s != "" {
			frase += " Escribió en la salida de errores: «" + recortar(s, 200) + "»."
		}
	default:
		frase += "."
	}
	if !repTerminoBien(ej) {
		return nil, frase
	}
	texto := "Lo ejecuté y terminó sin errores"
	if entrada == "" {
		texto = "Lo ejecuté (sin entrada) y terminó sin errores"
	}
	return &nucleo.Comprobacion{Nivel: nucleo.NivelSinFallos, Pasan: 1, Total: 1, Texto: texto, Comando: ej.Comando}, frase
}

// repUltimaLinea is the last non-empty line: the error a program prints when it gives up comes after
// its prompts ("Escribe un número entero (n):").
func repUltimaLinea(s string) string {
	ls := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(ls) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(ls[i]); l != "" {
			return l
		}
	}
	return ""
}

// repParrafosCambios: the compiler errors explained and every change with its reason.
func repParrafosCambios(errores []string, cambios []reparar.Cambio, suposiciones []string) []string {
	var out []string
	if len(errores) > 0 {
		lineas := errores
		if len(lineas) > 6 {
			lineas = append(append([]string(nil), lineas[:6]...), fmt.Sprintf("(y %d más)", len(errores)-6))
		}
		out = append(out, "Lo que decía el compilador:\n• "+strings.Join(lineas, "\n• "))
	}
	if len(cambios) > 0 {
		var ls []string
		for _, c := range cambios {
			s := mayusculaInicial(strings.TrimSpace(c.Porque))
			if s == "" {
				s = "Cambio «" + c.Antes + "» por «" + c.Despues + "»"
			}
			if c.Linea > 0 {
				s = fmt.Sprintf("Línea %d: %s", c.Linea, s)
			}
			ls = append(ls, s)
		}
		out = append(out, "Cambios:\n• "+strings.Join(ls, "\n• "))
	}
	for _, s := range suposiciones {
		out = append(out, "Ojo: "+s+".")
	}
	return out
}

// repResumenCambios is a short summary for the first line ("faltaba importar «strings»").
func repResumenCambios(cs []reparar.Cambio) string {
	if len(cs) == 0 {
		return "no hizo falta cambiar nada."
	}
	principal := cs[0]
	for _, c := range cs {
		if !strings.Contains(c.Porque, "suposición") && c.Regla != "quitar_variable" && c.Regla != "quitar_import" {
			principal = c
			break
		}
	}
	s := repMinuscula(strings.TrimSuffix(strings.TrimSpace(principal.Porque), "."))
	if s == "" {
		s = "cambio «" + principal.Antes + "» por «" + principal.Despues + "»"
	}
	switch {
	case len(cs) == 2:
		s += " (y 1 cambio más)"
	case len(cs) > 2:
		s += fmt.Sprintf(" (y %d cambios más)", len(cs)-1)
	}
	return recortar(s, 120) + "."
}

// repTablaAntesDespues: the cases that failed before, with the outputs before and after the fix.
func repTablaAntesDespues(f nucleo.Firma, casos []nucleo.Caso, antes, despues []nucleo.ResultadoCaso) nucleo.Tabla {
	tb := nucleo.Tabla{Titulo: "Antes y después del arreglo", Cabecera: []string{"Entrada", "Esperado", "Antes", "Después"}}
	for i, c := range casos {
		if len(tb.Filas) >= 10 {
			break
		}
		if i >= len(antes) || (antes[i].OK && c.Expectativa != nucleo.EspUsuario) {
			continue
		}
		esperado := "sin errores"
		if c.ConEsperado() {
			esperado = repValores(c.Esperado, f.Res)
		}
		d := nucleo.ResultadoCaso{}
		if i < len(despues) {
			d = despues[i]
		}
		marca := "mal"
		if d.OK {
			marca = "bien"
		}
		tb.Filas = append(tb.Filas, []string{repEntradas(f, c.Entradas), esperado, repObtenido(f, antes[i]), repObtenido(f, d)})
		tb.Marcas = append(tb.Marcas, marca)
	}
	return tb
}

func repObtenido(f nucleo.Firma, r nucleo.ResultadoCaso) string {
	switch {
	case !r.Ejecutado:
		return "—"
	case r.SinCombustible || r.Agotado:
		return "no termina"
	case r.Panico != "" || r.Caida:
		return "se rompe"
	}
	return repValores(r.Obtenido, f.Res)
}

// repJustificacion is what "¿por qué?" shows: what was run and how the fix was found.
func repJustificacion(k *tarea, e *repEspec, casos []nucleo.Caso, comps []nucleo.Comprobacion, cambios []reparar.Cambio) []string {
	out := []string{fmt.Sprintf("Probé «%s» con %d casos (semilla %d).", e.f.Nombre, len(casos), k.semilla)}
	for _, cp := range comps {
		out = append(out, fmt.Sprintf("%s (%d/%d).", cp.Texto, cp.Pasan, cp.Total))
	}
	if e.referencia != "" {
		out = append(out, "Comparé con «"+e.referencia+"», una función que ya conocía con el mismo propósito.")
	}
	for _, c := range cambios {
		if c.Regla != "" {
			out = append(out, fmt.Sprintf("Regla «%s»: %s.", c.Regla, strings.TrimSuffix(c.Porque, ".")))
		}
	}
	return out
}

// repGuardar stores a fix verified with the user's examples (Origen reparacion).
func (c *Cerebro) repGuardar(k *tarea, e *repEspec, nombre, fuente string, nivel nucleo.Nivel) string {
	cod, err := analisis.Funcion(fuente, nombre)
	if err != nil {
		return ""
	}
	if cod, err = analisis.Normalizar(cod, "solucion"); err != nil {
		return ""
	}
	guardados, _ := nucleo.GuardarCasos(e.f, e.usuario)
	desc := strings.TrimSpace(strings.TrimSuffix(k.q.Resto, ":"))
	if desc == "" {
		desc = "versión arreglada de " + nombre
	}
	frase, err := c.guardarFuncion(k, nucleo.FuncionAprendida{
		Nombre: e.f.Nombre, Firma: e.f, Codigo: cod, Descripcion: recortar(desc, 160),
		Conceptos: e.conceptos, Casos: guardados, Nivel: nivel, Origen: "reparacion",
	}, nil, false)
	if err != nil {
		return ""
	}
	return frase
}

// ---- small helpers ----

func repClave(vs []nucleo.Valor) string {
	var sb strings.Builder
	for _, v := range vs {
		sb.WriteString(nucleo.Clave(v))
		sb.WriteByte('|')
	}
	return sb.String()
}

func repIgualValores(a, b []nucleo.Valor) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !nucleo.Igual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// repEntradas writes a case's inputs as the user would ("[3, 1, 2]", "3, \"hola\"").
func repEntradas(f nucleo.Firma, vs []nucleo.Valor) string {
	ent := f.Entradas()
	var out []string
	for i, v := range vs {
		var t nucleo.Tipo
		if i < len(ent) {
			t = ent[i]
		}
		out = append(out, nucleo.FormatoHumano(v, t))
	}
	return strings.Join(out, ", ")
}

func repValores(vs []nucleo.Valor, ts []nucleo.Tipo) string {
	var out []string
	for i, v := range vs {
		var t nucleo.Tipo
		if i < len(ts) {
			t = ts[i]
		}
		out = append(out, nucleo.FormatoHumano(v, t))
	}
	if len(out) == 0 {
		return "nada"
	}
	return strings.Join(out, ", ")
}

func repSalida(f nucleo.Firma, vs []nucleo.Valor) string {
	if vs == nil {
		return "se rompe"
	}
	return repValores(vs, f.Res)
}

// repLeerEntradas reads typed inputs ("[3,3]", "(2, \"a\")", "2, a") for f.
func repLeerEntradas(s string, f nucleo.Firma) ([]nucleo.Valor, bool) {
	ent := f.Entradas()
	s = strings.TrimSpace(s)
	if len(ent) == 0 || s == "" {
		return nil, false
	}
	if len(ent) == 1 {
		if v, err := nucleo.ParseValor(s, ent[0]); err == nil {
			return []nucleo.Valor{v}, true
		}
		return nil, false
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = s[1 : len(s)-1]
	}
	partes := repPartirComas(s)
	if len(partes) != len(ent) {
		return nil, false
	}
	out := make([]nucleo.Valor, len(ent))
	for i, p := range partes {
		v, err := nucleo.ParseValor(strings.TrimSpace(p), ent[i])
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

// repPartirComas splits at top-level commas (outside brackets and quotes).
func repPartirComas(s string) []string {
	var out []string
	nivel, ini := 0, 0
	comilla := rune(0)
	for i, r := range s {
		switch {
		case comilla != 0:
			if r == comilla {
				comilla = 0
			}
		case r == '"' || r == '\'' || r == '`':
			comilla = r
		case r == '[' || r == '(' || r == '{':
			nivel++
		case r == ']' || r == ')' || r == '}':
			nivel--
		case r == ',' && nivel == 0:
			out = append(out, s[ini:i])
			ini = i + 1
		}
	}
	return append(out, s[ini:])
}

// repMinuscula lowers the first letter (for a sentence used inside another one).
func repMinuscula(s string) string {
	r := []rune(s)
	if len(r) < 2 || r[1] >= 'A' && r[1] <= 'Z' {
		return s
	}
	return strings.ToLower(string(r[0])) + string(r[1:])
}
