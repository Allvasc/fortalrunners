package club

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

func (h *Handler) Register(g *echo.Group) {
	g.GET("/clubs", h.list)
	g.POST("/clubs", h.create)
	g.GET("/clubs/leaderboard", h.leaderboard)
	g.GET("/clubs/:id", h.get)
	g.POST("/clubs/:id/join", h.join)
	g.POST("/clubs/:id/leave", h.leave)
}

func (h *Handler) list(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	clubs, err := h.svc.ListClubs(c.Request().Context(), auth.UserID(c), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar clubes")
	}
	return c.JSON(http.StatusOK, map[string]any{"clubs": clubs})
}

func (h *Handler) leaderboard(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	board, err := h.svc.Leaderboard(c.Request().Context(), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao montar ranking")
	}
	return c.JSON(http.StatusOK, map[string]any{"leaderboard": board})
}

func (h *Handler) leave(c echo.Context) error {
	if err := h.svc.LeaveClub(c.Request().Context(), auth.UserID(c), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao sair do clube")
	}
	return c.JSON(http.StatusOK, map[string]any{"left": true})
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
