package main

// ============================================================
//  TALLER · FORMAS — más herramientas para construir
// ------------------------------------------------------------
//  Lo que Nyx, Abla y tú podéis usar en el código de los objetos
//  y de las entidades (paquete "mundo"), además de Caja, Esfera,
//  Brilla, Tubo y Figura, que ya tenía Nyx Mundo:
//
//    c.Triangulo / c.Lamina / c.Cuadro      caras sueltas (mallas a mano)
//    c.Cilindro / c.Cono / c.Toro           sólidos de revolución sencillos
//    c.Revolucion(x,y,z, perfil, lados, col)  tornear un perfil (jarrones,
//                                             cúpulas, torres, columnas…)
//    c.Extruir(x,y,z, puntos, alto, col)    levantar un polígono (casas,
//                                             muros, estrellas, letras…)
//    c.Superficie(x,y,z, ancho, fondo, alturas, columnas, colores)
//                                           un relieve (montañas, murales)
//    c.Letras(texto, x,y,z, tam, fondo, col)  escribir en 3D (en Abla o en
//                                             español)
//    c.Marca() · c.Copiar(desde, dx,dy,dz, giro) · c.MoverDesde ·
//    c.GirarDesde · c.EscalarDesde          repetir y colocar partes
//    c.Escalar · c.Girar · c.Mover · c.Tenir   todo el cuerpo
//    c.Alto() · c.Ancho()                   cuánto mide lo que ya hay
//    c.MallaCodificada(datos)               un modelo importado (Blender)
//
//  Y funciones: Pow, Floor, Ceil, Mod, Exp, Log, Tan, Asin, Acos,
//  Atan, Round, Tanh, Ruido(x, y, semilla), Mezcla(a, b, t), HSV(h, s, v).
//
//  Todo es nativo (rápido): el código interpretado solo dice QUÉ
//  construir; el trabajo pesado lo hace Go compilado.
// ============================================================

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
)

func init() {
	ex := paqueteMundo["mundo/mundo"]
	for k, v := range map[string]any{
		"Pow": math.Pow, "Floor": math.Floor, "Ceil": math.Ceil, "Mod": math.Mod,
		"Exp": math.Exp, "Log": math.Log, "Tan": math.Tan, "Asin": math.Asin,
		"Acos": math.Acos, "Atan": math.Atan, "Round": math.Round, "Tanh": math.Tanh,
		"Ruido": TallerRuido, "Mezcla": TallerMezcla, "HSV": TallerHSV,
	} {
		ex[k] = reflect.ValueOf(v)
	}
}

// TallerRuido: ruido suave entre 0 y 1 (para relieves, cortezas, rocas).
func TallerRuido(x, y float64, semilla int) float64 {
	h := func(i, j int) float64 {
		n := uint64(int64(i)*374761393+int64(j)*668265263) ^ uint64(int64(semilla)*2246822519)
		n = (n ^ (n >> 13)) * 1274126177
		return float64((n^(n>>16))&0xffff) / 65535
	}
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	i, j := int(x0), int(y0)
	a := h(i, j) + (h(i+1, j)-h(i, j))*fx
	b := h(i, j+1) + (h(i+1, j+1)-h(i, j+1))*fx
	return a + (b-a)*fy
}

// TallerMezcla: un color entre a y b (t de 0 a 1).
func TallerMezcla(a, b Color, t float64) Color {
	return RGB(a.R+(b.R-a.R)*t, a.G+(b.G-a.G)*t, a.B+(b.B-a.B)*t)
}

// TallerHSV: color por tono (0-1, la vuelta del arcoíris), saturación y valor.
func TallerHSV(h, s, v float64) Color {
	h = math.Mod(math.Mod(h, 1)+1, 1) * 6
	i := math.Floor(h)
	f := h - i
	p, q, t := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	switch int(i) {
	case 0:
		return RGB(v, t, p)
	case 1:
		return RGB(q, v, p)
	case 2:
		return RGB(p, v, t)
	case 3:
		return RGB(p, q, v)
	case 4:
		return RGB(t, p, v)
	}
	return RGB(v, p, q)
}

