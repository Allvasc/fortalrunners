package route

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
	g.GET("/routes", h.list)
	g.POST("/routes", h.create)
	g.GET("/routes/:id", h.get)
	g.POST("/routes/:id/reviews", h.addReview)
}

func (h *Handler) list(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	routes, err := h.svc.ListRoutes(c.Request().Context(), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar rotas")
	}
	return c.JSON(http.StatusOK, map[string]any{"routes": routes})
}

type createRouteReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Surface     string `json:"surface"`
	LineWKT     string `json:"line_wkt"`
}

func (h *Handler) create(c echo.Context) error {
	userID := auth.UserID(c)
	var req createRouteReq
	if err := c.Bind(&req); err != nil || req.Name == "" || req.LineWKT == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "nome e geometria line_wkt são obrigatórios")
	}
	if len(req.Name) > 120 || len(req.Description) > 2000 {
		return echo.NewHTTPError(http.StatusBadRequest, "texto muito longo")
	}
	if req.Surface == "" {
		req.Surface = "asfalto"
	}

	r, err := h.svc.CreateRoute(c.Request().Context(), userID, req.Name, req.Description, req.Surface, req.LineWKT)
	if errors.Is(err, ErrBadGeometry) {
		return echo.NewHTTPError(http.StatusBadRequest, "geometria da rota inválida (LINESTRING WGS84, 100 m a 200 km)")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao criar rota")
	}

	return c.JSON(http.StatusCreated, r)
}

func (h *Handler) get(c echo.Context) error {
	routeID := c.Param("id")
	r, reviews, err := h.svc.GetRoute(c.Request().Context(), routeID)
	if errors.Is(err, ErrRouteNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "rota nao encontrada")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar rota")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"route":   r,
		"reviews": reviews,
	})
}

type addReviewReq struct {
	Rating int      `json:"rating"`
	Tags   []string `json:"tags"`
	Body   string   `json:"body"`
}

func (h *Handler) addReview(c echo.Context) error {
	userID := auth.UserID(c)
	routeID := c.Param("id")

	var req addReviewReq
	if err := c.Bind(&req); err != nil || req.Rating < 1 || req.Rating > 5 {
		return echo.NewHTTPError(http.StatusBadRequest, "rating deve ser entre 1 e 5")
	}

	rev, err := h.svc.AddReview(c.Request().Context(), userID, routeID, req.Rating, req.Tags, req.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao salvar avaliacao")
	}

	return c.JSON(http.StatusCreated, rev)
}
