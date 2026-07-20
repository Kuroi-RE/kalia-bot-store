# Kalia Store — Telegram Bot

A [Telegraf](https://telegraf.js.org/) (Node.js) bot that is a **pure consumer** of the backend's
`/api/v1/bot/*` REST endpoints. It never touches the database directly.

## What it does

- `/start` — welcome message with buttons for the admin-defined menus plus an "Order now" button.
- `/order` (configurable) — shows the active product catalog as inline buttons with prices.
- Tapping a product creates an order and shows the QRIS code + amount + order reference.
- Any other `/command` resolves an admin-defined static response (e.g. `/testimoni`) or menu reply.
- After an order is placed the bot polls its status and notifies the customer when payment is
  received. **The account credentials themselves are delivered automatically by the backend**
  (via the Telegram Bot API) once payment settles.

## Prerequisites

- Node.js 18+
- A running backend (see the repo root README)
- A Telegram bot token from [@BotFather](https://t.me/BotFather)

## Configuration

Copy `.env.example` to `.env` and fill in:

| Variable | Description |
|---|---|
| `TELEGRAM_BOT_TOKEN` | Bot token from @BotFather (**must be the same token** the backend uses to deliver credentials) |
| `BACKEND_URL` | Backend base URL, no trailing slash (`http://localhost:8080`, or `http://localhost` via nginx, or `http://api:8080` inside the compose network) |
| `BOT_SERVICE_TOKEN` | Must match `BOT_SERVICE_TOKEN` in the backend `.env` |
| `ORDER_COMMAND` | Command that opens the catalog (default `order`) |
| `POLL_INTERVAL_MS` | Status poll interval (default 10000) |
| `POLL_MAX_ATTEMPTS` | Max status polls per order (default 90) |

> The bot and the backend share the same `TELEGRAM_BOT_TOKEN`: the bot *receives* updates
> (long polling) and the backend *sends* credential messages. Only one process should call
> `getUpdates`, and only the bot does — so there is no conflict.

## Run

```bash
cd bot
npm install
cp .env.example .env      # then edit
npm start
```

## Trying it end-to-end without a Midtrans account

Run the backend with the fake payment gateway so orders can be created and settled locally:

```bash
# backend (repo root)
set PAYMENT_MODE=fake
go run ./cmd/api
```

Then in Telegram: `/start` → **Order now** → pick a product → you'll get a (placeholder) QR.
To simulate payment, call the dev settle endpoint (only enabled when `PAYMENT_MODE=fake`):

```bash
curl -X POST http://localhost:8080/api/v1/dev/settle/<ORDER_REF>
```

The backend then delivers the account credentials to your Telegram chat and the bot posts a
"payment received" update. For real payments, set a valid `MIDTRANS_SERVER_KEY`, drop
`PAYMENT_MODE`, and configure the Midtrans webhook (see the root README).

## Docker

A `Dockerfile` is provided and a `bot` service is wired into
`deployments/docker-compose.yml`. It requires a real `TELEGRAM_BOT_TOKEN` in the root `.env`
(otherwise it exits on startup).
