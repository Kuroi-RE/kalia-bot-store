-- Payment events: idempotency + audit log of gateway notifications.
CREATE TABLE payment_events (
    id              BIGSERIAL PRIMARY KEY,
    order_ref       TEXT NOT NULL,
    gateway_txn_id  TEXT NOT NULL DEFAULT '',
    event_status    TEXT NOT NULL,        -- transaction_status from the notification
    status_code     TEXT NOT NULL DEFAULT '',
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    payload         JSONB,
    processed       BOOLEAN NOT NULL DEFAULT FALSE,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Dedupe retried webhooks for the same (transaction, status).
    UNIQUE (gateway_txn_id, event_status)
);

CREATE INDEX idx_payment_events_order_ref ON payment_events (order_ref);
