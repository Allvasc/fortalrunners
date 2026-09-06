// Package h3grid é a grade de células para cobertura de território ("% de
// Fortaleza") e o denominador por bairro.
//
// Com o build tag "h3" (produção), usa H3 real (res 9, hexágonos). Sem o tag
// (dev/CI padrão, sem cgo), usa uma grade lat/lng equatorial como stand-in —
// em Fortaleza (-3,7°) a distorção de área é ~0,2%.
//
// Trocar de implementação muda Coverage.Tag(); o backend detecta a mudança no
// boot e repovoa h3_cells + neighborhoods.h3_total.
package h3grid

// Cell é uma célula da grade: id opaco e estável, mais o centro em graus.
type Cell struct {
	ID        string
	CenterLat float64
	CenterLng float64
}

// Grid é a grade de cobertura.
type Grid interface {
	// CellAt devolve a célula que contém o ponto.
	CellAt(lat, lng float64) Cell
	// CellsInBBox devolve as células cujo centro cai na caixa (graus). O chamador
	// ainda filtra pelo polígono real no SQL (ST_Contains).
	CellsInBBox(minLat, minLng, maxLat, maxLng float64) []Cell
	// Tag identifica a implementação + resolução — ex.: "h3:9" ou "deg:0.00055".
	Tag() string
}

// Coverage é a grade ativa. Definida em grid_h3.go ou grid_fallback.go conforme
// o build tag.
var Coverage Grid
