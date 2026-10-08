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
	"strconv"
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
	mat      string // el material que va poniendo (para no repetirlo)
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
	X        float64      `json:"x"`
	Y        float64      `json:"y"`
	Z        float64      `json:"z"`
	Rumbo    float64      `json:"rumbo"`
	Escala   float64      `json:"escala"`
	Color    [3]float64   `json:"color"`
	Tinte    string       `json:"tinte,omitempty"`    // el color dicho (tiñe el material)
	Material string       `json:"material,omitempty"` // madera, piedra, una palabra suya…
	Pila     [][8]float64 `json:"pila,omitempty"`
	Frases   int          `json:"frases"`
	Mano     string       `json:"mano,omitempty"` // la pieza que tiene cogida
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
	// dónde va lo siguiente
	gEncima
	gDebajo
	gJunto
	gDentro
	gAlrededor
	// nombres y materiales
	gNombrar
	gTextura
	gLiso
	gCaer
	// las manos (el editor)
	gTomar
	gSoltar
	gQuitar
	gCopiar
	gEscribir
	gDeshacer
	gInclinar
	// lo que pasa cuando alguien llega (juegos y eventos): para lo siguiente
	gPuerta
	gSaltar
	gPortal
	gPremio
	gCampana
	gSaludar
	// darle sentido a una palabra suya
	gAprender
)

var tallerSignificados = map[tallerGesto][]string{
	gSubir:     {"subir", "sube", "arriba", "alto", "alta", "cielo", "volar", "vuela", "nube", "estrella", "elevar", "crecer", "techo"},
	gBajar:     {"bajar", "baja", "abajo", "bajo", "tierra", "profundo", "profunda", "raiz", "hundir", "cueva", "fondo", "oscuro", "noche"},
	gAvanzar:   {"ir", "voy", "va", "camino", "lejos", "andar", "anda", "correr", "viaje", "seguir", "sigue", "adelante", "rio", "infinito"},
	gGirar:     {"girar", "gira", "vuelta", "redondo", "rueda", "ciclo", "torcer", "curva", "espiral"},
	gGrande:    {"grande", "mucho", "mucha", "enorme", "mas", "todo", "toda", "mundo", "inmenso", "gigante", "lleno"},
	gPequeno:   {"pequeno", "pequena", "poco", "poca", "menos", "nada", "breve", "punto"},
	gLuz:       {"luz", "brillar", "brilla", "fuego", "dia", "sol", "lampara", "claro", "ver", "ojo", "mirar"},
	gSuelo:     {"casa", "lugar", "sitio", "estar", "vivir", "aqui", "hogar", "piso", "suelo", "base", "plaza"},
	gRepetir:   {"repetir", "otra", "otro", "vez", "igual", "mismo", "misma", "todos", "muchos", "muchas", "siempre", "eco"},
	gSonido:    {"sonido", "voz", "palabra", "musica", "canto", "cantar", "decir", "hablar", "oir", "escuchar", "idioma", "abla"},
	gMover:     {"tiempo", "cambio", "cambiar", "vida", "vivo", "mover", "mueve", "bailar", "latir", "respirar", "despierto"},
	gGuardar:   {"recordar", "recuerdo", "memoria", "saber", "pensar", "pienso", "idea", "guardar"},
	gVolver:    {"volver", "vuelve", "olvidar", "regresar", "fin", "atras", "antes"},
	gEncima:    {"encima", "sobre", "apilar", "apila", "montar", "monta"},
	gDebajo:    {"debajo", "bajo de", "colgar", "cuelga"},
	gJunto:     {"junto", "lado", "cerca", "pegado", "vecino"},
	gDentro:    {"dentro", "interior", "corazon", "centro"},
	gAlrededor: {"alrededor", "rodear", "rodea", "anillo", "circulo", "corona"},
	gNombrar:   {"llamar", "llamo", "llama", "nombrar", "nombre", "bautizar", "apodar"},
	gTextura:   {"textura", "pintar", "pinta", "pinto", "material", "piel", "vestir"},
	gLiso:      {"liso", "lisa", "limpio", "desnudo"},
	gCaer:      {"caer", "cae", "gravedad", "posar", "posa", "aterrizar"},
	gTomar:     {"tomar", "toma", "tomo", "coger", "coge", "cojo", "agarrar", "agarra", "mano", "manos", "sostener", "elegir", "tocar", "toca"},
	gSoltar:    {"soltar", "suelta", "suelto", "dejar", "deja", "dejo", "libre", "liberar"},
	gQuitar:    {"quitar", "quita", "borrar", "borra", "romper", "rompe", "destruir", "desaparecer"},
	gCopiar:    {"copiar", "copia", "duplicar", "clonar", "gemelo", "doble"},
	gEscribir:  {"escribir", "escribe", "editar", "edita", "codigo", "construir", "construye", "hacer", "haz", "crear", "crea", "anadir", "poner", "pon"},
	gDeshacer:  {"deshacer", "deshaz", "arreglar", "arregla", "corregir"},
	gInclinar:  {"inclinar", "inclina", "tumbar", "tumba", "ladear", "ladea", "torcido"},
	gPuerta:    {"puerta", "abrir", "abre", "abierto", "cerrar", "cierra", "entrada", "salida", "ventana", "tapa"},
	gSaltar:    {"saltar", "salta", "salto", "brincar", "brinca", "rebotar", "rebota", "trampolin", "impulso", "lanzar"},
	gPortal:    {"portal", "viajar", "viaja", "cruzar", "cruza", "umbral", "atravesar", "teletransporte", "pasaje"},
	gPremio:    {"premio", "tesoro", "moneda", "ganar", "gana", "joya", "regalo", "jugar", "juega", "juego", "buscar", "encontrar"},
	gCampana:   {"campana", "tambor", "timbre", "sonar", "suena", "golpe", "golpear", "nota", "llamada"},
	gSaludar:   {"saludar", "saluda", "hola", "contar", "cuenta", "historia", "mensaje", "bienvenido", "bienvenida", "adios"},
	gAprender:  {"significa", "significar", "signo", "sentido", "aprender", "aprende", "aprendo", "ensenar", "ensena", "entender", "entiende"},
}

