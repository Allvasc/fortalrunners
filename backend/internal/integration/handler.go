package integration

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterSecured monta as rotas autenticadas sob /v1.
func (h *Handler) RegisterSecured(g *echo.Group) {
	g.GET("/integrations", h.list)
	g.GET("/integrations/:provider/connect", h.connect)
	g.GET("/integrations/:provider/callback", h.callback)
	g.POST("/integrations/:provider/sync", h.sync)
	g.POST("/integrations/health/sync", h.svc.IngestHealth)
	g.DELETE("/integrations/:provider", h.disconnect)
}

// RegisterPublic monta o receptor de webhook (verificação de assinatura própria).
func (h *Handler) RegisterPublic(g *echo.Group) {
	g.GET("/webhooks/strava", h.webhookVerify)
	g.POST("/webhooks/strava", h.webhookReceive)
}

func (h *Handler) list(c echo.Context) error {
	items, err := h.svc.List(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, map[string]any{"integrations": items})
}

func (h *Handler) connect(c echo.Context) error {
	url, err := h.svc.ConnectURL(c.Param("provider"), auth.UserID(c))
	if err != nil {
		return integErr(err)
	}
	// o navegador não manda Bearer num redirect: o front busca a URL e navega.
	if c.QueryParam("redirect") == "1" {
		return c.Redirect(http.StatusFound, url)
	}
	return c.JSON(http.StatusOK, map[string]string{"authorize_url": url})
}

func (h *Handler) callback(c echo.Context) error {
	code, state := c.QueryParam("code"), c.QueryParam("state")
	if code == "" || state == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code e state obrigatórios")
	}
	if _, err := h.svc.Complete(c.Request().Context(), c.Param("provider"), code, state); err != nil {
		return integErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"connected": true, "backfill": "enfileirado"})
}

func (h *Handler) sync(c echo.Context) error {
	if err := h.svc.Sync(c.Request().Context(), c.Param("provider"), auth.UserID(c)); err != nil {
		return integErr(err)
	}
	return c.JSON(http.StatusAccepted, map[string]any{"sync": "enfileirado"})
}

func (h *Handler) disconnect(c echo.Context) error {
	purge := c.QueryParam("purge") == "true"
	if err := h.svc.Disconnect(c.Request().Context(), c.Param("provider"), auth.UserID(c), purge); err != nil {
		return integErr(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- webhook ---

func (h *Handler) webhookVerify(c echo.Context) error {
	challenge, ok := h.svc.VerifyWebhook(
		c.QueryParam("hub.mode"), c.QueryParam("hub.verify_token"), c.QueryParam("hub.challenge"))
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "verify token inválido")
	}
	return c.JSON(http.StatusOK, map[string]string{"hub.challenge": challenge})
}

func (h *Handler) webhookReceive(c echo.Context) error {
	body, _ := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err := h.svc.HandleWebhook(c.Request().Context(), "strava", body); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.NoContent(http.StatusOK) // Strava exige 200 rápido
}

func integErr(err error) error {
	switch {
	case errors.Is(err, ErrProviderOff):
		return echo.NewHTTPError(http.StatusNotImplemented, err.Error())
	case errors.Is(err, ErrNotConnected):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, ErrState):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusBadGateway, "falha ao falar com o provedor")
	}
}
