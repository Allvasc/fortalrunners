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

func (s *Store) PerformCheckin(ctx context.Context, userID, landmarkID string, runID *string, photoKey *string, lat, lng float64) (*Checkin, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Verify landmark exists and position is within radius
	var radiusM int
	var badgeCode *string
	checkQuery := `
		SELECT radius_m, badge_code, ST_DWithin(geom::geography, ST_SetSRID(ST_Point($2, $3), 4326)::geography, radius_m) as within_radius
		FROM landmarks WHERE id = $1 AND status = 'active'
	`
	var withinRadius bool
	if err := tx.QueryRow(ctx, checkQuery, landmarkID, lng, lat).Scan(&radiusM, &badgeCode, &withinRadius); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLandmarkNotFound
		}
		return nil, fmt.Errorf("check landmark location: %w", err)
	}

	if !withinRadius {
		return nil, ErrTooFar
	}

	// Insert checkin
	checkinID := id.New()
	now := time.Now().UTC()
	insertQuery := `
		INSERT INTO landmark_checkins (id, user_id, landmark_id, run_id, photo_key, geom, taken_at, status)
		VALUES ($1, $2, $3, $4, $5, ST_SetSRID(ST_Point($6, $7), 4326), $8, 'approved')
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

	// Grant badge if associated
	if badgeCode != nil && *badgeCode != "" {
		badgeQuery := `
			INSERT INTO user_badges (user_id, badge_code, source, earned_at)
			VALUES ($1, $2, 'landmark', $3)
			ON CONFLICT (user_id, badge_code) DO NOTHING
		`
		if _, err := tx.Exec(ctx, badgeQuery, userID, *badgeCode, now); err != nil {
			return nil, fmt.Errorf("grant landmark badge: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit checkin: %w", err)
	}

	return &Checkin{
		ID:         retID,
		UserID:     userID,
		LandmarkID: landmarkID,
		RunID:      runID,
		PhotoKey:   photoKey,
		TakenAt:    retTime,
		Status:     "approved",
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

func (s *Store) AutoCheckinRun(ctx context.Context, userID, runID string, lineWKT string) ([]string, error) {
	query := `
		SELECT l.id, l.badge_code
		FROM landmarks l
		LEFT JOIN landmark_checkins lc ON lc.landmark_id = l.id AND lc.user_id = $1
		WHERE l.status = 'active' 
		  AND lc.id IS NULL
		  AND ST_DWithin(l.geom::geography, ST_GeomFromText($2, 4326)::geography, l.radius_m)
	`
	rows, err := s.pool.Query(ctx, query, userID, lineWKT)
	if err != nil {
		return nil, fmt.Errorf("query auto checkin landmarks: %w", err)
	}
	defer rows.Close()

	type toCheckin struct {
		id        string
		badgeCode *string
	}
	var targets []toCheckin
	for rows.Next() {
		var t toCheckin
		if err := rows.Scan(&t.id, &t.badgeCode); err == nil {
			targets = append(targets, t)
		}
	}

	var unlockedLandmarks []string
	now := time.Now().UTC()

	for _, target := range targets {
		chkID := id.New()
		insQuery := `
			INSERT INTO landmark_checkins (id, user_id, landmark_id, run_id, taken_at, status)
			VALUES ($1, $2, $3, $4, $5, 'approved')
			ON CONFLICT (user_id, landmark_id) DO NOTHING
		`
		tag, err := s.pool.Exec(ctx, insQuery, chkID, userID, target.id, runID, now)
		if err == nil && tag.RowsAffected() > 0 {
			unlockedLandmarks = append(unlockedLandmarks, target.id)
			if target.badgeCode != nil && *target.badgeCode != "" {
				_, _ = s.pool.Exec(ctx, `
					INSERT INTO user_badges (user_id, badge_code, source, earned_at)
					VALUES ($1, $2, 'landmark', $3)
					ON CONFLICT (user_id, badge_code) DO NOTHING
				`, userID, *target.badgeCode, now)
			}
		}
	}

	return unlockedLandmarks, nil
}
