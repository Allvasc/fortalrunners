// Package admin é o namespace isolado /v1/admin/* (plano §13). Toda rota exige
// role admin ou moderator + 2FA verificado, e toda mutação grava em audit_log.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Allvasc/fortalrunners/backend/internal/auth"
	"github.com/Allvasc/fortalrunners/backend/internal/landmark"
	"github.com/Allvasc/fortalrunners/backend/internal/platform/queue"
	"github.com/Allvasc/fortalrunners/backend/internal/route"
)

// LandmarkModerator e RouteModerator são as portas de moderação implementadas
// pelos serviços de landmark e route (evita duplicar a lógica de concessão de selo).
type LandmarkModerator interface {
	PendingCheckins(ctx context.Context, limit int) ([]landmark.PendingCheckin, error)
	ModerateCheckin(ctx context.Context, checkinID, decision, moderatorID string) error
}

type RouteModerator interface {
	PendingReviews(ctx context.Context, limit int) ([]route.PendingReview, error)
	ModerateReview(ctx context.Context, reviewID, decision string) error
}

type Handler struct {
	store     *store
	pub       queue.Publisher
	landmarks LandmarkModerator
	routes    RouteModerator
}

func NewHandler(pool *pgxpool.Pool, pub queue.Publisher, lm LandmarkModerator, rm RouteModerator) *Handler {
	return &Handler{store: newStore(pool), pub: pub, landmarks: lm, routes: rm}
}

// Register monta /v1/admin/* já sob o Bearer middleware. Aplica o gate de role +
// 2FA aqui dentro para o namespace inteiro.
func (h *Handler) Register(secured *echo.Group) {
	g := secured.Group("/admin", requireStaff, auth.RequireMFA(false))

	g.GET("/users", h.userSearch)
	g.POST("/users/:id/status", h.userStatus)
	g.POST("/users/:id/role", h.userRole)

	g.GET("/runs/flagged", h.flaggedRuns)
	g.POST("/runs/:id/review", h.reviewRun)

	g.GET("/risk-zones", h.riskZones)
	g.POST("/risk-zones", h.riskZoneUpsert)
	g.PATCH("/risk-zones/:id", h.riskZoneUpsert)
	g.DELETE("/risk-zones/:id", h.riskZoneDelete)

	g.GET("/config", h.configGet)
	g.PUT("/config/:key", h.configSet)

	g.GET("/audit", h.auditList)

	// --- moderação (plano §13) ---
	g.GET("/landmark-checkins", h.pendingCheckins)
	g.POST("/landmark-checkins/:id/moderate", h.moderateCheckin)
	g.GET("/route-reviews", h.pendingReviews)
	g.POST("/route-reviews/:id/moderate", h.moderateReview)
	g.GET("/reports", h.reportList)
	g.POST("/reports/:id/resolve", h.reportResolve)
	g.GET("/hazards", h.hazardList)
	g.POST("/hazards/:id/remove", h.hazardRemove)
	g.GET("/refunds", h.refundList)
	g.POST("/refunds/:id/decide", h.refundDecide)
	g.POST("/organizers", h.createOrganizer)
}

func (h *Handler) createOrganizer(c echo.Context) error {
	var in struct {
		OwnerID string `json:"owner_id"`
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Email   string `json:"contact_email"`
	}
	if err := c.Bind(&in); err != nil || in.OwnerID == "" || in.Name == "" || in.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "owner_id, name e contact_email obrigatórios")
	}
	ctx := c.Request().Context()
	oid, err := h.store.createOrganizer(ctx, in.OwnerID, in.Name, in.Kind, in.Email)
	if err != nil {
		return internalErr(err)
	}
	aid, arole, ip := h.actor(c)
	h.store.audit(ctx, aid, arole, "organizer.create", "organizer", oid,
		map[string]any{"owner_id": in.OwnerID, "name": in.Name}, ip)
	return c.JSON(http.StatusCreated, map[string]any{"id": oid})
}

// --- moderação ---

func decision(c echo.Context) (string, bool) {
	var in struct {
		Decision string `json:"decision"`
	}
	_ = c.Bind(&in)
	if in.Decision != "approve" && in.Decision != "reject" {
		return "", false
	}
	return in.Decision, true
}

