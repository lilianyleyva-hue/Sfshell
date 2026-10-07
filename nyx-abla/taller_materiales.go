package main

// ============================================================
//  TALLER · MATERIALES Y TEXTURAS — como en Roblox Studio o Godot
// ------------------------------------------------------------
//  c.Material("piedra")       lo que se construya después lleva ese material
//  c.Material("")             vuelve al color liso
//  c.PintarTodo("madera")     toda la pieza (lo de antes también)
//
//  Hay materiales con nombre (madera, piedra, ladrillo, metal, hierba,
//  arena, tela, mármol, baldosa, cristal, hielo, oro, roca, agua, neón…),
//  pero CUALQUIER palabra es una textura: se inventa su dibujo a partir de
//  la palabra (siempre el mismo para la misma palabra). Con «|rrggbb» se
//  tiñe de un color («piedra|aa3322»). Con «foto:<nombre>» es una foto que
//  vio Abla, y tus imágenes de taller/texturas/ (png o jpg) también valen
//  por su nombre.
//  «neón» no es dibujo: brilla.
// ============================================================

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Material: lo que se construya a partir de aquí lleva ese material.
func (c *Cuerpo) Material(nombre string) {
	x := tallerExtraDe(c)
	x.cerrarMaterial(c.Marca())
	x.material = strings.TrimSpace(nombre)
	x.materialDesde = c.Marca()
}

// PintarTodo: toda la pieza, hasta aquí, de ese material.
func (c *Cuerpo) PintarTodo(nombre string) {
	x := tallerExtraDe(c)
	x.cerrarMaterial(c.Marca())
	x.materiales = append(x.materiales, tallerTramoMaterial{0, c.Marca(), strings.TrimSpace(nombre)})
	x.materialDesde = c.Marca()
}

type tallerTramoMaterial struct {
	desde, hasta int
	nombre       string
}

func (x *tallerExtra) cerrarMaterial(hasta int) {
	if x.material != "" && hasta > x.materialDesde {
		x.materiales = append(x.materiales, tallerTramoMaterial{x.materialDesde, hasta, x.material})
	}
}

// tallerMaterialPorVertice: el material de cada vértice (el último que se
// puso gana); el neón brilla y no lleva dibujo.
func tallerMaterialPorVertice(c *Cuerpo, x *tallerExtra) []string {
	x.cerrarMaterial(len(c.Pos) / 3)
	x.material = ""
	nv := len(c.Pos) / 3
	mat := make([]string, nv)
	for _, r := range x.materiales {
		for v := max(0, r.desde); v < r.hasta && v < nv; v++ {
			mat[v] = r.nombre
		}
	}
	for v, m := range mat {
		base := strings.SplitN(m, "|", 2)[0]
		if base == "neon" || base == "neón" {
			c.Emi[v] = 4
			mat[v] = ""
		} else if m != "" && c.Emi[v] >= 3.5 {
			mat[v] = "" // lo que ya brillaba sigue brillando
		}
	}
	return mat
}

// ---------- los dibujos ----------

type tallerPatron int

const (
	pRuido tallerPatron = iota
	pTablas
	pLadrillo
	pCepillado
	pHierba
	pArena
	pTrama
	pVetas
	pBaldosa
	pCristal
	pRayas
	pPuntos
	pOndas
	pCeldas
	pCuadros
	pEspiral
)

type tallerReceta struct {
	patron tallerPatron
	a, b   [3]float64 // los dos colores
}

