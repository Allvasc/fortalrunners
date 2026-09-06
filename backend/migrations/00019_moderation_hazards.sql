-- +goose Up
-- Moderação (denúncias) e camada colaborativa de perigos na via (plano §5, §6, §13).

CREATE TABLE reports (
    id          text PRIMARY KEY,
    reporter_id text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    target_type text NOT NULL,            -- run | route | review | user | checkin | hazard | comment
    target_id   text NOT NULL,
    reason      text NOT NULL,
    detail      text,
    status      text NOT NULL DEFAULT 'open',   -- open | reviewing | actioned | dismissed
    handled_by  text REFERENCES users (id) ON DELETE SET NULL,
    handled_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reports_status_idx ON reports (status, created_at);
CREATE INDEX reports_target_idx ON reports (target_type, target_id);

-- Camada colaborativa da via: iluminação apagada, obra, alagamento, cachorro solto…
-- Linguagem neutra, sem nomear pessoas. Decai no tempo, sobe/desce por confirmação.
CREATE TABLE hazard_reports (
    id          text PRIMARY KEY,
    reporter_id text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type        text NOT NULL,            -- lighting | construction | flooding | blocked_sidewalk | loose_dog | traffic | attention
    geom        geometry(Point, 4326) NOT NULL,
    severity    smallint NOT NULL DEFAULT 2 CHECK (severity BETWEEN 1 AND 5),
    note        text,
    confirms    int NOT NULL DEFAULT 0,
    disputes    int NOT NULL DEFAULT 0,
    expires_at  timestamptz NOT NULL DEFAULT (now() + interval '14 days'),
    status      text NOT NULL DEFAULT 'active',   -- active | expired | removed
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX hazard_reports_geom_idx ON hazard_reports USING GIST (geom);
CREATE INDEX hazard_reports_active_idx ON hazard_reports (status) WHERE status = 'active';

CREATE TABLE hazard_votes (
    hazard_id  text NOT NULL REFERENCES hazard_reports (id) ON DELETE CASCADE,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vote       smallint NOT NULL,   -- +1 confirma, -1 disputa
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (hazard_id, user_id)
);

-- feed: adiciona visibilidade por evento (plano §14 — privacidade por corrida).
ALTER TABLE activity_events ADD COLUMN visibility text NOT NULL DEFAULT 'friends';
-- CHECK: public | friends | private

-- +goose Down
ALTER TABLE activity_events DROP COLUMN IF EXISTS visibility;
DROP TABLE IF EXISTS hazard_votes;
DROP TABLE IF EXISTS hazard_reports;
DROP TABLE IF EXISTS reports;
