package main

// ============================================================
//  TALLER · LAS 36 IAs — como Nyx y Abla, y pueden hacer de todo
// ------------------------------------------------------------
//  36 IAs más, sin cuerpo, como las dos principales: están en la charla,
//  contestan a lo último que se ha dicho (de Nyx, de Abla, de Nexo, de
//  otra de ellas) y lo que dicen se hace en el mundo. Cada una puede
//  hacer de todo:
//
//   · CONSTRUIR con palabras (el constructor: formas, materiales, manos,
//     eventos…), imaginando antes qué saldrá con el modelo del mundo.
//   · PROGRAMAR modelos 3D en Go: bucles, funciones, recursión, senos
//     (taller_programador.go).
//   · COMPONER música y grabarla como audio (.wav), con un instrumento
//     que la toca en el mundo (taller_musica.go).
//   · CREAR ENTIDADES: criaturas escritas en código que andan solas, te
//     siguen o huyen (y, con el mundo de Nyx de fondo, también entidades
//     de Nyx Mundo).
//   · USAR LAS MANOS sobre lo que ya hay: teñir, pintar de un material,
//     subir, girar, agrandar, copiar o escribirle código dentro.
//   · HACER JUEGOS: premios, puertas, trampolines, portales.
//   · EXPLORAR: llevar el mundo a donde no hay nada.
//
//  Cada una tiene un oficio que le gusta más (6 oficios, 6 de cada), pero
//  todas pueden todo. Lo que hacen bien les sale cada vez mejor (como en
//  las misiones de Abla) y lo que oyen y dicen se vuelve lo que les
//  interesa: así cada una acaba siendo distinta. Hablan con el modelo de
//  lenguaje del taller y piensan con la activación de Nexo.
//
//  Se guardan en taller/ias.json.
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

const tallerNumIAs = 36

var tallerAcciones = []string{"construir", "programar", "musica", "entidad", "manos", "juego", "explorar"}

var tallerNombreAccion = map[string]string{
	"construir": "construir con palabras", "programar": "programar modelos 3D", "musica": "componer música",
	"entidad": "crear entidades", "manos": "usar las manos", "juego": "hacer juegos", "explorar": "explorar",
}

// los oficios: lo que le gusta más a cada una (todas pueden todo)
var tallerOficios = []struct {
	Nombre string
	Gusta  string
}{
	{"constructora", "construir"},
	{"programadora", "programar"},
	{"música", "musica"},
	{"creadora de entidades", "entidad"},
	{"artista", "manos"},
	{"exploradora", "explorar"},
}

// palabras que llaman a cada cosa (si se habla de eso, apetece hacerlo)
var tallerLlaman = map[string][]string{
	"programar": {"codigo", "programa", "programar", "funcion", "bucle", "escribir", "numero", "calcular", "orden", "regla", "patron"},
	"musica":    {"musica", "cancion", "cantar", "canto", "sonido", "ritmo", "nota", "melodia", "oir", "escuchar", "voz", "campana", "tambor"},
	"entidad":   {"entidad", "criatura", "ser", "seres", "bicho", "animal", "vida", "vivo", "viva", "nacer", "alguien", "monstruo", "pajaro"},
	"manos":     {"coger", "tomar", "cambiar", "pintar", "color", "arreglar", "mover", "copiar", "tocar", "mano", "manos"},
	"juego":     {"juego", "jugar", "premio", "tesoro", "moneda", "puerta", "saltar", "portal", "ganar", "buscar"},
	"explorar":  {"lejos", "infinito", "explorar", "nuevo", "camino", "viaje", "ir", "mas", "todo", "mundo"},
}

type tallerIA struct {
	Nombre    string             `json:"nombre"`
	Oficio    int                `json:"oficio"`
	Foco      [3]float64         `json:"foco"`      // dónde está trabajando (no tiene cuerpo: es su atención)
	Habilidad map[string]float64 `json:"habilidad"` // por cosa que sabe hacer, 0..1
	Intereses map[string]float64 `json:"intereses"` // lo que le importa (de lo que oye y dice)
	Intentos  int                `json:"intentos"`
	Aciertos  int                `json:"aciertos"`
	Hechas    map[string]int     `json:"hechas"` // cuántas veces hizo cada cosa
	Sube      int                `json:"sube"`   // hacia dónde le gusta crecer: +1 arriba, -1 abajo
	Dijo      string             `json:"dijo,omitempty"`
	Ultima    string             `json:"ultima,omitempty"` // lo último que hizo
	Haciendo  string             `json:"haciendo,omitempty"`
	Turnos    int                `json:"turnos"`
}

