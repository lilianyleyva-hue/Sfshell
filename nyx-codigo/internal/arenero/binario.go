package arenero

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"nyxcodigo/internal/nucleo"
)

// maxRelanzamientos bounds the relaunches of one Probar after crashes (§4.2.2).
const maxRelanzamientos = 20

// Bounds on what the trusted parent takes from fd 3, so candidate code cannot push this process out of memory
// by returning a huge value, or a batch of many cases with big values (stdout and stderr are capped elsewhere,
// Impreso at 4 KiB and the panic text at 2 KiB per case).
const (
	maxResultadoCaso    = 1 << 20  // encoded results of one case; the harness sends only the size of a larger one
	maxResultadosProbar = 32 << 20 // estimated memory of the decoded results kept by one Probar (pesoResultado)
	maxAuxiliarProbar   = 32 << 20 // printed output and crash texts kept by one Probar; past it they are cut short
	maxLineaArnes       = 4 << 20  // one line of fd 3: results, at most 1 MiB of events, small fields
	costeValor          = 32       // estimated memory of one decoded value besides its text (an interface value)
	cortoAuxiliar       = 256      // what is kept of a printed output or a crash text past maxAuxiliarProbar
)

// binario is a built harness (nucleo.Binario).
type binario struct {
	a       *Arenero
	dir     string
	prog    string
	firma   nucleo.Firma
	n       int
	vars    []*variante
	instr   nucleo.OpcionesInstr
	permiso nucleo.Permiso

	mu      sync.Mutex
	cerrado bool
}

var _ nucleo.Binario = (*binario)(nil)

func (b *binario) NumVariantes() int { return b.n }

// Lineas maps statement ids of variant v to lines of its original source (nil for a variant left out).
func (b *binario) Lineas(v int) []int {
	if v < 0 || v >= len(b.vars) || b.vars[v].fuera {
		return nil
	}
	return append([]int(nil), b.vars[v].lineas...)
}

// Cerrar removes the temp module. Probar fails afterwards.
func (b *binario) Cerrar() error {
	b.mu.Lock()
	ya := b.cerrado
	b.cerrado = true
	b.mu.Unlock()
	if !ya {
		b.a.borrarDir(b.dir)
	}
	return nil
}

// lineaArnes is one event read from fd 3.
type lineaArnes struct {
	N       string             `json:"n"`
	I       *[2]int            `json:"i"`
	Agotado bool               `json:"agotado"`
	Fin     bool               `json:"fin"`
	V       *int               `json:"v"`
	C       int                `json:"c"`
	O       []json.RawMessage  `json:"o"`
	P       string             `json:"p"`
	SC      bool               `json:"sc"`
	PF      string             `json:"pf"`
	ND      bool               `json:"nd"`
	ED      string             `json:"ed"`
	US      int64              `json:"us"`
	Grande  int                `json:"grande"`
	Cov     []int32            `json:"cov"`
	Ev      []nucleo.EventoVar `json:"ev"`
	Imp     string             `json:"imp"`
	A       *int               `json:"a"`     // start-up code of variant A begins
	Listo   bool               `json:"listo"` // every init has run
}