// tallerAmbientes: palabras que, además de lo que hacen, dejan sonando un
// ambiente en la pieza (el motor los sabe tocar).
var tallerAmbientes = map[string]string{
	"agua": "agua", "rio": "agua", "mar": "agua", "lluvia": "agua", "lago": "agua", "ola": "agua", "fuente": "agua",
	"viento": "viento", "aire": "viento", "tormenta": "viento", "soplar": "viento",
	"pajaro": "pajaros", "pajaros": "pajaros", "bosque": "pajaros", "arbol": "pajaros", "selva": "pajaros",
	"fuego": "fuego", "hoguera": "fuego", "arder": "fuego",
	"magia": "magia", "sueno": "magia", "misterio": "magia", "hechizo": "magia",
	"abeja": "zumbido", "maquina": "zumbido", "motor": "zumbido", "electrico": "zumbido",
}

// el nombre de un evento, para la charla
var tallerNombreEvento = map[tallerGesto]string{
	gMover: "moverse", gPuerta: "puerta", gSaltar: "trampolín", gPortal: "portal", gPremio: "premio", gCampana: "sonar al tocar",
}

var tallerNumeros = map[string]int{"dos": 2, "tres": 3, "cuatro": 4, "cinco": 5, "seis": 6, "siete": 7, "ocho": 8,
	"nueve": 9, "diez": 10, "once": 11, "doce": 12, "quince": 15, "veinte": 20, "treinta": 30, "cien": 40, "mil": 40, "par": 2}

