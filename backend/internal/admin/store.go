package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var errNotFound = errors.New("não encontrado")

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

// --- auditoria (append-only) ---

func (s *store) audit(ctx context.Context, actorID, actorRole, action, targetType, targetID string, diff any, ip string) {
	d, _ := json.Marshal(diff)
	if len(d) == 0 {
		d = []byte("{}")
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO audit_log (id, actor_id, actor_role, action, target_type, target_id, diff_jsonb, ip)
		VALUES ($1,$2,$3,$4,$5,nullif($6,''),$7, nullif($8,'')::inet)`,
		id.New(), actorID, actorRole, action, targetType, targetID, d, ip)
}

type auditRow struct {
	ID         string          `json:"id"`
	ActorID    *string         `json:"actor_id"`
	ActorRole  string          `json:"actor_role"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   *string         `json:"target_id"`
	Diff       json.RawMessage `json:"diff"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (s *store) auditList(ctx context.Context, targetType, targetID string, limit int) ([]auditRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_id, actor_role, action, target_type, target_id, diff_jsonb, created_at
		FROM audit_log
		WHERE ($1 = '' OR target_type = $1) AND ($2 = '' OR target_id = $2)
		ORDER BY created_at DESC LIMIT $3`, targetType, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auditRow{}
	for rows.Next() {
		var a auditRow
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorRole, &a.Action, &a.TargetType, &a.TargetID, &a.Diff, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- usuários ---

type userRow struct {
	ID        string    `json:"id"`
	AthleteID string    `json:"athlete_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *store) userSearch(ctx context.Context, q string, limit int) ([]userRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, athlete_id, username, email, role, status, created_at
		FROM users
		WHERE deleted_at IS NULL AND ($1 = ''
			OR username ILIKE '%'||$1||'%' OR email::text ILIKE '%'||$1||'%' OR athlete_id ILIKE '%'||$1||'%')
		ORDER BY created_at DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []userRow{}
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.AthleteID, &u.Username, &u.Email, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *store) userByID(ctx context.Context, uid string) (userRow, error) {
	var u userRow
	err := s.pool.QueryRow(ctx, `
		SELECT id, athlete_id, username, email, role, status, created_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, uid).
		Scan(&u.ID, &u.AthleteID, &u.Username, &u.Email, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errNotFound
	}
	return u, err
}

func (s *store) setUserStatus(ctx context.Context, uid, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET status = $2::user_status, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, uid, status)
	if err == nil && tag.RowsAffected() == 0 {
		return errNotFound
	}
	if status != "active" {
		// suspenso/banido → derruba as sessões abertas
		_, _ = s.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, uid)
	}
	return err
}

func (s *store) setUserRole(ctx context.Context, uid, role string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET role = $2::user_role, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, uid, role)
	if err == nil && tag.RowsAffected() == 0 {
		return errNotFound
	}
	return err
}

// --- corridas sinalizadas ---

type flaggedRun struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	Username   string          `json:"username"`
	StartedAt  time.Time       `json:"started_at"`
	DistanceM  int             `json:"distance_m"`
	MovingS    int             `json:"moving_s"`
	AvgPaceS   int             `json:"avg_pace_s"`
	FraudScore float64         `json:"fraud_score"`
	FraudFlags json.RawMessage `json:"fraud_flags"`
	Status     string          `json:"status"`
}

