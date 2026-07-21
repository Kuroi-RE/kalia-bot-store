import { Telegraf, Markup } from 'telegraf';
import QRCode from 'qrcode';
import { config } from './config.js';
import { api, ApiError } from './api.js';
import { rupiah, escapeHTML, humanStatus, payBefore } from './format.js';
import { mainMenu, buildMainMenu, BTN, RESPONSE_BUTTONS } from './keyboards.js';
import { renderStyledQR } from './qr.js';

export function createBot() {
  const bot = new Telegraf(config.botToken);

  // Orders currently being polled, so the user is notified only once.
  const polling = new Set();

  // Cached enabled menus so the keyboard is dynamic without hitting the backend
  // on every message.
  let menuCache = [];
  let menuCacheAt = 0;
  const MENU_TTL_MS = 30000;

  async function getEnabledMenus() {
    if (Date.now() - menuCacheAt < MENU_TTL_MS) return menuCache;
    try {
      const { items = [] } = await api.listMenus();
      menuCache = items;
      menuCacheAt = Date.now();
    } catch {
      /* keep last cache on error */
    }
    return menuCache;
  }

  // currentKeyboard builds the reply keyboard: the fixed core buttons plus any
  // enabled backend menus appended below. Falls back to the core-only keyboard.
  async function currentKeyboard() {
    try {
      const menus = await getEnabledMenus();
      return buildMainMenu(menus.map((m) => m.title || m.command));
    } catch {
      return mainMenu;
    }
  }

  // confirmMessage is the alert shown when the user taps "Konfirmasi Pembayaran".
  function confirmMessage(status) {
    switch (status) {
      case 'PENDING':
        return '⏳ Pembayaran belum diterima. Jika sudah membayar, tunggu ±10 detik lalu tekan lagi.';
      case 'PAID':
        return '✅ Pembayaran diterima! Akun sedang diproses dan segera dikirim.';
      case 'DELIVERED':
        return '✅ Pembayaran berhasil! Akun sudah dikirim ke chat ini.';
      case 'EXPIRED':
        return '⌛ QR sudah kedaluwarsa. Silakan order ulang.';
      case 'CANCELLED':
        return '❌ Order dibatalkan.';
      case 'FAILED':
        return '⚠️ Ada kendala pengiriman, admin sedang menangani.';
      default:
        return humanStatus(status);
    }
  }

  const telegramUser = (ctx) => ({
    id: ctx.from.id,
    username: ctx.from.username || '',
    first_name: ctx.from.first_name || '',
  });

  // ---- top-level views ----

  const WELCOME =
    '👋 Selamat datang di <b>Kalia Store</b>!\n\n' +
    'Gunakan menu di bawah untuk melihat produk dan memesan. ' +
    'Detail akun dikirim otomatis ke chat ini setelah pembayaran dikonfirmasi.';

  async function showWelcome(ctx) {
    return ctx.reply(WELCOME, { parse_mode: 'HTML', ...(await currentKeyboard()) });
  }

  // showCatalog lists in-stock products (tiers) as "name - price" buttons.
  // Tapping one opens a detail view with specs before buying.
  async function showCatalog(ctx) {
    let data;
    try {
      data = await api.listProducts();
    } catch (err) {
      return ctx.reply(`Gagal memuat daftar akun: ${err.message}`, await currentKeyboard());
    }
    const items = data.items || [];
    if (items.length === 0) {
      return ctx.reply('Stok akun sedang kosong. Silakan cek kembali nanti.', await currentKeyboard());
    }
    const buttons = items.map((p) => [
      Markup.button.callback(`${p.name} - ${rupiah(p.price)}`, `detail:${p.product_id}`),
    ]);
    return ctx.reply('📦 <b>Pilih akun yang ingin dibeli:</b>', {
      parse_mode: 'HTML',
      ...Markup.inlineKeyboard(buttons),
    });
  }

  // showProductDetail shows a tier's price, stock and spec notes (description),
  // then one button per available handle so the buyer can pick the exact
  // account they want.
  async function showProductDetail(ctx, productId) {
    let products, accounts;
    try {
      [products, accounts] = await Promise.all([
        api.listProducts(),
        api.listAvailableAccounts(productId),
      ]);
    } catch (err) {
      return ctx.answerCbQuery(`Gagal memuat detail: ${err.message}`, { show_alert: true });
    }
    const p = (products.items || []).find((it) => it.product_id === productId);
    if (!p) {
      return ctx.answerCbQuery('Produk tidak tersedia atau stok habis.', { show_alert: true });
    }
    await ctx.answerCbQuery();

    let text =
      `📦 <b>${escapeHTML(p.name)}</b>\n` +
      `Harga: <b>${rupiah(p.price)}</b>\n` +
      `Stok: ${p.available} tersedia`;
    if (p.description && p.description.trim()) {
      text += `\n\n${escapeHTML(p.description)}`;
    }

    // One button per available handle (labels only, no secrets). Tapping buys
    // that specific account.
    const available = (accounts.items || []).filter((a) => a.product_id === productId);
    const MAX = 50;
    const rows = available
      .slice(0, MAX)
      .map((a) => [Markup.button.callback(`🛒 ${a.label}`, `buyacc:${a.account_id}`)]);

    if (rows.length) {
      text += `\n\nPilih akun yang ingin dibeli:`;
      if (available.length > MAX) {
        text += `\n<i>(menampilkan ${MAX} dari ${available.length} akun)</i>`;
      }
    } else {
      text += `\n\nStok kosong untuk saat ini.`;
    }
    rows.push([Markup.button.callback('⬅️ Kembali', 'catalog')]);

    return ctx.reply(text, { parse_mode: 'HTML', ...Markup.inlineKeyboard(rows) });
  }


  // sendResponse shows an admin-managed static response, or a fallback.
  async function sendResponse(ctx, command, fallback) {
    const kb = await currentKeyboard();
    try {
      const r = await api.getResponse(command);
      return ctx.reply(r.reply_text || fallback, kb);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        return ctx.reply(fallback, kb);
      }
      return ctx.reply(`Error: ${err.message}`, kb);
    }
  }

  // ---- order flow ----

  async function startOrder(ctx, opts) {
    let order;
    try {
      order = await api.createOrder(telegramUser(ctx), opts);
    } catch (err) {
      const msg =
        err instanceof ApiError && err.status === 409
          ? err.message || 'Stok habis'
          : `Order gagal: ${err.message}`;
      return ctx.answerCbQuery(msg, { show_alert: true });
    }
    await ctx.answerCbQuery();

    const caption =
      `🧾 <b>Order ${escapeHTML(order.order_ref)}</b>\n` +
      `Produk: ${escapeHTML(order.product?.name || '')}\n` +
      `Total: <b>${rupiah(order.amount)}</b>\n\n` +
      `Scan QRIS di bawah dengan e-wallet / m-banking untuk membayar.\n` +
      (order.expires_at ? `${payBefore(order.expires_at)}\n` : '') +
      `${humanStatus(order.status)}`;

    const statusKb = Markup.inlineKeyboard([
      Markup.button.callback('✅ Konfirmasi Pembayaran', `status:${order.order_ref}`),
    ]);

    // Render the QR: prefer the branded template frame with our dynamic QRIS
    // composited in; fall back to a plain QR PNG, then a provider image URL,
    // then text.
    let photoSent = false;
    if (order.qr_string) {
      try {
        const png = config.qrStyled
          ? await renderStyledQR(order.qr_string)
          : await QRCode.toBuffer(order.qr_string, { width: 360, margin: 2 });
        await ctx.replyWithPhoto({ source: png }, { caption, parse_mode: 'HTML', ...statusKb });
        photoSent = true;
      } catch (err) {
        console.error('QR render failed, falling back:', err.message);
      }
    }
    if (!photoSent && order.qr_image) {
      try {
        await ctx.replyWithPhoto(order.qr_image, { caption, parse_mode: 'HTML', ...statusKb });
        photoSent = true;
      } catch (err) {
        console.error('QR photo URL failed, falling back to text:', err.description || err.message);
      }
    }
    if (!photoSent) {
      const body = order.qr_string
        ? `${caption}\n\nQRIS:\n<code>${escapeHTML(order.qr_string)}</code>`
        : caption;
      await ctx.reply(body, { parse_mode: 'HTML', ...statusKb });
    }

    pollOrder(ctx, order.order_ref);
  }

  function pollOrder(ctx, orderRef) {
    if (polling.has(orderRef)) return;
    polling.add(orderRef);

    let attempts = 0;
    const tick = async () => {
      attempts += 1;
      if (attempts > config.pollMaxAttempts) {
        polling.delete(orderRef);
        return;
      }
      try {
        const order = await api.getOrder(orderRef);
        if (order.status && order.status !== 'PENDING') {
          polling.delete(orderRef);
          await ctx.telegram.sendMessage(
            ctx.from.id,
            `Order <b>${escapeHTML(orderRef)}</b>: ${humanStatus(order.status)}`,
            { parse_mode: 'HTML' },
          );
          return;
        }
      } catch {
        /* transient; keep polling */
      }
      setTimeout(tick, config.pollIntervalMs);
    };
    setTimeout(tick, config.pollIntervalMs);
  }

  // resolveCommand handles typed slash commands (order trigger, responses, menus).
  async function resolveCommand(ctx, command) {
    const cmd = command.replace(/^\//, '').toLowerCase();
    if (cmd === config.orderCommand) return showCatalog(ctx);
    const kb = await currentKeyboard();
    try {
      const resp = await api.getResponse(cmd);
      return ctx.reply(resp.reply_text || '(kosong)', kb);
    } catch (err) {
      if (!(err instanceof ApiError) || err.status !== 404) {
        return ctx.reply(`Error: ${err.message}`, kb);
      }
    }
    try {
      const menus = await getEnabledMenus();
      const menu = menus.find((m) => m.command.replace(/^\//, '').toLowerCase() === cmd);
      if (menu) return menu.reply_text ? ctx.reply(menu.reply_text, kb) : showCatalog(ctx);
    } catch {
      /* ignore */
    }
    return ctx.reply('Perintah tidak dikenali. Gunakan menu di bawah atau /start.', kb);
  }

  // handleMenuButton resolves a tapped dynamic menu button (its title text) to
  // the matching backend menu: opens the catalog for the order command, else
  // shows the menu's reply text.
  async function handleMenuButton(ctx) {
    const text = ctx.message?.text;
    if (!text) return;
    const menus = await getEnabledMenus();
    const menu = menus.find((m) => (m.title || m.command) === text);
    if (!menu) return; // not a menu button — ignore
    const cmd = (menu.command || '').replace(/^\//, '').toLowerCase();
    if (cmd === config.orderCommand) return showCatalog(ctx);
    if (menu.reply_text) return ctx.reply(menu.reply_text, await currentKeyboard());
    return showCatalog(ctx);
  }

  // ---- handlers ----

  bot.start(showWelcome);
  bot.command(config.orderCommand, showCatalog);

  // Persistent reply-keyboard buttons (their text is sent as a normal message).
  bot.hears(BTN.ORDER, showCatalog);
  for (const [label, { command, fallback }] of Object.entries(RESPONSE_BUTTONS)) {
    bot.hears(label, (ctx) => sendResponse(ctx, command, fallback));
  }

  // Any other typed slash command.
  bot.hears(/^\/[A-Za-z0-9_]+/, (ctx) => resolveCommand(ctx, ctx.message.text.split(/\s+/)[0]));

  // Any remaining text: resolve a tapped dynamic menu button by its title.
  bot.on('text', handleMenuButton);

  // Inline button actions.
  bot.on('callback_query', async (ctx) => {
    const data = ctx.callbackQuery.data || '';
    const parts = data.split(':');
    const action = parts[0];
    const arg = parts[1];
    try {
      if (action === 'catalog') {
        await ctx.answerCbQuery();
        return showCatalog(ctx);
      }
      if (action === 'detail') return showProductDetail(ctx, Number(arg));
      if (action === 'buyacc') return startOrder(ctx, { accountId: Number(arg) });
      if (action === 'buy') return startOrder(ctx, { productId: Number(arg) });
      if (action === 'status') {
        const order = await api.getOrder(arg);
        return ctx.answerCbQuery(confirmMessage(order.status), { show_alert: true });
      }
      return ctx.answerCbQuery();
    } catch (err) {
      return ctx.answerCbQuery(`Error: ${err.message}`, { show_alert: true });
    }
  });

  bot.catch((err, ctx) => {
    console.error(`bot error for ${ctx?.updateType}:`, err);
  });

  return bot;
}
