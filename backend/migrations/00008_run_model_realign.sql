-- +goose Up
-- Alinha o modelo de corrida ao banco-e-integracoes.html:
--   * runs recebe o conjunto completo de colunas de precisão (o doc já as lista)
--   * splits e traçado com altitude vão para run_tracks (não numa tabela à parte)
--   * personal_records usa distance_key / value_s
--   * run_metrics (introduzida em 00007, fora do modelo) é removida

-- --- runs: colunas de precisão do modelo ---
ALTER TABLE runs
    ADD COLUMN gap_pace_s        int,
    ADD COLUMN avg_hr            int,
    ADD COLUMN max_hr            int,
    ADD COLUMN calories          int,
    ADD COLUMN max_cadence_spm   int,
    ADD COLUMN stride_len_m      numeric(4,3),
    ADD COLUMN running_power_w   numeric(6,1),
    ADD COLUMN vert_osc_mm       numeric(5,1),
    ADD COLUMN ground_contact_ms numeric(6,1),
    ADD COLUMN elev_loss_m       int NOT NULL DEFAULT 0,
    ADD COLUMN alt_min_m         numeric(6,1),
    ADD COLUMN alt_max_m         numeric(6,1),
    ADD COLUMN best_km_pace_s    int,
    ADD COLUMN device_id         text;  -- FK devices quando a tabela existir

-- --- run_tracks: altitude no traçado + splits + melhores esforços + chave de partição ---
ALTER TABLE run_tracks
    ADD COLUMN started_at         timestamptz,
    ADD COLUMN splits_jsonb       jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN best_efforts_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb;
UPDATE run_tracks rt SET started_at = r.started_at FROM runs r WHERE r.id = rt.run_id;
ALTER TABLE run_tracks ALTER COLUMN geom TYPE geometry(LineStringZ, 4326) USING ST_Force3D(geom);
CREATE INDEX run_tracks_started_idx ON run_tracks (started_at);

-- --- personal_records: nomes do modelo ---
ALTER TABLE personal_records RENAME COLUMN key TO distance_key;
ALTER TABLE personal_records RENAME COLUMN value TO value_s;
ALTER TABLE personal_records ALTER COLUMN value_s TYPE bigint USING round(value_s)::bigint;

-- --- run_metrics: fora do modelo, migra para as colunas acima ---
DROP TABLE IF EXISTS run_metrics;

-- +goose Down
CREATE TABLE run_metrics (
    run_id                text PRIMARY KEY REFERENCES runs (id) ON DELETE CASCADE,
    splits_jsonb          jsonb NOT NULL DEFAULT '[]'::jsonb,
    elev_gain_m           numeric(7,1) NOT NULL DEFAULT 0,
    elev_loss_m           numeric(7,1) NOT NULL DEFAULT 0,
    alt_min_m             numeric(7,1),
    alt_max_m             numeric(7,1),
    avg_cadence_spm       int,
    max_cadence_spm       int,
    avg_hr_bpm            int,
    max_hr_bpm            int,
    best_km_pace_s        int,
    grade_adjusted_pace_s int,
    has_altitude          boolean NOT NULL DEFAULT false,
    has_cadence           boolean NOT NULL DEFAULT false,
    has_heart_rate        boolean NOT NULL DEFAULT false,
    computed_at           timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE personal_records ALTER COLUMN value_s TYPE numeric USING value_s::numeric;
ALTER TABLE personal_records RENAME COLUMN value_s TO value;
ALTER TABLE personal_records RENAME COLUMN distance_key TO key;

DROP INDEX IF EXISTS run_tracks_started_idx;
ALTER TABLE run_tracks ALTER COLUMN geom TYPE geometry(LineString, 4326) USING ST_Force2D(geom);
ALTER TABLE run_tracks
    DROP COLUMN IF EXISTS splits_jsonb,
    DROP COLUMN IF EXISTS best_efforts_jsonb,
    DROP COLUMN IF EXISTS started_at;

ALTER TABLE runs
    DROP COLUMN IF EXISTS gap_pace_s, DROP COLUMN IF EXISTS avg_hr, DROP COLUMN IF EXISTS max_hr,
    DROP COLUMN IF EXISTS calories, DROP COLUMN IF EXISTS max_cadence_spm, DROP COLUMN IF EXISTS stride_len_m,
    DROP COLUMN IF EXISTS running_power_w, DROP COLUMN IF EXISTS vert_osc_mm, DROP COLUMN IF EXISTS ground_contact_ms,
    DROP COLUMN IF EXISTS elev_loss_m, DROP COLUMN IF EXISTS alt_min_m, DROP COLUMN IF EXISTS alt_max_m,
    DROP COLUMN IF EXISTS best_km_pace_s, DROP COLUMN IF EXISTS device_id;
