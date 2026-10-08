package main

// ============================================================
//  TALLER · PROGRAMADOR — las IAs escriben código de verdad
// ------------------------------------------------------------
//  Además de construir palabra a palabra, cualquier IA puede escribir un
//  programa en Go entero, con bucles, funciones, recursión y
//  matemáticas (mundo.Sin, mundo.Cos…), como escribe Nyx Mundo el código
//  de sus entidades. No hay planos: lo que dicen elige qué estructura de
//  programa escriben y todos sus números (cuántos, cuánto sube, cuánto
//  gira, qué colores), y siempre sale lo mismo de las mismas palabras.
//
//  Escriben tres clases de programas:
//   · MODELOS 3D: espirales, anillos, escaleras hacia arriba o abajo,
//     arcos, cúpulas (revolución), árboles (una función que se llama a sí
//     misma), rejillas, olas (superficies con senos), muros con huecos,
//     letras con sus palabras… a veces con partes que se mueven.
//   · ENTIDADES: criaturas con cuerpo, patas y cabeza que se mueven solas
//     (scripts que andan, miran, te siguen o huyen, y se acuerdan).
//   · INSTRUMENTOS: lo que suena con la música que componen.
// ============================================================

import (
	"fmt"
	"math"
	"strings"
)

// tallerSemilla: los números que salen de unas palabras (siempre los mismos).
type tallerSemilla struct {
	h     uint64
	words []string
}

func tallerSembrar(palabras []string) *tallerSemilla {
	h := tallerHash(strings.Join(palabras, " "))
	return &tallerSemilla{h: h | 1, words: palabras}
}

// u: un número de 0 a 1 (cada vez otro, siempre los mismos en el mismo orden).
func (s *tallerSemilla) u() float64 {
	s.h ^= s.h << 13
	s.h ^= s.h >> 7
	s.h ^= s.h << 17
	return float64(s.h>>11) / float64(1<<53)
}

func (s *tallerSemilla) entre(a, b float64) float64 { return a + (b-a)*s.u() }
func (s *tallerSemilla) n(a, b int) int             { return a + int(s.u()*float64(b-a+1)) }

// cuenta: si dijeron un número, ese; si no, uno de las palabras.
func (s *tallerSemilla) cuenta(a, b int) int {
	for _, w := range s.words {
		if n, ok := tallerNumeros[tallerSinTildes.Replace(w)]; ok {
			return max(a, min(b, n))
		}
	}
	return s.n(a, b)
}

// color: el que dijeron (o el que ve Nexo en sus palabras), o uno de ellas.
func (s *tallerSemilla) color(sinestesia func(string) ([3]float64, bool)) [3]float64 {
	for _, w := range s.words {
		if c, ok := tallerColores[tallerSinTildes.Replace(w)]; ok {
			return c
		}
	}
	if sinestesia != nil {
		for _, w := range s.words {
			if c, ok := sinestesia(tallerSinTildes.Replace(w)); ok {
				return c
			}
		}
	}
	c := TallerHSV(s.u(), 0.5+0.3*s.u(), 0.75+0.2*s.u())
	return [3]float64{c.R, c.G, c.B}
}

// tiene: si alguna palabra hace ese gesto.
func (s *tallerSemilla) tiene(g tallerGesto) bool {
	for _, w := range s.words {
		b := tallerSinTildes.Replace(w)
		if sig, ok := tallerSignificado(b); ok {
			b = sig
		}
		if tallerGestoDe[b] == g {
			return true
		}
	}
	return false
}

// material: el que dijeron, si dijeron uno.
func (s *tallerSemilla) material() string {
	for _, w := range s.words {
		if b := tallerSinTildes.Replace(w); tallerMateriales[b] {
			return b
		}
	}
	return ""
}

func colGo(c [3]float64) string { return tallerCol(c) }

// tallerPrograma: lo que escribieron, y qué es.
type tallerPrograma struct {
	Codigo string
	Que    string // «una espiral de 14 piezas», «una criatura de 4 patas»…
}

