package main

// ============================================================
//  TALLER · MODELO DEL MUNDO 3D — lo que saben las IAs del espacio
// ------------------------------------------------------------
//  El modelo de lenguaje decide qué decir; este modelo sabe DÓNDE están
//  las cosas y QUÉ PASARÁ si dicen algo. Lo mejor de cada una:
//
//   · De NYX MUNDO: recordar los sitios (un mapa en 3D, por celdas de
//     4 m, de lo que hay, hasta dónde llega y qué palabras lo hicieron) y
//     su autoentreno: compara lo que imaginó con lo que de verdad salió
//     y corrige sus cálculos (se calibra solo).
//   · Del TALLER: imaginar. Antes de construir, prueba la frase en la
//     cabeza (el mismo constructor, sin tocar el mundo) y ve qué
//     saldría y dónde. Así las IAs eligen entre varias frases la que
//     mejor sirve para lo que quieren hacer.
//   · De ABLA: lo que se aprende con la práctica. Las habilidades suben
//     con cada intento (más si sale bien), como en sus misiones.
//   · Del CEREBRO WEB: saber dónde hay novedad. Lo que nadie ha visitado
//     llama la atención de las exploradoras.
//
//  Se guarda en taller/mundo3d.json (las visitas y la calibración; lo que
//  hay se vuelve a ver de las piezas al abrir).
// ============================================================

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const tallerCelda3D = 4.0

type tallerModelo3D struct {
	mu   sync.Mutex
	ruta string

	Visitas map[string]int `json:"visitas"`
	KAlto   float64        `json:"k_alto"`  // lo real / lo imaginado, de alto
	KAncho  float64        `json:"k_ancho"` // … y de ancho
	Error   float64        `json:"error"`   // cuánto se equivoca al imaginar (0 = nada)
	Pruebas int            `json:"pruebas"`

	ocupa    map[string]float64  // celda → cuánto está llena (0..1)
	techo    map[string]float64  // celda → hasta dónde llega lo que hay
	palabras map[string][]string // celda → palabras de lo que hay
	firma    string              // qué piezas había la última vez
}

func tallerNuevoModelo3D(dir string) *tallerModelo3D {
	m := &tallerModelo3D{ruta: filepath.Join(dir, "mundo3d.json")}
	if d, err := os.ReadFile(m.ruta); err == nil {
		_ = json.Unmarshal(d, m)
	}
	if m.Visitas == nil {
		m.Visitas = map[string]int{}
	}
	if m.KAlto <= 0 {
		m.KAlto = 1
	}
	if m.KAncho <= 0 {
		m.KAncho = 1
	}
	m.ocupa, m.techo, m.palabras = map[string]float64{}, map[string]float64{}, map[string][]string{}
	return m
}

func (m *tallerModelo3D) Guardar() error {
	m.mu.Lock()
	d, err := json.Marshal(m)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := m.ruta + ".tmp"
	if err := os.WriteFile(tmp, d, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.ruta)
}

func celda3D(x, y, z float64) [3]int {
	return [3]int{int(math.Floor(x / tallerCelda3D)), int(math.Floor(y / tallerCelda3D)), int(math.Floor(z / tallerCelda3D))}
}

func clave3D(c [3]int) string {
	return strconv.Itoa(c[0]) + "," + strconv.Itoa(c[1]) + "," + strconv.Itoa(c[2])
}

func centro3D(c [3]int) [3]float64 {
	return [3]float64{(float64(c[0]) + 0.5) * tallerCelda3D, float64(c[1]) * tallerCelda3D, (float64(c[2]) + 0.5) * tallerCelda3D}
}

