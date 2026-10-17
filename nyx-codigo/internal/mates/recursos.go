package mates

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	"nyxcodigo/internal/nucleo"
)

// ---- mates.recursos: resources that run out ----
//
// The idea comes from the Gnostos engine of Nyx (ResolverRecursos, ResolverRecursosDinamicos and the
// adaptive Runge-Kutta of v6): "tengo 90 litros de aceite y gasto 3 por hora, y 100 metros de mecha y
// gasto 2 por hora; ¿cuánto dura?". With constant rates the answer is exact (big.Rat): each resource
// lasts (stock − minimum)/rate, piece by piece between refills, and everything lasts until the first
// one runs out. It is checked by substitution (the levels at that instant, and before every refill)
// and by integrating the same system numerically. Rates that depend on t ("gasto 3*exp(-0.01*t) por
// hora") are integrated with RK4 and step doubling, labelled "aproximado", and checked by integrating
// again with a tolerance 1000 times smaller.

const (
	maxTextoRecursos = 3000
	maxRecursos      = 20
	maxRecargas      = 100
	maxPasosRK       = 400_000
	horizonteMax     = 1e7
)

var errNoRecursos = errors.New("no veo cuánto tienes y cuánto gastas por unidad de tiempo")

type periodoTiempo struct {
	uno, varias string
	segundos    int64
}

var unidadesTiempo = func() map[string]periodoTiempo {
	m := map[string]periodoTiempo{}
	for _, u := range []struct {
		formas      []string
		uno, varias string
		seg         int64
	}{
		{[]string{"segundo", "segundos", "seg", "segs"}, "segundo", "segundos", 1},
		{[]string{"minuto", "minutos", "min", "mins"}, "minuto", "minutos", 60},
		{[]string{"hora", "horas", "h"}, "hora", "horas", 3600},
		{[]string{"dia", "dias", "jornada", "jornadas"}, "día", "días", 86400},
		{[]string{"semana", "semanas"}, "semana", "semanas", 604800},
		{[]string{"mes", "meses"}, "mes", "meses", 2592000},
		{[]string{"año", "años", "ano", "anos"}, "año", "años", 31536000},
	} {
		for _, f := range u.formas {
			m[f] = periodoTiempo{u.uno, u.varias, u.seg}
		}
	}
	return m
}()

func (u periodoTiempo) texto(v *big.Rat) string {
	if v.Cmp(big.NewRat(1, 1)) == 0 {
		return u.uno
	}
	return u.varias
}

// unidadesMedida: the quantity units this reader knows, with their family and size, so that "300 ml
// por hora" can be taken from "2 litros".
var unidadesMedida = func() map[string]struct {
	familia string
	factor  *big.Rat
} {
	m := map[string]struct {
		familia string
		factor  *big.Rat
	}{}
	add := func(familia string, num, den int64, formas ...string) {
		for _, f := range formas {
			m[f] = struct {
				familia string
				factor  *big.Rat
			}{familia, big.NewRat(num, den)}
		}
	}
	add("l", 1, 1, "litro", "litros", "l")
	add("l", 1, 1000, "mililitro", "mililitros", "ml")
	add("l", 1, 100, "centilitro", "centilitros", "cl")
	add("l", 1, 10, "decilitro", "decilitros", "dl")
	add("m", 1, 1, "metro", "metros", "m")
	add("m", 1, 100, "centimetro", "centimetros", "cm")
	add("m", 1, 1000, "milimetro", "milimetros", "mm")
	add("m", 1000, 1, "kilometro", "kilometros", "km")
	add("g", 1, 1, "gramo", "gramos", "g", "gr")
	add("g", 1000, 1, "kilo", "kilos", "kilogramo", "kilogramos", "kg")
	add("g", 1, 1000, "miligramo", "miligramos", "mg")
	add("g", 1000000, 1, "tonelada", "toneladas")
	for _, w := range []string{"euro", "euros", "dolar", "dolares", "peso", "pesos", "unidad", "unidades", "galon", "galones",
		"barril", "barriles", "caja", "cajas", "bolsa", "bolsas", "botella", "botellas", "paquete", "paquetes",
		"saco", "sacos", "lata", "latas", "vaso", "vasos", "taza", "tazas", "tanque", "tanques", "bidon", "bidones"} {
		add(singular(w), 1, 1, w)
	}
	return m
}()

// convertir passes v from unit de to unit a (same family); ok=false if they cannot be compared.
func convertir(v *big.Rat, de, a string) (*big.Rat, bool) {
	if de == a || de == "" || a == "" {
		return v, true
	}
	ud, ok1 := unidadesMedida[de]
	ua, ok2 := unidadesMedida[a]
	if !ok1 || !ok2 || ud.familia != ua.familia {
		return nil, false
	}
	r := new(big.Rat).Mul(v, ud.factor)
	return r.Quo(r, ua.factor), true
}

// ---- the model ----

type recargaR struct {
	t, cantidad *big.Rat // t in the base unit of time
	texto       string
}

type recursoR struct {
	cosa, unidad string // as written ("aceite", "litros")
	clave        string
	stock        *big.Rat
	tasa         *big.Rat // per base unit of time (constant rates)
	tasaTexto    string   // "3 litros por hora"
	expr         Expr     // a rate that depends on t (nil if constant)
	escala       float64  // expression unit of time per base unit
	minimo       *big.Rat
	recargas     []recargaR
}

func (r *recursoR) min() *big.Rat {
	if r.minimo == nil {
		return new(big.Rat)
	}
	return r.minimo
}

// nombre: "el aceite", "la mecha", "las velas" ("" when the resource has no name).
func (r *recursoR) nombre() string {
	c := r.cosa
	if c == "" {
		return ""
	}
	n := nucleo.Normalizar(c)
	switch {
	case n == "agua" || n == "hacha" || n == "aguas":
		return "el " + c
	case strings.HasSuffix(n, "as"):
		return "las " + c
	case strings.HasSuffix(n, "s"):
		return "los " + c
	case strings.HasSuffix(n, "a") || strings.HasSuffix(n, "cion") || strings.HasSuffix(n, "dad") || strings.HasSuffix(n, "sion"):
		return "la " + c
	}
	return "el " + c
}

// etiqueta is the name for tables and lists ("aceite", or the unit).
func (r *recursoR) etiqueta() string {
	if r.cosa != "" {
		return mayusculaInicial(r.cosa)
	}
	if r.unidad != "" {
		return mayusculaInicial(r.unidad)
	}
	return "Lo que tienes"
}

func (r *recursoR) cantidad(v *big.Rat) string {
	s := textoRatCorto(v)
	if r.unidad != "" {
		s += " " + r.unidad
	}
	if r.unidad == "" && r.cosa != "" {
		s += " de " + r.cosa
	}
	return s
}

func textoRatCorto(v *big.Rat) string {
	if v.IsInt() {
		return textoRat(v)
	}
	f, _ := v.Float64()
	return textoRat(v) + " (≈ " + textoFloat(f) + ")"
}

type problemaRecursos struct {
	recursos []*recursoR
	base     periodoTiempo
	pregunta string // "dura" | "primero" | "queda" | "sobra" | ""
	cual     *recursoR
	en       *big.Rat // the instant asked about ("¿cuánto queda a las 5 horas?")
	enTexto  string
	numerico bool
	textoPre string
	notas    []string
}

// ---- reading Spanish ----

var (
	verbosStock = conjuntoPal(`tengo tenemos tiene tienen tienes hay habia guardo guardamos guarda guardan dispongo dispone
disponemos disponen llevo llevamos lleva llevan cuento contamos cuenta cuentan quedan`)
	verbosGasto = conjuntoPal(`gasto gasta gastan gastamos gastas gastaba gastaban consumo consume consumen consumimos
consumes quemo quema queman quemamos quemas uso usa usan usamos usas utilizo utiliza utilizan utilizamos bebo bebe
beben bebemos come comen comemos pierdo pierde pierden perdemos gastar consumir quemar usar gastando consumiendo
quemando usando`)
	verbosRecarga = conjuntoPal(`añado añade añaden añadimos echo echa echan echamos repongo repone reponen reponemos
meto mete meten metemos agrego agrega agregan agregamos pongo pone ponen ponemos recargo recarga recargan
recargamos relleno rellena rellenan rellenamos compro compra compran compramos llegan llega traen trae traigo
traemos sumo sumamos dan dan añadire echare repondre`)
	articulosR = conjuntoPal(`el la los las un una unos unas mi mis su sus tu tus nuestro nuestra nuestros nuestras lo`)
	noCosa     = conjuntoPal(`por cada a al y e o en de del que se hasta durante para con sin las los el la cuanto
cuanta cuando como mas menos ya me te le nos les lo hay tengo tiene gasto gasta pero`)
	funcionesTasa = conjuntoPal(`t exp sen cos tan ln log raiz sqrt pi`)
)

