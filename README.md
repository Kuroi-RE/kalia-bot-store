# Kalia Store — Telegram Digital Account Store (Backend)

Production-ready Go backend that sells digital accounts (Netflix, Disney+, ChatGPT Plus, etc.)
through a Telegram bot, with QRIS payments via **Midtrans** and **automatic delivery** of
account credentials once payment settles.

The Go/Fiber backend is the single source of truth. The Telegram bot and (future) React admin
dashboard interact **only through REST APIs** and never touch the database directly.

## Core guarantees

- **No double-selling:** inventory is allocated with `SELECT ... FOR UPDATE SKIP LOCKED` inside
  the order transaction, so two customers can never be sold the same account.
- **Idempotent payments:** webhook notifications are deduped on `(gateway_txn_id, event_status)`;
  our `order_ref` is reused as the Midtrans `order_id`; state transitions are forward-only.
- **Exactly-once delivery:** a unique `deliveries.order_id` row plus a delivered-status guard
  ensure credentials are dispatched at most once, even under duplicate settlement webhooks.
- **Webhook-first + polling reconciliation:** instant delivery via webhook, with a poller as a
  safety-net for missed notifications.

## Architecture

```
Telegram Bot ─┐                      ┌─ PostgreSQL
              ├─REST→ Fiber API ─────┤
Admin Dash ───┘         │            └─ Midtrans QRIS
                        │  (settlement webhook / status polling)
              Background Worker (poller, cleanup, delivery retry)
```

Layering: **Handler → Service → Repository (pgx)**. Handlers do transport + validation;
services hold business rules and transactions; repositories do data access.

Two binaries:
- `cmd/api` — the Fiber HTTP server (admin + bot + webhook). Runs migrations on startup.
- `cmd/worker` — background jobs (payment reconciler, expiry cleanup, delivery retry).

## Order state machine

```
PENDING ──settlement──▶ PAID ──delivered──▶ DELIVERED
   │                     │
   │                     └─delivery error─▶ FAILED ──retry──▶ DELIVERED
   ├─expire / TTL──▶ EXPIRED
   └─cancel / deny─▶ CANCELLED
```

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- A Midtrans account (sandbox is fine) for a Server Key

## Configuration (environment variables)

Copy `.env.example` to `.env` and fill in the values.

| Variable | Required | Default | Description |
|---|---|---|---|
| `APP_ENV` | no | `development` | `development` or `production` |
| `HTTP_PORT` | no | `8080` | API listen port |
| `LOG_LEVEL` | no | `info` | `debug`/`info`/`warn`/`error` |
| `DATABASE_URL` | **yes** | – | Postgres DSN (`postgres://user:pass@host:5432/db?sslmode=disable`) |
| `JWT_SECRET` | **yes** | – | Secret for signing admin JWTs (use a long random string) |
| `JWT_TTL` | no | `24h` | Access-token lifetime |
| `ADMIN_USERNAME` | no | – | Seeded on first startup if the admin table is empty |
| `ADMIN_PASSWORD` | no | – | Seeded on first startup if the admin table is empty |
| `BOT_SERVICE_TOKEN` | **yes** | – | Static token the bot presents on `/bot/*` endpoints |
| `TELEGRAM_BOT_TOKEN` | no | – | Bot token for backend→Telegram credential delivery |
| `RESERVATION_TTL` | no | `15m` | How long an account stays RESERVED |
| `PAYMENT_TTL` | no | `15m` | QRIS charge validity |
| `MIDTRANS_SERVER_KEY` | **yes** (prod) | – | Midtrans Core API Server Key |
| `MIDTRANS_BASE_URL` | no | `https://api.sandbox.midtrans.com` | Use `https://api.midtrans.com` in production |
| `MIDTRANS_ACQUIRER` | no | `gopay` | `gopay` or `shopeepay` |
| `WORKER_POLL_INTERVAL` | no | `90s` | Reconciler cadence |
| `WORKER_CLEANUP_INTERVAL` | no | `2m` | Expiry/reservation cleanup cadence |
| `WORKER_DELIVERY_RETRY_INTERVAL` | no | `3m` | Failed-delivery retry cadence |
| `WORKER_POLL_MIN_AGE` | no | `2m` | Only reconcile PENDING orders older than this |
| `WORKER_MAX_DELIVERY_ATTEMPTS` | no | `5` | Stop retrying after this many attempts |
| `WORKER_BATCH_LIMIT` | no | `100` | Rows processed per job tick |

Durations accept Go duration strings (`15m`, `90s`) or plain seconds (`900`).

> Secrets (`JWT_SECRET`, `MIDTRANS_SERVER_KEY`, `BOT_SERVICE_TOKEN`, `TELEGRAM_BOT_TOKEN`) must
> come from the environment and never be committed. `.env` is gitignored.

## Running with Docker Compose

