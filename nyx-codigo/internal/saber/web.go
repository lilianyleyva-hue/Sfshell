// Package saber is Nyx Código's knowledge source (§4.12): a safe HTTP client (SSRF guard, per-host rate
// limits, size caps, a cache of extracted documents), DuckDuckGo HTML search, gobyexample, pkg.go.dev,
// go.dev and Wikipedia adapters, its own HTML tokenizer and extractor, extraction of Go candidates and
// their adaptation to a target signature, mining of hints, BM25 passage ranking, fact extraction, the
// GOROOT Example index and local `go doc`.
//
// It never executes code: cerebro hands candidates to the sandbox. Fetched text is data only.
package saber

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// Config configures the web client. Empty fields take the defaults in comments.
type Config struct {
	DirCache        string                // ~/.local/share/nyx-codigo/web ("" = memory cache only)
	SinInternet     bool                  // every call returns nucleo.ErrSinInternet
	Agente          string                // "NyxCodigo/1.0 (uso personal)"
	PermitirLocal   bool                  // tests only: allow http and 127.0.0.1 targets
	BaseDDG         string                // "https://html.duckduckgo.com/html/"
	BaseGoByExample string                // "https://gobyexample.com/"
	BasePkgGoDev    string                // "https://pkg.go.dev/"
	BaseGoDev       string                // "https://go.dev/"
	BaseWikipedia   string                // "https://%s.wikipedia.org/api/rest_v1/page/summary/"
	Reloj           func() time.Time      // injectable (default time.Now)
	Dormir          func(d time.Duration) // injectable; nil = a context-aware timer
	GoBin           string
}

// Limits and timings (§4.12).
const (
	MaxCuerpo        = 2 << 20 // 2 MiB per response body
	MaxRedirecciones = 3
	TiempoPeticion   = 8 * time.Second
	TiempoDisponible = 3 * time.Second
	CacheDisponible  = 5 * time.Minute
	TTLBusqueda      = 7 * 24 * time.Hour
	TTLDocumento     = 30 * 24 * time.Hour
	MaxCacheBytes    = 200 << 20 // 200 MB on disk, evicted by oldest mtime at startup
	IntervaloDDG     = 2 * time.Second
	MaxDiaDDG        = 200
	IntervaloOtros   = 500 * time.Millisecond // 2 requests per second per host
	AgentePorDefecto = "NyxCodigo/1.0 (uso personal)"
)

var (
	// ErrBloqueado: the target is not allowed (non-https, loopback, private, link-local… address).
	ErrBloqueado = errors.New("saber: esa dirección no está permitida")
	// ErrDemasiadoGrande: the body exceeds MaxCuerpo.
	ErrDemasiadoGrande = errors.New("saber: la página es demasiado grande (más de 2 MiB)")
	// ErrTipoContenido: the content type is not text/html, text/plain or application/json.
	ErrTipoContenido = errors.New("saber: la página no es texto ni HTML")
	// ErrLimiteDiario: more than MaxDiaDDG searches today.
	ErrLimiteDiario = errors.New("saber: ya hice demasiadas búsquedas hoy; prueba mañana")
	// ErrNoEncontrado: the source has no such page or symbol.
	ErrNoEncontrado = errors.New("saber: no lo encuentro en esa fuente")
)

// Web is the safe web client. It is safe for concurrent use.
type Web struct {
	cfg     Config
	cliente *http.Client
	exentos sync.Map // proxy addresses "host:port" exempt from the SSRF check

	mu        sync.Mutex
	siguiente map[string]time.Time // limiter key → earliest next request
	diaDDG    string
	cuentaDDG int
	dispOK    bool
	dispCuand time.Time
	dispValid bool

	cache *cache
}

