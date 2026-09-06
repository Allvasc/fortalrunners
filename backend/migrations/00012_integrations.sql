-- +goose Up
-- Integrações com apps de corrida (plano §7): conectar conta externa, importar
-- histórico + corridas novas (webhook), pelo MESMO pipeline de território, com
-- data_source='import' e anti-fraude mais rígido.

CREATE TYPE integ_provider AS ENUM ('strava', 'garmin', 'fitbit', 'polar', 'apple_health', 'health_connect');
CREATE TYPE integ_status   AS ENUM ('active', 'expired', 'revoked', 'error');
CREATE TYPE import_kind    AS ENUM ('backfill', 'delta');
CREATE TYPE job_status     AS ENUM ('queued', 'running', 'done', 'failed');

CREATE TABLE integrations (
    id                  text PRIMARY KEY,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider            integ_provider NOT NULL,
    external_athlete_id text NOT NULL,
    access_token_enc    bytea,           -- AES-256-GCM (chave no cofre)
    refresh_token_enc   bytea,
    token_expires_at    timestamptz,
    scopes              text[] NOT NULL DEFAULT '{}',
    webhook_sub_id      text,
    last_sync_at        timestamptz,
    status              integ_status NOT NULL DEFAULT 'active',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_athlete_id)
);
CREATE INDEX integrations_user_idx ON integrations (user_id);

CREATE TABLE import_jobs (
    id             text PRIMARY KEY,
    user_id        text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    integration_id text NOT NULL REFERENCES integrations (id) ON DELETE CASCADE,
    kind           import_kind NOT NULL,
    cursor         text,
    status         job_status NOT NULL DEFAULT 'queued',
    stats_jsonb    jsonb NOT NULL DEFAULT '{}'::jsonb,   -- {imported, duplicated, errors}
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX import_jobs_integration_idx ON import_jobs (integration_id);
CREATE INDEX import_jobs_pending_idx ON import_jobs (status) WHERE status IN ('queued', 'running');

CREATE TABLE webhook_events (
    id           text PRIMARY KEY,
    provider     text NOT NULL,
    external_id  text NOT NULL,
    event_type   text,
    payload_jsonb jsonb NOT NULL,
    signature_ok bool NOT NULL DEFAULT false,
    processed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

-- rastreia a origem de uma corrida importada (dedup + revogação por provedor).
ALTER TABLE runs ADD COLUMN import_ref text;   -- provider:external_activity_id
CREATE UNIQUE INDEX runs_import_ref_uq ON runs (import_ref) WHERE import_ref IS NOT NULL;

-- +goose Down
ALTER TABLE runs DROP COLUMN IF EXISTS import_ref;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS import_jobs;
DROP TABLE IF EXISTS integrations;
DROP TYPE IF EXISTS job_status;
DROP TYPE IF EXISTS import_kind;
DROP TYPE IF EXISTS integ_status;
DROP TYPE IF EXISTS integ_provider;