// Probar runs the cases on the chosen variants and returns [variante][caso]. It never rebuilds: after a crash
// it relaunches the same binary on the pairs still missing (at most 20 times).
func (b *binario) Probar(ctx context.Context, casos []nucleo.Caso, op nucleo.OpcionesProbar) ([][]nucleo.ResultadoCaso, error) {
	b.mu.Lock()
	cerrado := b.cerrado
	b.mu.Unlock()
	if cerrado {
		return nil, errors.New("arenero: el binario ya está cerrado")
	}
	lineas, err := codificarCasos(b.firma, casos)
	if err != nil {
		return nil, err
	}
	res := make([][]nucleo.ResultadoCaso, b.n)
	for v := range res {
		res[v] = make([]nucleo.ResultadoCaso, len(casos))
		for c := range res[v] {
			res[v][c] = nucleo.ResultadoCaso{Variante: v, Caso: c}
		}
	}
	pedidas := op.Variantes
	if pedidas == nil {
		for v := 0; v < b.n; v++ {
			pedidas = append(pedidas, v)
		}
	}
	var variantes []int
	vista := map[int]bool{}
	for _, v := range pedidas {
		if v < 0 || v >= b.n {
			return nil, fmt.Errorf("arenero: la variante %d no existe (hay %d)", v, b.n)
		}
		if vista[v] {
			continue
		}
		vista[v] = true
		if b.vars[v].fuera && b.vars[v].motivo == motivoArranque {
			// left out because its start-up code kills the process: that is what happens on every case
			for c := range res[v] {
				r := &res[v][c]
				r.Ejecutado, r.Caida, r.Panico = true, true, b.vars[v].caida
			}
			continue
		}
		if b.vars[v].fuera {
			motivo := "no compila"
			if b.vars[v].motivo == "inseguro" {
				motivo = "no pasa la revisión de seguridad"
			}
			for c := range res[v] {
				res[v][c].Panico = "esta variante " + motivo
			}
			continue
		}
		variantes = append(variantes, v)
	}
	if len(casos) == 0 || len(variantes) == 0 {
		return res, nil
	}
	tcaso := op.TiempoCaso
	if tcaso <= 0 {
		tcaso = b.a.cfg.TCaso
	}
	s := &sesionProbar{
		b: b, casos: casos, lineas: lineas, res: res, op: op, tcaso: tcaso,
		hecho: map[[2]int]bool{}, parado: map[int]bool{}, variantes: variantes,
	}
	for lanz := 0; lanz <= maxRelanzamientos; lanz++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pendientes := s.pendientes()
		if len(pendientes) == 0 {
			break
		}
		terminado, progreso, err := s.lanzar(ctx, pendientes)
		if err != nil {
			return nil, err
		}
		if terminado || progreso == 0 {
			break
		}
	}
	return res, nil
}

// codificarCasos encodes every case once: `{"e":[…],"r":[…]}`.
func codificarCasos(f nucleo.Firma, casos []nucleo.Caso) ([][]byte, error) {
	ent := f.Entradas()
	out := make([][]byte, len(casos))
	for i, c := range casos {
		if len(c.Entradas) != len(ent) {
			return nil, fmt.Errorf("arenero: el caso %d tiene %d entradas y %s necesita %d", i+1, len(c.Entradas), f.Nombre, len(ent))
		}
		var sb strings.Builder
		sb.WriteString(`{"e":[`)
		for j, t := range ent {
			m, err := nucleo.CodificarJSON(c.Entradas[j], t)
			if err != nil {
				return nil, fmt.Errorf("arenero: caso %d, entrada %d: %w", i+1, j+1, err)
			}
			if j > 0 {
				sb.WriteByte(',')
			}
			sb.Write(m)
		}
		sb.WriteByte(']')
		if c.ConEsperado() {
			if len(c.Esperado) != len(f.Res) {
				return nil, fmt.Errorf("arenero: el caso %d espera %d resultados y %s devuelve %d", i+1, len(c.Esperado), f.Nombre, len(f.Res))
			}
			sb.WriteString(`,"r":[`)
			for j, t := range f.Res {
				m, err := nucleo.CodificarJSON(c.Esperado[j], t)
				if err != nil {
					return nil, fmt.Errorf("arenero: caso %d, resultado esperado %d: %w", i+1, j+1, err)
				}
				if j > 0 {
					sb.WriteByte(',')
				}
				sb.Write(m)
			}
			sb.WriteByte(']')
		}
		sb.WriteString("}\n")
		out[i] = []byte(sb.String())
	}
	return out, nil
}

