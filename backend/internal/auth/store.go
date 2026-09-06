package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
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

// --- identidades sociais (OAuth) ---

func (s *store) userIDByIdentity(ctx context.Context, provider, providerUID string) (string, error) {
	var uid string
	err := s.pool.QueryRow(ctx, `
		SELECT i.user_id FROM identities i JOIN users u ON u.id = i.user_id
		WHERE i.provider = $1::id_provider AND i.provider_uid = $2 AND u.deleted_at IS NULL`,
		provider, providerUID).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return uid, err
}

func (s *store) linkIdentity(ctx context.Context, idv, userID, provider, providerUID, email string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO identities (id, user_id, provider, provider_uid, email)
		VALUES ($1, $2, $3::id_provider, $4, nullif($5,''))
		ON CONFLICT (provider, provider_uid) DO NOTHING`,
		idv, userID, provider, providerUID, email)
	return err
}

func (s *store) usernameFree(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
	return !exists, err
}

// createOAuthUser cria a conta (sem senha) e a identidade numa transação.
func (s *store) createOAuthUser(ctx context.Context, u User, provider, providerUID string, emailVerified bool) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const q = `
		INSERT INTO users (id, athlete_id, username, email, display_name, email_verified_at, role, status)
		VALUES ($1, 'FR-' || lpad(nextval('athlete_id_seq')::text, 7, '0'), $2, $3, $4,
		        CASE WHEN $5 THEN now() ELSE NULL END, 'runner', 'active')
		RETURNING id, athlete_id, username, email, password_hash, display_name, role, status`
	created, err := scanUser(tx.QueryRow(ctx, q, u.ID, u.Username, u.Email, u.DisplayName, emailVerified))
	if err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO identities (id, user_id, provider, provider_uid, email)
		VALUES ($1, $2, $3::id_provider, $4, nullif($5,''))`,
		id.New(), created.ID, provider, providerUID, u.Email); err != nil {
		return User{}, err
	}
	return created, tx.Commit(ctx)
}

// --- sessões ---

type session struct {
	ID               string
	UserID           string
	FamilyID         string
	RefreshTokenHash string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	MFAVerified      bool
}

func (s *store) createSession(ctx context.Context, sess session, ip, ua string) error {
	const q = `
		INSERT INTO auth_sessions (id, user_id, refresh_token_hash, family_id, ip, user_agent, expires_at, mfa_verified)
		VALUES ($1, $2, $3, $4, nullif($5,'')::inet, nullif($6,''), $7, $8)`
	_, err := s.pool.Exec(ctx, q, sess.ID, sess.UserID, sess.RefreshTokenHash, sess.FamilyID, ip, ua, sess.ExpiresAt, sess.MFAVerified)
	return err
}

func (s *store) sessionByRefreshHash(ctx context.Context, hash string) (session, error) {
	const q = `
		SELECT id, user_id, family_id, refresh_token_hash, expires_at, revoked_at, mfa_verified
		FROM auth_sessions WHERE refresh_token_hash = $1`
	var sess session
	err := s.pool.QueryRow(ctx, q, hash).Scan(
		&sess.ID, &sess.UserID, &sess.FamilyID, &sess.RefreshTokenHash, &sess.ExpiresAt, &sess.RevokedAt, &sess.MFAVerified)
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

func (s *store) revokeAllSessions(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// --- redefinição de senha ---

func (s *store) createResetToken(ctx context.Context, tokenHash, userID string, exp time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, exp)
	return err
}

// consumeResetToken devolve o user_id e marca o token como usado, se válido.
func (s *store) consumeResetToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := s.pool.QueryRow(ctx, `
		UPDATE password_reset_tokens SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash).Scan(&userID)
	return userID, err
}

func (s *store) setPassword(ctx context.Context, userID, hash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, hash)
	return err
}

// --- 2FA / TOTP ---

// mfaState devolve o segredo cifrado (nil se não houver) e quando o TOTP foi
// ativado (nil = pendente ou desligado).
func (s *store) mfaState(ctx context.Context, userID string) (enc []byte, activatedAt *time.Time, err error) {
	const q = `SELECT totp_secret_enc, totp_activated_at FROM users WHERE id = $1 AND deleted_at IS NULL`
	err = s.pool.QueryRow(ctx, q, userID).Scan(&enc, &activatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, errNotFound
	}
	return enc, activatedAt, err
}

// setPendingTOTP grava um segredo novo e zera a ativação (re-enrollment inclusive).
func (s *store) setPendingTOTP(ctx context.Context, userID string, enc []byte) error {
	const q = `UPDATE users SET totp_secret_enc = $2, totp_activated_at = NULL, updated_at = now()
	           WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.pool.Exec(ctx, q, userID, enc)
	return err
}

// activateTOTP marca o TOTP como ativo e regrava os códigos de recuperação
// numa transação (troca atômica).
func (s *store) activateTOTP(ctx context.Context, userID string, recoveryHashes []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`UPDATE users SET totp_activated_at = now(), updated_at = now()
		 WHERE id = $1 AND totp_secret_enc IS NOT NULL`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM mfa_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, h := range recoveryHashes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO mfa_recovery_codes (user_id, code_hash) VALUES ($1, $2)`, userID, h); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// disableTOTP apaga o segredo, a ativação e todos os códigos de recuperação.
func (s *store) disableTOTP(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`UPDATE users SET totp_secret_enc = NULL, totp_activated_at = NULL, updated_at = now()
		 WHERE id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM mfa_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// consumeRecoveryCode marca um código como usado. Devolve true se consumiu.
func (s *store) consumeRecoveryCode(ctx context.Context, userID, codeHash string) (bool, error) {
	const q = `UPDATE mfa_recovery_codes SET used_at = now()
	           WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, userID, codeHash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// countUnusedRecoveryCodes serve para avisar o usuário quando estão acabando.
func (s *store) countUnusedRecoveryCodes(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM mfa_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, userID).Scan(&n)
	return n, err
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
