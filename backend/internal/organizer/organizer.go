// Package organizer é o portal de organizadores (plano §12, §13): namespace
// /v1/organizer/*, role organizer ou admin + 2FA verificado. Um organizador só
// enxerga e edita os eventos das organizações que ele possui.
package organizer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrNoOrg    = errors.New("você não é dono de nenhuma organização")
	ErrNotYours = errors.New("evento não pertence a você")
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// ownsEvent confirma que o evento pertence a uma organização do usuário.
func (s *Service) ownsEvent(ctx context.Context, userID, eventRef string) (string, error) {
	var eventID string
	err := s.pool.QueryRow(ctx, `
		SELECT e.id FROM events e
		JOIN organizers o ON o.id = e.organizer_id
		WHERE (e.id = $1 OR e.slug = $1) AND o.owner_id = $2`, eventRef, userID).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotYours
	}
	return eventID, err
}

func (s *Service) MyOrgs(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, kind, plan, status FROM organizers WHERE owner_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var oid, name, kind, plan, status string
		if err := rows.Scan(&oid, &name, &kind, &plan, &status); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": oid, "name": name, "kind": kind, "plan": plan, "status": status})
	}
	return out, nil
}

func (s *Service) firstOrg(ctx context.Context, userID string) (string, error) {
	var oid string
	err := s.pool.QueryRow(ctx, `SELECT id FROM organizers WHERE owner_id = $1 ORDER BY created_at LIMIT 1`, userID).Scan(&oid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoOrg
	}
	return oid, err
}

type EventInput struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Location    string    `json:"location_name"`
}

// CreateEvent cria um evento em rascunho (status draft — o admin aprova depois).
func (s *Service) CreateEvent(ctx context.Context, userID string, in EventInput) (string, error) {
	orgID, err := s.firstOrg(ctx, userID)
	if err != nil {
		return "", err
	}
	if in.Type == "" {
		in.Type = "race"
	}
	if in.Location == "" {
		in.Location = "Fortaleza, CE"
	}
	eid := id.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO events (id, slug, organizer_id, title, description, type, starts_at, ends_at, location_name, status)
		VALUES ($1, $2, $3, $4, $5, $6::event_type, $7, $8, $9, 'draft')`,
		eid, in.Slug, orgID, in.Title, in.Description, in.Type, in.StartsAt, in.EndsAt, in.Location)
	return eid, err
}

func (s *Service) UpdateEvent(ctx context.Context, userID, eventRef string, in EventInput) error {
	eid, err := s.ownsEvent(ctx, userID, eventRef)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE events SET
			title = COALESCE(NULLIF($2,''), title),
			description = COALESCE(NULLIF($3,''), description),
			location_name = COALESCE(NULLIF($4,''), location_name),
			starts_at = COALESCE($5, starts_at),
			ends_at = COALESCE($6, ends_at),
			updated_at = now()
		WHERE id = $1`, eid, in.Title, in.Description, in.Location, nullTime(in.StartsAt), nullTime(in.EndsAt))
	return err
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type PriceInput struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	AmountCents int    `json:"amount_cents"`
	Quota       int    `json:"quota"`
}

func (s *Service) AddPrice(ctx context.Context, userID, eventRef string, in PriceInput) (string, error) {
	eid, err := s.ownsEvent(ctx, userID, eventRef)
	if err != nil {
		return "", err
	}
	if in.AmountCents < 0 || in.Quota <= 0 {
		return "", errors.New("amount_cents e quota inválidos")
	}
	pid := id.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO event_prices (id, event_id, name, category, amount_cents, quota)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4,''),'geral'), $5, $6)`,
		pid, eid, in.Name, in.Category, in.AmountCents, in.Quota)
	return pid, err
}

type StationInput struct {
	Name    string  `json:"name"`
	Role    string  `json:"role"`
	Ord     int     `json:"ord"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	RadiusM int     `json:"radius_m"`
}

func (s *Service) AddStation(ctx context.Context, userID, eventRef string, in StationInput) (string, error) {
	eid, err := s.ownsEvent(ctx, userID, eventRef)
	if err != nil {
		return "", err
	}
	if in.Role == "" {
		in.Role = "checkpoint"
	}
	if in.RadiusM <= 0 {
		in.RadiusM = 30
	}
	sid := id.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO event_stations (id, event_id, name, role, ord, geom, radius_m)
		VALUES ($1, $2, $3, $4::station_role, $5, ST_SetSRID(ST_Point($6,$7),4326), $8)`,
		sid, eid, in.Name, in.Role, in.Ord, in.Lng, in.Lat, in.RadiusM)
	return sid, err
}

type CouponInput struct {
	Code         string     `json:"code"`
	DiscountType string     `json:"discount_type"`
	Value        int        `json:"value"`
	MaxUses      int        `json:"max_uses"`
	ValidUntil   *time.Time `json:"valid_until"`
}

func (s *Service) AddCoupon(ctx context.Context, userID, eventRef string, in CouponInput) (string, error) {
	eid, err := s.ownsEvent(ctx, userID, eventRef)
	if err != nil {
		return "", err
	}
	if in.Code == "" || in.Value <= 0 {
		return "", errors.New("code e value obrigatórios")
	}
	if in.DiscountType != "fixed" {
		in.DiscountType = "percent"
	}
	cid := id.New()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO coupons (id, code, scope, event_id, discount_type, value, max_uses, valid_until)
		VALUES ($1, $2, 'event', $3, $4, $5, $6, $7)`,
		cid, in.Code, eid, in.DiscountType, in.Value, in.MaxUses, in.ValidUntil)
	return cid, err
}

