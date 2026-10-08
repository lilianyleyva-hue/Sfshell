package main

// ============================================================
//  TALLER · CHARLA — Nyx y Abla trabajando juntas
// ------------------------------------------------------------
//  Empiezan sin nada: solo sus idiomas y una instrucción, «crea un mundo
//  infinito con lo que sabes». Por turnos, sin parar:
//    · Nyx contesta (con su cabeza, que no se toca) a lo que dijo Abla;
//    · uno de los 27 de Abla contesta a lo que dijo Nyx (o se oye lo que
//      han estado pensando);
//    · y lo que dice cada una se convierte en mundo, palabra a palabra
//      (taller_lengua.go): formas, suelos, luces, música, movimiento, y
//      con las manos cogen y cambian lo que ya hay, suyo o de la otra.
//  Y ven vídeos: «ven <dirección>» se lo enseña a las dos a la vez.
// ============================================================

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var tallerRePrefijo = regexp.MustCompile(`^\[t=\d+\]\s*`)

// Trabajar: los turnos, hasta que se cierre el taller.
func (o *tallerObra) Trabajar(parar <-chan struct{}) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		o.mu.Lock()
		espera, pausa := o.ritmo, o.pausa
		o.mu.Unlock()
		select {
		case <-parar:
			return
		case <-time.After(espera):
		}
		o.mu.Lock()
		estudiando := o.estudiando
		o.mu.Unlock()
		if pausa || estudiando {
			o.recogerVistas() // aunque descansen, lo que Abla ve se apunta
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					o.Decir("taller", fmt.Sprintf("(algo falló en este turno: %v; sigo)", r), "")
				}
			}()
			o.Turnar(rng)
		}()
	}
}

// recogerVistas: lo que Abla terminó de mirar pasa a ser recuerdo suyo.
func (o *tallerObra) recogerVistas() []tallerVista {
	if !o.abla.Viva() {
		return nil
	}
	nuevas := o.abla.VistasNuevas()
	o.mu.Lock()
	for i := range nuevas {
		nuevas[i].Video = o.viendo
		o.Vistas = append(o.Vistas, nuevas[i])
	}
	if len(o.Vistas) > 2000 {
		o.Vistas = o.Vistas[len(o.Vistas)-2000:]
	}
	o.mu.Unlock()
	return nuevas
}

// serDeTribu: uno de los 27 de esa tribu.
func (o *tallerObra) serDeTribu(rng *rand.Rand, tribu int) (tallerSerAbla, bool) {
	var de []tallerSerAbla
	for _, s := range o.abla.Seres() {
		if s.Tribu == tribu {
			de = append(de, s)
		}
	}
	if len(de) == 0 {
		return tallerSerAbla{Nombre: "Abla", Tribu: tribu}, false
	}
	return de[rng.Intn(len(de))], true
}

// tortuga: por dónde va construyendo cada una.
func (o *tallerObra) tortuga(quien string) *tallerTortuga {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.Tortugas == nil {
		o.Tortugas = map[string]*tallerTortuga{}
	}
	t := o.Tortugas[quien]
	if t == nil {
		x := 0.0
		if quien == "abla" {
			x = 6
		}
		if quien == "nexo" {
			x = -6
		}
		t = tallerNuevaTortuga(x, 0, [3]float64{0.8, 0.8, 0.8})
		o.Tortugas[quien] = t
	}
	return t
}

// Turnar: un turno de trabajo. Se turnan: lo que dice una le llega a la
// otra, y cada una construye (y cambia cosas con las manos) con lo suyo.
func (o *tallerObra) Turnar(rng *rand.Rand) {
	nuevas := o.recogerVistas()
	o.mu.Lock()
	o.Turno++
	t := o.Turno
	o.mu.Unlock()
	o.darInstruccion()
	if len(nuevas) > 0 {
		v := nuevas[len(nuevas)-1]
		o.Decir("Abla", fmt.Sprintf("ha mirado %d foto(s) más. La última: %s", len(nuevas), v.Descripcion), v.Glosa())
	}
	// Nexo ve lo mismo que Abla, y piensa un poco cada turno
	if o.nexo != nil {
		for _, v := range nuevas {
			o.nexo.Ver(v)
		}
		for i := 0; i < 3; i++ {
			o.nexo.Pensar()
		}
	}
	switch {
	case o.nexo != nil && t%3 == 0:
		o.TurnoNexo("")
	case o.abla.Viva() && t%3 == 2:
		o.TurnoAbla(rng, "")
	default:
		o.TurnoNyx(rng, "")
	}
	_ = o.Guardar()
}

