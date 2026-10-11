package instrumenta

import (
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// MaxEventosDefecto is used when OpcionesInstr.MaxEventos is 0.
const MaxEventosDefecto = 200

// MaxRunasValor caps the text of one traced value.
const MaxRunasValor = 60

// MaxBytesNombre caps the variable name of one traced event (an identifier can be as long as the file).
const MaxBytesNombre = 64

// Messages of the panic types (Spanish; also used by arenero to recognise them in a crashed program).
const (
	MensajeSinComb = "se acabó el combustible: demasiados pasos (¿un bucle sin fin?)"
	MensajeHondo   = "demasiadas llamadas anidadas (¿una recursión sin fin?)"
)

// Soporte returns the support file for package `paquete`: counters (plain int64; auto-run code has no goroutines),
// coverage array [n]uint32, event buffer, the panic types, and nyx__Reiniciar(fuel int64) used by the harness per case.
//
// Exported to the rest of the package (all names start with Prefijo):
//
//	nyx__SinComb, nyx__Hondo         panic types (they implement error)
//	nyx__f, nyx__p, nyx__maxPila     fuel left, current depth, depth limit
//	nyx__c                           coverage counters, one per statement id
//	nyx__ev(linea, nombre, valor)    records one variable event: the value is written like fmt's %v but
//	                                 structurally, so no String/Error method of the program runs while
//	                                 tracing (standard-library types still use theirs); tracing is not
//	                                 re-entrant and spends no fuel, depth or coverage
//	nyx__Reiniciar(fuel)             resets fuel, depth, coverage and events (once per case)
//	nyx__Cubiertas() []int32         ids of the statements executed since the last reset
//	nyx__Eventos() []nyx__Evento     events recorded since the last reset
//
// When op.Combustible is 0 the fuel counter starts very high, so code that was not instrumented
// for fuel never sees it run out.
func Soporte(paquete string, op nucleo.OpcionesInstr, nSentencias int) string {
	if nSentencias < 0 {
		nSentencias = 0
	}
	combustible := op.Combustible
	if combustible <= 0 {
		combustible = 1 << 62
	}
	maxPila := op.MaxPila
	if maxPila <= 0 {
		maxPila = 1 << 30
	}
	maxEventos := op.MaxEventos
	if maxEventos <= 0 {
		maxEventos = MaxEventosDefecto
	}
	r := strings.NewReplacer(
		"PAQUETE", paquete,
		"COMBUSTIBLE", strconv.FormatInt(combustible, 10),
		"MAXPILA", strconv.Itoa(maxPila),
		"NSENTENCIAS", strconv.Itoa(nSentencias),
		"MAXEVENTOS", strconv.Itoa(maxEventos),
		"MAXRUNAS", strconv.Itoa(MaxRunasValor),
		"MAXNOMBRE", strconv.Itoa(MaxBytesNombre),
		"MSGSINCOMB", strconv.Quote(MensajeSinComb),
		"MSGHONDO", strconv.Quote(MensajeHondo),
	)
	return r.Replace(plantillaSoporte)
}

const plantillaSoporte = `// Código generado por Nyx Código (soporte de instrumentación). No lo edites.

package PAQUETE

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type nyx__SinComb struct{}

func (nyx__SinComb) Error() string { return MSGSINCOMB }

type nyx__Hondo struct{}

func (nyx__Hondo) Error() string { return MSGHONDO }

type nyx__Evento struct {
	Linea int
	Var   string
	Valor string
}

var (
	nyx__f          int64 = COMBUSTIBLE
	nyx__p          int
	nyx__maxPila    int = MAXPILA
	nyx__c          [NSENTENCIAS]uint32
	nyx__evs        []nyx__Evento
	nyx__maxEventos int = MAXEVENTOS
	nyx__enEv       bool
)

// nyx__ev records one variable event. The value is written without calling any method of the program's own
// types (no String or Error of the candidate code runs while tracing), and a call made while another event is
// being formatted is ignored, so tracing never recurses and never changes fuel, depth or coverage.
func nyx__ev(linea int, nombre string, v any) {
	if nyx__enEv || len(nyx__evs) >= nyx__maxEventos {
		return
	}
	nyx__enEv = true
	s := nyx__mostrar(v)
	nyx__enEv = false
	if len(nombre) > MAXNOMBRE {
		i := MAXNOMBRE
		for i > 0 && !utf8.RuneStart(nombre[i]) {
			i--
		}
		nombre = nombre[:i] + "…"
	}
	if utf8.RuneCountInString(s) > MAXRUNAS {
		n := 0
		for i := range s {
			if n == MAXRUNAS-1 {
				s = s[:i] + "…"
				break
			}
			n++
		}
	}
	nyx__evs = append(nyx__evs, nyx__Evento{Linea: linea, Var: nombre, Valor: s})
}

// nyx__mostrar formats v like fmt's %v, but structurally.
func nyx__mostrar(v any) (s string) {
	f, p := nyx__f, nyx__p
	defer func() {
		nyx__f, nyx__p = f, p
		if recover() != nil {
			s = "?"
		}
	}()
	var sb strings.Builder
	nyx__formato(&sb, reflect.ValueOf(v), 0)
	return sb.String()
}

// nyx__lleno: enough text for one event (it is cut to MAXRUNAS runes anyway).
func nyx__lleno(sb *strings.Builder) bool { return sb.Len() > 4*MAXRUNAS+8 }

var (
	nyx__evTipoError    = reflect.TypeOf((*error)(nil)).Elem()
	nyx__evTipoStringer = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
)

// nyx__ajeno reports a named type of another package (the standard library): its String or Error may run.
func nyx__ajeno(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	pkg := t.PkgPath()
	if t.Name() == "" || pkg == "" || pkg == reflect.TypeOf(nyx__Evento{}).PkgPath() {
		return false
	}
	return !strings.Contains(strings.SplitN(pkg, "/", 2)[0], ".")
}

// nyx__metodo calls Error or String of a standard-library value. Whatever that runs (for example errors.Join
// calling a program's own Error) leaves no trace: coverage is restored and events are already blocked.
func nyx__metodo(v reflect.Value) (string, bool) {
	t := v.Type()
	if !v.CanInterface() || !nyx__ajeno(t) || !(t.Implements(nyx__evTipoError) || t.Implements(nyx__evTipoStringer)) {
		return "", false
	}
	if t.Kind() == reflect.Pointer && v.IsNil() {
		return "<nil>", true
	}
	copia := nyx__c
	defer func() { nyx__c = copia }()
	switch x := v.Interface().(type) {
	case error:
		return x.Error(), true
	case fmt.Stringer:
		return x.String(), true
	}
	return "", false
}

func nyx__formato(sb *strings.Builder, v reflect.Value, prof int) {
	if !v.IsValid() {
		sb.WriteString("<nil>")
		return
	}
	if prof > 6 || nyx__lleno(sb) {
		sb.WriteString("…")
		return
	}
	if s, ok := nyx__metodo(v); ok {
		sb.WriteString(s)
		return
	}
	switch v.Kind() {
	case reflect.Bool:
		sb.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sb.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		sb.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32:
		sb.WriteString(strconv.FormatFloat(v.Float(), 'g', -1, 32))
	case reflect.Float64:
		sb.WriteString(strconv.FormatFloat(v.Float(), 'g', -1, 64))
	case reflect.Complex64:
		sb.WriteString(strconv.FormatComplex(v.Complex(), 'g', -1, 64))
	case reflect.Complex128:
		sb.WriteString(strconv.FormatComplex(v.Complex(), 'g', -1, 128))
	case reflect.String:
		s := v.String()
		if len(s) > 4*MAXRUNAS+8 {
			s = s[:4*MAXRUNAS+8]
		}
		sb.WriteString(s)
	case reflect.Interface:
		if v.IsNil() {
			sb.WriteString("<nil>")
			return
		}
		nyx__formato(sb, v.Elem(), prof)
	case reflect.Pointer:
		if v.IsNil() {
			sb.WriteString("<nil>")
			return
		}
		switch v.Elem().Kind() {
		case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map:
			sb.WriteByte('&')
			nyx__formato(sb, v.Elem(), prof+1)
		default:
			sb.WriteString("0x" + strconv.FormatUint(uint64(v.Pointer()), 16))
		}
	case reflect.Slice, reflect.Array:
		sb.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			if nyx__lleno(sb) {
				sb.WriteString("…")
				break
			}
			nyx__formato(sb, v.Index(i), prof+1)
		}
		sb.WriteByte(']')
	case reflect.Map:
		sb.WriteString("map[")
		claves := v.MapKeys()
		sort.Slice(claves, func(i, j int) bool { return nyx__menor(claves[i], claves[j]) })
		for i, k := range claves {
			if i > 0 {
				sb.WriteByte(' ')
			}
			if nyx__lleno(sb) {
				sb.WriteString("…")
				break
			}
			nyx__formato(sb, k, prof+1)
			sb.WriteByte(':')
			nyx__formato(sb, v.MapIndex(k), prof+1)
		}
		sb.WriteByte(']')
	case reflect.Struct:
		sb.WriteByte('{')
		for i := 0; i < v.NumField(); i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			if nyx__lleno(sb) {
				sb.WriteString("…")
				break
			}
			nyx__formato(sb, v.Field(i), prof+1)
		}
		sb.WriteByte('}')
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		if v.IsNil() {
			sb.WriteString("<nil>")
			return
		}
		sb.WriteString("0x" + strconv.FormatUint(uint64(v.Pointer()), 16))
	default:
		sb.WriteString("?")
	}
}

// nyx__menor orders map keys like fmt does for the usual kinds (numbers, texts, booleans).
func nyx__menor(a, b reflect.Value) bool {
	for a.Kind() == reflect.Interface && !a.IsNil() {
		a = a.Elem()
	}
	for b.Kind() == reflect.Interface && !b.IsNil() {
		b = b.Elem()
	}
	if a.Kind() == b.Kind() {
		switch a.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return a.Int() < b.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return a.Uint() < b.Uint()
		case reflect.Float32, reflect.Float64:
			return a.Float() < b.Float()
		case reflect.String:
			return a.String() < b.String()
		case reflect.Bool:
			return !a.Bool() && b.Bool()
		}
	}
	var x, y strings.Builder
	nyx__formato(&x, a, 5)
	nyx__formato(&y, b, 5)
	return x.String() < y.String()
}

func nyx__Reiniciar(fuel int64) {
	nyx__f = fuel
	nyx__p = 0
	for i := range nyx__c {
		nyx__c[i] = 0
	}
	nyx__evs = nyx__evs[:0]
	nyx__enEv = false
}

func nyx__Cubiertas() []int32 {
	var out []int32
	for i, n := range nyx__c {
		if n > 0 {
			out = append(out, int32(i))
		}
	}
	return out
}

func nyx__Eventos() []nyx__Evento {
	return append([]nyx__Evento(nil), nyx__evs...)
}

var (
	_ = nyx__ev
	_ = nyx__Reiniciar
	_ = nyx__Cubiertas
	_ = nyx__Eventos
	_ = nyx__SinComb{}
	_ = nyx__Hondo{}
)
`
