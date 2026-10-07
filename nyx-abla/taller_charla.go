package main

// ============================================================
//  TALLER · CHARLA — Nyx y Abla trabajando juntas
// ------------------------------------------------------------
//  Por turnos, sin parar (mientras el taller está abierto):
//    · Abla cuenta lo que vio (en Abla y en español) y construye algo
//      con ello; Nyx le contesta (con su propia cabeza, que no se toca).
//    · Nyx construye algo con lo que recuerda y se lo cuenta a Abla;
//      uno de los 27 le contesta.
//    · Cada una retoca lo que hizo la otra.
//    · De vez en cuando Abla le pide a Nyx una criatura, y los seres de
//      Abla salen a caminar por el mundo diciendo lo que piensan.
//  Y las dos ven vídeos: «ven <dirección>» se lo enseña a las dos a la
//  vez (Nyx lo recuerda como sitio; Abla mira sus fotogramas).
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
		if pausa {
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

// vistaParaCrear: una foto que Abla no haya usado todavía (o, si ya las
// usó todas, cualquiera: los recuerdos vuelven).
func (o *tallerObra) vistaParaCrear(rng *rand.Rand) *tallerVista {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := len(o.Vistas) - 1; i >= 0; i-- {
		if !o.Vistas[i].Usada {
			o.Vistas[i].Usada = true
			v := o.Vistas[i]
			return &v
		}
	}
	if len(o.Vistas) == 0 {
		return nil
	}
	v := o.Vistas[rng.Intn(len(o.Vistas))]
	return &v
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

// ultimaDe: la última pieza que hizo alguien (para que la otra la retoque).
func (o *tallerObra) ultimaDe(autor string, rng *rand.Rand) *tallerPieza {
	o.mu.Lock()
	defer o.mu.Unlock()
	var de []*tallerPieza
	for _, p := range o.Piezas {
		if p.Autor == autor {
			de = append(de, p)
		}
	}
	if len(de) == 0 {
		return nil
	}
	// casi siempre una reciente; a veces una vieja
	if rng.Float64() < 0.7 {
		return de[len(de)-1]
	}
	return de[rng.Intn(len(de))]
}

// Turnar: un turno de trabajo.
func (o *tallerObra) Turnar(rng *rand.Rand) {
	nuevas := o.recogerVistas()
	o.mu.Lock()
	o.Turno++
	t := o.Turno
	o.mu.Unlock()
	abla := o.abla.Viva()
	if len(nuevas) > 0 {
		v := nuevas[len(nuevas)-1]
		o.Decir("Abla", fmt.Sprintf("ha mirado %d foto(s) más. La última: %s", len(nuevas), v.Descripcion), "")
	}
	if !abla {
		// Nyx sola: construye y retoca lo suyo
		if t%3 == 2 {
			o.retocaNyx(rng, "nyx", nil)
		} else {
			o.creaNyx(rng, "")
		}
		return
	}
	switch t % 6 {
	case 0, 3:
		o.creaAbla(rng, "", -1)
	case 1, 4:
		o.creaNyx(rng, "")
	case 2:
		o.retocaAbla(rng, "nyx", nil)
	case 5:
		o.retocaNyx(rng, "abla", nil)
	}
	if t%15 == 7 {
		o.seresAblaAlMundo(rng, 3)
	}
	if t%24 == 11 {
		o.ablaPideCriatura(rng)
	}
	_ = o.Guardar()
}

func (o *tallerObra) creaNyx(rng *rand.Rand, pedido string) (*tallerPieza, error) {
	titulo, de, codigo := tallerIdeaNyx(o.m, rng, pedido)
	p, err := o.Crear("nyx", titulo, de, codigo, math.NaN(), math.NaN())
	if err != nil {
		o.Decir("Nyx", "quise hacer "+titulo+" pero mi código no compiló: "+err.Error(), "")
		return nil, err
	}
	frase := fmt.Sprintf("He hecho %s (%s) con lo que recuerdo de «%s».", titulo, p.ID, de)
	o.Decir("Nyx", frase, "")
	if o.abla.Viva() {
		// se lo cuenta a Abla, con palabras que ella conoce
		o.m.mu.Lock()
		palabras := []string{titulo}
		for _, r := range o.m.Recuerdos {
			if r.Nombre == de {
				palabras = append(palabras, describirRasgosPalabras(r.Rasgos)...)
				break
			}
		}
		o.m.mu.Unlock()
		o.ablaContesta(rng, strings.Join(palabras, " "))
	}
	return p, nil
}

// ablaContesta: uno de los 27 contesta. Si no conoce ninguna de esas
// palabras, lo relaciona con lo último que vio ella.
func (o *tallerObra) ablaContesta(rng *rand.Rand, texto string) {
	ser, _ := o.serDeTribu(rng, rng.Intn(3))
	quien, dijo, glosa := o.abla.Decir(fmt.Sprint(ser.ID+1), texto)
	if quien == "" {
		if v := o.vistaReciente(); v != nil && len(v.Conceptos) > 0 {
			quien, dijo, glosa = o.abla.Decir(fmt.Sprint(ser.ID+1), strings.ReplaceAll(v.Glosa(), ",", ""))
		} else {
			quien, dijo, glosa = o.abla.Decir(fmt.Sprint(ser.ID+1), "luz mundo")
		}
	}
	if quien != "" {
		o.Decir("Abla · "+quien, dijo, glosa)
	}
}

func (o *tallerObra) creaAbla(rng *rand.Rand, pedido string, tribu int) (*tallerPieza, error) {
	if tribu < 0 {
		tribu = rng.Intn(3)
	}
	v := o.vistaParaCrear(rng)
	ser, _ := o.serDeTribu(rng, tribu)
	titulo, de, codigo := tallerIdeaAbla(o, rng, v, tribu, pedido)
	p, err := o.Crear("abla", titulo, de, codigo, math.NaN(), math.NaN())
	if err != nil {
		o.Decir("Abla · "+ser.Nombre, "quise hacer "+titulo+" pero mi código no compiló: "+err.Error(), "")
		return nil, err
	}
	if v != nil {
		o.Decir("Abla · "+ser.Nombre, fmt.Sprintf("%s → he hecho %s (%s) con lo que vi en «%s».", v.Abla, titulo, p.ID, de), v.Glosa())
	} else {
		o.Decir("Abla · "+ser.Nombre, fmt.Sprintf("he hecho %s (%s) de mi imaginación.", titulo, p.ID), "")
	}
	// Nyx le contesta con su cabeza (sin tocarla: le habla como tú)
	if v != nil && rng.Float64() < 0.5 {
		r := o.m.Responder(v.Glosa())
		o.Decir("Nyx", prefijo(r, 300), "")
	}
	return p, nil
}

// retocaAbla: Abla cambia una pieza (p, o si es nil, una reciente de deQuien).
func (o *tallerObra) retocaAbla(rng *rand.Rand, deQuien string, p *tallerPieza) {
	if p == nil {
		p = o.ultimaDe(deQuien, rng)
	}
	if p == nil {
		o.creaAbla(rng, "", -1)
		return
	}
	v := o.vistaReciente()
	var que string
	err := o.Cambiar(p, "Abla", "retoca", func(cod string) (string, error) {
		q, nuevo, err := tallerRetoqueAbla(rng, cod, v)
		que = q
		return nuevo, err
	})
	if err != nil {
		o.Decir("Abla", "quise cambiar "+p.Base+" pero "+err.Error(), "")
		return
	}
	o.mu.Lock()
	p.Cambios[len(p.Cambios)-1] = "Abla: " + que
	o.mu.Unlock()
	frase := fmt.Sprintf("En %s (de %s) %s.", p.Titulo, tallerNombreAutor(p.Autor), que)
	glosa := ""
	if v != nil {
		glosa = v.Glosa()
	}
	o.Decir("Abla", frase, glosa)
}

// retocaNyx: Nyx cambia una pieza (p, o si es nil, una reciente de deQuien).
func (o *tallerObra) retocaNyx(rng *rand.Rand, deQuien string, p *tallerPieza) {
	if p == nil {
		p = o.ultimaDe(deQuien, rng)
	}
	if p == nil {
		o.creaNyx(rng, "")
		return
	}
	var que string
	err := o.Cambiar(p, "Nyx", "retoca", func(cod string) (string, error) {
		q, nuevo, err := tallerRetoqueNyx(o.m, rng, cod)
		que = q
		return nuevo, err
	})
	if err != nil {
		o.Decir("Nyx", "quise cambiar "+p.Base+" pero "+err.Error(), "")
		return
	}
	o.mu.Lock()
	p.Cambios[len(p.Cambios)-1] = "Nyx: " + que
	o.mu.Unlock()
	o.Decir("Nyx", fmt.Sprintf("En %s (de %s) %s.", p.Titulo, tallerNombreAutor(p.Autor), que), "")
	if o.abla.Viva() {
		o.ablaContesta(rng, p.Titulo+" "+que)
	}
}

func tallerNombreAutor(a string) string {
	switch a {
	case "nyx":
		return "Nyx"
	case "abla":
		return "Abla"
	}
	return a
}

func (o *tallerObra) vistaReciente() *tallerVista {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.Vistas) == 0 {
		return nil
	}
	v := o.Vistas[len(o.Vistas)-1]
	return &v
}

// seresAblaAlMundo: algunos de los 27 salen a caminar por el mundo de Nyx
// (como entidades suyas, con su código en entidades/), diciendo lo último
// que han pensado.
func (o *tallerObra) seresAblaAlMundo(rng *rand.Rand, cuantos int) {
	seres := o.abla.Seres()
	if len(seres) == 0 {
		return
	}
	// lo que ha pensado cada uno
	frases := map[string][]string{}
	for _, l := range o.abla.Pensamientos() {
		l = tallerRePrefijo.ReplaceAllString(l, "")
		if i := strings.Index(l, ": "); i > 0 {
			quien := strings.Fields(l[:i])
			if len(quien) > 0 {
				frases[quien[0]] = append(frases[quien[0]], l[i+2:])
			}
		}
	}
	rng.Shuffle(len(seres), func(i, j int) { seres[i], seres[j] = seres[j], seres[i] })
	hechos := 0
	for _, s := range seres {
		if hechos >= cuantos {
			break
		}
		nombre := "abla-" + tallerSlug(s.Nombre)
		fs := frases[s.Nombre]
		if len(fs) > 6 {
			fs = fs[len(fs)-6:]
		}
		if len(fs) == 0 && s.Ultimo != "" {
			fs = []string{s.Ultimo}
		}
		codigo := tallerCodigoSerAbla(s, fs)
		o.m.mu.Lock()
		e := o.m.especie(nombre)
		o.m.mu.Unlock()
		nueva := e == nil
		if nueva {
			e = &Especie{Nombre: nombre, Pedido: "uno de los 27 de la especie Abla (" + s.Nombre + ", tribu " + tallerTribus[s.Tribu%3] + ")", Frecuencia: 0.08}
		}
		if err := Compilar(e, codigo); err != nil {
			continue
		}
		if nueva {
			o.m.mu.Lock()
			o.m.Especies = append(o.m.Especies, e)
			o.m.mu.Unlock()
		}
		_ = o.m.guardarCodigo(e)
		o.m.seresMu.Lock()
		jx, jz := o.m.jx, o.m.jz
		o.m.seresMu.Unlock()
		a := rng.Float64() * 2 * math.Pi
		o.m.Aparecer(e, jx+6*math.Cos(a), jz+6*math.Sin(a))
		if nueva {
			o.Decir("Abla · "+s.Nombre, "salgo a caminar por el mundo de Nyx", "")
		}
		hechos++
	}
	_ = o.m.Guardar()
}

// ablaPideCriatura: Abla le pide a Nyx que invente una criatura con lo que
// vio; Nyx la escribe a su manera (en Go, como siempre).
func (o *tallerObra) ablaPideCriatura(rng *rand.Rand) {
	o.m.mu.Lock()
	n := len(o.m.Especies)
	o.m.mu.Unlock()
	if n > 24 {
		return
	}
	v := o.vistaReciente()
	que := "luz"
	if v != nil && len(v.Conceptos) > 0 {
		que = v.Conceptos[rng.Intn(len(v.Conceptos))].Es
	}
	adj := []string{"que te sigue", "que huye", "que vaga despacio", "alta", "pequeña", "que brilla"}[rng.Intn(6)]
	pedido := fmt.Sprintf("crea una criatura de %s %s", que, adj)
	o.Decir("Abla", "Nyx, "+pedido+".", "")
	r := o.m.Responder(pedido)
	o.Decir("Nyx", prefijo(r, 300), "")
}

// ---------- ver vídeos juntas ----------

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
	o.Decir("taller", "Nyx y Abla ven «"+titulo+"»", "")
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