func (h *Handler) pendingCheckins(c echo.Context) error {
	list, err := h.landmarks.PendingCheckins(c.Request().Context(), clampLimit(c.QueryParam("limit"), 50, 200))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"checkins": list})
}

func (h *Handler) moderateCheckin(c echo.Context) error {
	d, ok := decision(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "decision deve ser approve ou reject")
	}
	ctx := c.Request().Context()
	aid, arole, ip := h.actor(c)
	if err := h.landmarks.ModerateCheckin(ctx, c.Param("id"), d, aid); err != nil {
		return mapErr(err)
	}
	h.store.audit(ctx, aid, arole, "landmark_checkin."+d, "landmark_checkin", c.Param("id"), nil, ip)
	return c.JSON(http.StatusOK, map[string]any{"status": d})
}

func (h *Handler) pendingReviews(c echo.Context) error {
	list, err := h.routes.PendingReviews(c.Request().Context(), clampLimit(c.QueryParam("limit"), 50, 200))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"reviews": list})
}

func (h *Handler) moderateReview(c echo.Context) error {
	d, ok := decision(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "decision deve ser approve ou reject")
	}
	ctx := c.Request().Context()
	aid, arole, ip := h.actor(c)
	if err := h.routes.ModerateReview(ctx, c.Param("id"), d); err != nil {
		return mapErr(err)
	}
	h.store.audit(ctx, aid, arole, "route_review."+d, "route_review", c.Param("id"), nil, ip)
	return c.JSON(http.StatusOK, map[string]any{"status": d})
}

func (h *Handler) reportList(c echo.Context) error {
	status := c.QueryParam("status")
	if status == "" {
		status = "open"
	}
	list, err := h.store.reportList(c.Request().Context(), status, clampLimit(c.QueryParam("limit"), 50, 200))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"reports": list})
}

func (h *Handler) reportResolve(c echo.Context) error {
	var in struct {
		Outcome string `json:"outcome"` // actioned | dismissed
	}
	_ = c.Bind(&in)
	if in.Outcome != "actioned" && in.Outcome != "dismissed" {
		return echo.NewHTTPError(http.StatusBadRequest, "outcome deve ser actioned ou dismissed")
	}
	ctx := c.Request().Context()
	aid, arole, ip := h.actor(c)
	if err := h.store.resolveReport(ctx, c.Param("id"), in.Outcome, aid); err != nil {
		return mapErr(err)
	}
	h.store.audit(ctx, aid, arole, "report."+in.Outcome, "report", c.Param("id"), nil, ip)
	return c.JSON(http.StatusOK, map[string]any{"status": in.Outcome})
}

func (h *Handler) hazardList(c echo.Context) error {
	list, err := h.store.hazardList(c.Request().Context(), clampLimit(c.QueryParam("limit"), 100, 500))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"hazards": list})
}

func (h *Handler) hazardRemove(c echo.Context) error {
	ctx := c.Request().Context()
	aid, arole, ip := h.actor(c)
	if err := h.store.removeHazard(ctx, c.Param("id")); err != nil {
		return mapErr(err)
	}
	h.store.audit(ctx, aid, arole, "hazard.remove", "hazard", c.Param("id"), nil, ip)
	return c.JSON(http.StatusOK, map[string]any{"status": "removed"})
}

func (h *Handler) refundList(c echo.Context) error {
	status := c.QueryParam("status")
	if status == "" {
		status = "requested"
	}
	list, err := h.store.refundList(c.Request().Context(), status, clampLimit(c.QueryParam("limit"), 50, 200))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"refunds": list})
}

func (h *Handler) refundDecide(c echo.Context) error {
	var in struct {
		Decision string `json:"decision"` // approve | deny
	}
	_ = c.Bind(&in)
	if in.Decision != "approve" && in.Decision != "deny" {
		return echo.NewHTTPError(http.StatusBadRequest, "decision deve ser approve ou deny")
	}
	ctx := c.Request().Context()
	aid, arole, ip := h.actor(c)
	if err := h.store.decideRefund(ctx, c.Param("id"), in.Decision, aid); err != nil {
		return mapErr(err)
	}
	h.store.audit(ctx, aid, arole, "refund."+in.Decision, "refund", c.Param("id"), nil, ip)
	return c.JSON(http.StatusOK, map[string]any{"status": in.Decision})
}

