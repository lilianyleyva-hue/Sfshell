package main

// ============================================================
//  TALLER · LAS 36 IAs DEL MUNDO 3D
// ------------------------------------------------------------
//  36 IAs que viven DENTRO del mundo: tienen cuerpo, andan con física
//  por lo que hay (suben escaleras, bajan a sótanos), vuelan por el
//  vacío, se hablan, construyen, pintan y juegan. Como los 27 de Abla,
//  van por tribus, 9 en cada una:
//
//   · CONSTRUCTORAS: buscan en el modelo del mundo un sitio libre pegado
//     a lo que ya hay (unas prefieren crecer hacia arriba y otras hacia
//     abajo) y construyen allí.
//   · EXPLORADORAS: van a donde nadie ha ido. Si llegan lejos de todo,
//     tienden suelo para que el mundo siga (sin fin).
//   · ARTISTAS: van a lo que hay y lo pintan con sus manos, del color
//     que Nexo une a sus palabras (la sinestesia).
//   · JUGADORAS: van a por los premios (y ganan puntos); si no hay
//     juegos cerca, hacen uno.
//
//  Para construir no tienen plantillas: hablan con el modelo de lenguaje
//  (las palabras que han oído a Nyx, a Abla y a ti), imaginan con el
//  modelo del mundo qué saldría de cada frase, y dicen la que mejor sirve
//  para lo que quieren. Luego comparan lo imaginado con lo que salió, y
//  así el modelo se corrige y ellas ganan habilidad (como en las misiones
//  de Abla): con más habilidad imaginan más frases antes de elegir.
//
//  Se guardan en taller/ias3d.json.
// ============================================================

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const tallerNumIAs3D = 36

// de una en una: cada cuánto puede construir o pintar alguna
const tallerTurnoIAs = 12 * time.Second

var tallerTribus3D = []struct {
	Nombre string
	Color  [3]float64
}{
	{"constructora", [3]float64{0.95, 0.68, 0.22}},
	{"exploradora", [3]float64{0.3, 0.78, 0.95}},
	{"artista", [3]float64{0.9, 0.35, 0.8}},
	{"jugadora", [3]float64{0.4, 0.88, 0.4}},
}

type tallerIA3D struct {
	Nombre    string     `json:"nombre"`
	Tribu     int        `json:"tribu"`
	X         float64    `json:"x"`
	Y         float64    `json:"y"`
	Z         float64    `json:"z"`
	Rumbo     float64    `json:"rumbo"`
	Meta      [3]float64 `json:"meta"`
	Accion    string     `json:"accion"`   // ir, construir, explorar, pintar, jugar
	Haciendo  string     `json:"haciendo"` // para verlo
	Dice      string     `json:"dice,omitempty"`
	Habilidad float64    `json:"habilidad"` // 0..1: sube con la práctica
	Intentos  int        `json:"intentos"`
	Aciertos  int        `json:"aciertos"`
	Piezas    int        `json:"piezas"`
	Sube      int        `json:"sube"` // hacia dónde le gusta crecer: +1 arriba, -1 abajo
	Vuela     bool       `json:"vuela"`
	Objetivo  string     `json:"objetivo,omitempty"` // la pieza a la que va (artistas)

	vy, espera, diceT, charlaT float64
	ocupada                    bool
}

type tallerPoblado struct {
	mu     sync.Mutex
	o      *tallerObra
	ruta   string
	Seres  []*tallerIA3D `json:"seres"`
	Pausa  bool          `json:"pausa"`
	modelo *tallerModelo3D
	rng    *rand.Rand
	turno  time.Time // la última vez que una construyó o pintó (de una en una)
	reloj  float64
	juegos map[string]bool // pieza → si da puntos
}

