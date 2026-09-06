-- +goose Up
CREATE TABLE email_verification_tokens (
    token_hash  text PRIMARY KEY,          -- sha256 do token (o token cru só vai no e-mail)
    user_id     text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email       text NOT NULL,             -- e-mail alvo no momento do pedido
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);
CREATE INDEX email_verification_user_idx ON email_verification_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS email_verification_tokens;
