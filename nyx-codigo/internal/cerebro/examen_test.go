package cerebro

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"nyxcodigo/internal/arenero"
	"nyxcodigo/internal/memoria"
	"nyxcodigo/internal/nucleo"
	"nyxcodigo/internal/nucleo/nucleotest"
	"nyxcodigo/internal/puzles"
	"nyxcodigo/internal/saber"
)

// The golden exam (§12): testdata/examen.tsv run through a real Cerebro (real arenero, real memoria in
// a temp dir, a fake web that finds nothing). The file header documents the format.

const separadorTurnos = " ⏎⏎ "

type filaExamen struct {
	num       int
	categoria string
	turnos    []string
	fijas     []string
	esperadas [][]string // all groups required; any alternative of a group is enough
	nivel     string     // "—" = not checked
}

// categoriasCodigo need the Go toolchain; rows 59 and 60 (seguir) too.
var categoriasCodigo = map[string]bool{"crear": true, "programa": true, "memoria": true, "reparar": true, "explicar": true, "docgo": true}

func (f filaExamen) necesitaGo() bool {
	return categoriasCodigo[f.categoria] || f.num == 59 || f.num == 60
}

func desescapar(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				sb.WriteByte('\n')
				i++
				continue
			case 't':
				sb.WriteByte('\t')
				i++
				continue
			case '\\':
				sb.WriteByte('\\')
				i++
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func leerExamen(t *testing.T) []filaExamen {
	t.Helper()
	a, err := os.Open(filepath.Join("testdata", "examen.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	var filas []filaExamen
	sc := bufio.NewScanner(a)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for linea := 1; sc.Scan(); linea++ {
		s := sc.Text()
		if strings.TrimSpace(s) == "" || strings.HasPrefix(s, "#") {
			continue
		}
		cols := strings.Split(s, "\t")
		if len(cols) != 5 {
			t.Fatalf("examen.tsv:%d: %d columnas, quiero 5", linea, len(cols))
		}
		f := filaExamen{num: len(filas) + 1, categoria: cols[0], nivel: strings.TrimSpace(cols[4])}
		for _, turno := range strings.Split(cols[1], separadorTurnos) {
			f.turnos = append(f.turnos, desescapar(turno))
		}
		if strings.TrimSpace(cols[2]) != "" {
			f.fijas = strings.Split(cols[2], "|")
		}
		for _, grupo := range strings.Split(cols[3], ";") {
			if strings.TrimSpace(grupo) != "" {
				f.esperadas = append(f.esperadas, strings.Split(grupo, "|"))
			}
		}
		filas = append(filas, f)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return filas
}

// fuerzaNivel orders the levels for the "nivel mínimo" column.
var fuerzaNivel = map[string]int{
	string(nucleo.NivelTusEjemplos): 7, string(nucleo.NivelEntendido): 6, string(nucleo.NivelCalculo): 6,
	string(nucleo.NivelPropiedades): 5, string(nucleo.NivelSinFallos): 4, string(nucleo.NivelFuente): 3,
	string(nucleo.NivelSinComprobar): 1, string(nucleo.NivelFallo): 0,
}

// plegar compares without case or accents (nucleo.Normalizar), keeping "!=".
func plegar(s string) string { return nucleo.Normalizar(strings.ReplaceAll(s, "!=", "≠")) }

// comprobar returns "" when the answer passes, or why it fails.
func (f filaExamen) comprobar(r nucleo.Respuesta) string {
	todo := plegar(todoElTexto(r) + "\n" + r.Diff + "\n" + r.CodigoTest + "\n" + strings.Join(r.Sugerencias, "\n"))
	if r.Tablero != nil {
		todo += " tablero"
	}
	for _, grupo := range f.esperadas {
		ok := false
		for _, alt := range grupo {
			if strings.Contains(todo, plegar(alt)) {
				ok = true
				break
			}
		}
		if !ok {
			return "falta «" + strings.Join(grupo, "» o «") + "»"
		}
	}
	if f.nivel != "—" && f.nivel != "" {
		quiero, ok := fuerzaNivel[f.nivel]
		if !ok {
			return "nivel mínimo desconocido: " + f.nivel
		}
		if fuerzaNivel[string(r.Nivel)] < quiero {
			return fmt.Sprintf("nivel %s, quiero al menos %s", r.Nivel, f.nivel)
		}
	}
	return ""
}

func TestExamenFormato(t *testing.T) {
	filas := leerExamen(t)
	if len(filas) != 60 {
		t.Fatalf("el examen tiene %d preguntas, quiero 60", len(filas))
	}
	cats := map[string]bool{"crear": true, "programa": true, "memoria": true, "reparar": true, "explicar": true, "docgo": true,
		"mates": true, "problema": true, "logica": true, "silogismo": true, "hechos": true, "puzle": true, "plan": true,
		"conteo": true, "secuencia": true, "charla": true, "seguir": true}
	for _, f := range filas {
		if !cats[f.categoria] {
			t.Errorf("#%d: categoría desconocida %q", f.num, f.categoria)
		}
		if _, ok := fuerzaNivel[f.nivel]; !ok && f.nivel != "—" {
			t.Errorf("#%d: nivel desconocido %q", f.num, f.nivel)
		}
		if len(f.esperadas) == 0 {
			t.Errorf("#%d: sin subcadenas esperadas", f.num)
		}
	}
	// #16 repeats #1 in the same session
	if len(filas[15].turnos) != 2 || filas[15].turnos[0] != filas[0].turnos[0] || filas[15].turnos[1] != filas[0].turnos[0] {
		t.Errorf("#16 debe repetir #1 en la misma sesión: %q", filas[15].turnos)
	}
	// #44 is an easy sudoku with exactly one solution
	g, err := puzles.LeerSudoku(filas[43].turnos[0])
	if err != nil {
		t.Fatalf("#44: %v", err)
	}
	if _, unica, _, err := puzles.Sudoku(context.Background(), g, nil); err != nil || !unica {
		t.Errorf("#44 debe tener solución única: única=%v err=%v", unica, err)
	}
}

// webFalsa is a web that answers nothing useful (every page is 404).
func webFalsa(t *testing.T, goBin string) *saber.Web {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return saber.NuevaWeb(saber.Config{
		PermitirLocal:   true,
		BaseDDG:         srv.URL + "/ddg/",
		BaseGoByExample: srv.URL + "/gobyexample/",
		BasePkgGoDev:    srv.URL + "/pkg/",
		BaseGoDev:       srv.URL + "/godev/",
		BaseWikipedia:   srv.URL + "/wiki/%s/",
		Dormir:          func(time.Duration) {},
		GoBin:           goBin,
	})
}

// cerebroExamen builds the exam's Cerebro: real arenero (own DirDatos, shared build cache), real
// memoria in a temp dir, the fake web.
func cerebroExamen(t *testing.T) *Cerebro {
	t.Helper()
	goBin, _ := exec.LookPath("go")
	cache := ""
	if dir, err := os.UserCacheDir(); err == nil {
		cache = filepath.Join(dir, "nyx-codigo-pruebas", "go-build")
	}
	var ej nucleo.Ejecutor
	if a, err := arenero.Nuevo(arenero.Config{DirDatos: t.TempDir(), DirCache: cache, GoBin: goBin}); err == nil {
		t.Cleanup(func() { a.Cerrar() })
		ej = a
	} else {
		t.Logf("sin arenero: %v", err)
	}
	mem, err := memoria.Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mem.Cerrar() })
	c, err := Nuevo(Config{Semilla: 42}, Dependencias{Ej: ej, Mem: mem, Web: webFalsa(t, goBin), GoBin: goBin})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestExamen(t *testing.T) {
	if testing.Short() {
		t.Skip("el examen de oro tarda: se salta con -short")
	}
	filas := leerExamen(t)
	_, errGo := exec.LookPath("go")
	hayGo := errGo == nil
	c := cerebroExamen(t)

	type cuenta struct{ bien, total, saltadas int }
	porCat := map[string]*cuenta{}
	var fallos []string
	bien, total, saltadas := 0, 0, 0
	inicio := time.Now()
	for _, f := range filas {
		cu := porCat[f.categoria]
		if cu == nil {
			cu = &cuenta{}
			porCat[f.categoria] = cu
		}
		if f.necesitaGo() && !hayGo {
			cu.saltadas++
			saltadas++
			continue
		}
		responder := nucleotest.RespuestasFijas(f.fijas...)
		sesion := fmt.Sprintf("examen-%02d", f.num)
		var r nucleo.Respuesta
		t0 := time.Now()
		for i, turno := range f.turnos {
			ctx, cancelar := context.WithTimeout(context.Background(), 150*time.Second)
			r = c.Responder(ctx, nucleo.Peticion{Tarea: fmt.Sprintf("e%02d-%d", f.num, i+1), Sesion: sesion, Texto: turno}, nucleo.NuevaTraza(nil, responder))
			cancelar()
		}
		total++
		cu.total++
		if motivo := f.comprobar(r); motivo != "" {
			fallos = append(fallos, fmt.Sprintf("  #%d [%s] %q → %q (%s; %s; %s)", f.num, f.categoria,
				recortar(strings.ReplaceAll(f.turnos[len(f.turnos)-1], "\n", " "), 60),
				recortar(primeraLinea(r.Texto), 90), r.Nivel, motivo, time.Since(t0).Round(time.Millisecond)))
			continue
		}
		bien++
		cu.bien++
	}

	var cats []string
	for k := range porCat {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Examen de oro: %d/%d bien", bien, total)
	if total > 0 {
		fmt.Fprintf(&sb, " (%.0f %%)", 100*float64(bien)/float64(total))
	}
	fmt.Fprintf(&sb, ", %d saltadas sin Go, %s\n", saltadas, time.Since(inicio).Round(time.Millisecond))
	for _, k := range cats {
		cu := porCat[k]
		pct := "—"
		if cu.total > 0 {
			pct = fmt.Sprintf("%3.0f %%", 100*float64(cu.bien)/float64(cu.total))
		}
		fmt.Fprintf(&sb, "  %-10s %2d/%-2d %s", k, cu.bien, cu.total, pct)
		if cu.saltadas > 0 {
			fmt.Fprintf(&sb, "  (%d saltadas)", cu.saltadas)
		}
		sb.WriteString("\n")
	}
	if len(fallos) > 0 {
		sb.WriteString("Fallan:\n" + strings.Join(fallos, "\n") + "\n")
	}
	t.Log("\n" + sb.String())
	if total == 0 {
		t.Skip("no se pudo hacer ninguna pregunta (sin Go)")
	}
	if tasa := float64(bien) / float64(total); tasa < 0.80 {
		t.Errorf("el examen pide al menos un 80 %% de aciertos y hay un %.0f %% (%d/%d)", 100*tasa, bien, total)
	}
}
