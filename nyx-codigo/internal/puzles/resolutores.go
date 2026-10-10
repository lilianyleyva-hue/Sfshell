package puzles

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// Resolutores returns the puzzle resolvers: "puzles.sudoku", "puzles.reinas", "puzles.cripto",
// "puzles.cebra", "puzles.colorear", "puzles.plan", "puzles.conteo", "puzles.probabilidad",
// "puzles.secuencia". lem may be nil (the readers here use their own tokenizer).
func Resolutores(lem nucleo.Lematizador) []nucleo.Resolutor {
	return []nucleo.Resolutor{
		resSudoku{}, resReinas{}, resCripto{}, resCebra{}, resColorear{},
		resPlan{}, resConteo{}, resProbabilidad{}, resSecuencia{},
	}
}

func respuestaCalculo(texto, metodo, comprobado string) nucleo.Respuesta {
	return nucleo.Respuesta{
		Texto:          texto,
		Metodo:         metodo,
		Nivel:          nucleo.NivelCalculo,
		Exito:          true,
		Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelCalculo, Pasan: 1, Total: 1, Texto: comprobado}},
	}
}

func normal(p *nucleo.Pregunta) string { return " " + nucleo.Normalizar(p.Texto) + " " }

func contiene(s string, fs ...string) bool {
	for _, f := range fs {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}

// --- sudoku

type resSudoku struct{}

func (resSudoku) Nombre() string { return "puzles.sudoku" }

func (resSudoku) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPuzle, Lectura: "sudoku"}
	if _, err := LeerSudoku(p.Texto); err == nil {
		rec.Puntos = 0.95
	} else if strings.Contains(normal(p), "sudoku") {
		rec.Puntos = 0.4
	}
	return rec
}

func (resSudoku) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	leer := n.Sub(nucleo.PasoEntender, "Leo el sudoku")
	t, err := LeerSudoku(p.Texto)
	if err != nil {
		leer.Mal("No encuentro las 9 filas")
		return nucleo.Respuesta{}, err
	}
	dados := 0
	for _, f := range t {
		for _, v := range f {
			if v != 0 {
				dados++
			}
		}
	}
	leer.Bien("Sudoku con %d casillas dadas", dados)
	sol, unica, est, det, err := SudokuDetallado(ctx, t, n)
	if err != nil && !(errors.Is(err, nucleo.ErrSinTiempo) && det.Soluciones > 0) {
		return nucleo.Respuesta{}, err
	}
	var texto string
	switch {
	case det.SoloTecnicas:
		texto = fmt.Sprintf("Resuelto solo con técnicas (único candidato, único lugar, par apuntador), sin probar caminos: %d pasos.", len(det.Pasos))
	case det.PorTecnicas > 0:
		texto = fmt.Sprintf("Con técnicas relleno %d casillas; después probé %d caminos (%d retrocesos).", det.PorTecnicas, est.Nodos, est.Retrocesos)
	default:
		texto = fmt.Sprintf("Probé %d caminos (%d retrocesos).", est.Nodos, est.Retrocesos)
	}
	if unica {
		texto += " La solución es única."
	} else {
		texto += " Tiene más de una solución; te enseño una."
	}
	r := respuestaCalculo(texto, "csp", "Comprobé las 27 filas, columnas y bloques, y que respeta las casillas dadas")
	r.Comprobaciones[0].Pasan, r.Comprobaciones[0].Total = 27, 27
	tb := &nucleo.Tablero{}
	for i := 0; i < 9; i++ {
		tb.Celdas = append(tb.Celdas, append([]int(nil), sol[i][:]...))
		var fijas []bool
		for j := 0; j < 9; j++ {
			fijas = append(fijas, t[i][j] != 0)
		}
		tb.Fijas = append(tb.Fijas, fijas)
	}
	r.Tablero = tb
	for i, paso := range det.Pasos {
		if i >= maxPasosDetallados {
			r.Justificacion = append(r.Justificacion, fmt.Sprintf("… y %d pasos más.", len(det.Pasos)-maxPasosDetallados))
			break
		}
		r.Justificacion = append(r.Justificacion, paso.Texto)
	}
	return r, nil
}

// --- queens

type resReinas struct{}

func (resReinas) Nombre() string { return "puzles.reinas" }

func (resReinas) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPuzle, Lectura: "problema de las reinas"}
	if strings.Contains(normal(p), " reinas ") || strings.Contains(normal(p), " reina ") {
		rec.Puntos = 0.9
	}
	return rec
}

