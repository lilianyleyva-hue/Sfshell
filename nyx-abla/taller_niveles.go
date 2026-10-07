package main

// ============================================================
//  TALLER · NIVELES — construir hacia arriba y hacia abajo, con
//  cosas que se mueven, suenan y se juegan
// ------------------------------------------------------------
//  torre-mirador  una escalera de caracol por fuera hasta lo alto
//                 (atraviesa el techo si hace falta) con un faro que gira
//  edificio       pisos de verdad, con escaleras, puerta (E para abrir)
//                 y azotea
//  mazmorra       se baja: escaleras bajo tierra, salas con antorchas,
//                 más niveles hacia abajo y un cofre con tesoro
//  ascensor       una plataforma que sube y baja (E) a una terraza alta
//  anillos        un juego: anillos que giran; quien los toca, puntúa
//                 (también juegan los seres de Abla y Nyx)
//  plataformas    un juego de saltos (Espacio) hasta una corona en lo alto
//  molino         aspas que giran con el viento (y suenan)
//  campanario     una campana que se toca con E (y suena sola a veces)
//  casa           hueca, con puerta que se abre y luz dentro
// ============================================================

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

// Brillante: lo que se construya mientras esté activo da luz propia (y no
// se choca: es luz). Para ventanas, botones, pantallas.
func (c *Cuerpo) Brillante(si bool) {
	x := tallerExtraDe(c)
	if si && x.brilloDesde < 0 {
		x.brilloDesde = c.Marca()
	}
	if !si && x.brilloDesde >= 0 {
		x.brillo = append(x.brillo, [2]int{x.brilloDesde, c.Marca()})
		x.brilloDesde = -1
	}
}

// tallerAplicarBrillo: al terminar de construir, lo que se hizo con
// Brillante(true) brilla.
func tallerAplicarBrillo(c *Cuerpo, x *tallerExtra) {
	if x.brilloDesde >= 0 {
		x.brillo = append(x.brillo, [2]int{x.brilloDesde, len(c.Pos) / 3})
		x.brilloDesde = -1
	}
	for _, r := range x.brillo {
		for v := max(0, r[0]); v < r[1] && v < len(c.Emi); v++ {
			c.Emi[v] = 4
		}
	}
}

// tallerMelodia: una frase en Abla, convertida en música (cada letra, una
// nota de la escala pentatónica; los espacios, silencios).
func tallerMelodia(frase string) string {
	escala := []int{60, 62, 64, 67, 69, 72, 74, 76, 79, 81}
	var notas []string
	for _, r := range strings.ToLower(frase) {
		if len(notas) >= 24 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z':
			notas = append(notas, fmt.Sprint(escala[int(r-'a')%len(escala)]))
		case r == ' ':
			notas = append(notas, "-")
		}
	}
	if len(notas) == 0 {
		return "60 64 67 72"
	}
	return strings.Join(notas, " ")
}

// ---------- hacia arriba ----------