// frasesMinimo introduce the amount that must stay ("sin bajar de 10").
var frasesMinimo = [][]string{
	{"sin", "bajar", "nunca", "de"}, {"sin", "bajar", "de"}, {"sin", "que", "baje", "de"}, {"sin", "que", "bajen", "de"},
	{"sin", "que", "quede", "menos", "de"}, {"sin", "que", "queden", "menos", "de"}, {"sin", "quedar", "por", "debajo", "de"},
	{"sin", "bajar", "por", "debajo", "de"}, {"dejando", "al", "menos"}, {"dejando", "como", "minimo"},
	{"dejando", "un", "minimo", "de"}, {"dejando"}, {"dejar", "al", "menos"}, {"dejar", "como", "minimo"}, {"dejar"},
	{"guardando"}, {"reservando"}, {"con", "una", "reserva", "de"}, {"una", "reserva", "de"}, {"un", "minimo", "de"},
	{"como", "minimo"}, {"quedando", "al", "menos"}, {"quedando", "como", "minimo"}, {"que", "queden", "al", "menos"},
	{"que", "quede", "al", "menos"}, {"sin", "gastar", "los", "ultimos"}, {"sin", "gastar", "las", "ultimas"},
	{"minimo", "de"},
}

// frasesMomento introduce an instant ("a las 10 horas", "al cabo de 3 días").
var frasesMomento = [][]string{
	{"a", "las"}, {"a", "los"}, {"al", "cabo", "de"}, {"tras"}, {"despues", "de"}, {"pasadas"}, {"pasados"},
	{"dentro", "de"}, {"luego", "de"},
}

type lectorRecursos struct {
	texto      string
	ts         []tokC
	recursos   []*recursoR
	modo       byte // 's' stock, 'g' consumption, 'r' refill
	pendientes []cantidadR
	recargas   []cantidadR // refills waiting for their instant
	momento    *big.Rat    // instant said in this sentence (base unit pending: kept with its unit)
	momentoU   periodoTiempo
	ultimo     *recursoR
	base       *periodoTiempo
	err        error
	periodico  bool
}

type cantidadR struct {
	valor        *big.Rat
	expr         Expr
	exprTexto    string
	unidad, cosa string
	texto        string
}

func palabraEn(ts []tokC, i int) string {
	if i >= 0 && i < len(ts) && ts[i].tipo == 'w' {
		return ts[i].w
	}
	return ""
}

func fraseEn(ts []tokC, i int, f []string) bool {
	if i+len(f) > len(ts) {
		return false
	}
	for k, w := range f {
		if palabraEn(ts, i+k) != w || ts[i+k].pregunta != ts[i].pregunta {
			return false
		}
	}
	return true
}

func cualquieraEn(ts []tokC, i int, fs [][]string) int {
	for _, f := range fs {
		if fraseEn(ts, i, f) {
			return len(f)
		}
	}
	return 0
}

// numeroEn reads a number at ts[i]: digits, words, "un"/"una" before a unit of time.
func numeroEn(ts []tokC, i int) (*big.Rat, int) {
	if i >= len(ts) {
		return nil, 0
	}
	if ts[i].tipo == 'n' {
		return ts[i].num, 1
	}
	if w := palabraEn(ts, i); (w == "un" || w == "una") && i+1 < len(ts) {
		if _, ok := unidadesTiempo[palabraEn(ts, i+1)]; ok {
			return big.NewRat(1, 1), 1
		}
	}
	return nil, 0
}

// momentoEn reads "a las 10 horas", "al cabo de 2 días", "a la hora 10".
func momentoEn(ts []tokC, i int) (*big.Rat, periodoTiempo, int, bool) {
	if k := cualquieraEn(ts, i, frasesMomento); k > 0 {
		if v, kn := numeroEn(ts, i+k); kn > 0 {
			if u, ok := unidadesTiempo[palabraEn(ts, i+k+kn)]; ok {
				return v, u, k + kn + 1, true
			}
		}
	}
	// "a la hora 10", "en el día 3"
	if fraseEn(ts, i, []string{"a", "la"}) || fraseEn(ts, i, []string{"en", "la"}) || fraseEn(ts, i, []string{"en", "el"}) || fraseEn(ts, i, []string{"el"}) {
		k := 2
		if palabraEn(ts, i) == "el" {
			k = 1
		}
		if u, ok := unidadesTiempo[palabraEn(ts, i+k)]; ok {
			if v, kn := numeroEn(ts, i+k+1); kn > 0 {
				return v, u, k + 1 + kn, true
			}
		}
	}
	return nil, periodoTiempo{}, 0, false
}

// periodoEn reads the time part of a rate: "por hora", "cada 2 horas", "a la semana", "al día",
// "en 3 horas", "/h", "diarios". The rate is the amount divided by the returned number of units.
func periodoEn(ts []tokC, i int) (periodoTiempo, *big.Rat, int, bool) {
	uno := big.NewRat(1, 1)
	w := palabraEn(ts, i)
	if strings.HasPrefix(w, "diari") {
		return unidadesTiempo["dia"], uno, 1, true
	}
	k := 0
	switch {
	case fraseEn(ts, i, []string{"por", "cada"}):
		k = 2
	case w == "por" || w == "cada" || w == "al" || w == "en":
		k = 1
	case fraseEn(ts, i, []string{"a", "la"}) || fraseEn(ts, i, []string{"a", "el"}):
		k = 2
	case i < len(ts) && ts[i].tipo == 'o' && ts[i].w == "/":
		k = 1
	default:
		return periodoTiempo{}, nil, 0, false
	}
	if u, ok := unidadesTiempo[palabraEn(ts, i+k)]; ok && w != "en" {
		return u, uno, k + 1, true
	}
	if v, kn := numeroEn(ts, i+k); kn > 0 && v.Sign() > 0 {
		if u, ok := unidadesTiempo[palabraEn(ts, i+k+kn)]; ok {
			return u, v, k + kn + 1, true
		}
	}
	return periodoTiempo{}, nil, 0, false
}

// juntarNumerosC turns runs of number words into number tokens ("noventa", "treinta y cinco").
func juntarNumerosC(texto string, ts []tokC) []tokC {
	ws := make([]string, len(ts))
	for i, t := range ts {
		if t.tipo == 'w' {
			ws[i] = t.w
		}
	}
	out := make([]tokC, 0, len(ts))
	for i := 0; i < len(ts); {
		if ts[i].tipo == 'w' {
			if v, k := numeroPalabras(ws, i); k > 0 {
				t := ts[i]
				t.tipo, t.num = 'n', big.NewRat(v, 1)
				t.hasta = ts[i+k-1].hasta
				t.orig = texto[t.desde:t.hasta]
				t.w = t.orig
				out = append(out, t)
				i += k
				continue
			}
		}
		out = append(out, ts[i])
		i++
	}
	return out
}

// cosaEn reads the thing after "de": "de aceite", "de la leña".
func cosaEn(ts []tokC, i int) (string, int) {
	j := i
	for j < len(ts) && articulosR[palabraEn(ts, j)] {
		j++
	}
	if w := palabraEn(ts, j); w != "" && !noCosa[w] {
		if _, esTiempo := unidadesTiempo[w]; !esTiempo {
			return ts[j].orig, j - i + 1
		}
	}
	return "", 0
}

