package main

// ============================================================
//  TALLER · MÚSICA — las IAs componen y graban audio
// ------------------------------------------------------------
//  Con lo que dicen componen una pieza: cada palabra es una nota (siempre
//  la misma para la misma palabra), la escala, el ritmo y el timbre salen
//  de la frase, y debajo va un bajo. Luego la tocan de verdad: la
//  sintetizan muestra a muestra y la graban como archivo .wav en
//  taller/sonidos/, que suena en el mundo (en la ventana, en 3D, desde
//  el instrumento que la toca). Todo local, sin nada de fuera.
//
//  Timbres: cuerda pulsada (Karplus-Strong), campana (parciales que no
//  son armónicos), flauta (casi un seno) y órgano (armónicos).
// ============================================================

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const tallerMuestras = 16000 // muestras por segundo (suficiente, y ocupa poco)

var tallerEscalas = []struct {
	Nombre string
	Pasos  []int
}{
	{"mayor", []int{0, 2, 4, 5, 7, 9, 11}},
	{"menor", []int{0, 2, 3, 5, 7, 8, 10}},
	{"pentatónica", []int{0, 2, 4, 7, 9}},
	{"dórica", []int{0, 2, 3, 5, 7, 9, 10}},
}

var tallerTimbres = []string{"cuerda", "campana", "flauta", "órgano"}

type tallerComposicion struct {
	Archivo string
	Notas   []int // MIDI; -1 = silencio
	Pulso   float64
	Escala  string
	Timbre  string
	Que     string
}

// Componer: la música de unas palabras.
func Componer(palabras []string) *tallerComposicion { return ComponerCon("", palabras) }

// ComponerCon: lo mismo, con el estilo de quien la compone (su escala, su
// timbre y su tono son suyos; las notas, de las palabras).
func ComponerCon(estilo string, palabras []string) *tallerComposicion {
	s := tallerSembrar(append([]string{estilo}, palabras...))
	esc := tallerEscalas[s.n(0, len(tallerEscalas)-1)]
	if s.tiene(gBajar) || s.tiene(gPequeno) {
		esc = tallerEscalas[1]
	}
	tonica := 48 + s.n(0, 11)
	pulso := s.entre(0.18, 0.42) // segundos por nota
	if s.tiene(gMover) || s.tiene(gAvanzar) {
		pulso *= 0.7
	}
	c := &tallerComposicion{Pulso: pulso, Escala: esc.Nombre, Timbre: tallerTimbres[s.n(0, len(tallerTimbres)-1)]}
	for _, w := range palabras {
		b := tallerSinTildes.Replace(strings.ToLower(w))
		if tallerVacias[b] {
			c.Notas = append(c.Notas, -1) // las palabras pequeñas son silencios
			continue
		}
		h := tallerHash(b)
		grado := int(h % uint64(len(esc.Pasos)*2))
		oct := grado / len(esc.Pasos)
		nota := tonica + 12 + esc.Pasos[grado%len(esc.Pasos)] + 12*oct
		switch tallerGestoDe[b] {
		case gSubir:
			nota += 12
		case gBajar:
			nota -= 12
		}
		c.Notas = append(c.Notas, nota)
		if len([]rune(b)) > 6 {
			c.Notas = append(c.Notas, nota) // las largas, más largas
		}
	}
	for len(c.Notas) < 8 {
		c.Notas = append(c.Notas, c.Notas...)
		if len(c.Notas) == 0 {
			c.Notas = []int{tonica + 12, tonica + 16, tonica + 19}
		}
	}
	for len(c.Notas) > 40 {
		c.Notas = c.Notas[:40]
	}
	c.Archivo = fmt.Sprintf("ia-%x.wav", s.h&0xffffffffff)
	c.Que = fmt.Sprintf("%d notas en escala %s, con timbre de %s", len(c.Notas), esc.Nombre, c.Timbre)
	return c
}

// Melodia: las notas como las entiende el motor («musica:60 64 - 67»).
func (c *tallerComposicion) Melodia() string {
	var p []string
	for _, n := range c.Notas {
		if n < 0 {
			p = append(p, "-")
		} else {
			p = append(p, fmt.Sprint(n))
		}
	}
	return "musica:" + strings.Join(p, " ")
}

func tallerFrecuencia(midi int) float64 { return 440 * math.Pow(2, float64(midi-69)/12) }

