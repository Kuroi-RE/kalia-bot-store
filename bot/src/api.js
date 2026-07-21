import { config } from './config.js';

// ApiError carries the backend's status code and error message.
export class ApiError extends Error {
  constructor(status, code, message) {
    super(message || code || `HTTP ${status}`);
    this.status = status;
    this.code = code;
  }
}

const base = `${config.backendURL}/api/v1/bot`;

async function request(method, path, body) {
  const res = await fetch(base + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-Bot-Token': config.botServiceToken,
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  let payload = null;
  const text = await res.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = { raw: text };
    }
  }

  if (!res.ok) {
    const err = payload?.error || {};
    throw new ApiError(res.status, err.code, err.message);
  }
  return payload;
}

export const api = {
  // GET /bot/products -> { items: [{ product_id, name, description, price, available }] }
  listProducts: () => request('GET', '/products'),

  // GET /bot/accounts?product_id= -> { items: [{ account_id, product_id, product_name, price, label }] }
  listAvailableAccounts: (productId) =>
    request('GET', productId ? `/accounts?product_id=${productId}` : '/accounts'),

  // GET /bot/menus -> { items: [...] }
  listMenus: () => request('GET', '/menus'),

  // GET /bot/responses/:command -> { command, reply_text, ... }
  getResponse: (command) =>
    request('GET', `/responses/${encodeURIComponent(command.replace(/^\//, ''))}`),

  // POST /bot/orders -> { order_ref, amount, qr_string, qr_image, expires_at, ... }
  // opts: { productId } to buy any available account of a product (tier), or
  // { accountId } to buy a specific account.
  createOrder: (telegramUser, opts = {}) =>
    request('POST', '/orders', {
      telegram_user: telegramUser,
      product_id: opts.productId,
      account_id: opts.accountId,
    }),

  // GET /bot/orders/:order_ref -> full order
  getOrder: (orderRef) => request('GET', `/orders/${encodeURIComponent(orderRef)}`),
};
