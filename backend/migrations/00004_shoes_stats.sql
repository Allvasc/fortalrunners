-- +goose Up

CREATE TYPE shoe_status AS ENUM ('active', 'rotation', 'retired');

-- shoe_catalog — base global de marca/modelo (submissoes da comunidade).
CREATE TABLE shoe_catalog (
    id                  text PRIMARY KEY,
    brand               text NOT NULL,
    model               text NOT NULL,
    year                int,
    default_lifespan_m  int NOT NULL DEFAULT 700000,
    agg_median_lifespan_m int,
    agg_user_count      int NOT NULL DEFAULT 0,
    agg_rating          numeric(3,2),
    status              text NOT NULL DEFAULT 'approved',
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (brand, model, year)
);

-- shoes — pares do corredor.
CREATE TABLE shoes (
    id                  text PRIMARY KEY,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    catalog_id          text REFERENCES shoe_catalog (id) ON DELETE SET NULL,
    brand               text NOT NULL,
    model               text NOT NULL,
    nickname            text,
    photo_key           text,
    purchased_at        date,
    purchase_price_cents int,
    initial_distance_m  int NOT NULL DEFAULT 0,
    lifespan_goal_m     int NOT NULL DEFAULT 700000,
    surface_pref        text[],
    status              shoe_status NOT NULL DEFAULT 'active',
    retired_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX shoes_user_idx ON shoes (user_id) WHERE status <> 'retired';

-- shoe_stats — rollup mantido a cada corrida valida.
CREATE TABLE shoe_stats (
    shoe_id           text PRIMARY KEY REFERENCES shoes (id) ON DELETE CASCADE,
    total_distance_m  bigint NOT NULL DEFAULT 0,
    total_steps       bigint NOT NULL DEFAULT 0,
    total_moving_s    bigint NOT NULL DEFAULT 0,
    run_count         int NOT NULL DEFAULT 0,
    elevation_gain_m  int NOT NULL DEFAULT 0,
    last_run_at       timestamptz
);

-- lifetime_stats — acumulado vitalicio por corredor.
CREATE TABLE lifetime_stats (
    user_id             text PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    total_moving_s      bigint NOT NULL DEFAULT 0,
    total_distance_m    bigint NOT NULL DEFAULT 0,
    total_steps         bigint NOT NULL DEFAULT 0,
    run_count           int NOT NULL DEFAULT 0,
    elevation_gain_m    bigint NOT NULL DEFAULT 0,
    territory_area_m2   numeric(14,2) NOT NULL DEFAULT 0,
    active_days         int NOT NULL DEFAULT 0,
    current_streak_days int NOT NULL DEFAULT 0,
    longest_streak_days int NOT NULL DEFAULT 0,
    last_run_date       date,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- personal_records — recordes pessoais.
CREATE TABLE personal_records (
    id           text PRIMARY KEY,
    user_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key          text NOT NULL,   -- longest_distance | max_elevation | biggest_territory
    value        numeric NOT NULL,
    run_id       text REFERENCES runs (id) ON DELETE SET NULL,
    achieved_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, key)
);

ALTER TABLE runs ADD COLUMN shoe_id text REFERENCES shoes (id) ON DELETE SET NULL;
ALTER TABLE runs ADD COLUMN stats_rolled_at timestamptz;  -- guarda contra dupla contagem

-- +goose Down
ALTER TABLE runs DROP COLUMN IF EXISTS stats_rolled_at;
ALTER TABLE runs DROP COLUMN IF EXISTS shoe_id;
DROP TABLE IF EXISTS personal_records;
DROP TABLE IF EXISTS lifetime_stats;
DROP TABLE IF EXISTS shoe_stats;
DROP TABLE IF EXISTS shoes;
DROP TABLE IF EXISTS shoe_catalog;
DROP TYPE IF EXISTS shoe_status;