```bash
cp .env.example .env         # then edit secrets
docker compose -f deployments/docker-compose.yml up -d --build
```

This starts `postgres`, `api`, `worker`, and `nginx`. Only nginx is published (ports 80/443).
The API applies database migrations automatically on startup.

Check health through nginx:

```bash
curl http://localhost/health/ready
```

### Enabling HTTPS (required for Midtrans webhooks)

Midtrans only calls public HTTPS endpoints. Mount TLS certs into nginx and uncomment the
`server { listen 443 ssl; ... }` block in `deployments/nginx/nginx.conf`, then point your domain
at the host.

### Midtrans webhook setup

In the Midtrans dashboard, set the **Payment Notification URL** to:

```
https://your-domain.com/webhooks/midtrans
```

The endpoint is public but every notification is signature-verified
(`SHA512(order_id + status_code + gross_amount + ServerKey)`); invalid signatures are rejected
with 401. Duplicate/retried notifications are deduped and acknowledged with 200.

## Running locally without Docker

```bash
# start just Postgres
docker compose -f deployments/docker-compose.yml up -d postgres   # exposed on localhost:5433

# point DATABASE_URL at it in .env, then:
go run ./cmd/api       # serves http://localhost:8080 (runs migrations)
go run ./cmd/worker    # background jobs
```

## Seeding demo data

The initial admin is auto-seeded from `ADMIN_USERNAME`/`ADMIN_PASSWORD`. For sample catalog
data you can apply `deployments/seed.sql`:

```bash
docker compose -f deployments/docker-compose.yml exec -T postgres \
  psql -U kalia -d kalia_store < deployments/seed.sql
```

## API overview

Base path `/api/v1`. Admin routes require `Authorization: Bearer <jwt>`; bot routes under
`/bot/*` require the bot service token (`X-Bot-Token` header or bearer).

- **Auth:** `POST /auth/login`, `GET /auth/me`
- **Products (admin):** `GET/POST /products`, `GET/PUT/DELETE /products/:id`, `PATCH /products/:id/status`
- **Inventory (admin):** `GET/POST /products/:id/accounts`, `GET /products/:id/inventory-summary`,
  `GET/PUT/DELETE /accounts/:id`
- **Telegram content (admin):** `GET/POST/PUT/DELETE /telegram/menus` (+`/status`), same for `/telegram/responses`
- **Orders/Payments (admin):** `GET /orders`, `GET /orders/:id`, `PATCH /orders/:id/cancel`,
  `GET /orders/:id/payment`, `GET /payments/:id`
- **Deliveries (admin):** `GET /deliveries`, `POST /orders/:id/redeliver`
- **Settings (admin):** `GET /settings`, `GET/PUT /settings/:key`
- **Bot:** `GET /bot/products`, `POST /bot/orders`, `GET /bot/orders/:order_ref`,
  `GET /bot/menus`, `GET /bot/responses/:command`
- **Webhook (public):** `POST /webhooks/midtrans`
- **Health:** `GET /health/live`, `GET /health/ready`

## Testing

The repository ships standard Go tests (`internal/app/*_test.go`, `pkg/**/*_test.go`). Where a
live database is needed they are gated on `KALIA_TEST_DB` and skipped otherwise.

An additional in-process integration harness lives at `cmd/verify` and exercises the full HTTP
flow (auth → catalog → order → charge → settlement → delivery, plus worker jobs and hardening)
against a real database using a fake payment gateway and sender:

```bash
docker compose -f deployments/docker-compose.yml up -d postgres
export KALIA_TEST_DB='postgres://kalia:kalia@localhost:5433/kalia_store?sslmode=disable'
go run ./cmd/verify
```

## Project layout

```
cmd/
  api/       API server entrypoint (Fiber bootstrap + migrations)
  worker/    background jobs entrypoint
  verify/    in-process integration harness
internal/
  config/     env loading, typed config
  database/   pgx pool, embedded golang-migrate runner + SQL migrations
  handler/    Fiber HTTP handlers (transport)
  middleware/ JWT, bot-token, rate limit
  service/    business logic, transactions, state machines
  repository/ pgx data access
  payment/    PaymentGateway interface + Midtrans adapter
  telegram/   backend→Telegram delivery client
  inventory/  concurrency-safe allocation
  worker/     job scheduler + definitions
  model/      domain types, DTOs, enums
  testkit/    fakes for tests/verification
pkg/          reusable helpers (logger, errors, crypto, token)
deployments/  docker-compose, nginx, seed.sql
```

## Notes / decisions

- **Payment gateway:** Midtrans only for the MVP; a thin `payment.Gateway` interface is retained
  purely for testability/mocking.
- **Order id:** `orders.id` is a `BIGSERIAL`; the externally-visible, gateway-safe identifier is
  `order_ref` (used as the Midtrans `order_id`).
- **Data access:** hand-written parameterized pgx queries (no string concatenation).
