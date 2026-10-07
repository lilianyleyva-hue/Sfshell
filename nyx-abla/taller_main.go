package main

// ============================================================
//  TALLER — Nyx y Abla construyen un mundo juntas
//    nyx taller                 abre el taller (consola)
//    nyx taller <orden>         una sola orden
//    nyx-taller …               lo mismo
//  Todo es local: corre en tu ordenador, con tus archivos. Solo sale a
//  internet para bajar los vídeos que les enseñes (yt-dlp).
//
//  Estos archivos (taller_*.go) se añaden a Nyx sin cambiar ni una
//  línea suya: ni su cabeza (mente.go), ni Nyx Mundo. El taller
//  usa a Nyx Mundo como la usarías tú.
// ============================================================

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// init: si te llaman como «nyx taller» (o nyx-taller), el taller se
// encarga y Nyx no se entera; si no, no hace nada.
func init() {
	if len(os.Args) == 0 {
		return
	}
	if filepath.Base(os.Args[0]) == "nyx-taller" {
		tallerMain(os.Args[1:])
		os.Exit(0)
	}
	if len(os.Args) > 1 && (os.Args[1] == "taller" || os.Args[1] == "juntas") {
		tallerMain(os.Args[2:])
		os.Exit(0)
	}
}

const tallerAyuda = `TALLER DE NYX Y ABLA — construyen un mundo en 3D con lo que recuerdan
  camina                       abre el mundo (las ves trabajar mientras paseas)
                               en la ventana: WASD andar · Espacio saltar ·
                               V volar (Espacio sube, C baja) · E usar (puertas,
                               ascensores, cofres, campanas) · Y la charla
  ven <fuente>                 que vean algo juntas: un vídeo de YouTube u otra
                               web, un archivo de vídeo o una carpeta de fotos
  charla [n]                   lo que se han dicho
  di <texto>                   háblales a las dos
  pausa · sigue                que paren o sigan trabajando solas
  ritmo <segundos>             cada cuánto hacen algo (9 si no dices)

  LA OBRA
  obra                         todas las piezas (quién las hizo y qué cambiaron)
  codigo <pieza>               su código en Go (y dónde está)
  nyx crea [idea]              que Nyx haga algo: torre, árbol, fuente, casa,
                               arco, escalera, pirámide, estatua, farolas…
                               hacia arriba: mirador, edificio, ascensor
                               hacia abajo: mazmorra (sótano, cueva, bajar)
                               que se mueve y suena: molino, campanario
                               juegos: anillos, plataformas
  abla crea [idea]             que Abla haga algo (también estela, mural,
                               fractal, girasol, orbes…)
  jugad                        que Nyx y los de Abla jueguen al juego que tengas
                               cerca (con la ventana abierta)
  nyx retoca <pieza>           que Nyx cambie algo (suya o de Abla)
  abla retoca <pieza>          que Abla cambie algo
  retoca <pieza> escala <k> | giro <grados> | tinte <r> <g> <b> <fuerza>
  mueve <pieza> aqui | <x> <z> llevarla a otro sitio
  quita <pieza>                sacarla del mundo (su código se queda)
  recompila <pieza>            si cambiaste su .go a mano: nueva versión con lo tuyo
  nueva <archivo.go> [nombre]  tu propio código como pieza (package objeto,
                               func Construir(c *mundo.Cuerpo))
  formas                       todo lo que se puede usar para construir

  BLENDER
  importa <archivo.obj> [alto <metros>] [como <nombre>]
                               un modelo tuyo (con sus colores y materiales)
  exporta                      todo el mundo como una escena .obj + .mtl
  (cada pieza tiene además su .obj y .mtl en ~/.local/share/nyx-mundo/objetos)

  ABLA
  abla <orden>                 una orden para la consola de Abla (seres, mision,
                               fotos, dic buscar <palabra>, decir <ser> <texto>…)
  seres                        que algunos de los 27 salgan a caminar por el mundo
  estado · ayuda · sale
  Lo demás va a Nyx Mundo tal cual (aprende, recuerdos, entidades, crea una
  entidad…, autoentrena…): escribe «ayuda mundo» para ver sus órdenes.
`

