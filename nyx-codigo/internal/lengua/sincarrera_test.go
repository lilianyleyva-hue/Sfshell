//go:build !race

package lengua

// conRace: the race detector makes the code several times slower (timing bounds scale with it).
const conRace = false
