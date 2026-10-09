//go:build linux

package arenero

import (
	"os"
	"syscall"
)

// atributosProceso: own process group, killed when the parent dies, and (when asked) new user and
// network namespaces mapping the current user to itself, so the child has no network at all.
func atributosProceso(aislar bool) *syscall.SysProcAttr {
	sa := &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if aislar {
		sa.Cloneflags = syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET
		sa.UidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}}
		sa.GidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}}
		sa.GidMappingsEnableSetgroups = false
	}
	return sa
}

func puedeAislar() bool { return true }

// aplicarLimites sets DATA, CPU, FSIZE, NOFILE and CORE. Never RLIMIT_AS: the Go runtime aborts under it.
func aplicarLimites(l limites) error {
	poner := func(recurso int, cur, max uint64) error {
		return syscall.Setrlimit(recurso, &syscall.Rlimit{Cur: cur, Max: max})
	}
	if l.Datos > 0 {
		if err := poner(syscall.RLIMIT_DATA, l.Datos, l.Datos); err != nil {
			return err
		}
	}
	if l.CPU > 0 {
		if err := poner(syscall.RLIMIT_CPU, l.CPU, l.CPU+1); err != nil {
			return err
		}
	}
	if l.Archivo > 0 {
		if err := poner(syscall.RLIMIT_FSIZE, l.Archivo, l.Archivo); err != nil {
			return err
		}
	}
	if l.Abiertos > 0 {
		if err := poner(syscall.RLIMIT_NOFILE, l.Abiertos, l.Abiertos); err != nil {
			return err
		}
	}
	return poner(syscall.RLIMIT_CORE, 0, 0)
}
