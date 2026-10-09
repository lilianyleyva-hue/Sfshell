package arenero

import (
	"regexp"
	"strconv"
	"strings"

	"nyxcodigo/internal/nucleo"
)

var reErrorGo = regexp.MustCompile(`^(?:vet: )?(\S*?\.go):(\d+)(?::(\d+))?: ?(.*)$`)

// ParseErrores reads compiler and vet output: "./x.go:12:5: msg" (also "x.go:12: msg", vet "x.go:3:2: …"
// and "vet: x.go:3:2: …"); it keeps continuation lines (indented with a tab) as extra lines of the message.
// Archivo keeps the path as written, without a leading "./". When no line has a position, every
// meaningful line (for example "go: …") becomes an error with Linea 0.
func ParseErrores(salida string) []nucleo.ErrorGo {
	var out []nucleo.ErrorGo
	var sueltas []string
	ultimoEsError := false
	for _, l := range strings.Split(salida, "\n") {
		l = strings.TrimRight(l, "\r")
		if m := reErrorGo.FindStringSubmatch(l); m != nil {
			linea, _ := strconv.Atoi(m[2])
			col, _ := strconv.Atoi(m[3])
			out = append(out, nucleo.ErrorGo{
				Archivo: strings.TrimPrefix(m[1], "./"),
				Linea:   linea,
				Col:     col,
				Msg:     strings.TrimSpace(m[4]),
			})
			ultimoEsError = true
			continue
		}
		if strings.HasPrefix(l, "\t") || strings.HasPrefix(l, "    ") {
			if ultimoEsError && len(out) > 0 {
				out[len(out)-1].Msg += "\n" + strings.TrimSpace(l)
			}
			continue
		}
		ultimoEsError = false
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "too many errors") {
			continue
		}
		sueltas = append(sueltas, t)
	}
	if len(out) == 0 {
		for _, s := range sueltas {
			out = append(out, nucleo.ErrorGo{Msg: s})
		}
	}
	return out
}