// darInstruccion: la única instrucción, una vez por sesión.
func (o *tallerObra) darInstruccion() {
	o.mu.Lock()
	ya := o.instruida
	o.instruida = true
	o.mu.Unlock()
	if ya {
		return
	}
	o.Decir("taller", "La única instrucción: «"+tallerInstruccion+"»", "")
	if o.abla.Viva() {
		_, _ = o.abla.pedir("@hablante Humano")
		o.abla.Ejecutar("decir todos " + tallerInstruccion)
	}
	o.mu.Lock()
	o.ultimoAbla = tallerInstruccion
	o.mu.Unlock()
}

// construirCon: lo que dijo alguien, hecho mundo (y lo que cambió con las manos).
func (o *tallerObra) construirCon(autor, quien, texto string) string {
	r, _, _ := o.construirConHecho(autor, quien, texto)
	return r
}

// construirConHecho: lo mismo, y devuelve también la pieza que salió (si
// salió) y lo que se hizo (para comparar con lo que se imaginó).
func (o *tallerObra) construirConHecho(autor, quien, texto string) (string, *tallerPieza, *tallerObraHecha) {
	palabras := tallerPalabras(texto)
	if len(palabras) == 0 {
		return "", nil, nil
	}
	var hecha *tallerPieza
	t := o.tortuga(autor)
	o.mu.Lock()
	tt := *t // se trabaja con una copia y se guarda al final
	o.mu.Unlock()
	f := tallerHablarYConstruir(&tt, palabras, texto, &tallerManos{o: o, autor: autor, quien: quien})
	resumen := tallerResumen(f)
	if f.formas > 0 {
		titulo := strings.Join(palabras[:min(3, len(palabras))], " ")
		f.esbozo.cabecera = fmt.Sprintf("// %s lo construyó diciendo: «%s»\n// (cada palabra es un gesto: ver taller_lengua.go). Es Go normal: se puede cambiar.\n", quien, prefijo(texto, 160))
		if p, err := o.CrearEn(autor, titulo, prefijo(texto, 120), f.esbozo.Codigo(), f.x, f.y, f.z); err != nil {
			resumen += " (no compiló: " + err.Error() + ")"
		} else {
			hecha = p
			resumen += " → " + p.ID
			// con premios: los seres que estén cerca van a jugar
			if f.Juego && o.motorEnMarcha() {
				go func(p *tallerPieza) {
					time.Sleep(2 * time.Second)
					o.motor.Jugar(p, 90)
				}(p)
			}
			// «llamar X»: desde ahora, esa palabra es esta forma
			if f.Nombrar != "" {
				if err := o.GuardarForma(f.Nombrar, p, quien); err == nil {
					resumen += " (guardada como forma «" + f.Nombrar + "»)"
				}
			}
		}
	}
	o.mu.Lock()
	*t = tt
	o.mu.Unlock()
	return resumen, hecha, f
}

// TurnoNyx: Nyx contesta (con su cabeza, que no se toca) a lo último que
// dijo Abla, o a la instrucción; y lo que dice, lo construye.
func (o *tallerObra) TurnoNyx(rng *rand.Rand, mensaje string) {
	if mensaje == "" {
		mensaje = o.loUltimo("nyx") // lo último que dijo otra (Abla, Nexo o una de las 36)
	}
	if mensaje == "" {
		mensaje = tallerInstruccion
	}
	dice := strings.TrimSpace(o.m.Responder(mensaje))
	if dice == "" {
		dice = tallerInstruccion
	}
	hecho := o.construirCon("nyx", "Nyx", dice)
	o.Decir("Nyx", prefijo(dice, 260)+"  ⟶ "+hecho, "")
	o.oyeNexo("Nyx", dice)
	o.mu.Lock()
	o.ultimoNyx = dice
	o.habla++
	o.nyxEn = o.habla
	o.mu.Unlock()
}

