package memoria

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// MaxCasos is the number of cases kept per stored function (user cases and counterexamples first).
const MaxCasos = 40

// MaxAnteriores is the number of previous versions of the code kept after "está mal".
const MaxAnteriores = 5

// Puntuada is a search hit with its score (0..1).
type Puntuada struct {
	F      nucleo.FuncionAprendida
	Puntos float64
}

type biblioteca struct {
	mu    sync.RWMutex
	lista []nucleo.FuncionAprendida
}

type archivoBiblio struct {
	Version   int                       `json:"version"`
	Funciones []nucleo.FuncionAprendida `json:"funciones"`
}

func (a *Almacen) cargarBiblioteca() {
	var d archivoBiblio
	if a.leerJSON(archivoBiblioteca, &d, &d.Version) {
		for _, f := range d.Funciones {
			if strings.TrimSpace(f.Nombre) == "" {
				continue
			}
			a.fun.lista = append(a.fun.lista, f)
		}
	}
}

// guardarBiblioteca rewrites biblioteca.json. The caller holds the write lock.
func (a *Almacen) guardarBiblioteca() error {
	if err := a.abierto(); err != nil {
		return err
	}
	return escribirJSON(filepath.Join(a.dir, archivoBiblioteca), archivoBiblio{Version: Version, Funciones: a.fun.lista})
}

// rangoNivel orders evidence levels for functions: higher is better.
var rangoNivel = map[nucleo.Nivel]int{
	nucleo.NivelTusEjemplos:  7,
	nucleo.NivelEntendido:    6,
	nucleo.NivelPropiedades:  5,
	nucleo.NivelCalculo:      4,
	nucleo.NivelFuente:       3,
	nucleo.NivelSinFallos:    2,
	nucleo.NivelSinComprobar: 1,
	nucleo.NivelFallo:        0,
}

func mejorNivel(a, b nucleo.Nivel) bool { return rangoNivel[a] > rangoNivel[b] }

func copiarFuncion(f nucleo.FuncionAprendida) nucleo.FuncionAprendida {
	f.Conceptos = append([]string(nil), f.Conceptos...)
	f.Palabras = append([]string(nil), f.Palabras...)
	f.Casos = append([]nucleo.CasoGuardado(nil), f.Casos...)
	f.Anteriores = append([]string(nil), f.Anteriores...)
	return f
}

// Funciones returns every stored function, oldest first.
func (a *Almacen) Funciones() []nucleo.FuncionAprendida {
	a.fun.mu.RLock()
	defer a.fun.mu.RUnlock()
	out := make([]nucleo.FuncionAprendida, len(a.fun.lista))
	for i, f := range a.fun.lista {
		out[i] = copiarFuncion(f)
	}
	return out
}

// indice finds a function by name: exact first, then ignoring case and accents. The caller holds a lock.
func (a *Almacen) indice(nombre string) int {
	for i, f := range a.fun.lista {
		if f.Nombre == nombre {
			return i
		}
	}
	n := nucleo.Normalizar(nombre)
	for i, f := range a.fun.lista {
		if nucleo.Normalizar(f.Nombre) == n {
			return i
		}
	}
	return -1
}

// Funcion returns the function stored under nombre (exact, or ignoring case and accents).
func (a *Almacen) Funcion(nombre string) (nucleo.FuncionAprendida, bool) {
	a.fun.mu.RLock()
	defer a.fun.mu.RUnlock()
	if i := a.indice(nombre); i >= 0 {
		return copiarFuncion(a.fun.lista[i]), true
	}
	return nucleo.FuncionAprendida{}, false
}

// GuardarFuncion stores f and returns the name it is stored under. A function with the same Huella and the
// same Forma is the same behaviour: the cases, concepts and words are merged into it, the best Nivel is kept
// (taking the new code only when it reached a strictly better level), and its existing name is returned.
// A different function with the same name gets a suffix: SumaPares2, SumaPares3…
// The stored Nombre is the memory key; Firma and Codigo keep the function's own name.
func (a *Almacen) GuardarFuncion(f nucleo.FuncionAprendida) (string, error) {
	f = copiarFuncion(f)
	if strings.TrimSpace(f.Nombre) == "" {
		f.Nombre = f.Firma.Nombre
	}
	f.Nombre = strings.TrimSpace(f.Nombre)
	if f.Nombre == "" {
		return "", errors.New("memoria: la función no tiene nombre")
	}
	if strings.TrimSpace(f.Codigo) == "" {
		return "", errors.New("memoria: la función no tiene código")
	}
	ahora := a.ahora()
	if f.Creada.IsZero() {
		f.Creada = ahora
	}
	f.Casos = ordenarCasos(f.Casos)

	a.fun.mu.Lock()
	defer a.fun.mu.Unlock()
	if err := a.abierto(); err != nil {
		return "", err
	}
	if f.Huella != "" {
		forma := f.Firma.Forma()
		for i, g := range a.fun.lista {
			if g.Huella != f.Huella || g.Firma.Forma() != forma {
				continue
			}
			m := fusionar(g, f)
			a.fun.lista[i] = m
			if err := a.guardarBiblioteca(); err != nil {
				a.fun.lista[i] = g
				return "", fmt.Errorf("memoria: no pude guardar la biblioteca: %w", err)
			}
			return g.Nombre, nil
		}
	}
	base := f.Nombre
	for n := 2; a.indice(f.Nombre) >= 0; n++ {
		f.Nombre = base + strconv.Itoa(n)
	}
	a.fun.lista = append(a.fun.lista, f)
	if err := a.guardarBiblioteca(); err != nil {
		a.fun.lista = a.fun.lista[:len(a.fun.lista)-1]
		return "", fmt.Errorf("memoria: no pude guardar la biblioteca: %w", err)
	}
	return f.Nombre, nil
}

