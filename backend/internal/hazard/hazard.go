// Package hazard é a camada colaborativa de condições da via (plano §6):
// iluminação apagada, obra, alagamento, calçada interditada, cachorro solto,
// trecho de atenção redobrada. Linguagem neutra, sem nomear pessoas. As
// marcações decaem no tempo e sobem/descem por confirmação da comunidade.
package hazard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var ErrNotFound = errors.New("marcação não encontrada")

var validTypes = map[string]bool{
	"lighting": true, "construction": true, "flooding": true,
	"blocked_sidewalk": true, "loose_dog": true, "traffic": true, "attention": true,
}

type Hazard struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Severity  int       `json:"severity"`
	Note      string    `json:"note,omitempty"`
	Confirms  int       `json:"confirms"`
	Disputes  int       `json:"disputes"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) List(ctx context.Context, bbox [4]float64) ([]Hazard, error) {
	// bbox = [minLng, minLat, maxLng, maxLat]; zerado → cidade toda.
	q := `
		SELECT id, type, ST_Y(geom), ST_X(geom), severity, COALESCE(note,''),
		       confirms, disputes, expires_at, created_at
		FROM hazard_reports
		WHERE status = 'active' AND expires_at > now()
		  AND ($1 = 0 AND $2 = 0 AND $3 = 0 AND $4 = 0
		       OR geom && ST_MakeEnvelope($1, $2, $3, $4, 4326))
		ORDER BY created_at DESC LIMIT 500`
	rows, err := s.pool.Query(ctx, q, bbox[0], bbox[1], bbox[2], bbox[3])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Hazard
	for rows.Next() {
		var h Hazard
		if err := rows.Scan(&h.ID, &h.Type, &h.Lat, &h.Lng, &h.Severity, &h.Note,
			&h.Confirms, &h.Disputes, &h.ExpiresAt, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, userID, typ string, lat, lng float64, severity int, note string) (*Hazard, error) {
	if severity < 1 || severity > 5 {
		severity = 2
	}
	h := &Hazard{
		ID: id.New(), Type: typ, Lat: lat, Lng: lng, Severity: severity, Note: note,
		ExpiresAt: time.Now().Add(14 * 24 * time.Hour), CreatedAt: time.Now().UTC(),
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO hazard_reports (id, reporter_id, type, geom, severity, note)
		VALUES ($1, $2, $3, ST_SetSRID(ST_Point($4,$5),4326), $6, NULLIF($7,''))`,
		h.ID, userID, typ, lng, lat, severity, note)
	return h, err
}

// Vote registra confirmação (+1) ou disputa (-1). Um voto por usuário; disputas
// suficientes acima das confirmações expiram a marcação.
func (s *Service) Vote(ctx context.Context, userID, hazardID string, delta int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT true FROM hazard_reports WHERE id = $1 AND status = 'active'`, hazardID,
	).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO hazard_votes (hazard_id, user_id, vote) VALUES ($1, $2, $3)
		ON CONFLICT (hazard_id, user_id) DO UPDATE SET vote = EXCLUDED.vote`,
		hazardID, userID, delta); err != nil {
		return err
	}

	var confirms, disputes int
	if err := tx.QueryRow(ctx, `
		WITH v AS (
			UPDATE hazard_reports SET
				confirms = (SELECT count(*) FROM hazard_votes WHERE hazard_id = $1 AND vote > 0),
				disputes = (SELECT count(*) FROM hazard_votes WHERE hazard_id = $1 AND vote < 0)
			WHERE id = $1
			RETURNING confirms, disputes
		) SELECT confirms, disputes FROM v`, hazardID).Scan(&confirms, &disputes); err != nil {
		return err
	}
	if disputes >= 3 && disputes > confirms {
		_, _ = tx.Exec(ctx, `UPDATE hazard_reports SET status = 'removed' WHERE id = $1`, hazardID)
	}
	return tx.Commit(ctx)
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *echo.Group) {
	g.GET("/hazards", h.list)
	g.POST("/hazards", h.create)
	g.POST("/hazards/:id/confirm", h.confirm)
	g.POST("/hazards/:id/dispute", h.dispute)
}

func parseBBox(s string) [4]float64 {
	var b [4]float64
	if s == "" {
		return b
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return b
	}
	for i := 0; i < 4; i++ {
		b[i], _ = strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
	}
	return b
}

func (h *Handler) list(c echo.Context) error {
	hz, err := h.svc.List(c.Request().Context(), parseBBox(c.QueryParam("bbox")))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao listar marcações")
	}
	return c.JSON(http.StatusOK, map[string]any{"hazards": hz})
}

type createReq struct {
	Type     string  `json:"type"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Severity int     `json:"severity"`
	Note     string  `json:"note"`
}

func (h *Handler) create(c echo.Context) error {
	var req createReq
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	if !validTypes[req.Type] {
		return echo.NewHTTPError(http.StatusBadRequest, "tipo de marcação inválido")
	}
	if len(req.Note) > 280 {
		return echo.NewHTTPError(http.StatusBadRequest, "nota muito longa")
	}
	hz, err := h.svc.Create(c.Request().Context(), auth.UserID(c), req.Type, req.Lat, req.Lng, req.Severity, req.Note)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao registrar marcação")
	}
	return c.JSON(http.StatusCreated, hz)
}

func (h *Handler) confirm(c echo.Context) error { return h.vote(c, 1) }
func (h *Handler) dispute(c echo.Context) error { return h.vote(c, -1) }

func (h *Handler) vote(c echo.Context, delta int) error {
	err := h.svc.Vote(c.Request().Context(), auth.UserID(c), c.Param("id"), delta)
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "marcação não encontrada")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro ao votar")
	}
	return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
}
