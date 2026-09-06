package poi

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/amenities", h.list)
}

func (h *Handler) list(c echo.Context) error {
	cityID := c.QueryParam("city_id")
	category := c.QueryParam("category")

	pois, err := h.store.ListPOIs(c.Request().Context(), cityID, category)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar pontos de apoio")
	}

	return c.JSON(http.StatusOK, map[string]any{"amenities": pois})
}
