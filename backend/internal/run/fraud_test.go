package run

import (
	"testing"
	"time"
)

func ingestFrom(pts []Point, mut func(*IngestInput)) (IngestInput, cleaned, Metrics, bool) {
	in := IngestInput{
		StartedAt:  time.UnixMilli(pts[0].T),
		EndedAt:    time.UnixMilli(pts[len(pts)-1].T),
		DataSource: "watch", // sem penalidade de attestation
		Points:     pts,
	}
	if mut != nil {
		mut(&in)
	}
	c, ok := clean(pts)
	if !ok {
		return in, c, Metrics{}, false
	}
	return in, c, computeMetrics(c.points, in.Cadence, in.HeartRate), true
}

func TestFraudCleanRunPasses(t *testing.T) {
	in, c, m, ok := ingestFrom(line(30, 100, 30, 0), nil)
	if !ok {
		t.Fatal("traçado válido rejeitado")
	}
	r := scoreFraud(in, c, m)
	if r.flagged() {
		t.Fatalf("corrida limpa foi sinalizada: %+v", r)
	}
}

func TestFraudMockLocation(t *testing.T) {
	in, c, m, _ := ingestFrom(line(30, 100, 30, 0), func(i *IngestInput) { i.MockLocation = true })
	r := scoreFraud(in, c, m)
	if !r.flagged() || !hasFlag(r, "mock_location") {
		t.Fatalf("mock location não sinalizou: %+v", r)
	}
}

func TestFraudImpossiblePace(t *testing.T) {
	// 100 m a cada 5 s = 20 m/s = 50 s/km (mas < 12 m/s por segmento? 20 > 12,
	// vira teleporte). Use 100 m / 8 s = 12.5 m/s -> ainda teleporte. 100/9 ~ 11.1.
	in, c, m, ok := ingestFrom(line(40, 100, 9, 0), nil)
	if !ok {
		t.Fatal("rejeitado")
	}
	r := scoreFraud(in, c, m)
	if !r.flagged() || !hasFlag(r, "pace_impossible") {
		t.Fatalf("pace impossível não sinalizou: %+v (pace=%.0f)", r, c.movingS/(c.distM/1000))
	}
}

func TestFraudTeleports(t *testing.T) {
	// traçado normal + vários pontos que saltam para longe (descartados como teleporte)
	pts := line(20, 100, 30, 0)
	for i := 0; i < 5; i++ {
		far := 10.0
		pts = append(pts, Point{Lat: -3.9, Lon: -38.9, Alt: &far, T: pts[len(pts)-1].T + 1000})
		pts = append(pts, Point{Lat: -3.73, Lon: -38.5, Alt: &far, T: pts[len(pts)-1].T + 1000})
	}
	in, c, m, ok := ingestFrom(pts, nil)
	if !ok {
		t.Fatal("rejeitado")
	}
	if c.teleports < 3 {
		t.Fatalf("esperava >=3 teleportes, veio %d", c.teleports)
	}
	r := scoreFraud(in, c, m)
	if !hasFlag(r, "teleport_jumps") {
		t.Fatalf("teleportes não sinalizaram: %+v", r)
	}
}

func TestFraudNoAttestationIsMild(t *testing.T) {
	in, c, m, _ := ingestFrom(line(30, 100, 30, 0), func(i *IngestInput) { i.DataSource = "phone" })
	r := scoreFraud(in, c, m)
	if !hasFlag(r, "no_attestation") {
		t.Fatal("celular sem attestation deveria marcar (leve)")
	}
	if r.flagged() {
		t.Fatalf("só no_attestation não deveria sinalizar: %+v", r)
	}
}

func hasFlag(r fraudResult, f string) bool {
	for _, x := range r.Flags {
		if x == f {
			return true
		}
	}
	return false
}
