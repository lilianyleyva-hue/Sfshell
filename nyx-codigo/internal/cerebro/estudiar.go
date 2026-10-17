package cerebro

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"nyxcodigo/internal/lengua"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/saber"
)

// estudiar (§4.14.2, N2): a long task (10 min budget) with its own trace: 1) warm the sandbox's build
// cache; 2) index the GOROOT examples; 3) read gobyexample (Web.Estudiar): its programs are vetted, and
// those that pass the automatic safety check and read no input are run once ("ejemplo comprobado");
// the hints mined from the code feed synthesis; 4) retry each pending question with 4× its budget and
// the web on; a solved one leaves pendientes. Report: "Aprendí N cosas nuevas: …".

const (
	estudiarMaxEjecutar   = 30               // gobyexample programs run per study session
	estudiarMaxPendientes = 20               // pending questions retried per study session
	estudiarMinPendiente  = 15 * time.Second // budget left needed to retry one more question
	estudiarReserva       = 10 * time.Second // kept for the final report
)

func pipeEstudiar(c *Cerebro, k *tarea) nucleo.Respuesta {
	k.raiz.Sub(nucleo.PasoPlan, "Plan: 1) preparar la caja segura 2) leer los ejemplos de Go 3) estudiar en la web 4) reintentar las preguntas pendientes").Info("")
	var aprendidas, notas, justif []string
	resueltas := 0

	// 1. warm the build cache
	if a, ok := c.ej.(interface{ Calentar(context.Context) error }); ok && c.ej != nil {
		n := k.raiz.Sub(nucleo.PasoPrueba, "1/4 Preparo la caja segura (compilo los paquetes de siempre)")
		if err := seguro(func() error { return a.Calentar(k.ctx) }); err != nil {
			n.Mal("No pude: %s", mensajeError(err))
			notas = append(notas, "No pude preparar la caja segura: "+mensajeError(err)+".")
		} else {
			n.Bien("")
			justif = append(justif, "Compilé los paquetes que más uso para que las próximas pruebas vayan rápido.")
		}
	} else {
		k.raiz.Sub(nucleo.PasoNota, "1/4 No tengo caja segura: no puedo compilar ni ejecutar").Info("")
		notas = append(notas, "No tengo caja segura, así que no he podido ejecutar nada.")
	}

	// 2. the GOROOT examples
	if s := c.estudiarIndice(k); s != "" {
		aprendidas = append(aprendidas, s)
	}

	// 3. the web
	if c.web == nil || !c.enLinea(k.ctx) {
		k.raiz.Sub(nucleo.PasoNota, "3/4 No tengo internet: hoy no estudio en la web").Info("")
		notas = append(notas, "No tenía internet, así que no he estudiado en la web.")
	} else if s, j := c.estudiarWeb(k); s != "" {
		aprendidas = append(aprendidas, s)
		justif = append(justif, j)
	}

	// 4. pending questions
	pend := c.mem.Pendientes()
	if len(pend) == 0 {
		k.raiz.Sub(nucleo.PasoNota, "4/4 No tengo preguntas pendientes").Info("")
	} else {
		n := k.raiz.Sub(nucleo.PasoIntento, "4/4 Reintento %s con más tiempo y con la web", cuantas(len(pend), "pregunta pendiente", "preguntas pendientes"))
		intentadas := 0
		for i, p := range pend {
			if intentadas == estudiarMaxPendientes || k.ctx.Err() != nil || k.restante() < estudiarMinPendiente+estudiarReserva {
				if len(pend)-i == 1 {
					notas = append(notas, "Me queda 1 pregunta pendiente para otra vez.")
				} else {
					notas = append(notas, fmt.Sprintf("Me quedan %d preguntas pendientes para otra vez.", len(pend)-i))
				}
				break
			}
			if estudiarSaltar(k, p) {
				continue
			}
			intentadas++
			n.Progreso("%d/%d", i+1, len(pend))
			np := n.Sub(nucleo.PasoIntento, "Reintento «%s»", recortar(primeraLinea(estudiarTexto(p)), 80))
			r, ok := c.estudiarReintentar(k, p)
			if ok {
				resueltas++
				if err := c.mem.ResolverPendiente(p.ID); err != nil {
					np.Detalle("No pude quitarla de pendientes: %s", mensajeError(err))
				}
				np.Bien("Ahora sí: %s", recortar(primeraLinea(r.Texto), 100))
				aprendidas = append(aprendidas, "a resolver «"+recortar(primeraLinea(estudiarTexto(p)), 60)+"»")
			} else {
				np.Mal("Todavía no: %s", recortar(primeraLinea(r.Texto), 100))
			}
		}
		if resueltas > 0 {
			n.Bien("Resolví %d de %d", resueltas, intentadas)
		} else {
			n.Info("No resolví ninguna (%s)", cuantas(intentadas, "intento", "intentos"))
		}
		if intentadas > 0 {
			justif = append(justif, "Reintenté "+cuantas(intentadas, "pregunta pendiente", "preguntas pendientes")+" con 4 veces más tiempo y con la web; cuento como resuelta la que pasa sus propias comprobaciones o se responde con una fuente.")
		}
	}

	r := nucleo.Respuesta{Comprobaciones: []nucleo.Comprobacion{}, Metodo: "estudiar", Exito: true}
	switch len(aprendidas) {
	case 0:
		r.Texto = "Esta vez no aprendí nada nuevo."
	case 1:
		r.Texto = "Aprendí 1 cosa nueva: " + aprendidas[0] + "."
	default:
		r.Texto = fmt.Sprintf("Aprendí %d cosas nuevas: %s.", len(aprendidas), strings.Join(aprendidas, "; "))
	}
	if resueltas > 0 {
		pregs := "preguntas"
		if resueltas == 1 {
			pregs = "pregunta"
		}
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("Aprendí a resolver %d %s que antes no sabía.", resueltas, pregs))
	}
	r.Parrafos = append(r.Parrafos, notas...)
	r.Justificacion = justif
	k.turno.Metodo = "estudiar"
	return r
}