// cantidadEn reads "90 litros de aceite", "50 de mecha", "100 velas", "3", or (when expr) a rate
// written as an expression of t ("3*exp(-0.01*t)").
func (l *lectorRecursos) cantidadEn(i int, expr bool) (cantidadR, int, bool) {
	ts := l.ts
	var c cantidadR
	j := i
	if expr {
		k := i
		hayT, hayOp := false, false
		for k < len(ts) && !ts[k].pregunta {
			t := ts[k]
			ok := t.tipo == 'n' || t.tipo == 'o' && t.w != "/" || t.tipo == 'o' && t.w == "/" && k+1 < len(ts) && ts[k+1].tipo != 'w' ||
				t.tipo == 's' && (t.w == "(" || t.w == ")") || t.tipo == 'w' && funcionesTasa[t.w]
			if !ok {
				break
			}
			hayT = hayT || t.tipo == 'w' && t.w == "t"
			hayOp = hayOp || t.tipo == 'o' || t.tipo == 'w' && t.w != "t"
			k++
		}
		if k > i+1 && (hayT || hayOp) {
			txt := strings.TrimSpace(l.texto[ts[i].desde:ts[k-1].hasta])
			e, err := AnalizarExpr(txt)
			if err != nil {
				l.err = fmt.Errorf("%w: no entiendo el gasto «%s»", nucleo.ErrNoEntiendo, txt)
				return c, 0, false
			}
			for _, v := range variables(e) {
				if v != "t" {
					l.err = fmt.Errorf("%w: en el gasto «%s» solo puede aparecer t (el tiempo), no «%s»", nucleo.ErrNoEntiendo, txt, v)
					return c, 0, false
				}
			}
			if len(variables(e)) == 0 {
				v, err := evaluar(context.Background(), e, nil)
				if err == nil && v.racional() {
					c.valor = new(big.Rat).Set(v.a)
				} else {
					c.expr = e
				}
			} else {
				c.expr = e
			}
			c.exprTexto = strings.ReplaceAll(e.String(), "*", "·")
			j = k
		}
	}
	if c.valor == nil && c.expr == nil {
		if ts[i].tipo != 'n' {
			return c, 0, false
		}
		c.valor = new(big.Rat).Set(ts[i].num)
		j = i + 1
	}
	// "2 y medio"
	if fraseEn(ts, j, []string{"y", "medio"}) || fraseEn(ts, j, []string{"y", "media"}) {
		if c.valor != nil {
			c.valor.Add(c.valor, big.NewRat(1, 2))
		}
		j += 2
	}
	w := palabraEn(ts, j)
	switch {
	case w == "de":
		if cosa, k := cosaEn(ts, j+1); k > 0 {
			c.cosa = cosa
			j += 1 + k
		}
	case w != "" && !noCosa[w] && !articulosR[w]:
		if _, esTiempo := unidadesTiempo[w]; esTiempo {
			return c, 0, false // "3 horas": an amount of time, not of a thing
		}
		if palabraEn(ts, j+1) == "de" {
			if cosa, k := cosaEn(ts, j+2); k > 0 {
				c.unidad, c.cosa = ts[j].orig, cosa
				j += 2 + k
				break
			}
		}
		if _, ok := unidadesMedida[w]; ok {
			c.unidad = ts[j].orig
		} else {
			c.cosa = ts[j].orig
		}
		j++
		// "2 litros y medio"
		if (fraseEn(ts, j, []string{"y", "medio"}) || fraseEn(ts, j, []string{"y", "media"})) && c.valor != nil {
			c.valor.Add(c.valor, big.NewRat(1, 2))
			j += 2
		}
	}
	c.texto = strings.TrimSpace(l.texto[ts[i].desde:ts[j-1].hasta])
	return c, j - i, true
}

func claveCosa(cosa string) string { return singular(nucleo.Normalizar(cosa)) }

func (l *lectorRecursos) buscar(cosa string) *recursoR {
	if cosa == "" {
		return nil
	}
	k := claveCosa(cosa)
	for _, r := range l.recursos {
		if r.clave == k {
			return r
		}
	}
	return nil
}

func (l *lectorRecursos) porUnidad(unidad string) *recursoR {
	if unidad == "" {
		return nil
	}
	var hallado *recursoR
	n := 0
	for _, r := range l.recursos {
		if r.unidad == "" {
			continue
		}
		if _, ok := convertir(big.NewRat(1, 1), nucleo.Normalizar(unidad), nucleo.Normalizar(r.unidad)); ok {
			hallado = r
			n++
		}
	}
	if n == 1 {
		return hallado
	}
	return nil
}

func (l *lectorRecursos) nuevo(c cantidadR) *recursoR {
	if len(l.recursos) >= maxRecursos {
		l.err = fmt.Errorf("%w: como mucho %d recursos", nucleo.ErrNoSoportado, maxRecursos)
		return nil
	}
	r := &recursoR{cosa: c.cosa, unidad: c.unidad, clave: claveCosa(c.cosa)}
	if c.cosa == "" {
		r.clave = "#" + nucleo.Normalizar(c.unidad)
	}
	l.recursos = append(l.recursos, r)
	return r
}

// stock records "90 litros de aceite".
func (l *lectorRecursos) stock(c cantidadR) {
	if c.valor == nil {
		l.err = fmt.Errorf("%w: no entiendo cuánto tienes en «%s»", nucleo.ErrNoEntiendo, c.texto)
		return
	}
	r := l.buscar(c.cosa)
	if r == nil && c.cosa == "" {
		for _, x := range l.recursos {
			if x.stock == nil && (c.unidad == "" || l.porUnidad(c.unidad) == x) {
				r = x
				break
			}
		}
	}
	if r == nil {
		if r = l.nuevo(c); r == nil {
			return
		}
	}
	if r.stock != nil {
		l.err = fmt.Errorf("%w: me das dos cantidades de %s y no sé cuál usar", nucleo.ErrNoEntiendo, r.etiqueta())
		return
	}
	v := c.valor
	if c.unidad != "" && r.unidad != "" {
		w, ok := convertir(v, nucleo.Normalizar(c.unidad), nucleo.Normalizar(r.unidad))
		if !ok {
			l.err = fmt.Errorf("%w: no sé pasar %s a %s", nucleo.ErrNoEntiendo, c.unidad, r.unidad)
			return
		}
		v = w
	}
	if r.unidad == "" {
		r.unidad = c.unidad
	}
	r.stock = v
	l.ultimo = r
}

// gasto gives the pending amounts their unit of time.
func (l *lectorRecursos) gasto(u periodoTiempo, cada *big.Rat, texto string) {
	if l.base == nil {
		b := u
		l.base = &b
	}
	for _, c := range l.pendientes {
		r := l.buscar(c.cosa)
		if r == nil && c.cosa == "" {
			r = l.porUnidad(c.unidad)
		}
		if r == nil && c.cosa == "" && l.ultimo != nil && l.ultimo.tasa == nil && l.ultimo.expr == nil {
			r = l.ultimo
		}
		if r == nil && c.cosa == "" {
			for _, x := range l.recursos {
				if x.tasa == nil && x.expr == nil {
					r = x
					break
				}
			}
		}
		if r == nil && c.cosa != "" {
			r = l.nuevo(c) // "gasto 3 litros de aceite por hora y tengo 90": the stock comes later
		}
		if r == nil {
			l.err = fmt.Errorf("%w: no sé de qué es el gasto «%s»", nucleo.ErrNoEntiendo, c.texto)
			return
		}
		if r.tasa != nil || r.expr != nil {
			l.err = fmt.Errorf("%w: me das dos gastos de %s y no sé cuál usar", nucleo.ErrNoEntiendo, r.etiqueta())
			return
		}
		// seconds of the base unit over seconds of this unit: how many of these units fit in one base unit
		escala := big.NewRat(l.base.segundos, u.segundos)
		escala.Quo(escala, cada)
		if c.valor != nil {
			v := new(big.Rat).Set(c.valor)
			if c.unidad != "" && r.unidad != "" {
				w, ok := convertir(v, nucleo.Normalizar(c.unidad), nucleo.Normalizar(r.unidad))
				if !ok {
					l.err = fmt.Errorf("%w: no sé pasar %s a %s", nucleo.ErrNoEntiendo, c.unidad, r.unidad)
					return
				}
				v = w
			}
			r.tasa = v.Mul(v, escala)
		} else {
			if c.unidad != "" && r.unidad != "" && nucleo.Normalizar(c.unidad) != nucleo.Normalizar(r.unidad) {
				l.err = fmt.Errorf("%w: el gasto «%s» va en otra unidad; escríbelo en %s", nucleo.ErrNoSoportado, c.texto, r.unidad)
				return
			}
			r.expr = c.expr
			// the expression's t is in its own unit: t = escala·τ, and the rate per base unit is escala·f(t)
			r.escala, _ = escala.Float64()
		}
		if r.unidad == "" {
			r.unidad = c.unidad
		}
		r.tasaTexto = strings.TrimSpace(c.texto + " " + texto)
		l.ultimo = r
	}
	l.pendientes = nil
}

// cerrarPendientes: amounts after a consumption verb that never got a unit of time are stocks
// ("…gasto 3 por hora, y 100 metros de mecha y gasto 2 por hora").
func (l *lectorRecursos) cerrarPendientes() {
	for _, c := range l.pendientes {
		if r := l.buscar(c.cosa); (r == nil || r.stock == nil) && (c.cosa != "" || c.unidad != "") && c.expr == nil {
			l.stock(c)
			continue
		}
		l.err = fmt.Errorf("%w: no sé en cuánto tiempo gastas «%s» (¿por hora, por día…?)", nucleo.ErrNoEntiendo, c.texto)
	}
	l.pendientes = nil
}