// requireStaff barra quem não é admin nem moderator (antes mesmo do 2FA).
func requireStaff(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		switch role(c) {
		case "admin", "moderator":
			return next(c)
		default:
			return echo.NewHTTPError(http.StatusNotFound, "não encontrado") // não revela o namespace
		}
	}
}

func role(c echo.Context) string { v, _ := c.Get("role").(string); return v }

func (h *Handler) actor(c echo.Context) (id, role, ip string) {
	return auth.UserID(c), c.Get("role").(string), c.RealIP()
}

// --- usuários ---

func (h *Handler) userSearch(c echo.Context) error {
	limit := clampLimit(c.QueryParam("limit"), 50, 200)
	list, err := h.store.userSearch(c.Request().Context(), strings.TrimSpace(c.QueryParam("q")), limit)
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"users": list})
}

func (h *Handler) userStatus(c echo.Context) error {
	var in struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := c.Bind(&in); err != nil || !isStatus(in.Status) {
		return echo.NewHTTPError(http.StatusBadRequest, "status inválido")
	}
	ctx := c.Request().Context()
	uid := c.Param("id")
	before, err := h.store.userByID(ctx, uid)
	if err != nil {
		return mapErr(err)
	}
	if err := h.store.setUserStatus(ctx, uid, in.Status); err != nil {
		return mapErr(err)
	}
	aid, arole, ip := h.actor(c)
	h.store.audit(ctx, aid, arole, "user.status", "user", uid,
		map[string]any{"from": before.Status, "to": in.Status, "reason": in.Reason}, ip)
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) userRole(c echo.Context) error {
	var in struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&in); err != nil || !isRole(in.Role) {
		return echo.NewHTTPError(http.StatusBadRequest, "role inválido")
	}
	if role(c) != "admin" {
		return echo.NewHTTPError(http.StatusForbidden, "só admin muda role")
	}
	ctx := c.Request().Context()
	uid := c.Param("id")
	before, err := h.store.userByID(ctx, uid)
	if err != nil {
		return mapErr(err)
	}
	if err := h.store.setUserRole(ctx, uid, in.Role); err != nil {
		return mapErr(err)
	}
	aid, arole, ip := h.actor(c)
	h.store.audit(ctx, aid, arole, "user.role", "user", uid,
		map[string]any{"from": before.Role, "to": in.Role}, ip)
	return c.NoContent(http.StatusNoContent)
}

// --- corridas sinalizadas ---

func (h *Handler) flaggedRuns(c echo.Context) error {
	list, err := h.store.flaggedRuns(c.Request().Context(), clampLimit(c.QueryParam("limit"), 50, 200))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"runs": list})
}

func (h *Handler) reviewRun(c echo.Context) error {
	var in struct {
		Decision      string `json:"decision"` // valid | rejected
		VoidTerritory bool   `json:"void_territory"`
	}
	if err := c.Bind(&in); err != nil || (in.Decision != "valid" && in.Decision != "rejected") {
		return echo.NewHTTPError(http.StatusBadRequest, "decision = valid | rejected")
	}
	ctx := c.Request().Context()
	runID := c.Param("id")
	userID, err := h.store.reviewRun(ctx, runID, in.Decision, in.VoidTerritory)
	if err != nil {
		return mapErr(err)
	}
	if in.Decision == "valid" {
		payload, _ := json.Marshal(map[string]string{"run_id": runID, "user_id": userID})
		_ = h.pub.Publish(ctx, queue.SubjectRunUploaded, payload)
	}
	aid, arole, ip := h.actor(c)
	h.store.audit(ctx, aid, arole, "run.review", "run", runID,
		map[string]any{"decision": in.Decision, "void_territory": in.VoidTerritory}, ip)
	return c.NoContent(http.StatusNoContent)
}

// --- zonas de risco ---

