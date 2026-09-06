package payment

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
	g.POST("/payments/checkout", h.Checkout)
	g.POST("/payments/webhook", h.Webhook)
}

type CheckoutReq struct {
	Kind        string `json:"kind"`
	AmountCents int    `json:"amount_cents"`
	Method      string `json:"method"`
}

func (h *Handler) Checkout(c echo.Context) error {
	userID := auth.UserID(c)
	var req CheckoutReq
	if err := c.Bind(&req); err != nil || req.AmountCents <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "parâmetros de checkout inválidos"})
	}
	if req.Kind == "" {
		req.Kind = "event_registration"
	}
	if req.Method == "" {
		req.Method = "pix"
	}

	ord, err := h.svc.Checkout(c.Request().Context(), userID, req.Kind, req.AmountCents, req.Method)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"order": ord})
}

type WebhookReq struct {
	Event   string `json:"event"`
	Payment struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"payment"`
}

func (h *Handler) Webhook(c echo.Context) error {
	var req WebhookReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
	}
	_ = h.svc.ProcessWebhook(c.Request().Context(), req.Payment.ID, req.Payment.Status)
	return c.JSON(http.StatusOK, map[string]string{"status": "received"})
}
