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
	points    []Point
	matched   []Point // traçado encaixado na malha viária (OSRM) — vazio quando desligado/sem match
	distM     float64
	movingS   float64
	durS      float64
	elevGain  float64
	teleports int     // pontos descartados por salto impossível (sinal de spoofing)
	rawKept   int     // pontos que passaram no filtro de precisão
	minLat    float64 // caixa envolvente — para o teste de "traçado reto demais"
	minLon    float64
	maxLat    float64
	maxLon    float64
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
	var teleports, kept int
	var prev *Point
	c := cleaned{minLat: 90, minLon: 180, maxLat: -90, maxLon: -180}

	for i := range pts {
		p := pts[i]
		if p.Acc != nil && *p.Acc > maxAccuracyM {
			continue
		}
		kept++
		c.minLat, c.maxLat = math.Min(c.minLat, p.Lat), math.Max(c.maxLat, p.Lat)
		c.minLon, c.maxLon = math.Min(c.minLon, p.Lon), math.Max(c.maxLon, p.Lon)
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
			teleports++
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
	c.points, c.distM, c.movingS, c.durS, c.elevGain = out, dist, moving, durS, elev
	c.teleports, c.rawKept = teleports, kept
	return c, true
}

// bboxDiagonalM devolve a diagonal da caixa envolvente do traçado, em metros.
func (c cleaned) bboxDiagonalM() float64 {
	return haversine(c.minLat, c.minLon, c.maxLat, c.maxLon)
}

// dropWithinRadius remove os pontos que caem dentro de um círculo (zona de
// ocultação de casa/trabalho — plano §14).
func dropWithinRadius(pts []Point, lat, lng, radiusM float64) []Point {
	out := pts[:0]
	for _, p := range pts {
		if haversine(lat, lng, p.Lat, p.Lon) > radiusM {
			out = append(out, p)
		}
	}
	return out
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	φ1, φ2 := lat1*math.Pi/180, lat2*math.Pi/180
	dφ := (lat2 - lat1) * math.Pi / 180
	dλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
