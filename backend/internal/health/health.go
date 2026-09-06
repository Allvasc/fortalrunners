// Package health expõe liveness e readiness.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	pool  *pgxpool.Pool
	start time.Time
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool, start: time.Now()}
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/healthz", h.live) // liveness — o processo está de pé
	e.GET("/readyz", h.ready) // readiness — dá para receber tráfego
}

func (h *Handler) live(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status": "ok",
		"uptime": time.Since(h.start).Round(time.Second).String(),
	})
}

func (h *Handler) ready(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"status": "db_down"})
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "ready"})
}
