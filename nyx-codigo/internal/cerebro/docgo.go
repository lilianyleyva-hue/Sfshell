package cerebro

import "nyxcodigo/internal/nucleo"

// pipeDocGo is the pipeline to answer questions about Go (doc_go, §4.14.2).
// STUB: its owner replaces this file (see INTERNO.md). Until then it answers honestly that it cannot.
func pipeDocGo(c *Cerebro, k *tarea) nucleo.Respuesta {
	return c.enConstruccion(k, "responder preguntas de Go")
}
