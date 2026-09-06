// Package ranking serve os leaderboards e as estatísticas pessoais de leitura.
// Fase 1: agregação direta em SQL. Redis sorted sets entram quando o volume
// justificar (é otimização, não muda o contrato).
package ranking

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/leaderboards/global", h.global)
	g.GET("/leaderboards/neighborhood/:id", h.neighborhood)
	g.GET("/leaderboards/friends", h.friends)
	g.GET("/leaderboards/club/:id", h.club)
	g.GET("/me/lifetime", h.lifetime)
	g.GET("/me/records", h.records)
	g.GET("/me/evolution", h.evolution)
	g.GET("/coverage", h.coverage)
}

// GET /v1/coverage — % de células cobertas pelo corredor, por bairro + total da cidade.
func (h *Handler) coverage(c echo.Context) error {
	uid := auth.UserID(c)
	const q = `
		WITH mine AS (
			SELECT neighborhood_id, count(*) AS covered
			FROM h3_cells WHERE owner_id = $1 GROUP BY neighborhood_id
		)
		SELECT n.id, n.name, n.h3_total, COALESCE(m.covered, 0)
		FROM neighborhoods n LEFT JOIN mine m ON m.neighborhood_id = n.id
		WHERE n.h3_total > 0
		ORDER BY n.name`
	rows, err := h.pool.Query(c.Request().Context(), q, uid)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	defer rows.Close()

	type nb struct {
		NeighborhoodID string  `json:"neighborhood_id"`
		Name           string  `json:"name"`
		Total          int     `json:"total_cells"`
		Covered        int     `json:"covered_cells"`
		Pct            float64 `json:"pct"`
	}
	list := []nb{}
	var totCells, covCells int
	for rows.Next() {
		var b nb
		if err := rows.Scan(&b.NeighborhoodID, &b.Name, &b.Total, &b.Covered); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
		}
		if b.Total > 0 {
			b.Pct = round1(float64(b.Covered) / float64(b.Total) * 100)
		}
		totCells += b.Total
		covCells += b.Covered
		list = append(list, b)
	}
	cityPct := 0.0
	if totCells > 0 {
		cityPct = round1(float64(covCells) / float64(totCells) * 100)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"city":          map[string]any{"total_cells": totCells, "covered_cells": covCells, "pct": cityPct},
		"neighborhoods": list,
	})
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }

type entry struct {
	Rank      int     `json:"rank"`
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	AthleteID string  `json:"athlete_id"`
	AreaM2    float64 `json:"area_m2"`
	Blocks    int     `json:"blocks"`
}

// shadow_banned some do ranking dos outros (plano §8), mas continua se vendo.
const notShadow = `(u.status <> 'shadow_banned' OR u.id = $1)`

const boardGlobal = `
WITH agg AS (
    SELECT t.user_id, SUM(t.area_m2) AS area_m2, COUNT(*) AS blocks
    FROM territories t
    WHERE t.status = 'active'
    GROUP BY t.user_id
),
ranked AS (
    SELECT a.*, RANK() OVER (ORDER BY a.area_m2 DESC) AS rk
    FROM agg a JOIN users u ON u.id = a.user_id
    WHERE ` + notShadow + `
)
SELECT r.rk, r.user_id, u.username, u.athlete_id, r.area_m2, r.blocks
FROM ranked r JOIN users u ON u.id = r.user_id
WHERE r.rk <= 50 OR r.user_id = $1
ORDER BY r.rk`

const boardNeighborhood = `
WITH agg AS (
    SELECT t.user_id, SUM(t.area_m2) AS area_m2, COUNT(*) AS blocks
    FROM territories t
    WHERE t.status = 'active' AND t.neighborhood_id = $2
    GROUP BY t.user_id
),
ranked AS (
    SELECT a.*, RANK() OVER (ORDER BY a.area_m2 DESC) AS rk
    FROM agg a JOIN users u ON u.id = a.user_id
    WHERE ` + notShadow + `
)
SELECT r.rk, r.user_id, u.username, u.athlete_id, r.area_m2, r.blocks
FROM ranked r JOIN users u ON u.id = r.user_id
WHERE r.rk <= 50 OR r.user_id = $1
ORDER BY r.rk`

// friends: amigos aceitos do solicitante + ele mesmo.
const boardFriends = `
WITH circle AS (
    SELECT $1::text AS user_id
    UNION
    SELECT CASE WHEN f.user_id = $1 THEN f.friend_id ELSE f.user_id END
    FROM friendships f
    WHERE (f.user_id = $1 OR f.friend_id = $1) AND f.status = 'accepted'
),
agg AS (
    SELECT t.user_id, SUM(t.area_m2) AS area_m2, COUNT(*) AS blocks
    FROM territories t JOIN circle c ON c.user_id = t.user_id
    WHERE t.status = 'active'
    GROUP BY t.user_id
),
ranked AS (SELECT a.*, RANK() OVER (ORDER BY a.area_m2 DESC) AS rk FROM agg a)
SELECT r.rk, r.user_id, u.username, u.athlete_id, r.area_m2, r.blocks
FROM ranked r JOIN users u ON u.id = r.user_id
ORDER BY r.rk`

