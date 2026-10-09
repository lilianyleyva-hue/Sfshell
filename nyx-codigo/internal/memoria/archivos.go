package memoria

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Version is the format version written in every file ("version":1).
const Version = 1

// antesDeRenombrar runs after the .tmp file is written and synced, just before the rename. Tests replace it
// to simulate a crash at the worst moment; in normal use it does nothing.
var antesDeRenombrar = func(tmp, final string) error { return nil }

// escribirAtomico writes datos to ruta crash-safely: <ruta>.tmp, fsync, rename, fsync of the directory.
// A crash at any point leaves either the old file or the new one, never a mix.
func escribirAtomico(ruta string, datos []byte) error {
	tmp := ruta + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(datos); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := antesDeRenombrar(tmp, ruta); err != nil {
		return err // the "crash": the .tmp stays behind, the old file is untouched
	}
	if err := os.Rename(tmp, ruta); err != nil {
		os.Remove(tmp)
		return err
	}
	sincronizarDir(filepath.Dir(ruta))
	return nil
}

// sincronizarDir fsyncs a directory so a rename survives a power cut. Some file systems refuse it;
// that is not an error worth reporting.
func sincronizarDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// escribirJSON marshals v and writes it with escribirAtomico.
func escribirJSON(ruta string, v any) error {
	datos, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return escribirAtomico(ruta, append(datos, '\n'))
}

// marcaRoto is the suffix of a corrupt file set aside: .roto-AAAAMMDD-hhmmss.
func marcaRoto(t time.Time) string { return ".roto-" + t.Format("20060102-150405") }

// apartarRoto renames a corrupt file to <ruta>.roto-AAAAMMDD-hhmmss (adding -2, -3… if taken) and returns the
// new base name.
func apartarRoto(ruta string) (string, error) {
	base := ruta + marcaRoto(time.Now())
	destino := base
	for i := 2; ; i++ {
		if _, err := os.Lstat(destino); errors.Is(err, os.ErrNotExist) {
			break
		}
		destino = fmt.Sprintf("%s-%d", base, i)
	}
	if err := os.Rename(ruta, destino); err != nil {
		return "", err
	}
	sincronizarDir(filepath.Dir(ruta))
	return filepath.Base(destino), nil
}

// leerJSON loads ruta into v. A missing file leaves v untouched and returns false. A corrupt file (bad JSON,
// or a version this program cannot read) is set aside with apartarRoto and a warning is added; v is then
// reset by the caller's zero value, so the store starts empty. It never fails because of the data.
func (a *Almacen) leerJSON(nombre string, v any, version *int) bool {
	ruta := filepath.Join(a.dir, nombre)
	datos, err := os.ReadFile(ruta)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			a.avisar(fmt.Sprintf("No pude leer %s (%v); sigo sin esos datos.", nombre, err))
		}
		return false
	}
	if len(bytes.TrimSpace(datos)) == 0 {
		return false
	}
	motivo := ""
	if err := json.Unmarshal(datos, v); err != nil {
		motivo = "estaba dañado"
	} else if version != nil && *version > Version {
		motivo = fmt.Sprintf("es de una versión más nueva (%d)", *version)
	}
	if motivo == "" {
		return true
	}
	if roto, err := apartarRoto(ruta); err == nil {
		a.avisar(fmt.Sprintf("%s %s: lo guardé aparte como %s y empecé esa parte de la memoria vacía.", nombre, motivo, roto))
	} else {
		a.avisar(fmt.Sprintf("%s %s y no pude apartarlo (%v): empecé esa parte de la memoria vacía.", nombre, motivo, err))
	}
	return false
}

// ---- JSONL ----

// cabeceraJSONL is the first line of every JSONL file.
type cabeceraJSONL struct {
	Version   int `json:"version"`
	Siguiente int `json:"siguiente,omitempty"` // next numeric id, kept across compactions
}

// lecturaJSONL is what leerJSONL found.
type lecturaJSONL struct {
	cabecera cabeceraJSONL
	lineas   [][]byte // record lines (without the header), in order, each a valid JSON object
	malas    int      // malformed or truncated lines skipped
	sinSalto bool     // the file does not end with a newline (an append was cut short)
	existe   bool
}

// leerJSONL reads a JSONL file tolerantly: malformed lines (including a truncated last line) are skipped and
// counted. The header line {"version":1,…} is recognised wherever it appears first.
func leerJSONL(ruta string) (lecturaJSONL, error) {
	var l lecturaJSONL
	f, err := os.Open(ruta)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l, nil
		}
		return l, err
	}
	defer f.Close()
	l.existe = true
	r := bufio.NewReaderSize(f, 64<<10)
	primera := true
	for {
		linea, err := r.ReadBytes('\n')
		if len(linea) > 0 {
			if linea[len(linea)-1] != '\n' {
				l.sinSalto = true
			}
			t := bytes.TrimSpace(linea)
			switch {
			case len(t) == 0:
			case !json.Valid(t) || t[0] != '{':
				l.malas++
			default:
				if primera {
					var c struct {
						Version   *int `json:"version"`
						Siguiente int  `json:"siguiente"`
					}
					if json.Unmarshal(t, &c) == nil && c.Version != nil {
						l.cabecera = cabeceraJSONL{Version: *c.Version, Siguiente: c.Siguiente}
						primera = false
						break
					}
				}
				primera = false
				l.lineas = append(l.lineas, append([]byte(nil), t...))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return l, err
		}
	}
	return l, nil
}

// reescribirJSONL writes a whole JSONL file atomically: header, then one line per record.
func reescribirJSONL(ruta string, cab cabeceraJSONL, registros []any) error {
	var buf bytes.Buffer
	c, err := json.Marshal(cab)
	if err != nil {
		return err
	}
	buf.Write(c)
	buf.WriteByte('\n')
	for _, r := range registros {
		d, err := json.Marshal(r)
		if err != nil {
			return err
		}
		buf.Write(d)
		buf.WriteByte('\n')
	}
	return escribirAtomico(ruta, buf.Bytes())
}

// anexarJSONL appends one record per line with O_APPEND, writing the header first when the file is new.
// Each record is written with a single write call, so concurrent appends never interleave inside a line.
func anexarJSONL(ruta string, registros ...any) error {
	var buf bytes.Buffer
	nuevo := false
	if st, err := os.Stat(ruta); errors.Is(err, os.ErrNotExist) {
		nuevo = true
	} else if err == nil && st.Size() == 0 {
		nuevo = true
	}
	if nuevo {
		c, _ := json.Marshal(cabeceraJSONL{Version: Version})
		buf.Write(c)
		buf.WriteByte('\n')
	}
	for _, r := range registros {
		d, err := json.Marshal(r)
		if err != nil {
			return err
		}
		buf.Write(d)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(ruta, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if nuevo {
		sincronizarDir(filepath.Dir(ruta))
	}
	return nil
}

// limpiarTemporales removes *.tmp files left by a write interrupted before its rename.
func limpiarTemporales(dir string) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entradas {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tmp") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
