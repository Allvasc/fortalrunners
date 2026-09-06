package territory

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

const (
	minPolyM2  = 500.0     // ignora laços minúsculos
	maxPolyM2  = 500_000.0 // teto por polígono (0,5 km²)
	minTotalM2 = 500.0     // área total mínima para virar território
)

// riskConfig lê o gate de zona de risco do game_config (ajustável no admin).
// Sem bloqueio → severidade efetiva 999 (nada é subtraído).
func (s *store) riskConfig(ctx context.Context) (minSeverity int) {
	var blocking bool
	var sev int
	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT value_jsonb::text::bool FROM game_config WHERE key = 'risk_zone_blocking'), true),
			COALESCE((SELECT value_jsonb::text::int  FROM game_config WHERE key = 'risk_zone_min_severity'), 3)`).
		Scan(&blocking, &sev)
	if err != nil || !blocking {
		return 999
	}
	return sev
}

type runInfo struct {
	UserID string
	NPts   int
}

func (s *store) runInfo(ctx context.Context, runID string) (runInfo, error) {
	var ri runInfo
	err := s.pool.QueryRow(ctx, `
		SELECT r.user_id, ST_NPoints(t.geom)
		FROM runs r JOIN run_tracks t ON t.run_id = r.id
		WHERE r.id = $1`, runID).Scan(&ri.UserID, &ri.NPts)
	if errors.Is(err, pgx.ErrNoRows) {
		return ri, errNotFound
	}
	return ri, err
}

var errNotFound = errors.New("run/track não encontrado")

// polygonize roda o núcleo geométrico em PostGIS: fecha o anel, node + polygonize,
// filtra por área, une, e subtrai as zonas de risco ativas.
// Devolve a geometria (WKB), a área em m² e o nº de partes (proxy de "quarteirões").
func (s *store) polygonize(ctx context.Context, runID string, minSeverity int) (wkb []byte, areaM2 float64, parts int, ok bool, err error) {
	const q = `
WITH src AS (SELECT geom AS line FROM run_tracks WHERE run_id = $1),
closed AS (
    SELECT CASE
        WHEN ST_DWithin(ST_StartPoint(line)::geography, ST_EndPoint(line)::geography, 30)
            THEN ST_AddPoint(line, ST_StartPoint(line))
        ELSE line
    END AS line
    FROM src
),
noded AS (SELECT ST_UnaryUnion(ST_Node(ST_Force2D(line))) AS g FROM closed),
polys AS (SELECT (ST_Dump(ST_Polygonize(g))).geom AS poly FROM noded),
kept AS (
    SELECT ST_MakeValid(poly) AS poly
    FROM polys
    WHERE ST_IsValid(poly) AND ST_Area(poly::geography) BETWEEN $2 AND $3
),
claimed AS (SELECT ST_UnaryUnion(ST_Collect(poly)) AS g FROM kept),
risk AS (
    SELECT ST_Union(rz.geom) AS g
    FROM risk_zones rz, claimed c
    WHERE c.g IS NOT NULL
      AND rz.status = 'active' AND rz.severity >= $4
      AND (rz.active_from IS NULL OR rz.active_from <= now())
      AND (rz.active_to   IS NULL OR rz.active_to   >= now())
      AND ST_Intersects(rz.geom, c.g)
),
fin AS (
    SELECT CASE
        WHEN (SELECT g FROM claimed) IS NULL THEN NULL
        WHEN (SELECT g FROM risk)    IS NULL THEN (SELECT g FROM claimed)
        ELSE ST_Difference((SELECT g FROM claimed), (SELECT g FROM risk))
    END AS g
)
SELECT ST_AsBinary(ST_Multi(g)), ST_Area(g::geography), ST_NumGeometries(ST_Multi(g))
FROM fin
WHERE g IS NOT NULL AND NOT ST_IsEmpty(g)`

	err = s.pool.QueryRow(ctx, q, runID, minPolyM2, maxPolyM2, minSeverity).Scan(&wkb, &areaM2, &parts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, 0, false, nil // corrida sem laço fechado — território vazio
	}
	if err != nil {
		return nil, 0, 0, false, err
	}
	return wkb, areaM2, parts, areaM2 >= minTotalM2, nil
}

// insertTerritory grava o território e resolve o bairro por ponto interno.
// O território é permanente e pessoal — nenhum outro corredor o apaga ou o disputa.
func (s *store) insertTerritory(ctx context.Context, id, userID, runID string, wkb []byte, areaM2 float64) error {
	const q = `
		INSERT INTO territories (id, user_id, run_id, neighborhood_id, geom, area_m2, status)
		VALUES (
			$1, $2, $3,
			(SELECT n.id FROM neighborhoods n
			 WHERE ST_Contains(n.geom, ST_PointOnSurface(ST_GeomFromWKB($4, 4326)))
			 LIMIT 1),
			ST_GeomFromWKB($4, 4326), $5, 'active'
		)`
	_, err := s.pool.Exec(ctx, q, id, userID, runID, wkb, areaM2)
	return err
}

// finishRun marca a corrida como processada.
func (s *store) finishRun(ctx context.Context, runID, status string, areaM2 float64, parts int, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE runs SET status = $2::run_status, territory_area_m2 = $3, new_blocks = $4,
		                error = nullif($5,''), updated_at = now()
		WHERE id = $1`, runID, status, areaM2, parts, errMsg)
	return err
}
