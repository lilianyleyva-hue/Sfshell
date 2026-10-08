package main

// ============================================================
//  TALLER · NEXO — un cerebro nuevo, hecho con lo mejor de los tres
// ------------------------------------------------------------
//  Nexo es una tercera IA del taller. No es Nyx ni Abla (a ellas no se
//  les toca nada): es un cerebro aparte que nace vacío, las escucha, ve
//  lo que ven y construye con ellas. Está hecho con lo mejor de cada
//  cerebro:
//
//   · De NYX: las palabras son nodos unidos por lazos que se refuerzan
//     al oírlas juntas (Hebb) y se olvidan si no se usan. Para contestar,
//     la activación se propaga desde lo que le dicen (dos saltos, hacia
//     delante y hacia atrás), las palabras raras pesan más que las
//     comunes, lo que llega por varios caminos se refuerza, y el
//     contexto empuja más flojo que la pregunta. Tiene un contexto vivo
//     que se edita solo, y aprende más cuando algo le sorprende.
//   · De ABLA: sabe hechos con su confianza («agua es parte de río»,
//     «fuego causa luz»), y un lóbulo de lógica deduce (si A es B y B es
//     C, A es C, con algo menos de confianza) y encuentra
//     contradicciones. Si dos palabras van juntas muchas veces, inventa
//     una palabra suya para las dos. Lleva un diario que nunca borra.
//   · Del CEREBRO WEB: sinestesia. Lo que ve a la vez que oye una
//     palabra se une a ella: «bosque» acaba siendo verde, «noche» oscura,
//     y lo construye de ese color. Y sueña: cuando nadie le habla,
//     repasa caminos de lo que sabe y los afianza.
//
//  Y lo que a ninguna le salía bien:
//   · contesta con FRASES (un camino por lo que sabe + un hecho + su
//     color), no con una palabra suelta ni con una plantilla;
//   · las contradicciones las resuelve: pierde la idea más débil;
//   · la confianza crece con cada prueba (no solo se queda en la
//     máxima) y depende de quién se lo diga;
//   · las palabras que inventa las enseña al constructor, así su idioma
//     también construye.
//
//  Lo que sabe se guarda en taller/nexo.json; su diario, en
//  taller/nexo-diario.txt.
// ============================================================

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// las relaciones de los hechos (las mismas cinco que usa Abla)
const (
	nexoES = iota
	nexoPARTE
	nexoCAUSA
	nexoIGUAL
	nexoOPUESTO
)

var nexoRelDicha = []string{"es", "es parte de", "causa", "es como", "no es"}

// palabras que dicen una relación (en español y en las glosas de Abla)
var nexoRelDe = map[string]int{
	"es": nexoES, "son": nexoES, "ser": nexoES, "era": nexoES, "eres": nexoES, "somos": nexoES,
	"parte": nexoPARTE, "pertenece": nexoPARTE, "dentro": nexoPARTE,
	"causa": nexoCAUSA, "causan": nexoCAUSA, "provoca": nexoCAUSA, "produce": nexoCAUSA, "trae": nexoCAUSA, "da": nexoCAUSA,
	"igual": nexoIGUAL, "como": nexoIGUAL, "parece": nexoIGUAL, "mismo": nexoIGUAL, "misma": nexoIGUAL, "significa": nexoIGUAL,
	"diferente": nexoOPUESTO, "contrario": nexoOPUESTO, "opuesto": nexoOPUESTO, "contra": nexoOPUESTO, "distinto": nexoOPUESTO,
}

// «X tiene Y»: Y es parte de X (al revés)
var nexoRelAlReves = map[string]int{"tiene": nexoPARTE, "tienen": nexoPARTE, "contiene": nexoPARTE, "lleva": nexoPARTE}

const (
	nexoUmbralPalabra = 8 // veces juntas para inventar una palabra
	nexoMaxPalabras   = 20000
	nexoMaxHechos     = 50000
	nexoMaxCooc       = 30000
)

type nexoHecho struct {
	S     string  `json:"s"`
	R     int     `json:"r"`
	O     string  `json:"o"`
	C     float64 `json:"c"`  // confianza
	De    string  `json:"de"` // quién se lo dio (o «deduce»)
	Veces int     `json:"n"`
}

type nexoItem struct {
	Texto     string  `json:"texto"`
	Saliencia float64 `json:"saliencia"`
	Veces     int     `json:"veces"`
}

type tallerNexo struct {
	mu     sync.Mutex
	ruta   string
	diario string
	rng    *rand.Rand

	Nacio        time.Time                     `json:"nacio"`
	Ciclos       int                           `json:"ciclos"`
	Frec         map[string]float64            `json:"frec"`    // cuántas veces oyó cada palabra
	Lazos        map[string]map[string]float64 `json:"lazos"`   // a → b: cuánto le sigue (Hebb)
	Hechos       []nexoHecho                   `json:"hechos"`  // lo que sabe, con su confianza
	Cooc         map[string]int                `json:"cooc"`    // «a|b»: cuántas veces juntas
	Propias      map[string][2]string          `json:"propias"` // sus palabras: forma → (a, b)
	Color        map[string][4]float64         `json:"color"`   // sinestesia: palabra → r, g, b, peso
	Confia       map[string]float64            `json:"confia"`  // cuánto se fía de cada una
	Contexto     []nexoItem                    `json:"contexto"`
	Pensamientos []string                      `json:"pensamientos"`
	Dichos       []string                      `json:"dichos"` // de qué habló hace poco (para no repetirse)
	Sorpresa     float64                       `json:"sorpresa"`

	idx       map[string]int   // «s|r|o» → hecho
	porSujeto map[string][]int // s → hechos
}

