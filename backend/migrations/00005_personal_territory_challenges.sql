-- +goose Up
-- Pivô de mecânica:
--   1. Território é PERMANENTE e PESSOAL — nenhum corredor apaga ou disputa o do
--      outro. Sem decaimento, sem ciclo que zera, sem sobreposição contestada.
--      A competição base é acumulada: quem cobre mais área (all-time).
--   2. Os ciclos passam a ser DESAFIOS (semanal / quinzenal / mensal): competições
--      com janela e recompensa própria, que NÃO mexem no território de ninguém.

-- --- 1. território permanente ---
ALTER TABLE territories DROP COLUMN IF EXISTS decays_at;

ALTER TYPE terr_status RENAME TO terr_status_old;
CREATE TYPE terr_status AS ENUM ('active', 'revoked');  -- revoked = removido por fraude (admin)
ALTER TABLE territories ALTER COLUMN status DROP DEFAULT;
ALTER TABLE territories ALTER COLUMN status TYPE terr_status
    USING (CASE status::text WHEN 'revoked' THEN 'revoked' ELSE 'active' END::terr_status);
ALTER TABLE territories ALTER COLUMN status SET DEFAULT 'active';
DROP TYPE terr_status_old;

-- h3_cells passa a ser POR CORREDOR (sobreposição livre; cobertura é pessoal).
ALTER TABLE h3_cells DROP CONSTRAINT h3_cells_owner_id_fkey;
DELETE FROM h3_cells WHERE owner_id IS NULL;
ALTER TABLE h3_cells ALTER COLUMN owner_id SET NOT NULL;
ALTER TABLE h3_cells DROP CONSTRAINT h3_cells_pkey;
ALTER TABLE h3_cells ADD PRIMARY KEY (h3_index, owner_id);
ALTER TABLE h3_cells ADD FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE;

-- --- 2. badges (conquistas permanentes) ---
CREATE TABLE badges (
    code          text PRIMARY KEY,
    name          text NOT NULL,
    description   text,
    shape         text NOT NULL DEFAULT 'circle',  -- circle | hex | shield
    type          text NOT NULL DEFAULT 'activity', -- activity | coverage | challenge | landmark | collection
    criteria_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_badges (
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    badge_code          text NOT NULL REFERENCES badges (code) ON DELETE RESTRICT,
    source              text NOT NULL DEFAULT 'milestone', -- milestone | challenge | manual
    challenge_period_id text,
    tier                text,
    earned_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, badge_code)
);
CREATE INDEX user_badges_user_idx ON user_badges (user_id);

-- --- 3. desafios recorrentes ---
CREATE TYPE challenge_cadence AS ENUM ('weekly', 'biweekly', 'monthly', 'oneoff');
CREATE TYPE challenge_metric  AS ENUM ('new_area_m2', 'new_blocks', 'new_neighborhoods', 'distance_m', 'elevation_gain_m');

CREATE TABLE challenges (
    id                text PRIMARY KEY,
    slug              text NOT NULL UNIQUE,
    title             text NOT NULL,
    description       text,
    cadence           challenge_cadence NOT NULL,
    metric            challenge_metric NOT NULL,
    goal              numeric,             -- alvo para "completar"; null = ranking puro
    reward_badge_code text REFERENCES badges (code) ON DELETE SET NULL,
    reward_top_n      int,                 -- top N ganham a badge; null = todos que batem o goal
    anchor_date       date NOT NULL DEFAULT current_date, -- ancora dos periodos quinzenais
    active            bool NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE challenge_periods (
    id           text PRIMARY KEY,
    challenge_id text NOT NULL REFERENCES challenges (id) ON DELETE CASCADE,
    period_no    int NOT NULL,
    starts_at    timestamptz NOT NULL,
    ends_at      timestamptz NOT NULL,
    closed_at    timestamptz,
    UNIQUE (challenge_id, period_no)
);
CREATE INDEX challenge_periods_window_idx ON challenge_periods (challenge_id, starts_at, ends_at);

CREATE TABLE challenge_entries (
    period_id     text NOT NULL REFERENCES challenge_periods (id) ON DELETE CASCADE,
    user_id       text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    metric_value  numeric NOT NULL DEFAULT 0,
    rank          int,
    completed     bool NOT NULL DEFAULT false,
    badge_awarded bool NOT NULL DEFAULT false,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (period_id, user_id)
);

-- --- seed: badges + desafios padrão ---
INSERT INTO badges (code, name, description, shape, type) VALUES
    ('conquistador_semana', 'Conquistador da semana', 'Top 3 em área conquistada numa semana', 'circle', 'challenge'),
    ('explorador_quinzena', 'Explorador da quinzena', 'Cobriu 3 bairros novos numa quinzena', 'hex', 'challenge'),
    ('maratonista_mes', 'Maratonista do mês', 'Top 10 em distância num mês', 'circle', 'challenge');

INSERT INTO challenges (id, slug, title, description, cadence, metric, goal, reward_badge_code, reward_top_n) VALUES
    ('chl_semana_conquista', 'semana-conquista', 'Conquista da semana',
     'Quem conquista mais área nova nesta semana.', 'weekly', 'new_area_m2', NULL, 'conquistador_semana', 3),
    ('chl_quinzena_bairros', 'quinzena-bairros', 'Quinzena dos bairros',
     'Cubra pela primeira vez 3 bairros diferentes em 15 dias.', 'biweekly', 'new_neighborhoods', 3, 'explorador_quinzena', NULL),
    ('chl_mes_maratona', 'mes-maratona', 'Mês maratonista',
     'A maior distância acumulada no mês.', 'monthly', 'distance_m', NULL, 'maratonista_mes', 10);

-- +goose Down
DROP TABLE IF EXISTS challenge_entries;
DROP TABLE IF EXISTS challenge_periods;
DROP TABLE IF EXISTS challenges;
DROP TYPE IF EXISTS challenge_metric;
DROP TYPE IF EXISTS challenge_cadence;
DROP TABLE IF EXISTS user_badges;
DROP TABLE IF EXISTS badges;

ALTER TABLE h3_cells DROP CONSTRAINT h3_cells_pkey;
ALTER TABLE h3_cells ADD PRIMARY KEY (h3_index);
ALTER TABLE territories ADD COLUMN decays_at timestamptz;
