package sintesis

import (
	"context"
	"math/rand"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Exports for the external tests (package sintesis_test).

var LeerFirmaPrueba = leerFirma

func ExpresionesReferencia() [][2]string { return expresionesReferencia }

// ProgramasDelBanco returns n programs of f's result type taken at random from a synthesis bank built up to
// costo (the kind of programs the search produces).
func ProgramasDelBanco(f nucleo.Firma, ejemplos []nucleo.Caso, costo, n int, semilla int64) []*Expr {
	esp := Especificacion{Firma: f, Ejemplos: ejemplos}
	s, err := nuevaSesion(context.Background(), esp, Base(), Opciones{}.normalizar(), true)
	if err != nil {
		return nil
	}
	s.construirPools(time.Now().Add(2 * time.Second))
	s.prepararMotor()
	s.mo.correr(costo)
	ids := append([]int32(nil), s.mo.objetivos...)
	r := rand.New(rand.NewSource(semilla))
	r.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	var out []*Expr
	for _, id := range ids {
		if len(out) >= n {
			break
		}
		if s.mo.entradas[id].hoja != nil {
			continue
		}
		out = append(out, s.mo.expr(id))
	}
	return out
}
