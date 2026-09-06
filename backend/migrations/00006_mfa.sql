-- +goose Up
-- 2FA por TOTP (RFC 6238). O segredo fica cifrado em repouso (AES-256-GCM,
-- chave MFA_ENC_KEY) na coluna users.totp_secret_enc, que já existe desde 00001.
--
-- Fluxo:
--   setup     -> gera segredo, cifra e grava; totp_activated_at fica NULL (pendente)
--   activate  -> confere um código; grava totp_activated_at e os códigos de recuperação
--   login     -> se totp_activated_at != NULL, devolve um desafio em vez dos tokens
--   verify    -> confere TOTP ou código de recuperação e emite os tokens de verdade

ALTER TABLE users ADD COLUMN totp_activated_at timestamptz;

-- Cada sessão lembra se passou pelo 2FA; o access token carrega esse bit e as
-- rotas privilegiadas exigem que ele seja true. O refresh preserva o valor.
ALTER TABLE auth_sessions ADD COLUMN mfa_verified boolean NOT NULL DEFAULT false;

-- Códigos de recuperação de uso único (hash SHA-256, nunca o texto).
CREATE TABLE mfa_recovery_codes (
    user_id    text        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  text        NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code_hash)
);
CREATE INDEX mfa_recovery_codes_user_idx ON mfa_recovery_codes (user_id) WHERE used_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS mfa_recovery_codes;
ALTER TABLE auth_sessions DROP COLUMN IF EXISTS mfa_verified;
ALTER TABLE users DROP COLUMN IF EXISTS totp_activated_at;
