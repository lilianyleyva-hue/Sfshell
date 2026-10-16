package saber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// Search (DuckDuckGo HTML) and the site adapters: gobyexample, pkg.go.dev, go.dev and Wikipedia.

// Enlace is one search result.
type Enlace struct {
	URL, Titulo, Resumen, Dominio string
	Peso                          float64
}

// Tema is one gobyexample topic.
type Tema struct{ Titulo, URL string }

// Resumen is a Wikipedia page summary.
type Resumen struct{ Titulo, Texto, URL string }

// ProgresoEstudio reports study progress: hecho of total pages, with a short Spanish note.
type ProgresoEstudio func(hecho, total int, nota string)

// Dominio returns the host of u without "www." ("" if u does not parse).
func Dominio(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(p.Hostname()), "www.")
}

// PesoDominio is the source weight of a URL: gobyexample 1.0, go.dev/pkg.go.dev 1.0, stackoverflow 0.8,
// geeksforgeeks/golangbyexample/programiz/freecodecamp 0.6, others 0.4.
func PesoDominio(u string) float64 {
	d := Dominio(u)
	es := func(x string) bool { return d == x || strings.HasSuffix(d, "."+x) }
	switch {
	case es("gobyexample.com"), es("go.dev"), es("golang.org"):
		return 1.0
	case es("stackoverflow.com"):
		return 0.8
	case es("geeksforgeeks.org"), es("golangbyexample.com"), es("programiz.com"), es("freecodecamp.org"):
		return 0.6
	}
	return 0.4
}

// ExtraerEnlacesDDG reads a DuckDuckGo HTML results page: every a.result__a, its target decoded from
// the uddg parameter, its title and the snippet that follows it. Ads and duplicates are skipped; the
// page order is kept.
func ExtraerEnlacesDDG(pagina []byte) []Enlace {
	l := &lectorHTML{s: string(pagina)}
	var out []Enlace
	vistos := map[string]bool{}
	type captura struct {
		activo    bool
		nombre    string
		prof      int
		sb        strings.Builder
		esResumen bool
	}
	var c captura
	ultimo := -1
	for n := 0; n < maxTokens; n++ {
		t, ok := l.siguiente()
		if !ok {
			break
		}
		switch t.tipo {
		case tokInicio:
			if c.activo {
				if t.nombre == c.nombre && !vacios[t.nombre] && !t.auto {
					c.prof++
				}
				continue
			}
			switch {
			case t.nombre == "a" && t.tieneClase("result__a"):
				href, _ := t.attr("href")
				destino := destinoDDG(href)
				ultimo = -1
				if destino == "" || vistos[destino] {
					c = captura{activo: true, nombre: "a", prof: 1} // capture and discard
					continue
				}
				vistos[destino] = true
				out = append(out, Enlace{URL: destino, Dominio: Dominio(destino), Peso: PesoDominio(destino)})
				ultimo = len(out) - 1
				c = captura{activo: true, nombre: "a", prof: 1}
			case t.tieneClase("result__snippet") && ultimo >= 0 && !vacios[t.nombre] && !t.auto:
				c = captura{activo: true, nombre: t.nombre, prof: 1, esResumen: true}
			}
		case tokFin:
			if !c.activo || t.nombre != c.nombre {
				continue
			}
			c.prof--
			if c.prof > 0 {
				continue
			}
			txt := colapsar(c.sb.String())
			if ultimo >= 0 {
				if c.esResumen {
					out[ultimo].Resumen = txt
				} else {
					out[ultimo].Titulo = txt
				}
			}
			c = captura{}
		case tokTexto:
			if c.activo {
				c.sb.WriteString(html.UnescapeString(t.texto))
			}
		}
	}
	return out
}

