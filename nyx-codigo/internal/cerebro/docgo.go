package cerebro

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/saber"
)

// doc_go (§4.14.2): 1) the Spanish note (recetas.BuscarNotas); 2) `go doc` (English, labelled); 3) a
// runnable example (the note's, and a GOROOT Example from the IndiceStd), run under PermisoAuto and
// compared with its expected output ("Lo probé y escribe: …"); 4) online: gobyexample and pkg.go.dev.
// Badges: "Según la documentación de Go" (fuente; "Según mis notas de Go" when only the shipped note
// backs it) and "Ejemplo comprobado" when an example ran and printed what it should.

var docgoReSimbolo = regexp.MustCompile(`\b((?:[a-z][a-z0-9]*/)*[a-z][a-z0-9]*)\.([A-Z][A-Za-z0-9_]*(?:\.[A-Z][A-Za-z0-9_]*)?)`)

// docgoVacias are words that never name the topic of a Go question.
var docgoVacias = map[string]bool{
	"que": true, "para": true, "servir": true, "hacer": true, "como": true, "usar": true, "ser": true, "es": true,
	"un": true, "una": true, "unos": true, "unas": true, "el": true, "la": true, "los": true, "las": true, "de": true,
	"del": true, "en": true, "go": true, "golang": true, "funcion": true, "uso": true, "explicar": true, "decir": true,
	"significar": true, "cual": true, "por": true, "y": true, "o": true, "a": true, "con": true, "mi": true, "me": true,
	"lo": true, "le": true, "se": true, "sirve": true, "hace": true, "estar": true, "haber": true, "poder": true,
	"saber": true, "querer": true, "algo": true, "esto": true, "eso": true, "este": true, "ese": true, "al": true,
	"funcionar": true, "buscar": true, "busca": true, "buscame": true, "encontrar": true, "dar": true, "ejemplo": true, "cosa": true, "mas": true, "bien": true, "mejor": true,
}

const docgoTiempoDoc = 6 * time.Second

