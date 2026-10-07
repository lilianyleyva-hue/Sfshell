package main

// ============================================================
//  TALLER · MOTOR — un pequeño Unity para Nyx y Abla
// ------------------------------------------------------------
//  Cada pieza es un objeto vivo:
//
//   · PARTES: c.Parte("puerta", px, py, pz) — lo que se construye
//     después es otra parte, que gira alrededor de ese pivote y se
//     puede mover, girar, escalar, esconder.
//   · SCRIPTS: funciones que la pieza puede tener (todas opcionales):
//       func Empezar(o *mundo.Objeto)                    al aparecer
//       func Actuar(o *mundo.Objeto, dt float64)         10 veces por segundo
//       func AlUsar(o *mundo.Objeto, quien string)       alguien pulsa E cerca
//       func AlEntrar(o *mundo.Objeto, quien string)     alguien entra en su zona
//       func AlSalir(o *mundo.Objeto, quien string)      … y sale
//       func AlTocar(o *mundo.Objeto, quien, parte string)  alguien toca una parte
//   · SONIDO: o.Sonar("campana", 1), o.Ambiente("agua", 0.6), música
//     («musica:60 64 67»), y archivos tuyos de taller/sonidos/.
//   · FÍSICA EXACTA: se calcula con la forma de verdad de cada parte
//     (dónde hay suelo para pisar y dónde hay algo sólido, a cada
//     altura), así que se suben escaleras, se anda por los pisos de
//     una torre, se baja a un sótano, y nada choca más de lo que mide.
//     c.Hueco(…) abre el suelo del mundo para bajar; c.Fantasma(true)
//     hace lo que venga atravesable (hojas, luces, decorado).
//   · JUEGOS: o.Puntos(quien, n) lleva el marcador; los seres de Abla y
//     Nyx también juegan (van hacia lo que hay que tocar).
// ============================================================

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
)

// ---------- lo que el código de una pieza apunta mientras se construye ----------

type tallerMarcaParte struct {
	nombre string
	desde  int // vértice donde empieza
	pivote [3]float64
}

type tallerExtra struct {
	partes        []tallerMarcaParte
	huecos        [][4]float64 // x0, z0, x1, z1 (en el sitio de la pieza)
	fantasma      [][2]int     // tramos de vértices sin física
	fantasmaDesde int
	brillo        [][2]int // tramos de vértices que dan luz
	brilloDesde   int
	materiales    []tallerTramoMaterial
	material      string
	materialDesde int
	matVertice    []string // al terminar: el material de cada vértice
}

var (
	tallerExtrasMu sync.Mutex
	tallerExtras   = map[*Cuerpo]*tallerExtra{}
)

func tallerExtraDe(c *Cuerpo) *tallerExtra {
	tallerExtrasMu.Lock()
	defer tallerExtrasMu.Unlock()
	if x, ok := tallerExtras[c]; ok {
		return x
	}
	if len(tallerExtras) > 4000 { // cuerpos de otros compiladores que nadie recogió
		tallerExtras = map[*Cuerpo]*tallerExtra{}
	}
	x := &tallerExtra{fantasmaDesde: -1, brilloDesde: -1}
	tallerExtras[c] = x
	return x
}

func tallerSoltarExtra(c *Cuerpo) *tallerExtra {
	tallerExtrasMu.Lock()
	defer tallerExtrasMu.Unlock()
	x := tallerExtras[c]
	delete(tallerExtras, c)
	if x == nil {
		x = &tallerExtra{fantasmaDesde: -1, brilloDesde: -1}
	}
	if x.fantasmaDesde >= 0 {
		x.fantasma = append(x.fantasma, [2]int{x.fantasmaDesde, len(c.Pos) / 3})
		x.fantasmaDesde = -1
	}
	tallerAplicarBrillo(c, x)
	x.matVertice = tallerMaterialPorVertice(c, x)
	return x
}

// Parte: lo que se construya a partir de aquí es una parte con nombre, que
// se mueve y gira alrededor del pivote (px, py, pz). Parte("") vuelve al
// cuerpo principal.
func (c *Cuerpo) Parte(nombre string, px, py, pz float64) {
	x := tallerExtraDe(c)
	x.partes = append(x.partes, tallerMarcaParte{nombre: nombre, desde: c.Marca(), pivote: [3]float64{px, py, pz}})
}

// Hueco: en ese rectángulo el suelo del mundo no cuenta (para bajar a un
// sótano, a una cueva, a otro nivel).
func (c *Cuerpo) Hueco(x0, z0, x1, z1 float64) {
	x := tallerExtraDe(c)
	x.huecos = append(x.huecos, [4]float64{math.Min(x0, x1), math.Min(z0, z1), math.Max(x0, x1), math.Max(z0, z1)})
}

// Fantasma: lo que se construya mientras esté activo se ve pero no se
// choca ni se pisa.
func (c *Cuerpo) Fantasma(si bool) {
	x := tallerExtraDe(c)
	if si && x.fantasmaDesde < 0 {
		x.fantasmaDesde = c.Marca()
	}
	if !si && x.fantasmaDesde >= 0 {
		x.fantasma = append(x.fantasma, [2]int{x.fantasmaDesde, c.Marca()})
		x.fantasmaDesde = -1
	}
}

// tallerTransformarExtra: cuando se gira, escala o mueve el cuerpo entero,
// los pivotes y los huecos van con él.
func tallerTransformarExtra(c *Cuerpo, f func(p [3]float64) [3]float64) {
	tallerExtrasMu.Lock()
	x := tallerExtras[c]
	tallerExtrasMu.Unlock()
	if x == nil {
		return
	}
	for i := range x.partes {
		x.partes[i].pivote = f(x.partes[i].pivote)
	}
	for i, h := range x.huecos {
		a, b := f([3]float64{h[0], 0, h[1]}), f([3]float64{h[2], 0, h[3]})
		d, e := f([3]float64{h[0], 0, h[3]}), f([3]float64{h[2], 0, h[1]})
		x.huecos[i] = [4]float64{math.Min(math.Min(a[0], b[0]), math.Min(d[0], e[0])), math.Min(math.Min(a[2], b[2]), math.Min(d[2], e[2])),
			math.Max(math.Max(a[0], b[0]), math.Max(d[0], e[0])), math.Max(math.Max(a[2], b[2]), math.Max(d[2], e[2]))}
	}
}

