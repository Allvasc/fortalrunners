package club

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
	g.GET("/clubs", h.list)
	g.POST("/clubs", h.create)
	g.GET("/clubs/:id", h.get)
	g.POST("/clubs/:id/join", h.join)
}

func (h *Handler) list(c echo.Context) error {
	userID := auth.UserID(c)
	clubs, err := h.svc.ListClubs(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar clubes")
	}
	return c.JSON(http.StatusOK, map[string]any{"clubs": clubs})
}

type createClubReq struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	NeighborhoodID *string `json:"neighborhood_id,omitempty"`
	ColorHex       string  `json:"color_hex"`
}

func (h *Handler) create(c echo.Context) error {
	userID := auth.UserID(c)
	var req createClubReq
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nome do clube e obrigatorio")
	}

	cl, err := h.svc.CreateClub(c.Request().Context(), userID, req.Name, req.Description, req.NeighborhoodID, req.ColorHex)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao criar clube")
	}

	return c.JSON(http.StatusCreated, cl)
}

func (h *Handler) get(c echo.Context) error {
	userID := auth.UserID(c)
	clubID := c.Param("id")

	cl, members, err := h.svc.GetClub(c.Request().Context(), clubID, userID)
	if errors.Is(err, ErrClubNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "clube nao encontrado")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar clube")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"club":    cl,
		"members": members,
	})
}

func (h *Handler) join(c echo.Context) error {
	userID := auth.UserID(c)
	clubID := c.Param("id")

	err := h.svc.JoinClub(c.Request().Context(), userID, clubID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao entrar no clube")
	}

	return c.JSON(http.StatusOK, map[string]any{"joined": true})
}
