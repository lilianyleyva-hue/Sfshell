package cerebro

import "nyxcodigo/internal/nucleo"

// pipeBuscar is the pipeline to search the web (buscar, §4.14.2).
// STUB: its owner replaces this file (see INTERNO.md). Until then it answers honestly that it cannot.
func pipeBuscar(c *Cerebro, k *tarea) nucleo.Respuesta {
	return c.enConstruccion(k, "buscar en internet")
}
