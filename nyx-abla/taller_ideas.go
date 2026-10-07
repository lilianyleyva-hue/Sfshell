package main

// ============================================================
//  TALLER · IDEAS — cómo escriben código Nyx y Abla
// ------------------------------------------------------------
//  Cada una escribe programas en Go que construyen cosas en 3D, a
//  partir de lo que recuerda:
//
//   · Nyx, de sus recuerdos de sitios (los colores de paredes, suelo
//     y techo, si había cielo, agua, plantas, oscuridad, techos altos,
//     y las cosas que recortó de lo que vio).
//   · Abla, de las fotos que miró (sus colores reales, sus bordes y
//     los conceptos que le evocaron), con el estilo de cada tribu:
//     lenguaje escribe (estelas con frases en Abla), data mide (murales
//     en relieve hechos con la foto misma) y lógica calcula (fractales,
//     espirales, escaleras).
//
//  Las formas se combinan al azar: nunca sale dos veces lo mismo.
// ============================================================

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// tallerEsbozo: un programa a medio escribir.
type tallerEsbozo struct {
	Titulo   string
	ayudas   []string // funciones de apoyo (fuera de Construir)
	cuerpo   strings.Builder
	cabecera string
	puestas  map[string]bool
	scripts  map[string][]string // Empezar, Actuar, AlUsar… → trozos de código
}

// las funciones que puede tener una pieza viva (el motor las llama)
var tallerFirmas = []struct{ nombre, firma string }{
	{"Empezar", "func Empezar(o *mundo.Objeto) {"},
	{"Actuar", "func Actuar(o *mundo.Objeto, dt float64) {"},
	{"AlUsar", "func AlUsar(o *mundo.Objeto, quien string) {"},
	{"AlEntrar", "func AlEntrar(o *mundo.Objeto, quien string) {"},
	{"AlSalir", "func AlSalir(o *mundo.Objeto, quien string) {"},
	{"AlTocar", "func AlTocar(o *mundo.Objeto, quien, parte string) {"},
}

// S: un trozo de script (va en su propio bloque, para que sus variables
// no choquen con las de otro trozo).
func (e *tallerEsbozo) S(funcion, codigo string) {
	if e.scripts == nil {
		e.scripts = map[string][]string{}
	}
	e.scripts[funcion] = append(e.scripts[funcion], codigo)
}

func (e *tallerEsbozo) L(formato string, a ...any) {
	fmt.Fprintf(&e.cuerpo, "\t"+formato+"\n", a...)
}

func (e *tallerEsbozo) ayuda(nombre, codigo string) {
	if e.puestas == nil {
		e.puestas = map[string]bool{}
	}
	if !e.puestas[nombre] {
		e.puestas[nombre] = true
		e.ayudas = append(e.ayudas, codigo)
	}
}

func (e *tallerEsbozo) Codigo() string {
	var b strings.Builder
	b.WriteString(e.cabecera)
	b.WriteString("package objeto\n\nimport \"mundo\"\n\n" + tallerParametros)
	for _, a := range e.ayudas {
		b.WriteString("\n" + a)
	}
	for _, f := range tallerFirmas {
		trozos := e.scripts[f.nombre]
		if len(trozos) == 0 {
			continue
		}
		b.WriteString("\n" + f.firma + "\n")
		for _, t := range trozos {
			b.WriteString("\t{\n")
			for _, l := range strings.Split(strings.TrimRight(t, "\n"), "\n") {
				b.WriteString("\t\t" + strings.TrimLeft(l, "\t ") + "\n")
			}
			b.WriteString("\t}\n")
		}
		b.WriteString("}\n")
	}
	b.WriteString("\nfunc Construir(c *mundo.Cuerpo) {\n")
	b.WriteString(e.cuerpo.String())
	b.WriteString(tallerCierre)
	return b.String()
}

// tallerCol: un color como código.
func tallerCol(c [3]float64) string {
	return fmt.Sprintf("mundo.RGB(%s, %s, %s)", f2(clamp01(c[0])), f2(clamp01(c[1])), f2(clamp01(c[2])))
}

func tallerAclarar(c [3]float64, k float64) [3]float64 {
	return [3]float64{c[0] + (1-c[0])*k, c[1] + (1-c[1])*k, c[2] + (1-c[2])*k}
}

func tallerOscurecer(c [3]float64, k float64) [3]float64 {
	return [3]float64{c[0] * (1 - k), c[1] * (1 - k), c[2] * (1 - k)}
}

// tallerVivo: un color que se vea (si es casi negro, lo aclara).
func tallerVivo(c [3]float64) [3]float64 {
	if lumDe(c) < 0.12 {
		return tallerAclarar(c, 0.25)
	}
	return c
}

// ---------- las formas que saben hacer ----------
// Cada una añade código a Construir. pal: sus colores (al menos 3).

func (e *tallerEsbozo) torre(rng *rand.Rand, pal [][3]float64, alta float64) {
	pisos := 3 + rng.Intn(5)
	r := 0.8 + rng.Float64()*1.2
	h := (1.6 + rng.Float64()*1.4) * (0.8 + alta)
	lados := []int{4, 6, 8, 12, 24}[rng.Intn(5)]
	e.L("// una torre de %d pisos", pisos)
	e.L("y := 0.0")
	e.L("r := %s", f2(r))
	e.L("for p := 0; p < %d; p++ {", pisos)
	e.L("\tcol := mundo.Mezcla(%s, %s, float64(p)/%d)", tallerCol(pal[0]), tallerCol(pal[1]), pisos)
	e.L("\tc.Revolucion(0, y, 0, []float64{r, 0, r*%s, %s, r*%s, %s}, %d, col)", f2(0.9+rng.Float64()*0.1), f2(h*0.9), f2(0.95), f2(h), lados)
	e.L("\tfor v := 0; v < %d; v++ { // ventanas", lados)
	e.L("\t\ta := (float64(v) + 0.5) / %d * 2 * mundo.Pi", lados)
	e.L("\t\tc.Brilla(mundo.Cos(a)*r*0.97, y+%s, mundo.Sin(a)*r*0.97, %s, %s)", f2(h*0.55), f2(math.Min(0.18, r*0.12)), tallerCol(tallerAclarar(pal[2], 0.5)))
	e.L("\t}")
	e.L("\tc.Toro(0, y+%s, 0, r*0.98, 0.06, %s)", f2(h), tallerCol(tallerOscurecer(pal[0], 0.3)))
	e.L("\ty += %s", f2(h))
	e.L("\tr *= %s", f2(0.78+rng.Float64()*0.17))
	e.L("}")
	switch rng.Intn(3) {
	case 0:
		e.L("c.Cono(0, y, 0, r*1.3, %s, %s) // tejado", f2(1+rng.Float64()*2.5), tallerCol(pal[2]))
	case 1:
		e.L("c.Revolucion(0, y, 0, []float64{r*1.1, 0, r*1.0, r*0.6, r*0.6, r*1.1, 0.01, r*1.3}, 16, %s) // cúpula", tallerCol(pal[2]))
	default:
		e.L("c.Brilla(0, y+0.6, 0, %s, %s) // una luz arriba", f2(0.3+rng.Float64()*0.4), tallerCol(tallerAclarar(pal[2], 0.6)))
	}
}

