package landmark

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
	g.GET("/landmarks", h.list)
	g.GET("/landmarks/:id", h.get)
	g.POST("/landmarks/:id/checkin", h.checkin)
	g.GET("/me/landmarks", h.userProgress)
}

func (h *Handler) list(c echo.Context) error {
	userID := auth.UserID(c)
	prog, err := h.svc.GetProgress(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar marcos")
	}
	return c.JSON(http.StatusOK, prog)
}

func (h *Handler) get(c echo.Context) error {
	userID := auth.UserID(c)
	lmID := c.Param("id")

	lm, err := h.svc.GetLandmark(c.Request().Context(), lmID, userID)
	if errors.Is(err, ErrLandmarkNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "marco não encontrado")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar marco")
	}

	return c.JSON(http.StatusOK, lm)
}

type checkinReq struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RunID    *string `json:"run_id,omitempty"`
	PhotoKey *string `json:"photo_key,omitempty"`
}

func (h *Handler) checkin(c echo.Context) error {
	userID := auth.UserID(c)
	lmID := c.Param("id")

	var req checkinReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "requisicao invalida")
	}

	chk, err := h.svc.Checkin(c.Request().Context(), userID, lmID, req.RunID, req.PhotoKey, req.Lat, req.Lng)
	if errors.Is(err, ErrLandmarkNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "marco não encontrado")
	}
	if errors.Is(err, ErrTooFar) {
		return echo.NewHTTPError(http.StatusBadRequest, "você precisa estar mais próximo do marco para fazer check-in")
	}
	if errors.Is(err, ErrAlreadyCheckedIn) {
		return echo.NewHTTPError(http.StatusConflict, "você já realizou o check-in neste marco")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao realizar check-in")
	}

	return c.JSON(http.StatusCreated, chk)
}

func (h *Handler) userProgress(c echo.Context) error {
	userID := auth.UserID(c)
	prog, err := h.svc.GetProgress(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar progresso")
	}
	return c.JSON(http.StatusOK, prog)
}