// TurnoAbla: uno de los 27 contesta a lo último que dijo Nyx (o se oye
// lo que han estado pensando), y lo que dice, lo construye.
func (o *tallerObra) TurnoAbla(rng *rand.Rand, mensaje string) {
	if !o.abla.Viva() {
		return
	}
	if mensaje == "" {
		mensaje = o.loUltimo("abla")
	}
	ser, _ := o.serDeTribu(rng, rng.Intn(3))
	quien, dijo, glosa := "", "", ""
	if mensaje != "" {
		quien, dijo, glosa = o.abla.Decir(fmt.Sprint(ser.ID+1), mensaje)
	}
	if quien == "" {
		// lo que han estado pensando (sin parar): una de las últimas ideas
		ps := o.abla.Pensamientos()
		for i := len(ps) - 1; i >= 0 && i >= len(ps)-40; i-- {
			l := tallerRePrefijo.ReplaceAllString(ps[i], "")
			if j := strings.Index(l, ": "); j > 0 && strings.Contains(l, "«") {
				quien, dijo = strings.Fields(l[:j])[0], l[j+2:]
				break
			}
		}
	}
	if quien == "" {
		quien, dijo, glosa = o.abla.DecirComo("Humano", fmt.Sprint(ser.ID+1), tallerInstruccion)
	}
	if quien == "" {
		return
	}
	texto := dijo
	if glosa != "" && !strings.Contains(dijo, glosa) {
		texto += " " + glosa
	}
	// lo que su traducción dice de cada palabra suya, lo aprende; y
	// entonces lo construye con sus palabras (que ya significan eso)
	nuevas, alineada := tallerAprenderDeGlosa(dijo, glosa, "Abla · "+quien)
	if len(nuevas) > 0 {
		o.Decir("taller", "Abla le da sentido a palabras suyas: "+strings.Join(nuevas, ", "), "")
	}
	construye := texto
	if alineada {
		construye = strings.Replace(dijo, "«"+glosa+"»", "", 1)
	}
	hecho := o.construirCon("abla", "Abla · "+quien, construye)
	o.Decir("Abla · "+quien, prefijo(dijo, 260)+"  ⟶ "+hecho, glosa)
	// Nexo oye lo que quiere decir (su traducción), no el «Nyx me dijo…»
	if glosa != "" {
		o.oyeNexo("Abla", glosa)
	} else {
		o.oyeNexo("Abla", texto)
	}
	o.mu.Lock()
	o.ultimoAbla = texto
	o.habla++
	o.ablaEn = o.habla
	o.mu.Unlock()
}

// ---------- las manos: el editor con el que cambian lo que ya hay ----------

type tallerManos struct {
	o     *tallerObra
	autor string // nyx, abla
	quien string // cómo sale en la charla
}

// Quien: cómo sale en la charla (para lo que aprende).
func (m *tallerManos) Quien() string { return m.quien }

func (m *tallerManos) Tomar(x, y, z float64) (string, [3]float64, bool) {
	m.o.mu.Lock()
	defer m.o.mu.Unlock()
	var mejor *tallerPieza
	dmin := 40.0
	for _, p := range m.o.Piezas {
		if d := math.Sqrt((p.X-x)*(p.X-x) + (p.Y-y)*(p.Y-y) + (p.Z-z)*(p.Z-z)); d < dmin {
			dmin, mejor = d, p
		}
	}
	if mejor == nil {
		return "", [3]float64{}, false
	}
	return mejor.Base, [3]float64{mejor.X, mejor.Y, mejor.Z}, true
}

// TomarNombre: lo que se llame así (por su título o su nombre), lo más
// cercano que haya.
func (m *tallerManos) TomarNombre(palabra string, x, y, z float64) (string, [3]float64, bool) {
	clave := tallerSlug(palabra)
	if clave == "" || clave == "pieza" {
		return "", [3]float64{}, false
	}
	m.o.mu.Lock()
	defer m.o.mu.Unlock()
	var mejor *tallerPieza
	dmin := math.Inf(1)
	for _, p := range m.o.Piezas {
		if !strings.Contains(tallerSlug(p.Titulo), clave) && !strings.Contains(p.Base, clave) {
			continue
		}
		if d := math.Sqrt((p.X-x)*(p.X-x) + (p.Y-y)*(p.Y-y) + (p.Z-z)*(p.Z-z)); d < dmin {
			dmin, mejor = d, p
		}
	}
	if mejor == nil {
		return "", [3]float64{}, false
	}
	return mejor.Base, [3]float64{mejor.X, mejor.Y, mejor.Z}, true
}

