package arenero

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"nyxcodigo/internal/instrumenta"
	"nyxcodigo/internal/nucleo"
)

// PermitidosAuto is the import allowlist for automatic runs (§7.1). "os" is allowed but restricted to
// Stdin, Stdout, Stderr, Args and Exit.
var PermitidosAuto = map[string]bool{
	"fmt": true, "strings": true, "strconv": true, "math": true, "math/big": true, "math/bits": true,
	"math/cmplx": true, "math/rand": true, "math/rand/v2": true, "sort": true, "slices": true, "maps": true,
	"cmp": true, "unicode": true, "unicode/utf8": true, "unicode/utf16": true, "errors": true, "bytes": true,
	"bufio": true, "io": true, "container/heap": true, "container/list": true, "container/ring": true,
	"regexp": true, "time": true, "encoding/hex": true, "encoding/base64": true, "encoding/binary": true,
	"hash/crc32": true, "hash/fnv": true, "crypto/sha256": true, "crypto/md5": true, "text/tabwriter": true,
	"os": true,
}

// Riesgos maps an import path to the Spanish risk sentence shown in the run-confirmation dialog.
var Riesgos = map[string]string{
	"os":            "puede leer, cambiar o borrar archivos",
	"os/exec":       "puede ejecutar otros programas",
	"os/signal":     "puede capturar las señales del sistema (como Ctrl+C)",
	"os/user":       "puede leer los datos de tu usuario",
	"net":           "puede usar internet (aquí estará sin red)",
	"net/http":      "puede usar internet (aquí estará sin red)",
	"net/url":       "trabaja con direcciones de internet",
	"net/smtp":      "puede enviar correos (aquí estará sin red)",
	"net/rpc":       "puede usar internet (aquí estará sin red)",
	"syscall":       "puede saltarse las protecciones de Go",
	"unsafe":        "puede saltarse las protecciones de Go",
	"path/filepath": "puede recorrer carpetas",
	"io/fs":         "puede recorrer carpetas",
	"io/ioutil":     "puede leer y escribir archivos",
	"runtime":       "puede cambiar cómo se ejecuta el programa",
	"runtime/debug": "puede cambiar cómo usa la memoria el programa",
	"plugin":        "puede cargar código desde otros archivos",
	"reflect":       "puede inspeccionar y cambiar valores saltándose los tipos",
	"sync":          "usa goroutines y cerrojos: no lo puedo limitar paso a paso",
	"sync/atomic":   "usa goroutines y operaciones atómicas: no lo puedo limitar paso a paso",
	"log":           "escribe mensajes de registro y puede terminar el programa",
	"embed":         "puede meter archivos dentro del programa",
	"crypto/rand":   "usa números aleatorios del sistema",
	"encoding/json": "no está en la lista segura, aunque suele ser inofensivo",
}

// usosOS are the only members of package os that automatic runs may use.
var usosOS = map[string]bool{"Stdin": true, "Stdout": true, "Stderr": true, "Args": true, "Exit": true}

// tiempoProhibido are the members of package time that automatic runs may not use.
var tiempoProhibido = map[string]bool{
	"Sleep": true, "After": true, "AfterFunc": true, "Tick": true, "NewTimer": true, "NewTicker": true,
}

// metodosProhibidos are forbidden on any value in automatic runs (they reach the file system or the OS).
var metodosProhibidos = map[string]bool{
	"SyscallConn": true, "Fd": true, "Chmod": true, "Chown": true, "Chdir": true, "Truncate": true,
	"ReadDir": true, "Readdir": true, "Readdirnames": true, "SetDeadline": true, "SetReadDeadline": true,
	"SetWriteDeadline": true,
}

// riesgo returns the sentence for an import path.
func riesgo(ruta string) string {
	if r, ok := Riesgos[ruta]; ok {
		return r
	}
	switch {
	case strings.HasPrefix(ruta, "net/"):
		return "puede usar internet (aquí estará sin red)"
	case strings.HasPrefix(ruta, "os/"):
		return "puede tocar el sistema operativo"
	case strings.HasPrefix(ruta, "runtime/"):
		return "puede cambiar cómo se ejecuta el programa"
	case strings.HasPrefix(ruta, "debug/"):
		return "puede leer programas compilados"
	}
	return "no está en la lista segura"
}

