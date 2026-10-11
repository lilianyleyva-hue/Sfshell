// Package servidor is Nyx Código's local HTTP server (§4.15, §5): security checks (Host, Origin,
// token, body size, CSP), the task registry with per-task event buffers, the SSE streams, pending
// questions, cancellation and the embedded UI (servidor/web).
//
// It imports only nucleo. The Motor (cerebro in the real program) does the thinking; the server only
// moves requests in and events out.
package servidor

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Motor is what the server needs from the reasoning core (cerebro implements it).
type Motor interface {
	Responder(ctx context.Context, p nucleo.Peticion, t *nucleo.Traza) nucleo.Respuesta
	Opinar(o nucleo.Opinion) error
	Funciones(q string) []nucleo.FuncionAprendida
	Hechos(q string) []nucleo.Hecho
	Palabras() map[string]string
	Olvidar(tipo, id string) error
	Riesgos(codigo string) []nucleo.Violacion
	Estado() nucleo.EstadoApp
}

// Config configures the server. Zero fields take the defaults in comments.
type Config struct {
	Token     string        // 32 random bytes, hex (required: without it every /api/ call is refused)
	Puerto    int           // for Host and Origin checks
	MaxCuerpo int64         // 1 MiB
	Retener   time.Duration // finished tasks kept 15 min
	MaxTareas int           // concurrent running tasks, default 4 (others queue)

	// Additive knobs (tests use small values).
	Limite         time.Duration // hard limit per task, default 130 s
	LimiteEstudio  time.Duration // hard limit for "estudiar", default 12 min
	EsperaPregunta time.Duration // how long a pending question waits, default 5 min
	Latido         time.Duration // SSE heartbeat, default 15 s
}

// Defaults.
const (
	MaxCuerpoPorDefecto = 1 << 20
	maxEventos          = 20000 // per task; pregunta/respuesta/fin are never dropped
	CSP                 = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

//go:embed web
var archivosWeb embed.FS

// Archivos returns the embedded UI files (index.html, app.js, estilo.css).
func Archivos() fs.FS {
	sub, err := fs.Sub(archivosWeb, "web")
	if err != nil {
		panic(err) // the embed pattern guarantees the directory
	}
	return sub
}

// Servidor is the http.Handler returned by Nuevo.
type Servidor struct {
	m   Motor
	c   Config
	mux *http.ServeMux
	web fs.FS

	mu     sync.Mutex
	tareas map[string]*tarea
	cupos  chan struct{}
	ahora  func() time.Time
}

// Nuevo builds the handler.
func Nuevo(m Motor, c Config) http.Handler {
	return nuevo(m, c)
}

func nuevo(m Motor, c Config) *Servidor {
	if c.MaxCuerpo <= 0 {
		c.MaxCuerpo = MaxCuerpoPorDefecto
	}
	if c.Retener <= 0 {
		c.Retener = 15 * time.Minute
	}
	if c.MaxTareas <= 0 {
		c.MaxTareas = 4
	}
	if c.Limite <= 0 {
		c.Limite = 130 * time.Second
	}
	if c.LimiteEstudio <= 0 {
		c.LimiteEstudio = 12 * time.Minute
	}
	if c.EsperaPregunta <= 0 {
		c.EsperaPregunta = 5 * time.Minute
	}
	if c.Latido <= 0 {
		c.Latido = 15 * time.Second
	}
	s := &Servidor{m: m, c: c, web: Archivos(), tareas: map[string]*tarea{},
		cupos: make(chan struct{}, c.MaxTareas), ahora: time.Now}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.indice)
	mux.HandleFunc("GET /estatico/{archivo}", s.estatico)
	mux.HandleFunc("POST /api/preguntar", s.preguntar)
	mux.HandleFunc("POST /api/codigo/{accion}", s.codigo)
	mux.HandleFunc("GET /api/tareas/{id}/eventos", s.eventos)
	mux.HandleFunc("POST /api/tareas/{id}/responder", s.responder)
	mux.HandleFunc("POST /api/tareas/{id}/cancelar", s.cancelar)
	mux.HandleFunc("POST /api/tareas/{id}/pensar-mas", s.pensarMas)
	mux.HandleFunc("POST /api/opinion", s.opinion)
	mux.HandleFunc("POST /api/estudiar", s.estudiar)
	mux.HandleFunc("GET /api/memoria/funciones", s.funciones)
	mux.HandleFunc("DELETE /api/memoria/funciones/{nombre}", s.olvidar("funcion", "nombre"))
	mux.HandleFunc("GET /api/memoria/hechos", s.hechos)
	mux.HandleFunc("DELETE /api/memoria/hechos/{id}", s.olvidar("hecho", "id"))
	mux.HandleFunc("GET /api/memoria/palabras", s.palabras)
	mux.HandleFunc("DELETE /api/memoria/palabras/{palabra}", s.olvidar("palabra", "palabra"))
	mux.HandleFunc("GET /api/memoria/pendientes", s.pendientes)
	mux.HandleFunc("GET /api/estado", s.estado)
	s.mux = mux
	return s
}

