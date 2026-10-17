// Package habla is how Nyx Código learns what words mean and learns to talk (§ «Cómo aprende a hablar
// y qué significa cada palabra» of the README). It keeps:
//
//   - a dictionary of meanings (Significado): it starts with the 66 core words of Nyx's Resh
//     dictionary, learns new ones when the user teaches them ("X significa Y") and when it reads
//     definitional sentences ("X es un Y", "X quiere decir Y", "X sirve para Y", "llamamos X a Y",
//     "X: Y"). A word with its own definition is known (Propio); a word that only appears inside other
//     definitions is anchored, not known: it inherits their meaning vectors but has no definition.
//   - meaning vectors and relations between words (afines, opuestos, vecinos de tema, sin relación),
//     built only from the definitions, with a polarity from gain/lose stems (ideas from Nyx's Gnostos
//     v5/v6: VectorSemantico, VectorSemanticoV6, polaridad, RelacionSemantica).
//   - an interpolated Kneser-Ney trigram language model trained on everything it reads, used by Decir to
//     build a sentence about a topic with only words whose meaning it knows.
//
// Everything is persisted in <dir>/habla.json (0600, atomic write). Every method is safe for
// concurrent use, bounded, and linear in the size of its input (Decir is bounded by its attempts).
package habla

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Version is the format version of habla.json.
const Version = 1

// Archivo is the file name inside the data directory.
const Archivo = "habla.json"

// Bounds.
const (
	MaxPalabra      = 40      // runes in a word or short phrase
	MaxDefinicion   = 300     // runes in a definition
	MaxSignificados = 20_000  // meanings in the dictionary
	MaxAnteriores   = 5       // previous definitions kept per word
	MaxLectura      = 1 << 20 // bytes read per Leer call
	maxFraseRunas   = 600     // longer sentences are skipped
	maxAnclas       = 50      // definitions averaged for an anchored word
	maxDesconocidas = 50
)

// Sources of a meaning.
const (
	FuenteSemilla = "semilla"
	FuenteUsuario = "usuario"
	FuenteLectura = "lectura"
	FuenteWeb     = "web"
)

// Significado is what a word means and where that came from.
type Significado struct {
	Palabra    string    `json:"palabra"`
	Definicion string    `json:"definicion"`
	Ingles     string    `json:"ingles,omitempty"`
	Conceptos  []string  `json:"conceptos,omitempty"` // the content words of the definition
	Fuente     string    `json:"fuente"`              // semilla | usuario | lectura | web
	Origen     string    `json:"origen"`              // "me lo enseñaste tú el 17/10", "lo leí en mis notas de Go"
	Confianza  float64   `json:"confianza"`
	Propio     bool      `json:"propio"` // true: its own definition; false: only anchored in other definitions
	Anteriores []string  `json:"anteriores,omitempty"`
	Fecha      time.Time `json:"fecha"`
}

// Errors.
var (
	ErrYaEnsenada = errors.New("ya tengo una definición más fiable de esa palabra")
	ErrLleno      = errors.New("mi diccionario está lleno: olvida alguna palabra primero")
	ErrNoLaSe     = errors.New("no sé qué significa esa palabra")
)

// Habla is the dictionary plus the language model. Create it with Abrir (or Nuevo, in memory).
type Habla struct {
	mu        sync.Mutex
	escritura sync.Mutex
	ruta      string

	sig       map[string]*Significado // clave → own meaning
	olvidadas map[string]bool         // seed words the user made me forget
	lecturas  map[string]string       // texts already read: name → version
	frases    int
	lm        *modelo

	// derived from sig (rebuilt by indexar when sucioIndice)
	sucioIndice bool
	porTallo    map[string][]string // stem → claves with an own meaning
	anclas      map[string][]string // stem of a word named in definitions → claves of those definitions
	muestra     map[string]string   // stem → a word to show
	genSig      int                 // bumped on every dictionary change
	permitido   []bool              // cache for Decir: vocabulary ids it may say
	genPermit   int

	sucio, urgente bool
	ultimo         time.Time
	avisos         []string
	ahora          func() time.Time
}

