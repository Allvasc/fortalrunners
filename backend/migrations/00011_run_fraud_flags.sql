-- +goose Up
-- Anti-fraude v1: guarda os sinais que sinalizaram a corrida, para o revisor
-- do admin ver "por quê" (plano §8 — decisão punitiva sempre com apelação humana).
ALTER TABLE runs ADD COLUMN fraud_flags jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE runs DROP COLUMN IF EXISTS fraud_flags;
