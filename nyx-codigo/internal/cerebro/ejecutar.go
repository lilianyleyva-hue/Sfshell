package cerebro

import (
	"regexp"
	"strconv"
	"strings"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/recetas"
	"nyxcodigo/internal/reparar"
)

// pipeEjecutar is the pipeline to ejecutar código (ejecutar, §4.14.2, §7): code that passes the automatic
// safety check runs at once; otherwise the answer carries the risks (Respuesta.Riesgos) and nothing runs
// until the user confirms (p.Confirmado), and grave violations never run. A function without main is
// wrapped into a program (recetas.Envolver); "con entrada 5" becomes the stdin "5⏎".
func pipeEjecutar(c *Cerebro, k *tarea) nucleo.Respuesta {
	codigo := c.repCodigo(k, "Pega el código que quieres que ejecute")
	if codigo == "" {
		return repSinCodigo("ejecutarlo")
	}
	if c.ej == nil {
		k.raiz.Sub(nucleo.PasoNota, "No tengo caja segura para ejecutar código").Info("")
		return ejcRespuesta("No puedo ejecutarlo: no tengo caja segura (falta Go).", "")
	}
	src := repCompletar(codigo)
	if errs, _ := reparar.ChequeoTipos(src.fuente); len(errs) > 0 {
		n := k.raiz.Sub(nucleo.PasoError, "No compila: %d errores", len(errs))
		var errores []string
		for _, e := range errs {
			t := reparar.Traducir(src.linea(e))
			errores = append(errores, t)
			n.Detalle("%s", t)
		}
		n.Mal("")
		r := ejcRespuesta("No compila, así que no lo puedo ejecutar.", "")
		r.Parrafos = repParrafosCambios(errores, nil, nil)
		r.Sugerencias = []string{"Arréglalo"}
		return r
	}

	programa := src.fuente
	caso := nucleo.CasoPrograma{Entrada: k.p.Entrada, Args: k.p.Args}
	pedida := ejcEntradaDelTexto(k)
	envuelto := false
	if src.paquete != "main" || !reFuncMain.MatchString(src.fuente) {
		inf, _ := analisis.Analizar(src.fuente)
		fn, hay := c.repElegir(k, src.fuente, inf)
		if !hay || fn.Firma == nil {
			return ejcRespuesta("No hay nada que ejecutar: no tiene func main ni una función que yo sepa llamar.", "")
		}
		n := k.raiz.Sub(nucleo.PasoPlan, "No tiene func main: envuelvo «%s» en un programa que lee su entrada", fn.Nombre)
		f := *fn.Firma
		k.q = repConFirma(k.q, &f)
		sola, err := analisis.Funcion(src.fuente, fn.Nombre)
		if err != nil {
			sola = src.fuente
		}
		prog, aCaso, err := recetas.Envolver(sola, f)
		if err != nil {
			n.Mal("No sé convertirla en programa: %s", err)
			return ejcRespuesta("No sé convertir «"+fn.Nombre+"» en un programa: "+err.Error()+".", "")
		}
		n.Bien("")
		programa, envuelto = prog, true
		if caso.Entrada == "" && pedida == "" && len(f.Entradas()) > 0 {
			if resp, ok := c.preguntarLibre(k, "¿Con qué datos la ejecuto? Escribe la entrada de "+f.Go(), nil, ""); ok {
				pedida = strings.TrimSpace(resp)
			}
		}
		if caso.Entrada == "" && pedida != "" {
			if ent, ok := repLeerEntradas(pedida, f); ok {
				if cp, ok := aCaso(nucleo.Caso{Entradas: ent}); ok {
					caso.Entrada = cp.Entrada
				}
			}
			if caso.Entrada == "" {
				caso.Entrada = pedida + "\n"
			}
		}
	} else if caso.Entrada == "" && pedida != "" {
		caso.Entrada = pedida + "\n"
	}

	permiso := nucleo.PermisoAuto
	if v := c.ej.Revisar(programa, nucleo.PermisoAuto); len(v) > 0 {
		if !k.p.Confirmado {
			n := k.raiz.Sub(nucleo.PasoPregunta, "No lo ejecuto sin tu permiso")
			for _, x := range v {
				n.Detalle("%s", ejcRiesgo(x))
			}
			n.Info("")
			r := ejcRespuesta("Antes de ejecutarlo necesito tu permiso:", ejcCodigo(envuelto, programa))
			for _, x := range v {
				r.Parrafos = append(r.Parrafos, mayusculaInicial(ejcRiesgo(x))+".")
			}
			r.Parrafos = append(r.Parrafos, "Si lo confirmas, lo ejecuto en la caja segura, sin red y con límites de tiempo y memoria.")
			r.Riesgos = v
			return r
		}
		var graves []nucleo.Violacion
		for _, x := range c.ej.Revisar(programa, nucleo.PermisoUsuario) {
			if x.Grave {
				graves = append(graves, x)
			}
		}
		if len(graves) > 0 {
			k.raiz.Sub(nucleo.PasoNota, "Esto no lo ejecuto nunca: %s", graves[0].Que).Mal("")
			r := ejcRespuesta("No lo ejecuto: "+graves[0].Que+".", ejcCodigo(envuelto, programa))
			r.Riesgos = graves
			return r
		}
		permiso = nucleo.PermisoUsuario
		k.raiz.Sub(nucleo.PasoNota, "Me diste permiso: lo ejecuto en la caja segura").Info("")
	}

	ej, err := c.repCorrerPrograma(k, k.raiz, programa, caso, permiso)
	if err != nil {
		return ejcRespuesta("No pude ejecutarlo: "+mensajeError(err)+".", ejcCodigo(envuelto, programa))
	}
	cp, frase := repResultadoPrograma(ej, caso.Entrada)
	r := ejcRespuesta(primeraLinea(frase), ejcCodigo(envuelto, programa))
	if resto := strings.TrimSpace(strings.TrimPrefix(frase, primeraLinea(frase))); resto != "" {
		r.Parrafos = append(r.Parrafos, resto)
	}
	r.Exito = true
	if cp != nil {
		r.Comprobaciones = []nucleo.Comprobacion{*cp}
		r.Nivel = cp.Nivel
	}
	if envuelto {
		r.Parrafos = append(r.Parrafos, "Lo envolví en un programa que lee la entrada. Guárdalo como `main.go` y escribe `go run main.go`.")
	}
	return r
}

