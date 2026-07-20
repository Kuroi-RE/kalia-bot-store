-- Deliveries: one credential dispatch per paid order (exactly-once).
CREATE TABLE deliveries (
    id           BIGSERIAL PRIMARY KEY,
    -- UNIQUE guarantees credentials are delivered at most once per order.
    order_id     BIGINT NOT NULL UNIQUE REFERENCES orders (id) ON DELETE CASCADE,
    account_id   BIGINT NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
    status       delivery_status NOT NULL DEFAULT 'PENDING',
    attempts     INTEGER NOT NULL DEFAULT 0,
    last_error   TEXT NOT NULL DEFAULT '',
    delivered_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deliveries_status ON deliveries (status);