// fusionar merges the new record n into the stored g (same behaviour).
func fusionar(g, n nucleo.FuncionAprendida) nucleo.FuncionAprendida {
	m := copiarFuncion(g)
	if mejorNivel(n.Nivel, g.Nivel) {
		m.Nivel = n.Nivel
		m.Codigo, m.DSL, m.Firma, m.Origen, m.Fuente = n.Codigo, n.DSL, n.Firma, n.Origen, n.Fuente
		if n.Descripcion != "" {
			m.Descripcion = n.Descripcion
		}
	}
	if m.Descripcion == "" {
		m.Descripcion = n.Descripcion
	}
	if m.DSL == "" && n.DSL != "" && n.Codigo == m.Codigo {
		m.DSL = n.DSL
	}
	m.Conceptos = unirListas(m.Conceptos, n.Conceptos)
	m.Palabras = unirListas(m.Palabras, n.Palabras)
	m.Casos = ordenarCasos(append(append([]nucleo.CasoGuardado(nil), n.Casos...), m.Casos...))
	m.Usos += n.Usos
	if n.Usada.After(m.Usada) {
		m.Usada = n.Usada
	}
	return m
}

func unirListas(a, b []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, x := range append(append([]string(nil), a...), b...) {
		if x == "" || visto[x] {
			continue
		}
		visto[x] = true
		out = append(out, x)
	}
	return out
}

// claveCaso identifies a stored case by its inputs.
func claveCaso(c nucleo.CasoGuardado) string {
	var sb strings.Builder
	for _, e := range c.Entradas {
		sb.Write(e)
		sb.WriteByte(0x1f)
	}
	return sb.String()
}

// prioridadCaso puts the user's cases and counterexamples first.
func prioridadCaso(c nucleo.CasoGuardado) int {
	switch {
	case c.Expectativa == nucleo.EspUsuario || c.Origen == nucleo.OrigenUsuario:
		if c.Origen == nucleo.OrigenContraejemplo {
			return 0
		}
		return 1
	case c.Origen == nucleo.OrigenContraejemplo:
		return 0
	case c.Expectativa == nucleo.EspReferencia:
		return 2
	case c.Expectativa == nucleo.EspInterpretacion:
		return 3
	}
	return 4
}

