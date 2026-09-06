// Package media serve os arquivos guardados no blobstore (plano §14): domínio
// isolado, URL por chave opaca, acesso só do dono ou do staff (moderação).
package media

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/blobstore"
)

type Handler struct {
	store blobstore.Store
}

func NewHandler(store blobstore.Store) *Handler { return &Handler{store: store} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/media/*", h.serve)
}

func (h *Handler) serve(c echo.Context) error {
	key := c.Param("*")
	if key == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "chave ausente")
	}
	ctx := c.Request().Context()

	owner, err := h.store.OwnerOf(ctx, key)
	if errors.Is(err, blobstore.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "não encontrado")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}

	role, _ := c.Get("role").(string)
	staff := role == "admin" || role == "moderator"
	if !staff && owner != "" && owner != auth.UserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, "não encontrado") // não revela existência
	}

	blob, err := h.store.Get(ctx, key)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "não encontrado")
	}
	c.Response().Header().Set("Cache-Control", "private, max-age=86400")
	return c.Blob(http.StatusOK, blob.ContentType, blob.Bytes)
}