func (l *lectorRecursos) cerrarRecargas() {
	for _, c := range l.recargas {
		l.err = fmt.Errorf("%w: no sé cuándo añades «%s» (por ejemplo: «a las 10 horas añado 50 de mecha»)", nucleo.ErrNoEntiendo, c.texto)
	}
	l.recargas = nil
}

func (l *lectorRecursos) recarga(c cantidadR, t *big.Rat, u periodoTiempo) {
	if c.valor == nil {
		l.err = fmt.Errorf("%w: no entiendo cuánto añades en «%s»", nucleo.ErrNoEntiendo, c.texto)
		return
	}
	r := l.buscar(c.cosa)
	if r == nil && c.cosa == "" {
		r = l.porUnidad(c.unidad)
	}
	if r == nil && c.cosa == "" && len(l.recursos) == 1 {
		r = l.recursos[0]
	}
	if r == nil && c.cosa == "" {
		r = l.ultimo
	}
	if r == nil {
		l.err = fmt.Errorf("%w: no sé a qué añades «%s»", nucleo.ErrNoEntiendo, c.texto)
		return
	}
	if len(r.recargas) >= maxRecargas {
		l.err = fmt.Errorf("%w: como mucho %d recargas", nucleo.ErrNoSoportado, maxRecargas)
		return
	}
	v := new(big.Rat).Set(c.valor)
	if c.unidad != "" && r.unidad != "" {
		w, ok := convertir(v, nucleo.Normalizar(c.unidad), nucleo.Normalizar(r.unidad))
		if !ok {
			l.err = fmt.Errorf("%w: no sé pasar %s a %s", nucleo.ErrNoEntiendo, c.unidad, r.unidad)
			return
		}
		v = w
	}
	r.recargas = append(r.recargas, recargaR{t: new(big.Rat).Set(t), cantidad: v, texto: fmt.Sprintf("a las %s %s añades %s", textoRat(t), u.texto(t), c.texto)})
	// the instant is converted to the base unit once the base unit is known (see aBase)
	r.recargas[len(r.recargas)-1].t = new(big.Rat).Mul(t, big.NewRat(u.segundos, 1))
}

// minimo records "sin bajar de 10 litros de aceite" for the named resource, the last one, or all.
func (l *lectorRecursos) minimo(c cantidadR, enPregunta bool) {
	if c.valor == nil {
		l.err = fmt.Errorf("%w: no entiendo el mínimo «%s»", nucleo.ErrNoEntiendo, c.texto)
		return
	}
	var destino []*recursoR
	switch r := l.buscar(c.cosa); {
	case r != nil:
		destino = []*recursoR{r}
	case c.cosa == "" && l.porUnidad(c.unidad) != nil:
		destino = []*recursoR{l.porUnidad(c.unidad)}
	case enPregunta || l.ultimo == nil:
		destino = l.recursos
	default:
		destino = []*recursoR{l.ultimo}
	}
	for _, r := range destino {
		v := new(big.Rat).Set(c.valor)
		if c.unidad != "" && r.unidad != "" {
			w, ok := convertir(v, nucleo.Normalizar(c.unidad), nucleo.Normalizar(r.unidad))
			if !ok {
				continue
			}
			v = w
		}
		r.minimo = v
	}
}

func finDeFrase(t tokC) bool {
	return t.tipo == 's' && (t.w == "." || t.w == ";" || t.w == "\n" || t.w == "?" || t.w == "!" || t.w == ":")
}

// leerRecursos reads a resources problem. It is linear in the text.
func leerRecursos(texto string) (*problemaRecursos, error) {
	if len(texto) > maxTextoRecursos {
		return nil, fmt.Errorf("%w: el texto es demasiado largo", nucleo.ErrNoSoportado)
	}
	l := &lectorRecursos{texto: texto}
	l.ts = juntarNumerosC(texto, tokensCadena(texto))
	ts := l.ts
	for i := 0; i < len(ts) && l.err == nil; i++ {
		t := ts[i]
		if t.pregunta {
			continue
		}
		if finDeFrase(t) {
			l.cerrarPendientes()
			l.cerrarRecargas()
			l.momento, l.modo = nil, 0
			continue
		}
		if k := cualquieraEn(ts, i, frasesMinimo); k > 0 {
			if c, kc, ok := l.cantidadEn(i+k, false); ok {
				l.cerrarPendientes()
				l.minimo(c, false)
				i += k + kc - 1
				continue
			}
		}
		if fraseEn(ts, i, []string{"cada"}) {
			if _, _, k, ok := periodoEn(ts, i); ok && (l.modo == 'r' || l.siguienteVerboRecarga(i+k)) && len(l.pendientes) == 0 {
				l.periodico = true
				i += k - 1
				continue
			}
		}
		if v, u, k, ok := momentoEn(ts, i); ok {
			l.momento, l.momentoU = v, u
			for _, c := range l.recargas {
				l.recarga(c, v, u)
			}
			l.recargas = nil
			i += k - 1
			continue
		}
		if l.modo == 'g' && len(l.pendientes) > 0 {
			if u, cada, k, ok := periodoEn(ts, i); ok {
				l.gasto(u, cada, strings.TrimSpace(texto[ts[i].desde:ts[i+k-1].hasta]))
				i += k - 1
				continue
			}
		}
		if t.tipo == 'w' {
			switch {
			case verbosStock[t.w]:
				l.cerrarPendientes()
				l.modo = 's'
				continue
			case verbosGasto[t.w]:
				l.cerrarPendientes()
				l.modo = 'g'
				continue
			case verbosRecarga[t.w]:
				l.cerrarPendientes()
				l.modo = 'r'
				continue
			}
			if r := l.buscar(t.orig); r != nil {
				l.ultimo = r
			}
		}
		if t.tipo == 'n' || l.modo == 'g' && (t.tipo == 'o' || t.tipo == 's' && t.w == "(" || t.tipo == 'w' && funcionesTasa[t.w]) {
			c, k, ok := l.cantidadEn(i, l.modo == 'g')
			if !ok {
				continue
			}
			switch l.modo {
			case 'g':
				l.pendientes = append(l.pendientes, c)
			case 'r':
				if l.momento != nil {
					l.recarga(c, l.momento, l.momentoU)
				} else {
					l.recargas = append(l.recargas, c)
				}
			default:
				if c.cosa != "" || c.unidad != "" || l.modo == 's' {
					l.stock(c)
				}
			}
			i += k - 1
		}
	}
	if l.err == nil {
		l.cerrarPendientes()
		l.cerrarRecargas()
	}
	if l.err != nil {
		return nil, l.err
	}
	if l.periodico {
		return nil, fmt.Errorf("%w: todavía no sé hacer recargas que se repiten («cada 10 horas…»)", nucleo.ErrNoSoportado)
	}
	pr := &problemaRecursos{}
	for _, r := range l.recursos {
		if r.stock == nil || (r.tasa == nil && r.expr == nil) {
			continue
		}
		pr.recursos = append(pr.recursos, r)
	}
	if len(pr.recursos) == 0 || l.base == nil {
		return nil, fmt.Errorf("%w: %s", nucleo.ErrNoEntiendo, errNoRecursos)
	}
	for _, r := range l.recursos {
		if r.stock == nil {
			return nil, fmt.Errorf("%w: no me dices cuánto tienes de %s", nucleo.ErrNoEntiendo, r.etiqueta())
		}
		if r.tasa == nil && r.expr == nil {
			pr.notas = append(pr.notas, fmt.Sprintf("De %s no me dices cuánto gastas: no lo tengo en cuenta.", strings.ToLower(r.etiqueta())))
		}
	}
	pr.base = *l.base
	segBase := big.NewRat(pr.base.segundos, 1)
	for _, r := range pr.recursos {
		for i := range r.recargas {
			r.recargas[i].t.Quo(r.recargas[i].t, segBase)
		}
		sort.SliceStable(r.recargas, func(a, b int) bool { return r.recargas[a].t.Cmp(r.recargas[b].t) < 0 })
		if r.expr != nil {
			pr.numerico = true
		}
	}
	l.leerPregunta(pr)
	if l.err != nil {
		return nil, l.err
	}
	return pr, nil
}

func (l *lectorRecursos) siguienteVerboRecarga(i int) bool {
	for k := i; k < len(l.ts) && k < i+4; k++ {
		if verbosRecarga[palabraEn(l.ts, k)] {
			return true
		}
	}
	return false
}

