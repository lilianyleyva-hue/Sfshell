// Package recetas is the hand-written, verified knowledge that ships inside the binary (§4.6): classic
// Go functions with test cases, whole-program templates with slots, the envoltorio that turns a function
// into a program that reads its inputs, and Spanish notes on standard library symbols and Go concepts.
// It also does lexical retrieval (TF-IDF over lemmas plus signature compatibility) over all of them.
//
// The content lives in datos/ (embedded): every data file is a sequence of entries, each one a header of
// "// clave: valor" lines followed by Go source.
package recetas

import (
	"embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

//go:embed datos
var datos embed.FS

// Receta is a verified classic function.
type Receta struct {
	Nombre      string        `json:"nombre"`      // "EsPrimo"
	Titulo      string        `json:"titulo"`      // "¿Es primo un número?"
	Descripcion string        `json:"descripcion"` // Spanish, 1–2 sentences
	Palabras    []string      `json:"palabras"`    // lemmas: ["primo", "numero", "comprobar"]
	Conceptos   []string      `json:"conceptos"`
	Firma       nucleo.Firma  `json:"firma"`
	Codigo      string        `json:"codigo"` // package solucion file
	Casos       []nucleo.Caso `json:"-"`      // ≥ 5 cases, Expectativa EspReferencia, Origen receta
}

// Hueco is a slot of a program template.
type Hueco struct {
	Nombre     string   `json:"nombre"`     // "op"
	Clase      string   `json:"clase"`      // "operacion" | "entero" | "texto" | "palabra" | "opcion"
	Opciones   []string `json:"opciones"`   // for operacion: ["suma","resta","multiplicación","división"]
	PorDefecto string   `json:"porDefecto"`
	Pregunta   string   `json:"pregunta"` // "¿Qué operación hago con los dos números?"
}

// CasoPlantilla is a scripted run of a template instantiated with Huecos.
type CasoPlantilla struct {
	Huecos map[string]string   `json:"huecos"`
	Caso   nucleo.CasoPrograma `json:"caso"`
}

// Plantilla is a whole-program template.
type Plantilla struct {
	Nombre      string          `json:"nombre"` // "dos_numeros_operacion"
	Titulo      string          `json:"titulo"` // "Programa que pide dos números y opera"
	Descripcion string          `json:"descripcion"`
	Palabras    []string        `json:"palabras"`
	Huecos      []Hueco         `json:"huecos"`
	Fuente      string          `json:"fuente"` // text/template; slot values only through go_texto / go_entero / go_op
	Casos       []CasoPlantilla `json:"casos"`
	Interactivo bool            `json:"interactivo"` // e.g. adivina el número: accepts "-semilla N" for deterministic tests
}

// Nota is a Spanish note on a standard library symbol or a Go concept.
type Nota struct {
	Clave        string   `json:"clave"` // "strings.Split" or concept "defer"
	Titulo       string   `json:"titulo"`
	Resumen      string   `json:"resumen"` // Spanish, ≤ 3 sentences, simple words
	Ejemplo      string   `json:"ejemplo"` // complete runnable program (package main)
	Salida       string   `json:"salida"`  // exact expected stdout of Ejemplo
	Relacionados []string `json:"relacionados"`
	Palabras     []string `json:"palabras"`
}

// Puntuada is a recipe with its retrieval score.
type Puntuada struct {
	Receta *Receta
	Puntos float64
}

// PuntuadaPlantilla is a template with its retrieval score.
type PuntuadaPlantilla struct {
	Plantilla *Plantilla
	Puntos    float64
}

// ---- loading ----

type contenido struct {
	funciones  []Receta
	plantillas []Plantilla
	notas      []Nota
	errores    []error // problems in the shipped data (tests require none)

	idxFunciones  *indice
	idxPlantillas *indice
	idxNotas      *indice
}

var (
	unaVez sync.Once
	todo   *contenido
)

func cargado() *contenido {
	unaVez.Do(func() {
		c := &contenido{}
		c.funciones = c.cargarFunciones()
		c.plantillas = c.cargarPlantillas()
		c.notas = c.cargarNotas()
		c.idxFunciones = indiceFunciones(c.funciones)
		c.idxPlantillas = indicePlantillas(c.plantillas)
		c.idxNotas = indiceNotas(c.notas)
		todo = c
	})
	return todo
}

// Funciones returns copies of the shipped recipes, in file order.
func Funciones() []Receta {
	c := cargado()
	out := make([]Receta, len(c.funciones))
	for i, r := range c.funciones {
		out[i] = copiarReceta(r)
	}
	return out
}

// Plantillas returns copies of the shipped program templates.
func Plantillas() []Plantilla {
	c := cargado()
	out := make([]Plantilla, len(c.plantillas))
	for i, p := range c.plantillas {
		out[i] = copiarPlantilla(p)
	}
	return out
}

// Notas returns copies of the shipped Spanish notes.
func Notas() []Nota {
	c := cargado()
	out := make([]Nota, len(c.notas))
	for i, n := range c.notas {
		out[i] = n
		out[i].Relacionados = append([]string(nil), n.Relacionados...)
		out[i].Palabras = append([]string(nil), n.Palabras...)
	}
	return out
}

// erroresDeCarga reports problems found while reading the shipped data.
func erroresDeCarga() []error { return cargado().errores }

func copiarReceta(r Receta) Receta {
	r.Palabras = append([]string(nil), r.Palabras...)
	r.Conceptos = append([]string(nil), r.Conceptos...)
	r.Firma = copiarFirma(r.Firma)
	casos := make([]nucleo.Caso, len(r.Casos))
	for i, c := range r.Casos {
		casos[i] = c
		casos[i].Entradas = copiarValores(c.Entradas)
		casos[i].Esperado = copiarValores(c.Esperado)
	}
	r.Casos = casos
	return r
}

func copiarValores(vs []nucleo.Valor) []nucleo.Valor {
	if vs == nil {
		return nil
	}
	out := make([]nucleo.Valor, len(vs))
	for i, v := range vs {
		out[i] = nucleo.Copiar(v)
	}
	return out
}

func copiarFirma(f nucleo.Firma) nucleo.Firma {
	if f.Receptor != nil {
		r := *f.Receptor
		f.Receptor = &r
	}
	f.Params = append([]nucleo.Param(nil), f.Params...)
	f.Res = append([]nucleo.Tipo(nil), f.Res...)
	return f
}

func copiarPlantilla(p Plantilla) Plantilla {
	p.Palabras = append([]string(nil), p.Palabras...)
	hs := make([]Hueco, len(p.Huecos))
	for i, h := range p.Huecos {
		hs[i] = h
		hs[i].Opciones = append([]string(nil), h.Opciones...)
	}
	p.Huecos = hs
	cs := make([]CasoPlantilla, len(p.Casos))
	for i, c := range p.Casos {
		cs[i] = c
		cs[i].Huecos = make(map[string]string, len(c.Huecos))
		for k, v := range c.Huecos {
			cs[i].Huecos[k] = v
		}
		cs[i].Caso.Args = append([]string(nil), c.Caso.Args...)
	}
	p.Casos = cs
	return p
}

// ---- data file format ----

// entrada is one entry of a data file: its header lines and its Go source.
type entrada struct {
	archivo string
	linea   int
	claves  []par
	codigo  string
}

type par struct{ k, v string }

func (e entrada) uno(k string) string {
	for _, p := range e.claves {
		if p.k == k {
			return p.v
		}
	}
	return ""
}

func (e entrada) todos(k string) []string {
	var out []string
	for _, p := range e.claves {
		if p.k == k {
			out = append(out, p.v)
		}
	}
	return out
}

func (e entrada) lista(k string) []string {
	var out []string
	for _, s := range strings.Split(e.uno(k), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// leerEntradas splits a data file into entries. An entry starts at a line "// <inicio>: …"; its header is
// the run of "// clave: valor" lines, and its code goes from the "package" line up to the next entry.
// Consecutive headers with no code between them share the code that follows.
func leerEntradas(archivo, texto, inicio string) []entrada {
	lineas := strings.Split(strings.ReplaceAll(texto, "\r\n", "\n"), "\n")
	var out []entrada
	var pendientes []entrada // headers waiting for their code
	var cod []string
	enCodigo := false
	cerrar := func() {
		if len(pendientes) == 0 {
			return
		}
		c := strings.TrimRight(strings.Join(cod, "\n"), "\n \t") + "\n"
		for _, p := range pendientes {
			p.codigo = c
			out = append(out, p)
		}
		pendientes, cod = nil, nil
	}
	marca := "// " + inicio + ":"
	for i, l := range lineas {
		if strings.HasPrefix(l, marca) {
			if enCodigo {
				cerrar()
				enCodigo = false
			}
			pendientes = append(pendientes, entrada{archivo: archivo, linea: i + 1})
		}
		if !enCodigo && len(pendientes) > 0 {
			if strings.HasPrefix(l, "package ") {
				enCodigo = true
				cod = append(cod, l)
				continue
			}
			if strings.HasPrefix(l, "//") {
				resto := strings.TrimPrefix(l, "//")
				resto = strings.TrimPrefix(resto, " ")
				if j := strings.Index(resto, ":"); j > 0 {
					k := strings.TrimSpace(resto[:j])
					v := resto[j+1:]
					v = strings.TrimPrefix(v, " ")
					if k != "salida" {
						v = strings.TrimSpace(v)
					}
					pendientes[len(pendientes)-1].claves = append(pendientes[len(pendientes)-1].claves, par{k, v})
				}
			}
			continue
		}
		if enCodigo {
			cod = append(cod, l)
		}
	}
	cerrar()
	return out
}

func (c *contenido) leerDir(dir, inicio string) []entrada {
	var out []entrada
	nombres, err := fs.Glob(datos, path.Join("datos", dir, "*.go.txt"))
	if err != nil {
		c.errores = append(c.errores, err)
		return nil
	}
	sort.Strings(nombres)
	for _, n := range nombres {
		b, err := datos.ReadFile(n)
		if err != nil {
			c.errores = append(c.errores, err)
			continue
		}
		out = append(out, leerEntradas(n, string(b), inicio)...)
	}
	return out
}

func (c *contenido) fallo(e entrada, formato string, a ...any) {
	c.errores = append(c.errores, fmt.Errorf("%s:%d: %s", e.archivo, e.linea, fmt.Sprintf(formato, a...)))
}

// normalizarPalabras lowercases and strips accents (nucleo.Normalizar) and drops duplicates.
func normalizarPalabras(ps []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, p := range ps {
		for _, w := range strings.Fields(nucleo.Normalizar(p)) {
			if !visto[w] {
				visto[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}

func (c *contenido) cargarFunciones() []Receta {
	var out []Receta
	nombres := map[string]bool{}
	for _, e := range c.leerDir("funciones", "receta") {
		r := Receta{
			Nombre:      e.uno("receta"),
			Titulo:      e.uno("titulo"),
			Descripcion: e.uno("descripcion"),
			Palabras:    normalizarPalabras(e.lista("palabras")),
			Conceptos:   e.lista("conceptos"),
			Codigo:      e.codigo,
		}
		if r.Nombre == "" || r.Titulo == "" || r.Descripcion == "" || len(r.Palabras) == 0 {
			c.fallo(e, "a la receta %q le falta el nombre, el título, la descripción o las palabras", r.Nombre)
			continue
		}
		if nombres[r.Nombre] {
			c.fallo(e, "la receta %s está repetida", r.Nombre)
			continue
		}
		nombres[r.Nombre] = true
		funcion := e.uno("funcion")
		if funcion == "" {
			funcion = r.Nombre
		}
		f, err := FirmaDe(r.Codigo, funcion)
		if err != nil {
			c.fallo(e, "receta %s: %v", r.Nombre, err)
			continue
		}
		if !f.Probable() {
			c.fallo(e, "receta %s: la firma %s no se puede probar", r.Nombre, f.Go())
			continue
		}
		r.Firma = f
		for i, txt := range e.todos("caso") {
			caso, err := leerCaso(f, txt)
			if err != nil {
				c.fallo(e, "receta %s, caso %d: %v", r.Nombre, i+1, err)
				continue
			}
			caso.Nota = "receta " + r.Nombre + ", caso " + strconv.Itoa(i+1)
			r.Casos = append(r.Casos, caso)
		}
		if len(r.Casos) < 5 {
			c.fallo(e, "la receta %s tiene %d casos (necesita al menos 5)", r.Nombre, len(r.Casos))
		}
		out = append(out, r)
	}
	return out
}

// leerCaso reads `["entrada1", …, "resultado1", …]`: inputs (receiver first) then results.
func leerCaso(f nucleo.Firma, txt string) (nucleo.Caso, error) {
	var partes []string
	if err := json.Unmarshal([]byte(txt), &partes); err != nil {
		return nucleo.Caso{}, fmt.Errorf("no es una lista JSON de textos: %v", err)
	}
	ent := f.Entradas()
	if len(partes) != len(ent)+len(f.Res) {
		return nucleo.Caso{}, fmt.Errorf("tiene %d valores y la firma pide %d entradas y %d resultados", len(partes), len(ent), len(f.Res))
	}
	c := nucleo.Caso{Expectativa: nucleo.EspReferencia, Origen: nucleo.OrigenReceta}
	for i, t := range ent {
		v, err := nucleo.ParseValor(partes[i], t)
		if err != nil {
			return nucleo.Caso{}, fmt.Errorf("entrada %d (%s): %v", i+1, partes[i], err)
		}
		c.Entradas = append(c.Entradas, v)
	}
	for i, t := range f.Res {
		v, err := nucleo.ParseValor(partes[len(ent)+i], t)
		if err != nil {
			return nucleo.Caso{}, fmt.Errorf("resultado %d (%s): %v", i+1, partes[len(ent)+i], err)
		}
		c.Esperado = append(c.Esperado, v)
	}
	return c, nil
}

func (c *contenido) cargarPlantillas() []Plantilla {
	var out []Plantilla
	for _, e := range c.leerDir("plantillas", "plantilla") {
		p := Plantilla{
			Nombre:      e.uno("plantilla"),
			Titulo:      e.uno("titulo"),
			Descripcion: e.uno("descripcion"),
			Palabras:    normalizarPalabras(e.lista("palabras")),
			Fuente:      e.codigo,
			Interactivo: e.uno("interactivo") == "si",
		}
		if p.Nombre == "" || p.Titulo == "" || len(p.Palabras) == 0 {
			c.fallo(e, "a la plantilla %q le falta el nombre, el título o las palabras", p.Nombre)
			continue
		}
		bien := true
		for _, h := range e.todos("hueco") {
			var hu Hueco
			if err := json.Unmarshal([]byte(h), &hu); err != nil {
				c.fallo(e, "plantilla %s: hueco mal escrito: %v", p.Nombre, err)
				bien = false
				continue
			}
			p.Huecos = append(p.Huecos, hu)
		}
		for i, cs := range e.todos("caso") {
			var cp CasoPlantilla
			if err := json.Unmarshal([]byte(cs), &cp); err != nil {
				c.fallo(e, "plantilla %s, caso %d: %v", p.Nombre, i+1, err)
				bien = false
				continue
			}
			if cp.Caso.Expectativa == "" {
				cp.Caso.Expectativa = nucleo.EspReferencia
			}
			if cp.Caso.Comparar == "" {
				cp.Caso.Comparar = "lineas"
			}
			if cp.Caso.Nota == "" {
				cp.Caso.Nota = "plantilla " + p.Nombre + ", caso " + strconv.Itoa(i+1)
			}
			p.Casos = append(p.Casos, cp)
		}
		if _, err := plantillaGo(p); err != nil {
			c.fallo(e, "plantilla %s: %v", p.Nombre, err)
			bien = false
		}
		if bien {
			out = append(out, p)
		}
	}
	return out
}

func (c *contenido) cargarNotas() []Nota {
	var out []Nota
	claves := map[string]bool{}
	for _, e := range c.leerDir("notas", "nota") {
		n := Nota{
			Clave:        e.uno("nota"),
			Titulo:       e.uno("titulo"),
			Resumen:      e.uno("resumen"),
			Ejemplo:      e.codigo,
			Relacionados: e.lista("relacionados"),
			Palabras:     normalizarPalabras(e.lista("palabras")),
		}
		salida := e.todos("salida")
		if len(salida) > 0 {
			n.Salida = strings.Join(salida, "\n") + "\n"
		}
		if n.Clave == "" || n.Titulo == "" || n.Resumen == "" || n.Ejemplo == "" {
			c.fallo(e, "a la nota %q le falta la clave, el título, el resumen o el ejemplo", n.Clave)
			continue
		}
		if claves[n.Clave] {
			c.fallo(e, "la nota %s está repetida", n.Clave)
			continue
		}
		claves[n.Clave] = true
		out = append(out, n)
	}
	return out
}

// ---- signatures from the AST ----

// FirmaDe builds the signature of a function ("Nombre") or method ("Tipo.Metodo") declared in codigo,
// resolving the struct types declared in the same file. Only the types the harness supports are accepted.
func FirmaDe(codigo, funcion string) (nucleo.Firma, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", codigo, 0)
	if err != nil {
		return nucleo.Firma{}, fmt.Errorf("no se puede leer el código: %v", err)
	}
	tipos := map[string]*ast.TypeSpec{}
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, sp := range gd.Specs {
				ts := sp.(*ast.TypeSpec)
				tipos[ts.Name.Name] = ts
			}
		}
	}
	receptor, nombre := "", funcion
	if i := strings.Index(funcion, "."); i >= 0 {
		receptor, nombre = funcion[:i], funcion[i+1:]
	}
	var fd *ast.FuncDecl
	for _, d := range f.Decls {
		x, ok := d.(*ast.FuncDecl)
		if !ok || x.Name.Name != nombre {
			continue
		}
		if (receptor == "") != (x.Recv == nil) {
			continue
		}
		if receptor != "" && nombreTipoReceptor(x.Recv.List[0].Type) != receptor {
			continue
		}
		fd = x
	}
	if fd == nil {
		return nucleo.Firma{}, fmt.Errorf("no encuentro la función %s", funcion)
	}
	conv := &convertidor{tipos: tipos, visto: map[string]bool{}}
	firma := nucleo.Firma{Nombre: nombre}
	if fd.Recv != nil {
		c := fd.Recv.List[0]
		t, err := conv.tipo(c.Type)
		if err != nil {
			return nucleo.Firma{}, err
		}
		firma.Receptor = &nucleo.Param{Tipo: t}
		if len(c.Names) > 0 {
			firma.Receptor.Nombre = c.Names[0].Name
		}
	}
	for i, c := range fd.Type.Params.List {
		tipoExpr := c.Type
		if el, ok := c.Type.(*ast.Ellipsis); ok {
			if i != len(fd.Type.Params.List)-1 {
				return nucleo.Firma{}, fmt.Errorf("«...» solo puede ir en el último parámetro")
			}
			firma.Variadica = true
			tipoExpr = &ast.ArrayType{Elt: el.Elt}
		}
		t, err := conv.tipo(tipoExpr)
		if err != nil {
			return nucleo.Firma{}, err
		}
		if len(c.Names) == 0 {
			firma.Params = append(firma.Params, nucleo.Param{Tipo: t})
		}
		for _, n := range c.Names {
			firma.Params = append(firma.Params, nucleo.Param{Nombre: n.Name, Tipo: t})
		}
	}
	if fd.Type.Results != nil {
		for _, c := range fd.Type.Results.List {
			t, err := conv.tipo(c.Type)
			if err != nil {
				return nucleo.Firma{}, err
			}
			k := len(c.Names)
			if k == 0 {
				k = 1
			}
			for i := 0; i < k; i++ {
				firma.Res = append(firma.Res, t)
			}
		}
	}
	return firma, nil
}

func nombreTipoReceptor(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return nombreTipoReceptor(x.X)
	case *ast.Ident:
		return x.Name
	case *ast.ParenExpr:
		return nombreTipoReceptor(x.X)
	}
	return ""
}

type convertidor struct {
	tipos map[string]*ast.TypeSpec
	visto map[string]bool
}

func (c *convertidor) tipo(e ast.Expr) (nucleo.Tipo, error) {
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "error" {
			return nucleo.TError, nil
		}
		if ts, ok := c.tipos[x.Name]; ok {
			if c.visto[x.Name] {
				return nucleo.Tipo{}, fmt.Errorf("el tipo %s es recursivo", x.Name)
			}
			c.visto[x.Name] = true
			defer delete(c.visto, x.Name)
			u, err := c.tipo(ts.Type)
			if err != nil {
				return nucleo.Tipo{}, err
			}
			if ts.Assign.IsValid() { // alias
				return u, nil
			}
			u.Nombre, u.Definido = x.Name, true
			return u, nil
		}
		return nucleo.ParseTipo(x.Name)
	case *ast.ParenExpr:
		return c.tipo(x.X)
	case *ast.ArrayType:
		el, err := c.tipo(x.Elt)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		if x.Len == nil {
			return nucleo.ListaDe(el), nil
		}
		lit, ok := x.Len.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return nucleo.Tipo{}, fmt.Errorf("el tamaño del arreglo debe ser un número")
		}
		n, err := strconv.Atoi(lit.Value)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		return nucleo.ArregloDe(n, el), nil
	case *ast.MapType:
		k, err := c.tipo(x.Key)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		v, err := c.tipo(x.Value)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		return nucleo.MapaDe(k, v), nil
	case *ast.StarExpr:
		el, err := c.tipo(x.X)
		if err != nil {
			return nucleo.Tipo{}, err
		}
		if el.Clase != nucleo.CStruct {
			return nucleo.Tipo{}, fmt.Errorf("solo sé usar punteros a estructuras")
		}
		return nucleo.PunteroA(el), nil
	case *ast.StructType:
		t := nucleo.Tipo{Clase: nucleo.CStruct}
		for _, campo := range x.Fields.List {
			ct, err := c.tipo(campo.Type)
			if err != nil {
				return nucleo.Tipo{}, err
			}
			if len(campo.Names) == 0 {
				t.Campos = append(t.Campos, nucleo.Campo{Nombre: nombreTipoReceptor(campo.Type), Tipo: ct})
			}
			for _, n := range campo.Names {
				t.Campos = append(t.Campos, nucleo.Campo{Nombre: n.Name, Tipo: ct})
			}
		}
		return t, nil
	case *ast.SelectorExpr:
		return nucleo.Tipo{}, fmt.Errorf("usa el tipo %s.%s de otro paquete", nombreTipoReceptor(x.X), x.Sel.Name)
	case *ast.ChanType:
		return nucleo.Tipo{}, fmt.Errorf("usa canales")
	case *ast.FuncType:
		return nucleo.Tipo{}, fmt.Errorf("usa funciones como valores")
	case *ast.InterfaceType:
		return nucleo.Tipo{}, fmt.Errorf("usa interfaces")
	}
	return nucleo.Tipo{}, fmt.Errorf("no sé representar este tipo")
}
