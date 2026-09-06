package event

import (
	"net/http"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/events", h.ListEvents)
	g.GET("/events/:id", h.GetEvent)
	g.POST("/events/:id/register", h.RegisterParticipant)
}

func (h *Handler) ListEvents(c echo.Context) error {
	userID := auth.UserID(c)
	events, err := h.svc.ListEvents(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) GetEvent(c echo.Context) error {
	userID := auth.UserID(c)
	eventID := c.Param("id")
	ev, err := h.svc.GetEvent(c.Request().Context(), eventID, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "evento não encontrado"})
	}
	return c.JSON(http.StatusOK, map[string]any{"event": ev})
}

type RegisterReq struct {
	Category  string `json:"category"`
	ShirtSize string `json:"shirt_size"`
}

func (h *Handler) RegisterParticipant(c echo.Context) error {
	userID := auth.UserID(c)
	eventID := c.Param("id")
	var req RegisterReq
	if err := c.Bind(&req); err != nil || req.Category == "" {
		req.Category = "5k"
		req.ShirtSize = "M"
	}
	p, err := h.svc.RegisterParticipant(c.Request().Context(), userID, eventID, req.Category, req.ShirtSize)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"participant": p, "message": "Inscrição confirmada!"})
}