// club: membros do clube.
const boardClub = `
WITH agg AS (
    SELECT t.user_id, SUM(t.area_m2) AS area_m2, COUNT(*) AS blocks
    FROM territories t JOIN club_members cm ON cm.user_id = t.user_id AND cm.club_id = $2
    WHERE t.status = 'active'
    GROUP BY t.user_id
),
ranked AS (SELECT a.*, RANK() OVER (ORDER BY a.area_m2 DESC) AS rk FROM agg a)
SELECT r.rk, r.user_id, u.username, u.athlete_id, r.area_m2, r.blocks
FROM ranked r JOIN users u ON u.id = r.user_id
ORDER BY r.rk`

func (h *Handler) global(c echo.Context) error {
	return h.board(c, boardGlobal, auth.UserID(c))
}

func (h *Handler) neighborhood(c echo.Context) error {
	return h.board(c, boardNeighborhood, auth.UserID(c), c.Param("id"))
}

func (h *Handler) friends(c echo.Context) error {
	return h.board(c, boardFriends, auth.UserID(c))
}

func (h *Handler) club(c echo.Context) error {
	return h.board(c, boardClub, auth.UserID(c), c.Param("id"))
}

func (h *Handler) board(c echo.Context, q string, args ...any) error {
	uid := auth.UserID(c)
	rows, err := h.pool.Query(c.Request().Context(), q, args...)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	defer rows.Close()

	list := []entry{}
	var mine *entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.Rank, &e.UserID, &e.Username, &e.AthleteID, &e.AreaM2, &e.Blocks); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
		}
		if e.UserID == uid {
			cp := e
			mine = &cp
		}
		if e.Rank <= 50 {
			list = append(list, e)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"entries": list, "me": mine})
}

func (h *Handler) lifetime(c echo.Context) error {
	const q = `
		SELECT total_moving_s, total_distance_m, total_steps, run_count,
		       elevation_gain_m, territory_area_m2, active_days,
		       current_streak_days, longest_streak_days, last_run_date
		FROM lifetime_stats WHERE user_id = $1`
	var (
		m struct {
			MovingS     int64   `json:"total_moving_s"`
			DistanceM   int64   `json:"total_distance_m"`
			Steps       int64   `json:"total_steps"`
			RunCount    int     `json:"run_count"`
			ElevationM  int64   `json:"elevation_gain_m"`
			AreaM2      float64 `json:"territory_area_m2"`
			ActiveDays  int     `json:"active_days"`
			Streak      int     `json:"current_streak_days"`
			LongestStrk int     `json:"longest_streak_days"`
			LastRunDate *string `json:"last_run_date"`
		}
		lastRun *time.Time
	)
	err := h.pool.QueryRow(c.Request().Context(), q, auth.UserID(c)).Scan(
		&m.MovingS, &m.DistanceM, &m.Steps, &m.RunCount, &m.ElevationM, &m.AreaM2,
		&m.ActiveDays, &m.Streak, &m.LongestStrk, &lastRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusOK, map[string]any{"run_count": 0})
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	if lastRun != nil {
		s := lastRun.Format("2006-01-02")
		m.LastRunDate = &s
	}
	return c.JSON(http.StatusOK, m)
}

// evolution devolve as últimas ~26 semanas: distância, área nova, quarteirões e
// nº de corridas por semana ISO. Alimenta os gráficos de evolução do portal.
func (h *Handler) evolution(c echo.Context) error {
	rows, err := h.pool.Query(c.Request().Context(), `
		SELECT date_trunc('week', started_at AT TIME ZONE 'America/Fortaleza')::date AS wk,
		       COALESCE(SUM(distance_m), 0)::bigint,
		       COALESCE(SUM(territory_area_m2), 0)::double precision,
		       COALESCE(SUM(new_blocks), 0)::int,
		       COUNT(*)::int
		FROM runs
		WHERE user_id = $1 AND status = 'valid'
		  AND started_at > now() - interval '27 weeks'
		GROUP BY wk ORDER BY wk`, auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	defer rows.Close()
	type week struct {
		Week       string  `json:"week"`
		DistanceM  int64   `json:"distance_m"`
		AreaM2     float64 `json:"area_m2"`
		NewBlocks  int     `json:"new_blocks"`
		Runs       int     `json:"runs"`
	}
	out := []week{}
	for rows.Next() {
		var w week
		var d time.Time
		if err := rows.Scan(&d, &w.DistanceM, &w.AreaM2, &w.NewBlocks, &w.Runs); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
		}
		w.Week = d.Format("2006-01-02")
		out = append(out, w)
	}
	return c.JSON(http.StatusOK, map[string]any{"weeks": out})
}

func (h *Handler) records(c echo.Context) error {
	rows, err := h.pool.Query(c.Request().Context(),
		`SELECT distance_key, value_s, run_id, achieved_at FROM personal_records WHERE user_id = $1 ORDER BY distance_key`,
		auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var key string
		var value int64
		var runID *string
		var at any
		if err := rows.Scan(&key, &value, &runID, &at); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
		}
		out = append(out, map[string]any{"distance_key": key, "value_s": value, "run_id": runID, "achieved_at": at})
	}
	return c.JSON(http.StatusOK, map[string]any{"records": out})
}
