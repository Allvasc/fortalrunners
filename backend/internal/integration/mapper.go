package integration

import (
	"time"

	"github.com/Allvasc/fortalrunners/backend/internal/run"
)

// tipos de atividade do Strava que viram corrida no FortalRunners.
var runLike = map[string]bool{"Run": true, "TrailRun": true, "Walk": true, "Hike": true}

// mapActivity converte uma atividade do Strava + seus streams num IngestInput.
// Devolve ok=false quando não é uma corrida a pé ou não tem GPS suficiente.
func mapActivity(a stravaActivity, st stravaStreams) (run.IngestInput, bool) {
	if !runLike[a.Type] && !runLike[a.SportType] {
		return run.IngestInput{}, false
	}
	if a.Manual || len(st.LatLng) < 2 || len(st.Time) != len(st.LatLng) {
		return run.IngestInput{}, false
	}
	start, err := time.Parse(time.RFC3339, a.StartDate)
	if err != nil {
		return run.IngestInput{}, false
	}
	base := start.UnixMilli()

	pts := make([]run.Point, 0, len(st.LatLng))
	for i, ll := range st.LatLng {
		if len(ll) != 2 {
			continue
		}
		p := run.Point{Lat: ll[0], Lon: ll[1], T: base + int64(st.Time[i])*1000}
		if i < len(st.Altitude) {
			alt := st.Altitude[i]
			p.Alt = &alt
		}
		pts = append(pts, p)
	}
	if len(pts) < 2 {
		return run.IngestInput{}, false
	}

	return run.IngestInput{
		StartedAt:  start,
		EndedAt:    start.Add(time.Duration(a.Elapsed) * time.Second),
		DataSource: "import",
		GNSSMode:   "import",
		Points:     pts,
	}, true
}