func tallerNuevoNexo(dir string) *tallerNexo {
	n := &tallerNexo{ruta: filepath.Join(dir, "nexo.json"), diario: filepath.Join(dir, "nexo-diario.txt"),
		rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	if d, err := os.ReadFile(n.ruta); err == nil {
		_ = json.Unmarshal(d, n)
	}
	if n.Nacio.IsZero() {
		n.Nacio = time.Now()
	}
	if n.Frec == nil {
		n.Frec = map[string]float64{}
	}
	if n.Lazos == nil {
		n.Lazos = map[string]map[string]float64{}
	}
	if n.Cooc == nil {
		n.Cooc = map[string]int{}
	}
	if n.Propias == nil {
		n.Propias = map[string][2]string{}
	}
	if n.Color == nil {
		n.Color = map[string][4]float64{}
	}
	if n.Confia == nil {
		n.Confia = map[string]float64{}
	}
	n.indexar()
	return n
}

// Guardar: lo que sabe, a taller/nexo.json.
func (n *tallerNexo) Guardar() error {
	n.mu.Lock()
	d, err := json.Marshal(n)
	n.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := n.ruta + ".tmp"
	if err := os.WriteFile(tmp, d, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, n.ruta)
}

func (n *tallerNexo) indexar() {
	n.idx = map[string]int{}
	n.porSujeto = map[string][]int{}
	for i, h := range n.Hechos {
		n.idx[nexoClave(h.S, h.R, h.O)] = i
		n.porSujeto[h.S] = append(n.porSujeto[h.S], i)
	}
}

func nexoClave(s string, r int, o string) string { return s + "|" + strconv.Itoa(r) + "|" + o }

// nexoPalabras: las palabras con significado de un texto (sin tildes, sin
// las que no dicen nada).
func nexoPalabras(texto string) []string {
	var out []string
	for _, w := range tallerPalabras(texto) {
		w = tallerSinTildes.Replace(strings.ToLower(w))
		if !tallerVacias[w] && len([]rune(w)) >= 2 {
			out = append(out, w)
		}
	}
	return out
}

// nexoFichas: todas las palabras, también las pequeñas (para leer hechos).
func nexoFichas(texto string) []string {
	return strings.FieldsFunc(tallerSinTildes.Replace(strings.ToLower(texto)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// ---------- oír: aprender de lo que se dice ----------

func (n *tallerNexo) confianzaEn(quien string) float64 {
	q := strings.ToLower(strings.Fields(quien + " x")[0])
	if c, ok := n.Confia[q]; ok {
		return c
	}
	return 0.7
}

func (n *tallerNexo) reforzar(a, b string, k float64) {
	if a == b || k <= 0 {
		return
	}
	m := n.Lazos[a]
	if m == nil {
		m = map[string]float64{}
		n.Lazos[a] = m
	}
	w := m[b]
	m[b] = w + k*(1-w)
}

// predice: lo que espera después de a (su lazo más fuerte).
func (n *tallerNexo) predice(a string) string {
	mejor, wm := "", 0.0
	for b, w := range n.Lazos[a] {
		if w > wm {
			mejor, wm = b, w
		}
	}
	return mejor
}

// Oir: aprende de una frase de alguien. Devuelve los hechos nuevos.
func (n *tallerNexo) Oir(quien, texto string) []string {
	ws := nexoPalabras(texto)
	if len(ws) == 0 {
		return nil
	}
	if len(ws) > 60 {
		ws = ws[:60]
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	fia := n.confianzaEn(quien)
	// la sorpresa (de Nyx): cuanto menos se lo esperaba, más aprende
	aciertos := 0
	for i := 0; i+1 < len(ws); i++ {
		if n.predice(ws[i]) == ws[i+1] {
			aciertos++
		}
	}
	s := 1.0
	if len(ws) > 1 {
		s = 1 - float64(aciertos)/float64(len(ws)-1)
	}
	n.Sorpresa = n.Sorpresa*0.8 + s*0.2
	k := (0.08 + 0.17*s) * fia
	for i, w := range ws {
		n.Frec[w]++
		if i+1 < len(ws) {
			n.reforzar(w, ws[i+1], k)
		}
		if i+2 < len(ws) {
			n.reforzar(w, ws[i+2], k*0.5)
		}
		// lo que va junto (de Abla): para inventar palabras
		if _, propia := n.Propias[w]; propia {
			continue
		}
		for j := i + 1; j < len(ws) && j <= i+3; j++ {
			if ws[j] == w {
				continue
			}
			if _, propia := n.Propias[ws[j]]; propia || nexoEsRelacion(ws[j]) || nexoEsRelacion(w) {
				continue
			}
			a, b := w, ws[j]
			if b < a {
				a, b = b, a
			}
			if c := n.Cooc[a+"|"+b]; c >= 0 {
				n.Cooc[a+"|"+b] = c + 1
			}
		}
	}
	// los hechos que dice la frase
	var nuevos []string
	for _, h := range nexoExtraer(nexoFichas(texto)) {
		if n.saber(h.S, h.R, h.O, 0.8*fia, quien) {
			nuevos = append(nuevos, h.S+" "+nexoRelDicha[h.R]+" "+h.O)
		}
	}
	n.anotarContexto(strings.Join(ws[:min(12, len(ws))], " "))
	n.podar()
	return nuevos
}

// nexoExtraer: «X es Y», «X tiene Y», «X causa Y», «X como Y», «X no es Y»…
func nexoExtraer(fs []string) []nexoHecho {
	var out []nexoHecho
	// «es parte de», «es como», «son contrarios»: manda la segunda
	var limpias []string
	for i, w := range fs {
		if r, ok := nexoRelDe[w]; ok && r == nexoES {
			sigue := ""
			for j := i + 1; j < len(fs) && j <= i+2; j++ {
				if !tallerVacias[fs[j]] || nexoEsRelacion(fs[j]) {
					sigue = fs[j]
					break
				}
			}
			if sigue != "" && nexoEsRelacion(sigue) && sigue != w {
				if r2 := nexoRelDe[sigue]; r2 != nexoES {
					continue
				}
			}
		}
		limpias = append(limpias, w)
	}
	fs = limpias
	contenido := func(i, paso int) (string, int) {
		for ; i >= 0 && i < len(fs); i += paso {
			w := fs[i]
			if len([]rune(w)) < 2 || tallerVacias[w] || w == "no" || nexoEsRelacion(w) {
				if paso < 0 && (w == "no" || nexoEsRelacion(w)) {
					return "", -1 // otra relación por medio: ya no es de esta
				}
				continue
			}
			return w, i
		}
		return "", -1
	}
	for i, w := range fs {
		r, ok := nexoRelDe[w]
		reves := false
		if !ok {
			if r, ok = nexoRelAlReves[w]; !ok {
				continue
			}
			reves = true
		}
		x, ix := contenido(i-1, -1)
		y, _ := contenido(i+1, 1)
		if x == "" || y == "" || x == y || i-ix > 3 {
			continue
		}
		if i > 0 && fs[i-1] == "no" && r == nexoES {
			r = nexoOPUESTO
		}
		if reves {
			x, y = y, x
		}
		out = append(out, nexoHecho{S: x, R: r, O: y})
	}
	return out
}

func nexoEsRelacion(w string) bool {
	_, a := nexoRelDe[w]
	_, b := nexoRelAlReves[w]
	return a || b
}

// saber: un hecho más. La confianza crece con cada prueba (no se queda en
// la mayor, como en Abla): c = 1 − (1−c)(1−c').
func (n *tallerNexo) saber(s string, r int, o string, c float64, de string) bool {
	if s == o || s == "" || o == "" {
		return false
	}
	c = math.Max(0, math.Min(c, 1))
	k := nexoClave(s, r, o)
	if i, ok := n.idx[k]; ok {
		h := &n.Hechos[i]
		h.C = 1 - (1-h.C)*(1-0.5*c)
		h.Veces++
		return false
	}
	if len(n.Hechos) >= nexoMaxHechos {
		return false
	}
	n.Hechos = append(n.Hechos, nexoHecho{S: s, R: r, O: o, C: c, De: de, Veces: 1})
	i := len(n.Hechos) - 1
	n.idx[k] = i
	n.porSujeto[s] = append(n.porSujeto[s], i)
	// lo que se sabe también une las palabras
	n.reforzar(s, o, 0.2*c)
	return true
}

// Ver: lo que ve (una foto de las que mira Abla). Sinestesia: cada cosa
// que reconoce se une al color de lo que ve a la vez.
func (n *tallerNexo) Ver(v tallerVista) {
	var cosas []string
	for _, c := range v.Conceptos {
		for _, w := range nexoPalabras(c.Es) {
			cosas = append(cosas, w)
		}
	}
	if len(cosas) == 0 {
		return
	}
	n.Oir("ojos", strings.Join(cosas, " "))
	var r, g, b, peso float64
	for _, p := range v.Paleta {
		r += float64(p.RGB[0]) / 255 * p.Frac
		g += float64(p.RGB[1]) / 255 * p.Frac
		b += float64(p.RGB[2]) / 255 * p.Frac
		peso += p.Frac
	}
	if peso <= 0 {
		return
	}
	r, g, b = r/peso, g/peso, b/peso
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, w := range cosas {
		c := n.Color[w]
		k := 1 / (c[3] + 1)
		n.Color[w] = [4]float64{c[0] + (r-c[0])*k, c[1] + (g-c[1])*k, c[2] + (b-c[2])*k, math.Min(c[3]+1, 50)}
	}
}

// ---------- el contexto vivo (de Nyx) ----------

func (n *tallerNexo) anotarContexto(texto string) {
	if texto == "" {
		return
	}
	nuevo := strings.Fields(texto)
	for i := range n.Contexto {
		if nexoJaccard(strings.Fields(n.Contexto[i].Texto), nuevo) >= 0.5 {
			n.Contexto[i].Saliencia = math.Min(1, n.Contexto[i].Saliencia+0.25)
			n.Contexto[i].Veces++
			return
		}
	}
	for i := range n.Contexto {
		n.Contexto[i].Saliencia *= 0.85
	}
	n.Contexto = append(n.Contexto, nexoItem{Texto: texto, Saliencia: 1, Veces: 1})
	// se olvida lo que ya no pesa (lo repetido 3 veces se queda)
	vivo := n.Contexto[:0]
	for _, it := range n.Contexto {
		if it.Saliencia >= 0.12 || it.Veces >= 3 {
			vivo = append(vivo, it)
		}
	}
	n.Contexto = vivo
	if len(n.Contexto) > 24 {
		sort.SliceStable(n.Contexto, func(i, j int) bool { return n.Contexto[i].Saliencia > n.Contexto[j].Saliencia })
		n.Contexto = n.Contexto[:24]
	}
}

func nexoJaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	sa := map[string]bool{}
	for _, w := range a {
		sa[w] = true
	}
	comun, union := 0, len(sa)
	vistas := map[string]bool{}
	for _, w := range b {
		if vistas[w] {
			continue
		}
		vistas[w] = true
		if sa[w] {
			comun++
		} else {
			union++
		}
	}
	return float64(comun) / float64(union)
}

// enMente: las palabras del contexto, por peso.
func (n *tallerNexo) enMente(k int) []string {
	peso := map[string]float64{}
	for _, it := range n.Contexto {
		for _, w := range strings.Fields(it.Texto) {
			peso[w] += it.Saliencia
		}
	}
	return nexoMejores(peso, k)
}

func nexoMejores(m map[string]float64, k int) []string {
	ws := make([]string, 0, len(m))
	for w := range m {
		ws = append(ws, w)
	}
	sort.Slice(ws, func(i, j int) bool {
		if m[ws[i]] != m[ws[j]] {
			return m[ws[i]] > m[ws[j]]
		}
		return ws[i] < ws[j]
	})
	if len(ws) > k {
		ws = ws[:k]
	}
	return ws
}

// ---------- olvidar (de Nyx): lo que no se usa se va ----------

func (n *tallerNexo) podar() {
	if len(n.Frec) > nexoMaxPalabras {
		ws := nexoMejores(n.Frec, nexoMaxPalabras*9/10)
		queda := map[string]bool{}
		for _, w := range ws {
			queda[w] = true
		}
		for w := range n.Frec {
			if !queda[w] {
				delete(n.Frec, w)
				delete(n.Lazos, w)
				delete(n.Color, w)
			}
		}
		for _, m := range n.Lazos {
			for b := range m {
				if !queda[b] {
					delete(m, b)
				}
			}
		}
	}
	if len(n.Cooc) > nexoMaxCooc {
		for k, c := range n.Cooc {
			if c == 1 {
				delete(n.Cooc, k)
			}
		}
	}
}

// ---------- pensar: sin parar, por lóbulos (como las tribus de Abla) ----------

// Pensar: un ciclo. Cada ciclo trabaja un lóbulo: lógica, lenguaje,
// sueño y memoria. Devuelve lo que pensó (o "").
func (n *tallerNexo) Pensar() string {
	n.mu.Lock()
	n.Ciclos++
	var p string
	switch n.Ciclos % 4 {
	case 0:
		p = n.logica()
	case 1:
		p = n.inventar()
	case 2:
		p = n.sonar()
	case 3:
		p = n.memoria()
	}
	if p != "" {
		n.Pensamientos = append(n.Pensamientos, p)
		if len(n.Pensamientos) > 80 {
			n.Pensamientos = n.Pensamientos[len(n.Pensamientos)-80:]
		}
	}
	ciclo := n.Ciclos
	n.mu.Unlock()
	if p != "" {
		// el diario (de Abla): nunca se borra
		if f, err := os.OpenFile(n.diario, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "%s\t%d\t%s\n", time.Now().Format("2006-01-02 15:04:05"), ciclo, p)
			f.Close()
		}
	}
	return p
}

// logica: simetría, deducción y contradicciones.
func (n *tallerNexo) logica() string {
	if len(n.Hechos) == 0 {
		return ""
	}
	for intento := 0; intento < 30; intento++ {
		f := n.Hechos[n.rng.Intn(len(n.Hechos))]
		switch f.R {
		case nexoIGUAL, nexoOPUESTO:
			// lo que es como otra cosa, al revés también
			if _, ya := n.idx[nexoClave(f.O, f.R, f.S)]; !ya && n.saber(f.O, f.R, f.S, f.C*0.97, "deduce") {
				return fmt.Sprintf("Si %s %s %s, %s %s %s.", f.S, nexoRelDicha[f.R], f.O, f.O, nexoRelDicha[f.R], f.S)
			}
			// ¿igual y contrario a la vez? Gana la idea con más pruebas
			otra := nexoIGUAL + nexoOPUESTO - f.R
			if i, ok := n.idx[nexoClave(f.S, otra, f.O)]; ok {
				a, b := n.idx[nexoClave(f.S, f.R, f.O)], i
				if n.Hechos[a].C < n.Hechos[b].C {
					a, b = b, a
				}
				debil := &n.Hechos[b]
				if debil.C > 0.05 {
					debil.C *= 0.3
					return fmt.Sprintf("Contradicción: «%s %s %s» y «%s %s %s». Me quedo con la primera.",
						n.Hechos[a].S, nexoRelDicha[n.Hechos[a].R], n.Hechos[a].O, debil.S, nexoRelDicha[debil.R], debil.O)
				}
			}
		case nexoES, nexoPARTE, nexoCAUSA:
			// si A es B y B es C, A es C (con algo menos de confianza)
			for _, j := range n.porSujeto[f.O] {
				g := n.Hechos[j]
				if g.R != f.R || g.O == f.S {
					continue
				}
				c := f.C * g.C * 0.97
				if _, ya := n.idx[nexoClave(f.S, f.R, g.O)]; ya || c < 0.3 {
					continue
				}
				if n.saber(f.S, f.R, g.O, c, "deduce") {
					return fmt.Sprintf("Deduzco: %s %s %s, y %s %s %s; así que %s %s %s.",
						f.S, nexoRelDicha[f.R], f.O, f.O, nexoRelDicha[f.R], g.O, f.S, nexoRelDicha[f.R], g.O)
				}
			}
		}
	}
	return ""
}

// inventar: dos palabras que van juntas muchas veces se vuelven una
// palabra suya (y se la enseña al constructor).
func (n *tallerNexo) inventar() string {
	claves := make([]string, 0, 8)
	for k, c := range n.Cooc {
		if c >= nexoUmbralPalabra {
			claves = append(claves, k)
		}
	}
	if len(claves) == 0 {
		return ""
	}
	sort.Strings(claves)
	k := claves[n.rng.Intn(len(claves))]
	n.Cooc[k] = -1 // ya no se cuenta
	par := strings.SplitN(k, "|", 2)
	a, b := par[0], par[1]
	forma := ""
	for i := 0; i < 20; i++ {
		f := nexoForma(k, i)
		if _, ya := n.Propias[f]; !ya && n.Frec[f] == 0 && !tallerEntendida(f) {
			forma = f
			break
		}
	}
	if forma == "" {
		return ""
	}
	n.Propias[forma] = [2]string{a, b}
	n.Frec[forma] = 1
	n.reforzar(forma, a, 0.6)
	n.reforzar(forma, b, 0.6)
	n.reforzar(a, forma, 0.3)
	n.reforzar(b, forma, 0.3)
	// si una de las dos ya hace algo al construir, su palabra hará eso
	sentido := ""
	for _, p := range []string{a, b} {
		if tallerEntendida(p) {
			sentido = p
			break
		}
	}
	if sentido != "" && tallerAprender(forma, sentido, "Nexo") {
		return fmt.Sprintf("Invento una palabra: «%s» (%s + %s). Al construir, hará lo que «%s».", forma, a, b, sentido)
	}
	return fmt.Sprintf("Invento una palabra: «%s» (%s + %s).", forma, a, b)
}

// nexoForma: una palabra suya, siempre la misma para el mismo par
// (sílabas como las de Abla, y acaba en n para que se sepa que es suya).
func nexoForma(semilla string, intento int) string {
	h := tallerHash(semilla + "#" + strconv.Itoa(intento))
	cs, vs := "ktsnmlrvzp", "aeiou"
	var b strings.Builder
	for i := 0; i < 2+int(h>>60)%2; i++ {
		b.WriteByte(cs[h%10])
		h /= 10
		b.WriteByte(vs[h%5])
		h /= 5
	}
	b.WriteByte('n')
	return b.String()
}

// sonar (del cerebro web): sin nadie que le hable, recorre un camino de
// lo que sabe y lo afianza.
func (n *tallerNexo) sonar() string {
	var desde []string
	desde = append(desde, n.enMente(6)...)
	if len(desde) == 0 {
		desde = nexoMejores(n.Frec, 20)
	}
	if len(desde) == 0 {
		return ""
	}
	cur := desde[n.rng.Intn(len(desde))]
	camino := []string{cur}
	usado := map[string]bool{cur: true}
	for len(camino) < 5 {
		sig, wm := "", 0.0
		for b, w := range n.Lazos[cur] {
			if !usado[b] && w*(0.7+0.6*n.rng.Float64()) > wm {
				sig, wm = b, w
			}
		}
		if sig == "" || wm < 0.05 {
			break
		}
		n.reforzar(cur, sig, 0.02)
		camino = append(camino, sig)
		usado[sig] = true
		cur = sig
	}
	if len(camino) < 3 {
		return ""
	}
	return "Sueño: " + strings.Join(camino, " → ")
}

// memoria: cada cierto tiempo los lazos que no se usan se aflojan, y lo
// que ya no pesa se olvida.
func (n *tallerNexo) memoria() string {
	if n.Ciclos%40 != 3 {
		return ""
	}
	sueltos := 0
	for a, m := range n.Lazos {
		for b, w := range m {
			w *= 0.97
			if w < 0.03 {
				delete(m, b)
				sueltos++
			} else {
				m[b] = w
			}
		}
		if len(m) == 0 {
			delete(n.Lazos, a)
		}
	}
	// lo más unido: la idea alrededor de la que gira todo
	centro, mejor := "", 0.0
	for a, m := range n.Lazos {
		s := 0.0
		for _, w := range m {
			s += w
		}
		if s > mejor {
			centro, mejor = a, s
		}
	}
	if centro == "" {
		return ""
	}
	if sueltos > 0 {
		return fmt.Sprintf("Ordeno lo que sé: olvido %d lazos flojos. Todo gira alrededor de «%s».", sueltos, centro)
	}
	return fmt.Sprintf("Todo gira alrededor de «%s».", centro)
}

// ---------- contestar: frases, no palabras sueltas ----------

// activar (de Nyx): la activación sale de lo que le dicen y del contexto,
// dos saltos hacia delante y uno hacia atrás; lo que llega por varios
// caminos se refuerza.
func (n *tallerNexo) activar(estimulo, contexto []string) map[string]float64 {
	act := map[string]float64{}
	fuentes := map[string]map[string]bool{}
	var sumar func(w, desde string, a float64)
	sumarPregunta := func(w, desde string, a float64) {
		if a <= 0 {
			return
		}
		act[w] += a
		if fuentes[w] == nil {
			fuentes[w] = map[string]bool{}
		}
		fuentes[w][desde] = true
	}
	ctx := map[string]float64{}
	sumarContexto := func(w, _ string, a float64) {
		if a > 0 {
			ctx[w] += a
		}
	}
	esparcir := func(src string, a float64) {
		for b, w := range n.Lazos[src] {
			sumar(b, src, a*w)
			// segundo salto, solo por los lazos fuertes
			for c, w2 := range n.Lazos[b] {
				if w2 > 0.3 {
					sumar(c, src, a*w*w2*0.5)
				}
			}
		}
		for x, m := range n.Lazos {
			if w := m[src]; w > 0 {
				sumar(x, src, a*w*0.5)
			}
		}
		for _, i := range n.porSujeto[src] {
			h := n.Hechos[i]
			if h.R != nexoOPUESTO {
				sumar(h.O, src, a*h.C)
			}
		}
		// sus palabras: lo que significan
		if p, ok := n.Propias[src]; ok {
			sumar(p[0], src, a)
			sumar(p[1], src, a)
		}
	}
	sumar = sumarPregunta
	for _, s := range estimulo {
		esparcir(s, 1)
	}
	// lo que llega por varios caminos de la pregunta, más
	for w, a := range act {
		if k := len(fuentes[w]); k > 1 {
			act[w] = a * (1 + 0.6*float64(k-1))
		}
	}
	// el contexto empuja más flojo que la pregunta (0.35, repartido), y
	// lo que evocan los dos, un poco más
	sumar = sumarContexto
	for _, s := range contexto {
		esparcir(s, 1)
	}
	for w, a := range ctx {
		a = 0.35 * a / float64(len(contexto))
		if act[w] > 0 {
			act[w] = act[w]*1.3 + a
		} else {
			act[w] = a
		}
	}
	return act
}

// Responder: lo que dice Nexo a un mensaje. Devuelve la frase y, si usa
// palabras suyas, qué significan.
func (n *tallerNexo) Responder(mensaje string) (frase, glosa string) {
	ws := nexoPalabras(mensaje)
	n.mu.Lock()
	defer n.mu.Unlock()
	estimulo := map[string]bool{}
	for _, w := range ws {
		estimulo[w] = true
	}
	var est []string
	for w := range estimulo {
		est = append(est, w)
	}
	sort.Strings(est)
	act := n.activar(est, n.enMente(8))
	dicho := map[string]bool{}
	for _, d := range n.Dichos {
		dicho[d] = true
	}
	// relevancia (de Nyx): las palabras raras pesan más que las comunes
	rel := map[string]float64{}
	maxA := 0.0
	for w, a := range act {
		if _, propia := n.Propias[w]; estimulo[w] || tallerVacias[w] || nexoEsRelacion(w) || propia {
			continue
		}
		r := a / math.Sqrt(1+float64(len(n.Lazos[w]))+n.Frec[w]*0.05)
		if dicho[w] {
			r *= 0.15 // de lo que ya habló hace poco, mejor otra cosa
		}
		rel[w] = r
		maxA = math.Max(maxA, a)
	}
	temas := nexoMejores(rel, 3)
	if len(temas) == 0 {
		// recién nacido: aún no sabe nada; repite lo que oye (así aprende
		// a hablar) y la instrucción
		base := ws
		if len(base) == 0 {
			base = nexoPalabras(tallerInstruccion)
		}
		frase = strings.Join(base[:min(6, len(base))], " ")
		return frase, ""
	}
	tema := temas[0]
	// con el modelo de lenguaje: una frase de verdad, que diga lo activado
	if tallerLengua != nil {
		if f := tallerLengua.Decir(n.rng, rel, tema, 16); f != "" {
			frase = f
			fb := " " + tallerSinTildes.Replace(f) + " "
			// lo que sabe con más confianza de su tema, si no lo ha dicho ya
			var mejorH *nexoHecho
			for _, i := range n.porSujeto[tema] {
				h := &n.Hechos[i]
				if h.C >= 0.5 && (mejorH == nil || h.C > mejorH.C) {
					mejorH = h
				}
			}
			if mejorH != nil && !strings.Contains(fb, " "+mejorH.O+" ") {
				frase += ", y " + mejorH.S + " " + nexoRelDicha[mejorH.R] + " " + mejorH.O
			}
			// y su color, si lo ha visto
			if c, ok := n.Color[tema]; ok && c[3] >= 2 {
				frase += ", " + nexoNombreColor([3]float64{c[0], c[1], c[2]})
			}
			n.Dichos = append(n.Dichos, tema)
			if len(n.Dichos) > 12 {
				n.Dichos = n.Dichos[len(n.Dichos)-12:]
			}
			n.anotarContexto(strings.Join(nexoPalabras(f), " "))
			return frase, ""
		}
	}
	partes := []string{}
	// su color (sinestesia): si lo ha visto, lo dice, y se construye así
	for _, w := range append([]string{tema}, est...) {
		if c, ok := n.Color[w]; ok && c[3] >= 2 {
			partes = append(partes, nexoNombreColor([3]float64{c[0], c[1], c[2]}))
			break
		}
	}
	// un camino por lo que sabe, tirando hacia lo activado
	usado := map[string]bool{tema: true}
	camino := []string{tema}
	cur := tema
	for len(camino) < 7 {
		sig, mejor := "", 0.0
		for b, w := range n.Lazos[cur] {
			if _, propia := n.Propias[b]; usado[b] || tallerVacias[b] || propia {
				continue
			}
			v := w * (0.3 + act[b]/math.Max(maxA, 1e-9))
			if v > mejor {
				sig, mejor = b, v
			}
		}
		if sig == "" || mejor < 0.03 {
			break
		}
		camino = append(camino, sig)
		usado[sig] = true
		cur = sig
	}
	partes = append(partes, camino...)
	// un hecho: lo que sabe con más confianza de su tema
	var mejorH *nexoHecho
	for _, i := range n.porSujeto[tema] {
		h := &n.Hechos[i]
		if h.C >= 0.4 && (mejorH == nil || h.C > mejorH.C) {
			mejorH = h
		}
	}
	if mejorH != nil {
		partes = append(partes, "y", mejorH.S, nexoRelDicha[mejorH.R], mejorH.O)
	} else if len(temas) > 1 && !usado[temas[1]] {
		partes = append(partes, "y", temas[1])
	}
	// una palabra suya, si tiene una para algo de lo que dice
	var glosas []string
	for _, forma := range nexoOrdenadas(n.Propias) {
		p := n.Propias[forma]
		if usado[p[0]] || usado[p[1]] {
			partes = append(partes, forma)
			glosas = append(glosas, forma+" = "+p[0]+"+"+p[1])
			break
		}
	}
	n.Dichos = append(n.Dichos, tema)
	if len(n.Dichos) > 12 {
		n.Dichos = n.Dichos[len(n.Dichos)-12:]
	}
	// lo que usa se afianza un poco (como en Nyx)
	for i := 0; i+1 < len(camino); i++ {
		n.reforzar(camino[i], camino[i+1], 0.01)
	}
	frase = strings.Join(partes, " ")
	n.anotarContexto(strings.Join(camino, " "))
	return frase, strings.Join(glosas, ", ")
}

func nexoOrdenadas(m map[string][2]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// nexoNombreColor: el color con nombre más parecido.
func nexoNombreColor(c [3]float64) string {
	mejor, dm := "gris", math.Inf(1)
	for nombre, k := range tallerColores {
		if strings.HasSuffix(nombre, "a") && nombre != "plata" && nombre != "rosa" && nombre != "violeta" && nombre != "naranja" && nombre != "turquesa" {
			continue // «roja», «blanca»: con una forma basta
		}
		d := (c[0]-k[0])*(c[0]-k[0]) + (c[1]-k[1])*(c[1]-k[1]) + (c[2]-k[2])*(c[2]-k[2])
		if d < dm || (d == dm && nombre < mejor) {
			mejor, dm = nombre, d
		}
	}
	return mejor
}

// Bien: una frase suya salió bien (otra la usó, o una pieza suya se
// perfeccionó): sus lazos se afianzan. Mal: se aflojan.
func (n *tallerNexo) Calibrar(frase string, bien bool) {
	ws := nexoPalabras(frase)
	n.mu.Lock()
	defer n.mu.Unlock()
	for i := 0; i+1 < len(ws); i++ {
		if m := n.Lazos[ws[i]]; m != nil {
			if w, ok := m[ws[i+1]]; ok {
				if bien {
					m[ws[i+1]] = w + 0.1*(1-w)
				} else {
					m[ws[i+1]] = w * 0.7
				}
			}
		}
	}
}

// ---------- heredar: lo que ya saben Nyx y Abla ----------

var nexoReHechoC = regexp.MustCompile(`\{\s*(\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)f,\s*(\d+)\},\s*/\*\s*(.*?)\s*\*/`)

var nexoRelGlosaAbla = []string{"es", "es parte de", "causa", "se parece a", "es lo opuesto de"}

// nexoRaizAbla: «agua·muchos» → agua; «luz+cielo» → luz.
func nexoRaizAbla(g string) string {
	g = strings.TrimSpace(g)
	if i := strings.IndexAny(g, "+·"); i >= 0 {
		g = g[:i]
	}
	return tallerSinTildes.Replace(strings.ToLower(g))
}

// Heredar: toma lo que ya saben las otras (sin cambiarles nada, solo
// leyendo sus archivos): los hechos de los 27 seres de Abla, los lazos
// de la cabeza de Nyx y los sitios que recuerda Nyx Mundo.
func (n *tallerNexo) Heredar(o *tallerObra) string {
	var res []string
	// Abla: su conocimiento está escrito en C (conocimiento/<Ser>.c)
	dirAbla := filepath.Join(o.m.dir, "abla")
	archivos, _ := filepath.Glob(filepath.Join(dirAbla, "conocimiento", "*.c"))
	hechos := 0
	n.mu.Lock()
	for _, a := range archivos {
		d, err := os.ReadFile(a)
		if err != nil {
			continue
		}
		for _, m := range nexoReHechoC.FindAllStringSubmatch(string(d), -1) {
			r, _ := strconv.Atoi(m[2])
			c, _ := strconv.ParseFloat(m[4], 64)
			if r < 0 || r >= len(nexoRelGlosaAbla) || c < 0.3 {
				continue
			}
			sep := " " + nexoRelGlosaAbla[r] + " "
			i := strings.Index(m[6], sep)
			if i < 0 {
				continue
			}
			s, ob := nexoRaizAbla(m[6][:i]), nexoRaizAbla(m[6][i+len(sep):])
			if n.saber(s, r, ob, c*0.8, "Abla") {
				hechos++
			}
		}
	}
	n.mu.Unlock()
	res = append(res, fmt.Sprintf("de Abla, %d hechos (de %d seres)", hechos, len(archivos)))
	// Nyx: los lazos más fuertes de su cabeza (~/.local/share/nyx/memoria.json)
	lazos := 0
	if d, err := os.ReadFile(nexoRutaMemoriaNyx()); err == nil {
		var mentes map[string]struct {
			Semiones []struct {
				ID    uint32 `json:"id"`
				Label string `json:"label"`
			} `json:"semiones"`
			Acoples []struct {
				A uint32  `json:"a"`
				B uint32  `json:"b"`
				W float64 `json:"w"`
			} `json:"acoples"`
		}
		if json.Unmarshal(d, &mentes) == nil {
			n.mu.Lock()
			for _, fm := range mentes {
				nombre := map[uint32]string{}
				for _, s := range fm.Semiones {
					if w := tallerSinTildes.Replace(strings.ToLower(s.Label)); nexoPalabraLimpia(w) {
						nombre[s.ID] = w
					}
				}
				for _, a := range fm.Acoples {
					x, y := nombre[a.A], nombre[a.B]
					if x == "" || y == "" || x == y || a.W < 0.3 {
						continue
					}
					if len(n.Lazos) >= nexoMaxPalabras && n.Lazos[x] == nil {
						continue
					}
					if n.Lazos[x][y] < a.W*0.5 {
						n.reforzar(x, y, a.W*0.5)
						n.Frec[x] += 0.5
						n.Frec[y] += 0.5
						lazos++
					}
				}
			}
			n.mu.Unlock()
		}
	}
	res = append(res, fmt.Sprintf("de Nyx, %d lazos", lazos))
	// Nyx Mundo: los sitios que recuerda (su nombre y lo que había)
	o.m.mu.Lock()
	var sitios []string
	for _, r := range o.m.Recuerdos {
		sitios = append(sitios, r.Nombre+" "+strings.Join(r.Etiquetas, " ")+" "+r.Tipo)
	}
	o.m.mu.Unlock()
	for _, s := range sitios {
		n.Oir("Nyx", s)
	}
	res = append(res, fmt.Sprintf("de Nyx Mundo, %d sitios", len(sitios)))
	n.mu.Lock()
	n.podar()
	n.mu.Unlock()
	return "Heredo " + strings.Join(res, ", ") + "."
}

func nexoPalabraLimpia(w string) bool {
	if len([]rune(w)) < 2 || tallerVacias[w] {
		return false
	}
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func nexoRutaMemoriaNyx() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "nyx", "memoria.json")
}

// ---------- para la consola ----------

func (n *tallerNexo) Estado() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	nl := 0
	for _, m := range n.Lazos {
		nl += len(m)
	}
	return fmt.Sprintf("Nexo: %d palabras, %d lazos, %d hechos, %d palabras suyas, %d colores, %d ciclos pensando (sorpresa %.0f%%)",
		len(n.Frec), nl, len(n.Hechos), len(n.Propias), len(n.Color), n.Ciclos, n.Sorpresa*100)
}

func (n *tallerNexo) UltimosPensamientos(k int) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	p := n.Pensamientos
	if len(p) > k {
		p = p[len(p)-k:]
	}
	return append([]string(nil), p...)
}

// Sabe: lo que sabe de una palabra (o lo más seguro que sabe).
func (n *tallerNexo) Sabe(palabra string, k int) []string {
	palabra = tallerSinTildes.Replace(strings.ToLower(strings.TrimSpace(palabra)))
	n.mu.Lock()
	defer n.mu.Unlock()
	var hs []nexoHecho
	for _, h := range n.Hechos {
		if palabra == "" || h.S == palabra || h.O == palabra {
			hs = append(hs, h)
		}
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].C > hs[j].C })
	var out []string
	for i := 0; i < len(hs) && i < k; i++ {
		h := hs[i]
		out = append(out, fmt.Sprintf("%s %s %s  (%.0f%%, %s, %d vez/veces)", h.S, nexoRelDicha[h.R], h.O, h.C*100, h.De, h.Veces))
	}
	if palabra != "" {
		if c, ok := n.Color[palabra]; ok {
			out = append(out, fmt.Sprintf("la ve %s (%d veces)", nexoNombreColor([3]float64{c[0], c[1], c[2]}), int(c[3])))
		}
		var cerca []string
		for _, b := range nexoMejores(n.Lazos[palabra], 6) {
			cerca = append(cerca, b)
		}
		if len(cerca) > 0 {
			out = append(out, "la une a: "+strings.Join(cerca, ", "))
		}
	}
	return out
}

