package cerebro

import "nyxcodigo/internal/nucleo"

// pipeEjecutar is the pipeline to ejecutar código (ejecutar, §4.14.2).
// STUB: its owner replaces this file (see INTERNO.md). Until then it answers honestly that it cannot.
func pipeEjecutar(c *Cerebro, k *tarea) nucleo.Respuesta {
	return c.enConstruccion(k, "ejecutar código")
}
