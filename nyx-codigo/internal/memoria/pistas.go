package memoria

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// MaxPistas is the number of queries whose web hints are kept (the least recently saved are dropped).
const MaxPistas = 500

// constanteGuardada is a hint constant with its type, so 3 comes back as int and 3.0 as float64.
type constanteGuardada struct {
	T string          `json:"t"` // "int" | "float64" | "string" | "bool"
	V json.RawMessage `json:"v"`
}

type pistasGuardadas struct {
	Llamadas   map[string]int      `json:"llamadas,omitempty"`
	Operadores map[string]int      `json:"operadores,omitempty"`
	Constantes []constanteGuardada `json:"constantes,omitempty"`
	Orden      int64               `json:"orden"` // save counter, for dropping the oldest
}

type datosPistas struct {
	Version     int                        `json:"version"`
	PorConsulta map[string]pistasGuardadas `json:"por_consulta"`
}

type almacenPistas struct {
	mu    sync.RWMutex
	por   map[string]pistasGuardadas
	orden int64
}

func (a *Almacen) cargarPistas() {
	a.pistas.por = map[string]pistasGuardadas{}
	var d datosPistas
	if !a.leerJSON(archivoPistas, &d, &d.Version) {
		return
	}
	for k, p := range d.PorConsulta {
		if k = normalizarPalabra(k); k != "" {
			a.pistas.por[k] = p
			if p.Orden > a.pistas.orden {
				a.pistas.orden = p.Orden
			}
		}
	}
}

var tiposConstante = map[string]nucleo.Tipo{
	"int": nucleo.TInt, "float64": nucleo.TFloat, "string": nucleo.TString, "bool": nucleo.TBool,
}

func tipoConstante(v nucleo.Valor) (string, bool) {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int", true
	case float32, float64:
		return "float64", true
	case string:
		return "string", true
	case bool:
		return "bool", true
	}
	return "", false
}

// GuardarPistas keeps the hints mined from web code for a query (pistas.json). Constants that are not
// numbers, texts or booleans are left out.
func (a *Almacen) GuardarPistas(consulta string, p nucleo.Pistas) error {
	k := normalizarPalabra(consulta)
	if k == "" {
		return errors.New("memoria: falta la consulta de las pistas")
	}
	g := pistasGuardadas{Llamadas: copiarConteo(p.Llamadas), Operadores: copiarConteo(p.Operadores)}
	for _, c := range p.Constantes {
		t, ok := tipoConstante(c)
		if !ok {
			continue
		}
		m, err := nucleo.CodificarJSON(c, tiposConstante[t])
		if err != nil {
			continue
		}
		g.Constantes = append(g.Constantes, constanteGuardada{T: t, V: m})
	}
	a.pistas.mu.Lock()
	defer a.pistas.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	antes, habia := a.pistas.por[k]
	a.pistas.orden++
	g.Orden = a.pistas.orden
	a.pistas.por[k] = g
	var quitadas []string
	if len(a.pistas.por) > MaxPistas {
		claves := make([]string, 0, len(a.pistas.por))
		for c := range a.pistas.por {
			claves = append(claves, c)
		}
		sort.Slice(claves, func(i, j int) bool { return a.pistas.por[claves[i]].Orden < a.pistas.por[claves[j]].Orden })
		quitadas = claves[:len(claves)-MaxPistas]
	}
	viejas := map[string]pistasGuardadas{}
	for _, c := range quitadas {
		viejas[c] = a.pistas.por[c]
		delete(a.pistas.por, c)
	}
	err := escribirJSON(filepath.Join(a.dir, archivoPistas), datosPistas{Version: Version, PorConsulta: a.pistas.por})
	if err != nil {
		for c, v := range viejas {
			a.pistas.por[c] = v
		}
		if habia {
			a.pistas.por[k] = antes
		} else {
			delete(a.pistas.por, k)
		}
		return fmt.Errorf("memoria: no pude guardar las pistas: %w", err)
	}
	return nil
}

// Pistas returns the hints saved for a query (compared after nucleo.Normalizar).
func (a *Almacen) Pistas(consulta string) (nucleo.Pistas, bool) {
	a.pistas.mu.RLock()
	g, ok := a.pistas.por[normalizarPalabra(consulta)]
	a.pistas.mu.RUnlock()
	if !ok {
		return nucleo.Pistas{}, false
	}
	p := nucleo.Pistas{Llamadas: copiarConteo(g.Llamadas), Operadores: copiarConteo(g.Operadores)}
	for _, c := range g.Constantes {
		t, ok := tiposConstante[c.T]
		if !ok {
			continue
		}
		if v, err := nucleo.DecodificarJSON(c.V, t); err == nil {
			p.Constantes = append(p.Constantes, v)
		}
	}
	return p, true
}

func copiarConteo(m map[string]int) map[string]int {
	if m == nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
