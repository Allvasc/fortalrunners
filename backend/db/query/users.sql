-- Consultas do módulo auth. Geradas por sqlc (make sqlc).
-- Fase 0 usa acesso direto via pgx (internal/auth/store.go); estas queries
-- servem de base para a migração para sqlc.

-- name: GetUserByID :one
SELECT id, athlete_id, username, email, password_hash, display_name, role, status
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT id, athlete_id, username, email, password_hash, display_name, role, status
FROM users
WHERE email = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (id, athlete_id, username, email, password_hash, display_name)
VALUES (
    $1,
    'FR-' || lpad(nextval('athlete_id_seq')::text, 7, '0'),
    $2, $3, $4, $5
)
RETURNING id, athlete_id, username, email, password_hash, display_name, role, status;

-- name: CreateSession :exec
INSERT INTO auth_sessions (id, user_id, refresh_token_hash, family_id, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetSessionByRefreshHash :one
SELECT id, user_id, family_id, refresh_token_hash, expires_at, revoked_at
FROM auth_sessions
WHERE refresh_token_hash = $1;

-- name: RevokeSession :exec
UPDATE auth_sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeSessionFamily :exec
UPDATE auth_sessions SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL;
