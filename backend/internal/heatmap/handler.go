package heatmap

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/heatmap", h.get)
}

// GET /v1/heatmap?bbox=minLon,minLat,maxLon,maxLat&scope=me&period=all&activity=run
func (h *Handler) get(c echo.Context) error {
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

	fc, err := h.svc.Query(c.Request().Context(), auth.UserID(c),
		c.QueryParam("scope"), c.QueryParam("period"), c.QueryParam("activity"), bbox)
	if errors.Is(err, ErrScopeUnsupported) {
		return echo.NewHTTPError(http.StatusNotImplemented, err.Error())
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSONBlob(http.StatusOK, []byte(fc))
}