// ordenarCasos removes repeated inputs (the first one wins), sorts by priority (stable) and keeps MaxCasos.
func ordenarCasos(cs []nucleo.CasoGuardado) []nucleo.CasoGuardado {
	visto := map[string]bool{}
	out := make([]nucleo.CasoGuardado, 0, len(cs))
	for _, c := range cs {
		k := claveCaso(c)
		if visto[k] {
			continue
		}
		visto[k] = true
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return prioridadCaso(out[i]) < prioridadCaso(out[j]) })
	if len(out) > MaxCasos {
		out = out[:MaxCasos]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ReemplazarFuncion puts new code under an existing name (after "está mal"): the old Codigo goes to
// Anteriores (at most 5, oldest dropped), and Creada and Usos are kept.
func (a *Almacen) ReemplazarFuncion(nombre string, f nucleo.FuncionAprendida) error {
	if strings.TrimSpace(f.Codigo) == "" {
		return errors.New("memoria: la función no tiene código")
	}
	a.fun.mu.Lock()
	defer a.fun.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	i := a.indice(nombre)
	if i < 0 {
		return fmt.Errorf("memoria: no conozco ninguna función llamada %s", nombre)
	}
	viejo := a.fun.lista[i]
	n := copiarFuncion(f)
	n.Nombre = viejo.Nombre
	n.Creada = viejo.Creada
	n.Usos = viejo.Usos + f.Usos
	if n.Usada.IsZero() {
		n.Usada = viejo.Usada
	}
	n.Casos = ordenarCasos(n.Casos)
	anteriores := append([]string(nil), viejo.Anteriores...)
	if viejo.Codigo != n.Codigo {
		anteriores = append(anteriores, viejo.Codigo)
	}
	if len(anteriores) > MaxAnteriores {
		anteriores = anteriores[len(anteriores)-MaxAnteriores:]
	}
	n.Anteriores = anteriores
	a.fun.lista[i] = n
	if err := a.guardarBiblioteca(); err != nil {
		a.fun.lista[i] = viejo
		return fmt.Errorf("memoria: no pude guardar la biblioteca: %w", err)
	}
	return nil
}

// OlvidarFuncion removes a stored function ("olvida SumaPares").
func (a *Almacen) OlvidarFuncion(nombre string) error {
	a.fun.mu.Lock()
	defer a.fun.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	i := a.indice(nombre)
	if i < 0 {
		return fmt.Errorf("memoria: no conozco ninguna función llamada %s", nombre)
	}
	antes := a.fun.lista
	a.fun.lista = append(append([]nucleo.FuncionAprendida(nil), antes[:i]...), antes[i+1:]...)
	if err := a.guardarBiblioteca(); err != nil {
		a.fun.lista = antes
		return fmt.Errorf("memoria: no pude guardar la biblioteca: %w", err)
	}
	return nil
}

// BuscarFunciones ranks stored functions by 0.6·Jaccard(conceptos) + 0.4·TF-IDF(palabras) (see docFuncion
// for a function's words). forma "" accepts any signature; otherwise only functions
// whose Firma.Forma() equals it. It returns at most k hits (k ≤ 0: all) with a positive score, best first.
func (a *Almacen) BuscarFunciones(conceptos, palabras []string, forma string, k int) []Puntuada {
	a.fun.mu.RLock()
	defer a.fun.mu.RUnlock()
	var cand []nucleo.FuncionAprendida
	for _, f := range a.fun.lista {
		if forma == "" || f.Firma.Forma() == forma {
			cand = append(cand, f)
		}
	}
	if len(cand) == 0 {
		return nil
	}
	docs := make([][]string, len(a.fun.lista))
	idx := map[string]int{}
	for i, f := range a.fun.lista {
		docs[i] = docFuncion(f)
		idx[f.Nombre] = i
	}
	var consulta []string
	for _, p := range palabras {
		consulta = append(consulta, tokens(p)...)
	}
	sim := tfidf(consulta, docs) // idf over the whole library
	var out []Puntuada
	for _, f := range cand {
		p := 0.6*jaccard(conceptos, f.Conceptos) + 0.4*sim[idx[f.Nombre]]
		if p > 0 {
			out = append(out, Puntuada{F: copiarFuncion(f), Puntos: p})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Puntos != out[j].Puntos {
			return out[i].Puntos > out[j].Puntos
		}
		if out[i].F.Usos != out[j].F.Usos {
			return out[i].F.Usos > out[j].F.Usos
		}
		return out[i].F.Nombre < out[j].F.Nombre
	})
	if k > 0 && len(out) > k {
		out = out[:k]
	}
	return out
}

// docFuncion is the bag of words of a function for TF-IDF: its Palabras (request lemmas), or, when it has
// none (a recipe or a web snippet), the words of its name and description.
func docFuncion(f nucleo.FuncionAprendida) []string {
	var d []string
	for _, p := range f.Palabras {
		d = append(d, tokens(p)...)
	}
	if len(d) == 0 {
		d = append(tokensNombre(f.Nombre), tokens(f.Descripcion)...)
	}
	return d
}

// PorHuella returns the functions with fingerprint h (same behaviour on the probes).
func (a *Almacen) PorHuella(h string) []nucleo.FuncionAprendida {
	if h == "" {
		return nil
	}
	a.fun.mu.RLock()
	defer a.fun.mu.RUnlock()
	var out []nucleo.FuncionAprendida
	for _, f := range a.fun.lista {
		if f.Huella == h {
			out = append(out, copiarFuncion(f))
		}
	}
	return out
}

// Usada records one more use of a stored function (Usos+1, Usada = now). Unknown names are ignored.
func (a *Almacen) Usada(nombre string) {
	a.fun.mu.Lock()
	defer a.fun.mu.Unlock()
	i := a.indice(nombre)
	if i < 0 {
		return
	}
	a.fun.lista[i].Usos++
	a.fun.lista[i].Usada = a.ahora()
	if err := a.guardarBiblioteca(); err != nil && !errors.Is(err, ErrCerrado) {
		a.avisar(fmt.Sprintf("No pude guardar el uso de %s: %v.", nombre, err))
	}
}
