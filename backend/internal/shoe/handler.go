package shoe

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/shoes", h.list)
	g.POST("/shoes", h.create)
	g.GET("/shoes/:id", h.get)
	g.POST("/shoes/:id/retire", h.retire)
}

func (h *Handler) list(c echo.Context) error {
	sh, err := h.svc.List(c.Request().Context(), auth.UserID(c), c.QueryParam("retired") == "true")
	if err != nil {
		return shoeErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"shoes": sh})
}

func (h *Handler) create(c echo.Context) error {
	var in CreateInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	sh, err := h.svc.Create(c.Request().Context(), auth.UserID(c), in)
	if err != nil {
		return shoeErr(err)
	}
	return c.JSON(http.StatusCreated, sh)
}

func (h *Handler) get(c echo.Context) error {
	sh, err := h.svc.Get(c.Request().Context(), auth.UserID(c), c.Param("id"))
	if err != nil {
		return shoeErr(err)
	}
	return c.JSON(http.StatusOK, sh)
}

func (h *Handler) retire(c echo.Context) error {
	if err := h.svc.Retire(c.Request().Context(), auth.UserID(c), c.Param("id")); err != nil {
		return shoeErr(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func shoeErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "tênis não encontrado")
	case errors.Is(err, ErrInvalid):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
}
