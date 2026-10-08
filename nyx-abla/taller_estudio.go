package main

// ============================================================
//  TALLER · ESTUDIO — les das una lista de vídeos, los ven y
//  perfeccionan lo que han construido
// ------------------------------------------------------------
//    estudia <vídeo> <vídeo> …      (o  estudia lista.txt, uno por línea)
//    estudio                        cómo va
//    estudio para                   que paren al acabar el vídeo de ahora
//
//  Con cada vídeo:
//   1. lo ven las dos (Nyx lo recuerda como sitio; Abla mira sus
//      fotogramas, y se espera a que termine);
//   2. hablan de lo que vieron, y lo que dicen se construye;
//   3. PERFECCIONAN: cada una repasa sus piezas y las compara con el vídeo
//      (sus colores, si es más alto o más ancho, cuánta luz hay). Prueba
//      cambios con sus manos (teñir con los colores del vídeo, ponerle de
//      textura un fotograma, un material parecido, estirarlo, darle luz)
//      y solo se queda con los que lo hacen PARECERSE MÁS: si un cambio no
//      mejora el parecido, lo deshace. En la charla se ve el parecido
//      antes y después.
//  La lista se guarda: si cierras el taller, al volver siguen por donde
//  iban.
// ============================================================

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type tallerEstudio struct {
	Pendientes []string `json:"pendientes,omitempty"`
	Hechos     []string `json:"hechos,omitempty"` // lo que salió de cada vídeo
	Actual     string   `json:"actual,omitempty"`
	parar      bool
	corriendo  bool
}

// tallerObjetivo: lo que vieron en un vídeo, para compararse con ello.
type tallerObjetivo struct {
	titulo   string
	paleta   [][3]float64
	pesos    []float64
	vertical float64 // 0 ancho … 1 alto
	brillo   float64
	fotos    []string // fotogramas (para usarlos de textura: «foto:<nombre>»)
	palabras []string // lo que les evocó
}

// ---------- la lista ----------

// Estudiar: añade vídeos a la lista (o un .txt con uno por línea) y, si no
// estaban ya, se ponen a estudiar.
func (o *tallerObra) Estudiar(entrada []string, avisar func(string)) int {
	var nuevos []string
	for _, e := range entrada {
		if strings.HasPrefix(e, "~/") {
			if h, err := os.UserHomeDir(); err == nil {
				e = filepath.Join(h, e[2:])
			}
		}
		if strings.HasSuffix(strings.ToLower(e), ".txt") {
			if f, err := os.Open(e); err == nil {
				sc := bufio.NewScanner(f)
				for sc.Scan() {
					if l := strings.TrimSpace(sc.Text()); l != "" && !strings.HasPrefix(l, "#") {
						nuevos = append(nuevos, strings.Fields(l)[0])
					}
				}
				f.Close()
			} else {
				avisar("  no puedo leer " + e + ": " + err.Error())
			}
			continue
		}
		nuevos = append(nuevos, e)
	}
	o.mu.Lock()
	o.Estudio.Pendientes = append(o.Estudio.Pendientes, nuevos...)
	o.Estudio.parar = false
	ya := o.Estudio.corriendo
	o.Estudio.corriendo = true
	o.mu.Unlock()
	_ = o.Guardar()
	if !ya {
		go o.estudiar(avisar)
	}
	return len(nuevos)
}

// SeguirEstudiando: al abrir el taller, si quedaba lista, sigue.
func (o *tallerObra) SeguirEstudiando(avisar func(string)) {
	o.mu.Lock()
	hay := len(o.Estudio.Pendientes) > 0 && !o.Estudio.corriendo
	if hay {
		o.Estudio.corriendo = true
	}
	o.mu.Unlock()
	if hay {
		avisar("taller: quedaban vídeos por estudiar; siguen con ellos ('estudio' para ver cómo va)")
		go o.estudiar(avisar)
	}
}

func (o *tallerObra) estudiar(avisar func(string)) {
	defer func() {
		o.mu.Lock()
		o.Estudio.corriendo, o.Estudio.Actual = false, ""
		o.mu.Unlock()
		_ = o.Guardar()
	}()
	for {
		o.mu.Lock()
		if o.Estudio.parar || len(o.Estudio.Pendientes) == 0 {
			o.mu.Unlock()
			if len(o.Estudio.Pendientes) == 0 {
				o.Decir("taller", "Han terminado de estudiar la lista.", "")
			}
			return
		}
		v := o.Estudio.Pendientes[0]
		o.Estudio.Actual = v
		o.mu.Unlock()
		res := o.estudiarUno(v, avisar)
		o.mu.Lock()
		if len(o.Estudio.Pendientes) > 0 && o.Estudio.Pendientes[0] == v {
			o.Estudio.Pendientes = o.Estudio.Pendientes[1:]
		}
		o.Estudio.Hechos = append(o.Estudio.Hechos, res)
		if len(o.Estudio.Hechos) > 200 {
			o.Estudio.Hechos = o.Estudio.Hechos[len(o.Estudio.Hechos)-200:]
		}
		o.mu.Unlock()
		_ = o.Guardar()
	}
}

