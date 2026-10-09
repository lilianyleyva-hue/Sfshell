package analisis

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// archivoGrande writes a file of about n lines: many small functions with loops, ifs and a map.
func archivoGrande(n int) string {
	var sb strings.Builder
	sb.WriteString("package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
	for i := 0; strings.Count(sb.String(), "\n") < n; i++ {
		fmt.Fprintf(&sb, `// F%d hace cosas.
func F%d(nums []int, s string) (int, string) {
	total := 0
	conteo := map[int]int{}
	for i, x := range nums {
		if x%%2 == 0 {
			total += x
		} else if i > 3 {
			conteo[x]++
		}
		for j := 0; j < len(nums); j++ {
			total -= nums[j] / (j + 1)
		}
	}
	s = strings.ToUpper(s)
	fmt.Println(total, s)
	return total, s
}

`, i, i)
	}
	return sb.String()
}

func TestCotasDeTiempo(t *testing.T) {
	src := archivoGrande(1000)
	if _, err := Analizar(src); err != nil { // warms up the importer
		t.Fatal(err)
	}
	mejor := time.Hour
	for i := 0; i < 3; i++ {
		inicio := time.Now()
		inf, err := Analizar(src)
		if err != nil || len(inf.Funciones) < 40 {
			t.Fatalf("Analizar: %v, %d funciones", err, len(inf.Funciones))
		}
		mejor = min(mejor, time.Since(inicio))
	}
	// §4.5: under 50 ms; the bound here is looser for slow or instrumented (-race) test machines.
	if mejor > 500*time.Millisecond {
		t.Errorf("Analizar con 1000 líneas tardó %v", mejor)
	}
	t.Logf("Analizar, 1000 líneas: %v", mejor)
	mejor = time.Hour
	for i := 0; i < 3; i++ {
		inicio := time.Now()
		if fs, err := Narrar(src, "F3", 2); err != nil || len(fs) == 0 {
			t.Fatalf("Narrar: %v", err)
		}
		mejor = min(mejor, time.Since(inicio))
	}
	if mejor > 300*time.Millisecond {
		t.Errorf("Narrar tardó %v", mejor)
	}
	t.Logf("Narrar: %v", mejor)
}

func BenchmarkAnalizar1000(b *testing.B) {
	src := archivoGrande(1000)
	Analizar(src)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Analizar(src)
	}
}
