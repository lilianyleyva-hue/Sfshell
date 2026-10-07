package main

// ============================================================
//  TALLER · FORMAS PROPIAS — las que inventan y guardan
// ------------------------------------------------------------
//  Cuando una dice «llamar <palabra>» (o nombrar, bautizar…), lo que
//  acaba de construir se guarda con ese nombre en taller/formas/. Desde
//  entonces, esa palabra ya no es una forma cualquiera: es SU forma, la
//  que guardaron (y cualquiera de las dos, o tú, la puede usar).
//
//  En el código: c.Forma("nombre", x, y, z, escala, giro)
//  En la consola: forma guarda <pieza> como <nombre> · forma lista ·
//                 forma borra <nombre>
// ============================================================

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type tallerFormaGuardada struct {
	Nombre string    `json:"nombre"`
	Autor  string    `json:"autor"`
	De     string    `json:"de"` // la pieza de la que salió
	Cuando time.Time `json:"cuando"`
	Alto   float64   `json:"alto"`
	Ancho  float64   `json:"ancho"`
	Pos    []float32 `json:"pos"` // centrada en x, z y apoyada en y = 0
	Nor    []float32 `json:"nor"`
	Col    []float32 `json:"col"`
	Emi    []float32 `json:"emi"`
	Mat    []string  `json:"mat,omitempty"` // por triángulo
}

var (
	tallerFormasMu  sync.Mutex
	tallerFormasDir string
	tallerFormasMem = map[string]*tallerFormaGuardada{}
	tallerFormasNo  = map[string]time.Time{} // las que no existen (para no mirar el disco cada vez)
)

func tallerClaveForma(nombre string) string {
	return tallerSlug(nombre)
}

// tallerFormaPropia: la forma guardada con ese nombre (o nil).
func tallerFormaPropia(nombre string) *tallerFormaGuardada {
	k := tallerClaveForma(nombre)
	tallerFormasMu.Lock()
	defer tallerFormasMu.Unlock()
	if f, ok := tallerFormasMem[k]; ok {
		return f
	}
	if t, ok := tallerFormasNo[k]; ok && time.Since(t) < 5*time.Second {
		return nil
	}
	if tallerFormasDir == "" {
		return nil
	}
	d, err := os.ReadFile(filepath.Join(tallerFormasDir, k+".json"))
	if err != nil {
		tallerFormasNo[k] = time.Now()
		return nil
	}
	var f tallerFormaGuardada
	if json.Unmarshal(d, &f) != nil {
		return nil
	}
	tallerFormasMem[k] = &f
	return &f
}

// Forma: pone una forma guardada (centrada en x, z, apoyada en y).
func (c *Cuerpo) Forma(nombre string, x, y, z, escala, giro float64) {
	f := tallerFormaPropia(nombre)
	if f == nil {
		return
	}
	if escala <= 0 {
		escala = 1
	}
	ex := tallerExtraDe(c)
	antes := ex.material
	s, co := math.Sin(giro), math.Cos(giro)
	rot := func(v [3]float64) [3]float64 { return [3]float64{v[0]*co - v[2]*s, v[1], v[0]*s + v[2]*co} }
	mat := ""
	for t := 0; t*9+8 < len(f.Pos); t++ {
		m := ""
		if t < len(f.Mat) {
			m = f.Mat[t]
		}
		if m != mat {
			c.Material(m)
			mat = m
		}
		var p, n [3][3]float64
		for k := 0; k < 3; k++ {
			i := (t*3 + k) * 3
			q := rot([3]float64{float64(f.Pos[i]) * escala, float64(f.Pos[i+1]) * escala, float64(f.Pos[i+2]) * escala})
			p[k] = [3]float64{q[0] + x, q[1] + y, q[2] + z}
			n[k] = rot([3]float64{float64(f.Nor[i]), float64(f.Nor[i+1]), float64(f.Nor[i+2])})
		}
		col := Color{float64(f.Col[t*9]), float64(f.Col[t*9+1]), float64(f.Col[t*9+2])}
		c.tri(p[0], p[1], p[2], n[0], n[1], n[2], col, f.Emi[t*3])
	}
	if mat != antes {
		c.Material(antes)
	}
}

// GuardarForma: lo que construye una pieza, guardado como forma con nombre.
func (o *tallerObra) GuardarForma(nombre string, p *tallerPieza, autor string) error {
	k := tallerClaveForma(nombre)
	if k == "" || k == "pieza" {
		return errors.New("ese nombre no vale")
	}
	d, err := os.ReadFile(filepath.Join(o.m.dir, "objetos", p.ID+".go"))
	if err != nil {
		return err
	}
	comp, err := tallerCompilar(string(d))
	if err != nil {
		return err
	}
	c := comp.entero
	mn := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	mx := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for i := 0; i+2 < len(c.Pos); i += 3 {
		for j := 0; j < 3; j++ {
			mn[j], mx[j] = math.Min(mn[j], float64(c.Pos[i+j])), math.Max(mx[j], float64(c.Pos[i+j]))
		}
	}
	cx, cz := (mn[0]+mx[0])/2, (mn[2]+mx[2])/2
	f := &tallerFormaGuardada{Nombre: nombre, Autor: autor, De: p.ID, Cuando: time.Now(), Alto: mx[1] - mn[1],
		Ancho: math.Max(mx[0]-mn[0], mx[2]-mn[2]), Nor: c.Nor, Col: c.Col, Emi: c.Emi, Mat: comp.matTri}
	f.Pos = make([]float32, len(c.Pos))
	for i := 0; i+2 < len(c.Pos); i += 3 {
		f.Pos[i], f.Pos[i+1], f.Pos[i+2] = c.Pos[i]-float32(cx), c.Pos[i+1]-float32(mn[1]), c.Pos[i+2]-float32(cz)
	}
	if err := os.MkdirAll(tallerFormasDir, 0o755); err != nil {
		return err
	}
	j, _ := json.Marshal(f)
	if err := os.WriteFile(filepath.Join(tallerFormasDir, k+".json"), j, 0o644); err != nil {
		return err
	}
	tallerFormasMu.Lock()
	tallerFormasMem[k] = f
	delete(tallerFormasNo, k)
	tallerFormasMu.Unlock()
	return nil
}

// tallerListaFormas: las formas guardadas (nombre, de quién, cuánto mide).
func tallerListaFormas() []string {
	es, _ := os.ReadDir(tallerFormasDir)
	var out []string
	for _, e := range es {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if f := tallerFormaPropia(strings.TrimSuffix(e.Name(), ".json")); f != nil {
			out = append(out, f.Nombre+"  (de "+f.Autor+", "+f2(f.Ancho)+" × "+f2(f.Alto)+" m, sacada de "+f.De+")")
		}
	}
	sort.Strings(out)
	return out
}

func tallerBorrarForma(nombre string) error {
	k := tallerClaveForma(nombre)
	tallerFormasMu.Lock()
	delete(tallerFormasMem, k)
	tallerFormasMu.Unlock()
	return os.Remove(filepath.Join(tallerFormasDir, k+".json"))
}