func (e *tallerEsbozo) torreMirador(rng *rand.Rand, pal [][3]float64, alta float64) {
	R := 1.0 + rng.Float64()*0.6
	H := math.Round((8+rng.Float64()*9+alta*6)*5) / 5
	n := int(math.Ceil((H + 0.15) / 0.2))
	e.L("// una torre-mirador de %s m: se sube por la escalera de fuera hasta arriba", f2(H))
	e.L("c.Revolucion(0, 0, 0, []float64{%s, 0, %s, %s}, 18, %s)", f2(R), f2(R*0.97), f2(H+0.15), tallerCol(pal[0]))
	e.L("for k := 0; k < %d; k++ { // los escalones, de 20 cm", n)
	e.L("\ty := float64(k+1) * 0.2")
	e.L("\tm := c.Marca()")
	e.L("\tc.Caja(%s, y-0.06, 0, 1.15, 0.12, 0.64, mundo.Mezcla(%s, %s, float64(k)/%d))", f2(R+0.57), tallerCol(pal[1]), tallerCol(pal[2]), n)
	e.L("\tif k%%3 == 0 {")
	e.L("\t\tc.Cilindro(%s, y, 0, 0.03, 0.95, %s) // la baranda", f2(R+1.1), tallerCol(tallerOscurecer(pal[0], 0.5)))
	e.L("\t}")
	e.L("\tc.GirarDesde(m, float64(k)*0.34)")
	e.L("}")
	e.L("// arriba: un pretil y un faro que gira")
	e.L("for k := 0; k < 12; k++ {")
	e.L("\ta := float64(k) * mundo.Pi / 6")
	e.L("\tif mundo.Abs(mundo.Mod(a-%s+4*mundo.Pi, 2*mundo.Pi)-mundo.Pi) > 2.6 {", f2(math.Mod(float64(n-1)*0.34, 2*math.Pi)))
	e.L("\t\tcontinue // por aquí se llega")
	e.L("\t}")
	e.L("\tc.Cilindro(mundo.Cos(a)*%s, %s, mundo.Sin(a)*%s, 0.04, 0.95, %s)", f2(R-0.08), f2(H+0.15), f2(R-0.08), tallerCol(tallerOscurecer(pal[0], 0.5)))
	e.L("}")
	e.L("c.Parte(\"faro\", 0, %s, 0)", f2(H+0.15))
	e.L("c.Cilindro(0, %s, 0, 0.07, 1.5, %s)", f2(H+0.15), tallerCol(tallerOscurecer(pal[0], 0.4)))
	e.L("c.Brilla(0, %s, 0, 0.3, %s)", f2(H+1.85), tallerCol(tallerAclarar(pal[2], 0.7)))
	e.L("c.Fantasma(true)")
	e.L("c.Caja(0.45, %s, 0, 0.6, 0.18, 0.18, %s)", f2(H+1.85), tallerCol(tallerOscurecer(pal[1], 0.3)))
	e.L("c.Fantasma(false)")
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `o.AmbienteEn("faro", "viento", 0.35)`)
	e.S("Actuar", `o.Rotar("faro", 0, o.Tiempo()*1.2, 0)`)
}