// Cerrar cancels every running task (used on shutdown).
func (s *Servidor) Cerrar() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tareas {
		t.cancelar()
	}
}

// ---- security ----

func (s *Servidor) hostValido(h string) bool {
	p := strconv.Itoa(s.c.Puerto)
	return h == "127.0.0.1:"+p || h == "localhost:"+p
}

func (s *Servidor) origenValido(o string) bool {
	p := strconv.Itoa(s.c.Puerto)
	return o == "http://127.0.0.1:"+p || o == "http://localhost:"+p
}

func (s *Servidor) tokenValido(t string) bool {
	if s.c.Token == "" || t == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.c.Token)) == 1
}

// ServeHTTP applies the checks of §5.1 to every request and then routes it.
func (s *Servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", CSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	api := strings.HasPrefix(r.URL.Path, "/api/")
	if api {
		h.Set("Cache-Control", "no-store")
	}
	if !s.hostValido(r.Host) {
		errorJSON(w, http.StatusForbidden, "Host no permitido")
		return
	}
	if o := r.Header.Get("Origin"); o != "" && !s.origenValido(o) {
		errorJSON(w, http.StatusForbidden, "Origen no permitido")
		return
	}
	if api {
		tok := r.Header.Get("X-Nyx-Token")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/eventos") {
			tok = r.URL.Query().Get("t") // EventSource cannot set headers
		}
		if !s.tokenValido(tok) {
			errorJSON(w, http.StatusForbidden, "Falta la clave de esta sesión: abre Nyx Código desde su icono")
			return
		}
	}
	if r.ContentLength > s.c.MaxCuerpo {
		errorJSON(w, http.StatusRequestEntityTooLarge, "La petición es demasiado grande (máximo 1 MiB)")
		return
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, s.c.MaxCuerpo)
	}
	s.mux.ServeHTTP(w, r)
}

// ---- JSON helpers ----

func escribirJSON(w http.ResponseWriter, codigo int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"no pude preparar la respuesta"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	w.Write(b)
}

func errorJSON(w http.ResponseWriter, codigo int, msg string) {
	escribirJSON(w, codigo, map[string]string{"error": msg})
}

// leerJSON decodes the body into v; it answers 413 or 400 itself and returns false on error.
func leerJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			errorJSON(w, http.StatusRequestEntityTooLarge, "La petición es demasiado grande (máximo 1 MiB)")
		} else {
			errorJSON(w, http.StatusBadRequest, "No pude leer la petición")
		}
		return false
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return true // empty body = zero value
	}
	if err := json.Unmarshal(b, v); err != nil {
		errorJSON(w, http.StatusBadRequest, "La petición no es JSON válido")
		return false
	}
	return true
}

func nuevoID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// ---- static files ----

func (s *Servidor) indice(w http.ResponseWriter, r *http.Request) {
	s.servirArchivo(w, "index.html")
}

var estaticosPermitidos = map[string]bool{"app.js": true, "estilo.css": true}

func (s *Servidor) estatico(w http.ResponseWriter, r *http.Request) {
	nombre := r.PathValue("archivo")
	if !estaticosPermitidos[nombre] {
		http.NotFound(w, r)
		return
	}
	s.servirArchivo(w, nombre)
}