func tallerNuevoPoblado(o *tallerObra) *tallerPoblado {
	p := &tallerPoblado{o: o, ruta: filepath.Join(o.dir, "ias3d.json"), modelo: tallerNuevoModelo3D(o.dir),
		rng: rand.New(&tallerAzarSeguro{s: rand.NewSource(time.Now().UnixNano()).(rand.Source64)}), juegos: map[string]bool{}}
	if d, err := os.ReadFile(p.ruta); err == nil {
		_ = json.Unmarshal(d, p)
	}
	usados := map[string]bool{}
	for _, s := range p.Seres {
		usados[s.Nombre] = true
	}
	for i := len(p.Seres); i < tallerNumIAs3D; i++ {
		nombre := ""
		for k := 0; nombre == "" || usados[nombre]; k++ {
			nombre = tallerNombreIA3D(i, k)
		}
		usados[nombre] = true
		ang := float64(i) / tallerNumIAs3D * 2 * math.Pi
		r := 8 + float64(i%9)*1.5
		sube := 1
		if i%2 == 1 {
			sube = -1
		}
		p.Seres = append(p.Seres, &tallerIA3D{Nombre: nombre, Tribu: i % 4, X: math.Cos(ang) * r, Z: math.Sin(ang) * r, Y: 1,
			Rumbo: ang, Sube: sube, Vuela: true, Accion: "ir", Haciendo: "acaba de nacer"})
	}
	for i, s := range p.Seres {
		s.Meta = [3]float64{s.X, s.Y, s.Z}
		s.espera = float64(i%9) * 0.7 // que no empiecen todas a la vez
	}
	return p
}

// tallerAzarSeguro: azar que pueden usar varias a la vez.
type tallerAzarSeguro struct {
	mu sync.Mutex
	s  rand.Source64
}

func (a *tallerAzarSeguro) Int63() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.s.Int63()
}

func (a *tallerAzarSeguro) Uint64() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.s.Uint64()
}

func (a *tallerAzarSeguro) Seed(x int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.s.Seed(x)
}

// tallerNombreIA3D: un nombre suyo (sílabas como las de Abla).
func tallerNombreIA3D(i, intento int) string {
	h := tallerHash(fmt.Sprintf("ia3d#%d#%d", i, intento))
	cs, vs := "ktsnmlrvzpdb", "aeiou"
	var b strings.Builder
	for k := 0; k < 2+int(h>>62)%2; k++ {
		b.WriteByte(cs[h%12])
		h /= 12
		b.WriteByte(vs[h%5])
		h /= 5
	}
	n := b.String()
	return strings.ToUpper(n[:1]) + n[1:]
}

func (p *tallerPoblado) Guardar() error {
	p.mu.Lock()
	d, err := json.MarshalIndent(p, "", " ")
	p.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := p.ruta + ".tmp"
	if err := os.WriteFile(tmp, d, 0o644); err != nil {
		return err
	}
	_ = p.modelo.Guardar()
	return os.Rename(tmp, p.ruta)
}

func (s *tallerIA3D) quien() string {
	return s.Nombre + " (" + tallerTribus3D[s.Tribu].Nombre + ")"
}

func (s *tallerIA3D) decir(texto string) {
	s.Dice, s.diceT = prefijo(texto, 140), 7
}

// ---------- el suelo de verdad (con la física de cada pieza) ----------

func tallerInversa(m tallerMat) (tallerMat, bool) {
	a, b, c, d, e, f, g, h, i := m[0], m[1], m[2], m[4], m[5], m[6], m[8], m[9], m[10]
	A, B, C := e*i-f*h, -(d*i - f*g), d*h-e*g
	det := a*A + b*B + c*C
	if math.Abs(det) < 1e-12 {
		return m, false
	}
	k := 1 / det
	r := tallerMat{A * k, -(b*i - c*h) * k, (b*f - c*e) * k, 0,
		B * k, (a*i - c*g) * k, -(a*f - c*d) * k, 0,
		C * k, -(a*h - b*g) * k, (a*e - b*d) * k, 0, 0, 0, 0, 1}
	tx, ty, tz := m[12], m[13], m[14]
	r[12] = -(r[0]*tx + r[4]*ty + r[8]*tz)
	r[13] = -(r[1]*tx + r[5]*ty + r[9]*tz)
	r[14] = -(r[2]*tx + r[6]*ty + r[10]*tz)
	return r, true
}

// tallerFoto: lo que hace falta de una pieza, copiado con el candado puesto
// (otra puede estar cambiándola a la vez).
type tallerFoto struct {
	p                    *tallerPieza
	ID, Base, Titulo     string
	X, Y, Z, Radio, Alto float64
}

func (o *tallerObra) fotos() []tallerFoto {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]tallerFoto, len(o.Piezas))
	for i, p := range o.Piezas {
		out[i] = tallerFoto{p, p.ID, p.Base, p.Titulo, p.X, p.Y, p.Z, p.Radio, p.Alto}
	}
	return out
}

