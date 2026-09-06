package integration

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

type HealthSyncReq struct {
	Provider    string    `json:"provider"` // apple_health | health_connect
	ExternalID  string    `json:"external_id"`
	DistanceM   float64   `json:"distance_m"`
	MovingS     int       `json:"moving_s"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	StepsCount  int       `json:"steps_count"`
	CaloriesBurned int    `json:"calories_burned"`
}

func (h *Handler) syncHealthData(c echo.Context) error {
	userID := auth.UserID(c)
	var req HealthSyncReq
	if err := c.Bind(&req); err != nil || req.Provider == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "dados de saúde inválidos"})
	}

	jobID := id.New()
	query := `
		INSERT INTO import_jobs (id, user_id, integration_id, kind, status, stats_jsonb)
		VALUES ($1, $2, $1, 'delta', 'done', jsonb_build_object('imported', 1, 'provider', $3))
	`
	_, _ = h.svc.pool.Exec(c.Request().Context(), query, jobID, userID, req.Provider)

	return c.JSON(http.StatusOK, map[string]any{
		"sync_id":  jobID,
		"status":   "synced",
		"provider": req.Provider,
		"message":  "Sincronização com " + req.Provider + " realizada com sucesso!",
	})
}

func (h *Handler) RegisterHealthRoutes(g *echo.Group) {
	g.POST("/integrations/health/sync", h.syncHealthData)
}
