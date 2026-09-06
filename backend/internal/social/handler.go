package social

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
	g.GET("/friends", h.listFriends)
	g.POST("/friends", h.sendRequest)              // plano §10
	g.PATCH("/friends/:id", h.patchFriend)         // accept | block
	g.DELETE("/friends/:id", h.removeFriend)       // desfaz / recusa
	g.POST("/friends/request", h.sendRequest)      // alias
	g.POST("/friends/accept", h.acceptRequestBody) // alias
	g.GET("/feed", h.feed)
	g.POST("/runs/:id/kudos", h.toggleKudos)
}

func (h *Handler) listFriends(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	friends, err := h.svc.ListFriends(c.Request().Context(), auth.UserID(c), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar amigos")
	}
	if friends == nil {
		friends = []Friend{}
	}
	return c.JSON(http.StatusOK, map[string]any{"friends": friends})
}

type friendReq struct {
	TargetID string `json:"target_id"`
}

func mapFriendErr(err error) error {
	switch {
	case errors.Is(err, ErrCannotFriendSelf):
		return echo.NewHTTPError(http.StatusBadRequest, "não pode adicionar a si mesmo")
	case errors.Is(err, ErrTargetNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "usuário não encontrado")
	case errors.Is(err, ErrRequestNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "pedido não encontrado")
	case errors.Is(err, ErrNotRecipient):
		return echo.NewHTTPError(http.StatusForbidden, "só quem recebeu o pedido pode aceitá-lo")
	case err != nil:
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return nil
}

func (h *Handler) sendRequest(c echo.Context) error {
	var req friendReq
	if err := c.Bind(&req); err != nil || req.TargetID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_id obrigatório")
	}
	if err := h.svc.SendRequest(c.Request().Context(), auth.UserID(c), req.TargetID); err != nil {
		return mapFriendErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "pending"})
}

func (h *Handler) acceptRequestBody(c echo.Context) error {
	var req friendReq
	if err := c.Bind(&req); err != nil || req.TargetID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_id obrigatório")
	}
	if err := h.svc.AcceptRequest(c.Request().Context(), auth.UserID(c), req.TargetID); err != nil {
		return mapFriendErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "accepted"})
}

func (h *Handler) patchFriend(c echo.Context) error {
	target := c.Param("id")
	var body struct {
		Action string `json:"action"` // accept | block
	}
	_ = c.Bind(&body)
	ctx := c.Request().Context()
	var err error
	switch body.Action {
	case "block":
		err = h.svc.Block(ctx, auth.UserID(c), target)
	default:
		err = h.svc.AcceptRequest(ctx, auth.UserID(c), target)
	}
	if err != nil {
		return mapFriendErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) removeFriend(c echo.Context) error {
	if err := h.svc.Remove(c.Request().Context(), auth.UserID(c), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao remover")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) feed(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	events, next, err := h.svc.GetFeed(c.Request().Context(), auth.UserID(c), c.QueryParam("cursor"), limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao buscar feed")
	}
	if events == nil {
		events = []FeedEvent{}
	}
	return c.JSON(http.StatusOK, map[string]any{"feed": events, "next_cursor": next})
}

func (h *Handler) toggleKudos(c echo.Context) error {
	kudosed, err := h.svc.ToggleKudos(c.Request().Context(), auth.UserID(c), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao processar kudos")
	}
	return c.JSON(http.StatusOK, map[string]any{"kudosed": kudosed})
}
