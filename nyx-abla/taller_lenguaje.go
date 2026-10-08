package main

// ============================================================
//  TALLER · LENGUA — el modelo de lenguaje del taller
// ------------------------------------------------------------
//  Un modelo de lenguaje de verdad, pequeño y local: predice la palabra
//  siguiente a partir de las dos anteriores (trigramas con suavizado de
//  Kneser-Ney, que reparte la probabilidad entre lo que nunca ha oído).
//  Está hecho con lo mejor de cada una:
//
//   · Aprende de todo lo que se dice (Nyx, Abla con su traducción, tú, y
//     los textos que le des con `lengua lee`), nunca de lo que dice él
//     mismo (así no se repite en bucle).
//   · Para decidir DE QUÉ hablar usa la activación de Nexo, que viene de
//     Nyx (lazos que se refuerzan, palabras raras que pesan más), los
//     hechos que se deducen como en Abla y el color que se une a cada
//     cosa, como en el cerebro web.
//   · Para decidir CÓMO decirlo, imagina varias frases y se queda con la
//     que mejor junta las dos cosas: lo que quiere decir y lo bien que
//     suena (la probabilidad del modelo).
//
//  Lo usan Nexo y las 36 IAs del mundo 3D. Se guarda en
//  taller/lengua.json.
// ============================================================

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
)

const (
	lmInicio      = "<s>"
	lmFin         = "</s>"
	lmDescuento   = 0.75
	lmMaxContexto = 150000 // contextos de dos palabras que recuerda
)

type tallerLM struct {
	mu   sync.Mutex
	ruta string

	Tokens  int                       `json:"tokens"`
	Uni     map[string]int            `json:"uni"`
	Bi      map[string]map[string]int `json:"bi"`
	Tri     map[string]map[string]int `json:"tri"`     // «a b» → siguiente
	Cont    map[string]int            `json:"cont"`    // de cuántas palabras distintas viene cada una (Kneser-Ney)
	Fuentes map[string]int            `json:"fuentes"` // cuántas palabras aprendió de cada una
	biTipos int
}

// tallerLengua: el modelo de lenguaje del taller (lo abre la obra).
var tallerLengua *tallerLM

func tallerNuevoLM(dir string) *tallerLM {
	lm := &tallerLM{ruta: filepath.Join(dir, "lengua.json")}
	if d, err := os.ReadFile(lm.ruta); err == nil {
		_ = json.Unmarshal(d, lm)
	}
	if lm.Uni == nil {
		lm.Uni = map[string]int{}
	}
	if lm.Bi == nil {
		lm.Bi = map[string]map[string]int{}
	}
	if lm.Tri == nil {
		lm.Tri = map[string]map[string]int{}
	}
	if lm.Cont == nil {
		lm.Cont = map[string]int{}
	}
	if lm.Fuentes == nil {
		lm.Fuentes = map[string]int{}
	}
	for _, m := range lm.Bi {
		lm.biTipos += len(m)
	}
	return lm
}

func (lm *tallerLM) Guardar() error {
	lm.mu.Lock()
	d, err := json.Marshal(lm)
	lm.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := lm.ruta + ".tmp"
	if err := os.WriteFile(tmp, d, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, lm.ruta)
}

// lmFrases: el texto en frases de palabras (en minúscula, con sus tildes).
func lmFrases(texto string) [][]string {
	var frases [][]string
	var cur []string
	var w strings.Builder
	cortar := func() {
		if w.Len() > 0 {
			cur = append(cur, w.String())
			w.Reset()
		}
	}
	fin := func() {
		cortar()
		if len(cur) > 0 {
			frases = append(frases, cur)
			cur = nil
		}
	}
	for _, r := range strings.ToLower(texto) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			w.WriteRune(r)
		case r == '.' || r == '!' || r == '?' || r == '\n' || r == ';' || r == '¿' || r == '¡' || r == '«' || r == '»' || r == '(' || r == ')' || r == '⟶':
			fin()
		default:
			cortar()
		}
	}
	fin()
	return frases
}