// SueloEn: el suelo más alto bajo (x, y, z) (lo que se pisa, con la forma
// exacta de cada cosa si ya está viva; si no, lo alto de su caja).
func (mt *tallerMotor) SueloEn(x, y, z float64) (float64, bool) {
	piezas := mt.o.fotos()
	mt.o.mu.Lock()
	pasillos := mt.o.Pasillos
	mt.o.mu.Unlock()
	mejor, hay := math.Inf(-1), false
	if pasillos && y >= 0 {
		mejor, hay = 0, true
	}
	for _, p := range piezas {
		if math.Hypot(p.X-x, p.Z-z) > p.Radio+1.5 || p.Y-1.5 > y {
			continue
		}
		mt.mu.Lock()
		v := mt.vivos[p.Base]
		if v == nil || v.modelo != p.ID || v.rota != "" {
			mt.mu.Unlock()
			if top := p.Y + p.Alto; top <= y && top > mejor && math.Hypot(p.X-x, p.Z-z) < math.Max(0.8, p.Radio*0.6) {
				mejor, hay = top, true
			}
			continue
		}
		type parte struct {
			m      tallerMat
			modelo string
		}
		var ps []parte
		for _, pt := range v.partes {
			if pt.visible && !pt.vacia {
				ps = append(ps, parte{v.matrizDe(pt), v.modelo + "~" + pt.nombre})
			}
		}
		mt.mu.Unlock()
		for _, q := range ps {
			f := mt.Fisica(q.modelo)
			inv, ok := tallerInversa(q.m)
			if f == nil || !ok {
				continue
			}
			l := inv.punto([3]float64{x, y, z})
			celda := f.Celdas[fmt.Sprintf("%d,%d", int(math.Floor(l[0]/f.C)), int(math.Floor(l[2]/f.C)))]
			if len(celda) == 0 {
				continue
			}
			n := int(celda[0])
			for k := 1; k <= n && k < len(celda); k++ {
				w := q.m.punto([3]float64{l[0], float64(celda[k]), l[2]})
				if w[1] <= y && w[1] > mejor {
					mejor, hay = w[1], true
				}
			}
		}
	}
	return mejor, hay
}

// PartesParaTocar: dónde están las cosas que dan algo al tocarlas (premios,
// campanas…) cerca de (x, z).
func (mt *tallerMotor) PartesParaTocar(x, z, radio float64) [][3]float64 {
	var out [][3]float64
	for _, p := range mt.o.fotos() {
		if math.Hypot(p.X-x, p.Z-z) > radio {
			continue
		}
		v := mt.vivo(p.p)
		if v == nil || v.comp.prog.alTocar == nil {
			continue
		}
		mt.mu.Lock()
		for _, pt := range v.partes {
			if pt.nombre != "" && pt.visible {
				out = append(out, v.matrizDe(pt).punto([3]float64{}))
			}
		}
		mt.mu.Unlock()
	}
	return out
}

// TocarCerca: lo que toca una IA al pasar (premios, campanas…), aunque no
// esté abierta la ventana.
func (mt *tallerMotor) TocarCerca(quien string, x, y, z float64) {
	for _, p := range mt.o.fotos() {
		if math.Hypot(p.X-x, p.Z-z) > p.Radio+4 {
			continue
		}
		v := mt.vivo(p.p)
		if v == nil || v.comp.prog.alTocar == nil {
			continue
		}
		var tocar []string
		mt.mu.Lock()
		for _, pt := range v.partes {
			if pt.nombre == "" {
				continue
			}
			c := v.matrizDe(pt).punto([3]float64{})
			k := quien + "|" + pt.nombre
			cerca := pt.visible && math.Hypot(x-c[0], z-c[2]) < 0.9 && y < c[1]+0.6 && y+1.9 > c[1]-0.4
			if cerca && !v.tocando[k] {
				v.tocando[k] = true
				tocar = append(tocar, pt.nombre)
			} else if !cerca {
				delete(v.tocando, k)
			}
		}
		mt.mu.Unlock()
		for _, nombre := range tocar {
			nombre := nombre
			mt.llamar(v, func(o *TallerObjeto) { v.comp.prog.alTocar(o, quien, nombre) })
		}
	}
}

// ---------- vivir ----------

// Vivir: el bucle de las 36 (5 veces por segundo).
func (p *tallerPoblado) Vivir(parar <-chan struct{}) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-parar:
			_ = p.Guardar()
			return
		case <-t.C:
			func() {
				defer func() { _ = recover() }()
				p.paso(0.2)
			}()
		}
	}
}

