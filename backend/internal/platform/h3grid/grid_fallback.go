//go:build !h3

package h3grid

import (
	"fmt"
	"math"
)

// Enabled: H3 real NÃO está compilado. Grade lat/lng stand-in.
const Enabled = false

// covDeg ~ 0,00055° ≈ 61 m no equador — entre res 9 e 10, como o stand-in antigo.
const covDeg = 0.00055

func init() { Coverage = degGrid{cell: covDeg} }

type degGrid struct{ cell float64 }

func (g degGrid) Tag() string { return fmt.Sprintf("deg:%g", g.cell) }

func (g degGrid) CellAt(lat, lng float64) Cell {
	return g.cellOf(int(math.Floor(lng/g.cell)), int(math.Floor(lat/g.cell)))
}

func (g degGrid) cellOf(gx, gy int) Cell {
	return Cell{
		ID:        fmt.Sprintf("g:%d:%d", gx, gy),
		CenterLng: (float64(gx) + 0.5) * g.cell,
		CenterLat: (float64(gy) + 0.5) * g.cell,
	}
}

func (g degGrid) CellsInBBox(minLat, minLng, maxLat, maxLng float64) []Cell {
	x0 := int(math.Floor(minLng / g.cell))
	x1 := int(math.Ceil(maxLng / g.cell))
	y0 := int(math.Floor(minLat / g.cell))
	y1 := int(math.Ceil(maxLat / g.cell))
	out := make([]Cell, 0, (x1-x0+1)*(y1-y0+1))
	for gy := y0; gy <= y1; gy++ {
		for gx := x0; gx <= x1; gx++ {
			out = append(out, g.cellOf(gx, gy))
		}
	}
	return out
}