func (resReinas) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	l := nuevaLectura(p.Texto)
	nR, ok := numeroAntes(l, "reinas", "reina")
	if !ok {
		if ns := numeros(l.ps); len(ns) > 0 {
			nR = ns[0]
		} else {
			nR = 8
		}
	}
	todas := l.tieneRaiz("cuant", "todas", "numero") && !l.tiene("una solucion", "coloca", "colocar", "pon", "dame una")
	paso := n.Sub(nucleo.PasoIntento, "%d reinas en un tablero de %d×%d", nR, nR, nR)
	cuenta, una, err := Reinas(ctx, int(nR), todas)
	if err != nil {
		paso.Mal("No lo conseguí")
		return nucleo.Respuesta{}, err
	}
	var r nucleo.Respuesta
	if todas {
		paso.Bien("%d soluciones", cuenta)
		comp := "Conté con búsqueda por máscaras de bits y coincide con la cifra conocida"
		if nR <= 8 {
			comp += "; lo repetí con el resolvedor de restricciones"
		}
		r = respuestaCalculo(fmt.Sprintf("Hay %d soluciones para %d reinas.", cuenta, nR), "busqueda", comp)
	} else {
		if una == nil {
			paso.Mal("No hay solución")
			return respuestaCalculo(fmt.Sprintf("No hay ninguna forma de colocar %d reinas sin que se ataquen.", nR), "busqueda", "Recorrí todas las posibilidades"), nil
		}
		paso.Bien("Encontré una colocación")
		r = respuestaCalculo(fmt.Sprintf("Una forma de colocar %d reinas sin que se ataquen: %s.", nR, textoReinas(una)), "busqueda", "Comprobé que ninguna reina ataca a otra")
	}
	if una != nil && len(una) <= 30 {
		tb := &nucleo.Tablero{}
		for f := range una {
			fila := make([]int, len(una))
			fila[una[f]] = 1
			tb.Celdas = append(tb.Celdas, fila)
			tb.Fijas = append(tb.Fijas, make([]bool, len(una)))
		}
		r.Tablero = tb
		if todas {
			r.Parrafos = append(r.Parrafos, "Por ejemplo: "+textoReinas(una)+".")
		}
	}
	return r, nil
}

func textoReinas(cols []int) string {
	var partes []string
	for f, c := range cols {
		if f >= 12 {
			partes = append(partes, "…")
			break
		}
		partes = append(partes, fmt.Sprintf("%c%d", 'a'+rune(c%26), f+1))
	}
	return strings.Join(partes, ", ")
}

// --- cryptarithms

type resCripto struct{}

var reCripto = regexp.MustCompile(`[\p{L}]+(\s*[+\-−]\s*[\p{L}]+)+\s*=\s*[\p{L}]+`)

func (resCripto) Nombre() string { return "puzles.cripto" }

func (resCripto) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPuzle, Lectura: "suma de letras (criptoaritmética)"}
	if m := reCripto.FindString(p.Texto); m != "" && strings.ToUpper(m) == m {
		rec.Puntos = 0.95
	} else if m != "" {
		rec.Puntos = 0.5
	}
	return rec
}

func (resCripto) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	expr := reCripto.FindString(p.Texto)
	if expr == "" {
		return nucleo.Respuesta{}, fmt.Errorf("puzles: no encuentro una suma de letras como SEND+MORE=MONEY: %w", nucleo.ErrNoEntiendo)
	}
	paso := n.Sub(nucleo.PasoIntento, "Busco cifras para %s columna a columna, con las llevadas", strings.ToUpper(expr))
	sols, nodos, err := CriptoTodas(ctx, expr)
	if err != nil {
		paso.Mal("No lo conseguí")
		return nucleo.Respuesta{}, err
	}
	s := sols[0]
	paso.Bien("Probé %d asignaciones", nodos)
	texto := EscribirCripto(expr, s)
	if len(sols) == 1 {
		texto += " (es la única solución)."
	} else {
		texto += " (hay más de una solución; esta es una)."
	}
	r := respuestaCalculo(texto, "busqueda", "Rehice la suma con los números: cuadra, cada letra es una cifra distinta y ninguna palabra empieza por 0")
	r.Parrafos = []string{"Letras: " + TextoAsignacion(s) + "."}
	return r, nil
}

// --- zebra

type resCebra struct{}

func (resCebra) Nombre() string { return "puzles.cebra" }

func (resCebra) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPuzle, Lectura: "acertijo de la cebra (casas y pistas)"}
	t := normal(p)
	casas := strings.Count(t, " casa")
	if strings.Contains(t, "cebra") && casas >= 2 {
		rec.Puntos = 0.9
	} else if casas >= 4 && strings.Count(t, ".") >= 4 {
		rec.Puntos = 0.6
	}
	return rec
}

