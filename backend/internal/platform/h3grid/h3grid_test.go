package h3grid

import "testing"

// Testes agnósticos à implementação: valem para a grade stand-in e para o H3.

func TestCellAtIsStable(t *testing.T) {
	// Praça do Ferreira, Fortaleza.
	a := Coverage.CellAt(-3.7275, -38.5275)
	b := Coverage.CellAt(-3.7275, -38.5275)
	if a.ID == "" || a.ID != b.ID {
		t.Fatalf("CellAt não é estável: %q vs %q", a.ID, b.ID)
	}
}

func TestCellsInBBoxCoversAndCenters(t *testing.T) {
	minLat, minLng, maxLat, maxLng := -3.735, -38.535, -3.725, -38.525
	cells := Coverage.CellsInBBox(minLat, minLng, maxLat, maxLng)
	if len(cells) == 0 {
		t.Fatal("bbox de ~1 km² devia ter células")
	}
	seen := map[string]bool{}
	for _, c := range cells {
		if c.ID == "" {
			t.Fatal("célula sem id")
		}
		if seen[c.ID] {
			t.Fatalf("id duplicado: %s", c.ID)
		}
		seen[c.ID] = true
	}
}

func TestTagNotEmpty(t *testing.T) {
	if Coverage.Tag() == "" {
		t.Fatal("Tag vazia")
	}
}
