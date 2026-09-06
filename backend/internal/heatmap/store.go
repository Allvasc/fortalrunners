package heatmap

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// cellDeg = ~50 m em Fortaleza (lat -3.73). Stand-in de grade até o H3 (Fase 2).
const cellDeg = 0.00045

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

// usersToRefresh: quem tem corrida válida mais nova que o último refresh do seu
// heatmap (ou nunca teve).
func (s *store) usersToRefresh(ctx context.Context, limit int) ([]string, error) {
	const q = `
		SELECT r.user_id
		FROM runs r
		WHERE r.status = 'valid'
		GROUP BY r.user_id
		HAVING max(r.updated_at) > COALESCE(
			(SELECT max(h.updated_at) FROM heat_agg h WHERE h.scope = 'user:' || r.user_id), 'epoch')
		LIMIT $1`
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// refreshUser recomputa o heatmap pessoal do corredor a partir dos pontos das
// suas corridas válidas. Recompute completo (poucos usuários na Fase 1);
// incremental de verdade quando o volume pedir.
func (s *store) refreshUser(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	scope := "user:" + userID
	if _, err := tx.Exec(ctx, `DELETE FROM heat_agg WHERE scope = $1`, scope); err != nil {
		return err
	}

	const q = `
		WITH pts AS (
			SELECT ST_SnapToGrid((dp).geom, 0, 0, $2::float8, $2::float8) AS cell
			FROM run_tracks rt
			JOIN runs r ON r.id = rt.run_id
			CROSS JOIN LATERAL ST_DumpPoints(rt.geom) dp
			WHERE r.user_id = $3 AND r.status = 'valid'
		),
		agg AS (SELECT cell, count(*)::int AS hits FROM pts GROUP BY cell),
		mx AS (SELECT GREATEST(max(hits), 1) AS m FROM agg)
		INSERT INTO heat_agg (h3_index, scope, period, activity, hits, weight, runner_count, cell_geom)
		SELECT
			floor((ST_Y(a.cell) + 90) / $2)::bigint * 10000000
				+ floor((ST_X(a.cell) + 180) / $2)::bigint,
			$1, 'all', 'run',
			a.hits,
			round(a.hits::numeric / mx.m, 4),
			1,
			ST_SetSRID(ST_Point(ST_X(a.cell) + $2/2, ST_Y(a.cell) + $2/2), 4326)
		FROM agg a, mx
		ON CONFLICT (h3_index, scope, period, activity) DO UPDATE
			SET hits = EXCLUDED.hits, weight = EXCLUDED.weight,
			    cell_geom = EXCLUDED.cell_geom, updated_at = now()`
	if _, err := tx.Exec(ctx, q, scope, cellDeg, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// featureCollection devolve os pontos de calor como GeoJSON (nunca traçados).
func (s *store) featureCollection(ctx context.Context, scope, period, activity string, bbox [4]float64, kAnon int) (string, error) {
	const q = `
		SELECT jsonb_build_object(
			'type', 'FeatureCollection',
			'features', COALESCE(jsonb_agg(f), '[]'::jsonb)
		)::text
		FROM (
			SELECT jsonb_build_object(
				'type', 'Feature',
				'geometry', ST_AsGeoJSON(cell_geom)::jsonb,
				'properties', jsonb_build_object('w', weight, 'hits', hits)
			) AS f
			FROM heat_agg
			WHERE scope = $1 AND period = $2 AND activity = $3
			  AND runner_count >= $4
			  AND ($5 = 0 AND $6 = 0 AND $7 = 0 AND $8 = 0
			       OR ST_Intersects(cell_geom, ST_MakeEnvelope($5, $6, $7, $8, 4326)))
			ORDER BY weight DESC
			LIMIT 20000
		) sub`
	var out string
	err := s.pool.QueryRow(ctx, q, scope, period, activity, kAnon,
		bbox[0], bbox[1], bbox[2], bbox[3]).Scan(&out)
	return out, err
}
