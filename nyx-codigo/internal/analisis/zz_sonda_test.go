package analisis

import (
	"fmt"
	"testing"
	"time"
)

func TestZZSonda(t *testing.T) {
	f := nuevoImportadorFuentes()
	for _, r := range []string{"fmt", "net", "os/user", "strings", "math/big"} {
		t0 := time.Now()
		muImportador.Lock()
		p, err := f.Import(r)
		muImportador.Unlock()
		fmt.Println(r, time.Since(t0), p != nil && p.Complete(), err)
	}
}
