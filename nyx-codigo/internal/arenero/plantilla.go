package arenero

// Trusted files of every temp module (§4.2.1). They are written by Nyx Código, never by a candidate,
// so they may import anything. Candidate files cannot reach them: imports are file-scoped and every
// name here contains "nyx__", which candidates may not use.

const fuenteMain = `// Código generado por Nyx Código. No lo edites.

package main

import "nyxprueba/solucion"

func main() { solucion.Nyx__Principal() }
`

// fuenteNyxfd opens the private pipes before any candidate init runs (package solucion imports it).
const fuenteNyxfd = `// Código generado por Nyx Código: el extremo de confianza de las tuberías del arnés. No lo edites.

package nyxfd

import (
	"bufio"
	"os"
	"syscall"
)

var (
	Nonce      string
	Resultados *os.File
	Casos      *bufio.Reader
)

func init() {
	Nonce = os.Getenv("NYX__NONCE")
	os.Unsetenv("NYX__NONCE")
	syscall.CloseOnExec(3)
	syscall.CloseOnExec(4)
	Resultados = os.NewFile(3, "nyx-resultados")
	Casos = bufio.NewReaderSize(os.NewFile(4, "nyx-casos"), 1<<16)
}
`

// plantillaArnes is the static part of zz_nyx_arnes.go. "IMPORTACIONES_EXTRA" and "TAMPILA" are replaced;
// the generated variant table, runners and property closures follow it.
const plantillaArnes = `// Código generado por Nyx Código (arnés de pruebas). No lo edites.

package solucion

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"nyxprueba/nyxfd"
IMPORTACIONES_EXTRA)

var (
	nyx__mu        sync.Mutex
	nyx__nonce     string
	nyx__actual    atomic.Int64
	nyx__salida    *os.File
	nyx__tipoError = reflect.TypeOf((*error)(nil)).Elem()
)

type nyx__Cabecera struct {
	Nonce     string
	Fuel      int64
	MaxPila   int
	Tcaso_ms  int64
	Repetir   int
	Parar     bool
	Saltar    [][2]int
	Variantes []int
}

type nyx__Linea struct {
	E []any
	R []any
}

// nyx__Ejec is one case of one variant.
type nyx__Ejec struct {
	E, R        []any
	ConR        bool
	Repetir     int
	Fuel        int64
	O           [][]byte
	Panico      string
	SinComb     bool
	Distinto    bool
	PropFallida string
	NoDet       bool
	Micros      int64
	Imp         string
	Cov         []int32
	Ev          []nyx__Evento
	ErrDec      string
}

func (x *nyx__Ejec) ok() bool {
	return x.ErrDec == "" && x.Panico == "" && !x.SinComb && !x.Distinto && x.PropFallida == "" && !x.NoDet
}

// Dec records a decoding error and reports whether there was one.
func (x *nyx__Ejec) Dec(err error) bool {
	if err != nil && x.ErrDec == "" {
		x.ErrDec = err.Error()
	}
	return err != nil
}

func nyx__texto(r any) (s string) {
	defer func() {
		if recover() != nil {
			s = "pánico con un valor que no se puede mostrar"
		}
	}()
	return fmt.Sprint(r)
}

func nyx__recortar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}

func nyx__panico(r any) (string, bool) {
	switch v := r.(type) {
	case nyx__SinComb:
		return v.Error(), true
	case nyx__Hondo:
		return v.Error(), false
	}
	return nyx__recortar(nyx__texto(r), 2048), false
}

// Llamar runs the variant once with fresh counters, catching panics, timing it, and collecting
// coverage, variable events and printed output.
func (x *nyx__Ejec) Llamar(f func()) (bien bool) {
	nyx__Reiniciar(x.Fuel)
	nyx__tomarSalida()
	os.Stdout = nyx__salida
	inicio := time.Now()
	defer func() {
		x.Micros = time.Since(inicio).Microseconds()
		if r := recover(); r != nil {
			x.Panico, x.SinComb = nyx__panico(r)
			bien = false
		}
		x.Cov = nyx__Cubiertas()
		x.Ev = nyx__Eventos()
		x.Imp = nyx__tomarSalida()
	}()
	f()
	return true
}

// Prop evaluates one property (cumple, saltada); a panic counts as a failure.
func (x *nyx__Ejec) Prop(nombre string, f func() (bool, bool)) {
	if x.PropFallida != "" {
		return
	}
	nyx__Reiniciar(x.Fuel)
	os.Stdout = nyx__salida
	cumple := false
	func() {
		defer func() {
			if recover() != nil {
				cumple = false
			}
		}()
		c, saltada := f()
		cumple = c || saltada
	}()
	if !cumple {
		x.PropFallida = nombre
	}
}

// Repetida runs the variant again on fresh inputs and compares the encoded results.
func (x *nyx__Ejec) Repetida(f func() [][]byte) {
	if x.NoDet {
		return
	}
	nyx__Reiniciar(x.Fuel)
	os.Stdout = nyx__salida
	var o [][]byte
	mal := false
	func() {
		defer func() {
			if recover() != nil {
				mal = true
			}
		}()
		o = f()
	}()
	if mal || len(o) != len(x.O) {
		x.NoDet = true
		return
	}
	for i := range o {
		if string(o[i]) != string(x.O[i]) {
			x.NoDet = true
			return
		}
	}
}

func nyx__escribir(b []byte) {
	nyx__mu.Lock()
	defer nyx__mu.Unlock()
	if _, err := nyxfd.Resultados.Write(b); err != nil {
		os.Exit(5)
	}
}

func nyx__morir(msg string) {
	fmt.Fprintln(os.Stderr, "arnés de Nyx Código: "+msg)
	os.Exit(4)
}

func nyx__agotado() {
	a := nyx__actual.Load()
	b := []byte("{\"n\":\"" + nyx__nonce + "\",\"i\":[" + strconv.FormatInt(a>>32, 10) + "," + strconv.FormatInt(a&0xffffffff, 10) + "],\"agotado\":true}\n")
	nyx__escribir(b)
	os.Exit(3)
}

// nyx__abrirSalida creates the per-case capture of os.Stdout: an unlinked temp file (writes never block).
func nyx__abrirSalida() {
	f, err := os.CreateTemp("", "nyx-salida-")
	if err == nil {
		os.Remove(f.Name())
	} else {
		f, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	nyx__salida = f
}

// nyx__tomarSalida returns what was printed since the last call (at most 4 KiB) and empties the capture.
func nyx__tomarSalida() string {
	f := nyx__salida
	if f == nil {
		return ""
	}
	n, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		nyx__abrirSalida()
		return ""
	}
	if n <= 0 {
		return ""
	}
	if n > 4096 {
		n = 4096
	}
	b := make([]byte, n)
	m, _ := f.ReadAt(b, 0)
	f.Truncate(0)
	f.Seek(0, io.SeekStart)
	return string(b[:m])
}

func nyx__marca(b []byte, v, c int) []byte {
	b = append(b, "{\"n\":\""...)
	b = append(b, nyx__nonce...)
	b = append(b, "\",\"i\":["...)
	b = strconv.AppendInt(b, int64(v), 10)
	b = append(b, ',')
	b = strconv.AppendInt(b, int64(c), 10)
	return append(b, "]}\n"...)
}

func nyx__resultado(b []byte, v, c int, x *nyx__Ejec) []byte {
	b = append(b, "{\"n\":\""...)
	b = append(b, nyx__nonce...)
	b = append(b, "\",\"v\":"...)
	b = strconv.AppendInt(b, int64(v), 10)
	b = append(b, ",\"c\":"...)
	b = strconv.AppendInt(b, int64(c), 10)
	b = append(b, ",\"ok\":"...)
	b = strconv.AppendBool(b, x.ok())
	if x.Panico == "" && x.ErrDec == "" && x.O != nil {
		b = append(b, ",\"o\":["...)
		for i, o := range x.O {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, o...)
		}
		b = append(b, ']')
	}
	if x.Panico != "" {
		b = append(b, ",\"p\":"...)
		b = nyx__cadena(b, x.Panico)
	}
	if x.SinComb {
		b = append(b, ",\"sc\":true"...)
	}
	if x.Distinto {
		b = append(b, ",\"df\":true"...)
	}
	if x.PropFallida != "" {
		b = append(b, ",\"pf\":"...)
		b = nyx__cadena(b, x.PropFallida)
	}
	if x.NoDet {
		b = append(b, ",\"nd\":true"...)
	}
	if x.ErrDec != "" {
		b = append(b, ",\"ed\":"...)
		b = nyx__cadena(b, x.ErrDec)
	}
	b = append(b, ",\"us\":"...)
	b = strconv.AppendInt(b, x.Micros, 10)
	if len(x.Cov) > 0 {
		b = append(b, ",\"cov\":["...)
		for i, id := range x.Cov {
			if i > 0 {
				b = append(b, ',')
			}
			b = strconv.AppendInt(b, int64(id), 10)
		}
		b = append(b, ']')
	}
	if len(x.Ev) > 0 {
		b = append(b, ",\"ev\":["...)
		for i, e := range x.Ev {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, "{\"linea\":"...)
			b = strconv.AppendInt(b, int64(e.Linea), 10)
			b = append(b, ",\"var\":"...)
			b = nyx__cadena(b, e.Var)
			b = append(b, ",\"valor\":"...)
			b = nyx__cadena(b, e.Valor)
			b = append(b, '}')
		}
		b = append(b, ']')
	}
	if x.Imp != "" {
		b = append(b, ",\"imp\":"...)
		b = nyx__cadena(b, x.Imp)
	}
	return append(b, "}\n"...)
}

// Nyx__Principal reads the header and the cases from fd 4, runs them and writes the results on fd 3.
func Nyx__Principal() {
	debug.SetMaxStack(TAMPILA)
	nyx__nonce = nyxfd.Nonce
	dec := json.NewDecoder(nyxfd.Casos)
	dec.UseNumber()
	var cab nyx__Cabecera
	if err := dec.Decode(&cab); err != nil {
		nyx__morir("no pude leer la cabecera: " + err.Error())
	}
	if cab.Nonce == "" || cab.Nonce != nyx__nonce {
		nyx__morir("la cabecera no trae la clave de esta ejecución")
	}
	var casos []nyx__Linea
	for {
		var l nyx__Linea
		err := dec.Decode(&l)
		if err == io.EOF {
			break
		}
		if err != nil {
			nyx__morir("no pude leer un caso: " + err.Error())
		}
		casos = append(casos, l)
	}
	if cab.MaxPila > 0 {
		nyx__maxPila = cab.MaxPila
	}
	if cab.Fuel <= 0 {
		cab.Fuel = 1 << 62
	}
	tcaso := time.Duration(cab.Tcaso_ms) * time.Millisecond
	if tcaso <= 0 {
		tcaso = 2 * time.Second
	}
	saltar := make(map[[2]int]bool, len(cab.Saltar))
	for _, p := range cab.Saltar {
		saltar[p] = true
	}
	nyx__abrirSalida()
	reloj := time.AfterFunc(time.Hour, nyx__agotado)
	reloj.Stop()
	var pendiente []byte
	for _, v := range cab.Variantes {
		if v < 0 || v >= len(nyx__tabla) || nyx__tabla[v] == nil {
			continue
		}
		for c := range casos {
			if saltar[[2]int{v, c}] {
				continue
			}
			pendiente = nyx__marca(pendiente, v, c)
			nyx__escribir(pendiente)
			pendiente = pendiente[:0]
			nyx__actual.Store(int64(v)<<32 | int64(c))
			x := &nyx__Ejec{E: casos[c].E, R: casos[c].R, ConR: casos[c].R != nil, Repetir: cab.Repetir, Fuel: cab.Fuel}
			reloj.Reset(tcaso)
			nyx__tabla[v](x)
			reloj.Stop()
			nyx__tomarSalida()
			pendiente = nyx__resultado(pendiente, v, c, x)
			if cab.Parar && !x.ok() {
				break
			}
		}
	}
	pendiente = append(pendiente, "{\"n\":\""+nyx__nonce+"\",\"fin\":true}\n"...)
	nyx__escribir(pendiente)
	os.Exit(0)
}

// ---- codec helpers (the §3.3 wire format) ----

func nyx__malTipo(x any, que string) error {
	return fmt.Errorf("se esperaba %s y llegó %s", que, nyx__recortar(nyx__texto(x), 60))
}

func nyx__decEntero(x any) (int64, error) {
	n, ok := x.(json.Number)
	if !ok {
		return 0, nyx__malTipo(x, "un número entero")
	}
	return strconv.ParseInt(string(n), 10, 64)
}

func nyx__decNatural(x any) (uint64, error) {
	n, ok := x.(json.Number)
	if !ok {
		return 0, nyx__malTipo(x, "un número natural")
	}
	return strconv.ParseUint(string(n), 10, 64)
}

func nyx__decDecimal(x any) (float64, error) {
	switch y := x.(type) {
	case json.Number:
		return strconv.ParseFloat(string(y), 64)
	case string:
		switch y {
		case "NaN":
			return math.NaN(), nil
		case "+Inf", "Inf":
			return math.Inf(1), nil
		case "-Inf":
			return math.Inf(-1), nil
		}
	}
	return 0, nyx__malTipo(x, "un número decimal")
}

func nyx__decLogico(x any) (bool, error) {
	b, ok := x.(bool)
	if !ok {
		return false, nyx__malTipo(x, "true o false")
	}
	return b, nil
}

func nyx__decTexto(x any) (string, error) {
	switch y := x.(type) {
	case string:
		return y, nil
	case map[string]any:
		if s, ok := y["b64"].(string); ok && len(y) == 1 {
			b, err := base64.StdEncoding.DecodeString(s)
			return string(b), err
		}
	}
	return "", nyx__malTipo(x, "un texto")
}

func nyx__decError(x any) (error, error) {
	switch y := x.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		if s, ok := y["error"].(string); ok {
			return errors.New(s), nil
		}
	}
	return nil, nyx__malTipo(x, "un error")
}

// nyx__decLista returns the elements, whether the value was null, and an error.
func nyx__decLista(x any) ([]any, bool, error) {
	if x == nil {
		return nil, true, nil
	}
	xs, ok := x.([]any)
	if !ok {
		return nil, false, nyx__malTipo(x, "una lista")
	}
	return xs, false, nil
}

func nyx__decPar(x any) (any, any, error) {
	p, ok := x.([]any)
	if !ok || len(p) != 2 {
		return nil, nil, nyx__malTipo(x, "un par [clave, valor]")
	}
	return p[0], p[1], nil
}

func nyx__largo(xs []any, n int) error {
	if len(xs) != n {
		return fmt.Errorf("hay %d elementos y deben ser %d", len(xs), n)
	}
	return nil
}

func nyx__cadena(b []byte, s string) []byte {
	s = strings.ToValidUTF8(s, "�")
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for _, r := range s {
		switch {
		case r == '"':
			b = append(b, '\\', '"')
		case r == '\\':
			b = append(b, '\\', '\\')
		case r == '\n':
			b = append(b, '\\', 'n')
		case r == '\r':
			b = append(b, '\\', 'r')
		case r == '\t':
			b = append(b, '\\', 't')
		case r < 0x20:
			b = append(b, '\\', 'u', '0', '0', hex[r>>4], hex[r&0xF])
		case r == 0x2028:
			b = append(b, "\\u2028"...)
		case r == 0x2029:
			b = append(b, "\\u2029"...)
		default:
			b = utf8.AppendRune(b, r)
		}
	}
	return append(b, '"')
}

func nyx__encTexto(b []byte, s string) []byte {
	if utf8.ValidString(s) {
		return nyx__cadena(b, s)
	}
	b = append(b, "{\"b64\":\""...)
	b = append(b, base64.StdEncoding.EncodeToString([]byte(s))...)
	return append(b, "\"}"...)
}

func nyx__encDecimal(b []byte, f float64, bits int) []byte {
	switch {
	case math.IsNaN(f):
		return append(b, "\"NaN\""...)
	case math.IsInf(f, 1):
		return append(b, "\"+Inf\""...)
	case math.IsInf(f, -1):
		return append(b, "\"-Inf\""...)
	}
	return strconv.AppendFloat(b, f, 'g', -1, bits)
}

func nyx__encError(b []byte, e error) []byte {
	if e == nil {
		return append(b, "null"...)
	}
	msg := func() (s string) {
		defer func() {
			if recover() != nil {
				s = "error"
			}
		}()
		return e.Error()
	}()
	b = append(b, "{\"error\":"...)
	b = nyx__cadena(b, msg)
	return append(b, '}')
}

// ---- property helpers (same semantics as nucleo.Igual / Comparar) ----

func nyx__Igual(a, b any) bool { return nyx__igual(reflect.ValueOf(a), reflect.ValueOf(b), 0) }

func nyx__desenvolver(v reflect.Value) reflect.Value {
	for v.IsValid() && v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func nyx__vacio(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}

func nyx__esEntero(k reflect.Kind) bool {
	return k >= reflect.Int && k <= reflect.Uintptr
}

func nyx__esDecimal(k reflect.Kind) bool { return k == reflect.Float32 || k == reflect.Float64 }

func nyx__comoDecimal(v reflect.Value) float64 {
	switch {
	case nyx__esDecimal(v.Kind()):
		return v.Float()
	case v.Kind() >= reflect.Int && v.Kind() <= reflect.Int64:
		return float64(v.Int())
	default:
		return float64(v.Uint())
	}
}

func nyx__decimalesIguales(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	if a == b {
		return true
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return false
	}
	escala := math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	return math.Abs(a-b) <= 1e-9*escala
}

func nyx__igual(a, b reflect.Value, prof int) bool {
	if prof > 200 {
		return false
	}
	a, b = nyx__desenvolver(a), nyx__desenvolver(b)
	if nyx__vacio(a) || nyx__vacio(b) {
		return nyx__vacio(a) && nyx__vacio(b)
	}
	if a.Type().Implements(nyx__tipoError) && b.Type().Implements(nyx__tipoError) {
		return true // two non-nil errors are equal whatever the message
	}
	ka, kb := a.Kind(), b.Kind()
	if (nyx__esEntero(ka) || nyx__esDecimal(ka)) && (nyx__esEntero(kb) || nyx__esDecimal(kb)) {
		if nyx__esDecimal(ka) || nyx__esDecimal(kb) {
			return nyx__decimalesIguales(nyx__comoDecimal(a), nyx__comoDecimal(b))
		}
		sa := ka >= reflect.Int && ka <= reflect.Int64
		sb := kb >= reflect.Int && kb <= reflect.Int64
		switch {
		case sa && sb:
			return a.Int() == b.Int()
		case !sa && !sb:
			return a.Uint() == b.Uint()
		case sa:
			return a.Int() >= 0 && uint64(a.Int()) == b.Uint()
		default:
			return b.Int() >= 0 && uint64(b.Int()) == a.Uint()
		}
	}
	switch ka {
	case reflect.Bool:
		return kb == reflect.Bool && a.Bool() == b.Bool()
	case reflect.String:
		return kb == reflect.String && a.String() == b.String()
	case reflect.Complex64, reflect.Complex128:
		return (kb == reflect.Complex64 || kb == reflect.Complex128) && a.Complex() == b.Complex()
	case reflect.Slice, reflect.Array:
		if kb != reflect.Slice && kb != reflect.Array {
			return false
		}
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !nyx__igual(a.Index(i), b.Index(i), prof+1) {
				return false
			}
		}
		return true
	case reflect.Map:
		if kb != reflect.Map || a.Len() != b.Len() {
			return false
		}
		if a.Type().Key() == b.Type().Key() {
			it := a.MapRange()
			for it.Next() {
				w := b.MapIndex(it.Key())
				if !w.IsValid() || !nyx__igual(it.Value(), w, prof+1) {
					return false
				}
			}
			return true
		}
		usados := make([]bool, b.Len())
		claves := b.MapKeys()
		it := a.MapRange()
		for it.Next() {
			hallado := false
			for j, k := range claves {
				if !usados[j] && nyx__igual(it.Key(), k, prof+1) && nyx__igual(it.Value(), b.MapIndex(k), prof+1) {
					usados[j], hallado = true, true
					break
				}
			}
			if !hallado {
				return false
			}
		}
		return true
	case reflect.Struct:
		if kb != reflect.Struct || a.NumField() != b.NumField() {
			return false
		}
		for i := 0; i < a.NumField(); i++ {
			if !nyx__igual(a.Field(i), b.Field(i), prof+1) {
				return false
			}
		}
		return true
	case reflect.Pointer:
		if kb != reflect.Pointer {
			return false
		}
		if a.Pointer() == b.Pointer() {
			return true
		}
		return nyx__igual(a.Elem(), b.Elem(), prof+1)
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return ka == kb && a.Pointer() == b.Pointer()
	}
	return false
}

// nyx__comparar orders numbers, strings and booleans; anything else compares equal (0) and ok=false.
func nyx__comparar(a, b reflect.Value) (int, bool) {
	a, b = nyx__desenvolver(a), nyx__desenvolver(b)
	if !a.IsValid() || !b.IsValid() {
		return 0, false
	}
	ka, kb := a.Kind(), b.Kind()
	if (nyx__esEntero(ka) || nyx__esDecimal(ka)) && (nyx__esEntero(kb) || nyx__esDecimal(kb)) {
		if nyx__igual(a, b, 0) {
			return 0, true
		}
		x, y := nyx__comoDecimal(a), nyx__comoDecimal(b)
		if !nyx__esDecimal(ka) && !nyx__esDecimal(kb) && (ka >= reflect.Int && ka <= reflect.Int64) == (kb >= reflect.Int && kb <= reflect.Int64) {
			if ka >= reflect.Int && ka <= reflect.Int64 {
				if a.Int() < b.Int() {
					return -1, true
				}
				return 1, true
			}
			if a.Uint() < b.Uint() {
				return -1, true
			}
			return 1, true
		}
		if math.IsNaN(x) {
			return -1, true
		}
		if math.IsNaN(y) || x > y {
			return 1, true
		}
		return -1, true
	}
	if ka == reflect.String && kb == reflect.String {
		return strings.Compare(a.String(), b.String()), true
	}
	if ka == reflect.Bool && kb == reflect.Bool {
		switch {
		case a.Bool() == b.Bool():
			return 0, true
		case !a.Bool():
			return -1, true
		}
		return 1, true
	}
	return 0, false
}

// nyx__elementos returns the elements of a slice, an array or a string (as runes).
func nyx__elementos(x any) ([]reflect.Value, bool) {
	v := nyx__desenvolver(reflect.ValueOf(x))
	if !v.IsValid() {
		return nil, true
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]reflect.Value, v.Len())
		for i := range out {
			out[i] = v.Index(i)
		}
		return out, true
	case reflect.String:
		var out []reflect.Value
		for _, r := range v.String() {
			out = append(out, reflect.ValueOf(r))
		}
		return out, true
	}
	return nil, false
}

// nyx__Ordenada: every element is ≥ the previous one.
func nyx__Ordenada(xs any) bool {
	es, ok := nyx__elementos(xs)
	if !ok {
		return false
	}
	for i := 1; i < len(es); i++ {
		if c, ok := nyx__comparar(es[i-1], es[i]); !ok || c > 0 {
			return false
		}
	}
	return true
}

// nyx__OrdenadaDesc: every element is ≤ the previous one.
func nyx__OrdenadaDesc(xs any) bool {
	es, ok := nyx__elementos(xs)
	if !ok {
		return false
	}
	for i := 1; i < len(es); i++ {
		if c, ok := nyx__comparar(es[i-1], es[i]); !ok || c < 0 {
			return false
		}
	}
	return true
}

// nyx__EsPermutacion: a and b have the same elements, in any order.
func nyx__EsPermutacion(a, b any) bool {
	ea, oka := nyx__elementos(a)
	eb, okb := nyx__elementos(b)
	if !oka || !okb || len(ea) != len(eb) {
		return false
	}
	ordenables := true
	for _, e := range append(append([]reflect.Value(nil), ea...), eb...) {
		if _, ok := nyx__comparar(e, e); !ok {
			ordenables = false
			break
		}
	}
	if ordenables {
		menor := func(xs []reflect.Value) func(i, j int) bool {
			return func(i, j int) bool { c, _ := nyx__comparar(xs[i], xs[j]); return c < 0 }
		}
		sort.SliceStable(ea, menor(ea))
		sort.SliceStable(eb, menor(eb))
		for i := range ea {
			if !nyx__igual(ea[i], eb[i], 0) {
				return false
			}
		}
		return true
	}
	usados := make([]bool, len(eb))
	for _, x := range ea {
		hallado := false
		for j, y := range eb {
			if !usados[j] && nyx__igual(x, y, 0) {
				usados[j], hallado = true, true
				break
			}
		}
		if !hallado {
			return false
		}
	}
	return true
}

// nyx__Subsecuencia: every element of r appears in e, in the same order.
func nyx__Subsecuencia(r, e any) bool {
	er, okr := nyx__elementos(r)
	ee, oke := nyx__elementos(e)
	if !okr || !oke {
		return false
	}
	j := 0
	for _, x := range er {
		for j < len(ee) && !nyx__igual(x, ee[j], 0) {
			j++
		}
		if j == len(ee) {
			return false
		}
		j++
	}
	return true
}

// nyx__Contiene: x is one of the elements of xs (for a text, also a piece of it).
func nyx__Contiene(xs, x any) bool {
	if s, ok := xs.(string); ok {
		if t, ok := x.(string); ok {
			return strings.Contains(s, t)
		}
	}
	es, ok := nyx__elementos(xs)
	if !ok {
		return false
	}
	for _, e := range es {
		if nyx__igual(e, reflect.ValueOf(x), 0) {
			return true
		}
	}
	return false
}

// nyx__SinRepetidos: no two elements are equal.
func nyx__SinRepetidos(xs any) bool {
	es, ok := nyx__elementos(xs)
	if !ok {
		return false
	}
	for i := range es {
		for j := i + 1; j < len(es); j++ {
			if nyx__igual(es[i], es[j], 0) {
				return false
			}
		}
	}
	return true
}

// nyx__Invertir returns a reversed copy of a slice, an array or a string (by runes).
func nyx__Invertir[T any](x T) T {
	v := reflect.ValueOf(&x).Elem()
	switch v.Kind() {
	case reflect.String:
		rs := []rune(v.String())
		for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
			rs[i], rs[j] = rs[j], rs[i]
		}
		v.SetString(string(rs))
	case reflect.Slice:
		if v.IsNil() {
			return x
		}
		n := v.Len()
		c := reflect.MakeSlice(v.Type(), n, n)
		for i := 0; i < n; i++ {
			c.Index(i).Set(v.Index(n - 1 - i))
		}
		v.Set(c)
	case reflect.Array:
		n := v.Len()
		c := reflect.New(v.Type()).Elem()
		for i := 0; i < n; i++ {
			c.Index(i).Set(v.Index(n - 1 - i))
		}
		v.Set(c)
	}
	return x
}

var (
	_ = nyx__Igual
	_ = nyx__Ordenada
	_ = nyx__OrdenadaDesc
	_ = nyx__EsPermutacion
	_ = nyx__Subsecuencia
	_ = nyx__Contiene
	_ = nyx__SinRepetidos
	_ = nyx__Invertir[int]
	_ = nyx__decEntero
	_ = nyx__decNatural
	_ = nyx__decDecimal
	_ = nyx__decLogico
	_ = nyx__decTexto
	_ = nyx__decError
	_ = nyx__decLista
	_ = nyx__decPar
	_ = nyx__largo
	_ = nyx__encTexto
	_ = nyx__encDecimal
	_ = nyx__encError
)

// ---- generated below: the variant table, one runner per variant, the property closures ----
`
