package run

import "testing"

// Zona de ocultação de casa/trabalho (plano §14): pontos dentro do raio somem
// do traçado antes de qualquer métrica ou geometria.
func TestDropWithinRadius(t *testing.T) {
	home := struct{ lat, lng float64 }{-3.7319, -38.5267}

	// ~11m por 0.0001° de latitude em Fortaleza.
	inside1 := pt(home.lat+0.0002, home.lng, 0)        // ~22 m do centro
	inside2 := pt(home.lat, home.lng+0.0003, 10)       // ~33 m
	outside1 := pt(home.lat+0.0060, home.lng, 20)      // ~665 m
	outside2 := pt(home.lat, home.lng+0.0100, 30)      // ~1.1 km

	pts := []Point{inside1, inside2, outside1, outside2}
	got := dropWithinRadius(append([]Point(nil), pts...), home.lat, home.lng, 150)

	if len(got) != 2 {
		t.Fatalf("esperava 2 pontos fora do raio, veio %d", len(got))
	}
	if got[0].T != outside1.T || got[1].T != outside2.T {
		t.Fatalf("pontos errados sobreviveram: %+v", got)
	}
}

func TestDropWithinRadius_ZeroRadiusKeepsRealTrack(t *testing.T) {
	// raio 0: só sumiria um ponto exatamente sobre o centro; um traçado real
	// (todos os pontos a metros de distância) fica intacto.
	pts := []Point{pt(-3.7305, -38.5210, 0), pt(-3.7402, -38.5305, 10)}
	got := dropWithinRadius(append([]Point(nil), pts...), -3.73, -38.52, 0)
	if len(got) != 2 {
		t.Fatalf("raio 0 removeu pontos de um traçado real: %d", len(got))
	}
}

func TestDropWithinRadius_AllInside(t *testing.T) {
	c := struct{ lat, lng float64 }{-3.73, -38.52}
	pts := []Point{pt(c.lat, c.lng, 0), pt(c.lat+0.0001, c.lng, 10), pt(c.lat, c.lng+0.0001, 20)}
	got := dropWithinRadius(append([]Point(nil), pts...), c.lat, c.lng, 300)
	if len(got) != 0 {
		t.Fatalf("esperava traçado vazio dentro da zona, veio %d", len(got))
	}
}
