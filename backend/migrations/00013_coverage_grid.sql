-- +goose Up
-- Cobertura "% de Fortaleza" (plano §2/§3). H3 real está adiado (cgo) → usamos a
-- mesma grade stand-in do heatmap, aqui a ~60 m (entre res 9 e 10). h3_cells
-- guarda 1 linha por (célula, corredor); neighborhoods.h3_total é o denominador.
--
-- h3_index = gy*1e7 + (gx + 5e6), com gx/gy = índices inteiros da grade de 0.00055°.

UPDATE neighborhoods n SET h3_total = COALESCE((
    SELECT count(*)
    FROM generate_series(floor(ST_XMin(n.geom) / 0.00055)::int, ceil(ST_XMax(n.geom) / 0.00055)::int) gx
    CROSS JOIN generate_series(floor(ST_YMin(n.geom) / 0.00055)::int, ceil(ST_YMax(n.geom) / 0.00055)::int) gy
    WHERE ST_Contains(n.geom, ST_SetSRID(ST_Point((gx + 0.5) * 0.00055, (gy + 0.5) * 0.00055), 4326))
), 0);

-- +goose Down
UPDATE neighborhoods SET h3_total = 0;
