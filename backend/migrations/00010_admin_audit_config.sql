-- +goose Up
-- Fundação do painel de admin (plano §13): trilha de auditoria append-only e
-- config de jogo ajustável em runtime (feature flags + parâmetros).

CREATE TABLE audit_log (
    id          text PRIMARY KEY,
    actor_id    text REFERENCES users (id) ON DELETE SET NULL,
    actor_role  text NOT NULL,
    action      text NOT NULL,              -- ex.: user.suspend, run.reject, risk_zone.create
    target_type text NOT NULL,              -- user | run | territory | risk_zone | config
    target_id   text,
    diff_jsonb  jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip          inet,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_actor_idx  ON audit_log (actor_id, created_at DESC);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id);

-- game_config — feature flags e parâmetros do jogo, mexíveis no admin sem deploy
-- (plano §13 "Config do jogo", §18 "feature flags"). Valor é JSON puro.
CREATE TABLE game_config (
    key         text PRIMARY KEY,
    value_jsonb jsonb NOT NULL,
    updated_by  text REFERENCES users (id) ON DELETE SET NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
INSERT INTO game_config (key, value_jsonb) VALUES
    ('risk_zone_blocking',     'true'),
    ('risk_zone_min_severity', '3'),
    ('loop_close_radius_m',    '30'),
    ('territory_min_area_m2',  '500'),
    ('territory_max_area_m2',  '500000');

-- +goose Down
DROP TABLE IF EXISTS game_config;
DROP TABLE IF EXISTS audit_log;