const tallerFormas = `Para construir (paquete "mundo"), en objetos y entidades:
  c.Caja(x,y,z, ancho,alto,fondo, col)      c.Esfera(x,y,z, r, col)
  c.Brilla(x,y,z, r, col)  (da luz)          c.Tubo(x1,y1,z1, x2,y2,z2, r1,r2, col)
  c.Cilindro(x,y,z, r, alto, col)           c.Cono(x,y,z, r, alto, col)
  c.Toro(x,y,z, R, r, col)                  c.Figura(filas, paleta, ancho, alto, grosor, cruzada)
  c.Revolucion(x,y,z, []float64{r0,y0, r1,y1, …}, lados, col)
  c.Extruir(x,y,z, []float64{x0,z0, x1,z1, …}, alto, col)
  c.Superficie(x,y,z, ancho, fondo, alturas, columnas, colores)
  c.Triangulo(…9 números…, col) · c.Lamina(…) · c.Cuadro(…12 números…, col)
  c.Letras(texto, x,y,z, tam, fondo, col)
  m := c.Marca() · c.Copiar(m, dx,dy,dz, giro) · c.MoverDesde(m, dx,dy,dz)
  c.GirarDesde(m, a) · c.EscalarDesde(m, k) · c.LevantarDesde(m)
  c.Escalar(k) · c.Girar(a) · c.Mover(dx,dy,dz) · c.Tenir(col, fuerza)
  c.Alto() · c.Ancho() · c.MallaCodificada(datos)
  c.Parte("nombre", px,py,pz)  lo que venga es una parte que se mueve y gira
                               alrededor de ese pivote · c.Parte("", 0,0,0) vuelve
  c.Hueco(x0,z0, x1,z1)        ahí el suelo del mundo se abre (para bajar)
  c.Fantasma(true/false)       lo que venga se ve pero no choca
  c.Brillante(true/false)      lo que venga da luz (ventanas, botones)

Scripts (funciones que puede tener una pieza; todas opcionales):
  func Empezar(o *mundo.Objeto)                      al aparecer
  func Actuar(o *mundo.Objeto, dt float64)           10 veces por segundo
  func AlUsar(o *mundo.Objeto, quien string)         alguien pulsa E cerca
  func AlEntrar(o *mundo.Objeto, quien string)       alguien entra en su zona
  func AlSalir(o *mundo.Objeto, quien string)        … y sale
  func AlTocar(o *mundo.Objeto, quien, parte string) alguien toca una parte
  o.Mover(parte, x,y,z) · o.Rotar(parte, x,y,z) · o.Escalar(parte, k)
  o.Mostrar(parte, si) · o.Visible(parte) · o.Partes() · o.Zona(radio)
  o.Sonar(sonido, tono) · o.SonarEn(parte, sonido, tono)
       campana nota moneda tambor puerta magia victoria golpe paso
       o un archivo tuyo de ~/.local/share/nyx-mundo/taller/sonidos/
  o.Ambiente(sonido, volumen) · o.AmbienteEn(parte, sonido, volumen)
       agua fuego viento zumbido pajaros magia  o  "musica:60 62 64 - 67"
  o.Decir(texto) · o.Llevar(x,y,z) (te lleva: ascensores, trampillas)
  o.Puntos(quien, n) · o.PuntosDe(quien) · o.Guardar(clave, v) · o.Valor(clave)
  o.Tiempo() · o.Azar() · o.Jugador() (x, y, z, distancia) · o.Nombre() · o.Autor()
  Y los seres: mundo.Meta(s) dice adónde ir si están jugando.
  mundo.RGB(r,g,b) · mundo.HSV(h,s,v) · mundo.Mezcla(a,b,t) · mundo.Ruido(x,y,semilla)
  mundo.Pi Sin Cos Tan Asin Acos Atan Atan2 Sqrt Pow Exp Log Abs Min Max Hypot
        Floor Ceil Round Mod Tanh
`

type tallerSesion struct {
	o        *tallerObra
	parar    chan struct{}
	abierta  sync.Once
	ventana  string
	salidaMu sync.Mutex
}