func pipeDocGo(c *Cerebro, k *tarea) nucleo.Respuesta {
	simbolo, palabras := docgoTema(k)
	tema := simbolo
	if tema == "" {
		tema = strings.Join(palabras, " ")
	}
	k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) mis notas de Go en español 2) la documentación de Go 3) probar un ejemplo").Info("")
	r := nucleo.Respuesta{Comprobaciones: []nucleo.Comprobacion{}, Metodo: "documentacion"}
	var justif []string

	// 1. the Spanish note
	nn := k.raiz.Sub(nucleo.PasoRecuerdo, "Miro mis notas de Go en español")
	nota, hayNota := docgoElegirNota(recetas.BuscarNotas(palabras, simbolo, 4), simbolo, palabras)
	if hayNota {
		nn.Bien("Tengo una nota: %s", nota.Titulo)
		justif = append(justif, "Busqué «"+tema+"» en mis notas de Go en español y encontré «"+nota.Titulo+"».")
	} else {
		nn.Info("No tengo nota sobre «%s»", tema)
	}

	// 2. go doc (English)
	sym := simbolo
	if sym == "" && hayNota && docgoReSimbolo.MatchString(nota.Clave) {
		sym = nota.Clave
	}
	var doc docgoDoc
	if sym != "" {
		doc = c.docgoGoDoc(k, sym)
		if doc.texto != "" {
			justif = append(justif, "Leí `go doc "+sym+"` de tu instalación de Go (el mismo texto que pkg.go.dev, en inglés).")
		}
	}

	// online: pkg.go.dev when go doc is not available, and gobyexample
	enLinea := c.enLinea(k.ctx)
	var webDoc saber.Documento
	if doc.texto == "" && sym != "" && enLinea {
		nw := k.raiz.Sub(nucleo.PasoBusqueda, "Leo pkg.go.dev")
		if d, err := c.web.PkgGoDev(k.ctx, sym); err == nil && docgoTextoDoc(d) != "" {
			webDoc = d
			c.hablaLeerWeb(d) // only Spanish pages are read (habla.go)
			nw.Bien("")
		} else {
			nw.Info("No lo encontré")
		}
	}
	var temaGBE *saber.Tema
	if enLinea {
		temaGBE = c.docgoGoByExample(k, simbolo, palabras, nota, hayNota)
	}

	if !hayNota && doc.texto == "" && docgoTextoDoc(webDoc) == "" {
		k.pendiente = "no encontré documentación"
		r.Texto = "No tengo nada sobre «" + tema + "» en mis notas ni en la documentación de Go que tengo aquí."
		r.Nivel = nucleo.NivelSinComprobar
		if temaGBE != nil {
			r.Parrafos = append(r.Parrafos, "Quizá te sirva este ejemplo: "+temaGBE.Titulo+".")
			r.Fuentes = append(r.Fuentes, nucleo.Fuente{URL: temaGBE.URL, Titulo: "Go by Example: " + temaGBE.Titulo})
		}
		r.Parrafos = append(r.Parrafos, "Puedes escribirlo con el nombre exacto (por ejemplo «strings.Split») o pedirme que lo busque en internet.")
		r.Sugerencias = []string{"Busca " + tema + " en Go"}
		return r
	}

	// 3. run the examples
	ejemploOK := false
	if hayNota && strings.TrimSpace(nota.Ejemplo) != "" {
		r.Codigo = nota.Ejemplo
		if salida, ok, motivo := c.docgoProbar(k, "de la nota", nota.Ejemplo, nota.Salida, false); ok {
			ejemploOK = true
			r.Parrafos = append(r.Parrafos, "Lo probé y escribe: «"+docgoSalida(salida)+"».")
			justif = append(justif, "Ejecuté el ejemplo en la caja segura y escribió exactamente lo que debía.")
		} else if motivo != "" {
			r.Parrafos = append(r.Parrafos, "No probé el ejemplo: "+motivo+".")
		}
	}
	if ix := c.indice(); ix != nil && sym != "" {
		ejs := ix.Buscar(palabras, sym, 1)
		if len(ejs) > 0 {
			e := ejs[0]
			if prog := ix.Programa(e); prog != "" && e.Salida != "" {
				if salida, ok, _ := c.docgoProbar(k, e.Nombre, prog, e.Salida, e.Desordenada); ok {
					ejemploOK = true
					r.Parrafos = append(r.Parrafos, "También probé el ejemplo oficial `"+e.Nombre+"` y escribe: «"+docgoSalida(salida)+"».")
					justif = append(justif, "Ejecuté el ejemplo oficial "+e.Nombre+" de la biblioteca estándar y su salida coincide con su // Output:.")
					if r.Codigo == "" {
						r.Codigo = prog
					}
				}
			}
		}
	}

	// the answer
	switch {
	case hayNota:
		r.Texto = nota.Titulo
		r.Parrafos = append([]string{nota.Resumen}, r.Parrafos...)
	case doc.texto != "":
		r.Texto = "`" + sym + "`, según la documentación de Go (en inglés):"
	default:
		r.Texto = "`" + sym + "`, según " + saber.Dominio(webDoc.URL) + " (en inglés):"
	}
	if doc.texto != "" {
		p := "Documentación de Go (en inglés): "
		if doc.firma != "" {
			p += "`" + doc.firma + "` — "
		}
		r.Parrafos = append(r.Parrafos, p+doc.texto)
		r.Fuentes = append(r.Fuentes, nucleo.Fuente{URL: doc.url, Titulo: sym + " en pkg.go.dev", Nota: "documentación oficial de Go, en inglés (la leí con go doc)"})
	}
	if t := docgoTextoDoc(webDoc); t != "" {
		r.Parrafos = append(r.Parrafos, segun(webDoc.URL, recortar(t, 500)+" (en inglés)"))
		r.Fuentes = append(r.Fuentes, nucleo.Fuente{URL: webDoc.URL, Titulo: sym + " en pkg.go.dev", Nota: "documentación oficial de Go, en inglés"})
		justif = append(justif, "Leí la documentación de "+sym+" en pkg.go.dev.")
	}
	if temaGBE != nil {
		r.Parrafos = append(r.Parrafos, "Hay un ejemplo completo en Go by Example: «"+temaGBE.Titulo+"».")
		r.Fuentes = append(r.Fuentes, nucleo.Fuente{URL: temaGBE.URL, Titulo: "Go by Example: " + temaGBE.Titulo, Nota: "ejemplos con su código"})
	}
	if hayNota && len(nota.Relacionados) > 0 {
		r.Parrafos = append(r.Parrafos, "Relacionado: "+strings.Join(nota.Relacionados, ", ")+".")
	}

	fuente := nucleo.Comprobacion{Nivel: nucleo.NivelFuente, Pasan: 1, Total: 1, Texto: "Según la documentación de Go"}
	switch {
	case doc.texto != "":
		fuente.Detalle = "go doc " + sym
	case docgoTextoDoc(webDoc) != "":
		fuente.Detalle = webDoc.URL
	default:
		fuente.Texto = "Según mis notas de Go"
		fuente.Detalle = "nota «" + nota.Clave + "», comprobada al publicar Nyx Código"
	}
	r.Comprobaciones = append(r.Comprobaciones, fuente)
	if ejemploOK {
		r.Comprobaciones = append(r.Comprobaciones, nucleo.Comprobacion{Nivel: nucleo.NivelEntendido, Pasan: 1, Total: 1,
			Texto: "Ejemplo comprobado", Detalle: "lo ejecuté en la caja segura y escribió lo esperado"})
	}
	r.Nivel = nucleo.NivelFuente
	r.Exito = true
	r.Justificacion = justif
	if hayNota {
		for _, rel := range nota.Relacionados {
			if len(r.Sugerencias) == 3 {
				break
			}
			r.Sugerencias = append(r.Sugerencias, "¿Qué es "+rel+"?")
		}
	}
	if r.Codigo != "" && len(r.Sugerencias) < 4 {
		r.Sugerencias = append(r.Sugerencias, "Explícalo")
	}
	k.turno.Metodo = "documentacion"
	return r
}