func (p *tallerPoblado) paso(dt float64) {
	p.mu.Lock()
	p.reloj += dt
	reloj := p.reloj
	pausa := p.Pausa
	seres := append([]*tallerIA3D(nil), p.Seres...)
	p.mu.Unlock()
	o := p.o
	if int(reloj/dt)%50 == 0 { // cada 10 s: ver lo que hay
		p.modelo.Sincronizar(o.fotos())
	}
	if int(reloj/dt)%150 == 75 {
		go func() { _ = p.Guardar() }()
	}
	if pausa {
		return
	}
	for i, s := range seres {
		p.mover(s, dt)
		if int(reloj/dt)%5 == i%5 { // una vez por segundo cada una
			p.modelo.Visitar(s.X, s.Y, s.Z)
			o.motor.TocarCerca(s.Nombre, s.X, s.Y, s.Z)
		}
		p.mu.Lock()
		s.espera -= dt
		s.charlaT -= dt
		if s.diceT -= dt; s.diceT <= 0 {
			s.Dice = ""
		}
		decide := !s.ocupada && s.espera <= 0
		p.mu.Unlock()
		if decide {
			p.decidir(s)
		}
	}
	p.charlar(seres)
}

// mover: hacia su meta, andando por lo que hay o volando por el vacío.
func (p *tallerPoblado) mover(s *tallerIA3D, dt float64) {
	p.mu.Lock()
	x, y, z, meta, vuela, vy := s.X, s.Y, s.Z, s.Meta, s.Vuela, s.vy
	aPremio := s.Accion == "premio"
	vel := 1.7
	if s.Tribu == 1 {
		vel = 2.6
	}
	p.mu.Unlock()
	dx, dy, dz := meta[0]-x, meta[1]-y, meta[2]-z
	dh := math.Hypot(dx, dz)
	rumbo := math.NaN()
	if dh > 0.3 {
		rumbo = math.Atan2(dz, dx)
	}
	if vuela {
		d := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if d > 0.05 {
			k := math.Min(1, vel*1.4*dt/d)
			x, y, z = x+dx*k, y+dy*k, z+dz*k
		}
		// llegó: si hay donde posarse, se posa (si va a por un premio, no:
		// lo coge en el aire)
		if d < 1.2 && !aPremio {
			if g, ok := p.o.motor.SueloEn(x, y+0.6, z); ok && y-g < 3 {
				vuela, y, vy = false, g, 0
			}
		}
	} else {
		if dh > 0.05 {
			k := math.Min(1, vel*dt/dh)
			x, z = x+dx*k, z+dz*k
		}
		g, ok := p.o.motor.SueloEn(x, y+0.6, z)
		switch {
		case !ok || y-g > 30:
			vuela = true // debajo no hay nada: vuela
		case math.Abs(dy) > 2.5 && dh < 2:
			vuela = true // la meta está en otro nivel
		case y-g <= 0.6:
			y, vy = g, 0 // anda (y sube escalones)
		default:
			vy -= 18 * dt // cae
			y += vy * dt
			if y <= g {
				y, vy = g, 0
			}
		}
	}
	p.mu.Lock()
	s.X, s.Y, s.Z, s.Vuela, s.vy = x, y, z, vuela, vy
	if !math.IsNaN(rumbo) {
		s.Rumbo = rumbo
	}
	p.mu.Unlock()
}

// llego: si ya está en su meta.
func (s *tallerIA3D) llego() bool {
	return math.Hypot(s.Meta[0]-s.X, s.Meta[2]-s.Z) < 2 && math.Abs(s.Meta[1]-s.Y) < 3
}