func tallerNormal(a, b, d [3]float64) [3]float64 {
	return normalizar(cruz([3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}, [3]float64{d[0] - a[0], d[1] - a[1], d[2] - a[2]}))
}

// Triangulo: una cara (se ve por el lado desde el que sus puntos van en
// sentido antihorario).
func (c *Cuerpo) Triangulo(x1, y1, z1, x2, y2, z2, x3, y3, z3 float64, col Color) {
	a, b, d := [3]float64{x1, y1, z1}, [3]float64{x2, y2, z2}, [3]float64{x3, y3, z3}
	n := tallerNormal(a, b, d)
	c.tri(a, b, d, n, n, n, col, 3)
}

// Lamina: un triángulo que se ve por los dos lados (hojas, velas, alas).
func (c *Cuerpo) Lamina(x1, y1, z1, x2, y2, z2, x3, y3, z3 float64, col Color) {
	c.Triangulo(x1, y1, z1, x2, y2, z2, x3, y3, z3, col)
	c.Triangulo(x1, y1, z1, x3, y3, z3, x2, y2, z2, col)
}

// Cuadro: cuatro puntos, en orden alrededor.
func (c *Cuerpo) Cuadro(x1, y1, z1, x2, y2, z2, x3, y3, z3, x4, y4, z4 float64, col Color) {
	c.Triangulo(x1, y1, z1, x2, y2, z2, x3, y3, z3, col)
	c.Triangulo(x1, y1, z1, x3, y3, z3, x4, y4, z4, col)
}

// Cilindro: de pie, con la base en (x, y, z).
func (c *Cuerpo) Cilindro(x, y, z, r, alto float64, col Color) {
	c.Tubo(x, y, z, x, y+alto, z, r, r, col)
}

// Cono: de pie, con la base en (x, y, z).
func (c *Cuerpo) Cono(x, y, z, r, alto float64, col Color) {
	c.Tubo(x, y, z, x, y+alto, z, r, 0, col)
}

// Toro: un aro tumbado (radio grande R, grosor r) con centro en (x, y, z).
func (c *Cuerpo) Toro(x, y, z, R, r float64, col Color) {
	const u, v = 24, 10
	p := func(i, j int) ([3]float64, [3]float64) {
		a := float64(i) / u * 2 * math.Pi
		b := float64(j) / v * 2 * math.Pi
		n := [3]float64{math.Cos(b) * math.Cos(a), math.Sin(b), math.Cos(b) * math.Sin(a)}
		return [3]float64{x + (R+r*math.Cos(b))*math.Cos(a), y + r*math.Sin(b), z + (R+r*math.Cos(b))*math.Sin(a)}, n
	}
	for i := 0; i < u; i++ {
		for j := 0; j < v; j++ {
			a, na := p(i, j)
			b, nb := p(i+1, j)
			d, nd := p(i+1, j+1)
			e, ne := p(i, j+1)
			c.tri(a, d, b, na, nd, nb, col, 3)
			c.tri(a, e, d, na, ne, nd, col, 3)
		}
	}
}

