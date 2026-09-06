package social

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/friends", h.listFriends)
	g.POST("/friends/request", h.sendRequest)
	g.POST("/friends/accept", h.acceptRequest)
	g.GET("/feed", h.feed)
	g.POST("/runs/:id/kudos", h.toggleKudos)
}

func (h *Handler) listFriends(c echo.Context) error {
	userID := auth.UserID(c)
	friends, err := h.svc.ListFriends(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar amigos")
	}
	return c.JSON(http.StatusOK, map[string]any{"friends": friends})
}

type friendReq struct {
	TargetID string `json:"target_id"`
}

func (h *Handler) sendRequest(c echo.Context) error {
	userID := auth.UserID(c)
	var req friendReq
	if err := c.Bind(&req); err != nil || req.TargetID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_id obrigatorio")
	}

	err := h.svc.SendRequest(c.Request().Context(), userID, req.TargetID)
	if errors.Is(err, ErrCannotFriendSelf) {
		return echo.NewHTTPError(http.StatusBadRequest, "nao pode adicionar a si mesmo")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao enviar solicitação")
	}

	return c.JSON(http.StatusOK, map[string]any{"status": "pending"})
}

func (h *Handler) acceptRequest(c echo.Context) error {
	userID := auth.UserID(c)
	var req friendReq
	if err := c.Bind(&req); err != nil || req.TargetID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_id obrigatorio")
	}

	err := h.svc.AcceptRequest(c.Request().Context(), userID, req.TargetID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao aceitar amizade")
	}

	return c.JSON(http.StatusOK, map[string]any{"status": "accepted"})
}

func (h *Handler) feed(c echo.Context) error {
	userID := auth.UserID(c)
	events, err := h.svc.GetFeed(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar feed")
	}
	return c.JSON(http.StatusOK, map[string]any{"feed": events})
}

func (h *Handler) toggleKudos(c echo.Context) error {
	userID := auth.UserID(c)
	runID := c.Param("id")

	kudosed, err := h.svc.ToggleKudos(c.Request().Context(), userID, runID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao processar kudos")
	}

	return c.JSON(http.StatusOK, map[string]any{"kudosed": kudosed})
}
