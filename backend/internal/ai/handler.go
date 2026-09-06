package ai

import (
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

func (h *Handler) RegisterRoutes(g *echo.Group) {
	// nomes do plano §10
	g.POST("/coach/ask", h.ask)
	g.GET("/coach/summary", h.summary)
	// alias usado pelos clientes atuais
	g.POST("/ai/coach", h.ask)
}

type askReq struct {
	Prompt string `json:"prompt"`
}

func (h *Handler) ask(c echo.Context) error {
	var req askReq
	_ = c.Bind(&req)
	res, err := h.svc.AskCoach(c.Request().Context(), auth.UserID(c), req.Prompt)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "coach indisponível no momento")
	}
	return c.JSON(http.StatusOK, map[string]any{"coach": res})
}

func (h *Handler) summary(c echo.Context) error {
	s, err := h.svc.Summary(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao montar resumo")
	}
	return c.JSON(http.StatusOK, s)
}
