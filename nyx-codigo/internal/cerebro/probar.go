package cerebro

import (
	"fmt"
	"strings"

	"nyxcodigo/internal/analisis"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/pruebas"
	"nyxcodigo/internal/reparar"
)

// pipeProbar is the pipeline to probar código (probar, §4.14.2; follow-ups probar_con and vacia): the
// same spec and generated cases as reparar, run on the user's code, plus a downloadable _test.go, go vet
// and the static findings. A failure is shown as its minimized case: "Falla con [-1]: esperaba 0, da -1".
func pipeProbar(c *Cerebro, k *tarea) (resp nucleo.Respuesta) {
	codigo := c.repCodigo(k, "Pega el código que quieres que pruebe")
	if codigo == "" {
		return repSinCodigo("probarlo")
	}
	plan := k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) ver si compila 2) elegir la función 3) preparar casos 4) probarla y escribir el _test.go")
	plan.Info("")
	src := repCompletar(codigo)
	if src.sinMain {
		k.raiz.Nota("Es `package main` sin `func main`: lo trato como un archivo de funciones")
		defer func() { src.volverMain(&resp) }()
	}

	if errs, _ := reparar.ChequeoTipos(src.fuente); len(errs) > 0 {
		n := k.raiz.Sub(nucleo.PasoError, "No compila: %d errores", len(errs))
		var errores []string
		for _, e := range errs {
			t := reparar.Traducir(src.linea(e))
			errores = append(errores, t)
			n.Detalle("%s", t)
		}
		n.Mal("")
		return nucleo.Respuesta{
			Texto:          "No compila, así que todavía no puedo probarlo.",
			Parrafos:       repParrafosCambios(errores, nil, nil),
			Comprobaciones: []nucleo.Comprobacion{},
			Nivel:          nucleo.NivelSinComprobar,
			Sugerencias:    []string{"Arréglalo"},
			Metodo:         "pruebas",
		}
	}

	inf, _ := analisis.Analizar(src.fuente)
	avisos := prbAvisos(src, inf)
	fn, hay := c.repElegir(k, src.fuente, inf)
	if !hay {
		r := prbSinPruebas("No hay ninguna función que probar: es un programa. Puedo ejecutarlo.", avisos)
		r.Sugerencias = []string{"Ejecútalo"}
		return r
	}
	if fn.Firma == nil || !fn.Firma.Probable() {
		motivo := fn.NoProbable
		if motivo == "" {
			motivo = "sus tipos no los sé generar"
		}
		return prbSinPruebas("No sé probar «"+fn.Nombre+"» automáticamente: "+motivo+".", avisos)
	}
	f := *fn.Firma
	k.q = repConFirma(k.q, &f)
	if c.ej == nil {
		k.raiz.Sub(nucleo.PasoNota, "No tengo caja segura: no puedo ejecutar las pruebas").Info("")
		return prbSinPruebas("No puedo ejecutar pruebas: no tengo caja segura (falta Go).", avisos)
	}
	permiso := nucleo.PermisoAuto
	if v := c.ej.Revisar(src.fuente, nucleo.PermisoAuto); len(v) > 0 {
		pr, ok := c.ejcPermiso(k, src.fuente, v, "probarlo")
		if !ok {
			r := prbSinPruebas(pr.Texto, avisos)
			r.Parrafos = append(pr.Parrafos, r.Parrafos...)
			r.Riesgos = pr.Riesgos
			if len(r.Riesgos) > 0 {
				r.Codigo = src.fuente // what the dialog sends back with confirmado:true
			}
			return r
		}
		permiso = nucleo.PermisoUsuario
	}

	e := c.repEspec(k, f, fn.Nombre)
	defer e.cerrar()
	repRecordarEspec(k, e)
	var tuyo *nucleo.Caso
	if ent, ok := prbEntradaPedida(k, f); ok {
		cs := nucleo.Caso{Entradas: ent, Origen: nucleo.OrigenUsuario, Nota: "tu entrada"}
		tuyo = &cs
		e.extra = append(e.extra, cs)
	} else if strings.TrimSpace(k.probarCon) != "" {
		k.raiz.Nota("No sé leer «%s» como entrada de %s: pruebo con otros datos", k.probarCon, f.Go())
	}
	casos := e.casos(c, k)

	if vet := c.ej.Vet(k.ctx, src.fuente); len(vet.Errores) > 0 {
		for _, v := range vet.Errores {
			avisos = append(avisos, "go vet: "+reparar.Traducir(src.linea(v)))
		}
	}
	b, rs, err := c.repCorrerCasos(k, k.raiz, fmt.Sprintf("Pruebo «%s» con %d casos", f.Nombre, len(casos)), src.fuente, e, casos, permiso)
	if err != nil {
		return prbSinPruebas("No pude probarlo: "+mensajeError(err)+".", avisos)
	}
	defer b.Cerrar()
	comps := e.comprobaciones(k, casos, rs)
	fallos := repFallos(rs, len(casos))

	r := nucleo.Respuesta{
		Comprobaciones: comps,
		Nivel:          pruebas.NivelGlobal(comps),
		Tablas:         []nucleo.Tabla{pruebas.Tabla(f, casos, rs, 0)},
		CodigoTest:     pruebas.ArchivoTest(f, prbCasosTest(casos), src.paqueteUsuario()),
		Metodo:         "pruebas",
		Exito:          true,
	}
	if len(fallos) == 0 {
		r.Texto = fmt.Sprintf("Le hice %d pruebas a «%s»: todas bien.", len(casos), f.Nombre)
	} else {
		r.Texto = fmt.Sprintf("Le hice %d pruebas a «%s» y falla en %d.", len(casos), f.Nombre, len(fallos))
		if caso, rc, ok := c.repContraejemploCaso(k, b, e, casos, rs, fallos); ok {
			r.Parrafos = append(r.Parrafos, mayusculaInicial(repFrase(e, caso, rc, false))+".")
		}
		r.Sugerencias = []string{"Arréglalo"}
	}
	if tuyo != nil {
		for i, cs := range casos {
			if repClave(cs.Entradas) == repClave(tuyo.Entradas) && i < len(rs) {
				r.Parrafos = append(r.Parrafos, prbFraseEntrada(f, cs, rs[i]))
				break
			}
		}
	}
	if e.vacia() {
		r.Parrafos = append(r.Parrafos, "No sé qué debe dar, así que solo comprobé que no se rompe y que siempre da lo mismo. Dame un ejemplo (entrada -> salida) y lo compruebo.")
	}
	if len(avisos) > 0 {
		r.Parrafos = append(r.Parrafos, "Avisos:\n• "+strings.Join(prbRecortarLista(avisos, 8), "\n• "))
	}
	nombreTest := strings.ToLower(f.Nombre) + "_test.go"
	r.Parrafos = append(r.Parrafos, "Te dejo las pruebas en un archivo ("+nombreTest+"): guárdalo junto a tu código y escribe `go test`.")
	r.Justificacion = repJustificacion(k, e, casos, comps, nil)
	return r
}