func partirPistas(texto string) []string {
	var out []string
	var sb strings.Builder
	for _, r := range texto {
		switch r {
		case '\n', '.', ';':
			out = append(out, sb.String())
			sb.Reset()
		case '?':
			sb.WriteRune(r)
			out = append(out, sb.String())
			sb.Reset()
		case '¿':
			out = append(out, sb.String())
			sb.Reset()
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}
	out = append(out, sb.String())
	var limpio []string
	for _, s := range out {
		s = strings.TrimSpace(s)
		s = strings.TrimLeftFunc(s, func(r rune) bool { return unicode.IsDigit(r) || r == ')' || r == '-' || r == ' ' })
		if s != "" {
			limpio = append(limpio, s)
		}
	}
	return limpio
}

func (resCebra) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	pistas := partirPistas(p.Texto)
	paso := n.Sub(nucleo.PasoEntender, "Leo %d pistas", len(pistas))
	res, err := ResolverCebra(ctx, pistas)
	if err != nil {
		paso.Mal("No lo conseguí")
		return nucleo.Respuesta{}, err
	}
	paso.Bien("%d casas, %d restricciones", res.Casas, res.Restricciones)
	texto := "Esta es la solución."
	if len(res.Respuestas) > 0 {
		var frases []string
		for _, s := range res.Respuestas {
			frases = append(frases, s)
		}
		texto = mayus(strings.Join(frases, "; ")) + "."
	}
	if !res.Unica {
		texto += " Ojo: con estas pistas hay más de una solución."
	}
	r := respuestaCalculo(texto, "csp", "Comprobé cada pista con la solución, y busqué una segunda solución")
	r.Tablas = []nucleo.Tabla{res.Tabla}
	if len(res.NoEntendidas) > 0 {
		r.Parrafos = append(r.Parrafos, "No entendí estas pistas, así que no las usé: «"+strings.Join(res.NoEntendidas, "», «")+"».")
	}
	return r, nil
}

func mayus(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

// --- map colouring

type resColorear struct{}

func (resColorear) Nombre() string { return "puzles.colorear" }

var reArista = regexp.MustCompile(`([\p{L}\d_]+)\s*(?:-|–|—|<->)\s*([\p{L}\d_]+)`)

func (resColorear) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPuzle, Lectura: "colorear un mapa o un grafo"}
	t := normal(p)
	if contiene(t, "colorea", "colorear", "coloreo", "pinta el mapa", "pintar el mapa") && (reArista.MatchString(p.Texto) || strings.Contains(p.Texto, ":")) {
		rec.Puntos = 0.85
	}
	return rec
}

// leerAristas reads "A-B, B-C" pairs or "A: B, C" adjacency lines.
func leerAristas(texto string) [][2]string {
	var out [][2]string
	visto := map[[2]string]bool{}
	agregar := func(a, b string) {
		a, b = strings.TrimSpace(a), strings.TrimSpace(b)
		if a == "" || b == "" || a == b {
			return
		}
		k := [2]string{a, b}
		if b < a {
			k = [2]string{b, a}
		}
		if !visto[k] {
			visto[k] = true
			out = append(out, [2]string{a, b})
		}
	}
	for _, m := range reArista.FindAllStringSubmatch(texto, -1) {
		agregar(m[1], m[2])
	}
	if len(out) > 0 {
		return out
	}
	for _, linea := range strings.Split(texto, "\n") {
		i := strings.Index(linea, ":")
		if i <= 0 {
			continue
		}
		a := strings.TrimSpace(linea[:i])
		if strings.Contains(a, " ") {
			continue
		}
		for _, b := range strings.FieldsFunc(linea[i+1:], func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
			if b != "y" && b != "e" {
				agregar(a, strings.Trim(b, "."))
			}
		}
	}
	return out
}

func (resColorear) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	aristas := leerAristas(p.Texto)
	if len(aristas) == 0 {
		return nucleo.Respuesta{}, fmt.Errorf("puzles: escribe las fronteras como «A-B, B-C»: %w", nucleo.ErrNoEntiendo)
	}
	l := nuevaLectura(p.Texto)
	k, conK := numeroAntes(l, "colores", "color")
	paso := n.Sub(nucleo.PasoIntento, "Coloreo %d fronteras", len(aristas))
	var colores map[string]int
	var err error
	if conK {
		colores, err = Colorear(ctx, aristas, int(k))
	} else {
		var kk int
		kk, colores, err = NumeroCromatico(ctx, aristas, 10)
		k = int64(kk)
	}
	if err != nil {
		if errors.Is(err, ErrSinColoreo) {
			paso.Mal("Imposible")
			return respuestaCalculo(fmt.Sprintf("No se puede colorear con %d colores sin que dos vecinos compartan color.", k), "csp", "Recorrí todas las posibilidades con el resolvedor de restricciones"), nil
		}
		paso.Mal("No lo conseguí")
		return nucleo.Respuesta{}, err
	}
	paso.Bien("Coloreado con %d colores", k)
	var nodos []string
	vistos := map[string]bool{}
	for _, a := range aristas {
		for _, x := range a {
			if !vistos[x] {
				vistos[x] = true
				nodos = append(nodos, x)
			}
		}
	}
	tb := nucleo.Tabla{Titulo: "Colores", Cabecera: []string{"Región", "Color"}}
	var partes []string
	for _, x := range nodos {
		c := colores[x]
		nombre := fmt.Sprintf("color %d", c+1)
		if c < len(NombresColores) {
			nombre = NombresColores[c]
		}
		partes = append(partes, x+" "+nombre)
		tb.Filas = append(tb.Filas, []string{x, nombre})
	}
	texto := fmt.Sprintf("Se puede con %d colores: %s.", k, strings.Join(partes, ", "))
	if !conK {
		texto = fmt.Sprintf("Hacen falta %d colores: %s.", k, strings.Join(partes, ", "))
	}
	r := respuestaCalculo(texto, "csp", fmt.Sprintf("Comprobé las %d fronteras: los vecinos tienen colores distintos", len(aristas)))
	r.Tablas = []nucleo.Tabla{tb}
	return r, nil
}

