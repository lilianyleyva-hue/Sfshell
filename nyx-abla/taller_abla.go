package main

// ============================================================
//  TALLER · ABLA — la otra especie, despierta al lado de Nyx
// ------------------------------------------------------------
//  Abla es otro programa (en C): 27 seres que piensan sin parar en
//  su propio idioma. Aquí se arranca en «modo puente»
//  (abla --puente) y se le habla por su entrada; por cada línea
//  contesta UNA línea JSON. Su memoria vive aparte, en
//  ~/.local/share/nyx-mundo/abla/, y lo que ve (fotogramas de
//  vídeos) lo deja en fotos/vistas.jsonl.
//
//  No toca el cerebro de Nyx: se hablan con palabras, como tú.
// ============================================================

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type tallerAbla struct {
	mu       sync.Mutex
	dir      string // su mundo (memoria, fotos)
	bin      string
	cmd      *exec.Cmd
	entrada  io.WriteCloser
	lineas   chan string
	vivo     bool
	fallo    string
	cursor   float64 // hasta qué pensamiento ha leído el taller
	leido    int64   // hasta dónde ha leído fotos/vistas.jsonl
	nombres  []string
	palabras int
}

// tallerVista: lo que Abla vio en una foto (un fotograma).
type tallerVista struct {
	Foto        string  `json:"foto"`
	Mini        string  `json:"mini"`
	Descripcion string  `json:"descripcion"`
	Abla        string  `json:"abla"`
	Brillo      float64 `json:"brillo"`
	Contraste   float64 `json:"contraste"`
	Bordes      float64 `json:"bordes"`
	BordesH     float64 `json:"bordes_h"`
	BordesV     float64 `json:"bordes_v"`
	Arriba      string  `json:"arriba"`
	Abajo       string  `json:"abajo"`
	Paleta      []struct {
		Nombre string  `json:"nombre"`
		RGB    [3]int  `json:"rgb"`
		Frac   float64 `json:"frac"`
	} `json:"paleta"`
	Conceptos []struct {
		Abla string  `json:"abla"`
		Es   string  `json:"es"`
		Peso float64 `json:"peso"`
	} `json:"conceptos"`
	Video string `json:"video,omitempty"` // de qué vídeo era (lo pone el taller)
	Usada bool   `json:"usada,omitempty"` // si ya construyó algo con ella
}

// Glosa: lo que vio, en palabras sueltas (en español).
func (v *tallerVista) Glosa() string {
	var p []string
	for _, c := range v.Conceptos {
		p = append(p, c.Es)
	}
	return strings.Join(p, ", ")
}

// tallerBuscarAbla: dónde está el programa abla.
func tallerBuscarAbla() string {
	if b := os.Getenv("NYX_ABLA"); b != "" {
		return b
	}
	if b, err := exec.LookPath("abla"); err == nil {
		return b
	}
	cand := []string{filepath.Join(dirMundo(), "abla", "abla")}
	if yo, err := os.Executable(); err == nil {
		cand = append(cand, filepath.Join(filepath.Dir(yo), "abla"))
	}
	for _, c := range cand {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func tallerNuevaAbla(dir string) *tallerAbla {
	return &tallerAbla{dir: dir}
}

// Despertar: arranca a Abla. Si no está instalada, lo dice y el taller
// sigue (Nyx puede trabajar sola).
func (a *tallerAbla) Despertar() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vivo {
		return nil
	}
	a.bin = tallerBuscarAbla()
	if a.bin == "" {
		a.fallo = "no encuentro el programa abla (compílalo con  make  en la carpeta especie y ejecuta instalar-abla.sh)"
		return errors.New(a.fallo)
	}
	if err := os.MkdirAll(filepath.Join(a.dir, "fotos", "entrada"), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(a.bin, "--puente", "--mundo", a.dir, "--ritmo", "500")
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	ent, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	sal, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		a.fallo = err.Error()
		return err
	}
	a.cmd, a.entrada, a.lineas = cmd, ent, make(chan string, 16)
	go func(ch chan string) {
		lec := bufio.NewReaderSize(sal, 1<<20)
		for {
			l, err := lec.ReadString('\n')
			if l = strings.TrimSpace(l); strings.HasPrefix(l, "{") {
				ch <- l
			}
			if err != nil {
				close(ch)
				return
			}
		}
	}(a.lineas)
	select {
	case l, ok := <-a.lineas:
		var hola struct {
			Listo    bool `json:"listo"`
			Palabras int  `json:"palabras"`
		}
		if !ok || json.Unmarshal([]byte(l), &hola) != nil || !hola.Listo {
			a.fallo = "abla no despertó bien"
			_ = cmd.Process.Kill()
			return errors.New(a.fallo)
		}
		a.palabras = hola.Palabras
	case <-time.After(30 * time.Second):
		a.fallo = "abla tarda demasiado en despertar"
		_ = cmd.Process.Kill()
		return errors.New(a.fallo)
	}
	a.vivo, a.fallo = true, ""
	// no repetir lo que ya vio en otras sesiones
	if st, err := os.Stat(filepath.Join(a.dir, "fotos", "vistas.jsonl")); err == nil && a.leido == 0 {
		a.leido = st.Size()
	}
	return nil
}

// pedir: una línea a Abla, una línea JSON de vuelta.
func (a *tallerAbla) pedir(linea string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.vivo {
		return "", errors.New("Abla está dormida")
	}
	linea = strings.NewReplacer("\n", " ", "\r", " ").Replace(linea)
	if _, err := io.WriteString(a.entrada, linea+"\n"); err != nil {
		a.vivo, a.fallo = false, "se cerró: "+err.Error()
		return "", err
	}
	select {
	case l, ok := <-a.lineas:
		if !ok {
			a.vivo, a.fallo = false, "se cerró"
			return "", errors.New(a.fallo)
		}
		return l, nil
	case <-time.After(30 * time.Second):
		return "", errors.New("Abla no contesta")
	}
}

// Ejecutar: una orden de su consola (seres, decir, mision, fotos, dic…).
func (a *tallerAbla) Ejecutar(orden string) string {
	l, err := a.pedir(orden)
	if err != nil {
		return "(" + err.Error() + ")"
	}
	var r struct {
		Salida string `json:"salida"`
	}
	_ = json.Unmarshal([]byte(l), &r)
	return strings.TrimRight(r.Salida, "\n")
}

var tallerReRespuesta = regexp.MustCompile(`(?m)^([^:\n(]+): (.+)$`)
var tallerReGlosa = regexp.MustCompile(`«([^»]*)»`)

// Decir: Nyx le dice algo a uno de sus seres (en español: entiende las
// palabras que son raíces de Abla) y devuelve quién contestó, qué dijo y,
// si la hay, la traducción.
func (a *tallerAbla) Decir(ser, texto string) (quien, dijo, glosa string) {
	return a.DecirComo("Nyx", ser, texto)
}

// DecirComo: lo mismo, diciendo quién habla (Nyx, Humano…).
func (a *tallerAbla) DecirComo(hablante, ser, texto string) (quien, dijo, glosa string) {
	_, _ = a.pedir("@hablante " + hablante)
	sal := a.Ejecutar("decir " + ser + " " + texto)
	ms := tallerReRespuesta.FindAllStringSubmatch(sal, -1)
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i][1] != "En Abla" {
			quien, dijo = strings.TrimSpace(ms[i][1]), strings.TrimSpace(ms[i][2])
			break
		}
	}
	if g := tallerReGlosa.FindAllStringSubmatch(dijo, -1); len(g) > 0 {
		glosa = g[len(g)-1][1]
	}
	return
}