// docgoTema finds the Go symbol ("strings.Fields") and the topic words of the question.
func docgoTema(k *tarea) (string, []string) {
	simbolo := ""
	texto := k.p.Texto
	if k.q != nil && k.q.Texto != "" {
		texto = k.q.Texto
	}
	if m := docgoReSimbolo.FindStringSubmatch(texto); m != nil {
		simbolo = m[1] + "." + m[2]
	}
	var palabras []string
	visto := map[string]bool{}
	if k.q != nil {
		for _, tk := range k.q.Tokens {
			if tk.Clase != "palabra" {
				continue
			}
			w := tk.Lema
			if w == "" {
				w = tk.Norma
			}
			if strings.Contains(w, ".") || docgoVacias[w] || docgoVacias[tk.Norma] || len([]rune(w)) < 2 || visto[w] {
				continue
			}
			visto[w] = true
			palabras = append(palabras, w)
		}
	}
	if simbolo != "" {
		for _, parte := range strings.Split(strings.ToLower(simbolo), ".") {
			if i := strings.LastIndexByte(parte, '/'); i >= 0 {
				parte = parte[i+1:]
			}
			if !visto[parte] {
				visto[parte] = true
				palabras = append(palabras, parte)
			}
		}
	}
	return simbolo, palabras
}