// Sincronizar: vuelve a ver lo que hay (si las piezas cambiaron).
func (m *tallerModelo3D) Sincronizar(piezas []tallerFoto) {
	var b strings.Builder
	for _, p := range piezas {
		fmt.Fprintf(&b, "%s@%.1f,%.1f,%.1f;", p.ID, p.X, p.Y, p.Z)
	}
	firma := b.String()
	m.mu.Lock()
	defer m.mu.Unlock()
	if firma == m.firma {
		return
	}
	m.firma = firma
	m.ocupa, m.techo, m.palabras = map[string]float64{}, map[string]float64{}, map[string][]string{}
	for _, p := range piezas {
		r := math.Max(0.5, math.Min(p.Radio, 30))
		y0, y1 := p.Y-1, p.Y+math.Max(0.3, p.Alto)
		ws := nexoPalabras(p.Titulo)
		n := 0
		for i := int(math.Floor((p.X - r) / tallerCelda3D)); i <= int(math.Floor((p.X+r)/tallerCelda3D)); i++ {
			for k := int(math.Floor((p.Z - r) / tallerCelda3D)); k <= int(math.Floor((p.Z+r)/tallerCelda3D)); k++ {
				for j := int(math.Floor(y0 / tallerCelda3D)); j <= int(math.Floor(y1/tallerCelda3D)); j++ {
					if n++; n > 3000 {
						break
					}
					c := clave3D([3]int{i, j, k})
					m.ocupa[c] = math.Min(1, m.ocupa[c]+0.5)
					m.techo[c] = math.Max(m.techo[c], y1)
					if pw := m.palabras[c]; len(pw) < 6 {
						for _, w := range ws {
							if len(m.palabras[c]) < 6 {
								m.palabras[c] = append(m.palabras[c], w)
							}
						}
					}
				}
			}
		}
	}
}

// Visitar: alguien pasó por aquí (lo nuevo deja de serlo).
func (m *tallerModelo3D) Visitar(x, y, z float64) {
	m.mu.Lock()
	m.Visitas[clave3D(celda3D(x, y, z))]++
	m.mu.Unlock()
}

// Lleno: si en esa celda hay algo.
func (m *tallerModelo3D) Lleno(c [3]int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ocupa[clave3D(c)] > 0
}

// Solape: qué parte de una caja ya está ocupada (0 = está libre).
func (m *tallerModelo3D) Solape(c [6]float64) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, b := celda3D(c[0], c[1], c[2]), celda3D(c[3], c[4], c[5])
	tot, llenas := 0, 0
	for i := a[0]; i <= b[0]; i++ {
		for j := a[1]; j <= b[1]; j++ {
			for k := a[2]; k <= b[2]; k++ {
				if tot++; tot > 4000 {
					return float64(llenas) / float64(tot)
				}
				if m.ocupa[clave3D([3]int{i, j, k})] > 0 {
					llenas++
				}
			}
		}
	}
	if tot == 0 {
		return 0
	}
	return float64(llenas) / float64(tot)
}

// PalabrasCerca: de qué está hecho lo que hay alrededor (de eso hablan).
func (m *tallerModelo3D) PalabrasCerca(x, y, z, radio float64) map[string]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]float64{}
	c := celda3D(x, y, z)
	r := int(math.Ceil(radio / tallerCelda3D))
	for i := -r; i <= r; i++ {
		for j := -1; j <= 1; j++ {
			for k := -r; k <= r; k++ {
				for _, w := range m.palabras[clave3D([3]int{c[0] + i, c[1] + j, c[2] + k})] {
					out[w] += 1 / (1 + math.Abs(float64(i)) + math.Abs(float64(k)))
				}
			}
		}
	}
	return out
}

// Frontera: un sitio libre pegado a lo que ya hay (donde seguir
// construyendo), cerca de (x, y, z). sube: hacia dónde prefiere crecer
// (+1 arriba, -1 abajo, 0 igual).
func (m *tallerModelo3D) Frontera(rng *rand.Rand, x, y, z, radio float64, sube int) ([3]float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	llenas := make([]string, 0, 64)
	for k := range m.ocupa {
		llenas = append(llenas, k)
	}
	sort.Strings(llenas)
	if len(llenas) == 0 {
		return [3]float64{}, false
	}
	vecinas := [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}, {0, 1, 0}, {0, -1, 0}}
	mejor, mejorV, hay := [3]float64{}, math.Inf(-1), false
	for i := 0; i < 60; i++ {
		var c [3]int
		fmt.Sscanf(llenas[rng.Intn(len(llenas))], "%d,%d,%d", &c[0], &c[1], &c[2])
		v := vecinas[rng.Intn(len(vecinas))]
		n := [3]int{c[0] + v[0], c[1] + v[1], c[2] + v[2]}
		if m.ocupa[clave3D(n)] > 0 {
			continue
		}
		p := centro3D(n)
		d := math.Sqrt((p[0]-x)*(p[0]-x) + (p[1]-y)*(p[1]-y) + (p[2]-z)*(p[2]-z))
		if d > radio {
			continue
		}
		val := -d/radio + float64(sube*v[1])*1.5 - float64(m.Visitas[clave3D(n)])*0.02 + rng.Float64()*0.3
		if val > mejorV {
			mejor, mejorV, hay = p, val, true
		}
	}
	return mejor, hay
}