func (e *tallerEsbozo) edificio(rng *rand.Rand, pal [][3]float64, oscuro bool) {
	N := 2 + rng.Intn(3)
	W := 7 + rng.Float64()*3
	D := 6.6 + rng.Float64()*2.4
	const H, run = 3.0, 0.32
	x0l, x1l := W/2-1.45, W/2-0.2
	lc, lw := (x0l+x1l)/2, x1l-x0l
	zs := -D/2 + 0.6
	ze := zs + 12*run
	alto := float64(N)*H + 0.15
	pared, suelo, techo := tallerCol(pal[0]), tallerCol(pal[1]), tallerCol(pal[2])
	e.L("// un edificio de %d pisos con escaleras y azotea (atraviesa el techo si es más alto)", N)
	e.L("c.Caja(0, 0.075, 0, %s, 0.15, %s, %s) // el suelo de abajo", f2(W), f2(D), suelo)
	e.L("for k := 1; k <= %d; k++ { // los pisos (con el hueco de la escalera)", N)
	e.L("\ty := float64(k)*%s + 0.075", f2(H))
	e.L("\tc.Caja(%s, y, 0, %s, 0.15, %s, %s)", f2((-W/2+x0l)/2), f2(x0l+W/2), f2(D), suelo)
	e.L("\tc.Caja(%s, y, %s, %s, 0.15, %s, %s)", f2(lc), f2((ze+0.05+D/2)/2), f2(lw+0.1), f2(D/2-ze-0.05), suelo)
	e.L("\tc.Caja(%s, y, %s, %s, 0.15, %s, %s)", f2(lc), f2((-D/2+zs-0.05)/2), f2(lw+0.1), f2(zs-0.05+D/2), suelo)
	e.L("}")
	e.L("for k := 0; k < %d; k++ { // las escaleras: 12 escalones por piso", N)
	e.L("\tfor i := 1; i <= 12; i++ {")
	e.L("\t\tarriba := float64(k)*%s + 0.15 + float64(i)*0.25", f2(H))
	e.L("\t\tc.Caja(%s, arriba-0.125, %s+(float64(i)-0.5)*%s, %s, 0.25, %s, %s)", f2(lc), f2(zs), f2(run), f2(lw), f2(run+0.02), techo)
	e.L("\t}")
	e.L("}")
	dx := -W / 4
	e.L("// las paredes (la de delante, con la puerta)")
	e.L("c.Caja(0, %s, %s, %s, %s, 0.2, %s)", f2(alto/2), f2(D/2), f2(W), f2(alto), pared)
	e.L("c.Caja(%s, %s, 0, 0.2, %s, %s, %s)", f2(-W/2), f2(alto/2), f2(alto), f2(D), pared)
	e.L("c.Caja(%s, %s, 0, 0.2, %s, %s, %s)", f2(W/2), f2(alto/2), f2(alto), f2(D), pared)
	e.L("c.Caja(%s, %s, %s, %s, %s, 0.2, %s)", f2((-W/2+dx-0.65)/2), f2(H/2), f2(-D/2), f2(dx-0.65+W/2), f2(H), pared)
	e.L("c.Caja(%s, %s, %s, %s, %s, 0.2, %s)", f2((W/2+dx+0.65)/2), f2(H/2), f2(-D/2), f2(W/2-dx-0.65), f2(H), pared)
	e.L("c.Caja(%s, %s, %s, 1.3, %s, 0.2, %s)", f2(dx), f2((2.45+H)/2), f2(-D/2), f2(H-2.45), pared)
	e.L("c.Caja(0, %s, %s, %s, %s, 0.2, %s)", f2(H+(alto-H)/2), f2(-D/2), f2(W), f2(alto-H), pared)
	e.L("// la azotea: un pretil para no caerse")
	for _, p := range [][4]float64{{0, D/2 - 0.1, W, 0.2}, {0, -D/2 + 0.1, W, 0.2}, {-W/2 + 0.1, 0, 0.2, D}, {W/2 - 0.1, 0, 0.2, D}} {
		e.L("c.Caja(%s, %s, %s, %s, 1.0, %s, %s)", f2(p[0]), f2(alto+0.5), f2(p[1]), f2(p[2]), f2(p[3]), pared)
	}
	luz := tallerAclarar(pal[2], 0.6)
	if oscuro {
		luz = [3]float64{1, 0.85, 0.55}
	}
	e.L("c.Brillante(true) // ventanas y luces")
	e.L("for k := 0; k < %d; k++ {", N)
	e.L("\ty := float64(k)*%s + 1.6", f2(H))
	e.L("\tc.Caja(%s, y, %s, 1.1, 1.0, 0.24, %s)", f2(-W/4), f2(D/2), tallerCol(luz))
	e.L("\tc.Caja(%s, y, %s, 1.1, 1.0, 0.24, %s)", f2(W/5), f2(D/2), tallerCol(luz))
	e.L("\tc.Caja(%s, y, 0, 0.24, 1.0, 1.4, %s)", f2(-W/2), tallerCol(luz))
	e.L("\tc.Caja(%s, y+1.25, 0, 0.6, 0.06, 0.6, %s)", f2(-W/4), tallerCol(luz))
	e.L("}")
	e.L("c.Brillante(false)")
	e.L("// la puerta: gira sobre su bisagra")
	e.L("c.Parte(\"puerta\", %s, 0.15, %s)", f2(dx-0.65), f2(-D/2))
	e.L("c.Caja(%s, 1.3, %s, 1.28, 2.3, 0.08, %s)", f2(dx), f2(-D/2), tallerCol(tallerOscurecer(pal[2], 0.45)))
	e.L("c.Esfera(%s, 1.25, %s, 0.06, mundo.RGB(0.85, 0.75, 0.35))", f2(dx+0.45), f2(-D/2-0.07))
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `if o.Valor("abierta") == 1 {
	o.Rotar("puerta", 0, -1.45, 0)
}`)
	if oscuro {
		e.S("Empezar", `o.Ambiente("zumbido", 0.15)`)
	}
	e.S("AlUsar", `a := 0.0
if o.Valor("abierta") == 0 {
	a = -1.45
	o.Guardar("abierta", 1)
} else {
	o.Guardar("abierta", 0)
}
o.Rotar("puerta", 0, a, 0)
o.SonarEn("puerta", "puerta", 1)`)
}

// ---------- hacia abajo ----------