// decidir: qué hace ahora (según su tribu y lo que sabe del mundo).
func (p *tallerPoblado) decidir(s *tallerIA3D) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rng := p.rng
	s.espera = 1.5 + rng.Float64()*2
	llego := s.llego()
	switch {
	case s.Accion == "construir" && llego,
		s.Accion == "explorar" && llego,
		s.Accion == "pintar" && llego,
		s.Accion == "jugar" && llego:
		if p.o.ocupadas() || time.Since(p.turno) < tallerTurnoIAs {
			s.espera = 2 + rng.Float64()*4 // espera su turno
			return
		}
		p.turno = time.Now()
		s.ocupada = true
		go p.actuar(s)
		return
	case !llego && s.espera > -25 && s.Accion != "ir":
		return // sigue yendo
	}
	// una meta nueva
	switch s.Tribu {
	case 0: // constructora: un sitio libre pegado a lo que hay
		if m, ok := p.modelo.Frontera(rng, s.X, s.Y, s.Z, 45, s.Sube); ok {
			s.Meta = m
		} else {
			s.Meta = [3]float64{s.X + (rng.Float64()-0.5)*16, s.Y + float64(s.Sube)*rng.Float64()*6, s.Z + (rng.Float64()-0.5)*16}
		}
		s.Accion, s.Haciendo = "construir", "va a construir"
	case 1: // exploradora: a donde nadie ha ido
		s.Meta = p.modelo.Novedad(rng, s.X, s.Y, s.Z, 50)
		s.Vuela = true
		s.Accion, s.Haciendo = "explorar", "explora"
	case 2: // artista: a algo que pintar
		if b, pos, ok := p.algoCerca(s, 45); ok {
			s.Objetivo, s.Meta = b, pos
			s.Accion, s.Haciendo = "pintar", "va a pintar «"+b+"»"
		} else {
			s.Meta = p.modelo.Novedad(rng, s.X, s.Y, s.Z, 25)
			s.Accion, s.Haciendo = "ir", "busca qué pintar"
		}
	case 3: // jugadora: a por un premio, o a hacer un juego
		p.mu.Unlock()
		premios := p.o.motor.PartesParaTocar(s.X, s.Z, 60)
		p.mu.Lock()
		if len(premios) > 0 {
			sort.Slice(premios, func(i, j int) bool {
				return math.Hypot(premios[i][0]-s.X, premios[i][2]-s.Z) < math.Hypot(premios[j][0]-s.X, premios[j][2]-s.Z)
			})
			c := premios[rng.Intn(min(3, len(premios)))]
			s.Meta = [3]float64{c[0], c[1] - 0.5, c[2]}
			s.Vuela = true // derecha a por él
			s.Accion, s.Haciendo = "premio", "va a por un premio"
		} else {
			s.Meta = [3]float64{s.X + (rng.Float64()-0.5)*12, s.Y, s.Z + (rng.Float64()-0.5)*12}
			s.Accion, s.Haciendo = "jugar", "va a hacer un juego"
		}
	}
	s.espera = 4 + rng.Float64()*6
}

// algoCerca: una pieza cerca (para las artistas), que no sea suya.
func (p *tallerPoblado) algoCerca(s *tallerIA3D, radio float64) (string, [3]float64, bool) {
	p.o.mu.Lock()
	defer p.o.mu.Unlock()
	var cand []*tallerPieza
	for _, q := range p.o.Piezas {
		if math.Hypot(q.X-s.X, q.Z-s.Z) < radio && q.Autor != "ia:"+s.Nombre {
			cand = append(cand, q)
		}
	}
	if len(cand) == 0 {
		return "", [3]float64{}, false
	}
	q := cand[p.rng.Intn(len(cand))]
	ang := p.rng.Float64() * 2 * math.Pi
	r := math.Max(1.5, q.Radio*0.6+1)
	return q.Base, [3]float64{q.X + math.Cos(ang)*r, q.Y + q.Alto + 0.5, q.Z + math.Sin(ang)*r}, true
}

// ocupadas: si ahora no les toca (pausa, estudio).
func (o *tallerObra) ocupadas() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pausa || o.estudiando
}

// ---------- actuar: imaginar, elegir, hacer y aprender ----------

func (p *tallerPoblado) actuar(s *tallerIA3D) {
	defer func() {
		_ = recover()
		p.mu.Lock()
		s.ocupada = false
		s.Accion = "ir"
		s.espera = 3 + p.rng.Float64()*5
		p.mu.Unlock()
	}()
	p.mu.Lock()
	tribu, accion, objetivo := s.Tribu, s.Accion, s.Objetivo
	x, y, z := s.X, s.Y, s.Z
	p.mu.Unlock()
	if accion == "pintar" {
		p.pintar(s, objetivo)
		return
	}
	// construir (constructoras, exploradoras lejos de todo, jugadoras)
	if accion == "explorar" && p.modelo.Solape([6]float64{x - 10, y - 4, z - 10, x + 10, y + 6, z + 10}) > 0 {
		p.mu.Lock()
		s.decir(p.hablar(s, 4))
		s.Haciendo = "mira lo que hay"
		p.mu.Unlock()
		return
	}
	p.construir(s, tribu, accion)
}

// palabrasQueSabe: las que pueden usar (las que han oído), con lo que
// hace cada una al construir (lo imaginan una vez y se acuerdan).
func (p *tallerPoblado) palabrasQueSabe() []string {
	var ws []string
	if tallerLengua != nil {
		for w, c := range tallerLengua.Vocabulario() {
			if c >= 2 && len([]rune(w)) >= 2 && !tallerVacias[w] {
				ws = append(ws, w)
			}
		}
	}
	if len(ws) < 8 {
		ws = append(ws, nexoPalabras(tallerInstruccion)...)
	}
	sort.Strings(ws)
	return ws
}

