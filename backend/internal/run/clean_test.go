package run

import (
	"math"
	"testing"
)

// pt cria um ponto a t segundos, com precisão boa.
func pt(lat, lon float64, tSec int64) Point {
	acc := 5.0
	return Point{Lat: lat, Lon: lon, T: tSec * 1000, Acc: &acc}
}

func TestCleanSquareLoop(t *testing.T) {
	// ~quarteirão de 100m x 100m em Fortaleza, volta ao início.
	const d = 0.0009 // ~100m em graus na latitude de Fortaleza
	lat, lon := -3.7400, -38.5000
	in := []Point{
		pt(lat, lon, 0),
		pt(lat+d, lon, 30),
		pt(lat+d, lon+d, 60),
		pt(lat, lon+d, 90),
		pt(lat, lon, 120),
	}
	c, ok := clean(in)
	if !ok {
		t.Fatal("loop válido rejeitado")
	}
	if len(c.points) != 5 {
		t.Fatalf("pontos = %d, esperado 5", len(c.points))
	}
	// perímetro ~ 4 * 100m = 400m (tolerância ampla)
	if c.distM < 300 || c.distM > 500 {
		t.Fatalf("distância = %.0fm, esperado ~400m", c.distM)
	}
	if c.durS != 120 {
		t.Fatalf("duração = %.0fs, esperado 120", c.durS)
	}
}

func TestCleanDropsBadAccuracy(t *testing.T) {
	bad := 80.0
	in := []Point{
		pt(-3.74, -38.50, 0),
		{Lat: -3.7405, Lon: -38.50, T: 10_000, Acc: &bad}, // descartado
		pt(-3.741, -38.50, 20),
		pt(-3.742, -38.50, 30),
	}
	c, ok := clean(in)
	if !ok {
		t.Fatal("rejeitou traçado válido")
	}
	if len(c.points) != 3 {
		t.Fatalf("pontos = %d, esperado 3 (um descartado por precisão)", len(c.points))
	}
}

func TestCleanDropsTeleport(t *testing.T) {
	in := []Point{
		pt(-3.740, -38.50, 0),
		pt(-3.740, -38.40, 1), // ~11 km em 1s = teleporte
		pt(-3.745, -38.50, 60),
		pt(-3.750, -38.50, 120),
	}
	c, ok := clean(in)
	if !ok {
		t.Fatal("rejeitou traçado válido")
	}
	for _, p := range c.points {
		if math.Abs(p.Lon+38.40) < 0.01 {
			t.Fatal("ponto de teleporte não foi descartado")
		}
	}
	if c.distM < 900 || c.distM > 1200 {
		t.Fatalf("distância = %.0fm, esperado ~1100m (2 segmentos de ~550m)", c.distM)
	}
}

func TestCleanRejectsTooShort(t *testing.T) {
	in := []Point{pt(-3.74, -38.50, 0), pt(-3.74001, -38.50, 5)} // ~1m
	if _, ok := clean(in); ok {
		t.Fatal("traçado de 1m deveria ser rejeitado")
	}
}
