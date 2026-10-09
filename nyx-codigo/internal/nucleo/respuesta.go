package nucleo

import (
	"context"
	"errors"
	"time"
)

// Answers, evidence and memory records (§3.8).

type Nivel string

const (
	NivelTusEjemplos  Nivel = "tus_ejemplos" // passes every EspUsuario case
	NivelEntendido    Nivel = "entendido"    // passes EspInterpretacion/EspReferencia cases (my reading)
	NivelPropiedades  Nivel = "propiedades"  // passes concept properties
	NivelSinFallos    Nivel = "sin_fallos"   // only no-panic / determinism checked
	NivelCalculo      Nivel = "calculo"      // reasoning re-checked (substitution, constraint re-check, brute force, re-simulation)
	NivelFuente       Nivel = "fuente"       // taken from a source, with link
	NivelSinComprobar Nivel = "sin_comprobar"
	NivelFallo        Nivel = "fallo" // "No lo he conseguido"
)

type Comprobacion struct {
	Nivel   Nivel  `json:"nivel"`
	Pasan   int    `json:"pasan"`
	Total   int    `json:"total"`
	Texto   string `json:"texto"`             // UI badge text, e.g. "Cumple tus 3 ejemplos"
	Detalle string `json:"detalle,omitempty"` // "semilla 42; 200 al azar, 37 de borde"
	Comando string `json:"comando,omitempty"` // "go build … && harness (fd3)"
}

type Fuente struct {
	URL    string `json:"url"`
	Titulo string `json:"titulo"`
	Nota   string `json:"nota,omitempty"` // "fragmento de la web (licencia del sitio)"
}

type Tablero struct {
	Celdas [][]int  `json:"celdas"`
	Fijas  [][]bool `json:"fijas"` // given cells shown bold
}

type Respuesta struct {
	Tarea          string         `json:"tarea"`
	Texto          string         `json:"texto"` // first line of the bubble
	Parrafos       []string       `json:"parrafos,omitempty"`
	Entendi        string         `json:"entendi,omitempty"` // "Así lo entendí: SumaPares([1,2,3,4]) = 6 …" (shown with "No es eso")
	Codigo         string         `json:"codigo,omitempty"`
	CodigoTest     string         `json:"codigoTest,omitempty"` // generated _test.go
	Diff           string         `json:"diff,omitempty"`
	Tablas         []Tabla        `json:"tablas,omitempty"`
	Tablero        *Tablero       `json:"tablero,omitempty"`
	Plan           []string       `json:"plan,omitempty"` // numbered steps (planning, equations)
	Comprobaciones []Comprobacion `json:"comprobaciones"`
	Nivel          Nivel          `json:"nivel"` // overall (best honest level)
	Fuentes        []Fuente       `json:"fuentes,omitempty"`
	Aprendido      string         `json:"aprendido,omitempty"`     // "Lo he guardado en mi memoria como SumaPares"
	Justificacion  []string       `json:"justificacion,omitempty"` // for "¿por qué?"
	Sugerencias    []string       `json:"sugerencias,omitempty"`   // follow-up chips
	Metodo         string         `json:"metodo"`                  // "sintesis", "memoria", "receta", "web", "dpll", "gauss"…
	Exito          bool           `json:"exito"`
	Riesgos        []Violacion    `json:"riesgos,omitempty"` // for the run-confirmation dialog
}

type Reconocimiento struct {
	Puntos    float64 // 0..1
	Intencion TipoIntencion
	Lectura   string // Spanish: "ecuación lineal en x"
}

// Resolutor is implemented by reasoning leaves (mates, logica, puzles) and by cerebro's own pipelines.
type Resolutor interface {
	Nombre() string                      // stable id used for strategy statistics: "mates.ecuacion"
	Reconoce(p *Pregunta) Reconocimiento // must take < 5 ms; no I/O
	Resolver(ctx context.Context, p *Pregunta, n *Nodo) (Respuesta, error)
}

var (
	ErrNoEntiendo  = errors.New("nucleo: no entiendo el enunciado")
	ErrSinTiempo   = errors.New("nucleo: se acabó el tiempo")
	ErrSinInternet = errors.New("nucleo: sin internet")
	ErrNoSoportado = errors.New("nucleo: no sé hacer esto todavía")
)

// Contador is the learning-statistics port (memoria implements it; nucleotest.ContadorMemoria for tests).
type Contador interface {
	Exito(clave string, ok bool)
	Tasa(clave string) float64 // Beta mean (éxitos+1)/(intentos+2)
	Usos(clave string) int
}