// candidatas: frases para elegir. Con el modelo de lenguaje, frases de
// verdad que tiran hacia lo que quieren; si no, palabras que han oído.
func (p *tallerPoblado) candidatas(s *tallerIA3D, deseo map[string]float64, n int) []string {
	var out []string
	if tallerLengua != nil {
		for i := 0; i < n; i++ {
			if f := tallerLengua.Decir(p.rng, deseo, "", 3); f != "" {
				out = append(out, f)
			}
		}
	}
	ws := p.palabrasQueSabe()
	mejores := nexoMejores(deseo, 12)
	for len(out) < n && len(ws) > 0 {
		k := 2 + p.rng.Intn(5)
		var f []string
		for j := 0; j < k; j++ {
			if len(mejores) > 0 && p.rng.Float64() < 0.5 {
				f = append(f, mejores[p.rng.Intn(len(mejores))])
			} else {
				f = append(f, ws[p.rng.Intn(len(ws))])
			}
		}
		out = append(out, strings.Join(f, " "))
	}
	return out
}

// efecto: lo que hace una palabra al construir (imaginada sola).
var tallerEfectos = struct {
	sync.Mutex
	m map[string][4]float64 // sube, ancho, formas, eventos
}{m: map[string][4]float64{}}

func (p *tallerPoblado) efecto(w string) [4]float64 {
	tallerEfectos.Lock()
	e, ok := tallerEfectos.m[w]
	tallerEfectos.Unlock()
	if ok {
		return e
	}
	t := *tallerNuevaTortuga(0, 0, [3]float64{0.8, 0.8, 0.8})
	f := p.modelo.Imaginar(t, w+" "+w)
	e = [4]float64{f.Caja[4] - f.Caja[1], math.Max(f.Caja[3]-f.Caja[0], f.Caja[5]-f.Caja[2]), float64(f.formas), float64(f.Eventos)}
	if !f.HayCaja {
		e = [4]float64{0, 0, 0, float64(f.Eventos)}
	}
	if g, ok := tallerGestoDe[w]; ok {
		switch g {
		case gSubir:
			e[0] += 3
		case gBajar:
			e[0] -= 3
		case gSuelo:
			e[1] += 4
		}
	}
	tallerEfectos.Lock()
	tallerEfectos.m[w] = e
	tallerEfectos.Unlock()
	return e
}

func (p *tallerPoblado) construir(s *tallerIA3D, tribu int, accion string) {
	o := p.o
	p.mu.Lock()
	x, y, z, meta, sube, hab := s.X, s.Y, s.Z, s.Meta, s.Sube, s.Habilidad
	nombre, quien := s.Nombre, s.quien()
	p.mu.Unlock()
	// lo que quieren: qué palabras sirven (según lo que hacen) y de qué
	// hablar (lo que hay alrededor)
	deseo := p.modelo.PalabrasCerca(x, y, z, 12)
	for _, w := range p.palabrasQueSabe() {
		e := p.efecto(w)
		switch {
		case tribu == 0:
			deseo[w] += math.Max(0, e[0]*float64(sube))*0.3 + e[2]*0.1
		case tribu == 1:
			deseo[w] += e[1] * 0.25
		case tribu == 3:
			deseo[w] += e[3] * 2
		}
	}
	// su constructor sale de donde está ella
	t := o.tortuga("ia:" + nombre)
	o.mu.Lock()
	if t.Frases == 0 {
		t.Color = tallerTribus3D[tribu].Color
	}
	t.X, t.Y, t.Z, t.Mano = x, y, z, ""
	t.Rumbo = math.Atan2(meta[2]-z, meta[0]-x) + (p.rng.Float64()-0.5)*0.6
	base := *t
	o.mu.Unlock()
	// imagina varias frases (más cuanta más habilidad) y elige la mejor
	n := 3 + int(hab*9)
	mejor, mejorV := "", math.Inf(-1)
	var mejorPred *tallerObraHecha
	for _, frase := range p.candidatas(s, deseo, n) {
		pred := p.modelo.Imaginar(base, frase)
		if !pred.HayCaja {
			continue
		}
		v := p.valorar(tribu, pred, meta, sube)
		if tallerLengua != nil {
			v += 0.15 * tallerLengua.LogProb(strings.Fields(frase))
		}
		if v > mejorV {
			mejor, mejorV, mejorPred = frase, v, pred
		}
	}
	if mejor == "" {
		return
	}
	resumen, pieza, _ := o.construirConHecho("ia:"+nombre, quien, mejor)
	err := p.modelo.Comparar(mejorPred, pieza, o)
	p.mu.Lock()
	s.decir(mejor)
	s.Intentos++
	// como en las misiones de Abla: se aprende siempre, más si sale bien
	if pieza != nil && err < 0.35 {
		s.Aciertos++
		s.Habilidad += (1 - s.Habilidad) * 0.12
	} else {
		s.Habilidad += (1 - s.Habilidad) * 0.06
	}
	s.Habilidad = math.Min(s.Habilidad, 0.98)
	if pieza != nil {
		s.Piezas++
		s.Haciendo = "ha construido «" + prefijo(mejor, 40) + "»"
	}
	p.mu.Unlock()
	if pieza != nil {
		o.Decir(quien, prefijo(mejor, 200)+"  ⟶ "+resumen, "")
	}
}

