package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("não encontrado")

// User é a projeção mínima usada pelo módulo auth.
type User struct {
	ID           string
	AthleteID    string
	Username     string
	Email        string
	PasswordHash *string
	DisplayName  *string
	Role         string
	Status       string
}

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

// createUser insere o usuário já com o número público (FR-000000N).
func (s *store) createUser(ctx context.Context, u User) (User, error) {
	const q = `
		INSERT INTO users (id, athlete_id, username, email, password_hash, display_name, role, status)
		VALUES ($1, 'FR-' || lpad(nextval('athlete_id_seq')::text, 7, '0'), $2, $3, $4, $5, 'runner', 'active')
		RETURNING id, athlete_id, username, email, password_hash, display_name, role, status`
	row := s.pool.QueryRow(ctx, q, u.ID, u.Username, u.Email, u.PasswordHash, u.DisplayName)
	return scanUser(row)
}

func (s *store) userByEmail(ctx context.Context, email string) (User, error) {
	const q = `
		SELECT id, athlete_id, username, email, password_hash, display_name, role, status
		FROM users WHERE email = $1 AND deleted_at IS NULL`
	return scanUser(s.pool.QueryRow(ctx, q, email))
}

func (s *store) userByID(ctx context.Context, id string) (User, error) {
	const q = `
		SELECT id, athlete_id, username, email, password_hash, display_name, role, status
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	return scanUser(s.pool.QueryRow(ctx, q, id))
}

// --- sessões ---

type session struct {
	ID               string
	UserID           string
	FamilyID         string
	RefreshTokenHash string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}

func (s *store) createSession(ctx context.Context, sess session, ip, ua string) error {
	const q = `
		INSERT INTO auth_sessions (id, user_id, refresh_token_hash, family_id, ip, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, nullif($5,'')::inet, nullif($6,''), $7)`
	_, err := s.pool.Exec(ctx, q, sess.ID, sess.UserID, sess.RefreshTokenHash, sess.FamilyID, ip, ua, sess.ExpiresAt)
	return err
}

func (s *store) sessionByRefreshHash(ctx context.Context, hash string) (session, error) {
	const q = `
		SELECT id, user_id, family_id, refresh_token_hash, expires_at, revoked_at
		FROM auth_sessions WHERE refresh_token_hash = $1`
	var sess session
	err := s.pool.QueryRow(ctx, q, hash).Scan(
		&sess.ID, &sess.UserID, &sess.FamilyID, &sess.RefreshTokenHash, &sess.ExpiresAt, &sess.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return sess, errNotFound
	}
	return sess, err
}

func (s *store) revokeSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

// revokeFamily invalida toda a família — usado quando um refresh antigo é reapresentado.
func (s *store) revokeFamily(ctx context.Context, familyID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`, familyID)
	return err
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.AthleteID, &u.Username, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errNotFound
	}
	if err != nil {
		return u, fmt.Errorf("auth: scan user: %w", err)
	}
	return u, nil
}