func (e *tallerEsbozo) mazmorra(rng *rand.Rand, pal [][3]float64, niveles int) {
	if niveles < 1 {
		niveles = 1 + rng.Intn(3)
	}
	const pasos, run, rise, ancho, techo = 16, 0.35, 0.25, 1.6, 3.2
	largo := pasos * run
	salaL, salaW := 7+rng.Float64()*3, 6+rng.Float64()*3
	piedra, oscura := tallerCol(pal[0]), tallerCol(tallerOscurecer(pal[0], 0.35))
	e.L("// una mazmorra de %d nivel(es) hacia abajo, con un tesoro al fondo", niveles)
	e.L("c.Hueco(0, %s, %s, %s) // aquí el suelo del mundo se abre", f2(-ancho/2), f2(largo), f2(ancho/2))
	e.L("c.Fantasma(true)")
	e.L("c.Caja(%s, 0.03, 0, %s, 0.02, %s, mundo.RGB(0.02, 0.02, 0.03)) // la boca, oscura", f2(largo/2+0.6), f2(largo-1.2), f2(ancho))
	e.L("c.Fantasma(false)")
	X := 0.0
	for lvl := 1; lvl <= niveles; lvl++ {
		top, fondo := -4*float64(lvl-1), -4*float64(lvl)
		e.L("// nivel %d: la escalera (baja 4 m) y su túnel", -lvl)
		e.L("for i := 1; i <= %d; i++ {", pasos)
		e.L("\tarriba := %s - float64(i)*%s", f2(top), f2(rise))
		e.L("\tx := %s + (float64(i)-0.5)*%s", f2(X), f2(run))
		e.L("\tc.Caja(x, (arriba+(%s))/2, 0, %s, arriba-(%s), %s, mundo.Mezcla(%s, %s, float64(i)/%d))", f2(fondo-0.3), f2(run+0.02), f2(fondo-0.3), f2(ancho), piedra, oscura, pasos)
		e.L("\tif i%%2 == 1 {")
		e.L("\t\tc.Caja(x+%s, arriba+2.8, 0, %s, 0.2, %s, %s) // el techo del túnel", f2(run/2), f2(run*2+0.02), f2(ancho+0.4), oscura)
		e.L("\t}")
		e.L("}")
		for _, s := range []float64{-1, 1} {
			e.L("c.Caja(%s, %s, %s, %s, %s, 0.2, %s)", f2(X+largo/2), f2((top+3+fondo-0.3)/2), f2(s*(ancho/2+0.1)), f2(largo), f2(top+3-fondo+0.3), piedra)
		}
		xs := X + largo
		cx := xs + salaL/2
		e.L("// la sala del nivel %d", -lvl)
		e.L("c.Caja(%s, %s, 0, %s, 0.3, %s, %s)", f2(cx), f2(fondo-0.15), f2(salaL+0.4), f2(salaW+0.4), oscura)
		e.L("c.Caja(%s, %s, 0, %s, 0.2, %s, %s)", f2(cx), f2(fondo+techo+0.1), f2(salaL+0.4), f2(salaW+0.4), oscura)
		h := techo + 0.5
		yc := fondo + techo/2
		for _, s := range []float64{-1, 1} {
			e.L("c.Caja(%s, %s, %s, %s, %s, 0.2, %s)", f2(cx), f2(yc), f2(s*salaW/2), f2(salaL), f2(h), piedra)
		}
		// las dos paredes de los extremos, con la puerta del túnel
		for k, x := range []float64{xs, xs + salaL} {
			if k == 1 && lvl == niveles {
				e.L("c.Caja(%s, %s, 0, 0.2, %s, %s, %s)", f2(x), f2(yc), f2(h), f2(salaW), piedra)
				continue
			}
			lado := (salaW/2 - ancho/2 - 0.1)
			for _, s := range []float64{-1, 1} {
				e.L("c.Caja(%s, %s, %s, 0.2, %s, %s, %s)", f2(x), f2(yc), f2(s*(ancho/2+0.1+lado/2)), f2(h), f2(lado), piedra)
			}
			e.L("c.Caja(%s, %s, 0, 0.2, %s, %s, %s)", f2(x), f2(fondo+2.85+(techo-2.65)/2), f2(techo-2.65), f2(ancho+0.2), piedra)
		}
		e.L("c.Parte(\"sala%d\", %s, %s, 0)", lvl, f2(cx), f2(fondo))
		e.L("for k := 0; k < 4; k++ { // antorchas")
		e.L("\tx := %s + float64(k/2)*%s", f2(xs+1.5), f2(salaL-3))
		e.L("\tz := %s * float64(1-2*(k%%2))", f2(salaW/2-0.25))
		e.L("\tc.Caja(x, %s, z, 0.12, 0.4, 0.12, mundo.RGB(0.3, 0.2, 0.12))", f2(fondo+1.75))
		e.L("\tc.Brilla(x, %s, z, 0.13, mundo.RGB(1, 0.6, 0.2))", f2(fondo+2.05))
		e.L("}")
		e.L("c.Parte(\"\", 0, 0, 0)")
		e.S("Empezar", fmt.Sprintf(`o.AmbienteEn("sala%d", "fuego", 0.35)`, lvl))
		if rng.Float64() < 0.5 {
			e.L("c.Cilindro(%s, %s, %s, 0.3, %s, %s) // una columna", f2(cx), f2(fondo), f2((rng.Float64()-0.5)*(salaW-2.5)), f2(techo), piedra)
		}
		if lvl == niveles {
			bx := xs + salaL - 1.5
			e.L("// el cofre (E para abrirlo)")
			e.L("c.Caja(%s, %s, 0, 0.9, 0.6, 1.3, mundo.RGB(0.42, 0.26, 0.12))", f2(bx), f2(fondo+0.3))
			e.L("c.Brilla(%s, %s, 0, 0.16, mundo.RGB(1, 0.85, 0.3))", f2(bx), f2(fondo+0.55))
			e.L("c.Parte(\"tapa\", %s, %s, 0)", f2(bx-0.45), f2(fondo+0.6))
			e.L("c.Caja(%s, %s, 0, 0.92, 0.16, 1.32, mundo.RGB(0.5, 0.31, 0.15))", f2(bx), f2(fondo+0.68))
			e.L("c.Parte(\"\", 0, 0, 0)")
			e.S("Empezar", fmt.Sprintf(`o.AmbienteEn("sala%d", "agua", 0.15)
if o.Valor("abierto") == 1 {
	o.Rotar("tapa", 0, 0, 1.2)
}`, lvl))
			e.S("AlUsar", `if o.Valor("abierto") == 0 {
	o.Guardar("abierto", 1)
	o.Rotar("tapa", 0, 0, 1.2)
	o.SonarEn("tapa", "magia", 1)
	o.Puntos(quien, 10)
	o.Decir(quien + " encontró el tesoro del fondo de la mazmorra (+10)")
} else {
	o.Guardar("abierto", 0)
	o.Rotar("tapa", 0, 0, 0)
	o.SonarEn("tapa", "puerta", 0.7)
}`)
		}
		X = xs + salaL
	}
	e.S("Empezar", `o.Ambiente("viento", 0.15)`)
}