// ---------- compilar una pieza (con sus scripts) ----------

type tallerProg struct {
	construir func(*Cuerpo)
	empezar   func(*TallerObjeto)
	actuar    func(*TallerObjeto, float64)
	alUsar    func(*TallerObjeto, string)
	alEntrar  func(*TallerObjeto, string)
	alSalir   func(*TallerObjeto, string)
	alTocar   func(*TallerObjeto, string, string)
}

func (p *tallerProg) tieneScripts() bool {
	return p.empezar != nil || p.actuar != nil || p.alUsar != nil || p.alEntrar != nil || p.alSalir != nil || p.alTocar != nil
}

type tallerParteMalla struct {
	nombre string
	pivote [3]float64
	cuerpo *Cuerpo  // con el pivote en el origen
	solido []bool   // por triángulo
	mat    []string // material de cada triángulo ("" = color liso)
}

type tallerCompilada struct {
	prog   *tallerProg
	entero *Cuerpo
	matTri []string            // material de cada triángulo del cuerpo entero
	partes []*tallerParteMalla // la 0 es el cuerpo principal ("")
	huecos [][4]float64
}

var tallerRePaquete = regexp.MustCompile(`(?m)^package\s+(\w+)`)

// tallerCompilar: lee el código encerrado (como todo en Nyx Mundo: solo ve
// el paquete "mundo"), construye el cuerpo y lo parte en sus partes.
func tallerCompilar(codigo string) (*tallerCompilada, error) {
	i := interp.New(interp.Options{GoPath: "/nyx-sin-disco", SourcecodeFilesystem: sinDisco})
	if err := i.Use(paqueteMundo); err != nil {
		return nil, err
	}
	if _, err := conTiempo(4*time.Second, func() (any, error) { return i.Eval(codigo) }); err != nil {
		return nil, fmt.Errorf("el código no compila: %v", err)
	}
	pkg := "objeto"
	if m := tallerRePaquete.FindStringSubmatch(codigo); m != nil {
		pkg = m[1]
	}
	vc, err := i.Eval(pkg + ".Construir")
	if err != nil {
		return nil, errors.New("falta  func Construir(c *mundo.Cuerpo)")
	}
	construir, ok := vc.Interface().(func(*Cuerpo))
	if !ok {
		return nil, errors.New("Construir tiene que ser  func Construir(c *mundo.Cuerpo)")
	}
	p := &tallerProg{construir: construir}
	opt := func(n string) any {
		v, err := i.Eval(pkg + "." + n)
		if err != nil || !v.IsValid() {
			return nil
		}
		return v.Interface()
	}
	p.empezar, _ = opt("Empezar").(func(*TallerObjeto))
	p.actuar, _ = opt("Actuar").(func(*TallerObjeto, float64))
	p.alUsar, _ = opt("AlUsar").(func(*TallerObjeto, string))
	p.alEntrar, _ = opt("AlEntrar").(func(*TallerObjeto, string))
	p.alSalir, _ = opt("AlSalir").(func(*TallerObjeto, string))
	p.alTocar, _ = opt("AlTocar").(func(*TallerObjeto, string, string))
	c := &Cuerpo{}
	tallerExtraDe(c)
	_, err = conTiempo(4*time.Second, func() (any, error) { construir(c); return nil, nil })
	ex := tallerSoltarExtra(c)
	if err != nil {
		return nil, fmt.Errorf("Construir: %v", err)
	}
	if len(c.Pos) == 0 {
		return nil, errors.New("Construir no hizo ningún cuerpo")
	}
	return tallerPartir(p, c, ex), nil
}

// tallerPartir: separa el cuerpo en sus partes (cada una con el pivote en
// el origen) y apunta qué triángulos son sólidos.
func tallerPartir(p *tallerProg, c *Cuerpo, ex *tallerExtra) *tallerCompilada {
	nv := len(c.Pos) / 3
	nombreDe := make([]string, nv)
	marcas := append([]tallerMarcaParte(nil), ex.partes...)
	sort.SliceStable(marcas, func(a, b int) bool { return marcas[a].desde < marcas[b].desde })
	pivotes := map[string][3]float64{"": {}}
	orden := []string{""}
	for k, mk := range marcas {
		fin := nv
		if k+1 < len(marcas) {
			fin = marcas[k+1].desde
		}
		for v := mk.desde; v < fin && v < nv; v++ {
			nombreDe[v] = mk.nombre
		}
		if _, ok := pivotes[mk.nombre]; !ok {
			pivotes[mk.nombre] = mk.pivote
			orden = append(orden, mk.nombre)
		}
	}
	fantasma := make([]bool, nv)
	for _, r := range ex.fantasma {
		for v := r[0]; v < r[1] && v < nv; v++ {
			if v >= 0 {
				fantasma[v] = true
			}
		}
	}
	out := &tallerCompilada{prog: p, entero: c, huecos: ex.huecos}
	for t := 0; t+2 < nv; t += 3 {
		m := ""
		if t < len(ex.matVertice) {
			m = ex.matVertice[t]
		}
		out.matTri = append(out.matTri, m)
	}
	for _, n := range orden {
		pv := pivotes[n]
		pm := &tallerParteMalla{nombre: n, pivote: pv, cuerpo: &Cuerpo{}}
		for t := 0; t+2 < nv; t += 3 {
			if nombreDe[t] != n {
				continue
			}
			for k := t; k < t+3; k++ {
				pm.cuerpo.Pos = append(pm.cuerpo.Pos, c.Pos[k*3]-float32(pv[0]), c.Pos[k*3+1]-float32(pv[1]), c.Pos[k*3+2]-float32(pv[2]))
				pm.cuerpo.Nor = append(pm.cuerpo.Nor, c.Nor[k*3:k*3+3]...)
				pm.cuerpo.Col = append(pm.cuerpo.Col, c.Col[k*3:k*3+3]...)
				pm.cuerpo.Emi = append(pm.cuerpo.Emi, c.Emi[k])
			}
			pm.solido = append(pm.solido, !fantasma[t] && c.Emi[t] < 3.5)
			m := ""
			if t < len(ex.matVertice) {
				m = ex.matVertice[t]
			}
			pm.mat = append(pm.mat, m)
		}
		if len(pm.cuerpo.Pos) > 0 || n == "" {
			out.partes = append(out.partes, pm)
		}
	}
	return out
}