// Sintetizar: la toca, muestra a muestra (dos vueltas a la melodía).
func (c *tallerComposicion) Sintetizar() []float64 {
	nNotas := len(c.Notas)
	dur := c.Pulso*float64(nNotas*2) + 1.5
	out := make([]float64, int(dur*tallerMuestras))
	paso := int(c.Pulso * tallerMuestras)
	semilla := uint64(len(c.Archivo)*7919 + nNotas)
	azar := func() float64 {
		semilla = semilla*6364136223846793005 + 1442695040888963407
		return float64(semilla>>11)/float64(1<<53)*2 - 1
	}
	tocar := func(inicio int, midi int, durMuestras int, vol float64, timbre string) {
		f := tallerFrecuencia(midi)
		switch timbre {
		case "cuerda": // Karplus-Strong: un ruido que se vuelve cuerda
			n := int(tallerMuestras / f)
			if n < 2 {
				return
			}
			buf := make([]float64, n)
			for i := range buf {
				buf[i] = azar()
			}
			for i := 0; i < durMuestras*3 && inicio+i < len(out); i++ {
				k := i % n
				v := buf[k]
				buf[k] = 0.996 * 0.5 * (buf[k] + buf[(k+1)%n])
				out[inicio+i] += v * vol
			}
		default:
			for i := 0; i < durMuestras*3 && inicio+i < len(out); i++ {
				t := float64(i) / tallerMuestras
				var v, env float64
				switch timbre {
				case "campana": // parciales que no son armónicos, que se apagan
					v = math.Sin(2*math.Pi*f*t) + 0.5*math.Sin(2*math.Pi*f*2.76*t)*math.Exp(-t*3) + 0.25*math.Sin(2*math.Pi*f*5.4*t)*math.Exp(-t*6)
					env = math.Exp(-t * 2.2)
				case "flauta":
					v = math.Sin(2*math.Pi*f*t) + 0.08*math.Sin(2*math.Pi*f*2*t) + 0.02*azar()
					env = math.Min(1, t*20) * math.Exp(-math.Max(0, t-float64(durMuestras)/tallerMuestras)*12)
				default: // órgano
					v = math.Sin(2*math.Pi*f*t) + 0.5*math.Sin(2*math.Pi*f*2*t) + 0.25*math.Sin(2*math.Pi*f*3*t)
					env = math.Min(1, t*40) * math.Exp(-math.Max(0, t-float64(durMuestras)/tallerMuestras)*15)
				}
				out[inicio+i] += v * env * vol * 0.5
			}
		}
	}
	for vuelta := 0; vuelta < 2; vuelta++ {
		for i, n := range c.Notas {
			t0 := (vuelta*nNotas + i) * paso
			if n >= 0 {
				tocar(t0, n, paso, 0.35, c.Timbre)
			}
			// el bajo: la tónica cada cuatro notas
			if i%4 == 0 && n >= 0 {
				tocar(t0, n-24-(n-24)%12+(c.Notas[0]%12), paso*4, 0.2, "órgano")
			}
		}
	}
	// un eco (sala), y que no sature
	retardo := int(0.21 * tallerMuestras)
	for i := retardo; i < len(out); i++ {
		out[i] += out[i-retardo] * 0.28
	}
	pico := 1e-9
	for _, v := range out {
		pico = math.Max(pico, math.Abs(v))
	}
	for i := range out {
		out[i] = out[i] / pico * 0.85
	}
	return out
}

// Grabar: la toca y la guarda como .wav (16 bits, mono) en dir.
func (c *tallerComposicion) Grabar(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	m := c.Sintetizar()
	datos := make([]byte, 44+2*len(m))
	copy(datos[0:], "RIFF")
	binary.LittleEndian.PutUint32(datos[4:], uint32(36+2*len(m)))
	copy(datos[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(datos[16:], 16)
	binary.LittleEndian.PutUint16(datos[20:], 1) // PCM
	binary.LittleEndian.PutUint16(datos[22:], 1) // mono
	binary.LittleEndian.PutUint32(datos[24:], tallerMuestras)
	binary.LittleEndian.PutUint32(datos[28:], tallerMuestras*2)
	binary.LittleEndian.PutUint16(datos[32:], 2)
	binary.LittleEndian.PutUint16(datos[34:], 16)
	copy(datos[36:], "data")
	binary.LittleEndian.PutUint32(datos[40:], uint32(2*len(m)))
	for i, v := range m {
		binary.LittleEndian.PutUint16(datos[44+2*i:], uint16(int16(v*32767)))
	}
	ruta := filepath.Join(dir, c.Archivo)
	return ruta, os.WriteFile(ruta, datos, 0o644)
}