// Revolucion: gira un perfil alrededor del eje vertical que pasa por
// (x, y, z). perfil = radio0, altura0, radio1, altura1, … de abajo arriba.
func (c *Cuerpo) Revolucion(x, y, z float64, perfil []float64, lados int, col Color) {
	if lados < 3 {
		lados = 3
	}
	if lados > 64 {
		lados = 64
	}
	n := len(perfil) / 2
	for k := 0; k+1 < n; k++ {
		r1, y1, r2, y2 := perfil[2*k], perfil[2*k+1], perfil[2*k+2], perfil[2*k+3]
		// normal del tramo (hacia fuera)
		dy, dr := y2-y1, r2-r1
		for i := 0; i < lados; i++ {
			a1 := float64(i) / float64(lados) * 2 * math.Pi
			a2 := float64(i+1) / float64(lados) * 2 * math.Pi
			p := func(r, h, a float64) [3]float64 { return [3]float64{x + r*math.Cos(a), y + h, z + r*math.Sin(a)} }
			nn := func(a float64) [3]float64 { return normalizar([3]float64{dy * math.Cos(a), -dr, dy * math.Sin(a)}) }
			c.tri(p(r1, y1, a1), p(r2, y2, a2), p(r1, y1, a2), nn(a1), nn(a2), nn(a2), col, 3)
			c.tri(p(r1, y1, a1), p(r2, y2, a1), p(r2, y2, a2), nn(a1), nn(a1), nn(a2), col, 3)
		}
	}
	// tapas, si el perfil no se cierra solo
	tapa := func(r, h float64, arriba bool) {
		if r <= 0 {
			return
		}
		s := -1.0
		if arriba {
			s = 1
		}
		nv := [3]float64{0, s, 0}
		cen := [3]float64{x, y + h, z}
		for i := 0; i < lados; i++ {
			a1 := float64(i) / float64(lados) * 2 * math.Pi
			a2 := float64(i+1) / float64(lados) * 2 * math.Pi
			p1 := [3]float64{x + r*math.Cos(a1), y + h, z + r*math.Sin(a1)}
			p2 := [3]float64{x + r*math.Cos(a2), y + h, z + r*math.Sin(a2)}
			if arriba {
				c.tri(cen, p2, p1, nv, nv, nv, col, 3)
			} else {
				c.tri(cen, p1, p2, nv, nv, nv, col, 3)
			}
		}
	}
	if n >= 2 {
		tapa(perfil[0], perfil[1], false)
		tapa(perfil[2*n-2], perfil[2*n-1], true)
	}
}

// Extruir: levanta un polígono del suelo. puntos = x0, z0, x1, z1, …
// (alrededor, en cualquier sentido), con la base en y.
func (c *Cuerpo) Extruir(x, y, z float64, puntos []float64, alto float64, col Color) {
	n := len(puntos) / 2
	if n < 3 {
		return
	}
	// sentido: que las caras miren hacia fuera
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += puntos[2*i]*puntos[2*j+1] - puntos[2*j]*puntos[2*i+1]
	}
	px := func(i int) (float64, float64) {
		if area < 0 {
			i = n - 1 - i
		}
		return x + puntos[2*i], z + puntos[2*i+1]
	}
	cx, cz := 0.0, 0.0
	for i := 0; i < n; i++ {
		a, b := px(i)
		cx, cz = cx+a/float64(n), cz+b/float64(n)
	}
	for i := 0; i < n; i++ {
		x1, z1 := px(i)
		x2, z2 := px((i + 1) % n)
		c.Cuadro(x1, y, z1, x1, y+alto, z1, x2, y+alto, z2, x2, y, z2, col)
		c.Triangulo(cx, y+alto, cz, x2, y+alto, z2, x1, y+alto, z1, col)
		c.Triangulo(cx, y, cz, x1, y, z1, x2, y, z2, col)
	}
}

