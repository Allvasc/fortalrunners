-- +goose Up

CREATE TYPE run_source AS ENUM ('phone', 'watch', 'import');
CREATE TYPE run_status AS ENUM ('processing', 'valid', 'flagged', 'rejected');
CREATE TYPE terr_status AS ENUM ('active', 'contested', 'decayed', 'revoked');
CREATE TYPE risk_source AS ENUM ('admin', 'hazard_cluster', 'public_data');

-- neighborhoods — bairros (seed simplificado; limites reais virão do IBGE).
CREATE TABLE neighborhoods (
    id       text PRIMARY KEY,
    city_id  text NOT NULL DEFAULT 'fortaleza',
    name     text NOT NULL,
    geom     geometry(MultiPolygon, 4326) NOT NULL,
    h3_total int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX neighborhoods_geom_gix ON neighborhoods USING gist (geom);
CREATE INDEX neighborhoods_city_idx ON neighborhoods (city_id);

-- risk_zones — gate de conquista (territoryAllowedAt / missionAllowedAt).
CREATE TABLE risk_zones (
    id          text PRIMARY KEY,
    city_id     text NOT NULL DEFAULT 'fortaleza',
    geom        geometry(MultiPolygon, 4326) NOT NULL,
    severity    smallint NOT NULL DEFAULT 3 CHECK (severity BETWEEN 1 AND 5),
    source      risk_source NOT NULL DEFAULT 'admin',
    note        text,
    active_from timestamptz,
    active_to   timestamptz,
    status      text NOT NULL DEFAULT 'active',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX risk_zones_geom_gix ON risk_zones USING gist (geom);

-- runs — uma corrida.
CREATE TABLE runs (
    id                 text PRIMARY KEY,
    user_id            text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    started_at         timestamptz NOT NULL,
    ended_at           timestamptz NOT NULL,
    distance_m         int NOT NULL DEFAULT 0,
    moving_s           int NOT NULL DEFAULT 0,
    duration_s         int NOT NULL DEFAULT 0,
    avg_pace_s         int NOT NULL DEFAULT 0,
    elevation_gain_m   int NOT NULL DEFAULT 0,
    avg_cadence_spm    int,
    step_count         int,
    gnss_mode          text,
    avg_hdop           numeric(4,2),
    signal_quality_pct smallint,
    data_source        run_source NOT NULL DEFAULT 'phone',
    weather_jsonb      jsonb,
    fraud_score        numeric(4,3) NOT NULL DEFAULT 0,
    territory_area_m2  numeric(12,2) NOT NULL DEFAULT 0,
    new_blocks         int NOT NULL DEFAULT 0,
    status             run_status NOT NULL DEFAULT 'processing',
    raw_gpx_key        text,
    error              text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX runs_user_started_idx ON runs (user_id, started_at DESC);
CREATE INDEX runs_status_idx ON runs (status) WHERE status IN ('processing', 'flagged');

-- run_tracks — traçado limpo + amostra bruta (particionar por mês na Fase 2).
CREATE TABLE run_tracks (
    run_id               text PRIMARY KEY REFERENCES runs (id) ON DELETE CASCADE,
    geom                 geometry(LineString, 4326) NOT NULL,
    points_jsonb         jsonb NOT NULL,
    sensor_streams_jsonb jsonb,
    quality_segments_jsonb jsonb
);
CREATE INDEX run_tracks_geom_gix ON run_tracks USING gist (geom);

-- territories — uma linha por conquista.
CREATE TABLE territories (
    id              text PRIMARY KEY,
    user_id         text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    run_id          text REFERENCES runs (id) ON DELETE SET NULL,
    neighborhood_id text REFERENCES neighborhoods (id) ON DELETE SET NULL,
    geom            geometry(MultiPolygon, 4326) NOT NULL,
    area_m2         numeric(12,2) NOT NULL,
    claimed_at      timestamptz NOT NULL DEFAULT now(),
    decays_at       timestamptz,
    status          terr_status NOT NULL DEFAULT 'active',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX territories_user_status_idx ON territories (user_id, status);
CREATE INDEX territories_geom_gix ON territories USING gist (geom);

-- h3_cells — unidade de contagem (res 9). PK = h3_index.
CREATE TABLE h3_cells (
    h3_index        bigint PRIMARY KEY,
    city_id         text NOT NULL DEFAULT 'fortaleza',
    neighborhood_id text REFERENCES neighborhoods (id) ON DELETE SET NULL,
    owner_id        text REFERENCES users (id) ON DELETE SET NULL,
    territory_id    text REFERENCES territories (id) ON DELETE SET NULL,
    claimed_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX h3_cells_owner_idx ON h3_cells (owner_id);
CREATE INDEX h3_cells_nbh_idx ON h3_cells (neighborhood_id);

-- +goose Down
DROP TABLE IF EXISTS h3_cells;
DROP TABLE IF EXISTS territories;
DROP TABLE IF EXISTS run_tracks;
DROP TABLE IF EXISTS runs;
DROP TABLE IF EXISTS risk_zones;
DROP TABLE IF EXISTS neighborhoods;
DROP TYPE IF EXISTS risk_source;
DROP TYPE IF EXISTS terr_status;
DROP TYPE IF EXISTS run_status;
DROP TYPE IF EXISTS run_source;
