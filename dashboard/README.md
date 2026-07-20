# Kalia Store — Admin Dashboard

React + Vite single-page admin dashboard with a **Neobrutalism** design. Consumes the backend
admin REST API (JWT-authenticated).

## Features

- **Login** (JWT) — uses the admin seeded from the backend's `ADMIN_USERNAME`/`ADMIN_PASSWORD`.
- **Products** — create/edit/enable/disable/delete, including a credential-schema editor
  (per-product fields: key/label/type/required).
- **Inventory** — per product: available/reserved/sold summary, list accounts (filter by status),
  add accounts (form built from the product schema), delete.
- **Telegram** — manage static **Responses** (edit `/testimoni`, `/contact`, `/bantuan`, …) and
  **Menus** (command/title/reply/sort/enable).
- **Orders** — list + filter by status, view detail with payment info, cancel PENDING orders.
- **Deliveries** — list + filter, retry FAILED deliveries.
- **Settings** — edit runtime key/value settings.

## Prerequisites

- Node.js 18+
- The backend running (see repo root README). The dashboard talks to `/api/v1`.

## Run (development)

```bash
cd dashboard
npm install
cp .env.example .env      # set VITE_PROXY_TARGET to your backend (default http://localhost via nginx)
npm run dev               # http://localhost:5173
```

`vite dev` proxies `/api` and `/health` to `VITE_PROXY_TARGET`, so there are no CORS concerns in
development. Log in with your admin credentials (default `admin` / `admin12345` from the sample
`.env`).

## Build (production)

```bash
npm run build             # outputs static files to dist/
npm run preview           # preview the production build locally
```

Serve `dist/` behind any static host / nginx, and reverse-proxy `/api` to the backend. The
backend enables CORS (`CORS_ALLOWED_ORIGINS`, default `*`) so the SPA may also call it directly
via `VITE_API_BASE`.

## Design

Neobrutalism: thick black borders, hard offset shadows (no blur), bright saturated colors,
chunky controls, Space Grotesk / JetBrains Mono type. Tokens live in `tailwind.config.js`
(`shadow-brutal*`, color palette) and component classes in `src/index.css` (`.nb-card`,
`.nb-btn*`, `.nb-input`, `.nb-badge`, …).