// --- planning

type resPlan struct{}

func (resPlan) Nombre() string { return "puzles.plan" }

func (resPlan) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IPlan, Lectura: "planificación"}
	t := normal(p)
	switch {
	case contiene(t, " jarra", " cubo de ", " cubos de ", " garrafa", " recipientes de "):
		rec.Puntos, rec.Lectura = 0.9, "jarras de agua"
	case contiene(t, " lobo") && contiene(t, " cabra"):
		rec.Puntos, rec.Lectura = 0.9, "el lobo, la cabra y la col"
	case contiene(t, " misioner") && contiene(t, " canibal"):
		rec.Puntos, rec.Lectura = 0.9, "misioneros y caníbales"
	case contiene(t, " hanoi", " hanói"):
		rec.Puntos, rec.Lectura = 0.9, "torres de Hanói"
	case contiene(t, " puzzle", " puzle de 8", " 8-puzzle", " ocho piezas", " rompecabezas deslizante"):
		rec.Puntos, rec.Lectura = 0.85, "el 8-puzzle"
	case contiene(t, " laberinto") || (strings.Contains(p.Texto, "#") && strings.Contains(p.Texto, "S") && strings.Contains(p.Texto, "E") && strings.Count(p.Texto, "\n") >= 2):
		rec.Puntos, rec.Lectura = 0.85, "laberinto"
	}
	return rec
}

func (rp resPlan) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	t := normal(p)
	l := nuevaLectura(p.Texto)
	nums := numeros(l.ps)
	switch {
	case contiene(t, " hanoi", " hanói"):
		d, ok := numeroAntes(l, "discos", "disco", "anillos", "piezas")
		if !ok {
			if len(nums) == 0 {
				return nucleo.Respuesta{}, fmt.Errorf("puzles: ¿con cuántos discos?: %w", nucleo.ErrNoEntiendo)
			}
			d = nums[0]
		}
		return resolverHanoi(int(d), n)
	case contiene(t, " jarra", " cubo de ", " cubos de ", " garrafa", " recipientes de "):
		if len(nums) < 2 {
			return nucleo.Respuesta{}, fmt.Errorf("puzles: dime las capacidades y cuánto quieres sacar: %w", nucleo.ErrNoEntiendo)
		}
		// the goal is the number after "saco/medir/obtener…", or the last number
		var idxs []int
		for k, w := range l.ps {
			if w.esNum && w.norma != "un" && w.norma != "una" && w.norma != "uno" {
				idxs = append(idxs, k)
			}
		}
		metaIdx := idxs[len(idxs)-1]
		for k, w := range l.ps {
			if contiene(" "+w.norma+" ", " saco ", " sacar ", " medir ", " mido ", " obtener ", " conseguir ", " consigo ", " obtengo ", " exactamente ", " quiero ", " tener ") {
				if _, j2, ok := l.numeroTras(k); ok {
					metaIdx = j2
				}
			}
		}
		meta := l.ps[metaIdx].num
		var caps []int
		for _, k := range idxs {
			if k != metaIdx {
				caps = append(caps, int(l.ps[k].num))
			}
		}
		if len(caps) == 0 || len(caps) > 4 {
			return nucleo.Respuesta{}, fmt.Errorf("puzles: no entiendo las capacidades de las jarras: %w", nucleo.ErrNoEntiendo)
		}
		for _, c := range caps {
			if c <= 0 || c > 1000 {
				return nucleo.Respuesta{}, fmt.Errorf("puzles: capacidad %d fuera de rango: %w", c, nucleo.ErrNoEntiendo)
			}
		}
		lect := fmt.Sprintf("jarras de %s litros; quiero %d litros en una de ellas", unirInts(caps), meta)
		return resolverPlan(ctx, Jarras(caps, int(meta)), lect, "bfs", n)
	case contiene(t, " lobo") && contiene(t, " cabra"):
		return resolverPlan(ctx, Rio(), "el granjero debe cruzar el río con el lobo, la cabra y la col; la barca solo lleva una cosa", "bfs", n)
	case contiene(t, " misioner") && contiene(t, " canibal"):
		m, c, b := 3, 3, 2
		if v, ok := numeroAntes(l, "misioneros"); ok {
			m = int(v)
		}
		if v, ok := numeroAntes(l, "canibales"); ok {
			c = int(v)
		}
		for i, w := range l.ps {
			if strings.HasPrefix(w.norma, "barca") || strings.HasPrefix(w.norma, "bote") {
				if v, _, ok := l.numeroTras(i); ok {
					b = int(v)
				}
			}
		}
		if m < 0 || c < 0 || m > 50 || c > 50 || b < 1 || b > 10 {
			return nucleo.Respuesta{}, fmt.Errorf("puzles: números fuera de rango: %w", nucleo.ErrNoEntiendo)
		}
		lect := fmt.Sprintf("%d misioneros y %d caníbales, barca de %d", m, c, b)
		return resolverPlan(ctx, Misioneros(m, c, b), lect, "bfs", n)
	case contiene(t, " laberinto") || strings.Contains(p.Texto, "#"):
		var lineas []string
		for _, linea := range strings.Split(p.Texto, "\n") {
			if strings.ContainsAny(linea, "#SE") && !strings.Contains(strings.ToLower(linea), "laberinto") {
				lineas = append(lineas, linea)
			}
		}
		lab, err := Laberinto(lineas)
		if err != nil {
			return nucleo.Respuesta{}, err
		}
		return resolverPlan(ctx, lab, "un laberinto de "+strconv.Itoa(len(lineas))+" filas", "a*", n)
	default:
		var fichas []int
		for _, r := range p.Texto {
			switch {
			case r >= '0' && r <= '8':
				fichas = append(fichas, int(r-'0'))
			case r == '_':
				fichas = append(fichas, 0)
			}
		}
		if len(fichas) > 9 {
			fichas = fichas[len(fichas)-9:]
		}
		var ini [9]int
		if len(fichas) != 9 {
			return nucleo.Respuesta{}, fmt.Errorf("puzles: escribe las 9 casillas del puzzle (0 o _ para el hueco): %w", nucleo.ErrNoEntiendo)
		}
		copy(ini[:], fichas)
		pp, resoluble := Puzzle8(ini)
		if pp == nil {
			return nucleo.Respuesta{}, fmt.Errorf("puzles: las casillas deben ser los números del 0 al 8 sin repetir: %w", nucleo.ErrNoEntiendo)
		}
		if !resoluble {
			n.Sub(nucleo.PasoComprobar, "Cuento las inversiones: son impares").Mal("")
			return respuestaCalculo("Ese puzzle no tiene solución: el número de inversiones es impar, y cada movimiento conserva su paridad.", "paridad", "Conté las inversiones de las fichas"), nil
		}
		return resolverPlan(ctx, pp, "8-puzzle "+pp.Inicial().String(), "ida*", n)
	}
}

