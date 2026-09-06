package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("corrida não encontrada")

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

type createArgs struct {
	ID         string
	UserID     string
	In         IngestInput
	Clean      cleaned
	Metrics    Metrics
	FraudScore float64
	Status     string
}

// create insere runs + run_tracks numa transação. A geometria da linha é
// montada no Postgres a partir do array de pontos limpos.
func (s *store) create(ctx context.Context, a createArgs) (View, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return View{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	src := a.In.DataSource
	if src != "phone" && src != "watch" && src != "import" {
		src = "phone"
	}

	pace := 0
	if a.Clean.distM > 0 {
		pace = int(a.Clean.movingS / (a.Clean.distM / 1000.0))
	}
	// A elevação e a cadência vêm das métricas de precisão (suavizadas / do stream).
	elevGain := int(math.Round(a.Metrics.ElevGainM))
	var cadence, steps *int
	if a.Metrics.HasCadence {
		c := int(math.Round(a.Metrics.AvgCadenceSPM))
		cadence = &c
		if a.Metrics.StepCount > 0 {
			steps = &a.Metrics.StepCount
		}
	}

	const insRun = `
		INSERT INTO runs (id, user_id, shoe_id, started_at, ended_at, distance_m, moving_s, duration_s,
		                  avg_pace_s, elevation_gain_m, avg_cadence_spm, step_count, gnss_mode, avg_hdop, data_source,
		                  weather_jsonb, fraud_score, status)
		VALUES ($1,$2,nullif($3,''),$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::run_source,$16,$17,$18::run_status)
		RETURNING id, user_id, started_at, ended_at, distance_m, moving_s, duration_s,
		          avg_pace_s, elevation_gain_m, data_source::text, territory_area_m2,
		          new_blocks, status::text, created_at`
	weather, _ := json.Marshal(a.In.Weather)
	var v View
	err = tx.QueryRow(ctx, insRun,
		a.ID, a.UserID, a.In.ShoeID, a.In.StartedAt, a.In.EndedAt,
		int(a.Clean.distM), int(a.Clean.movingS), int(a.Clean.durS),
		pace, elevGain, cadence, steps, nullStr(a.In.GNSSMode), a.In.AvgHDOP, src,
		weather, a.FraudScore, a.Status,
	).Scan(&v.ID, &v.UserID, &v.StartedAt, &v.EndedAt, &v.DistanceM, &v.MovingS, &v.DurationS,
		&v.AvgPaceS, &v.ElevationGainM, &v.DataSource, &v.TerritoryAreaM2, &v.NewBlocks, &v.Status, &v.CreatedAt)
	if err != nil {
		return View{}, fmt.Errorf("run: insert: %w", err)
	}

	ptsJSON, _ := json.Marshal(a.Clean.points)
	var streams []byte
	if len(a.In.Cadence) > 0 || len(a.In.HeartRate) > 0 {
		streams, _ = json.Marshal(map[string]any{"cadence": a.In.Cadence, "heart_rate": a.In.HeartRate})
	}
	const insTrack = `
		INSERT INTO run_tracks (run_id, geom, points_jsonb, sensor_streams_jsonb)
		VALUES (
			$1,
			ST_MakeLine(ARRAY(
				SELECT ST_SetSRID(ST_MakePoint((p->>'lon')::float8, (p->>'lat')::float8), 4326)
				FROM jsonb_array_elements($2::jsonb) WITH ORDINALITY AS e(p, ord)
				ORDER BY ord
			)),
			$2, $3
		)`
	if _, err := tx.Exec(ctx, insTrack, a.ID, ptsJSON, streams); err != nil {
		return View{}, fmt.Errorf("run: insert track: %w", err)
	}

	if err := insertMetrics(ctx, tx, a.ID, a.Metrics); err != nil {
		return View{}, fmt.Errorf("run: insert metrics: %w", err)
	}

	return v, tx.Commit(ctx)
}

func insertMetrics(ctx context.Context, tx pgx.Tx, runID string, m Metrics) error {
	splits, _ := json.Marshal(m.Splits)
	const q = `
		INSERT INTO run_metrics (run_id, splits_jsonb, elev_gain_m, elev_loss_m, alt_min_m, alt_max_m,
		    avg_cadence_spm, max_cadence_spm, avg_hr_bpm, max_hr_bpm, best_km_pace_s, grade_adjusted_pace_s,
		    has_altitude, has_cadence, has_heart_rate)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`
	_, err := tx.Exec(ctx, q, runID, splits,
		round1(m.ElevGainM), round1(m.ElevLossM),
		altPtr(m.HasAltitude, m.AltMinM), altPtr(m.HasAltitude, m.AltMaxM),
		intPtr(m.HasCadence, m.AvgCadenceSPM), intPtr(m.HasCadence, m.MaxCadenceSPM),
		intPtr(m.HasHR, m.AvgHRBPM), intPtr(m.HasHR, m.MaxHRBPM),
		intPtr(m.BestKmPaceS > 0, m.BestKmPaceS), intPtr(m.GradeAdjPaceS > 0, m.GradeAdjPaceS),
		m.HasAltitude, m.HasCadence, m.HasHR)
	return err
}

func (s *store) metrics(ctx context.Context, id, userID string) (MetricsView, error) {
	const q = `
		SELECT m.run_id, m.splits_jsonb, m.elev_gain_m, m.elev_loss_m, m.alt_min_m, m.alt_max_m,
		       m.avg_cadence_spm, m.max_cadence_spm, m.avg_hr_bpm, m.max_hr_bpm,
		       m.best_km_pace_s, m.grade_adjusted_pace_s,
		       m.has_altitude, m.has_cadence, m.has_heart_rate, m.computed_at
		FROM run_metrics m JOIN runs r ON r.id = m.run_id
		WHERE m.run_id = $1 AND r.user_id = $2`
	var mv MetricsView
	var splitsRaw []byte
	err := s.pool.QueryRow(ctx, q, id, userID).Scan(
		&mv.RunID, &splitsRaw, &mv.ElevGainM, &mv.ElevLossM, &mv.AltMinM, &mv.AltMaxM,
		&mv.AvgCadenceSPM, &mv.MaxCadenceSPM, &mv.AvgHRBPM, &mv.MaxHRBPM,
		&mv.BestKmPaceS, &mv.GradeAdjPaceS,
		&mv.HasAltitude, &mv.HasCadence, &mv.HasHR, &mv.ComputedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return mv, ErrNotFound
	}
	if err != nil {
		return mv, err
	}
	if err := json.Unmarshal(splitsRaw, &mv.Splits); err != nil {
		return mv, fmt.Errorf("run: splits: %w", err)
	}
	return mv, nil
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func altPtr(ok bool, v float64) *float64 {
	if !ok {
		return nil
	}
	r := round1(v)
	return &r
}

func intPtr(ok bool, v float64) *int {
	if !ok {
		return nil
	}
	i := int(math.Round(v))
	return &i
}

func (s *store) get(ctx context.Context, id, userID string) (View, error) {
	const q = `
		SELECT id, user_id, started_at, ended_at, distance_m, moving_s, duration_s,
		       avg_pace_s, elevation_gain_m, data_source::text, territory_area_m2,
		       new_blocks, status::text, created_at
		FROM runs WHERE id = $1 AND user_id = $2`
	return scanView(s.pool.QueryRow(ctx, q, id, userID))
}

func (s *store) list(ctx context.Context, userID string, limit int) ([]View, error) {
	const q = `
		SELECT id, user_id, started_at, ended_at, distance_m, moving_s, duration_s,
		       avg_pace_s, elevation_gain_m, data_source::text, territory_area_m2,
		       new_blocks, status::text, created_at
		FROM runs WHERE user_id = $1 ORDER BY started_at DESC LIMIT $2`
	rows, err := s.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []View{}
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanView(row pgx.Row) (View, error) {
	var v View
	err := row.Scan(&v.ID, &v.UserID, &v.StartedAt, &v.EndedAt, &v.DistanceM, &v.MovingS, &v.DurationS,
		&v.AvgPaceS, &v.ElevationGainM, &v.DataSource, &v.TerritoryAreaM2, &v.NewBlocks, &v.Status, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
