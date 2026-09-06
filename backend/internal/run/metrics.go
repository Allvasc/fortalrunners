package run

import (
	"math"
	"sort"
)

// Parâmetros de cálculo de precisão.
const (
	splitMeters   = 1000.0 // 1 km
	minElevStepM  = 1.0    // ruído de barômetro abaixo disso não conta
	movingSpeedMS = 0.5    // abaixo disso: parado (não conta em "moving")
	minSplitM     = 50.0   // não emite um split final ridiculamente curto
	minGradient   = -0.45  // limites do modelo de Minetti
	maxGradient   = 0.45
	flatCostJKgM  = 3.6 // custo de correr no plano (Minetti et al. 2002)
)

// Sample é uma amostra de sensor no tempo (t = unix ms).
type Sample struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// Split é o resumo de um quilômetro (o último pode ser parcial).
type Split struct {
	Index      int     `json:"index"`
	DistanceM  float64 `json:"distance_m"`
	ElapsedS   float64 `json:"elapsed_s"`
	MovingS    float64 `json:"moving_s"`
	PaceS      float64 `json:"pace_s_per_km"`
	ElevGainM  float64 `json:"elev_gain_m"`
	ElevLossM  float64 `json:"elev_loss_m"`
	AvgCadence float64 `json:"avg_cadence_spm,omitempty"`
	AvgHR      float64 `json:"avg_hr_bpm,omitempty"`
}

// Metrics é o pacote de precisão de uma corrida.
type Metrics struct {
	Splits        []Split `json:"splits"`
	ElevGainM     float64 `json:"elev_gain_m"`
	ElevLossM     float64 `json:"elev_loss_m"`
	AltMinM       float64 `json:"alt_min_m"`
	AltMaxM       float64 `json:"alt_max_m"`
	AvgCadenceSPM float64 `json:"avg_cadence_spm"`
	MaxCadenceSPM float64 `json:"max_cadence_spm"`
	AvgHRBPM      float64 `json:"avg_hr_bpm"`
	MaxHRBPM      float64 `json:"max_hr_bpm"`
	BestKmPaceS   float64 `json:"best_km_pace_s"`
	GradeAdjPaceS float64 `json:"grade_adjusted_pace_s"`
	StepCount     int     `json:"step_count"`
	HasAltitude   bool    `json:"has_altitude"`
	HasCadence    bool    `json:"has_cadence"`
	HasHR         bool    `json:"has_heart_rate"`
}

