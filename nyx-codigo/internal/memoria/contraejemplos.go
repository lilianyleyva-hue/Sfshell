package memoria

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// MaxContraejemplos is the number of counterexamples kept per key (the oldest are dropped).
const MaxContraejemplos = 50

type datosContraejemplos struct {
	Version  int                              `json:"version"`
	PorClave map[string][]nucleo.CasoGuardado `json:"por_clave"`
}

type contraejemplos struct {
	mu  sync.RWMutex
	por map[string][]nucleo.CasoGuardado
}

func (a *Almacen) cargarContraejemplos() {
	a.contra.por = map[string][]nucleo.CasoGuardado{}
	var d datosContraejemplos
	if !a.leerJSON(archivoContraejemplos, &d, &d.Version) {
		return
	}
	for k, cs := range d.PorClave {
		if k != "" && len(cs) > 0 {
			a.contra.por[k] = cs
		}
	}
}

// ClaveContraejemplo builds the key of GuardarContraejemplo: the sorted, distinct concepts joined by ","
// plus "|" plus f.Forma(), e.g. "par,sumar|([]int)int".
func ClaveContraejemplo(conceptos []string, f nucleo.Firma) string {
	cs := normalizarLista(conceptos)
	sort.Strings(cs)
	return strings.Join(cs, ",") + "|" + f.Forma()
}

// GuardarContraejemplo stores a case from "está mal: con X da Y" under clave (see ClaveContraejemplo). A
// case with the same inputs replaces the older one. Origen defaults to "contraejemplo" and Expectativa to
// "usuario" when the case has an expected output.
func (a *Almacen) GuardarContraejemplo(clave string, f nucleo.Firma, c nucleo.Caso) error {
	if strings.TrimSpace(clave) == "" {
		return errors.New("memoria: falta la clave del contraejemplo")
	}
	if c.Origen == "" {
		c.Origen = nucleo.OrigenContraejemplo
	}
	if c.Expectativa == nucleo.EspNinguna && c.Esperado != nil {
		c.Expectativa = nucleo.EspUsuario
	}
	gs, err := nucleo.GuardarCasos(f, []nucleo.Caso{c})
	if err != nil {
		return fmt.Errorf("memoria: no puedo guardar el contraejemplo: %w", err)
	}
	g := gs[0]
	a.contra.mu.Lock()
	defer a.contra.mu.Unlock()
	if err := a.abierto(); err != nil {
		return err
	}
	antes := a.contra.por[clave]
	k := claveCaso(g)
	lista := make([]nucleo.CasoGuardado, 0, len(antes)+1)
	for _, x := range antes {
		if claveCaso(x) != k {
			lista = append(lista, x)
		}
	}
	lista = append(lista, g)
	if len(lista) > MaxContraejemplos {
		lista = lista[len(lista)-MaxContraejemplos:]
	}
	a.contra.por[clave] = lista
	if err := a.guardarContraejemplos(); err != nil {
		if antes == nil {
			delete(a.contra.por, clave)
		} else {
			a.contra.por[clave] = antes
		}
		return fmt.Errorf("memoria: no pude guardar el contraejemplo: %w", err)
	}
	return nil
}

func (a *Almacen) guardarContraejemplos() error {
	return escribirJSON(filepath.Join(a.dir, archivoContraejemplos), datosContraejemplos{Version: Version, PorClave: a.contra.por})
}

// Contraejemplos returns the cases stored under clave, decoded for f (oldest first). Cases that do not fit f
// are skipped.
func (a *Almacen) Contraejemplos(clave string, f nucleo.Firma) []nucleo.Caso {
	a.contra.mu.RLock()
	gs := append([]nucleo.CasoGuardado(nil), a.contra.por[clave]...)
	a.contra.mu.RUnlock()
	var out []nucleo.Caso
	for _, g := range gs {
		cs, err := nucleo.LeerCasos(f, []nucleo.CasoGuardado{g})
		if err != nil {
			continue
		}
		out = append(out, cs[0])
	}
	return out
}
