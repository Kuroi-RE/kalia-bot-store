-- Enums for the domain state machines.
CREATE TYPE account_status AS ENUM ('AVAILABLE', 'RESERVED', 'SOLD');
CREATE TYPE order_status AS ENUM ('PENDING', 'PAID', 'DELIVERED', 'EXPIRED', 'CANCELLED', 'FAILED');
CREATE TYPE payment_status AS ENUM ('PENDING', 'SETTLEMENT', 'EXPIRED', 'DENIED', 'CANCELLED');
CREATE TYPE delivery_status AS ENUM ('PENDING', 'DELIVERED', 'FAILED');

-- Admins: authenticate the dashboard / admin API.
CREATE TABLE admins (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
