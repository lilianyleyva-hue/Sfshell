package nucleo

import (
	"math/big"
	"strings"
	"unicode"
)

// Parsed requests, intents, frames and concepts (§3.7).

type Token struct {
	Texto string   `json:"texto"` // as written
	Norma string   `json:"norma"` // Normalizar(Texto)
	Lema  string   `json:"lema"`  // "sumar", "par", "perro"
	Clase string   `json:"clase"` // "palabra" | "numero" | "simbolo" | "cita" (quoted text) | "lista" ([…] literal)
	Num   *big.Rat `json:"-"`     // for numbers (digits or words)
	Desde int      `json:"desde"` // byte offsets in Pregunta.Texto
	Hasta int      `json:"hasta"`
}

type Numero struct {
	Valor *big.Rat
	Desde int
	Hasta int
	Texto string
}

type TipoIntencion string

const (
	ICrearFuncion   TipoIntencion = "crear_funcion"
	ICrearPrograma  TipoIntencion = "crear_programa"
	IReparar        TipoIntencion = "reparar"
	IExplicar       TipoIntencion = "explicar"
	IProbar         TipoIntencion = "probar"
	IEjecutar       TipoIntencion = "ejecutar"
	IOptimizar      TipoIntencion = "optimizar"
	IDocGo          TipoIntencion = "doc_go"
	ICalculo        TipoIntencion = "calculo"
	IEcuacion       TipoIntencion = "ecuacion"
	IProblema       TipoIntencion = "problema" // word problem
	INumeros        TipoIntencion = "numeros"  // primes, factorization, mcd, percentages
	ILogica         TipoIntencion = "logica"
	ISilogismo      TipoIntencion = "silogismo"
	IRecordar       TipoIntencion = "recordar"
	IPreguntarHecho TipoIntencion = "preguntar_hecho"
	IPuzle          TipoIntencion = "puzle"
	IConteo         TipoIntencion = "conteo"
	ISecuencia      TipoIntencion = "secuencia"
	IPlan           TipoIntencion = "plan"
	IBuscar         TipoIntencion = "buscar"
	IMemoria        TipoIntencion = "memoria" // "¿qué sabes?", "olvida X"
	ICharla         TipoIntencion = "charla"
	IEstudiar       TipoIntencion = "estudiar"
)

// Familia groups intents for disambiguation: "programar", "razonar", "saber", "charla" ("" if unknown).
func Familia(t TipoIntencion) string {
	switch t {
	case ICrearFuncion, ICrearPrograma, IReparar, IExplicar, IProbar, IEjecutar, IOptimizar:
		return "programar"
	case ICalculo, IEcuacion, IProblema, INumeros, ILogica, ISilogismo, IPuzle, IConteo, ISecuencia, IPlan:
		return "razonar"
	case IDocGo, IBuscar, IRecordar, IPreguntarHecho, IMemoria, IEstudiar:
		return "saber"
	case ICharla:
		return "charla"
	}
	return ""
}

type Candidata struct {
	Tipo   TipoIntencion `json:"tipo"`
	Puntos float64       `json:"puntos"` // 0..1
	Motivo string        `json:"motivo"` // "veo código y la palabra «arregla»"
}

// Marco is the semantic frame of a programming request: VERBO(MOD(OBJ)).
type Marco struct {
	Accion   string        `json:"accion"`             // a concept: "sumar", "contar", "maximo", "invertir"… (see Conceptos)
	Objeto   string        `json:"objeto"`             // "numeros", "palabras", "letras", "frase", "numero", "matriz", "lista"
	Elemento string        `json:"elemento,omitempty"` // "numero", "palabra", "letra", "vocal"
	Mods     []Modificador `json:"mods,omitempty"`
	Plural   bool          `json:"plural"`            // "las palabras más largas" → true (list result)
	Salida   string        `json:"salida,omitempty"`  // "numero" | "decimal" | "booleano" | "texto" | "lista" | "mapa" | ""
	Entrada  string        `json:"entrada,omitempty"` // explicit "recibe una lista de enteros" → "lista_numeros"
	Programa bool          `json:"programa"`          // "programa que pida/lea/muestre…"
	Lee      []string      `json:"lee,omitempty"`     // program inputs described: ["numero","numero"]
	Muestra  string        `json:"muestra,omitempty"` // program output described
}

type Modificador struct {
	Concepto string    `json:"concepto"`          // "par", "mayor_que", "divisible_por", "sin_repetir", "vocal"
	Numeros  []float64 `json:"numeros,omitempty"` // "mayores que 5" → [5]
	Texto    string    `json:"texto,omitempty"`   // "que empiecen por «a»" → "a"
	Negado   bool      `json:"negado,omitempty"`
}

