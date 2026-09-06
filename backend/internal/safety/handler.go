package safety

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

func (h *Handler) Register(g *echo.Group) {
	g.GET("/safety/contacts", h.listContacts)
	g.POST("/safety/contacts", h.addContact)
	g.DELETE("/safety/contacts/:id", h.deleteContact)
	g.POST("/safety/sos", h.triggerSOS)
}

func (h *Handler) listContacts(c echo.Context) error {
	userID := auth.UserID(c)
	contacts, err := h.svc.ListContacts(c.Request().Context(), userID)
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
	userID := auth.UserID(c)
	var req addContactReq
	if err := c.Bind(&req); err != nil || req.Name == "" || req.Phone == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nome e telefone sao obrigatorios")
	}

	cnt, err := h.svc.AddContact(c.Request().Context(), userID, req.Name, req.Phone, req.Relation)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao adicionar contato")
	}

	return c.JSON(http.StatusCreated, cnt)
}

func (h *Handler) deleteContact(c echo.Context) error {
	userID := auth.UserID(c)
	cID := c.Param("id")

	err := h.svc.DeleteContact(c.Request().Context(), userID, cID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao remover contato")
	}

	return c.NoContent(http.StatusNoContent)
}

type sosReq struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Note string  `json:"note"`
}

func (h *Handler) triggerSOS(c echo.Context) error {
	userID := auth.UserID(c)
	var req sosReq
	_ = c.Bind(&req)

	sos, err := h.svc.TriggerSOS(c.Request().Context(), userID, req.Lat, req.Lng, req.Note)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao disparar alerta SOS")
	}

	return c.JSON(http.StatusCreated, sos)
}
