package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("corrida não encontrada")

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

// privacyZone lê a zona de ocultação do usuário de users.privacy_jsonb
// ({ home_lat, home_lng, home_blur_m }). ok=false quando não configurada.
func (s *store) privacyZone(ctx context.Context, userID string) (lat, lng, radiusM float64, ok bool) {
	var la, ln, r *float64
	err := s.pool.QueryRow(ctx, `
		SELECT (privacy_jsonb->>'home_lat')::float8,
		       (privacy_jsonb->>'home_lng')::float8,
		       (privacy_jsonb->>'home_blur_m')::float8
		FROM users WHERE id = $1`, userID).Scan(&la, &ln, &r)
	if err != nil || la == nil || ln == nil || r == nil || *r <= 0 {
		return 0, 0, 0, false
	}
	return *la, *ln, *r, true
}

type createArgs struct {
	ID         string
	UserID     string
	In         IngestInput
	Clean      cleaned
	Metrics    Metrics
	FraudScore float64
	FraudFlags []string
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
	m := a.Metrics
	// elevação e cadência vêm das métricas de precisão (suavizadas / do stream).
	var cadence, maxCad, steps *int
	if m.HasCadence {
		cadence = intPtr(true, m.AvgCadenceSPM)
		maxCad = intPtr(true, m.MaxCadenceSPM)
		if m.StepCount > 0 {
			steps = &m.StepCount
		}
	}

	const insRun = `
		INSERT INTO runs (id, user_id, shoe_id, started_at, ended_at, distance_m, moving_s, duration_s,
		                  avg_pace_s, gap_pace_s, best_km_pace_s, elevation_gain_m, elev_loss_m, alt_min_m, alt_max_m,
		                  avg_cadence_spm, max_cadence_spm, step_count, avg_hr, max_hr,
		                  gnss_mode, avg_hdop, data_source, weather_jsonb, fraud_score, fraud_flags, import_ref, status, calories)
		VALUES ($1,$2,nullif($3,''),$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
		        $21,$22,$23::run_source,$24,$25,$26,nullif($27,''),$28::run_status,nullif($29,0))
		RETURNING id, user_id, started_at, ended_at, distance_m, moving_s, duration_s,
		          avg_pace_s, elevation_gain_m, data_source::text, territory_area_m2,
		          new_blocks, status::text, created_at`
	weather, _ := json.Marshal(a.In.Weather)
	flags, _ := json.Marshal(a.FraudFlags)
	if len(flags) == 0 || string(flags) == "null" {
		flags = []byte("[]")
	}
	var v View
	err = tx.QueryRow(ctx, insRun,
		a.ID, a.UserID, a.In.ShoeID, a.In.StartedAt, a.In.EndedAt,
		int(a.Clean.distM), int(a.Clean.movingS), int(a.Clean.durS),
		pace, intPtr(m.GradeAdjPaceS > 0, m.GradeAdjPaceS), intPtr(m.BestKmPaceS > 0, m.BestKmPaceS),
		int(math.Round(m.ElevGainM)), int(math.Round(m.ElevLossM)),
		altPtr(m.HasAltitude, m.AltMinM), altPtr(m.HasAltitude, m.AltMaxM),
		cadence, maxCad, steps, intPtr(m.HasHR, m.AvgHRBPM), intPtr(m.HasHR, m.MaxHRBPM),
		nullStr(a.In.GNSSMode), a.In.AvgHDOP, src, weather, a.FraudScore, flags, a.In.ImportRef, a.Status, m.CaloriesEst,
	).Scan(&v.ID, &v.UserID, &v.StartedAt, &v.EndedAt, &v.DistanceM, &v.MovingS, &v.DurationS,
		&v.AvgPaceS, &v.ElevationGainM, &v.DataSource, &v.TerritoryAreaM2, &v.NewBlocks, &v.Status, &v.CreatedAt)
	if err != nil {
		return View{}, fmt.Errorf("run: insert: %w", err)
	}

	ptsJSON, _ := json.Marshal(a.Clean.points)
	splitsJSON, _ := json.Marshal(m.Splits)
	effortsJSON, _ := json.Marshal(m.BestEfforts)
	if len(effortsJSON) == 0 || string(effortsJSON) == "null" {
		effortsJSON = []byte("{}")
	}
	var streams []byte
	if len(a.In.Cadence) > 0 || len(a.In.HeartRate) > 0 {
		streams, _ = json.Marshal(map[string]any{"cadence": a.In.Cadence, "heart_rate": a.In.HeartRate})
	}
	// LineStringZ: (lon, lat, alt) — altitude 0 quando o ponto não a traz.
	const insTrack = `
		INSERT INTO run_tracks (run_id, started_at, geom, points_jsonb, sensor_streams_jsonb, splits_jsonb, best_efforts_jsonb)
		VALUES (
			$1, $2,
			ST_SetSRID(ST_MakeLine(ARRAY(
				SELECT ST_MakePoint((p->>'lon')::float8, (p->>'lat')::float8, COALESCE((p->>'alt')::float8, 0))
				FROM jsonb_array_elements($3::jsonb) WITH ORDINALITY AS e(p, ord)
				ORDER BY ord
			)), 4326),
			$3, $4, $5, $6
		)`
	if _, err := tx.Exec(ctx, insTrack, a.ID, a.In.StartedAt, ptsJSON, streams, splitsJSON, effortsJSON); err != nil {
		return View{}, fmt.Errorf("run: insert track: %w", err)
	}

	return v, tx.Commit(ctx)
}

func (s *store) metrics(ctx context.Context, id, userID string) (MetricsView, error) {
	const q = `
		SELECT r.id, rt.splits_jsonb,
		       r.elevation_gain_m, r.elev_loss_m, r.alt_min_m, r.alt_max_m,
		       r.avg_cadence_spm, r.max_cadence_spm, r.avg_hr, r.max_hr,
		       r.best_km_pace_s, r.gap_pace_s,
		       (r.alt_min_m IS NOT NULL) AS has_alt,
		       (r.avg_cadence_spm IS NOT NULL) AS has_cad,
		       (r.avg_hr IS NOT NULL) AS has_hr,
		       r.created_at
		FROM runs r JOIN run_tracks rt ON rt.run_id = r.id
		WHERE r.id = $1 AND r.user_id = $2`
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
	if len(splitsRaw) > 0 {
		if err := json.Unmarshal(splitsRaw, &mv.Splits); err != nil {
			return mv, fmt.Errorf("run: splits: %w", err)
		}
	}
	return mv, nil
}

func altPtr(ok bool, v float64) *float64 {
	if !ok {
		return nil
	}
	r := math.Round(v*10) / 10
	return &r
}

func intPtr(ok bool, v float64) *int {
	if !ok {
		return nil
	}
	i := int(math.Round(v))
	return &i
}

// duplicateImport: true se a corrida importada já existe (mesma ref) ou colide
// no tempo (±5 min do início) com uma corrida nativa do mesmo usuário.
func (s *store) duplicateImport(ctx context.Context, userID, importRef string, startedAt time.Time) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM runs
			WHERE user_id = $1
			  AND (import_ref = $2
			       OR (import_ref IS NULL AND abs(extract(epoch FROM started_at - $3)) < 300))
		)`, userID, importRef, startedAt).Scan(&exists)
	return exists, err
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