// NuevaWeb builds the client and evicts the disk cache down to MaxCacheBytes.
func NuevaWeb(c Config) *Web {
	if c.Agente == "" {
		c.Agente = AgentePorDefecto
	}
	if c.BaseDDG == "" {
		c.BaseDDG = "https://html.duckduckgo.com/html/"
	}
	if c.BaseGoByExample == "" {
		c.BaseGoByExample = "https://gobyexample.com/"
	}
	if c.BasePkgGoDev == "" {
		c.BasePkgGoDev = "https://pkg.go.dev/"
	}
	if c.BaseGoDev == "" {
		c.BaseGoDev = "https://go.dev/"
	}
	if c.BaseWikipedia == "" {
		c.BaseWikipedia = "https://%s.wikipedia.org/api/rest_v1/page/summary/"
	}
	if c.Reloj == nil {
		c.Reloj = time.Now
	}
	w := &Web{cfg: c, siguiente: map[string]time.Time{}}
	w.cache = nuevaCache(c.DirCache, c.Reloj)
	w.cache.desalojar(MaxCacheBytes)

	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transporte := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			u, err := http.ProxyFromEnvironment(r)
			if u != nil {
				w.exentos.Store(direccionProxy(u), true)
			}
			return u, err
		},
		DialContext: func(ctx context.Context, red, dir string) (net.Conn, error) {
			return w.marcar(ctx, dialer, red, dir)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   6 * time.Second,
		ResponseHeaderTimeout: TiempoPeticion,
	}
	w.cliente = &http.Client{
		Transport: transporte,
		Timeout:   TiempoPeticion,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > MaxRedirecciones {
				return fmt.Errorf("saber: demasiadas redirecciones (más de %d)", MaxRedirecciones)
			}
			if err := w.comprobarURL(req.URL); err != nil {
				return err
			}
			return w.comprobarDestino(req.Context(), req)
		},
	}
	return w
}

func (w *Web) ahora() time.Time { return w.cfg.Reloj() }

// Config returns the effective configuration (defaults filled in).
func (w *Web) Config() Config { return w.cfg }

// ---- SSRF guard ----

var redCGNAT = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}
var redCero = &net.IPNet{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)}

// IPProhibida reports whether ip is loopback, private, link-local, multicast, unspecified or otherwise
// not a public unicast address.
func IPProhibida(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
		if redCGNAT.Contains(ip) || redCero.Contains(ip) || ip.Equal(net.IPv4bcast) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// comprobarURL checks scheme, credentials and literal hosts (no DNS).
func (w *Web) comprobarURL(u *url.URL) error {
	if u == nil {
		return ErrBloqueado
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !w.cfg.PermitirLocal {
			return fmt.Errorf("%w: solo uso https (%s)", ErrBloqueado, u.Redacted())
		}
	default:
		return fmt.Errorf("%w: esquema %q", ErrBloqueado, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: la dirección lleva usuario o contraseña", ErrBloqueado)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: falta el servidor", ErrBloqueado)
	}
	if w.cfg.PermitirLocal {
		return nil
	}
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return fmt.Errorf("%w: %s es local", ErrBloqueado, host)
	}
	if ip := net.ParseIP(host); ip != nil && IPProhibida(ip) {
		return fmt.Errorf("%w: %s no es una dirección pública", ErrBloqueado, host)
	}
	return nil
}

// comprobarDestino: when the request goes through a proxy the dialer never sees the target, so the host
// is resolved here (best effort: if this machine cannot resolve it, the proxy decides).
func (w *Web) comprobarDestino(ctx context.Context, req *http.Request) error {
	if w.cfg.PermitirLocal {
		return nil
	}
	p, _ := http.ProxyFromEnvironment(req)
	if p == nil {
		return nil // the dialer checks every resolved address
	}
	host := req.URL.Hostname()
	if net.ParseIP(host) != nil {
		return nil // literal, already checked by comprobarURL
	}
	c, cancelar := context.WithTimeout(ctx, time.Second)
	defer cancelar()
	ips, err := net.DefaultResolver.LookupIPAddr(c, host)
	if err != nil {
		return nil
	}
	for _, ip := range ips {
		if IPProhibida(ip.IP) {
			return fmt.Errorf("%w: %s apunta a %s", ErrBloqueado, host, ip.IP)
		}
	}
	return nil
}

func direccionProxy(u *url.URL) string {
	puerto := u.Port()
	if puerto == "" {
		switch u.Scheme {
		case "https":
			puerto = "443"
		case "socks5", "socks5h":
			puerto = "1080"
		default:
			puerto = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), puerto)
}

