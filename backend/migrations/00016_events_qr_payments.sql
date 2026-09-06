-- +goose Up
-- Migration 00016: Eventos, QR Code, Estações, Pagamentos (Asaas) e Telemetria de IA

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'org_kind') THEN
        CREATE TYPE org_kind AS ENUM ('race', 'company', 'brand', 'ngo');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'event_type') THEN
        CREATE TYPE event_type AS ENUM ('race', 'challenge', 'corporate', 'brand', 'charity');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'station_role') THEN
        CREATE TYPE station_role AS ENUM ('kit', 'start', 'checkpoint', 'finish', 'lap', 'aid', 'sponsor');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'qr_kind') THEN
        CREATE TYPE qr_kind AS ENUM ('rotating', 'static');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'scan_kind') THEN
        CREATE TYPE scan_kind AS ENUM ('kit', 'start', 'checkpoint', 'finish', 'lap', 'aid', 'sponsor', 'friend', 'landmark');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'scan_status') THEN
        CREATE TYPE scan_status AS ENUM ('recorded', 'pending', 'disputed', 'rejected');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'order_kind') THEN
        CREATE TYPE order_kind AS ENUM ('event_registration', 'subscription', 'cosmetic', 'donation');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'order_status') THEN
        CREATE TYPE order_status AS ENUM ('pending', 'paid', 'failed', 'refunded', 'cancelled');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'pay_method') THEN
        CREATE TYPE pay_method AS ENUM ('pix', 'boleto', 'card');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'pay_status') THEN
        CREATE TYPE pay_status AS ENUM ('pending', 'confirmed', 'received', 'overdue', 'refunded', 'chargeback');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS organizers (
    id                  text PRIMARY KEY,
    owner_id            text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name                text NOT NULL,
    kind                org_kind NOT NULL DEFAULT 'race',
    contact_email       text NOT NULL,
    doc_number          text,
    plan                text NOT NULL DEFAULT 'basic',
    status              text NOT NULL DEFAULT 'active',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS events (
    id                  text PRIMARY KEY,
    slug                text UNIQUE NOT NULL,
    organizer_id        text NOT NULL REFERENCES organizers (id) ON DELETE CASCADE,
    title               text NOT NULL,
    description         text NOT NULL DEFAULT '',
    type                event_type NOT NULL DEFAULT 'race',
    starts_at           timestamptz NOT NULL,
    ends_at             timestamptz NOT NULL,
    location_name       text NOT NULL DEFAULT 'Fortaleza, CE',
    area_geom           geometry(MultiPolygon, 4326),
    route_geom          geometry(LineString, 4326),
    goals_jsonb         jsonb NOT NULL DEFAULT '{}'::jsonb,
    rewards_jsonb       jsonb NOT NULL DEFAULT '{}'::jsonb,
    status              text NOT NULL DEFAULT 'live',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS event_prices (
    id                  text PRIMARY KEY,
    event_id            text NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    name                text NOT NULL,
    category            text NOT NULL DEFAULT 'geral',
    amount_cents        int NOT NULL DEFAULT 0,
    starts_at           timestamptz,
    ends_at             timestamptz,
    quota               int NOT NULL DEFAULT 100,
    sold                int NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS event_stations (
    id                  text PRIMARY KEY,
    event_id            text NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    name                text NOT NULL,
    role                station_role NOT NULL DEFAULT 'checkpoint',
    ord                 int NOT NULL DEFAULT 1,
    geom                geometry(Point, 4326),
    radius_m            int NOT NULL DEFAULT 30,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id                  text PRIMARY KEY,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind                order_kind NOT NULL DEFAULT 'event_registration',
    status              order_status NOT NULL DEFAULT 'pending',
    amount_cents        int NOT NULL DEFAULT 0,
    discount_cents      int NOT NULL DEFAULT 0,
    currency            char(3) NOT NULL DEFAULT 'BRL',
    asaas_customer_id   text,
    idempotency_key     text UNIQUE,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS event_participants (
    event_id            text NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    order_id            text REFERENCES orders (id) ON DELETE SET NULL,
    bib_number          text,
    category            text NOT NULL DEFAULT '5k',
    shirt_size          text NOT NULL DEFAULT 'M',
    progress_jsonb      jsonb NOT NULL DEFAULT '{}'::jsonb,
    joined_at           timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz,
    PRIMARY KEY (event_id, user_id)
);

CREATE TABLE IF NOT EXISTS qr_tokens (
    id                  text PRIMARY KEY,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind                qr_kind NOT NULL DEFAULT 'rotating',
    payload_sig         text NOT NULL,
    event_id            text REFERENCES events (id) ON DELETE CASCADE,
    issued_at           timestamptz NOT NULL DEFAULT now(),
    expires_at          timestamptz NOT NULL,
    revoked             boolean NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS scan_events (
    id                  text PRIMARY KEY,
    athlete_id          text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    station_id          text REFERENCES event_stations (id) ON DELETE SET NULL,
    event_id            text REFERENCES events (id) ON DELETE SET NULL,
    scanned_by          text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind                scan_kind NOT NULL DEFAULT 'checkpoint',
    geom                geometry(Point, 4326),
    scanned_at          timestamptz NOT NULL DEFAULT now(),
    status              scan_status NOT NULL DEFAULT 'recorded'
);

CREATE TABLE IF NOT EXISTS payments (
    id                  text PRIMARY KEY,
    order_id            text NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    asaas_charge_id     text UNIQUE,
    method              pay_method NOT NULL DEFAULT 'pix',
    status              pay_status NOT NULL DEFAULT 'pending',
    paid_at             timestamptz,
    net_amount_cents    int NOT NULL DEFAULT 0,
    receipt_url         text,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_interactions (
    id                  text PRIMARY KEY,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    feature             text NOT NULL DEFAULT 'coach',
    model               text NOT NULL DEFAULT 'gemini-flash-2.0',
    prompt_tokens       int NOT NULL DEFAULT 0,
    completion_tokens   int NOT NULL DEFAULT 0,
    latency_ms          int NOT NULL DEFAULT 0,
    outcome             text NOT NULL DEFAULT 'ok',
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- Seed de organizadores e eventos de Fortaleza.
-- Só roda quando já existe um usuário para ser dono do organizador.
-- Em um banco de produção novo (sem usuários) o bloco é ignorado.
DO $$
DECLARE seed_owner text;
BEGIN
    SELECT id INTO seed_owner FROM users LIMIT 1;
    IF seed_owner IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO organizers (id, owner_id, name, kind, contact_email)
    VALUES
      ('org_fortal_marathon', seed_owner, 'Associação de Corredores de Fortaleza', 'race', 'contato@fortalmarathon.com.br')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO events (id, slug, organizer_id, title, description, type, starts_at, ends_at, location_name)
    VALUES
      ('evt_beira_mar_night', 'beira-mar-night-run-2026', 'org_fortal_marathon', 'Beira Mar Night Run 2026', 'Corrida noturna de 5k e 10k ao longo do calçadão da Beira-Mar com hidratação e medalha.', 'race', now() + interval '7 days', now() + interval '7 days 4 hours', 'Beira-Mar, Fortaleza - CE'),
      ('evt_desafio_praia_futuro', 'desafio-praia-do-futuro', 'org_fortal_marathon', 'Desafio Orla & Areia Praia do Futuro', 'Corrida rústica de praia com terreno misto e selos exclusivos.', 'challenge', now() + interval '21 days', now() + interval '21 days 5 hours', 'Praia do Futuro, Fortaleza - CE')
    ON CONFLICT (id) DO NOTHING;

    INSERT INTO event_prices (id, event_id, name, category, amount_cents, quota)
    VALUES
      ('prc_bm_5k', 'evt_beira_mar_night', 'Lote 1 - 5km', '5k', 4990, 200),
      ('prc_bm_10k', 'evt_beira_mar_night', 'Lote 1 - 10km', '10k', 6990, 150)
    ON CONFLICT (id) DO NOTHING;
END $$;

-- +goose Down
DROP TABLE IF EXISTS ai_interactions;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS scan_events;
DROP TABLE IF EXISTS qr_tokens;
DROP TABLE IF EXISTS event_participants;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS event_stations;
DROP TABLE IF EXISTS event_prices;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS organizers;
