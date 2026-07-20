import { Markup } from 'telegraf';

// Core fixed buttons — the CURRENT keyboard, kept exactly as-is.
const CORE_ROWS = [
  ['📦 Order', '⭐ Testimoni'],
  ['📞 Contact'], ['❓ Bantuan'],
];

// Button labels (exported so handlers can match on them).
export const BTN = {
  ORDER: '📦 Order',
  TESTIMONI: '⭐ Testimoni',
  CONTACT: '📞 Contact',
  BANTUAN: '❓ Bantuan',
};

const CORE_LABELS = new Set(Object.values(BTN));

// Buttons that display an admin-managed static response (telegram_responses).
// `command` is looked up via GET /bot/responses/:command; `fallback` is shown
// when the admin hasn't configured that response yet.
export const RESPONSE_BUTTONS = {
  [BTN.TESTIMONI]: { command: 'testimoni', fallback: 'link testi: t.me/testibykayi' },
  [BTN.CONTACT]: { command: 'contact', fallback: 'Silakan hubungi admin untuk bantuan.' },
  [BTN.BANTUAN]: {
    command: 'bantuan',
    fallback:
      'Cara order:\n1. Tekan 📦 Order untuk melihat produk\n2. Pilih produk yang ingin dibeli\n3. Bayar QRIS yang muncul\n4. Akun dikirim otomatis setelah pembayaran',
  },
};

// Static fallback keyboard — the core buttons only. Used when backend menus
// can't be loaded.
export const mainMenu = Markup.keyboard(CORE_ROWS).resize();

// buildMainMenu keeps the core buttons unchanged and appends admin-defined
// menu buttons (from the dashboard) below them, two per row. Titles that would
// duplicate a core button or are empty are skipped.
export function buildMainMenu(menuTitles = []) {
  const seen = new Set(CORE_LABELS);
  const extras = [];
  for (const t of menuTitles) {
    const title = (t || '').trim();
    if (!title || seen.has(title)) continue;
    seen.add(title);
    extras.push(title);
  }
  const rows = CORE_ROWS.map((row) => [...row]);
  for (let i = 0; i < extras.length; i += 2) {
    rows.push(extras.slice(i, i + 2));
  }
  return Markup.keyboard(rows).resize();
}
