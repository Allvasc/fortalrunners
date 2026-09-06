-- +goose Up
-- Endurecimento da segurança pessoal (plano §6, §14):
--   - telefone do contato de emergência cifrado em repouso (AES-256-GCM)
--   - SOS ganha link de beacon com token que expira, PIN de cancelamento e resolução

ALTER TABLE safety_contacts ADD COLUMN phone_enc bytea;
ALTER TABLE safety_contacts ALTER COLUMN phone DROP NOT NULL;

ALTER TABLE sos_events ADD COLUMN run_id text REFERENCES runs (id) ON DELETE SET NULL;
ALTER TABLE sos_events ADD COLUMN share_token text UNIQUE;
ALTER TABLE sos_events ADD COLUMN cancel_pin_hash text;
ALTER TABLE sos_events ADD COLUMN resolved_by text REFERENCES users (id) ON DELETE SET NULL;
ALTER TABLE sos_events ADD COLUMN resolved_at timestamptz;
ALTER TABLE sos_events ADD COLUMN last_lat double precision;
ALTER TABLE sos_events ADD COLUMN last_lng double precision;

CREATE INDEX sos_events_share_idx ON sos_events (share_token) WHERE share_token IS NOT NULL;
CREATE INDEX sos_events_active_idx ON sos_events (user_id) WHERE status = 'active';

CREATE TABLE sos_notifications (
    id         text PRIMARY KEY,
    sos_id     text NOT NULL REFERENCES sos_events (id) ON DELETE CASCADE,
    contact_id text REFERENCES safety_contacts (id) ON DELETE SET NULL,
    channel    text NOT NULL,               -- sms | push | log
    status     text NOT NULL DEFAULT 'queued',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sos_notifications_sos_idx ON sos_notifications (sos_id);

-- +goose Down
DROP TABLE IF EXISTS sos_notifications;
DROP INDEX IF EXISTS sos_events_active_idx;
DROP INDEX IF EXISTS sos_events_share_idx;
ALTER TABLE sos_events DROP COLUMN IF EXISTS last_lng;
ALTER TABLE sos_events DROP COLUMN IF EXISTS last_lat;
ALTER TABLE sos_events DROP COLUMN IF EXISTS resolved_at;
ALTER TABLE sos_events DROP COLUMN IF EXISTS resolved_by;
ALTER TABLE sos_events DROP COLUMN IF EXISTS cancel_pin_hash;
ALTER TABLE sos_events DROP COLUMN IF EXISTS share_token;
ALTER TABLE sos_events DROP COLUMN IF EXISTS run_id;
ALTER TABLE safety_contacts DROP COLUMN IF EXISTS phone_enc;