// Superficie: un relieve de ancho × fondo con la esquina en (x, y, z).
// alturas tiene filas de 'columnas' números; colores, uno por punto
// (o uno solo para todo).
func (c *Cuerpo) Superficie(x, y, z, ancho, fondo float64, alturas []float64, columnas int, colores []Color) {
	if columnas < 2 || len(alturas) < columnas*2 || len(colores) == 0 {
		return
	}
	filas := len(alturas) / columnas
	dx, dz := ancho/float64(columnas-1), fondo/float64(filas-1)
	h := func(i, j int) float64 { return alturas[j*columnas+i] }
	col := func(i, j int) Color {
		if len(colores) == 1 {
			return colores[0]
		}
		k := j*columnas + i
		if k >= len(colores) {
			k = len(colores) - 1
		}
		return colores[k]
	}
	p := func(i, j int) [3]float64 { return [3]float64{x + float64(i)*dx, y + h(i, j), z + float64(j)*dz} }
	nor := func(i, j int) [3]float64 {
		l, r, d, u := h(max(i-1, 0), j), h(min(i+1, columnas-1), j), h(i, max(j-1, 0)), h(i, min(j+1, filas-1))
		return normalizar([3]float64{(l - r) / (2 * dx), 1, (d - u) / (2 * dz)})
	}
	for j := 0; j+1 < filas; j++ {
		for i := 0; i+1 < columnas; i++ {
			k := col(i, j)
			c.tri(p(i, j), p(i, j+1), p(i+1, j+1), nor(i, j), nor(i, j+1), nor(i+1, j+1), k, 3)
			c.tri(p(i, j), p(i+1, j+1), p(i+1, j), nor(i, j), nor(i+1, j+1), nor(i+1, j), k, 3)
		}
	}
}

// ---------- partes: marcar, copiar, mover ----------

// Marca: por dónde va el cuerpo (para luego copiar o mover lo que venga).
func (c *Cuerpo) Marca() int { return len(c.Pos) / 3 }

func (c *Cuerpo) transformar(desde int, f func(p, n [3]float64) ([3]float64, [3]float64)) {
	if desde < 0 {
		desde = 0
	}
	for i := desde; i < len(c.Pos)/3; i++ {
		p := [3]float64{float64(c.Pos[i*3]), float64(c.Pos[i*3+1]), float64(c.Pos[i*3+2])}
		n := [3]float64{float64(c.Nor[i*3]), float64(c.Nor[i*3+1]), float64(c.Nor[i*3+2])}
		p, n = f(p, n)
		for k := 0; k < 3; k++ {
			c.Pos[i*3+k], c.Nor[i*3+k] = float32(p[k]), float32(n[k])
		}
	}
}

func giroY(a float64) func(p, n [3]float64) ([3]float64, [3]float64) {
	s, co := math.Sin(a), math.Cos(a)
	g := func(v [3]float64) [3]float64 { return [3]float64{v[0]*co - v[2]*s, v[1], v[0]*s + v[2]*co} }
	return func(p, n [3]float64) ([3]float64, [3]float64) { return g(p), g(n) }
}

// MoverDesde: mueve lo construido desde la marca.
func (c *Cuerpo) MoverDesde(desde int, dx, dy, dz float64) {
	c.transformar(desde, func(p, n [3]float64) ([3]float64, [3]float64) {
		return [3]float64{p[0] + dx, p[1] + dy, p[2] + dz}, n
	})
}

// GirarDesde: gira lo construido desde la marca alrededor del eje vertical.
func (c *Cuerpo) GirarDesde(desde int, angulo float64) { c.transformar(desde, giroY(angulo)) }

// EscalarDesde: agranda (o encoge) lo construido desde la marca.
func (c *Cuerpo) EscalarDesde(desde int, k float64) {
	c.transformar(desde, func(p, n [3]float64) ([3]float64, [3]float64) {
		return [3]float64{p[0] * k, p[1] * k, p[2] * k}, n
	})
}

// LevantarDesde: lo construido tumbado en el suelo (como una Superficie)
// se pone de pie: lo que iba hacia +Z sube (z → y), lo que iba hacia +X
// va hacia +Z, y el relieve mira hacia +X.
func (c *Cuerpo) LevantarDesde(desde int) {
	c.transformar(desde, func(p, n [3]float64) ([3]float64, [3]float64) {
		return [3]float64{p[1], p[2], p[0]}, [3]float64{n[1], n[2], n[0]}
	})
}

