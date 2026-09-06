package run

import "time"

// Point é uma amostra de GPS vinda do cliente. t = unix milissegundos.
type Point struct {
	Lat float64  `json:"lat"`
	Lon float64  `json:"lon"`
	Alt *float64 `json:"alt,omitempty"`
	T   int64    `json:"t"`
	Acc *float64 `json:"acc,omitempty"` // precisão horizontal em metros
	Spd *float64 `json:"spd,omitempty"` // m/s
}

// IngestInput é o corpo de POST /v1/runs.
type IngestInput struct {
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	DataSource string    `json:"data_source"` // phone | watch | import
	GNSSMode   string    `json:"gnss_mode"`
	AvgHDOP    *float64  `json:"avg_hdop,omitempty"`
	Points     []Point   `json:"points"`
	Weather    any       `json:"weather,omitempty"`
}

// View é a projeção de leitura de uma corrida.
type View struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	DistanceM       int       `json:"distance_m"`
	MovingS         int       `json:"moving_s"`
	DurationS       int       `json:"duration_s"`
	AvgPaceS        int       `json:"avg_pace_s"`
	ElevationGainM  int       `json:"elevation_gain_m"`
	DataSource      string    `json:"data_source"`
	TerritoryAreaM2 float64   `json:"territory_area_m2"`
	NewBlocks       int       `json:"new_blocks"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}
