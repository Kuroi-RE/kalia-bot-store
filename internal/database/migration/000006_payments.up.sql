-- Payments: one active payment attempt per order.
CREATE TABLE payments (
    id                   BIGSERIAL PRIMARY KEY,
    order_id             BIGINT NOT NULL UNIQUE REFERENCES orders (id) ON DELETE CASCADE,
    gateway              TEXT NOT NULL DEFAULT 'midtrans',
    gateway_txn_id       TEXT NOT NULL DEFAULT '',
    status               payment_status NOT NULL DEFAULT 'PENDING',
    gross_amount         BIGINT NOT NULL CHECK (gross_amount >= 0),
    acquirer             TEXT NOT NULL DEFAULT '',
    qr_string            TEXT NOT NULL DEFAULT '',
    qr_image_url         TEXT NOT NULL DEFAULT '',
    expires_at           TIMESTAMPTZ,
    raw_charge_response  JSONB,
    settled_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_status ON payments (status);
CREATE INDEX idx_payments_gateway_txn ON payments (gateway_txn_id);
