package saber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"nyxcodigo/internal/nucleo"
)

// The local Go knowledge: an index of the GOROOT Example functions (with their "// Output:") and
// `go doc`. Both work offline.

// EjemploStd is one Example function of the standard library.
type EjemploStd struct {
	Paquete, Nombre, Codigo, Salida, Archivo string
	Simbolo                                  string   `json:",omitempty"` // "Split", "Builder.WriteString", "" for package examples
	Imports                                  []string `json:",omitempty"` // import specs of the file (`"fmt"`, `name "path"`)
	Juego                                    string   `json:",omitempty"` // complete runnable program (go/doc's playable version), if any
	Desordenada                              bool     `json:",omitempty"` // "// Unordered output:"
}

// IndiceStd maps names to examples, with a small token index.
type IndiceStd struct {
	GoVersion string       `json:"goVersion"`
	Goroot    string       `json:"goroot"`
	Ejemplos  []EjemploStd `json:"ejemplos"`

	porSimbolo map[string][]int // "strings.split" → indices
	porToken   map[string][]int // token → indices
}

const archivoIndice = "indice-std.json"

// IndexarGOROOT walks GOROOT/src/**/example*_test.go and indexes every Example function with its
// output. The result is cached in dirCache/indice-std.json and rebuilt when goVersion changes.
func IndexarGOROOT(goroot, goVersion, dirCache string) (*IndiceStd, error) {
	if goroot == "" {
		return nil, errors.New("saber: no sé dónde está GOROOT")
	}
	if dirCache != "" {
		if b, err := os.ReadFile(filepath.Join(dirCache, archivoIndice)); err == nil {
			var ix IndiceStd
			if json.Unmarshal(b, &ix) == nil && ix.GoVersion == goVersion && ix.Goroot == goroot && len(ix.Ejemplos) > 0 {
				ix.preparar()
				return &ix, nil
			}
		}
	}
	src := filepath.Join(goroot, "src")
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("saber: no encuentro el código de la biblioteca en %s", src)
	}
	ix := &IndiceStd{GoVersion: goVersion, Goroot: goroot}
	err := filepath.WalkDir(src, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		nombre := d.Name()
		if d.IsDir() {
			if ruta != src && (nombre == "testdata" || nombre == "vendor" || nombre == "internal" || nombre == "cmd" ||
				strings.HasPrefix(nombre, ".") || strings.HasPrefix(nombre, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasPrefix(nombre, "example") || !strings.HasSuffix(nombre, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(src, filepath.Dir(ruta))
		if err != nil {
			return nil
		}
		ix.Ejemplos = append(ix.Ejemplos, ejemplosDeArchivo(ruta, filepath.ToSlash(rel))...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(ix.Ejemplos) == 0 {
		return nil, fmt.Errorf("saber: no encontré ejemplos en %s", src)
	}
	sort.SliceStable(ix.Ejemplos, func(i, j int) bool {
		if ix.Ejemplos[i].Paquete != ix.Ejemplos[j].Paquete {
			return ix.Ejemplos[i].Paquete < ix.Ejemplos[j].Paquete
		}
		return ix.Ejemplos[i].Nombre < ix.Ejemplos[j].Nombre
	})
	ix.preparar()
	if dirCache != "" {
		if b, err := json.Marshal(ix); err == nil {
			if os.MkdirAll(dirCache, 0o700) == nil {
				tmp := filepath.Join(dirCache, archivoIndice+".tmp")
				if os.WriteFile(tmp, b, 0o600) == nil {
					os.Rename(tmp, filepath.Join(dirCache, archivoIndice))
				}
			}
		}
	}
	return ix, nil
}

func ejemplosDeArchivo(ruta, paquete string) []EjemploStd {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, ruta, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	var imports []string
	for _, s := range f.Imports {
		im := s.Path.Value
		if s.Name != nil {
			im = s.Name.Name + " " + im
		}
		imports = append(imports, im)
	}
	decls := map[string]*ast.FuncDecl{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
			decls[fd.Name.Name] = fd
		}
	}
	var out []EjemploStd
	for _, ex := range doc.Examples(f) {
		fd := decls["Example"+ex.Name]
		if fd == nil {
			continue
		}
		e := EjemploStd{Paquete: paquete, Nombre: fd.Name.Name, Salida: ex.Output, Archivo: ruta,
			Imports: imports, Desordenada: ex.Unordered}
		var buf bytes.Buffer
		if printer.Fprint(&buf, fset, &printer.CommentedNode{Node: fd, Comments: f.Comments}) == nil {
			e.Codigo = buf.String()
		}
		e.Simbolo = simboloDeEjemplo(ex.Name)
		if ex.Play != nil {
			var pb bytes.Buffer
			if format.Node(&pb, token.NewFileSet(), ex.Play) == nil {
				e.Juego = pb.String()
			}
		}
		out = append(out, e)
	}
	return out
}

// simboloDeEjemplo: "Split" → "Split", "Split_second" → "Split", "Builder_WriteString" →
// "Builder.WriteString", "" or "_suffix" → "" (package example).
func simboloDeEjemplo(nombre string) string {
	var partes []string
	for _, p := range strings.Split(nombre, "_") {
		if p == "" || !unicode.IsUpper([]rune(p)[0]) {
			break
		}
		partes = append(partes, p)
	}
	return strings.Join(partes, ".")
}

func (ix *IndiceStd) preparar() {
	ix.porSimbolo = map[string][]int{}
	ix.porToken = map[string][]int{}
	for i, e := range ix.Ejemplos {
		ult := e.Paquete[strings.LastIndexByte(e.Paquete, '/')+1:]
		claves := []string{strings.ToLower(e.Paquete + "." + e.Simbolo), strings.ToLower(ult + "." + e.Simbolo)}
		if e.Simbolo == "" {
			claves = []string{strings.ToLower(e.Paquete), strings.ToLower(ult)}
		}
		for _, k := range claves {
			if !contieneInt(ix.porSimbolo[k], i) {
				ix.porSimbolo[k] = append(ix.porSimbolo[k], i)
			}
		}
		vistos := map[string]bool{}
		for _, t := range tokensEjemplo(e) {
			if !vistos[t] {
				vistos[t] = true
				ix.porToken[t] = append(ix.porToken[t], i)
			}
		}
	}
}

func contieneInt(xs []int, x int) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// partirCamel splits "WriteString" into "write", "string".
func partirCamel(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))) {
			if len(cur) > 0 {
				out = append(out, strings.ToLower(string(cur)))
			}
			cur = cur[:0]
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	if len(cur) > 0 {
		out = append(out, strings.ToLower(string(cur)))
	}
	return out
}

func tokensEjemplo(e EjemploStd) []string {
	var out []string
	for _, p := range strings.Split(e.Paquete, "/") {
		out = append(out, strings.ToLower(p))
	}
	for _, p := range strings.Split(e.Simbolo, ".") {
		if p != "" {
			out = append(out, strings.ToLower(p))
			out = append(out, partirCamel(p)...)
		}
	}
	// identifiers used in the code (selectors like strings.Fields count as "fields")
	for _, w := range strings.FieldsFunc(e.Codigo, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) >= 3 && unicode.IsUpper([]rune(w)[0]) {
			out = append(out, strings.ToLower(w))
		}
	}
	return out
}

// Buscar returns up to max examples: exact symbol matches first ("strings.Split", "Split", "strings"),
// then the examples that share the most words.
func (ix *IndiceStd) Buscar(palabras []string, simbolo string, max int) []EjemploStd {
	if ix == nil {
		return nil
	}
	if max <= 0 {
		max = 3
	}
	if ix.porSimbolo == nil {
		ix.preparar()
	}
	var out []EjemploStd
	usados := map[int]bool{}
	tomar := func(i int) {
		if !usados[i] && len(out) < max {
			usados[i] = true
			out = append(out, ix.Ejemplos[i])
		}
	}
	simbolo = strings.ToLower(strings.TrimSpace(simbolo))
	if simbolo != "" {
		for _, i := range ix.porSimbolo[simbolo] {
			tomar(i)
		}
		if !strings.Contains(simbolo, ".") {
			// a bare symbol: "Split" in any package
			var cand []int
			for i, e := range ix.Ejemplos {
				if strings.ToLower(e.Simbolo) == simbolo {
					cand = append(cand, i)
				}
			}
			for _, i := range cand {
				tomar(i)
			}
		}
	}
	if len(out) >= max || len(palabras) == 0 {
		return out
	}
	puntos := map[int]float64{}
	n := float64(len(ix.Ejemplos))
	for _, p := range palabras {
		for _, t := range partirCamel(p) {
			lista := ix.porToken[t]
			if len(lista) == 0 {
				continue
			}
			idf := 1 + logSuave(n/float64(len(lista)))
			for _, i := range lista {
				puntos[i] += idf
			}
		}
	}
	type par struct {
		i int
		p float64
	}
	var orden []par
	for i, p := range puntos {
		orden = append(orden, par{i, p})
	}
	sort.Slice(orden, func(a, b int) bool {
		if orden[a].p != orden[b].p {
			return orden[a].p > orden[b].p
		}
		return orden[a].i < orden[b].i
	})
	for _, o := range orden {
		tomar(o.i)
	}
	return out
}

func logSuave(x float64) float64 {
	if x <= 1 {
		return 0
	}
	r := 0.0
	for x > 1 {
		x /= 2
		r++
	}
	return r
}

// Programa wraps an Example into a package main program that runs it. It prefers go/doc's playable
// version; otherwise it uses the example function with its file's imports. It returns "" when the
// example cannot run on its own (it belongs to the package's internal tests).
func (ix *IndiceStd) Programa(e EjemploStd) string {
	if e.Juego != "" {
		return e.Juego
	}
	if e.Codigo == "" || e.Nombre == "" {
		return ""
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "ejemplo.go", "package main\n\n"+e.Codigo, parser.ParseComments)
	if err != nil {
		return ""
	}
	usados := calificadoresUsados(f)
	var imps []string
	for _, im := range e.Imports {
		ruta := im
		nombre := ""
		if i := strings.IndexByte(im, ' '); i >= 0 {
			nombre, ruta = im[:i], im[i+1:]
		}
		ruta = strings.Trim(ruta, `"`)
		q := nombre
		if q == "" {
			q = ruta[strings.LastIndexByte(ruta, '/')+1:]
		}
		if usados[q] || nombre == "_" {
			imps = append(imps, im)
		}
	}
	// an unqualified call to something not declared here (a package-internal name or a helper of the
	// test file) means the example cannot run on its own
	interno := false
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Obj == nil && !predeclarados[id.Name] {
				interno = true
			}
		}
		return true
	})
	if interno {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for _, im := range imps {
		sb.WriteString("import " + im + "\n")
	}
	sb.WriteString("\n" + e.Codigo + "\n\nfunc main() {\n\t" + e.Nombre + "()\n}\n")
	return gofmt(sb.String())
}