var tallerEstructuras = []string{"espiral", "anillo", "escalera", "arco", "cupula", "arbol", "rejilla", "ola", "muro", "letras"}

// ProgramarModelo: un modelo 3D escrito como programa.
func ProgramarModelo(quien string, palabras []string, sinestesia func(string) ([3]float64, bool)) tallerPrograma {
	s := tallerSembrar(palabras)
	col := s.color(sinestesia)
	col2 := [3]float64{col[2]*0.5 + 0.3, col[0]*0.5 + 0.3, col[1]*0.5 + 0.3}
	estructura := tallerEstructuras[int(s.h>>3)%len(tallerEstructuras)]
	switch {
	case s.tiene(gSubir) || s.tiene(gBajar):
		if s.u() < 0.6 {
			estructura = "escalera"
		}
	case s.tiene(gGirar):
		estructura = "espiral"
	case s.tiene(gAlrededor):
		estructura = "anillo"
	case s.tiene(gSuelo):
		if s.u() < 0.5 {
			estructura = "rejilla"
		}
	}
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	frase := strings.Join(palabras, " ")
	w("// %s lo programó diciendo: «%s»\n// Es Go normal: se puede cambiar (bucles, funciones, mundo.Sin…).\npackage objeto\n\nimport \"mundo\"\n\n", quien, prefijo(frase, 160))
	que := ""
	mat := s.material()
	movil := s.tiene(gMover)
	cuerpo := func() {
		if mat != "" {
			w("\tc.Material(%q)\n", mat)
		}
	}
	switch estructura {
	case "espiral":
		n, r, paso := s.cuenta(8, 40), s.entre(1.5, 6), s.entre(0.25, 0.8)
		giro := s.entre(0.25, 0.7)
		w("const piezas = %d\n\n// una espiral: cada pieza un poco más arriba y más girada\nfunc Construir(c *mundo.Cuerpo) {\n", n)
		cuerpo()
		w("\tfor i := 0; i < piezas; i++ {\n\t\ta := float64(i) * %s\n\t\tr := %s + float64(i)*%s\n", f2(giro), f2(r), f2(s.entre(-0.05, 0.08)))
		w("\t\tc.Caja(mundo.Cos(a)*r, float64(i)*%s, mundo.Sin(a)*r, %s, %s, %s, mundo.RGB(%s+0.3*mundo.Sin(a), %s, %s))\n\t}\n",
			f2(paso), f2(s.entre(0.6, 2)), f2(paso*0.9), f2(s.entre(0.6, 1.5)), f2(col[0]*0.7), f2(col[1]), f2(col[2]))
		w("\tc.Brilla(0, float64(piezas)*%s+0.5, 0, %s, %s)\n}\n", f2(paso), f2(s.entre(0.2, 0.6)), colGo(col2))
		que = fmt.Sprintf("una espiral de %d piezas", n)
	case "anillo":
		n, r := s.cuenta(6, 24), s.entre(3, 12)
		alto := s.entre(1, 5)
		w("const n = %d\nconst radio = %s\n\n// un anillo de columnas, con un aro encima\nfunc Construir(c *mundo.Cuerpo) {\n", n, f2(r))
		cuerpo()
		w("\tfor i := 0; i < n; i++ {\n\t\ta := 2 * mundo.Pi * float64(i) / n\n")
		w("\t\tc.Cilindro(mundo.Cos(a)*radio, 0, mundo.Sin(a)*radio, %s, %s, %s)\n\t}\n", f2(s.entre(0.15, 0.6)), f2(alto), colGo(col))
		w("\tc.Toro(0, %s, 0, radio, %s, %s)\n", f2(alto), f2(s.entre(0.1, 0.4)), colGo(col2))
		if s.u() < 0.5 {
			w("\tc.Cilindro(0, 0, 0, radio+0.5, 0.15, %s)\n", colGo([3]float64{col[0] * 0.5, col[1] * 0.5, col[2] * 0.5}))
		}
		w("}\n")
		que = fmt.Sprintf("un anillo de %d columnas", n)
	case "escalera":
		n := s.cuenta(8, 40)
		sube := s.entre(0.18, 0.35)
		if s.tiene(gBajar) && !s.tiene(gSubir) {
			sube = -sube
		}
		giro := 0.0
		if s.u() < 0.5 {
			giro = s.entre(0.12, 0.4)
		}
		w("const escalones = %d\n\n// una escalera: cada escalón %s m más %s\nfunc Construir(c *mundo.Cuerpo) {\n", n, f2(math.Abs(sube)), map[bool]string{true: "arriba", false: "abajo"}[sube > 0])
		cuerpo()
		w("\tx, z, rumbo := 0.0, 0.0, 0.0\n\tfor i := 0; i < escalones; i++ {\n")
		w("\t\tc.Caja(x, float64(i)*%s, z, 1.6, 0.2, 1.6, %s)\n", f2(sube), colGo(col))
		w("\t\trumbo += %s\n\t\tx += mundo.Cos(rumbo) * 0.9\n\t\tz += mundo.Sin(rumbo) * 0.9\n\t}\n", f2(giro))
		w("\tc.Caja(x, float64(escalones)*%s, z, 4, 0.2, 4, %s) // el rellano\n}\n", f2(sube), colGo(col2))
		que = fmt.Sprintf("una escalera de %d escalones hacia %s", n, map[bool]string{true: "arriba", false: "abajo"}[sube > 0])
	case "arco":
		n, r := s.cuenta(9, 30), s.entre(2, 8)
		w("// un arco: piezas sobre medio círculo\nfunc Construir(c *mundo.Cuerpo) {\n")
		cuerpo()
		w("\tfor i := 0; i <= %d; i++ {\n\t\ta := mundo.Pi * float64(i) / %d\n", n, n)
		w("\t\tc.Caja(mundo.Cos(a)*%s, mundo.Sin(a)*%s, 0, %s, %s, %s, %s)\n\t}\n", f2(r), f2(r), f2(s.entre(0.5, 1.2)), f2(s.entre(0.5, 1.2)), f2(s.entre(0.5, 2)), colGo(col))
		w("\tc.Caja(-%s, -0.1, 0, 1.4, 0.2, 2.4, %s)\n\tc.Caja(%s, -0.1, 0, 1.4, 0.2, 2.4, %s)\n}\n", f2(r), colGo(col2), f2(r), colGo(col2))
		que = "un arco"
	case "cupula":
		pasos := s.n(5, 10)
		r, alto := s.entre(2, 9), s.entre(2, 8)
		w("// una cúpula: un perfil que gira (revolución)\nfunc Construir(c *mundo.Cuerpo) {\n")
		cuerpo()
		w("\tvar perfil []float64\n\tfor i := 0; i <= %d; i++ {\n\t\ta := mundo.Pi / 2 * float64(i) / %d\n", pasos, pasos)
		w("\t\tperfil = append(perfil, mundo.Cos(a)*%s+0.01, mundo.Sin(a)*%s)\n\t}\n", f2(r), f2(alto))
		w("\tc.Revolucion(0, 0, 0, perfil, %d, %s)\n", s.n(8, 24), colGo(col))
		w("\tc.Brilla(0, %s, 0, %s, %s)\n}\n", f2(alto+0.4), f2(s.entre(0.2, 0.5)), colGo(col2))
		que = "una cúpula"
	case "arbol":
		prof := s.n(3, 5)
		w(`// un árbol: una rama que se parte en ramas (una función que se llama a sí misma)
func rama(c *mundo.Cuerpo, x, y, z, largo, angulo, giro float64, nivel int) {
	if nivel == 0 {
		c.Esfera(x, y, z, largo*0.8, %s)
		return
	}
	x2 := x + mundo.Sin(angulo)*mundo.Cos(giro)*largo
	y2 := y + mundo.Cos(angulo)*largo
	z2 := z + mundo.Sin(angulo)*mundo.Sin(giro)*largo
	c.Tubo(x, y, z, x2, y2, z2, largo*0.12, largo*0.08, %s)
	for k := 0; k < %d; k++ {
		rama(c, x2, y2, z2, largo*%s, angulo+%s, giro+float64(k)*2*mundo.Pi/%d, nivel-1)
	}
}

func Construir(c *mundo.Cuerpo) {
`, colGo(col2), colGo([3]float64{col[0] * 0.6, col[1] * 0.45, col[2] * 0.3}), s.n(2, 3), f2(s.entre(0.6, 0.78)), f2(s.entre(0.3, 0.7)), s.n(2, 3))
		cuerpo()
		w("\trama(c, 0, 0, 0, %s, 0, 0, %d)\n}\n", f2(s.entre(1.5, 4)), prof)
		que = fmt.Sprintf("un árbol de %d niveles de ramas", prof)
	case "rejilla":
		nx, nz := s.cuenta(3, 9), s.n(3, 9)
		sep := s.entre(2.5, 6)
		w("// una rejilla: dos bucles, uno dentro de otro (como una ciudad)\nfunc Construir(c *mundo.Cuerpo) {\n")
		cuerpo()
		w("\tfor i := 0; i < %d; i++ {\n\t\tfor j := 0; j < %d; j++ {\n", nx, nz)
		w("\t\t\talto := %s + %s*mundo.Abs(mundo.Sin(float64(i*7+j*3)))\n", f2(s.entre(0.5, 2)), f2(s.entre(1, 8)))
		w("\t\t\tc.Caja(float64(i)*%s, alto/2, float64(j)*%s, %s, alto, %s, mundo.RGB(%s, %s+0.04*float64(j), %s))\n\t\t}\n\t}\n",
			f2(sep), f2(sep), f2(sep*0.6), f2(sep*0.6), f2(col[0]), f2(col[1]*0.7), f2(col[2]))
		w("\tc.Caja(%s, -0.1, %s, %s, 0.2, %s, %s) // el suelo\n}\n", f2(sep*float64(nx-1)/2), f2(sep*float64(nz-1)/2), f2(sep*float64(nx)+2), f2(sep*float64(nz)+2), colGo(col2))
		que = fmt.Sprintf("una rejilla de %d × %d", nx, nz)
	case "ola":
		k := s.n(8, 14)
		w("// una ola: una superficie que sube y baja con senos\nfunc Construir(c *mundo.Cuerpo) {\n")
		cuerpo()
		w("\tconst k = %d\n\tvar alturas []float64\n\tfor i := 0; i < k; i++ {\n\t\tfor j := 0; j < k; j++ {\n", k)
		w("\t\t\talturas = append(alturas, %s+%s*mundo.Sin(float64(i)*%s)*mundo.Cos(float64(j)*%s))\n\t\t}\n\t}\n",
			f2(s.entre(0.5, 2)), f2(s.entre(0.5, 2.5)), f2(s.entre(0.3, 0.9)), f2(s.entre(0.3, 0.9)))
		w("\tc.Superficie(0, 0, 0, %s, %s, alturas, k, []mundo.Color{%s})\n}\n", f2(s.entre(8, 20)), f2(s.entre(8, 20)), colGo(col))
		que = "una ola"
	case "muro":
		largo, alto := s.cuenta(6, 20), s.n(3, 7)
		w("// un muro de bloques, con huecos donde el número sale par\nfunc Construir(c *mundo.Cuerpo) {\n")
		cuerpo()
		w("\tfor i := 0; i < %d; i++ {\n\t\tfor j := 0; j < %d; j++ {\n\t\t\tif (i*3+j*5)%%%d == 0 {\n\t\t\t\tcontinue // una ventana\n\t\t\t}\n", largo, alto, s.n(3, 7))
		w("\t\t\tc.Caja(float64(i)*1.0+float64(j%%2)*0.5, float64(j)*0.6+0.3, 0, 0.98, 0.58, 0.6, %s)\n\t\t}\n\t}\n}\n", colGo(col))
		que = fmt.Sprintf("un muro de %d bloques de largo, con ventanas", largo)
	default: // letras: sus palabras, escritas en 3D
		txt := strings.ToUpper(strings.Join(palabras[:min(3, len(palabras))], " "))
		w("// sus palabras, escritas en el mundo\nfunc Construir(c *mundo.Cuerpo) {\n")
		cuerpo()
		w("\tc.Letras(%q, 0, 0.3, 0, %s, 0.3, %s)\n", txt, f2(s.entre(0.8, 2)), colGo(col))
		w("\tc.Caja(0, 0.1, %s, 1.5, 0.2, %s, %s)\n}\n", f2(float64(len([]rune(txt)))*0.5), f2(float64(len([]rune(txt)))*1.2+1), colGo(col2))
		que = "sus palabras en letras de 3D"
	}
	codigo := b.String()
	if movil {
		// que se mueva: gira despacio todo lo que construye
		codigo = strings.Replace(codigo, "func Construir(c *mundo.Cuerpo) {\n", "func Construir(c *mundo.Cuerpo) {\n\tc.Parte(\"todo\", 0, 0, 0)\n", 1)
		codigo += fmt.Sprintf("\nfunc Actuar(o *mundo.Objeto, dt float64) {\n\to.Rotar(\"todo\", 0, o.Tiempo()*%s, 0)\n}\n", f2(s.entre(0.1, 0.5)))
		que += " que gira"
	}
	return tallerPrograma{Codigo: codigo, Que: que}
}

