-- +goose Up
CREATE TABLE password_reset_tokens (
    token_hash  text PRIMARY KEY,          -- sha256 do token (o token cru só vai no e-mail)
    user_id     text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);
CREATE INDEX password_reset_user_idx ON password_reset_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS password_reset_tokens;
