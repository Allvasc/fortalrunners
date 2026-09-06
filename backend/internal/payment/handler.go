package payment

import (
	"errors"
	"io"
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

// RegisterSecured monta as rotas autenticadas sob /v1.
func (h *Handler) RegisterSecured(g *echo.Group) {
	g.POST("/payments/checkout", h.checkout)
	g.POST("/orders/:id/refund", h.refund)
	g.GET("/me/subscription", h.getSubscription)
	g.POST("/me/subscription", h.subscribe)
	g.DELETE("/me/subscription", h.cancelSubscription)
}

// RegisterPublic monta o receptor de webhook do Asaas (assinatura própria, sem Bearer).
func (h *Handler) RegisterPublic(g *echo.Group) {
	g.POST("/webhooks/asaas", h.webhook)
}

type checkoutReq struct {
	PriceID    string `json:"price_id"`
	Method     string `json:"method"`
	CouponCode string `json:"coupon_code"`
}

func (h *Handler) checkout(c echo.Context) error {
	var req checkoutReq
	if err := c.Bind(&req); err != nil || req.PriceID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "price_id obrigatório")
	}

	ord, err := h.svc.Checkout(
		c.Request().Context(), auth.UserID(c), req.PriceID, req.Method, req.CouponCode,
		c.Request().Header.Get("Idempotency-Key"),
	)
	switch {
	case errors.Is(err, ErrPriceNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, ErrBadCoupon):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrLotClosed), errors.Is(err, ErrLotSoldOut), errors.Is(err, ErrAlreadyOrdered):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao criar cobrança")
	}
	return c.JSON(http.StatusOK, map[string]any{"order": ord})
}

type refundReq struct {
	Reason string `json:"reason"`
}

func (h *Handler) refund(c echo.Context) error {
	var req refundReq
	_ = c.Bind(&req)
	err := h.svc.RequestRefund(c.Request().Context(), auth.UserID(c), c.Param("id"), req.Reason)
	switch {
	case errors.Is(err, ErrOrderNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "pedido não encontrado")
	case errors.Is(err, ErrNotRefundable):
		return echo.NewHTTPError(http.StatusConflict, "pedido não é reembolsável neste estado")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao solicitar reembolso")
	}
	return c.JSON(http.StatusAccepted, map[string]any{"status": "requested"})
}

func (h *Handler) getSubscription(c echo.Context) error {
	sub, err := h.svc.Subscription(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, sub)
}

type subReq struct {
	Plan string `json:"plan"`
}

func (h *Handler) subscribe(c echo.Context) error {
	var req subReq
	_ = c.Bind(&req)
	if err := h.svc.Subscribe(c.Request().Context(), auth.UserID(c), req.Plan); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao assinar")
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "active"})
}

func (h *Handler) cancelSubscription(c echo.Context) error {
	if err := h.svc.CancelSubscription(c.Request().Context(), auth.UserID(c)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao cancelar")
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "canceled"})
}

func (h *Handler) webhook(c echo.Context) error {
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	sig := c.Request().Header.Get("asaas-access-token")
	if sig == "" {
		sig = c.Request().Header.Get("X-Signature")
	}

	if err := h.svc.HandleWebhook(c.Request().Context(), body, sig); err != nil {
		if errors.Is(err, ErrBadSignature) {
			return echo.NewHTTPError(http.StatusUnauthorized, "assinatura inválida")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao processar")
	}
	return c.NoContent(http.StatusOK)
}