// Novedad: un sitio que nadie ha visitado (las exploradoras van allí; a
// veces, lejos, al vacío, para que el mundo no se acabe nunca).
func (m *tallerModelo3D) Novedad(rng *rand.Rand, x, y, z, radio float64) [3]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	mejor, mejorV := [3]float64{x, y, z}, math.Inf(-1)
	for i := 0; i < 40; i++ {
		ang := rng.Float64() * 2 * math.Pi
		d := radio * (0.3 + 0.7*rng.Float64())
		p := [3]float64{x + math.Cos(ang)*d, y + (rng.Float64()-0.5)*radio*0.5, z + math.Sin(ang)*d}
		c := clave3D(celda3D(p[0], p[1], p[2]))
		val := -float64(m.Visitas[c]) + rng.Float64()
		if m.ocupa[c] > 0 {
			val += 0.5 // lo que hay y nadie ha visto, interesa más
		}
		if val > mejorV {
			mejor, mejorV = p, val
		}
	}
	return mejor
}

// Imaginar: qué saldría si dijera esa frase desde donde está su
// constructor (sin tocar nada del mundo), con la calibración aprendida.
func (m *tallerModelo3D) Imaginar(t tallerTortuga, frase string) *tallerObraHecha {
	c := t
	c.Pila = append([][8]float64(nil), t.Pila...)
	c.Mano = ""
	c.imaginando = true
	f := tallerHablarYConstruir(&c, tallerPalabras(frase), frase, nil)
	if f.HayCaja {
		m.mu.Lock()
		ka, kw := m.KAlto, m.KAncho
		m.mu.Unlock()
		cx, cz := (f.Caja[0]+f.Caja[3])/2, (f.Caja[2]+f.Caja[5])/2
		f.Caja[4] = f.Caja[1] + (f.Caja[4]-f.Caja[1])*ka
		f.Caja[0], f.Caja[3] = cx-(cx-f.Caja[0])*kw, cx+(f.Caja[3]-cx)*kw
		f.Caja[2], f.Caja[5] = cz-(cz-f.Caja[2])*kw, cz+(f.Caja[5]-cz)*kw
	}
	return f
}

// Comparar: lo que imaginó contra lo que salió de verdad. Corrige su
// calibración (como el autoentreno de Nyx) y devuelve el error (0..1).
func (m *tallerModelo3D) Comparar(imaginado *tallerObraHecha, real *tallerPieza, o *tallerObra) float64 {
	if imaginado == nil || !imaginado.HayCaja || real == nil {
		return 1
	}
	o.mu.Lock()
	altoReal, radioReal := real.Alto, real.Radio
	o.mu.Unlock()
	altoI := math.Max(0.2, imaginado.Caja[4]-imaginado.y)
	anchoI := math.Max(0.2, math.Max(imaginado.Caja[3]-imaginado.Caja[0], imaginado.Caja[5]-imaginado.Caja[2]))
	altoR, anchoR := math.Max(0.2, altoReal), math.Max(0.2, radioReal)
	err := math.Min(1, math.Abs(altoR-altoI)/math.Max(altoR, altoI)*0.6+math.Abs(anchoR-anchoI)/math.Max(anchoR, anchoI)*0.4)
	m.mu.Lock()
	defer m.mu.Unlock()
	// lo que se ve en la cabeza ya lleva la calibración: se corrige poco a poco
	m.KAlto = math.Max(0.3, math.Min(3, m.KAlto*(1+0.15*(altoR/altoI-1))))
	m.KAncho = math.Max(0.3, math.Min(3, m.KAncho*(1+0.15*(anchoR/anchoI-1))))
	m.Pruebas++
	m.Error = m.Error*0.9 + err*0.1
	if m.Pruebas == 1 {
		m.Error = err
	}
	return err
}

func (m *tallerModelo3D) Estado() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fmt.Sprintf("Modelo del mundo: %d celdas llenas, %d visitadas · al imaginar se equivoca un %.0f%% (%d pruebas; calibración alto ×%.2f, ancho ×%.2f)",
		len(m.ocupa), len(m.Visitas), m.Error*100, m.Pruebas, m.KAlto, m.KAncho)
}
