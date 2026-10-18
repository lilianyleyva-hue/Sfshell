package cerebro

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/reparar"
	"nyxcodigo/internal/saber"
)

// buscar (§4.14.2, N2): search → documents → Rankear (top 3 passages, each "Según <dominio>: …" with its
// link) plus code candidates. A candidate that passes the automatic safety check is run: a program
// without input once, a function with a signature I can call on generated cases (no crash, same result
// twice). Anything else is only compiled and badged "Sin comprobar · compila". ExtraerHechos are stored
// with their sources. Web code never runs with extended permissions.

const (
	buscarMaxCandidatas = 6
	buscarMaxHechos     = 10
	buscarCasos         = 100
	buscarRepetir       = 3 // runs per case to catch results that change from run to run
)

func pipeBuscar(c *Cerebro, k *tarea) nucleo.Respuesta {
	consulta := buscarConsulta(k)
	tema := buscarTema(k, consulta) // the user's words: the English query is only for the search engine
	k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) buscar en internet 2) leer las mejores páginas 3) probar el código que encuentre").Info("")
	r := nucleo.Respuesta{Comprobaciones: []nucleo.Comprobacion{}, Metodo: "web"}
	if consulta == "" {
		r.Texto = "¿Qué quieres que busque? Escríbelo así: «busca cómo leer un archivo en Go»."
		r.Nivel = nucleo.NivelSinComprobar
		return r
	}
	if !c.enLinea(k.ctx) {
		k.raiz.Sub(nucleo.PasoNota, "No tengo internet").Info("")
		k.pendiente = "sin internet"
		r.Texto = "No puedo buscarlo: ahora mismo no tengo internet."
		r.Nivel = nucleo.NivelSinComprobar
		_, palabras := docgoTema(k)
		if nota, ok := docgoResumenNota(palabras); ok {
			r.Parrafos = append(r.Parrafos, "Mientras tanto, esto dicen mis notas: "+nota.Titulo+". "+nota.Resumen)
			r.Codigo = nota.Ejemplo
			r.Comprobaciones = append(r.Comprobaciones, nucleo.Comprobacion{Nivel: nucleo.NivelFuente, Pasan: 1, Total: 1,
				Texto: "Según mis notas de Go", Detalle: "nota «" + nota.Clave + "»"})
			r.Nivel = nucleo.NivelFuente
		}
		r.Parrafos = append(r.Parrafos, "Lo he apuntado: lo buscaré cuando estudie con internet.")
		return r
	}

	nb := k.raiz.Sub(nucleo.PasoBusqueda, "Busco «%s»", tema)
	if consulta != tema {
		nb.Detalle("Lo busco con estas palabras en inglés: %s", consulta)
	}
	docs, err := c.buscarDocumentos(k.ctx, consulta)
	c.hablaLeerWeb(docs...) // Spanish pages also teach meanings and how to talk (habla.go)
	if err != nil || len(docs) == 0 {
		motivo := "no encontré páginas"
		if err != nil {
			motivo = mensajeError(err)
		}
		nb.Mal("No encontré nada: %s", motivo)
		k.pendiente = "búsqueda sin resultados"
		r.Texto = "No encontré nada útil sobre «" + tema + "» (" + motivo + ")."
		r.Nivel = nucleo.NivelSinComprobar
		r.Parrafos = []string{"Prueba a decirlo con otras palabras, o con el nombre exacto de la función de Go."}
		return r
	}
	nb.Bien("Leí %s", cuantas(len(docs), "página", "páginas"))
	titulos := map[string]string{}
	for _, d := range docs {
		titulos[d.URL] = d.Titulo
	}
	fuentes := map[string]bool{}
	ponerFuente := func(url, nota string) {
		if url == "" || fuentes[url] {
			return
		}
		fuentes[url] = true
		t := titulos[url]
		if t == "" {
			t = saber.Dominio(url)
		}
		r.Fuentes = append(r.Fuentes, nucleo.Fuente{URL: url, Titulo: t, Nota: nota})
	}

	// passages
	np := k.raiz.Sub(nucleo.PasoBusqueda, "Elijo las frases que mejor responden")
	pasajes := c.rankear(consulta, docs, 3)
	for _, p := range pasajes {
		r.Parrafos = append(r.Parrafos, segun(p.URL, recortar(p.Texto, 400)))
		ponerFuente(p.URL, "")
		np.Detalle("%s — %s", saber.Dominio(p.URL), recortar(p.Texto, 200))
	}
	if len(pasajes) > 0 {
		np.Bien("%s", cuantas(len(pasajes), "frase", "frases"))
	} else {
		np.Info("Ninguna frase responde bien")
	}

	// code: a candidate that was run and fails is only offered when there is nothing else to show
	bc := c.buscarCodigo(k, docs)
	codigo := bc.codigo
	if codigo != "" && bc.falla != "" && len(pasajes) > 0 {
		r.Parrafos = append(r.Parrafos, "También encontré código, pero lo probé y "+bc.falla+", así que no te lo enseño.")
		codigo = ""
	}
	if codigo != "" {
		r.Codigo = codigo
		if bc.comp != nil {
			r.Comprobaciones = append(r.Comprobaciones, *bc.comp)
		}
		if bc.frase != "" {
			r.Parrafos = append(r.Parrafos, bc.frase)
		}
		ponerFuente(bc.url, "código de la web (licencia del sitio)")
	}

	// facts
	if n := c.buscarHechos(k, docs); n > 0 {
		r.Aprendido = "Guardé " + cuantas(n, "dato", "datos") + " con su fuente."
	}

	switch {
	case len(pasajes) > 0:
		r.Texto = "Esto es lo que encontré sobre «" + tema + "»:"
	case codigo != "" && bc.falla != "":
		k.pendiente = "búsqueda sin respuesta clara"
		r.Texto = "Encontré código para «" + tema + "», pero lo probé y falla."
		r.Nivel = nucleo.NivelFallo
		return r
	case codigo != "":
		r.Texto = "Encontré este código para «" + tema + "»:"
	default:
		k.pendiente = "búsqueda sin respuesta clara"
		r.Texto = "Leí " + cuantas(len(docs), "página", "páginas") + " sobre «" + tema + "», pero ninguna lo explica claramente."
		r.Nivel = nucleo.NivelSinComprobar
		for _, d := range docs {
			ponerFuente(d.URL, "")
		}
		return r
	}
	r.Comprobaciones = append(r.Comprobaciones, nucleo.Comprobacion{Nivel: nucleo.NivelFuente, Pasan: 1, Total: 1,
		Texto: "Según " + cuantas(len(r.Fuentes), "fuente", "fuentes") + " de la web", Detalle: "consulta: " + consulta})
	r.Nivel = nucleo.NivelFuente
	r.Exito = true
	busque := "Busqué «" + tema + "»"
	if consulta != tema {
		busque += " (con las palabras en inglés «" + consulta + "»)"
	}
	r.Justificacion = []string{
		busque + " y leí " + buscarMejores(len(docs)) + ".",
		"Elegí las frases que más se parecen a la pregunta (BM25, con más peso para go.dev y gobyexample y para las webs cuyo código me funcionó otras veces).",
		"El código de la web solo lo ejecuto si pasa la revisión de seguridad automática; si no, solo lo compilo.",
	}
	if codigo != "" {
		r.Sugerencias = []string{"Explícalo", "Hazle pruebas"}
	}
	k.turno.Metodo = "web"
	return r
}

