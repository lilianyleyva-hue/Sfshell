package cerebro

import (
	"fmt"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
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
)

func pipeBuscar(c *Cerebro, k *tarea) nucleo.Respuesta {
	consulta := buscarConsulta(k)
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

	nb := k.raiz.Sub(nucleo.PasoBusqueda, "Busco «%s»", consulta)
	_, docs, err := c.web.BuscarDocumentos(k.ctx, consulta)
	c.hablaLeerWeb(docs...) // Spanish pages also teach meanings and how to talk (habla.go)
	if err != nil || len(docs) == 0 {
		motivo := "no encontré páginas"
		if err != nil {
			motivo = mensajeError(err)
		}
		nb.Mal("No encontré nada: %s", motivo)
		k.pendiente = "búsqueda sin resultados"
		r.Texto = "No encontré nada útil sobre «" + consulta + "» (" + motivo + ")."
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
	pasajes := saber.Rankear(consulta, docs, 3)
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
		r.Parrafos = append(r.Parrafos, "También encontré código, pero lo probé y falla ("+bc.falla+"), así que no te lo enseño.")
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
		r.Texto = "Esto es lo que encontré sobre «" + consulta + "»:"
	case codigo != "" && bc.falla != "":
		k.pendiente = "búsqueda sin respuesta clara"
		r.Texto = "Encontré código para «" + consulta + "», pero lo probé y falla."
		r.Nivel = nucleo.NivelFallo
		return r
	case codigo != "":
		r.Texto = "Encontré este código para «" + consulta + "»:"
	default:
		k.pendiente = "búsqueda sin respuesta clara"
		r.Texto = "Leí " + cuantas(len(docs), "página", "páginas") + " sobre «" + consulta + "», pero ninguna lo explica claramente."
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
	r.Justificacion = []string{
		"Busqué «" + consulta + "» y leí " + buscarMejores(len(docs)) + ".",
		"Elegí las frases que más se parecen a la pregunta (BM25, con más peso para go.dev y gobyexample).",
		"El código de la web solo lo ejecuto si pasa la revisión de seguridad automática; si no, solo lo compilo.",
	}
	if codigo != "" {
		r.Sugerencias = []string{"Explícalo", "Hazle pruebas"}
	}
	k.turno.Metodo = "web"
	return r
}

// buscarConsulta is the English query lengua prepared, or the text without the verb.
func buscarConsulta(k *tarea) string {
	if k.q != nil {
		if q := strings.TrimSpace(k.q.ConsultaWeb); q != "" && q != "golang" {
			return q
		}
	}
	t := strings.TrimSpace(k.p.Texto)
	for _, pre := range []string{"búscame", "buscame", "busca", "buscar", "Busca", "Búscame"} {
		t = strings.TrimSpace(strings.TrimPrefix(t, pre))
	}
	return strings.Trim(t, " ¿?.")
}

// buscado is the code buscar offers: the first candidate that runs well, else the first that only
// compiles (never run), else the first that was run and fails (falla says why; Nivel fallo).
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
				salida, falla, corrio := c.buscarCorrerPrograma(k, n, cd.Codigo)
				switch {
				case corrio && falla == "":
					n.Bien("Uno funciona")
					return buscado{codigo: cd.Codigo, url: cd.URL,
						comp: &nucleo.Comprobacion{Nivel: nucleo.NivelSinFallos, Pasan: 1, Total: 1,
							Texto: "Lo ejecuté: terminó sin errores", Detalle: cd.URL},
						frase: "Lo ejecuté y escribió: «" + docgoSalida(salida) + "»."}
				case corrio:
					if fallido == nil {
						fallido = &buscado{codigo: cd.Codigo, url: cd.URL, falla: falla,
							comp: &nucleo.Comprobacion{Nivel: nucleo.NivelFallo, Pasan: 0, Total: 1,
								Texto: "Lo ejecuté y falla", Detalle: falla + " · " + cd.URL}}
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
				v, err := c.verificar(k, n, cd.Codigo, f, casos, nil, nucleo.OpcionesProbar{Repetir: 2})
				switch {
				case err != nil:
					motivo = "no pude probarlo en la caja segura"
				case len(v.Fallos) > 0:
					if fallido == nil {
						falla := buscarFallaFuncion(f, v)
						fallido = &buscado{codigo: cd.Codigo, url: cd.URL, falla: falla,
							comp: &nucleo.Comprobacion{Nivel: nucleo.NivelFallo, Pasan: len(v.Casos) - len(v.Fallos), Total: len(v.Casos),
								Texto: "Lo probé y falla", Detalle: falla + " · " + cd.URL}}
					}
					continue // it ran and failed: never offered as "compila"
				case len(v.Comprobaciones) > 0:
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
			if comp := c.ej.Compilar(k.ctx, cd.Codigo); comp.OK {
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
		fallido.frase = "Ojo: lo probé y falla (" + fallido.falla + "). No lo uses tal cual."
		return *fallido
	}
	n.Mal("Ninguno compila")
	return buscado{}
}

// buscarFallaFuncion says how a web function failed on the generated cases: "falla en 2 de 89 casos:
// se cae con xs = [] (index out of range…)".
func buscarFallaFuncion(f nucleo.Firma, v verificacion) string {
	s := fmt.Sprintf("falla en %d de %d casos", len(v.Fallos), len(v.Casos))
	i := v.Fallos[0]
	if i >= len(v.Resultados) || i >= len(v.Casos) {
		return s
	}
	r, ent := v.Resultados[i], explicarEntradas(f, v.Casos[i].Entradas)
	switch {
	case r.NoDeterminista:
		return s + ": con " + ent + " da resultados distintos cada vez"
	case r.Panico != "" || r.Caida:
		d := "se cae con " + ent
		if p := strings.TrimSpace(primeraLinea(r.Panico)); p != "" {
			d += " (" + recortar(p, 80) + ")"
		}
		return s + ": " + d
	case !r.Ejecutado || r.Agotado || r.SinCombustible:
		return s + ": con " + ent + " no termina"
	}
	return s + ": con " + ent
}

// buscarCorrerPrograma runs a program once under PermisoAuto, with no input. corrio is false when it
// could not be run at all; falla says why a run failed ("" when it ended well).
func (c *Cerebro) buscarCorrerPrograma(k *tarea, nodo *nucleo.Nodo, fuente string) (salida, falla string, corrio bool) {
	p, comp, err := c.ej.PrepararPrograma(k.ctx, fuente, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		nodo.Sub(nucleo.PasoError, "No compila: %s", resumenErrores(comp, err)).Mal("")
		return "", "", false
	}
	defer p.Cerrar()
	ej, err := p.Correr(k.ctx, nucleo.CasoPrograma{})
	if err != nil {
		nodo.Sub(nucleo.PasoError, "No pude ejecutarlo: %s", mensajeError(err)).Mal("")
		return "", "", false
	}
	if ej.Codigo != 0 || ej.Agotado || ej.SinCombustible {
		falla := buscarFallaPrograma(ej)
		nodo.Sub(nucleo.PasoError, "Al ejecutarlo falla: %s", falla).Mal("")
		return ej.Salida, falla, true
	}
	nodo.Sub(nucleo.PasoPrueba, "Lo ejecuté: %s", recortar(docgoSalida(ej.Salida), 120)).Bien("")
	return ej.Salida, "", true
}

// buscarFallaPrograma describes a failed run: "terminó con error (código 2): panic: …".
func buscarFallaPrograma(ej nucleo.Ejecucion) string {
	var s string
	switch {
	case ej.Agotado:
		s = "no terminó a tiempo"
	case ej.SinCombustible:
		s = "no terminó (demasiados pasos)"
	default:
		s = fmt.Sprintf("terminó con error (código %d)", ej.Codigo)
	}
	for _, l := range strings.Split(ej.ErrSalida, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return s + ": " + recortar(l, 100)
		}
	}
	return s
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