type Contexto struct {
	Codigo    string        // last code in the session (pasted or produced)
	Firma     *Firma        // last function signature
	Intencion TipoIntencion // last intent
	Texto     string        // last question text
	Marco     *Marco
	Numeros   []Numero
	Funcion   string // last produced/learned function name
}

type Pregunta struct {
	Texto       string         `json:"texto"`
	Normal      string         `json:"normal"`
	Tokens      []Token        `json:"tokens"`
	Numeros     []Numero       `json:"-"`
	Codigo      string         `json:"codigo,omitempty"` // Go code extracted (fenced block, or lines that look like Go), or pasted separately
	Resto       string         `json:"resto"`            // text without the code
	ParesTexto  [][2]string    `json:"pares,omitempty"`  // raw example pairs "[1,2] -> 3"
	Ejemplos    []Caso         `json:"-"`                // typed once a Firma is known
	EjProgramas []CasoPrograma `json:"-"`                // "si escribo 5 debe salir 25"
	Firma       *Firma         `json:"firma,omitempty"`
	Marco       *Marco         `json:"marco,omitempty"`
	Conceptos   []string       `json:"conceptos,omitempty"`
	Intenciones []Candidata    `json:"intenciones"`           // sorted, best first
	ConsultaWeb string         `json:"consultaWeb,omitempty"` // English query
	Modo        string         `json:"modo,omitempty"`        // UI chip: "", "programar", "arreglar", "explicar", "mates", "logica", "puzles", "buscar"
	Contexto    *Contexto      `json:"-"`
}

// Lematizador is injected into leaves that need Spanish tokens (logica, puzles, mates).
type Lematizador func(texto string) []Token

// Normalizar: lowercase; á→a é→e í→i ó→o ú→u ü→u (ñ kept); remove ¿ ¡ and their closing ? !;
// unify quotes (“”«» → "); collapse spaces (any run of Unicode white space becomes one space); trim.
// Normalizar("¿Cuántas MANERAS?") == "cuantas maneras" (§3.10).
func Normalizar(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	espacio := false
	for _, r := range s {
		r = unicode.ToLower(r)
		switch r {
		case 'á':
			r = 'a'
		case 'é':
			r = 'e'
		case 'í':
			r = 'i'
		case 'ó':
			r = 'o'
		case 'ú', 'ü':
			r = 'u'
		case '¿', '¡', '?', '!':
			continue
		case '“', '”', '«', '»':
			r = '"'
		}
		if unicode.IsSpace(r) {
			espacio = sb.Len() > 0
			continue
		}
		if espacio {
			sb.WriteByte(' ')
			espacio = false
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// Conceptos is the frozen concept vocabulary shared by lengua (lexicon), sintesis (primitive tags),
// pruebas (properties) and recetas (keywords). conceptos_test.go checks that every package uses only these.
var Conceptos = []string{
	// actions
	"sumar", "restar", "multiplicar", "dividir", "contar", "filtrar", "transformar", "ordenar", "ordenar_desc",
	"invertir", "maximo", "minimo", "promedio", "producto", "unir", "separar", "mayusculas", "minusculas",
	"titulo", "capitalizar", "quitar_espacios", "reemplazar", "repetir", "buscar", "contiene", "posicion",
	"primero", "ultimo", "tomar", "quitar_primeros", "sin_repetir", "frecuencia", "longitud", "mas_largo",
	"mas_corto", "todos", "alguno", "ninguno", "comprobar", "a_texto", "a_numero", "factorial", "fibonacci",
	"primos", "mcd", "mcm", "digitos", "suma_digitos", "invertir_numero", "potencia", "cuadrado", "raiz",
	"absoluto", "doble", "triple", "mitad", "rango", "acumular", "transponer", "aplanar", "binario",
	"palindromo", "anagrama", "concatenar", "agregar", "eliminar", "intercambiar", "redondear", "comparar",
	// predicates / modifiers
	"par", "impar", "positivo", "negativo", "cero", "mayor_que", "menor_que", "igual_a", "distinto_de",
	"divisible_por", "primo", "vocal", "consonante", "digito", "letra", "mayuscula", "minuscula", "espacio",
	"empieza_con", "termina_con", "longitud_mayor", "longitud_menor",
	// objects
	"numero", "numeros", "decimal", "lista", "texto", "palabra", "palabras", "letras", "frase", "matriz", "mapa",
}

var conjuntoConceptos = func() map[string]bool {
	m := make(map[string]bool, len(Conceptos))
	for _, c := range Conceptos {
		m[c] = true
	}
	return m
}()

// EsConcepto reports whether c is in Conceptos.
func EsConcepto(c string) bool { return conjuntoConceptos[c] }