func (s *store) flaggedRuns(ctx context.Context, limit int) ([]flaggedRun, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.user_id, u.username, r.started_at, r.distance_m, r.moving_s,
		       r.avg_pace_s, r.fraud_score, r.fraud_flags, r.status::text
		FROM runs r JOIN users u ON u.id = r.user_id
		WHERE r.status = 'flagged'
		ORDER BY r.fraud_score DESC, r.started_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []flaggedRun{}
	for rows.Next() {
		var f flaggedRun
		if err := rows.Scan(&f.ID, &f.UserID, &f.Username, &f.StartedAt, &f.DistanceM,
			&f.MovingS, &f.AvgPaceS, &f.FraudScore, &f.FraudFlags, &f.Status); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// reviewRun aplica a decisão da moderação: 'valid' libera (e enfileira o
// processamento), 'rejected' descarta. voidTerritory anula o território ligado.
func (s *store) reviewRun(ctx context.Context, runID, decision string, voidTerritory bool) (userID string, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	err = tx.QueryRow(ctx, `
		UPDATE runs SET status = $2::run_status, updated_at = now()
		WHERE id = $1 AND status = 'flagged'
		RETURNING user_id`, runID, decision).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	if err != nil {
		return "", err
	}
	if voidTerritory {
		if _, err = tx.Exec(ctx, `
			UPDATE territories SET status = 'revoked' WHERE run_id = $1 AND status = 'active'`, runID); err != nil {
			return "", err
		}
	}
	return userID, tx.Commit(ctx)
}

// --- zonas de risco ---

type riskZone struct {
	ID       string          `json:"id"`
	CityID   string          `json:"city_id"`
	Severity int             `json:"severity"`
	Source   string          `json:"source"`
	Note     *string         `json:"note"`
	Status   string          `json:"status"`
	Geom     json.RawMessage `json:"geom"`
}

func (s *store) riskZones(ctx context.Context, bbox [4]float64) (string, error) {
	const q = `
		SELECT jsonb_build_object('type','FeatureCollection','features',
			COALESCE(jsonb_agg(jsonb_build_object(
				'type','Feature',
				'geometry', ST_AsGeoJSON(geom)::jsonb,
				'properties', jsonb_build_object('id', id, 'severity', severity,
					'source', source::text, 'status', status, 'note', note,
					'active_from', active_from, 'active_to', active_to)
			)), '[]'::jsonb))::text
		FROM risk_zones
		WHERE ($1=0 AND $2=0 AND $3=0 AND $4=0
		       OR ST_Intersects(geom, ST_MakeEnvelope($1,$2,$3,$4,4326)))`
	var out string
	err := s.pool.QueryRow(ctx, q, bbox[0], bbox[1], bbox[2], bbox[3]).Scan(&out)
	return out, err
}

// upsertRiskZone cria (id vazio) ou atualiza uma zona a partir de um GeoJSON de geometria.
func (s *store) upsertRiskZone(ctx context.Context, z riskZoneInput) (string, error) {
	if z.ID == "" {
		z.ID = id.New()
		_, err := s.pool.Exec(ctx, `
			INSERT INTO risk_zones (id, city_id, geom, severity, source, note, active_from, active_to, status)
			VALUES ($1, COALESCE(nullif($2,''),'fortaleza'),
			        ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($3), 4326)),
			        $4, COALESCE(nullif($5,''),'admin')::risk_source, nullif($6,''), $7, $8,
			        COALESCE(nullif($9,''),'active'))`,
			z.ID, z.CityID, string(z.Geom), z.Severity, z.Source, z.Note, z.ActiveFrom, z.ActiveTo, z.Status)
		return z.ID, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE risk_zones SET
			severity = COALESCE($2, severity),
			note = COALESCE(nullif($3,''), note),
			status = COALESCE(nullif($4,''), status),
			active_from = COALESCE($5, active_from),
			active_to = COALESCE($6, active_to),
			geom = CASE WHEN $7 <> '' THEN ST_Multi(ST_SetSRID(ST_GeomFromGeoJSON($7), 4326)) ELSE geom END
		WHERE id = $1`,
		z.ID, nullInt(z.Severity), z.Note, z.Status, z.ActiveFrom, z.ActiveTo, string(z.Geom))
	if err == nil && tag.RowsAffected() == 0 {
		return "", errNotFound
	}
	return z.ID, err
}

func (s *store) deleteRiskZone(ctx context.Context, zid string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE risk_zones SET status = 'expired' WHERE id = $1`, zid)
	if err == nil && tag.RowsAffected() == 0 {
		return errNotFound
	}
	return err
}

// --- game_config ---

func (s *store) configAll(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value_jsonb FROM game_config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var k string
		var v json.RawMessage
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *store) setConfig(ctx context.Context, key string, value json.RawMessage, actorID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE game_config SET value_jsonb = $2, updated_by = $3, updated_at = now() WHERE key = $1`,
		key, value, actorID)
	if err == nil && tag.RowsAffected() == 0 {
		return errNotFound
	}
	return err
}

func nullInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