func (e *tallerEsbozo) ascensor(rng *rand.Rand, pal [][3]float64, alta float64) {
	H := math.Round(6 + rng.Float64()*7 + alta*5)
	metal, piso := tallerCol(tallerOscurecer(pal[0], 0.3)), tallerCol(pal[1])
	e.L("// un ascensor de %s m: E para subir o bajar", f2(H))
	for _, p := range [][2]float64{{-1.15, -1.15}, {-1.15, 1.15}, {1.15, -1.15}, {1.15, 1.15}} {
		e.L("c.Cilindro(%s, 0, %s, 0.08, %s, %s)", f2(p[0]), f2(p[1]), f2(H+2.4), metal)
	}
	e.L("c.Caja(0, %s, 0, 2.6, 0.2, 2.6, %s) // arriba", f2(H+2.5), metal)
	e.L("// la terraza de arriba")
	e.L("c.Caja(2.55, %s, 0, 3.0, 0.15, 2.0, %s)", f2(H+0.075), piso)
	e.L("c.Caja(4.0, %s, 0, 0.1, 1.0, 2.0, %s)", f2(H+0.65), metal)
	e.L("c.Caja(2.55, %s, 1.0, 3.0, 1.0, 0.1, %s)", f2(H+0.65), metal)
	e.L("c.Caja(2.55, %s, -1.0, 3.0, 1.0, 0.1, %s)", f2(H+0.65), metal)
	e.L("c.Cilindro(3.9, 0, 0.9, 0.08, %s, %s)", f2(H), metal)
	e.L("c.Cilindro(3.9, 0, -0.9, 0.08, %s, %s)", f2(H), metal)
	e.L("c.Parte(\"plataforma\", 0, 0, 0)")
	e.L("c.Caja(0, 0.075, 0, 2.0, 0.15, 2.0, %s)", piso)
	e.L("c.Caja(-0.95, 0.65, 0, 0.08, 1.0, 1.9, %s)", metal)
	e.L("c.Caja(0, 0.65, 0.95, 1.9, 1.0, 0.08, %s)", metal)
	e.L("c.Caja(0, 0.65, -0.95, 1.9, 1.0, 0.08, %s)", metal)
	e.L("c.Brilla(-0.85, 1.2, 0, 0.07, mundo.RGB(1, 0.3, 0.2)) // el botón")
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `o.Guardar("y", 0)
o.Guardar("meta", 0)
o.Guardar("iba", 0)`)
	e.S("Actuar", `y := o.Valor("y")
meta := o.Valor("meta")
if y < meta {
	y = mundo.Min(meta, y+1.4*dt)
} else if y > meta {
	y = mundo.Max(meta, y-1.4*dt)
}
anda := mundo.Abs(y-meta) > 0.001
if anda {
	o.AmbienteEn("plataforma", "zumbido", 0.35)
	o.Guardar("iba", 1)
} else {
	o.AmbienteEn("plataforma", "zumbido", 0)
	if o.Valor("iba") == 1 {
		o.SonarEn("plataforma", "campana", 2)
	}
	o.Guardar("iba", 0)
}
o.Guardar("y", y)
o.Mover("plataforma", 0, y, 0)`)
	e.S("AlUsar", fmt.Sprintf(`if o.Valor("meta") > 0 {
	o.Guardar("meta", 0)
} else {
	o.Guardar("meta", %s)
}
o.SonarEn("plataforma", "nota", 72)`, f2(H)))
}