type tallerPoblado struct {
	mu     sync.Mutex
	o      *tallerObra
	ruta   string
	Seres  []*tallerIA `json:"seres"`
	Pausa  bool        `json:"pausa"`
	Ritmo  float64     `json:"ritmo"` // segundos entre turno y turno
	modelo *tallerModelo3D
	rng    *rand.Rand
	sig    int
	ahora  bool // que hable una ya (desde la consola)
}

func tallerNuevoPoblado(o *tallerObra) *tallerPoblado {
	p := &tallerPoblado{o: o, ruta: filepath.Join(o.dir, "ias.json"), modelo: tallerNuevoModelo3D(o.dir), Ritmo: 12,
		rng: rand.New(&tallerAzarSeguro{s: rand.NewSource(time.Now().UnixNano()).(rand.Source64)})}
	if d, err := os.ReadFile(p.ruta); err == nil {
		_ = json.Unmarshal(d, p)
	}
	if p.Ritmo < 3 {
		p.Ritmo = 12
	}
	usados := map[string]bool{}
	for _, s := range p.Seres {
		usados[s.Nombre] = true
	}
	for i := len(p.Seres); i < tallerNumIAs; i++ {
		nombre := ""
		for k := 0; nombre == "" || usados[nombre]; k++ {
			nombre = tallerNombreIA(i, k)
		}
		usados[nombre] = true
		ang := float64(i) / tallerNumIAs * 2 * math.Pi
		sube := 1
		if i%2 == 1 {
			sube = -1
		}
		p.Seres = append(p.Seres, &tallerIA{Nombre: nombre, Oficio: i % len(tallerOficios), Sube: sube,
			Foco: [3]float64{math.Cos(ang) * 14, 0, math.Sin(ang) * 14}})
	}
	for _, s := range p.Seres {
		if s.Habilidad == nil {
			s.Habilidad = map[string]float64{}
		}
		if s.Intereses == nil {
			s.Intereses = map[string]float64{}
		}
		if s.Hechas == nil {
			s.Hechas = map[string]int{}
		}
	}
	return p
}

// tallerNombreIA: un nombre suyo (sílabas como las de Abla).
func tallerNombreIA(i, intento int) string {
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

func (s *tallerIA) quien() string {
	return s.Nombre + " (" + tallerOficios[s.Oficio].Nombre + ")"
}

// ---------- la conversación: lo último que se dijo ----------

// loUltimo: lo último que dijo alguien que no sea «yo» (nyx, abla, nexo o
// una de las 36), para contestarle.
func (o *tallerObra) loUltimo(yo string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	cands := []struct {
		quien, texto string
		en           int
	}{{"nyx", o.ultimoNyx, o.nyxEn}, {"abla", o.ultimoAbla, o.ablaEn}, {"nexo", o.ultimoNexo, o.nexoEn}, {o.quienIA, o.ultimoIA, o.iaEn}}
	mejor, en := "", -1
	for _, c := range cands {
		if c.texto != "" && c.quien != yo && c.en > en {
			mejor, en = c.texto, c.en
		}
	}
	return mejor
}

// ---------- vivir: se turnan ----------

// Vivir: cada «ritmo» segundos, una de las 36 contesta y hace algo.
func (p *tallerPoblado) Vivir(parar <-chan struct{}) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var desde float64
	for vuelta := 0; ; vuelta++ {
		select {
		case <-parar:
			_ = p.Guardar()
			return
		case <-t.C:
		}
		if vuelta%10 == 0 {
			p.modelo.Sincronizar(p.o.fotos())
		}
		if vuelta%60 == 30 {
			_ = p.Guardar()
		}
		p.mu.Lock()
		pausa, ritmo, ahora := p.Pausa, p.Ritmo, p.ahora
		p.ahora = false
		p.mu.Unlock()
		desde++
		if !ahora && (pausa || desde < ritmo || p.o.ocupadas()) {
			continue
		}
		desde = 0
		func() {
			defer func() { _ = recover() }()
			p.Turno(p.elegir(), "")
		}()
	}
}