// marcar is the guarded DialContext: it resolves the host itself, refuses forbidden addresses and dials
// the checked IP (no second resolution, so DNS rebinding cannot slip through).
func (w *Web) marcar(ctx context.Context, d *net.Dialer, red, dir string) (net.Conn, error) {
	if w.cfg.PermitirLocal {
		return d.DialContext(ctx, red, dir)
	}
	if _, ok := w.exentos.Load(dir); ok {
		return d.DialContext(ctx, red, dir)
	}
	host, puerto, err := net.SplitHostPort(dir)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, a := range addrs {
			ips = append(ips, a.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("saber: %s no tiene dirección", host)
	}
	for _, ip := range ips {
		if IPProhibida(ip) {
			return nil, fmt.Errorf("%w: %s apunta a %s", ErrBloqueado, host, ip)
		}
	}
	var ultimo error
	for _, ip := range ips {
		c, err := d.DialContext(ctx, red, net.JoinHostPort(ip.String(), puerto))
		if err == nil {
			return c, nil
		}
		ultimo = err
	}
	return nil, ultimo
}

// ---- rate limiting ----

// reservar books the next slot for clave and returns how long to wait for it.
func (w *Web) reservar(clave string, intervalo time.Duration, diario bool) (time.Duration, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ahora := w.ahora()
	if diario {
		dia := ahora.Format("2006-01-02")
		if dia != w.diaDDG {
			w.diaDDG, w.cuentaDDG = dia, 0
		}
		if w.cuentaDDG >= MaxDiaDDG {
			return 0, ErrLimiteDiario
		}
		w.cuentaDDG++
	}
	t := w.siguiente[clave]
	if t.Before(ahora) {
		t = ahora
	}
	w.siguiente[clave] = t.Add(intervalo)
	return t.Sub(ahora), nil
}

func (w *Web) esperar(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	if w.cfg.Dormir != nil {
		w.cfg.Dormir(d)
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---- fetching ----

type limite struct {
	clave     string
	intervalo time.Duration
	diario    bool
}

func limiteHost(u *url.URL) limite {
	return limite{clave: strings.ToLower(u.Host), intervalo: IntervaloOtros}
}

var limiteDDG = limite{clave: "ddg", intervalo: IntervaloDDG, diario: true}

type respuestaWeb struct {
	cuerpo []byte
	tipo   string // media type, lowercase
	url    string // final URL after redirects
}

// obtener performs one guarded GET (or HEAD when cabeza) with the rate limit lim (zero = per host).
func (w *Web) obtener(ctx context.Context, crudo string, lim limite, cabeza bool) (respuestaWeb, error) {
	if w.cfg.SinInternet {
		return respuestaWeb{}, nucleo.ErrSinInternet
	}
	u, err := url.Parse(crudo)
	if err != nil {
		return respuestaWeb{}, fmt.Errorf("saber: dirección mal escrita: %w", err)
	}
	if err := w.comprobarURL(u); err != nil {
		return respuestaWeb{}, err
	}
	if lim.clave == "" {
		lim = limiteHost(u)
	}
	espera, err := w.reservar(lim.clave, lim.intervalo, lim.diario)
	if err != nil {
		return respuestaWeb{}, err
	}
	if err := w.esperar(ctx, espera); err != nil {
		return respuestaWeb{}, err
	}
	metodo := http.MethodGet
	if cabeza {
		metodo = http.MethodHead
	}
	req, err := http.NewRequestWithContext(ctx, metodo, u.String(), nil)
	if err != nil {
		return respuestaWeb{}, err
	}
	req.Header.Set("User-Agent", w.cfg.Agente)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,text/plain;q=0.8")
	req.Header.Set("Accept-Language", "es,en;q=0.8")
	if err := w.comprobarDestino(ctx, req); err != nil {
		return respuestaWeb{}, err
	}
	resp, err := w.cliente.Do(req)
	if err != nil {
		if errors.Is(err, ErrBloqueado) {
			return respuestaWeb{}, err
		}
		if ctx.Err() != nil {
			return respuestaWeb{}, ctx.Err()
		}
		return respuestaWeb{}, fmt.Errorf("saber: no pude traer %s: %w", u.Redacted(), err)
	}
	defer resp.Body.Close()
	final := u.String()
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return respuestaWeb{}, fmt.Errorf("%w (%s respondió %d)", ErrNoEncontrado, u.Host, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return respuestaWeb{}, fmt.Errorf("saber: %s respondió %d", u.Host, resp.StatusCode)
	}
	if cabeza {
		return respuestaWeb{url: final}, nil
	}
	tipo, params, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	tipo = strings.ToLower(tipo)
	switch tipo {
	case "text/html", "application/xhtml+xml", "text/plain", "application/json":
	case "":
		tipo = "text/html"
	default:
		return respuestaWeb{}, fmt.Errorf("%w (%s)", ErrTipoContenido, tipo)
	}
	if resp.ContentLength > MaxCuerpo {
		return respuestaWeb{}, ErrDemasiadoGrande
	}
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, MaxCuerpo+1))
	if err != nil {
		if ctx.Err() != nil {
			return respuestaWeb{}, ctx.Err()
		}
		return respuestaWeb{}, fmt.Errorf("saber: se cortó la descarga de %s: %w", u.Host, err)
	}
	if len(cuerpo) > MaxCuerpo {
		return respuestaWeb{}, ErrDemasiadoGrande
	}
	cuerpo = aUTF8(cuerpo, params["charset"])
	return respuestaWeb{cuerpo: cuerpo, tipo: tipo, url: final}, nil
}

// aUTF8 converts Latin-1 / Windows-1252 bodies (the only common non-UTF-8 charsets) to UTF-8.
func aUTF8(b []byte, charset string) []byte {
	cs := strings.ToLower(strings.TrimSpace(charset))
	latino := cs == "iso-8859-1" || cs == "latin1" || cs == "windows-1252" || cs == "iso-8859-15"
	if !latino || utf8.Valid(b) {
		return b
	}
	var sb strings.Builder
	sb.Grow(len(b) + len(b)/8)
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return []byte(sb.String())
}

// Disponible reports whether the internet answers: HEAD BaseGoDev with a 3 s timeout, cached 5 minutes.
func (w *Web) Disponible(ctx context.Context) bool {
	if w.cfg.SinInternet {
		return false
	}
	w.mu.Lock()
	if w.dispValid && w.ahora().Sub(w.dispCuand) < CacheDisponible {
		ok := w.dispOK
		w.mu.Unlock()
		return ok
	}
	w.mu.Unlock()
	c, cancelar := context.WithTimeout(ctx, TiempoDisponible)
	defer cancelar()
	_, err := w.obtener(c, w.cfg.BaseGoDev, limite{}, true)
	ok := err == nil || (!errors.Is(err, context.DeadlineExceeded) && strings.Contains(err.Error(), "respondió"))
	w.mu.Lock()
	w.dispOK, w.dispCuand, w.dispValid = ok, w.ahora(), true
	w.mu.Unlock()
	return ok
}

// Traer fetches one page and returns its extracted document (cached 30 days; raw HTML is never stored).
func (w *Web) Traer(ctx context.Context, direccion string) (Documento, error) {
	if w.cfg.SinInternet {
		return Documento{}, nucleo.ErrSinInternet
	}
	clave := "doc:" + direccion
	if e, ok := w.cache.leer(clave, TTLDocumento); ok && e.Documento != nil {
		return *e.Documento, nil
	}
	r, err := w.obtener(ctx, direccion, limite{}, false)
	if err != nil {
		return Documento{}, err
	}
	d := documentoDe(r)
	d.URL = direccion
	d.Fecha = w.ahora()
	w.cache.escribir(clave, entradaCache{Documento: &d})
	return d, nil
}

// documentoDe turns a fetched body into a Documento according to its media type.
func documentoDe(r respuestaWeb) Documento {
	switch r.tipo {
	case "text/plain":
		d := Documento{URL: r.url}
		for _, p := range strings.Split(strings.ReplaceAll(string(r.cuerpo), "\r\n", "\n"), "\n\n") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if pareceCodigo(p) {
				d.Bloques = append(d.Bloques, Bloque{Tipo: "codigo", Texto: limpiarCodigo(p)})
			} else {
				d.Bloques = append(d.Bloques, Bloque{Tipo: "parrafo", Texto: strings.Join(strings.Fields(p), " ")})
			}
			if len(d.Bloques) >= maxBloques {
				break
			}
		}
		return d
	case "application/json":
		texto := string(r.cuerpo)
		if len(texto) > 20000 {
			texto = texto[:20000]
		}
		return Documento{URL: r.url, Bloques: []Bloque{{Tipo: "codigo", Texto: texto}}}
	}
	return ExtraerDocumento(r.cuerpo, r.url)
}

// pareceCodigo is a cheap guess for plain-text pages.
func pareceCodigo(s string) bool {
	return strings.Contains(s, "func ") || strings.Contains(s, "package ") || strings.Contains(s, ":= ") ||
		(strings.Contains(s, "{") && strings.Contains(s, "}") && strings.Contains(s, "\n"))
}

// TraerVarios fetches the first n links, two at a time, and returns the documents that arrived, in the
// order of the links. Errors on single pages are skipped; ctx cancellation stops early.
func (w *Web) TraerVarios(ctx context.Context, enlaces []Enlace, n int) []Documento {
	if n <= 0 || n > len(enlaces) {
		n = len(enlaces)
	}
	docs := make([]*Documento, n)
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			d, err := w.Traer(ctx, enlaces[i].URL)
			if err == nil {
				docs[i] = &d
			}
		}(i)
	}
	wg.Wait()
	var out []Documento
	for _, d := range docs {
		if d != nil {
			out = append(out, *d)
		}
	}
	return out
}