// ---------- juegos ----------

func (e *tallerEsbozo) anillos(rng *rand.Rand, pal [][3]float64) {
	n := 6 + rng.Intn(5)
	R := 4 + rng.Float64()*3
	oro := tallerAclarar(tallerVivo(pal[2]), 0.35)
	e.L("// un juego: %d anillos que giran; quien los toca, puntúa", n)
	e.L("c.Revolucion(0, 0, 0, []float64{0.45, 0, 0.35, 0.9, 0.2, 1.0}, 12, %s)", tallerCol(pal[0]))
	e.L("c.Brilla(0, 1.2, 0, 0.18, %s)", tallerCol(oro))
	e.L("c.Letras(\"anillos\", 0.46, 0.25, 0.75, 0.25, 0.04, %s)", tallerCol(tallerAclarar(pal[1], 0.5)))
	for i := 0; i < n; i++ {
		a := float64(i)*2*math.Pi/float64(n) + rng.Float64()*0.4
		r := R * (0.6 + 0.4*rng.Float64())
		x, z, y := math.Cos(a)*r, math.Sin(a)*r, 0.9+rng.Float64()*0.8
		e.L("c.Parte(\"anillo%d\", %s, %s, %s)", i, f2(x), f2(y), f2(z))
		e.L("c.Fantasma(true)")
		e.L("c.Toro(%s, %s, %s, 0.42, 0.07, %s)", f2(x), f2(y), f2(z), tallerCol(oro))
		e.L("c.Brilla(%s, %s, %s, 0.1, %s)", f2(x), f2(y), f2(z), tallerCol(tallerAclarar(oro, 0.5)))
		e.L("c.Fantasma(false)")
	}
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `o.Guardar("cogidos", 0)
o.Guardar("reinicio", 0)`)
	e.S("Actuar", `t := o.Tiempo()
for i, p := range o.Partes() {
	o.Rotar(p, 1.5708, t*1.6+float64(i), 0)
}
if r := o.Valor("reinicio"); r > 0 && t > r {
	for _, p := range o.Partes() {
		o.Mostrar(p, true)
	}
	o.Guardar("cogidos", 0)
	o.Guardar("reinicio", 0)
	o.Sonar("magia", 1)
}`)
	e.S("AlTocar", fmt.Sprintf(`if len(parte) < 6 || parte[:6] != "anillo" {
	return
}
o.Mostrar(parte, false)
o.Puntos(quien, 1)
o.SonarEn(parte, "moneda", 1)
n := o.Valor("cogidos") + 1
o.Guardar("cogidos", n)
if n >= %d {
	o.Sonar("victoria", 1)
	o.Decir(quien + " ha cogido el último anillo")
	o.Guardar("reinicio", o.Tiempo()+20)
}`, n))
}

