package run

import (
	"math"
	"sort"
)

const (
	maxAccuracyM   = 30.0 // descarta pontos com pior precisão
	maxSpeedMS     = 12.0 // ~43 km/h entre dois pontos = teleporte
	dedupDistM     = 1.0
	dedupGapS      = 1.0
	minCleanPoints = 2
	minDistanceM   = 50.0
)

type cleaned struct {
	points   []Point
	distM    float64
	movingS  float64
	durS     float64
	elevGain float64
}

// clean ordena, filtra e resume os pontos. Devolve ok=false quando o traçado
// não dá para virar corrida.
func clean(in []Point) (cleaned, bool) {
	if len(in) < minCleanPoints {
		return cleaned{}, false
	}
	pts := append([]Point(nil), in...)
	sort.Slice(pts, func(i, j int) bool { return pts[i].T < pts[j].T })

	out := make([]Point, 0, len(pts))
	var dist, moving, elev float64
	var prev *Point

	for i := range pts {
		p := pts[i]
		if p.Acc != nil && *p.Acc > maxAccuracyM {
			continue
		}
		if prev == nil {
			out = append(out, p)
			prev = &out[len(out)-1]
			continue
		}
		d := haversine(prev.Lat, prev.Lon, p.Lat, p.Lon)
		dt := float64(p.T-prev.T) / 1000.0
		if dt <= 0 {
			continue
		}
		if d < dedupDistM && dt < dedupGapS {
			continue // parado / ruído
		}
		if d/dt > maxSpeedMS {
			continue // salto impossível
		}
		dist += d
		if d/dt > 0.5 { // ~1,8 km/h — considera "em movimento"
			moving += dt
		}
		if prev.Alt != nil && p.Alt != nil {
			if g := *p.Alt - *prev.Alt; g > 0 {
				elev += g
			}
		}
		out = append(out, p)
		prev = &out[len(out)-1]
	}

	if len(out) < minCleanPoints || dist < minDistanceM {
		return cleaned{}, false
	}
	durS := float64(out[len(out)-1].T-out[0].T) / 1000.0
	if moving == 0 {
		moving = durS
	}
	return cleaned{points: out, distM: dist, movingS: moving, durS: durS, elevGain: elev}, true
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	φ1, φ2 := lat1*math.Pi/180, lat2*math.Pi/180
	dφ := (lat2 - lat1) * math.Pi / 180
	dλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