// ProgramarEntidad: una criatura que se mueve sola. Su cuerpo (cuántas
// patas, cuánto mide, de qué color), cómo anda y qué hace si te ve salen de
// sus palabras.
func ProgramarEntidad(quien, nombre string, palabras []string, sinestesia func(string) ([3]float64, bool)) tallerPrograma {
	s := tallerSembrar(append([]string{nombre}, palabras...))
	col := s.color(sinestesia)
	ojo := [3]float64{1, 1, 0.8}
	patas := []int{0, 2, 4, 6, 8}[s.n(0, 4)]
	alto := s.entre(0.5, 2.5)
	if s.tiene(gGrande) {
		alto *= 2
	}
	if s.tiene(gPequeno) {
		alto *= 0.5
	}
	ancho := alto * s.entre(0.5, 1.2)
	vel := s.entre(0.4, 2.2)
	radio := s.entre(4, 14)
	caracter := []string{"curiosa", "tímida", "vaga"}[s.n(0, 2)]
	vuela := s.tiene(gSubir) || patas == 0 && s.u() < 0.5
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("// %s: una entidad que se inventó %s diciendo: «%s»\n// Anda sola (Actuar), se acuerda de dónde está (Guardar/Valor) y es %s.\npackage objeto\n\nimport \"mundo\"\n\n",
		nombre, quien, prefijo(strings.Join(palabras, " "), 140), caracter)
	w("const (\n\tvelocidad = %s\n\tradio     = %s // no se aleja más de esto de donde nació\n\talto      = %s\n)\n\n", f2(vel), f2(radio), f2(alto))
	w("func Construir(c *mundo.Cuerpo) {\n\tc.Parte(\"cuerpo\", 0, 0, 0)\n")
	base := alto * 0.25
	if vuela {
		base = alto * 0.8
	}
	w("\tc.Esfera(0, %s, 0, %s, %s)\n", f2(base+alto*0.35), f2(ancho*0.5), colGo(col))
	w("\tc.Esfera(%s, %s, 0, %s, %s) // la cabeza\n", f2(ancho*0.55), f2(base+alto*0.7), f2(ancho*0.3), colGo([3]float64{col[0]*0.7 + 0.3, col[1]*0.7 + 0.3, col[2]*0.7 + 0.3}))
	w("\tc.Brilla(%s, %s, %s, %s, %s)\n\tc.Brilla(%s, %s, %s, %s, %s)\n",
		f2(ancho*0.8), f2(base+alto*0.75), f2(ancho*0.12), f2(ancho*0.06), colGo(ojo),
		f2(ancho*0.8), f2(base+alto*0.75), f2(-ancho*0.12), f2(ancho*0.06), colGo(ojo))
	if vuela {
		w("\tfor _, lado := range []float64{-1, 1} { // alas\n\t\tc.Lamina(0, %s, 0, %s, %s, lado*%s, %s, %s, lado*%s, %s)\n\t}\n",
			f2(base+alto*0.45), f2(-ancho*0.4), f2(base+alto*0.5), f2(ancho*1.6), f2(ancho*0.3), f2(base+alto*0.5), f2(ancho*1.4), colGo(col))
	}
	var caderas [][2]float64
	for i := 0; i < patas; i++ {
		lado := 1.0
		if i%2 == 1 {
			lado = -1
		}
		x := (float64(i/2)/math.Max(1, float64(patas/2-1)) - 0.5) * ancho * 0.9
		if patas == 2 {
			x = 0
		}
		caderas = append(caderas, [2]float64{x, lado * ancho * 0.3})
		w("\tc.Parte(\"pata%d\", %s, %s, %s)\n\tc.Tubo(%s, %s, %s, %s, 0, %s, %s, %s, %s)\n",
			i, f2(x), f2(base+alto*0.3), f2(lado*ancho*0.3),
			f2(x), f2(base+alto*0.3), f2(lado*ancho*0.3), f2(x), f2(lado*ancho*0.45), f2(ancho*0.07), f2(ancho*0.05), colGo([3]float64{col[0] * 0.6, col[1] * 0.6, col[2] * 0.6}))
	}
	w("\tc.Parte(\"\", 0, 0, 0)\n}\n\n")
	w("// Actuar: 10 veces por segundo. Anda, gira, mueve las patas")
	switch caracter {
	case "curiosa":
		w(" y, si te acercas, va hacia ti.\n")
	case "tímida":
		w(" y, si te acercas, huye.\n")
	default:
		w(" a su aire.\n")
	}
	w("func Actuar(o *mundo.Objeto, dt float64) {\n\tx, z, rumbo := o.Valor(\"x\"), o.Valor(\"z\"), o.Valor(\"rumbo\")\n")
	w("\tpx, _, pz := o.Donde() // dónde nació\n\tjx, _, jz, _ := o.Jugador()\n\tdx, dz := jx-(px+x), jz-(pz+z)\n\td := mundo.Hypot(dx, dz)\n")
	switch caracter {
	case "curiosa":
		w("\tif d < 10 && d > 1.5 {\n\t\trumbo = mundo.Atan2(dz, dx) // hacia ti\n\t} else ")
	case "tímida":
		w("\tif d < 6 {\n\t\trumbo = mundo.Atan2(-dz, -dx) // lejos de ti\n\t} else ")
	default:
		w("\t_ = d\n\t")
	}
	w("if o.Azar() < dt*0.4 {\n\t\trumbo += (o.Azar() - 0.5) * 2.5\n\t}\n")
	w("\tnx, nz := x+mundo.Cos(rumbo)*velocidad*dt, z+mundo.Sin(rumbo)*velocidad*dt\n")
	w("\tif mundo.Hypot(nx, nz) > radio { // no se va de su sitio\n\t\trumbo += mundo.Pi\n\t\tnx, nz = x, z\n\t}\n")
	w("\to.Guardar(\"x\", nx)\n\to.Guardar(\"z\", nz)\n\to.Guardar(\"rumbo\", rumbo)\n")
	if vuela {
		w("\to.Mover(\"cuerpo\", nx, %s*mundo.Sin(o.Tiempo()*2), nz)\n", f2(alto*0.3))
	} else {
		w("\to.Mover(\"cuerpo\", nx, 0.04*mundo.Abs(mundo.Sin(o.Tiempo()*velocidad*4)), nz)\n")
	}
	w("\to.Rotar(\"cuerpo\", 0, rumbo, 0) // mira hacia donde va\n")
	if patas > 0 {
		w("\tca, sa := mundo.Cos(rumbo), mundo.Sin(rumbo)\n")
		w("\tpata := func(nombre string, hx, hz, fase float64) { // las patas van con el cuerpo y se balancean\n")
		w("\t\to.Mover(nombre, nx+hx*ca-hz*sa-hx, 0, nz+hx*sa+hz*ca-hz)\n\t\to.Rotar(nombre, 0, rumbo, 0.5*mundo.Sin(o.Tiempo()*velocidad*6+fase))\n\t}\n")
		for i, c := range caderas {
			fase := "0"
			if i%2 == 1 {
				fase = "mundo.Pi"
			}
			w("\tpata(\"pata%d\", %s, %s, %s)\n", i, f2(c[0]), f2(c[1]), fase)
		}
	}
	w("}\n\n")
	w("// si la tocas, dice algo\nfunc AlTocar(o *mundo.Objeto, quien, parte string) {\n\to.Decir(%q)\n\to.Sonar(\"nota\", %s)\n}\n",
		prefijo(strings.Join(palabras, " "), 80), f2(48+s.entre(0, 30)))
	que := fmt.Sprintf("una entidad %s de %d patas", caracter, patas)
	if vuela {
		que = fmt.Sprintf("una entidad %s que vuela", caracter)
	}
	return tallerPrograma{Codigo: b.String(), Que: que}
}

