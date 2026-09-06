//go:build h3

package h3grid

import (
	"fmt"

	h3 "github.com/uber/h3-go/v4"
)

// Enabled: H3 real (cgo) está compilado.
const Enabled = true

// covResolution 9 ≈ aresta de 174 m, área de 0,105 km² — a "grade H3 res 9 por
// corredor" do plano.
const covResolution = 9

func init() { Coverage = h3Grid{res: covResolution} }

type h3Grid struct{ res int }

func (g h3Grid) Tag() string { return fmt.Sprintf("h3:%d", g.res) }

func (g h3Grid) cell(c h3.Cell) Cell {
	ll, err := h3.CellToLatLng(c)
	if err != nil {
		return Cell{ID: c.String()}
	}
	return Cell{ID: c.String(), CenterLat: ll.Lat, CenterLng: ll.Lng}
}

func (g h3Grid) CellAt(lat, lng float64) Cell {
	c, err := h3.LatLngToCell(h3.LatLng{Lat: lat, Lng: lng}, g.res)
	if err != nil {
		return Cell{}
	}
	return g.cell(c)
}

func (g h3Grid) CellsInBBox(minLat, minLng, maxLat, maxLng float64) []Cell {
	loop := h3.GeoLoop{
		{Lat: minLat, Lng: minLng},
		{Lat: minLat, Lng: maxLng},
		{Lat: maxLat, Lng: maxLng},
		{Lat: maxLat, Lng: minLng},
	}
	cells, err := h3.PolygonToCells(h3.GeoPolygon{GeoLoop: loop}, g.res)
	if err != nil {
		return nil
	}
	out := make([]Cell, 0, len(cells))
	for _, c := range cells {
		out = append(out, g.cell(c))
	}
	return out
}
