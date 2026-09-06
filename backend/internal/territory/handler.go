package territory

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

// Handler serve a camada de território como GeoJSON para o mapa.
type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/territories", h.list)
}

// GET /v1/territories?bbox=minLon,minLat,maxLon,maxLat&scope=me
// Fase 1: scope=me apenas.
func (h *Handler) list(c echo.Context) error {
	userID := auth.UserID(c)

	var bbox [4]float64
	if raw := c.QueryParam("bbox"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			return echo.NewHTTPError(http.StatusBadRequest, "bbox = minLon,minLat,maxLon,maxLat")
		}
		for i, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "bbox inválido")
			}
			bbox[i] = v
		}
	}

	fc, err := h.featureCollection(c.Request().Context(), userID, bbox)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSONBlob(http.StatusOK, []byte(fc))
}

func (h *Handler) featureCollection(ctx context.Context, userID string, bbox [4]float64) (string, error) {
	const q = `
		SELECT COALESCE(
			jsonb_build_object(
				'type', 'FeatureCollection',
				'features', COALESCE(jsonb_agg(
					jsonb_build_object(
						'type', 'Feature',
						'geometry', ST_AsGeoJSON(t.geom)::jsonb,
						'properties', jsonb_build_object(
							'id', t.id,
							'user_id', t.user_id,
							'area_m2', t.area_m2,
							'neighborhood_id', t.neighborhood_id,
							'claimed_at', t.claimed_at,
							'status', t.status
						)
					)
				), '[]'::jsonb)
			),
			jsonb_build_object('type','FeatureCollection','features','[]'::jsonb)
		)::text
		FROM territories t
		WHERE t.user_id = $1 AND t.status = 'active'
		  AND ($2 = 0 AND $3 = 0 AND $4 = 0 AND $5 = 0
		       OR ST_Intersects(t.geom, ST_MakeEnvelope($2,$3,$4,$5,4326)))`
	var out string
	err := h.pool.QueryRow(ctx, q, userID, bbox[0], bbox[1], bbox[2], bbox[3]).Scan(&out)
	return out, err
}
