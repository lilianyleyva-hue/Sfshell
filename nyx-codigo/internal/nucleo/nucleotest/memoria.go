package nucleotest

import (
	"context"
	"strconv"
	"sync"

	"nyxcodigo/internal/nucleo"
)

// ContadorMemoria implements nucleo.Contador in memory: per key, [éxitos, intentos].
type ContadorMemoria struct {
	mu sync.Mutex
	m  map[string][2]int
}

var _ nucleo.Contador = (*ContadorMemoria)(nil)

// NuevoContador returns an empty ContadorMemoria.
func NuevoContador() *ContadorMemoria { return &ContadorMemoria{m: map[string][2]int{}} }

func (c *ContadorMemoria) Exito(clave string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string][2]int{}
	}
	v := c.m[clave]
	if ok {
		v[0]++
	}
	v[1]++
	c.m[clave] = v
}

// Tasa is the Beta mean (éxitos+1)/(intentos+2).
func (c *ContadorMemoria) Tasa(clave string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.m[clave]
	return float64(v[0]+1) / float64(v[1]+2)
}

// Usos is the number of attempts recorded for clave.
func (c *ContadorMemoria) Usos(clave string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[clave][1]
}

// BaseHechosMemoria has the shape of logica.BaseHechos (Hechos, Todos, Reglas) and of logica.Guardar
// (GuardarHecho, GuardarRegla on the pointer). It is not safe for concurrent writes.
type BaseHechosMemoria struct {
	H []nucleo.Hecho
	R []nucleo.Regla
}

// Hechos returns the facts whose Sujeto is sujeto (compared after nucleo.Normalizar).
func (b BaseHechosMemoria) Hechos(sujeto string) []nucleo.Hecho {
	s := nucleo.Normalizar(sujeto)
	var out []nucleo.Hecho
	for _, h := range b.H {
		if nucleo.Normalizar(h.Sujeto) == s {
			out = append(out, h)
		}
	}
	return out
}

// Todos returns every fact.
func (b BaseHechosMemoria) Todos() []nucleo.Hecho { return append([]nucleo.Hecho(nil), b.H...) }

// Reglas returns every rule.
func (b BaseHechosMemoria) Reglas() []nucleo.Regla { return append([]nucleo.Regla(nil), b.R...) }

// GuardarHecho appends h, giving it an ID ("h1", "h2"…) when it has none.
func (b *BaseHechosMemoria) GuardarHecho(h nucleo.Hecho) (string, error) {
	if h.ID == "" {
		h.ID = "h" + strconv.Itoa(len(b.H)+1)
	}
	b.H = append(b.H, h)
	return h.ID, nil
}

// GuardarRegla appends r, giving it an ID ("r1", "r2"…) when it has none.
func (b *BaseHechosMemoria) GuardarRegla(r nucleo.Regla) (string, error) {
	if r.ID == "" {
		r.ID = "r" + strconv.Itoa(len(b.R)+1)
	}
	b.R = append(b.R, r)
	return r.ID, nil
}

// RespuestasFijas answers questions in order, then returns nucleo.ErrSinRespuesta (with the question's
// PorDefecto). It is safe for concurrent use and honours ctx.
func RespuestasFijas(r ...string) nucleo.Responder {
	var mu sync.Mutex
	pendientes := append([]string(nil), r...)
	return func(ctx context.Context, p nucleo.PreguntaUsuario) (string, error) {
		if err := ctx.Err(); err != nil {
			return p.PorDefecto, err
		}
		mu.Lock()
		defer mu.Unlock()
		if len(pendientes) == 0 {
			return p.PorDefecto, nucleo.ErrSinRespuesta
		}
		s := pendientes[0]
		pendientes = pendientes[1:]
		return s, nil
	}
}
