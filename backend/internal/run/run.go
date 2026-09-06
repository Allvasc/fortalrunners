// Package run recebe corridas do cliente, valida o essencial e enfileira o
// processamento pesado (limpeza, map-matching, território, H3, anti-fraude)
// para o territory-worker.
//
// Fase 0: apenas o esqueleto do endpoint de upload. A ingestão real e a
// tabela `runs` entram na Fase 1.
package run

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
)

type Handler struct {
	pub queue.Publisher
}

func NewHandler(pub queue.Publisher) *Handler { return &Handler{pub: pub} }

// Register monta as rotas protegidas de corrida sob /v1.
func (h *Handler) Register(g *echo.Group) {
	g.POST("/runs", h.upload)
}

// upload — placeholder: aceita o corpo, responde 202 e publica um evento.
func (h *Handler) upload(c echo.Context) error {
	userID := auth.UserID(c)

	// TODO(fase-1): validar esquema (pontos, sensores), gravar bruto no object
	// storage, criar linha em `runs` com status=processing, publicar run.uploaded.
	_ = h.pub.Publish(c.Request().Context(), queue.SubjectRunUploaded, []byte(`{"user_id":"`+userID+`"}`))

	return c.JSON(http.StatusAccepted, map[string]any{
		"status":  "processing",
		"message": "ingestão de corrida ainda não implementada (Fase 1)",
	})
}