var predeclarados = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(`append cap clear close complex copy delete imag len make max min new panic print
		println real recover bool byte complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune
		string uint uint8 uint16 uint32 uint64 uintptr any`) {
		m[n] = true
	}
	return m
}()

// ---- go doc ----

var reSimboloDoc = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./-]*$`)

// GoDoc runs `go doc -short pkg` (for a bare package) or `go doc pkg.Sym`, offline, with a 5 s limit.
func GoDoc(ctx context.Context, goBin, simbolo string) (string, error) {
	simbolo = strings.TrimSpace(simbolo)
	if !reSimboloDoc.MatchString(simbolo) || strings.Contains(simbolo, "..") || len(simbolo) > 200 {
		return "", fmt.Errorf("saber: «%s» no parece un nombre de Go", simbolo)
	}
	if goBin == "" {
		p, err := exec.LookPath("go")
		if err != nil {
			return "", nucleo.ErrSinGo
		}
		goBin = p
	}
	ctx, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()
	args := []string{"doc"}
	paquete, ancla, err := partirSimbolo(simbolo)
	if err != nil {
		return "", err
	}
	if ancla == "" {
		args = append(args, "-short", paquete)
	} else {
		args = append(args, simbolo)
	}
	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Dir = os.TempDir()
	cmd.Env = append(entornoSinGo(os.Environ()), "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=", "GOWORK=off",
		"GO111MODULE=off", "GOTELEMETRY=off")
	var salida, errores bytes.Buffer
	cmd.Stdout = &limitado{b: &salida, max: 64 << 10}
	cmd.Stderr = &limitado{b: &errores, max: 8 << 10}
	err = cmd.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("saber: go doc tardó demasiado: %w", nucleo.ErrSinTiempo)
	}
	if err != nil {
		msg := strings.TrimSpace(errores.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%w: go doc no conoce «%s» (%s)", ErrNoEncontrado, simbolo, primeraLinea(msg))
	}
	return strings.TrimRight(salida.String(), "\n"), nil
}

func primeraLinea(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// entornoSinGo drops GO* variables that would make go doc look at modules or the network.
func entornoSinGo(env []string) []string {
	var out []string
	for _, kv := range env {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		switch k {
		case "GOFLAGS", "GOPROXY", "GOTOOLCHAIN", "GOWORK", "GO111MODULE", "GOTELEMETRY":
			continue
		}
		out = append(out, kv)
	}
	return out
}

type limitado struct {
	b   *bytes.Buffer
	max int
}

func (l *limitado) Write(p []byte) (int, error) {
	if resto := l.max - l.b.Len(); resto > 0 {
		if len(p) > resto {
			l.b.Write(p[:resto])
		} else {
			l.b.Write(p)
		}
	}
	return len(p), nil
}