// estudiarIndice indexes the GOROOT examples when there is no index yet.
func (c *Cerebro) estudiarIndice(k *tarea) string {
	if ix := c.indice(); ix != nil {
		if len(ix.Ejemplos) == 1 {
			k.raiz.Sub(nucleo.PasoRecuerdo, "2/4 Ya tengo indexado 1 ejemplo de la biblioteca estándar").Info("")
		} else {
			k.raiz.Sub(nucleo.PasoRecuerdo, "2/4 Ya tengo indexados %d ejemplos de la biblioteca estándar", len(ix.Ejemplos)).Info("")
		}
		return ""
	}
	n := k.raiz.Sub(nucleo.PasoRecuerdo, "2/4 Leo los ejemplos de la biblioteca estándar de Go")
	goroot, version := c.estudiarGoroot(k.ctx)
	if goroot == "" {
		n.Info("No sé dónde está Go instalado")
		return ""
	}
	var ix *saber.IndiceStd
	err := seguro(func() error {
		var e error
		ix, e = saber.IndexarGOROOT(goroot, version, c.cfg.DirDatos)
		return e
	})
	if err != nil || ix == nil {
		n.Mal("No pude: %s", mensajeError(err))
		return ""
	}
	c.PonerIndice(ix)
	n.Bien("Indexé %s", cuantas(len(ix.Ejemplos), "ejemplo", "ejemplos"))
	return cuantas(len(ix.Ejemplos), "ejemplo", "ejemplos") + " de la biblioteca estándar, con su salida"
}

// estudiarGoroot finds GOROOT and the Go version (from the sandbox, else from `go env`).
func (c *Cerebro) estudiarGoroot(ctx context.Context) (goroot, version string) {
	if a, ok := c.ej.(interface{ GoRoot() string }); ok && c.ej != nil {
		goroot = a.GoRoot()
	}
	if c.ej != nil {
		version = c.ej.Estado().GoVersion
	}
	if (goroot == "" || version == "") && c.goBin != "" {
		cctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
		defer cancelar()
		cmd := exec.CommandContext(cctx, c.goBin, "env", "GOROOT", "GOVERSION")
		cmd.Dir = os.TempDir()
		cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=", "GO111MODULE=off")
		if out, err := cmd.Output(); err == nil {
			lineas := strings.Split(strings.TrimSpace(string(out)), "\n")
			if goroot == "" && len(lineas) > 0 {
				goroot = strings.TrimSpace(lineas[0])
			}
			if version == "" && len(lineas) > 1 {
				version = strings.TrimSpace(lineas[1])
			}
		}
	}
	return goroot, version
}

