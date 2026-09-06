package territory

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/h3grid"
)

// EnsureCoverageGrid repovoa h3_cells e recalcula neighborhoods.h3_total quando a
// grade de cobertura muda (troca do stand-in lat/lng para H3 real, ou vice-versa).
// É idempotente: sai cedo quando o marcador em game_config já bate com a grade
// compilada. Roda no boot da API.
func EnsureCoverageGrid(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	want := h3grid.Coverage.Tag()

	var stored string
	_ = pool.QueryRow(ctx, `SELECT value_jsonb #>> '{}' FROM game_config WHERE key = 'coverage_grid_tag'`).Scan(&stored)
	if stored == want {
		return nil
	}
	log.Info("grade de cobertura mudou — refazendo h3_cells e h3_total", "de", stored, "para", want)

	s := newStore(pool)

	if _, err := pool.Exec(ctx, `DELETE FROM h3_cells`); err != nil {
		return err
	}

	// h3_total por bairro = nº de células da grade com centro dentro do bairro.
	rows, err := pool.Query(ctx, `
		SELECT id, ST_YMin(geom), ST_XMin(geom), ST_YMax(geom), ST_XMax(geom)
		FROM neighborhoods WHERE geom IS NOT NULL`)
	if err != nil {
		return err
	}
	type nbh struct {
		id                             string
		minLat, minLng, maxLat, maxLng float64
	}
	var nbhs []nbh
	for rows.Next() {
		var n nbh
		if err := rows.Scan(&n.id, &n.minLat, &n.minLng, &n.maxLat, &n.maxLng); err != nil {
			rows.Close()
			return err
		}
		nbhs = append(nbhs, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, n := range nbhs {
		cells := h3grid.Coverage.CellsInBBox(n.minLat, n.minLng, n.maxLat, n.maxLng)
		lats := make([]float64, len(cells))
		lngs := make([]float64, len(cells))
		for i, c := range cells {
			lats[i], lngs[i] = c.CenterLat, c.CenterLng
		}
		if _, err := pool.Exec(ctx, `
			UPDATE neighborhoods n SET h3_total = COALESCE((
				SELECT count(*)
				FROM unnest($2::float8[], $3::float8[]) AS c(lat, lng)
				WHERE ST_Contains(n.geom, ST_SetSRID(ST_Point(c.lng, c.lat), 4326))
			), 0)
			WHERE n.id = $1`, n.id, lats, lngs); err != nil {
			return err
		}
	}

	// Repovoa a cobertura pessoal a partir dos territórios ativos.
	trows, err := pool.Query(ctx, `SELECT id, user_id FROM territories WHERE status = 'active'`)
	if err != nil {
		return err
	}
	type terr struct{ id, userID string }
	var terrs []terr
	for trows.Next() {
		var t terr
		if err := trows.Scan(&t.id, &t.userID); err != nil {
			trows.Close()
			return err
		}
		terrs = append(terrs, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return err
	}

	for _, t := range terrs {
		if err := s.polyfillCells(ctx, t.id, t.userID); err != nil {
			log.Warn("polyfill de território falhou no backfill", "territory", t.id, "err", err)
		}
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO game_config (key, value_jsonb) VALUES ('coverage_grid_tag', to_jsonb($1::text))
		ON CONFLICT (key) DO UPDATE SET value_jsonb = EXCLUDED.value_jsonb, updated_at = now()`, want)
	if err != nil {
		return err
	}
	log.Info("grade de cobertura atualizada", "grade", want, "bairros", len(nbhs), "territorios", len(terrs))
	return nil
}