// tallerEditor: las manos. Lo que se coge, se mueve, se gira, se inclina,
// se agranda, se tiñe, se pinta de un material, se apila, se deja caer, se
// copia, se quita, se deshace o se le escribe más código. Lo hace la obra
// (ver taller_charla.go); aquí solo se decide con las palabras.
type tallerEditor interface {
	Tomar(x, y, z float64) (base string, origen [3]float64, ok bool)
	TomarNombre(palabra string, x, y, z float64) (base string, origen [3]float64, ok bool)
	Origen(base string) ([3]float64, bool)
	Mover(base string, dx, dy, dz float64)
	Girar(base string, angulo float64)
	Inclinar(base string, angulo float64)
	Escalar(base string, k float64)
	Tenir(base string, col [3]float64)
	Pintar(base, material string)
	Apilar(base string) bool
	Caer(base string) bool
	Copiar(base string, x, y, z float64) string
	Quitar(base string)
	Deshacer(base string) bool
	Escribir(base, lineas string)
	SueloBajo(x, y, z float64) (float64, bool)
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
	"en": true, "un": true, "una": true, "unos": true, "unas": true, "al": true, "lo": true, "le": true, "les": true,
	"me": true, "te": true, "por": true, "con": true, "para": true, "es": true, "su": true, "sus": true, "mi": true,
	"tu": true, "o": true, "pero": true, "como": true, "este": true, "esta": true, "eso": true, "esto": true, "ya": true,
	"t": true, "soy": true, "he": true, "ha": true, "hay": true, "son": true, "se": true, "si": true, "muy": true,
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

// tallerPalabras: las palabras de lo que dijo (en Abla o en español; los
// números también cuentan).
func tallerPalabras(texto string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(texto), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(w)) < 2 && !unicode.IsDigit([]rune(w)[0]) {
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
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	return x
}

func tallerHex(c [3]float64) string {
	return fmt.Sprintf("%02x%02x%02x", int(clamp01(c[0])*255), int(clamp01(c[1])*255), int(clamp01(c[2])*255))
}

// ---------- de palabras a código ----------

// tallerObraHecha: lo que sale de una frase.
type tallerObraHecha struct {
	esbozo  *tallerEsbozo
	gestos  []string
	formas  int
	x, y, z float64  // dónde empieza la pieza
	Nombrar string   // si la llamaron de alguna manera: así se guarda la forma
	Juego   bool     // tiene premios: los seres pueden ir a jugar
	Aprendo []string // palabras suyas a las que dieron sentido («kevo = subir»)
}

// tallerHablarYConstruir: la tortuga recorre la frase; cada palabra, un
// gesto. Con ed (las manos), las palabras también cogen y cambian lo que
// ya existe.
func tallerHablarYConstruir(t *tallerTortuga, palabras []string, frase string, ed tallerEditor) *tallerObraHecha {
	if t.Escala <= 0 {
		t.Escala = 1
	}
	e := &tallerEsbozo{}
	f := &tallerObraHecha{esbozo: e, x: t.X, y: t.Y, z: t.Z}
	quien := "alguien"
	if q, ok := ed.(interface{ Quien() string }); ok {
		quien = q.Quien()
	}
	ox, oy, oz := t.X, t.Y, t.Z
	gesto := func(s string) {
		if len(f.gestos) < 48 {
			f.gestos = append(f.gestos, s)
		}
	}
	var ultima struct {
		ok                          bool
		w                           string
		cx, cz, bot, top, ancho, ru float64
	}
	cuenta := 0
	modo := gForma
	sonido, saludo := false, false
	ambientes := map[string]bool{}
	// especial: lo siguiente que se ponga se mueve, es una puerta, un
	// trampolín, un portal, un premio o suena al tocarlo
	especial, especialW := gForma, ""
	partes := 0
	type portal struct {
		nombre     string
		x, y, z, h float64
	}
	var portales []portal
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
	material := func() string {
		if t.Material == "" {
			return ""
		}
		if t.Tinte != "" {
			return t.Material + "|" + t.Tinte
		}
		return t.Material
	}
	// poner: la forma de una palabra (una o varias veces, donde toque)
	poner := func(w string) {
		dst, bx, by, bz := e, ox, oy, oz
		if escrito != nil {
			dst, bx, by, bz = escrito, eo[0], eo[1], eo[2]
		}
		n := max(1, cuenta)
		if modo == gAlrededor && cuenta == 0 {
			n = 6
		}
		cuenta = 0
		esc := t.Escala
		if modo == gDentro {
			esc *= 0.5
		}
		esp, espW := especial, especialW
		especial, especialW = gForma, ""
		if escrito != nil {
			esp = gForma // en el código de otra pieza no hay eventos
		}
		if esp == gPremio {
			esc *= 0.5 // los premios, pequeños (se cogen al pasar)
		}
		a := tallerAnchoForma(w, esc)
		alto := tallerAlturaForma(w, esc)
		ca, sa := math.Cos(t.Rumbo), math.Sin(t.Rumbo)
		if m := material(); m != dst.mat {
			dst.L("c.Material(%q)", m)
			dst.mat = m
		}
		var cx, cz, y float64
		for k := 0; k < n; k++ {
			switch {
			case modo == gEncima && ultima.ok:
				cx, cz, y = ultima.cx, ultima.cz, ultima.top+float64(k)*alto
			case modo == gDebajo && ultima.ok:
				cx, cz, y = ultima.cx, ultima.cz, ultima.bot-alto*float64(k+1)
			case modo == gDentro && ultima.ok:
				cx, cz, y = ultima.cx, ultima.cz, ultima.bot
			case modo == gAlrededor && ultima.ok:
				r := ultima.ancho/2 + a/2 + 0.2
				ang := ultima.ru + float64(k)/float64(n)*2*math.Pi
				cx, cz, y = ultima.cx+math.Cos(ang)*r, ultima.cz+math.Sin(ang)*r, ultima.bot
			default:
				cx, cz, y = t.X+ca*a/2, t.Z+sa*a/2, t.Y
				t.X += ca * a
				t.Z += sa * a
			}
			linea := tallerFormaEn(w, esc, t.Rumbo, cx-bx, y-by, cz-bz, t.Color)
			if esp != gForma {
				partes++
				nombre := fmt.Sprintf("p%d", partes)
				px, py, pz := cx-bx, y-by, cz-bz
				switch esp {
				case gSaltar:
					py += alto // se pisa por arriba
				case gPremio, gCampana:
					py += alto / 2
				}
				dst.L("c.Parte(%q, %s, %s, %s)", nombre, f2(px), f2(py), f2(pz))
				if esp == gPremio || esp == gPortal {
					dst.L("c.Fantasma(true) // se atraviesa")
				}
				dst.L("%s", linea)
				if esp == gPremio || esp == gPortal {
					dst.L("c.Fantasma(false)")
				}
				dst.L("c.Parte(\"\", 0, 0, 0)")
				h := tallerHash(w)
				switch esp {
				case gMover:
					vel := 0.3 + float64(h%7)*0.25
					if h%2 == 0 {
						e.S("Actuar", fmt.Sprintf(`o.Rotar(%q, 0, o.Tiempo()*%s, 0)`, nombre, f2(vel)))
					} else {
						e.S("Actuar", fmt.Sprintf(`o.Mover(%q, 0, mundo.Sin(o.Tiempo()*%s)*%s, 0)`, nombre, f2(vel), f2(esc)))
					}
				case gPuerta:
					// se abre (sube) y se cierra al usarla (E), y se acuerda
					sube := f2(alto + 0.2)
					e.S("Empezar", fmt.Sprintf(`if o.Valor(%q) == 1 {
o.Mover(%q, 0, %s, 0)
}`, nombre, nombre, sube))
					e.S("AlUsar", fmt.Sprintf(`if o.Valor(%q) == 0 {
o.Mover(%q, 0, %s, 0)
o.Guardar(%q, 1)
} else {
o.Mover(%q, 0, 0, 0)
o.Guardar(%q, 0)
}
o.SonarEn(%q, "puerta", 1)`, nombre, nombre, sube, nombre, nombre, nombre, nombre))
				case gSaltar:
					// al pisarlo, te lanza hacia arriba
					sube := f2(py + 4 + float64(h%9)*1.5)
					e.S("AlTocar", fmt.Sprintf(`if parte == %q {
o.Llevar(%s, %s, %s)
o.SonarEn(%q, "magia", 1.5)
}`, nombre, f2(px), sube, f2(pz), nombre))
				case gPortal:
					// te lleva a otro sitio (se decide al acabar la frase)
					portales = append(portales, portal{nombre, px, py, pz, float64(h % 1000)})
				case gPremio:
					// gira; al tocarlo, un punto (y vuelve al minuto)
					e.S("Actuar", fmt.Sprintf(`if o.Visible(%q) {
o.Rotar(%q, 0, o.Tiempo()*2, 0)
} else if o.Tiempo()-o.Valor(%q) > 60 || o.Tiempo() < o.Valor(%q) {
o.Mostrar(%q, true)
}`, nombre, nombre, "t"+nombre, "t"+nombre, nombre))
					e.S("AlTocar", fmt.Sprintf(`if parte == %q && o.Visible(%q) {
o.Mostrar(%q, false)
o.Guardar(%q, o.Tiempo())
o.Puntos(quien, 1)
o.SonarEn(%q, "moneda", 1)
}`, nombre, nombre, nombre, "t"+nombre, nombre))
					f.Juego = true
				case gCampana:
					son, tono := "nota", f2(48+float64(h%24))
					switch espW {
					case "campana", "timbre", "llamada":
						son, tono = "campana", f2(0.7+float64(h%5)*0.15)
					case "tambor", "golpe", "golpear":
						son, tono = "tambor", f2(0.8+float64(h%5)*0.1)
					}
					e.S("AlTocar", fmt.Sprintf(`if parte == %q {
o.SonarEn(%q, %q, %s)
}`, nombre, nombre, son, tono))
				}
			} else {
				dst.L("%s", linea)
			}
			if escrito == nil {
				f.formas++ // lo escrito en otra pieza no cuenta como pieza nueva
			}
		}
		quedo := " ×" + fmt.Sprint(n)
		if n == 1 {
			quedo = ""
		}
		switch modo {
		case gEncima:
			gesto(w + " encima" + quedo)
		case gDebajo:
			gesto(w + " debajo" + quedo)
		case gDentro:
			gesto(w + " dentro")
		case gAlrededor:
			gesto(w + " alrededor" + quedo)
		default:
			gesto(w + quedo)
		}
		if escrito != nil {
			gesto("(escrito en su código)")
		}
		if modo != gAlrededor || !ultima.ok { // lo de alrededor sigue rodeando a lo mismo
			ultima.ok, ultima.w, ultima.cx, ultima.cz, ultima.bot, ultima.top, ultima.ancho, ultima.ru = true, w, cx, cz, y, y+alto, a, t.Rumbo
		}
		if modo == gForma {
			h := tallerHash(w)
			t.Rumbo += (float64((h>>40)%5) - 2) * math.Pi / 12
			hue := float64((h>>16)%360) / 360
			c := TallerHSV(hue, 0.55, 0.85)
			t.Color = [3]float64{t.Color[0]*0.75 + c.R*0.25, t.Color[1]*0.75 + c.G*0.25, t.Color[2]*0.75 + c.B*0.25}
		}
		modo = gForma
	}
	siguiente := func(i int) string {
		if i+1 < len(palabras) {
			return palabras[i+1]
		}
		return ""
	}
	const maxPalabras = 48
	for i := 0; i < len(palabras) && i < maxPalabras; i++ {
		w := palabras[i]
		base := tallerSinTildes.Replace(w)
		// una palabra suya a la que ya le dieron sentido
		if s, ok := tallerSignificado(base); ok {
			base = s
		}
		h := tallerHash(w)
		// «X significa Y»: desde ahora X hace lo que hace Y
		if g, ok := tallerGestoDe[base]; ok && g == gAprender {
			if i > 0 && i+1 < len(palabras) {
				x, y := palabras[i-1], palabras[i+1]
				if tallerAprender(x, y, quien) {
					i++
					f.Aprendo = append(f.Aprendo, x+" = "+y)
					gesto("«" + x + "» ahora es «" + y + "»")
				}
			}
			continue
		}
		// el sonido de lo que nombran (agua, viento, pájaros…)
		if s, ok := tallerAmbientes[base]; ok && escrito == nil && !ambientes[s] && len(ambientes) < 3 {
			ambientes[s] = true
			e.S("Empezar", fmt.Sprintf(`o.Ambiente(%q, 0.5)`, s))
			gesto("suena a " + s)
		}
		paso := (1 + float64(h%4)) * t.Escala
		// un número: cuántas de lo siguiente
		if n, ok := tallerNumeros[base]; ok {
			cuenta = n
			continue
		}
		if n, err := strconv.Atoi(base); err == nil {
			cuenta = max(1, min(n, 40))
			continue
		}
		// los colores
		if col, ok := tallerColores[base]; ok {
			if enMano() && escrito == nil {
				ed.Tenir(t.Mano, col)
				gesto("lo tiñe de " + base)
			} else {
				t.Color, t.Tinte = col, tallerHex(col)
				gesto(base)
			}
			continue
		}
		// los materiales
		if tallerMateriales[base] {
			if enMano() && escrito == nil {
				m := base
				if t.Tinte != "" {
					m += "|" + t.Tinte
				}
				ed.Pintar(t.Mano, m)
				gesto("lo hace de " + base)
			} else {
				t.Material = base
				gesto("de " + base)
			}
			continue
		}
		g, ok := tallerGestoDe[base]
		if !ok {
			g = gForma
		}
		// con algo en la mano, la dirección, el tamaño y la forma de estar
		// son para eso que tiene cogido
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
			case gJunto:
				dx, dz := -math.Sin(t.Rumbo)*paso*1.5, math.Cos(t.Rumbo)*paso*1.5
				ed.Mover(t.Mano, dx, 0, dz)
				gesto("lo aparta a un lado")
			case gGirar:
				ed.Girar(t.Mano, (float64(h%5)-2+0.5)*math.Pi/4)
				gesto("lo gira")
			case gInclinar:
				ed.Inclinar(t.Mano, (float64(h%3)-1+0.5)*math.Pi/6)
				gesto("lo inclina")
			case gGrande:
				ed.Escalar(t.Mano, 1.5)
				gesto("lo agranda")
			case gPequeno:
				ed.Escalar(t.Mano, 1/1.5)
				gesto("lo achica")
			case gEncima:
				if ed.Apilar(t.Mano) {
					gesto("lo pone encima de otra cosa")
				}
			case gCaer:
				if ed.Caer(t.Mano) {
					gesto("lo deja caer")
				}
			case gDeshacer:
				if ed.Deshacer(t.Mano) {
					gesto("deshace su último cambio")
				}
			case gTextura:
				if m := siguiente(i); m != "" {
					i++
					m = tallerSinTildes.Replace(m)
					if t.Tinte != "" {
						m += "|" + t.Tinte
					}
					ed.Pintar(t.Mano, m)
					gesto("lo pinta de «" + palabras[i] + "»")
				}
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
			if ed == nil {
				continue
			}
			// «coger <nombre>»: lo que se llame así; si no, lo más cercano
			if nombre := siguiente(i); nombre != "" {
				if _, esGesto := tallerGestoDe[tallerSinTildes.Replace(nombre)]; !esGesto {
					if b, o, ok := ed.TomarNombre(nombre, t.X, t.Y, t.Z); ok {
						i++
						t.Mano = b
						t.X, t.Y, t.Z = o[0], o[1], o[2]
						gesto("coge «" + b + "»")
						continue
					}
				}
			}
			if b, o, ok := ed.Tomar(t.X, t.Y, t.Z); ok {
				t.Mano = b
				t.X, t.Y, t.Z = o[0], o[1], o[2]
				gesto("coge «" + b + "»")
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
		case gNombrar:
			if n := siguiente(i); n != "" {
				i++
				f.Nombrar = n
				gesto("lo llama «" + n + "»")
			}
			continue
		case gTextura:
			if m := siguiente(i); m != "" {
				i++
				t.Material = tallerSinTildes.Replace(m)
				gesto("con textura de «" + palabras[i] + "»")
			}
			continue
		case gLiso:
			t.Material, t.Tinte = "", ""
			gesto("liso")
			continue
		case gCaer:
			if ed != nil {
				if y, ok := ed.SueloBajo(t.X, t.Y, t.Z); ok {
					t.Y = y
					gesto("baja hasta lo de debajo")
				}
			}
			continue
		case gEncima, gDebajo, gDentro, gAlrededor:
			if ultima.ok {
				modo = g
			}
			continue
		case gJunto:
			if ultima.ok {
				d := math.Max(1, ultima.ancho) + 0.2
				t.X, t.Z = ultima.cx-math.Sin(t.Rumbo)*d, ultima.cz+math.Cos(t.Rumbo)*d
				t.Y = ultima.bot
				gesto("al lado")
			}
			continue
		case gDeshacer, gInclinar:
			continue // sin nada en la mano no hacen nada
		case gPuerta, gSaltar, gPortal, gPremio, gCampana:
			especial, especialW = g, base
			gesto(tallerNombreEvento[g])
			continue
		case gSaludar:
			// al acercarse alguien, la pieza dice la frase
			if !saludo && escrito == nil {
				saludo = true
				e.S("Empezar", "o.Zona(5)")
				e.S("AlEntrar", fmt.Sprintf(`o.Decir(%q)`, prefijo(frase, 140)))
				gesto("saluda a quien llega")
			}
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
			if m := material(); m != e.mat {
				e.L("c.Material(%q)", m)
				e.mat = m
			}
			e.L("c.Caja(%s, %s, %s, %s, %s, %s, %s) // «%s»: un suelo", f2(t.X-ox), f2(t.Y-oy-0.1*t.Escala), f2(t.Z-oz), f2(l), f2(0.2*t.Escala), f2(l), tallerCol(t.Color), w)
			f.formas++
			gesto("suelo")
		case gRepetir:
			if ultima.ok {
				if cuenta == 0 {
					cuenta = 2 + int(h%3)
				}
				poner(ultima.w)
			}
		case gSonido:
			if !sonido {
				sonido = true
				e.S("Empezar", fmt.Sprintf(`o.Ambiente(%q, 0.3)`, "musica:"+tallerMelodia(frase)))
				gesto("música")
			}
		case gMover:
			especial = gMover
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
			poner(w)
		}
	}
	soltarEscrito()
	// los portales: al sitio que recordaron, o a donde acabó la frase; si
	// es el mismo sitio, a otro nivel (arriba o abajo)
	for _, p := range portales {
		dx, dy, dz := t.X-ox, t.Y-oy, t.Z-oz
		if n := len(t.Pila); n > 0 {
			dx, dy, dz = t.Pila[n-1][0]-ox, t.Pila[n-1][1]-oy, t.Pila[n-1][2]-oz
		}
		if math.Abs(dx-p.x)+math.Abs(dy-p.y)+math.Abs(dz-p.z) < 3 {
			dy = p.y + 15 + math.Mod(p.h, 25)
			if int(p.h)%2 == 1 {
				dy = p.y - 15 - math.Mod(p.h, 25)
			}
			dx += 2
		}
		e.S("AlTocar", fmt.Sprintf(`if parte == %q {
o.Llevar(%s, %s, %s)
o.SonarEn(%q, "magia", 0.8)
}`, p.nombre, f2(dx), f2(dy+0.3), f2(dz), p.nombre))
	}
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

func tallerAnchoForma(w string, esc float64) float64 {
	if f := tallerFormaPropia(tallerSinTildes.Replace(w)); f != nil {
		return math.Max(0.2, f.Ancho*esc)
	}
	a, _, _ := tallerMedidas(w, esc)
	return a
}

func tallerAlturaForma(w string, esc float64) float64 {
	if f := tallerFormaPropia(tallerSinTildes.Replace(w)); f != nil {
		return math.Max(0.1, f.Alto*esc)
	}
	a, b, _ := tallerMedidas(w, esc)
	switch tallerHash(w) % 8 {
	case 2:
		return a
	case 4:
		return a / 4
	case 6:
		return b * 0.4
	}
	return b
}

// tallerForma: la forma de una palabra donde está la tortuga (para quien
// la use sin colocación especial).
func tallerForma(w string, t *tallerTortuga, ox, oy, oz float64) string {
	a := tallerAnchoForma(w, t.Escala)
	return tallerFormaEn(w, t.Escala, t.Rumbo, t.X-ox+math.Cos(t.Rumbo)*a/2, t.Y-oy, t.Z-oz+math.Sin(t.Rumbo)*a/2, t.Color)
}

// tallerFormaEn: la línea de código de la forma de una palabra, centrada
// en (x, z) y apoyada en y. Siempre la misma forma para la misma palabra;
// si es una forma que guardaron con ese nombre, esa.
func tallerFormaEn(w string, esc, rumbo, x, y, z float64, color [3]float64) string {
	com := fmt.Sprintf(" // «%s»", w)
	if tallerFormaPropia(tallerSinTildes.Replace(w)) != nil {
		return fmt.Sprintf("c.Forma(%q, %s, %s, %s, %s, %s)%s", tallerSinTildes.Replace(w), f2(x), f2(y), f2(z), f2(esc), f2(rumbo), com)
	}
	h := tallerHash(w)
	a, b, c := tallerMedidas(w, esc)
	col := tallerCol(color)
	ca, sa := math.Cos(rumbo), math.Sin(rumbo)
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
		return fmt.Sprintf("c.Tubo(%s, %s, %s, %s, %s, %s, %s, %s, %s)%s", f2(x-ca*a/2), f2(y), f2(z-sa*a/2), f2(x+ca*a/2), f2(y+b), f2(z+sa*a/2), f2(0.08*esc+a/12), f2(0.05*esc+a/16), col, com)
	}
}

// tallerResumen: los gestos, para la charla.
func tallerResumen(f *tallerObraHecha) string {
	if len(f.gestos) == 0 {
		return "nada"
	}
	g := f.gestos
	if len(g) > 16 {
		g = append(append([]string(nil), g[:16]...), "…")
	}
	return strings.Join(g, " · ")
}
