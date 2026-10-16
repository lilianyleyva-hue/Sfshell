package cerebro

import "nyxcodigo/internal/nucleo"

// pipeEstudiar is the pipeline to study mode (estudiar, §4.14.2).
// STUB: its owner replaces this file (see INTERNO.md). Until then it answers honestly that it cannot.
func pipeEstudiar(c *Cerebro, k *tarea) nucleo.Respuesta {
	return c.enConstruccion(k, "estudiar Go")
}
