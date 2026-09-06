-- +goose Up
-- Social (Amigos, Feed & Kudos), Clubes e Segurança (plano §5, §6 / banco §3, §6).

CREATE TYPE friend_status AS ENUM ('pending', 'accepted', 'blocked');
CREATE TYPE activity_type AS ENUM ('run', 'claim', 'badge', 'checkin');

-- Amizades (user_id < friend_id obrigatoriamente)
CREATE TABLE friendships (
    user_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    friend_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    requested_by text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status       friend_status NOT NULL DEFAULT 'pending',
    created_at   timestamptz NOT NULL DEFAULT now(),
    accepted_at  timestamptz,
    PRIMARY KEY (user_id, friend_id),
    CONSTRAINT chk_friendship_order CHECK (user_id < friend_id)
);
CREATE INDEX friendships_user_idx ON friendships (user_id);
CREATE INDEX friendships_friend_idx ON friendships (friend_id);

-- Clubes de Corrida
CREATE TABLE clubs (
    id              text PRIMARY KEY,
    owner_id        text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name            text NOT NULL,
    description     text,
    neighborhood_id text REFERENCES neighborhoods (id) ON DELETE SET NULL,
    color_hex       char(7) NOT NULL DEFAULT '#0E7C86',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE club_members (
    club_id   text NOT NULL REFERENCES clubs (id) ON DELETE CASCADE,
    user_id   text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role      text NOT NULL DEFAULT 'member', -- owner, admin, member
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (club_id, user_id)
);
CREATE INDEX club_members_user_idx ON club_members (user_id);

-- Feed de Atividades Sociais
CREATE TABLE activity_events (
    id            text PRIMARY KEY,
    actor_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type          activity_type NOT NULL,
    subject_id    text NOT NULL,
    payload_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_events_actor_idx ON activity_events (actor_id, created_at DESC);

-- Kudos / Curtidas em Corridas
CREATE TABLE kudos (
    run_id     text NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, user_id)
);

-- Contatos de Emergência
CREATE TABLE safety_contacts (
    id         text PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL,
    phone      text NOT NULL,
    relation   text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX safety_contacts_user_idx ON safety_contacts (user_id);

-- Eventos de Alerta SOS
CREATE TABLE sos_events (
    id         text PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    geom       geometry(Point, 4326),
    note       text,
    status     text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- --- SEED INICIAL ---
-- Só semeia os clubes de demonstração quando já existe um usuário para ser dono.
-- Em um banco de produção novo (sem usuários) o bloco é ignorado.
-- +goose StatementBegin
DO $$
DECLARE seed_owner text;
BEGIN
    SELECT id INTO seed_owner FROM users LIMIT 1;
    IF seed_owner IS NULL THEN
        RETURN;
    END IF;
    INSERT INTO clubs (id, owner_id, name, description, color_hex) VALUES
        ('clb_beiramar_runners', seed_owner, 'Beira-Mar Runners', 'Clube oficial de corredores do calçadão da Beira-Mar.', '#0E7C86'),
        ('clb_coco_trail', seed_owner, 'Grupo Cocó Trail & Natureza', 'Grupo dedicado a corridas nas trilhas do Parque do Cocó.', '#2E8F63')
    ON CONFLICT (id) DO NOTHING;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS sos_events;
DROP TABLE IF EXISTS safety_contacts;
DROP TABLE IF EXISTS kudos;
DROP TABLE IF EXISTS activity_events;
DROP TABLE IF EXISTS club_members;
DROP TABLE IF EXISTS clubs;
DROP TABLE IF EXISTS friendships;
DROP TYPE IF EXISTS activity_type;
DROP TYPE IF EXISTS friend_status;
