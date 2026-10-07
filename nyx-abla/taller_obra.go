package main

// ============================================================
//  TALLER · OBRA — el mundo que construyen juntas
// ------------------------------------------------------------
//  Cada pieza es un programa en Go (objetos/<id>.go), su modelo 3D
//  para Blender (objetos/<id>.obj + .mtl) y un sitio en el mundo.
//  Cualquiera de las dos (o tú) puede cambiar una pieza: el cambio
//  es otra versión (<id>-v2, -v3…) y la anterior queda guardada.
//
//    taller/obra.json      qué piezas hay, dónde y quién las hizo
//    taller/charla.txt     todo lo que se han dicho
//    taller/blender/       el mundo entero como una escena .obj
// ============================================================

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
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

type tallerPieza struct {
	ID       string             `json:"id"`   // el modelo que se ve ahora (objetos/<id>.go)
	Base     string             `json:"base"` // el nombre de la pieza, sin versión
	Version  int                `json:"version"`
	Autor    string             `json:"autor"` // nyx, abla o tú
	Titulo   string             `json:"titulo"`
	De       string             `json:"de"` // el recuerdo del que salió
	X        float64            `json:"x"`
	Y        float64            `json:"y"`
	Z        float64            `json:"z"`
	Rumbo    float64            `json:"rumbo"`
	RotX     float64            `json:"rotX,omitempty"`     // inclinada (las manos)
	Historia []string           `json:"historia,omitempty"` // las versiones de antes (para deshacer)
	Ultima   int                `json:"ultima,omitempty"`   // el número de la última versión que se hizo
	Radio    float64            `json:"radio"`
	Alto     float64            `json:"alto"`
	Cambios  []string           `json:"cambios,omitempty"` // quién la cambió y cómo
	Memoria  map[string]float64 `json:"memoria,omitempty"` // lo que guardan sus scripts
	Cuando   time.Time          `json:"cuando"`
}

type tallerFrase struct {
	N      int       `json:"n"`
	Quien  string    `json:"quien"` // Nyx, Abla (o uno de sus seres), tú, taller
	Texto  string    `json:"texto"`
	Glosa  string    `json:"glosa,omitempty"`
	Cuando time.Time `json:"cuando"`
}

type tallerObra struct {
	mu     sync.Mutex
	m      *Mundo
	dir    string
	abla   *tallerAbla
	Piezas []*tallerPieza `json:"piezas"`
	Sig    int            `json:"sig"`
	Vistas []tallerVista  `json:"vistas"`           // los recuerdos de Abla (lo que vio)
	Puntos map[string]int `json:"puntos,omitempty"` // el marcador de los juegos
	// por dónde va construyendo cada una (sin límite hacia ningún lado)
	Tortugas map[string]*tallerTortuga `json:"tortugas,omitempty"`
	// Pasillos: si el fondo es el mundo de Nyx Mundo (sus pasillos y sitios).
	// Si no, es el vacío: todo lo que hay lo hacen ellas.
	Pasillos bool `json:"pasillos,omitempty"`
	// Estudio: la lista de vídeos que estudian (y lo que salió de cada uno)
	Estudio       tallerEstudio `json:"estudio"`
	Turno         int           `json:"turno"`
	charla        []tallerFrase
	nfrase        int
	version       int            // sube con cada cambio de la obra
	trozos        map[string]int // trozo "cx,cz" → versión en que cambió
	pausa         bool
	ritmo         time.Duration
	viendo        string // el vídeo que están viendo ahora
	avisarFn      func(string)
	motor         *tallerMotor
	motorVivo     bool   // la ventana está abierta y el motor corre
	instruida     bool   // ya se les dio la instrucción en esta sesión
	codigoVentana string // el código que pide el servidor del mundo
	estudiando    bool   // perfeccionan con un vídeo: los turnos de siempre esperan
	ultimoNyx     string
	ultimoAbla    string
}