// prbSinPruebas: nothing could be run; the static findings are still shown.
func prbSinPruebas(texto string, avisos []string) nucleo.Respuesta {
	r := nucleo.Respuesta{
		Texto:          texto,
		Comprobaciones: []nucleo.Comprobacion{},
		Nivel:          nucleo.NivelSinComprobar,
		Metodo:         "pruebas",
	}
	if len(avisos) > 0 {
		r.Parrafos = append(r.Parrafos, "Mirando el código sin ejecutarlo:\n• "+strings.Join(prbRecortarLista(avisos, 8), "\n• "))
	}
	return r
}

// prbAvisos are analisis' findings, in the user's line numbers.
func prbAvisos(src repFuente, inf analisis.Informe) []string {
	var out []string
	for _, h := range inf.Hallazgos {
		s := h.Mensaje
		if h.Linea > 0 {
			s = fmt.Sprintf("Línea %d: %s", src.lineaUsuario(h.Linea), h.Mensaje)
		}
		if h.Consejo != "" {
			s += " " + h.Consejo
		}
		out = append(out, s)
	}
	return out
}

func prbRecortarLista(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string(nil), xs[:n]...), fmt.Sprintf("(y %d más)", len(xs)-n))
}

// prbEntradaPedida: the input of "pruébala con [3,3]" or "y si la lista está vacía".
func prbEntradaPedida(k *tarea, f nucleo.Firma) ([]nucleo.Valor, bool) {
	if k.vacia {
		ent := f.Entradas()
		if len(ent) == 0 {
			return nil, false
		}
		out := make([]nucleo.Valor, len(ent))
		for i, t := range ent {
			out[i] = nucleo.ValorCero(t)
		}
		return out, true
	}
	if strings.TrimSpace(k.probarCon) == "" {
		return nil, false
	}
	return repLeerEntradas(k.probarCon, f)
}

// prbFraseEntrada says what the code does with the input the user asked for (observed, not judged).
func prbFraseEntrada(f nucleo.Firma, cs nucleo.Caso, r nucleo.ResultadoCaso) string {
	ent := repEntradas(f, cs.Entradas)
	switch {
	case !r.Ejecutado:
		return "Con " + ent + " no llegó a ejecutarse."
	case r.SinCombustible || r.Agotado:
		return "Con " + ent + " no termina (parece un bucle sin fin)."
	case r.Panico != "" || r.Caida:
		return "Con " + ent + " se rompe: " + reparar.TraducirPanico(primeraLinea(r.Panico))
	}
	s := "Con " + ent + " da " + repValores(r.Obtenido, f.Res) + "."
	if cs.ConEsperado() && !repIgualValores(r.Obtenido, cs.Esperado) {
		s += " Según " + cs.Nota + " debería dar " + repValores(cs.Esperado, f.Res) + "."
	}
	return s
}

// prbCasosTest: every case with an expected value plus a sample of the generated ones (as no-panic
// checks), so the _test.go stays readable.
func prbCasosTest(casos []nucleo.Caso) []nucleo.Caso {
	var out []nucleo.Caso
	sin := 0
	for _, c := range casos {
		if c.ConEsperado() {
			if len(out) < 60 {
				out = append(out, c)
			}
			continue
		}
		if sin < 20 {
			sin++
			out = append(out, c)
		}
	}
	return out
}