func tallerMain(args []string) {
	m := NuevoMundo(dirMundo())
	o := tallerNuevaObra(m)
	s := &tallerSesion{o: o, parar: make(chan struct{})}
	o.avisarFn = func(l string) {
		s.salidaMu.Lock()
		fmt.Println("  " + l)
		s.salidaMu.Unlock()
	}
	cerrar := func() {
		close(s.parar)
		_ = o.Guardar()
		o.abla.Dormir()
		_ = m.Guardar()
	}
	senal := make(chan os.Signal, 1)
	signal.Notify(senal, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-senal
		fmt.Println("\ntaller: guardando a las dos…")
		cerrar()
		os.Exit(0)
	}()
	if err := o.abla.Despertar(); err != nil {
		fmt.Println("taller: Abla no está:", err)
		fmt.Println("        (Nyx trabajará sola hasta que esté)")
	}
	if len(args) > 0 {
		// una sola orden (sin trabajar solas)
		o.avisarFn = nil
		s.ejecutar(strings.Join(args, " "))
		if len(args) > 0 && (args[0] == "camina" || args[0] == "ventana") {
			go o.Trabajar(s.parar)
			select {}
		}
		cerrar()
		return
	}
	o.mu.Lock()
	np := len(o.Piezas)
	o.mu.Unlock()
	fmt.Printf("TALLER de Nyx y Abla · %d piezas en el mundo · Nyx recuerda %d sitios · Abla %s\n",
		np, len(m.ListaRecuerdos()), o.abla.Estado())
	fmt.Println("Ya están trabajando. Escribe 'camina' para verlas, 'ven <vídeo>' para enseñarles algo, 'ayuda' para más.")
	go o.Trabajar(s.parar)
	lector := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("taller› ")
		l, err := lector.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}
		if s.ejecutar(l) {
			break
		}
	}
	cerrar()
	fmt.Println("taller: guardado. Hasta luego.")
}

func (s *tallerSesion) decir(formato string, a ...any) {
	s.salidaMu.Lock()
	fmt.Printf(formato+"\n", a...)
	s.salidaMu.Unlock()
}

