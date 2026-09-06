-- +goose Up
-- Marcos Históricos, Selos, Rotas Comunitárias e Pontos de Apoio (plano §3, §5, §6 / banco §5, §6, §10).

CREATE TYPE checkin_status AS ENUM ('pending', 'approved', 'rejected');
CREATE TYPE mod_status     AS ENUM ('pending', 'approved', 'hidden', 'removed');

-- Coleções de Marcos
CREATE TABLE landmark_collections (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    description text,
    badge_code  text REFERENCES badges (code) ON DELETE SET NULL,
    reward_xp   int NOT NULL DEFAULT 100,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Marcos Históricos / Turísticos
CREATE TABLE landmarks (
    id               text PRIMARY KEY,
    collection_id    text REFERENCES landmark_collections (id) ON DELETE SET NULL,
    name             text NOT NULL,
    badge_code       text REFERENCES badges (code) ON DELETE SET NULL,
    geom             geometry(Point, 4326) NOT NULL,
    radius_m         int NOT NULL DEFAULT 40,
    blurb            text,
    hero_photo_key   text,
    difficulty       int NOT NULL DEFAULT 1,
    requires_on_foot bool NOT NULL DEFAULT true,
    quiz_jsonb       jsonb NOT NULL DEFAULT '{}'::jsonb,
    active_from      timestamptz,
    active_to        timestamptz,
    status           text NOT NULL DEFAULT 'active',
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX landmarks_geom_idx ON landmarks USING GIST (geom);
CREATE INDEX landmarks_collection_idx ON landmarks (collection_id);

-- Check-ins nos Marcos
CREATE TABLE landmark_checkins (
    id               text PRIMARY KEY,
    user_id          text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    landmark_id      text NOT NULL REFERENCES landmarks (id) ON DELETE CASCADE,
    run_id           text REFERENCES runs (id) ON DELETE SET NULL,
    photo_key        text,
    geom             geometry(Point, 4326),
    taken_at         timestamptz NOT NULL DEFAULT now(),
    sensor_sig_jsonb jsonb NOT NULL DEFAULT '{}'::jsonb,
    quiz_ok          bool NOT NULL DEFAULT true,
    status           checkin_status NOT NULL DEFAULT 'approved',
    moderated_by     text REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, landmark_id)
);
CREATE INDEX landmark_checkins_user_idx ON landmark_checkins (user_id);
CREATE INDEX landmark_checkins_landmark_idx ON landmark_checkins (landmark_id);

-- Rotas Comunitárias
CREATE TABLE routes (
    id          text PRIMARY KEY,
    created_by  text REFERENCES users (id) ON DELETE SET NULL,
    name        text NOT NULL,
    description text,
    distance_m  int NOT NULL DEFAULT 0,
    surface     text NOT NULL DEFAULT 'asfalto', -- asfalto, calçadão, trilha
    is_official bool NOT NULL DEFAULT false,
    geom        geometry(LineString, 4326) NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX routes_geom_idx ON routes USING GIST (geom);

-- Avaliações de Rotas
CREATE TABLE route_reviews (
    id         text PRIMARY KEY,
    route_id   text NOT NULL REFERENCES routes (id) ON DELETE CASCADE,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rating     smallint NOT NULL CHECK (rating >= 1 AND rating <= 5),
    tags_jsonb jsonb NOT NULL DEFAULT '[]'::jsonb,
    body       text,
    status     mod_status NOT NULL DEFAULT 'approved',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (route_id, user_id)
);
CREATE INDEX route_reviews_route_idx ON route_reviews (route_id);

-- Pontos de Apoio Urbanos (Bebedouros, Banheiros, etc.)
CREATE TABLE map_pois (
    id         text PRIMARY KEY,
    city_id    text NOT NULL DEFAULT 'fortaleza',
    name       text NOT NULL,
    category   text NOT NULL, -- bebedouro, banheiro, hidratacao, emergencia, sombra
    geom       geometry(Point, 4326) NOT NULL,
    note       text,
    status     text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX map_pois_geom_idx ON map_pois USING GIST (geom);
CREATE INDEX map_pois_city_cat_idx ON map_pois (city_id, category);

-- --- SEED INICIAL ---

-- Coleção de Fortaleza
INSERT INTO landmark_collections (id, name, description, reward_xp) VALUES
    ('col_fortaleza_orla', 'Circuito Orla de Fortaleza', 'Marcos emblemáticos do litoral de Fortaleza.', 250),
    ('col_fortaleza_historico', 'Circuito Histórico e Cultural', 'Principais cartões-postais culturais do Centro e Aldeota.', 300);

-- Badges dos Marcos
INSERT INTO badges (code, name, description, shape, type) VALUES
    ('badge_farol_mucuripe', 'Farol do Mucuripe', 'Visitou o histórico Farol do Mucuripe', 'shield', 'landmark'),
    ('badge_iracema_guardia', 'Iracema Guardiã', 'Chegou à estátua de Iracema no aterro', 'shield', 'landmark'),
    ('badge_mercado_central', 'Mercado Central', 'Passou pelo Mercado Central no Centro', 'shield', 'landmark'),
    ('badge_parque_coco', 'Pulmão Verde do Cocó', 'Correu no Parque Ecológico do Cocó', 'shield', 'landmark'),
    ('badge_dragao_do_mar', 'Dragão do Mar', 'Visitou o Centro Cultural Dragão do Mar', 'shield', 'landmark'),
    ('badge_ponte_ingleses', 'Ponte dos Ingleses', 'Chegou ao icônico píer da Praia de Iracema', 'shield', 'landmark'),
    ('badge_catedral', 'Catedral Metropolitana', 'Visitou a Catedral no Centro Histórico', 'shield', 'landmark'),
    ('badge_passeio_publico', 'Passeio Público', 'Passou pela praça mais antiga de Fortaleza', 'shield', 'landmark'),
    ('badge_luiza_tavora', 'Praça Luíza Távora', 'Correu na Praça da CEART na Aldeota', 'shield', 'landmark'),
    ('badge_feirinha_beiramar', 'Feirinha da Beira-Mar', 'Visitou o coração do Meireles', 'shield', 'landmark'),
    ('badge_espigao_nautico', 'Espigão do Náutico', 'Foi até a ponta do Espigão no Meireles', 'shield', 'landmark'),
    ('badge_rachel_queiroz', 'Parque Rachel de Queiroz', 'Conquistou o parque no Presidente Kennedy', 'shield', 'landmark'),
    ('badge_arena_castelao', 'Arena Castelão', 'Correu ao redor do templo do futebol cearense', 'shield', 'landmark'),
    ('badge_jose_alencar', 'Casa José de Alencar', 'Passou pelo sítio histórico em Messejana', 'shield', 'landmark'),
    ('badge_mercado_peixes', 'Mercado dos Peixes', 'Chegou à enseada do Mucuripe', 'shield', 'landmark')
ON CONFLICT (code) DO NOTHING;

-- Seed de 15 Marcos de Fortaleza (WGS84)
INSERT INTO landmarks (id, collection_id, name, badge_code, geom, radius_m, blurb) VALUES
    ('lmk_farol_mucuripe', 'col_fortaleza_orla', 'Farol do Mucuripe', 'badge_farol_mucuripe', ST_SetSRID(ST_Point(-38.4728, -3.7198), 4326), 50, 'Histórico farol de 1846 no Mucuripe.'),
    ('lmk_iracema_guardia', 'col_fortaleza_orla', 'Estátua de Iracema Guardiã', 'badge_iracema_guardia', ST_SetSRID(ST_Point(-38.5133, -3.7197), 4326), 40, 'Escultura icônica no Aterro da Praia de Iracema.'),
    ('lmk_mercado_central', 'col_fortaleza_historico', 'Mercado Central de Fortaleza', 'badge_mercado_central', ST_SetSRID(ST_Point(-38.5245, -3.7225), 4326), 50, 'Maior mercado de artesanato das Américas.'),
    ('lmk_parque_coco', 'col_fortaleza_historico', 'Parque Ecológico do Cocó', 'badge_parque_coco', ST_SetSRID(ST_Point(-38.4812, -3.7483), 4326), 60, 'Maior parque ecológico urbano do Norte/Nordeste.'),
    ('lmk_dragao_do_mar', 'col_fortaleza_historico', 'Centro Dragão do Mar', 'badge_dragao_do_mar', ST_SetSRID(ST_Point(-38.5218, -3.7212), 4326), 50, 'Complexo cultural e arquitetônico de Fortaleza.'),
    ('lmk_ponte_ingleses', 'col_fortaleza_orla', 'Ponte dos Ingleses', 'badge_ponte_ingleses', ST_SetSRID(ST_Point(-38.5186, -3.7169), 4326), 40, 'Famoso píer e cartão-postal do pôr do sol.'),
    ('lmk_catedral', 'col_fortaleza_historico', 'Catedral Metropolitana', 'badge_catedral', ST_SetSRID(ST_Point(-38.5258, -3.7250), 4326), 50, 'Terceira maior catedral do Brasil.'),
    ('lmk_passeio_publico', 'col_fortaleza_historico', 'Passeio Público', 'badge_passeio_publico', ST_SetSRID(ST_Point(-38.5283, -3.7214), 4326), 40, 'Praça dos Mártires, a mais antiga da cidade.'),
    ('lmk_luiza_tavora', 'col_fortaleza_historico', 'Praça Luíza Távora (CEART)', 'badge_luiza_tavora', ST_SetSRID(ST_Point(-38.5082, -3.7335), 4326), 40, 'Espaço verde e cultural na Aldeota.'),
    ('lmk_feirinha_beiramar', 'col_fortaleza_orla', 'Feirinha da Beira-Mar', 'badge_feirinha_beiramar', ST_SetSRID(ST_Point(-38.4985, -3.7251), 4326), 40, 'Tradicional feira de artesanato no Meireles.'),
    ('lmk_espigao_nautico', 'col_fortaleza_orla', 'Espigão do Náutico', 'badge_espigao_nautico', ST_SetSRID(ST_Point(-38.4942, -3.7241), 4326), 40, 'Passarela de maradentro no Meireles.'),
    ('lmk_rachel_queiroz', NULL, 'Parque Rachel de Queiroz', 'badge_rachel_queiroz', ST_SetSRID(ST_Point(-38.5681, -3.7289), 4326), 60, 'Parque linear e lagoas na Zona Oeste.'),
    ('lmk_arena_castelao', NULL, 'Arena Castelão', 'badge_arena_castelao', ST_SetSRID(ST_Point(-38.5269, -3.8073), 4326), 80, 'Estádio Governador Plácido Aderaldo Castelo.'),
    ('lmk_jose_alencar', NULL, 'Casa de José de Alencar', 'badge_jose_alencar', ST_SetSRID(ST_Point(-38.4891, -3.8315), 4326), 50, 'Sítio histórico tombado pelo IPHAN.'),
    ('lmk_mercado_peixes', 'col_fortaleza_orla', 'Mercado dos Peixes', 'badge_mercado_peixes', ST_SetSRID(ST_Point(-38.4775, -3.7188), 4326), 40, 'Gastronomia marítima na ponta do Mucuripe.');

-- Seed de 2 Rotas Oficiais de Fortaleza
INSERT INTO routes (id, name, description, distance_m, surface, is_official, geom) VALUES
    ('rte_beiramar_5k', 'Circuito Beira-Mar 5k', 'Percurso clássico do Aterro de Iracema até o Espigão do Náutico.', 5000, 'calçadão', true,
     ST_SetSRID(ST_GeomFromText('LINESTRING(-38.5133 -3.7197, -38.5085 -3.7220, -38.4985 -3.7251, -38.4942 -3.7241)'), 4326)),
    ('rte_coco_7k', 'Circuito Trilha do Cocó 7k', 'Volta completa pelas trilhas sombreadas do Parque do Cocó.', 7000, 'trilha', true,
     ST_SetSRID(ST_GeomFromText('LINESTRING(-38.4812 -3.7483, -38.4850 -3.7520, -38.4790 -3.7560, -38.4750 -3.7500, -38.4812 -3.7483)'), 4326));

-- Seed de Pontos de Apoio (Bebedouros & Banheiros)
INSERT INTO map_pois (id, name, category, geom, note) VALUES
    ('poi_bebedouro_aterro', 'Bebedouro Público Aterro', 'bebedouro', ST_SetSRID(ST_Point(-38.5125, -3.7201), 4326), 'Bebedouro gelado próximo ao posto da Guarda.'),
    ('poi_banheiro_nautico', 'Banheiros do Náutico', 'banheiro', ST_SetSRID(ST_Point(-38.4948, -3.7245), 4326), 'Banheiros públicos abertos das 5h às 22h.'),
    ('poi_bebedouro_coco', 'Bebedouro Trilha Principal Cocó', 'bebedouro', ST_SetSRID(ST_Point(-38.4815, -3.7485), 4326), 'Ponto de hidratação na entrada das trilhas.'),
    ('poi_banheiro_feirinha', 'Banheiros da Feirinha', 'banheiro', ST_SetSRID(ST_Point(-38.4988, -3.7255), 4326), 'Posto com acessibilidade e fraldário.');

-- +goose Down
DROP TABLE IF EXISTS map_pois;
DROP TABLE IF EXISTS route_reviews;
DROP TABLE IF EXISTS routes;
DROP TABLE IF EXISTS landmark_checkins;
DROP TABLE IF EXISTS landmarks;
DROP TABLE IF EXISTS landmark_collections;
DROP TYPE IF EXISTS mod_status;
DROP TYPE IF EXISTS checkin_status;
