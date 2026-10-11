package saber

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// The cache keeps EXTRACTED results only (documents, link lists, topic lists, summaries), never raw
// HTML. On disk each entry lives at DirCache/xx/<sha256(clave)>.json; a small in-memory map sits in
// front of it. With DirCache == "" only the memory map is used.

const maxCacheMemoria = 512 // entries kept in memory

type entradaCache struct {
	Clave     string     `json:"clave"`
	Fecha     time.Time  `json:"fecha"`
	Documento *Documento `json:"documento,omitempty"`
	Enlaces   []Enlace   `json:"enlaces,omitempty"`
	Temas     []Tema     `json:"temas,omitempty"`
	Resumen   *Resumen   `json:"resumen,omitempty"`
}

type cache struct {
	dir   string
	reloj func() time.Time
	mu    sync.Mutex
	mem   map[string]entradaCache
}

func nuevaCache(dir string, reloj func() time.Time) *cache {
	return &cache{dir: dir, reloj: reloj, mem: map[string]entradaCache{}}
}

func hashClave(clave string) string {
	h := sha256.Sum256([]byte(clave))
	return hex.EncodeToString(h[:])
}

func (c *cache) ruta(clave string) string {
	h := hashClave(clave)
	return filepath.Join(c.dir, h[:2], h+".json")
}

// leer returns the entry for clave if it is younger than ttl.
func (c *cache) leer(clave string, ttl time.Duration) (entradaCache, bool) {
	ahora := c.reloj()
	c.mu.Lock()
	e, ok := c.mem[clave]
	c.mu.Unlock()
	if ok {
		if ahora.Sub(e.Fecha) < ttl {
			return e, true
		}
		return entradaCache{}, false
	}
	if c.dir == "" {
		return entradaCache{}, false
	}
	b, err := os.ReadFile(c.ruta(clave))
	if err != nil {
		return entradaCache{}, false
	}
	if err := json.Unmarshal(b, &e); err != nil || e.Clave != clave {
		return entradaCache{}, false
	}
	if ahora.Sub(e.Fecha) >= ttl {
		return entradaCache{}, false
	}
	c.recordar(e)
	return e, true
}

func (c *cache) recordar(e entradaCache) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.mem) >= maxCacheMemoria {
		n := 0
		for k := range c.mem { // drop an arbitrary quarter
			delete(c.mem, k)
			n++
			if n >= maxCacheMemoria/4 {
				break
			}
		}
	}
	c.mem[e.Clave] = e
}

// escribir stores e under clave (dated now). Disk errors are ignored: the cache is an optimisation.
func (c *cache) escribir(clave string, e entradaCache) {
	e.Clave = clave
	e.Fecha = c.reloj()
	c.recordar(e)
	if c.dir == "" {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	ruta := c.ruta(clave)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(ruta), ".tmp-*")
	if err != nil {
		return
	}
	nombre := tmp.Name()
	_, errW := tmp.Write(b)
	errC := tmp.Close()
	if errW != nil || errC != nil {
		os.Remove(nombre)
		return
	}
	os.Chmod(nombre, 0o600)
	if err := os.Rename(nombre, ruta); err != nil {
		os.Remove(nombre)
	}
}

// desalojar deletes the oldest files (by mtime) until the cache holds at most max bytes.
func (c *cache) desalojar(max int64) {
	if c.dir == "" {
		return
	}
	type archivo struct {
		ruta  string
		tam   int64
		fecha time.Time
	}
	var todos []archivo
	var total int64
	filepath.WalkDir(c.dir, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		todos = append(todos, archivo{ruta, info.Size(), info.ModTime()})
		total += info.Size()
		return nil
	})
	if total <= max {
		return
	}
	sort.Slice(todos, func(i, j int) bool { return todos[i].fecha.Before(todos[j].fecha) })
	for _, a := range todos {
		if total <= max {
			break
		}
		if os.Remove(a.ruta) == nil {
			total -= a.tam
		}
	}
}