// ejecutar: una orden del taller. true = salir.
func (s *tallerSesion) ejecutar(linea string) bool {
	o := s.o
	linea = strings.TrimSpace(linea)
	if linea == "" {
		return false
	}
	partes := strings.Fields(linea)
	orden := strings.ToLower(partes[0])
	resto := strings.TrimSpace(strings.TrimPrefix(linea, partes[0]))
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	switch orden {
	case "ayuda", "help", "-h", "--help":
		if strings.HasPrefix(strings.ToLower(resto), "mundo") {
			ejecutaMundo(o.m, "ayuda")
		} else {
			fmt.Print(tallerAyuda)
		}
	case "formas":
		fmt.Print(tallerFormas)
	case "sale", "salir", "exit":
		return true
	case "estado":
		o.mu.Lock()
		s.decir("piezas: %d · turnos: %d · Abla vio %d fotos · %s", len(o.Piezas), o.Turno, len(o.Vistas),
			map[bool]string{true: "en pausa", false: fmt.Sprintf("trabajan cada %v", o.ritmo)}[o.pausa])
		o.mu.Unlock()
		s.decir("Nyx recuerda %d sitios · Abla: %s (%d fotos en cola)", len(o.m.ListaRecuerdos()), o.abla.Estado(), o.abla.EnCola())
		if s.ventana != "" {
			s.decir("ventana: %s", s.ventana)
		}
	case "camina", "ventana", "ui":
		s.abrirVentana()
	case "ven", "ved", "mirad", "aprended":
		if resto == "" {
			s.decir("uso: ven <vídeo de YouTube, archivo o carpeta>")
			break
		}
		go func() {
			if err := o.Ver(resto, func(l string) { s.decir("%s", l) }); err != nil {
				s.decir("taller: no pudieron verlo: %v", err)
			} else {
				s.decir("taller: Nyx ya lo recuerda; Abla lo sigue mirando (charla para ver qué dicen)")
			}
		}()
	case "charla":
		n := 20
		if v, err := strconv.Atoi(resto); err == nil && v > 0 {
			n = v
		}
		for _, f := range o.Charla(n) {
			g := ""
			if f.Glosa != "" {
				g = "  («" + f.Glosa + "»)"
			}
			s.decir("%s: %s%s", f.Quien, f.Texto, g)
		}
	case "di", "decid":
		if resto == "" {
			break
		}
		o.Decir("tú", resto, "")
		o.Decir("Nyx", prefijo(o.m.Responder(resto), 400), "")
		if o.abla.Viva() {
			quien, dijo, glosa := o.abla.DecirComo("Humano", fmt.Sprint(1+rng.Intn(27)), resto)
			if quien != "" {
				o.Decir("Abla · "+quien, dijo, glosa)
			} else {
				o.Decir("Abla", "(no entiende esas palabras: prueba con palabras sencillas o en Abla; «abla dic buscar <palabra>»)", "")
			}
		}
	case "pausa", "parad":
		o.mu.Lock()
		o.pausa = true
		o.mu.Unlock()
		s.decir("taller: descansan (siguen pensando, pero no construyen). 'sigue' para que vuelvan.")
	case "sigue", "seguid", "trabajad":
		o.mu.Lock()
		o.pausa = false
		o.mu.Unlock()
		s.decir("taller: vuelven a trabajar.")
	case "ritmo":
		v, err := strconv.ParseFloat(resto, 64)
		if err != nil || v < 1 {
			s.decir("uso: ritmo <segundos> (1 o más)")
			break
		}
		o.mu.Lock()
		o.ritmo = time.Duration(v * float64(time.Second))
		o.mu.Unlock()
		s.decir("taller: harán algo cada %v", time.Duration(v*float64(time.Second)))
	case "obra", "piezas":
		o.mu.Lock()
		ps := append([]*tallerPieza(nil), o.Piezas...)
		o.mu.Unlock()
		if len(ps) == 0 {
			s.decir("Todavía no hay nada: espera un poco (o 'nyx crea', 'abla crea').")
		}
		for i, p := range ps {
			s.decir("%3d. %-34s de %-4s  «%s»  (%.0f, %.0f)  de: %s", i+1, p.ID, p.Autor, p.Titulo, p.X, p.Z, prefijo(p.De, 40))
			for _, c := range p.Cambios {
				s.decir("       · %s", c)
			}
		}
	case "codigo", "código":
		p := o.Pieza(resto)
		if p == nil {
			s.decir("¿qué pieza? (los nombres o números salen en 'obra')")
			break
		}
		ruta := filepath.Join(o.m.dir, "objetos", p.ID+".go")
		d, err := os.ReadFile(ruta)
		if err != nil {
			s.decir("no encuentro %s", ruta)
			break
		}
		fmt.Print(string(d))
		s.decir("(está en %s; su modelo para Blender, en %s.obj)", ruta, strings.TrimSuffix(ruta, ".go"))
	case "nyx", "abla":
		s.ordenDeUna(orden, resto, rng)
	case "retoca":
		s.retocar(resto)
	case "mueve":
		f := strings.Fields(resto)
		if len(f) < 2 {
			s.decir("uso: mueve <pieza> aqui | <x> <z>")
			break
		}
		p := o.Pieza(f[0])
		if p == nil {
			s.decir("no encuentro la pieza %s", f[0])
			break
		}
		var x, z float64
		if strings.HasPrefix(f[1], "aq") {
			o.m.seresMu.Lock()
			x, z = o.m.jx+2, o.m.jz+2
			o.m.seresMu.Unlock()
		} else if len(f) >= 3 {
			x, _ = strconv.ParseFloat(f[1], 64)
			z, _ = strconv.ParseFloat(f[2], 64)
		}
		o.Mover(p, x, z, p.Rumbo)
		o.Decir("tú", fmt.Sprintf("muevo %s a (%.0f, %.0f)", p.Base, x, z), "")
	case "quita":
		p := o.Pieza(resto)
		if p == nil {
			s.decir("no encuentro esa pieza")
			break
		}
		o.Quitar(p)
		o.Decir("tú", "quito "+p.Base+" del mundo (su código se queda en objetos/)", "")
	case "recompila":
		p := o.Pieza(resto)
		if p == nil {
			s.decir("no encuentro esa pieza")
			break
		}
		if err := o.Cambiar(p, "tú", "lo cambiaste a mano", func(c string) (string, error) { return c, nil }); err != nil {
			s.decir("taller: %v", err)
		} else {
			s.decir("taller: %s ahora es %s", p.Base, p.ID)
		}
	case "nueva":
		f := strings.Fields(resto)
		if len(f) == 0 {
			s.decir("uso: nueva <archivo.go> [nombre]")
			break
		}
		d, err := os.ReadFile(f[0])
		if err != nil {
			s.decir("no puedo leerlo: %v", err)
			break
		}
		nombre := strings.TrimSuffix(filepath.Base(f[0]), ".go")
		if len(f) > 1 {
			nombre = strings.Join(f[1:], " ")
		}
		p, err := o.Crear("tú", nombre, f[0], string(d), math.NaN(), math.NaN())
		if err != nil {
			s.decir("no compila: %v", err)
			break
		}
		o.Decir("tú", "pongo mi pieza "+p.ID+" en el mundo", "")
	case "importa":
		s.importar(resto)
	case "exporta":
		ruta, n, err := o.ExportarEscena()
		if err != nil {
			s.decir("taller: %v", err)
			break
		}
		s.decir("taller: %d piezas en %s (ábrelo en Blender: Archivo → Importar → Wavefront .obj)", n, ruta)
	case "jugad", "jugar", "juega":
		if !o.motorEnMarcha() {
			s.decir("taller: primero abre el mundo (camina): se juega allí")
			break
		}
		if !o.Jugar(rng) {
			s.decir("taller: no hay ningún juego cerca de ti. Pide uno: «nyx crea anillos» o «abla crea plataformas»")
		}
	case "seres":
		if !o.abla.Viva() {
			s.decir("Abla está dormida: %s", o.abla.Estado())
			break
		}
		o.seresAblaAlMundo(rng, 5)
		s.decir("taller: algunos de los 27 caminan ya por el mundo (salen en 'entidades')")
	default:
		ejecutaMundo(o.m, linea)
	}
	return false
}

