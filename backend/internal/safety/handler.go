package safety

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
	g.GET("/safety/contacts", h.listContacts)
	g.POST("/safety/contacts", h.addContact)
	g.DELETE("/safety/contacts/:id", h.deleteContact)
	g.POST("/safety/sos", h.triggerSOS)
	g.POST("/safety/sos/:id/beacon", h.updateBeacon)
	g.POST("/safety/sos/:id/cancel", h.cancelSOS)
}

// RegisterPublic monta a página pública do beacon (sem login — plano §6, GET /s/:token).
func (h *Handler) RegisterPublic(e *echo.Echo) {
	e.GET("/s/:token", h.beacon)
}

func (h *Handler) listContacts(c echo.Context) error {
	contacts, err := h.svc.ListContacts(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar contatos")
	}
	return c.JSON(http.StatusOK, map[string]any{"contacts": contacts})
}

type addContactReq struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Relation string `json:"relation"`
}

func (h *Handler) addContact(c echo.Context) error {
	var req addContactReq
	if err := c.Bind(&req); err != nil || req.Name == "" || req.Phone == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nome e telefone são obrigatórios")
	}
	if len(req.Phone) > 32 || len(req.Name) > 120 {
		return echo.NewHTTPError(http.StatusBadRequest, "campo muito longo")
	}
	cnt, err := h.svc.AddContact(c.Request().Context(), auth.UserID(c), req.Name, req.Phone, req.Relation)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao adicionar contato")
	}
	return c.JSON(http.StatusCreated, cnt)
}

func (h *Handler) deleteContact(c echo.Context) error {
	if err := h.svc.DeleteContact(c.Request().Context(), auth.UserID(c), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao remover contato")
	}
	return c.NoContent(http.StatusNoContent)
}

type sosReq struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Note  string  `json:"note"`
	PIN   string  `json:"pin"`
	RunID string  `json:"run_id"`
}

func (h *Handler) triggerSOS(c echo.Context) error {
	var req sosReq
	_ = c.Bind(&req)
	sos, err := h.svc.TriggerSOS(c.Request().Context(), auth.UserID(c), req.Lat, req.Lng, req.Note, req.PIN, req.RunID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao disparar alerta SOS")
	}
	return c.JSON(http.StatusCreated, sos)
}

type beaconUpdateReq struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (h *Handler) updateBeacon(c echo.Context) error {
	var req beaconUpdateReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	if err := h.svc.UpdateBeacon(c.Request().Context(), auth.UserID(c), c.Param("id"), req.Lat, req.Lng); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao atualizar beacon")
	}
	return c.NoContent(http.StatusNoContent)
}

type cancelReq struct {
	PIN string `json:"pin"`
}

func (h *Handler) cancelSOS(c echo.Context) error {
	var req cancelReq
	_ = c.Bind(&req)
	err := h.svc.CancelSOS(c.Request().Context(), auth.UserID(c), c.Param("id"), req.PIN)
	switch {
	case errors.Is(err, ErrSOSNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "SOS não encontrado")
	case errors.Is(err, ErrBadPIN):
		return echo.NewHTTPError(http.StatusForbidden, "PIN incorreto")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao cancelar")
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "resolved"})
}

func (h *Handler) beacon(c echo.Context) error {
	b, err := h.svc.Beacon(c.Request().Context(), c.Param("token"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "beacon não encontrado ou expirado")
	}
	return c.JSON(http.StatusOK, b)
}