func vacio() *Habla {
	return &Habla{sig: map[string]*Significado{}, olvidadas: map[string]bool{}, lecturas: map[string]string{},
		lm: nuevoModelo(), sucioIndice: true, ahora: time.Now, genPermit: -1}
}

// Nuevo returns a seeded dictionary kept only in memory (Guardar does nothing).
func Nuevo() *Habla {
	h := vacio()
	h.sembrar()
	return h
}

type archivo struct {
	Version      int               `json:"version"`
	Significados []Significado     `json:"significados"`
	Olvidadas    []string          `json:"olvidadas,omitempty"`
	Lecturas     map[string]string `json:"lecturas,omitempty"`
	Frases       int               `json:"frases"`
	Modelo       archivoModelo     `json:"modelo"`
}

// Abrir loads <dir>/habla.json, creating the folder (0700) and a seeded file (0600) when missing. A
// corrupt file is kept as habla.json.roto and the dictionary starts seeded again, with a Spanish
// warning in Avisos. It only fails when the folder cannot be used.
func Abrir(dir string) (*Habla, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("habla: falta la carpeta de datos")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("habla: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("habla: no puedo crear %s: %w", abs, err)
	}
	ruta := filepath.Join(abs, Archivo)
	datos, err := os.ReadFile(ruta)
	h := vacio()
	h.ruta = ruta
	switch {
	case errors.Is(err, os.ErrNotExist):
		h.sembrar()
		h.sucio = true
	case err != nil:
		return nil, fmt.Errorf("habla: no puedo leer %s: %w", ruta, err)
	default:
		if !h.cargar(datos) {
			avisos := h.avisos
			h = vacio()
			h.ruta = ruta
			h.avisos = avisos
			if e := os.Rename(ruta, ruta+".roto"); e != nil {
				h.avisar(fmt.Sprintf("Mi diccionario (habla.json) estaba dañado y no pude apartarlo: %v. Empiezo con el diccionario de Nyx.", e))
			} else {
				h.avisar("Mi diccionario (habla.json) estaba dañado: lo guardé como habla.json.roto y empiezo con el diccionario de Nyx.")
			}
			h.sembrar()
			h.sucio = true
		}
	}
	if h.sucio {
		if err := h.Guardar(); err != nil {
			h.avisar(fmt.Sprintf("No pude guardar mi diccionario: %v.", err))
		}
	}
	h.ultimo = h.ahora()
	return h, nil
}

// cargar reads the file; false when it is not a habla.json this program understands.
func (h *Habla) cargar(datos []byte) bool {
	var a archivo
	if err := json.Unmarshal(datos, &a); err != nil || a.Version != Version {
		return false
	}
	for _, s := range a.Significados {
		k := clave(s.Palabra)
		if k == "" || strings.TrimSpace(s.Definicion) == "" || largo(s.Palabra) > MaxPalabra || len(h.sig) >= MaxSignificados {
			continue
		}
		s.Propio = true
		s.Conceptos = conceptosDe(s.Definicion)
		if len(s.Anteriores) > MaxAnteriores {
			s.Anteriores = s.Anteriores[len(s.Anteriores)-MaxAnteriores:]
		}
		c := s
		h.sig[k] = &c
	}
	for _, o := range a.Olvidadas {
		h.olvidadas[clave(o)] = true
	}
	for k, v := range a.Lecturas {
		h.lecturas[k] = v
	}
	h.frases = a.Frases
	m, ok := importar(a.Modelo)
	h.lm = m
	if !ok {
		h.avisar("Mis frases aprendidas (en habla.json) estaban dañadas: vuelvo a aprender a hablar desde cero.")
		h.sucio = true
	}
	if h.sembrar() > 0 {
		h.sucio = true
	}
	return true
}

// sembrar adds the seed words that are missing (and not forgotten); it returns how many.
func (h *Habla) sembrar() int {
	n := 0
	for _, s := range semilla {
		k := clave(s.palabra)
		if h.sig[k] != nil || h.olvidadas[k] {
			continue
		}
		h.sig[k] = &Significado{Palabra: s.palabra, Definicion: s.definicion, Ingles: s.ingles,
			Conceptos: conceptosDe(s.definicion), Fuente: FuenteSemilla, Origen: OrigenSemilla, Confianza: 0.9, Propio: true}
		h.entrenarDefinicion(s.palabra, s.definicion)
		n++
	}
	if n > 0 {
		h.cambioDiccionario()
	}
	return n
}