func unirInts(xs []int) string {
	var ps []string
	for _, x := range xs {
		ps = append(ps, strconv.Itoa(x))
	}
	if len(ps) <= 1 {
		return strings.Join(ps, "")
	}
	return strings.Join(ps[:len(ps)-1], ", ") + " y " + ps[len(ps)-1]
}

func resolverPlan(ctx context.Context, pp ProblemaPlan, lectura, metodo string, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	n.Sub(nucleo.PasoEntender, "Así lo entiendo: %s", lectura).Bien("")
	busca := n.Sub(nucleo.PasoIntento, "Busco el plan más corto (%s)", metodo)
	var plan Plan
	var err error
	switch metodo {
	case "a*":
		plan, err = AEstrella(ctx, pp, MaxEstados)
	case "ida*":
		plan, err = IDAEstrella(ctx, pp, MaxEstados)
	default:
		plan, err = BFS(ctx, pp, MaxEstados)
	}
	if err != nil {
		busca.Mal("Exploré %d estados sin éxito", plan.Explorados)
		if errors.Is(err, ErrSinPlan) {
			r := respuestaCalculo(fmt.Sprintf("No se puede: exploré los %d estados posibles y ninguno llega a la meta.", plan.Explorados), metodo, "Recorrí todos los estados alcanzables")
			r.Entendi = "Así lo entendí: " + lectura
			return r, nil
		}
		return nucleo.Respuesta{}, err
	}
	busca.Bien("Plan de %d pasos (exploré %d estados)", len(plan.Pasos), plan.Explorados)
	comp := n.Sub(nucleo.PasoComprobar, "Vuelvo a simular el plan paso a paso")
	if !Simular(pp, plan) {
		comp.Mal("La simulación no llega a la meta")
		return nucleo.Respuesta{}, errors.New("puzles: el plan no pasa la simulación (error interno)")
	}
	comp.Bien("La simulación llega a la meta")
	texto := fmt.Sprintf("Se consigue en %d pasos, el mínimo posible (exploré %d estados).", len(plan.Pasos), plan.Explorados)
	if len(plan.Pasos) == 0 {
		texto = "Ya está resuelto: no hace falta ningún paso."
	}
	r := respuestaCalculo(texto, metodo, "Volví a simular el plan paso a paso y llega a la meta")
	for i, paso := range plan.Pasos {
		r.Plan = append(r.Plan, fmt.Sprintf("%d. %s → %s", i+1, paso, plan.Estados[i+1]))
	}
	r.Entendi = "Así lo entendí: " + lectura
	return r, nil
}

