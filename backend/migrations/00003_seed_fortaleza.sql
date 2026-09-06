-- +goose Up
-- Seed simplificado de bairros de Fortaleza (polígonos aproximados).
-- Limites reais serão importados do IBGE/prefeitura antes do lançamento.

INSERT INTO neighborhoods (id, name, geom) VALUES
  ('nbh_centro', 'Centro',
   ST_Multi(ST_GeomFromText('POLYGON((-38.535 -3.720, -38.515 -3.720, -38.515 -3.736, -38.535 -3.736, -38.535 -3.720))', 4326))),
  ('nbh_praia_iracema', 'Praia de Iracema',
   ST_Multi(ST_GeomFromText('POLYGON((-38.525 -3.715, -38.505 -3.715, -38.505 -3.727, -38.525 -3.727, -38.525 -3.715))', 4326))),
  ('nbh_meireles', 'Meireles',
   ST_Multi(ST_GeomFromText('POLYGON((-38.502 -3.718, -38.480 -3.718, -38.480 -3.731, -38.502 -3.731, -38.502 -3.718))', 4326))),
  ('nbh_aldeota', 'Aldeota',
   ST_Multi(ST_GeomFromText('POLYGON((-38.507 -3.726, -38.485 -3.726, -38.485 -3.746, -38.507 -3.746, -38.507 -3.726))', 4326))),
  ('nbh_mucuripe', 'Mucuripe',
   ST_Multi(ST_GeomFromText('POLYGON((-38.480 -3.708, -38.455 -3.708, -38.455 -3.726, -38.480 -3.726, -38.480 -3.708))', 4326)));

-- +goose Down
DELETE FROM neighborhoods WHERE id IN
  ('nbh_centro', 'nbh_praia_iracema', 'nbh_meireles', 'nbh_aldeota', 'nbh_mucuripe');