// docgoElegirNota keeps the note that really is about the topic: the symbol's own note, or a note
// whose key, title or words contain a topic word.
func docgoElegirNota(notas []recetas.Nota, simbolo string, palabras []string) (recetas.Nota, bool) {
	if simbolo != "" {
		s := nucleo.Normalizar(simbolo)
		corto := s
		if i := strings.LastIndexByte(corto, '/'); i >= 0 {
			corto = corto[i+1:]
		}
		for _, n := range notas {
			cl := nucleo.Normalizar(n.Clave)
			if cl == s || cl == corto {
				return n, true
			}
		}
		return recetas.Nota{}, false
	}
	for _, n := range notas {
		campos := " " + nucleo.Normalizar(n.Clave+" "+n.Titulo+" "+strings.Join(n.Palabras, " ")) + " "
		for _, w := range palabras {
			w = nucleo.Normalizar(w)
			if len([]rune(w)) >= 4 && strings.Contains(campos, w) || strings.Contains(campos, " "+w+" ") {
				return n, true
			}
		}
	}
	return recetas.Nota{}, false
}

type docgoDoc struct {
	firma, texto, url string
}

// docgoGoDoc runs `go doc sym` and keeps the declaration and its first paragraph.
func (c *Cerebro) docgoGoDoc(k *tarea, sym string) docgoDoc {
	n := k.raiz.Sub(nucleo.PasoRecuerdo, "Leo `go doc %s` (en inglés)", sym)
	if c.goBin == "" {
		n.Info("No tengo Go para leer su documentación")
		return docgoDoc{}
	}
	ctx, cancelar := context.WithTimeout(k.ctx, docgoTiempoDoc)
	defer cancelar()
	salida, err := saber.GoDoc(ctx, c.goBin, sym)
	if err != nil || strings.TrimSpace(salida) == "" {
		n.Info("No lo encuentro: %s", mensajeError(err))
		return docgoDoc{}
	}
	d := docgoLeerGoDoc(salida, sym)
	if d.texto == "" {
		n.Info("No dice nada útil")
		return docgoDoc{}
	}
	n.Detalle("%s", recortar(salida, 1500))
	n.Bien("")
	return d
}

// docgoLeerGoDoc reads `go doc` output: "package strings // import "strings"", the declaration, then
// the indented description.
func docgoLeerGoDoc(salida, sym string) docgoDoc {
	var d docgoDoc
	ruta := ""
	var desc []string
	enDesc := false
	for _, linea := range strings.Split(salida, "\n") {
		t := strings.TrimSpace(linea)
		if t == "" {
			if enDesc && len(strings.Join(desc, " ")) > 80 {
				break
			}
			enDesc = false
			continue
		}
		sangrada := strings.HasPrefix(linea, " ") || strings.HasPrefix(linea, "\t")
		switch {
		case strings.HasPrefix(t, "package ") && strings.Contains(t, "// import "):
			if i := strings.Index(t, `"`); i >= 0 {
				ruta = strings.Trim(t[i:], `"`)
			}
		case d.firma == "" && !sangrada:
			d.firma = t
		case sangrada && d.firma != "":
			enDesc = true
			desc = append(desc, t)
		}
	}
	d.texto = recortar(strings.Join(desc, " "), 500)
	resto := sym
	if i := strings.LastIndexByte(resto, '/'); i >= 0 {
		resto = resto[i+1:]
	}
	ancla := ""
	if i := strings.IndexByte(resto, '.'); i >= 0 {
		ancla = resto[i+1:]
	}
	if ruta == "" {
		ruta = strings.TrimSuffix(strings.TrimSuffix(sym, ancla), ".")
	}
	d.url = "https://pkg.go.dev/" + ruta
	if ancla != "" {
		d.url += "#" + ancla
	}
	return d
}

// docgoTextoDoc is the first useful paragraph of a fetched documentation page.
func docgoTextoDoc(d saber.Documento) string {
	for _, b := range d.Bloques {
		if b.Tipo == "parrafo" && len([]rune(b.Texto)) >= 30 {
			return b.Texto
		}
	}
	return ""
}

