-- +goose Up
-- Semeia os clubes oficiais de Fortaleza.
--
-- O bloco de seed da 00015 escolhia `SELECT id FROM users LIMIT 1` como dono e
-- fazia RETURN quando a tabela estava vazia. Em produção as migrations rodaram
-- num banco sem usuários, então nenhum clube foi criado. Aqui criamos uma conta
-- de sistema dedicada (sem senha, sem login) para ser dona do conteúdo oficial e
-- semeamos os clubes de forma idempotente.

INSERT INTO users (id, athlete_id, username, email, display_name, color_hex, role, status)
VALUES (
    'usr_fr_sistema',
    'FR-0000000',
    'fortalrunners',
    'sistema@fortalrunners.app',
    'FortalRunners',
    '#0E7C86',
    'runner',
    'active'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO clubs (id, owner_id, name, description, color_hex)
VALUES
    ('clb_beiramar_runners', 'usr_fr_sistema', 'Beira-Mar Runners',
     'Clube oficial de corredores do calçadão da Beira-Mar.', '#0E7C86'),
    ('clb_coco_trail', 'usr_fr_sistema', 'Grupo Cocó Trail & Natureza',
     'Grupo dedicado a corridas nas trilhas do Parque do Cocó.', '#2E8F63')
ON CONFLICT (id) DO NOTHING;

INSERT INTO club_members (club_id, user_id, role)
VALUES
    ('clb_beiramar_runners', 'usr_fr_sistema', 'owner'),
    ('clb_coco_trail', 'usr_fr_sistema', 'owner')
ON CONFLICT (club_id, user_id) DO NOTHING;

-- +goose Down
DELETE FROM club_members WHERE club_id IN ('clb_beiramar_runners', 'clb_coco_trail');
DELETE FROM clubs WHERE id IN ('clb_beiramar_runners', 'clb_coco_trail');
DELETE FROM users WHERE id = 'usr_fr_sistema';
