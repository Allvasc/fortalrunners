package landmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var ErrLandmarkNotFound = errors.New("landmark not found")
var ErrAlreadyCheckedIn = errors.New("already checked in")
var ErrTooFar = errors.New("position too far from landmark")
var ErrRiskZone = errors.New("landmark in an active risk zone")
var ErrPhotoRequired = errors.New("photo required for landmark check-in")

type Landmark struct {
	ID           string     `json:"id"`
	CollectionID *string    `json:"collection_id,omitempty"`
	Name         string     `json:"name"`
	BadgeCode    *string    `json:"badge_code,omitempty"`
	RadiusM      int        `json:"radius_m"`
	Blurb        string     `json:"blurb,omitempty"`
	HeroPhotoKey *string    `json:"hero_photo_key,omitempty"`
	Difficulty   int        `json:"difficulty"`
	Lat          float64    `json:"lat"`
	Lng          float64    `json:"lng"`
	CheckedIn    bool       `json:"checked_in"`
	CheckedInAt  *time.Time `json:"checked_in_at,omitempty"`
}

type Collection struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	BadgeCode   *string `json:"badge_code,omitempty"`
	RewardXP    int     `json:"reward_xp"`
	Total       int     `json:"total"`
	Unlocked    int     `json:"unlocked"`
}

type Checkin struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	LandmarkID string    `json:"landmark_id"`
	RunID      *string   `json:"run_id,omitempty"`
	PhotoKey   *string   `json:"photo_key,omitempty"`
	TakenAt    time.Time `json:"taken_at"`
	Status     string    `json:"status"`
}

type Progress struct {
	TotalLandmarks int          `json:"total_landmarks"`
	UnlockedCount  int          `json:"unlocked_count"`
	Landmarks      []Landmark   `json:"landmarks"`
	Collections    []Collection `json:"collections"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListLandmarks(ctx context.Context, userID string) ([]Landmark, error) {
	query := `
		SELECT 
			l.id, l.collection_id, l.name, l.badge_code, l.radius_m, COALESCE(l.blurb, ''), 
			l.hero_photo_key, l.difficulty, ST_Y(l.geom) as lat, ST_X(l.geom) as lng,
			(lc.id IS NOT NULL) as checked_in, lc.taken_at
		FROM landmarks l
		LEFT JOIN landmark_checkins lc ON lc.landmark_id = l.id AND lc.user_id = $1 AND lc.status = 'approved'
		WHERE l.status = 'active'
		ORDER BY l.name ASC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list landmarks query: %w", err)
	}
	defer rows.Close()

	var landmarks []Landmark
	for rows.Next() {
		var lm Landmark
		var colID, badgeCode, heroPhoto *string
		var takenAt *time.Time

		if err := rows.Scan(
			&lm.ID, &colID, &lm.Name, &badgeCode, &lm.RadiusM, &lm.Blurb,
			&heroPhoto, &lm.Difficulty, &lm.Lat, &lm.Lng,
			&lm.CheckedIn, &takenAt,
		); err != nil {
			return nil, fmt.Errorf("scan landmark: %w", err)
		}

		lm.CollectionID = colID
		lm.BadgeCode = badgeCode
		lm.HeroPhotoKey = heroPhoto
		lm.CheckedInAt = takenAt
		landmarks = append(landmarks, lm)
	}
	if landmarks == nil {
		landmarks = []Landmark{}
	}
	return landmarks, nil
}

func (s *Store) GetLandmark(ctx context.Context, landmarkID, userID string) (*Landmark, error) {
	query := `
		SELECT 
			l.id, l.collection_id, l.name, l.badge_code, l.radius_m, COALESCE(l.blurb, ''), 
			l.hero_photo_key, l.difficulty, ST_Y(l.geom) as lat, ST_X(l.geom) as lng,
			(lc.id IS NOT NULL) as checked_in, lc.taken_at
		FROM landmarks l
		LEFT JOIN landmark_checkins lc ON lc.landmark_id = l.id AND lc.user_id = $2 AND lc.status = 'approved'
		WHERE l.id = $1 AND l.status = 'active'
	`
	row := s.pool.QueryRow(ctx, query, landmarkID, userID)
	var lm Landmark
	var colID, badgeCode, heroPhoto *string
	var takenAt *time.Time

	if err := row.Scan(
		&lm.ID, &colID, &lm.Name, &badgeCode, &lm.RadiusM, &lm.Blurb,
		&heroPhoto, &lm.Difficulty, &lm.Lat, &lm.Lng,
		&lm.CheckedIn, &takenAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandmarkNotFound
		}
		return nil, fmt.Errorf("get landmark scan: %w", err)
	}

	lm.CollectionID = colID
	lm.BadgeCode = badgeCode
	lm.HeroPhotoKey = heroPhoto
	lm.CheckedInAt = takenAt
	return &lm, nil
}

