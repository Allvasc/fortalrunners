// Package stats faz o rollup pós-corrida: quilometragem por tênis, acumulado
// vitalício (com streak) e recordes pessoais. Roda no worker, depois do
// processamento de território. Idempotente via runs.stats_rolled_at.
package stats

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

type Rollup struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewRollup(pool *pgxpool.Pool, log *slog.Logger) *Rollup {
	return &Rollup{pool: pool, log: log}
}

func (r *Rollup) Run(ctx context.Context, runID string) {
	log := r.log.With("run_id", runID)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		log.Error("stats: begin", "err", err)
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// trava a corrida e garante que ainda não foi contada
	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id FROM runs
		WHERE id = $1 AND status = 'valid' AND stats_rolled_at IS NULL
		FOR UPDATE`, runID).Scan(&userID)
	if err != nil {
		return // já contada, rejeitada, ou não existe — nada a fazer
	}

	if _, err = tx.Exec(ctx, lifetimeUpsert, runID); err != nil {
		log.Error("stats: lifetime", "err", err)
		return
	}
	if _, err = tx.Exec(ctx, shoeUpdate, runID); err != nil {
		log.Error("stats: shoe", "err", err)
		return
	}
	for _, pr := range []struct{ key, expr string }{
		{"longest_distance", "distance_m"},
		{"max_elevation", "elevation_gain_m"},
		{"biggest_territory", "territory_area_m2"},
	} {
		if _, err = tx.Exec(ctx, prUpsert(pr.expr), id.New(), runID, pr.key); err != nil {
			log.Error("stats: pr", "key", pr.key, "err", err)
			return
		}
	}

	if _, err = tx.Exec(ctx, `UPDATE runs SET stats_rolled_at = now() WHERE id = $1`, runID); err != nil {
		log.Error("stats: mark", "err", err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		log.Error("stats: commit", "err", err)
		return
	}
	log.Info("stats: rollup concluído", "user_id", userID)
}

const lifetimeUpsert = `
INSERT INTO lifetime_stats (user_id, total_moving_s, total_distance_m, total_steps,
    run_count, elevation_gain_m, territory_area_m2, active_days,
    current_streak_days, longest_streak_days, last_run_date, updated_at)
SELECT r.user_id, r.moving_s, r.distance_m, COALESCE(r.step_count,0), 1, r.elevation_gain_m,
       r.territory_area_m2, 1, 1, 1,
       (r.started_at AT TIME ZONE 'America/Fortaleza')::date, now()
FROM runs r WHERE r.id = $1
ON CONFLICT (user_id) DO UPDATE SET
    total_moving_s   = lifetime_stats.total_moving_s   + EXCLUDED.total_moving_s,
    total_distance_m = lifetime_stats.total_distance_m + EXCLUDED.total_distance_m,
    total_steps      = lifetime_stats.total_steps      + EXCLUDED.total_steps,
    run_count        = lifetime_stats.run_count + 1,
    elevation_gain_m = lifetime_stats.elevation_gain_m + EXCLUDED.elevation_gain_m,
    territory_area_m2 = lifetime_stats.territory_area_m2 + EXCLUDED.territory_area_m2,
    active_days = lifetime_stats.active_days +
        CASE WHEN lifetime_stats.last_run_date IS DISTINCT FROM EXCLUDED.last_run_date THEN 1 ELSE 0 END,
    current_streak_days = CASE
        WHEN lifetime_stats.last_run_date = EXCLUDED.last_run_date     THEN lifetime_stats.current_streak_days
        WHEN lifetime_stats.last_run_date = EXCLUDED.last_run_date - 1 THEN lifetime_stats.current_streak_days + 1
        ELSE 1 END,
    longest_streak_days = GREATEST(lifetime_stats.longest_streak_days, CASE
        WHEN lifetime_stats.last_run_date = EXCLUDED.last_run_date     THEN lifetime_stats.current_streak_days
        WHEN lifetime_stats.last_run_date = EXCLUDED.last_run_date - 1 THEN lifetime_stats.current_streak_days + 1
        ELSE 1 END),
    last_run_date = EXCLUDED.last_run_date,
    updated_at = now()`

const shoeUpdate = `
UPDATE shoe_stats st SET
    total_distance_m = st.total_distance_m + r.distance_m,
    total_steps      = st.total_steps + COALESCE(r.step_count,0),
    total_moving_s   = st.total_moving_s + r.moving_s,
    run_count        = st.run_count + 1,
    elevation_gain_m = st.elevation_gain_m + r.elevation_gain_m,
    last_run_at      = r.started_at
FROM runs r
WHERE r.id = $1 AND r.shoe_id IS NOT NULL AND st.shoe_id = r.shoe_id`

func prUpsert(valueExpr string) string {
	return `
INSERT INTO personal_records (id, user_id, key, value, run_id)
SELECT $1, r.user_id, $3, r.` + valueExpr + `, r.id FROM runs r WHERE r.id = $2
  AND r.` + valueExpr + ` > 0
ON CONFLICT (user_id, key) DO UPDATE
    SET value = EXCLUDED.value, run_id = EXCLUDED.run_id, achieved_at = now()
    WHERE personal_records.value < EXCLUDED.value`
}