// IssueScannerCredential gera um token para a equipe do evento (mostrado uma vez).
func (s *Service) IssueScannerCredential(ctx context.Context, userID, eventRef, stationID, label, role string) (string, error) {
	eid, err := s.ownsEvent(ctx, userID, eventRef)
	if err != nil {
		return "", err
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := "frscan_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	if role == "" {
		role = "checkpoint"
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO scanner_credentials (id, event_id, station_id, label, token_hash, role, created_by)
		VALUES ($1, $2, NULLIF($3,''), $4, $5, $6, $7)`,
		id.New(), eid, stationID, label, hex.EncodeToString(sum[:]), role, userID)
	return token, err
}

func (s *Service) Participants(ctx context.Context, userID, eventRef string, limit int) ([]map[string]any, error) {
	eid, err := s.ownsEvent(ctx, userID, eventRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.username, u.athlete_id, COALESCE(ep.bib_number,''), ep.category,
		       ep.shirt_size, ep.joined_at, ep.completed_at IS NOT NULL
		FROM event_participants ep JOIN users u ON u.id = ep.user_id
		WHERE ep.event_id = $1 ORDER BY ep.joined_at ASC LIMIT $2`, eid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var uname, aid, bib, cat, shirt string
		var joined time.Time
		var done bool
		if err := rows.Scan(&uname, &aid, &bib, &cat, &shirt, &joined, &done); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"username": uname, "athlete_id": aid, "bib_number": bib, "category": cat,
			"shirt_size": shirt, "joined_at": joined, "completed": done,
		})
	}
	return out, nil
}

// --- handler ---

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// requireOrganizer barra quem não é organizer nem admin (404, não revela o namespace).
func requireOrganizer(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		switch role, _ := c.Get("role").(string); role {
		case "organizer", "admin":
			return next(c)
		default:
			return echo.NewHTTPError(http.StatusNotFound, "não encontrado")
		}
	}
}

func (h *Handler) Register(secured *echo.Group) {
	g := secured.Group("/organizer", requireOrganizer, auth.RequireMFA(false))
	g.GET("/me", h.myOrgs)
	g.POST("/events", h.createEvent)
	g.PATCH("/events/:id", h.updateEvent)
	g.POST("/events/:id/prices", h.addPrice)
	g.POST("/events/:id/stations", h.addStation)
	g.POST("/events/:id/coupons", h.addCoupon)
	g.POST("/events/:id/scanner-credentials", h.issueCredential)
	g.GET("/events/:id/participants", h.participants)
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, ErrNoOrg):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case errors.Is(err, ErrNotYours):
		return echo.NewHTTPError(http.StatusNotFound, "evento não encontrado")
	default:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
}

func (h *Handler) myOrgs(c echo.Context) error {
	o, err := h.svc.MyOrgs(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
	}
	return c.JSON(http.StatusOK, map[string]any{"organizers": o})
}

func (h *Handler) createEvent(c echo.Context) error {
	var in EventInput
	if err := c.Bind(&in); err != nil || in.Slug == "" || in.Title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "slug e title obrigatórios")
	}
	eid, err := h.svc.CreateEvent(c.Request().Context(), auth.UserID(c), in)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": eid, "status": "draft"})
}

func (h *Handler) updateEvent(c echo.Context) error {
	var in EventInput
	_ = c.Bind(&in)
	if err := h.svc.UpdateEvent(c.Request().Context(), auth.UserID(c), c.Param("id"), in); err != nil {
		return mapErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"updated": true})
}

func (h *Handler) addPrice(c echo.Context) error {
	var in PriceInput
	if err := c.Bind(&in); err != nil || in.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name obrigatório")
	}
	pid, err := h.svc.AddPrice(c.Request().Context(), auth.UserID(c), c.Param("id"), in)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": pid})
}

func (h *Handler) addStation(c echo.Context) error {
	var in StationInput
	if err := c.Bind(&in); err != nil || in.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name obrigatório")
	}
	sid, err := h.svc.AddStation(c.Request().Context(), auth.UserID(c), c.Param("id"), in)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": sid})
}

func (h *Handler) addCoupon(c echo.Context) error {
	var in CouponInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "dados inválidos")
	}
	cid, err := h.svc.AddCoupon(c.Request().Context(), auth.UserID(c), c.Param("id"), in)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": cid})
}

func (h *Handler) issueCredential(c echo.Context) error {
	var in struct {
		StationID string `json:"station_id"`
		Label     string `json:"label"`
		Role      string `json:"role"`
	}
	if err := c.Bind(&in); err != nil || in.Label == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "label obrigatório")
	}
	token, err := h.svc.IssueScannerCredential(c.Request().Context(), auth.UserID(c), c.Param("id"), in.StationID, in.Label, in.Role)
	if err != nil {
		return mapErr(err)
	}
	// token é mostrado UMA vez — só o hash fica no banco.
	return c.JSON(http.StatusCreated, map[string]any{"token": token, "note": "guarde agora — não será exibido de novo"})
}

func (h *Handler) participants(c echo.Context) error {
	list, err := h.svc.Participants(c.Request().Context(), auth.UserID(c), c.Param("id"), 2000)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"participants": list})
}
