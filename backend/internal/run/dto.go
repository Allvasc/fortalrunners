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
	ShoeID     string    `json:"shoe_id,omitempty"`
	Points     []Point   `json:"points"`
	Cadence    []Sample  `json:"cadence,omitempty"`    // passos por minuto ao longo do tempo
	HeartRate  []Sample  `json:"heart_rate,omitempty"` // bpm ao longo do tempo
	Weather    any       `json:"weather,omitempty"`

	// Sinais anti-fraude vindos do cliente.
	MockLocation bool   `json:"mock_location,omitempty"` // flag de localização simulada do SO
	Attestation  string `json:"attestation,omitempty"`   // token Play Integrity / App Attest
}

// MetricsView é a projeção de leitura de GET /v1/runs/:id/metrics.
type MetricsView struct {
	RunID         string    `json:"run_id"`
	Splits        []Split   `json:"splits"`
	ElevGainM     int       `json:"elev_gain_m"`
	ElevLossM     int       `json:"elev_loss_m"`
	AltMinM       *float64  `json:"alt_min_m,omitempty"`
	AltMaxM       *float64  `json:"alt_max_m,omitempty"`
	AvgCadenceSPM *int      `json:"avg_cadence_spm,omitempty"`
	MaxCadenceSPM *int      `json:"max_cadence_spm,omitempty"`
	AvgHRBPM      *int      `json:"avg_hr_bpm,omitempty"`
	MaxHRBPM      *int      `json:"max_hr_bpm,omitempty"`
	BestKmPaceS   *int      `json:"best_km_pace_s,omitempty"`
	GradeAdjPaceS *int      `json:"grade_adjusted_pace_s,omitempty"`
	HasAltitude   bool      `json:"has_altitude"`
	HasCadence    bool      `json:"has_cadence"`
	HasHR         bool      `json:"has_heart_rate"`
	ComputedAt    time.Time `json:"computed_at"`
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