func (m *tallerManos) Inclinar(base string, a float64) {
	if p := m.o.Pieza(base); p != nil {
		m.o.mu.Lock()
		p.RotX += a
		m.o.cambio(p.X, p.Z)
		m.o.mu.Unlock()
		_ = m.o.Guardar()
	}
}

// Pintar: toda la pieza de un material (madera, piedra, una palabra suya…).
func (m *tallerManos) Pintar(base, material string) {
	if p := m.o.Pieza(base); p != nil {
		_ = m.o.Cambiar(p, m.quien, "la pinta de "+material, func(c string) (string, error) {
			return tallerAnadir(c, m.quien, fmt.Sprintf("c.PintarTodo(%q)", material))
		})
	}
}

// Apilar: lo pone encima de lo más cercano que tenga debajo o al lado.
func (m *tallerManos) Apilar(base string) bool {
	p := m.o.Pieza(base)
	if p == nil {
		return false
	}
	m.o.mu.Lock()
	var otra *tallerPieza
	dmin := 40.0
	for _, q := range m.o.Piezas {
		if q == p {
			continue
		}
		if d := math.Hypot(q.X-p.X, q.Z-p.Z) + math.Abs(q.Y-p.Y)*0.5; d < dmin {
			dmin, otra = d, q
		}
	}
	m.o.mu.Unlock()
	if otra == nil {
		return false
	}
	m.o.MoverA(p, otra.X, otra.Y+otra.Alto, otra.Z, p.Rumbo)
	return true
}

// Caer: baja hasta lo primero que tenga debajo (si no hay nada, se queda:
// en el vacío no hay fondo).
func (m *tallerManos) Caer(base string) bool {
	p := m.o.Pieza(base)
	if p == nil {
		return false
	}
	y, ok := m.sueloBajo(p.X, p.Y, p.Z, p)
	if !ok {
		return false
	}
	m.o.MoverA(p, p.X, y, p.Z, p.Rumbo)
	return true
}

func (m *tallerManos) SueloBajo(x, y, z float64) (float64, bool) { return m.sueloBajo(x, y, z, nil) }

func (m *tallerManos) sueloBajo(x, y, z float64, sin *tallerPieza) (float64, bool) {
	m.o.mu.Lock()
	defer m.o.mu.Unlock()
	mejor, hay := math.Inf(-1), false
	for _, q := range m.o.Piezas {
		if q == sin {
			continue
		}
		top := q.Y + q.Alto
		if top <= y+0.01 && math.Hypot(q.X-x, q.Z-z) < math.Max(1, q.Radio) && top > mejor {
			mejor, hay = top, true
		}
	}
	if m.o.Pasillos && (!hay || mejor < 0) && y >= 0 {
		return 0, true // con el mundo de Nyx de fondo, su suelo
	}
	return mejor, hay
}

// Deshacer: vuelve a la versión de antes (las viejas se guardan).
func (m *tallerManos) Deshacer(base string) bool {
	p := m.o.Pieza(base)
	if p == nil {
		return false
	}
	m.o.mu.Lock()
	defer m.o.mu.Unlock()
	n := len(p.Historia)
	if n == 0 {
		return false
	}
	p.Ultima = max(p.Ultima, p.Version)
	p.ID = p.Historia[n-1]
	p.Historia = p.Historia[:n-1]
	p.Version = len(p.Historia) + 1
	if c, err := os.ReadFile(filepath.Join(m.o.m.dir, "objetos", p.ID+".go")); err == nil {
		if comp, err := tallerCompilar(string(c)); err == nil {
			_, p.Alto = medirCuerpo(comp.entero)
			p.Radio = tallerRadio(comp.entero)
		}
	}
	p.Cambios = append(p.Cambios, m.quien+": deshace su último cambio")
	m.o.cambio(p.X, p.Z)
	go func() { _ = m.o.Guardar() }()
	return true
}

// Estirar: más alto, más ancho o más fondo (cada eje por su lado).
func (m *tallerManos) Estirar(base string, kx, ky, kz float64) {
	if p := m.o.Pieza(base); p != nil {
		_ = m.o.Cambiar(p, m.quien, "lo estira", func(c string) (string, error) {
			return tallerAnadir(c, m.quien, fmt.Sprintf("c.EscalarEjes(%s, %s, %s)", f3(kx), f3(ky), f3(kz)))
		})
	}
}