// ---------- física exacta ----------

// tallerFisica: por cada celda de 20 cm, dónde hay suelo que pisar y qué
// alturas están ocupadas por algo sólido. Cada celda: [nSuelos, suelos…,
// x0, z0, x1, z1 (lo que de verdad ocupa dentro de la celda), desde,
// hasta, desde, hasta…].
type tallerFisica struct {
	C      float64              `json:"c"`
	Celdas map[string][]float32 `json:"celdas"`
	Huecos [][4]float64         `json:"huecos,omitempty"`
}

const tallerCelda = 0.2

func tallerCalcularFisica(c *Cuerpo, solido []bool, huecos [][4]float64) *tallerFisica {
	// las celdas son de 20 cm; en las cosas enormes, más grandes (que no
	// pase de unas 400 por lado)
	celda := tallerCelda
	mn, mx := math.Inf(1), math.Inf(-1)
	for i := 0; i+2 < len(c.Pos); i += 3 {
		for _, v := range []float32{c.Pos[i], c.Pos[i+2]} {
			mn, mx = math.Min(mn, float64(v)), math.Max(mx, float64(v))
		}
	}
	if ext := mx - mn; ext/celda > 400 {
		celda = ext / 400
	}
	suelos := map[[2]int][]float64{}
	ocupa := map[[2]int][]float64{}
	caja := map[[2]int][4]float64{}
	nt := len(c.Pos) / 9
	for t := 0; t < nt; t++ {
		if t < len(solido) && !solido[t] {
			continue
		}
		var v [3][3]float64
		for k := 0; k < 3; k++ {
			for j := 0; j < 3; j++ {
				v[k][j] = float64(c.Pos[(t*3+k)*3+j])
			}
		}
		e1 := [3]float64{v[1][0] - v[0][0], v[1][1] - v[0][1], v[1][2] - v[0][2]}
		e2 := [3]float64{v[2][0] - v[0][0], v[2][1] - v[0][1], v[2][2] - v[0][2]}
		n := cruz(e1, e2)
		l := math.Sqrt(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])
		if l < 1e-12 {
			continue
		}
		pisable := math.Abs(n[1])/l > 0.6
		n1 := int(math.Ceil(math.Sqrt(e1[0]*e1[0]+e1[1]*e1[1]+e1[2]*e1[2]) / (celda / 2)))
		n2 := int(math.Ceil(math.Sqrt(e2[0]*e2[0]+e2[1]*e2[1]+e2[2]*e2[2]) / (celda / 2)))
		n1, n2 = max(1, min(n1, 120)), max(1, min(n2, 120))
		for a := 0; a <= n1; a++ {
			for b := 0; b <= n2; b++ {
				u, w := float64(a)/float64(n1), float64(b)/float64(n2)
				if u+w > 1+1e-9 {
					continue
				}
				x := v[0][0] + e1[0]*u + e2[0]*w
				y := v[0][1] + e1[1]*u + e2[1]*w
				z := v[0][2] + e1[2]*u + e2[2]*w
				k := [2]int{int(math.Floor(x / celda)), int(math.Floor(z / celda))}
				ocupa[k] = append(ocupa[k], y)
				if b, ok := caja[k]; ok {
					caja[k] = [4]float64{math.Min(b[0], x), math.Min(b[1], z), math.Max(b[2], x), math.Max(b[3], z)}
				} else {
					caja[k] = [4]float64{x, z, x, z}
				}
				if pisable {
					suelos[k] = append(suelos[k], y)
				}
			}
		}
	}
	f := &tallerFisica{C: celda, Celdas: map[string][]float32{}, Huecos: huecos}
	for k, ys := range ocupa {
		sort.Float64s(ys)
		var tramos [][2]float64
		for _, y := range ys {
			if n := len(tramos); n > 0 && y-tramos[n-1][1] <= 0.3 {
				tramos[n-1][1] = y
			} else {
				tramos = append(tramos, [2]float64{y, y})
			}
		}
		ss := suelos[k]
		sort.Float64s(ss)
		var pisos []float64
		for _, y := range ss {
			if n := len(pisos); n > 0 && y-pisos[n-1] <= 0.06 {
				pisos[n-1] = y
			} else {
				pisos = append(pisos, y)
			}
		}
		celda := []float32{float32(len(pisos))}
		for _, y := range pisos {
			celda = append(celda, float32(math.Round(y*1000)/1000))
		}
		b := caja[k]
		for _, v := range b {
			celda = append(celda, float32(math.Round(v*1000)/1000))
		}
		for _, t := range tramos {
			celda = append(celda, float32(math.Round(t[0]*1000)/1000), float32(math.Round(t[1]*1000)/1000))
		}
		f.Celdas[strconv.Itoa(k[0])+","+strconv.Itoa(k[1])] = celda
	}
	return f
}

// ---------- matrices (como las de la ventana: por columnas) ----------

type tallerMat [16]float64

func tallerIdentidad() tallerMat { return tallerMat{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

func (a tallerMat) por(b tallerMat) tallerMat {
	var o tallerMat
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				o[j*4+i] += a[k*4+i] * b[j*4+k]
			}
		}
	}
	return o
}

func tallerTras(x, y, z float64) tallerMat {
	m := tallerIdentidad()
	m[12], m[13], m[14] = x, y, z
	return m
}

func tallerRotY(a float64) tallerMat {
	c, s := math.Cos(a), math.Sin(a)
	return tallerMat{c, 0, s, 0, 0, 1, 0, 0, -s, 0, c, 0, 0, 0, 0, 1}
}