// buscarConsulta is the English query lengua prepared (for the search engine), or the user's words.
func buscarConsulta(k *tarea) string {
	if k.q != nil {
		if q := strings.TrimSpace(k.q.ConsultaWeb); q != "" && q != "golang" {
			return q
		}
	}
	return buscarTema(k, "")
}

// reBuscarVerbo is the request verb in front of what to search: "busca", "¿puedes buscar en internet…".
var reBuscarVerbo = regexp.MustCompile(`(?i)^(?:por favor,?\s+)?(?:(?:me\s+)?(?:puedes|podrías|podrias)\s+)?(?:búscame|buscame|búscalo|buscalo|busca|buscar|encuéntrame|encuentrame|encuentra|investiga)\b[\s,:]*(?:(?:en\s+)?(?:internet|la\s+web|google)\b[\s,:]*)?`)

// buscarTema is what the user asked to search, in their words ("cómo leer un archivo línea por línea en
// Go"): the text without the verb. Answers show it instead of the English query. otro is the fallback.
func buscarTema(k *tarea, otro string) string {
	t := k.p.Texto
	if k.q != nil && strings.TrimSpace(k.q.Resto) != "" {
		t = k.q.Resto
	}
	t = strings.TrimSpace(primeraLinea(strings.TrimSpace(t)))
	t = strings.TrimSpace(strings.TrimLeft(t, "¿¡ "))
	t = reBuscarVerbo.ReplaceAllString(t, "")
	t = strings.Trim(strings.TrimSpace(t), " ¿?¡!.:;,")
	if t == "" {
		return otro
	}
	return recortar(t, 100)
}

