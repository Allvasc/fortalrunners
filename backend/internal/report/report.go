// Package report é a denúncia genérica (plano §10: POST /v1/reports). Qualquer
// usuário reporta uma corrida, rota, avaliação, usuário, foto ou marcação; a fila
// é resolvida no painel de admin (/v1/admin/reports).
package report

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var validTargets = map[string]bool{
	"run": true, "route": true, "review": true, "user": true,
	"checkin": true, "hazard": true, "comment": true,
}

type Report struct {
	ID         string    `json:"id"`
	ReporterID string    `json:"reporter_id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Reason     string    `json:"reason"`
	Detail     string    `json:"detail,omitempty"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Create(ctx context.Context, reporterID, targetType, targetID, reason, detail string) (*Report, error) {
	r := &Report{
		ID: id.New(), ReporterID: reporterID, TargetType: targetType, TargetID: targetID,
		Reason: reason, Detail: detail, Status: "open", CreatedAt: time.Now().UTC(),
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail, status, created_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), 'open', $7)`,
		r.ID, reporterID, targetType, targetID, reason, detail, r.CreatedAt)
	return r, err
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.POST("/reports", h.create)
}

type createReq struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Detail     string `json:"detail"`
}

func (h *Handler) create(c echo.Context) error {
	var req createReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	if !validTargets[req.TargetType] || req.TargetID == "" || req.Reason == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "target_type, target_id e reason são obrigatórios")
	}
	if len(req.Reason) > 200 || len(req.Detail) > 2000 {
		return echo.NewHTTPError(http.StatusBadRequest, "texto muito longo")
	}
	r, err := h.svc.Create(c.Request().Context(), auth.UserID(c), req.TargetType, req.TargetID, req.Reason, req.Detail)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao registrar denúncia")
	}
	return c.JSON(http.StatusCreated, r)
}
