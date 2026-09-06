// Package profile cobre as rotas de perfil que faltavam (plano §10):
// PATCH /v1/me, GET /v1/me/badges, GET /v1/me/calibration,
// GET/POST/DELETE /v1/devices.
package profile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// --- PATCH /v1/me ---

type ProfilePatch struct {
	DisplayName *string         `json:"display_name,omitempty"`
	ColorHex    *string         `json:"color_hex,omitempty"`
	Privacy     json.RawMessage `json:"privacy,omitempty"` // { home_blur_m, default_visibility, ghost }
}

var ErrBadColor = errors.New("cor inválida (use #RRGGBB)")

func (s *Service) UpdateMe(ctx context.Context, userID string, p ProfilePatch) error {
	if p.ColorHex != nil && !hexColor.MatchString(*p.ColorHex) {
		return ErrBadColor
	}
	if p.DisplayName != nil && len(*p.DisplayName) > 60 {
		return errors.New("nome muito longo")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET
			display_name = COALESCE($2, display_name),
			color_hex    = COALESCE($3, color_hex),
			privacy_jsonb = CASE WHEN $4::jsonb IS NULL THEN privacy_jsonb
			                     ELSE privacy_jsonb || $4::jsonb END,
			updated_at = now()
		WHERE id = $1`, userID, p.DisplayName, p.ColorHex, nullJSON(p.Privacy))
	return err
}

func nullJSON(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	return []byte(r)
}

// --- GET /v1/me/badges ---

type Badge struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Shape       string    `json:"shape,omitempty"`
	Type        string    `json:"type,omitempty"`
	Source      string    `json:"source,omitempty"`
	EarnedAt    time.Time `json:"earned_at"`
}

func (s *Service) Badges(ctx context.Context, userID string) ([]Badge, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.code, b.name, COALESCE(b.description,''), COALESCE(b.shape,''),
		       COALESCE(b.type,''), COALESCE(ub.source,''), ub.earned_at
		FROM user_badges ub JOIN badges b ON b.code = ub.badge_code
		WHERE ub.user_id = $1 ORDER BY ub.earned_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Badge
	for rows.Next() {
		var b Badge
		if err := rows.Scan(&b.Code, &b.Name, &b.Description, &b.Shape, &b.Type, &b.Source, &b.EarnedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// --- GET /v1/me/calibration ---

func (s *Service) Calibration(ctx context.Context, userID string) (map[string]any, error) {
	var strideLen, confidence float64
	var samples int
	var updated time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT stride_len_m, confidence, sample_count, updated_at
		FROM user_stride_calibration WHERE user_id = $1`, userID).Scan(&strideLen, &confidence, &samples, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{"calibrated": false}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"calibrated": true, "stride_len_m": strideLen, "confidence": confidence,
		"sample_count": samples, "updated_at": updated,
	}, nil
}

// --- /v1/devices ---

type Device struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Brand      string     `json:"brand,omitempty"`
	Model      string     `json:"model,omitempty"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

var validKind = map[string]bool{"phone": true, "watch": true, "footpod": true, "hrm": true}

func (s *Service) ListDevices(ctx context.Context, userID string) ([]Device, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, COALESCE(brand,''), COALESCE(model,''), last_seen_at, created_at
		FROM devices WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Kind, &d.Brand, &d.Model, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Service) AddDevice(ctx context.Context, userID, kind, brand, model string) (*Device, error) {
	if !validKind[kind] {
		return nil, errors.New("kind deve ser phone|watch|footpod|hrm")
	}
	d := &Device{ID: id.New(), Kind: kind, Brand: brand, Model: model, CreatedAt: time.Now().UTC()}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (id, user_id, kind, brand, model, last_seen_at)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), now())`,
		d.ID, userID, kind, brand, model)
	return d, err
}

func (s *Service) DeleteDevice(ctx context.Context, userID, deviceID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM devices WHERE id = $1 AND user_id = $2`, deviceID, userID)
	return err
}

// --- handler ---

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.PATCH("/me", h.patchMe)
	g.GET("/me/badges", h.badges)
	g.GET("/me/calibration", h.calibration)
	g.GET("/devices", h.listDevices)
	g.POST("/devices", h.addDevice)
	g.DELETE("/devices/:id", h.deleteDevice)
}

func (h *Handler) patchMe(c echo.Context) error {
	var p ProfilePatch
	if err := c.Bind(&p); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	if err := h.svc.UpdateMe(c.Request().Context(), auth.UserID(c), p); err != nil {
		if errors.Is(err, ErrBadColor) {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao atualizar perfil")
	}
	return c.JSON(http.StatusOK, map[string]any{"updated": true})
}

func (h *Handler) badges(c echo.Context) error {
	b, err := h.svc.Badges(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar selos")
	}
	return c.JSON(http.StatusOK, map[string]any{"badges": b})
}

func (h *Handler) calibration(c echo.Context) error {
	cal, err := h.svc.Calibration(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, cal)
}

func (h *Handler) listDevices(c echo.Context) error {
	d, err := h.svc.ListDevices(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar dispositivos")
	}
	return c.JSON(http.StatusOK, map[string]any{"devices": d})
}

type addDeviceReq struct {
	Kind  string `json:"kind"`
	Brand string `json:"brand"`
	Model string `json:"model"`
}

func (h *Handler) addDevice(c echo.Context) error {
	var req addDeviceReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	d, err := h.svc.AddDevice(c.Request().Context(), auth.UserID(c), req.Kind, req.Brand, req.Model)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusCreated, d)
}

func (h *Handler) deleteDevice(c echo.Context) error {
	if err := h.svc.DeleteDevice(c.Request().Context(), auth.UserID(c), c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao remover dispositivo")
	}
	return c.NoContent(http.StatusNoContent)
}