// buscado is the code buscar offers: the first candidate that runs well, else the first that only
// compiles (never run), else the first that was run and fails (falla says how, written to follow
// "lo probé y …": "falla en 2 de 89 casos…", "no termina nunca…"; Nivel fallo).
type buscado struct {
	codigo, url, frase string
	comp               *nucleo.Comprobacion
	falla              string
}

// buscarCodigo tries the code candidates of the documents in order. A candidate that was run and failed
// is never shown as merely "compila": it is offered only when nothing better exists, with a fallo badge.
func (c *Cerebro) buscarCodigo(k *tarea, docs []saber.Documento) buscado {
	var cands []saber.Candidata
	for _, d := range docs {
		cands = append(cands, saber.Candidatas(d)...)
	}
	if len(cands) == 0 {
		return buscado{}
	}
	if len(cands) > buscarMaxCandidatas {
		cands = cands[:buscarMaxCandidatas]
	}
	n := k.raiz.Sub(nucleo.PasoPrueba, "Pruebo el código que encontré (%s)", cuantas(len(cands), "trozo", "trozos"))
	if c.ej == nil {
		n.Info("No tengo caja segura: no lo compilo ni lo ejecuto")
		cd := cands[0]
		return buscado{codigo: cd.Codigo, url: cd.URL,
			comp:  &nucleo.Comprobacion{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar", Detalle: cd.URL},
			frase: "No he podido compilarlo ni probarlo: no tengo caja segura."}
	}
	var compila, fallido *buscado
	for i := range cands {
		cd := cands[i]
		if k.ctx.Err() != nil {
			break
		}
		viol := c.ej.Revisar(cd.Codigo, nucleo.PermisoAuto)
		motivo := ""
		if cd.EsPrograma {
			switch {
			case len(viol) > 0:
				motivo = "por seguridad no lo ejecuto solo (" + viol[0].Que + ")"
			case buscarLeeEntrada(cd.Codigo):
				motivo = "lee datos del teclado"
			default:
				salida, falla, crudo, corrio := c.buscarCorrerPrograma(k, n, cd.Codigo)
				switch {
				case corrio && falla == "":
					c.contarDominio(cd.URL, true)
					n.Bien("Uno funciona")
					return buscado{codigo: cd.Codigo, url: cd.URL,
						comp: &nucleo.Comprobacion{Nivel: nucleo.NivelSinFallos, Pasan: 1, Total: 1,
							Texto: "Lo ejecuté: terminó sin errores", Detalle: cd.URL},
						frase: "Lo ejecuté y escribió: «" + docgoSalida(salida) + "»."}
				case corrio:
					c.contarDominio(cd.URL, false)
					if fallido == nil {
						fallido = &buscado{codigo: cd.Codigo, url: cd.URL, falla: falla,
							comp: &nucleo.Comprobacion{Nivel: nucleo.NivelFallo, Pasan: 0, Total: 1,
								Texto: "Lo ejecuté y falla", Detalle: buscarDetalle(falla, crudo, cd.URL)}}
					}
					continue // it ran and failed: never offered as "compila"
				default:
					motivo = "no pude ejecutarlo en la caja segura"
				}
			}
		} else {
			switch {
			case cd.Firma == nil || !cd.Firma.Probable():
				motivo = "no sé con qué datos llamarlo"
			case len(viol) > 0:
				motivo = "por seguridad no lo ejecuto solo (" + viol[0].Que + ")"
			default:
				f := *cd.Firma
				casos := c.generarCasos(k, f, nil, 0, buscarCasos)
				if len(casos) == 0 {
					motivo = "no sé con qué datos llamarlo"
					break
				}
				// three runs per case: one repeat often misses code that depends on map order
				v, err := c.verificar(k, n, cd.Codigo, f, casos, nil, nucleo.OpcionesProbar{Repetir: buscarRepetir})
				switch {
				case err != nil:
					motivo = "no pude probarlo en la caja segura"
				case len(v.Fallos) > 0:
					c.contarDominio(cd.URL, false)
					if fallido == nil {
						falla, crudo := buscarFallaFuncion(f, v)
						fallido = &buscado{codigo: cd.Codigo, url: cd.URL, falla: falla,
							comp: &nucleo.Comprobacion{Nivel: nucleo.NivelFallo, Pasan: len(v.Casos) - len(v.Fallos), Total: len(v.Casos),
								Texto: "Lo probé y falla", Detalle: buscarDetalle(falla, crudo, cd.URL)}}
					}
					continue // it ran and failed: never offered as "compila"
				case len(v.Comprobaciones) > 0:
					c.contarDominio(cd.URL, true)
					comp := v.Comprobaciones[0]
					for _, cp := range v.Comprobaciones {
						if cp.Nivel == nucleo.NivelSinFallos {
							comp = cp
						}
					}
					n.Bien("Uno funciona")
					return buscado{codigo: cd.Codigo, url: cd.URL, comp: &comp,
						frase: fmt.Sprintf("Lo probé con %s generadas: no falla y da siempre lo mismo (no sé qué debe dar, así que no compruebo el resultado).", cuantas(len(casos), "entrada", "entradas"))}
				default:
					motivo = "no pude probarlo"
				}
			}
		}
		if compila == nil {
			comp := c.ej.Compilar(k.ctx, cd.Codigo)
			if !comp.OK && k.ctx.Err() == nil {
				c.contarDominio(cd.URL, false)
			}
			if comp.OK {
				compila = &buscado{codigo: cd.Codigo, url: cd.URL,
					comp:  &nucleo.Comprobacion{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar · compila", Detalle: cd.URL},
					frase: "Este código compila, pero no lo he ejecutado: " + motivo + "."}
			}
		}
	}
	switch {
	case compila != nil:
		n.Info("Compila, pero no lo he ejecutado")
		if fallido != nil {
			compila.frase += " Otro trozo que encontré lo probé y falla, así que no te lo enseño."
		}
		return *compila
	case fallido != nil:
		n.Mal("El código que encontré falla al probarlo")
		fallido.frase = "Ojo: lo probé y " + fallido.falla + ". No lo uses tal cual."
		return *fallido
	}
	n.Mal("Ninguno compila")
	return buscado{}
}

// buscarFallaFuncion says how a web function failed on the generated cases, to follow "lo probé y":
// "falla en 2 de 89 casos; por ejemplo, con xs = [] se para con un error (intentaste leer…)". crudo is
// the runtime's own message, kept for the badge's detail.
func buscarFallaFuncion(f nucleo.Firma, v verificacion) (falla, crudo string) {
	s := fmt.Sprintf("falla en %d de %d casos", len(v.Fallos), len(v.Casos))
	i := v.Fallos[0]
	if i >= len(v.Resultados) || i >= len(v.Casos) {
		return s, ""
	}
	r, ent := v.Resultados[i], explicarEntradas(f, v.Casos[i].Entradas)
	s += "; por ejemplo, con " + ent
	switch {
	case r.NoDeterminista:
		return s + " da resultados distintos cada vez", ""
	case r.Panico != "" || r.Caida:
		crudo = strings.TrimSpace(primeraLinea(r.Panico))
		return s + " se para con un error" + buscarPorQue(crudo), crudo
	case !r.Ejecutado || r.Agotado || r.SinCombustible:
		return s + " no termina", ""
	}
	return s + " no da lo esperado", ""
}

// buscarPorQue explains a runtime error in simple Spanish, in parentheses: " (intentaste leer la posición
// 3 de una lista vacía)". Without a known explanation it quotes the message.
func buscarPorQue(crudo string) string {
	crudo = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(crudo), "panic:"))
	if crudo == "" {
		return ""
	}
	t := reparar.TraducirPanico(crudo)
	if strings.HasPrefix(t, "El programa se detuvo") {
		return " («" + recortar(crudo, 80) + "»)"
	}
	return " (" + explicarMinuscula(strings.TrimRight(t, ". ")) + ")"
}