// Seres: los nombres de sus 27 (con su tribu).
func (a *tallerAbla) Seres() []tallerSerAbla {
	l, err := a.pedir("@estado")
	if err != nil {
		return nil
	}
	var e struct {
		Seres []tallerSerAbla `json:"seres"`
	}
	_ = json.Unmarshal([]byte(l), &e)
	return e.Seres
}

type tallerSerAbla struct {
	ID       int    `json:"id"`
	Nombre   string `json:"nombre"`
	Tribu    int    `json:"tribu"` // 0 lenguaje, 1 data, 2 lógica
	Concepto string `json:"concepto"`
	Ultimo   string `json:"ultimo"`
}

var tallerTribus = []string{"lenguaje", "data", "lógica"}

// Pensamientos: lo que han pensado desde la última vez.
func (a *tallerAbla) Pensamientos() []string {
	a.mu.Lock()
	desde := a.cursor
	a.mu.Unlock()
	l, err := a.pedir(fmt.Sprintf("@corriente %.0f", desde))
	if err != nil {
		return nil
	}
	var c struct {
		Hasta  float64 `json:"hasta"`
		Lineas []struct {
			Texto string `json:"texto"`
		} `json:"lineas"`
	}
	if json.Unmarshal([]byte(l), &c) != nil {
		return nil
	}
	a.mu.Lock()
	a.cursor = c.Hasta
	a.mu.Unlock()
	var out []string
	for _, x := range c.Lineas {
		out = append(out, x.Texto)
	}
	return out
}

// VistasNuevas: las fotos que ha terminado de mirar desde la última vez.
func (a *tallerAbla) VistasNuevas() []tallerVista {
	f, err := os.Open(filepath.Join(a.dir, "fotos", "vistas.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	a.mu.Lock()
	desde := a.leido
	a.mu.Unlock()
	if _, err := f.Seek(desde, io.SeekStart); err != nil {
		return nil
	}
	var out []tallerVista
	lec := bufio.NewReader(f)
	for {
		l, err := lec.ReadString('\n')
		if err != nil { // una línea a medias: se lee la próxima vez
			break
		}
		desde += int64(len(l))
		var v tallerVista
		if json.Unmarshal([]byte(l), &v) == nil {
			out = append(out, v)
		}
	}
	a.mu.Lock()
	a.leido = desde
	a.mu.Unlock()
	return out
}

// EnCola: cuántas fotos le quedan por mirar.
func (a *tallerAbla) EnCola() int {
	es, _ := os.ReadDir(filepath.Join(a.dir, "fotos", "entrada"))
	n := 0
	for _, e := range es {
		if strings.HasSuffix(e.Name(), ".ppm") {
			n++
		}
	}
	return n
}

func (a *tallerAbla) Viva() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.vivo
}

func (a *tallerAbla) Estado() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vivo {
		return fmt.Sprintf("despierta (%s, %d palabras)", a.bin, a.palabras)
	}
	if a.fallo != "" {
		return "dormida: " + a.fallo
	}
	return "dormida"
}

// Dormir: guarda su memoria y la cierra.
func (a *tallerAbla) Dormir() {
	if !a.Viva() {
		return
	}
	_, _ = a.pedir("@guardar")
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = io.WriteString(a.entrada, "@salir\n")
	_ = a.entrada.Close()
	hecho := make(chan struct{})
	go func() { _ = a.cmd.Wait(); close(hecho) }()
	select {
	case <-hecho:
	case <-time.After(10 * time.Second):
		_ = a.cmd.Process.Kill()
	}
	a.vivo = false
}
