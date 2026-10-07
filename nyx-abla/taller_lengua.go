package main

// ============================================================
//  TALLER · LENGUA — construir con palabras
// ------------------------------------------------------------
//  Nyx y Abla no tienen planos ni plantillas: no saben qué es una casa,
//  una torre o un árbol. Solo tienen sus idiomas y una instrucción:
//
//        «crea un mundo infinito con lo que sabes»
//
//  Lo que dicen y lo que piensan se convierte en construcción. Cada una
//  tiene un constructor (una «tortuga») que va por el mundo, sin límite
//  hacia ningún lado, ni hacia arriba ni hacia abajo, y cada palabra es
//  un gesto:
//
//   · las palabras que significan dirección la mueven: subir, arriba,
//     cielo… la suben; bajar, abajo, tierra, profundo… la bajan; ir,
//     camino, lejos… la llevan adelante; girar, vuelta… la giran;
//   · las de tamaño la agrandan o achican; las de color la tiñen; las de
//     luz encienden algo; las de lugar ponen un suelo donde pisar; las de
//     sonido dejan su música; las de tiempo hacen que lo siguiente se
//     mueva; recordar guarda dónde está, volver regresa allí;
//   · y TODAS las demás palabras (de Abla o de Nyx) son formas: cada
//     palabra es siempre la misma forma, del mismo tamaño y tono. Así su
//     idioma se convierte en un idioma visible: lo que repiten se repite
//     en el mundo.
//
//  De vez en cuando, en vez de empezar algo nuevo, una añade lo que dice
//  encima de lo último que hizo la otra.
// ============================================================

