package main

// ============================================================
//  TALLER · VOCABULARIO — las palabras que aprenden
// ------------------------------------------------------------
//  Nadie les da un diccionario. Una palabra suya (de Abla, de Nyx o
//  inventada) no hace nada más que su forma hasta que alguna le da
//  sentido:
//
//   · diciéndolo: «kevo significa subir», «zela aprende luz».
//   · Abla, con su propio diccionario: cuando uno de sus seres dice una
//     frase y su traducción palabra por palabra, cada palabra suya que
//     se traduce por un gesto (subir, luz, puerta, rojo, madera…) pasa
//     a ser ese gesto.
//
//  Desde entonces, al decir esa palabra hacen lo que significa. Lo que
//  aprenden se guarda en taller/palabras.json (y `palabras` lo enseña).
// ============================================================

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type tallerSignifica struct {
	Es     string    `json:"es"`    // lo que significa (una palabra que ya se entiende)
	Quien  string    `json:"quien"` // quién se lo dio
	Veces  int       `json:"veces"` // cuántas veces se ha dicho así
	Cuando time.Time `json:"cuando"`
}

var tallerVocab = struct {
	sync.Mutex
	ruta string
	sig  map[string]*tallerSignifica
}{sig: map[string]*tallerSignifica{}}

const tallerMaxVocabulario = 4000

// tallerCargarVocabulario: lo que ya aprendieron (al abrir la obra).
func tallerCargarVocabulario(dir string) {
	tallerVocab.Lock()
	defer tallerVocab.Unlock()
	tallerVocab.ruta = filepath.Join(dir, "palabras.json")
	tallerVocab.sig = map[string]*tallerSignifica{}
	if d, err := os.ReadFile(tallerVocab.ruta); err == nil {
		_ = json.Unmarshal(d, &tallerVocab.sig)
	}
	if tallerVocab.sig == nil {
		tallerVocab.sig = map[string]*tallerSignifica{}
	}
}

func tallerGuardarVocabulario() {
	if tallerVocab.ruta == "" {
		return
	}
	d, err := json.MarshalIndent(tallerVocab.sig, "", " ")
	if err != nil {
		return
	}
	tmp := tallerVocab.ruta + ".tmp"
	if os.WriteFile(tmp, d, 0o644) == nil {
		_ = os.Rename(tmp, tallerVocab.ruta)
	}
}

// tallerSignificado: lo que significa una palabra que aprendieron.
func tallerSignificado(w string) (string, bool) {
	tallerVocab.Lock()
	defer tallerVocab.Unlock()
	if s, ok := tallerVocab.sig[w]; ok {
		return s.Es, true
	}
	return "", false
}

// tallerEntendida: una palabra que ya hace algo (un gesto, un color, un
// material, un número, un sonido o una forma guardada).
func tallerEntendida(w string) bool {
	if w == "" {
		return false
	}
	if g, ok := tallerGestoDe[w]; ok && g != gAprender {
		return true
	}
	if _, ok := tallerColores[w]; ok {
		return true
	}
	if _, ok := tallerNumeros[w]; ok {
		return true
	}
	if _, err := strconv.Atoi(w); err == nil {
		return true
	}
	if _, ok := tallerAmbientes[w]; ok {
		return true
	}
	return tallerMateriales[w] || tallerFormaPropia(w) != nil
}

// tallerAprender: desde ahora, w significa es. Devuelve si es nuevo (o
// cambió de sentido).
func tallerAprender(w, es, quien string) bool {
	w, es = tallerSinTildes.Replace(strings.ToLower(w)), tallerSinTildes.Replace(strings.ToLower(es))
	if s, ok := tallerSignificado(es); ok {
		es = s // lo que ya significaba otra palabra suya
	}
	if w == es || len([]rune(w)) < 2 || tallerVacias[w] || tallerEntendida(w) || !tallerEntendida(es) {
		return false
	}
	tallerVocab.Lock()
	defer tallerVocab.Unlock()
	if s, ok := tallerVocab.sig[w]; ok {
		s.Veces++
		if s.Es == es {
			return false
		}
		s.Es, s.Quien, s.Cuando = es, quien, time.Now()
	} else {
		if len(tallerVocab.sig) >= tallerMaxVocabulario {
			return false
		}
		tallerVocab.sig[w] = &tallerSignifica{Es: es, Quien: quien, Veces: 1, Cuando: time.Now()}
	}
	tallerGuardarVocabulario()
	return true
}

// tallerOlvidarPalabra: que vuelva a ser solo su forma.
func tallerOlvidarPalabra(w string) bool {
	w = tallerSinTildes.Replace(strings.ToLower(w))
	tallerVocab.Lock()
	defer tallerVocab.Unlock()
	if _, ok := tallerVocab.sig[w]; !ok {
		return false
	}
	delete(tallerVocab.sig, w)
	tallerGuardarVocabulario()
	return true
}

// tallerAprenderDeGlosa: Abla dice una frase en su idioma con su
// traducción palabra por palabra; cada palabra suya que se traduce por
// algo que ya se entiende, lo aprende.
//
// alineada: si cada palabra suya tenía su traducción (entonces lo suyo ya
// dice lo mismo que la traducción y no hace falta construirla dos veces).
func tallerAprenderDeGlosa(dijo, glosa, quien string) (nuevas []string, alineada bool) {
	if glosa == "" {
		return nil, false
	}
	// «Nyx me dijo «mesu mika» («cielo luz»)»: lo suyo va justo antes de la
	// traducción; si no, es lo que hay antes de las comillas
	abla := dijo
	if gs := tallerReGlosa.FindAllStringSubmatch(dijo, -1); len(gs) >= 2 {
		abla = gs[len(gs)-2][1]
	} else if i := strings.Index(abla, "«"); i >= 0 {
		abla = abla[:i]
	}
	palabras := func(s string) []string {
		var out []string
		for _, w := range strings.Fields(strings.ToLower(s)) {
			if w = strings.Trim(w, ".,;:!?¡¿()\"'«»"); w != "" {
				out = append(out, w)
			}
		}
		return out
	}
	ws, gs := palabras(abla), palabras(glosa)
	if len(ws) == 0 || len(ws) != len(gs) {
		return nil, false
	}
	for i := range ws {
		w, g := ws[i], tallerSinTildes.Replace(gs[i])
		if tallerAprender(w, g, quien) {
			nuevas = append(nuevas, fmt.Sprintf("%s = %s", w, g))
		}
	}
	return nuevas, true
}

// tallerListaVocabulario: lo que han aprendido, para la consola.
func tallerListaVocabulario() string {
	tallerVocab.Lock()
	defer tallerVocab.Unlock()
	if len(tallerVocab.sig) == 0 {
		return "Todavía no le han dado sentido a ninguna palabra suya.\n" +
			"(Lo hacen diciendo «<palabra> significa <gesto>», o Abla con su traducción.)\n"
	}
	ws := make([]string, 0, len(tallerVocab.sig))
	for w := range tallerVocab.sig {
		ws = append(ws, w)
	}
	sort.Strings(ws)
	var b strings.Builder
	fmt.Fprintf(&b, "Palabras suyas con sentido (%d):\n", len(ws))
	for _, w := range ws {
		s := tallerVocab.sig[w]
		fmt.Fprintf(&b, "  %-14s = %-12s (%s, %d vez/veces)\n", w, s.Es, s.Quien, s.Veces)
	}
	return b.String()
}