// leerPregunta finds what is asked: how long it lasts, what runs out first, or what is left at an
// instant. A minimum said in the question ("¿cuánto dura sin bajar de 10?") applies too.
func (l *lectorRecursos) leerPregunta(pr *problemaRecursos) {
	ts := l.ts
	var ws, partes []string
	for i := 0; i < len(ts); i++ {
		if !ts[i].pregunta {
			continue
		}
		partes = append(partes, ts[i].orig)
		if k := cualquieraEn(ts, i, frasesMinimo); k > 0 {
			if c, kc, ok := l.cantidadEn(i+k, false); ok {
				l.minimo(c, true)
				i += k + kc - 1
				continue
			}
		}
		if v, u, k, ok := momentoEn(ts, i); ok && pr.en == nil {
			pr.en = new(big.Rat).Mul(v, big.NewRat(u.segundos, pr.base.segundos))
			pr.enTexto = fmt.Sprintf("%s %s", textoRat(v), u.texto(v))
			i += k - 1
			continue
		}
		if ts[i].tipo == 'w' {
			ws = append(ws, ts[i].w)
			for _, r := range pr.recursos {
				if r.cosa != "" && claveCosa(ts[i].orig) == r.clave {
					if pr.cual == nil || pr.cual == r {
						pr.cual = r
					} else {
						pr.cual = &recursoR{} // two resources named: overall answer
					}
				}
			}
		}
		if ts[i].tipo == 'n' && pr.en == nil {
			// "¿cuánto queda a las 5?" (unit left out): the base unit
			if i > 0 && (palabraEn(ts, i-1) == "las" || palabraEn(ts, i-1) == "los") {
				pr.en = new(big.Rat).Set(ts[i].num)
				pr.enTexto = fmt.Sprintf("%s %s", textoRat(ts[i].num), pr.base.texto(ts[i].num))
			}
		}
	}
	if pr.cual != nil && pr.cual.stock == nil {
		pr.cual = nil
	}
	pr.textoPre = strings.Join(partes, " ")
	s := " " + strings.Join(ws, " ") + " "
	tiene := func(fs ...string) bool {
		for _, f := range fs {
			if strings.Contains(s, " "+f+" ") {
				return true
			}
		}
		return false
	}
	switch {
	case tiene("primero", "antes", "dura menos", "duran menos", "agota antes", "acaba antes"):
		pr.pregunta = "primero"
	case pr.en != nil && tiene("queda", "quedan", "quedara", "quedaran", "sobra", "sobran", "hay", "habra", "tengo", "tendre", "tendra", "quedaria"):
		pr.pregunta = "queda"
	case tiene("dura", "duran", "durara", "duraran", "durar", "aguanta", "aguantan", "alcanza", "alcanzan", "llega", "tarda",
		"tardan", "duracion", "cuanto tiempo", "cuantas horas", "cuantos dias", "cuantos minutos", "cuantas semanas",
		"cuantos meses", "cuantos años", "cuantos segundos", "para cuanto"):
		pr.pregunta = "dura"
	case tiene("cuando") && tiene("acaba", "acaban", "agota", "agotan", "termina", "terminan", "gasta", "gastan", "consume",
		"consumen", "vacia", "vacian", "apaga", "apagan", "acabara", "acabaran", "agotara", "agotaran", "termine", "acabe", "agote"):
		pr.pregunta = "dura"
	case tiene("queda", "quedan", "sobra", "sobran"):
		pr.pregunta = "sobra"
	}
	if pr.pregunta == "primero" || pr.cual != nil && pr.cual.clave == "" {
		pr.cual = nil
	}
}

// ---- the exact method (constant rates) ----

type agotado struct {
	t      *big.Rat // nil: it never runs out
	justo  bool     // a refill arrives exactly when it was going to run out
	pasos  []string
	yaBajo bool
}

// agotarExacto: when the level of r reaches its minimum. Between refills the level falls in a
// straight line, so each piece is (level − minimum)/rate.
func agotarExacto(r *recursoR, u periodoTiempo) agotado {
	var a agotado
	m := r.min()
	nivel := new(big.Rat).Set(r.stock)
	ahora := new(big.Rat)
	k := 0
	for ; k < len(r.recargas) && r.recargas[k].t.Sign() <= 0; k++ {
		nivel.Add(nivel, r.recargas[k].cantidad)
		a.pasos = append(a.pasos, fmt.Sprintf("al empezar añades %s → %s", textoRat(r.recargas[k].cantidad), r.cantidad(nivel)))
	}
	if r.tasa.Sign() <= 0 {
		a.pasos = append(a.pasos, "no gastas nada (o ganas), así que no se acaba")
		return a
	}
	if nivel.Cmp(m) <= 0 {
		a.t, a.yaBajo = new(big.Rat), true
		a.pasos = append(a.pasos, fmt.Sprintf("ya empieza en %s, que no está por encima del mínimo", r.cantidad(nivel)))
		return a
	}
	for {
		margen := new(big.Rat).Sub(nivel, m)
		dur := new(big.Rat).Quo(margen, r.tasa)
		fin := new(big.Rat).Add(ahora, dur)
		cuenta := fmt.Sprintf("%s ÷ %s por %s = %s %s", textoRat(margen), textoRat(r.tasa), u.uno, textoRat(dur), u.texto(dur))
		if m.Sign() != 0 {
			cuenta = fmt.Sprintf("(%s − %s) ÷ %s por %s = %s %s", textoRat(nivel), textoRat(m), textoRat(r.tasa), u.uno, textoRat(dur), u.texto(dur))
		}
		if k >= len(r.recargas) || fin.Cmp(r.recargas[k].t) < 0 {
			if ahora.Sign() == 0 {
				a.pasos = append(a.pasos, cuenta)
			} else {
				a.pasos = append(a.pasos, fmt.Sprintf("desde las %s %s: %s más → se acaba a las %s %s", textoRat(ahora), u.texto(ahora), cuenta, textoRat(fin), u.texto(fin)))
			}
			a.t = fin
			return a
		}
		rc := r.recargas[k]
		if fin.Cmp(rc.t) == 0 {
			a.justo = true
		}
		gastado := new(big.Rat).Mul(r.tasa, new(big.Rat).Sub(rc.t, ahora))
		nivel.Sub(nivel, gastado)
		antes := new(big.Rat).Set(nivel)
		nivel.Add(nivel, rc.cantidad)
		a.pasos = append(a.pasos, fmt.Sprintf("a las %s %s quedan %s y añades %s → %s", textoRat(rc.t), u.texto(rc.t), textoRatCorto(antes), textoRatCorto(rc.cantidad), r.cantidad(nivel)))
		ahora.Set(rc.t)
		k++
	}
}

// nivelExacto is the level of r at instant t with the closed formula: stock + refills up to t − rate·t.
func nivelExacto(r *recursoR, t *big.Rat, incluirEnT bool) *big.Rat {
	v := new(big.Rat).Set(r.stock)
	for _, rc := range r.recargas {
		if c := rc.t.Cmp(t); c < 0 || c == 0 && incluirEnT {
			v.Add(v, rc.cantidad)
		}
	}
	return v.Sub(v, new(big.Rat).Mul(r.tasa, t))
}

// comprobarExacto checks by substitution, with the closed formula and not with the piece-by-piece
// loop: at t* the first resources are exactly at their minimum, every resource is at or above its
// minimum, and before every refill nobody was below.
func comprobarExacto(pr *problemaRecursos, tFin *big.Rat, primeros []*recursoR) bool {
	for _, r := range pr.recursos {
		m := r.min()
		if r.tasa.Sign() > 0 && nivelExacto(r, new(big.Rat), true).Cmp(m) <= 0 {
			if tFin.Sign() != 0 {
				return false
			}
			continue
		}
		if nivelExacto(r, tFin, false).Cmp(m) < 0 {
			return false
		}
		for _, rc := range r.recargas {
			if rc.t.Sign() > 0 && rc.t.Cmp(tFin) <= 0 && nivelExacto(r, rc.t, false).Cmp(m) < 0 {
				return false
			}
		}
	}
	for _, r := range primeros {
		if tFin.Sign() == 0 {
			continue
		}
		if nivelExacto(r, tFin, false).Cmp(r.min()) != 0 {
			return false
		}
	}
	return true
}

// ---- the numeric method: RK4 with step doubling ----

