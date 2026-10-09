package nucleo

import "strings"

type Param struct {
	Nombre string `json:"nombre"`
	Tipo   Tipo   `json:"tipo"`
}

type Firma struct {
	Nombre    string  `json:"nombre"`
	Receptor  *Param  `json:"receptor,omitempty"` // method receiver (struct or *struct), nil for plain funcs
	Params    []Param `json:"params"`
	Res       []Tipo  `json:"res"`                 // 0..3 results; the last may be TError
	Variadica bool    `json:"variadica,omitempty"` // last param is ...T; its Tipo is ListaDe(T)
}

// Go returns the declaration line: "func SumaPares(nums []int) int",
// "func (p Punto) Dist(q Punto) (float64, error)", "func Suma(a, b int) int", "func Max(xs ...int) int".
// Consecutive named parameters of the same type are grouped, as gofmt'd idiomatic Go does.
func (f Firma) Go() string {
	var sb strings.Builder
	sb.WriteString("func ")
	if f.Receptor != nil {
		sb.WriteString("(")
		if f.Receptor.Nombre != "" {
			sb.WriteString(f.Receptor.Nombre)
			sb.WriteString(" ")
		}
		sb.WriteString(f.Receptor.Tipo.Go())
		sb.WriteString(") ")
	}
	sb.WriteString(f.Nombre)
	sb.WriteString("(")
	sb.WriteString(f.paramsGo())
	sb.WriteString(")")
	switch len(f.Res) {
	case 0:
	case 1:
		sb.WriteString(" ")
		sb.WriteString(f.Res[0].Go())
	default:
		sb.WriteString(" (")
		for i, r := range f.Res {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(r.Go())
		}
		sb.WriteString(")")
	}
	return sb.String()
}

func (f Firma) tipoParamGo(i int) string {
	t := f.Params[i].Tipo
	if f.Variadica && i == len(f.Params)-1 && t.Clase == CLista && !t.Definido {
		return "..." + elemDe(t.Elem).Go()
	}
	return t.Go()
}

func (f Firma) paramsGo() string {
	conNombres := true
	for _, p := range f.Params {
		if p.Nombre == "" {
			conNombres = false
		}
	}
	partes := make([]string, 0, len(f.Params))
	if !conNombres {
		for i := range f.Params {
			partes = append(partes, f.tipoParamGo(i))
		}
		return strings.Join(partes, ", ")
	}
	for i := 0; i < len(f.Params); {
		j := i
		tipo := f.tipoParamGo(i)
		nombres := []string{f.Params[i].Nombre}
		for j+1 < len(f.Params) && f.tipoParamGo(j+1) == tipo && !strings.HasPrefix(tipo, "...") {
			j++
			nombres = append(nombres, f.Params[j].Nombre)
		}
		partes = append(partes, strings.Join(nombres, ", ")+" "+tipo)
		i = j + 1
	}
	return strings.Join(partes, ", ")
}

// Entradas returns the input types: receiver first (if any), then params.
func (f Firma) Entradas() []Tipo {
	out := make([]Tipo, 0, len(f.Params)+1)
	if f.Receptor != nil {
		out = append(out, f.Receptor.Tipo)
	}
	for _, p := range f.Params {
		out = append(out, p.Tipo)
	}
	return out
}

// Probable: every input/result Codificable, ≤6 inputs, ≤3 results, ≥1 result. Inputs may not be
// error, only the last result may be error, and a receiver must be a struct or a pointer to one.
func (f Firma) Probable() bool {
	ent := f.Entradas()
	if len(ent) > 6 || len(f.Res) == 0 || len(f.Res) > 3 {
		return false
	}
	if f.Receptor != nil {
		c := f.Receptor.Tipo.Clase
		if c != CStruct && c != CPuntero {
			return false
		}
	}
	if f.Variadica && (len(f.Params) == 0 || f.Params[len(f.Params)-1].Tipo.Clase != CLista) {
		return false
	}
	for _, t := range ent {
		if t.Clase == CError || !t.Codificable() {
			return false
		}
	}
	for i, t := range f.Res {
		if !t.Codificable() || (t.Clase == CError && i != len(f.Res)-1) {
			return false
		}
	}
	return true
}

// Forma returns the types only, e.g. "([]int)int", "(Punto,Punto)(float64,error)", "(...int)int".
// It is the index key for the library.
func (f Firma) Forma() string {
	var sb strings.Builder
	sb.WriteString("(")
	ent := f.Entradas()
	desp := len(ent) - len(f.Params)
	for i, t := range ent {
		if i > 0 {
			sb.WriteString(",")
		}
		if i >= desp && f.Variadica && i == len(ent)-1 && t.Clase == CLista && !t.Definido {
			sb.WriteString("..." + elemDe(t.Elem).Go())
		} else {
			sb.WriteString(t.Go())
		}
	}
	sb.WriteString(")")
	switch len(f.Res) {
	case 0:
	case 1:
		sb.WriteString(f.Res[0].Go())
	default:
		sb.WriteString("(")
		for i, r := range f.Res {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(r.Go())
		}
		sb.WriteString(")")
	}
	return sb.String()
}
