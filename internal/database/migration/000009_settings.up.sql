-- Settings: runtime key/value configuration tunable without redeploys.
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO settings (key, value) VALUES
    ('default_acquirer', 'gopay'),
    ('payment_ttl_seconds', '900'),
    ('reservation_ttl_seconds', '900'),
    ('delivery_message_template', '')
ON CONFLICT (key) DO NOTHING;
