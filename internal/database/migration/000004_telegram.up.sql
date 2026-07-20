-- Dynamic Telegram command menus (can trigger flows like /order).
CREATE TABLE telegram_menus (
    id         BIGSERIAL PRIMARY KEY,
    command    TEXT NOT NULL UNIQUE,
    title      TEXT NOT NULL DEFAULT '',
    reply_text TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Static command replies (text-only, e.g. /testimoni).
CREATE TABLE telegram_responses (
    id         BIGSERIAL PRIMARY KEY,
    command    TEXT NOT NULL UNIQUE,
    reply_text TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_telegram_menus_enabled ON telegram_menus (is_enabled, sort_order);
CREATE INDEX idx_telegram_responses_enabled ON telegram_responses (is_enabled);