// buscarDetalle is the badge detail: what failed, the runtime's message and the link.
func buscarDetalle(falla, crudo, url string) string {
	d := falla
	if crudo != "" {
		d += " · " + recortar(crudo, 120)
	}
	return d + " · " + url
}

// buscarCorrerPrograma runs a program once under PermisoAuto, with no input. corrio is false when it
// could not be run at all; falla says how a run failed ("" when it ended well), crudo is the first line
// of its error output.
func (c *Cerebro) buscarCorrerPrograma(k *tarea, nodo *nucleo.Nodo, fuente string) (salida, falla, crudo string, corrio bool) {
	p, comp, err := c.ej.PrepararPrograma(k.ctx, fuente, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		nodo.Sub(nucleo.PasoError, "No compila: %s", resumenErrores(comp, err)).Mal("")
		return "", "", "", false
	}
	defer p.Cerrar()
	ej, err := p.Correr(k.ctx, nucleo.CasoPrograma{})
	if err != nil {
		nodo.Sub(nucleo.PasoError, "No pude ejecutarlo: %s", mensajeError(err)).Mal("")
		return "", "", "", false
	}
	if ej.Codigo != 0 || ej.Agotado || ej.SinCombustible {
		falla, crudo := buscarFallaPrograma(ej)
		nodo.Sub(nucleo.PasoError, "Al ejecutarlo %s", falla).Mal("")
		return ej.Salida, falla, crudo, true
	}
	nodo.Sub(nucleo.PasoPrueba, "Lo ejecuté: %s", recortar(docgoSalida(ej.Salida), 120)).Bien("")
	return ej.Salida, "", "", true
}