// evalTasa computes an expression in float64 with t given.
func evalTasa(e Expr, t float64) (float64, error) {
	switch x := e.(type) {
	case Num:
		f, _ := x.V.Float64()
		return f, nil
	case Val:
		return x.v.float(), nil
	case Var:
		if x.Nombre == "t" {
			return t, nil
		}
		return 0, ErrIncognita
	case Neg:
		a, err := evalTasa(x.A, t)
		return -a, err
	case Func:
		a, err := evalTasa(x.A, t)
		if err != nil {
			return 0, err
		}
		v, err := aplicarFunc(x.Nombre, vAprox(a))
		if err != nil {
			return 0, err
		}
		return v.float(), nil
	case Op:
		a, err := evalTasa(x.A, t)
		if err != nil {
			return 0, err
		}
		b, err := evalTasa(x.B, t)
		if err != nil {
			return 0, err
		}
		var v float64
		switch x.Op {
		case '+':
			v = a + b
		case '-':
			v = a - b
		case '*':
			v = a * b
		case '/':
			if b == 0 {
				return 0, ErrDivisionCero
			}
			v = a / b
		case '^':
			v = math.Pow(a, b)
		default:
			return 0, fmt.Errorf("no sé usar «%c» en un gasto", x.Op)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, ErrNoReal
		}
		return v, nil
	}
	return 0, fmt.Errorf("no sé calcular %s en un gasto", e)
}

// tasaEn is the consumption per base unit of time at instant τ (base units).
func (r *recursoR) tasaEn(tau float64) (float64, error) {
	if r.expr == nil {
		f, _ := r.tasa.Float64()
		return f, nil
	}
	v, err := evalTasa(r.expr, r.escala*tau)
	return r.escala * v, err
}

type resultadoNum struct {
	t         float64
	primeros  []int
	niveles   []float64
	errNivel  float64 // accumulated local error estimate (levels)
	pasos     int
	nunca     bool
	horizonte float64
}

func pasoRK4(rs []*recursoR, t float64, y []float64, h float64) ([]float64, error) {
	out := make([]float64, len(y))
	for i, r := range rs {
		k1, err := r.tasaEn(t)
		if err != nil {
			return nil, err
		}
		k2, err := r.tasaEn(t + h/2)
		if err != nil {
			return nil, err
		}
		k4, err := r.tasaEn(t + h)
		if err != nil {
			return nil, err
		}
		// the derivative depends only on t, so k2 = k3
		out[i] = y[i] - h/6*(k1+4*k2+k4)
	}
	return out, nil
}

type eventoR struct {
	t    float64
	i    int
	cant float64
}

// integrar integrates every level from its stock until the first reaches its minimum (or the
// horizon). The step is halved while the step-doubling estimate exceeds tol and doubled when it is
// far below; refills are steps of their own; the crossing is refined by bisection inside the step.
func integrar(ctx context.Context, rs []*recursoR, horizonte, tol float64) (resultadoNum, error) {
	res := resultadoNum{horizonte: horizonte}
	n := len(rs)
	y := make([]float64, n)
	mins := make([]float64, n)
	var eventos []eventoR
	for i, r := range rs {
		y[i], _ = r.stock.Float64()
		mins[i], _ = r.min().Float64()
		for _, rc := range r.recargas {
			te, _ := rc.t.Float64()
			q, _ := rc.cantidad.Float64()
			eventos = append(eventos, eventoR{te, i, q})
		}
	}
	sort.SliceStable(eventos, func(a, b int) bool { return eventos[a].t < eventos[b].t })
	ev := 0
	for ; ev < len(eventos) && eventos[ev].t <= 0; ev++ {
		y[eventos[ev].i] += eventos[ev].cant
	}
	for i := range rs {
		if g, err := rs[i].tasaEn(0); err == nil && g > 0 && y[i] <= mins[i] {
			res.t, res.primeros, res.niveles = 0, []int{i}, y
			return res, nil
		}
	}
	t := 0.0
	hMax := horizonte / 64
	h := hMax / 64
	hMin := horizonte * 1e-15
	for res.pasos = 0; res.pasos < maxPasosRK; {
		if res.pasos%256 == 0 && ctx.Err() != nil {
			return res, nucleo.ErrSinTiempo
		}
		if t >= horizonte {
			res.nunca, res.t, res.niveles = true, horizonte, y
			return res, nil
		}
		hEste := math.Min(h, horizonte-t)
		aEvento := false
		if ev < len(eventos) && eventos[ev].t-t <= hEste {
			hEste, aEvento = eventos[ev].t-t, true
		}
		y1, err := pasoRK4(rs, t, y, hEste)
		if err != nil {
			return res, err
		}
		ym, err := pasoRK4(rs, t, y, hEste/2)
		if err != nil {
			return res, err
		}
		y2, err := pasoRK4(rs, t+hEste/2, ym, hEste/2)
		if err != nil {
			return res, err
		}
		errPaso := 0.0
		for i := range y {
			errPaso = math.Max(errPaso, math.Abs(y2[i]-y1[i])/15)
		}
		if errPaso > tol && hEste > hMin {
			h = hEste / 2
			continue
		}
		res.pasos++
		res.errNivel += errPaso
		// the first crossing inside this step, refined by bisection
		mejor, cruce := 2.0, []int(nil)
		for i := range y {
			if y2[i] > mins[i] {
				continue
			}
			lo, hi := 0.0, 1.0
			for it := 0; it < 64; it++ {
				mid := (lo + hi) / 2
				ys, err := pasoRK4(rs, t, y, hEste*mid)
				if err != nil {
					return res, err
				}
				if ys[i] > mins[i] {
					lo = mid
				} else {
					hi = mid
				}
			}
			switch {
			case hi < mejor-1e-12:
				mejor, cruce = hi, []int{i}
			case math.Abs(hi-mejor) <= 1e-12:
				cruce = append(cruce, i)
			}
		}
		if cruce != nil {
			yf, err := pasoRK4(rs, t, y, hEste*mejor)
			if err != nil {
				return res, err
			}
			res.t, res.primeros, res.niveles = t+hEste*mejor, cruce, yf
			return res, nil
		}
		y = y2
		if aEvento {
			t = eventos[ev].t
			for ; ev < len(eventos) && eventos[ev].t <= t; ev++ {
				y[eventos[ev].i] += eventos[ev].cant
			}
		} else {
			t += hEste
		}
		if errPaso < tol/32 {
			h = math.Min(2*hEste, hMax)
		}
	}
	return res, fmt.Errorf("%w: el cálculo numérico necesita demasiados pasos", nucleo.ErrSinTiempo)
}

// horizonteDe: how far to integrate: a thousand times the longest time any resource would last at its
// initial rate (at least 1000 units, at most horizonteMax).
func horizonteDe(rs []*recursoR) float64 {
	h := 1000.0
	for _, r := range rs {
		total, _ := r.stock.Float64()
		for _, rc := range r.recargas {
			q, _ := rc.cantidad.Float64()
			total += q
			te, _ := rc.t.Float64()
			h = math.Max(h, 2*te)
		}
		if g, err := r.tasaEn(0); err == nil && g > 0 {
			h = math.Max(h, 1000*total/g)
		}
	}
	return math.Min(h, horizonteMax)
}

// ---- the resolver ----

type resRecursos struct{}

func (resRecursos) Nombre() string { return "mates.recursos" }

var (
	raicesGasto  = []string{"gast", "consum", "quem", "usa", "uso", "utiliz", "beb", "come", "pierd", "perd"}
	palabrasHora = []string{"segundo", "minuto", "hora", "dia", "semana", "mes", "año", "ano", "diari"}
)

// pareceRecursos is the cheap test: a consumption verb and a unit of time.
func pareceRecursos(t string) bool {
	n := nucleo.Normalizar(t)
	gasto := false
	for _, w := range strings.Fields(n) {
		for _, r := range raicesGasto {
			if strings.HasPrefix(w, r) {
				gasto = true
			}
		}
	}
	if !gasto {
		return false
	}
	for _, h := range palabrasHora {
		if strings.Contains(n, h) {
			return true
		}
	}
	return false
}

func (resRecursos) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	if p == nil || strings.TrimSpace(p.Codigo) != "" {
		return nucleo.Reconocimiento{}
	}
	t := textoDe(p)
	if len(t) > maxTextoRecursos || !pareceRecursos(t) {
		return nucleo.Reconocimiento{}
	}
	pr, err := leerRecursos(t)
	if err != nil {
		// a resources question I cannot read: say why instead of letting it be read as something else
		n := " " + nucleo.Normalizar(t) + " "
		if strings.Contains(n, " por ") && (strings.Contains(n, " dura") || strings.Contains(n, " se acaba") || strings.Contains(n, " se agota")) {
			return nucleo.Reconocimiento{Puntos: 0.6, Intencion: nucleo.IProblema, Lectura: "recursos que se gastan con el tiempo"}
		}
		return nucleo.Reconocimiento{}
	}
	rec := nucleo.Reconocimiento{Puntos: 0.85, Intencion: nucleo.IProblema, Lectura: "recursos que se gastan con el tiempo"}
	if pr.pregunta != "" {
		rec.Puntos = 0.96
	}
	return rec
}

