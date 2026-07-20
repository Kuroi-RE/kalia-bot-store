-- Accounts: inventory units belonging to a product.
CREATE TABLE accounts (
    id                BIGSERIAL PRIMARY KEY,
    product_id        BIGINT NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    -- Structured per the product's credential_schema.
    credentials       JSONB NOT NULL DEFAULT '{}'::jsonb,
    status            account_status NOT NULL DEFAULT 'AVAILABLE',
    -- Reservation linkage. FK to orders is added by the orders migration
    -- (orders does not exist yet at this point).
    reserved_order_id BIGINT,
    reserved_until    TIMESTAMPTZ,
    sold_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast allocation queries: find AVAILABLE accounts for a product.
CREATE INDEX idx_accounts_product_status ON accounts (product_id, status);
CREATE INDEX idx_accounts_reserved_order ON accounts (reserved_order_id);