// Pistas are hints mined from web code (calls, operators, constants) that bias synthesis priors.
type Pistas struct {
	Llamadas   map[string]int `json:"llamadas"`   // "strings.Fields": 3
	Operadores map[string]int `json:"operadores"` // "%": 2
	Constantes []Valor        `json:"constantes"`
}

type FuncionAprendida struct {
	Nombre      string         `json:"nombre"`
	Firma       Firma          `json:"firma"`
	Codigo      string         `json:"codigo"`        // complete file, package solucion
	DSL         string         `json:"dsl,omitempty"` // s-expression if synthesized
	Descripcion string         `json:"descripcion"`   // Spanish (sintesis.Describir or user text)
	Conceptos   []string       `json:"conceptos"`
	Palabras    []string       `json:"palabras"` // lemmas of the request(s)
	Casos       []CasoGuardado `json:"casos"`    // ≤ 40, user + counterexamples first
	Huella      string         `json:"huella"`
	Nivel       Nivel          `json:"nivel"`
	Usos        int            `json:"usos"`
	Origen      string         `json:"origen"` // "sintesis" | "receta" | "web" | "reparacion" | "usuario"
	Fuente      string         `json:"fuente,omitempty"`
	Creada      time.Time      `json:"creada"`
	Usada       time.Time      `json:"usada"`
	Anteriores  []string       `json:"anteriores,omitempty"` // previous Codigo versions (≤ 5) after "está mal"
}

type Hecho struct {
	ID        string    `json:"id"`
	Sujeto    string    `json:"sujeto"`   // lemma form: "toby"
	Relacion  string    `json:"relacion"` // "es_un", "es", "tiene", "significa", "creado_por"…
	Objeto    string    `json:"objeto"`   // "perro"
	Negado    bool      `json:"negado,omitempty"`
	Texto     string    `json:"texto"`  // original sentence
	Fuente    string    `json:"fuente"` // "usuario" or URL
	Confianza float64   `json:"confianza"`
	Fecha     time.Time `json:"fecha"`
	Usos      int       `json:"usos"`
}

// Regla: universally quantified categorical rule, e.g. todos los perro son mamifero:
// Si = {Sujeto:"?x", Relacion:"es_un", Objeto:"perro"}, Entonces = {Sujeto:"?x", Relacion:"es_un", Objeto:"mamifero"}.
type Regla struct {
	ID       string    `json:"id"`
	Si       []Hecho   `json:"si"`
	Entonces Hecho     `json:"entonces"`
	Texto    string    `json:"texto"`
	Fuente   string    `json:"fuente"`
	Fecha    time.Time `json:"fecha"`
}

type Peticion struct {
	Tarea       string        `json:"tarea"` // assigned by servidor
	Sesion      string        `json:"sesion"`
	Texto       string        `json:"texto"`
	Codigo      string        `json:"codigo,omitempty"`
	Modo        string        `json:"modo,omitempty"`
	Accion      string        `json:"accion,omitempty"`  // "" | "arreglar" | "explicar" | "probar" | "ejecutar" | "estudiar" | "pensar_mas"
	Entrada     string        `json:"entrada,omitempty"` // stdin for ejecutar
	Args        []string      `json:"args,omitempty"`
	Confirmado  bool          `json:"confirmado,omitempty"` // user accepted the risk dialog
	Previa      string        `json:"previa,omitempty"`     // task id to redo (pensar_mas)
	Presupuesto time.Duration `json:"-"`
}

type Opinion struct {
	Sesion     string `json:"sesion"`
	Tarea      string `json:"tarea"`
	Correcto   bool   `json:"correcto"`
	Comentario string `json:"comentario,omitempty"`
	Ejemplo    string `json:"ejemplo,omitempty"` // "con [2] debe dar 0"
}

type Episodio struct {
	Fecha     time.Time     `json:"fecha"`
	Sesion    string        `json:"sesion"`
	Tarea     string        `json:"tarea"`
	Texto     string        `json:"texto"`
	Intencion TipoIntencion `json:"intencion"`
	Metodo    string        `json:"metodo"`
	Exito     bool          `json:"exito"`
	Nivel     Nivel         `json:"nivel"`
	Ms        int64         `json:"ms"`
	Opinion   string        `json:"opinion,omitempty"` // "bien" | "mal" | ""
}

type EstadoApp struct {
	Version   string        `json:"version"`
	Arenero   EstadoArenero `json:"arenero"`
	Internet  bool          `json:"internet"`
	Funciones int           `json:"funciones"`
	Hechos    int           `json:"hechos"`
	Palabras  int           `json:"palabras"`
	Avisos    []string      `json:"avisos,omitempty"`
}