var tallerRecetas = map[string]tallerReceta{
	"madera":   {pTablas, [3]float64{0.55, 0.36, 0.2}, [3]float64{0.38, 0.23, 0.12}},
	"piedra":   {pRuido, [3]float64{0.55, 0.55, 0.53}, [3]float64{0.35, 0.35, 0.34}},
	"roca":     {pRuido, [3]float64{0.45, 0.4, 0.36}, [3]float64{0.25, 0.22, 0.2}},
	"ladrillo": {pLadrillo, [3]float64{0.65, 0.28, 0.2}, [3]float64{0.8, 0.78, 0.72}},
	"metal":    {pCepillado, [3]float64{0.7, 0.72, 0.75}, [3]float64{0.5, 0.52, 0.55}},
	"hierba":   {pHierba, [3]float64{0.3, 0.6, 0.22}, [3]float64{0.18, 0.4, 0.14}},
	"cesped":   {pHierba, [3]float64{0.3, 0.6, 0.22}, [3]float64{0.18, 0.4, 0.14}},
	"arena":    {pArena, [3]float64{0.86, 0.78, 0.55}, [3]float64{0.72, 0.62, 0.42}},
	"tela":     {pTrama, [3]float64{0.6, 0.25, 0.3}, [3]float64{0.45, 0.18, 0.22}},
	"marmol":   {pVetas, [3]float64{0.92, 0.92, 0.9}, [3]float64{0.55, 0.55, 0.58}},
	"baldosa":  {pBaldosa, [3]float64{0.85, 0.85, 0.82}, [3]float64{0.45, 0.45, 0.45}},
	"cristal":  {pCristal, [3]float64{0.65, 0.85, 0.95}, [3]float64{0.9, 0.97, 1}},
	"hielo":    {pCristal, [3]float64{0.75, 0.9, 1}, [3]float64{0.95, 0.98, 1}},
	"oro":      {pCepillado, [3]float64{0.95, 0.78, 0.3}, [3]float64{0.75, 0.55, 0.15}},
	"agua":     {pOndas, [3]float64{0.15, 0.4, 0.7}, [3]float64{0.35, 0.65, 0.9}},
	"tierra":   {pRuido, [3]float64{0.45, 0.32, 0.2}, [3]float64{0.3, 0.2, 0.12}},
	"nieve":    {pArena, [3]float64{0.95, 0.96, 0.98}, [3]float64{0.82, 0.86, 0.92}},
	"plastico": {pRuido, [3]float64{0.85, 0.85, 0.85}, [3]float64{0.8, 0.8, 0.8}},
}

// tallerMateriales: las palabras que son materiales (para la lengua).
var tallerMateriales = func() map[string]bool {
	m := map[string]bool{"neon": true}
	for k := range tallerRecetas {
		m[k] = true
	}
	return m
}()

