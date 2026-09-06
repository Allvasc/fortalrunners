package challenge

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/challenges", h.list)
	g.GET("/challenges/:slug/leaderboard", h.leaderboard)
}

func (h *Handler) list(c echo.Context) error {
	vs, err := h.svc.ListForUser(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, map[string]any{"challenges": vs})
}

func (h *Handler) leaderboard(c echo.Context) error {
	res, err := h.svc.Leaderboard(c.Request().Context(), c.Param("slug"))
	if errors.Is(err, errNoChallenge) {
		return echo.NewHTTPError(http.StatusNotFound, "desafio não encontrado")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, res)
}