// ocupadas: si ahora no les toca (pausa, estudio).
func (o *tallerObra) ocupadas() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pausa || o.estudiando
}

// elegir: a quién le toca. Por turno, pero entre unas pocas habla la que
// más tiene que decir de lo último que se dijo.
func (p *tallerPoblado) elegir() *tallerIA {
	ws := nexoPalabras(p.o.loUltimo(""))
	p.mu.Lock()
	defer p.mu.Unlock()
	var mejor *tallerIA
	mejorV := math.Inf(-1)
	for k := 0; k < 4; k++ {
		s := p.Seres[(p.sig+k)%len(p.Seres)]
		v := p.rng.Float64() - float64(k)*0.3
		for _, w := range ws {
			v += s.Intereses[w]
		}
		if v > mejorV {
			mejor, mejorV = s, v
		}
	}
	p.sig = (p.sig + 1) % len(p.Seres)
	return mejor
}

// Turno: una de ellas oye lo último, piensa, dice y hace. mensaje: si se
// lo dices tú (si no, lo último que se dijo).
func (p *tallerPoblado) Turno(s *tallerIA, mensaje string) string {
	o := p.o
	if mensaje == "" {
		mensaje = o.loUltimo("ia:" + s.Nombre)
	}
	if mensaje == "" {
		mensaje = tallerInstruccion
	}
	oye := nexoPalabras(mensaje)
	p.mu.Lock()
	s.Turnos++
	p.interesar(s, oye, 0.3)
	intereses := nexoMejores(s.Intereses, 8)
	p.mu.Unlock()
	// lo que le viene a la cabeza (la activación de Nexo, y lo suyo)
	act := map[string]float64{}
	if o.nexo != nil {
		act = o.nexo.Activacion(oye, intereses)
	}
	p.mu.Lock()
	for w, v := range s.Intereses {
		act[w] += v * 0.3
	}
	// qué le apetece hacer
	accion := p.queHacer(s, oye)
	hab := s.Habilidad[accion]
	p.mu.Unlock()
	tema := ""
	if ms := nexoMejores(act, 1); len(ms) > 0 {
		tema = ms[0]
	}
	dice := ""
	if tallerLengua != nil {
		dice = tallerLengua.Decir(p.rng, act, tema, 4+int(hab*8))
	}
	if dice == "" { // aún no sabe hablar: lo que le viene a la cabeza
		ws := nexoMejores(act, 5)
		if len(ws) == 0 {
			ws = oye
		}
		dice = strings.Join(ws[:min(6, len(ws))], " ")
	}
	if dice == "" {
		dice = tallerInstruccion
	}
	hecho, bien := p.hacer(s, accion, dice)
	p.mu.Lock()
	s.Dijo, s.Haciendo = dice, hecho
	s.Intentos++
	s.Hechas[accion]++
	s.Ultima = accion
	h := s.Habilidad[accion]
	if bien {
		s.Aciertos++
		h += (1 - h) * 0.12
	} else {
		h += (1 - h) * 0.06
	}
	s.Habilidad[accion] = math.Min(h, 0.98)
	p.interesar(s, nexoPalabras(dice), 0.15)
	quien := s.quien()
	p.mu.Unlock()
	o.Decir(quien, prefijo(dice, 220)+"  ⟶ "+hecho, "")
	o.mu.Lock()
	o.ultimoIA, o.quienIA = dice, "ia:"+s.Nombre
	o.habla++
	o.iaEn = o.habla
	o.mu.Unlock()
	return hecho
}

// interesar: lo que oye y dice se vuelve lo que le importa (con p.mu).
func (p *tallerPoblado) interesar(s *tallerIA, ws []string, k float64) {
	for _, w := range ws {
		s.Intereses[w] += k
	}
	if len(s.Intereses) > 200 {
		for w, v := range s.Intereses {
			if v *= 0.9; v < 0.05 {
				delete(s.Intereses, w)
			} else {
				s.Intereses[w] = v
			}
		}
	}
}

