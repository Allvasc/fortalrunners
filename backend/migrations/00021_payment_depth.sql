-- +goose Up
-- Profundidade de pagamento (plano §9, §15): cupons, reembolso, assinatura premium.

CREATE TABLE coupons (
    id            text PRIMARY KEY,
    code          text NOT NULL,
    scope         text NOT NULL DEFAULT 'event',   -- event | global | campaign
    event_id      text REFERENCES events (id) ON DELETE CASCADE,
    discount_type text NOT NULL DEFAULT 'percent',  -- percent | fixed
    value         int NOT NULL,                     -- % (1-100) ou centavos
    max_uses      int NOT NULL DEFAULT 0,           -- 0 = ilimitado
    used          int NOT NULL DEFAULT 0,
    valid_until   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (code)
);

CREATE TABLE refunds (
    id              text PRIMARY KEY,
    payment_id      text NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    order_id        text NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    asaas_refund_id text,
    amount_cents    int NOT NULL,
    reason          text,
    status          text NOT NULL DEFAULT 'requested', -- requested | approved | done | denied
    requested_by    text REFERENCES users (id) ON DELETE SET NULL,
    handled_by      text REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    handled_at      timestamptz
);
CREATE INDEX refunds_status_idx ON refunds (status);

CREATE TABLE subscriptions (
    user_id               text PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    asaas_subscription_id text,
    plan                  text NOT NULL DEFAULT 'premium_monthly',
    status                text NOT NULL DEFAULT 'active',  -- active | past_due | canceled
    current_period_end    timestamptz,
    cancel_at             timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE orders ADD COLUMN coupon_id text REFERENCES coupons (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE orders DROP COLUMN IF EXISTS coupon_id;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS coupons;
