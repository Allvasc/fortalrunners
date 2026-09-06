-- +goose Up
-- Mapa de calor: passagem agregada por célula (banco-e-integracoes.html §heat_agg).
-- "Onde se corre", não "o que se domina" — nunca traçados individuais.
--
-- Fase 1: só o próprio corredor (scope 'user:<id>', period 'all', activity 'run').
-- Amigos/clube/cidade + k-anonimato = Fase 2/3.
--
-- H3 está adiado (é cgo — ver 00005): h3_index aqui é um STAND-IN decodificável —
-- latIdx*1e7 + lonIdx numa grade de ~50 m — e cell_geom guarda o centro da célula
-- para o filtro por bbox. Quando o H3 entrar, h3_index vira o índice real (res
-- 10–11) e cell_geom passa a ser derivável dele; hits/weight/runner_count ficam.

CREATE TABLE heat_agg (
    h3_index     bigint       NOT NULL,           -- stand-in de grade até o H3
    scope        text         NOT NULL,           -- user:<id> | club:<id> | city
    period       text         NOT NULL DEFAULT 'all',   -- all | 2026-w36 | 2026-09
    activity     text         NOT NULL DEFAULT 'run',   -- run | walk | trail
    hits         int          NOT NULL DEFAULT 0,       -- pontos de GPS na célula
    weight       numeric(6,4) NOT NULL DEFAULT 0,       -- densidade normalizada 0..1
    runner_count int          NOT NULL DEFAULT 1,       -- k-anonimato (cidade: servir só com >= N)
    cell_geom    geometry(Point, 4326) NOT NULL,
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (h3_index, scope, period, activity)
);
CREATE INDEX heat_agg_scope_idx ON heat_agg (scope, period, activity);
CREATE INDEX heat_agg_geom_gix  ON heat_agg USING gist (cell_geom);

-- +goose Down
DROP TABLE IF EXISTS heat_agg;