// estudiarWeb reads gobyexample: vets the programs, runs the safe ones once, and stores mined hints.
func (c *Cerebro) estudiarWeb(k *tarea) (string, string) {
	n := k.raiz.Sub(nucleo.PasoBusqueda, "3/4 Estudio Go by Example")
	lim := k.restante() / 2
	if lim <= 0 {
		n.Info("No me queda tiempo")
		return "", ""
	}
	ctx, cancelar := context.WithTimeout(k.ctx, lim)
	defer cancelar()
	docs, err := c.web.Estudiar(ctx, func(hecho, total int, nota string) {
		n.Progreso("%d/%d %s", hecho, total, nota)
	})
	c.hablaLeerWeb(docs...) // Spanish pages also teach meanings and how to talk (habla.go)
	if len(docs) == 0 {
		n.Mal("No pude leer nada: %s", mensajeError(err))
		return "", ""
	}
	vetados, ejecutados, comprobados, pistas := 0, 0, 0, 0
	for i, d := range docs {
		if ctx.Err() != nil {
			break
		}
		n.Progreso("%d/%d Pruebo «%s»", i+1, len(docs), d.Titulo)
		var codigos []string
		for _, cd := range saber.Candidatas(d) {
			codigos = append(codigos, cd.Codigo)
			if !cd.EsPrograma || c.ej == nil {
				continue
			}
			if v := c.ej.Vet(ctx, cd.Codigo); !v.OK {
				continue
			}
			vetados++
			if ejecutados >= estudiarMaxEjecutar || len(c.ej.Revisar(cd.Codigo, nucleo.PermisoAuto)) > 0 || buscarLeeEntrada(cd.Codigo) {
				continue
			}
			ejecutados++
			if _, ok := c.estudiarCorrer(ctx, cd.Codigo); ok {
				comprobados++
			}
		}
		if len(codigos) > 0 {
			if p := saber.Minar(codigos); len(p.Llamadas)+len(p.Operadores)+len(p.Constantes) > 0 {
				if err := c.mem.GuardarPistas(d.Titulo, p); err == nil {
					pistas++
				}
			}
		}
	}
	paginas := cuantas(len(docs), "página", "páginas")
	if c.ej == nil {
		// without a sandbox nothing was vetted or run: say so instead of "0 programas pasan go vet"
		n.Bien("Leí %s (sin caja segura no pasé go vet ni ejecuté sus programas)", paginas)
		s := paginas + " de Go by Example (sin ejecutar sus programas: no tengo caja segura)"
		j := "Leí " + paginas + " de gobyexample.com sin comprobar sus programas (no tengo caja segura). Guardé pistas de " + cuantas(pistas, "página", "páginas") + " para construir funciones."
		return s, j
	}
	n.Bien("Leí %s: %s go vet, %s al ejecutarlos", paginas, cuantas(vetados, "programa pasa", "programas pasan"), cuantas(comprobados, "funciona", "funcionan"))
	s := paginas + " de Go by Example (" + cuantas(comprobados, "programa comprobado", "programas comprobados") + " al ejecutarlos)"
	j := "Leí " + paginas + " de gobyexample.com; pasé go vet a sus programas y ejecuté una vez los que pasan la revisión de seguridad automática (nunca con permisos extra). Guardé pistas de " + cuantas(pistas, "página", "páginas") + " para construir funciones."
	return s, j
}

// estudiarCorrer runs a web program once under PermisoAuto, with no input.
func (c *Cerebro) estudiarCorrer(ctx context.Context, fuente string) (string, bool) {
	p, _, err := c.ej.PrepararPrograma(ctx, fuente, nucleo.InstrNormal, nucleo.PermisoAuto)
	if err != nil {
		return "", false
	}
	defer p.Cerrar()
	ej, err := p.Correr(ctx, nucleo.CasoPrograma{})
	if err != nil || ej.Codigo != 0 || ej.Agotado || ej.SinCombustible {
		return ej.Salida, false
	}
	return ej.Salida, true
}

func estudiarTexto(p memoria.Pendiente) string {
	if strings.TrimSpace(p.Texto) != "" {
		return p.Texto
	}
	return p.Codigo
}

// estudiarSaltar: questions that would start another study session are not retried.
func estudiarSaltar(k *tarea, p memoria.Pendiente) bool {
	q := lengua.Analizar(p.Texto, p.Codigo, "", k.lex, nil)
	return len(q.Intenciones) > 0 && q.Intenciones[0].Tipo == nucleo.IEstudiar
}

// estudiarReintentar answers a pending question again, as a request of its own (own session and trace),
// with 4× the budget of its intent and the web on (pensarMas). It reports whether it is solved now.
func (c *Cerebro) estudiarReintentar(k *tarea, p memoria.Pendiente) (nucleo.Respuesta, bool) {
	q := lengua.Analizar(p.Texto, p.Codigo, "", k.lex, nil)
	base := presupuestoRazonar
	if len(q.Intenciones) > 0 {
		base = c.presupuestoDe(q.Intenciones[0].Tipo)
	}
	presupuesto := 4 * base
	if max := k.restante() - estudiarReserva; presupuesto > max {
		presupuesto = max
	}
	pet := nucleo.Peticion{
		Tarea:       k.p.Tarea + "-" + p.ID,
		Sesion:      "estudiar-" + p.ID,
		Texto:       p.Texto,
		Codigo:      p.Codigo,
		Presupuesto: presupuesto,
	}
	tr := nucleo.NuevaTraza(nil, nil)
	s := c.sesion(pet.Sesion)
	k2 := c.nuevaTarea(k.ctx, pet, s, tr)
	k2.pensarMas = true
	defer k2.cerrar()
	var r nucleo.Respuesta
	if err := seguro(func() error {
		r = c.rutear(k2)
		c.redactar(k2, &r)
		c.aprender(k2, &r)
		s.registrar(k2, r)
		return nil
	}); err != nil {
		r = nucleo.Respuesta{Texto: "error interno: " + err.Error(), Nivel: nucleo.NivelFallo}
	}
	tr.Terminar(&r, "terminada")
	return r, estudiarResuelta(r, k2.pendiente)
}

// estudiarResuelta: a retried question counts as solved when the answer succeeded, left nothing pending,
// and is backed by a check (examples, my reading, properties, maths) or, for buscar and doc_go, by a
// source (fuente is their correct level, §4.14.3).
func estudiarResuelta(r nucleo.Respuesta, pendiente string) bool {
	return r.Exito && pendiente == "" && (nivelVerificado(r.Nivel) || r.Nivel == nucleo.NivelFuente)
}