func ejcRespuesta(texto, codigo string) nucleo.Respuesta {
	return nucleo.Respuesta{
		Texto:          texto,
		Codigo:         codigo,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "ejecutar",
	}
}

// ejcCodigo: the wrapped program is shown (it is what ran); the user's own code stays in the session.
func ejcCodigo(envuelto bool, programa string) string {
	if envuelto {
		return programa
	}
	return ""
}

func ejcRiesgo(v nucleo.Violacion) string {
	if v.Linea > 0 {
		return "línea " + strconv.Itoa(v.Linea) + ": " + v.Que
	}
	return v.Que
}

var (
	reConEntrada = regexp.MustCompile(`(?i)\bcon\s+(?:la\s+|esta\s+)?entrada\s*:?\s*(.+?)\s*$`)
	reConDatos   = regexp.MustCompile(`(?i)\bcon\s+(?:los\s+datos\s+|el\s+valor\s+)?(.+?)\s*$`)
)

// ejcEntradaDelTexto reads "con entrada 5" (or "con [3,1,2]") from the request text.
func ejcEntradaDelTexto(k *tarea) string {
	texto := k.q.Resto
	if strings.TrimSpace(texto) == "" {
		texto = k.p.Texto
	}
	texto = primeraLinea(texto)
	limpiar := func(s string) string {
		s = strings.TrimSpace(strings.TrimRight(s, ":.;"))
		s = strings.Trim(s, "«»“”")
		return strings.ReplaceAll(s, "⏎", "\n")
	}
	if m := reConEntrada.FindStringSubmatch(texto); m != nil {
		return limpiar(m[1])
	}
	if m := reConDatos.FindStringSubmatch(texto); m != nil {
		s := limpiar(m[1])
		if s != "" && strings.ContainsAny(s[:1], "[(\"'0123456789-") {
			return s
		}
	}
	return ""
}