// docgoProbar runs an example program under PermisoAuto and compares its output with the expected one.
// motivo says why it was not run ("" when it ran).
func (c *Cerebro) docgoProbar(k *tarea, titulo, programa, esperado string, desordenada bool) (salida string, ok bool, motivo string) {
	if c.ej == nil {
		return "", false, "no tengo caja segura para ejecutar código"
	}
	if strings.TrimSpace(esperado) == "" {
		return "", false, "no sé qué debe escribir"
	}
	if v := c.ej.Revisar(programa, nucleo.PermisoAuto); len(v) > 0 {
		return "", false, "por seguridad no lo ejecuto solo (" + v[0].Que + ")"
	}
	n := k.raiz.Sub(nucleo.PasoPrueba, "Pruebo el ejemplo %s", titulo)
	prog, comp, err := c.ej.PrepararPrograma(k.ctx, programa, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		n.Mal("No compila: %s", resumenErrores(comp, err))
		return "", false, "no compila (" + resumenErrores(comp, err) + ")"
	}
	defer prog.Cerrar()
	ej, err := prog.Correr(k.ctx, nucleo.CasoPrograma{})
	if err != nil {
		n.Mal("No pude ejecutarlo: %s", mensajeError(err))
		return "", false, "no pude ejecutarlo (" + mensajeError(err) + ")"
	}
	salida = ej.Salida
	ok = ej.Codigo == 0 && !ej.Agotado && pruebas.CompararSalida(salida, nucleo.CasoPrograma{Esperado: esperado, Comparar: "lineas"})
	if !ok && desordenada && ej.Codigo == 0 {
		ok = docgoMismasLineas(salida, esperado)
	}
	if ok {
		n.Bien("Escribe lo que debe: %s", recortar(docgoSalida(salida), 120))
	} else {
		n.Mal("Escribió otra cosa: %s", recortar(docgoSalida(salida), 120))
	}
	return salida, ok, ""
}

func docgoMismasLineas(a, b string) bool {
	partir := func(s string) []string {
		var out []string
		for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
			out = append(out, strings.TrimRight(l, " \t\r"))
		}
		sort.Strings(out)
		return out
	}
	x, y := partir(a), partir(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// docgoSalida shows a program's output on one line: "[uno dos tres] ⏎ 3".
func docgoSalida(s string) string {
	return recortar(strings.ReplaceAll(strings.TrimSpace(s), "\n", " ⏎ "), 200)
}

// docgoGoByExample finds a gobyexample topic about the question (the topic list is cached).
func (c *Cerebro) docgoGoByExample(k *tarea, simbolo string, palabras []string, nota recetas.Nota, hayNota bool) *saber.Tema {
	ctx, cancelar := context.WithTimeout(k.ctx, docgoTiempoDoc)
	defer cancelar()
	temas, err := c.web.TemasGoByExample(ctx)
	if err != nil || len(temas) == 0 {
		return nil
	}
	claves := append([]string(nil), palabras...)
	if hayNota {
		claves = append(claves, strings.Fields(strings.NewReplacer(".", " ", "/", " ").Replace(nota.Clave))...)
	}
	if simbolo != "" {
		claves = append(claves, strings.Fields(strings.NewReplacer(".", " ", "/", " ").Replace(simbolo))...)
	}
	for _, t := range temas {
		titulo := " " + nucleo.Normalizar(t.Titulo) + " "
		for _, w := range claves {
			w = nucleo.Normalizar(w)
			if len([]rune(w)) >= 3 && (strings.Contains(titulo, " "+w+" ") || strings.Contains(titulo, " "+w+"s ")) {
				tt := t
				k.raiz.Sub(nucleo.PasoBusqueda, "Go by Example tiene «%s»", t.Titulo).Bien("")
				return &tt
			}
		}
	}
	return nil
}

// docgoResumenNota is used by buscar when there is no internet: the best note for some words.
func docgoResumenNota(palabras []string) (recetas.Nota, bool) {
	return docgoElegirNota(recetas.BuscarNotas(palabras, "", 3), "", palabras)
}