// valorar: cuánto sirve lo imaginado para lo que quiere cada tribu.
func (p *tallerPoblado) valorar(tribu int, f *tallerObraHecha, meta [3]float64, sube int) float64 {
	c := f.Caja
	solape := p.modelo.Solape(c)
	cx, cy, cz := (c[0]+c[3])/2, (c[1]+c[4])/2, (c[2]+c[5])/2
	switch tribu {
	case 0: // llegar a su sitio, y crecer hacia donde le gusta
		d := math.Sqrt((cx-meta[0])*(cx-meta[0]) + (cy-meta[1])*(cy-meta[1]) + (cz-meta[2])*(cz-meta[2]))
		crece := c[4] - f.y
		if sube < 0 {
			crece = f.y - c[1]
		}
		return -d/8 + 0.15*crece + 0.1*math.Min(float64(f.formas), 6) - 2*solape
	case 1: // un suelo ancho y bajo, para seguir
		area := (c[3] - c[0]) * (c[5] - c[2])
		return math.Log(1+area) - 0.2*(c[4]-c[1]) - 2*solape
	default: // un juego
		return 2*float64(f.Eventos) + 0.1*math.Min(float64(f.formas), 6) - solape
	}
}

// pintar: las artistas cogen algo y lo tiñen del color que Nexo une a sus
// palabras (o del que les sugieren), y a veces le cambian el material.
func (p *tallerPoblado) pintar(s *tallerIA3D, base string) {
	o := p.o
	q := o.Pieza(base)
	if q == nil {
		return
	}
	o.mu.Lock()
	titulo := q.Titulo
	o.mu.Unlock()
	p.mu.Lock()
	quien, nombre := s.quien(), s.Nombre
	p.mu.Unlock()
	ws := nexoPalabras(titulo)
	col, de := [3]float64{}, ""
	if o.nexo != nil {
		o.nexo.mu.Lock()
		for _, w := range ws {
			if _, gesto := tallerGestoDe[w]; gesto {
				continue
			}
			if c, ok := o.nexo.Color[w]; ok && c[3] >= 1 {
				col, de = [3]float64{c[0], c[1], c[2]}, w
				break
			}
		}
		o.nexo.mu.Unlock()
	}
	if de == "" {
		// sin color visto: el de la cosa (como lo hace el constructor)
		for _, w := range ws {
			if _, gesto := tallerGestoDe[w]; gesto || tallerColores[w] != [3]float64{} || len([]rune(w)) < 3 {
				continue
			}
			h := tallerHash(w)
			c := TallerHSV(float64((h>>16)%360)/360, 0.55, 0.85)
			col, de = [3]float64{c.R, c.G, c.B}, w
			break
		}
	}
	if de == "" {
		return
	}
	m := &tallerManos{o: o, autor: "ia:" + nombre, quien: quien}
	m.Tenir(base, col)
	que := "lo tiñe de " + nexoNombreColor(col) + " (por «" + de + "»)"
	// un material que conozcan (lo han oído) y que vaya con su palabra
	var mats []string
	for _, w := range p.palabrasQueSabe() {
		if tallerMateriales[w] {
			mats = append(mats, w)
		}
	}
	if len(mats) > 0 && p.rng.Float64() < 0.4 {
		mat := mats[int(tallerHash(de)%uint64(len(mats)))]
		m.Pintar(base, mat+"|"+tallerHex(col))
		que += " y lo hace de " + mat
	}
	p.mu.Lock()
	s.Intentos++
	s.Habilidad += (1 - s.Habilidad) * 0.06
	s.decir(que)
	s.Haciendo = "ha pintado «" + titulo + "»"
	p.mu.Unlock()
	o.Decir(quien, "coge «"+titulo+"» y "+que, "")
}