// Copiar: repite lo construido desde la marca, girado y desplazado.
func (c *Cuerpo) Copiar(desde int, dx, dy, dz, giro float64) {
	fin := len(c.Pos) / 3
	if desde < 0 || desde >= fin {
		return
	}
	g := giroY(giro)
	for i := desde; i+2 < fin; i += 3 {
		var v [3][3]float64
		var n [3][3]float64
		for k := 0; k < 3; k++ {
			p := [3]float64{float64(c.Pos[(i+k)*3]), float64(c.Pos[(i+k)*3+1]), float64(c.Pos[(i+k)*3+2])}
			nn := [3]float64{float64(c.Nor[(i+k)*3]), float64(c.Nor[(i+k)*3+1]), float64(c.Nor[(i+k)*3+2])}
			p, nn = g(p, nn)
			v[k], n[k] = [3]float64{p[0] + dx, p[1] + dy, p[2] + dz}, nn
		}
		col := Color{float64(c.Col[i*3]), float64(c.Col[i*3+1]), float64(c.Col[i*3+2])}
		c.tri(v[0], v[1], v[2], n[0], n[1], n[2], col, c.Emi[i])
	}
}

// Escalar, Girar, Mover: todo el cuerpo (con sus pivotes y huecos).
func (c *Cuerpo) Escalar(k float64) {
	if k > 0 && k != 1 {
		c.EscalarDesde(0, k)
		tallerTransformarExtra(c, func(p [3]float64) [3]float64 { return [3]float64{p[0] * k, p[1] * k, p[2] * k} })
	}
}
func (c *Cuerpo) Girar(angulo float64) {
	if angulo != 0 {
		c.GirarDesde(0, angulo)
		g := giroY(angulo)
		tallerTransformarExtra(c, func(p [3]float64) [3]float64 { q, _ := g(p, p); return q })
	}
}
func (c *Cuerpo) Mover(dx, dy, dz float64) {
	c.MoverDesde(0, dx, dy, dz)
	tallerTransformarExtra(c, func(p [3]float64) [3]float64 { return [3]float64{p[0] + dx, p[1] + dy, p[2] + dz} })
}

// Tenir: tiñe todo el cuerpo hacia un color (fuerza de 0 a 1). Lo que
// brilla solo también cambia de color.
func (c *Cuerpo) Tenir(col Color, fuerza float64) {
	if fuerza <= 0 {
		return
	}
	f := float32(math.Min(1, fuerza))
	t := [3]float32{float32(col.R), float32(col.G), float32(col.B)}
	for i := range c.Col {
		c.Col[i] += (t[i%3] - c.Col[i]) * f
	}
}

// Alto: la altura de lo construido (lo más alto).
func (c *Cuerpo) Alto() float64 {
	m := 0.0
	for i := 1; i < len(c.Pos); i += 3 {
		m = math.Max(m, float64(c.Pos[i]))
	}
	return m
}

// Ancho: el radio que ocupa en el suelo (desde el centro).
func (c *Cuerpo) Ancho() float64 {
	m := 0.0
	for i := 0; i+2 < len(c.Pos); i += 3 {
		m = math.Max(m, math.Hypot(float64(c.Pos[i]), float64(c.Pos[i+2])))
	}
	return m
}

// ---------- letras en 3D ----------