func (h *Habla) cambioDiccionario() {
	h.sucioIndice = true
	h.genSig++
	h.sucio = true
}

// Avisos are Spanish warnings (corrupt file…).
func (h *Habla) Avisos() []string {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.avisos...)
}

func (h *Habla) avisar(s string) {
	if len(h.avisos) < 20 {
		h.avisos = append(h.avisos, s)
	}
}

// ---- saving ----

// Guardar writes habla.json when something changed (temp file, fsync, rename).
func (h *Habla) Guardar() error {
	if h == nil {
		return nil
	}
	h.escritura.Lock()
	defer h.escritura.Unlock()
	h.mu.Lock()
	if !h.sucio || h.ruta == "" {
		h.mu.Unlock()
		return nil
	}
	a := archivo{Version: Version, Lecturas: map[string]string{}, Frases: h.frases, Modelo: h.lm.exportar()}
	for _, k := range h.clavesOrdenadas() {
		a.Significados = append(a.Significados, copiar(*h.sig[k]))
	}
	for k := range h.olvidadas {
		a.Olvidadas = append(a.Olvidadas, k)
	}
	sort.Strings(a.Olvidadas)
	for k, v := range h.lecturas {
		a.Lecturas[k] = v
	}
	datos, err := json.Marshal(a)
	h.sucio, h.urgente = false, false
	h.ultimo = h.ahora()
	ruta := h.ruta
	h.mu.Unlock()
	if err == nil {
		err = escribirAtomico(ruta, append(datos, '\n'))
	}
	if err != nil {
		h.mu.Lock()
		h.sucio = true
		h.mu.Unlock()
	}
	return err
}

// GuardarSiHaceFalta saves now when something the user taught changed, or when there are unsaved
// changes older than intervalo (what is read from every request is saved at most that often).
func (h *Habla) GuardarSiHaceFalta(intervalo time.Duration) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	toca := h.sucio && (h.urgente || h.ahora().Sub(h.ultimo) >= intervalo)
	h.mu.Unlock()
	if !toca {
		return nil
	}
	return h.Guardar()
}