// queHacer: lo que le gusta, lo que le sale bien y de lo que se habla
// (con p.mu).
func (p *tallerPoblado) queHacer(s *tallerIA, oye []string) string {
	mejor, mejorV := "construir", math.Inf(-1)
	for _, a := range tallerAcciones {
		v := 1.0
		if tallerOficios[s.Oficio].Gusta == a {
			v = 3
		}
		v *= 0.5 + s.Habilidad[a]
		llama := 0.0
		for _, w := range oye {
			for _, l := range tallerLlaman[a] {
				if w == l {
					llama += 0.7
				}
			}
		}
		v += math.Min(llama, 1.5)
		if a == s.Ultima {
			v *= 0.6 // y que no haga siempre lo mismo
		}
		v *= 0.4 + p.rng.Float64()
		if v > mejorV {
			mejor, mejorV = a, v
		}
	}
	return mejor
}

// ---------- hacer ----------

// sinestesia: el color que Nexo une a una palabra (si lo ha visto).
func (o *tallerObra) sinestesia(w string) ([3]float64, bool) {
	if o.nexo == nil {
		return [3]float64{}, false
	}
	o.nexo.mu.Lock()
	defer o.nexo.mu.Unlock()
	if c, ok := o.nexo.Color[w]; ok && c[3] >= 1 {
		return [3]float64{c[0], c[1], c[2]}, true
	}
	return [3]float64{}, false
}

// sitio: dónde hacerlo (cerca de su foco, pegado a lo que hay; posado en
// un suelo si lo hay).
func (p *tallerPoblado) sitio(s *tallerIA, accion string) [3]float64 {
	p.mu.Lock()
	foco, sube := s.Foco, s.Sube
	p.mu.Unlock()
	var m [3]float64
	ok := false
	if accion == "explorar" {
		m, ok = p.modelo.Novedad(p.rng, foco[0], foco[1], foco[2], 45), true
	} else {
		m, ok = p.modelo.Frontera(p.rng, foco[0], foco[1], foco[2], 45, sube)
	}
	if !ok {
		ang := p.rng.Float64() * 2 * math.Pi
		m = [3]float64{foco[0] + math.Cos(ang)*8, foco[1], foco[2] + math.Sin(ang)*8}
	}
	if y, hay := p.o.motor.SueloEn(m[0], m[1]+3, m[2]); hay && m[1]-y < 6 {
		m[1] = y
	}
	p.modelo.Visitar(m[0], m[1], m[2]) // ya no es nuevo: aquí ha estado una
	p.mu.Lock()
	s.Foco = m
	p.mu.Unlock()
	return m
}

func (p *tallerPoblado) hacer(s *tallerIA, accion, dice string) (string, bool) {
	o := p.o
	autor, quien := "ia:"+s.Nombre, s.quien()
	palabras := tallerPalabras(dice)
	titulo := strings.Join(palabras[:min(3, len(palabras))], " ")
	switch accion {
	case "manos":
		if r, ok := p.manos(s, palabras); r != "" {
			return r, ok
		}
		accion = "construir" // no había nada que coger
	case "programar":
		x := p.sitio(s, accion)
		prog := ProgramarModelo(quien, palabras, o.sinestesia)
		if pz, err := o.CrearEn(autor, titulo, prefijo(dice, 120), prog.Codigo, x[0], x[1], x[2]); err == nil {
			return "programa " + prog.Que + " → " + pz.ID, true
		} else {
			return "programa " + prog.Que + ", pero no compila: " + err.Error(), false
		}
	case "musica":
		x := p.sitio(s, accion)
		comp := ComponerCon(s.Nombre, palabras)
		if _, err := comp.Grabar(filepath.Join(o.dir, "sonidos")); err != nil {
			return "quiere componer, pero no puede grabar: " + err.Error(), false
		}
		prog := ProgramarInstrumento(quien, comp.Archivo, palabras, o.sinestesia)
		if pz, err := o.CrearEn(autor, "música "+titulo, prefijo(dice, 120), prog.Codigo, x[0], x[1], x[2]); err == nil {
			return fmt.Sprintf("compone «%s» (%s) y %s → %s", comp.Archivo, comp.Que, prog.Que, pz.ID), true
		} else {
			return "compone «" + comp.Archivo + "», pero su instrumento no compila: " + err.Error(), false
		}
	case "entidad":
		x := p.sitio(s, accion)
		nombre := tallerNombreIA(int(tallerHash(dice)%100000), 7)
		prog := ProgramarEntidad(quien, nombre, palabras, o.sinestesia)
		pz, err := o.CrearEn(autor, nombre, prefijo(dice, 120), prog.Codigo, x[0], x[1], x[2])
		if err != nil {
			return "inventa a «" + nombre + "», pero su código no compila: " + err.Error(), false
		}
		r := fmt.Sprintf("da vida a «%s», %s → %s", nombre, prog.Que, pz.ID)
		// con el mundo de Nyx de fondo, también una entidad de Nyx Mundo
		o.mu.Lock()
		pasillos := o.Pasillos
		o.mu.Unlock()
		if pasillos {
			if e, err := o.m.CrearEspecie("crea una entidad " + dice); err == nil && e != nil {
				o.m.Aparecer(e, x[0], x[2])
				r += " · y en Nyx Mundo, la entidad «" + e.Nombre + "»"
			}
		}
		return r, true
	}
	// construir, juego y explorar: con palabras, imaginando antes
	return p.construir(s, accion, dice)
}