func resolverHanoi(d int, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	if d <= 0 || d > 64 {
		return nucleo.Respuesta{}, fmt.Errorf("puzles: el número de discos debe estar entre 1 y 64: %w", nucleo.ErrNoEntiendo)
	}
	movs, total := Hanoi(d)
	prueba := "Para mover n discos hay que mover n−1 discos a la torre auxiliar, el disco mayor a la final y otra vez los n−1 encima: T(n) = 2·T(n−1) + 1 con T(1) = 1, así que T(n) = 2ⁿ − 1."
	if movs == nil {
		r := respuestaCalculo(fmt.Sprintf("Hacen falta %s movimientos (2^%d − 1); son demasiados para listarlos.", total, d), "formula", "Por la recurrencia T(n) = 2·T(n−1) + 1, comprobada con la lista completa hasta 10 discos")
		r.Justificacion = []string{prueba}
		return r, nil
	}
	c := n.Sub(nucleo.PasoComprobar, "Simulo los %d movimientos", len(movs))
	if err := ComprobarHanoi(d, movs); err != nil || int64(len(movs)) != total.Int64() {
		c.Mal("La simulación falla")
		return nucleo.Respuesta{}, fmt.Errorf("puzles: la solución de Hanói no se sostiene (error interno): %v", err)
	}
	c.Bien("Todos los movimientos son legales y los discos acaban en C")
	r := respuestaCalculo(fmt.Sprintf("Hacen falta %d movimientos (2^%d − 1), de la torre A a la C.", len(movs), d), "recursion", "Simulé todos los movimientos: son legales y los discos acaban en la torre C")
	for i, m := range movs {
		r.Plan = append(r.Plan, fmt.Sprintf("%d. %s", i+1, m))
	}
	r.Justificacion = []string{prueba}
	return r, nil
}

// --- counting

type resConteo struct{}

func (resConteo) Nombre() string { return "puzles.conteo" }

func (resConteo) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IConteo, Lectura: "contar"}
	t := normal(p)
	if strings.Contains(t, "probabilidad") {
		rec.Puntos = 0.2
		return rec
	}
	switch {
	case contiene(t, " anagrama", "cuantas formas", "cuantas maneras", "de cuantos modos", "cuantas combinaciones", "cuantas permutaciones", "cuantas variaciones", "cuantos caminos", "cuantas contraseñas", "cuantos codigos", "cuantas claves", "cuantos pin", "cuantos grupos", "cuantos comites", "cuantos equipos"):
		rec.Puntos = 0.85
	case contiene(t, " cuant") && contiene(t, " del ", " entre ", " desde ") && contiene(t, " divisible", " multiplo", " pares ", " impares ", " primos ", " contienen ", " tienen el "):
		rec.Puntos = 0.85
	case contiene(t, "cuantos numeros de", "cuantas palabras de", " escalones"):
		rec.Puntos = 0.7
	}
	if contiene(t, " reina", " sudoku", " siguiente ") {
		rec.Puntos = min(rec.Puntos, 0.3)
	}
	return rec
}

// modeloReducido shrinks a model so that brute force can check the formula on a smaller case.
func modeloReducido(m Modelo) (Modelo, bool) {
	r := m
	switch {
	case len(m.Escalones) > 0:
		r.Longitud = min(m.Longitud, 18)
	case m.esRango():
		r.Rango = [2]int64{m.Rango[0], m.Rango[0] + 99_999}
	case len(m.Multiconj) > 0:
		r.Multiconj = nil
		total := 0
		for _, c := range m.Multiconj {
			c = min(c, 3)
			if total+c > 10 {
				break
			}
			r.Multiconj = append(r.Multiconj, c)
			total += c
		}
		r.Alfabeto = m.Alfabeto[:len(r.Multiconj)]
		r.Longitud = total
	default:
		n := min(len(m.Alfabeto), 9)
		r.Alfabeto = m.Alfabeto[:n]
		r.Longitud = min(m.Longitud, 5)
		if !m.Repeticion {
			r.Longitud = min(r.Longitud, n)
		}
		if m.Circular {
			r.Longitud = n
		}
		var cs []int
		for _, c := range m.Contiene {
			if c < n {
				cs = append(cs, c)
			}
		}
		r.Contiene = cs
	}
	esp := Espacio(r)
	return r, esp != nil && esp.Cmp(big.NewInt(LimiteFuerza)) <= 0
}