// ordenDeUna: «nyx crea …», «abla retoca …», «abla <orden de su consola>».
func (s *tallerSesion) ordenDeUna(quien, resto string, rng *mrand.Rand) {
	o := s.o
	f := strings.Fields(resto)
	sub := ""
	if len(f) > 0 {
		sub = strings.ToLower(f[0])
	}
	arg := strings.TrimSpace(strings.TrimPrefix(resto, sub))
	if len(f) > 0 {
		arg = strings.TrimSpace(resto[len(f[0]):])
	}
	switch {
	case sub == "crea" || sub == "haz" || sub == "construye":
		if quien == "nyx" {
			_, _ = o.creaNyx(rng, arg)
		} else {
			if !o.abla.Viva() {
				s.decir("Abla está dormida: %s", o.abla.Estado())
				return
			}
			tribu := -1
			for i, t := range tallerTribus {
				if strings.Contains(strings.ToLower(arg), t) {
					tribu = i
				}
			}
			_, _ = o.creaAbla(rng, arg, tribu)
		}
	case sub == "retoca" || sub == "cambia":
		p := o.Pieza(arg)
		if p == nil {
			s.decir("¿qué pieza? (las ves en 'obra')")
			return
		}
		if quien == "nyx" {
			o.retocaNyx(rng, p.Autor, p)
		} else {
			o.retocaAbla(rng, p.Autor, p)
		}
	case quien == "abla":
		if resto == "" {
			s.decir("Abla: %s", o.abla.Estado())
			return
		}
		fmt.Println(o.abla.Ejecutar(resto))
	default:
		fmt.Println("nyx-mundo:", o.m.Responder(resto))
	}
}

func (s *tallerSesion) retocar(resto string) {
	o := s.o
	f := strings.Fields(resto)
	if len(f) < 3 {
		s.decir("uso: retoca <pieza> escala <k> | giro <grados> | tinte <r> <g> <b> <fuerza>")
		return
	}
	p := o.Pieza(f[0])
	if p == nil {
		s.decir("no encuentro la pieza %s", f[0])
		return
	}
	var nums []float64
	for _, x := range f[2:] {
		v, err := strconv.ParseFloat(x, 64)
		if err != nil {
			s.decir("%q no es un número", x)
			return
		}
		nums = append(nums, v)
	}
	que := strings.ToLower(f[1])
	switch que {
	case "giro":
		nums[0] = nums[0] * math.Pi / 180
	case "tinte":
		if len(nums) < 4 {
			s.decir("uso: retoca <pieza> tinte <r> <g> <b> <fuerza>   (de 0 a 1)")
			return
		}
	case "escala":
	default:
		s.decir("se puede retocar: escala, giro o tinte")
		return
	}
	err := o.Cambiar(p, "tú", "retocas "+que, func(c string) (string, error) { return tallerRetocar(c, que, nums...) })
	if err != nil {
		s.decir("taller: %v", err)
		return
	}
	o.Decir("tú", fmt.Sprintf("retoco %s (%s %s)", p.Base, que, strings.Join(f[2:], " ")), "")
}