// construir: imagina varias frases (la suya y otras con las palabras que
// sabe) y dice la que mejor sirve para lo que quiere.
func (p *tallerPoblado) construir(s *tallerIA, accion, dice string) (string, bool) {
	o := p.o
	x := p.sitio(s, accion)
	p.mu.Lock()
	sube, hab, nombre, quien := s.Sube, s.Habilidad[accion], s.Nombre, s.quien()
	p.mu.Unlock()
	// qué palabras sirven para lo que quiere (lo que hace cada una, imaginado)
	deseo := p.modelo.PalabrasCerca(x[0], x[1], x[2], 12)
	for _, w := range p.palabrasQueSabe() {
		e := p.efecto(w)
		switch accion {
		case "explorar":
			deseo[w] += e[1] * 0.25
		case "juego":
			deseo[w] += e[3] * 2
		default:
			deseo[w] += math.Max(0, e[0]*float64(sube))*0.3 + e[2]*0.1
		}
	}
	t := o.tortuga("ia:" + nombre)
	o.mu.Lock()
	if t.Frases == 0 {
		h := tallerHash(nombre)
		c := TallerHSV(float64(h%360)/360, 0.5, 0.85)
		t.Color = [3]float64{c.R, c.G, c.B}
	}
	t.X, t.Y, t.Z, t.Mano = x[0], x[1], x[2], ""
	t.Rumbo = p.rng.Float64() * 2 * math.Pi
	base := *t
	o.mu.Unlock()
	tribu := map[string]int{"explorar": 1, "juego": 3}[accion]
	meta := [3]float64{x[0], x[1] + float64(sube)*4, x[2]}
	frases := append([]string{dice}, p.candidatas(deseo, 2+int(hab*8))...)
	mejor, mejorV := "", math.Inf(-1)
	var mejorPred *tallerObraHecha
	for i, frase := range frases {
		pred := p.modelo.Imaginar(base, frase)
		if !pred.HayCaja {
			continue
		}
		v := p.valorar(tribu, pred, meta, sube)
		if i == 0 {
			v += 0.5 // lo que ha dicho, si sirve, mejor
		}
		if tallerLengua != nil {
			v += 0.15 * tallerLengua.LogProb(strings.Fields(frase))
		}
		if v > mejorV {
			mejor, mejorV, mejorPred = frase, v, pred
		}
	}
	if mejor == "" {
		return "no se le ocurre qué hacer con eso", false
	}
	resumen, pieza, _ := o.construirConHecho("ia:"+nombre, quien, mejor)
	err := p.modelo.Comparar(mejorPred, pieza, o)
	if mejor != dice {
		resumen = "imagina otra cosa y dice «" + prefijo(mejor, 80) + "»: " + resumen
	}
	return resumen, pieza != nil && err < 0.35
}