// hablar: lo que dice de lo que tiene alrededor (con p.mu cogido).
func (p *tallerPoblado) hablar(s *tallerIA3D, intentos int) string {
	cerca := p.modelo.PalabrasCerca(s.X, s.Y, s.Z, 10)
	if tallerLengua != nil {
		tema := ""
		if ms := nexoMejores(cerca, 1); len(ms) > 0 {
			tema = ms[0]
		}
		if f := tallerLengua.Decir(p.rng, cerca, tema, intentos); f != "" {
			return f
		}
	}
	ws := nexoMejores(cerca, 4)
	if len(ws) == 0 {
		return strings.Join(nexoPalabras(tallerInstruccion), " ")
	}
	return strings.Join(ws, " ")
}

// charlar: las que se encuentran se dicen algo (en su bocadillo).
func (p *tallerPoblado) charlar(seres []*tallerIA3D) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, a := range seres {
		if a.charlaT > 0 || a.ocupada || a.Dice != "" {
			continue
		}
		for _, b := range seres[i+1:] {
			if math.Hypot(a.X-b.X, a.Z-b.Z) < 5 && math.Abs(a.Y-b.Y) < 3 && b.Dice == "" {
				a.decir(p.hablar(a, 4))
				a.charlaT = 25 + p.rng.Float64()*20
				b.charlaT = 8
				return // una por paso, que no se pisen
			}
		}
	}
}

// ---------- para la ventana y la consola ----------

type tallerVistaIA struct {
	N string  `json:"n"`
	T int     `json:"t"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	R float64 `json:"r"`
	D string  `json:"d,omitempty"`
	H string  `json:"h,omitempty"`
}

// Vista: las que están cerca de (x, z).
func (p *tallerPoblado) Vista(x, z, radio float64) []tallerVistaIA {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []tallerVistaIA
	for _, s := range p.Seres {
		if math.Hypot(s.X-x, s.Z-z) < radio {
			out = append(out, tallerVistaIA{N: s.Nombre, T: s.Tribu, X: s.X, Y: s.Y, Z: s.Z, R: s.Rumbo, D: s.Dice, H: s.Haciendo})
		}
	}
	return out
}

func (p *tallerPoblado) Lista() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	seres := append([]*tallerIA3D(nil), p.Seres...)
	sort.SliceStable(seres, func(i, j int) bool { return seres[i].Tribu < seres[j].Tribu })
	var out []string
	for _, s := range seres {
		out = append(out, fmt.Sprintf("%-8s %-13s habilidad %3.0f%%  piezas %-3d en (%.0f, %+.0f, %.0f)  %s",
			s.Nombre, tallerTribus3D[s.Tribu].Nombre, s.Habilidad*100, s.Piezas, s.X, s.Y, s.Z, s.Haciendo))
	}
	return out
}

func (p *tallerPoblado) Una(nombre string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.Seres {
		if strings.EqualFold(s.Nombre, nombre) {
			sube := "hacia arriba"
			if s.Sube < 0 {
				sube = "hacia abajo"
			}
			return fmt.Sprintf("%s, %s. Habilidad %.0f%% (%d intentos, %d le salieron como los imaginó). Ha hecho %d piezas. Le gusta crecer %s.\nEstá en (%.1f, %.1f, %.1f)%s. Ahora: %s.\nLo último que dijo: «%s»",
				s.Nombre, tallerTribus3D[s.Tribu].Nombre, s.Habilidad*100, s.Intentos, s.Aciertos, s.Piezas, sube,
				s.X, s.Y, s.Z, map[bool]string{true: " volando", false: ""}[s.Vuela], s.Haciendo, s.Dice), true
		}
	}
	return "", false
}

// Traer: que vengan todas cerca de (x, y, z).
func (p *tallerPoblado) Traer(x, y, z float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, s := range p.Seres {
		ang := float64(i) / float64(len(p.Seres)) * 2 * math.Pi
		r := 4 + float64(i%6)
		s.Meta = [3]float64{x + math.Cos(ang)*r, y, z + math.Sin(ang)*r}
		s.Vuela, s.Accion, s.Haciendo, s.espera = true, "ir", "viene a verte", 8
	}
}

func (p *tallerPoblado) PonerPausa(si bool) {
	p.mu.Lock()
	p.Pausa = si
	p.mu.Unlock()
}