func (s *tallerSesion) importar(resto string) {
	f := strings.Fields(resto)
	if len(f) == 0 {
		s.decir("uso: importa <archivo.obj> [alto <metros>] [como <nombre>]")
		return
	}
	ruta, alto, nombre := f[0], 0.0, strings.TrimSuffix(filepath.Base(f[0]), filepath.Ext(f[0]))
	for i := 1; i < len(f); i++ {
		switch f[i] {
		case "alto":
			if i+1 < len(f) {
				alto, _ = strconv.ParseFloat(f[i+1], 64)
				i++
			}
		case "como":
			nombre = strings.Join(f[i+1:], " ")
			i = len(f)
		}
	}
	s.decir("taller: importando %s…", ruta)
	p, err := s.o.ImportarOBJ(ruta, nombre, alto)
	if err != nil {
		s.decir("taller: no pude: %v", err)
		return
	}
	s.o.Decir("tú", fmt.Sprintf("traigo «%s» de Blender: es %s (%.1f m de alto)", nombre, p.ID, p.Alto), "")
}

// ---------- la ventana ----------

func (s *tallerSesion) abrirVentana() {
	if s.ventana != "" {
		s.decir("taller: la ventana ya está en %s", s.ventana)
		go abrirNavegador(s.ventana)
		return
	}
	listo := make(chan string, 1)
	go func() {
		if err := tallerServir(s.o, 8471, listo); err != nil {
			s.decir("taller: la ventana falló: %v", err)
			listo <- ""
		}
	}()
	s.ventana = <-listo
	if s.ventana != "" {
		s.decir("taller: ventana en %s (sigues aquí: puedes enseñarles vídeos mientras paseas)", s.ventana)
	}
}

// tallerServir: la misma ventana de Nyx Mundo (con todo lo suyo), con las
// piezas del taller en el mundo y la charla de las dos encima.
func tallerServir(o *tallerObra, puerto int, listo chan<- string) error {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	srv := &servidorMundo{m: o.m, codigo: hex.EncodeToString(b)}
	suyo := srv.rutas()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			suyo.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'unsafe-inline'; img-src 'self' data:")
		pag := strings.Replace(paginaMundo, "{{CODIGO}}", srv.codigo, 1)
		if i := strings.LastIndex(pag, "</body>"); i >= 0 {
			pag = pag[:i] + tallerPanel + "<script>\n" + tallerMotorJS + "\n</script>\n" + pag[i:]
		} else {
			pag += tallerPanel + "<script>\n" + tallerMotorJS + "\n</script>\n"
		}
		_, _ = io.WriteString(w, pag)
	})
	// el motor: partes que se mueven, física exacta, sonidos, juegos
	o.motor.rutas(mux, suyo, srv.codigo)
	parar := make(chan struct{})
	go o.motor.Correr(parar)
	o.mu.Lock()
	o.motorVivo = true
	o.mu.Unlock()
	mux.HandleFunc("/api/taller", func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		trozos := map[string]int{}
		for k, v := range o.trozos {
			trozos[k] = v
		}
		res := map[string]any{"version": o.version, "trozos": trozos, "piezas": len(o.Piezas), "viendo": o.viendo, "pausa": o.pausa}
		o.mu.Unlock()
		res["charla"] = o.Charla(14)
		res["abla"] = o.abla.Estado()
		responder(w, res, nil)
	})
	mux.HandleFunc("/api/taller-di", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Nyx-Codigo")), []byte(srv.codigo)) != 1 {
			http.Error(w, "sin permiso", http.StatusForbidden)
			return
		}
		d, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		t := strings.TrimSpace(string(d))
		if t != "" {
			go (&tallerSesion{o: o}).ejecutar("di " + t)
		}
		responder(w, map[string]any{"ok": true}, nil)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", puerto))
	if err != nil {
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return err
		}
	}
	dir := "http://" + ln.Addr().String() + "/"
	listo <- dir
	go abrirNavegador(dir)
	// lo mismo que hace la ventana de Nyx Mundo: se entrena, los seres viven
	go o.m.Entrenar(make(chan struct{}), false, 3*time.Second)
	go func() {
		t := time.NewTicker(50 * time.Millisecond)
		vueltas := 0
		for range t.C {
			o.m.Vivir(0.05)
			o.m.Acontecer(0.05)
			if vueltas++; vueltas%40 == 0 {
				for _, l := range o.m.Recargar() {
					o.Decir("taller", l, "")
				}
			}
		}
	}()
	return (&http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}).Serve(ln)
}