func tallerH(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// tallerRecetaDe: la receta de una textura por su nombre.
func tallerRecetaDe(nombre string) tallerReceta {
	base := tallerSinTildes.Replace(strings.ToLower(nombre))
	if r, ok := tallerRecetas[base]; ok {
		return r
	}
	// cualquier palabra: su dibujo y sus colores salen de ella
	h := tallerH(base)
	hue := float64(h%360) / 360
	c1 := TallerHSV(hue, 0.35+float64((h>>9)%40)/100, 0.45+float64((h>>17)%40)/100)
	c2 := TallerHSV(hue+0.08+float64((h>>23)%30)/100, 0.3+float64((h>>29)%50)/100, 0.25+float64((h>>35)%50)/100)
	return tallerReceta{tallerPatron((h >> 41) % 16), [3]float64{c1.R, c1.G, c1.B}, [3]float64{c2.R, c2.G, c2.B}}
}

const tallerLadoTex = 128

// tallerDibujar: la textura (128×128, se repite sin costuras).
func tallerDibujar(r tallerReceta, semilla uint64) *image.RGBA {
	L := tallerLadoTex
	im := image.NewRGBA(image.Rect(0, 0, L, L))
	s := int(semilla % 100000)
	ruido := func(x, y, f float64) float64 { // periódico en el borde
		return tallerRuidoCiclico(s, x*f/float64(L), y*f/float64(L), int(f))
	}
	for y := 0; y < L; y++ {
		for x := 0; x < L; x++ {
			fx, fy := float64(x), float64(y)
			t := 0.0
			switch r.patron {
			case pRuido:
				t = 0.6*ruido(fx, fy, 4) + 0.3*ruido(fx, fy, 16) + 0.1*ruido(fx, fy, 32)
			case pTablas:
				tabla := math.Floor(fy / 32)
				t = 0.5 + 0.35*math.Sin(fx/float64(L)*2*math.Pi*3+ruido(fx, fy+tabla*7, 8)*6) + 0.15*ruido(fx, fy, 32)
				if int(fy)%32 < 2 {
					t = 1.1
				}
			case pLadrillo:
				fila := int(fy) / 16
				dx := 0
				if fila%2 == 1 {
					dx = 16
				}
				if int(fy)%16 < 2 || (int(fx)+dx)%32 < 2 {
					t = 1.2 // la junta
				} else {
					t = 0.25 * ruido(fx, fy, 16)
				}
			case pCepillado:
				t = 0.5 + 0.4*ruido(fx*0.1, fy, 32) + 0.1*ruido(fx, fy, 8)
			case pHierba:
				t = 0.55*ruido(fx, fy, 32) + 0.45*ruido(fx, fy*0.3, 64)
			case pArena:
				t = 0.7*ruido(fx, fy, 64) + 0.3*ruido(fx, fy, 8)
			case pTrama:
				t = 0.5 + 0.25*math.Sin(fx*math.Pi/2) + 0.25*math.Sin(fy*math.Pi/2)
			case pVetas:
				t = math.Pow(math.Abs(math.Sin((fx+fy)/float64(L)*2*math.Pi*2+ruido(fx, fy, 8)*5)), 6)
			case pBaldosa:
				if int(fx)%32 < 2 || int(fy)%32 < 2 {
					t = 1.2
				} else {
					t = 0.15 * ruido(fx, fy, 16)
				}
			case pCristal:
				t = math.Max(0, math.Sin((fx-fy)/float64(L)*2*math.Pi*2)) * 0.8
			case pRayas:
				t = 0.5 + 0.5*math.Sin(fx/float64(L)*2*math.Pi*4)
			case pPuntos:
				cx, cy := math.Mod(fx, 16)-8, math.Mod(fy, 16)-8
				if cx*cx+cy*cy < 20 {
					t = 1
				}
			case pOndas:
				t = 0.5 + 0.5*math.Sin(fx/float64(L)*2*math.Pi*2+math.Sin(fy/float64(L)*2*math.Pi*3)*2)
			case pCeldas:
				t = tallerCeldas(fx, fy, s)
			case pCuadros:
				if (int(fx)/16+int(fy)/16)%2 == 0 {
					t = 1
				}
			default: // espiral
				dx, dy := fx-64, fy-64
				t = 0.5 + 0.5*math.Sin(math.Atan2(dy, dx)*3+math.Hypot(dx, dy)/6)
			}
			t = math.Max(0, math.Min(1.25, t))
			k := math.Min(1, t)
			c := [3]float64{}
			for i := 0; i < 3; i++ {
				c[i] = r.a[i]*(1-k) + r.b[i]*k
				if t > 1 { // juntas, brillos
					c[i] = r.b[i]
				}
			}
			im.SetRGBA(x, y, color.RGBA{uint8(clamp01(c[0]) * 255), uint8(clamp01(c[1]) * 255), uint8(clamp01(c[2]) * 255), 255})
		}
	}
	return im
}

func tallerRuidoCiclico(semilla int, x, y float64, periodo int) float64 {
	if periodo < 1 {
		periodo = 1
	}
	h := func(i, j int) float64 {
		i, j = ((i%periodo)+periodo)%periodo, ((j%periodo)+periodo)%periodo
		n := uint64(i*374761393+j*668265263) ^ uint64(semilla)*2246822519
		n = (n ^ (n >> 13)) * 1274126177
		return float64((n^(n>>16))&0xffff) / 65535
	}
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	i, j := int(x0), int(y0)
	a := h(i, j) + (h(i+1, j)-h(i, j))*fx
	b := h(i, j+1) + (h(i+1, j+1)-h(i, j+1))*fx
	return a + (b-a)*fy
}

func tallerCeldas(x, y float64, s int) float64 {
	L := float64(tallerLadoTex)
	best := 1e9
	for k := 0; k < 12; k++ {
		px := float64((s*31+k*977)%tallerLadoTex) + 0.5
		py := float64((s*17+k*613)%tallerLadoTex) + 0.5
		for _, d := range [][2]float64{{0, 0}, {L, 0}, {-L, 0}, {0, L}, {0, -L}} {
			best = math.Min(best, math.Hypot(x-px-d[0], y-py-d[1]))
		}
	}
	return math.Min(1, best/40)
}

// ---------- servir las texturas ----------

var (
	tallerTexMu    sync.Mutex
	tallerTexCache = map[string][]byte{}
)

// tallerTexturaPNG: el PNG de una textura por su nombre (con su tinte).
func tallerTexturaPNG(o *tallerObra, nombre string) ([]byte, error) {
	tallerTexMu.Lock()
	if d, ok := tallerTexCache[nombre]; ok {
		tallerTexMu.Unlock()
		return d, nil
	}
	tallerTexMu.Unlock()
	base, tinte := nombre, ""
	if i := strings.Index(nombre, "|"); i >= 0 {
		base, tinte = nombre[:i], nombre[i+1:]
	}
	var im image.Image
	switch {
	case strings.HasPrefix(base, "foto:"):
		im = tallerTexturaDeFoto(o, strings.TrimPrefix(base, "foto:"))
	default:
		im = tallerTexturaTuya(o, base)
	}
	if im == nil {
		im = tallerDibujar(tallerRecetaDe(base), tallerH(base))
	}
	if tinte != "" {
		if v, err := strconv.ParseUint(tinte, 16, 32); err == nil {
			im = tallerTenirImagen(im, [3]float64{float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		return nil, err
	}
	tallerTexMu.Lock()
	if len(tallerTexCache) > 500 {
		tallerTexCache = map[string][]byte{}
	}
	tallerTexCache[nombre] = b.Bytes()
	tallerTexMu.Unlock()
	return b.Bytes(), nil
}

// tallerTenirImagen: mezcla la textura con un color (sin perder el dibujo).
func tallerTenirImagen(im image.Image, c [3]float64) image.Image {
	r := im.Bounds()
	out := image.NewRGBA(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			R, G, B, _ := im.At(x, y).RGBA()
			l := (0.299*float64(R) + 0.587*float64(G) + 0.114*float64(B)) / 65535
			k := 0.55
			f := func(v uint32, ci float64) uint8 {
				o := float64(v)/65535*(1-k) + ci*(0.35+0.9*l)*k
				return uint8(clamp01(o) * 255)
			}
			out.SetRGBA(x, y, color.RGBA{f(R, c[0]), f(G, c[1]), f(B, c[2]), 255})
		}
	}
	return out
}

// tallerTexturaTuya: una imagen tuya de taller/texturas/<nombre>.png|.jpg.
func tallerTexturaTuya(o *tallerObra, nombre string) image.Image {
	if o == nil || strings.ContainsAny(nombre, "/\\") || strings.HasPrefix(nombre, ".") {
		return nil
	}
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		f, err := os.Open(filepath.Join(o.dir, "texturas", nombre+ext))
		if err != nil {
			continue
		}
		im, _, err := image.Decode(f)
		f.Close()
		if err == nil {
			return tallerReducir(im, 256)
		}
	}
	return nil
}

// tallerTexturaDeFoto: una foto que vio Abla (fotos/vistas/<nombre>.ppm).
func tallerTexturaDeFoto(o *tallerObra, nombre string) image.Image {
	if o == nil || strings.ContainsAny(nombre, "/\\") || strings.HasPrefix(nombre, ".") {
		return nil
	}
	w, h, rgb, err := tallerLeerPPM(filepath.Join(o.abla.dir, "fotos", "vistas", nombre+".ppm"))
	if err != nil {
		return nil
	}
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := rgb[(y*w+x)*3:]
			im.SetRGBA(x, y, color.RGBA{p[0], p[1], p[2], 255})
		}
	}
	return im
}

