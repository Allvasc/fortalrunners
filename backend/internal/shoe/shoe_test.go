package shoe

import "testing"

func TestDeriveLifeAndAlert(t *testing.T) {
	price := 89900 // R$ 899,00

	cases := []struct {
		name       string
		total      int
		goal       int
		wantAlert  string
		wantLifeLo float64
		wantLifeHi float64
	}{
		{"novo", 100_000, 700_000, "", 14, 15},
		{"trocar em breve", 560_000, 700_000, "trocar_em_breve", 79, 81},
		{"vencido", 720_000, 700_000, "vencido", 102, 103},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sh := Shoe{TotalDistanceM: c.total, LifespanGoalM: c.goal, PurchasePriceC: &price}
			derive(&sh)
			if sh.Alert != c.wantAlert {
				t.Fatalf("alert = %q, esperado %q", sh.Alert, c.wantAlert)
			}
			if sh.LifePct < c.wantLifeLo || sh.LifePct > c.wantLifeHi {
				t.Fatalf("life_pct = %.1f fora de [%.0f,%.0f]", sh.LifePct, c.wantLifeLo, c.wantLifeHi)
			}
			if sh.CostPerKmCents == nil || *sh.CostPerKmCents <= 0 {
				t.Fatal("custo por km não calculado")
			}
		})
	}
}

func TestDeriveNoPriceNoCost(t *testing.T) {
	sh := Shoe{TotalDistanceM: 300_000, LifespanGoalM: 700_000}
	derive(&sh)
	if sh.CostPerKmCents != nil {
		t.Fatal("custo por km não deveria existir sem preço")
	}
}
