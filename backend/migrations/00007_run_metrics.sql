-- +goose Up
-- Métricas de precisão por corrida: splits por km, elevação suavizada,
-- cadência, FC e pace ajustado ao aclive (GAP, modelo de Minetti).
-- Calculado no ingest, a partir do traçado limpo + streams de sensor.

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

-- +goose Down
DROP TABLE IF EXISTS run_metrics;
