//go:build !linux

package arenero

import "syscall"

// Outside Linux there are no namespaces, no Pdeathsig and no rlimits from the trampoline:
// Estado reports Limites=false and Nivel "básica".

func atributosProceso(aislar bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func puedeAislar() bool { return false }

func aplicarLimites(l limites) error { return errSinSoporte }
