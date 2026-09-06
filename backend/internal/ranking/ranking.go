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
	g.GET("/me/lifetime", h.lifetime)
	g.GET("/me/records", h.records)
}

type entry struct {
	Rank      int     `json:"rank"`
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	AthleteID string  `json:"athlete_id"`
	AreaM2    float64 `json:"area_m2"`
	Blocks    int     `json:"blocks"`
}

const boardGlobal = `
WITH agg AS (
    SELECT t.user_id, SUM(t.area_m2) AS area_m2, COUNT(*) AS blocks
    FROM territories t
    WHERE t.status = 'active'
    GROUP BY t.user_id
),
ranked AS (SELECT a.*, RANK() OVER (ORDER BY a.area_m2 DESC) AS rk FROM agg a)
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
ranked AS (SELECT a.*, RANK() OVER (ORDER BY a.area_m2 DESC) AS rk FROM agg a)
SELECT r.rk, r.user_id, u.username, u.athlete_id, r.area_m2, r.blocks
FROM ranked r JOIN users u ON u.id = r.user_id
WHERE r.rk <= 50 OR r.user_id = $1
ORDER BY r.rk`

func (h *Handler) global(c echo.Context) error {
	return h.board(c, boardGlobal, auth.UserID(c))
}

func (h *Handler) neighborhood(c echo.Context) error {
	return h.board(c, boardNeighborhood, auth.UserID(c), c.Param("id"))
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
