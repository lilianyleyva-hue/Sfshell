package saber

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRedReal talks to the real internet. It only runs with NYX_RED=1 (never in the normal suite).
func TestRedReal(t *testing.T) {
	if os.Getenv("NYX_RED") != "1" {
		t.Skip("NYX_RED=1 para probar contra internet de verdad")
	}
	w := NuevaWeb(Config{DirCache: t.TempDir()})
	ctx, cancelar := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelar()
	if !w.Disponible(ctx) {
		t.Fatal("sin internet")
	}
	en, err := w.Buscar(ctx, "golang reverse string", 6)
	if err != nil {
		t.Fatalf("Buscar: %v", err)
	}
	for _, e := range en {
		t.Logf("%.1f %s — %s", e.Peso, e.URL, e.Titulo)
	}
	docs := w.TraerVarios(ctx, en, 4)
	t.Logf("%d documentos", len(docs))
	for _, d := range docs {
		t.Logf("%s: %d bloques, %d candidatas", d.URL, len(d.Bloques), len(Candidatas(d)))
	}
	for _, p := range Rankear("reverse a string in go", docs, 3) {
		t.Logf("pasaje %.2f %s", p.Puntos, p.Texto)
	}
	temas, err := w.TemasGoByExample(ctx)
	if err != nil || len(temas) < 50 {
		t.Fatalf("temas: %d %v", len(temas), err)
	}
	d, err := w.Traer(ctx, temas[0].URL)
	if err != nil || len(Candidatas(d)) == 0 {
		t.Fatalf("gobyexample: %v %+v", err, d)
	}
	pd, err := w.PkgGoDev(ctx, "strings.Split")
	if err != nil || !strings.Contains(pd.Bloques[0].Texto+pd.Bloques[1].Texto, "Split") {
		t.Fatalf("pkg.go.dev: %v %+v", err, pd)
	}
	r, err := w.Wikipedia(ctx, "Go (lenguaje de programación)", "es")
	t.Logf("wikipedia: %+v %v", r, err)
}
