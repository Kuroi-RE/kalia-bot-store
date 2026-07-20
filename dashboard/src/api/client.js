// Base path is relative so the Vite dev proxy (or nginx in prod) forwards it to
// the backend. Override with VITE_API_BASE for a direct absolute URL.
const API_BASE = (import.meta.env.VITE_API_BASE || '') + '/api/v1';

const TOKEN_KEY = 'kalia_token';

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (t) => localStorage.setItem(TOKEN_KEY, t),
  clear: () => localStorage.removeItem(TOKEN_KEY),
};

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message || code || `HTTP ${status}`);
    this.status = status;
    this.code = code;
  }
}

// onUnauthorized is set by the auth layer to react to 401s (e.g. logout).
let onUnauthorized = () => {};
export function setUnauthorizedHandler(fn) {
  onUnauthorized = fn;
}

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  const token = tokenStore.get();
  if (token) headers.Authorization = `Bearer ${token}`;

  const res = await fetch(API_BASE + path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return null;

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
    if (res.status === 401) onUnauthorized();
    const err = payload?.error || {};
    throw new ApiError(res.status, err.code, err.message);
  }
  return payload;
}

const qs = (params) => {
  const clean = Object.entries(params || {}).filter(([, v]) => v !== undefined && v !== '' && v !== null);
  return clean.length ? '?' + new URLSearchParams(clean).toString() : '';
};

export const api = {
  // auth
  login: (username, password) => request('POST', '/auth/login', { username, password }),
  me: () => request('GET', '/auth/me'),

  // products
  listProducts: (params) => request('GET', `/products${qs(params)}`),
  getProduct: (id) => request('GET', `/products/${id}`),
  createProduct: (body) => request('POST', '/products', body),
  updateProduct: (id, body) => request('PUT', `/products/${id}`, body),
  setProductStatus: (id, isActive) => request('PATCH', `/products/${id}/status`, { is_active: isActive }),
  deleteProduct: (id, force) => request('DELETE', `/products/${id}${force ? '?force=true' : ''}`),

  // accounts / inventory
  listAccounts: (productId, params) => request('GET', `/products/${productId}/accounts${qs(params)}`),
  inventorySummary: (productId) => request('GET', `/products/${productId}/inventory-summary`),
  createAccounts: (productId, body) => request('POST', `/products/${productId}/accounts`, body),
  updateAccount: (id, body) => request('PUT', `/accounts/${id}`, body),
  deleteAccount: (id) => request('DELETE', `/accounts/${id}`),

  // telegram menus
  listMenus: () => request('GET', '/telegram/menus'),
  createMenu: (body) => request('POST', '/telegram/menus', body),
  updateMenu: (id, body) => request('PUT', `/telegram/menus/${id}`, body),
  setMenuStatus: (id, isEnabled) => request('PATCH', `/telegram/menus/${id}/status`, { is_enabled: isEnabled }),
  deleteMenu: (id) => request('DELETE', `/telegram/menus/${id}`),

  // telegram responses
  listResponses: () => request('GET', '/telegram/responses'),
  createResponse: (body) => request('POST', '/telegram/responses', body),
  updateResponse: (id, body) => request('PUT', `/telegram/responses/${id}`, body),
  setResponseStatus: (id, isEnabled) => request('PATCH', `/telegram/responses/${id}/status`, { is_enabled: isEnabled }),
  deleteResponse: (id) => request('DELETE', `/telegram/responses/${id}`),

  // orders / payments
  listOrders: (params) => request('GET', `/orders${qs(params)}`),
  getOrder: (id) => request('GET', `/orders/${id}`),
  cancelOrder: (id) => request('PATCH', `/orders/${id}/cancel`),
  orderPayment: (id) => request('GET', `/orders/${id}/payment`),

  // deliveries
  listDeliveries: (params) => request('GET', `/deliveries${qs(params)}`),
  redeliver: (orderId) => request('POST', `/orders/${orderId}/redeliver`),

  // settings
  listSettings: () => request('GET', '/settings'),
  setSetting: (key, value) => request('PUT', `/settings/${encodeURIComponent(key)}`, { value }),
};
