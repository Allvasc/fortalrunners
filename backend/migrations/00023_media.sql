-- +goose Up
-- Armazenamento de mídia (fotos de check-in de marco, avatares) — plano §14.
-- Em produção real isto vira object storage (S3/R2) via URL assinada; por ora
-- os bytes vivem no Postgres para sobreviver ao restart do container (Render).

CREATE TABLE media (
    key          text PRIMARY KEY,          -- <prefixo>/<ulid>.<ext>
    owner_id     text REFERENCES users (id) ON DELETE SET NULL,
    content_type text NOT NULL,
    bytes        bytea NOT NULL,
    size_bytes   int NOT NULL,
    purpose      text NOT NULL DEFAULT 'landmark_checkin',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX media_owner_idx ON media (owner_id);

-- +goose Down
DROP TABLE IF EXISTS media;
