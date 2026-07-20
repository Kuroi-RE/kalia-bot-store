-- Telegram customers placing orders (bot has no DB access; backend records them).
CREATE TABLE telegram_users (
    id          BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    username    TEXT NOT NULL DEFAULT '',
    first_name  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Orders: the central transaction record.
CREATE TABLE orders (
    id               BIGSERIAL PRIMARY KEY,
    -- Human/gateway-safe id (<=30 chars) used as the Midtrans order_id.
    order_ref        TEXT NOT NULL UNIQUE,
    telegram_user_id BIGINT NOT NULL REFERENCES telegram_users (id) ON DELETE RESTRICT,
    product_id       BIGINT NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    account_id       BIGINT REFERENCES accounts (id) ON DELETE SET NULL,
    amount           BIGINT NOT NULL CHECK (amount >= 0),
    status           order_status NOT NULL DEFAULT 'PENDING',
    expires_at       TIMESTAMPTZ,
    paid_at          TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_telegram_user ON orders (telegram_user_id);
CREATE INDEX idx_orders_expires_at ON orders (expires_at) WHERE status = 'PENDING';

-- Clear any stale reservations before wiring the FK (no orders exist yet, so
-- any reserved_order_id is orphaned pre-launch/test data).
UPDATE accounts
SET status = 'AVAILABLE', reserved_order_id = NULL, reserved_until = NULL
WHERE reserved_order_id IS NOT NULL;

ALTER TABLE accounts
    ADD CONSTRAINT fk_accounts_reserved_order
    FOREIGN KEY (reserved_order_id) REFERENCES orders (id) ON DELETE SET NULL;