func (e *tallerEsbozo) arbol(rng *rand.Rand, pal [][3]float64) {
	e.S("Empezar", `o.Ambiente("pajaros", 0.25)`)
	e.ayuda("rama", `// rama: un trozo de árbol que se parte en otros más pequeños
func rama(c *mundo.Cuerpo, x, y, z, largo, rumbo, inclina, grosor float64, nivel int, madera, hoja mundo.Color, partes int, giro float64) {
	x2 := x + mundo.Sin(inclina)*mundo.Cos(rumbo)*largo
	y2 := y + mundo.Cos(inclina)*largo
	z2 := z + mundo.Sin(inclina)*mundo.Sin(rumbo)*largo
	c.Tubo(x, y, z, x2, y2, z2, grosor, grosor*0.7, madera)
	if nivel == 0 {
		c.Esfera(x2, y2, z2, largo*0.9, hoja)
		return
	}
	for i := 0; i < partes; i++ {
		r := rumbo + giro + float64(i)*2*mundo.Pi/float64(partes)
		rama(c, x2, y2, z2, largo*0.72, r, inclina*0.6+0.45, grosor*0.68, nivel-1, madera, hoja, partes, giro)
	}
}
`)
	madera := tallerOscurecer(pal[2], 0.45)
	if lumDe(madera) > 0.35 {
		madera = [3]float64{0.32, 0.22, 0.14}
	}
	nivel := 3 + rng.Intn(2)
	partes := 2 + rng.Intn(2)
	if nivel == 4 && partes == 3 {
		partes = 2
	}
	e.L("// un árbol que crece en %d niveles", nivel)
	e.L("rama(c, 0, 0, 0, %s, %s, 0, %s, %d, %s, %s, %d, %s)", f2(1.4+rng.Float64()*1.2), f2(rng.Float64()*6), f2(0.18+rng.Float64()*0.12),
		nivel, tallerCol(madera), tallerCol(tallerVivo(pal[0])), partes, f2(0.3+rng.Float64()*0.9))
}

func (e *tallerEsbozo) fuente(rng *rand.Rand, pal [][3]float64) {
	e.S("Empezar", `o.Ambiente("agua", 0.5)`)
	r := 1.4 + rng.Float64()*1.4
	e.L("// una fuente")
	e.L("c.Revolucion(0, 0, 0, []float64{%s, 0, %s, 0.5, %s, 0.5, %s, 0.15}, 32, %s)", f2(r), f2(r*1.05), f2(r*0.9), f2(r*0.85), tallerCol(pal[1]))
	e.L("c.Superficie(%s, 0.38, %s, %s, %s, []float64{0, 0, 0, 0}, 2, []mundo.Color{%s})", f2(-r*0.62), f2(-r*0.62), f2(r*1.24), f2(r*1.24), tallerCol(tallerAclarar(pal[0], 0.2)))
	e.L("c.Cilindro(0, 0, 0, %s, %s, %s)", f2(0.18+rng.Float64()*0.1), f2(1.2+rng.Float64()), tallerCol(pal[2]))
	e.L("for k := 0; k < %d; k++ { // el agua que cae", 5+rng.Intn(6))
	e.L("\ta := float64(k) * 2 * mundo.Pi / %d", 5+rng.Intn(6))
	e.L("\tc.Tubo(0, %s, 0, mundo.Cos(a)*%s, 0.45, mundo.Sin(a)*%s, 0.05, 0.02, %s)", f2(1.6), f2(r*0.6), f2(r*0.6), tallerCol(tallerAclarar([3]float64{0.3, 0.6, 0.9}, 0.3)))
	e.L("}")
	e.L("c.Brilla(0, %s, 0, 0.22, %s)", f2(1.75), tallerCol(tallerAclarar(pal[0], 0.6)))
}

func (e *tallerEsbozo) arco(rng *rand.Rand, pal [][3]float64) {
	ancho := 2.5 + rng.Float64()*3
	alto := 2.5 + rng.Float64()*2.5
	g := 0.3 + rng.Float64()*0.3
	e.L("// un arco para pasar por debajo")
	e.L("c.Caja(0, %s, %s, %s, %s, %s, %s)", f2(alto/2), f2(-ancho/2), f2(g), f2(alto), f2(g), tallerCol(pal[0]))
	e.L("c.Caja(0, %s, %s, %s, %s, %s, %s)", f2(alto/2), f2(ancho/2), f2(g), f2(alto), f2(g), tallerCol(pal[0]))
	e.L("for k := 0; k <= 18; k++ {")
	e.L("\ta := float64(k) / 18 * mundo.Pi")
	e.L("\tc.Caja(0, %s+mundo.Sin(a)*%s, -mundo.Cos(a)*%s, %s, %s, %s, mundo.Mezcla(%s, %s, mundo.Sin(a)))",
		f2(alto), f2(ancho*0.45), f2(ancho/2), f2(g*1.1), f2(g), f2(g*1.1), tallerCol(pal[0]), tallerCol(pal[1]))
	e.L("}")
	e.L("c.Brilla(0, %s, 0, %s, %s)", f2(alto+ancho*0.45+0.1), f2(g*0.8), tallerCol(tallerAclarar(pal[2], 0.5)))
}

func (e *tallerEsbozo) farolas(rng *rand.Rand, pal [][3]float64, n int, radio float64) {
	e.S("Empezar", `o.Ambiente("zumbido", 0.08)`)
	e.L("// %d farolas alrededor", n)
	e.L("for k := 0; k < %d; k++ {", n)
	e.L("\ta := float64(k)*2*mundo.Pi/%d + %s", n, f2(rng.Float64()))
	e.L("\tx, z := mundo.Cos(a)*%s, mundo.Sin(a)*%s", f2(radio), f2(radio))
	e.L("\tc.Cilindro(x, 0, z, 0.06, 2.2, %s)", tallerCol(tallerOscurecer(pal[1], 0.5)))
	e.L("\tc.Brilla(x, 2.3, z, 0.17, %s)", tallerCol(tallerAclarar(pal[2], 0.7)))
	e.L("}")
}

func (e *tallerEsbozo) piramide(rng *rand.Rand, pal [][3]float64) {
	pisos := 3 + rng.Intn(5)
	lado := 2.5 + rng.Float64()*3
	h := 0.4 + rng.Float64()*0.5
	e.L("// una pirámide escalonada de %d pisos", pisos)
	e.L("for p := 0; p < %d; p++ {", pisos)
	e.L("\tl := %s * (1 - float64(p)/%d)", f2(lado), pisos+1)
	e.L("\tc.Caja(0, %s*float64(p)+%s, 0, l, %s, l, mundo.Mezcla(%s, %s, float64(p)/%d))", f2(h), f2(h/2), f2(h), tallerCol(pal[0]), tallerCol(pal[1]), pisos)
	e.L("}")
	e.L("c.Revolucion(0, %s, 0, []float64{%s, 0, 0.01, %s}, 4, %s)", f2(h*float64(pisos)), f2(lado/(float64(pisos)+1)*0.75), f2(lado*0.3), tallerCol(pal[2]))
}