// manos: coge algo de cerca (de otra) y lo cambia según lo que dice.
func (p *tallerPoblado) manos(s *tallerIA, palabras []string) (string, bool) {
	o := p.o
	p.mu.Lock()
	foco, nombre, quien := s.Foco, s.Nombre, s.quien()
	p.mu.Unlock()
	var cand []tallerFoto
	for _, f := range o.fotos() {
		if math.Hypot(f.X-foco[0], f.Z-foco[2]) < 50 && f.p.Autor != "ia:"+nombre {
			cand = append(cand, f)
		}
	}
	if len(cand) == 0 {
		return "", false
	}
	f := cand[p.rng.Intn(len(cand))]
	m := &tallerManos{o: o, autor: "ia:" + nombre, quien: quien}
	var hizo []string
	for _, w := range palabras {
		if len(hizo) >= 2 {
			break
		}
		b := tallerSinTildes.Replace(w)
		if sig, ok := tallerSignificado(b); ok {
			b = sig
		}
		if c, ok := tallerColores[b]; ok {
			m.Tenir(f.Base, c)
			hizo = append(hizo, "lo tiñe de "+b)
			continue
		}
		if tallerMateriales[b] {
			m.Pintar(f.Base, b)
			hizo = append(hizo, "lo hace de "+b)
			continue
		}
		switch tallerGestoDe[b] {
		case gSubir:
			m.Mover(f.Base, 0, 2, 0)
			hizo = append(hizo, "lo sube")
		case gBajar:
			m.Mover(f.Base, 0, -2, 0)
			hizo = append(hizo, "lo baja")
		case gGirar:
			m.Girar(f.Base, math.Pi/4)
			hizo = append(hizo, "lo gira")
		case gGrande:
			m.Escalar(f.Base, 1.4)
			hizo = append(hizo, "lo agranda")
		case gPequeno:
			m.Escalar(f.Base, 1/1.4)
			hizo = append(hizo, "lo achica")
		case gCopiar:
			if c := m.Copiar(f.Base, f.X+f.Radio+3, f.Y, f.Z); c != "" {
				hizo = append(hizo, "lo copia")
			}
		case gLuz, gEscribir:
			// le escribe código: una luz encima, del color de lo que dice
			c := tallerSembrar(palabras).color(o.sinestesia)
			m.Escribir(f.Base, fmt.Sprintf("c.Brilla(0, %s, 0, %s, %s) // lo escribió %s", f2(f.Alto+0.6), f2(0.2+0.1*float64(len(palabras)%4)), tallerCol(c), nombre))
			hizo = append(hizo, "le escribe una luz en su código")
		}
	}
	if len(hizo) == 0 {
		// del color que Nexo ve en sus palabras (sinestesia)
		for _, w := range nexoPalabras(f.Titulo) {
			if c, ok := o.sinestesia(w); ok {
				m.Tenir(f.Base, c)
				hizo = append(hizo, "lo tiñe de "+nexoNombreColor(c)+" (por «"+w+"»)")
				break
			}
		}
	}
	if len(hizo) == 0 {
		c := tallerSembrar(palabras).color(o.sinestesia)
		m.Tenir(f.Base, c)
		hizo = append(hizo, "lo tiñe de "+nexoNombreColor(c))
	}
	return "coge «" + f.Titulo + "» y " + strings.Join(hizo, " y "), true
}

// palabrasQueSabe: las que pueden usar (las que han oído).
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

// candidatas: otras frases para elegir (del modelo de lenguaje, tirando
// hacia lo que sirve; o con palabras que han oído).
func (p *tallerPoblado) candidatas(deseo map[string]float64, n int) []string {
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

// efecto: lo que hace una palabra al construir (imaginada sola, una vez).
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
	e = [4]float64{0, 0, 0, float64(f.Eventos)}
	if f.HayCaja {
		e = [4]float64{f.Caja[4] - f.Caja[1], math.Max(f.Caja[3]-f.Caja[0], f.Caja[5]-f.Caja[2]), float64(f.formas), float64(f.Eventos)}
	}
	switch tallerGestoDe[w] {
	case gSubir:
		e[0] += 3
	case gBajar:
		e[0] -= 3
	case gSuelo:
		e[1] += 4
	}
	tallerEfectos.Lock()
	tallerEfectos.m[w] = e
	tallerEfectos.Unlock()
	return e
}