// destinoDDG turns a result href into the real target: the uddg parameter of "//duckduckgo.com/l/?uddg=…",
// or the href itself when it is already an absolute http(s) URL outside DuckDuckGo. Ads give "".
func destinoDDG(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if strings.HasSuffix(u.Hostname(), "duckduckgo.com") || u.Host == "" {
		if strings.HasPrefix(u.Path, "/y.js") {
			return "" // ad
		}
		crudo := ""
		for _, parte := range strings.Split(u.RawQuery, "&") {
			if strings.HasPrefix(parte, "uddg=") {
				crudo = parte[len("uddg="):]
				break
			}
		}
		if crudo == "" {
			return ""
		}
		dest, err := url.QueryUnescape(crudo)
		if err != nil {
			return ""
		}
		href = dest
		u, err = url.Parse(dest)
		if err != nil {
			return ""
		}
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	if strings.HasSuffix(u.Hostname(), "duckduckgo.com") {
		return ""
	}
	return href
}

// Buscar asks DuckDuckGo (HTML version) and returns up to max results, ordered by source weight (ties
// keep DuckDuckGo's order). Results are cached 7 days. At most 1 request every 2 s and 200 per day.
func (w *Web) Buscar(ctx context.Context, consulta string, max int) ([]Enlace, error) {
	if w.cfg.SinInternet {
		return nil, nucleo.ErrSinInternet
	}
	consulta = strings.Join(strings.Fields(consulta), " ")
	if consulta == "" {
		return nil, errors.New("saber: la búsqueda está vacía")
	}
	if max <= 0 {
		max = 10
	}
	clave := "ddg:" + consulta
	var enlaces []Enlace
	if e, ok := w.cache.leer(clave, TTLBusqueda); ok {
		enlaces = e.Enlaces
	} else {
		sep := "?"
		if strings.Contains(w.cfg.BaseDDG, "?") {
			sep = "&"
		}
		r, err := w.obtener(ctx, w.cfg.BaseDDG+sep+"q="+url.QueryEscape(consulta), limiteDDG, false)
		if err != nil {
			return nil, err
		}
		enlaces = ExtraerEnlacesDDG(r.cuerpo)
		if len(enlaces) == 0 {
			// Nothing is cached: an empty page is usually DuckDuckGo's bot check (HTTP 202) or a
			// passing failure, and caching it would hide the query for a week.
			if r.estado == 202 { // 202 Accepted
				return nil, ErrBuscadorOcupado
			}
			return nil, nil
		}
		w.cache.escribir(clave, entradaCache{Enlaces: enlaces})
	}
	out := append([]Enlace(nil), enlaces...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Peso > out[j].Peso })
	if len(out) > max {
		out = out[:max]
	}
	return out, nil
}

// BuscarDocumentos searches and fetches the top 4 documents, two at a time.
func (w *Web) BuscarDocumentos(ctx context.Context, consulta string) ([]Enlace, []Documento, error) {
	enlaces, err := w.Buscar(ctx, consulta, 10)
	if err != nil {
		return nil, nil, err
	}
	return enlaces, w.TraerVarios(ctx, enlaces, 4), nil
}

// ---- gobyexample ----

var reTemaGBE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// TemasGoByExample lists the topics on the gobyexample index page (cached 30 days).
func (w *Web) TemasGoByExample(ctx context.Context) ([]Tema, error) {
	if w.cfg.SinInternet {
		return nil, nucleo.ErrSinInternet
	}
	base := w.cfg.BaseGoByExample
	clave := "temas:" + base
	if e, ok := w.cache.leer(clave, TTLDocumento); ok && len(e.Temas) > 0 {
		return e.Temas, nil
	}
	r, err := w.obtener(ctx, base, limite{}, false)
	if err != nil {
		return nil, err
	}
	temas := temasDe(r.cuerpo, base)
	if len(temas) == 0 {
		return nil, fmt.Errorf("%w: la página de gobyexample no tiene temas", ErrNoEncontrado)
	}
	w.cache.escribir(clave, entradaCache{Temas: temas})
	return temas, nil
}