func (e *tallerEsbozo) plataformas(rng *rand.Rand, pal [][3]float64) {
	n := 8 + rng.Intn(6)
	const R, da, rise = 3.2, 0.78, 0.65
	e.L("// un juego de saltos (Espacio): %d plataformas hasta la corona", n)
	e.L("c.Cilindro(0, 0, 0, 0.3, %s, %s)", f2(0.5+float64(n)*rise+1.5), tallerCol(tallerOscurecer(pal[0], 0.3)))
	var tx, ty, tz float64
	for i := 0; i < n; i++ {
		a := float64(i) * da
		x, y, z := math.Cos(a)*R, 0.5+float64(i)*rise, math.Sin(a)*R
		tx, ty, tz = x, y, z
		if i > 2 && rng.Float64() < 0.3 {
			e.L("c.Parte(\"movil%d\", %s, %s, %s)", i, f2(x), f2(y), f2(z))
		}
		e.L("c.Caja(%s, %s, %s, 1.5, 0.24, 1.5, mundo.HSV(%s, 0.55, 0.85))", f2(x), f2(y-0.12), f2(z), f2(float64(i)/float64(n)))
		e.L("c.Parte(\"\", 0, 0, 0)")
	}
	e.L("c.Parte(\"corona\", %s, %s, %s)", f2(tx), f2(ty+1.0), f2(tz))
	e.L("c.Fantasma(true)")
	e.L("c.Toro(%s, %s, %s, 0.35, 0.06, mundo.RGB(1, 0.85, 0.3))", f2(tx), f2(ty+1.0), f2(tz))
	e.L("c.Brilla(%s, %s, %s, 0.16, mundo.RGB(1, 0.95, 0.6))", f2(tx), f2(ty+1.0), f2(tz))
	e.L("c.Fantasma(false)")
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Actuar", `t := o.Tiempo()
for i, p := range o.Partes() {
	if len(p) > 5 && p[:5] == "movil" {
		o.Mover(p, mundo.Sin(t*0.9+float64(i))*0.8, 0, 0)
	}
}
o.Rotar("corona", 0, t*2, 0)
if r := o.Valor("reinicio"); r > 0 && t > r {
	o.Mostrar("corona", true)
	o.Guardar("reinicio", 0)
}`)
	e.S("AlTocar", `if parte == "corona" {
	o.Mostrar("corona", false)
	o.Puntos(quien, 5)
	o.SonarEn("corona", "victoria", 1)
	o.Decir(quien + " ha llegado a lo más alto (+5)")
	o.Guardar("reinicio", o.Tiempo()+30)
}`)
	e.S("Empezar", `o.Guardar("reinicio", 0)`)
}

// ---------- cosas que se mueven y suenan ----------

func (e *tallerEsbozo) molino(rng *rand.Rand, pal [][3]float64) {
	H := 7 + rng.Float64()*5
	L := 3.5 + rng.Float64()*1.5
	e.L("// un molino: las aspas giran con el viento")
	e.L("c.Revolucion(0, 0, 0, []float64{1.6, 0, 1.2, %s, 0.01, %s}, 10, %s)", f2(H), f2(H+1.3), tallerCol(pal[0]))
	e.L("c.Caja(1.5, 1.1, 0, 0.1, 2.0, 0.9, %s) // la puerta", tallerCol(tallerOscurecer(pal[2], 0.5)))
	e.L("c.Parte(\"aspas\", 1.35, %s, 0)", f2(H-0.6))
	e.L("c.Fantasma(true)")
	e.L("for k := 0; k < 4; k++ {")
	e.L("\ta := float64(k) * mundo.Pi / 2")
	e.L("\tca, sa := mundo.Cos(a), mundo.Sin(a)")
	e.L("\ty, z := %s, 0.0", f2(H-0.6))
	e.L("\tc.Cuadro(1.35, y-sa*0.12, z+ca*0.12, 1.35, y+ca*%s-sa*0.4, z+sa*%s+ca*0.4, 1.35, y+ca*%s+sa*0.4, z+sa*%s-ca*0.4, 1.35, y+sa*0.12, z-ca*0.12, %s)", f2(L), f2(L), f2(L), f2(L), tallerCol(tallerAclarar(pal[1], 0.3)))
	e.L("\tc.Cuadro(1.35, y+sa*0.12, z-ca*0.12, 1.35, y+ca*%s+sa*0.4, z+sa*%s-ca*0.4, 1.35, y+ca*%s-sa*0.4, z+sa*%s+ca*0.4, 1.35, y-sa*0.12, z+ca*0.12, %s)", f2(L), f2(L), f2(L), f2(L), tallerCol(tallerAclarar(pal[1], 0.3)))
	e.L("}")
	e.L("c.Esfera(1.4, %s, 0, 0.25, %s)", f2(H-0.6), tallerCol(tallerOscurecer(pal[0], 0.4)))
	e.L("c.Fantasma(false)")
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `o.AmbienteEn("aspas", "viento", 0.4)`)
	e.S("Actuar", `o.Rotar("aspas", o.Tiempo()*1.1, 0, 0)`)
}

func (e *tallerEsbozo) campanario(rng *rand.Rand, pal [][3]float64) {
	H := 6 + rng.Float64()*4
	W := 2.2
	piedra := tallerCol(pal[0])
	e.L("// un campanario: tira de la cuerda (E) y suena")
	e.L("c.Caja(0, %s, 0, %s, %s, %s, %s)", f2((H-2.5)/2), f2(W), f2(H-2.5), f2(W), piedra)
	for _, p := range [][2]float64{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
		e.L("c.Caja(%s, %s, %s, 0.3, 2.5, 0.3, %s)", f2(p[0]*(W/2-0.15)), f2(H-1.25), f2(p[1]*(W/2-0.15)), piedra)
	}
	e.L("c.Caja(0, %s, 0, %s, 0.2, %s, %s)", f2(H+0.1), f2(W+0.3), f2(W+0.3), piedra)
	e.L("c.Revolucion(0, %s, 0, []float64{%s, 0, 0.01, 1.8}, 4, %s)", f2(H+0.2), f2(W*0.8), tallerCol(pal[1]))
	e.L("c.Parte(\"campana\", 0, %s, 0)", f2(H-0.2))
	e.L("c.Revolucion(0, %s, 0, []float64{0.55, 0, 0.5, 0.15, 0.3, 0.7, 0.25, 1.0, 0.05, 1.2}, 16, mundo.RGB(0.72, 0.52, 0.22))", f2(H-1.4))
	e.L("c.Parte(\"cuerda\", %s, 0, 0)", f2(W/2+0.35))
	e.L("c.Cilindro(%s, 0.5, 0, 0.025, %s, mundo.RGB(0.6, 0.5, 0.35))", f2(W/2+0.35), f2(H-1.5))
	e.L("c.Brilla(%s, 1.0, 0, 0.07, mundo.RGB(1, 0.8, 0.4))", f2(W/2+0.35))
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `o.Guardar("toque", -100)
o.Guardar("auto", 0)`)
	e.S("Actuar", `if o.Tiempo()-o.Valor("auto") > 150 {
	o.Guardar("auto", o.Tiempo())
	o.Guardar("toque", o.Tiempo())
	o.SonarEn("campana", "campana", 0.8)
}
t := o.Tiempo() - o.Valor("toque")
a := 0.0
if t < 7 {
	a = mundo.Sin(t*5) * 0.5 * mundo.Exp(-t*0.6)
}
o.Rotar("campana", a, 0, 0)`)
	e.S("AlUsar", `o.Guardar("toque", o.Tiempo())
o.SonarEn("campana", "campana", 1)`)
}