// sesionProbar is the state of one Probar across relaunches.
type sesionProbar struct {
	b         *binario
	casos     []nucleo.Caso
	lineas    [][]byte
	res       [][]nucleo.ResultadoCaso
	op        nucleo.OpcionesProbar
	tcaso     time.Duration
	retenido  int // estimated memory of the results kept so far (maxResultadosProbar)
	auxiliar  int // bytes of printed output and crash texts kept so far (maxAuxiliarProbar)
	variantes []int
	hecho     map[[2]int]bool // has a result, or was blamed for a crash
	parado    map[int]bool    // PararAlFallar: the variant already failed
}

func (s *sesionProbar) pendientes() []int {
	var out []int
	for _, v := range s.variantes {
		if s.parado[v] {
			continue
		}
		for c := range s.casos {
			if !s.hecho[[2]int{v, c}] {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

type cabecera struct {
	Nonce     string   `json:"nonce"`
	Fuel      int64    `json:"fuel"`
	MaxPila   int      `json:"maxPila"`
	TCasoMs   int64    `json:"tcaso_ms"`
	MaxO      int      `json:"maxo"`
	Repetir   int      `json:"repetir"`
	Parar     bool     `json:"parar"`
	Saltar    [][2]int `json:"saltar"`
	Variantes []int    `json:"variantes"`
}

// correrArnes runs a harness binary once: it writes the header and the case lines on fd 4 and hands to leer
// every line of fd 3 that carries the nonce (x == nil and larga: a line too long to read, not kept).
func (a *Arenero) correrArnes(ctx context.Context, dir, prog, nonce string, encabezado []byte, lineas [][]byte, leer func(x *lineaArnes, larga bool)) fin {
	malo := func(err error) fin {
		return fin{salida: nuevoLimitado(0), errSalida: nuevoLimitado(0), codigo: -1, err: fmt.Errorf("arenero: %w", err)}
	}
	r3, w3, err := os.Pipe()
	if err != nil {
		return malo(err)
	}
	r4, w4, err := os.Pipe()
	if err != nil {
		r3.Close()
		w3.Close()
		return malo(err)
	}
	o := orden{
		dir: dir, bin: prog, env: entornoEjecucion(dir, nonce),
		extra: []*os.File{w3, r4}, pared: a.cfg.TEjecutar, aislar: true,
	}
	mientras := func() {
		defer r3.Close()
		go func() {
			defer w4.Close()
			bw := bufio.NewWriterSize(w4, 1<<16)
			bw.Write(encabezado)
			bw.WriteByte('\n')
			for _, l := range lineas {
				if _, err := bw.Write(l); err != nil {
					return
				}
			}
			bw.Flush()
		}()
		lector := bufio.NewReaderSize(r3, 1<<16)
		for {
			l, larga, err := leerLinea(lector, maxLineaArnes)
			if larga {
				leer(nil, true)
			} else if len(l) > 0 {
				var x lineaArnes
				if json.Unmarshal(l, &x) == nil && x.N == nonce {
					leer(&x, false)
				}
			}
			if err != nil {
				return
			}
		}
	}
	return a.ejecutar(ctx, o, mientras)
}

// lanzar runs the binary once. terminado: the harness wrote "fin". progreso: new results or blamed pairs.
func (s *sesionProbar) lanzar(ctx context.Context, variantes []int) (terminado bool, progreso int, err error) {
	b := s.b
	if err := b.a.tomar(ctx); err != nil {
		return false, 0, err
	}
	defer b.a.soltar()
	nonce := aleatorio(16)
	cab := cabecera{
		Nonce: nonce, Fuel: b.instr.Combustible, MaxPila: b.instr.MaxPila, TCasoMs: s.tcaso.Milliseconds(), MaxO: maxResultadoCaso,
		Repetir: s.op.Repetir, Parar: s.op.PararAlFallar, Variantes: variantes, Saltar: [][2]int{},
	}
	for _, v := range variantes {
		for c := range s.casos {
			if s.hecho[[2]int{v, c}] {
				cab.Saltar = append(cab.Saltar, [2]int{v, c})
			}
		}
	}
	if cab.Fuel <= 0 {
		cab.Fuel = 1 << 62
	}
	encabezado, err := json.Marshal(cab)
	if err != nil {
		return false, 0, err
	}
	var marca *[2]int
	var tMarca time.Time
	arranque, listo := -1, false // last variant whose start-up code began; every init has run
	agotados := map[[2]int]bool{}
	f := b.a.correrArnes(ctx, b.dir, b.prog, nonce, encabezado, s.lineas, func(x *lineaArnes, larga bool) {
		if larga {
			if marca != nil && !s.hecho[*marca] {
				// the line of the case that was running is too big to read: drop it, blame the case
				s.fallar(*marca, "el resultado de este caso es demasiado grande (más de "+tamano(maxLineaArnes)+") y no lo leí")
				progreso++
			}
			return
		}
		switch {
		case x.Fin:
			terminado = true
		case x.Listo:
			listo = true
		case x.A != nil:
			arranque = *x.A
		case x.I != nil && x.Agotado:
			p := *x.I
			if s.valido(p) && !s.hecho[p] {
				agotados[p] = true
			}
		case x.I != nil:
			p := *x.I
			marca, tMarca = &p, time.Now()
		case x.V != nil:
			p := [2]int{*x.V, x.C}
			if s.valido(p) && !s.hecho[p] {
				s.anotar(p, x)
				progreso++
			}
		}
	})
	if f.err != nil && ctx.Err() != nil {
		return false, progreso, ctx.Err()
	}
	if terminado {
		return true, progreso, nil
	}
	if f.err != nil && marca == nil && progreso == 0 {
		return false, 0, f.err
	}
	// The process died before "fin". Blame the case that was running.
	culpa := func(p [2]int, agotado bool, texto string) {
		r := &s.res[p[0]][p[1]]
		r.Ejecutado, r.OK = true, false
		if agotado {
			r.Agotado = true
			r.Panico = fmt.Sprintf("tardó más de %v en este caso (¿un bucle sin fin?)", s.tcaso)
		} else {
			r.Caida = true
			r.Panico = texto
		}
		s.guardarAuxiliar(r)
		s.hecho[p] = true
		if s.op.PararAlFallar {
			s.parado[p[0]] = true
		}
		progreso++
	}
	for p := range agotados {
		if !s.hecho[p] {
			culpa(p, true, "")
		}
	}
	switch {
	case marca != nil && !s.hecho[*marca]:
		// a wall-clock kill only blames the case when that case itself ran for a long time
		if f.agotado && time.Since(tMarca) < s.tcaso {
			break
		}
		culpa(*marca, false, resumenCaida(f))
	case marca == nil && progreso == 0:
		// It died before the first case: the package-level code of a variant (a var initializer or an init
		// function) or the harness itself. Every variant's start-up code runs in every process, so only the
		// culprit is blamed first; if it dies again with nothing of its own left, nothing else can run.
		culpable := -1
		if !listo && arranque >= 0 && arranque < len(s.b.vars) {
			culpable = arranque
		}
		texto := resumenCaida(f)
		if culpable >= 0 {
			texto = sinSufijo(texto, s.b.vars[culpable].sufijo)
		}
		propios := 0
		for _, v := range variantes {
			if v != culpable {
				continue
			}
			for c := range s.casos {
				if p := [2]int{v, c}; !s.hecho[p] {
					culpa(p, false, texto)
					propios++
				}
			}
		}
		if propios > 0 {
			break
		}
		if culpable >= 0 {
			texto = fmt.Sprintf("la variante %d se cae al arrancar y no deja ejecutar ninguna otra: %s", culpable, texto)
		}
		for _, v := range variantes {
			for c := range s.casos {
				if p := [2]int{v, c}; !s.hecho[p] {
					culpa(p, false, texto)
				}
			}
		}
	}
	return false, progreso, nil
}

// fallar records a case that ran but whose result could not be taken.
func (s *sesionProbar) fallar(p [2]int, motivo string) {
	r := &s.res[p[0]][p[1]]
	r.Ejecutado, r.OK = true, false
	r.Panico = motivo
	s.guardarAuxiliar(r)
	s.hecho[p] = true
	if s.op.PararAlFallar {
		s.parado[p[0]] = true
	}
}

// sinSufijo removes a variant's renaming suffix from a text shown to the user.
func sinSufijo(texto, sufijo string) string {
	if sufijo == "" {
		return texto
	}
	return regexp.MustCompile(regexp.QuoteMeta(sufijo)+`([^0-9]|$)`).ReplaceAllString(texto, "$1")
}

func (s *sesionProbar) valido(p [2]int) bool {
	return p[0] >= 0 && p[0] < len(s.res) && p[1] >= 0 && p[1] < len(s.casos)
}

// anotar fills one result from a harness line; OK is recomputed here with nucleo.Igual.
func (s *sesionProbar) anotar(p [2]int, x *lineaArnes) {
	r := &s.res[p[0]][p[1]]
	f := s.b.firma
	r.Ejecutado = true
	r.Panico = x.P
	r.SinCombustible = x.SC
	r.PropFallida = x.PF
	r.NoDeterminista = x.ND
	r.Micros = x.US
	r.Cubiertas = x.Cov
	r.Eventos = x.Ev
	r.Impreso = x.Imp
	if x.ED != "" {
		r.Panico = "no pude leer la entrada: " + x.ED
	}
	ok := r.Panico == "" && !r.SinCombustible && r.PropFallida == "" && !r.NoDeterminista
	tam, peso := 0, 0
	for _, o := range x.O {
		tam += len(o)
		peso += pesoResultado(o)
	}
	switch {
	case r.Panico != "":
	case x.Grande > 0 || tam > maxResultadoCaso:
		r.Panico = "el resultado es demasiado grande (" + tamano(max(x.Grande, tam)) + "; el máximo es " + tamano(maxResultadoCaso) + "): no lo puedo comprobar"
		ok = false
	case s.retenido+peso > maxResultadosProbar:
		r.Panico = "los resultados de esta prueba ya ocupan demasiado en memoria (más de " + tamano(maxResultadosProbar) + "): no guardo este"
		ok = false
	}
	if r.Panico == "" && x.O != nil {
		s.retenido += peso
		if len(x.O) != len(f.Res) {
			r.Panico = "el arnés devolvió " + strconv.Itoa(len(x.O)) + " resultados"
			ok = false
		} else {
			r.Obtenido = make([]nucleo.Valor, len(f.Res))
			for i, t := range f.Res {
				v, err := nucleo.DecodificarJSON(x.O[i], t)
				if err != nil {
					r.Panico = "no pude leer el resultado: " + err.Error()
					r.Obtenido, ok = nil, false
					break
				}
				r.Obtenido[i] = v
			}
		}
	} else if r.Panico == "" {
		ok = false
	}
	c := s.casos[p[1]]
	if ok && c.ConEsperado() {
		for i := range c.Esperado {
			if i >= len(r.Obtenido) || !nucleo.Igual(r.Obtenido[i], c.Esperado[i]) {
				ok = false
				break
			}
		}
	}
	r.OK = ok
	s.guardarAuxiliar(r)
	s.hecho[p] = true
	if !ok && s.op.PararAlFallar {
		s.parado[p[0]] = true
	}
}

// pesoResultado estimates the memory one encoded result takes once decoded: its text plus costeValor per
// value, because every element of a list, map or struct becomes an interface value, so "[0,0,0,…]" weighs
// far more than its bytes. Commas inside quoted texts are counted too, which only overestimates.
func pesoResultado(o []byte) int {
	n := len(o) + costeValor
	for _, c := range o {
		if c == ',' || c == '[' {
			n += costeValor
		}
	}
	return n
}

// guardarAuxiliar keeps the printed output and the crash text of a case within the Probar's budget. Past it
// they are cut short and copied, so the big originals can be freed (300 variants × 300 cases printing 4 KiB
// each would otherwise keep 360 MiB). The verdict of the case does not change.
func (s *sesionProbar) guardarAuxiliar(r *nucleo.ResultadoCaso) {
	n := len(r.Impreso) + len(r.Panico)
	if s.auxiliar+n > maxAuxiliarProbar {
		r.Impreso = cortarTexto(r.Impreso, cortoAuxiliar)
		r.Panico = cortarTexto(primeraLinea(r.Panico), cortoAuxiliar)
		n = len(r.Impreso) + len(r.Panico)
	}
	s.auxiliar += n
}

// cortarTexto returns a copy of at most n bytes of s, cut at a character boundary, with "…" when cut.
func cortarTexto(s string, n int) string {
	if len(s) <= n {
		return strings.Clone(s)
	}
	i := n
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return strings.Clone(s[:i]) + "…"
}

// leerLinea reads one line of at most max bytes. A longer line is read to its end but not kept (larga).
func leerLinea(r *bufio.Reader, max int) (linea []byte, larga bool, err error) {
	for {
		trozo, e := r.ReadSlice('\n')
		if !larga {
			if len(linea)+len(trozo) > max {
				linea, larga = nil, true
			} else {
				linea = append(linea, trozo...)
			}
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		return linea, larga, e
	}
}

// tamano writes a byte count in simple words ("1 MiB", "60,0 MiB", "512 KiB").
func tamano(n int) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return strconv.Itoa(n>>20) + " MiB"
	case n >= 1<<20:
		return strings.Replace(strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64), ".", ",", 1) + " MiB"
	case n >= 1<<10:
		return strconv.Itoa(n>>10) + " KiB"
	}
	return strconv.Itoa(n) + " bytes"
}

// resumenCaida turns stderr into the Panico text of a crash. A runtime fatal error ("fatal error: stack
// overflow", "runtime: out of memory", a deadlock…) prints its reason first and then a dump of every
// goroutine, so the reason is looked for at the head of stderr; without one, the last 2 KiB are used.
func resumenCaida(f fin) string {
	cola := extraerCaida(f.errSalida.String())
	if cola == "" {
		cola = strings.TrimSpace(f.errSalida.Cola(2048))
	}
	var motivo string
	switch {
	case f.agotado:
		motivo = "el programa tardó demasiado y lo detuve"
	case f.senal != "":
		motivo = "el programa se cayó: " + explicarSenal(f.senal)
	case f.codigo == 2 || cola != "":
		motivo = fmt.Sprintf("el programa se cayó (código de salida %d)", f.codigo)
	default:
		motivo = fmt.Sprintf("el programa terminó antes de tiempo con el código de salida %d (¿llamó a os.Exit?)", f.codigo)
	}
	if cola == "" {
		return motivo
	}
	return cola + "\n(" + motivo + ")"
}

// maxCaida caps the crash text kept in Panico.
const maxCaida = 2048

// extraerCaida finds the first line that explains a crash ("fatal error:", "panic:", "runtime:",
// "SIGSEGV…") and returns it with what follows, cut at a line boundary within maxCaida bytes.
// It returns "" when stderr has no such line.
func extraerCaida(errSalida string) string {
	lineas := strings.Split(strings.ToValidUTF8(errSalida, ""), "\n")
	inicio := -1
	for i, l := range lineas {
		if esLineaCaida(l) {
			inicio = i
			break
		}
	}
	if inicio < 0 {
		return ""
	}
	var sb strings.Builder
	for _, l := range lineas[inicio:] {
		if sb.Len()+len(l)+1 > maxCaida {
			if sb.Len() == 0 {
				sb.WriteString(strings.TrimSuffix(cortarTexto(l, maxCaida), "…"))
			}
			break
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(l)
	}
	return strings.TrimSpace(sb.String())
}

func esLineaCaida(l string) bool {
	for _, p := range []string{"fatal error:", "panic:", "runtime:", "SIGSEGV", "SIGBUS", "SIGILL", "SIGFPE", "unexpected fault"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}
