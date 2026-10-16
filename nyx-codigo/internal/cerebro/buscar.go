package cerebro

import (
	"fmt"
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
	nb.Bien("Leí %d páginas", len(docs))
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
		np.Bien("%d frases", len(pasajes))
	} else {
		np.Info("Ninguna frase responde bien")
	}

	// code
	codigo, comp, frase, url := c.buscarCodigo(k, docs)
	if codigo != "" {
		r.Codigo = codigo
		if comp != nil {
			r.Comprobaciones = append(r.Comprobaciones, *comp)
		}
		if frase != "" {
			r.Parrafos = append(r.Parrafos, frase)
		}
		ponerFuente(url, "código de la web (licencia del sitio)")
	}

	// facts
	if n := c.buscarHechos(k, docs); n > 0 {
		r.Aprendido = fmt.Sprintf("Guardé %d datos con su fuente.", n)
	}

	switch {
	case len(pasajes) > 0:
		r.Texto = "Esto es lo que encontré sobre «" + consulta + "»:"
	case codigo != "":
		r.Texto = "Encontré este código para «" + consulta + "»:"
	default:
		k.pendiente = "búsqueda sin respuesta clara"
		r.Texto = "Leí " + fmt.Sprint(len(docs)) + " páginas sobre «" + consulta + "», pero ninguna lo explica claramente."
		r.Nivel = nucleo.NivelSinComprobar
		for _, d := range docs {
			ponerFuente(d.URL, "")
		}
		return r
	}
	r.Comprobaciones = append(r.Comprobaciones, nucleo.Comprobacion{Nivel: nucleo.NivelFuente, Pasan: 1, Total: 1,
		Texto: "Según " + fmt.Sprint(len(r.Fuentes)) + " fuentes de la web", Detalle: "consulta: " + consulta})
	r.Nivel = nucleo.NivelFuente
	r.Exito = true
	r.Justificacion = []string{
		"Busqué «" + consulta + "» y leí las " + fmt.Sprint(len(docs)) + " mejores páginas.",
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

// buscarCodigo tries the code candidates of the documents in order and returns the first that runs
// (or, failing that, the first that compiles) with its badge, a sentence and its URL.
func (c *Cerebro) buscarCodigo(k *tarea, docs []saber.Documento) (string, *nucleo.Comprobacion, string, string) {
	var cands []saber.Candidata
	for _, d := range docs {
		cands = append(cands, saber.Candidatas(d)...)
	}
	if len(cands) == 0 {
		return "", nil, "", ""
	}
	if len(cands) > buscarMaxCandidatas {
		cands = cands[:buscarMaxCandidatas]
	}
	n := k.raiz.Sub(nucleo.PasoPrueba, "Pruebo el código que encontré (%d trozos)", len(cands))
	if c.ej == nil {
		n.Info("No tengo caja segura: no lo compilo ni lo ejecuto")
		cd := cands[0]
		return cd.Codigo, &nucleo.Comprobacion{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar", Detalle: cd.URL},
			"No he podido compilarlo ni probarlo: no tengo caja segura.", cd.URL
	}
	var compila *saber.Candidata
	var motivoCompila string
	for i := range cands {
		cd := cands[i]
		if k.ctx.Err() != nil {
			break
		}
		viol := c.ej.Revisar(cd.Codigo, nucleo.PermisoAuto)
		switch {
		case cd.EsPrograma && len(viol) == 0 && !buscarLeeEntrada(cd.Codigo):
			if salida, ok := c.buscarCorrerPrograma(k, n, cd.Codigo); ok {
				n.Bien("Uno funciona")
				return cd.Codigo, &nucleo.Comprobacion{Nivel: nucleo.NivelSinFallos, Pasan: 1, Total: 1,
						Texto: "Lo ejecuté: terminó sin errores", Detalle: cd.URL},
					"Lo ejecuté y escribió: «" + docgoSalida(salida) + "».", cd.URL
			}
		case !cd.EsPrograma && cd.Firma != nil && cd.Firma.Probable() && len(viol) == 0:
			f := *cd.Firma
			casos := c.generarCasos(k, f, nil, 0, buscarCasos)
			if len(casos) == 0 {
				break
			}
			v, err := c.verificar(k, n, cd.Codigo, f, casos, nil, nucleo.OpcionesProbar{Repetir: 2})
			if err == nil && len(v.Fallos) == 0 && len(v.Comprobaciones) > 0 {
				comp := v.Comprobaciones[0]
				for _, cp := range v.Comprobaciones {
					if cp.Nivel == nucleo.NivelSinFallos {
						comp = cp
					}
				}
				n.Bien("Uno funciona")
				return cd.Codigo, &comp, fmt.Sprintf("Lo probé con %d entradas generadas: no falla y da siempre lo mismo (no sé qué debe dar, así que no compruebo el resultado).", len(casos)), cd.URL
			}
		}
		if compila == nil {
			comp := c.ej.Compilar(k.ctx, cd.Codigo)
			if comp.OK {
				compila = &cands[i]
				switch {
				case len(viol) > 0:
					motivoCompila = "por seguridad no lo ejecuto solo (" + viol[0].Que + ")"
				case cd.EsPrograma && buscarLeeEntrada(cd.Codigo):
					motivoCompila = "lee datos del teclado"
				case cd.Firma == nil || !cd.Firma.Probable():
					motivoCompila = "no sé con qué datos llamarlo"
				default:
					motivoCompila = "falló al probarlo"
				}
			}
		}
	}
	if compila != nil {
		n.Info("Compila, pero no lo he ejecutado")
		return compila.Codigo, &nucleo.Comprobacion{Nivel: nucleo.NivelSinComprobar, Texto: "Sin comprobar · compila", Detalle: compila.URL},
			"Este código compila, pero no lo he ejecutado: " + motivoCompila + ".", compila.URL
	}
	n.Mal("Ninguno compila")
	return "", nil, "", ""
}

// buscarCorrerPrograma runs a program once under PermisoAuto, with no input.
func (c *Cerebro) buscarCorrerPrograma(k *tarea, nodo *nucleo.Nodo, fuente string) (string, bool) {
	p, comp, err := c.ej.PrepararPrograma(k.ctx, fuente, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		nodo.Sub(nucleo.PasoError, "No compila: %s", resumenErrores(comp, err)).Mal("")
		return "", false
	}
	defer p.Cerrar()
	ej, err := p.Correr(k.ctx, nucleo.CasoPrograma{})
	if err != nil || ej.Codigo != 0 || ej.Agotado || ej.SinCombustible {
		nodo.Sub(nucleo.PasoError, "Al ejecutarlo falla").Mal("")
		return ej.Salida, false
	}
	nodo.Sub(nucleo.PasoPrueba, "Lo ejecuté: %s", recortar(docgoSalida(ej.Salida), 120)).Bien("")
	return ej.Salida, true
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
	n.Bien("Guardé %d", guardados)
	return guardados
}
