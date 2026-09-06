-- +goose Up
-- Integridade de pagamento: amarra o pedido ao evento/lote, guarda o status bruto
-- do PSP separado do enum, e corrige o modelo default de IA (plano §16: Claude é o padrão).

ALTER TABLE orders ADD COLUMN event_id text REFERENCES events (id) ON DELETE SET NULL;
ALTER TABLE orders ADD COLUMN price_id text REFERENCES event_prices (id) ON DELETE SET NULL;
ALTER TABLE orders ADD COLUMN platform_fee_cents int NOT NULL DEFAULT 0;

ALTER TABLE payments ADD COLUMN provider text NOT NULL DEFAULT 'manual';
ALTER TABLE payments ADD COLUMN raw_status text;

ALTER TABLE ai_interactions ALTER COLUMN model SET DEFAULT 'claude-sonnet-5';

-- +goose Down
ALTER TABLE ai_interactions ALTER COLUMN model SET DEFAULT 'gemini-flash-2.0';
ALTER TABLE payments DROP COLUMN IF EXISTS raw_status;
ALTER TABLE payments DROP COLUMN IF EXISTS provider;
ALTER TABLE orders DROP COLUMN IF EXISTS platform_fee_cents;
ALTER TABLE orders DROP COLUMN IF EXISTS price_id;
ALTER TABLE orders DROP COLUMN IF EXISTS event_id;
