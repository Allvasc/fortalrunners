package run

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ExportData é o mínimo para gerar GPX/TCX de uma corrida.
type ExportData struct {
	RunID     string
	StartedAt time.Time
	Sport     string
	Points    []Point
}

func (s *store) exportData(ctx context.Context, id, userID string) (ExportData, error) {
	var e ExportData
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT r.id, r.started_at, COALESCE(rt.points_jsonb, '[]'::jsonb)
		FROM runs r JOIN run_tracks rt ON rt.run_id = r.id
		WHERE r.id = $1 AND r.user_id = $2`, id, userID).Scan(&e.RunID, &e.StartedAt, &raw)
	if err != nil {
		return e, ErrNotFound
	}
	if err := json.Unmarshal(raw, &e.Points); err != nil {
		return e, err
	}
	e.Sport = "Running"
	return e, nil
}

func (s *Service) Export(ctx context.Context, id, userID, format string) (string, string, error) {
	d, err := s.store.exportData(ctx, id, userID)
	if err != nil {
		return "", "", err
	}
	if len(d.Points) == 0 {
		return "", "", ErrTooFewPoints
	}
	switch strings.ToLower(format) {
	case "tcx":
		return buildTCX(d), "application/vnd.garmin.tcx+xml", nil
	default:
		return buildGPX(d), "application/gpx+xml", nil
	}
}

func ts(unixMs int64) string { return time.UnixMilli(unixMs).UTC().Format(time.RFC3339) }

func buildGPX(d ExportData) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<gpx version="1.1" creator="FortalRunners" xmlns="http://www.topografix.com/GPX/1/1">` + "\n")
	b.WriteString("  <trk>\n    <name>Corrida " + d.StartedAt.UTC().Format("2006-01-02") + "</name>\n    <type>running</type>\n    <trkseg>\n")
	for _, p := range d.Points {
		b.WriteString(fmt.Sprintf(`      <trkpt lat="%.6f" lon="%.6f">`, p.Lat, p.Lon))
		if p.Alt != nil {
			b.WriteString(fmt.Sprintf("<ele>%.1f</ele>", *p.Alt))
		}
		b.WriteString("<time>" + ts(p.T) + "</time></trkpt>\n")
	}
	b.WriteString("    </trkseg>\n  </trk>\n</gpx>\n")
	return b.String()
}

func buildTCX(d ExportData) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2">` + "\n")
	b.WriteString("  <Activities>\n    <Activity Sport=\"Running\">\n")
	b.WriteString("      <Id>" + d.StartedAt.UTC().Format(time.RFC3339) + "</Id>\n")
	b.WriteString("      <Lap StartTime=\"" + d.StartedAt.UTC().Format(time.RFC3339) + "\">\n        <Track>\n")
	for _, p := range d.Points {
		b.WriteString("          <Trackpoint>\n")
		b.WriteString("            <Time>" + ts(p.T) + "</Time>\n")
		b.WriteString(fmt.Sprintf("            <Position><LatitudeDegrees>%.6f</LatitudeDegrees><LongitudeDegrees>%.6f</LongitudeDegrees></Position>\n", p.Lat, p.Lon))
		if p.Alt != nil {
			b.WriteString(fmt.Sprintf("            <AltitudeMeters>%.1f</AltitudeMeters>\n", *p.Alt))
		}
		b.WriteString("          </Trackpoint>\n")
	}
	b.WriteString("        </Track>\n      </Lap>\n      <Creator><Name>FortalRunners</Name></Creator>\n")
	b.WriteString("    </Activity>\n  </Activities>\n</TrainingCenterDatabase>\n")
	return b.String()
}
