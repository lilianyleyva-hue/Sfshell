package cerebro

import (
	"context"
	"sort"

	"nyxcodigo/internal/saber"
)

// Source quality (§9): every web snippet that cerebro checks moves the "dominio:<host>" counter of its
// site, verified (it compiled, ran and passed its checks) or rejected (it did not compile, failed go
// vet, crashed or failed the user's examples). The counters reorder what the web gives: the search
// results to read and the passages to quote. A site with no history keeps saber's fixed weights.

// claveDominio is the counter name of a URL's site ("" when the URL has no host).
func claveDominio(url string) string {
	d := saber.Dominio(url)
	if d == "" {
		return ""
	}
	return "dominio:" + d
}

// contarDominio records one checked snippet of url's site.
func (c *Cerebro) contarDominio(url string, verificado bool) {
	if clave := claveDominio(url); clave != "" && c.stats != nil {
		c.stats.Exito(clave, verificado)
	}
}

// confianzaDominio is 0.5 + the site's success rate: 1 with no history, between 0.5 and 1.5 with it.
func (c *Cerebro) confianzaDominio(url string) float64 {
	clave := claveDominio(url)
	if clave == "" || c.stats == nil {
		return 1
	}
	return 0.5 + c.stats.Tasa(clave)
}

// ordenarEnlaces reorders search results by their position in the results page, saber's weight of the
// site and what its code did before. Without history the order and the fixed weights are kept.
func (c *Cerebro) ordenarEnlaces(es []saber.Enlace) []saber.Enlace {
	type puntuado struct {
		e saber.Enlace
		p float64
	}
	ps := make([]puntuado, len(es))
	for i, e := range es {
		ps[i] = puntuado{e, c.confianzaDominio(e.URL) / (1 + 0.25*float64(i))}
	}
	sort.SliceStable(ps, func(a, b int) bool { return ps[a].p > ps[b].p })
	out := make([]saber.Enlace, len(ps))
	for i, p := range ps {
		out[i] = p.e
	}
	return out
}

// buscarDocumentos is saber.Web.BuscarDocumentos with the results reordered by source quality before
// the best 4 are read.
func (c *Cerebro) buscarDocumentos(ctx context.Context, consulta string) ([]saber.Documento, error) {
	enlaces, err := c.web.Buscar(ctx, consulta, 10)
	if err != nil {
		return nil, err
	}
	return c.web.TraerVarios(ctx, c.ordenarEnlaces(enlaces), 4), nil
}

// rankear is saber.Rankear with each passage's score multiplied by its site's confianzaDominio.
func (c *Cerebro) rankear(consulta string, docs []saber.Documento, n int) []saber.Pasaje {
	ps := saber.Rankear(consulta, docs, 3*n)
	for i := range ps {
		ps[i].Puntos *= c.confianzaDominio(ps[i].URL)
	}
	sort.SliceStable(ps, func(a, b int) bool { return ps[a].Puntos > ps[b].Puntos })
	if len(ps) > n {
		ps = ps[:n]
	}
	return ps
}
