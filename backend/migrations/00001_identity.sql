-- +goose Up
-- Extensões (idempotente; também criadas no init.sql do container de dev).
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS citext;

-- Enums de identidade.
CREATE TYPE user_role   AS ENUM ('runner', 'organizer', 'moderator', 'admin');
CREATE TYPE user_status AS ENUM ('active', 'suspended', 'shadow_banned', 'banned', 'deleted');
CREATE TYPE id_provider AS ENUM ('google', 'apple', 'email');

-- users — raiz do modelo.
CREATE TABLE users (
    id                 text PRIMARY KEY,                 -- ULID
    athlete_id        text NOT NULL UNIQUE,              -- ex.: FR-0001234
    username          citext NOT NULL UNIQUE,
    email             citext NOT NULL UNIQUE,
    email_verified_at timestamptz,
    password_hash     text,                              -- null quando só social
    totp_secret_enc   bytea,
    display_name      text,
    color_hex         char(7) NOT NULL DEFAULT '#08A6A0',
    avatar_key        text,
    birth_date        date,
    home_blur_geom    geometry(Polygon, 4326),
    privacy_jsonb     jsonb NOT NULL DEFAULT '{}'::jsonb,
    consent_jsonb     jsonb NOT NULL DEFAULT '{}'::jsonb,
    role              user_role   NOT NULL DEFAULT 'runner',
    status            user_status NOT NULL DEFAULT 'active',
    city_id           text NOT NULL DEFAULT 'fortaleza',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz
);
CREATE INDEX users_email_idx ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX users_username_trgm ON users USING gin (username gin_trgm_ops);
CREATE INDEX users_home_blur_gix ON users USING gist (home_blur_geom);

-- identities — logins sociais ligados à conta.
CREATE TABLE identities (
    id           text PRIMARY KEY,
    user_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider     id_provider NOT NULL,
    provider_uid text NOT NULL,
    email        citext,
    linked_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);
CREATE INDEX identities_user_idx ON identities (user_id);

-- auth_sessions — refresh token rotativo por dispositivo.
CREATE TABLE auth_sessions (
    id                 text PRIMARY KEY,
    user_id            text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    refresh_token_hash text NOT NULL UNIQUE,
    family_id          text NOT NULL,              -- detecção de reuso
    device_id          text,
    ip                 inet,
    user_agent         text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz NOT NULL,
    revoked_at         timestamptz
);
CREATE INDEX auth_sessions_user_idx   ON auth_sessions (user_id);
CREATE INDEX auth_sessions_family_idx ON auth_sessions (family_id);
CREATE INDEX auth_sessions_expiry_idx ON auth_sessions (expires_at);

-- Sequência do número público do atleta (FR-0000001, FR-0000002, ...).
CREATE SEQUENCE athlete_id_seq START 1;

-- +goose Down
DROP SEQUENCE IF EXISTS athlete_id_seq;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS id_provider;
DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS user_role;