func (m *tallerManos) Origen(base string) ([3]float64, bool) {
	p := m.o.Pieza(base)
	if p == nil {
		return [3]float64{}, false
	}
	m.o.mu.Lock()
	defer m.o.mu.Unlock()
	return [3]float64{p.X, p.Y, p.Z}, true
}

func (m *tallerManos) Mover(base string, dx, dy, dz float64) {
	if p := m.o.Pieza(base); p != nil {
		m.o.MoverA(p, p.X+dx, p.Y+dy, p.Z+dz, p.Rumbo)
	}
}

func (m *tallerManos) Girar(base string, a float64) {
	if p := m.o.Pieza(base); p != nil {
		m.o.MoverA(p, p.X, p.Y, p.Z, p.Rumbo+a)
	}
}

func (m *tallerManos) Escalar(base string, k float64) {
	p := m.o.Pieza(base)
	if p == nil {
		return
	}
	que := "lo agranda"
	if k < 1 {
		que = "lo achica"
	}
	_ = m.o.Cambiar(p, m.quien, que, func(c string) (string, error) {
		e := tallerValor(c, "escala")
		if e <= 0 {
			e = 1
		}
		return tallerRetocar(c, "escala", math.Max(0.02, math.Min(e*k, 200)))
	})
}

func (m *tallerManos) Tenir(base string, col [3]float64) {
	if p := m.o.Pieza(base); p != nil {
		_ = m.o.Cambiar(p, m.quien, "lo tiñe", func(c string) (string, error) {
			return tallerRetocar(c, "tinte", col[0], col[1], col[2], 0.6)
		})
	}
}

func (m *tallerManos) Copiar(base string, x, y, z float64) string {
	p := m.o.Pieza(base)
	if p == nil {
		return ""
	}
	d, err := os.ReadFile(filepath.Join(m.o.m.dir, "objetos", p.ID+".go"))
	if err != nil {
		return ""
	}
	q, err := m.o.CrearEn(m.autor, "copia de "+p.Titulo, p.Base, string(d), x, y, z)
	if err != nil {
		return ""
	}
	return q.Base
}

func (m *tallerManos) Quitar(base string) {
	if p := m.o.Pieza(base); p != nil {
		m.o.Quitar(p)
	}
}

func (m *tallerManos) Escribir(base, lineas string) {
	if p := m.o.Pieza(base); p != nil {
		_ = m.o.Cambiar(p, m.quien, "le escribe código", func(c string) (string, error) {
			return tallerAnadir(c, m.quien, lineas)
		})
	}
}

func tallerNombreAutor(a string) string {
	switch a {
	case "nyx":
		return "Nyx"
	case "abla":
		return "Abla"
	case "nexo":
		return "Nexo"
	}
	if strings.HasPrefix(a, "ia:") {
		return a[3:]
	}
	return a
}

// Ver: les enseña un vídeo (de YouTube u otra web, un archivo, o una
// carpeta de fotos) a las dos.
func (o *tallerObra) Ver(fuente string, avisar func(string)) error {
	ruta, titulo := fuente, ""
	if strings.HasPrefix(fuente, "http://") || strings.HasPrefix(fuente, "https://") {
		h := sha1.Sum([]byte(fuente))
		dir := filepath.Join(o.dir, "videos", hex.EncodeToString(h[:6]))
		_ = os.MkdirAll(dir, 0o755)
		if d, err := os.ReadFile(filepath.Join(dir, "titulo.txt")); err == nil {
			titulo = strings.TrimSpace(string(d))
		}
		es, _ := os.ReadDir(dir)
		for _, e := range es {
			if strings.HasPrefix(e.Name(), "video.") && !strings.HasSuffix(e.Name(), ".part") {
				ruta = filepath.Join(dir, e.Name())
			}
		}
		if ruta == fuente {
			avisar("  bajando el vídeo (para las dos)…")
			var err error
			if ruta, titulo, err = BajarVideo(fuente, dir); err != nil {
				return err
			}
			_ = os.WriteFile(filepath.Join(dir, "titulo.txt"), []byte(titulo), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "fuente.txt"), []byte(fuente), 0o644)
		}
	}
	if titulo == "" {
		titulo = strings.TrimSuffix(filepath.Base(ruta), filepath.Ext(ruta))
	}
	o.mu.Lock()
	o.viendo = titulo
	o.mu.Unlock()
	o.Decir("taller", "Nyx, Abla y Nexo ven «"+titulo+"»", "")
	// Abla: sus fotogramas, a la cola de lo que mira
	if o.abla.Viva() {
		n, err := tallerFotogramasParaAbla(ruta, tallerSlug(titulo), filepath.Join(o.abla.dir, "fotos", "entrada"))
		if err != nil {
			avisar("  Abla no puede verlo: " + err.Error())
		} else {
			avisar(fmt.Sprintf("  Abla tiene %d fotogramas en la cola (los mira poco a poco, mientras piensa)", n))
		}
	}
	// Nyx: lo aprende a su manera (lo recuerda como sitio)
	return o.m.Aprender(ruta, titulo, avisar)
}

