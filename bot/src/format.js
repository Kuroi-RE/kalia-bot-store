const idr = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
});

// rupiah formats an integer IDR amount, e.g. 55000 -> "Rp 55.000".
export function rupiah(amount) {
  return idr.format(Number(amount) || 0);
}

// escapeHTML escapes text for Telegram HTML parse mode.
export function escapeHTML(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

// humanStatus maps an order status to a friendly line.
export function humanStatus(status) {
  switch (status) {
    case 'PENDING':
      return '⏳ Waiting for payment';
    case 'PAID':
      return '💰 Payment received — preparing your account';
    case 'DELIVERED':
      return '✅ Delivered — check the message with your account details';
    case 'EXPIRED':
      return '⌛ Payment window expired';
    case 'CANCELLED':
      return '❌ Order cancelled';
    case 'FAILED':
      return '⚠️ Delivery issue — our team was notified';
    default:
      return status || 'Unknown';
  }
}