// buscarFallaPrograma describes a failed run, to follow "lo probé y": "no termina nunca (da demasiadas
// vueltas; ¿un bucle sin fin?)", "se para con un error (…)". crudo is the first line of its error output.
func buscarFallaPrograma(ej nucleo.Ejecucion) (falla, crudo string) {
	for _, l := range strings.Split(ej.ErrSalida, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			crudo = l
			break
		}
	}
	switch {
	case ej.SinCombustible:
		return "no termina nunca (da demasiadas vueltas; ¿un bucle sin fin?)", crudo
	case ej.Agotado:
		return "no termina a tiempo", crudo
	case strings.HasPrefix(crudo, "panic:") || strings.HasPrefix(crudo, "fatal error:") || strings.HasPrefix(crudo, "runtime:"):
		return "se para con un error" + buscarPorQue(crudo), crudo
	}
	return fmt.Sprintf("termina con un error (código %d)", ej.Codigo), crudo
}

// cuantas writes a count with its noun in the right number: "1 página", "2 páginas".
func cuantas(n int, una, varias string) string {
	if n == 1 {
		return "1 " + una
	}
	return strconv.Itoa(n) + " " + varias
}

// buscarLeeEntrada: the program reads the keyboard.
func buscarLeeEntrada(fuente string) bool {
	for _, s := range []string{"os.Stdin", "fmt.Scan", "bufio.NewReader(os.Stdin)"} {
		if strings.Contains(fuente, s) {
			return true
		}
	}
	return false
}

// buscarHechos extracts simple facts about the query's words and stores them with their sources.
func (c *Cerebro) buscarHechos(k *tarea, docs []saber.Documento) int {
	var temas []string
	_, palabras := docgoTema(k)
	for _, w := range palabras {
		if len([]rune(w)) >= 4 && len(temas) < 3 {
			temas = append(temas, w)
		}
	}
	if len(temas) == 0 {
		return 0
	}
	var listas [][]nucleo.Hecho
	for _, d := range docs {
		for _, t := range temas {
			listas = append(listas, saber.ExtraerHechos(d, t))
		}
	}
	hs := saber.UnirHechos(listas...)
	if len(hs) == 0 {
		return 0
	}
	n := k.raiz.Sub(nucleo.PasoMemoria, "Guardo datos que encontré, con su fuente")
	guardados := 0
	for _, h := range hs {
		if guardados == buscarMaxHechos {
			break
		}
		if h.Fecha.IsZero() {
			h.Fecha = c.ahora()
		}
		if _, err := c.mem.GuardarHecho(h); err == nil {
			guardados++
			n.Detalle("%s (%s)", recortar(h.Texto, 120), saber.Dominio(h.Fuente))
		}
	}
	n.Bien("Guardé %s", cuantas(guardados, "dato", "datos"))
	return guardados
}

// buscarMejores: "la mejor página" / "las 3 mejores páginas".
func buscarMejores(n int) string {
	if n == 1 {
		return "la mejor página"
	}
	return "las " + strconv.Itoa(n) + " mejores páginas"
}
