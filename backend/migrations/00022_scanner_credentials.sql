-- +goose Up
-- Credenciais de scanner por estação (plano §3, §12): a equipe do evento recebe
-- um token assinado por posto/função para gravar as leituras.

CREATE TABLE scanner_credentials (
    id           text PRIMARY KEY,
    event_id     text NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    station_id   text REFERENCES event_stations (id) ON DELETE CASCADE,
    label        text NOT NULL,
    token_hash   text NOT NULL,             -- sha256 do token entregue à equipe
    role         text NOT NULL DEFAULT 'checkpoint',
    expires_at   timestamptz,
    revoked      boolean NOT NULL DEFAULT false,
    created_by   text REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX scanner_credentials_event_idx ON scanner_credentials (event_id);
CREATE UNIQUE INDEX scanner_credentials_token_idx ON scanner_credentials (token_hash);

-- +goose Down
DROP TABLE IF EXISTS scanner_credentials;