// tallerMotorJS: lo que el taller añade a la ventana (caminar con física,
// las piezas que se mueven, el sonido en 3D). Está en taller_motor.js.
//
//go:embed taller_motor.js
var tallerMotorJS string

// tallerMismoCodigo: la petición viene de la ventana (lleva su código).
func tallerMismoCodigo(r *http.Request, codigo string) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Nyx-Codigo")), []byte(codigo)) == 1
}

// tallerPanel: la charla encima del mundo (Y la muestra u oculta) y, al
// cambiar una pieza, se vuelve a pedir solo su trozo.
const tallerPanel = `
<div id="taller" style="position:fixed;right:14px;bottom:14px;width:min(430px,44vw);max-height:46vh;overflow:auto;
 background:rgba(8,8,14,.72);color:#e8e4da;font:13px/1.45 system-ui,sans-serif;border-radius:10px;padding:10px 12px;
 border:1px solid rgba(255,255,255,.12);z-index:20">
 <div style="display:flex;justify-content:space-between;gap:8px;align-items:baseline">
  <b>Taller · Nyx ↔ Abla</b><span id="taller-e" style="font-size:11px;opacity:.7"></span></div>
 <div id="taller-l"></div>
 <form id="taller-f" style="margin-top:6px"><input id="taller-t" placeholder="Háblales a las dos (Enter)" autocomplete="off"
  style="width:100%;box-sizing:border-box;background:rgba(255,255,255,.08);color:inherit;border:1px solid rgba(255,255,255,.15);border-radius:6px;padding:5px 7px"></form>
 <div style="font-size:11px;opacity:.6;margin-top:4px">Y: mostrar/ocultar</div>
</div>
<script>
(() => {
  const panel = document.getElementById("taller"), lista = document.getElementById("taller-l");
  const colores = {"Nyx": "#c9a6ff", "tú": "#9fe0b0", "taller": "#9aa"};
  let ultima = -1;
  async function tick() {
    try {
      const d = await (await fetch("/api/taller")).json();
      document.getElementById("taller-e").textContent = d.piezas + " piezas" + (d.viendo ? " · viendo «" + d.viendo + "»" : "") + (d.pausa ? " · en pausa" : "");
      const c = d.charla || [];
      if (c.length && c[c.length - 1].n !== ultima) {
        ultima = c[c.length - 1].n;
        lista.replaceChildren();
        for (const f of c) {
          const p = document.createElement("p");
          p.style.margin = "4px 0";
          const q = document.createElement("b");
          q.textContent = f.quien + ": ";
          q.style.color = colores[f.quien] || (f.quien.startsWith("Abla") ? "#7fe3d6" : "#ddd");
          p.append(q, document.createTextNode(f.texto));
          if (f.glosa) { const g = document.createElement("span"); g.textContent = "  «" + f.glosa + "»"; g.style.opacity = ".65"; p.append(g); }
          lista.append(p);
        }
        panel.scrollTop = panel.scrollHeight;
      }
    } catch (_) {}
  }
  setInterval(tick, 1500); tick();
  document.addEventListener("keydown", e => {
    const a = document.activeElement;
    if ((e.key === "y" || e.key === "Y") && !(a && (a.tagName === "INPUT" || a.tagName === "TEXTAREA")))
      panel.hidden = !panel.hidden;
  });
  const t = document.getElementById("taller-t");
  t.addEventListener("keydown", e => e.stopPropagation());
  t.addEventListener("focus", () => { try { document.exitPointerLock(); } catch (_) {} });
  document.getElementById("taller-f").addEventListener("submit", async e => {
    e.preventDefault();
    const v = t.value.trim(); if (!v) return;
    t.value = "";
    try { await fetch("/api/taller-di", {method: "POST", headers: {"X-Nyx-Codigo": CODIGO}, body: v}); } catch (_) {}
    setTimeout(tick, 800);
  });
})();
</script>
`