func (resRecursos) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	pr, err := leerRecursos(textoDe(p))
	if err != nil {
		n.Sub(nucleo.PasoError, "No sé plantearlo: %s", mensaje(err)).Mal("")
		return nucleo.Respuesta{}, err
	}
	u := pr.base
	leer := n.Sub(nucleo.PasoEntender, "Leo %d %s que se %s", len(pr.recursos), plural(len(pr.recursos), "recurso", "recursos"), plural(len(pr.recursos), "gasta", "gastan"))
	var entendi []string
	for _, r := range pr.recursos {
		linea := r.cantidad(r.stock)
		if r.expr != nil {
			linea += fmt.Sprintf("; gasto %s por %s (t en %s)", r.tasaTexto, u.uno, u.varias)
		} else {
			linea += fmt.Sprintf("; gasto %s por %s", textoRatCorto(r.tasa), u.uno)
		}
		if r.minimo != nil {
			linea += "; sin bajar de " + textoRatCorto(r.minimo)
		}
		for _, rc := range r.recargas {
			linea += fmt.Sprintf("; a las %s %s añades %s", textoRat(rc.t), u.texto(rc.t), textoRatCorto(rc.cantidad))
		}
		leer.Detalle("%s", linea)
		entendi = append(entendi, linea)
	}
	if pr.textoPre != "" {
		leer.Detalle("Pregunta: %s", pr.textoPre)
	}
	leer.Info("")
	var r nucleo.Respuesta
	if pr.numerico {
		r, err = resolverNumerico(ctx, pr, n)
	} else {
		r, err = resolverExacto(ctx, pr, n)
	}
	if err != nil {
		return nucleo.Respuesta{}, err
	}
	r.Entendi = "Así lo entendí: " + strings.Join(entendi, " | ")
	if pr.textoPre != "" {
		r.Entendi += "; pregunta: " + pr.textoPre
	}
	r.Parrafos = append(r.Parrafos, pr.notas...)
	r.Justificacion = r.Plan
	r.Exito, r.Nivel = true, nucleo.NivelCalculo
	r.Sugerencias = []string{"¿Por qué?"}
	return r, nil
}

func resolverExacto(ctx context.Context, pr *problemaRecursos, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	u := pr.base
	var r nucleo.Respuesta
	cuenta := n.Sub(nucleo.PasoIntento, "Calculo cuándo se acaba cada uno (exacto, con fracciones)")
	tiempos := make([]agotado, len(pr.recursos))
	var tFin *big.Rat
	var primeros []*recursoR
	for i, x := range pr.recursos {
		a := agotarExacto(x, u)
		tiempos[i] = a
		etiqueta := x.etiqueta()
		pasos := strings.Join(a.pasos, "; ")
		if a.t == nil {
			r.Plan = append(r.Plan, fmt.Sprintf("%s: %s", etiqueta, pasos))
			cuenta.Detalle("%s: no se acaba", etiqueta)
			continue
		}
		r.Plan = append(r.Plan, fmt.Sprintf("%s: %s → se acaba a las %s %s", etiqueta, pasos, textoRatCorto(a.t), u.texto(a.t)))
		cuenta.Detalle("%s: %s %s", etiqueta, textoRatCorto(a.t), u.texto(a.t))
		switch {
		case tFin == nil || a.t.Cmp(tFin) < 0:
			tFin, primeros = a.t, []*recursoR{x}
		case a.t.Cmp(tFin) == 0:
			primeros = append(primeros, x)
		}
	}
	cuenta.Bien("")
	if ctx.Err() != nil {
		return nucleo.Respuesta{}, nucleo.ErrSinTiempo
	}
	if tFin == nil {
		comp := n.Sub(nucleo.PasoComprobar, "Compruebo: ningún gasto es positivo")
		for _, x := range pr.recursos {
			if x.tasa.Sign() > 0 {
				comp.Mal("")
				return nucleo.Respuesta{}, errSinComprobar
			}
		}
		comp.Bien("")
		r.Texto = "No se acaba nunca: no gastas nada (o lo que gastas se repone)."
		r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(1, 1, "Comprobado por cálculo", "ningún gasto es positivo")}
		r.Metodo = "exacto"
		return r, nil
	}
	// check 1: substitution in the closed formula
	comp := n.Sub(nucleo.PasoComprobar, "Compruebo sustituyendo t = %s en la fórmula de cada nivel", textoRat(tFin))
	if !comprobarExacto(pr, tFin, primeros) {
		comp.Mal("No cuadra")
		return nucleo.Respuesta{}, errSinComprobar
	}
	comp.Bien("Cuadra: a esa hora se acaba y antes no se acaba nada")
	// check 2: the same system integrated numerically must give the same instant
	num := n.Sub(nucleo.PasoComprobar, "Lo integro también paso a paso (Runge-Kutta 4 con paso adaptativo)")
	tf, _ := tFin.Float64()
	res, err := integrar(ctx, pr.recursos, horizonteDe(pr.recursos), 1e-9*escalaNiveles(pr.recursos))
	if err != nil {
		num.Mal("%s", mensaje(err))
		return nucleo.Respuesta{}, err
	}
	if res.nunca || math.Abs(res.t-tf) > 1e-7*math.Max(1, tf) {
		num.Mal("Sale %s y no %s", textoFloat(res.t), textoFloat(tf))
		return nucleo.Respuesta{}, errSinComprobar
	}
	num.Bien("Sale %s %s en %d pasos: coincide", textoFloat(res.t), u.varias, res.pasos)

	tTxt := textoRatCorto(tFin) + " " + u.texto(tFin)
	nombres := nombresDe(primeros)
	quedan := func(t *big.Rat, excepto []*recursoR) []string {
		var out []string
		for _, x := range pr.recursos {
			if contieneRecurso(excepto, x) {
				continue
			}
			out = append(out, x.cantidadNombre(nivelExacto(x, t, true)))
		}
		return out
	}
	r.Metodo = "exacto"
	switch {
	case pr.pregunta == "queda" && pr.en != nil:
		var partes []string
		for i, x := range pr.recursos {
			if a := tiempos[i]; a.t != nil && a.t.Cmp(pr.en) < 0 {
				partes = append(partes, fmt.Sprintf("%s ya se ha acabado (a las %s %s)", x.nombreO(), textoRatCorto(a.t), u.texto(a.t)))
				continue
			}
			partes = append(partes, x.cantidadNombre(nivelExacto(x, pr.en, true)))
		}
		r.Texto = fmt.Sprintf("A las %s %s: %s.", textoRatCorto(pr.en), u.texto(pr.en), unirY(partes))
		if tFin.Cmp(pr.en) < 0 {
			r.Parrafos = append(r.Parrafos, fmt.Sprintf("Ojo: a las %s ya se ha acabado %s.", tTxt, nombres))
		}
	case pr.cual != nil:
		i := indiceRecurso(pr.recursos, pr.cual)
		a := tiempos[i]
		if a.t == nil {
			r.Texto = fmt.Sprintf("%s no se acaba nunca.", mayusculaInicial(pr.cual.nombreO()))
		} else {
			r.Texto = fmt.Sprintf("%s dura %s %s.", mayusculaInicial(pr.cual.nombreO()), textoRatCorto(a.t), u.texto(a.t))
		}
		if len(pr.recursos) > 1 && !contieneRecurso(primeros, pr.cual) {
			r.Parrafos = append(r.Parrafos, fmt.Sprintf("Pero antes, a las %s, se acaba %s.", tTxt, nombres))
		}
	case pr.pregunta == "primero":
		if len(primeros) > 1 {
			r.Texto = fmt.Sprintf("Se acaban a la vez %s, a las %s.", nombres, tTxt)
		} else {
			r.Texto = fmt.Sprintf("Se acaba primero %s, a las %s.", nombres, tTxt)
		}
	default:
		switch {
		case len(pr.recursos) == 1:
			r.Texto = fmt.Sprintf("Dura %s.", tTxt)
		case len(primeros) > 1:
			r.Texto = fmt.Sprintf("Dura %s: %s se acaban a la vez.", tTxt, nombres)
		default:
			r.Texto = fmt.Sprintf("Dura %s: lo primero que se acaba es %s.", tTxt, nombres)
		}
	}
	if q := quedan(tFin, primeros); len(q) > 0 && pr.pregunta != "queda" {
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("Cuando se acaba %s (a las %s) quedan %s.", nombres, tTxt, unirY(q)))
	}
	for i := range pr.recursos {
		if tiempos[i].justo {
			r.Parrafos = append(r.Parrafos, fmt.Sprintf("%s llega a su mínimo justo cuando le añades más: sigue.", mayusculaInicial(pr.recursos[i].nombreO())))
		}
	}
	r.Parrafos = append(r.Parrafos,
		fmt.Sprintf("Compruebo: a las %s %s está justo en su mínimo y nada se ha acabado antes ✔", tTxt, nombres),
		fmt.Sprintf("Lo integro también paso a paso (Runge-Kutta 4 con paso adaptativo): sale %s %s ✔", textoFloat(res.t), u.varias))
	r.Tablas = []nucleo.Tabla{tablaRecursos(pr, tiempos, tFin)}
	r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(2, 2, "Comprobado por dos caminos",
		"sustituí el instante en la fórmula de cada nivel y lo integré paso a paso (Runge-Kutta 4): coinciden")}
	return r, nil
}