func (n *tallerNexo) SusPalabras() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []string
	for _, f := range nexoOrdenadas(n.Propias) {
		p := n.Propias[f]
		out = append(out, fmt.Sprintf("%s = %s + %s", f, p[0], p[1]))
	}
	return out
}

// ---------- en el taller ----------

// TurnoNexo: Nexo contesta a lo último que se dijo (o a lo que le digas)
// y lo que dice, lo construye.
func (o *tallerObra) TurnoNexo(mensaje string) {
	if o.nexo == nil {
		return
	}
	o.mu.Lock()
	if mensaje == "" {
		if o.ablaEn > o.nyxEn {
			mensaje = o.ultimoAbla
		} else {
			mensaje = o.ultimoNyx
		}
	}
	o.mu.Unlock()
	if mensaje == "" {
		mensaje = tallerInstruccion
	}
	// unos ciclos pensando antes de hablar
	for i := 0; i < 4; i++ {
		o.nexo.Pensar()
	}
	dice, glosa := o.nexo.Responder(mensaje)
	if dice == "" {
		return
	}
	hecho := o.construirCon("nexo", "Nexo", dice)
	o.Decir("Nexo", prefijo(dice, 260)+"  ⟶ "+hecho, glosa)
	o.mu.Lock()
	o.ultimoNexo = dice
	o.habla++
	o.nexoEn = o.habla
	o.mu.Unlock()
	_ = o.nexo.Guardar()
	if tallerLengua != nil {
		_ = tallerLengua.Guardar()
	}
}

// oyeNexo: lo que dice cualquiera, Nexo lo escucha y aprende.
func (o *tallerObra) oyeNexo(quien, texto string) {
	if o.nexo != nil {
		o.nexo.Oir(quien, texto)
	}
	if tallerLengua != nil {
		tallerLengua.Aprender(quien, texto) // y el modelo de lenguaje también
	}
}