func escribirAtomico(ruta string, datos []byte) error {
	tmp := ruta + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(datos); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, ruta); err != nil {
		os.Remove(tmp)
		return err
	}
	if d, err := os.Open(filepath.Dir(ruta)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// ---- lookups ----

func copiar(s Significado) Significado {
	s.Conceptos = append([]string(nil), s.Conceptos...)
	s.Anteriores = append([]string(nil), s.Anteriores...)
	return s
}

func (h *Habla) clavesOrdenadas() []string {
	ks := make([]string, 0, len(h.sig))
	for k := range h.sig {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// propioEstricto finds an own meaning by the word itself, without an article, or one of its forms
// (plural, feminine, verb forms). No stem guessing.
func (h *Habla) propioEstricto(palabra string) *Significado {
	k := clave(palabra)
	if k == "" {
		return nil
	}
	if s := h.sig[k]; s != nil {
		return s
	}
	k = clave(sinArticulo(k))
	if s := h.sig[k]; s != nil {
		return s
	}
	if strings.Contains(k, " ") {
		return h.sig[clave(singularFrase(k))]
	}
	for _, f := range formas(k) {
		if s := h.sig[f]; s != nil {
			return s
		}
	}
	return nil
}

// propio is propioEstricto plus the stem: "programar" finds "programa" when it is the only word with
// that stem.
func (h *Habla) propio(palabra string) *Significado {
	if s := h.propioEstricto(palabra); s != nil {
		return s
	}
	k := clave(sinArticulo(palabra))
	if k == "" || strings.Contains(k, " ") || largo(k) < 4 {
		return nil
	}
	h.indexar()
	if cs := h.porTallo[tallo(k)]; len(cs) == 1 {
		return h.sig[cs[0]]
	}
	return nil
}

// anclado returns the claves of the definitions that name this word.
func (h *Habla) anclado(palabra string) []string {
	k := clave(sinArticulo(palabra))
	if k == "" || strings.Contains(k, " ") || largo(k) < 3 {
		return nil
	}
	h.indexar()
	return h.anclas[tallo(k)]
}

// indexar rebuilds the stem and anchor indexes. Linear in the size of the dictionary.
func (h *Habla) indexar() {
	if !h.sucioIndice {
		return
	}
	h.porTallo = map[string][]string{}
	h.anclas = map[string][]string{}
	h.muestra = map[string]string{}
	for _, k := range h.clavesOrdenadas() {
		s := h.sig[k]
		if !strings.Contains(k, " ") {
			t := tallo(k)
			h.porTallo[t] = append(h.porTallo[t], k)
			h.muestra[t] = strings.ToLower(s.Palabra)
		}
	}
	for _, k := range h.clavesOrdenadas() {
		s := h.sig[k]
		propia := tallo(k)
		visto := map[string]bool{}
		for _, t := range tokenizar(s.Definicion) {
			if t.clase != 'w' || esVacia(t.pleg) || largo(t.pleg) < 3 {
				continue
			}
			st := tallo(t.pleg)
			if st == propia || visto[st] {
				continue
			}
			visto[st] = true
			h.anclas[st] = append(h.anclas[st], k)
			if _, ok := h.muestra[st]; !ok {
				h.muestra[st] = singular(strings.ToLower(t.texto))
			}
		}
	}
	h.sucioIndice = false
}

// conoce: a function word, a word with its own meaning, or an anchored one.
func (h *Habla) conoce(palabra string) (propio, anclado bool) {
	if esFuncion(plegar(palabra)) {
		return true, false
	}
	if h.propio(palabra) != nil {
		return true, false
	}
	return false, len(h.anclado(palabra)) > 0
}

// Buscar finds what a word means: its own definition (Propio) or, when it only appears inside other
// definitions, an anchored meaning (Propio false, no Definicion, Conceptos = the words whose
// definitions name it). Tolerant to case, accents, plurals and verb forms.
func (h *Habla) Buscar(palabra string) (Significado, bool) {
	if h == nil {
		return Significado{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.propio(palabra); s != nil {
		return copiar(*s), true
	}
	des := h.anclado(palabra)
	if len(des) == 0 {
		return Significado{}, false
	}
	var nombres []string
	for i, k := range des {
		if i == 6 {
			break
		}
		nombres = append(nombres, h.sig[k].Palabra)
	}
	w := strings.ToLower(sinArticulo(strings.TrimSpace(palabra)))
	return Significado{Palabra: w, Conceptos: nombres, Fuente: "anclada", Confianza: 0.3,
		Origen: "aparece en lo que significan " + listaComillas(nombres)}, true
}

func listaComillas(ps []string) string {
	q := make([]string, len(ps))
	for i, p := range ps {
		q[i] = "«" + p + "»"
	}
	switch len(q) {
	case 0:
		return ""
	case 1:
		return q[0]
	}
	return strings.Join(q[:len(q)-1], ", ") + " y " + q[len(q)-1]
}

// conceptosDe are the content words of a definition (lowercase, singular, without repeats).
func conceptosDe(def string) []string {
	var out []string
	visto := map[string]bool{}
	for _, t := range tokenizar(def) {
		if t.clase != 'w' || esVacia(t.pleg) || largo(t.pleg) < 3 {
			continue
		}
		w := strings.ToLower(t.texto)
		if !visto[w] && len(out) < 12 {
			visto[w] = true
			out = append(out, w)
		}
	}
	return out
}

// ---- teaching ----

func limpiar(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(strings.Trim(s, "\"'«»“”.,;: "))
}

func validar(palabra, definicion string) (string, string, error) {
	p, d := limpiar(palabra), limpiar(definicion)
	switch {
	case p == "" || d == "":
		return "", "", errors.New("falta la palabra o lo que significa")
	case largo(p) > MaxPalabra:
		return "", "", fmt.Errorf("la palabra es demasiado larga (más de %d letras)", MaxPalabra)
	case largo(d) > MaxDefinicion:
		return "", "", fmt.Errorf("lo que significa es demasiado largo (más de %d letras)", MaxDefinicion)
	case !esLetras(p):
		return "", "", errors.New("la palabra solo puede tener letras")
	case len(strings.Fields(p)) > 4:
		return "", "", errors.New("eso es una frase, no una palabra")
	case clave(p) == clave(d):
		return "", "", errors.New("una palabra no puede significar ella misma")
	}
	return p, d, nil
}

// confianzaDe is the confidence of a meaning taught from a source.
func confianzaDe(fuente string) float64 {
	switch fuente {
	case FuenteUsuario:
		return 1
	case FuenteSemilla:
		return 0.9
	case FuenteWeb:
		return 0.5
	}
	return 0.6
}

// rangoDe orders how much a meaning is trusted: taught by the user > seed > read.
func rangoDe(fuente string, conf float64) int {
	switch {
	case fuente == FuenteUsuario && conf >= 0.99:
		return 3
	case fuente == FuenteSemilla:
		return 2
	}
	return 1
}

// Ensenar learns (or changes) what a word means. A new definition taught by the user replaces the
// old one, which is kept in Anteriores; other sources never replace a meaning the user taught.
func (h *Habla) Ensenar(palabra, definicion, fuente, origen string) (Significado, error) {
	if h == nil {
		return Significado{}, errors.New("no tengo diccionario")
	}
	p, d, err := validar(palabra, definicion)
	if err != nil {
		return Significado{}, err
	}
	if fuente == "" {
		fuente = FuenteUsuario
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	s, cambio, err := h.poner(p, d, fuente, origen, confianzaDe(fuente), true)
	if cambio && fuente == FuenteUsuario {
		h.urgente = true
	}
	return s, err
}

// poner stores a validated meaning under the rules of Ensenar; the caller holds h.mu.
func (h *Habla) poner(p, d, fuente, origen string, conf float64, entrenar bool) (Significado, bool, error) {
	k := clave(p)
	ex := h.sig[k]
	if ex == nil {
		if ks := clave(singularFrase(p)); ks != k && h.sig[ks] != nil {
			ex, k = h.sig[ks], ks // "funciones significa…" changes "función"
		}
	}
	ahora := h.ahora()
	if ex == nil {
		if len(h.sig) >= MaxSignificados {
			return Significado{}, false, ErrLleno
		}
		s := &Significado{Palabra: strings.ToLower(p), Definicion: d, Conceptos: conceptosDe(d), Fuente: fuente,
			Origen: origen, Confianza: conf, Propio: true, Fecha: ahora}
		h.sig[k] = s
		delete(h.olvidadas, k)
		h.cambioDiccionario()
		if entrenar {
			h.entrenarDefinicion(s.Palabra, d)
		}
		return copiar(*s), true, nil
	}
	nuevo, viejo := rangoDe(fuente, conf), rangoDe(ex.Fuente, ex.Confianza)
	if clave(ex.Definicion) == clave(d) {
		if nuevo > viejo || (nuevo == viejo && conf > ex.Confianza) {
			ex.Fuente, ex.Origen, ex.Confianza, ex.Fecha = fuente, origen, conf, ahora
			h.sucio = true
			return copiar(*ex), true, nil
		}
		return copiar(*ex), false, nil
	}
	if nuevo < viejo || (nuevo == viejo && nuevo < 3 && conf < ex.Confianza) {
		return copiar(*ex), false, ErrYaEnsenada
	}
	ex.Anteriores = append(ex.Anteriores, ex.Definicion)
	if len(ex.Anteriores) > MaxAnteriores {
		ex.Anteriores = append([]string(nil), ex.Anteriores[len(ex.Anteriores)-MaxAnteriores:]...)
	}
	ex.Definicion, ex.Conceptos = d, conceptosDe(d)
	ex.Fuente, ex.Origen, ex.Confianza, ex.Fecha, ex.Propio = fuente, origen, conf, ahora, true
	h.cambioDiccionario()
	if entrenar {
		h.entrenarDefinicion(ex.Palabra, d)
	}
	return copiar(*ex), true, nil
}

// entrenarDefinicion teaches the model to say the meaning: "variable es una caja…", "sumar significa…".
func (h *Habla) entrenarDefinicion(palabra, def string) {
	ws := strings.Fields(def)
	if len(ws) == 0 {
		return
	}
	verbo := " significa "
	if articulos[plegar(ws[0])] {
		verbo = " es "
	}
	toks, may := tokensLM(tokenizar(palabra + verbo + def))
	h.lm.entrenar(toks, may)
}

// Olvidar forgets the meaning of a word (seed words stay forgotten).
func (h *Habla) Olvidar(palabra string) error {
	if h == nil {
		return ErrNoLaSe
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.propioEstricto(palabra)
	if s == nil {
		return ErrNoLaSe
	}
	k := clave(s.Palabra)
	delete(h.sig, k)
	for _, x := range semilla {
		if clave(x.palabra) == k {
			h.olvidadas[k] = true
		}
	}
	h.cambioDiccionario()
	h.urgente = true
	return nil
}

// Lista returns every own meaning, sorted by word.
func (h *Habla) Lista() []Significado {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Significado, 0, len(h.sig))
	for _, k := range h.clavesOrdenadas() {
		out = append(out, copiar(*h.sig[k]))
	}
	return out
}

// Leida returns the version of a text already read ("" when never read).
func (h *Habla) Leida(nombre string) string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lecturas[nombre]
}

// MarcarLeida records that a text was read at a version.
func (h *Habla) MarcarLeida(nombre, version string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.lecturas) < 100 || h.lecturas[nombre] != "" {
		h.lecturas[nombre] = version
		h.sucio, h.urgente = true, true
	}
}

// Estado counts what it knows.
type Estado struct {
	ConSignificado int // words with their own definition
	Anclados       int // words that only appear inside definitions
	DeSemilla      int
	DelUsuario     int // taught by the user
	Leidos         int // learned by reading (notes, texts, the web)
	Vocabulario    int // different words the language model has seen
	Frases         int // sentences read to learn to talk
	Ngramas        int // trigrams + bigrams of the model
	Fluidez        float64
	HayFluidez     bool // false without held-out sentences
	Reservadas     int
}

// Estado returns the counts and the fluency (average log-probability per word of held-out sentences).
func (h *Habla) Estado() Estado {
	if h == nil {
		return Estado{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.indexar()
	e := Estado{ConSignificado: len(h.sig), Frases: h.frases, Reservadas: len(h.lm.reservadas)}
	for _, s := range h.sig {
		switch {
		case s.Fuente == FuenteSemilla:
			e.DeSemilla++
		case rangoDe(s.Fuente, s.Confianza) == 3:
			e.DelUsuario++
		default:
			e.Leidos++
		}
	}
	for t := range h.anclas {
		if len(h.porTallo[t]) == 0 {
			e.Anclados++
		}
	}
	for _, w := range h.lm.vocab[numEspeciales:] {
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsLetter(r) {
			e.Vocabulario++
		}
	}
	h.lm.reconstruir()
	e.Ngramas = len(h.lm.tri) + h.lm.bigrams
	e.Fluidez, e.HayFluidez = h.lm.fluidez()
	return e
}

// Desconocidas returns the content words of a text whose meaning it does not know (neither its own
// nor anchored), in order, without repeats (at most 50).
func (h *Habla) Desconocidas(texto string) []string {
	if h == nil {
		return nil
	}
	if len(texto) > MaxLectura {
		texto = recortarBytes(texto, MaxLectura)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	visto := map[string]bool{}
	for _, t := range tokenizar(texto) {
		if t.clase != 'w' || esFuncion(t.pleg) || largo(t.pleg) < 3 || pareceIdentificador(t.texto) || visto[t.pleg] {
			continue
		}
		visto[t.pleg] = true
		if p, a := h.conoce(t.pleg); p || a {
			continue
		}
		out = append(out, strings.ToLower(t.texto))
		if len(out) >= maxDesconocidas {
			break
		}
	}
	return out
}

// pareceIdentificador: capitals after the first letter (SumaPares, HTTP) mark a name, not a word.
func pareceIdentificador(w string) bool {
	for i, r := range w {
		if i > 0 && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// recortarBytes cuts s to at most n bytes at a rune boundary.
func recortarBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// Avisar adds a Spanish warning (shown in the app status).
func (h *Habla) Avisar(s string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.avisar(s)
}
