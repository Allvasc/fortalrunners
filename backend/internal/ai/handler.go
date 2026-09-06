package ai

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
	g.POST("/ai/coach", h.AskCoach)
}

type AskReq struct {
	Prompt string `json:"prompt"`
}

func (h *Handler) AskCoach(c echo.Context) error {
	userID := auth.UserID(c)
	var req AskReq
	if err := c.Bind(&req); err != nil || req.Prompt == "" {
		req.Prompt = "Qual a melhor dica para meu treino em Fortaleza hoje?"
	}

	res, err := h.svc.AskCoach(c.Request().Context(), userID, req.Prompt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"coach": res})
}
