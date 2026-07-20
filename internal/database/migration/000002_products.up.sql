-- Products: sellable catalog items.
CREATE TABLE products (
    id                BIGSERIAL PRIMARY KEY,
    name              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    base_price        BIGINT NOT NULL CHECK (base_price >= 0), -- IDR, minor-unit safe
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    -- Field template describing which credential fields this product uses,
    -- e.g. [{"key":"email","label":"Email","type":"string","required":true}].
    credential_schema JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_is_active ON products (is_active);