func (resConteo) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	ent := n.Sub(nucleo.PasoEntender, "Busco qué hay que contar")
	m, lect, err := LeerConteo(p)
	if err != nil {
		ent.Mal("No sé plantearlo")
		return nucleo.Respuesta{}, err
	}
	ent.Bien("Así lo entiendo: %s", lect)
	valor, expl, hayFormula := Formula(m)
	if hayFormula {
		n.Sub(nucleo.PasoIntento, "Fórmula: %s", expl).Bien("")
	}
	esp := Espacio(m)
	var cuenta *big.Int
	comprobado := ""
	if esp != nil && esp.Cmp(big.NewInt(LimiteFuerza)) <= 0 {
		c := n.Sub(nucleo.PasoComprobar, "Cuento uno a uno (%s casos)", esp)
		cuenta, err = ContarFuerza(ctx, m, LimiteFuerza)
		if err != nil {
			c.Mal("No terminé el recuento")
			if !hayFormula {
				return nucleo.Respuesta{}, err
			}
		} else {
			c.Bien("El recuento da %s", cuenta)
		}
	}
	var texto string
	var res *big.Int
	switch {
	case cuenta != nil && hayFormula && cuenta.Cmp(valor) == 0:
		res = cuenta
		texto = fmt.Sprintf("%s = %s; lo conté uno a uno: coincide.", res, expl)
		comprobado = "La fórmula y el recuento uno a uno coinciden"
	case cuenta != nil && hayFormula:
		res = cuenta
		texto = fmt.Sprintf("Hay %s. (La fórmula daba %s, pero la fórmula y el recuento no coinciden: me quedo con el recuento.)", cuenta, valor)
		n.Nota("la fórmula y el recuento no coinciden: me quedo con el recuento")
		comprobado = "Contado uno a uno"
	case cuenta != nil:
		res = cuenta
		texto = fmt.Sprintf("Hay %s: lo conté uno a uno.", cuenta)
		comprobado = "Contado uno a uno"
	case hayFormula:
		res = valor
		red, ok := modeloReducido(m)
		if !ok {
			return nucleo.Respuesta{}, errors.New("puzles: no puedo comprobar la fórmula en un caso pequeño")
		}
		vr, explR, okR := Formula(red)
		cr, errR := ContarFuerza(ctx, red, LimiteFuerza)
		if !okR || errR != nil || vr.Cmp(cr) != 0 {
			return nucleo.Respuesta{}, errors.New("puzles: la fórmula no coincide con el recuento en un caso pequeño; no me fío")
		}
		texto = fmt.Sprintf("%s = %s. Son demasiados casos para contarlos uno a uno, pero comprobé la misma fórmula en un caso más pequeño (%s) y coincide con el recuento.", res, expl, explR)
		comprobado = "Fórmula comprobada con un recuento en un caso más pequeño"
	default:
		return nucleo.Respuesta{}, fmt.Errorf("puzles: no tengo fórmula y son demasiados casos para contar: %w", nucleo.ErrNoSoportado)
	}
	r := respuestaCalculo(texto, "conteo", comprobado)
	r.Entendi = "Así lo entendí: " + lect
	return r, nil
}

// --- probability

type resProbabilidad struct{}

func (resProbabilidad) Nombre() string { return "puzles.probabilidad" }

func (resProbabilidad) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.IConteo, Lectura: "probabilidad"}
	t := normal(p)
	if strings.Contains(t, "probabilidad") {
		rec.Puntos = 0.6
		if contiene(t, "moneda", "dado") {
			rec.Puntos = 0.9
		}
	}
	return rec
}

func (resProbabilidad) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	ent := n.Sub(nucleo.PasoEntender, "Busco los casos posibles y los favorables")
	m, exito, lect, formula, err := LeerProbabilidad(p)
	if err != nil {
		ent.Mal("No sé plantearlo")
		return nucleo.Respuesta{}, err
	}
	ent.Bien("Así lo entiendo: %s", lect)
	c := n.Sub(nucleo.PasoComprobar, "Cuento los casos uno a uno (%s casos)", Espacio(m))
	prob, err := Probabilidad(ctx, m, exito)
	if err != nil {
		c.Mal("No pude contarlos")
		return nucleo.Respuesta{}, err
	}
	c.Bien("Probabilidad %s", prob.RatString())
	texto := fmt.Sprintf("La probabilidad es %s", prob.RatString())
	if !prob.IsInt() {
		f, _ := prob.Float64()
		texto += fmt.Sprintf(" (≈ %s)", strings.Replace(strconv.FormatFloat(f, 'f', 4, 64), ".", ",", 1))
	}
	comprobado := "Contados uno a uno los casos posibles y los favorables"
	switch {
	case formula != nil && formula.Cmp(prob) == 0:
		texto += "; la fórmula da lo mismo."
		comprobado = "El recuento uno a uno y la fórmula coinciden"
	case formula != nil:
		texto += fmt.Sprintf(". (La fórmula daba %s, pero la fórmula y el recuento no coinciden: me quedo con el recuento.)", formula.RatString())
		n.Nota("la fórmula y el recuento no coinciden: me quedo con el recuento")
	default:
		texto += "."
	}
	r := respuestaCalculo(texto, "conteo", comprobado)
	r.Entendi = "Así lo entendí: " + lect
	return r, nil
}