// ProgramarInstrumento: lo que suena con su música (un archivo .wav en
// taller/sonidos). Suena siempre cerca y, si lo tocas o pulsas E, otra vez.
func ProgramarInstrumento(quien, archivo string, palabras []string, sinestesia func(string) ([3]float64, bool)) tallerPrograma {
	s := tallerSembrar(append([]string{archivo}, palabras...))
	col := s.color(sinestesia)
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("// %s compuso «%s» y escribió lo que la toca\npackage objeto\n\nimport \"mundo\"\n\n", quien, archivo)
	forma := s.n(0, 2)
	w("func Construir(c *mundo.Cuerpo) {\n")
	switch forma {
	case 0: // tubos de órgano
		n := s.n(4, 9)
		w("\tfor i := 0; i < %d; i++ { // tubos, de corto a largo\n\t\tc.Cilindro(float64(i)*0.5, 0, 0, 0.2, 1+float64(i)*%s, %s)\n\t}\n", n, f2(s.entre(0.2, 0.5)), colGo(col))
	case 1: // campanas colgando de un arco
		w("\tfor i := 0; i < 5; i++ {\n\t\ta := mundo.Pi * float64(i) / 4\n\t\tc.Cono(mundo.Cos(a)*2, 2+mundo.Sin(a)*1.5, 0, 0.35, 0.6, %s)\n\t}\n\tc.Toro(0, 2, 0, 2, 0.08, %s)\n", colGo(col), colGo([3]float64{0.8, 0.7, 0.3}))
	default: // un tambor
		w("\tc.Cilindro(0, 0, 0, %s, %s, %s)\n\tc.Cilindro(0, %s, 0, %s, 0.05, mundo.RGB(0.95, 0.92, 0.85))\n", f2(s.entre(0.6, 1.4)), f2(s.entre(0.5, 1.2)), colGo(col), f2(s.entre(0.5, 1.2)), f2(s.entre(0.6, 1.4)))
	}
	w("\tc.Brilla(0, %s, 0, 0.15, %s)\n}\n\n", f2(s.entre(2.5, 4)), colGo(col))
	w("func Empezar(o *mundo.Objeto) {\n\to.Ambiente(%q, 0.45) // su música, mientras estés cerca\n}\n\n", archivo)
	w("func AlUsar(o *mundo.Objeto, quien string) {\n\to.Sonar(%q, 1)\n}\n\n", archivo)
	w("func AlTocar(o *mundo.Objeto, quien, parte string) {\n\to.Sonar(%q, 1.5)\n}\n", archivo)
	return tallerPrograma{Codigo: b.String(), Que: "un instrumento que toca «" + archivo + "»"}
}