// tallerFotogramasParaAbla: con ffmpeg, un fotograma cada pocos segundos,
// en PPM de hasta 320×240 (lo que entiende Abla). Se escriben aparte y se
// mueven al final, para que Abla no lea uno a medias.
func tallerFotogramasParaAbla(ruta, nombre, cola string) (int, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return 0, fmt.Errorf("falta ffmpeg:  sudo apt install ffmpeg")
	}
	if err := os.MkdirAll(cola, 0o755); err != nil {
		return 0, err
	}
	// al lado de la cola (mismo disco): luego basta con renombrar
	tmp, err := os.MkdirTemp(filepath.Dir(cola), "llegando-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)
	escala := "scale=320:240:force_original_aspect_ratio=decrease"
	var cmd *exec.Cmd
	if st, err := os.Stat(ruta); err == nil && st.IsDir() {
		es, _ := os.ReadDir(ruta)
		k := 0
		for _, e := range es {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" && ext != ".bmp" {
				continue
			}
			k++
			c := exec.Command("ffmpeg", "-loglevel", "error", "-y", "-i", filepath.Join(ruta, e.Name()), "-vf", escala, "-frames:v", "1",
				filepath.Join(tmp, fmt.Sprintf("%s-%03d.ppm", nombre, k)))
			_ = c.Run()
			if k >= 100 {
				break
			}
		}
	} else {
		cmd = exec.Command("ffmpeg", "-loglevel", "error", "-y", "-i", ruta, "-vf", "fps=1/3,"+escala, "-frames:v", "80",
			filepath.Join(tmp, nombre+"-%03d.ppm"))
		if out, err := cmd.CombinedOutput(); err != nil {
			return 0, fmt.Errorf("ffmpeg: %s", strings.TrimSpace(string(out)))
		}
	}
	es, _ := os.ReadDir(tmp)
	n := 0
	for _, e := range es {
		if err := tallerMoverArchivo(filepath.Join(tmp, e.Name()), filepath.Join(cola, e.Name())); err == nil {
			n++
		}
	}
	return n, nil
}

// tallerMoverArchivo: renombrar (y si está en otro disco, copiar y borrar).
func tallerMoverArchivo(a, b string) error {
	if os.Rename(a, b) == nil {
		return nil
	}
	d, err := os.ReadFile(a)
	if err != nil {
		return err
	}
	parcial := strings.TrimSuffix(b, filepath.Ext(b)) + ".parcial"
	if err := os.WriteFile(parcial, d, 0o644); err != nil {
		return err
	}
	_ = os.Remove(a)
	return os.Rename(parcial, b)
}

// ---------- jugar ----------

func (o *tallerObra) motorEnMarcha() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.motorVivo
}

// CrearTu: lo que escribes tú, construido con las mismas reglas que ellas
// (tu constructor empieza cerca de donde estás).
func (o *tallerObra) CrearTu(texto string) string {
	t := o.tortuga("tú")
	o.motor.mu.Lock()
	j := o.motor.jug
	o.motor.mu.Unlock()
	o.mu.Lock()
	if math.Hypot(t.X-j[0], t.Z-j[2]) > 25 || math.Abs(t.Y-j[1]) > 25 {
		t.X, t.Y, t.Z = j[0]+2, j[1], j[2]
	}
	o.mu.Unlock()
	r := o.construirCon("tú", "tú", texto)
	o.Decir("tú", texto+"  ⟶ "+r, "")
	return r
}