func temasDe(pagina []byte, base string) []Tema {
	bu, err := url.Parse(base)
	if err != nil {
		return nil
	}
	var out []Tema
	vistos := map[string]bool{}
	enlaces := enlacesHTML(pagina)
	soloListas := false
	for _, e := range enlaces {
		if e.enLista {
			soloListas = true
			break
		}
	}
	for _, e := range enlaces {
		if soloListas && !e.enLista {
			continue
		}
		h := strings.TrimPrefix(strings.TrimSpace(e.href), "./")
		h = strings.TrimSuffix(h, ".html")
		if !reTemaGBE.MatchString(h) || e.texto == "" || vistos[h] {
			continue
		}
		vistos[h] = true
		out = append(out, Tema{Titulo: e.texto, URL: bu.ResolveReference(&url.URL{Path: h}).String()})
	}
	return out
}

// Estudiar fetches every gobyexample page (cached 30 days) and returns their documents. Pages that
// fail are skipped; ctx cancellation returns what was fetched so far together with ctx.Err().
func (w *Web) Estudiar(ctx context.Context, progreso ProgresoEstudio) ([]Documento, error) {
	temas, err := w.TemasGoByExample(ctx)
	if err != nil {
		return nil, err
	}
	if progreso == nil {
		progreso = func(int, int, string) {}
	}
	var docs []Documento
	fallos := 0
	for i, t := range temas {
		if ctx.Err() != nil {
			return docs, ctx.Err()
		}
		d, err := w.Traer(ctx, t.URL)
		if err != nil {
			if ctx.Err() != nil {
				return docs, ctx.Err()
			}
			fallos++
			progreso(i+1, len(temas), "No pude leer «"+t.Titulo+"»")
			continue
		}
		if d.Titulo == "" {
			d.Titulo = t.Titulo
		}
		docs = append(docs, d)
		progreso(i+1, len(temas), "Leo «"+t.Titulo+"»")
	}
	if len(docs) == 0 && fallos > 0 {
		return nil, fmt.Errorf("saber: no pude leer ninguna página de gobyexample (%d fallos)", fallos)
	}
	return docs, nil
}

// ---- pkg.go.dev / go.dev ----

var reSimbolo = regexp.MustCompile(`^[A-Za-z0-9_]+(/[A-Za-z0-9_.-]+)*(\.[A-Za-z_][A-Za-z0-9_]*){0,2}$`)

// partirSimbolo: "strings.Split" → ("strings", "Split"); "math/rand.Intn" → ("math/rand", "Intn");
// "strings.Builder.WriteString" → ("strings", "Builder.WriteString"); "strings" → ("strings", "").
func partirSimbolo(s string) (paquete, ancla string, err error) {
	s = strings.TrimSpace(s)
	if !reSimbolo.MatchString(s) {
		return "", "", fmt.Errorf("saber: «%s» no parece un nombre de Go", s)
	}
	barra := strings.LastIndexByte(s, '/')
	ultimo := s[barra+1:]
	punto := strings.IndexByte(ultimo, '.')
	if punto < 0 {
		return s, "", nil
	}
	return s[:barra+1] + ultimo[:punto], ultimo[punto+1:], nil
}

// PkgGoDev fetches the pkg.go.dev page of a package and keeps the section of simbolo ("strings.Split",
// "strings.Builder.WriteString") or, for a bare package ("strings"), its overview. Cached 30 days.
func (w *Web) PkgGoDev(ctx context.Context, simbolo string) (Documento, error) {
	if w.cfg.SinInternet {
		return Documento{}, nucleo.ErrSinInternet
	}
	paquete, ancla, err := partirSimbolo(simbolo)
	if err != nil {
		return Documento{}, err
	}
	pagina := strings.TrimSuffix(w.cfg.BasePkgGoDev, "/") + "/" + paquete
	seccion := ancla
	if seccion == "" {
		seccion = "pkg-overview"
	}
	direccion := pagina + "#" + seccion
	clave := "pkg:" + direccion
	if e, ok := w.cache.leer(clave, TTLDocumento); ok && e.Documento != nil {
		return *e.Documento, nil
	}
	r, err := w.obtener(ctx, pagina, limite{}, false)
	if err != nil {
		return Documento{}, err
	}
	d, err := extraer(ctx, r.cuerpo, direccion, seccion)
	if err != nil {
		return Documento{}, err
	}
	if len(d.Bloques) == 0 {
		return Documento{}, fmt.Errorf("%w: pkg.go.dev no tiene «%s»", ErrNoEncontrado, simbolo)
	}
	d.Fecha = w.ahora()
	w.cache.escribir(clave, entradaCache{Documento: &d})
	return d, nil
}