// PerformCheckin registra um check-in por FOTO. Ele NÃO concede o selo na hora:
// entra como 'pending' e a fila de moderação (admin) aprova/rejeita — plano §3.
// Gate de zona de risco: marco dentro de risk_zone ativa acima do limiar não
// aceita check-in (missionAllowedAt).
func (s *Store) PerformCheckin(ctx context.Context, userID, landmarkID string, runID *string, photoKey *string, lat, lng float64) (*Checkin, error) {
	if photoKey == nil || *photoKey == "" {
		return nil, ErrPhotoRequired
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// posição vs. raio + gate de zona de risco, numa query.
	var withinRadius, inRiskZone bool
	checkQuery := `
		SELECT
			ST_DWithin(l.geom::geography, ST_SetSRID(ST_Point($2, $3), 4326)::geography, l.radius_m),
			EXISTS (
				SELECT 1 FROM risk_zones rz
				WHERE rz.status = 'active'
				  AND rz.severity >= COALESCE((SELECT value_jsonb::text::int FROM game_config WHERE key = 'risk_zone_min_severity'), 3)
				  AND (rz.active_to IS NULL OR rz.active_to > now())
				  AND ST_Intersects(rz.geom, l.geom)
			)
		FROM landmarks l WHERE l.id = $1 AND l.status = 'active'
	`
	if err := tx.QueryRow(ctx, checkQuery, landmarkID, lng, lat).Scan(&withinRadius, &inRiskZone); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandmarkNotFound
		}
		return nil, fmt.Errorf("check landmark location: %w", err)
	}
	if inRiskZone {
		return nil, ErrRiskZone
	}
	if !withinRadius {
		return nil, ErrTooFar
	}

	checkinID := id.New()
	now := time.Now().UTC()
	insertQuery := `
		INSERT INTO landmark_checkins (id, user_id, landmark_id, run_id, photo_key, geom, taken_at, status)
		VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_Point($6, $7), 4326), $8, 'pending')
		ON CONFLICT (user_id, landmark_id) DO NOTHING
		RETURNING id, taken_at
	`
	var retID string
	var retTime time.Time
	err = tx.QueryRow(ctx, insertQuery, checkinID, userID, landmarkID, runID, photoKey, lng, lat, now).Scan(&retID, &retTime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAlreadyCheckedIn
		}
		return nil, fmt.Errorf("insert checkin: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit checkin: %w", err)
	}

	return &Checkin{
		ID: retID, UserID: userID, LandmarkID: landmarkID,
		RunID: runID, PhotoKey: photoKey, TakenAt: retTime, Status: "pending",
	}, nil
}

func (s *Store) ListCollections(ctx context.Context, userID string) ([]Collection, error) {
	query := `
		SELECT 
			c.id, c.name, COALESCE(c.description, ''), c.badge_code, c.reward_xp,
			COUNT(l.id) as total,
			COUNT(lc.id) FILTER (WHERE lc.status = 'approved') as unlocked
		FROM landmark_collections c
		JOIN landmarks l ON l.collection_id = c.id AND l.status = 'active'
		LEFT JOIN landmark_checkins lc ON lc.landmark_id = l.id AND lc.user_id = $1
		GROUP BY c.id, c.name, c.description, c.badge_code, c.reward_xp
		ORDER BY c.name ASC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var col Collection
		var badgeCode *string
		if err := rows.Scan(&col.ID, &col.Name, &col.Description, &badgeCode, &col.RewardXP, &col.Total, &col.Unlocked); err != nil {
			return nil, fmt.Errorf("scan collection: %w", err)
		}
		col.BadgeCode = badgeCode
		collections = append(collections, col)
	}
	return collections, nil
}

// PendingCheckin é uma linha da fila de moderação (plano §13).
type PendingCheckin struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	LandmarkID   string    `json:"landmark_id"`
	LandmarkName string    `json:"landmark_name"`
	PhotoKey     *string   `json:"photo_key,omitempty"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	TakenAt      time.Time `json:"taken_at"`
}

// PendingCheckins lista os check-ins por foto aguardando moderação.
func (s *Store) PendingCheckins(ctx context.Context, limit int) ([]PendingCheckin, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT lc.id, lc.user_id, u.username, lc.landmark_id, l.name, lc.photo_key,
		       ST_Y(lc.geom), ST_X(lc.geom), lc.taken_at
		FROM landmark_checkins lc
		JOIN users u ON u.id = lc.user_id
		JOIN landmarks l ON l.id = lc.landmark_id
		WHERE lc.status = 'pending'
		ORDER BY lc.taken_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingCheckin
	for rows.Next() {
		var p PendingCheckin
		var lat, lng *float64
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.LandmarkID, &p.LandmarkName,
			&p.PhotoKey, &lat, &lng, &p.TakenAt); err != nil {
			return nil, err
		}
		if lat != nil {
			p.Lat = *lat
		}
		if lng != nil {
			p.Lng = *lng
		}
		out = append(out, p)
	}
	return out, nil
}

// ModerateCheckin aprova ou rejeita um check-in pendente. Aprovar concede o selo.
func (s *Store) ModerateCheckin(ctx context.Context, checkinID, decision, moderatorID string) error {
	newStatus := "rejected"
	if decision == "approve" {
		newStatus = "approved"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID, landmarkID string
	err = tx.QueryRow(ctx, `
		UPDATE landmark_checkins SET status = $2, moderated_by = $3
		WHERE id = $1 AND status = 'pending'
		RETURNING user_id, landmark_id`, checkinID, newStatus, moderatorID).Scan(&userID, &landmarkID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLandmarkNotFound
	}
	if err != nil {
		return err
	}

	if newStatus == "approved" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_badges (user_id, badge_code, source, earned_at)
			SELECT $1, l.badge_code, 'landmark', now()
			FROM landmarks l WHERE l.id = $2 AND l.badge_code IS NOT NULL AND l.badge_code != ''
			ON CONFLICT (user_id, badge_code) DO NOTHING`, userID, landmarkID); err != nil {
			return fmt.Errorf("grant badge: %w", err)
		}
	}
	return tx.Commit(ctx)
}