func (h *Handler) riskZones(c echo.Context) error {
	var bbox [4]float64
	if raw := c.QueryParam("bbox"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			return echo.NewHTTPError(http.StatusBadRequest, "bbox inválido")
		}
		for i, p := range parts {
			bbox[i], _ = strconv.ParseFloat(strings.TrimSpace(p), 64)
		}
	}
	fc, err := h.store.riskZones(c.Request().Context(), bbox)
	if err != nil {
		return internalErr(err)
	}
	return c.JSONBlob(http.StatusOK, []byte(fc))
}

type riskZoneInput struct {
	ID         string          `json:"id"`
	CityID     string          `json:"city_id"`
	Geom       json.RawMessage `json:"geom"` // GeoJSON de geometria (Polygon/MultiPolygon)
	Severity   int             `json:"severity"`
	Source     string          `json:"source"`
	Note       string          `json:"note"`
	Status     string          `json:"status"`
	ActiveFrom *string         `json:"active_from"`
	ActiveTo   *string         `json:"active_to"`
}

func (h *Handler) riskZoneUpsert(c echo.Context) error {
	var in riskZoneInput
	if err := c.Bind(&in); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "corpo inválido")
	}
	if p := c.Param("id"); p != "" {
		in.ID = p
	}
	if in.ID == "" && (len(in.Geom) == 0 || in.Severity < 1 || in.Severity > 5) {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "geom e severity (1–5) obrigatórios")
	}
	ctx := c.Request().Context()
	zid, err := h.store.upsertRiskZone(ctx, in)
	if err != nil {
		return mapErr(err)
	}
	aid, arole, ip := h.actor(c)
	action := "risk_zone.update"
	if c.Request().Method == http.MethodPost && c.Param("id") == "" {
		action = "risk_zone.create"
	}
	h.store.audit(ctx, aid, arole, action, "risk_zone", zid,
		map[string]any{"severity": in.Severity, "status": in.Status}, ip)
	return c.JSON(http.StatusOK, map[string]any{"id": zid})
}

func (h *Handler) riskZoneDelete(c echo.Context) error {
	ctx := c.Request().Context()
	zid := c.Param("id")
	if err := h.store.deleteRiskZone(ctx, zid); err != nil {
		return mapErr(err)
	}
	aid, arole, ip := h.actor(c)
	h.store.audit(ctx, aid, arole, "risk_zone.delete", "risk_zone", zid, nil, ip)
	return c.NoContent(http.StatusNoContent)
}

// --- config ---

func (h *Handler) configGet(c echo.Context) error {
	cfg, err := h.store.configAll(c.Request().Context())
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) configSet(c echo.Context) error {
	key := c.Param("key")
	var value json.RawMessage
	if err := c.Bind(&value); err != nil || len(value) == 0 || !json.Valid(value) {
		return echo.NewHTTPError(http.StatusBadRequest, "valor JSON obrigatório")
	}
	ctx := c.Request().Context()
	if err := h.store.setConfig(ctx, key, value, auth.UserID(c)); err != nil {
		return mapErr(err)
	}
	aid, arole, ip := h.actor(c)
	h.store.audit(ctx, aid, arole, "config.set", "config", key, map[string]any{"value": value}, ip)
	return c.NoContent(http.StatusNoContent)
}

// --- auditoria ---

func (h *Handler) auditList(c echo.Context) error {
	list, err := h.store.auditList(c.Request().Context(),
		c.QueryParam("target_type"), c.QueryParam("target_id"),
		clampLimit(c.QueryParam("limit"), 100, 500))
	if err != nil {
		return internalErr(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"entries": list})
}

// --- helpers ---

func isStatus(s string) bool {
	switch s {
	case "active", "suspended", "shadow_banned", "banned":
		return true
	}
	return false
}
func isRole(s string) bool {
	switch s {
	case "runner", "organizer", "moderator", "admin":
		return true
	}
	return false
}

func clampLimit(raw string, def, max int) int {
	n, _ := strconv.Atoi(raw)
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func mapErr(err error) error {
	if errors.Is(err, errNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "não encontrado")
	}
	return internalErr(err)
}

func internalErr(_ error) error {
	return echo.NewHTTPError(http.StatusInternalServerError, "erro interno")
}