// estudiarUno: ver, hablar y perfeccionar con un vídeo.
func (o *tallerObra) estudiarUno(fuente string, avisar func(string)) string {
	o.m.mu.Lock()
	antes := len(o.m.Recuerdos)
	o.m.mu.Unlock()
	if err := o.Ver(fuente, avisar); err != nil {
		o.Decir("taller", "no pudieron ver "+fuente+": "+err.Error(), "")
		return fuente + ": no se pudo ver (" + err.Error() + ")"
	}
	o.mu.Lock()
	titulo := o.viendo
	o.mu.Unlock()
	// Abla lo mira poco a poco: se espera a que acabe (como mucho 20 minutos)
	if o.abla.Viva() {
		limite := time.Now().Add(20 * time.Minute)
		for o.abla.EnCola() > 0 && time.Now().Before(limite) {
			o.recogerVistas()
			time.Sleep(3 * time.Second)
		}
		time.Sleep(2 * time.Second)
		o.recogerVistas()
	}
	obj := o.objetivoDe(titulo, antes)
	if len(obj.paleta) == 0 {
		return titulo + ": no sacaron nada de él"
	}
	// hablan de lo que vieron (y se construye)
	o.mu.Lock()
	o.estudiando = true
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.estudiando = false
		o.mu.Unlock()
	}()
	rng := newRand()
	visto := strings.Join(obj.palabras[:min(8, len(obj.palabras))], " ")
	o.TurnoNyx(rng, "he visto «"+titulo+"»: "+visto)
	if o.abla.Viva() {
		o.TurnoAbla(rng, visto)
	}
	// y perfeccionan
	a1, d1, n1 := o.perfeccionar("nyx", "Nyx", obj)
	a2, d2, n2 := 0.0, 0.0, 0
	if o.abla.Viva() {
		a2, d2, n2 = o.perfeccionar("abla", "Abla", obj)
	}
	n := n1 + n2
	if n == 0 {
		return titulo + ": vieron, hablaron, y no tenían nada que perfeccionar todavía"
	}
	antesM, despuesM := (a1*float64(n1)+a2*float64(n2))/float64(n), (d1*float64(n1)+d2*float64(n2))/float64(n)
	r := fmt.Sprintf("%s: perfeccionaron %d pieza(s), parecido medio %.0f%% → %.0f%%", titulo, n, antesM*100, despuesM*100)
	o.Decir("taller", r, "")
	return r
}

// objetivoDe: lo que sacaron de ese vídeo (lo de Abla y lo de Nyx juntos).
func (o *tallerObra) objetivoDe(titulo string, recuerdosAntes int) *tallerObjetivo {
	obj := &tallerObjetivo{titulo: titulo}
	peso := map[[3]float64]float64{}
	nv := 0.0
	o.mu.Lock()
	for _, v := range o.Vistas {
		if v.Video != titulo {
			continue
		}
		nv++
		for _, p := range v.Paleta {
			c := [3]float64{math.Round(float64(p.RGB[0])/255*10) / 10, math.Round(float64(p.RGB[1])/255*10) / 10, math.Round(float64(p.RGB[2])/255*10) / 10}
			peso[c] += p.Frac
		}
		if s := v.BordesH + v.BordesV; s > 0 {
			obj.vertical += v.BordesV / s
		} else {
			obj.vertical += 0.5
		}
		obj.brillo += v.Brillo
		if len(obj.fotos) < 6 {
			obj.fotos = append(obj.fotos, v.Foto)
		}
		for _, c := range v.Conceptos {
			obj.palabras = append(obj.palabras, c.Es)
		}
		obj.palabras = append(obj.palabras, strings.Fields(v.Abla)...)
	}
	o.mu.Unlock()
	if nv > 0 {
		obj.vertical /= nv
		obj.brillo /= nv
	}
	// lo que recuerda Nyx del mismo vídeo (sus sitios nuevos)
	o.m.mu.Lock()
	var nuevos []*Recuerdo
	if recuerdosAntes < len(o.m.Recuerdos) {
		nuevos = append(nuevos, o.m.Recuerdos[recuerdosAntes:]...)
	}
	o.m.mu.Unlock()
	for _, r := range nuevos {
		for _, c := range [][3]float64{r.Rasgos.Pared, r.Rasgos.Suelo, r.Rasgos.Techo} {
			k := [3]float64{math.Round(c[0]*10) / 10, math.Round(c[1]*10) / 10, math.Round(c[2]*10) / 10}
			peso[k] += 0.3
		}
		if nv == 0 {
			obj.vertical = 0.3 + 0.5*clamp01(r.Rasgos.Altura)
			obj.brillo = 1 - clamp01(r.Rasgos.Oscuridad)
		}
		obj.palabras = append(obj.palabras, describirRasgosPalabras(r.Rasgos)...)
	}
	type cp struct {
		c [3]float64
		p float64
	}
	var cs []cp
	for c, p := range peso {
		cs = append(cs, cp{c, p})
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].p > cs[j].p })
	for i, x := range cs {
		if i >= 5 {
			break
		}
		obj.paleta = append(obj.paleta, x.c)
		obj.pesos = append(obj.pesos, x.p)
	}
	if nv == 0 && len(nuevos) == 0 {
		obj.paleta = nil
	}
	return obj
}