func tallerNuevaObra(m *Mundo) *tallerObra {
	o := &tallerObra{m: m, dir: filepath.Join(m.dir, "taller"), trozos: map[string]int{}, ritmo: 9 * time.Second}
	_ = os.MkdirAll(filepath.Join(o.dir, "blender"), 0o755)
	_ = os.MkdirAll(filepath.Join(m.dir, "objetos"), 0o755)
	if d, err := os.ReadFile(filepath.Join(o.dir, "obra.json")); err == nil {
		_ = json.Unmarshal(d, o)
	}
	if o.Sig == 0 {
		o.Sig = 1
	}
	o.abla = tallerNuevaAbla(filepath.Join(m.dir, "abla"))
	tallerFormasDir = filepath.Join(o.dir, "formas")
	o.motor = tallerNuevoMotor(o)
	// lo último que se dijeron, para seguir la conversación
	if f, err := os.Open(filepath.Join(o.dir, "charla.txt")); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<16), 1<<20)
		var ls []string
		for sc.Scan() {
			ls = append(ls, sc.Text())
		}
		f.Close()
		if len(ls) > 40 {
			ls = ls[len(ls)-40:]
		}
		for _, l := range ls {
			if i := strings.Index(l, " | "); i > 0 {
				if j := strings.Index(l[i+3:], ": "); j > 0 {
					o.nfrase++
					o.charla = append(o.charla, tallerFrase{N: o.nfrase, Quien: l[i+3 : i+3+j], Texto: l[i+3+j+2:]})
				}
			}
		}
	}
	return o
}

func (o *tallerObra) Guardar() error {
	o.mu.Lock()
	d, err := json.MarshalIndent(o, "", " ")
	o.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := filepath.Join(o.dir, "obra.json.tmp")
	if err := os.WriteFile(tmp, d, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(o.dir, "obra.json"))
}

// Decir: una frase en la charla (y en taller/charla.txt).
func (o *tallerObra) Decir(quien, texto, glosa string) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return
	}
	o.mu.Lock()
	o.nfrase++
	f := tallerFrase{N: o.nfrase, Quien: quien, Texto: texto, Glosa: glosa, Cuando: time.Now()}
	o.charla = append(o.charla, f)
	if len(o.charla) > 300 {
		o.charla = o.charla[len(o.charla)-300:]
	}
	avisar := o.avisarFn
	o.mu.Unlock()
	linea := texto
	if glosa != "" {
		linea += "  («" + glosa + "»)"
	}
	if fh, err := os.OpenFile(filepath.Join(o.dir, "charla.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintf(fh, "%s | %s: %s\n", f.Cuando.Format("2006-01-02 15:04:05"), quien, strings.ReplaceAll(linea, "\n", " "))
		fh.Close()
	}
	if avisar != nil {
		avisar(quien + ": " + linea)
	}
}

func (o *tallerObra) Charla(n int) []tallerFrase {
	o.mu.Lock()
	defer o.mu.Unlock()
	if n > len(o.charla) {
		n = len(o.charla)
	}
	return append([]tallerFrase(nil), o.charla[len(o.charla)-n:]...)
}

// ---------- las piezas ----------

var tallerReNoNombre = regexp.MustCompile(`[^a-z0-9]+`)

