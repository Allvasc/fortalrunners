package qr

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
	g.GET("/qr/token", h.GetToken)
	g.POST("/qr/scan", h.ScanToken)
}

func (h *Handler) GetToken(c echo.Context) error {
	userID := auth.UserID(c)
	eventID := c.QueryParam("event_id")
	tok, err := h.svc.GetToken(c.Request().Context(), userID, eventID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"qr": tok})
}

type ScanReq struct {
	TokenSig string `json:"token_sig"`
	Kind     string `json:"kind"`
}

func (h *Handler) ScanToken(c echo.Context) error {
	userID := auth.UserID(c)
	var req ScanReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "dados inválidos"})
	}
	res, err := h.svc.ScanToken(c.Request().Context(), userID, req.TokenSig, req.Kind)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"scan": res, "message": "Leitura registrada com sucesso!"})
}