// GoDev fetches a page under go.dev (for example "doc/effective_go"), cached 30 days.
func (w *Web) GoDev(ctx context.Context, ruta string) (Documento, error) {
	ruta = strings.TrimPrefix(strings.TrimSpace(ruta), "/")
	if strings.Contains(ruta, "..") || strings.ContainsAny(ruta, " \\?#") {
		return Documento{}, fmt.Errorf("saber: «%s» no es una página de go.dev", ruta)
	}
	return w.Traer(ctx, strings.TrimSuffix(w.cfg.BaseGoDev, "/")+"/"+ruta)
}

// ---- Wikipedia ----

var reIdioma = regexp.MustCompile(`^[a-z]{2,3}$`)

type resumenWiki struct {
	Titulo       string `json:"title"`
	Extracto     string `json:"extract"`
	Tipo         string `json:"type"`
	URLContenido struct {
		Escritorio struct {
			Pagina string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

// Wikipedia returns the summary of a page (REST API, cached 30 days). idioma defaults to "es".
func (w *Web) Wikipedia(ctx context.Context, titulo, idioma string) (Resumen, error) {
	if w.cfg.SinInternet {
		return Resumen{}, nucleo.ErrSinInternet
	}
	if idioma == "" {
		idioma = "es"
	}
	if !reIdioma.MatchString(idioma) {
		return Resumen{}, fmt.Errorf("saber: idioma «%s» no válido", idioma)
	}
	titulo = strings.Join(strings.Fields(titulo), " ")
	if titulo == "" {
		return Resumen{}, errors.New("saber: falta el título de la página")
	}
	base := w.cfg.BaseWikipedia
	if strings.Contains(base, "%s") {
		base = fmt.Sprintf(base, idioma)
	}
	direccion := base + url.PathEscape(strings.ReplaceAll(titulo, " ", "_"))
	clave := "wiki:" + direccion
	if e, ok := w.cache.leer(clave, TTLDocumento); ok && e.Resumen != nil {
		return *e.Resumen, nil
	}
	r, err := w.obtener(ctx, direccion, limite{}, false)
	if err != nil {
		return Resumen{}, err
	}
	var rw resumenWiki
	if err := json.Unmarshal(r.cuerpo, &rw); err != nil {
		return Resumen{}, fmt.Errorf("saber: Wikipedia respondió algo que no entiendo: %w", err)
	}
	if strings.TrimSpace(rw.Extracto) == "" {
		return Resumen{}, fmt.Errorf("%w: Wikipedia no tiene resumen de «%s»", ErrNoEncontrado, titulo)
	}
	res := Resumen{Titulo: rw.Titulo, Texto: colapsar(rw.Extracto), URL: rw.URLContenido.Escritorio.Pagina}
	if res.Titulo == "" {
		res.Titulo = titulo
	}
	if res.URL == "" || !(strings.HasPrefix(res.URL, "https://") || strings.HasPrefix(res.URL, "http://")) {
		res.URL = fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", idioma, url.PathEscape(strings.ReplaceAll(titulo, " ", "_")))
	}
	w.cache.escribir(clave, entradaCache{Resumen: &res})
	return res, nil
}

// DocumentoDeResumen turns a Wikipedia summary into a one-paragraph document (for Rankear/ExtraerHechos).
func DocumentoDeResumen(r Resumen) Documento {
	return Documento{URL: r.URL, Titulo: r.Titulo, Bloques: []Bloque{{Tipo: "parrafo", Texto: r.Texto}}}
}
