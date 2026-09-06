package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

	const insRun = `
		INSERT INTO runs (id, user_id, started_at, ended_at, distance_m, moving_s, duration_s,
		                  avg_pace_s, elevation_gain_m, gnss_mode, avg_hdop, data_source,
		                  weather_jsonb, fraud_score, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::run_source,$13,$14,$15::run_status)
		RETURNING id, user_id, started_at, ended_at, distance_m, moving_s, duration_s,
		          avg_pace_s, elevation_gain_m, data_source::text, territory_area_m2,
		          new_blocks, status::text, created_at`
	weather, _ := json.Marshal(a.In.Weather)
	var v View
	err = tx.QueryRow(ctx, insRun,
		a.ID, a.UserID, a.In.StartedAt, a.In.EndedAt,
		int(a.Clean.distM), int(a.Clean.movingS), int(a.Clean.durS),
		pace, int(a.Clean.elevGain), nullStr(a.In.GNSSMode), a.In.AvgHDOP, src,
		weather, a.FraudScore, a.Status,
	).Scan(&v.ID, &v.UserID, &v.StartedAt, &v.EndedAt, &v.DistanceM, &v.MovingS, &v.DurationS,
		&v.AvgPaceS, &v.ElevationGainM, &v.DataSource, &v.TerritoryAreaM2, &v.NewBlocks, &v.Status, &v.CreatedAt)
	if err != nil {
		return View{}, fmt.Errorf("run: insert: %w", err)
	}

	ptsJSON, _ := json.Marshal(a.Clean.points)
	const insTrack = `
		INSERT INTO run_tracks (run_id, geom, points_jsonb)
		VALUES (
			$1,
			ST_MakeLine(ARRAY(
				SELECT ST_SetSRID(ST_MakePoint((p->>'lon')::float8, (p->>'lat')::float8), 4326)
				FROM jsonb_array_elements($2::jsonb) WITH ORDINALITY AS e(p, ord)
				ORDER BY ord
			)),
			$2
		)`
	if _, err := tx.Exec(ctx, insTrack, a.ID, ptsJSON); err != nil {
		return View{}, fmt.Errorf("run: insert track: %w", err)
	}

	return v, tx.Commit(ctx)
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