func (s *Servidor) servirArchivo(w http.ResponseWriter, nombre string) {
	b, err := fs.ReadFile(s.web, nombre)
	if err != nil {
		http.Error(w, "falta el archivo "+nombre, http.StatusInternalServerError)
		return
	}
	tipo := mime.TypeByExtension(path.Ext(nombre))
	switch path.Ext(nombre) {
	case ".html":
		tipo = "text/html; charset=utf-8"
	case ".js":
		tipo = "text/javascript; charset=utf-8"
	case ".css":
		tipo = "text/css; charset=utf-8"
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(b)
}

// ---- requests that create tasks ----

type cuerpoPreguntar struct {
	Sesion string `json:"sesion"`
	Texto  string `json:"texto"`
	Codigo string `json:"codigo"`
	Modo   string `json:"modo"`
}

func (s *Servidor) preguntar(w http.ResponseWriter, r *http.Request) {
	var c cuerpoPreguntar
	if !leerJSON(w, r, &c) {
		return
	}
	if strings.TrimSpace(c.Texto) == "" && strings.TrimSpace(c.Codigo) == "" {
		errorJSON(w, http.StatusBadRequest, "Escribe una pregunta o pega código")
		return
	}
	p := nucleo.Peticion{Sesion: c.Sesion, Texto: c.Texto, Codigo: c.Codigo, Modo: c.Modo}
	escribirJSON(w, http.StatusOK, map[string]string{"tarea": s.lanzar(p)})
}

type cuerpoCodigo struct {
	Sesion     string   `json:"sesion"`
	Codigo     string   `json:"codigo"`
	Texto      string   `json:"texto"`
	Entrada    string   `json:"entrada"`
	Args       []string `json:"args"`
	Confirmado bool     `json:"confirmado"`
}

var accionesCodigo = map[string]bool{"arreglar": true, "explicar": true, "probar": true, "ejecutar": true}

func (s *Servidor) codigo(w http.ResponseWriter, r *http.Request) {
	accion := r.PathValue("accion")
	if !accionesCodigo[accion] {
		errorJSON(w, http.StatusNotFound, "No sé hacer «"+accion+"» con el código")
		return
	}
	var c cuerpoCodigo
	if !leerJSON(w, r, &c) {
		return
	}
	if strings.TrimSpace(c.Codigo) == "" {
		errorJSON(w, http.StatusBadRequest, "Falta el código")
		return
	}
	if accion == "ejecutar" && !c.Confirmado {
		if v := s.m.Riesgos(c.Codigo); len(v) > 0 {
			escribirJSON(w, http.StatusConflict, map[string]any{"riesgos": v})
			return
		}
	}
	p := nucleo.Peticion{Sesion: c.Sesion, Texto: c.Texto, Codigo: c.Codigo, Accion: accion,
		Entrada: c.Entrada, Args: c.Args, Confirmado: c.Confirmado}
	escribirJSON(w, http.StatusOK, map[string]string{"tarea": s.lanzar(p)})
}

func (s *Servidor) pensarMas(w http.ResponseWriter, r *http.Request) {
	vieja := s.tarea(r.PathValue("id"))
	if vieja == nil {
		errorJSON(w, http.StatusNotFound, "Esa tarea ya no existe")
		return
	}
	old := vieja.peticion
	p := nucleo.Peticion{Sesion: old.Sesion, Texto: old.Texto, Codigo: old.Codigo, Modo: old.Modo,
		Entrada: old.Entrada, Args: old.Args, Accion: "pensar_mas", Previa: vieja.id}
	escribirJSON(w, http.StatusOK, map[string]string{"tarea": s.lanzar(p)})
}

func (s *Servidor) estudiar(w http.ResponseWriter, r *http.Request) {
	var c struct {
		Sesion string `json:"sesion"`
	}
	if !leerJSON(w, r, &c) {
		return
	}
	p := nucleo.Peticion{Sesion: c.Sesion, Accion: "estudiar", Texto: "estudiar Go"}
	escribirJSON(w, http.StatusOK, map[string]string{"tarea": s.lanzar(p)})
}

// ---- task control ----

func (s *Servidor) tarea(id string) *tarea {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tareas[id]
}

func (s *Servidor) responder(w http.ResponseWriter, r *http.Request) {
	t := s.tarea(r.PathValue("id"))
	if t == nil {
		errorJSON(w, http.StatusNotFound, "Esa tarea ya no existe")
		return
	}
	var c struct {
		Pregunta  string `json:"pregunta"`
		Respuesta string `json:"respuesta"`
	}
	if !leerJSON(w, r, &c) {
		return
	}
	if !t.contestar(c.Pregunta, c.Respuesta) {
		errorJSON(w, http.StatusConflict, "Esa pregunta ya no espera respuesta")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) cancelar(w http.ResponseWriter, r *http.Request) {
	t := s.tarea(r.PathValue("id"))
	if t == nil {
		errorJSON(w, http.StatusNotFound, "Esa tarea ya no existe")
		return
	}
	t.marcarCancelada()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) opinion(w http.ResponseWriter, r *http.Request) {
	var o nucleo.Opinion
	if !leerJSON(w, r, &o) {
		return
	}
	if err := s.m.Opinar(o); err != nil {
		errorJSON(w, http.StatusInternalServerError, "No pude guardar tu opinión: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- memory views ----

type casoVista struct {
	Entrada  string `json:"entrada"`
	Esperado string `json:"esperado,omitempty"`
	Origen   string `json:"origen"`
	Nota     string `json:"nota,omitempty"`
}

type funcionVista struct {
	nucleo.FuncionAprendida
	FirmaGo string      `json:"firmaGo"`
	Casos   []casoVista `json:"casos"` // shadows the encoded cases
}

// vistaFuncion decodes the stored cases into display strings.
func vistaFuncion(f nucleo.FuncionAprendida) funcionVista {
	v := funcionVista{FuncionAprendida: f, FirmaGo: f.Firma.Go(), Casos: []casoVista{}}
	casos, err := nucleo.LeerCasos(f.Firma, f.Casos)
	if err != nil {
		return v
	}
	ent := f.Firma.Entradas()
	for _, c := range casos {
		var args []string
		for i, x := range c.Entradas {
			args = append(args, nucleo.FormatoHumano(x, ent[i]))
		}
		cv := casoVista{Entrada: f.Firma.Nombre + "(" + strings.Join(args, ", ") + ")", Origen: string(c.Origen), Nota: c.Nota}
		if c.Esperado != nil {
			var res []string
			for i, x := range c.Esperado {
				res = append(res, nucleo.FormatoHumano(x, f.Firma.Res[i]))
			}
			cv.Esperado = strings.Join(res, ", ")
		}
		v.Casos = append(v.Casos, cv)
	}
	return v
}

func (s *Servidor) funciones(w http.ResponseWriter, r *http.Request) {
	fs := s.m.Funciones(r.URL.Query().Get("q"))
	out := make([]funcionVista, 0, len(fs))
	for _, f := range fs {
		out = append(out, vistaFuncion(f))
	}
	escribirJSON(w, http.StatusOK, out)
}

func (s *Servidor) hechos(w http.ResponseWriter, r *http.Request) {
	hs := s.m.Hechos(r.URL.Query().Get("q"))
	if hs == nil {
		hs = []nucleo.Hecho{}
	}
	escribirJSON(w, http.StatusOK, hs)
}

func (s *Servidor) palabras(w http.ResponseWriter, r *http.Request) {
	p := s.m.Palabras()
	if p == nil {
		p = map[string]string{}
	}
	escribirJSON(w, http.StatusOK, p)
}

func (s *Servidor) olvidar(tipo, param string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue(param)
		if strings.TrimSpace(id) == "" {
			errorJSON(w, http.StatusBadRequest, "Falta qué olvidar")
			return
		}
		if err := s.m.Olvidar(tipo, id); err != nil {
			errorJSON(w, http.StatusNotFound, "No pude olvidarlo: "+err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// pendientes calls the Motor's optional Pendientes() method (cerebro returns []memoria.Pendiente,
// which this package cannot name without importing memoria, so it is found by reflection).
func (s *Servidor) pendientes(w http.ResponseWriter, r *http.Request) {
	var out any = []any{}
	if m := reflect.ValueOf(s.m).MethodByName("Pendientes"); m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
		v := m.Call(nil)[0]
		if !(v.Kind() == reflect.Slice && v.IsNil()) {
			out = v.Interface()
		}
	}
	escribirJSON(w, http.StatusOK, out)
}

func (s *Servidor) estado(w http.ResponseWriter, r *http.Request) {
	escribirJSON(w, http.StatusOK, s.m.Estado())
}
