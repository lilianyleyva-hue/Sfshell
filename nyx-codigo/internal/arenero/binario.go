package arenero

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"nyxcodigo/internal/nucleo"
)

// maxRelanzamientos bounds the relaunches of one Probar after crashes (§4.2.2).
const maxRelanzamientos = 20

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
	Cov     []int32            `json:"cov"`
	Ev      []nucleo.EventoVar `json:"ev"`
	Imp     string             `json:"imp"`
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
	Repetir   int      `json:"repetir"`
	Parar     bool     `json:"parar"`
	Saltar    [][2]int `json:"saltar"`
	Variantes []int    `json:"variantes"`
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
		Nonce: nonce, Fuel: b.instr.Combustible, MaxPila: b.instr.MaxPila, TCasoMs: s.tcaso.Milliseconds(),
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
	r3, w3, err := os.Pipe()
	if err != nil {
		return false, 0, fmt.Errorf("arenero: %w", err)
	}
	r4, w4, err := os.Pipe()
	if err != nil {
		r3.Close()
		w3.Close()
		return false, 0, fmt.Errorf("arenero: %w", err)
	}
	o := orden{
		dir: b.dir, bin: b.prog, env: entornoEjecucion(b.dir, nonce),
		extra: []*os.File{w3, r4}, pared: b.a.cfg.TEjecutar, aislar: true,
	}
	var marca *[2]int
	var tMarca time.Time
	agotados := map[[2]int]bool{}
	mientras := func() {
		defer r3.Close()
		go func() {
			defer w4.Close()
			bw := bufio.NewWriterSize(w4, 1<<16)
			bw.Write(encabezado)
			bw.WriteByte('\n')
			for _, l := range s.lineas {
				if _, err := bw.Write(l); err != nil {
					return
				}
			}
			bw.Flush()
		}()
		lector := bufio.NewReaderSize(r3, 1<<16)
		for {
			l, err := lector.ReadBytes('\n')
			if len(l) > 0 {
				var x lineaArnes
				if json.Unmarshal(l, &x) == nil && x.N == nonce {
					switch {
					case x.Fin:
						terminado = true
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
							s.anotar(p, &x)
							progreso++
						}
					}
				}
			}
			if err != nil {
				return
			}
		}
	}
	f := b.a.ejecutar(ctx, o, mientras)
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
	culpa := func(p [2]int, agotado bool) {
		r := &s.res[p[0]][p[1]]
		r.Ejecutado, r.OK = true, false
		if agotado {
			r.Agotado = true
			r.Panico = fmt.Sprintf("tardó más de %v en este caso (¿un bucle sin fin?)", s.tcaso)
		} else {
			r.Caida = true
			r.Panico = resumenCaida(f)
		}
		s.hecho[p] = true
		if s.op.PararAlFallar {
			s.parado[p[0]] = true
		}
		progreso++
	}
	for p := range agotados {
		if !s.hecho[p] {
			culpa(p, true)
		}
	}
	switch {
	case marca != nil && !s.hecho[*marca]:
		// a wall-clock kill only blames the case when that case itself ran for a long time
		if f.agotado && time.Since(tMarca) < s.tcaso {
			break
		}
		culpa(*marca, false)
	case marca == nil && progreso == 0:
		// it died before the first case (an init of the candidate code, or the harness itself)
		for _, v := range variantes {
			for c := range s.casos {
				if p := [2]int{v, c}; !s.hecho[p] {
					culpa(p, false)
				}
			}
		}
	}
	return false, progreso, nil
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
	if r.Panico == "" && x.O != nil {
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
	s.hecho[p] = true
	if !ok && s.op.PararAlFallar {
		s.parado[p[0]] = true
	}
}

// resumenCaida turns the end of stderr into the Panico text of a crash.
func resumenCaida(f fin) string {
	cola := strings.TrimSpace(f.errSalida.Cola(2048))
	if cola != "" {
		lineas := strings.Split(cola, "\n")
		for i, l := range lineas {
			if strings.HasPrefix(l, "fatal error:") || strings.HasPrefix(l, "panic:") || strings.HasPrefix(l, "runtime:") {
				cola = strings.Join(lineas[i:], "\n")
				break
			}
		}
	}
	var motivo string
	switch {
	case f.agotado:
		motivo = "el programa tardó demasiado y lo detuve"
	case f.senal != "":
		motivo = "el programa se cayó: " + explicarSenal(f.senal)
	default:
		motivo = fmt.Sprintf("el programa se cayó (código de salida %d)", f.codigo)
	}
	if cola == "" {
		return motivo
	}
	return cola + "\n(" + motivo + ")"
}