func escalaNiveles(rs []*recursoR) float64 {
	m := 1.0
	for _, r := range rs {
		f, _ := r.stock.Float64()
		m = math.Max(m, math.Abs(f))
	}
	return m
}

func resolverNumerico(ctx context.Context, pr *problemaRecursos, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	u := pr.base
	var r nucleo.Respuesta
	horizonte := horizonteDe(pr.recursos)
	tol := 1e-8 * escalaNiveles(pr.recursos)
	paso := n.Sub(nucleo.PasoIntento, "El gasto cambia con el tiempo: lo integro paso a paso (Runge-Kutta 4 con paso adaptativo)")
	a, err := integrar(ctx, pr.recursos, horizonte, tol)
	if err != nil {
		paso.Mal("%s", mensaje(err))
		return nucleo.Respuesta{}, err
	}
	paso.Bien("%d pasos aceptados", a.pasos)
	comp := n.Sub(nucleo.PasoComprobar, "Lo repito con una tolerancia 1000 veces menor")
	b, err := integrar(ctx, pr.recursos, horizonte, tol/1000)
	if err != nil {
		comp.Mal("%s", mensaje(err))
		return nucleo.Respuesta{}, err
	}
	if a.nunca != b.nunca || len(a.primeros) == 0 != (len(b.primeros) == 0) {
		comp.Mal("Las dos integraciones no dicen lo mismo")
		return nucleo.Respuesta{}, errSinComprobar
	}
	dif := math.Abs(a.t - b.t)
	errT := dif
	if !b.nunca && len(b.primeros) > 0 {
		if g, err := pr.recursos[b.primeros[0]].tasaEn(b.t); err == nil && g > 0 {
			errT = math.Max(errT, b.errNivel/g)
		}
	}
	if dif > 1e-5*math.Max(1, b.t) {
		comp.Mal("Difieren en %s", textoFloat(dif))
		return nucleo.Respuesta{}, errSinComprobar
	}
	comp.Bien("Coinciden (diferencia %s)", textoFloat(dif))
	r.Metodo = "aproximado"
	cota := "< " + textoFloat(math.Max(errT, 1e-9))
	if b.nunca {
		r.Texto = fmt.Sprintf("En las primeras %s %s no se acaba: el gasto baja tan deprisa que no llega a agotarlo.", textoFloat(b.horizonte), u.varias)
		r.Parrafos = append(r.Parrafos, "Es un cálculo aproximado (paso a paso), no una fórmula exacta.")
		for i, x := range pr.recursos {
			r.Parrafos = append(r.Parrafos, fmt.Sprintf("A las %s %s quedan ≈ %s de %s.", textoFloat(b.horizonte), u.varias, textoFloat(b.niveles[i]), strings.ToLower(x.etiqueta())))
		}
	} else {
		var primeros []*recursoR
		for _, i := range b.primeros {
			primeros = append(primeros, pr.recursos[i])
		}
		nombres := nombresDe(primeros)
		tTxt := "≈ " + textoFloat(b.t) + " " + u.varias
		switch {
		case pr.pregunta == "primero":
			r.Texto = fmt.Sprintf("Se acaba primero %s, a las %s (aproximado).", nombres, tTxt)
		case len(pr.recursos) == 1:
			r.Texto = fmt.Sprintf("Dura %s (aproximado).", tTxt)
		default:
			r.Texto = fmt.Sprintf("Dura %s (aproximado): lo primero que se acaba es %s.", tTxt, nombres)
		}
		var quedan []string
		for i, x := range pr.recursos {
			if !contieneRecurso(primeros, x) {
				quedan = append(quedan, fmt.Sprintf("≈ %s de %s", textoFloat(b.niveles[i]), strings.ToLower(x.etiqueta())))
			}
		}
		if len(quedan) > 0 {
			r.Parrafos = append(r.Parrafos, fmt.Sprintf("Cuando se acaba %s quedan %s.", nombres, unirY(quedan)))
		}
		r.Parrafos = append(r.Parrafos, fmt.Sprintf("Es aproximado: lo integré paso a paso (Runge-Kutta 4, %d pasos) y lo repetí con una tolerancia 1000 veces menor; el error estimado es %s %s.", b.pasos, cota, u.varias))
	}
	for _, x := range pr.recursos {
		if x.expr != nil {
			r.Plan = append(r.Plan, fmt.Sprintf("%s: nivel(t) = %s − ∫ %s dt", x.etiqueta(), textoRat(x.stock), strings.ReplaceAll(x.expr.String(), "*", "·")))
		} else {
			r.Plan = append(r.Plan, fmt.Sprintf("%s: nivel(t) = %s − %s·t", x.etiqueta(), textoRat(x.stock), textoRat(x.tasa)))
		}
	}
	r.Plan = append(r.Plan, fmt.Sprintf("Integro con Runge-Kutta 4: cada paso se hace entero y en dos mitades, y la diferencia decide el tamaño del paso (%d pasos)", b.pasos))
	r.Comprobaciones = []nucleo.Comprobacion{comprobacionCalculo(1, 1, "Comprobado repitiendo el cálculo",
		"integré dos veces, la segunda con una tolerancia 1000 veces menor: la diferencia es "+textoFloat(dif))}
	return r, nil
}

func tablaRecursos(pr *problemaRecursos, tiempos []agotado, tFin *big.Rat) nucleo.Tabla {
	u := pr.base
	tb := nucleo.Tabla{Titulo: "Recursos", Cabecera: []string{"Recurso", "Tengo", "Gasto por " + u.uno, "Se acaba a las", "Queda a las " + textoRat(tFin) + " " + u.texto(tFin)}}
	for i, x := range pr.recursos {
		fin := "nunca"
		if a := tiempos[i]; a.t != nil {
			fin = textoRatCorto(a.t) + " " + u.texto(a.t)
		}
		tb.Filas = append(tb.Filas, []string{x.etiqueta(), x.cantidad(x.stock), textoRatCorto(x.tasa), fin, x.cantidad(nivelExacto(x, tFin, true))})
	}
	return tb
}

func (r *recursoR) nombreO() string {
	if n := r.nombre(); n != "" {
		return n
	}
	return "lo que tienes"
}

// cantidadNombre: "40 metros de mecha".
func (r *recursoR) cantidadNombre(v *big.Rat) string {
	s := textoRatCorto(v)
	switch {
	case r.unidad != "" && r.cosa != "":
		return s + " " + r.unidad + " de " + r.cosa
	case r.unidad != "":
		return s + " " + r.unidad
	case r.cosa != "":
		return s + " de " + r.cosa
	}
	return s
}

func nombresDe(rs []*recursoR) string {
	var ps []string
	for _, r := range rs {
		ps = append(ps, r.nombreO())
	}
	return unirY(ps)
}

func unirY(ps []string) string {
	switch len(ps) {
	case 0:
		return ""
	case 1:
		return ps[0]
	}
	return strings.Join(ps[:len(ps)-1], ", ") + " y " + ps[len(ps)-1]
}

func contieneRecurso(rs []*recursoR, x *recursoR) bool {
	for _, r := range rs {
		if r == x {
			return true
		}
	}
	return false
}

func indiceRecurso(rs []*recursoR, x *recursoR) int {
	for i, r := range rs {
		if r == x {
			return i
		}
	}
	return 0
}
