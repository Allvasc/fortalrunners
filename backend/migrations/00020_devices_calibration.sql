-- +goose Up
-- Dispositivos pareados e calibração de passada (plano §4, §9).

CREATE TABLE devices (
    id                     text PRIMARY KEY,
    user_id                text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind                   text NOT NULL,          -- phone | watch | footpod | hrm
    brand                  text,
    model                  text,
    last_seen_at           timestamptz,
    precision_profile_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at             timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX devices_user_idx ON devices (user_id);

CREATE TABLE user_stride_calibration (
    user_id      text PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    stride_len_m numeric(4,3) NOT NULL,
    confidence   numeric(4,3) NOT NULL DEFAULT 0,
    sample_count int NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS user_stride_calibration;
DROP TABLE IF EXISTS devices;
