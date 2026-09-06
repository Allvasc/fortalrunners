// Package riskzone serve as zonas de risco ATIVAS para o mapa (plano §6, §10:
// GET /v1/risk-zones?bbox=). O CRUD fica em /v1/admin/risk-zones.
package riskzone

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/risk-zones", h.list)
}

func (h *Handler) list(c echo.Context) error {
	var b [4]float64
	if raw := c.QueryParam("bbox"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			return echo.NewHTTPError(http.StatusBadRequest, "bbox = minLon,minLat,maxLon,maxLat")
		}
		for i, p := range parts {
			b[i], _ = strconv.ParseFloat(strings.TrimSpace(p), 64)
		}
	}
	const q = `
		SELECT COALESCE(jsonb_build_object(
			'type','FeatureCollection',
			'features', COALESCE(jsonb_agg(jsonb_build_object(
				'type','Feature',
				'geometry', ST_AsGeoJSON(geom)::jsonb,
				'properties', jsonb_build_object('id', id, 'severity', severity, 'note', note)
			)), '[]'::jsonb)
		), jsonb_build_object('type','FeatureCollection','features','[]'::jsonb))::text
		FROM risk_zones
		WHERE status = 'active' AND (active_to IS NULL OR active_to > now())
		  AND ($1 = 0 AND $2 = 0 AND $3 = 0 AND $4 = 0
		       OR ST_Intersects(geom, ST_MakeEnvelope($1,$2,$3,$4,4326)))`
	var out string
	if err := h.pool.QueryRow(c.Request().Context(), q, b[0], b[1], b[2], b[3]).Scan(&out); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSONBlob(http.StatusOK, []byte(out))
}
