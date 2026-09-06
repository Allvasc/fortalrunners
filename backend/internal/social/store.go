package social

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var ErrCannotFriendSelf = errors.New("cannot friend self")

type Friend struct {
	ID        string    `json:"id"`
	AthleteID string    `json:"athlete_id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Since     time.Time `json:"since"`
}

type FeedEvent struct {
	ID         string         `json:"id"`
	ActorID    string         `json:"actor_id"`
	ActorName  string         `json:"actor_name"`
	Type       string         `json:"type"` // run, claim, badge, checkin
	SubjectID  string         `json:"subject_id"`
	Payload    map[string]any `json:"payload"`
	KudosCount int            `json:"kudos_count"`
	HasKudos   bool           `json:"has_kudos"`
	CreatedAt  time.Time      `json:"created_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) SendRequest(ctx context.Context, userID, targetID string) error {
	if userID == targetID {
		return ErrCannotFriendSelf
	}

	u1, u2 := userID, targetID
	if u1 > u2 {
		u1, u2 = targetID, userID
	}

	query := `
		INSERT INTO friendships (user_id, friend_id, requested_by, status, created_at)
		VALUES ($1, $2, $3, 'pending', now())
		ON CONFLICT (user_id, friend_id) DO NOTHING
	`
	_, err := s.pool.Exec(ctx, query, u1, u2, userID)
	return err
}

func (s *Store) AcceptRequest(ctx context.Context, userID, targetID string) error {
	u1, u2 := userID, targetID
	if u1 > u2 {
		u1, u2 = targetID, userID
	}

	query := `
		UPDATE friendships
		SET status = 'accepted', accepted_at = now()
		WHERE user_id = $1 AND friend_id = $2 AND status = 'pending'
	`
	_, err := s.pool.Exec(ctx, query, u1, u2)
	return err
}

func (s *Store) ListFriends(ctx context.Context, userID string) ([]Friend, error) {
	query := `
		SELECT 
			u.id, u.athlete_id, u.username, COALESCE(u.display_name, u.username),
			f.status, COALESCE(f.accepted_at, f.created_at)
		FROM friendships f
		JOIN users u ON (u.id = CASE WHEN f.user_id = $1 THEN f.friend_id ELSE f.user_id END)
		WHERE (f.user_id = $1 OR f.friend_id = $1)
		ORDER BY f.created_at DESC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list friends query: %w", err)
	}
	defer rows.Close()

	var friends []Friend
	for rows.Next() {
		var fr Friend
		if err := rows.Scan(&fr.ID, &fr.AthleteID, &fr.Username, &fr.Name, &fr.Status, &fr.Since); err != nil {
			return nil, fmt.Errorf("scan friend: %w", err)
		}
		friends = append(friends, fr)
	}
	return friends, nil
}

func (s *Store) GetFeed(ctx context.Context, userID string) ([]FeedEvent, error) {
	query := `
		SELECT 
			e.id, e.actor_id, COALESCE(u.display_name, u.username) as actor_name,
			e.type, e.subject_id, e.payload_jsonb,
			(SELECT COUNT(*) FROM kudos k WHERE k.run_id = e.subject_id) as kudos_count,
			EXISTS(SELECT 1 FROM kudos k WHERE k.run_id = e.subject_id AND k.user_id = $1) as has_kudos,
			e.created_at
		FROM activity_events e
		JOIN users u ON u.id = e.actor_id
		WHERE e.actor_id = $1
		   OR e.actor_id IN (
				SELECT CASE WHEN f.user_id = $1 THEN f.friend_id ELSE f.user_id END
				FROM friendships f
				WHERE (f.user_id = $1 OR f.friend_id = $1) AND f.status = 'accepted'
		   )
		ORDER BY e.created_at DESC
		LIMIT 50
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("feed query: %w", err)
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var ev FeedEvent
		var payloadJSON map[string]any
		if err := rows.Scan(&ev.ID, &ev.ActorID, &ev.ActorName, &ev.Type, &ev.SubjectID, &payloadJSON, &ev.KudosCount, &ev.HasKudos, &ev.CreatedAt); err == nil {
			ev.Payload = payloadJSON
			events = append(events, ev)
		}
	}
	return events, nil
}

func (s *Store) ToggleKudos(ctx context.Context, userID, runID string) (bool, error) {
	// Check if already kudos
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM kudos WHERE run_id = $1 AND user_id = $2)`
	_ = s.pool.QueryRow(ctx, checkQuery, runID, userID).Scan(&exists)

	if exists {
		_, err := s.pool.Exec(ctx, `DELETE FROM kudos WHERE run_id = $1 AND user_id = $2`, runID, userID)
		return false, err
	}

	_, err := s.pool.Exec(ctx, `INSERT INTO kudos (run_id, user_id, created_at) VALUES ($1, $2, now()) ON CONFLICT DO NOTHING`, runID, userID)
	return true, err
}

func (s *Store) RecordActivity(ctx context.Context, actorID string, actType string, subjectID string, payload map[string]any) error {
	eventID := id.New()
	query := `
		INSERT INTO activity_events (id, actor_id, type, subject_id, payload_jsonb, created_at)
		VALUES ($1, $2, $3::activity_type, $4, $5, now())
	`
	_, err := s.pool.Exec(ctx, query, eventID, actorID, actType, subjectID, payload)
	return err
}
