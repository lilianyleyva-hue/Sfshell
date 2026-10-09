//go:build !race

package sintesis

// factorCarrera stretches time limits when the race detector slows everything down.
const factorCarrera = 1.0
