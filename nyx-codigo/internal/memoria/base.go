package memoria

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// baseJSONL is an append-only store of records with an "id" field. A later line with the same id replaces
// the earlier one; {"id":…,"borrado":true} is a tombstone. At Abrir the file is compacted (rewritten with
// only the live records) when dead lines exceed 30 %, or when it had damaged lines.
type baseJSONL[T any] struct {
	mu        sync.RWMutex
	nombre    string // file name
	prefijo   string // id prefix: "h", "r", "p"
	idDe      func(*T) *string
	reg       map[string]T
	orden     []string // live ids, in insertion order
	siguiente int      // next numeric id
}

type lapida struct {
	ID      string `json:"id"`
	Borrado bool   `json:"borrado"`
}

// cargarBase loads b from its file, compacting it when needed.
func cargarBase[T any](a *Almacen, b *baseJSONL[T]) {
	b.reg = map[string]T{}
	b.siguiente = 1
	ruta := filepath.Join(a.dir, b.nombre)
	l, err := leerJSONL(ruta)
	if err != nil {
		a.avisar(fmt.Sprintf("No pude leer %s (%v); sigo con lo que leí.", b.nombre, err))
	}
	if !l.existe {
		return
	}
	if l.cabecera.Version > Version {
		a.avisar(fmt.Sprintf("%s es de una versión más nueva (%d): leo lo que entiendo.", b.nombre, l.cabecera.Version))
	}
	muertas := 0
	malas := l.malas
	for _, linea := range l.lineas {
		var p lapida
		if json.Unmarshal(linea, &p) != nil || p.ID == "" {
			malas++
			continue
		}
		b.verID(p.ID)
		if p.Borrado {
			if _, ok := b.reg[p.ID]; ok {
				delete(b.reg, p.ID)
				b.orden = quitarID(b.orden, p.ID)
				muertas++ // the record it buries
			}
			muertas++ // the tombstone itself
			continue
		}
		var r T
		if json.Unmarshal(linea, &r) != nil {
			malas++
			continue
		}
		if _, ok := b.reg[p.ID]; ok {
			muertas++ // superseded version
		} else {
			b.orden = append(b.orden, p.ID)
		}
		b.reg[p.ID] = r
	}
	if l.cabecera.Siguiente > b.siguiente {
		b.siguiente = l.cabecera.Siguiente
	}
	if malas > 0 {
		a.avisar(fmt.Sprintf("%s tenía %d línea(s) dañada(s) o cortada(s); las he quitado.", b.nombre, malas))
	}
	total := len(l.lineas) + l.malas
	if malas > 0 || l.sinSalto || (total > 0 && muertas*10 > total*3) {
		if err := b.compactar(ruta); err != nil {
			a.avisar(fmt.Sprintf("No pude compactar %s: %v.", b.nombre, err))
		}
	}
}

// verID keeps siguiente above every numeric id seen ("h12" → 13).
func (b *baseJSONL[T]) verID(id string) {
	if !strings.HasPrefix(id, b.prefijo) {
		return
	}
	if n, err := strconv.Atoi(id[len(b.prefijo):]); err == nil && n >= b.siguiente {
		b.siguiente = n + 1
	}
}

// compactar rewrites the file with only the live records. The caller holds no lock or the write lock.
func (b *baseJSONL[T]) compactar(ruta string) error {
	regs := make([]any, 0, len(b.orden))
	for _, id := range b.orden {
		regs = append(regs, b.reg[id])
	}
	return reescribirJSONL(ruta, cabeceraJSONL{Version: Version, Siguiente: b.siguiente}, regs)
}

// nuevoID returns an unused id. The caller holds the write lock.
func (b *baseJSONL[T]) nuevoID() string {
	for {
		id := b.prefijo + strconv.Itoa(b.siguiente)
		b.siguiente++
		if _, ok := b.reg[id]; !ok {
			return id
		}
	}
}

// poner appends r (with its id already set) and keeps it in memory. The caller holds the write lock.
func (b *baseJSONL[T]) poner(dir string, r T) error {
	id := *b.idDe(&r)
	if err := anexarJSONL(filepath.Join(dir, b.nombre), r); err != nil {
		return err
	}
	b.verID(id)
	if _, ok := b.reg[id]; !ok {
		b.orden = append(b.orden, id)
	}
	b.reg[id] = r
	return nil
}

// borrar appends a tombstone for id. The caller holds the write lock.
func (b *baseJSONL[T]) borrar(dir, id string) error {
	if err := anexarJSONL(filepath.Join(dir, b.nombre), lapida{ID: id, Borrado: true}); err != nil {
		return err
	}
	delete(b.reg, id)
	b.orden = quitarID(b.orden, id)
	return nil
}

// todos returns the live records in insertion order. The caller holds a lock.
func (b *baseJSONL[T]) todos() []T {
	out := make([]T, 0, len(b.orden))
	for _, id := range b.orden {
		out = append(out, b.reg[id])
	}
	return out
}

func quitarID(ids []string, id string) []string {
	for i, x := range ids {
		if x == id {
			return append(ids[:i:i], ids[i+1:]...)
		}
	}
	return ids
}