// computeMetrics deriva splits, elevação suavizada, cadência, FC e pace ajustado
// ao aclive a partir do traçado JÁ limpo (clean()) e dos streams de sensor.
func computeMetrics(pts []Point, cadence, hr []Sample) Metrics {
	var m Metrics
	if len(pts) < 2 {
		return m
	}
	sort.Slice(cadence, func(i, j int) bool { return cadence[i].T < cadence[j].T })
	sort.Slice(hr, func(i, j int) bool { return hr[i].T < hr[j].T })
	m.HasCadence = len(cadence) > 0
	m.HasHR = len(hr) > 0

	// Altitude: min/max e ganho/perda com histerese contra ruído barométrico.
	haveAlt := false
	var refAlt float64
	for _, p := range pts {
		if p.Alt == nil {
			continue
		}
		a := *p.Alt
		if !haveAlt {
			haveAlt, refAlt = true, a
			m.AltMinM, m.AltMaxM = a, a
			continue
		}
		m.AltMinM = math.Min(m.AltMinM, a)
		m.AltMaxM = math.Max(m.AltMaxM, a)
		if d := a - refAlt; d >= minElevStepM {
			m.ElevGainM += d
			refAlt = a
		} else if d <= -minElevStepM {
			m.ElevLossM += -d
			refAlt = a
		}
	}
	m.HasAltitude = haveAlt

	// Percurso segmento a segmento: distância, tempo, "moving", custo de Minetti.
	var totalDist, totalMoving, weightedCost float64
	splitStartT := pts[0].T
	var split Split
	split.Index = 1

	flush := func(atT int64) {
		split.ElapsedS = float64(atT-splitStartT) / 1000.0
		if split.MovingS == 0 {
			split.MovingS = split.ElapsedS
		}
		if split.DistanceM > 0 {
			split.PaceS = split.MovingS / (split.DistanceM / 1000.0)
		}
		if n := meanInWindow(cadence, splitStartT, atT); n > 0 {
			split.AvgCadence = n
		}
		if n := meanInWindow(hr, splitStartT, atT); n > 0 {
			split.AvgHR = n
		}
		m.Splits = append(m.Splits, split)
		split = Split{Index: split.Index + 1}
		splitStartT = atT
	}

	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		d := haversine(a.Lat, a.Lon, b.Lat, b.Lon)
		dt := float64(b.T-a.T) / 1000.0
		if dt <= 0 {
			continue
		}
		totalDist += d
		if d/dt >= movingSpeedMS {
			totalMoving += dt
			split.MovingS += dt
		}
		// custo de Minetti ponderado pela distância (para o GAP)
		if a.Alt != nil && b.Alt != nil && d > 1 {
			grade := clamp((*b.Alt-*a.Alt)/d, minGradient, maxGradient)
			weightedCost += minettiCost(grade) * d
		} else {
			weightedCost += flatCostJKgM * d
		}
		elevSeg(a, b, &split)

		// fecha um ou mais splits se cruzou a fronteira de km
		for split.DistanceM+d >= splitMeters {
			remain := splitMeters - split.DistanceM
			frac := remain / d
			crossT := a.T + int64(frac*float64(b.T-a.T))
			split.DistanceM = splitMeters
			flush(crossT)
			d -= remain
			a.T = crossT
		}
		split.DistanceM += d
	}

	// último split parcial
	if split.DistanceM >= minSplitM {
		flush(pts[len(pts)-1].T)
	} else if n := len(m.Splits); n > 0 {
		// funde o resto no split anterior
		last := &m.Splits[n-1]
		last.DistanceM += split.DistanceM
		last.ElevGainM += split.ElevGainM
		last.ElevLossM += split.ElevLossM
		last.MovingS += split.MovingS
		last.ElapsedS = float64(pts[len(pts)-1].T-pts[0].T)/1000.0 - sumElapsed(m.Splits[:n-1])
		if last.DistanceM > 0 {
			last.PaceS = last.MovingS / (last.DistanceM / 1000.0)
		}
	}

	// resumos
	m.BestKmPaceS = bestFullKmPace(m.Splits)
	if totalMoving > 0 && totalDist > 0 {
		avgSpeed := totalDist / totalMoving
		avgCost := weightedCost / totalDist
		gapSpeed := avgSpeed * (avgCost / flatCostJKgM)
		if gapSpeed > 0 {
			m.GradeAdjPaceS = 1000.0 / gapSpeed
		}
	}
	m.AvgCadenceSPM, m.MaxCadenceSPM = meanMax(cadence)
	m.AvgHRBPM, m.MaxHRBPM = meanMax(hr)
	if m.HasCadence && totalMoving > 0 {
		m.StepCount = int(math.Round(m.AvgCadenceSPM / 60.0 * totalMoving))
	}
	return m
}

func elevSeg(a, b Point, s *Split) {
	if a.Alt == nil || b.Alt == nil {
		return
	}
	if d := *b.Alt - *a.Alt; d >= minElevStepM {
		s.ElevGainM += d
	} else if d <= -minElevStepM {
		s.ElevLossM += -d
	}
}

// minettiCost devolve o custo energético de correr (J/kg/m) para um gradiente i,
// polinômio de Minetti et al. (J. Appl. Physiol. 2002).
func minettiCost(i float64) float64 {
	return 155.4*math.Pow(i, 5) - 30.4*math.Pow(i, 4) - 43.3*math.Pow(i, 3) +
		46.3*i*i + 19.5*i + flatCostJKgM
}

func meanInWindow(s []Sample, from, to int64) float64 {
	var sum float64
	var n int
	for _, x := range s {
		if x.T >= from && x.T <= to {
			sum += x.V
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func meanMax(s []Sample) (mean, max float64) {
	if len(s) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range s {
		sum += x.V
		max = math.Max(max, x.V)
	}
	return sum / float64(len(s)), max
}

func bestFullKmPace(splits []Split) float64 {
	best := math.MaxFloat64
	for _, s := range splits {
		if s.DistanceM >= splitMeters-1 && s.PaceS > 0 && s.PaceS < best {
			best = s.PaceS
		}
	}
	if best == math.MaxFloat64 {
		return 0
	}
	return best
}

func sumElapsed(splits []Split) float64 {
	var t float64
	for _, s := range splits {
		t += s.ElapsedS
	}
	return t
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