// valorar: cuánto sirve lo imaginado (0 construir, 1 explorar, 3 juego).
func (p *tallerPoblado) valorar(tipo int, f *tallerObraHecha, meta [3]float64, sube int) float64 {
	c := f.Caja
	solape := p.modelo.Solape(c)
	cx, cy, cz := (c[0]+c[3])/2, (c[1]+c[4])/2, (c[2]+c[5])/2
	switch tipo {
	case 1: // un suelo ancho y bajo, para seguir
		area := (c[3] - c[0]) * (c[5] - c[2])
		return math.Log(1+area) - 0.2*(c[4]-c[1]) - 2*solape
	case 3: // un juego
		return 2*float64(f.Eventos) + 0.1*math.Min(float64(f.formas), 6) - solape
	default: // llegar a su sitio, y crecer hacia donde le gusta
		d := math.Sqrt((cx-meta[0])*(cx-meta[0]) + (cy-meta[1])*(cy-meta[1]) + (cz-meta[2])*(cz-meta[2]))
		crece := c[4] - f.y
		if sube < 0 {
			crece = f.y - c[1]
		}
		return -d/8 + 0.15*crece + 0.1*math.Min(float64(f.formas), 6) - 2*solape
	}
}

// ---------- el suelo de verdad (para posar lo que hacen) ----------

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

// SueloEn: el suelo más alto bajo (x, y, z), con la forma exacta de cada
// cosa si ya está viva (si no, lo alto de su caja).
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

// ---------- para la consola ----------

func (s *tallerIA) mejores(k int) string {
	ws := nexoMejores(s.Habilidad, k)
	var out []string
	for _, w := range ws {
		out = append(out, fmt.Sprintf("%s %.0f%%", tallerNombreAccion[w], s.Habilidad[w]*100))
	}
	return strings.Join(out, ", ")
}

func (p *tallerPoblado) Lista() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	seres := append([]*tallerIA(nil), p.Seres...)
	sort.SliceStable(seres, func(i, j int) bool { return seres[i].Oficio < seres[j].Oficio })
	var out []string
	for _, s := range seres {
		out = append(out, fmt.Sprintf("%-8s %-22s %3d turnos · mejor en: %s", s.Nombre, tallerOficios[s.Oficio].Nombre, s.Turnos, s.mejores(2)))
	}
	return out
}

func (p *tallerPoblado) buscar(nombre string) *tallerIA {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.Seres {
		if strings.EqualFold(s.Nombre, nombre) {
			return s
		}
	}
	return nil
}

func (p *tallerPoblado) Una(nombre string) (string, bool) {
	s := p.buscar(nombre)
	if s == nil {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var hechas []string
	for _, a := range tallerAcciones {
		if n := s.Hechas[a]; n > 0 {
			hechas = append(hechas, fmt.Sprintf("%s ×%d", tallerNombreAccion[a], n))
		}
	}
	sube := "hacia arriba"
	if s.Sube < 0 {
		sube = "hacia abajo"
	}
	return fmt.Sprintf("%s, %s. Le gusta crecer %s.\nHa hecho: %s.\nHabilidad: %s.\nLe interesa: %s.\nAhora trabaja en (%.0f, %.0f, %.0f). Lo último que dijo: «%s» ⟶ %s",
		s.Nombre, tallerOficios[s.Oficio].Nombre, sube, strings.Join(hechas, ", "), s.mejores(7),
		strings.Join(nexoMejores(s.Intereses, 10), ", "), s.Foco[0], s.Foco[1], s.Foco[2], s.Dijo, s.Haciendo), true
}

func (p *tallerPoblado) PonerPausa(si bool) {
	p.mu.Lock()
	p.Pausa = si
	p.mu.Unlock()
}

func (p *tallerPoblado) PonerRitmo(seg float64) {
	p.mu.Lock()
	p.Ritmo = math.Max(3, seg)
	p.mu.Unlock()
}

// Ya: que hable una en cuanto pueda.
func (p *tallerPoblado) Ya() {
	p.mu.Lock()
	p.ahora = true
	p.mu.Unlock()
}
