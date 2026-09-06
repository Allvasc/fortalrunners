package run

import (
	"math"
	"testing"
)

const degPerMeterLat = 1.0 / 110574.0

// line cria um traçado reto para o norte: n pontos, `stepM` metros e `stepS`
// segundos entre eles, altitude subindo `climbPerStep` a cada ponto.
func line(n int, stepM, stepS, climbPerStep float64) []Point {
	pts := make([]Point, n)
	lat, t := -3.73, int64(0)
	alt := 10.0
	for i := 0; i < n; i++ {
		a := alt
		pts[i] = Point{Lat: lat, Lon: -38.5, Alt: &a, T: t}
		lat += stepM * degPerMeterLat
		t += int64(stepS * 1000)
		alt += climbPerStep
	}
	return pts
}

func constSamples(pts []Point, v float64) []Sample {
	out := make([]Sample, len(pts))
	for i, p := range pts {
		out[i] = Sample{T: p.T, V: v}
	}
	return out
}

func TestSplitsBasic(t *testing.T) {
	pts := line(30, 100, 30, 0) // 2900 m, ~300 s/km, plano
	m := computeMetrics(pts, nil, nil)

	if len(m.Splits) != 3 {
		t.Fatalf("esperava 3 splits (1000 + 1000 + ~900), veio %d", len(m.Splits))
	}
	for i, s := range m.Splits[:2] {
		if math.Abs(s.DistanceM-1000) > 1 {
			t.Errorf("split %d: distância %.1f, esperava ~1000", i+1, s.DistanceM)
		}
		if s.PaceS < 290 || s.PaceS > 310 {
			t.Errorf("split %d: pace %.0f s/km fora do esperado (~300)", i+1, s.PaceS)
		}
	}
	last := m.Splits[2]
	if last.DistanceM < 850 || last.DistanceM > 950 {
		t.Errorf("split parcial: %.1f m, esperava ~900", last.DistanceM)
	}
	if m.BestKmPaceS <= 0 || m.BestKmPaceS > 310 {
		t.Errorf("best_km_pace_s = %.0f", m.BestKmPaceS)
	}
}

func TestElevationHysteresis(t *testing.T) {
	// Sobe 2 m por ponto: passa o limiar de 1 m, tudo conta como ganho.
	up := computeMetrics(line(30, 100, 30, 2), nil, nil)
	if up.ElevGainM < 50 || up.ElevLossM != 0 {
		t.Fatalf("subida: gain=%.1f loss=%.1f (esperava gain>50, loss=0)", up.ElevGainM, up.ElevLossM)
	}
	// Ruído de ±0.4 m nunca cruza o limiar: nada conta.
	pts := line(20, 100, 30, 0)
	for i := range pts {
		j := 0.4
		if i%2 == 0 {
			j = -0.4
		}
		a := *pts[i].Alt + j
		pts[i].Alt = &a
	}
	flat := computeMetrics(pts, nil, nil)
	if flat.ElevGainM != 0 || flat.ElevLossM != 0 {
		t.Fatalf("ruído virou elevação: gain=%.1f loss=%.1f", flat.ElevGainM, flat.ElevLossM)
	}
}

func TestGradeAdjustedPace(t *testing.T) {
	// Mesma velocidade no solo, um plano e um subindo 4 m / 100 m (4%).
	flat := computeMetrics(line(40, 100, 30, 0), nil, nil)
	hill := computeMetrics(line(40, 100, 30, 4), nil, nil)

	if flat.GradeAdjPaceS <= 0 || hill.GradeAdjPaceS <= 0 {
		t.Fatal("GAP não calculado")
	}
	// No plano, GAP ≈ pace real.
	rawFlat := 300.0
	if math.Abs(flat.GradeAdjPaceS-rawFlat) > 5 {
		t.Errorf("plano: GAP %.0f distante do pace real %.0f", flat.GradeAdjPaceS, rawFlat)
	}
	// Subindo, o esforço equivale a um pace de plano MAIS RÁPIDO (menos segundos).
	if hill.GradeAdjPaceS >= rawFlat {
		t.Errorf("subida: GAP %.0f deveria ser < %.0f", hill.GradeAdjPaceS, rawFlat)
	}
}

func TestCadenceStreamAndSteps(t *testing.T) {
	pts := line(30, 100, 30, 0)
	m := computeMetrics(pts, constSamples(pts, 170), constSamples(pts, 150))

	if !m.HasCadence || !m.HasHR {
		t.Fatal("flags de stream não marcadas")
	}
	if math.Abs(m.AvgCadenceSPM-170) > 0.1 || math.Abs(m.MaxCadenceSPM-170) > 0.1 {
		t.Errorf("cadência avg=%.1f max=%.1f", m.AvgCadenceSPM, m.MaxCadenceSPM)
	}
	if math.Abs(m.AvgHRBPM-150) > 0.1 {
		t.Errorf("FC avg=%.1f", m.AvgHRBPM)
	}
	// ~870 s em movimento * 170/60 ≈ 2465 passos
	if m.StepCount < 2300 || m.StepCount > 2600 {
		t.Errorf("step_count = %d, esperava ~2465", m.StepCount)
	}
	for _, s := range m.Splits {
		if math.Abs(s.AvgCadence-170) > 1 {
			t.Errorf("split %d cadência %.1f", s.Index, s.AvgCadence)
		}
	}
}

func TestBestEfforts(t *testing.T) {
	// 6 km a 100 m / 30 s constante => 300 s/km. 1k=300, 5k=1500. Sem 10k.
	m := computeMetrics(line(61, 100, 30, 0), nil, nil)
	if got := m.BestEfforts["1k"]; got < 295 || got > 305 {
		t.Errorf("1k = %d, esperava ~300", got)
	}
	if got := m.BestEfforts["5k"]; got < 1490 || got > 1510 {
		t.Errorf("5k = %d, esperava ~1500", got)
	}
	if _, ok := m.BestEfforts["10k"]; ok {
		t.Error("não deveria haver recorde de 10k numa corrida de 6 km")
	}

	// corrida longa (1h20) => recorde de "1h" em metros
	long := computeMetrics(line(481, 20, 10, 0), nil, nil) // 9600 m em 4800 s
	if _, ok := long.BestEfforts["1h"]; !ok {
		t.Error("esperava recorde de 1h")
	}
	// ~20 m / 10 s = 2 m/s => 7200 m numa hora
	if d := long.BestEfforts["1h"]; d < 6800 || d > 7400 {
		t.Errorf("1h = %d m, esperava ~7200", d)
	}
}

func TestMetricsEmpty(t *testing.T) {
	m := computeMetrics([]Point{{Lat: -3.73, Lon: -38.5, T: 0}}, nil, nil)
	if len(m.Splits) != 0 || m.HasAltitude || m.HasCadence {
		t.Fatalf("traçado de 1 ponto deveria dar métricas vazias: %+v", m)
	}
}