func (e *tallerEsbozo) fractal(rng *rand.Rand, pal [][3]float64) {
	e.ayuda("esponja", `// esponja: un cubo hecho de cubos con huecos (como la de Menger)
func esponja(c *mundo.Cuerpo, x, y, z, lado float64, nivel int, a, b mundo.Color) {
	if nivel == 0 {
		c.Caja(x, y, z, lado, lado, lado, mundo.Mezcla(a, b, mundo.Ruido(x*3, z*3+y, 7)))
		return
	}
	t := lado / 3
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			for k := -1; k <= 1; k++ {
				ceros := 0
				if i == 0 { ceros++ }
				if j == 0 { ceros++ }
				if k == 0 { ceros++ }
				if ceros >= 2 { continue }
				esponja(c, x+float64(i)*t, y+float64(j)*t, z+float64(k)*t, t, nivel-1, a, b)
			}
		}
	}
}
`)
	lado := 1.8 + rng.Float64()*1.8
	e.L("// un fractal: cada cubo es 20 cubos más pequeños")
	e.L("esponja(c, 0, %s, 0, %s, 2, %s, %s)", f2(lado/2), f2(lado), tallerCol(pal[0]), tallerCol(pal[1]))
}

func (e *tallerEsbozo) girasol(rng *rand.Rand, pal [][3]float64) {
	n := 60 + rng.Intn(80)
	e.L("// %d semillas en espiral (el ángulo de oro); la flor gira despacio", n)
	e.L("c.Parte(\"flor\", 0, 0, 0)")
	e.L("for k := 1; k <= %d; k++ {", n)
	e.L("\tr := mundo.Sqrt(float64(k)) * %s", f2(0.16+rng.Float64()*0.08))
	e.L("\ta := float64(k) * 2.39996")
	e.L("\ty := %s - r*r*%s", f2(1.5+rng.Float64()*2), f2(0.08+rng.Float64()*0.1))
	e.L("\tcol := mundo.Mezcla(%s, %s, float64(k)/%d)", tallerCol(pal[0]), tallerCol(pal[1]), n)
	e.L("\tc.Esfera(mundo.Cos(a)*r, y, mundo.Sin(a)*r, 0.07+0.04*mundo.Sin(float64(k)), col)")
	e.L("}")
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.L("c.Cilindro(0, 0, 0, 0.12, %s, %s)", f2(1.4), tallerCol(tallerOscurecer(pal[2], 0.3)))
	e.S("Actuar", `o.Rotar("flor", 0, o.Tiempo()*0.25, 0)`)
}

func (e *tallerEsbozo) escalera(rng *rand.Rand, pal [][3]float64) {
	n := 18 + rng.Intn(24)
	e.L("// una escalera de caracol de %d peldaños", n)
	e.L("for k := 0; k < %d; k++ {", n)
	e.L("\tm := c.Marca()")
	e.L("\tc.Caja(%s, 0.08+float64(k)*%s, 0, %s, 0.08, 0.4, mundo.Mezcla(%s, %s, float64(k)/%d))", f2(0.9), f2(0.18), f2(1.4), tallerCol(pal[0]), tallerCol(pal[1]), n)
	e.L("\tc.GirarDesde(m, float64(k)*%s)", f2(0.35+rng.Float64()*0.2))
	e.L("}")
	e.L("c.Cilindro(0, 0, 0, 0.18, %s, %s)", f2(float64(n)*0.18+0.4), tallerCol(pal[2]))
	e.L("c.Brilla(0, %s, 0, 0.25, %s)", f2(float64(n)*0.18+0.6), tallerCol(tallerAclarar(pal[2], 0.6)))
}

func (e *tallerEsbozo) casa(rng *rand.Rand, pal [][3]float64, oscura bool) {
	an, fo, al := 3.6+rng.Float64()*2.5, 3.6+rng.Float64()*2.5, 2.5+rng.Float64()*1.0
	te := 0.8 + rng.Float64()*1.4
	pared, techo := tallerCol(pal[0]), tallerCol(pal[1])
	e.L("// una casita hueca: se entra por la puerta (E para abrirla)")
	e.L("c.Caja(0, 0.05, 0, %s, 0.1, %s, %s) // el suelo", f2(an), f2(fo), tallerCol(tallerOscurecer(pal[1], 0.3)))
	e.L("c.Caja(%s, %s, 0, 0.15, %s, %s, %s) // la pared de atrás", f2(-an/2), f2(al/2), f2(al), f2(fo), pared)
	e.L("c.Caja(0, %s, %s, %s, %s, 0.15, %s)", f2(al/2), f2(fo/2), f2(an), f2(al), pared)
	e.L("c.Caja(0, %s, %s, %s, %s, 0.15, %s)", f2(al/2), f2(-fo/2), f2(an), f2(al), pared)
	lado := fo/2 - 0.55
	e.L("c.Caja(%s, %s, %s, 0.15, %s, %s, %s) // la de delante, con la puerta", f2(an/2), f2(al/2), f2(0.55+lado/2), f2(al), f2(lado), pared)
	e.L("c.Caja(%s, %s, %s, 0.15, %s, %s, %s)", f2(an/2), f2(al/2), f2(-0.55-lado/2), f2(al), f2(lado), pared)
	e.L("c.Caja(%s, %s, 0, 0.15, %s, 1.1, %s)", f2(an/2), f2((2.1+al)/2), f2(al-2.1), pared)
	e.L("c.Lamina(%s, %s, %s, %s, %s, %s, 0, %s, %s, %s)", f2(-an/2-0.2), f2(al), f2(-fo/2-0.2), f2(an/2+0.2), f2(al), f2(-fo/2-0.2), f2(al+te), f2(-fo/2-0.2), techo)
	e.L("c.Lamina(%s, %s, %s, %s, %s, %s, 0, %s, %s, %s)", f2(-an/2-0.2), f2(al), f2(fo/2+0.2), f2(an/2+0.2), f2(al), f2(fo/2+0.2), f2(al+te), f2(fo/2+0.2), techo)
	e.L("c.Cuadro(%s, %s, %s, %s, %s, %s, 0, %s, %s, 0, %s, %s, %s)", f2(an/2+0.2), f2(al), f2(-fo/2-0.2), f2(an/2+0.2), f2(al), f2(fo/2+0.2), f2(al+te), f2(fo/2+0.2), f2(al+te), f2(-fo/2-0.2), techo)
	e.L("c.Cuadro(%s, %s, %s, 0, %s, %s, 0, %s, %s, %s, %s, %s, %s)", f2(-an/2-0.2), f2(al), f2(-fo/2-0.2), f2(al+te), f2(-fo/2-0.2), f2(al+te), f2(fo/2+0.2), f2(-an/2-0.2), f2(al), f2(fo/2+0.2), techo)
	luz := tallerAclarar(pal[2], 0.4)
	if oscura {
		luz = [3]float64{1, 0.85, 0.5}
	}
	e.L("c.Brillante(true) // ventanas y la luz de dentro")
	e.L("c.Caja(%s, 1.5, %s, 0.2, 0.8, 0.8, %s)", f2(-an/2), f2(fo/4), tallerCol(luz))
	e.L("c.Caja(0, 1.5, %s, 0.8, 0.8, 0.2, %s)", f2(fo/2), tallerCol(luz))
	e.L("c.Caja(0, %s, 0, 0.5, 0.06, 0.5, %s)", f2(al-0.1), tallerCol(luz))
	e.L("c.Brillante(false)")
	e.L("c.Parte(\"puerta\", %s, 0.1, -0.53)", f2(an/2))
	e.L("c.Caja(%s, 1.1, 0, 0.06, 2.0, 1.05, %s)", f2(an/2), tallerCol(tallerOscurecer(pal[2], 0.5)))
	e.L("c.Esfera(%s, 1.05, 0.35, 0.05, mundo.RGB(0.85, 0.75, 0.35))", f2(an/2+0.06))
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.S("Empezar", `if o.Valor("abierta") == 1 {
	o.Rotar("puerta", 0, 1.4, 0)
}`)
	e.S("AlUsar", `a := 0.0
if o.Valor("abierta") == 0 {
	a = 1.4
	o.Guardar("abierta", 1)
} else {
	o.Guardar("abierta", 0)
}
o.Rotar("puerta", 0, a, 0)
o.SonarEn("puerta", "puerta", 1)`)
}