// Aprender: un texto más, de alguien.
func (lm *tallerLM) Aprender(quien, texto string) int {
	n := 0
	lm.mu.Lock()
	defer lm.mu.Unlock()
	for _, f := range lmFrases(texto) {
		if len(f) > 60 {
			f = f[:60]
		}
		ws := append(append([]string{lmInicio, lmInicio}, f...), lmFin)
		for i := 2; i < len(ws); i++ {
			a, b, w := ws[i-2], ws[i-1], ws[i]
			lm.Uni[w]++
			m := lm.Bi[b]
			if m == nil {
				m = map[string]int{}
				lm.Bi[b] = m
			}
			if m[w] == 0 {
				lm.Cont[w]++
				lm.biTipos++
			}
			m[w]++
			k := a + " " + b
			t := lm.Tri[k]
			if t == nil {
				t = map[string]int{}
				lm.Tri[k] = t
			}
			t[w]++
			n++
		}
	}
	lm.Tokens += n
	q := strings.ToLower(strings.Fields(quien + " x")[0])
	lm.Fuentes[q] += n
	lm.podar()
	return n
}

// podar: si recuerda demasiados contextos, olvida los que oyó una vez.
func (lm *tallerLM) podar() {
	if len(lm.Tri) <= lmMaxContexto {
		return
	}
	for k, m := range lm.Tri {
		if len(m) == 1 {
			for _, c := range m {
				if c == 1 {
					delete(lm.Tri, k)
				}
			}
		}
	}
}

func lmSuma(m map[string]int) int {
	s := 0
	for _, c := range m {
		s += c
	}
	return s
}

// p: la probabilidad de w después de «a b» (Kneser-Ney interpolado).
func (lm *tallerLM) p(a, b, w string) float64 {
	pc := (float64(lm.Cont[w]) + 0.1) / (float64(lm.biTipos) + 0.1*float64(len(lm.Uni)+1))
	pb := pc
	if m := lm.Bi[b]; len(m) > 0 {
		s := float64(lmSuma(m))
		pb = math.Max(float64(m[w])-lmDescuento, 0)/s + lmDescuento*float64(len(m))/s*pc
	}
	if t := lm.Tri[a+" "+b]; len(t) > 0 {
		s := float64(lmSuma(t))
		return math.Max(float64(t[w])-lmDescuento, 0)/s + lmDescuento*float64(len(t))/s*pb
	}
	return pb
}

// LogProb: lo bien que suena una frase (media por palabra; más alto es mejor).
func (lm *tallerLM) LogProb(frase []string) float64 {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	return lm.logProb(frase)
}

func (lm *tallerLM) logProb(frase []string) float64 {
	ws := append(append([]string{lmInicio, lmInicio}, frase...), lmFin)
	s := 0.0
	for i := 2; i < len(ws); i++ {
		s += math.Log(lm.p(ws[i-2], ws[i-1], ws[i]) + 1e-12)
	}
	return s / float64(len(ws)-2)
}

// muestrear: una frase, tirando hacia las palabras activadas.
func (lm *tallerLM) muestrear(rng *rand.Rand, inicio []string, act map[string]float64, max int, temp float64) []string {
	a, b := lmInicio, lmInicio
	var out []string
	for _, w := range inicio {
		out = append(out, w)
		a, b = b, w
	}
	usadas := map[string]int{}
	for len(out) < max {
		cand := map[string]bool{}
		for w := range lm.Tri[a+" "+b] {
			cand[w] = true
		}
		if len(cand) < 3 {
			for w := range lm.Bi[b] {
				cand[w] = true
			}
		}
		if len(cand) == 0 {
			break
		}
		ws := make([]string, 0, len(cand))
		for w := range cand {
			ws = append(ws, w)
		}
		sort.Strings(ws)
		pesos := make([]float64, len(ws))
		total := 0.0
		for i, w := range ws {
			p := lm.p(a, b, w)
			if w == lmFin && len(out) < 3 {
				p *= 0.05 // no acabar enseguida
			}
			if act != nil {
				p *= 1 + 3*act[lmBase(w)]
			}
			if usadas[w] > 0 && len([]rune(w)) > 3 {
				p *= 0.15 // no repetir lo mismo
			}
			pesos[i] = math.Pow(p, 1/temp)
			total += pesos[i]
		}
		x := rng.Float64() * total
		elegida := ws[len(ws)-1]
		for i, pw := range pesos {
			x -= pw
			if x <= 0 {
				elegida = ws[i]
				break
			}
		}
		if elegida == lmFin {
			break
		}
		out = append(out, elegida)
		usadas[elegida]++
		a, b = b, elegida
	}
	return out
}