func tallerReducir(im image.Image, lado int) image.Image {
	r := im.Bounds()
	if r.Dx() <= lado && r.Dy() <= lado {
		return im
	}
	k := math.Max(float64(r.Dx()), float64(r.Dy())) / float64(lado)
	w, h := int(float64(r.Dx())/k), int(float64(r.Dy())/k)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y, im.At(r.Min.X+int(float64(x)*k), r.Min.Y+int(float64(y)*k)))
		}
	}
	return out
}

// tallerLeerPPM: una foto PPM (las que guarda Abla).
func tallerLeerPPM(ruta string) (int, int, []byte, error) {
	d, err := os.ReadFile(ruta)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(d) < 2 || string(d[:2]) != "P6" {
		return 0, 0, nil, fmt.Errorf("no es un PPM P6")
	}
	campos := []int{}
	i := 2
	for len(campos) < 3 && i < len(d) {
		for i < len(d) && (d[i] == ' ' || d[i] == '\n' || d[i] == '\r' || d[i] == '\t') {
			i++
		}
		if i < len(d) && d[i] == '#' {
			for i < len(d) && d[i] != '\n' {
				i++
			}
			continue
		}
		n := 0
		for i < len(d) && d[i] >= '0' && d[i] <= '9' {
			n = n*10 + int(d[i]-'0')
			i++
		}
		campos = append(campos, n)
	}
	i++
	if len(campos) < 3 || campos[2] != 255 || len(d)-i < campos[0]*campos[1]*3 {
		return 0, 0, nil, fmt.Errorf("PPM raro")
	}
	return campos[0], campos[1], d[i:], nil
}

// tallerUV: dónde cae un punto en su textura (proyección por la cara que
// más mira hacia cada eje; una textura cada 2 m).
func tallerUV(p, n [3]float64) (float64, float64) {
	ax, ay, az := math.Abs(n[0]), math.Abs(n[1]), math.Abs(n[2])
	switch {
	case ay >= ax && ay >= az:
		return p[0] / 2, p[2] / 2
	case ax >= az:
		return p[2] / 2, p[1] / 2
	}
	return p[0] / 2, p[1] / 2
}

func tallerRutaTextura(mux *http.ServeMux, o *tallerObra) {
	mux.HandleFunc("/api/taller-textura", func(w http.ResponseWriter, r *http.Request) {
		d, err := tallerTexturaPNG(o, r.URL.Query().Get("n"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(d)
	})
}

func tallerListaMateriales() []string {
	var l []string
	for k := range tallerRecetas {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}