func (e *tallerEsbozo) piedras(rng *rand.Rand, pal [][3]float64) {
	n := 5 + rng.Intn(8)
	e.L("// un círculo de %d piedras", n)
	e.L("for k := 0; k < %d; k++ {", n)
	e.L("\ta := float64(k) * 2 * mundo.Pi / %d", n)
	e.L("\tx, z := mundo.Cos(a)*%s, mundo.Sin(a)*%s", f2(2.2+rng.Float64()*2), f2(2.2+rng.Float64()*2))
	e.L("\th := 0.8 + 1.6*mundo.Ruido(float64(k), 3, %d)", rng.Intn(1000))
	e.L("\tc.Revolucion(x, 0, z, []float64{0.45, 0, 0.5, h*0.3, 0.38, h*0.8, 0.05, h}, 7, mundo.Mezcla(%s, %s, mundo.Ruido(float64(k), 9, 2)))", tallerCol(pal[0]), tallerCol(pal[1]))
	e.L("}")
}

func (e *tallerEsbozo) hoguera(rng *rand.Rand) {
	e.S("Empezar", `o.Ambiente("fuego", 0.6)`)
	e.L("// una hoguera")
	e.L("for k := 0; k < 7; k++ {")
	e.L("\ta := float64(k) * 2 * mundo.Pi / 7")
	e.L("\tc.Tubo(mundo.Cos(a)*0.6, 0.05, mundo.Sin(a)*0.6, mundo.Cos(a)*0.1, 0.5, mundo.Sin(a)*0.1, 0.07, 0.05, mundo.RGB(0.3, 0.2, 0.12))")
	e.L("}")
	e.L("for k := 0; k < %d; k++ {", 5+rng.Intn(5))
	e.L("\tc.Brilla(0.3*mundo.Cos(float64(k)*2.4), 0.3+float64(k)*0.12, 0.3*mundo.Sin(float64(k)*2.4), 0.25-float64(k)*0.02, mundo.HSV(0.02+float64(k)*0.015, 0.9, 1))")
	e.L("}")
}

func (e *tallerEsbozo) orbes(rng *rand.Rand, pal [][3]float64, alto float64) {
	n := 6 + rng.Intn(10)
	e.L("// %d luces que giran en un anillo de pie", n)
	e.L("c.Parte(\"orbes\", 0, %s, 0)", f2(alto))
	e.L("for k := 0; k < %d; k++ {", n)
	e.L("\ta := float64(k) * 2 * mundo.Pi / %d", n)
	e.L("\tc.Brilla(0, %s+mundo.Sin(a)*%s, mundo.Cos(a)*%s, 0.16, mundo.Mezcla(%s, %s, float64(k)/%d))", f2(alto), f2(alto*0.7), f2(alto*0.7), tallerCol(tallerAclarar(pal[0], 0.4)), tallerCol(tallerAclarar(pal[1], 0.4)), n)
	e.L("}")
	e.L("c.Parte(\"\", 0, 0, 0)")
	e.L("c.Cilindro(0, 0, 0, 0.1, %s, %s)", f2(alto*0.3), tallerCol(tallerOscurecer(pal[2], 0.3)))
	e.S("Empezar", `o.AmbienteEn("orbes", "magia", 0.25)`)
	e.S("Actuar", `o.Rotar("orbes", o.Tiempo()*0.6, 0, 0)`)
}

// estela: una piedra de pie con una frase escrita.
func (e *tallerEsbozo) estela(rng *rand.Rand, pal [][3]float64, frase string) {
	frase = strings.TrimSpace(frase)
	if frase == "" {
		frase = "abla"
	}
	if len([]rune(frase)) > 28 {
		frase = string([]rune(frase)[:28])
	}
	e.S("AlUsar", fmt.Sprintf(`o.Decir(%q)
o.Sonar("magia", 1)`, frase))
	tam := 0.35
	largo := float64(len([]rune(frase)))*tam*0.8 + 0.6
	alto := 2.0 + rng.Float64()
	e.L("// una estela con algo escrito: «%s»", frase)
	e.L("c.Caja(0, %s, 0, 0.4, %s, %s, %s)", f2(alto/2), f2(alto), f2(largo), tallerCol(pal[0]))
	e.L("c.Letras(%q, 0.22, %s, %s, %s, 0.08, %s)", frase, f2(alto*0.55), f2(largo/2-0.3), f2(tam), tallerCol(tallerAclarar(pal[1], 0.5)))
	e.L("c.Brilla(0, %s, 0, 0.2, %s)", f2(alto+0.3), tallerCol(tallerAclarar(pal[2], 0.6)))
}

// figura: una de las cosas que Nyx recortó de lo que vio, en relieve.
func (e *tallerEsbozo) figura(f Figura, pedestal [3]float64) {
	e.L("// «%s», una cosa que Nyx vio, sobre un pedestal", f.ID)
	e.L("c.Caja(0, 0.25, 0, %s, 0.5, %s, %s)", f2(f.Ancho+0.4), f2(f.Ancho+0.4), tallerCol(pedestal))
	e.L("m := c.Marca()")
	var pal []string
	for _, p := range f.Paleta {
		pal = append(pal, tallerCol(p))
	}
	var filas []string
	for _, fl := range f.Forma {
		filas = append(filas, fmt.Sprintf("%q", fl))
	}
	e.L("c.Figura([]string{%s}, []mundo.Color{%s}, %s, %s, %s, true)", strings.Join(filas, ", "), strings.Join(pal, ", "),
		f2(f.Ancho), f2(f.Alto), f3(math.Max(0.04, math.Min(f.Ancho, f.Alto)*0.12)))
	e.L("c.MoverDesde(m, 0, 0.5, 0)")
}

