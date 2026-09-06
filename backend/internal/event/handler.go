package event

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/events", h.list)
	g.GET("/events/:id", h.get)
	g.GET("/events/:id/leaderboard", h.leaderboard)
	g.POST("/events/:id/join", h.register)     // plano §10
	g.POST("/events/:id/register", h.register) // alias
	g.POST("/events/:id/checkpoints/:station/checkin", h.checkpoint)
}

func (h *Handler) list(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	events, err := h.svc.ListEvents(c.Request().Context(), auth.UserID(c), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) get(c echo.Context) error {
	ev, err := h.svc.GetEvent(c.Request().Context(), c.Param("id"), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "evento não encontrado")
	}
	return c.JSON(http.StatusOK, map[string]any{"event": ev})
}

func (h *Handler) leaderboard(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	board, err := h.svc.Leaderboard(c.Request().Context(), c.Param("id"), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao montar ranking")
	}
	return c.JSON(http.StatusOK, map[string]any{"leaderboard": board})
}

type registerReq struct {
	Category  string `json:"category"`
	ShirtSize string `json:"shirt_size"`
}

func (h *Handler) register(c echo.Context) error {
	var req registerReq
	if err := c.Bind(&req); err != nil || req.Category == "" {
		req.Category = "5k"
		req.ShirtSize = "M"
	}
	p, err := h.svc.RegisterParticipant(c.Request().Context(), auth.UserID(c), c.Param("id"), req.Category, req.ShirtSize)
	if errors.Is(err, ErrPaidEvent) {
		return echo.NewHTTPError(http.StatusConflict, "evento pago — finalize pelo checkout")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao inscrever")
	}
	return c.JSON(http.StatusOK, map[string]any{"participant": p, "message": "Inscrição confirmada!"})
}

type checkpointReq struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (h *Handler) checkpoint(c echo.Context) error {
	var req checkpointReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "posição obrigatória")
	}
	kind, err := h.svc.CheckpointCheckin(c.Request().Context(), auth.UserID(c),
		c.Param("id"), c.Param("station"), req.Lat, req.Lng)
	switch {
	case errors.Is(err, ErrStationNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "estação não encontrada")
	case errors.Is(err, ErrTooFar):
		return echo.NewHTTPError(http.StatusBadRequest, "você está longe demais da estação")
	case errors.Is(err, ErrNotRegistered):
		return echo.NewHTTPError(http.StatusForbidden, "você não está inscrito neste evento")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao registrar checkpoint")
	}
	return c.JSON(http.StatusOK, map[string]any{"checkpoint": kind, "status": "recorded"})
}
