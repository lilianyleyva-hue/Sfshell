//go:build !unix

package sintesis

import "time"

// tiempoCPU is not measured on this system: the timing tests use the wall clock alone.
func tiempoCPU() (time.Duration, bool) { return 0, false }