func tallerRotX(a float64) tallerMat {
	c, s := math.Cos(a), math.Sin(a)
	return tallerMat{1, 0, 0, 0, 0, c, s, 0, 0, -s, c, 0, 0, 0, 0, 1}
}

func tallerRotZ(a float64) tallerMat {
	c, s := math.Cos(a), math.Sin(a)
	return tallerMat{c, s, 0, 0, -s, c, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func tallerEsc(k float64) tallerMat {
	return tallerMat{k, 0, 0, 0, 0, k, 0, 0, 0, 0, k, 0, 0, 0, 0, 1}
}

func (a tallerMat) punto(p [3]float64) [3]float64 {
	return [3]float64{a[0]*p[0] + a[4]*p[1] + a[8]*p[2] + a[12], a[1]*p[0] + a[5]*p[1] + a[9]*p[2] + a[13], a[2]*p[0] + a[6]*p[1] + a[10]*p[2] + a[14]}
}

// ---------- el motor ----------

type tallerParteViva struct {
	nombre  string
	pivote  [3]float64
	pos     [3]float64
	rot     [3]float64
	esc     float64
	visible bool
	vacia   bool // sin triángulos (solo un pivote)
}

type tallerPiezaViva struct {
	pieza    *tallerPieza
	modelo   string // la versión de la pieza que está compilada
	comp     *tallerCompilada
	partes   []*tallerParteViva
	obj      *TallerObjeto
	nacio    time.Time
	empezado bool
	rota     string
	lentas   int
	zona     float64
	dentro   map[string]bool
	tocando  map[string]bool
	ambiente map[string]float64
}

type tallerSonido struct {
	N    int     `json:"n"`
	S    string  `json:"s"`
	Tono float64 `json:"tono"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
}

type tallerMotor struct {
	mu        sync.Mutex
	o         *tallerObra
	vivos     map[string]*tallerPiezaViva // por pieza (Base)
	fisicas   map[string]*tallerFisica
	jug       [3]float64
	jugHora   time.Time
	usar      bool
	sonidos   []tallerSonido
	nsonido   int
	llevar    *[3]float64
	nllevar   int
	compilado map[string]string // base → error de compilación (para no repetir)
}

func tallerNuevoMotor(o *tallerObra) *tallerMotor {
	return &tallerMotor{o: o, vivos: map[string]*tallerPiezaViva{}, fisicas: map[string]*tallerFisica{}, compilado: map[string]string{}}
}

// vivo: la pieza, compilada (si cambió de versión, se vuelve a compilar).
func (mt *tallerMotor) vivo(p *tallerPieza) *tallerPiezaViva {
	mt.o.mu.Lock()
	id, base := p.ID, p.Base
	mt.o.mu.Unlock()
	mt.mu.Lock()
	v := mt.vivos[base]
	if v != nil && v.modelo == id {
		mt.mu.Unlock()
		return v
	}
	if e, ok := mt.compilado[id]; ok && e != "" {
		mt.mu.Unlock()
		return nil
	}
	mt.mu.Unlock()
	d, err := os.ReadFile(filepath.Join(mt.o.m.dir, "objetos", id+".go"))
	if err != nil {
		mt.mu.Lock()
		mt.compilado[id] = err.Error()
		mt.mu.Unlock()
		return nil
	}
	comp, err := tallerCompilar(string(d))
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if err != nil {
		mt.compilado[id] = err.Error()
		return nil
	}
	nv := &tallerPiezaViva{pieza: p, modelo: id, comp: comp, nacio: time.Now(), zona: 2.5,
		dentro: map[string]bool{}, tocando: map[string]bool{}, ambiente: map[string]float64{}}
	for _, pm := range comp.partes {
		nv.partes = append(nv.partes, &tallerParteViva{nombre: pm.nombre, pivote: pm.pivote, esc: 1, visible: true, vacia: len(pm.cuerpo.Pos) == 0})
	}
	nv.obj = &TallerObjeto{mt: mt, v: nv}
	mt.vivos[base] = nv
	return nv
}

func (v *tallerPiezaViva) parte(nombre string) *tallerParteViva {
	for _, p := range v.partes {
		if p.nombre == nombre {
			return p
		}
	}
	return nil
}

// matrizDe: dónde está una parte en el mundo ahora mismo.
func (v *tallerPiezaViva) matrizDe(p *tallerParteViva) tallerMat {
	pz := v.pieza
	m := tallerTras(pz.X, pz.Y, pz.Z).por(tallerRotY(pz.Rumbo)).por(tallerRotX(pz.RotX))
	m = m.por(tallerTras(p.pivote[0]+p.pos[0], p.pivote[1]+p.pos[1], p.pivote[2]+p.pos[2]))
	m = m.por(tallerRotY(p.rot[1])).por(tallerRotX(p.rot[0])).por(tallerRotZ(p.rot[2])).por(tallerEsc(p.esc))
	return m
}

// mallaDe: el cuerpo de una parte («pieza~parte»), para dibujarlo.
func (mt *tallerMotor) mallaDe(modelo string) (*Cuerpo, []bool, [][4]float64, bool) {
	c, s, h, _, ok := mt.mallaConMateriales(modelo)
	return c, s, h, ok
}

// mallaConMateriales: lo mismo, con el material de cada triángulo.
func (mt *tallerMotor) mallaConMateriales(modelo string) (*Cuerpo, []bool, [][4]float64, []string, bool) {
	i := strings.LastIndex(modelo, "~")
	if i < 0 {
		return nil, nil, nil, nil, false
	}
	id, nombre := modelo[:i], modelo[i+1:]
	mt.o.mu.Lock()
	var p *tallerPieza
	for _, q := range mt.o.Piezas {
		if q.ID == id {
			p = q
		}
	}
	mt.o.mu.Unlock()
	var comp *tallerCompilada
	if p != nil {
		if v := mt.vivo(p); v != nil {
			comp = v.comp
		}
	}
	if comp == nil { // una versión vieja, o una pieza que ya no está
		d, err := os.ReadFile(filepath.Join(mt.o.m.dir, "objetos", id+".go"))
		if err != nil {
			return nil, nil, nil, nil, false
		}
		if comp, err = tallerCompilar(string(d)); err != nil {
			return nil, nil, nil, nil, false
		}
	}
	for _, pm := range comp.partes {
		if pm.nombre == nombre {
			var h [][4]float64
			if nombre == "" {
				h = comp.huecos
			}
			return pm.cuerpo, pm.solido, h, pm.mat, true
		}
	}
	return nil, nil, nil, nil, false
}

// Fisica: la de un modelo (una parte de una pieza, o una cosa que vio Nyx).
func (mt *tallerMotor) Fisica(modelo string) *tallerFisica {
	mt.mu.Lock()
	f := mt.fisicas[modelo]
	mt.mu.Unlock()
	if f != nil {
		return f
	}
	if strings.HasPrefix(modelo, "fig:") {
		c, err := mt.o.m.CuerpoDeFigura(strings.TrimPrefix(modelo, "fig:"))
		if err != nil {
			return nil
		}
		f = tallerCalcularFisica(c, nil, nil)
	} else {
		c, solido, huecos, ok := mt.mallaDe(modelo)
		if !ok {
			return nil
		}
		f = tallerCalcularFisica(c, solido, huecos)
	}
	mt.mu.Lock()
	if len(mt.fisicas) > 3000 {
		mt.fisicas = map[string]*tallerFisica{}
	}
	mt.fisicas[modelo] = f
	mt.mu.Unlock()
	return f
}

// Jugador: dónde está quien camina (lo dice la ventana) y si pulsó E.
func (mt *tallerMotor) Jugador(x, y, z float64, usar bool) {
	mt.mu.Lock()
	mt.jug, mt.jugHora = [3]float64{x, y, z}, time.Now()
	if usar {
		mt.usar = true
	}
	mt.mu.Unlock()
	mt.o.m.Posicion(x, z)
}

// ---------- el bucle: 10 veces por segundo ----------

func (mt *tallerMotor) Correr(parar <-chan struct{}) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-parar:
			return
		case <-t.C:
			func() {
				defer func() { _ = recover() }()
				mt.paso(0.1)
			}()
		}
	}
}

type tallerQuien struct {
	nombre  string
	x, y, z float64
	jugador bool
}

func (mt *tallerMotor) paso(dt float64) {
	mt.o.mu.Lock()
	piezas := append([]*tallerPieza(nil), mt.o.Piezas...)
	mt.o.mu.Unlock()
	mt.mu.Lock()
	jug, jugHora, usar := mt.jug, mt.jugHora, mt.usar
	mt.usar = false
	// las que ya no están
	estan := map[string]bool{}
	for _, p := range piezas {
		estan[p.Base] = true
	}
	for b := range mt.vivos {
		if !estan[b] {
			delete(mt.vivos, b)
		}
	}
	mt.mu.Unlock()
	hayJugador := time.Since(jugHora) < 3*time.Second
	// quién anda por ahí: tú y los seres
	var quienes []tallerQuien
	if hayJugador {
		quienes = append(quienes, tallerQuien{"tú", jug[0], jug[1], jug[2], true})
	}
	for _, s := range mt.o.m.SeresCerca(jug[0], jug[2], 70) {
		quienes = append(quienes, tallerQuien{nombre: tallerNombreSer(s.Especie), x: s.X, y: math.NaN(), z: s.Z})
	}
	// E: la pieza más cercana que se pueda usar
	var usada *tallerPiezaViva
	mejor := 3.5
	for _, p := range piezas {
		if math.Hypot(p.X-jug[0], p.Z-jug[2]) > 70 {
			continue
		}
		v := mt.vivo(p)
		if v == nil {
			continue
		}
		mt.mu.Lock()
		if usar && v.comp.prog.alUsar != nil && v.rota == "" {
			conNombre := len(v.partes) > 1
			for _, pt := range v.partes {
				if conNombre && pt.nombre == "" {
					continue // con partes, se usa una parte (la puerta, el botón…)
				}
				c := v.matrizDe(pt).punto([3]float64{})
				if d := math.Hypot(c[0]-jug[0], c[2]-jug[2]); d < mejor && math.Abs(c[1]-jug[1]-1) < 3.5 {
					mejor, usada = d, v
				}
			}
		}
		mt.mu.Unlock()
		mt.vivir(v, dt, quienes)
	}
	if usada != nil {
		mt.llamar(usada, func(o *TallerObjeto) { usada.comp.prog.alUsar(o, "tú") })
	}
	tallerActualizarMetas(mt)
}

// vivir: los scripts de una pieza en este instante.
func (mt *tallerMotor) vivir(v *tallerPiezaViva, dt float64, quienes []tallerQuien) {
	pr := v.comp.prog
	if !pr.tieneScripts() {
		return
	}
	mt.mu.Lock()
	rota := v.rota
	mt.mu.Unlock()
	if rota != "" {
		return
	}
	if !v.empezado {
		v.empezado = true
		if pr.empezar != nil {
			mt.llamar(v, func(o *TallerObjeto) { pr.empezar(o) })
		}
	}
	if pr.actuar != nil {
		mt.llamar(v, func(o *TallerObjeto) { pr.actuar(o, dt) })
	}
	// entrar y salir de su zona
	if pr.alEntrar != nil || pr.alSalir != nil {
		for _, q := range quienes {
			d := math.Hypot(q.x-v.pieza.X, q.z-v.pieza.Z)
			esta := d < v.zona && (math.IsNaN(q.y) || math.Abs(q.y-v.pieza.Y) < 4)
			if esta && !v.dentro[q.nombre] {
				v.dentro[q.nombre] = true
				if pr.alEntrar != nil {
					mt.llamar(v, func(o *TallerObjeto) { pr.alEntrar(o, q.nombre) })
				}
			} else if !esta && v.dentro[q.nombre] {
				delete(v.dentro, q.nombre)
				if pr.alSalir != nil {
					mt.llamar(v, func(o *TallerObjeto) { pr.alSalir(o, q.nombre) })
				}
			}
		}
	}
	// tocar una parte
	if pr.alTocar != nil {
		mt.mu.Lock()
		type centro struct {
			nombre string
			p      [3]float64
		}
		var cs []centro
		for _, pt := range v.partes {
			if pt.nombre != "" && pt.visible {
				cs = append(cs, centro{pt.nombre, v.matrizDe(pt).punto([3]float64{})})
			}
		}
		mt.mu.Unlock()
		for _, q := range quienes {
			for _, c := range cs {
				k := q.nombre + "|" + c.nombre
				cerca := math.Hypot(q.x-c.p[0], q.z-c.p[2]) < 0.9
				if math.IsNaN(q.y) {
					cerca = cerca && c.p[1]-v.pieza.Y < 2.4
				} else {
					cerca = cerca && q.y < c.p[1]+0.6 && q.y+1.9 > c.p[1]-0.4
				}
				if cerca && !v.tocando[k] {
					v.tocando[k] = true
					nombre, parte := q.nombre, c.nombre
					mt.llamar(v, func(o *TallerObjeto) { pr.alTocar(o, nombre, parte) })
				} else if !cerca {
					delete(v.tocando, k)
				}
			}
		}
	}
}

// llamar: ejecuta un script con tiempo límite; si se rompe o tarda
// demasiado tres veces, sus scripts se apagan (la pieza se sigue viendo).
func (mt *tallerMotor) llamar(v *tallerPiezaViva, f func(o *TallerObjeto)) {
	_, err := conTiempo(200*time.Millisecond, func() (any, error) { f(v.obj); return nil, nil })
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if err == nil {
		v.lentas = 0
		return
	}
	if strings.Contains(err.Error(), "tarda") {
		if v.lentas++; v.lentas < 3 {
			return
		}
	}
	v.rota = err.Error()
	go mt.o.Decir("motor", fmt.Sprintf("los scripts de %s se apagaron: %v", v.pieza.Base, err), "")
}

// ---------- lo que ve la ventana ----------

type tallerVistaParte struct {
	M string      `json:"m"` // modelo: pieza~parte
	F [16]float32 `json:"f"`
}

type tallerVistaPieza struct {
	B      string             `json:"b"`
	Titulo string             `json:"titulo"`
	Autor  string             `json:"autor"`
	X      float64            `json:"x"`
	Y      float64            `json:"y"`
	Z      float64            `json:"z"`
	Usar   bool               `json:"usar,omitempty"`
	Partes []tallerVistaParte `json:"partes"`
}

type tallerVistaAmbiente struct {
	K string  `json:"k"`
	S string  `json:"s"`
	V float64 `json:"v"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Vista: lo que hay cerca de quien camina.
func (mt *tallerMotor) Vista(desdeSonido, desdeLlevar int) map[string]any {
	mt.mu.Lock()
	jug := mt.jug
	mt.mu.Unlock()
	mt.o.mu.Lock()
	piezas := append([]*tallerPieza(nil), mt.o.Piezas...)
	mt.o.mu.Unlock()
	var vp []tallerVistaPieza
	var amb []tallerVistaAmbiente
	for _, p := range piezas {
		if math.Hypot(p.X-jug[0], p.Z-jug[2]) > 90 {
			continue
		}
		v := mt.vivo(p)
		if v == nil {
			continue
		}
		mt.mu.Lock()
		x := tallerVistaPieza{B: p.Base, Titulo: p.Titulo, Autor: p.Autor, X: p.X, Y: p.Y, Z: p.Z, Usar: v.comp.prog.alUsar != nil && v.rota == ""}
		for _, pt := range v.partes {
			if !pt.visible || pt.vacia {
				continue
			}
			m := v.matrizDe(pt)
			var f [16]float32
			for i := range m {
				f[i] = float32(m[i])
			}
			x.Partes = append(x.Partes, tallerVistaParte{M: v.modelo + "~" + pt.nombre, F: f})
		}
		for k, vol := range v.ambiente {
			if vol <= 0 {
				continue
			}
			i := strings.Index(k, "|")
			pt := v.parte(k[:i])
			if pt == nil {
				pt = v.partes[0]
			}
			c := v.matrizDe(pt).punto([3]float64{0, 1, 0})
			amb = append(amb, tallerVistaAmbiente{K: p.Base + "|" + k, S: k[i+1:], V: vol, X: c[0], Y: c[1], Z: c[2]})
		}
		mt.mu.Unlock()
		vp = append(vp, x)
	}
	mt.mu.Lock()
	var ss []tallerSonido
	for _, s := range mt.sonidos {
		if s.N > desdeSonido {
			ss = append(ss, s)
		}
	}
	res := map[string]any{"piezas": vp, "ambientes": amb, "sonidos": ss, "nsonido": mt.nsonido, "nllevar": mt.nllevar}
	if mt.llevar != nil && mt.nllevar > desdeLlevar {
		res["llevar"] = mt.llevar[:]
	}
	mt.mu.Unlock()
	mt.o.mu.Lock()
	res["puntos"] = tallerMarcador(mt.o.Puntos)
	mt.o.mu.Unlock()
	return res
}

func tallerMarcador(p map[string]int) []string {
	type par struct {
		q string
		n int
	}
	var ps []par
	for q, n := range p {
		ps = append(ps, par{q, n})
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].n > ps[j].n || ps[i].n == ps[j].n && ps[i].q < ps[j].q })
	var out []string
	for i, x := range ps {
		if i >= 6 {
			break
		}
		out = append(out, fmt.Sprintf("%s %d", x.q, x.n))
	}
	return out
}

// tallerNombreSer: cómo se llama en el juego un ser del mundo.
func tallerNombreSer(especie string) string {
	switch {
	case especie == "nyx-avatar":
		return "Nyx"
	case strings.HasPrefix(especie, "abla-"):
		n := strings.TrimPrefix(especie, "abla-")
		if n != "" {
			n = strings.ToUpper(n[:1]) + n[1:]
		}
		return "Abla · " + n
	}
	return especie
}

// ---------- el objeto, tal como lo ve su script (paquete "mundo") ----------

// TallerObjeto: la pieza viva. En el código se llama mundo.Objeto.
type TallerObjeto struct {
	mt *tallerMotor
	v  *tallerPiezaViva
}

func (o *TallerObjeto) parte(nombre string) *tallerParteViva {
	return o.v.parte(nombre)
}

// Tiempo: segundos desde que la pieza despertó.
func (o *TallerObjeto) Tiempo() float64 { return time.Since(o.v.nacio).Seconds() }

// Azar: un número al azar entre 0 y 1.
func (o *TallerObjeto) Azar() float64 { return rand.Float64() }

// Nombre y Autor de la pieza.
func (o *TallerObjeto) Nombre() string { return o.v.pieza.Titulo }
func (o *TallerObjeto) Autor() string  { return o.v.pieza.Autor }

// Jugador: dónde estás (x, y de los pies, z) y a qué distancia de la pieza.
func (o *TallerObjeto) Jugador() (float64, float64, float64, float64) {
	o.mt.mu.Lock()
	j := o.mt.jug
	o.mt.mu.Unlock()
	p := o.v.pieza
	return j[0], j[1], j[2], math.Sqrt((j[0]-p.X)*(j[0]-p.X) + (j[1]-p.Y)*(j[1]-p.Y) + (j[2]-p.Z)*(j[2]-p.Z))
}

// Partes: los nombres de sus partes.
func (o *TallerObjeto) Partes() []string {
	var out []string
	for _, p := range o.v.partes {
		if p.nombre != "" {
			out = append(out, p.nombre)
		}
	}
	return out
}

// Mover: desplaza una parte desde donde se construyó ("" = toda la pieza).
func (o *TallerObjeto) Mover(parte string, x, y, z float64) {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	if p := o.parte(parte); p != nil {
		p.pos = [3]float64{x, y, z}
	}
}

// Rotar: gira una parte alrededor de su pivote (en radianes, ejes x, y, z).
func (o *TallerObjeto) Rotar(parte string, x, y, z float64) {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	if p := o.parte(parte); p != nil {
		p.rot = [3]float64{x, y, z}
	}
}

// Escalar: el tamaño de una parte (1 = como se construyó).
func (o *TallerObjeto) Escalar(parte string, k float64) {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	if p := o.parte(parte); p != nil && k > 0 {
		p.esc = math.Min(k, 50)
	}
}

// Mostrar: que una parte se vea (y se choque) o no.
func (o *TallerObjeto) Mostrar(parte string, si bool) {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	if p := o.parte(parte); p != nil {
		p.visible = si
	}
}

// Visible: si una parte se ve.
func (o *TallerObjeto) Visible(parte string) bool {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	if p := o.parte(parte); p != nil {
		return p.visible
	}
	return false
}

// Zona: a qué distancia cuenta como «entrar» (AlEntrar / AlSalir).
func (o *TallerObjeto) Zona(radio float64) {
	o.mt.mu.Lock()
	o.v.zona = math.Max(0.3, math.Min(radio, 60))
	o.mt.mu.Unlock()
}

// Sonar: un sonido suelto en la pieza (campana, nota, tambor, moneda,
// puerta, magia, victoria, golpe… o un archivo tuyo de taller/sonidos).
// tono: 1 normal, 2 una octava arriba; en «nota», 60 es el do central.
func (o *TallerObjeto) Sonar(sonido string, tono float64) { o.SonarEn("", sonido, tono) }

// SonarEn: lo mismo, desde una parte.
func (o *TallerObjeto) SonarEn(parte, sonido string, tono float64) {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	p := o.parte(parte)
	if p == nil {
		p = o.v.partes[0]
	}
	c := o.v.matrizDe(p).punto([3]float64{})
	o.mt.nsonido++
	o.mt.sonidos = append(o.mt.sonidos, tallerSonido{N: o.mt.nsonido, S: sonido, Tono: tono, X: c[0], Y: c[1], Z: c[2]})
	if len(o.mt.sonidos) > 64 {
		o.mt.sonidos = o.mt.sonidos[len(o.mt.sonidos)-64:]
	}
}

// Ambiente: un sonido que no para mientras estés cerca (agua, fuego,
// viento, zumbido, pajaros, magia, o «musica:60 62 64 67»). 0 lo apaga.
func (o *TallerObjeto) Ambiente(sonido string, volumen float64) { o.AmbienteEn("", sonido, volumen) }

// AmbienteEn: lo mismo, saliendo de una parte (y se mueve con ella).
func (o *TallerObjeto) AmbienteEn(parte, sonido string, volumen float64) {
	o.mt.mu.Lock()
	defer o.mt.mu.Unlock()
	o.v.ambiente[parte+"|"+sonido] = math.Max(0, math.Min(volumen, 1))
}

// Decir: la pieza dice algo (sale en la charla).
func (o *TallerObjeto) Decir(texto string) {
	go o.mt.o.Decir("«"+o.v.pieza.Titulo+"»", prefijo(texto, 200), "")
}

// Llevar: te lleva a un punto (en el sitio de la pieza): ascensores,
// trampillas, portales hacia arriba o hacia abajo.
func (o *TallerObjeto) Llevar(x, y, z float64) {
	p := o.v.pieza
	w := tallerTras(p.X, p.Y, p.Z).por(tallerRotY(p.Rumbo)).punto([3]float64{x, y, z})
	o.mt.mu.Lock()
	o.mt.llevar = &w
	o.mt.nllevar++
	o.mt.mu.Unlock()
}

// Puntos: suma puntos a alguien en el marcador del mundo.
func (o *TallerObjeto) Puntos(quien string, n int) {
	ob := o.mt.o
	ob.mu.Lock()
	if ob.Puntos == nil {
		ob.Puntos = map[string]int{}
	}
	ob.Puntos[quien] += n
	ob.mu.Unlock()
}

// PuntosDe: los puntos que lleva alguien.
func (o *TallerObjeto) PuntosDe(quien string) int {
	ob := o.mt.o
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return ob.Puntos[quien]
}

// Guardar / Valor: la memoria de la pieza (se guarda con el mundo).
func (o *TallerObjeto) Guardar(clave string, v float64) {
	ob := o.mt.o
	ob.mu.Lock()
	if o.v.pieza.Memoria == nil {
		o.v.pieza.Memoria = map[string]float64{}
	}
	if len(o.v.pieza.Memoria) < 256 {
		o.v.pieza.Memoria[clave] = v
	}
	ob.mu.Unlock()
}

func (o *TallerObjeto) Valor(clave string) float64 {
	ob := o.mt.o
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return o.v.pieza.Memoria[clave]
}

// ---------- los seres que juegan: van hacia lo que hay que tocar ----------

var (
	tallerMetasMu sync.Mutex
	tallerMetas   = map[int]tallerMeta{} // id del ser → adónde va
)

type tallerMeta struct {
	pieza string
	x, z  float64
	hasta time.Time
}

// TallerMetaDe: para el código de los seres (mundo.Meta): adónde quiere ir
// ahora (si está jugando a algo).
func TallerMetaDe(s *Ser) (float64, float64, bool) {
	tallerMetasMu.Lock()
	defer tallerMetasMu.Unlock()
	m, ok := tallerMetas[s.ID]
	if !ok || time.Now().After(m.hasta) {
		return 0, 0, false
	}
	return m.x, m.z, true
}

func init() {
	paqueteMundo["mundo/mundo"]["Objeto"] = reflect.ValueOf((*TallerObjeto)(nil))
	paqueteMundo["mundo/mundo"]["Meta"] = reflect.ValueOf(TallerMetaDe)
}

// Jugar: los seres cercanos a un juego van a por lo que hay que tocar.
func (mt *tallerMotor) Jugar(p *tallerPieza, segundos float64) int {
	v := mt.vivo(p)
	if v == nil || v.comp.prog.alTocar == nil {
		return 0
	}
	n := 0
	for _, s := range mt.o.m.SeresCerca(p.X, p.Z, 35) {
		tallerMetasMu.Lock()
		tallerMetas[s.ID] = tallerMeta{pieza: p.Base, x: p.X, z: p.Z, hasta: time.Now().Add(time.Duration(segundos * float64(time.Second)))}
		tallerMetasMu.Unlock()
		n++
	}
	tallerActualizarMetas(mt)
	return n
}

// tallerActualizarMetas: cada jugador va a la parte visible más cercana de
// su juego (las que ya se tocaron no cuentan).
func tallerActualizarMetas(mt *tallerMotor) {
	tallerMetasMu.Lock()
	if len(tallerMetas) == 0 {
		tallerMetasMu.Unlock()
		return
	}
	metas := map[int]tallerMeta{}
	for k, v := range tallerMetas {
		if time.Now().Before(v.hasta) {
			metas[k] = v
		}
	}
	tallerMetasMu.Unlock()
	mt.mu.Lock()
	jug := mt.jug
	mt.mu.Unlock()
	pos := map[int][2]float64{}
	for _, s := range mt.o.m.SeresCerca(jug[0], jug[2], 200) {
		pos[s.ID] = [2]float64{s.X, s.Z}
	}
	for id, me := range metas {
		mt.mu.Lock()
		v := mt.vivos[me.pieza]
		if v == nil {
			mt.mu.Unlock()
			delete(metas, id)
			continue
		}
		yo, ok := pos[id]
		if !ok {
			mt.mu.Unlock()
			continue
		}
		mejor := math.Inf(1)
		for _, pt := range v.partes {
			if pt.nombre == "" || !pt.visible {
				continue
			}
			c := v.matrizDe(pt).punto([3]float64{})
			if c[1]-v.pieza.Y > 2.4 {
				continue // no llega
			}
			if d := math.Hypot(c[0]-yo[0], c[2]-yo[1]); d < mejor {
				mejor, me.x, me.z = d, c[0], c[2]
			}
		}
		mt.mu.Unlock()
		if math.IsInf(mejor, 1) {
			delete(metas, id)
			continue
		}
		metas[id] = me
	}
	tallerMetasMu.Lock()
	tallerMetas = metas
	tallerMetasMu.Unlock()
}

// ---------- la ventana pide ----------

func (mt *tallerMotor) rutas(mux *http.ServeMux, suyo http.Handler, codigo string) {
	// los modelos de las partes («pieza~parte»); lo demás, como siempre
	mux.HandleFunc("/api/figura", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if !strings.Contains(id, "~") {
			suyo.ServeHTTP(w, r)
			return
		}
		c, solido, _, mats, ok := mt.mallaConMateriales(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		sol := make([]int, len(solido))
		for i, b := range solido {
			if b {
				sol[i] = 1
			}
		}
		res := map[string]any{"pos": c.Pos, "nor": c.Nor, "col": c.Col, "emi": c.Emi, "sol": sol}
		// los materiales: una lista y, por triángulo, cuál (-1 = color liso)
		var lista []string
		idx := map[string]int{}
		tm := make([]int, len(mats))
		hay := false
		for i, m := range mats {
			if m == "" {
				tm[i] = -1
				continue
			}
			k, ok := idx[m]
			if !ok {
				k = len(lista)
				idx[m] = k
				lista = append(lista, m)
			}
			tm[i], hay = k, true
		}
		if hay {
			res["mats"], res["tm"] = lista, tm
		}
		responder(w, res, nil)
	})
	mux.HandleFunc("/api/taller-fisica", func(w http.ResponseWriter, r *http.Request) {
		f := mt.Fisica(r.URL.Query().Get("m"))
		if f == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "max-age=600")
		responder(w, f, nil)
	})
	mux.HandleFunc("/api/taller-motor", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		num := func(k string) float64 { v, _ := strconv.ParseFloat(q.Get(k), 64); return v }
		if q.Get("x") != "" {
			usar := q.Get("usar") == "1" && r.Method == "POST" && tallerMismoCodigo(r, codigo)
			mt.Jugador(num("x"), num("y"), num("z"), usar)
		}
		responder(w, mt.Vista(int(num("s")), int(num("l"))), nil)
	})
	tallerRutaTextura(mux, mt.o)
	mux.HandleFunc("/api/taller-sonido", func(w http.ResponseWriter, r *http.Request) {
		n := filepath.Base(r.URL.Query().Get("f"))
		if n == "." || n == "/" || strings.HasPrefix(n, ".") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(mt.o.dir, "sonidos", n))
	})
}