// ---------- medir el parecido ----------

type tallerMedida struct {
	colores [][3]float64 // un color por triángulo (con su textura)
	brillan float64      // qué parte da luz
	alto    float64
	ancho   float64
}

var (
	tallerColTexMu sync.Mutex
	tallerColTex   = map[string][3]float64{}
)

// tallerColorDeTextura: el color medio de una textura.
func tallerColorDeTextura(o *tallerObra, nombre string) [3]float64 {
	tallerColTexMu.Lock()
	c, ok := tallerColTex[nombre]
	tallerColTexMu.Unlock()
	if ok {
		return c
	}
	c = [3]float64{0.6, 0.6, 0.6}
	if d, err := tallerTexturaPNG(o, nombre); err == nil {
		if im, _, err := image.Decode(bytes.NewReader(d)); err == nil {
			r := im.Bounds()
			var s [3]float64
			n := 0.0
			for y := r.Min.Y; y < r.Max.Y; y += 4 {
				for x := r.Min.X; x < r.Max.X; x += 4 {
					R, G, B, _ := im.At(x, y).RGBA()
					s[0], s[1], s[2] = s[0]+float64(R)/65535, s[1]+float64(G)/65535, s[2]+float64(B)/65535
					n++
				}
			}
			if n > 0 {
				c = [3]float64{s[0] / n, s[1] / n, s[2] / n}
			}
		}
	}
	tallerColTexMu.Lock()
	tallerColTex[nombre] = c
	tallerColTexMu.Unlock()
	return c
}

func (o *tallerObra) medir(p *tallerPieza) (*tallerMedida, bool) {
	d, err := os.ReadFile(o.m.dir + "/objetos/" + p.ID + ".go")
	if err != nil {
		return nil, false
	}
	comp, err := tallerCompilar(string(d))
	if err != nil {
		return nil, false
	}
	c := comp.entero
	m := &tallerMedida{}
	nt := len(c.Pos) / 9
	if nt == 0 {
		return nil, false
	}
	mn := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	mx := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for i := 0; i+2 < len(c.Pos); i += 3 {
		for j := 0; j < 3; j++ {
			mn[j], mx[j] = math.Min(mn[j], float64(c.Pos[i+j])), math.Max(mx[j], float64(c.Pos[i+j]))
		}
	}
	m.alto, m.ancho = mx[1]-mn[1], math.Max(mx[0]-mn[0], mx[2]-mn[2])
	paso := max(1, nt/400) // como mucho 400 triángulos de muestra
	brillan, cuantos := 0.0, 0.0
	for t := 0; t < nt; t += paso {
		cuantos++
		if c.Emi[t*3] >= 3.5 {
			brillan++
		}
		col := [3]float64{float64(c.Col[t*9]), float64(c.Col[t*9+1]), float64(c.Col[t*9+2])}
		if t < len(comp.matTri) && comp.matTri[t] != "" {
			col = tallerColorDeTextura(o, comp.matTri[t])
		}
		m.colores = append(m.colores, col)
	}
	m.brillan = brillan / cuantos
	return m, true
}

// parecido: cuánto se parece una pieza a lo que vieron (de 0 a 1).
func (obj *tallerObjetivo) parecido(m *tallerMedida) float64 {
	if m == nil || len(obj.paleta) == 0 {
		return 0
	}
	dist := 0.0
	for _, c := range m.colores {
		mejor := math.Inf(1)
		for _, p := range obj.paleta {
			mejor = math.Min(mejor, math.Sqrt((c[0]-p[0])*(c[0]-p[0])+(c[1]-p[1])*(c[1]-p[1])+(c[2]-p[2])*(c[2]-p[2])))
		}
		dist += mejor
	}
	color := 1 - math.Min(1, dist/float64(len(m.colores))/0.6)
	vert := m.alto / math.Max(1e-6, m.alto+m.ancho)
	forma := 1 - math.Min(1, math.Abs(vert-obj.vertical)*1.6)
	luzQuiere := clamp01((obj.brillo - 0.45) * 0.6)
	luz := 1 - math.Min(1, math.Abs(m.brillan-luzQuiere)*3)
	return 0.6*color + 0.3*forma + 0.1*luz
}