func tallerSlug(s string) string {
	s = strings.ToLower(strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u").Replace(s))
	s = strings.Trim(tallerReNoNombre.ReplaceAllString(s, "-"), "-")
	if len(s) > 24 {
		s = strings.Trim(s[:24], "-")
	}
	if s == "" {
		s = "pieza"
	}
	return s
}

func (o *tallerObra) Pieza(id string) *tallerPieza {
	o.mu.Lock()
	defer o.mu.Unlock()
	id = strings.ToLower(strings.TrimSpace(id))
	for _, p := range o.Piezas {
		if p.ID == id || p.Base == id {
			return p
		}
	}
	if n, err := strconv.Atoi(id); err == nil && n >= 1 && n <= len(o.Piezas) {
		return o.Piezas[n-1]
	}
	return nil
}

// compilarYGuardar: escribe el código, lo compila encerrado (como todo lo
// de Nyx Mundo, con sus partes y scripts) y guarda el modelo para Blender.
func (o *tallerObra) compilarYGuardar(id, codigo string) (*Cuerpo, error) {
	comp, err := tallerCompilar(codigo)
	if err != nil {
		return nil, err
	}
	ruta := filepath.Join(o.m.dir, "objetos", id)
	if err := os.WriteFile(ruta+".go", []byte(codigo), 0o644); err != nil {
		return nil, err
	}
	_ = o.tallerGuardarOBJ(comp.entero, comp.matTri, ruta, id)
	return comp.entero, nil
}

// Crear: una pieza nueva en el mundo. Si x, z son NaN, busca un sitio
// cerca de quien camina.
func (o *tallerObra) Crear(autor, titulo, de, codigo string, x, z float64) (*tallerPieza, error) {
	o.mu.Lock()
	n := o.Sig
	o.Sig++
	o.mu.Unlock()
	base := fmt.Sprintf("%s-%s-%d", tallerSlug(autor), tallerSlug(titulo), n)
	c, err := o.compilarYGuardar(base, codigo)
	if err != nil {
		return nil, err
	}
	_, alto := medirCuerpo(c)
	radio := tallerRadio(c)
	if math.IsNaN(x) || math.IsNaN(z) {
		x, z = o.sitioLibre(radio)
	}
	p := &tallerPieza{ID: base, Base: base, Version: 1, Autor: autor, Titulo: titulo, De: de,
		X: x, Z: z, Rumbo: rand.Float64() * 2 * math.Pi, Radio: radio, Alto: alto, Cuando: time.Now()}
	p.Y = o.suelo(x, z)
	o.mu.Lock()
	o.Piezas = append(o.Piezas, p)
	o.cambio(p.X, p.Z)
	o.mu.Unlock()
	_ = o.Guardar()
	return p, nil
}

// CrearEn: una pieza nueva justo en (x, y, z), sin girar: así la ponen las
// tortugas de Nyx y Abla (sin buscar suelo: puede estar en el aire o bajo
// tierra, sin límite).
func (o *tallerObra) CrearEn(autor, titulo, de, codigo string, x, y, z float64) (*tallerPieza, error) {
	o.mu.Lock()
	n := o.Sig
	o.Sig++
	o.mu.Unlock()
	base := fmt.Sprintf("%s-%s-%d", tallerSlug(autor), tallerSlug(titulo), n)
	c, err := o.compilarYGuardar(base, codigo)
	if err != nil {
		return nil, err
	}
	_, alto := medirCuerpo(c)
	p := &tallerPieza{ID: base, Base: base, Version: 1, Autor: autor, Titulo: titulo, De: de,
		X: x, Y: y, Z: z, Radio: tallerRadio(c), Alto: alto, Cuando: time.Now()}
	o.mu.Lock()
	o.Piezas = append(o.Piezas, p)
	o.cambio(p.X, p.Z)
	o.mu.Unlock()
	_ = o.Guardar()
	return p, nil
}

// MoverA: la pieza exactamente ahí (también la altura).
func (o *tallerObra) MoverA(p *tallerPieza, x, y, z, rumbo float64) {
	o.mu.Lock()
	o.cambio(p.X, p.Z)
	p.X, p.Y, p.Z, p.Rumbo = x, y, z, rumbo
	o.cambio(x, z)
	o.mu.Unlock()
	_ = o.Guardar()
}

// Cambiar: reescribe el código de una pieza con una función y guarda la
// nueva versión (si no compila, la pieza se queda como estaba).
func (o *tallerObra) Cambiar(p *tallerPieza, quien, que string, f func(codigo string) (string, error)) error {
	viejo, err := os.ReadFile(filepath.Join(o.m.dir, "objetos", p.ID+".go"))
	if err != nil {
		return fmt.Errorf("no encuentro el código de %s", p.ID)
	}
	nuevo, err := f(string(viejo))
	if err != nil {
		return err
	}
	// cada versión con su propio nombre, aunque se haya deshecho otra antes
	// (la ventana y Godot guardan los modelos por su nombre)
	o.mu.Lock()
	v := max(p.Version, p.Ultima) + 1
	o.mu.Unlock()
	id := fmt.Sprintf("%s-v%d", p.Base, v)
	nuevo = strings.Replace(nuevo, "\npackage objeto", fmt.Sprintf("\n// v%d: %s %s.\npackage objeto", v, quien, que), 1)
	c, err := o.compilarYGuardar(id, nuevo)
	if err != nil {
		return fmt.Errorf("el cambio no compila (%v): lo dejo como estaba", err)
	}
	_, alto := medirCuerpo(c)
	o.mu.Lock()
	p.Historia = append(p.Historia, p.ID)
	p.ID, p.Ultima, p.Alto, p.Radio = id, v, alto, tallerRadio(c)
	p.Version = len(p.Historia) + 1
	p.Cambios = append(p.Cambios, quien+": "+que)
	o.cambio(p.X, p.Z)
	o.mu.Unlock()
	return o.Guardar()
}

// Mover: la pieza a otro sitio.
func (o *tallerObra) Mover(p *tallerPieza, x, z, rumbo float64) {
	y := o.suelo(x, z)
	o.mu.Lock()
	o.cambio(p.X, p.Z)
	p.X, p.Y, p.Z, p.Rumbo = x, y, z, rumbo
	o.cambio(x, z)
	o.mu.Unlock()
	_ = o.Guardar()
}

// Quitar: la pieza sale del mundo (su código y su modelo se quedan).
func (o *tallerObra) Quitar(p *tallerPieza) {
	o.mu.Lock()
	for i, q := range o.Piezas {
		if q == p {
			o.Piezas = append(o.Piezas[:i], o.Piezas[i+1:]...)
			break
		}
	}
	o.cambio(p.X, p.Z)
	o.mu.Unlock()
	_ = o.Guardar()
}

// cambio: apunta qué trozo hay que volver a pedir (con o.mu tomado).
func (o *tallerObra) cambio(x, z float64) {
	o.version++
	o.trozos[tallerClave(x, z)] = o.version
}

func tallerClave(x, z float64) string {
	return fmt.Sprintf("%d,%d", int(math.Floor(x/ladoChunk)), int(math.Floor(z/ladoChunk)))
}

// suelo: la altura del suelo en (x, z).
func (o *tallerObra) suelo(x, z float64) float64 {
	if !o.Pasillos { // en el vacío no hay suelo: a la altura de quien camina
		o.motor.mu.Lock()
		y := o.motor.jug[1]
		o.motor.mu.Unlock()
		return y
	}
	ch := o.m.Construir(int(math.Floor(x/ladoChunk)), int(math.Floor(z/ladoChunk)))
	return ch.AlturaEn(x, z)
}

// sitioLibre: un sitio cerca de quien camina, lejos de paredes, del agua
// y de otras piezas.
func (o *tallerObra) sitioLibre(radio float64) (float64, float64) {
	o.m.seresMu.Lock()
	jx, jz := o.m.jx, o.m.jz
	o.m.seresMu.Unlock()
	if jx == 0 && jz == 0 {
		jx, jz = 8, 8
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	r := radio + 0.5
	for intento := 0; intento < 200; intento++ {
		d := 4 + float64(intento)*0.12 + rng.Float64()*8
		a := rng.Float64() * 2 * math.Pi
		x, z := jx+d*math.Cos(a), jz+d*math.Sin(a)
		if math.Hypot(x-jx, z-jz) < r+1.5 {
			continue
		}
		libre := true
		if o.Pasillos { // en el vacío no hay paredes ni agua de Nyx Mundo
			ch := o.m.Construir(int(math.Floor(x/ladoChunk)), int(math.Floor(z/ladoChunk)))
			o.m.mu.Lock()
			for _, p := range ch.Paredes {
				if distSegmento(x, z, p.X1, p.Z1, p.X2, p.Z2) < r {
					libre = false
					break
				}
			}
			for _, w := range ch.Agua {
				if x > w.X0-r && x < w.X1+r && z > w.Z0-r && z < w.Z1+r {
					libre = false
				}
			}
			o.m.mu.Unlock()
		}
		o.mu.Lock()
		for _, p := range o.Piezas {
			if math.Hypot(p.X-x, p.Z-z) < r+p.Radio+0.8 {
				libre = false
			}
		}
		o.mu.Unlock()
		if libre {
			return x, z
		}
	}
	return jx + 6, jz + 6
}

func tallerRadio(c *Cuerpo) float64 {
	return math.Max(0.2, math.Min(c.Ancho(), 30))
}

// ---------- Blender: exportar ----------

// tallerGuardarOBJ: el cuerpo como .obj + .mtl (Blender o Godot lo abren
// con sus colores y sus texturas; lo que brilla va como material emisivo).
func (o *tallerObra) tallerGuardarOBJ(c *Cuerpo, mats []string, ruta, nombre string) error {
	var b, mt strings.Builder
	fmt.Fprintf(&b, "# %s — hecho en el taller de Nyx y Abla\nmtllib %s.mtl\no %s\n", nombre, filepath.Base(ruta), nombre)
	usados := tallerEscribirMalla(&b, c, mats, 0, func(p [3]float64) [3]float64 { return p })
	o.tallerEscribirMateriales(&mt, usados, filepath.Dir(ruta))
	if err := os.WriteFile(ruta+".mtl", []byte(mt.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(ruta+".obj", []byte(b.String()), 0o644)
}

type tallerMatOBJ struct {
	col    [3]float64
	brilla bool
	tex    string // nombre de la textura ("" = color liso)
}

// tallerEscribirMalla: vértices (con color), coordenadas de textura,
// normales y caras agrupadas por material. base: cuántos vértices hay ya.
func tallerEscribirMalla(b *strings.Builder, c *Cuerpo, mats []string, base int, mover func([3]float64) [3]float64) map[string]tallerMatOBJ {
	n := len(c.Pos) / 3
	f := func(x float32) string { return strconv.FormatFloat(float64(x), 'f', 4, 32) }
	for i := 0; i < n; i++ {
		p := mover([3]float64{float64(c.Pos[i*3]), float64(c.Pos[i*3+1]), float64(c.Pos[i*3+2])})
		fmt.Fprintf(b, "v %s %s %s %s %s %s\n", f(float32(p[0])), f(float32(p[1])), f(float32(p[2])), f(c.Col[i*3]), f(c.Col[i*3+1]), f(c.Col[i*3+2]))
	}
	for i := 0; i < n; i++ {
		u, v := tallerUV([3]float64{float64(c.Pos[i*3]), float64(c.Pos[i*3+1]), float64(c.Pos[i*3+2])}, [3]float64{float64(c.Nor[i*3]), float64(c.Nor[i*3+1]), float64(c.Nor[i*3+2])})
		fmt.Fprintf(b, "vt %s %s\n", f(float32(u)), f(float32(v)))
	}
	for i := 0; i < n; i++ {
		q := mover([3]float64{float64(c.Nor[i*3]), float64(c.Nor[i*3+1]), float64(c.Nor[i*3+2])})
		o := mover([3]float64{})
		fmt.Fprintf(b, "vn %s %s %s\n", f(float32(q[0]-o[0])), f(float32(q[1]-o[1])), f(float32(q[2]-o[2])))
	}
	grupos := map[string][]int{}
	usados := map[string]tallerMatOBJ{}
	for t := 0; t+2 < n; t += 3 {
		if m := ""; t/3 < len(mats) && mats[t/3] != "" {
			m = mats[t/3]
			nombre := "t_" + tallerSlug(strings.ReplaceAll(m, "|", "-"))
			grupos[nombre] = append(grupos[nombre], t)
			usados[nombre] = tallerMatOBJ{col: [3]float64{1, 1, 1}, tex: m}
			continue
		}
		q := func(v float32) int { return int(math.Round(float64(v) * 15)) }
		brilla := 0
		if c.Emi[t] >= 4 {
			brilla = 1
		}
		nombre := fmt.Sprintf("c%x%x%x_%d", q(c.Col[t*3]), q(c.Col[t*3+1]), q(c.Col[t*3+2]), brilla)
		grupos[nombre] = append(grupos[nombre], t)
		usados[nombre] = tallerMatOBJ{col: [3]float64{float64(q(c.Col[t*3])) / 15, float64(q(c.Col[t*3+1])) / 15, float64(q(c.Col[t*3+2])) / 15}, brilla: brilla == 1}
	}
	nombres := make([]string, 0, len(grupos))
	for k := range grupos {
		nombres = append(nombres, k)
	}
	sort.Strings(nombres)
	for _, k := range nombres {
		fmt.Fprintf(b, "usemtl %s\n", k)
		for _, t := range grupos[k] {
			a := base + t + 1
			fmt.Fprintf(b, "f %d/%d/%d %d/%d/%d %d/%d/%d\n", a, a, a, a+1, a+1, a+1, a+2, a+2, a+2)
		}
	}
	return usados
}

// tallerEscribirMateriales: el .mtl (y las texturas como .png al lado).
func (o *tallerObra) tallerEscribirMateriales(mt *strings.Builder, usados map[string]tallerMatOBJ, dir string) {
	nombres := make([]string, 0, len(usados))
	for k := range usados {
		nombres = append(nombres, k)
	}
	sort.Strings(nombres)
	for _, k := range nombres {
		c := usados[k]
		fmt.Fprintf(mt, "newmtl %s\nKd %.3f %.3f %.3f\nKa 0 0 0\nKs 0.05 0.05 0.05\nNs 20\nd 1\nillum 2\n", k, c.col[0], c.col[1], c.col[2])
		if c.brilla {
			fmt.Fprintf(mt, "Ke %.3f %.3f %.3f\n", c.col[0], c.col[1], c.col[2])
		}
		if c.tex != "" {
			archivo := k + ".png"
			if d, err := tallerTexturaPNG(o, c.tex); err == nil && os.WriteFile(filepath.Join(dir, archivo), d, 0o644) == nil {
				fmt.Fprintf(mt, "map_Kd %s\n", archivo)
			}
		}
		mt.WriteString("\n")
	}
}

// ExportarEscena: todas las piezas, cada una en su sitio, en un solo
// archivo para abrir en Blender (taller/blender/mundo.obj).
func (o *tallerObra) ExportarEscena() (string, int, error) {
	o.mu.Lock()
	piezas := append([]*tallerPieza(nil), o.Piezas...)
	o.mu.Unlock()
	var b, mt strings.Builder
	b.WriteString("# El mundo de Nyx y Abla: cada pieza en su sitio (y hacia arriba es +Y)\nmtllib mundo.mtl\n")
	todos := map[string]tallerMatOBJ{}
	base := 0
	for _, p := range piezas {
		d, err := os.ReadFile(filepath.Join(o.m.dir, "objetos", p.ID+".go"))
		if err != nil {
			continue
		}
		comp, err := tallerCompilar(string(d))
		if err != nil {
			continue
		}
		c := comp.entero
		M := tallerTras(p.X, p.Y, p.Z).por(tallerRotY(p.Rumbo)).por(tallerRotX(p.RotX))
		fmt.Fprintf(&b, "o %s\n", p.ID)
		usados := tallerEscribirMalla(&b, c, comp.matTri, base, M.punto)
		for k, v := range usados {
			todos[k] = v
		}
		base += len(c.Pos) / 3
	}
	dir := filepath.Join(o.dir, "blender")
	o.tallerEscribirMateriales(&mt, todos, dir)
	if err := os.WriteFile(filepath.Join(dir, "mundo.mtl"), []byte(mt.String()), 0o644); err != nil {
		return "", 0, err
	}
	ruta := filepath.Join(dir, "mundo.obj")
	return ruta, len(piezas), os.WriteFile(ruta, []byte(b.String()), 0o644)
}

// ---------- Blender: importar ----------

// ImportarOBJ: lee un .obj (de Blender o de donde sea), lo deja de pie
// sobre el suelo con la altura que digas y lo convierte en una pieza con
// su propio código Go (que luego Nyx, Abla o tú podéis cambiar).
func (o *tallerObra) ImportarOBJ(ruta, nombre string, alto float64) (*tallerPieza, error) {
	tris, cols, err := tallerLeerOBJ(ruta)
	if err != nil {
		return nil, err
	}
	if len(tris) == 0 {
		return nil, errors.New("ese .obj no tiene caras")
	}
	// la caja que lo contiene
	mn := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	mx := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, t := range tris {
		for k := 0; k < 9; k++ {
			mn[k%3], mx[k%3] = math.Min(mn[k%3], t[k]), math.Max(mx[k%3], t[k])
		}
	}
	tam := [3]float64{mx[0] - mn[0], mx[1] - mn[1], mx[2] - mn[2]}
	if alto <= 0 {
		alto = math.Min(math.Max(tam[1], 0.5), 12)
	}
	k := 1.0
	if tam[1] > 1e-9 {
		k = alto / tam[1]
	}
	// demasiados triángulos para el mundo: se simplifica juntando vértices
	tris, cols = tallerSimplificar(tris, cols, maxTriangulos-200)
	// centrado en el suelo
	cx, cz := (mn[0]+mx[0])/2, (mn[2]+mx[2])/2
	caja := [6]float32{float32((mn[0] - cx) * k), 0, float32((mn[2] - cz) * k), float32(tam[0]*k + 1e-6), float32(tam[1]*k + 1e-6), float32(tam[2]*k + 1e-6)}
	datos := make([]byte, 24, 24+len(tris)*22)
	for i, v := range caja {
		binary.LittleEndian.PutUint32(datos[i*4:], math.Float32bits(v))
	}
	var t22 [22]byte
	for i, t := range tris {
		for j := 0; j < 9; j++ {
			v := (t[j] - [3]float64{cx, mn[1], cz}[j%3]) * k
			q := (v - float64(caja[j%3])) / float64(caja[3+j%3])
			binary.LittleEndian.PutUint16(t22[j*2:], uint16(math.Round(math.Max(0, math.Min(1, q))*65535)))
		}
		t22[18], t22[19], t22[20], t22[21] = cols[i][0], cols[i][1], cols[i][2], cols[i][3]
		datos = append(datos, t22[:]...)
	}
	enc := base64.StdEncoding.EncodeToString(datos)
	var b strings.Builder
	fmt.Fprintf(&b, "// %s — importado de %s (%d triángulos).\n// Puedes cambiarlo: lo de abajo de la malla es Go normal.\n", nombre, filepath.Base(ruta), len(tris))
	b.WriteString("package objeto\n\nimport \"mundo\"\n\n" + tallerParametros + "\nconst malla = `")
	for i := 0; i < len(enc); i += 100 {
		b.WriteString(enc[i:min(i+100, len(enc))] + "\n")
	}
	b.WriteString("`\n\nfunc Construir(c *mundo.Cuerpo) {\n\tc.MallaCodificada(malla)\n" + tallerCierre)
	return o.Crear("tú", nombre, filepath.Base(ruta), b.String(), math.NaN(), math.NaN())
}

// tallerLeerOBJ: triángulos (9 números) y su color (r, g, b, brillo).
func tallerLeerOBJ(ruta string) ([][9]float64, [][4]byte, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var vs [][3]float64
	var vc [][3]float64
	tieneColor := false
	mats := map[string][4]float64{}
	actual := [4]float64{0.75, 0.75, 0.75, 0}
	var tris [][9]float64
	var cols [][4]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<24)
	num := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	for sc.Scan() {
		c := strings.Fields(sc.Text())
		if len(c) == 0 {
			continue
		}
		switch c[0] {
		case "v":
			if len(c) < 4 {
				continue
			}
			vs = append(vs, [3]float64{num(c[1]), num(c[2]), num(c[3])})
			if len(c) >= 7 {
				vc = append(vc, [3]float64{num(c[4]), num(c[5]), num(c[6])})
				tieneColor = true
			} else {
				vc = append(vc, [3]float64{-1, -1, -1})
			}
		case "mtllib":
			for _, m := range c[1:] {
				for k, v := range tallerLeerMTL(filepath.Join(filepath.Dir(ruta), m)) {
					mats[k] = v
				}
			}
		case "usemtl":
			if len(c) > 1 {
				if m, ok := mats[c[1]]; ok {
					actual = m
				}
			}
		case "f":
			var idx []int
			for _, p := range c[1:] {
				i, err := strconv.Atoi(strings.SplitN(p, "/", 2)[0])
				if err != nil {
					continue
				}
				if i < 0 {
					i = len(vs) + i + 1
				}
				if i >= 1 && i <= len(vs) {
					idx = append(idx, i-1)
				}
			}
			for k := 1; k+1 < len(idx); k++ { // en abanico
				a, b, d := vs[idx[0]], vs[idx[k]], vs[idx[k+1]]
				tris = append(tris, [9]float64{a[0], a[1], a[2], b[0], b[1], b[2], d[0], d[1], d[2]})
				col := actual
				if tieneColor && vc[idx[0]][0] >= 0 {
					for j := 0; j < 3; j++ {
						col[j] = (vc[idx[0]][j] + vc[idx[k]][j] + vc[idx[k+1]][j]) / 3
					}
				}
				b8 := func(x float64) byte { return byte(math.Round(math.Max(0, math.Min(1, x)) * 255)) }
				var br byte
				if col[3] > 0 {
					br = 1
				}
				cols = append(cols, [4]byte{b8(col[0]), b8(col[1]), b8(col[2]), br})
			}
		}
	}
	return tris, cols, sc.Err()
}

// tallerLeerMTL: el color de cada material (y si brilla).
func tallerLeerMTL(ruta string) map[string][4]float64 {
	out := map[string][4]float64{}
	f, err := os.Open(ruta)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	nombre := ""
	for sc.Scan() {
		c := strings.Fields(sc.Text())
		if len(c) == 0 {
			continue
		}
		num := func(i int) float64 {
			if i >= len(c) {
				return 0
			}
			v, _ := strconv.ParseFloat(c[i], 64)
			return v
		}
		switch c[0] {
		case "newmtl":
			if len(c) > 1 {
				nombre = c[1]
				out[nombre] = [4]float64{0.75, 0.75, 0.75, 0}
			}
		case "Kd":
			m := out[nombre]
			m[0], m[1], m[2] = num(1), num(2), num(3)
			out[nombre] = m
		case "Ke":
			if num(1)+num(2)+num(3) > 0.05 {
				m := out[nombre]
				m[3] = 1
				out[nombre] = m
			}
		}
	}
	return out
}

// tallerSimplificar: si hay más triángulos de los que caben, junta los
// vértices cercanos (en una rejilla cada vez más gruesa) y quita los
// triángulos que se aplastan.
func tallerSimplificar(tris [][9]float64, cols [][4]byte, maximo int) ([][9]float64, [][4]byte) {
	if len(tris) <= maximo {
		return tris, cols
	}
	mn, mx := math.Inf(1), math.Inf(-1)
	for _, t := range tris {
		for _, v := range t {
			mn, mx = math.Min(mn, v), math.Max(mx, v)
		}
	}
	for celdas := 512.0; celdas >= 8; celdas *= 0.8 {
		paso := (mx - mn) / celdas
		var nt [][9]float64
		var nc [][4]byte
		vistos := map[[9]int64]bool{}
		for i, t := range tris {
			var q [9]int64
			var r [9]float64
			for k := 0; k < 9; k++ {
				q[k] = int64(math.Round((t[k] - mn) / paso))
				r[k] = mn + float64(q[k])*paso
			}
			a, b, d := [3]int64{q[0], q[1], q[2]}, [3]int64{q[3], q[4], q[5]}, [3]int64{q[6], q[7], q[8]}
			if a == b || b == d || a == d || vistos[q] {
				continue
			}
			vistos[q] = true
			nt = append(nt, r)
			nc = append(nc, cols[i])
		}
		if len(nt) <= maximo {
			return nt, nc
		}
	}
	return tris[:maximo], cols[:maximo]
}

// ---------- el esqueleto de todo código del taller ----------

// Cada pieza empieza con estos números: cambiándolos se cambia la pieza
// sin tocar su forma (es lo que hacen Nyx y Abla cuando retocan algo).
const tallerParametros = `// lo que se puede retocar sin tocar la forma
var escala = 1.00
var giro = 0.00
var tinte = mundo.RGB(1.00, 1.00, 1.00)
var fuerzaTinte = 0.00
`

// Y terminan así: lo que se añade va justo antes de «cambios».
const tallerCierre = `	// ── añadidos ──
	// ── cambios ──
	c.Tenir(tinte, fuerzaTinte)
	c.Girar(giro)
	c.Escalar(escala)
}
`

var (
	tallerReEscala = regexp.MustCompile(`(?m)^var escala = .*$`)
	tallerReGiro   = regexp.MustCompile(`(?m)^var giro = .*$`)
	tallerReTinte  = regexp.MustCompile(`(?m)^var tinte = .*$`)
	tallerReFuerza = regexp.MustCompile(`(?m)^var fuerzaTinte = .*$`)
)

// Retocar: cambia uno de los números de la pieza.
func tallerRetocar(codigo, que string, valores ...float64) (string, error) {
	if !strings.Contains(codigo, "var escala =") {
		return "", errors.New("esta pieza no tiene los números de retoque (var escala, giro, tinte…)")
	}
	switch que {
	case "escala":
		return tallerReEscala.ReplaceAllString(codigo, "var escala = "+f2(valores[0])), nil
	case "giro":
		return tallerReGiro.ReplaceAllString(codigo, "var giro = "+f2(valores[0])), nil
	case "tinte":
		codigo = tallerReTinte.ReplaceAllString(codigo, fmt.Sprintf("var tinte = mundo.RGB(%s, %s, %s)", f2(valores[0]), f2(valores[1]), f2(valores[2])))
		return tallerReFuerza.ReplaceAllString(codigo, "var fuerzaTinte = "+f2(valores[3])), nil
	}
	return "", fmt.Errorf("no sé retocar %q", que)
}

// tallerValor: el número que tiene ahora la pieza.
func tallerValor(codigo, nombre string) float64 {
	re := regexp.MustCompile(`(?m)^var ` + nombre + ` = ([-0-9.]+)`)
	if m := re.FindStringSubmatch(codigo); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	return 0
}

// Añadir: mete líneas de código dentro de Construir (en su propio bloque,
// para que sus variables no choquen con las de la forma).
func tallerAnadir(codigo, quien, lineas string) (string, error) {
	i := strings.LastIndex(codigo, "\t// ── cambios ──")
	if i < 0 {
		// código escrito a mano: se añade antes del final de Construir
		j := strings.Index(codigo, "func Construir(")
		if j < 0 {
			return "", errors.New("no encuentro func Construir")
		}
		i = strings.LastIndex(codigo, "}")
		if i < j {
			return "", errors.New("no encuentro el final de Construir")
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\t{ // %s\n\t\tc.Parte(\"\", 0, 0, 0)\n", quien)
	for _, l := range strings.Split(strings.TrimRight(lineas, "\n"), "\n") {
		b.WriteString("\t\t" + strings.TrimLeftFunc(l, unicode.IsSpace) + "\n")
	}
	b.WriteString("\t}\n")
	return codigo[:i] + b.String() + codigo[i:], nil
}