// Revisar is the static safety check (§4.2.4, §7). With nucleo.PermisoAuto it returns everything that
// blocks an automatic run; with nucleo.PermisoUsuario only what can never run (Grave). Non-grave
// violations of PermisoAuto are the risks to list in the confirmation dialog.
// A file that does not parse gives no violations (compilation will fail anyway), except the size limit.
func Revisar(fuente string, p nucleo.Permiso) []nucleo.Violacion {
	var out []nucleo.Violacion
	if len(fuente) > instrumenta.MaxBytes {
		return []nucleo.Violacion{{Que: "el archivo es demasiado grande (más de 64 KiB)", Grave: true}}
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", fuente, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	linea := func(pos token.Pos) int { return fset.Position(pos).Line }
	agregar := func(v nucleo.Violacion) { out = append(out, v) }

	// imports
	nombres := map[string]string{} // local name → import path
	for _, im := range f.Imports {
		ruta, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			continue
		}
		l := linea(im.Pos())
		local := nombreImport(ruta)
		if im.Name != nil {
			local = im.Name.Name
		}
		nombres[local] = ruta
		primero := strings.SplitN(ruta, "/", 2)[0]
		switch {
		case ruta == "C":
			agregar(nucleo.Violacion{Linea: l, Paquete: ruta, Que: "usa cgo (import \"C\"): eso nunca lo ejecuto", Grave: true})
		case strings.Contains(primero, ".") || primero == "nyxprueba" || primero == "nyxcodigo":
			agregar(nucleo.Violacion{Linea: l, Paquete: ruta, Que: "usa el paquete " + ruta + ", que no es de la biblioteca estándar de Go", Grave: true})
		case im.Name != nil && im.Name.Name == "." && (ruta == "os" || ruta == "time"):
			agregar(nucleo.Violacion{Linea: l, Paquete: ruta, Que: "importa " + ruta + " con punto (import . \"" + ruta + "\"): así no puedo revisar qué usa"})
		case !PermitidosAuto[ruta]:
			agregar(nucleo.Violacion{Linea: l, Paquete: ruta, Que: "usa el paquete " + ruta + ": " + riesgo(ruta)})
		}
	}

	// directives (also catches build constraints)
	for _, g := range f.Comments {
		for _, c := range g.List {
			for i, texto := range strings.Split(c.Text, "\n") {
				t := strings.TrimSpace(texto)
				if strings.HasPrefix(c.Text, "/*") && i == 0 {
					t = strings.TrimSpace(strings.TrimPrefix(t, "/*"))
				}
				l := linea(c.Pos()) + i
				switch {
				case strings.HasPrefix(t, "//go:linkname"), strings.HasPrefix(t, "//go:cgo_"), strings.HasPrefix(t, "//export"),
					strings.HasPrefix(t, "#cgo"):
					agregar(nucleo.Violacion{Linea: l, Que: "usa una directiva peligrosa del compilador (" + cortar(t, 40) + "): eso nunca lo ejecuto", Grave: true})
				case strings.HasPrefix(t, "//go:"), strings.HasPrefix(t, "//line"), strings.HasPrefix(t, "line "),
					strings.HasPrefix(t, "// +build"):
					agregar(nucleo.Violacion{Linea: l, Que: "usa una directiva especial del compilador (" + cortar(t, 40) + ")"})
				}
			}
		}
	}

	// the reserved prefix
	if l, ok := instrumenta.PosicionPrefijo(fuente); ok {
		agregar(nucleo.Violacion{Linea: l, Que: "usa el prefijo reservado nyx__ (es solo para Nyx Código)", Grave: true})
	}

	// goroutines and restricted members
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			agregar(nucleo.Violacion{Linea: linea(x.Pos()), Que: "lanza una goroutine (go …): en las pruebas automáticas no lo permito"})
		case *ast.SelectorExpr:
			id, esIdent := x.X.(*ast.Ident)
			if esIdent {
				if ruta, ok := nombres[id.Name]; ok {
					switch {
					case ruta == "os" && !usosOS[x.Sel.Name]:
						agregar(nucleo.Violacion{Linea: linea(x.Pos()), Paquete: "os", Que: "usa os." + x.Sel.Name + ": " + Riesgos["os"]})
					case ruta == "time" && tiempoProhibido[x.Sel.Name]:
						agregar(nucleo.Violacion{Linea: linea(x.Pos()), Paquete: "time", Que: "usa time." + x.Sel.Name + ": hace esperar al programa"})
					}
					return true
				}
			}
			if metodosProhibidos[x.Sel.Name] {
				agregar(nucleo.Violacion{Linea: linea(x.Pos()), Paquete: "os", Que: "usa el método " + x.Sel.Name + ": puede tocar archivos o el sistema"})
			}
		}
		return true
	})

	sort.SliceStable(out, func(i, j int) bool { return out[i].Linea < out[j].Linea })
	visto := map[string]bool{}
	filtrado := out[:0]
	for _, v := range out {
		k := fmt.Sprintf("%d|%s", v.Linea, v.Que)
		if visto[k] {
			continue
		}
		visto[k] = true
		if p == nucleo.PermisoUsuario && !v.Grave {
			continue
		}
		filtrado = append(filtrado, v)
	}
	if len(filtrado) == 0 {
		return nil
	}
	return filtrado
}

// Revisar is the method form of the package function (nucleo.Ejecutor).
func (a *Arenero) Revisar(fuente string, p nucleo.Permiso) []nucleo.Violacion {
	return Revisar(fuente, p)
}

func cortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// nombreImport guesses the package name of an import path: its last element, skipping /vN.
func nombreImport(ruta string) string {
	partes := strings.Split(ruta, "/")
	n := partes[len(partes)-1]
	if len(partes) > 1 && len(n) > 1 && n[0] == 'v' {
		if _, err := strconv.Atoi(n[1:]); err == nil {
			n = partes[len(partes)-2]
		}
	}
	return n
}
