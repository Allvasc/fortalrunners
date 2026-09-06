-- +goose Up
-- H3 real (build tag "h3"): o índice de célula passa de bigint (grade stand-in)
-- para text — acomoda tanto o índice H3 em hex quanto a chave da grade de
-- fallback ("g:<gx>:<gy>"). h3_cells é repovoado no boot quando a grade muda
-- (territory.EnsureCoverageGrid), então não há perda real de cobertura.
ALTER TABLE h3_cells ALTER COLUMN h3_index TYPE text USING h3_index::text;

-- marcador da grade ativa: EnsureCoverageGrid compara com h3grid.Coverage.Tag()
INSERT INTO game_config (key, value_jsonb) VALUES ('coverage_grid_tag', '""')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM game_config WHERE key = 'coverage_grid_tag';
-- volta a bigint só é possível com a grade stand-in (valores numéricos);
-- com índices H3 em hex isto falha de propósito — reverta o código antes.
ALTER TABLE h3_cells ALTER COLUMN h3_index TYPE bigint USING h3_index::bigint;
