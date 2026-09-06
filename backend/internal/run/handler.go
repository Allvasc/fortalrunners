package run

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register monta as rotas protegidas de corrida sob /v1.
func (h *Handler) Register(g *echo.Group) {
	g.POST("/runs", h.upload)
	g.GET("/runs", h.list)
	g.GET("/runs/:id", h.get)
	g.GET("/runs/:id/metrics", h.metrics)
}

func (h *Handler) upload(c echo.Context) error {
	var in IngestInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	if len(in.Points) > 200_000 {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "muitos pontos")
	}
	v, err := h.svc.Ingest(c.Request().Context(), auth.UserID(c), in)
	if err != nil {
		return runErr(err)
	}
	return c.JSON(http.StatusAccepted, v)
}

func (h *Handler) get(c echo.Context) error {
	v, err := h.svc.Get(c.Request().Context(), c.Param("id"), auth.UserID(c))
	if err != nil {
		return runErr(err)
	}
	return c.JSON(http.StatusOK, v)
}

func (h *Handler) metrics(c echo.Context) error {
	mv, err := h.svc.Metrics(c.Request().Context(), c.Param("id"), auth.UserID(c))
	if err != nil {
		return runErr(err)
	}
	return c.JSON(http.StatusOK, mv)
}

func (h *Handler) list(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	vs, err := h.svc.List(c.Request().Context(), auth.UserID(c), limit)
	if err != nil {
		return runErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"runs": vs})
}

func runErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "corrida não encontrada")
	case errors.Is(err, ErrTooFewPoints), errors.Is(err, ErrBadWindow):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrShoeNotYours):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
}