// lmBase: la palabra sin tildes (como la usan Nexo y el constructor).
func lmBase(w string) string { return tallerSinTildes.Replace(w) }

// Decir: imagina varias frases y se queda con la que mejor dice lo que
// quiere decir (act) y mejor suena. Devuelve "" si aún no sabe hablar.
func (lm *tallerLM) Decir(rng *rand.Rand, act map[string]float64, tema string, intentos int) string {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.Tokens < 200 {
		return ""
	}
	maxAct := 1e-9
	for _, a := range act {
		maxAct = math.Max(maxAct, a)
	}
	norm := map[string]float64{}
	for w, a := range act {
		norm[w] = a / maxAct
	}
	// el tema tal como lo dice (con sus tildes), si sabe cómo seguirlo
	forma := ""
	if tema != "" {
		mejorC := 0
		for w, c := range lm.Uni {
			if lmBase(w) == tema && len(lm.Bi[w]) > 0 && c > mejorC {
				forma, mejorC = w, c
			}
		}
	}
	mejor, mejorV := "", math.Inf(-1)
	for i := 0; i < intentos; i++ {
		var inicio []string
		// la mitad de las veces empieza por el tema
		if forma != "" && i%2 == 0 {
			inicio = []string{forma}
		}
		f := lm.muestrear(rng, inicio, norm, 14, 0.85)
		if len(f) < 3 {
			continue
		}
		// lo que dice: cuánto de lo activado lleva (cada palabra una vez)
		visto := map[string]bool{}
		dice := 0.0
		for _, w := range f {
			b := lmBase(w)
			if !visto[b] {
				dice += norm[b]
				visto[b] = true
			}
			if b == tema {
				dice += 0.5
			}
		}
		v := dice + 0.6*lm.logProb(f)
		if v > mejorV {
			mejor, mejorV = strings.Join(f, " "), v
		}
	}
	return mejor
}

// Continuar: sigue un principio (para probarlo desde la consola).
func (lm *tallerLM) Continuar(rng *rand.Rand, principio string) string {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	var ini []string
	for _, f := range lmFrases(principio) {
		ini = append(ini, f...)
	}
	if len(ini) > 2 {
		ini = ini[len(ini)-2:]
	}
	return strings.Join(lm.muestrear(rng, ini, nil, 18, 0.8), " ")
}

// Vocabulario: las palabras que ya sabe decir (sin tildes), con cuántas veces.
func (lm *tallerLM) Vocabulario() map[string]int {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	out := make(map[string]int, len(lm.Uni))
	for w, c := range lm.Uni {
		if w != lmFin {
			out[lmBase(w)] += c
		}
	}
	return out
}

func (lm *tallerLM) Estado() string {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	var fs []string
	for q, n := range lm.Fuentes {
		fs = append(fs, fmt.Sprintf("%s %d", q, n))
	}
	sort.Strings(fs)
	return fmt.Sprintf("Lengua: %d palabras oídas, %d distintas, %d pares, %d tríos (de: %s)",
		lm.Tokens, len(lm.Uni), lm.biTipos, len(lm.Tri), strings.Join(fs, ", "))
}

// LeerArchivo: aprende de un texto tuyo (un libro, tus notas…), local.
func (lm *tallerLM) LeerArchivo(ruta string) (int, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<22)
	n := 0
	for sc.Scan() {
		n += lm.Aprender("textos", sc.Text())
	}
	return n, sc.Err()
}

// lmDeLaCharla: la primera vez, aprende de lo que ya se dijeron (charla.txt).
func (lm *tallerLM) deLaCharla(ruta string) {
	f, err := os.Open(ruta)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		l := sc.Text()
		i := strings.Index(l, " | ")
		if i < 0 {
			continue
		}
		l = l[i+3:]
		j := strings.Index(l, ": ")
		if j < 0 {
			continue
		}
		quien, texto := l[:j], l[j+2:]
		if k := strings.Index(texto, "  ⟶"); k >= 0 {
			texto = texto[:k] // lo que construyó no es lo que dijo
		}
		switch {
		case quien == "tú", quien == "Nyx":
			lm.Aprender(quien, texto)
		case strings.HasPrefix(quien, "Abla"):
			if g := tallerReGlosa.FindAllStringSubmatch(l, -1); len(g) > 0 {
				lm.Aprender("abla", g[len(g)-1][1])
			}
		}
	}
}
