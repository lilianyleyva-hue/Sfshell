//go:build unix

package sintesis

import (
	"syscall"
	"time"
)

// tiempoCPU is the CPU time this test process has used so far (user + system), and whether it is known.
func tiempoCPU() (time.Duration, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano()), true
}