// tallerFuente: letras de 3×5 (cada fila, 3 bits).
var tallerFuente = map[rune][5]uint8{
	'a': {2, 5, 7, 5, 5}, 'b': {6, 5, 6, 5, 6}, 'c': {3, 4, 4, 4, 3}, 'd': {6, 5, 5, 5, 6},
	'e': {7, 4, 6, 4, 7}, 'f': {7, 4, 6, 4, 4}, 'g': {3, 4, 5, 5, 3}, 'h': {5, 5, 7, 5, 5},
	'i': {7, 2, 2, 2, 7}, 'j': {1, 1, 1, 5, 2}, 'k': {5, 5, 6, 5, 5}, 'l': {4, 4, 4, 4, 7},
	'm': {5, 7, 7, 5, 5}, 'n': {6, 5, 5, 5, 5}, 'ñ': {7, 0, 6, 5, 5}, 'o': {2, 5, 5, 5, 2},
	'p': {6, 5, 6, 4, 4}, 'q': {2, 5, 5, 6, 3}, 'r': {6, 5, 6, 5, 5}, 's': {3, 4, 2, 1, 6},
	't': {7, 2, 2, 2, 2}, 'u': {5, 5, 5, 5, 7}, 'v': {5, 5, 5, 5, 2}, 'w': {5, 5, 7, 7, 5},
	'x': {5, 5, 2, 5, 5}, 'y': {5, 5, 2, 2, 2}, 'z': {7, 1, 2, 4, 7},
	'0': {7, 5, 5, 5, 7}, '1': {2, 6, 2, 2, 7}, '2': {6, 1, 2, 4, 7}, '3': {6, 1, 2, 1, 6},
	'4': {5, 5, 7, 1, 1}, '5': {7, 4, 6, 1, 6}, '6': {3, 4, 7, 5, 7}, '7': {7, 1, 2, 2, 2},
	'8': {7, 5, 7, 5, 7}, '9': {7, 5, 7, 1, 6}, '.': {0, 0, 0, 0, 2}, ',': {0, 0, 0, 2, 4},
	'!': {2, 2, 2, 0, 2}, '?': {6, 1, 2, 0, 2}, '-': {0, 0, 7, 0, 0}, '\'': {2, 2, 0, 0, 0},
}

// Letras: escribe un texto en 3D, de pie, mirando hacia +X, empezando en
// (x, y, z) y avanzando hacia +Z. tam es la altura de una letra.
func (c *Cuerpo) Letras(texto string, x, y, z, tam, fondo float64, col Color) {
	cel := tam / 5
	quitar := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u")
	k := 0
	for _, r := range quitar.Replace(strings.ToLower(texto)) {
		if r == ' ' {
			k += 2
			continue
		}
		g, ok := tallerFuente[r]
		if !ok {
			k += 2
			continue
		}
		for fila := 0; fila < 5; fila++ {
			for b := 0; b < 3; b++ {
				if g[fila]&(4>>b) != 0 {
					c.Caja(x, y+float64(4-fila)*cel+cel/2, z-float64(k*4+b)*cel-cel/2, fondo, cel, cel, col)
				}
			}
		}
		k++
	}
}

// ---------- modelos importados ----------

// MallaCodificada: un modelo guardado como datos (lo escribe el taller al
// importar un .obj de Blender). Formato: base64 de
//
//	6 float32 (mínimo x,y,z y tamaño x,y,z) y, por triángulo,
//	9 uint16 (sus 3 puntos dentro de esa caja), r, g, b y brillo (bytes).
func (c *Cuerpo) MallaCodificada(datos string) {
	b, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(datos), ""))
	if err != nil || len(b) < 24 {
		return
	}
	var caja [6]float64
	for i := range caja {
		caja[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:])))
	}
	const tam = 9*2 + 4
	for o := 24; o+tam <= len(b); o += tam {
		var v [3][3]float64
		for k := 0; k < 9; k++ {
			q := float64(binary.LittleEndian.Uint16(b[o+k*2:])) / 65535
			v[k/3][k%3] = caja[k%3] + q*caja[3+k%3]
		}
		n := tallerNormal(v[0], v[1], v[2])
		col := Color{float64(b[o+18]) / 255, float64(b[o+19]) / 255, float64(b[o+20]) / 255}
		emi := float32(3)
		if b[o+21] > 0 {
			emi = 4
		}
		c.tri(v[0], v[1], v[2], n, n, n, col, emi)
	}
}
