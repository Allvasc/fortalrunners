package qr

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
	// nomes do plano §10
	g.GET("/me/qr", h.getToken)
	g.GET("/me/scans", h.myScans)
	g.POST("/scan", h.scan)
	g.POST("/scan/confirm", h.confirm)
	// aliases usados pelos clientes atuais
	g.GET("/qr/token", h.getToken)
	g.POST("/qr/scan", h.scan)
}

func (h *Handler) getToken(c echo.Context) error {
	tok, err := h.svc.GetToken(c.Request().Context(), auth.UserID(c), c.QueryParam("event_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao emitir QR")
	}
	return c.JSON(http.StatusOK, map[string]any{"qr": tok})
}

type scanReq struct {
	// aceita os dois nomes: payload assinado do QR
	Payload  string `json:"payload"`
	TokenSig string `json:"token_sig"`
	Kind     string `json:"kind"`
}

func (h *Handler) scan(c echo.Context) error {
	var req scanReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	payload := req.Payload
	if payload == "" {
		payload = req.TokenSig
	}
	res, err := h.svc.ScanToken(c.Request().Context(), auth.UserID(c), payload, req.Kind)
	switch {
	case errors.Is(err, ErrBadToken), errors.Is(err, ErrRevoked):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "QR inválido ou expirado")
	case errors.Is(err, ErrSelfScan):
		return echo.NewHTTPError(http.StatusBadRequest, "não é possível escanear o próprio QR")
	case errors.Is(err, ErrScanScope):
		return echo.NewHTTPError(http.StatusBadRequest, "tipo de leitura inválido")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao registrar leitura")
	}
	return c.JSON(http.StatusOK, map[string]any{"scan": res})
}

type confirmReq struct {
	ScanID string `json:"scan_id"`
}

func (h *Handler) confirm(c echo.Context) error {
	var req confirmReq
	if err := c.Bind(&req); err != nil || req.ScanID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "scan_id obrigatório")
	}
	if err := h.svc.ConfirmScan(c.Request().Context(), auth.UserID(c), req.ScanID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "leitura pendente não encontrada")
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "recorded"})
}

func (h *Handler) myScans(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	scans, err := h.svc.MyScans(c.Request().Context(), auth.UserID(c), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar leituras")
	}
	return c.JSON(http.StatusOK, map[string]any{"scans": scans})
}
