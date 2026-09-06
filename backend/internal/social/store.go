package social

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

var (
	ErrCannotFriendSelf = errors.New("cannot friend self")
	ErrTargetNotFound   = errors.New("usuário não encontrado")
	ErrNotRecipient     = errors.New("apenas quem recebeu o pedido pode aceitá-lo")
	ErrRequestNotFound  = errors.New("pedido de amizade não encontrado")
)

type Friend struct {
	ID        string    `json:"id"`
	AthleteID string    `json:"athlete_id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Incoming  bool      `json:"incoming"` // pedido recebido aguardando minha resposta
	Since     time.Time `json:"since"`
}

type FeedEvent struct {
	ID         string         `json:"id"`
	ActorID    string         `json:"actor_id"`
	ActorName  string         `json:"actor_name"`
	Type       string         `json:"type"`
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

func ordered(a, b string) (string, string) {
	if a > b {
		return b, a
	}
	return a, b
}

func (s *Store) SendRequest(ctx context.Context, userID, rawTarget string) error {
	// aceita tanto o id (ULID) quanto o athlete_id (FR-0001234, case-insensitive).
	var targetID string
	if err := s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE (id = $1 OR upper(athlete_id) = upper($1)) AND status = 'active'`,
		strings.TrimSpace(rawTarget),
	).Scan(&targetID); errors.Is(err, pgx.ErrNoRows) {
		return ErrTargetNotFound
	} else if err != nil {
		return err
	}
	if userID == targetID {
		return ErrCannotFriendSelf
	}

	u1, u2 := ordered(userID, targetID)
	// se já existe pedido do outro lado, aceitar direto.
	tag, err := s.pool.Exec(ctx, `
		UPDATE friendships SET status = 'accepted', accepted_at = now()
		WHERE user_id = $1 AND friend_id = $2 AND status = 'pending' AND requested_by = $3`,
		u1, u2, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO friendships (user_id, friend_id, requested_by, status, created_at)
		VALUES ($1, $2, $3, 'pending', now())
		ON CONFLICT (user_id, friend_id) DO NOTHING`, u1, u2, userID)
	return err
}

func (s *Store) AcceptRequest(ctx context.Context, userID, targetID string) error {
	u1, u2 := ordered(userID, targetID)
	tag, err := s.pool.Exec(ctx, `
		UPDATE friendships SET status = 'accepted', accepted_at = now()
		WHERE user_id = $1 AND friend_id = $2 AND status = 'pending' AND requested_by <> $3`,
		u1, u2, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRequestNotFound
	}
	return nil
}

// Remove desfaz a amizade OU recusa um pedido pendente.
func (s *Store) Remove(ctx context.Context, userID, targetID string) error {
	u1, u2 := ordered(userID, targetID)
	_, err := s.pool.Exec(ctx,
		`DELETE FROM friendships WHERE user_id = $1 AND friend_id = $2`, u1, u2)
	return err
}

func (s *Store) Block(ctx context.Context, userID, targetID string) error {
	if userID == targetID {
		return ErrCannotFriendSelf
	}
	u1, u2 := ordered(userID, targetID)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO friendships (user_id, friend_id, requested_by, status, created_at)
		VALUES ($1, $2, $3, 'blocked', now())
		ON CONFLICT (user_id, friend_id) DO UPDATE SET status = 'blocked', requested_by = $3`,
		u1, u2, userID)
	return err
}

func (s *Store) ListFriends(ctx context.Context, userID string, limit int) ([]Friend, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.athlete_id, u.username, COALESCE(u.display_name, u.username),
		       f.status, (f.status = 'pending' AND f.requested_by <> $1) AS incoming,
		       COALESCE(f.accepted_at, f.created_at)
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.user_id = $1 THEN f.friend_id ELSE f.user_id END
		WHERE (f.user_id = $1 OR f.friend_id = $1) AND f.status <> 'blocked'
		ORDER BY incoming DESC, f.created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list friends query: %w", err)
	}
	defer rows.Close()

	var friends []Friend
	for rows.Next() {
		var fr Friend
		if err := rows.Scan(&fr.ID, &fr.AthleteID, &fr.Username, &fr.Name, &fr.Status, &fr.Incoming, &fr.Since); err != nil {
			return nil, fmt.Errorf("scan friend: %w", err)
		}
		friends = append(friends, fr)
	}
	return friends, nil
}

// GetFeed pagina por cursor (created_at ISO). Respeita a visibilidade do evento:
// eventos 'private' nunca aparecem para terceiros; 'friends' só para amigos.
func (s *Store) GetFeed(ctx context.Context, userID, cursor string, limit int) ([]FeedEvent, string, error) {
	var before time.Time
	if cursor != "" {
		before, _ = time.Parse(time.RFC3339Nano, cursor)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.actor_id, COALESCE(u.display_name, u.username),
		       e.type, e.subject_id, e.payload_jsonb,
		       (SELECT COUNT(*) FROM kudos k WHERE k.run_id = e.subject_id),
		       EXISTS(SELECT 1 FROM kudos k WHERE k.run_id = e.subject_id AND k.user_id = $1),
		       e.created_at
		FROM activity_events e
		JOIN users u ON u.id = e.actor_id
		WHERE ($2::timestamptz IS NULL OR e.created_at < $2)
		  AND (u.status <> 'shadow_banned' OR e.actor_id = $1)
		  AND (
		    e.actor_id = $1
		    OR (e.visibility = 'public')
		    OR (e.visibility = 'friends' AND e.actor_id IN (
		          SELECT CASE WHEN f.user_id = $1 THEN f.friend_id ELSE f.user_id END
		          FROM friendships f
		          WHERE (f.user_id = $1 OR f.friend_id = $1) AND f.status = 'accepted'))
		  )
		ORDER BY e.created_at DESC
		LIMIT $3`, userID, nullTime(before), limit)
	if err != nil {
		return nil, "", fmt.Errorf("feed query: %w", err)
	}
	defer rows.Close()

	var events []FeedEvent
	for rows.Next() {
		var ev FeedEvent
		var payload map[string]any
		if err := rows.Scan(&ev.ID, &ev.ActorID, &ev.ActorName, &ev.Type, &ev.SubjectID,
			&payload, &ev.KudosCount, &ev.HasKudos, &ev.CreatedAt); err == nil {
			ev.Payload = payload
			events = append(events, ev)
		}
	}
	next := ""
	if len(events) == limit {
		next = events[len(events)-1].CreatedAt.Format(time.RFC3339Nano)
	}
	return events, next, nil
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *Store) ToggleKudos(ctx context.Context, userID, runID string) (bool, error) {
	var exists bool
	_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kudos WHERE run_id = $1 AND user_id = $2)`, runID, userID).Scan(&exists)
	if exists {
		_, err := s.pool.Exec(ctx, `DELETE FROM kudos WHERE run_id = $1 AND user_id = $2`, runID, userID)
		return false, err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO kudos (run_id, user_id, created_at) VALUES ($1, $2, now()) ON CONFLICT DO NOTHING`, runID, userID)
	return true, err
}

// RecordActivity publica no feed (chamado pelo domínio, não pelo cliente).
func (s *Store) RecordActivity(ctx context.Context, actorID, actType, subjectID, visibility string, payload map[string]any) error {
	if visibility == "" {
		visibility = "friends"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO activity_events (id, actor_id, type, subject_id, payload_jsonb, visibility, created_at)
		VALUES ($1, $2, $3::activity_type, $4, $5, $6, now())`,
		id.New(), actorID, actType, subjectID, payload, visibility)
	return err
}