// mural: un relieve hecho con la foto misma (cada punto, su color real y
// su luz como altura).
func (e *tallerEsbozo) mural(ruta string, alto float64) bool {
	w, h, rgb, err := tallerLeerPPM(ruta)
	if err != nil || w < 4 || h < 4 {
		return false
	}
	cw := 40
	ch := cw * h / w
	if ch < 4 {
		ch = 4
	}
	var alts, cols []string
	for j := 0; j < ch; j++ {
		for i := 0; i < cw; i++ {
			// visto desde delante (+X), +Z queda a la izquierda: se recorre al revés
			x, y := (cw-1-i)*w/cw, (ch-1-j)*h/ch
			p := rgb[(y*w+x)*3:]
			r, g, b := float64(p[0])/255, float64(p[1])/255, float64(p[2])/255
			alts = append(alts, f2(lumDe([3]float64{r, g, b})*0.5))
			cols = append(cols, f2(r), f2(g), f2(b))
		}
	}
	ancho := alto * float64(cw) / float64(ch)
	e.ayuda("mural", `// mural: levanta un relieve de pie a partir de luces y colores
func mural(c *mundo.Cuerpo, ancho, alto float64, columnas int, luz, rgb []float64) {
	var cols []mundo.Color
	for i := 0; i+2 < len(rgb); i += 3 {
		cols = append(cols, mundo.RGB(rgb[i], rgb[i+1], rgb[i+2]))
	}
	m := c.Marca()
	c.Superficie(-ancho/2, 0, -alto/2, ancho, alto, luz, columnas, cols)
	c.LevantarDesde(m) // de tumbado en el suelo a de pie, mirando hacia +X
	c.MoverDesde(m, 0, alto/2+0.3, 0)
}
`)
	e.L("// un mural en relieve de lo que vio (%d×%d puntos de la foto)", cw, ch)
	e.L("mural(c, %s, %s, %d, []float64{%s}, []float64{%s})", f2(ancho), f2(alto), cw, strings.Join(alts, ","), strings.Join(cols, ","))
	e.L("c.Caja(0, 0.15, 0, 0.5, 0.3, %s, mundo.RGB(0.25, 0.22, 0.2))", f2(ancho+0.3))
	return true
}

// ---------- leer una foto PPM (las que guarda Abla) ----------

func tallerLeerPPM(ruta string) (int, int, []byte, error) {
	d, err := os.ReadFile(ruta)
	if err != nil {
		return 0, 0, nil, err
	}
	campos := []int{}
	i := 2
	if len(d) < 2 || string(d[:2]) != "P6" {
		return 0, 0, nil, fmt.Errorf("no es un PPM P6")
	}
	for len(campos) < 3 && i < len(d) {
		for i < len(d) && (d[i] == ' ' || d[i] == '\n' || d[i] == '\r' || d[i] == '\t') {
			i++
		}
		if i < len(d) && d[i] == '#' {
			for i < len(d) && d[i] != '\n' {
				i++
			}
			continue
		}
		n := 0
		for i < len(d) && d[i] >= '0' && d[i] <= '9' {
			n = n*10 + int(d[i]-'0')
			i++
		}
		campos = append(campos, n)
	}
	i++
	if len(campos) < 3 || campos[2] != 255 || len(d)-i < campos[0]*campos[1]*3 {
		return 0, 0, nil, fmt.Errorf("PPM raro")
	}
	return campos[0], campos[1], d[i:], nil
}

// ---------- NYX escribe ----------

// tallerIdeaNyx: una pieza a partir de un recuerdo suyo. pedido: si se lo
// pediste tú ("una torre", "un árbol"…), manda eso.
func tallerIdeaNyx(m *Mundo, rng *rand.Rand, pedido string) (titulo, de, codigo string) {
	m.mu.Lock()
	var r *Recuerdo
	if len(m.Recuerdos) > 0 {
		// prefiere los más recientes, pero cualquiera puede volver
		k := len(m.Recuerdos) - 1 - int(math.Abs(rng.NormFloat64())*float64(len(m.Recuerdos))/2.5)
		if k < 0 {
			k = rng.Intn(len(m.Recuerdos))
		}
		r = m.Recuerdos[k]
	}
	m.mu.Unlock()
	pal := [][3]float64{{0.55, 0.52, 0.48}, {0.35, 0.33, 0.3}, {0.8, 0.75, 0.6}}
	var ras Rasgos
	de = "su imaginación (todavía no ha visto nada)"
	var figs []Figura
	if r != nil {
		ras = r.Rasgos
		pal = [][3]float64{tallerVivo(ras.Pared), tallerVivo(ras.Suelo), tallerVivo(ras.Techo)}
		de = r.Nombre
		figs = r.Figuras
	}
	e := &tallerEsbozo{}
	// lo que pesa cada idea según cómo era el sitio
	pesos := map[string]float64{
		"torre": 0.6 + ras.Altura*2 + ras.Cielo, "árbol": 0.4 + ras.Verde*3, "fuente": 0.3 + ras.Agua*3,
		"arco": 0.4 + ras.Pasillo*2 + ras.Cielo, "casa": 0.5 + ras.Paredes*1.5, "piedras": 0.3 + ras.Relieve*2 + ras.Irregular,
		"farolas": 0.2 + ras.Oscuridad*2 + ras.Luces, "escalera": 0.3 + ras.Altura, "estatua": 0,
		"pirámide": 0.3 + ras.Apertura, "hoguera": 0.2 + ras.Oscuridad,
		// hacia arriba, hacia abajo, cosas vivas y juegos
		"torre-mirador": 0.35 + ras.Altura*1.5 + ras.Cielo, "edificio": 0.35 + ras.Paredes + ras.Pasillo,
		"mazmorra": 0.3 + ras.Oscuridad*1.5 + ras.Irregular, "ascensor": 0.2 + ras.Altura,
		"anillos": 0.35, "plataformas": 0.3, "molino": 0.2 + ras.Verde + ras.Cielo*0.5, "campanario": 0.25,
	}
	if len(figs) > 0 {
		pesos["estatua"] = 2.5
	}
	idea := tallerElegir(rng, pesos, pedido)
	titulo = idea
	switch idea {
	case "torre":
		e.torre(rng, pal, ras.Altura)
	case "árbol":
		e.arbol(rng, pal)
	case "fuente":
		e.fuente(rng, pal)
	case "arco":
		e.arco(rng, pal)
	case "casa":
		e.casa(rng, pal, ras.Oscuridad > 0.5)
	case "piedras":
		e.piedras(rng, pal)
	case "farolas":
		e.farolas(rng, pal, 4+rng.Intn(5), 2+rng.Float64()*2)
	case "escalera":
		e.escalera(rng, pal)
	case "pirámide":
		e.piramide(rng, pal)
	case "hoguera":
		e.hoguera(rng)
		e.piedras(rng, pal)
	case "estatua":
		f := figs[rng.Intn(len(figs))]
		e.figura(f, tallerOscurecer(pal[1], 0.2))
		titulo = "estatua"
	default:
		e.tallerGrande(idea, rng, pal, ras.Altura, ras.Oscuridad > 0.5)
	}
	// y a veces algo más alrededor (si es pequeña)
	pequena := map[string]bool{"torre": true, "árbol": true, "fuente": true, "arco": true, "piedras": true, "escalera": true, "hoguera": true, "estatua": true}
	if pequena[idea] && (ras.Oscuridad > 0.55 || rng.Float64() < 0.25) {
		e.farolas(rng, pal, 3+rng.Intn(3), 3.5+rng.Float64()*2)
	}
	e.Titulo = titulo
	e.cabecera = fmt.Sprintf("// %s — la escribió Nyx con lo que recuerda de «%s».\n// Es Go normal: puedes cambiarlo (o pedírselo a ella o a Abla).\n", titulo, de)
	return titulo, de, e.Codigo()
}

// ---------- ABLA escribe ----------

// tallerIdeaAbla: una pieza a partir de algo que vio. tribu: 0 lenguaje,
// 1 data, 2 lógica (cada una construye a su manera).
func tallerIdeaAbla(o *tallerObra, rng *rand.Rand, v *tallerVista, tribu int, pedido string) (titulo, de, codigo string) {
	pal := [][3]float64{{0.6, 0.55, 0.8}, {0.3, 0.6, 0.7}, {0.9, 0.8, 0.5}}
	de = "su imaginación"
	conceptos := map[string]bool{}
	frase := "abla"
	if v != nil {
		for i, p := range v.Paleta {
			if i < 3 {
				pal[i] = tallerVivo([3]float64{float64(p.RGB[0]) / 255, float64(p.RGB[1]) / 255, float64(p.RGB[2]) / 255})
			}
		}
		for _, c := range v.Conceptos {
			conceptos[c.Es] = true
		}
		de = v.Foto
		if v.Video != "" {
			de = v.Video + " (" + v.Foto + ")"
		}
		if v.Abla != "" {
			frase = v.Abla
		}
	}
	tiene := func(ws ...string) float64 {
		for _, w := range ws {
			if conceptos[w] {
				return 1
			}
		}
		return 0
	}
	e := &tallerEsbozo{}
	pesos := map[string]float64{
		"torre":    0.3 + 2*tiene("cielo", "arriba", "alto"),
		"árbol":    0.3 + 2.5*tiene("árbol", "planta", "hierba", "bosque", "verde"),
		"fuente":   0.2 + 2.5*tiene("agua", "mar", "río", "lluvia"),
		"orbes":    0.3 + 2*tiene("luz", "día", "sol", "noche", "estrella"),
		"pirámide": 0.3 + 2*tiene("piedra", "tierra", "montaña", "roca"),
		"hoguera":  0.1 + 2.5*tiene("fuego", "calor", "rojo"),
		"fractal":  0.2 + 2*tiene("patrón"),
		"girasol":  0.3, "escalera": 0.3, "arco": 0.3,
		"estela": 0, "mural": 0,
		"torre-mirador": 0.2 + 2*tiene("cielo", "arriba", "alto"),
		"mazmorra":      0.2 + 2*tiene("noche", "piedra", "tierra", "cueva", "oscuro", "negro"),
		"anillos":       0.3, "plataformas": 0.3, "edificio": 0.2, "ascensor": 0.2,
		"molino":     0.2 + 1.5*tiene("viento", "aire", "verde"),
		"campanario": 0.2,
	}
	// cada tribu tiene su manera
	switch tribu {
	case 0:
		pesos["estela"] = 3
		pesos["campanario"] += 0.8
	case 1:
		if v != nil && v.Mini != "" {
			pesos["mural"] = 3.5
		}
	case 2:
		pesos["fractal"] += 1.5
		pesos["girasol"] += 1.2
		pesos["escalera"] += 1
		pesos["anillos"] += 0.8
		pesos["plataformas"] += 0.8
	}
	idea := tallerElegir(rng, pesos, pedido)
	titulo = idea
	switch idea {
	case "torre":
		e.torre(rng, pal, 0.3+v.bordesV())
	case "árbol":
		e.arbol(rng, pal)
	case "fuente":
		e.fuente(rng, pal)
	case "orbes":
		e.orbes(rng, pal, 1.5+rng.Float64()*1.5)
	case "pirámide":
		e.piramide(rng, pal)
	case "hoguera":
		e.hoguera(rng)
	case "fractal":
		e.fractal(rng, pal)
	case "girasol":
		e.girasol(rng, pal)
	case "escalera":
		e.escalera(rng, pal)
	case "arco":
		e.arco(rng, pal)
	case "estela":
		e.estela(rng, pal, frase)
	case "mural":
		if !e.mural(filepath.Join(o.abla.dir, v.Mini), 2+rng.Float64()*1.5) {
			e.estela(rng, pal, frase)
			titulo = "estela"
		}
	default:
		e.tallerGrande(idea, rng, pal, 0.3+v.bordesV(), tiene("noche", "oscuro") > 0)
	}
	// su música: la frase que le evocó, tocada nota a nota
	if rng.Float64() < 0.45 {
		e.S("Empezar", fmt.Sprintf(`o.Ambiente(%q, 0.25)`, "musica:"+tallerMelodia(frase)))
	}
	// lo que es de lenguaje siempre lleva algo escrito (si cabe al lado)
	grande := map[string]bool{"edificio": true, "mazmorra": true, "anillos": true, "plataformas": true, "ascensor": true}
	if tribu == 0 && idea != "estela" && !grande[idea] {
		m := "m := c.Marca()"
		e.L("%s", m)
		e.estela(rng, pal, frase)
		e.L("c.MoverDesde(m, %s, 0, 0)", f2(3+rng.Float64()))
	}
	e.Titulo = titulo
	e.cabecera = fmt.Sprintf("// %s — la escribió Abla (tribu %s) con lo que vio en «%s».\n// En Abla: %s\n// Es Go normal: puedes cambiarlo (o pedírselo a ella o a Nyx).\n", titulo, tallerTribus[tribu], de, frase)
	return titulo, de, e.Codigo()
}

func (v *tallerVista) bordesV() float64 {
	if v == nil {
		return 0
	}
	return v.BordesV
}

// tallerElegir: una idea al azar según sus pesos (o la que pediste, si la
// nombraste).
func tallerElegir(rng *rand.Rand, pesos map[string]float64, pedido string) string {
	p := strings.ToLower(pedido)
	sinoes := map[string][]string{
		"torre": {"torre", "faro"}, "árbol": {"árbol", "arbol", "bosque", "planta"},
		"fuente": {"fuente", "agua", "estanque"}, "arco": {"arco", "puerta", "portal"},
		"casa": {"casa", "cabaña", "refugio", "hogar"}, "piedras": {"piedras", "rocas", "círculo", "circulo"},
		"farolas": {"farola", "luces", "lámpara", "lampara"}, "escalera": {"escalera", "caracol"},
		"pirámide": {"pirámide", "piramide", "templo", "zigurat"}, "hoguera": {"hoguera", "fuego"},
		"estatua": {"estatua", "escultura", "monumento"}, "orbes": {"orbes", "luces que giran"},
		"torre-mirador": {"mirador", "subir", "torre alta", "arriba"}, "edificio": {"edificio", "pisos", "rascacielos", "bloque"},
		"mazmorra": {"mazmorra", "sótano", "sotano", "cueva", "bajar", "subterráneo", "subterraneo", "abajo", "túnel", "tunel"},
		"ascensor": {"ascensor", "elevador"}, "anillos": {"anillos", "monedas", "juego"},
		"plataformas": {"plataformas", "saltos", "parkour", "saltar"}, "molino": {"molino"},
		"campanario": {"campanario", "campana"},
		"fractal":    {"fractal", "esponja", "cubos"}, "girasol": {"girasol", "espiral", "semillas"},
		"estela": {"estela", "letrero", "escrito", "frase", "palabras"}, "mural": {"mural", "cuadro", "relieve", "foto"},
	}
	if p != "" {
		// la palabra más larga que encaje manda («farolas» no es «faro»)
		mejor, largo := "", 0
		for idea, ws := range sinoes {
			peso, ok := pesos[idea]
			if !ok || (peso <= 0 && (idea == "estatua" || idea == "mural")) {
				continue // sin cosas vistas o sin foto no se puede
			}
			for _, w := range ws {
				if strings.Contains(p, w) && (len(w) > largo || len(w) == largo && idea < mejor) {
					mejor, largo = idea, len(w)
				}
			}
		}
		if mejor != "" {
			return mejor
		}
	}
	total := 0.0
	claves := make([]string, 0, len(pesos))
	for k, v := range pesos {
		if v > 0 {
			total += v
			claves = append(claves, k)
		}
	}
	// orden fijo, para que el azar dependa solo de la semilla
	for i := range claves {
		for j := i + 1; j < len(claves); j++ {
			if claves[j] < claves[i] {
				claves[i], claves[j] = claves[j], claves[i]
			}
		}
	}
	x := rng.Float64() * total
	for _, k := range claves {
		if x -= pesos[k]; x <= 0 {
			return k
		}
	}
	return claves[len(claves)-1]
}

// ---------- cambiar lo de la otra ----------

// tallerRetoqueAbla: lo que Abla le hace a una pieza (normalmente de Nyx).
// Devuelve qué hizo (para la charla) y el código nuevo.
func tallerRetoqueAbla(rng *rand.Rand, codigo string, v *tallerVista) (string, string, error) {
	col := [3]float64{0.7, 0.6, 1}
	frase := "abla"
	if v != nil {
		if len(v.Paleta) > 0 {
			col = tallerVivo([3]float64{float64(v.Paleta[0].RGB[0]) / 255, float64(v.Paleta[0].RGB[1]) / 255, float64(v.Paleta[0].RGB[2]) / 255})
		}
		if v.Abla != "" {
			frase = v.Abla
		}
	}
	switch rng.Intn(4) {
	case 0:
		n := 5 + rng.Intn(8)
		c, err := tallerAnadir(codigo, "Abla: luces que giran arriba", fmt.Sprintf(`h := c.Alto()
r := c.Ancho()*0.8 + 0.4
for k := 0; k < %d; k++ {
a := float64(k) * 2 * mundo.Pi / %d
c.Brilla(mundo.Cos(a)*r, h+0.5+0.3*mundo.Sin(a*3), mundo.Sin(a)*r, 0.13, mundo.HSV(%s+float64(k)*0.04, 0.5, 1))
}`, n, n, f2(rng.Float64())))
		return "le puse un anillo de luces arriba", c, err
	case 1:
		if len([]rune(frase)) > 20 {
			frase = string([]rune(frase)[:20])
		}
		c, err := tallerAnadir(codigo, "Abla: lo escribió en su idioma", fmt.Sprintf(`h := c.Alto()
r := c.Ancho()
c.Letras(%q, r+0.15, h*0.4+0.3, %s, 0.3, 0.06, %s)`, frase, f2(float64(len([]rune(frase)))*0.12), tallerCol(tallerAclarar(col, 0.5))))
		return "le escribí «" + frase + "» en Abla", c, err
	case 2:
		esc := tallerValor(codigo, "escala")
		if esc == 0 {
			esc = 1
		}
		k := 1.15 + rng.Float64()*0.35
		if esc > 2.2 {
			k = 0.8
		}
		c, err := tallerRetocar(codigo, "escala", esc*k)
		return fmt.Sprintf("la hice %s", map[bool]string{true: "más grande", false: "más pequeña"}[k > 1]), c, err
	default:
		c, err := tallerRetocar(codigo, "tinte", col[0], col[1], col[2], 0.25+rng.Float64()*0.25)
		return "la teñí con el color de lo que vi (" + nombreColor(col) + ")", c, err
	}
}

// tallerRetoqueNyx: lo que Nyx le hace a una pieza (normalmente de Abla).
func tallerRetoqueNyx(m *Mundo, rng *rand.Rand, codigo string) (string, string, error) {
	m.mu.Lock()
	var r *Recuerdo
	if len(m.Recuerdos) > 0 {
		r = m.Recuerdos[rng.Intn(len(m.Recuerdos))]
	}
	m.mu.Unlock()
	pared, suelo := [3]float64{0.5, 0.48, 0.45}, [3]float64{0.35, 0.33, 0.3}
	nombre := "lo que imagina"
	var figs []Figura
	var ras Rasgos
	if r != nil {
		pared, suelo, nombre, figs, ras = tallerVivo(r.Rasgos.Pared), tallerVivo(r.Rasgos.Suelo), r.Nombre, r.Figuras, r.Rasgos
	}
	op := rng.Intn(4)
	if op == 0 && len(figs) == 0 {
		op = 1
	}
	switch op {
	case 0:
		f := figs[rng.Intn(len(figs))]
		e := &tallerEsbozo{}
		e.L("h := c.Alto()")
		e.L("m := c.Marca()")
		var pal, filas []string
		for _, p := range f.Paleta {
			pal = append(pal, tallerCol(p))
		}
		for _, fl := range f.Forma {
			filas = append(filas, fmt.Sprintf("%q", fl))
		}
		k := math.Min(1, 1.2/math.Max(f.Alto, 0.1))
		e.L("c.Figura([]string{%s}, []mundo.Color{%s}, %s, %s, 0.06, true)", strings.Join(filas, ", "), strings.Join(pal, ", "), f2(f.Ancho*k), f2(f.Alto*k))
		e.L("c.MoverDesde(m, 0, h, 0)")
		c, err := tallerAnadir(codigo, "Nyx: «"+f.ID+"», de «"+nombre+"»", e.cuerpo.String())
		return "le puse encima algo que vi en «" + nombre + "»", c, err
	case 1:
		c, err := tallerAnadir(codigo, "Nyx: un suelo como el de «"+nombre+"»", fmt.Sprintf(`r := c.Ancho() + 0.6
c.Revolucion(0, 0, 0, []float64{r, 0, r, 0.08, r*0.97, 0.12}, 24, %s)
c.Toro(0, 0.12, 0, r*0.97, 0.05, %s)`, tallerCol(suelo), tallerCol(pared)))
		return "le hice un suelo del color de «" + nombre + "»", c, err
	case 2:
		esc := tallerValor(codigo, "escala")
		if esc == 0 {
			esc = 1
		}
		k := 0.85 + ras.Altura*0.6 + rng.Float64()*0.2
		if esc*k > 3 {
			k = 0.75
		}
		c, err := tallerRetocar(codigo, "escala", esc*k)
		return fmt.Sprintf("le cambié el tamaño (como los techos de «%s»)", nombre), c, err
	default:
		c, err := tallerRetocar(codigo, "tinte", pared[0], pared[1], pared[2], 0.2+rng.Float64()*0.3)
		return "la teñí del color de las paredes de «" + nombre + "» (" + nombreColor(pared) + ")", c, err
	}
}

// ---------- Nyx, en persona ----------

// tallerCodigoAvatarNyx: Nyx también camina por su mundo (y juega): alta,
// oscura, con ojos que brillan; dice lo último que ha dicho en el taller.
func tallerCodigoAvatarNyx(frases []string) string {
	c := tallerCodigoSerAbla(tallerSerAbla{Nombre: "Nyx", Tribu: 0, Concepto: "noche"}, frases)
	i := strings.Index(c, "func Construir(c *mundo.Cuerpo) {")
	j := strings.Index(c, "func Pensar(")
	if i < 0 || j < 0 {
		return c
	}
	cuerpo := `func Construir(c *mundo.Cuerpo) {
	c.Tubo(0, 0, 0, 0, 1.2, 0, 0.22, 0.14, mundo.RGB(0.16, 0.1, 0.24))
	c.Tubo(0, 1.2, 0, 0, 1.6, 0, 0.14, 0.1, mundo.RGB(0.2, 0.13, 0.3))
	c.Esfera(0, 1.82, 0, 0.2, mundo.RGB(0.12, 0.08, 0.18))
	c.Brilla(0.17, 1.86, 0.07, 0.035, mundo.RGB(0.8, 0.6, 1))
	c.Brilla(0.17, 1.86, -0.07, 0.035, mundo.RGB(0.8, 0.6, 1))
	c.Toro(0, 2.1, 0, 0.16, 0.02, mundo.RGB(0.75, 0.55, 1))
}

`
	cab := "// Nyx, en persona: camina por el mundo que construye (y juega).\n// Lo escribe el taller. Puedes cambiarlo.\n"
	return cab + c[strings.Index(c, "package ser"):i] + cuerpo + c[j:]
}

// ---------- los seres de Abla, en el mundo ----------

// tallerCodigoSerAbla: una entidad (para el mundo de Nyx) que es uno de
// los 27 de Abla: camina, se acerca si estás, y dice lo que piensa en
// Abla.
func tallerCodigoSerAbla(s tallerSerAbla, frases []string) string {
	col := [][3]float64{{0.55, 0.45, 0.95}, {0.3, 0.8, 0.75}, {0.95, 0.75, 0.3}}[s.Tribu%3]
	var b strings.Builder
	fmt.Fprintf(&b, "// %s, de la tribu %s de la especie Abla («quien %s»).\n// Lo escribe el taller con lo último que ha pensado. Puedes cambiarlo.\n", s.Nombre, tallerTribus[s.Tribu%3], s.Concepto)
	b.WriteString("package ser\n\nimport \"mundo\"\n\nvar frases = []string{\n")
	if len(frases) == 0 {
		frases = []string{s.Nombre}
	}
	for _, f := range frases {
		fmt.Fprintf(&b, "\t%q,\n", prefijo(f, 110))
	}
	b.WriteString("}\n\nfunc Construir(c *mundo.Cuerpo) {\n")
	cuerpo := tallerCol(col)
	brillo := tallerCol(tallerAclarar(col, 0.6))
	switch s.Tribu % 3 {
	case 0: // lenguaje: alto y fino, con la cabeza que brilla
		fmt.Fprintf(&b, "\tc.Tubo(0, 0, 0, 0, 1.5, 0, 0.18, 0.08, %s)\n\tc.Brilla(0, 1.75, 0, 0.22, %s)\n\tc.Toro(0, 1.75, 0, 0.35, 0.025, %s)\n", cuerpo, brillo, cuerpo)
	case 1: // data: cubos apilados
		fmt.Fprintf(&b, "\tfor k := 0; k < 4; k++ {\n\t\tm := c.Marca()\n\t\tc.Caja(0, 0.25+float64(k)*0.42, 0, 0.4, 0.4, 0.4, mundo.Mezcla(%s, %s, float64(k)/4))\n\t\tc.GirarDesde(m, float64(k)*0.4)\n\t}\n\tc.Brilla(0.21, 1.45, 0, 0.07, %s)\n", cuerpo, brillo, brillo)
	default: // lógica: esferas en equilibrio
		fmt.Fprintf(&b, "\tc.Esfera(0, 0.35, 0, 0.35, %s)\n\tc.Esfera(0, 0.95, 0, 0.25, %s)\n\tc.Esfera(0, 1.38, 0, 0.17, %s)\n\tc.Brilla(0.15, 1.42, 0, 0.05, %s)\n", cuerpo, tallerCol(tallerAclarar(col, 0.2)), tallerCol(tallerAclarar(col, 0.35)), brillo)
	}
	b.WriteString(`}

func Pensar(s *mundo.Ser, dt float64) {
	// si está jugando a algo, va a por ello
	if mx, mz, ok := mundo.Meta(s); ok {
		s.MirarHacia(mx, mz)
		s.Andar(1.8)
		return
	}
	jx, jz, d := s.Jugador()
	if d < 9 && d > 2.2 {
		s.MirarHacia(jx, jz)
		s.Andar(0.9)
	} else {
		if s.Choco() || s.Azar() < dt*0.3 {
			s.Girar((s.Azar() - 0.5) * 2.5)
		}
		s.Andar(0.6)
	}
	t := s.Recuerda("hablo")
	if s.Tiempo()-t > 7 && d < 12 {
		s.Recordar("hablo", s.Tiempo())
		k := int(s.Recuerda("frase"))
		s.Decir(frases[k%len(frases)])
		s.Recordar("frase", float64(k+1))
	}
}
`)
	return b.String()
}

// tallerGrande: las piezas de varios niveles, las que se mueven y los juegos.
func (e *tallerEsbozo) tallerGrande(idea string, rng *rand.Rand, pal [][3]float64, alta float64, oscuro bool) {
	switch idea {
	case "torre-mirador":
		e.torreMirador(rng, pal, alta)
	case "edificio":
		e.edificio(rng, pal, oscuro)
	case "mazmorra":
		e.mazmorra(rng, pal, 0)
	case "ascensor":
		e.ascensor(rng, pal, alta)
	case "anillos":
		e.anillos(rng, pal)
	case "plataformas":
		e.plataformas(rng, pal)
	case "molino":
		e.molino(rng, pal)
	case "campanario":
		e.campanario(rng, pal)
	default:
		e.torre(rng, pal, alta)
	}
}