import (
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// tallerInstruccion: lo único que se les dice.
const tallerInstruccion = "crea un mundo infinito con lo que sabes"

// ---------- el código que escriben ----------

// tallerEsbozo: un programa a medio escribir.
type tallerEsbozo struct {
	Titulo   string
	ayudas   []string
	cuerpo   strings.Builder
	cabecera string
	puestas  map[string]bool
	scripts  map[string][]string
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

func (e *tallerEsbozo) L(formato string, a ...any) {
	fmt.Fprintf(&e.cuerpo, "\t"+formato+"\n", a...)
}

// S: un trozo de script, en su propio bloque.
func (e *tallerEsbozo) S(funcion, codigo string) {
	if e.scripts == nil {
		e.scripts = map[string][]string{}
	}
	e.scripts[funcion] = append(e.scripts[funcion], codigo)
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

func tallerCol(c [3]float64) string {
	return fmt.Sprintf("mundo.RGB(%s, %s, %s)", f2(clamp01(c[0])), f2(clamp01(c[1])), f2(clamp01(c[2])))
}

func tallerAclarar(c [3]float64, k float64) [3]float64 {
	return [3]float64{c[0] + (1-c[0])*k, c[1] + (1-c[1])*k, c[2] + (1-c[2])*k}
}

// tallerMelodia: una frase, convertida en música (cada letra, una nota de
// la escala pentatónica; los espacios, silencios).
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

// Brillante: lo que se construya mientras esté activo da luz propia (y no
// se choca: es luz).
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

// ---------- la tortuga: por dónde va construyendo cada una ----------

type tallerTortuga struct {
	X      float64      `json:"x"`
	Y      float64      `json:"y"`
	Z      float64      `json:"z"`
	Rumbo  float64      `json:"rumbo"`
	Escala float64      `json:"escala"`
	Color  [3]float64   `json:"color"`
	Pila   [][8]float64 `json:"pila,omitempty"`
	Frases int          `json:"frases"`
	Mano   string       `json:"mano,omitempty"` // la pieza que tiene cogida
}

func tallerNuevaTortuga(x, z float64, color [3]float64) *tallerTortuga {
	return &tallerTortuga{X: x, Z: z, Escala: 1, Color: color}
}

// ---------- el significado de las palabras ----------

type tallerGesto int

const (
	gForma tallerGesto = iota
	gSubir
	gBajar
	gAvanzar
	gGirar
	gGrande
	gPequeno
	gLuz
	gSuelo
	gRepetir
	gSonido
	gMover
	gGuardar
	gVolver
	gColor
	// las manos (el editor)
	gTomar
	gSoltar
	gQuitar
	gCopiar
	gEscribir
)

var tallerSignificados = map[tallerGesto][]string{
	gSubir:   {"subir", "sube", "arriba", "alto", "alta", "cielo", "sobre", "encima", "volar", "vuela", "nube", "estrella", "elevar", "crecer", "techo"},
	gBajar:   {"bajar", "baja", "abajo", "bajo", "tierra", "profundo", "profunda", "raiz", "hundir", "cueva", "fondo", "debajo", "caer", "cae", "suelo bajo", "oscuro", "noche"},
	gAvanzar: {"ir", "voy", "va", "camino", "lejos", "andar", "anda", "correr", "viaje", "seguir", "sigue", "adelante", "rio", "infinito", "mas alla"},
	gGirar:   {"girar", "gira", "vuelta", "circulo", "redondo", "rueda", "ciclo", "torcer", "curva", "espiral"},
	gGrande:  {"grande", "mucho", "mucha", "enorme", "mas", "todo", "toda", "mundo", "inmenso", "gigante", "lleno"},
	gPequeno: {"pequeno", "pequena", "poco", "poca", "menos", "uno", "una sola", "nada", "breve", "punto"},
	gLuz:     {"luz", "brillar", "brilla", "fuego", "dia", "sol", "lampara", "claro", "ver", "ojo", "mirar"},
	gSuelo:   {"casa", "lugar", "sitio", "estar", "vivir", "dentro", "aqui", "hogar", "piso", "suelo", "base", "plaza"},
	gRepetir: {"repetir", "otra", "otro", "vez", "igual", "mismo", "misma", "todos", "muchos", "muchas", "siempre", "eco"},
	gSonido:  {"sonido", "voz", "palabra", "musica", "canto", "cantar", "decir", "hablar", "oir", "escuchar", "idioma", "abla"},
	gMover:   {"tiempo", "cambio", "cambiar", "vida", "vivo", "mover", "mueve", "bailar", "latir", "respirar", "despierto"},
	gGuardar: {"recordar", "recuerdo", "memoria", "saber", "se", "pensar", "pienso", "idea", "guardar"},
	gVolver:  {"volver", "vuelve", "olvidar", "regresar", "fin", "atras", "antes"},
	// las manos: coger lo que ya hay (suyo o de la otra) y cambiarlo
	gTomar:    {"tomar", "toma", "tomo", "coger", "coge", "cojo", "agarrar", "agarra", "mano", "manos", "sostener", "elegir", "tocar", "toca"},
	gSoltar:   {"soltar", "suelta", "suelto", "dejar", "deja", "dejo", "libre", "liberar"},
	gQuitar:   {"quitar", "quita", "borrar", "borra", "romper", "rompe", "destruir", "deshacer", "desaparecer"},
	gCopiar:   {"copiar", "copia", "duplicar", "clonar", "gemelo", "doble"},
	gEscribir: {"escribir", "escribe", "editar", "edita", "codigo", "construir", "construye", "hacer", "haz", "crear", "crea", "anadir", "poner", "pon"},
}

// tallerEditor: las manos. Lo que se coge, se mueve, se gira, se agranda,
// se tiñe, se copia, se quita o se le escribe más código. Lo hace la obra
// (ver taller_charla.go); aquí solo se decide con las palabras.
type tallerEditor interface {
	Tomar(x, y, z float64) (base string, origen [3]float64, ok bool)
	Origen(base string) ([3]float64, bool)
	Mover(base string, dx, dy, dz float64)
	Girar(base string, angulo float64)
	Escalar(base string, k float64)
	Tenir(base string, col [3]float64)
	Copiar(base string, x, y, z float64) string
	Quitar(base string)
	Escribir(base, lineas string)
}

var tallerColores = map[string][3]float64{
	"rojo": {0.85, 0.2, 0.18}, "roja": {0.85, 0.2, 0.18}, "azul": {0.2, 0.4, 0.9}, "verde": {0.25, 0.7, 0.3},
	"amarillo": {0.95, 0.85, 0.25}, "amarilla": {0.95, 0.85, 0.25}, "blanco": {0.92, 0.92, 0.9}, "blanca": {0.92, 0.92, 0.9},
	"negro": {0.08, 0.08, 0.1}, "negra": {0.08, 0.08, 0.1}, "gris": {0.5, 0.5, 0.52}, "naranja": {0.95, 0.55, 0.15},
	"violeta": {0.55, 0.3, 0.85}, "morado": {0.5, 0.25, 0.7}, "rosa": {0.95, 0.55, 0.7}, "marron": {0.45, 0.3, 0.18},
	"turquesa": {0.2, 0.75, 0.75}, "dorado": {0.9, 0.75, 0.3}, "plata": {0.75, 0.77, 0.8},
}

// palabras que no dicen nada por sí solas
var tallerVacias = map[string]bool{
	"el": true, "la": true, "los": true, "las": true, "de": true, "del": true, "que": true, "y": true, "a": true,
	"en": true, "un": true, "unos": true, "unas": true, "al": true, "lo": true, "le": true, "les": true, "se": false,
	"me": true, "te": true, "por": true, "con": true, "para": true, "es": true, "su": true, "sus": true, "mi": true,
	"tu": true, "o": true, "pero": true, "como": true, "este": true, "esta": true, "eso": true, "esto": true, "ya": true,
	"t": true, "soy": true, "he": true, "ha": true, "hay": true, "son": true,
}

var tallerSinTildes = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")

var tallerGestoDe = func() map[string]tallerGesto {
	m := map[string]tallerGesto{}
	for g, ws := range tallerSignificados {
		for _, w := range ws {
			m[w] = g
		}
	}
	return m
}()

// tallerPalabras: las palabras de lo que dijo (en Abla o en español).
func tallerPalabras(texto string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(texto), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len([]rune(w)) < 2 && w != "y" {
			continue
		}
		if tallerVacias[tallerSinTildes.Replace(w)] {
			continue
		}
		out = append(out, w)
	}
	return out
}

func tallerHash(w string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(w))
	x := h.Sum64()
	// mezclar un poco más los bits
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	return x
}

// ---------- de palabras a código ----------

// tallerObraHecha: lo que sale de una frase: el código de una pieza (o lo
// que se añade a otra) y un resumen de los gestos.
type tallerObraHecha struct {
	esbozo  *tallerEsbozo
	gestos  []string
	formas  int
	x, y, z float64 // dónde empieza (la tortuga al empezar)
}

// tallerHablarYConstruir: la tortuga recorre la frase; cada palabra, un
// gesto. El código de la pieza nueva va relativo a donde empieza. Con ed
// (las manos), las palabras también cogen y cambian lo que ya existe.
func tallerHablarYConstruir(t *tallerTortuga, palabras []string, frase string, ed tallerEditor) *tallerObraHecha {
	if t.Escala <= 0 {
		t.Escala = 1
	}
	e := &tallerEsbozo{}
	f := &tallerObraHecha{esbozo: e, x: t.X, y: t.Y, z: t.Z}
	ox, oy, oz := t.X, t.Y, t.Z
	ultimaForma := ""
	moverSiguiente := false
	// escribir dentro de lo que tiene en la mano: otro esbozo, relativo a esa pieza
	var escrito *tallerEsbozo
	var eo [3]float64
	soltarEscrito := func() {
		if escrito != nil && ed != nil && t.Mano != "" && escrito.cuerpo.Len() > 0 {
			ed.Escribir(t.Mano, escrito.cuerpo.String())
		}
		escrito = nil
	}
	enMano := func() bool { return ed != nil && t.Mano != "" }
	partes := 0
	sonido := false
	gesto := func(s string) {
		if len(f.gestos) < 40 {
			f.gestos = append(f.gestos, s)
		}
	}
	const maxGestos = 32
	for i, w := range palabras {
		if i >= maxGestos {
			break
		}
		base := tallerSinTildes.Replace(w)
		h := tallerHash(w)
		paso := (1 + float64(h%4)) * t.Escala
		if col, ok := tallerColores[base]; ok {
			if enMano() && escrito == nil {
				ed.Tenir(t.Mano, col)
				gesto("lo tiñe de " + base)
			} else {
				t.Color = col
				gesto(base)
			}
			continue
		}
		g, ok := tallerGestoDe[base]
		if !ok {
			g = gForma
		}
		// con algo en la mano, los gestos de dirección, tamaño y color son
		// para eso que tiene cogido
		if enMano() && escrito == nil {
			hecho := true
			switch g {
			case gSubir:
				ed.Mover(t.Mano, 0, paso*0.9, 0)
				t.Y += paso * 0.9
				gesto("lo sube")
			case gBajar:
				ed.Mover(t.Mano, 0, -paso*0.9, 0)
				t.Y -= paso * 0.9
				gesto("lo baja")
			case gAvanzar:
				dx, dz := math.Cos(t.Rumbo)*paso*2, math.Sin(t.Rumbo)*paso*2
				ed.Mover(t.Mano, dx, 0, dz)
				t.X += dx
				t.Z += dz
				gesto("lo lleva")
			case gGirar:
				ed.Girar(t.Mano, (float64(h%5)-2+0.5)*math.Pi/4)
				gesto("lo gira")
			case gGrande:
				ed.Escalar(t.Mano, 1.5)
				gesto("lo agranda")
			case gPequeno:
				ed.Escalar(t.Mano, 1/1.5)
				gesto("lo achica")
			default:
				hecho = false
			}
			if hecho {
				continue
			}
		}
		switch g {
		case gTomar:
			soltarEscrito()
			if ed != nil {
				if b, o, ok := ed.Tomar(t.X, t.Y, t.Z); ok {
					t.Mano = b
					t.X, t.Y, t.Z = o[0], o[1], o[2]
					gesto("coge «" + b + "»")
				}
			}
			continue
		case gSoltar:
			soltarEscrito()
			if t.Mano != "" {
				gesto("lo suelta")
			}
			t.Mano = ""
			continue
		case gQuitar:
			soltarEscrito()
			if enMano() {
				ed.Quitar(t.Mano)
				gesto("lo quita")
				t.Mano = ""
			}
			continue
		case gCopiar:
			if enMano() {
				t.X += math.Cos(t.Rumbo) * paso * 3
				t.Z += math.Sin(t.Rumbo) * paso * 3
				if b := ed.Copiar(t.Mano, t.X, t.Y, t.Z); b != "" {
					t.Mano = b
					gesto("lo copia")
				}
			}
			continue
		case gEscribir:
			if enMano() && escrito == nil {
				escrito = &tallerEsbozo{}
				eo, _ = ed.Origen(t.Mano)
				gesto("escribe en su código")
			}
			continue
		}
		// mientras escribe en lo que tiene cogido, las formas van ahí
		if escrito != nil && g == gForma {
			escrito.L("%s", tallerForma(w, t, eo[0], eo[1], eo[2]))
			a, _, _ := tallerMedidas(w, t.Escala)
			t.X += math.Cos(t.Rumbo) * a
			t.Z += math.Sin(t.Rumbo) * a
			ultimaForma = w
			gesto(w + " (escrito)")
			continue
		}
		switch g {
		case gSubir:
			t.Y += paso * 0.9
			gesto("subir")
		case gBajar:
			t.Y -= paso * 0.9
			gesto("bajar")
		case gAvanzar:
			t.X += math.Cos(t.Rumbo) * paso * 2
			t.Z += math.Sin(t.Rumbo) * paso * 2
			gesto("avanzar")
		case gGirar:
			t.Rumbo += (float64(h%5) - 2 + 0.5) * math.Pi / 4
			gesto("girar")
		case gGrande:
			t.Escala = math.Min(t.Escala*1.5, 60)
			gesto("crecer")
		case gPequeno:
			t.Escala = math.Max(t.Escala/1.5, 0.05)
			gesto("encoger")
		case gLuz:
			e.L("c.Brilla(%s, %s, %s, %s, %s) // «%s»", f2(t.X-ox), f2(t.Y-oy+0.5*t.Escala), f2(t.Z-oz), f2(0.25*t.Escala), tallerCol(tallerAclarar(t.Color, 0.5)), w)
			f.formas++
			gesto("luz")
		case gSuelo:
			l := 4 * t.Escala
			e.L("c.Caja(%s, %s, %s, %s, %s, %s, %s) // «%s»: un suelo", f2(t.X-ox), f2(t.Y-oy-0.1*t.Escala), f2(t.Z-oz), f2(l), f2(0.2*t.Escala), f2(l), tallerCol(t.Color), w)
			f.formas++
			gesto("suelo")
		case gRepetir:
			if ultimaForma != "" {
				n := 2 + int(h%3)
				for k := 0; k < n; k++ {
					t.X += math.Cos(t.Rumbo) * paso
					t.Z += math.Sin(t.Rumbo) * paso
					e.L("%s", tallerForma(ultimaForma, t, ox, oy, oz))
					f.formas++
				}
				gesto(fmt.Sprintf("repetir×%d", n))
			}
		case gSonido:
			if !sonido {
				sonido = true
				e.S("Empezar", fmt.Sprintf(`o.Ambiente(%q, 0.3)`, "musica:"+tallerMelodia(frase)))
				gesto("música")
			}
		case gMover:
			moverSiguiente = true
			gesto("moverse")
		case gGuardar:
			if len(t.Pila) < 64 {
				t.Pila = append(t.Pila, [8]float64{t.X, t.Y, t.Z, t.Rumbo, t.Escala, t.Color[0], t.Color[1], t.Color[2]})
			}
			gesto("recordar")
		case gVolver:
			if n := len(t.Pila); n > 0 {
				p := t.Pila[n-1]
				t.Pila = t.Pila[:n-1]
				t.X, t.Y, t.Z, t.Rumbo, t.Escala, t.Color = p[0], p[1], p[2], p[3], p[4], [3]float64{p[5], p[6], p[7]}
				gesto("volver")
			}
		default: // una forma: la palabra misma
			if moverSiguiente {
				moverSiguiente = false
				partes++
				nombre := fmt.Sprintf("p%d", partes)
				e.L("c.Parte(%q, %s, %s, %s)", nombre, f2(t.X-ox), f2(t.Y-oy), f2(t.Z-oz))
				e.L("%s", tallerForma(w, t, ox, oy, oz))
				e.L("c.Parte(\"\", 0, 0, 0)")
				vel := 0.3 + float64(h%7)*0.25
				if h%2 == 0 {
					e.S("Actuar", fmt.Sprintf(`o.Rotar(%q, 0, o.Tiempo()*%s, 0)`, nombre, f2(vel)))
				} else {
					e.S("Actuar", fmt.Sprintf(`o.Mover(%q, 0, mundo.Sin(o.Tiempo()*%s)*%s, 0)`, nombre, f2(vel), f2(t.Escala)))
				}
			} else {
				e.L("%s", tallerForma(w, t, ox, oy, oz))
			}
			ultimaForma = w
			f.formas++
			gesto(w)
			// la forma la hace avanzar un poco, y la tuerce según la palabra
			a, _, _ := tallerMedidas(w, t.Escala)
			t.X += math.Cos(t.Rumbo) * a
			t.Z += math.Sin(t.Rumbo) * a
			t.Rumbo += (float64((h>>40)%5) - 2) * math.Pi / 12
			// el color se va mezclando con el tono de cada palabra
			hue := float64((h>>16)%360) / 360
			c := TallerHSV(hue, 0.55, 0.85)
			t.Color = [3]float64{t.Color[0]*0.75 + c.R*0.25, t.Color[1]*0.75 + c.G*0.25, t.Color[2]*0.75 + c.B*0.25}
		}
	}
	soltarEscrito()
	t.Frases++
	return f
}

// tallerMedidas: el tamaño de la forma de una palabra.
func tallerMedidas(w string, esc float64) (a, b, c float64) {
	h := tallerHash(w)
	a = (0.4 + float64((h>>8)&0xff)/255*2.6) * esc
	b = (0.4 + float64((h>>16)&0xff)/255*3.6) * esc
	c = (0.4 + float64((h>>24)&0xff)/255*2.6) * esc
	return
}

// tallerForma: la línea de código de la forma de una palabra, donde está
// la tortuga (siempre la misma forma para la misma palabra).
func tallerForma(w string, t *tallerTortuga, ox, oy, oz float64) string {
	h := tallerHash(w)
	a, b, c := tallerMedidas(w, t.Escala)
	col := tallerCol(t.Color)
	ca, sa := math.Cos(t.Rumbo), math.Sin(t.Rumbo)
	x, y, z := t.X-ox+ca*a/2, t.Y-oy, t.Z-oz+sa*a/2
	com := fmt.Sprintf(" // «%s»", w)
	switch h % 8 {
	case 0:
		return fmt.Sprintf("c.Caja(%s, %s, %s, %s, %s, %s, %s)%s", f2(x), f2(y+b/2), f2(z), f2(a), f2(b), f2(c), col, com)
	case 1:
		return fmt.Sprintf("c.Cilindro(%s, %s, %s, %s, %s, %s)%s", f2(x), f2(y), f2(z), f2(a/2), f2(b), col, com)
	case 2:
		return fmt.Sprintf("c.Esfera(%s, %s, %s, %s, %s)%s", f2(x), f2(y+a/2), f2(z), f2(a/2), col, com)
	case 3:
		return fmt.Sprintf("c.Cono(%s, %s, %s, %s, %s, %s)%s", f2(x), f2(y), f2(z), f2(a/2), f2(b), col, com)
	case 4:
		return fmt.Sprintf("c.Toro(%s, %s, %s, %s, %s, %s)%s", f2(x), f2(y+a/8), f2(z), f2(a/2), f2(a/8), col, com)
	case 5: // una lámina de pie, de frente a donde va
		px, pz := -sa*c/2, ca*c/2
		return fmt.Sprintf("c.Lamina(%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)%s\n\tc.Lamina(%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
			f2(x-px), f2(y), f2(z-pz), f2(x+px), f2(y), f2(z+pz), f2(x+px), f2(y+b), f2(z+pz), col, com,
			f2(x-px), f2(y), f2(z-pz), f2(x+px), f2(y+b), f2(z+pz), f2(x-px), f2(y+b), f2(z-pz), col)
	case 6: // un relieve
		alts := make([]string, 25)
		for i := range alts {
			alts[i] = f2(float64((h>>(i%60))&7) / 7 * b * 0.4)
		}
		return fmt.Sprintf("c.Superficie(%s, %s, %s, %s, %s, []float64{%s}, 5, []mundo.Color{%s})%s", f2(x-a/2), f2(y), f2(z-c/2), f2(a), f2(c), strings.Join(alts, ", "), col, com)
	default: // una viga inclinada
		return fmt.Sprintf("c.Tubo(%s, %s, %s, %s, %s, %s, %s, %s, %s)%s", f2(t.X-ox), f2(y), f2(t.Z-oz), f2(t.X-ox+ca*a), f2(y+b), f2(t.Z-oz+sa*a), f2(0.08*t.Escala+a/12), f2(0.05*t.Escala+a/16), col, com)
	}
}

// tallerResumen: los gestos, para la charla.
func tallerResumen(f *tallerObraHecha) string {
	if len(f.gestos) == 0 {
		return "nada"
	}
	g := f.gestos
	if len(g) > 14 {
		g = append(append([]string(nil), g[:14]...), "…")
	}
	return strings.Join(g, " · ")
}