// --- sequences

type resSecuencia struct{}

func (resSecuencia) Nombre() string { return "puzles.secuencia" }

var (
	reNumeroSec = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:/\d+)?`)
	reNumeroDec = regexp.MustCompile(`-?\d+(?:[.,]\d+)?(?:/\d+)?`)
	reCuantos   = regexp.MustCompile(`(?i)(?:los|las)?\s*(\d+)\s+(?:siguientes|próximos|proximos|términos|terminos|números|numeros)`)
)

// LeerSecuencia extracts the numbers of a sequence question and how many terms are asked
// ("los 3 siguientes"). Commas followed by a space separate terms; otherwise commas separate too.
func LeerSecuencia(texto string) ([]*big.Rat, int) {
	cuantos := 1
	if m := reCuantos.FindStringSubmatchIndex(texto); m != nil {
		if v, err := strconv.Atoi(texto[m[2]:m[3]]); err == nil && v > 0 && v <= 20 {
			cuantos = v
			texto = texto[:m[2]] + strings.Repeat(" ", m[3]-m[2]) + texto[m[3]:]
		}
	}
	re := reNumeroSec
	if strings.Contains(texto, ", ") || strings.Contains(texto, "; ") {
		re = reNumeroDec
	}
	var out []*big.Rat
	for _, s := range re.FindAllString(texto, -1) {
		s = strings.Replace(s, ",", ".", 1)
		if r, ok := new(big.Rat).SetString(s); ok {
			out = append(out, r)
		}
	}
	return out, cuantos
}

func (resSecuencia) Reconoce(p *nucleo.Pregunta) nucleo.Reconocimiento {
	rec := nucleo.Reconocimiento{Intencion: nucleo.ISecuencia, Lectura: "continuar una secuencia"}
	t := normal(p)
	seq, _ := LeerSecuencia(p.Texto)
	if len(seq) >= 2 && contiene(t, "siguiente", " sigue", "continua", "que numero va", "proximo", "falta", "serie", "sucesion", "secuencia") {
		rec.Puntos = 0.85
	} else if len(seq) >= 4 && strings.Count(p.Texto, ",") >= 3 && strings.Contains(p.Texto, "?") {
		rec.Puntos = 0.55
	}
	return rec
}

func (resSecuencia) Resolver(ctx context.Context, p *nucleo.Pregunta, n *nucleo.Nodo) (nucleo.Respuesta, error) {
	seq, cuantos := LeerSecuencia(p.Texto)
	var vs []string
	for _, x := range seq {
		vs = append(vs, TextoRat(x))
	}
	ent := n.Sub(nucleo.PasoEntender, "Secuencia: %s", strings.Join(vs, ", "))
	if len(seq) < 2 {
		ent.Mal("Hacen falta al menos dos números")
		return nucleo.Respuesta{}, fmt.Errorf("puzles: no encuentro los números de la secuencia: %w", nucleo.ErrNoEntiendo)
	}
	ent.Bien("")
	busca := n.Sub(nucleo.PasoIntento, "Pruebo reglas de la más sencilla a la más complicada")
	regla, err := Siguiente(seq, cuantos)
	if errors.Is(err, ErrPocosDatos) {
		busca.Info("Posible, pero con pocos datos")
		return nucleo.Respuesta{
			Texto:          "Posible, pero con pocos datos: con tan pocos números encajan varias reglas. Dame algún término más.",
			Metodo:         "secuencias",
			Nivel:          nucleo.NivelSinComprobar,
			Exito:          false,
			Comprobaciones: []nucleo.Comprobacion{{Nivel: nucleo.NivelSinComprobar, Texto: "pocos datos"}},
		}, nil
	}
	if err != nil {
		busca.Mal("No encuentro la regla")
		return nucleo.Respuesta{}, err
	}
	busca.Bien("Regla: %s", regla.Descripcion)
	var sig []string
	for _, x := range regla.Siguientes {
		sig = append(sig, TextoRat(x))
	}
	texto := fmt.Sprintf("El siguiente es %s: %s.", sig[0], regla.Descripcion)
	if len(sig) > 1 {
		texto = fmt.Sprintf("Los siguientes son %s: %s.", strings.Join(sig, ", "), regla.Descripcion)
	}
	if regla.Confianza != "alta" {
		texto += " Encaja, pero tengo pocos datos para estar seguro."
	}
	r := respuestaCalculo(texto, "secuencias", fmt.Sprintf("La regla da exactamente los %d términos que me diste", len(seq)))
	r.Comprobaciones[0].Pasan, r.Comprobaciones[0].Total = len(seq), len(seq)
	r.Entendi = "Así lo entendí: " + strings.Join(vs, ", ") + ", ¿…?"
	return r, nil
}