// ---------- perfeccionar ----------

// perfeccionar: una repasa sus piezas (las más recientes) y se queda con
// los cambios que las hacen parecerse más al vídeo.
func (o *tallerObra) perfeccionar(autor, quien string, obj *tallerObjetivo) (antesMedio, despuesMedio float64, n int) {
	o.mu.Lock()
	var suyas []*tallerPieza
	for i := len(o.Piezas) - 1; i >= 0 && len(suyas) < 6; i-- {
		if o.Piezas[i].Autor == autor {
			suyas = append(suyas, o.Piezas[i])
		}
	}
	o.mu.Unlock()
	manos := &tallerManos{o: o, autor: autor, quien: quien}
	for _, p := range suyas {
		m, ok := o.medir(p)
		if !ok {
			continue
		}
		s0 := obj.parecido(m)
		inicio := s0
		var hizo []string
		probar := func(que string, hacer func()) {
			v := p.Version
			hacer()
			if p.Version == v {
				return // no cambió nada
			}
			m2, ok := o.medir(p)
			if s := obj.parecido(m2); ok && s > s0+0.005 {
				s0 = s
				hizo = append(hizo, que)
				return
			}
			manos.Deshacer(p.Base) // no mejora: fuera
		}
		for ronda := 0; ronda < 2; ronda++ {
			// teñir con un color del vídeo
			for i, c := range obj.paleta {
				if i >= 2 {
					break
				}
				c := c
				probar("lo tiñe de "+nombreColor(c), func() { manos.Tenir(p.Base, c) })
			}
			// un fotograma del vídeo como textura
			for i, f := range obj.fotos {
				if i >= 2 {
					break
				}
				f := f
				probar("le pone de textura lo que vio («"+f+"»)", func() { manos.Pintar(p.Base, "foto:"+f) })
			}
			// el material que más se parece al color principal
			if mat := tallerMaterialParecido(o, obj.paleta[0]); mat != "" {
				probar("lo hace de "+mat, func() { manos.Pintar(p.Base, mat) })
			}
			// estirarlo hacia la forma del vídeo
			vert := m.alto / math.Max(1e-6, m.alto+m.ancho)
			if d := obj.vertical - vert; math.Abs(d) > 0.12 {
				k := 1 + math.Max(-0.5, math.Min(1.5, d*2.5))
				if d > 0 {
					probar("lo estira hacia arriba", func() { manos.Estirar(p.Base, 1, k, 1) })
				} else {
					probar("lo ensancha", func() { manos.Estirar(p.Base, 1/k, 1, 1/k) })
				}
			}
			// luz, si el vídeo era luminoso y la pieza no
			if obj.brillo > 0.55 && m.brillan < 0.05 {
				probar("le da luz", func() {
					manos.Escribir(p.Base, fmt.Sprintf("c.Brilla(0, c.Alto()+0.3, 0, %s, %s)", f2(math.Max(0.2, m.ancho*0.15)), tallerCol(tallerAclarar(obj.paleta[0], 0.5))))
				})
			}
			if m2, ok := o.medir(p); ok {
				m = m2
			}
		}
		antesMedio += inicio
		despuesMedio += s0
		n++
		if len(hizo) > 0 {
			o.Decir(quien, fmt.Sprintf("perfecciona «%s» con lo que vio en «%s»: %s. Parecido %.0f%% → %.0f%%",
				p.Titulo, obj.titulo, strings.Join(hizo, ", "), inicio*100, s0*100), "")
		} else {
			o.Decir(quien, fmt.Sprintf("repasa «%s» con lo que vio en «%s»: ya se parecía lo que podía (%.0f%%)", p.Titulo, obj.titulo, s0*100), "")
		}
	}
	if n > 0 {
		antesMedio /= float64(n)
		despuesMedio /= float64(n)
	}
	return
}

// tallerMaterialParecido: el material con nombre de color más parecido.
func tallerMaterialParecido(o *tallerObra, c [3]float64) string {
	mejor, dmin := "", math.Inf(1)
	for _, m := range tallerListaMateriales() {
		t := tallerColorDeTextura(o, m)
		if d := math.Abs(t[0]-c[0]) + math.Abs(t[1]-c[1]) + math.Abs(t[2]-c[2]); d < dmin {
			mejor, dmin = m, d
		}
	}
	return mejor
}

func newRand() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }
