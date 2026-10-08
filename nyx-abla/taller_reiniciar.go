package main

// ============================================================
//  TALLER · REINICIAR — el mundo vuelve a estar vacío
// ------------------------------------------------------------
//  `reinicia si` quita todo lo construido: las piezas y todas sus
//  versiones (.go, .obj, .mtl), la música que compusieron las IAs, los
//  puntos de los juegos y por dónde iba cada constructor. Lo que SABEN se
//  queda: el modelo de lenguaje, Nexo, las palabras que aprendieron, las
//  habilidades de las 36, lo que vio Abla, la charla y la cabeza de Nyx.
//  Solo borra archivos del taller: lo que guarda Nyx Mundo en la misma
//  carpeta (sus propios objetos) no se toca.
// ============================================================

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var tallerReVersion = regexp.MustCompile(`^(.*?)(-v\d+)?$`)

// Reiniciar: vacía el mundo. Devuelve cuántas piezas quitó y cuántos
// archivos borró.
func (o *tallerObra) Reiniciar() (piezas, archivos int) {
	o.mu.Lock()
	bases := map[string]bool{}
	for _, p := range o.Piezas {
		bases[p.Base] = true
		o.cambio(p.X, p.Z) // que la ventana vuelva a pedir esos trozos
	}
	piezas = len(o.Piezas)
	o.Piezas, o.Tortugas, o.Puntos, o.Sig = nil, nil, nil, 1
	o.mu.Unlock()

	// sus archivos: «base.go», «base-v3.obj»… (solo los de las piezas del taller)
	dir := filepath.Join(o.m.dir, "objetos")
	if es, err := os.ReadDir(dir); err == nil {
		for _, e := range es {
			nombre := e.Name()
			ext := filepath.Ext(nombre)
			if ext != ".go" && ext != ".obj" && ext != ".mtl" {
				continue
			}
			m := tallerReVersion.FindStringSubmatch(strings.TrimSuffix(nombre, ext))
			if m != nil && bases[m[1]] && os.Remove(filepath.Join(dir, nombre)) == nil {
				archivos++
			}
		}
	}
	// la música que compusieron (la tuya, la que pusiste tú, se queda)
	if ms, err := filepath.Glob(filepath.Join(o.dir, "sonidos", "ia-*.wav")); err == nil {
		for _, m := range ms {
			if os.Remove(m) == nil {
				archivos++
			}
		}
	}

	// el motor olvida lo que tenía compilado
	mt := o.motor
	mt.mu.Lock()
	mt.vivos = map[string]*tallerPiezaViva{}
	mt.fisicas = map[string]*tallerFisica{}
	mt.compilado = map[string]string{}
	mt.mu.Unlock()

	// el modelo del mundo: no queda nada en el mapa (lo aprendido al
	// imaginar, su calibración, se queda)
	if o.ias != nil {
		md := o.ias.modelo
		md.mu.Lock()
		md.Visitas = map[string]int{}
		md.ocupa, md.techo, md.palabras = map[string]float64{}, map[string]float64{}, map[string][]string{}
		md.firma = ""
		md.mu.Unlock()
		// las 36 vuelven a empezar alrededor del centro
		o.ias.mu.Lock()
		for i, s := range o.ias.Seres {
			ang := float64(i) / float64(len(o.ias.Seres)) * 2 * math.Pi
			s.Foco = [3]float64{math.Cos(ang) * 14, 0, math.Sin(ang) * 14}
			s.Haciendo = ""
		}
		o.ias.mu.Unlock()
		_ = o.ias.Guardar()
	}
	_ = o.Guardar()
	return piezas, archivos
}
